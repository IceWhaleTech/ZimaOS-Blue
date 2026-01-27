<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useToolStore } from '@/stores/tool'
import type { Tool, RemoteTool } from '@/api/tool'

const { t } = useI18n()
const toolStore = useToolStore()

// Tab state
const activeTab = ref<'installed' | 'store'>('installed')

const searchQuery = ref('')
const filterCategory = ref<string>('all')
const selectedSource = ref<string>('')

// Modal states
const showAddSourceModal = ref(false)

// New source form
const newSource = ref({
  id: '',
  name: '',
  url: '',
  type: 'custom' as 'registry' | 'github' | 'custom',
  description: '',
})

const filteredTools = computed(() => {
  let result = toolStore.tools

  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase()
    result = result.filter(
      (t) =>
        t.name.toLowerCase().includes(query) ||
        t.description?.toLowerCase().includes(query) ||
        t.author?.toLowerCase().includes(query)
    )
  }

  if (filterCategory.value !== 'all') {
    result = result.filter((t) => t.category === filterCategory.value)
  }

  // Stable sort: by name (case-insensitive), then by id
  return [...result].sort((a, b) => {
    const nameCompare = a.name.toLowerCase().localeCompare(b.name.toLowerCase())
    if (nameCompare !== 0) return nameCompare
    return a.id.localeCompare(b.id)
  })
})

const filteredRemoteTools = computed(() => {
  let result = toolStore.remoteTools

  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase()
    result = result.filter(
      (t) =>
        t.name.toLowerCase().includes(query) ||
        t.description?.toLowerCase().includes(query) ||
        t.author?.toLowerCase().includes(query)
    )
  }

  if (filterCategory.value !== 'all') {
    result = result.filter((t) => t.category === filterCategory.value)
  }

  if (selectedSource.value) {
    result = result.filter((t) => t.source_id === selectedSource.value)
  }

  // Stable sort: by name (case-insensitive), then by id
  return [...result].sort((a, b) => {
    const nameCompare = a.name.toLowerCase().localeCompare(b.name.toLowerCase())
    if (nameCompare !== 0) return nameCompare
    return a.id.localeCompare(b.id)
  })
})

const toolStats = computed(() => ({
  total: toolStore.tools.length,
  enabled: toolStore.enabledTools.length,
  available: toolStore.remoteTools.filter(t => !t.installed).length,
}))

onMounted(async () => {
  await Promise.all([
    toolStore.fetchTools(),
    toolStore.fetchSources(),
  ])
  if (toolStore.remoteTools.length === 0) {
    await toolStore.refreshSources()
  }
})

async function handleToggle(tool: Tool) {
  if (tool.enabled) {
    await toolStore.disableTool(tool.id)
  } else {
    await toolStore.enableTool(tool.id)
  }
}

async function installTool(tool: RemoteTool) {
  await toolStore.installTool(tool.id)
}

async function uninstallTool(tool: Tool) {
  await toolStore.uninstallTool(tool.id)
}

async function refreshStore() {
  await toolStore.refreshSources()
}

async function addSource() {
  if (!newSource.value.id || !newSource.value.url) return
  await toolStore.addSource(newSource.value)
  showAddSourceModal.value = false
  newSource.value = { id: '', name: '', url: '', type: 'custom', description: '' }
}

function getCategoryIcon(category?: string): string {
  const icons: Record<string, string> = {
    search: '🔍',
    system: '💻',
    development: '🛠️',
    media: '🖼️',
    integration: '🔗',
    community: '👥',
  }
  return icons[category || ''] || '🔧'
}
</script>

