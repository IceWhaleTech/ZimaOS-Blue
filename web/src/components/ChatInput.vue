<script setup lang="ts">
import { ref, computed, onUnmounted, nextTick, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { type MarketplaceAdviceResponse, type RemoteSkill, skillApi } from '@/api/skill'
import { AudioRecorder, voiceApi } from '@/api/voice'
import { speechApi } from '@/api/speech'
import { convertToWav } from '@/utils/audioConverter'
import {
  clearDraftAttachments as clearStoredDraftAttachments,
  loadDraftAttachments as loadStoredDraftAttachments,
  saveDraftAttachments as saveStoredDraftAttachments,
  type DraftAttachmentPayload,
} from '@/utils/chatDraftAttachmentStorage'
import { EnergyVAD } from '@/utils/vad'
import ImagePreview from '@/components/chat/ImagePreview.vue'
import ModelDownloadPrompt from '@/components/speech/ModelDownloadPrompt.vue'
import { useLocaleStore } from '@/stores/locale'
import { useChatStore } from '@/stores/chat'
import { useSettingsStore } from '@/stores/settings'
import { classifyFeatureIntent } from '@/composables/useFeatureIntent'
import { rafThrottle } from '@/utils/rafThrottle'
import { useRouter } from 'vue-router'

const { t, te } = useI18n()
const localeStore = useLocaleStore()
const chatStore = useChatStore()
const settingsStore = useSettingsStore()
const router = useRouter()

const SPEECH_ERROR_KEY_BY_CODE: Record<string, string> = {
  timeout: 'speech.error.timeout',
  audio_invalid: 'speech.error.audioInvalid',
  service_unavailable: 'speech.error.serviceUnavailable',
  rate_limit: 'speech.error.rateLimit',
  no_asr_provider: 'speech.error.noAsrProvider',
}

function getSpeechErrorMessage(errorCodeRaw: unknown): string | null {
  if (typeof errorCodeRaw !== 'string' || !errorCodeRaw.trim()) return null
  const normalized = errorCodeRaw.trim().toLowerCase()
  const key = SPEECH_ERROR_KEY_BY_CODE[normalized]
  if (!key) return null
  if (!te(key)) return null
  return t(key)
}

function chatText(
  key: string,
  fallback: string,
  params?: Record<string, string | number | boolean>
): string {
  if (!te(key)) return fallback
  return params ? t(key, params) : t(key)
}

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
  canCancel?: boolean
  maxFileSize?: number // in bytes, default 10MB
  allowedTypes?: string[] // MIME types
  conversationId?: string
}>()

