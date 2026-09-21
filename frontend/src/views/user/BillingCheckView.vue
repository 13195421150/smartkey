<template>
  <div class="portal-view portal-query">
    <RedeemModeTabs />
    <form class="query-form" @submit.prevent="query">
      <div class="form-group">
        <div class="query-field-heading">
          <label for="billing-session">Session JSON / accessToken</label>
          <a class="app-link" href="https://chatgpt.com/api/auth/session" target="_blank" rel="noopener noreferrer">{{ t('queryCenter.getSession') }} ↗</a>
        </div>
        <textarea id="billing-session" v-model="tokenInput" rows="5" class="input mono"
          :placeholder="t('queryCenter.sessionPlaceholder')" :disabled="loading"
          spellcheck="false" autocomplete="off" />
      </div>
      <p v-if="error" role="alert" class="alert alert-error">{{ error }}</p>
      <div class="query-actions">
        <button type="button" class="btn-secondary" :disabled="loading" @click="clear">{{ t('queryCenter.clear') }}</button>
        <button type="submit" class="btn-primary" :disabled="loading || !tokenInput.trim()">
          {{ loading ? t('common.querying') : t('queryCenter.querySubscription') }}
        </button>
      </div>
    </form>

    <dl v-if="summary" class="query-results subscription-summary" aria-live="polite" :aria-label="t('redeemTabs.billing')">
      <div><dt>{{ t('queryCenter.plan') }}</dt><dd>{{ planLabel }}</dd></div>
      <div><dt>{{ t('queryCenter.subscription') }}</dt><dd>{{ summary.has_active_subscription == null ? '—' : t(summary.has_active_subscription ? 'queryCenter.active' : 'queryCenter.inactive') }}</dd></div>
      <div><dt>{{ t('queryCenter.autoRenew') }}</dt><dd>{{ summary.will_renew == null ? '—' : t(summary.will_renew ? 'queryCenter.on' : 'queryCenter.off') }}</dd></div>
      <div><dt>{{ t('queryCenter.expires') }}</dt><dd>{{ summary.active_until || summary.expires_at || summary.renews_at || '—' }}</dd></div>
    </dl>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import RedeemModeTabs from '../../components/RedeemModeTabs.vue'

interface SubscriptionSummary {
  plan_type?: string
  subscription_plan?: string
  has_active_subscription?: boolean | null
  will_renew?: boolean | null
  active_until?: string
  expires_at?: string
  renews_at?: string
}
const { t } = useI18n({ useScope: 'global' })
const tokenInput = ref('')
const loading = ref(false)
const error = ref('')
const summary = ref<SubscriptionSummary | null>(null)
const planLabel = computed(() => {
  const raw = summary.value?.plan_type || summary.value?.subscription_plan || ''
  if (!raw) return '—'
  if (raw === 'free') return t('queryCenter.free')
  return raw.replace('chatgpt', 'ChatGPT ').replace(/_/g, ' ')
})
function clear() {
  tokenInput.value = ''
  summary.value = null
  error.value = ''
}
async function query() {
  if (loading.value || !tokenInput.value.trim()) return
  error.value = ''
  summary.value = null
  loading.value = true
  try {
    const response = await fetch('/api/v1/public/billing/check', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ token_input: tokenInput.value.trim() }),
    })
    const data = await response.json()
    if (!response.ok) {
      error.value = data.error || t('queryCenter.failed')
      return
    }
    summary.value = data.summary || {}
  } catch {
    error.value = t('cdkLookup.errNetwork')
  } finally {
    loading.value = false
  }
}
</script>
