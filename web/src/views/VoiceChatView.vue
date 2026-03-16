<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  VoiceWebSocket,
  AudioRecorder,
  blobToBase64,
  playAudioFromBase64,
  voiceApi,
} from '@/api/voice'
import type { VoiceSessionState, Voice } from '@/api/voice'
import { speechApi } from '@/api/speech'
import { WakeWordDetector } from '@/utils/wakeword'
import { convertToWav } from '@/utils/audioConverter'
import ModelDownloadPrompt from '@/components/speech/ModelDownloadPrompt.vue'
import {
  isTtsAutoPlayEnabled,
  isTtsSpeechMuted,
  setTtsAutoPlayEnabled,
} from '@/utils/ttsPreferences'
import { useChatStore } from '@/stores/chat'

const { t } = useI18n()

// State
const isConnected = ref(false)
const isRecording = ref(false)
const sessionState = ref<VoiceSessionState>('idle')
const transcript = ref('')
const response = ref('')
const error = ref<string | null>(null)
const messages = ref<{ role: 'user' | 'assistant'; text: string }[]>([])
const voices = ref<Voice[]>([])
const selectedVoice = ref('alloy')
const selectedLanguage = ref('en')
const autoPlayResponse = ref(isTtsAutoPlayEnabled())
const continuousListening = ref(false)
const isPlaying = ref(false)
const isDesktopRuntime = computed(
  () => typeof window !== 'undefined' && !!(window as any).__BLUE_DESKTOP__
)

// Wake word state
const wakeWordEnabled = ref(false)
const wakeWord = ref('Hey Blue')
const wakeWordListening = ref(false)
const wakeWordSupported = ref(WakeWordDetector.isSupported())
const browserWakeWordAvailable = computed(() => wakeWordSupported.value && !isDesktopRuntime.value)

// ASR model download prompt
const showASRDownloadPrompt = ref(false)
const chatStore = useChatStore()

// WebSocket, recorder, and wake word detector
let ws: VoiceWebSocket | null = null
let recorder: AudioRecorder | null = null
let wakeWordDetector: WakeWordDetector | null = null
let pendingQuestionTimer: ReturnType<typeof setInterval> | null = null

// Computed
const stateText = computed(() => {
  switch (sessionState.value) {
    case 'idle':
      return t('voiceView.state.ready')
    case 'listening':
      return t('voiceView.state.listening')
    case 'processing':
      return t('voiceView.state.processing')
    case 'speaking':
      return t('voiceView.state.speaking')
    default:
      return t('voiceView.state.unknown')
  }
})

const canRecord = computed(() => {
  return isConnected.value && !isRecording.value && sessionState.value === 'idle'
})

function shouldRequestAutoPlayResponse(): boolean {
  return autoPlayResponse.value && !isTtsSpeechMuted()
}

const pendingCheckpointQuestion = computed(() => {
  const q = chatStore.pendingQuestion
  if (!q?.context || q.context.kind !== 'browser_checkpoint') return null
  return q
})

// Lifecycle
onMounted(async () => {
  await loadVoices()
  await checkAndConnect()
  await chatStore.checkPendingQuestion()
  pendingQuestionTimer = setInterval(() => {
    chatStore.checkPendingQuestion()
  }, 5000)
  initWakeWordDetector()
})

// Check ASR model and connect
async function checkAndConnect() {
  try {
    const res = await speechApi.getStatus()
    if (!res.data?.asr?.ready) {
      showASRDownloadPrompt.value = true
      return
    }
  } catch {
    // Assume ready if check fails
  }
  await connectWebSocket()
}

onUnmounted(() => {
  disconnect()
  stopWakeWordDetection()
  if (pendingQuestionTimer) {
    clearInterval(pendingQuestionTimer)
    pendingQuestionTimer = null
  }
})

// Methods
async function loadVoices() {
  try {
    const res = await voiceApi.listVoices()
    voices.value = res.data
  } catch (e) {
    console.error('Failed to load voices:', e)
  }
}

