<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useToolStore } from '@/stores/tool'
import type { Tool } from '@/api/tool'
import { getLocalizedToolDescription, getLocalizedToolName } from '@/utils/toolLocalization'

const { t, te, locale } = useI18n()
const toolStore = useToolStore()

const searchQuery = ref('')
const filterCategory = ref<string>('all')

const isChineseLocale = computed(() => locale.value.toLowerCase().startsWith('zh'))
const galleryHint = computed(() =>
  isChineseLocale.value
    ? '以卡片方式浏览工具，快速查看用途、参数规模，并直接启用或停用。'
    : 'Browse tools as cards, review what they do, and toggle them on or off quickly.'
)

type ToolCardPalette = {
  tint: string
  tintSoft: string
  ring: string
  glow: string
  iconBg: string
  iconBorder: string
  iconText: string
}

const toolCardPalettes: ToolCardPalette[] = [
  {
    tint: '#3b82f6',
    tintSoft: 'rgba(59, 130, 246, 0.12)',
    ring: 'rgba(96, 165, 250, 0.34)',
    glow: 'rgba(59, 130, 246, 0.18)',
    iconBg: 'linear-gradient(180deg, rgba(191, 219, 254, 0.28), rgba(147, 197, 253, 0.16))',
    iconBorder: 'rgba(147, 197, 253, 0.54)',
    iconText: '#2563eb',
  },
  {
    tint: '#a855f7',
    tintSoft: 'rgba(168, 85, 247, 0.12)',
    ring: 'rgba(192, 132, 252, 0.34)',
    glow: 'rgba(168, 85, 247, 0.16)',
    iconBg: 'linear-gradient(180deg, rgba(233, 213, 255, 0.3), rgba(216, 180, 254, 0.16))',
    iconBorder: 'rgba(216, 180, 254, 0.54)',
    iconText: '#9333ea',
  },
  {
    tint: '#22c55e',
    tintSoft: 'rgba(34, 197, 94, 0.12)',
    ring: 'rgba(134, 239, 172, 0.34)',
    glow: 'rgba(34, 197, 94, 0.16)',
    iconBg: 'linear-gradient(180deg, rgba(187, 247, 208, 0.28), rgba(134, 239, 172, 0.14))',
    iconBorder: 'rgba(134, 239, 172, 0.54)',
    iconText: '#16a34a',
  },
  {
    tint: '#f97316',
    tintSoft: 'rgba(249, 115, 22, 0.12)',
    ring: 'rgba(253, 186, 116, 0.34)',
    glow: 'rgba(249, 115, 22, 0.18)',
    iconBg: 'linear-gradient(180deg, rgba(254, 215, 170, 0.3), rgba(253, 186, 116, 0.16))',
    iconBorder: 'rgba(253, 186, 116, 0.54)',
    iconText: '#ea580c',
  },
  {
    tint: '#ec4899',
    tintSoft: 'rgba(236, 72, 153, 0.12)',
    ring: 'rgba(249, 168, 212, 0.34)',
    glow: 'rgba(236, 72, 153, 0.16)',
    iconBg: 'linear-gradient(180deg, rgba(251, 207, 232, 0.28), rgba(249, 168, 212, 0.16))',
    iconBorder: 'rgba(249, 168, 212, 0.54)',
    iconText: '#db2777',
  },
  {
    tint: '#14b8a6',
    tintSoft: 'rgba(20, 184, 166, 0.12)',
    ring: 'rgba(94, 234, 212, 0.34)',
    glow: 'rgba(20, 184, 166, 0.16)',
    iconBg: 'linear-gradient(180deg, rgba(153, 246, 228, 0.28), rgba(94, 234, 212, 0.14))',
    iconBorder: 'rgba(94, 234, 212, 0.48)',
    iconText: '#0f766e',
  },
]

function hashSeed(value: string): number {
  return Array.from(value).reduce((total, char, index) => total + char.charCodeAt(0) * (index + 1), 0)
}