<template>
  <div class="tool-tab">
    <!-- Stats -->
    <div class="stats">
      <div class="stat">
        <span class="stat-value">{{ toolStats.total }}</span>
        <span class="stat-label">{{ t('extensions.stats.installed') }}</span>
      </div>
      <div class="stat">
        <span class="stat-value">{{ toolStats.enabled }}</span>
        <span class="stat-label">{{ t('extensions.stats.enabled') }}</span>
      </div>
      <div class="stat">
        <span class="stat-value">{{ toolStats.available }}</span>
        <span class="stat-label">{{ t('extensions.stats.available') }}</span>
      </div>
    </div>

    <!-- Sub Tabs -->
    <div class="sub-tabs">
      <button
        :class="['sub-tab', { active: activeTab === 'installed' }]"
        @click="activeTab = 'installed'"
      >
        {{ t('extensions.tabs.installedWithCount', { count: toolStore.tools.length }) }}
      </button>
      <button
        :class="['sub-tab', { active: activeTab === 'store' }]"
        @click="activeTab = 'store'"
      >
        {{ t('extensions.tabs.storeWithCount', { count: toolStore.remoteTools.length }) }}
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

      <select v-model="filterCategory" class="filter-select">
        <option value="all">{{ t('extensions.filters.allCategories') }}</option>
        <option v-for="cat in toolStore.categories" :key="cat" :value="cat">
          {{ cat }}
        </option>
      </select>

      <select v-if="activeTab === 'store'" v-model="selectedSource" class="filter-select">
        <option value="">{{ t('extensions.filters.allSources') }}</option>
        <option v-for="source in toolStore.sources" :key="source.id" :value="source.id">
          {{ source.name }}
        </option>
      </select>

      <div class="filter-actions">
        <button v-if="activeTab === 'store'" class="btn-refresh" @click="refreshStore" :disabled="toolStore.refreshing">
          <svg v-if="!toolStore.refreshing" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
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
        <button v-if="activeTab === 'installed'" class="btn-refresh" @click="toolStore.fetchTools()" :disabled="toolStore.loading">
          <svg v-if="!toolStore.loading" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
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
    <div v-if="toolStore.error" class="error-banner">
      {{ toolStore.error }}
      <button @click="toolStore.clearError">×</button>
    </div>

    <!-- Loading -->
    <div v-if="toolStore.loading && !filteredTools.length && !filteredRemoteTools.length" class="loading">
      <div class="spinner"></div>
      <span>{{ t('common.loading') }}</span>
    </div>

    <!-- Installed Tools Grid -->
    <div v-else-if="activeTab === 'installed'" class="items-grid">
      <div
        v-for="tool in filteredTools"
        :key="tool.id"
        :class="['item-card', { disabled: !tool.enabled }]"
      >
        <div class="item-header">
          <span class="item-icon">{{ getCategoryIcon(tool.category) }}</span>
          <div class="item-title">
            <h3 :title="tool.name">{{ tool.name }}</h3>
            <div class="item-title-meta">
              <span v-if="tool.version && tool.version !== '1.0.0' && tool.version !== 'latest'" class="item-version">v{{ tool.version }}</span>
              <span v-if="tool.category" class="category-badge">{{ tool.category }}</span>
              <span v-if="tool.builtin" class="builtin-badge">{{ t('extensions.status.builtin') }}</span>
            </div>
          </div>
        </div>

        <p class="item-description">{{ tool.description || t('extensions.noDescription') }}</p>

        <div v-if="tool.tags?.length" class="item-tags">
          <span v-for="tag in tool.tags.slice(0, 3)" :key="tag" class="tag">{{ tag }}</span>
          <span v-if="tool.tags.length > 3" class="tag more">+{{ tool.tags.length - 3 }}</span>
        </div>

        <div class="item-meta">
          <span v-if="tool.author && tool.author !== 'clawdbot' && tool.author !== 'moltbot'" class="meta-item">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2" />
              <circle cx="12" cy="7" r="4" />
            </svg>
            {{ tool.author }}
          </span>
        </div>

        <div class="item-actions">
          <label class="toggle-switch">
            <input
              type="checkbox"
              :checked="tool.enabled"
              :disabled="toolStore.loading"
              @change="handleToggle(tool)"
            />
            <span class="toggle-slider"></span>
            <span class="toggle-label">{{ tool.enabled ? t('extensions.actions.enable') : t('extensions.actions.disable') }}</span>
          </label>
          <div class="action-buttons">
            <button
              v-if="!tool.builtin"
              class="btn-icon btn-uninstall"
              :title="t('extensions.actions.uninstall')"
              @click="uninstallTool(tool)"
            >
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
              </svg>
            </button>
          </div>
        </div>
      </div>

      <div v-if="filteredTools.length === 0" class="empty-state">
        <p>{{ searchQuery || filterCategory !== 'all' ? t('extensions.empty.noMatchingTools') : t('extensions.empty.noToolsInstalled') }}</p>
      </div>
    </div>

    <!-- Store Tools Grid -->
    <div v-else class="items-grid">
      <div
        v-for="tool in filteredRemoteTools"
        :key="tool.id"
        :class="['item-card', 'store-card', { installed: tool.installed }]"
      >
        <div class="item-header">
          <span class="item-icon">{{ getCategoryIcon(tool.category) }}</span>
          <div class="item-title">
            <h3 :title="tool.name">{{ tool.name }}</h3>
            <div class="item-title-meta">
              <span v-if="tool.version && tool.version !== 'latest'" class="item-version">v{{ tool.version }}</span>
              <span v-if="tool.category" class="category-badge">{{ tool.category }}</span>
              <span class="source-badge">{{ tool.source_name }}</span>
            </div>
          </div>
        </div>

        <p class="item-description">{{ tool.description }}</p>

        <div v-if="tool.tags?.length" class="item-tags">
          <span v-for="tag in tool.tags.slice(0, 3)" :key="tag" class="tag">{{ tag }}</span>
          <span v-if="tool.tags.length > 3" class="tag more">+{{ tool.tags.length - 3 }}</span>
        </div>

        <div class="item-meta">
          <span v-if="tool.author && tool.author !== 'clawdbot' && tool.author !== 'moltbot'" class="meta-item">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2" />
              <circle cx="12" cy="7" r="4" />
            </svg>
            {{ tool.author }}
          </span>
          <span v-if="tool.stars" class="meta-item">
            <svg viewBox="0 0 24 24" fill="currentColor">
              <path d="M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z" />
            </svg>
            {{ tool.stars }}
          </span>
          <span v-if="tool.downloads" class="meta-item">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
              <polyline points="7,10 12,15 17,10" />
              <line x1="12" y1="15" x2="12" y2="3" />
            </svg>
            {{ tool.downloads }}
          </span>
        </div>

        <div class="item-actions">
          <button
            v-if="!tool.installed"
            class="btn-install"
            @click="installTool(tool)"
            :disabled="toolStore.loading"
          >
            {{ t('extensions.actions.install') }}
          </button>
          <span v-else class="installed-badge">{{ t('extensions.status.installed') }}</span>
          <a v-if="tool.homepage" :href="tool.homepage" target="_blank" class="btn-link">
            {{ t('skillStore.actions.details') }}
          </a>
        </div>
      </div>

      <div v-if="filteredRemoteTools.length === 0" class="empty-state">
        <p v-if="toolStore.refreshing">{{ t('skillStore.empty.loadingStore') }}</p>
        <p v-else>{{ t('extensions.empty.noMatchingToolsStore') }}</p>
      </div>
    </div>

    <!-- Add Source Modal -->
    <div v-if="showAddSourceModal" class="modal-overlay" @click.self="showAddSourceModal = false">
      <div class="modal">
        <div class="modal-header">
          <h2>{{ t('skillStore.modal.addSourceTitle') }}</h2>
          <button class="modal-close" @click="showAddSourceModal = false">×</button>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label>{{ t('skillStore.modal.sourceId') }}</label>
            <input v-model="newSource.id" type="text" :placeholder="t('skillStore.modal.sourceIdPlaceholder')" />
          </div>
          <div class="form-group">
            <label>{{ t('skillStore.modal.name') }}</label>
            <input v-model="newSource.name" type="text" :placeholder="t('skillStore.modal.namePlaceholder')" />
          </div>
          <div class="form-group">
            <label>{{ t('skillStore.modal.type') }}</label>
            <select v-model="newSource.type">
              <option value="custom">{{ t('skillStore.modal.typeCustom') }}</option>
              <option value="github">{{ t('skillStore.modal.typeGithub') }}</option>
              <option value="registry">{{ t('skillStore.modal.typeClawdhub') }}</option>
            </select>
          </div>
          <div class="form-group">
            <label>{{ t('skillStore.modal.url') }}</label>
            <input v-model="newSource.url" type="text" :placeholder="t('skillStore.modal.urlPlaceholder')" />
          </div>
          <div class="form-group">
            <label>{{ t('skillStore.modal.descriptionOptional') }}</label>
            <input v-model="newSource.description" type="text" :placeholder="t('skillStore.modal.descriptionPlaceholder')" />
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn-cancel" @click="showAddSourceModal = false">{{ t('skillStore.modal.cancel') }}</button>
          <button class="btn-confirm" @click="addSource">{{ t('skillStore.modal.add') }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
@import './extension-tab.css';
</style>
