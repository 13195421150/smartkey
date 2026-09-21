import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import CDKStatusView from '../../views/user/CDKStatusView.vue'
import BillingCheckView from '../../views/user/BillingCheckView.vue'
import zh from '../../i18n/locales/zh'

const route = vi.hoisted(() => ({ path: '/history', query: {} }))
vi.mock('vue-router', () => ({ useRoute: () => route }))
const fetchMock = vi.fn()
const wrappers: ReturnType<typeof mount>[] = []
beforeEach(() => { fetchMock.mockReset(); localStorage.clear(); vi.stubGlobal('fetch', fetchMock) })
afterEach(() => { wrappers.splice(0).forEach(wrapper => wrapper.unmount()); vi.unstubAllGlobals() })
function render(subscription = false) {
  route.path = subscription ? '/billing' : '/history'
  const wrapper = mount(subscription ? BillingCheckView : CDKStatusView, { global: {
    plugins: [createI18n({ legacy: false, locale: 'zh', messages: { zh } })],
    stubs: { RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' } },
  } })
  wrappers.push(wrapper)
  return wrapper
}
const reply = (data: unknown, status = 200) => new Response(JSON.stringify(data), { status })

it('shows redemption time in Beijing time, independent of browser timezone', async () => {
  fetchMock.mockResolvedValue(reply({ cdk_code: 'TEST-CODE', status: 'used', order_status: 'completed', used_at: '2026-09-20T09:50:21.216Z' }))
  const wrapper = render()
  await wrapper.find('textarea').setValue('TEST-CODE')
  await wrapper.find('form').trigger('submit'); await flushPromises()
  expect(wrapper.text()).toContain('兑换时间（北京时间）：2026-09-20 17:50:21')
  expect(wrapper.text()).not.toContain('09:50:21')
})

it('shows failure reason, error code and expanded public timeline', async () => {
  fetchMock.mockResolvedValue(reply({ cdk_code: 'TEST-CODE', status: 'failed', order_status: 'declined', order_id: 12, stage: 'payment', failure_reason: '支付被拒：卡片余额不足', error_code: 'INSUFFICIENT_CARD_BALANCE', created_at: '2026-09-20T09:00:00Z', events: [{ created_at: '2026-09-20T09:01:00Z', step: 'payment', code: 'PAYMENT_DECLINED', message: '支付渠道拒绝了本次付款' }] }))
  const wrapper = render()
  await wrapper.find('textarea').setValue('TEST-CODE')
  await wrapper.find('form').trigger('submit'); await flushPromises()
  expect(wrapper.text()).toContain('支付被拒：卡片余额不足')
  expect(wrapper.text()).toContain('INSUFFICIENT_CARD_BALANCE')
  expect(wrapper.text()).toContain('支付渠道拒绝了本次付款')
  expect(wrapper.text()).toContain('2026-09-20 17:01:00')
  expect(wrapper.find('details').attributes('open')).toBeDefined()
})

it('shows a preflight failure even if no order was created', async () => {
  fetchMock.mockResolvedValue(reply({ cdk_code: 'TEST-CODE', status: 'unused', last_attempt: { phase: 'preflight', error_code: 'GPT_SESSION_INVALID', message: 'Session 已过期，请重新登录', occurred_at: '2026-09-20T09:00:00Z', uncertain: false } }))
  const wrapper = render()
  await wrapper.find('textarea').setValue('TEST-CODE')
  await wrapper.find('form').trigger('submit'); await flushPromises()
  expect(wrapper.text()).toContain('最近一次提交提示')
  expect(wrapper.text()).toContain('账号校验')
  expect(wrapper.text()).toContain('Session 已过期，请重新登录')
})

it('queries a single code through the same form and clears input and results', async () => {
  fetchMock.mockResolvedValue(reply({ cdk_code: 'TEST-CODE', status: 'processing', message: '正在确认' }))
  const wrapper = render()
  expect(wrapper.findAll('nav a').map(link => link.text())).toEqual(['卡密进度', '订阅状态'])
  expect(wrapper.findAll('textarea')).toHaveLength(1)
  expect(wrapper.findAll('button').map(button => button.text())).toEqual(['清空', '查询'])
  await wrapper.find('textarea').setValue('test-code')
  await wrapper.find('form').trigger('submit')
  await flushPromises()
  expect(fetchMock).toHaveBeenCalledWith('/api/v1/lookup/cdk?code=TEST-CODE', expect.objectContaining({ cache: 'no-store' }))
  expect(wrapper.text()).toContain('正在确认')
  await wrapper.find('button[type="button"]').trigger('click')
  expect((wrapper.find('textarea').element as HTMLTextAreaElement).value).toBe('')
  expect(wrapper.find('.query-results').exists()).toBe(false)
})

it('automatically uses batch lookup for multiple codes and removes duplicates', async () => {
  fetchMock.mockResolvedValue(reply({ results: [
    { cdk_code: 'TEST-ONE', status: 'used' }, { cdk_code: 'TEST-TWO', status: 'failed' },
  ] }))
  const wrapper = render()
  await wrapper.find('textarea').setValue('test-one\nTEST-TWO\ntest-one')
  await wrapper.find('form').trigger('submit')
  await flushPromises()
  expect(fetchMock).toHaveBeenCalledWith('/api/v1/lookup/cdk/batch', expect.objectContaining({
    method: 'POST', body: JSON.stringify({ codes: ['TEST-ONE', 'TEST-TWO'] }),
  }))
  expect(wrapper.findAll('.query-result')).toHaveLength(2)
})

it('rejects excess codes rather than silently omitting part of the query', async () => {
  const wrapper = render()
  await wrapper.find('textarea').setValue(Array.from({ length: 101 }, (_, i) => `TEST-${i}`).join('\n'))
  await wrapper.find('form').trigger('submit')
  expect(fetchMock).not.toHaveBeenCalled()
  expect(wrapper.find('[role="alert"]').text()).toContain('最多查询 100')
})

it('does not display cached used status as a successful order', async () => {
  fetchMock.mockResolvedValue(reply({ cdk_code: 'TEST-CODE', status: 'used', account_email: 'fixture@example.test', message: '卡密使用成功', can_resubmit: true }))
  const wrapper = render()
  await wrapper.find('textarea').setValue('TEST-CODE')
  await wrapper.find('form').trigger('submit')
  await flushPromises()
  expect(wrapper.find('.query-status').text()).toBe('结果待确认')
  expect(wrapper.text()).not.toContain('使用成功')
  expect(wrapper.find('.portal-inline-link').exists()).toBe(false)
})

it.each([
  ['failed', 'failed_precharge', '使用失败'],
  ['processing', 'plus_paid', '处理中'],
  ['review', 'review', '待对账'],
  ['used', 'completed', '兑换完成'],
])('renders %s from the reconciled order', async (status, order_status, label) => {
  fetchMock.mockResolvedValue(reply({ cdk_code: 'TEST-CODE', status, order_status }))
  const wrapper = render()
  await wrapper.find('textarea').setValue('TEST-CODE')
  await wrapper.find('form').trigger('submit')
  await flushPromises()
  expect(wrapper.find('.query-status').text()).toBe(label)
})

it('preserves the redemption device when looking up a bound token', async () => {
  localStorage.setItem('cdk_device_id', 'same-browser-fixture')
  fetchMock.mockResolvedValue(reply({ cdk_code: 'TEST-CODE', status: 'processing' }))
  const wrapper = render()
  await wrapper.find('textarea').setValue('TEST-CODE')
  await wrapper.find('form').trigger('submit')
  await flushPromises()
  expect(new Headers(fetchMock.mock.calls[0][1].headers).get('X-Redemption-Device')).toBe('same-browser-fixture')
})

it('blocks duplicate requests while a query is pending', async () => {
  let resolve!: (response: Response) => void
  fetchMock.mockImplementation(() => new Promise<Response>(done => { resolve = done }))
  const wrapper = render()
  await wrapper.find('textarea').setValue('TEST-CODE')
  await wrapper.find('form').trigger('submit')
  await wrapper.find('form').trigger('submit')
  expect(fetchMock).toHaveBeenCalledTimes(1)
  expect(wrapper.find('button[type="button"]').attributes('disabled')).toBeDefined()
  resolve(reply({ cdk_code: 'TEST-CODE', status: 'used' }))
  await flushPromises()
})

it('allows retry after a failed query without showing stale results', async () => {
  fetchMock.mockResolvedValueOnce(reply({ error: '卡密不存在' }, 404))
    .mockResolvedValueOnce(reply({ cdk_code: 'TEST-CODE', status: 'used' }))
  const wrapper = render()
  await wrapper.find('textarea').setValue('TEST-CODE')
  await wrapper.find('form').trigger('submit')
  await flushPromises()
  expect(wrapper.find('[role="alert"]').text()).toBe('卡密不存在')
  await wrapper.find('form').trigger('submit')
  await flushPromises()
  expect(wrapper.find('[role="alert"]').exists()).toBe(false)
  expect(wrapper.find('.query-results').exists()).toBe(true)
})

it('shows only subscription status even when the API also returns invoices', async () => {
  fetchMock.mockResolvedValue(reply({ summary: { plan_type: 'plus', has_active_subscription: true, will_renew: false, expires_at: '2026-10-20' }, invoices: [{ description: 'hidden-invoice', hosted_invoice_url: 'https://example.test/invoice' }] }))
  const wrapper = render(true)
  const session = '{"accessToken":"local-fixture-only"}'
  await wrapper.find('textarea').setValue(session)
  await wrapper.find('form').trigger('submit')
  await flushPromises()
  expect(fetchMock).toHaveBeenCalledWith('/api/v1/public/billing/check', expect.objectContaining({ body: JSON.stringify({ token_input: session }) }))
  expect(wrapper.text()).toContain('有效')
  expect(wrapper.text()).toContain('已关闭')
  expect(wrapper.text()).toContain('2026-10-20')
  expect(wrapper.text()).not.toMatch(/账单|hidden-invoice|Session 查询|卡密查询/)
  await wrapper.find('button[type="button"]').trigger('click')
  expect(wrapper.find('.subscription-summary').exists()).toBe(false)
  expect((wrapper.find('textarea').element as HTMLTextAreaElement).value).toBe('')
})

it('keeps unknown subscription fields unknown instead of reporting inactive', async () => {
  fetchMock.mockResolvedValue(reply({ summary: {} }))
  const wrapper = render(true)
  await wrapper.find('textarea').setValue('local-access-token-fixture')
  await wrapper.find('form').trigger('submit')
  await flushPromises()
  expect(wrapper.findAll('dd').map(field => field.text())).toEqual(['—', '—', '—', '—'])
})