function getToolPalette(tool: Tool): ToolCardPalette {
  const seed = `${tool.id}:${tool.category || ''}:${tool.name}`
  return toolCardPalettes[hashSeed(seed) % toolCardPalettes.length]!
}

function getToolAccentStyle(tool: Tool): Record<string, string> {
  const accent = getToolPalette(tool)
  return {
    '--tool-accent-a': accent.tint,
    '--tool-accent-soft': accent.tintSoft,
    '--tool-accent-ring': accent.ring,
    '--tool-accent-glow': accent.glow,
    '--tool-icon-bg': accent.iconBg,
    '--tool-icon-border': accent.iconBorder,
    '--tool-icon-fg': accent.iconText,
  }
}

function getToolName(tool: Tool): string {
  return getLocalizedToolName(tool.name, t, te)
}

function getToolDescription(tool: Tool): string {
  return getLocalizedToolDescription(tool.name, tool.description, t, te)
}

function getToolBadge(tool: Tool): string {
  return tool.builtin ? t('skillStore.status.builtin') : t('skillStore.status.local')
}

function getVisibleTags(tool: Tool): string[] {
  return (tool.tags || []).filter(Boolean).slice(0, 1)
}

function getToolMonogram(tool: Tool): string {
  const source = getToolName(tool).trim() || tool.id.trim()
  return Array.from(source)[0]?.toLocaleUpperCase(locale.value) || '?'
}

function formatToolVersion(value?: string): string {
  if (!value) return '-'
  return value.startsWith('v') ? value : `v${value}`
}

function getToolIconUrl(tool: Tool): string | null {
  const iconAliases: Record<string, string> = {
    image: 'mediagen',
    video: 'mediagen',
    image_generate: 'mediagen',
    video_generate: 'mediagen',
    ppt: 'mediagen',
  }
  const availableIcons = new Set([
    'analyze',
    'browser',
    'eye',
    'file-read',
    'file-write',
    'mediagen',
    'memory',
    'notifications',
    'process',
    'question',
    'sandbox',
    'schedule',
    'terminal',
    'web-search',
    'workflow',
  ])

  if (tool.icon) {
    const normalizedIcon = iconAliases[tool.icon] || tool.icon
    if (availableIcons.has(normalizedIcon)) {
      return `/icons/tools/${normalizedIcon}.svg`
    }
  }

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
    ppt: 'mediagen',
  }
  const iconName = iconMap[tool.name]
  if (iconName) return `/icons/tools/${iconName}.svg`
  return null
}

function getCategoryIcon(category?: string): string {
  const icons: Record<string, string> = {
    search: '🔍',
    system: '🖥',
    development: '🛠',
    media: '🖼',
    integration: '🔗',
    community: '👥',
    ai: '🤖',
    utility: '⚙',
  }
  return icons[category || ''] || '🔧'
}

function getCategoryLabel(category?: string): string {
  const value = category || 'other'
  const key = `plugins.categories.${value}`
  if (te(key)) return t(key)
  return value
    .split(/[_-]/g)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ')
}

const filteredTools = computed(() => {
  let result = toolStore.tools
  const query = searchQuery.value.trim().toLowerCase()

  if (query) {
    result = result.filter((tool) => {
      const name = getToolName(tool).toLowerCase()
      const desc = getToolDescription(tool).toLowerCase()
      const tags = (tool.tags || []).join(' ').toLowerCase()
      const id = tool.id.toLowerCase()
      return (
        name.includes(query) || desc.includes(query) || tags.includes(query) || id.includes(query)
      )
    })
  }

  if (filterCategory.value !== 'all') {
    result = result.filter((tool) => (tool.category || 'other') === filterCategory.value)
  }

  return [...result].sort((a, b) =>
    getToolName(a).toLowerCase().localeCompare(getToolName(b).toLowerCase())
  )
})

const toolStats = computed(() => ({
  total: toolStore.tools.length,
  enabled: toolStore.enabledTools.length,
  builtin: toolStore.builtinTools.length,
  local: toolStore.installedTools.length,
}))

