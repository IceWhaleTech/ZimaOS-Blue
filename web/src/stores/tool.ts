import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { toolApi } from '@/api/tool'
import type { Tool, ToolSource, RemoteTool, BrowseParams } from '@/api/tool'

export const useToolStore = defineStore('tool', () => {
  // State
  const tools = ref<Tool[]>([])
  const sources = ref<ToolSource[]>([])
  const remoteTools = ref<RemoteTool[]>([])
  const loading = ref(false)
  const refreshing = ref(false)
  const error = ref<string | null>(null)

  // Computed
  const enabledTools = computed(() => tools.value.filter((t) => t.enabled))
  const disabledTools = computed(() => tools.value.filter((t) => !t.enabled))
  const builtinTools = computed(() => tools.value.filter((t) => t.builtin))
  const installedTools = computed(() => tools.value.filter((t) => !t.builtin))

  const toolsByCategory = computed(() => {
    const grouped: Record<string, Tool[]> = {}
    tools.value.forEach((t) => {
      const category = t.category || 'other'
      if (!grouped[category]) {
        grouped[category] = []
      }
      grouped[category].push(t)
    })
    return grouped
  })

  const remoteToolsBySource = computed(() => {
    const grouped: Record<string, RemoteTool[]> = {}
    remoteTools.value.forEach((t) => {
      if (!grouped[t.source_id]) {
        grouped[t.source_id] = []
      }
      grouped[t.source_id]?.push(t)
    })
    return grouped
  })

  const categories = computed(() => {
    const cats = new Set<string>()
    tools.value.forEach((t) => {
      if (t.category) cats.add(t.category)
    })
    remoteTools.value.forEach((t) => {
      if (t.category) cats.add(t.category)
    })
    return Array.from(cats).sort()
  })

  // Actions
  async function fetchTools() {
    try {
      loading.value = true
      error.value = null
      const response = await toolApi.list()
      tools.value = response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch tools'
    } finally {
      loading.value = false
    }
  }

  async function enableTool(id: string) {
    try {
      loading.value = true
      error.value = null
      await toolApi.enable(id)
      const tool = tools.value.find((t) => t.id === id)
      if (tool) {
        tool.enabled = true
      }
      return true
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to enable tool'
      return false
    } finally {
      loading.value = false
    }
  }

  async function disableTool(id: string) {
    try {
      loading.value = true
      error.value = null
      await toolApi.disable(id)
      const tool = tools.value.find((t) => t.id === id)
      if (tool) {
        tool.enabled = false
      }
      return true
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to disable tool'
      return false
    } finally {
      loading.value = false
    }
  }

  async function fetchSources() {
    try {
      const response = await toolApi.listSources()
      sources.value = response.data
    } catch (e) {
      console.error('Failed to fetch sources:', e)
    }
  }

  async function addSource(source: Omit<ToolSource, 'enabled'> & { enabled?: boolean }) {
    try {
      loading.value = true
      error.value = null
      await toolApi.addSource(source)
      await fetchSources()
      return true
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to add source'
      return false
    } finally {
      loading.value = false
    }
  }

  async function removeSource(id: string) {
    try {
      loading.value = true
      error.value = null
      await toolApi.removeSource(id)
      sources.value = sources.value.filter((s) => s.id !== id)
      return true
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to remove source'
      return false
    } finally {
      loading.value = false
    }
  }

  async function browseTools(params?: BrowseParams) {
    try {
      loading.value = true
      error.value = null
      const response = await toolApi.browse(params)
      remoteTools.value = response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to browse tools'
    } finally {
      loading.value = false
    }
  }

  async function installTool(id: string) {
    try {
      loading.value = true
      error.value = null
      const response = await toolApi.install(id)
      if (response.data.success) {
        // Mark as installed in remote tools
        const tool = remoteTools.value.find((t) => t.id === id)
        if (tool) {
          tool.installed = true
        }
        // Refresh local tools
        await fetchTools()
      }
      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to install tool'
      return { success: false, message: error.value }
    } finally {
      loading.value = false
    }
  }

  async function uninstallTool(id: string) {
    try {
      loading.value = true
      error.value = null
      const response = await toolApi.uninstall(id)
      if (response.data.success) {
        // Remove from local tools
        tools.value = tools.value.filter((t) => t.id !== id)
        // Mark as not installed in remote tools
        const tool = remoteTools.value.find((t) => t.id === id)
        if (tool) {
          tool.installed = false
        }
      }
      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to uninstall tool'
      return { success: false, message: error.value }
    } finally {
      loading.value = false
    }
  }

  async function refreshSources() {
    try {
      refreshing.value = true
      error.value = null
      const response = await toolApi.refresh()
      if (response.data.errors && response.data.errors.length > 0) {
        error.value = response.data.errors.join(', ')
      }
      // Refresh browse results
      await browseTools()
      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to refresh sources'
      return { success: false, tools_count: 0 }
    } finally {
      refreshing.value = false
    }
  }

  function clearError() {
    error.value = null
  }

  return {
    // State
    tools,
    sources,
    remoteTools,
    loading,
    refreshing,
    error,

    // Computed
    enabledTools,
    disabledTools,
    builtinTools,
    installedTools,
    toolsByCategory,
    remoteToolsBySource,
    categories,

    // Actions
    fetchTools,
    enableTool,
    disableTool,
    fetchSources,
    addSource,
    removeSource,
    browseTools,
    installTool,
    uninstallTool,
    refreshSources,
    clearError,
  }
})
