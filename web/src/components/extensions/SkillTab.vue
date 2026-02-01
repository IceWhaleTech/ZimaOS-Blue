<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useSkillStore } from '@/stores/skill'
import type { Skill } from '@/api/skill'

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
        :class="['item-card', { disabled: !skill.enabled }]"
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
          <label class="toggle-switch">
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
