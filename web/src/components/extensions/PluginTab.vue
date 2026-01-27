<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { usePluginStore } from '@/stores/plugin'
import PluginConfigForm from '@/components/PluginConfigForm.vue'
import type { Plugin, PluginLog, RemotePlugin } from '@/api/plugin'
import { getPluginIconOrDefault } from '@/utils/channelIcons'

const { t } = useI18n()
const pluginStore = usePluginStore()

// Tab state
const activeTab = ref<'installed' | 'store'>('installed')

const searchQuery = ref('')
const filterType = ref<string>('all')
const filterStatus = ref<string>('all')
const selectedSource = ref<string>('')

// Modal states
const showConfigModal = ref(false)
const showLogsModal = ref(false)
const showAddSourceModal = ref(false)
const selectedPlugin = ref<Plugin | null>(null)
const pluginLogs = ref<PluginLog[]>([])
const logsLoading = ref(false)
const logLevel = ref<string>('all')

// New source form
const newSource = ref({
  id: '',
  name: '',
  url: '',
  type: 'custom' as 'registry' | 'github' | 'custom',
  description: '',
})

const filteredPlugins = computed(() => {
  let result = pluginStore.plugins

  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase()
    result = result.filter(
      (p) =>
        p.name.toLowerCase().includes(query) ||
        p.description?.toLowerCase().includes(query) ||
        p.author?.toLowerCase().includes(query)
    )
  }

  if (filterType.value !== 'all') {
    result = result.filter((p) => p.type === filterType.value)
  }

  if (filterStatus.value !== 'all') {
    if (filterStatus.value === 'enabled') {
      result = result.filter((p) => p.enabled)
    } else if (filterStatus.value === 'disabled') {
      result = result.filter((p) => !p.enabled)
    } else if (filterStatus.value === 'error') {
      result = result.filter((p) => p.status === 'error')
    }
  }

  // Stable sort: by name (case-insensitive), then by id
  return [...result].sort((a, b) => {
    const nameCompare = a.name.toLowerCase().localeCompare(b.name.toLowerCase())
    if (nameCompare !== 0) return nameCompare
    return a.id.localeCompare(b.id)
  })
})

const filteredRemotePlugins = computed(() => {
  let result = pluginStore.remotePlugins

  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase()
    result = result.filter(
      (p) =>
        p.name.toLowerCase().includes(query) ||
        p.description?.toLowerCase().includes(query) ||
        p.author?.toLowerCase().includes(query)
    )
  }

  if (filterType.value !== 'all') {
    result = result.filter((p) => p.type === filterType.value)
  }

  if (selectedSource.value) {
    result = result.filter((p) => p.source_id === selectedSource.value)
  }

  // Stable sort: by name (case-insensitive), then by id
  return [...result].sort((a, b) => {
    const nameCompare = a.name.toLowerCase().localeCompare(b.name.toLowerCase())
    if (nameCompare !== 0) return nameCompare
    return a.id.localeCompare(b.id)
  })
})

const pluginStats = computed(() => ({
  total: pluginStore.plugins.length,
  enabled: pluginStore.enabledPlugins.length,
  available: pluginStore.availablePlugins.length,
}))

