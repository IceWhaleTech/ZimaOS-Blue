<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { skillApi, type Skill, type SkillContractMetadata } from '@/api/skill'
import SkillContractNotice from './SkillContractNotice.vue'
import { useSkillStore } from '@/stores/skill'
import { parseFrontmatter } from '@/utils/frontmatter'
import { renderMarkdown as renderMarkdownHtml } from '@/utils/markdown'
import { formatVersionLabel } from '@/utils/version-label'

const { t, te, locale } = useI18n()
const skillStore = useSkillStore()
const emit = defineEmits<{
  (e: 'install-skill'): void
}>()

const searchQuery = ref('')
const filterCategory = ref<string>('all')
const filterStatus = ref<'all' | 'enabled' | 'disabled'>('all')

const selectedSkillId = ref<string | null>(null)
const showDetailModal = ref(false)
const contentBySkillId = ref<Record<string, string>>({})
const contentContractBySkillId = ref<Record<string, SkillContractMetadata>>({})
const contentLoadingIds = ref<Set<string>>(new Set())
const uninstallingSkillId = ref<string | null>(null)

function browseText(key: string, fallback: string): string {
  return te(key) ? t(key) : fallback
}

const galleryHint = computed(() =>
  browseText(
    'extensions.browse.skillGalleryHint',
    'Browse skills as cards. Open any skill to review docs.'
  )
)
const skillManagementHint = computed(() =>
  browseText(
    'extensions.browse.skillManagementHint',
    'Uninstall is available only for local skills.'
  )
)
const closeDetailLabel = computed(() =>
  browseText('extensions.browse.closeSkillDetails', 'Close skill details')
)
const sourceMetaLabel = computed(() => browseText('extensions.browse.sourceLabel', 'Source'))
const builtinSkillDetailHint = computed(() =>
  browseText(
    'extensions.browse.builtinSkillDetailHint',
    'This is a built-in skill and cannot be uninstalled.'
  )
)

type SkillCardPalette = {
  tint: string
  tintSoft: string
  ring: string
  glow: string
  iconBg: string
  iconBorder: string
  iconText: string
}

const skillCardPalettes: SkillCardPalette[] = [
  {
    tint: '#3b82f6',
    tintSoft: 'rgba(59, 130, 246, 0.12)',
    ring: 'rgba(96, 165, 250, 0.34)',
    glow: 'rgba(59, 130, 246, 0.2)',
    iconBg: 'linear-gradient(180deg, rgba(59, 130, 246, 0.2), rgba(147, 197, 253, 0.12))',
    iconBorder: 'rgba(147, 197, 253, 0.52)',
    iconText: '#2563eb',
  },
  {
    tint: '#a855f7',
    tintSoft: 'rgba(168, 85, 247, 0.12)',
    ring: 'rgba(192, 132, 252, 0.34)',
    glow: 'rgba(168, 85, 247, 0.18)',
    iconBg: 'linear-gradient(180deg, rgba(233, 213, 255, 0.32), rgba(216, 180, 254, 0.16))',
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
    iconBg: 'linear-gradient(180deg, rgba(254, 215, 170, 0.28), rgba(253, 186, 116, 0.16))',
    iconBorder: 'rgba(253, 186, 116, 0.5)',
    iconText: '#ea580c',
  },
  {
    tint: '#ec4899',
    tintSoft: 'rgba(236, 72, 153, 0.12)',
    ring: 'rgba(249, 168, 212, 0.34)',
    glow: 'rgba(236, 72, 153, 0.18)',
    iconBg: 'linear-gradient(180deg, rgba(251, 207, 232, 0.3), rgba(249, 168, 212, 0.14))',
    iconBorder: 'rgba(249, 168, 212, 0.52)',
    iconText: '#db2777',
  },
  {
    tint: '#14b8a6',
    tintSoft: 'rgba(20, 184, 166, 0.12)',
    ring: 'rgba(94, 234, 212, 0.34)',
    glow: 'rgba(20, 184, 166, 0.18)',
    iconBg: 'linear-gradient(180deg, rgba(153, 246, 228, 0.28), rgba(94, 234, 212, 0.14))',
    iconBorder: 'rgba(94, 234, 212, 0.48)',
    iconText: '#0f766e',
  },
]

function hashSeed(value: string): number {
  return Array.from(value).reduce((total, char, index) => {
    return total + char.charCodeAt(0) * (index + 1)
  }, 0)
}

function getSkillCardPalette(skill: Skill): SkillCardPalette {
  const seed = `${skill.id}:${skill.category || ''}:${skill.name}`
  return skillCardPalettes[hashSeed(seed) % skillCardPalettes.length]!
}

function getSkillName(skill: Skill): string {
  const candidates = skill.builtin
    ? [`skills.builtin.${skill.id}.name`, `skills.catalog.${skill.id}.name`]
    : [`skills.catalog.${skill.id}.name`]

  for (const key of candidates) {
    if (te(key)) return t(key)
  }

  return skill.name
}

