import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import SettingsView from '@/views/SettingsView.vue'
import { useSettingsStore } from '@/stores/settings'
import { i18n } from '@/i18n'
import { settingsApi } from '@/api/settings'
import { providerPoolApi } from '@/api/providerPool'
import { claudeCodeApi } from '@/api/claudecode'
import { backupApi } from '@/api/index'
import { proxyCacheApi } from '@/api/proxyCache'
import { serviceApi } from '@/api/service'

let routeTab: 'general' | 'proxy' | 'memory' | 'llm' = 'proxy'
const routerReplace = vi.fn()
const { tauriState } = vi.hoisted(() => ({
  tauriState: {
    isTauri: false,
    platform: 'unknown',
    setCloseBehavior: vi.fn(),
  },
}))

vi.mock('vue-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-router')>()
  return {
    ...actual,
    useRoute: () => ({ query: { tab: routeTab } }),
    useRouter: () => ({ replace: routerReplace }),
  }
})

vi.mock('@/components/ClaudeCodeSettings.vue', () => ({
  default: { name: 'ClaudeCodeSettings', template: '<div />' },
}))

vi.mock('@/components/ProviderPoolSection.vue', () => ({
  default: { name: 'ProviderPoolSection', template: '<div />' },
}))

vi.mock('@/components/UserDataExport.vue', () => ({
  default: { name: 'UserDataExport', template: '<div />' },
}))

vi.mock('@/components/settings/NetworkSettings.vue', () => ({
  default: { name: 'NetworkSettings', template: '<div />' },
}))

vi.mock('@/components/settings/SpeechSettings.vue', () => ({
  default: { name: 'SpeechSettings', template: '<div />' },
}))

vi.mock('@/components/settings/WorkspaceSettings.vue', () => ({
  default: { name: 'WorkspaceSettings', template: '<div />' },
}))

vi.mock('@/components/settings/UpdateSettings.vue', () => ({
  default: { name: 'UpdateSettings', template: '<div />' },
}))

vi.mock('@/components/settings/ApiProxySettings.vue', () => ({
  default: { name: 'ApiProxySettings', template: '<div />' },
}))

vi.mock('@/components/MemoryManager.vue', () => ({
  default: { name: 'MemoryManager', template: '<div />' },
}))

vi.mock('@/components/BackupManager.vue', () => ({
  default: { name: 'BackupManager', template: '<div />' },
}))

vi.mock('@/api/chat', () => ({
  toolApi: {
    list: vi.fn(),
  },
}))

vi.mock('@/api/providerPool', () => ({
  providerPoolApi: {
    listProviders: vi.fn(),
    fetchProviderModels: vi.fn(),
  },
}))

vi.mock('@/api/claudecode', () => ({
  claudeCodeApi: {
    getConfig: vi.fn(),
  },
}))

vi.mock('@/api/settings', () => ({
  settingsApi: {
    get: vi.fn(),
    update: vi.fn(),
    patch: vi.fn(),
    getSkillRerankerModelStatus: vi.fn(),
    downloadSkillRerankerModel: vi.fn(),
    cancelSkillRerankerModelDownload: vi.fn(),
    getSmallModelStatus: vi.fn(),
    downloadSmallModel: vi.fn(),
    cancelSmallModelDownload: vi.fn(),
    getSmallModelStats: vi.fn(),
    resetSmallModelStats: vi.fn(),
  },
}))

vi.mock('@/api/index', () => ({
  backupApi: {
    list: vi.fn(),
    create: vi.fn(),
    restore: vi.fn(),
    delete: vi.fn(),
  },
}))

vi.mock('@/api/proxyCache', () => ({
  proxyCacheApi: {
    getPrunerConfig: vi.fn(),
    getPrunerStats: vi.fn(),
    updatePrunerConfig: vi.fn(),
  },
}))

vi.mock('@/api/service', () => ({
  serviceApi: {
    getInfo: vi.fn(),
    install: vi.fn(),
    enable: vi.fn(),
    disable: vi.fn(),
    uninstall: vi.fn(),
  },
}))

vi.mock('@/composables/useTauri', async () => {
  const { ref } = await import('vue')
  return {
    useTauri: () => ({
      isTauri: ref(tauriState.isTauri),
      platform: ref(tauriState.platform),
      setCloseBehavior: tauriState.setCloseBehavior,
    }),
  }
})

const localStorageMock = (() => {
  let store: Record<string, string> = {}
  return {
    getItem: (key: string) => (key in store ? store[key] : null),
    setItem: (key: string, value: string) => {
      store[key] = String(value)
    },
    removeItem: (key: string) => {
      delete store[key]
    },
    clear: () => {
      store = {}
    },
  }
})()

