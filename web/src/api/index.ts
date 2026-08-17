import { api } from './client'
import type {
  Alert,
  AlertDetail,
  AlertFilterParams,
  Channel,
  DashboardStats,
  Delivery,
  DeliveryFilterParams,
  ListEnvelope,
  ResendResult,
  Role,
  RoutingRule,
  Source,
  SystemInfo,
  Template,
  TemplatePreview,
  TestSendResult,
  TokenPair,
  User,
} from './types'

// ---- 认证 ----
export async function login(username: string, password: string): Promise<TokenPair> {
  const { data } = await api.post<TokenPair>('/auth/login', { username, password })
  return data
}

export async function logout(refreshToken: string): Promise<void> {
  await api.post('/auth/logout', { refresh_token: refreshToken })
}

// ---- 系统信息 / 当前用户 ----
export async function getSystemInfo(): Promise<SystemInfo> {
  const { data } = await api.get<SystemInfo>('/v1/system/info')
  return data
}

export async function getMe(): Promise<User> {
  const { data } = await api.get<User>('/v1/users/me')
  return data
}

export async function changePassword(oldPassword: string, newPassword: string): Promise<void> {
  await api.put('/v1/users/me/password', {
    old_password: oldPassword,
    new_password: newPassword,
  })
}

// ---- 接入源 ----
export interface SourceCreateInput {
  name: string
  description?: string
}

export interface SourcePatch {
  name?: string
  description?: string
  enabled?: boolean
}

export async function listSources(page = 1, pageSize = 100): Promise<ListEnvelope<Source>> {
  const { data } = await api.get<ListEnvelope<Source>>('/v1/sources', {
    params: { page, page_size: pageSize },
  })
  return data
}

export async function createSource(input: SourceCreateInput): Promise<Source> {
  const { data } = await api.post<Source>('/v1/sources', input)
  return data
}

export async function updateSource(id: number, patch: SourcePatch): Promise<Source> {
  const { data } = await api.put<Source>(`/v1/sources/${id}`, patch)
  return data
}

export async function deleteSource(id: number): Promise<void> {
  await api.delete(`/v1/sources/${id}`)
}

export async function rotateSourceToken(id: number): Promise<Source> {
  const { data } = await api.post<Source>(`/v1/sources/${id}/rotate-token`)
  return data
}

// ---- 通知渠道 ----
export interface ChannelCreateInput {
  name: string
  /** feishu / dingtalk / wecom */
  type: string
  webhook_url: string
  secret?: string
  keyword?: string
  template_id?: number
  at_all?: boolean
  enabled?: boolean
}

export interface ChannelPatch {
  name?: string
  /** 不传或为空 = 保持当前值 */
  webhook_url?: string
  /** 不传 = 保持；'' = 清除 */
  secret?: string
  keyword?: string
  /** 不传 = 保持；0 = 解绑 */
  template_id?: number
  at_all?: boolean
  enabled?: boolean
}

export async function listChannels(page = 1, pageSize = 100): Promise<ListEnvelope<Channel>> {
  const { data } = await api.get<ListEnvelope<Channel>>('/v1/channels', {
    params: { page, page_size: pageSize },
  })
  return data
}

export async function createChannel(input: ChannelCreateInput): Promise<Channel> {
  const { data } = await api.post<Channel>('/v1/channels', input)
  return data
}

export async function updateChannel(id: number, patch: ChannelPatch): Promise<Channel> {
  const { data } = await api.put<Channel>(`/v1/channels/${id}`, patch)
  return data
}

export async function deleteChannel(id: number): Promise<void> {
  await api.delete(`/v1/channels/${id}`)
}

export async function testChannel(id: number): Promise<TestSendResult> {
  const { data } = await api.post<TestSendResult>(`/v1/channels/${id}/test`)
  return data
}

// ---- 消息模板 ----
export interface TemplateInput {
  name: string
  channel_type: string
  content: string
  remark?: string
}

