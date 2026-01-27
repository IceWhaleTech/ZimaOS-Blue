<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useSkillStore } from '@/stores/skill'
import type { Skill, RemoteSkill } from '@/api/skill'

const { t, te } = useI18n()
const skillStore = useSkillStore()

// Tab state
const activeTab = ref<'installed' | 'store'>('installed')

// Filter state
const searchQuery = ref('')
const selectedCategory = ref('')
const selectedSource = ref('')

// UI state
const showAddSourceModal = ref(false)
const newSource = ref({
  id: '',
  name: '',
  url: '',
  type: 'custom' as 'clawdhub' | 'github' | 'custom',
  description: '',
})

// Computed
const filteredInstalledSkills = computed(() => {
  let result = skillStore.skills
  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase()
    result = result.filter(
      (s) =>
        s.name.toLowerCase().includes(query) ||
        s.description.toLowerCase().includes(query) ||
        s.tags?.some((t) => t.toLowerCase().includes(query))
    )
  }
  if (selectedCategory.value) {
    result = result.filter((s) => s.category === selectedCategory.value)
  }
  return result
})

const filteredRemoteSkills = computed(() => {
  let result = skillStore.remoteSkills
  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase()
    result = result.filter(
      (s) =>
        s.name.toLowerCase().includes(query) ||
        s.description.toLowerCase().includes(query) ||
        s.tags?.some((t) => t.toLowerCase().includes(query))
    )
  }
  if (selectedCategory.value) {
    result = result.filter((s) => s.category === selectedCategory.value)
  }
  if (selectedSource.value) {
    result = result.filter((s) => s.source_id === selectedSource.value)
  }
  return result
})

const stats = computed(() => ({
  total: skillStore.skills.length,
  enabled: skillStore.enabledSkills.length,
  builtin: skillStore.builtinSkills.length,
  installed: skillStore.installedSkills.length,
  available: skillStore.remoteSkills.filter((s) => !s.installed).length,
}))

