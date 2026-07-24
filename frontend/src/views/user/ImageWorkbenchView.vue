<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">{{ text.title }}</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-300">{{ text.subtitle }}</p>
        </div>
        <router-link
          to="/image-logs"
          class="rounded-lg border border-gray-200 px-4 py-2 text-sm text-gray-700 hover:bg-gray-50 dark:border-dark-600 dark:text-dark-200 dark:hover:bg-dark-700"
        >
          {{ text.viewLogs }}
        </router-link>
      </div>

      <div v-if="loadError" class="rounded-lg bg-red-50 p-3 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300">
        {{ loadError }}
      </div>

      <div class="grid min-w-0 gap-6 xl:grid-cols-[380px_minmax(0,1fr)] 2xl:grid-cols-[420px_minmax(0,1fr)]">
        <aside class="min-w-0 rounded-2xl border border-gray-200 bg-white p-6 shadow-sm dark:border-dark-700 dark:bg-dark-800">
          <div class="space-y-6">
            <label class="block">
              <span class="mb-2 block text-sm font-medium text-gray-700 dark:text-dark-200">{{ text.apiKey }}</span>
              <Select
                v-model="form.apiKeyId"
                :options="apiKeyOptions"
                :disabled="loadingKeys"
                searchable
                class="w-full"
              />
              <span class="mt-2 block text-xs text-gray-500 dark:text-dark-400">{{ text.apiKeyHelp }}</span>
            </label>

            <section>
              <div class="mb-3 text-sm font-medium text-gray-700 dark:text-dark-200">{{ text.modelTier }}</div>
              <div class="grid grid-cols-2 gap-3">
                <button
                  v-for="preset in modelPresets"
                  :key="preset.model"
                  type="button"
                  class="choice-btn"
                  :class="form.model === preset.model && 'choice-btn-active'"
                  @click="applyModelPreset(preset)"
                >
                  {{ preset.label }}
                </button>
              </div>
            </section>

            <section>
              <div class="mb-3 text-sm font-medium text-gray-700 dark:text-dark-200">{{ text.mode }}</div>
              <div class="grid grid-cols-2 gap-3">
                <button
                  type="button"
                  class="choice-btn"
                  :class="form.mode === 'text' && 'choice-btn-active'"
                  @click="form.mode = 'text'"
                >
                  {{ text.textToImage }}
                </button>
                <button
                  type="button"
                  class="choice-btn"
                  :class="form.mode === 'image' && 'choice-btn-active'"
                  @click="form.mode = 'image'"
                >
                  {{ text.imageToImage }}
                </button>
              </div>
            </section>

            <section>
              <div class="mb-3 text-sm font-medium text-gray-700 dark:text-dark-200">{{ text.aspectRatio }}</div>
              <div class="grid grid-cols-4 gap-3">
                <button
                  v-for="ratio in aspectRatios"
                  :key="ratio"
                  type="button"
                  class="choice-btn"
                  :class="form.aspectRatio === ratio && 'choice-btn-active'"
                  @click="form.aspectRatio = ratio"
                >
                  {{ ratio }}
                </button>
              </div>
            </section>

            <div class="grid grid-cols-2 gap-4">
              <label class="block">
                <span class="mb-2 block text-sm font-medium text-gray-700 dark:text-dark-200">{{ text.quality }}</span>
                <Select v-model="form.quality" :options="qualityOptions" class="w-full" />
              </label>
              <label class="block">
                <span class="mb-2 block text-sm font-medium text-gray-700 dark:text-dark-200">{{ text.count }}</span>
                <Select v-model="form.n" :options="countOptions" class="w-full" />
              </label>
            </div>

            <div class="rounded-xl bg-gray-50 p-4 text-xs leading-6 text-gray-500 dark:bg-dark-900/40 dark:text-dark-400">
              {{ text.asyncHelp }}
            </div>
          </div>
        </aside>

        <main class="min-w-0 rounded-2xl border border-gray-200 bg-white p-6 shadow-sm dark:border-dark-700 dark:bg-dark-800">
          <form class="space-y-5" @submit.prevent="submit">
            <label class="block min-w-0">
              <div class="mb-2 flex items-center justify-between">
                <span class="text-sm font-medium text-gray-700 dark:text-dark-200">{{ text.prompt }}</span>
                <span class="text-xs text-gray-500 dark:text-dark-400">{{ text.oneTask }}</span>
              </div>
              <textarea
                v-model="form.prompt"
                class="input min-h-[320px] w-full max-w-full resize-y overflow-x-hidden whitespace-pre-wrap break-words text-base leading-7"
                wrap="soft"
                :placeholder="text.promptPlaceholder"
                required
              />
              <p class="mt-2 text-xs text-gray-500 dark:text-dark-400">{{ text.promptHelp }}</p>
            </label>

            <label v-if="form.mode === 'image'" class="block min-w-0">
              <span class="mb-2 block text-sm font-medium text-gray-700 dark:text-dark-200">{{ text.refImageUrl }}</span>
              <textarea
                v-model.trim="form.imageUrlsText"
                class="input min-h-24 w-full max-w-full resize-y overflow-x-hidden break-words"
                placeholder="https://example.com/ref.png"
              />
              <p class="mt-2 text-xs text-gray-500 dark:text-dark-400">{{ text.refImageHelp }}</p>
            </label>

            <div class="flex flex-wrap items-center gap-3">
              <button
                class="rounded-xl bg-primary-600 px-6 py-3 text-sm font-semibold text-white shadow-sm hover:bg-primary-700 disabled:cursor-not-allowed disabled:opacity-60"
                :disabled="submitting || imageKeys.length === 0 || !promptText"
              >
                {{ submitting ? text.generating : text.generate }}
              </button>
              <span class="text-xs text-gray-500 dark:text-dark-400">
                {{ text.current }}{{ selectedPresetLabel }} / {{ form.aspectRatio }} / {{ form.n }} {{ text.sheet }}
              </span>
            </div>
          </form>

          <section class="mt-6 rounded-xl border border-gray-200 bg-gray-50 p-4 dark:border-dark-700 dark:bg-dark-900/40">
            <div class="mb-3 flex items-center justify-between">
              <h2 class="text-sm font-semibold text-gray-800 dark:text-dark-100">{{ text.result }}</h2>
              <button
                v-if="results.length"
                type="button"
                class="text-xs text-gray-500 hover:text-gray-700 dark:text-dark-400 dark:hover:text-dark-200"
                @click="clearResults"
              >
                {{ text.clearResult }}
              </button>
            </div>

            <div v-if="error" class="mb-3 rounded-lg bg-red-50 p-3 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300">
              {{ error }}
            </div>

            <div v-if="results.length" class="space-y-4">
              <article
                v-for="item in results"
                :key="item.id"
                class="rounded-lg border border-gray-200 bg-white p-3 dark:border-dark-700 dark:bg-dark-800"
              >
                <div class="mb-2 flex items-start justify-between gap-3">
                  <div class="min-w-0">
                    <div class="line-clamp-2 break-words text-sm font-medium text-gray-900 dark:text-white">{{ item.prompt }}</div>
                    <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ item.status }}</div>
                  </div>
                  <span v-if="item.taskId" class="shrink-0 text-xs text-blue-600 dark:text-blue-300">{{ text.task }} {{ item.taskId }}</span>
                </div>
                <div v-if="item.error" class="rounded bg-red-50 p-2 text-xs text-red-700 dark:bg-red-900/20 dark:text-red-300">
                  {{ item.error }}
                </div>
                <div v-else-if="item.urls.length" class="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
                  <a
                    v-for="url in item.urls"
                    :key="url"
                    :href="url"
                    target="_blank"
                    class="block overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800"
                  >
                    <img :src="url" class="h-auto w-full object-contain" :alt="text.generatedImageAlt" />
                  </a>
                </div>
              </article>
            </div>
            <p v-else class="text-sm text-gray-500 dark:text-dark-400">{{ text.emptyResult }}</p>
          </section>
        </main>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import AppLayout from '@/components/layout/AppLayout.vue'
