<script setup lang="ts">
import { computed, ref, watch, onMounted, onUnmounted, nextTick, defineAsyncComponent } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useSystemStore } from '@/stores/system'
import { useAuthStore } from '@/stores/auth'
import { useChatStore } from '@/stores/chat'
import { usePreviewStore } from '@/stores/preview'
import { useThemeStore } from '@/stores/theme'
import type { WorkspaceFile, WorkspaceStats, WorkspaceTreeEntry } from '@/api/workspace'
import type { Conversation, Message } from '@/api/chat'
import { PagePermissions } from '@/constants/pagePermissions'
import { storeToRefs } from 'pinia'
import { useTauri } from '@/composables/useTauri'
import { getPreferredNetworkAddress, useNetwork } from '@/composables/useNetwork'
import { getLocaleDirection } from '@/i18n'
import { isLocalAbsolutePath } from '@/utils/localPath'
import { prefetchRoute } from '@/utils/prefetch'
import { getWorkspaceVisibleTokenCount } from '@/utils/workspaceTokenEstimate'
import { publicAsset } from '@/utils/publicAsset'
import { resetPreviewModeStatus } from '@/router'
import { extractLocalPathCandidatesFromCard } from '@/utils/workspaceGeneratedFiles'
const PreviewUpgradeForm = defineAsyncComponent(
  () => import('@/components/preview/PreviewUpgradeForm.vue')
)

type WorkspaceApiModule = typeof import('@/api/workspace')
type ChatApiModule = typeof import('@/api/chat')
type TypelessUtilsModule = typeof import('@/utils/typeless')

let workspaceApiModulePromise: Promise<WorkspaceApiModule> | null = null
let chatApiModulePromise: Promise<ChatApiModule> | null = null
let typelessUtilsModulePromise: Promise<TypelessUtilsModule> | null = null

async function loadWorkspaceApi() {
  if (!workspaceApiModulePromise) {
    workspaceApiModulePromise = import('@/api/workspace')
  }
  return (await workspaceApiModulePromise).workspaceApi
}

async function loadConversationApi() {
  if (!chatApiModulePromise) {
    chatApiModulePromise = import('@/api/chat')
  }
  return (await chatApiModulePromise).conversationApi
}

async function loadMessageApi() {
  if (!chatApiModulePromise) {
    chatApiModulePromise = import('@/api/chat')
  }
  return (await chatApiModulePromise).messageApi
}

async function loadTypelessUtils() {
  if (!typelessUtilsModulePromise) {
    typelessUtilsModulePromise = import('@/utils/typeless')
  }
  return await typelessUtilsModulePromise
}

interface NavItem {
  id: string
  name: string
  icon: string
  path?: string
  permission?: string
  permissions?: string[]
  adminOnly?: boolean
  action?: 'workspace'
}

interface CoreWorkspaceFileInfo {
  icon: string
  labelKey: string
  descKey: string
}

interface GeneratedWorkspaceFile {
  id: string
  path: string
  filename: string
  source: string
  conversationId: string
  conversationTitle: string
  messageId: string
  messageCreatedAt: string
}

interface WorkspaceTreeRow {
  entry: WorkspaceTreeEntry
  directGeneratedRecord: GeneratedWorkspaceFile | null
  generatedRecord: GeneratedWorkspaceFile | null
  isCurrentConversation: boolean
  isCurrentConversationDirectMatch: boolean
  isRecentCurrentConversationDirectMatch: boolean
}

const GENERATED_SCAN_CONVERSATION_LIMIT = 20
const GENERATED_SCAN_MESSAGE_LIMIT = 120
const GENERATED_REFRESH_DEBOUNCE_MS = 1200
const GENERATED_RECENT_WINDOW_MS = 2 * 60 * 1000

const coreWorkspaceFileInfo: Record<string, CoreWorkspaceFileInfo> = {
  'SOUL.md': { icon: '🧠', labelKey: 'workspace.label.soul', descKey: 'workspace.desc.soul' },
  'USER.md': { icon: '👤', labelKey: 'workspace.label.user', descKey: 'workspace.desc.user' },
  'IDENTITY.md': {
    icon: '🏷️',
    labelKey: 'workspace.label.identity',
    descKey: 'workspace.desc.identity',
  },
  'MEMORY.md': { icon: '💾', labelKey: 'workspace.label.memory', descKey: 'workspace.desc.memory' },
  'AGENTS.md': { icon: '📋', labelKey: 'workspace.label.agents', descKey: 'workspace.desc.agents' },
  'TOOLS.md': { icon: '🧰', labelKey: 'workspace.label.tools', descKey: 'workspace.desc.tools' },
}
const coreWorkspaceFileNames = new Set(Object.keys(coreWorkspaceFileInfo))
const GITHUB_REPO_URL = 'https://github.com/IceWhaleTech/ZimaOS-Blue'

const { t, te, locale } = useI18n()
const route = useRoute()
const router = useRouter()
const systemStore = useSystemStore()
const authStore = useAuthStore()
const chatStore = useChatStore()
const previewStore = usePreviewStore()
const themeStore = useThemeStore()
const { isTauri, platform, browserName, openInBrowser, revealInFileManager, startWindowDragging } =
  useTauri()
const {
  addresses: networkAddresses,
  loading: networkAddressesLoading,
  fetchAddresses: fetchNetworkAddresses,
} = useNetwork({ autoFetch: false })
const { health } = storeToRefs(systemStore)
const { isAdmin } = storeToRefs(authStore)
const { isPreviewMode } = storeToRefs(previewStore)

// Mobile menu state
const isOpen = ref(false)
const externalBrowserOpening = ref(false)
const networkAddressesRequested = ref(false)
const isRtl = computed(() => getLocaleDirection(locale.value) === 'rtl')

// Collapsed state (desktop only)
const SIDEBAR_COLLAPSED_KEY = 'sidebar-collapsed'
const isCollapsed = ref(false)
const CONFIGURATION_GROUP_EXPANDED_KEY = 'sidebar-configuration-expanded'
const LEGACY_SETTINGS_GROUP_EXPANDED_KEY = 'sidebar-settings-expanded'
const isConfigurationGroupExpanded = ref(true)
const isMac =
  typeof navigator !== 'undefined' && navigator.platform.toUpperCase().indexOf('MAC') >= 0
const showPreviewUpgradeModal = ref(false)
const showMacosDesktopDragRegion = computed(() => isTauri.value && platform.value === 'macos')

// Workspace panel state
const showWorkspacePanel = ref(false)
const activeWorkspaceTab = ref<'core' | 'generated'>('generated')
const workspaceDir = ref('')
const workspaceMetaRequested = ref(false)
const workspaceMetaLoading = ref(false)
const workspaceMetaLoaded = ref(false)
const workspaceFilesLoaded = ref(false)
const workspaceLoading = ref(false)
const workspaceError = ref('')
const workspaceFiles = ref<WorkspaceFile[]>([])
const workspaceStats = ref<WorkspaceStats | null>(null)

// Core workspace editor state
const coreEditingFile = ref<string | null>(null)
const coreEditDraft = ref('')
const coreSaving = ref(false)
const coreEditorRef = ref<HTMLTextAreaElement | null>(null)

// Generated files state
const generatedFilesLoaded = ref(false)
const generatedFilesLoading = ref(false)
const generatedFilesError = ref('')
const generatedWorkspaceFiles = ref<GeneratedWorkspaceFile[]>([])

// Real workspace tree state
const workspaceTreeLoaded = ref(false)
const workspaceTreeLoading = ref(false)
const workspaceTreeError = ref('')
const workspaceTreeRoot = ref('')
const workspaceTreeEntries = ref<WorkspaceTreeEntry[]>([])
const workspaceTreeCollapsedDirs = ref<Set<string>>(new Set())
const workspaceTreeShowLinkedOnly = ref(false)
const workspaceTreeFocusCurrentConversation = ref(true)
let workspaceGeneratedRefreshTimer: ReturnType<typeof setTimeout> | null = null

function getStorageItem(key: string): string | null {
  try {
    if (typeof localStorage === 'undefined' || typeof localStorage.getItem !== 'function') {
      return null
    }
    return localStorage.getItem(key)
  } catch {
    return null
  }
}

function setStorageItem(key: string, value: string): void {
  try {
    if (typeof localStorage === 'undefined' || typeof localStorage.setItem !== 'function') {
      return
    }
    localStorage.setItem(key, value)
  } catch {
    // Ignore storage failures in restricted environments.
  }
}

function tr(key: string, fallback: string, values?: Record<string, string | number>): string {
  if (te(key)) return values ? t(key, values) : t(key)
  return fallback
}

function translateNavLabel(key: string): string {
  if (te(key)) return t(key)
  const fallback = key.split('.').pop() || key
  return fallback.charAt(0).toUpperCase() + fallback.slice(1)
}

function getCoreFileInfo(name: string): CoreWorkspaceFileInfo {
  return coreWorkspaceFileInfo[name] || { icon: '📄', labelKey: '', descKey: '' }
}

function getCoreFileLabel(name: string): string {
  const info = getCoreFileInfo(name)
  const fallback = name.replace(/\.md$/i, '')
  return info.labelKey ? tr(info.labelKey, fallback) : fallback
}

function getCoreFileDesc(name: string): string {
  const info = getCoreFileInfo(name)
  return info.descKey ? tr(info.descKey, '') : ''
}

function formatTimestamp(value: string): string {
  const parsed = Date.parse(value)
  if (Number.isNaN(parsed)) return value
  return new Date(parsed).toLocaleString()
}

function messageTimeMs(value: string): number {
  const parsed = Date.parse(value)
  return Number.isNaN(parsed) ? 0 : parsed
}

function workspaceGeneratedMessageFingerprint(content: string): string {
  let hash = 0
  for (let index = 0; index < content.length; index += 1) {
    hash = (hash * 31 + content.charCodeAt(index)) | 0
  }
  return `${content.length}:${hash >>> 0}`
}

function normalizeConversationId(value: unknown): string {
  if (Array.isArray(value)) {
    return normalizeConversationId(value[0] ?? '')
  }
  return typeof value === 'string' ? value.trim() : ''
}

function toPathKey(value: string): string {
  const trimmed = value.trim()
  if (!trimmed) return ''
  const noTrailingSlash = trimmed.replace(/[\\/]+$/, '') || trimmed
  const normalized = noTrailingSlash.replace(/\\/g, '/')
  if (/^[a-zA-Z]:\//.test(normalized)) {
    return normalized.charAt(0).toLowerCase() + normalized.slice(1)
  }
  return normalized
}

function getAncestorPathKeys(path: string): string[] {
  const normalized = toPathKey(path)
  if (!normalized) return []
  const parts = normalized.split('/').filter(Boolean)
  if (parts.length <= 1) return []
  const first = parts[0]
  if (!first) return []

  const ancestors: string[] = []
  let current = first
  ancestors.push(current)
  for (let i = 1; i < parts.length - 1; i++) {
    current = `${current}/${parts[i]}`
    ancestors.push(current)
  }
  return ancestors
}

function isDescendantOfCollapsedDir(path: string, collapsed: Set<string>): boolean {
  for (const ancestor of getAncestorPathKeys(path)) {
    if (collapsed.has(ancestor)) return true
  }
  return false
}

function isWorkspaceTreeDir(entry: WorkspaceTreeEntry): boolean {
  return String(entry.type || '').toLowerCase() === 'dir'
}

function isWorkspaceTreeDirCollapsed(path: string): boolean {
  const key = toPathKey(path)
  if (!key) return false
  return workspaceTreeCollapsedDirs.value.has(key)
}

function toggleWorkspaceTreeDir(entry: WorkspaceTreeEntry): void {
  if (!isWorkspaceTreeDir(entry)) return
  const key = toPathKey(entry.path)
  if (!key) return
  const next = new Set(workspaceTreeCollapsedDirs.value)
  if (next.has(key)) {
    next.delete(key)
  } else {
    next.add(key)
  }
  workspaceTreeCollapsedDirs.value = next
}

function toggleWorkspaceTreeLinkedFilter(): void {
  workspaceTreeShowLinkedOnly.value = !workspaceTreeShowLinkedOnly.value
  if (workspaceTreeShowLinkedOnly.value) {
    workspaceTreeCollapsedDirs.value = new Set()
  }
}

function toggleWorkspaceTreeCurrentConversationFocus(): void {
  if (!workspaceTreeHasCurrentConversationMatches.value) return
  workspaceTreeFocusCurrentConversation.value = !workspaceTreeFocusCurrentConversation.value
  if (workspaceTreeFocusCurrentConversation.value) {
    workspaceTreeCollapsedDirs.value = new Set()
  }
}

