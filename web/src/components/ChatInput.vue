<script setup lang="ts">
import { ref, computed, onUnmounted, nextTick, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { AudioRecorder, voiceApi } from '@/api/voice'
import { speechApi } from '@/api/speech'
import ImagePreview from '@/components/chat/ImagePreview.vue'
import ModelDownloadPrompt from '@/components/speech/ModelDownloadPrompt.vue'

const { t } = useI18n()

export interface FileAttachment {
  id: string
  file: File
  name: string
  size: number
  type: string
  preview?: string
}

const props = defineProps<{
  disabled?: boolean
  streaming?: boolean
  maxFileSize?: number // in bytes, default 10MB
  allowedTypes?: string[] // MIME types
}>()

const emit = defineEmits<{
  send: [message: string, attachments: FileAttachment[]]
  cancel: []
  openTalkMode: []
}>()

const message = ref('')
const textareaRef = ref<HTMLTextAreaElement | null>(null)
const fileInputRef = ref<HTMLInputElement | null>(null)
const cameraInputRef = ref<HTMLInputElement | null>(null)
const attachments = ref<FileAttachment[]>([])
const dragOver = ref(false)

// Mobile menu state
const isMobile = ref(false)
const showMobileMenu = ref(false)
const mobileMenuRef = ref<HTMLDivElement | null>(null)

// Image preview state
const showImagePreview = ref(false)
const previewImageSrc = ref('')
const previewImageAlt = ref('')

// Voice recording state
const isRecording = ref(false)
const isTranscribing = ref(false)
const recorder = ref<AudioRecorder | null>(null)
const voiceError = ref<string | null>(null)

// Model download prompt state
const showASRDownloadPrompt = ref(false)
const asrModelReady = ref(true) // Assume ready until checked

const maxSize = computed(() => props.maxFileSize || 10 * 1024 * 1024) // 10MB default
const allowedMimeTypes = computed(() => props.allowedTypes || [
  'image/*',
  'application/pdf',
  'text/plain',
  'text/markdown',
  'application/json',
  'text/csv',
])

const canSend = computed(() =>
  (message.value.trim().length > 0 || attachments.value.length > 0) && !props.disabled
)

// Shorter placeholder for mobile
const placeholder = computed(() =>
  isMobile.value ? t('chat.inputPlaceholderShort') : t('chat.inputPlaceholder')
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

  emit('send', message.value.trim(), [...attachments.value])
  message.value = ''
  attachments.value = []

  // Reset textarea height
  if (textareaRef.value) {
    textareaRef.value.style.height = 'auto'
  }
}

function handleCancel() {
  emit('cancel')
}

function handleKeydown(event: KeyboardEvent) {
  // Send on Enter (without Shift)
  if (event.key === 'Enter' && !event.shiftKey) {
    event.preventDefault()
    handleSend()
  }
}

function handleInput() {
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
    const res = await speechApi.getASRStatus()
    return res.data?.ready ?? false
  } catch {
    return true // Assume ready if check fails
  }
}

