<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { TypelessCardResult } from '@/types/typeless'
import { formatToolWarningCodeLabel } from '@/utils/toolWarnings'
import { translateCardActionLabel } from '@/utils/cardActionLabels'
import { useTauri } from '@/composables/useTauri'
import { isApiPath, isHttpUrl, isLocalAbsolutePath } from '@/utils/localPath'
import { getLocalizedToolName } from '@/utils/toolLocalization'
import { translateHistoricalEnglishBrowserResult } from '@/i18n/browser-result-compat'

const { t, te } = useI18n()
const { openInBrowser } = useTauri()

const props = defineProps<{
  card: TypelessCardResult
  actionLoading?: boolean
  activeActionId?: string
}>()

const emit = defineEmits<{
  action: [actionId: string, cardId?: string]
}>()

const copiedIndex = ref<number | null>(null)
const titleCopied = ref(false)

interface ResolvedImageItem {
  alt: string
  src: string
}

interface DirectoryListingEntry {
  path: string
  type: string
  mode?: string
  size?: number
  modifiedAt?: string
  display?: string
}

interface DirectoryListingData {
  kind: 'ls' | 'find'
  basePath: string
  count: number | null
  maxDepth: number | null
  maxEntries: number | null
  includeHidden: boolean | null
  truncated: boolean
  hiddenEntries: number | null
  pattern?: string
  typeFilter?: string
  entries: DirectoryListingEntry[]
}

interface TextSearchMatch {
  path: string
  line?: number
  column?: number
  preview?: string
}

interface TextSearchData {
  basePath: string
  pattern: string
  count: number | null
  maxResults: number | null
  caseSensitive: boolean | null
  includeHidden: boolean | null
  truncated: boolean
  backend?: string
  backendSource?: string
  fallbackReason?: string
  matches: TextSearchMatch[]
}

