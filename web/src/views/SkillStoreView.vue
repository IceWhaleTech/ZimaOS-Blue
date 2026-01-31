<script setup lang="ts">
import { ref, computed, onMounted, watch, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useSkillStore } from '@/stores/skill'
import { useNotificationStore } from '@/stores/notification'
import { skillApi } from '@/api/skill'
import type { Skill, RemoteSkill, LocalSkill } from '@/api/skill'

const { t, te } = useI18n()
const skillStore = useSkillStore()
const notificationStore = useNotificationStore()

// Tab state
const activeTab = ref<'installed' | 'store' | 'featured' | 'local'>('installed')

// Featured skills state
const featuredSkills = ref<RemoteSkill[]>([])
const loadingFeatured = ref(false)

// Local skills state
const localSkills = ref<LocalSkill[]>([])
const loadingLocal = ref(false)
const scanningLocal = ref(false)

// Verification state
const verificationStatus = ref<Record<string, 'pending' | 'verified' | 'error'>>({})
const verifyingSkillId = ref<string | null>(null)

// Filter state
const searchQuery = ref('')
const debouncedSearchQuery = ref('')
const searchDebounceTimer = ref<ReturnType<typeof setTimeout> | null>(null)
const selectedCategory = ref('')
const selectedSource = ref('')

// Debounce search input
watch(searchQuery, (newValue) => {
  if (searchDebounceTimer.value) {
    clearTimeout(searchDebounceTimer.value)
  }
  searchDebounceTimer.value = setTimeout(() => {
    debouncedSearchQuery.value = newValue
  }, 300)
})

