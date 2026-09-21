<template>
  <div class="portal-shell">
    <a class="portal-skip" href="#portal-content">跳到操作区域</a>
    <header class="portal-header">
      <div class="portal-header-inner">
        <router-link to="/" class="portal-brand" aria-label="兑换中心首页">
          <span class="portal-logo"><img src="/black-cat.png" alt="" width="64" height="64" /></span>
          <span><strong>{{ en ? 'Redeem' : '卡密兑换' }}</strong><small>{{ en ? 'SELF-SERVICE PORTAL' : '自助兑换中心' }}</small></span>
        </router-link>
        <nav class="portal-header-links" :aria-label="en ? 'Main navigation' : '主导航'">
          <router-link to="/tutorial">{{ en ? 'How it works' : '使用指南' }}</router-link>
          <router-link to="/history">{{ en ? 'Check progress' : '查询进度' }}</router-link>
        </nav>
        <div class="portal-header-tools"><LanguageToggle /><ThemeToggle /></div>
      </div>
    </header>

    <main class="portal-main">
      <section class="portal-hero">
        <div class="portal-eyebrow"><span></span> SELF-SERVICE · {{ en ? 'MADE SIMPLE' : '简单一点，清楚一点' }}</div>
        <h1 v-if="section === 'redeem'">{{ en ? 'Your next upgrade,' : '让每一次升级，' }}<span>{{ en ? ' starts here.' : '轻松一点。' }}</span></h1>
        <h1 v-else-if="section === 'query'">{{ en ? 'Every step,' : '每一步进度，' }}<span>{{ en ? ' in the clear.' : '都有答案。' }}</span></h1>
        <h1 v-else-if="section === 'partner'">{{ en ? 'A little help,' : '遇到问题，' }}<span>{{ en ? ' to keep going.' : '一起解决。' }}</span></h1>
        <h1 v-else>{{ en ? 'A clear guide,' : '开始之前，' }}<span>{{ en ? ' from start to finish.' : '看这里。' }}</span></h1>
        <p>{{ heroSubtitle }}</p>
      </section>

      <div class="portal-grid" :class="{ 'portal-grid-wide': route.path === '/batch', 'portal-grid-query': section === 'query' }">
        <section class="portal-workspace" aria-label="兑换工作台">
          <nav class="portal-main-nav" aria-label="功能导航">
            <router-link to="/" :class="{ selected: section === 'redeem' }"><Key aria-hidden="true" />{{ en ? 'Redeem' : '充值兑换' }}</router-link>
            <router-link to="/history" :class="{ selected: section === 'query' }"><Search aria-hidden="true" />{{ en ? 'Query center' : '查询中心' }}</router-link>
            <router-link to="/tutorial" :class="{ selected: section === 'guide' }"><Reading aria-hidden="true" />{{ en ? 'Guide & help' : '指南与帮助' }}</router-link>
          </nav>
          <div id="portal-content" class="portal-workspace-body" tabindex="-1"><router-view /></div>
        </section>

        <aside v-if="section !== 'query'" class="portal-aside">
          <section class="portal-note">
            <div class="portal-note-top"><span>GOOD TO KNOW</span><svg viewBox="0 0 94 40" fill="none" aria-hidden="true"><circle cx="20" cy="20" r="16" /><circle cx="20" cy="20" r="5" /><path d="M36 15h53v10H76v9H66v-9H36" /></svg></div>
            <h2>{{ en ? 'Ready, in three steps.' : '准备好，三步就好。' }}</h2>
            <ol class="portal-checklist">
              <li><span>01</span><div><b>{{ en ? 'Have your code ready' : '准备完整卡密' }}</b><p>{{ en ? 'We will identify your plan for you.' : '粘贴购买的卡密，自动识别套餐。' }}</p></div></li>
              <li><span>02</span><div><b>{{ en ? 'Check the right account' : '确认充值账号' }}</b><p>{{ en ? 'Review your account before submitting.' : '核对邮箱与套餐，再确认兑换。' }}</p></div></li>
              <li><span>03</span><div><b>{{ en ? 'Follow your progress' : '等待结果确认' }}</b><p>{{ en ? 'Check progress here. No duplicate orders.' : '处理中可随时查询，无需重复提交。' }}</p></div></li>
            </ol>
            <router-link to="/tutorial" class="portal-note-link">{{ en ? 'Read the full guide' : '第一次使用？查看完整指南' }}<ArrowRight aria-hidden="true" /></router-link>
          </section>
          <section class="portal-support">
            <span class="portal-support-icon"><ChatDotRound aria-hidden="true" /></span>
            <div><h2>{{ en ? 'Need a little help?' : '需要一点帮助？' }}</h2><p>{{ en ? 'Read the guide, or contact the person who provided your code.' : '先查看使用指南，或联系向你提供卡密的发码方。' }}</p><router-link to="/tutorial">{{ en ? 'View common questions' : '查看常见问题' }} <span aria-hidden="true">→</span></router-link></div>
          </section>
          <p class="portal-privacy"><Lock aria-hidden="true" /><span>{{ en ? 'Keep your code and account credentials private.' : '请妥善保管卡密与账号凭据，不要在群聊或截图中公开。' }}</span></p>
        </aside>
      </div>

      <section class="portal-bottom-note" aria-label="服务说明"><span><CircleCheck aria-hidden="true" />{{ en ? 'Confirm before submitting' : '先确认，再提交' }}</span><span><Tickets aria-hidden="true" />{{ en ? 'Progress you can follow' : '处理进度可查询' }}</span><span><Lock aria-hidden="true" />{{ en ? 'Encrypted connection' : 'HTTPS 加密连接' }}</span></section>
    </main>
    <footer class="portal-footer"><span>{{ en ? 'Self-service redemption' : '自助卡密兑换' }}</span><p>{{ en ? 'Independent digital service provider. Not affiliated with OpenAI.' : '独立数字服务商，非 OpenAI 官方网站。' }}</p><span class="portal-connection" :class="connection" role="status"><i></i>{{ connectionText }}</span></footer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Key, Search, Reading, ArrowRight, ChatDotRound, Lock, CircleCheck, Tickets } from '@element-plus/icons-vue'
