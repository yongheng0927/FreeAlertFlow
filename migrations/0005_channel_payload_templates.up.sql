-- 0005_channel_payload_templates：内置模板内容从「渠道无关 Markdown + Go 插值」
-- 回退为「按渠道类型编写的完整消息 payload Go template」（markdown 中间层
-- 方案被否决后的定稿） 4 个模板名 × 3 个渠道类型 = 12 条种子
-- 内容为 internal/pkg/render/templates/*.tmpl 的逐字节副本
--（一致性由 internal/pkg/render/render_test.go 强制保证）

-- 移除 0004 写入的 markdown 版内置模板
DELETE FROM templates
WHERE is_builtin
  AND channel_type IN ('feishu', 'dingtalk', 'wecom')
  AND name IN ('critical-card', 'warning-card', 'resolved-card', 'plain-text');

INSERT INTO templates (name, channel_type, content, is_builtin, remark) VALUES
('critical-card', 'feishu', $faf$
{
  "msg_type": "interactive",
  "card": {
    "header": {
      "template": "red",
      "title": {"tag": "plain_text", "content": "🔥 [{{ .Status | upper }}] {{ label .CommonLabels "alertname" | jesc }}"}
    },
    "elements": [
      {"tag": "div", "fields": [
        {"is_short": true, "text": {"tag": "lark_md", "content": "**级别**\n{{ label .CommonLabels "severity" | jesc }}"}},
        {"is_short": true, "text": {"tag": "lark_md", "content": "**状态**\n{{ jesc .Status }}"}},
        {"is_short": true, "text": {"tag": "lark_md", "content": "**来源**\n{{ jesc .SourceName }}"}},
        {"is_short": true, "text": {"tag": "lark_md", "content": "**告警条数**\n{{ len .Alerts }}"}}
      ]},
      {{ range $i, $a := .Alerts }}{{ if $i }},{{ end }}
      {"tag": "div", "text": {"tag": "lark_md", "content": "**实例** {{ label $a.Labels "instance" | jesc }}\n**开始时间** {{ timeFormat "2006-01-02 15:04:05" $a.StartsAt }}\n{{ label $a.Annotations "summary" | truncate 200 | mdEscape | jesc }}"}}
      {{ end }}
    ]
  }
}$faf$, TRUE, 'Builtin: critical alert card (red header)'),
('warning-card', 'feishu', $faf$
{
  "msg_type": "interactive",
  "card": {
    "header": {
      "template": "orange",
      "title": {"tag": "plain_text", "content": "⚠️ [{{ .Status | upper }}] {{ label .CommonLabels "alertname" | jesc }}"}
    },
    "elements": [
      {"tag": "div", "fields": [
        {"is_short": true, "text": {"tag": "lark_md", "content": "**级别**\n{{ label .CommonLabels "severity" | jesc }}"}},
        {"is_short": true, "text": {"tag": "lark_md", "content": "**状态**\n{{ jesc .Status }}"}},
        {"is_short": true, "text": {"tag": "lark_md", "content": "**来源**\n{{ jesc .SourceName }}"}},
        {"is_short": true, "text": {"tag": "lark_md", "content": "**告警条数**\n{{ len .Alerts }}"}}
      ]},
      {{ range $i, $a := .Alerts }}{{ if $i }},{{ end }}
      {"tag": "div", "text": {"tag": "lark_md", "content": "**实例** {{ label $a.Labels "instance" | jesc }}\n**开始时间** {{ timeFormat "2006-01-02 15:04:05" $a.StartsAt }}\n{{ label $a.Annotations "summary" | truncate 200 | mdEscape | jesc }}"}}
      {{ end }}
    ]
  }
}$faf$, TRUE, 'Builtin: warning alert card (orange header)'),
('resolved-card', 'feishu', $faf$
{
  "msg_type": "interactive",
  "card": {
    "header": {
      "template": "green",
      "title": {"tag": "plain_text", "content": "✅ [RESOLVED] {{ label .CommonLabels "alertname" | jesc }}"}
    },
    "elements": [
      {"tag": "div", "fields": [
        {"is_short": true, "text": {"tag": "lark_md", "content": "**级别**\n{{ label .CommonLabels "severity" | jesc }}"}},
        {"is_short": true, "text": {"tag": "lark_md", "content": "**来源**\n{{ jesc .SourceName }}"}},
        {"is_short": true, "text": {"tag": "lark_md", "content": "**告警条数**\n{{ len .Alerts }}"}}
      ]},
      {{ range $i, $a := .Alerts }}{{ if $i }},{{ end }}
      {"tag": "div", "text": {"tag": "lark_md", "content": "**实例** {{ label $a.Labels "instance" | jesc }}\n**开始时间** {{ timeFormat "2006-01-02 15:04:05" $a.StartsAt }}\n**恢复时间** {{ timeFormat "2006-01-02 15:04:05" $a.EndsAt }}"}}
      {{ end }}
    ]
  }
}$faf$, TRUE, 'Builtin: resolved notification card (green header)'),
('plain-text', 'feishu', $faf$
{
  "msg_type": "text",
  "content": {
    "text": "[{{ .Status | upper }}] {{ label .CommonLabels "alertname" | jesc }}\n级别: {{ label .CommonLabels "severity" | jesc }}\n来源: {{ jesc .SourceName }}\n告警条数: {{ len .Alerts }}{{ range .Alerts }}\n实例: {{ label .Labels "instance" | jesc }} 开始: {{ timeFormat "2006-01-02 15:04:05" .StartsAt }}\n{{ label .Annotations "summary" | truncate 200 | jesc }}{{ end }}"
  }
}$faf$, TRUE, 'Builtin: plain text message'),
('critical-card', 'dingtalk', $faf$
{
  "msgtype": "markdown",
  "markdown": {
    "title": "🔥 [{{ .Status | upper }}] {{ label .CommonLabels "alertname" | jesc }}",
    "text": "## 🔥 [{{ .Status | upper }}] {{ label .CommonLabels "alertname" | jesc }}\n\n- **级别**：{{ label .CommonLabels "severity" | jesc }}\n- **状态**：{{ jesc .Status }}\n- **来源**：{{ jesc .SourceName }}\n- **告警条数**：{{ len .Alerts }}\n{{ range $a := .Alerts }}\n---\n\n**实例**：{{ label $a.Labels "instance" | jesc }}\n\n**开始时间**：{{ timeFormat "2006-01-02 15:04:05" $a.StartsAt }}\n\n{{ label $a.Annotations "summary" | truncate 200 | jesc }}\n{{ end }}"
  }
}$faf$, TRUE, 'Builtin: critical alert (markdown)'),
('warning-card', 'dingtalk', $faf$
{
  "msgtype": "markdown",
  "markdown": {
    "title": "⚠️ [{{ .Status | upper }}] {{ label .CommonLabels "alertname" | jesc }}",
    "text": "## ⚠️ [{{ .Status | upper }}] {{ label .CommonLabels "alertname" | jesc }}\n\n- **级别**：{{ label .CommonLabels "severity" | jesc }}\n- **状态**：{{ jesc .Status }}\n- **来源**：{{ jesc .SourceName }}\n- **告警条数**：{{ len .Alerts }}\n{{ range $a := .Alerts }}\n---\n\n**实例**：{{ label $a.Labels "instance" | jesc }}\n\n**开始时间**：{{ timeFormat "2006-01-02 15:04:05" $a.StartsAt }}\n\n{{ label $a.Annotations "summary" | truncate 200 | jesc }}\n{{ end }}"
  }
}$faf$, TRUE, 'Builtin: warning alert (markdown)'),
('resolved-card', 'dingtalk', $faf$
{
  "msgtype": "markdown",
  "markdown": {
    "title": "✅ [RESOLVED] {{ label .CommonLabels "alertname" | jesc }}",
    "text": "## ✅ [RESOLVED] {{ label .CommonLabels "alertname" | jesc }}\n\n- **级别**：{{ label .CommonLabels "severity" | jesc }}\n- **来源**：{{ jesc .SourceName }}\n- **告警条数**：{{ len .Alerts }}\n{{ range $a := .Alerts }}\n---\n\n**实例**：{{ label $a.Labels "instance" | jesc }}\n\n**开始时间**：{{ timeFormat "2006-01-02 15:04:05" $a.StartsAt }}\n\n**恢复时间**：{{ timeFormat "2006-01-02 15:04:05" $a.EndsAt }}\n{{ end }}"
  }
}$faf$, TRUE, 'Builtin: resolved notification (markdown)'),
('plain-text', 'dingtalk', $faf$
{
  "msgtype": "markdown",
  "markdown": {
    "title": "[{{ .Status | upper }}] {{ label .CommonLabels "alertname" | jesc }}",
    "text": "[{{ .Status | upper }}] {{ label .CommonLabels "alertname" | jesc }}\n\n**级别**：{{ label .CommonLabels "severity" | jesc }}　**来源**：{{ jesc .SourceName }}　**告警条数**：{{ len .Alerts }}{{ range .Alerts }}\n\n---\n\n**实例**：{{ label .Labels "instance" | jesc }}　**开始**：{{ timeFormat "2006-01-02 15:04:05" .StartsAt }}\n\n{{ label .Annotations "summary" | truncate 200 | jesc }}{{ end }}"
  }
}$faf$, TRUE, 'Builtin: plain markdown message'),
('critical-card', 'wecom', $faf$
{
  "msgtype": "markdown",
  "markdown": {
    "content": "## 🔥 [{{ .Status | upper }}] {{ label .CommonLabels "alertname" | jesc }}\n> **级别**：{{ label .CommonLabels "severity" | jesc }}\n> **状态**：{{ jesc .Status }}\n> **来源**：{{ jesc .SourceName }}\n> **告警条数**：{{ len .Alerts }}{{ range $a := .Alerts }}\n\n**实例**：{{ label $a.Labels "instance" | jesc }}\n**开始时间**：{{ timeFormat "2006-01-02 15:04:05" $a.StartsAt }}\n{{ label $a.Annotations "summary" | truncate 200 | jesc }}{{ end }}"
  }
}$faf$, TRUE, 'Builtin: critical alert (markdown)'),
('warning-card', 'wecom', $faf$
{
  "msgtype": "markdown",
  "markdown": {
    "content": "## ⚠️ [{{ .Status | upper }}] {{ label .CommonLabels "alertname" | jesc }}\n> **级别**：{{ label .CommonLabels "severity" | jesc }}\n> **状态**：{{ jesc .Status }}\n> **来源**：{{ jesc .SourceName }}\n> **告警条数**：{{ len .Alerts }}{{ range $a := .Alerts }}\n\n**实例**：{{ label $a.Labels "instance" | jesc }}\n**开始时间**：{{ timeFormat "2006-01-02 15:04:05" $a.StartsAt }}\n{{ label $a.Annotations "summary" | truncate 200 | jesc }}{{ end }}"
  }
}$faf$, TRUE, 'Builtin: warning alert (markdown)'),
('resolved-card', 'wecom', $faf$
{
  "msgtype": "markdown",
  "markdown": {
    "content": "## ✅ [RESOLVED] {{ label .CommonLabels "alertname" | jesc }}\n> **级别**：{{ label .CommonLabels "severity" | jesc }}\n> **来源**：{{ jesc .SourceName }}\n> **告警条数**：{{ len .Alerts }}{{ range $a := .Alerts }}\n\n**实例**：{{ label $a.Labels "instance" | jesc }}\n**开始时间**：{{ timeFormat "2006-01-02 15:04:05" $a.StartsAt }}\n**恢复时间**：{{ timeFormat "2006-01-02 15:04:05" $a.EndsAt }}{{ end }}"
  }
}$faf$, TRUE, 'Builtin: resolved notification (markdown)'),
('plain-text', 'wecom', $faf$
{
  "msgtype": "markdown",
  "markdown": {
    "content": "**[{{ .Status | upper }}] {{ label .CommonLabels "alertname" | jesc }}**\n> 级别：{{ label .CommonLabels "severity" | jesc }}　来源：{{ jesc .SourceName }}　告警条数：{{ len .Alerts }}{{ range .Alerts }}\n\n**实例**：{{ label .Labels "instance" | jesc }}　**开始**：{{ timeFormat "2006-01-02 15:04:05" .StartsAt }}\n{{ label .Annotations "summary" | truncate 200 | jesc }}{{ end }}"
  }
}$faf$, TRUE, 'Builtin: plain markdown message');
