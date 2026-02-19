<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { speechApi, type SpeechStatus, type ASRModel, type TTSModel } from '@/api/speech'

const { t, locale } = useI18n()

// Tab state
const activeTab = ref<'asr' | 'tts'>('asr')

const status = ref<SpeechStatus | null>(null)
const asrModels = ref<ASRModel[]>([])
const ttsModels = ref<TTSModel[]>([])
const loading = ref(false)
const switchingModelId = ref<string | null>(null)  // Track which model is switching
const asrDownloadingModelId = ref<string | null>(null)  // Track which model is downloading
const _ttsDownloadingModelId = ref<string | null>(null)
const _asrDownloadProgress = ref(0)
const ttsDownloadProgress = ref(0)
const ttsDownloading = ref(false)
const error = ref<string | null>(null)

// Kokoro state
const kokoroReady = ref(false)
const kokoroDownloading = ref(false)
const kokoroDownloadProgress = ref(0)
const kokoroDownloadSpeed = ref('')
const kokoroDownloadETA = ref('')
const kokoroDownloadFile = ref('')
const kokoroDownloadFileIndex = ref(0)
const kokoroDownloadTotalFiles = ref(0)
const kokoroDownloadedHuman = ref('')
const kokoroLangsExpanded = ref(false)

const kokoroLanguages = ['en-US', 'en-GB', 'ja-JP', 'zh-CN', 'es-ES', 'fr-FR', 'hi-IN', 'it-IT', 'pt-BR']
const kokoroCurrentLangSupported = computed(() => kokoroLanguages.includes(locale.value))
const kokoroOtherLangs = computed(() =>
  kokoroLanguages.filter(l => l !== locale.value)
)

// eSpeak-NG state (engine is statically linked, data dir detected at runtime)
const espeakDataReady = ref(false)
const espeakLangCount = ref(0)
const espeakDataSize = ref(0)

// Computed: get all downloading models from server status
const serverDownloadingASRModels = computed(() => {
  const downloads = status.value?.asr?.downloads || []
  return downloads.map((d: { model_type: string }) => d.model_type)
})

// Get progress for a specific model
function getModelProgress(modelId: string) {
  const downloads = status.value?.asr?.downloads || []
  const download = downloads.find((d: { model_type: string }) => d.model_type === modelId)
  if (!download) return null
  return {
    percentage: download.progress.percentage || 0,
    downloaded: download.progress.downloaded,
    total: download.progress.total,
    speed: download.progress.speed_human,
    eta: download.progress.eta
  }
}

// Check if a specific model is downloading
function isModelDownloading(modelId: string) {
  return serverDownloadingASRModels.value.includes(modelId) || asrDownloadingModelId.value === modelId
}

const _asrReady = computed(() => status.value?.asr?.ready ?? false)
const _ttsReady = computed(() => status.value?.tts?.ready ?? false)
const currentASRModel = computed(() => status.value?.asr?.model_type ?? '')
const _currentTTSModel = computed(() => status.value?.tts?.model_type ?? '')
const editBeforeSend = computed({
  get: () => status.value?.asr?.edit_before_send ?? false,
  set: async (value: boolean) => {
    // TODO: Add API call to update edit_before_send setting
    if (status.value?.asr) {
      status.value.asr.edit_before_send = value
    }
  }
})

// TTS Provider selection - synced from server status
const selectedProvider = ref(localStorage.getItem('tts-provider') || '')
const selectedTTSModel = ref(localStorage.getItem('tts-model') || 'piper-en')
const selectedASRModel = ref(localStorage.getItem('asr-model') || '')

// Voice customization
const speechRate = ref(parseFloat(localStorage.getItem('tts-speech-rate') || '1.0'))
const speechPitch = ref(parseFloat(localStorage.getItem('tts-speech-pitch') || '0'))
const speechVolume = ref(parseFloat(localStorage.getItem('tts-speech-volume') || '100'))

