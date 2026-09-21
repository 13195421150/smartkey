import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import BatchRedeemView from '../../views/user/BatchRedeemView.vue'
import ExcelImportBlock from '../../components/ExcelImportBlock.vue'
import { AUTO_SUBMIT_CONCURRENCY } from '../batch-session'

vi.mock('vue-router', () => ({ useRoute: () => ({ path: '/batch', query: {} }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

let wrapper: VueWrapper | undefined
const reply = (data: unknown, status = 200) => new Response(JSON.stringify(data), { status })
function button(text: string) {
  const found = wrapper!.findAll('button').find(b => b.text().includes(text))
  if (!found) throw new Error(`Button not found: ${text}`)
  return found
}
beforeEach(() => { localStorage.clear(); sessionStorage.clear(); vi.useFakeTimers() })
afterEach(() => { wrapper?.unmount(); wrapper = undefined; vi.useRealTimers(); vi.unstubAllGlobals() })

const globals = { stubs: { RedeemModeTabs: true, ElTag: { template: '<span><slot /></span>' } } }

it('shows the used-CDK message for a rejected batch item', async () => {
  vi.stubGlobal('fetch', vi.fn(async () => reply({ code: 400, error_code: 'CDK_ALREADY_USED', msg: '卡密无效CDK已被使用' }, 400)))
  wrapper = mount(BatchRedeemView, { global: globals })
  await wrapper.find('#batch-codes').setValue('ZC-ALREADY-USED')
  await button('batch.verifyStart').trigger('click'); await flushPromises()
  expect(wrapper.text()).toContain('卡密无效CDK已被使用')
})
function sessionFixture(index: number) {
  return JSON.stringify({ sessionToken: `local-session-fixture-${index}`, user: { email: `fixture${index}@example.test` } })
}
async function importCsv(csv: string) {
  const file = new File([csv], 'sessions.csv', { type: 'text/csv' })
  Object.defineProperty(file, 'text', { value: async () => csv })
  wrapper!.findComponent(ExcelImportBlock).vm.$emit('pick', file)
  await flushPromises()
}

it('pauses new batch work while allowing already-started tasks to finish', async () => {
  const pending: Array<(response: Response) => void> = []
  let submitted = 0
  vi.stubGlobal('fetch', vi.fn(async (url: string, init: RequestInit = {}) => {
    const body = init.body ? JSON.parse(String(init.body)) : {}
    if (url.endsWith('/preview')) return reply({ code: 0, data: { redemption_token: `fixture-${body.code}`, plan: 'plus' } })
    if (url.endsWith('/preflight')) {
      expect(body.credential.mode).toBe('session')
      expect(Object.keys(body.credential).sort()).toEqual(['mode', 'session'])
      return new Promise<Response>(resolve => pending.push(resolve))
    }
    if (url.endsWith('/redeem')) { submitted++; return reply({ code: 0, data: { status: 'queued' } }, 202) }
    if (url.includes('/result')) return reply({ code: 0, data: { order: { status: 'pending' } } })
    throw new Error(`Unexpected request ${url}`)
  }))
  wrapper = mount(BatchRedeemView, { global: globals })
  const total = AUTO_SUBMIT_CONCURRENCY + 1
  await wrapper.find('#batch-codes').setValue(Array.from({ length: total }, (_, i) => `ZC-LOCAL-${i}-ONLY`).join('\n'))
  await importCsv('session\n' + Array.from({ length: total }, (_, i) => `"${sessionFixture(i).replace(/"/g, '""')}"`).join('\n'))
  await button('batch.verifyStart').trigger('click'); await flushPromises()
  await button('batch.autoSubmit').trigger('click'); await flushPromises()
  expect(pending).toHaveLength(AUTO_SUBMIT_CONCURRENCY)
  await button('暂停后续提交').trigger('click')
  for (const resolve of pending) resolve(reply({ code: 0, data: { preflight_token: 'fixture-preflight', email: 'fixture@example.test' } }))
  await flushPromises(); await flushPromises()
  expect(submitted).toBe(AUTO_SUBMIT_CONCURRENCY)
  expect(pending).toHaveLength(AUTO_SUBMIT_CONCURRENCY)
  expect(button('继续提交剩余任务').exists()).toBe(true)
})

it('accepts only Session input for a manually submitted batch item', async () => {
  const preflights: Array<{ credential: { mode: string; session: string } }> = []
  vi.stubGlobal('fetch', vi.fn(async (url: string, init: RequestInit = {}) => {
    const body = init.body ? JSON.parse(String(init.body)) : {}
    if (url.endsWith('/preview')) return reply({ code: 0, data: { redemption_token: 'fixture-token', plan: 'plus' } })
    if (url.endsWith('/preflight')) {
      preflights.push(body)
      return reply({ code: 0, data: { preflight_token: 'fixture-preflight', email: 'fixture0@example.test' } })
    }
    if (url.endsWith('/redeem')) return reply({ code: 0, data: { status: 'queued' } }, 202)
    if (url.includes('/result')) return reply({ code: 0, data: { order: { status: 'pending' } } })
    throw new Error(`Unexpected request ${url}`)
  }))
  wrapper = mount(BatchRedeemView, { global: globals })
  expect(wrapper.find('#batch-mailboxes').exists()).toBe(false)
  expect(wrapper.text()).not.toContain('batch.modeMailbox')
  await wrapper.find('#batch-codes').setValue('ZC-LOCAL-SESSION-ONLY')
  await button('batch.verifyStart').trigger('click'); await flushPromises()
  expect(wrapper.find('#batch-mailboxes').exists()).toBe(false)
  await wrapper.find('#batch-session').setValue('{"accessToken":"eyJ.test.token"}')
  await button('batch.submitNext').trigger('click'); await flushPromises()
  expect(preflights).toHaveLength(0)
  expect(wrapper.text()).toContain('不能只用 Access Token')
  await wrapper.find('#batch-session').setValue(sessionFixture(0))
  await button('batch.submitNext').trigger('click'); await flushPromises()
  expect(preflights).toHaveLength(1)
  expect(preflights[0].credential).toEqual({ mode: 'session', session: sessionFixture(0) })
})

it('does not import an email-password sheet as Session credentials', async () => {
  wrapper = mount(BatchRedeemView, { global: globals })
  await importCsv('email,password\nfixture@example.test,local-test-password')
  expect(wrapper.text()).toContain('未识别到 Session 列')
  expect(wrapper.findComponent(ExcelImportBlock).props('sessionPool')).toEqual([])
})
