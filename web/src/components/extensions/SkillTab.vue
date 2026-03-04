<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { skillApi, type Skill } from '@/api/skill'
import { useSkillStore } from '@/stores/skill'
import { parseFrontmatter } from '@/utils/frontmatter'

const { t, te } = useI18n()
const skillStore = useSkillStore()

const searchQuery = ref('')
const filterCategory = ref<string>('all')
const filterStatus = ref<'all' | 'enabled' | 'disabled'>('all')

const selectedSkillId = ref<string | null>(null)
const contentBySkillId = ref<Record<string, string>>({})
const contentLoadingIds = ref<Set<string>>(new Set())

function getSkillName(skill: Skill): string {
  if (skill.builtin && te(`skills.builtin.${skill.id}.name`)) {
    return t(`skills.builtin.${skill.id}.name`)
  }
  return skill.name
}

function getSkillDescription(skill: Skill): string {
  if (skill.builtin && te(`skills.builtin.${skill.id}.description`)) {
    return t(`skills.builtin.${skill.id}.description`)
  }
  return skill.description || ''
}

function getSkillIconUrl(icon?: string): string | null {
  if (!icon) return null
  return `/icons/skills/${icon}.svg`
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
  return icons[category || ''] || '⚡'
}

function getCategoryLabel(category?: string): string {
  const cat = category || 'other'
  const key = `plugins.categories.${cat}`
  if (te(key)) return t(key)
  return category || t('plugins.categories.other')
}

const filteredSkills = computed(() => {
  let result = skillStore.skills
  const query = searchQuery.value.trim().toLowerCase()

  if (query) {
    result = result.filter((s) => {
      const name = getSkillName(s).toLowerCase()
      const desc = getSkillDescription(s).toLowerCase()
      const tags = (s.tags || []).join(' ').toLowerCase()
      const id = s.id.toLowerCase()
      return name.includes(query) || desc.includes(query) || tags.includes(query) || id.includes(query)
    })
  }

  if (filterCategory.value !== 'all') {
    result = result.filter((s) => (s.category || 'other') === filterCategory.value)
  }

  if (filterStatus.value === 'enabled') {
    result = result.filter((s) => s.enabled)
  } else if (filterStatus.value === 'disabled') {
    result = result.filter((s) => !s.enabled)
  }

  return [...result].sort((a, b) => getSkillName(a).toLowerCase().localeCompare(getSkillName(b).toLowerCase()))
})

const selectedSkill = computed(() => {
  if (!selectedSkillId.value) return null
  return filteredSkills.value.find((s) => s.id === selectedSkillId.value)
    || skillStore.skills.find((s) => s.id === selectedSkillId.value)
    || null
})

const selectedSkillContent = computed(() => {
  if (!selectedSkill.value) return ''
  return contentBySkillId.value[selectedSkill.value.id] || ''
})

const selectedSkillContentParsed = computed(() => parseFrontmatter(selectedSkillContent.value))
const selectedSkillDocContent = computed(() => selectedSkillContentParsed.value.body)
const selectedSkillFrontmatter = computed(() => selectedSkillContentParsed.value.entries)
const frontmatterLabel = computed(() => (te('skills.detail.sections.frontmatter') ? t('skills.detail.sections.frontmatter') : 'Frontmatter'))

const selectedSkillContentLoading = computed(() => {
  if (!selectedSkill.value) return false
  return contentLoadingIds.value.has(selectedSkill.value.id)
})

const skillStats = computed(() => ({
  total: skillStore.skills.length,
  enabled: skillStore.enabledSkills.length,
  disabled: skillStore.skills.length - skillStore.enabledSkills.length,
  builtin: skillStore.skills.filter((s) => s.builtin).length,
  categories: skillStore.categories.length,
}))

watch(filteredSkills, (list) => {
  if (!list.length) {
    selectedSkillId.value = null
    return
  }
  if (!selectedSkillId.value || !list.some((s) => s.id === selectedSkillId.value)) {
    void selectSkill(list[0]!)
  }
}, { immediate: true })

onMounted(async () => {
  await skillStore.fetchSkills()
})

function setLoading(skillId: string, loading: boolean) {
  const next = new Set(contentLoadingIds.value)
  if (loading) next.add(skillId)
  else next.delete(skillId)
  contentLoadingIds.value = next
}

async function ensureSkillContent(skillId: string) {
  if (contentBySkillId.value[skillId] !== undefined || contentLoadingIds.value.has(skillId)) return

  setLoading(skillId, true)
  try {
    const response = await skillApi.getContent(skillId)
    contentBySkillId.value = {
      ...contentBySkillId.value,
      [skillId]: response.data.content || '',
    }
  } catch {
    contentBySkillId.value = {
      ...contentBySkillId.value,
      [skillId]: '',
    }
  } finally {
    setLoading(skillId, false)
  }
}

