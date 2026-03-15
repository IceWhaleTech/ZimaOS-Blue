import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { pluginApi } from '@/api/plugin'
import type { Plugin, PluginLog, PluginSource, RemotePlugin } from '@/api/plugin'

export const usePluginStore = defineStore('plugin', () => {
  // State
  const plugins = ref<Plugin[]>([])
  const selectedPluginId = ref<string | null>(null)
  const pluginLogs = ref<Record<string, PluginLog[]>>({})
  const loading = ref(false)
  const error = ref<string | null>(null)

  // Store state
  const sources = ref<PluginSource[]>([])
  const remotePlugins = ref<RemotePlugin[]>([])
  const refreshing = ref(false)

  // Computed
  const selectedPlugin = computed(() => plugins.value.find((p) => p.id === selectedPluginId.value))

  const enabledPlugins = computed(() => plugins.value.filter((p) => p.enabled))

  const disabledPlugins = computed(() => plugins.value.filter((p) => !p.enabled))

  const pluginsByType = computed(() => {
    const grouped: Record<string, Plugin[]> = {
      native: [],
      js: [],
      wasm: [],
    }
    plugins.value.forEach((p) => {
      const group = grouped[p.type]
      if (group) {
        group.push(p)
      }
    })
    return grouped
  })

  const availablePlugins = computed(() => remotePlugins.value.filter((p) => !p.installed))

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

  // Store actions
  async function fetchSources() {
    try {
      const response = await pluginApi.listSources()
      sources.value = response.data
    } catch (e) {
      console.error('Failed to fetch plugin sources:', e)
    }
  }

  async function addSource(source: Omit<PluginSource, 'enabled'> & { enabled?: boolean }) {
    try {
      loading.value = true
      error.value = null
      await pluginApi.addSource(source)
      await fetchSources()
      await refreshSources()
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
      await pluginApi.removeSource(id)
      sources.value = sources.value.filter((s) => s.id !== id)
      remotePlugins.value = remotePlugins.value.filter((p) => p.source_id !== id)
      return true
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to remove source'
      return false
    } finally {
      loading.value = false
    }
  }

  async function refreshSources() {
    try {
      refreshing.value = true
      error.value = null
      await pluginApi.refresh()
      const response = await pluginApi.browse()
      remotePlugins.value = response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to refresh plugin store'
    } finally {
      refreshing.value = false
    }
  }

  async function installPlugin(id: string) {
    try {
      loading.value = true
      error.value = null
      await pluginApi.install(id)
      // Mark as installed in remote list
      const remote = remotePlugins.value.find((p) => p.id === id)
      if (remote) {
        remote.installed = true
      }
      // Refresh local plugins
      await fetchPlugins()
      return true
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to install plugin'
      return false
    } finally {
      loading.value = false
    }
  }

  async function uninstallPlugin(id: string) {
    try {
      loading.value = true
      error.value = null
      await pluginApi.uninstall(id)
      // Mark as not installed in remote list
      const remote = remotePlugins.value.find((p) => p.id === id)
      if (remote) {
        remote.installed = false
      }
      // Remove from local plugins
      plugins.value = plugins.value.filter((p) => p.id !== id)
      return true
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to uninstall plugin'
      return false
    } finally {
      loading.value = false
    }
  }

  return {
    // State
    plugins,
    selectedPluginId,
    pluginLogs,
    loading,
    error,
    sources,
    remotePlugins,
    refreshing,

    // Computed
    selectedPlugin,
    enabledPlugins,
    disabledPlugins,
    pluginsByType,
    availablePlugins,

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
    fetchSources,
    addSource,
    removeSource,
    refreshSources,
    installPlugin,
    uninstallPlugin,
  }
})