function treeIndentStyle(depth: number): { paddingInlineStart: string } {
  const indent = Math.max(0, depth - 1) * 16
  return { paddingInlineStart: `${indent}px` }
}

function formatBytes(bytes: number | undefined): string {
  if (typeof bytes !== 'number' || Number.isNaN(bytes) || bytes < 0) return ''
  if (bytes < 1024) return `${bytes} B`
  const units = ['KB', 'MB', 'GB', 'TB']
  let value = bytes / 1024
  let idx = 0
  while (value >= 1024 && idx < units.length - 1) {
    value /= 1024
    idx++
  }
  return `${value.toFixed(value >= 10 ? 0 : 1)} ${units[idx]}`
}

async function loadWhitelistWorkspaceTreeEntries(
  workspaceRootPath: string
): Promise<WorkspaceTreeEntry[]> {
  void workspaceRootPath
  return []
}

function buildGeneratedFileRecord(
  path: string,
  source: string,
  conversation: Conversation,
  message: Message
): GeneratedWorkspaceFile {
  const normalized = path.trim()
  const clean = normalized.replace(/[\\/]+$/, '') || normalized
  const filename = clean.split(/[/\\]/).pop() || clean
  const safePath = clean.replace(/[^a-zA-Z0-9_.-]+/g, '_').slice(-120)
  const title =
    String(conversation.title || '').trim() ||
    tr('nav.workspaceUnknownConversation', 'Conversation')
  return {
    id: `${conversation.id}:${message.id}:${safePath}`,
    path: clean,
    filename,
    source,
    conversationId: conversation.id,
    conversationTitle: title,
    messageId: message.id,
    messageCreatedAt: message.created_at,
  }
}

async function extractGeneratedFilesFromMessage(
  conversation: Conversation,
  message: Message,
  workspaceRootPath: string
): Promise<GeneratedWorkspaceFile[]> {
  if (!message.content || !message.content.trim()) return []

  const { parseTypelessContent } = await loadTypelessUtils()
  const parsed = parseTypelessContent(message.content, message.id, conversation.id)
  if (!parsed.cards.length) return []

  const records: GeneratedWorkspaceFile[] = []
  const seenPaths = new Set<string>()

  for (const card of parsed.cards) {
    const source = card.type || 'card'
    const candidates = extractLocalPathCandidatesFromCard(card, workspaceRootPath)
    for (const candidate of candidates) {
      if (seenPaths.has(candidate)) continue
      seenPaths.add(candidate)
      records.push(buildGeneratedFileRecord(candidate, source, conversation, message))
    }
  }

  return records
}

async function ensureWorkspaceMeta(options?: { silent?: boolean }) {
  if (workspaceMetaLoaded.value) return
  if (workspaceMetaLoading.value) return

  const silent = options?.silent === true
  workspaceMetaRequested.value = true
  workspaceMetaLoading.value = true
  try {
    const workspaceApi = await loadWorkspaceApi()
    const res = await workspaceApi.getMeta()
    workspaceDir.value = String(res.data?.dir || '').trim()
    workspaceMetaLoaded.value = true
  } catch (e) {
    if (!silent) {
      console.error('Failed to load workspace metadata:', e)
      workspaceError.value = tr('nav.workspaceLoadFailed', 'Failed to load workspace details')
    }
  } finally {
    workspaceMetaLoading.value = false
  }
}

async function ensureWorkspaceFiles(force = false) {
  if (workspaceFilesLoaded.value && !force) return

  workspaceLoading.value = true
  workspaceError.value = ''
  try {
    const workspaceApi = await loadWorkspaceApi()
    const [filesRes, statsRes] = await Promise.all([
      workspaceApi.listFiles(),
      workspaceApi.getStats(),
    ])
    workspaceFiles.value = Array.isArray(filesRes.data?.files) ? filesRes.data.files : []
    workspaceStats.value = statsRes.data || null
    workspaceFilesLoaded.value = true
  } catch (e) {
    console.error('Failed to load workspace files:', e)
    workspaceError.value = tr('nav.workspaceFilesLoadFailed', 'Failed to load workspace files')
  } finally {
    workspaceLoading.value = false
  }
}

async function ensureWorkspaceTree(force = false) {
  if (workspaceTreeLoaded.value && !force) return

  workspaceTreeLoading.value = true
  workspaceTreeError.value = ''
  try {
    const workspaceApi = await loadWorkspaceApi()
    const res = await workspaceApi.getTree({ max_depth: 16 })
    workspaceTreeRoot.value = String(res.data?.root || '').trim()
    const baseEntries = Array.isArray(res.data?.entries) ? res.data.entries : []
    const whitelistEntries = await loadWhitelistWorkspaceTreeEntries(workspaceTreeRoot.value)
    workspaceTreeEntries.value = [...baseEntries, ...whitelistEntries]
    workspaceTreeCollapsedDirs.value = new Set()
    workspaceTreeLoaded.value = true
    if (!workspaceDir.value && workspaceTreeRoot.value) {
      workspaceDir.value = workspaceTreeRoot.value
    }
  } catch (e) {
    console.error('Failed to load workspace tree:', e)
    workspaceTreeError.value = tr(
      'nav.workspaceTreeLoadFailed',
      'Failed to load workspace directory tree'
    )
  } finally {
    workspaceTreeLoading.value = false
  }
}

async function ensureGeneratedWorkspaceFiles(force = false) {
  if (generatedFilesLoaded.value && !force) return

  generatedFilesLoading.value = true
  generatedFilesError.value = ''
  try {
    const [conversationApi, messageApi] = await Promise.all([
      loadConversationApi(),
      loadMessageApi(),
    ])
    if (!workspaceDir.value.trim()) {
      await ensureWorkspaceMeta({ silent: true })
    }
    const workspaceRootPath = workspaceDir.value.trim() || workspaceTreeRoot.value.trim()
    const convRes = await conversationApi.list(GENERATED_SCAN_CONVERSATION_LIMIT, 0)
    const conversations = Array.isArray(convRes.data) ? convRes.data : []

    const allRecords: GeneratedWorkspaceFile[] = []

    for (const conversation of conversations) {
      try {
        const msgRes = await messageApi.list(conversation.id, GENERATED_SCAN_MESSAGE_LIMIT, 0)
        const messages = Array.isArray(msgRes.data) ? msgRes.data : []
        for (const message of messages) {
          allRecords.push(
            ...(await extractGeneratedFilesFromMessage(conversation, message, workspaceRootPath))
          )
        }
      } catch (e) {
        console.error('Failed to scan messages for generated files:', e)
      }
    }

    allRecords.sort((a, b) => messageTimeMs(b.messageCreatedAt) - messageTimeMs(a.messageCreatedAt))

    // Deduplicate by conversation + path, keep latest record.
    const deduped = new Map<string, GeneratedWorkspaceFile>()
    for (const record of allRecords) {
      const key = `${record.conversationId}::${record.path}`
      if (!deduped.has(key)) deduped.set(key, record)
    }

    generatedWorkspaceFiles.value = [...deduped.values()].sort(
      (a, b) => messageTimeMs(b.messageCreatedAt) - messageTimeMs(a.messageCreatedAt)
    )
    generatedFilesLoaded.value = true
  } catch (e) {
    console.error('Failed to load generated workspace files:', e)
    generatedFilesError.value = tr(
      'nav.workspaceGeneratedLoadFailed',
      'Failed to load generated files'
    )
  } finally {
    generatedFilesLoading.value = false
  }
}

async function refreshGeneratedWorkspaceView() {
  clearWorkspaceGeneratedRefreshTimer()
  await Promise.all([ensureWorkspaceTree(true), ensureGeneratedWorkspaceFiles(true)])
}

function clearWorkspaceGeneratedRefreshTimer(): void {
  if (workspaceGeneratedRefreshTimer === null) return
  clearTimeout(workspaceGeneratedRefreshTimer)
  workspaceGeneratedRefreshTimer = null
}

function scheduleWorkspaceGeneratedRefresh(): void {
  if (!showWorkspacePanel.value || activeWorkspaceTab.value !== 'generated') return
  clearWorkspaceGeneratedRefreshTimer()
  workspaceGeneratedRefreshTimer = setTimeout(() => {
    workspaceGeneratedRefreshTimer = null
    void refreshGeneratedWorkspaceView().catch(() => {})
  }, GENERATED_REFRESH_DEBOUNCE_MS)
}

function startCoreEdit(file: WorkspaceFile) {
  coreEditingFile.value = file.name
  coreEditDraft.value = file.content || ''
  nextTick(() => {
    if (!coreEditorRef.value) return
    coreEditorRef.value.setSelectionRange(0, 0)
    coreEditorRef.value.focus()
  })
}

function cancelCoreEdit() {
  if (coreSaving.value) return
  coreEditingFile.value = null
  coreEditDraft.value = ''
}

async function saveCoreEdit(name: string) {
  if (!name) return
  coreSaving.value = true
  workspaceError.value = ''
  try {
    const workspaceApi = await loadWorkspaceApi()
    await workspaceApi.putFile(name, coreEditDraft.value)
    const target = workspaceFiles.value.find((file) => file.name === name)
    if (target) {
      target.content = coreEditDraft.value
      target.missing = false
    }
    coreEditingFile.value = null
    const statsRes = await workspaceApi.getStats()
    workspaceStats.value = statsRes.data || null
  } catch (e) {
    console.error('Failed to save workspace file:', e)
    workspaceError.value = t('common.saveFailed')
  } finally {
    coreSaving.value = false
  }
}

function closeWorkspacePanel() {
  clearWorkspaceGeneratedRefreshTimer()
  cancelCoreEdit()
  showWorkspacePanel.value = false
  activeWorkspaceTab.value = 'generated'
}

async function switchWorkspaceTab(tab: 'core' | 'generated') {
  activeWorkspaceTab.value = tab
  if (tab === 'core') {
    await ensureWorkspaceFiles()
    return
  }
  await refreshGeneratedWorkspaceView()
}

async function handleWorkspaceClick() {
  if (showWorkspacePanel.value) {
    closeWorkspacePanel()
    return
  }

  workspaceError.value = ''
  showWorkspacePanel.value = true
  await ensureWorkspaceMeta()
  if (activeWorkspaceTab.value === 'core') {
    await ensureWorkspaceFiles()
    return
  }
  await refreshGeneratedWorkspaceView()
}

async function handleOpenWorkspaceLocation() {
  if (!canRevealWorkspaceDir.value) return
  const revealed = await revealInFileManager(workspaceDir.value)
  if (!revealed) {
    workspaceError.value = tr('nav.workspaceOpenFailed', 'Unable to open workspace in file manager')
  }
}

async function handleOpenWorkspaceTreeEntry(entry: WorkspaceTreeEntry) {
  workspaceError.value = ''
  const targetPath = String(entry.abs_path || '').trim()
  const opened = await openInBrowser(targetPath)
  if (!opened) {
    workspaceError.value = tr('nav.workspaceTreeOpenFailed', 'Unable to open workspace file')
  }
}

async function handleRevealWorkspaceTreeEntry(entry: WorkspaceTreeEntry) {
  workspaceError.value = ''
  const targetPath = String(entry.abs_path || '').trim()
  const revealed = await revealInFileManager(targetPath)
  if (!revealed) {
    workspaceError.value = tr('nav.workspaceTreeRevealFailed', 'Unable to open file location')
  }
}

async function jumpToGeneratedConversation(file: GeneratedWorkspaceFile) {
  closeWorkspacePanel()
  isOpen.value = false
  await router.push({
    path: '/chat',
    query: { conversationId: file.conversationId },
  })
}

// Keyboard shortcut handler (Cmd+B on macOS, Alt+B on others)
function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && showWorkspacePanel.value) {
    if (coreEditingFile.value) {
      cancelCoreEdit()
    } else {
      closeWorkspacePanel()
    }
    return
  }

  const modifierKey = isMac ? e.metaKey : e.altKey
  if (modifierKey && e.key.toLowerCase() === 'b') {
    e.preventDefault()
    toggleCollapse()
  }
}

// Load collapsed state from localStorage and setup keyboard listener
onMounted(() => {
  const saved = getStorageItem(SIDEBAR_COLLAPSED_KEY)
  if (saved !== null) {
    isCollapsed.value = saved === 'true'
  }
  const savedConfigurationGroupExpanded =
    getStorageItem(CONFIGURATION_GROUP_EXPANDED_KEY) ??
    getStorageItem(LEGACY_SETTINGS_GROUP_EXPANDED_KEY)
  if (savedConfigurationGroupExpanded !== null) {
    isConfigurationGroupExpanded.value = savedConfigurationGroupExpanded === 'true'
  }
  if (isConfigurationChildRouteActive.value) {
    isConfigurationGroupExpanded.value = true
  }
  window.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  clearWorkspaceGeneratedRefreshTimer()
  window.removeEventListener('keydown', handleKeydown)
})

