import { useCallback, useEffect, useState } from 'react'
import {
  Button,
  Card,
  Form,
  Input,
  Modal,
  Popconfirm,
  Space,
  Switch,
  Table,
  Tag,
  Typography,
  message,
} from 'antd'
import { CopyOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'

import {
  createSource,
  deleteSource,
  getSystemInfo,
  listSources,
  rotateSourceToken,
  updateSource,
} from '../api'
import { errorMessage } from '../api/client'
import type { Source } from '../api/types'
import { copyText, formatTime, webhookUrl } from '../utils'
import { useAuth } from '../auth/useAuth'

function amYamlExample(url: string): string {
  return `# Alertmanager 配置示例（alertmanager.yml）
receivers:
  - name: 'fenghuo'
    webhook_configs:
      - url: '${url}'
        send_resolved: true

route:
  receiver: 'fenghuo'
  # group_wait / group_interval / repeat_interval 按需调整`
}

export default function SourcesPage() {
  const { canWrite } = useAuth()
  const [data, setData] = useState<Source[]>([])
  const [loading, setLoading] = useState(false)
  const [rootUrl, setRootUrl] = useState('')

  const [editOpen, setEditOpen] = useState(false)
  const [editing, setEditing] = useState<Source | null>(null)
  const [saving, setSaving] = useState(false)
  const [form] = Form.useForm<{ name: string; description?: string; enabled?: boolean }>()

  const [yamlModal, setYamlModal] = useState<Source | null>(null)
  const [createdSource, setCreatedSource] = useState<Source | null>(null)

  useEffect(() => {
    getSystemInfo()
      .then((info) => setRootUrl(info.root_url))
      .catch(() => undefined)
  }, [])

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const r = await listSources()
      setData(r.list)
    } catch (err) {
      message.error(errorMessage(err, '加载接入源失败'))
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

  const openEdit = (s: Source) => {
    setEditing(s)
    form.setFieldsValue({ name: s.name, description: s.description, enabled: s.enabled })
    setEditOpen(true)
  }

  const onSave = async () => {
    const v = await form.validateFields()
    setSaving(true)
    try {
      if (editing) {
        await updateSource(editing.id, {
          name: v.name,
          description: v.description ?? '',
          enabled: v.enabled ?? true,
        })
        message.success('已保存')
      } else {
        const created = await createSource({ name: v.name, description: v.description ?? '' })
        message.success('接入源已创建')
        setCreatedSource(created)
      }
      setEditOpen(false)
      void load()
    } catch (err) {
      message.error(errorMessage(err, '保存失败'))
    } finally {
      setSaving(false)
    }
  }

  const onDelete = async (s: Source) => {
    try {
      await deleteSource(s.id)
      message.success('已删除')
      void load()
    } catch (err) {
      message.error(errorMessage(err, '删除失败'))
    }
  }

  const onRotate = async (s: Source) => {
    try {
      const updated = await rotateSourceToken(s.id)
      message.success('Token 已重置，旧的 Webhook URL 已失效')
      setYamlModal(updated)
      void load()
    } catch (err) {
      message.error(errorMessage(err, '重置失败'))
    }
  }

  const onToggleEnabled = async (s: Source, enabled: boolean) => {
    try {
      await updateSource(s.id, { enabled })
      void load()
    } catch (err) {
      message.error(errorMessage(err, '操作失败'))
    }
  }

  const columns: ColumnsType<Source> = [
    { title: '名称', dataIndex: 'name', width: 160 },
    {
      title: 'Webhook Token',
      dataIndex: 'token',
      width: 300,
      render: (t: string) => (
        <Space size={4}>
          <Typography.Text code style={{ fontSize: 12 }}>
            {t}
          </Typography.Text>
          <Button
            type="text"
            size="small"
            icon={<CopyOutlined />}
            onClick={() => void copyText(webhookUrl(t, rootUrl), 'Webhook URL 已复制')}
          />
        </Space>
      ),
    },
    { title: '描述', dataIndex: 'description', ellipsis: true },
    {
      title: '启用',
      dataIndex: 'enabled',
      width: 80,
      render: (enabled: boolean, s) =>
        canWrite ? (
          <Switch size="small" checked={enabled} onChange={(v) => void onToggleEnabled(s, v)} />
        ) : (
          <Tag color={enabled ? 'green' : 'default'}>{enabled ? '启用' : '停用'}</Tag>
        ),
    },
    { title: '最近告警', dataIndex: 'last_alert_at', width: 175, render: formatTime },
    { title: '创建时间', dataIndex: 'created_at', width: 175, render: formatTime },
    ...(canWrite
      ? [
          {
            title: '操作',
            width: 280,
            render: (_: unknown, s: Source) => (
              <Space size={0} wrap>
                <Button size="small" type="link" onClick={() => setYamlModal(s)}>
                  配置示例
                </Button>
                <Button size="small" type="link" onClick={() => openEdit(s)}>
                  编辑
                </Button>
                <Popconfirm
                  title="重置 Token"
                  description="重置后旧 Webhook URL 立即失效，需同步修改 Alertmanager 配置。确认继续？"
                  okText="重置"
                  okButtonProps={{ danger: true }}
                  cancelText="取消"
                  onConfirm={() => void onRotate(s)}
                >
                  <Button size="small" type="link" danger>
                    重置 Token
                  </Button>
                </Popconfirm>
                <Popconfirm
                  title="删除接入源"
                  description="删除后该 Webhook 立即不可用。确认删除？"
                  okText="删除"
                  okButtonProps={{ danger: true }}
                  cancelText="取消"
                  onConfirm={() => void onDelete(s)}
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
      title="接入源"
      extra={
        <Space>
          <Button icon={<ReloadOutlined />} onClick={() => void load()}>
            刷新
          </Button>
          {canWrite && (
            <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
              新建接入源
            </Button>
          )}
        </Space>
      }
    >
      <Table<Source>
        rowKey="id"
        loading={loading}
        columns={columns}
        dataSource={data}
        pagination={false}
      />

      {/* 创建/编辑 */}
      <Modal
        title={editing ? '编辑接入源' : '新建接入源'}
        open={editOpen}
        onOk={() => void onSave()}
        onCancel={() => setEditOpen(false)}
        confirmLoading={saving}
        okText="保存"
        cancelText="取消"
        destroyOnClose
      >
        <Form form={form} layout="vertical" initialValues={{ enabled: true }}>
          <Form.Item
            name="name"
            label="名称"
            rules={[{ required: true, message: '请输入名称' }]}
          >
            <Input placeholder="如：生产环境 Alertmanager" maxLength={64} />
          </Form.Item>
          <Form.Item name="description" label="描述">
            <Input.TextArea rows={2} placeholder="可选" maxLength={255} />
          </Form.Item>
          {editing && (
            <Form.Item name="enabled" label="启用" valuePropName="checked">
              <Switch />
            </Form.Item>
          )}
        </Form>
      </Modal>

      {/* 创建成功：展示完整 Webhook URL */}
      <Modal
        title="接入源创建成功"
        open={createdSource !== null}
        footer={
          <Button type="primary" onClick={() => setCreatedSource(null)}>
            完成
          </Button>
        }
        onCancel={() => setCreatedSource(null)}
      >
        {createdSource && (
          <>
            <Typography.Paragraph>
              请将以下 Webhook URL 配置到 Alertmanager 的 <Typography.Text code>webhook_configs</Typography.Text>：
            </Typography.Paragraph>
            <Typography.Paragraph copyable={{ text: webhookUrl(createdSource.token, rootUrl) }}>
              <Typography.Text code style={{ wordBreak: 'break-all' }}>
                {webhookUrl(createdSource.token, rootUrl)}
              </Typography.Text>
            </Typography.Paragraph>
            <Typography.Text type="secondary">
              该 URL 中的 token 即访问凭证，请妥善保管；泄露后可在列表中「重置 Token」。
            </Typography.Text>
          </>
        )}
      </Modal>

      {/* Alertmanager 配置示例 */}
      <Modal
        title={yamlModal ? `Alertmanager 配置示例：${yamlModal.name}` : ''}
        open={yamlModal !== null}
        footer={
          <Button
            type="primary"
            icon={<CopyOutlined />}
            onClick={() =>
              yamlModal && void copyText(amYamlExample(webhookUrl(yamlModal.token, rootUrl)), 'YAML 已复制')
            }
          >
            复制 YAML
          </Button>
        }
        onCancel={() => setYamlModal(null)}
        width={640}
      >
        {yamlModal && <pre className="json-view">{amYamlExample(webhookUrl(yamlModal.token, rootUrl))}</pre>}
      </Modal>
    </Card>
  )
}
