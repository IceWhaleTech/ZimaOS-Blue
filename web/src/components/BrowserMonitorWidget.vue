<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  getBrowserOverview,
  getSessionMonitor,
  type BrowserSession,
  type BrowserSessionMonitorResponse,
  type BrowserSessionScreenshot,
} from '@/api/browser'
import type { UserTaskProjection } from '@/api/tasks'
import { useBrowserMonitor } from '@/composables/useBrowserMonitor'
import { offSSEEvent, onSSEEvent } from '@/composables/useEventStream'
import { useChatStore } from '@/stores/chat'
import {
  localizeTaskProjectionSubtitle,
  localizeTaskProjectionTitle,
} from '@/utils/taskProjectionText'
import { canOpenTaskConversation } from '@/utils/taskProjectionActions'

const MONITOR_POSITION_KEY = 'zima.browser.monitor.position.v1'
const MONITOR_LAUNCHER_POSITION_KEY = 'zima.browser.monitor.launcher.position.v1'
const MONITOR_SIZE_KEY = 'zima.browser.monitor.size.v1'
const MONITOR_SESSION_KEY = 'zima.browser.monitor.session.v1'
const MONITOR_TASK_VIEW_KEY = 'zima.browser.monitor.task.view.v1'
const LEGACY_DEFAULT_WIDTH = 860
const LEGACY_DEFAULT_HEIGHT = 520
const DEFAULT_WIDTH = 700
const DEFAULT_HEIGHT = 430
const DEFAULT_COLLAPSED_WIDTH = 300
const DEFAULT_COLLAPSED_HEIGHT = 252
const DEFAULT_LAUNCHER_WIDTH = 180
const DEFAULT_LAUNCHER_HEIGHT = 76
const MIN_WIDTH = 600
const MIN_HEIGHT = 350
const VIEWPORT_PADDING = 12
const DEFAULT_PANEL_TOP_OFFSET = 88
const OVERVIEW_POLL_MS = 4000
const SCREENSHOT_POLL_MS = 2200
const TASK_OVERVIEW_EVENT_REFRESH_DELAY_MS = 150
const SESSION_MONITOR_EVENT_REFRESH_DELAY_MS = 75
const SESSION_MONITOR_EVENT_TYPE = 'browser_session_monitor_updated'
const SESSION_ACTIVITY_EVENT_TYPE = 'browser_session_activity'
const TASK_OVERVIEW_EVENT_TYPES = [
  'task_created',
  'task_planning',
  'task_progress',
  'task_step_completed',
  'task_reflection_started',
  'task_reflection_completed',
  'task_completed',
  'task_failed',
  'task_cancelled',
  'task_user_message',
  'task_question',
  'task_question_answered',
  'deep_research.job_created',
  'deep_research.job_updated',
  'deep_research.job_completed',
  'deep_research.job_failed',
  'deep_research.job_cancelled',
] as const
type TaskViewMode = 'smart' | 'current' | 'all'
type ScreenshotState = {
  screenshot: string
  history: BrowserSessionScreenshot[]
  lastScreenshotAt: string
}
type TextMonitorState = {
  title: string
  url: string
  summary: string
  treePreview: string
  interactiveCount: number
  updatedAt: string
  status: string
}

const route = useRoute()
const router = useRouter()
const chatStore = useChatStore()
const { t, te, locale } = useI18n()
const browserMonitor = useBrowserMonitor()

const panelRef = ref<HTMLElement | null>(null)
const launcherRef = ref<HTMLElement | null>(null)
const isOpen = computed({
  get: () => browserMonitor.isOpen.value,
  set: (value: boolean) => browserMonitor.setOpen(value),
})
const isCollapsed = computed({
  get: () => browserMonitor.isCollapsed.value,
  set: (value: boolean) => browserMonitor.setCollapsed(value),
})
const position = reactive(loadStoredPosition())
const launcherPosition = reactive(loadStoredLauncherPosition())
const size = reactive(loadStoredSize())
const selectedSessionId = ref(loadStoredString(MONITOR_SESSION_KEY))
const taskViewMode = ref<TaskViewMode>(loadStoredTaskViewMode())
const dragTarget = ref<'panel' | 'launcher' | null>(null)
const dragMoved = ref(false)
const activePointerId = ref<number | null>(null)
const suppressLauncherClick = ref(false)
const sessions = ref<BrowserSession[]>([])
const tasks = ref<UserTaskProjection[]>([])
const screenshot = ref('')
const screenshotHistory = ref<BrowserSessionScreenshot[]>([])
const screenshotCache = ref<Record<string, ScreenshotState>>({})
const textMonitor = ref<TextMonitorState | null>(null)
const textMonitorCache = ref<Record<string, TextMonitorState>>({})
const screenshotError = ref('')
const overviewError = ref('')
const lastScreenshotAt = ref('')
const activeScreenshotKey = ref('')
const overviewLoading = ref(false)
const screenshotLoading = ref(false)
const dragOffset = reactive({ x: 0, y: 0 })
const dragStart = reactive({ x: 0, y: 0 })

let overviewTimer: ReturnType<typeof setInterval> | null = null
let screenshotTimer: ReturnType<typeof setInterval> | null = null
let panelResizeObserver: ResizeObserver | null = null
let launcherClickResetTimer: ReturnType<typeof setTimeout> | null = null
let taskOverviewEventRefreshTimer: ReturnType<typeof setTimeout> | null = null
let sessionMonitorEventRefreshTimer: ReturnType<typeof setTimeout> | null = null
const compactPreviewWarmupInFlight = new Set<string>()

function tr(key: string, fallback: string): string {
  return te(key) ? t(key) : fallback
}

function interpolateTemplate(
  template: string,
  params: Record<string, string | number> = {}
): string {
  return Object.entries(params).reduce((result, [name, value]) => {
    return result.split(`{${name}}`).join(String(value))
  }, template)
}

function trp(key: string, fallback: string, params: Record<string, string | number> = {}): string {
  return te(key) ? String(t(key, params)) : interpolateTemplate(fallback, params)
}

function humanizeLabel(value: string | undefined): string {
  return String(value || '')
    .trim()
    .replace(/_/g, ' ')
}

function sessionLayerValue(session: BrowserSession | null | undefined): string {
  const explicit = String(session?.session_layer || '')
    .trim()
    .toLowerCase()
  if (explicit) return explicit
  if (session?.monitor_kind === 'text' || session?.engine === 'lightpanda') return 'read'
  if (session?.engine_detail === 'lightpanda_binary') return 'browser_lite'
  return 'full_browser'
}

function sessionLayerLabel(session: BrowserSession | null | undefined): string {
  switch (sessionLayerValue(session)) {
    case 'read':
      return tr('browserMonitor.sessionKindRead', 'Read session')
    case 'browser_lite':
      return tr('browserMonitor.sessionKindBrowserLite', 'Browser-lite session')
    default:
      return tr('browserMonitor.sessionKindFullBrowser', 'Full browser session')
  }
}

function sessionEngineLabel(session: BrowserSession | null | undefined): string {
  const detail = humanizeLabel(session?.engine_detail)
  if (detail) return detail
  return humanizeLabel(session?.engine)
}

function sessionDescriptorLabel(session: BrowserSession | null | undefined): string {
  const layer = sessionLayerLabel(session)
  const engine = sessionEngineLabel(session)
  if (!engine) return layer
  if (engine.toLowerCase() === layer.toLowerCase()) return layer
  return `${layer} · ${engine}`
}

function loadStoredString(key: string): string {
  try {
    return localStorage.getItem(key) || ''
  } catch {
    return ''
  }
}

function loadStoredPosition() {
  const defaultPosition = getDefaultPanelPosition()
  try {
    const raw = localStorage.getItem(MONITOR_POSITION_KEY)
    if (!raw) return defaultPosition
    const parsed = JSON.parse(raw) as { x?: number; y?: number }
    const x = Number(parsed?.x)
    const y = Number(parsed?.y)
    return {
      x: Number.isFinite(x) ? x : defaultPosition.x,
      y: Number.isFinite(y) ? y : defaultPosition.y,
    }
  } catch {
    return defaultPosition
  }
}

function loadStoredLauncherPosition() {
  const defaultPosition = getDefaultLauncherPosition()
  try {
    const raw = localStorage.getItem(MONITOR_LAUNCHER_POSITION_KEY)
    if (!raw) return defaultPosition
    const parsed = JSON.parse(raw) as { x?: number; y?: number }
    const x = Number(parsed?.x)
    const y = Number(parsed?.y)
    return {
      x: Number.isFinite(x) ? x : defaultPosition.x,
      y: Number.isFinite(y) ? y : defaultPosition.y,
    }
  } catch {
    return defaultPosition
  }
}

function viewportLimit(axis: 'width' | 'height', fallback: number): number {
  if (typeof window === 'undefined') return fallback
  const raw = axis === 'width' ? window.innerWidth - 24 : window.innerHeight - 24
  return Math.max(axis === 'width' ? 320 : 280, raw)
}

function clampSize(width: number, height: number) {
  const maxWidth = viewportLimit('width', DEFAULT_WIDTH)
  const maxHeight = viewportLimit('height', DEFAULT_HEIGHT)
  const minWidth = Math.min(MIN_WIDTH, maxWidth)
  const minHeight = Math.min(MIN_HEIGHT, maxHeight)
  return {
    width: Math.max(minWidth, Math.min(Math.round(width || DEFAULT_WIDTH), maxWidth)),
    height: Math.max(minHeight, Math.min(Math.round(height || DEFAULT_HEIGHT), maxHeight)),
  }
}

function loadStoredSize() {
  const defaultSize = clampSize(DEFAULT_WIDTH, DEFAULT_HEIGHT)
  try {
    const raw = localStorage.getItem(MONITOR_SIZE_KEY)
    if (!raw) return defaultSize
    const parsed = JSON.parse(raw) as { width?: number; height?: number }
    const storedSize = clampSize(Number(parsed?.width), Number(parsed?.height))
    if (storedSize.width === LEGACY_DEFAULT_WIDTH && storedSize.height === LEGACY_DEFAULT_HEIGHT) {
      return defaultSize
    }
    return storedSize
  } catch {
    return defaultSize
  }
}

function loadStoredTaskViewMode(): TaskViewMode {
  try {
    const raw = String(localStorage.getItem(MONITOR_TASK_VIEW_KEY) || '').trim()
    if (raw === 'current' || raw === 'all') return raw
  } catch {
    // Ignore storage failures in restricted environments.
  }
  return 'smart'
}

function getDefaultPanelPosition() {
  if (typeof window === 'undefined') {
    return { x: 16, y: 16 }
  }
  const defaultSize = clampSize(DEFAULT_WIDTH, DEFAULT_HEIGHT)
  const maxY = Math.max(
    VIEWPORT_PADDING,
    window.innerHeight - defaultSize.height - VIEWPORT_PADDING
  )
  return {
    x: Math.max(VIEWPORT_PADDING, window.innerWidth - defaultSize.width - VIEWPORT_PADDING),
    y: Math.min(DEFAULT_PANEL_TOP_OFFSET, maxY),
  }
}

