<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { speechApi, type SpeechStatus, type ASRModel, type TTSModel } from '@/api/speech'

const { t } = useI18n()

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

// TTS Provider selection
const selectedProvider = ref(localStorage.getItem('tts-provider') || 'edge')
const selectedTTSModel = ref(localStorage.getItem('tts-model') || 'piper-en')
const selectedASRModel = ref(localStorage.getItem('asr-model') || '')

// Voice customization
const speechRate = ref(parseFloat(localStorage.getItem('tts-speech-rate') || '1.0'))
const speechPitch = ref(parseFloat(localStorage.getItem('tts-speech-pitch') || '0'))
const speechVolume = ref(parseFloat(localStorage.getItem('tts-speech-volume') || '100'))

// Auto-play TTS for assistant responses
const autoPlayTTS = ref(localStorage.getItem('tts-auto-play') === 'true')

// eSpeak-NG language packs - 27 languages with sizes
const espeak_languages = ref<Array<{code: string; name: string; downloaded: boolean; size: string}>>([
  { code: 'en', name: 'English', downloaded: true, size: '250KB' },
  { code: 'zh', name: 'Chinese', downloaded: false, size: '400KB' },
  { code: 'es', name: 'Spanish', downloaded: false, size: '280KB' },
  { code: 'fr', name: 'French', downloaded: false, size: '300KB' },
  { code: 'de', name: 'German', downloaded: false, size: '320KB' },
  { code: 'ja', name: 'Japanese', downloaded: false, size: '420KB' },
  { code: 'ko', name: 'Korean', downloaded: false, size: '410KB' },
  { code: 'ru', name: 'Russian', downloaded: false, size: '350KB' },
  { code: 'pt', name: 'Portuguese', downloaded: false, size: '310KB' },
  { code: 'it', name: 'Italian', downloaded: false, size: '290KB' },
  { code: 'nl', name: 'Dutch', downloaded: false, size: '300KB' },
  { code: 'pl', name: 'Polish', downloaded: false, size: '320KB' },
  { code: 'tr', name: 'Turkish', downloaded: false, size: '340KB' },
  { code: 'ar', name: 'Arabic', downloaded: false, size: '380KB' },
  { code: 'hi', name: 'Hindi', downloaded: false, size: '360KB' },
  { code: 'th', name: 'Thai', downloaded: false, size: '350KB' },
  { code: 'vi', name: 'Vietnamese', downloaded: false, size: '330KB' },
  { code: 'id', name: 'Indonesian', downloaded: false, size: '280KB' },
  { code: 'fil', name: 'Filipino', downloaded: false, size: '290KB' },
  { code: 'uk', name: 'Ukrainian', downloaded: false, size: '340KB' },
  { code: 'cs', name: 'Czech', downloaded: false, size: '310KB' },
  { code: 'sv', name: 'Swedish', downloaded: false, size: '280KB' },
  { code: 'da', name: 'Danish', downloaded: false, size: '260KB' },
  { code: 'no', name: 'Norwegian', downloaded: false, size: '270KB' },
  { code: 'fi', name: 'Finnish', downloaded: false, size: '290KB' },
  { code: 'el', name: 'Greek', downloaded: false, size: '320KB' },
  { code: 'he', name: 'Hebrew', downloaded: false, size: '350KB' },
])
const espeak_downloading = ref(false)
const espeak_download_progress = ref(0)

const allPacksDownloaded = computed(() => {
  return espeak_languages.value.every(lang => lang.downloaded)
})

async function saveProvider() {
  localStorage.setItem('tts-provider', selectedProvider.value)
  // Call API to switch provider and enable
  try {
    await speechApi.switchTTSProvider(selectedProvider.value)
    await fetchStatus()
  } catch (err) {
    console.error('Failed to switch TTS provider:', err)
    error.value = t('speech.switchError')
  }
}

