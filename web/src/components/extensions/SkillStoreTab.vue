<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { onSSEEvent, offSSEEvent } from '@/composables/useEventStream'
import { skillApi, type RemoteSkill, type SearchParams, type SyncStatus } from '@/api/skill'

const { t, te } = useI18n()

const skills = ref<RemoteSkill[]>([])
const loading = ref(false)
const loadingMore = ref(false)
const error = ref<string | null>(null)
const initializing = ref(false)
const searchQuery = ref('')
const filterCategory = ref<string>('all')
const sortBy = ref<'downloads' | 'stars' | 'updated' | 'name'>('downloads')
const categories = ref<string[]>([])
const installing = ref<Set<string>>(new Set())
const installProgress = ref<Map<string, number>>(new Map())
const selectedSkillId = ref<string | null>(null)

const syncing = ref(false)
const syncProgress = ref<SyncStatus | null>(null)
let syncPollTimer: ReturnType<typeof setInterval> | null = null

const pageSize = 20
const nextCursor = ref<string | null>(null)
const hasMore = ref(false)

const selectedSkill = computed(() => {
  if (!selectedSkillId.value) return null
  return skills.value.find((s) => s.id === selectedSkillId.value) || null
})

watch(skills, (list) => {
  if (!list.length) {
    selectedSkillId.value = null
    return
  }
  if (!selectedSkillId.value || !list.some((s) => s.id === selectedSkillId.value)) {
    selectedSkillId.value = list[0]!.id
  }
}, { immediate: true })

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
  const firstCategory = category?.split(',')[0]?.trim() || ''
  return icons[firstCategory] || '⚡'
}

function getCategoryLabel(category?: string): string {
  const cat = (category || 'other').trim()
  const key = `plugins.categories.${cat}`
  if (te(key)) return t(key)
  return category || t('plugins.categories.other')
}

function getCategories(category?: string): string[] {
  if (!category) return []
  return category.split(',').map((c) => c.trim()).filter((c) => c)
}

function formatNumber(num?: number): string {
  if (!num) return '0'
  if (num >= 1000) return `${(num / 1000).toFixed(1)}k`
  return num.toString()
}

function formatDate(dateStr?: string): string {
  if (!dateStr) return '-'
  const date = new Date(dateStr)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
}

function selectSkill(skill: RemoteSkill) {
  selectedSkillId.value = skill.id
}

function openSkillHomepage(skill: RemoteSkill) {
  const url = skill.homepage || skill.download_url
  if (url) {
    window.open(url, '_blank', 'noopener,noreferrer')
  }
}