export async function listTemplates(channelType?: string): Promise<ListEnvelope<Template>> {
  const { data } = await api.get<ListEnvelope<Template>>('/v1/templates', {
    params: { page: 1, page_size: 100, channel_type: channelType || undefined },
  })
  return data
}

export async function createTemplate(input: TemplateInput): Promise<Template> {
  const { data } = await api.post<Template>('/v1/templates', input)
  return data
}

export async function updateTemplate(id: number, input: TemplateInput): Promise<Template> {
  const { data } = await api.put<Template>(`/v1/templates/${id}`, input)
  return data
}

export async function deleteTemplate(id: number): Promise<void> {
  await api.delete(`/v1/templates/${id}`)
}

export async function previewTemplate(content: string, channelType: string): Promise<TemplatePreview> {
  const { data } = await api.post<TemplatePreview>('/v1/templates/preview', {
    content,
    channel_type: channelType,
  })
  return data
}

export async function testTemplateSend(
  content: string,
  channelType: string,
  channelId: number,
): Promise<TestSendResult> {
  const { data } = await api.post<TestSendResult>('/v1/templates/test-send', {
    content,
    channel_type: channelType,
    channel_id: channelId,
  })
  return data
}

// ---- 路由规则 ----
export interface RuleInput {
  source_id: number
  name?: string
  priority?: number
  match_labels: Record<string, string>
  channel_id: number
  continue_matching?: boolean
  enabled?: boolean
}

export async function listRules(sourceId?: number): Promise<ListEnvelope<RoutingRule>> {
  const { data } = await api.get<ListEnvelope<RoutingRule>>('/v1/rules', {
    params: { page: 1, page_size: 100, source_id: sourceId || undefined },
  })
  return data
}

export async function createRule(input: RuleInput): Promise<RoutingRule> {
  const { data } = await api.post<RoutingRule>('/v1/rules', input)
  return data
}

export async function updateRule(id: number, input: RuleInput): Promise<RoutingRule> {
  const { data } = await api.put<RoutingRule>(`/v1/rules/${id}`, input)
  return data
}

export async function deleteRule(id: number): Promise<void> {
  await api.delete(`/v1/rules/${id}`)
}

// ---- 告警 ----
export async function listAlerts(params: AlertFilterParams): Promise<ListEnvelope<Alert>> {
  const { data } = await api.get<ListEnvelope<Alert>>('/v1/alerts', { params })
  return data
}

export async function getAlert(id: number): Promise<{ alert: AlertDetail; deliveries: Delivery[] }> {
  const { data } = await api.get<{ alert: AlertDetail; deliveries: Delivery[] }>(`/v1/alerts/${id}`)
  return data
}

// ---- 投递记录（editor/admin） ----
export async function listDeliveries(params: DeliveryFilterParams): Promise<ListEnvelope<Delivery>> {
  const { data } = await api.get<ListEnvelope<Delivery>>('/v1/deliveries', { params })
  return data
}

export async function resendDelivery(id: number): Promise<ResendResult> {
  const { data } = await api.post<ResendResult>(`/v1/deliveries/${id}/resend`)
  return data
}

// ---- 仪表盘统计 ----
export async function getDashboardStats(): Promise<DashboardStats> {
  const { data } = await api.get<DashboardStats>('/v1/stats/dashboard')
  return data
}

// ---- 用户管理（admin） ----
export async function listUsers(page = 1, pageSize = 100): Promise<ListEnvelope<User>> {
  const { data } = await api.get<ListEnvelope<User>>('/v1/users', {
    params: { page, page_size: pageSize },
  })
  return data
}

export async function createUser(input: { username: string; password: string; role?: Role }): Promise<User> {
  const { data } = await api.post<User>('/v1/users', input)
  return data
}

export async function updateUser(id: number, patch: { role?: Role; enabled?: boolean }): Promise<User> {
  const { data } = await api.put<User>(`/v1/users/${id}`, patch)
  return data
}

export async function deleteUser(id: number): Promise<void> {
  await api.delete(`/v1/users/${id}`)
}

export async function resetUserPassword(id: number, password: string): Promise<void> {
  await api.put(`/v1/users/${id}/password`, { password })
}