// Download and enable eSpeak-NG directly
async function downloadAndEnableEspeak() {
  espeak_downloading.value = true
  espeak_download_progress.value = 0
  error.value = null
  try {
    // Download all language packs
    await speechApi.downloadAllEspeakLanguages()
    await syncEspeakLanguages()

    // Enable eSpeak-NG provider
    selectedProvider.value = 'espeak-ng'
    await speechApi.switchTTSProvider('espeak-ng')
    await fetchStatus()
  } catch (e: unknown) {
    console.error('Failed to download and enable eSpeak:', e)
    error.value = t('speech.downloadError')
  } finally {
    espeak_downloading.value = false
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

async function syncEspeakLanguages() {
  try {
    const res = await speechApi.listEspeakLanguages()
    const languages = res.data?.languages || []

    // Update downloaded status from server
    for (const lang of languages) {
      const local = espeak_languages.value.find(l => l.code === lang.code)
      if (local) {
        local.downloaded = lang.downloaded
      }
    }
  } catch (e) {
    console.error('Failed to sync eSpeak languages:', e)
  }
}

onMounted(() => {
  fetchStatus()
  fetchASRModels()
  fetchTTSModels()
  syncEspeakLanguages()
})
</script>

<template>
  <div class="space-y-6">
    <!-- Tab Navigation -->
    <div class="bg-white dark:bg-gray-700 rounded-lg shadow-sm">
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
      <!-- ASR Models -->
      <div class="bg-white dark:bg-gray-700 rounded-lg p-4 shadow-sm">
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
      <div class="bg-white dark:bg-gray-700 rounded-lg p-4 shadow-sm">
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
      <div class="bg-white dark:bg-gray-700 rounded-lg p-4 shadow-sm">
        <h4 class="text-sm font-medium text-gray-900 dark:text-white mb-3">
          {{ t('speech.ttsProvider') }}
        </h4>
        <div class="space-y-2">
          <label
class="flex items-center p-3 border rounded-lg cursor-pointer transition-colors"
            :class="selectedProvider === 'edge-tts' ? 'border-gray-900 dark:border-white bg-gray-100 dark:bg-gray-700/30' : 'border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/50'">
            <input v-model="selectedProvider" type="radio" value="edge-tts" class="sr-only" @change="saveProvider" />
            <div class="flex-1">
              <span class="text-sm font-medium text-gray-900 dark:text-white">Edge TTS</span>
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('speech.edgeTTSDesc') }}</p>
            </div>
            <span v-if="selectedProvider === 'edge-tts'" class="text-gray-900 dark:text-white">
              <svg class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"/></svg>
            </span>
          </label>
          <label
class="flex items-start p-3 border rounded-lg cursor-pointer transition-colors"
            :class="selectedProvider === 'espeak-ng' ? 'border-gray-900 dark:border-white bg-gray-100 dark:bg-gray-700/30' : 'border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/50'">
            <input v-model="selectedProvider" type="radio" value="espeak-ng" class="sr-only" @change="saveProvider" />
            <div class="flex-1 min-w-0">
              <div class="flex items-center gap-2">
                <span class="text-sm font-medium text-gray-900 dark:text-white">eSpeak-NG</span>
                <span class="text-xs text-gray-400">8.7 MB</span>
              </div>
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('speech.espeakNGDesc') }}</p>
              <!-- Download Progress (inline) -->
              <div v-if="espeak_downloading" class="mt-2">
                <div class="flex items-center gap-2">
                  <div class="flex-1 bg-gray-300 dark:bg-gray-600 rounded-full h-1.5">
                    <div class="bg-gray-400 dark:bg-gray-500 h-1.5 rounded-full transition-all duration-300" :style="{ width: `${Math.floor(espeak_download_progress)}%` }"></div>
                  </div>
                  <span class="text-xs text-gray-500">{{ Math.floor(espeak_download_progress) }}%</span>
                </div>
              </div>
            </div>
            <div class="flex items-center gap-2 ml-2">
              <button
                v-if="!allPacksDownloaded"
                :disabled="espeak_downloading"
                class="px-2 py-1 bg-gray-700 dark:bg-gray-500 text-white rounded text-xs hover:bg-gray-700 dark:bg-gray-500 disabled:bg-gray-400 disabled:cursor-not-allowed transition-colors whitespace-nowrap"
                @click.prevent="downloadAndEnableEspeak"
              >
                {{ espeak_downloading ? t('speech.downloading') : t('common.download') }}
              </button>
              <span v-else class="text-xs text-green-600 dark:text-green-400">{{ t('common.downloaded') }}</span>
              <span v-if="selectedProvider === 'espeak-ng'" class="text-gray-900 dark:text-white">
                <svg class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"/></svg>
              </span>
            </div>
          </label>
        </div>
      </div>

      <!-- Voice Customization -->
      <div class="bg-white dark:bg-gray-700 rounded-lg p-4 shadow-sm">
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
      <div class="bg-white dark:bg-gray-700 rounded-lg p-4 shadow-sm">
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

    <!-- Info -->
    <div class="bg-gray-100 dark:bg-gray-700/30 rounded-lg p-4">
      <p class="text-sm text-gray-700 dark:text-gray-400">
        {{ t('speech.info') }}
      </p>
    </div>
  </div>
</template>
