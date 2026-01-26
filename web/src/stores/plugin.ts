import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { pluginApi } from '@/api/plugin'
import type { Plugin, PluginLog } from '@/api/plugin'

export const usePluginStore = defineStore('plugin', () => {
  // State
  const plugins = ref<Plugin[]>([])
  const selectedPluginId = ref<string | null>(null)
  const pluginLogs = ref<Record<string, PluginLog[]>>({})
  const loading = ref(false)
  const error = ref<string | null>(null)

  // Computed
  const selectedPlugin = computed(() =>
    plugins.value.find((p) => p.id === selectedPluginId.value)
  )

  const enabledPlugins = computed(() => plugins.value.filter((p) => p.enabled))

  const disabledPlugins = computed(() => plugins.value.filter((p) => !p.enabled))

  const pluginsByType = computed(() => {
    const grouped: Record<string, Plugin[]> = {
      native: [],
      js: [],
      wasm: [],
    }
    plugins.value.forEach((p) => {
      if (grouped[p.type]) {
        grouped[p.type].push(p)
      }
    })
    return grouped
  })

  // Actions
  async function fetchPlugins() {
    try {
      loading.value = true
      error.value = null
      const response = await pluginApi.list()
      plugins.value = response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch plugins'
    } finally {
      loading.value = false
    }
  }

  async function fetchPlugin(id: string) {
    try {
      loading.value = true
      error.value = null
      const response = await pluginApi.get(id)
      const index = plugins.value.findIndex((p) => p.id === id)
      if (index !== -1) {
        plugins.value[index] = response.data
      } else {
        plugins.value.push(response.data)
      }
      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch plugin'
      return null
    } finally {
      loading.value = false
    }
  }

  async function enablePlugin(id: string) {
    try {
      loading.value = true
      error.value = null
      await pluginApi.enable(id)
      const plugin = plugins.value.find((p) => p.id === id)
      if (plugin) {
        plugin.enabled = true
        plugin.status = 'running'
      }
      return true
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to enable plugin'
      return false
    } finally {
      loading.value = false
    }
  }

  async function disablePlugin(id: string) {
    try {
      loading.value = true
      error.value = null
      await pluginApi.disable(id)
      const plugin = plugins.value.find((p) => p.id === id)
      if (plugin) {
        plugin.enabled = false
        plugin.status = 'stopped'
      }
      return true
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to disable plugin'
      return false
    } finally {
      loading.value = false
    }
  }

  async function updatePluginConfig(id: string, config: Record<string, unknown>) {
    try {
      loading.value = true
      error.value = null
      await pluginApi.updateConfig(id, config)
      const plugin = plugins.value.find((p) => p.id === id)
      if (plugin) {
        plugin.config = config
      }
      return true
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to update plugin config'
      return false
    } finally {
      loading.value = false
    }
  }

  async function fetchPluginLogs(id: string, params?: { limit?: number; level?: string }) {
    try {
      const response = await pluginApi.getLogs(id, params)
      pluginLogs.value = { ...pluginLogs.value, [id]: response.data }
      return response.data
    } catch (e) {
      console.error('Failed to fetch plugin logs:', e)
      return []
    }
  }

  async function reloadPlugin(id: string) {
    try {
      loading.value = true
      error.value = null
      await pluginApi.reload(id)
      await fetchPlugin(id)
      return true
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to reload plugin'
      return false
    } finally {
      loading.value = false
    }
  }

  function selectPlugin(id: string | null) {
    selectedPluginId.value = id
  }

  function clearError() {
    error.value = null
  }

  return {
    // State
    plugins,
    selectedPluginId,
    pluginLogs,
    loading,
    error,

    // Computed
    selectedPlugin,
    enabledPlugins,
    disabledPlugins,
    pluginsByType,

    // Actions
    fetchPlugins,
    fetchPlugin,
    enablePlugin,
    disablePlugin,
    updatePluginConfig,
    fetchPluginLogs,
    reloadPlugin,
    selectPlugin,
    clearError,
  }
})
