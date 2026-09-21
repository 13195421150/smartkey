<template>
  <nav class="portal-subnav" :aria-label="queryMode ? '查询方式' : '兑换方式'">
      <router-link
        v-for="tab in tabs"
        :key="tab.to"
        :to="tab.to"
        :class="{ active: isActive(tab.to) }"
      >
        {{ tab.label }}
      </router-link>
  </nav>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'

const route = useRoute()
const { t } = useI18n({ useScope: 'global' })

const queryMode = computed(() => ['/history', '/billing'].includes(route.path))
const tabs = computed(() => queryMode.value ? [
  { to: '/history', label: t('redeemTabs.lookup') },
  { to: '/billing', label: t('redeemTabs.billing') },
] : [
  { to: '/', label: t('redeemTabs.single') },
  { to: '/batch', label: t('redeemTabs.batch') },
])

function isActive(to: string) {
  if (to === '/') return route.path === '/' || route.path === '/recharge'
  return route.path === to || route.path.startsWith(to + '/')
}
</script>