onMounted(async () => {
  await Promise.all([
    pluginStore.fetchPlugins(),
    pluginStore.fetchSources(),
  ])
  if (pluginStore.remotePlugins.length === 0) {
    await pluginStore.refreshSources()
  }
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

async function installPlugin(plugin: RemotePlugin) {
  await pluginStore.installPlugin(plugin.id)
}

async function uninstallPlugin(plugin: Plugin) {
  await pluginStore.uninstallPlugin(plugin.id)
}

async function refreshStore() {
  await pluginStore.refreshSources()
}

async function addSource() {
  if (!newSource.value.id || !newSource.value.url) return
  await pluginStore.addSource(newSource.value)
  showAddSourceModal.value = false
  newSource.value = { id: '', name: '', url: '', type: 'custom', description: '' }
}

function getTypeIcon(type: string): string {
  const icons: Record<string, string> = {
    native: '🔧',
    js: '📜',
    wasm: '⚡',
  }
  return icons[type] || '📦'
}

function getPluginIconSrc(plugin: Plugin | RemotePlugin): string | null {
  const icon = getPluginIconOrDefault({ id: plugin.id, name: plugin.name })
  // Return null if it's the default icon (we'll show emoji instead)
  return icon !== '/icons/channels/default.svg' ? icon : null
}

function getTypeLabel(type: string): string {
  const labels: Record<string, string> = {
    native: 'Native',
    js: 'JavaScript',
    wasm: 'WebAssembly',
  }
  return labels[type] || type
}

function getStatusColor(status: string): string {
  switch (status) {
    case 'running': return 'status-running'
    case 'loaded': return 'status-loaded'
    case 'stopped': return 'status-stopped'
    case 'error': return 'status-error'
    default: return 'status-stopped'
  }
}

function getStatusText(status: string): string {
  switch (status) {
    case 'running': return t('extensions.status.running')
    case 'loaded': return t('extensions.status.loaded')
    case 'stopped': return t('extensions.status.stopped')
    case 'error': return t('extensions.status.error')
    default: return t('extensions.status.unknown')
  }
}

function getLogLevelClass(level: string): string {
  switch (level) {
    case 'error': return 'text-red-400'
    case 'warn': return 'text-yellow-400'
    case 'info': return 'text-blue-400'
    case 'debug': return 'text-gray-400'
    default: return 'text-gray-300'
  }
}

function formatLogTime(timestamp: string): string {
  return new Date(timestamp).toLocaleTimeString([], {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}
</script>

<template>
  <div class="plugin-tab">
    <!-- Stats -->
    <div class="stats">
      <div class="stat">
        <span class="stat-value">{{ pluginStats.total }}</span>
        <span class="stat-label">{{ t('extensions.stats.installed') }}</span>
      </div>
      <div class="stat">
        <span class="stat-value">{{ pluginStats.enabled }}</span>
        <span class="stat-label">{{ t('extensions.stats.enabled') }}</span>
      </div>
      <div class="stat">
        <span class="stat-value">{{ pluginStats.available }}</span>
        <span class="stat-label">{{ t('extensions.stats.available') }}</span>
      </div>
    </div>

    <!-- Sub Tabs -->
    <div class="sub-tabs">
      <button
        :class="['sub-tab', { active: activeTab === 'installed' }]"
        @click="activeTab = 'installed'"
      >
        {{ t('extensions.tabs.installedWithCount', { count: pluginStore.plugins.length }) }}
      </button>
      <button
        :class="['sub-tab', { active: activeTab === 'store' }]"
        @click="activeTab = 'store'"
      >
        {{ t('extensions.tabs.storeWithCount', { count: pluginStore.remotePlugins.length }) }}
      </button>
    </div>

    <!-- Filters -->
    <div class="filters">
      <div class="search-box">
        <svg class="search-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="11" cy="11" r="8" />
          <path d="m21 21-4.35-4.35" />
        </svg>
        <input
          v-model="searchQuery"
          type="text"
          :placeholder="t('extensions.filters.searchPlaceholder')"
          class="search-input"
        />
      </div>

      <select v-model="filterType" class="filter-select">
        <option value="all">{{ t('extensions.filters.allTypes') }}</option>
        <option value="native">Native</option>
        <option value="js">JavaScript</option>
        <option value="wasm">WebAssembly</option>
      </select>

      <select v-if="activeTab === 'installed'" v-model="filterStatus" class="filter-select">
        <option value="all">{{ t('extensions.filters.allStatus') }}</option>
        <option value="enabled">{{ t('extensions.filters.enabled') }}</option>
        <option value="disabled">{{ t('extensions.filters.disabled') }}</option>
        <option value="error">{{ t('plugins.error') }}</option>
      </select>

      <select v-if="activeTab === 'store'" v-model="selectedSource" class="filter-select">
        <option value="">{{ t('extensions.filters.allSources') }}</option>
        <option v-for="source in pluginStore.sources" :key="source.id" :value="source.id">
          {{ source.name }}
        </option>
      </select>

      <div class="filter-actions">
        <button v-if="activeTab === 'store'" class="btn-refresh" @click="refreshStore" :disabled="pluginStore.refreshing">
          <svg v-if="!pluginStore.refreshing" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M21 12a9 9 0 0 0-9-9 9.75 9.75 0 0 0-6.74 2.74L3 8" />
            <path d="M3 3v5h5" />
            <path d="M3 12a9 9 0 0 0 9 9 9.75 9.75 0 0 0 6.74-2.74L21 16" />
            <path d="M16 21h5v-5" />
          </svg>
          <span v-else class="spinner"></span>
          {{ t('extensions.actions.refresh') }}
        </button>
        <button v-if="activeTab === 'store'" class="btn-add-source" @click="showAddSourceModal = true">
          + {{ t('skillStore.actions.addSource') }}
        </button>
        <button v-if="activeTab === 'installed'" class="btn-refresh" @click="pluginStore.fetchPlugins()" :disabled="pluginStore.loading">
          <svg v-if="!pluginStore.loading" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M21 12a9 9 0 0 0-9-9 9.75 9.75 0 0 0-6.74 2.74L3 8" />
            <path d="M3 3v5h5" />
            <path d="M3 12a9 9 0 0 0 9 9 9.75 9.75 0 0 0 6.74-2.74L21 16" />
            <path d="M16 21h5v-5" />
          </svg>
          <span v-else class="spinner"></span>
          {{ t('extensions.actions.refresh') }}
        </button>
      </div>
    </div>

    <!-- Error message -->
    <div v-if="pluginStore.error" class="error-banner">
      {{ pluginStore.error }}
      <button @click="pluginStore.clearError">×</button>
    </div>

    <!-- Loading -->
    <div v-if="pluginStore.loading && !filteredPlugins.length && !filteredRemotePlugins.length" class="loading">
      <div class="spinner"></div>
      <span>{{ t('common.loading') }}</span>
    </div>

    <!-- Installed Plugins Grid -->
    <div v-else-if="activeTab === 'installed'" class="items-grid">
      <div
        v-for="plugin in filteredPlugins"
        :key="plugin.id"
        :class="['item-card', { disabled: !plugin.enabled, error: plugin.status === 'error' }]"
      >
        <div class="item-header">
          <img v-if="getPluginIconSrc(plugin)" :src="getPluginIconSrc(plugin)" :alt="plugin.name" class="item-icon-img" />
          <span v-else class="item-icon">{{ getTypeIcon(plugin.type) }}</span>
          <div class="item-title">
            <h3 :title="plugin.name">{{ plugin.name }}</h3>
            <div class="item-title-meta">
              <span v-if="plugin.version && plugin.version !== '1.0.0' && plugin.version !== 'latest'" class="item-version">v{{ plugin.version }}</span>
              <span class="type-badge">{{ getTypeLabel(plugin.type) }}</span>
              <span :class="['status-badge', getStatusColor(plugin.status)]">
                {{ getStatusText(plugin.status) }}
              </span>
            </div>
          </div>
        </div>

        <p class="item-description">{{ plugin.description || t('extensions.noDescription') }}</p>

        <div v-if="plugin.error" class="item-error">
          {{ plugin.error }}
        </div>

        <div v-if="plugin.capabilities?.length" class="item-capabilities">
          <span v-for="cap in plugin.capabilities.slice(0, 3)" :key="cap" class="capability">{{ cap }}</span>
          <span v-if="plugin.capabilities.length > 3" class="capability more">+{{ plugin.capabilities.length - 3 }}</span>
        </div>

        <div class="item-meta">
          <span v-if="plugin.author && plugin.author !== 'clawdbot' && plugin.author !== 'moltbot'" class="meta-item">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2" />
              <circle cx="12" cy="7" r="4" />
            </svg>
            {{ plugin.author }}
          </span>
        </div>

        <div class="item-actions">
          <label class="toggle-switch">
            <input
              type="checkbox"
              :checked="plugin.enabled"
              :disabled="pluginStore.loading"
              @change="handleToggle(plugin)"
            />
            <span class="toggle-slider"></span>
            <span class="toggle-label">{{ plugin.enabled ? t('extensions.actions.enable') : t('extensions.actions.disable') }}</span>
          </label>
          <div class="action-buttons">
            <button
              v-if="plugin.config_schema"
              class="btn-icon"
              :title="t('extensions.actions.configure')"
              @click="openConfigModal(plugin)"
            >
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
                <circle cx="12" cy="12" r="3" />
              </svg>
            </button>
            <button class="btn-icon" :title="t('extensions.actions.viewLogs')" @click="openLogsModal(plugin)">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
              </svg>
            </button>
            <button class="btn-icon" :title="t('extensions.actions.reload')" :disabled="pluginStore.loading" @click="handleReload(plugin)">
              <svg :class="{ 'animate-spin': pluginStore.loading }" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
              </svg>
            </button>
            <button class="btn-icon btn-uninstall" :title="t('extensions.actions.uninstall')" @click="uninstallPlugin(plugin)">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
              </svg>
            </button>
          </div>
        </div>
      </div>

      <div v-if="filteredPlugins.length === 0" class="empty-state">
        <p>
          {{
            searchQuery || filterType !== 'all' || filterStatus !== 'all'
              ? t('extensions.empty.noMatchingPlugins')
              : t('extensions.empty.noPluginsInstalled')
          }}
        </p>
      </div>
    </div>

    <!-- Store Plugins Grid -->
    <div v-else class="items-grid">
      <div
        v-for="plugin in filteredRemotePlugins"
        :key="plugin.id"
        :class="['item-card', 'store-card', { installed: plugin.installed }]"
      >
        <div class="item-header">
          <img v-if="getPluginIconSrc(plugin)" :src="getPluginIconSrc(plugin)" :alt="plugin.name" class="item-icon-img" />
          <span v-else class="item-icon">{{ getTypeIcon(plugin.type) }}</span>
          <div class="item-title">
            <h3 :title="plugin.name">{{ plugin.name }}</h3>
            <div class="item-title-meta">
              <span v-if="plugin.version && plugin.version !== '1.0.0' && plugin.version !== 'latest'" class="item-version">v{{ plugin.version }}</span>
              <span class="type-badge">{{ getTypeLabel(plugin.type) }}</span>
              <span class="source-badge">{{ plugin.source_name }}</span>
            </div>
          </div>
        </div>

        <p class="item-description">{{ plugin.description }}</p>

        <div v-if="plugin.capabilities?.length" class="item-capabilities">
          <span v-for="cap in plugin.capabilities.slice(0, 3)" :key="cap" class="capability">{{ cap }}</span>
          <span v-if="plugin.capabilities.length > 3" class="capability more">+{{ plugin.capabilities.length - 3 }}</span>
        </div>

        <div class="item-meta">
          <span v-if="plugin.author && plugin.author !== 'clawdbot' && plugin.author !== 'moltbot'" class="meta-item">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2" />
              <circle cx="12" cy="7" r="4" />
            </svg>
            {{ plugin.author }}
          </span>
          <span v-if="plugin.stars" class="meta-item">
            <svg viewBox="0 0 24 24" fill="currentColor">
              <path d="M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z" />
            </svg>
            {{ plugin.stars }}
          </span>
          <span v-if="plugin.downloads" class="meta-item">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
              <polyline points="7,10 12,15 17,10" />
              <line x1="12" y1="15" x2="12" y2="3" />
            </svg>
            {{ plugin.downloads }}
          </span>
        </div>

        <div class="item-actions">
          <button
            v-if="!plugin.installed"
            class="btn-install"
            @click="installPlugin(plugin)"
            :disabled="pluginStore.loading"
          >
            {{ t('extensions.actions.install') }}
          </button>
          <span v-else class="installed-badge">{{ t('extensions.status.installed') }}</span>
          <a v-if="plugin.homepage" :href="plugin.homepage" target="_blank" class="btn-link">
            {{ t('extensions.actions.details') }}
          </a>
        </div>
      </div>

      <div v-if="filteredRemotePlugins.length === 0" class="empty-state">
        <p v-if="pluginStore.refreshing">{{ t('extensions.empty.loadingStore') }}</p>
        <p v-else>{{ t('extensions.empty.noMatchingPluginsStore') }}</p>
      </div>
    </div>

    <!-- Add Source Modal -->
    <div v-if="showAddSourceModal" class="modal-overlay" @click.self="showAddSourceModal = false">
      <div class="modal">
        <div class="modal-header">
          <h2>{{ t('extensions.modal.addPluginSourceTitle') }}</h2>
          <button class="modal-close" @click="showAddSourceModal = false">×</button>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label>{{ t('extensions.modal.sourceId') }}</label>
            <input v-model="newSource.id" type="text" :placeholder="t('extensions.modal.sourceIdPlaceholder')" />
          </div>
          <div class="form-group">
            <label>{{ t('extensions.modal.name') }}</label>
            <input v-model="newSource.name" type="text" :placeholder="t('extensions.modal.namePlaceholder')" />
          </div>
          <div class="form-group">
            <label>{{ t('extensions.modal.type') }}</label>
            <select v-model="newSource.type">
              <option value="custom">{{ t('extensions.modal.typeCustom') }}</option>
              <option value="github">{{ t('extensions.modal.typeGithub') }}</option>
              <option value="registry">{{ t('extensions.modal.typeRegistry') }}</option>
            </select>
          </div>
          <div class="form-group">
            <label>{{ t('extensions.modal.url') }}</label>
            <input v-model="newSource.url" type="text" :placeholder="t('extensions.modal.urlPlaceholder')" />
          </div>
          <div class="form-group">
            <label>{{ t('extensions.modal.descriptionOptional') }}</label>
            <input v-model="newSource.description" type="text" :placeholder="t('extensions.modal.descriptionPlaceholder')" />
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn-cancel" @click="showAddSourceModal = false">{{ t('extensions.modal.cancel') }}</button>
          <button class="btn-confirm" @click="addSource">{{ t('extensions.modal.add') }}</button>
        </div>
      </div>
    </div>

    <!-- Config Modal -->
    <div
      v-if="showConfigModal && selectedPlugin"
      class="modal-overlay"
      @click.self="closeConfigModal"
    >
      <div class="modal config-modal">
        <div class="modal-header">
          <h2>{{ t('extensions.modal.configure', { name: selectedPlugin.name }) }}</h2>
          <button class="modal-close" @click="closeConfigModal">×</button>
        </div>
        <div class="modal-body">
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
      class="modal-overlay"
      @click.self="closeLogsModal"
    >
      <div class="modal logs-modal">
        <div class="modal-header">
          <h2>{{ t('extensions.modal.logs', { name: selectedPlugin.name }) }}</h2>
          <div class="logs-header-actions">
            <select v-model="logLevel" class="filter-select" @change="fetchLogs">
              <option value="all">{{ t('extensions.modal.allLevels') }}</option>
              <option value="error">{{ t('extensions.modal.error') }}</option>
              <option value="warn">{{ t('extensions.modal.warning') }}</option>
              <option value="info">{{ t('extensions.modal.info') }}</option>
              <option value="debug">{{ t('extensions.modal.debug') }}</option>
            </select>
            <button class="modal-close" @click="closeLogsModal">×</button>
          </div>
        </div>
        <div class="logs-body">
          <div v-if="logsLoading" class="loading">
            <div class="spinner"></div>
            <span>{{ t('extensions.empty.loadingLogs') }}</span>
          </div>
          <div v-else-if="pluginLogs.length === 0" class="empty-state">
            <p>{{ t('extensions.empty.noLogs') }}</p>
          </div>
          <div v-else class="logs-list">
            <div
              v-for="(log, index) in pluginLogs"
              :key="index"
              class="log-entry"
            >
              <span class="log-time">{{ formatLogTime(log.timestamp) }}</span>
              <span :class="['log-level', getLogLevelClass(log.level)]">{{ log.level.toUpperCase() }}</span>
              <span class="log-message">{{ log.message }}</span>
            </div>
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn-refresh" @click="fetchLogs" :disabled="logsLoading">{{ t('extensions.modal.refresh') }}</button>
          <button class="btn-cancel" @click="closeLogsModal">{{ t('extensions.modal.close') }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
@import './extension-tab.css';

.config-modal {
  max-width: 600px;
}

.logs-modal {
  max-width: 800px;
  max-height: 80vh;
  display: flex;
  flex-direction: column;
}

.logs-header-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.logs-body {
  flex: 1;
  overflow-y: auto;
  padding: 16px 20px;
  font-family: monospace;
  font-size: 12px;
  background: rgba(0, 0, 0, 0.2);
}

.logs-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.log-entry {
  display: flex;
  gap: 12px;
  padding: 4px 8px;
  border-radius: 4px;
}

.log-entry:hover {
  background: rgba(255, 255, 255, 0.05);
}

.log-time {
  color: var(--text-muted);
  flex-shrink: 0;
}

.log-level {
  width: 50px;
  flex-shrink: 0;
  font-weight: 600;
  text-transform: uppercase;
}

.log-message {
  color: var(--text-secondary);
  word-break: break-all;
}

.text-red-400 { color: #f87171; }
.text-yellow-400 { color: #facc15; }
.text-blue-400 { color: #60a5fa; }
.text-gray-400 { color: #9ca3af; }
.text-gray-300 { color: #d1d5db; }
</style>
