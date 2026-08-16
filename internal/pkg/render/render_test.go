package render

import (
	"encoding/json"
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
// 负载渲染成合法的飞书消息体
func TestBuiltinTemplatesRender(t *testing.T) {
	builtins, err := BuiltinTemplates()
	if err != nil {
		t.Fatalf("BuiltinTemplates: %v", err)
	}
	if len(builtins) != 4 {
		t.Fatalf("got %d builtin templates, want 4", len(builtins))
	}
	engine := NewEngine(time.UTC)
	for _, b := range builtins {
		ctx := sampleContext()
		if b.Name == "resolved-card" {
			ctx.Status = "resolved"
			ctx.Alerts[0].Status = "resolved"
			ctx.Alerts[0].EndsAt = time.Date(2026, 8, 15, 2, 30, 0, 0, time.UTC)
		}
		out, err := engine.Render(b.Content, ctx)
		if err != nil {
			t.Errorf("%s: Render: %v", b.Name, err)
			continue
		}
		var body map[string]any
		if err := json.Unmarshal([]byte(out), &body); err != nil {
			t.Errorf("%s: result is not valid JSON: %v", b.Name, err)
			continue
		}
		switch body["msg_type"] {
		case "interactive":
			card, ok := body["card"].(map[string]any)
			if !ok {
				t.Errorf("%s: interactive message missing card", b.Name)
			}
			header, _ := card["header"].(map[string]any)
			title, _ := header["title"].(map[string]any)
			content, _ := title["content"].(string)
			if !strings.Contains(content, "HighCPU") {
				t.Errorf("%s: card title lacks alertname: %q", b.Name, content)
			}
		case "text":
			c, _ := body["content"].(map[string]any)
			text, _ := c["text"].(string)
			if !strings.Contains(text, "HighCPU") {
				t.Errorf("%s: text lacks alertname: %q", b.Name, text)
			}
		default:
			t.Errorf("%s: unexpected msg_type %v", b.Name, body["msg_type"])
		}
	}
}

// TestBuiltinTemplatesMatchMigration 强制保证内嵌 .tmpl 文件与迁移 0002
// 写入的数据行内容一致
func TestBuiltinTemplatesMatchMigration(t *testing.T) {
	sql, err := os.ReadFile("../../../migrations/0002_builtin_templates.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	re := regexp.MustCompile(`(?s)\('([^']+)', 'feishu', \$faf\$\n(.*?)\$faf\$, TRUE,`)
	rows := map[string]string{}
	for _, m := range re.FindAllStringSubmatch(string(sql), -1) {
		rows[m[1]] = strings.TrimSpace(m[2])
	}
	builtins, err := BuiltinTemplates()
	if err != nil {
		t.Fatalf("BuiltinTemplates: %v", err)
	}
	for _, b := range builtins {
		content, ok := rows[b.Name]
		if !ok {
			t.Errorf("migration 0002 has no row for %q", b.Name)
			continue
		}
		if content != strings.TrimSpace(b.Content) {
			t.Errorf("migration 0002 content of %q differs from embedded template", b.Name)
		}
		delete(rows, b.Name)
	}
	for name := range rows {
		t.Errorf("migration 0002 row %q has no embedded template", name)
	}
}

func TestRenderRejectsBadTemplate(t *testing.T) {
	engine := NewEngine(nil)
	if _, err := engine.Render("{{ .NoSuchField", sampleContext()); err == nil {
		t.Fatal("unparsable template must fail")
	}
	if _, err := engine.Render("not json at all", sampleContext()); err == nil {
		t.Fatal("non-JSON result must fail validation")
	}
	if _, err := engine.Render(`{"foo": "bar"}`, sampleContext()); err == nil {
		t.Fatal("JSON without msg_type must fail validation")
	}
	if _, err := engine.Render(`{"msg_type": "video"}`, sampleContext()); err == nil {
		t.Fatal("illegal msg_type must fail validation")
	}
}

func TestCustomFuncs(t *testing.T) {
	engine := NewEngine(time.UTC)

	if got := severityColor("critical"); got != "red" {
		t.Errorf("severityColor(critical) = %q", got)
	}
	if got := severityColor("warning"); got != "orange" {
		t.Errorf("severityColor(warning) = %q", got)
	}
	if got := severityColor("info"); got != "blue" {
		t.Errorf("severityColor(info) = %q", got)
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

	if got := mdEscape("a*b_[x]"); got != `a\*b\_\[x\]` {
		t.Errorf("mdEscape = %q", got)
	}

	if got := jsonEscape(`say "hi"`); got != `say \"hi\"` {
		t.Errorf("jsonEscape = %q", got)
	}
}

// TestJSONInjectionEscaped 确保模板使用 jesc 时，恶意的标签值无法破坏
// 渲染出的 JSON
func TestJSONInjectionEscaped(t *testing.T) {
	engine := NewEngine(time.UTC)
	ctx := sampleContext()
	ctx.CommonLabels["alertname"] = "evil\"}\n{\"msg_type\":\"x"
	out, err := engine.Render(`{"msg_type":"text","content":{"text":"{{ label .CommonLabels "alertname" | jesc }}"}}`, ctx)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(out), &body); err != nil {
		t.Fatalf("result must remain valid JSON: %v", err)
	}
	text := body["content"].(map[string]any)["text"].(string)
	if text != "evil\"}\n{\"msg_type\":\"x" {
		t.Fatalf("escaping changed the value: %q", text)
	}
}
