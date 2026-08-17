import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button, Card, DatePicker, Form, Input, Select, Table, Tooltip, Typography, message } from 'antd'
import { ReloadOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import type { Dayjs } from 'dayjs'

import { listAlerts, listChannels, listSources } from '../api'
import { errorMessage } from '../api/client'
import type { Alert, Channel, Source } from '../api/types'
import { AlertStatusTag, DispositionTag, SeverityTag } from '../components/tags'
import { formatTime } from '../utils'

interface FilterValues {
  status?: string
  severity?: string
  alertname?: string
  channel_id?: number
  range?: [Dayjs | null, Dayjs | null] | null
}

export default function AlertsPage() {
  const [form] = Form.useForm<FilterValues>()
  const navigate = useNavigate()
  const [data, setData] = useState<Alert[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [loading, setLoading] = useState(false)
  const [sources, setSources] = useState<Source[]>([])
  const [channels, setChannels] = useState<Channel[]>([])

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
          onClick: () => navigate(`/alerts/${row.id}`),
          onKeyDown: (e) => {
            if (e.key === 'Enter') navigate(`/alerts/${row.id}`)
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
    </Card>
  )
}
