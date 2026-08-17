import { useCallback, useEffect, useState } from 'react'
import {
  Avatar,
  Button,
  Card,
  Form,
  Input,
  Modal,
  Popconfirm,
  Select,
  Space,
  Switch,
  Table,
  Tag,
  message,
} from 'antd'
import { PlusOutlined, ReloadOutlined, UserOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'

import { createUser, deleteUser, listUsers, resetUserPassword, updateUser } from '../api'
import { errorMessage } from '../api/client'
import type { Role, User } from '../api/types'
import { formatTime } from '../utils'
import { useAuth } from '../auth/useAuth'

const ROLE_TAG: Record<string, { text: string; color: string }> = {
  viewer: { text: 'Viewer', color: 'default' },
  editor: { text: 'Editor', color: 'blue' },
  admin: { text: 'Admin', color: 'gold' },
}

export default function UsersPage() {
  const { user: me } = useAuth()
  const [data, setData] = useState<User[]>([])
  const [loading, setLoading] = useState(false)
  const [togglingId, setTogglingId] = useState<number | null>(null)
  const [deletingId, setDeletingId] = useState<number | null>(null)

  const [roleModalUser, setRoleModalUser] = useState<User | null>(null)
  const [roleValue, setRoleValue] = useState<Role>('viewer')
  const [saving, setSaving] = useState(false)
  const [createOpen, setCreateOpen] = useState(false)
  const [creating, setCreating] = useState(false)
  const [createForm] = Form.useForm<{ username: string; password: string; role: Role }>()
  const [resetUser, setResetUser] = useState<User | null>(null)
  const [resetting, setResetting] = useState(false)
  const [resetForm] = Form.useForm<{ password: string }>()

  const onResetPassword = async () => {
    if (!resetUser) return
    const values = await resetForm.validateFields()
    setResetting(true)
    try {
      await resetUserPassword(resetUser.id, values.password)
      message.success(`已重置 ${resetUser.username} 的密码，其所有会话已强制下线`)
      setResetUser(null)
      resetForm.resetFields()
    } catch (err) {
      message.error(errorMessage(err, '重置密码失败'), 6)
    } finally {
      setResetting(false)
    }
  }

  const onCreate = async () => {
    const values = await createForm.validateFields()
    setCreating(true)
    try {
      await createUser(values)
      message.success(`已创建用户 ${values.username}`)
      setCreateOpen(false)
      createForm.resetFields()
      void load()
    } catch (err) {
      message.error(errorMessage(err, '创建用户失败'), 6)
    } finally {
      setCreating(false)
    }
  }

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const r = await listUsers()
      setData(r.list)
    } catch (err) {
      message.error(errorMessage(err, '加载用户列表失败'))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  const onChangeRole = async () => {
    if (!roleModalUser) return
    setSaving(true)
    try {
      await updateUser(roleModalUser.id, { role: roleValue })
      message.success('角色已更新')
      setRoleModalUser(null)
      void load()
    } catch (err) {
      message.error(errorMessage(err, '修改角色失败'), 6)
    } finally {
      setSaving(false)
    }
  }

  const onToggleEnabled = async (u: User, enabled: boolean) => {
    if (togglingId !== null) return
    setTogglingId(u.id)
    try {
      await updateUser(u.id, { enabled })
      message.success(enabled ? `已启用 ${u.username}` : `已禁用 ${u.username}`)
      void load()
    } catch (err) {
      message.error(errorMessage(err, '操作失败'), 6)
    } finally {
      setTogglingId(null)
    }
  }

  const onDelete = async (u: User) => {
    if (deletingId !== null) return
    setDeletingId(u.id)
    try {
      await deleteUser(u.id)
      message.success('已删除')
      void load()
    } catch (err) {
      message.error(errorMessage(err, '删除失败'), 6)
    } finally {
      setDeletingId(null)
    }
  }

  const columns: ColumnsType<User> = [
    {
      title: '名称',
      width: 220,
      render: (_, u) => (
        <Space>
          <Avatar size="small" src={u.avatar_url || undefined} icon={<UserOutlined />} />
          <span>
            {u.name || u.username}
            {me?.id === u.id && <Tag style={{ marginLeft: 6 }}>我</Tag>}
            {u.is_initial && <Tag color="gold">初始</Tag>}
          </span>
        </Space>
      ),
    },
    { title: '用户名', dataIndex: 'username', width: 140 },
    {
      title: '角色',
      dataIndex: 'role',
      width: 100,
      render: (r: string) => {
        const t = ROLE_TAG[r]
        return t ? <Tag color={t.color}>{t.text}</Tag> : <Tag>{r}</Tag>
      },
    },
    {
      title: '启用',
      dataIndex: 'enabled',
      width: 80,
      render: (enabled: boolean, u) => (
        <Switch
          size="small"
          checked={enabled}
          disabled={me?.id === u.id || u.is_initial}
          loading={togglingId === u.id}
          onChange={(v) => void onToggleEnabled(u, v)}
        />
      ),
    },
    { title: '最近登录', dataIndex: 'last_login_at', width: 175, render: formatTime },
    { title: '创建时间', dataIndex: 'created_at', width: 175, render: formatTime },
    {
      title: '操作',
      width: 160,
      render: (_, u) => (
        <Space size={0}>
          <Button
            size="small"
            type="link"
            disabled={u.is_initial}
            onClick={() => {
              setRoleModalUser(u)
              setRoleValue(u.role)
            }}
          >
            改角色
          </Button>
          {me?.is_initial && !u.is_initial && u.has_password && (
            <Button
              size="small"
              type="link"
              onClick={() => {
                setResetUser(u)
                resetForm.resetFields()
              }}
            >
              重置密码
            </Button>
          )}
          <Popconfirm
            title="删除用户"
            description={`确认删除用户 ${u.username}？该操作不可恢复。`}
            okText="删除"
            okButtonProps={{ danger: true, loading: deletingId === u.id }}
            cancelText="取消"
            onConfirm={() => void onDelete(u)}
          >
            <Button size="small" type="link" danger disabled={me?.id === u.id || u.is_initial}>
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <Card
      title="用户管理"
      extra={
        <Space>
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
            新建用户
          </Button>
          <Button icon={<ReloadOutlined />} onClick={() => void load()}>
            刷新
          </Button>
        </Space>
      }
    >
      <Table<User>
        rowKey="id"
        loading={loading}
        columns={columns}
        dataSource={data}
        pagination={false}
      />

      <Modal
        title="新建用户"
        open={createOpen}
        onOk={() => void onCreate()}
        onCancel={() => setCreateOpen(false)}
        confirmLoading={creating}
        okText="创建"
        cancelText="取消"
        destroyOnHidden
      >
        <Form form={createForm} layout="vertical" initialValues={{ role: 'viewer' }}>
          <Form.Item
            name="username"
            label="用户名"
            rules={[
              { required: true, message: '请输入用户名' },
              { max: 64, message: '用户名最长 64 个字符' },
            ]}
          >
            <Input placeholder="登录用户名" autoComplete="off" />
          </Form.Item>
          <Form.Item
            name="password"
            label="初始密码"
            rules={[
              { required: true, message: '请输入初始密码' },
              { min: 8, message: '密码至少 8 位' },
            ]}
          >
            <Input.Password placeholder="至少 8 位，用户登录后可自行修改" autoComplete="new-password" />
          </Form.Item>
          <Form.Item name="role" label="角色">
            <Select
              options={[
                { value: 'viewer', label: 'Viewer' },
                { value: 'editor', label: 'Editor' },
                { value: 'admin', label: 'Admin' },
              ]}
            />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={resetUser ? `重置密码：${resetUser.username}` : ''}
        open={resetUser !== null}
        onOk={() => void onResetPassword()}
        onCancel={() => setResetUser(null)}
        confirmLoading={resetting}
        okText="重置"
        cancelText="取消"
        destroyOnHidden
      >
        <Form form={resetForm} layout="vertical">
          <Form.Item
            name="password"
            label="新密码"
            rules={[
              { required: true, message: '请输入新密码' },
              { min: 8, message: '密码至少 8 位' },
            ]}
          >
            <Input.Password placeholder="至少 8 位，重置后该用户所有会话失效" autoComplete="new-password" />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={roleModalUser ? `修改角色：${roleModalUser.username}` : ''}
        open={roleModalUser !== null}
        onOk={() => void onChangeRole()}
        onCancel={() => setRoleModalUser(null)}
        confirmLoading={saving}
        okText="保存"
        cancelText="取消"
      >
        <Select
          style={{ width: '100%', marginTop: 8 }}
          value={roleValue}
          onChange={(v) => setRoleValue(v)}
          options={[
            { value: 'viewer', label: 'Viewer' },
            { value: 'editor', label: 'Editor' },
            { value: 'admin', label: 'Admin' },
          ]}
        />
      </Modal>
    </Card>
  )
}