// Methods
async function toggleSkill(skill: Skill) {
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
  if (skill.builtin) return
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

function getCategoryIcon(category: string | undefined): string {
  const icons: Record<string, string> = {
    productivity: '📝',
    utility: '🔧',
    system: '⚙️',
    communication: '💬',
    information: '📰',
    integration: '🔗',
    extension: '🧩',
    development: '💻',
    analytics: '📊',
    other: '📦',
  }
  return icons[category || 'other'] || '📦'
}

function getCategoryLabel(category: string | undefined): string {
  const cat = category || 'other'
  const key = `skillStore.categories.${cat}`
  if (te(key)) return t(key)
  return category || t('skillStore.categories.other')
}

// Lifecycle
onMounted(async () => {
  await Promise.all([
    skillStore.fetchSkills(),
    skillStore.fetchSources(),
  ])
  // Auto refresh store on first load
  if (skillStore.remoteSkills.length === 0) {
    await skillStore.refreshSources()
  }
})
</script>

<template>
  <div class="skill-store">
    <!-- Header -->
    <div class="header">
      <div class="title-section">
        <h1>{{ t('skillStore.title') }}</h1>
        <p class="subtitle">{{ t('skillStore.subtitle') }}</p>
      </div>
      <div class="stats">
        <div class="stat">
          <span class="stat-value">{{ stats.total }}</span>
          <span class="stat-label">{{ t('skillStore.stats.installed') }}</span>
        </div>
        <div class="stat">
          <span class="stat-value">{{ stats.enabled }}</span>
          <span class="stat-label">{{ t('skillStore.stats.enabled') }}</span>
        </div>
        <div class="stat">
          <span class="stat-value">{{ stats.available }}</span>
          <span class="stat-label">{{ t('skillStore.stats.available') }}</span>
        </div>
      </div>
    </div>

    <!-- Tabs -->
    <div class="tabs">
      <button
        :class="['tab', { active: activeTab === 'installed' }]"
        @click="activeTab = 'installed'"
      >
        {{ t('skillStore.tabs.installedWithCount', { count: skillStore.skills.length }) }}
      </button>
      <button
        :class="['tab', { active: activeTab === 'store' }]"
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

      <select v-model="selectedCategory" class="filter-select">
        <option value="">{{ t('skillStore.filters.allCategories') }}</option>
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
      </div>
    </div>

    <!-- Error message -->
    <div v-if="skillStore.error" class="error-banner">
      {{ skillStore.error }}
      <button @click="skillStore.clearError">×</button>
    </div>

    <!-- Loading -->
    <div v-if="skillStore.loading" class="loading">
      <div class="spinner"></div>
      <span>{{ t('common.loading') }}</span>
    </div>

    <!-- Installed Skills Grid -->
    <div v-else-if="activeTab === 'installed'" class="skills-grid">
      <div
        v-for="skill in filteredInstalledSkills"
        :key="skill.id"
        :class="['skill-card', { disabled: !skill.enabled, builtin: skill.builtin }]"
      >
        <div class="skill-header">
          <span class="skill-icon">{{ getCategoryIcon(skill.category) }}</span>
          <div class="skill-title">
            <h3 :title="skill.name">{{ skill.name }}</h3>
            <div class="skill-title-meta">
              <span v-if="skill.version && skill.version !== 'latest'" class="skill-version">v{{ skill.version }}</span>
              <span v-if="skill.builtin" class="badge builtin">{{ t('skillStore.status.builtin') }}</span>
              <span :class="['badge', skill.enabled ? 'enabled' : 'disabled']">
                {{ skill.enabled ? t('skillStore.actions.enable') : t('skillStore.actions.disable') }}
              </span>
            </div>
          </div>
        </div>

        <p class="skill-description">{{ skill.description }}</p>

        <div v-if="skill.tags?.length" class="skill-tags">
          <span v-for="tag in skill.tags.slice(0, 3)" :key="tag" class="tag">{{ tag }}</span>
        </div>

        <div class="skill-meta">
          <span v-if="skill.author && skill.author !== 'clawdbot' && skill.author !== 'moltbot'" class="meta-item">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2" />
              <circle cx="12" cy="7" r="4" />
            </svg>
            {{ skill.author }}
          </span>
          <span v-if="skill.category" class="meta-item">
            {{ getCategoryLabel(skill.category) }}
          </span>
        </div>

        <div class="skill-actions">
          <button
            :class="['btn-toggle', { active: skill.enabled }]"
            @click="toggleSkill(skill)"
          >
            {{ skill.enabled ? t('skillStore.actions.disable') : t('skillStore.actions.enable') }}
          </button>
          <button
            v-if="!skill.builtin"
            class="btn-uninstall"
            @click="uninstallSkill(skill)"
          >
            {{ t('skillStore.actions.uninstall') }}
          </button>
        </div>
      </div>

      <div v-if="filteredInstalledSkills.length === 0" class="empty-state">
        <p>{{ t('skillStore.empty.noMatchingInstalled') }}</p>
      </div>
    </div>

    <!-- Store Skills Grid -->
    <div v-else class="skills-grid">
      <div
        v-for="skill in filteredRemoteSkills"
        :key="skill.id"
        :class="['skill-card', 'store-card', { installed: skill.installed }]"
      >
        <div class="skill-header">
          <span class="skill-icon">{{ getCategoryIcon(skill.category) }}</span>
          <div class="skill-title">
            <h3 :title="skill.name">{{ skill.name }}</h3>
            <div class="skill-title-meta">
              <span v-if="skill.version && skill.version !== 'latest'" class="skill-version">v{{ skill.version }}</span>
              <span class="source-badge">{{ skill.source_name }}</span>
            </div>
          </div>
        </div>

        <p class="skill-description">{{ skill.description }}</p>

        <div v-if="skill.tags?.length" class="skill-tags">
          <span v-for="tag in skill.tags.slice(0, 3)" :key="tag" class="tag">{{ tag }}</span>
        </div>

        <div class="skill-meta">
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

        <div class="skill-actions">
          <button
            v-if="!skill.installed"
            class="btn-install"
            @click="installSkill(skill)"
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
.skill-store {
  padding: 24px;
  max-width: 1400px;
  margin: 0 auto;
}

.header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 24px;
}

.title-section h1 {
  font-size: 28px;
  font-weight: 600;
  margin: 0;
  color: var(--text-primary);
}

.subtitle {
  color: var(--text-secondary);
  margin: 4px 0 0;
}

.stats {
  display: flex;
  gap: 24px;
}

.stat {
  text-align: center;
}

.stat-value {
  display: block;
  font-size: 24px;
  font-weight: 600;
  color: var(--primary);
}

.stat-label {
  font-size: 12px;
  color: var(--text-secondary);
}

