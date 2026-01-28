<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { toolApi } from '@/api/tool'
import type { Tool, ToolStoreItem } from '@/api/tool'

const { t } = useI18n()

const loading = ref(false)
const installedTools = ref<Tool[]>([])
const storeTools = ref<ToolStoreItem[]>([])
const activeTab = ref<'installed' | 'store'>('installed')
const searchQuery = ref('')
const installing = ref<string | null>(null)

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
  // Auto refresh store on first load if empty
  if (storeTools.value.length === 0) {
    await refreshStore()
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
    </div>

    <!-- Search -->
    <div class="mb-6">
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
  </div>
</template>
