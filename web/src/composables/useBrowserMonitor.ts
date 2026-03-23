import { computed, ref } from 'vue'

const MONITOR_OPEN_KEY = 'zima.browser.monitor.open.v1'
const MONITOR_COLLAPSED_KEY = 'zima.browser.monitor.collapsed.v1'

const initialized = ref(false)
const isOpen = ref(false)
const isCollapsed = ref(false)
const activeTaskCount = ref(0)
const sessionCount = ref(0)

function loadStoredBoolean(key: string, fallback: boolean): boolean {
  try {
    const value = localStorage.getItem(key)
    if (value == null) return fallback
    return value === '1'
  } catch {
    return fallback
  }
}

function persistBoolean(key: string, value: boolean) {
  try {
    localStorage.setItem(key, value ? '1' : '0')
  } catch {
    // Ignore storage failures in restricted environments.
  }
}

function ensureInitialized() {
  if (initialized.value) return
  isOpen.value = loadStoredBoolean(MONITOR_OPEN_KEY, false)
  isCollapsed.value = loadStoredBoolean(MONITOR_COLLAPSED_KEY, false)
  initialized.value = true
}

function setOpen(value: boolean) {
  ensureInitialized()
  isOpen.value = value
  persistBoolean(MONITOR_OPEN_KEY, value)
}

function setCollapsed(value: boolean) {
  ensureInitialized()
  isCollapsed.value = value
  persistBoolean(MONITOR_COLLAPSED_KEY, value)
}

function openMonitor(options?: { expand?: boolean }) {
  setOpen(true)
  if (options?.expand) setCollapsed(false)
}

function closeMonitor() {
  setOpen(false)
}

function toggleMonitor(options?: { expandWhenOpening?: boolean }) {
  ensureInitialized()
  const nextOpen = !isOpen.value
  setOpen(nextOpen)
  if (nextOpen && options?.expandWhenOpening) {
    setCollapsed(false)
  }
}

function setActivitySnapshot(snapshot: { activeTaskCount?: number; sessionCount?: number }) {
  activeTaskCount.value = Math.max(0, Math.round(Number(snapshot.activeTaskCount || 0)))
  sessionCount.value = Math.max(0, Math.round(Number(snapshot.sessionCount || 0)))
}

const hasLiveActivity = computed(() => activeTaskCount.value > 0 || sessionCount.value > 0)
const activitySummary = computed(() => {
  const parts: string[] = []
  if (activeTaskCount.value > 0) {
    parts.push(`${activeTaskCount.value} tasks`)
  }
  if (sessionCount.value > 0) {
    parts.push(`${sessionCount.value} tabs`)
  }
  return parts.join(' · ')
})

export function useBrowserMonitor() {
  ensureInitialized()

  return {
    isOpen,
    isCollapsed,
    activeTaskCount,
    sessionCount,
    hasLiveActivity,
    activitySummary,
    setOpen,
    setCollapsed,
    openMonitor,
    closeMonitor,
    toggleMonitor,
    setActivitySnapshot,
  }
}

export function __resetBrowserMonitorStateForTests() {
  initialized.value = false
  isOpen.value = false
  isCollapsed.value = false
  activeTaskCount.value = 0
  sessionCount.value = 0
}