const emit = defineEmits<{
  send: [message: string, attachments: FileAttachment[]]
  inject: [message: string]
  cancel: []
  openTalkMode: []
  warmup: []
  'cancel-pre-ttft': []
  'draft-change': [message: string]
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
const compactModeInfoCard = ref<null | 'research' | 'loop' | 'report' | 'ui'>(null)
const isMobileUserAgent = /android|webos|iphone|ipad|ipod|blackberry|iemobile|opera mini/i.test(
  navigator.userAgent.toLowerCase()
)

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

const LEGACY_DRAFT_STORAGE_KEY = 'zima.chat.input_draft.v1'
const DRAFT_STORAGE_KEY_PREFIX = 'zima.chat.input_draft.v2'
const NEW_CHAT_DRAFT_SCOPE = '__new__'
const MAX_TEXTAREA_HEIGHT = 200
const COMPACT_TEXTAREA_MIN_HEIGHT = 40
const DESKTOP_TEXTAREA_MIN_HEIGHT = 43
const SKILL_ADVICE_DEBOUNCE_MS = 650
const SKILL_ADVICE_MIN_QUERY_LENGTH = 12
const sendIconPath = 'M12 18.5V5.5m0 0L6.75 10.75M12 5.5l5.25 5.25'
const mobileClearButtonStyle = { insetInlineEnd: '0.5rem' }
const mobileMenuDropdownStyle = { insetInlineEnd: '0' }
const attachmentRemoveButtonStyle = { insetInlineEnd: '-0.25rem' }

const mobileDictationButtonStyle = computed(() => ({
  insetInlineEnd: isDictating.value || message.value.length > 0 ? '2rem' : '0.5rem',
}))

const draftStorageScope = computed(() => {
  const conversationId = props.conversationId?.trim()
  return conversationId || NEW_CHAT_DRAFT_SCOPE
})

const draftStorageKey = computed(() => `${DRAFT_STORAGE_KEY_PREFIX}:${draftStorageScope.value}`)
let attachmentDraftRestoreRequestId = 0

function readDraftFromStorage(key: string): string {
  try {
    return localStorage.getItem(key) || ''
  } catch {
    return ''
  }
}

function loadDraftFromStorage(key: string, allowLegacy = false): string {
  const draft = readDraftFromStorage(key)
  if (draft || !allowLegacy) {
    return draft
  }

  try {
    return localStorage.getItem(LEGACY_DRAFT_STORAGE_KEY) || ''
  } catch {
    return ''
  }
}

function persistDraftToStorage(key: string, value: string, clearLegacy = false) {
  try {
    if (value) {
      localStorage.setItem(key, value)
    } else {
      localStorage.removeItem(key)
    }
    if (clearLegacy) {
      localStorage.removeItem(LEGACY_DRAFT_STORAGE_KEY)
    }
  } catch {
    // Ignore storage errors (private mode/quota)
  }
}

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

// Inline dictation (VAD-based tap-to-dictate)
const isDictating = ref(false)
let dictationVAD: EnergyVAD | null = null
const skillAdvice = ref<MarketplaceAdviceResponse | null>(null)
const skillAdviceLoading = ref(false)
let latestSkillAdviceRequestId = 0
let skillAdviceTimer: ReturnType<typeof window.setTimeout> | null = null

const maxSize = computed(() => props.maxFileSize || 10 * 1024 * 1024) // 10MB default
const canShowCancelButton = computed(() => props.canCancel ?? props.streaming ?? false)
const allowedMimeTypes = computed(
  () =>
    props.allowedTypes || [
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
    ]
)

const canSend = computed(
  () => (message.value.trim().length > 0 || attachments.value.length > 0) && !props.disabled
)

// Shorter placeholder for mobile
const placeholder = computed(() =>
  isCompact.value ? t('chat.inputPlaceholderShort') : t('chat.inputPlaceholder')
)

function normalizeSkillSearchQuery(value: string): string {
  return value.replace(/\s+/g, ' ').trim()
}

function shouldFetchSkillAdvice(query: string): boolean {
  if (!query || query.length < SKILL_ADVICE_MIN_QUERY_LENGTH) return false
  if (query.includes(' ')) return true
  return query.length >= 24
}

const featureIntent = computed(() => classifyFeatureIntent(message.value))
const normalizedSkillAdviceQuery = computed(() => normalizeSkillSearchQuery(message.value))
const activeSkillAdvice = computed<MarketplaceAdviceResponse | null>(() => {
  const advice = skillAdvice.value
  if (!advice) return null
  return normalizeSkillSearchQuery(advice.query) === normalizedSkillAdviceQuery.value ? advice : null
})
const showFeatureHint = computed(() => {
  if (props.disabled || props.streaming) return false
  if (!message.value.trim()) return false
  if (featureIntent.value.deepResearch && !chatStore.deepResearchEnabled) return true
  if (featureIntent.value.agentMode && !settingsStore.agentMode) return true
  return false
})

const skillHintQueries = computed(() => {
  const advice = activeSkillAdvice.value
  const current = normalizedSkillAdviceQuery.value.toLowerCase()
  const seen = new Set<string>()
  return (advice?.search_queries || [])
    .map((value) => normalizeSkillSearchQuery(value))
    .filter((value) => {
      const normalized = value.toLowerCase()
      if (!value || normalized === current || seen.has(normalized)) return false
      seen.add(normalized)
      return true
    })
    .slice(0, 4)
})

const skillHintTags = computed(() => {
  const seen = new Set<string>()
  return (activeSkillAdvice.value?.capability_tags || [])
    .map((value) => value.trim())
    .filter((value) => {
      const normalized = value.toLowerCase()
      if (!normalized || seen.has(normalized)) return false
      seen.add(normalized)
      return true
    })
    .slice(0, 6)
})

const skillHintRecommendedSkills = computed<RemoteSkill[]>(() => {
  const advice = activeSkillAdvice.value
  const results = advice?.results || []
  if (!results.length) return []

  const byID = new Map(results.map((item) => [item.skill.id, item.skill] as const))
  const ordered: RemoteSkill[] = []

  for (const id of advice?.recommended_ids || []) {
    const skill = byID.get(id)
    if (skill) ordered.push(skill)
  }
  for (const result of results) {
    if (!ordered.some((item) => item.id === result.skill.id)) {
      ordered.push(result.skill)
    }
  }

  return ordered.slice(0, 3)
})

const skillHintInstalledSkill = computed(
  () => activeSkillAdvice.value?.installed_decision?.selected_skill?.trim() || ''
)
const skillHintStoreQuery = computed(
  () => skillHintQueries.value[0] || normalizedSkillAdviceQuery.value
)
const showSkillAdviceHint = computed(() => {
  if (props.disabled || props.streaming) return false
  if (!shouldFetchSkillAdvice(normalizedSkillAdviceQuery.value)) return false
  return (
    skillAdviceLoading.value ||
    !!skillHintInstalledSkill.value ||
    skillHintQueries.value.length > 0 ||
    skillHintTags.value.length > 0 ||
    skillHintRecommendedSkills.value.length > 0
  )
})
const skillAdviceDescription = computed(() => {
  if (skillAdviceLoading.value && !activeSkillAdvice.value) {
    return chatText(
      'chat.skillAdvisorLoadingBody',
      'Analyzing this task to suggest skill keywords and capability tags.'
    )
  }
  if (skillHintInstalledSkill.value) {
    return te('chat.skillAdvisorInstalledBody')
      ? t('chat.skillAdvisorInstalledBody', { skill: skillHintInstalledSkill.value })
      : `An installed skill may already fit: ${skillHintInstalledSkill.value}`
  }
  return chatText(
    'chat.skillAdvisorBody',
    'Use these keywords in the skill store if you want to find matching skills before you send.'
  )
})

const deepResearchInfoTags = computed(() => [
  t('chat.deepResearchStageRetrieve', 'Retrieve'),
  t('chat.deepResearchStageVerify', 'Verify'),
  t('chat.deepResearchCitations', 'Citations'),
])

const analyzeReportInfoTags = computed(() => [
  t('chat.analyzeReportHoverTagReport', 'Report'),
  t('chat.analyzeReportHoverTagInsights', 'Insights'),
  t('chat.analyzeReportHoverTagRecommendations', 'Recommendations'),
])

const ralphLoopAutoConfirmStateLabel = computed(() =>
  settingsStore.agentAutoConfirm ? t('common.enabled', 'Enabled') : t('common.disabled', 'Disabled')
)

const ralphLoopInfoTags = computed(() => [
  t('chat.ralphLoopHoverPlan', 'Plan'),
  t('chat.ralphLoopHoverAct', 'Act'),
  t('chat.ralphLoopHoverCheck', 'Check'),
  `${t('agent.autoConfirm')}: ${ralphLoopAutoConfirmStateLabel.value}`,
])

const uiReviewInfoTags = computed(() => [
  t('uiReview.visual', 'Visual'),
  t('chat.uiReviewHoverUsability', 'Usability'),
  t('uiReview.accessibility', 'Accessibility'),
])

const compactModeInfoCardMeta = computed(() => {
  if (compactModeInfoCard.value === 'research') {
    return {
      kind: 'research' as const,
      title: t('ui.deepResearchTitle'),
      state: chatStore.deepResearchEnabled
        ? t('common.enabled', 'Enabled')
        : t('common.disabled', 'Disabled'),
      description: t(
        'chat.deepResearchHoverDescription',
        'Launch a structured research workflow with retrieval, verification, and source-backed answers.'
      ),
      tags: deepResearchInfoTags.value,
    }
  }

  if (compactModeInfoCard.value === 'loop') {
    return {
      kind: 'loop' as const,
      title: t('chat.taskLoop'),
      state: settingsStore.agentMode
        ? t('common.enabled', 'Enabled')
        : t('common.disabled', 'Disabled'),
      description: t(
        'chat.ralphLoopHoverDescription',
        'Let the agent plan, use tools, apply changes, and keep iterating until the task lands cleanly.'
      ),
      tags: ralphLoopInfoTags.value,
    }
  }

  if (compactModeInfoCard.value === 'report') {
    return {
      kind: 'report' as const,
      title: t('chat.analyzeReportShortcutTitle', 'Analysis Report'),
      state: t('chat.analyzeReportHoverState', 'Prompt template'),
      description: t(
        'chat.analyzeReportHoverDescription',
        'Turn URLs, search results, or pasted text into a structured report with findings, comparisons, and recommendations.'
      ),
      tags: analyzeReportInfoTags.value,
    }
  }

  if (compactModeInfoCard.value === 'ui') {
    return {
      kind: 'ui' as const,
      title: t('chat.uiReviewShortcutTitle', 'UI Review'),
      state: t('chat.uiReviewHoverState', 'Prompt template'),
      description: t(
        'chat.uiReviewHoverDescription',
        'Audit a page or screenshot for visual quality, interaction clarity, and accessibility, then list concrete issues and fixes.'
      ),
      tags: uiReviewInfoTags.value,
    }
  }

  return null
})

function generateId(): string {
  return Math.random().toString(36).substring(2, 15)
}

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

function clearSkillAdviceTimer() {
  if (skillAdviceTimer) {
    window.clearTimeout(skillAdviceTimer)
    skillAdviceTimer = null
  }
}

function clearSkillAdvice() {
  clearSkillAdviceTimer()
  latestSkillAdviceRequestId += 1
  skillAdviceLoading.value = false
  skillAdvice.value = null
}

async function fetchSkillAdvice(options?: { force?: boolean }) {
  const query = normalizedSkillAdviceQuery.value
  if (props.disabled || props.streaming || !shouldFetchSkillAdvice(query)) {
    clearSkillAdvice()
    return
  }
  if (
    !options?.force &&
    activeSkillAdvice.value &&
    normalizeSkillSearchQuery(activeSkillAdvice.value.query) === query
  ) {
    return
  }

  const requestId = ++latestSkillAdviceRequestId
  skillAdviceLoading.value = true

  try {
    const response = await skillApi.adviseMarket({ query })
    if (requestId !== latestSkillAdviceRequestId) return
    skillAdvice.value = response.data
  } catch (err) {
    if (requestId !== latestSkillAdviceRequestId) return
    console.error('Failed to fetch chat skill advice:', err)
    skillAdvice.value = null
  } finally {
    if (requestId === latestSkillAdviceRequestId) {
      skillAdviceLoading.value = false
    }
  }
}

function scheduleSkillAdvice(options?: { force?: boolean }) {
  clearSkillAdviceTimer()

  const query = normalizedSkillAdviceQuery.value
  if (props.disabled || props.streaming || !shouldFetchSkillAdvice(query)) {
    clearSkillAdvice()
    return
  }

  skillAdviceTimer = window.setTimeout(() => {
    skillAdviceTimer = null
    void fetchSkillAdvice(options)
  }, options?.force ? 0 : SKILL_ADVICE_DEBOUNCE_MS)
}

function openSkillStore(query?: string) {
  const normalized = normalizeSkillSearchQuery(query || skillHintStoreQuery.value)
  void router.push({
    name: 'Plugins',
    query: normalized ? { tab: 'store', q: normalized } : { tab: 'store' },
  })
}

function isAllowedType(file: File): boolean {
  return allowedMimeTypes.value.some((type) => {
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

function invalidatePendingAttachmentRestore() {
  attachmentDraftRestoreRequestId += 1
}

function mapAttachmentToDraftPayload(attachment: FileAttachment): DraftAttachmentPayload {
  return {
    id: attachment.id,
    name: attachment.name,
    type: attachment.type,
    duration: attachment.duration,
    lastModified: attachment.file.lastModified,
    blob: attachment.file,
  }
}

async function mapDraftPayloadToAttachment(
  attachment: DraftAttachmentPayload
): Promise<FileAttachment> {
  const file = new File([attachment.blob], attachment.name, {
    type: attachment.type,
    lastModified: attachment.lastModified ?? Date.now(),
  })
  return {
    id: attachment.id || generateId(),
    file,
    name: attachment.name,
    size: file.size,
    type: attachment.type,
    preview: await createPreview(file),
    duration: attachment.duration,
  }
}

async function persistAttachmentsForStorageKey(key: string, nextAttachments: FileAttachment[]) {
  if (nextAttachments.length === 0) {
    await clearStoredDraftAttachments(key)
    return
  }

  await saveStoredDraftAttachments(
    key,
    nextAttachments.map((attachment) => mapAttachmentToDraftPayload(attachment))
  )
}

function persistAttachmentsForCurrentConversation(nextAttachments = attachments.value) {
  return persistAttachmentsForStorageKey(draftStorageKey.value, nextAttachments)
}

async function restoreAttachmentsForCurrentConversation() {
  const requestId = ++attachmentDraftRestoreRequestId
  const key = draftStorageKey.value
  attachments.value = []

  const storedAttachments = await loadStoredDraftAttachments(key)
  if (requestId !== attachmentDraftRestoreRequestId || key !== draftStorageKey.value) {
    return
  }

  attachments.value = await Promise.all(
    storedAttachments.map((attachment) => mapDraftPayloadToAttachment(attachment))
  )
}

async function addFiles(files: FileList | File[]) {
  invalidatePendingAttachmentRestore()
  const fileArray = Array.from(files)
  const nextAttachments = [...attachments.value]

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
    if (nextAttachments.some((a) => a.name === file.name && a.size === file.size)) {
      continue
    }

    const preview = await createPreview(file)

    nextAttachments.push({
      id: generateId(),
      file,
      name: file.name,
      size: file.size,
      type: file.type,
      preview,
    })
  }

  attachments.value = nextAttachments
  await persistAttachmentsForCurrentConversation(nextAttachments)
}

function removeAttachment(id: string) {
  invalidatePendingAttachmentRestore()
  attachments.value = attachments.value.filter((a) => a.id !== id)
  void persistAttachmentsForCurrentConversation(attachments.value)
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
    invalidatePendingAttachmentRestore()
    attachments.value = []
    void persistAttachmentsForCurrentConversation([])
  }
  message.value = ''
  warmupSent.value = false // Reset so next typing triggers warmup again
  preTTFTCancelSent.value = false

  // Reset textarea height
  resizeTextarea()
}

function clearMessage() {
  message.value = ''
  resizeTextarea()
  textareaRef.value?.focus()
}

function insertShortcutPrompt(prompt: string) {
  const trimmedPrompt = prompt.trim()
  if (!trimmedPrompt) return

  if (isCompact.value && voiceMode.value) {
    voiceMode.value = false
  }
  compactModeInfoCard.value = null

  const currentMessage = message.value.trimEnd()
  if (currentMessage.endsWith(trimmedPrompt)) {
    nextTick(() => textareaRef.value?.focus())
    return
  }

  message.value = currentMessage ? `${currentMessage}\n\n${trimmedPrompt}` : trimmedPrompt
  handleInput()
  nextTick(() => {
    textareaRef.value?.focus()
    const end = message.value.length
    textareaRef.value?.setSelectionRange(end, end)
  })
}

function handleAnalyzeReportShortcut() {
  insertShortcutPrompt(
    t(
      'chat.analyzeReportPrompt',
      'Create a structured analysis report.\n- Topic:\n- URLs, files, or input text:\n- Key questions, comparisons, or decisions to cover:'
    )
  )
}

function handleUIReviewShortcut() {
  insertShortcutPrompt(
    t(
      'chat.uiReviewPrompt',
      'Please run a UI review and return clear findings plus improvement suggestions.\n- Page URL or screenshot:\n- Target device: desktop / mobile\n- Focus areas: visual hierarchy, interaction flow, accessibility'
    )
  )
}

function toggleCompactModeInfo(kind: 'research' | 'loop' | 'report' | 'ui') {
  compactModeInfoCard.value = compactModeInfoCard.value === kind ? null : kind
}

function closeCompactModeInfo() {
  compactModeInfoCard.value = null
}

function toggleDeepResearch() {
  compactModeInfoCard.value = null
  chatStore.setDeepResearchEnabled(!chatStore.deepResearchEnabled)
}

function toggleAgentMode() {
  compactModeInfoCard.value = null
  settingsStore.setAgentMode(!settingsStore.agentMode).catch((err) => {
    console.error('Failed to update Ralph Loop mode:', err)
  })
}

function toggleAgentAutoConfirm() {
  settingsStore.setAgentAutoConfirm(!settingsStore.agentAutoConfirm).catch((err) => {
    console.error('Failed to update Ralph Loop auto-confirm:', err)
  })
}

function handleAgentModeContextMenu(event: MouseEvent) {
  event.preventDefault()
  toggleAgentAutoConfirm()
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

  resizeTextarea()
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
      } else {
        const mappedError = getSpeechErrorMessage(errorCode)
        if (mappedError) {
          voiceError.value = mappedError
        } else {
          const serverMsg =
            error?.response?.data?.error || error?.response?.data?.message || error?.message
          voiceError.value = serverMsg || t('chat.voiceTranscriptionError')
        }
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
      stream.getTracks().forEach((t) => t.stop())
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
  const touch = e.touches.item(0)
  if (!touch) return
  touchStartY.value = touch.clientY
  touchStartX.value = touch.clientX
  slideCancelled.value = false
  swipeDirection.value = 'none'
  // Start recording immediately — no delay
  startRecording()
  // Haptic feedback
  if (navigator.vibrate) navigator.vibrate(10)
}

function handleVoiceTouchMove(e: TouchEvent) {
  if (!isTouchDevice.value || !isRecording.value) return
  const touch = e.touches.item(0)
  if (!touch) return
  const dy = touchStartY.value - touch.clientY
  const dx = touch.clientX - touchStartX.value

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
      setTimeout(() => {
        if (voiceError.value === t('chat.voiceMessageTooShort')) voiceError.value = null
      }, 2000)
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
  if (!pendingAudioBlob.value) {
    dismissVoiceChoice()
    return
  }

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
    } else {
      const mappedError = getSpeechErrorMessage(errorCode)
      if (mappedError) {
        voiceError.value = mappedError
      } else {
        const serverMsg =
          error?.response?.data?.error || error?.response?.data?.message || error?.message
        voiceError.value = serverMsg || t('chat.voiceTranscriptionError')
      }
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
    stream.getTracks().forEach((t) => t.stop())
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
  void persistAttachmentsForCurrentConversation([...attachments.value])
  clearSkillAdvice()
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
  window.removeEventListener('resize', checkMobileOnResize)
  checkMobileOnResize.cancel()
  document.removeEventListener('click', handleClickOutside)
})

// Check if compact mode (narrow screen) and mobile device (UA)
function checkMobile() {
  isCompact.value = window.innerWidth < 768
  isMobile.value = isMobileUserAgent
  if (!isCompact.value) {
    showMobileMenu.value = false
    compactModeInfoCard.value = null
  }
}

const checkMobileOnResize = rafThrottle(checkMobile)

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

function toggleDeepResearchFromHint(event: Event) {
  const checked = (event.target as HTMLInputElement).checked
  chatStore.setDeepResearchEnabled(checked)
}

function toggleAgentModeFromHint(event: Event) {
  const checked = (event.target as HTMLInputElement).checked
  settingsStore.setAgentMode(checked).catch((err) => {
    console.error('Failed to update agent mode from IR hint:', err)
  })
}

onMounted(() => {
  checkMobile()
  isTouchDevice.value = 'ontouchstart' in window || navigator.maxTouchPoints > 0
  window.addEventListener('resize', checkMobileOnResize)
  document.addEventListener('click', handleClickOutside)

  restoreDraftForCurrentConversation()
})

watch(message, (nextMessage) => {
  emit('draft-change', nextMessage)
  persistDraftToStorage(
    draftStorageKey.value,
    nextMessage,
    draftStorageScope.value === NEW_CHAT_DRAFT_SCOPE
  )
  scheduleSkillAdvice()
})

watch(draftStorageKey, (nextKey, previousKey) => {
  if (previousKey && previousKey !== nextKey) {
    persistDraftToStorage(
      previousKey,
      message.value,
      previousKey === `${DRAFT_STORAGE_KEY_PREFIX}:${NEW_CHAT_DRAFT_SCOPE}`
    )
    void persistAttachmentsForStorageKey(previousKey, [...attachments.value])
  }
  restoreDraftForCurrentConversation()
})

watch(textareaRef, (textarea) => {
  if (!textarea) return
  nextTick(() => {
    resizeTextarea()
  })
})

watch(isCompact, () => {
  nextTick(() => {
    resizeTextarea()
  })
})

watch([() => props.disabled, () => props.streaming], ([disabled, streaming]) => {
  if (disabled || streaming) {
    clearSkillAdvice()
    return
  }
  scheduleSkillAdvice({ force: true })
})

function focus() {
  textareaRef.value?.focus()
}

function resizeTextarea() {
  const textarea = textareaRef.value
  if (!textarea) return

  textarea.style.height = 'auto'

  const minHeight = isCompact.value ? COMPACT_TEXTAREA_MIN_HEIGHT : DESKTOP_TEXTAREA_MIN_HEIGHT
  const contentHeight = Math.max(textarea.scrollHeight, minHeight)
  const nextHeight = Math.min(contentHeight, MAX_TEXTAREA_HEIGHT)

  textarea.style.height = `${nextHeight}px`
  textarea.style.overflowY = contentHeight > MAX_TEXTAREA_HEIGHT ? 'auto' : 'hidden'
}

function setInput(text: string) {
  message.value = text
  nextTick(() => {
    resizeTextarea()
  })
}

function restoreTextDraftForCurrentConversation() {
  const cachedDraft = loadDraftFromStorage(
    draftStorageKey.value,
    draftStorageScope.value === NEW_CHAT_DRAFT_SCOPE
  )
  setInput(cachedDraft)
}

function restoreDraftForCurrentConversation() {
  restoreTextDraftForCurrentConversation()
  void restoreAttachmentsForCurrentConversation()
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
    :class="isMobile ? 'px-3 pb-0 pt-0' : isCompact ? 'px-3 pb-0 pt-0' : 'px-3 sm:px-4 pb-0 pt-0.5'"
  >
    <div
      class="chat-input-container p-2.5 sm:px-3.5 sm:py-2.5 max-w-5xl mx-auto"
      :class="[
        isMobile || isCompact
          ? 'chat-input-floating-shell'
          : 'chat-input-desktop-shell rounded-2xl',
        isMobile || isCompact ? 'chat-input-container--compact' : 'chat-input-container--desktop',
        { 'ring-2 ring-accent': dragOver },
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
      <ImagePreview v-model="showImagePreview" :src="previewImageSrc" :alt="previewImageAlt" />

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
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M7 21h10a2 2 0 002-2V9.414a1 1 0 00-.293-.707l-5.414-5.414A1 1 0 0012.586 3H7a2 2 0 00-2 2v14a2 2 0 002 2z"
              />
            </svg>
            <svg
              v-else-if="getFileIcon(attachment.type) === 'text'"
              class="w-6 h-6 text-gray-900 dark:text-gray-300"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
              />
            </svg>
            <svg
              v-else
              class="w-6 h-6 text-slate-400"
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
          </div>

          <!-- File info -->
          <div class="flex-1 min-w-0">
            <p class="text-sm text-gray-900 dark:text-white truncate">{{ attachment.name }}</p>
            <p class="text-xs text-gray-500 dark:text-slate-400">
              {{ formatFileSize(attachment.size) }}
            </p>
          </div>

          <!-- Remove button -->
          <button
            class="absolute -top-1 w-5 h-5 bg-red-500 hover:bg-red-600 rounded-full flex items-center justify-center opacity-0 group-hover:opacity-100 transition-all duration-200 cursor-pointer"
            :style="attachmentRemoveButtonStyle"
            @click="removeAttachment(attachment.id)"
          >
            <svg class="w-3 h-3 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
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

      <div v-if="showFeatureHint" class="mb-2 flex flex-wrap items-center gap-2 text-xs">
        <span class="text-gray-500 dark:text-slate-400">{{
          t('chat.featureHintTapToEnable')
        }}</span>
        <label
          v-if="featureIntent.deepResearch && !chatStore.deepResearchEnabled"
          class="inline-flex items-center gap-1.5 px-2 py-1 rounded-md border border-emerald-200/80 dark:border-emerald-700/70 bg-emerald-50/70 dark:bg-emerald-900/20 text-emerald-700 dark:text-emerald-300 cursor-pointer"
        >
          <input
            type="checkbox"
            class="h-3.5 w-3.5 accent-emerald-500"
            :checked="chatStore.deepResearchEnabled"
            @change="toggleDeepResearchFromHint"
          />
          <span>{{ t('ui.deepResearchTitle') }}</span>
        </label>
        <label
          v-if="featureIntent.agentMode && !settingsStore.agentMode"
          class="inline-flex items-center gap-1.5 px-2 py-1 rounded-md border border-blue-200/80 dark:border-blue-700/70 bg-blue-50/70 dark:bg-blue-900/20 text-blue-700 dark:text-blue-300 cursor-pointer"
        >
          <input
            type="checkbox"
            class="h-3.5 w-3.5 accent-blue-500"
            :checked="settingsStore.agentMode"
            @change="toggleAgentModeFromHint"
          />
          <span>{{ t('chat.taskLoop') }}</span>
        </label>
      </div>

      <div
        v-if="showSkillAdviceHint"
        class="chat-skill-advice mb-2 rounded-2xl border border-amber-200/80 bg-amber-50/80 px-3 py-2.5 text-xs text-amber-950 shadow-sm dark:border-amber-500/20 dark:bg-amber-500/10 dark:text-amber-50"
      >
        <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
          <div class="min-w-0">
            <p class="text-[11px] font-semibold uppercase tracking-[0.14em] text-amber-600/90 dark:text-amber-300/80">
              {{
                chatText(
                  'chat.skillAdvisorKicker',
                  'Skill guidance'
                )
              }}
            </p>
            <p class="mt-1 text-xs leading-5 text-amber-900/85 dark:text-amber-100/85">
              {{ skillAdviceDescription }}
            </p>
          </div>
          <button
            type="button"
            data-testid="chat-skill-store-link"
            class="chat-skill-advice__store-link inline-flex items-center justify-center rounded-full border border-amber-300/80 bg-white/70 px-3 py-1.5 text-xs font-medium text-amber-700 transition-colors hover:bg-white dark:border-amber-300/20 dark:bg-white/5 dark:text-amber-100 dark:hover:bg-white/10"
            @click="openSkillStore()"
          >
            {{ chatText('chat.skillAdvisorOpenStore', 'Open Skill Store') }}
          </button>
        </div>

        <div
          v-if="skillHintInstalledSkill"
          class="mt-2 inline-flex items-center gap-1.5 rounded-full border border-emerald-300/70 bg-emerald-100/80 px-2.5 py-1 text-[11px] font-medium text-emerald-800 dark:border-emerald-400/20 dark:bg-emerald-400/10 dark:text-emerald-200"
        >
          <span>{{ chatText('chat.skillAdvisorInstalledLabel', 'Installed match') }}</span>
          <span>{{ skillHintInstalledSkill }}</span>
        </div>

        <div
          v-if="skillHintQueries.length"
          class="mt-2.5 flex flex-col gap-1.5"
        >
          <span class="text-[11px] font-medium text-amber-700/90 dark:text-amber-200/80">
            {{ chatText('chat.skillAdvisorSearchQueries', 'Suggested search phrases') }}
          </span>
          <div class="flex flex-wrap gap-2">
            <button
              v-for="query in skillHintQueries"
              :key="query"
              type="button"
              class="chat-skill-advice__query-chip rounded-full border border-amber-300/80 bg-white/80 px-2.5 py-1 text-[11px] font-medium text-amber-700 transition-colors hover:bg-white dark:border-amber-300/20 dark:bg-white/5 dark:text-amber-100 dark:hover:bg-white/10"
              @click="openSkillStore(query)"
            >
              {{ query }}
            </button>
          </div>
        </div>

        <div
          v-if="skillHintTags.length"
          class="mt-2.5 flex flex-col gap-1.5"
        >
          <span class="text-[11px] font-medium text-amber-700/90 dark:text-amber-200/80">
            {{ chatText('chat.skillAdvisorCapabilityTags', 'Capability tags') }}
          </span>
          <div class="flex flex-wrap gap-2">
            <button
              v-for="tag in skillHintTags"
              :key="tag"
              type="button"
              class="chat-skill-advice__tag-chip rounded-full border border-transparent bg-amber-100/90 px-2.5 py-1 text-[11px] font-medium text-amber-800 transition-colors hover:bg-amber-100 dark:bg-amber-400/10 dark:text-amber-100 dark:hover:bg-amber-400/15"
              @click="openSkillStore(tag)"
            >
              #{{ tag }}
            </button>
          </div>
        </div>

        <div
          v-if="skillHintRecommendedSkills.length"
          class="mt-2.5 flex flex-col gap-1.5"
        >
          <span class="text-[11px] font-medium text-amber-700/90 dark:text-amber-200/80">
            {{ chatText('chat.skillAdvisorRecommendedSkills', 'Marketplace matches') }}
          </span>
          <div class="flex flex-wrap gap-2">
            <button
              v-for="skill in skillHintRecommendedSkills"
              :key="skill.id"
              type="button"
              class="chat-skill-advice__skill-chip rounded-full border border-amber-300/70 bg-white/70 px-2.5 py-1 text-[11px] font-medium text-amber-700 transition-colors hover:bg-white dark:border-amber-300/20 dark:bg-white/5 dark:text-amber-100 dark:hover:bg-white/10"
              @click="openSkillStore(skill.name || skill.id)"
            >
              {{ skill.name || skill.id }}
            </button>
          </div>
        </div>
      </div>

      <div v-if="isCompact" class="compact-mode-section mb-3">
        <div class="composer-mode-row">
          <div class="compact-mode-action">
            <button
              class="mode-chip mode-chip-research"
              :class="{ 'is-active': chatStore.deepResearchEnabled }"
              :aria-pressed="chatStore.deepResearchEnabled"
              :title="t('ui.deepResearchTitle')"
              @click="toggleDeepResearch"
            >
              <svg class="mode-chip__icon" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <circle cx="10.5" cy="10.5" r="4.75" stroke-width="1.7" />
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="1.7"
                  d="M14 14l4 4M16 5.25h3M17.5 3.75v3"
                />
              </svg>
              <span class="mode-chip__label">{{ t('ui.deepResearchTitle') }}</span>
            </button>
            <button
              class="compact-mode-info-toggle compact-mode-info-toggle--research"
              :aria-expanded="compactModeInfoCard === 'research'"
              :title="t('chat.showShortcutDetails', 'Show details')"
              @click.stop="toggleCompactModeInfo('research')"
            >
              <svg fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <circle cx="12" cy="12" r="8.25" stroke-width="1.8" />
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="1.8"
                  d="M12 10.5v4.25M12 7.9h.01"
                />
              </svg>
            </button>
          </div>

          <div class="compact-mode-action">
            <button
              class="mode-chip mode-chip-loop"
              :class="{ 'is-active-agent': settingsStore.agentMode }"
              :aria-pressed="settingsStore.agentMode"
              :title="`${t('chat.taskLoop')} · ${t('agent.autoConfirm')}: ${ralphLoopAutoConfirmStateLabel}`"
              @click="toggleAgentMode"
              @contextmenu="handleAgentModeContextMenu"
            >
              <svg class="mode-chip__icon" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="1.7"
                  d="M9 4.75h6a1.75 1.75 0 011.75 1.75v10.75A1.75 1.75 0 0115 19H9a1.75 1.75 0 01-1.75-1.75V6.5A1.75 1.75 0 019 4.75z"
                />
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="1.7"
                  d="M9.75 3h4.5M10.25 9h4M10.25 12h4M10.25 15h2.5"
                />
              </svg>
              <span class="mode-chip__label">{{ t('chat.taskLoop') }}</span>
            </button>
            <button
              class="compact-mode-info-toggle compact-mode-info-toggle--loop"
              :aria-expanded="compactModeInfoCard === 'loop'"
              :title="t('chat.showShortcutDetails', 'Show details')"
              @click.stop="toggleCompactModeInfo('loop')"
            >
              <svg fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <circle cx="12" cy="12" r="8.25" stroke-width="1.8" />
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="1.8"
                  d="M12 10.5v4.25M12 7.9h.01"
                />
              </svg>
            </button>
          </div>

          <div class="compact-mode-action">
            <button
              class="mode-chip mode-chip-report"
              :title="t('chat.analyzeReportShortcutTitle', 'Analysis Report')"
              @click="handleAnalyzeReportShortcut"
            >
              <svg class="mode-chip__icon" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="1.7"
                  d="M7.75 4.75h6.5L18.25 8.75v8.5A1.75 1.75 0 0116.5 19h-8A1.75 1.75 0 016.75 17.25V6.5A1.75 1.75 0 018.5 4.75z"
                />
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="1.7"
                  d="M10 11.25h4M10 14h5M10 16.75h3.25M14.25 4.75V8.5h3.75"
                />
              </svg>
              <span class="mode-chip__label">{{
                t('chat.analyzeReportShortcutTitle', 'Analysis Report')
              }}</span>
            </button>
            <button
              class="compact-mode-info-toggle compact-mode-info-toggle--report"
              :aria-expanded="compactModeInfoCard === 'report'"
              :title="t('chat.showShortcutDetails', 'Show details')"
              @click.stop="toggleCompactModeInfo('report')"
            >
              <svg fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <circle cx="12" cy="12" r="8.25" stroke-width="1.8" />
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="1.8"
                  d="M12 10.5v4.25M12 7.9h.01"
                />
              </svg>
            </button>
          </div>

          <div class="compact-mode-action">
            <button
              class="mode-chip mode-chip-ui"
              :title="t('chat.uiReviewShortcutTitle', 'UI Review')"
              @click="handleUIReviewShortcut"
            >
              <svg class="mode-chip__icon" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="1.7"
                  d="M2.75 12s3.25-5.25 9.25-5.25S21.25 12 21.25 12s-3.25 5.25-9.25 5.25S2.75 12 2.75 12z"
                />
                <circle cx="12" cy="12" r="2.5" stroke-width="1.7" />
              </svg>
              <span class="mode-chip__label">{{
                t('chat.uiReviewShortcutTitle', 'UI Review')
              }}</span>
            </button>
            <button
              class="compact-mode-info-toggle compact-mode-info-toggle--ui"
              :aria-expanded="compactModeInfoCard === 'ui'"
              :title="t('chat.showShortcutDetails', 'Show details')"
              @click.stop="toggleCompactModeInfo('ui')"
            >
              <svg fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <circle cx="12" cy="12" r="8.25" stroke-width="1.8" />
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="1.8"
                  d="M12 10.5v4.25M12 7.9h.01"
                />
              </svg>
            </button>
          </div>
        </div>

        <div
          v-if="compactModeInfoCardMeta"
          class="compact-mode-info-card"
          :class="`compact-mode-info-card--${compactModeInfoCardMeta.kind}`"
        >
          <div class="compact-mode-info-card__header">
            <div class="compact-mode-info-card__eyebrow">{{ compactModeInfoCardMeta.state }}</div>
            <button
              class="compact-mode-info-card__close"
              :title="t('chat.hideShortcutDetails', 'Hide details')"
              @click="closeCompactModeInfo"
            >
              <svg fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2.2"
                  d="M6 18L18 6M6 6l12 12"
                />
              </svg>
            </button>
          </div>
          <div class="compact-mode-info-card__title">{{ compactModeInfoCardMeta.title }}</div>
          <p class="compact-mode-info-card__description">
            {{ compactModeInfoCardMeta.description }}
          </p>
          <div class="compact-mode-info-card__chips">
            <span
              v-for="tag in compactModeInfoCardMeta.tags"
              :key="`${compactModeInfoCardMeta.kind}-${tag}`"
              class="compact-mode-info-card__chip"
            >
              {{ tag }}
            </span>
          </div>
        </div>
      </div>

      <div :class="isCompact ? 'flex items-center gap-2 sm:gap-3' : 'desktop-composer-root'">
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
            <svg
              v-if="voiceMode"
              xmlns="http://www.w3.org/2000/svg"
              class="h-5 w-5"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <rect x="2" y="4" width="20" height="16" rx="2" stroke-width="2" />
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M6 8h.01M10 8h.01M14 8h.01M18 8h.01M6 12h.01M10 12h.01M14 12h.01M18 12h.01M8 16h8"
              />
            </svg>
            <!-- Mic icon when in text mode -->
            <svg
              v-else
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
                d="M19 11a7 7 0 01-7 7m0 0a7 7 0 01-7-7m7 7v4m0 0H8m4 0h4m-4-8a3 3 0 01-3-3V5a3 3 0 116 0v6a3 3 0 01-3 3z"
              />
            </svg>
          </button>

          <!-- Center: Voice hold-to-speak or Textarea -->
          <div v-if="voiceMode" class="flex-1 relative">
            <!-- Voice choice panel (shown after recording) -->
            <div v-if="showVoiceChoice" class="voice-choice-panel flex items-center gap-2 w-full">
              <button
                class="voice-choice-btn voice-choice-btn--transcribe flex-1 h-10 rounded-xl flex items-center justify-center gap-1.5 transition-all cursor-pointer"
                @click="handleVoiceChoiceTranscribe"
              >
                <svg
                  class="w-4 h-4"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  stroke-width="2"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
                  />
                </svg>
                <span class="text-sm font-medium">{{ t('chat.voiceToText') }}</span>
              </button>
              <button
                class="voice-choice-btn voice-choice-btn--send flex-1 h-10 rounded-xl flex items-center justify-center gap-1.5 transition-all cursor-pointer"
                @click="handleVoiceChoiceSend"
              >
                <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
                  <path
                    d="M3 20l1.3-4.8C3.5 13.8 3 12.4 3 11c0-5 4-9 9-9s9 4 9 9-4 9-9 9c-1.4 0-2.8-.5-4.2-1.2L3 20z"
                  />
                  <path
                    d="M9 10h6M9 14h4"
                    fill="none"
                    stroke="white"
                    stroke-width="1.5"
                    stroke-linecap="round"
                  />
                </svg>
                <span class="text-sm font-medium"
                  >{{ pendingAudioDuration }}s · {{ t('chat.sendVoice') }}</span
                >
              </button>
              <button
                class="voice-choice-dismiss flex-shrink-0 w-8 h-8 rounded-full flex items-center justify-center transition-colors cursor-pointer"
                @click="dismissVoiceChoice"
              >
                <svg
                  class="w-4 h-4"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  stroke-width="2.5"
                >
                  <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>
            <!-- Hold-to-speak button -->
            <button
              v-else
              :disabled="disabled || streaming || isTranscribing"
              class="w-full h-10 rounded-xl flex items-center justify-center gap-2 transition-all duration-200 select-none cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
              :class="
                isRecording
                  ? slideCancelled
                    ? 'bg-gray-500/20 text-gray-400 border border-gray-500/30'
                    : 'bg-green-500/20 text-green-500 border border-green-500/30 voice-breathing'
                  : isTranscribing
                    ? 'glass-card text-gray-500 dark:text-slate-300'
                    : 'glass-card text-gray-500 dark:text-slate-300 hover:text-gray-900 dark:hover:text-white hover:bg-gray-100 dark:hover:bg-white/10 active:bg-gray-200 dark:active:bg-white/20'
              "
              @touchstart.prevent="handleVoiceTouchStart"
              @touchmove.prevent="handleVoiceTouchMove"
              @touchend.prevent="handleVoiceTouchEnd"
              @touchcancel="handleVoiceTouchCancel"
              @click="handleVoiceClick"
            >
              <svg
                v-if="isTranscribing"
                class="h-5 w-5 animate-spin"
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
              <svg
                v-else
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
                  d="M19 11a7 7 0 01-7 7m0 0a7 7 0 01-7-7m7 7v4m0 0H8m4 0h4m-4-8a3 3 0 01-3-3V5a3 3 0 116 0v6a3 3 0 01-3 3z"
                />
              </svg>
              <span class="text-sm">
                <template v-if="isTranscribing">{{ t('chat.voiceTranscribing') }}</template>
                <template v-else-if="isRecording && slideCancelled">{{
                  t('chat.releaseToCancel')
                }}</template>
                <template v-else-if="isRecording"
                  >{{ recordingDuration }}s · {{ t('chat.recording') }}</template
                >
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
              :wrap="isCompact ? 'soft' : 'off'"
              enterkeyhint="send"
              class="chat-textarea w-full glass-input text-gray-900 dark:text-white resize-none disabled:opacity-50 disabled:cursor-not-allowed"
              rows="1"
              @keydown="handleKeydown"
              @input="handleInput"
              @paste="handlePaste"
            />
            <!-- Inline mic icon for tap-to-dictate (mobile text mode) -->
            <button
              v-if="!isTranscribing"
              class="absolute top-1/2 -translate-y-1/2 w-5 h-5 flex items-center justify-center rounded-full transition-all duration-200 cursor-pointer"
              :style="mobileDictationButtonStyle"
              :class="
                isDictating
                  ? 'text-green-500 dictation-glow'
                  : 'text-gray-400 hover:text-gray-600 dark:hover:text-gray-200'
              "
              :title="isDictating ? t('chat.stopDictation') : t('chat.startDictation')"
              @click="toggleDictation"
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="h-3.5 w-3.5"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                stroke-width="2"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  d="M19 11a7 7 0 01-7 7m0 0a7 7 0 01-7-7m7 7v4m0 0H8m4 0h4m-4-8a3 3 0 01-3-3V5a3 3 0 116 0v6a3 3 0 01-3 3z"
                />
              </svg>
            </button>
            <button
              v-if="message.length > 0"
              class="absolute top-1/2 -translate-y-1/2 w-5 h-5 flex items-center justify-center rounded-full text-gray-400 hover:text-gray-600 dark:hover:text-gray-200 hover:bg-gray-200/50 dark:hover:bg-gray-600/50 transition-colors cursor-pointer"
              :style="mobileClearButtonStyle"
              title="Clear"
              @click="clearMessage"
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="h-3.5 w-3.5"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                stroke-width="2.5"
              >
                <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>

          <!-- PC Narrow: Send button (between textarea and + button) -->
          <button
            v-if="isCompact && !isMobile"
            :disabled="!canSend"
            class="chat-send-btn compact-send-btn flex-shrink-0 w-10 h-10 rounded-full flex items-center justify-center disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
            :class="{ 'chat-send-btn--ready': canSend }"
            :title="t('chat.send')"
            @click="handleSend"
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="chat-send-btn__icon h-5 w-5"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="1.9"
                :d="sendIconPath"
              />
            </svg>
          </button>

          <!-- Right: + button for extensions (or Cancel during streaming) -->
          <button
            v-if="canShowCancelButton"
            class="flex-shrink-0 w-10 h-10 rounded-xl bg-red-500/20 hover:bg-red-500/30 text-red-400 hover:text-red-300 border border-red-500/30 flex items-center justify-center transition-all duration-200 cursor-pointer"
            :title="t('common.cancel')"
            @click="handleCancel"
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
                d="M6 18L18 6M6 6l12 12"
              />
            </svg>
          </button>
          <div v-else ref="mobileMenuRef" class="relative">
            <button
              :disabled="disabled"
              class="flex-shrink-0 w-10 h-10 rounded-xl glass-card text-gray-500 dark:text-slate-300 hover:text-gray-900 dark:hover:text-white hover:bg-gray-100 dark:hover:bg-white/10 flex items-center justify-center transition-all duration-200 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
              :class="{
                'bg-gray-200 dark:bg-gray-600/20 text-gray-900 dark:text-gray-300': showMobileMenu,
              }"
              :title="t('chat.moreActions')"
              @click.stop="toggleMobileMenu"
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="h-6 w-6"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                stroke-width="2"
              >
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
                class="absolute bottom-full mb-2 w-48 glass-card rounded-xl shadow-lg border border-white/10 overflow-hidden z-50"
                :style="mobileMenuDropdownStyle"
              >
                <button
                  :disabled="disabled || streaming"
                  class="w-full px-4 py-3 flex items-center gap-3 text-gray-700 dark:text-slate-200 hover:bg-gray-100 dark:hover:bg-white/10 transition-colors disabled:opacity-50"
                  @click="handleMobileAttachment"
                >
                  <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M15.172 7l-6.586 6.586a2 2 0 102.828 2.828l6.414-6.586a4 4 0 00-5.656-5.656l-6.415 6.585a6 6 0 108.486 8.486L20.5 13"
                    />
                  </svg>
                  <span class="text-sm">{{ t('chat.attachFile') }}</span>
                </button>

                <button
                  :disabled="disabled || streaming"
                  class="w-full px-4 py-3 flex items-center gap-3 text-gray-700 dark:text-slate-200 hover:bg-gray-100 dark:hover:bg-white/10 transition-colors disabled:opacity-50"
                  @click="handleMobileCamera"
                >
                  <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
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
                  <span class="text-sm">{{ t('chat.takePhoto') }}</span>
                </button>

                <button
                  :disabled="disabled || streaming"
                  class="w-full px-4 py-3 flex items-center gap-3 text-gray-700 dark:text-slate-200 hover:bg-gray-100 dark:hover:bg-white/10 transition-colors disabled:opacity-50"
                  @click="handleMobileTalkMode"
                >
                  <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M3 5a2 2 0 012-2h3.28a1 1 0 01.948.684l1.498 4.493a1 1 0 01-.502 1.21l-2.257 1.13a11.042 11.042 0 005.516 5.516l1.13-2.257a1 1 0 011.21-.502l4.493 1.498a1 1 0 01.684.949V19a2 2 0 01-2 2h-1C9.716 21 3 14.284 3 6V5z"
                    />
                  </svg>
                  <span class="text-sm">{{ t('chat.talkMode.title') }}</span>
                </button>
              </div>
            </Transition>
          </div>
        </template>

        <template v-else>
          <div class="desktop-composer-shell">
            <div class="desktop-composer-toolbar">
              <div class="desktop-toolbar-left">
                <button
                  :disabled="disabled || streaming"
                  class="composer-toolbar-btn"
                  :title="t('chat.attachFile')"
                  @click="openFileDialog"
                >
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    class="h-[1.05rem] w-[1.05rem]"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="1.8"
                      d="M9.5 12.5l5.6-5.6a3.25 3.25 0 114.6 4.6l-7.42 7.42a5.25 5.25 0 11-7.42-7.42l7.08-7.08"
                    />
                  </svg>
                </button>

                <button
                  :disabled="disabled || streaming"
                  class="composer-toolbar-btn"
                  :title="t('chat.takePhoto')"
                  @click="openCameraDialog"
                >
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    class="h-[1.05rem] w-[1.05rem]"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="1.7"
                      d="M4.75 8.5A1.75 1.75 0 016.5 6.75h1.4a1 1 0 00.8-.4l.8-1.05a1 1 0 01.8-.4h3.4a1 1 0 01.8.4l.8 1.05a1 1 0 00.8.4h1.4a1.75 1.75 0 011.75 1.75v8.75A1.75 1.75 0 0119.5 19H6.5a1.75 1.75 0 01-1.75-1.75V8.5z"
                    />
                    <circle cx="13" cy="13" r="3.25" stroke-width="1.7" />
                  </svg>
                </button>

                <button
                  :disabled="disabled || streaming || isTranscribing"
                  class="composer-toolbar-btn"
                  :class="{ 'is-recording': isRecording }"
                  :title="isRecording ? t('chat.stopRecording') : t('chat.startRecording')"
                  @click="toggleRecording"
                >
                  <svg
                    v-if="isTranscribing"
                    class="h-4 w-4 animate-spin"
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
                  <svg
                    v-else
                    xmlns="http://www.w3.org/2000/svg"
                    class="h-[1.05rem] w-[1.05rem]"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="1.7"
                      d="M12 15.75a3.25 3.25 0 003.25-3.25v-4a3.25 3.25 0 10-6.5 0v4A3.25 3.25 0 0012 15.75z"
                    />
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="1.7"
                      d="M6.75 11.75a5.25 5.25 0 0010.5 0M12 17v3.25M9.25 20.25h5.5"
                    />
                  </svg>
                </button>

                <button
                  :disabled="disabled || streaming"
                  class="composer-toolbar-btn"
                  :title="t('chat.talkMode.title')"
                  @click="$emit('openTalkMode')"
                >
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    class="h-[1.05rem] w-[1.05rem]"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="1.7"
                      d="M6.92 4.25h2.08a1 1 0 01.96.73l.82 2.86a1 1 0 01-.42 1.11l-1.67 1.03a10.5 10.5 0 005.04 5.04l1.03-1.67a1 1 0 011.11-.42l2.86.82a1 1 0 01.73.96v2.08A1.75 1.75 0 0118.69 20h-.44C10.44 20 4 13.56 4 5.75v-.44A1.75 1.75 0 016.92 4.25z"
                    />
                  </svg>
                </button>

                <div v-if="isRecording" class="desktop-recording-indicator" aria-live="polite">
                  <span class="desktop-recording-indicator__dot"></span>
                  <span>{{ t('chat.recording') }} {{ recordingDuration }}s</span>
                </div>
              </div>

              <div class="desktop-toolbar-right">
                <div class="mode-chip-hover-shell mode-chip-hover-shell--research">
                  <button
                    class="mode-chip desktop-mode-chip mode-chip-research"
                    :class="{ 'is-active': chatStore.deepResearchEnabled }"
                    :aria-label="t('ui.deepResearchTitle')"
                    :aria-pressed="chatStore.deepResearchEnabled"
                    @click="toggleDeepResearch"
                  >
                    <svg
                      class="mode-chip__icon"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                    >
                      <circle cx="10.5" cy="10.5" r="4.75" stroke-width="1.7" />
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="1.7"
                        d="M14 14l4 4M16 5.25h3M17.5 3.75v3"
                      />
                    </svg>
                    <span class="mode-chip__label">{{ t('ui.deepResearchTitle') }}</span>
                  </button>
                  <div class="mode-info-card mode-info-card--research" aria-hidden="true">
                    <div class="mode-info-card__hero">
                      <div class="mode-info-card__hero-orb">
                        <svg fill="none" viewBox="0 0 24 24" stroke="currentColor">
                          <circle cx="10.5" cy="10.5" r="4.75" stroke-width="1.7" />
                          <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            stroke-width="1.7"
                            d="M14 14l4 4M16 5.25h3M17.5 3.75v3"
                          />
                        </svg>
                      </div>
                      <div class="mode-info-card__hero-meters" aria-hidden="true">
                        <span></span>
                        <span></span>
                        <span></span>
                      </div>
                      <span class="mode-info-card__state">
                        {{
                          chatStore.deepResearchEnabled
                            ? t('common.enabled', 'Enabled')
                            : t('common.disabled', 'Disabled')
                        }}
                      </span>
                    </div>
                    <div class="mode-info-card__title">{{ t('ui.deepResearchTitle') }}</div>
                    <p class="mode-info-card__description">
                      {{
                        t(
                          'chat.deepResearchHoverDescription',
                          'Launch a structured research workflow with retrieval, verification, and source-backed answers.'
                        )
                      }}
                    </p>
                    <div class="mode-info-card__chips">
                      <span
                        v-for="tag in deepResearchInfoTags"
                        :key="tag"
                        class="mode-info-card__chip"
                      >
                        {{ tag }}
                      </span>
                    </div>
                  </div>
                </div>
                <div class="mode-chip-hover-shell mode-chip-hover-shell--loop">
                  <button
                    class="mode-chip desktop-mode-chip mode-chip-loop"
                    :class="{ 'is-active-agent': settingsStore.agentMode }"
                    :aria-label="t('chat.taskLoop')"
                    :aria-pressed="settingsStore.agentMode"
                    :title="`${t('chat.taskLoop')} · ${t('agent.autoConfirm')}: ${ralphLoopAutoConfirmStateLabel}`"
                    @click="toggleAgentMode"
                    @contextmenu="handleAgentModeContextMenu"
                  >
                    <svg
                      class="mode-chip__icon"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                    >
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="1.7"
                        d="M9 4.75h6a1.75 1.75 0 011.75 1.75v10.75A1.75 1.75 0 0115 19H9a1.75 1.75 0 01-1.75-1.75V6.5A1.75 1.75 0 019 4.75z"
                      />
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="1.7"
                        d="M9.75 3h4.5M10.25 9h4M10.25 12h4M10.25 15h2.5"
                      />
                    </svg>
                    <span class="mode-chip__label">{{ t('chat.taskLoop') }}</span>
                  </button>
                  <div class="mode-info-card mode-info-card--loop" aria-hidden="true">
                    <div class="mode-info-card__hero">
                      <div class="mode-info-card__hero-orb">
                        <svg fill="none" viewBox="0 0 24 24" stroke="currentColor">
                          <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            stroke-width="1.7"
                            d="M9 4.75h6a1.75 1.75 0 011.75 1.75v10.75A1.75 1.75 0 0115 19H9a1.75 1.75 0 01-1.75-1.75V6.5A1.75 1.75 0 019 4.75z"
                          />
                          <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            stroke-width="1.7"
                            d="M9.75 3h4.5M10.25 9h4M10.25 12h4M10.25 15h2.5"
                          />
                        </svg>
                      </div>
                      <div class="mode-info-card__hero-meters" aria-hidden="true">
                        <span></span>
                        <span></span>
                        <span></span>
                      </div>
                      <span class="mode-info-card__state">
                        {{
                          settingsStore.agentMode
                            ? t('common.enabled', 'Enabled')
                            : t('common.disabled', 'Disabled')
                        }}
                      </span>
                    </div>
                    <div class="mode-info-card__title">{{ t('chat.taskLoop') }}</div>
                    <p class="mode-info-card__description">
                      {{
                        t(
                          'chat.ralphLoopHoverDescription',
                          'Let the agent plan, use tools, apply changes, and keep iterating until the task lands cleanly.'
                        )
                      }}
                    </p>
                    <div class="mode-info-card__chips">
                      <span
                        v-for="tag in ralphLoopInfoTags"
                        :key="tag"
                        class="mode-info-card__chip"
                      >
                        {{ tag }}
                      </span>
                    </div>
                  </div>
                </div>
                <div class="mode-chip-hover-shell mode-chip-hover-shell--report">
                  <button
                    class="mode-chip desktop-mode-chip mode-chip-report"
                    :aria-label="t('chat.analyzeReportShortcutTitle', 'Analysis Report')"
                    @click="handleAnalyzeReportShortcut"
                  >
                    <svg
                      class="mode-chip__icon"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                    >
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="1.7"
                        d="M7.75 4.75h6.5L18.25 8.75v8.5A1.75 1.75 0 0116.5 19h-8A1.75 1.75 0 016.75 17.25V6.5A1.75 1.75 0 018.5 4.75z"
                      />
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="1.7"
                        d="M10 11.25h4M10 14h5M10 16.75h3.25M14.25 4.75V8.5h3.75"
                      />
                    </svg>
                    <span class="mode-chip__label">{{
                      t('chat.analyzeReportShortcutTitle', 'Analysis Report')
                    }}</span>
                  </button>
                  <div class="mode-info-card mode-info-card--report" aria-hidden="true">
                    <div class="mode-info-card__hero">
                      <div class="mode-info-card__hero-orb">
                        <svg fill="none" viewBox="0 0 24 24" stroke="currentColor">
                          <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            stroke-width="1.7"
                            d="M7.75 4.75h6.5L18.25 8.75v8.5A1.75 1.75 0 0116.5 19h-8A1.75 1.75 0 016.75 17.25V6.5A1.75 1.75 0 018.5 4.75z"
                          />
                          <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            stroke-width="1.7"
                            d="M10 11.25h4M10 14h5M10 16.75h3.25M14.25 4.75V8.5h3.75"
                          />
                        </svg>
                      </div>
                      <div class="mode-info-card__hero-meters" aria-hidden="true">
                        <span></span>
                        <span></span>
                        <span></span>
                      </div>
                      <span class="mode-info-card__state">{{
                        t('chat.analyzeReportHoverState', 'Prompt template')
                      }}</span>
                    </div>
                    <div class="mode-info-card__title">
                      {{ t('chat.analyzeReportShortcutTitle', 'Analysis Report') }}
                    </div>
                    <p class="mode-info-card__description">
                      {{
                        t(
                          'chat.analyzeReportHoverDescription',
                          'Turn URLs, search results, or pasted text into a structured report with findings, comparisons, and recommendations.'
                        )
                      }}
                    </p>
                    <div class="mode-info-card__chips">
                      <span
                        v-for="tag in analyzeReportInfoTags"
                        :key="tag"
                        class="mode-info-card__chip"
                      >
                        {{ tag }}
                      </span>
                    </div>
                  </div>
                </div>
                <div class="mode-chip-hover-shell mode-chip-hover-shell--ui">
                  <button
                    class="mode-chip desktop-mode-chip mode-chip-ui"
                    :aria-label="t('chat.uiReviewShortcutTitle', 'UI Review')"
                    @click="handleUIReviewShortcut"
                  >
                    <svg
                      class="mode-chip__icon"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                    >
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="1.7"
                        d="M2.75 12s3.25-5.25 9.25-5.25S21.25 12 21.25 12s-3.25 5.25-9.25 5.25S2.75 12 2.75 12z"
                      />
                      <circle cx="12" cy="12" r="2.5" stroke-width="1.7" />
                    </svg>
                    <span class="mode-chip__label">{{
                      t('chat.uiReviewShortcutTitle', 'UI Review')
                    }}</span>
                  </button>
                  <div class="mode-info-card mode-info-card--ui" aria-hidden="true">
                    <div class="mode-info-card__hero">
                      <div class="mode-info-card__hero-orb">
                        <svg fill="none" viewBox="0 0 24 24" stroke="currentColor">
                          <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            stroke-width="1.7"
                            d="M2.75 12s3.25-5.25 9.25-5.25S21.25 12 21.25 12s-3.25 5.25-9.25 5.25S2.75 12 2.75 12z"
                          />
                          <circle cx="12" cy="12" r="2.5" stroke-width="1.7" />
                        </svg>
                      </div>
                      <div class="mode-info-card__hero-meters" aria-hidden="true">
                        <span></span>
                        <span></span>
                        <span></span>
                      </div>
                      <span class="mode-info-card__state">{{
                        t('chat.uiReviewHoverState', 'Prompt template')
                      }}</span>
                    </div>
                    <div class="mode-info-card__title">
                      {{ t('chat.uiReviewShortcutTitle', 'UI Review') }}
                    </div>
                    <p class="mode-info-card__description">
                      {{
                        t(
                          'chat.uiReviewHoverDescription',
                          'Audit a page or screenshot for visual quality, interaction clarity, and accessibility, then list concrete issues and fixes.'
                        )
                      }}
                    </p>
                    <div class="mode-info-card__chips">
                      <span v-for="tag in uiReviewInfoTags" :key="tag" class="mode-info-card__chip">
                        {{ tag }}
                      </span>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <div class="desktop-compose-main">
              <div class="desktop-textarea-wrap">
                <textarea
                  ref="textareaRef"
                  v-model="message"
                  :disabled="disabled"
                  :placeholder="placeholder"
                  wrap="off"
                  enterkeyhint="send"
                  class="chat-textarea desktop-chat-textarea w-full text-gray-900 dark:text-white resize-none disabled:opacity-50 disabled:cursor-not-allowed"
                  rows="1"
                  @keydown="handleKeydown"
                  @input="handleInput"
                  @paste="handlePaste"
                />
                <div class="desktop-textarea-actions">
                  <button
                    v-if="!isTranscribing && (!canSend || isDictating)"
                    class="desktop-inline-icon-btn"
                    :class="{ 'is-active': isDictating }"
                    :title="isDictating ? t('chat.stopDictation') : t('chat.startDictation')"
                    @click="toggleDictation"
                  >
                    <svg
                      xmlns="http://www.w3.org/2000/svg"
                      class="h-5 w-5"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                      stroke-width="2"
                    >
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        d="M19 11a7 7 0 01-7 7m0 0a7 7 0 01-7-7m7 7v4m0 0H8m4 0h4m-4-8a3 3 0 01-3-3V5a3 3 0 116 0v6a3 3 0 01-3 3z"
                      />
                    </svg>
                  </button>
                  <div v-else-if="isTranscribing" class="desktop-inline-icon-btn is-passive">
                    <svg class="h-5 w-5 animate-spin" fill="none" viewBox="0 0 24 24">
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
                  </div>

                  <button
                    v-if="canShowCancelButton"
                    class="desktop-cancel-btn desktop-inline-action flex-shrink-0 rounded-full flex items-center justify-center transition-all duration-200 cursor-pointer"
                    :title="t('common.cancel')"
                    @click="handleCancel"
                  >
                    <svg
                      xmlns="http://www.w3.org/2000/svg"
                      class="h-4 w-4 sm:h-5 sm:w-5"
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
                    v-if="canSend"
                    :disabled="!canSend"
                    class="chat-send-btn desktop-send-btn desktop-inline-action flex-shrink-0 rounded-full flex items-center justify-center disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
                    :class="{ 'chat-send-btn--ready': canSend }"
                    :title="streaming ? t('chat.sendDuringStream') : t('chat.send')"
                    @click="handleSend"
                  >
                    <svg
                      xmlns="http://www.w3.org/2000/svg"
                      class="chat-send-btn__icon h-4 w-4 sm:h-5 sm:w-5"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                    >
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="1.9"
                        :d="sendIconPath"
                      />
                    </svg>
                  </button>
                </div>
              </div>
            </div>
          </div>
        </template>
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
        v-if="isRecording && isCompact && !voiceMode"
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
  background: none;
}