vi.stubGlobal('localStorage', localStorageMock)

function primeApiMocks() {
  vi.mocked(providerPoolApi.listProviders).mockResolvedValue({
    data: { providers: [] },
  } as never)
  vi.mocked(claudeCodeApi.getConfig).mockResolvedValue({
    data: { enabled: false },
  } as never)
  vi.mocked(settingsApi.get).mockResolvedValue({
    data: {
      small_model_enabled: true,
      small_model_runtime: 'llama.cpp',
      small_model_id: 'qwen3.5-0.8b-gguf-q4km',
      small_model_summary_enabled: true,
      small_model_context_compress_enabled: true,
      small_model_doc_extract_enabled: true,
      context_compression_mode: 'auto',
      small_model_route_image_qa_enabled: true,
      small_model_route_short_qa_enabled: true,
      no_llm_degrade_mode: 'deepresearch',
      small_model_unavailable_policy: 'ir_first',
      small_model_context_prune_enabled: true,
      small_model_media_intent_enabled: true,
      offline_ir_fallback_enabled: true,
      feature_intent_ir_enabled: true,
    },
  } as never)
  vi.mocked(settingsApi.patch).mockImplementation(
    async (payload: unknown) => ({ data: payload }) as never
  )
  vi.mocked(settingsApi.getSkillRerankerModelStatus).mockResolvedValue({
    data: { ready: false, downloading: false, state: 'idle' },
  } as never)
  vi.mocked(settingsApi.getSmallModelStatus).mockResolvedValue({
    data: {
      ready: false,
      downloading: false,
      model_id: 'qwen3.5-0.8b-gguf-q4km',
      runtime: 'llama.cpp',
      model_path: '/tmp/model.gguf',
      state: 'idle',
    },
  } as never)
  vi.mocked(settingsApi.getSmallModelStats).mockResolvedValue({
    data: {
      short_qa_route_attempts: 12,
      short_qa_route_success: 10,
      image_qa_route_attempts: 7,
      image_qa_route_success: 6,
      summary_attempts: 4,
      summary_success: 3,
      context_compress_attempts: 5,
      context_compress_success: 4,
      context_compress_latency_ms: 11,
      doc_extract_attempts: 2,
      doc_extract_success: 1,
      no_provider_deepresearch_total: 2,
      ir_takeover_total: 3,
      fallback_reasons: { timeout: 1 },
    },
  } as never)
  vi.mocked(settingsApi.resetSmallModelStats).mockResolvedValue({
    data: { success: true },
  } as never)
  vi.mocked(proxyCacheApi.getPrunerConfig).mockResolvedValue({
    data: {
      enabled: false,
      backend: 'local',
      threshold: 0.5,
      min_lines: 80,
      timeout_ms: 5000,
    },
  } as never)
  vi.mocked(proxyCacheApi.getPrunerStats).mockResolvedValue({
    data: {
      enabled: false,
      stats: {
        total_requests: 10,
        pruned_requests: 3,
        passthrough_requests: 7,
        total_tokens_before: 1000,
        total_tokens_after: 700,
        tokens_saved: 300,
        avg_compression_rate: 0.3,
        avg_latency_ms: 12,
      },
    },
  } as never)
  vi.mocked(proxyCacheApi.updatePrunerConfig).mockImplementation(
    async ({ enabled }: { enabled: boolean }) =>
      ({
        data: {
          success: true,
          config: {
            enabled,
            backend: 'local',
            threshold: 0.5,
            min_lines: 80,
            timeout_ms: 5000,
          },
        },
      }) as never
  )
  vi.mocked(backupApi.list).mockResolvedValue({ data: [] } as never)
  vi.mocked(serviceApi.getInfo).mockResolvedValue({
    data: { installed: false, enabled: false },
  } as never)
}

