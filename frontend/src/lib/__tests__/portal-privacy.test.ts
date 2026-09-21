import { afterEach, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import PortalLayout from '../../layouts/PortalLayout.vue'

vi.mock('vue-router', () => ({ useRoute: () => ({ path: '/', query: {} }) }))
vi.mock('vue-i18n', async importOriginal => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ locale: { value: 'zh' } }),
}))
afterEach(() => vi.unstubAllGlobals())

it('keeps the downstream portal free of operator branding and contact links', async () => {
  vi.stubGlobal('fetch', vi.fn(async () => new Response('{"status":"ok"}', { status: 200 })))
  const wrapper = mount(PortalLayout, { global: { stubs: {
    ThemeToggle: true, LanguageToggle: true, RouterView: true,
    RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' },
  } } })
  await flushPromises()
  expect(wrapper.text()).not.toMatch(/smartkey|QQ|微信|aishop|@qq\.com/i)
  expect(wrapper.find('img').attributes('src')).toBe('/black-cat.png')
  for (const link of wrapper.findAll('a')) {
    expect(link.attributes('href')).toMatch(/^(\/|#)/)
  }
  expect(wrapper.text()).toContain('发码方')
  wrapper.unmount()
})
