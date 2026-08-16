import { useEffect, useState } from 'react'
import { Button, Card, Descriptions, Form, Input, Tag, Typography, message } from 'antd'
import { useNavigate } from 'react-router-dom'

import { changePassword, getSystemInfo } from '../api'
import { clearTokens, errorMessage } from '../api/client'
import type { SystemInfo } from '../api/types'
import { formatTime } from '../utils'
import { useAuth } from '../auth/useAuth'

const ROLE_TEXT: Record<string, string> = {
  viewer: 'Viewer（只读）',
  editor: 'Editor（运维）',
  admin: 'Admin（管理员）',
}

export default function SettingsPage() {
  const { user, isAdmin } = useAuth()
  const navigate = useNavigate()
  const [sysInfo, setSysInfo] = useState<SystemInfo | null>(null)
  const [saving, setSaving] = useState(false)
  const [form] = Form.useForm<{ old_password: string; new_password: string; confirm: string }>()

  useEffect(() => {
    getSystemInfo().then(setSysInfo).catch(() => undefined)
  }, [])

  const onChangePassword = async (v: { old_password: string; new_password: string }) => {
    setSaving(true)
    try {
      await changePassword(v.old_password, v.new_password)
      message.success('密码已修改，请重新登录')
      clearTokens()
      navigate('/login', { replace: true })
    } catch (err) {
      message.error(errorMessage(err, '修改密码失败'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div style={{ maxWidth: 720 }}>
      <Card title="个人信息" style={{ marginBottom: 16 }}>
        <Descriptions column={1} size="small">
          <Descriptions.Item label="用户名">{user?.username}</Descriptions.Item>
          <Descriptions.Item label="姓名">{user?.name || '-'}</Descriptions.Item>
          <Descriptions.Item label="邮箱">{user?.email || '-'}</Descriptions.Item>
          {isAdmin && (
            <Descriptions.Item label="角色">
              <Tag color={user?.role === 'admin' ? 'gold' : user?.role === 'editor' ? 'blue' : 'default'}>
                {user ? ROLE_TEXT[user.role] : '-'}
              </Tag>
            </Descriptions.Item>
          )}
          <Descriptions.Item label="最近登录">{formatTime(user?.last_login_at)}</Descriptions.Item>
          <Descriptions.Item label="账号创建">{formatTime(user?.created_at)}</Descriptions.Item>
        </Descriptions>
      </Card>

      <Card title="修改密码" style={{ marginBottom: 16 }}>
        <Typography.Paragraph type="secondary" style={{ fontSize: 12 }}>
          修改成功后所有已登录会话都会失效，需要重新登录。
        </Typography.Paragraph>
        <Form
          form={form}
          layout="vertical"
          style={{ maxWidth: 360 }}
          onFinish={(v) => void onChangePassword(v)}
        >
          <Form.Item
            name="old_password"
            label="当前密码"
            rules={[{ required: true, message: '请输入当前密码' }]}
          >
            <Input.Password autoComplete="current-password" />
          </Form.Item>
          <Form.Item
            name="new_password"
            label="新密码"
            rules={[
              { required: true, message: '请输入新密码' },
              { min: 8, message: '新密码至少 8 个字符' },
            ]}
          >
            <Input.Password autoComplete="new-password" />
          </Form.Item>
          <Form.Item
            name="confirm"
            label="确认新密码"
            dependencies={['new_password']}
            rules={[
              { required: true, message: '请再次输入新密码' },
              ({ getFieldValue }) => ({
                validator: (_, value) =>
                  !value || getFieldValue('new_password') === value
                    ? Promise.resolve()
                    : Promise.reject(new Error('两次输入的密码不一致')),
              }),
            ]}
          >
            <Input.Password autoComplete="new-password" />
          </Form.Item>
          <Form.Item style={{ marginBottom: 0 }}>
            <Button type="primary" htmlType="submit" loading={saving}>
              修改密码
            </Button>
          </Form.Item>
        </Form>
      </Card>

      {sysInfo && (
        <Card title="系统信息" size="small">
          <Descriptions column={1} size="small">
            <Descriptions.Item label="版本">{sysInfo.version}</Descriptions.Item>
            <Descriptions.Item label="Root URL">{sysInfo.root_url || '-'}</Descriptions.Item>
            <Descriptions.Item label="飞书 OAuth 登录">
              {sysInfo.oauth_enabled ? '已启用' : '未启用'}
            </Descriptions.Item>
          </Descriptions>
        </Card>
      )}
    </div>
  )
}
