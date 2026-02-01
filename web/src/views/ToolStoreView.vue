<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { toolApi } from '@/api/tool'
import { skillApi, type RemoteSkill, type SearchParams } from '@/api/skill'
import type { Tool, ToolStoreItem } from '@/api/tool'

const { t } = useI18n()

const loading = ref(false)
const installedTools = ref<Tool[]>([])
const storeTools = ref<ToolStoreItem[]>([])
const activeTab = ref<'installed' | 'store' | 'skills'>('installed')
const searchQuery = ref('')
const installing = ref<string | null>(null)

// Skill store state
const skills = ref<RemoteSkill[]>([])
const skillsLoading = ref(false)
const skillsError = ref<string | null>(null)
const skillCategories = ref<string[]>([])
const skillFilterCategory = ref<string>('all')
const skillSortBy = ref<'downloads' | 'stars' | 'updated' | 'name'>('downloads')
const skillInstalling = ref<Set<string>>(new Set())
const skillPage = ref(1)
const skillPageSize = 20
const skillTotal = ref(0)
const skillTotalPages = computed(() => Math.ceil(skillTotal.value / skillPageSize))

const filteredInstalledTools = computed(() => {
  if (!searchQuery.value) return installedTools.value
  const query = searchQuery.value.toLowerCase()
  return installedTools.value.filter(t =>
    t.name.toLowerCase().includes(query) ||
    t.description?.toLowerCase().includes(query)
  )
})

const filteredStoreTools = computed(() => {
  if (!searchQuery.value) return storeTools.value
  const query = searchQuery.value.toLowerCase()
  return storeTools.value.filter(t =>
    t.name.toLowerCase().includes(query) ||
    t.description?.toLowerCase().includes(query)
  )
})

onMounted(async () => {
  await loadInstalledTools()
  await loadStoreTools()
  if (storeTools.value.length === 0) {
    await refreshStore()
  }
})

// Load skills when switching to skills tab
watch(activeTab, async (newTab) => {
  if (newTab === 'skills' && skills.value.length === 0) {
    await fetchSkills()
    await fetchSkillCategories()
  }
})

async function refreshStore() {
  try {
    loading.value = true
    await toolApi.refresh()
    await loadStoreTools()
  } catch {
    // Handle error
  } finally {
    loading.value = false
  }
}

async function loadInstalledTools() {
  try {
    loading.value = true
    const response = await toolApi.list()
    installedTools.value = response.data
  } catch {
    installedTools.value = []
  } finally {
    loading.value = false
  }
}

async function loadStoreTools() {
  try {
    const response = await toolApi.listStore()
    storeTools.value = response.data
  } catch {
    storeTools.value = []
  }
}

async function installTool(tool: ToolStoreItem) {
  try {
    installing.value = tool.id
    await toolApi.install(tool.id)
    await loadInstalledTools()
  } catch {
    // Handle error
  } finally {
    installing.value = null
  }
}

async function uninstallTool(tool: Tool) {
  if (!confirm(t('tool.confirmUninstall', { name: tool.name }))) return

  try {
    await toolApi.uninstall(tool.id)
    await loadInstalledTools()
  } catch {
    // Handle error
  }
}

async function toggleTool(tool: Tool) {
  try {
    if (tool.enabled) {
      await toolApi.disable(tool.id)
    } else {
      await toolApi.enable(tool.id)
    }
    await loadInstalledTools()
  } catch {
    // Handle error
  }
}

function isInstalled(toolId: string): boolean {
  return installedTools.value.some(t => t.id === toolId)
}

// Skill store functions
async function fetchSkills() {
  skillsLoading.value = true
  skillsError.value = null
  try {
    const params: SearchParams = {
      page: skillPage.value,
      page_size: skillPageSize,
      sort_by: skillSortBy.value,
      sort_order: 'desc',
    }
    if (searchQuery.value) {
      params.q = searchQuery.value
    }
    if (skillFilterCategory.value !== 'all') {
      params.categories = skillFilterCategory.value
    }
    const response = await skillApi.search(params)
    const skillsData = response.data.skills || []
    skills.value = skillsData.map(s => ({
      ...s,
      tags: s.tags ? s.tags.split(',').map(tag => tag.trim()) : [],
    })) as unknown as RemoteSkill[]
    skillTotal.value = response.data.total || 0
  } catch (err) {
    skillsError.value = err instanceof Error ? err.message : t('skillStore.fetchError')
  } finally {
    skillsLoading.value = false
  }
}

