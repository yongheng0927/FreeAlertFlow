import { createContext, useContext } from 'react'

import type { Role, User } from '../api/types'

export interface AuthState {
  /** undefined = 正在恢复会话；null = 未登录 */
  user: User | null | undefined
  login: (username: string, password: string) => Promise<void>
  logout: () => Promise<void>
  /** editor 或 admin：允许执行写操作 */
  canWrite: boolean
  isAdmin: boolean
  hasRole: (...roles: Role[]) => boolean
}

export const AuthContext = createContext<AuthState | null>(null)

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