function getSkillDescription(skill: Skill): string {
  const candidates = skill.builtin
    ? [`skills.builtin.${skill.id}.description`, `skills.catalog.${skill.id}.description`]
    : [`skills.catalog.${skill.id}.description`]

  for (const key of candidates) {
    if (te(key)) return t(key)
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
    utility: '🛠',
    system: '🖥',
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

function getSkillBadge(skill: Skill): string {
  return skill.builtin ? t('skillStore.status.builtin') : t('skillStore.status.local')
}

function getSkillContractChip(skill: Skill): string {
  switch (skill.contract_status) {
    case 'legacy_fallback':
      return locale.value.toLowerCase().startsWith('zh') ? '兼容回退' : 'Legacy'
    case 'generated_contract':
      return locale.value.toLowerCase().startsWith('zh') ? '生成契约' : 'Generated'
    default:
      return ''
  }
}

function getVisibleTags(skill: Skill): string[] {
  return (skill.tags || []).filter(Boolean).slice(0, 1)
}

function getSkillMonogram(skill: Skill): string {
  const source = getSkillName(skill).trim() || skill.id.trim()
  return Array.from(source)[0]?.toLocaleUpperCase(locale.value) || '?'
}

function formatSkillVersion(value?: string): string {
  return formatVersionLabel(value)
}

function skillMetaText(value?: string): string {
  if (value?.trim()) return value
  return t('common.unknown')
}

function getSkillAccentStyle(skill: Skill): Record<string, string> {
  const accent = getSkillCardPalette(skill)
  return {
    '--skill-accent-a': accent.tint,
    '--skill-accent-soft': accent.tintSoft,
    '--skill-accent-glow': accent.glow,
    '--skill-accent-ring': accent.ring,
    '--skill-icon-bg': accent.iconBg,
    '--skill-icon-border': accent.iconBorder,
    '--skill-icon-fg': accent.iconText,
  }
}

const filteredSkills = computed(() => {
  let result = skillStore.skills
  const query = searchQuery.value.trim().toLowerCase()

  if (query) {
    result = result.filter((skill) => {
      const name = getSkillName(skill).toLowerCase()
      const desc = getSkillDescription(skill).toLowerCase()
      const tags = (skill.tags || []).join(' ').toLowerCase()
      const id = skill.id.toLowerCase()
      return (
        name.includes(query) || desc.includes(query) || tags.includes(query) || id.includes(query)
      )
    })
  }

  if (filterCategory.value !== 'all') {
    result = result.filter((skill) => (skill.category || 'other') === filterCategory.value)
  }

  if (filterStatus.value === 'enabled') {
    result = result.filter((skill) => skill.enabled)
  } else if (filterStatus.value === 'disabled') {
    result = result.filter((skill) => !skill.enabled)
  }

  return [...result].sort((a, b) =>
    getSkillName(a).toLowerCase().localeCompare(getSkillName(b).toLowerCase())
  )
})

const selectedSkill = computed(() => {
  if (!selectedSkillId.value) return null
  return (
    filteredSkills.value.find((skill) => skill.id === selectedSkillId.value) ||
    skillStore.skills.find((skill) => skill.id === selectedSkillId.value) ||
    null
  )
})

const selectedSkillContent = computed(() => {
  if (!selectedSkill.value) return ''
  return contentBySkillId.value[selectedSkill.value.id] || ''
})

const selectedSkillContentParsed = computed(() => parseFrontmatter(selectedSkillContent.value))
const selectedSkillDocContent = computed(() => selectedSkillContentParsed.value.body)
const selectedSkillContract = computed<SkillContractMetadata | null>(() => {
  if (!selectedSkill.value) return null

  const cachedContract = contentContractBySkillId.value[selectedSkill.value.id] || {}
  const mergedNotes = Array.from(
    new Set([...(selectedSkill.value.contract_notes || []), ...(cachedContract.contract_notes || [])])
  ).filter(Boolean)

  const contract = {
    contract_status: cachedContract.contract_status || selectedSkill.value.contract_status,
    contract_source: cachedContract.contract_source || selectedSkill.value.contract_source,
    contract_notes: mergedNotes,
  }
  if (!contract.contract_status && !contract.contract_source && !contract.contract_notes?.length) {
    return null
  }
  return contract
})

const selectedSkillContentLoading = computed(() => {
  if (!selectedSkill.value) return false
  return contentLoadingIds.value.has(selectedSkill.value.id)
})
const canUninstallSelectedSkill = computed(
  () => !!selectedSkill.value && !selectedSkill.value.builtin
)

const skillStats = computed(() => ({
  total: skillStore.skills.length,
  enabled: skillStore.enabledSkills.length,
  builtin: skillStore.builtinSkills.length,
  local: skillStore.installedSkills.length,
}))

watch(
  filteredSkills,
  (list) => {
    if (!list.length) {
      selectedSkillId.value = null
      showDetailModal.value = false
      return
    }
    if (!selectedSkillId.value || !list.some((skill) => skill.id === selectedSkillId.value)) {
      void primeSkill(list[0]!)
    }
  },
  { immediate: true }
)

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
    const contentResponse = response.data
    contentBySkillId.value = {
      ...contentBySkillId.value,
      [skillId]: contentResponse.content || '',
    }
    contentContractBySkillId.value = {
      ...contentContractBySkillId.value,
      [skillId]: {
        contract_status: contentResponse.contract_status,
        contract_source: contentResponse.contract_source,
        contract_notes: contentResponse.contract_notes,
      },
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

async function primeSkill(skill: Skill) {
  selectedSkillId.value = skill.id
  await ensureSkillContent(skill.id)
}

async function openSkillDetail(skill: Skill) {
  showDetailModal.value = true
  await primeSkill(skill)
}

function closeSkillDetail() {
  showDetailModal.value = false
}

async function handleUninstall(skill: Skill) {
  if (skill.builtin || uninstallingSkillId.value === skill.id) return

  const confirmed = window.confirm(
    t('skillStore.modal.confirmUninstallMessage', { name: getSkillName(skill) })
  )
  if (!confirmed) return

  uninstallingSkillId.value = skill.id
  try {
    const result = await skillStore.uninstallSkill(skill.id)
    if (result?.success) {
      closeSkillDetail()
      await skillStore.fetchSkills()
    }
  } finally {
    uninstallingSkillId.value = null
  }
}
</script>

<template>
  <div class="skill-tab skill-gallery">
    <section class="extension-market-hero dashboard-card-surface">
      <div class="extension-market-hero__copy">
        <span class="extension-market-hero__kicker">{{ t('extensions.skills') }}</span>
        <h2 class="extension-market-hero__title">{{ t('plugins.subtitle') }}</h2>
        <p class="extension-market-hero__hint">{{ galleryHint }}</p>
        <p class="extension-market-hero__hint">{{ skillManagementHint }}</p>
      </div>

      <div class="extension-market-hero__stats">
        <span class="extension-market-hero__stat-pill">
          <span>{{ t('plugins.stats.total') }}</span>
          <strong>{{ skillStats.total }}</strong>
        </span>
        <span class="extension-market-hero__stat-pill">
          <span>{{ t('plugins.stats.enabled') }}</span>
          <strong>{{ skillStats.enabled }}</strong>
        </span>
        <span class="extension-market-hero__stat-pill">
          <span>{{ t('skillStore.status.local') }}</span>
          <strong>{{ skillStats.local }}</strong>
        </span>
        <span class="extension-market-hero__stat-pill">
          <span>{{ t('skillStore.status.builtin') }}</span>
          <strong>{{ skillStats.builtin }}</strong>
        </span>
      </div>

      <div class="filters extension-market-hero__filters">
        <div class="search-box extension-market-hero__search">
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

        <div class="filter-actions extension-market-hero__actions">
          <button class="btn-add-source" type="button" @click="emit('install-skill')">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M12 4v16m8-8H4" />
            </svg>
            <span>{{ t('plugins.uploadSkill') }}</span>
          </button>

          <button
            class="btn-refresh"
            type="button"
            :disabled="skillStore.loading"
            @click="skillStore.fetchSkills()"
          >
            <svg
              v-if="!skillStore.loading"
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

    <div v-if="skillStore.error" class="error-banner">
      {{ skillStore.error }}
      <button @click="skillStore.clearError">×</button>
    </div>

    <div v-if="skillStore.loading && !filteredSkills.length" class="loading">
      <div class="spinner"></div>
      <span>{{ t('common.loading') }}</span>
    </div>

    <section v-else class="skill-gallery__grid">
      <div v-if="filteredSkills.length === 0" class="empty-state list-empty">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
          <path d="M9.75 9.75l4.5 4.5m0-4.5l-4.5 4.5M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
        <h3>{{ t('skills.empty.title') }}</h3>
        <p>{{ t('skills.empty.description') }}</p>
      </div>

      <article
        v-for="skill in filteredSkills"
        v-else
        :key="skill.id"
        :class="[
          'skill-showcase-card',
          'dashboard-card-surface',
          {
            'skill-showcase-card--active': selectedSkillId === skill.id,
            'skill-showcase-card--disabled': !skill.enabled,
          },
        ]"
        :style="getSkillAccentStyle(skill)"
        tabindex="0"
        role="button"
        @click="openSkillDetail(skill)"
        @keydown.enter.prevent="openSkillDetail(skill)"
        @keydown.space.prevent="openSkillDetail(skill)"
      >
        <div class="skill-showcase-card__topline">
          <span class="skill-showcase-card__badge">{{ getSkillBadge(skill) }}</span>
          <span
            :class="[
              'skill-showcase-card__state',
              skill.enabled
                ? 'skill-showcase-card__state--enabled'
                : 'skill-showcase-card__state--disabled',
            ]"
          >
            <span class="skill-showcase-card__state-dot"></span>
            {{ skill.enabled ? t('common.enabled') : t('common.disabled') }}
          </span>
        </div>

        <div :class="['skill-showcase-card__hero', 'dashboard-card-subsurface']">
          <div class="skill-showcase-card__orb">
            <img
              v-if="getSkillIconUrl(skill.icon)"
              :src="getSkillIconUrl(skill.icon)!"
              class="skill-showcase-card__orb-image"
              :alt="getSkillName(skill)"
            />
            <span v-else class="skill-showcase-card__orb-fallback">{{
              getSkillMonogram(skill)
            }}</span>
          </div>

          <div class="skill-showcase-card__hero-copy">
            <div class="skill-showcase-card__title-row">
              <h3 :title="getSkillName(skill)">{{ getSkillName(skill) }}</h3>
              <span
                v-for="tag in getVisibleTags(skill)"
                :key="`${skill.id}-${tag}`"
                class="skill-showcase-card__chip skill-showcase-card__chip--soft"
              >
                {{ tag }}
              </span>
            </div>
            <code class="skill-showcase-card__id">{{ skill.id }}</code>
            <p>{{ getSkillDescription(skill) || t('plugins.noDescription') }}</p>
            <div class="skill-showcase-card__chips">
              <span class="skill-showcase-card__chip skill-showcase-card__chip--primary">{{
                getCategoryLabel(skill.category)
              }}</span>
              <span
                v-if="getSkillContractChip(skill)"
                class="skill-showcase-card__chip skill-showcase-card__chip--contract"
              >
                {{ getSkillContractChip(skill) }}
              </span>
              <span
                v-if="skill.author"
                class="skill-showcase-card__chip skill-showcase-card__chip--soft"
              >
                {{ skill.author }}
              </span>
            </div>
          </div>
        </div>

        <div class="skill-showcase-card__content">
          <div class="skill-showcase-card__footer">
            <div class="skill-showcase-card__metrics skill-showcase-card__metrics--rail">
              <span class="skill-showcase-card__metric skill-showcase-card__metric--inline">
                <svg
                  class="skill-showcase-card__metric-icon"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="1.8"
                >
                  <path
                    d="M21 8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16Z"
                  />
                  <path d="m3.3 7 8.7 5 8.7-5" />
                  <path d="M12 22V12" />
                </svg>
                <strong>{{ formatSkillVersion(skill.version) }}</strong>
              </span>
              <span class="skill-showcase-card__metric skill-showcase-card__metric--inline">
                <svg
                  class="skill-showcase-card__metric-icon"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="1.8"
                >
                  <path d="M12 19V5" />
                  <path d="m5 12 7-7 7 7" />
                </svg>
                <strong>{{ skill.inputs?.length || 0 }}</strong>
              </span>
              <span class="skill-showcase-card__metric skill-showcase-card__metric--inline">
                <svg
                  class="skill-showcase-card__metric-icon"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="1.8"
                >
                  <path d="M12 5v14" />
                  <path d="m19 12-7 7-7-7" />
                </svg>
                <strong>{{ skill.outputs?.length || 0 }}</strong>
              </span>
            </div>
            <span class="skill-showcase-card__link-hint">{{
              t('skillStore.actions.details')
            }}</span>
          </div>
        </div>
      </article>
    </section>

    <Teleport to="body">
      <div
        v-if="showDetailModal && selectedSkill"
        class="skill-detail-modal-backdrop"
        @click.self="closeSkillDetail"
      >
        <div
          class="skill-detail-modal"
          role="dialog"
          aria-modal="true"
          :aria-label="getSkillName(selectedSkill)"
          :style="getSkillAccentStyle(selectedSkill)"
        >
          <div class="skill-detail-modal__handle" aria-hidden="true"></div>
          <button
            class="skill-detail-modal__close"
            type="button"
            :aria-label="closeDetailLabel"
            @click="closeSkillDetail"
          >
            ×
          </button>

          <div class="skill-detail-content">
            <div class="detail-header">
              <div class="detail-title-row">
                <div class="detail-icon" :style="getSkillAccentStyle(selectedSkill)">
                  <img
                    v-if="getSkillIconUrl(selectedSkill.icon)"
                    :src="getSkillIconUrl(selectedSkill.icon)!"
                    class="detail-icon__image"
                    :alt="getSkillName(selectedSkill)"
                  />
                  <span v-else>{{ getSkillMonogram(selectedSkill) }}</span>
                </div>

                <div class="detail-title-copy">
                  <div class="detail-title-copy__row">
                    <h2>{{ getSkillName(selectedSkill) }}</h2>
                    <span class="skill-showcase-card__badge">{{
                      getSkillBadge(selectedSkill)
                    }}</span>
                  </div>
                  <code class="skill-showcase-card__id skill-showcase-card__id--detail">{{
                    selectedSkill.id
                  }}</code>
                  <p>{{ getSkillDescription(selectedSkill) || t('plugins.noDescription') }}</p>
                  <div class="detail-pill-row">
                    <span class="detail-version-pill">{{
                      formatSkillVersion(selectedSkill.version)
                    }}</span>
                    <span class="skill-showcase-card__chip skill-showcase-card__chip--primary">{{
                      getCategoryLabel(selectedSkill.category)
                    }}</span>
                    <span
                      v-if="selectedSkill.author"
                      class="skill-showcase-card__chip skill-showcase-card__chip--soft"
                    >
                      {{ selectedSkill.author }}
                    </span>
                  </div>
                </div>
              </div>

              <div class="detail-actions">
                <span
                  :class="[
                    'skill-showcase-card__state',
                    selectedSkill.enabled
                      ? 'skill-showcase-card__state--enabled'
                      : 'skill-showcase-card__state--disabled',
                  ]"
                >
                  <span class="skill-showcase-card__state-dot"></span>
                  {{ selectedSkill.enabled ? t('common.enabled') : t('common.disabled') }}
                </span>

                <button
                  v-if="canUninstallSelectedSkill"
                  type="button"
                  class="detail-uninstall-button"
                  :disabled="uninstallingSkillId === selectedSkill.id || skillStore.loading"
                  @click="handleUninstall(selectedSkill)"
                >
                  {{
                    uninstallingSkillId === selectedSkill.id
                      ? t('common.loading')
                      : t('skillStore.actions.uninstall')
                  }}
                </button>
              </div>
              <p v-if="selectedSkill.builtin" class="detail-management-note">
                {{ builtinSkillDetailHint }}
              </p>
            </div>

            <div class="detail-layout">
              <aside class="detail-sidebar">
                <section class="detail-section detail-overview-panel">
                  <div class="detail-hero-stats">
                    <article class="detail-hero-stat">
                      <span>{{ t('skills.detail.sections.inputs') }}</span>
                      <strong>{{ selectedSkill.inputs?.length || 0 }}</strong>
                    </article>
                    <article class="detail-hero-stat">
                      <span>{{ t('skills.detail.sections.outputs') }}</span>
                      <strong>{{ selectedSkill.outputs?.length || 0 }}</strong>
                    </article>
                  </div>

                  <div class="detail-meta-grid">
                    <div class="meta-entry">
                      <span>{{ t('skills.detail.labels.id') }}</span>
                      <code>{{ selectedSkill.id }}</code>
                    </div>
                    <div class="meta-entry">
                      <span>{{ t('skills.detail.labels.author') }}</span>
                      <strong>{{ skillMetaText(selectedSkill.author) }}</strong>
                    </div>
                    <div class="meta-entry">
                      <span>{{ t('skills.detail.labels.category') }}</span>
                      <strong>{{ getCategoryLabel(selectedSkill.category) }}</strong>
                    </div>
                    <div class="meta-entry">
                      <span>{{ sourceMetaLabel }}</span>
                      <strong>{{ getSkillBadge(selectedSkill) }}</strong>
                    </div>
                  </div>

                  <SkillContractNotice
                    v-if="selectedSkillContract"
                    class="detail-contract-panel"
                    :contract="selectedSkillContract"
                  />
                </section>
              </aside>

              <div class="detail-main">
                <section
                  v-if="selectedSkill.inputs?.length || selectedSkill.outputs?.length"
                  class="detail-section detail-parameters"
                >
                  <div class="detail-section__head">
                    <h4>{{ t('skills.detail.sections.parameters') }}</h4>
                    <p class="detail-section__caption">
                      <span
                        >{{ t('skills.detail.sections.inputs') }}
                        {{ selectedSkill.inputs?.length || 0 }}</span
                      >
                      <span aria-hidden="true">·</span>
                      <span
                        >{{ t('skills.detail.sections.outputs') }}
                        {{ selectedSkill.outputs?.length || 0 }}</span
                      >
                    </p>
                  </div>

                  <div class="param-columns">
                    <div v-if="selectedSkill.inputs?.length" class="param-group">
                      <p class="param-title">{{ t('skills.detail.sections.inputs') }}</p>
                      <ul class="param-list">
                        <li v-for="input in selectedSkill.inputs" :key="`in-${input.name}`">
                          <div class="param-head">
                            <code>{{ input.name }}</code>
                            <span class="param-type">{{ input.type }}</span>
                            <span v-if="input.required" class="param-required">{{
                              t('skills.detail.required')
                            }}</span>
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
                  </div>
                </section>

                <section class="detail-section detail-docs">
                  <div class="detail-section__head">
                    <h4>{{ t('skills.detail.sections.documentation') }}</h4>
                  </div>

                  <div class="detail-docs__surface">
                    <div v-if="selectedSkillContentLoading" class="loading-content">
                      <div class="spinner"></div>
                      <span>{{ t('common.loading') }}</span>
                    </div>
                    <template v-else-if="selectedSkillContent">
                      <div
                        v-if="selectedSkillDocContent"
                        class="skill-content markdown-body"
                        v-html="renderMarkdownHtml(selectedSkillDocContent)"
                      ></div>
                      <div v-else class="no-content">
                        <p>{{ t('skills.noContent') }}</p>
                      </div>
                    </template>
                    <div v-else class="no-content">
                      <p>{{ t('skills.noContent') }}</p>
                    </div>
                  </div>
                </section>
              </div>
            </div>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<style scoped>
