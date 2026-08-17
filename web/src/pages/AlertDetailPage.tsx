import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { Button, Card, Descriptions, Space, Table, Tabs, Typography, message } from 'antd'
import { ArrowLeftOutlined, ReloadOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'

import { getAlert, listChannels, listSources } from '../api'
import { errorMessage } from '../api/client'
import type { AlertDetail, Channel, Delivery, Source } from '../api/types'
import {
  AlertStatusTag,
  DeliveryStatusTag,
  DispositionTag,
  SeverityTag,
} from '../components/tags'
import { CardPreview } from '../components/CardPreview'
import { JsonView } from '../components/JsonView'
import { formatTime } from '../utils'

function KvTable({ data }: { data: Record<string, string> }) {
  const rows = Object.entries(data ?? {}).map(([k, v]) => ({ key: k, value: v }))
  if (rows.length === 0) return <Typography.Text type="secondary">无</Typography.Text>
  return (
    <Table
      size="small"
      rowKey="key"
      pagination={false}
      dataSource={rows}
      columns={[
        { title: '键', dataIndex: 'key', width: 200 },
        { title: '值', dataIndex: 'value' },
      ]}
    />
  )
}

/** 解析 JSON 字符串，失败时按原字符串返回（JsonView 会按字符串展示） */
function safeParse(s: string): unknown {
  try {
    return JSON.parse(s)
  } catch {
    return s
  }
}

const deliveryColumns: ColumnsType<Delivery> = [
  { title: '渠道', dataIndex: 'channel_name', width: 140 },
  {
    title: '状态',
    dataIndex: 'status',
    width: 80,
    render: (s: string) => <DeliveryStatusTag status={s} />,
  },
  {
    title: '结果',
    width: 220,
    render: (_, d) =>
      d.response_msg ? (
        <Typography.Text type={d.status === 'failed' ? 'danger' : undefined}>
          code={d.response_code} {d.response_msg}
        </Typography.Text>
      ) : (
        `HTTP ${d.http_status}`
      ),
  },
  { title: '耗时', dataIndex: 'duration_ms', width: 90, render: (v: number) => `${v} ms` },
  { title: '尝试', dataIndex: 'attempts', width: 60 },
  {
    title: '触发',
    dataIndex: 'trigger_type',
    width: 80,
    render: (t: string) => (t === 'manual' ? '手动' : '自动'),
  },
  { title: '时间', dataIndex: 'sent_at', width: 170, render: formatTime },
]

export default function AlertDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [detail, setDetail] = useState<AlertDetail | null>(null)
  const [deliveries, setDeliveries] = useState<Delivery[]>([])
  const [loading, setLoading] = useState(false)
  const [sources, setSources] = useState<Source[]>([])
  const [channels, setChannels] = useState<Channel[]>([])

  const sourceName = useMemo(() => {
    const m = new Map(sources.map((s) => [s.id, s.name]))
    return (sid: number) => m.get(sid) ?? `#${sid}`
  }, [sources])

  const channelType = useMemo(() => {
    const m = new Map(channels.map((c) => [c.id, c.type]))
    return (cid: number) => m.get(cid) ?? ''
  }, [channels])

  useEffect(() => {
    listSources().then((r) => setSources(r.list)).catch(() => undefined)
    listChannels().then((r) => setChannels(r.list)).catch(() => undefined)
  }, [])

  const load = async () => {
    setLoading(true)
    try {
      const r = await getAlert(Number(id))
      setDetail(r.alert)
      setDeliveries(r.deliveries)
    } catch (err) {
      message.error(errorMessage(err, '加载告警详情失败'))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id])

  return (
    <Card
      loading={loading && detail === null}
      title={
        <Space>
          <Button
            type="text"
            size="small"
            icon={<ArrowLeftOutlined />}
            onClick={() => navigate('/alerts')}
          />
          <span>告警详情{detail ? `：${detail.alertname}` : ''}</span>
        </Space>
      }
      extra={
        <Button icon={<ReloadOutlined />} onClick={() => void load()}>
          刷新
        </Button>
      }
    >
      {detail && (
        <>
          <Descriptions size="small" column={2} bordered style={{ marginBottom: 16 }}>
            <Descriptions.Item label="状态">
              <AlertStatusTag status={detail.status} />
            </Descriptions.Item>
            <Descriptions.Item label="级别">
              <SeverityTag severity={detail.severity} />
            </Descriptions.Item>
            <Descriptions.Item label="来源">{sourceName(detail.source_id)}</Descriptions.Item>
            <Descriptions.Item label="处置">
              <DispositionTag disposition={detail.disposition} />
            </Descriptions.Item>
            <Descriptions.Item label="指纹" span={2}>
              <Typography.Text code>{detail.fingerprint}</Typography.Text>
            </Descriptions.Item>
            <Descriptions.Item label="开始时间">{formatTime(detail.starts_at)}</Descriptions.Item>
            <Descriptions.Item label="结束时间">{formatTime(detail.ends_at)}</Descriptions.Item>
            <Descriptions.Item label="接收时间" span={2}>
              {formatTime(detail.received_at)}
            </Descriptions.Item>
          </Descriptions>
          <Tabs
            items={[
              {
                key: 'labels',
                label: 'Labels',
                children: <KvTable data={detail.labels} />,
              },
              {
                key: 'annotations',
                label: 'Annotations',
                children: <KvTable data={detail.annotations} />,
              },
              {
                key: 'raw',
                label: '原始 Payload',
                children: <JsonView data={detail.raw_payload} />,
              },
              {
                key: 'deliveries',
                label: `投递记录（${deliveries.length}）`,
                children: (
                  <Table<Delivery>
                    rowKey="id"
                    size="small"
                    columns={deliveryColumns}
                    dataSource={deliveries}
                    pagination={false}
                    locale={{ emptyText: '该告警没有投递记录（可能被去重或未匹配路由规则）' }}
                    expandable={{
                      rowExpandable: (d) => d.rendered_payload !== null,
                      expandedRowRender: (d) => {
                        const payload = d.rendered_payload ?? ''
                        return (
                          <div style={{ padding: '4px 0' }}>
                            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                              实际发送的消息体
                              {channelType(d.channel_id) === 'feishu' &&
                                '（卡片效果为本地模拟，以实际发送为准）'}
                            </Typography.Text>
                            {channelType(d.channel_id) === 'feishu' && (
                              <div style={{ marginTop: 8, marginBottom: 12 }}>
                                <CardPreview payload={payload} />
                              </div>
                            )}
                            <div style={{ marginTop: 8 }}>
                              <JsonView data={safeParse(payload)} />
                            </div>
                          </div>
                        )
                      },
                    }}
                  />
                ),
              },
            ]}
          />
        </>
      )}
    </Card>
  )
}
