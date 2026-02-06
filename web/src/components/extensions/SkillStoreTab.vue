<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { skillApi, type RemoteSkill, type SearchParams } from '@/api/skill'

const { t } = useI18n()

const skills = ref<RemoteSkill[]>([])
const loading = ref(false)
const loadingMore = ref(false)
const error = ref<string | null>(null)
const searchQuery = ref('')
const filterCategory = ref<string>('all')
const sortBy = ref<'downloads' | 'rating' | 'updated' | 'name'>('downloads')
const categories = ref<string[]>([])
const installing = ref<Set<string>>(new Set())

// Infinite scroll state
const pageSize = 20
const total = ref(0)
const nextCursor = ref<string | null>(null)
const hasMore = ref(false)
const scrollContainer = ref<HTMLElement | null>(null)

async function fetchSkills(append = false) {
  if (append) {
    if (!hasMore.value || loadingMore.value) return
    loadingMore.value = true
  } else {
    loading.value = true
    skills.value = []
    nextCursor.value = null
  }
  error.value = null

  try {
    const params: SearchParams = {
      count: pageSize,
      sort_by: sortBy.value,
      sort_order: sortBy.value === 'name' ? 'asc' : 'desc',
    }
    if (append && nextCursor.value) {
      params.cursor = nextCursor.value
    }
    if (searchQuery.value) {
      params.q = searchQuery.value
    }
    if (filterCategory.value !== 'all') {
      params.categories = filterCategory.value
    }
    const response = await skillApi.search(params)

    // Handle null or undefined skills array
    const skillsData = response.data.skills || []
    const newSkills = skillsData.map(s => ({
      ...s,
      tags: s.tags ? s.tags.split(',').map(t => t.trim()) : [],
    })) as unknown as RemoteSkill[]

    if (append) {
      skills.value = [...skills.value, ...newSkills]
    } else {
      skills.value = newSkills
    }
    total.value = response.data.total || 0
    nextCursor.value = response.data.next_cursor || null
    hasMore.value = response.data.has_more || false
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Failed to fetch skills'
  } finally {
    loading.value = false
    loadingMore.value = false
  }
}

async function fetchCategories() {
  try {
    const response = await skillApi.categories()
    categories.value = response.data
  } catch {
    // Ignore
  }
}

async function installSkill(skill: RemoteSkill) {
  if (installing.value.has(skill.id)) return
  installing.value.add(skill.id)
  try {
    await skillApi.install(skill.id)
    skill.installed = true
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Failed to install skill'
  } finally {
    installing.value.delete(skill.id)
  }
}

function handleSearch() {
  fetchSkills()
}

function handleScroll(e: Event) {
  const target = e.target as HTMLElement
  const scrollBottom = target.scrollHeight - target.scrollTop - target.clientHeight
  if (scrollBottom < 200 && hasMore.value && !loadingMore.value) {
    fetchSkills(true)
  }
}

function getCategoryIcon(category?: string): string {
  const icons: Record<string, string> = {
    integration: '🔗',
    productivity: '📊',
    development: '💻',
    analytics: '📈',
    extension: '🧩',
    utility: '🔧',
    system: '💻',
    communication: '💬',
    information: '📰',
  }
  // Handle multi-category (comma-separated), use first category for icon
  const firstCategory = category?.split(',')[0]?.trim() || ''
  return icons[firstCategory] || '⚡'
}

function getCategories(category?: string): string[] {
  if (!category) return []
  return category.split(',').map(c => c.trim()).filter(c => c)
}

function formatNumber(num?: number): string {
  if (!num) return '0'
  if (num >= 1000) return (num / 1000).toFixed(1) + 'k'
  return num.toString()
}

function _filterVersionTags(tags: string[]): string[] {
  // Filter out version-like tags (e.g., "1.0.0", "v1.0.0", "1.0.1")
  const versionPattern = /^v?\d+\.\d+(\.\d+)?$/
  return tags.filter(tag => !versionPattern.test(tag))
}

function openSkillHomepage(skill: RemoteSkill) {
  const url = skill.homepage || skill.download_url
  if (url) {
    window.open(url, '_blank', 'noopener,noreferrer')
  }
}

onMounted(() => {
  fetchSkills()
  fetchCategories()
})
</script>

