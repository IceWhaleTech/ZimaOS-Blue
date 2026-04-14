<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ttsAudioManager, voiceApi } from '@/api/voice'
import { speechApi } from '@/api/speech'
import { convertToWav } from '@/utils/audioConverter'
import { EnergyVAD } from '@/utils/vad'
import {
  isTtsAutoPlayEnabled,
  isTtsSpeechMuted,
  setTtsAutoPlayEnabled,
} from '@/utils/ttsPreferences'
import { createProcessTraceItem, type ProcessTraceItem } from '@/utils/processTrace'
import ModelDownloadPrompt from '@/components/speech/ModelDownloadPrompt.vue'
import { useLocaleStore } from '@/stores/locale'
import { useChatStore } from '@/stores/chat'

const { t, te } = useI18n()
const localeStore = useLocaleStore()
const chatStore = useChatStore()

const props = defineProps<{
  modelValue: boolean
  conversationId?: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  transcript: [text: string]
  response: [text: string]
}>()

type ConversationState =
  | 'idle'
  | 'listening'
  | 'preparing_audio'
  | 'uploading_audio'
  | 'transcribing'
  | 'requesting'
  | 'waiting_response'
  | 'tool_processing'
  | 'awaiting_confirmation'
  | 'tts_preparing'
  | 'speaking'
  | 'retrying'

type TalkBubble = {
  id: string
  role: 'user' | 'assistant'
  text: string
}

type MarkdownModule = typeof import('@/utils/markdown')
type TalkModeErrorLike = {
  name?: string
  message?: string
  error_code?: string
  response?: {
    data?: {
      error_code?: unknown
      error?: unknown
      message?: unknown
    }
  }
}

const conversationState = ref<ConversationState>('idle')
const autoPlayTTS = ref(isTtsAutoPlayEnabled())
const error = ref<string | null>(null)
const conversationBubbles = ref<TalkBubble[]>([])
const bubbleListRef = ref<HTMLElement | null>(null)
const showASRDownloadPrompt = ref(false)
const audioLevel = ref(0)
const isMobile = ref(false)
const lastSpokenAssistantMessageId = ref<string | null>(null)
const uploadProgress = ref(0)
const processDetailsExpanded = ref(false)
const localProcessTrace = ref<ProcessTraceItem[]>([])
const localPhase = ref<ConversationState | null>(null)
const listeningReady = ref(false)
const interruptionHint = ref<string | null>(null)

let vad: EnergyVAD | null = null
let isSynthesizingTTS = false
let ttsInterruptedByBargeIn = false
let interruptionHintTimer: number | null = null
let requestPhaseTimer: number | null = null
let markdownModulePromise: Promise<MarkdownModule> | null = null

function loadMarkdownModule(): Promise<MarkdownModule> {
  if (!markdownModulePromise) {
    markdownModulePromise = import('@/utils/markdown')
  }
  return markdownModulePromise
}

const recentProcessTrace = computed(() =>
  [...localProcessTrace.value, ...chatStore.processTrace]
    .sort((a, b) => a.timestamp - b.timestamp)
    .slice(-3)
    .reverse()
)

const fullProcessTrace = computed(() =>
  [...localProcessTrace.value, ...chatStore.processTrace].sort((a, b) => b.timestamp - a.timestamp)
)

const hasProcessTrace = computed(() => fullProcessTrace.value.length > 0)
const uploadPercent = computed(() => Math.round(uploadProgress.value * 100))

function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  let value = bytes
  let index = 0
  while (value >= 1024 && index < units.length - 1) {
    value /= 1024
    index++
  }
  return `${value >= 10 || index === 0 ? value.toFixed(0) : value.toFixed(1)} ${units[index]}`
}

function formatDuration(seconds?: number): string {
  if (!seconds || seconds <= 0) return '0.0s'
  if (seconds < 10) return `${seconds.toFixed(1)}s`
  return `${Math.round(seconds)}s`
}

function getTalkModeErrorMeta(error: unknown): {
  name?: string
  message?: string
  errorCode?: string
  serverMessage?: string
} {
  if (!error || typeof error !== 'object') return {}
  const source = error as TalkModeErrorLike
  const responseData = source.response?.data
  return {
    name: typeof source.name === 'string' ? source.name : undefined,
    message: typeof source.message === 'string' ? source.message : undefined,
    errorCode:
      typeof source.error_code === 'string'
        ? source.error_code
        : typeof responseData?.error_code === 'string'
          ? responseData.error_code
          : undefined,
    serverMessage:
      typeof responseData?.error === 'string'
        ? responseData.error
        : typeof responseData?.message === 'string'
          ? responseData.message
          : undefined,
  }
}