/* No gradient on mobile - flush to bottom */
@media (max-width: 767px) {
  .chat-input-wrapper {
    background: none;
  }
}

:root.dark .chat-input-wrapper,
[data-theme='dark'] .chat-input-wrapper {
  background: none;
}

@media (max-width: 767px) {
  :root.dark .chat-input-wrapper,
  [data-theme='dark'] .chat-input-wrapper {
    background: none;
  }
}

/* Desktop: glass-card handles border. Mobile/compact: Tailwind border-t handles it. */

.chat-input-container {
  border: 1px solid rgba(221, 223, 226, 0.94);
  background: linear-gradient(180deg, rgba(250, 251, 253, 0.98), rgba(245, 247, 250, 0.96));
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.78),
    0 20px 42px -34px rgba(15, 23, 42, 0.28);
  backdrop-filter: blur(18px);
  transition:
    border-color 0.18s ease,
    box-shadow 0.18s ease;
}

.chat-input-desktop-shell {
  border-radius: 1.8rem;
}

.chat-input-floating-shell {
  border-radius: 1.55rem;
}

.desktop-composer-root {
  width: 100%;
}

.chat-input-container--desktop {
  padding-top: 0.34rem;
  padding-bottom: 0.12rem;
}

.desktop-composer-shell {
  display: flex;
  flex-direction: column;
  gap: 0.04rem;
}