@import './extension-tab.css';

.skill-gallery,
.skill-detail-modal-backdrop {
  --skills-shell-border: rgba(71, 85, 105, 0.52);
  --skills-shell-bg-top: rgba(24, 33, 53, 0.98);
  --skills-shell-bg-bottom: rgba(9, 15, 28, 0.99);
  --skills-shell-shadow:
    0 20px 36px -30px rgba(2, 6, 23, 0.56), 0 14px 28px -24px rgba(14, 165, 233, 0.12);
  --skills-shell-hint-bg: rgba(34, 197, 94, 0.14);
  --skills-shell-hint-text: #86efac;
  --skills-stat-border: rgba(71, 85, 105, 0.48);
  --skills-stat-bg: rgba(15, 23, 42, 0.74);
  --skills-stat-text: #94a3b8;
  --skills-stat-value: #f8fafc;
  --skills-card-border: rgba(71, 85, 105, 0.56);
  --skills-card-bg-top: rgba(18, 27, 45, 0.98);
  --skills-card-bg-bottom: rgba(8, 14, 27, 0.99);
  --skills-card-shadow:
    0 24px 42px -36px rgba(2, 6, 23, 0.7), 0 14px 26px -22px rgba(8, 47, 73, 0.26);
  --skills-card-shadow-active:
    0 28px 48px -34px rgba(2, 6, 23, 0.78), 0 18px 30px -24px rgba(8, 47, 73, 0.3);
  --skills-card-title: #f8fafc;
  --skills-card-text: rgba(226, 232, 240, 0.76);
  --skills-card-outline: rgba(255, 255, 255, 0.05);
  --skills-card-highlight: rgba(148, 163, 184, 0.06);
  --skills-preview-border: rgba(120, 143, 173, 0.24);
  --skills-preview-top: rgba(255, 255, 255, 0.08);
  --skills-preview-mid: rgba(18, 31, 48, 0.94);
  --skills-preview-bottom: rgba(8, 13, 26, 0.96);
  --skills-preview-enabled-border: rgba(167, 243, 208, 0.34);
  --skills-preview-enabled-tint: rgba(16, 185, 129, 0.18);
  --skills-preview-disabled-border: rgba(125, 145, 175, 0.22);
  --skills-preview-disabled-tint: rgba(59, 130, 246, 0.12);
  --skills-icon-bg-top: rgba(255, 255, 255, 0.16);
  --skills-icon-bg-bottom: rgba(46, 81, 108, 0.68);
  --skills-icon-border: rgba(255, 255, 255, 0.16);
  --skills-icon-fg: #f8fafc;
  --skills-eyebrow-text: rgba(191, 219, 254, 0.88);
  --skills-badge-border: rgba(148, 163, 184, 0.16);
  --skills-badge-bg: rgba(15, 23, 42, 0.42);
  --skills-badge-text: #cbd5e1;
  --skills-state-border: rgba(148, 163, 184, 0.16);
  --skills-state-bg: rgba(15, 23, 42, 0.42);
  --skills-state-text: #e2e8f0;
  --skills-state-enabled-text: #dcfce7;
  --skills-state-enabled-bg: rgba(34, 197, 94, 0.14);
  --skills-state-enabled-border: rgba(110, 231, 183, 0.24);
  --skills-state-disabled-text: #e2e8f0;
  --skills-state-disabled-bg: rgba(148, 163, 184, 0.12);
  --skills-state-disabled-border: rgba(148, 163, 184, 0.18);
  --skills-chip-border: rgba(148, 163, 184, 0.16);
  --skills-chip-bg: rgba(30, 41, 59, 0.56);
  --skills-chip-primary-bg: rgba(15, 23, 42, 0.48);
  --skills-chip-text: #f8fafc;
  --skills-chip-soft-text: rgba(226, 232, 240, 0.78);
  --skills-meta-border: rgba(148, 163, 184, 0.16);
  --skills-meta-bg: rgba(15, 23, 42, 0.38);
  --skills-meta-text: #cbd5e1;
  --skills-action-border: rgba(148, 163, 184, 0.16);
  --skills-action-bg: rgba(8, 14, 27, 0.34);
  --skills-action-text: #e2e8f0;
  --skills-detail-backdrop: rgba(2, 6, 23, 0.72);
  --skills-detail-modal-border: rgba(100, 116, 139, 0.28);
  --skills-detail-modal-top: rgba(18, 27, 45, 0.99);
  --skills-detail-modal-bottom: rgba(8, 13, 26, 1);
  --skills-detail-shadow:
    0 56px 140px -52px rgba(2, 6, 23, 0.92), 0 24px 44px -30px rgba(2, 132, 199, 0.18);
  --skills-detail-close-bg: rgba(148, 163, 184, 0.14);
  --skills-detail-close-bg-hover: rgba(148, 163, 184, 0.22);
  --skills-detail-section-border: rgba(148, 163, 184, 0.16);
  --skills-detail-section-bg: rgba(255, 255, 255, 0.04);
  --skills-detail-section-bg-strong: rgba(15, 23, 42, 0.42);
  --skills-detail-label: #94a3b8;
  --skills-detail-code-bg: rgba(15, 23, 42, 0.58);
  --skills-detail-code-border: rgba(148, 163, 184, 0.18);
  --skills-detail-link: #7dd3fc;
}