const IMAGE_DETAIL_LABELS = new Set([
  'image',
  'images',
  'media_url',
  'preview_image',
  'preview_url',
  'screenshot',
  'screenshots',
  'thumbnail',
  'thumbnail_url',
])
const IMAGE_URL_SUFFIX_RE = /\.(?:png|jpe?g|gif|webp|bmp|svg)(?:[?#].*)?$/i
const WINDOWS_ABS_PATH_RE = /^(?:[a-zA-Z]:[\\/]|\\\\)/
const POSIX_LOCAL_ROOT_SEGMENTS = new Set([
  'users',
  'home',
  'tmp',
  'var',
  'private',
  'mnt',
  'media',
  'volumes',
  'root',
  'opt',
  'srv',
  'data',
  'run',
])

// Title is explicitly marked as copyable by the backend (e.g. exec command)
const isTitleCopyable = computed(() => {
  if (!('title_copyable' in props.card)) return false
  return props.card.title_copyable === true
})

const statusConfig = {
  success: {
    headerBg: 'bg-emerald-50 dark:bg-emerald-900/20',
    headerBorder: 'border-emerald-100 dark:border-emerald-800/50',
    border: 'border-emerald-200 dark:border-emerald-800/60',
    icon: '✓',
    iconBg: 'bg-emerald-100 dark:bg-emerald-900/40',
    iconColor: 'text-emerald-600 dark:text-emerald-400',
  },
  error: {
    headerBg: 'bg-red-50 dark:bg-red-900/20',
    headerBorder: 'border-red-100 dark:border-red-800/50',
    border: 'border-red-200 dark:border-red-800/60',
    icon: '✗',
    iconBg: 'bg-red-100 dark:bg-red-900/40',
    iconColor: 'text-red-600 dark:text-red-400',
  },
  warning: {
    headerBg: 'bg-amber-50 dark:bg-amber-900/20',
    headerBorder: 'border-amber-100 dark:border-amber-800/50',
    border: 'border-amber-200 dark:border-amber-800/60',
    icon: '⚠',
    iconBg: 'bg-amber-100 dark:bg-amber-900/40',
    iconColor: 'text-amber-600 dark:text-amber-400',
  },
  info: {
    headerBg: 'bg-blue-50 dark:bg-blue-900/20',
    headerBorder: 'border-blue-100 dark:border-blue-800/50',
    border: 'border-blue-200 dark:border-blue-800/60',
    icon: 'ℹ',
    iconBg: 'bg-blue-100 dark:bg-blue-900/40',
    iconColor: 'text-blue-600 dark:text-blue-400',
  },
} as const

type StatusKey = keyof typeof statusConfig

const cardStatus = computed<StatusKey>(() => {
  const s = props.card.status
  return s && s in statusConfig ? (s as StatusKey) : 'info'
})

const buttonClasses = {
  primary: 'bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white',
  secondary:
    'bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 text-gray-700 dark:text-gray-300',
  danger: 'bg-red-500 hover:bg-red-600 text-white',
}

/** Check if a value is a plain object (map) */
function isMapValue(val: unknown): val is Record<string, unknown> {
  return val !== null && typeof val === 'object' && !Array.isArray(val)
}

function tryParseJSON(val: unknown): unknown | null {
  if (typeof val !== 'string') return null
  const trimmed = val.trim()
  if (!trimmed || (!trimmed.startsWith('{') && !trimmed.startsWith('['))) return null
  try {
    return JSON.parse(trimmed)
  } catch {
    return null
  }
}

/** Try to parse a string as JSON object; returns the object or null */
function tryParseObject(val: unknown): Record<string, unknown> | null {
  if (isMapValue(val)) return val
  const parsed = tryParseJSON(val)
  if (isMapValue(parsed)) {
    return parsed
  }
  return null
}

/** Flatten a value to a copyable string */
function toDisplayString(val: unknown): string {
  if (typeof val === 'string') return val
  if (val === null || val === undefined) return ''
  return JSON.stringify(val, null, 2)
}

async function copyValue(value: unknown, index: number) {
  try {
    await navigator.clipboard.writeText(toDisplayString(value))
    copiedIndex.value = index
    setTimeout(() => {
      copiedIndex.value = null
    }, 2000)
  } catch {
    console.error('Failed to copy to clipboard')
  }
}

async function copyTitle() {
  try {
    await navigator.clipboard.writeText(props.card.title)
    titleCopied.value = true
    setTimeout(() => {
      titleCopied.value = false
    }, 2000)
  } catch {
    console.error('Failed to copy title')
  }
}

function isActionActive(actionId: string): boolean {
  return props.actionLoading === true && props.activeActionId === actionId
}

function isActionDisabled(action: { disabled?: boolean }): boolean {
  return props.actionLoading === true || action.disabled === true
}

function actionButtonLabel(action: { id: string; label: string }): string {
  return isActionActive(action.id)
    ? t('common.processing', 'Processing...')
    : tAction(action.id, action.label)
}

function handleAction(actionId: string, disabled = false) {
  if (props.actionLoading || disabled) return
  emit('action', actionId, props.card.id)
}

function toNumberOrNull(value: unknown): number | null {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string' && value.trim()) {
    const parsed = Number(value)
    if (Number.isFinite(parsed)) return parsed
  }
  return null
}

function toBooleanOrNull(value: unknown): boolean | null {
  if (typeof value === 'boolean') return value
  if (typeof value === 'string') {
    const lowered = value.trim().toLowerCase()
    if (lowered === 'true') return true
    if (lowered === 'false') return false
  }
  return null
}

function normalizeDirectoryEntry(value: unknown): DirectoryListingEntry | null {
  if (!isMapValue(value)) return null
  const pathValue =
    typeof value.path === 'string'
      ? value.path.trim()
      : typeof value.name === 'string'
        ? value.name.trim()
        : ''
  if (!pathValue) return null

  const typeValue =
    typeof value.type === 'string' ? value.type.trim() : pathValue.endsWith('/') ? 'dir' : 'file'

  const sizeValue = toNumberOrNull(value.size)
  const modifiedAtValue =
    typeof value.modified_at === 'string'
      ? value.modified_at.trim()
      : typeof value.modifiedAt === 'string'
        ? value.modifiedAt.trim()
        : ''
  const modeValue = typeof value.mode === 'string' ? value.mode.trim() : ''
  const displayValue = typeof value.display === 'string' ? value.display.trim() : ''

  return {
    path: pathValue,
    type: typeValue || 'file',
    mode: modeValue || undefined,
    size: sizeValue ?? undefined,
    modifiedAt: modifiedAtValue || undefined,
    display: displayValue || undefined,
  }
}

function extractDirectoryEntries(value: unknown): DirectoryListingEntry[] {
  if (Array.isArray(value)) {
    return value
      .map((entry) => normalizeDirectoryEntry(entry))
      .filter((entry): entry is DirectoryListingEntry => !!entry)
  }
  if (isMapValue(value)) {
    const nestedEntries = Array.isArray(value.entries) ? value.entries : []
    if (nestedEntries.length > 0) {
      return extractDirectoryEntries(nestedEntries)
    }
    const normalized = normalizeDirectoryEntry(value)
    return normalized ? [normalized] : []
  }
  if (typeof value === 'string') {
    const parsed = tryParseJSON(value)
    if (parsed !== null) {
      return extractDirectoryEntries(parsed)
    }
    return parseDirectoryListingPreview(value)
  }
  return []
}

function normalizeTextSearchMatch(value: unknown): TextSearchMatch | null {
  if (!isMapValue(value)) return null
  const path =
    typeof value.path === 'string'
      ? value.path.trim()
      : typeof value.file === 'string'
        ? value.file.trim()
        : ''
  if (!path) return null
  return {
    path,
    line: toNumberOrNull(value.line) ?? undefined,
    column: toNumberOrNull(value.column) ?? undefined,
    preview:
      typeof value.preview === 'string'
        ? value.preview.trim()
        : typeof value.content === 'string'
          ? value.content.trim()
          : undefined,
  }
}

function extractTextSearchMatches(value: unknown): TextSearchMatch[] {
  if (Array.isArray(value)) {
    return value
      .map((entry) => normalizeTextSearchMatch(entry))
      .filter((entry): entry is TextSearchMatch => !!entry)
  }
  if (isMapValue(value)) {
    const nestedMatches = Array.isArray(value.matches) ? value.matches : []
    if (nestedMatches.length > 0) return extractTextSearchMatches(nestedMatches)
    const normalized = normalizeTextSearchMatch(value)
    return normalized ? [normalized] : []
  }
  if (typeof value === 'string') {
    const parsed = tryParseJSON(value)
    if (parsed !== null) return extractTextSearchMatches(parsed)
  }
  return []
}

function extractTextSearchPayload(value: unknown): TextSearchData | null {
  const parsed = typeof value === 'string' ? (tryParseJSON(value) ?? value) : value
  if (!isMapValue(parsed)) return null
  const basePath =
    typeof parsed.base_path === 'string'
      ? parsed.base_path.trim()
      : typeof parsed.path === 'string'
        ? parsed.path.trim()
        : '.'
  const pattern = typeof parsed.pattern === 'string' ? parsed.pattern.trim() : ''
  const count = toNumberOrNull(parsed.count)
  const matches = extractTextSearchMatches(parsed.matches)
  if (!pattern) return null
  if (matches.length === 0 && count !== 0) return null

  return {
    basePath: basePath || '.',
    pattern,
    count,
    maxResults: toNumberOrNull(parsed.max_results ?? parsed.maxResults),
    caseSensitive: toBooleanOrNull(parsed.case_sensitive ?? parsed.caseSensitive),
    includeHidden: toBooleanOrNull(parsed.include_hidden ?? parsed.includeHidden),
    truncated: parsed.truncated === true,
    backend: typeof parsed.backend === 'string' ? parsed.backend.trim() : undefined,
    backendSource:
      typeof parsed.backend_source === 'string' ? parsed.backend_source.trim() : undefined,
    fallbackReason:
      typeof parsed.fallback_reason === 'string' ? parsed.fallback_reason.trim() : undefined,
    matches,
  }
}

function extractTextSearchFromDetails(
  details: TypelessCardResult['details']
): TextSearchData | null {
  if (!details?.length) return null

  let basePath = '.'
  let pattern = ''
  let count: number | null = null
  let maxResults: number | null = null
  let caseSensitive: boolean | null = null
  let includeHidden: boolean | null = null
  let truncated = false
  let backend = ''
  let backendSource = ''
  let fallbackReason = ''
  let matches: TextSearchMatch[] = []

  details.forEach((detail) => {
    const key = normalizeResultCardKey(detail.label)
    if (key === 'path' || key === 'base_path') {
      const value = toDisplayString(detail.value).trim()
      if (value) basePath = value
      return
    }
    if (key === 'pattern') {
      pattern = toDisplayString(detail.value).trim()
      return
    }
    if (key === 'count') {
      count = toNumberOrNull(detail.value)
      return
    }
    if (key === 'max_results') {
      maxResults = toNumberOrNull(detail.value)
      return
    }
    if (key === 'case_sensitive') {
      caseSensitive = toBooleanOrNull(detail.value)
      return
    }
    if (key === 'include_hidden') {
      includeHidden = toBooleanOrNull(detail.value)
      return
    }
    if (key === 'truncated') {
      truncated = toBooleanOrNull(detail.value) === true
      return
    }
    if (key === 'backend') {
      backend = toDisplayString(detail.value).trim()
      return
    }
    if (key === 'backend_source') {
      backendSource = toDisplayString(detail.value).trim()
      return
    }
    if (key === 'fallback_reason') {
      fallbackReason = toDisplayString(detail.value).trim()
      return
    }
    if (key !== 'matches') return

    const structured = extractTextSearchPayload(detail.value)
    if (structured) {
      matches = structured.matches
      if (structured.basePath) basePath = structured.basePath
      if (structured.pattern) pattern = structured.pattern
      if (structured.count !== null) count = structured.count
      if (structured.maxResults !== null) maxResults = structured.maxResults
      if (structured.caseSensitive !== null) caseSensitive = structured.caseSensitive
      if (structured.includeHidden !== null) includeHidden = structured.includeHidden
      truncated = truncated || structured.truncated
      if (structured.backend) backend = structured.backend
      if (structured.backendSource) backendSource = structured.backendSource
      if (structured.fallbackReason) fallbackReason = structured.fallbackReason
      return
    }

    matches = extractTextSearchMatches(detail.value)
  })

  if (!pattern || (matches.length === 0 && count !== 0)) return null

  return {
    basePath,
    pattern,
    count,
    maxResults,
    caseSensitive,
    includeHidden,
    truncated,
    backend: backend || undefined,
    backendSource: backendSource || undefined,
    fallbackReason: fallbackReason || undefined,
    matches,
  }
}

function extractDirectoryListingPayload(value: unknown): DirectoryListingData | null {
  const parsed = typeof value === 'string' ? (tryParseJSON(value) ?? value) : value
  if (!isMapValue(parsed)) return null

  const entries = extractDirectoryEntries(parsed.entries)
  const count = toNumberOrNull(parsed.count)
  const basePath =
    typeof parsed.base_path === 'string'
      ? parsed.base_path.trim()
      : typeof parsed.basePath === 'string'
        ? parsed.basePath.trim()
        : '.'
  if (entries.length === 0 && !(count === 0 && basePath)) return null

  const pattern =
    typeof parsed.pattern === 'string'
      ? parsed.pattern.trim()
      : typeof parsed.glob === 'string'
        ? parsed.glob.trim()
        : ''
  const typeFilter =
    typeof parsed.type === 'string'
      ? parsed.type.trim()
      : typeof parsed.type_filter === 'string'
        ? parsed.type_filter.trim()
        : ''

  return {
    kind: pattern ? 'find' : 'ls',
    basePath: basePath || '.',
    count,
    maxDepth: toNumberOrNull(parsed.max_depth ?? parsed.maxDepth),
    maxEntries: toNumberOrNull(parsed.max_entries ?? parsed.maxEntries),
    includeHidden: toBooleanOrNull(parsed.include_hidden ?? parsed.includeHidden),
    truncated: parsed.truncated === true,
    hiddenEntries: toNumberOrNull(parsed.entries_hidden_in_card ?? parsed.hiddenEntries),
    pattern: pattern || undefined,
    typeFilter: typeFilter || undefined,
    entries,
  }
}

function parseDirectoryListingPreview(raw: string): DirectoryListingEntry[] {
  const entries: DirectoryListingEntry[] = []
  raw
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean)
    .forEach((line) => {
      const match = line.match(/^\[(dir|file)\]\s+(.+?)(?:\s+\((\d+)\s+B\))?$/i)
      if (!match) return
      entries.push({
        path: (match[2] || '').trim(),
        type: (match[1] || 'file').toLowerCase(),
        size: match[3] ? Number(match[3]) : undefined,
      })
    })
  return entries
}