// UI state
const showAddSourceModal = ref(false)
const showInstallURLModal = ref(false)
const showUninstallConfirm = ref(false)
const skillToUninstall = ref<Skill | null>(null)
const installURL = ref('')
const installURLName = ref('')
const isInstallingFromURL = ref(false)
const installingSkillId = ref<string | null>(null)
const installRetryCount = ref<Record<string, number>>({})
const installRetryMax = 3
const initialLoading = ref(true)

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
  if (debouncedSearchQuery.value) {
    const query = debouncedSearchQuery.value.toLowerCase()
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
  if (debouncedSearchQuery.value) {
    const query = debouncedSearchQuery.value.toLowerCase()
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

async function installSkill(skill: RemoteSkill, isRetry = false) {
  installingSkillId.value = skill.id

  // Initialize or increment retry count
  if (!isRetry) {
    installRetryCount.value[skill.id] = 0
  }

  try {
    const result = await skillStore.installSkill(skill.id)
    if (result.success) {
      // Clear retry count on success
      delete installRetryCount.value[skill.id]
      notificationStore.success(
        t('skillStore.notifications.installSuccess'),
        t('skillStore.notifications.installSuccessMessage', { name: skill.name })
      )
    } else {
      // Check if we should retry
      const currentRetry = installRetryCount.value[skill.id] || 0
      if (currentRetry < installRetryMax - 1) {
        installRetryCount.value[skill.id] = currentRetry + 1
        notificationStore.warning(
          t('skillStore.notifications.installRetrying'),
          t('skillStore.notifications.installRetryingMessage', {
            name: skill.name,
            attempt: currentRetry + 2,
            max: installRetryMax
          })
        )
        // Wait a bit before retrying
        await new Promise(resolve => setTimeout(resolve, 1000))
        return installSkill(skill, true)
      } else {
        // Max retries reached
        delete installRetryCount.value[skill.id]
        notificationStore.error(
          t('skillStore.notifications.installError'),
          result.message || t('skillStore.notifications.installErrorMessage', { name: skill.name })
        )
      }
    }
  } catch {
    // Check if we should retry on exception
    const currentRetry = installRetryCount.value[skill.id] || 0
    if (currentRetry < installRetryMax - 1) {
      installRetryCount.value[skill.id] = currentRetry + 1
      notificationStore.warning(
        t('skillStore.notifications.installRetrying'),
        t('skillStore.notifications.installRetryingMessage', {
          name: skill.name,
          attempt: currentRetry + 2,
          max: installRetryMax
        })
      )
      await new Promise(resolve => setTimeout(resolve, 1000))
      return installSkill(skill, true)
    } else {
      delete installRetryCount.value[skill.id]
      notificationStore.error(
        t('skillStore.notifications.installError'),
        t('skillStore.notifications.installErrorMessage', { name: skill.name })
      )
    }
  } finally {
    if (!installRetryCount.value[skill.id]) {
      installingSkillId.value = null
    }
  }
}

function confirmUninstall(skill: Skill) {
  if (skill.builtin) return
  skillToUninstall.value = skill
  showUninstallConfirm.value = true
}

async function uninstallSkill(skill: Skill) {
  if (skill.builtin) return
  const result = await skillStore.uninstallSkill(skill.id)
  showUninstallConfirm.value = false
  skillToUninstall.value = null
  if (result.success) {
    notificationStore.success(
      t('skillStore.notifications.uninstallSuccess'),
      t('skillStore.notifications.uninstallSuccessMessage', { name: skill.name })
    )
  } else {
    notificationStore.error(
      t('skillStore.notifications.uninstallError'),
      result.message || t('skillStore.notifications.uninstallErrorMessage', { name: skill.name })
    )
  }
}

function cancelUninstall() {
  showUninstallConfirm.value = false
  skillToUninstall.value = null
}

async function installFromURL() {
  if (!installURL.value) return
  isInstallingFromURL.value = true
  try {
    const result = await skillStore.installFromURL({
      url: installURL.value,
      name: installURLName.value || undefined,
    })
    if (result.success) {
      showInstallURLModal.value = false
      installURL.value = ''
      installURLName.value = ''
      notificationStore.success(
        t('skillStore.notifications.installSuccess'),
        t('skillStore.notifications.installFromURLSuccess', { name: result.skill?.name || 'Skill' })
      )
    } else {
      notificationStore.error(
        t('skillStore.notifications.installError'),
        t('skillStore.notifications.installFromURLError')
      )
    }
  } finally {
    isInstallingFromURL.value = false
  }
}

async function refreshStore() {
  await skillStore.refreshSources()
}

async function verifySkill(skillId: string) {
  verifyingSkillId.value = skillId
  verificationStatus.value[skillId] = 'pending'
  try {
    const response = await skillApi.verify(skillId)
    if (response.data.visible) {
      verificationStatus.value[skillId] = 'verified'
    } else {
      verificationStatus.value[skillId] = 'error'
    }
  } catch {
    verificationStatus.value[skillId] = 'error'
  } finally {
    verifyingSkillId.value = null
  }
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

function highlightText(text: string, query: string): string {
  if (!query || !text) return text
  const escapedQuery = query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  const regex = new RegExp(`(${escapedQuery})`, 'gi')
  return text.replace(regex, '<mark class="search-highlight">$1</mark>')
}

function getInstallButtonText(skillId: string): string {
  if (installingSkillId.value !== skillId) {
    return t('skillStore.actions.install')
  }
  const retryCount = installRetryCount.value[skillId]
  if (retryCount && retryCount > 0) {
    return t('skillStore.notifications.retryAttempt', { attempt: retryCount + 1, max: installRetryMax })
  }
  return t('skillStore.modal.installing')
}

// Filtered featured skills
const filteredFeaturedSkills = computed(() => {
  let result = featuredSkills.value
  if (debouncedSearchQuery.value) {
    const query = debouncedSearchQuery.value.toLowerCase()
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

// Fetch featured skills
async function fetchFeatured() {
  loadingFeatured.value = true
  try {
    featuredSkills.value = await skillStore.fetchFeaturedSkills()
  } finally {
    loadingFeatured.value = false
  }
}

// Watch for tab changes to load featured skills
watch(activeTab, (newTab) => {
  if (newTab === 'featured' && featuredSkills.value.length === 0) {
    fetchFeatured()
  }
  if (newTab === 'local' && localSkills.value.length === 0) {
    fetchLocal()
  }
})

// Filtered local skills
const filteredLocalSkills = computed(() => {
  let result = localSkills.value
  if (debouncedSearchQuery.value) {
    const query = debouncedSearchQuery.value.toLowerCase()
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

// Combined search results across all sources
interface CombinedSearchResult {
  id: string
  name: string
  description: string
  source: 'installed' | 'featured' | 'store' | 'local'
  installed: boolean
  enabled?: boolean
  category?: string
  tags?: string[]
  author?: string
  homepage?: string
  source_url?: string
  stars?: number
}

const combinedSearchResults = computed((): CombinedSearchResult[] => {
  if (!debouncedSearchQuery.value) return []

  const results: CombinedSearchResult[] = []
  const seenIds = new Set<string>()

  // Add installed skills
  for (const skill of filteredInstalledSkills.value) {
    if (!seenIds.has(skill.id)) {
      seenIds.add(skill.id)
      results.push({
        id: skill.id,
        name: skill.name,
        description: skill.description,
        source: 'installed',
        installed: true,
        enabled: skill.enabled,
        category: skill.category,
        tags: skill.tags,
      })
    }
  }

  // Add featured skills (not already installed)
  for (const skill of filteredFeaturedSkills.value) {
    if (!seenIds.has(skill.id)) {
      seenIds.add(skill.id)
      results.push({
        id: skill.id,
        name: skill.name,
        description: skill.description,
        source: 'featured',
        installed: skill.installed || false,
        category: skill.category,
        tags: skill.tags,
        author: skill.author,
        homepage: skill.homepage,
        source_url: skill.source_url ?? undefined,
        stars: skill.stars,
      })
    }
  }

  // Add store skills (not already in results)
  for (const skill of filteredRemoteSkills.value) {
    if (!seenIds.has(skill.id)) {
      seenIds.add(skill.id)
      results.push({
        id: skill.id,
        name: skill.name,
        description: skill.description,
        source: 'store',
        installed: skill.installed || false,
        category: skill.category,
        tags: skill.tags,
        author: skill.author,
        homepage: skill.homepage,
        source_url: skill.source_url ?? undefined,
        stars: skill.stars,
      })
    }
  }

  // Add local skills (not already in results)
  for (const skill of filteredLocalSkills.value) {
    if (!seenIds.has(skill.id)) {
      seenIds.add(skill.id)
      results.push({
        id: skill.id,
        name: skill.name,
        description: skill.description,
        source: 'local',
        installed: skill.installed || false,
        category: skill.category,
        tags: skill.tags,
        author: skill.author,
      })
    }
  }

  return results
})

// Fetch local skills
async function fetchLocal() {
  loadingLocal.value = true
  try {
    const response = await skillStore.fetchLocalSkills()
    localSkills.value = response.skills || []
  } finally {
    loadingLocal.value = false
  }
}

// Scan local skills directory
async function scanLocal() {
  scanningLocal.value = true
  try {
    const result = await skillStore.scanLocalSkills()
    await fetchLocal()
    if (result.success) {
      notificationStore.success(
        t('skillStore.notifications.scanSuccess'),
        t('skillStore.notifications.scanSuccessMessage', { count: result.skills_found })
      )
    }
  } finally {
    scanningLocal.value = false
  }
}

// Lifecycle
onMounted(async () => {
  // Fetch installed skills first (fast, local)
  await skillStore.fetchSkills()
  initialLoading.value = false

  // Fetch sources in background (don't block)
  skillStore.fetchSources().catch(() => {})

  // Auto refresh store in background (don't block page load)
  if (skillStore.remoteSkills.length === 0) {
    skillStore.refreshSources().catch(() => {})
  }
})

onUnmounted(() => {
  // Cleanup debounce timer
  if (searchDebounceTimer.value) {
    clearTimeout(searchDebounceTimer.value)
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
        :class="['tab', { active: activeTab === 'featured' }]"
        @click="activeTab = 'featured'"
      >
        {{ t('skillStore.tabs.featured') }}
      </button>
      <button
        :class="['tab', { active: activeTab === 'local' }]"
        @click="activeTab = 'local'"
      >
        {{ t('skillStore.tabs.local') }}
      </button>
      <button
        :class="['tab', { active: activeTab === 'store' }]"
        @click="activeTab = 'store'"
      >
        <template v-if="skillStore.refreshing">
          {{ t('skillStore.tabs.store') }}
          <span class="tab-spinner"></span>
        </template>
        <template v-else-if="skillStore.remoteSkills.length > 0">
          {{ t('skillStore.tabs.storeWithCount', { count: skillStore.remoteSkills.length }) }}
        </template>
        <template v-else>
          {{ t('skillStore.tabs.store') }}
        </template>
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
        <button v-if="activeTab === 'store'" class="btn-refresh" :disabled="skillStore.refreshing" @click="refreshStore">
          <svg v-if="!skillStore.refreshing" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M21 12a9 9 0 0 0-9-9 9.75 9.75 0 0 0-6.74 2.74L3 8" />
            <path d="M3 3v5h5" />
            <path d="M3 12a9 9 0 0 0 9 9 9.75 9.75 0 0 0 6.74-2.74L21 16" />
            <path d="M16 21h5v-5" />
          </svg>
          <span v-else class="spinner"></span>
          {{ t('skillStore.actions.refresh') }}
        </button>
        <button class="btn-install-url" @click="showInstallURLModal = true">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71" />
            <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71" />
          </svg>
          {{ t('skillStore.actions.installFromURL') }}
        </button>
        <button v-if="activeTab === 'store'" class="btn-add-source" @click="showAddSourceModal = true">
          + {{ t('skillStore.actions.addSource') }}
        </button>
        <button v-if="activeTab === 'local'" class="btn-scan-local" :disabled="scanningLocal" @click="scanLocal">
          <svg v-if="!scanningLocal" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z" />
            <line x1="12" y1="11" x2="12" y2="17" />
            <line x1="9" y1="14" x2="15" y2="14" />
          </svg>
          <span v-else class="spinner"></span>
          {{ t('skillStore.actions.scanLocal') }}
        </button>
      </div>
    </div>

    <!-- Error message -->
    <div v-if="skillStore.error" class="error-banner">
      {{ skillStore.error }}
      <button @click="skillStore.clearError">×</button>
    </div>

    <!-- Search Results Summary -->
    <div v-if="debouncedSearchQuery && !initialLoading" class="search-results-summary">
      <div class="search-results-header">
        <h3>{{ t('skillStore.search.resultsCount', { count: combinedSearchResults.length, query: debouncedSearchQuery }) }}</h3>
        <button class="btn-clear-search" @click="searchQuery = ''; debouncedSearchQuery = ''">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <line x1="18" y1="6" x2="6" y2="18" />
            <line x1="6" y1="6" x2="18" y2="18" />
          </svg>
          {{ t('skillStore.search.clearSearch') }}
        </button>
      </div>
      <div v-if="combinedSearchResults.length > 0" class="search-results-breakdown">
        <span v-if="filteredInstalledSkills.length > 0" class="result-source" @click="activeTab = 'installed'">
          {{ t('skillStore.search.sourceInstalled') }}: {{ filteredInstalledSkills.length }}
        </span>
        <span v-if="filteredFeaturedSkills.length > 0" class="result-source" @click="activeTab = 'featured'">
          {{ t('skillStore.search.sourceFeatured') }}: {{ filteredFeaturedSkills.length }}
        </span>
        <span v-if="filteredRemoteSkills.length > 0" class="result-source" @click="activeTab = 'store'">
          {{ t('skillStore.search.sourceStore') }}: {{ filteredRemoteSkills.length }}
        </span>
        <span v-if="filteredLocalSkills.length > 0" class="result-source" @click="activeTab = 'local'">
          {{ t('skillStore.search.sourceLocal') }}: {{ filteredLocalSkills.length }}
        </span>
      </div>
    </div>

    <!-- Loading -->
    <div v-if="initialLoading" class="loading">
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
            <h3 :title="skill.name" v-html="highlightText(skill.name, debouncedSearchQuery)"></h3>
            <div class="skill-title-meta">
              <span v-if="skill.version && skill.version !== 'latest'" class="skill-version">v{{ skill.version }}</span>
              <span v-if="skill.builtin" class="badge builtin">{{ t('skillStore.status.builtin') }}</span>
              <span :class="['badge', skill.enabled ? 'enabled' : 'disabled']">
                {{ skill.enabled ? t('skillStore.actions.enable') : t('skillStore.actions.disable') }}
              </span>
              <!-- Verification status indicator -->
              <span
                v-if="!skill.builtin && verificationStatus[skill.id]"
                :class="['badge', 'verification', verificationStatus[skill.id]]"
                :title="verificationStatus[skill.id] === 'verified' ? t('skillStore.status.verified') : t('skillStore.status.verificationFailed')"
              >
                <svg v-if="verificationStatus[skill.id] === 'verified'" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="verify-icon">
                  <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" />
                  <polyline points="22 4 12 14.01 9 11.01" />
                </svg>
                <svg v-else-if="verificationStatus[skill.id] === 'error'" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="verify-icon">
                  <circle cx="12" cy="12" r="10" />
                  <line x1="15" y1="9" x2="9" y2="15" />
                  <line x1="9" y1="9" x2="15" y2="15" />
                </svg>
                <span v-else class="spinner-small"></span>
              </span>
            </div>
          </div>
        </div>

        <p class="skill-description" v-html="highlightText(skill.description, debouncedSearchQuery)"></p>

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
            class="btn-verify"
            :disabled="verifyingSkillId === skill.id"
            :title="t('skillStore.actions.verify')"
            @click="verifySkill(skill.id)"
          >
            <span v-if="verifyingSkillId === skill.id" class="spinner-small"></span>
            <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" />
              <polyline points="22 4 12 14.01 9 11.01" />
            </svg>
          </button>
          <button
            v-if="!skill.builtin"
            class="btn-uninstall"
            @click="confirmUninstall(skill)"
          >
            {{ t('skillStore.actions.uninstall') }}
          </button>
        </div>
      </div>

      <div v-if="filteredInstalledSkills.length === 0" class="empty-state">
        <template v-if="debouncedSearchQuery">
          <p class="no-results-title">{{ t('skillStore.empty.noResultsTitle', { query: debouncedSearchQuery }) }}</p>
          <div class="suggestions">
            <p class="suggestions-label">{{ t('skillStore.empty.suggestions') }}:</p>
            <ul>
              <li>{{ t('skillStore.empty.tryDifferentKeywords') }}</li>
              <li>{{ t('skillStore.empty.checkSpelling') }}</li>
              <li class="suggestion-link" @click="activeTab = 'store'">{{ t('skillStore.empty.checkFeatured') }}</li>
            </ul>
          </div>
        </template>
        <p v-else>{{ t('skillStore.empty.noMatchingInstalled') }}</p>
      </div>
    </div>

    <!-- Featured Skills Grid -->
    <div v-else-if="activeTab === 'featured'" class="skills-grid">
      <div v-if="loadingFeatured" class="loading-state">
        <div class="spinner"></div>
        <span>{{ t('common.loading') }}</span>
      </div>
      <template v-else>
        <div
          v-for="skill in filteredFeaturedSkills"
          :key="skill.id"
          :class="['skill-card', 'featured-card', { installed: skill.installed }]"
        >
          <div class="skill-header">
            <span class="skill-icon">{{ getCategoryIcon(skill.category) }}</span>
            <div class="skill-title">
              <h3 :title="skill.name" v-html="highlightText(skill.name, debouncedSearchQuery)"></h3>
              <div class="skill-title-meta">
                <span v-if="skill.version && skill.version !== 'latest'" class="skill-version">v{{ skill.version }}</span>
                <span class="featured-badge">{{ t('skillStore.status.featured') }}</span>
              </div>
            </div>
          </div>

          <p class="skill-description" v-html="highlightText(skill.description, debouncedSearchQuery)"></p>

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
            <span v-if="skill.category" class="meta-item">
              {{ getCategoryLabel(skill.category) }}
            </span>
          </div>

          <div class="skill-actions">
            <button
              v-if="!skill.installed"
              class="btn-install"
              :class="{ retrying: (installRetryCount[skill.id] ?? 0) > 0 }"
              :disabled="installingSkillId === skill.id"
              @click="installSkill(skill)"
            >
              <span v-if="installingSkillId === skill.id" class="spinner"></span>
              {{ getInstallButtonText(skill.id) }}
            </button>
            <span v-else class="installed-badge">{{ t('skillStore.status.installed') }}</span>
            <a v-if="skill.homepage" :href="skill.homepage" target="_blank" class="btn-link">
              {{ t('skillStore.actions.details') }}
            </a>
          </div>
        </div>

        <div v-if="filteredFeaturedSkills.length === 0 && !loadingFeatured" class="empty-state">
          <template v-if="debouncedSearchQuery">
            <p class="no-results-title">{{ t('skillStore.empty.noResultsTitle', { query: debouncedSearchQuery }) }}</p>
            <div class="suggestions">
              <p class="suggestions-label">{{ t('skillStore.empty.suggestions') }}:</p>
              <ul>
                <li>{{ t('skillStore.empty.tryDifferentKeywords') }}</li>
                <li>{{ t('skillStore.empty.checkSpelling') }}</li>
                <li class="suggestion-link" @click="activeTab = 'store'">{{ t('skillStore.empty.browseCategories') }}</li>
              </ul>
            </div>
          </template>
          <p v-else>{{ t('skillStore.empty.noFeatured') }}</p>
        </div>
      </template>
    </div>

    <!-- Local Skills Grid -->
    <div v-else-if="activeTab === 'local'" class="skills-grid">
      <div v-if="loadingLocal" class="loading-state">
        <div class="spinner"></div>
        <span>{{ t('common.loading') }}</span>
      </div>
      <template v-else>
        <div
          v-for="skill in filteredLocalSkills"
          :key="skill.id"
          :class="['skill-card', 'local-card', { installed: skill.installed }]"
        >
          <div class="skill-header">
            <span class="skill-icon">{{ getCategoryIcon(skill.category) }}</span>
            <div class="skill-title">
              <h3 :title="skill.name">{{ skill.name }}</h3>
              <div class="skill-title-meta">
                <span v-if="skill.version" class="skill-version">v{{ skill.version }}</span>
                <span class="local-badge">{{ t('skillStore.status.local') }}</span>
              </div>
            </div>
          </div>

          <p class="skill-description">{{ skill.description }}</p>

          <div v-if="skill.tags?.length" class="skill-tags">
            <span v-for="tag in skill.tags.slice(0, 3)" :key="tag" class="tag">{{ tag }}</span>
          </div>

          <div class="skill-meta">
            <span v-if="skill.author" class="meta-item">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2" />
                <circle cx="12" cy="7" r="4" />
              </svg>
              {{ skill.author }}
            </span>
            <span class="meta-item" :title="skill.file_path">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z" />
              </svg>
              {{ t('skillStore.status.localFile') }}
            </span>
          </div>

          <div class="skill-actions">
            <span v-if="skill.installed" class="installed-badge">{{ t('skillStore.status.installed') }}</span>
            <span v-else class="local-ready-badge">{{ t('skillStore.status.ready') }}</span>
          </div>
        </div>

        <div v-if="filteredLocalSkills.length === 0 && !loadingLocal" class="empty-state">
          <template v-if="debouncedSearchQuery">
            <p class="no-results-title">{{ t('skillStore.empty.noResultsTitle', { query: debouncedSearchQuery }) }}</p>
            <div class="suggestions">
              <p class="suggestions-label">{{ t('skillStore.empty.suggestions') }}:</p>
              <ul>
                <li>{{ t('skillStore.empty.tryDifferentKeywords') }}</li>
                <li>{{ t('skillStore.empty.checkSpelling') }}</li>
                <li class="suggestion-link" @click="scanLocal">{{ t('skillStore.empty.scanLocalSkills') }}</li>
              </ul>
            </div>
          </template>
          <template v-else>
            <p>{{ t('skillStore.empty.noLocal') }}</p>
            <p class="empty-hint">{{ t('skillStore.empty.localHint') }}</p>
          </template>
        </div>
      </template>
    </div>

    <!-- Store Skills Grid -->
    <div v-else class="skills-grid">
      <div v-if="skillStore.refreshing" class="loading-state">
        <div class="spinner"></div>
        <span>{{ t('skillStore.empty.loadingStore') }}</span>
      </div>
      <template v-else>
        <div
          v-for="skill in filteredRemoteSkills"
          :key="skill.id"
          :class="['skill-card', 'store-card', { installed: skill.installed }]"
        >
        <div class="skill-header">
          <span class="skill-icon">{{ getCategoryIcon(skill.category) }}</span>
          <div class="skill-title">
            <h3 :title="skill.name" v-html="highlightText(skill.name, debouncedSearchQuery)"></h3>
            <div class="skill-title-meta">
              <span v-if="skill.version && skill.version !== 'latest'" class="skill-version">v{{ skill.version }}</span>
              <span class="source-badge">{{ skill.source_name }}</span>
            </div>
          </div>
        </div>

        <p class="skill-description" v-html="highlightText(skill.description, debouncedSearchQuery)"></p>

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
            :class="{ retrying: (installRetryCount[skill.id] ?? 0) > 0 }"
            :disabled="installingSkillId === skill.id"
            @click="installSkill(skill)"
          >
            <span v-if="installingSkillId === skill.id" class="spinner"></span>
            {{ getInstallButtonText(skill.id) }}
          </button>
          <span v-else class="installed-badge">{{ t('skillStore.status.installed') }}</span>
          <a v-if="skill.homepage" :href="skill.homepage" target="_blank" class="btn-link">
            {{ t('skillStore.actions.details') }}
          </a>
        </div>
      </div>

      <div v-if="filteredRemoteSkills.length === 0 && !skillStore.refreshing" class="empty-state">
        <template v-if="debouncedSearchQuery">
          <p class="no-results-title">{{ t('skillStore.empty.noResultsTitle', { query: debouncedSearchQuery }) }}</p>
          <div class="suggestions">
            <p class="suggestions-label">{{ t('skillStore.empty.suggestions') }}:</p>
            <ul>
              <li>{{ t('skillStore.empty.tryDifferentKeywords') }}</li>
              <li>{{ t('skillStore.empty.checkSpelling') }}</li>
              <li v-if="selectedCategory" class="suggestion-link" @click="selectedCategory = ''">{{ t('skillStore.empty.tryClearFilters') }}</li>
              <li class="suggestion-link" @click="showInstallURLModal = true">{{ t('skillStore.empty.installFromUrl') }}</li>
            </ul>
          </div>
        </template>
        <p v-else>{{ t('skillStore.empty.noMatchingStore') }}</p>
      </div>
      </template>
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

    <!-- Install from URL Modal -->
    <div v-if="showInstallURLModal" class="modal-overlay" @click.self="showInstallURLModal = false">
      <div class="modal">
        <div class="modal-header">
          <h2>{{ t('skillStore.modal.installFromURLTitle') }}</h2>
          <button class="modal-close" @click="showInstallURLModal = false">×</button>
        </div>
        <div class="modal-body">
          <p class="modal-description">{{ t('skillStore.modal.installFromURLDescription') }}</p>
          <div class="form-group">
            <label>{{ t('skillStore.modal.skillURL') }}</label>
            <input
              v-model="installURL"
              type="text"
              :placeholder="t('skillStore.modal.skillURLPlaceholder')"
            />
          </div>
          <div class="form-group">
            <label>{{ t('skillStore.modal.skillNameOptional') }}</label>
            <input
              v-model="installURLName"
              type="text"
              :placeholder="t('skillStore.modal.skillNamePlaceholder')"
            />
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn-cancel" @click="showInstallURLModal = false">{{ t('skillStore.modal.cancel') }}</button>
          <button class="btn-confirm" :disabled="!installURL || isInstallingFromURL" @click="installFromURL">
            <span v-if="isInstallingFromURL" class="spinner"></span>
            {{ isInstallingFromURL ? t('skillStore.modal.installing') : t('skillStore.modal.install') }}
          </button>
        </div>
      </div>
    </div>

    <!-- Uninstall Confirmation Modal -->
    <div v-if="showUninstallConfirm" class="modal-overlay" @click.self="cancelUninstall">
      <div class="modal modal-confirm">
        <div class="modal-header">
          <h2>{{ t('skillStore.modal.confirmUninstallTitle') }}</h2>
          <button class="modal-close" @click="cancelUninstall">×</button>
        </div>
        <div class="modal-body">
          <p class="confirm-message">
            {{ t('skillStore.modal.confirmUninstallMessage', { name: skillToUninstall?.name }) }}
          </p>
        </div>
        <div class="modal-footer">
          <button class="btn-cancel" @click="cancelUninstall">{{ t('skillStore.modal.cancel') }}</button>
          <button class="btn-danger" @click="uninstallSkill(skillToUninstall!)">{{ t('skillStore.modal.uninstall') }}</button>
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

.tab-spinner {
  display: inline-block;
  width: 12px;
  height: 12px;
  border: 2px solid currentColor;
  border-top-color: transparent;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
  margin-left: 6px;
  vertical-align: middle;
  opacity: 0.7;
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

/* Search Results Summary */
.search-results-summary {
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 12px 16px;
  margin-bottom: 16px;
}

.search-results-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.search-results-header h3 {
  margin: 0;
  font-size: 14px;
  font-weight: 500;
  color: var(--text-primary);
}

.btn-clear-search {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 6px 12px;
  border: none;
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  font-size: 13px;
  border-radius: 4px;
  transition: all 0.2s;
}

.btn-clear-search:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
}

.btn-clear-search svg {
  width: 14px;
  height: 14px;
}

.search-results-breakdown {
  display: flex;
  gap: 16px;
  margin-top: 8px;
  flex-wrap: wrap;
}

.result-source {
  font-size: 13px;
  color: var(--primary);
  cursor: pointer;
  padding: 4px 8px;
  background: rgba(59, 130, 246, 0.1);
  border-radius: 4px;
  transition: all 0.2s;
}

.result-source:hover {
  background: rgba(59, 130, 246, 0.2);
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

.featured-badge {
  padding: 2px 6px;
  border-radius: var(--radius-full, 9999px);
  font-size: 10px;
  font-weight: 500;
  background: rgba(168, 85, 247, 0.15);
  color: #c4b5fd;
  border: 1px solid rgba(168, 85, 247, 0.3);
}

.featured-card::before {
  background: linear-gradient(90deg, #a855f7, #ec4899);
}

.local-card::before {
  background: linear-gradient(90deg, #06b6d4, #3b82f6);
}

.local-badge {
  padding: 2px 6px;
  border-radius: var(--radius-full, 9999px);
  font-size: 10px;
  font-weight: 500;
  background: rgba(6, 182, 212, 0.15);
  color: #67e8f9;
  border: 1px solid rgba(6, 182, 212, 0.3);
}

.local-ready-badge {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
  padding: 6px;
  color: #67e8f9;
  font-size: 11px;
  font-weight: 500;
  background: rgba(6, 182, 212, 0.15);
  border: 1px solid rgba(6, 182, 212, 0.3);
  border-radius: var(--radius-md, 8px);
}

.btn-scan-local {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 10px 16px;
  border: 1px solid rgba(6, 182, 212, 0.5);
  border-radius: 8px;
  background: transparent;
  color: #67e8f9;
  cursor: pointer;
  font-size: 14px;
  transition: all 0.2s;
}

.btn-scan-local:hover {
  background: rgba(6, 182, 212, 0.15);
}

.btn-scan-local:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.btn-scan-local svg {
  width: 16px;
  height: 16px;
}

.empty-hint {
  font-size: 12px;
  color: var(--text-muted);
  margin-top: 8px;
}

.loading-state {
  grid-column: 1 / -1;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 48px;
  color: var(--text-secondary);
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

:root.light .featured-badge,
[data-theme="light"] .featured-badge {
  background: rgba(168, 85, 247, 0.1);
  color: #7c3aed;
  border-color: rgba(168, 85, 247, 0.2);
}

:root.light .local-badge,
[data-theme="light"] .local-badge {
  background: rgba(6, 182, 212, 0.1);
  color: #0891b2;
  border-color: rgba(6, 182, 212, 0.2);
}

:root.light .local-ready-badge,
[data-theme="light"] .local-ready-badge {
  background: rgba(6, 182, 212, 0.1);
  color: #0891b2;
  border-color: rgba(6, 182, 212, 0.2);
}

:root.light .btn-scan-local,
[data-theme="light"] .btn-scan-local {
  color: #0891b2;
  border-color: rgba(6, 182, 212, 0.4);
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

.btn-verify {
  padding: 0.5rem;
  border: 1px solid rgba(59, 130, 246, 0.3);
  background: transparent;
  color: var(--primary);
  border-radius: 6px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s;
}

.btn-verify svg {
  width: 16px;
  height: 16px;
}

.btn-verify:hover {
  background: rgba(59, 130, 246, 0.15);
}

.btn-verify:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.badge.verification {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  padding: 2px 6px;
}

.badge.verification.verified {
  background: rgba(34, 197, 94, 0.15);
  color: #22c55e;
}

.badge.verification.error {
  background: rgba(239, 68, 68, 0.15);
  color: #ef4444;
}

.badge.verification.pending {
  background: rgba(59, 130, 246, 0.15);
  color: var(--primary);
}

.verify-icon {
  width: 12px;
  height: 12px;
}

.spinner-small {
  width: 12px;
  height: 12px;
  border: 2px solid transparent;
  border-top-color: currentColor;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
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

.btn-install:disabled {
  opacity: 0.7;
  cursor: not-allowed;
  transform: none;
}

.btn-install .spinner {
  width: 14px;
  height: 14px;
  border-width: 2px;
  margin-right: 4px;
}

.btn-install.retrying {
  background: linear-gradient(135deg, #f59e0b, #d97706);
  box-shadow: 0 2px 8px rgba(245, 158, 11, 0.3);
  animation: pulse-retry 1.5s ease-in-out infinite;
}

@keyframes pulse-retry {
  0%, 100% {
    opacity: 1;
  }
  50% {
    opacity: 0.7;
  }
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

.no-results-title {
  font-size: 18px;
  font-weight: 500;
  color: var(--text-primary);
  margin-bottom: 16px;
}

.suggestions {
  display: inline-block;
  text-align: left;
  background: var(--bg-secondary);
  padding: 16px 24px;
  border-radius: 8px;
  margin-top: 8px;
}

.suggestions-label {
  font-weight: 500;
  color: var(--text-primary);
  margin: 0 0 8px 0;
}

.suggestions ul {
  margin: 0;
  padding-left: 20px;
}

.suggestions li {
  margin: 6px 0;
  color: var(--text-secondary);
}

.suggestion-link {
  color: var(--primary);
  cursor: pointer;
  text-decoration: underline;
}

.suggestion-link:hover {
  color: var(--primary-hover);
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
  display: flex;
  align-items: center;
  gap: 6px;
}

.btn-confirm:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.btn-danger {
  padding: 10px 20px;
  border-radius: 8px;
  font-size: 14px;
  cursor: pointer;
  border: none;
  background: var(--error);
  color: white;
}

.btn-danger:hover {
  background: #dc2626;
}

.btn-install-url {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 10px 16px;
  border: 1px solid var(--primary);
  border-radius: 8px;
  background: transparent;
  color: var(--primary);
  cursor: pointer;
  font-size: 14px;
  transition: all 0.2s;
}

.btn-install-url:hover {
  background: var(--primary);
  color: white;
}

.btn-install-url svg {
  width: 16px;
  height: 16px;
}

.modal-description {
  color: var(--text-secondary);
  font-size: 14px;
  margin: 0 0 16px;
  line-height: 1.5;
}

.modal-confirm {
  max-width: 400px;
}

.confirm-message {
  color: var(--text-primary);
  font-size: 14px;
  line-height: 1.6;
  margin: 0;
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

/* Search highlight */
:deep(.search-highlight) {
  background: rgba(59, 130, 246, 0.3);
  color: inherit;
  padding: 0 2px;
  border-radius: 2px;
}

:root.light :deep(.search-highlight),
[data-theme="light"] :deep(.search-highlight) {
  background: rgba(59, 130, 246, 0.2);
}
</style>
