import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Button,
  Card,
  DatePicker,
  Descriptions,
  Drawer,
  Form,
  Input,
  Select,
  Table,
  Tabs,
  Tooltip,
  Typography,
  message,
} from 'antd'
import { ReloadOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import type { Dayjs } from 'dayjs'

import { getAlert, listAlerts, listChannels, listSources } from '../api'
import { errorMessage } from '../api/client'
import type { Alert, AlertDetail, Channel, Delivery, Source } from '../api/types'
import {
  AlertStatusTag,
  DeliveryStatusTag,
  DispositionTag,
  SeverityTag,
} from '../components/tags'
import { formatTime } from '../utils'

interface FilterValues {
  status?: string
  severity?: string
  alertname?: string
  channel_id?: number
  range?: [Dayjs | null, Dayjs | null] | null
}

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

export default function AlertsPage() {
  const [form] = Form.useForm<FilterValues>()
  const [data, setData] = useState<Alert[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [loading, setLoading] = useState(false)
  const [sources, setSources] = useState<Source[]>([])
  const [channels, setChannels] = useState<Channel[]>([])

  const [drawerOpen, setDrawerOpen] = useState(false)
  const [detail, setDetail] = useState<AlertDetail | null>(null)
  const [deliveries, setDeliveries] = useState<Delivery[]>([])
  const [detailLoading, setDetailLoading] = useState(false)

  const sourceName = useMemo(() => {
    const m = new Map(sources.map((s) => [s.id, s.name]))
    return (id: number) => m.get(id) ?? `#${id}`
  }, [sources])

  useEffect(() => {
    listSources().then((r) => setSources(r.list)).catch(() => undefined)
    listChannels().then((r) => setChannels(r.list)).catch(() => undefined)
  }, [])

  const load = useCallback(
    async (p: number, ps: number) => {
      setLoading(true)
      const v = form.getFieldsValue()
      const [start, end] = v.range ?? [null, null]
      try {
        const r = await listAlerts({
          status: v.status || undefined,
          severity: v.severity || undefined,
          alertname: v.alertname || undefined,
          channel_id: v.channel_id || undefined,
          start: start ? start.toISOString() : undefined,
          end: end ? end.toISOString() : undefined,
          page: p,
          page_size: ps,
        })
        setData(r.list)
        setTotal(r.total)
        setPage(r.page)
        setPageSize(r.page_size)
      } catch (err) {
        message.error(errorMessage(err, '加载告警列表失败'))
      } finally {
        setLoading(false)
      }
    },
    [form],
  )

  useEffect(() => {
    void load(1, pageSize)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const openDetail = async (row: Alert) => {
    setDrawerOpen(true)
    setDetailLoading(true)
    setDetail(null)
    setDeliveries([])
    try {
      const r = await getAlert(row.id)
      setDetail(r.alert)
      setDeliveries(r.deliveries)
    } catch (err) {
      message.error(errorMessage(err, '加载告警详情失败'))
    } finally {
      setDetailLoading(false)
    }
  }

  const columns: ColumnsType<Alert> = [
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      render: (s: string) => <AlertStatusTag status={s} />,
    },
    {
      title: '级别',
      dataIndex: 'severity',
      width: 90,
      render: (s: string) => <SeverityTag severity={s} />,
    },
    {
      title: '告警名称',
      dataIndex: 'alertname',
      width: 200,
      render: (v: string) => (
        <Tooltip title={v}>
          <Typography.Text ellipsis style={{ maxWidth: 180 }}>
            {v}
          </Typography.Text>
        </Tooltip>
      ),
    },
    {
      title: '实例',
      width: 180,
      render: (_, a) => a.labels?.instance ?? '-',
    },
    { title: '来源', dataIndex: 'source_id', width: 120, render: (id: number) => sourceName(id) },
    {
      title: '处置',
      dataIndex: 'disposition',
      width: 110,
      render: (d: string) => <DispositionTag disposition={d} />,
    },
    { title: '接收时间', dataIndex: 'received_at', width: 175, render: formatTime },
  ]

  return (
    <Card
      title="告警记录"
      extra={
        <Button icon={<ReloadOutlined />} onClick={() => void load(page, pageSize)}>
          刷新
        </Button>
      }
    >
      <Form
        form={form}
        layout="inline"
        style={{ marginBottom: 16, rowGap: 8 }}
        onFinish={() => void load(1, pageSize)}
      >
        <Form.Item name="status">
          <Select
            placeholder="状态"
            allowClear
            style={{ width: 120 }}
            options={[
              { value: 'firing', label: 'firing' },
              { value: 'resolved', label: 'resolved' },
            ]}
          />
        </Form.Item>
        <Form.Item name="severity">
          <Select
            placeholder="级别"
            allowClear
            style={{ width: 120 }}
            options={[
              { value: 'critical', label: 'critical' },
              { value: 'warning', label: 'warning' },
              { value: 'info', label: 'info' },
            ]}
          />
        </Form.Item>
        <Form.Item name="alertname">
          <Input placeholder="告警名称" allowClear style={{ width: 160 }} />
        </Form.Item>
        <Form.Item name="channel_id">
          <Select
            placeholder="渠道"
            allowClear
            style={{ width: 160 }}
            options={channels.map((c) => ({ value: c.id, label: c.name }))}
          />
        </Form.Item>
        <Form.Item name="range">
          <DatePicker.RangePicker showTime />
        </Form.Item>
        <Form.Item>
          <Button type="primary" htmlType="submit" loading={loading}>
            查询
          </Button>
        </Form.Item>
      </Form>

      <Table<Alert>
        rowKey="id"
        loading={loading}
        columns={columns}
        dataSource={data}
        onRow={(row) => ({
          onClick: () => void openDetail(row),
          onKeyDown: (e) => {
            if (e.key === 'Enter') void openDetail(row)
          },
          tabIndex: 0,
          style: { cursor: 'pointer' },
        })}
        pagination={{
          current: page,
          pageSize,
          total,
          showSizeChanger: true,
          showTotal: (t) => `共 ${t} 条`,
          onChange: (p, ps) => void load(p, ps),
        }}
      />

      <Drawer
        title={detail ? `告警详情：${detail.alertname}` : '告警详情'}
        width={860}
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        loading={detailLoading}
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
                  children: (
                    <pre className="json-view">{JSON.stringify(detail.raw_payload, null, 2)}</pre>
                  ),
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
                    />
                  ),
                },
              ]}
            />
          </>
        )}
      </Drawer>
    </Card>
  )
}
