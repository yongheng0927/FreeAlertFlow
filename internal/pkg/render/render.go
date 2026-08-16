// Package render 实现消息模板引擎（FR-2.3）：Go text/template + Sprig +
// 自定义函数，渲染出按渠道类型编写的完整消息体 JSON，并按渠道类型校验
// 其中包含合法的消息类型字段
package render

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/Masterminds/sprig/v3"
)

// BuiltinChannelTypes 是内置模板覆盖的渠道类型；模板必须归属具体渠道，
// 内容是该渠道消息体的完整 payload Go template
var BuiltinChannelTypes = []string{"feishu", "dingtalk", "wecom"}

// Context 是暴露给消息模板的模板上下文（FR-2.3）
type Context struct {
	Version           string
	Status            string // 分组状态：firing / resolved
	Receiver          string
	GroupKey          string
	ExternalURL       string
	GroupLabels       map[string]string
	CommonLabels      map[string]string
	CommonAnnotations map[string]string
	Alerts            []Alert

	// 系统变量
	SourceName string
	RootURL    string
}

// Alert 是模板上下文中的单条告警
type Alert struct {
	Status       string
	Labels       map[string]string
	Annotations  map[string]string
	StartsAt     time.Time
	EndsAt       time.Time
	GeneratorURL string
	Fingerprint  string
}

// legalFeishuMsgTypes 是可作为渲染结果的飞书消息类型
var legalFeishuMsgTypes = map[string]bool{
	"text":        true,
	"post":        true,
	"interactive": true,
	"image":       true,
	"share_chat":  true,
}

// legalMarkdownMsgTypes 是钉钉/企微机器人支持的消息类型
var legalMarkdownMsgTypes = map[string]bool{
	"text":     true,
	"markdown": true,
}

// Engine 使用 Sprig 和自定义函数渲染模板
type Engine struct {
	loc *time.Location
}

// NewEngine 创建 Engine；loc 是 timeFormat 使用的时区（nil 表示 time.Local）
func NewEngine(loc *time.Location) *Engine {
	if loc == nil {
		loc = time.Local
	}
	return &Engine{loc: loc}
}

// Render 用 ctx 执行 tmplText，并按 channelType 校验结果是该渠道的合法
// 消息体（有效 JSON 且消息类型字段合法）
func (e *Engine) Render(tmplText string, ctx *Context, channelType string) (string, error) {
	tmpl, err := template.New("message").Funcs(e.funcMap()).Parse(tmplText)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	out := buf.String()
	if err := ValidatePayloadJSON(out, channelType); err != nil {
		return "", err
	}
	return out, nil
}

// ValidatePayloadJSON 检查 s 是 channelType 渠道的合法消息体：
// 飞书要求 msg_type 已知；钉钉/企微要求 msgtype ∈ {text, markdown}
func ValidatePayloadJSON(s, channelType string) error {
	var body map[string]any
	if err := json.Unmarshal([]byte(s), &body); err != nil {
		return fmt.Errorf("rendered result is not valid JSON: %w", err)
	}
	switch channelType {
	case "feishu":
		mt, _ := body["msg_type"].(string)
		if !legalFeishuMsgTypes[mt] {
			return fmt.Errorf("rendered result has missing or illegal msg_type %q", mt)
		}
	case "dingtalk", "wecom":
		mt, _ := body["msgtype"].(string)
		if !legalMarkdownMsgTypes[mt] {
			return fmt.Errorf("rendered result has missing or illegal msgtype %q (want text or markdown)", mt)
		}
	default:
		return fmt.Errorf("unsupported channel type %q", channelType)
	}
	return nil
}

func (e *Engine) funcMap() template.FuncMap {
	fm := sprig.TxtFuncMap()
	fm["severityColor"] = SeverityColor
	fm["timeFormat"] = e.timeFormat
	fm["label"] = labelValue
	fm["truncate"] = truncate
	fm["mdEscape"] = mdEscape
	fm["jesc"] = jsonEscape
	return fm
}

// SeverityColor 将 severity 映射为渠道卡片/消息的提示色（如飞书卡片头部
// 颜色）
func SeverityColor(sev string) string {
	switch strings.ToLower(sev) {
	case "critical", "fatal", "error":
		return "red"
	case "warning", "warn":
		return "orange"
	case "info":
		return "blue"
	case "resolved", "ok", "none":
		return "green"
	default:
		return "blue"
	}
}

