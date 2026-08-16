-- 0004_channel_templates：内置模板按渠道类型各备一套（FR-2.3）
-- 模板回退为按渠道类型归属（feishu / dingtalk / wecom 三选一），内容仍是
-- 渠道无关的 Markdown + Go 插值；4 个内置模板 × 3 个渠道类型 = 12 条种子
-- 内容为 internal/pkg/render/templates/*.tmpl 的逐字节副本
--（一致性由 internal/pkg/render/render_test.go 强制保证）

-- 移除 0003 写入的 common 内置模板
DELETE FROM templates WHERE is_builtin AND channel_type = 'common';

-- 存量用户自定义的 common 模板统一归入 feishu：common 已不再合法，必须归属
-- 某个渠道类型；选 feishu 是预发布阶段的任意取舍（生产尚无 common 自定义
-- 模板），用户可在界面上改绑其他渠道类型的模板
UPDATE templates SET channel_type = 'feishu' WHERE channel_type = 'common';

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
