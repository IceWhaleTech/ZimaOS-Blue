<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { isTtsAutoPlayEnabled, setTtsAutoPlayEnabled } from '@/utils/ttsPreferences'
import { speechApi, type SpeechStatus, type ASRModel } from '@/api/speech'
import { useTauri } from '@/composables/useTauri'
import VoiceWakeSettingsSection from '@/components/settings/VoiceWakeSettingsSection.vue'

const { t, te, locale } = useI18n()
const { openInBrowser } = useTauri()

// Tab state
const activeTab = ref<'asr' | 'tts'>('asr')

const status = ref<SpeechStatus | null>(null)
const asrModels = ref<ASRModel[]>([])
const loading = ref(false)
const switchingModelId = ref<string | null>(null) // Track which model is switching
const asrDownloadingModelId = ref<string | null>(null) // Track which model is downloading
const error = ref<string | null>(null)
const showDictationWarning = ref(false)
const recheckingDictation = ref(false)

// Kokoro state (derived from unified status response)
const kokoroComponent = computed(() => status.value?.tts?.components?.kokoro)
const kokoroReady = computed(() => kokoroComponent.value?.ready ?? false)
const _kokoroDownloadingLocal = ref(false) // optimistic flag during download initiation
const kokoroDownloading = computed(
  () => _kokoroDownloadingLocal.value || (kokoroComponent.value?.downloading ?? false)
)
const kokoroDownloadProgress = computed(() => kokoroComponent.value?.progress ?? 0)
const kokoroDownloadSpeed = computed(() => kokoroComponent.value?.speed ?? '')
const kokoroDownloadETA = computed(() => kokoroComponent.value?.eta ?? '')
const kokoroDownloadFile = computed(() => kokoroComponent.value?.file ?? '')
const kokoroDownloadFileIndex = computed(() => kokoroComponent.value?.file_index ?? 0)
const kokoroDownloadTotalFiles = computed(() => kokoroComponent.value?.total_files ?? 0)
const kokoroDownloadedHuman = computed(() => kokoroComponent.value?.downloaded_human ?? '')
const kokoroLangsExpanded = ref(false)

const kokoroLanguages = [
  'en-US',
  'en-GB',
  'ja-JP',
  'zh-CN',
  'es-ES',
  'fr-FR',
  'hi-IN',
  'it-IT',
  'pt-BR',
]
const kokoroCurrentLangSupported = computed(() => kokoroLanguages.includes(locale.value))
const kokoroOtherLangs = computed(() => kokoroLanguages.filter((l) => l !== locale.value))

// Offline dictation languages (macOS native STT) — fetched lazily via separate endpoint
const offlineLanguages = ref<string[]>([])
const offlineLangsFetched = ref(false)
const currentLangOfflineInstalled = computed(() => {
  const langs = offlineLanguages.value
  if (!langs.length) return false
  const cur = locale.value
  return langs.includes(cur) || langs.some((l) => l.split('-')[0] === cur.split('-')[0])
})

// Display name for a locale code — reuse speech.langName, fallback to Intl.DisplayNames
const langDisplayNames = new Intl.DisplayNames([locale.value], { type: 'language' })
function langName(code: string): string {
  const i18nKey = `speech.langName.${code}`
  if (te(i18nKey)) return t(i18nKey)
  try {
    return langDisplayNames.of(code) ?? code
  } catch {
    return code
  }
}

function trModelText(value: string): string {
  return te(value) ? t(value) : value
}

// eSpeak-NG state (derived from unified status response)
const espeakDataReady = computed(() => status.value?.espeak?.installed ?? false)
const espeakLangCount = computed(() => status.value?.espeak?.language_count ?? 0)
const espeakDataSize = computed(() => status.value?.espeak?.data_size ?? 0)

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
    eta: download.progress.eta,
  }
}

// Check if a specific model is downloading
function isModelDownloading(modelId: string) {
  return (
    serverDownloadingASRModels.value.includes(modelId) || asrDownloadingModelId.value === modelId
  )
}

const currentASRModel = computed(() => status.value?.asr?.model_name ?? '')

// Available TTS providers from server (respects build tags)
const availableProviders = computed(() => status.value?.tts?.available_providers ?? [])
function isProviderAvailable(provider: string) {
  // Wait for status to load before showing providers
  if (!status.value || availableProviders.value.length === 0) return false
  return availableProviders.value.includes(provider)
}

// TTS Provider selection - synced from server status
const selectedProvider = ref(localStorage.getItem('tts-provider') || '')

// Voice customization
const speechRate = ref(parseFloat(localStorage.getItem('tts-speech-rate') || '1.0'))
const speechPitch = ref(parseFloat(localStorage.getItem('tts-speech-pitch') || '0'))
const speechVolume = ref(parseFloat(localStorage.getItem('tts-speech-volume') || '100'))

// Auto-play TTS for assistant responses
const autoPlayTTS = ref(isTtsAutoPlayEnabled())

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
  setTtsAutoPlayEnabled(autoPlayTTS.value)
}

