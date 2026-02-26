<script setup lang="ts">
import { ref, computed, onUnmounted, nextTick, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { AudioRecorder, voiceApi } from '@/api/voice'
import { speechApi } from '@/api/speech'
import { convertToWav } from '@/utils/audioConverter'
import { EnergyVAD } from '@/utils/vad'
import ImagePreview from '@/components/chat/ImagePreview.vue'
import ModelDownloadPrompt from '@/components/speech/ModelDownloadPrompt.vue'
import { useLocaleStore } from '@/stores/locale'
import { useChatStore } from '@/stores/chat'

const { t } = useI18n()
const localeStore = useLocaleStore()
const chatStore = useChatStore()

export interface FileAttachment {
  id: string
  file: File
  name: string
  size: number
  type: string
  preview?: string
  duration?: number // audio duration in seconds
}

const props = defineProps<{
  disabled?: boolean
  streaming?: boolean
  maxFileSize?: number // in bytes, default 10MB
  allowedTypes?: string[] // MIME types
}>()

const emit = defineEmits<{
  send: [message: string, attachments: FileAttachment[]]
  inject: [message: string]
  cancel: []
  openTalkMode: []
  warmup: []
  'cancel-pre-ttft': []
}>()

const message = ref('')
const textareaRef = ref<HTMLTextAreaElement | null>(null)
const fileInputRef = ref<HTMLInputElement | null>(null)
const cameraInputRef = ref<HTMLInputElement | null>(null)
const attachments = ref<FileAttachment[]>([])
const dragOver = ref(false)

// Warmup: fire once per conversation when user starts typing
const warmupSent = ref(false)
// Pre-TTFT cancel: fire once per cancel cycle
const preTTFTCancelSent = ref(false)

// Compact mode state (for narrow screens, including non-mobile)
const isCompact = ref(false)
const isMobile = ref(false)
const showMobileMenu = ref(false)
const mobileMenuRef = ref<HTMLDivElement | null>(null)

// Voice mode state (compact: replace textarea with voice button)
const voiceMode = ref(false)
const isTouchDevice = ref(false)
const longPressTimer = ref<ReturnType<typeof setTimeout> | null>(null)

// Slide-to-cancel state
const touchStartY = ref(0)
const touchStartX = ref(0)
const slideCancelled = ref(false)
const swipeDirection = ref<'none' | 'left' | 'right'>('none') // left=send voice, right=transcribe
const CANCEL_SLIDE_THRESHOLD = 50 // px upward to cancel
const SWIPE_THRESHOLD = 40 // px horizontal to trigger swipe choice

// Image preview state
const showImagePreview = ref(false)
const previewImageSrc = ref('')
const previewImageAlt = ref('')

// Voice recording state
const isRecording = ref(false)
const isTranscribing = ref(false)
const recorder = ref<AudioRecorder | null>(null)
const voiceError = ref<string | null>(null)
const recordingStartTime = ref(0)
const recordingDuration = ref(0) // seconds
let recordingTimer: ReturnType<typeof setInterval> | null = null

// Voice choice panel (shown after recording stops)
const showVoiceChoice = ref(false)
const pendingAudioBlob = ref<Blob | null>(null)
const pendingAudioDuration = ref(0)

// Model download prompt state
const showASRDownloadPrompt = ref(false)
const _asrModelReady = ref(true) // Assume ready until checked

// Inline dictation (VAD-based tap-to-dictate)
const isDictating = ref(false)
let dictationVAD: EnergyVAD | null = null

const maxSize = computed(() => props.maxFileSize || 10 * 1024 * 1024) // 10MB default
const allowedMimeTypes = computed(() => props.allowedTypes || [
  // Images
  'image/*',
  // PDF
  'application/pdf',
  // Text files
  'text/plain',
  'text/markdown',
  'text/csv',
  'text/html',
  'text/xml',
  'text/rtf',
  // Code/Data files
  'application/json',
  'application/xml',
  'application/x-yaml',
  'text/yaml',
  'text/x-python',
  'text/javascript',
  'application/javascript',
  'text/x-java-source',
  'text/x-c',
  'text/x-c++',
  'text/x-go',
  'text/x-rust',
  'text/x-typescript',
  // Microsoft Office documents
  'application/msword', // .doc
  'application/vnd.openxmlformats-officedocument.wordprocessingml.document', // .docx
  'application/vnd.ms-excel', // .xls
  'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet', // .xlsx
  'application/vnd.ms-powerpoint', // .ppt
  'application/vnd.openxmlformats-officedocument.presentationml.presentation', // .pptx
  'application/rtf', // .rtf
  // OpenDocument formats
  'application/vnd.oasis.opendocument.text', // .odt
  'application/vnd.oasis.opendocument.spreadsheet', // .ods
  'application/vnd.oasis.opendocument.presentation', // .odp
  // Archives
  'application/zip',
  'application/x-zip-compressed',
  'application/x-rar-compressed',
  'application/x-7z-compressed',
  'application/gzip',
  'application/x-tar',
  // Audio
  'audio/*',
  // Video
  'video/*',
  // E-books
  'application/epub+zip', // .epub
])

const canSend = computed(() =>
  (message.value.trim().length > 0 || attachments.value.length > 0) && !props.disabled
)

// Shorter placeholder for mobile
const placeholder = computed(() =>
  isCompact.value ? t('chat.inputPlaceholderShort') : t('chat.inputPlaceholder')
)

function generateId(): string {
  return Math.random().toString(36).substring(2, 15)
}

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

function isAllowedType(file: File): boolean {
  return allowedMimeTypes.value.some(type => {
    if (type.endsWith('/*')) {
      const category = type.slice(0, -2)
      return file.type.startsWith(category)
    }
    return file.type === type
  })
}

async function createPreview(file: File): Promise<string | undefined> {
  if (file.type.startsWith('image/')) {
    return new Promise((resolve) => {
      const reader = new FileReader()
      reader.onload = (e) => resolve(e.target?.result as string)
      reader.onerror = () => resolve(undefined)
      reader.readAsDataURL(file)
    })
  }
  return undefined
}

async function addFiles(files: FileList | File[]) {
  const fileArray = Array.from(files)

  for (const file of fileArray) {
    // Check file size
    if (file.size > maxSize.value) {
      console.warn(`File ${file.name} exceeds maximum size of ${formatFileSize(maxSize.value)}`)
      continue
    }

    // Check file type
    if (!isAllowedType(file)) {
      console.warn(`File type ${file.type} is not allowed`)
      continue
    }

    // Check for duplicates
    if (attachments.value.some(a => a.name === file.name && a.size === file.size)) {
      continue
    }

    const preview = await createPreview(file)

    attachments.value.push({
      id: generateId(),
      file,
      name: file.name,
      size: file.size,
      type: file.type,
      preview,
    })
  }
}

function removeAttachment(id: string) {
  attachments.value = attachments.value.filter(a => a.id !== id)
}

function handleFileSelect(event: Event) {
  const input = event.target as HTMLInputElement
  if (input.files) {
    addFiles(input.files)
    input.value = '' // Reset input
  }
}

function handleDragOver(event: DragEvent) {
  event.preventDefault()
  dragOver.value = true
}

function handleDragLeave() {
  dragOver.value = false
}

function handleDrop(event: DragEvent) {
  event.preventDefault()
  dragOver.value = false

  if (event.dataTransfer?.files) {
    addFiles(event.dataTransfer.files)
  }
}

function handlePaste(event: ClipboardEvent) {
  const items = event.clipboardData?.items
  if (!items) return

  const files: File[] = []
  for (const item of items) {
    if (item.kind === 'file') {
      const file = item.getAsFile()
      if (file) files.push(file)
    }
  }

  if (files.length > 0) {
    addFiles(files)
  }
}

function openFileDialog() {
  fileInputRef.value?.click()
}

function openCameraDialog() {
  cameraInputRef.value?.click()
}

function handleCameraCapture(event: Event) {
  const input = event.target as HTMLInputElement
  if (input.files) {
    addFiles(input.files)
    input.value = '' // Reset input
  }
}

function openImagePreview(attachment: FileAttachment) {
  if (attachment.preview) {
    previewImageSrc.value = attachment.preview
    previewImageAlt.value = attachment.name
    showImagePreview.value = true
  }
}

function handleSend() {
  if (!canSend.value) return

  if (props.streaming) {
    // Mid-stream injection: send message without interrupting attachments
    emit('inject', message.value.trim())
  } else {
    emit('send', message.value.trim(), [...attachments.value])
    attachments.value = []
  }
  message.value = ''
  warmupSent.value = false // Reset so next typing triggers warmup again
  preTTFTCancelSent.value = false

  // Reset textarea height
  if (textareaRef.value) {
    textareaRef.value.style.height = 'auto'
  }
}

function clearMessage() {
  message.value = ''
  if (textareaRef.value) {
    textareaRef.value.style.height = 'auto'
    textareaRef.value.focus()
  }
}

function handleCancel() {
  emit('cancel')
}

function handleKeydown(event: KeyboardEvent) {
  // Skip Enter during IME composition (e.g. Chinese input confirming character selection)
  if (event.isComposing || event.keyCode === 229) return

  // Send on Enter (without Shift)
  if (event.key === 'Enter' && !event.shiftKey) {
    event.preventDefault()
    handleSend()
  }
}

function handleInput() {
  // Detect typing during pre-TTFT wait → cancel and enter "listening" mode
  if (chatStore.isPreTTFT && !preTTFTCancelSent.value && message.value.length > 0) {
    preTTFTCancelSent.value = true
    emit('cancel-pre-ttft')
  }

  // Trigger warmup on first input (pre-compute system prompt to reduce TTFT)
  if (!warmupSent.value && message.value.length > 0) {
    warmupSent.value = true
    emit('warmup')
  }

  // Auto-resize textarea
  if (textareaRef.value) {
    textareaRef.value.style.height = 'auto'
    textareaRef.value.style.height = `${Math.min(textareaRef.value.scrollHeight, 200)}px`
  }
}

function getFileIcon(type: string): string {
  if (type.startsWith('image/')) return 'image'
  if (type === 'application/pdf') return 'pdf'
  if (type.startsWith('text/')) return 'text'
  return 'file'
}

// Voice recording functions
async function checkASRModelReady(): Promise<boolean> {
  try {
    const res = await speechApi.getStatus()
    // Check for macOS permission denied
    if (res.data?.asr?.permission_denied) {
      const appName = res.data.asr.permission_app_name || 'Terminal'
      voiceError.value = t('speech.macosNativePermissionToast', { appName })
      return false
    }
    return res.data?.asr?.ready ?? false
  } catch {
    return true // Assume ready if check fails
  }
}

async function startRecording() {
  if (props.disabled || props.streaming) return

  // Check if ASR model is ready
  const ready = await checkASRModelReady()
  if (!ready) {
    // If voiceError was set (e.g. permission denied), don't show download prompt
    if (!voiceError.value) {
      showASRDownloadPrompt.value = true
    }
    return
  }

  voiceError.value = null
  recorder.value = new AudioRecorder()

  // Trigger warmup when voice recording starts
  if (!warmupSent.value) {
    warmupSent.value = true
    emit('warmup')
  }

  recorder.value.onStop = async (audioBlob: Blob) => {
    isRecording.value = false
    isTranscribing.value = true

    try {
      // Convert webm to wav for whisper.cpp
      const wavBlob = await convertToWav(audioBlob)
      if (!wavBlob) {
        isTranscribing.value = false
        return
      }

      // Transcribe audio with user's locale language
      const lang = localeStore.currentLocale.split('-')[0]
      const response = await voiceApi.transcribe(wavBlob, 'wav', lang)
      if (response.data.text) {
        // In compact voice mode, exit voice mode so textarea becomes visible
        if (isCompact.value && voiceMode.value) {
          voiceMode.value = false
        }
        // Append transcribed text to message textarea
        if (message.value.trim()) {
          message.value += ' ' + response.data.text
        } else {
          message.value = response.data.text
        }
        handleInput()
        // Focus textarea so user can edit or press Enter to send
        nextTick(() => textareaRef.value?.focus())
      }
    } catch (error: any) {
      console.error('Transcription error:', error)
      const errorCode = error?.response?.data?.error_code
      if (error?.code === 'ECONNABORTED' || error?.message?.includes('timeout')) {
        voiceError.value = t('chat.voiceTranscriptionTimeout')
      } else if (errorCode === 'on_device_unavailable') {
        voiceError.value = t('speech.onDeviceUnavailableError')
      } else if (errorCode && t(`speech.error.${errorCode}`) !== `speech.error.${errorCode}`) {
        voiceError.value = t(`speech.error.${errorCode}`)
      } else {
        const serverMsg = error?.response?.data?.error || error?.response?.data?.message || error?.message
        voiceError.value = serverMsg || t('chat.voiceTranscriptionError')
      }
    } finally {
      isTranscribing.value = false
      recorder.value = null
    }
  }

  recorder.value.onError = (error: Error) => {
    console.error('Recording error:', error)
    voiceError.value = t('chat.voiceRecordingError')
    isRecording.value = false
    recorder.value = null
  }

  try {
    await recorder.value.start()
    isRecording.value = true
    recordingStartTime.value = Date.now()
    recordingDuration.value = 0
    recordingTimer = setInterval(() => {
      recordingDuration.value = Math.floor((Date.now() - recordingStartTime.value) / 1000)
    }, 1000)
  } catch (error) {
    console.error('Failed to start recording:', error)
    voiceError.value = t('chat.voiceMicrophoneError')
    recorder.value = null
  }
}

function stopRecording() {
  if (recorder.value && isRecording.value) {
    recorder.value.stop()
  }
  if (recordingTimer) {
    clearInterval(recordingTimer)
    recordingTimer = null
  }
}

// Send audio blob as a voice message attachment
function sendVoiceMessage(audioBlob: Blob, durationSec: number) {
  const mimeType = audioBlob.type || 'audio/webm'
  const ext = mimeType.includes('ogg') ? 'ogg' : mimeType.includes('mp4') ? 'm4a' : 'webm'
  const fileName = `voice_${Date.now()}.${ext}`
  const file = new File([audioBlob], fileName, { type: mimeType })

  const reader = new FileReader()
  reader.onloadend = () => {
    const base64 = (reader.result as string).split(',')[1] || ''
    if (!base64) return

    const voiceAttachment: FileAttachment = {
      id: `voice-${Date.now()}`,
      file,
      name: fileName,
      size: audioBlob.size,
      type: mimeType,
      duration: Math.max(1, durationSec),
    }
    emit('send', '', [voiceAttachment])
  }
  reader.readAsDataURL(audioBlob)
}

function toggleRecording() {
  if (isRecording.value) {
    stopRecording()
  } else {
    startRecording()
  }
}

// Voice mode toggle (compact only)
async function toggleVoiceMode() {
  if (!voiceMode.value) {
    // Pre-request mic permission so the browser dialog doesn't interrupt long-press
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true })
      stream.getTracks().forEach(t => t.stop())
    } catch {
      voiceError.value = t('chat.voiceMicrophoneError')
      return
    }
  }
  voiceMode.value = !voiceMode.value
}

