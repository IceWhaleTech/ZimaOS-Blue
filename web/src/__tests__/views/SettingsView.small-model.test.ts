import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import SettingsView from '@/views/SettingsView.vue'
import { useSettingsStore } from '@/stores/settings'
import { i18n, setLocale } from '@/i18n'
import { settingsApi } from '@/api/settings'
import { providerPoolApi } from '@/api/providerPool'
import { backupApi } from '@/api/index'
import { proxyCacheApi } from '@/api/proxyCache'
import { serviceApi } from '@/api/service'

let routeTab: 'general' | 'proxy' | 'memory' | 'llm' | 'userdata' = 'proxy'
const routerReplace = vi.fn()
const { tauriState } = vi.hoisted(() => ({
  tauriState: {
    isTauri: false,
    platform: 'unknown',
    setCloseBehavior: vi.fn(),
    restartServerRuntime: vi.fn(),
  },
}))

vi.mock('vue-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-router')>()
  return {
    ...actual,
    useRoute: () => ({
      path: '/settings',
      fullPath: `/settings?tab=${routeTab}`,
      query: { tab: routeTab },
    }),
    useRouter: () => ({ replace: routerReplace }),
  }
})

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

vi.mock('@/components/settings/ExternalAgentsSection.vue', () => ({
  default: { name: 'ExternalAgentsSection', template: '<div />' },
}))

vi.mock('@/components/MemoryManager.vue', () => ({
  default: { name: 'MemoryManager', template: '<div />' },
}))