// Toggle collapsed state
function setSidebarCollapsed(collapsed: boolean) {
  isCollapsed.value = collapsed
  setStorageItem(SIDEBAR_COLLAPSED_KEY, String(collapsed))
}

function toggleCollapse() {
  setSidebarCollapsed(!isCollapsed.value)
}

function setConfigurationGroupExpanded(expanded: boolean) {
  isConfigurationGroupExpanded.value = expanded
  setStorageItem(CONFIGURATION_GROUP_EXPANDED_KEY, String(expanded))
}

function toggleConfigurationGroup() {
  if (isCollapsed.value) {
    setSidebarCollapsed(false)
  }
  setConfigurationGroupExpanded(!isConfigurationGroupExpanded.value)
}

// Close menu when route changes
watch(
  () => route.path,
  () => {
    isOpen.value = false
    closeWorkspacePanel()
    if (isConfigurationChildRouteActive.value) {
      setConfigurationGroupExpanded(true)
    }
  }
)

// Expose toggle function for parent components
defineExpose({
  toggle: () => {
    isOpen.value = !isOpen.value
  },
  open: () => {
    isOpen.value = true
  },
  close: () => {
    isOpen.value = false
  },
  isOpen,
  isCollapsed,
  workspacePanelOpen: showWorkspacePanel,
  toggleCollapse,
})

// Check if user has permission for a page
const hasPermission = (permission?: string, permissions?: string[]) => {
  if (Array.isArray(permissions) && permissions.length > 0) {
    return authStore.hasAnyPermission(permissions)
  }
  if (!permission) return true
  return authStore.hasPermission(permission)
}

// Define visible sidebar structure as primary entries plus a collapsible settings group.
const primaryNavItemsConfig: NavItem[] = [
  {
    id: 'dashboard',
    name: 'nav.dashboard',
    path: '/home',
    icon: 'M4.75 14.5a7.25 7.25 0 1114.5 0M12 14.5l3.1-3.1M9.5 18.75h5',
    permission: PagePermissions.HOME,
    adminOnly: true,
  },
  {
    id: 'chat',
    name: 'nav.chat',
    path: '/chat',
    icon: 'M8.25 10.5h.01M12 10.5h.01M15.75 10.5h.01M6.75 18.75L3.75 21V6.75A2.25 2.25 0 016 4.5h12a2.25 2.25 0 012.25 2.25v7.5A2.25 2.25 0 0118 16.5H8.25l-1.5 2.25z',
    permission: PagePermissions.CHAT,
  },
  {
    id: 'workspace',
    name: 'nav.workspace',
    icon: 'M12 6.25a2.25 2.25 0 100-4.5 2.25 2.25 0 000 4.5zm-6.5 11.5a2.25 2.25 0 100-4.5 2.25 2.25 0 000 4.5zm13 0a2.25 2.25 0 100-4.5 2.25 2.25 0 000 4.5zM12 8.5v3M12 11.5L7.5 14M12 11.5l4.5 2.5',
    action: 'workspace',
  },
]

const configurationNavItemsConfig: NavItem[] = [
  {
    id: 'channels',
    name: 'nav.channels',
    path: '/channels',
    icon: 'M4.75 6.75A2.25 2.25 0 017 4.5h10a2.25 2.25 0 012.25 2.25v6.5A2.25 2.25 0 0117 15.5h-6.5l-3.75 2.75V15.5H7a2.25 2.25 0 01-2.25-2.25v-6.5zM4.75 18.5h14.5',
    permission: PagePermissions.CHANNELS,
    adminOnly: true,
  },
  {
    id: 'automation',
    name: 'nav.automation',
    path: '/operations',
    icon: 'M12 4l1.4 3.6L17 9l-3.6 1.4L12 14l-1.4-3.6L7 9l3.6-1.4L12 4zm6.5 7.5l.75 1.75L21 14l-1.75.75L18.5 16.5l-.75-1.75L16 14l1.75-.75.75-1.75zM5.5 14.5l1 2.5L9 18l-2.5 1L5.5 21.5l-1-2.5L2 18l2.5-1 1-2.5z',
    permission: PagePermissions.AUTOMATION,
    adminOnly: true,
  },
  {
    id: 'plugins',
    name: 'nav.plugins',
    path: '/plugins',
    icon: 'M9 3v4.5M15 3v4.5M6 7.5h12M6 7.5v2.25a6 6 0 0012 0V7.5M12 15v6',
    permission: PagePermissions.PLUGINS,
    adminOnly: true,
  },
  {
    id: 'security',
    name: 'nav.security',
    path: '/security',
    icon: 'M12 3l7.5 3v5.25c0 4.18-2.86 8.1-7.5 9.75-4.64-1.65-7.5-5.57-7.5-9.75V6L12 3zm0 5.25v4.5m0 3h.01',
    permission: PagePermissions.SECURITY,
    adminOnly: true,
  },
  {
    id: 'settings',
    name: 'nav.settings',
    path: '/settings',
    icon: 'M11.25 4.5c.414-1.656 2.086-1.656 2.5 0a1.575 1.575 0 002.35.974c1.455-.886 3.122.781 2.236 2.236a1.575 1.575 0 00.974 2.35c1.656.414 1.656 2.086 0 2.5a1.575 1.575 0 00-.974 2.35c.886 1.455-.781 3.122-2.236 2.236a1.575 1.575 0 00-2.35.974c-.414 1.656-2.086 1.656-2.5 0a1.575 1.575 0 00-2.35-.974c-1.455.886-3.122-.781-2.236-2.236a1.575 1.575 0 00-.974-2.35c-1.656-.414-1.656-2.086 0-2.5a1.575 1.575 0 00.974-2.35c-.886-1.455.781-3.122 2.236-2.236a1.575 1.575 0 002.35-.974zM12 15a3 3 0 100-6 3 3 0 000 6z',
    permission: PagePermissions.SETTINGS,
    adminOnly: true,
  },
]

function localizeNavItem(item: NavItem): NavItem {
  return {
    ...item,
    name: translateNavLabel(item.name),
  }
}

function getNavItemTestId(item: NavItem): string {
  return `sidebar-nav-${item.id}`
}

function isNavItemVisible(item: NavItem): boolean {
  if (isPreviewMode.value) return true
  if (item.adminOnly && !isAdmin.value) return false
  return hasPermission(item.permission, item.permissions)
}

// Check if a nav item is active (handles trailing slashes and sub-paths)
const isActive = (itemPath: string) => {
  const currentPath = route.path.replace(/\/+$/, '') || '/'
  const navPath = itemPath.replace(/\/+$/, '') || '/'
  return currentPath === navPath || currentPath.startsWith(navPath + '/')
}

function isNavItemActive(item: NavItem): boolean {
  if (item.action === 'workspace') return showWorkspacePanel.value
  return typeof item.path === 'string' ? isActive(item.path) : false
}

function prefetchNavItem(item: NavItem) {
  if (!item.path) return
  const resolvedName = router.resolve(item.path).name
  if (typeof resolvedName !== 'string' || !resolvedName.trim()) return
  prefetchRoute(resolvedName)
}

function prefetchProfileRoute() {
  prefetchRoute('Profile')
}

async function handleNavItemClick(item: NavItem) {
  isOpen.value = false

  if (item.action === 'workspace') {
    await handleWorkspaceClick()
    return
  }

  if (item.path) {
    await router.push(item.path)
  }
}

const primaryNavItems = computed(() => {
  return primaryNavItemsConfig.filter(isNavItemVisible).map(localizeNavItem)
})

const configurationNavItems = computed(() => {
  return configurationNavItemsConfig.filter(isNavItemVisible).map(localizeNavItem)
})

const configurationLabel = computed(() => tr('nav.configuration', 'Configuration'))

const isConfigurationChildRouteActive = computed(() => {
  return configurationNavItems.value.some(
    (item) => typeof item.path === 'string' && isActive(item.path)
  )
})

const showConfigurationGroup = computed(() => {
  return configurationNavItems.value.length > 0
})

const sortedWorkspaceFiles = computed(() => {
  return [...workspaceFiles.value].sort((a, b) => a.name.localeCompare(b.name))
})

const coreWorkspaceFiles = computed(() => {
  return sortedWorkspaceFiles.value.filter((file) => coreWorkspaceFileNames.has(file.name))
})

const coreWorkspaceTokenTotal = computed(() => {
  return getWorkspaceVisibleTokenCount(workspaceStats.value, coreWorkspaceFileNames)
})

const activeConversationId = computed(() => {
  const storeConversationId = normalizeConversationId(chatStore.currentConversationId)
  if (storeConversationId) return storeConversationId
  return normalizeConversationId(route.query.conversationId)
})

const generatedRecordByAbsPathKey = computed(() => {
  const map = new Map<string, GeneratedWorkspaceFile>()
  for (const record of generatedWorkspaceFiles.value) {
    const key = toPathKey(record.path)
    if (!key || map.has(key)) continue
    map.set(key, record)
  }
  return map
})

const workspaceTreeRows = computed<WorkspaceTreeRow[]>(() => {
  const currentConversationId = activeConversationId.value
  const now = Date.now()
  const baseRows = workspaceTreeEntries.value.map((entry) => {
    const absPathKey = toPathKey(String(entry.abs_path || ''))
    const directGeneratedRecord = absPathKey
      ? generatedRecordByAbsPathKey.value.get(absPathKey) || null
      : null
    return { entry, directGeneratedRecord }
  })

  const inheritedRecordByTreePath = new Map<string, GeneratedWorkspaceFile>()
  const linkedRows = [...baseRows]
    .filter(
      (
        row
      ): row is {
        entry: WorkspaceTreeEntry
        directGeneratedRecord: GeneratedWorkspaceFile
      } => row.directGeneratedRecord !== null
    )
    .sort(
      (a, b) =>
        messageTimeMs(b.directGeneratedRecord.messageCreatedAt) -
        messageTimeMs(a.directGeneratedRecord.messageCreatedAt)
    )

  for (const row of linkedRows) {
    const entryPathKey = toPathKey(String(row.entry.path || ''))
    if (!entryPathKey) continue
    for (const ancestor of getAncestorPathKeys(entryPathKey)) {
      if (!inheritedRecordByTreePath.has(ancestor)) {
        inheritedRecordByTreePath.set(ancestor, row.directGeneratedRecord)
      }
    }
  }

  return baseRows.map((row) => {
    const entryPathKey = toPathKey(String(row.entry.path || ''))
    const generatedRecord =
      row.directGeneratedRecord ||
      (entryPathKey ? inheritedRecordByTreePath.get(entryPathKey) : null) ||
      null
    const isCurrentConversation =
      !!generatedRecord &&
      !!currentConversationId &&
      generatedRecord.conversationId === currentConversationId
    const isCurrentConversationDirectMatch =
      !!row.directGeneratedRecord &&
      !!currentConversationId &&
      row.directGeneratedRecord.conversationId === currentConversationId
    const isRecentCurrentConversationDirectMatch =
      isCurrentConversationDirectMatch &&
      now - messageTimeMs(row.directGeneratedRecord!.messageCreatedAt) <= GENERATED_RECENT_WINDOW_MS

    return {
      entry: row.entry,
      directGeneratedRecord: row.directGeneratedRecord,
      generatedRecord,
      isCurrentConversation,
      isCurrentConversationDirectMatch,
      isRecentCurrentConversationDirectMatch,
    }
  })
})

const workspaceTreeCurrentConversationPathKeys = computed(() => {
  const pathKeys = new Set<string>()
  for (const row of workspaceTreeRows.value) {
    if (!row.isCurrentConversation) continue
    const entryPathKey = toPathKey(String(row.entry.path || ''))
    if (!entryPathKey) continue
    pathKeys.add(entryPathKey)
    for (const ancestor of getAncestorPathKeys(entryPathKey)) {
      pathKeys.add(ancestor)
    }
  }
  return pathKeys
})

const workspaceTreeLinkedPathKeys = computed(() => {
  const pathKeys = new Set<string>()
  for (const row of workspaceTreeRows.value) {
    if (!row.generatedRecord) continue
    const selfKey = toPathKey(row.entry.path)
    if (!selfKey) continue
    pathKeys.add(selfKey)
    for (const ancestor of getAncestorPathKeys(selfKey)) {
      pathKeys.add(ancestor)
    }
  }
  return pathKeys
})

const workspaceTreeCurrentConversationCount = computed(() => {
  return workspaceTreeRows.value.filter(
    (row) => !isWorkspaceTreeDir(row.entry) && row.isCurrentConversationDirectMatch
  ).length
})

const workspaceTreeHasCurrentConversationMatches = computed(() => {
  return !!activeConversationId.value && workspaceTreeCurrentConversationCount.value > 0
})

