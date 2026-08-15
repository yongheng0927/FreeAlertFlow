import { Tag } from 'antd'

const SEVERITY_COLOR: Record<string, string> = {
  critical: 'red',
  warning: 'orange',
  info: 'blue',
}

export function SeverityTag({ severity }: { severity: string }) {
  if (!severity) return <Tag>-</Tag>
  return <Tag color={SEVERITY_COLOR[severity] ?? 'default'}>{severity}</Tag>
}

export function AlertStatusTag({ status }: { status: string }) {
  if (status === 'firing') return <Tag color="red">firing</Tag>
  if (status === 'resolved') return <Tag color="green">resolved</Tag>
  return <Tag>{status || '-'}</Tag>
}

const DISPOSITION: Record<string, { text: string; color: string }> = {
  delivered: { text: '已投递', color: 'green' },
  deduped: { text: '已去重', color: 'default' },
  unmatched: { text: '未匹配规则', color: 'orange' },
}

export function DispositionTag({ disposition }: { disposition: string }) {
  const d = DISPOSITION[disposition]
  return d ? <Tag color={d.color}>{d.text}</Tag> : <Tag>{disposition || '-'}</Tag>
}

export function DeliveryStatusTag({ status }: { status: string }) {
  if (status === 'success') return <Tag color="green">成功</Tag>
  if (status === 'failed') return <Tag color="red">失败</Tag>
  return <Tag>{status || '-'}</Tag>
}