async function fetchStatus() {
  loading.value = true
  error.value = null
  try {
    const res = await speechApi.getStatus()
    status.value = res.data
    // Populate models from status response, filter out native providers (shown separately)
    asrModels.value = (res.data?.asr?.models || []).filter(
      (m: ASRModel) => m.id !== 'macos-native' && m.id !== 'windows-native'
    )
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
        await fetchStatus()
      }
    }, 1000)
  } catch (e) {
    console.error('Failed to download ASR model:', e)
    error.value = t('speech.downloadError')
    asrDownloadingModelId.value = null
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

async function cancelASRDownload() {
  try {
    await speechApi.cancelASRDownload()
    asrDownloadingModelId.value = null
    await fetchStatus()
  } catch (e) {
    console.error('Failed to cancel download:', e)
    error.value = t('speech.cancelError')
  }
}

async function downloadKokoro() {
  _kokoroDownloadingLocal.value = true
  error.value = null
  try {
    await speechApi.downloadKokoro()
    let pollCount = 0
    const pollInterval = setInterval(async () => {
      await fetchStatus()
      pollCount++
      const comp = status.value?.tts?.components?.kokoro
      // Grace period: don't stop polling in first 3s (goroutine may not have started yet)
      if (comp && !comp.downloading && pollCount > 3) {
        _kokoroDownloadingLocal.value = false
        clearInterval(pollInterval)
      }
    }, 1000)
  } catch (e) {
    console.error('Failed to download Kokoro model:', e)
    error.value = t('speech.downloadError')
    _kokoroDownloadingLocal.value = false
  }
}

async function cancelKokoroDownload() {
  try {
    await speechApi.cancelKokoroDownload()
    _kokoroDownloadingLocal.value = false
    await fetchStatus()
  } catch (e) {
    console.error('Failed to cancel Kokoro download:', e)
    error.value = t('speech.cancelError')
  }
}

function openMacOSSettings() {
  // Open macOS System Settings > Privacy & Security > Speech Recognition
  openInBrowser('x-apple.systempreferences:com.apple.preference.security?Privacy_SpeechRecognition')
}

async function toggleOnDevice() {
  if (!status.value?.asr) return
  const newValue = !status.value.asr.on_device_only
  showDictationWarning.value = false
  try {
    const resp = await speechApi.setASROnDevice(newValue)
    if (resp.data.error === 'dictation_disabled') {
      showDictationWarning.value = true
      return
    }
    if (status.value.asr) {
      status.value.asr.on_device_only = resp.data.on_device_only
      status.value.asr.on_device_supported = resp.data.on_device_supported
      status.value.asr.dictation_available = resp.data.dictation_available
    }
    // Fetch offline languages when enabling on-device
    if (resp.data.on_device_only) {
      offlineLangsFetched.value = false
      fetchOfflineLanguages()
    }
  } catch (e) {
    console.error('Failed to toggle on-device mode', e)
  }
}

async function recheckDictation() {
  recheckingDictation.value = true
  try {
    const resp = await speechApi.setASROnDevice(true)
    if (resp.data.error === 'dictation_disabled') {
      // Still not enabled
      return
    }
    // Passed — on-device is now enabled
    showDictationWarning.value = false
    if (status.value?.asr) {
      status.value.asr.on_device_only = resp.data.on_device_only
      status.value.asr.on_device_supported = resp.data.on_device_supported
      status.value.asr.dictation_available = resp.data.dictation_available
    }
    // Fetch offline languages after recheck succeeds
    if (resp.data.on_device_only) {
      offlineLangsFetched.value = false
      fetchOfflineLanguages()
    }
  } catch (e) {
    console.error('Failed to recheck dictation', e)
  } finally {
    recheckingDictation.value = false
  }
}

async function fetchOfflineLanguages() {
  if (offlineLangsFetched.value) return
  try {
    const res = await speechApi.getOfflineLanguages()
    offlineLanguages.value = res.data?.offline_languages ?? []
    offlineLangsFetched.value = true
  } catch (e) {
    console.error('Failed to fetch offline languages:', e)
  }
}

// Lazy-fetch offline languages when on-device mode is active
watch(
  () => status.value?.asr?.on_device_only,
  (onDevice) => {
    if (onDevice && status.value?.asr?.dictation_available) {
      fetchOfflineLanguages()
    }
  },
  { immediate: true }
)

onMounted(async () => {
  await fetchStatus()
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
              : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-200',
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
              : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-200',
          ]"
          @click="activeTab = 'tts'"
        >
          {{ t('speech.ttsTab') }}
        </button>
      </div>
    </div>

    <!-- Error Message -->
    <div
      v-if="error"
      class="p-3 bg-red-100 dark:bg-red-900/30 text-red-600 dark:text-red-400 rounded-lg text-sm"
    >
      {{ error }}
    </div>

    <!-- ASR Tab Content -->
    <div v-show="activeTab === 'asr'" class="space-y-6">
      <!-- macOS Native STT Status -->
      <div
        v-if="status?.asr?.provider === 'macos-native'"
        class="bg-white dark:bg-gray-700/30 rounded-lg p-4 shadow-sm"
      >
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-3">
            <div
              class="w-8 h-8 rounded-lg flex items-center justify-center"
              :class="
                status?.asr?.permission_denied
                  ? 'bg-amber-100 dark:bg-amber-900/30'
                  : 'bg-blue-100 dark:bg-blue-900/30'
              "
            >
              <!-- Warning icon when permission denied -->
              <svg
                v-if="status?.asr?.permission_denied"
                class="w-4 h-4 text-amber-600 dark:text-amber-400"
                viewBox="0 0 20 20"
                fill="currentColor"
              >
                <path
                  fill-rule="evenodd"
                  d="M8.485 2.495c.673-1.167 2.357-1.167 3.03 0l6.28 10.875c.673 1.167-.17 2.625-1.516 2.625H3.72c-1.347 0-2.189-1.458-1.515-2.625L8.485 2.495zM10 5a.75.75 0 01.75.75v3.5a.75.75 0 01-1.5 0v-3.5A.75.75 0 0110 5zm0 9a1 1 0 100-2 1 1 0 000 2z"
                  clip-rule="evenodd"
                />
              </svg>
              <!-- Apple icon when OK -->
              <svg
                v-else
                class="w-4 h-4 text-blue-600 dark:text-blue-400 dark:drop-shadow-[0_0_2px_rgba(255,255,255,0.5)]"
                viewBox="0 0 24 24"
                fill="currentColor"
              >
                <path
                  d="M18.71 19.5C17.88 20.74 17 21.95 15.66 21.97C14.32 22 13.89 21.18 12.37 21.18C10.84 21.18 10.37 21.95 9.1 22C7.79 22.05 6.8 20.68 5.96 19.47C4.25 16.56 2.93 11.3 4.7 7.72C5.57 5.94 7.36 4.86 9.28 4.84C10.56 4.81 11.78 5.72 12.57 5.72C13.36 5.72 14.85 4.62 16.4 4.8C17.07 4.83 18.89 5.08 20.07 6.77C19.96 6.84 17.62 8.23 17.65 11.1C17.68 14.54 20.59 15.62 20.63 15.63C20.59 15.72 20.12 17.37 18.71 19.5ZM13 3.5C13.73 2.67 14.94 2.04 15.94 2C16.07 3.17 15.6 4.35 14.9 5.19C14.21 6.04 13.07 6.7 11.95 6.61C11.8 5.46 12.36 4.26 13 3.5Z"
                />
              </svg>
            </div>
            <div>
              <span class="text-sm font-medium text-gray-900 dark:text-white">{{
                t('speech.macosNativeName')
              }}</span>
              <p
                v-if="status?.asr?.permission_denied"
                class="text-xs text-amber-600 dark:text-amber-400"
              >
                {{ t('speech.macosNativePermissionDenied') }}
              </p>
              <p v-else class="text-xs text-gray-500 dark:text-gray-400">
                {{ t('speech.macosNativeDesc') }}
              </p>
            </div>
          </div>
          <span
            v-if="status?.asr?.ready"
            class="text-xs px-2 py-1 rounded-full bg-green-100 dark:bg-green-900/30 text-green-600 dark:text-green-400"
            >{{ t('speech.ready') }}</span
          >
          <span
            v-else-if="status?.asr?.permission_denied"
            class="text-xs px-2 py-1 rounded-full bg-amber-100 dark:bg-amber-900/30 text-amber-600 dark:text-amber-400"
            >{{ t('speech.macosNativePermissionDenied') }}</span
          >
        </div>
        <!-- Permission guide -->
        <div
          v-if="status?.asr?.permission_denied"
          class="mt-3 p-3 bg-amber-50 dark:bg-amber-900/20 rounded-lg border border-amber-200 dark:border-amber-800/50"
        >
          <p class="text-xs text-amber-700 dark:text-amber-300 mb-2">
            {{
              t('speech.macosNativePermissionGuide', {
                appName: status?.asr?.permission_app_name || 'Terminal',
              })
            }}
          </p>
          <button
            class="px-3 py-1.5 bg-amber-600 hover:bg-amber-700 text-white rounded-lg text-xs font-medium transition-colors"
            @click="openMacOSSettings"
          >
            {{ t('speech.macosNativeOpenSettings') }}
          </button>
        </div>
        <!-- On-device toggle -->
        <div v-if="status?.asr?.ready" class="mt-3 p-3 bg-gray-50 dark:bg-gray-700/50 rounded-lg">
          <div class="flex items-center justify-between">
            <div class="speech-settings-inline-end-gap flex-1">
              <span class="text-sm font-medium text-gray-900 dark:text-white">{{
                t('speech.macosNativeOnDeviceOnly')
              }}</span>
              <p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
                {{ t('speech.macosNativeOnDeviceDesc') }}
              </p>
              <p
                v-if="!status?.asr?.on_device_supported && status?.asr?.on_device_only"
                class="text-xs text-amber-500 dark:text-amber-400 mt-0.5"
              >
                {{ t('speech.macosNativeOnDeviceUnsupported') }}
              </p>
            </div>
            <button
              class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none"
              :class="
                status?.asr?.on_device_only
                  ? 'bg-green-600 dark:bg-green-500'
                  : 'bg-gray-300 dark:bg-gray-600'
              "
              @click="toggleOnDevice"
            >
              <span
                class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
                :class="status?.asr?.on_device_only ? 'translate-x-5' : 'translate-x-0'"
              />
            </button>
          </div>
          <!-- Offline dictation languages (shown when on-device is enabled and dictation is available) -->
          <div
            v-if="
              status?.asr?.on_device_only &&
              status?.asr?.dictation_available &&
              offlineLanguages.length > 0
            "
            class="mt-3 pt-3 border-t border-gray-200 dark:border-gray-600"
          >
            <!-- Windows Native STT Status -->
            <div
              v-if="String(status?.asr?.provider) === 'windows-native'"
              class="bg-white dark:bg-gray-700/30 rounded-lg p-4 shadow-sm"
            >
              <div class="flex items-center justify-between">
                <div class="flex items-center gap-3">
                  <div
                    class="w-8 h-8 rounded-lg flex items-center justify-center bg-blue-100 dark:bg-blue-900/30"
                  >
                    <!-- Windows icon -->
                    <svg
                      class="w-4 h-4 text-[#0078D4] dark:drop-shadow-[0_0_2px_rgba(255,255,255,0.5)]"
                      viewBox="0 0 24 24"
                      fill="currentColor"
                    >
                      <path
                        d="M0,0 L10.5,0 L10.5,10.5 L0,10.5 Z M12,0 L24,0 L24,10.5 L12,10.5 Z M0,12 L10.5,12 L10.5,24 L0,24 Z M12,12 L24,12 L24,24 L12,24 Z"
                      />
                    </svg>
                  </div>
                  <div>
                    <span class="text-sm font-medium text-gray-900 dark:text-white">{{
                      t('speech.windowsNativeName')
                    }}</span>
                    <p class="text-xs text-gray-500 dark:text-gray-400">
                      {{ t('speech.windowsNativeASRDesc') }}
                    </p>
                  </div>
                </div>
                <span
                  v-if="status?.asr?.ready"
                  class="text-xs px-2 py-1 rounded-full bg-green-100 dark:bg-green-900/30 text-green-600 dark:text-green-400"
                  >{{ t('speech.ready') }}</span
                >
              </div>
            </div>
            <div class="flex items-center justify-between mb-2">
              <span class="text-xs font-medium text-gray-700 dark:text-gray-300">{{
                t('speech.offlineLanguages')
              }}</span>
              <span
                v-if="currentLangOfflineInstalled"
                class="text-xs px-2 py-0.5 rounded-full bg-green-100 dark:bg-green-900/30 text-green-600 dark:text-green-400"
              >
                {{ t('speech.currentLangInstalled') }}
              </span>
              <span
                v-else
                class="text-xs px-2 py-0.5 rounded-full bg-amber-100 dark:bg-amber-900/30 text-amber-600 dark:text-amber-400"
              >
                {{ t('speech.currentLangNotInstalled') }}
              </span>
            </div>
            <div class="flex flex-wrap gap-1.5">
              <span
                v-for="lang in offlineLanguages"
                :key="lang"
                class="text-xs px-2 py-0.5 rounded-full"
                :class="
                  lang.split('-')[0] === locale.split('-')[0]
                    ? 'bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300 font-medium'
                    : 'bg-gray-200 dark:bg-gray-600 text-gray-600 dark:text-gray-300'
                "
                >{{ langName(lang) }}</span
              >
            </div>
          </div>
        </div>
        <!-- Dictation disabled warning (shown when user tries to enable on-device but dictation is off) -->
        <div
          v-if="showDictationWarning"
          class="mt-3 p-3 bg-amber-50 dark:bg-amber-900/20 rounded-lg border border-amber-200 dark:border-amber-800/50"
        >
          <p class="text-xs text-amber-700 dark:text-amber-300 mb-2">
            {{ t('speech.dictationDisabledGuide') }}
          </p>
          <div class="flex gap-2">
            <button
              class="px-3 py-1.5 bg-amber-600 hover:bg-amber-700 text-white rounded-lg text-xs font-medium transition-colors"
              @click="
                openInBrowser('x-apple.systempreferences:com.apple.Keyboard-Settings.extension')
              "
            >
              {{ t('speech.macosNativeOpenSettings') }}
            </button>
            <button
              class="px-3 py-1.5 bg-gray-200 dark:bg-gray-600 hover:bg-gray-300 dark:hover:bg-gray-500 text-gray-700 dark:text-white rounded-lg text-xs font-medium transition-colors"
              :disabled="recheckingDictation"
              @click="recheckDictation"
            >
              {{ recheckingDictation ? t('common.checking') : t('speech.recheckDictation') }}
            </button>
          </div>
        </div>
      </div>

      <VoiceWakeSettingsSection />

      <!-- Windows Native STT Status -->
      <div
        v-if="status?.asr?.provider === 'windows-native'"
        class="bg-white dark:bg-gray-700/30 rounded-lg p-4 shadow-sm"
      >
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-3">
            <div
              class="w-8 h-8 rounded-lg bg-blue-100 dark:bg-blue-900/30 flex items-center justify-center"
            >
              <svg
                class="w-4 h-4 text-blue-600 dark:text-blue-400 dark:drop-shadow-[0_0_2px_rgba(255,255,255,0.5)]"
                viewBox="0 0 24 24"
                fill="currentColor"
              >
                <path
                  d="M0,0 L10.5,0 L10.5,10.5 L0,10.5 Z M12,0 L24,0 L24,10.5 L12,10.5 Z M0,12 L10.5,12 L10.5,24 L0,24 Z M12,12 L24,12 L24,24 L12,24 Z"
                />
              </svg>
            </div>
            <div>
              <span class="text-sm font-medium text-gray-900 dark:text-white">{{
                t('speech.windowsNativeName')
              }}</span>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                {{ t('speech.windowsNativeASRDesc') }}
              </p>
            </div>
          </div>
          <span
            v-if="status?.asr?.ready"
            class="text-xs px-2 py-1 rounded-full bg-green-100 dark:bg-green-900/30 text-green-600 dark:text-green-400"
            >{{ t('speech.ready') }}</span
          >
        </div>
      </div>

      <div
        v-if="asrModels.length > 0"
        class="bg-white dark:bg-gray-700/30 rounded-lg p-4 shadow-sm"
      >
        <h4 class="text-sm font-medium text-gray-900 dark:text-white mb-3">
          {{ t('speech.asrModels') }}
        </h4>
        <div class="space-y-2">
          <div
            v-for="model in asrModels"
            :key="model.id"
            class="p-3 border rounded-lg transition-colors"
            :class="
              currentASRModel === model.id
                ? 'border-gray-900 dark:border-white bg-gray-100 dark:bg-gray-700/30'
                : 'border-gray-200 dark:border-gray-700'
            "
          >
            <label
              class="flex items-center cursor-pointer"
              :class="{
                'opacity-50 cursor-not-allowed':
                  model.permission_denied || (!model.downloaded && !isModelDownloading(model.id)),
              }"
            >
              <input
                type="radio"
                :value="model.id"
                :checked="currentASRModel === model.id"
                :disabled="model.permission_denied || !model.downloaded || !!switchingModelId"
                class="sr-only"
                @change="switchASRModel(model.id)"
              />
              <div class="flex-1">
                <div class="flex items-center gap-2">
                  <!-- Warning icon for permission denied -->
                  <svg
                    v-if="model.permission_denied"
                    class="w-4 h-4 text-amber-500"
                    viewBox="0 0 20 20"
                    fill="currentColor"
                  >
                    <path
                      fill-rule="evenodd"
                      d="M8.485 2.495c.673-1.167 2.357-1.167 3.03 0l6.28 10.875c.673 1.167-.17 2.625-1.516 2.625H3.72c-1.347 0-2.189-1.458-1.515-2.625L8.485 2.495zM10 5a.75.75 0 01.75.75v3.5a.75.75 0 01-1.5 0v-3.5A.75.75 0 0110 5zm0 9a1 1 0 100-2 1 1 0 000 2z"
                      clip-rule="evenodd"
                    />
                  </svg>
                  <!-- Apple icon for macOS native -->
                  <svg
                    v-else-if="model.id === 'macos-native'"
                    class="w-4 h-4 text-blue-600 dark:text-blue-400 dark:drop-shadow-[0_0_2px_rgba(255,255,255,0.5)]"
                    viewBox="0 0 24 24"
                    fill="currentColor"
                  >
                    <path
                      d="M18.71 19.5C17.88 20.74 17 21.95 15.66 21.97C14.32 22 13.89 21.18 12.37 21.18C10.84 21.18 10.37 21.95 9.1 22C7.79 22.05 6.8 20.68 5.96 19.47C4.25 16.56 2.93 11.3 4.7 7.72C5.57 5.94 7.36 4.86 9.28 4.84C10.56 4.81 11.78 5.72 12.57 5.72C13.36 5.72 14.85 4.62 16.4 4.8C17.07 4.83 18.89 5.08 20.07 6.77C19.96 6.84 17.62 8.23 17.65 11.1C17.68 14.54 20.59 15.62 20.63 15.63C20.59 15.72 20.12 17.37 18.71 19.5ZM13 3.5C13.73 2.67 14.94 2.04 15.94 2C16.07 3.17 15.6 4.35 14.9 5.19C14.21 6.04 13.07 6.7 11.95 6.61C11.8 5.46 12.36 4.26 13 3.5Z"
                    />
                  </svg>
                  <!-- Windows icon for Windows native -->
                  <svg
                    v-else-if="model.id === 'windows-native'"
                    class="w-4 h-4 text-[#0078D4] dark:drop-shadow-[0_0_2px_rgba(255,255,255,0.5)]"
                    viewBox="0 0 24 24"
                    fill="currentColor"
                  >
                    <path
                      d="M0,0 L10.5,0 L10.5,10.5 L0,10.5 Z M12,0 L24,0 L24,10.5 L12,10.5 Z M0,12 L10.5,12 L10.5,24 L0,24 Z M12,12 L24,12 L24,24 L12,24 Z"
                    />
                  </svg>
                  <!-- Whisper / AI icon -->
                  <svg
                    v-else
                    class="w-4 h-4 text-gray-600 dark:text-gray-300"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      d="M19 11a7 7 0 01-7 7m0 0a7 7 0 01-7-7m7 7v4m0 0H8m4 0h4m-4-8a3 3 0 01-3-3V5a3 3 0 116 0v6a3 3 0 01-3 3z"
                    />
                  </svg>
                  <span class="text-sm font-medium text-gray-900 dark:text-white">{{
                    trModelText(model.name)
                  }}</span>
                  <span
                    v-if="model.size"
                    class="text-xs px-1.5 py-0.5 rounded bg-gray-100 dark:bg-gray-600/50 text-gray-600 dark:text-gray-300"
                    >{{ model.size }}</span
                  >
                  <span
                    v-if="model.recommended"
                    class="text-xs px-1.5 py-0.5 rounded bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400"
                    >{{ t('remoteAccess.recommended') }}</span
                  >
                  <span
                    v-if="model.streaming"
                    class="text-xs px-1.5 py-0.5 rounded bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-400"
                    >Streaming</span
                  >
                </div>
                <p
                  v-if="model.permission_denied"
                  class="text-xs text-amber-500 dark:text-amber-400 mt-0.5"
                >
                  {{ t('speech.macosNativePermissionDenied') }}
                </p>
                <p v-else class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
                  {{ trModelText(model.description) }}
                </p>
              </div>
              <!-- Checkmark for active model -->
              <span
                v-if="currentASRModel === model.id"
                class="speech-settings-inline-start-gap text-gray-900 dark:text-white flex-shrink-0"
              >
                <svg class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
                  <path
                    fill-rule="evenodd"
                    d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z"
                    clip-rule="evenodd"
                  />
                </svg>
              </span>
              <!-- Switching spinner -->
              <span
                v-else-if="switchingModelId === model.id"
                class="speech-settings-inline-start-gap text-gray-900 dark:text-white text-xs font-medium flex items-center gap-1 flex-shrink-0"
              >
                <svg
                  class="animate-spin h-4 w-4"
                  xmlns="http://www.w3.org/2000/svg"
                  fill="none"
                  viewBox="0 0 24 24"
                >
                  <circle
                    class="opacity-25"
                    cx="12"
                    cy="12"
                    r="10"
                    stroke="currentColor"
                    stroke-width="4"
                  ></circle>
                  <path
                    class="opacity-75"
                    fill="currentColor"
                    d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                  ></path>
                </svg>
              </span>
              <!-- Download button (right side) -->
              <button
                v-else-if="!model.downloaded && !isModelDownloading(model.id)"
                class="speech-settings-inline-start-gap px-3 py-1 bg-gray-900 dark:bg-gray-500 text-white rounded-lg hover:bg-gray-700 dark:hover:bg-gray-600 text-xs font-medium flex-shrink-0"
                @click.prevent="downloadASRModel(model.id)"
              >
                {{ t('common.download') }}
              </button>
            </label>
            <!-- Download progress -->
            <div v-if="isModelDownloading(model.id)" class="mt-2">
              <div class="flex items-center gap-2">
                <div class="flex-1 bg-gray-300 dark:bg-gray-600 rounded-full h-1.5">
                  <div
                    class="bg-gray-500 dark:bg-gray-400 h-1.5 rounded-full transition-all duration-300"
                    :style="{
                      width: `${Math.floor(getModelProgress(model.id)?.percentage || 0)}%`,
                    }"
                  ></div>
                </div>
                <span class="text-xs text-gray-500"
                  >{{ Math.floor(getModelProgress(model.id)?.percentage || 0) }}%</span
                >
                <button class="text-red-500 hover:text-red-600 text-xs" @click="cancelASRDownload">
                  {{ t('common.cancel') }}
                </button>
              </div>
              <div
                v-if="getModelProgress(model.id)"
                class="flex items-center gap-3 mt-1 text-xs text-gray-400"
              >
                <span v-if="getModelProgress(model.id)?.speed">{{
                  getModelProgress(model.id)?.speed
                }}</span>
                <span v-if="getModelProgress(model.id)?.eta"
                  >{{ $t('speech.eta') }}: {{ getModelProgress(model.id)?.eta }}</span
                >
                <span v-if="getModelProgress(model.id)?.total"
                  >{{ Math.round((getModelProgress(model.id)?.downloaded || 0) / 1024 / 1024) }}MB /
                  {{ Math.round((getModelProgress(model.id)?.total || 0) / 1024 / 1024) }}MB</span
                >
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Edit Before Send Toggle — deprecated, replaced by swipe gesture in ChatInput -->
      <!-- Setting kept in store for backward compat but hidden from UI -->
    </div>

    <!-- TTS Tab Content -->
    <div v-show="activeTab === 'tts'" class="space-y-6">
      <!-- TTS Provider Selection -->
      <div class="bg-white dark:bg-gray-700/30 rounded-lg p-4 shadow-sm">
        <h4 class="text-sm font-medium text-gray-900 dark:text-white mb-3">
          {{ t('speech.ttsProvider') }}
        </h4>
        <!-- Loading skeleton while status is being fetched -->
        <div v-if="loading || !status" class="space-y-2">
          <div
            v-for="i in 2"
            :key="i"
            class="p-3 border border-gray-200 dark:border-gray-700 rounded-lg animate-pulse"
          >
            <div class="flex items-center gap-2">
              <div class="w-4 h-4 bg-gray-200 dark:bg-gray-600 rounded"></div>
              <div class="h-4 w-24 bg-gray-200 dark:bg-gray-600 rounded"></div>
            </div>
            <div class="h-3 w-48 bg-gray-100 dark:bg-gray-700 rounded mt-1.5"></div>
          </div>
        </div>
        <div v-else class="space-y-2">
          <label
            v-if="isProviderAvailable('macos-native')"
            class="flex items-center p-3 border rounded-lg cursor-pointer transition-colors"
            :class="
              selectedProvider === 'macos-native'
                ? 'border-gray-900 dark:border-white bg-gray-100 dark:bg-gray-700/30'
                : 'border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/50'
            "
          >
            <input
              v-model="selectedProvider"
              type="radio"
              value="macos-native"
              class="sr-only"
              @change="saveProvider"
            />
            <div class="flex-1">
              <div class="flex items-center gap-2">
                <svg
                  class="w-4 h-4 text-blue-600 dark:text-blue-400 dark:drop-shadow-[0_0_2px_rgba(255,255,255,0.5)]"
                  viewBox="0 0 24 24"
                  fill="currentColor"
                >
                  <path
                    d="M18.71 19.5C17.88 20.74 17 21.95 15.66 21.97C14.32 22 13.89 21.18 12.37 21.18C10.84 21.18 10.37 21.95 9.1 22C7.79 22.05 6.8 20.68 5.96 19.47C4.25 16.56 2.93 11.3 4.7 7.72C5.57 5.94 7.36 4.86 9.28 4.84C10.56 4.81 11.78 5.72 12.57 5.72C13.36 5.72 14.85 4.62 16.4 4.8C17.07 4.83 18.89 5.08 20.07 6.77C19.96 6.84 17.62 8.23 17.65 11.1C17.68 14.54 20.59 15.62 20.63 15.63C20.59 15.72 20.12 17.37 18.71 19.5ZM13 3.5C13.73 2.67 14.94 2.04 15.94 2C16.07 3.17 15.6 4.35 14.9 5.19C14.21 6.04 13.07 6.7 11.95 6.61C11.8 5.46 12.36 4.26 13 3.5Z"
                  />
                </svg>
                <span class="text-sm font-medium text-gray-900 dark:text-white">{{
                  t('speech.macosNativeName')
                }}</span>
                <span
                  class="text-xs px-1.5 py-0.5 rounded bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-400"
                  >{{ t('speech.macosNativeQuality') }}</span
                >
              </div>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                {{ t('speech.macosNativeDesc') }}
              </p>
            </div>
            <span v-if="selectedProvider === 'macos-native'" class="text-gray-900 dark:text-white">
              <svg class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
                <path
                  fill-rule="evenodd"
                  d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z"
                  clip-rule="evenodd"
                />
              </svg>
            </span>
          </label>
          <label
            v-if="isProviderAvailable('windows-native')"
            class="flex items-center p-3 border rounded-lg cursor-pointer transition-colors"
            :class="
              selectedProvider === 'windows-native'
                ? 'border-gray-900 dark:border-white bg-gray-100 dark:bg-gray-700/30'
                : 'border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/50'
            "
          >
            <input
              v-model="selectedProvider"
              type="radio"
              value="windows-native"
              class="sr-only"
              @change="saveProvider"
            />
            <div class="flex-1">
              <div class="flex items-center gap-2">
                <svg
                  class="w-4 h-4 text-[#0078D4] dark:drop-shadow-[0_0_2px_rgba(255,255,255,0.5)]"
                  viewBox="0 0 24 24"
                  fill="currentColor"
                >
                  <path
                    d="M0,0 L10.5,0 L10.5,10.5 L0,10.5 Z M12,0 L24,0 L24,10.5 L12,10.5 Z M0,12 L10.5,12 L10.5,24 L0,24 Z M12,12 L24,12 L24,24 L12,24 Z"
                  />
                </svg>
                <span class="text-sm font-medium text-gray-900 dark:text-white">{{
                  t('speech.windowsNativeName')
                }}</span>
                <span
                  class="text-xs px-1.5 py-0.5 rounded bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-400"
                  >{{ t('speech.systemNative') }}</span
                >
              </div>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                {{ t('speech.windowsNativeDesc') }}
              </p>
            </div>
            <span
              v-if="selectedProvider === 'windows-native'"
              class="text-gray-900 dark:text-white"
            >
              <svg class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
                <path
                  fill-rule="evenodd"
                  d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z"
                  clip-rule="evenodd"
                />
              </svg>
            </span>
          </label>
          <label
            v-if="isProviderAvailable('edge-tts')"
            class="flex items-center p-3 border rounded-lg cursor-pointer transition-colors"
            :class="
              selectedProvider === 'edge-tts'
                ? 'border-gray-900 dark:border-white bg-gray-100 dark:bg-gray-700/30'
                : 'border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/50'
            "
          >
            <input
              v-model="selectedProvider"
              type="radio"
              value="edge-tts"
              class="sr-only"
              @change="saveProvider"
            />
            <div class="flex-1">
              <div class="flex items-center gap-2">
                <svg
                  class="w-4 h-4 text-[#0078D4] dark:drop-shadow-[0_0_2px_rgba(255,255,255,0.5)]"
                  viewBox="0 0 24 24"
                  fill="currentColor"
                >
                  <path
                    d="M21.17 3.25Q21.5 3.25 21.76 3.5 22 3.74 22 4.08V19.92Q22 20.26 21.76 20.5 21.5 20.75 21.17 20.75H2.83Q2.5 20.75 2.24 20.5 2 20.26 2 19.92V4.08Q2 3.74 2.24 3.5 2.5 3.25 2.83 3.25ZM12.67 12.13Q12.67 10.41 11.78 9.5 10.89 8.58 9.33 8.58 7.78 8.58 6.89 9.5 6 10.41 6 12.13 6 13.84 6.89 14.76 7.78 15.67 9.33 15.67 10.89 15.67 11.78 14.76 12.67 13.84 12.67 12.13ZM18 8.75H14.5V9.92H18ZM18 11.42H14.5V12.58H18ZM18 14.08H14.5V15.25H18Z"
                  />
                </svg>
                <span class="text-sm font-medium text-gray-900 dark:text-white">{{
                  t('speech.edgeTTSName')
                }}</span>
                <span
                  class="text-xs px-1.5 py-0.5 rounded bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400"
                  >{{ t('speech.edgeTTSQuality') }}</span
                >
              </div>
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('speech.edgeTTSDesc') }}</p>
            </div>
            <span v-if="selectedProvider === 'edge-tts'" class="text-gray-900 dark:text-white">
              <svg class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
                <path
                  fill-rule="evenodd"
                  d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z"
                  clip-rule="evenodd"
                />
              </svg>
            </span>
          </label>
          <!-- eSpeak-NG + HiFi-GAN -->
          <div
            v-if="isProviderAvailable('espeak-ng')"
            class="p-3 border rounded-lg transition-colors"
            :class="
              selectedProvider === 'espeak-ng'
                ? 'border-gray-900 dark:border-white bg-gray-100 dark:bg-gray-700/30'
                : 'border-gray-200 dark:border-gray-700'
            "
          >
            <label class="flex items-center cursor-pointer">
              <input
                v-model="selectedProvider"
                type="radio"
                value="espeak-ng"
                class="sr-only"
                @change="saveProvider"
                :disabled="!espeakDataReady"
              />
              <div class="flex-1">
                <div class="flex items-center gap-2">
                  <span class="text-sm font-medium text-gray-900 dark:text-white">eSpeak-NG</span>
                  <span
                    class="text-xs px-1.5 py-0.5 rounded bg-orange-100 dark:bg-orange-900/30 text-orange-700 dark:text-orange-400"
                    >{{ t('speech.espeakNGQuality') }}</span
                  >
                </div>
                <p class="text-xs text-gray-500 dark:text-gray-400">
                  {{ t('speech.espeakNGDesc') }}
                </p>
              </div>
              <span v-if="selectedProvider === 'espeak-ng'" class="text-gray-900 dark:text-white">
                <svg class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
                  <path
                    fill-rule="evenodd"
                    d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z"
                    clip-rule="evenodd"
                  />
                </svg>
              </span>
            </label>
            <!-- Status -->
            <div class="mt-2">
              <div
                v-if="espeakDataReady"
                class="flex items-center gap-1 text-xs text-green-600 dark:text-green-400"
              >
                <svg class="w-3.5 h-3.5" fill="currentColor" viewBox="0 0 20 20">
                  <path
                    fill-rule="evenodd"
                    d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z"
                    clip-rule="evenodd"
                  />
                </svg>
                {{ t('speech.espeakDepLib') }}
                <span class="text-gray-400"
                  >({{ espeakLangCount }} {{ t('speech.espeakLangs') }},
                  {{ (espeakDataSize / 1024 / 1024).toFixed(1) }}MB)</span
                >
              </div>
              <div v-else class="text-xs text-red-500 dark:text-red-400">
                <span>✗ {{ t('speech.espeakDepLib') }}</span>
                <p class="text-red-400 mt-1">{{ t('speech.espeakNeedRebuild') }}</p>
              </div>
            </div>
          </div>
          <!-- Kokoro TTS -->
          <div
            v-if="isProviderAvailable('kokoro')"
            class="p-3 border rounded-lg transition-colors"
            :class="
              selectedProvider === 'kokoro'
                ? 'border-gray-900 dark:border-white bg-gray-100 dark:bg-gray-700/30'
                : 'border-gray-200 dark:border-gray-700'
            "
          >
            <label class="flex items-center cursor-pointer">
              <input
                v-model="selectedProvider"
                type="radio"
                value="kokoro"
                class="sr-only"
                @change="saveProvider"
                :disabled="!kokoroReady"
              />
              <div class="flex-1">
                <div class="flex items-center gap-2">
                  <span class="text-sm font-medium text-gray-900 dark:text-white">Kokoro</span>
                  <a
                    :href="t('speech.kokoroGithub')"
                    target="_blank"
                    rel="noopener"
                    class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
                    @click.stop
                  >
                    <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="currentColor">
                      <path
                        d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z"
                      />
                    </svg>
                  </a>
                  <span
                    class="text-xs px-1.5 py-0.5 rounded bg-purple-100 dark:bg-purple-900/30 text-purple-700 dark:text-purple-400"
                    >{{ t('speech.kokoroQuality') }}</span
                  >
                </div>
                <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('speech.kokoroDesc') }}</p>
              </div>
              <span v-if="selectedProvider === 'kokoro'" class="text-gray-900 dark:text-white">
                <svg class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
                  <path
                    fill-rule="evenodd"
                    d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z"
                    clip-rule="evenodd"
                  />
                </svg>
              </span>
            </label>
            <!-- Download / Status -->
            <div class="mt-2">
              <div
                v-if="kokoroReady"
                class="flex items-center gap-1 text-xs text-green-600 dark:text-green-400"
              >
                <svg class="w-3.5 h-3.5" fill="currentColor" viewBox="0 0 20 20">
                  <path
                    fill-rule="evenodd"
                    d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z"
                    clip-rule="evenodd"
                  />
                </svg>
                {{ t('speech.kokoroReady') }}
              </div>
              <div v-else-if="kokoroDownloading" class="space-y-1">
                <div class="flex items-center gap-2">
                  <div class="flex-1 bg-gray-300 dark:bg-gray-600 rounded-full h-1.5">
                    <div
                      class="bg-purple-500 h-1.5 rounded-full transition-all duration-300"
                      :style="{
                        width: kokoroDownloadProgress > 0 ? `${kokoroDownloadProgress}%` : '100%',
                        animation:
                          kokoroDownloadProgress <= 0 ? 'pulse 2s ease-in-out infinite' : 'none',
                        opacity: kokoroDownloadProgress <= 0 ? 0.5 : 1,
                      }"
                    ></div>
                  </div>
                  <span class="text-xs text-gray-500 whitespace-nowrap">{{
                    kokoroDownloadProgress > 0
                      ? Math.floor(kokoroDownloadProgress) + '%'
                      : kokoroDownloadedHuman || '...'
                  }}</span>
                  <button
                    class="text-red-500 hover:text-red-600 text-xs whitespace-nowrap"
                    @click="cancelKokoroDownload"
                  >
                    {{ t('speech.kokoroCancelDownload') }}
                  </button>
                </div>
                <p class="text-xs text-gray-400">
                  <span v-if="kokoroDownloadFile">{{ kokoroDownloadFile }}</span>
                  <span v-if="kokoroDownloadTotalFiles > 1">
                    ({{ kokoroDownloadFileIndex + 1 }}/{{ kokoroDownloadTotalFiles }})</span
                  >
                  <span v-if="kokoroDownloadSpeed"> · {{ kokoroDownloadSpeed }}</span>
                  <span v-if="kokoroDownloadETA"> · {{ kokoroDownloadETA }}</span>
                  <span v-if="!kokoroDownloadFile">{{ t('speech.kokoroDownloading') }}</span>
                </p>
              </div>
              <button
                v-else
                class="px-3 py-1.5 bg-purple-600 text-white rounded-lg hover:bg-purple-700 text-xs font-medium"
                @click="downloadKokoro"
              >
                {{ t('speech.kokoroDownload') }}
              </button>
            </div>
            <!-- Supported Languages -->
            <div class="mt-2">
              <p class="text-xs font-medium text-gray-600 dark:text-gray-300 mb-1">
                {{ t('speech.kokoroLanguages') }}
              </p>
              <!-- Current language supported -->
              <div
                v-if="kokoroCurrentLangSupported"
                class="flex items-center gap-1 text-xs text-green-600 dark:text-green-400 mb-1"
              >
                <svg class="w-3.5 h-3.5" fill="currentColor" viewBox="0 0 20 20">
                  <path
                    fill-rule="evenodd"
                    d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z"
                    clip-rule="evenodd"
                  />
                </svg>
                {{ t('speech.kokoroSupportsYourLang', { lang: t(`speech.langName.${locale}`) }) }}
              </div>
              <!-- Collapsed: show "other N languages" button -->
              <button
                v-if="!kokoroLangsExpanded"
                class="text-xs text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 underline"
                @click="kokoroLangsExpanded = true"
              >
                {{ t('speech.kokoroOtherLangs', { count: kokoroOtherLangs.length }) }}
              </button>
              <!-- Expanded: show all other languages -->
              <div v-else class="flex flex-wrap gap-1">
                <span
                  v-for="lang in kokoroOtherLangs"
                  :key="lang"
                  class="text-xs px-1.5 py-0.5 rounded bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300"
                >
                  {{ t(`speech.langName.${lang}`) }}
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Auto-play TTS -->
      <div class="bg-white dark:bg-gray-700/30 rounded-lg p-4 shadow-sm">
        <div class="flex items-center justify-between">
          <div>
            <label class="text-sm font-medium text-gray-900 dark:text-white">{{
              t('speech.autoPlayTTS')
            }}</label>
            <p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
              {{ t('speech.autoPlayTTSDesc') }}
            </p>
          </div>
          <label class="relative inline-flex items-center cursor-pointer">
            <input
              v-model="autoPlayTTS"
              type="checkbox"
              class="sr-only peer"
              @change="saveAutoPlayTTS"
            />
            <div
              class="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-2 peer-focus:ring-gray-900 dark:focus:ring-gray-400 dark:peer-focus:ring-gray-900 dark:focus:ring-gray-400 rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full rtl:peer-checked:after:-translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:start-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-green-600 dark:peer-checked:bg-green-500"
            ></div>
          </label>
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
              <span class="text-sm font-medium text-gray-900 dark:text-white"
                >{{ speechRate.toFixed(1) }}x</span
              >
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
              <label class="text-sm text-gray-700 dark:text-gray-300">{{
                t('speech.pitch')
              }}</label>
              <span class="text-sm font-medium text-gray-900 dark:text-white">{{
                speechPitch
              }}</span>
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
              <label class="text-sm text-gray-700 dark:text-gray-300">{{
                t('speech.volume')
              }}</label>
              <span class="text-sm font-medium text-gray-900 dark:text-white"
                >{{ speechVolume }}%</span
              >
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
    </div>
  </div>
</template>

<style scoped>
.speech-settings-inline-end-gap {
  margin-inline-end: 0.75rem;
}

.speech-settings-inline-start-gap {
  margin-inline-start: 0.5rem;
}
</style>