function extractDirectoryListingFromDetails(
  details: TypelessCardResult['details']
): DirectoryListingData | null {
  if (!details?.length) return null

  let basePath = '.'
  let count: number | null = null
  let maxDepth: number | null = null
  let maxEntries: number | null = null
  let includeHidden: boolean | null = null
  let hiddenEntries: number | null = null
  let truncated = false
  let pattern = ''
  let typeFilter = ''
  let entries: DirectoryListingEntry[] = []

  details.forEach((detail) => {
    const key = normalizeResultCardKey(detail.label)
    if (key === 'path' || key === 'base_path') {
      const value = toDisplayString(detail.value).trim()
      if (value) basePath = value
      return
    }
    if (key === 'count') {
      count = toNumberOrNull(detail.value)
      return
    }
    if (key === 'max_depth') {
      maxDepth = toNumberOrNull(detail.value)
      return
    }
    if (key === 'max_entries') {
      maxEntries = toNumberOrNull(detail.value)
      return
    }
    if (key === 'include_hidden') {
      includeHidden = toBooleanOrNull(detail.value)
      return
    }
    if (key === 'entries_hidden_in_card') {
      hiddenEntries = toNumberOrNull(detail.value)
      return
    }
    if (key === 'truncated') {
      truncated = toBooleanOrNull(detail.value) === true
      return
    }
    if (key === 'pattern') {
      pattern = toDisplayString(detail.value).trim()
      return
    }
    if (key === 'type') {
      typeFilter = toDisplayString(detail.value).trim()
      return
    }
    if (key !== 'entries') return

    const structuredEntries = extractDirectoryListingPayload(detail.value)
    if (structuredEntries) {
      entries = structuredEntries.entries
      if (structuredEntries.count !== null) count = structuredEntries.count
      if (structuredEntries.maxDepth !== null) maxDepth = structuredEntries.maxDepth
      if (structuredEntries.maxEntries !== null) maxEntries = structuredEntries.maxEntries
      if (structuredEntries.includeHidden !== null) includeHidden = structuredEntries.includeHidden
      if (structuredEntries.hiddenEntries !== null) hiddenEntries = structuredEntries.hiddenEntries
      if (structuredEntries.pattern) pattern = structuredEntries.pattern
      if (structuredEntries.typeFilter) typeFilter = structuredEntries.typeFilter
      truncated = truncated || structuredEntries.truncated
      if (structuredEntries.basePath) basePath = structuredEntries.basePath
      return
    }

    entries = extractDirectoryEntries(detail.value)
  })

  if (entries.length === 0 && count !== 0) return null

  return {
    kind: pattern ? 'find' : 'ls',
    basePath,
    count,
    maxDepth,
    maxEntries,
    includeHidden,
    truncated,
    hiddenEntries,
    pattern: pattern || undefined,
    typeFilter: typeFilter || undefined,
    entries,
  }
}

function normalizeSyntheticDetailValue(value: unknown): string | Record<string, unknown> {
  if (value === null || value === undefined) return ''
  if (isMapValue(value)) return value
  if (Array.isArray(value)) return JSON.stringify(value, null, 2)
  if (typeof value === 'string') return value
  if (typeof value === 'number' || typeof value === 'boolean') return String(value)
  return JSON.stringify(value, null, 2)
}

function buildDirectoryListingMessage(listing: DirectoryListingData): string {
  const basePath = listing.basePath || '.'
  const count = listing.count ?? listing.entries.length
  if (listing.kind === 'find') {
    if (count === 0) {
      return listing.pattern
        ? `No matches for ${listing.pattern} in ${basePath}`
        : `No matches in ${basePath}`
    }
    if (listing.pattern) {
      return `${count} matches for ${listing.pattern} in ${basePath}`
    }
    return `${count} matches in ${basePath}`
  }
  if (listing.truncated) {
    return `Showing first ${count} entries in ${basePath} (more omitted)`
  }
  if (count === 0) return `No entries in ${basePath}`
  if (count === 1) return `1 entry in ${basePath}`
  return `${count} entries in ${basePath}`
}

function buildTextSearchMessage(search: TextSearchData): string {
  const basePath = search.basePath || '.'
  const count = search.count ?? search.matches.length
  if (count === 0) return `No matches for ${search.pattern} in ${basePath}`
  return `${count} matches for ${search.pattern} in ${basePath}`
}

function directoryEntryBadgeClass(entry: DirectoryListingEntry): string {
  return entry.type === 'dir'
    ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-200'
    : 'bg-slate-200 text-slate-700 dark:bg-slate-700/80 dark:text-slate-200'
}

function formatDirectoryEntrySize(size?: number): string {
  if (typeof size !== 'number' || !Number.isFinite(size) || size < 0) return ''
  if (size < 1024) return `${size} B`
  const units = ['KB', 'MB', 'GB', 'TB']
  let value = size
  let unitIndex = -1
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024
    unitIndex += 1
  }
  if (unitIndex < 0) return `${size} B`
  const precision = value >= 10 ? 0 : 1
  return `${value.toFixed(precision)} ${units[unitIndex]}`
}

