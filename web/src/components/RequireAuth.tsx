import type { ReactNode } from 'react'
import { Navigate, useLocation } from 'react-router-dom'
import { Spin } from 'antd'

import { useAuth } from '../auth/useAuth'
import { getAccessToken } from '../api/client'
import type { Role } from '../api/types'

/**
 * 路由守卫：本地无令牌或会话失效时重定向到 /login
 * 传入 `roles` 时当前用户必须具备其中一个角色，否则跳回 /dashboard
 */
export function RequireAuth({ children, roles }: { children: ReactNode; roles?: Role[] }) {
  const { user, hasRole } = useAuth()
  const location = useLocation()

  if (!getAccessToken()) {
    return <Navigate to="/login" state={{ from: location.pathname }} replace />
  }
  if (user === undefined) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', paddingTop: 120 }}>
        <Spin size="large" />
      </div>
    )
  }
  if (user === null) {
    return <Navigate to="/login" replace />
  }
  if (roles && !hasRole(...roles)) {
    return <Navigate to="/dashboard" replace />
  }
  return <>{children}</>
}