import Select from '@/components/common/Select.vue'
import { computed, onMounted, reactive, ref } from 'vue'
import keysAPI from '@/api/keys'
import { generateImage, pollImageTask } from '@/api/imageWorkbench'
import type { ApiKey } from '@/types'

type WorkbenchMode = 'text' | 'image'

interface ModelPreset {
  label: string
  model: string
  outputResolution?: string
  imageSize?: string
  size?: string
}

interface ResultItem {
  id: string
  prompt: string
  status: string
  urls: string[]
  taskId?: string
  error?: string
}

const text = {
  title: '生图工作台',
  subtitle: '选择生图密钥，填写提示词，提交后自动等待出图。',
  viewLogs: '查看生图记录',
  apiKey: '生图密钥',
  apiKeyHelp: '只显示已绑定且分组开启“允许图片生成”的 API Key。',
  modelTier: '模型档位',
  mode: '生成方式',
  textToImage: '文生图',
  imageToImage: '图生图',
  aspectRatio: '画面比例',
  quality: '质量',
  count: '出图数量',
  asyncHelp: '已默认使用异步生成：提交后页面会自动查询任务状态，完成后直接显示图片。',
  prompt: '提示词',
  oneTask: '整段作为一个任务',
  promptPlaceholder: '请输入完整提示词，可以换行描述细节；系统会把整段内容作为一个任务提交，不会按行拆分。',
  promptHelp: '换行只是为了排版，不会拆成多个任务。',
  refImageUrl: '参考图 URL',
  refImageHelp: '多张参考图可换行填写，最多 9 张。',
  generating: '生成中...',
  generate: '生成图片',
  current: '当前：',
  sheet: '张',
  result: '生成结果',
  clearResult: '清空结果',
  task: '任务',
  generatedImageAlt: '生成图片',
  emptyResult: '结果会显示在这里。',
  loadingKeys: '加载 API Key 中...',
  selectImageKey: '请选择生图密钥',
  noImageKey: '暂无可用生图密钥，请到分组开启允许图片生成',
  noApiKey: '暂无 API Key，请先创建密钥',
  unnamedGroup: '未命名分组',
  defaultQuality: '默认',
  lowQuality: '低',
  mediumQuality: '中',
  highQuality: '高',
  loadKeysFailed: '加载 API Key 失败',
  fillPrompt: '请填写提示词',
  submitting: '提交中...',
  completed: '已完成',
  upstreamFailed: '上游生成失败',
  failed: '失败',
  inProgress: '生成中...',
  queued: '排队中...',
  stillGenerating: '仍在生成中，请稍后到生图记录查看',
  submittedWait: '已提交，等待出图...',
  returned: '已返回',
  generateFailed: '生成失败'
}

