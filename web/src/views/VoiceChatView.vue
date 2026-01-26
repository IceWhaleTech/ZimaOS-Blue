<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, watch } from 'vue'
import {
  VoiceWebSocket,
  AudioRecorder,
  blobToBase64,
  playAudioFromBase64,
  voiceApi,
} from '@/api/voice'
import type { VoiceSessionState, Voice } from '@/api/voice'
import { WakeWordDetector } from '@/utils/wakeword'

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
const autoPlayResponse = ref(true)
const continuousListening = ref(false)
const isPlaying = ref(false)

// Wake word state
const wakeWordEnabled = ref(false)
const wakeWord = ref('hey echo')
const wakeWordListening = ref(false)
const wakeWordSupported = ref(WakeWordDetector.isSupported())

// WebSocket, recorder, and wake word detector
let ws: VoiceWebSocket | null = null
let recorder: AudioRecorder | null = null
let wakeWordDetector: WakeWordDetector | null = null

// Computed
const stateText = computed(() => {
  switch (sessionState.value) {
    case 'idle':
      return 'Ready'
    case 'listening':
      return 'Listening...'
    case 'processing':
      return 'Processing...'
    case 'speaking':
      return 'Speaking...'
    default:
      return 'Unknown'
  }
})

const canRecord = computed(() => {
  return isConnected.value && !isRecording.value && sessionState.value === 'idle'
})

// Lifecycle
onMounted(async () => {
  await loadVoices()
  await connectWebSocket()
  initWakeWordDetector()
})

onUnmounted(() => {
  disconnect()
  stopWakeWordDetection()
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
  }

  ws.onResponse = (text) => {
    response.value = text
    messages.value.push({ role: 'assistant', text })
  }

  ws.onAudioResponse = async (audio, contentType) => {
    if (autoPlayResponse.value) {
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
      auto_play_response: autoPlayResponse.value,
      continuous_listening: continuousListening.value,
    })
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Connection failed'
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
      const base64 = await blobToBase64(audio)
      const format = getFormatFromMimeType(recorder?.mimeType || 'audio/webm')
      ws?.sendAudio(base64, format)
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to process audio'
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
    error.value = e instanceof Error ? e.message : 'Failed to start recording'
  }
}

function stopRecording() {
  if (recorder?.isRecording) {
    recorder.stop()
  }
}

function getFormatFromMimeType(mimeType: string): string {
  if (mimeType.includes('webm')) return 'webm'
  if (mimeType.includes('ogg')) return 'ogg'
  if (mimeType.includes('mp4')) return 'mp4'
  if (mimeType.includes('wav')) return 'wav'
  return 'webm'
}