async function fetchSkillCategories() {
  try {
    const response = await skillApi.categories()
    skillCategories.value = response.data
  } catch {
    // Ignore
  }
}

async function installSkill(skill: RemoteSkill) {
  if (skillInstalling.value.has(skill.id)) return
  skillInstalling.value.add(skill.id)
  try {
    await skillApi.install(skill.id)
    skill.installed = true
  } catch (err) {
    skillsError.value = err instanceof Error ? err.message : t('skillStore.installError')
  } finally {
    skillInstalling.value.delete(skill.id)
  }
}

async function uninstallSkill(skill: RemoteSkill) {
  if (skillInstalling.value.has(skill.id)) return
  if (!confirm(t('skillStore.confirmUninstall', { name: skill.name }))) return
  skillInstalling.value.add(skill.id)
  try {
    await skillApi.uninstall(skill.id)
    skill.installed = false
  } catch (err) {
    skillsError.value = err instanceof Error ? err.message : t('skillStore.uninstallError')
  } finally {
    skillInstalling.value.delete(skill.id)
  }
}

function handleSkillSearch() {
  skillPage.value = 1
  fetchSkills()
}

function handleSkillPageChange(newPage: number) {
  skillPage.value = newPage
  fetchSkills()
}

async function loadMoreSkills() {
  skillPage.value++
  skillsLoading.value = true
  try {
    const params: SearchParams = {
      page: skillPage.value,
      page_size: skillPageSize,
      sort_by: skillSortBy.value,
      sort_order: 'desc',
    }
    if (searchQuery.value) {
      params.q = searchQuery.value
    }
    if (skillFilterCategory.value !== 'all') {
      params.categories = skillFilterCategory.value
    }
    const response = await skillApi.search(params)
    const skillsData = response.data.skills || []
    const newSkills = skillsData.map(s => ({
      ...s,
      tags: s.tags ? s.tags.split(',').map(tag => tag.trim()) : [],
    })) as unknown as RemoteSkill[]
    skills.value = [...skills.value, ...newSkills]
    skillTotal.value = response.data.total || skills.value.length
  } catch (err) {
    skillsError.value = err instanceof Error ? err.message : t('skillStore.fetchError')
    skillPage.value-- // Revert page on error
  } finally {
    skillsLoading.value = false
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
    system: '⚙️',
    communication: '💬',
    information: '📰',
  }
  return icons[category || ''] || '⚡'
}

function formatNumber(num?: number): string {
  if (!num) return '0'
  if (num >= 1000) return (num / 1000).toFixed(1) + 'k'
  return num.toString()
}
</script>

