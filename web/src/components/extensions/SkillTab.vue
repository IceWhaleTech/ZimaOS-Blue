<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useSkillStore } from '@/stores/skill'
import { skillApi, type Skill } from '@/api/skill'

const { t, te } = useI18n()
const skillStore = useSkillStore()

// Built-in skill i18n
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

const searchQuery = ref('')
const filterCategory = ref<string>('all')

// Skill detail modal state
const showDetailModal = ref(false)
const selectedSkill = ref<Skill | null>(null)
const skillContent = ref('')
const loadingContent = ref(false)

const filteredSkills = computed(() => {
  let result = skillStore.skills

  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase()
    result = result.filter(
      (s) =>
        s.name.toLowerCase().includes(query) ||
        s.description?.toLowerCase().includes(query)
    )
  }

  if (filterCategory.value !== 'all') {
    result = result.filter((s) => s.category === filterCategory.value)
  }

  return [...result].sort((a, b) => a.name.toLowerCase().localeCompare(b.name.toLowerCase()))
})

const skillStats = computed(() => ({
  total: skillStore.skills.length,
  enabled: skillStore.enabledSkills.length,
  disabled: skillStore.skills.length - skillStore.enabledSkills.length,
}))

onMounted(async () => {
  await skillStore.fetchSkills()
})

async function handleToggle(skill: Skill) {
  if (skill.enabled) {
    await skillStore.disableSkill(skill.id)
  } else {
    await skillStore.enableSkill(skill.id)
  }
}

async function openSkillDetail(skill: Skill) {
  selectedSkill.value = skill
  skillContent.value = ''
  showDetailModal.value = true
  loadingContent.value = true

  try {
    const response = await skillApi.getContent(skill.id)
    skillContent.value = response.data.content || ''
  } catch {
    skillContent.value = ''
  } finally {
    loadingContent.value = false
  }
}

