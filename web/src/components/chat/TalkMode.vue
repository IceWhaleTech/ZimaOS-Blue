<script setup lang="ts">
import { ref, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { AudioRecorder, VoiceWebSocket, playAudioFromBase64 } from '@/api/voice'
import type { VoiceSessionState } from '@/api/voice'
import { speechApi } from '@/api/speech'
import TranscriptionEditor from './TranscriptionEditor.vue'
import ModelDownloadPrompt from '@/components/speech/ModelDownloadPrompt.vue'
import { useLocaleStore } from '@/stores/locale'
import { useChatStore } from '@/stores/chat'

const { t } = useI18n()
const localeStore = useLocaleStore()
const chatStore = useChatStore()

const props = defineProps<{
  modelValue: boolean
  conversationId?: string
  editBeforeSend?: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  'transcript': [text: string]
  'response': [text: string]
}>()

// Talk mode state
type TalkModeType = 'conversation' | 'walkie-talkie'
const talkMode = ref<TalkModeType>('conversation')
const isConnected = ref(false)
const isListening = ref(false)
const isSpeaking = ref(false)
const isProcessing = ref(false)
const autoPlayTTS = ref(localStorage.getItem('tts-auto-play') !== 'false') // Default to true in talk mode
const error = ref<string | null>(null)
const transcript = ref('')
const response = ref('')

// Edit before send state
const showTranscriptionEditor = ref(false)
const pendingTranscription = ref('')
const transcriptionLanguage = ref('')
const transcriptionConfidence = ref(0)

// ASR model download prompt
const showASRDownloadPrompt = ref(false)

// Audio visualization
const audioLevel = ref(0)
const audioLevelInterval = ref<number | null>(null)

// Audio recording for local ASR
const recordedChunks = ref<Blob[]>([])

// WebSocket and recorder instances
let voiceWs: VoiceWebSocket | null = null
let recorder: AudioRecorder | null = null

// Check if ASR model is ready
async function checkASRModelReady(): Promise<boolean> {
  try {
    const res = await speechApi.getASRStatus()
    return res.data?.ready ?? false
  } catch {
    return true // Assume ready if check fails
  }
}

// Connect to voice WebSocket
async function connect() {
  if (voiceWs?.isConnected) return

  // Check if ASR model is ready first
  const ready = await checkASRModelReady()
  if (!ready) {
    showASRDownloadPrompt.value = true
    return
  }

  error.value = null
  voiceWs = new VoiceWebSocket()

  voiceWs.onConnect = () => {
    isConnected.value = true
    error.value = null
  }

  voiceWs.onDisconnect = () => {
    isConnected.value = false
    isListening.value = false
    isSpeaking.value = false
    isProcessing.value = false
  }

  voiceWs.onStateChange = (state: VoiceSessionState) => {
    isListening.value = state === 'listening'
    isProcessing.value = state === 'processing'
    isSpeaking.value = state === 'speaking'
  }

  voiceWs.onTranscript = (text: string) => {
    transcript.value = text
    emit('transcript', text)
  }

  voiceWs.onResponse = (text: string) => {
    response.value = text
    emit('response', text)
  }

  voiceWs.onAudioResponse = async (audio: string, contentType: string) => {
    isSpeaking.value = true
    try {
      await playAudioFromBase64(audio, contentType)
    } catch (e) {
      console.error('Failed to play audio response:', e)
    } finally {
      isSpeaking.value = false
      // In walkie-talkie mode, don't auto-restart listening
      if (talkMode.value === 'conversation') {
        startListening()
      }
    }
  }

  voiceWs.onError = (err: string) => {
    error.value = err
    isListening.value = false
    isProcessing.value = false
  }

  try {
    // Pass user's locale language to WebSocket (e.g., 'zh-CN' -> 'zh')
    const lang = localeStore.currentLocale.split('-')[0]
    await voiceWs.connect(lang)
    // Configure for continuous listening in conversation mode
    voiceWs.updateConfig({
      continuous_listening: talkMode.value === 'conversation',
      auto_play_response: true,
    })
  } catch (e) {
    error.value = t('chat.talkMode.connectionError')
    console.error('Failed to connect to voice WebSocket:', e)
  }
}

// Disconnect from voice WebSocket
function disconnect() {
  if (voiceWs) {
    voiceWs.disconnect()
    voiceWs = null
  }
  stopListening()
  isConnected.value = false
}

// Start listening (recording)
async function startListening() {
  if (isListening.value || isSpeaking.value) return

  error.value = null
  recordedChunks.value = []
  recorder = new AudioRecorder()

  recorder.onDataAvailable = async (data: Blob) => {
    // Collect all chunks for conversion at the end
    recordedChunks.value.push(data)
  }

  recorder.onError = (err: Error) => {
    error.value = t('chat.voiceRecordingError')
    console.error('Recording error:', err)
    isListening.value = false
  }

  try {
    await recorder.start()
    isListening.value = true
    startAudioLevelMonitor()
  } catch (e) {
    error.value = t('chat.voiceMicrophoneError')
    console.error('Failed to start recording:', e)
  }
}

// Stop listening
async function stopListening() {
  if (recorder) {
    recorder.stop()
    recorder = null
  }
  isListening.value = false
  stopAudioLevelMonitor()

  // Transcribe recorded audio if we have any
  if (recordedChunks.value.length > 0) {
    await transcribeLocally()
  }
  recordedChunks.value = []
}

// Transcribe audio locally using Whisper ASR
async function transcribeLocally() {
  if (recordedChunks.value.length === 0) return

  isProcessing.value = true
  error.value = null

  try {
    // Get the actual mime type from recorder or default to webm
    const mimeType = recorder?.mimeType || 'audio/webm'
    const audioBlob = new Blob(recordedChunks.value, { type: mimeType })

    // Determine format from mime type (e.g., 'audio/webm' -> 'webm')
    const format = mimeType.split('/')[1]?.split(';')[0] || 'webm'

    // Use user's locale language for transcription (e.g., 'zh-CN' -> 'zh')
    const lang = localeStore.currentLocale.split('-')[0]

    // Send directly to backend - backend will handle format conversion
    const result = await speechApi.transcribe(audioBlob, format, lang)

    if (result.text) {
      // If editBeforeSend is enabled, show editor; otherwise emit directly
      if (props.editBeforeSend) {
        pendingTranscription.value = result.text
        transcriptionLanguage.value = result.language || ''
        transcriptionConfidence.value = result.confidence || 0
        showTranscriptionEditor.value = true
      } else {
        // Emit transcript directly
        transcript.value = result.text
        emit('transcript', result.text)
      }
    }
  } catch (e) {
    console.error('Local transcription failed:', e)
    error.value = t('chat.talkMode.transcriptionError')
  } finally {
    isProcessing.value = false
  }
}

// Handle transcription confirmation
function handleTranscriptionConfirm(text: string) {
  transcript.value = text
  emit('transcript', text)
  showTranscriptionEditor.value = false
  pendingTranscription.value = ''
}

// Handle transcription cancel
function handleTranscriptionCancel() {
  showTranscriptionEditor.value = false
  pendingTranscription.value = ''
}

// Toggle listening (for walkie-talkie mode)
function toggleListening() {
  if (isListening.value) {
    stopListening()
  } else {
    startListening()
  }
}

// Audio level monitoring for visualization
function startAudioLevelMonitor() {
  audioLevelInterval.value = window.setInterval(() => {
    // Simulate audio level (in real implementation, use Web Audio API)
    audioLevel.value = Math.random() * 100
  }, 100)
}

function stopAudioLevelMonitor() {
  if (audioLevelInterval.value) {
    clearInterval(audioLevelInterval.value)
    audioLevelInterval.value = null
  }
  audioLevel.value = 0
}

// Switch talk mode
function switchMode(mode: TalkModeType) {
  talkMode.value = mode
  if (voiceWs?.isConnected) {
    voiceWs.updateConfig({
      continuous_listening: mode === 'conversation',
    })
  }
  // Reset state when switching modes
  stopListening()
  transcript.value = ''
  response.value = ''
}

// Close talk mode
function close() {
  disconnect()
  emit('update:modelValue', false)
}

// Toggle mute
function toggleAutoPlay() {
  autoPlayTTS.value = !autoPlayTTS.value
  localStorage.setItem('tts-auto-play', autoPlayTTS.value.toString())
}

// Watch for modelValue changes
watch(() => props.modelValue, (newValue) => {
  if (newValue) {
    connect()
  } else {
    disconnect()
  }
})

// Watch for AI response completion and play TTS
watch(() => chatStore.streaming, async (streaming, wasStreaming) => {
  // When streaming ends, get the last assistant message and play TTS
  if (wasStreaming && !streaming && props.modelValue) {
    const messages = chatStore.messages
    if (messages.length > 0) {
      const lastMsg = messages[messages.length - 1]
      if (lastMsg.role === 'assistant' && lastMsg.content) {
        response.value = lastMsg.content
        // Play TTS for the response
        await playResponseTTS(lastMsg.content)
      }
    }
  }
})

// Play TTS for AI response
async function playResponseTTS(text: string) {
  if (!text.trim() || !autoPlayTTS.value) return

  isSpeaking.value = true
  try {
    const result = await speechApi.synthesize(text)
    if (result.audio) {
      await playAudioFromBase64(result.audio, result.content_type || 'audio/mp3')
    }
  } catch (e) {
    console.error('TTS playback failed:', e)
  } finally {
    isSpeaking.value = false
  }
}

// Cleanup on unmount
onUnmounted(() => {
  disconnect()
})
</script>

<template>
  <Teleport to="body">
    <Transition name="fade">
      <div
        v-if="modelValue"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-sm"
        @click.self="close"
      >
        <div class="talk-mode-container glass-card w-full max-w-md mx-4 p-6 rounded-2xl">
          <!-- Header -->
          <div class="flex items-center justify-between mb-6">
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('chat.talkMode.title') }}
            </h3>
            <div class="flex items-center gap-2">
              <!-- Auto-play TTS toggle button -->
              <button
                class="p-2 rounded-lg transition-colors cursor-pointer"
                :class="!autoPlayTTS
                  ? 'bg-red-100 dark:bg-red-900/30 text-red-500 hover:bg-red-200 dark:hover:bg-red-900/50'
                  : 'hover:bg-gray-200 dark:hover:bg-white/10 text-gray-500 dark:text-gray-400'"
                :title="autoPlayTTS ? t('chat.talkMode.mute') : t('chat.talkMode.unmute')"
                @click="toggleAutoPlay"
              >
                <!-- Speaker icon (auto-play on) -->
                <svg v-if="autoPlayTTS" class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.536 8.464a5 5 0 010 7.072m2.828-9.9a9 9 0 010 12.728M5.586 15H4a1 1 0 01-1-1v-4a1 1 0 011-1h1.586l4.707-4.707C10.923 3.663 12 4.109 12 5v14c0 .891-1.077 1.337-1.707.707L5.586 15z" />
                </svg>
                <!-- Muted icon (auto-play off) -->
                <svg v-else class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5.586 15H4a1 1 0 01-1-1v-4a1 1 0 011-1h1.586l4.707-4.707C10.923 3.663 12 4.109 12 5v14c0 .891-1.077 1.337-1.707.707L5.586 15z" />
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2" />
                </svg>
              </button>
              <!-- Close button -->
              <button
                class="p-2 rounded-lg hover:bg-gray-200 dark:hover:bg-white/10 transition-colors cursor-pointer"
                @click="close"
              >
                <svg class="w-5 h-5 text-gray-500 dark:text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>
          </div>

          <!-- Mode selector -->
          <div class="flex gap-2 mb-6">
            <button
              class="flex-1 py-2 px-4 rounded-lg text-sm font-medium transition-colors cursor-pointer"
              :class="talkMode === 'conversation'
                ? 'bg-gray-700 dark:bg-gray-500 text-white'
                : 'bg-gray-100 dark:bg-white/10 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-white/20'"
              @click="switchMode('conversation')"
            >
              <div class="flex items-center justify-center gap-2">
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
                </svg>
                {{ t('chat.talkMode.conversation') }}
              </div>
            </button>
            <button
              class="flex-1 py-2 px-4 rounded-lg text-sm font-medium transition-colors cursor-pointer"
              :class="talkMode === 'walkie-talkie'
                ? 'bg-gray-700 dark:bg-gray-500 text-white'
                : 'bg-gray-100 dark:bg-white/10 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-white/20'"
              @click="switchMode('walkie-talkie')"
            >
              <div class="flex items-center justify-center gap-2">
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11a7 7 0 01-7 7m0 0a7 7 0 01-7-7m7 7v4m0 0H8m4 0h4m-4-8a3 3 0 01-3-3V5a3 3 0 116 0v6a3 3 0 01-3 3z" />
                </svg>
                {{ t('chat.talkMode.walkieTalkie') }}
              </div>
            </button>
          </div>

          <!-- Status indicator -->
          <div class="flex flex-col items-center mb-6">
            <!-- Main action button -->
            <button
              class="relative w-24 h-24 rounded-full flex items-center justify-center transition-all duration-300 cursor-pointer"
              :class="{
                'bg-red-500 hover:bg-red-600 animate-pulse': isListening,
                'bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400': !isListening && isConnected,
                'bg-gray-400': !isConnected,
                'bg-gray-600 dark:bg-gray-500': isSpeaking,
                'bg-yellow-500': isProcessing,
              }"
              :disabled="!isConnected || isSpeaking || isProcessing"
              @click="toggleListening"
              @mousedown="talkMode === 'walkie-talkie' && startListening()"
              @mouseup="talkMode === 'walkie-talkie' && stopListening()"
              @mouseleave="talkMode === 'walkie-talkie' && isListening && stopListening()"
            >
              <!-- Audio level visualization -->
              <div
                v-if="isListening"
                class="absolute inset-0 rounded-full border-4 border-white/30 animate-ping"
              />
              <div
                v-if="isListening"
                class="absolute inset-0 rounded-full"
                :style="{ transform: `scale(${1 + audioLevel / 200})`, opacity: 0.3 }"
              />

              <!-- Icon -->
              <svg
                v-if="isListening"
                class="w-10 h-10 text-white"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11a7 7 0 01-7 7m0 0a7 7 0 01-7-7m7 7v4m0 0H8m4 0h4m-4-8a3 3 0 01-3-3V5a3 3 0 116 0v6a3 3 0 01-3 3z" />
              </svg>
              <svg
                v-else-if="isSpeaking"
                class="w-10 h-10 text-white"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.536 8.464a5 5 0 010 7.072m2.828-9.9a9 9 0 010 12.728M5.586 15H4a1 1 0 01-1-1v-4a1 1 0 011-1h1.586l4.707-4.707C10.923 3.663 12 4.109 12 5v14c0 .891-1.077 1.337-1.707.707L5.586 15z" />
              </svg>
              <svg
                v-else-if="isProcessing"
                class="w-10 h-10 text-white animate-spin"
                fill="none"
                viewBox="0 0 24 24"
              >
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
              </svg>
              <svg
                v-else
                class="w-10 h-10 text-white"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11a7 7 0 01-7 7m0 0a7 7 0 01-7-7m7 7v4m0 0H8m4 0h4m-4-8a3 3 0 01-3-3V5a3 3 0 116 0v6a3 3 0 01-3 3z" />
              </svg>
            </button>

            <!-- Status text -->
            <p class="mt-4 text-sm text-gray-600 dark:text-gray-400">
              <template v-if="!isConnected">
                {{ t('chat.talkMode.connecting') }}
              </template>
              <template v-else-if="isListening">
                {{ t('chat.talkMode.listening') }}
              </template>
              <template v-else-if="isProcessing">
                {{ t('chat.talkMode.processing') }}
              </template>
              <template v-else-if="isSpeaking">
                {{ t('chat.talkMode.speaking') }}
              </template>
              <template v-else>
                {{ talkMode === 'walkie-talkie' ? t('chat.talkMode.holdToTalk') : t('chat.talkMode.tapToStart') }}
              </template>
            </p>
          </div>

          <!-- Transcript display -->
          <div v-if="transcript || response" class="space-y-3">
            <div v-if="transcript" class="p-3 rounded-lg bg-gray-100 dark:bg-white/5">
              <p class="text-xs text-gray-500 dark:text-gray-400 mb-1">{{ t('chat.talkMode.you') }}</p>
              <p class="text-sm text-gray-900 dark:text-white">{{ transcript }}</p>
            </div>
            <div v-if="response" class="p-3 rounded-lg bg-gray-100 dark:bg-gray-700/30">
              <p class="text-xs text-gray-500 dark:text-gray-400 mb-1">{{ t('chat.talkMode.assistant') }}</p>
              <p class="text-sm text-gray-900 dark:text-white">{{ response }}</p>
            </div>
          </div>

          <!-- Transcription Editor (for edit-before-send) -->
          <TranscriptionEditor
            v-model:visible="showTranscriptionEditor"
            :text="pendingTranscription"
            :language="transcriptionLanguage"
            :confidence="transcriptionConfidence"
            @confirm="handleTranscriptionConfirm"
            @cancel="handleTranscriptionCancel"
          />

          <!-- ASR Model Download Prompt -->
          <ModelDownloadPrompt
            v-model:model-visible="showASRDownloadPrompt"
            type="asr"
            @downloaded="connect"
          />

          <!-- Error message -->
          <div
            v-if="error"
            class="mt-4 p-3 rounded-lg bg-red-500/10 text-red-400 text-sm flex items-center gap-2"
          >
            <svg class="w-4 h-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            <span>{{ error }}</span>
          </div>

          <!-- Mode description -->
          <p class="mt-4 text-xs text-gray-500 dark:text-gray-400 text-center">
            {{ talkMode === 'conversation' ? t('chat.talkMode.conversationDesc') : t('chat.talkMode.walkieTalkieDesc') }}
          </p>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.talk-mode-container {
  max-height: 90vh;
  overflow-y: auto;
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.3s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