<template>
  <div class="tool-store-view p-4 sm:p-6 max-w-6xl mx-auto">
    <h1 class="text-xl sm:text-2xl font-bold text-gray-900 dark:text-white mb-6">{{ t('tool.title') }}</h1>

    <!-- Tabs -->
    <div class="flex gap-4 mb-6 border-b border-gray-200 dark:border-slate-700">
      <button
        :class="[
          'pb-3 px-1 text-sm font-medium border-b-2 transition-colors',
          activeTab === 'installed'
            ? 'border-accent text-accent'
            : 'border-transparent text-gray-500 dark:text-slate-400 hover:text-gray-900 dark:hover:text-white'
        ]"
        @click="activeTab = 'installed'"
      >
        {{ t('tool.installed') }} ({{ installedTools.length }})
      </button>
      <button
        :class="[
          'pb-3 px-1 text-sm font-medium border-b-2 transition-colors',
          activeTab === 'store'
            ? 'border-accent text-accent'
            : 'border-transparent text-gray-500 dark:text-slate-400 hover:text-gray-900 dark:hover:text-white'
        ]"
        @click="activeTab = 'store'"
      >
        {{ t('tool.store') }}
      </button>
      <button
        :class="[
          'pb-3 px-1 text-sm font-medium border-b-2 transition-colors',
          activeTab === 'skills'
            ? 'border-accent text-accent'
            : 'border-transparent text-gray-500 dark:text-slate-400 hover:text-gray-900 dark:hover:text-white'
        ]"
        @click="activeTab = 'skills'"
      >
        {{ t('skillStore.title') }}
      </button>
    </div>

    <!-- Search (for installed and store tabs) -->
    <div v-if="activeTab !== 'skills'" class="mb-6">
      <input
        v-model="searchQuery"
        type="text"
        :placeholder="t('tool.searchPlaceholder')"
        class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-accent border border-gray-200 dark:border-slate-600"
      />
    </div>

    <!-- Installed Tools -->
    <div v-if="activeTab === 'installed'">
      <div v-if="loading" class="text-center py-8 text-gray-500 dark:text-slate-400">
        {{ t('common.loading') }}
      </div>

      <div v-else-if="filteredInstalledTools.length === 0" class="text-center py-8 text-gray-500 dark:text-slate-400">
        {{ t('tool.noInstalledTools') }}
      </div>

      <div v-else class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
        <div v-for="tool in filteredInstalledTools" :key="tool.id" class="glass-card p-4">
          <div class="flex items-start justify-between mb-2">
            <h3 class="text-gray-900 dark:text-white font-medium">{{ tool.name }}</h3>
            <span
              :class="[
                'px-2 py-0.5 rounded-full text-xs font-medium',
                tool.enabled
                  ? 'bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300'
                  : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300'
              ]"
            >
              {{ tool.enabled ? t('tool.enabled') : t('tool.disabled') }}
            </span>
          </div>
          <p class="text-sm text-gray-500 dark:text-slate-400 mb-4 line-clamp-2">
            {{ tool.description }}
          </p>
          <div class="flex gap-2">
            <button
              :class="[
                'flex-1 px-3 py-1.5 rounded text-sm transition-colors',
                tool.enabled
                  ? 'bg-yellow-100 dark:bg-yellow-900/30 text-yellow-700 dark:text-yellow-300 hover:bg-yellow-200 dark:hover:bg-yellow-900/50'
                  : 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300 hover:bg-green-200 dark:hover:bg-green-900/50'
              ]"
              @click="toggleTool(tool)"
            >
              {{ tool.enabled ? t('tool.disable') : t('tool.enable') }}
            </button>
            <button
              v-if="!tool.builtin"
              class="px-3 py-1.5 bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300 hover:bg-red-200 dark:hover:bg-red-900/50 rounded text-sm transition-colors"
              @click="uninstallTool(tool)"
            >
              {{ t('tool.uninstall') }}
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Store -->
    <div v-if="activeTab === 'store'">
      <div v-if="filteredStoreTools.length === 0" class="text-center py-8 text-gray-500 dark:text-slate-400">
        {{ t('tool.noStoreTools') }}
      </div>

      <div v-else class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
        <div v-for="tool in filteredStoreTools" :key="tool.id" class="glass-card p-4">
          <div class="flex items-start justify-between mb-2">
            <h3 class="text-gray-900 dark:text-white font-medium">{{ tool.name }}</h3>
            <span v-if="isInstalled(tool.id)" class="px-2 py-0.5 bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300 rounded-full text-xs font-medium">
              {{ t('tool.installed') }}
            </span>
          </div>
          <p class="text-sm text-gray-500 dark:text-slate-400 mb-2 line-clamp-2">
            {{ tool.description }}
          </p>
          <div class="flex items-center gap-2 text-xs text-gray-400 dark:text-slate-500 mb-4">
            <span>v{{ tool.version }}</span>
            <span>{{ tool.author }}</span>
          </div>
          <button
            v-if="!isInstalled(tool.id)"
            :disabled="installing === tool.id"
            class="w-full px-3 py-1.5 bg-accent hover:bg-accent-hover text-white rounded text-sm transition-colors disabled:opacity-50"
            @click="installTool(tool)"
          >
            {{ installing === tool.id ? t('tool.installing') : t('tool.install') }}
          </button>
        </div>
      </div>
    </div>

    <!-- Skills Store -->
    <div v-if="activeTab === 'skills'" class="skill-store-tab">
      <!-- Filters -->
      <div class="flex flex-wrap gap-3 mb-6 items-center">
        <div class="relative flex-1 min-w-[200px]">
          <svg class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <circle cx="11" cy="11" r="8" />
            <path d="m21 21-4.35-4.35" />
          </svg>
          <input
            v-model="searchQuery"
            type="text"
            :placeholder="t('skillStore.filters.searchSkillsPlaceholder')"
            class="w-full pl-10 pr-4 py-2 bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg border border-gray-200 dark:border-slate-600 focus:outline-none focus:ring-2 focus:ring-accent"
            @keyup.enter="handleSkillSearch"
          />
        </div>

        <select
          v-model="skillFilterCategory"
          class="px-3 py-2 bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg border border-gray-200 dark:border-slate-600 focus:outline-none focus:ring-2 focus:ring-accent"
          @change="handleSkillSearch"
        >
          <option value="all">{{ t('plugins.allCategories') }}</option>
          <option v-for="cat in skillCategories" :key="cat" :value="cat">
            {{ getCategoryIcon(cat) }} {{ cat }}
          </option>
        </select>

        <select
          v-model="skillSortBy"
          class="px-3 py-2 bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg border border-gray-200 dark:border-slate-600 focus:outline-none focus:ring-2 focus:ring-accent"
          @change="handleSkillSearch"
        >
          <option value="downloads">{{ t('skillStore.sort.downloads') }}</option>
          <option value="stars">{{ t('skillStore.sort.stars') }}</option>
          <option value="updated">{{ t('skillStore.sort.updated') }}</option>
          <option value="name">{{ t('skillStore.sort.name') }}</option>
        </select>

        <button
          class="px-3 py-2 bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg border border-gray-200 dark:border-slate-600 hover:bg-gray-200 dark:hover:bg-slate-600 transition-colors"
          :disabled="skillsLoading"
          @click="fetchSkills"
        >
          <svg v-if="!skillsLoading" class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M21 12a9 9 0 0 0-9-9 9.75 9.75 0 0 0-6.74 2.74L3 8" />
            <path d="M3 3v5h5" />
            <path d="M3 12a9 9 0 0 0 9 9 9.75 9.75 0 0 0 6.74-2.74L21 16" />
            <path d="M16 21h5v-5" />
          </svg>
          <div v-else class="w-4 h-4 border-2 border-gray-400 border-t-transparent rounded-full animate-spin"></div>
        </button>
      </div>

      <!-- Error message -->
      <div v-if="skillsError" class="mb-4 p-3 bg-red-100 dark:bg-red-900/30 border border-red-200 dark:border-red-800 rounded-lg flex justify-between items-center">
        <span class="text-red-700 dark:text-red-300 text-sm">{{ skillsError }}</span>
        <button class="text-red-700 dark:text-red-300 text-xl" @click="skillsError = null">&times;</button>
      </div>

      <!-- Loading -->
      <div v-if="skillsLoading && !skills.length" class="text-center py-12 text-gray-500 dark:text-slate-400">
        <div class="w-8 h-8 border-2 border-accent border-t-transparent rounded-full animate-spin mx-auto mb-3"></div>
        <span>{{ t('common.loading') }}</span>
      </div>

      <!-- Skills Grid -->
      <div v-else class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
        <div
          v-for="skill in skills"
          :key="skill.id"
          :class="['glass-card p-4 transition-all hover:shadow-lg', { 'border-green-500/30': skill.installed }]"
        >
          <div class="flex items-start gap-3 mb-3">
            <span class="w-9 h-9 flex items-center justify-center text-lg bg-gray-100 dark:bg-slate-700 rounded-lg border border-gray-200 dark:border-slate-600">
              {{ getCategoryIcon(skill.category) }}
            </span>
            <div class="flex-1 min-w-0">
              <h3 class="font-medium text-gray-900 dark:text-white truncate" :title="skill.name">{{ skill.name }}</h3>
              <div class="flex items-center gap-2 text-xs text-gray-500 dark:text-slate-400">
                <span v-if="skill.author">{{ skill.author }}</span>
                <span v-if="skill.version && skill.version !== '1.0.0' && skill.version !== 'v1.0.0'">v{{ skill.version }}</span>
              </div>
            </div>
            <button
              v-if="!skill.installed"
              class="px-3 py-1.5 text-xs font-medium text-white bg-accent hover:bg-accent-hover rounded-lg transition-colors disabled:opacity-50"
              :disabled="skillInstalling.has(skill.id)"
              @click="installSkill(skill)"
            >
              <span v-if="skillInstalling.has(skill.id)" class="w-3 h-3 border-2 border-white border-t-transparent rounded-full animate-spin inline-block"></span>
              <span v-else>{{ t('skillStore.install') }}</span>
            </button>
            <div v-else class="flex items-center gap-2">
              <span v-if="skill.builtin" class="px-2 py-1 text-xs font-medium text-green-700 dark:text-green-300 bg-green-100 dark:bg-green-900/30 rounded">
                {{ t('skillStore.status.builtin') }}
              </span>
              <button
                v-else
                class="px-2 py-1 text-xs font-medium text-red-700 dark:text-red-300 bg-red-100 dark:bg-red-900/30 hover:bg-red-200 dark:hover:bg-red-900/50 rounded transition-colors"
                :disabled="skillInstalling.has(skill.id)"
                @click="uninstallSkill(skill)"
              >
                <span v-if="skillInstalling.has(skill.id)" class="w-3 h-3 border-2 border-red-500 border-t-transparent rounded-full animate-spin inline-block"></span>
                <span v-else>{{ t('skillStore.actions.uninstall') }}</span>
              </button>
            </div>
          </div>

          <!-- Category badge -->
          <div v-if="skill.category" class="mb-2">
            <span class="px-2 py-0.5 text-xs font-medium bg-purple-100 dark:bg-purple-900/30 text-purple-700 dark:text-purple-300 rounded">
              {{ skill.category }}
            </span>
          </div>

          <p class="text-sm text-gray-500 dark:text-slate-400 mb-3 line-clamp-2">
            {{ skill.description || skill.summary || t('plugins.noDescription') }}
          </p>

          <div class="flex items-center gap-4 text-xs text-gray-400 dark:text-slate-500">
            <span class="flex items-center gap-1" :title="t('skillStore.downloads')">
              <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
                <polyline points="7,10 12,15 17,10" />
                <line x1="12" y1="15" x2="12" y2="3" />
              </svg>
              {{ formatNumber(skill.downloads) }}
            </span>
            <span v-if="skill.rating" class="flex items-center gap-1" :title="t('skillStore.rating')">
              <svg class="w-3.5 h-3.5 text-yellow-500" viewBox="0 0 24 24" fill="currentColor">
                <path d="M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z" />
              </svg>
              {{ skill.rating.toFixed(1) }}
            </span>
          </div>

          <div v-if="skill.tags?.length" class="flex flex-wrap gap-1 mt-3">
            <span
              v-for="tag in (Array.isArray(skill.tags) ? skill.tags : []).slice(0, 3)"
              :key="tag"
              class="px-2 py-0.5 text-xs bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300 rounded"
            >
              {{ tag }}
            </span>
          </div>
        </div>

        <div v-if="skills.length === 0 && !skillsLoading" class="col-span-full text-center py-12 text-gray-500 dark:text-slate-400">
          <svg class="w-12 h-12 mx-auto mb-3 opacity-50" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
            <path d="M9.75 9.75l4.5 4.5m0-4.5l-4.5 4.5M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          <p>{{ t('skillStore.noSkillsFound') }}</p>
        </div>
      </div>

      <!-- Pagination / Load More -->
      <div v-if="skills.length > 0" class="flex justify-center items-center gap-4 mt-6 pt-4 border-t border-gray-200 dark:border-slate-700">
        <template v-if="skillTotalPages > 1">
          <button
            :disabled="skillPage <= 1"
            class="px-4 py-2 text-sm bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg border border-gray-200 dark:border-slate-600 hover:bg-gray-200 dark:hover:bg-slate-600 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            @click="handleSkillPageChange(skillPage - 1)"
          >
            {{ t('common.previous') }}
          </button>
          <span class="text-sm text-gray-500 dark:text-slate-400">{{ skillPage }} / {{ skillTotalPages }}</span>
          <button
            :disabled="skillPage >= skillTotalPages"
            class="px-4 py-2 text-sm bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg border border-gray-200 dark:border-slate-600 hover:bg-gray-200 dark:hover:bg-slate-600 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            @click="handleSkillPageChange(skillPage + 1)"
          >
            {{ t('common.next') }}
          </button>
        </template>
        <template v-else-if="skills.length >= skillPageSize">
          <button
            :disabled="skillsLoading"
            class="px-4 py-2 text-sm bg-accent hover:bg-accent-hover text-white rounded-lg disabled:opacity-50 transition-colors"
            @click="loadMoreSkills"
          >
            {{ skillsLoading ? t('common.loading') : t('skillStore.actions.loadMore') }}
          </button>
        </template>
      </div>
    </div>
  </div>
</template>
