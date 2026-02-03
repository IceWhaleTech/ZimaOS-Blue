<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { speechApi, type SpeechStatus, type ASRModel, type TTSModel } from '@/api/speech'

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

// TTS Provider selection
const selectedProvider = ref(localStorage.getItem('tts-provider') || '')
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
// Support multiple languages simultaneously
const espeak_selected_langs = ref<string[]>(
  JSON.parse(localStorage.getItem('espeak-langs') || '["en"]')
)

async function downloadEspeakLanguage(langCode: string) {
  const lang = espeak_languages.value.find(l => l.code === langCode)
  if (!lang) return

  // If already downloaded, just toggle it
  if (lang.downloaded) {
    toggleEspeakLanguage(langCode)
    return
  }

  espeak_downloading.value = true
  espeak_download_progress.value = 0
  error.value = null
  try {
    await speechApi.downloadEspeakLanguage(langCode)
    lang.downloaded = true
    // Add to selected languages if not already there
    if (!espeak_selected_langs.value.includes(langCode)) {
      espeak_selected_langs.value.push(langCode)
      saveEspeakLanguages()
    }
    // Sync state from server to ensure consistency
    await syncEspeakLanguages()
  } catch (e: any) {
    // If already downloaded error, mark as downloaded anyway and sync
    if (e.response?.data?.error?.includes('already downloaded')) {
      lang.downloaded = true
      if (!espeak_selected_langs.value.includes(langCode)) {
        espeak_selected_langs.value.push(langCode)
        saveEspeakLanguages()
      }
      await syncEspeakLanguages()
    } else {
      console.error('Failed to download eSpeak language:', e)
      error.value = t('speech.downloadError')
    }
  } finally {
    espeak_downloading.value = false
  }
}

function toggleEspeakLanguage(langCode: string) {
  const index = espeak_selected_langs.value.indexOf(langCode)
  if (index > -1) {
    espeak_selected_langs.value.splice(index, 1)
  } else {
    espeak_selected_langs.value.push(langCode)
  }
  saveEspeakLanguages()
}

function saveEspeakLanguages() {
  localStorage.setItem('espeak-langs', JSON.stringify(espeak_selected_langs.value))
}

function saveProvider() {
  localStorage.setItem('tts-provider', selectedProvider.value)
  // Call API to switch provider
  speechApi.switchTTSProvider(selectedProvider.value).catch(err => {
    console.error('Failed to switch TTS provider:', err)
    error.value = t('speech.switchError')
  })
}

function saveTTSModel() {
  localStorage.setItem('tts-model', selectedTTSModel.value)
  if (selectedTTSModel.value) {
    switchTTSModel(selectedTTSModel.value)
  }
}

function saveASRModel() {
  localStorage.setItem('asr-model', selectedASRModel.value)
  if (selectedASRModel.value) {
    switchASRModel(selectedASRModel.value)
  }
}

function saveSpeechRate() {
  localStorage.setItem('tts-speech-rate', speechRate.value.toString())
}

function saveSpeechPitch() {
  localStorage.setItem('tts-speech-pitch', speechPitch.value.toString())
}