.desktop-composer-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.56rem;
  min-height: 2.14rem;
  padding: 0 0.08rem 0.03rem;
  border-bottom: none;
}

.desktop-toolbar-left,
.desktop-toolbar-right {
  display: flex;
  align-items: center;
  gap: 0.46rem;
  flex-wrap: wrap;
}

.desktop-toolbar-left {
  flex: 1 1 auto;
  min-width: 0;
}

.desktop-recording-indicator {
  flex: 0 0 auto;
  display: inline-flex;
  align-items: center;
  gap: 0.38rem;
  min-height: 1.72rem;
  padding: 0 0.72rem;
  margin-inline-start: 0.16rem;
  border-radius: 999px;
  border: 1px solid rgba(248, 113, 113, 0.22);
  background: rgba(254, 242, 242, 0.94);
  color: rgb(220, 38, 38);
  font-size: 0.74rem;
  font-weight: 600;
  line-height: 1;
  white-space: nowrap;
}

.desktop-recording-indicator__dot {
  width: 0.42rem;
  height: 0.42rem;
  border-radius: 999px;
  background: rgb(239, 68, 68);
  animation: recording-pulse 1.2s ease-in-out infinite;
  flex-shrink: 0;
}

.desktop-toolbar-right {
  justify-content: flex-end;
  flex-shrink: 0;
}