<template>
  <div class="skill-store-tab">
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
          @keyup.enter="handleSearch"
        />
      </div>

      <select v-model="filterCategory" class="filter-select" @change="handleSearch">
        <option value="all">{{ t('plugins.allCategories') }}</option>
        <option v-for="cat in categories" :key="cat" :value="cat">
          {{ getCategoryIcon(cat) }} {{ cat }}
        </option>
      </select>

      <select v-model="sortBy" class="filter-select" @change="handleSearch">
        <option value="downloads">{{ t('skillStore.sort.downloads') }}</option>
        <option value="rating">{{ t('skillStore.sort.rating') }}</option>
        <option value="updated">{{ t('skillStore.sort.updated') }}</option>
        <option value="name">{{ t('skillStore.sort.name') }}</option>
      </select>

      <button class="btn-refresh" :disabled="loading" @click="fetchSkills">
        <svg v-if="!loading" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M21 12a9 9 0 0 0-9-9 9.75 9.75 0 0 0-6.74 2.74L3 8" />
          <path d="M3 3v5h5" />
          <path d="M3 12a9 9 0 0 0 9 9 9.75 9.75 0 0 0 6.74-2.74L21 16" />
          <path d="M16 21h5v-5" />
        </svg>
        <span v-else class="spinner"></span>
      </button>
    </div>

    <!-- Error message -->
    <div v-if="error" class="error-banner">
      {{ error }}
      <button @click="error = null">×</button>
    </div>

    <!-- Loading -->
    <div v-if="loading && !skills.length" class="loading">
      <div class="spinner"></div>
      <span>{{ t('common.loading') }}</span>
    </div>

    <!-- Empty State -->
    <div v-else-if="!loading && skills.length === 0" class="empty-state">
      <svg class="empty-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <circle cx="11" cy="11" r="8" />
        <path d="m21 21-4.35-4.35" />
      </svg>
      <h3>{{ t('skillStore.noResults') }}</h3>
      <p>{{ searchQuery ? t('skillStore.noResultsForQuery') : t('skillStore.noSkillsAvailable') }}</p>
      <button v-if="searchQuery" class="btn-clear-search" @click="searchQuery = ''; handleSearch()">
        {{ t('skillStore.clearSearch') }}
      </button>
    </div>

    <!-- Skills Grid with infinite scroll -->
    <div v-else ref="scrollContainer" class="items-grid-container" @scroll="handleScroll">
      <div class="items-grid">
        <div
          v-for="skill in skills"
          :key="skill.id"
          :class="['item-card', { installed: skill.installed }]"
          @click="openSkillHomepage(skill)"
        >
          <div class="item-header">
            <span class="item-icon">{{ getCategoryIcon(skill.category) }}</span>
            <div class="item-title">
              <h3 :title="skill.name">{{ skill.name }}</h3>
              <div class="item-title-meta">
                <span v-if="skill.version" class="item-version">v{{ skill.version }}</span>
                <span v-if="skill.author" class="author">{{ skill.author }}</span>
              </div>
            </div>
          </div>

          <p class="item-description" :title="skill.description || skill.summary">{{ skill.description || skill.summary || t('plugins.noDescription') }}</p>

          <div class="item-stats">
            <span class="stat" :title="t('skillStore.downloads')">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
                <polyline points="7,10 12,15 17,10" />
                <line x1="12" y1="15" x2="12" y2="3" />
              </svg>
              {{ formatNumber(skill.downloads) }}
            </span>
            <span v-if="skill.stars" class="stat" :title="t('skillStore.stars')">
              <svg viewBox="0 0 24 24" fill="currentColor">
                <path d="M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z" />
              </svg>
              {{ formatNumber(skill.stars) }}
            </span>
            <span v-if="skill.rating" class="stat" :title="t('skillStore.rating')">
              <svg viewBox="0 0 24 24" fill="currentColor">
                <path d="M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z" />
              </svg>
              {{ skill.rating.toFixed(1) }}
            </span>
            <span v-if="skill.reviews" class="stat" :title="t('skillStore.reviews')">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z" />
              </svg>
              {{ formatNumber(skill.reviews) }}
            </span>
          </div>

          <div v-if="getCategories(skill.category).length" class="item-categories">
            <span v-for="cat in getCategories(skill.category)" :key="cat" class="category-badge">{{ cat }}</span>
          </div>

          <div class="item-footer">
            <button
              v-if="!skill.installed"
              class="btn-install"
              :disabled="installing.has(skill.id)"
              @click.stop="installSkill(skill)"
            >
              <span v-if="installing.has(skill.id)" class="spinner-small"></span>
              <span v-else>{{ t('skillStore.install') }}</span>
            </button>
            <span v-else class="installed-badge">{{ t('skillStore.installed') }}</span>
          </div>
        </div>

        <div v-if="skills.length === 0 && !loading" class="empty-state">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
            <path d="M9.75 9.75l4.5 4.5m0-4.5l-4.5 4.5M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          <p>{{ t('skillStore.noSkillsFound') }}</p>
        </div>
      </div>

      <!-- Loading more indicator -->
      <div v-if="loadingMore" class="loading-more">
        <div class="spinner-small"></div>
        <span>{{ t('common.loading') }}</span>
      </div>

      <!-- Load more info -->
      <div v-if="skills.length > 0 && !hasMore && !loadingMore" class="load-more-info">
        {{ t('skillStore.allLoaded', { count: skills.length }) }}
      </div>
    </div>
  </div>
