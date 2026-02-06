<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { speechApi, type ASRModel } from '@/api/speech'

const { t } = useI18n()

const props = defineProps<{
  modelVisible: boolean
  type: 'asr' | 'tts'
}>()

const emit = defineEmits<{
  'update:modelVisible': [value: boolean]
  'downloaded': []
}>()

interface ModelWithStatus extends ASRModel {
  downloaded?: boolean
}

interface DownloadProgress {
  percentage: number
  speed_human: string
  eta: string
  downloaded: number
  total: number
}

const models = ref<ModelWithStatus[]>([])
const downloadingModelId = ref<string | null>(null)
const switchingModelId = ref<string | null>(null)
const progress = ref<DownloadProgress | null>(null)
const error = ref<string | null>(null)
let pollInterval: ReturnType<typeof setInterval> | null = null

const title = computed(() =>
  props.type === 'asr' ? t('speech.prompt.asrTitle') : t('speech.prompt.ttsTitle')
)

const description = computed(() =>
  props.type === 'asr' ? t('speech.prompt.asrDescription') : t('speech.prompt.ttsDescription')
)

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

async function loadModels() {
  try {
    const res = props.type === 'asr'
      ? await speechApi.listASRModels()
      : await speechApi.listTTSModels()
    models.value = res.data?.models || []
  } catch (e) {
    console.error('Failed to load models:', e)
  }
}

async function handleDownload(modelId: string) {
  if (downloadingModelId.value) return

  downloadingModelId.value = modelId
  progress.value = null
  error.value = null

  try {
    if (props.type === 'asr') {
      await speechApi.downloadASRModel(modelId)
    } else {
      await speechApi.downloadTTSModel(modelId)
    }

    pollInterval = setInterval(async () => {
      try {
        const statusRes = props.type === 'asr'
          ? await speechApi.getASRStatus()
          : await speechApi.getTTSStatus()

        if (statusRes.data?.progress) {
          progress.value = statusRes.data.progress
        }

        if (statusRes.data?.ready && !statusRes.data?.downloading) {
          clearPollInterval()
          downloadingModelId.value = null
          emit('update:modelVisible', false)
          emit('downloaded')
        }
      } catch (e) {
        console.error('Status poll error:', e)
      }
    }, 500)
  } catch (e) {
    console.error('Download failed:', e)
    error.value = t('speech.downloadError')
    downloadingModelId.value = null
  }
}

function clearPollInterval() {
  if (pollInterval) {
    clearInterval(pollInterval)
    pollInterval = null
  }
}

function handleClose() {
  clearPollInterval()
  emit('update:modelVisible', false)
}

async function handleCancel() {
  try {
    if (props.type === 'asr') {
      await speechApi.cancelASRDownload()
    }
    clearPollInterval()
    downloadingModelId.value = null
    progress.value = null
  } catch (e) {
    console.error('Cancel failed:', e)
  }
}

async function handleUse(modelId: string) {
  if (switchingModelId.value) return

  switchingModelId.value = modelId
  error.value = null

  try {
    await speechApi.switchASRModel(modelId)
    await loadModels()
    emit('update:modelVisible', false)
    emit('downloaded')
  } catch (e) {
    console.error('Switch failed:', e)
    error.value = t('speech.switchError')
  } finally {
    switchingModelId.value = null
  }
}

watch(() => props.modelVisible, (visible) => {
  if (visible) {
    downloadingModelId.value = null
    progress.value = null
    error.value = null
    loadModels()
  } else {
    clearPollInterval()
  }
})
</script>

