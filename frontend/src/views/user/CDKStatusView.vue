<template>
  <div class="portal-view portal-query">
    <RedeemModeTabs />
    <form class="query-form" @submit.prevent="query">
      <div class="form-group">
        <label for="lookup-codes">{{ t('queryCenter.codesLabel') }}</label>
        <textarea id="lookup-codes" v-model="codesText" rows="5" class="input mono"
          :placeholder="t('queryCenter.codesPlaceholder')" :disabled="querying"
          spellcheck="false" autocomplete="off" />
      </div>
      <p v-if="error" role="alert" class="alert alert-error">{{ error }}</p>
      <div class="query-actions">
        <button type="button" class="btn-secondary" :disabled="querying" @click="clear">{{ t('queryCenter.clear') }}</button>
        <button type="submit" class="btn-primary" :disabled="querying || !codes.length">
          {{ querying ? t('common.querying') : t('queryCenter.query') }}
        </button>
      </div>
    </form>

    <section v-if="results.length" class="query-results" aria-live="polite" :aria-label="t('cdkLookup.resultTitle')">
      <article v-for="row in results" :key="row.cdk_code" class="query-result">
        <div class="query-result-heading">
          <span class="mono query-code">{{ row.cdk_code }}</span>
          <b class="query-status" :style="{ color: statusColor(row.status) }">{{ statusText(row.status) }}</b>
        </div>
        <p v-if="row.account_email || row.plan" class="query-result-detail">{{ [row.account_email, row.plan].filter(Boolean).join(' · ') }}</p>
        <p v-if="row.message" class="query-result-detail">{{ row.message }}</p>
        <p v-if="row.used_at" class="query-result-detail">{{ t('cdkLookup.usedAt') }}：{{ formatBeijingTime(row.used_at) }}</p>
        <div v-if="row.status === 'failed'" class="alert alert-error mt-3 break-words">
          <b>{{ t('cdkLookup.failureReason') }}：</b>{{ row.failure_reason || row.notes || t('cdkLookup.noFailureReason') }}
          <p v-if="row.error_code" class="mt-1">{{ t('cdkLookup.errorCode') }}：<code>{{ row.error_code }}</code></p>
        </div>
        <details v-if="row.order_id || row.events?.length || row.last_attempt" :open="row.status === 'failed' || !!row.last_attempt" class="mt-3 rounded-xl border border-line p-3">
          <summary class="cursor-pointer font-medium">{{ t('cdkLookup.details') }}</summary>
          <div class="mt-3 space-y-3 text-sm break-words">
            <p v-if="row.order_id">{{ t('cdkLookup.orderId') }}：<span class="mono">{{ row.order_id }}</span></p>
            <p v-if="row.stage">{{ t('cdkLookup.stage') }}：{{ phaseText(row.stage) }}</p>
            <p v-if="row.created_at">{{ t('cdkLookup.createdAt') }}：{{ formatBeijingTime(row.created_at) }}</p>
            <p v-if="row.updated_at">{{ t('cdkLookup.updatedAt') }}：{{ formatBeijingTime(row.updated_at) }}</p>
            <section v-if="row.last_attempt" class="rounded-lg bg-soft p-3 space-y-1">
              <b>{{ t('cdkLookup.lastAttempt') }}</b>
              <p>{{ phaseText(row.last_attempt.phase) }} · {{ formatBeijingTime(row.last_attempt.occurred_at) }} {{ t('cdkLookup.beijingTime') }}</p>
              <p>{{ row.last_attempt.message }}</p>
              <p v-if="row.last_attempt.error_code">{{ t('cdkLookup.errorCode') }}：<code>{{ row.last_attempt.error_code }}</code></p>
            </section>
            <div v-if="row.events?.length">
              <b>{{ t('cdkLookup.timeline') }}</b>
              <ol class="mt-2 space-y-3">
                <li v-for="(event, i) in row.events" :key="i" class="border-l-2 border-line pl-3 space-y-1">
                  <p class="text-muted"><time>{{ formatBeijingTime(event.created_at) }}</time> · {{ phaseText(event.step || event.status || '') }}</p>
                  <p>{{ event.message || event.status || '—' }}</p>
                  <code v-if="event.code" class="text-xs text-muted">{{ event.code }}</code>
                </li>
              </ol>
            </div>
            <p v-else-if="row.order_id" class="text-muted">{{ t(row.details_available ? 'cdkLookup.noEvents' : 'cdkLookup.detailsUnavailable') }}</p>
          </div>
        </details>
        <router-link v-if="row.can_resubmit || row.status === 'unused'" class="portal-inline-link"
          :to="{ path: '/recharge', query: { cdk: row.cdk_code } }">
          {{ t(row.can_resubmit ? 'cdkLookup.resubmit' : 'cdkLookup.goRedeem') }}
        </router-link>
      </article>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import RedeemModeTabs from '../../components/RedeemModeTabs.vue'
