<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { ttsApi, type SherpaModelStatus, type TTSProvider, type SherpaAvailableModel } from '@/api/tts'

const { t } = useI18n()

const loading = ref(false)
const downloading = ref(false)
const error = ref<string | null>(null)

const providers = ref<TTSProvider[]>([])
const defaultProvider = ref('')
const sherpaStatus = ref<SherpaModelStatus | null>(null)
const availableModels = ref<SherpaAvailableModel[]>([])
const selectedModelType = ref('kokoro-en')

// Computed properties
const modelReady = computed(() => sherpaStatus.value?.ready ?? false)
const downloadProgress = computed(() => sherpaStatus.value?.progress ?? sherpaStatus.value?.saved_progress)
const isDownloading = computed(() => sherpaStatus.value?.downloading ?? false)
const hasPendingDownload = computed(() => sherpaStatus.value?.has_pending ?? false)
const pendingModel = computed(() => sherpaStatus.value?.pending_model)

// Load TTS status
async function loadStatus() {
  loading.value = true
  error.value = null
  try {
    const [providersRes, sherpaRes, modelsRes] = await Promise.all([
      ttsApi.listProviders(),
      ttsApi.sherpa.getStatus().catch(() => null),
      ttsApi.sherpa.getAvailableModels().catch(() => null)
    ])
    providers.value = providersRes.data.providers
    defaultProvider.value = providersRes.data.default_provider
    sherpaStatus.value = sherpaRes?.data ?? null
    availableModels.value = modelsRes?.data?.models ?? []

    // If there's a pending download, start polling
    if (sherpaRes?.data?.downloading) {
      pollDownloadStatus()
    }
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    loading.value = false
  }
}

// Download model
async function downloadModel(modelType?: string) {
  downloading.value = true
  error.value = null
  try {
    const type = modelType || selectedModelType.value
    await ttsApi.sherpa.downloadModel(type)
    // Poll for status updates
    pollDownloadStatus()
  } catch (e) {
    error.value = (e as Error).message
    downloading.value = false
  }
}

// Resume pending download
async function resumeDownload() {
  if (!pendingModel.value) return
  await downloadModel(pendingModel.value)
}

// Poll download status
let pollTimer: number | null = null
function pollDownloadStatus() {
  if (pollTimer) clearInterval(pollTimer)
  pollTimer = window.setInterval(async () => {
    try {
      const res = await ttsApi.sherpa.getStatus()
      sherpaStatus.value = res.data
      if (!res.data.downloading) {
        if (pollTimer) clearInterval(pollTimer)
        pollTimer = null
        downloading.value = false
        // Reload models to update downloaded status
        loadModels()
      }
    } catch {
      if (pollTimer) clearInterval(pollTimer)
      pollTimer = null
      downloading.value = false
    }
  }, 2000)
}

// Load available models
async function loadModels() {
  try {
    const res = await ttsApi.sherpa.getAvailableModels()
    availableModels.value = res.data.models
  } catch {
    // Ignore errors
  }
}

// Delete model
async function deleteModel() {
  if (!confirm(t('settings.tts.confirmDelete'))) return
  try {
    await ttsApi.sherpa.deleteModel()
    await loadStatus()
  } catch (e) {
    error.value = (e as Error).message
  }
}

// Switch to a different model
async function switchModel(modelType: string) {
  try {
    await ttsApi.sherpa.switchModel(modelType)
    await loadStatus()
  } catch (e) {
    error.value = (e as Error).message
  }
}

// Format bytes
function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

onMounted(() => {
  loadStatus()
})