// Auto-play TTS for assistant responses
const autoPlayTTS = ref(localStorage.getItem('tts-auto-play') === 'true')

async function saveProvider() {
  localStorage.setItem('tts-provider', selectedProvider.value)
  try {
    await speechApi.switchTTSProvider(selectedProvider.value)
    await fetchStatus()
  } catch (err) {
    console.error('Failed to switch TTS provider:', err)
    error.value = t('speech.switchError')
  }
}

// Get localized ASR model name
function _getAsrModelName(model: { id: string; name: string }): string {
  // Backend returns i18n key like "speech.asrModelInfo.whisperTiny.name"
  const translated = t(model.name)
  return translated === model.name ? model.name : translated
}

// Get localized ASR model description
function _getAsrModelDescription(model: { id: string; description: string }): string {
  // Backend returns i18n key like "speech.asrModelInfo.whisperTiny.description"
  const translated = t(model.description)
  return translated === model.description ? model.description : translated
}

function _saveTTSModel() {
  localStorage.setItem('tts-model', selectedTTSModel.value)
  if (selectedTTSModel.value) {
    switchTTSModel(selectedTTSModel.value)
  }
}

function _saveASRModel() {
  localStorage.setItem('asr-model', selectedASRModel.value)
  if (selectedASRModel.value) {
    switchASRModel(selectedASRModel.value)
  }
}

async function saveSpeechRate() {
  localStorage.setItem('tts-speech-rate', speechRate.value.toString())
  await saveTTSConfig()
}

async function saveSpeechPitch() {
  localStorage.setItem('tts-speech-pitch', speechPitch.value.toString())
  await saveTTSConfig()
}

async function saveSpeechVolume() {
  localStorage.setItem('tts-speech-volume', speechVolume.value.toString())
  await saveTTSConfig()
}

async function saveTTSConfig() {
  try {
    await speechApi.setTTSConfig(speechRate.value, speechPitch.value, speechVolume.value)
  } catch (err) {
    console.error('Failed to save TTS config:', err)
  }
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
    // Sync provider from server when no local preference is set
    if (!selectedProvider.value && res.data?.tts?.provider && res.data.tts.provider !== 'none') {
      selectedProvider.value = res.data.tts.provider
      localStorage.setItem('tts-provider', selectedProvider.value)
    }
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
  asrDownloadingModelId.value = modelType
  error.value = null
  try {
    await speechApi.downloadASRModel(modelType)
    // Poll for progress by fetching full status (includes model_type and progress)
    const pollInterval = setInterval(async () => {
      await fetchStatus()
      if (!status.value?.asr?.downloading) {
        clearInterval(pollInterval)
        asrDownloadingModelId.value = null
        await fetchASRModels()
      }
    }, 1000)
  } catch (e) {
    console.error('Failed to download ASR model:', e)
    error.value = t('speech.downloadError')
    asrDownloadingModelId.value = null
  }
}