const categories = computed(() => {
  const cats = new Set<string>()
  toolStore.tools.forEach((tool) => {
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
</script>

<template>
  <div class="tool-tab tool-gallery">
    <section class="tools-showcase dashboard-card-surface">
      <div class="tools-showcase__lead">
        <div class="tools-showcase__intro">
          <span class="dashboard-card-label">{{ t('extensions.tools') }}</span>
          <h2 class="tools-showcase__title">{{ t('plugins.subtitle') }}</h2>
          <p class="tools-showcase__hint">
            <span class="tools-showcase__hint-icon" aria-hidden="true">!</span>
            <span>{{ galleryHint }}</span>
          </p>
        </div>

        <div class="tools-showcase__stats">
          <article class="tools-showcase__stat-card dashboard-card-subsurface">
            <span>{{ t('plugins.stats.total') }}</span>
            <strong>{{ toolStats.total }}</strong>
          </article>
          <article class="tools-showcase__stat-card dashboard-card-subsurface">
            <span>{{ t('plugins.stats.enabled') }}</span>
            <strong>{{ toolStats.enabled }}</strong>
          </article>
          <article class="tools-showcase__stat-card dashboard-card-subsurface">
            <span>{{ t('skillStore.status.local') }}</span>
            <strong>{{ toolStats.local }}</strong>
          </article>
          <article class="tools-showcase__stat-card dashboard-card-subsurface">
            <span>{{ t('skillStore.status.builtin') }}</span>
            <strong>{{ toolStats.builtin }}</strong>
          </article>
        </div>
      </div>

      <div class="filters tools-showcase__filters dashboard-card-subsurface">
        <div class="search-box tools-showcase__search">
          <svg
            class="search-icon"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          >
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
            {{ getCategoryIcon(cat) }} {{ getCategoryLabel(cat) }}
          </option>
        </select>

        <div class="filter-actions">
          <button class="btn-refresh" :disabled="toolStore.loading" @click="toolStore.fetchTools()">
            <svg
              v-if="!toolStore.loading"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
            >
              <path d="M21 12a9 9 0 0 0-9-9 9.75 9.75 0 0 0-6.74 2.74L3 8" />
              <path d="M3 3v5h5" />
              <path d="M3 12a9 9 0 0 0 9 9 9.75 9.75 0 0 0 6.74-2.74L21 16" />
              <path d="M16 21h5v-5" />
            </svg>
            <span v-else class="spinner"></span>
          </button>
        </div>
      </div>
    </section>

    <div v-if="toolStore.error" class="error-banner">
      {{ toolStore.error }}
      <button @click="toolStore.clearError">×</button>
    </div>

    <div v-if="toolStore.loading && !filteredTools.length" class="loading">
      <div class="spinner"></div>
      <span>{{ t('common.loading') }}</span>
    </div>

    <section v-else class="tool-gallery__grid">
      <div v-if="filteredTools.length === 0" class="empty-state list-empty">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
          <path d="M9.75 9.75l4.5 4.5m0-4.5l-4.5 4.5M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
        <h3>{{ te('plugins.noToolsTitle') ? t('plugins.noToolsTitle') : 'No tools found' }}</h3>
        <p>
          {{
            searchQuery || filterCategory !== 'all'
              ? t('plugins.noMatchingTools')
              : t('plugins.noTools')
          }}
        </p>
      </div>

      <article
        v-for="tool in filteredTools"
        v-else
        :key="tool.id"
        :class="[
          'tool-showcase-card',
          'dashboard-card-surface',
          { 'tool-showcase-card--disabled': !tool.enabled },
        ]"
        :style="getToolAccentStyle(tool)"
      >
        <div class="tool-showcase-card__topline">
          <span class="tool-showcase-card__badge">{{ getToolBadge(tool) }}</span>
          <div class="tool-showcase-card__topline-actions">
            <span
              :class="[
                'tool-showcase-card__state',
                tool.enabled
                  ? 'tool-showcase-card__state--enabled'
                  : 'tool-showcase-card__state--disabled',
              ]"
            >
              <span class="tool-showcase-card__state-dot"></span>
              {{ tool.enabled ? t('common.enabled') : t('common.disabled') }}
            </span>
            <label class="toggle-switch" @click.stop>
              <input
                type="checkbox"
                :checked="tool.enabled"
                :disabled="toolStore.loading"
                @change="handleToggle(tool)"
              />
              <span class="toggle-slider"></span>
            </label>
          </div>
        </div>

        <div class="tool-showcase-card__hero dashboard-card-subsurface">
          <div class="tool-showcase-card__orb">
            <img
              v-if="getToolIconUrl(tool)"
              :src="getToolIconUrl(tool)!"
              class="tool-showcase-card__orb-image"
              :alt="getToolName(tool)"
            />
            <span v-else class="tool-showcase-card__orb-fallback">{{ getToolMonogram(tool) }}</span>
          </div>

          <div class="tool-showcase-card__hero-copy">
            <div class="tool-showcase-card__title-row">
              <h3 :title="getToolName(tool)">{{ getToolName(tool) }}</h3>
              <span
                v-for="tag in getVisibleTags(tool)"
                :key="`${tool.id}-${tag}`"
                class="tool-showcase-card__chip tool-showcase-card__chip--soft"
              >
                {{ tag }}
              </span>
            </div>
            <code class="tool-showcase-card__id">{{ tool.id }}</code>
            <p>{{ getToolDescription(tool) || t('plugins.noDescription') }}</p>
            <div class="tool-showcase-card__chips">
              <span class="tool-showcase-card__chip tool-showcase-card__chip--primary">{{
                getCategoryLabel(tool.category)
              }}</span>
              <span
                v-if="tool.author"
                class="tool-showcase-card__chip tool-showcase-card__chip--soft"
              >
                {{ tool.author }}
              </span>
            </div>
          </div>
        </div>

        <div class="tool-showcase-card__content">
          <div class="tool-showcase-card__footer">
            <div class="tool-showcase-card__metrics tool-showcase-card__metrics--rail">
              <span class="tool-showcase-card__metric tool-showcase-card__metric--inline">
                <svg
                  class="tool-showcase-card__metric-icon"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="1.8"
                >
                  <path d="M21 8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16Z" />
                  <path d="m3.3 7 8.7 5 8.7-5" />
                  <path d="M12 22V12" />
                </svg>
                <strong>{{ formatToolVersion(tool.version) }}</strong>
              </span>
              <span class="tool-showcase-card__metric tool-showcase-card__metric--inline">
                <svg
                  class="tool-showcase-card__metric-icon"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="1.8"
                >
                  <rect x="4" y="5" width="16" height="14" rx="2" />
                  <path d="M8 9h8" />
                  <path d="M8 13h5" />
                </svg>
                <strong>{{ tool.parameters?.length || 0 }}</strong>
              </span>
              <span class="tool-showcase-card__metric tool-showcase-card__metric--inline">
                <svg
                  class="tool-showcase-card__metric-icon"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="1.8"
                >
                  <path d="M12 12a4 4 0 1 0 0-8 4 4 0 0 0 0 8Z" />
                  <path d="M6 20a6 6 0 0 1 12 0" />
                </svg>
                <strong>{{ tool.author || getToolBadge(tool) }}</strong>
              </span>
            </div>
          </div>
        </div>
      </article>
    </section>
  </div>
</template>

<style scoped>
@import './extension-tab.css';

.tool-gallery {
  --tools-shell-border: rgba(203, 213, 225, 0.82);
  --tools-shell-bg-top: rgba(255, 255, 255, 0.98);
  --tools-shell-bg-bottom: rgba(239, 244, 249, 0.96);
  --tools-shell-shadow:
    0 20px 32px -24px rgba(15, 23, 42, 0.14),
    0 12px 22px -18px rgba(59, 130, 246, 0.08);
  --tools-shell-hint-bg: rgba(22, 163, 74, 0.12);
  --tools-shell-hint-text: #15803d;
  --tools-stat-border: rgba(203, 213, 225, 0.88);
  --tools-stat-bg: rgba(255, 255, 255, 0.84);
  --tools-stat-text: #64748b;
  --tools-stat-value: #0f172a;
  --tools-card-border: rgba(203, 213, 225, 0.88);
  --tools-card-bg-top: rgba(255, 255, 255, 0.98);
  --tools-card-bg-bottom: rgba(244, 248, 251, 0.98);
  --tools-card-shadow:
    0 18px 30px -22px rgba(15, 23, 42, 0.12),
    0 10px 18px -16px rgba(59, 130, 246, 0.08);
  --tools-card-shadow-active:
    0 22px 34px -22px rgba(15, 23, 42, 0.14),
    0 14px 22px -16px rgba(59, 130, 246, 0.1);
  --tools-card-title: #0f172a;
  --tools-card-text: #475569;
  --tools-card-outline: rgba(255, 255, 255, 0.8);
  --tools-badge-border: rgba(203, 213, 225, 0.92);
  --tools-badge-bg: rgba(248, 250, 252, 0.92);
  --tools-badge-text: #475569;
  --tools-state-border: rgba(203, 213, 225, 0.88);
  --tools-state-bg: rgba(255, 255, 255, 0.82);
  --tools-state-text: #334155;
  --tools-state-enabled-text: #166534;
  --tools-state-enabled-bg: rgba(34, 197, 94, 0.12);
  --tools-state-enabled-border: rgba(134, 239, 172, 0.46);
  --tools-state-disabled-text: #475569;
  --tools-state-disabled-bg: rgba(248, 250, 252, 0.94);
  --tools-state-disabled-border: rgba(203, 213, 225, 0.92);
  --tools-chip-border: rgba(203, 213, 225, 0.88);
  --tools-chip-bg: rgba(248, 250, 252, 0.94);
  --tools-chip-primary-bg: rgba(239, 246, 255, 0.96);
  --tools-chip-text: #334155;
  --tools-chip-soft-text: #64748b;
  --tools-meta-border: rgba(203, 213, 225, 0.88);
  --tools-detail-label: #64748b;
  --tools-detail-code-bg: rgba(241, 245, 249, 0.98);
  --tools-detail-code-border: rgba(203, 213, 225, 0.86);
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.tools-showcase {
  padding: 18px;
  border: 1px solid var(--tools-shell-border);
  border-radius: 28px;
  background:
    radial-gradient(circle at top right, color-mix(in srgb, var(--tool-accent-a, var(--primary)) 14%, transparent), transparent 34%),
    linear-gradient(180deg, var(--tools-shell-bg-top) 0%, var(--tools-shell-bg-bottom) 100%);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.08),
    var(--tools-shell-shadow);
}

.tools-showcase__lead {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(420px, 0.92fr);
  gap: 14px;
  margin-bottom: 14px;
}

.tools-showcase__intro {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.tools-showcase__title {
  margin: 0;
  color: var(--text-primary);
  font-size: clamp(1.1rem, 1vw + 0.95rem, 1.5rem);
  line-height: 1.1;
  letter-spacing: -0.03em;
}

.tools-showcase__hint {
  display: inline-flex;
  align-items: flex-start;
  gap: 9px;
  margin: 0;
  color: var(--text-secondary);
  font-size: 12px;
  line-height: 1.5;
  max-width: 42rem;
}

.tools-showcase__hint-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  border-radius: 999px;
  background: var(--tools-shell-hint-bg);
  color: var(--tools-shell-hint-text);
  font-size: 10px;
  font-weight: 700;
  flex-shrink: 0;
}

.tools-showcase__stats {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
}

.tools-showcase__stat-card {
  min-height: 68px;
  padding: 10px;
  border: 1px solid var(--tools-stat-border);
  border-radius: 14px;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  gap: 6px;
  background:
    linear-gradient(180deg, color-mix(in srgb, var(--tools-stat-bg) 88%, white 4%) 0%, var(--tools-stat-bg) 100%);
  color: var(--tools-stat-text);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.06);
}

.tools-showcase__stat-card span {
  font-size: 8px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.tools-showcase__stat-card strong {
  color: var(--tools-stat-value);
  font-size: clamp(1rem, 0.55vw + 0.85rem, 1.2rem);
  line-height: 1;
  letter-spacing: -0.03em;
}

.tools-showcase__filters {
  margin-bottom: 0;
  padding: 12px;
  border-radius: 18px;
  border: 1px solid var(--tools-stat-border);
  background: color-mix(in srgb, var(--tools-stat-bg) 84%, transparent);
}

.tools-showcase__search {
  min-width: min(240px, 100%);
  max-width: 320px;
}

.tool-gallery__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(420px, 1fr));
  gap: 18px;
}