.skill-gallery {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

:global(.light .skill-tab.skill-gallery),
:global([data-theme='light'] .skill-tab.skill-gallery),
:global(.light .skill-detail-modal-backdrop),
:global([data-theme='light'] .skill-detail-modal-backdrop) {
  --skills-shell-border: rgba(203, 213, 225, 0.82);
  --skills-shell-bg-top: rgba(255, 255, 255, 0.98);
  --skills-shell-bg-bottom: rgba(239, 244, 249, 0.96);
  --skills-shell-shadow:
    0 20px 32px -24px rgba(15, 23, 42, 0.14), 0 12px 22px -18px rgba(59, 130, 246, 0.08);
  --skills-shell-hint-bg: rgba(22, 163, 74, 0.12);
  --skills-shell-hint-text: #15803d;
  --skills-stat-border: rgba(203, 213, 225, 0.88);
  --skills-stat-bg: rgba(255, 255, 255, 0.84);
  --skills-stat-text: #64748b;
  --skills-stat-value: #0f172a;
  --skills-card-border: rgba(203, 213, 225, 0.88);
  --skills-card-bg-top: rgba(255, 255, 255, 0.98);
  --skills-card-bg-bottom: rgba(244, 248, 251, 0.98);
  --skills-card-shadow:
    0 18px 30px -22px rgba(15, 23, 42, 0.12), 0 10px 18px -16px rgba(59, 130, 246, 0.08);
  --skills-card-shadow-active:
    0 22px 34px -22px rgba(15, 23, 42, 0.14), 0 14px 22px -16px rgba(59, 130, 246, 0.1);
  --skills-card-title: #0f172a;
  --skills-card-text: #475569;
  --skills-card-outline: rgba(255, 255, 255, 0.8);
  --skills-card-highlight: rgba(148, 163, 184, 0.04);
  --skills-preview-border: rgba(148, 163, 184, 0.18);
  --skills-preview-top: rgba(255, 255, 255, 0.9);
  --skills-preview-mid: rgba(245, 249, 252, 0.92);
  --skills-preview-bottom: rgba(232, 241, 247, 0.96);
  --skills-preview-enabled-border: rgba(134, 239, 172, 0.48);
  --skills-preview-enabled-tint: rgba(16, 185, 129, 0.16);
  --skills-preview-disabled-border: rgba(191, 219, 254, 0.78);
  --skills-preview-disabled-tint: rgba(59, 130, 246, 0.12);
  --skills-icon-bg-top: rgba(255, 255, 255, 0.9);
  --skills-icon-bg-bottom: rgba(191, 219, 254, 0.34);
  --skills-icon-border: rgba(148, 163, 184, 0.2);
  --skills-icon-fg: #0f172a;
  --skills-eyebrow-text: rgba(37, 99, 235, 0.78);
  --skills-badge-border: rgba(203, 213, 225, 0.92);
  --skills-badge-bg: rgba(248, 250, 252, 0.92);
  --skills-badge-text: #475569;
  --skills-state-border: rgba(203, 213, 225, 0.88);
  --skills-state-bg: rgba(255, 255, 255, 0.82);
  --skills-state-text: #334155;
  --skills-state-enabled-text: #166534;
  --skills-state-enabled-bg: rgba(34, 197, 94, 0.12);
  --skills-state-enabled-border: rgba(134, 239, 172, 0.46);
  --skills-state-disabled-text: #475569;
  --skills-state-disabled-bg: rgba(248, 250, 252, 0.94);
  --skills-state-disabled-border: rgba(203, 213, 225, 0.92);
  --skills-chip-border: rgba(203, 213, 225, 0.88);
  --skills-chip-bg: rgba(248, 250, 252, 0.94);
  --skills-chip-primary-bg: rgba(239, 246, 255, 0.96);
  --skills-chip-text: #334155;
  --skills-chip-soft-text: #64748b;
  --skills-meta-border: rgba(203, 213, 225, 0.88);
  --skills-meta-bg: rgba(248, 250, 252, 0.92);
  --skills-meta-text: #475569;
  --skills-action-border: rgba(203, 213, 225, 0.88);
  --skills-action-bg: rgba(255, 255, 255, 0.9);
  --skills-action-text: #0f172a;
  --skills-detail-backdrop: rgba(226, 232, 240, 0.66);
  --skills-detail-modal-border: rgba(203, 213, 225, 0.88);
  --skills-detail-modal-top: rgba(255, 255, 255, 0.99);
  --skills-detail-modal-bottom: rgba(243, 247, 250, 0.98);
  --skills-detail-shadow:
    0 48px 120px -52px rgba(15, 23, 42, 0.3), 0 20px 36px -24px rgba(59, 130, 246, 0.12);
  --skills-detail-close-bg: rgba(226, 232, 240, 0.88);
  --skills-detail-close-bg-hover: rgba(203, 213, 225, 0.96);
  --skills-detail-section-border: rgba(203, 213, 225, 0.86);
  --skills-detail-section-bg: rgba(255, 255, 255, 0.78);
  --skills-detail-section-bg-strong: rgba(248, 250, 252, 0.96);
  --skills-detail-label: #64748b;
  --skills-detail-code-bg: rgba(241, 245, 249, 0.98);
  --skills-detail-code-border: rgba(203, 213, 225, 0.86);
  --skills-detail-link: #0369a1;
}

.skills-showcase {
  padding: 14px;
  border: 1px solid var(--skills-shell-border);
  border-radius: 20px;
  background: var(--skills-shell-bg-bottom);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.08),
    var(--skills-shell-shadow);
}

