<template>
  <!-- 自定义首页内容：保持后台配置能力不变 -->
  <div v-if="homeContent" class="min-h-screen">
    <iframe
      v-if="isHomeContentUrl"
      :src="homeContent.trim()"
      class="h-screen w-full border-0"
      allowfullscreen
    ></iframe>
    <div v-else v-html="homeContent"></div>
  </div>

  <!-- 简洁默认首页 -->
  <div v-else class="home-page min-h-screen bg-white text-slate-900 dark:bg-slate-950 dark:text-white">
    <header class="border-b border-slate-200/70 bg-white/85 backdrop-blur dark:border-slate-800 dark:bg-slate-950/80">
      <nav class="mx-auto flex max-w-6xl items-center justify-between px-5 py-4">
        <div class="flex items-center gap-3">
          <div class="flex h-10 w-10 items-center justify-center overflow-hidden rounded-xl border border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-900">
            <img :src="siteLogo || '/logo.svg'" alt="Logo" class="h-full w-full object-contain" />
          </div>
          <div>
            <div class="text-base font-semibold tracking-tight">{{ siteName }}</div>
            <div class="hidden text-xs text-slate-500 dark:text-slate-400 sm:block">AI API 服务平台</div>
          </div>
        </div>

        <div class="flex items-center gap-2">
          <LocaleSwitcher />
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="icon-button"
            title="查看文档"
          >
            <Icon name="book" size="md" />
          </a>
          <button class="icon-button" :title="isDark ? '切换浅色模式' : '切换深色模式'" @click="toggleTheme">
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>
          <router-link :to="isAuthenticated ? dashboardPath : '/login'" class="top-login">
            {{ isAuthenticated ? '控制台' : '登录' }}
          </router-link>
        </div>
      </nav>
    </header>

    <main>
      <section class="mx-auto max-w-6xl px-5 py-16 text-center sm:py-24">
        <div class="mx-auto mb-5 inline-flex items-center gap-2 rounded-full border border-slate-200 bg-slate-50 px-3 py-1 text-xs font-medium text-slate-600 dark:border-slate-800 dark:bg-slate-900 dark:text-slate-300">
          <span class="h-1.5 w-1.5 rounded-full bg-emerald-500"></span>
          统一 API · 账号池 · 生图工作台
        </div>

        <h1 class="mx-auto max-w-4xl text-4xl font-bold tracking-tight text-slate-950 dark:text-white sm:text-6xl">
          简单、稳定地管理你的 API 服务
        </h1>

        <p class="mx-auto mt-6 max-w-2xl text-base leading-8 text-slate-600 dark:text-slate-400 sm:text-lg">
          {{ siteSubtitle }}。统一接入模型、分组、密钥、余额和生图能力，让用户打开就知道怎么用。
        </p>

        <div class="mt-9 flex flex-col items-center justify-center gap-3 sm:flex-row">
          <router-link :to="isAuthenticated ? dashboardPath : '/login'" class="primary-button">
            {{ isAuthenticated ? '进入控制台' : '开始使用' }}
            <Icon name="arrowRight" size="sm" />
          </router-link>
          <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer" class="secondary-button">
            查看文档
          </a>
        </div>
      </section>

      <section class="mx-auto max-w-6xl px-5 pb-16">
        <div class="grid gap-4 md:grid-cols-3">
          <article v-for="item in features" :key="item.title" class="feature-card">
            <div class="feature-icon">
              <Icon :name="item.icon" size="md" />
            </div>
            <h2 class="mt-4 text-lg font-semibold text-slate-950 dark:text-white">{{ item.title }}</h2>
            <p class="mt-2 text-sm leading-6 text-slate-600 dark:text-slate-400">{{ item.desc }}</p>
          </article>
        </div>
      </section>

      <section class="mx-auto max-w-6xl px-5 pb-20">
        <div class="rounded-3xl border border-slate-200 bg-slate-50 p-5 dark:border-slate-800 dark:bg-slate-900/60 sm:p-8">
          <div class="grid gap-8 lg:grid-cols-[0.9fr_1.1fr] lg:items-center">
            <div class="text-left">
              <h2 class="text-2xl font-semibold tracking-tight text-slate-950 dark:text-white">兼容 OpenAI 风格接口</h2>
              <p class="mt-3 text-sm leading-7 text-slate-600 dark:text-slate-400">
                用户可以用自己的 Key 调接口，也可以直接在网页里的生图工作台提交任务、查看结果和历史记录。
              </p>
            </div>

            <div class="code-box">
              <pre><code>POST {{ apiBaseExample }}/v1/images/generations
Authorization: Bearer sk-你的密钥

{
  "model": "gemini-banana-pro-4k",
  "prompt": "一只橘猫趴在窗台上晒太阳",
  "async": true,
  "stream": false
}</code></pre>
            </div>
          </div>
        </div>
      </section>
    </main>

    <footer class="border-t border-slate-200 py-6 dark:border-slate-800">
      <div class="mx-auto flex max-w-6xl flex-col items-center justify-between gap-3 px-5 text-sm text-slate-500 dark:text-slate-400 sm:flex-row">
        <span>© {{ currentYear }} {{ siteName }}. All rights reserved.</span>
        <div class="flex items-center gap-4">
          <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer" class="hover:text-slate-900 dark:hover:text-white">文档</a>
          <a :href="githubUrl" target="_blank" rel="noopener noreferrer" class="hover:text-slate-900 dark:hover:text-white">GitHub</a>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useAuthStore, useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import { sanitizeUrl } from '@/utils/url'

