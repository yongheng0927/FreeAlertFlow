-- 0005_channel_payload_templates down：删除 payload 版内置模板，恢复 0004
-- 的 markdown 版 12 条种子

DELETE FROM templates
WHERE is_builtin
  AND channel_type IN ('feishu', 'dingtalk', 'wecom')
  AND name IN ('critical-card', 'warning-card', 'resolved-card', 'plain-text');

INSERT INTO templates (name, channel_type, content, is_builtin, remark) VALUES
('critical-card', 'feishu', $faf$
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
$faf$, TRUE, 'Builtin: critical alert (markdown)'),('critical-card', 'dingtalk', $faf$
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
$faf$, TRUE, 'Builtin: critical alert (markdown)'),('critical-card', 'wecom', $faf$
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
$faf$, TRUE, 'Builtin: critical alert (markdown)'),('warning-card', 'feishu', $faf$
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
$faf$, TRUE, 'Builtin: warning alert (markdown)'),('warning-card', 'dingtalk', $faf$
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
$faf$, TRUE, 'Builtin: warning alert (markdown)'),('warning-card', 'wecom', $faf$
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
$faf$, TRUE, 'Builtin: warning alert (markdown)'),('resolved-card', 'feishu', $faf$
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
$faf$, TRUE, 'Builtin: resolved notification (markdown)'),('resolved-card', 'dingtalk', $faf$
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
$faf$, TRUE, 'Builtin: resolved notification (markdown)'),('resolved-card', 'wecom', $faf$
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
$faf$, TRUE, 'Builtin: resolved notification (markdown)'),('plain-text', 'feishu', $faf$
[{{ .Status | upper }}] {{ label .CommonLabels "alertname" }}

**级别**：{{ label .CommonLabels "severity" }}　**来源**：{{ .SourceName }}　**告警条数**：{{ len .Alerts }}
{{ range .Alerts }}
---

**实例**：{{ label .Labels "instance" }}　**开始**：{{ timeFormat "2006-01-02 15:04:05" .StartsAt }}

{{ label .Annotations "summary" | truncate 200 }}
{{ end }}
$faf$, TRUE, 'Builtin: plain markdown message'),('plain-text', 'dingtalk', $faf$
[{{ .Status | upper }}] {{ label .CommonLabels "alertname" }}

**级别**：{{ label .CommonLabels "severity" }}　**来源**：{{ .SourceName }}　**告警条数**：{{ len .Alerts }}
{{ range .Alerts }}
---

**实例**：{{ label .Labels "instance" }}　**开始**：{{ timeFormat "2006-01-02 15:04:05" .StartsAt }}

{{ label .Annotations "summary" | truncate 200 }}
{{ end }}
$faf$, TRUE, 'Builtin: plain markdown message'),('plain-text', 'wecom', $faf$
[{{ .Status | upper }}] {{ label .CommonLabels "alertname" }}

**级别**：{{ label .CommonLabels "severity" }}　**来源**：{{ .SourceName }}　**告警条数**：{{ len .Alerts }}
{{ range .Alerts }}
---

**实例**：{{ label .Labels "instance" }}　**开始**：{{ timeFormat "2006-01-02 15:04:05" .StartsAt }}

{{ label .Annotations "summary" | truncate 200 }}
{{ end }}
$faf$, TRUE, 'Builtin: plain markdown message');
