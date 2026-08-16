-- 0003_markdown_templates：内置模板改为渠道无关的 Markdown 模板（FR-2.3）
-- 模板内容不再是完整的飞书消息 JSON，而是 Go text/template + Sprig 插值的
-- Markdown 文本；发送时由后端按渠道方言转换并包装成消息体
-- 内容为 internal/pkg/render/templates/*.tmpl 的逐字节副本
--（一致性由 internal/pkg/render/render_test.go 强制保证）

-- 移除 0002 写入的旧飞书 JSON 内置模板
DELETE FROM templates WHERE is_builtin;

INSERT INTO templates (name, channel_type, content, is_builtin, remark) VALUES
('critical-card', 'common', $faf$
## 🔥 [{{ .Status | upper }}] {{ label .CommonLabels "alertname" }}

- **级别**：{{ label .CommonLabels "severity" }}
- **状态**：{{ .Status }}
- **来源**：{{ .SourceName }}
- **告警条数**：{{ len .Alerts }}
{{ range $a := .Alerts }}
---

**实例**：{{ label $a.Labels "instance" }}

**开始时间**：{{ timeFormat "2006-01-02 15:04:05" $a.StartsAt }}

{{ label $a.Annotations "summary" | truncate 200 }}
{{ end }}
$faf$, TRUE, 'Builtin: critical alert (markdown)'),
('warning-card', 'common', $faf$
## ⚠️ [{{ .Status | upper }}] {{ label .CommonLabels "alertname" }}

- **级别**：{{ label .CommonLabels "severity" }}
- **状态**：{{ .Status }}
- **来源**：{{ .SourceName }}
- **告警条数**：{{ len .Alerts }}
{{ range $a := .Alerts }}
---

**实例**：{{ label $a.Labels "instance" }}

**开始时间**：{{ timeFormat "2006-01-02 15:04:05" $a.StartsAt }}

{{ label $a.Annotations "summary" | truncate 200 }}
{{ end }}
$faf$, TRUE, 'Builtin: warning alert (markdown)'),
('resolved-card', 'common', $faf$
## ✅ [RESOLVED] {{ label .CommonLabels "alertname" }}

- **级别**：{{ label .CommonLabels "severity" }}
- **来源**：{{ .SourceName }}
- **告警条数**：{{ len .Alerts }}
{{ range $a := .Alerts }}
---

**实例**：{{ label $a.Labels "instance" }}

**开始时间**：{{ timeFormat "2006-01-02 15:04:05" $a.StartsAt }}

**恢复时间**：{{ timeFormat "2006-01-02 15:04:05" $a.EndsAt }}
{{ end }}
$faf$, TRUE, 'Builtin: resolved notification (markdown)'),
('plain-text', 'common', $faf$
[{{ .Status | upper }}] {{ label .CommonLabels "alertname" }}

**级别**：{{ label .CommonLabels "severity" }}　**来源**：{{ .SourceName }}　**告警条数**：{{ len .Alerts }}
{{ range .Alerts }}
---

**实例**：{{ label .Labels "instance" }}　**开始**：{{ timeFormat "2006-01-02 15:04:05" .StartsAt }}

{{ label .Annotations "summary" | truncate 200 }}
{{ end }}
$faf$, TRUE, 'Builtin: plain markdown message');