.tool-showcase-card {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 18px 20px;
  border: 1px solid var(--tools-card-border);
  border-radius: 28px;
  background:
    radial-gradient(circle at top right, color-mix(in srgb, var(--tool-accent-a) 10%, transparent), transparent 32%),
    linear-gradient(180deg, var(--tools-card-bg-top) 0%, var(--tools-card-bg-bottom) 100%);
  box-shadow:
    inset 0 1px 0 var(--tools-card-outline),
    var(--tools-card-shadow);
  overflow: hidden;
  transition:
    transform 0.22s ease,
    box-shadow 0.22s ease,
    border-color 0.22s ease,
    background 0.22s ease;
}

.tool-showcase-card:hover {
  transform: translateY(-2px);
  border-color: var(--tool-accent-ring);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.08),
    var(--tools-card-shadow-active),
    0 0 0 1px color-mix(in srgb, var(--tool-accent-ring) 42%, transparent);
}

.tool-showcase-card--disabled {
  opacity: 0.92;
  filter: saturate(0.88);
}

.tool-showcase-card__topline {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.tool-showcase-card__topline-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.tool-showcase-card__hero {
  display: flex;
  align-items: flex-start;
  gap: 16px;
  padding: 0;
  border: 0;
  background: transparent;
}

.tool-showcase-card__orb {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 64px;
  height: 64px;
  border-radius: 22px;
  background: var(--tool-icon-bg);
  border: 1px solid var(--tool-icon-border);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.18),
    0 18px 28px -24px var(--tool-accent-glow);
  color: var(--tool-icon-fg);
  flex-shrink: 0;
}

