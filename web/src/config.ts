// 由后端（M4）以 window.__FENGHUO_CONFIG__ 形式注入的运行时配置
// 当注入不存在时（开发服务器，或尚未注入配置的后端）回退到合理默认值
export interface FenghuoRuntimeConfig {
  /** Fenghuo 服务器的公网根 URL，例如 https://alert.example.com */
  rootUrl?: string
  /** 是否启用飞书 OAuth 登录（M4） */
  oauthEnabled?: boolean
  /** SPA 部署的基础路径，例如 /faf/ */
  base?: string
}

declare global {
  interface Window {
    __FENGHUO_CONFIG__?: FenghuoRuntimeConfig
  }
}

const injected: FenghuoRuntimeConfig =
  typeof window !== 'undefined' && window.__FENGHUO_CONFIG__ ? window.__FENGHUO_CONFIG__ : {}

function normalizeBase(base: string | undefined): string {
  if (!base || base === '/') return '/'
  const withLeading = base.startsWith('/') ? base : `/${base}`
  return withLeading.endsWith('/') ? withLeading : `${withLeading}/`
}

export const runtimeConfig = {
  rootUrl: injected.rootUrl ?? '',
  oauthEnabled: injected.oauthEnabled ?? false,
  /** SPA 基础路径，始终以 '/' 结尾（默认为 '/'） */
  base: normalizeBase(injected.base),
}

/** 所有 API 请求的基础 URL（相对路径，可配合开发代理使用） */
export const apiBase = `${runtimeConfig.base}api`.replace(/\/{2,}/g, '/')

/** 拼接 Webhook 地址用的公网根 URL：优先用后端配置的 rootUrl，未配置时退回当前页面 origin */
export function publicRootUrl(): string {
  if (runtimeConfig.rootUrl) return runtimeConfig.rootUrl.replace(/\/+$/, '')
  if (typeof window !== 'undefined') return window.location.origin
  return ''
}