.tabs {
  display: flex;
  gap: 8px;
  margin-bottom: 16px;
  border-bottom: 1px solid var(--border);
  padding-bottom: 8px;
}

.tab {
  padding: 8px 16px;
  border: none;
  background: none;
  color: var(--text-secondary);
  cursor: pointer;
  font-size: 14px;
  border-radius: 6px;
  transition: all 0.2s;
}

.tab:hover {
  background: var(--bg-hover);
}

.tab.active {
  background: var(--primary);
  color: white;
}

.filters {
  display: flex;
  gap: 12px;
  margin-bottom: 20px;
  flex-wrap: wrap;
  align-items: center;
}

.search-box {
  position: relative;
  flex: 1;
  min-width: 200px;
}

.search-icon {
  position: absolute;
  left: 12px;
  top: 50%;
  transform: translateY(-50%);
  width: 18px;
  height: 18px;
  color: var(--text-secondary);
}

.search-input {
  width: 100%;
  padding: 10px 12px 10px 40px;
  border: 1px solid var(--border);
  border-radius: 8px;
  font-size: 14px;
  background: var(--bg-secondary);
  color: var(--text-primary);
}

.search-input:focus {
  outline: none;
  border-color: var(--primary);
}

.filter-select {
  padding: 10px 12px;
  border: 1px solid var(--border);
  border-radius: 8px;
  font-size: 14px;
  background: var(--bg-secondary);
  color: var(--text-primary);
  min-width: 120px;
}

.filter-actions {
  display: flex;
  gap: 8px;
}

.btn-refresh,
.btn-add-source {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 10px 16px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--bg-secondary);
  color: var(--text-primary);
  cursor: pointer;
  font-size: 14px;
  transition: all 0.2s;
}

.btn-refresh:hover,
.btn-add-source:hover {
  background: var(--bg-hover);
}

.btn-refresh:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.btn-refresh svg,
.btn-add-source svg {
  width: 16px;
  height: 16px;
}

.error-banner {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background: #fee2e2;
  border: 1px solid #fecaca;
  border-radius: 8px;
  color: #dc2626;
  margin-bottom: 16px;
}

.error-banner button {
  background: none;
  border: none;
  font-size: 20px;
  cursor: pointer;
  color: inherit;
}

.loading {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 48px;
  color: var(--text-secondary);
}

.spinner {
  width: 20px;
  height: 20px;
  border: 2px solid var(--border);
  border-top-color: var(--primary);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

.skills-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 14px;
}

.skill-card {
  position: relative;
  background: var(--glass-bg, rgba(255, 255, 255, 0.05));
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg, 12px);
  padding: 14px;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  overflow: hidden;
}

.skill-card::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 3px;
  background: linear-gradient(90deg, var(--primary), #8b5cf6);
  opacity: 0;
  transition: opacity 0.3s;
}

.skill-card:hover {
  background: var(--glass-bg-hover, rgba(255, 255, 255, 0.1));
  border-color: rgba(255, 255, 255, 0.2);
  box-shadow: var(--shadow-lg, 0 10px 15px rgba(0, 0, 0, 0.5)), var(--shadow-glow, 0 0 20px rgba(59, 130, 246, 0.15));
  transform: translateY(-4px);
}

.skill-card:hover::before {
  opacity: 1;
}

.skill-card.disabled {
  opacity: 0.5;
}

.skill-card.disabled::before {
  background: linear-gradient(90deg, var(--text-muted), var(--text-secondary));
}

.skill-card.installed {
  border-color: rgba(34, 197, 94, 0.4);
}

