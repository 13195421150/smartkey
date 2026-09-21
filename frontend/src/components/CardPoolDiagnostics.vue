<template>
  <section class="card space-y-4" aria-labelledby="pool-title">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div><h2 id="pool-title" class="text-xl font-bold text-ink">实际选卡诊断</h2><p class="text-sm text-muted mt-1">直接读取卡台卡池，不需要把卡片导入本站。预览不会开卡或充值。</p></div>
      <div class="flex flex-wrap items-center gap-2">
        <label for="pool-plan" class="text-sm">诊断套餐</label>
        <select id="pool-plan" v-model="plan" class="input !w-auto" :disabled="loading" @change="load">
          <option value="plus">Plus</option><option value="pro_5x">Pro 5x</option><option value="pro_20x">Pro 20x</option>
        </select>
        <button class="btn-secondary" :disabled="loading" @click="load">{{ loading ? '读取中…' : '刷新卡池' }}</button>
      </div>
    </div>
    <p v-if="error" role="alert" class="alert alert-error">{{ error }}</p>
    <p v-if="loading" role="status" class="text-sm text-muted">正在核对卡台的候选卡与剩余次数…</p>
    <template v-if="pool">
      <p class="text-sm text-muted">实际普通档上限：{{ limit(pool.rule.light_max_uses) }}；Pro 20x：{{ limit(pool.rule.pro20_max_uses) }}；失败计次：{{ pool.rule.count_failures ? '是' : '否' }}；卡台失败换卡：{{ pool.rule.auto_switch_on_fail ? `最多 ${pool.rule.max_auto_switches} 次` : '关闭' }}。</p>
      <p class="text-sm text-muted">卡台优先范围：{{ priorityText }}。当前排序：{{ modeText }}。</p>
      <p v-if="!pool.candidates.length" class="text-sm text-muted">卡台没有返回候选卡。请检查主站卡片状态与产品开关。</p>
      <div class="grid gap-3 md:grid-cols-2" aria-live="polite">
        <article v-for="card in pool.candidates" :key="card.card_id" class="rounded-xl border p-4 space-y-2" style="border-color: var(--brd)">
          <div class="flex flex-wrap justify-between gap-2"><b>卡尾号 {{ card.last_four || '—' }}</b><span>{{ card.locally_excluded ? '本站已排除' : card.picked ? '卡台当前首选' : card.skip ? '卡台跳过' : '候选卡' }}</span></div>
          <p class="text-sm text-muted">{{ card.issuer }} · 卡台 ID {{ card.card_id }} · 参考余额 ${{ card.available_usd || '—' }}</p>
          <p class="text-sm">普通档：已用 {{ card.light_used }}，剩余 {{ remaining(card.light_remaining) }}；Pro 20x：已用 {{ card.pro20_used }}，剩余 {{ remaining(card.pro20_remaining) }}</p>
          <p v-if="card.skip_reason" class="text-sm break-words">跳过原因：{{ card.skip_reason }}</p>
          <p v-if="card.locally_excluded" class="text-sm">该卡在本站排除名单中，CDK 兑换不会使用。</p>
          <button class="btn-secondary" :disabled="usageLoading" @click="loadUsage(card.card_id)">查看占用与失败计次</button>
        </article>
      </div>
      <p v-if="usageError" role="alert" class="alert alert-error">{{ usageError }}</p>
      <div v-if="usage" class="rounded-xl bg-soft p-4 text-sm space-y-2" aria-live="polite">
        <b>卡台 ID {{ usage.card_id }} 的实时用量</b>
        <p>状态：{{ usage.card_status }}；占用中：{{ usage.busy ? '是' : '否' }}；冷却中：{{ usage.cooling ? '是' : '否' }}。</p>
        <p>普通档：成功 {{ usage.light.completed }} / 失败 {{ usage.light.failed }} / 预留 {{ usage.light.reserved }}。</p>
        <p>Pro 20x：成功 {{ usage.pro20.completed }} / 失败 {{ usage.pro20.failed }} / 预留 {{ usage.pro20.reserved }}。</p>
      </div>
      <p class="text-xs text-muted">这里只读诊断，次数上限由卡台用卡规则控制。余额为卡台返回的参考值；正式兑换还会检查占用、余额及平台风控。选卡优先级保存后可刷新核对实际生效范围。</p>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { authFetch } from '../lib/api'

const props = defineProps<{ refreshKey?: number }>()
interface Candidate { card_id: number; last_four: string; issuer: string; picked: boolean; skip: boolean; skip_reason: string; locally_excluded: boolean; available_usd: string; light_used: number; light_remaining: number; pro20_used: number; pro20_remaining: number }
interface Bucket { completed: number; failed: number; reserved: number }
interface Usage { card_id: number; card_status: string; busy: boolean; cooling: boolean; light: Bucket; pro20: Bucket }
interface Pool { rule: { light_max_uses: number; pro20_max_uses: number; count_failures: boolean; auto_switch_on_fail: boolean; max_auto_switches: number; select_mode: string; select_priority?: { issuer: string; segment_key: string }[] }; candidates: Candidate[] }
const plan = ref('plus')
const pool = ref<Pool | null>(null)
const loading = ref(false)
const error = ref('')
const usage = ref<Usage | null>(null)
const usageLoading = ref(false)
const usageError = ref('')
let loadID = 0
const limit = (n: number) => n === 0 ? '不限' : `${n} 次`
const remaining = (n: number) => n === -1 ? '不限' : `${n} 次`
const priorityText = computed(() => pool.value?.rule.select_priority?.map(p => `${p.issuer}/${p.segment_key}`).join(' → ') || '卡台默认规则')
const modeText = computed(() => ({ default: '余额优先', lowest_usage: '使用次数最少', newest: '最新开卡' })[pool.value?.rule.select_mode as 'default'] || pool.value?.rule.select_mode || '—')
async function load() {
  const id = ++loadID
  loading.value = true; error.value = ''; pool.value = null; usage.value = null
  try {
    const r = await authFetch(`/api/v1/admin/card-selection/pool?plan=${plan.value}`, { cache: 'no-store' })
    const d = await r.json()
    if (id !== loadID) return
    if (!r.ok) throw new Error(d.error || '读取卡池失败')
    pool.value = d
  } catch (e) { if (id === loadID) error.value = e instanceof Error ? e.message : '读取失败，请刷新重试' }
  finally { if (id === loadID) loading.value = false }
}
async function loadUsage(id: number) {
  usageLoading.value = true; usageError.value = ''; usage.value = null
  try {
    const r = await authFetch(`/api/v1/admin/card-selection/cards/${id}/usage`, { cache: 'no-store' })
    const d = await r.json()
    if (!r.ok) throw new Error(d.error || '读取用量失败')
    usage.value = d
  } catch (e) { usageError.value = e instanceof Error ? e.message : '读取用量失败，请重试' }
  finally { usageLoading.value = false }
}
watch(() => props.refreshKey, load)
onMounted(load)
</script>