.tool-showcase-card__orb-image {
  width: 28px;
  height: 28px;
  object-fit: contain;
  filter: drop-shadow(0 8px 14px rgba(15, 23, 42, 0.28));
}

.tool-showcase-card__orb-fallback {
  font-size: 28px;
  font-weight: 700;
  line-height: 1;
  filter: drop-shadow(0 8px 14px rgba(15, 23, 42, 0.28));
}

.tool-showcase-card__hero-copy {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
  flex: 1;
}

.tool-showcase-card__title-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.tool-showcase-card__title-row h3 {
  margin: 0;
  color: var(--tools-card-title);
  font-size: 1.3rem;
  line-height: 1.15;
  letter-spacing: -0.025em;
}

.tool-showcase-card__id {
  display: inline-flex;
  width: fit-content;
  padding: 3px 7px;
  border-radius: 999px;
  border: 1px solid var(--tools-detail-code-border);
  background: var(--tools-detail-code-bg);
  color: var(--tools-detail-label);
  font-size: 10px;
  line-height: 1;
}

.tool-showcase-card__hero-copy p {
  margin: 0;
  color: var(--tools-card-text);
  font-size: 1rem;
  line-height: 1.62;
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.tool-showcase-card__badge {
  display: inline-flex;
  align-items: center;
  padding: 4px 8px;
  border-radius: 999px;
  border: 1px solid var(--tools-badge-border);
  background: var(--tools-badge-bg);
  color: var(--tools-badge-text);
  font-size: 9px;
  font-weight: 600;
  white-space: nowrap;
}

.tool-showcase-card__state {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 5px 9px;
  border-radius: 999px;
  border: 1px solid var(--tools-state-border);
  background: var(--tools-state-bg);
  color: var(--tools-state-text);
  font-size: 9px;
  font-weight: 600;
  white-space: nowrap;
}

.tool-showcase-card__state-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}