const workspaceTreeVisibleRows = computed<WorkspaceTreeRow[]>(() => {
  const collapsed = workspaceTreeCollapsedDirs.value
  const linkedKeys = workspaceTreeShowLinkedOnly.value ? workspaceTreeLinkedPathKeys.value : null
  const currentConversationKeys =
    workspaceTreeFocusCurrentConversation.value && workspaceTreeHasCurrentConversationMatches.value
      ? workspaceTreeCurrentConversationPathKeys.value
      : null
  return workspaceTreeRows.value.filter((row) => {
    const rowKey = toPathKey(row.entry.path)
    if (!rowKey) return false
    if (linkedKeys && !linkedKeys.has(rowKey)) return false
    if (currentConversationKeys && !currentConversationKeys.has(rowKey)) return false
    if (isDescendantOfCollapsedDir(rowKey, collapsed)) return false
    return true
  })
})

const workspaceTreeDirectoryCount = computed(() => {
  return workspaceTreeRows.value.filter((row) => isWorkspaceTreeDir(row.entry)).length
})

const workspaceTreeFileCount = computed(() => {
  return workspaceTreeRows.value.filter((row) => !isWorkspaceTreeDir(row.entry)).length
})

const workspaceTreeLinkedCount = computed(() => {
  return workspaceTreeRows.value.filter((row) => row.generatedRecord).length
})

const workspaceGeneratedLoading = computed(() => {
  return workspaceTreeLoading.value || generatedFilesLoading.value
})

const workspaceTreeRootLabel = computed(() => {
  return workspaceTreeRoot.value || workspacePathLabel.value
})

const workspaceTreeEmptyLabel = computed(() => {
  if (workspaceTreeShowLinkedOnly.value) {
    return tr(
      'nav.workspaceTreeFilteredEmpty',
      'No conversation-linked files found in the current tree view.'
    )
  }
  return tr('nav.workspaceTreeEmpty', 'No files found in workspace tree.')
})

const workspaceTreeFilterLabel = computed(() => {
  if (workspaceTreeShowLinkedOnly.value) {
    return tr('nav.workspaceTreeShowAll', 'Show all files')
  }
  return tr('nav.workspaceTreeShowLinkedOnly', 'Only linked files')
})

const workspaceTreeCurrentConversationFilterLabel = computed(() => {
  if (
    workspaceTreeFocusCurrentConversation.value &&
    workspaceTreeHasCurrentConversationMatches.value
  ) {
    return tr('nav.workspaceTreeShowAllConversations', 'Show all conversations')
  }
  return tr('nav.workspaceTreeFocusCurrentConversation', 'Current conversation')
})

const workspaceGeneratedRefreshSignal = computed(() => {
  const conversationId = activeConversationId.value
  const tail = chatStore.messages
    .slice(-4)
    .map((message) => {
      const contentFingerprint = workspaceGeneratedMessageFingerprint(String(message.content || ''))
      return `${message.id}:${message.role}:${message.created_at}:${contentFingerprint}`
    })
    .join('|')
  return `${conversationId}::${tail}`
})

watch(activeConversationId, (next, previous) => {
  if (!next || next === previous) return
  workspaceTreeFocusCurrentConversation.value = true
  if (showWorkspacePanel.value && activeWorkspaceTab.value === 'generated') {
    workspaceTreeCollapsedDirs.value = new Set()
  }
})

watch(workspaceGeneratedRefreshSignal, () => {
  scheduleWorkspaceGeneratedRefresh()
})

watch(
  () => [showWorkspacePanel.value, activeWorkspaceTab.value] as const,
  ([panelOpen, activeTab]) => {
    if (!panelOpen || activeTab !== 'generated') {
      clearWorkspaceGeneratedRefreshTimer()
    }
  }
)

const canRevealWorkspaceDir = computed(() => {
  return isTauri.value && isLocalAbsolutePath(workspaceDir.value)
})

const openWorkspaceLocationLabel = computed(() => {
  if (platform.value === 'macos') return tr('nav.openWorkspaceInFinder', 'Open in Finder')
  if (platform.value === 'windows') return tr('nav.openWorkspaceInExplorer', 'Open in Explorer')
  return tr('nav.openWorkspaceIn', 'Open in file manager')
})

const workspacePathLabel = computed(() => {
  const dir = workspaceDir.value.trim()
  if (dir) return dir
  if (!workspaceMetaRequested.value || workspaceMetaLoading.value) {
    return tr('nav.workspacePathLoading', 'Loading workspace path...')
  }
  return tr('nav.workspacePathUnavailable', 'Workspace path unavailable')
})

const profileLabel = computed(() => tr('profile.title', 'Profile'))

const profileName = computed(() => {
  const username = String(authStore.user?.username || '').trim()
  return username || profileLabel.value
})

const profileInitial = computed(() => {
  const firstChar = profileName.value.trim().charAt(0)
  return firstChar ? firstChar.toUpperCase() : 'U'
})

const profileRole = computed(() => {
  const role = String(authStore.user?.role || '').trim()
  if (!role) return profileLabel.value
  return role.charAt(0).toUpperCase() + role.slice(1)
})

const isDarkTheme = computed(
  () =>
    themeStore.theme === 'dark' || (themeStore.theme === 'system' && themeStore.systemPrefersDark)
)
const sidebarStatusHealthy = computed(() => {
  const status = String(health.value?.status || '')
    .trim()
    .toLowerCase()
  return !status || status === 'ok'
})
const sidebarStatusLabel = computed(() => {
  if (sidebarStatusHealthy.value) return t('common.online')
  return String(health.value?.status || '').trim()
})
const preferredExternalBrowserAddress = computed(() =>
  getPreferredNetworkAddress(networkAddresses.value, { isTauri: isTauri.value })
)
const showExternalBrowserButton = computed(
  () => isTauri.value && (networkAddresses.value?.lan.length || 0) > 0
)
const showExpandedUtilityLabels = computed(
  () => !isCollapsed.value && !isPreviewMode.value && !showExternalBrowserButton.value
)
const externalBrowserButtonTitle = computed(() => {
  if (showExternalBrowserButton.value) {
    if (!te('network.openIn')) return `Open in ${browserName.value}`
    return t('network.openIn', { browser: browserName.value })
  }
  return tr('network.openInBrowser', 'Open in Browser')
})
const githubButtonTitle = computed(() => tr('brand.githubTooltip', 'Open GitHub'))
const githubButtonLabel = computed(() => 'GitHub')
const themeButtonLabel = computed(() =>
  isDarkTheme.value ? tr('common.light', 'Light') : tr('common.dark', 'Dark')
)
const themeButtonTitle = computed(() => {
  return `${tr('common.theme', 'Theme')}: ${themeButtonLabel.value}`
})

function openPreviewUpgradeModal(): void {
  showPreviewUpgradeModal.value = true
}

function closePreviewUpgradeModal(): void {
  showPreviewUpgradeModal.value = false
}

function handlePreviewUpgradeSuccess(): void {
  showPreviewUpgradeModal.value = false
  resetPreviewModeStatus()
  window.location.reload()
}

watch(
  isTauri,
  (desktop) => {
    if (!desktop || networkAddressesRequested.value) return
    networkAddressesRequested.value = true
    void fetchNetworkAddresses()
  },
  { immediate: true }
)

function openGithubRepo(): void {
  void openInBrowser(GITHUB_REPO_URL)
}

async function openExternalBrowser(): Promise<void> {
  if (!preferredExternalBrowserAddress.value || externalBrowserOpening.value) return

  externalBrowserOpening.value = true
  try {
    await openInBrowser(preferredExternalBrowserAddress.value)
  } finally {
    externalBrowserOpening.value = false
  }
}

function toggleSidebarTheme(): void {
  themeStore.setTheme(isDarkTheme.value ? 'light' : 'dark')
}

function handleWindowDragMouseDown(event: MouseEvent): void {
  if (!showMacosDesktopDragRegion.value || event.button !== 0) return
  void startWindowDragging()
}
</script>

