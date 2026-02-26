<script setup lang="ts">
import { ref, onUnmounted, watch, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { playAudioFromBase64 } from '@/api/voice'
import { speechApi } from '@/api/speech'
import { convertToWav } from '@/utils/audioConverter'
import { EnergyVAD } from '@/utils/vad'
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

// Conversation state machine
type ConversationState = 'idle' | 'listening' | 'transcribing' | 'processing' | 'speaking'
const conversationState = ref<ConversationState>('idle')
const autoPlayTTS = ref(localStorage.getItem('tts-auto-play') !== 'false')
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

// Audio visualization (0-100)
const audioLevel = ref(0)

// Mobile detection
const isMobile = ref(false)

// VAD instance
let vad: EnergyVAD | null = null

// Derived state helpers
const isListening = () => conversationState.value === 'listening'
const isActive = () => conversationState.value !== 'idle'

// Check mobile on mount
onMounted(() => {
  isMobile.value = window.innerWidth < 768
})

// Open talk mode — check permissions, then start VAD loop
async function open() {
  // Secure context check
  if (!window.isSecureContext) {
    error.value = t('chat.voiceSecureContextError')
    return
  }

  // getUserMedia support check
  if (!navigator.mediaDevices?.getUserMedia) {
    error.value = t('chat.voiceNotSupportedError')
    return
  }

  // Pre-check mic permission
  try {
    const stream = await navigator.mediaDevices.getUserMedia({ audio: true })
    stream.getTracks().forEach(t => t.stop())
  } catch (err: any) {
    if (err.name === 'NotAllowedError' || err.name === 'PermissionDeniedError') {
      error.value = t('chat.voiceMicrophonePermissionDenied')
    } else if (err.name === 'NotFoundError' || err.name === 'DevicesNotFoundError') {
      error.value = t('chat.voiceMicrophoneNotFound')
    } else if (err.name === 'NotReadableError' || err.name === 'TrackStartError') {
      error.value = t('chat.voiceMicrophoneInUse')
    } else {
      error.value = t('chat.voiceMicrophoneError')
    }
    return
  }

  // Check ASR model readiness
  try {
    const res = await speechApi.getStatus()
    if (!res.data?.asr?.ready) {
      showASRDownloadPrompt.value = true
      return
    }
  } catch {
    // Assume ready if check fails
  }

  error.value = null
  startListening()
}

// Start VAD listening
async function startListening() {
  if (conversationState.value === 'listening' || conversationState.value === 'speaking') return

  error.value = null

  if (vad) {
    vad.destroy()
    vad = null
  }

  const lang = localeStore.currentLocale.split('-')[0]

  vad = new EnergyVAD({
    speechThreshold: 0.015,
    silenceThreshold: 0.01,
    silenceDuration: 1500,
    minSpeechDuration: 500,
    onSpeechStart: () => {
      // Visual feedback handled by audioLevel reactivity
    },
    onSpeechEnd: async (audioBlob: Blob) => {
      conversationState.value = 'transcribing'

      try {
        const wavBlob = await convertToWav(audioBlob)
        if (!wavBlob) {
          resumeListening()
          return
        }
        const result = await speechApi.transcribe(wavBlob, 'wav', lang)

        if (result.text) {
          if (props.editBeforeSend) {
            pendingTranscription.value = result.text
            transcriptionLanguage.value = result.language || ''
            transcriptionConfidence.value = result.confidence || 0
            showTranscriptionEditor.value = true
          } else {
            transcript.value = result.text
            conversationState.value = 'processing'
            emit('transcript', result.text)
          }
        } else {
          // Empty transcription — resume
          resumeListening()
        }
      } catch (e: any) {
        console.error('Transcription failed:', e)
        const errorCode = e?.error_code || e?.response?.data?.error_code
        if (errorCode === 'timeout' || e?.name === 'AbortError') {
          error.value = t('chat.voiceTranscriptionTimeout')
        } else if (errorCode === 'on_device_unavailable') {
          error.value = t('speech.onDeviceUnavailableError')
        } else {
          const serverMsg = e?.response?.data?.error || e?.response?.data?.message || e?.message
          error.value = serverMsg || t('chat.talkMode.transcriptionError')
        }
        // Resume listening after error
        resumeListening()
      }
    },
    onVolumeChange: (level: number) => {
      audioLevel.value = level * 100
    },
  })

  try {
    await vad.start()
    conversationState.value = 'listening'
  } catch (e) {
    error.value = t('chat.voiceMicrophoneError')
    console.error('Failed to start VAD:', e)
    conversationState.value = 'idle'
  }
}

// Resume listening (after transcription/TTS)
function resumeListening() {
  if (vad && vad.isListening) {
    vad.resume()
    conversationState.value = 'listening'
  } else {
    startListening()
  }
}

// Stop everything
function stopAll() {
  if (vad) {
    vad.destroy()
    vad = null
  }
  conversationState.value = 'idle'
  audioLevel.value = 0
}

// Toggle listening (main button)
function toggleListening() {
  if (isActive()) {
    stopAll()
  } else {
    startListening()
  }
}

// Handle transcription confirmation (from edit-before-send editor)
function handleTranscriptionConfirm(text: string) {
  transcript.value = text
  conversationState.value = 'processing'
  emit('transcript', text)
  showTranscriptionEditor.value = false
  pendingTranscription.value = ''
}

// Handle transcription cancel
function handleTranscriptionCancel() {
  showTranscriptionEditor.value = false
  pendingTranscription.value = ''
  resumeListening()
}

// Close talk mode
function close() {
  stopAll()
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
    open()
  } else {
    stopAll()
  }
})