// Touch handlers for hold-to-speak recording (mobile)
function handleVoiceTouchStart(e: TouchEvent) {
  if (!isTouchDevice.value || props.disabled || props.streaming || isTranscribing.value) return
  touchStartY.value = e.touches[0].clientY
  touchStartX.value = e.touches[0].clientX
  slideCancelled.value = false
  swipeDirection.value = 'none'
  // Start recording immediately — no delay
  startRecording()
  // Haptic feedback
  if (navigator.vibrate) navigator.vibrate(10)
}

function handleVoiceTouchMove(e: TouchEvent) {
  if (!isTouchDevice.value || !isRecording.value) return
  const dy = touchStartY.value - e.touches[0].clientY
  const dx = e.touches[0].clientX - touchStartX.value

  // Vertical: slide up to cancel
  if (dy > CANCEL_SLIDE_THRESHOLD) {
    slideCancelled.value = true
    swipeDirection.value = 'none'
    return
  } else {
    slideCancelled.value = false
  }

  // Horizontal: swipe to choose mode
  if (dx < -SWIPE_THRESHOLD) {
    swipeDirection.value = 'left' // Send voice
  } else if (dx > SWIPE_THRESHOLD) {
    swipeDirection.value = 'right' // Transcribe to text
  } else {
    swipeDirection.value = 'none'
  }
}