function getDefaultLauncherPosition() {
  if (typeof window === 'undefined') {
    return { x: 16, y: 16 }
  }
  return {
    x: Math.max(VIEWPORT_PADDING, window.innerWidth - DEFAULT_LAUNCHER_WIDTH - 16),
    y: Math.max(VIEWPORT_PADDING, window.innerHeight - DEFAULT_LAUNCHER_HEIGHT - 16),
  }
}

function persistString(key: string, value: string) {
  try {
    if (!value) {
      localStorage.removeItem(key)
      return
    }
    localStorage.setItem(key, value)
  } catch {
    // Ignore storage failures in restricted environments.
  }
}

function persistTaskViewMode(value: TaskViewMode) {
  try {
    localStorage.setItem(MONITOR_TASK_VIEW_KEY, value)
  } catch {
    // Ignore storage failures in restricted environments.
  }
}

function persistPosition() {
  try {
    localStorage.setItem(
      MONITOR_POSITION_KEY,
      JSON.stringify({
        x: Math.round(position.x),
        y: Math.round(position.y),
      })
    )
  } catch {
    // Ignore storage failures in restricted environments.
  }
}

function persistLauncherPosition() {
  try {
    localStorage.setItem(
      MONITOR_LAUNCHER_POSITION_KEY,
      JSON.stringify({
        x: Math.round(launcherPosition.x),
        y: Math.round(launcherPosition.y),
      })
    )
  } catch {
    // Ignore storage failures in restricted environments.
  }
}

function persistSize() {
  try {
    localStorage.setItem(
      MONITOR_SIZE_KEY,
      JSON.stringify({
        width: Math.round(size.width),
        height: Math.round(size.height),
      })
    )
  } catch {
    // Ignore storage failures in restricted environments.
  }
}

function isTaskActive(task: Pick<UserTaskProjection, 'status'> | null | undefined): boolean {
  if (!task) return false
  return task.status === 'running' || task.status === 'waiting_user'
}

function compareTasks(left: UserTaskProjection, right: UserTaskProjection): number {
  const leftActive = isTaskActive(left)
  const rightActive = isTaskActive(right)
  if (leftActive !== rightActive) return leftActive ? -1 : 1
  return Date.parse(right.updated_at || '') - Date.parse(left.updated_at || '')
}

function formatRelativeTime(value: string): string {
  const timestamp = Date.parse(value)
  if (Number.isNaN(timestamp)) return tr('browserMonitor.justNow', 'just now')

  const diffMs = Date.now() - timestamp
  const diffMinutes = Math.round(diffMs / 60000)
  if (diffMinutes <= 0) return tr('browserMonitor.justNow', 'just now')
  if (diffMinutes < 60) return `${diffMinutes}m`

  const diffHours = Math.round(diffMinutes / 60)
  if (diffHours < 24) return `${diffHours}h`

  const diffDays = Math.round(diffHours / 24)
  return `${diffDays}d`
}

function formatAbsoluteTime(value: string): string {
  const timestamp = Date.parse(value)
  if (Number.isNaN(timestamp)) return ''

  try {
    return new Intl.DateTimeFormat(locale.value || undefined, {
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hour12: false,
    }).format(timestamp)
  } catch {
    return new Date(timestamp).toLocaleString()
  }
}

function formatScreenshotScope(value: string | undefined): string {
  const normalized = String(value || '')
    .trim()
    .toLowerCase()

  switch (normalized) {
    case 'viewport':
      return tr('browserMonitor.previewScopeViewport', 'Viewport')
    case 'full_page':
    case 'fullpage':
      return tr('browserMonitor.previewScopeFullPage', 'Full page')
    default:
      return normalized
        ? normalized.replace(/_/g, ' ')
        : tr('browserMonitor.previewScopeLatestFrame', 'Latest frame')
  }
}

function screenshotDataUrl(value: string | undefined | null): string {
  const data = String(value || '').trim()
  return data ? `data:image/png;base64,${data}` : ''
}

