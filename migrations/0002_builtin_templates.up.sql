-- 0002_builtin_templates：写入内置飞书消息模板（FR-2.3）
-- 内容为 internal/pkg/render/templates/*.tmpl 的逐字节副本
--（一致性由 internal/pkg/render/render_test.go 强制保证）

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
}
$faf$, TRUE, 'Builtin: critical alert card (red header)'),
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
}
$faf$, TRUE, 'Builtin: warning alert card (orange header)'),
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
}
$faf$, TRUE, 'Builtin: resolved notification card (green header)'),
('plain-text', 'feishu', $faf$
{
  "msg_type": "text",
  "content": {
    "text": "[{{ .Status | upper }}] {{ label .CommonLabels "alertname" | jesc }}\n级别: {{ label .CommonLabels "severity" | jesc }}\n来源: {{ jesc .SourceName }}\n告警条数: {{ len .Alerts }}{{ range .Alerts }}\n实例: {{ label .Labels "instance" | jesc }} 开始: {{ timeFormat "2006-01-02 15:04:05" .StartsAt }}\n{{ label .Annotations "summary" | truncate 200 | jesc }}{{ end }}"
  }
}
$faf$, TRUE, 'Builtin: plain text message');