.skills-showcase__lead {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(420px, 0.92fr);
  gap: 10px;
  margin-bottom: 10px;
}

.skills-showcase__intro {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.skills-showcase__title {
  margin: 0;
  color: var(--text-primary);
  font-size: clamp(1.1rem, 1vw + 0.95rem, 1.5rem);
  line-height: 1.1;
  letter-spacing: -0.03em;
}

.skills-showcase__hint {
  display: inline-flex;
  align-items: flex-start;
  gap: 9px;
  margin: 0;
  color: var(--text-secondary);
  font-size: 12px;
  line-height: 1.5;
  max-width: 42rem;
}

.skills-showcase__hint-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  border-radius: 999px;
  background: var(--skills-shell-hint-bg);
  color: var(--skills-shell-hint-text);
  font-size: 10px;
  font-weight: 700;
  flex-shrink: 0;
}

.skills-showcase__stats {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 5px;
}

.skills-showcase__stat-card {
  min-height: 54px;
  padding: 7px;
  border: 1px solid var(--skills-stat-border);
  border-radius: 11px;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  gap: 6px;
  background: linear-gradient(
    180deg,
    color-mix(in srgb, var(--skills-stat-bg) 88%, white 4%) 0%,
    var(--skills-stat-bg) 100%
  );
  color: var(--skills-stat-text);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.06);
}