function sessionPreviewFallback(session: BrowserSession | null | undefined): string {
  const source = String(session?.page_title || session?.current_url || '').trim()
  const normalized = source.replace(/^https?:\/\//, '').trim()
  const initial = normalized.charAt(0).toUpperCase()
  return initial || '•'
}

function screenshotFrameKey(frame: Pick<BrowserSessionScreenshot, 'captured_at' | 'data'>): string {
  return `${String(frame.captured_at || '').trim()}:${String(frame.data || '')
    .trim()
    .slice(0, 32)}`
}

function normalizeScreenshotHistory(
  value: BrowserSessionScreenshot[] | null | undefined
): BrowserSessionScreenshot[] {
  if (!Array.isArray(value) || value.length === 0) return []
  const normalized: BrowserSessionScreenshot[] = []
  const seenKeys = new Set<string>()
  for (const item of value) {
    const data = String(item?.data || '').trim()
    if (!data) continue
    const frame: BrowserSessionScreenshot = {
      data,
      url: String(item?.url || '').trim() || undefined,
      title: String(item?.title || '').trim() || undefined,
      captured_at: String(item?.captured_at || '').trim(),
      scope: String(item?.scope || '').trim() || undefined,
    }
    const key = screenshotFrameKey(frame)
    if (seenKeys.has(key)) continue
    seenKeys.add(key)
    normalized.push(frame)
  }
  return normalized
}

function cloneScreenshotHistory(
  value: BrowserSessionScreenshot[] | null | undefined
): BrowserSessionScreenshot[] {
  if (!Array.isArray(value) || value.length === 0) return []
  return value.map((frame) => ({ ...frame }))
}

function clearScreenshotState() {
  screenshot.value = ''
  screenshotHistory.value = []
  lastScreenshotAt.value = ''
  activeScreenshotKey.value = ''
}

function applyScreenshotState(state: ScreenshotState | null | undefined) {
  if (!state) {
    clearScreenshotState()
    return
  }

  screenshot.value = String(state.screenshot || '').trim()
  screenshotHistory.value = cloneScreenshotHistory(state.history)
  lastScreenshotAt.value = String(state.lastScreenshotAt || '').trim()
  if (!screenshot.value && screenshotHistory.value.length === 0) {
    activeScreenshotKey.value = ''
  }
}

function resolveScreenshotState(
  session: BrowserSession | null | undefined,
  nextScreenshot: string,
  nextHistory: BrowserSessionScreenshot[]
): ScreenshotState | null {
  const screenshotValue = String(nextScreenshot || '').trim() || nextHistory[0]?.data || ''
  if (!screenshotValue) return null

  const capturedAt = String(nextHistory[0]?.captured_at || '').trim() || new Date().toISOString()
  const history =
    nextHistory.length > 0
      ? cloneScreenshotHistory(nextHistory)
      : [
          {
            data: screenshotValue,
            captured_at: capturedAt,
            title: String(session?.page_title || '').trim() || undefined,
            url: String(session?.current_url || '').trim() || undefined,
          },
        ]

  return {
    screenshot: screenshotValue,
    history,
    lastScreenshotAt: capturedAt,
  }
}

function rememberScreenshotState(sessionID: string, state: ScreenshotState | null | undefined) {
  const normalizedID = String(sessionID || '').trim()
  if (!normalizedID || !state) return
  if (!state.screenshot && state.history.length === 0) return

  screenshotCache.value = {
    ...screenshotCache.value,
    [normalizedID]: {
      screenshot: String(state.screenshot || '').trim(),
      history: cloneScreenshotHistory(state.history),
      lastScreenshotAt: String(state.lastScreenshotAt || '').trim(),
    },
  }
}

function getCachedScreenshotState(sessionID: string): ScreenshotState | null {
  const normalizedID = String(sessionID || '').trim()
  if (!normalizedID) return null
  const cached = screenshotCache.value[normalizedID]
  if (!cached) return null
  if (!cached.screenshot && cached.history.length === 0) return null

  return {
    screenshot: String(cached.screenshot || '').trim(),
    history: cloneScreenshotHistory(cached.history),
    lastScreenshotAt: String(cached.lastScreenshotAt || '').trim(),
  }
}

function clearTextMonitorState() {
  textMonitor.value = null
}

function applyTextMonitorState(state: TextMonitorState | null | undefined) {
  if (!state) {
    clearTextMonitorState()
    return
  }
  textMonitor.value = {
    title: String(state.title || '').trim(),
    url: String(state.url || '').trim(),
    summary: String(state.summary || '').trim(),
    treePreview: String(state.treePreview || '').trim(),
    interactiveCount: Number.isFinite(Number(state.interactiveCount))
      ? Number(state.interactiveCount)
      : 0,
    updatedAt: String(state.updatedAt || '').trim(),
    status: String(state.status || '').trim(),
  }
  lastScreenshotAt.value = textMonitor.value.updatedAt
}

function normalizeTextMonitorState(
  payload: BrowserSessionMonitorResponse['text'] | null | undefined
): TextMonitorState | null {
  if (!payload) return null
  const summary = String(payload.summary || '').trim()
  const treePreview = String(payload.tree_preview || '').trim()
  if (!summary && !treePreview) return null
  return {
    title: String(payload.title || '').trim(),
    url: String(payload.url || '').trim(),
    summary,
    treePreview,
    interactiveCount: Number.isFinite(Number(payload.interactive_count))
      ? Number(payload.interactive_count)
      : 0,
    updatedAt: String(payload.updated_at || '').trim(),
    status: String(payload.status || '').trim(),
  }
}

function rememberTextMonitorState(sessionID: string, state: TextMonitorState | null | undefined) {
  const normalizedID = String(sessionID || '').trim()
  if (!normalizedID || !state) return
  textMonitorCache.value = {
    ...textMonitorCache.value,
    [normalizedID]: { ...state },
  }
}

function getCachedTextMonitorState(sessionID: string): TextMonitorState | null {
  const normalizedID = String(sessionID || '').trim()
  if (!normalizedID) return null
  const cached = textMonitorCache.value[normalizedID]
  return cached ? { ...cached } : null
}

function stageLabel(task: Pick<UserTaskProjection, 'stage'> | null | undefined): string {
  switch (task?.stage) {
    case 'planning':
      return tr('chat.taskStagePlanning', 'Planning')
    case 'working':
      return tr('chat.taskStageWorking', 'Working')
    case 'verifying':
      return tr('chat.taskStageVerifying', 'Verifying')
    case 'waiting_user':
      return tr('chat.taskStageWaiting', 'Waiting')
    case 'completed':
      return tr('chat.taskStageCompleted', 'Completed')
    case 'failed':
      return tr('chat.taskStageFailed', 'Failed')
    case 'cancelled':
      return tr('chat.taskStageCancelled', 'Cancelled')
    default:
      return tr('browserMonitor.stageRunning', 'Running')
  }
}

function stageTone(task: Pick<UserTaskProjection, 'stage'> | null | undefined): string {
  switch (task?.stage) {
    case 'completed':
      return 'is-success'
    case 'failed':
      return 'is-danger'
    case 'cancelled':
      return 'is-muted'
    case 'waiting_user':
      return 'is-warn'
    default:
      return 'is-info'
  }
}

function progressWidth(value: number | undefined): string {
  const normalized = Math.max(0, Math.min(100, Math.round(Number(value || 0))))
  return `${normalized}%`
}

function blockerLabel(task: Pick<UserTaskProjection, 'blocker'> | null | undefined): string {
  if (!task?.blocker) return ''
  switch (task.blocker.kind) {
    case 'approval':
      return tr('chat.taskWaitingForApproval', 'Waiting for your approval')
    case 'question':
      return tr('chat.taskWaitingForAnswer', 'Waiting for your answer')
    default:
      return task.blocker.label || tr('browserMonitor.blockerFallback', 'Waiting for input')
  }
}

function taskTitle(task: Pick<UserTaskProjection, 'title' | 'kind'> | null | undefined): string {
  if (!task) return ''
  return localizeTaskProjectionTitle(task.title, task.kind, t)
}

function taskSubtitle(
  task: Pick<UserTaskProjection, 'subtitle' | 'kind'> | null | undefined
): string {
  if (!task) return ''
  return localizeTaskProjectionSubtitle(task.subtitle, task.kind, t)
}

function getPanelDimensions() {
  if (isCollapsed.value) {
    return {
      width: panelRef.value?.offsetWidth || DEFAULT_COLLAPSED_WIDTH,
      height: panelRef.value?.offsetHeight || DEFAULT_COLLAPSED_HEIGHT,
    }
  }
  return {
    width: panelRef.value?.offsetWidth || size.width,
    height: panelRef.value?.offsetHeight || size.height,
  }
}

function getLauncherDimensions() {
  return {
    width: launcherRef.value?.offsetWidth || DEFAULT_LAUNCHER_WIDTH,
    height: launcherRef.value?.offsetHeight || DEFAULT_LAUNCHER_HEIGHT,
  }
}

function clampCoordinates(x: number, y: number, target: 'panel' | 'launcher') {
  if (typeof window === 'undefined') {
    return { x, y }
  }
  const dimensions = target === 'panel' ? getPanelDimensions() : getLauncherDimensions()
  return {
    x: Math.max(
      VIEWPORT_PADDING,
      Math.min(Math.round(x), window.innerWidth - dimensions.width - VIEWPORT_PADDING)
    ),
    y: Math.max(
      VIEWPORT_PADDING,
      Math.min(Math.round(y), window.innerHeight - dimensions.height - VIEWPORT_PADDING)
    ),
  }
}

function clampPanelPosition() {
  if (typeof window === 'undefined') return
  if (!isCollapsed.value) {
    const nextSize = clampSize(size.width, size.height)
    size.width = nextSize.width
    size.height = nextSize.height
  }
  const nextPosition = clampCoordinates(position.x, position.y, 'panel')
  position.x = nextPosition.x
  position.y = nextPosition.y
}

function clampLauncherPosition() {
  const nextPosition = clampCoordinates(launcherPosition.x, launcherPosition.y, 'launcher')
  launcherPosition.x = nextPosition.x
  launcherPosition.y = nextPosition.y
}

function stopDragging() {
  const draggedLauncher = dragTarget.value === 'launcher' && dragMoved.value
  activePointerId.value = null
  dragTarget.value = null
  dragMoved.value = false
  document.removeEventListener('pointermove', onDrag)
  document.removeEventListener('pointerup', stopDragging)
  document.removeEventListener('pointercancel', stopDragging)
  if (draggedLauncher) {
    suppressLauncherClick.value = true
    if (launcherClickResetTimer) {
      clearTimeout(launcherClickResetTimer)
    }
    launcherClickResetTimer = setTimeout(() => {
      suppressLauncherClick.value = false
      launcherClickResetTimer = null
    }, 0)
  }
}

function onDrag(event: PointerEvent) {
  if (!dragTarget.value || activePointerId.value !== event.pointerId) return
  if (
    !dragMoved.value &&
    (Math.abs(event.clientX - dragStart.x) > 3 || Math.abs(event.clientY - dragStart.y) > 3)
  ) {
    dragMoved.value = true
  }
  const nextX = event.clientX - dragOffset.x
  const nextY = event.clientY - dragOffset.y
  const nextPosition = clampCoordinates(nextX, nextY, dragTarget.value)
  if (dragTarget.value === 'panel') {
    position.x = nextPosition.x
    position.y = nextPosition.y
    return
  }
  launcherPosition.x = nextPosition.x
  launcherPosition.y = nextPosition.y
}

function beginDragging(
  event: PointerEvent,
  target: 'panel' | 'launcher',
  origin: { x: number; y: number }
) {
  if (launcherClickResetTimer) {
    clearTimeout(launcherClickResetTimer)
    launcherClickResetTimer = null
  }
  suppressLauncherClick.value = false
  activePointerId.value = event.pointerId
  dragTarget.value = target
  dragMoved.value = false
  dragStart.x = event.clientX
  dragStart.y = event.clientY
  dragOffset.x = event.clientX - origin.x
  dragOffset.y = event.clientY - origin.y
  document.addEventListener('pointermove', onDrag)
  document.addEventListener('pointerup', stopDragging)
  document.addEventListener('pointercancel', stopDragging)
}

function isInvalidDragStart(event: PointerEvent) {
  return !event.isPrimary || (event.pointerType === 'mouse' && event.button !== 0)
}

function startPanelDragging(event: PointerEvent) {
  if (isInvalidDragStart(event)) return
  if (!(event.target as HTMLElement).closest('.browser-monitor__handle')) return
  if ((event.target as HTMLElement).closest('button, a, input, textarea, select')) return
  beginDragging(event, 'panel', position)
}

function startLauncherDragging(event: PointerEvent) {
  if (isInvalidDragStart(event)) return
  beginDragging(event, 'launcher', launcherPosition)
}

function handleLauncherClick() {
  if (suppressLauncherClick.value) {
    suppressLauncherClick.value = false
    return
  }
  isOpen.value = true
}

function observePanelSize() {
  if (!panelResizeObserver || !panelRef.value) return
  panelResizeObserver.disconnect()
  panelResizeObserver.observe(panelRef.value)
}

async function refreshTasks() {
  const overview = await getBrowserOverview({
    scope: effectiveTaskScope.value,
    conversationId:
      effectiveTaskScope.value === 'current'
        ? activeTaskConversationId.value || undefined
        : undefined,
    limit: 12,
  })
  tasks.value = overview.tasks.slice().sort(compareTasks)
  sessions.value = overview.sessions.slice().sort((left, right) => {
    if (left.status === right.status) return 0
    if (left.status === 'active') return -1
    if (right.status === 'active') return 1
    return 0
  })
}

async function refreshOverview() {
  overviewLoading.value = true
  overviewError.value = ''
  try {
    await refreshTasks()
  } catch (error) {
    overviewError.value =
      error instanceof Error ? error.message : tr('browserMonitor.overviewError', 'Refresh failed')
  } finally {
    overviewLoading.value = false
  }
}

function scheduleOverviewRefreshFromEvent() {
  if (taskOverviewEventRefreshTimer) return
  taskOverviewEventRefreshTimer = setTimeout(() => {
    taskOverviewEventRefreshTimer = null
    void refreshOverview()
  }, TASK_OVERVIEW_EVENT_REFRESH_DELAY_MS)
}

function scheduleScreenshotRefreshFromEvent(payload: unknown) {
  if (!isOpen.value) return
  if (isTextMonitor.value) return
  if (screenshotLoading.value) return
  const data = payload && typeof payload === 'object' ? (payload as Record<string, unknown>) : {}
  const sessionID = String(data.session_id || '').trim()
  if (!sessionID || sessionID !== selectedSession.value?.id) return
  const capturedAt = String(data.captured_at || '').trim()
  if (capturedAt && lastScreenshotAt.value && capturedAt <= lastScreenshotAt.value) return
  if (sessionMonitorEventRefreshTimer) return
  sessionMonitorEventRefreshTimer = setTimeout(() => {
    sessionMonitorEventRefreshTimer = null
    void refreshScreenshot()
  }, SESSION_MONITOR_EVENT_REFRESH_DELAY_MS)
}

function scheduleScreenshotRefreshFromActivity(payload: unknown) {
  if (!isOpen.value) return
  if (isTextMonitor.value) return
  if (screenshotLoading.value) return
  const data = payload && typeof payload === 'object' ? (payload as Record<string, unknown>) : {}
  const sessionID = String(data.session_id || '').trim()
  if (!sessionID || sessionID !== selectedSession.value?.id) return
  const observedAt = String(data.observed_at || '').trim()
  if (observedAt && lastScreenshotAt.value && observedAt <= lastScreenshotAt.value) return
  if (sessionMonitorEventRefreshTimer) return
  sessionMonitorEventRefreshTimer = setTimeout(() => {
    sessionMonitorEventRefreshTimer = null
    void refreshScreenshot()
  }, SESSION_MONITOR_EVENT_REFRESH_DELAY_MS)
}

async function refreshScreenshot() {
  const session = selectedSession.value
  if (!isOpen.value || !session?.id) return
  const sessionID = session.id
  screenshotLoading.value = true
  screenshotError.value = ''
  try {
    const response = await getSessionMonitor(sessionID)
    if (response.kind === 'text' || session.monitor_kind === 'text') {
      const nextTextState = normalizeTextMonitorState(response.text)
      if (nextTextState) {
        rememberTextMonitorState(sessionID, nextTextState)
      }
      if (selectedSession.value?.id === sessionID) {
        applyTextMonitorState(nextTextState || getCachedTextMonitorState(sessionID))
        clearScreenshotState()
      }
      if (!nextTextState) {
        screenshotError.value =
          response.error || tr('browserMonitor.screenshotError', 'Preview unavailable')
      } else if (response.error) {
        screenshotError.value = response.error
      }
      return
    }

    const nextHistory = normalizeScreenshotHistory(response.image?.history || [])
    const nextScreenshot = String(response.image?.screenshot || '').trim()
    const resolvedState = resolveScreenshotState(session, nextScreenshot, nextHistory)
    if (resolvedState) {
      rememberScreenshotState(sessionID, resolvedState)
    }

    if (selectedSession.value?.id === sessionID) {
      clearTextMonitorState()
      if (resolvedState) {
        applyScreenshotState(resolvedState)
      } else {
        const cachedState = getCachedScreenshotState(sessionID)
        if (cachedState) {
          applyScreenshotState(cachedState)
        }
      }
    }

    if (!resolvedState) {
      screenshotError.value =
        response.error || tr('browserMonitor.screenshotError', 'Preview unavailable')
    } else if (response.error && !nextScreenshot) {
      screenshotError.value = response.error
    }
  } catch (error) {
    if (session.monitor_kind === 'text') {
      const cachedState = getCachedTextMonitorState(sessionID)
      if (cachedState && selectedSession.value?.id === sessionID) {
        applyTextMonitorState(cachedState)
      }
    } else {
      const cachedState = getCachedScreenshotState(sessionID)
      if (cachedState && selectedSession.value?.id === sessionID) {
        applyScreenshotState(cachedState)
      }
    }
    screenshotError.value =
      error instanceof Error
        ? error.message
        : tr('browserMonitor.screenshotError', 'Preview unavailable')
  } finally {
    screenshotLoading.value = false
  }
}

async function warmCollapsedSessionPreviews() {
  if (!isOpen.value || !isCollapsed.value || sessions.value.length < 2) return

  const candidates = sessions.value
    .filter((session) => {
      if (!session.id || session.monitor_kind !== 'image') return false
      if (session.id === selectedSession.value?.id) return false
      if (getCachedScreenshotState(session.id)) return false
      return !compactPreviewWarmupInFlight.has(session.id)
    })
    .slice(0, 6)

  if (candidates.length === 0) return

  await Promise.allSettled(
    candidates.map(async (session) => {
      compactPreviewWarmupInFlight.add(session.id)
      try {
        const response = await getSessionMonitor(session.id)
        if (response.kind !== 'image') return
        const nextHistory = normalizeScreenshotHistory(response.image?.history || [])
        const nextScreenshot = String(response.image?.screenshot || '').trim()
        const resolvedState = resolveScreenshotState(session, nextScreenshot, nextHistory)
        if (resolvedState) {
          rememberScreenshotState(session.id, resolvedState)
        }
      } catch {
        // Ignore warmup failures and keep placeholder thumbnails for uncached sessions.
      } finally {
        compactPreviewWarmupInFlight.delete(session.id)
      }
    })
  )
}

async function refreshAll() {
  await refreshOverview()
  await refreshScreenshot()
}

async function openTask(task: UserTaskProjection) {
  if (!canOpenTaskConversation(task) || !task.conversation_id) return
  await router.push({ name: 'Chat', query: { conversationId: task.conversation_id } })
}

function closeMonitor() {
  isOpen.value = false
}

const activeTasks = computed(() => tasks.value.filter((task) => isTaskActive(task)))
const recentTasks = computed(() => tasks.value)
const hasAutoOpenSuppressedCurrentRun = ref(false)
const hasMonitorActivity = computed(() => activeTasks.value.length > 0)
const routeConversationId = computed(() => {
  const raw = route.query.conversationId
  if (Array.isArray(raw)) return String(raw[0] || '').trim()
  return String(raw || '').trim()
})
const activeTaskConversationId = computed(() => {
  const routeID = routeConversationId.value
  if (routeID) return routeID
  return String(chatStore.currentConversationId || '').trim()
})
const effectiveTaskScope = computed<'current' | 'all'>(() => {
  if (taskViewMode.value === 'all') return 'all'
  if (taskViewMode.value === 'current') return 'current'
  return activeTaskConversationId.value ? 'current' : 'all'
})
const taskViewLabel = computed(() => {
  if (effectiveTaskScope.value === 'current') {
    return tr('browserMonitor.taskViewCurrent', 'Current chat')
  }
  return tr('browserMonitor.taskViewAll', 'All tasks')
})
const selectedSession = computed<BrowserSession | null>(() => {
  const preferredId = selectedSessionId.value.trim()
  if (preferredId) {
    const match = sessions.value.find((session) => session.id === preferredId)
    if (match) return match
  }
  return sessions.value.find((session) => session.status === 'active') || sessions.value[0] || null
})
const leadTask = computed(() => activeTasks.value[0] || recentTasks.value[0] || null)
const panelStyle = computed(() => {
  const style: Record<string, string> = {
    top: `${position.y}px`,
    left: `${position.x}px`,
  }
  if (isCollapsed.value) return style
  style.width = `${size.width}px`
  style.height = `${size.height}px`
  return style
})
const launcherStyle = computed(() => ({
  top: `${launcherPosition.y}px`,
  left: `${launcherPosition.x}px`,
}))
const launcherLabel = computed(() => {
  if (activeTasks.value.length > 0) {
    return tr('browserMonitor.launcherActive', 'Execution monitor')
  }
  return tr('browserMonitor.launcherIdle', 'Live monitor')
})
const launcherMeta = computed(() => {
  const parts: string[] = []
  if (activeTasks.value.length > 0) {
    parts.push(`${activeTasks.value.length} ${tr('browserMonitor.tasksShort', 'tasks')}`)
  }
  if (sessions.value.length > 0) {
    parts.push(`${sessions.value.length} ${tr('browserMonitor.tabsShort', 'tabs')}`)
  }
  return parts.join(' · ') || tr('browserMonitor.launcherMeta', 'Click to open')
})
const isTextMonitor = computed(() => selectedSession.value?.monitor_kind === 'text')
const activeTextMonitor = computed<TextMonitorState | null>(() => {
  if (!isTextMonitor.value) return null
  return textMonitor.value
})
const previewFrames = computed<BrowserSessionScreenshot[]>(() => {
  if (isTextMonitor.value) return []
  if (screenshotHistory.value.length > 0) return screenshotHistory.value
  if (!screenshot.value) return []
  return [
    {
      data: screenshot.value,
      captured_at: lastScreenshotAt.value,
      title: selectedSession.value?.page_title,
      url: selectedSession.value?.current_url,
    },
  ]
})
const activeScreenshotFrame = computed<BrowserSessionScreenshot | null>(() => {
  const activeKey = activeScreenshotKey.value.trim()
  if (activeKey) {
    const match = previewFrames.value.find((frame) => screenshotFrameKey(frame) === activeKey)
    if (match) return match
  }
  return previewFrames.value[0] || null
})
const previewTitle = computed(() => {
  return (
    activeTextMonitor.value?.title ||
    activeScreenshotFrame.value?.title ||
    selectedSession.value?.page_title ||
    tr('browserMonitor.previewIdleTitle', 'Waiting for browser activity')
  )
})
const previewUrl = computed(() => {
  return (
    activeTextMonitor.value?.url ||
    activeScreenshotFrame.value?.url ||
    selectedSession.value?.current_url ||
    tr('browserMonitor.previewIdleUrl', 'Open or reuse a browser tab to start the live feed.')
  )
})
const screenshotSrc = computed(() => screenshotDataUrl(activeScreenshotFrame.value?.data))
const compactSessionThumbnails = computed(() =>
  sessions.value.map((session) => {
    const cachedState =
      session.id === selectedSession.value?.id ? null : getCachedScreenshotState(session.id)
    const cachedData = String(cachedState?.history[0]?.data || cachedState?.screenshot || '').trim()
    return {
      session,
      src:
        session.id === selectedSession.value?.id
          ? screenshotSrc.value
          : screenshotDataUrl(cachedData),
      fallback: sessionPreviewFallback(session),
      isActive: session.id === selectedSession.value?.id,
      label: session.page_title || tr('browserMonitor.untitledTab', 'Untitled tab'),
    }
  })
)
const activeFrameCapturedAt = computed(() =>
  String(activeScreenshotFrame.value?.captured_at || lastScreenshotAt.value || '').trim()
)
const activeFrameIndex = computed(() => {
  const activeFrame = activeScreenshotFrame.value
  if (!activeFrame) return -1
  return previewFrames.value.findIndex(
    (frame) => screenshotFrameKey(frame) === screenshotFrameKey(activeFrame)
  )
})
const previewMeta = computed(() => {
  if (isTextMonitor.value) {
    const items: string[] = []
    const absoluteTime = formatAbsoluteTime(
      activeTextMonitor.value?.updatedAt || lastScreenshotAt.value
    )
    if (absoluteTime) {
      items.push(absoluteTime)
    }
    if ((activeTextMonitor.value?.interactiveCount || 0) > 0) {
      items.push(
        trp('browserMonitor.textInteractiveCount', '{count} interactive elements', {
          count: activeTextMonitor.value?.interactiveCount || 0,
        })
      )
    }
    if (selectedSession.value) {
      items.push(sessionDescriptorLabel(selectedSession.value))
    }
    return items.filter((item) => item.trim().length > 0)
  }
  if (!activeScreenshotFrame.value && !activeFrameCapturedAt.value) return []

  const items: string[] = [formatScreenshotScope(activeScreenshotFrame.value?.scope)]
  const absoluteTime = formatAbsoluteTime(activeFrameCapturedAt.value)
  if (absoluteTime) {
    items.push(absoluteTime)
  }
  if (previewFrames.value.length > 1 && activeFrameIndex.value >= 0) {
    items.push(
      trp('browserMonitor.previewFrameLabel', 'Frame {current}/{total}', {
        current: activeFrameIndex.value + 1,
        total: previewFrames.value.length,
      })
    )
  }
  return items.filter((item) => item.trim().length > 0)
})
const taskViewOptions = computed<
  Array<{ id: TaskViewMode; label: string; active: boolean; disabled: boolean }>
>(() => [
  {
    id: 'smart',
    label: tr('browserMonitor.taskViewSmart', 'Smart'),
    active: taskViewMode.value === 'smart',
    disabled: false,
  },
  {
    id: 'current',
    label: tr('browserMonitor.taskViewCurrentShort', 'Current'),
    active: taskViewMode.value === 'current',
    disabled: !activeTaskConversationId.value,
  },
  {
    id: 'all',
    label: tr('browserMonitor.taskViewAllShort', 'All'),
    active: taskViewMode.value === 'all',
    disabled: false,
  },
])
const capabilityCards = computed(() => {
  const cards = [
    {
      id: 'task-focus',
      priority: leadTask.value ? 'P1' : 'P2',
      label: tr('browserMonitor.capabilityTask', 'Task focus'),
      detail: leadTask.value
        ? `${stageLabel(leadTask.value)} · ${Math.round(leadTask.value.progress || 0)}%`
        : tr('browserMonitor.capabilityTaskIdle', 'Waiting for the next run'),
      tone: leadTask.value ? 'is-critical' : 'is-muted',
    },
    {
      id: 'browser-session',
      priority: selectedSession.value ? 'P1' : 'P3',
      label: tr('browserMonitor.capabilityBrowser', 'Browser continuity'),
      detail: selectedSession.value
        ? sessionLayerLabel(selectedSession.value)
        : tr('browserMonitor.capabilityBrowserIdle', 'No active browser tab'),
      tone: selectedSession.value ? 'is-strong' : 'is-muted',
    },
  ]

  if (leadTask.value?.blocker) {
    cards.push({
      id: 'run-blocker',
      priority: 'P0',
      label: tr('browserMonitor.capabilityBlocker', 'Run blocker'),
      detail: blockerLabel(leadTask.value),
      tone: 'is-alert',
    })
  }

  cards.push({
    id: 'preview-freshness',
    priority: selectedSession.value ? 'P2' : 'P4',
    label: tr('browserMonitor.capabilityPreview', 'Preview freshness'),
    detail: lastScreenshotAt.value
      ? `${tr('browserMonitor.updated', 'Updated')} ${formatRelativeTime(lastScreenshotAt.value)}`
      : tr('browserMonitor.capabilityPreviewIdle', 'Waiting for a fresh frame'),
    tone: lastScreenshotAt.value ? 'is-calm' : 'is-muted',
  })

  return cards
})
const emptyTaskMessage = computed(() => {
  if (effectiveTaskScope.value === 'current' && !activeTaskConversationId.value) {
    return tr(
      'browserMonitor.noCurrentConversation',
      'Open a chat conversation to follow only the current run.'
    )
  }
  if (effectiveTaskScope.value === 'current') {
    return tr('browserMonitor.noCurrentTasks', 'No recent tasks in this conversation yet.')
  }
  return tr('browserMonitor.noTasks', 'No recent tasks yet.')
})
const taskPanelHint = computed(() => {
  const countLabel = `${recentTasks.value.length} ${tr('browserMonitor.tasksCount', 'items')}`
  return `${taskViewLabel.value} · ${countLabel}`
})

watch(
  isOpen,
  async (nextOpen) => {
    if (nextOpen) {
      await nextTick()
      observePanelSize()
      clampPanelPosition()
      void refreshAll()
      return
    }
    clampLauncherPosition()
    panelResizeObserver?.disconnect()
    if (hasMonitorActivity.value) {
      hasAutoOpenSuppressedCurrentRun.value = true
    }
    screenshotError.value = ''
  },
  { immediate: true }
)

watch(
  isCollapsed,
  (nextCollapsed) => {
    if (!nextCollapsed) {
      void nextTick().then(() => {
        observePanelSize()
        clampPanelPosition()
      })
    }
  },
  { immediate: true }
)

watch(taskViewMode, (nextMode) => {
  persistTaskViewMode(nextMode)
  void refreshOverview()
})

watch(
  () => selectedSession.value?.id || '',
  (nextID, previousID) => {
    if (!nextID) {
      screenshotError.value = ''
      clearTextMonitorState()
      return
    }

    selectedSessionId.value = nextID
    persistString(MONITOR_SESSION_KEY, nextID)
    if (selectedSession.value?.monitor_kind === 'text') {
      const cachedState = getCachedTextMonitorState(nextID)
      if (cachedState) {
        applyTextMonitorState(cachedState)
      } else if (nextID !== previousID) {
        clearTextMonitorState()
      }
      clearScreenshotState()
    } else {
      const cachedState = getCachedScreenshotState(nextID)
      if (cachedState) {
        applyScreenshotState(cachedState)
      } else if (nextID !== previousID) {
        clearScreenshotState()
      }
      clearTextMonitorState()
    }
    screenshotError.value = ''
    void refreshScreenshot()
  },
  { immediate: true }
)

watch(
  () =>
    `${isOpen.value}:${isCollapsed.value}:${selectedSession.value?.id || ''}:${sessions.value
      .map((session) => `${session.id}:${session.monitor_kind}`)
      .join('|')}`,
  () => {
    void warmCollapsedSessionPreviews()
  },
  { immediate: true }
)

watch(
  previewFrames,
  (frames, previousFrames) => {
    if (frames.length === 0) {
      activeScreenshotKey.value = ''
      return
    }
    const latestFrame = frames[0]
    if (!latestFrame) {
      activeScreenshotKey.value = ''
      return
    }
    const latestKey = screenshotFrameKey(latestFrame)

    if (isCollapsed.value) {
      activeScreenshotKey.value = latestKey
      return
    }

    const activeKey = activeScreenshotKey.value.trim()
    if (!activeKey) {
      activeScreenshotKey.value = latestKey
      return
    }

    const previousLatestFrame = previousFrames?.[0]
    if (previousLatestFrame) {
      const previousLatestKey = screenshotFrameKey(previousLatestFrame)
      if (previousLatestKey && activeKey === previousLatestKey) {
        activeScreenshotKey.value = latestKey
        return
      }
    }

    if (frames.some((frame) => screenshotFrameKey(frame) === activeKey)) {
      return
    }
    activeScreenshotKey.value = latestKey
  },
  { immediate: true }
)

watch(
  () => [effectiveTaskScope.value, activeTaskConversationId.value, route.fullPath],
  () => {
    void refreshOverview()
  }
)

watch(
  () => [activeTasks.value.length, sessions.value.length] as const,
  ([nextActiveTaskCount, nextSessionCount]) => {
    browserMonitor.setActivitySnapshot({
      activeTaskCount: nextActiveTaskCount,
      sessionCount: nextSessionCount,
    })
  },
  { immediate: true }
)

watch(
  hasMonitorActivity,
  (nextHasActivity, previousHasActivity) => {
    if (!nextHasActivity) {
      hasAutoOpenSuppressedCurrentRun.value = false
      return
    }
    if (previousHasActivity || isOpen.value || hasAutoOpenSuppressedCurrentRun.value) {
      return
    }
    browserMonitor.openMonitor({ expand: true })
  },
  { immediate: true }
)

watch(
  () => [position.x, position.y],
  () => {
    persistPosition()
  }
)

watch(
  () => [launcherPosition.x, launcherPosition.y],
  () => {
    persistLauncherPosition()
  }
)

watch(
  () => [size.width, size.height],
  () => {
    persistSize()
  }
)

onMounted(() => {
  for (const eventType of TASK_OVERVIEW_EVENT_TYPES) {
    onSSEEvent(eventType, scheduleOverviewRefreshFromEvent)
  }
  onSSEEvent(SESSION_MONITOR_EVENT_TYPE, scheduleScreenshotRefreshFromEvent)
  onSSEEvent(SESSION_ACTIVITY_EVENT_TYPE, scheduleScreenshotRefreshFromActivity)
  void refreshOverview()
  overviewTimer = setInterval(() => {
    void refreshOverview()
  }, OVERVIEW_POLL_MS)
  screenshotTimer = setInterval(() => {
    void refreshScreenshot()
  }, SCREENSHOT_POLL_MS)
  window.addEventListener('resize', clampLauncherPosition)
  window.addEventListener('resize', clampPanelPosition)
  if (typeof ResizeObserver !== 'undefined') {
    panelResizeObserver = new ResizeObserver(() => {
      if (!panelRef.value || isCollapsed.value) return
      const nextSize = clampSize(panelRef.value.offsetWidth, panelRef.value.offsetHeight)
      if (nextSize.width !== size.width) size.width = nextSize.width
      if (nextSize.height !== size.height) size.height = nextSize.height
      clampPanelPosition()
    })
    observePanelSize()
  }
  clampLauncherPosition()
  clampPanelPosition()
})

onUnmounted(() => {
  stopDragging()
  for (const eventType of TASK_OVERVIEW_EVENT_TYPES) {
    offSSEEvent(eventType, scheduleOverviewRefreshFromEvent)
  }
  offSSEEvent(SESSION_MONITOR_EVENT_TYPE, scheduleScreenshotRefreshFromEvent)
  offSSEEvent(SESSION_ACTIVITY_EVENT_TYPE, scheduleScreenshotRefreshFromActivity)
  if (overviewTimer) clearInterval(overviewTimer)
  if (screenshotTimer) clearInterval(screenshotTimer)
  if (launcherClickResetTimer) clearTimeout(launcherClickResetTimer)
  if (taskOverviewEventRefreshTimer) clearTimeout(taskOverviewEventRefreshTimer)
  if (sessionMonitorEventRefreshTimer) clearTimeout(sessionMonitorEventRefreshTimer)
  panelResizeObserver?.disconnect()
  window.removeEventListener('resize', clampLauncherPosition)
  window.removeEventListener('resize', clampPanelPosition)
})
</script>

<template>
  <Teleport to="body">
    <button
      v-if="!isOpen"
      ref="launcherRef"
      type="button"
      class="browser-monitor-launcher"
      :class="{ 'has-activity': activeTasks.length > 0 || sessions.length > 0 }"
      :style="launcherStyle"
      @pointerdown="startLauncherDragging"
      @click="handleLauncherClick"
    >
      <span class="browser-monitor-launcher__dot" aria-hidden="true" />
      <span class="browser-monitor-launcher__copy">
        <strong>{{ launcherLabel }}</strong>
        <span>{{ launcherMeta }}</span>
      </span>
    </button>

    <section
      v-else
      ref="panelRef"
      class="browser-monitor"
      :class="{ 'is-collapsed': isCollapsed, 'is-resizable': !isCollapsed }"
      :style="panelStyle"
      @pointerdown="startPanelDragging"
    >
      <header class="browser-monitor__header browser-monitor__handle">
        <div class="browser-monitor__title-group">
          <span v-if="!isCollapsed" class="browser-monitor__eyebrow">{{
            tr('browserMonitor.eyebrow', 'Browser execution')
          }}</span>
          <h3 class="browser-monitor__title" :class="{ 'is-compact': isCollapsed }">
            {{ tr('browserMonitor.title', 'Live monitor') }}
          </h3>
          <p v-if="!isCollapsed" class="browser-monitor__subtitle">
            {{ tr('browserMonitor.subtitle', 'Track the latest task and tab state here.') }}
          </p>
        </div>

        <div class="browser-monitor__actions">
          <button
            type="button"
            class="browser-monitor__icon-button"
            :title="tr('browserMonitor.refresh', 'Refresh')"
            @click="refreshAll"
          >
            ↻
          </button>
          <button
            type="button"
            class="browser-monitor__icon-button"
            :title="
              isCollapsed
                ? tr('browserMonitor.expand', 'Expand')
                : tr('browserMonitor.collapse', 'Collapse')
            "
            @click="isCollapsed = !isCollapsed"
          >
            {{ isCollapsed ? '▢' : '—' }}
          </button>
          <button
            type="button"
            class="browser-monitor__icon-button is-close"
            :title="tr('common.close', 'Close')"
            @click="closeMonitor"
          >
            ×
          </button>
        </div>
      </header>

      <div v-if="isCollapsed" class="browser-monitor__compact">
        <div v-if="compactSessionThumbnails.length > 1" class="browser-monitor__compact-strip">
          <button
            v-for="item in compactSessionThumbnails"
            :key="item.session.id"
            type="button"
            class="browser-monitor__compact-thumb"
            :class="{ 'is-active': item.isActive }"
            :title="item.session.page_title || item.session.current_url || item.label"
            @click="selectedSessionId = item.session.id"
          >
            <img
              v-if="item.src"
              :src="item.src"
              :alt="item.label"
              class="browser-monitor__compact-thumb-image"
            />
            <span v-else class="browser-monitor__compact-thumb-fallback">{{ item.fallback }}</span>
          </button>
        </div>

        <div class="browser-monitor__compact-frame">
          <div v-if="isTextMonitor && activeTextMonitor" class="browser-monitor__compact-text">
            <strong>{{ previewTitle }}</strong>
            <p>
              {{
                activeTextMonitor.summary ||
                tr('browserMonitor.textSummaryIdle', 'Waiting for readable summary...')
              }}
            </p>
          </div>
          <img
            v-else-if="screenshotSrc"
            :src="screenshotSrc"
            :alt="tr('browserMonitor.previewAlt', 'Browser preview')"
            class="browser-monitor__compact-image"
          />
          <div v-else class="browser-monitor__compact-frame-empty">
            <strong>{{ tr('browserMonitor.previewEmptyTitle', 'No preview frame yet') }}</strong>
            <span>{{
              screenshotLoading
                ? tr('browserMonitor.previewLoading', 'Capturing the current viewport...')
                : tr(
                    'browserMonitor.previewEmptyBody',
                    'The monitor will show screenshots as soon as a browser tab is active.'
                  )
            }}</span>
          </div>
        </div>
      </div>

      <div v-else class="browser-monitor__body">
        <aside class="browser-monitor__sidebar">
          <div class="browser-monitor__panel">
            <div class="browser-monitor__panel-heading">
              <span>{{ tr('browserMonitor.capabilityHeading', 'Capability priorities') }}</span>
              <span v-if="overviewLoading" class="browser-monitor__hint">{{
                tr('browserMonitor.refreshing', 'Refreshing')
              }}</span>
            </div>

            <div class="browser-monitor__capabilities">
              <article
                v-for="card in capabilityCards"
                :key="card.id"
                class="browser-monitor__capability"
                :class="card.tone"
              >
                <div class="browser-monitor__capability-topline">
                  <span class="browser-monitor__priority-pill">{{ card.priority }}</span>
                  <span class="browser-monitor__capability-label">{{ card.label }}</span>
                </div>
                <p class="browser-monitor__capability-detail">{{ card.detail }}</p>
              </article>
            </div>
          </div>

          <div class="browser-monitor__panel browser-monitor__panel--tasks">
            <div class="browser-monitor__panel-heading">
              <span>{{ tr('browserMonitor.tasksHeading', 'Current task pulse') }}</span>
              <span class="browser-monitor__hint">{{ taskPanelHint }}</span>
            </div>

            <div class="browser-monitor__filters">
              <button
                v-for="option in taskViewOptions"
                :key="option.id"
                type="button"
                class="browser-monitor__filter-chip"
                :class="{ 'is-active': option.active }"
                :disabled="option.disabled"
                @click="taskViewMode = option.id"
              >
                {{ option.label }}
              </button>
            </div>

            <div class="browser-monitor__task-list">
              <div v-if="recentTasks.length === 0" class="browser-monitor__empty">
                {{ emptyTaskMessage }}
              </div>

              <button
                v-for="task in recentTasks"
                :key="task.id"
                type="button"
                class="browser-monitor__task"
                :class="{ 'is-clickable': canOpenTaskConversation(task) && !!task.conversation_id }"
                @click="openTask(task)"
              >
                <div class="browser-monitor__task-row">
                  <span class="browser-monitor__stage-pill" :class="stageTone(task)">
                    {{ stageLabel(task) }}
                  </span>
                  <span class="browser-monitor__task-meta">
                    {{ Math.round(task.progress || 0) }}%
                  </span>
                </div>
                <strong class="browser-monitor__task-title">{{ taskTitle(task) }}</strong>
                <p v-if="taskSubtitle(task)" class="browser-monitor__task-subtitle">
                  {{ taskSubtitle(task) }}
                </p>
                <p v-if="task.blocker" class="browser-monitor__task-blocker">
                  {{ blockerLabel(task) }}
                </p>
                <div class="browser-monitor__progress-track">
                  <div
                    class="browser-monitor__progress-bar"
                    :class="stageTone(task)"
                    :style="{ width: progressWidth(task.progress) }"
                  />
                </div>
              </button>
            </div>
          </div>

          <p v-if="overviewError" class="browser-monitor__error">{{ overviewError }}</p>
        </aside>

        <section class="browser-monitor__preview">
          <div class="browser-monitor__preview-header">
            <div>
              <span class="browser-monitor__preview-eyebrow">{{
                tr('browserMonitor.previewHeading', 'Browser preview')
              }}</span>
              <h4 class="browser-monitor__preview-title">{{ previewTitle }}</h4>
              <p class="browser-monitor__preview-url">{{ previewUrl }}</p>
            </div>
            <span class="browser-monitor__preview-badge">
              {{
                activeScreenshotFrame?.captured_at || lastScreenshotAt
                  ? `${tr('browserMonitor.updated', 'Updated')} ${formatRelativeTime(activeScreenshotFrame?.captured_at || lastScreenshotAt)}`
                  : tr('browserMonitor.previewPending', 'Pending')
              }}
            </span>
          </div>

          <div v-if="previewMeta.length > 0" class="browser-monitor__preview-meta">
            <span v-for="item in previewMeta" :key="item" class="browser-monitor__preview-chip">
              {{ item }}
            </span>
          </div>

          <div class="browser-monitor__frame">
            <div v-if="isTextMonitor && activeTextMonitor" class="browser-monitor__text-monitor">
              <div class="browser-monitor__text-card">
                <strong>{{ tr('browserMonitor.textSummaryHeading', 'Summary') }}</strong>
                <p>
                  {{
                    activeTextMonitor.summary ||
                    tr('browserMonitor.textSummaryIdle', 'Waiting for readable summary...')
                  }}
                </p>
              </div>
              <div class="browser-monitor__text-card">
                <strong>{{ tr('browserMonitor.textTreeHeading', 'Tree Preview') }}</strong>
                <pre>{{
                  activeTextMonitor.treePreview ||
                  tr('browserMonitor.textTreeIdle', 'Waiting for structured tree preview...')
                }}</pre>
              </div>
              <div class="browser-monitor__text-stats">
                <span>{{ sessionDescriptorLabel(selectedSession) }}</span>
                <span>{{
                  trp('browserMonitor.textInteractiveCount', '{count} interactive elements', {
                    count: activeTextMonitor.interactiveCount,
                  })
                }}</span>
              </div>
            </div>
            <img
              v-else-if="screenshotSrc"
              :src="screenshotSrc"
              :alt="tr('browserMonitor.previewAlt', 'Browser preview')"
              class="browser-monitor__image"
            />
            <div v-else class="browser-monitor__frame-empty">
              <strong>{{ tr('browserMonitor.previewEmptyTitle', 'No preview frame yet') }}</strong>
              <span>{{
                screenshotLoading
                  ? tr('browserMonitor.previewLoading', 'Capturing the current viewport...')
                  : tr(
                      'browserMonitor.previewEmptyBody',
                      'The monitor will show screenshots as soon as a browser tab is active.'
                    )
              }}</span>
            </div>
          </div>

          <div v-if="!isTextMonitor && previewFrames.length > 1" class="browser-monitor__timeline">
            <button
              v-for="frame in previewFrames"
              :key="screenshotFrameKey(frame)"
              type="button"
              class="browser-monitor__timeline-shot"
              :class="{ 'is-active': screenshotFrameKey(frame) === activeScreenshotKey }"
              :title="frame.title || frame.url || formatRelativeTime(frame.captured_at)"
              @click="activeScreenshotKey = screenshotFrameKey(frame)"
            >
              <img
                :src="`data:image/png;base64,${frame.data}`"
                :alt="tr('browserMonitor.previewAlt', 'Browser preview')"
                class="browser-monitor__timeline-image"
              />
              <span class="browser-monitor__timeline-time">
                {{ formatRelativeTime(frame.captured_at) }}
              </span>
            </button>
          </div>

          <div v-if="sessions.length > 1" class="browser-monitor__tabs">
            <button
              v-for="session in sessions"
              :key="session.id"
              type="button"
              class="browser-monitor__tab"
              :class="{ 'is-active': session.id === selectedSession?.id }"
              @click="selectedSessionId = session.id"
            >
              <span
                class="browser-monitor__tab-dot"
                :class="{ 'is-active': session.status === 'active' }"
              />
              <span class="browser-monitor__tab-copy">
                <strong>{{
                  session.page_title || tr('browserMonitor.untitledTab', 'Untitled tab')
                }}</strong>
                <span class="browser-monitor__tab-meta">{{ sessionLayerLabel(session) }}</span>
              </span>
            </button>
          </div>

          <p v-if="screenshotError" class="browser-monitor__error">{{ screenshotError }}</p>
        </section>
      </div>

      <div v-if="!isCollapsed" class="browser-monitor__resize-grip" aria-hidden="true" />
    </section>
  </Teleport>
</template>

<style scoped>
.browser-monitor-launcher {
  position: fixed;
  z-index: 70;
  display: inline-flex;
  align-items: flex-start;
  gap: 0.58rem;
  width: min(11.1rem, calc(100vw - 1rem));
  max-width: calc(100vw - 1rem);
  padding: 0.62rem 0.72rem;
  border: 1px solid rgba(148, 163, 184, 0.24);
  border-radius: 0.92rem;
  background:
    linear-gradient(135deg, rgba(255, 247, 237, 0.96), rgba(239, 246, 255, 0.96)),
    rgba(255, 255, 255, 0.92);
  box-shadow: 0 18px 44px rgba(15, 23, 42, 0.16);
  backdrop-filter: blur(18px);
  cursor: grab;
  text-align: start;
  user-select: none;
  touch-action: none;
}

.browser-monitor-launcher:active {
  cursor: grabbing;
}

.browser-monitor-launcher.has-activity {
  border-color: rgba(245, 158, 11, 0.35);
  box-shadow: 0 18px 44px rgba(217, 119, 6, 0.18);
}

.browser-monitor-launcher__dot {
  width: 0.78rem;
  height: 0.78rem;
  border-radius: 999px;
  background: linear-gradient(135deg, #0ea5e9, #14b8a6);
  box-shadow: 0 0 0 0.28rem rgba(14, 165, 233, 0.16);
  flex-shrink: 0;
}

.browser-monitor-launcher.has-activity .browser-monitor-launcher__dot {
  background: linear-gradient(135deg, #f59e0b, #f97316);
  box-shadow: 0 0 0 0.28rem rgba(249, 115, 22, 0.18);
}

.browser-monitor-launcher__copy {
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
  min-width: 0;
}

.browser-monitor-launcher__copy strong {
  display: block;
  font-size: 0.78rem;
  line-height: 1.18;
  color: #0f172a;
  word-break: break-word;
}

.browser-monitor-launcher__copy span {
  display: block;
  font-size: 0.66rem;
  line-height: 1.22;
  color: #475569;
  word-break: break-word;
}

.browser-monitor__preview-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 0.45rem;
}

.browser-monitor__preview-chip {
  display: inline-flex;
  align-items: center;
  padding: 0.34rem 0.62rem;
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.14);
  color: rgba(226, 232, 240, 0.92);
  font-size: 0.68rem;
  font-weight: 600;
}

.browser-monitor {
  position: fixed;
  z-index: 70;
  display: flex;
  flex-direction: column;
  width: min(44rem, calc(100vw - 1.5rem));
  max-width: calc(100vw - 1rem);
  max-height: calc(100vh - 1rem);
  border: 1px solid rgba(148, 163, 184, 0.22);
  border-radius: 1.35rem;
  background:
    radial-gradient(circle at top left, rgba(251, 191, 36, 0.16), transparent 36%),
    radial-gradient(circle at top right, rgba(34, 197, 94, 0.1), transparent 30%),
    rgba(248, 250, 252, 0.94);
  box-shadow: 0 28px 70px rgba(15, 23, 42, 0.26);
  backdrop-filter: blur(22px);
  overflow: hidden;
}

.browser-monitor.is-resizable {
  resize: both;
}

.browser-monitor.is-collapsed {
  width: min(18.75rem, calc(100vw - 1rem));
  height: auto;
}

.browser-monitor__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.85rem;
  padding: 0.84rem 0.9rem 0.78rem;
  background: linear-gradient(135deg, rgba(15, 23, 42, 0.96), rgba(30, 41, 59, 0.88));
  color: #f8fafc;
  cursor: move;
  user-select: none;
  touch-action: none;
}

