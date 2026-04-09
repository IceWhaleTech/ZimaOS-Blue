import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { reactive, ref } from 'vue'
import ChatView from '@/views/ChatView.vue'
import { i18n, setLocale } from '@/i18n'

const mocks = vi.hoisted(() => ({
  chatStore: {
    awaitingConfirmation: false,
    preTTFTCancelActive: false,
    pendingApproval: null,
    pendingExecApproval: null,
    pendingQuestion: null,
    contextTrimInfo: null,
    processTrace: [] as Array<Record<string, unknown>>,
    currentConversation: {
      id: 'conv-1',
      title: 'Test conversation',
      created_at: '2026-03-08T00:00:00.000Z',
      updated_at: '2026-03-08T00:00:00.000Z',
    },
    currentConversationId: 'conv-1',
    error: null,
    hasMoreMessages: false,
    isMultiSelectMode: false,
    isPreTTFT: false,
    isRecovering: false,
    isStreamInterrupted: false,
    loading: false,
    loadingMore: false,
    messages: [] as Array<Record<string, unknown>>,
    modelPreference: 'auto',
    recentTodoCompletion: null as null | { messageId: string; todoCardId?: string },
    searching: false,
    securityBlocked: null,
    selectedMessageIds: new Set<string>(),
    sending: false,
    streamError: null,
    streamProgress: null,
    statusStartedAt: 0,
    statusSummary: null,
    streamUIState: { phase: 'idle' },
    streaming: false,
    streamingContent: '',
    executingConversationIds: [] as string[],
    sortedConversations: [] as Array<Record<string, unknown>>,
    toolExecuting: false,
    toolExecutingCommands: [] as string[],
    toolExecutingNames: [] as string[],
    toolResults: [] as Array<Record<string, unknown>>,
    toolSandboxAvailable: false,
    trialExhausted: false,
    fetchConversations: vi.fn(),
    selectConversation: vi.fn(),
    getMessageMetadata: vi.fn(),
    sendMessage: vi.fn(),
    recoverPendingConfirmations: vi.fn(),
    loadMoreMessages: vi.fn(),
    searchConversations: vi.fn(),
    setModelPreference: vi.fn(),
    createConversation: vi.fn(),
    deleteConversation: vi.fn(),
    pinConversation: vi.fn(),
    unpinConversation: vi.fn(),
    continueMessage: vi.fn(),
    regenerateMessage: vi.fn(),
    editMessageAndResubmit: vi.fn(),
    injectMessage: vi.fn(),
    warmupConversation: vi.fn(),
    resetWarmup: vi.fn(),
    clearError: vi.fn(),
    clearStreamError: vi.fn(),
    clearSecurityBlocked: vi.fn(),
    cancelStreaming: vi.fn(),
    retryInterruptedStreamRecovery: vi.fn(),
    cancelPreTTFT: vi.fn(),
    deleteSelectedMessages: vi.fn(),
    enterMultiSelectMode: vi.fn(),
    exitMultiSelectMode: vi.fn(),
  },
  settingsStore: {
    agentAutoConfirm: false,
    agentMode: false,
    experimentalAgentcoreRunnerEnabled: false,
    agentcoreRunnerStatus: null as null | Record<string, unknown>,
    agentcoreRunnerLastRun: null as null | Record<string, unknown>,
    showToolDetails: true,
    fetchTools: vi.fn(),
    fetchAgentcoreRunnerStatus: vi.fn(),
    updateFromPoolProviders: vi.fn(),
    setAgentAutoConfirm: vi.fn(),
    setAgentMode: vi.fn(),
    setShowToolDetails: vi.fn(),
  },
  providerPoolStore: {
    activeProviders: [] as unknown[],
    cloudProviders: [] as unknown[],
    enabledProviders: [] as unknown[],
    hasCloudProviders: false,
    hasLocalProviders: false,
    localProviders: [] as unknown[],
    models: [] as unknown[],
    providers: [] as unknown[],
    routingMode: 'auto',
    trialProviders: [] as unknown[],
    trialQuota: null as null | Record<string, unknown>,
    getProviderDisplayName: vi.fn((providerId: string) => providerId),
    fetchProviders: vi.fn(),
    fetchRoutingMode: vi.fn(),
    fetchTrialQuota: vi.fn(),
    setRoutingMode: vi.fn(),
  },
  deepResearchJobsStore: {
    activeJobs: [] as Array<Record<string, unknown>>,
    jobMap: {} as Record<string, unknown>,
    loading: false,
    hydrated: true,
    pendingFocusJobId: null as string | null,
    fetchActiveJobs: vi.fn(),
    handleGlobalEvent: vi.fn(),
    applyJobSnapshot: vi.fn(),
    openJob: vi.fn(),
    cancelJob: vi.fn(),
    consumePendingFocusJobId: vi.fn(),
  },
  taskProjectionsStore: {
    currentTasks: [] as Array<Record<string, unknown>>,
    currentActiveTasks: [] as Array<Record<string, unknown>>,
    currentTerminalTasks: [] as Array<Record<string, unknown>>,
    backgroundTasks: [] as Array<Record<string, unknown>>,
    recentOutcome: null as null | Record<string, unknown>,
    loading: false,
    hydrated: true,
    hasActiveTasks: false,
    refreshNow: vi.fn(),
    setConversation: vi.fn(),
    performTaskAction: vi.fn(),
    cancelTask: vi.fn(),
    resumeTask: vi.fn(),
    openTask: vi.fn(),
    dismissRecentOutcome: vi.fn(),
    stopPolling: vi.fn(),
  },
  mediaGenerate: {
    showPanel: { value: false },
    intent: { value: null },
    models: { value: [] as unknown[] },
    selectedModel: { value: '' },
    generating: { value: false },
    ambiguous: { value: false },
    task: { value: null },
    classify: vi.fn(),
    generate: vi.fn(),
    reset: vi.fn(),
    confirmAmbiguous: vi.fn(),
    cancel: vi.fn(),
    switchCategory: vi.fn(),
  },
  cardActionSubmit: vi.fn(),
  speechGetStatus: vi.fn(),
  authFetch: vi.fn(),
  onSSEEvent: vi.fn(),
  offSSEEvent: vi.fn(),
  notificationStore: {
    success: vi.fn(),
    error: vi.fn(),
    info: vi.fn(),
    remove: vi.fn(),
  },
}))

mocks.chatStore = reactive(mocks.chatStore)
mocks.settingsStore = reactive(mocks.settingsStore)
mocks.providerPoolStore = reactive(mocks.providerPoolStore)
mocks.deepResearchJobsStore = reactive(mocks.deepResearchJobsStore)
mocks.taskProjectionsStore = reactive(mocks.taskProjectionsStore)
mocks.mediaGenerate = reactive(mocks.mediaGenerate)

const helpers = vi.hoisted(() => ({
  asAsyncSFCModule(component: Record<string, unknown>) {
    return {
      __esModule: true,
      __isTeleport: false,
      __isKeepAlive: false,
      default: component,
      ...component,
    }
  },
}))

vi.mock('@/stores/chat', () => ({
  useChatStore: () => mocks.chatStore,
}))

vi.mock('@/stores/settings', () => ({
  useSettingsStore: () => mocks.settingsStore,
}))

vi.mock('@/stores/providerPool', () => ({
  useProviderPoolStore: () => mocks.providerPoolStore,
}))

vi.mock('@/stores/deepResearchJobs', () => ({
  useDeepResearchJobsStore: () => mocks.deepResearchJobsStore,
}))

vi.mock('@/stores/taskProjections', () => ({
  useTaskProjectionsStore: () => mocks.taskProjectionsStore,
}))

vi.mock('@/stores/notification', () => ({
  useNotificationStore: () => mocks.notificationStore,
}))

vi.mock('@/api/chat', () => ({
  cardActionApi: {
    submit: (...args: unknown[]) => mocks.cardActionSubmit(...args),
  },
}))

vi.mock('@/api/client', () => ({
  authFetch: (...args: unknown[]) => mocks.authFetch(...args),
}))

vi.mock('@/api/voice', () => ({
  ttsAudioManager: {
    stop: vi.fn(),
  },
  streamingTTSManager: {
    setLocale: vi.fn(),
    stop: vi.fn(),
    streamAndPlay: vi.fn(),
    reset: vi.fn(),
    streamText: vi.fn(),
    play: vi.fn(),
    onComplete: null as null | (() => void),
  },
}))

vi.mock('@/api/speech', () => ({
  speechApi: {
    getStatus: (...args: unknown[]) => mocks.speechGetStatus(...args),
  },
}))