.mode-chip-hover-shell {
  position: relative;
  display: inline-flex;
  isolation: isolate;
}

.mode-info-card {
  position: absolute;
  inset-inline-end: 0;
  bottom: calc(100% + 0.82rem);
  width: min(22rem, calc(100vw - 3rem));
  padding: 0.88rem;
  border-radius: 1.28rem;
  border: 1px solid rgba(203, 213, 225, 0.84);
  background:
    radial-gradient(circle at top right, rgba(var(--mode-card-accent-rgb), 0.16), transparent 46%),
    linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(248, 250, 252, 0.96));
  box-shadow:
    0 24px 46px -32px rgba(15, 23, 42, 0.24),
    0 14px 24px -20px rgba(var(--mode-card-shadow-rgb), 0.22);
  opacity: 0;
  visibility: hidden;
  pointer-events: none;
  transform: translateY(10px) scale(0.985);
  transform-origin: bottom right;
  transition:
    opacity 0.18s ease,
    transform 0.18s ease,
    visibility 0.18s ease;
  z-index: 24;
}

.mode-info-card::after {
  content: '';
  position: absolute;
  inset-inline-end: 1.2rem;
  bottom: -0.4rem;
  width: 0.82rem;
  height: 0.82rem;
  border-inline-end: 1px solid rgba(203, 213, 225, 0.84);
  border-bottom: 1px solid rgba(203, 213, 225, 0.84);
  background: rgba(255, 255, 255, 0.98);
  transform: rotate(45deg);
}

.mode-chip-hover-shell:hover .mode-info-card,
.mode-chip-hover-shell:focus-within .mode-info-card {
  opacity: 1;
  visibility: visible;
  transform: translateY(0) scale(1);
}

.mode-info-card--research {
  --mode-card-accent-rgb: 34, 197, 94;
  --mode-card-shadow-rgb: 16, 185, 129;
}

.mode-info-card--loop {
  --mode-card-accent-rgb: 59, 130, 246;
  --mode-card-shadow-rgb: 245, 158, 11;
}

.mode-info-card--report {
  --mode-card-accent-rgb: 245, 158, 11;
  --mode-card-shadow-rgb: 249, 115, 22;
}

.mode-info-card--ui {
  --mode-card-accent-rgb: 14, 165, 233;
  --mode-card-shadow-rgb: 20, 184, 166;
}

.mode-info-card__hero {
  position: relative;
  display: flex;
  align-items: center;
  gap: 0.82rem;
  min-height: 5.4rem;
  padding: 0.95rem 1rem;
  border-radius: 1rem;
  overflow: hidden;
  background:
    linear-gradient(135deg, rgba(255, 255, 255, 0.88), rgba(255, 255, 255, 0.64)),
    linear-gradient(160deg, rgba(var(--mode-card-accent-rgb), 0.14), rgba(255, 255, 255, 0.48));
  border: 1px solid rgba(226, 232, 240, 0.92);
}

.mode-info-card__hero::before,
.mode-info-card__hero::after {
  content: '';
  position: absolute;
  border-radius: 999px;
  pointer-events: none;
}

.mode-info-card__hero::before {
  top: -1.4rem;
  inset-inline-end: -0.6rem;
  width: 5rem;
  height: 5rem;
  background: rgba(var(--mode-card-accent-rgb), 0.16);
  filter: blur(2px);
}

.mode-info-card__hero::after {
  inset-inline-start: 0.9rem;
  bottom: -1.7rem;
  width: 6.6rem;
  height: 3.2rem;
  background: rgba(var(--mode-card-accent-rgb), 0.1);
  filter: blur(16px);
}

.mode-info-card__hero-orb {
  position: relative;
  z-index: 1;
  width: 3.2rem;
  height: 3.2rem;
  border-radius: 1rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: rgb(var(--mode-card-accent-rgb));
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.96), rgba(255, 255, 255, 0.7)),
    rgba(var(--mode-card-accent-rgb), 0.12);
  border: 1px solid rgba(var(--mode-card-accent-rgb), 0.18);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.38),
    0 12px 28px -20px rgba(var(--mode-card-accent-rgb), 0.34);
}

.mode-info-card__hero-orb svg {
  width: 1.46rem;
  height: 1.46rem;
}

.mode-info-card__hero-meters {
  position: relative;
  z-index: 1;
  flex: 1 1 auto;
  display: grid;
  gap: 0.44rem;
}

.mode-info-card__hero-meters span {
  display: block;
  height: 0.42rem;
  border-radius: 999px;
  background: linear-gradient(
    90deg,
    rgba(var(--mode-card-accent-rgb), 0.84),
    rgba(var(--mode-card-accent-rgb), 0.22)
  );
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.28);
}

.mode-info-card__hero-meters span:nth-child(1) {
  width: 78%;
}

.mode-info-card__hero-meters span:nth-child(2) {
  width: 62%;
  opacity: 0.82;
}

.mode-info-card__hero-meters span:nth-child(3) {
  width: 88%;
  opacity: 0.68;
}

.mode-info-card__state {
  position: relative;
  z-index: 1;
  flex-shrink: 0;
  align-self: flex-start;
  padding: 0.34rem 0.62rem;
  border-radius: 999px;
  border: 1px solid rgba(203, 213, 225, 0.86);
  background: rgba(255, 255, 255, 0.78);
  color: rgba(15, 23, 42, 0.8);
  font-size: 0.68rem;
  font-weight: 600;
  letter-spacing: 0.01em;
}

.mode-info-card__title {
  margin-top: 0.88rem;
  color: rgba(15, 23, 42, 0.96);
  font-size: 1rem;
  font-weight: 700;
  line-height: 1.25;
}

.mode-info-card__description {
  margin-top: 0.46rem;
  color: rgba(71, 85, 105, 0.95);
  font-size: 0.77rem;
  line-height: 1.55;
}

.mode-info-card__chips {
  margin-top: 0.82rem;
  display: flex;
  flex-wrap: wrap;
  gap: 0.48rem;
}

.mode-info-card__chip {
  padding: 0.38rem 0.62rem;
  border-radius: 999px;
  border: 1px solid rgba(203, 213, 225, 0.88);
  background: rgba(255, 255, 255, 0.82);
  color: rgba(30, 41, 59, 0.92);
  font-size: 0.68rem;
  font-weight: 600;
  line-height: 1;
  letter-spacing: 0.01em;
}

