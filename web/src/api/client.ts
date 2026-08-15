import axios, { AxiosError, AxiosHeaders, type AxiosRequestConfig } from 'axios'

import { apiBase, runtimeConfig } from '../config'
import type { TokenPair } from './types'

const ACCESS_KEY = 'faf.access_token'
const REFRESH_KEY = 'faf.refresh_token'

export function getAccessToken(): string | null {
  return localStorage.getItem(ACCESS_KEY)
}

export function getRefreshToken(): string | null {
  return localStorage.getItem(REFRESH_KEY)
}

export function setTokens(pair: TokenPair): void {
  localStorage.setItem(ACCESS_KEY, pair.access_token)
  localStorage.setItem(REFRESH_KEY, pair.refresh_token)
}

export function clearTokens(): void {
  localStorage.removeItem(ACCESS_KEY)
  localStorage.removeItem(REFRESH_KEY)
}

/** 强制登出：清空本地令牌并跳转登录页，供 refresh 失败等不可恢复的场景调用 */
export function forceLogout(): void {
  clearTokens()
  const loginPath = `${runtimeConfig.base}login`.replace(/\/{2,}/g, '/')
  if (window.location.pathname !== loginPath) {
    window.location.assign(loginPath)
  }
}

export const api = axios.create({ baseURL: apiBase })

api.interceptors.request.use((cfg) => {
  const token = getAccessToken()
  if (token) {
    cfg.headers = AxiosHeaders.from(cfg.headers).set('Authorization', `Bearer ${token}`)
  }
  return cfg
})

interface RetriedConfig extends AxiosRequestConfig {
  _retried?: boolean
}

// 把并发的 401 串行化到同一次 refresh 调用之后
let refreshing: Promise<string> | null = null

async function doRefresh(): Promise<string> {
  const refreshToken = getRefreshToken()
  if (!refreshToken) throw new Error('no refresh token')
  // 裸 axios 调用：不走拦截器，避免请求拦截器带上已过期的 access token，也避免响应拦截器对 refresh 的 401 再次进入重试逻辑
  const resp = await axios.post<TokenPair>(`${apiBase}/auth/refresh`, {
    refresh_token: refreshToken,
  })
  setTokens(resp.data)
  return resp.data.access_token
}

api.interceptors.response.use(
  (resp) => resp,
  async (err: AxiosError) => {
    const cfg = (err.config ?? {}) as RetriedConfig
    const url = cfg.url ?? ''
    const isAuthCall = url.includes('/auth/login') || url.includes('/auth/refresh')
    if (err.response?.status !== 401 || isAuthCall || cfg._retried) {
      return Promise.reject(err)
    }
    cfg._retried = true
    try {
      refreshing = refreshing ?? doRefresh()
      const token = await refreshing
      cfg.headers = AxiosHeaders.from({
        ...(cfg.headers as Record<string, unknown> | undefined),
        Authorization: `Bearer ${token}`,
      })
      return await api.request(cfg)
    } catch {
      forceLogout()
      return Promise.reject(err)
    } finally {
      refreshing = null
    }
  },
)

/** 提取失败请求的错误信息：优先取后端统一返回的 {"error": "..."}，否则按 403/404 状态码给兜底文案 */
export function errorMessage(err: unknown, fallback = '请求失败'): string {
  if (axios.isAxiosError(err)) {
    const data = err.response?.data as { error?: string } | undefined
    if (data?.error) return data.error
    if (err.response?.status === 403) return '没有权限执行此操作'
    if (err.response?.status === 404) return '资源不存在'
  }
  return fallback
}