.tool-showcase-card__state--enabled {
  color: var(--tools-state-enabled-text);
  background: var(--tools-state-enabled-bg);
  border-color: var(--tools-state-enabled-border);
}

.tool-showcase-card__state--disabled {
  color: var(--tools-state-disabled-text);
  background: var(--tools-state-disabled-bg);
  border-color: var(--tools-state-disabled-border);
}

.tool-showcase-card__content {
  min-width: 0;
  margin-top: auto;
  padding-top: 2px;
}

.tool-showcase-card__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-top: auto;
  padding-top: 14px;
  border-top: 1px solid var(--tools-meta-border);
}

.tool-showcase-card__metrics {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.tool-showcase-card__metrics--rail {
  row-gap: 6px;
}

.tool-showcase-card__metric {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 0;
  padding: 0;
  border: 0;
  background: transparent;
}

.tool-showcase-card__metric--inline {
  color: var(--tools-detail-label);
}

.tool-showcase-card__metric-icon {
  width: 14px;
  height: 14px;
  color: var(--tool-accent-a);
  flex-shrink: 0;
}

.tool-showcase-card__metric strong {
  color: var(--text-primary);
  font-size: 12px;
  line-height: 1.2;
}

.tool-showcase-card__chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-items: center;
}

.tool-showcase-card__chip {
  display: inline-flex;
  align-items: center;
  padding: 4px 9px;
  border-radius: 999px;
  border: 1px solid var(--tools-chip-border);
  background: var(--tools-chip-bg);
  color: var(--tools-chip-text);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04);
  font-size: 9px;
  font-weight: 600;
}