.browser-monitor.is-collapsed .browser-monitor__header {
  align-items: center;
  gap: 0.6rem;
  padding: 0.48rem 0.54rem 0.44rem;
}

.browser-monitor__title-group {
  min-width: 0;
}

.browser-monitor__eyebrow,
.browser-monitor__preview-eyebrow {
  display: inline-flex;
  font-size: 0.64rem;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: rgba(226, 232, 240, 0.76);
}

.browser-monitor__title {
  margin-top: 0.24rem;
  font-size: 0.96rem;
  font-weight: 700;
}

.browser-monitor__title.is-compact {
  margin-top: 0;
  font-size: 0.72rem;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.browser-monitor__subtitle {
  margin-top: 0.22rem;
  max-width: 34rem;
  font-size: 0.74rem;
  color: rgba(226, 232, 240, 0.8);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.browser-monitor__actions {
  display: flex;
  align-items: center;
  gap: 0.36rem;
}

.browser-monitor.is-collapsed .browser-monitor__actions {
  gap: 0.24rem;
}

.browser-monitor__icon-button {
  width: 1.86rem;
  height: 1.86rem;
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.08);
  color: #f8fafc;
  font-size: 0.88rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
}

.browser-monitor__icon-button:focus-visible {
  outline: 2px solid rgba(125, 211, 252, 0.88);
  outline-offset: 2px;
}

.browser-monitor.is-collapsed .browser-monitor__icon-button {
  width: 1.55rem;
  height: 1.55rem;
  font-size: 0.78rem;
}

.browser-monitor__icon-button.is-close {
  background: rgba(248, 113, 113, 0.16);
}

.browser-monitor__body {
  display: grid;
  grid-template-columns: 15.2rem minmax(0, 1fr);
  flex: 1;
  min-height: 0;
}

.browser-monitor__sidebar {
  display: flex;
  flex-direction: column;
  min-height: 0;
  gap: 0.72rem;
  padding: 0.82rem;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.84), rgba(241, 245, 249, 0.92)),
    rgba(255, 255, 255, 0.82);
  border-inline-end: 1px solid rgba(148, 163, 184, 0.18);
}