async function connectWebSocket() {
  ws = new VoiceWebSocket()

  ws.onConnect = () => {
    isConnected.value = true
    error.value = null
  }

  ws.onDisconnect = () => {
    isConnected.value = false
  }

  ws.onStateChange = (state) => {
    sessionState.value = state
  }

  ws.onTranscript = (text) => {
    transcript.value = text
    messages.value.push({ role: 'user', text })
    void trySubmitCheckpointByVoice(text)
  }

  ws.onResponse = (text) => {
    response.value = text
    messages.value.push({ role: 'assistant', text })
  }

  ws.onAudioResponse = async (audio, contentType) => {
    if (shouldRequestAutoPlayResponse()) {
      isPlaying.value = true
      try {
        await playAudioFromBase64(audio, contentType)
      } catch (e) {
        console.error('Failed to play audio:', e)
      } finally {
        isPlaying.value = false
        // Continue listening if enabled
        if (continuousListening.value && canRecord.value) {
          startRecording()
        }
      }
    }
  }

  ws.onError = (err) => {
    error.value = err
  }

  try {
    await ws.connect()
    // Send initial config
    ws.updateConfig({
      language: selectedLanguage.value,
      voice: selectedVoice.value,
      auto_play_response: shouldRequestAutoPlayResponse(),
      continuous_listening: continuousListening.value,
    })
  } catch (e) {
    error.value = e instanceof Error ? e.message : t('voiceView.errors.connectionFailed')
  }
}

function disconnect() {
  if (recorder?.isRecording) {
    recorder.stop()
  }
  ws?.disconnect()
  isConnected.value = false
}

async function startRecording() {
  if (!canRecord.value) return

  recorder = new AudioRecorder()

  recorder.onStop = async (audio) => {
    isRecording.value = false
    try {
      // Convert webm to wav for whisper.cpp
      const wavBlob = await convertToWav(audio)
      if (!wavBlob) return
      const base64 = await blobToBase64(wavBlob)
      // Sync latest autoplay state (including mute gate) before each turn.
      updateConfig()
      ws?.sendAudio(base64, 'wav')
    } catch (e) {
      error.value = e instanceof Error ? e.message : t('voiceView.errors.processAudioFailed')
    }
  }

  recorder.onError = (err) => {
    isRecording.value = false
    error.value = err.message
  }

  try {
    await recorder.start()
    isRecording.value = true
    transcript.value = ''
    response.value = ''
  } catch (e) {
    error.value = e instanceof Error ? e.message : t('voiceView.errors.startRecordingFailed')
  }
}

function stopRecording() {
  if (recorder?.isRecording) {
    recorder.stop()
  }
}

function updateAutoPlayResponse() {
  setTtsAutoPlayEnabled(autoPlayResponse.value)
  updateConfig()
}

function updateConfig() {
  ws?.updateConfig({
    language: selectedLanguage.value,
    voice: selectedVoice.value,
    auto_play_response: shouldRequestAutoPlayResponse(),
    continuous_listening: continuousListening.value,
  })
}

function clearMessages() {
  messages.value = []
  transcript.value = ''
  response.value = ''
}

// Wake word detection
function initWakeWordDetector() {
  if (!browserWakeWordAvailable.value) return

  wakeWordDetector = new WakeWordDetector({
    wakeWord: wakeWord.value,
    sensitivity: 0.7,
  })

  wakeWordDetector.setOnWakeWord(() => {
    if (canRecord.value && !isRecording.value) {
      // Wake word detected, start recording
      startRecording()
      // Auto-stop after 5 seconds
      setTimeout(() => {
        if (isRecording.value) {
          stopRecording()
        }
      }, 5000)
    }
  })

  wakeWordDetector.setOnError((err) => {
    console.error('Wake word error:', err)
  })
}

function toggleWakeWordDetection() {
  if (!wakeWordDetector) return

  if (wakeWordEnabled.value) {
    const started = wakeWordDetector.start()
    wakeWordListening.value = started
    if (!started) {
      wakeWordEnabled.value = false
      error.value = t('voiceView.errors.wakeWordStartFailed')
    }
  } else {
    wakeWordDetector.stop()
    wakeWordListening.value = false
  }
}

