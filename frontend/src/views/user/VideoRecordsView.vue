<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="rounded-3xl border border-gray-200 bg-white p-6 shadow-sm dark:border-dark-700 dark:bg-dark-800">
        <div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <div class="mb-3 inline-flex items-center rounded-full bg-primary-50 px-3 py-1 text-xs font-medium text-primary-700 dark:bg-primary-900/30 dark:text-primary-200">
              视频任务记录
            </div>
            <h1 class="text-2xl font-bold tracking-tight text-gray-950 dark:text-white">视频记录</h1>
            <p class="mt-2 text-sm text-gray-500 dark:text-dark-300">查看通过 API 提交的视频生成任务、生成结果、扣费和失败退费情况。</p>
          </div>
          <button class="rounded-xl border border-gray-200 px-4 py-2 text-sm text-gray-600 transition hover:bg-gray-50 disabled:opacity-60 dark:border-dark-600 dark:text-dark-200 dark:hover:bg-dark-700" :disabled="loading" @click="loadRecords()">
            {{ loading ? '刷新中...' : '刷新' }}
          </button>
        </div>
      </section>

      <section class="rounded-3xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-800">
        <div class="grid gap-3 lg:grid-cols-[220px_minmax(0,1fr)_auto]">
          <label class="block">
            <span class="mb-2 block text-xs font-medium text-gray-500 dark:text-dark-300">状态筛选</span>
            <Select v-model="filters.status" :options="statusOptions" class="w-full" @change="loadRecords(1)" />
          </label>
          <label class="block min-w-0">
            <span class="mb-2 block text-xs font-medium text-gray-500 dark:text-dark-300">模型搜索</span>
            <input v-model.trim="filters.model" class="input h-[46px] w-full" placeholder="输入模型名，例如 sd2_full_no_real_multi_res" @keyup.enter="loadRecords(1)" />
          </label>
          <div class="flex items-end gap-2">
            <button class="h-[46px] rounded-xl bg-gray-900 px-5 text-sm font-semibold text-white transition hover:bg-gray-800 dark:bg-primary-600 dark:hover:bg-primary-700" @click="loadRecords(1)">搜索</button>
            <button class="h-[46px] rounded-xl border border-gray-200 px-4 text-sm text-gray-600 transition hover:bg-gray-50 dark:border-dark-600 dark:text-dark-200 dark:hover:bg-dark-700" @click="resetFilters">重置</button>
          </div>
        </div>
      </section>

      <div v-if="error" class="rounded-2xl bg-red-50 p-4 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300">{{ error }}</div>

      <section class="overflow-hidden rounded-3xl border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-800">
        <div class="flex items-center justify-between border-b border-gray-100 px-5 py-4 dark:border-dark-700">
          <div>
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">任务列表</h2>
            <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">共 {{ pagination.total }} 条记录</p>
          </div>
        </div>

        <div class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-100 dark:divide-dark-700">
            <thead class="bg-gray-50/80 dark:bg-dark-900/60">
              <tr>
                <th class="table-th w-[170px]">时间</th>
                <th class="table-th w-[220px]">模型</th>
                <th class="table-th min-w-[300px]">提示词</th>
                <th class="table-th w-[120px]">状态</th>
                <th class="table-th w-[110px]">耗时</th>
                <th class="table-th w-[180px]">扣费/退费</th>
                <th class="table-th w-[170px]">结果</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 bg-white dark:divide-dark-700 dark:bg-dark-800">
              <tr v-if="loading">
                <td colspan="7" class="px-5 py-14 text-center text-sm text-gray-500">加载中...</td>
              </tr>
              <tr v-else-if="records.length === 0">
                <td colspan="7" class="px-5 py-14 text-center text-sm text-gray-500">暂无视频记录</td>
              </tr>
              <tr v-for="item in records" :key="item.id" class="align-top transition hover:bg-primary-50/40 dark:hover:bg-dark-700/50">
                <td class="whitespace-nowrap px-5 py-4 text-sm text-gray-600 dark:text-dark-300">{{ formatTime(item.created_at) }}</td>
                <td class="px-5 py-4 text-sm">
                  <div class="font-semibold text-gray-900 dark:text-white">{{ item.model || '-' }}</div>
                </td>
                <td class="max-w-xl px-5 py-4 text-sm text-gray-700 dark:text-dark-200">
                  <div class="line-clamp-3 break-words leading-6">{{ item.prompt || '-' }}</div>
                  <div class="mt-2 flex flex-wrap gap-2 text-xs text-gray-500">
                    <span v-if="item.ratio" class="tag">{{ item.ratio }}</span>
                    <span v-if="item.resolution" class="tag">{{ item.resolution }}</span>
                    <span v-if="item.duration_seconds" class="tag">{{ item.duration_seconds }} 秒</span>
                  </div>
                  <div v-if="item.error_message" class="mt-2 rounded-lg bg-red-50 px-3 py-2 text-xs text-red-600 dark:bg-red-900/20 dark:text-red-300">{{ item.error_message }}</div>
                </td>
                <td class="px-5 py-4 text-sm">
                  <span :class="statusClass(item.status)" class="inline-flex items-center rounded-full px-3 py-1 text-xs font-semibold">{{ statusText(item.status) }}</span>
                </td>
                <td class="whitespace-nowrap px-5 py-4 text-sm text-gray-600 dark:text-dark-300">{{ formatElapsed(item) }}</td>
                <td class="px-5 py-4 text-sm text-gray-700 dark:text-dark-200">
                  <div>扣费：{{ money(item.cost) }}</div>
                  <div v-if="item.refund_amount" class="mt-1 text-emerald-600 dark:text-emerald-300">已退：{{ money(item.refund_amount) }}</div>
                  <div v-else-if="item.status === 'failed'" class="mt-1 text-xs text-gray-400">未产生退费或费用为 0</div>
                </td>
                <td class="px-5 py-4 text-sm">
                  <div v-if="item.video_url || item.download_url" class="flex flex-col items-start gap-2">
                    <button v-if="item.video_url" class="link-btn" :disabled="openingTask === item.task_id" @click="openContent(item, false)">播放链接</button>
                    <button v-if="item.download_url" class="link-btn" :disabled="openingTask === item.task_id" @click="openContent(item, true)">下载链接</button>
                  </div>
                  <div v-else class="max-w-[220px] truncate rounded-lg bg-gray-50 px-3 py-2 text-xs text-gray-500 dark:bg-dark-900/50 dark:text-dark-300" :title="item.task_id">
                    任务：{{ item.task_id }}
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <div class="flex flex-col gap-3 border-t border-gray-100 px-5 py-4 text-sm text-gray-500 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between">
          <span>共 {{ pagination.total }} 条</span>
          <div class="flex items-center gap-2">
            <button class="page-btn" :disabled="pagination.page <= 1" @click="loadRecords(pagination.page - 1)">上一页</button>
            <span class="px-2 py-1">第 {{ pagination.page }} 页</span>
            <button class="page-btn" :disabled="pagination.page * pagination.page_size >= pagination.total" @click="loadRecords(pagination.page + 1)">下一页</button>
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
import { fetchVideoRecordContent, listVideoRecords, type VideoRecord } from '@/api/videoRecords'