.browser-monitor__panel {
  padding: 0.72rem;
  border-radius: 1rem;
  background: rgba(255, 255, 255, 0.74);
  border: 1px solid rgba(226, 232, 240, 0.82);
}

.browser-monitor__panel--tasks {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-height: 0;
}

.browser-monitor__panel-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  margin-bottom: 0.62rem;
  font-size: 0.75rem;
  font-weight: 700;
  color: #0f172a;
}

.browser-monitor__hint {
  font-size: 0.64rem;
  color: #64748b;
}

.browser-monitor__capabilities {
  display: flex;
  flex-direction: column;
  gap: 0.48rem;
}

.browser-monitor__filters {
  display: flex;
  flex-wrap: wrap;
  gap: 0.38rem;
  margin-bottom: 0.62rem;
}

.browser-monitor__task-list {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-height: 0;
  overflow-y: auto;
  padding-inline-end: 0.15rem;
}

.browser-monitor__filter-chip {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0.28rem 0.54rem;
  border-radius: 999px;
  border: 1px solid rgba(203, 213, 225, 0.92);
  background: rgba(248, 250, 252, 0.86);
  color: #475569;
  font-size: 0.64rem;
  font-weight: 700;
  cursor: pointer;
}

.browser-monitor__filter-chip.is-active {
  border-color: rgba(14, 165, 233, 0.34);
  background: rgba(224, 242, 254, 0.94);
  color: #0369a1;
}