function closeDetailModal() {
  showDetailModal.value = false
  selectedSkill.value = null
  skillContent.value = ''
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
</script>

<template>
  <div class="skill-tab">
    <!-- Stats -->
    <div class="stats">
      <div class="stat">
        <span class="stat-value">{{ skillStats.total }}</span>
        <span class="stat-label">{{ t('plugins.stats.total') }}</span>
      </div>
      <div class="stat enabled">
        <span class="stat-value">{{ skillStats.enabled }}</span>
        <span class="stat-label">{{ t('plugins.stats.enabled') }}</span>
      </div>
      <div class="stat disabled">
        <span class="stat-value">{{ skillStats.disabled }}</span>
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

    <!-- Error message -->
    <div v-if="skillStore.error" class="error-banner">
      {{ skillStore.error }}
      <button @click="skillStore.clearError">×</button>
    </div>

    <!-- Loading -->
    <div v-if="skillStore.loading && !filteredSkills.length" class="loading">
      <div class="spinner"></div>
      <span>{{ t('common.loading') }}</span>
    </div>

    <!-- Skills Grid -->
    <div v-else class="items-grid">
      <div
        v-for="skill in filteredSkills"
        :key="skill.id"
        :class="['item-card', 'clickable', { disabled: !skill.enabled }]"
        @click="openSkillDetail(skill)"
      >
        <div class="item-header">
          <img v-if="getSkillIconUrl(skill.icon)" :src="getSkillIconUrl(skill.icon)!" class="item-icon-svg" :alt="getSkillName(skill)" />
          <span v-else class="item-icon">{{ getCategoryIcon(skill.category) }}</span>
          <div class="item-title">
            <h3 :title="getSkillName(skill)">{{ getSkillName(skill) }}</h3>
            <div class="item-title-meta">
              <span v-if="skill.category" class="category-badge">{{ getCategoryLabel(skill.category) }}</span>
              <span class="builtin-badge">{{ t('plugins.builtin') }}</span>
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
      </div>

      <div v-if="filteredSkills.length === 0" class="empty-state">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
          <path d="M9.75 9.75l4.5 4.5m0-4.5l-4.5 4.5M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
        <p>{{ searchQuery || filterCategory !== 'all' ? t('plugins.noMatchingTools') : t('plugins.noTools') }}</p>
      </div>
    </div>

    <!-- Skill Detail Modal -->
    <Teleport to="body">
      <div v-if="showDetailModal" class="modal-overlay" @click.self="closeDetailModal">
        <div class="modal-content skill-detail-modal">
          <div class="modal-header">
            <div class="modal-title-row">
              <span v-if="selectedSkill" class="modal-icon">{{ getCategoryIcon(selectedSkill.category) }}</span>
              <h2>{{ selectedSkill ? getSkillName(selectedSkill) : '' }}</h2>
            </div>
            <button class="modal-close" @click="closeDetailModal">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M18 6L6 18M6 6l12 12" />
              </svg>
            </button>
          </div>
          <div class="modal-body">
            <div v-if="loadingContent" class="loading-content">
              <div class="spinner"></div>
              <span>{{ t('common.loading') }}</span>
            </div>
            <div v-else-if="skillContent" class="skill-content markdown-body" v-html="renderMarkdown(skillContent)"></div>
            <div v-else class="no-content">
              <p>{{ t('skills.noContent') }}</p>
            </div>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script lang="ts">
// Simple markdown renderer
function renderMarkdown(content: string): string {
  if (!content) return ''

  let html = content
    // Escape HTML
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    // Headers
    .replace(/^### (.+)$/gm, '<h3>$1</h3>')
    .replace(/^## (.+)$/gm, '<h2>$1</h2>')
    .replace(/^# (.+)$/gm, '<h1>$1</h1>')
    // Bold
    .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
    // Italic
    .replace(/\*(.+?)\*/g, '<em>$1</em>')
    // Code blocks
    .replace(/```(\w*)\n([\s\S]*?)```/g, '<pre><code class="language-$1">$2</code></pre>')
    // Inline code
    .replace(/`([^`]+)`/g, '<code>$1</code>')
    // Links
    .replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" target="_blank" rel="noopener">$1</a>')
    // Lists
    .replace(/^- (.+)$/gm, '<li>$1</li>')
    // Paragraphs
    .replace(/\n\n/g, '</p><p>')

  // Wrap in paragraph
  html = '<p>' + html + '</p>'

  // Wrap consecutive li elements in ul
  html = html.replace(/(<li>.*?<\/li>)+/gs, '<ul>$&</ul>')

  return html
}
</script>

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

.item-card.clickable {
  cursor: pointer;
  transition: transform 0.2s, box-shadow 0.2s;
}

.item-card.clickable:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}

/* Modal styles */
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.6);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  padding: 20px;
}

.skill-detail-modal {
  background: var(--color-bg-elevated, #ffffff);
  border-radius: 16px;
  width: 100%;
  max-width: 700px;
  max-height: 80vh;
  display: flex;
  flex-direction: column;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.2);
  border: 1px solid var(--color-border, rgba(0, 0, 0, 0.1));
}

:root.dark .skill-detail-modal,
[data-theme="dark"] .skill-detail-modal {
  background: var(--color-bg-elevated, #1a1a2e);
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.4);
  border-color: var(--color-border, rgba(255, 255, 255, 0.1));
}

.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 20px 24px;
  border-bottom: 1px solid var(--color-border, rgba(0, 0, 0, 0.1));
}

:root.dark .modal-header,
[data-theme="dark"] .modal-header {
  border-bottom-color: var(--color-border, rgba(255, 255, 255, 0.1));
}

.modal-title-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.modal-icon {
  font-size: 24px;
}

.modal-header h2 {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
  color: var(--color-text-primary);
}

.modal-close {
  background: none;
  border: none;
  padding: 8px;
  cursor: pointer;
  color: var(--color-text-secondary);
  border-radius: 8px;
  transition: background 0.2s;
}

.modal-close:hover {
  background: var(--color-bg-secondary, rgba(255, 255, 255, 0.1));
}

.modal-close svg {
  width: 20px;
  height: 20px;
}

.modal-body {
  flex: 1;
  overflow-y: auto;
  padding: 24px;
}

.loading-content {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 40px;
  gap: 12px;
  color: var(--color-text-secondary);
}

.no-content {
  text-align: center;
  padding: 40px;
  color: var(--color-text-secondary);
}

.skill-content {
  line-height: 1.6;
  color: var(--color-text-primary);
}

.skill-content :deep(h1) {
  font-size: 24px;
  font-weight: 600;
  margin: 0 0 16px 0;
  color: var(--color-text-primary);
}

.skill-content :deep(h2) {
  font-size: 20px;
  font-weight: 600;
  margin: 24px 0 12px 0;
  color: var(--color-text-primary);
}

.skill-content :deep(h3) {
  font-size: 16px;
  font-weight: 600;
  margin: 20px 0 8px 0;
  color: var(--color-text-primary);
}

.skill-content :deep(p) {
  margin: 0 0 12px 0;
}

.skill-content :deep(ul) {
  margin: 0 0 12px 0;
  padding-left: 20px;
}

.skill-content :deep(li) {
  margin: 4px 0;
}

.skill-content :deep(code) {
  background: var(--color-bg-secondary);
  color: var(--color-text-primary);
  padding: 2px 6px;
  border-radius: 4px;
  font-family: monospace;
  font-size: 13px;
}

.skill-content :deep(pre) {
  background: var(--color-bg-secondary);
  color: var(--color-text-primary);
  padding: 16px;
  border-radius: 8px;
  overflow-x: auto;
  margin: 12px 0;
}

.skill-content :deep(pre code) {
  background: none;
  padding: 0;
  color: inherit;
}

/* Light theme support - use CSS variables that adapt to theme */
.skill-content :deep(code) {
  background: var(--color-bg-tertiary, var(--color-bg-secondary));
  color: var(--color-code-text, var(--color-text-primary));
}

.skill-content :deep(pre) {
  background: var(--color-bg-tertiary, var(--color-bg-secondary));
  color: var(--color-text-primary);
}

/* Ensure markdown-body adapts to dark mode */
.markdown-body {
  background: transparent !important;
  color: var(--color-text-primary) !important;
}

.skill-content :deep(strong) {
  font-weight: 600;
  color: var(--color-text-primary);
}

.skill-content :deep(a) {
  color: var(--color-accent, #3b82f6);
  text-decoration: none;
}

.skill-content :deep(a:hover) {
  text-decoration: underline;
}
</style>
