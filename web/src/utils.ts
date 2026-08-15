import dayjs from 'dayjs'
import { message } from 'antd'

import { publicRootUrl } from './config'

export function formatTime(t?: string | null): string {
  if (!t) return '-'
  const d = dayjs(t)
  return d.isValid() ? d.format('YYYY-MM-DD HH:mm:ss') : '-'
}

export async function copyText(text: string, tip = '已复制到剪贴板'): Promise<void> {
  try {
    await navigator.clipboard.writeText(text)
    message.success(tip)
  } catch {
    message.warning('复制失败，请手动复制')
  }
}

/** 拼接某个接入源的完整 Webhook 接收地址 */
export function webhookUrl(token: string, rootUrl?: string): string {
  const root = (rootUrl || publicRootUrl()).replace(/\/+$/, '')
  return `${root}/api/v1/alerts/webhook/${token}`
}