function formatDirectoryEntryTimestamp(value?: string): string {
  if (!value) return ''
  const parsed = Date.parse(value)
  if (Number.isNaN(parsed)) return value
  try {
    return new Intl.DateTimeFormat(undefined, {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    }).format(new Date(parsed))
  } catch {
    return value
  }
}

function directoryEntryMeta(entry: DirectoryListingEntry): string {
  const parts = [
    entry.mode,
    formatDirectoryEntrySize(entry.size),
    formatDirectoryEntryTimestamp(entry.modifiedAt),
  ]
    .map((part) => (part || '').trim())
    .filter(Boolean)
  if (parts.length > 0) return parts.join('  ')
  return (entry.display || '').trim()
}

const listingSummaryItems = computed(() => {
  const listing = directoryListing.value
  if (!listing) return []

  const items = [
    { label: 'path', value: listing.basePath || '.' },
    ...(listing.pattern ? [{ label: 'pattern', value: listing.pattern }] : []),
    ...(listing.typeFilter && listing.typeFilter !== 'all'
      ? [{ label: 'type', value: listing.typeFilter }]
      : []),
    ...(listing.maxDepth !== null ? [{ label: 'max_depth', value: String(listing.maxDepth) }] : []),
    ...(listing.maxEntries !== null
      ? [{ label: 'max_entries', value: String(listing.maxEntries) }]
      : []),
    ...(listing.includeHidden !== null
      ? [
          {
            label: 'include_hidden',
            value: listing.includeHidden ? t('common.yes', 'Yes') : t('common.no', 'No'),
          },
        ]
      : []),
  ]

  if (listing.hiddenEntries && listing.hiddenEntries > 0) {
    items.push({
      label: 'entries_hidden_in_card',
      value: String(listing.hiddenEntries),
    })
  }

  return items
})

function searchMatchLocation(match: TextSearchMatch): string {
  const line = match.line ?? null
  const column = match.column ?? null
  if (line !== null && column !== null) return `L${line}:C${column}`
  if (line !== null) return `L${line}`
  if (column !== null) return `C${column}`
  return ''
}

const textSearchBadgeLabel = computed(() => {
  const normalizedTitle = normalizeResultCardKey(props.card.title || '')
  if (normalizedTitle === 'rg' || normalizedTitle === 'ripgrep') return 'RG'
  if (normalizedTitle === 'grep') return 'GREP'
  if (textSearchResults.value?.backend === 'ripgrep') return 'RG'
  if (textSearchResults.value?.backend === 'builtin') return 'GREP'
  return 'SEARCH'
})

function normalizeResultCardKey(input: string): string {
  return input
    .toLowerCase()
    .trim()
    .replace(/[\s-]+/g, '_')
    .replace(/[^a-z0-9_]+/g, '')
    .replace(/_+/g, '_')
    .replace(/^_+|_+$/g, '')
}

type ResultCardTextParams = Record<string, string | number>

type ResultCardPattern = {
  key: string
  regex: RegExp
  params?: (match: RegExpMatchArray) => ResultCardTextParams
}

const RESULT_CARD_MESSAGE_PATTERNS: ResultCardPattern[] = [
  {
    key: 'found_results',
    regex: /^Found (\d+) results$/,
    params: (match) => ({ count: Number(match[1] || 0) }),
  },
  {
    key: 'reminder_count',
    regex: /^(\d+) reminders$/,
    params: (match) => ({ count: Number(match[1] || 0) }),
  },
  {
    key: 'cleared_reminders',
    regex: /^Cleared (\d+) reminders$/,
    params: (match) => ({ count: Number(match[1] || 0) }),
  },
  {
    key: 'showing_first_entries_in_path',
    regex: /^Showing first (\d+) entries in (.+) \(more omitted\)$/,
    params: (match) => ({ count: Number(match[1] || 0), path: match[2] || '' }),
  },
  {
    key: 'single_entry_in_path',
    regex: /^1 entry in (.+)$/,
    params: (match) => ({ path: match[1] || '' }),
  },
  {
    key: 'no_entries_in_path',
    regex: /^No entries in (.+)$/,
    params: (match) => ({ path: match[1] || '' }),
  },
  {
    key: 'entries_in_path',
    regex: /^(\d+) entries in (.+)$/,
    params: (match) => ({ count: Number(match[1] || 0), path: match[2] || '' }),
  },
  {
    key: 'matches_for_pattern_in_path',
    regex: /^(\d+) matches for (.+) in (.+)$/,
    params: (match) => ({
      count: Number(match[1] || 0),
      pattern: match[2] || '',
      path: match[3] || '',
    }),
  },
  {
    key: 'no_matches_for_pattern_in_path',
    regex: /^No matches for (.+) in (.+)$/,
    params: (match) => ({ pattern: match[1] || '', path: match[2] || '' }),
  },
  {
    key: 'screenshot_captured_for_tab',
    regex: /^Screenshot captured for tab (.+)$/,
    params: (match) => ({ target: match[1] || '' }),
  },
  {
    key: 'screenshot_captured_for',
    regex: /^Screenshot captured for (.+)$/,
    params: (match) => ({ target: match[1] || '' }),
  },
  {
    key: 'browser_start_failed',
    regex: /^browser start failed:\s*(.+)$/i,
    params: (match) => ({ detail: match[1] || '' }),
  },
  {
    key: 'navigation_failed',
    regex: /^navigation failed:\s*(.+)$/i,
    params: (match) => ({ detail: match[1] || '' }),
  },
]

const RESULT_CARD_WARNING_PATTERNS: ResultCardPattern[] = [
  {
    key: 'listing_truncated',
    regex: /^Listing was truncated; narrow the path or increase max_entries\.$/,
  },
]

function translateResultCardPattern(
  scope: string,
  raw: string,
  patterns: ResultCardPattern[]
): string {
  for (const pattern of patterns) {
    const match = raw.match(pattern.regex)
    if (!match) continue
    const key = `${scope}.${pattern.key}`
    if (!te(key)) return raw
    const params = pattern.params ? pattern.params(match) : null
    return params ? t(key, params) : t(key)
  }
  return raw
}

function translateResultCardMessage(raw: string): string {
  const trimmed = raw.trim()
  if (!trimmed) return ''

  const directKey = `resultCard.messages.${normalizeResultCardKey(trimmed)}`
  if (te(directKey)) return t(directKey)

  const browserCompat = translateHistoricalEnglishBrowserResult(
    trimmed,
    {
      t: (key, named) => String(named ? t(key, named) : t(key)),
      te: (key) => te(key),
    },
    {
      preserveRawEnglishScreenshot: true,
    }
  )
  if (browserCompat !== trimmed) return browserCompat

  return translateResultCardPattern(
    'resultCard.messageTemplates',
    trimmed,
    RESULT_CARD_MESSAGE_PATTERNS
  )
}

function translateResultCardWarning(raw: string): string {
  const trimmed = raw.trim()
  if (!trimmed) return ''

  if (RESULT_CARD_WARNING_PATTERNS[0]?.regex.test(trimmed)) {
    return t(
      'resultCard.warnings.listing_truncated',
      'Listing was truncated; narrow the path or increase max_entries.'
    )
  }

  return raw
}

