<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="overflow-hidden rounded-3xl border border-primary-100 bg-gradient-to-br from-primary-50 via-white to-cyan-50 p-6 shadow-sm dark:border-dark-700 dark:from-dark-800 dark:via-dark-800 dark:to-dark-900">
        <div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <div class="mb-3 inline-flex items-center rounded-full bg-white/80 px-3 py-1 text-xs font-medium text-primary-700 shadow-sm dark:bg-dark-700/80 dark:text-primary-200">
              {{ text.badge }}
            </div>
            <h1 class="text-3xl font-bold tracking-tight text-gray-950 dark:text-white">{{ text.title }}</h1>
            <p class="mt-2 max-w-2xl text-sm leading-6 text-gray-500 dark:text-dark-300">{{ text.subtitle }}</p>
          </div>
          <router-link
            to="/image-workbench"
            class="inline-flex items-center justify-center rounded-2xl bg-primary-600 px-5 py-3 text-sm font-semibold text-white shadow-sm transition hover:bg-primary-700"
          >
            {{ text.goGenerate }}
          </router-link>
        </div>
      </section>

      <section class="rounded-3xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-800">
        <div class="grid gap-3 lg:grid-cols-[220px_minmax(0,1fr)_auto]">
          <label class="block">
            <span class="mb-2 block text-xs font-medium text-gray-500 dark:text-dark-300">{{ text.statusFilter }}</span>
            <Select v-model="filters.status" :options="statusOptions" class="w-full" @change="loadLogs(1)" />
          </label>
          <label class="block min-w-0">
            <span class="mb-2 block text-xs font-medium text-gray-500 dark:text-dark-300">{{ text.modelSearch }}</span>
            <input
              v-model.trim="filters.model"
              class="input h-[46px] w-full"
              :placeholder="text.searchPlaceholder"
              @keyup.enter="loadLogs(1)"
            />
          </label>
          <div class="flex items-end gap-2">
            <button
              class="h-[46px] rounded-xl bg-gray-900 px-5 text-sm font-semibold text-white transition hover:bg-gray-800 dark:bg-primary-600 dark:hover:bg-primary-700"
              @click="loadLogs(1)"
            >
              {{ text.search }}
            </button>
            <button
              class="h-[46px] rounded-xl border border-gray-200 px-4 text-sm text-gray-600 transition hover:bg-gray-50 dark:border-dark-600 dark:text-dark-200 dark:hover:bg-dark-700"
              @click="resetFilters"
            >
              {{ text.reset }}
            </button>
          </div>
        </div>
      </section>

      <div v-if="error" class="rounded-2xl bg-red-50 p-4 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300">{{ error }}</div>

      <section class="overflow-hidden rounded-3xl border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-800">
        <div class="flex items-center justify-between border-b border-gray-100 px-5 py-4 dark:border-dark-700">
          <div>
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ text.listTitle }}</h2>
            <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ text.totalPrefix }} {{ pagination.total }} {{ text.totalSuffix }}</p>
          </div>
          <button
            class="rounded-xl border border-gray-200 px-4 py-2 text-sm text-gray-600 transition hover:bg-gray-50 dark:border-dark-600 dark:text-dark-200 dark:hover:bg-dark-700"
            :disabled="loading"
            @click="loadLogs()"
          >
            {{ loading ? text.loading : text.refresh }}
          </button>
        </div>

        <div class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-100 dark:divide-dark-700">
            <thead class="bg-gray-50/80 dark:bg-dark-900/60">
              <tr>
                <th class="table-th w-[180px]">{{ text.time }}</th>
                <th class="table-th w-[240px]">{{ text.modelKey }}</th>
                <th class="table-th min-w-[320px]">{{ text.prompt }}</th>
                <th class="table-th w-[120px]">{{ text.status }}</th>
                <th class="table-th w-[220px]">{{ text.result }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 bg-white dark:divide-dark-700 dark:bg-dark-800">
              <tr v-if="loading">
                <td colspan="5" class="px-5 py-14 text-center text-sm text-gray-500">{{ text.loading }}</td>
              </tr>
              <tr v-else-if="logs.length === 0">
                <td colspan="5" class="px-5 py-14 text-center">
                  <div class="mx-auto max-w-sm">
                    <div class="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-2xl bg-gray-100 text-xl dark:bg-dark-700">???</div>
                    <p class="text-sm font-medium text-gray-700 dark:text-dark-200">{{ text.emptyTitle }}</p>
                    <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ text.emptyDesc }}</p>
                  </div>
                </td>
              </tr>
              <tr v-for="log in logs" :key="log.id" class="align-top transition hover:bg-primary-50/40 dark:hover:bg-dark-700/50">
                <td class="whitespace-nowrap px-5 py-4 text-sm text-gray-600 dark:text-dark-300">{{ formatTime(log.created_at) }}</td>
                <td class="px-5 py-4 text-sm">
                  <div class="font-semibold text-gray-900 dark:text-white">{{ log.model || '-' }}</div>
                  <div class="mt-1 text-xs text-gray-500">{{ log.api_key_name || `${text.keyPrefix}${log.api_key_id}` }}</div>
                  <div v-if="log.group_name" class="mt-1 inline-flex rounded-full bg-gray-100 px-2 py-0.5 text-xs text-gray-500 dark:bg-dark-700 dark:text-dark-300">{{ log.group_name }}</div>
                </td>
                <td class="max-w-xl px-5 py-4 text-sm text-gray-700 dark:text-dark-200">
                  <div class="line-clamp-3 break-words leading-6">{{ log.prompt || '-' }}</div>
                  <div v-if="log.error_message" class="mt-2 rounded-lg bg-red-50 px-3 py-2 text-xs text-red-600 dark:bg-red-900/20 dark:text-red-300">{{ log.error_message }}</div>
                </td>
                <td class="px-5 py-4 text-sm">
                  <span :class="statusClass(log.status)" class="inline-flex items-center rounded-full px-3 py-1 text-xs font-semibold">
                    {{ statusText(log.status) }}
                  </span>
                </td>
                <td class="px-5 py-4 text-sm">
                  <div v-if="log.image_urls && log.image_urls.length" class="flex flex-wrap gap-2">
                    <a
                      v-for="url in log.image_urls"
                      :key="url"
                      :href="url"
                      target="_blank"
                      class="group block h-16 w-16 overflow-hidden rounded-xl border border-gray-200 bg-gray-50 transition hover:scale-105 hover:border-primary-300 dark:border-dark-600 dark:bg-dark-700"
                    >
                      <img :src="url" class="h-full w-full object-cover" :alt="text.generatedImageAlt" />
                    </a>
                  </div>
                  <div v-else-if="log.task_id" class="max-w-[180px] truncate rounded-lg bg-gray-50 px-3 py-2 text-xs text-gray-500 dark:bg-dark-900/50 dark:text-dark-300" :title="log.task_id">
                    {{ text.task }}?{{ log.task_id }}
                  </div>
                  <span v-else class="text-xs text-gray-400">-</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <div class="flex flex-col gap-3 border-t border-gray-100 px-5 py-4 text-sm text-gray-500 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between">
          <span>{{ text.totalPrefix }} {{ pagination.total }} {{ text.totalSuffix }}</span>
          <div class="flex items-center gap-2">
            <button class="page-btn" :disabled="pagination.page <= 1" @click="loadLogs(pagination.page - 1)">{{ text.prev }}</button>
            <span class="px-2 py-1">{{ text.pagePrefix }} {{ pagination.page }} {{ text.pageSuffix }}</span>
            <button class="page-btn" :disabled="pagination.page * pagination.page_size >= pagination.total" @click="loadLogs(pagination.page + 1)">{{ text.next }}</button>
          </div>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import AppLayout from '@/components/layout/AppLayout.vue'