function updateConfig() {
  ws?.updateConfig({
    language: selectedLanguage.value,
    voice: selectedVoice.value,
    auto_play_response: autoPlayResponse.value,
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
  if (!wakeWordSupported.value) return

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
      error.value = 'Failed to start wake word detection'
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

// Watch for wake word toggle
watch(wakeWordEnabled, () => {
  toggleWakeWordDetection()
})
</script>

<template>
  <div class="voice-chat-view p-6 max-w-4xl mx-auto">
    <div class="flex items-center justify-between mb-6">
      <h1 class="text-2xl font-bold text-white">Voice Assistant</h1>
      <div class="flex items-center gap-2">
        <span
          class="w-3 h-3 rounded-full"
          :class="isConnected ? 'bg-green-500' : 'bg-red-500'"
        ></span>
        <span class="text-sm text-gray-400">
          {{ isConnected ? 'Connected' : 'Disconnected' }}
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
        <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
        </svg>
      </button>
    </div>

    <!-- Settings -->
    <div class="bg-gray-800 rounded-lg p-4 mb-6">
      <h2 class="text-sm font-medium text-gray-400 mb-3">Settings</h2>
      <div class="grid grid-cols-2 md:grid-cols-4 gap-4 mb-4">
        <!-- Language -->
        <div>
          <label class="block text-xs text-gray-500 mb-1">Language</label>
          <select
            v-model="selectedLanguage"
            class="w-full bg-gray-700 text-white text-sm rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500"
            @change="updateConfig"
          >
            <option value="en">English</option>
            <option value="zh">Chinese</option>
            <option value="ja">Japanese</option>
            <option value="ko">Korean</option>
            <option value="de">German</option>
            <option value="fr">French</option>
            <option value="es">Spanish</option>
          </select>
        </div>

        <!-- Voice -->
        <div>
          <label class="block text-xs text-gray-500 mb-1">Voice</label>
          <select
            v-model="selectedVoice"
            class="w-full bg-gray-700 text-white text-sm rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500"
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
              class="w-4 h-4 rounded border-gray-600 bg-gray-700 text-blue-600 focus:ring-blue-500"
              @change="updateConfig"
            />
            <span class="text-sm text-gray-300">Auto-play</span>
          </label>
        </div>

        <!-- Continuous -->
        <div class="flex items-center">
          <label class="flex items-center gap-2 cursor-pointer">
            <input
              v-model="continuousListening"
              type="checkbox"
              class="w-4 h-4 rounded border-gray-600 bg-gray-700 text-blue-600 focus:ring-blue-500"
              @change="updateConfig"
            />
            <span class="text-sm text-gray-300">Continuous</span>
          </label>
        </div>
      </div>

      <!-- Wake Word Settings -->
      <div v-if="wakeWordSupported" class="border-t border-gray-700 pt-4 mt-4">
        <div class="flex items-center justify-between mb-3">
          <h3 class="text-sm font-medium text-gray-400">Wake Word Detection</h3>
          <label class="flex items-center gap-2 cursor-pointer">
            <input
              v-model="wakeWordEnabled"
              type="checkbox"
              class="w-4 h-4 rounded border-gray-600 bg-gray-700 text-blue-600 focus:ring-blue-500"
            />
            <span class="text-sm text-gray-300">Enable</span>
          </label>
        </div>
        <div class="flex items-center gap-4">
          <div class="flex-1">
            <label class="block text-xs text-gray-500 mb-1">Wake Word</label>
            <input
              v-model="wakeWord"
              type="text"
              class="w-full bg-gray-700 text-white text-sm rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500"
              placeholder="e.g., hey echo"
              @change="updateWakeWord"
            />
          </div>
          <div class="flex items-center gap-2 pt-5">
            <span
              class="w-2 h-2 rounded-full"
              :class="wakeWordListening ? 'bg-green-500 animate-pulse' : 'bg-gray-500'"
            ></span>
            <span class="text-xs text-gray-500">
              {{ wakeWordListening ? 'Listening for wake word...' : 'Not listening' }}
            </span>
          </div>
        </div>
      </div>
    </div>

    <!-- Messages -->
    <div class="bg-gray-800 rounded-lg mb-6 min-h-[300px] max-h-[400px] overflow-y-auto">
      <div v-if="messages.length === 0" class="p-8 text-center text-gray-500">
        <svg xmlns="http://www.w3.org/2000/svg" class="h-12 w-12 mx-auto mb-4 opacity-50" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11a7 7 0 01-7 7m0 0a7 7 0 01-7-7m7 7v4m0 0H8m4 0h4m-4-8a3 3 0 01-3-3V5a3 3 0 116 0v6a3 3 0 01-3 3z" />
        </svg>
        <p>Press and hold the microphone button to speak</p>
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
            :class="msg.role === 'user' ? 'bg-blue-600 text-white' : 'bg-gray-700 text-gray-200'"
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
            'text-blue-400': sessionState === 'speaking',
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
          'bg-blue-600 hover:bg-blue-700': canRecord && !isRecording,
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
        {{ isRecording ? 'Release to send' : 'Hold to speak' }}
      </p>

      <!-- Clear Button -->
      <button
        v-if="messages.length > 0"
        class="text-sm text-gray-400 hover:text-gray-300"
        @click="clearMessages"
      >
        Clear conversation
      </button>
    </div>
  </div>
</template>