function renderMarkdown(content: string): string {
  if (!content) return ''

  let html = content
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/^### (.+)$/gm, '<h3>$1</h3>')
    .replace(/^## (.+)$/gm, '<h2>$1</h2>')
    .replace(/^# (.+)$/gm, '<h1>$1</h1>')
    .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
    .replace(/\*(.+?)\*/g, '<em>$1</em>')
    .replace(/```(\w*)\n([\s\S]*?)```/g, '<pre><code class="language-$1">$2</code></pre>')
    .replace(/`([^`]+)`/g, '<code>$1</code>')
    .replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" target="_blank" rel="noopener">$1</a>')
    .replace(/^- (.+)$/gm, '<li>$1</li>')
    .replace(/\n\n/g, '</p><p>')

  html = '<p>' + html + '</p>'
  html = html.replace(/(<li>.*?<\/li>)+/gs, '<ul>$&</ul>')
  return html
}

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

    if (response.data.initializing) {
      initializing.value = true
      skills.value = []
      setTimeout(() => fetchSkills(), 3000)
      return
    }

    initializing.value = false

    const skillsData = response.data.skills || []
    const newSkills = skillsData.map((s) => ({
      ...s,
      tags: s.tags ? s.tags.split(',').map((t) => t.trim()) : [],
    })) as unknown as RemoteSkill[]

    if (append) {
      skills.value = [...skills.value, ...newSkills]
    } else {
      skills.value = newSkills
    }
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
    // Ignore category loading failures.
  }
}

async function installSkill(skill: RemoteSkill) {
  if (installing.value.has(skill.id)) return

  installing.value.add(skill.id)
  installProgress.value.set(skill.id, 0)

  try {
    await skillApi.install(skill.id)
    installProgress.value.set(skill.id, 100)
    skill.installed = true
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Failed to install skill'
  } finally {
    installing.value.delete(skill.id)
    setTimeout(() => installProgress.value.delete(skill.id), 800)
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

async function triggerSync() {
  try {
    const res = await skillApi.refresh()
    if (res.data.syncing) {
      syncing.value = true
      const active = res.data.sync_status?.find((s) => s.status === 'in_progress')
      if (active) syncProgress.value = active
      startSyncPolling()
    }
  } catch {
    // Ignore sync trigger failures.
  }
}

function startSyncPolling() {
  stopSyncPolling()
  syncPollTimer = setInterval(async () => {
    try {
      const res = await skillApi.syncStatus()
      const active = res.data.find((s) => s.status === 'in_progress')
      if (active) {
        syncProgress.value = active
      } else {
        syncing.value = false
        syncProgress.value = null
        stopSyncPolling()
        fetchSkills()
      }
    } catch {
      // Ignore polling failures.
    }
  }, 2000)
}

function stopSyncPolling() {
  if (syncPollTimer) {
    clearInterval(syncPollTimer)
    syncPollTimer = null
  }
}

function onInstallProgress(data: { id?: string; percent?: number }) {
  if (data.id && typeof data.percent === 'number') {
    installProgress.value.set(data.id, data.percent)
  }
}

function onInstallComplete(data: { id?: string }) {
  if (!data.id) return
  installProgress.value.set(data.id, 100)
  const s = skills.value.find((skill) => skill.id === data.id)
  if (s) s.installed = true
}

function onInstallError(data: { id?: string; error?: string }) {
  if (!data.id) return
  installing.value.delete(data.id)
  installProgress.value.delete(data.id)
  if (data.error) error.value = data.error
}

onMounted(() => {
  fetchSkills()
  fetchCategories()
  triggerSync()

  onSSEEvent('skill.install.progress', onInstallProgress)
  onSSEEvent('skill.install.complete', onInstallComplete)
  onSSEEvent('skill.install.error', onInstallError)
})

onUnmounted(() => {
  stopSyncPolling()
  offSSEEvent('skill.install.progress', onInstallProgress)
  offSSEEvent('skill.install.complete', onInstallComplete)
  offSSEEvent('skill.install.error', onInstallError)
})
</script>

<template>
  <div class="skill-store-tab skill-store-redesign">
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
          {{ getCategoryIcon(cat) }} {{ getCategoryLabel(cat) }}
        </option>
      </select>

      <select v-model="sortBy" class="filter-select" @change="handleSearch">
        <option value="downloads">{{ t('skillStore.sort.downloads') }}</option>
        <option value="stars">{{ t('skillStore.sort.stars') }}</option>
        <option value="updated">{{ t('skillStore.sort.updated') }}</option>
        <option value="name">{{ t('skillStore.sort.name') }}</option>
      </select>

      <button class="btn-refresh" :disabled="loading" @click="fetchSkills()">
        <svg v-if="!loading" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M21 12a9 9 0 0 0-9-9 9.75 9.75 0 0 0-6.74 2.74L3 8" />
          <path d="M3 3v5h5" />
          <path d="M3 12a9 9 0 0 0 9 9 9.75 9.75 0 0 0 6.74-2.74L21 16" />
          <path d="M16 21h5v-5" />
        </svg>
        <span v-else class="spinner"></span>
      </button>
    </div>

    <div v-if="syncing" class="sync-banner">
      <div class="sync-banner-content">
        <div class="sync-spinner"></div>
        <span class="sync-label">{{ t('skillStore.status.syncing') }}</span>
        <span v-if="syncProgress?.progress" class="sync-detail">
          {{ t('skillStore.status.skillsSynced', { count: syncProgress.progress.skills_synced }) }}
        </span>
      </div>
    </div>

    <div v-if="error" class="error-banner">
      {{ error }}
      <button @click="error = null">×</button>
    </div>

    <div v-if="initializing && !skills.length" class="initializing-state">
      <svg class="initializing-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
        <path d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4" />
      </svg>
      <div class="initializing-spinner"></div>
      <h3>{{ t('skillStore.status.initializing') }}</h3>
      <p>{{ t('skillStore.status.initializingDesc') }}</p>
    </div>

    <div v-else-if="loading && !skills.length" class="loading">
      <div class="spinner"></div>
      <span>{{ t('common.loading') }}</span>
    </div>

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

    <div v-else class="store-layout">
      <div class="items-grid-container store-list-panel" @scroll="handleScroll">
        <div class="items-grid store-grid">
          <article
            v-for="skill in skills"
            :key="skill.id"
            :class="['item-card', 'store-card', { installed: skill.installed, active: selectedSkillId === skill.id }]"
            tabindex="0"
            role="button"
            @click="selectSkill(skill)"
            @keydown.enter.prevent="selectSkill(skill)"
            @keydown.space.prevent="selectSkill(skill)"
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

            <p class="item-description" :title="skill.description || skill.summary">
              {{ skill.description || skill.summary || t('plugins.noDescription') }}
            </p>

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
            </div>

            <div v-if="getCategories(skill.category).length" class="item-categories">
              <span
                v-for="cat in getCategories(skill.category)"
                :key="cat"
                class="category-badge"
              >
                {{ getCategoryLabel(cat) }}
              </span>
            </div>

            <div class="item-footer">
              <button
                v-if="!skill.installed"
                :class="['btn-install', { 'btn-installing': installing.has(skill.id) }]"
                :disabled="installing.has(skill.id)"
                @click.stop="installSkill(skill)"
              >
                <template v-if="installing.has(skill.id)">
                  <div class="install-progress-bar">
                    <div class="install-progress-fill" :style="{ width: (installProgress.get(skill.id) || 0) + '%' }"></div>
                  </div>
                  <span class="install-progress-text">{{ installProgress.get(skill.id) || 0 }}%</span>
                </template>
                <span v-else>{{ t('skillStore.install') }}</span>
              </button>
              <span v-else class="installed-badge">{{ t('skillStore.installed') }}</span>

              <button class="btn-link-small" @click.stop="openSkillHomepage(skill)">
                {{ t('skillStore.detail.openLink') }}
              </button>
            </div>
          </article>
        </div>

        <div v-if="loadingMore" class="loading-more">
          <div class="spinner-small"></div>
          <span>{{ t('common.loading') }}</span>
        </div>

        <div v-if="skills.length > 0 && !hasMore && !loadingMore" class="load-more-info">
          {{ t('skillStore.allLoaded', { count: skills.length }) }}
        </div>
      </div>

      <aside class="store-detail-panel">
        <div v-if="selectedSkill" class="store-detail-content">
          <div class="detail-header">
            <div class="detail-title-row">
              <span class="detail-icon">{{ getCategoryIcon(selectedSkill.category) }}</span>
              <div>
                <h2>{{ selectedSkill.name }}</h2>
                <p>{{ selectedSkill.summary || selectedSkill.description || t('plugins.noDescription') }}</p>
              </div>
            </div>
            <span class="source-badge">{{ selectedSkill.source_name || selectedSkill.source_id }}</span>
          </div>

          <div class="detail-actions">
            <button
              v-if="!selectedSkill.installed"
              :class="['btn-install', { 'btn-installing': installing.has(selectedSkill.id) }]"
              :disabled="installing.has(selectedSkill.id)"
              @click="installSkill(selectedSkill)"
            >
              <template v-if="installing.has(selectedSkill.id)">
                <div class="install-progress-bar">
                  <div class="install-progress-fill" :style="{ width: (installProgress.get(selectedSkill.id) || 0) + '%' }"></div>
                </div>
                <span class="install-progress-text">{{ installProgress.get(selectedSkill.id) || 0 }}%</span>
              </template>
              <span v-else>{{ t('skillStore.install') }}</span>
            </button>
            <span v-else class="installed-badge">{{ t('skillStore.installed') }}</span>

            <button class="btn-link-light" @click="openSkillHomepage(selectedSkill)">
              {{ t('skillStore.detail.openLink') }}
            </button>
          </div>

          <div class="detail-meta-grid">
            <div class="meta-entry">
              <span>{{ t('skillStore.detail.meta.author') }}</span>
              <strong>{{ selectedSkill.author || '-' }}</strong>
            </div>
            <div class="meta-entry">
              <span>{{ t('skillStore.detail.meta.version') }}</span>
              <strong>{{ selectedSkill.version || '-' }}</strong>
            </div>
            <div class="meta-entry">
              <span>{{ t('skillStore.detail.meta.updated') }}</span>
              <strong>{{ formatDate(selectedSkill.updated_at || selectedSkill.synced_at) }}</strong>
            </div>
            <div class="meta-entry">
              <span>{{ t('skillStore.detail.meta.downloads') }}</span>
              <strong>{{ formatNumber(selectedSkill.downloads) }}</strong>
            </div>
            <div class="meta-entry">
              <span>{{ t('skillStore.detail.meta.stars') }}</span>
              <strong>{{ formatNumber(selectedSkill.stars) }}</strong>
            </div>
            <div class="meta-entry">
              <span>{{ t('skillStore.detail.meta.rating') }}</span>
              <strong>{{ selectedSkill.rating ? selectedSkill.rating.toFixed(1) : '-' }}</strong>
            </div>
          </div>

          <section class="detail-section" v-if="selectedSkill.description">
            <h4>{{ t('skillStore.detail.sections.description') }}</h4>
            <p>{{ selectedSkill.description }}</p>
          </section>

          <section class="detail-section" v-if="selectedSkill.readme">
            <h4>{{ t('skillStore.detail.sections.readme') }}</h4>
            <div class="skill-content markdown-body" v-html="renderMarkdown(selectedSkill.readme)"></div>
          </section>

          <section class="detail-section" v-else-if="selectedSkill.changelog">
            <h4>{{ t('skillStore.detail.sections.changelog') }}</h4>
            <div class="skill-content markdown-body" v-html="renderMarkdown(selectedSkill.changelog)"></div>
          </section>

          <section class="detail-section" v-else>
            <h4>{{ t('skillStore.detail.sections.details') }}</h4>
            <p>{{ t('skillStore.detail.noDetails') }}</p>
          </section>
        </div>

        <div v-else class="detail-empty">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
            <path d="M8 6h13M8 12h13M8 18h13M3 6h.01M3 12h.01M3 18h.01" />
          </svg>
          <h3>{{ t('skillStore.detail.emptyTitle') }}</h3>
          <p>{{ t('skillStore.detail.emptyDescription') }}</p>
        </div>
      </aside>
    </div>
  </div>
</template>

<style scoped>
@import './extension-tab.css';

.skill-store-redesign {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.store-layout {
  display: grid;
  grid-template-columns: minmax(0, 1.3fr) minmax(340px, 1fr);
  gap: 16px;
  align-items: start;
}

.store-list-panel {
  max-height: calc(100vh - 250px);
  overflow: auto;
  padding-right: 4px;
}

.store-grid {
  grid-template-columns: repeat(auto-fill, minmax(250px, 1fr));
}

.store-card {
  outline: none;
}

.store-card.active {
  border-color: rgba(59, 130, 246, 0.45);
  box-shadow: 0 0 0 1px rgba(59, 130, 246, 0.35), 0 10px 22px rgba(59, 130, 246, 0.18);
}

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
  color: var(--color-gray-900);
}

.item-stats .stat:nth-child(2) svg,
.item-stats .stat:nth-child(3) svg {
  color: #fbbf24;
}

.item-categories {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-top: 8px;
}

.item-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid var(--color-border);
}

.item-description {
  -webkit-line-clamp: 5;
  min-height: 95px;
}

.btn-install {
  padding: 6px 12px;
  font-size: 12px;
  font-weight: 500;
  color: white;
  background: #374151;
  border: none;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s;
  min-width: 76px;
  position: relative;
  overflow: hidden;
}

.btn-install.btn-installing {
  padding: 6px 8px;
  min-width: 98px;
  display: flex;
  align-items: center;
  gap: 6px;
  background: #1f2937;
}

.install-progress-bar {
  flex: 1;
  height: 4px;
  background: rgba(255, 255, 255, 0.15);
  border-radius: 2px;
  overflow: hidden;
}

.install-progress-fill {
  height: 100%;
  background: #22c55e;
  border-radius: 2px;
  transition: width 0.3s ease;
}

.install-progress-text {
  font-size: 11px;
  font-weight: 600;
  min-width: 28px;
  text-align: right;
  font-variant-numeric: tabular-nums;
}

.btn-install:hover:not(:disabled) {
  background: #4b5563;
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
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.btn-link-small,
.btn-link-light {
  border: 1px solid var(--border);
  background: transparent;
  color: var(--text-primary);
  border-radius: 6px;
  font-size: 12px;
  padding: 6px 8px;
  cursor: pointer;
  transition: all 0.2s;
  white-space: nowrap;
}

.btn-link-small:hover,
.btn-link-light:hover {
  background: var(--bg-hover);
  border-color: var(--primary);
  color: var(--primary);
}

.store-detail-panel {
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--glass-bg, rgba(255, 255, 255, 0.05));
  min-height: 360px;
  max-height: calc(100vh - 250px);
  overflow: auto;
  position: sticky;
  top: 12px;
}

.store-detail-content {
  padding: 14px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.detail-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
}

.detail-title-row {
  display: flex;
  gap: 10px;
  min-width: 0;
}

.detail-icon {
  width: 38px;
  height: 38px;
  border: 1px solid var(--border);
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 18px;
  flex-shrink: 0;
  background: var(--glass-bg, rgba(255, 255, 255, 0.05));
}

.detail-title-row h2 {
  margin: 0;
  font-size: 18px;
  color: var(--text-primary);
}

.detail-title-row p {
  margin: 4px 0 0;
  font-size: 13px;
  color: var(--text-secondary);
  line-height: 1.45;
}

.detail-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.detail-meta-grid {
  display: grid;
  gap: 8px;
  grid-template-columns: repeat(auto-fit, minmax(130px, 1fr));
}

.meta-entry {
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--glass-bg, rgba(255, 255, 255, 0.04));
  padding: 9px 10px;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.meta-entry span {
  font-size: 11px;
  color: var(--text-secondary);
}

.meta-entry strong {
  font-size: 12px;
  color: var(--text-primary);
  word-break: break-word;
}

.detail-section {
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--glass-bg, rgba(255, 255, 255, 0.03));
  padding: 12px;
}

.detail-section h4 {
  margin: 0 0 10px;
  font-size: 14px;
  color: var(--text-primary);
}

.detail-section p {
  margin: 0;
  color: var(--text-secondary);
  font-size: 13px;
  line-height: 1.5;
}

.skill-content {
  line-height: 1.6;
  color: var(--text-primary);
}

.skill-content :deep(h1) {
  font-size: 20px;
  margin: 0 0 12px;
}

.skill-content :deep(h2) {
  font-size: 17px;
  margin: 18px 0 10px;
}

.skill-content :deep(h3) {
  font-size: 15px;
  margin: 14px 0 8px;
}

.skill-content :deep(p) {
  margin: 0 0 10px;
}

.skill-content :deep(ul) {
  margin: 0 0 10px;
  padding-left: 20px;
}

.skill-content :deep(code) {
  background: var(--color-bg-tertiary, var(--color-bg-secondary));
  color: var(--color-code-text, var(--color-text-primary));
  padding: 2px 6px;
  border-radius: 4px;
  font-family: monospace;
  font-size: 12px;
}

.skill-content :deep(pre) {
  background: var(--color-bg-tertiary, var(--color-bg-secondary));
  color: var(--color-text-primary);
  padding: 12px;
  border-radius: 8px;
  overflow-x: auto;
  margin: 10px 0;
}

.markdown-body {
  background: transparent !important;
  color: var(--color-text-primary) !important;
}

.skill-content :deep(a) {
  color: var(--color-gray-900, #3b82f6);
  text-decoration: none;
}

.skill-content :deep(a:hover) {
  text-decoration: underline;
}

.detail-empty {
  min-height: 280px;
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  text-align: center;
  gap: 8px;
  padding: 18px;
  color: var(--text-secondary);
}

.detail-empty svg {
  width: 44px;
  height: 44px;
  opacity: 0.3;
}

.detail-empty h3 {
  margin: 0;
  color: var(--text-primary);
  font-size: 15px;
}

.detail-empty p {
  margin: 0;
  font-size: 13px;
}

.author {
  color: var(--color-text-secondary);
  font-size: 12px;
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
  background: #374151;
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

.loading-more {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 16px;
  color: var(--color-text-secondary);
}

.spinner-small {
  width: 14px;
  height: 14px;
  border: 2px solid var(--color-border);
  border-top-color: var(--color-text-primary);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

.load-more-info {
  text-align: center;
  color: var(--color-text-secondary);
  font-size: 12px;
  padding: 12px 0 2px;
}

.initializing-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 60px 20px;
  text-align: center;
  color: var(--color-text-secondary);
}

.initializing-icon {
  width: 72px;
  height: 72px;
  margin-bottom: 16px;
  opacity: 0.25;
}

.initializing-spinner {
  width: 24px;
  height: 24px;
  border: 3px solid rgba(107, 114, 128, 0.2);
  border-top-color: var(--color-text-secondary);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
  margin-bottom: 16px;
}

.initializing-state h3 {
  font-size: 16px;
  font-weight: 600;
  margin-bottom: 6px;
  color: var(--color-text-primary);
}

.initializing-state p {
  font-size: 13px;
}

.sync-banner {
  margin-bottom: 2px;
  padding: 10px 14px;
  background: rgba(59, 130, 246, 0.08);
  border: 1px solid rgba(59, 130, 246, 0.2);
  border-radius: 8px;
}

.sync-banner-content {
  display: flex;
  align-items: center;
  gap: 10px;
}

.sync-spinner {
  width: 16px;
  height: 16px;
  border: 2px solid rgba(59, 130, 246, 0.25);
  border-top-color: #3b82f6;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
  flex-shrink: 0;
}

.sync-label {
  font-size: 13px;
  font-weight: 500;
  color: var(--color-text-primary);
}

.sync-detail {
  font-size: 12px;
  color: var(--color-text-secondary);
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

@media (max-width: 1180px) {
  .store-layout {
    grid-template-columns: 1fr;
  }

  .store-list-panel,
  .store-detail-panel {
    max-height: none;
    position: static;
  }
}
</style>