.skills-showcase__stat-card span {
  font-size: 6.5px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.skills-showcase__stat-card strong {
  color: var(--skills-stat-value);
  font-size: clamp(0.86rem, 0.4vw + 0.76rem, 1rem);
  line-height: 1;
  letter-spacing: -0.03em;
}

.skills-showcase__filters {
  margin-bottom: 0;
  padding: 8px;
  border-radius: 14px;
  border: none;
  background: color-mix(in srgb, var(--skills-stat-bg) 84%, transparent);
  box-shadow: none;
}

.skills-showcase__search {
  min-width: min(240px, 100%);
  max-width: 320px;
}

.skill-gallery__grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

.skill-showcase-card {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 14px;
  border: 1px solid var(--skills-card-border);
  border-radius: 20px;
  background:
    radial-gradient(
      circle at top right,
      color-mix(in srgb, var(--skill-accent-a) 10%, transparent),
      transparent 32%
    ),
    linear-gradient(180deg, var(--skills-card-bg-top) 0%, var(--skills-card-bg-bottom) 100%);
  box-shadow:
    inset 0 1px 0 var(--skills-card-outline),
    var(--skills-card-shadow);
  cursor: pointer;
  overflow: hidden;
  transition:
    transform 0.22s ease,
    box-shadow 0.22s ease,
    border-color 0.22s ease,
    background 0.22s ease;
}

.skill-showcase-card::before,
.skill-showcase-card::after {
  content: none;
  position: absolute;
  pointer-events: none;
}

.skill-showcase-card:hover,
.skill-showcase-card:focus-visible,
.skill-showcase-card--active {
  outline: none;
  transform: translateY(-2px);
  border-color: var(--skill-accent-ring);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.08),
    var(--skills-card-shadow-active),
    0 0 0 1px color-mix(in srgb, var(--skill-accent-ring) 42%, transparent);
}

.skill-showcase-card--disabled {
  opacity: 0.9;
  filter: saturate(0.88);
}

.skill-showcase-card__topline {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
}

.skill-showcase-card__hero {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 0;
  border: 0;
  border-radius: 0;
  background: transparent;
  box-shadow: none;
}

.skill-showcase-card__orb,
.detail-icon {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 48px;
  height: 48px;
  border-radius: 16px;
  background: var(--skill-icon-bg);
  border: 1px solid var(--skill-icon-border);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.18),
    0 18px 28px -24px var(--skill-accent-glow);
  color: var(--skill-icon-fg);
  flex-shrink: 0;
}

.detail-icon {
  width: 72px;
  height: 72px;
  border-radius: 22px;
  font-size: 28px;
}

.skill-showcase-card__hero-copy {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
  flex: 1;
}

.skill-showcase-card__orb-image,
.detail-icon__image {
  width: 20px;
  height: 20px;
  object-fit: contain;
  filter: drop-shadow(0 8px 14px rgba(15, 23, 42, 0.28));
}

.detail-icon__image {
  width: 36px;
  height: 36px;
}

.skill-showcase-card__orb-fallback {
  font-size: 20px;
  font-weight: 700;
  line-height: 1;
  filter: drop-shadow(0 8px 14px rgba(15, 23, 42, 0.28));
}

.skill-showcase-card__title-row,
.detail-title-copy__row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.skill-showcase-card__title-row h3 {
  margin: 0;
  color: var(--skills-card-title);
  font-size: 1.02rem;
  line-height: 1.15;
  letter-spacing: -0.025em;
}

.skill-showcase-card__id {
  display: inline-flex;
  width: fit-content;
  padding: 2px 5px;
  border-radius: 999px;
  border: 1px solid var(--skills-detail-code-border);
  background: var(--skills-detail-code-bg);
  color: var(--skills-detail-label);
  font-size: 8px;
  line-height: 1;
}

.skill-showcase-card__id--detail {
  margin-top: 2px;
}

.detail-title-copy h2 {
  margin: 0;
  color: var(--text-primary);
  font-size: 24px;
  line-height: 1.15;
  letter-spacing: -0.03em;
}

.skill-showcase-card__hero-copy p {
  margin: 0;
  color: var(--skills-card-text);
  font-size: 0.84rem;
  line-height: 1.42;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.detail-title-copy p {
  margin: 4px 0 0;
  color: var(--text-secondary);
  font-size: 14px;
  line-height: 1.6;
}

.skill-showcase-card__badge {
  display: inline-flex;
  align-items: center;
  padding: 3px 7px;
  border-radius: 999px;
  border: 1px solid var(--skills-badge-border);
  background: var(--skills-badge-bg);
  color: var(--skills-badge-text);
  font-size: 7.5px;
  font-weight: 600;
  white-space: nowrap;
}

.skill-showcase-card__state {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 3px 7px;
  border-radius: 999px;
  border: 1px solid var(--skills-state-border);
  background: var(--skills-state-bg);
  color: var(--skills-state-text);
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
  font-size: 7.5px;
  font-weight: 600;
  white-space: nowrap;
}

.skill-showcase-card__state-dot {
  width: 4px;
  height: 4px;
  border-radius: 50%;
  background: currentColor;
}

.skill-showcase-card__state--enabled {
  color: var(--skills-state-enabled-text);
  background: var(--skills-state-enabled-bg);
  border-color: var(--skills-state-enabled-border);
}

.skill-showcase-card__state--disabled {
  color: var(--skills-state-disabled-text);
  background: var(--skills-state-disabled-bg);
  border-color: var(--skills-state-disabled-border);
}

.skill-showcase-card__content {
  position: relative;
  z-index: 1;
  min-width: 0;
  margin-top: auto;
  padding-top: 2px;
}

.skill-showcase-card__metrics {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
}

.skill-showcase-card__metric {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 0;
  padding: 0;
  border-radius: 0;
  border: 0;
  background: transparent;
}

.skill-showcase-card__metrics--rail {
  row-gap: 3px;
}

.skill-showcase-card__metric--inline {
  padding: 0;
  color: var(--skills-detail-label);
  font-size: 10px;
}

.skill-showcase-card__metric-icon {
  width: 11px;
  height: 11px;
  color: var(--skill-accent-a);
  flex-shrink: 0;
}

.skill-showcase-card__metric strong {
  color: var(--text-primary);
  font-size: 10px;
  line-height: 1.2;
}

.skill-showcase-card__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-top: auto;
  padding-top: 10px;
  border-top: 1px solid var(--skills-meta-border);
}

.skill-showcase-card__chips {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  align-items: center;
}

.skill-showcase-card__chip {
  display: inline-flex;
  align-items: center;
  padding: 2px 7px;
  border-radius: 999px;
  border: 1px solid var(--skills-chip-border);
  background: var(--skills-chip-bg);
  color: var(--skills-chip-text);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04);
  font-size: 7.5px;
  font-weight: 600;
}

.skill-showcase-card__chip--primary {
  background: var(--skills-chip-primary-bg);
}

.skill-showcase-card__chip--soft {
  color: var(--skills-chip-soft-text);
  background: rgba(255, 255, 255, 0.56);
}

.skill-showcase-card__chip--contract {
  border-color: color-mix(in srgb, var(--skill-accent-ring) 74%, transparent);
  background: color-mix(in srgb, var(--skill-accent-soft) 92%, white 6%);
  color: color-mix(in srgb, var(--skill-accent-a) 72%, var(--skills-chip-text));
}

.skill-showcase-card__link-hint {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: var(--primary);
  font-size: 9px;
  font-weight: 700;
  white-space: nowrap;
}

.skill-showcase-card__link-hint::after {
  content: '↗';
  font-size: 9px;
  line-height: 1;
}

