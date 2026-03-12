<script setup lang="ts">
import { computed, ref, watch, onMounted, onUnmounted, nextTick } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useSystemStore } from '@/stores/system'
import { useAuthStore } from '@/stores/auth'
import { usePreviewStore } from '@/stores/preview'
import { workspaceApi } from '@/api/workspace'
import type { WorkspaceFile, WorkspaceStats, WorkspaceTreeEntry } from '@/api/workspace'
import { claudeCodeApi, type DirectoryWhitelistEntry } from '@/api/claudecode'
import { conversationApi, messageApi, type Conversation, type Message } from '@/api/chat'
import type { TypelessCard, TypelessCardConvertTask, TypelessCardFile, TypelessCardResult } from '@/types/typeless'
import { parseTypelessContent } from '@/utils/typeless'
import { PagePermissions } from '@/api/users'
import { storeToRefs } from 'pinia'
import { useTauri } from '@/composables/useTauri'
import { isLocalAbsolutePath } from '@/utils/localPath'

interface NavItem {
  name: string
  path: string
  icon: string
  permission: string
  adminOnly?: boolean
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
  generatedRecord: GeneratedWorkspaceFile | null
}

interface WhitelistTreeTarget {
  path: string
  label: string
}

const GENERATED_SCAN_CONVERSATION_LIMIT = 20
const GENERATED_SCAN_MESSAGE_LIMIT = 120

const pathLikeKeys = ['path', 'file', 'file_path', 'output_path', 'download_url', 'local_path', 'artifact_path', 'url']

const coreWorkspaceFileInfo: Record<string, CoreWorkspaceFileInfo> = {
  'SOUL.md': { icon: '🧠', labelKey: 'workspace.label.soul', descKey: 'workspace.desc.soul' },
  'USER.md': { icon: '👤', labelKey: 'workspace.label.user', descKey: 'workspace.desc.user' },
  'IDENTITY.md': { icon: '🏷️', labelKey: 'workspace.label.identity', descKey: 'workspace.desc.identity' },
  'MEMORY.md': { icon: '💾', labelKey: 'workspace.label.memory', descKey: 'workspace.desc.memory' },
  'AGENTS.md': { icon: '📋', labelKey: 'workspace.label.agents', descKey: 'workspace.desc.agents' },
  'TOOLS.md': { icon: '🧰', labelKey: 'workspace.label.tools', descKey: 'workspace.desc.tools' },
  'HEARTBEAT.md': { icon: '💗', labelKey: 'workspace.label.heartbeat', descKey: 'workspace.desc.heartbeat' },
}
const coreWorkspaceFileNames = new Set(Object.keys(coreWorkspaceFileInfo))

const { t, te } = useI18n()
const route = useRoute()
const router = useRouter()
const systemStore = useSystemStore()
const authStore = useAuthStore()
const previewStore = usePreviewStore()
const { isTauri, platform, openInBrowser, revealInFileManager } = useTauri()
const { health } = storeToRefs(systemStore)
const { isAdmin } = storeToRefs(authStore)
const { isPreviewMode } = storeToRefs(previewStore)

// Mobile menu state
const isOpen = ref(false)

// Collapsed state (desktop only)
const SIDEBAR_COLLAPSED_KEY = 'sidebar-collapsed'
const isCollapsed = ref(false)
const isMac = typeof navigator !== 'undefined' && navigator.platform.toUpperCase().indexOf('MAC') >= 0

// Workspace panel state
const showWorkspacePanel = ref(false)
const activeWorkspaceTab = ref<'core' | 'generated'>('core')
const workspaceDir = ref('')
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