const keys = ref<ApiKey[]>([])
const loadError = ref('')
const loadingKeys = ref(false)
const submitting = ref(false)
const error = ref('')
const results = ref<ResultItem[]>([])

const form = reactive({
  apiKeyId: 0,
  model: 'nano-banana-pro-4k',
  mode: 'text' as WorkbenchMode,
  prompt: '',
  aspectRatio: '1:1',
  outputResolution: '4K',
  imageSize: '4K',
  size: '',
  quality: '',
  n: 1,
  imageUrlsText: ''
})

const modelPresets: ModelPreset[] = [
  { label: 'Banana 1K', model: 'nano-banana-pro-1k', outputResolution: '1K', imageSize: '1K' },
  { label: 'Banana 2K', model: 'nano-banana-pro-2k', outputResolution: '2K', imageSize: '2K' },
  { label: 'Banana 4K', model: 'nano-banana-pro-4k', outputResolution: '4K', imageSize: '4K' },
  { label: 'GPT 1K', model: 'gpt-image-2-1k', size: '1024x1024' },
  { label: 'GPT 2K', model: 'gpt-image-2-2k', size: '2048x2048' },
  { label: 'GPT 4K', model: 'gpt-image-2-4k', size: '3840x2160' }
]
const aspectRatios = ['1:1', '3:2', '2:3', '4:3', '3:4', '16:9', '9:16', '21:9']

const activeKeys = computed(() => keys.value.filter(k => k.status === 'active'))
const imageKeys = computed(() => activeKeys.value.filter(k => k.group?.allow_image_generation))
const apiKeyPlaceholder = computed(() => {
  if (loadingKeys.value) return text.loadingKeys
  if (imageKeys.value.length > 0) return text.selectImageKey
  if (activeKeys.value.length > 0) return text.noImageKey
  return text.noApiKey
})
const apiKeyOptions = computed(() => [
  { value: 0, label: apiKeyPlaceholder.value, disabled: true },
  ...imageKeys.value.map(key => ({
    value: key.id,
    label: `${key.name} / ${key.group?.name || text.unnamedGroup}`
  }))
])
const qualityOptions = [
  { value: '', label: text.defaultQuality },
  { value: 'low', label: text.lowQuality },
  { value: 'medium', label: text.mediumQuality },
  { value: 'high', label: text.highQuality }
]
const countOptions = [1, 2, 3, 4].map(value => ({ value, label: `${value} ${text.sheet}` }))
const promptText = computed(() => form.prompt.trim())
const selectedPresetLabel = computed(() => modelPresets.find(item => item.model === form.model)?.label || form.model)

function applyModelPreset(preset: ModelPreset) {
  form.model = preset.model
  form.outputResolution = preset.outputResolution || ''
  form.imageSize = preset.imageSize || ''
  form.size = preset.size || ''
}

