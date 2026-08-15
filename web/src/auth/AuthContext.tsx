import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'

import * as authApi from '../api'
import { clearTokens, getAccessToken, getRefreshToken, setTokens } from '../api/client'
import type { Role, User } from '../api/types'
import { AuthContext, type AuthState } from './useAuth'

export default function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null | undefined>(undefined)

  useEffect(() => {
    if (!getAccessToken()) {
      setUser(null)
      return
    }
    authApi
      .getMe()
      .then(setUser)
      .catch(() => {
        clearTokens()
        setUser(null)
      })
  }, [])

  const login = useCallback(async (username: string, password: string) => {
    const pair = await authApi.login(username, password)
    setTokens(pair)
    setUser(await authApi.getMe())
  }, [])

  const logout = useCallback(async () => {
    const refreshToken = getRefreshToken()
    if (refreshToken) {
      // 尽力而为：在服务端吊销 refresh token，失败也照常登出
      await authApi.logout(refreshToken).catch(() => undefined)
    }
    clearTokens()
    setUser(null)
  }, [])

  const value = useMemo<AuthState>(() => {
    const role = user?.role
    return {
      user,
      login,
      logout,
      canWrite: role === 'editor' || role === 'admin',
      isAdmin: role === 'admin',
      hasRole: (...roles: Role[]) => (role ? roles.includes(role) : false),
    }
  }, [user, login, logout])

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}