async function startRecording() {
  if (props.disabled || props.streaming) return

  // Check if ASR model is ready
  const ready = await checkASRModelReady()
  if (!ready) {
    showASRDownloadPrompt.value = true
    return
  }

  voiceError.value = null
  recorder.value = new AudioRecorder()

  recorder.value.onStop = async (audioBlob: Blob) => {
    isRecording.value = false
    isTranscribing.value = true

    try {
      // Get format from mime type
      const mimeType = recorder.value?.mimeType || 'audio/webm'
      const format = mimeType.includes('webm') ? 'webm' : mimeType.includes('ogg') ? 'ogg' : 'wav'

      // Transcribe audio
      const response = await voiceApi.transcribe(audioBlob, format)
      if (response.data.text) {
        // Append transcribed text to message
        if (message.value.trim()) {
          message.value += ' ' + response.data.text
        } else {
          message.value = response.data.text
        }
        // Trigger input resize
        handleInput()
      }
    } catch (error) {
      console.error('Transcription error:', error)
      voiceError.value = t('chat.voiceTranscriptionError')
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
}

function toggleRecording() {
  if (isRecording.value) {
    stopRecording()
  } else {
    startRecording()
  }
}

// Cleanup on unmount
onUnmounted(() => {
  if (recorder.value) {
    recorder.value.stop()
  }
  window.removeEventListener('resize', checkMobile)
  document.removeEventListener('click', handleClickOutside)
})

// Check if mobile
function checkMobile() {
  isMobile.value = window.innerWidth < 640
  if (!isMobile.value) {
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

function handleMobileVoice() {
  showMobileMenu.value = false
  toggleRecording()
}

function handleMobileTalkMode() {
  showMobileMenu.value = false
  emit('openTalkMode')
}

onMounted(() => {
  checkMobile()
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

defineExpose({ focus, setInput })
</script>

<template>
  <div
    class="chat-input-wrapper px-3 sm:px-4 pb-3 sm:pb-4 pt-2"
    @dragover="handleDragOver"
    @dragleave="handleDragLeave"
    @drop="handleDrop"
  >
    <div
      class="chat-input-container glass-card shadow-lg rounded-2xl p-3 sm:p-4 max-w-4xl mx-auto"
      :class="{ 'ring-2 ring-accent': dragOver }"
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
            class="w-6 h-6 text-accent"
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
      <!-- Mobile: Collapsed menu button -->
      <div v-if="isMobile" ref="mobileMenuRef" class="relative">
        <button
          :disabled="disabled || streaming"
          class="flex-shrink-0 w-10 h-10 rounded-xl glass-card text-gray-500 dark:text-slate-300 hover:text-gray-900 dark:hover:text-white hover:bg-gray-100 dark:hover:bg-white/10 flex items-center justify-center transition-all duration-200 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
          :class="{ 'bg-accent/20 text-accent': showMobileMenu }"
          :title="t('chat.moreActions')"
          @click.stop="toggleMobileMenu"
        >
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
              d="M12 6v.01M12 12v.01M12 18v.01"
            />
          </svg>
        </button>

        <!-- Mobile menu dropdown -->
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
            class="absolute bottom-full left-0 mb-2 w-48 glass-card rounded-xl shadow-lg border border-white/10 overflow-hidden z-50"
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
              :disabled="disabled || streaming || isTranscribing"
              class="w-full px-4 py-3 flex items-center gap-3 transition-colors disabled:opacity-50"
              :class="isRecording
                ? 'bg-red-500/20 text-red-400'
                : 'text-gray-700 dark:text-slate-200 hover:bg-gray-100 dark:hover:bg-white/10'"
              @click="handleMobileVoice"
            >
              <svg v-if="isTranscribing" class="w-5 h-5 animate-spin" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
              </svg>
              <svg v-else class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11a7 7 0 01-7 7m0 0a7 7 0 01-7-7m7 7v4m0 0H8m4 0h4m-4-8a3 3 0 01-3-3V5a3 3 0 116 0v6a3 3 0 01-3 3z" />
              </svg>
              <span class="text-sm">{{ isRecording ? t('chat.stopRecording') : t('chat.startRecording') }}</span>
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

      <!-- Desktop: Individual buttons -->
      <!-- Attachment button -->
      <button
        v-if="!isMobile"
        :disabled="disabled || streaming"
        class="flex-shrink-0 w-10 h-10 sm:w-11 sm:h-11 rounded-xl glass-card text-gray-500 dark:text-slate-300 hover:text-gray-900 dark:hover:text-white hover:bg-gray-100 dark:hover:bg-white/10 flex items-center justify-center transition-all duration-200 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
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
        v-if="!isMobile"
        :disabled="disabled || streaming"
        class="flex-shrink-0 w-10 h-10 sm:w-11 sm:h-11 rounded-xl glass-card text-gray-500 dark:text-slate-300 hover:text-gray-900 dark:hover:text-white hover:bg-gray-100 dark:hover:bg-white/10 flex items-center justify-center transition-all duration-200 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
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
        v-if="!isMobile"
        :disabled="disabled || streaming || isTranscribing"
        class="flex-shrink-0 w-10 h-10 sm:w-11 sm:h-11 rounded-xl glass-card flex items-center justify-center transition-all duration-200 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
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
        v-if="!isMobile"
        :disabled="disabled || streaming"
        class="flex-shrink-0 w-10 h-10 sm:w-11 sm:h-11 rounded-xl glass-card text-gray-500 dark:text-slate-300 hover:text-gray-900 dark:hover:text-white hover:bg-gray-100 dark:hover:bg-white/10 flex items-center justify-center transition-all duration-200 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
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

      <div class="flex-1 relative">
        <textarea
          ref="textareaRef"
          v-model="message"
          :disabled="disabled || streaming"
          :placeholder="placeholder"
          class="w-full glass-input text-gray-900 dark:text-white px-3 py-2 sm:px-4 sm:py-3 pr-12 resize-none disabled:opacity-50 disabled:cursor-not-allowed"
          rows="1"
          @keydown="handleKeydown"
          @input="handleInput"
          @paste="handlePaste"
        />
      </div>

      <!-- Send/Cancel button -->
      <button
        v-if="streaming"
        class="flex-shrink-0 w-10 h-10 sm:w-11 sm:h-11 rounded-xl bg-red-500/20 hover:bg-red-500/30 text-red-400 hover:text-red-300 border border-red-500/30 flex items-center justify-center transition-all duration-200 cursor-pointer"
        :title="t('common.cancel')"
        @click="handleCancel"
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
            d="M6 18L18 6M6 6l12 12"
          />
        </svg>
      </button>

      <button
        v-else
        :disabled="!canSend"
        class="flex-shrink-0 w-10 h-10 sm:w-11 sm:h-11 rounded-xl bg-gradient-to-r from-accent to-cta text-white flex items-center justify-center transition-all duration-200 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer hover:shadow-glow"
        :title="t('chat.send')"
        @click="handleSend"
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
            d="M12 19l9 2-9-18-9 18 9-2zm0 0v-8"
          />
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
      v-if="isRecording"
      class="text-xs text-red-400 mt-2 text-center flex items-center justify-center gap-2"
    >
      <span class="w-2 h-2 bg-red-500 rounded-full animate-pulse"></span>
      <span>{{ t('chat.recording') }}</span>
    </div>

    <!-- Hint text -->
      <div v-if="!isRecording && !voiceError" class="text-xs text-gray-400 dark:text-slate-500 mt-2 text-center hidden sm:block">
        {{ t('chat.enterToSend') }} <kbd class="px-1.5 py-0.5 glass rounded text-gray-500 dark:text-slate-400">Enter</kbd>,
        <kbd class="px-1.5 py-0.5 glass rounded text-gray-500 dark:text-slate-400">Shift + Enter</kbd> {{ t('chat.newLine') }}
        <span class="mx-2 text-gray-300 dark:text-slate-600">|</span>
        {{ t('chat.dragDropHint') }}
      </div>
    </div>
  </div>
</template>

<style scoped>
.chat-input-wrapper {
  background: linear-gradient(to top, var(--color-bg-base) 60%, transparent);
}

:root.light .chat-input-wrapper,
[data-theme="light"] .chat-input-wrapper {
  background: linear-gradient(to top, rgb(249 250 251) 60%, transparent);
}

.chat-input-container {
  border: 1px solid var(--glass-border);
}

:root.light .chat-input-container,
[data-theme="light"] .chat-input-container {
  background: rgba(255, 255, 255, 0.95);
  border-color: rgba(0, 0, 0, 0.1);
}

textarea {
  min-height: 40px;
  max-height: 200px;
  border-radius: var(--radius-lg);
}

@media (min-width: 640px) {
  textarea {
    min-height: 44px;
  }
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
</style>