.skill-detail-modal-backdrop {
  position: fixed;
  inset: 0;
  z-index: 1100;
  display: flex;
  align-items: flex-start;
  justify-content: center;
  padding: 16px;
  overflow-y: auto;
  overscroll-behavior: contain;
  background: var(--skills-detail-backdrop);
  backdrop-filter: blur(14px);
  -webkit-backdrop-filter: blur(14px);
}

.skill-detail-modal {
  position: relative;
  width: min(1040px, 100%);
  border-radius: 22px;
  border: 1px solid var(--skills-detail-modal-border);
  background:
    radial-gradient(
      circle at top right,
      color-mix(in srgb, var(--skill-accent-soft) 70%, transparent),
      transparent 30%
    ),
    linear-gradient(
      180deg,
      var(--skills-detail-modal-top) 0%,
      var(--skills-detail-modal-bottom) 100%
    );
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.08),
    var(--skills-detail-shadow);
  overflow: hidden;
}

.skill-detail-modal__handle {
  width: 56px;
  height: 6px;
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.34);
  margin: 14px auto -6px;
}

.skill-detail-modal__close {
  position: sticky;
  top: 14px;
  float: right;
  margin: 14px 14px 0 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  border: 0;
  border-radius: 999px;
  background: var(--skills-detail-close-bg);
  color: var(--text-secondary);
  font-size: 20px;
  cursor: pointer;
  z-index: 1;
  transition:
    background 0.2s ease,
    color 0.2s ease,
    transform 0.2s ease;
}

.skill-detail-modal__close:hover {
  color: var(--text-primary);
  background: var(--skills-detail-close-bg-hover);
  transform: translateY(-1px);
}

.skill-detail-content {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 16px 16px 20px;
}

.detail-layout {
  display: grid;
  grid-template-columns: minmax(290px, 0.82fr) minmax(0, 1.48fr);
  gap: 14px;
  align-items: start;
}

.detail-sidebar,
.detail-main {
  display: flex;
  flex-direction: column;
  gap: 14px;
  min-width: 0;
}

.detail-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 14px;
  padding: 16px;
  border: 1px solid var(--skills-detail-section-border);
  border-radius: 22px;
  background: linear-gradient(
    180deg,
    color-mix(in srgb, var(--skills-detail-section-bg) 88%, white 4%) 0%,
    var(--skills-detail-section-bg) 100%
  );
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.08),
    0 18px 28px -26px rgba(2, 6, 23, 0.38);
}

.detail-title-row {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  min-width: 0;
}

.detail-title-copy {
  min-width: 0;
}

.detail-title-copy__row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.detail-title-copy__row h2 {
  margin: 0;
  color: var(--text-primary);
  font-size: clamp(1.55rem, 1vw + 1.2rem, 2rem);
  line-height: 1.04;
  letter-spacing: -0.04em;
}

.detail-title-copy p {
  margin: 8px 0 0;
  max-width: 56ch;
  color: var(--text-secondary);
  font-size: 13px;
  line-height: 1.62;
}

.detail-pill-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 8px;
  align-items: center;
}

.detail-version-pill {
  display: inline-flex;
  align-items: center;
  border-radius: 999px;
  padding: 6px 12px;
  border: 1px solid rgba(34, 197, 94, 0.22);
  background: rgba(34, 197, 94, 0.12);
  color: #16a34a;
  font-size: 12px;
  font-weight: 700;
}

.detail-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  justify-content: flex-end;
  align-self: flex-start;
}

.detail-management-note {
  margin: 8px 0 0;
  max-width: 28rem;
  color: var(--text-secondary);
  font-size: 12px;
  line-height: 1.5;
  text-align: right;
  align-self: flex-end;
}

.detail-uninstall-button {
  border: 1px solid rgba(248, 113, 113, 0.28);
  background: rgba(127, 29, 29, 0.14);
  color: #fca5a5;
  border-radius: 999px;
  padding: 9px 14px;
  font-size: 12px;
  font-weight: 600;
  line-height: 1;
  cursor: pointer;
  transition:
    transform 0.18s ease,
    background 0.18s ease,
    border-color 0.18s ease;
}

.detail-uninstall-button:hover:not(:disabled) {
  transform: translateY(-1px);
  background: rgba(127, 29, 29, 0.2);
  border-color: rgba(248, 113, 113, 0.38);
}

.detail-uninstall-button:disabled {
  opacity: 0.64;
  cursor: not-allowed;
}

.detail-hero-stats {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.detail-overview-panel {
  padding: 12px;
}

.detail-overview-panel .detail-meta-grid {
  margin-top: 10px;
  grid-template-columns: 1fr;
}

.detail-contract-panel {
  margin-top: 10px;
}

.detail-hero-stat {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-height: 88px;
  justify-content: center;
  align-items: center;
  text-align: center;
  padding: 12px;
  border-radius: 18px;
  border: 1px solid var(--skills-detail-section-border);
  background:
    radial-gradient(circle at top right, var(--skill-accent-soft) 0%, transparent 54%),
    color-mix(in srgb, var(--skills-detail-section-bg) 92%, white 6%);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.08);
}

.detail-hero-stat span {
  font-size: 9px;
  color: var(--skills-detail-label);
  text-transform: uppercase;
  letter-spacing: 0.06em;
}

.detail-hero-stat strong {
  font-size: clamp(1.15rem, 0.7vw + 1rem, 1.65rem);
  line-height: 1;
  color: var(--text-primary);
  letter-spacing: -0.04em;
}

.detail-meta-grid {
  display: grid;
  gap: 8px;
  grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
}

.meta-entry {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 12px 13px;
  border: 1px solid var(--skills-detail-section-border);
  border-radius: 16px;
  background: linear-gradient(
    180deg,
    color-mix(in srgb, var(--skills-detail-section-bg) 90%, white 4%) 0%,
    var(--skills-detail-section-bg) 100%
  );
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04);
}

.meta-entry span {
  font-size: 9px;
  color: var(--skills-detail-label);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.meta-entry strong,
.meta-entry code {
  font-size: 12px;
  color: var(--text-primary);
  word-break: break-word;
}

.detail-section {
  border: 1px solid var(--skills-detail-section-border);
  border-radius: 20px;
  padding: 12px;
  background: linear-gradient(
    180deg,
    color-mix(in srgb, var(--skills-detail-section-bg) 90%, white 4%) 0%,
    var(--skills-detail-section-bg) 100%
  );
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.04),
    0 18px 26px -28px rgba(2, 6, 23, 0.34);
}

.detail-section__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 10px;
}

.detail-section h4 {
  margin: 0;
  font-size: 11px;
  color: var(--text-primary);
  text-transform: uppercase;
  letter-spacing: 0.08em;
}

.detail-section__caption {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  flex-wrap: wrap;
  margin: 0;
  color: var(--skills-detail-label);
  font-size: 11px;
  line-height: 1.4;
  text-align: end;
}

.param-columns {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.detail-parameters .param-group {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
}

.param-columns .param-group + .param-group {
  margin-top: 0;
  padding-top: 0;
  border-top: 0;
}

.param-title {
  margin: 0 0 8px;
  font-size: 11px;
  font-weight: 600;
  color: var(--skills-detail-label);
}

.param-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.param-list li {
  padding: 11px 12px;
  border-radius: 16px;
  border: 1px solid var(--skills-detail-section-border);
  background: color-mix(in srgb, var(--skills-detail-section-bg-strong) 74%, transparent);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04);
}

.param-head {
  display: flex;
  align-items: center;
  gap: 7px;
  margin-bottom: 5px;
  flex-wrap: wrap;
}

