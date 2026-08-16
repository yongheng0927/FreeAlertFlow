-- 0003_markdown_templates down：移除渠道无关的 Markdown 内置模板，
-- 恢复 0002 写入的飞书 JSON 内置模板

DELETE FROM templates
WHERE channel_type = 'common' AND is_builtin
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