vi.mock('@/composables/useEventStream', () => ({
  onSSEEvent: (...args: unknown[]) => mocks.onSSEEvent(...args),
  offSSEEvent: (...args: unknown[]) => mocks.offSSEEvent(...args),
}))

vi.mock('@/composables/useKeyboardShortcuts', () => ({
  useChatShortcuts: vi.fn(),
}))

vi.mock('@/composables/useMediaGenerate', () => ({
  useMediaGenerate: () => mocks.mediaGenerate,
}))

vi.mock('@/components/ConversationList.vue', () =>
  helpers.asAsyncSFCModule({
    name: 'ConversationList',
    props: {
      conversations: { type: Array, default: () => [] },
      executingConversationIds: { type: Array, default: () => [] },
    },
    emits: ['select', 'create', 'delete', 'search', 'pin', 'unpin'],
    template: `
      <div
        class="conversation-list-stub"
        :data-executing-ids="(executingConversationIds || []).join(',')"
      >
        <button
          v-for="conversation in conversations"
          :key="conversation.id"
          class="conversation-select-stub"
          @click="$emit('select', conversation.id)"
        >
          {{ conversation.id }}
        </button>
      </div>
    `,
  })
)

vi.mock('@/components/ChatInput.vue', () =>
  helpers.asAsyncSFCModule({
    name: 'ChatInput',
    props: {
      disabled: { type: Boolean, default: false },
      streaming: { type: Boolean, default: false },
      canCancel: { type: Boolean, default: false },
      showInlineCancel: { type: Boolean, default: true },
    },
    emits: ['cancel'],
    template: `
      <div
        class="chat-input-stub"
        :data-disabled="String(disabled)"
        :data-streaming="String(streaming)"
        :data-can-cancel="String(canCancel)"
        :data-show-inline-cancel="String(showInlineCancel !== false)"
      >
        <button
          v-if="canCancel && showInlineCancel !== false"
          type="button"
          data-testid="chat-input-inline-cancel"
          @click="$emit('cancel')"
        >
          Stop generating
        </button>
      </div>
    `,
  })
)

vi.mock('@/components/onboarding/PresetQuestions.vue', () =>
  helpers.asAsyncSFCModule({
    name: 'PresetQuestions',
    template: '<div class="preset-questions-stub" />',
  })
)

vi.mock('@/components/VirtualScroll.vue', () =>
  helpers.asAsyncSFCModule({
    name: 'VirtualScroll',
    template: '<div class="virtual-scroll-stub"><slot /></div>',
  })
)

vi.mock('@/components/chat/TalkMode.vue', () =>
  helpers.asAsyncSFCModule({
    name: 'TalkMode',
    template: '<div class="talk-mode-stub" />',
  })
)

vi.mock('@/components/ToolApprovalDialog.vue', () =>
  helpers.asAsyncSFCModule({
    name: 'ToolApprovalDialog',
    template: '<div class="tool-approval-stub" />',
  })
)

vi.mock('@/components/ExecApprovalDialog.vue', () =>
  helpers.asAsyncSFCModule({
    name: 'ExecApprovalDialog',
    template: '<div class="exec-approval-stub" />',
  })
)

vi.mock('@/components/MediaParamPanel.vue', () =>
  helpers.asAsyncSFCModule({
    name: 'MediaParamPanel',
    template: '<div class="media-param-panel-stub" />',
  })
)

vi.mock('@/components/AgentTaskPanel.vue', () =>
  helpers.asAsyncSFCModule({
    name: 'AgentTaskPanel',
    template: '<div class="agent-task-panel-stub" />',
  })
)

vi.mock('@/components/DeepResearchTaskDock.vue', () =>
  helpers.asAsyncSFCModule({
    name: 'DeepResearchTaskDock',
    setup() {
      return { store: mocks.deepResearchJobsStore, expanded: ref(false) }
    },
    template: `
      <section
        v-if="Array.isArray(store.activeJobs) && store.activeJobs.length > 0"
        data-testid="deep-research-task-dock"
        class="deep-research-task-dock-stub"
      >
        <button
          type="button"
          data-testid="deep-research-task-dock-toggle"
          @click="expanded = !expanded"
        >
          Toggle
        </button>
        <button
          v-if="expanded && store.activeJobs[0] && store.activeJobs[0].conversation_id"
          type="button"
          :data-testid="'deep-research-task-dock-view-' + store.activeJobs[0].job_id"
          @click="store.openJob(store.activeJobs[0].job_id, store.activeJobs[0].conversation_id)"
        >
          View result
        </button>
        <button
          v-if="expanded && store.activeJobs[0] && store.activeJobs[0].job_id"
          type="button"
          :data-testid="'deep-research-task-dock-cancel-' + store.activeJobs[0].job_id"
          @click="store.cancelJob(store.activeJobs[0].job_id)"
        >
          Cancel
        </button>
      </section>
    `,
  })
)

vi.mock('@/components/UserTaskProjectionCard.vue', () =>
  helpers.asAsyncSFCModule({
    name: 'UserTaskProjectionCard',
    props: {
      task: { type: Object, default: null },
    },
    emits: ['action', 'open', 'navigate'],
    template: `
      <div class="agent-task-panel-stub">
        <button
          v-if="task && task.detail_href"
          class="task-projection-card-navigate-stub"
          @click="$emit('navigate', task.detail_href)"
        >
          Open run details
        </button>
        <button
          v-if="task && task.actions && Array.isArray(task.actions.items) && task.actions.items[0]"
          class="task-projection-card-action-stub"
          @click="$emit('action', task, task.actions.items[0].id)"
        >
          {{ task.actions.items[0].label }}
        </button>
      </div>
    `,
  })
)

vi.mock('@/components/UserTaskProjectionDock.vue', () =>
  helpers.asAsyncSFCModule({
    name: 'UserTaskProjectionDock',
    props: {
      tasks: { type: Array, default: () => [] },
    },
    emits: ['action', 'open', 'navigate'],
    template: `
      <section
        v-if="Array.isArray(tasks) && tasks.length > 0"
        data-testid="task-projection-dock"
        class="task-projection-dock-stub"
      >
        <button
          v-if="Array.isArray(tasks) && tasks[0] && tasks[0].detail_href"
          class="task-projection-dock-navigate-stub"
          @click="$emit('navigate', tasks[0].detail_href)"
        >
          Open run details
        </button>
        <button
          v-if="
            Array.isArray(tasks) &&
            tasks[0] &&
            tasks[0].actions &&
            Array.isArray(tasks[0].actions.items) &&
            tasks[0].actions.items[0]
          "
          class="task-projection-dock-action-stub"
          @click="$emit('action', tasks[0], tasks[0].actions.items[0].id)"
        >
          {{ tasks[0].actions.items[0].label }}
        </button>
      </section>
    `,
  })
)

const localStorageMock = (() => {
  let store: Record<string, string> = {}
  return {
    getItem: (key: string) => (key in store ? store[key] : null),
    setItem: (key: string, value: string) => {
      store[key] = String(value)
    },
    removeItem: (key: string) => {
      delete store[key]
    },
    clear: () => {
      store = {}
    },
  }
})()

vi.stubGlobal('localStorage', localStorageMock)
vi.stubGlobal(
  'matchMedia',
  vi.fn().mockImplementation(() => ({
    matches: false,
    media: '',
    onchange: null,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }))
)
vi.stubGlobal(
  'ResizeObserver',
  class {
    observe() {}
    unobserve() {}
    disconnect() {}
  }
)

const WEB_FETCH_URL = 'https://www.reddit.com/r/test'
const WEB_FETCH_CARD_ID = 'web-fetch-https%3A%2F%2Fwww.reddit.com%2Fr%2Ftest'
const BROWSER_CARD_ID = 'browser-https%3A%2F%2Fwww.reddit.com%2Fr%2Ftest'

function makeAssistantMessage(content: string, id = 'msg-1', extra: Record<string, unknown> = {}) {
  return {
    id,
    conversation_id: 'conv-1',
    role: 'assistant' as const,
    content,
    created_at: '2026-03-08T00:00:00.000Z',
    ...extra,
  }
}

function makeTypelessBlock(payload: Record<string, unknown>) {
  return ['```typeless', JSON.stringify(payload), '```'].join('\n')
}

function makeAgentcoreRunnerLastRun(overrides: Record<string, unknown> = {}) {
  return {
    id: 'runner-run-1',
    created_at: '2026-04-04T08:00:00.000Z',
    reason: 'manual_optimize',
    candidate_id: 'candidate-7',
    eval_run_id: 'eval-9',
    optimization_surface: 'skill_revision',
    runner_protocol: 'agentcore/v1',
    runner_stop_reason: 'completed',
    runner_duration_ms: 4200,
    runner_response_text: 'Runner proposed a cache optimization patch.',
    runner_transcript: [
      {
        direction: 'runner',
        method: 'response',
        text: 'Runner proposed a cache optimization patch.',
      },
      {
        direction: 'runner',
        method: 'note',
        text: 'Validated the optimization candidate against the latest baseline.',
      },
    ],
    ...overrides,
  }
}