.param-type {
  font-size: 10px;
  color: var(--skills-detail-label);
  border: 1px solid var(--skills-detail-section-border);
  border-radius: 999px;
  padding: 2px 7px;
}

.param-required {
  font-size: 10px;
  color: #f59e0b;
}

.param-list p {
  margin: 0;
  color: var(--text-secondary);
  font-size: 11px;
  line-height: 1.55;
}

.detail-docs .loading-content,
.detail-docs .no-content {
  min-height: 120px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: var(--text-secondary);
}

.detail-docs__surface {
  padding: 2px 0 0;
}

.list-empty {
  grid-column: 1 / -1;
  padding: 38px 16px;
  text-align: center;
  border: 1px dashed var(--skills-detail-section-border);
  border-radius: 18px;
  color: var(--text-secondary);
  background: var(--skills-detail-section-bg);
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

.skill-content {
  font-size: 13px;
  line-height: 1.72;
  color: var(--text-primary);
}

.skill-content :deep(h1) {
  margin: 0 0 12px;
  font-size: 22px;
  line-height: 1.12;
  letter-spacing: -0.035em;
}

.skill-content :deep(h2) {
  margin: 22px 0 10px;
  font-size: 17px;
  line-height: 1.2;
  letter-spacing: -0.02em;
}

.skill-content :deep(h3) {
  margin: 16px 0 8px;
  font-size: 14px;
  line-height: 1.3;
  letter-spacing: -0.01em;
}

.skill-content :deep(p) {
  margin: 0 0 10px;
}

.skill-content :deep(ul) {
  margin: 0 0 10px;
  padding-inline-start: 18px;
}

.skill-content :deep(ol) {
  margin: 0 0 10px;
  padding-inline-start: 20px;
}

.skill-content :deep(li) {
  margin: 0 0 4px;
}

.skill-content :deep(.markdown-table-wrap) {
  margin: 14px 0;
  overflow-x: auto;
  overflow-y: hidden;
  border: 1px solid var(--skills-detail-section-border);
  border-radius: 18px;
  background: color-mix(in srgb, var(--skills-detail-section-bg-strong) 92%, transparent);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04);
  scrollbar-gutter: stable both-edges;
}

.skill-content :deep(table) {
  width: 100%;
  min-width: 520px;
  border-collapse: separate;
  border-spacing: 0;
  border: 0;
  color: var(--text-primary);
}

.skill-content :deep(thead) {
  background: color-mix(in srgb, var(--skills-detail-code-bg) 78%, transparent);
}

.skill-content :deep(th),
.skill-content :deep(td) {
  border: 0;
  padding: 10px 12px;
  text-align: start;
  vertical-align: top;
  font-size: 12px;
  line-height: 1.55;
}

.skill-content :deep(tr > *:not(:last-child)) {
  border-inline-end: 1px solid var(--skills-detail-section-border);
}

.skill-content :deep(tr:not(:last-child) > *) {
  border-bottom: 1px solid var(--skills-detail-section-border);
}

.skill-content :deep(th) {
  color: var(--text-primary);
  font-weight: 700;
  white-space: nowrap;
  background: color-mix(in srgb, var(--skills-detail-code-bg) 88%, transparent);
}

.skill-content :deep(td) {
  color: var(--text-secondary);
  background: transparent;
}

.skill-content :deep(tbody tr:nth-child(even) td) {
  background: color-mix(in srgb, var(--skills-detail-section-bg) 88%, transparent);
}

.skill-content :deep(tbody tr:hover td) {
  background: color-mix(in srgb, var(--skills-detail-code-bg) 64%, transparent);
}

.skill-content :deep(blockquote) {
  margin: 14px 0;
  padding: 12px 16px;
  border-inline-start: 3px solid
    color-mix(in srgb, var(--skill-accent-a, var(--primary)) 44%, transparent);
  border-start-start-radius: 0;
  border-end-start-radius: 0;
  border-start-end-radius: 16px;
  border-end-end-radius: 16px;
  background: color-mix(in srgb, var(--skills-detail-section-bg) 90%, transparent);
  color: var(--text-secondary);
}

.skill-content :deep(hr) {
  border: 0;
  height: 1px;
  margin: 18px 0;
  background: var(--skills-detail-section-border);
}

.skill-content :deep(strong) {
  color: var(--text-primary);
  font-weight: 700;
}

.skill-content :deep(code) {
  background: var(--skills-detail-code-bg);
  color: var(--text-primary);
  border: 1px solid var(--skills-detail-code-border);
  padding: 2px 6px;
  border-radius: 6px;
  font-family:
    ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New',
    monospace;
  font-size: 11px;
}

.skill-content :deep(pre) {
  background: var(--skills-detail-code-bg);
  color: var(--text-primary);
  border: 1px solid var(--skills-detail-code-border);
  padding: 14px;
  border-radius: 18px;
  overflow-x: auto;
  margin: 14px 0;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04);
}

.skill-content :deep(pre code) {
  display: block;
  padding: 0;
  border: 0;
  border-radius: 0;
  background: transparent;
  font-size: 11px;
  line-height: 1.65;
}

.markdown-body {
  background: transparent !important;
  color: var(--text-primary) !important;
}

.skill-content :deep(a) {
  color: var(--skills-detail-link);
  text-decoration: none;
}

.skill-content :deep(a:hover) {
  text-decoration: underline;
}

@media (max-width: 1400px) {
  .skill-gallery__grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 980px) {
  .skills-showcase__lead {
    grid-template-columns: 1fr;
  }

  .skills-showcase__stats {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }

  .detail-layout {
    grid-template-columns: 1fr;
  }

  .detail-header {
    flex-direction: column;
    align-items: stretch;
  }

  .detail-overview-panel .detail-hero-stats {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .detail-overview-panel .detail-meta-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .param-columns {
    grid-template-columns: 1fr;
  }

  .param-group + .param-group {
    margin-top: 0;
    padding-top: 0;
    border-top: 0;
  }

  .skill-showcase-card__hero,
  .skill-showcase-card__footer {
    align-items: stretch;
  }

  .skill-showcase-card__footer {
    flex-direction: column;
  }

  .skill-gallery__grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 720px) {
  .skills-showcase {
    padding: 12px;
    border-radius: 18px;
  }

  .skills-showcase__title {
    font-size: 1.2rem;
  }

  .skills-showcase__stats {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .skill-gallery__grid {
    grid-template-columns: 1fr;
    gap: 12px;
  }

  .skill-showcase-card {
    padding: 15px;
    border-radius: 22px;
  }

  .skill-showcase-card__orb {
    width: 56px;
    height: 56px;
    border-radius: 18px;
  }

  .skill-detail-modal-backdrop {
    padding: 10px;
  }

  .skill-detail-modal {
    border-radius: 18px;
  }

  .skill-detail-content {
    padding: 14px;
    gap: 12px;
  }

  .detail-header {
    padding: 14px;
    border-radius: 18px;
  }

  .detail-section__head {
    flex-direction: column;
    align-items: stretch;
    gap: 6px;
  }

  .detail-section__caption {
    text-align: start;
  }

  .detail-title-row {
    gap: 12px;
  }

  .detail-overview-panel .detail-hero-stats,
  .detail-overview-panel .detail-meta-grid,
  .detail-meta-grid {
    grid-template-columns: 1fr;
  }

  .detail-docs__surface {
    padding: 14px;
  }

  .detail-title-copy h2 {
    font-size: 17px;
  }

  .detail-icon {
    width: 60px;
    height: 60px;
    border-radius: 18px;
  }
}
</style>