<template>
  <!-- Mobile overlay -->
  <div
    v-if="isOpen"
    class="fixed inset-0 bg-black/50 z-40 lg:hidden"
    @click="isOpen = false"
  />

  <!-- Sidebar -->
  <aside
    class="app-sidebar h-full flex flex-col fixed inset-y-0 z-50 transform transition-all duration-300 ease-in-out lg:transform-none"
    :style="{ insetInlineStart: '0' }"
    :class="[
      isOpen
        ? 'translate-x-0'
        : isRtl
          ? 'translate-x-full lg:translate-x-0'
          : '-translate-x-full lg:translate-x-0',
      isCollapsed ? 'w-[4.125rem]' : 'w-[13rem]',
    ]"
  >
    <div
      v-if="showMacosDesktopDragRegion"
      class="sidebar-window-drag-region"
      aria-hidden="true"
      data-tauri-drag-region
      @mousedown="handleWindowDragMouseDown"
    />
    <div class="sidebar-card flex flex-1 min-h-0 flex-col">
      <div
        class="sidebar-brand-shell"
        :class="isCollapsed ? 'px-2 pb-2.5' : 'px-3.5 pb-2.5'"
      >
        <div class="sidebar-brand-row flex items-center justify-between gap-2">
          <RouterLink
            to="/"
            class="sidebar-brand"
            :class="[
              isCollapsed
                ? 'flex-1 justify-center px-1.5 py-1.5'
                : 'flex-1 justify-between px-1.5 py-1.5 gap-2.5',
            ]"
            :title="isCollapsed ? 'Blue' : undefined"
          >
            <div class="sidebar-brand-main">
              <div class="sidebar-brand-mark">
                <img
                  :src="publicAsset('logo.svg')"
                  alt="ZimaOS Blue"
                  class="h-6 w-6 object-contain dark:brightness-150"
                >
              </div>
              <div
                v-if="!isCollapsed"
                class="sidebar-brand-copy"
              >
                <span class="sidebar-brand-name">Blue</span>
              </div>
            </div>

            <span
              v-if="!isCollapsed"
              class="sidebar-status-badge"
              :class="
                sidebarStatusHealthy ? 'sidebar-status-badge-online' : 'sidebar-status-badge-alert'
              "
              :title="sidebarStatusLabel"
            >
              <span
                class="sidebar-status-dot"
                :class="
                  sidebarStatusHealthy ? 'sidebar-status-dot-online' : 'sidebar-status-dot-alert'
                "
                aria-hidden="true"
              />
              <span class="sidebar-status-label">{{ sidebarStatusLabel }}</span>
            </span>
          </RouterLink>

          <button
            class="sidebar-icon-btn sidebar-mobile-close lg:hidden p-1.5 rounded-lg text-gray-500 dark:text-slate-300 hover:text-gray-900 dark:hover:text-white"
            @click="isOpen = false"
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="h-[1.1rem] w-[1.1rem]"
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

      <nav class="px-3.5 py-1.5 flex-1 min-h-0 flex flex-col">
        <div class="space-y-1.5 flex-1 min-h-0 overflow-y-auto">
          <button
            v-for="item in primaryNavItems"
            :key="item.id"
            type="button"
            class="sidebar-nav-item w-full flex items-center px-3 py-[0.6rem] rounded-[1.1rem] transition-all duration-200 cursor-pointer group text-start"
            :class="[
              isNavItemActive(item) ? 'sidebar-nav-item-active' : 'sidebar-nav-item-inactive',
              isCollapsed ? 'justify-center' : 'gap-2.5',
            ]"
            :data-testid="getNavItemTestId(item)"
            :title="isCollapsed ? item.name : undefined"
            @mouseenter="prefetchNavItem(item)"
            @focus="prefetchNavItem(item)"
            @click="handleNavItemClick(item)"
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="h-[1.1rem] w-[1.1rem] transition-transform duration-200 group-hover:scale-110 flex-shrink-0"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="1.8"
                :d="item.icon"
              />
            </svg>
            <span
              v-if="!isCollapsed"
              class="text-[0.95rem] font-medium whitespace-nowrap"
            >{{
              item.name
            }}</span>
          </button>

          <div
            v-if="showConfigurationGroup"
            class="sidebar-section mt-3.5"
          >
            <button
              v-if="!isCollapsed"
              type="button"
              class="sidebar-section-trigger flex w-full items-center justify-between gap-2.5 px-2 py-1.5 text-start"
              data-testid="sidebar-section-configuration"
              :title="
                isConfigurationGroupExpanded
                  ? tr('common.collapse', 'Collapse')
                  : tr('common.expand', 'Expand')
              "
              :aria-expanded="isConfigurationGroupExpanded"
              @click="toggleConfigurationGroup"
            >
              <span class="sidebar-section-label">{{ configurationLabel }}</span>
              <span
                class="sidebar-section-chevron flex items-center justify-center rounded-md p-1 text-gray-500 dark:text-slate-400 transition-colors"
                aria-hidden="true"
              >
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  class="h-4 w-4 transition-transform duration-200"
                  :class="isConfigurationGroupExpanded ? '' : '-rotate-90'"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="1.8"
                    d="M7 10l5 5 5-5"
                  />
                </svg>
              </span>
            </button>

            <div
              v-if="(isCollapsed || isConfigurationGroupExpanded) && configurationNavItems.length"
              class="space-y-1.5"
              :class="isCollapsed ? 'mt-0' : 'mt-1.5'"
            >
              <button
                v-for="item in configurationNavItems"
                :key="item.id"
                type="button"
                class="sidebar-nav-item w-full flex items-center px-3 py-[0.6rem] rounded-[1.1rem] transition-all duration-200 cursor-pointer group text-start"
                :class="[
                  isNavItemActive(item) ? 'sidebar-nav-item-active' : 'sidebar-nav-item-inactive',
                  isCollapsed ? 'justify-center' : 'gap-2.5',
                ]"
                :data-testid="getNavItemTestId(item)"
                :title="isCollapsed ? item.name : undefined"
                @mouseenter="prefetchNavItem(item)"
                @focus="prefetchNavItem(item)"
                @click="handleNavItemClick(item)"
              >
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  class="h-[1.1rem] w-[1.1rem] transition-transform duration-200 group-hover:scale-110 flex-shrink-0"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="1.8"
                    :d="item.icon"
                  />
                </svg>
                <span
                  v-if="!isCollapsed"
                  class="text-[0.95rem] font-medium whitespace-nowrap"
                >{{
                  item.name
                }}</span>
              </button>
            </div>
          </div>
        </div>
      </nav>

      <!-- Bottom section: Version + Collapse toggle -->
      <div
        class="sidebar-footer px-3.5 pt-2.5 pb-3.5 border-t border-gray-200/45 dark:border-slate-700/60"
        :class="{ 'sidebar-footer-preview': isPreviewMode }"
      >
        <div
          v-if="!isPreviewMode"
          class="flex items-center"
          :class="isCollapsed ? 'justify-center' : 'justify-between'"
        >
          <!-- Version info -->
          <div
            v-if="health"
            class="sidebar-footer-meta text-[10px] text-gray-500 dark:text-slate-400 uppercase tracking-[0.12em]"
            :class="{ hidden: isCollapsed }"
          >
            v{{ health.version }}
          </div>
          <!-- Collapse toggle button (desktop only) -->
          <button
            class="sidebar-icon-btn hidden lg:flex p-1.5 rounded-full text-gray-400 hover:text-gray-700 dark:hover:text-slate-100 transition-colors"
            :title="
              (isCollapsed ? t('nav.expandSidebar') : t('nav.collapseSidebar')) +
                (isMac ? ' (⌘B)' : ' (Alt+B)')
            "
            @click="toggleCollapse"
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="h-4 w-4 transition-transform duration-300"
              :class="isCollapsed ? 'rotate-180' : ''"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M11 19l-7-7 7-7m8 14l-7-7 7-7"
              />
            </svg>
          </button>
        </div>

        <div
          class="sidebar-footer-actions"
          :class="[
            !isPreviewMode ? 'mt-3' : '',
            isPreviewMode
              ? isCollapsed
                ? 'sidebar-footer-actions-preview-collapsed'
                : 'sidebar-footer-actions-preview'
              : 'sidebar-footer-actions-default',
          ]"
        >
          <RouterLink
            v-if="!isPreviewMode"
            to="/profile"
            class="sidebar-account-link flex items-center"
            :class="[
              isActive('/profile')
                ? 'sidebar-account-link-active'
                : 'sidebar-account-link-inactive',
              isCollapsed
                ? 'justify-center mx-auto h-9 w-9 rounded-full'
                : 'gap-2 px-[0.5625rem] py-[0.4375rem] rounded-[0.9rem]',
            ]"
            data-testid="sidebar-nav-profile"
            :title="isCollapsed ? profileName : undefined"
            @mouseenter="prefetchProfileRoute"
            @focus="prefetchProfileRoute"
          >
            <span class="sidebar-profile-avatar flex-shrink-0">{{ profileInitial }}</span>
            <div
              v-if="!isCollapsed"
              class="min-w-0"
            >
              <div class="text-[0.86rem] font-medium leading-none truncate">
                {{ profileName }}
              </div>
              <div class="text-[10px] text-gray-500 dark:text-slate-400 truncate mt-0.5">
                {{ profileRole }}
              </div>
            </div>
          </RouterLink>

          <button
            v-if="isPreviewMode"
            type="button"
            class="sidebar-preview-create-btn"
            :class="[
              isCollapsed
                ? 'sidebar-preview-create-btn-collapsed'
                : 'sidebar-preview-create-btn-expanded',
            ]"
            data-testid="sidebar-preview-create-account"
            data-onboarding-anchor="preview-create-account"
            :title="t('preview.createAccount')"
            :aria-label="t('preview.createAccount')"
            @click="openPreviewUpgradeModal"
          >
            <span
              class="sidebar-preview-create-icon"
              aria-hidden="true"
            >+</span>
            <span
              v-if="!isCollapsed"
              class="sidebar-preview-create-label"
            >{{
              t('preview.createAccount')
            }}</span>
          </button>

          <div
            class="sidebar-utility-row"
            :class="[
              isCollapsed ? 'flex-col items-center' : '',
              isPreviewMode ? 'sidebar-utility-row-preview' : 'sidebar-utility-row-default',
            ]"
          >
            <button
              v-if="showExternalBrowserButton"
              type="button"
              class="sidebar-utility-button"
              data-testid="sidebar-open-external-browser"
              :title="externalBrowserButtonTitle"
              :aria-label="externalBrowserButtonTitle"
              :disabled="externalBrowserOpening || networkAddressesLoading"
              @click="openExternalBrowser"
            >
              <svg
                v-if="externalBrowserOpening || networkAddressesLoading"
                xmlns="http://www.w3.org/2000/svg"
                class="h-[1.05rem] w-[1.05rem] animate-spin"
                fill="none"
                viewBox="0 0 24 24"
                aria-hidden="true"
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
              <svg
                v-else
                xmlns="http://www.w3.org/2000/svg"
                class="h-[1.05rem] w-[1.05rem]"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                aria-hidden="true"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="1.8"
                  d="M10 6H6a2 2 0 0 0-2 2v10a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2v-4M14 4h6m0 0v6m0-6L10 14"
                />
              </svg>
            </button>

            <button
              type="button"
              class="sidebar-utility-button"
              :class="{ 'sidebar-utility-button-expanded': showExpandedUtilityLabels }"
              :title="githubButtonTitle"
              data-testid="sidebar-open-github"
              aria-label="Open GitHub"
              @click="openGithubRepo"
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="h-[1.05rem] w-[1.05rem]"
                viewBox="0 0 24 24"
                fill="currentColor"
                aria-hidden="true"
              >
                <path
                  d="M12 0C5.373 0 0 5.373 0 12c0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386C24 5.373 18.627 0 12 0z"
                />
              </svg>
              <span
                v-if="showExpandedUtilityLabels"
                class="sidebar-utility-label"
              >{{
                githubButtonLabel
              }}</span>
            </button>

            <button
              type="button"
              class="sidebar-utility-button"
              :class="{
                'sidebar-utility-button-active': isDarkTheme,
                'sidebar-utility-button-expanded': showExpandedUtilityLabels,
              }"
              :title="themeButtonTitle"
              :aria-label="themeButtonTitle"
              @click="toggleSidebarTheme"
            >
              <svg
                v-if="isDarkTheme"
                xmlns="http://www.w3.org/2000/svg"
                class="h-[1.05rem] w-[1.05rem]"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                aria-hidden="true"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="1.8"
                  d="M12 3v1.5m0 15V21m8.5-9H19m-14 0H3.5m14.51 6.01-1.06-1.06M7.05 7.05 5.99 5.99m12.02 0-1.06 1.06M7.05 16.95l-1.06 1.06M15.75 12a3.75 3.75 0 1 1-7.5 0 3.75 3.75 0 0 1 7.5 0Z"
                />
              </svg>
              <svg
                v-else
                xmlns="http://www.w3.org/2000/svg"
                class="h-[1.05rem] w-[1.05rem]"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                aria-hidden="true"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="1.8"
                  d="M21 12.79A9 9 0 1 1 11.21 3a7 7 0 0 0 9.79 9.79Z"
                />
              </svg>
              <span
                v-if="showExpandedUtilityLabels"
                class="sidebar-utility-label"
              >{{
                themeButtonLabel
              }}</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  </aside>

  <PreviewUpgradeForm
    v-if="showPreviewUpgradeModal"
    @close="closePreviewUpgradeModal"
    @success="handlePreviewUpgradeSuccess"
  />

  <Transition name="workspace-panel">
    <div
      v-if="showWorkspacePanel"
      class="workspace-panel-layer"
      @click.self="closeWorkspacePanel"
    >
      <div
        class="workspace-panel-shell border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 shadow-2xl overflow-hidden flex flex-col"
      >
        <div
          class="px-4 py-3 border-b border-gray-200 dark:border-gray-700 flex items-start justify-between gap-3"
        >
          <div class="min-w-0">
            <h3 class="text-base font-semibold text-gray-900 dark:text-white">
              {{ tr('nav.workspacePanelTitle', 'Workspace') }}
            </h3>
            <p class="text-xs text-gray-500 dark:text-slate-400 truncate mt-0.5">
              {{ workspacePathLabel }}
            </p>
            <p
              v-if="workspaceError"
              class="text-xs text-red-600 dark:text-red-400 mt-1"
            >
              {{ workspaceError }}
            </p>
          </div>
          <div class="flex items-center gap-2">
            <button
              v-if="canRevealWorkspaceDir"
              class="px-2.5 py-1.5 text-xs rounded-lg border border-gray-200 dark:border-gray-600 text-gray-700 dark:text-slate-200 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
              @click="handleOpenWorkspaceLocation"
            >
              {{ openWorkspaceLocationLabel }}
            </button>
            <button
              class="p-1.5 rounded-lg text-gray-500 dark:text-slate-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
              :title="tr('common.close', 'Close')"
              @click="closeWorkspacePanel"
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
          </div>
        </div>

        <div class="px-4 py-2 border-b border-gray-100 dark:border-gray-700/70">
          <div
            class="inline-flex rounded-lg border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/70 p-1"
          >
            <button
              class="px-3 py-1.5 text-xs rounded-md transition-colors"
              :class="
                activeWorkspaceTab === 'core'
                  ? 'bg-white dark:bg-gray-700 text-gray-900 dark:text-white shadow-sm'
                  : 'text-gray-600 dark:text-slate-300 hover:text-gray-900 dark:hover:text-white'
              "
              @click="switchWorkspaceTab('core')"
            >
              {{ tr('nav.workspaceCoreTab', 'Core Context Files') }}
            </button>
            <button
              class="px-3 py-1.5 text-xs rounded-md transition-colors"
              :class="
                activeWorkspaceTab === 'generated'
                  ? 'bg-white dark:bg-gray-700 text-gray-900 dark:text-white shadow-sm'
                  : 'text-gray-600 dark:text-slate-300 hover:text-gray-900 dark:hover:text-white'
              "
              @click="switchWorkspaceTab('generated')"
            >
              {{ tr('nav.workspaceGeneratedTab', 'Workspace Files') }}
            </button>
          </div>
        </div>

        <div class="flex-1 min-h-0 overflow-y-auto p-4">
          <div
            v-if="activeWorkspaceTab === 'core'"
            class="space-y-3"
          >
            <div class="flex items-center justify-between">
              <div>
                <h4 class="text-base font-semibold text-gray-900 dark:text-white">
                  {{ t('workspace.title') }}
                </h4>
                <p class="text-sm text-gray-500 dark:text-gray-400 mt-0.5">
                  {{ t('workspace.description') }}
                </p>
              </div>
              <span
                v-if="workspaceStats"
                class="text-xs px-2 py-1 rounded-full flex-shrink-0"
                :title="
                  tr('workspace.coreTokensHint', 'Counts only the core workspace files shown here.')
                "
                :class="
                  coreWorkspaceTokenTotal > 4096
                    ? 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400'
                    : 'bg-gray-100 text-gray-500 dark:bg-gray-700 dark:text-gray-400'
                "
              >
                {{
                  tr(
                    'workspace.coreTokens',
                    `~${coreWorkspaceTokenTotal.toLocaleString()} core-file tokens`,
                    { count: coreWorkspaceTokenTotal.toLocaleString() }
                  )
                }}
              </span>
            </div>

            <div
              v-if="workspaceLoading && coreWorkspaceFiles.length === 0"
              class="text-sm text-gray-500 dark:text-gray-400 py-4 text-center"
            >
              {{ tr('nav.workspaceLoading', 'Loading workspace files...') }}
            </div>

            <div
              v-else-if="coreWorkspaceFiles.length === 0"
              class="text-sm text-gray-500 dark:text-gray-400 py-4 text-center"
            >
              {{ t('workspace.noFiles') }}
            </div>

            <div
              v-else
              class="relative"
            >
              <Transition name="ws-grid">
                <div
                  v-if="!coreEditingFile"
                  class="grid grid-cols-2 sm:grid-cols-3 gap-2"
                >
                  <button
                    v-for="file in coreWorkspaceFiles"
                    :key="file.name"
                    class="ws-card group flex flex-col items-center gap-1 p-3 rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800/50 hover:border-gray-300 dark:hover:border-gray-600 hover:shadow-sm transition-all duration-150 cursor-pointer text-center"
                    @click="startCoreEdit(file)"
                  >
                    <span class="text-2xl leading-none">{{ getCoreFileInfo(file.name).icon }}</span>
                    <span
                      class="text-xs font-medium text-gray-900 dark:text-white truncate w-full"
                    >{{ getCoreFileLabel(file.name) }}</span>
                    <span class="text-[10px] text-gray-400 dark:text-gray-500 leading-tight">{{
                      getCoreFileDesc(file.name)
                    }}</span>
                  </button>
                </div>
              </Transition>

              <Transition name="ws-expand">
                <div
                  v-if="coreEditingFile"
                  class="rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800/80 overflow-hidden"
                >
                  <div
                    class="flex items-center justify-between px-4 py-2 border-b border-gray-100 dark:border-gray-700"
                  >
                    <div class="flex items-center gap-2 min-w-0">
                      <span class="text-lg flex-shrink-0">{{
                        getCoreFileInfo(coreEditingFile).icon
                      }}</span>
                      <span class="text-sm font-medium text-gray-900 dark:text-white truncate">{{
                        getCoreFileLabel(coreEditingFile)
                      }}</span>
                      <span
                        class="text-xs text-gray-400 dark:text-gray-500 hidden sm:inline flex-shrink-0"
                      >
                        {{ getCoreFileDesc(coreEditingFile) }}
                      </span>
                    </div>
                    <div class="flex items-center gap-1.5 flex-shrink-0">
                      <button
                        class="px-3 py-1 text-xs rounded-lg transition-colors bg-gray-800 dark:bg-gray-200 text-white dark:text-gray-900 hover:bg-gray-700 dark:hover:bg-gray-300 disabled:opacity-50"
                        :disabled="coreSaving"
                        @click="saveCoreEdit(coreEditingFile)"
                      >
                        {{ coreSaving ? t('common.saving') : t('common.save') }}
                      </button>
                      <button
                        class="px-3 py-1 text-xs rounded-lg transition-colors bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600 disabled:opacity-50"
                        :disabled="coreSaving"
                        @click="cancelCoreEdit"
                      >
                        {{ t('common.cancel') }}
                      </button>
                    </div>
                  </div>
                  <textarea
                    ref="coreEditorRef"
                    v-model="coreEditDraft"
                    rows="12"
                    class="w-full px-4 py-3 text-sm font-mono bg-transparent text-gray-800 dark:text-gray-200 focus:outline-none resize-y min-h-[240px]"
                    @keydown.meta.enter="saveCoreEdit(coreEditingFile!)"
                    @keydown.ctrl.enter="saveCoreEdit(coreEditingFile!)"
                  />
                  <div
                    class="px-4 py-1.5 text-[10px] text-gray-400 dark:text-gray-500 border-t border-gray-100 dark:border-gray-700 flex justify-between"
                  >
                    <span>Esc {{ t('common.cancel') }} · ⌘↵ {{ t('common.save') }}</span>
                    <span>{{
                      t('workspace.chars', { count: coreEditDraft.length.toLocaleString() })
                    }}</span>
                  </div>
                </div>
              </Transition>
            </div>
          </div>

          <div
            v-else
            class="space-y-3"
          >
            <div class="flex items-start justify-between gap-3">
              <h4 class="text-base font-semibold text-gray-900 dark:text-white">
                {{ tr('nav.workspaceGeneratedTitle', 'Workspace Directory Tree') }}
              </h4>
              <div
                class="flex items-center gap-1.5 text-[11px] text-gray-500 dark:text-gray-400 flex-wrap justify-end"
              >
                <span class="px-2 py-0.5 rounded bg-gray-100 dark:bg-gray-700">{{ tr('nav.workspaceTreeDirCount', 'Dirs') }}
                  {{ workspaceTreeDirectoryCount }}</span>
                <span class="px-2 py-0.5 rounded bg-gray-100 dark:bg-gray-700">{{ tr('nav.workspaceTreeFileCount', 'Files') }}
                  {{ workspaceTreeFileCount }}</span>
                <span
                  class="px-2 py-0.5 rounded bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300"
                >
                  {{ tr('nav.workspaceTreeLinkedCount', 'Linked') }} {{ workspaceTreeLinkedCount }}
                </span>
              </div>
            </div>
            <p class="text-sm text-gray-500 dark:text-gray-400 -mt-1">
              {{
                tr(
                  'nav.workspaceGeneratedDescription',
                  'Real workspace directory tree with source conversation links for generated files.'
                )
              }}
            </p>
            <div class="flex flex-wrap items-center gap-2">
              <button
                v-if="workspaceTreeHasCurrentConversationMatches"
                data-testid="workspace-current-conversation-filter"
                class="px-2.5 py-1 text-xs rounded-lg border transition-colors inline-flex items-center gap-1.5"
                :class="
                  workspaceTreeFocusCurrentConversation
                    ? 'border-blue-500 bg-blue-600 text-white hover:bg-blue-500'
                    : 'border-blue-200 bg-blue-50 text-blue-700 hover:bg-blue-100 dark:border-blue-800 dark:bg-blue-950/30 dark:text-blue-200 dark:hover:bg-blue-900/40'
                "
                @click="toggleWorkspaceTreeCurrentConversationFocus"
              >
                <span
                  class="h-1.5 w-1.5 rounded-full"
                  :class="
                    workspaceTreeFocusCurrentConversation ? 'bg-white animate-pulse' : 'bg-blue-500'
                  "
                />
                <span>{{ workspaceTreeCurrentConversationFilterLabel }}</span>
                <span
                  class="px-1.5 py-0.5 rounded-full text-[10px]"
                  :class="
                    workspaceTreeFocusCurrentConversation
                      ? 'bg-white/20 text-white'
                      : 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-200'
                  "
                >
                  {{ workspaceTreeCurrentConversationCount }}
                </span>
              </button>
              <button
                class="px-2.5 py-1 text-xs rounded-lg border border-gray-200 dark:border-gray-700 text-gray-700 dark:text-slate-200 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
                @click="toggleWorkspaceTreeLinkedFilter"
              >
                {{ workspaceTreeFilterLabel }}
              </button>
            </div>

            <p
              v-if="workspaceTreeError"
              class="text-xs text-red-600 dark:text-red-400"
            >
              {{ workspaceTreeError }}
            </p>
            <p
              v-if="generatedFilesError"
              class="text-xs text-red-600 dark:text-red-400"
            >
              {{ generatedFilesError }}
            </p>

            <div
              v-if="workspaceGeneratedLoading"
              class="text-sm text-gray-500 dark:text-gray-400 py-4 text-center"
            >
              {{
                tr('nav.workspaceTreeLoading', 'Loading workspace tree and conversation links...')
              }}
            </div>

            <div
              v-else-if="workspaceTreeVisibleRows.length === 0"
              class="text-sm text-gray-500 dark:text-gray-400 py-4 text-center"
            >
              {{ workspaceTreeEmptyLabel }}
            </div>

            <div
              v-else
              class="rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800/60 overflow-hidden"
            >
              <div
                class="px-3 py-2 text-xs text-gray-600 dark:text-slate-300 border-b border-gray-100 dark:border-gray-700 truncate"
              >
                {{ workspaceTreeRootLabel }}
              </div>
              <div class="max-h-[52vh] overflow-y-auto lg:max-h-none lg:overflow-visible">
                <div
                  v-for="row in workspaceTreeVisibleRows"
                  :key="row.entry.abs_path || row.entry.path"
                  class="px-3 py-2 border-b last:border-b-0 border-gray-100 dark:border-gray-700/70 transition-colors duration-200"
                  :class="
                    row.isCurrentConversation
                      ? 'bg-blue-50/80 dark:bg-blue-950/30'
                      : 'bg-transparent'
                  "
                  :data-current-conversation="row.isCurrentConversation ? 'true' : 'false'"
                >
                  <div class="flex items-start justify-between gap-3">
                    <div class="min-w-0 flex-1">
                      <div
                        class="flex items-center gap-2 min-w-0"
                        :style="treeIndentStyle(row.entry.depth)"
                      >
                        <span class="text-sm leading-none">{{
                          isWorkspaceTreeDir(row.entry) ? '📁' : '📄'
                        }}</span>
                        <p class="text-sm font-medium text-gray-900 dark:text-white truncate">
                          {{ row.entry.name }}
                        </p>
                        <span
                          v-if="row.isCurrentConversation"
                          class="px-1.5 py-0.5 rounded-full text-[10px] font-medium bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-200 flex-shrink-0"
                        >
                          {{
                            tr('nav.workspaceTreeCurrentConversationBadge', 'Current conversation')
                          }}
                        </span>
                        <span
                          v-if="row.isRecentCurrentConversationDirectMatch"
                          class="px-1.5 py-0.5 rounded-full text-[10px] font-medium bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200 flex items-center gap-1 flex-shrink-0 animate-pulse"
                        >
                          <span class="h-1.5 w-1.5 rounded-full bg-emerald-500" />
                          {{ tr('nav.workspaceTreeRecentGeneratedBadge', 'New') }}
                        </span>
                        <button
                          v-if="isWorkspaceTreeDir(row.entry)"
                          class="h-4 w-4 rounded text-gray-500 dark:text-slate-300 hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center justify-center transition-colors flex-shrink-0"
                          :title="
                            isWorkspaceTreeDirCollapsed(row.entry.path)
                              ? tr('nav.workspaceTreeExpandDir', 'Expand folder')
                              : tr('nav.workspaceTreeCollapseDir', 'Collapse folder')
                          "
                          @click.stop="toggleWorkspaceTreeDir(row.entry)"
                        >
                          <svg
                            xmlns="http://www.w3.org/2000/svg"
                            class="h-3 w-3 transition-transform duration-150"
                            :class="isWorkspaceTreeDirCollapsed(row.entry.path) ? '' : 'rotate-90'"
                            viewBox="0 0 20 20"
                            fill="currentColor"
                          >
                            <path
                              fill-rule="evenodd"
                              d="M7.21 14.77a.75.75 0 01.02-1.06L10.94 10 7.23 6.29a.75.75 0 111.06-1.06l4.24 4.24a.75.75 0 010 1.06l-4.24 4.24a.75.75 0 01-1.08 0z"
                              clip-rule="evenodd"
                            />
                          </svg>
                        </button>
                        <span
                          v-if="
                            !isWorkspaceTreeDir(row.entry) &&
                              typeof row.entry.size_bytes === 'number'
                          "
                          class="text-[10px] px-1.5 py-0.5 rounded bg-gray-100 dark:bg-gray-700 text-gray-500 dark:text-gray-300 flex-shrink-0"
                        >
                          {{ formatBytes(row.entry.size_bytes) }}
                        </span>
                      </div>
                      <p
                        class="text-xs text-gray-500 dark:text-slate-400 truncate mt-0.5"
                        :style="treeIndentStyle(row.entry.depth)"
                      >
                        {{ row.entry.path }}
                      </p>
                      <p
                        v-if="row.generatedRecord"
                        class="text-[11px] mt-1 truncate"
                        :class="
                          row.isCurrentConversation
                            ? 'text-blue-700 dark:text-blue-200'
                            : 'text-gray-500 dark:text-slate-400'
                        "
                        :style="treeIndentStyle(row.entry.depth)"
                      >
                        {{ row.generatedRecord.conversationTitle }} ·
                        {{ formatTimestamp(row.generatedRecord.messageCreatedAt) }}
                      </p>
                    </div>
                    <div class="flex items-center gap-1.5 flex-wrap justify-end flex-shrink-0">
                      <button
                        v-if="!isWorkspaceTreeDir(row.entry)"
                        class="px-2 py-1 text-xs rounded bg-gray-800 text-white hover:bg-gray-700 transition-colors"
                        @click="handleOpenWorkspaceTreeEntry(row.entry)"
                      >
                        {{ tr('common.download', 'Download') }}
                      </button>
                      <button
                        v-if="isLocalAbsolutePath(row.entry.abs_path)"
                        class="px-2 py-1 text-xs rounded bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-slate-200 hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors"
                        @click="handleRevealWorkspaceTreeEntry(row.entry)"
                      >
                        {{ tr('common.openLocation', 'Open location') }}
                      </button>
                      <button
                        v-if="row.generatedRecord"
                        class="px-2 py-1 text-xs rounded bg-blue-600 text-white hover:bg-blue-500 transition-colors"
                        @click="jumpToGeneratedConversation(row.generatedRecord)"
                      >
                        {{ tr('nav.workspaceJumpToConversation', 'Go to conversation') }}
                      </button>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </Transition>
