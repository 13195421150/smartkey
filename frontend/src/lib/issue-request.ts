const KEY = 'cdk_pending_issue_v1'
interface IssueAttempt { plan: string; count: number; key: string }

export function pendingIssue(): IssueAttempt | null {
  const raw = sessionStorage.getItem(KEY)
  return raw ? JSON.parse(raw) as IssueAttempt : null
}
export function beginIssue(plan: string, count: number): string {
  const pending = pendingIssue()
  if (pending) {
    if (pending.plan !== plan || pending.count !== count) throw new Error(`上一笔 ${pending.plan} × ${pending.count} 的结果尚未确认，请恢复该套餐和数量，用同一请求重试。`)
    return pending.key
  }
  const attempt = { plan, count, key: `issue-${crypto.randomUUID()}` }
  // Persist before sending; failing storage must not create an untracked purchase.
  sessionStorage.setItem(KEY, JSON.stringify(attempt))
  return attempt.key
}
export function finishIssue() { sessionStorage.removeItem(KEY) }