.browser-monitor__filter-chip:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.browser-monitor__capability {
  padding: 0.64rem;
  border-radius: 0.92rem;
  border: 1px solid rgba(148, 163, 184, 0.14);
}

.browser-monitor__capability.is-critical {
  background: linear-gradient(135deg, rgba(255, 251, 235, 0.92), rgba(255, 237, 213, 0.94));
}

.browser-monitor__capability.is-strong {
  background: linear-gradient(135deg, rgba(224, 242, 254, 0.94), rgba(207, 250, 254, 0.94));
}

.browser-monitor__capability.is-calm {
  background: linear-gradient(135deg, rgba(236, 253, 245, 0.94), rgba(220, 252, 231, 0.94));
}

.browser-monitor__capability.is-alert {
  background: linear-gradient(135deg, rgba(254, 242, 242, 0.94), rgba(255, 237, 213, 0.94));
}

.browser-monitor__capability.is-muted {
  background: linear-gradient(135deg, rgba(248, 250, 252, 0.96), rgba(241, 245, 249, 0.96));
}

.browser-monitor__capability-topline {
  display: flex;
  align-items: center;
  gap: 0.42rem;
}

.browser-monitor__priority-pill {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 2rem;
  padding: 0.16rem 0.4rem;
  border-radius: 999px;
  background: rgba(15, 23, 42, 0.08);
  font-size: 0.61rem;
  font-weight: 800;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: #0f172a;
}