import Select from '@/components/common/Select.vue'
import { onMounted, reactive, ref } from 'vue'
import { listImageWorkbenchLogs, type ImageWorkbenchLog } from '@/api/imageWorkbench'

const text = {
  badge: '生图任务中心',
  title: '生图记录',
  subtitle: '查看通过生图工作台提交的任务、出图结果和失败原因。',
  goGenerate: '去生图',
  statusFilter: '状态筛选',
  modelSearch: '模型搜索',
  searchPlaceholder: '输入模型名称，例如 gpt-image-2-1k',
  search: '搜索',
  reset: '重置',
  listTitle: '记录列表',
  totalPrefix: '共',
  totalSuffix: '条记录',
  refresh: '刷新',
  loading: '加载中...',
  time: '时间',
  modelKey: '模型 / 密钥',
  prompt: '提示词',
  status: '状态',
  result: '结果',
  emptyTitle: '暂无生图记录',
  emptyDesc: '提交生图任务后，这里会显示记录和结果图。',
  keyPrefix: 'Key #',
  generatedImageAlt: '生成图片',
  task: '任务',
  prev: '上一页',
  next: '下一页',
  pagePrefix: '第',
  pageSuffix: '页',
  loadFailed: '加载失败',
  allStatus: '全部状态',
  submitted: '已提交',
  completed: '已完成',
  queued: '排队中',
  inProgress: '生成中',
  failed: '失败',
  unknown: '未知状态'
}