:root.dark .mode-info-card,
[data-theme='dark'] .mode-info-card,
html.dark .mode-info-card {
  border: 1px solid rgba(148, 163, 184, 0.2);
  background:
    radial-gradient(circle at top right, rgba(var(--mode-card-accent-rgb), 0.24), transparent 46%),
    linear-gradient(180deg, rgba(11, 18, 32, 0.98), rgba(15, 23, 42, 0.96));
  box-shadow:
    0 24px 46px -32px rgba(15, 23, 42, 0.72),
    0 14px 24px -20px rgba(var(--mode-card-shadow-rgb), 0.54);
}

:root.dark .mode-info-card::after,
[data-theme='dark'] .mode-info-card::after,
html.dark .mode-info-card::after {
  border-inline-end: 1px solid rgba(148, 163, 184, 0.2);
  border-bottom: 1px solid rgba(148, 163, 184, 0.2);
  background: rgba(15, 23, 42, 0.98);
}

html[dir='rtl'] .mode-info-card {
  transform-origin: bottom left;
}

:root.dark .mode-info-card__hero,
[data-theme='dark'] .mode-info-card__hero,
html.dark .mode-info-card__hero {
  background:
    linear-gradient(135deg, rgba(255, 255, 255, 0.08), rgba(255, 255, 255, 0.02)),
    linear-gradient(160deg, rgba(var(--mode-card-accent-rgb), 0.18), rgba(15, 23, 42, 0.04));
  border: 1px solid rgba(255, 255, 255, 0.08);
}

:root.dark .mode-info-card__hero::after,
[data-theme='dark'] .mode-info-card__hero::after,
html.dark .mode-info-card__hero::after {
  background: rgba(255, 255, 255, 0.06);
}

:root.dark .mode-info-card__hero-orb,
[data-theme='dark'] .mode-info-card__hero-orb,
html.dark .mode-info-card__hero-orb {
  color: rgb(255, 255, 255);
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.18), rgba(255, 255, 255, 0.04)),
    rgba(var(--mode-card-accent-rgb), 0.24);
  border: 1px solid rgba(255, 255, 255, 0.12);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.18),
    0 12px 28px -20px rgba(var(--mode-card-accent-rgb), 0.92);
}

:root.dark .mode-info-card__hero-meters span,
[data-theme='dark'] .mode-info-card__hero-meters span,
html.dark .mode-info-card__hero-meters span {
  background: linear-gradient(
    90deg,
    rgba(var(--mode-card-accent-rgb), 0.94),
    rgba(255, 255, 255, 0.22)
  );
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.18);
}

:root.dark .mode-info-card__state,
[data-theme='dark'] .mode-info-card__state,
html.dark .mode-info-card__state {
  border: 1px solid rgba(255, 255, 255, 0.12);
  background: rgba(255, 255, 255, 0.08);
  color: rgba(255, 255, 255, 0.92);
}

:root.dark .mode-info-card__title,
[data-theme='dark'] .mode-info-card__title,
html.dark .mode-info-card__title {
  color: rgba(248, 250, 252, 0.98);
}

:root.dark .mode-info-card__description,
[data-theme='dark'] .mode-info-card__description,
html.dark .mode-info-card__description {
  color: rgba(203, 213, 225, 0.92);
}

:root.dark .mode-info-card__chip,
[data-theme='dark'] .mode-info-card__chip,
html.dark .mode-info-card__chip {
  border: 1px solid rgba(255, 255, 255, 0.12);
  background: rgba(255, 255, 255, 0.06);
  color: rgba(241, 245, 249, 0.96);
}

@media (max-width: 767px) {
  .mode-info-card {
    display: none;
  }
}

.composer-mode-row {
  display: flex;
  align-items: center;
  gap: 0.55rem;
  overflow-x: auto;
  padding-bottom: 0.1rem;
  scrollbar-width: none;
}

.composer-mode-row::-webkit-scrollbar {
  display: none;
}

.compact-mode-section {
  display: grid;
  gap: 0.7rem;
}

.compact-mode-action {
  display: inline-flex;
  align-items: center;
  gap: 0.32rem;
  flex: 0 0 auto;
}

.compact-mode-info-toggle {
  width: 1.82rem;
  height: 1.82rem;
  border-radius: 999px;
  border: 1px solid rgba(216, 222, 229, 0.92);
  background: rgba(255, 255, 255, 0.96);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: rgb(100, 116, 139);
  cursor: pointer;
  transition:
    border-color 0.16s ease,
    background-color 0.16s ease,
    color 0.16s ease,
    transform 0.16s ease;
}

.compact-mode-info-toggle:hover {
  transform: translateY(-1px);
}

.compact-mode-info-toggle svg {
  width: 0.88rem;
  height: 0.88rem;
}

.compact-mode-info-toggle--research {
  border-color: rgba(74, 222, 128, 0.56);
  color: rgb(22, 163, 74);
  background: rgba(240, 253, 244, 0.98);
}

.compact-mode-info-toggle--research:hover {
  border-color: rgba(34, 197, 94, 0.8);
  background: rgba(220, 252, 231, 1);
  color: rgb(21, 128, 61);
}

.compact-mode-info-toggle--loop {
  border-color: rgba(165, 180, 252, 0.72);
  color: rgb(79, 70, 229);
  background: rgba(238, 242, 255, 0.98);
}

.compact-mode-info-toggle--loop:hover {
  border-color: rgba(99, 102, 241, 0.84);
  background: rgba(224, 231, 255, 1);
  color: rgb(67, 56, 202);
}

.compact-mode-info-toggle--report {
  border-color: rgba(251, 191, 36, 0.66);
  color: rgb(217, 119, 6);
  background: rgba(255, 251, 235, 0.98);
}

.compact-mode-info-toggle--report:hover {
  border-color: rgba(245, 158, 11, 0.84);
  background: rgba(254, 243, 199, 0.98);
  color: rgb(180, 83, 9);
}

.compact-mode-info-toggle--ui {
  border-color: rgba(56, 189, 248, 0.58);
  color: rgb(2, 132, 199);
  background: rgba(240, 249, 255, 0.98);
}

.compact-mode-info-toggle--ui:hover {
  border-color: rgba(14, 165, 233, 0.82);
  background: rgba(224, 242, 254, 1);
  color: rgb(3, 105, 161);
}

.compact-mode-info-card {
  position: relative;
  overflow: hidden;
  border-radius: 1.15rem;
  border: 1px solid rgba(203, 213, 225, 0.9);
  padding: 0.95rem 1rem 1rem;
  background:
    radial-gradient(
      circle at top right,
      rgba(var(--compact-mode-card-accent-rgb), 0.15),
      transparent 42%
    ),
    linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(248, 250, 252, 0.96));
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.74),
    0 18px 36px -28px rgba(15, 23, 42, 0.22);
}

.compact-mode-info-card--report {
  --compact-mode-card-accent-rgb: 245, 158, 11;
}

.compact-mode-info-card--research {
  --compact-mode-card-accent-rgb: 34, 197, 94;
}

.compact-mode-info-card--loop {
  --compact-mode-card-accent-rgb: 59, 130, 246;
}

.compact-mode-info-card--ui {
  --compact-mode-card-accent-rgb: 14, 165, 233;
}

.compact-mode-info-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.compact-mode-info-card__eyebrow {
  font-size: 0.66rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: rgba(71, 85, 105, 0.86);
}

.compact-mode-info-card__close {
  width: 1.7rem;
  height: 1.7rem;
  border-radius: 999px;
  border: 1px solid rgba(203, 213, 225, 0.86);
  background: rgba(255, 255, 255, 0.84);
  color: rgba(71, 85, 105, 0.9);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
}

.compact-mode-info-card__close svg {
  width: 0.82rem;
  height: 0.82rem;
}

.compact-mode-info-card__title {
  margin-top: 0.52rem;
  color: rgba(15, 23, 42, 0.96);
  font-size: 0.96rem;
  font-weight: 700;
  line-height: 1.3;
}

.compact-mode-info-card__description {
  margin-top: 0.42rem;
  color: rgba(71, 85, 105, 0.95);
  font-size: 0.78rem;
  line-height: 1.58;
}

.compact-mode-info-card__chips {
  margin-top: 0.78rem;
  display: flex;
  flex-wrap: wrap;
  gap: 0.45rem;
}

.compact-mode-info-card__chip {
  padding: 0.36rem 0.6rem;
  border-radius: 999px;
  border: 1px solid rgba(203, 213, 225, 0.9);
  background: rgba(255, 255, 255, 0.82);
  color: rgba(30, 41, 59, 0.92);
  font-size: 0.67rem;
  font-weight: 600;
  line-height: 1;
}

.composer-toolbar-btn {
  width: 1.62rem;
  height: 1.62rem;
  border-radius: 999px;
  border: 1px solid transparent;
  background: transparent;
  color: rgb(63, 63, 70);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  transition:
    color 0.16s ease,
    background-color 0.16s ease,
    border-color 0.16s ease,
    transform 0.16s ease;
  cursor: pointer;
}

.composer-toolbar-btn:hover {
  border-color: rgba(228, 228, 231, 0.98);
  color: rgb(24, 24, 27);
  background: rgba(255, 255, 255, 0.72);
  transform: translateY(-1px);
}

.composer-toolbar-btn.is-recording {
  border-color: rgba(248, 113, 113, 0.55);
  color: rgb(220, 38, 38);
  background: rgba(254, 242, 242, 0.92);
}

.composer-toolbar-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.mode-chip {
  height: 1.82rem;
  border-radius: 999px;
  border: 1px solid rgba(216, 222, 229, 0.96);
  background: rgba(255, 255, 255, 0.96);
  padding: 0 0.78rem;
  font-size: 0.75rem;
  font-weight: 600;
  color: rgb(63, 63, 70);
  display: inline-flex;
  align-items: center;
  gap: 0.38rem;
  white-space: nowrap;
  cursor: pointer;
  transition: all 0.16s ease;
  position: relative;
}

.mode-chip:hover {
  border-color: rgba(199, 208, 218, 1);
  background: rgba(250, 250, 250, 1);
}

.mode-chip__icon {
  width: 0.92rem;
  height: 0.92rem;
  flex-shrink: 0;
}

.mode-chip__label {
  line-height: 1;
}

.desktop-mode-chip {
  height: 1.72rem;
  padding: 0 0.66rem;
  border-radius: 0.82rem;
  font-size: 0.74rem;
  font-weight: 500;
  gap: 0.34rem;
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.88),
    0 10px 22px -24px rgba(15, 23, 42, 0.3);
}

.desktop-mode-chip .mode-chip__icon {
  width: 0.88rem;
  height: 0.88rem;
}

.mode-chip__status-dot {
  position: absolute;
  top: 0.24rem;
  inset-inline-end: 0.3rem;
  width: 0.34rem;
  height: 0.34rem;
  border-radius: 999px;
  background: rgb(148, 163, 184);
  box-shadow: 0 0 0 2px rgba(255, 255, 255, 0.98);
}

.mode-chip__status-dot.is-error {
  background: rgb(239, 68, 68);
}

.mode-chip__status-dot.is-pending {
  background: rgb(245, 158, 11);
}

.mode-chip__status-dot.is-none {
  background: rgb(148, 163, 184);
}

.mode-chip-research {
  color: rgb(71, 85, 105);
  border-color: rgba(203, 213, 225, 0.98);
  background: rgba(255, 255, 255, 0.94);
}

.mode-chip-research .mode-chip__icon {
  color: rgb(34, 197, 94);
}

.mode-chip-research:hover {
  color: rgb(22, 101, 52);
  border-color: rgba(74, 222, 128, 0.56);
  background: rgba(240, 253, 244, 0.78);
}

.mode-chip-research.is-active {
  color: rgb(21, 128, 61);
  border-color: rgba(34, 197, 94, 0.88);
  background: rgba(236, 253, 245, 0.98);
  box-shadow:
    inset 0 0 0 1px rgba(187, 247, 208, 0.65),
    0 8px 18px -18px rgba(22, 163, 74, 0.38);
}

.mode-chip-research.is-active .mode-chip__icon {
  color: rgb(21, 128, 61);
}

.mode-chip-research.is-active:hover {
  color: rgb(22, 101, 52);
  border-color: rgba(22, 163, 74, 0.94);
  background: rgba(220, 252, 231, 1);
}

.desktop-mode-chip.mode-chip-research {
  color: rgb(71, 85, 105);
  border-color: rgba(203, 213, 225, 0.98);
  background: rgba(255, 255, 255, 0.98);
}

.desktop-mode-chip.mode-chip-research .mode-chip__icon {
  color: rgb(34, 197, 94);
}

.desktop-mode-chip.mode-chip-research:hover {
  color: rgb(22, 101, 52);
  border-color: rgba(74, 222, 128, 0.56);
  background: rgba(240, 253, 244, 0.86);
}

.desktop-mode-chip.mode-chip-research.is-active {
  color: rgb(21, 128, 61);
  border-color: rgba(34, 197, 94, 0.9);
  background: rgba(236, 253, 245, 0.98);
  box-shadow:
    inset 0 0 0 1px rgba(187, 247, 208, 0.72),
    0 12px 24px -22px rgba(22, 163, 74, 0.42);
}

.desktop-mode-chip.mode-chip-research.is-active .mode-chip__icon {
  color: rgb(21, 128, 61);
}

.desktop-mode-chip.mode-chip-research.is-active:hover {
  color: rgb(22, 101, 52);
  border-color: rgba(22, 163, 74, 0.94);
  background: rgba(220, 252, 231, 1);
}

.mode-chip-loop {
  color: rgb(63, 63, 70);
  border-color: rgba(216, 222, 229, 0.96);
  background: rgba(255, 255, 255, 0.98);
}

.mode-chip-loop .mode-chip__icon {
  color: rgb(99, 102, 241);
}

.mode-chip-loop:hover {
  color: rgb(49, 46, 129);
  border-color: rgba(165, 180, 252, 0.76);
  background: rgba(238, 242, 255, 0.88);
}

.mode-chip-loop.is-active-agent {
  color: rgb(37, 99, 235);
  border-color: rgba(96, 165, 250, 0.82);
  background: rgba(239, 246, 255, 0.98);
  box-shadow:
    inset 0 0 0 1px rgba(191, 219, 254, 0.72),
    0 10px 22px -22px rgba(59, 130, 246, 0.52);
}

.mode-chip-loop.is-active-agent .mode-chip__icon {
  color: rgb(67, 56, 202);
}

.desktop-mode-chip.mode-chip-loop {
  color: rgb(63, 63, 70);
  border-color: rgba(216, 222, 229, 0.96);
  background: rgba(255, 255, 255, 0.98);
}

.desktop-mode-chip.mode-chip-loop .mode-chip__icon {
  color: rgb(99, 102, 241);
}

.desktop-mode-chip.mode-chip-loop:hover {
  color: rgb(49, 46, 129);
  border-color: rgba(165, 180, 252, 0.76);
  background: rgba(245, 247, 255, 1);
}

.desktop-mode-chip.mode-chip-loop.is-active-agent {
  color: rgb(29, 78, 216);
  border-color: rgba(96, 165, 250, 0.86);
  background: rgba(239, 246, 255, 0.98);
  box-shadow:
    inset 0 0 0 1px rgba(191, 219, 254, 0.78),
    0 12px 24px -22px rgba(59, 130, 246, 0.58);
}