function isLikelyLocalFilesystemPath(raw: string): boolean {
  const trimmed = raw.trim()
  if (!trimmed || !isLocalAbsolutePath(trimmed) || isApiPath(trimmed) || isHttpUrl(trimmed)) {
    return false
  }

  if (WINDOWS_ABS_PATH_RE.test(trimmed)) return true
  if (!trimmed.startsWith('/')) return false

  const firstSegment = trimmed.slice(1).split('/')[0]?.trim().toLowerCase()

  return !!firstSegment && POSIX_LOCAL_ROOT_SEGMENTS.has(firstSegment)
}

function toLocalFileContentUrl(path: string): string {
  const trimmed = path.trim()
  if (!isLikelyLocalFilesystemPath(trimmed)) return ''
  return `/api/v1/system/local-file/content?path=${encodeURIComponent(trimmed)}&inline=1`
}

function normalizeImageSource(raw: string, mimeType?: string): string {
  const trimmed = raw.trim()
  if (!trimmed) return ''
  const localFileContentUrl = toLocalFileContentUrl(trimmed)
  if (localFileContentUrl) return localFileContentUrl
  if (
    trimmed.startsWith('data:image/') ||
    trimmed.startsWith('http://') ||
    trimmed.startsWith('https://') ||
    trimmed.startsWith('/')
  ) {
    return trimmed
  }
  const mime = mimeType?.trim()
  if (mime?.startsWith('image/')) {
    return `data:${mime};base64,${trimmed}`
  }
  return `data:image/png;base64,${trimmed}`
}

function isImageDetailLabel(label: string): boolean {
  return IMAGE_DETAIL_LABELS.has(normalizeResultCardKey(label))
}

function isLikelyImageString(raw: string): boolean {
  const trimmed = raw.trim()
  if (!trimmed) return false
  if (isLikelyLocalFilesystemPath(trimmed)) {
    return IMAGE_URL_SUFFIX_RE.test(trimmed)
  }
  if (
    trimmed.startsWith('data:image/') ||
    trimmed.startsWith('http://') ||
    trimmed.startsWith('https://') ||
    trimmed.startsWith('/')
  ) {
    return trimmed.startsWith('data:image/') || IMAGE_URL_SUFFIX_RE.test(trimmed)
  }
  return /^[A-Za-z0-9+/=\r\n]+$/.test(trimmed) && trimmed.length >= 64
}

function pushResolvedImage(
  items: ResolvedImageItem[],
  seen: Set<string>,
  raw: string,
  alt: string,
  mimeType?: string
) {
  const src = normalizeImageSource(raw, mimeType)
  if (!src || seen.has(src)) return
  seen.add(src)
  items.push({ src, alt })
}

function extractResolvedImages(
  value: unknown,
  label: string,
  items: ResolvedImageItem[],
  seen: Set<string>
) {
  if (typeof value === 'string') {
    const trimmed = value.trim()
    if (!trimmed) return
    if (trimmed.startsWith('{') || trimmed.startsWith('[')) {
      try {
        const parsed = JSON.parse(trimmed)
        extractResolvedImages(parsed, label, items, seen)
        return
      } catch {
        // Fall back to string heuristics below.
      }
    }
    if (isImageDetailLabel(label) || isLikelyImageString(trimmed)) {
      pushResolvedImage(items, seen, trimmed, label)
    }
    return
  }

  if (Array.isArray(value)) {
    value.forEach((entry) => extractResolvedImages(entry, label, items, seen))
    return
  }

  if (!isMapValue(value)) return

  const mimeType =
    typeof value.mime_type === 'string'
      ? value.mime_type
      : typeof value.mimeType === 'string'
        ? value.mimeType
        : undefined

  const directStringKeys = ['src', 'url', 'image', 'screenshot', 'thumbnail']
  directStringKeys.forEach((key) => {
    const candidate = value[key]
    if (typeof candidate === 'string' && candidate.trim()) {
      pushResolvedImage(items, seen, candidate, label || key, mimeType)
    }
  })

  const binaryKeys = ['data', 'base64']
  binaryKeys.forEach((key) => {
    const candidate = value[key]
    if (typeof candidate === 'string' && candidate.trim()) {
      pushResolvedImage(items, seen, candidate, label || key, mimeType)
    }
  })

  for (const [subKey, subValue] of Object.entries(value)) {
    if (
      typeof subValue === 'string' &&
      !isImageDetailLabel(subKey) &&
      !isLikelyImageString(subValue)
    )
      continue
    extractResolvedImages(subValue, subKey, items, seen)
  }
}

function tLabel(label: string): string {
  const key = 'resultCard.labels.' + normalizeResultCardKey(label)
  return te(key) ? t(key) : label
}

function tDetailValue(_label: string, value: unknown): string {
  if (typeof value === 'boolean') {
    return value ? t('common.yes', 'Yes') : t('common.no', 'No')
  }

  const rawValue = typeof value === 'string' ? value.trim() : toDisplayString(value)
  if (!rawValue) return ''

  const lowered = rawValue.toLowerCase()
  if (lowered === 'true') return t('common.yes', 'Yes')
  if (lowered === 'false') return t('common.no', 'No')

  if (normalizeResultCardKey(_label) === 'strategy') {
    switch (normalizeResultCardKey(rawValue)) {
      case 'strict':
        return t('resultCard.values.strategy.strict', rawValue)
    }
  }

  return rawValue
}

function tAction(id: string, fallback: string): string {
  return translateCardActionLabel({
    id,
    fallback,
    t,
    te,
    scopes: ['resultCard.actions'],
  })
}

async function handleRevealLocation(value: unknown) {
  if (typeof value !== 'string') return
  const path = value.trim()
  if (!isLocalAbsolutePath(path)) return
  await openInBrowser(path)
}

const translatedTitle = computed(() => {
  if (!props.card.title) return ''
  const rawTitle = props.card.title.trim()
  const normalizedTitle = rawTitle.toLowerCase().replace(/\s+/g, '_')
  const localizedToolName = getLocalizedToolName(normalizedTitle, t, te)
  if (
    localizedToolName !==
    normalizedTitle.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase())
  ) {
    return localizedToolName
  }
  const key = 'resultCard.titles.' + normalizedTitle
  const translated = t(key, rawTitle)
  return translated === key ? rawTitle : translated
})

const parsedMessagePayload = computed(() => tryParseObject(props.card.message))
const textSearchResults = computed<TextSearchData | null>(() => {
  const fromMessage = extractTextSearchPayload(parsedMessagePayload.value)
  if (fromMessage) return fromMessage
  return extractTextSearchFromDetails(props.card.details)
})
const directoryListing = computed<DirectoryListingData | null>(() => {
  const fromMessage = extractDirectoryListingPayload(parsedMessagePayload.value)
  if (fromMessage) return fromMessage
  return extractDirectoryListingFromDetails(props.card.details)
})
const displayMessage = computed(() => {
  const payload = parsedMessagePayload.value
  if (payload) {
    const nestedMessage = typeof payload.message === 'string' ? payload.message.trim() : ''
    if (nestedMessage) return nestedMessage
    const search = extractTextSearchPayload(payload)
    if (search) return buildTextSearchMessage(search)
    const listing = extractDirectoryListingPayload(payload)
    if (listing) return buildDirectoryListingMessage(listing)
    return ''
  }
  const plainMessage = (props.card.message || '').trim()
  if (plainMessage) return plainMessage
  if (textSearchResults.value) return buildTextSearchMessage(textSearchResults.value)
  if (directoryListing.value) return buildDirectoryListingMessage(directoryListing.value)
  return ''
})

