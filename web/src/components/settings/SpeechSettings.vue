<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { speechApi, type SpeechStatus, type ASRModel, type TTSModel } from '@/api/speech'
import TTSSettingsNew from './TTSSettingsNew.vue'

const { t } = useI18n()

const status = ref<SpeechStatus | null>(null)
const asrModels = ref<ASRModel[]>([])
const ttsModels = ref<TTSModel[]>([])
const loading = ref(false)
const asrDownloading = ref(false)
const ttsDownloading = ref(false)
const asrDownloadProgress = ref(0)
const ttsDownloadProgress = ref(0)
const error = ref<string | null>(null)

const asrReady = computed(() => status.value?.asr?.ready ?? false)
const ttsReady = computed(() => status.value?.tts?.ready ?? false)
const currentASRModel = computed(() => status.value?.asr?.model_type ?? '')
const currentTTSModel = computed(() => status.value?.tts?.model_type ?? '')
const editBeforeSend = computed(() => status.value?.asr?.edit_before_send ?? false)

// TTS speech speed
const speechSpeed = ref(parseFloat(localStorage.getItem('tts-speech-speed') || '1.0'))

// Auto-play TTS for assistant responses
const autoPlayTTS = ref(localStorage.getItem('tts-auto-play') === 'true')

function saveSpeechSpeed() {
  localStorage.setItem('tts-speech-speed', speechSpeed.value.toString())
}

function saveAutoPlayTTS() {
  localStorage.setItem('tts-auto-play', autoPlayTTS.value.toString())
}

async function fetchStatus() {
  loading.value = true
  error.value = null
  try {
    const res = await speechApi.getStatus()
    status.value = res.data
  } catch (e) {
    console.error('Failed to fetch speech status:', e)
    error.value = t('speech.fetchError')
  } finally {
    loading.value = false
  }
}

async function fetchASRModels() {
  try {
    const res = await speechApi.listASRModels()
    asrModels.value = res.data?.models || []
  } catch (e) {
    console.error('Failed to fetch ASR models:', e)
  }
}

async function fetchTTSModels() {
  try {
    const res = await speechApi.listTTSModels()
    ttsModels.value = res.data?.models || []
  } catch (e) {
    console.error('Failed to fetch TTS models:', e)
  }
}

async function downloadASRModel(modelType: string) {
  asrDownloading.value = true
  asrDownloadProgress.value = 0
  error.value = null
  try {
    await speechApi.downloadASRModel(modelType)
    // Poll for progress
    const pollInterval = setInterval(async () => {
      const res = await speechApi.getASRStatus()
      if (res.data?.progress) {
        asrDownloadProgress.value = res.data.progress.percentage
      }
      if (!res.data?.downloading) {
        clearInterval(pollInterval)
        asrDownloading.value = false
        await fetchStatus()
        await fetchASRModels()
      }
    }, 1000)
  } catch (e) {
    console.error('Failed to download ASR model:', e)
    error.value = t('speech.downloadError')
    asrDownloading.value = false
  }
}

async function downloadTTSModel(modelType: string) {
  ttsDownloading.value = true
  ttsDownloadProgress.value = 0
  error.value = null
  try {
    await speechApi.downloadTTSModel(modelType)
    // Poll for progress
    const pollInterval = setInterval(async () => {
      const res = await speechApi.getTTSStatus()
      if (res.data?.progress) {
        ttsDownloadProgress.value = res.data.progress.percentage
      }
      if (!res.data?.downloading) {
        clearInterval(pollInterval)
        ttsDownloading.value = false
        await fetchStatus()
        await fetchTTSModels()
      }
    }, 1000)
  } catch (e) {
    console.error('Failed to download TTS model:', e)
    error.value = t('speech.downloadError')
    ttsDownloading.value = false
  }
}

async function switchASRModel(modelType: string) {
  try {
    await speechApi.switchASRModel(modelType)
    await fetchStatus()
  } catch (e) {
    console.error('Failed to switch ASR model:', e)
    error.value = t('speech.switchError')
  }
}