function handleVoiceTouchEnd() {
  if (!isTouchDevice.value) return
  if (longPressTimer.value) {
    clearTimeout(longPressTimer.value)
    longPressTimer.value = null
  }
  if (isRecording.value) {
    const duration = Math.max(1, Math.round((Date.now() - recordingStartTime.value) / 1000))
    if (recordingTimer) {
      clearInterval(recordingTimer)
      recordingTimer = null
    }

    if (slideCancelled.value) {
      // Swipe up — discard recording
      if (recorder.value) {
        recorder.value.onStop = null as any
        recorder.value.stop()
      }
      isRecording.value = false
      recorder.value = null
    } else if (duration < 1) {
      // Too short — discard
      if (recorder.value) {
        recorder.value.onStop = null as any
        recorder.value.stop()
      }
      isRecording.value = false
      recorder.value = null
      voiceError.value = t('chat.voiceMessageTooShort')
      setTimeout(() => { if (voiceError.value === t('chat.voiceMessageTooShort')) voiceError.value = null }, 2000)
    } else {
      // Normal release — stop recording and show choice panel
      pendingAudioDuration.value = duration
      if (recorder.value) {
        recorder.value.onStop = (audioBlob: Blob) => {
          isRecording.value = false
          pendingAudioBlob.value = audioBlob
          showVoiceChoice.value = true
          recorder.value = null
        }
        recorder.value.stop()
      }
    }
  }
  slideCancelled.value = false
  swipeDirection.value = 'none'
}

// Voice choice panel actions
function handleVoiceChoiceSend() {
  if (pendingAudioBlob.value) {
    sendVoiceMessage(pendingAudioBlob.value, pendingAudioDuration.value)
  }
  dismissVoiceChoice()
}