.tool-showcase-card__chip--primary {
  background: var(--tools-chip-primary-bg);
}

.tool-showcase-card__chip--soft {
  color: var(--tools-chip-soft-text);
  background: rgba(255, 255, 255, 0.56);
}

.list-empty {
  grid-column: 1 / -1;
  padding: 38px 16px;
  text-align: center;
  border: 1px dashed var(--tools-stat-border);
  border-radius: 24px;
  color: var(--text-secondary);
  background: rgba(255, 255, 255, 0.72);
}

.list-empty svg {
  width: 32px;
  height: 32px;
  opacity: 0.34;
}

.list-empty h3 {
  margin: 8px 0 0;
  color: var(--text-primary);
  font-size: 14px;
}

.list-empty p {
  margin: 6px 0 0;
  font-size: 12px;
  line-height: 1.5;
}

@media (max-width: 980px) {
  .tools-showcase__lead {
    grid-template-columns: 1fr;
  }

  .tools-showcase__stats {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }

  .tool-gallery__grid {
    grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  }

  .tool-showcase-card__footer {
    flex-direction: column;
    align-items: stretch;
  }
}

@media (max-width: 720px) {
  .tools-showcase {
    padding: 12px;
    border-radius: 18px;
  }

  .tools-showcase__title {
    font-size: 1.2rem;
  }

  .tools-showcase__stats {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .tool-gallery__grid {
    grid-template-columns: 1fr;
    gap: 12px;
  }

  .tool-showcase-card {
    padding: 15px;
    border-radius: 22px;
  }

  .tool-showcase-card__hero {
    gap: 14px;
  }

  .tool-showcase-card__orb {
    width: 56px;
    height: 56px;
    border-radius: 18px;
  }

  .tool-showcase-card__topline,
  .tool-showcase-card__topline-actions {
    flex-wrap: wrap;
  }
}
</style>