</template>

<style scoped>
@import './extension-tab.css';

.item-stats {
  display: flex;
  gap: 12px;
  margin-top: 8px;
  font-size: 12px;
  color: var(--color-text-secondary);
}

.item-stats .stat {
  display: flex;
  align-items: center;
  gap: 4px;
}

.item-stats svg {
  width: 14px;
  height: 14px;
}

.item-stats .stat:first-child svg {
  color: var(--color-accent);
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 60px 20px;
  text-align: center;
  color: var(--color-text-secondary);
}

.empty-icon {
  width: 64px;
  height: 64px;
  margin-bottom: 16px;
  opacity: 0.3;
}

.empty-state h3 {
  font-size: 18px;
  font-weight: 600;
  margin-bottom: 8px;
  color: var(--color-text-primary);
}

.empty-state p {
  font-size: 14px;
  margin-bottom: 16px;
}

.btn-clear-search {
  padding: 8px 16px;
  background: var(--color-accent);
  color: white;
  border: none;
  border-radius: 6px;
  cursor: pointer;
  font-size: 14px;
  transition: opacity 0.2s;
}

.btn-clear-search:hover {
  opacity: 0.9;
}

.item-stats .stat:nth-child(2) svg {
  color: #fbbf24;
}

.item-footer {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid var(--color-border);
}

/* Override description to show more lines */
.item-description {
  -webkit-line-clamp: 5;
  min-height: 95px;
}

.btn-install {
  padding: 6px 12px;
  font-size: 12px;
  font-weight: 500;
  color: white;
  background: var(--color-accent);
  border: none;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s;
}

.btn-install:hover:not(:disabled) {
  background: var(--color-accent-hover);
}

.btn-install:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.installed-badge {
  padding: 4px 8px;
  font-size: 11px;
  font-weight: 500;
  color: #22c55e;
  background: rgba(34, 197, 94, 0.1);
  border-radius: 4px;
}

.item-card.installed {
  border-color: rgba(34, 197, 94, 0.3);
}

.author {
  color: var(--color-text-secondary);
  font-size: 12px;
}

.version {
  color: var(--color-text-tertiary);
  font-size: 11px;
}

.item-categories {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-top: 8px;
}

.category-badge {
  padding: 2px 8px;
  font-size: 11px;
  font-weight: 500;
  color: var(--color-accent);
  background: rgba(var(--color-accent-rgb, 59, 130, 246), 0.1);
  border-radius: 4px;
}

.spinner-small {
  width: 12px;
  height: 12px;
  border: 2px solid rgba(255, 255, 255, 0.3);
  border-top-color: white;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

.items-grid-container {
  max-height: calc(100vh - 280px);
  overflow-y: auto;
  padding-right: 8px;
}

.loading-more {
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 8px;
  padding: 16px;
  color: var(--color-text-secondary);
  font-size: 13px;
}

.load-more-info {
  text-align: center;
  padding: 16px;
  color: var(--color-text-secondary);
  font-size: 13px;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}
</style>
