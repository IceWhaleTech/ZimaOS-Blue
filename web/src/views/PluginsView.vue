<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { usePluginStore } from '@/stores/plugin'
import PluginCard from '@/components/PluginCard.vue'
import PluginConfigForm from '@/components/PluginConfigForm.vue'
import type { Plugin, PluginLog } from '@/api/plugin'

const pluginStore = usePluginStore()

const searchQuery = ref('')
const filterType = ref<string>('all')
const filterStatus = ref<string>('all')

// Modal states
const showConfigModal = ref(false)
const showLogsModal = ref(false)
const selectedPlugin = ref<Plugin | null>(null)
const pluginLogs = ref<PluginLog[]>([])
const logsLoading = ref(false)
const logLevel = ref<string>('all')

const filteredPlugins = computed(() => {
  let result = pluginStore.plugins

  // Search filter
  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase()
    result = result.filter(
      (p) =>
        p.name.toLowerCase().includes(query) ||
        p.description?.toLowerCase().includes(query) ||
        p.author?.toLowerCase().includes(query)
    )
  }

  // Type filter
  if (filterType.value !== 'all') {
    result = result.filter((p) => p.type === filterType.value)
  }

  // Status filter
  if (filterStatus.value !== 'all') {
    if (filterStatus.value === 'enabled') {
      result = result.filter((p) => p.enabled)
    } else if (filterStatus.value === 'disabled') {
      result = result.filter((p) => !p.enabled)
    } else if (filterStatus.value === 'error') {
      result = result.filter((p) => p.status === 'error')
    }
  }

  return result
})

const pluginStats = computed(() => ({
  total: pluginStore.plugins.length,
  enabled: pluginStore.enabledPlugins.length,
  disabled: pluginStore.disabledPlugins.length,
  native: pluginStore.pluginsByType.native.length,
  js: pluginStore.pluginsByType.js.length,
  wasm: pluginStore.pluginsByType.wasm.length,
}))

onMounted(async () => {
  await pluginStore.fetchPlugins()
})

async function handleToggle(plugin: Plugin) {
  if (plugin.enabled) {
    await pluginStore.disablePlugin(plugin.id)
  } else {
    await pluginStore.enablePlugin(plugin.id)
  }
}

function openConfigModal(plugin: Plugin) {
  selectedPlugin.value = plugin
  showConfigModal.value = true
}

function closeConfigModal() {
  showConfigModal.value = false
  selectedPlugin.value = null
}

async function handleSaveConfig(config: Record<string, unknown>) {
  if (selectedPlugin.value) {
    const success = await pluginStore.updatePluginConfig(selectedPlugin.value.id, config)
    if (success) {
      closeConfigModal()
    }
  }
}

async function openLogsModal(plugin: Plugin) {
  selectedPlugin.value = plugin
  showLogsModal.value = true
  await fetchLogs()
}

function closeLogsModal() {
  showLogsModal.value = false
  selectedPlugin.value = null
  pluginLogs.value = []
}

async function fetchLogs() {
  if (!selectedPlugin.value) return

  logsLoading.value = true
  const params: { limit?: number; level?: string } = { limit: 100 }
  if (logLevel.value !== 'all') {
    params.level = logLevel.value
  }
  pluginLogs.value = await pluginStore.fetchPluginLogs(selectedPlugin.value.id, params)
  logsLoading.value = false
}

async function handleReload(plugin: Plugin) {
  await pluginStore.reloadPlugin(plugin.id)
}

function getLogLevelClass(level: string): string {
  switch (level) {
    case 'error':
      return 'text-red-400'
    case 'warn':
      return 'text-yellow-400'
    case 'info':
      return 'text-blue-400'
    case 'debug':
      return 'text-gray-400'
    default:
      return 'text-gray-300'
  }
}

function formatLogTime(timestamp: string): string {
  return new Date(timestamp).toLocaleTimeString()
}
</script>

