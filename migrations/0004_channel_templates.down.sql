-- 0004_channel_templates down：删除按渠道类型的 12 条内置模板，恢复 0003
-- 的 4 条渠道无关（common）内置模板
-- 注意：up 中归入 feishu 的存量自定义 common 模板无法精确还原，不回退

DELETE FROM templates
WHERE is_builtin
  AND channel_type IN ('feishu', 'dingtalk', 'wecom')
  AND name IN ('critical-card', 'warning-card', 'resolved-card', 'plain-text');

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
