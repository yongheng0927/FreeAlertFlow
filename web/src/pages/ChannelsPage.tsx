import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Button,
  Card,
  Checkbox,
  Form,
  Input,
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
import { PlusOutlined, ReloadOutlined, SendOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'

import {
  createChannel,
  deleteChannel,
  listChannels,
  listTemplates,
  testChannel,
  updateChannel,
  type ChannelPatch,
} from '../api'
import { errorMessage } from '../api/client'
import type { Channel, Template } from '../api/types'
import { formatTime } from '../utils'
import { useAuth } from '../auth/useAuth'

interface ChannelFormValues {
  name: string
  webhook_url?: string
  secret?: string
  clear_secret?: boolean
  keyword?: string
  template_id?: number
  at_all: boolean
  enabled: boolean
}

export default function ChannelsPage() {
  const { canWrite } = useAuth()
  const [data, setData] = useState<Channel[]>([])
  const [templates, setTemplates] = useState<Template[]>([])
  const [loading, setLoading] = useState(false)
  const [testingId, setTestingId] = useState<number | null>(null)

  const [editOpen, setEditOpen] = useState(false)
  const [editing, setEditing] = useState<Channel | null>(null)
  const [saving, setSaving] = useState(false)
  const [form] = Form.useForm<ChannelFormValues>()

  const templateName = useMemo(() => {
    const m = new Map(templates.map((t) => [t.id, t.name]))
    return (id: number | null) => (id ? (m.get(id) ?? `#${id}`) : '默认')
  }, [templates])

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const [chs, tmpls] = await Promise.all([listChannels(), listTemplates()])
      setData(chs.list)
      setTemplates(tmpls.list)
    } catch (err) {
      message.error(errorMessage(err, '加载渠道失败'))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  const openCreate = () => {
    setEditing(null)
    form.resetFields()
    setEditOpen(true)
  }

  const openEdit = (c: Channel) => {
    setEditing(c)
    form.setFieldsValue({
      name: c.name,
      webhook_url: '',
      secret: '',
      clear_secret: false,
      keyword: c.keyword,
      template_id: c.template_id ?? undefined,
      at_all: c.at_all,
      enabled: c.enabled,
    })
    setEditOpen(true)
  }

  const onSave = async () => {
    const v = await form.validateFields()
    setSaving(true)
    try {
      if (editing) {
        const patch: ChannelPatch = {
          name: v.name,
          keyword: v.keyword ?? '',
          at_all: v.at_all,
          enabled: v.enabled,
        }
        // webhook_url：留空表示不修改
        if (v.webhook_url) patch.webhook_url = v.webhook_url
        // secret：填写 = 替换；勾选清除 = 显式传空串
        if (v.clear_secret) patch.secret = ''
        else if (v.secret) patch.secret = v.secret
        // template_id：清空 = 0（解绑）
        patch.template_id = v.template_id ?? 0
        await updateChannel(editing.id, patch)
        message.success('已保存')
      } else {
        await createChannel({
          name: v.name,
          webhook_url: v.webhook_url ?? '',
          secret: v.secret || undefined,
          keyword: v.keyword || undefined,
          template_id: v.template_id,
          at_all: v.at_all,
          enabled: v.enabled,
        })
        message.success('渠道已创建')
      }
      setEditOpen(false)
      void load()
    } catch (err) {
      message.error(errorMessage(err, '保存失败'), 6)
    } finally {
      setSaving(false)
    }
  }

  const onDelete = async (c: Channel) => {
    try {
      await deleteChannel(c.id)
      message.success('已删除')
      void load()
    } catch (err) {
      message.error(errorMessage(err, '删除失败'), 6)
    }
  }

  const onTest = async (c: Channel) => {
    setTestingId(c.id)
    try {
      const r = await testChannel(c.id)
      if (r.success) {
        message.success(`测试消息发送成功（飞书 code=${r.code}，耗时 ${r.duration_ms} ms）`)
      } else {
        Modal.warning({
          title: '测试消息发送失败',
          content: (
            <div>
              <p>HTTP 状态：{r.http_status}</p>
              <p>
                飞书返回：code={r.code}，msg={r.msg || '-'}
              </p>
              <p>耗时：{r.duration_ms} ms</p>
            </div>
          ),
        })
      }
    } catch (err) {
      message.error(errorMessage(err, '测试发送失败'))
    } finally {
      setTestingId(null)
    }
  }

  const onToggleEnabled = async (c: Channel, enabled: boolean) => {
    try {
      await updateChannel(c.id, { enabled })
      void load()
    } catch (err) {
      message.error(errorMessage(err, '操作失败'))
    }
  }

  const columns: ColumnsType<Channel> = [
    { title: '名称', dataIndex: 'name', width: 150 },
    {
      title: 'Webhook URL',
      dataIndex: 'webhook_url',
      ellipsis: true,
      render: (v: string) => <Typography.Text code style={{ fontSize: 12 }}>{v}</Typography.Text>,
    },
    {
      title: '签名',
      dataIndex: 'has_secret',
      width: 70,
      render: (v: boolean) => (v ? <Tag color="blue">已加签</Tag> : <Tag>未加签</Tag>),
    },
    { title: '关键词', dataIndex: 'keyword', width: 100, render: (v: string) => v || '-' },
    {
      title: '模板',
      dataIndex: 'template_id',
      width: 130,
      render: (id: number | null) => templateName(id),
    },
    {
      title: '@所有人',
      dataIndex: 'at_all',
      width: 90,
      render: (v: boolean) => (v ? '是' : '否'),
    },
    {
      title: '启用',
      dataIndex: 'enabled',
      width: 80,
      render: (enabled: boolean, c) =>
        canWrite ? (
          <Switch size="small" checked={enabled} onChange={(v) => void onToggleEnabled(c, v)} />
        ) : (
          <Tag color={enabled ? 'green' : 'default'}>{enabled ? '启用' : '停用'}</Tag>
        ),
    },
    { title: '更新时间', dataIndex: 'updated_at', width: 175, render: formatTime },
    ...(canWrite
      ? [
          {
            title: '操作',
            width: 240,
            render: (_: unknown, c: Channel) => (
              <Space size={0} wrap>
                <Button
                  size="small"
                  type="link"
                  icon={<SendOutlined />}
                  loading={testingId === c.id}
                  onClick={() => void onTest(c)}
                >
                  测试
                </Button>
                <Button size="small" type="link" onClick={() => openEdit(c)}>
                  编辑
                </Button>
                <Popconfirm
                  title="删除渠道"
                  description="被路由规则引用的渠道无法删除。确认删除？"
                  okText="删除"
                  okButtonProps={{ danger: true }}
                  cancelText="取消"
                  onConfirm={() => void onDelete(c)}
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
      title="通知渠道"
      extra={
        <Space>
          <Button icon={<ReloadOutlined />} onClick={() => void load()}>
            刷新
          </Button>
          {canWrite && (
            <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
              新建渠道
            </Button>
          )}
        </Space>
      }
    >
      <Table<Channel>
        rowKey="id"
        loading={loading}
        columns={columns}
        dataSource={data}
        pagination={false}
      />

      <Modal
        title={editing ? '编辑渠道' : '新建渠道'}
        open={editOpen}
        onOk={() => void onSave()}
        onCancel={() => setEditOpen(false)}
        confirmLoading={saving}
        okText="保存"
        cancelText="取消"
        destroyOnClose
      >
        <Form form={form} layout="vertical" initialValues={{ at_all: false, enabled: true }}>
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '请输入名称' }]}>
            <Input placeholder="如：值班群告警机器人" maxLength={64} />
          </Form.Item>
          <Form.Item
            name="webhook_url"
            label="飞书机器人 Webhook URL"
            tooltip={editing ? '列表中显示的是脱敏值；留空表示不修改' : undefined}
            rules={
              editing
                ? []
                : [{ required: true, message: '请输入 Webhook URL' }]
            }
            extra={editing ? `当前：${editing.webhook_url}` : undefined}
          >
            <Input
              placeholder={
                editing ? '留空表示不修改' : 'https://open.feishu.cn/open-apis/bot/v2/hook/xxx'
              }
            />
          </Form.Item>
          <Form.Item
            name="secret"
            label="签名 Secret"
            tooltip="飞书机器人「签名校验」开启时必填"
          >
            <Input.Password
              placeholder={editing ? (editing.has_secret ? '已配置，留空表示不修改' : '未配置') : '可选'}
              autoComplete="new-password"
            />
          </Form.Item>
          {editing?.has_secret && (
            <Form.Item name="clear_secret" valuePropName="checked" style={{ marginTop: -12 }}>
              <Checkbox>清除已配置的签名 Secret</Checkbox>
            </Form.Item>
          )}
          <Form.Item
            name="keyword"
            label="关键词"
            tooltip="飞书机器人「自定义关键词」开启时，消息需包含该关键词"
          >
            <Input placeholder="可选" maxLength={64} />
          </Form.Item>
          <Form.Item name="template_id" label="绑定模板" tooltip="不绑定则使用内置默认模板">
            <Select
              allowClear
              placeholder="默认模板"
              options={templates.map((t) => ({
                value: t.id,
                label: t.is_builtin ? `${t.name}（内置）` : t.name,
              }))}
            />
          </Form.Item>
          <Form.Item name="at_all" valuePropName="checked">
            <Checkbox>消息 @所有人</Checkbox>
          </Form.Item>
          <Form.Item name="enabled" label="启用" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  )
}