const statusOptions = [
  { value: 'all', label: '全部状态' },
  { value: 'queued', label: '排队中' },
  { value: 'processing', label: '生成中' },
  { value: 'succeeded', label: '已完成' },
  { value: 'failed', label: '失败' },
]

const records = ref<VideoRecord[]>([])
const loading = ref(false)
const error = ref('')
const openingTask = ref('')
const filters = reactive({ status: 'all', model: '' })
const pagination = reactive({ page: 1, page_size: 20, total: 0 })

function statusText(status: string) {
  const map: Record<string, string> = {
    queued: '排队中',
    processing: '生成中',
    in_progress: '生成中',
    succeeded: '已完成',
    completed: '已完成',
    failed: '失败',
  }
  return map[status] || status || '-'
}

function statusClass(status: string) {
  if (status === 'succeeded' || status === 'completed') return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300'
  if (status === 'failed') return 'bg-red-50 text-red-700 dark:bg-red-900/20 dark:text-red-300'
  if (status === 'processing' || status === 'in_progress') return 'bg-blue-50 text-blue-700 dark:bg-blue-900/20 dark:text-blue-300'
  return 'bg-amber-50 text-amber-700 dark:bg-amber-900/20 dark:text-amber-300'
}

function formatTime(value: string) {
  if (!value) return '-'
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}