async function mountChatViewWithMessagesHarness(
  messages: Array<{
    id: string
    content: string
    role?: 'assistant' | 'user'
    todo_card_id?: string
    extra?: Record<string, unknown>
  }>
) {
  mocks.chatStore.messages = messages.map((message) => ({
    id: message.id,
    conversation_id: 'conv-1',
    role: message.role ?? 'assistant',
    content: message.content,
    created_at: '2026-03-08T00:00:00.000Z',
    ...(message.todo_card_id ? { todo_card_id: message.todo_card_id } : {}),
    ...(message.extra || {}),
  }))

  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/chat', component: { template: '<div />' } },
      { path: '/settings', component: { template: '<div />' } },
      { path: '/security', component: { template: '<div />' } },
      { path: '/operations/harness/:id', component: { template: '<div />' } },
    ],
  })
  router.push('/chat')
  await router.isReady()

  const wrapper = mount(ChatView, {
    global: {
      plugins: [i18n, router],
      stubs: {
        Teleport: true,
        Transition: true,
      },
    },
  })

  await flushPromises()
  await vi.dynamicImportSettled()
  await flushPromises()
  mountedWrappers.push(wrapper)
  return { wrapper, router }
}

async function mountChatViewWithMessages(
  messages: Array<{
    id: string
    content: string
    role?: 'assistant' | 'user'
    todo_card_id?: string
    extra?: Record<string, unknown>
  }>
) {
  const { wrapper } = await mountChatViewWithMessagesHarness(messages)
  return wrapper
}

const mountedWrappers: Array<{ unmount: () => void }> = []

async function mountChatView(content: string) {
  return mountChatViewWithMessages([{ id: 'msg-1', content }])
}

function findButtonByText(wrapper: ReturnType<typeof mount>, text: string) {
  return wrapper.findAll('button').find((button) => button.text().includes(text))
}

async function expandCardById(wrapper: ReturnType<typeof mount>, cardId: string) {
  const toggle = wrapper.get(`[id="${cardId}"] button`)
  if (toggle.attributes('aria-expanded') !== 'true') {
    await toggle.trigger('click')
    await flushPromises()
    await vi.dynamicImportSettled()
    await flushPromises()
  }
}