const logs = ref<ImageWorkbenchLog[]>([])
const loading = ref(false)
const error = ref('')
const filters = reactive({ status: '', model: '' })
const statusOptions = [
  { value: '', label: text.allStatus },
  { value: 'submitted', label: text.submitted },
  { value: 'queued', label: text.queued },
  { value: 'in_progress', label: text.inProgress },
  { value: 'completed', label: text.completed },
  { value: 'failed', label: text.failed }
]
const pagination = reactive({ page: 1, page_size: 20, total: 0 })

function formatTime(value: string) {
  if (!value) return '-'
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}

function statusText(status: string) {
  const normalized = (status || '').trim()
  const map: Record<string, string> = {
    submitted: text.submitted,
    queued: text.queued,
    in_progress: text.inProgress,
    completed: text.completed,
    failed: text.failed
  }
  return map[normalized] || normalized || text.unknown
}

function statusClass(status: string) {
  if (status === 'completed') return 'bg-emerald-50 text-emerald-700 ring-1 ring-emerald-100 dark:bg-emerald-900/30 dark:text-emerald-300 dark:ring-emerald-800'
  if (status === 'failed') return 'bg-red-50 text-red-700 ring-1 ring-red-100 dark:bg-red-900/30 dark:text-red-300 dark:ring-red-800'
  if (status === 'queued') return 'bg-amber-50 text-amber-700 ring-1 ring-amber-100 dark:bg-amber-900/30 dark:text-amber-300 dark:ring-amber-800'
  return 'bg-blue-50 text-blue-700 ring-1 ring-blue-100 dark:bg-blue-900/30 dark:text-blue-300 dark:ring-blue-800'
}

function resetFilters() {
  filters.status = ''
  filters.model = ''
  loadLogs(1)
}

async function loadLogs(page = pagination.page) {
  loading.value = true
  error.value = ''
  try {
    const res = await listImageWorkbenchLogs({
      page,
      page_size: pagination.page_size,
      status: filters.status || undefined,
      model: filters.model || undefined
    })
    logs.value = res.data
    pagination.page = res.pagination.page
    pagination.page_size = res.pagination.page_size
    pagination.total = res.pagination.total
  } catch (e: any) {
    error.value = e?.response?.data?.error || e?.message || text.loadFailed
  } finally {
    loading.value = false
  }
}

onMounted(() => loadLogs(1))
</script>

<style scoped>
.table-th {
  @apply px-5 py-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-dark-300;
}

.page-btn {
  @apply rounded-xl border border-gray-200 px-3 py-1.5 text-sm transition hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50 dark:border-dark-600 dark:hover:bg-dark-700;
}
</style>