.browser-monitor__capability-label {
  font-size: 0.71rem;
  font-weight: 700;
  color: #0f172a;
}

.browser-monitor__capability-detail {
  margin-top: 0.34rem;
  font-size: 0.68rem;
  line-height: 1.45;
  color: #334155;
}

.browser-monitor__task {
  width: 100%;
  padding: 0.68rem;
  border-radius: 0.92rem;
  border: 1px solid rgba(226, 232, 240, 0.86);
  background: rgba(248, 250, 252, 0.85);
  text-align: start;
}

.browser-monitor__task.is-clickable {
  cursor: pointer;
}

.browser-monitor__task + .browser-monitor__task {
  margin-top: 0.44rem;
}

.browser-monitor__task-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.42rem;
}

.browser-monitor__stage-pill {
  display: inline-flex;
  align-items: center;
  padding: 0.22rem 0.48rem;
  border-radius: 999px;
  font-size: 0.62rem;
  font-weight: 700;
}

.browser-monitor__stage-pill.is-info,
.browser-monitor__progress-bar.is-info {
  background: rgba(59, 130, 246, 0.18);
  color: #1d4ed8;
}

.browser-monitor__stage-pill.is-warn,
.browser-monitor__progress-bar.is-warn {
  background: rgba(245, 158, 11, 0.18);
  color: #b45309;
}

.browser-monitor__stage-pill.is-success,
.browser-monitor__progress-bar.is-success {
  background: rgba(34, 197, 94, 0.18);
  color: #15803d;
}

.browser-monitor__stage-pill.is-danger,
.browser-monitor__progress-bar.is-danger {
  background: rgba(239, 68, 68, 0.18);
  color: #b91c1c;
}

.browser-monitor__stage-pill.is-muted,
.browser-monitor__progress-bar.is-muted {
  background: rgba(148, 163, 184, 0.22);
  color: #475569;
}

.browser-monitor__task-meta {
  font-size: 0.64rem;
  color: #64748b;
}

.browser-monitor__task-title {
  display: block;
  margin-top: 0.42rem;
  font-size: 0.78rem;
  color: #0f172a;
}

.browser-monitor__task-subtitle {
  margin-top: 0.26rem;
  font-size: 0.68rem;
  color: #64748b;
  line-height: 1.45;
}

.browser-monitor__task-blocker {
  margin-top: 0.36rem;
  padding: 0.38rem 0.5rem;
  border-radius: 0.68rem;
  background: rgba(245, 158, 11, 0.12);
  color: #b45309;
  font-size: 0.66rem;
  line-height: 1.4;
}

.browser-monitor__progress-track {
  margin-top: 0.52rem;
  height: 0.32rem;
  border-radius: 999px;
  background: rgba(226, 232, 240, 0.9);
  overflow: hidden;
}

.browser-monitor__progress-bar {
  height: 100%;
  border-radius: inherit;
}

.browser-monitor__preview {
  display: flex;
  flex-direction: column;
  gap: 0.72rem;
  min-height: 0;
  min-width: 0;
  padding: 0.82rem;
  background:
    radial-gradient(circle at top, rgba(14, 165, 233, 0.1), transparent 28%),
    linear-gradient(180deg, rgba(15, 23, 42, 0.96), rgba(15, 23, 42, 0.9));
  color: #f8fafc;
  overflow: hidden;
}

.browser-monitor__preview-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.62rem;
}

.browser-monitor__preview-title {
  margin-top: 0.26rem;
  font-size: 0.92rem;
  font-weight: 700;
}

.browser-monitor__preview-url {
  margin-top: 0.22rem;
  font-size: 0.7rem;
  color: rgba(226, 232, 240, 0.72);
  word-break: break-all;
}

.browser-monitor__preview-badge {
  display: inline-flex;
  align-items: center;
  padding: 0.3rem 0.58rem;
  border-radius: 999px;
  background: rgba(14, 165, 233, 0.14);
  color: #bae6fd;
  font-size: 0.64rem;
  font-weight: 700;
  white-space: nowrap;
}

.browser-monitor__frame {
  flex: 1;
  min-height: 10.5rem;
  border-radius: 1rem;
  overflow: hidden;
  border: 1px solid rgba(148, 163, 184, 0.18);
  background:
    linear-gradient(135deg, rgba(15, 23, 42, 0.98), rgba(30, 41, 59, 0.96)), rgba(15, 23, 42, 0.96);
  display: flex;
  align-items: center;
  justify-content: center;
}

.browser-monitor__image {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: contain;
  background: rgba(15, 23, 42, 0.98);
}

.browser-monitor__text-monitor {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 0.9rem;
  width: 100%;
  height: 100%;
  padding: 1rem;
  color: #e2e8f0;
}

.browser-monitor__text-card {
  display: flex;
  flex-direction: column;
  gap: 0.42rem;
  padding: 0.82rem 0.9rem;
  border-radius: 0.82rem;
  background: rgba(15, 23, 42, 0.56);
  border: 1px solid rgba(148, 163, 184, 0.16);
}

.browser-monitor__text-card strong {
  font-size: 0.78rem;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: #bae6fd;
}

.browser-monitor__text-card p,
.browser-monitor__text-card pre {
  margin: 0;
  font-size: 0.84rem;
  line-height: 1.55;
  color: rgba(226, 232, 240, 0.9);
  white-space: pre-wrap;
  word-break: break-word;
}

.browser-monitor__text-card pre {
  font-family: 'SFMono-Regular', 'IBM Plex Mono', monospace;
}

.browser-monitor__text-stats {
  display: flex;
  gap: 0.55rem;
  flex-wrap: wrap;
}

.browser-monitor__text-stats span {
  display: inline-flex;
  align-items: center;
  padding: 0.3rem 0.58rem;
  border-radius: 999px;
  background: rgba(125, 211, 252, 0.12);
  color: #e0f2fe;
  font-size: 0.72rem;
  font-weight: 600;
}

.browser-monitor__frame-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.36rem;
  padding: 1.15rem;
  text-align: center;
  color: rgba(226, 232, 240, 0.82);
}

.browser-monitor__frame-empty strong {
  color: #f8fafc;
}

.browser-monitor__timeline {
  display: flex;
  gap: 0.55rem;
  overflow-x: auto;
  padding-bottom: 0.1rem;
}

.browser-monitor__timeline-shot {
  display: inline-flex;
  flex-direction: column;
  gap: 0.32rem;
  width: 5.6rem;
  padding: 0.32rem;
  border-radius: 0.82rem;
  border: 1px solid rgba(148, 163, 184, 0.18);
  background: rgba(30, 41, 59, 0.7);
  color: rgba(226, 232, 240, 0.8);
  cursor: pointer;
  flex-shrink: 0;
}

.browser-monitor__timeline-shot.is-active {
  border-color: rgba(125, 211, 252, 0.48);
  background: rgba(14, 165, 233, 0.14);
  color: #f8fafc;
}

.browser-monitor__timeline-image {
  display: block;
  width: 100%;
  aspect-ratio: 4 / 3;
  border-radius: 0.62rem;
  object-fit: cover;
  background: rgba(15, 23, 42, 0.94);
}