function summarizeText(text: string, maxLen = 96): string {
  const normalized = text.replace(/\s+/g, ' ').trim()
  if (!normalized) return ''
  if (normalized.length <= maxLen) return normalized
  return normalized.slice(0, maxLen - 1) + '…'
}

function getLatestChatTrace(
  categories: Array<ProcessTraceItem['category']>,
  statuses: Array<ProcessTraceItem['status']> = ['active', 'pending', 'error']
): ProcessTraceItem | null {
  for (let i = chatStore.processTrace.length - 1; i >= 0; i--) {
    const item = chatStore.processTrace[i]
    if (!item) continue
    if (!categories.includes(item.category)) continue
    if (!statuses.includes(item.status)) continue
    return item
  }
  return null
}

function trimLocalProcessTrace() {
  if (localProcessTrace.value.length <= 24) return
  localProcessTrace.value = localProcessTrace.value.slice(-24)
}

function upsertLocalProcessTrace(
  item: Omit<ProcessTraceItem, 'id' | 'timestamp'>,
  options?: { replaceLatestByEvent?: boolean }
) {
  const next = createProcessTraceItem(item)
  const trace = [...localProcessTrace.value]
  if (options?.replaceLatestByEvent) {
    for (let i = trace.length - 1; i >= 0; i--) {
      if (trace[i]?.event === next.event) {
        trace[i] = next
        localProcessTrace.value = trace
        trimLocalProcessTrace()
        return
      }
    }
  }
  trace.push(next)
  localProcessTrace.value = trace
  trimLocalProcessTrace()
}

function clearRequestPhaseTimer() {
  if (requestPhaseTimer !== null) {
    window.clearTimeout(requestPhaseTimer)
    requestPhaseTimer = null
  }
}

function setLocalPhase(next: ConversationState | null) {
  localPhase.value = next
  syncConversationState()
}

function setInterruptionHint(text: string) {
  interruptionHint.value = text
  if (interruptionHintTimer !== null) {
    window.clearTimeout(interruptionHintTimer)
  }
  interruptionHintTimer = window.setTimeout(() => {
    interruptionHint.value = null
    interruptionHintTimer = null
  }, 2500)
}

function resolveTraceText(
  key: string,
  fallback: string,
  named?: Record<string, string | number>
): string {
  const path = `chat.processTrace.${key}`
  return te(path) ? String(named ? t(path, named) : t(path)) : fallback
}

function resolveTraceField(key: string, fallback: string): string {
  return resolveTraceText(`fields.${key}`, fallback)
}

function resolveTraceSummaryValue(key: string, fallback: string): string {
  return resolveTraceText(`summaryValues.${key}`, fallback)
}

function buildAudioDetail(
  blob: Blob,
  format: string,
  options?: {
    duration?: number
    progress?: number
    transcript?: string
  }
): string {
  const fieldFormat = resolveTraceField('format', 'Format')
  const fieldSize = resolveTraceField('size', 'Size')
  const fieldDuration = resolveTraceField('duration', 'Duration')
  const fieldUpload = resolveTraceField('upload', 'Upload')
  const fieldTranscript = resolveTraceField('transcript', 'Transcript')
  const lines = [`${fieldFormat}: ${format}`, `${fieldSize}: ${formatBytes(blob.size)}`]
  if (typeof options?.duration === 'number') {
    lines.push(`${fieldDuration}: ${formatDuration(options.duration)}`)
  }
  if (typeof options?.progress === 'number') {
    lines.push(`${fieldUpload}: ${Math.round(options.progress * 100)}%`)
  }
  if (options?.transcript) {
    lines.push(`${fieldTranscript}: ${summarizeText(options.transcript)}`)
  }
  return lines.join('\n')
}

function buildRequestDetail(text: string, interrupted: boolean): string {
  const fieldTranscript = resolveTraceField('transcript', 'Transcript')
  const fieldMode = resolveTraceField('mode', 'Mode')
  const fieldConversation = resolveTraceField('conversation', 'Conversation')
  const lines = [`${fieldTranscript}: ${summarizeText(text)}`]
  lines.push(
    `${fieldMode}: ${
      interrupted
        ? resolveTraceSummaryValue('interruptCurrentReply', 'Interrupt current reply')
        : resolveTraceSummaryValue('sendNewRequest', 'Send new request')
    }`
  )
  if (props.conversationId) {
    lines.push(`${fieldConversation}: ${props.conversationId}`)
  }
  return lines.join('\n')
}

