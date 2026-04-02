import { beforeEach, describe, expect, it, vi } from 'vitest'

const state = vi.hoisted(() => {
  let apiProxyLoads = 0
  let speechSettingsLoads = 0

  return {
    getApiProxyLoads() {
      return apiProxyLoads
    },
    getSpeechSettingsLoads() {
      return speechSettingsLoads
    },
    trackApiProxyLoad() {
      apiProxyLoads += 1
    },
    trackSpeechSettingsLoad() {
      speechSettingsLoads += 1
    },
    reset() {
      apiProxyLoads = 0
      speechSettingsLoads = 0
    },
  }
})

vi.mock('@/components/settings/ApiProxySettings.vue', () => {
  state.trackApiProxyLoad()
  return {
    default: { name: 'ApiProxySettings', template: '<div class="api-proxy-stub" />' },
  }
})

vi.mock('@/components/settings/SpeechSettings.vue', () => {
  state.trackSpeechSettingsLoad()
  return {
    default: { name: 'SpeechSettings', template: '<div class="speech-settings-stub" />' },
  }
})

vi.mock('@/components/settings/NetworkSettings.vue', () => ({
  default: { name: 'NetworkSettings', template: '<div class="network-settings-stub" />' },
}))

vi.mock('@/components/settings/UpdateSettings.vue', () => ({
  default: { name: 'UpdateSettings', template: '<div class="update-settings-stub" />' },
}))

vi.mock('@/components/ProviderPoolSection.vue', () => ({
  default: { name: 'ProviderPoolSection', template: '<div class="provider-pool-stub" />' },
}))

vi.mock('@/components/UserDataExport.vue', () => ({
  default: { name: 'UserDataExport', template: '<div class="user-data-export-stub" />' },
}))

vi.mock('@/components/settings/ExternalAgentsSection.vue', () => ({
  default: { name: 'ExternalAgentsSection', template: '<div class="external-agents-stub" />' },
}))

vi.mock('@/components/MemoryManager.vue', () => ({
  default: { name: 'MemoryManager', template: '<div class="memory-manager-stub" />' },
}))

vi.mock('@/components/BackupManager.vue', () => ({
  default: { name: 'BackupManager', template: '<div class="backup-manager-stub" />' },
}))

async function loadSettingsView() {
  return (await import('@/views/SettingsView.vue')).default
}

describe('SettingsView async tabs', () => {
  beforeEach(() => {
    vi.resetModules()
    state.reset()
  })

  it('does not eagerly load proxy and speech tab sections when the settings route module is imported', async () => {
    await loadSettingsView()

    expect(state.getApiProxyLoads()).toBe(0)
    expect(state.getSpeechSettingsLoads()).toBe(0)
  })
})
