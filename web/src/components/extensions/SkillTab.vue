<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useSkillStore } from '@/stores/skill'
import type { Skill, RemoteSkill, SkillSource } from '@/api/skill'

const { t, te } = useI18n()
const skillStore = useSkillStore()

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
  type: 'custom' as 'clawdhub' | 'github' | 'custom',
  description: '',
})

const filteredSkills = computed(() => {
  let result = skillStore.skills

  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase()
    result = result.filter(
      (s) =>
        s.name.toLowerCase().includes(query) ||
        s.description?.toLowerCase().includes(query) ||
        s.author?.toLowerCase().includes(query)
    )
  }

  if (filterCategory.value !== 'all') {
    result = result.filter((s) => s.category === filterCategory.value)
  }

  // Stable sort: by name (case-insensitive), then by id
  return [...result].sort((a, b) => {
    const nameCompare = a.name.toLowerCase().localeCompare(b.name.toLowerCase())
    if (nameCompare !== 0) return nameCompare
    return a.id.localeCompare(b.id)
  })
})

const filteredRemoteSkills = computed(() => {
  let result = skillStore.remoteSkills

  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase()
    result = result.filter(
      (s) =>
        s.name.toLowerCase().includes(query) ||
        s.description?.toLowerCase().includes(query) ||
        s.author?.toLowerCase().includes(query)
    )
  }

  if (filterCategory.value !== 'all') {
    result = result.filter((s) => s.category === filterCategory.value)
  }

  if (selectedSource.value) {
    result = result.filter((s) => s.source_id === selectedSource.value)
  }

  // Stable sort: by name (case-insensitive), then by id
  return [...result].sort((a, b) => {
    const nameCompare = a.name.toLowerCase().localeCompare(b.name.toLowerCase())
    if (nameCompare !== 0) return nameCompare
    return a.id.localeCompare(b.id)
  })
})

const skillStats = computed(() => ({
  total: skillStore.skills.length,
  enabled: skillStore.enabledSkills.length,
  available: skillStore.remoteSkills.filter(s => !s.installed).length,
}))

onMounted(async () => {
  await Promise.all([
    skillStore.fetchSkills(),
    skillStore.fetchSources(),
  ])
  if (skillStore.remoteSkills.length === 0) {
    await skillStore.refreshSources()
  }
})

async function handleToggle(skill: Skill) {
  if (skill.enabled) {
    await skillStore.disableSkill(skill.id)
  } else {
    await skillStore.enableSkill(skill.id)
  }
}

async function installSkill(skill: RemoteSkill) {
  await skillStore.installSkill(skill.id)
}

async function uninstallSkill(skill: Skill) {
  await skillStore.uninstallSkill(skill.id)
}

async function refreshStore() {
  await skillStore.refreshSources()
}

async function addSource() {
  if (!newSource.value.id || !newSource.value.url) return
  await skillStore.addSource(newSource.value)
  showAddSourceModal.value = false
  newSource.value = { id: '', name: '', url: '', type: 'custom', description: '' }
}

function getCategoryIcon(category?: string): string {
  const icons: Record<string, string> = {
    integration: '🔗',
    productivity: '📊',
    development: '💻',
    analytics: '📈',
    extension: '🧩',
  }
  return icons[category || ''] || '⚡'
}

function getCategoryLabel(category?: string): string {
  const cat = category || 'other'
  const key = `skillStore.categories.${cat}`
  if (te(key)) return t(key)
  return category || t('skillStore.categories.other')
}
</script>

