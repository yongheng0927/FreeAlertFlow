import { Button, Card, Col, Divider, Row, Space, Typography } from 'antd'

const HEADER_COLORS: Record<string, string> = {
  red: '#f5222d',
  orange: '#fa8c16',
  green: '#52c41a',
  blue: '#1677ff',
  yellow: '#fadb14',
  grey: '#8c8c8c',
  purple: '#722ed1',
  indigo: '#2f54eb',
  turquoise: '#13c2c2',
  violet: '#eb2f96',
  carmine: '#cf1322',
  wathet: '#69c0ff',
}

interface FeishuText {
  tag?: string
  content?: string
}

interface FeishuElement {
  tag?: string
  text?: FeishuText
  fields?: { text?: FeishuText }[]
  actions?: { text?: FeishuText; url?: string; type?: string }[]
}

interface FeishuCard {
  header?: { template?: string; title?: FeishuText }
  elements?: FeishuElement[]
}

interface FeishuMessage {
  msg_type?: string
  card?: FeishuCard
  content?: { text?: string }
}

/** 剥离最基础的 markdown 标记，做纯文本展示 */
function stripMd(s: string): string {
  return s
    .replace(/\*\*/g, '')
    .replace(/~~/g, '')
    .replace(/<at[^>]*><\/at>/g, '@')
}

/** 本地模拟的飞书卡片预览：header 颜色/标题 + 文本、按钮、分栏等常见元素的近似渲染 */
export function CardPreview({ payload }: { payload: string }) {
  let msg: FeishuMessage
  try {
    msg = JSON.parse(payload) as FeishuMessage
  } catch {
    return <Typography.Text type="warning">飞书消息体不是合法 JSON，无法生成卡片预览</Typography.Text>
  }

  if (msg.msg_type === 'text') {
    return (
      <Card size="small" style={{ maxWidth: 480 }}>
        <div style={{ whiteSpace: 'pre-wrap' }}>{stripMd(msg.content?.text ?? '')}</div>
      </Card>
    )
  }

  if (msg.msg_type !== 'interactive' || !msg.card) {
    return <Typography.Text type="secondary">暂不支持预览 msg_type={msg.msg_type ?? '未知'} 的卡片效果</Typography.Text>
  }

  const header = msg.card.header
  const headerColor = HEADER_COLORS[header?.template ?? ''] ?? '#1677ff'

  return (
    <Card
      size="small"
      style={{ maxWidth: 480, borderTop: `4px solid ${headerColor}` }}
      title={
        header?.title?.content ? (
          <span style={{ color: headerColor }}>{stripMd(header.title.content)}</span>
        ) : undefined
      }
    >
      {(msg.card.elements ?? []).map((el, i) => {
        if (el.tag === 'hr') return <Divider key={i} style={{ margin: '8px 0' }} />
        if (el.tag === 'action') {
          return (
            <Space key={i} wrap>
              {(el.actions ?? []).map((a, j) => (
                <Button key={j} size="small" type={a.type === 'primary' ? 'primary' : 'default'} href={a.url} target="_blank">
                  {a.text?.content ?? '按钮'}
                </Button>
              ))}
            </Space>
          )
        }
        if (el.fields && el.fields.length > 0) {
          return (
            <Row key={i} gutter={[8, 8]} style={{ marginBottom: 8 }}>
              {el.fields.map((f, j) => (
                <Col span={12} key={j} style={{ whiteSpace: 'pre-wrap', fontSize: 12 }}>
                  {stripMd(f.text?.content ?? '')}
                </Col>
              ))}
            </Row>
          )
        }
        return (
          <div key={i} style={{ whiteSpace: 'pre-wrap', marginBottom: 8, fontSize: 12 }}>
            {stripMd(el.text?.content ?? '')}
          </div>
        )
      })}
    </Card>
  )
}
