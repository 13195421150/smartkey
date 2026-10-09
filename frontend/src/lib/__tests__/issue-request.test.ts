import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { beginIssue, finishIssue, pendingIssue } from '../issue-request'

beforeEach(() => sessionStorage.clear())
afterEach(() => vi.unstubAllGlobals())
it('reuses a persisted request after an uncertain response or page reload', () => {
  const key = beginIssue('plus', 2)
  expect(pendingIssue()).toEqual({ plan: 'plus', count: 2, region: '', key })
  expect(beginIssue('plus', 2)).toBe(key)
})
it('does not reuse an uncertain purchase for a different quantity, plan, or region', () => {
  beginIssue('plus', 2, 'PH')
  expect(() => beginIssue('plus', 3)).toThrow('尚未确认')
  expect(() => beginIssue('pro_5x', 2)).toThrow('尚未确认')
  expect(() => beginIssue('plus', 2, 'CL')).toThrow('尚未确认')
  expect(beginIssue('plus', 2, 'ph')).toBe(pendingIssue()?.key)
})
it('treats legacy attempts without a region as the default region', () => {
  sessionStorage.setItem('cdk_pending_issue_v1', JSON.stringify({ plan: 'plus', count: 1, key: 'legacy-key' }))
  expect(pendingIssue()).toEqual({ plan: 'plus', count: 1, region: '', key: 'legacy-key' })
  expect(beginIssue('plus', 1, '')).toBe('legacy-key')
})
it('creates a new request only after a known completed/rejected attempt is cleared', () => {
  const key = beginIssue('plus', 2)
  finishIssue()
  expect(pendingIssue()).toBeNull()
  expect(beginIssue('plus', 2)).not.toBe(key)
})
it('creates an idempotency key when randomUUID is unavailable on HTTP deployments', () => {
  vi.stubGlobal('crypto', {
    getRandomValues<T extends ArrayBufferView>(values: T): T {
      new Uint8Array(values.buffer, values.byteOffset, values.byteLength).fill(0xab)
      return values
    },
  })
  expect(beginIssue('plus', 1, 'CL')).toBe(`issue-${'ab'.repeat(16)}`)
})