describe('SettingsView small-model controls', () => {
  beforeEach(() => {
    localStorageMock.clear()
    routerReplace.mockReset()
    vi.clearAllMocks()
    tauriState.isTauri = false
    tauriState.platform = 'unknown'
    primeApiMocks()
  })

  it('triggers IR/capability toggles, download, and stats reset on proxy tab', async () => {
    routeTab = 'proxy'
    const pinia = createPinia()
    setActivePinia(pinia)
    const store = useSettingsStore()

    const contextPruneSpy = vi.spyOn(store, 'setSmallModelContextPruneEnabled').mockResolvedValue()
    const mediaIntentSpy = vi.spyOn(store, 'setSmallModelMediaIntentEnabled').mockResolvedValue()
    const offlineIRFallbackSpy = vi.spyOn(store, 'setOfflineIRFallbackEnabled').mockResolvedValue()
    const featureIntentIRSpy = vi.spyOn(store, 'setFeatureIntentIREnabled').mockResolvedValue()
    const irMasterSpy = vi.spyOn(store, 'setSmallModelIRFeaturesEnabled').mockResolvedValue()
    const summarySpy = vi.spyOn(store, 'setSmallModelSummaryEnabled').mockResolvedValue()
    const contextCompressSpy = vi
      .spyOn(store, 'setSmallModelContextCompressEnabled')
      .mockResolvedValue()
    const compressionModeSpy = vi.spyOn(store, 'setContextCompressionMode').mockResolvedValue()
    const docExtractSpy = vi.spyOn(store, 'setSmallModelDocExtractEnabled').mockResolvedValue()
    const imageQASpy = vi.spyOn(store, 'setSmallModelRouteImageQAEnabled').mockResolvedValue()
    const shortQASpy = vi.spyOn(store, 'setSmallModelRouteShortQAEnabled').mockResolvedValue()
    const downloadSpy = vi.spyOn(store, 'startSmallModelDownload').mockResolvedValue({} as never)
    const resetSpy = vi.spyOn(store, 'resetSmallModelStats').mockResolvedValue({} as never)

    const wrapper = mount(SettingsView, {
      shallow: true,
      global: {
        plugins: [pinia, i18n],
      },
    })
    await flushPromises()

    expect(wrapper.find('[data-testid="small-model-context-prune-switch"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="proxy-pruner-switch"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="smart-tool-selection-switch"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="small-model-ir-master-switch"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="small-model-rerank-switch"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="small-model-advanced-toggle"]').exists()).toBe(false)
    expect(
      wrapper
        .get('[data-testid="small-model-ir-section-header"]')
        .find('[data-testid="small-model-ir-master-switch"]')
        .exists()
    ).toBe(true)
    expect(wrapper.get('[data-testid="small-model-ir-grid"]').classes()).toContain('sm:grid-cols-2')
    expect(wrapper.get('[data-testid="small-model-resource-status-row"]').text()).toContain(
      'Resource Footprint'
    )
    expect(
      wrapper
        .get('[data-testid="small-model-resource-status-row"]')
        .find('[data-testid="small-model-storage-usage"]')
        .exists()
    ).toBe(true)
    expect(
      wrapper
        .get('[data-testid="small-model-resource-status-row"]')
        .find('[data-testid="small-model-runtime-usage"]')
        .exists()
    ).toBe(true)
    expect(
      wrapper
        .get('[data-testid="small-model-resource-status-row"]')
        .find('[data-testid="small-model-status-badge"]')
        .exists()
    ).toBe(true)
    expect(wrapper.get('[data-testid="small-model-status-badge"]').text()).toContain(
      i18n.global.t('settings.smallModel.notReady', 'Not Ready')
    )

    await wrapper.get('[data-testid="small-model-ir-master-switch"]').trigger('click')
    await flushPromises()
    expect(irMasterSpy).toHaveBeenCalledWith(true)
    expect(proxyCacheApi.updatePrunerConfig).toHaveBeenNthCalledWith(1, { enabled: true })

    await wrapper.get('[data-testid="small-model-context-prune-switch"]').trigger('click')
    await flushPromises()
    expect(contextPruneSpy).toHaveBeenCalledWith(false)

    expect(wrapper.get('[data-testid="proxy-pruner-switch"]').attributes('aria-checked')).toBe(
      'true'
    )
    await wrapper.get('[data-testid="proxy-pruner-switch"]').trigger('click')
    await flushPromises()
    expect(proxyCacheApi.updatePrunerConfig).toHaveBeenNthCalledWith(2, { enabled: false })
    expect(wrapper.get('[data-testid="proxy-pruner-switch"]').attributes('aria-checked')).toBe(
      'false'
    )

    await wrapper.get('[data-testid="small-model-media-intent-switch"]').trigger('click')
    await flushPromises()
    expect(mediaIntentSpy).toHaveBeenCalledWith(false)

    await wrapper.get('[data-testid="offline-ir-fallback-switch"]').trigger('click')
    await flushPromises()
    expect(offlineIRFallbackSpy).toHaveBeenCalledWith(false)

    await wrapper.get('[data-testid="feature-intent-ir-switch"]').trigger('click')
    await flushPromises()
    expect(featureIntentIRSpy).toHaveBeenCalledWith(false)

    await wrapper.get('[data-testid="small-model-summary-switch"]').trigger('click')
    await flushPromises()
    expect(summarySpy).toHaveBeenCalledWith(false)

    await wrapper.get('[data-testid="small-model-context-compress-switch"]').trigger('click')
    await flushPromises()
    expect(contextCompressSpy).toHaveBeenCalledWith(false)

    await wrapper.get('[data-testid="context-compression-mode-select"]').setValue('offline')
    await flushPromises()
    expect(compressionModeSpy).toHaveBeenCalledWith('offline')

    await wrapper.get('[data-testid="small-model-doc-extract-switch"]').trigger('click')
    await flushPromises()
    expect(docExtractSpy).toHaveBeenCalledWith(false)

    await wrapper.get('[data-testid="small-model-image-qa-switch"]').trigger('click')
    await flushPromises()
    expect(imageQASpy).toHaveBeenCalledWith(false)

    await wrapper.get('[data-testid="small-model-short-qa-switch"]').trigger('click')
    await flushPromises()
    expect(shortQASpy).toHaveBeenCalledWith(false)

    await wrapper.get('[data-testid="small-model-download"]').trigger('click')
    await flushPromises()
    expect(downloadSpy).toHaveBeenCalledTimes(1)

    await wrapper.get('[data-testid="small-model-stats-toggle"]').trigger('click')
    await flushPromises()

    await wrapper.get('[data-testid="small-model-stats-reset"]').trigger('click')
    await flushPromises()
    expect(resetSpy).toHaveBeenCalledTimes(1)

    wrapper.unmount()
  })

  it('removes duplicate card titles from optimization section headers', async () => {
    routeTab = 'proxy'
    const pinia = createPinia()
    setActivePinia(pinia)

    const wrapper = mount(SettingsView, {
      shallow: true,
      global: {
        plugins: [pinia, i18n],
      },
    })
    await flushPromises()

    expect(wrapper.get('[data-testid="small-model-ir-section-header"]').find('h3').exists()).toBe(
      false
    )
    expect(wrapper.get('[data-testid="small-model-main-section-header"]').find('h3').exists()).toBe(
      false
    )

    wrapper.unmount()
  })

  it('defaults auto reflection to enabled on memory tab when unset', async () => {
    routeTab = 'memory'
    const pinia = createPinia()
    setActivePinia(pinia)
    const store = useSettingsStore()

    const wrapper = mount(SettingsView, {
      shallow: true,
      global: {
        plugins: [pinia, i18n],
      },
    })
    await flushPromises()

    expect(store.agentAutoReflect).toBe(true)
    expect(wrapper.find('[data-testid="agent-auto-reflect-switch"]').exists()).toBe(false)

    wrapper.unmount()
  })

  it('opens the llm tab directly even when no providers are configured', async () => {
    routeTab = 'proxy'
    const pinia = createPinia()
    setActivePinia(pinia)

    const wrapper = mount(SettingsView, {
      shallow: true,
      global: {
        plugins: [pinia, i18n],
      },
    })
    await flushPromises()

    const llmTabButton = wrapper
      .findAll('button')
      .find((button) => button.text().includes(i18n.global.t('settings.tab.llm')))
    expect(llmTabButton).toBeTruthy()
    await llmTabButton!.trigger('click')
    await flushPromises()

    expect(wrapper.findComponent({ name: 'ProviderPoolSection' }).exists()).toBe(true)
    wrapper.unmount()
  })

  it('honors an llm tab query without redirecting back to general', async () => {
    routeTab = 'llm'
    const pinia = createPinia()
    setActivePinia(pinia)

    const wrapper = mount(SettingsView, {
      shallow: true,
      global: {
        plugins: [pinia, i18n],
      },
    })
    await flushPromises()

    expect(wrapper.findComponent({ name: 'ProviderPoolSection' }).exists()).toBe(true)
    expect(routerReplace).toHaveBeenCalledWith({ query: { tab: 'llm' } })

    wrapper.unmount()
  })

  it('keeps close behavior icon-only while using menu bar wording on macOS desktop', async () => {
    routeTab = 'general'
    tauriState.isTauri = true
    tauriState.platform = 'macos'
    const pinia = createPinia()
    setActivePinia(pinia)

    const wrapper = mount(SettingsView, {
      shallow: true,
      global: {
        plugins: [pinia, i18n],
      },
    })
    await flushPromises()

    const buttons = wrapper.findAll('.settings-pill-button')

    expect(wrapper.text()).not.toContain('Minimize to Menu Bar')
    expect(wrapper.text()).not.toContain('Minimize to Tray')
    expect(wrapper.findAll('.settings-pill-button__icon').length).toBe(2)
    expect(buttons).toHaveLength(2)
    expect(buttons[0]?.attributes('aria-label')).toBe(i18n.global.t('settings.closeBehaviorQuit'))
    expect(buttons[1]?.attributes('aria-label')).toBe('Minimize to Menu Bar')

    wrapper.unmount()
  })
})