<template>
  <div class="plugins-view p-6 max-w-6xl mx-auto">
    <div class="flex items-center justify-between mb-6">
      <h1 class="text-2xl font-bold text-white">Plugins</h1>
      <button
        class="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-white rounded-lg text-sm transition-colors flex items-center gap-2"
        :disabled="pluginStore.loading"
        @click="pluginStore.fetchPlugins()"
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="h-4 w-4"
          :class="{ 'animate-spin': pluginStore.loading }"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
          />
        </svg>
        Refresh
      </button>
    </div>

    <!-- Stats Cards -->
    <div class="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
      <div class="bg-gray-800 rounded-lg p-4">
        <div class="text-2xl font-bold text-white">{{ pluginStats.total }}</div>
        <div class="text-sm text-gray-400">Total Plugins</div>
      </div>
      <div class="bg-gray-800 rounded-lg p-4">
        <div class="text-2xl font-bold text-green-400">{{ pluginStats.enabled }}</div>
        <div class="text-sm text-gray-400">Enabled</div>
      </div>
      <div class="bg-gray-800 rounded-lg p-4">
        <div class="text-2xl font-bold text-gray-400">{{ pluginStats.disabled }}</div>
        <div class="text-sm text-gray-400">Disabled</div>
      </div>
      <div class="bg-gray-800 rounded-lg p-4">
        <div class="flex items-center gap-2">
          <span class="text-sm text-blue-400">{{ pluginStats.native }} Native</span>
          <span class="text-gray-600">|</span>
          <span class="text-sm text-yellow-400">{{ pluginStats.js }} JS</span>
          <span class="text-gray-600">|</span>
          <span class="text-sm text-purple-400">{{ pluginStats.wasm }} WASM</span>
        </div>
        <div class="text-sm text-gray-400 mt-1">By Type</div>
      </div>
    </div>

    <!-- Filters -->
    <div class="bg-gray-800 rounded-lg p-4 mb-6">
      <div class="flex flex-col md:flex-row gap-4">
        <!-- Search -->
        <div class="flex-1">
          <div class="relative">
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="h-5 w-5 absolute left-3 top-1/2 -translate-y-1/2 text-gray-400"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
              />
            </svg>
            <input
              v-model="searchQuery"
              type="text"
              placeholder="Search plugins..."
              class="w-full bg-gray-700 text-white rounded-lg pl-10 pr-4 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>
        </div>

        <!-- Type Filter -->
        <select
          v-model="filterType"
          class="bg-gray-700 text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500"
        >
          <option value="all">All Types</option>
          <option value="native">Native</option>
          <option value="js">JavaScript</option>
          <option value="wasm">WebAssembly</option>
        </select>

        <!-- Status Filter -->
        <select
          v-model="filterStatus"
          class="bg-gray-700 text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500"
        >
          <option value="all">All Status</option>
          <option value="enabled">Enabled</option>
          <option value="disabled">Disabled</option>
          <option value="error">Error</option>
        </select>
      </div>
    </div>

    <!-- Error Message -->
    <div
      v-if="pluginStore.error"
      class="bg-red-900/30 border border-red-800 rounded-lg p-4 mb-6 text-red-300"
    >
      {{ pluginStore.error }}
    </div>

    <!-- Plugin Grid -->
    <div v-if="filteredPlugins.length > 0" class="grid md:grid-cols-2 lg:grid-cols-3 gap-4">
      <PluginCard
        v-for="plugin in filteredPlugins"
        :key="plugin.id"
        :plugin="plugin"
        :loading="pluginStore.loading"
        @toggle="handleToggle(plugin)"
        @configure="openConfigModal(plugin)"
        @view-logs="openLogsModal(plugin)"
        @reload="handleReload(plugin)"
      />
    </div>

    <!-- Empty State -->
    <div v-else class="bg-gray-800 rounded-lg p-12 text-center">
      <svg
        xmlns="http://www.w3.org/2000/svg"
        class="h-12 w-12 mx-auto text-gray-600 mb-4"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4"
        />
      </svg>
      <h3 class="text-lg font-medium text-gray-300 mb-2">No plugins found</h3>
      <p class="text-gray-500">
        {{ searchQuery || filterType !== 'all' || filterStatus !== 'all'
          ? 'Try adjusting your filters'
          : 'No plugins are installed yet' }}
      </p>
    </div>

    <!-- Config Modal -->
    <div
      v-if="showConfigModal && selectedPlugin"
      class="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4"
      @click.self="closeConfigModal"
    >
      <div class="bg-gray-800 rounded-lg max-w-lg w-full max-h-[90vh] overflow-y-auto">
        <div class="p-6">
          <div class="flex items-center justify-between mb-4">
            <h3 class="text-lg font-semibold text-white">
              Configure {{ selectedPlugin.name }}
            </h3>
            <button
              class="p-1 text-gray-400 hover:text-white"
              @click="closeConfigModal"
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
          <PluginConfigForm
            :plugin="selectedPlugin"
            :loading="pluginStore.loading"
            @save="handleSaveConfig"
            @cancel="closeConfigModal"
          />
        </div>
      </div>
    </div>

    <!-- Logs Modal -->
    <div
      v-if="showLogsModal && selectedPlugin"
      class="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4"
      @click.self="closeLogsModal"
    >
      <div class="bg-gray-800 rounded-lg max-w-3xl w-full max-h-[90vh] flex flex-col">
        <div class="p-4 border-b border-gray-700 flex items-center justify-between">
          <h3 class="text-lg font-semibold text-white">
            {{ selectedPlugin.name }} Logs
          </h3>
          <div class="flex items-center gap-4">
            <select
              v-model="logLevel"
              class="bg-gray-700 text-white rounded px-3 py-1 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              @change="fetchLogs"
            >
              <option value="all">All Levels</option>
              <option value="error">Error</option>
              <option value="warn">Warning</option>
              <option value="info">Info</option>
              <option value="debug">Debug</option>
            </select>
            <button
              class="p-1 text-gray-400 hover:text-white"
              @click="closeLogsModal"
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
        <div class="flex-1 overflow-y-auto p-4 font-mono text-sm">
          <div v-if="logsLoading" class="text-gray-400 text-center py-8">
            Loading logs...
          </div>
          <div v-else-if="pluginLogs.length === 0" class="text-gray-400 text-center py-8">
            No logs available
          </div>
          <div v-else class="space-y-1">
            <div
              v-for="(log, index) in pluginLogs"
              :key="index"
              class="flex gap-3 py-1 hover:bg-gray-700/50 px-2 rounded"
            >
              <span class="text-gray-500 flex-shrink-0">{{ formatLogTime(log.timestamp) }}</span>
              <span
                :class="getLogLevelClass(log.level)"
                class="w-12 flex-shrink-0 uppercase text-xs font-medium"
              >
                {{ log.level }}
              </span>
              <span class="text-gray-300 break-all">{{ log.message }}</span>
            </div>
          </div>
        </div>
        <div class="p-4 border-t border-gray-700 flex justify-between">
          <button
            class="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-white rounded-lg text-sm transition-colors"
            :disabled="logsLoading"
            @click="fetchLogs"
          >
            Refresh
          </button>
          <button
            class="px-4 py-2 bg-gray-600 hover:bg-gray-500 text-white rounded-lg text-sm transition-colors"
            @click="closeLogsModal"
          >
            Close
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
