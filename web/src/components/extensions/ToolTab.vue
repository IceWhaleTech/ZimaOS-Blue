<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useToolStore } from '@/stores/tool'
import type { Tool } from '@/api/tool'

const { t, te } = useI18n()
const toolStore = useToolStore()

function getToolName(tool: Tool): string {
  const key = `tools.names.${tool.name}`
  if (te(key)) return t(key)
  return tool.name
}

function getToolDescription(tool: Tool): string {
  const key = `tools.builtin.${tool.name}.description`
  if (te(key)) return t(key)
  return tool.description || ''
}

const searchQuery = ref('')
const filterCategory = ref<string>('all')

const filteredTools = computed(() => {
  let result = toolStore.tools

  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase()
    result = result.filter(
      (tool) =>
        tool.name.toLowerCase().includes(query) ||
        tool.description?.toLowerCase().includes(query)
    )
  }

  if (filterCategory.value !== 'all') {
    result = result.filter((tool) => tool.category === filterCategory.value)
  }

  return [...result].sort((a, b) => a.name.toLowerCase().localeCompare(b.name.toLowerCase()))
})

const toolStats = computed(() => ({
  total: toolStore.tools.length,
  enabled: toolStore.enabledTools.length,
  disabled: toolStore.tools.length - toolStore.enabledTools.length,
}))

const categories = computed(() => {
  const cats = new Set<string>()
  toolStore.tools.forEach(tool => {
    if (tool.category) cats.add(tool.category)
  })
  return Array.from(cats).sort()
})

onMounted(async () => {
  await toolStore.fetchTools()
})

async function handleToggle(tool: Tool) {
  if (tool.enabled) {
    await toolStore.disableTool(tool.id)
  } else {
    await toolStore.enableTool(tool.id)
  }
}

function getToolIconUrl(tool: Tool): string | null {
  const iconAliases: Record<string, string> = {
    image: 'mediagen',
    video: 'mediagen',
    image_generate: 'mediagen',
    video_generate: 'mediagen',
  }
  const availableIcons = new Set([
    'analyze', 'browser', 'eye', 'file-read', 'file-write', 'mediagen', 'memory',
    'notifications', 'process', 'question', 'sandbox', 'schedule', 'terminal',
    'web-search', 'workflow',
  ])

  // First try to use the icon field
  if (tool.icon) {
    const normalizedIcon = iconAliases[tool.icon] || tool.icon
    if (availableIcons.has(normalizedIcon)) {
      return `/icons/tools/${normalizedIcon}.svg`
    }
  }
  // Fallback: map tool name to icon
  const iconMap: Record<string, string> = {
    exec: 'terminal',
    execute_command: 'terminal',
    browser: 'browser',
    browser_navigate: 'browser',
    browser_click: 'browser',
    browser_read: 'browser',
    browser_screenshot: 'browser',
    memory: 'memory',
    memory_search: 'memory',
    web_search: 'web-search',
    scheduler: 'schedule',
    ui_reviewer: 'eye',
    analyze: 'analyze',
    sandbox: 'sandbox',
    workflows: 'workflow',
    notifications: 'notifications',
    ask: 'question',
    mediagen: 'mediagen',
    image_generate: 'mediagen',
    video_generate: 'mediagen',
  }
  const iconName = iconMap[tool.name]
  if (iconName) {
    return `/icons/tools/${iconName}.svg`
  }
  return null
}