async function switchTTSModel(modelType: string) {
  try {
    await speechApi.switchTTSModel(modelType)
    await fetchStatus()
  } catch (e) {
    console.error('Failed to switch TTS model:', e)
    error.value = t('speech.switchError')
  }
}

async function deleteASRModel(modelType?: string) {
  if (!confirm(t('speech.confirmDelete'))) return
  try {
    await speechApi.deleteASRModel(modelType)
    await fetchStatus()
    await fetchASRModels()
  } catch (e) {
    console.error('Failed to delete ASR model:', e)
    error.value = t('speech.deleteError')
  }
}

async function deleteTTSModel(modelType?: string) {
  if (!confirm(t('speech.confirmDelete'))) return
  try {
    await speechApi.deleteTTSModel(modelType)
    await fetchStatus()
    await fetchTTSModels()
  } catch (e) {
    console.error('Failed to delete TTS model:', e)
    error.value = t('speech.deleteError')
  }
}

onMounted(() => {
  fetchStatus()
  fetchASRModels()
  fetchTTSModels()
})
</script>

<template>
  <div class="space-y-6">
    <!-- Status Card -->
    <div class="bg-white dark:bg-gray-800 rounded-lg p-4 shadow-sm">
      <h3 class="text-lg font-medium text-gray-900 dark:text-white mb-4">
        {{ t('speech.status') }}
      </h3>

      <div v-if="loading" class="flex items-center justify-center py-8">
        <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-500"></div>
      </div>

      <div v-else-if="status" class="space-y-4">
        <!-- ASR Status -->
        <div class="flex items-center justify-between">
          <span class="text-gray-600 dark:text-gray-400">{{ t('speech.asrStatus') }}</span>
          <span
            :class="asrReady ? 'text-green-500' : 'text-yellow-500'"
            class="font-medium"
          >
            {{ asrReady ? t('speech.ready') : t('speech.notReady') }}
          </span>
        </div>

        <!-- Current ASR Model -->
        <div v-if="currentASRModel" class="flex items-center justify-between">
          <span class="text-gray-600 dark:text-gray-400">{{ t('speech.currentASRModel') }}</span>
          <span class="font-medium text-gray-900 dark:text-white">{{ currentASRModel }}</span>
        </div>

        <!-- Edit Before Send -->
        <div class="flex items-center justify-between">
          <span class="text-gray-600 dark:text-gray-400">{{ t('speech.editBeforeSend') }}</span>
          <span
            :class="editBeforeSend ? 'text-green-500' : 'text-gray-500'"
            class="font-medium"
          >
            {{ editBeforeSend ? t('common.enabled') : t('common.disabled') }}
          </span>
        </div>

        <!-- TTS Status -->
        <div class="flex items-center justify-between">
          <span class="text-gray-600 dark:text-gray-400">{{ t('speech.ttsStatus') }}</span>
          <span
            :class="ttsReady ? 'text-green-500' : 'text-yellow-500'"
            class="font-medium"
          >
            {{ ttsReady ? t('speech.ready') : t('speech.notReady') }}
          </span>
        </div>

        <!-- Current TTS Model -->
        <div v-if="currentTTSModel" class="flex items-center justify-between">
          <span class="text-gray-600 dark:text-gray-400">{{ t('speech.currentTTSModel') }}</span>
          <span class="font-medium text-gray-900 dark:text-white">{{ currentTTSModel }}</span>
        </div>
      </div>

      <div v-if="error" class="mt-4 p-3 bg-red-100 dark:bg-red-900/30 text-red-600 dark:text-red-400 rounded-lg text-sm">
        {{ error }}
      </div>
    </div>

    <!-- ASR Models -->
    <div class="bg-white dark:bg-gray-800 rounded-lg p-4 shadow-sm">
      <h3 class="text-lg font-medium text-gray-900 dark:text-white mb-4">
        {{ t('speech.asrModels') }}
      </h3>

      <div v-if="asrDownloading" class="mb-4">
        <div class="flex items-center justify-between mb-2">
          <span class="text-sm text-gray-600 dark:text-gray-400">{{ t('speech.downloading') }}</span>
          <span class="text-sm font-medium text-gray-900 dark:text-white">{{ Math.floor(asrDownloadProgress) }}%</span>
        </div>
        <div class="w-full bg-gray-200 dark:bg-gray-700 rounded-full h-2">
          <div
            class="bg-blue-500 h-2 rounded-full transition-all duration-300"
            :style="{ width: `${Math.floor(asrDownloadProgress)}%` }"
          ></div>
        </div>
        <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">{{ t('speech.resumeSupported') }}</p>
      </div>

      <div class="space-y-3">
        <div
          v-for="model in asrModels"
          :key="model.id"
          class="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-700/50 rounded-lg"
        >
          <div class="flex-1">
            <div class="font-medium text-gray-900 dark:text-white">{{ model.name }}</div>
            <div class="text-sm text-gray-500 dark:text-gray-400">
              {{ model.description }} · {{ model.size }}
            </div>
            <div v-if="model.languages?.length" class="text-xs text-gray-400 dark:text-gray-500 mt-1">
              {{ model.languages.join(', ') }}
            </div>
          </div>

          <div class="flex items-center gap-2 ml-4">
            <template v-if="model.downloaded">
              <button
                v-if="currentASRModel !== model.id"
                class="px-3 py-1.5 text-sm bg-blue-500 text-white rounded-lg hover:bg-blue-600 transition-colors"
                @click="switchASRModel(model.id)"
              >
                {{ t('speech.use') }}
              </button>
              <span v-else class="px-3 py-1.5 text-sm bg-green-500/20 text-green-500 rounded-lg">
                {{ t('speech.inUse') }}
              </span>
              <button
                class="p-1.5 text-red-500 hover:bg-red-500/10 rounded-lg transition-colors"
                :title="t('common.delete')"
                @click="deleteASRModel(model.id)"
              >
                <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                </svg>
              </button>
            </template>
            <button
              v-else
              class="px-3 py-1.5 text-sm bg-gray-200 dark:bg-gray-600 text-gray-700 dark:text-gray-200 rounded-lg hover:bg-gray-300 dark:hover:bg-gray-500 transition-colors"
              :disabled="asrDownloading"
              @click="downloadASRModel(model.id)"
            >
              {{ t('speech.download') }}
            </button>
          </div>
        </div>

        <div v-if="asrModels.length === 0" class="text-center py-8 text-gray-500 dark:text-gray-400">
          {{ t('speech.noModels') }}
        </div>
      </div>
    </div>

    <!-- TTS Models -->
    <div class="bg-white dark:bg-gray-800 rounded-lg p-4 shadow-sm">
      <h3 class="text-lg font-medium text-gray-900 dark:text-white mb-4">
        {{ t('speech.ttsModels') }}
      </h3>

      <div v-if="ttsDownloading" class="mb-4">
        <div class="flex items-center justify-between mb-2">
          <span class="text-sm text-gray-600 dark:text-gray-400">{{ t('speech.downloading') }}</span>
          <span class="text-sm font-medium text-gray-900 dark:text-white">{{ Math.floor(ttsDownloadProgress) }}%</span>
        </div>
        <div class="w-full bg-gray-200 dark:bg-gray-700 rounded-full h-2">
          <div
            class="bg-green-500 h-2 rounded-full transition-all duration-300"
            :style="{ width: `${Math.floor(ttsDownloadProgress)}%` }"
          ></div>
        </div>
        <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">{{ t('speech.resumeSupported') }}</p>
      </div>

      <div class="space-y-3">
        <div
          v-for="model in ttsModels"
          :key="model.id"
          class="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-700/50 rounded-lg"
        >
          <div class="flex-1">
            <div class="font-medium text-gray-900 dark:text-white">{{ model.name }}</div>
            <div class="text-sm text-gray-500 dark:text-gray-400">
              {{ model.description }} · {{ model.size }}
            </div>
            <div v-if="model.languages?.length" class="text-xs text-gray-400 dark:text-gray-500 mt-1">
              {{ model.languages.join(', ') }}
            </div>
          </div>

          <div class="flex items-center gap-2 ml-4">
            <template v-if="model.downloaded">
              <button
                v-if="currentTTSModel !== model.id"
                class="px-3 py-1.5 text-sm bg-green-500 text-white rounded-lg hover:bg-green-600 transition-colors"
                @click="switchTTSModel(model.id)"
              >
                {{ t('speech.use') }}
              </button>
              <span v-else class="px-3 py-1.5 text-sm bg-green-500/20 text-green-500 rounded-lg">
                {{ t('speech.inUse') }}
              </span>
              <button
                class="p-1.5 text-red-500 hover:bg-red-500/10 rounded-lg transition-colors"
                :title="t('common.delete')"
                @click="deleteTTSModel(model.id)"
              >
                <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                </svg>
              </button>
            </template>
            <button
              v-else
              class="px-3 py-1.5 text-sm bg-gray-200 dark:bg-gray-600 text-gray-700 dark:text-gray-200 rounded-lg hover:bg-gray-300 dark:hover:bg-gray-500 transition-colors"
              :disabled="ttsDownloading"
              @click="downloadTTSModel(model.id)"
            >
              {{ t('speech.download') }}
            </button>
          </div>
        </div>

        <div v-if="ttsModels.length === 0" class="text-center py-8 text-gray-500 dark:text-gray-400">
          {{ t('speech.noModels') }}
        </div>
      </div>
    </div>

    <!-- TTS Settings -->
    <div class="bg-white dark:bg-gray-800 rounded-lg p-4 shadow-sm">
      <h3 class="text-lg font-medium text-gray-900 dark:text-white mb-4">
        {{ t('speech.ttsSettings') }}
      </h3>

      <div class="space-y-4">
        <!-- Auto-play TTS -->
        <div class="flex items-center justify-between">
          <div>
            <span class="text-gray-900 dark:text-white">{{ t('speech.autoPlayTTS') }}</span>
            <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('speech.autoPlayTTSDesc') }}</p>
          </div>
          <label class="relative inline-flex items-center cursor-pointer">
            <input
              v-model="autoPlayTTS"
              type="checkbox"
              class="sr-only peer"
              @change="saveAutoPlayTTS"
            />
            <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-accent/20 dark:peer-focus:ring-accent/40 rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full rtl:peer-checked:after:-translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:start-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-accent"></div>
          </label>
        </div>

        <!-- Speech Speed -->
        <div class="space-y-2">
          <div class="flex items-center justify-between">
            <span class="text-gray-600 dark:text-gray-400">{{ t('speech.speechSpeed') }}</span>
            <span class="font-medium text-gray-900 dark:text-white">{{ speechSpeed.toFixed(1) }}x</span>
          </div>
          <input
            v-model="speechSpeed"
            type="range"
            min="0.5"
            max="2.0"
            step="0.1"
            class="w-full h-2 bg-gray-200 dark:bg-gray-700 rounded-lg appearance-none cursor-pointer accent-accent"
            @change="saveSpeechSpeed"
          />
          <div class="flex justify-between text-xs text-gray-500 dark:text-gray-400">
            <span>0.5x</span>
            <span>1.0x</span>
            <span>2.0x</span>
          </div>
        </div>
      </div>
    </div>

    <!-- New TTS Settings -->
    <TTSSettingsNew />

    <!-- Info -->
    <div class="bg-blue-50 dark:bg-blue-900/20 rounded-lg p-4">
      <p class="text-sm text-blue-700 dark:text-blue-300">
        {{ t('speech.info') }}
      </p>
    </div>
  </div>
</template>
