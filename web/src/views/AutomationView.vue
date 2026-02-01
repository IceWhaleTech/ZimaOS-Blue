<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { cronApi } from '@/api/cron'
import { autoReplyApi } from '@/api/autoreply'
import { workflowApi } from '@/api/workflow'
import { sandboxApi } from '@/api/sandbox'
import * as browserApi from '@/api/browser'
import * as haApi from '@/api/homeassistant'

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
  sandbox: { supported: false, running: 0 },
})

// Loading state
const loading = ref(true)

// Fetch stats from APIs
async function fetchStats() {
  loading.value = true
  try {
    // Fetch Home Assistant stats
    try {
      const entities = await haApi.getEntities()
      stats.value.smartHome.entities = entities?.length || 0
      const automations = await haApi.getAutomations()
      stats.value.smartHome.automations = automations?.length || 0
      const scenes = await haApi.getScenes()
      stats.value.smartHome.scenes = scenes?.length || 0
    } catch {
      // HA not configured
    }

    // Fetch Cron stats
    try {
      const response = await cronApi.list()
      const jobs = response.data
      stats.value.cron.total = jobs?.length || 0
      stats.value.cron.active = jobs?.filter((j) => j.enabled)?.length || 0
    } catch {
      // Cron API error
    }

    // Fetch Auto-Reply stats
    try {
      const response = await autoReplyApi.list()
      const rules = response.data
      stats.value.autoReply.rules = rules?.length || 0
      stats.value.autoReply.enabled = rules?.filter((r) => r.enabled)?.length || 0
      stats.value.autoReply.matches = rules?.reduce((sum, r) => sum + (r.match_count || 0), 0) || 0
    } catch {
      // Auto-reply API error
    }

    // Fetch Browser Automation stats
    try {
      const tasks = await browserApi.getTasks()
      stats.value.browser.tasks = tasks?.length || 0
      stats.value.browser.running = tasks?.filter((t) => t.status === 'running')?.length || 0
      const sessions = await browserApi.getSessions()
      stats.value.browser.sessions = sessions?.length || 0
    } catch {
      // Browser API error
    }

    // Fetch Workflow stats
    try {
      const response = await workflowApi.list()
      const workflows = response.data.workflows
      stats.value.workflow.total = workflows?.length || 0
      stats.value.workflow.active = workflows?.filter((w) => w.status === 'active')?.length || 0
    } catch {
      // Workflow API error
    }

    // Fetch Sandbox stats
    try {
      const response = await sandboxApi.getInfo()
      console.log('Sandbox info response:', response.data)
      stats.value.sandbox.supported = response.data?.supported ?? false
    } catch (e) {
      console.warn('Failed to fetch sandbox info:', e)
      // Sandbox API error - keep default false
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
        {{ t('automation.title') }}
      </h1>
      <p class="mt-1 text-gray-500 dark:text-slate-400">
        {{ t('automation.subtitle') }}
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
                {{ t('automation.tabs.smartHome') }}
              </h3>
              <p class="text-sm text-gray-500 dark:text-slate-400">
                {{ t('automation.tabs.smartHomeDesc') }}
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
            <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('automation.stats.entities') }}</div>
          </div>
          <div class="text-center">
            <div class="text-2xl font-bold text-orange-500">{{ stats.smartHome.automations }}</div>
            <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('automation.stats.automations') }}</div>
          </div>
          <div class="text-center">
            <div class="text-2xl font-bold text-orange-500">{{ stats.smartHome.scenes }}</div>
            <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('automation.stats.scenes') }}</div>
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
                {{ t('automation.tabs.cron') }}
              </h3>
              <p class="text-sm text-gray-500 dark:text-slate-400">
                {{ t('automation.tabs.cronDesc') }}
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
            <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('automation.stats.totalJobs') }}</div>
          </div>
          <div class="text-center">
            <div class="text-2xl font-bold text-blue-500">{{ stats.cron.active }}</div>
            <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('automation.stats.activeJobs') }}</div>
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
                {{ t('automation.tabs.workflow') }}
              </h3>
              <p class="text-sm text-gray-500 dark:text-slate-400">
                {{ t('automation.tabs.workflowDesc') }}
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
            <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('automation.stats.workflows') }}</div>
          </div>
          <div class="text-center">
            <div class="text-2xl font-bold text-indigo-500">{{ stats.workflow.active }}</div>
            <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('automation.stats.active') }}</div>
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
                {{ t('automation.tabs.autoReply') }}
              </h3>
              <p class="text-sm text-gray-500 dark:text-slate-400">
                {{ t('automation.tabs.autoReplyDesc') }}
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
            <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('automation.stats.rules') }}</div>
          </div>
          <div class="text-center">
            <div class="text-2xl font-bold text-purple-500">{{ stats.autoReply.enabled }}</div>
            <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('automation.stats.enabled') }}</div>
          </div>
          <div class="text-center">
            <div class="text-2xl font-bold text-purple-500">{{ stats.autoReply.matches }}</div>
            <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('automation.stats.matches') }}</div>
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
                {{ t('automation.tabs.browser') }}
              </h3>
              <p class="text-sm text-gray-500 dark:text-slate-400">
                {{ t('automation.tabs.browserDesc') }}
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
            <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('automation.stats.tasks') }}</div>
          </div>
          <div class="text-center">
            <div class="text-2xl font-bold text-green-500">{{ stats.browser.running }}</div>
            <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('automation.stats.running') }}</div>
          </div>
          <div class="text-center">
            <div class="text-2xl font-bold text-green-500">{{ stats.browser.sessions }}</div>
            <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('automation.stats.sessions') }}</div>
          </div>
        </div>
      </div>

      <!-- Sandbox Card -->
      <div
        class="glass-card p-6 cursor-pointer hover:scale-[1.02] transition-all duration-200 group"
        @click="navigateToPage('/sandbox')"
      >
        <div class="flex items-start justify-between">
          <div class="flex items-center space-x-4">
            <div class="w-12 h-12 rounded-xl bg-gradient-to-br from-teal-500 to-cyan-500 flex items-center justify-center shadow-lg">
              <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4" />
              </svg>
            </div>
            <div>
              <h3 class="text-lg font-semibold text-gray-900 dark:text-white group-hover:text-teal-500 transition-colors">
                {{ t('automation.tabs.sandbox') }}
              </h3>
              <p class="text-sm text-gray-500 dark:text-slate-400">
                {{ t('automation.tabs.sandboxDesc') }}
              </p>
            </div>
          </div>
          <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-gray-400 group-hover:text-teal-500 group-hover:translate-x-1 transition-all" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
          </svg>
        </div>

        <!-- Stats -->
        <div class="mt-6 grid grid-cols-2 gap-4">
          <div class="text-center">
            <div class="text-2xl font-bold" :class="stats.sandbox.supported ? 'text-green-500' : 'text-gray-400'">
              {{ stats.sandbox.supported ? t('common.enabled') : t('common.disabled') }}
            </div>
            <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('automation.stats.status') }}</div>
          </div>
          <div class="text-center">
            <div class="text-2xl font-bold text-teal-500">{{ stats.sandbox.running }}</div>
            <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('automation.stats.running') }}</div>
          </div>
        </div>
      </div>
    </div>

    <!-- Quick Actions -->
    <div class="mt-8">
      <h2 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">
        {{ t('automation.quickActions') }}
      </h2>
      <div class="flex flex-wrap gap-3">
        <button
          class="px-4 py-2 rounded-lg bg-orange-500/10 text-orange-500 hover:bg-orange-500/20 transition-colors flex items-center space-x-2"
          @click="navigateToPage('/smart-home')"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z" />
          </svg>
          <span>{{ t('automation.actions.controlLights') }}</span>
        </button>
        <button
          class="px-4 py-2 rounded-lg bg-blue-500/10 text-blue-500 hover:bg-blue-500/20 transition-colors flex items-center space-x-2"
          @click="navigateToPage('/cron')"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v6m0 0v6m0-6h6m-6 0H6" />
          </svg>
          <span>{{ t('automation.actions.createJob') }}</span>
        </button>
        <button
          class="px-4 py-2 rounded-lg bg-indigo-500/10 text-indigo-500 hover:bg-indigo-500/20 transition-colors flex items-center space-x-2"
          @click="navigateToPage('/workflows')"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v6m0 0v6m0-6h6m-6 0H6" />
          </svg>
          <span>{{ t('automation.actions.createWorkflow') }}</span>
        </button>
        <button
          class="px-4 py-2 rounded-lg bg-purple-500/10 text-purple-500 hover:bg-purple-500/20 transition-colors flex items-center space-x-2"
          @click="navigateToPage('/auto-reply')"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v6m0 0v6m0-6h6m-6 0H6" />
          </svg>
          <span>{{ t('automation.actions.createRule') }}</span>
        </button>
        <button
          class="px-4 py-2 rounded-lg bg-green-500/10 text-green-500 hover:bg-green-500/20 transition-colors flex items-center space-x-2"
          @click="navigateToPage('/browser-automation')"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v6m0 0v6m0-6h6m-6 0H6" />
          </svg>
          <span>{{ t('automation.actions.createTask') }}</span>
        </button>
      </div>
    </div>
  </div>
</template>
