import assert from 'node:assert/strict'
import { readFileSync, mkdtempSync, rmSync } from 'node:fs'
import { resolve } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'
import { parse, compileScript } from '@vue/compiler-sfc'
import { build } from 'esbuild'
import { effectScope } from 'vue'
const root = fileURLToPath(new URL('..', import.meta.url))
const storage = new Map()
globalThis.localStorage = { getItem: k => storage.get(k) || null, setItem: (k, v) => storage.set(k, v) }
const calls = [], alerts = []
let consent = true, failed = true
globalThis.__rotationTest = {
  confirm: async () => consent,
  alert: async message => { alerts.push(message) },
  fetch: async (path, init) => {
    calls.push({ path, body: JSON.parse(init.body) })
    return failed
      ? new Response(JSON.stringify({ error: 'synthetic uncertainty' }), { status: 502 })
      : new Response(JSON.stringify({ id: 7, code: 'PLUS-NEW-SYNTHETIC-0001', full_code: 'PLUS-NEW-SYNTHETIC-0001', generation: 1, site_rotated: true }))
  },
}
const mocks = {
  'vue-router': 'export const useRouter=()=>({push(){}})',
  '../../lib/api': 'export const authFetch=globalThis.__rotationTest.fetch',
  '../../lib/dialog': 'export const dialog={confirm:globalThis.__rotationTest.confirm,alert:globalThis.__rotationTest.alert,toast(){}}',
}
const { descriptor } = parse(readFileSync(resolve(root, 'src/views/admin/CDKeyManagement.vue'), 'utf8'))
const compiled = compileScript(descriptor, { id: 'rotation-test' })
const tmp = mkdtempSync(resolve(root, '.rotation-test-')), scope = effectScope()
try {
  await build({ stdin: { contents: compiled.content, resolveDir: resolve(root, 'src/views/admin'), loader: 'ts' }, outfile: resolve(tmp, 'component.mjs'), bundle: true, platform: 'node', format: 'esm', external: ['vue'], plugins: [{ name: 'mocks', setup(b) {
    b.onResolve({ filter: /.*/ }, a => a.path in mocks ? { path: a.path, namespace: 'mock' } : undefined)
    b.onLoad({ filter: /.*/, namespace: 'mock' }, a => ({ contents: mocks[a.path], loader: 'js' }))
  } }] })
  const { default: component } = await import(pathToFileURL(resolve(tmp, 'component.mjs')).href)
  const vm = scope.run(() => component.setup({}, { expose() {} }))
  const row = { id: 7, code: 'PLUS-OLD-SYNTHETIC-0001', full_code: 'PLUS-OLD-SYNTHETIC-0001', status: 'unused', plan: 'plus', generation: 0, note: 'unchanged', created_at: 'unchanged', task_id: 123, failure_reason: 'unchanged' }
  const other = { id: 8, code: 'PLUS-OTHER-SYNTHETIC-0001', status: 'unused' }
  vm.rows.value = [row, other]
  vm.rememberIssued([row], 'plus')
  consent = false
  await vm.rotateOne(vm.displayRows.value[0]); assert.equal(calls.length, 0)
  consent = true
  await vm.rotateOne(vm.displayRows.value[0]); assert.equal(calls.length, 1)
  assert.equal(vm.rows.value[0].code, row.code)
  failed = false
  await vm.rotateOne(vm.displayRows.value[0]); assert.equal(calls.length, 2)
  assert.equal(calls[0].body.client_request_id, calls[1].body.client_request_id, 'uncertain retries must be idempotent')
  assert.equal(calls[1].path, '/api/v1/admin/cardplatform/cdks/7/rotate')
  assert.equal(calls[1].body.generation, 0)
  assert.equal(vm.displayRows.value[0].fullCode, 'PLUS-NEW-SYNTHETIC-0001')
  for (const field of ['id', 'status', 'plan', 'note', 'created_at', 'task_id', 'failure_reason']) assert.equal(vm.rows.value[0][field], row[field])
  assert.deepEqual(vm.rows.value[1], other)
  assert.ok(!storage.get('cdk_full_code_cache_v1').includes(row.code), 'old browser key must be discarded')
  assert.ok(alerts[0].includes('PLUS-NEW-SYNTHETIC-0001'))
  console.log('PASS: admin rotation consent, retry idempotency, same row ID/history, untouched other rows, current code copy/cache')
} finally {
  scope.stop(); rmSync(tmp, { recursive: true }); delete globalThis.__rotationTest; delete globalThis.localStorage
}