</template>

<style scoped>
.app-sidebar {
  --sidebar-brand-top-pad: 0.875rem;
  --sidebar-window-drag-height: 0px;
  position: fixed;
  background: transparent;
  backdrop-filter: none;
  -webkit-backdrop-filter: none;
}

html[data-blue-macos-glass='true'] .app-sidebar {
  --sidebar-brand-top-pad: 0.34rem;
  --sidebar-window-drag-height: 1.7rem;
}

.sidebar-card {
  display: flex;
  flex: 1 1 auto;
  min-height: 0;
  flex-direction: column;
  background:
    radial-gradient(circle at 50% 0%, rgba(14, 165, 233, 0.12), transparent 28%),
    linear-gradient(180deg, rgba(15, 23, 42, 0.84), rgba(15, 23, 42, 0.76));
  backdrop-filter: blur(22px);
  -webkit-backdrop-filter: blur(22px);
  border-inline-end: 1px solid rgba(148, 163, 184, 0.26);
  box-shadow: 18px 0 36px -28px rgba(2, 6, 23, 0.9);
}

html[data-blue-macos-glass='true'] .sidebar-card {
  background:
    radial-gradient(circle at 46% 0%, rgba(56, 189, 248, 0.14), transparent 28%),
    linear-gradient(180deg, rgba(15, 23, 42, 0.7), rgba(15, 23, 42, 0.58));
  backdrop-filter: blur(30px) saturate(1.16);
  -webkit-backdrop-filter: blur(30px) saturate(1.16);
  border-inline-end-color: rgba(148, 163, 184, 0.22);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.08),
    18px 0 36px -28px rgba(2, 6, 23, 0.54);
}

