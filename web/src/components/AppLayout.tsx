import { Layout, Menu, Dropdown, Avatar, Tag, Typography, message, theme } from 'antd'
import {
  AlertOutlined,
  ApiOutlined,
  DashboardOutlined,
  FileTextOutlined,
  LogoutOutlined,
  NodeIndexOutlined,
  SendOutlined,
  SettingOutlined,
  SwapOutlined,
  TeamOutlined,
  UserOutlined,
} from '@ant-design/icons'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'

import { useAuth } from '../auth/useAuth'

const { Sider, Header, Content } = Layout

const ROLE_LABEL: Record<string, { text: string; color: string }> = {
  viewer: { text: 'Viewer', color: 'default' },
  editor: { text: 'Editor', color: 'blue' },
  admin: { text: 'Admin', color: 'gold' },
}

export default function AppLayout() {
  const { user, canWrite, isAdmin, logout } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const {
    token: { colorBgContainer },
  } = theme.useToken()

  const items = [
    { key: '/dashboard', icon: <DashboardOutlined />, label: '仪表盘' },
    { key: '/alerts', icon: <AlertOutlined />, label: '告警记录' },
    ...(canWrite ? [{ key: '/deliveries', icon: <SendOutlined />, label: '失败投递' }] : []),
    { type: 'divider' as const },
    { key: '/sources', icon: <ApiOutlined />, label: '接入源' },
    { key: '/channels', icon: <NodeIndexOutlined />, label: '通知渠道' },
    { key: '/templates', icon: <FileTextOutlined />, label: '消息模板' },
    { key: '/rules', icon: <SwapOutlined />, label: '路由规则' },
    { type: 'divider' as const },
    { key: '/settings', icon: <SettingOutlined />, label: '系统设置' },
    ...(isAdmin ? [{ key: '/users', icon: <TeamOutlined />, label: '用户管理' }] : []),
  ]

  const selectedKey =
    items
      .filter((it) => 'key' in it)
      .map((it) => (it as { key: string }).key)
      .find((k) => location.pathname.startsWith(k)) ?? '/dashboard'

  const role = user ? ROLE_LABEL[user.role] : undefined

  const onLogout = async () => {
    await logout()
    message.success('已退出登录')
    navigate('/login', { replace: true })
  }

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider theme="dark" width={208}>
        <div
          style={{
            height: 56,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            color: '#fff',
            fontSize: 17,
            fontWeight: 600,
            letterSpacing: 0.5,
          }}
        >
          FreeAlertFlow
        </div>
        <Menu
          theme="dark"
          mode="inline"
          selectedKeys={[selectedKey]}
          items={items}
          onClick={({ key }) => navigate(key)}
        />
      </Sider>
      <Layout>
        <Header
          style={{
            background: colorBgContainer,
            padding: '0 24px',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'flex-end',
            borderBottom: '1px solid #f0f0f0',
          }}
        >
          <Dropdown
            menu={{
              items: [
                { key: 'settings', icon: <SettingOutlined />, label: '系统设置' },
                { type: 'divider' },
                { key: 'logout', icon: <LogoutOutlined />, label: '退出登录', danger: true },
              ],
              onClick: ({ key }) => {
                if (key === 'logout') void onLogout()
                if (key === 'settings') navigate('/settings')
              },
            }}
          >
            <div style={{ cursor: 'pointer', display: 'flex', alignItems: 'center', gap: 8 }}>
              <Avatar src={user?.avatar_url || undefined} icon={<UserOutlined />} size="small" />
              <Typography.Text strong>{user?.name || user?.username}</Typography.Text>
              {role && <Tag color={role.color} style={{ marginInlineEnd: 0 }}>{role.text}</Tag>}
            </div>
          </Dropdown>
        </Header>
        <Content style={{ padding: 24, overflow: 'auto' }}>
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  )
}