.desktop-mode-chip.mode-chip-loop.is-active-agent .mode-chip__icon {
  color: rgb(67, 56, 202);
}

.desktop-mode-chip.mode-chip-loop.is-active-agent:hover {
  color: rgb(30, 64, 175);
  border-color: rgba(59, 130, 246, 0.92);
  background: rgba(219, 234, 254, 1);
}

.mode-chip-report {
  color: rgb(146, 64, 14);
  border-color: rgba(251, 191, 36, 0.66);
  background: rgba(255, 251, 235, 0.96);
}

.mode-chip-report .mode-chip__icon {
  color: rgb(217, 119, 6);
}

.mode-chip-report:hover {
  color: rgb(120, 53, 15);
  border-color: rgba(245, 158, 11, 0.8);
  background: rgba(254, 243, 199, 0.96);
}

.desktop-mode-chip.mode-chip-report {
  color: rgb(146, 64, 14);
  border-color: rgba(251, 191, 36, 0.66);
  background: rgba(255, 251, 235, 0.98);
}

.desktop-mode-chip.mode-chip-report .mode-chip__icon {
  color: rgb(217, 119, 6);
}

.desktop-mode-chip.mode-chip-report:hover {
  color: rgb(120, 53, 15);
  border-color: rgba(245, 158, 11, 0.84);
  background: rgba(254, 243, 199, 0.98);
}

.mode-chip-ui {
  color: rgb(8, 47, 73);
  border-color: rgba(56, 189, 248, 0.58);
  background: rgba(240, 249, 255, 0.96);
}

.mode-chip-ui .mode-chip__icon {
  color: rgb(2, 132, 199);
}

.mode-chip-ui:hover {
  color: rgb(12, 74, 110);
  border-color: rgba(14, 165, 233, 0.78);
  background: rgba(224, 242, 254, 0.98);
}

.desktop-mode-chip.mode-chip-ui {
  color: rgb(8, 47, 73);
  border-color: rgba(56, 189, 248, 0.58);
  background: rgba(240, 249, 255, 0.98);
}

.desktop-mode-chip.mode-chip-ui .mode-chip__icon {
  color: rgb(2, 132, 199);
}

.desktop-mode-chip.mode-chip-ui:hover {
  color: rgb(12, 74, 110);
  border-color: rgba(14, 165, 233, 0.82);
  background: rgba(224, 242, 254, 1);
}

.desktop-compose-main {
  display: flex;
  align-items: stretch;
}

.desktop-textarea-wrap {
  flex: 1;
  min-width: 0;
  position: relative;
  min-height: 2.66rem;
  border: 1px solid rgba(221, 223, 226, 0.98);
  border-radius: 1.32rem;
  background: rgba(252, 253, 255, 0.98);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.92),
    0 12px 28px -30px rgba(15, 23, 42, 0.2);
  overflow: hidden;
  transition:
    border-color 0.18s ease,
    box-shadow 0.18s ease;
}

.desktop-textarea-wrap:focus-within {
  border-color: rgba(59, 130, 246, 0.38);
  box-shadow:
    0 0 0 3px rgba(59, 130, 246, 0.12),
    inset 0 1px 0 rgba(255, 255, 255, 0.92),
    0 12px 28px -30px rgba(15, 23, 42, 0.2);
}

.desktop-chat-textarea {
  min-height: 2.66rem;
  height: 2.66rem;
  border: none;
  border-radius: inherit;
  background: transparent;
  box-shadow: none;
  padding-block: 0.5rem 0.46rem;
  padding-inline: 0.92rem;
  padding-inline-end: 4.15rem;
  font-size: 0.84rem;
  line-height: 1.24;
  overflow-y: hidden;
  overflow-x: auto;
  white-space: pre;
}

.desktop-chat-textarea::placeholder {
  color: rgba(161, 161, 170, 0.8);
  font-size: 0.84rem;
}

.desktop-textarea-actions {
  position: absolute;
  inset-inline-end: 0.58rem;
  top: 50%;
  bottom: auto;
  transform: translateY(-50%);
  display: flex;
  align-items: center;
  gap: 0.3rem;
}

.desktop-inline-icon-btn {
  width: 1.72rem;
  height: 1.72rem;
  border-radius: 999px;
  border: 1px solid transparent;
  background: transparent;
  color: rgb(82, 82, 91);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  transition:
    color 0.16s ease,
    background-color 0.16s ease,
    border-color 0.16s ease,
    transform 0.16s ease;
  cursor: pointer;
}

.desktop-inline-icon-btn:hover {
  color: rgb(24, 24, 27);
  background: rgba(244, 244, 245, 0.92);
  transform: translateY(-1px);
}

.desktop-inline-icon-btn.is-active {
  color: rgb(22, 163, 74);
  border-color: rgba(34, 197, 94, 0.28);
  background: rgba(220, 252, 231, 0.9);
  box-shadow: 0 12px 22px -20px rgba(22, 163, 74, 0.42);
}

.desktop-inline-icon-btn.is-passive {
  color: rgba(161, 161, 170, 0.95);
  cursor: default;
}

.desktop-inline-action {
  width: 1.72rem;
  height: 1.72rem;
  box-shadow: 0 12px 24px -20px rgba(15, 23, 42, 0.48);
}

.desktop-chat-textarea:focus {
  border-color: transparent;
  box-shadow: none;
  outline: none;
}

.desktop-chat-textarea:focus-visible {
  outline: none;
}

.compact-send-btn,
.desktop-send-btn {
  border: 1px solid rgba(96, 165, 250, 0.34);
  color: rgb(239, 246, 255);
  background: linear-gradient(180deg, #3b82f6, #2563eb);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.24),
    0 16px 28px -18px rgba(37, 99, 235, 0.58);
}

.compact-send-btn:hover:not(:disabled),
.desktop-send-btn:hover:not(:disabled) {
  border-color: rgba(147, 197, 253, 0.5);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.3),
    0 22px 36px -20px rgba(37, 99, 235, 0.68);
}

.compact-send-btn:disabled,
.desktop-send-btn:disabled {
  background: linear-gradient(180deg, rgba(226, 232, 240, 0.98), rgba(203, 213, 225, 0.98));
  color: rgba(100, 116, 139, 0.88);
  border-color: rgba(203, 213, 225, 0.68);
  box-shadow: none;
}

.desktop-cancel-btn {
  border: 1px solid rgba(252, 165, 165, 0.5);
  color: rgb(239, 68, 68);
  background: rgba(254, 242, 242, 0.96);
  box-shadow: none;
}

.desktop-cancel-btn:hover {
  background: rgba(254, 226, 226, 1);
}

.chat-input-container--compact:focus-within {
  border-color: rgba(201, 204, 211, 0.98);
  box-shadow:
    0 0 0 1px rgba(232, 234, 237, 0.94),
    0 24px 44px -34px rgba(15, 23, 42, 0.3);
}

:root.dark .chat-input-container,
[data-theme='dark'] .chat-input-container {
  border-color: rgba(71, 85, 105, 0.72);
  background: linear-gradient(180deg, rgba(15, 23, 42, 0.94), rgba(15, 23, 42, 0.88));
  box-shadow:
    inset 0 1px 0 rgba(148, 163, 184, 0.08),
    0 18px 36px -26px rgba(2, 6, 23, 0.72);
}

:root.dark .desktop-composer-toolbar,
[data-theme='dark'] .desktop-composer-toolbar {
  border-bottom-color: rgba(71, 85, 105, 0.82);
}

:root.dark .desktop-textarea-wrap,
[data-theme='dark'] .desktop-textarea-wrap {
  border-color: rgba(71, 85, 105, 0.76);
  background: rgba(15, 23, 42, 0.76);
  box-shadow:
    inset 0 1px 0 rgba(148, 163, 184, 0.08),
    0 16px 32px -28px rgba(2, 6, 23, 0.52);
}

:root.dark .desktop-textarea-wrap:focus-within,
[data-theme='dark'] .desktop-textarea-wrap:focus-within {
  border-color: rgba(56, 189, 248, 0.44);
  box-shadow:
    0 0 0 3px rgba(14, 165, 233, 0.16),
    inset 0 1px 0 rgba(148, 163, 184, 0.08),
    0 16px 32px -28px rgba(2, 6, 23, 0.52);
}

:root.dark .desktop-chat-textarea::placeholder,
[data-theme='dark'] .desktop-chat-textarea::placeholder {
  color: rgba(148, 163, 184, 0.72);
}

