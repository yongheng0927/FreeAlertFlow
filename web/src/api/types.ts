// API 类型，与 Go 后端的 JSON 结构严格一致（snake_case）

export interface ListEnvelope<T> {
  list: T[]
  total: number
  page: number
  page_size: number
}

export interface ApiError {
  error: string
}

export interface TokenPair {
  access_token: string
  refresh_token: string
  token_type: string
  expires_in: number
}

export type Role = 'viewer' | 'editor' | 'admin'

export interface User {
  id: number
  username: string
  name: string
  email: string
  avatar_url: string
  role: Role
  enabled: boolean
  last_login_at: string | null
  created_at: string
}

export interface SystemInfo {
  version: string
  root_url: string
  oauth_enabled: boolean
}

export interface Source {
  id: number
  name: string
  token: string
  description: string
  enabled: boolean
  last_alert_at: string | null
  created_at: string
  updated_at: string
}

export interface Channel {
  id: number
  name: string
  type: string
  /** 已脱敏，例如 https://open.feishu.cn/open-apis/bot/v2/hook/****ab12 */
  webhook_url: string
  has_secret: boolean
  /** 已脱敏的 secret，未设置时为 '' */
  secret: string
  keyword: string
  template_id: number | null
  at_all: boolean
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface TestSendResult {
  success: boolean
  http_status: number
  code: number
  msg: string
  duration_ms: number
}

export interface Template {
  id: number
  name: string
  channel_type: string
  content: string
  is_builtin: boolean
  remark: string
  created_at: string
  updated_at: string
}

export interface RoutingRule {
  id: number
  source_id: number
  name: string
  priority: number
  /** 标签匹配条件，键和值均为字符串 */
  match_labels: Record<string, string>
  channel_id: number
  continue_matching: boolean
  enabled: boolean
  created_at: string
  updated_at: string
}

export type AlertStatus = 'firing' | 'resolved'
export type AlertDisposition = 'delivered' | 'deduped' | 'unmatched'

export interface Alert {
  id: number
  source_id: number
  fingerprint: string
  status: AlertStatus | string
  alertname: string
  severity: string
  labels: Record<string, string>
  annotations: Record<string, string>
  starts_at: string
  ends_at: string | null
  disposition: AlertDisposition | string
  received_at: string
}

export interface AlertDetail extends Alert {
  content_hash: string
  raw_payload: unknown
}

export interface Delivery {
  id: number
  alert_id: number
  channel_id: number
  channel_name: string
  rule_id: number
  /** auto（自动）/ manual（手动重发） */
  trigger_type: string
  attempts: number
  /** success（成功）/ failed（失败） */
  status: string
  http_status: number
  response_code: number
  response_msg: string
  duration_ms: number
  rendered_payload: string | null
  sent_at: string
}

export interface ResendResult {
  success: boolean
  delivery: Delivery
}

export interface AlertFilterParams {
  status?: string
  severity?: string
  alertname?: string
  channel_id?: number
  start?: string
  end?: string
  page?: number
  page_size?: number
}

export interface DeliveryFilterParams {
  status?: string
  channel_id?: number
  start?: string
  end?: string
  page?: number
  page_size?: number
}