async function selectSkill(skill: Skill) {
  selectedSkillId.value = skill.id
  await ensureSkillContent(skill.id)
}

async function handleToggle(skill: Skill) {
  if (skill.enabled) {
    await skillStore.disableSkill(skill.id)
  } else {
    await skillStore.enableSkill(skill.id)
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
</script>

<template>
  <div class="skill-tab skill-tab-redesign">
    <div class="overview-grid">
      <div class="overview-item">
        <span class="overview-value">{{ skillStats.total }}</span>
        <span class="overview-label">{{ t('plugins.stats.total') }}</span>
      </div>
      <div class="overview-item">
        <span class="overview-value good">{{ skillStats.enabled }}</span>
        <span class="overview-label">{{ t('plugins.stats.enabled') }}</span>
      </div>
      <div class="overview-item">
        <span class="overview-value muted">{{ skillStats.disabled }}</span>
        <span class="overview-label">{{ t('plugins.stats.disabled') }}</span>
      </div>
      <div class="overview-item">
        <span class="overview-value">{{ skillStats.builtin }}</span>
        <span class="overview-label">{{ t('skills.detail.labels.builtin') }}</span>
      </div>
      <div class="overview-item">
        <span class="overview-value">{{ skillStats.categories }}</span>
        <span class="overview-label">{{ t('skills.detail.labels.categories') }}</span>
      </div>
    </div>

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
        <option value="all">{{ t('plugins.allCategories') }}</option>
        <option v-for="cat in skillStore.categories" :key="cat" :value="cat">
          {{ getCategoryIcon(cat) }} {{ getCategoryLabel(cat) }}
        </option>
      </select>

      <select v-model="filterStatus" class="filter-select">
        <option value="all">{{ t('skills.filters.allStatus') }}</option>
        <option value="enabled">{{ t('common.enabled') }}</option>
        <option value="disabled">{{ t('common.disabled') }}</option>
      </select>

      <button class="btn-refresh" :disabled="skillStore.loading" @click="skillStore.fetchSkills()">
        <svg v-if="!skillStore.loading" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M21 12a9 9 0 0 0-9-9 9.75 9.75 0 0 0-6.74 2.74L3 8" />
          <path d="M3 3v5h5" />
          <path d="M3 12a9 9 0 0 0 9 9 9.75 9.75 0 0 0 6.74-2.74L21 16" />
          <path d="M16 21h5v-5" />
        </svg>
        <span v-else class="spinner"></span>
      </button>
    </div>

    <div v-if="skillStore.error" class="error-banner">
      {{ skillStore.error }}
      <button @click="skillStore.clearError">×</button>
    </div>

    <div v-if="skillStore.loading && !filteredSkills.length" class="loading">
      <div class="spinner"></div>
      <span>{{ t('common.loading') }}</span>
    </div>

    <div v-else class="skill-layout">
      <div class="skill-list">
        <div v-if="filteredSkills.length === 0" class="empty-state list-empty">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
            <path d="M9.75 9.75l4.5 4.5m0-4.5l-4.5 4.5M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          <h3>{{ t('skills.empty.title') }}</h3>
          <p>{{ t('skills.empty.description') }}</p>
        </div>

        <div v-else class="items-grid skill-grid">
          <article
            v-for="skill in filteredSkills"
            :key="skill.id"
            :class="['item-card', 'skill-card', { active: selectedSkillId === skill.id, disabled: !skill.enabled }]"
            tabindex="0"
            role="button"
            @click="selectSkill(skill)"
            @keydown.enter.prevent="selectSkill(skill)"
            @keydown.space.prevent="selectSkill(skill)"
          >
            <div class="item-header">
              <img
                v-if="getSkillIconUrl(skill.icon)"
                :src="getSkillIconUrl(skill.icon)!"
                class="item-icon-svg"
                :alt="getSkillName(skill)"
              />
              <span v-else class="item-icon">{{ getCategoryIcon(skill.category) }}</span>

              <div class="item-title">
                <h3 :title="getSkillName(skill)">{{ getSkillName(skill) }}</h3>
                <div class="item-title-meta">
                  <span v-if="skill.category" class="category-badge">{{ getCategoryLabel(skill.category) }}</span>
                  <span class="builtin-badge">{{ t('plugins.builtin') }}</span>
                  <span class="status-pill" :class="skill.enabled ? 'status-enabled' : 'status-disabled'">
                    {{ skill.enabled ? t('common.enabled') : t('common.disabled') }}
                  </span>
                </div>
              </div>

              <label class="toggle-switch" @click.stop>
                <input
                  type="checkbox"
                  :checked="skill.enabled"
                  :disabled="skillStore.loading"
                  @change="handleToggle(skill)"
                />
                <span class="toggle-slider"></span>
              </label>
            </div>

            <p class="item-description">{{ getSkillDescription(skill) || t('plugins.noDescription') }}</p>

            <div v-if="skill.tags?.length" class="item-tags">
              <span v-for="tag in skill.tags.slice(0, 4)" :key="tag" class="tag">{{ tag }}</span>
            </div>
          </article>
        </div>
      </div>

      <aside class="skill-detail-panel">
        <div v-if="selectedSkill" class="skill-detail-content">
          <div class="detail-header">
            <div class="detail-title-row">
              <span class="detail-icon">{{ getCategoryIcon(selectedSkill.category) }}</span>
              <div>
                <h2>{{ getSkillName(selectedSkill) }}</h2>
                <p>{{ getSkillDescription(selectedSkill) || t('plugins.noDescription') }}</p>
              </div>
            </div>

            <label class="toggle-switch">
              <input
                type="checkbox"
                :checked="selectedSkill.enabled"
                :disabled="skillStore.loading"
                @change="handleToggle(selectedSkill)"
              />
              <span class="toggle-slider"></span>
            </label>
          </div>

          <div class="detail-meta-grid">
            <div class="meta-entry">
              <span>{{ t('skills.detail.labels.id') }}</span>
              <code>{{ selectedSkill.id }}</code>
            </div>
            <div class="meta-entry">
              <span>{{ t('skills.detail.labels.version') }}</span>
              <strong>{{ selectedSkill.version || '-' }}</strong>
            </div>
            <div class="meta-entry">
              <span>{{ t('skills.detail.labels.author') }}</span>
              <strong>{{ selectedSkill.author || '-' }}</strong>
            </div>
            <div class="meta-entry">
              <span>{{ t('skills.detail.labels.category') }}</span>
              <strong>{{ getCategoryLabel(selectedSkill.category) }}</strong>
            </div>
          </div>

          <section class="detail-section" v-if="selectedSkill.inputs?.length || selectedSkill.outputs?.length">
            <h4>{{ t('skills.detail.sections.parameters') }}</h4>

            <div v-if="selectedSkill.inputs?.length" class="param-group">
              <p class="param-title">{{ t('skills.detail.sections.inputs') }}</p>
              <ul class="param-list">
                <li v-for="input in selectedSkill.inputs" :key="`in-${input.name}`">
                  <div class="param-head">
                    <code>{{ input.name }}</code>
                    <span class="param-type">{{ input.type }}</span>
                    <span v-if="input.required" class="param-required">{{ t('skills.detail.required') }}</span>
                  </div>
                  <p>{{ input.description || t('common.noDescriptionAvailable') }}</p>
                </li>
              </ul>
            </div>

            <div v-if="selectedSkill.outputs?.length" class="param-group">
              <p class="param-title">{{ t('skills.detail.sections.outputs') }}</p>
              <ul class="param-list">
                <li v-for="output in selectedSkill.outputs" :key="`out-${output.name}`">
                  <div class="param-head">
                    <code>{{ output.name }}</code>
                    <span class="param-type">{{ output.type }}</span>
                  </div>
                  <p>{{ output.description || t('common.noDescriptionAvailable') }}</p>
                </li>
              </ul>
            </div>
          </section>

          <section class="detail-section detail-docs">
            <h4>{{ t('skills.detail.sections.documentation') }}</h4>
            <div v-if="selectedSkillContentLoading" class="loading-content">
              <div class="spinner"></div>
              <span>{{ t('common.loading') }}</span>
            </div>
            <div v-else-if="selectedSkillContent">
              <div v-if="selectedSkillFrontmatter.length" class="frontmatter-panel">
                <p class="frontmatter-title">{{ frontmatterLabel }}</p>
                <div class="frontmatter-grid">
                  <div v-for="entry in selectedSkillFrontmatter" :key="entry.key" class="frontmatter-item">
                    <span class="frontmatter-key">{{ entry.key }}</span>
                    <code class="frontmatter-value">{{ entry.value }}</code>
                  </div>
                </div>
              </div>
              <div v-if="selectedSkillDocContent" class="skill-content markdown-body" v-html="renderMarkdown(selectedSkillDocContent)"></div>
              <div v-else class="no-content">
                <p>{{ t('skills.noContent') }}</p>
              </div>
            </div>
            <div v-else class="no-content">
              <p>{{ t('skills.noContent') }}</p>
            </div>
          </section>
        </div>

        <div v-else class="detail-empty">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
            <path d="M8 6h13M8 12h13M8 18h13M3 6h.01M3 12h.01M3 18h.01" />
          </svg>
          <h3>{{ t('skills.detail.emptyTitle') }}</h3>
          <p>{{ t('skills.detail.emptyDescription') }}</p>
        </div>
      </aside>
    </div>
  </div>
</template>

<style scoped>
@import './extension-tab.css';

.skill-tab-redesign {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.overview-grid {
  display: grid;
  gap: 10px;
  grid-template-columns: repeat(auto-fit, minmax(130px, 1fr));
}

.overview-item {
  padding: 12px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--glass-bg, rgba(255, 255, 255, 0.05));
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.overview-value {
  font-size: 22px;
  font-weight: 600;
  color: var(--text-primary);
  line-height: 1.1;
}

.overview-value.good {
  color: #22c55e;
}

.overview-value.muted {
  color: var(--text-secondary);
}

.overview-label {
  font-size: 12px;
  color: var(--text-secondary);
}

.skill-layout {
  display: grid;
  grid-template-columns: minmax(0, 1.35fr) minmax(340px, 1fr);
  gap: 16px;
  align-items: start;
}

.skill-list {
  min-width: 0;
}

.skill-grid {
  grid-template-columns: repeat(auto-fill, minmax(250px, 1fr));
}

.skill-card {
  cursor: pointer;
  outline: none;
}

.skill-card.active {
  border-color: rgba(59, 130, 246, 0.45);
  box-shadow: 0 0 0 1px rgba(59, 130, 246, 0.35), 0 10px 22px rgba(59, 130, 246, 0.18);
}

.status-pill {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  border-radius: 999px;
  font-size: 10px;
  border: 1px solid transparent;
}

.status-enabled {
  color: #22c55e;
  background: rgba(34, 197, 94, 0.14);
  border-color: rgba(34, 197, 94, 0.28);
}

.status-disabled {
  color: var(--text-secondary);
  background: rgba(148, 163, 184, 0.14);
  border-color: rgba(148, 163, 184, 0.24);
}

.skill-detail-panel {
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--glass-bg, rgba(255, 255, 255, 0.05));
  min-height: 360px;
  max-height: calc(100vh - 260px);
  overflow: auto;
  position: sticky;
  top: 12px;
}

.skill-detail-content {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 14px;
}

.detail-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.detail-title-row {
  display: flex;
  align-items: flex-start;
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

.detail-meta-grid {
  display: grid;
  gap: 8px;
  grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
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

.meta-entry strong,
.meta-entry code {
  font-size: 12px;
  color: var(--text-primary);
  word-break: break-all;
}

.detail-section {
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 12px;
  background: var(--glass-bg, rgba(255, 255, 255, 0.03));
}

.detail-section h4 {
  margin: 0 0 10px;
  font-size: 14px;
  color: var(--text-primary);
}

.param-group + .param-group {
  margin-top: 14px;
  padding-top: 12px;
  border-top: 1px dashed var(--border);
}

.param-title {
  margin: 0 0 8px;
  font-size: 12px;
  color: var(--text-secondary);
  font-weight: 600;
}

.param-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.param-list li {
  padding: 8px 9px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: var(--glass-bg, rgba(255, 255, 255, 0.02));
}

.param-head {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
}

.param-type {
  font-size: 11px;
  color: var(--text-secondary);
  border: 1px solid var(--border);
  border-radius: 999px;
  padding: 1px 6px;
}

.param-required {
  font-size: 11px;
  color: #f59e0b;
}

.param-list p {
  margin: 0;
  color: var(--text-secondary);
  font-size: 12px;
  line-height: 1.45;
}

.detail-docs .loading-content {
  min-height: 120px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  color: var(--text-secondary);
}

.detail-docs .no-content {
  min-height: 100px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-secondary);
}

.frontmatter-panel {
  margin-bottom: 12px;
  border: 1px dashed var(--border);
  border-radius: 8px;
  padding: 10px;
  background: var(--glass-bg, rgba(255, 255, 255, 0.02));
}

.frontmatter-title {
  margin: 0 0 8px;
  font-size: 12px;
  font-weight: 600;
  color: var(--text-secondary);
}

.frontmatter-grid {
  display: grid;
  gap: 8px;
  grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
}

.frontmatter-item {
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 7px 8px;
  background: var(--glass-bg, rgba(255, 255, 255, 0.02));
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.frontmatter-key {
  font-size: 11px;
  color: var(--text-secondary);
  text-transform: uppercase;
}

.frontmatter-value {
  font-size: 12px;
  color: var(--text-primary);
  word-break: break-word;
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

.list-empty {
  padding: 48px 18px;
}

.list-empty h3 {
  margin: 2px 0 0;
  color: var(--text-primary);
  font-size: 16px;
}

.list-empty p {
  margin: 2px 0 0;
  font-size: 13px;
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

@media (max-width: 1180px) {
  .skill-layout {
    grid-template-columns: 1fr;
  }

  .skill-detail-panel {
    max-height: none;
    position: static;
  }
}
</style>