:root.dark .compact-send-btn,
:root.dark .desktop-send-btn,
[data-theme='dark'] .compact-send-btn,
[data-theme='dark'] .desktop-send-btn {
  border-color: rgba(147, 197, 253, 0.42);
  color: rgb(239, 246, 255);
  background: linear-gradient(180deg, #3b82f6, #2563eb);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.18),
    0 18px 32px -20px rgba(37, 99, 235, 0.72);
}

:root.dark .compact-send-btn:disabled,
:root.dark .desktop-send-btn:disabled,
[data-theme='dark'] .compact-send-btn:disabled,
[data-theme='dark'] .desktop-send-btn:disabled {
  background: linear-gradient(180deg, rgba(51, 65, 85, 0.96), rgba(30, 41, 59, 0.96));
  color: rgba(148, 163, 184, 0.88);
  border-color: rgba(71, 85, 105, 0.72);
  box-shadow: none;
}

:root.dark .composer-toolbar-btn,
[data-theme='dark'] .composer-toolbar-btn {
  border-color: transparent;
  background: transparent;
  color: rgb(148, 163, 184);
}

:root.dark .composer-toolbar-btn:hover,
[data-theme='dark'] .composer-toolbar-btn:hover {
  border-color: rgba(71, 85, 105, 0.74);
  color: rgb(226, 232, 240);
  background: rgba(30, 41, 59, 0.92);
}

:root.dark .composer-toolbar-btn.is-recording,
[data-theme='dark'] .composer-toolbar-btn.is-recording {
  border-color: rgba(248, 113, 113, 0.48);
  color: rgb(252, 165, 165);
  background: rgba(69, 10, 10, 0.42);
}

:root.dark .desktop-recording-indicator,
[data-theme='dark'] .desktop-recording-indicator {
  border-color: rgba(248, 113, 113, 0.28);
  background: rgba(69, 10, 10, 0.34);
  color: rgb(252, 165, 165);
}

:root.dark .mode-chip,
[data-theme='dark'] .mode-chip {
  border-color: rgba(71, 85, 105, 0.58);
  background: rgba(15, 23, 42, 0.58);
  color: rgb(148, 163, 184);
}

:root.dark .mode-chip__status-dot,
[data-theme='dark'] .mode-chip__status-dot {
  box-shadow: 0 0 0 2px rgba(15, 23, 42, 0.92);
}

:root.dark .desktop-mode-chip,
[data-theme='dark'] .desktop-mode-chip {
  box-shadow:
    inset 0 1px 0 rgba(148, 163, 184, 0.08),
    0 12px 26px -24px rgba(2, 6, 23, 0.72);
}

:root.dark .mode-chip-research,
[data-theme='dark'] .mode-chip-research {
  color: rgb(203, 213, 225);
  border-color: rgba(71, 85, 105, 0.72);
  background: rgba(15, 23, 42, 0.72);
}

:root.dark .mode-chip-research .mode-chip__icon,
[data-theme='dark'] .mode-chip-research .mode-chip__icon {
  color: rgb(74, 222, 128);
}

:root.dark .mode-chip-research:hover,
[data-theme='dark'] .mode-chip-research:hover {
  color: rgb(167, 243, 208);
  border-color: rgba(52, 211, 153, 0.62);
  background: rgba(6, 78, 59, 0.28);
}

:root.dark .mode-chip-research.is-active,
[data-theme='dark'] .mode-chip-research.is-active {
  border-color: rgba(52, 211, 153, 0.72);
  color: rgb(167, 243, 208);
  background: rgba(6, 78, 59, 0.42);
  box-shadow:
    inset 0 0 0 1px rgba(16, 185, 129, 0.16),
    0 12px 24px -22px rgba(16, 185, 129, 0.42);
}

:root.dark .mode-chip-research.is-active .mode-chip__icon,
[data-theme='dark'] .mode-chip-research.is-active .mode-chip__icon {
  color: rgb(110, 231, 183);
}

:root.dark .mode-chip-research.is-active:hover,
[data-theme='dark'] .mode-chip-research.is-active:hover {
  border-color: rgba(74, 222, 128, 0.84);
  background: rgba(6, 95, 70, 0.54);
}

:root.dark .desktop-mode-chip.mode-chip-research,
[data-theme='dark'] .desktop-mode-chip.mode-chip-research {
  color: rgb(203, 213, 225);
  border-color: rgba(71, 85, 105, 0.82);
  background: rgba(15, 23, 42, 0.8);
}

:root.dark .desktop-mode-chip.mode-chip-research.is-active,
[data-theme='dark'] .desktop-mode-chip.mode-chip-research.is-active {
  color: rgb(167, 243, 208);
  border-color: rgba(52, 211, 153, 0.72);
  background: rgba(6, 78, 59, 0.44);
  box-shadow:
    inset 0 0 0 1px rgba(16, 185, 129, 0.18),
    0 14px 28px -24px rgba(16, 185, 129, 0.4);
}

:root.dark .desktop-mode-chip.mode-chip-research.is-active .mode-chip__icon,
[data-theme='dark'] .desktop-mode-chip.mode-chip-research.is-active .mode-chip__icon {
  color: rgb(110, 231, 183);
}

:root.dark .desktop-mode-chip.mode-chip-research.is-active:hover,
[data-theme='dark'] .desktop-mode-chip.mode-chip-research.is-active:hover {
  border-color: rgba(74, 222, 128, 0.84);
  background: rgba(6, 95, 70, 0.54);
}

:root.dark .mode-chip-loop,
[data-theme='dark'] .mode-chip-loop {
  color: rgb(203, 213, 225);
  border-color: rgba(71, 85, 105, 0.72);
  background: rgba(30, 41, 59, 0.82);
}

:root.dark .mode-chip-loop .mode-chip__icon,
[data-theme='dark'] .mode-chip-loop .mode-chip__icon {
  color: rgb(165, 180, 252);
}

:root.dark .mode-chip-loop:hover,
[data-theme='dark'] .mode-chip-loop:hover {
  color: rgb(224, 231, 255);
  border-color: rgba(99, 102, 241, 0.48);
  background: rgba(30, 41, 59, 0.96);
}

:root.dark .mode-chip-loop.is-active-agent,
[data-theme='dark'] .mode-chip-loop.is-active-agent {
  border-color: rgba(96, 165, 250, 0.62);
  color: rgb(147, 197, 253);
  background: rgba(30, 64, 175, 0.3);
  box-shadow:
    inset 0 0 0 1px rgba(96, 165, 250, 0.16),
    0 12px 26px -24px rgba(59, 130, 246, 0.44);
}

:root.dark .mode-chip-loop.is-active-agent .mode-chip__icon,
[data-theme='dark'] .mode-chip-loop.is-active-agent .mode-chip__icon {
  color: rgb(196, 181, 253);
}

:root.dark .desktop-mode-chip.mode-chip-loop,
[data-theme='dark'] .desktop-mode-chip.mode-chip-loop {
  color: rgb(226, 232, 240);
  border-color: rgba(71, 85, 105, 0.82);
  background: rgba(30, 41, 59, 0.88);
}

:root.dark .desktop-mode-chip.mode-chip-loop.is-active-agent,
[data-theme='dark'] .desktop-mode-chip.mode-chip-loop.is-active-agent {
  border-color: rgba(96, 165, 250, 0.72);
  color: rgb(191, 219, 254);
  background: rgba(30, 64, 175, 0.34);
  box-shadow:
    inset 0 0 0 1px rgba(96, 165, 250, 0.18),
    0 14px 28px -24px rgba(59, 130, 246, 0.48);
}

:root.dark .desktop-mode-chip.mode-chip-loop.is-active-agent .mode-chip__icon,
[data-theme='dark'] .desktop-mode-chip.mode-chip-loop.is-active-agent .mode-chip__icon {
  color: rgb(196, 181, 253);
}

:root.dark .desktop-mode-chip.mode-chip-loop.is-active-agent:hover,
[data-theme='dark'] .desktop-mode-chip.mode-chip-loop.is-active-agent:hover {
  border-color: rgba(96, 165, 250, 0.86);
  background: rgba(30, 64, 175, 0.42);
}

:root.dark .compact-mode-info-toggle,
[data-theme='dark'] .compact-mode-info-toggle {
  border-color: rgba(71, 85, 105, 0.76);
  background: rgba(30, 41, 59, 0.9);
  color: rgb(203, 213, 225);
}

:root.dark .compact-mode-info-toggle--research,
[data-theme='dark'] .compact-mode-info-toggle--research {
  color: rgb(110, 231, 183);
  border-color: rgba(52, 211, 153, 0.42);
  background: rgba(6, 78, 59, 0.28);
}

:root.dark .compact-mode-info-toggle--research:hover,
[data-theme='dark'] .compact-mode-info-toggle--research:hover {
  color: rgb(167, 243, 208);
  border-color: rgba(74, 222, 128, 0.56);
  background: rgba(6, 95, 70, 0.38);
}

:root.dark .compact-mode-info-toggle--loop,
[data-theme='dark'] .compact-mode-info-toggle--loop {
  color: rgb(196, 181, 253);
  border-color: rgba(99, 102, 241, 0.42);
  background: rgba(30, 64, 175, 0.24);
}

:root.dark .compact-mode-info-toggle--loop:hover,
[data-theme='dark'] .compact-mode-info-toggle--loop:hover {
  color: rgb(224, 231, 255);
  border-color: rgba(129, 140, 248, 0.56);
  background: rgba(30, 64, 175, 0.34);
}

:root.dark .mode-chip-report,
[data-theme='dark'] .mode-chip-report {
  color: rgb(253, 230, 138);
  border-color: rgba(245, 158, 11, 0.42);
  background: rgba(120, 53, 15, 0.24);
}

:root.dark .mode-chip-report .mode-chip__icon,
[data-theme='dark'] .mode-chip-report .mode-chip__icon {
  color: rgb(251, 191, 36);
}

:root.dark .mode-chip-report:hover,
[data-theme='dark'] .mode-chip-report:hover {
  color: rgb(254, 243, 199);
  border-color: rgba(251, 191, 36, 0.54);
  background: rgba(146, 64, 14, 0.34);
}

:root.dark .desktop-mode-chip.mode-chip-report,
[data-theme='dark'] .desktop-mode-chip.mode-chip-report {
  color: rgb(253, 230, 138);
  border-color: rgba(245, 158, 11, 0.42);
  background: rgba(120, 53, 15, 0.24);
}

:root.dark .desktop-mode-chip.mode-chip-report:hover,
[data-theme='dark'] .desktop-mode-chip.mode-chip-report:hover {
  color: rgb(254, 243, 199);
  border-color: rgba(251, 191, 36, 0.58);
  background: rgba(146, 64, 14, 0.36);
}

:root.dark .compact-mode-info-toggle--report,
[data-theme='dark'] .compact-mode-info-toggle--report {
  color: rgb(251, 191, 36);
  border-color: rgba(245, 158, 11, 0.42);
  background: rgba(120, 53, 15, 0.28);
}

:root.dark .compact-mode-info-toggle--report:hover,
[data-theme='dark'] .compact-mode-info-toggle--report:hover {
  color: rgb(253, 230, 138);
  border-color: rgba(251, 191, 36, 0.56);
  background: rgba(146, 64, 14, 0.36);
}

:root.dark .mode-chip-ui,
[data-theme='dark'] .mode-chip-ui {
  color: rgb(186, 230, 253);
  border-color: rgba(14, 165, 233, 0.42);
  background: rgba(8, 47, 73, 0.28);
}

:root.dark .mode-chip-ui .mode-chip__icon,
[data-theme='dark'] .mode-chip-ui .mode-chip__icon {
  color: rgb(56, 189, 248);
}

:root.dark .mode-chip-ui:hover,
[data-theme='dark'] .mode-chip-ui:hover {
  color: rgb(224, 242, 254);
  border-color: rgba(56, 189, 248, 0.58);
  background: rgba(12, 74, 110, 0.34);
}

:root.dark .desktop-mode-chip.mode-chip-ui,
[data-theme='dark'] .desktop-mode-chip.mode-chip-ui {
  color: rgb(186, 230, 253);
  border-color: rgba(14, 165, 233, 0.42);
  background: rgba(8, 47, 73, 0.28);
}

:root.dark .desktop-mode-chip.mode-chip-ui:hover,
[data-theme='dark'] .desktop-mode-chip.mode-chip-ui:hover {
  color: rgb(224, 242, 254);
  border-color: rgba(56, 189, 248, 0.6);
  background: rgba(12, 74, 110, 0.38);
}

:root.dark .compact-mode-info-toggle--ui,
[data-theme='dark'] .compact-mode-info-toggle--ui {
  color: rgb(56, 189, 248);
  border-color: rgba(14, 165, 233, 0.42);
  background: rgba(8, 47, 73, 0.32);
}

:root.dark .compact-mode-info-toggle--ui:hover,
[data-theme='dark'] .compact-mode-info-toggle--ui:hover {
  color: rgb(125, 211, 252);
  border-color: rgba(56, 189, 248, 0.58);
  background: rgba(12, 74, 110, 0.4);
}

:root.dark .compact-mode-info-card,
[data-theme='dark'] .compact-mode-info-card {
  border-color: rgba(148, 163, 184, 0.2);
  background:
    radial-gradient(
      circle at top right,
      rgba(var(--compact-mode-card-accent-rgb), 0.26),
      transparent 44%
    ),
    linear-gradient(180deg, rgba(11, 18, 32, 0.98), rgba(15, 23, 42, 0.96));
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.04),
    0 22px 42px -28px rgba(15, 23, 42, 0.62);
}

:root.dark .compact-mode-info-card__eyebrow,
[data-theme='dark'] .compact-mode-info-card__eyebrow {
  color: rgba(148, 163, 184, 0.92);
}

:root.dark .compact-mode-info-card__close,
[data-theme='dark'] .compact-mode-info-card__close {
  border-color: rgba(255, 255, 255, 0.12);
  background: rgba(255, 255, 255, 0.06);
  color: rgba(226, 232, 240, 0.96);
}

:root.dark .compact-mode-info-card__title,
[data-theme='dark'] .compact-mode-info-card__title {
  color: rgba(248, 250, 252, 0.98);
}

:root.dark .compact-mode-info-card__description,
[data-theme='dark'] .compact-mode-info-card__description {
  color: rgba(203, 213, 225, 0.92);
}

:root.dark .compact-mode-info-card__chip,
[data-theme='dark'] .compact-mode-info-card__chip {
  border-color: rgba(255, 255, 255, 0.12);
  background: rgba(255, 255, 255, 0.06);
  color: rgba(241, 245, 249, 0.96);
}

:root.dark .desktop-inline-icon-btn,
[data-theme='dark'] .desktop-inline-icon-btn {
  color: rgb(148, 163, 184);
}

:root.dark .desktop-inline-icon-btn:hover,
[data-theme='dark'] .desktop-inline-icon-btn:hover {
  color: rgb(226, 232, 240);
  background: rgba(30, 41, 59, 0.92);
}

:root.dark .desktop-inline-icon-btn.is-active,
[data-theme='dark'] .desktop-inline-icon-btn.is-active {
  color: rgb(110, 231, 183);
  border-color: rgba(16, 185, 129, 0.22);
  background: rgba(6, 78, 59, 0.36);
}

:root.dark .desktop-inline-icon-btn.is-passive,
[data-theme='dark'] .desktop-inline-icon-btn.is-passive {
  color: rgba(148, 163, 184, 0.72);
}

:root.dark .desktop-cancel-btn,
[data-theme='dark'] .desktop-cancel-btn {
  border-color: rgba(248, 113, 113, 0.36);
  color: rgb(252, 165, 165);
  background: rgba(69, 10, 10, 0.36);
}

:root.dark .chat-input-container--compact:focus-within,
[data-theme='dark'] .chat-input-container--compact:focus-within {
  border-color: rgba(100, 116, 139, 0.9);
  box-shadow:
    0 0 0 1px rgba(51, 65, 85, 0.7),
    0 18px 32px -24px rgba(2, 6, 23, 0.82);
}

textarea {
  max-height: 200px;
  border-radius: var(--radius-lg);
  box-sizing: border-box;
}

.chat-textarea:not(.desktop-chat-textarea) {
  height: 40px;
  min-height: 40px;
  padding-top: 9px;
  padding-bottom: 9px;
  padding-inline: 0.75rem;
  padding-inline-end: 3.5rem;
  line-height: 20px;
  overflow-y: auto;
  margin: 0;
  display: block;
  vertical-align: top;
  border: 1px solid rgba(186, 203, 223, 0.72);
  background: rgba(255, 255, 255, 0.94);
}

:root.dark .chat-textarea:not(.desktop-chat-textarea),
[data-theme='dark'] .chat-textarea:not(.desktop-chat-textarea) {
  background: rgba(15, 23, 42, 0.68);
  border-color: rgba(71, 85, 105, 0.74);
}

.chat-textarea:not(.desktop-chat-textarea):focus {
  border-color: rgba(14, 165, 233, 0.5);
  box-shadow: 0 0 0 3px rgba(14, 165, 233, 0.16);
  outline: none;
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
  0%,
  100% {
    transform: scale(1);
    box-shadow: 0 0 0 0 rgba(34, 197, 94, 0.3);
  }
  50% {
    transform: scale(1.02);
    box-shadow: 0 0 12px 2px rgba(34, 197, 94, 0.25);
  }
}
.voice-breathing {
  animation: voice-breathing 1.2s ease-in-out infinite;
}

/* Voice choice panel animation */
.voice-choice-panel {
  animation: voice-choice-slide-in 0.2s ease-out;
}

.voice-choice-btn {
  border: 1px solid transparent;
}

.voice-choice-btn--transcribe {
  color: #38bdf8;
  background: rgba(14, 165, 233, 0.15);
  border-color: rgba(14, 165, 233, 0.3);
}

.voice-choice-btn--transcribe:hover {
  background: rgba(14, 165, 233, 0.25);
}

.voice-choice-btn--transcribe:active {
  background: rgba(14, 165, 233, 0.35);
}

.voice-choice-btn--send {
  color: #22c55e;
  background: rgba(34, 197, 94, 0.15);
  border-color: rgba(34, 197, 94, 0.3);
}

.voice-choice-btn--send:hover {
  background: rgba(34, 197, 94, 0.25);
}

.voice-choice-btn--send:active {
  background: rgba(34, 197, 94, 0.35);
}

.voice-choice-dismiss {
  color: #9ca3af;
}

.voice-choice-dismiss:hover {
  color: #4b5563;
  background: rgba(148, 163, 184, 0.25);
}

@keyframes voice-choice-slide-in {
  from {
    opacity: 0;
    transform: translateY(4px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@keyframes recording-pulse {
  0%,
  100% {
    opacity: 1;
    transform: scale(1);
  }
  50% {
    opacity: 0.55;
    transform: scale(0.92);
  }
}

/* Inline dictation green glow */
@keyframes dictation-pulse {
  0%,
  100% {
    box-shadow: 0 0 4px rgba(34, 197, 94, 0.4);
  }
  50% {
    box-shadow: 0 0 8px rgba(34, 197, 94, 0.6);
  }
}
.dictation-glow {
  animation: dictation-pulse 1.5s ease-in-out infinite;
  border-radius: 9999px;
}

.chat-send-btn {
  position: relative;
  overflow: hidden;
  border: 1px solid rgba(96, 165, 250, 0.34);
  color: var(--chat-send-fg, rgb(239, 246, 255));
  background: var(--chat-send-bg, linear-gradient(180deg, #3b82f6 0%, #2563eb 100%));
  box-shadow: var(
    --chat-send-shadow,
    inset 0 1px 0 rgba(255, 255, 255, 0.24),
    0 16px 28px -18px rgba(37, 99, 235, 0.58)
  );
  transition:
    transform 0.18s ease,
    box-shadow 0.22s ease,
    filter 0.18s ease,
    border-color 0.18s ease;
}

.chat-send-btn::before {
  content: '';
  position: absolute;
  inset: 1px;
  border-radius: inherit;
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.22), rgba(255, 255, 255, 0) 48%);
  opacity: 0.95;
  pointer-events: none;
}

.chat-send-btn:hover:not(:disabled) {
  transform: translateY(-1px) scale(1.01);
  border-color: rgba(191, 219, 254, 0.56);
  box-shadow: var(
    --chat-send-shadow-hover,
    inset 0 1px 0 rgba(255, 255, 255, 0.32),
    0 22px 36px -20px rgba(37, 99, 235, 0.74)
  );
  filter: saturate(1.05);
}

.chat-send-btn:active:not(:disabled) {
  transform: translateY(0) scale(0.97);
}

.chat-send-btn:focus-visible {
  outline: none;
  box-shadow:
    0 0 0 3px var(--chat-send-ring, rgba(59, 130, 246, 0.28)),
    var(
      --chat-send-shadow-focus,
      inset 0 1px 0 rgba(255, 255, 255, 0.28),
      0 20px 34px -20px rgba(37, 99, 235, 0.7)
    );
}

.chat-send-btn:disabled {
  filter: none;
}

.chat-send-btn__icon {
  position: relative;
  z-index: 1;
  transition: transform 0.2s ease;
}

.chat-send-btn:hover:not(:disabled) .chat-send-btn__icon {
  transform: translateY(-0.8px);
}

@keyframes send-ready-breathe {
  0%,
  100% {
    box-shadow: var(--chat-send-shadow, 0 10px 24px -16px rgba(15, 23, 42, 0.72));
  }
  50% {
    box-shadow: var(--chat-send-shadow-active, 0 14px 28px -14px rgba(15, 23, 42, 0.85));
  }
}

.chat-send-btn--ready:not(:hover):not(:active):not(:disabled) {
  animation: none;
}
</style>