type HomeIconName = 'server' | 'shield' | 'sparkles'

const authStore = useAuthStore()
const appStore = useAppStore()

const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || 'AI API Gateway Platform')
const docUrl = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || ''))
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')
const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

const isDark = ref(document.documentElement.classList.contains('dark'))
const githubUrl = 'https://github.com/Wei-Shaw/sub2api'
const isAuthenticated = computed(() => authStore.isAuthenticated)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => isAdmin.value ? '/admin/dashboard' : '/dashboard')
const currentYear = computed(() => new Date().getFullYear())
const apiBaseExample = computed(() => window.location.origin)

const features: Array<{ icon: HomeIconName; title: string; desc: string }> = [
  { icon: 'server', title: '统一网关', desc: '一个入口管理多平台模型，减少接入成本。' },
  { icon: 'shield', title: '分组管控', desc: '按用户、分组、模型和额度做权限控制。' },
  { icon: 'sparkles', title: '生图工作台', desc: '网页端直接生图，结果和任务记录清晰可查。' },
]

function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

function initTheme() {
  const savedTheme = localStorage.getItem('theme')
  if (savedTheme === 'dark' || (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)) {
    isDark.value = true
    document.documentElement.classList.add('dark')
  }
}

onMounted(() => {
  initTheme()
  authStore.checkAuth()
  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }
})
</script>

<style scoped>
.home-page {
  background-image:
    radial-gradient(circle at top left, rgba(14, 165, 233, 0.08), transparent 32rem),
    radial-gradient(circle at top right, rgba(99, 102, 241, 0.06), transparent 30rem);
}

.icon-button {
  display: inline-flex;
  height: 2.25rem;
  width: 2.25rem;
  align-items: center;
  justify-content: center;
  border-radius: 0.75rem;
  color: rgb(100 116 139);
  transition: background 0.2s ease, color 0.2s ease;
}

.icon-button:hover {
  background: rgb(241 245 249);
  color: rgb(15 23 42);
}

.dark .icon-button {
  color: rgb(148 163 184);
}

.dark .icon-button:hover {
  background: rgb(30 41 59);
  color: white;
}

.top-login,
.primary-button,
.secondary-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.45rem;
  font-weight: 600;
  transition: transform 0.15s ease, background 0.2s ease, border-color 0.2s ease;
}

.top-login {
  border-radius: 0.85rem;
  background: rgb(15 23 42);
  padding: 0.55rem 0.9rem;
  font-size: 0.875rem;
  color: white;
}

.dark .top-login {
  background: white;
  color: rgb(15 23 42);
}

.primary-button {
  border-radius: 1rem;
  background: rgb(15 23 42);
  padding: 0.85rem 1.25rem;
  color: white;
}

.dark .primary-button {
  background: white;
  color: rgb(15 23 42);
}

.secondary-button {
  border-radius: 1rem;
  border: 1px solid rgb(203 213 225);
  padding: 0.85rem 1.25rem;
  color: rgb(51 65 85);
}

.dark .secondary-button {
  border-color: rgb(51 65 85);
  color: rgb(226 232 240);
}

.top-login:hover,
.primary-button:hover,
.secondary-button:hover {
  transform: translateY(-1px);
}

.feature-card {
  border: 1px solid rgb(226 232 240);
  border-radius: 1.5rem;
  background: rgba(255, 255, 255, 0.78);
  padding: 1.5rem;
  text-align: left;
  transition: border-color 0.2s ease, transform 0.2s ease, box-shadow 0.2s ease;
}

.dark .feature-card {
  border-color: rgb(30 41 59);
  background: rgba(15, 23, 42, 0.72);
}

.feature-card:hover {
  transform: translateY(-2px);
  border-color: rgb(148 163 184);
  box-shadow: 0 18px 40px rgba(15, 23, 42, 0.07);
}

.feature-icon {
  display: inline-flex;
  height: 2.75rem;
  width: 2.75rem;
  align-items: center;
  justify-content: center;
  border-radius: 1rem;
  background: rgb(241 245 249);
  color: rgb(15 23 42);
}

.dark .feature-icon {
  background: rgb(30 41 59);
  color: white;
}

.code-box {
  overflow: hidden;
  border-radius: 1rem;
  border: 1px solid rgb(226 232 240);
  background: white;
  padding: 1rem;
}

.dark .code-box {
  border-color: rgb(30 41 59);
  background: rgb(2 6 23);
}

.code-box pre {
  overflow-x: auto;
  white-space: pre;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', monospace;
  font-size: 0.8125rem;
  line-height: 1.8;
  color: rgb(51 65 85);
}

.dark .code-box pre {
  color: rgb(203 213 225);
}
</style>