async function loadKeys() {
  loadingKeys.value = true
  try {
    const res = await keysAPI.list(1, 100, { status: 'active' })
    keys.value = res.items
    if (!form.apiKeyId && imageKeys.value.length) {
      form.apiKeyId = imageKeys.value[0].id
    }
  } catch (e: any) {
    loadError.value = e?.response?.data?.error || e?.message || text.loadKeysFailed
  } finally {
    loadingKeys.value = false
  }
}

function buildPayload(prompt: string): Record<string, unknown> {
  const payload: Record<string, unknown> = {
    model: form.model,
    prompt,
    async: true,
    stream: false
  }
  if (form.aspectRatio) payload.aspect_ratio = form.aspectRatio
  if (form.outputResolution) payload.output_resolution = form.outputResolution
  if (form.imageSize) payload.image_size = form.imageSize
  if (form.size) payload.size = form.size
  if (form.quality) payload.quality = form.quality
  if (form.n > 1) payload.n = form.n

  if (form.mode === 'image') {
    const refs = form.imageUrlsText.split('\n').map(s => s.trim()).filter(Boolean).slice(0, 9)
    if (refs.length === 1) payload.image = refs[0]
    if (refs.length > 1) payload.images = refs
  }
  return payload
}

function extractImages(resp: Record<string, unknown>): string[] {
  const data = Array.isArray(resp.data) ? resp.data : []
  return data.flatMap((item: any) => {
    if (item?.url) return [String(item.url)]
    if (item?.b64_json) return [`data:image/png;base64,${item.b64_json}`]
    return []
  })
}

function extractTaskID(resp: Record<string, unknown>): string {
  return String((resp.task_id || resp.id || '') as string)
}

function extractStatus(resp: Record<string, unknown>): string {
  return String((resp.status || '') as string)
}

function extractError(e: any): string {
  const message = String(e?.response?.data?.error?.message || e?.response?.data?.error || e?.message || text.generateFailed)
  if (message.includes('images endpoint requires an image model')) {
    return '当前模型没有被后端识别为生图模型，我已放宽兼容规则；请刷新后重试。'
  }
  if (message.includes('Request failed with status code 400')) {
    return '请求参数有误，请检查模型、尺寸或账号配置后重试。'
  }
  return message
}

function clearResults() {
  results.value = []
}

async function waitForTask(item: ResultItem) {
  if (!item.taskId) return
  const maxAttempts = 80
  for (let attempt = 0; attempt < maxAttempts; attempt += 1) {
    await new Promise(resolve => window.setTimeout(resolve, attempt === 0 ? 1500 : 3500))
    const resp = await pollImageTask(item.taskId, form.apiKeyId)
    const status = extractStatus(resp)
    item.urls = extractImages(resp)
    if (item.urls.length) {
      item.status = text.completed
      return
    }
    if (status === 'failed') {
      item.status = text.failed
      item.error = (resp.error as any)?.message || text.upstreamFailed
      return
    }
    item.status = status === 'in_progress' ? text.inProgress : text.queued
  }
  item.status = text.stillGenerating
}

async function submit() {
  error.value = ''
  if (!form.apiKeyId) {
    error.value = text.selectImageKey
    return
  }
  const prompt = promptText.value
  if (!prompt) {
    error.value = text.fillPrompt
    return
  }

  submitting.value = true
  const item: ResultItem = {
    id: `${Date.now()}`,
    prompt,
    status: text.submitting,
    urls: []
  }
  results.value = [item]

  try {
    const resp = await generateImage({ api_key_id: form.apiKeyId, payload: buildPayload(prompt) })
    item.urls = extractImages(resp)
    item.taskId = extractTaskID(resp)
    if (item.urls.length) {
      item.status = text.completed
      return
    }
    if (item.taskId) {
      item.status = text.submittedWait
      await waitForTask(item)
      return
    }
    item.status = text.returned
  } catch (e: any) {
    item.error = extractError(e)
    item.status = text.failed
  } finally {
    submitting.value = false
  }
}

onMounted(loadKeys)
</script>

<style scoped>
.choice-btn {
  @apply rounded-xl border border-gray-200 bg-white px-4 py-3 text-center text-sm font-medium text-gray-700 transition hover:border-primary-300 hover:bg-primary-50 dark:border-dark-600 dark:bg-dark-800 dark:text-dark-200 dark:hover:border-primary-500/60 dark:hover:bg-primary-900/20;
}

.choice-btn-active {
  @apply border-primary-500 bg-primary-50 text-primary-700 shadow-sm dark:border-primary-500 dark:bg-primary-900/30 dark:text-primary-200;
}
</style>
