import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Button,
  Card,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Select,
  Space,
  Switch,
  Table,
  Tag,
  Typography,
  message,
} from 'antd'
import { MinusCircleOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'

import { createRule, deleteRule, listChannels, listRules, listSources, updateRule } from '../api'
import { errorMessage } from '../api/client'
import type { Channel, RoutingRule, Source } from '../api/types'
import { useAuth } from '../auth/useAuth'

interface RuleFormValues {
  source_id: number
  name?: string
  priority: number
  pairs: { key: string; value: string }[]
  channel_id: number
  continue_matching: boolean
  enabled: boolean
}

export default function RulesPage() {
  const { canWrite } = useAuth()
  const [data, setData] = useState<RoutingRule[]>([])
  const [sources, setSources] = useState<Source[]>([])
  const [channels, setChannels] = useState<Channel[]>([])
  const [loading, setLoading] = useState(false)
  const [togglingId, setTogglingId] = useState<number | null>(null)
  const [deletingId, setDeletingId] = useState<number | null>(null)
  const [sourceFilter, setSourceFilter] = useState<number | undefined>(undefined)

  const [editOpen, setEditOpen] = useState(false)
  const [editing, setEditing] = useState<RoutingRule | null>(null)
  const [saving, setSaving] = useState(false)
  const [form] = Form.useForm<RuleFormValues>()

  const sourceName = useMemo(() => {
    const m = new Map(sources.map((s) => [s.id, s.name]))
    return (id: number) => m.get(id) ?? `#${id}`
  }, [sources])

  const channelName = useMemo(() => {
    const m = new Map(channels.map((c) => [c.id, c.name]))
    return (id: number) => m.get(id) ?? `#${id}`
  }, [channels])

  const load = useCallback(async (sourceId?: number) => {
    setLoading(true)
    try {
      const [rules, srcs, chs] = await Promise.all([
        listRules(sourceId),
        listSources(),
        listChannels(),
      ])
      setData(rules.list.sort((a, b) => a.priority - b.priority))
      setSources(srcs.list)
      setChannels(chs.list)
    } catch (err) {
      message.error(errorMessage(err, '加载路由规则失败'))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load(sourceFilter)
  }, [load, sourceFilter])

  const openCreate = () => {
    setEditing(null)
    form.resetFields()
    setEditOpen(true)
  }

  const openEdit = (r: RoutingRule) => {
    setEditing(r)
    form.setFieldsValue({
      source_id: r.source_id,
      name: r.name,
      priority: r.priority,
      pairs: Object.entries(r.match_labels ?? {}).map(([key, value]) => ({ key, value })),
      channel_id: r.channel_id,
      continue_matching: r.continue_matching,
      enabled: r.enabled,
    })
    setEditOpen(true)
  }

  const onSave = async () => {
    const v = await form.validateFields()
    const matchLabels: Record<string, string> = {}
    for (const p of v.pairs ?? []) {
      if (p.key) matchLabels[p.key] = p.value ?? ''
    }
    const input = {
      source_id: v.source_id,
      name: v.name ?? '',
      priority: v.priority ?? 0,
      match_labels: matchLabels,
      channel_id: v.channel_id,
      continue_matching: v.continue_matching,
      enabled: v.enabled,
    }
    setSaving(true)
    try {
      if (editing) {
        await updateRule(editing.id, input)
        message.success('已保存')
      } else {
        await createRule(input)
        message.success('规则已创建')
      }
      setEditOpen(false)
      void load(sourceFilter)
    } catch (err) {
      message.error(errorMessage(err, '保存失败'), 6)
    } finally {
      setSaving(false)
    }
  }

  const onDelete = async (r: RoutingRule) => {
    if (deletingId !== null) return
    setDeletingId(r.id)
    try {
      await deleteRule(r.id)
      message.success('已删除')
      void load(sourceFilter)
    } catch (err) {
      message.error(errorMessage(err, '删除失败'))
    } finally {
      setDeletingId(null)
    }
  }

  const onToggleEnabled = async (r: RoutingRule, enabled: boolean) => {
    if (togglingId !== null) return
    setTogglingId(r.id)
    try {
      await updateRule(r.id, {
        source_id: r.source_id,
        name: r.name,
        priority: r.priority,
        match_labels: r.match_labels ?? {},
        channel_id: r.channel_id,
        continue_matching: r.continue_matching,
        enabled,
      })
      void load(sourceFilter)
    } catch (err) {
      message.error(errorMessage(err, '操作失败'))
    } finally {
      setTogglingId(null)
    }
  }

  const columns: ColumnsType<RoutingRule> = [
    { title: '优先级', dataIndex: 'priority', width: 80 },
    { title: '规则名', dataIndex: 'name', width: 150, render: (v: string) => v || '-' },
    { title: '接入源', dataIndex: 'source_id', width: 140, render: (id: number) => sourceName(id) },
    {
      title: '匹配 Labels',
      dataIndex: 'match_labels',
      render: (labels: Record<string, string>) => {
        const entries = Object.entries(labels ?? {})
        if (entries.length === 0) return <Tag color="blue">默认规则（匹配全部）</Tag>
        return (
          <Space size={4} wrap>
            {entries.map(([k, v]) => (
              <Tag key={k}>
                {k}={v}
              </Tag>
            ))}
          </Space>
        )
      },
    },
    {
      title: '目标渠道',
      dataIndex: 'channel_id',
      width: 150,
      render: (id: number) => channelName(id),
    },
    {
      title: '继续匹配',
      dataIndex: 'continue_matching',
      width: 90,
      render: (v: boolean) => (v ? '是' : '否'),
    },
    {
      title: '启用',
      dataIndex: 'enabled',
      width: 80,
      render: (enabled: boolean, r) =>
        canWrite ? (
          <Switch
            size="small"
            checked={enabled}
            loading={togglingId === r.id}
            onChange={(v) => void onToggleEnabled(r, v)}
          />
        ) : (
          <Tag color={enabled ? 'green' : 'default'}>{enabled ? '启用' : '停用'}</Tag>
        ),
    },
    ...(canWrite
      ? [
          {
            title: '操作',
            width: 150,
            render: (_: unknown, r: RoutingRule) => (
              <Space size={0}>
                <Button size="small" type="link" onClick={() => openEdit(r)}>
                  编辑
                </Button>
                <Popconfirm
                  title="删除规则"
                  description="确认删除该路由规则？"
                  okText="删除"
                  okButtonProps={{ danger: true, loading: deletingId === r.id }}
                  cancelText="取消"
                  onConfirm={() => void onDelete(r)}
                >
                  <Button size="small" type="link" danger>
                    删除
                  </Button>
                </Popconfirm>
              </Space>
            ),
          },
        ]
      : []),
  ]

  return (
    <Card
      title="路由规则"
      extra={
        <Space>
          <Select
            placeholder="按接入源筛选"
            allowClear
            style={{ width: 180 }}
            value={sourceFilter}
            options={sources.map((s) => ({ value: s.id, label: s.name }))}
            onChange={(v) => setSourceFilter(v)}
          />
          <Button icon={<ReloadOutlined />} onClick={() => void load(sourceFilter)}>
            刷新
          </Button>
          {canWrite && (
            <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
              新建规则
            </Button>
          )}
        </Space>
      }
    >
      <Typography.Paragraph type="secondary" style={{ fontSize: 12 }}>
        规则按优先级从小到大依次匹配；命中即投递，勾选「继续匹配」则继续尝试后续规则。
        匹配 Labels 为空的规则是兜底默认规则。
      </Typography.Paragraph>

      <Table<RoutingRule>
        rowKey="id"
        loading={loading}
        columns={columns}
        dataSource={data}
        pagination={false}
      />

      <Modal
        title={editing ? '编辑路由规则' : '新建路由规则'}
        open={editOpen}
        onOk={() => void onSave()}
        onCancel={() => setEditOpen(false)}
        confirmLoading={saving}
        okText="保存"
        cancelText="取消"
        width={640}
        destroyOnClose
      >
        <Form
          form={form}
          layout="vertical"
          initialValues={{ priority: 0, pairs: [], continue_matching: false, enabled: true }}
        >
          <Form.Item
            name="source_id"
            label="接入源"
            rules={[{ required: true, message: '请选择接入源' }]}
          >
            <Select
              placeholder="选择接入源"
              options={sources.map((s) => ({ value: s.id, label: s.name }))}
            />
          </Form.Item>
          <Form.Item name="name" label="规则名">
            <Input placeholder="可选，如：critical 告警到值班群" maxLength={64} />
          </Form.Item>
          <Form.Item
            name="priority"
            label="优先级（数字越小越先匹配；填 0 使用默认 100）"
          >
            <InputNumber min={0} max={100000} style={{ width: 200 }} />
          </Form.Item>
          <Form.Item label="匹配 Labels（全部满足才命中；留空 = 默认规则）">
            <Form.List name="pairs">
              {(fields, { add, remove }) => (
                <>
                  {fields.map((field) => (
                    <Space key={field.key} style={{ display: 'flex', marginBottom: 8 }} align="baseline">
                      <Form.Item
                        name={[field.name, 'key']}
                        rules={[{ required: true, message: 'label 名' }]}
                        noStyle
                      >
                        <Input placeholder="label 名，如 severity" style={{ width: 200 }} />
                      </Form.Item>
                      <span>=</span>
                      <Form.Item
                        name={[field.name, 'value']}
                        rules={[{ required: true, message: 'label 值' }]}
                        noStyle
                      >
                        <Input placeholder="label 值，如 critical" style={{ width: 200 }} />
                      </Form.Item>
                      <MinusCircleOutlined onClick={() => remove(field.name)} />
                    </Space>
                  ))}
                  <Button type="dashed" size="small" icon={<PlusOutlined />} onClick={() => add({ key: '', value: '' })}>
                    添加匹配条件
                  </Button>
                </>
              )}
            </Form.List>
          </Form.Item>
          <Form.Item
            name="channel_id"
            label="目标渠道"
            rules={[{ required: true, message: '请选择渠道' }]}
          >
            <Select
              placeholder="选择渠道"
              options={channels.map((c) => ({ value: c.id, label: c.name }))}
            />
          </Form.Item>
          <Form.Item name="continue_matching" valuePropName="checked">
            <Switch checkedChildren="继续匹配后续规则" unCheckedChildren="命中后停止" />
          </Form.Item>
          <Form.Item name="enabled" label="启用" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  )
}