<template>
  <div class="skill-tab">
    <!-- Stats -->
    <div class="stats">
      <div class="stat">
        <span class="stat-value">{{ skillStats.total }}</span>
        <span class="stat-label">{{ t('skillStore.stats.installed') }}</span>
      </div>
      <div class="stat">
        <span class="stat-value">{{ skillStats.enabled }}</span>
        <span class="stat-label">{{ t('skillStore.stats.enabled') }}</span>
      </div>
      <div class="stat">
        <span class="stat-value">{{ skillStats.available }}</span>
        <span class="stat-label">{{ t('skillStore.stats.available') }}</span>
      </div>
    </div>

    <!-- Sub Tabs -->
    <div class="sub-tabs">
      <button
        :class="['sub-tab', { active: activeTab === 'installed' }]"
        @click="activeTab = 'installed'"
      >
        {{ t('skillStore.tabs.installedWithCount', { count: skillStore.skills.length }) }}
      </button>
      <button
        :class="['sub-tab', { active: activeTab === 'store' }]"
        @click="activeTab = 'store'"
      >
        {{ t('skillStore.tabs.storeWithCount', { count: skillStore.remoteSkills.length }) }}
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
          :placeholder="t('skillStore.filters.searchSkillsPlaceholder')"
          class="search-input"
        />
      </div>

      <select v-model="filterCategory" class="filter-select">
        <option value="all">{{ t('skillStore.filters.allCategories') }}</option>
        <option v-for="cat in skillStore.categories" :key="cat" :value="cat">
          {{ getCategoryLabel(cat) }}
        </option>
      </select>

      <select v-if="activeTab === 'store'" v-model="selectedSource" class="filter-select">
        <option value="">{{ t('skillStore.filters.allSources') }}</option>
        <option v-for="source in skillStore.sources" :key="source.id" :value="source.id">
          {{ source.name }}
        </option>
      </select>

      <div class="filter-actions">
        <button v-if="activeTab === 'store'" class="btn-refresh" @click="refreshStore" :disabled="skillStore.refreshing">
          <svg v-if="!skillStore.refreshing" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M21 12a9 9 0 0 0-9-9 9.75 9.75 0 0 0-6.74 2.74L3 8" />
            <path d="M3 3v5h5" />
            <path d="M3 12a9 9 0 0 0 9 9 9.75 9.75 0 0 0 6.74-2.74L21 16" />
            <path d="M16 21h5v-5" />
          </svg>
          <span v-else class="spinner"></span>
          {{ t('skillStore.actions.refresh') }}
        </button>
        <button v-if="activeTab === 'store'" class="btn-add-source" @click="showAddSourceModal = true">
          + {{ t('skillStore.actions.addSource') }}
        </button>
        <button v-if="activeTab === 'installed'" class="btn-refresh" @click="skillStore.fetchSkills()" :disabled="skillStore.loading">
          <svg v-if="!skillStore.loading" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M21 12a9 9 0 0 0-9-9 9.75 9.75 0 0 0-6.74 2.74L3 8" />
            <path d="M3 3v5h5" />
            <path d="M3 12a9 9 0 0 0 9 9 9.75 9.75 0 0 0 6.74-2.74L21 16" />
            <path d="M16 21h5v-5" />
          </svg>
          <span v-else class="spinner"></span>
          {{ t('skillStore.actions.refresh') }}
        </button>
      </div>
    </div>

    <!-- Error message -->
    <div v-if="skillStore.error" class="error-banner">
      {{ skillStore.error }}
      <button @click="skillStore.clearError">×</button>
    </div>

    <!-- Loading -->
    <div v-if="skillStore.loading && !filteredSkills.length && !filteredRemoteSkills.length" class="loading">
      <div class="spinner"></div>
      <span>{{ t('common.loading') }}</span>
    </div>

    <!-- Installed Skills Grid -->
    <div v-else-if="activeTab === 'installed'" class="items-grid">
      <div
        v-for="skill in filteredSkills"
        :key="skill.id"
        :class="['item-card', { disabled: !skill.enabled }]"
      >
        <div class="item-header">
          <span class="item-icon">{{ getCategoryIcon(skill.category) }}</span>
          <div class="item-title">
            <h3 :title="skill.name">{{ skill.name }}</h3>
            <div class="item-title-meta">
              <span v-if="skill.version && skill.version !== '1.0.0' && skill.version !== 'latest'" class="item-version">v{{ skill.version }}</span>
              <span v-if="skill.category" class="category-badge">{{ getCategoryLabel(skill.category) }}</span>
              <span v-if="skill.builtin" class="builtin-badge">{{ t('skillStore.status.builtin') }}</span>
            </div>
          </div>
        </div>

        <p class="item-description">{{ skill.description || t('plugins.noDescription') }}</p>

        <div v-if="skill.tags?.length" class="item-tags">
          <span v-for="tag in skill.tags.slice(0, 3)" :key="tag" class="tag">{{ tag }}</span>
          <span v-if="skill.tags.length > 3" class="tag more">+{{ skill.tags.length - 3 }}</span>
        </div>

        <div class="item-meta">
          <span v-if="skill.author && skill.author !== 'clawdbot' && skill.author !== 'moltbot'" class="meta-item">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2" />
              <circle cx="12" cy="7" r="4" />
            </svg>
            {{ skill.author }}
          </span>
        </div>

        <div class="item-actions">
          <label class="toggle-switch">
            <input
              type="checkbox"
              :checked="skill.enabled"
              :disabled="skillStore.loading"
              @change="handleToggle(skill)"
            />
            <span class="toggle-slider"></span>
            <span class="toggle-label">{{ skill.enabled ? t('skillStore.actions.enable') : t('skillStore.actions.disable') }}</span>
          </label>
          <div class="action-buttons">
            <button
              v-if="!skill.builtin"
              class="btn-icon btn-uninstall"
              :title="t('skillStore.actions.uninstall')"
              @click="uninstallSkill(skill)"
            >
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
              </svg>
            </button>
          </div>
        </div>
      </div>

      <div v-if="filteredSkills.length === 0" class="empty-state">
        <p>
          {{
            searchQuery || filterCategory !== 'all'
              ? t('skillStore.empty.noMatchingInstalled')
              : t('skillStore.empty.noInstalledYet')
          }}
        </p>
      </div>
    </div>

    <!-- Store Skills Grid -->
    <div v-else class="items-grid">
      <div
        v-for="skill in filteredRemoteSkills"
        :key="skill.id"
        :class="['item-card', 'store-card', { installed: skill.installed }]"
      >
        <div class="item-header">
          <span class="item-icon">{{ getCategoryIcon(skill.category) }}</span>
          <div class="item-title">
            <h3 :title="skill.name">{{ skill.name }}</h3>
            <div class="item-title-meta">
              <span v-if="skill.version && skill.version !== 'latest'" class="item-version">v{{ skill.version }}</span>
              <span v-if="skill.category" class="category-badge">{{ getCategoryLabel(skill.category) }}</span>
              <span class="source-badge">{{ skill.source_name }}</span>
            </div>
          </div>
        </div>

        <p class="item-description">{{ skill.description }}</p>

        <div v-if="skill.tags?.length" class="item-tags">
          <span v-for="tag in skill.tags.slice(0, 3)" :key="tag" class="tag">{{ tag }}</span>
          <span v-if="skill.tags.length > 3" class="tag more">+{{ skill.tags.length - 3 }}</span>
        </div>

        <div class="item-meta">
          <span v-if="skill.author && skill.author !== 'clawdbot' && skill.author !== 'moltbot'" class="meta-item">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2" />
              <circle cx="12" cy="7" r="4" />
            </svg>
            {{ skill.author }}
          </span>
          <span v-if="skill.stars" class="meta-item">
            <svg viewBox="0 0 24 24" fill="currentColor">
              <path d="M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z" />
            </svg>
            {{ skill.stars }}
          </span>
          <span v-if="skill.downloads" class="meta-item">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
              <polyline points="7,10 12,15 17,10" />
              <line x1="12" y1="15" x2="12" y2="3" />
            </svg>
            {{ skill.downloads }}
          </span>
        </div>

        <div class="item-actions">
          <button
            v-if="!skill.installed"
            class="btn-install"
            @click="installSkill(skill)"
            :disabled="skillStore.loading"
          >
            {{ t('skillStore.actions.install') }}
          </button>
          <span v-else class="installed-badge">{{ t('skillStore.status.installed') }}</span>
          <a v-if="skill.homepage" :href="skill.homepage" target="_blank" class="btn-link">
            {{ t('skillStore.actions.details') }}
          </a>
        </div>
      </div>

      <div v-if="filteredRemoteSkills.length === 0" class="empty-state">
        <p v-if="skillStore.refreshing">{{ t('skillStore.empty.loadingStore') }}</p>
        <p v-else>{{ t('skillStore.empty.noMatchingStore') }}</p>
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
              <option value="clawdhub">{{ t('skillStore.modal.typeClawdhub') }}</option>
            </select>
          </div>
          <div class="form-group">
            <label>{{ t('skillStore.modal.url') }}</label>
            <input v-model="newSource.url" type="text" :placeholder="t('skillStore.modal.urlPlaceholder')" />
          </div>
          <div class="form-group">
            <label>{{ t('skillStore.modal.descriptionOptional') }}</label>
            <input
              v-model="newSource.description"
              type="text"
              :placeholder="t('skillStore.modal.descriptionPlaceholder')"
            />
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