async function _downloadTTSModel(modelType: string) {
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
    switchingModelId.value = modelType
    await speechApi.switchASRModel(modelType)
    await fetchStatus()
  } catch (e) {
    console.error('Failed to switch ASR model:', e)
    error.value = t('speech.switchError')
  } finally {
    switchingModelId.value = null
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

async function _deleteASRModel(modelType?: string) {
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

async function cancelASRDownload() {
  try {
    await speechApi.cancelASRDownload()
    asrDownloadingModelId.value = null
    await fetchStatus()
    await fetchASRModels()
  } catch (e) {
    console.error('Failed to cancel download:', e)
    error.value = t('speech.cancelError')
  }
}

async function fetchKokoroStatus() {
  try {
    const res = await speechApi.getKokoroStatus()
    kokoroReady.value = res.data.ready
    kokoroDownloading.value = res.data.downloading
    if (res.data.progress !== undefined) {
      kokoroDownloadProgress.value = res.data.progress
    }
    kokoroDownloadSpeed.value = res.data.speed || ''
    kokoroDownloadETA.value = res.data.eta || ''
    kokoroDownloadFile.value = res.data.file || ''
    kokoroDownloadFileIndex.value = res.data.file_index || 0
    kokoroDownloadTotalFiles.value = res.data.total_files || 0
    kokoroDownloadedHuman.value = res.data.downloaded_human || ''
  } catch (e) {
    console.error('Failed to fetch Kokoro status:', e)
  }
}

async function downloadKokoro() {
  kokoroDownloading.value = true
  kokoroDownloadProgress.value = 0
  error.value = null
  try {
    await speechApi.downloadKokoro()
    let pollCount = 0
    const pollInterval = setInterval(async () => {
      await fetchKokoroStatus()
      pollCount++
      // Grace period: don't stop polling in first 3s (goroutine may not have started yet)
      if (!kokoroDownloading.value && pollCount > 3) {
        clearInterval(pollInterval)
      }
    }, 1000)
  } catch (e) {
    console.error('Failed to download Kokoro model:', e)
    error.value = t('speech.downloadError')
    kokoroDownloading.value = false
  }
}

async function cancelKokoroDownload() {
  try {
    await speechApi.cancelKokoroDownload()
    kokoroDownloading.value = false
    kokoroDownloadProgress.value = 0
  } catch (e) {
    console.error('Failed to cancel Kokoro download:', e)
    error.value = t('speech.cancelError')
  }
}

async function fetchEspeakStatus() {
  try {
    const res = await speechApi.getEspeakLibraryStatus()
    espeakDataReady.value = res.data.installed
    espeakLangCount.value = res.data.language_count || 0
    espeakDataSize.value = res.data.data_size || 0
  } catch (e) {
    console.error('Failed to fetch eSpeak status:', e)
  }
}

async function _deleteTTSModel(modelType?: string) {
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
  fetchKokoroStatus()
  fetchEspeakStatus()
})
</script>

<template>
  <div class="space-y-6">
    <!-- Tab Navigation -->
    <div class="bg-white dark:bg-gray-700/30 rounded-lg shadow-sm">
      <div class="flex border-b border-gray-200 dark:border-gray-700">
        <button
          :class="[
            'flex-1 px-4 py-3 text-sm font-medium transition-colors',
            activeTab === 'asr'
              ? 'text-gray-900 dark:text-white border-b-2 border-gray-900 dark:border-white'
              : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-200'
          ]"
          @click="activeTab = 'asr'"
        >
          {{ t('speech.asrTab') }}
        </button>
        <button
          :class="[
            'flex-1 px-4 py-3 text-sm font-medium transition-colors',
            activeTab === 'tts'
              ? 'text-gray-900 dark:text-white border-b-2 border-gray-900 dark:border-white'
              : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-200'
          ]"
          @click="activeTab = 'tts'"
        >
          {{ t('speech.ttsTab') }}
        </button>
      </div>
    </div>

    <!-- Error Message -->
    <div v-if="error" class="p-3 bg-red-100 dark:bg-red-900/30 text-red-600 dark:text-red-400 rounded-lg text-sm">
      {{ error }}
    </div>

    <!-- ASR Tab Content -->
    <div v-show="activeTab === 'asr'" class="space-y-6">
      <!-- macOS Native STT Status -->
      <div v-if="status?.asr?.provider === 'macos-native'" class="bg-white dark:bg-gray-700/30 rounded-lg p-4 shadow-sm">
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-3">
            <div class="w-8 h-8 rounded-lg bg-blue-100 dark:bg-blue-900/30 flex items-center justify-center">
              <svg class="w-4 h-4 text-blue-600 dark:text-blue-400" fill="currentColor" viewBox="0 0 24 24"><path d="M12 1a3 3 0 0 0-3 3v8a3 3 0 0 0 6 0V4a3 3 0 0 0-3-3zM7 12a5 5 0 0 0 10 0h2a7 7 0 0 1-6 6.93V22h-2v-3.07A7 7 0 0 1 5 12h2z"/></svg>
            </div>
            <div>
              <span class="text-sm font-medium text-gray-900 dark:text-white">{{ t('speech.macosNativeName') }}</span>
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('speech.macosNativeSTTDesc') }}</p>
            </div>
          </div>
          <span v-if="status?.asr?.ready" class="text-xs px-2 py-1 rounded-full bg-green-100 dark:bg-green-900/30 text-green-600 dark:text-green-400">{{ t('speech.ready') }}</span>
        </div>
      </div>

      <!-- ASR Models -->
      <div class="bg-white dark:bg-gray-700/30 rounded-lg p-4 shadow-sm">
        <h4 class="text-sm font-medium text-gray-900 dark:text-white mb-3">
          {{ t('speech.asrModels') }}
        </h4>
        <div v-if="asrModels.length === 0" class="text-sm text-gray-500 dark:text-gray-400">
          {{ t('speech.noModelsAvailable') }}
        </div>
        <div v-else class="space-y-2">
          <div
