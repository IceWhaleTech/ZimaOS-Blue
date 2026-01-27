<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'

const { t } = useI18n()
const router = useRouter()

// Navigate to the specific page
function navigateToPage(route: string) {
  router.push(route)
}

// Quick stats
const stats = ref({
  smartHome: { entities: 0, automations: 0, scenes: 0 },
  cron: { total: 0, active: 0 },
  autoReply: { rules: 0, enabled: 0, matches: 0 },
  browser: { tasks: 0, running: 0, sessions: 0 },
  workflow: { total: 0, active: 0, executions: 0 },
  a2ui: { components: 0, canvases: 0 },
})

// Loading state
const loading = ref(true)

// Fetch stats from APIs
async function fetchStats() {
  loading.value = true
  try {
    // Fetch Home Assistant stats
    try {
      const haResponse = await fetch('/api/homeassistant/entities')
      if (haResponse.ok) {
        const entities = await haResponse.json()
        stats.value.smartHome.entities = entities?.length || 0
      }
      const haAutoResponse = await fetch('/api/homeassistant/automations')
      if (haAutoResponse.ok) {
        const automations = await haAutoResponse.json()
        stats.value.smartHome.automations = automations?.length || 0
      }
      const haScenesResponse = await fetch('/api/homeassistant/scenes')
      if (haScenesResponse.ok) {
        const scenes = await haScenesResponse.json()
        stats.value.smartHome.scenes = scenes?.length || 0
      }
    } catch {
      // HA not configured
    }

    // Fetch Cron stats
    try {
      const cronResponse = await fetch('/api/cron/jobs')
      if (cronResponse.ok) {
        const jobs = await cronResponse.json()
        stats.value.cron.total = jobs?.length || 0
        stats.value.cron.active = jobs?.filter((j: { enabled: boolean }) => j.enabled)?.length || 0
      }
    } catch {
      // Cron API error
    }

    // Fetch Auto-Reply stats
    try {
      const autoReplyResponse = await fetch('/api/autoreply/rules')
      if (autoReplyResponse.ok) {
        const rules = await autoReplyResponse.json()
        stats.value.autoReply.rules = rules?.length || 0
        stats.value.autoReply.enabled = rules?.filter((r: { enabled: boolean }) => r.enabled)?.length || 0
        stats.value.autoReply.matches = rules?.reduce((sum: number, r: { match_count?: number }) => sum + (r.match_count || 0), 0) || 0
      }
    } catch {
      // Auto-reply API error
    }

    // Fetch Browser Automation stats
    try {
      const browserResponse = await fetch('/api/browser/tasks')
      if (browserResponse.ok) {
        const tasks = await browserResponse.json()
        stats.value.browser.tasks = tasks?.length || 0
        stats.value.browser.running = tasks?.filter((t: { status: string }) => t.status === 'running')?.length || 0
      }
      const sessionsResponse = await fetch('/api/browser/sessions')
      if (sessionsResponse.ok) {
        const sessions = await sessionsResponse.json()
        stats.value.browser.sessions = sessions?.length || 0
      }
    } catch {
      // Browser API error
    }

    // Fetch Workflow stats
    try {
      const workflowResponse = await fetch('/api/workflows')
      if (workflowResponse.ok) {
        const workflows = await workflowResponse.json()
        stats.value.workflow.total = workflows?.length || 0
        stats.value.workflow.active = workflows?.filter((w: { enabled: boolean }) => w.enabled)?.length || 0
      }
    } catch {
      // Workflow API error
    }

    // Fetch A2UI stats
    try {
      const a2uiResponse = await fetch('/api/a2ui/canvases')
      if (a2uiResponse.ok) {
        const canvases = await a2uiResponse.json()
        stats.value.a2ui.canvases = canvases?.length || 0
      }
    } catch {
      // A2UI API error
    }
  } finally {
    loading.value = false
  }
}

// Auto refresh
let refreshInterval: ReturnType<typeof setInterval> | null = null

onMounted(() => {
  fetchStats()
  refreshInterval = setInterval(fetchStats, 30000) // Refresh every 30 seconds
})

onUnmounted(() => {
  if (refreshInterval) {
    clearInterval(refreshInterval)
  }
})
</script>