describe('ChatView page-level card actions', () => {
  beforeAll(async () => {
    await setLocale('zh-CN')
    await setLocale('en-US')
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      const wrapper = mountedWrappers.pop()
      try {
        wrapper?.unmount()
      } catch {
        // ignore duplicate unmounts from tests that already cleaned up explicitly
      }
    }
    vi.useRealTimers()
  })

  beforeEach(() => {
    delete (globalThis as Record<string, unknown>).__zima_chat_card_disclosure_state_v1__

    Object.defineProperty(window, 'innerWidth', { value: 1280, writable: true, configurable: true })
    Object.defineProperty(window.navigator, 'userAgent', { value: 'desktop', configurable: true })
    Object.defineProperty(window.navigator, 'platform', { value: 'MacIntel', configurable: true })

    localStorageMock.clear()
    i18n.global.locale.value = 'en-US'

    mocks.chatStore.awaitingConfirmation = false
    mocks.chatStore.preTTFTCancelActive = false
    mocks.chatStore.pendingApproval = null
    mocks.chatStore.pendingExecApproval = null
    mocks.chatStore.pendingQuestion = null
    mocks.chatStore.contextTrimInfo = null
    mocks.chatStore.processTrace = []
    mocks.chatStore.currentConversation = {
      id: 'conv-1',
      title: 'Test conversation',
      created_at: '2026-03-08T00:00:00.000Z',
      updated_at: '2026-03-08T00:00:00.000Z',
    }
    mocks.chatStore.currentConversationId = 'conv-1'
    mocks.chatStore.error = null
    mocks.chatStore.hasMoreMessages = false
    mocks.chatStore.isMultiSelectMode = false
    mocks.chatStore.isPreTTFT = false
    mocks.chatStore.isRecovering = false
    mocks.chatStore.loading = false
    mocks.chatStore.loadingMore = false
    mocks.chatStore.messages = []
    mocks.chatStore.modelPreference = 'auto'
    mocks.chatStore.recentTodoCompletion = null
    mocks.chatStore.searching = false
    mocks.chatStore.securityBlocked = null
    mocks.chatStore.selectedMessageIds = new Set<string>()
    mocks.chatStore.sending = false
    mocks.chatStore.streamError = null
    mocks.chatStore.streamProgress = null
    mocks.chatStore.statusStartedAt = 0
    mocks.chatStore.statusSummary = null
    mocks.chatStore.streamUIState = { phase: 'idle' }
    mocks.chatStore.streaming = false
    mocks.chatStore.streamingContent = ''
    mocks.chatStore.executingConversationIds = []
    mocks.chatStore.sortedConversations = []
    mocks.chatStore.toolExecuting = false
    mocks.chatStore.toolExecutingCommands = []
    mocks.chatStore.toolExecutingNames = []
    mocks.chatStore.toolResults = []
    mocks.chatStore.toolSandboxAvailable = false
    mocks.chatStore.trialExhausted = false
    mocks.chatStore.fetchConversations.mockReset().mockResolvedValue(undefined)
    mocks.chatStore.selectConversation.mockReset().mockResolvedValue(undefined)
    mocks.chatStore.getMessageMetadata.mockReset().mockReturnValue(null)
    mocks.chatStore.sendMessage.mockReset().mockResolvedValue(undefined)
    mocks.chatStore.recoverPendingConfirmations.mockReset()
    mocks.chatStore.loadMoreMessages.mockReset()
    mocks.chatStore.searchConversations.mockReset()
    mocks.chatStore.setModelPreference.mockReset()
    mocks.chatStore.createConversation.mockReset()
    mocks.chatStore.deleteConversation.mockReset()
    mocks.chatStore.pinConversation.mockReset()
    mocks.chatStore.unpinConversation.mockReset()
    mocks.chatStore.continueMessage.mockReset()
    mocks.chatStore.regenerateMessage.mockReset()
    mocks.chatStore.injectMessage.mockReset()
    mocks.chatStore.warmupConversation.mockReset()
    mocks.chatStore.resetWarmup.mockReset()
    mocks.chatStore.clearError.mockReset()
    mocks.chatStore.clearStreamError.mockReset()
    mocks.chatStore.clearSecurityBlocked.mockReset()
    mocks.chatStore.cancelStreaming.mockReset()
    mocks.chatStore.cancelPreTTFT.mockReset()
    mocks.chatStore.deleteSelectedMessages.mockReset()
    mocks.chatStore.enterMultiSelectMode.mockReset()
    mocks.chatStore.exitMultiSelectMode.mockReset()

    mocks.settingsStore.agentAutoConfirm = false
    mocks.settingsStore.agentMode = false
    mocks.settingsStore.experimentalAgentcoreRunnerEnabled = false
    mocks.settingsStore.agentcoreRunnerStatus = null
    mocks.settingsStore.agentcoreRunnerLastRun = null
    mocks.settingsStore.showToolDetails = true
    mocks.settingsStore.fetchTools.mockReset().mockResolvedValue(undefined)
    mocks.settingsStore.fetchAgentcoreRunnerStatus.mockReset().mockResolvedValue(undefined)
    mocks.settingsStore.updateFromPoolProviders.mockReset()
    mocks.settingsStore.setAgentAutoConfirm.mockReset()
    mocks.settingsStore.setAgentMode.mockReset()
    mocks.settingsStore.setShowToolDetails.mockReset()

    mocks.providerPoolStore.activeProviders = []
    mocks.providerPoolStore.cloudProviders = []
    mocks.providerPoolStore.enabledProviders = []
    mocks.providerPoolStore.hasCloudProviders = false
    mocks.providerPoolStore.hasLocalProviders = false
    mocks.providerPoolStore.localProviders = []
    mocks.providerPoolStore.models = []
    mocks.providerPoolStore.providers = []
    mocks.providerPoolStore.routingMode = 'auto'
    mocks.providerPoolStore.trialProviders = []
    mocks.providerPoolStore.trialQuota = null
    mocks.providerPoolStore.getProviderDisplayName
      .mockReset()
      .mockImplementation((providerId: string) => providerId)
    mocks.providerPoolStore.fetchProviders.mockReset().mockResolvedValue(undefined)
    mocks.providerPoolStore.fetchRoutingMode.mockReset().mockResolvedValue(undefined)
    mocks.providerPoolStore.fetchTrialQuota.mockReset().mockResolvedValue(undefined)
    mocks.deepResearchJobsStore.activeJobs = []
    mocks.taskProjectionsStore.currentTasks = []
    mocks.taskProjectionsStore.currentActiveTasks = []
    mocks.taskProjectionsStore.currentTerminalTasks = []
    mocks.taskProjectionsStore.backgroundTasks = []
    mocks.taskProjectionsStore.recentOutcome = null
    mocks.taskProjectionsStore.refreshNow.mockReset().mockResolvedValue(undefined)
    mocks.taskProjectionsStore.setConversation.mockReset().mockResolvedValue(undefined)
    mocks.taskProjectionsStore.performTaskAction.mockReset().mockResolvedValue(undefined)
    mocks.taskProjectionsStore.cancelTask.mockReset().mockResolvedValue(undefined)
    mocks.taskProjectionsStore.resumeTask.mockReset().mockResolvedValue(undefined)
    mocks.taskProjectionsStore.openTask.mockReset().mockResolvedValue(undefined)
    mocks.taskProjectionsStore.dismissRecentOutcome.mockReset()
    mocks.taskProjectionsStore.stopPolling.mockReset()
    mocks.deepResearchJobsStore.fetchActiveJobs.mockReset().mockResolvedValue(undefined)
    mocks.deepResearchJobsStore.handleGlobalEvent.mockReset()
    mocks.deepResearchJobsStore.applyJobSnapshot.mockReset()
    mocks.deepResearchJobsStore.openJob.mockReset().mockResolvedValue(undefined)
    mocks.deepResearchJobsStore.cancelJob.mockReset().mockResolvedValue(undefined)
    mocks.deepResearchJobsStore.consumePendingFocusJobId.mockReset()
    mocks.providerPoolStore.setRoutingMode.mockReset().mockResolvedValue(undefined)

    mocks.mediaGenerate.showPanel.value = false
    mocks.mediaGenerate.intent.value = null
    mocks.mediaGenerate.models.value = []
    mocks.mediaGenerate.selectedModel.value = ''
    mocks.mediaGenerate.generating.value = false
    mocks.mediaGenerate.ambiguous.value = false
    mocks.mediaGenerate.task.value = null
    mocks.mediaGenerate.classify.mockReset()
    mocks.mediaGenerate.generate.mockReset()
    mocks.mediaGenerate.reset.mockReset()
    mocks.mediaGenerate.confirmAmbiguous.mockReset()
    mocks.mediaGenerate.cancel.mockReset()
    mocks.mediaGenerate.switchCategory.mockReset()

    mocks.cardActionSubmit.mockReset()
    mocks.speechGetStatus.mockReset().mockResolvedValue({ data: {} })

    mocks.authFetch.mockReset().mockResolvedValue({})
    mocks.onSSEEvent.mockReset()
    mocks.offSSEEvent.mockReset()

    mocks.notificationStore.success.mockReset()
    mocks.notificationStore.error.mockReset()
    mocks.notificationStore.info.mockReset()
    mocks.notificationStore.remove.mockReset()
  })

  it('localizes the awaiting confirmation indicator', async () => {
    i18n.global.locale.value = 'zh-CN'
    mocks.chatStore.awaitingConfirmation = true

    const wrapper = await mountChatViewWithMessages([
      { id: 'msg-confirm', content: 'Please confirm' },
    ])

    expect(wrapper.text()).toContain('等待你的确认以继续')
    expect(wrapper.text()).not.toContain('Waiting for your confirmation to continue')

    i18n.global.locale.value = 'en-US'
  }, 10000)

  it('renders localized copy for request build failures', async () => {
    mocks.chatStore.streamError = 'requestBuildFailed'

    const wrapper = await mountChatView('Idle state')

    expect(wrapper.text()).toContain(
      'The upstream relay failed to build this request. This usually means the request shape or tool arguments were invalid.'
    )
  })

  it('shows the work-details toggle near the input area and keeps it clickable', async () => {
    mocks.settingsStore.showToolDetails = false

    const wrapper = await mountChatViewWithMessages([
      { id: 'msg-details', content: 'Need more detail' },
    ])

    const toggle = wrapper.find(
      '.chat-thread-actions button.chat-thread-detail-btn:not(.chat-thread-routing-btn)'
    )
    expect(toggle.exists()).toBe(true)

    await toggle.trigger('click')

    expect(mocks.settingsStore.setShowToolDetails).toHaveBeenCalledWith(true)
  })

  it('shows the routing mode control beside work details', async () => {
    const activeProvider = {
      id: 'openai',
      type: 'builtin',
      enabled: true,
      status: 'active',
    } as Record<string, unknown>

    mocks.providerPoolStore.providers = [activeProvider]
    mocks.providerPoolStore.enabledProviders = [activeProvider]
    mocks.providerPoolStore.activeProviders = [activeProvider]
    mocks.providerPoolStore.cloudProviders = [activeProvider]
    mocks.providerPoolStore.hasCloudProviders = true

    const wrapper = await mountChatViewWithMessages([
      { id: 'msg-routing', content: 'Need routing access' },
    ])

    const actions = wrapper.get('.chat-thread-actions')
    const detailButton = actions
      .findAll('button')
      .find(
        (button) =>
          button.classes().includes('chat-thread-detail-btn') &&
          !button.classes().includes('chat-thread-routing-btn')
      )
    expect(detailButton?.exists()).toBe(true)

    const routingButton = actions.find('button.chat-thread-routing-btn')
    expect(routingButton.exists()).toBe(true)

    await routingButton.trigger('click')
    await flushPromises()

    expect(wrapper.find('.routing-menu-floating').exists()).toBe(true)
  })

  it('renders the latest todo checklist status above the composer', async () => {
    const scrollIntoViewMock = vi.fn()
    Object.defineProperty(Element.prototype, 'scrollIntoView', {
      value: scrollIntoViewMock,
      configurable: true,
    })

    const wrapper = await mountChatViewWithMessages([
      {
        id: 'msg-old',
        content: '- [ ] 旧任务\n- [ ] 旧验证',
        todo_card_id: 'todo-checklist-msg-old',
      },
      {
        id: 'msg-latest',
        content:
          '- [x] 梳理 ChatView/ChatInput 与 todo 状态来源\n- [ ] 实现输入框上方的活跃 todo 状态展示与样式\n- [ ] 补上或更新前端测试，验证状态展示逻辑\n\n继续执行第二步。',
      },
    ])

    const panel = wrapper.get('[data-testid="active-todo-panel"]')
    expect(panel.text()).toContain('1 out of 3 tasks completed')
    expect(panel.text()).toContain('梳理 ChatView/ChatInput 与 todo 状态来源')
    expect(panel.text()).toContain('实现输入框上方的活跃 todo 状态展示与样式')
    expect(panel.text()).toContain('补上或更新前端测试，验证状态展示逻辑')
    expect(panel.text()).not.toContain('旧任务')
    expect(wrapper.find('.active-todo-panel__list').exists()).toBe(true)

    await wrapper.get('[data-testid="active-todo-panel-jump"]').trigger('click')
    await flushPromises()

    expect(scrollIntoViewMock).toHaveBeenCalled()
    expect(wrapper.get('[data-message-id="msg-latest"]').classes()).toContain('is-todo-focused')

    await wrapper.get('[data-testid="active-todo-panel-toggle"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('.active-todo-panel__list').exists()).toBe(false)
    expect(localStorageMock.getItem('zima.chat.active_todo_collapsed.v1')).toBe('1')

    await wrapper.get('[data-testid="active-todo-panel-toggle"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('.active-todo-panel__list').exists()).toBe(true)
    expect(localStorageMock.getItem('zima.chat.active_todo_collapsed.v1')).toBeNull()

    const dockChildren = Array.from(wrapper.get('.chat-input-dock').element.children)
    const panelIndex = dockChildren.findIndex(
      (child) =>
        child instanceof HTMLElement &&
        (child.matches('[data-testid="active-todo-panel"]') ||
          !!child.querySelector('[data-testid="active-todo-panel"]'))
    )
    const inputIndex = dockChildren.findIndex(
      (child) => child instanceof HTMLElement && child.classList.contains('chat-input-stub')
    )

    expect(panelIndex).toBeGreaterThanOrEqual(0)
    expect(inputIndex).toBeGreaterThan(panelIndex)
  })

  it('renders the latest runner execution card inside the chat flow when the feature is enabled', async () => {
    mocks.settingsStore.experimentalAgentcoreRunnerEnabled = true
    mocks.settingsStore.agentcoreRunnerLastRun = makeAgentcoreRunnerLastRun()

    const wrapper = await mountChatViewWithMessages([
      { id: 'msg-runner', content: 'Show me the latest optimization details.' },
    ])

    const card = wrapper.get('[data-testid="chat-runner-execution-card"]')
    expect(card.text()).toContain('Agentcore Runner')
    expect(card.text()).toContain('Runner proposed a cache optimization patch.')
    expect(card.text()).toContain('manual_optimize')
  })

  it('does not render the runner execution card when the feature is disabled', async () => {
    mocks.settingsStore.experimentalAgentcoreRunnerEnabled = false
    mocks.settingsStore.agentcoreRunnerLastRun = makeAgentcoreRunnerLastRun()

    const wrapper = await mountChatViewWithMessages([
      { id: 'msg-no-runner', content: 'No runner card should be visible.' },
    ])

    expect(wrapper.find('[data-testid="chat-runner-execution-card"]').exists()).toBe(false)
  })

  it('refreshes runner status in the background when the feature is enabled', async () => {
    vi.useFakeTimers()

    try {
      mocks.settingsStore.experimentalAgentcoreRunnerEnabled = true

      await mountChatViewWithMessages([
        { id: 'msg-refresh-runner', content: 'Refresh runner state on mount.' },
      ])

      expect(mocks.settingsStore.fetchAgentcoreRunnerStatus).not.toHaveBeenCalled()

      await vi.advanceTimersByTimeAsync(1000)
      await flushPromises()

      expect(mocks.settingsStore.fetchAgentcoreRunnerStatus).toHaveBeenCalledTimes(1)
    } finally {
      vi.useRealTimers()
    }
  })

  it('hides the active todo panel after a completion-style final summary', async () => {
    const wrapper = await mountChatViewWithMessages([
      {
        id: 'msg-checklist',
        content: '- [x] 收集信息\n- [ ] 最终总结\n\n我先整理交付结果。',
        todo_card_id: 'todo-checklist-msg-checklist',
      },
      {
        id: 'msg-final',
        content: '任务已完成。\n完成内容：已输出最终结论。\n使用方法：直接查看上面的结果。',
      },
    ])

    expect(wrapper.find('[data-testid="active-todo-panel"]').exists()).toBe(false)
  })

  it('hides the active todo panel when the latest checklist has an explicit completion signal', async () => {
    mocks.chatStore.recentTodoCompletion = {
      messageId: 'msg-checklist',
      todoCardId: 'todo-checklist-msg-checklist',
    }

    const wrapper = await mountChatViewWithMessages([
      {
        id: 'msg-checklist',
        content: '- [x] 收集信息\n- [ ] 输出最终总结',
        todo_card_id: 'todo-checklist-msg-checklist',
      },
    ])

    expect(wrapper.find('[data-testid="active-todo-panel"]').exists()).toBe(false)
  })

  it('keeps the active todo panel visible while a completed checklist is still in-flight', async () => {
    mocks.chatStore.streaming = true
    mocks.chatStore.streamUIState = { phase: 'streaming', label: 'Streaming' }
    mocks.chatStore.recentTodoCompletion = {
      messageId: 'msg-checklist',
      todoCardId: 'todo-checklist-msg-checklist',
    }

    const wrapper = await mountChatViewWithMessages([
      {
        id: 'msg-checklist',
        content: '- [x] 收集信息\n- [x] 输出最终总结',
        todo_card_id: 'todo-checklist-msg-checklist',
      },
    ])

    expect(wrapper.find('[data-testid="active-todo-panel"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('2 out of 2 tasks completed')
  })

  it('keeps the active todo panel visible while a completion summary is only streaming text', async () => {
    mocks.chatStore.streaming = true
    mocks.chatStore.streamUIState = { phase: 'streaming', label: 'Streaming' }

    const wrapper = await mountChatViewWithMessages([
      {
        id: 'msg-checklist',
        content: '- [x] 收集信息\n- [ ] 输出最终总结',
        todo_card_id: 'todo-checklist-msg-checklist',
      },
      {
        id: 'streaming-summary',
        content: '任务已完成。\n完成内容：已输出最终结论。\n使用方法：直接查看上面的结果。',
      },
    ])

    expect(wrapper.find('[data-testid="active-todo-panel"]').exists()).toBe(true)
  })

  it('hides the active todo panel during streaming once the checklist message recorded a successful file write', async () => {
    mocks.chatStore.streaming = true
    mocks.chatStore.streamUIState = { phase: 'executing', label: 'Working' }
    mocks.chatStore.recentTodoCompletion = {
      messageId: 'msg-checklist',
      todoCardId: 'todo-checklist-msg-checklist',
    }

    const wrapper = await mountChatViewWithMessages([
      {
        id: 'msg-checklist',
        content: '- [x] 收集信息\n- [x] 将完整报告写入 reports/final.md',
        todo_card_id: 'todo-checklist-msg-checklist',
        extra: {
          local_process_tool_results: [{ name: 'write_commit', icon: '✓' }],
        },
      },
    ])

    expect(wrapper.find('[data-testid="active-todo-panel"]').exists()).toBe(false)
  })

  it('toggles waiting indicator, stop button, and input disabled state across conversation switches', async () => {
    mocks.chatStore.messages = [makeAssistantMessage('Streaming answer', 'msg-streaming')]
    mocks.chatStore.streaming = true
    mocks.chatStore.sending = true
    mocks.chatStore.awaitingConfirmation = true

    const wrapper = await mountChatViewWithMessages([
      { id: 'msg-streaming', content: 'Streaming answer' },
    ])

    expect(wrapper.text()).toContain('Waiting for your confirmation to continue')
    expect(wrapper.get('[data-testid="chat-input-inline-cancel"]').exists()).toBe(true)
    expect(wrapper.find('.chat-input-stub').attributes('data-disabled')).toBe('true')
    expect(wrapper.find('.chat-input-stub').attributes('data-streaming')).toBe('true')
    expect(wrapper.find('.chat-input-stub').attributes('data-can-cancel')).toBe('true')

    wrapper.unmount()

    mocks.chatStore.currentConversationId = 'conv-2'
    mocks.chatStore.currentConversation = {
      id: 'conv-2',
      title: 'Other conversation',
      created_at: '2026-03-08T00:00:01.000Z',
      updated_at: '2026-03-08T00:00:01.000Z',
    }
    mocks.chatStore.messages = []
    mocks.chatStore.streaming = false
    mocks.chatStore.sending = false
    mocks.chatStore.awaitingConfirmation = false

    const otherWrapper = await mountChatViewWithMessages([])

    expect(otherWrapper.text()).not.toContain('Waiting for your confirmation to continue')
    expect(otherWrapper.find('[data-testid="chat-input-inline-cancel"]').exists()).toBe(false)
    expect(otherWrapper.find('.chat-input-stub').attributes('data-disabled')).toBe('false')
    expect(otherWrapper.find('.chat-input-stub').attributes('data-streaming')).toBe('false')
    expect(otherWrapper.find('.chat-input-stub').attributes('data-can-cancel')).toBe('false')

    otherWrapper.unmount()

    mocks.chatStore.currentConversationId = 'conv-1'
    mocks.chatStore.currentConversation = {
      id: 'conv-1',
      title: 'Test conversation',
      created_at: '2026-03-08T00:00:00.000Z',
      updated_at: '2026-03-08T00:00:00.000Z',
    }
    mocks.chatStore.messages = [makeAssistantMessage('Streaming answer', 'msg-streaming')]
    mocks.chatStore.streaming = true
    mocks.chatStore.sending = true
    mocks.chatStore.awaitingConfirmation = true

    const restoredWrapper = await mountChatViewWithMessages([
      { id: 'msg-streaming', content: 'Streaming answer' },
    ])

    expect(restoredWrapper.text()).toContain('Waiting for your confirmation to continue')
    expect(restoredWrapper.get('[data-testid="chat-input-inline-cancel"]').exists()).toBe(true)
    expect(restoredWrapper.find('.chat-input-stub').attributes('data-disabled')).toBe('true')
    expect(restoredWrapper.find('.chat-input-stub').attributes('data-streaming')).toBe('true')
    expect(restoredWrapper.find('.chat-input-stub').attributes('data-can-cancel')).toBe('true')
  })

  it('shows the restored external stream rail when the last streaming assistant message only contains cards', async () => {
    const cardOnlyMessage = makeTypelessBlock({
      type: 'result',
      id: 'card-only-result',
      title: 'Quick result',
      content: 'Ready',
    })

    mocks.chatStore.messages = [makeAssistantMessage(cardOnlyMessage, 'msg-streaming-card-only')]
    mocks.chatStore.streaming = true
    mocks.chatStore.sending = true

    const wrapper = await mountChatViewWithMessages([
      { id: 'msg-streaming-card-only', content: cardOnlyMessage },
    ])

    expect(wrapper.find('.chat-stream-status-rail').exists()).toBe(true)
    expect(wrapper.find('[data-testid="chat-activity-dock"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="chat-input-inline-cancel"]').exists()).toBe(true)
    expect(wrapper.find('.chat-streaming-actions').exists()).toBe(false)
  })

  it('switches conversation through ConversationList click and updates streaming controls', async () => {
    mocks.chatStore.currentConversationId = 'conv-1'
    mocks.chatStore.currentConversation = {
      id: 'conv-1',
      title: 'Test conversation',
      created_at: '2026-03-08T00:00:00.000Z',
      updated_at: '2026-03-08T00:00:00.000Z',
    }
    mocks.chatStore.messages = [makeAssistantMessage('Streaming answer', 'msg-streaming')]
    mocks.chatStore.streaming = true
    mocks.chatStore.sending = true
    mocks.chatStore.awaitingConfirmation = true
    mocks.chatStore.sortedConversations = [
      {
        id: 'conv-1',
        title: 'Test conversation',
        created_at: '2026-03-08T00:00:00.000Z',
        updated_at: '2026-03-08T00:00:00.000Z',
      },
      {
        id: 'conv-2',
        title: 'Other conversation',
        created_at: '2026-03-08T00:00:01.000Z',
        updated_at: '2026-03-08T00:00:01.000Z',
      },
    ]

    mocks.chatStore.selectConversation.mockImplementation(async (id: string) => {
      mocks.chatStore.currentConversationId = id
      mocks.chatStore.currentConversation =
        id === 'conv-1'
          ? {
              id: 'conv-1',
              title: 'Test conversation',
              created_at: '2026-03-08T00:00:00.000Z',
              updated_at: '2026-03-08T00:00:00.000Z',
            }
          : {
              id: 'conv-2',
              title: 'Other conversation',
              created_at: '2026-03-08T00:00:01.000Z',
              updated_at: '2026-03-08T00:00:01.000Z',
            }
      mocks.chatStore.messages =
        id === 'conv-1' ? [makeAssistantMessage('Streaming answer', 'msg-streaming')] : []
      mocks.chatStore.streaming = id === 'conv-1'
      mocks.chatStore.sending = id === 'conv-1'
      mocks.chatStore.awaitingConfirmation = id === 'conv-1'
    })

    const wrapper = await mountChatViewWithMessages([
      { id: 'msg-streaming', content: 'Streaming answer' },
    ])

    expect(wrapper.text()).toContain('Waiting for your confirmation to continue')
    expect(wrapper.get('[data-testid="chat-input-inline-cancel"]').exists()).toBe(true)

    const convoButtons = wrapper.findAll('.conversation-select-stub')
    expect(convoButtons).toHaveLength(2)

    await convoButtons[1]!.trigger('click')
    expect(mocks.chatStore.selectConversation).toHaveBeenCalledWith('conv-2')
    wrapper.vm.$forceUpdate()
    await wrapper.vm.$nextTick()
    await flushPromises()

    expect(wrapper.text()).not.toContain('Waiting for your confirmation to continue')
    expect(wrapper.find('[data-testid="chat-input-inline-cancel"]').exists()).toBe(false)
    expect(wrapper.find('.chat-input-stub').attributes('data-disabled')).toBe('false')
    expect(wrapper.find('.chat-input-stub').attributes('data-streaming')).toBe('false')

    await convoButtons[0]!.trigger('click')
    expect(mocks.chatStore.selectConversation).toHaveBeenCalledWith('conv-1')
    wrapper.vm.$forceUpdate()
    await wrapper.vm.$nextTick()
    await flushPromises()

    expect(wrapper.text()).toContain('Waiting for your confirmation to continue')
    expect(wrapper.get('[data-testid="chat-input-inline-cancel"]').exists()).toBe(true)
    expect(wrapper.find('.chat-input-stub').attributes('data-disabled')).toBe('true')
    expect(wrapper.find('.chat-input-stub').attributes('data-streaming')).toBe('true')
  })

  it('persists the selected desktop conversation in the route query so refresh can restore task controls', async () => {
    mocks.chatStore.currentConversationId = 'conv-1'
    mocks.chatStore.currentConversation = {
      id: 'conv-1',
      title: 'Test conversation',
      created_at: '2026-03-08T00:00:00.000Z',
      updated_at: '2026-03-08T00:00:00.000Z',
    }
    mocks.chatStore.sortedConversations = [
      {
        id: 'conv-1',
        title: 'Test conversation',
        created_at: '2026-03-08T00:00:00.000Z',
        updated_at: '2026-03-08T00:00:00.000Z',
      },
      {
        id: 'conv-2',
        title: 'Other conversation',
        created_at: '2026-03-08T00:00:01.000Z',
        updated_at: '2026-03-08T00:00:01.000Z',
      },
    ]
    mocks.chatStore.selectConversation.mockImplementation(async (id: string) => {
      mocks.chatStore.currentConversationId = id
      mocks.chatStore.currentConversation = {
        id,
        title: id === 'conv-1' ? 'Test conversation' : 'Other conversation',
        created_at: '2026-03-08T00:00:00.000Z',
        updated_at: '2026-03-08T00:00:01.000Z',
      }
    })

    const { wrapper, router } = await mountChatViewWithMessagesHarness([
      { id: 'msg-routing', content: 'Keep the selected conversation on refresh.' },
    ])

    const convoButtons = wrapper.findAll('.conversation-select-stub')
    expect(convoButtons).toHaveLength(2)

    await convoButtons[1]!.trigger('click')
    await flushPromises()

    expect(mocks.chatStore.selectConversation).toHaveBeenCalledWith('conv-2')
    expect(router.currentRoute.value.fullPath).toBe('/chat?conversationId=conv-2')
  })

  it('keeps conversation running badges for local streams, agent tasks, and deep research jobs', async () => {
    mocks.chatStore.executingConversationIds = ['conv-streaming']
    mocks.chatStore.sortedConversations = [
      {
        id: 'conv-streaming',
        title: 'Streaming conversation',
        created_at: '2026-03-08T00:00:00.000Z',
        updated_at: '2026-03-08T00:00:00.000Z',
      },
      {
        id: 'conv-agent',
        title: 'Agent conversation',
        created_at: '2026-03-08T00:00:00.000Z',
        updated_at: '2026-03-08T00:00:00.000Z',
      },
      {
        id: 'conv-research',
        title: 'Research conversation',
        created_at: '2026-03-08T00:00:00.000Z',
        updated_at: '2026-03-08T00:00:00.000Z',
      },
    ]
    mocks.taskProjectionsStore.currentActiveTasks = [
      {
        id: 'task-1',
        kind: 'agent_task',
        conversation_id: 'conv-agent',
        title: 'Investigate regression',
        status: 'running',
        stage: 'planning',
        progress: 12,
        updated_at: '2026-03-08T00:00:00.000Z',
      },
    ]
    mocks.taskProjectionsStore.backgroundTasks = [
      {
        id: 'job-1',
        kind: 'research',
        conversation_id: 'conv-research',
        title: 'Track session badge regressions',
        status: 'running',
        stage: 'working',
        progress: 48,
        updated_at: '2026-03-08T00:00:00.000Z',
      },
    ]

    const wrapper = await mountChatView('Idle state')
    await flushPromises()

    const executingIds = wrapper
      .get('.conversation-list-stub')
      .attributes('data-executing-ids')
      ?.split(',')
      .filter(Boolean)

    expect(executingIds).toEqual(['conv-streaming', 'conv-agent', 'conv-research'])
  })

  it('opens a task-action dialog for workflow resume and submits structured input', async () => {
    const workflowTask = {
      id: 'task-workflow',
      kind: 'workflow',
      scope: 'current',
      conversation_id: 'conv-1',
      title: 'Workflow gate',
      status: 'waiting_user',
      stage: 'waiting_user',
      progress: 75,
      actions: {
        items: [
          {
            id: 'resume',
            label: 'Resume workflow',
            method: 'POST',
            path: '/tasks/task-workflow/actions/resume',
            requires_input: true,
          },
        ],
      },
      updated_at: '2026-03-20T12:00:00.000Z',
    }

    mocks.taskProjectionsStore.currentTasks = [workflowTask]
    mocks.taskProjectionsStore.currentActiveTasks = [workflowTask]

    const wrapper = await mountChatViewWithMessages([
      { id: 'msg-task', content: 'Workflow is waiting for input.' },
    ])

    const resumeButton = findButtonByText(wrapper, 'Resume workflow')
    expect(resumeButton?.exists()).toBe(true)

    await resumeButton!.trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="task-action-dialog"]').exists()).toBe(true)

    await wrapper.get('[data-testid="task-action-decision-input"]').setValue('approve')
    await wrapper.get('[data-testid="task-action-payload-input"]').setValue('{"ticket":"A-9"}')
    await wrapper.get('[data-testid="task-action-confirm"]').trigger('click')
    await flushPromises()

    expect(mocks.taskProjectionsStore.performTaskAction).toHaveBeenCalledWith(
      expect.objectContaining({ id: 'task-workflow' }),
      'resume',
      {
        decision: 'approve',
        payload: { ticket: 'A-9' },
      }
    )
  })

  it('opens the same task-action dialog from the background task dock', async () => {
    const workflowTask = {
      id: 'task-workflow-bg',
      kind: 'workflow',
      scope: 'background',
      conversation_id: 'conv-2',
      title: 'Background workflow gate',
      status: 'waiting_user',
      stage: 'waiting_user',
      progress: 63,
      actions: {
        items: [
          {
            id: 'resume',
            label: 'Resume workflow',
            method: 'POST',
            path: '/tasks/task-workflow-bg/actions/resume',
            requires_input: true,
          },
        ],
      },
      updated_at: '2026-03-20T12:00:00.000Z',
    }

    mocks.taskProjectionsStore.backgroundTasks = [workflowTask]

    const wrapper = await mountChatViewWithMessages([
      { id: 'msg-bg-task', content: 'Background workflow is waiting for input.' },
    ])

    expect(wrapper.get('[data-testid="task-projection-dock"]').exists()).toBe(true)
    const resumeButton = wrapper.get('.task-projection-dock-action-stub')

    await resumeButton.trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="task-action-dialog"]').exists()).toBe(true)

    await wrapper.get('[data-testid="task-action-decision-input"]').setValue('approve')
    await wrapper.get('[data-testid="task-action-payload-input"]').setValue('{"ticket":"B-2"}')
    await wrapper.get('[data-testid="task-action-confirm"]').trigger('click')
    await flushPromises()

    expect(mocks.taskProjectionsStore.performTaskAction).toHaveBeenCalledWith(
      expect.objectContaining({ id: 'task-workflow-bg' }),
      'resume',
      {
        decision: 'approve',
        payload: { ticket: 'B-2' },
      }
    )
  })

  it('renders the deep research dock above the composer and forwards view plus cancel actions', async () => {
    mocks.deepResearchJobsStore.activeJobs = [
      {
        id: 'job-1',
        job_id: 'job-1',
        conversation_id: 'conv-research',
        query: 'Track session badge regressions',
        status: 'running',
        stage: 'planning',
        progress: 48,
        updated_at: '2026-03-20T12:00:00.000Z',
      },
    ]

    const wrapper = await mountChatViewWithMessages([
      { id: 'msg-research', content: 'Deep research is running.' },
    ])

    expect(wrapper.get('[data-testid="deep-research-task-dock"]').exists()).toBe(true)

    const dockChildren = Array.from(wrapper.get('.chat-input-dock').element.children)
    const researchDockIndex = dockChildren.findIndex(
      (child) =>
        child instanceof HTMLElement &&
        !!child.querySelector('[data-testid="deep-research-task-dock"]')
    )
    const inputIndex = dockChildren.findIndex(
      (child) => child instanceof HTMLElement && child.classList.contains('chat-input-stub')
    )

    expect(researchDockIndex).toBeGreaterThanOrEqual(0)
    expect(inputIndex).toBeGreaterThan(researchDockIndex)

    await wrapper.get('[data-testid="deep-research-task-dock-toggle"]').trigger('click')

    await wrapper.get('[data-testid="deep-research-task-dock-view-job-1"]').trigger('click')
    expect(mocks.deepResearchJobsStore.openJob).toHaveBeenCalledWith('job-1', 'conv-research')

    await wrapper.get('[data-testid="deep-research-task-dock-cancel-job-1"]').trigger('click')
    expect(mocks.deepResearchJobsStore.cancelJob).toHaveBeenCalledWith('job-1')
  })

  it('shows a notification when a direct task action fails', async () => {
    const consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
    const task = {
      id: 'task-cancel-fail',
      kind: 'agent_task',
      scope: 'current',
      conversation_id: 'conv-1',
      title: 'Cancelable task',
      status: 'running',
      stage: 'working',
      progress: 40,
      actions: {
        items: [
          {
            id: 'cancel',
            label: 'Cancel',
            method: 'POST',
            path: '/tasks/task-cancel-fail/actions/cancel',
            requires_input: false,
          },
        ],
      },
      updated_at: '2026-03-20T12:00:00.000Z',
    }

    mocks.taskProjectionsStore.currentTasks = [task]
    mocks.taskProjectionsStore.currentActiveTasks = [task]
    mocks.taskProjectionsStore.performTaskAction.mockRejectedValueOnce(
      new Error('permission denied')
    )

    const wrapper = await mountChatViewWithMessages([
      { id: 'msg-cancel-fail', content: 'Task action should fail.' },
    ])

    const cancelButton = findButtonByText(wrapper, 'Cancel')
    expect(cancelButton?.exists()).toBe(true)

    await cancelButton!.trigger('click')
    await flushPromises()

    expect(mocks.notificationStore.error).toHaveBeenCalledWith(
      'Task action failed',
      'permission denied'
    )
    expect(consoleErrorSpy).toHaveBeenCalled()
    consoleErrorSpy.mockRestore()
  })

  it('navigates to harness details from projected task shortcuts', async () => {
    const task = {
      id: 'task-harness-nav',
      kind: 'agent_task',
      scope: 'current',
      conversation_id: 'conv-1',
      title: 'Harness regression',
      status: 'failed',
      stage: 'partial',
      progress: 100,
      actions: { items: [] },
      detail_href: '/operations/harness/group-1',
      updated_at: '2026-03-20T12:00:00.000Z',
    }

    mocks.taskProjectionsStore.currentTasks = [task]

    const wrapper = await mountChatViewWithMessages([
      { id: 'msg-harness-nav', content: 'Harness task completed.' },
    ])

    await wrapper.get('.task-projection-card-navigate-stub').trigger('click')
    await flushPromises()

    expect(wrapper.vm.$route.path).toBe('/operations/harness/group-1')
  })

  it('submits use_browser from a web-fetch card rendered inside ChatView', async () => {
    mocks.cardActionSubmit.mockResolvedValue({
      data: {
        success: true,
        message: `Open ${WEB_FETCH_URL} with the browser tool.`,
      },
    })

    const wrapper = await mountChatView(
      makeTypelessBlock({
        type: 'web-fetch',
        id: WEB_FETCH_CARD_ID,
        title: 'Sign in',
        status: 'warning',
        url: WEB_FETCH_URL,
        content: 'Log in to continue',
        content_type: 'text/html',
        extract_mode: 'text',
        extractor: 'html',
        warning: 'page appears to be a login wall; use browser or pass browser_target_id',
        warning_code: 'login_wall',
        actions: [
          {
            id: 'use_browser',
            label: 'Use browser',
            variant: 'primary',
            form_data: { url: WEB_FETCH_URL },
          },
        ],
      })
    )

    await expandCardById(wrapper, WEB_FETCH_CARD_ID)
    const button = findButtonByText(wrapper, 'Use browser')
    expect(button?.exists()).toBe(true)

    await button!.trigger('click')
    await flushPromises()

    expect(mocks.chatStore.fetchConversations).toHaveBeenCalledTimes(1)
    expect(mocks.cardActionSubmit).toHaveBeenCalledWith('conv-1', 'msg-1', {
      card_id: WEB_FETCH_CARD_ID,
      action_id: 'use_browser',
      action_label: 'Use browser',
      card_type: 'web-fetch',
      card_title: 'Sign in',
      form_data: { url: WEB_FETCH_URL },
    })
    expect(mocks.chatStore.sendMessage).toHaveBeenCalledWith(
      `Open ${WEB_FETCH_URL} with the browser tool.`
    )
  })

  it('submits browser extract action from ChatView with browser_target_id intact', async () => {
    mocks.cardActionSubmit.mockResolvedValue({
      data: {
        success: true,
        message: `Use web_fetch on ${WEB_FETCH_URL} with browser_target_id=tab-42 to extract readable content.`,
      },
    })

    const wrapper = await mountChatView(
      makeTypelessBlock({
        type: 'result',
        id: BROWSER_CARD_ID,
        title: 'Browser page',
        status: 'info',
        message: 'Interactive page opened in the browser session.',
        details: [
          { label: 'url', value: WEB_FETCH_URL },
          { label: 'browser_target_id', value: 'tab-42' },
        ],
        actions: [
          {
            id: 'extract_with_web_fetch',
            label: 'Extract readable content',
            variant: 'primary',
            form_data: {
              url: WEB_FETCH_URL,
              browser_target_id: 'tab-42',
            },
          },
        ],
      })
    )

    const button = findButtonByText(wrapper, 'Extract readable content')
    expect(button?.exists()).toBe(true)

    await button!.trigger('click')
    await flushPromises()

    expect(mocks.cardActionSubmit).toHaveBeenCalledWith('conv-1', 'msg-1', {
      card_id: BROWSER_CARD_ID,
      action_id: 'extract_with_web_fetch',
      action_label: 'Extract readable content',
      card_type: 'result',
      card_title: 'Browser page',
      form_data: {
        url: WEB_FETCH_URL,
        browser_target_id: 'tab-42',
      },
    })
    expect(mocks.chatStore.sendMessage).toHaveBeenCalledWith(
      `Use web_fetch on ${WEB_FETCH_URL} with browser_target_id=tab-42 to extract readable content.`
    )
  })

  it('preserves the Reddit login-wall action chain across assistant messages in ChatView', async () => {
    mocks.cardActionSubmit
      .mockResolvedValueOnce({
        data: {
          success: true,
          message: `Open ${WEB_FETCH_URL} with the browser tool.`,
        },
      })
      .mockResolvedValueOnce({
        data: {
          success: true,
          message: `Use web_fetch on ${WEB_FETCH_URL} with browser_target_id=tab-42 to extract readable content.`,
        },
      })

    const wrapper = await mountChatViewWithMessages([
      {
        id: 'msg-1',
        content: makeTypelessBlock({
          type: 'web-fetch',
          id: WEB_FETCH_CARD_ID,
          title: 'Sign in',
          status: 'warning',
          url: WEB_FETCH_URL,
          content: 'Log in to continue',
          content_type: 'text/html',
          extract_mode: 'text',
          extractor: 'html',
          warning: 'page appears to be a login wall; use browser or pass browser_target_id',
          warning_code: 'login_wall',
          actions: [
            {
              id: 'use_browser',
              label: 'Use browser',
              variant: 'primary',
              form_data: { url: WEB_FETCH_URL },
            },
          ],
        }),
      },
      {
        id: 'msg-2',
        content: makeTypelessBlock({
          type: 'result',
          id: BROWSER_CARD_ID,
          title: 'Browser page',
          status: 'info',
          message: 'Interactive page opened in the browser session.',
          details: [
            { label: 'url', value: WEB_FETCH_URL },
            { label: 'browser_target_id', value: 'tab-42' },
          ],
          actions: [
            {
              id: 'extract_with_web_fetch',
              label: 'Extract readable content',
              variant: 'primary',
              form_data: {
                url: WEB_FETCH_URL,
                browser_target_id: 'tab-42',
              },
            },
          ],
        }),
      },
    ])

    await expandCardById(wrapper, WEB_FETCH_CARD_ID)
    const useBrowserButton = findButtonByText(wrapper, 'Use browser')
    const extractButton = findButtonByText(wrapper, 'Extract readable content')

    expect(useBrowserButton?.exists()).toBe(true)
    expect(extractButton?.exists()).toBe(true)

    await useBrowserButton!.trigger('click')
    await flushPromises()

    await extractButton!.trigger('click')
    await flushPromises()

    expect(mocks.cardActionSubmit).toHaveBeenCalledTimes(2)
    expect(mocks.cardActionSubmit).toHaveBeenNthCalledWith(1, 'conv-1', 'msg-1', {
      card_id: WEB_FETCH_CARD_ID,
      action_id: 'use_browser',
      action_label: 'Use browser',
      card_type: 'web-fetch',
      card_title: 'Sign in',
      form_data: { url: WEB_FETCH_URL },
    })
    expect(mocks.cardActionSubmit).toHaveBeenNthCalledWith(2, 'conv-1', 'msg-2', {
      card_id: BROWSER_CARD_ID,
      action_id: 'extract_with_web_fetch',
      action_label: 'Extract readable content',
      card_type: 'result',
      card_title: 'Browser page',
      form_data: {
        url: WEB_FETCH_URL,
        browser_target_id: 'tab-42',
      },
    })
    expect(mocks.chatStore.sendMessage.mock.calls.map((call) => call[0])).toEqual([
      `Open ${WEB_FETCH_URL} with the browser tool.`,
      `Use web_fetch on ${WEB_FETCH_URL} with browser_target_id=tab-42 to extract readable content.`,
    ])
  })
  it('renders a streamed browser follow-up after use_browser and preserves extract form_data', async () => {
    mocks.cardActionSubmit
      .mockResolvedValueOnce({
        data: {
          success: true,
          message: `Open ${WEB_FETCH_URL} with the browser tool.`,
        },
      })
      .mockResolvedValueOnce({
        data: {
          success: true,
          message: `Use web_fetch on ${WEB_FETCH_URL} with browser_target_id=tab-42 to extract readable content.`,
        },
      })

    const wrapper = await mountChatView(
      makeTypelessBlock({
        type: 'web-fetch',
        id: WEB_FETCH_CARD_ID,
        title: 'Sign in',
        status: 'warning',
        url: WEB_FETCH_URL,
        content: 'Log in to continue',
        content_type: 'text/html',
        extract_mode: 'text',
        extractor: 'html',
        warning: 'page appears to be a login wall; use browser or pass browser_target_id',
        warning_code: 'login_wall',
        actions: [
          {
            id: 'use_browser',
            label: 'Use browser',
            variant: 'primary',
            form_data: { url: WEB_FETCH_URL },
          },
        ],
      })
    )

    await expandCardById(wrapper, WEB_FETCH_CARD_ID)
    const useBrowserButton = findButtonByText(wrapper, 'Use browser')
    expect(useBrowserButton?.exists()).toBe(true)

    await useBrowserButton!.trigger('click')
    await flushPromises()

    expect(mocks.cardActionSubmit).toHaveBeenCalledTimes(1)
    expect(mocks.cardActionSubmit).toHaveBeenNthCalledWith(1, 'conv-1', 'msg-1', {
      card_id: WEB_FETCH_CARD_ID,
      action_id: 'use_browser',
      action_label: 'Use browser',
      card_type: 'web-fetch',
      card_title: 'Sign in',
      form_data: { url: WEB_FETCH_URL },
    })
    expect(mocks.chatStore.sendMessage).toHaveBeenCalledWith(
      `Open ${WEB_FETCH_URL} with the browser tool.`
    )

    mocks.chatStore.messages = [
      ...mocks.chatStore.messages,
      makeAssistantMessage(
        makeTypelessBlock({
          type: 'result',
          id: BROWSER_CARD_ID,
          title: 'Browser page',
          status: 'info',
          message: 'Interactive page opened in the browser session.',
          details: [
            { label: 'url', value: WEB_FETCH_URL },
            { label: 'browser_target_id', value: 'tab-42' },
          ],
          actions: [
            {
              id: 'extract_with_web_fetch',
              label: 'Extract readable content',
              variant: 'primary',
              form_data: {
                url: WEB_FETCH_URL,
                browser_target_id: 'tab-42',
              },
            },
          ],
        }),
        'msg-2'
      ),
    ]

    wrapper.vm.$forceUpdate()
    await wrapper.vm.$nextTick()
    await flushPromises()
    await vi.dynamicImportSettled()
    await flushPromises()

    const extractButton = findButtonByText(wrapper, 'Extract readable content')
    expect(extractButton?.exists()).toBe(true)
    expect(wrapper.text()).toContain('Browser page')

    await extractButton!.trigger('click')
    await flushPromises()

    expect(mocks.cardActionSubmit).toHaveBeenCalledTimes(2)
    expect(mocks.cardActionSubmit).toHaveBeenNthCalledWith(2, 'conv-1', 'msg-2', {
      card_id: BROWSER_CARD_ID,
      action_id: 'extract_with_web_fetch',
      action_label: 'Extract readable content',
      card_type: 'result',
      card_title: 'Browser page',
      form_data: {
        url: WEB_FETCH_URL,
        browser_target_id: 'tab-42',
      },
    })
    expect(mocks.chatStore.sendMessage.mock.calls.map((call) => call[0])).toEqual([
      `Open ${WEB_FETCH_URL} with the browser tool.`,
      `Use web_fetch on ${WEB_FETCH_URL} with browser_target_id=tab-42 to extract readable content.`,
    ])
  })
})