.sidebar-brand-shell {
  position: relative;
  padding-top: var(--sidebar-brand-top-pad);
}

.sidebar-window-drag-region {
  flex: 0 0 auto;
  height: var(--sidebar-window-drag-height);
  user-select: none;
  -webkit-user-select: none;
}

.sidebar-brand-row {
  align-items: center;
  gap: 0.65rem;
  width: 100%;
}

.sidebar-brand-shell::after {
  content: '';
  position: absolute;
  inset-inline-start: 1.25rem;
  inset-inline-end: 1.25rem;
  bottom: 0;
  height: 1px;
  background: linear-gradient(90deg, transparent, rgba(209, 213, 219, 0.72), transparent);
}

.sidebar-brand {
  display: flex;
  align-items: center;
  min-width: 0;
  border-radius: 1rem;
  border: 1px solid transparent;
  background: transparent;
  transition:
    background-color 0.2s ease,
    transform 0.2s ease;
}

.sidebar-brand-main {
  display: flex;
  align-items: center;
  min-width: 0;
  gap: 0.7rem;
}

.sidebar-brand:hover {
  background: rgba(243, 244, 246, 0.8);
  transform: translateY(-1px);
}

.sidebar-brand-mark {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 2rem;
  height: 2rem;
  flex-shrink: 0;
  border-radius: 0.7rem;
  border: 1px solid rgba(209, 213, 219, 0.95);
  background: #f9fafb;
}

.sidebar-brand-copy {
  min-width: 0;
  display: flex;
}

.sidebar-brand-name {
  font-size: 0.9rem;
  line-height: 1;
  font-weight: 650;
  color: #111827;
  letter-spacing: -0.02em;
}

.sidebar-status-badge {
  display: inline-flex;
  align-items: center;
  gap: 0.34rem;
  flex-shrink: 0;
  min-height: 1.42rem;
  padding: 0 0.48rem;
  border-radius: 999px;
  border: 1px solid rgba(148, 163, 184, 0.28);
  background: rgba(255, 255, 255, 0.78);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.58);
}

.sidebar-status-label {
  font-size: 0.58rem;
  line-height: 1;
  font-weight: 600;
  letter-spacing: 0.03em;
  white-space: nowrap;
}

.sidebar-status-dot {
  width: 0.34rem;
  height: 0.34rem;
  border-radius: 999px;
  flex-shrink: 0;
}

.sidebar-status-badge-online {
  color: #15803d;
  border-color: rgba(74, 222, 128, 0.34);
  background: rgba(240, 253, 244, 0.78);
}

.sidebar-status-dot-online {
  background: #22c55e;
  box-shadow: 0 0 0 3px rgba(34, 197, 94, 0.12);
}

.sidebar-status-badge-alert {
  color: #b91c1c;
  border-color: rgba(248, 113, 113, 0.34);
  background: rgba(254, 242, 242, 0.8);
}

.sidebar-status-dot-alert {
  background: #ef4444;
  box-shadow: 0 0 0 3px rgba(239, 68, 68, 0.12);
}

.sidebar-mobile-close {
  flex-shrink: 0;
}

.sidebar-icon-btn {
  border: 1px solid transparent;
}

.sidebar-icon-btn:hover {
  border-color: rgba(148, 163, 184, 0.35);
  background: rgba(148, 163, 184, 0.14);
}

.sidebar-nav-item {
  position: relative;
  border: 1px solid transparent;
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
  transition:
    background-color 0.2s ease,
    border-color 0.2s ease,
    color 0.2s ease,
    box-shadow 0.2s ease,
    transform 0.2s ease;
}

.sidebar-nav-item-active {
  color: #111827;
  background: rgba(255, 255, 255, 0.94);
  border-color: rgba(209, 213, 219, 0.96);
  box-shadow:
    0 1px 2px rgba(15, 23, 42, 0.06),
    inset 0 1px 0 rgba(255, 255, 255, 0.7);
}

.sidebar-nav-item-inactive {
  color: #4b5563;
}

.sidebar-nav-item-inactive:hover {
  color: #111827;
  background: rgba(243, 244, 246, 0.88);
  border-color: rgba(226, 232, 240, 0.96);
}

.sidebar-section-label {
  font-size: 0.76rem;
  line-height: 1;
  font-weight: 650;
  letter-spacing: -0.01em;
  color: #6b7280;
}

.sidebar-section-trigger {
  border: 1px solid transparent;
  border-radius: 1rem;
  color: inherit;
  transition:
    background-color 0.2s ease,
    border-color 0.2s ease,
    color 0.2s ease;
}

.sidebar-section-trigger:hover,
.sidebar-section-trigger:focus-visible {
  background: rgba(243, 244, 246, 0.82);
  border-color: rgba(226, 232, 240, 0.96);
  outline: none;
}

.sidebar-section-trigger:hover .sidebar-section-label,
.sidebar-section-trigger:focus-visible .sidebar-section-label {
  color: #374151;
}

.sidebar-footer {
  margin-top: auto;
}

.sidebar-footer-preview {
  padding-top: 0.45rem;
  padding-inline: 0.55rem;
  padding-bottom: 0.6rem;
  border-top-color: transparent !important;
}

.sidebar-footer-meta {
  opacity: 0.72;
}

.sidebar-account-link {
  width: 100%;
  max-width: 100%;
  border: 1px solid transparent;
  color: #4b5563;
  transition:
    background-color 0.2s ease,
    border-color 0.2s ease,
    color 0.2s ease,
    transform 0.2s ease;
}

.sidebar-account-link-inactive:hover {
  color: #111827;
  background: rgba(243, 244, 246, 0.82);
  border-color: rgba(226, 232, 240, 0.94);
}

.sidebar-account-link-active {
  color: #111827;
  background: rgba(255, 255, 255, 0.94);
  border-color: rgba(209, 213, 219, 0.96);
}

.sidebar-footer-actions-default {
  display: block;
}

.sidebar-footer-actions-preview {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  gap: 0.4rem;
}

.sidebar-footer-actions-preview-collapsed {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.55rem;
}

.sidebar-utility-row {
  display: flex;
  align-items: center;
  gap: 0.45rem;
}

.sidebar-utility-row-default {
  margin-top: 0.65rem;
}

.sidebar-utility-row-preview {
  margin-inline-start: auto;
  flex-shrink: 0;
  justify-content: flex-end;
  gap: 0.22rem;
}

.sidebar-utility-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 2.1rem;
  height: 2.1rem;
  border-radius: 999px;
  border: 1px solid rgba(209, 213, 219, 0.94);
  color: #4b5563;
  background: rgba(255, 255, 255, 0.9);
  transition:
    background-color 0.2s ease,
    border-color 0.2s ease,
    color 0.2s ease,
    transform 0.2s ease;
}

.sidebar-utility-button-expanded {
  flex: 1 1 0;
  width: auto;
  justify-content: flex-start;
  gap: 0.42rem;
  padding: 0 0.72rem;
  border-radius: 999px;
}

.sidebar-utility-button:hover {
  color: #111827;
  background: rgba(243, 244, 246, 0.9);
  border-color: rgba(203, 213, 225, 0.96);
  transform: translateY(-1px);
}

.sidebar-utility-button:disabled {
  cursor: not-allowed;
  opacity: 0.72;
  transform: none;
}

.sidebar-utility-label {
  font-size: 0.74rem;
  font-weight: 600;
  line-height: 1;
  white-space: nowrap;
}

.sidebar-utility-button-active {
  color: #1d4ed8;
  background: rgba(219, 234, 254, 0.92);
  border-color: rgba(147, 197, 253, 0.92);
}

.sidebar-utility-row-preview .sidebar-utility-button {
  width: 1.62rem;
  height: 1.62rem;
  border-color: transparent;
  background: transparent;
  box-shadow: none;
}

.sidebar-utility-row-preview .sidebar-utility-button:hover {
  background: rgba(243, 244, 246, 0.82);
  border-color: transparent;
}

.sidebar-utility-row-preview .sidebar-utility-button-active {
  color: #1f2937;
  background: rgba(243, 244, 246, 0.92);
  border-color: transparent;
}

.sidebar-preview-create-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid rgba(37, 99, 235, 0.2);
  transition:
    transform 0.2s ease,
    box-shadow 0.2s ease,
    filter 0.2s ease,
    background-color 0.2s ease,
    border-color 0.2s ease;
}

.sidebar-preview-create-btn-expanded {
  gap: 0.36rem;
  min-height: 1.95rem;
  padding: 0.4rem 0.65rem;
  border-radius: 999px;
  background: linear-gradient(135deg, #2563eb, #1d4ed8);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.18),
    0 14px 24px -18px rgba(37, 99, 235, 0.9);
}

.sidebar-preview-create-btn-collapsed {
  width: 1.95rem;
  height: 1.95rem;
  border-radius: 999px;
  background: linear-gradient(135deg, #2563eb, #1d4ed8);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.18),
    0 14px 24px -18px rgba(37, 99, 235, 0.9);
}

.sidebar-preview-create-btn:hover,
.sidebar-preview-create-btn:focus-visible {
  transform: translateY(-1px);
  filter: brightness(1.03);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.2),
    0 16px 26px -18px rgba(37, 99, 235, 0.95);
}

.sidebar-preview-create-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 0.78rem;
  color: #eff6ff;
  font-size: 0.9rem;
  font-weight: 600;
  line-height: 1;
}

.sidebar-preview-create-label {
  color: #f8fafc;
  font-size: 0.76rem;
  font-weight: 600;
  line-height: 1;
  white-space: nowrap;
}

.sidebar-profile-avatar {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 2rem;
  height: 2rem;
  border-radius: 999px;
  border: 1px solid rgba(191, 219, 254, 0.18);
  font-size: 0.74rem;
  font-weight: 700;
  letter-spacing: 0.01em;
  color: #eff6ff;
  background:
    radial-gradient(circle at 30% 28%, rgba(255, 255, 255, 0.2), transparent 54%),
    linear-gradient(135deg, rgba(59, 130, 246, 0.9), rgba(30, 64, 175, 0.92));
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.16),
    0 12px 20px -16px rgba(30, 64, 175, 0.72);
}

:root.light .app-sidebar,
[data-theme='light'] .app-sidebar {
  background: transparent;
}

