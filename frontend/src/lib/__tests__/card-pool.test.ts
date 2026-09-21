import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import CardPoolDiagnostics from '../../components/CardPoolDiagnostics.vue'
const fetch = vi.hoisted(() => vi.fn())
vi.mock('../api', () => ({ authFetch: fetch }))
beforeEach(() => fetch.mockReset())
afterEach(() => vi.restoreAllMocks())
const reply = (data: unknown, status = 200) => new Response(JSON.stringify(data), { status })
const pool = { rule: { light_max_uses: 5, pro20_max_uses: 3, count_failures: true, auto_switch_on_fail: true, max_auto_switches: 2, select_mode: 'default', select_priority: [] }, candidates: [{ card_id: 7, last_four: '7637', issuer: '渠道1', skip: true, skip_reason: '不在当前选卡优先级内', light_used: 1, light_remaining: 4, pro20_used: 1, pro20_remaining: 2, available_usd: '0.31' }] }
it('shows actual remaining capacity and exclusion reason without claiming a local card import is needed', async () => {
  fetch.mockResolvedValueOnce(reply(pool)).mockResolvedValueOnce(reply({ card_id: 7, card_status: 'ACTIVE', busy: false, cooling: false, light: { completed: 1, failed: 0, reserved: 0 }, pro20: { completed: 1, failed: 0, reserved: 0 } }))
  const w = mount(CardPoolDiagnostics)
  await flushPromises()
  expect(w.text()).toContain('不在当前选卡优先级内')
  expect(w.text()).toContain('剩余 4 次')
  expect(w.text()).toContain('普通档上限：5 次')
  await w.findAll('button')[1].trigger('click')
  await flushPromises()
  expect(fetch).toHaveBeenLastCalledWith('/api/v1/admin/card-selection/cards/7/usage', { cache: 'no-store' })
  expect(w.text()).toContain('冷却中：否')
  w.unmount()
})
it('clears old diagnostic data and shows an error when a refreshed request fails', async () => {
  fetch.mockResolvedValueOnce(reply(pool)).mockResolvedValueOnce(reply({ error: '卡台暂不可达' }, 502))
  const w = mount(CardPoolDiagnostics)
  await flushPromises()
  await w.find('button').trigger('click')
  await flushPromises()
  expect(w.find('[role="alert"]').text()).toBe('卡台暂不可达')
  expect(w.text()).not.toContain('7637')
  w.unmount()
})
