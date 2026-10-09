const KEY = 'cdk_pending_issue_v1'
interface IssueAttempt { plan: string; count: number; region: string; key: string }

function normalizeRegion(region = ''): string {
  return region.trim().toUpperCase()
}

function newIssueKey(): string {
  if (typeof crypto.randomUUID === 'function') return `issue-${crypto.randomUUID()}`
  const bytes = new Uint8Array(16)
  crypto.getRandomValues(bytes)
  return `issue-${Array.from(bytes, byte => byte.toString(16).padStart(2, '0')).join('')}`
}

export function pendingIssue(): IssueAttempt | null {
  const raw = sessionStorage.getItem(KEY)
  if (!raw) return null
  const parsed = JSON.parse(raw) as Partial<IssueAttempt>
  return {
    plan: String(parsed.plan || ''),
    count: Number(parsed.count || 0),
    region: normalizeRegion(parsed.region),
    key: String(parsed.key || ''),
  }
}
export function beginIssue(plan: string, count: number, region = ''): string {
  const normalizedRegion = normalizeRegion(region)
  const pending = pendingIssue()
  if (pending) {
    if (pending.plan !== plan || pending.count !== count || pending.region !== normalizedRegion) {
      const pendingRegion = pending.region || '默认(菲律宾)'
      throw new Error(`上一笔 ${pending.plan} × ${pending.count} · ${pendingRegion} 的结果尚未确认，请恢复相同套餐、数量和地区，用同一请求重试。`)
    }
    return pending.key
  }
  const attempt = { plan, count, region: normalizedRegion, key: newIssueKey() }
  // Persist before sending; failing storage must not create an untracked purchase.
  sessionStorage.setItem(KEY, JSON.stringify(attempt))
  return attempt.key
}
export function finishIssue() { sessionStorage.removeItem(KEY) }