// timeFormat 按引擎时区以 Go layout 格式化 t
func (e *Engine) timeFormat(layout string, t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.In(e.loc).Format(layout)
}

// labelValue 返回 m[key]，不存在时返回 ""（模板内永不报错）
func labelValue(m map[string]string, key string) string {
	return m[key]
}

// truncate 将 s 截断到最多 n 个 rune，发生截断时追加省略号
func truncate(n int, s string) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// mdEscape 为 markdown（如 lark_md）文本转义 Markdown 元字符
var mdEscaper = strings.NewReplacer(
	`\`, `\\`,
	"`", "\\`",
	"*", `\*`,
	"_", `\_`,
	"[", `\[`,
	"]", `\]`,
)

func mdEscape(s string) string { return mdEscaper.Replace(s) }

// jsonEscape 转义 s，使其可以嵌入 JSON 字符串字面量内部
// （不添加首尾引号）
func jsonEscape(s string) string {
	q := strconv.Quote(s)
	return q[1 : len(q)-1]
}

// --- 内置模板（由迁移 0005 镜像到数据库） ---

//go:embed templates/*.tmpl
var builtinFS embed.FS

// BuiltinTemplate 描述一条内置模板记录
type BuiltinTemplate struct {
	Name        string
	ChannelType string
	Content     string
	Remark      string
}

// builtinFiles 按渠道类型组织内置模板文件：4 个模板名 × 3 个渠道类型 =
// 12 条记录，文件命名 templates/{channel}_{name}.tmpl
var builtinFiles = []struct {
	name, channelType, file, remark string
}{
	{"critical-card", "feishu", "templates/feishu_critical_card.tmpl", "Builtin: critical alert card (red header)"},
	{"warning-card", "feishu", "templates/feishu_warning_card.tmpl", "Builtin: warning alert card (orange header)"},
	{"resolved-card", "feishu", "templates/feishu_resolved_card.tmpl", "Builtin: resolved notification card (green header)"},
	{"plain-text", "feishu", "templates/feishu_plain_text.tmpl", "Builtin: plain text message"},
	{"critical-card", "dingtalk", "templates/dingtalk_critical_card.tmpl", "Builtin: critical alert (markdown)"},
	{"warning-card", "dingtalk", "templates/dingtalk_warning_card.tmpl", "Builtin: warning alert (markdown)"},
	{"resolved-card", "dingtalk", "templates/dingtalk_resolved_card.tmpl", "Builtin: resolved notification (markdown)"},
	{"plain-text", "dingtalk", "templates/dingtalk_plain_text.tmpl", "Builtin: plain markdown message"},
	{"critical-card", "wecom", "templates/wecom_critical_card.tmpl", "Builtin: critical alert (markdown)"},
	{"warning-card", "wecom", "templates/wecom_warning_card.tmpl", "Builtin: warning alert (markdown)"},
	{"resolved-card", "wecom", "templates/wecom_resolved_card.tmpl", "Builtin: resolved notification (markdown)"},
	{"plain-text", "wecom", "templates/wecom_plain_text.tmpl", "Builtin: plain markdown message"},
}

// BuiltinTemplates 返回所有内置模板：每个模板文件一条记录
func BuiltinTemplates() ([]BuiltinTemplate, error) {
	out := make([]BuiltinTemplate, 0, len(builtinFiles))
	for _, b := range builtinFiles {
		content, err := builtinFS.ReadFile(b.file)
		if err != nil {
			return nil, err
		}
		out = append(out, BuiltinTemplate{
			Name:        b.name,
			ChannelType: b.channelType,
			Content:     string(content),
			Remark:      b.remark,
		})
	}
	return out, nil
}

// DefaultBuiltinName 为未绑定模板的 channel 挑选内置默认模板：resolved
// 分组用 resolved 卡片，critical 级别用 critical 卡片，其余用 warning 卡片
func DefaultBuiltinName(status, severity string) string {
	if strings.EqualFold(status, "resolved") {
		return "resolved-card"
	}
	if strings.EqualFold(severity, "critical") {
		return "critical-card"
	}
	return "warning-card"
}