<template>
  <div class="p-6 max-w-7xl mx-auto">
    <!-- Header -->
    <div class="mb-8">
      <h1 class="text-2xl font-bold text-gray-900 dark:text-white">
        {{ t('taskCenter.title') }}
      </h1>
      <p class="mt-1 text-gray-500 dark:text-slate-400">
        {{ t('taskCenter.subtitle') }}
      </p>
    </div>

    <!-- Task Cards Grid -->
    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
      <!-- Smart Home Card -->
      <div
        class="glass-card p-6 cursor-pointer hover:scale-[1.02] transition-all duration-200 group"
        @click="navigateToPage('/smart-home')"
      >
        <div class="flex items-start justify-between">
          <div class="flex items-center space-x-4">
            <div class="w-12 h-12 rounded-xl bg-gradient-to-br from-orange-500 to-amber-500 flex items-center justify-center shadow-lg">
              <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6" />
              </svg>
            </div>
            <div>
              <h3 class="text-lg font-semibold text-gray-900 dark:text-white group-hover:text-orange-500 transition-colors">
                {{ t('taskCenter.tabs.smartHome') }}
              </h3>
              <p class="text-sm text-gray-500 dark:text-slate-400">
                {{ t('taskCenter.tabs.smartHomeDesc') }}
              </p>
            </div>
          </div>
          <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-gray-400 group-hover:text-orange-500 group-hover:translate-x-1 transition-all" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
          </svg>
        </div>

        <!-- Stats -->
        <div class="mt-6 grid grid-cols-3 gap-4">
          <div class="text-center">
            <div class="text-2xl font-bold text-orange-500">{{ stats.smartHome.entities }}</div>
            <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('taskCenter.stats.entities') }}</div>
          </div>
          <div class="text-center">
            <div class="text-2xl font-bold text-orange-500">{{ stats.smartHome.automations }}</div>
            <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('taskCenter.stats.automations') }}</div>
          </div>
          <div class="text-center">
            <div class="text-2xl font-bold text-orange-500">{{ stats.smartHome.scenes }}</div>
            <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('taskCenter.stats.scenes') }}</div>
          </div>
        </div>
      </div>

      <!-- Cron Jobs Card -->
      <div
        class="glass-card p-6 cursor-pointer hover:scale-[1.02] transition-all duration-200 group"
        @click="navigateToPage('/cron')"
      >
        <div class="flex items-start justify-between">
          <div class="flex items-center space-x-4">
            <div class="w-12 h-12 rounded-xl bg-gradient-to-br from-blue-500 to-cyan-500 flex items-center justify-center shadow-lg">
              <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            </div>
            <div>
              <h3 class="text-lg font-semibold text-gray-900 dark:text-white group-hover:text-blue-500 transition-colors">
                {{ t('taskCenter.tabs.cron') }}
              </h3>
              <p class="text-sm text-gray-500 dark:text-slate-400">
                {{ t('taskCenter.tabs.cronDesc') }}
              </p>
            </div>
          </div>
          <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-gray-400 group-hover:text-blue-500 group-hover:translate-x-1 transition-all" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
          </svg>
        </div>

        <!-- Stats -->
        <div class="mt-6 grid grid-cols-2 gap-4">
          <div class="text-center">
            <div class="text-2xl font-bold text-blue-500">{{ stats.cron.total }}</div>
            <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('taskCenter.stats.totalJobs') }}</div>
          </div>
          <div class="text-center">
            <div class="text-2xl font-bold text-blue-500">{{ stats.cron.active }}</div>
            <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('taskCenter.stats.activeJobs') }}</div>
          </div>
        </div>
      </div>

      <!-- Workflow Card -->
      <div
        class="glass-card p-6 cursor-pointer hover:scale-[1.02] transition-all duration-200 group"
        @click="navigateToPage('/workflows')"
      >
        <div class="flex items-start justify-between">
          <div class="flex items-center space-x-4">
            <div class="w-12 h-12 rounded-xl bg-gradient-to-br from-indigo-500 to-violet-500 flex items-center justify-center shadow-lg">
              <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 5a1 1 0 011-1h14a1 1 0 011 1v2a1 1 0 01-1 1H5a1 1 0 01-1-1V5zM4 13a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H5a1 1 0 01-1-1v-6zM16 13a1 1 0 011-1h2a1 1 0 011 1v6a1 1 0 01-1 1h-2a1 1 0 01-1-1v-6z" />
              </svg>
            </div>
            <div>
              <h3 class="text-lg font-semibold text-gray-900 dark:text-white group-hover:text-indigo-500 transition-colors">
                {{ t('taskCenter.tabs.workflow') }}
              </h3>
              <p class="text-sm text-gray-500 dark:text-slate-400">
                {{ t('taskCenter.tabs.workflowDesc') }}
              </p>
            </div>
          </div>
          <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-gray-400 group-hover:text-indigo-500 group-hover:translate-x-1 transition-all" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
          </svg>
        </div>

        <!-- Stats -->
        <div class="mt-6 grid grid-cols-2 gap-4">
          <div class="text-center">
            <div class="text-2xl font-bold text-indigo-500">{{ stats.workflow.total }}</div>
            <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('taskCenter.stats.workflows') }}</div>
          </div>
          <div class="text-center">
            <div class="text-2xl font-bold text-indigo-500">{{ stats.workflow.active }}</div>
            <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('taskCenter.stats.active') }}</div>
          </div>
        </div>
      </div>

      <!-- Auto-Reply Card -->
      <div
        class="glass-card p-6 cursor-pointer hover:scale-[1.02] transition-all duration-200 group"
        @click="navigateToPage('/auto-reply')"
      >
        <div class="flex items-start justify-between">
          <div class="flex items-center space-x-4">
            <div class="w-12 h-12 rounded-xl bg-gradient-to-br from-purple-500 to-pink-500 flex items-center justify-center shadow-lg">
              <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
              </svg>
            </div>
            <div>
              <h3 class="text-lg font-semibold text-gray-900 dark:text-white group-hover:text-purple-500 transition-colors">
                {{ t('taskCenter.tabs.autoReply') }}
              </h3>
              <p class="text-sm text-gray-500 dark:text-slate-400">
                {{ t('taskCenter.tabs.autoReplyDesc') }}
              </p>
            </div>
          </div>
          <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-gray-400 group-hover:text-purple-500 group-hover:translate-x-1 transition-all" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
          </svg>
        </div>

        <!-- Stats -->
        <div class="mt-6 grid grid-cols-3 gap-4">
          <div class="text-center">
            <div class="text-2xl font-bold text-purple-500">{{ stats.autoReply.rules }}</div>
            <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('taskCenter.stats.rules') }}</div>
          </div>
          <div class="text-center">
            <div class="text-2xl font-bold text-purple-500">{{ stats.autoReply.enabled }}</div>
            <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('taskCenter.stats.enabled') }}</div>
          </div>
          <div class="text-center">
            <div class="text-2xl font-bold text-purple-500">{{ stats.autoReply.matches }}</div>
            <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('taskCenter.stats.matches') }}</div>
          </div>
        </div>
      </div>

      <!-- Browser Automation Card -->
      <div
        class="glass-card p-6 cursor-pointer hover:scale-[1.02] transition-all duration-200 group"
        @click="navigateToPage('/browser-automation')"
      >
        <div class="flex items-start justify-between">
          <div class="flex items-center space-x-4">
            <div class="w-12 h-12 rounded-xl bg-gradient-to-br from-green-500 to-emerald-500 flex items-center justify-center shadow-lg">
              <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9m-9 9a9 9 0 019-9" />
              </svg>
            </div>
            <div>
              <h3 class="text-lg font-semibold text-gray-900 dark:text-white group-hover:text-green-500 transition-colors">
                {{ t('taskCenter.tabs.browser') }}
              </h3>
              <p class="text-sm text-gray-500 dark:text-slate-400">
                {{ t('taskCenter.tabs.browserDesc') }}
              </p>
            </div>
          </div>
          <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-gray-400 group-hover:text-green-500 group-hover:translate-x-1 transition-all" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
          </svg>
        </div>

        <!-- Stats -->
        <div class="mt-6 grid grid-cols-3 gap-4">
          <div class="text-center">
            <div class="text-2xl font-bold text-green-500">{{ stats.browser.tasks }}</div>
            <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('taskCenter.stats.tasks') }}</div>
          </div>
          <div class="text-center">
            <div class="text-2xl font-bold text-green-500">{{ stats.browser.running }}</div>
            <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('taskCenter.stats.running') }}</div>
          </div>
          <div class="text-center">
            <div class="text-2xl font-bold text-green-500">{{ stats.browser.sessions }}</div>
            <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('taskCenter.stats.sessions') }}</div>
          </div>
        </div>
      </div>

      <!-- A2UI Card -->
      <div
        class="glass-card p-6 cursor-pointer hover:scale-[1.02] transition-all duration-200 group"
        @click="navigateToPage('/a2ui')"
      >
        <div class="flex items-start justify-between">
          <div class="flex items-center space-x-4">
            <div class="w-12 h-12 rounded-xl bg-gradient-to-br from-rose-500 to-red-500 flex items-center justify-center shadow-lg">
              <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 5a1 1 0 011-1h14a1 1 0 011 1v2a1 1 0 01-1 1H5a1 1 0 01-1-1V5zM4 13a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H5a1 1 0 01-1-1v-6zM16 13a1 1 0 011-1h2a1 1 0 011 1v6a1 1 0 01-1 1h-2a1 1 0 01-1-1v-6z" />
              </svg>
            </div>
            <div>
              <h3 class="text-lg font-semibold text-gray-900 dark:text-white group-hover:text-rose-500 transition-colors">
                {{ t('taskCenter.tabs.a2ui') }}
              </h3>
              <p class="text-sm text-gray-500 dark:text-slate-400">
                {{ t('taskCenter.tabs.a2uiDesc') }}
              </p>
            </div>
          </div>
          <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-gray-400 group-hover:text-rose-500 group-hover:translate-x-1 transition-all" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
          </svg>
        </div>

        <!-- Stats -->
        <div class="mt-6 grid grid-cols-2 gap-4">
          <div class="text-center">
            <div class="text-2xl font-bold text-rose-500">{{ stats.a2ui.canvases }}</div>
            <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('taskCenter.stats.canvases') }}</div>
          </div>
          <div class="text-center">
            <div class="text-2xl font-bold text-rose-500">{{ stats.a2ui.components }}</div>
            <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('taskCenter.stats.components') }}</div>
          </div>
        </div>
      </div>
    </div>

    <!-- Quick Actions -->
    <div class="mt-8">
      <h2 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">
        {{ t('taskCenter.quickActions') }}
      </h2>
      <div class="flex flex-wrap gap-3">
        <button
          class="px-4 py-2 rounded-lg bg-orange-500/10 text-orange-500 hover:bg-orange-500/20 transition-colors flex items-center space-x-2"
          @click="navigateToPage('/smart-home')"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z" />
          </svg>
          <span>{{ t('taskCenter.actions.controlLights') }}</span>
        </button>
        <button
          class="px-4 py-2 rounded-lg bg-blue-500/10 text-blue-500 hover:bg-blue-500/20 transition-colors flex items-center space-x-2"
          @click="navigateToPage('/cron')"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v6m0 0v6m0-6h6m-6 0H6" />
          </svg>
          <span>{{ t('taskCenter.actions.createJob') }}</span>
        </button>
        <button
          class="px-4 py-2 rounded-lg bg-indigo-500/10 text-indigo-500 hover:bg-indigo-500/20 transition-colors flex items-center space-x-2"
          @click="navigateToPage('/workflows')"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v6m0 0v6m0-6h6m-6 0H6" />
          </svg>
          <span>{{ t('taskCenter.actions.createWorkflow') }}</span>
        </button>
        <button
          class="px-4 py-2 rounded-lg bg-purple-500/10 text-purple-500 hover:bg-purple-500/20 transition-colors flex items-center space-x-2"
          @click="navigateToPage('/auto-reply')"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v6m0 0v6m0-6h6m-6 0H6" />
          </svg>
          <span>{{ t('taskCenter.actions.createRule') }}</span>
        </button>
        <button
          class="px-4 py-2 rounded-lg bg-green-500/10 text-green-500 hover:bg-green-500/20 transition-colors flex items-center space-x-2"
          @click="navigateToPage('/browser-automation')"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v6m0 0v6m0-6h6m-6 0H6" />
          </svg>
          <span>{{ t('taskCenter.actions.createTask') }}</span>
        </button>
        <button
          class="px-4 py-2 rounded-lg bg-rose-500/10 text-rose-500 hover:bg-rose-500/20 transition-colors flex items-center space-x-2"
          @click="navigateToPage('/a2ui')"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v6m0 0v6m0-6h6m-6 0H6" />
          </svg>
          <span>{{ t('taskCenter.actions.createCanvas') }}</span>
        </button>
      </div>
    </div>
  </div>
</template>