function saveSpeechVolume() {
  localStorage.setItem('tts-speech-volume', speechVolume.value.toString())
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

    // Sync selected languages with downloaded status
    espeak_selected_langs.value = espeak_selected_langs.value.filter(code => {
      const lang = espeak_languages.value.find(l => l.code === code)
      return lang && lang.downloaded
    })
    saveEspeakLanguages()
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
    <!-- Speech Status Overview -->
    <div class="bg-white dark:bg-gray-800 rounded-lg p-4 shadow-sm">
      <div v-if="loading" class="flex items-center justify-center py-8">
        <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-500"></div>
      </div>

      <div v-else-if="status" class="space-y-6">
        <!-- ASR Section -->
        <div>
          <div class="flex items-center justify-between mb-3">
            <h4 class="font-medium text-gray-900 dark:text-white">{{ t('speech.asrStatus') }}</h4>
            <span
              :class="asrReady ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400' : 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400'"
              class="px-2 py-1 rounded text-xs font-medium"
            >
              {{ asrReady ? t('speech.ready') : t('speech.notReady') }}
            </span>
          </div>
          <div v-if="currentASRModel" class="text-sm text-gray-600 dark:text-gray-400">
            {{ t('speech.currentASRModel') }}: <span class="font-medium text-gray-900 dark:text-white">{{ currentASRModel }}</span>
          </div>
          <div class="text-sm text-gray-600 dark:text-gray-400 mt-1">
            {{ t('speech.editBeforeSend') }}:
            <span
              :class="editBeforeSend ? 'text-green-600 dark:text-green-400' : 'text-gray-500 dark:text-gray-500'"
              class="font-medium"
            >
              {{ editBeforeSend ? t('common.enabled') : t('common.disabled') }}
            </span>
          </div>
        </div>

        <div class="border-t border-gray-200 dark:border-gray-700"></div>

        <!-- TTS Section -->
        <div>
          <div class="flex items-center justify-between mb-3">
            <h4 class="font-medium text-gray-900 dark:text-white">{{ t('speech.ttsStatus') }}</h4>
            <span
              :class="ttsReady ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400' : 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400'"
              class="px-2 py-1 rounded text-xs font-medium"
            >
              {{ ttsReady ? t('speech.ready') : t('speech.notReady') }}
            </span>
          </div>
          <div v-if="currentTTSModel" class="text-sm text-gray-600 dark:text-gray-400">
            {{ t('speech.currentTTSModel') }}: <span class="font-medium text-gray-900 dark:text-white">{{ currentTTSModel }}</span>
          </div>
        </div>
      </div>

      <div v-if="error" class="mt-4 p-3 bg-red-100 dark:bg-red-900/30 text-red-600 dark:text-red-400 rounded-lg text-sm">
        {{ error }}
      </div>
    </div>

    <!-- ASR Models Section -->
    <div class="bg-white dark:bg-gray-800 rounded-lg p-4 shadow-sm">
      <h3 class="text-lg font-medium text-gray-900 dark:text-white mb-4">
        {{ t('speech.asrModels') }}
      </h3>

      <div class="space-y-4">
        <!-- ASR Models List -->
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
    </div>

    <!-- TTS Settings -->
    <div class="bg-white dark:bg-gray-800 rounded-lg p-4 shadow-sm">
      <h3 class="text-lg font-medium text-gray-900 dark:text-white mb-4">
        {{ t('speech.ttsSettings') }}
      </h3>

      <div class="space-y-6">
        <!-- Provider Selection -->
        <div>
          <label class="block text-sm font-medium text-gray-900 dark:text-white mb-2">
            {{ t('speech.provider') }}
          </label>
          <div class="flex gap-2">
            <select
              v-model="selectedProvider"
              class="flex-1 px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm"
            >
              <option value="">{{ t('common.select') }} TTS {{ t('common.provider') }}</option>
              <option value="espeak-ng">eSpeak-NG ({{ t('speech.offline') }})</option>
              <option value="edge-tts">Edge-TTS ({{ t('speech.online') }}, 100+ voices)</option>
            </select>
            <button
              type="button"
              @click="saveProvider"
              class="px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600 active:bg-blue-700 transition-colors text-sm font-medium whitespace-nowrap"
            >
              {{ t('common.save') }}
            </button>
          </div>
          <p v-if="selectedProvider === 'edge-tts'" class="text-xs text-yellow-600 dark:text-yellow-400 mt-2">
            ⚠️ {{ t('speech.privacyWarning') }}
          </p>
        </div>

        <!-- eSpeak-NG Language Packs (show when eSpeak-NG is selected) -->
        <div v-if="selectedProvider === 'espeak-ng'" class="bg-gray-50 dark:bg-gray-700/30 rounded-lg p-4 border border-gray-200 dark:border-gray-600">
          <h4 class="text-sm font-medium text-gray-900 dark:text-white mb-4">
            {{ t('speech.languagePacks') }}
          </h4>

          <div v-if="espeak_downloading" class="mb-4">
            <div class="flex items-center justify-between mb-2">
              <span class="text-sm text-gray-600 dark:text-gray-400">{{ t('speech.downloading') }}</span>
              <span class="text-sm font-medium text-gray-900 dark:text-white">{{ Math.floor(espeak_download_progress) }}%</span>
            </div>
            <div class="w-full bg-gray-200 dark:bg-gray-700 rounded-full h-2">
              <div
                class="bg-purple-500 h-2 rounded-full transition-all duration-300"
                :style="{ width: `${Math.floor(espeak_download_progress)}%` }"
              ></div>
            </div>
          </div>

          <div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 gap-3">
            <div
              v-for="lang in espeak_languages"
              :key="lang.code"
              class="p-3 bg-white dark:bg-gray-700 rounded-lg border border-gray-200 dark:border-gray-600 hover:border-purple-400 dark:hover:border-purple-500 transition-colors"
            >
              <div class="flex items-start justify-between gap-2">
                <div class="flex-1 min-w-0">
                  <div class="font-medium text-gray-900 dark:text-white text-sm truncate">{{ lang.name }}</div>
                  <div class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                    {{ lang.code.toUpperCase() }} · {{ lang.size }}
                  </div>
                </div>
              </div>

              <div class="mt-2">
                <template v-if="lang.downloaded">
                  <label class="flex items-center gap-2 cursor-pointer">
                    <input
                      type="checkbox"
                      :checked="espeak_selected_langs.includes(lang.code)"
                      @change="toggleEspeakLanguage(lang.code)"
                      class="w-4 h-4 rounded border-gray-300 text-purple-500 focus:ring-purple-500"
                    />
                    <span class="text-xs text-gray-700 dark:text-gray-300">
                      {{ t('speech.inUse') }}
                    </span>
                  </label>
                </template>
                <button
                  v-else
                  class="w-full px-2 py-1.5 text-xs bg-gray-200 dark:bg-gray-600 text-gray-700 dark:text-gray-200 rounded hover:bg-gray-300 dark:hover:bg-gray-500 transition-colors"
                  :disabled="espeak_downloading"
                  @click="downloadEspeakLanguage(lang.code)"
                >
                  {{ t('speech.download') }}
                </button>
              </div>
            </div>
          </div>
        </div>

        <div class="border-t border-gray-200 dark:border-gray-700"></div>

        <!-- Voice Customization -->
        <div>
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
                class="w-full h-2 bg-gray-200 dark:bg-gray-700 rounded-lg appearance-none cursor-pointer accent-accent"
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
                class="w-full h-2 bg-gray-200 dark:bg-gray-700 rounded-lg appearance-none cursor-pointer accent-accent"
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
                class="w-full h-2 bg-gray-200 dark:bg-gray-700 rounded-lg appearance-none cursor-pointer accent-accent"
                @change="saveSpeechVolume"
              />
            </div>
          </div>
        </div>

        <div class="border-t border-gray-200 dark:border-gray-700"></div>

        <!-- Auto-play TTS -->
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
            <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-accent/20 dark:peer-focus:ring-accent/40 rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full rtl:peer-checked:after:-translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:start-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-accent"></div>
          </label>
        </div>
      </div>
    </div>

    <!-- Info -->
    <div class="bg-blue-50 dark:bg-blue-900/20 rounded-lg p-4">
      <p class="text-sm text-blue-700 dark:text-blue-300">
        {{ t('speech.info') }}
      </p>
    </div>
  </div>
</template>