v-for="model in asrModels" :key="model.id"
            class="flex items-center justify-between p-3 border rounded-lg"
            :class="model.downloaded ? 'border-gray-300 dark:border-gray-600 bg-gray-50 dark:bg-gray-700/20' : 'border-gray-200 dark:border-gray-700'">
            <div class="flex-1">
              <span class="text-sm font-medium text-gray-900 dark:text-white">{{ t(model.name) }}</span>
              <span class="text-xs text-gray-400 ml-2">{{ model.size }}</span>
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t(model.description) }}</p>
              <!-- Download progress for this specific model -->
              <div v-if="isModelDownloading(model.id)" class="mt-2">
                <div class="flex items-center gap-2">
                  <div class="flex-1 bg-gray-300 dark:bg-gray-600 rounded-full h-1.5">
                    <div class="bg-gray-400 dark:bg-gray-500 h-1.5 rounded-full transition-all duration-300" :style="{ width: `${Math.floor(getModelProgress(model.id)?.percentage || 0)}%` }"></div>
                  </div>
                  <span class="text-xs text-gray-500">{{ Math.floor(getModelProgress(model.id)?.percentage || 0) }}%</span>
                  <button class="text-red-500 hover:text-red-600 text-xs" @click="cancelASRDownload">
                    {{ t('common.cancel') }}
                  </button>
                </div>
                <!-- Detailed progress info -->
                <div v-if="getModelProgress(model.id)" class="flex items-center gap-3 mt-1 text-xs text-gray-400">
                  <span v-if="getModelProgress(model.id)?.speed">{{ getModelProgress(model.id)?.speed }}</span>
                  <span v-if="getModelProgress(model.id)?.eta">{{ $t('speech.eta') }}: {{ getModelProgress(model.id)?.eta }}</span>
                  <span v-if="getModelProgress(model.id)?.total">{{ Math.round((getModelProgress(model.id)?.downloaded || 0) / 1024 / 1024) }}MB / {{ Math.round((getModelProgress(model.id)?.total || 0) / 1024 / 1024) }}MB</span>
                </div>
              </div>
            </div>
            <!-- Download button for not downloaded models -->
            <button
v-if="!model.downloaded && !isModelDownloading(model.id)"
              class="px-3 py-1.5 bg-gray-700 dark:bg-gray-500 text-white rounded-lg hover:bg-black dark:hover:bg-gray-600 text-xs font-medium"
              @click="downloadASRModel(model.id)">
              {{ t('common.download') }}
            </button>
            <!-- Downloading indicator -->
            <span