function stopWakeWordDetection() {
  if (wakeWordDetector) {
    wakeWordDetector.stop()
    wakeWordListening.value = false
  }
}

function updateWakeWord() {
  if (wakeWordDetector) {
    wakeWordDetector.setWakeWord(wakeWord.value)
  }
}

function parseCheckpointDecision(text: string): 'continue' | 'cancel' | '' {
  const normalized = text
    .trim()
    .toLowerCase()
    .replace(
      /^[\s.,!?;:，。！？；：、'"`“”‘’()（）【】\[\]-]+|[\s.,!?;:，。！？；：、'"`“”‘’()（）【】\[\]-]+$/g,
      ''
    )

  if (!normalized) return ''
  if (
    [
      '1',
      'y',
      'yes',
      'ok',
      'okay',
      'continue',
      'proceed',
      'confirm',
      '好',
      '好的',
      '行',
      '可以',
      '继续',
      '继续吧',
      '确认',
      '繼續',
      '確認',
      'oui',
      'continuer',
      'confirmer',
      'ja',
      'weiter',
      'bestätigen',
      'bestaetigen',
      'sí',
      'si',
      'continuar',
      'confirmar',
      'continúa',
      'continua',
      'sì',
      'continua',
      'conferma',
      'sim',
      'да',
      'продолжить',
      'подтвердить',
      'はい',
      '続行',
      '確認する',
      '네',
      '예',
      '계속',
      '확인',
      'ano',
      'pokračovat',
      'pokracovat',
      'potvrdit',
      'tak',
      'kontynuuj',
      'potwierdz',
      'نعم',
      'ναι',
      'συνέχεια',
      'συνεχίστε',
      'συνεχισε',
      'igen',
      'folytatás',
      'folytatas',
      'megerősít',
      'megerosit',
      'da',
      'nastavi',
      'potvrdi',
      'confirmă',
      'confirma',
      'fortsett',
      'bekreft',
      'fortsätt',
      'fortsaett',
      'bekräfta',
      'bekrafta',
      'lean ar aghaidh',
      'deimhnigh',
      'അതെ',
      'തുടരുക',
      'സ്ഥിരീകരിക്കുക',
    ].includes(normalized)
  ) {
    return 'continue'
  }
  if (
    [
      '2',
      'n',
      'no',
      'cancel',
      'stop',
      'deny',
      'reject',
      'abort',
      '取消',
      '拒绝',
      '不要',
      '停止',
      '中止',
      '取消吧',
      '拒絕',
      'non',
      'annuler',
      'arrêter',
      'arreter',
      'refuser',
      'nein',
      'abbrechen',
      'stopp',
      'ablehnen',
      'cancelar',
      'detener',
      'rechazar',
      'annulla',
      'ferma',
      'rifiuta',
      'não',
      'nao',
      'parar',
      'recusar',
      'нет',
      'отмена',
      'стоп',
      'отклонить',
      'いいえ',
      'キャンセル',
      '停止',
      '拒否',
      '아니요',
      '아니오',
      '취소',
      '중지',
      '거부',
      'ne',
      'zrušit',
      'zrusit',
      'zamítnout',
      'zamitnout',
      'nie',
      'anuluj',
      'odrzuć',
      'odrzuc',
      'όχι',
      'ακύρωση',
      'ακυρωση',
      'σταμάτα',
      'σταματα',
      'nem',
      'mégse',
      'megse',
      'elutasít',
      'elutasit',
      'otkaži',
      'otkazi',
      'odbij',
      'nu',
      'anulează',
      'anuleaza',
      'respinge',
      'stans',
      'afbryd',
      'nei',
      'avbryt',
      'nej',
      'avbryt',
      'avbryt',
      'ná',
      'na',
      'cealaigh',
      'diúltaigh',
      'diultaigh',
      'ഇല്ല',
      'റദ്ദാക്കുക',
      'നിർത്തുക',
    ].includes(normalized)
  ) {
    return 'cancel'
  }
  return ''
}

async function submitCheckpointDecision(decision: 'continue' | 'cancel') {
  const pending = pendingCheckpointQuestion.value
  if (!pending || pending.questions.length === 0) return
  const first = pending.questions[0]
  if (!first) return
  await chatStore.submitQuestionAnswers([
    {
      question_id: first.id,
      selected: [decision],
    },
  ])
}

async function trySubmitCheckpointByVoice(text: string) {
  if (!pendingCheckpointQuestion.value) return
  const decision = parseCheckpointDecision(text)
  if (!decision) return
  await submitCheckpointDecision(decision)
}

// Watch for wake word toggle
watch(wakeWordEnabled, () => {
  toggleWakeWordDetection()
})
</script>

<template>
  <div class="voice-chat-view p-6 max-w-4xl mx-auto">
    <div class="flex items-center justify-between mb-6">
      <h1 class="text-2xl font-bold text-white">{{ t('voiceView.title') }}</h1>
      <div class="flex items-center gap-2">
        <span
          class="w-3 h-3 rounded-full"
          :class="isConnected ? 'bg-green-500' : 'bg-red-500'"
        ></span>
        <span class="text-sm text-gray-400">
          {{ isConnected ? t('voiceView.connected') : t('voiceView.disconnected') }}
        </span>
      </div>
    </div>

    <!-- Error Message -->
    <div
      v-if="error"
      class="mb-4 bg-red-900/50 border border-red-500 text-red-200 px-4 py-3 rounded-lg flex items-center justify-between"
    >
      <span>{{ error }}</span>
      <button class="text-red-300 hover:text-red-100" @click="error = null">
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="h-5 w-5"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M6 18L18 6M6 6l12 12"
          />
        </svg>
      </button>
    </div>

    <!-- Settings -->
    <div class="bg-gray-700 rounded-lg p-4 mb-6">
      <h2 class="text-sm font-medium text-gray-400 mb-3">{{ t('voiceView.settingsTitle') }}</h2>
      <div class="grid grid-cols-2 md:grid-cols-4 gap-4 mb-4">
        <!-- Language -->
        <div>
          <label class="block text-xs text-gray-500 mb-1">{{ t('voiceView.languageLabel') }}</label>
          <select
            v-model="selectedLanguage"
            class="w-full bg-gray-700 text-white text-sm rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400"
            @change="updateConfig"
          >
            <option value="en">{{ t('voiceView.languages.en') }}</option>
            <option value="zh">{{ t('voiceView.languages.zh') }}</option>
            <option value="ja">{{ t('voiceView.languages.ja') }}</option>
            <option value="ko">{{ t('voiceView.languages.ko') }}</option>
            <option value="de">{{ t('voiceView.languages.de') }}</option>
            <option value="fr">{{ t('voiceView.languages.fr') }}</option>
            <option value="es">{{ t('voiceView.languages.es') }}</option>
          </select>
        </div>

        <!-- Voice -->
        <div>
          <label class="block text-xs text-gray-500 mb-1">{{ t('voiceView.voiceLabel') }}</label>
          <select
            v-model="selectedVoice"
            class="w-full bg-gray-700 text-white text-sm rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400"
            @change="updateConfig"
          >
            <option v-for="voice in voices" :key="voice.id" :value="voice.id">
              {{ voice.name }}
            </option>
          </select>
        </div>

        <!-- Auto Play -->
        <div class="flex items-center">
          <label class="flex items-center gap-2 cursor-pointer">
            <input
              v-model="autoPlayResponse"
              type="checkbox"
              class="w-4 h-4 rounded border-gray-600 bg-gray-700 text-gray-900 dark:text-white focus:ring-gray-900 dark:focus:ring-gray-400"
              @change="updateAutoPlayResponse"
            />
            <span class="text-sm text-gray-300">{{ t('voiceView.autoPlay') }}</span>
          </label>
        </div>

        <!-- Continuous -->
        <div class="flex items-center">
          <label class="flex items-center gap-2 cursor-pointer">
            <input
              v-model="continuousListening"
              type="checkbox"
              class="w-4 h-4 rounded border-gray-600 bg-gray-700 text-gray-900 dark:text-white focus:ring-gray-900 dark:focus:ring-gray-400"
              @change="updateConfig"
            />
            <span class="text-sm text-gray-300">{{ t('voiceView.continuous') }}</span>
          </label>
        </div>
      </div>

      <!-- Wake Word Settings -->
      <div v-if="browserWakeWordAvailable" class="border-t border-gray-700 pt-4 mt-4">
        <div class="flex items-center justify-between mb-3">
          <h3 class="text-sm font-medium text-gray-400">{{ t('voiceView.wakeWordTitle') }}</h3>
          <label class="flex items-center gap-2 cursor-pointer">
            <input
              v-model="wakeWordEnabled"
              type="checkbox"
              class="w-4 h-4 rounded border-gray-600 bg-gray-700 text-gray-900 dark:text-white focus:ring-gray-900 dark:focus:ring-gray-400"
            />
            <span class="text-sm text-gray-300">{{ t('voiceView.wakeWordEnable') }}</span>
          </label>
        </div>
        <div class="flex items-center gap-4">
          <div class="flex-1">
            <label class="block text-xs text-gray-500 mb-1">{{
              t('voiceView.wakeWordLabel')
            }}</label>
            <input
              v-model="wakeWord"
              type="text"
              class="w-full bg-gray-700 text-white text-sm rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400"
              :placeholder="t('voiceView.wakeWordPlaceholder')"
              @change="updateWakeWord"
            />
          </div>
          <div class="flex items-center gap-2 pt-5">
            <span
              class="w-2 h-2 rounded-full"
              :class="wakeWordListening ? 'bg-green-500 animate-pulse' : 'bg-gray-500'"
            ></span>
            <span class="text-xs text-gray-500">
              {{
                wakeWordListening ? t('voiceView.wakeWordListening') : t('voiceView.wakeWordIdle')
              }}
            </span>
          </div>
        </div>
      </div>
      <div
        v-else-if="isDesktopRuntime"
        data-testid="voicewake-desktop-note"
        class="border-t border-gray-700 pt-4 mt-4"
      >
        <div class="rounded-lg border border-blue-500/30 bg-blue-500/10 px-4 py-3 text-sm">
          <p class="font-medium text-blue-200">{{ t('speech.voiceWake.title') }}</p>
          <p class="mt-1 text-blue-100/80">
            {{ t('speech.voiceWake.desktopNoteDescription') }}
          </p>
        </div>
      </div>
    </div>

    <!-- Browser checkpoint confirmation (voice + button fallback) -->
    <div
      v-if="pendingCheckpointQuestion"
      class="bg-blue-900/30 border border-blue-500/40 rounded-lg p-4 mb-6"
    >
      <div class="text-sm font-medium text-blue-200 mb-1">
        {{ t('voiceView.checkpoint.title') }}
      </div>
      <div class="text-xs text-blue-100/90 mb-2">
        {{
          pendingCheckpointQuestion.questions[0]?.question ||
          t('voiceView.checkpoint.fallbackQuestion')
        }}
      </div>
      <div class="text-xs text-blue-200/80 mb-3">{{ t('voiceView.checkpoint.help') }}</div>
      <div class="flex gap-2">
        <button
          class="px-3 py-2 text-xs font-medium rounded-md bg-blue-600 hover:bg-blue-700 text-white transition-colors"
          @click="submitCheckpointDecision('continue')"
        >
          {{ t('voiceView.checkpoint.continue') }}
        </button>
        <button
          class="px-3 py-2 text-xs font-medium rounded-md bg-gray-600 hover:bg-gray-500 text-white transition-colors"
          @click="submitCheckpointDecision('cancel')"
        >
          {{ t('voiceView.checkpoint.cancel') }}
        </button>
      </div>
    </div>

    <!-- Messages -->
    <div class="bg-gray-700 rounded-lg mb-6 min-h-[300px] max-h-[400px] overflow-y-auto">
      <div v-if="messages.length === 0" class="p-8 text-center text-gray-500">
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="h-12 w-12 mx-auto mb-4 opacity-50"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M19 11a7 7 0 01-7 7m0 0a7 7 0 01-7-7m7 7v4m0 0H8m4 0h4m-4-8a3 3 0 01-3-3V5a3 3 0 116 0v6a3 3 0 01-3 3z"
          />
        </svg>
        <p>{{ t('voiceView.emptyHint') }}</p>
      </div>
      <div v-else class="p-4 space-y-4">
        <div
          v-for="(msg, index) in messages"
          :key="index"
          class="flex"
          :class="msg.role === 'user' ? 'justify-end' : 'justify-start'"
        >
          <div
            class="max-w-[80%] rounded-lg px-4 py-2"
            :class="
              msg.role === 'user'
                ? 'bg-gray-700 dark:bg-gray-500 text-white'
                : 'bg-gray-700 text-gray-200'
            "
          >
            {{ msg.text }}
          </div>
        </div>
      </div>
    </div>

    <!-- Status and Controls -->
    <div class="flex flex-col items-center gap-4">
      <!-- Status -->
      <div class="text-center">
        <div
          class="text-lg font-medium"
          :class="{
            'text-gray-400': sessionState === 'idle',
            'text-green-400': sessionState === 'listening',
            'text-yellow-400': sessionState === 'processing',
            'text-gray-900 dark:text-white': sessionState === 'speaking',
          }"
        >
          {{ stateText }}
        </div>
        <div v-if="transcript && sessionState !== 'idle'" class="text-sm text-gray-500 mt-1">
          "{{ transcript }}"
        </div>
      </div>

      <!-- Record Button -->
      <button
        :disabled="!isConnected || sessionState !== 'idle'"
        class="w-20 h-20 rounded-full flex items-center justify-center transition-all duration-200"
        :class="{
          'bg-gray-700 cursor-not-allowed': !isConnected || sessionState !== 'idle',
          'bg-red-600 hover:bg-red-700 scale-110': isRecording,
          'bg-gray-700 dark:bg-gray-500 hover:bg-gray-700 dark:bg-gray-500':
            canRecord && !isRecording,
        }"
        @mousedown="startRecording"
        @mouseup="stopRecording"
        @mouseleave="stopRecording"
        @touchstart.prevent="startRecording"
        @touchend.prevent="stopRecording"
      >
        <svg
          v-if="isRecording"
          xmlns="http://www.w3.org/2000/svg"
          class="h-10 w-10 text-white animate-pulse"
          fill="currentColor"
          viewBox="0 0 24 24"
        >
          <rect x="6" y="6" width="12" height="12" rx="2" />
        </svg>
        <svg
          v-else
          xmlns="http://www.w3.org/2000/svg"
          class="h-10 w-10 text-white"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M19 11a7 7 0 01-7 7m0 0a7 7 0 01-7-7m7 7v4m0 0H8m4 0h4m-4-8a3 3 0 01-3-3V5a3 3 0 116 0v6a3 3 0 01-3 3z"
          />
        </svg>
      </button>

      <p class="text-sm text-gray-500">
        {{ isRecording ? t('voiceView.releaseToSend') : t('voiceView.holdToSpeak') }}
      </p>

      <!-- Clear Button -->
      <button
        v-if="messages.length > 0"
        class="text-sm text-gray-400 hover:text-gray-300"
        @click="clearMessages"
      >
        {{ t('voiceView.clearConversation') }}
      </button>
    </div>

    <!-- ASR Model Download Prompt -->
    <ModelDownloadPrompt
      v-model:model-visible="showASRDownloadPrompt"
      type="asr"
      @downloaded="connectWebSocket"
    />
  </div>
</template>
