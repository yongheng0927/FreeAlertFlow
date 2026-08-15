import { useCallback, useEffect, useState } from 'react'
import {
  Button,
  Card,
  DatePicker,
  Form,
  Popconfirm,
  Select,
  Table,
  Tooltip,
  Typography,
  message,
} from 'antd'
import { RedoOutlined, ReloadOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import type { Dayjs } from 'dayjs'

import { listChannels, listDeliveries, resendDelivery } from '../api'
import { errorMessage } from '../api/client'
import type { Channel, Delivery } from '../api/types'
import { DeliveryStatusTag } from '../components/tags'
import { formatTime } from '../utils'

interface FilterValues {
  channel_id?: number
  range?: [Dayjs | null, Dayjs | null] | null
}

export default function DeliveriesPage() {
  const [form] = Form.useForm<FilterValues>()
  const [data, setData] = useState<Delivery[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [loading, setLoading] = useState(false)
  const [resendingId, setResendingId] = useState<number | null>(null)
  const [channels, setChannels] = useState<Channel[]>([])

  useEffect(() => {
    listChannels().then((r) => setChannels(r.list)).catch(() => undefined)
  }, [])

  const load = useCallback(
    async (p: number, ps: number) => {
      setLoading(true)
      const v = form.getFieldsValue()
      const [start, end] = v.range ?? [null, null]
      try {
        const r = await listDeliveries({
          status: 'failed',
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
        message.error(errorMessage(err, '加载失败投递列表失败'))
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

  const onResend = async (row: Delivery) => {
    setResendingId(row.id)
    try {
      const r = await resendDelivery(row.id)
      if (r.success) {
        message.success(`重发成功（渠道：${r.delivery.channel_name}，耗时 ${r.delivery.duration_ms} ms）`)
      } else {
        message.error(
          `重发仍失败：${r.delivery.response_msg || `HTTP ${r.delivery.http_status}`}`,
          6,
        )
      }
      void load(page, pageSize)
    } catch (err) {
      message.error(errorMessage(err, '重发失败'))
    } finally {
      setResendingId(null)
    }
  }

  const columns: ColumnsType<Delivery> = [
    { title: 'ID', dataIndex: 'id', width: 70 },
    { title: '渠道', dataIndex: 'channel_name', width: 140 },
    { title: '告警 ID', dataIndex: 'alert_id', width: 90 },
    {
      title: '状态',
      dataIndex: 'status',
      width: 80,
      render: (s: string) => <DeliveryStatusTag status={s} />,
    },
    {
      title: '失败原因',
      render: (_, d) => (
        <Tooltip title={d.response_msg}>
          <Typography.Text type="danger" ellipsis style={{ maxWidth: 320 }}>
            {d.response_msg || `HTTP ${d.http_status}`}
          </Typography.Text>
        </Tooltip>
      ),
    },
    { title: '尝试', dataIndex: 'attempts', width: 70 },
    { title: '耗时', dataIndex: 'duration_ms', width: 90, render: (v: number) => `${v} ms` },
    { title: '发送时间', dataIndex: 'sent_at', width: 175, render: formatTime },
    {
      title: '操作',
      width: 110,
      render: (_, row) => (
        <Popconfirm
          title="手动重发"
          description="将按当前渠道配置与模板重新渲染发送，并生成一条新的投递记录。"
          okText="重发"
          cancelText="取消"
          onConfirm={() => void onResend(row)}
        >
          <Button
            size="small"
            type="link"
            icon={<RedoOutlined />}
            loading={resendingId === row.id}
          >
            重发
          </Button>
        </Popconfirm>
      ),
    },
  ]

  return (
    <Card
      title="失败投递"
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
        <Form.Item name="channel_id">
          <Select
            placeholder="渠道"
            allowClear
            style={{ width: 180 }}
            options={channels.map((c) => ({ value: c.id, label: c.name }))}
          />
        </Form.Item>
        <Form.Item name="range">
          <DatePicker.RangePicker showTime />
        </Form.Item>
        <Form.Item>
          <Button type="primary" htmlType="submit">
            查询
          </Button>
        </Form.Item>
      </Form>

      <Table<Delivery>
        rowKey="id"
        loading={loading}
        columns={columns}
        dataSource={data}
        expandable={{
          expandedRowRender: (d) => {
            const rp = d.rendered_payload
            if (!rp) return <Typography.Text type="secondary">无渲染内容</Typography.Text>
            let pretty = rp
            try {
              pretty = JSON.stringify(JSON.parse(rp), null, 2)
            } catch {
              // 不是合法 JSON 时保留原文直出
            }
            return <pre className="json-view">{pretty}</pre>
          },
        }}
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