.skill-card.installed::before {
  background: linear-gradient(90deg, var(--success), #10b981);
  opacity: 1;
}

.skill-card.store-card:not(.installed)::before {
  background: linear-gradient(90deg, var(--warning), #f97316);
}

.skill-header {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  margin-bottom: 10px;
}

.skill-icon {
  width: 40px;
  height: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
  background: var(--glass-bg, rgba(255, 255, 255, 0.05));
  border: 1px solid var(--border);
  border-radius: var(--radius-md, 8px);
  flex-shrink: 0;
}

.skill-title {
  flex: 1;
  min-width: 0;
}

.skill-title h3 {
  margin: 0 0 4px;
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.skill-title-meta {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.skill-version {
  display: inline-block;
  font-size: 10px;
  color: var(--text-muted);
  background: var(--glass-bg, rgba(255, 255, 255, 0.05));
  border: 1px solid var(--border);
  padding: 1px 5px;
  border-radius: var(--radius-full, 9999px);
}

.badge {
  padding: 2px 6px;
  border-radius: var(--radius-full, 9999px);
  font-size: 10px;
  font-weight: 500;
  letter-spacing: 0.02em;
}

.badge.builtin {
  background: rgba(99, 102, 241, 0.15);
  color: #a5b4fc;
  border: 1px solid rgba(99, 102, 241, 0.3);
}

.badge.enabled {
  background: rgba(34, 197, 94, 0.15);
  color: #86efac;
  border: 1px solid rgba(34, 197, 94, 0.3);
}

.badge.disabled {
  background: var(--glass-bg, rgba(255, 255, 255, 0.05));
  color: var(--text-muted);
  border: 1px solid var(--border);
}

.source-badge {
  padding: 2px 6px;
  border-radius: var(--radius-full, 9999px);
  font-size: 10px;
  font-weight: 500;
  background: rgba(245, 158, 11, 0.15);
  color: #fcd34d;
  border: 1px solid rgba(245, 158, 11, 0.3);
}

/* Light mode badge overrides */
:root.light .badge.builtin,
[data-theme="light"] .badge.builtin {
  background: rgba(99, 102, 241, 0.1);
  color: #4f46e5;
  border-color: rgba(99, 102, 241, 0.2);
}

:root.light .badge.enabled,
[data-theme="light"] .badge.enabled {
  background: rgba(34, 197, 94, 0.1);
  color: #059669;
  border-color: rgba(34, 197, 94, 0.2);
}

:root.light .source-badge,
[data-theme="light"] .source-badge {
  background: rgba(245, 158, 11, 0.1);
  color: #b45309;
  border-color: rgba(245, 158, 11, 0.2);
}

.skill-description {
  font-size: 12px;
  color: var(--text-secondary);
  margin: 0 0 10px;
  line-height: 1.5;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  min-height: 36px;
}

.skill-tags {
  display: flex;
  gap: 4px;
  margin-bottom: 10px;
  flex-wrap: wrap;
}

.tag {
  padding: 2px 6px;
  background: var(--glass-bg, rgba(255, 255, 255, 0.05));
  border: 1px solid var(--border);
  border-radius: var(--radius-sm, 6px);
  font-size: 10px;
  color: var(--text-secondary);
  transition: all 0.2s;
}

.tag:hover {
  background: var(--primary);
  border-color: var(--primary);
  color: white;
}

.skill-meta {
  display: flex;
  gap: 10px;
  margin-bottom: 10px;
  font-size: 11px;
  color: var(--text-muted);
  padding: 8px 0;
  border-top: 1px dashed var(--border);
  flex-wrap: wrap;
}

.meta-item {
  display: flex;
  align-items: center;
  gap: 4px;
}

.meta-item svg {
  width: 12px;
  height: 12px;
  opacity: 0.7;
}

.skill-actions {
  display: flex;
  gap: 6px;
  padding-top: 8px;
  border-top: 1px solid var(--border);
}

.btn-toggle,
.btn-uninstall,
.btn-install,
.btn-link {
  padding: 6px 12px;
  border-radius: var(--radius-md, 8px);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  text-decoration: none;
}

.btn-toggle {
  flex: 1;
  border: 1px solid var(--border);
  background: var(--glass-bg, rgba(255, 255, 255, 0.05));
  color: var(--text-primary);
}

.btn-toggle:hover {
  background: var(--glass-bg-hover, rgba(255, 255, 255, 0.1));
  border-color: var(--primary);
}

.btn-toggle.active {
  background: rgba(239, 68, 68, 0.15);
  border-color: rgba(239, 68, 68, 0.3);
  color: #f87171;
}

.btn-toggle.active:hover {
  background: rgba(239, 68, 68, 0.25);
}

.btn-uninstall {
  border: 1px solid rgba(239, 68, 68, 0.3);
  background: transparent;
  color: #f87171;
}

.btn-uninstall:hover {
  background: rgba(239, 68, 68, 0.15);
}

.btn-install {
  flex: 1;
  border: none;
  background: linear-gradient(135deg, var(--primary), #6366f1);
  color: white;
  box-shadow: 0 2px 8px rgba(59, 130, 246, 0.3);
}

.btn-install:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(59, 130, 246, 0.4);
}

.btn-install:active {
  transform: translateY(0);
}

.installed-badge {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
  padding: 6px;
  color: #86efac;
  font-size: 11px;
  font-weight: 500;
  background: rgba(34, 197, 94, 0.15);
  border: 1px solid rgba(34, 197, 94, 0.3);
  border-radius: var(--radius-md, 8px);
}

.installed-badge::before {
  content: '\2713';
  font-weight: bold;
}

.btn-link {
  border: 1px solid var(--border);
  background: transparent;
  color: var(--text-primary);
  display: flex;
  align-items: center;
  justify-content: center;
}

.btn-link:hover {
  background: var(--glass-bg-hover, rgba(255, 255, 255, 0.1));
  border-color: var(--primary);
  color: var(--primary);
}

/* Light mode button overrides */
:root.light .btn-toggle.active,
[data-theme="light"] .btn-toggle.active {
  background: rgba(239, 68, 68, 0.1);
  color: #dc2626;
}

:root.light .btn-uninstall,
[data-theme="light"] .btn-uninstall {
  color: #dc2626;
}

:root.light .installed-badge,
[data-theme="light"] .installed-badge {
  background: rgba(34, 197, 94, 0.1);
  color: #059669;
}

.empty-state {
  grid-column: 1 / -1;
  text-align: center;
  padding: 48px;
  color: var(--text-secondary);
}

/* Modal */
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.modal {
  background: var(--bg-primary);
  border-radius: 12px;
  width: 100%;
  max-width: 480px;
  box-shadow: 0 20px 40px rgba(0, 0, 0, 0.2);
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 20px;
  border-bottom: 1px solid var(--border);
}

.modal-header h2 {
  margin: 0;
  font-size: 18px;
}

.modal-close {
  background: none;
  border: none;
  font-size: 24px;
  cursor: pointer;
  color: var(--text-secondary);
}

.modal-body {
  padding: 20px;
}

.form-group {
  margin-bottom: 16px;
}

.form-group label {
  display: block;
  margin-bottom: 6px;
  font-size: 14px;
  font-weight: 500;
  color: var(--text-primary);
}

.form-group input,
.form-group select {
  width: 100%;
  padding: 10px 12px;
  border: 1px solid var(--border);
  border-radius: 8px;
  font-size: 14px;
  background: var(--bg-secondary);
  color: var(--text-primary);
}

.form-group input:focus,
.form-group select:focus {
  outline: none;
  border-color: var(--primary);
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  padding: 16px 20px;
  border-top: 1px solid var(--border);
}

.btn-cancel,
.btn-confirm {
  padding: 10px 20px;
  border-radius: 8px;
  font-size: 14px;
  cursor: pointer;
}

.btn-cancel {
  border: 1px solid var(--border);
  background: transparent;
  color: var(--text-primary);
}

.btn-confirm {
  border: none;
  background: var(--primary);
  color: white;
}

/* Use global design system variables */
.skill-store {
  --bg-primary: var(--color-bg-elevated, #1E293B);
  --bg-secondary: var(--color-bg-surface, #334155);
  --bg-hover: var(--glass-bg-hover, rgba(255, 255, 255, 0.1));
  --text-primary: var(--color-text-primary, #F8FAFC);
  --text-secondary: var(--color-text-secondary, #94A3B8);
  --text-muted: var(--color-text-muted, #64748B);
  --border: var(--glass-border, rgba(255, 255, 255, 0.1));
  --primary: var(--color-accent, #3B82F6);
  --primary-hover: var(--color-accent-hover, #2563EB);
  --success: var(--color-success, #22C55E);
  --warning: var(--color-warning, #F59E0B);
  --error: var(--color-error, #EF4444);
}

/* Light mode overrides */
:root.light .skill-store,
[data-theme="light"] .skill-store {
  --bg-primary: var(--color-bg-elevated, #FFFFFF);
  --bg-secondary: var(--color-bg-surface, #F1F5F9);
  --bg-hover: var(--glass-bg-hover, rgba(0, 0, 0, 0.05));
  --text-primary: var(--color-text-primary, #0F172A);
  --text-secondary: var(--color-text-secondary, #475569);
  --text-muted: var(--color-text-muted, #64748B);
  --border: var(--glass-border, rgba(0, 0, 0, 0.1));
}
</style>