function money(value?: number | null) {
  const n = Number(value || 0)
  return n.toFixed(4)
}

function formatElapsed(item: VideoRecord) {
  let seconds = Number(item.elapsed_seconds || 0)
  if (!seconds && item.created_at && item.updated_at) {
    const diff = Math.floor((new Date(item.updated_at).getTime() - new Date(item.created_at).getTime()) / 1000)
    if (Number.isFinite(diff) && diff > 0) seconds = diff
  }
  if (!seconds) return '-'
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = seconds % 60
  if (h > 0) return `${h}小时${m}分${s}秒`
  if (m > 0) return `${m}分${s}秒`
  return `${s}秒`
}

async function openContent(item: VideoRecord, download: boolean) {
  if (!item.task_id) return
  openingTask.value = item.task_id
  try {
    const blob = await fetchVideoRecordContent(item.task_id, download)
    const url = URL.createObjectURL(blob)
    if (download) {
      const a = document.createElement('a')
      a.href = url
      a.download = `${item.task_id}.mp4`
      document.body.appendChild(a)
      a.click()
      a.remove()
      window.setTimeout(() => URL.revokeObjectURL(url), 30_000)
    } else {
      window.open(url, '_blank', 'noopener')
      window.setTimeout(() => URL.revokeObjectURL(url), 10 * 60_000)
    }
  } catch (err: any) {
    error.value = err?.response?.data?.error || err?.message || '打开视频失败'
  } finally {
    openingTask.value = ''
  }
}

async function loadRecords(page = pagination.page) {
  loading.value = true
  error.value = ''
  try {
    const res = await listVideoRecords({ page, page_size: pagination.page_size, status: filters.status, model: filters.model })
    records.value = res.data
    pagination.page = res.pagination.page
    pagination.page_size = res.pagination.page_size
    pagination.total = res.pagination.total
  } catch (err: any) {
    error.value = err?.response?.data?.error || err?.message || '加载失败'
  } finally {
    loading.value = false
  }
}

function resetFilters() {
  filters.status = 'all'
  filters.model = ''
  loadRecords(1)
}

onMounted(() => loadRecords(1))
</script>

<style scoped>
.table-th {
  padding: 0.75rem 1.25rem;
  text-align: left;
  font-size: 0.75rem;
  font-weight: 600;
  color: rgb(107 114 128);
}

.page-btn {
  border-radius: 0.75rem;
  border: 1px solid rgb(229 231 235);
  padding: 0.5rem 0.75rem;
  transition: background 0.2s;
}

.page-btn:not(:disabled):hover {
  background: rgb(249 250 251);
}

.page-btn:disabled {
  cursor: not-allowed;
  opacity: 0.45;
}

.tag {
  border-radius: 9999px;
  background: rgb(243 244 246);
  padding: 0.125rem 0.5rem;
}

.link-btn {
  border-radius: 0.75rem;
  background: rgb(239 246 255);
  padding: 0.375rem 0.625rem;
  font-size: 0.75rem;
  color: rgb(29 78 216);
  transition: background 0.2s;
}

.link-btn:not(:disabled):hover {
  background: rgb(219 234 254);
}

.link-btn:disabled {
  cursor: wait;
  opacity: 0.6;
}
</style>
