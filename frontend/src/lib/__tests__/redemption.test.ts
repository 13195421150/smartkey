import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import RechargeView from '../../views/user/RechargeView.vue'
import { extractFullSession, redemptionRequestId, previewErrorMessage, USED_CDK_MESSAGE } from '../redemption'

vi.mock('vue-router', () => ({ useRoute: () => ({ path: '/', query: {} }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

let wrapper: VueWrapper | undefined
const reply = (data: unknown, status = 200) => new Response(JSON.stringify(data), { status, headers: { 'Content-Type': 'application/json' } })
const globals = { stubs: { RedeemModeTabs: true, RouterLink: { template: '<a><slot /></a>' }, ElTag: { template: '<span><slot /></span>' } } }

beforeEach(() => { localStorage.clear(); sessionStorage.clear(); vi.useFakeTimers() })
afterEach(() => { wrapper?.unmount(); wrapper = undefined; vi.useRealTimers(); vi.unstubAllGlobals() })

function button(text: string) {
  const found = wrapper!.findAll('button').find(item => item.text().includes(text))
  if (!found) throw new Error(`Button not found: ${text}`)
  return found
}

async function prepareConfirmation(redeem: (init: RequestInit) => Promise<Response>) {
  const requests: RequestInit[] = []
  vi.stubGlobal('fetch', vi.fn(async (url: string, init: RequestInit = {}) => {
    if (url.endsWith('/preview')) return reply({ code: 0, data: { redemption_token: 'local-test-token', plan: 'plus' } })
    if (url.endsWith('/preflight')) {
      expect(JSON.parse(String(init.body)).credential).toEqual({
        mode: 'session', session: JSON.stringify({ sessionToken: 'local-fixture', accessToken: 'not-a-real-token' }),
      })
      return reply({ code: 0, data: { preflight_token: 'local-preflight-token', email: 'demo@example.test', currentPlan: 'free', subscription_has_active: false } })
    }
    if (url.endsWith('/redeem')) { requests.push(init); return redeem(init) }
    if (url.includes('/result')) return reply({ code: 0, data: { order: { status: 'review', stage: 'review' } } })
    throw new Error(`Unexpected request: ${url}`)
  }))
  wrapper = mount(RechargeView, { global: globals })
  await wrapper.find('#redeem-code').setValue('ZC-LOCAL-TEST-ONLY')
  await wrapper.find('form').trigger('submit')
  await flushPromises()
  expect(wrapper.find('#redeem-email').exists()).toBe(false)
  expect(wrapper.find('input[type="password"]').exists()).toBe(false)
  expect(wrapper.findAll('button').some(item => item.text() === '邮箱')).toBe(false)
  await wrapper.find('#redeem-session').setValue(JSON.stringify({ sessionToken: 'local-fixture', accessToken: 'not-a-real-token' }))
  await button('校验账号信息').trigger('click')
  await flushPromises()
  return requests
}

describe('redemption safety and recovery', () => {
  it('shows the exact used-CDK rejection without resuming a completed redemption', async () => {
    const fetch = vi.fn(async () => reply({ error_code: 'CDK_ALREADY_USED', msg: USED_CDK_MESSAGE }, 400))
    vi.stubGlobal('fetch', fetch)
    wrapper = mount(RechargeView, { global: globals })
    await wrapper.find('#redeem-code').setValue('ZC-ALREADY-USED')
    await wrapper.find('form').trigger('submit'); await flushPromises()
    expect(wrapper.find('[role="alert"]').text()).toBe(USED_CDK_MESSAGE)
    expect(wrapper.find('#redeem-session').isVisible()).toBe(false)
    expect(fetch).toHaveBeenCalledTimes(1)
  })

  it('recognizes a completed historical order but recovers unfinished orders', async () => {
    for (const status of ['completed', 'review']) {
      const fetch = vi.fn(async (url: string) => url.endsWith('/preview')
        ? reply({ msg: 'CDK 无效或不可用' }, 400)
        : reply({ code: 0, redemption_token: 'fixture-token', data: { order: { status } } }))
      vi.stubGlobal('fetch', fetch)
      wrapper = mount(RechargeView, { global: globals })
      await wrapper.find('#redeem-code').setValue('ZC-HISTORY-CODE')
      await wrapper.find('form').trigger('submit'); await flushPromises()
      expect(wrapper.text()).toContain(status === 'completed' ? USED_CDK_MESSAGE : '等待对账')
      expect(fetch.mock.calls.some(([url]) => url.endsWith('/redeem'))).toBe(false)
      wrapper.unmount(); wrapper = undefined; sessionStorage.clear()
    }
  })

  it('does not label an unused or merely invalid code as already consumed', () => {
    expect(previewErrorMessage({ msg: 'CDK unused' })).toBe('CDK unused')
    expect(previewErrorMessage({ msg: 'CDK not used' })).toBe('CDK not used')
    expect(previewErrorMessage({ msg: '卡密已被使用' })).toBe(USED_CDK_MESSAGE)
    expect(previewErrorMessage({ msg: 'CDK 无效或不可用' })).toBe('CDK 无效或不可用')
  })
  it('rejects access-token-only JSON but accepts a full session', () => {
    expect(extractFullSession('{"accessToken":"eyJ.test.token"}')).toBe('')
    expect(extractFullSession('eyJ.test.token')).toBe('')
    expect(extractFullSession('{invalid')).toBe('')
    expect(extractFullSession('{"sessionToken":"fixture"}')).toContain('fixture')
  })

  it('retains a submission identifier for the same attempt', () => {
    const id = redemptionRequestId()
    expect(id).toMatch(/^web-[\da-f-]{36}$/)
    expect(redemptionRequestId(id)).toBe(id)
    expect(redemptionRequestId()).not.toBe(id)
  })

  it('requires explicit account confirmation and clears credentials after preflight', async () => {
    const requests = await prepareConfirmation(async () => reply({ code: 0, data: { status: 'queued' } }, 202))
    expect(button('确认并开始兑换').attributes('disabled')).toBeDefined()
    expect((wrapper!.find('#redeem-session').element as HTMLTextAreaElement).value).toBe('')
    await button('确认并开始兑换').trigger('click')
    expect(requests).toHaveLength(0)
    await wrapper!.find('#confirm-account').setValue(true)
    await button('确认并开始兑换').trigger('click')
    await flushPromises()
    expect(requests).toHaveLength(1)
    expect(wrapper!.text()).toContain('等待对账')
    expect(wrapper!.findAll('button').filter(b => b.text() === '兑换其他卡密')).toHaveLength(0)
  })

  it('uses the same identifier when retrying a rejected submission', async () => {
    const requests = await prepareConfirmation(async () => reply({ msg: '请求校验未通过' }, 400))
    await wrapper!.find('#confirm-account').setValue(true)
    await button('确认并开始兑换').trigger('click'); await flushPromises()
    await button('确认并开始兑换').trigger('click'); await flushPromises()
    expect(requests).toHaveLength(2)
    expect(JSON.parse(String(requests[0].body)).client_request_id).toBe(JSON.parse(String(requests[1].body)).client_request_id)
  })

  it('does not resubmit after a network failure and restores the original order on reload', async () => {
    const requests = await prepareConfirmation(async () => { throw new TypeError('offline') })
    await wrapper!.find('#confirm-account').setValue(true)
    await button('确认并开始兑换').trigger('click'); await flushPromises()
    expect(requests).toHaveLength(1)
    const saved = JSON.parse(sessionStorage.getItem('cdk_redeem_progress_v1')!)
    expect(saved.step).toBe(4)
    expect(saved.clientRequestId).toBe(JSON.parse(String(requests[0].body)).client_request_id)
    expect(sessionStorage.getItem('cdk_redeem_progress_v1')).not.toContain('not-a-real-token')
    wrapper!.unmount(); wrapper = mount(RechargeView, { global: globals }); await flushPromises()
    expect(wrapper!.text()).toContain('等待对账')
    expect(requests).toHaveLength(1)
  })

  it('renders network validation errors without advancing the flow', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => { throw new TypeError('offline') }))
    wrapper = mount(RechargeView, { global: globals })
    await wrapper.find('#redeem-code').setValue('ZC-LOCAL-INVALID')
    await wrapper.find('form').trigger('submit'); await flushPromises()
    expect(wrapper.find('[role="alert"]').text()).toContain('网络连接暂时中断')
    expect(wrapper.find('form').isVisible()).toBe(true)
  })
})