async function handleVoiceChoiceTranscribe() {
  if (!pendingAudioBlob.value) { dismissVoiceChoice(); return }

  const audioBlob = pendingAudioBlob.value
  dismissVoiceChoice()
  isTranscribing.value = true

  try {
    const wavBlob = await convertToWav(audioBlob)
    if (!wavBlob) {
      isTranscribing.value = false
      return
    }
    const lang = localeStore.currentLocale.split('-')[0]
    const response = await voiceApi.transcribe(wavBlob, 'wav', lang)
    if (response.data.text) {
      if (isCompact.value && voiceMode.value) {
        voiceMode.value = false
      }
      if (message.value.trim()) {
        message.value += ' ' + response.data.text
      } else {
        message.value = response.data.text
      }
      handleInput()
      nextTick(() => textareaRef.value?.focus())
    }
  } catch (error: any) {
    console.error('Transcription error:', error)
    const errorCode = error?.response?.data?.error_code
    if (error?.code === 'ECONNABORTED' || error?.message?.includes('timeout')) {
      voiceError.value = t('chat.voiceTranscriptionTimeout')
    } else if (errorCode === 'on_device_unavailable') {
      voiceError.value = t('speech.onDeviceUnavailableError')
    } else if (errorCode && t(`speech.error.${errorCode}`) !== `speech.error.${errorCode}`) {
      voiceError.value = t(`speech.error.${errorCode}`)
    } else {
      const serverMsg = error?.response?.data?.error || error?.response?.data?.message || error?.message
      voiceError.value = serverMsg || t('chat.voiceTranscriptionError')
    }
  } finally {
    isTranscribing.value = false
  }
}

function dismissVoiceChoice() {
  showVoiceChoice.value = false
  pendingAudioBlob.value = null
  pendingAudioDuration.value = 0
}

function handleVoiceTouchCancel() {
  if (longPressTimer.value) {
    clearTimeout(longPressTimer.value)
    longPressTimer.value = null
  }
  if (isRecording.value) {
    // Cancel — discard recording
    if (recorder.value) {
      recorder.value.onStop = null as any
      recorder.value.stop()
    }
    isRecording.value = false
    recorder.value = null
  }
  slideCancelled.value = false
}

// Click handler for PC voice recording (non-touch)
function handleVoiceClick() {
  if (isTouchDevice.value) return
  toggleRecording()
}

// Inline dictation — tap mic icon to start VAD-based speech-to-text
async function toggleDictation() {
  if (isDictating.value) {
    stopDictation()
    return
  }

  // Check mic permission
  try {
    const stream = await navigator.mediaDevices.getUserMedia({ audio: true })
    stream.getTracks().forEach(t => t.stop())
  } catch {
    voiceError.value = t('chat.voiceMicrophoneError')
    return
  }

  const lang = localeStore.currentLocale.split('-')[0]

  dictationVAD = new EnergyVAD({
    speechThreshold: 0.015,
    silenceThreshold: 0.01,
    silenceDuration: 1200,
    minSpeechDuration: 300,
    onSpeechEnd: async (audioBlob: Blob) => {
      isDictating.value = false
      isTranscribing.value = true

      try {
        const wavBlob = await convertToWav(audioBlob)
        if (!wavBlob) return
        const result = await speechApi.transcribe(wavBlob, 'wav', lang)
        if (result.text) {
          // Insert at cursor position or append
          if (message.value) {
            message.value += ' ' + result.text
          } else {
            message.value = result.text
          }
          handleInput()
          nextTick(() => textareaRef.value?.focus())
        }
      } catch (err: any) {
        console.error('Dictation transcription error:', err)
        voiceError.value = t('chat.voiceTranscriptionError')
      } finally {
        isTranscribing.value = false
        stopDictation()
      }
    },
  })

  try {
    await dictationVAD.start()
    isDictating.value = true
  } catch {
    voiceError.value = t('chat.voiceMicrophoneError')
  }
}

function stopDictation() {
  if (dictationVAD) {
    dictationVAD.destroy()
    dictationVAD = null
  }
  isDictating.value = false
}

// Cleanup on unmount
onUnmounted(() => {
  if (recorder.value) {
    recorder.value.stop()
  }
  if (longPressTimer.value) {
    clearTimeout(longPressTimer.value)
  }
  if (recordingTimer) {
    clearInterval(recordingTimer)
    recordingTimer = null
  }
  dismissVoiceChoice()
  stopDictation()
  window.removeEventListener('resize', checkMobile)
  document.removeEventListener('click', handleClickOutside)
})

// Check if compact mode (narrow screen) and mobile device (UA)
function checkMobile() {
  isCompact.value = window.innerWidth < 768
  const ua = navigator.userAgent.toLowerCase()
  isMobile.value = /android|webos|iphone|ipad|ipod|blackberry|iemobile|opera mini/i.test(ua)
  if (!isCompact.value) {
    showMobileMenu.value = false
  }
}

// Handle click outside mobile menu
function handleClickOutside(event: MouseEvent) {
  if (mobileMenuRef.value && !mobileMenuRef.value.contains(event.target as Node)) {
    showMobileMenu.value = false
  }
}

// Toggle mobile menu
function toggleMobileMenu() {
  showMobileMenu.value = !showMobileMenu.value
}

// Mobile menu action handlers
function handleMobileAttachment() {
  showMobileMenu.value = false
  openFileDialog()
}

function handleMobileCamera() {
  showMobileMenu.value = false
  openCameraDialog()
}

function handleMobileTalkMode() {
  showMobileMenu.value = false
  emit('openTalkMode')
}

onMounted(() => {
  checkMobile()
  isTouchDevice.value = 'ontouchstart' in window || navigator.maxTouchPoints > 0
  window.addEventListener('resize', checkMobile)
  document.addEventListener('click', handleClickOutside)
})

function focus() {
  textareaRef.value?.focus()
}

function setInput(text: string) {
  message.value = text
  nextTick(() => {
    if (textareaRef.value) {
      textareaRef.value.style.height = 'auto'
      textareaRef.value.style.height = `${Math.min(textareaRef.value.scrollHeight, 200)}px`
    }
  })
}

function resetWarmup() {
  warmupSent.value = false
  preTTFTCancelSent.value = false
}

defineExpose({ focus, setInput, handleDragOver, handleDragLeave, handleDrop, resetWarmup })
</script>