function tr(key: string, fallback: string): string {
  if (te(key)) return t(key)
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

function sanitizeWhitelistLabel(value: string): string {
  const trimmed = value.trim()
  if (!trimmed) return 'whitelist'
  const safe = trimmed.replace(/[\\/]+/g, '-').replace(/\s+/g, '_')
  return safe || 'whitelist'
}

function buildWhitelistTreeTargets(
  whitelist: DirectoryWhitelistEntry[] | undefined,
  workspaceRootPath: string,
): WhitelistTreeTarget[] {
  if (!Array.isArray(whitelist) || whitelist.length === 0) return []

  const workspaceKey = toPathKey(workspaceRootPath)
  const seenPaths = new Set<string>()
  const usedLabels = new Set<string>()
  const targets: WhitelistTreeTarget[] = []

  for (const entry of whitelist) {
    const path = String(entry?.path || '').trim()
    if (!path) continue
    const pathKey = toPathKey(path)
    if (!pathKey || seenPaths.has(pathKey) || (workspaceKey && pathKey === workspaceKey)) continue
    seenPaths.add(pathKey)

    const rawLabel = entry.alias?.trim() || path.split(/[/\\]/).filter(Boolean).pop() || path
    const baseLabel = sanitizeWhitelistLabel(rawLabel)
    let label = baseLabel
    let suffix = 2
    while (usedLabels.has(label)) {
      label = `${baseLabel}_${suffix}`
      suffix++
    }
    usedLabels.add(label)
    targets.push({ path, label })
  }

  return targets
}

function mapWhitelistTreeEntries(
  label: string,
  rootPath: string,
  entries: WorkspaceTreeEntry[],
): WorkspaceTreeEntry[] {
  const prefix = `@${label}`
  const mapped: WorkspaceTreeEntry[] = [{
    path: prefix,
    abs_path: rootPath,
    name: prefix,
    type: 'dir',
    depth: 1,
  }]

  for (const entry of entries) {
    const relPath = String(entry.path || '').trim()
    if (!relPath) continue
    mapped.push({
      ...entry,
      path: `${prefix}/${relPath}`,
      depth: Math.max(1, Number(entry.depth) || 1) + 1,
    })
  }
  return mapped
}

async function loadWhitelistWorkspaceTreeEntries(workspaceRootPath: string): Promise<WorkspaceTreeEntry[]> {
  try {
    const cfgRes = await claudeCodeApi.getConfig()
    const config = cfgRes.data
    if (!config?.whitelist_enabled || !Array.isArray(config.directory_whitelist) || config.directory_whitelist.length === 0) {
      return []
    }

    const targets = buildWhitelistTreeTargets(config.directory_whitelist, workspaceRootPath)
    if (targets.length === 0) return []

    const treeChunks = await Promise.all(targets.map(async (target) => {
      try {
        const treeRes = await workspaceApi.getTree({ max_depth: 16, root: target.path })
        const root = String(treeRes.data?.root || '').trim() || target.path
        const entries = Array.isArray(treeRes.data?.entries) ? treeRes.data.entries : []
        return mapWhitelistTreeEntries(target.label, root, entries)
      } catch (e) {
        console.warn('Failed to load whitelist tree root:', target.path, e)
        return [] as WorkspaceTreeEntry[]
      }
    }))

    return treeChunks.flat()
  } catch (e) {
    console.warn('Failed to load directory whitelist config:', e)
    return []
  }
}

function collectPathCandidates(value: unknown, into: string[]): void {
  if (typeof value === 'string') {
    const path = value.trim()
    if (isLocalAbsolutePath(path)) into.push(path)
    return
  }

  if (Array.isArray(value)) {
    for (const item of value) collectPathCandidates(item, into)
    return
  }

  if (!value || typeof value !== 'object') return

  const record = value as Record<string, unknown>
  for (const key of pathLikeKeys) {
    const candidate = record[key]
    if (typeof candidate === 'string' && isLocalAbsolutePath(candidate.trim())) {
      into.push(candidate.trim())
    }
  }
}

function extractLocalPathCandidatesFromCard(card: TypelessCard): string[] {
  const paths: string[] = []

  if (card.type === 'file') {
    const fileCard = card as TypelessCardFile
    collectPathCandidates(fileCard.downloadUrl, paths)
    collectPathCandidates(fileCard.previewUrl, paths)
  }

  if (card.type === 'result') {
    const resultCard = card as TypelessCardResult
    for (const detail of resultCard.details || []) {
      collectPathCandidates(detail.value, paths)
    }
  }

  if (card.type === 'convert-task') {
    const convertCard = card as TypelessCardConvertTask
    for (const output of convertCard.outputs || []) {
      collectPathCandidates(output.download_url, paths)
    }
  }

  const genericCard = card as unknown as Record<string, unknown>
  collectPathCandidates(genericCard.artifacts, paths)
  collectPathCandidates(genericCard.download_url, paths)

  return Array.from(new Set(paths))
}

function buildGeneratedFileRecord(
  path: string,
  source: string,
  conversation: Conversation,
  message: Message,
): GeneratedWorkspaceFile {
  const normalized = path.trim()
  const clean = normalized.replace(/[\\/]+$/, '') || normalized
  const filename = clean.split(/[/\\]/).pop() || clean
  const safePath = clean.replace(/[^a-zA-Z0-9_.-]+/g, '_').slice(-120)
  const title = String(conversation.title || '').trim() || tr('nav.workspaceUnknownConversation', 'Conversation')
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

function extractGeneratedFilesFromMessage(conversation: Conversation, message: Message): GeneratedWorkspaceFile[] {
  if (!message.content || !message.content.trim()) return []

  const parsed = parseTypelessContent(message.content, message.id, conversation.id)
  if (!parsed.cards.length) return []

  const records: GeneratedWorkspaceFile[] = []
  const seenPaths = new Set<string>()

  for (const card of parsed.cards) {
    const source = card.type || 'card'
    const candidates = extractLocalPathCandidatesFromCard(card)
    for (const candidate of candidates) {
      if (seenPaths.has(candidate)) continue
      seenPaths.add(candidate)
      records.push(buildGeneratedFileRecord(candidate, source, conversation, message))
    }
  }

  return records
}

async function ensureWorkspaceMeta() {
  if (workspaceMetaLoaded.value) return

  try {
    const res = await workspaceApi.getMeta()
    workspaceDir.value = String(res.data?.dir || '').trim()
    workspaceMetaLoaded.value = true
  } catch (e) {
    console.error('Failed to load workspace metadata:', e)
    workspaceError.value = tr('nav.workspaceLoadFailed', 'Failed to load workspace details')
  }
}

async function ensureWorkspaceFiles(force = false) {
  if (workspaceFilesLoaded.value && !force) return

  workspaceLoading.value = true
  workspaceError.value = ''
  try {
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
    workspaceTreeError.value = tr('nav.workspaceTreeLoadFailed', 'Failed to load workspace directory tree')
  } finally {
    workspaceTreeLoading.value = false
  }
}

async function ensureGeneratedWorkspaceFiles(force = false) {
  if (generatedFilesLoaded.value && !force) return

  generatedFilesLoading.value = true
  generatedFilesError.value = ''
  try {
    const convRes = await conversationApi.list(GENERATED_SCAN_CONVERSATION_LIMIT, 0)
    const conversations = Array.isArray(convRes.data) ? convRes.data : []

    const allRecords: GeneratedWorkspaceFile[] = []

    for (const conversation of conversations) {
      try {
        const msgRes = await messageApi.list(conversation.id, GENERATED_SCAN_MESSAGE_LIMIT, 0)
        const messages = Array.isArray(msgRes.data) ? msgRes.data : []
        for (const message of messages) {
          allRecords.push(...extractGeneratedFilesFromMessage(conversation, message))
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
      (a, b) => messageTimeMs(b.messageCreatedAt) - messageTimeMs(a.messageCreatedAt),
    )
    generatedFilesLoaded.value = true
  } catch (e) {
    console.error('Failed to load generated workspace files:', e)
    generatedFilesError.value = tr('nav.workspaceGeneratedLoadFailed', 'Failed to load generated files')
  } finally {
    generatedFilesLoading.value = false
  }
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
    await workspaceApi.putFile(name, coreEditDraft.value)
    const target = workspaceFiles.value.find(file => file.name === name)
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
  cancelCoreEdit()
  showWorkspacePanel.value = false
  activeWorkspaceTab.value = 'core'
}

async function switchWorkspaceTab(tab: 'core' | 'generated') {
  activeWorkspaceTab.value = tab
  if (tab === 'core') {
    await ensureWorkspaceFiles()
    return
  }
  await Promise.all([ensureWorkspaceTree(), ensureGeneratedWorkspaceFiles()])
}

async function handleWorkspaceClick() {
  workspaceError.value = ''
  await ensureWorkspaceMeta()

  if (canRevealWorkspaceDir.value) {
    const revealed = await revealInFileManager(workspaceDir.value)
    if (revealed) return
    workspaceError.value = tr('nav.workspaceOpenFailed', 'Unable to open workspace in file manager')
  }

  showWorkspacePanel.value = true
  await ensureWorkspaceFiles()
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
  window.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeydown)
})

// Toggle collapsed state
function toggleCollapse() {
  isCollapsed.value = !isCollapsed.value
  setStorageItem(SIDEBAR_COLLAPSED_KEY, String(isCollapsed.value))
}

// Close menu when route changes
watch(() => route.path, () => {
  isOpen.value = false
  closeWorkspacePanel()
})

// Expose toggle function for parent components
defineExpose({
  toggle: () => { isOpen.value = !isOpen.value },
  open: () => { isOpen.value = true },
  close: () => { isOpen.value = false },
  isOpen,
  isCollapsed,
  toggleCollapse,
})

// Check if user has permission for a page
const hasPermission = (permission?: string) => {
  if (!permission) return true
  return authStore.hasPermission(permission)
}

// Define all nav items with their permissions
const allNavItems: NavItem[] = [
  {
    name: 'nav.dashboard',
    path: '/home',
    icon: 'M4 5a1 1 0 011-1h14a1 1 0 011 1v2a1 1 0 01-1 1H5a1 1 0 01-1-1V5zM4 13a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H5a1 1 0 01-1-1v-6zM16 13a1 1 0 011-1h2a1 1 0 011 1v6a1 1 0 01-1 1h-2a1 1 0 01-1-1v-6z',
    permission: PagePermissions.HOME,
    adminOnly: true,
  },
  {
    name: 'nav.chat',
    path: '/chat',
    icon: 'M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z',
    permission: PagePermissions.CHAT,
  },
  {
    name: 'nav.channels',
    path: '/channels',
    icon: 'M17 8h2a2 2 0 012 2v6a2 2 0 01-2 2h-2v4l-4-4H9a1.994 1.994 0 01-1.414-.586m0 0L11 14h4a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2v4l.586-.586z',
    permission: PagePermissions.CHANNELS,
    adminOnly: true,
  },
  {
    name: 'nav.automation',
    path: '/cron',
    icon: 'M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15M9 12l2 2 4-4',
    permission: PagePermissions.AUTOMATION,
    adminOnly: true,
  },
  {
    name: 'nav.plugins',
    path: '/plugins',
    icon: 'M13 10V3L4 14h7v7l9-11h-7z',
    permission: PagePermissions.PLUGINS,
    adminOnly: true,
  },
  {
    name: 'nav.security',
    path: '/security',
    icon: 'M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z',
    permission: PagePermissions.SECURITY,
    adminOnly: true,
  },
  {
    name: 'nav.settings',
    path: '/settings',
    icon: 'M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z',
    permission: PagePermissions.SETTINGS,
    adminOnly: true,
  },
]

// Check if a nav item is active (handles trailing slashes and sub-paths)
const isActive = (itemPath: string) => {
  const currentPath = route.path.replace(/\/+$/, '') || '/'
  const navPath = itemPath.replace(/\/+$/, '') || '/'
  return currentPath === navPath || currentPath.startsWith(navPath + '/')
}

// Filter nav items based on permissions
const navItems = computed(() => {
  return allNavItems
    .filter((item) => {
      // In preview mode, show all items (preview user has full access)
      if (isPreviewMode.value) return true
      if (item.adminOnly && !isAdmin.value) {
        return false
      }
      return hasPermission(item.permission)
    })
    .map((item) => ({
      ...item,
      name: translateNavLabel(item.name),
    }))
})

const sortedWorkspaceFiles = computed(() => {
  return [...workspaceFiles.value].sort((a, b) => a.name.localeCompare(b.name))
})

const coreWorkspaceFiles = computed(() => {
  return sortedWorkspaceFiles.value.filter((file) => coreWorkspaceFileNames.has(file.name))
})

const generatedRecordByPathKey = computed(() => {
  const map = new Map<string, GeneratedWorkspaceFile>()
  for (const record of generatedWorkspaceFiles.value) {
    const key = toPathKey(record.path)
    if (!key || map.has(key)) continue
    map.set(key, record)
  }
  return map
})

const workspaceTreeRows = computed<WorkspaceTreeRow[]>(() => {
  return workspaceTreeEntries.value.map((entry) => {
    const pathKey = toPathKey(String(entry.abs_path || ''))
    const generatedRecord = isWorkspaceTreeDir(entry)
      ? null
      : (generatedRecordByPathKey.value.get(pathKey) || null)
    return { entry, generatedRecord }
  })
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

const workspaceTreeVisibleRows = computed<WorkspaceTreeRow[]>(() => {
  const collapsed = workspaceTreeCollapsedDirs.value
  const linkedKeys = workspaceTreeShowLinkedOnly.value ? workspaceTreeLinkedPathKeys.value : null
  return workspaceTreeRows.value.filter((row) => {
    const rowKey = toPathKey(row.entry.path)
    if (!rowKey) return false
    if (linkedKeys && !linkedKeys.has(rowKey)) return false
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
    return tr('nav.workspaceTreeFilteredEmpty', 'No conversation-linked files found in the current tree view.')
  }
  return tr('nav.workspaceTreeEmpty', 'No files found in workspace tree.')
})

const workspaceTreeFilterLabel = computed(() => {
  if (workspaceTreeShowLinkedOnly.value) {
    return tr('nav.workspaceTreeShowAll', 'Show all files')
  }
  return tr('nav.workspaceTreeShowLinkedOnly', 'Only linked files')
})

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
  return dir || tr('nav.workspacePathUnavailable', 'Workspace path unavailable')
})
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
    class="glass-sidebar min-h-full flex flex-col fixed lg:relative inset-y-0 left-0 z-50 transform transition-all duration-300 ease-in-out lg:transform-none"
    :class="[
      isOpen ? 'translate-x-0' : '-translate-x-full lg:translate-x-0',
      isCollapsed ? 'w-16' : 'w-52',
    ]"
  >
    <!-- Close button for mobile -->
    <div class="lg:hidden p-3 border-b border-gray-200 dark:border-glass-border flex justify-end">
      <button
        class="p-2 rounded-lg text-gray-500 hover:bg-gray-100 dark:hover:bg-white/10"
        @click="isOpen = false"
      >
        <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
        </svg>
      </button>
    </div>

    <nav class="p-3 space-y-1 flex-1 overflow-y-auto">
      <RouterLink
        v-for="item in navItems"
        :key="item.path"
        :to="item.path"
        class="flex items-center px-4 py-2.5 rounded-lg transition-all duration-200 cursor-pointer group"
        :class="[
          isActive(item.path)
            ? 'bg-gray-100 dark:bg-gray-700/20 text-gray-900 dark:text-gray-300 border border-gray-900/30 dark:border-gray-700/30'
            : 'text-gray-600 dark:text-slate-300 hover:bg-gray-100 dark:hover:bg-white/5 hover:text-gray-900 dark:hover:text-white border border-transparent',
          isCollapsed ? 'justify-center' : 'space-x-3',
        ]"
        :title="isCollapsed ? item.name : undefined"
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="h-5 w-5 transition-transform duration-200 group-hover:scale-110 flex-shrink-0"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="item.icon" />
        </svg>
        <span v-if="!isCollapsed" class="font-medium whitespace-nowrap">{{ item.name }}</span>
      </RouterLink>
    </nav>

    <div class="px-3 pb-3">
      <button
        class="w-full flex items-center rounded-lg border transition-colors"
        :class="[
          isCollapsed ? 'justify-center px-2 py-2.5' : 'px-3 py-2.5 gap-2.5',
          'border-gray-200 dark:border-gray-700 text-gray-700 dark:text-slate-200 hover:bg-gray-100 dark:hover:bg-white/5',
        ]"
        :title="isCollapsed ? tr('nav.workspace', 'Workspace') : undefined"
        @click="handleWorkspaceClick"
      >
        <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 7a2 2 0 012-2h4l2 2h8a2 2 0 012 2v8a2 2 0 01-2 2H5a2 2 0 01-2-2V7z" />
        </svg>
        <div v-if="!isCollapsed" class="min-w-0 text-left">
          <div class="text-sm font-medium truncate">{{ tr('nav.workspace', 'Workspace') }}</div>
          <div class="text-xs text-gray-500 dark:text-slate-400 truncate">{{ workspacePathLabel }}</div>
        </div>
      </button>
    </div>

    <!-- Bottom section: Version + Collapse toggle -->
    <div class="p-3 border-t border-gray-200 dark:border-glass-border">
      <div class="flex items-center" :class="isCollapsed ? 'justify-center' : 'justify-between'">
        <!-- Version info -->
        <div v-if="health" class="text-xs text-gray-500 dark:text-slate-400" :class="{ hidden: isCollapsed }">
          v{{ health.version }}
        </div>
        <!-- Collapse toggle button (desktop only) -->
        <button
          class="hidden lg:flex p-1.5 rounded-lg text-gray-400 hover:text-gray-600 dark:hover:text-slate-200 hover:bg-gray-100 dark:hover:bg-white/10 transition-colors"
          :title="(isCollapsed ? t('nav.expandSidebar') : t('nav.collapseSidebar')) + (isMac ? ' (⌘B)' : ' (Alt+B)')"
          @click="toggleCollapse"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 transition-transform duration-300" :class="isCollapsed ? 'rotate-180' : ''" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 19l-7-7 7-7m8 14l-7-7 7-7" />
          </svg>
        </button>
      </div>
    </div>
  </aside>

  <div
    v-if="showWorkspacePanel"
    class="fixed inset-0 z-[70] bg-black/50 p-4 sm:p-6 flex items-center justify-center"
    @click.self="closeWorkspacePanel"
  >
    <div class="w-full max-w-5xl max-h-[88vh] rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 shadow-2xl overflow-hidden flex flex-col">
      <div class="px-4 py-3 border-b border-gray-200 dark:border-gray-700 flex items-start justify-between gap-3">
        <div class="min-w-0">
          <h3 class="text-base font-semibold text-gray-900 dark:text-white">
            {{ tr('nav.workspacePanelTitle', 'Workspace') }}
          </h3>
          <p class="text-xs text-gray-500 dark:text-slate-400 truncate mt-0.5">{{ workspacePathLabel }}</p>
          <p v-if="workspaceError" class="text-xs text-red-600 dark:text-red-400 mt-1">
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
            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
      </div>

      <div class="px-4 py-2 border-b border-gray-100 dark:border-gray-700/70">
        <div class="inline-flex rounded-lg border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/70 p-1">
          <button
            class="px-3 py-1.5 text-xs rounded-md transition-colors"
            :class="activeWorkspaceTab === 'core'
              ? 'bg-white dark:bg-gray-700 text-gray-900 dark:text-white shadow-sm'
              : 'text-gray-600 dark:text-slate-300 hover:text-gray-900 dark:hover:text-white'"
            @click="switchWorkspaceTab('core')"
          >
            {{ tr('nav.workspaceCoreTab', 'Workspace Files') }}
          </button>
          <button
            class="px-3 py-1.5 text-xs rounded-md transition-colors"
            :class="activeWorkspaceTab === 'generated'
              ? 'bg-white dark:bg-gray-700 text-gray-900 dark:text-white shadow-sm'
              : 'text-gray-600 dark:text-slate-300 hover:text-gray-900 dark:hover:text-white'"
            @click="switchWorkspaceTab('generated')"
          >
            {{ tr('nav.workspaceGeneratedTab', 'Generated Files') }}
          </button>
        </div>
      </div>

      <div class="flex-1 min-h-0 overflow-y-auto p-4">
        <div v-if="activeWorkspaceTab === 'core'" class="space-y-3">
          <div class="flex items-center justify-between">
            <div>
              <h4 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('workspace.title') }}</h4>
              <p class="text-sm text-gray-500 dark:text-gray-400 mt-0.5">{{ t('workspace.description') }}</p>
            </div>
            <span
              v-if="workspaceStats"
              class="text-xs px-2 py-1 rounded-full flex-shrink-0"
              :class="workspaceStats.total_tokens > 4096
                ? 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400'
                : 'bg-gray-100 text-gray-500 dark:bg-gray-700 dark:text-gray-400'"
            >
              {{ t('workspace.tokens', { count: workspaceStats.total_tokens.toLocaleString() }) }}
            </span>
          </div>

          <div v-if="workspaceLoading && coreWorkspaceFiles.length === 0" class="text-sm text-gray-500 dark:text-gray-400 py-4 text-center">
            {{ tr('nav.workspaceLoading', 'Loading workspace files...') }}
          </div>

          <div v-else-if="coreWorkspaceFiles.length === 0" class="text-sm text-gray-500 dark:text-gray-400 py-4 text-center">
            {{ t('workspace.noFiles') }}
          </div>

          <div v-else class="relative">
            <Transition name="ws-grid">
              <div v-if="!coreEditingFile" class="grid grid-cols-2 sm:grid-cols-3 gap-2">
                <button
                  v-for="file in coreWorkspaceFiles"
                  :key="file.name"
                  class="ws-card group flex flex-col items-center gap-1 p-3 rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800/50 hover:border-gray-300 dark:hover:border-gray-600 hover:shadow-sm transition-all duration-150 cursor-pointer text-center"
                  @click="startCoreEdit(file)"
                >
                  <span class="text-2xl leading-none">{{ getCoreFileInfo(file.name).icon }}</span>
                  <span class="text-xs font-medium text-gray-900 dark:text-white truncate w-full">{{ getCoreFileLabel(file.name) }}</span>
                  <span class="text-[10px] text-gray-400 dark:text-gray-500 leading-tight">{{ getCoreFileDesc(file.name) }}</span>
                </button>
              </div>
            </Transition>

            <Transition name="ws-expand">
              <div
                v-if="coreEditingFile"
                class="rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800/80 overflow-hidden"
              >
                <div class="flex items-center justify-between px-4 py-2 border-b border-gray-100 dark:border-gray-700">
                  <div class="flex items-center gap-2 min-w-0">
                    <span class="text-lg flex-shrink-0">{{ getCoreFileInfo(coreEditingFile).icon }}</span>
                    <span class="text-sm font-medium text-gray-900 dark:text-white truncate">{{ getCoreFileLabel(coreEditingFile) }}</span>
                    <span class="text-xs text-gray-400 dark:text-gray-500 hidden sm:inline flex-shrink-0">
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
                <div class="px-4 py-1.5 text-[10px] text-gray-400 dark:text-gray-500 border-t border-gray-100 dark:border-gray-700 flex justify-between">
                  <span>Esc {{ t('common.cancel') }} · ⌘↵ {{ t('common.save') }}</span>
                  <span>{{ t('workspace.chars', { count: coreEditDraft.length.toLocaleString() }) }}</span>
                </div>
              </div>
            </Transition>
          </div>
        </div>

        <div v-else class="space-y-3">
          <div class="flex items-start justify-between gap-3">
            <h4 class="text-base font-semibold text-gray-900 dark:text-white">{{ tr('nav.workspaceGeneratedTitle', 'Workspace Tree') }}</h4>
            <div class="flex items-center gap-1.5 text-[11px] text-gray-500 dark:text-gray-400 flex-wrap justify-end">
              <span class="px-2 py-0.5 rounded bg-gray-100 dark:bg-gray-700">{{ tr('nav.workspaceTreeDirCount', 'Dirs') }} {{ workspaceTreeDirectoryCount }}</span>
              <span class="px-2 py-0.5 rounded bg-gray-100 dark:bg-gray-700">{{ tr('nav.workspaceTreeFileCount', 'Files') }} {{ workspaceTreeFileCount }}</span>
              <span class="px-2 py-0.5 rounded bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300">
                {{ tr('nav.workspaceTreeLinkedCount', 'Linked') }} {{ workspaceTreeLinkedCount }}
              </span>
            </div>
          </div>
          <p class="text-sm text-gray-500 dark:text-gray-400 -mt-1">
            {{ tr('nav.workspaceGeneratedDescription', 'Real workspace directory tree with source conversation links for generated files.') }}
          </p>
          <div>
            <button
              class="px-2.5 py-1 text-xs rounded-lg border border-gray-200 dark:border-gray-700 text-gray-700 dark:text-slate-200 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
              @click="toggleWorkspaceTreeLinkedFilter"
            >
              {{ workspaceTreeFilterLabel }}
            </button>
          </div>

          <p v-if="workspaceTreeError" class="text-xs text-red-600 dark:text-red-400">
            {{ workspaceTreeError }}
          </p>
          <p v-if="generatedFilesError" class="text-xs text-red-600 dark:text-red-400">
            {{ generatedFilesError }}
          </p>

          <div v-if="workspaceGeneratedLoading" class="text-sm text-gray-500 dark:text-gray-400 py-4 text-center">
            {{ tr('nav.workspaceTreeLoading', 'Loading workspace tree and conversation links...') }}
          </div>

          <div v-else-if="workspaceTreeVisibleRows.length === 0" class="text-sm text-gray-500 dark:text-gray-400 py-4 text-center">
            {{ workspaceTreeEmptyLabel }}
          </div>

          <div v-else class="rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800/60 overflow-hidden">
            <div class="px-3 py-2 text-xs text-gray-600 dark:text-slate-300 border-b border-gray-100 dark:border-gray-700 truncate">
              {{ workspaceTreeRootLabel }}
            </div>
            <div class="max-h-[52vh] overflow-y-auto">
              <div
                v-for="row in workspaceTreeVisibleRows"
                :key="row.entry.abs_path || row.entry.path"
                class="px-3 py-2 border-b last:border-b-0 border-gray-100 dark:border-gray-700/70"
              >
                <div class="flex items-start justify-between gap-3">
                  <div class="min-w-0 flex-1">
                    <div class="flex items-center gap-2 min-w-0" :style="treeIndentStyle(row.entry.depth)">
                      <span class="text-sm leading-none">{{ isWorkspaceTreeDir(row.entry) ? '📁' : '📄' }}</span>
                      <p class="text-sm font-medium text-gray-900 dark:text-white truncate">{{ row.entry.name }}</p>
                      <button
                        v-if="isWorkspaceTreeDir(row.entry)"
                        class="h-4 w-4 rounded text-gray-500 dark:text-slate-300 hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center justify-center transition-colors flex-shrink-0"
                        :title="isWorkspaceTreeDirCollapsed(row.entry.path)
                          ? tr('nav.workspaceTreeExpandDir', 'Expand folder')
                          : tr('nav.workspaceTreeCollapseDir', 'Collapse folder')"
                        @click.stop="toggleWorkspaceTreeDir(row.entry)"
                      >
                        <svg
                          xmlns="http://www.w3.org/2000/svg"
                          class="h-3 w-3 transition-transform duration-150"
                          :class="isWorkspaceTreeDirCollapsed(row.entry.path) ? '' : 'rotate-90'"
                          viewBox="0 0 20 20"
                          fill="currentColor"
                        >
                          <path fill-rule="evenodd" d="M7.21 14.77a.75.75 0 01.02-1.06L10.94 10 7.23 6.29a.75.75 0 111.06-1.06l4.24 4.24a.75.75 0 010 1.06l-4.24 4.24a.75.75 0 01-1.08 0z" clip-rule="evenodd" />
                        </svg>
                      </button>
                      <span
                        v-if="!isWorkspaceTreeDir(row.entry) && typeof row.entry.size_bytes === 'number'"
                        class="text-[10px] px-1.5 py-0.5 rounded bg-gray-100 dark:bg-gray-700 text-gray-500 dark:text-gray-300 flex-shrink-0"
                      >
                        {{ formatBytes(row.entry.size_bytes) }}
                      </span>
                    </div>
                    <p class="text-xs text-gray-500 dark:text-slate-400 truncate mt-0.5" :style="treeIndentStyle(row.entry.depth)">
                      {{ row.entry.path }}
                    </p>
                    <p
                      v-if="row.generatedRecord"
                      class="text-[11px] text-gray-500 dark:text-slate-400 mt-1 truncate"
                      :style="treeIndentStyle(row.entry.depth)"
                    >
                      {{ row.generatedRecord.conversationTitle }} · {{ formatTimestamp(row.generatedRecord.messageCreatedAt) }}
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
</template>

<style scoped>
.ws-card:hover {
  transform: translateY(-1px);
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