import { parseCdks } from '../../lib/batch-session'
import { formatBeijingTime } from '../../lib/beijing-time'

const { t } = useI18n({ useScope: 'global' })
const LOOKUP_BATCH_MAX = 100
interface CDKStatusResult {
  cdk_code: string
  status: string
  order_status?: string
  can_resubmit?: boolean
  account_email?: string
  plan?: string
  used_at?: string
  message?: string
  order_id?: number
  failure_reason?: string
  notes?: string
  error_code?: string
  stage?: string
  created_at?: string
  updated_at?: string
  details_available?: boolean
  events?: Array<{ created_at?: string; step?: string; code?: string; message?: string; status?: string }>
  last_attempt?: { phase: string; message: string; error_code?: string; occurred_at: string; uncertain: boolean }
}
const codesText = ref('')
const querying = ref(false)
const error = ref('')
const results = ref<CDKStatusResult[]>([])
const codes = computed(() => parseCdks(codesText.value))

function phaseText(phase: string) {
  const key = `cdkLookup.phases.${phase}`
  const label = t(key)
  return label === key ? phase : label
}

function statusText(status: string) {
  const key = `cdkLookup.status.${status}`
  const label = t(key)
  return label === key ? status : label
}
function statusColor(status: string) {
  if (status === 'used') return 'var(--good)'
  if (status === 'failed') return 'var(--err)'
  if (status === 'processing') return 'var(--primary)'
  if (['disabled', 'expired', 'unknown', 'unconfirmed', 'review', 'frozen'].includes(status)) return 'var(--warn)'
  return 'var(--ink)'
}
function clear() {
  codesText.value = ''
  results.value = []
  error.value = ''
}
async function query() {
  if (querying.value || !codes.value.length) return
  error.value = ''
  results.value = []
  if (codes.value.length > LOOKUP_BATCH_MAX) {
    error.value = t('queryCenter.tooMany', { max: LOOKUP_BATCH_MAX })
    return
  }
  const requested = [...codes.value]
  querying.value = true
  try {
    const headers = new Headers()
    const device = localStorage.getItem('cdk_device_id')
    if (device) headers.set('X-Redemption-Device', device)
    const response = requested.length === 1
      ? await fetch(`/api/v1/lookup/cdk?code=${encodeURIComponent(requested[0])}`, { headers, cache: 'no-store' })
      : await fetch('/api/v1/lookup/cdk/batch', {
        method: 'POST', headers: { ...Object.fromEntries(headers), 'Content-Type': 'application/json' }, cache: 'no-store',
        body: JSON.stringify({ codes: requested }),
      })
    const data = await response.json()
    if (!response.ok) {
      error.value = data.error || data.message || t('cdkLookup.errNotFound')
      return
    }
    const rows: CDKStatusResult[] = requested.length === 1 ? [data] : data.results || []
    results.value = rows.map(row => row.status === 'used' && row.order_status !== 'completed'
      ? { ...row, status: 'unconfirmed', can_resubmit: false, message: t('cdkLookup.msgUnconfirmed') }
      : row)
  } catch {
    error.value = t('cdkLookup.errNetwork')
  } finally {
    querying.value = false
  }
}
</script>
