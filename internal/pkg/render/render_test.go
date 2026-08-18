package render

import (
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

func sampleContext() *Context {
	return &Context{
		Version:  "4",
		Status:   "firing",
		Receiver: "freealertflow",
		GroupKey: "{}:{alertname=\"HighCPU\"}",
		GroupLabels: map[string]string{
			"alertname": "HighCPU",
		},
		CommonLabels: map[string]string{
			"alertname": "HighCPU",
			"severity":  "critical",
			"instance":  "10.0.0.1:9100",
			"namespace": "prod",
		},
		CommonAnnotations: map[string]string{
			"summary": "CPU usage above 90% for 5m",
		},
		ExternalURL: "http://alertmanager:9093",
		Alerts: []Alert{
			{
				Status: "firing",
				Labels: map[string]string{
					"alertname": "HighCPU",
					"severity":  "critical",
					"instance":  "10.0.0.1:9100",
				},
				Annotations: map[string]string{
					"summary": "CPU usage above 90% for 5m",
				},
				StartsAt:    time.Date(2026, 8, 15, 2, 0, 0, 0, time.UTC),
				Fingerprint: "0123456789abcdef",
			},
		},
		SourceName: "生产环境 Prometheus",
		RootURL:    "https://alerts.example.com/",
	}
}

// TestBuiltinTemplatesRender 检查每个内置模板都能把示例 Alertmanager v4
// 负载渲染成通过本渠道校验的消息体，且包含关键告警信息
func TestBuiltinTemplatesRender(t *testing.T) {
	builtins, err := BuiltinTemplates()
	if err != nil {
		t.Fatalf("BuiltinTemplates: %v", err)
	}
	if len(builtins) != 4*len(BuiltinChannelTypes) {
		t.Fatalf("got %d builtin templates, want %d", len(builtins), 4*len(BuiltinChannelTypes))
	}
	validType := map[string]bool{}
	for _, ct := range BuiltinChannelTypes {
		validType[ct] = true
	}
	engine := NewEngine(time.UTC)
	for _, b := range builtins {
		if !validType[b.ChannelType] {
			t.Errorf("%s/%s: unknown channel type", b.Name, b.ChannelType)
			continue
		}
		ctx := sampleContext()
		if b.Name == "resolved-card" {
			ctx.Status = "resolved"
			ctx.Alerts[0].Status = "resolved"
			ctx.Alerts[0].EndsAt = time.Date(2026, 8, 15, 2, 30, 0, 0, time.UTC)
		}
		// Render 内部已按渠道类型校验 payload（合法 JSON + 消息类型字段）
		out, err := engine.Render(b.Content, ctx, b.ChannelType)
		if err != nil {
			t.Errorf("%s/%s: Render: %v", b.Name, b.ChannelType, err)
			continue
		}
		for _, want := range []string{"HighCPU", "critical", "10.0.0.1:9100"} {
			if !strings.Contains(out, want) {
				t.Errorf("%s/%s: rendered payload lacks %q:\n%s", b.Name, b.ChannelType, want, out)
			}
		}
		if b.Name == "resolved-card" && !strings.Contains(out, "2026-08-15 02:30:00") {
			t.Errorf("resolved-card/%s: rendered payload lacks endsAt:\n%s", b.ChannelType, out)
		}
	}
}

// TestBuiltinTemplatesMatchMigration 强制保证内嵌 .tmpl 文件与迁移 0005
// 写入的数据行内容一致
func TestBuiltinTemplatesMatchMigration(t *testing.T) {
	sql, err := os.ReadFile("../../../migrations/0005_channel_payload_templates.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	// Windows 检出（core.autocrlf=true）会带入 CRLF，比较前统一归一为 LF
	sqlText := strings.ReplaceAll(string(sql), "\r\n", "\n")
	re := regexp.MustCompile(`(?s)\('([^']+)', '([^']+)', \$faf\$\n(.*?)\$faf\$, TRUE,`)
	type key struct{ name, channelType string }
	rows := map[key]string{}
	for _, m := range re.FindAllStringSubmatch(sqlText, -1) {
		rows[key{m[1], m[2]}] = strings.TrimSpace(m[3])
	}
	builtins, err := BuiltinTemplates()
	if err != nil {
		t.Fatalf("BuiltinTemplates: %v", err)
	}
	for _, b := range builtins {
		k := key{b.Name, b.ChannelType}
		content, ok := rows[k]
		if !ok {
			t.Errorf("migration 0005 has no row for %q/%q", b.Name, b.ChannelType)
			continue
		}
		if content != strings.TrimSpace(strings.ReplaceAll(b.Content, "\r\n", "\n")) {
			t.Errorf("migration 0005 content of %q/%q differs from embedded template", b.Name, b.ChannelType)
		}
		delete(rows, k)
	}
	for k := range rows {
		t.Errorf("migration 0005 row %q/%q has no embedded template", k.name, k.channelType)
	}
}

func TestRenderRejectsBadTemplate(t *testing.T) {
	engine := NewEngine(nil)
	if _, err := engine.Render("{{ .NoSuchField", sampleContext(), "feishu"); err == nil {
		t.Fatal("unparsable template must fail")
	}
	// 渲染结果必须是合法 JSON
	if _, err := engine.Render("not json at all", sampleContext(), "feishu"); err == nil {
		t.Fatal("non-JSON result must fail")
	}
}

// TestRenderRejectsEnvFuncs 模板引擎使用 Hermetic 函数表，env/expandenv/
// getHostByName 等可读取进程环境或网络的函数必须不可用，防止模板编辑者
// 通过渲染窃取服务器机密
func TestRenderRejectsEnvFuncs(t *testing.T) {
	engine := NewEngine(nil)
	for _, fn := range []string{"env", "expandenv", "getHostByName"} {
		tmpl := `{"msg_type":"text","content":{"text":"{{ ` + fn + ` \"HOME\" }}"}}`
		if _, err := engine.Render(tmpl, sampleContext(), "feishu"); err == nil {
			t.Errorf("template using %q must fail to parse", fn)
		}
	}
}

// TestValidatePayloadJSON 按渠道类型校验渲染结果的消息类型字段
func TestValidatePayloadJSON(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		chType  string
		wantErr bool
	}{
		{"feishu text", `{"msg_type":"text","content":{"text":"hi"}}`, "feishu", false},
		{"feishu card", `{"msg_type":"interactive","card":{}}`, "feishu", false},
		{"feishu bad msg_type", `{"msg_type":"audio"}`, "feishu", true},
		{"feishu missing msg_type", `{"content":{}}`, "feishu", true},
		{"feishu dingtalk key mismatch", `{"msgtype":"markdown"}`, "feishu", true},
		{"dingtalk markdown", `{"msgtype":"markdown","markdown":{"title":"t","text":"x"}}`, "dingtalk", false},
		{"dingtalk text", `{"msgtype":"text","text":{"content":"x"}}`, "dingtalk", false},
		{"dingtalk interactive not allowed", `{"msg_type":"interactive"}`, "dingtalk", true},
		{"dingtalk actionCard not allowed", `{"msgtype":"actionCard"}`, "dingtalk", true},
		{"wecom markdown", `{"msgtype":"markdown","markdown":{"content":"x"}}`, "wecom", false},
		{"wecom image not allowed", `{"msgtype":"image"}`, "wecom", true},
		{"unknown channel type", `{"msg_type":"text"}`, "slack", true},
		{"invalid json", `not json`, "feishu", true},
	}
	for _, tc := range cases {
		err := ValidatePayloadJSON(tc.payload, tc.chType)
		if (err != nil) != tc.wantErr {
			t.Errorf("%s: ValidatePayloadJSON err = %v, wantErr = %v", tc.name, err, tc.wantErr)
		}
	}
}

func TestCustomFuncs(t *testing.T) {
	engine := NewEngine(time.UTC)

	if got := SeverityColor("critical"); got != "red" {
		t.Errorf("SeverityColor(critical) = %q", got)
	}
	if got := SeverityColor("warning"); got != "orange" {
		t.Errorf("SeverityColor(warning) = %q", got)
	}
	if got := SeverityColor("info"); got != "blue" {
		t.Errorf("SeverityColor(info) = %q", got)
	}

	ts := time.Date(2026, 8, 15, 12, 30, 45, 0, time.UTC)
	if got := engine.timeFormat("2006-01-02 15:04:05", ts); got != "2026-08-15 12:30:45" {
		t.Errorf("timeFormat = %q", got)
	}

	if got := labelValue(map[string]string{"a": "b"}, "a"); got != "b" {
		t.Errorf("labelValue = %q", got)
	}
	if got := labelValue(map[string]string{}, "missing"); got != "" {
		t.Errorf("labelValue missing = %q", got)
	}

	if got := truncate(3, "abcdef"); got != "abc…" {
		t.Errorf("truncate = %q", got)
	}
	if got := truncate(10, "短文本"); got != "短文本" {
		t.Errorf("truncate runes = %q", got)
	}

	if got := jsonEscape(`he"llo
world`); got != `he\"llo\nworld` {
		t.Errorf("jesc = %q", got)
	}
	if got := mdEscape("a*b_c`d[e]\\f"); got != "a\\*b\\_c\\`d\\[e\\]\\\\f" {
		t.Errorf("mdEscape = %q", got)
	}
}