<template>
  <Teleport to="body">
    <Transition name="modal">
      <div
        v-if="modelVisible"
        class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50"
        @click.self="handleClose"
      >
        <div class="bg-white dark:bg-gray-800 rounded-xl shadow-xl max-w-md w-full p-6">
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-2">
            {{ title }}
          </h3>
          <p class="text-sm text-gray-600 dark:text-gray-400 mb-4">
            {{ description }}
          </p>

          <!-- Model List -->
          <div class="space-y-2 mb-4 max-h-80 overflow-y-auto">
            <div
              v-for="model in models"
              :key="model.id"
              class="p-3 rounded-lg border transition-colors"
              :class="[
                model.downloaded
                  ? 'bg-green-50 dark:bg-green-900/20 border-green-200 dark:border-green-800'
                  : downloadingModelId === model.id
                    ? 'bg-blue-50 dark:bg-blue-900/20 border-blue-300 dark:border-blue-700'
                    : 'bg-gray-50 dark:bg-gray-700 border-gray-200 dark:border-gray-600'
              ]"
            >
              <div class="flex items-center gap-3">
                <!-- Status icon -->
                <svg v-if="model.downloaded" class="w-5 h-5 text-green-500 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                </svg>
                <svg v-else-if="downloadingModelId === model.id" class="w-5 h-5 text-blue-500 flex-shrink-0 animate-spin" fill="none" viewBox="0 0 24 24">
                  <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                  <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
                <!-- Download icon for not-downloaded models -->
                <svg v-else class="w-5 h-5 text-gray-400 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
                </svg>

                <!-- Model info -->
                <div class="flex-1 min-w-0">
                  <div class="font-medium text-gray-900 dark:text-white">
                    {{ t(model.name) }}
                    <span class="text-xs text-gray-400 ml-1">{{ model.size }}</span>
                    <span v-if="model.active" class="text-xs text-green-600 dark:text-green-400 ml-1">{{ t('speech.inUse') }}</span>
                  </div>
                  <div class="text-xs text-gray-500 dark:text-gray-400 truncate">
                    {{ t(model.description) }}
                  </div>
                </div>

                <!-- Cancel button when downloading -->
                <button
                  v-if="downloadingModelId === model.id"
                  class="px-3 py-1.5 text-sm text-red-600 dark:text-red-400 bg-red-100 dark:bg-red-900/30 rounded-lg hover:bg-red-200 dark:hover:bg-red-900/50 transition-colors"
                  @click="handleCancel"
                >
                  {{ t('common.cancel') }}
                </button>
                <!-- Download button -->
                <button
                  v-if="!model.downloaded && downloadingModelId !== model.id"
                  class="px-3 py-1.5 text-sm text-white bg-accent rounded-lg hover:bg-accent/90 transition-colors disabled:opacity-50"
                  :disabled="!!downloadingModelId"
                  @click="handleDownload(model.id)"
                >
                  {{ t('speech.download') }}
                </button>
                <!-- Use button for downloaded models -->
                <button
                  v-else-if="model.downloaded && !model.active && switchingModelId !== model.id"
                  class="px-3 py-1.5 text-sm text-white bg-green-500 rounded-lg hover:bg-green-600 transition-colors disabled:opacity-50"
                  :disabled="!!switchingModelId"
                  @click="handleUse(model.id)"
                >
                  {{ t('common.use') }}
                </button>
                <!-- Switching indicator -->
                <span v-else-if="switchingModelId === model.id" class="text-blue-500 text-sm flex items-center gap-1">
                  <svg class="animate-spin h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                    <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                    <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                  </svg>
                  {{ t('speech.switching') }}
                </span>
              </div>

              <!-- Progress for this model -->
              <div v-if="downloadingModelId === model.id" class="mt-3">
                <div class="w-full bg-gray-200 dark:bg-gray-600 rounded-full h-2 mb-1">
                  <div
                    class="bg-accent h-2 rounded-full transition-all duration-300"
                    :style="{ width: `${progress?.percentage || 0}%` }"
                  ></div>
                </div>
                <div class="flex justify-between text-xs text-gray-500 dark:text-gray-400">
                  <span>{{ formatBytes(progress?.downloaded || 0) }} / {{ formatBytes(progress?.total || 0) }}</span>
                  <span>{{ progress?.speed_human || '-- MB/s' }} · {{ progress?.eta || '--:--' }}</span>
                </div>
              </div>
            </div>
          </div>

          <!-- Error -->
          <div v-if="error" class="mb-4 p-3 bg-red-100 dark:bg-red-900/30 text-red-600 dark:text-red-400 rounded-lg text-sm">
            {{ error }}
          </div>

          <!-- Close button -->
          <button
            class="w-full px-4 py-2 text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors"
            @click="handleClose"
          >
            {{ t('common.close') }}
          </button>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.modal-enter-active,
.modal-leave-active {
  transition: opacity 0.2s ease;
}
.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}
</style>