function getCategoryIcon(category?: string): string {
  const icons: Record<string, string> = {
    search: '🔍',
    system: '💻',
    development: '🛠️',
    media: '🖼️',
    integration: '🔗',
    community: '👥',
    ai: '🤖',
    utility: '⚙️',
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
        <span class="stat-label">{{ t('plugins.stats.total') }}</span>
      </div>
      <div class="stat enabled">
        <span class="stat-value">{{ toolStats.enabled }}</span>
        <span class="stat-label">{{ t('plugins.stats.enabled') }}</span>
      </div>
      <div class="stat disabled">
        <span class="stat-value">{{ toolStats.disabled }}</span>
        <span class="stat-label">{{ t('plugins.stats.disabled') }}</span>
      </div>
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
          :placeholder="t('plugins.searchPlaceholder')"
          class="search-input"
        />
      </div>

      <select v-model="filterCategory" class="filter-select">
        <option value="all">{{ t('plugins.allCategories') }}</option>
        <option v-for="cat in categories" :key="cat" :value="cat">
          {{ getCategoryIcon(cat) }} {{ cat }}
        </option>
      </select>

      <button class="btn-refresh" :disabled="toolStore.loading" @click="toolStore.fetchTools()">
        <svg v-if="!toolStore.loading" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M21 12a9 9 0 0 0-9-9 9.75 9.75 0 0 0-6.74 2.74L3 8" />
          <path d="M3 3v5h5" />
          <path d="M3 12a9 9 0 0 0 9 9 9.75 9.75 0 0 0 6.74-2.74L21 16" />
          <path d="M16 21h5v-5" />
        </svg>
        <span v-else class="spinner"></span>
      </button>
    </div>

    <!-- Error message -->
    <div v-if="toolStore.error" class="error-banner">
      {{ toolStore.error }}
      <button @click="toolStore.clearError">×</button>
    </div>

    <!-- Loading -->
    <div v-if="toolStore.loading && !filteredTools.length" class="loading">
      <div class="spinner"></div>
      <span>{{ t('common.loading') }}</span>
    </div>

    <!-- Tools Grid -->
    <div v-else class="items-grid">
      <div
        v-for="tool in filteredTools"
        :key="tool.id"
        :class="['item-card', { disabled: !tool.enabled }]"
      >
        <div class="item-header">
          <img v-if="getToolIconUrl(tool)" :src="getToolIconUrl(tool)!" class="item-icon-svg" :alt="getToolName(tool)" />
          <span v-else class="item-icon">{{ getCategoryIcon(tool.category) }}</span>
          <div class="item-title">
            <h3 :title="getToolName(tool)">{{ getToolName(tool) }}</h3>
            <div class="item-title-meta">
              <span v-if="tool.category" class="category-badge">{{ tool.category }}</span>
              <span class="builtin-badge">{{ t('plugins.builtin') }}</span>
            </div>
          </div>
          <label class="toggle-switch">
            <input
              type="checkbox"
              :checked="tool.enabled"
              :disabled="toolStore.loading"
              @change="handleToggle(tool)"
            />
            <span class="toggle-slider"></span>
          </label>
        </div>

        <p class="item-description">{{ getToolDescription(tool) || t('plugins.noDescription') }}</p>

        <div v-if="tool.tags?.length" class="item-tags">
          <span v-for="tag in tool.tags.slice(0, 4)" :key="tag" class="tag">{{ tag }}</span>
        </div>
      </div>

      <div v-if="filteredTools.length === 0" class="empty-state">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
          <path d="M9.75 9.75l4.5 4.5m0-4.5l-4.5 4.5M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
        <p>{{ searchQuery || filterCategory !== 'all' ? t('plugins.noMatchingTools') : t('plugins.noTools') }}</p>
      </div>
    </div>
  </div>
</template>

<style scoped>
@import './extension-tab.css';

.stats {
  display: flex;
  gap: 16px;
  margin-bottom: 24px;
  padding: 16px;
  background: var(--glass-bg, rgba(255, 255, 255, 0.05));
  border: 1px solid var(--glass-border, rgba(255, 255, 255, 0.1));
  border-radius: 12px;
}

.stat {
  flex: 1;
  text-align: center;
  padding: 8px;
  border-radius: 8px;
}

.stat.enabled {
  background: rgba(34, 197, 94, 0.1);
}

.stat.disabled {
  background: rgba(156, 163, 175, 0.1);
}

.stat-value {
  display: block;
  font-size: 24px;
  font-weight: 600;
  color: var(--color-text-primary);
}

.stat-label {
  font-size: 12px;
  color: var(--color-text-secondary);
}
</style>