import ThemeToggle from '../components/ThemeToggle.vue'
import LanguageToggle from '../components/LanguageToggle.vue'
import '../portal.css'

const route = useRoute()
const { locale } = useI18n({ useScope: 'global' })
const en = computed(() => locale.value === 'en')
const section = computed(() => ['/history', '/billing'].includes(route.path) ? 'query' : route.path === '/tutorial' ? 'guide' : route.path === '/partner/swap' ? 'partner' : 'redeem')
const heroSubtitle = computed(() => {
  if (section.value === 'query') return en.value ? 'Check your CDK progress and subscription status.' : '查看卡密进度与账号订阅状态。'
  if (section.value === 'guide') return en.value ? 'From your first code to the final result. A guide for every step.' : '从卡密到兑换结果，把每一步说明白。'
  if (section.value === 'partner') return en.value ? 'A dedicated workspace for authorized partners.' : '面向授权代理的卡密补发入口。'
  return en.value ? 'Verify your code. Confirm your account. Follow your progress.' : '验证卡密，确认账户，查看结果。把复杂的事交给我们。'
})
const connection = ref<'checking' | 'online' | 'offline'>('checking')
const connectionText = computed(() => connection.value === 'online' ? (en.value ? 'Service connected' : '服务连接正常') : connection.value === 'offline' ? (en.value ? 'Connection unavailable' : '连接暂不可用') : (en.value ? 'Checking connection' : '检查连接中'))
onMounted(async () => {
  const controller = new AbortController()
  const timer = setTimeout(() => controller.abort(), 8000)
  try { connection.value = (await fetch('/health', { signal: controller.signal })).ok ? 'online' : 'offline' } catch { connection.value = 'offline' } finally { clearTimeout(timer) }
})
</script>