onUnmounted(() => {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
})
</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex items-center justify-between">
      <div>
        <h3 class="text-lg font-medium text-gray-900 dark:text-white">
          {{ t('settings.tts.title') }}
        </h3>
        <p class="text-sm text-gray-500 dark:text-gray-400">
          {{ t('settings.tts.description') }}
        </p>
      </div>
      <button
        class="text-sm text-gray-900 dark:text-gray-300 hover:text-gray-900 dark:text-gray-300/80"
        :disabled="loading"
        @click="loadStatus"
      >
        {{ t('common.refresh') }}
      </button>
    </div>

    <!-- Error Alert -->
    <div v-if="error" class="p-4 bg-red-50 dark:bg-red-900/20 rounded-lg">
      <p class="text-sm text-red-600 dark:text-red-400">{{ error }}</p>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="flex items-center justify-center py-8">
      <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900 dark:border-gray-700"></div>
    </div>

    <!-- Providers List -->
    <div v-else class="space-y-4">
      <div class="bg-white dark:bg-gray-700 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
        <div class="px-4 py-3 border-b border-gray-200 dark:border-gray-700">
          <h4 class="font-medium text-gray-900 dark:text-white">
            {{ t('settings.tts.providers') }}
          </h4>
        </div>
        <div class="divide-y divide-gray-200 dark:divide-gray-700">
          <div
            v-for="provider in providers"
            :key="provider.type"
            class="px-4 py-3 flex items-center justify-between"
          >
            <div class="flex items-center gap-3">
              <span
                class="w-2 h-2 rounded-full"
                :class="provider.enabled ? 'bg-green-500' : 'bg-gray-300'"
              ></span>
              <div>
                <p class="font-medium text-gray-900 dark:text-white">
                  {{ provider.type }}
                  <span v-if="provider.native" class="ml-1 text-xs text-green-600">(Native)</span>
                </p>
                <p v-if="provider.type === 'sherpa' || provider.type === 'kokoro'" class="text-xs text-gray-500">
                  {{ provider.model_ready ? t('settings.tts.modelReady') : t('settings.tts.modelNotReady') }}
                </p>
              </div>
            </div>
            <span
              v-if="provider.type === defaultProvider"
              class="px-2 py-1 text-xs bg-gray-700 dark:bg-gray-700/10 text-gray-900 dark:text-gray-300 rounded"
            >
              {{ t('common.default') }}
            </span>
          </div>
        </div>
      </div>

      <!-- Sherpa Local TTS Section -->
      <div class="bg-white dark:bg-gray-700 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
        <div class="px-4 py-3 border-b border-gray-200 dark:border-gray-700">
          <div class="flex items-center justify-between">
            <div>
              <h4 class="font-medium text-gray-900 dark:text-white">
                Sherpa TTS ({{ t('settings.tts.local') }})
              </h4>
              <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                {{ t('settings.tts.nativeDescription') }}
              </p>
            </div>
            <span
              class="px-2 py-1 text-xs rounded"
              :class="modelReady ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400' : 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400'"
            >
              {{ modelReady ? t('settings.tts.ready') : t('settings.tts.notReady') }}
            </span>
          </div>
        </div>

        <div class="p-4 space-y-4">
          <!-- Model Status -->
          <div>
            <div class="flex items-center justify-between mb-2">
              <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
                {{ t('settings.tts.model') }}: {{ sherpaStatus?.model_type || 'kokoro-en' }}
              </span>
              <span
                class="text-xs"
                :class="modelReady ? 'text-green-600' : 'text-gray-500'"
              >
                {{ modelReady ? t('settings.tts.downloaded') : t('settings.tts.notDownloaded') }}
              </span>
            </div>

            <!-- Model Directory -->
            <div v-if="sherpaStatus?.model_dir" class="text-xs text-gray-500 mb-3">
              {{ t('settings.tts.modelDir') }}: {{ sherpaStatus.model_dir }}
            </div>

            <!-- Download Progress -->
            <div v-if="(isDownloading || hasPendingDownload) && downloadProgress" class="mb-3">
              <div class="flex items-center justify-between text-xs mb-1">
                <span class="text-gray-600 dark:text-gray-400">
                  {{ downloadProgress.file }}
                </span>
                <span class="text-gray-600 dark:text-gray-400">
                  {{ downloadProgress.percentage.toFixed(1) }}% - {{ downloadProgress.speed_human }}
                </span>
              </div>
              <div class="w-full bg-gray-700 dark:bg-gray-700 rounded-full h-2">
                <div
                  class="bg-gray-700 dark:bg-gray-700 h-2 rounded-full transition-all"
                  :style="{ width: `${downloadProgress.percentage}%` }"
                ></div>
              </div>
              <div class="flex items-center justify-between mt-1">
                <p class="text-xs text-gray-500">
                  {{ formatBytes(downloadProgress.downloaded) }} / {{ formatBytes(downloadProgress.total) }}
                </p>
                <p class="text-xs text-gray-500">
                  {{ t('settings.tts.eta') }}: {{ downloadProgress.eta || '--' }}
                </p>
              </div>
            </div>

            <!-- Resume Button (for pending downloads) -->
            <div v-if="hasPendingDownload && !isDownloading" class="mb-3">
              <div class="p-3 bg-yellow-50 dark:bg-yellow-900/20 rounded-lg">
                <p class="text-sm text-yellow-700 dark:text-yellow-400 mb-2">
                  {{ t('settings.tts.pendingDownload') }}: {{ pendingModel }}
                </p>
                <button
                  class="w-full px-3 py-2 text-sm bg-yellow-500 text-white rounded-lg hover:bg-yellow-600 disabled:opacity-50"
                  :disabled="downloading"
                  @click="resumeDownload"
                >
                  {{ t('settings.tts.resumeDownload') }}
                </button>
              </div>
            </div>

            <!-- Model Selection and Download -->
            <div v-if="!modelReady && !isDownloading && !hasPendingDownload">
              <!-- Model Selector -->
              <div v-if="availableModels.length > 0" class="mb-3">
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                  {{ t('settings.tts.selectModel') }}
                </label>
                <select
                  v-model="selectedModelType"
                  class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
                >
                  <option
                    v-for="model in availableModels"
                    :key="model.id"
                    :value="model.id"
                  >
                    {{ model.name }} ({{ model.size }}) - {{ model.languages.join(', ') }}
                  </option>
                </select>
                <p v-if="availableModels.find(m => m.id === selectedModelType)?.description" class="text-xs text-gray-500 mt-1">
                  {{ availableModels.find(m => m.id === selectedModelType)?.description }}
                </p>
              </div>

              <button
                class="w-full px-3 py-2 text-sm bg-gray-700 dark:bg-gray-700 text-white rounded-lg hover:bg-gray-700 dark:bg-gray-700/90 disabled:opacity-50"
                :disabled="downloading"
                @click="downloadModel()"
              >
                {{ t('settings.tts.downloadModel') }}
              </button>
              <p class="text-xs text-gray-500 mt-2">
                {{ t('settings.tts.downloadNote') }}
              </p>
            </div>

            <!-- Delete Button -->
            <button
              v-if="modelReady"
              class="text-sm text-red-600 hover:text-red-700"
              @click="deleteModel"
            >
              {{ t('settings.tts.deleteModel') }}
            </button>
          </div>
        </div>
      </div>

      <!-- Available Models List -->
      <div v-if="availableModels.length > 0" class="bg-white dark:bg-gray-700 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
        <div class="px-4 py-3 border-b border-gray-200 dark:border-gray-700">
          <h4 class="font-medium text-gray-900 dark:text-white">
            {{ t('settings.tts.availableModels') }}
          </h4>
        </div>
        <div class="divide-y divide-gray-200 dark:divide-gray-700">
          <div
            v-for="model in availableModels"
            :key="model.id"
            class="px-4 py-3 flex items-center justify-between"
          >
            <div>
              <p class="font-medium text-gray-900 dark:text-white">
                {{ model.name }}
                <span
                  v-if="model.downloaded"
                  class="ml-2 px-2 py-0.5 text-xs bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400 rounded"
                >
                  {{ t('settings.tts.downloaded') }}
                </span>
                <span
                  v-if="sherpaStatus?.model_type === model.id"
                  class="ml-2 px-2 py-0.5 text-xs bg-gray-700 dark:bg-gray-700 text-gray-900 dark:text-white dark:bg-gray-700 dark:bg-gray-700/30 dark:text-gray-900 dark:text-white rounded"
                >
                  {{ t('settings.tts.inUse') }}
                </span>
              </p>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                {{ model.description }}
              </p>
              <p class="text-xs text-gray-400 dark:text-gray-500 mt-1">
                {{ model.languages.join(', ') }} - {{ model.size }}
              </p>
            </div>
            <div class="flex items-center gap-2">
              <button
                v-if="model.downloaded && sherpaStatus?.model_type !== model.id"
                class="px-3 py-1 text-xs bg-gray-700 dark:bg-gray-700 text-white rounded hover:bg-gray-700 dark:bg-gray-700"
                :disabled="isDownloading"
                @click="switchModel(model.id)"
              >
                {{ t('settings.tts.useModel') }}
              </button>
              <button
                v-if="!model.downloaded && !isDownloading"
                class="px-3 py-1 text-xs bg-gray-700 dark:bg-gray-700 text-white rounded hover:bg-gray-700 dark:bg-gray-700/90"
                @click="downloadModel(model.id)"
              >
                {{ t('settings.tts.download') }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