:root.light .sidebar-card,
[data-theme='light'] .sidebar-card {
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.99), rgba(248, 249, 251, 0.98));
  border-inline-end-color: rgba(209, 213, 219, 0.9);
  box-shadow: 12px 0 26px -24px rgba(15, 23, 42, 0.1);
}

html.light[data-blue-macos-glass='true'] .app-sidebar,
html[data-theme='light'][data-blue-macos-glass='true'] .app-sidebar {
  background: transparent;
}

html.light[data-blue-macos-glass='true'] .sidebar-card,
html[data-theme='light'][data-blue-macos-glass='true'] .sidebar-card {
  background:
    radial-gradient(circle at 50% 0%, rgba(96, 165, 250, 0.18), transparent 30%),
    linear-gradient(180deg, rgba(255, 255, 255, 0.82), rgba(241, 245, 249, 0.72));
  border-inline-end-color: rgba(186, 203, 223, 0.68);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.76),
    12px 0 26px -24px rgba(148, 163, 184, 0.24);
}

html[dir='rtl'] .sidebar-card {
  box-shadow: -18px 0 36px -28px rgba(2, 6, 23, 0.9);
}

html[data-blue-macos-glass='true'][dir='rtl'] .sidebar-card {
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.08),
    -18px 0 36px -28px rgba(2, 6, 23, 0.54);
}

:root.light[dir='rtl'] .sidebar-card,
html[data-theme='light'][dir='rtl'] .sidebar-card {
  box-shadow: -12px 0 26px -24px rgba(15, 23, 42, 0.1);
}

html.light[data-blue-macos-glass='true'][dir='rtl'] .sidebar-card,
html[data-theme='light'][data-blue-macos-glass='true'][dir='rtl'] .sidebar-card {
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.76),
    -12px 0 26px -24px rgba(148, 163, 184, 0.24);
}

:root.light .sidebar-brand,
[data-theme='light'] .sidebar-brand {
  background: transparent;
}

:root.light .sidebar-brand:hover,
[data-theme='light'] .sidebar-brand:hover {
  background: rgba(243, 244, 246, 0.82);
}

:root.light .sidebar-brand-mark,
[data-theme='light'] .sidebar-brand-mark {
  background: #f9fafb;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.92);
}

:root.light .sidebar-brand-name,
[data-theme='light'] .sidebar-brand-name {
  color: #0f172a;
}

:root.light .sidebar-status-badge,
[data-theme='light'] .sidebar-status-badge {
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.86),
    0 8px 18px -18px rgba(15, 23, 42, 0.18);
}

:root.light .sidebar-brand-shell::after,
[data-theme='light'] .sidebar-brand-shell::after {
  background: linear-gradient(90deg, transparent, rgba(209, 213, 219, 0.95), transparent);
}

:root.light .sidebar-profile-avatar,
[data-theme='light'] .sidebar-profile-avatar {
  border-color: rgba(191, 219, 254, 0.9);
  color: #1e3a8a;
  background:
    radial-gradient(circle at 30% 30%, rgba(255, 255, 255, 0.92), transparent 54%),
    linear-gradient(135deg, rgba(238, 242, 255, 0.98), rgba(191, 219, 254, 0.96));
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.96),
    0 12px 20px -16px rgba(59, 130, 246, 0.24);
}

:root.light .sidebar-account-link {
  color: #4b5563;
}

:root.light .sidebar-utility-button,
[data-theme='light'] .sidebar-utility-button {
  color: #4b5563;
  background: rgba(255, 255, 255, 0.9);
}

:root.light .sidebar-utility-button-active,
[data-theme='light'] .sidebar-utility-button-active {
  color: #1d4ed8;
  background: rgba(219, 234, 254, 0.92);
  border-color: rgba(147, 197, 253, 0.92);
}

:root.light .sidebar-utility-row-preview .sidebar-utility-button,
[data-theme='light'] .sidebar-utility-row-preview .sidebar-utility-button {
  color: #4b5563;
  background: transparent;
  border-color: transparent;
}

:root.light .sidebar-utility-row-preview .sidebar-utility-button-active,
[data-theme='light'] .sidebar-utility-row-preview .sidebar-utility-button-active {
  color: #1f2937;
  background: rgba(243, 244, 246, 0.92);
  border-color: transparent;
}

:root.dark .sidebar-nav-item-active,
[data-theme='dark'] .sidebar-nav-item-active {
  color: #f8fafc;
  background: rgba(15, 23, 42, 0.78);
  border-color: rgba(100, 116, 139, 0.65);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04);
}

:root.dark .sidebar-nav-item-inactive,
[data-theme='dark'] .sidebar-nav-item-inactive {
  color: #cbd5e1;
}

:root.dark .sidebar-nav-item-inactive:hover,
[data-theme='dark'] .sidebar-nav-item-inactive:hover {
  color: #f8fafc;
  background: rgba(255, 255, 255, 0.06);
  border-color: rgba(100, 116, 139, 0.35);
}

:root.dark .sidebar-section-label,
[data-theme='dark'] .sidebar-section-label {
  color: #94a3b8;
}

:root.dark .sidebar-section-trigger:hover,
:root.dark .sidebar-section-trigger:focus-visible,
[data-theme='dark'] .sidebar-section-trigger:hover,
[data-theme='dark'] .sidebar-section-trigger:focus-visible {
  background: rgba(255, 255, 255, 0.06);
  border-color: rgba(100, 116, 139, 0.35);
}

:root.dark .sidebar-section-trigger:hover .sidebar-section-label,
:root.dark .sidebar-section-trigger:focus-visible .sidebar-section-label,
[data-theme='dark'] .sidebar-section-trigger:hover .sidebar-section-label,
[data-theme='dark'] .sidebar-section-trigger:focus-visible .sidebar-section-label {
  color: #e2e8f0;
}

:root.dark .sidebar-footer-meta,
[data-theme='dark'] .sidebar-footer-meta {
  opacity: 0.78;
}

:root.dark .sidebar-account-link,
[data-theme='dark'] .sidebar-account-link {
  color: #cbd5e1;
}

:root.dark .sidebar-utility-button,
[data-theme='dark'] .sidebar-utility-button {
  color: #cbd5e1;
  background: rgba(15, 23, 42, 0.78);
  border-color: rgba(100, 116, 139, 0.48);
}

:root.dark .sidebar-utility-button:hover,
[data-theme='dark'] .sidebar-utility-button:hover {
  color: #f8fafc;
  background: rgba(255, 255, 255, 0.06);
  border-color: rgba(100, 116, 139, 0.42);
}

:root.dark .sidebar-utility-button-active,
[data-theme='dark'] .sidebar-utility-button-active {
  color: #e0f2fe;
  background: rgba(30, 64, 175, 0.32);
  border-color: rgba(96, 165, 250, 0.5);
}

:root.dark .sidebar-utility-row-preview .sidebar-utility-button,
[data-theme='dark'] .sidebar-utility-row-preview .sidebar-utility-button {
  color: #cbd5e1;
  background: transparent;
  border-color: transparent;
}

:root.dark .sidebar-utility-row-preview .sidebar-utility-button:hover,
[data-theme='dark'] .sidebar-utility-row-preview .sidebar-utility-button:hover {
  color: #f8fafc;
  background: rgba(255, 255, 255, 0.08);
  border-color: transparent;
}

:root.dark .sidebar-utility-row-preview .sidebar-utility-button-active,
[data-theme='dark'] .sidebar-utility-row-preview .sidebar-utility-button-active {
  color: #f8fafc;
  background: rgba(255, 255, 255, 0.1);
  border-color: transparent;
}

:root.dark .sidebar-account-link-inactive:hover,
[data-theme='dark'] .sidebar-account-link-inactive:hover {
  color: #f8fafc;
  background: rgba(255, 255, 255, 0.06);
  border-color: rgba(100, 116, 139, 0.35);
}

:root.dark .sidebar-account-link-active,
[data-theme='dark'] .sidebar-account-link-active {
  color: #f8fafc;
  background: rgba(15, 23, 42, 0.78);
  border-color: rgba(100, 116, 139, 0.65);
}

:root.dark .sidebar-brand,
[data-theme='dark'] .sidebar-brand {
  background: transparent;
}

:root.dark .sidebar-brand:hover,
[data-theme='dark'] .sidebar-brand:hover {
  background: rgba(255, 255, 255, 0.06);
}

:root.dark .sidebar-brand-mark,
[data-theme='dark'] .sidebar-brand-mark {
  border-color: rgba(100, 116, 139, 0.55);
  background: rgba(15, 23, 42, 0.62);
}

:root.dark .sidebar-brand-name,
[data-theme='dark'] .sidebar-brand-name {
  color: #f8fafc;
}

:root.dark .sidebar-status-badge,
[data-theme='dark'] .sidebar-status-badge {
  border-color: rgba(100, 116, 139, 0.34);
  background: rgba(15, 23, 42, 0.68);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04);
}

:root.dark .sidebar-status-badge-online,
[data-theme='dark'] .sidebar-status-badge-online {
  color: #86efac;
  border-color: rgba(34, 197, 94, 0.28);
  background: rgba(6, 78, 59, 0.2);
}

:root.dark .sidebar-status-badge-alert,
[data-theme='dark'] .sidebar-status-badge-alert {
  color: #fca5a5;
  border-color: rgba(248, 113, 113, 0.28);
  background: rgba(127, 29, 29, 0.22);
}

@media (min-width: 1024px) {
  .app-sidebar {
    position: sticky;
    inset: auto;
    top: 0;
    align-self: flex-start;
    flex-shrink: 0;
    min-height: var(--layout-sidebar-height, calc(100vh - 1.6rem));
    height: var(--layout-sidebar-height, calc(100vh - 1.6rem));
    max-height: var(--layout-sidebar-height, calc(100vh - 1.6rem));
  }

  .sidebar-card {
    border: 1px solid rgba(148, 163, 184, 0.22);
    border-radius: 1.75rem;
    overflow: hidden;
  }

  :root.light .sidebar-card,
  [data-theme='light'] .sidebar-card {
    border-color: rgba(186, 203, 223, 0.78);
  }
}

@media (max-width: 1023px) {
  .sidebar-status-badge {
    display: none;
  }
}

.ws-card:hover {
  transform: translateY(-1px);
}

.workspace-panel-layer {
  position: fixed;
  inset: 0;
  z-index: 70;
  display: flex;
  align-items: stretch;
  justify-content: flex-end;
  padding: 0;
  background: rgba(2, 6, 23, 0.5);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
}

.workspace-panel-shell {
  width: min(28rem, calc(100vw - 0.75rem));
  max-width: 100%;
  height: 100%;
  margin-inline-start: auto;
  border-start-start-radius: 1.5rem;
  border-end-start-radius: 1.5rem;
  border-start-end-radius: 0;
  border-end-end-radius: 0;
}

.workspace-panel-enter-active,
.workspace-panel-leave-active {
  transition: opacity 0.22s ease;
}

.workspace-panel-enter-active .workspace-panel-shell,
.workspace-panel-leave-active .workspace-panel-shell {
  transition:
    transform 0.22s ease,
    opacity 0.22s ease;
}

.workspace-panel-enter-from,
.workspace-panel-leave-to {
  opacity: 0;
}

.workspace-panel-enter-from .workspace-panel-shell,
.workspace-panel-leave-to .workspace-panel-shell {
  opacity: 0;
  transform: translateX(24px);
}

html[dir='rtl'] .workspace-panel-enter-from .workspace-panel-shell,
html[dir='rtl'] .workspace-panel-leave-to .workspace-panel-shell {
  transform: translateX(-24px);
}

@media (min-width: 1024px) {
  .workspace-panel-layer {
    inset-block: 0.9rem;
    inset-inline-end: 0.9rem;
    inset-inline-start: auto;
    width: var(--workspace-dock-width, 28rem);
    padding: 0;
    align-items: stretch;
    justify-content: flex-end;
    background: transparent;
    backdrop-filter: none;
    -webkit-backdrop-filter: none;
  }

  .workspace-panel-shell {
    width: 100%;
    height: 100%;
    border-radius: 2rem;
    box-shadow: 0 30px 52px -30px rgba(15, 23, 42, 0.55);
  }
}

.ws-grid-enter-active,
.ws-grid-leave-active {
  transition: opacity 0.15s ease;
}
.ws-grid-enter-from,
.ws-grid-leave-to {
  opacity: 0;
}

.ws-expand-enter-active {
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
}
.ws-expand-leave-active {
  transition: all 0.15s cubic-bezier(0.4, 0, 0.2, 1);
}
.ws-expand-enter-from {
  opacity: 0;
  transform: scale(0.96) translateY(-4px);
}
.ws-expand-leave-to {
  opacity: 0;
  transform: scale(0.96);
}
</style>