<template>
  <div
    class="chat-input-wrapper"
    :class="isMobile ? 'px-0 pb-0 pt-0' : (isCompact ? 'px-0 pb-0 pt-0' : 'px-3 sm:px-4 pb-3 sm:pb-4 pt-2')"
  >
    <div
      class="chat-input-container p-3 sm:p-4 max-w-4xl mx-auto"
      :class="[
        isMobile
          ? 'border-t border-gray-200 dark:border-glass-border bg-white dark:bg-gray-800/80'
          : (isCompact
            ? 'border-t border-gray-200 dark:border-glass-border bg-white dark:bg-gray-800/80'
            : 'glass-card shadow-lg rounded-2xl'),
        { 'ring-2 ring-accent': dragOver }
      ]"
    >
    <!-- Hidden file input -->
    <input
      ref="fileInputRef"
      type="file"
      multiple
      class="hidden"
      :accept="allowedMimeTypes.join(',')"
      @change="handleFileSelect"
    />

    <!-- Hidden camera input -->
    <input
      ref="cameraInputRef"
      type="file"
      accept="image/*"
      capture="environment"
      class="hidden"
      @change="handleCameraCapture"
    />

    <!-- Image Preview Modal -->
    <ImagePreview
      v-model="showImagePreview"
      :src="previewImageSrc"
      :alt="previewImageAlt"
    />

    <!-- ASR Model Download Prompt -->
    <ModelDownloadPrompt
      v-model:model-visible="showASRDownloadPrompt"
      type="asr"
      @downloaded="startRecording"
    />

    <!-- Attachments preview -->
    <div v-if="attachments.length > 0" class="mb-3 flex flex-wrap gap-2">
      <div
        v-for="attachment in attachments"
        :key="attachment.id"
        class="relative group glass-card p-2 flex items-center gap-2 max-w-xs"
      >
        <!-- Preview or icon -->
        <div
          class="w-10 h-10 flex-shrink-0 rounded-lg overflow-hidden bg-surface-card flex items-center justify-center"
          :class="{ 'cursor-pointer': attachment.preview }"
          @click="attachment.preview && openImagePreview(attachment)"
        >
          <img
            v-if="attachment.preview"
            :src="attachment.preview"
            :alt="attachment.name"
            class="w-full h-full object-cover hover:opacity-80 transition-opacity"
          />
          <svg
            v-else-if="getFileIcon(attachment.type) === 'pdf'"
            class="w-6 h-6 text-red-400"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 21h10a2 2 0 002-2V9.414a1 1 0 00-.293-.707l-5.414-5.414A1 1 0 0012.586 3H7a2 2 0 00-2 2v14a2 2 0 002 2z" />
          </svg>
          <svg
            v-else-if="getFileIcon(attachment.type) === 'text'"
            class="w-6 h-6 text-gray-900 dark:text-gray-300"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
          </svg>
          <svg
            v-else
            class="w-6 h-6 text-slate-400"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.172 7l-6.586 6.586a2 2 0 102.828 2.828l6.414-6.586a4 4 0 00-5.656-5.656l-6.415 6.585a6 6 0 108.486 8.486L20.5 13" />
          </svg>
        </div>

        <!-- File info -->
        <div class="flex-1 min-w-0">
          <p class="text-sm text-gray-900 dark:text-white truncate">{{ attachment.name }}</p>
          <p class="text-xs text-gray-500 dark:text-slate-400">{{ formatFileSize(attachment.size) }}</p>
        </div>

        <!-- Remove button -->
        <button
          class="absolute -top-1 -right-1 w-5 h-5 bg-red-500 hover:bg-red-600 rounded-full flex items-center justify-center opacity-0 group-hover:opacity-100 transition-all duration-200 cursor-pointer"
          @click="removeAttachment(attachment.id)"
        >
          <svg class="w-3 h-3 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
    </div>

    <div class="flex items-center gap-2 sm:gap-3">
      <!-- Mobile: WeChat-style layout -->
      <template v-if="isCompact">
        <!-- Left: Voice/Text toggle button -->
        <button
          :disabled="disabled || streaming"
          class="flex-shrink-0 w-10 h-10 rounded-xl glass-card text-gray-500 dark:text-slate-300 hover:text-gray-900 dark:hover:text-white hover:bg-gray-100 dark:hover:bg-white/10 flex items-center justify-center transition-all duration-200 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
          :title="voiceMode ? t('chat.switchToKeyboard') : t('chat.switchToVoice')"
          @click="toggleVoiceMode"
        >
          <!-- Keyboard icon when in voice mode -->
          <svg v-if="voiceMode" xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <rect x="2" y="4" width="20" height="16" rx="2" stroke-width="2" />
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 8h.01M10 8h.01M14 8h.01M18 8h.01M6 12h.01M10 12h.01M14 12h.01M18 12h.01M8 16h8" />
          </svg>
          <!-- Mic icon when in text mode -->
          <svg v-else xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11a7 7 0 01-7 7m0 0a7 7 0 01-7-7m7 7v4m0 0H8m4 0h4m-4-8a3 3 0 01-3-3V5a3 3 0 116 0v6a3 3 0 01-3 3z" />
          </svg>
        </button>

        <!-- Center: Voice hold-to-speak or Textarea -->
        <div v-if="voiceMode" class="flex-1 relative">
          <!-- Voice choice panel (shown after recording) -->
          <div v-if="showVoiceChoice" class="voice-choice-panel flex items-center gap-2 w-full">
            <button
              class="flex-1 h-10 rounded-xl flex items-center justify-center gap-1.5 bg-blue-500/15 text-blue-500 border border-blue-500/30 hover:bg-blue-500/25 active:bg-blue-500/35 transition-all cursor-pointer"
              @click="handleVoiceChoiceTranscribe"
            >
              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
              </svg>
              <span class="text-sm font-medium">{{ t('chat.voiceToText') }}</span>
            </button>
            <button
              class="flex-1 h-10 rounded-xl flex items-center justify-center gap-1.5 bg-green-500/15 text-green-500 border border-green-500/30 hover:bg-green-500/25 active:bg-green-500/35 transition-all cursor-pointer"
              @click="handleVoiceChoiceSend"
            >
              <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
                <path d="M3 20l1.3-4.8C3.5 13.8 3 12.4 3 11c0-5 4-9 9-9s9 4 9 9-4 9-9 9c-1.4 0-2.8-.5-4.2-1.2L3 20z"/>
                <path d="M9 10h6M9 14h4" fill="none" stroke="white" stroke-width="1.5" stroke-linecap="round"/>
              </svg>
              <span class="text-sm font-medium">{{ pendingAudioDuration }}s · {{ t('chat.sendVoice') }}</span>
            </button>
            <button
              class="flex-shrink-0 w-8 h-8 rounded-full flex items-center justify-center text-gray-400 hover:text-gray-600 dark:hover:text-gray-200 hover:bg-gray-200/50 dark:hover:bg-gray-600/50 transition-colors cursor-pointer"
              @click="dismissVoiceChoice"
            >
              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
                <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
          <!-- Hold-to-speak button -->
          <button
            v-else
            :disabled="disabled || streaming || isTranscribing"
            class="w-full h-10 rounded-xl flex items-center justify-center gap-2 transition-all duration-200 select-none cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
            :class="isRecording
              ? (slideCancelled
                ? 'bg-gray-500/20 text-gray-400 border border-gray-500/30'
                : 'bg-green-500/20 text-green-500 border border-green-500/30 voice-breathing')
              : isTranscribing
                ? 'glass-card text-gray-500 dark:text-slate-300'
                : 'glass-card text-gray-500 dark:text-slate-300 hover:text-gray-900 dark:hover:text-white hover:bg-gray-100 dark:hover:bg-white/10 active:bg-gray-200 dark:active:bg-white/20'"
            @touchstart.prevent="handleVoiceTouchStart"
            @touchmove.prevent="handleVoiceTouchMove"
            @touchend.prevent="handleVoiceTouchEnd"
            @touchcancel="handleVoiceTouchCancel"
            @click="handleVoiceClick"
          >
            <svg v-if="isTranscribing" class="h-5 w-5 animate-spin" fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>
            <svg v-else xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11a7 7 0 01-7 7m0 0a7 7 0 01-7-7m7 7v4m0 0H8m4 0h4m-4-8a3 3 0 01-3-3V5a3 3 0 116 0v6a3 3 0 01-3 3z" />
            </svg>
            <span class="text-sm">
              <template v-if="isTranscribing">{{ t('chat.voiceTranscribing') }}</template>
              <template v-else-if="isRecording && slideCancelled">{{ t('chat.releaseToCancel') }}</template>
              <template v-else-if="isRecording">{{ recordingDuration }}s · {{ t('chat.recording') }}</template>
              <template v-else>{{ t('chat.holdToSpeak') }}</template>
            </span>
          </button>
        </div>
        <div v-else class="flex-1 relative">
          <textarea
            ref="textareaRef"
            v-model="message"
            :disabled="disabled"
            :placeholder="placeholder"
            enterkeyhint="send"
            class="chat-textarea w-full glass-input text-gray-900 dark:text-white px-3 pr-14 resize-none disabled:opacity-50 disabled:cursor-not-allowed"
            rows="1"
            @keydown="handleKeydown"
            @input="handleInput"
            @paste="handlePaste"
          />
          <!-- Inline mic icon for tap-to-dictate (mobile text mode) -->
          <button
            v-if="!isTranscribing"
            class="absolute top-1/2 -translate-y-1/2 w-5 h-5 flex items-center justify-center rounded-full transition-all duration-200 cursor-pointer"
            :class="isDictating
              ? 'right-8 text-green-500 dictation-glow'
              : (message.length > 0 ? 'right-8' : 'right-2') + ' text-gray-400 hover:text-gray-600 dark:hover:text-gray-200'"
            :title="isDictating ? t('chat.stopDictation') : t('chat.startDictation')"
            @click="toggleDictation"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M19 11a7 7 0 01-7 7m0 0a7 7 0 01-7-7m7 7v4m0 0H8m4 0h4m-4-8a3 3 0 01-3-3V5a3 3 0 116 0v6a3 3 0 01-3 3z" />
            </svg>
          </button>
          <button
            v-if="message.length > 0"
            class="absolute right-2 top-1/2 -translate-y-1/2 w-5 h-5 flex items-center justify-center rounded-full text-gray-400 hover:text-gray-600 dark:hover:text-gray-200 hover:bg-gray-200/50 dark:hover:bg-gray-600/50 transition-colors cursor-pointer"
            title="Clear"
            @click="clearMessage"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
              <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        <!-- PC Narrow: Send button (between textarea and + button) -->
        <button
          v-if="isCompact && !isMobile"
          :disabled="!canSend"
          class="flex-shrink-0 w-10 h-10 rounded-xl bg-gradient-to-r from-purple-500 to-pink-500 text-white flex items-center justify-center transition-all duration-200 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer hover:shadow-glow"
          :title="t('chat.send')"
          @click="handleSend"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 19l9 2-9-18-9 18 9-2zm0 0v-8" />
          </svg>
        </button>

        <!-- Right: + button for extensions (or Cancel during streaming) -->
        <button
          v-if="streaming"
          class="flex-shrink-0 w-10 h-10 rounded-xl bg-red-500/20 hover:bg-red-500/30 text-red-400 hover:text-red-300 border border-red-500/30 flex items-center justify-center transition-all duration-200 cursor-pointer"
          :title="t('common.cancel')"
          @click="handleCancel"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
        <div v-else ref="mobileMenuRef" class="relative">
          <button
            :disabled="disabled"
            class="flex-shrink-0 w-10 h-10 rounded-xl glass-card text-gray-500 dark:text-slate-300 hover:text-gray-900 dark:hover:text-white hover:bg-gray-100 dark:hover:bg-white/10 flex items-center justify-center transition-all duration-200 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
            :class="{ 'bg-gray-200 dark:bg-gray-600/20 text-gray-900 dark:text-gray-300': showMobileMenu }"
            :title="t('chat.moreActions')"
            @click.stop="toggleMobileMenu"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M12 4v16m8-8H4" />
            </svg>
          </button>

          <!-- Mobile extension menu dropdown -->
          <Transition
            enter-active-class="transition ease-out duration-200"
            enter-from-class="opacity-0 translate-y-2"
            enter-to-class="opacity-100 translate-y-0"
            leave-active-class="transition ease-in duration-150"
            leave-from-class="opacity-100 translate-y-0"
            leave-to-class="opacity-0 translate-y-2"
          >
            <div
              v-if="showMobileMenu"
              class="absolute bottom-full right-0 mb-2 w-48 glass-card rounded-xl shadow-lg border border-white/10 overflow-hidden z-50"
            >
              <button
                :disabled="disabled || streaming"
                class="w-full px-4 py-3 flex items-center gap-3 text-gray-700 dark:text-slate-200 hover:bg-gray-100 dark:hover:bg-white/10 transition-colors disabled:opacity-50"
                @click="handleMobileAttachment"
              >
                <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.172 7l-6.586 6.586a2 2 0 102.828 2.828l6.414-6.586a4 4 0 00-5.656-5.656l-6.415 6.585a6 6 0 108.486 8.486L20.5 13" />
                </svg>
                <span class="text-sm">{{ t('chat.attachFile') }}</span>
              </button>

              <button
                :disabled="disabled || streaming"
                class="w-full px-4 py-3 flex items-center gap-3 text-gray-700 dark:text-slate-200 hover:bg-gray-100 dark:hover:bg-white/10 transition-colors disabled:opacity-50"
                @click="handleMobileCamera"
              >
                <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 9a2 2 0 012-2h.93a2 2 0 001.664-.89l.812-1.22A2 2 0 0110.07 4h3.86a2 2 0 011.664.89l.812 1.22A2 2 0 0018.07 7H19a2 2 0 012 2v9a2 2 0 01-2 2H5a2 2 0 01-2-2V9z" />
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 13a3 3 0 11-6 0 3 3 0 016 0z" />
                </svg>
                <span class="text-sm">{{ t('chat.takePhoto') }}</span>
              </button>

              <button
                :disabled="disabled || streaming"
                class="w-full px-4 py-3 flex items-center gap-3 text-gray-700 dark:text-slate-200 hover:bg-gray-100 dark:hover:bg-white/10 transition-colors disabled:opacity-50"
                @click="handleMobileTalkMode"
              >
                <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 5a2 2 0 012-2h3.28a1 1 0 01.948.684l1.498 4.493a1 1 0 01-.502 1.21l-2.257 1.13a11.042 11.042 0 005.516 5.516l1.13-2.257a1 1 0 011.21-.502l4.493 1.498a1 1 0 01.684.949V19a2 2 0 01-2 2h-1C9.716 21 3 14.284 3 6V5z" />
                </svg>
                <span class="text-sm">{{ t('chat.talkMode.title') }}</span>
              </button>
            </div>
          </Transition>
        </div>
      </template>

      <!-- Desktop: Individual buttons -->
      <!-- Attachment button -->
      <button
        v-if="!isCompact"
        :disabled="disabled || streaming"
        class="flex-shrink-0 w-10 h-10 rounded-xl glass-card text-gray-500 dark:text-slate-300 hover:text-gray-900 dark:hover:text-white hover:bg-gray-100 dark:hover:bg-white/10 flex items-center justify-center transition-all duration-200 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
        :title="t('chat.attachFile')"
        @click="openFileDialog"
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="h-5 w-5 sm:h-6 sm:w-6"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M15.172 7l-6.586 6.586a2 2 0 102.828 2.828l6.414-6.586a4 4 0 00-5.656-5.656l-6.415 6.585a6 6 0 108.486 8.486L20.5 13"
          />
        </svg>
      </button>

      <!-- Camera button (desktop) -->
      <button
        v-if="!isCompact"
        :disabled="disabled || streaming"
        class="flex-shrink-0 w-10 h-10 rounded-xl glass-card text-gray-500 dark:text-slate-300 hover:text-gray-900 dark:hover:text-white hover:bg-gray-100 dark:hover:bg-white/10 flex items-center justify-center transition-all duration-200 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
        :title="t('chat.takePhoto')"
        @click="openCameraDialog"
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="h-5 w-5 sm:h-6 sm:w-6"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M3 9a2 2 0 012-2h.93a2 2 0 001.664-.89l.812-1.22A2 2 0 0110.07 4h3.86a2 2 0 011.664.89l.812 1.22A2 2 0 0018.07 7H19a2 2 0 012 2v9a2 2 0 01-2 2H5a2 2 0 01-2-2V9z"
          />
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M15 13a3 3 0 11-6 0 3 3 0 016 0z"
          />
        </svg>
      </button>

      <!-- Voice input button (desktop) -->
      <button
        v-if="!isCompact"
        :disabled="disabled || streaming || isTranscribing"
        class="flex-shrink-0 w-10 h-10 rounded-xl glass-card flex items-center justify-center transition-all duration-200 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
        :class="isRecording
          ? 'bg-red-500/20 text-red-400 border border-red-500/30 animate-pulse'
          : 'text-gray-500 dark:text-slate-300 hover:text-gray-900 dark:hover:text-white hover:bg-gray-100 dark:hover:bg-white/10'"
        :title="isRecording ? t('chat.stopRecording') : t('chat.startRecording')"
        @click="toggleRecording"
      >
        <svg
          v-if="isTranscribing"
          class="h-5 w-5 sm:h-6 sm:w-6 animate-spin"
          xmlns="http://www.w3.org/2000/svg"
          fill="none"
          viewBox="0 0 24 24"
        >
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
        </svg>
        <svg
          v-else
          xmlns="http://www.w3.org/2000/svg"
          class="h-5 w-5 sm:h-6 sm:w-6"
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

      <!-- Talk Mode button (desktop) -->
      <button
        v-if="!isCompact"
        :disabled="disabled || streaming"
        class="flex-shrink-0 w-10 h-10 rounded-xl glass-card text-gray-500 dark:text-slate-300 hover:text-gray-900 dark:hover:text-white hover:bg-gray-100 dark:hover:bg-white/10 flex items-center justify-center transition-all duration-200 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
        :title="t('chat.talkMode.title')"
        @click="$emit('openTalkMode')"
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="h-5 w-5 sm:h-6 sm:w-6"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M3 5a2 2 0 012-2h3.28a1 1 0 01.948.684l1.498 4.493a1 1 0 01-.502 1.21l-2.257 1.13a11.042 11.042 0 005.516 5.516l1.13-2.257a1 1 0 011.21-.502l4.493 1.498a1 1 0 01.684.949V19a2 2 0 01-2 2h-1C9.716 21 3 14.284 3 6V5z"
          />
        </svg>
      </button>

      <!-- Desktop: Normal textarea -->
      <div v-if="!isCompact" class="flex-1 relative">
        <textarea
          ref="textareaRef"
          v-model="message"
          :disabled="disabled"
          :placeholder="placeholder"
          enterkeyhint="send"
          class="chat-textarea w-full glass-input text-gray-900 dark:text-white px-3 pr-16 resize-none disabled:opacity-50 disabled:cursor-not-allowed"
          rows="1"
          @keydown="handleKeydown"
          @input="handleInput"
          @paste="handlePaste"
        />
        <!-- Inline mic icon for tap-to-dictate -->
        <button
          v-if="!isTranscribing"
          class="absolute top-1/2 -translate-y-1/2 w-5 h-5 flex items-center justify-center rounded-full transition-all duration-200 cursor-pointer"
          :class="isDictating
            ? 'right-8 text-green-500 dictation-glow'
            : (message.length > 0 ? 'right-8' : 'right-2') + ' text-gray-400 hover:text-gray-600 dark:hover:text-gray-200'"
          :title="isDictating ? t('chat.stopDictation') : t('chat.startDictation')"
          @click="toggleDictation"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M19 11a7 7 0 01-7 7m0 0a7 7 0 01-7-7m7 7v4m0 0H8m4 0h4m-4-8a3 3 0 01-3-3V5a3 3 0 116 0v6a3 3 0 01-3 3z" />
          </svg>
        </button>
        <!-- Transcribing spinner -->
        <div
          v-else
          class="absolute right-8 top-1/2 -translate-y-1/2 w-5 h-5 flex items-center justify-center"
        >
          <svg class="h-3.5 w-3.5 animate-spin text-gray-400" fill="none" viewBox="0 0 24 24">
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
          </svg>
        </div>
        <button
          v-if="message.length > 0"
          class="absolute right-2 top-1/2 -translate-y-1/2 w-5 h-5 flex items-center justify-center rounded-full text-gray-400 hover:text-gray-600 dark:hover:text-gray-200 hover:bg-gray-200/50 dark:hover:bg-gray-600/50 transition-colors cursor-pointer"
          title="Clear"
          @click="clearMessage"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>

      <!-- Desktop: Cancel button (during streaming) -->
      <button
        v-if="!isCompact && streaming"
        class="flex-shrink-0 w-10 h-10 rounded-xl bg-red-500/20 hover:bg-red-500/30 text-red-400 hover:text-red-300 border border-red-500/30 flex items-center justify-center transition-all duration-200 cursor-pointer"
        :title="t('common.cancel')"
        @click="handleCancel"
      >
        <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 sm:h-6 sm:w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
        </svg>
      </button>
      <!-- Desktop: Send button (always shown, works as inject during streaming) -->
      <button
        v-if="!isCompact && (!streaming || canSend)"
        :disabled="!canSend"
        class="flex-shrink-0 w-10 h-10 rounded-xl bg-gradient-to-r from-purple-500 to-pink-500 text-white flex items-center justify-center transition-all duration-200 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer hover:shadow-glow"
        :title="streaming ? t('chat.sendDuringStream') : t('chat.send')"
        @click="handleSend"
      >
        <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 sm:h-6 sm:w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 19l9 2-9-18-9 18 9-2zm0 0v-8" />
        </svg>
      </button>
    </div>

    <!-- Voice error message -->
    <div
      v-if="voiceError"
      class="text-xs text-red-400 mt-2 text-center flex items-center justify-center gap-2"
    >
      <span>{{ voiceError }}</span>
      <button
        class="text-red-400 hover:text-red-300 underline cursor-pointer"
        @click="voiceError = null"
      >
        {{ t('chat.dismiss') }}
      </button>
    </div>

    <!-- Recording indicator -->
    <div
      v-if="isRecording && !(isCompact && voiceMode)"
      class="text-xs text-red-400 mt-2 text-center flex items-center justify-center gap-2"
    >
      <span class="w-2 h-2 bg-red-500 rounded-full animate-pulse"></span>
      <span>{{ t('chat.recording') }} {{ recordingDuration }}s</span>
    </div>

    </div>
  </div>