const errorKeyMap: [RegExp, string][] = []

const translatedMessage = computed(() => {
  const msg = displayMessage.value
  if (!msg) return ''
  const directOrTemplated = translateResultCardMessage(msg)
  if (directOrTemplated !== msg) return directOrTemplated
  if (props.card.status === 'error') {
    for (const [re, key] of errorKeyMap) {
      if (re.test(msg) && te(key)) return t(key, msg)
    }
  }
  return msg
})

const warningText = computed(() => (props.card.warning || '').trim())
const translatedWarningText = computed(() => translateResultCardWarning(warningText.value))
const warningCodeLabel = computed(() => formatToolWarningCodeLabel(props.card.warning_code, t))
const syntheticMessageDetails = computed(() => {
  const payload = parsedMessagePayload.value
  if (!payload) return []

  const nestedMessage = typeof payload.message === 'string' ? payload.message.trim() : ''
  if (nestedMessage) return []

  return Object.entries(payload)
    .filter(([key]) => key !== 'message')
    .filter(([key]) => !(textSearchResults.value && normalizeResultCardKey(key) === 'matches'))
    .filter(([key]) => !(directoryListing.value && normalizeResultCardKey(key) === 'entries'))
    .filter(([key]) => !(hasRenderedImages.value && isImageDetailLabel(key)))
    .map(([key, value]) => {
      const normalizedValue = normalizeSyntheticDetailValue(value)
      return {
        label: key,
        value: normalizedValue,
        multiline:
          typeof normalizedValue === 'string' &&
          (normalizedValue.includes('\n') || normalizedValue.length > 120),
      }
    })
    .filter((detail) => {
      if (typeof detail.value === 'string') return detail.value.trim() !== ''
      return true
    })
})
const resolvedImageItems = computed<ResolvedImageItem[]>(() => {
  const items: ResolvedImageItem[] = []
  const seen = new Set<string>()

  ;(props.card.images || []).forEach((image) => {
    if (!image) return
    pushResolvedImage(
      items,
      seen,
      image.src,
      image.alt || translatedTitle.value || 'image',
      undefined
    )
  })

  const rawCardImage = (props.card.image || '').trim()
  if (rawCardImage) {
    pushResolvedImage(items, seen, rawCardImage, translatedTitle.value || 'image')
  }

  if (parsedMessagePayload.value) {
    extractResolvedImages(parsedMessagePayload.value, translatedTitle.value || 'image', items, seen)
  }

  ;(props.card.details || []).forEach((detail) => {
    if (!isImageDetailLabel(detail.label)) return
    extractResolvedImages(detail.value, detail.label, items, seen)
  })

  return items
})
const hasRenderedImages = computed(() => resolvedImageItems.value.length > 0)
const textSearchSummaryItems = computed(() => {
  const search = textSearchResults.value
  if (!search) return []

  return [
    { label: 'path', value: search.basePath || '.' },
    { label: 'pattern', value: search.pattern },
    ...(search.maxResults !== null
      ? [{ label: 'max_results', value: String(search.maxResults) }]
      : []),
    ...(search.caseSensitive !== null
      ? [
          {
            label: 'case_sensitive',
            value: search.caseSensitive ? t('common.yes', 'Yes') : t('common.no', 'No'),
          },
        ]
      : []),
    ...(search.includeHidden !== null
      ? [
          {
            label: 'include_hidden',
            value: search.includeHidden ? t('common.yes', 'Yes') : t('common.no', 'No'),
          },
        ]
      : []),
    ...(search.backend ? [{ label: 'backend', value: search.backend }] : []),
    ...(search.backendSource ? [{ label: 'backend_source', value: search.backendSource }] : []),
  ]
})

// Filter out details that are redundant with the title/message
const visibleDetails = computed(() => {
  const explicitDetails = props.card.details || []
  const seenLabels = new Set(explicitDetails.map((detail) => normalizeResultCardKey(detail.label)))
  const mergedDetails = [...explicitDetails]

  syntheticMessageDetails.value.forEach((detail) => {
    const key = normalizeResultCardKey(detail.label)
    if (seenLabels.has(key)) return
    seenLabels.add(key)
    mergedDetails.push(detail)
  })

  return mergedDetails
    .filter((d) => {
      const lbl = d.label.toLowerCase()
      if (lbl === 'status' || lbl === '状态') return false
      if (lbl === 'warning' || lbl === 'warning_code') return false
      if (isImageDetailLabel(d.label) && hasRenderedImages.value) return false
      if (textSearchResults.value) {
        const normalizedLabel = normalizeResultCardKey(d.label)
        if (
          normalizedLabel === 'matches' ||
          normalizedLabel === 'path' ||
          normalizedLabel === 'base_path' ||
          normalizedLabel === 'pattern' ||
          normalizedLabel === 'count' ||
          normalizedLabel === 'max_results' ||
          normalizedLabel === 'case_sensitive' ||
          normalizedLabel === 'include_hidden' ||
          normalizedLabel === 'truncated' ||
          normalizedLabel === 'backend' ||
          normalizedLabel === 'backend_source'
        ) {
          return false
        }
      }
      if (directoryListing.value) {
        const normalizedLabel = normalizeResultCardKey(d.label)
        if (
          normalizedLabel === 'entries' ||
          normalizedLabel === 'path' ||
          normalizedLabel === 'base_path' ||
          normalizedLabel === 'count' ||
          normalizedLabel === 'pattern' ||
          normalizedLabel === 'type' ||
          normalizedLabel === 'max_depth' ||
          normalizedLabel === 'max_entries' ||
          normalizedLabel === 'include_hidden' ||
          normalizedLabel === 'entries_hidden_in_card' ||
          normalizedLabel === 'truncated'
        ) {
          return false
        }
      }
      if ((lbl === 'result' || lbl === '结果') && props.card.message) return false
      return true
    })
    .map((d) => {
      const revealPath =
        typeof d.reveal_path === 'string' && isLocalAbsolutePath(d.reveal_path)
          ? d.reveal_path.trim()
          : ''
      const directPath =
        typeof d.value === 'string' && isLocalAbsolutePath(d.value) ? d.value.trim() : ''
      const localPathTarget = revealPath || directPath

      return {
        ...d,
        parsedObject: tryParseObject(d.value),
        isMultiline: d.multiline || (typeof d.value === 'string' && d.value.includes('\n')),
        isLink: typeof d.value === 'string' && (isHttpUrl(d.value) || isApiPath(d.value)),
        localPathTarget,
        isLocalPath: localPathTarget !== '',
      }
    })
})
const emptyStateMessage = computed(() => t('resultCard.messages.no_result_data', 'No result data'))
const showEmptyState = computed(() => {
  if (translatedMessage.value || warningText.value || warningCodeLabel.value) return false
  if (hasRenderedImages.value || textSearchResults.value || directoryListing.value) return false
  if (visibleDetails.value.length > 0) return false
  return !props.card.actions || props.card.actions.length === 0
})
</script>

