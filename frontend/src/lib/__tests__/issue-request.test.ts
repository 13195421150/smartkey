import { beforeEach, expect, it } from 'vitest'
import { beginIssue, finishIssue, pendingIssue } from '../issue-request'

beforeEach(() => sessionStorage.clear())
it('reuses a persisted request after an uncertain response or page reload', () => {
  const key = beginIssue('plus', 2)
  expect(pendingIssue()).toEqual({ plan: 'plus', count: 2, key })
  expect(beginIssue('plus', 2)).toBe(key)
})
it('does not reuse an uncertain purchase for a different quantity or plan', () => {
  beginIssue('plus', 2)
  expect(() => beginIssue('plus', 3)).toThrow('尚未确认')
  expect(() => beginIssue('pro_5x', 2)).toThrow('尚未确认')
})
it('creates a new request only after a known completed/rejected attempt is cleared', () => {
  const key = beginIssue('plus', 2)
  finishIssue()
  expect(pendingIssue()).toBeNull()
  expect(beginIssue('plus', 2)).not.toBe(key)
})