v-else-if="isModelDownloading(model.id)"
              class="text-gray-900 dark:text-white text-xs font-medium">
              {{ t('speech.downloading') }}
            </span>
            <!-- Switch button for downloaded models -->
            <div v-else class="flex items-center gap-2">
              <button
                v-if="currentASRModel !== model.id && switchingModelId !== model.id"
                :disabled="!!switchingModelId"
                class="px-3 py-1.5 bg-green-500 text-white rounded-lg hover:bg-green-600 text-xs font-medium disabled:opacity-50"
                @click="switchASRModel(model.id)">
                {{ t('common.use') }}
              </button>
              <span v-else-if="switchingModelId === model.id" class="text-gray-900 dark:text-white text-xs font-medium flex items-center gap-1">
                <svg class="animate-spin h-3 w-3" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                  <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                  <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
                {{ t('speech.switching') }}
              </span>
              <span v-else class="text-green-600 dark:text-green-400 text-xs font-medium">
                {{ t('common.inUse') }}
              </span>
            </div>
          </div>
        </div>
      </div>

      <!-- Edit Before Send Toggle -->
      <div class="bg-white dark:bg-gray-700/30 rounded-lg p-4 shadow-sm">
        <div class="flex items-center justify-between">
          <div>
            <label class="text-sm font-medium text-gray-900 dark:text-white">{{ t('speech.editBeforeSend') }}</label>
            <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">{{ t('speech.editBeforeSendDesc') }}</p>
          </div>
          <label class="relative inline-flex items-center cursor-pointer">
            <input
              v-model="editBeforeSend"
              type="checkbox"
              class="sr-only peer"
            />
            <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-gray-900 dark:focus:ring-gray-400 dark:peer-focus:ring-gray-900 dark:focus:ring-gray-400 rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full rtl:peer-checked:after:-translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:start-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-green-600 dark:peer-checked:bg-green-500"></div>
          </label>
        </div>
      </div>
    </div>

    <!-- TTS Tab Content -->
    <div v-show="activeTab === 'tts'" class="space-y-6">
      <!-- TTS Provider Selection -->
      <div class="bg-white dark:bg-gray-700/30 rounded-lg p-4 shadow-sm">
        <h4 class="text-sm font-medium text-gray-900 dark:text-white mb-3">
          {{ t('speech.ttsProvider') }}
        </h4>
        <div class="space-y-2">
          <label
class="flex items-center p-3 border rounded-lg cursor-pointer transition-colors"
            :class="selectedProvider === 'macos-native' ? 'border-gray-900 dark:border-white bg-gray-100 dark:bg-gray-700/30' : 'border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/50'">
            <input v-model="selectedProvider" type="radio" value="macos-native" class="sr-only" @change="saveProvider" />
            <div class="flex-1">
              <div class="flex items-center gap-2">
                <svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor"><path d="M18.71 19.5C17.88 20.74 17 21.95 15.66 21.97C14.32 22 13.89 21.18 12.37 21.18C10.84 21.18 10.37 21.95 9.1 22C7.79 22.05 6.8 20.68 5.96 19.47C4.25 16.56 2.93 11.3 4.7 7.72C5.57 5.94 7.36 4.86 9.28 4.84C10.56 4.81 11.78 5.72 12.57 5.72C13.36 5.72 14.85 4.62 16.4 4.8C17.07 4.83 18.89 5.08 20.07 6.77C19.96 6.84 17.62 8.23 17.65 11.1C17.68 14.54 20.59 15.62 20.63 15.63C20.59 15.72 20.12 17.37 18.71 19.5ZM13 3.5C13.73 2.67 14.94 2.04 15.94 2C16.07 3.17 15.6 4.35 14.9 5.19C14.21 6.04 13.07 6.7 11.95 6.61C11.8 5.46 12.36 4.26 13 3.5Z"/></svg>
                <span class="text-sm font-medium text-gray-900 dark:text-white">{{ t('speech.macosNativeName') }}</span>
                <span class="text-xs px-1.5 py-0.5 rounded bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-400">{{ t('speech.macosNativeQuality') }}</span>
              </div>
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('speech.macosNativeDesc') }}</p>
            </div>
            <span v-if="selectedProvider === 'macos-native'" class="text-gray-900 dark:text-white">
              <svg class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"/></svg>
            </span>
          </label>
          <label