// Watch for AI response completion and play TTS
watch(() => chatStore.streaming, async (streaming, wasStreaming) => {
  if (wasStreaming && !streaming && props.modelValue) {
    const messages = chatStore.messages
    if (messages.length > 0) {
      const lastMsg = messages[messages.length - 1]
      if (lastMsg.role === 'assistant' && lastMsg.content) {
        response.value = lastMsg.content
        await playResponseTTS(lastMsg.content)
      }
    }
  }
})

// Play TTS for AI response
async function playResponseTTS(text: string) {
  if (!text.trim() || !autoPlayTTS.value) {
    // No TTS — resume listening immediately
    resumeListening()
    return
  }

  conversationState.value = 'speaking'
  if (vad) vad.pause()

  try {
    const result = await speechApi.synthesize(text)
    if (result.audio) {
      await playAudioFromBase64(result.audio, result.content_type || 'audio/mp3')
    }
  } catch (e) {
    console.error('TTS playback failed:', e)
  } finally {
    // Resume listening for next turn
    resumeListening()
  }
}

// Cleanup on unmount
onUnmounted(() => {
  stopAll()
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
        <div
          class="talk-mode-container glass-card w-full p-6"
          :class="isMobile
            ? 'mobile-fullscreen'
            : 'max-w-md mx-4 rounded-2xl'"
        >
          <!-- Header -->
          <div class="flex items-center justify-between mb-6">
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('chat.talkMode.title') }}
            </h3>
            <div class="flex items-center gap-2">
              <!-- Auto-play TTS toggle button -->
              <button
                class="p-2 rounded-lg transition-colors cursor-pointer hover:bg-gray-200 dark:hover:bg-white/10 text-gray-500 dark:text-gray-400"
                :title="autoPlayTTS ? t('chat.talkMode.mute') : t('chat.talkMode.unmute')"
                @click="toggleAutoPlay"
              >
                <svg v-if="autoPlayTTS" class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.536 8.464a5 5 0 010 7.072m2.828-9.9a9 9 0 010 12.728M5.586 15H4a1 1 0 01-1-1v-4a1 1 0 011-1h1.586l4.707-4.707C10.923 3.663 12 4.109 12 5v14c0 .891-1.077 1.337-1.707.707L5.586 15z" />
                </svg>
                <svg v-else class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5.586 15H4a1 1 0 01-1-1v-4a1 1 0 011-1h1.586l4.707-4.707C10.923 3.663 12 4.109 12 5v14c0 .891-1.077 1.337-1.707.707L5.586 15z" />
                  <line x1="3" y1="21" x2="21" y2="3" stroke="currentColor" stroke-width="2" stroke-linecap="round" />
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

          <!-- Main action button -->
          <div class="flex flex-col items-center mb-6">
            <button
              class="relative w-24 h-24 rounded-full flex items-center justify-center transition-all duration-300 cursor-pointer"
              :class="{
                'bg-green-500 hover:bg-green-600': conversationState === 'listening',
                'bg-yellow-500': conversationState === 'transcribing',
                'bg-blue-500': conversationState === 'processing',
                'bg-purple-500': conversationState === 'speaking',
                'bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400': conversationState === 'idle',
              }"
              @click="toggleListening"
            >
              <!-- Audio level ring (listening) -->
              <div
                v-if="conversationState === 'listening'"
                class="absolute inset-0 rounded-full transition-transform duration-100"
                :style="{ transform: `scale(${1 + audioLevel / 150})`, opacity: 0.2, background: 'rgba(34, 197, 94, 0.4)' }"
              />
              <div
                v-if="conversationState === 'listening'"
                class="absolute inset-0 rounded-full border-4 border-green-300/40 talk-breathing"
              />

              <!-- Icons per state -->
              <svg v-if="conversationState === 'listening'" class="w-10 h-10 text-white relative z-10" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11a7 7 0 01-7 7m0 0a7 7 0 01-7-7m7 7v4m0 0H8m4 0h4m-4-8a3 3 0 01-3-3V5a3 3 0 116 0v6a3 3 0 01-3 3z" />
              </svg>
              <svg v-else-if="conversationState === 'transcribing' || conversationState === 'processing'" class="w-10 h-10 text-white animate-spin" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
              </svg>
              <svg v-else-if="conversationState === 'speaking'" class="w-10 h-10 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.536 8.464a5 5 0 010 7.072m2.828-9.9a9 9 0 010 12.728M5.586 15H4a1 1 0 01-1-1v-4a1 1 0 011-1h1.586l4.707-4.707C10.923 3.663 12 4.109 12 5v14c0 .891-1.077 1.337-1.707.707L5.586 15z" />
              </svg>
              <svg v-else class="w-10 h-10 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11a7 7 0 01-7 7m0 0a7 7 0 01-7-7m7 7v4m0 0H8m4 0h4m-4-8a3 3 0 01-3-3V5a3 3 0 116 0v6a3 3 0 01-3 3z" />
              </svg>
            </button>

            <!-- Status text -->
            <p class="mt-4 text-sm text-gray-600 dark:text-gray-400">
              <template v-if="conversationState === 'listening'">
                {{ t('chat.talkMode.listening') }}
              </template>
              <template v-else-if="conversationState === 'transcribing'">
                {{ t('chat.voiceTranscribing') }}
              </template>
              <template v-else-if="conversationState === 'processing'">
                {{ t('chat.talkMode.processing') }}
              </template>
              <template v-else-if="conversationState === 'speaking'">
                {{ t('chat.talkMode.speaking') }}
              </template>
              <template v-else>
                {{ t('chat.talkMode.tapToStart') }}
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
            @downloaded="open"
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
            {{ t('chat.talkMode.conversationDesc') }}
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

/* Mobile fullscreen */
.mobile-fullscreen {
  max-width: 100% !important;
  margin: 0 !important;
  border-radius: 0 !important;
  height: 100%;
  max-height: 100vh;
  display: flex;
  flex-direction: column;
  justify-content: center;
  padding-top: env(safe-area-inset-top, 20px);
  padding-bottom: env(safe-area-inset-bottom, 20px);
}

/* Breathing animation for listening state */
@keyframes talk-breathing {
  0%, 100% { transform: scale(1); opacity: 0.3; }
  50% { transform: scale(1.08); opacity: 0.15; }
}
.talk-breathing {
  animation: talk-breathing 2s ease-in-out infinite;
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