<template>
  <div
    class="rounded-lg border overflow-hidden bg-white dark:bg-gray-800 shadow-sm"
    :class="statusConfig[cardStatus].border"
  >
    <!-- Header -->
    <div
      class="flex items-center gap-2.5 px-4 py-2.5 border-b group/header"
      :class="[statusConfig[cardStatus].headerBg, statusConfig[cardStatus].headerBorder]"
    >
      <span
        class="w-5 h-5 rounded-full flex items-center justify-center text-xs flex-shrink-0"
        :class="[statusConfig[cardStatus].iconBg, statusConfig[cardStatus].iconColor]"
      >
        {{ statusConfig[cardStatus].icon }}
      </span>
      <span
        class="text-sm font-medium text-gray-800 dark:text-gray-100 truncate flex-1"
        :class="{ 'font-mono': isTitleCopyable }"
      >
        {{ translatedTitle }}
      </span>
      <span
        v-if="warningCodeLabel"
        class="inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wide bg-amber-100 text-amber-800 dark:bg-amber-900/50 dark:text-amber-200"
      >
        {{ warningCodeLabel }}
      </span>
      <button
        v-if="isTitleCopyable"
        class="p-1 rounded opacity-0 group-hover/header:opacity-100 hover:bg-black/10 dark:hover:bg-white/10 transition-all flex-shrink-0"
        :title="titleCopied ? t('resultCard.copied', 'Copied!') : t('resultCard.copy', 'Copy')"
        @click="copyTitle"
      >
        <svg
          v-if="!titleCopied"
          xmlns="http://www.w3.org/2000/svg"
          class="h-3.5 w-3.5 text-gray-400"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"
          />
        </svg>
        <svg
          v-else
          xmlns="http://www.w3.org/2000/svg"
          class="h-3.5 w-3.5 text-emerald-500"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M5 13l4 4L19 7"
          />
        </svg>
      </button>
    </div>

    <!-- Body -->
    <div class="px-4 py-3">
      <div
        v-if="hasRenderedImages"
        class="rounded-md overflow-hidden border border-gray-200 dark:border-gray-700/60 bg-gray-50 dark:bg-gray-900/60"
      >
        <img
          v-if="resolvedImageItems.length === 1"
          :src="resolvedImageItems[0]?.src"
          :alt="resolvedImageItems[0]?.alt || translatedTitle || 'image'"
          class="w-full max-h-[22rem] object-contain"
          loading="lazy"
        />
        <div v-else class="grid grid-cols-2 gap-2 p-2">
          <img
            v-for="(image, index) in resolvedImageItems"
            :key="`${image.src}-${index}`"
            :src="image.src"
            :alt="image.alt || translatedTitle || 'image'"
            class="w-full max-h-56 rounded object-contain bg-white/60 dark:bg-gray-950/40"
            loading="lazy"
          />
        </div>
      </div>

      <!-- Message -->
      <p
        v-if="translatedMessage"
        class="text-sm text-gray-600 dark:text-gray-300 leading-relaxed"
        :class="hasRenderedImages ? 'mt-3' : ''"
      >
        {{ translatedMessage }}
      </p>

      <p
        v-else-if="showEmptyState"
        class="text-sm text-gray-500 dark:text-gray-400 leading-relaxed"
        :class="hasRenderedImages ? 'mt-3' : ''"
      >
        {{ emptyStateMessage }}
      </p>

      <div
        v-if="warningText || warningCodeLabel"
        class="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 dark:border-amber-800/60 dark:bg-amber-900/20"
        :class="translatedMessage ? 'mt-3' : ''"
      >
        <div
          class="flex items-center gap-2 text-xs font-semibold text-amber-800 dark:text-amber-200"
        >
          <span>{{ warningCodeLabel || t('toolWarnings.warning', 'Warning') }}</span>
        </div>
        <p
          v-if="translatedWarningText"
          class="mt-1 text-sm leading-relaxed text-amber-900 dark:text-amber-100"
        >
          {{ translatedWarningText }}
        </p>
      </div>

      <div
        v-if="textSearchResults"
        class="rounded-md border border-gray-200 bg-gray-50 dark:border-gray-700/60 dark:bg-gray-900/40"
        :class="translatedMessage || warningText || warningCodeLabel ? 'mt-3' : ''"
      >
        <div
          v-if="textSearchSummaryItems.length > 0"
          class="flex flex-wrap gap-2 px-3.5 py-3 border-b border-gray-200 dark:border-gray-700/60"
        >
          <span
            class="inline-flex items-center rounded-full px-2.5 py-1 text-[11px] font-semibold tracking-wide bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-200"
          >
            {{ textSearchBadgeLabel }}
          </span>
          <span
            v-for="item in textSearchSummaryItems"
            :key="`${item.label}-${item.value}`"
            class="inline-flex items-center gap-1 rounded-full border border-gray-200 bg-white px-2.5 py-1 text-[11px] text-gray-600 dark:border-gray-700 dark:bg-gray-800/80 dark:text-gray-300"
          >
            <span class="text-gray-400 dark:text-gray-500">{{ tLabel(item.label) }}</span>
            <span class="font-medium text-gray-700 dark:text-gray-100">{{ item.value }}</span>
          </span>
        </div>
        <div
          v-if="textSearchResults.matches.length > 0"
          class="max-h-80 overflow-y-auto divide-y divide-gray-200 dark:divide-gray-700/60"
        >
          <div
            v-for="(match, index) in textSearchResults.matches"
            :key="`${match.path}-${match.line ?? 0}-${match.column ?? 0}-${index}`"
            class="px-3.5 py-3"
          >
            <div class="flex items-start gap-3">
              <div class="min-w-0 flex-1">
                <div class="flex items-start justify-between gap-3">
                  <div class="break-all font-mono text-xs text-gray-800 dark:text-gray-100">
                    {{ match.path }}
                  </div>
                  <span
                    v-if="searchMatchLocation(match)"
                    class="inline-flex items-center rounded-full border border-amber-200 bg-white px-2 py-0.5 text-[11px] font-medium text-amber-700 dark:border-amber-800/60 dark:bg-gray-800/80 dark:text-amber-200"
                  >
                    {{ searchMatchLocation(match) }}
                  </span>
                </div>
                <pre
                  v-if="match.preview"
                  class="mt-2 overflow-x-auto rounded border border-gray-200 bg-white px-3 py-2 text-xs leading-relaxed text-gray-700 dark:border-gray-700/60 dark:bg-gray-950/50 dark:text-gray-200 font-mono whitespace-pre-wrap break-all"
                  >{{ match.preview }}</pre
                >
              </div>
            </div>
          </div>
        </div>
      </div>

      <div
        v-if="directoryListing && directoryListing.entries.length > 0"
        class="rounded-md border border-gray-200 bg-gray-50 dark:border-gray-700/60 dark:bg-gray-900/40"
        :class="translatedMessage || warningText || warningCodeLabel ? 'mt-3' : ''"
      >
        <div
          v-if="listingSummaryItems.length > 0"
          class="flex flex-wrap gap-2 px-3.5 py-3 border-b border-gray-200 dark:border-gray-700/60"
        >
          <span
            class="inline-flex items-center rounded-full px-2.5 py-1 text-[11px] font-semibold tracking-wide"
            :class="
              directoryListing.kind === 'find'
                ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-200'
                : 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-200'
            "
          >
            {{ directoryListing.kind === 'find' ? 'FIND' : 'LIST' }}
          </span>
          <span
            v-for="item in listingSummaryItems"
            :key="`${item.label}-${item.value}`"
            class="inline-flex items-center gap-1 rounded-full border border-gray-200 bg-white px-2.5 py-1 text-[11px] text-gray-600 dark:border-gray-700 dark:bg-gray-800/80 dark:text-gray-300"
          >
            <span class="text-gray-400 dark:text-gray-500">{{ tLabel(item.label) }}</span>
            <span class="font-medium text-gray-700 dark:text-gray-100">{{ item.value }}</span>
          </span>
        </div>
        <div class="max-h-80 overflow-y-auto divide-y divide-gray-200 dark:divide-gray-700/60">
          <div
            v-for="(entry, index) in directoryListing.entries"
            :key="`${entry.path}-${index}`"
            class="px-3.5 py-2.5"
          >
            <div class="flex items-start gap-3">
              <span
                class="mt-0.5 inline-flex min-w-[3.5rem] items-center justify-center rounded-full px-2 py-0.5 text-[11px] font-semibold tracking-wide"
                :class="directoryEntryBadgeClass(entry)"
              >
                {{ entry.type === 'dir' ? 'DIR' : 'FILE' }}
              </span>
              <div class="min-w-0 flex-1">
                <div class="break-all font-mono text-xs text-gray-800 dark:text-gray-100">
                  {{ entry.path }}
                </div>
                <div
                  v-if="directoryEntryMeta(entry)"
                  class="mt-1 break-all text-[11px] leading-relaxed text-gray-500 dark:text-gray-400"
                >
                  {{ directoryEntryMeta(entry) }}
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Details -->
      <div v-if="visibleDetails.length > 0" class="mt-2.5">
        <div
          class="rounded-md bg-gray-50 dark:bg-gray-900/40 divide-y divide-gray-100 dark:divide-gray-700/50"
        >
          <div
            v-for="(detail, index) in visibleDetails"
            :key="index"
            class="result-detail-row px-3.5 py-2.5 text-sm group"
            :class="
              detail.parsedObject || detail.isMultiline
                ? 'result-detail-row--stacked flex flex-col gap-1.5'
                : 'result-detail-row--inline grid grid-cols-[fit-content(8rem)_minmax(0,1fr)] items-center gap-3'
            "
          >
            <span class="result-detail-label text-gray-400 dark:text-gray-500 text-xs">{{
              tLabel(detail.label)
            }}</span>
            <!-- Nested table for map/object values -->
            <div
              v-if="detail.parsedObject"
              class="rounded border border-gray-200 dark:border-gray-700/60 bg-white dark:bg-gray-800/60 divide-y divide-gray-100 dark:divide-gray-700/40 overflow-hidden"
            >
              <div
                v-for="(subVal, subKey) in detail.parsedObject"
                :key="String(subKey)"
                class="result-detail-subrow grid grid-cols-[fit-content(8rem)_minmax(0,1fr)] items-center gap-3 px-3 py-1.5 text-xs"
              >
                <span class="result-detail-label text-gray-400 dark:text-gray-500">{{
                  tLabel(String(subKey))
                }}</span>
                <span
                  class="result-detail-value text-gray-700 dark:text-gray-300 font-mono min-w-0 break-all"
                  >{{ toDisplayString(subVal) }}</span
                >
              </div>
            </div>
            <!-- Multiline text value (e.g. stdout) -->
            <div
              v-else-if="detail.isMultiline"
              class="rounded border border-gray-200 dark:border-gray-700/60 bg-gray-50 dark:bg-gray-900/60 overflow-hidden"
            >
              <pre
                class="px-3 py-2 text-xs text-gray-700 dark:text-gray-300 font-mono whitespace-pre-wrap break-all overflow-x-auto max-h-64 overflow-y-auto leading-relaxed"
                >{{ detail.value }}</pre
              >
            </div>
            <!-- Link value -->
            <div v-else-if="detail.isLink" class="flex items-center gap-1.5">
              <a
                :href="String(detail.value)"
                target="_blank"
                rel="noopener noreferrer"
                class="text-xs font-medium text-blue-600 dark:text-blue-400 hover:underline"
                >{{ t('resultCard.openLink', 'Open') }} ↗</a
              >
            </div>
            <div
              v-else-if="detail.isLocalPath"
              class="result-detail-content flex min-w-0 items-center justify-end gap-3"
            >
              <span
                class="min-w-0 flex-1 break-all text-right text-gray-700 dark:text-gray-300 font-mono text-xs"
              >
                {{ tDetailValue(detail.label, detail.value)
                }}<template v-if="detail.suffix"> {{ tLabel(detail.suffix) }}</template>
              </span>
              <button
                class="shrink-0 text-xs font-medium text-blue-600 dark:text-blue-400 hover:underline"
                @click="handleRevealLocation(detail.localPathTarget)"
              >
                {{ t('common.openLocation', 'Open location') }}
              </button>
            </div>
            <!-- Simple string value -->
            <div
              v-else
              class="result-detail-content flex min-w-0 items-center justify-end gap-1.5"
            >
              <span class="min-w-0 break-all text-right text-gray-700 dark:text-gray-300 font-mono text-xs"
                >{{ tDetailValue(detail.label, detail.value)
                }}<template v-if="detail.suffix"> {{ tLabel(detail.suffix) }}</template></span
              >
              <button
                v-if="detail.copyable"
                class="shrink-0 rounded p-0.5 opacity-0 transition-all hover:bg-gray-200 group-hover:opacity-100 dark:hover:bg-gray-700"
                :title="
                  copiedIndex === index
                    ? t('resultCard.copied', 'Copied!')
                    : t('resultCard.copy', 'Copy')
                "
                @click="copyValue(detail.value, index)"
              >
                <svg
                  v-if="copiedIndex !== index"
                  xmlns="http://www.w3.org/2000/svg"
                  class="h-3.5 w-3.5 text-gray-400"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"
                  />
                </svg>
                <svg
                  v-else
                  xmlns="http://www.w3.org/2000/svg"
                  class="h-3.5 w-3.5 text-emerald-500"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M5 13l4 4L19 7"
                  />
                </svg>
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- Actions -->
      <div v-if="card.actions && card.actions.length > 0" class="mt-3 flex flex-wrap gap-2">
        <button
          v-for="action in card.actions"
          :key="action.id"
          class="px-3 py-1.5 rounded-md text-xs font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          :class="[
            buttonClasses[action.variant || 'secondary'],
            { 'opacity-60 cursor-wait': actionLoading },
          ]"
          :disabled="isActionDisabled(action)"
          :aria-busy="isActionActive(action.id) ? 'true' : undefined"
          @click="handleAction(action.id, !!action.disabled)"
        >
          <span
            v-if="isActionActive(action.id)"
            class="result-action-icon-gap result-action-spinner inline-block h-3 w-3 animate-spin rounded-full border border-current align-[-2px]"
          />
          <span v-else-if="action.icon" class="result-action-icon-gap">{{ action.icon }}</span>
          {{ actionButtonLabel(action) }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.result-detail-label {
  overflow-wrap: anywhere;
}

.result-detail-content {
  min-width: 0;
}

.result-detail-value {
  min-width: 0;
  overflow-wrap: anywhere;
  text-align: end;
}

.result-action-icon-gap {
  margin-inline-end: 0.25rem;
}

.result-action-spinner {
  border-inline-end-color: transparent;
}
</style>