class="flex items-center p-3 border rounded-lg cursor-pointer transition-colors"
            :class="selectedProvider === 'edge-tts' ? 'border-gray-900 dark:border-white bg-gray-100 dark:bg-gray-700/30' : 'border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/50'">
            <input v-model="selectedProvider" type="radio" value="edge-tts" class="sr-only" @change="saveProvider" />
            <div class="flex-1">
              <div class="flex items-center gap-2">
                <svg class="w-4 h-4 text-[#0078D4]" viewBox="0 0 24 24" fill="currentColor"><path d="M21.17 3.25Q21.5 3.25 21.76 3.5 22 3.74 22 4.08V19.92Q22 20.26 21.76 20.5 21.5 20.75 21.17 20.75H2.83Q2.5 20.75 2.24 20.5 2 20.26 2 19.92V4.08Q2 3.74 2.24 3.5 2.5 3.25 2.83 3.25ZM12.67 12.13Q12.67 10.41 11.78 9.5 10.89 8.58 9.33 8.58 7.78 8.58 6.89 9.5 6 10.41 6 12.13 6 13.84 6.89 14.76 7.78 15.67 9.33 15.67 10.89 15.67 11.78 14.76 12.67 13.84 12.67 12.13ZM18 8.75H14.5V9.92H18ZM18 11.42H14.5V12.58H18ZM18 14.08H14.5V15.25H18Z"/></svg>
                <span class="text-sm font-medium text-gray-900 dark:text-white">{{ t('speech.edgeTTSName') }}</span>
                <span class="text-xs px-1.5 py-0.5 rounded bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400">{{ t('speech.edgeTTSQuality') }}</span>
              </div>
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('speech.edgeTTSDesc') }}</p>
            </div>
            <span v-if="selectedProvider === 'edge-tts'" class="text-gray-900 dark:text-white">
              <svg class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"/></svg>
            </span>
          </label>
          <!-- eSpeak-NG + HiFi-GAN -->
          <div
            class="p-3 border rounded-lg transition-colors"
            :class="selectedProvider === 'espeak-ng' ? 'border-gray-900 dark:border-white bg-gray-100 dark:bg-gray-700/30' : 'border-gray-200 dark:border-gray-700'">
            <label class="flex items-center cursor-pointer">
              <input v-model="selectedProvider" type="radio" value="espeak-ng" class="sr-only" @change="saveProvider" :disabled="!espeakDataReady" />
              <div class="flex-1">
                <div class="flex items-center gap-2">
                  <span class="text-sm font-medium text-gray-900 dark:text-white">eSpeak-NG</span>
                  <span class="text-xs px-1.5 py-0.5 rounded bg-orange-100 dark:bg-orange-900/30 text-orange-700 dark:text-orange-400">{{ t('speech.espeakNGQuality') }}</span>
                </div>
                <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('speech.espeakNGDesc') }}</p>
              </div>
              <span v-if="selectedProvider === 'espeak-ng'" class="text-gray-900 dark:text-white">
                <svg class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"/></svg>
              </span>
            </label>
            <!-- Status -->
            <div class="mt-2">
              <div v-if="espeakDataReady" class="flex items-center gap-1 text-xs text-green-600 dark:text-green-400">
                <svg class="w-3.5 h-3.5" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"/></svg>
                {{ t('speech.espeakDepLib') }}
                <span class="text-gray-400">({{ espeakLangCount }} {{ t('speech.espeakLangs') }}, {{ (espeakDataSize / 1024 / 1024).toFixed(1) }}MB)</span>
              </div>
              <div v-else class="text-xs text-red-500 dark:text-red-400">
                <span>✗ {{ t('speech.espeakDepLib') }}</span>
                <p class="text-red-400 mt-1">{{ t('speech.espeakNeedRebuild') }}</p>
              </div>
            </div>
          </div>
          <!-- Kokoro TTS -->
          <div
            class="p-3 border rounded-lg transition-colors"
            :class="selectedProvider === 'kokoro' ? 'border-gray-900 dark:border-white bg-gray-100 dark:bg-gray-700/30' : 'border-gray-200 dark:border-gray-700'">
            <label class="flex items-center cursor-pointer">
              <input v-model="selectedProvider" type="radio" value="kokoro" class="sr-only" @change="saveProvider" :disabled="!kokoroReady" />
              <div class="flex-1">
                <div class="flex items-center gap-2">
                  <span class="text-sm font-medium text-gray-900 dark:text-white">Kokoro</span>
                  <a :href="t('speech.kokoroGithub')" target="_blank" rel="noopener" class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300" @click.stop>
                    <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="currentColor"><path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z"/></svg>
                  </a>
                  <span class="text-xs px-1.5 py-0.5 rounded bg-purple-100 dark:bg-purple-900/30 text-purple-700 dark:text-purple-400">{{ t('speech.kokoroQuality') }}</span>
                </div>
                <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('speech.kokoroDesc') }}</p>
              </div>
              <span v-if="selectedProvider === 'kokoro'" class="text-gray-900 dark:text-white">
                <svg class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"/></svg>
              </span>
            </label>
            <!-- Download / Status -->
            <div class="mt-2">
              <div v-if="kokoroReady" class="flex items-center gap-1 text-xs text-green-600 dark:text-green-400">
                <svg class="w-3.5 h-3.5" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"/></svg>
                {{ t('speech.kokoroReady') }}
              </div>
              <div v-else-if="kokoroDownloading" class="space-y-1">
                <div class="flex items-center gap-2">
                  <div class="flex-1 bg-gray-300 dark:bg-gray-600 rounded-full h-1.5">
                    <div class="bg-purple-500 h-1.5 rounded-full transition-all duration-300" :style="{ width: kokoroDownloadProgress > 0 ? `${kokoroDownloadProgress}%` : '100%', animation: kokoroDownloadProgress <= 0 ? 'pulse 2s ease-in-out infinite' : 'none', opacity: kokoroDownloadProgress <= 0 ? 0.5 : 1 }"></div>
                  </div>
                  <span class="text-xs text-gray-500 whitespace-nowrap">{{ kokoroDownloadProgress > 0 ? Math.floor(kokoroDownloadProgress) + '%' : kokoroDownloadedHuman || '...' }}</span>
                  <button class="text-red-500 hover:text-red-600 text-xs whitespace-nowrap" @click="cancelKokoroDownload">
                    {{ t('speech.kokoroCancelDownload') }}
                  </button>
                </div>
                <p class="text-xs text-gray-400">
                  <span v-if="kokoroDownloadFile">{{ kokoroDownloadFile }}</span>
                  <span v-if="kokoroDownloadTotalFiles > 1"> ({{ kokoroDownloadFileIndex + 1 }}/{{ kokoroDownloadTotalFiles }})</span>
                  <span v-if="kokoroDownloadSpeed"> · {{ kokoroDownloadSpeed }}</span>
                  <span v-if="kokoroDownloadETA"> · {{ kokoroDownloadETA }}</span>
                  <span v-if="!kokoroDownloadFile">{{ t('speech.kokoroDownloading') }}</span>
                </p>
              </div>
              <button
                v-else
                class="px-3 py-1.5 bg-purple-600 text-white rounded-lg hover:bg-purple-700 text-xs font-medium"
                @click="downloadKokoro">
                {{ t('speech.kokoroDownload') }}
              </button>
            </div>
            <!-- Supported Languages -->
            <div class="mt-2">
              <p class="text-xs font-medium text-gray-600 dark:text-gray-300 mb-1">{{ t('speech.kokoroLanguages') }}</p>
              <!-- Current language supported -->
              <div v-if="kokoroCurrentLangSupported" class="flex items-center gap-1 text-xs text-green-600 dark:text-green-400 mb-1">
                <svg class="w-3.5 h-3.5" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"/></svg>
                {{ t('speech.kokoroSupportsYourLang', { lang: t(`speech.langName.${locale}`) }) }}
              </div>
              <!-- Collapsed: show "other N languages" button -->
              <button
                v-if="!kokoroLangsExpanded"
                class="text-xs text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 underline"
                @click="kokoroLangsExpanded = true">
                {{ t('speech.kokoroOtherLangs', { count: kokoroOtherLangs.length }) }}
              </button>
              <!-- Expanded: show all other languages -->
              <div v-else class="flex flex-wrap gap-1">
                <span v-for="lang in kokoroOtherLangs" :key="lang"
                  class="text-xs px-1.5 py-0.5 rounded bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300">
                  {{ t(`speech.langName.${lang}`) }}
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Voice Customization -->
      <div class="bg-white dark:bg-gray-700/30 rounded-lg p-4 shadow-sm">
        <h4 class="text-sm font-medium text-gray-900 dark:text-white mb-4">
          {{ t('speech.voiceSettings') }}
        </h4>
        <div class="space-y-4">
          <!-- Speech Rate -->
          <div>
            <div class="flex items-center justify-between mb-2">
              <label class="text-sm text-gray-700 dark:text-gray-300">{{ t('speech.rate') }}</label>
              <span class="text-sm font-medium text-gray-900 dark:text-white">{{ speechRate.toFixed(1) }}x</span>
            </div>
            <input
              v-model.number="speechRate"
              type="range"
              min="0.5"
              max="2.0"
              step="0.1"
              class="w-full h-2 bg-gray-100 dark:bg-gray-700/30 rounded-lg appearance-none cursor-pointer accent-gray-900 dark:accent-gray-400"
              @change="saveSpeechRate"
            />
          </div>

          <!-- Speech Pitch -->
          <div>
            <div class="flex items-center justify-between mb-2">
              <label class="text-sm text-gray-700 dark:text-gray-300">{{ t('speech.pitch') }}</label>
              <span class="text-sm font-medium text-gray-900 dark:text-white">{{ speechPitch }}</span>
            </div>
            <input
              v-model.number="speechPitch"
              type="range"
              min="-50"
              max="50"
              step="1"
              class="w-full h-2 bg-gray-100 dark:bg-gray-700/30 rounded-lg appearance-none cursor-pointer accent-gray-900 dark:accent-gray-400"
              @change="saveSpeechPitch"
            />
          </div>

          <!-- Speech Volume -->
          <div>
            <div class="flex items-center justify-between mb-2">
              <label class="text-sm text-gray-700 dark:text-gray-300">{{ t('speech.volume') }}</label>
              <span class="text-sm font-medium text-gray-900 dark:text-white">{{ speechVolume }}%</span>
            </div>
            <input
              v-model.number="speechVolume"
              type="range"
              min="0"
              max="100"
              step="1"
              class="w-full h-2 bg-gray-100 dark:bg-gray-700/30 rounded-lg appearance-none cursor-pointer accent-gray-900 dark:accent-gray-400"
              @change="saveSpeechVolume"
            />
          </div>
        </div>
      </div>

      <!-- Auto-play TTS -->
      <div class="bg-white dark:bg-gray-700/30 rounded-lg p-4 shadow-sm">
        <div class="flex items-center justify-between">
          <div>
            <label class="text-sm font-medium text-gray-900 dark:text-white">{{ t('speech.autoPlayTTS') }}</label>
            <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">{{ t('speech.autoPlayTTSDesc') }}</p>
          </div>
          <label class="relative inline-flex items-center cursor-pointer">
            <input
              v-model="autoPlayTTS"
              type="checkbox"
              class="sr-only peer"
              @change="saveAutoPlayTTS"
            />
            <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-gray-900 dark:focus:ring-gray-400 dark:peer-focus:ring-gray-900 dark:focus:ring-gray-400 rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full rtl:peer-checked:after:-translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:start-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-green-600 dark:peer-checked:bg-green-500"></div>
          </label>
        </div>
      </div>
    </div>

  </div>
</template>
