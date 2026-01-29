import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type {
  CompanionSession,
  SessionEvent,
  Alert,
  Stats,
  FlowGraph,
  ListOptions,
  Platform,
  ThreatLevel,
  AlertSeverity,
} from '@/api/companion'
import { companionApi } from '@/api/companion'

const PAGE_SIZE = 50

export const useCompanionStore = defineStore('companion', () => {
  // State
  const sessions = ref<CompanionSession[]>([])
  const currentSessionId = ref<string | null>(null)
  const currentSession = ref<CompanionSession | null>(null)
  const sessionEvents = ref<SessionEvent[]>([])
  const sessionFlow = ref<FlowGraph | null>(null)
  const alerts = ref<Alert[]>([])
  const stats = ref<Stats | null>(null)

  // Loading states
  const loading = ref(false)
  const loadingSession = ref(false)
  const loadingEvents = ref(false)
  const loadingFlow = ref(false)
  const loadingAlerts = ref(false)
  const loadingStats = ref(false)

  // Pagination
  const totalSessions = ref(0)
  const totalEvents = ref(0)
  const totalAlerts = ref(0)
  const hasMoreSessions = ref(false)
  const hasMoreEvents = ref(false)
  const hasMoreAlerts = ref(false)
  const currentSessionPage = ref(0)
  const currentEventPage = ref(0)
  const currentAlertPage = ref(0)

  // Filters
  const sessionFilters = ref<ListOptions>({})
  const alertFilters = ref<{ severity?: AlertSeverity; acknowledged?: boolean }>({})

  // Error
  const error = ref<string | null>(null)

  // Real-time events from WebSocket
  const realtimeEvents = ref<SessionEvent[]>([])
  const isStreaming = ref(false)

  // Computed
  const activeSessions = computed(() =>
    sessions.value.filter((s) => s.status === 'active')
  )

  const sortedSessions = computed(() =>
    [...sessions.value].sort(
      (a, b) => new Date(b.started_at).getTime() - new Date(a.started_at).getTime()
    )
  )

  const unacknowledgedAlerts = computed(() =>
    alerts.value.filter((a) => !a.acknowledged)
  )

  const threatDistribution = computed(() => {
    const dist: Record<ThreatLevel, number> = {
      none: 0,
      low: 0,
      medium: 0,
      high: 0,
      critical: 0,
    }
    sessions.value.forEach((s) => {
      dist[s.threat_level]++
    })
    return dist
  })

  const platformDistribution = computed(() => {
    const dist: Partial<Record<Platform, number>> = {}
    sessions.value.forEach((s) => {
      dist[s.platform] = (dist[s.platform] || 0) + 1
    })
    return dist
  })

  // Actions
  async function fetchSessions(opts?: ListOptions, append = false) {
    try {
      loading.value = true
      error.value = null

      const params = {
        ...sessionFilters.value,
        ...opts,
        limit: PAGE_SIZE,
        offset: append ? sessions.value.length : 0,
      }

      const response = await companionApi.listSessions(params)
      const data = response.data

      if (append) {
        sessions.value = [...sessions.value, ...(data.sessions || [])]
      } else {
        sessions.value = data.sessions || []
        currentSessionPage.value = 0
      }

      totalSessions.value = data.total
      hasMoreSessions.value = sessions.value.length < data.total
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch sessions'
    } finally {
      loading.value = false
    }
  }

  async function loadMoreSessions() {
    if (loading.value || !hasMoreSessions.value) return
    currentSessionPage.value++
    await fetchSessions({ offset: currentSessionPage.value * PAGE_SIZE }, true)
  }

  async function fetchSession(id: string) {
    try {
      loadingSession.value = true
      error.value = null
      currentSessionId.value = id

      const response = await companionApi.getSession(id)
      currentSession.value = response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch session'
      currentSession.value = null
    } finally {
      loadingSession.value = false
    }
  }

  async function fetchSessionEvents(sessionId: string, opts?: ListOptions, append = false) {
    try {
      loadingEvents.value = true
      error.value = null

      const params = {
        ...opts,
        limit: PAGE_SIZE,
        offset: append ? sessionEvents.value.length : 0,
      }

      const response = await companionApi.getSessionEvents(sessionId, params)
      const data = response.data

      if (append) {
        sessionEvents.value = [...sessionEvents.value, ...(data.events || [])]
      } else {
        sessionEvents.value = data.events || []
        currentEventPage.value = 0
      }

      totalEvents.value = data.total
      hasMoreEvents.value = sessionEvents.value.length < data.total
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch session events'
    } finally {
      loadingEvents.value = false
    }
  }

  async function loadMoreEvents() {
    if (!currentSessionId.value || loadingEvents.value || !hasMoreEvents.value) return
    currentEventPage.value++
    await fetchSessionEvents(
      currentSessionId.value,
      { offset: currentEventPage.value * PAGE_SIZE },
      true
    )
  }

  async function fetchSessionFlow(sessionId: string) {
    try {
      loadingFlow.value = true
      error.value = null

      const response = await companionApi.getSessionFlow(sessionId)
      sessionFlow.value = response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch session flow'
      sessionFlow.value = null
    } finally {
      loadingFlow.value = false
    }
  }

  async function fetchAlerts(opts?: ListOptions & { severity?: AlertSeverity; acknowledged?: boolean }, append = false) {
    try {
      loadingAlerts.value = true
      error.value = null

      const params = {
        ...alertFilters.value,
        ...opts,
        limit: PAGE_SIZE,
        offset: append ? alerts.value.length : 0,
      }

      const response = await companionApi.listAlerts(params)
      const data = response.data

      if (append) {
        alerts.value = [...alerts.value, ...(data.alerts || [])]
      } else {
        alerts.value = data.alerts || []
        currentAlertPage.value = 0
      }

      totalAlerts.value = data.total
      hasMoreAlerts.value = alerts.value.length < data.total
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch alerts'
    } finally {
      loadingAlerts.value = false
    }
  }

  async function loadMoreAlerts() {
    if (loadingAlerts.value || !hasMoreAlerts.value) return
    currentAlertPage.value++
    await fetchAlerts({ offset: currentAlertPage.value * PAGE_SIZE }, true)
  }

  async function acknowledgeAlert(id: string) {
    try {
      const response = await companionApi.acknowledgeAlert(id)
      const updatedAlert = response.data

      // Update in local state
      const index = alerts.value.findIndex((a) => a.id === id)
      if (index !== -1) {
        alerts.value[index] = updatedAlert
      }

      return updatedAlert
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to acknowledge alert'
      throw e
    }
  }

  async function bulkAcknowledgeAlerts(ids: string[]) {
    try {
      const response = await companionApi.bulkAcknowledgeAlerts(ids)
      const count = response.data.acknowledged

      // Update local state - mark all as acknowledged
      const now = new Date().toISOString()
      ids.forEach((id) => {
        const index = alerts.value.findIndex((a) => a.id === id)
        if (index !== -1) {
          alerts.value[index] = {
            ...alerts.value[index],
            acknowledged: true,
            ackedAt: now,
          }
        }
      })

      return count
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to bulk acknowledge alerts'
      throw e
    }
  }

  async function fetchStats() {
    try {
      loadingStats.value = true
      error.value = null

      const response = await companionApi.getStats()
      stats.value = response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch stats'
    } finally {
      loadingStats.value = false
    }
  }

  async function deleteSession(id: string) {
    try {
      await companionApi.deleteSession(id)
      // Remove from local state
      sessions.value = sessions.value.filter((s) => s.id !== id)
      totalSessions.value = Math.max(0, totalSessions.value - 1)
      // Clear current session if it was deleted
      if (currentSessionId.value === id) {
        clearCurrentSession()
      }
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to delete session'
      throw e
    }
  }

  async function exportData(opts?: { format?: 'json' | 'csv'; sessionIds?: string[]; from?: string; to?: string }) {
    try {
      const response = await companionApi.exportData(opts)
      const blob = response.data as Blob
      const filename = `companion-export.${opts?.format || 'json'}`

      // Trigger download
      const url = window.URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = filename
      document.body.appendChild(link)
      link.click()
      document.body.removeChild(link)
      window.URL.revokeObjectURL(url)
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to export data'
      throw e
    }
  }

  // Real-time event handling
  function addRealtimeEvent(event: SessionEvent) {
    realtimeEvents.value.unshift(event)
    // Keep only last 100 events
    if (realtimeEvents.value.length > 100) {
      realtimeEvents.value = realtimeEvents.value.slice(0, 100)
    }

    // Update session in list if it exists
    const sessionIndex = sessions.value.findIndex((s) => s.id === event.sessionId)
    if (sessionIndex !== -1) {
      const session = sessions.value[sessionIndex]
      if (session) {
        session.event_count++
        if (event.security && event.security.threatLevel !== 'none') {
          const currentLevel = session.threat_level
          if (threatPriority(event.security.threatLevel) > threatPriority(currentLevel)) {
            session.threat_level = event.security.threatLevel
          }
        }
      }
    }

    // Update current session events if viewing
    if (currentSessionId.value === event.sessionId) {
      sessionEvents.value.unshift(event)
    }
  }

  function setStreaming(streaming: boolean) {
    isStreaming.value = streaming
  }

  function clearRealtimeEvents() {
    realtimeEvents.value = []
  }

  // Filter setters
  function setSessionFilters(filters: ListOptions) {
    sessionFilters.value = filters
  }

  function setAlertFilters(filters: { severity?: AlertSeverity; acknowledged?: boolean }) {
    alertFilters.value = filters
  }

  // Clear state
  function clearCurrentSession() {
    currentSessionId.value = null
    currentSession.value = null
    sessionEvents.value = []
    sessionFlow.value = null
    currentEventPage.value = 0
    totalEvents.value = 0
    hasMoreEvents.value = false
  }

  function clearError() {
    error.value = null
  }

  // Demo mode
  const isDemoMode = ref(false)
  let demoInterval: ReturnType<typeof setInterval> | null = null

  function generateDemoSession(): CompanionSession {
    const platforms: Platform[] = ['telegram', 'whatsapp', 'discord', 'slack', 'web', 'api']
    const statuses = ['active', 'idle', 'ended'] as const
    const threats: ThreatLevel[] = ['none', 'none', 'none', 'low', 'medium']
    const id = crypto.randomUUID()
    const now = new Date()
    const startedAt = new Date(now.getTime() - Math.random() * 3600000)

    return {
      id,
      platform: platforms[Math.floor(Math.random() * platforms.length)],
      user_id: `user-${Math.floor(Math.random() * 1000)}`,
      tenant_id: 'demo-tenant',
      status: statuses[Math.floor(Math.random() * statuses.length)],
      started_at: startedAt.toISOString(),
      ended_at: Math.random() > 0.5 ? now.toISOString() : undefined,
      duration: now.getTime() - startedAt.getTime(),
      event_count: Math.floor(Math.random() * 50) + 5,
      threat_level: threats[Math.floor(Math.random() * threats.length)],
      threat_score: Math.floor(Math.random() * 30),
      metadata: {
        message_count: Math.floor(Math.random() * 20) + 1,
        tool_call_count: Math.floor(Math.random() * 10),
        llm_call_count: Math.floor(Math.random() * 15) + 1,
        total_tokens: Math.floor(Math.random() * 5000) + 500,
      },
    }
  }

  function generateDemoEvent(sessionId: string): SessionEvent {
    const eventTypes = ['message_received', 'message_sent', 'tool_call', 'llm_request'] as const
    const eventType = eventTypes[Math.floor(Math.random() * eventTypes.length)]
    const platforms: Platform[] = ['telegram', 'whatsapp', 'discord', 'slack', 'web']

    return {
      id: crypto.randomUUID(),
      sessionId,
      timestamp: new Date().toISOString(),
      eventType,
      platform: platforms[Math.floor(Math.random() * platforms.length)],
      userId: `user-${Math.floor(Math.random() * 1000)}`,
      tenantId: 'demo-tenant',
      duration: Math.floor(Math.random() * 500) + 50,
      status: 'success',
    }
  }

  function generateDemoAlert(): Alert {
    const severities: AlertSeverity[] = ['info', 'warning', 'error', 'critical']
    const titleKeys = [
      'unusualPattern',
      'highTokenUsage',
      'promptInjection',
      'rateLimitApproaching',
      'longResponseTime',
    ]
    const severity = severities[Math.floor(Math.random() * severities.length)]
    const titleKey = titleKeys[Math.floor(Math.random() * titleKeys.length)]

    return {
      id: crypto.randomUUID(),
      sessionId: sessions.value[0]?.id || crypto.randomUUID(),
      title: titleKey, // Use i18n key, will be translated in component
      description: 'demoDescription', // Use i18n key
      severity,
      createdAt: new Date().toISOString(),
      acknowledged: false,
    }
  }

  function startDemo() {
    if (isDemoMode.value) return

    isDemoMode.value = true

    // Generate initial demo data
    const demoSessions = Array.from({ length: 5 }, () => generateDemoSession())
    sessions.value = demoSessions
    totalSessions.value = demoSessions.length

    const demoAlerts = Array.from({ length: 3 }, () => generateDemoAlert())
    alerts.value = demoAlerts
    totalAlerts.value = demoAlerts.length

    stats.value = {
      activeSessions: demoSessions.filter(s => s.status === 'active').length,
      totalSessions: demoSessions.length,
      totalEvents: demoSessions.reduce((sum, s) => sum + s.event_count, 0),
      totalAlerts: demoAlerts.length,
    }

    // Simulate real-time events
    demoInterval = setInterval(() => {
      if (!isDemoMode.value) return

      // Add random event
      const randomSession = sessions.value[Math.floor(Math.random() * sessions.value.length)]
      if (randomSession) {
        const event = generateDemoEvent(randomSession.id)
        addRealtimeEvent(event)
      }

      // Occasionally add new alert
      if (Math.random() < 0.1) {
        const alert = generateDemoAlert()
        alerts.value.unshift(alert)
        totalAlerts.value++
        if (stats.value) {
          stats.value.totalAlerts++
        }
      }

      // Update stats
      if (stats.value) {
        stats.value.totalEvents++
      }
    }, 2000)
  }

  function stopDemo() {
    isDemoMode.value = false

    if (demoInterval) {
      clearInterval(demoInterval)
      demoInterval = null
    }

    // Clear demo data
    sessions.value = []
    alerts.value = []
    realtimeEvents.value = []
    stats.value = null
    totalSessions.value = 0
    totalAlerts.value = 0
  }

  return {
    // State
    sessions,
    currentSessionId,
    currentSession,
    sessionEvents,
    sessionFlow,
    alerts,
    stats,
    realtimeEvents,
    isStreaming,

    // Loading states
    loading,
    loadingSession,
    loadingEvents,
    loadingFlow,
    loadingAlerts,
    loadingStats,

    // Pagination
    totalSessions,
    totalEvents,
    totalAlerts,
    hasMoreSessions,
    hasMoreEvents,
    hasMoreAlerts,

    // Filters
    sessionFilters,
    alertFilters,

    // Error
    error,

    // Computed
    activeSessions,
    sortedSessions,
    unacknowledgedAlerts,
    threatDistribution,
    platformDistribution,

    // Actions
    fetchSessions,
    loadMoreSessions,
    fetchSession,
    fetchSessionEvents,
    loadMoreEvents,
    fetchSessionFlow,
    fetchAlerts,
    loadMoreAlerts,
    acknowledgeAlert,
    bulkAcknowledgeAlerts,
    fetchStats,
    deleteSession,
    exportData,
    addRealtimeEvent,
    setStreaming,
    clearRealtimeEvents,
    setSessionFilters,
    setAlertFilters,
    clearCurrentSession,
    clearError,

    // Demo mode
    isDemoMode,
    startDemo,
    stopDemo,
  }
})

// Helper function
function threatPriority(level: ThreatLevel): number {
  const priorities: Record<ThreatLevel, number> = {
    none: 0,
    low: 1,
    medium: 2,
    high: 3,
    critical: 4,
  }
  return priorities[level] || 0
}