</template>

<style scoped>
.chat-input-wrapper {
  background: linear-gradient(to top, var(--color-bg-base) 60%, transparent);
}

/* No gradient on mobile - flush to bottom */
@media (max-width: 767px) {
  .chat-input-wrapper {
    background: none;
  }
}

:root.light .chat-input-wrapper,
[data-theme="light"] .chat-input-wrapper {
  background: linear-gradient(to top, rgb(249 250 251) 60%, transparent);
}

@media (max-width: 767px) {
  :root.light .chat-input-wrapper,
  [data-theme="light"] .chat-input-wrapper {
    background: none;
  }
}

/* Desktop: glass-card handles border. Mobile/compact: Tailwind border-t handles it. */

:root.light .chat-input-container,
[data-theme="light"] .chat-input-container {
  background: rgba(255, 255, 255, 0.95);
}

textarea {
  max-height: 200px;
  border-radius: var(--radius-lg);
  box-sizing: border-box;
}

.chat-textarea {
  height: 40px;
  min-height: 40px;
  padding-top: 9px;
  padding-bottom: 9px;
  line-height: 20px;
  overflow-y: auto;
  margin: 0;
  display: block;
  vertical-align: top;
}

textarea::-webkit-scrollbar {
  width: 6px;
}

textarea::-webkit-scrollbar-track {
  background: transparent;
}

