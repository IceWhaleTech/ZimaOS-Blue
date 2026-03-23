import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type {
  MediaCategory,
  MediaIntent,
  MediaModelInfo,
  MediaTask,
  DirectGenerateRequest,
} from '@/api/media'
import { classifyMediaIntent } from './useMediaIntent'
import { useChatStore } from '@/stores/chat'

type MediaApiModule = typeof import('@/api/media')
let mediaApiModulePromise: Promise<MediaApiModule> | null = null

function loadMediaApiModule(): Promise<MediaApiModule> {
  if (!mediaApiModulePromise) {
    mediaApiModulePromise = import('@/api/media')
  }
  return mediaApiModulePromise
}

async function listMediaModels() {
  const { listModels } = await loadMediaApiModule()
  return listModels()
}

async function classifyMediaRequest(...args: Parameters<MediaApiModule['classifyIntent']>) {
  const { classifyIntent } = await loadMediaApiModule()
  return classifyIntent(...args)
}

async function directMediaGenerate(req: DirectGenerateRequest) {
  const { directGenerate } = await loadMediaApiModule()
  return directGenerate(req)
}

async function fetchMediaTask(taskId: string) {
  const { getTask } = await loadMediaApiModule()
  return getTask(taskId)
}

function mergeIntentParams(
  preferred?: Record<string, any>,
  fallback?: Record<string, any>
): Record<string, any> | undefined {
  if (!preferred && !fallback) return undefined
  return {
    ...(fallback ?? {}),
    ...(preferred ?? {}),
  }
}

function cancelMediaTaskById(taskId: string) {
  void loadMediaApiModule()
    .then(({ cancelTask }) => cancelTask(taskId))
    .catch(() => {})
}

const MODEL_MEMORY_KEY = 'media-last-model'

// Module-level cache for media provider availability.
// Shared across all useMediaGenerate() instances.
let _mediaAvailable: boolean | null = null
let _mediaAvailableCheckedAt = 0
const AVAILABILITY_CACHE_TTL = 60_000 // 60s

function loadModelMemory(): Record<string, string> {
  try {
    return JSON.parse(localStorage.getItem(MODEL_MEMORY_KEY) || '{}')
  } catch {
    return {}
  }
}

function saveModelMemory(category: string, modelId: string) {
  const mem = loadModelMemory()
  mem[category] = modelId
  localStorage.setItem(MODEL_MEMORY_KEY, JSON.stringify(mem))
}

export function getLastModel(category: string): string {
  return loadModelMemory()[category] || ''
}

/**
 * Composable for media generation workflow:
 * classify → select model → generate → poll/stream until done.
 */