vi.mock('@/components/BackupManager.vue', () => ({
  default: {
    name: 'BackupManager',
    template: '<button data-testid="mock-backup-restore" @click="$emit(\'restore\', \'backup-1\')" />',
  },
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
    getAgentcoreRunnerStatus: vi.fn(),
    getAgentcoreRunnerLastRun: vi.fn(),
    getAgentcoreRunnerTags: vi.fn(),
    prepareAgentcoreRunner: vi.fn(),
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
      restartServerRuntime: tauriState.restartServerRuntime,
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

async function settleSettingsAsyncTabComponents() {
  await flushPromises()
  await Promise.resolve()
  await flushPromises()
}

function primeApiMocks() {
  vi.mocked(providerPoolApi.listProviders).mockResolvedValue({
    data: { providers: [] },
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
  vi.mocked(settingsApi.getAgentcoreRunnerStatus).mockResolvedValue({
    data: {
      enabled: true,
      repo_url: 'https://github.com/example/runner',
      resolved_ref: 'main',
      resolved_commit: 'abc123',
      required_go_version: '1.24.0',
      installed_go_version: '1.24.0',
      toolchain_ready: true,
      binary_ready: true,
      binary_path: '/tmp/agentcore-runner',
      binary_sha256: 'deadbeef',
      last_prepare_at: '2026-04-03T12:00:00Z',
      last_prepare_state: 'ready',
      last_error: '',
      last_optimization_run_id: 'opt-123',
      last_optimization_at: '2026-04-03T12:05:00Z',
      last_optimization_state: 'completed',
      last_optimization_summary: 'agentcore runner summary',
    },
  } as never)
  vi.mocked(settingsApi.getAgentcoreRunnerLastRun).mockResolvedValue({
    data: {
      id: 'opt-123',
      reason: 'execution_gate_failed',
      candidate_id: 'candidate-123',
      runner_protocol: 'acp',
      runner_stop_reason: 'completed',
      runner_duration_ms: 128,
      runner_response_text: 'agentcore-runner received optimization evidence',
      runner_transcript: [
        {
          direction: 'out',
          method: 'session/prompt',
          text: 'Optimization trigger received.',
        },
        {
          direction: 'in',
          method: 'session/update',
          text: 'agentcore-runner received optimization evidence',
        },
      ],
    },
  } as never)
  vi.mocked(settingsApi.getAgentcoreRunnerTags).mockResolvedValue({
    data: {
      repo_url: 'https://github.com/IceWhaleTech/ZimaOS-Blue',
      default_ref: 'main',
      tags: ['release/v1', 'v0.10.37', 'v0.10.36'],
    },
  } as never)
  vi.mocked(settingsApi.prepareAgentcoreRunner).mockResolvedValue({
    data: {
      enabled: true,
      repo_url: 'https://github.com/example/runner',
      resolved_ref: 'main',
      resolved_commit: 'def456',
      required_go_version: '1.24.0',
      installed_go_version: '1.24.0',
      toolchain_ready: true,
      binary_ready: true,
      binary_path: '/tmp/agentcore-runner',
      binary_sha256: 'beadfeed',
      last_prepare_at: '2026-04-03T12:30:00Z',
      last_prepare_state: 'ready',
      last_error: '',
      last_optimization_run_id: 'opt-456',
      last_optimization_at: '2026-04-03T12:31:45Z',
      last_optimization_state: 'completed',
      last_optimization_summary: 'agentcore runner summary 2',
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
  beforeEach(async () => {
    localStorageMock.clear()
    routerReplace.mockReset()
    vi.resetAllMocks()
    tauriState.isTauri = false
    tauriState.platform = 'unknown'
    tauriState.restartServerRuntime.mockReset()
    tauriState.restartServerRuntime.mockResolvedValue(true)
    await setLocale('en-US')
    primeApiMocks()
    vi.stubGlobal('confirm', vi.fn(() => true))
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
      global: {
        plugins: [pinia, i18n],
      },
    })
    await settleSettingsAsyncTabComponents()

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

  it('renders localized fallback reasons in the small-model stats panel', async () => {
    routeTab = 'proxy'
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
        fallback_reasons: {
          deepresearch_unavailable: 2,
          auto_rollback_doc_extract_fallback_rate: 1,
        },
      },
    } as never)

    const pinia = createPinia()
    setActivePinia(pinia)

    const wrapper = mount(SettingsView, {
      global: {
        plugins: [pinia, i18n],
      },
    })
    await flushPromises()

    await wrapper.get('[data-testid="small-model-stats-toggle"]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain(
      i18n.global.t('settings.smallModel.fallbackReasonLabels.deepresearch_unavailable')
    )
    expect(wrapper.text()).toContain(
      i18n.global.t('settings.smallModel.fallbackReasonLabels.auto_rollback_doc_extract_fallback_rate')
    )
    expect(wrapper.text()).not.toContain('deepresearch_unavailable')
    expect(wrapper.text()).not.toContain('auto_rollback_doc_extract_fallback_rate')

    wrapper.unmount()
  })

  it('renders the agentcore runner card, refreshes status, and prepares the runner', async () => {
    routeTab = 'proxy'
    vi.mocked(settingsApi.getAgentcoreRunnerStatus)
      .mockResolvedValueOnce({
        data: {
          enabled: true,
          repo_url: 'https://github.com/example/runner',
          resolved_ref: 'main',
          resolved_commit: 'abc123',
          required_go_version: '1.24.0',
          installed_go_version: '1.24.0',
          toolchain_ready: true,
          binary_ready: false,
          binary_path: '',
          binary_sha256: '',
          last_prepare_at: '2026-04-03T12:00:00Z',
          last_prepare_state: 'idle',
          last_error: '',
          last_optimization_run_id: 'opt-123',
          last_optimization_at: '2026-04-03T12:05:00Z',
          last_optimization_state: 'failed',
          last_optimization_summary: 'runner binary checksum mismatch',
        },
      } as never)
      .mockResolvedValueOnce({
        data: {
          enabled: true,
          repo_url: 'https://github.com/example/runner',
          resolved_ref: 'release/v1',
          resolved_commit: 'def456',
          required_go_version: '1.24.0',
          installed_go_version: '1.24.0',
          toolchain_ready: true,
          binary_ready: true,
          binary_path: '/tmp/agentcore-runner',
          binary_sha256: 'beadfeed',
          last_prepare_at: '2026-04-03T12:30:00Z',
          last_prepare_state: 'ready',
          last_error: '',
          last_optimization_run_id: 'opt-456',
          last_optimization_at: '2026-04-03T12:31:45Z',
          last_optimization_state: 'completed',
          last_optimization_summary: 'execution gate improved after managed runner check',
        },
      } as never)
      .mockResolvedValueOnce({
        data: {
          enabled: true,
          repo_url: 'https://github.com/example/runner',
          resolved_ref: 'release/v1',
          resolved_commit: 'def456',
          required_go_version: '1.24.0',
          installed_go_version: '1.24.0',
          toolchain_ready: true,
          binary_ready: true,
          binary_path: '/tmp/agentcore-runner',
          binary_sha256: 'beadfeed',
          last_prepare_at: '2026-04-03T12:30:00Z',
          last_prepare_state: 'ready',
          last_error: '',
          last_optimization_run_id: 'opt-456',
          last_optimization_at: '2026-04-03T12:31:45Z',
          last_optimization_state: 'completed',
          last_optimization_summary: 'execution gate improved after managed runner check',
        },
      } as never)
    vi.mocked(settingsApi.getAgentcoreRunnerLastRun)
      .mockResolvedValueOnce({
        data: {
          id: 'opt-123',
          reason: 'selector_gate_failed',
          candidate_id: 'candidate-123',
          runner_protocol: 'acp',
          runner_error: 'runner binary checksum mismatch',
          runner_transcript: [],
        },
      } as never)
      .mockResolvedValueOnce({
        data: {
          id: 'opt-456',
          reason: 'execution_gate_failed',
          candidate_id: 'candidate-456',
          eval_run_id: 'eval-456',
          runner_protocol: 'acp',
          runner_stop_reason: 'completed',
          runner_duration_ms: 612,
          optimization_surface: 'runner_code',
          runner_response_text: 'execution gate improved after managed runner check',
          runner_transcript: [
            {
              direction: 'out',
              method: 'session/prompt',
              text: 'Optimization trigger received.',
            },
            {
              direction: 'out',
              method: 'session/context',
              text: 'Constraint diff prepared.',
            },
            {
              direction: 'out',
              method: 'session/context',
              text: 'Build recipe prepared.',
            },
            {
              direction: 'in',
              method: 'session/update',
              text: 'execution gate improved after managed runner check',
            },
          ],
        },
      } as never)
      .mockResolvedValueOnce({
        data: {
          id: 'opt-456',
          reason: 'execution_gate_failed',
          candidate_id: 'candidate-456',
          eval_run_id: 'eval-456',
          runner_protocol: 'acp',
          runner_stop_reason: 'completed',
          runner_duration_ms: 612,
          optimization_surface: 'runner_code',
          runner_response_text: 'execution gate improved after managed runner check',
          runner_transcript: [
            {
              direction: 'out',
              method: 'session/prompt',
              text: 'Optimization trigger received.',
            },
            {
              direction: 'out',
              method: 'session/context',
              text: 'Constraint diff prepared.',
            },
            {
              direction: 'out',
              method: 'session/context',
              text: 'Build recipe prepared.',
            },
            {
              direction: 'in',
              method: 'session/update',
              text: 'execution gate improved after managed runner check',
            },
          ],
        },
      } as never)

    vi.mocked(settingsApi.prepareAgentcoreRunner).mockResolvedValueOnce({
      data: {
        enabled: true,
        repo_url: 'https://github.com/example/runner',
        resolved_ref: 'release/v1',
        resolved_commit: 'def456',
        required_go_version: '1.24.0',
        installed_go_version: '1.24.0',
        toolchain_ready: true,
        binary_ready: true,
        binary_path: '/tmp/agentcore-runner',
        binary_sha256: 'beadfeed',
        last_prepare_at: '2026-04-03T12:30:00Z',
        last_prepare_state: 'ready',
        last_error: '',
        last_optimization_run_id: 'opt-456',
        last_optimization_at: '2026-04-03T12:31:45Z',
        last_optimization_state: 'completed',
        last_optimization_summary: 'execution gate improved after managed runner check',
      },
    } as never)

    const pinia = createPinia()
    setActivePinia(pinia)

    const wrapper = mount(SettingsView, {
      global: {
        plugins: [pinia, i18n],
      },
    })
    await settleSettingsAsyncTabComponents()

    expect(settingsApi.getAgentcoreRunnerStatus).toHaveBeenCalledTimes(1)
    const runnerCard = wrapper.get('[data-testid="agentcore-runner-card"]')
    expect(runnerCard.text()).toContain('Agentcore Runner')
    expect(
      Array.from(runnerCard.get('.space-y-1\\.5').element.children).map((node) => node.tagName)
    ).toEqual(['SPAN', 'H2', 'P'])
    expect(runnerCard.get('.settings-module__eyebrow').text()).toContain('Harness')
    expect(
      (wrapper.get('[data-testid="agentcore-runner-repo-input"]').element as HTMLInputElement).value
    ).toBe('https://github.com/IceWhaleTech/ZimaOS-Blue')
    expect(wrapper.find('[data-testid="agentcore-runner-status-content"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="agentcore-runner-status-toggle"]').text()).toContain('Expand')
    expect(settingsApi.getAgentcoreRunnerTags).toHaveBeenCalledWith(
      'https://github.com/IceWhaleTech/ZimaOS-Blue'
    )
    const refSelect = wrapper.get('[data-testid="agentcore-runner-ref-input"]')
    expect((refSelect.element as HTMLSelectElement).value).toBe('main')
    expect(refSelect.text()).toContain('main')
    expect(refSelect.text()).toContain('v0.10.37')

    await wrapper.get('[data-testid="agentcore-runner-status-toggle"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="agentcore-runner-status-content"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="agentcore-runner-status-toggle"]').text()).toContain('Collapse')
    expect(wrapper.get('[data-testid="agentcore-runner-resolved-commit"]').text()).toContain('abc123')
    expect(wrapper.get('[data-testid="agentcore-runner-last-optimization-state"]').text()).toContain(
      'failed'
    )
    expect(wrapper.get('[data-testid="agentcore-runner-last-optimization-summary"]').text()).toContain(
      'checksum mismatch'
    )
    expect(wrapper.get('[data-testid="agentcore-runner-last-run-meta"]').text()).toContain(
      'selector_gate_failed'
    )
    expect(wrapper.get('[data-testid="agentcore-runner-last-run-meta"]').text()).toContain(
      'candidate-123'
    )
    expect(wrapper.get('[data-testid="agentcore-runner-last-run-error"]').text()).toContain(
      'runner binary checksum mismatch'
    )

    await wrapper.get('[data-testid="agentcore-runner-repo-input"]').setValue('owner/repo-next')
    await wrapper.get('[data-testid="agentcore-runner-ref-input"]').setValue('release/v1')
    await flushPromises()
    await wrapper.get('[data-testid="agentcore-runner-prepare"]').trigger('click')
    await flushPromises()

    expect(settingsApi.patch).toHaveBeenCalledWith({
      experimental_agentcore_runner_repo_url: 'owner/repo-next',
      experimental_agentcore_runner_ref: 'release/v1',
    })
    expect(settingsApi.prepareAgentcoreRunner).toHaveBeenCalledTimes(1)
    expect(settingsApi.getAgentcoreRunnerStatus).toHaveBeenCalledTimes(3)
    expect(settingsApi.getAgentcoreRunnerLastRun).toHaveBeenCalledTimes(3)
    expect(wrapper.get('[data-testid="agentcore-runner-resolved-commit"]').text()).toContain('def456')
    expect(wrapper.get('[data-testid="agentcore-runner-binary-path"]').text()).toContain(
      '/tmp/agentcore-runner'
    )
    expect(wrapper.get('[data-testid="agentcore-runner-last-optimization-state"]').text()).toContain(
      'completed'
    )
    expect(wrapper.get('[data-testid="agentcore-runner-last-optimization-time"]').text()).toContain(
      '2026'
    )
    expect(wrapper.get('[data-testid="agentcore-runner-last-optimization-summary"]').text()).toContain(
      'execution gate improved after managed runner check'
    )
    expect(wrapper.get('[data-testid="agentcore-runner-last-run-meta"]').text()).toContain('acp')
    expect(wrapper.get('[data-testid="agentcore-runner-last-run-meta"]').text()).toContain(
      'completed'
    )
    expect(wrapper.get('[data-testid="agentcore-runner-last-run-meta"]').text()).toContain(
      'runner_code'
    )
    expect(wrapper.get('[data-testid="agentcore-runner-last-run-meta"]').text()).toContain(
      'execution_gate_failed'
    )
    expect(wrapper.get('[data-testid="agentcore-runner-last-run-meta"]').text()).toContain(
      'candidate-456'
    )
    expect(wrapper.get('[data-testid="agentcore-runner-last-run-meta"]').text()).toContain('612ms')
    expect(wrapper.get('[data-testid="agentcore-runner-last-run-response"]').text()).toContain(
      'execution gate improved after managed runner check'
    )
    expect(wrapper.get('[data-testid="agentcore-runner-last-run-transcript"]').text()).toContain(
      'session/prompt'
    )
    expect(wrapper.get('[data-testid="agentcore-runner-last-run-transcript"]').text()).toContain(
      'Optimization trigger received.'
    )
    expect(wrapper.get('[data-testid="agentcore-runner-last-run-transcript"]').text()).toContain(
      'Constraint diff prepared.'
    )
    expect(
      wrapper.get('[data-testid="agentcore-runner-last-run-transcript"]').text()
    ).not.toContain('Build recipe prepared.')
    expect(wrapper.get('[data-testid="agentcore-runner-transcript-toggle"]').text()).toContain(
      'Expand'
    )
    expect(
      wrapper.get('[data-testid="agentcore-runner-last-run-transcript"]').text()
    ).not.toContain('execution gate improved after managed runner check')

    await wrapper.get('[data-testid="agentcore-runner-transcript-toggle"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="agentcore-runner-transcript-toggle"]').text()).toContain(
      'Collapse'
    )
    expect(wrapper.get('[data-testid="agentcore-runner-last-run-transcript"]').text()).toContain(
      'Build recipe prepared.'
    )
    expect(wrapper.text()).toContain('Agentcore Runner prepared successfully')

    wrapper.unmount()
  })

  it('refreshes the ref dropdown tags after the repo changes', async () => {
    routeTab = 'proxy'
    vi.mocked(settingsApi.getAgentcoreRunnerTags)
      .mockResolvedValueOnce({
        data: {
          repo_url: 'https://github.com/IceWhaleTech/ZimaOS-Blue',
          default_ref: 'main',
          tags: ['v0.10.37', 'v0.10.36'],
        },
      } as never)
      .mockResolvedValueOnce({
        data: {
          repo_url: 'https://github.com/example/runner',
          default_ref: 'main',
          tags: ['v2.0.0', 'v1.9.0'],
        },
      } as never)

    const pinia = createPinia()
    setActivePinia(pinia)

    const wrapper = mount(SettingsView, {
      global: {
        plugins: [pinia, i18n],
      },
    })
    await settleSettingsAsyncTabComponents()

    await wrapper.get('[data-testid="agentcore-runner-repo-input"]').setValue('example/runner')
    await wrapper.get('[data-testid="agentcore-runner-repo-input"]').trigger('blur')
    await flushPromises()

    expect(settingsApi.patch).toHaveBeenCalledWith({
      experimental_agentcore_runner_repo_url: 'example/runner',
      experimental_agentcore_runner_ref: 'main',
    })
    expect(settingsApi.getAgentcoreRunnerTags).toHaveBeenLastCalledWith('example/runner')

    const refSelect = wrapper.get('[data-testid="agentcore-runner-ref-input"]')
    expect(refSelect.text()).toContain('v2.0.0')
    expect(refSelect.text()).not.toContain('v0.10.37')

    wrapper.unmount()
  })

  it('shows prepare failure state and restores the prepare button label', async () => {
    routeTab = 'proxy'
    let rejectPrepare: ((reason?: unknown) => void) | null = null
    const preparePromise = new Promise((_, reject) => {
      rejectPrepare = reject
    })
    vi.mocked(settingsApi.getAgentcoreRunnerStatus).mockResolvedValueOnce({
      data: {
        enabled: true,
        repo_url: 'https://github.com/example/runner',
        resolved_ref: 'main',
        resolved_commit: 'abc123',
        required_go_version: '1.24.0',
        installed_go_version: '1.24.0',
        toolchain_ready: true,
        binary_ready: false,
        binary_path: '',
        binary_sha256: '',
        last_prepare_at: '2026-04-03T12:00:00Z',
        last_prepare_state: 'failed',
        last_error: 'previous prepare error',
        last_optimization_run_id: '',
        last_optimization_at: '',
        last_optimization_state: '',
        last_optimization_summary: '',
      },
    } as never)
    vi.mocked(settingsApi.prepareAgentcoreRunner).mockReturnValueOnce(preparePromise as never)

    const pinia = createPinia()
    setActivePinia(pinia)

    const wrapper = mount(SettingsView, {
      global: {
        plugins: [pinia, i18n],
      },
    })
    await settleSettingsAsyncTabComponents()

    expect(wrapper.find('[data-testid="agentcore-runner-status-content"]').exists()).toBe(false)
    await wrapper.get('[data-testid="agentcore-runner-status-toggle"]').trigger('click')
    await flushPromises()

    const prepareButton = wrapper.get('[data-testid="agentcore-runner-prepare"]')
    expect(prepareButton.text()).toContain('Prepare Runner')
    expect(wrapper.get('[data-testid="agentcore-runner-resolved-commit"]').text()).toContain(
      'abc123'
    )

    await prepareButton.trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="agentcore-runner-prepare"]').text()).toContain('Preparing...')

    rejectPrepare?.(new Error('prepare failed'))
    await flushPromises()

    expect(wrapper.get('[data-testid="agentcore-runner-prepare"]').text()).toContain(
      'Prepare Runner'
    )
    expect(wrapper.text()).toContain('Failed to prepare Agentcore Runner')
    expect(settingsApi.getAgentcoreRunnerStatus).toHaveBeenCalledTimes(1)

    wrapper.unmount()
  })

  it('hides the legacy missing repo validation error from the status panel', async () => {
    routeTab = 'proxy'
    vi.mocked(settingsApi.getAgentcoreRunnerStatus).mockResolvedValueOnce({
      data: {
        enabled: true,
        repo_url: 'https://github.com/example/runner',
        resolved_ref: 'main',
        resolved_commit: 'abc123',
        required_go_version: '1.24.0',
        installed_go_version: '1.24.0',
        toolchain_ready: false,
        binary_ready: false,
        binary_path: '',
        binary_sha256: '',
        last_prepare_at: '2026-04-03T12:00:00Z',
        last_prepare_state: 'failed',
        last_error: 'repo url is required',
        last_optimization_run_id: '',
        last_optimization_at: '',
        last_optimization_state: '',
        last_optimization_summary: '',
      },
    } as never)

    const pinia = createPinia()
    setActivePinia(pinia)

    const wrapper = mount(SettingsView, {
      global: {
        plugins: [pinia, i18n],
      },
    })
    await settleSettingsAsyncTabComponents()

    await wrapper.get('[data-testid="agentcore-runner-status-toggle"]').trigger('click')
    await flushPromises()

    const lastError = wrapper.get('[data-testid="agentcore-runner-last-error"]')
    expect(lastError.text()).not.toContain('repo url is required')
    expect(lastError.text()).toContain('Not available')
    expect(lastError.classes()).toContain('text-gray-500')
    expect(lastError.classes()).not.toContain('text-red-600')

    wrapper.unmount()
  })

  it('refreshes agentcore runner status and last-run details without preparing again', async () => {
    routeTab = 'proxy'
    vi.mocked(settingsApi.getAgentcoreRunnerStatus)
      .mockResolvedValueOnce({
        data: {
          enabled: true,
          repo_url: 'https://github.com/example/runner',
          resolved_ref: 'main',
          resolved_commit: 'abc123',
          required_go_version: '1.24.0',
          installed_go_version: '1.24.0',
          toolchain_ready: true,
          binary_ready: true,
          binary_path: '/tmp/agentcore-runner',
          binary_sha256: 'deadbeef',
          last_prepare_at: '2026-04-03T12:00:00Z',
          last_prepare_state: 'ready',
          last_error: '',
          last_optimization_run_id: 'opt-123',
          last_optimization_at: '2026-04-03T12:05:00Z',
          last_optimization_state: 'failed',
          last_optimization_summary: 'selector gate still blocked',
        },
      } as never)
      .mockResolvedValueOnce({
        data: {
          enabled: true,
          repo_url: 'https://github.com/example/runner',
          resolved_ref: 'main',
          resolved_commit: 'abc789',
          required_go_version: '1.24.0',
          installed_go_version: '1.24.0',
          toolchain_ready: true,
          binary_ready: true,
          binary_path: '/tmp/agentcore-runner',
          binary_sha256: 'beadfeed',
          last_prepare_at: '2026-04-03T12:00:00Z',
          last_prepare_state: 'ready',
          last_error: '',
          last_optimization_run_id: 'opt-789',
          last_optimization_at: '2026-04-03T12:45:00Z',
          last_optimization_state: 'completed',
          last_optimization_summary: 'selector gate improved after refresh',
        },
      } as never)
    vi.mocked(settingsApi.getAgentcoreRunnerLastRun)
      .mockResolvedValueOnce({
        data: {
          id: 'opt-123',
          reason: 'selector_gate_failed',
          candidate_id: 'candidate-123',
          runner_protocol: 'acp',
          runner_error: 'selector gate still blocked',
          runner_transcript: [],
        },
      } as never)
      .mockResolvedValueOnce({
        data: {
          id: 'opt-789',
          reason: 'selector_gate_failed',
          candidate_id: 'candidate-789',
          runner_protocol: 'acp',
          runner_stop_reason: 'completed',
          runner_duration_ms: 245,
          optimization_surface: 'constraints',
          runner_response_text: 'selector gate improved after refresh',
          runner_transcript: [
            {
              direction: 'out',
              method: 'session/prompt',
              text: 'Optimization trigger received.',
            },
            {
              direction: 'out',
              method: 'session/context',
              text: 'Constraint delta applied.',
            },
            {
              direction: 'in',
              method: 'session/update',
              text: 'selector gate improved after refresh',
            },
          ],
        },
      } as never)

    const pinia = createPinia()
    setActivePinia(pinia)

    const wrapper = mount(SettingsView, {
      global: {
        plugins: [pinia, i18n],
      },
    })
    await settleSettingsAsyncTabComponents()

    expect(wrapper.find('[data-testid="agentcore-runner-status-content"]').exists()).toBe(false)
    await wrapper.get('[data-testid="agentcore-runner-status-toggle"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="agentcore-runner-resolved-commit"]').text()).toContain('abc123')
    expect(wrapper.get('[data-testid="agentcore-runner-last-optimization-summary"]').text()).toContain(
      'selector gate still blocked'
    )
    expect(wrapper.get('[data-testid="agentcore-runner-last-run-error"]').text()).toContain(
      'selector gate still blocked'
    )

    await wrapper.get('[data-testid="agentcore-runner-refresh"]').trigger('click')
    await flushPromises()

    expect(settingsApi.prepareAgentcoreRunner).not.toHaveBeenCalled()
    expect(settingsApi.getAgentcoreRunnerStatus).toHaveBeenCalledTimes(2)
    expect(settingsApi.getAgentcoreRunnerLastRun).toHaveBeenCalledTimes(2)
    expect(wrapper.get('[data-testid="agentcore-runner-resolved-commit"]').text()).toContain('abc789')
    expect(wrapper.get('[data-testid="agentcore-runner-last-optimization-state"]').text()).toContain(
      'completed'
    )
    expect(wrapper.get('[data-testid="agentcore-runner-last-optimization-summary"]').text()).toContain(
      'selector gate improved after refresh'
    )
    expect(wrapper.get('[data-testid="agentcore-runner-last-run-meta"]').text()).toContain(
      'candidate-789'
    )
    expect(wrapper.get('[data-testid="agentcore-runner-last-run-meta"]').text()).toContain(
      'constraints'
    )
    expect(wrapper.get('[data-testid="agentcore-runner-last-run-response"]').text()).toContain(
      'selector gate improved after refresh'
    )
    expect(wrapper.get('[data-testid="agentcore-runner-last-run-transcript"]').text()).toContain(
      'Constraint delta applied.'
    )

    wrapper.unmount()
  })

  it('defaults auto reflection to enabled on memory tab when unset', async () => {
    routeTab = 'memory'
    const pinia = createPinia()
    setActivePinia(pinia)
    const store = useSettingsStore()

    const wrapper = mount(SettingsView, {
      global: {
        plugins: [pinia, i18n],
      },
    })
    await flushPromises()

    expect(store.agentAutoReflect).toBe(true)
    expect(wrapper.find('[data-testid="agent-auto-reflect-switch"]').exists()).toBe(false)

    wrapper.unmount()
  })

  it('defers llm and proxy bootstrap calls while the general tab is active', async () => {
    routeTab = 'general'
    const pinia = createPinia()
    setActivePinia(pinia)

    const wrapper = mount(SettingsView, {
      global: {
        plugins: [pinia, i18n],
      },
    })
    await flushPromises()

    expect(providerPoolApi.listProviders).not.toHaveBeenCalled()
    expect(settingsApi.getSmallModelStatus).not.toHaveBeenCalled()
    expect(settingsApi.getSmallModelStats).not.toHaveBeenCalled()
    expect(proxyCacheApi.getPrunerConfig).not.toHaveBeenCalled()
    expect(serviceApi.getInfo).toHaveBeenCalledTimes(1)
    expect(settingsApi.get).toHaveBeenCalledTimes(1)

    await wrapper.get('[data-tab="proxy"]').trigger('click')
    await flushPromises()

    expect(settingsApi.getSmallModelStatus).toHaveBeenCalledTimes(1)
    expect(settingsApi.getSmallModelStats).toHaveBeenCalledTimes(1)
    expect(proxyCacheApi.getPrunerConfig).toHaveBeenCalledTimes(1)

    await wrapper.get('[data-tab="llm"]').trigger('click')
    await flushPromises()

    expect(providerPoolApi.listProviders).not.toHaveBeenCalled()

    wrapper.unmount()
  })

  it('opens the llm tab directly even when no providers are configured', async () => {
    routeTab = 'proxy'
    const pinia = createPinia()
    setActivePinia(pinia)

    const wrapper = mount(SettingsView, {
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
    await settleSettingsAsyncTabComponents()

    expect(wrapper.findComponent({ name: 'ProviderPoolSection' }).exists()).toBe(true)
    expect(wrapper.findComponent({ name: 'ExternalAgentsSection' }).exists()).toBe(true)
    wrapper.unmount()
  })

  it('honors an llm tab query without redirecting back to general', async () => {
    routeTab = 'llm'
    const pinia = createPinia()
    setActivePinia(pinia)

    const wrapper = mount(SettingsView, {
      global: {
        plugins: [pinia, i18n],
      },
    })
    await settleSettingsAsyncTabComponents()

    expect(wrapper.findComponent({ name: 'ProviderPoolSection' }).exists()).toBe(true)
    expect(wrapper.findComponent({ name: 'ExternalAgentsSection' }).exists()).toBe(true)
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