textarea::-webkit-scrollbar-thumb {
  background: var(--color-bg-surface);
  border-radius: 3px;
}

/* Hide scrollbar when content fits in one line */
textarea::-webkit-scrollbar-thumb {
  background: transparent;
}

textarea:hover::-webkit-scrollbar-thumb {
  background: var(--color-bg-surface);
}

textarea::-webkit-scrollbar-thumb:hover {
  background: rgba(156, 163, 175, 0.5);
}

/* Voice breathing animation for hold-to-speak */
@keyframes voice-breathing {
  0%, 100% { transform: scale(1); box-shadow: 0 0 0 0 rgba(34, 197, 94, 0.3); }
  50% { transform: scale(1.02); box-shadow: 0 0 12px 2px rgba(34, 197, 94, 0.25); }
}
.voice-breathing {
  animation: voice-breathing 1.2s ease-in-out infinite;
}

/* Voice choice panel animation */
.voice-choice-panel {
  animation: voice-choice-slide-in 0.2s ease-out;
}

@keyframes voice-choice-slide-in {
  from { opacity: 0; transform: translateY(4px); }
  to { opacity: 1; transform: translateY(0); }
}

/* Inline dictation green glow */
@keyframes dictation-pulse {
  0%, 100% { box-shadow: 0 0 4px rgba(34, 197, 94, 0.4); }
  50% { box-shadow: 0 0 8px rgba(34, 197, 94, 0.6); }
}
.dictation-glow {
  animation: dictation-pulse 1.5s ease-in-out infinite;
  border-radius: 9999px;
}
</style>