function deriveChatConversationState(): ConversationState | null {
  if (chatStore.awaitingConfirmation) {
    return 'awaiting_confirmation'
  }
  if (getLatestChatTrace(['retry', 'recovery'])) {
    return 'retrying'
  }
  if (chatStore.toolExecuting) {
    return 'tool_processing'
  }
  if (chatStore.streaming || chatStore.sending) {
    return 'waiting_response'
  }
  return null
}

function syncConversationState() {
  if (!props.modelValue) {
    conversationState.value = 'idle'
    return
  }
  if (localPhase.value) {
    conversationState.value = localPhase.value
    return
  }
  const storeState = deriveChatConversationState()
  if (storeState) {
    conversationState.value = storeState
    return
  }
  conversationState.value = listeningReady.value ? 'listening' : 'idle'
}

function pushBubble(role: 'user' | 'assistant', text: string) {
  const value = text.trim()
  if (!value) return
  const prev = conversationBubbles.value[conversationBubbles.value.length - 1]
  if (prev && prev.role === role && prev.text === value) return
  conversationBubbles.value.push({
    id: `${role}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
    role,
    text: value,
  })
  nextTick(() => {
    const el = bubbleListRef.value
    if (el) el.scrollTop = el.scrollHeight
  })
}

function clearInterruptionHint() {
  if (interruptionHintTimer !== null) {
    window.clearTimeout(interruptionHintTimer)
    interruptionHintTimer = null
  }
  interruptionHint.value = null
}

function stopTTSPlayback() {
  ttsAudioManager.stop()
  if (isSynthesizingTTS || conversationState.value === 'speaking') {
    voiceApi.stopSpeaking().catch(() => {})
  }
}

async function resumeListening(options?: { interrupted?: boolean }) {
  if (!props.modelValue) return
  clearRequestPhaseTimer()
  setLocalPhase(null)
  uploadProgress.value = 0
  if (options?.interrupted) {
    setInterruptionHint(t('chat.stillListening'))
  }
  if (vad && vad.isListening) {
    try {
      const resumed = await vad.resume()
      if (resumed) {
        listeningReady.value = true
        syncConversationState()
        return
      }
    } catch {
      // Fall through to a full restart when the retained audio pipeline is no longer healthy.
    }
  }
  if (!props.modelValue) return
  await startListening()
}

function handleBargeIn() {
  if (!props.modelValue) return
  if (!isSynthesizingTTS && !ttsAudioManager.isPlaying()) return
  ttsInterruptedByBargeIn = true
  upsertLocalProcessTrace(
    {
      source: 'client',
      event: 'barge_in',
      category: 'audio',
      status: 'active',
      label: t('chat.stillListening'),
      detail: resolveTraceText('details.bargeIn', 'Playback stopped so you can continue speaking.'),
    },
    { replaceLatestByEvent: true }
  )
  stopTTSPlayback()
  void resumeListening({ interrupted: true })
}

function handleWindowBlur() {
  stopTTSPlayback()
}

function handleVisibilityChange() {
  if (document.hidden) {
    stopTTSPlayback()
  }
}

function syncMobileState() {
  isMobile.value = window.innerWidth < 768
}

const statusLabel = computed(() => {
  if (conversationState.value === 'listening' && interruptionHint.value) {
    return interruptionHint.value
  }

  switch (conversationState.value) {
    case 'listening':
      return t('chat.talkMode.listening')
    case 'preparing_audio':
      return t('chat.talkMode.preparingAudio')
    case 'uploading_audio':
      return t('chat.talkMode.uploadingAudio', { percent: uploadPercent.value || 0 })
    case 'transcribing':
      return t('chat.voiceTranscribing')
    case 'requesting':
      return t('chat.talkMode.requesting')
    case 'waiting_response':
      return t('chat.talkMode.waitingResponse')
    case 'tool_processing':
      return t('chat.talkMode.toolProcessing')
    case 'awaiting_confirmation':
      return t('chat.awaitingConfirmation')
    case 'tts_preparing':
      return t('chat.talkMode.ttsPreparing')
    case 'speaking':
      return t('chat.talkMode.speaking')
    case 'retrying':
      return t('chat.talkMode.retrying')
    default:
      return t('chat.talkMode.tapToStart')
  }
})

function traceItemStatusClass(item: ProcessTraceItem) {
  if (item.status === 'error') return 'talk-process-item--error'
  if (item.status === 'active' || item.status === 'pending') return 'talk-process-item--active'
  return 'talk-process-item--success'
}

function traceItemLabel(item: ProcessTraceItem) {
  if (typeof item.progress === 'number' && item.progress >= 0 && item.progress < 100) {
    return `${item.label} ${Math.round(item.progress)}%`
  }
  return item.label
}

function stopAll() {
  clearRequestPhaseTimer()
  clearInterruptionHint()
  stopTTSPlayback()
  if (vad) {
    vad.destroy()
    vad = null
  }
  listeningReady.value = false
  audioLevel.value = 0
  uploadProgress.value = 0
  localPhase.value = null
  conversationState.value = 'idle'
}

function toggleListening() {
  if (conversationState.value === 'idle') {
    void startListening()
    return
  }
  stopAll()
}

function close() {
  stopAll()
  emit('update:modelValue', false)
}

function toggleAutoPlay() {
  autoPlayTTS.value = !autoPlayTTS.value
  setTtsAutoPlayEnabled(autoPlayTTS.value)
}

async function open() {
  if (!window.isSecureContext) {
    error.value = t('chat.voiceSecureContextError')
    return
  }

  if (!navigator.mediaDevices?.getUserMedia) {
    error.value = t('chat.voiceNotSupportedError')
    return
  }

  try {
    const stream = await navigator.mediaDevices.getUserMedia({ audio: true })
    stream.getTracks().forEach((track) => track.stop())
  } catch (err) {
    const { name } = getTalkModeErrorMeta(err)
    if (name === 'NotAllowedError' || name === 'PermissionDeniedError') {
      error.value = t('chat.voiceMicrophonePermissionDenied')
    } else if (name === 'NotFoundError' || name === 'DevicesNotFoundError') {
      error.value = t('chat.voiceMicrophoneNotFound')
    } else if (name === 'NotReadableError' || name === 'TrackStartError') {
      error.value = t('chat.voiceMicrophoneInUse')
    } else {
      error.value = t('chat.voiceMicrophoneError')
    }
    return
  }

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
  await startListening()
}

async function startListening() {
  if (listeningReady.value && conversationState.value === 'listening') return

  clearInterruptionHint()
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
    bargeInThreshold: 0.055,
    bargeInFrames: 7,
    bargeInMinDuration: 350,
    onSpeechEnd: async (audioBlob: Blob) => {
      listeningReady.value = false
      uploadProgress.value = 0
      setLocalPhase('preparing_audio')
      const preparingLabel = t('chat.talkMode.preparingAudio')
      upsertLocalProcessTrace(
        {
          source: 'client',
          event: 'audio_preparing',
          category: 'audio',
          status: 'active',
          label: preparingLabel,
          detail: buildAudioDetail(audioBlob, 'webm'),
        },
        { replaceLatestByEvent: true }
      )

      try {
        const wavBlob = await convertToWav(audioBlob)
        if (!wavBlob) {
          await resumeListening()
          return
        }

        upsertLocalProcessTrace(
          {
            source: 'client',
            event: 'audio_preparing',
            category: 'audio',
            status: 'success',
            label: preparingLabel,
            detail: buildAudioDetail(wavBlob, 'wav'),
          },
          { replaceLatestByEvent: true }
        )

        setLocalPhase('uploading_audio')
        upsertLocalProcessTrace(
          {
            source: 'client',
            event: 'audio_uploading',
            category: 'audio',
            status: 'active',
            label: t('chat.talkMode.uploadingAudio', { percent: 0 }),
            detail: buildAudioDetail(wavBlob, 'wav', { progress: 0 }),
            progress: 0,
          },
          { replaceLatestByEvent: true }
        )

        const result = await speechApi.transcribe(wavBlob, 'wav', lang, {
          onUploadProgress: (progress) => {
            uploadProgress.value = progress
            upsertLocalProcessTrace(
              {
                source: 'client',
                event: 'audio_uploading',
                category: 'audio',
                status: progress >= 1 ? 'success' : 'active',
                label:
                  progress >= 1
                    ? t('chat.talkMode.audioUploaded')
                    : t('chat.talkMode.uploadingAudio', {
                        percent: Math.round(progress * 100),
                      }),
                detail: buildAudioDetail(wavBlob, 'wav', { progress }),
                progress: Math.round(progress * 100),
              },
              { replaceLatestByEvent: true }
            )
            if (progress >= 1 && localPhase.value === 'uploading_audio') {
              setLocalPhase('transcribing')
              upsertLocalProcessTrace(
                {
                  source: 'client',
                  event: 'audio_transcribing',
                  category: 'audio',
                  status: 'active',
                  label: t('chat.voiceTranscribing'),
                  detail: buildAudioDetail(wavBlob, 'wav'),
                },
                { replaceLatestByEvent: true }
              )
            }
          },
        })

        setLocalPhase('transcribing')
        upsertLocalProcessTrace(
          {
            source: 'client',
            event: 'audio_transcribing',
            category: 'audio',
            status: 'success',
            label: t('chat.talkMode.transcriptReady'),
            detail: buildAudioDetail(wavBlob, 'wav', {
              duration: result.duration,
              transcript: result.text,
            }),
          },
          { replaceLatestByEvent: true }
        )

        if (result.text?.trim()) {
          const transcript = result.text.trim()
          const interrupted = chatStore.streaming
          pushBubble('user', transcript)
          upsertLocalProcessTrace({
            source: 'client',
            event: 'talk_request_summary',
            category: 'summary',
            status: 'info',
            label: interrupted
              ? t('chat.talkMode.interruptReady')
              : t('chat.talkMode.requestReady'),
            command: summarizeText(transcript),
            detail: buildRequestDetail(transcript, interrupted),
          })
          upsertLocalProcessTrace(
            {
              source: 'client',
              event: 'talk_request_dispatched',
              category: 'lifecycle',
              status: 'active',
              label: interrupted
                ? t('chat.talkMode.interruptingRequest')
                : t('chat.talkMode.requesting'),
              detail: interrupted
                ? resolveTraceText(
                    'details.voiceInterruptDispatched',
                    'Blue is restarting with your latest voice interruption.'
                  )
                : resolveTraceText(
                    'details.voiceRequestDispatched',
                    'Blue is sending your voice request.'
                  ),
            },
            { replaceLatestByEvent: true }
          )
          upsertLocalProcessTrace(
            {
              source: 'client',
              event: 'talk_waiting_response',
              category: 'lifecycle',
              status: 'active',
              label: t('chat.talkMode.waitingResponse'),
              detail: resolveTraceText(
                'details.voiceWaitingForResponse',
                'Waiting for the first response from Blue.'
              ),
            },
            { replaceLatestByEvent: true }
          )
          setLocalPhase('requesting')
          clearRequestPhaseTimer()
          requestPhaseTimer = window.setTimeout(() => {
            if (localPhase.value === 'requesting') {
              setLocalPhase(null)
            }
            requestPhaseTimer = null
          }, 350)
          emit('transcript', transcript)
        } else {
          await resumeListening()
        }
      } catch (e) {
        console.error('Transcription failed:', e)
        const { errorCode, name, message, serverMessage } = getTalkModeErrorMeta(e)
        if (errorCode === 'timeout' || name === 'AbortError') {
          error.value = t('chat.voiceTranscriptionTimeout')
        } else if (errorCode === 'on_device_unavailable') {
          error.value = t('speech.onDeviceUnavailableError')
        } else {
          error.value = serverMessage || message || t('chat.talkMode.transcriptionError')
        }
        upsertLocalProcessTrace(
          {
            source: 'client',
            event: 'audio_transcribing',
            category: 'audio',
            status: 'error',
            label: t('chat.talkMode.transcriptionError'),
            detail:
              error.value ||
              resolveTraceText('details.transcriptionFailed', 'Transcription failed.'),
          },
          { replaceLatestByEvent: true }
        )
        await resumeListening()
      }
    },
    onVolumeChange: (level: number) => {
      audioLevel.value = level * 100
    },
    onBargeIn: handleBargeIn,
  })

  try {
    await vad.start()
    listeningReady.value = true
    setLocalPhase(null)
  } catch (e) {
    error.value = t('chat.voiceMicrophoneError')
    console.error('Failed to start VAD:', e)
    listeningReady.value = false
    conversationState.value = 'idle'
  }
}

async function playResponseTTS(text: string) {
  if (!text.trim() || !autoPlayTTS.value || isTtsSpeechMuted()) {
    await resumeListening()
    return
  }

  if (vad) {
    vad.pause({ monitorBargeIn: true })
  }
  listeningReady.value = false

  ttsInterruptedByBargeIn = false
  setLocalPhase('tts_preparing')
  upsertLocalProcessTrace(
    {
      source: 'client',
      event: 'tts_preparing',
      category: 'tts',
      status: 'active',
      label: t('chat.talkMode.ttsPreparing'),
      detail: summarizeText(text),
    },
    { replaceLatestByEvent: true }
  )

  try {
    isSynthesizingTTS = true
    const result = await speechApi.synthesize(text)
    if (!result.audio || ttsInterruptedByBargeIn) {
      return
    }

    setLocalPhase('speaking')
    upsertLocalProcessTrace(
      {
        source: 'client',
        event: 'tts_playback',
        category: 'tts',
        status: 'active',
        label: t('chat.talkMode.speaking'),
        detail: summarizeText(text),
      },
      { replaceLatestByEvent: true }
    )
    await ttsAudioManager.play(result.audio, result.content_type || 'audio/mp3')
    upsertLocalProcessTrace(
      {
        source: 'client',
        event: 'tts_playback',
        category: 'tts',
        status: 'success',
        label: t('chat.talkMode.speakingCompleted'),
        detail: summarizeText(text),
      },
      { replaceLatestByEvent: true }
    )
  } catch (e) {
    const { name, message } = getTalkModeErrorMeta(e)
    if (name !== 'AbortError') {
      console.error('TTS playback failed:', e)
      upsertLocalProcessTrace(
        {
          source: 'client',
          event: 'tts_playback',
          category: 'tts',
          status: 'error',
          label: t('chat.ttsError'),
          detail:
            message || resolveTraceText('details.ttsPlaybackFailed', 'TTS playback failed.'),
        },
        { replaceLatestByEvent: true }
      )
    }
  } finally {
    isSynthesizingTTS = false
    setLocalPhase(null)
    if (props.modelValue && !chatStore.streaming && !ttsInterruptedByBargeIn) {
      await resumeListening()
    }
  }
}

onMounted(() => {
  syncMobileState()
  window.addEventListener('resize', syncMobileState)
  window.addEventListener('blur', handleWindowBlur)
  document.addEventListener('visibilitychange', handleVisibilityChange)
})

watch(
  () => props.modelValue,
  (newValue) => {
    if (newValue) {
      conversationBubbles.value = []
      localProcessTrace.value = []
      processDetailsExpanded.value = false
      uploadProgress.value = 0
      lastSpokenAssistantMessageId.value = null
      clearInterruptionHint()
      void open()
    } else {
      stopAll()
    }
  }
)

watch(
  () => [
    chatStore.streaming,
    chatStore.sending,
    chatStore.toolExecuting,
    chatStore.awaitingConfirmation,
    chatStore.processTrace.length,
  ],
  () => {
    if (localPhase.value === 'requesting' && (chatStore.streaming || chatStore.sending)) {
      setLocalPhase(null)
    } else {
      syncConversationState()
    }
  }
)

watch(
  () => chatStore.streaming,
  async (streaming, wasStreaming) => {
    if (wasStreaming && !streaming && props.modelValue) {
      const messages = chatStore.messages
      const lastMsg = messages.length > 0 ? messages[messages.length - 1] : undefined
      if (lastMsg?.role === 'assistant' && lastMsg.content) {
        const { markdownToText } = await loadMarkdownModule()
        const plainText = markdownToText(lastMsg.content)
        if (lastSpokenAssistantMessageId.value === lastMsg.id) {
          await resumeListening()
          return
        }
        pushBubble('assistant', plainText)
        emit('response', plainText)
        await playResponseTTS(plainText)
        lastSpokenAssistantMessageId.value = lastMsg.id
      } else if (props.modelValue && !isSynthesizingTTS) {
        await resumeListening()
      }
    }
  }
)

onUnmounted(() => {
  stopAll()
  window.removeEventListener('resize', syncMobileState)
  window.removeEventListener('blur', handleWindowBlur)
  document.removeEventListener('visibilitychange', handleVisibilityChange)
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
          class="talk-mode-container w-full p-6"
          :class="isMobile ? 'mobile-fullscreen' : 'glass-card max-w-md mx-4 rounded-2xl'"
        >
          <div class="flex items-center justify-between mb-4">
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('chat.talkMode.title') }}
            </h3>
            <div class="flex items-center gap-2">
              <button
                class="p-2 rounded-lg transition-colors cursor-pointer hover:bg-gray-200 dark:hover:bg-white/10 text-gray-500 dark:text-gray-400"
                :title="autoPlayTTS ? t('chat.talkMode.mute') : t('chat.talkMode.unmute')"
                @click="toggleAutoPlay"
              >
                <svg
                  v-if="autoPlayTTS"
                  class="w-5 h-5"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M15.536 8.464a5 5 0 010 7.072m2.828-9.9a9 9 0 010 12.728M5.586 15H4a1 1 0 01-1-1v-4a1 1 0 011-1h1.586l4.707-4.707C10.923 3.663 12 4.109 12 5v14c0 .891-1.077 1.337-1.707.707L5.586 15z"
                  />
                </svg>
                <svg
                  v-else
                  class="w-5 h-5"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M5.586 15H4a1 1 0 01-1-1v-4a1 1 0 011-1h1.586l4.707-4.707C10.923 3.663 12 4.109 12 5v14c0 .891-1.077 1.337-1.707.707L5.586 15z"
                  />
                  <line
                    x1="3"
                    y1="21"
                    x2="21"
                    y2="3"
                    stroke="currentColor"
                    stroke-width="2"
                    stroke-linecap="round"
                  />
                </svg>
              </button>
              <button
                class="p-2 rounded-lg hover:bg-gray-200 dark:hover:bg-white/10 transition-colors cursor-pointer"
                @click="close"
              >
                <svg
                  class="w-5 h-5 text-gray-500 dark:text-gray-400"
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
          </div>

          <div
            ref="bubbleListRef"
            class="talk-bubble-list mb-4"
          >
            <div
              v-for="bubble in conversationBubbles"
              :key="bubble.id"
              class="talk-bubble-row"
              :class="bubble.role === 'user' ? 'justify-end' : 'justify-start'"
            >
              <div
                class="talk-bubble"
                :class="bubble.role === 'user' ? 'talk-bubble-user' : 'talk-bubble-assistant'"
              >
                {{ bubble.text }}
              </div>
            </div>
            <div
              v-if="!conversationBubbles.length"
              class="text-xs text-gray-500 dark:text-gray-400 text-center py-6"
            >
              {{ t('chat.talkMode.conversationDesc') }}
            </div>
          </div>

          <div class="flex flex-col items-center">
            <button
              class="relative w-24 h-24 rounded-full flex items-center justify-center transition-all duration-300 cursor-pointer"
              :class="{
                'bg-green-500 hover:bg-green-600': conversationState === 'listening',
                'bg-amber-500':
                  conversationState === 'preparing_audio' ||
                  conversationState === 'uploading_audio' ||
                  conversationState === 'transcribing',
                'bg-sky-500':
                  conversationState === 'requesting' ||
                  conversationState === 'waiting_response' ||
                  conversationState === 'tts_preparing',
                'bg-orange-500':
                  conversationState === 'retrying' || conversationState === 'tool_processing',
                'bg-rose-500': conversationState === 'awaiting_confirmation',
                'bg-indigo-500': conversationState === 'speaking',
                'bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400':
                  conversationState === 'idle',
              }"
              @click="toggleListening"
            >
              <div
                v-if="conversationState === 'listening'"
                class="absolute inset-0 rounded-full transition-transform duration-100"
                :style="{
                  transform: `scale(${1 + audioLevel / 150})`,
                  opacity: 0.2,
                  background: 'rgba(34, 197, 94, 0.4)',
                }"
              />
              <div
                v-if="conversationState === 'listening'"
                class="absolute inset-0 rounded-full border-4 border-green-300/40 talk-breathing"
              />

              <svg
                v-if="conversationState === 'listening' || conversationState === 'idle'"
                class="w-10 h-10 text-white relative z-10"
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
              <svg
                v-else-if="conversationState === 'awaiting_confirmation'"
                class="w-10 h-10 text-white"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M8.228 9c.549-1.165 1.918-2 3.522-2 2.071 0 3.75 1.343 3.75 3 0 1.235-.931 2.296-2.25 2.75-.69.238-1.25.921-1.25 1.651V15m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                />
              </svg>
              <svg
                v-else-if="conversationState === 'speaking'"
                class="w-10 h-10 text-white"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M15.536 8.464a5 5 0 010 7.072m2.828-9.9a9 9 0 010 12.728M5.586 15H4a1 1 0 01-1-1v-4a1 1 0 011-1h1.586l4.707-4.707C10.923 3.663 12 4.109 12 5v14c0 .891-1.077 1.337-1.707.707L5.586 15z"
                />
              </svg>
              <svg
                v-else
                class="w-10 h-10 text-white animate-spin"
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
                />
                <path
                  class="opacity-75"
                  fill="currentColor"
                  d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                />
              </svg>
            </button>

            <p class="mt-4 text-sm text-gray-600 dark:text-gray-400 text-center">
              {{ statusLabel }}
            </p>

            <div
              v-if="conversationState === 'uploading_audio'"
              class="talk-upload-progress mt-3"
            >
              <div class="talk-upload-progress__track">
                <div
                  class="talk-upload-progress__fill"
                  :style="{ width: `${uploadPercent}%` }"
                />
              </div>
              <div class="talk-upload-progress__meta">
                {{ uploadPercent }}%
              </div>
            </div>
          </div>

          <div
            v-if="hasProcessTrace"
            class="talk-process-panel mt-5"
          >
            <div class="talk-process-panel__header">
              <span class="talk-process-panel__title">{{ t('chat.talkMode.recentActivity') }}</span>
              <button
                class="talk-process-panel__toggle"
                @click="processDetailsExpanded = !processDetailsExpanded"
              >
                {{ processDetailsExpanded ? t('chat.hideToolDetails') : t('chat.showToolDetails') }}
              </button>
            </div>

            <div class="talk-process-list">
              <div
                v-for="item in processDetailsExpanded ? fullProcessTrace : recentProcessTrace"
                :key="item.id"
                class="talk-process-item"
                :class="traceItemStatusClass(item)"
              >
                <div class="talk-process-item__row">
                  <span class="talk-process-item__dot" />
                  <span class="talk-process-item__label">{{ traceItemLabel(item) }}</span>
                </div>
                <div
                  v-if="item.command"
                  class="talk-process-item__command"
                >
                  {{ item.command }}
                </div>
                <div
                  v-if="processDetailsExpanded && item.detail"
                  class="talk-process-item__detail"
                >
                  {{ item.detail }}
                </div>
              </div>
            </div>
          </div>

          <ModelDownloadPrompt
            v-model:model-visible="showASRDownloadPrompt"
            type="asr"
            @downloaded="open"
          />

          <div
            v-if="error"
            class="mt-4 p-3 rounded-lg bg-red-500/10 text-red-400 text-sm flex items-center gap-2"
          >
            <svg
              class="w-4 h-4 flex-shrink-0"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
            <span>{{ error }}</span>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.talk-mode-container {
  max-height: 90vh;
  overflow-y: auto;
  background: rgba(17, 24, 39, 0.8);
  border: 1px solid rgba(255, 255, 255, 0.12);
  backdrop-filter: blur(10px);
}

.mobile-fullscreen {
  max-width: 100% !important;
  margin: 0 !important;
  border-radius: 0 !important;
  min-height: 100dvh;
  max-height: 100dvh;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  padding-top: env(safe-area-inset-top, 20px);
  padding-bottom: env(safe-area-inset-bottom, 20px);
  padding-inline: 16px;
  background: linear-gradient(180deg, rgba(10, 13, 23, 0.96) 0%, rgba(18, 24, 38, 0.95) 100%);
  border: none;
}

.talk-bubble-list {
  flex: 1;
  overflow-y: auto;
  padding: 6px 2px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.talk-bubble-row {
  display: flex;
}

.talk-bubble {
  max-width: 86%;
  border-radius: 16px;
  padding: 10px 12px;
  font-size: 14px;
  line-height: 1.45;
  color: #fff;
  white-space: pre-wrap;
  word-break: break-word;
}

.talk-bubble-user {
  background: rgba(37, 99, 235, 0.88);
  border-bottom-right-radius: 8px;
}

.talk-bubble-assistant {
  background: rgba(255, 255, 255, 0.14);
  border-bottom-left-radius: 8px;
}

.talk-upload-progress {
  width: min(260px, 100%);
}

.talk-upload-progress__track {
  width: 100%;
  height: 6px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.12);
  overflow: hidden;
}

.talk-upload-progress__fill {
  height: 100%;
  border-radius: 999px;
  background: linear-gradient(90deg, #38bdf8 0%, #22c55e 100%);
  transition: width 120ms ease;
}

.talk-upload-progress__meta {
  margin-top: 6px;
  text-align: center;
  font-size: 12px;
  color: rgba(255, 255, 255, 0.72);
}

.talk-process-panel {
  border-radius: 16px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: rgba(255, 255, 255, 0.06);
  padding: 12px;
}

.talk-process-panel__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 10px;
}

.talk-process-panel__title {
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: rgba(255, 255, 255, 0.72);
}

.talk-process-panel__toggle {
  font-size: 12px;
  color: rgba(255, 255, 255, 0.82);
  cursor: pointer;
}

.talk-process-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.talk-process-item {
  border-radius: 12px;
  padding: 10px 12px;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid transparent;
}

.talk-process-item--active {
  border-color: rgba(56, 189, 248, 0.34);
  background: rgba(14, 116, 144, 0.16);
}

.talk-process-item--success {
  border-color: rgba(74, 222, 128, 0.24);
}

.talk-process-item--error {
  border-color: rgba(248, 113, 113, 0.32);
  background: rgba(127, 29, 29, 0.18);
}

.talk-process-item__row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.talk-process-item__dot {
  width: 8px;
  height: 8px;
  border-radius: 999px;
  background: currentColor;
  opacity: 0.9;
}

.talk-process-item__label {
  font-size: 13px;
  line-height: 1.35;
  color: #fff;
}

.talk-process-item__command {
  margin-top: 6px;
  font-size: 12px;
  line-height: 1.4;
  color: rgba(255, 255, 255, 0.78);
  word-break: break-word;
}

.talk-process-item__detail {
  margin-top: 8px;
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 12px;
  line-height: 1.45;
  color: rgba(255, 255, 255, 0.72);
}

@keyframes talk-breathing {
  0%,
  100% {
    transform: scale(1);
    opacity: 0.3;
  }
  50% {
    transform: scale(1.08);
    opacity: 0.15;
  }
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
