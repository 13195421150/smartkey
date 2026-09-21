export const USED_CDK_MESSAGE = '卡密无效CDK已被使用'

export function previewErrorMessage(data: any): string {
  const message = String(data?.error || data?.msg || data?.message || 'CDK 无效或不可用')
  if (data?.error_code === 'CDK_ALREADY_USED') return USED_CDK_MESSAGE
  if (!/\b(?:unused|not\s+used)\b/i.test(message) && /已兑换|已(?:被)?使用|已消耗|\b(?:used|redeemed|consumed)\b/i.test(message)) return USED_CDK_MESSAGE
  return message
}

/** Keep the identifier for one submission, including retries after a network failure. */
export function redemptionRequestId(existing = ''): string {
  return existing || `web-${crypto.randomUUID()}`
}

export function extractFullSession(raw: string): string {
  const value = raw.trim()
  if (!value) return ''
  if (value.startsWith('{')) {
    try {
      const data = JSON.parse(value)
      const session = data.sessionToken || data.session_token || data.token?.sessionToken
      return typeof session === 'string' && session.trim() ? value : ''
    } catch { return '' }
  }
  if (value.startsWith('eyJ') && value.split('.').length === 3) return ''
  if (value.split('.').length === 5) return value
  return value.length > 40 ? value : ''
}

export function redemptionStatusLabel(status: string): string {
  const labels: Record<string, string> = {
    queued: '排队中', awaiting_card: '准备卡片', funding_pending: '准备付款',
    dispatching: '正在提交', running: '处理中', pending: '等待结果确认',
    requires_action: '需要进一步确认', plus_paid: '升级处理中', review: '等待对账',
    completed: '兑换完成', declined: '支付未通过', failed_precharge: '兑换未完成',
    cancelled: '已取消', failed: '兑换未完成', error: '查询异常',
  }
  return labels[status.toLowerCase()] || status || '等待更新'
}