.browser-monitor__timeline-time {
  font-size: 0.62rem;
  font-weight: 700;
  white-space: nowrap;
}

.browser-monitor__tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  max-height: 6.5rem;
  overflow-y: auto;
  align-content: flex-start;
}

.browser-monitor__tab {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  min-width: 0;
  max-width: 100%;
  padding: 0.56rem 0.7rem;
  border-radius: 0.88rem;
  border: 1px solid rgba(148, 163, 184, 0.18);
  background: rgba(30, 41, 59, 0.7);
  color: #f8fafc;
  cursor: pointer;
}

.browser-monitor__tab.is-active {
  border-color: rgba(125, 211, 252, 0.44);
  background: rgba(14, 165, 233, 0.12);
}

.browser-monitor__tab-dot {
  width: 0.55rem;
  height: 0.55rem;
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.55);
  flex-shrink: 0;
}

.browser-monitor__tab-dot.is-active {
  background: #22c55e;
  box-shadow: 0 0 0 0.22rem rgba(34, 197, 94, 0.18);
}

.browser-monitor__tab-copy {
  display: flex;
  flex-direction: column;
  min-width: 0;
  gap: 0.16rem;
}

.browser-monitor__tab-copy strong,
.browser-monitor__tab-meta {
  display: block;
  max-width: 100%;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.browser-monitor__tab-copy strong {
  font-size: 0.72rem;
}

.browser-monitor__tab-meta {
  font-size: 0.64rem;
  color: rgba(226, 232, 240, 0.68);
}

.browser-monitor__tab-kind {
  display: inline-flex;
  align-self: flex-start;
  max-width: 100%;
  padding: 0.14rem 0.42rem;
  border-radius: 999px;
  background: rgba(125, 211, 252, 0.12);
  color: #dbeafe;
  font-size: 0.6rem;
  font-weight: 700;
  letter-spacing: 0.01em;
}

.browser-monitor__empty,
.browser-monitor__error {
  padding: 0.7rem;
  border-radius: 0.82rem;
  font-size: 0.7rem;
  line-height: 1.45;
}

.browser-monitor__empty {
  background: rgba(241, 245, 249, 0.9);
  color: #64748b;
}

.browser-monitor__error {
  background: rgba(254, 242, 242, 0.92);
  color: #b91c1c;
}

.browser-monitor__compact {
  display: flex;
  flex-direction: column;
  gap: 0.46rem;
  padding: 0.56rem;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.72), rgba(248, 250, 252, 0.9)),
    rgba(255, 255, 255, 0.78);
}

.browser-monitor__compact-strip {
  display: flex;
  gap: 0.34rem;
  overflow-x: auto;
  padding-bottom: 0.08rem;
}

.browser-monitor__compact-thumb {
  width: 2.9rem;
  aspect-ratio: 4 / 3;
  padding: 0;
  border-radius: 0.66rem;
  border: 1px solid rgba(148, 163, 184, 0.26);
  background: rgba(15, 23, 42, 0.14);
  overflow: hidden;
  flex-shrink: 0;
  cursor: pointer;
}

.browser-monitor__compact-thumb.is-active {
  border-color: rgba(14, 165, 233, 0.68);
  box-shadow: 0 0 0 1px rgba(125, 211, 252, 0.42);
}

.browser-monitor__compact-thumb:focus-visible {
  outline: 2px solid rgba(14, 165, 233, 0.82);
  outline-offset: 2px;
}

.browser-monitor__compact-thumb-image,
.browser-monitor__compact-image {
  display: block;
  width: 100%;
  height: 100%;
  background: rgba(15, 23, 42, 0.98);
}

.browser-monitor__compact-thumb-image {
  object-fit: cover;
}

.browser-monitor__compact-thumb-fallback {
  display: flex;
  width: 100%;
  height: 100%;
  align-items: center;
  justify-content: center;
  background:
    radial-gradient(circle at top, rgba(56, 189, 248, 0.22), transparent 55%),
    linear-gradient(135deg, rgba(15, 23, 42, 0.96), rgba(30, 41, 59, 0.94));
  color: #e2e8f0;
  font-size: 0.76rem;
  font-weight: 800;
}

.browser-monitor__compact-frame {
  position: relative;
  width: 100%;
  min-height: 9.8rem;
  aspect-ratio: 16 / 10;
  border-radius: 0.92rem;
  overflow: hidden;
  border: 1px solid rgba(148, 163, 184, 0.24);
  background:
    radial-gradient(circle at top, rgba(56, 189, 248, 0.12), transparent 32%),
    linear-gradient(180deg, rgba(15, 23, 42, 0.98), rgba(15, 23, 42, 0.94));
}

.browser-monitor__compact-image {
  object-fit: contain;
}

.browser-monitor__compact-text,
.browser-monitor__compact-frame-empty {
  display: flex;
  width: 100%;
  height: 100%;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 0.88rem;
  text-align: center;
}

.browser-monitor__compact-text {
  align-items: flex-start;
  justify-content: flex-start;
  gap: 0.5rem;
  text-align: left;
  color: #e2e8f0;
}

.browser-monitor__compact-text strong,
.browser-monitor__compact-frame-empty strong {
  font-size: 0.74rem;
  color: #f8fafc;
}

.browser-monitor__compact-text p,
.browser-monitor__compact-frame-empty span {
  margin: 0;
  font-size: 0.66rem;
  line-height: 1.45;
  color: rgba(226, 232, 240, 0.82);
  word-break: break-word;
}

.browser-monitor__resize-grip {
  position: absolute;
  inset-inline-end: 0;
  bottom: 0;
  width: 1.1rem;
  height: 1.1rem;
  pointer-events: none;
  background: linear-gradient(
    135deg,
    transparent 0 42%,
    rgba(148, 163, 184, 0.35) 42% 52%,
    transparent 52% 62%,
    rgba(148, 163, 184, 0.6) 62% 72%,
    transparent 72% 100%
  );
}

@media (max-width: 900px) {
  .browser-monitor {
    width: calc(100vw - 1rem);
  }

  .browser-monitor__body {
    grid-template-columns: 1fr;
  }

  .browser-monitor__sidebar {
    border-inline-end: 0;
    border-bottom: 1px solid rgba(148, 163, 184, 0.18);
  }
}

@media (max-width: 640px) {
  .browser-monitor {
    width: calc(100vw - 0.75rem);
  }

  .browser-monitor__header {
    padding: 0.74rem 0.74rem 0.68rem;
  }

  .browser-monitor.is-collapsed {
    width: calc(100vw - 0.75rem);
  }

  .browser-monitor-launcher {
    max-width: calc(100vw - 0.75rem);
    width: min(10.65rem, calc(100vw - 0.75rem));
  }
}

:global([data-theme='dark']) .browser-monitor-launcher {
  border-color: rgba(71, 85, 105, 0.48);
  background:
    linear-gradient(135deg, rgba(15, 23, 42, 0.96), rgba(30, 41, 59, 0.94)), rgba(15, 23, 42, 0.94);
}

:global([data-theme='dark']) .browser-monitor-launcher__copy strong {
  color: #f8fafc;
}

:global([data-theme='dark']) .browser-monitor-launcher__copy span {
  color: #cbd5e1;
}

:global([data-theme='dark']) .browser-monitor {
  border-color: rgba(71, 85, 105, 0.42);
  background:
    radial-gradient(circle at top left, rgba(245, 158, 11, 0.12), transparent 30%),
    radial-gradient(circle at top right, rgba(14, 165, 233, 0.12), transparent 28%),
    rgba(15, 23, 42, 0.9);
}

:global([data-theme='dark']) .browser-monitor__sidebar {
  background:
    linear-gradient(180deg, rgba(15, 23, 42, 0.9), rgba(15, 23, 42, 0.82)), rgba(15, 23, 42, 0.84);
  border-right-color: rgba(71, 85, 105, 0.38);
}

:global([data-theme='dark']) .browser-monitor__panel,
:global([data-theme='dark']) .browser-monitor__task {
  background: rgba(15, 23, 42, 0.72);
  border-color: rgba(71, 85, 105, 0.4);
}

:global([data-theme='dark']) .browser-monitor__filter-chip {
  border-color: rgba(71, 85, 105, 0.52);
  background: rgba(30, 41, 59, 0.88);
  color: #cbd5e1;
}

:global([data-theme='dark']) .browser-monitor__filter-chip.is-active {
  border-color: rgba(125, 211, 252, 0.34);
  background: rgba(14, 165, 233, 0.18);
  color: #bae6fd;
}

:global([data-theme='dark']) .browser-monitor__capability.is-alert {
  background: linear-gradient(135deg, rgba(127, 29, 29, 0.42), rgba(120, 53, 15, 0.34));
}

:global([data-theme='dark']) .browser-monitor__panel-heading,
:global([data-theme='dark']) .browser-monitor__capability-label,
:global([data-theme='dark']) .browser-monitor__task-title {
  color: #f8fafc;
}

:global([data-theme='dark']) .browser-monitor__hint,
:global([data-theme='dark']) .browser-monitor__capability-detail,
:global([data-theme='dark']) .browser-monitor__task-meta,
:global([data-theme='dark']) .browser-monitor__task-subtitle,
:global([data-theme='dark']) .browser-monitor__task-blocker,
:global([data-theme='dark']) .browser-monitor__empty {
  color: #cbd5e1;
}

:global([data-theme='dark']) .browser-monitor__empty {
  background: rgba(30, 41, 59, 0.82);
}

:global([data-theme='dark']) .browser-monitor__priority-pill {
  background: rgba(255, 255, 255, 0.08);
  color: #e2e8f0;
}

:global([data-theme='dark']) .browser-monitor__task-blocker {
  background: rgba(245, 158, 11, 0.18);
  color: #fde68a;
}

:global([data-theme='dark']) .browser-monitor__timeline-shot {
  background: rgba(15, 23, 42, 0.76);
  border-color: rgba(71, 85, 105, 0.4);
  color: #cbd5e1;
}

:global([data-theme='dark']) .browser-monitor__timeline-shot.is-active {
  border-color: rgba(125, 211, 252, 0.38);
  background: rgba(14, 165, 233, 0.18);
  color: #f8fafc;
}

:global([data-theme='dark']) .browser-monitor__compact {
  background:
    linear-gradient(180deg, rgba(15, 23, 42, 0.66), rgba(15, 23, 42, 0.82)), rgba(15, 23, 42, 0.76);
}

:global([data-theme='dark']) .browser-monitor__compact-thumb {
  border-color: rgba(71, 85, 105, 0.48);
  background: rgba(15, 23, 42, 0.72);
}

:global([data-theme='dark']) .browser-monitor__compact-thumb.is-active {
  border-color: rgba(125, 211, 252, 0.52);
  box-shadow: 0 0 0 1px rgba(125, 211, 252, 0.24);
}

:global([data-theme='dark']) .browser-monitor__resize-grip {
  background: linear-gradient(
    135deg,
    transparent 0 42%,
    rgba(100, 116, 139, 0.4) 42% 52%,
    transparent 52% 62%,
    rgba(148, 163, 184, 0.78) 62% 72%,
    transparent 72% 100%
  );
}
</style>