export function useMediaGenerate() {
  const { t } = useI18n()
  const intent = ref<MediaIntent | null>(null)
  const models = ref<MediaModelInfo[]>([])
  const alternativeModels = ref<MediaModelInfo[]>([]) // models for alternative_category (e.g. kf2v)
  const selectedModel = ref('')
  const task = ref<MediaTask | null>(null)
  const generating = ref(false)
  const error = ref('')
  const showPanel = ref(false)
  const ambiguous = ref(false) // true when confidence < 0.7

  // Store image files for reference_images (i2v, i2i, kf2v)
  let pendingImageFiles: File[] = []

  let abortController: AbortController | null = null

  const isTerminal = computed(() => {
    const s = task.value?.status
    return s === 'succeeded' || s === 'failed' || s === 'cancelled'
  })

  /**
   * Classify a message locally (zero-latency), then fetch matching models from server.
   * Returns true if media intent was detected (including ambiguous).
   * When confidence < 0.7, sets ambiguous=true for disambiguation UI.
   */
  async function classify(
    message: string,
    hasImages = false,
    imageCount = 0,
    locale = '',
    imageFiles: File[] = []
  ): Promise<boolean> {
    reset()

    // Check if any media providers are active (cached, TTL 60s).
    // If not, skip intent detection entirely so the message goes to chat.
    const now = Date.now()
    if (_mediaAvailable === null || now - _mediaAvailableCheckedAt > AVAILABILITY_CACHE_TTL) {
      try {
        const allModels = await listMediaModels()
        _mediaAvailable = allModels.length > 0
      } catch {
        _mediaAvailable = false
      }
      _mediaAvailableCheckedAt = now
    }
    if (!_mediaAvailable) return false

    // Store image files for later use in generate()
    pendingImageFiles = imageFiles

    // Client-side classification first (instant)
    const localIntent = classifyMediaIntent(message, hasImages, imageCount, locale)
    if (!localIntent) return false

    // Low confidence (0.4-0.7): show disambiguation prompt
    if (localIntent.confidence < 0.7) {
      intent.value = localIntent
      ambiguous.value = true
      showPanel.value = true
      return true
    }

    intent.value = localIntent
    ambiguous.value = false

    // Fetch models from server for the detected category
    try {
      const resp = await classifyMediaRequest(message, hasImages, imageCount, locale)
      if (resp.intent) {
        intent.value = {
          ...resp.intent,
          params: mergeIntentParams(localIntent.params, resp.intent.params),
        }
      }
      if (resp.models?.length) {
        models.value = resp.models
      } else {
        // Fallback: fetch all models and filter
        const all = await listMediaModels()
        models.value = all.filter((m) => m.category === localIntent.category)
        // Also fetch alternative category models if present
        if (localIntent.alternative_category) {
          alternativeModels.value = all.filter(
            (m) => m.category === localIntent.alternative_category
          )
        }
      }
    } catch {
      // Server classify failed — use local intent, try to get models
      try {
        const all = await listMediaModels()
        models.value = all.filter((m) => m.category === localIntent.category)
        if (localIntent.alternative_category) {
          alternativeModels.value = all.filter(
            (m) => m.category === localIntent.alternative_category
          )
        }
      } catch {
        // No models available
      }
    }

    // Restore last-used model
    const last = getLastModel(localIntent.category)
    if (last && models.value.some((m) => m.id === last)) {
      selectedModel.value = last
    } else {
      const first = models.value[0]
      if (first) selectedModel.value = first.id
    }

    // Auto-submit when there's exactly one model and no alternative category —
    // no point making the user confirm when there's nothing to choose.
    if (
      models.value.length === 1 &&
      !intent.value?.alternative_category &&
      alternativeModels.value.length === 0
    ) {
      await generate()
      return true
    }

    showPanel.value = true
    return true
  }

  /**
   * Confirm an ambiguous intent — user explicitly wants media generation.
   * Loads models and transitions to the normal param panel.
   */
  async function confirmAmbiguous() {
    if (!intent.value) return
    ambiguous.value = false

    // Now load models for the confirmed category
    const category = intent.value.category
    try {
      const resp = await classifyMediaRequest(
        intent.value.prompt,
        intent.value.has_image,
        intent.value.image_count,
        ''
      )
      if (resp.models?.length) {
        models.value = resp.models
      } else {
        const all = await listMediaModels()
        models.value = all.filter((m) => m.category === category)
      }
    } catch {
      try {
        const all = await listMediaModels()
        models.value = all.filter((m) => m.category === category)
      } catch {
        // No models available
      }
    }

    // Restore last-used model
    const last = getLastModel(category)
    if (last && models.value.some((m) => m.id === last)) {
      selectedModel.value = last
    } else {
      const first = models.value[0]
      if (first) selectedModel.value = first.id
    }

    // Auto-submit when there's only one model — nothing to choose
    if (
      models.value.length === 1 &&
      !intent.value?.alternative_category &&
      alternativeModels.value.length === 0
    ) {
      await generate()
      return
    }
  }

  /**
   * Switch between primary and alternative category (e.g. i2v ↔ kf2v).
   * Swaps models lists and updates intent.category.
   */
  function switchCategory(newCategory: MediaCategory) {
    if (!intent.value || newCategory === intent.value.category) return

    // Swap models
    const tmp = models.value
    models.value = alternativeModels.value
    alternativeModels.value = tmp

    // Update intent category and alternative
    const oldCategory = intent.value.category
    intent.value = {
      ...intent.value,
      category: newCategory,
      alternative_category: oldCategory,
    }

    // Restore last-used model for the new category
    const last = getLastModel(newCategory)
    if (last && models.value.some((m) => m.id === last)) {
      selectedModel.value = last
    } else {
      const first = models.value[0]
      if (first) {
        selectedModel.value = first.id
      } else {
        selectedModel.value = ''
      }
    }
  }

  /**
   * Submit generation request. Backend creates messages in the conversation.
   * Frontend refreshes messages to pick up the server-created [media_task:xxx] message.
   * MediaPlaceholder then polls for task status on its own — works from any device.
   */
  async function generate(messageId?: string) {
    if (!intent.value) return

    generating.value = true
    error.value = ''

    // Convert pending image files to base64 data URLs for reference_images
    let referenceImages: string[] | undefined
    if (pendingImageFiles.length > 0) {
      referenceImages = await Promise.all(
        pendingImageFiles.map(
          (file) =>
            new Promise<string>((resolve) => {
              const reader = new FileReader()
              reader.onload = () => resolve(reader.result as string)
              reader.readAsDataURL(file)
            })
        )
      )
    }

    const chatStore = useChatStore()

    // Ensure conversation exists
    if (!chatStore.currentConversationId) {
      await chatStore.createConversation()
    }

    const req: DirectGenerateRequest = {
      category: intent.value.category,
      prompt: intent.value.prompt,
      model: selectedModel.value || undefined,
      params: intent.value.params,
      reference_images: referenceImages,
      conversation_id: chatStore.currentConversationId || undefined,
      message_id: messageId,
      source:
        typeof intent.value.params?.source === 'string' && intent.value.params.source.trim()
          ? intent.value.params.source
          : 'web',
    }

    try {
      const resp = await directMediaGenerate(req)

      // Remember model choice
      if (selectedModel.value) {
        saveModelMemory(intent.value.category, selectedModel.value)
      }

      task.value = {
        id: resp.task_id,
        status: 'pending' as const,
        type: intent.value.category.includes('v') ? 'video' : 'image',
        category: intent.value.category,
        provider: '',
        model: resp.model || selectedModel.value,
        progress: 0,
        message_id: resp.message_id,
        created_at: new Date().toISOString(),
      }

      // Dismiss the param panel
      showPanel.value = false

      // Refresh messages from server — the backend created user + assistant messages
      // The assistant message contains [media_task:xxx] which MediaPlaceholder will detect and poll
      if (chatStore.currentConversationId) {
        await chatStore.fetchMessages(chatStore.currentConversationId)
      }
      // Refresh sidebar to pick up backend-set title
      chatStore.fetchConversations()

      generating.value = false
    } catch (e) {
      error.value = e instanceof Error ? e.message : String(e)
      generating.value = false
      // Refresh sidebar — backend sets title before task creation, so it may exist even on error.
      // If backend didn't set it either, set it locally as fallback.
      chatStore.fetchConversations().then(() => {
        const conv = chatStore.currentConversation
        if (conv && !conv.title && intent.value) {
          const catLabel = t(`media.${intent.value.category}`, intent.value.category)
          const prompt = intent.value.prompt
          const snippet = prompt.length > 20 ? prompt.slice(0, 20) + '...' : prompt
          conv.title = catLabel + ': ' + snippet
        }
      })
    }
  }

  /**
   * Poll task status (fallback when SSE is not available).
   */
  async function pollTask(taskId: string, intervalMs = 2000) {
    const poll = async () => {
      try {
        const t = await fetchMediaTask(taskId)
        task.value = t
        if (t.status === 'succeeded' || t.status === 'failed' || t.status === 'cancelled') {
          generating.value = false
          return
        }
        setTimeout(poll, intervalMs)
      } catch {
        generating.value = false
      }
    }
    poll()
  }

  function cancel() {
    abortController?.abort()
    abortController = null
    generating.value = false
    // Cancel the backend task if one is active
    if (task.value?.id && !isTerminal.value) {
      cancelMediaTaskById(task.value.id)
    }
  }

  /**
   * Retry a failed/cancelled task with the same parameters.
   */
  async function retry(messageId?: string) {
    if (!intent.value) return
    // Reset task state but keep intent, models, and selectedModel
    task.value = null
    error.value = ''
    await generate(messageId)
  }

  function reset() {
    cancel()
    intent.value = null
    models.value = []
    alternativeModels.value = []
    selectedModel.value = ''
    task.value = null
    error.value = ''
    showPanel.value = false
    ambiguous.value = false
    pendingImageFiles = []
  }

  function dismissPanel() {
    showPanel.value = false
  }

  return {
    intent,
    models,
    alternativeModels,
    selectedModel,
    task,
    generating,
    error,
    showPanel,
    ambiguous,
    isTerminal,
    classify,
    confirmAmbiguous,
    switchCategory,
    generate,
    pollTask,
    cancel,
    retry,
    reset,
    dismissPanel,
  }
}
