import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { ref } from 'vue'
import SettingsView from '@/views/SettingsView.vue'
import { i18n, setLocale } from '@/i18n'
import { backupApi } from '@/api/index'
import { memoryApi } from '@/api/memory'
import { providerPoolApi } from '@/api/providerPool'
import { settingsApi } from '@/api/settings'
import { serviceApi } from '@/api/service'

const { routeState, routerReplace, tauriState } = vi.hoisted(() => ({
  routeState: {
    tab: 'userdata',
    fullPath: '/settings?tab=userdata',
  },
  routerReplace: vi.fn(),
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
      fullPath: routeState.fullPath,
      query: { tab: routeState.tab },
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

vi.mock('@/components/KnowledgeManagerCard.vue', () => ({
  default: { name: 'KnowledgeManagerCard', template: '<div />' },
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

vi.mock('@/api/service', () => ({
  serviceApi: {
    getInfo: vi.fn(),
    install: vi.fn(),
    enable: vi.fn(),
    disable: vi.fn(),
    uninstall: vi.fn(),
  },
}))

vi.mock('@/api/memory', () => ({
  memoryApi: {
    store: vi.fn(),
    search: vi.fn(),
    get: vi.fn(),
    delete: vi.fn(),
    prune: vi.fn(),
    clear: vi.fn(),
    stats: vi.fn(),
    dreamStatus: vi.fn(),
    runDream: vi.fn(),
    exportMarkdown: vi.fn(),
    importMarkdown: vi.fn(),
  },
}))

vi.mock('@/composables/useTauri', () => ({
  useTauri: () => ({
    isTauri: ref(tauriState.isTauri),
    platform: ref(tauriState.platform),
    setCloseBehavior: tauriState.setCloseBehavior,
    restartServerRuntime: tauriState.restartServerRuntime,
  }),
}))

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

function statsPayload(overrides: Record<string, unknown> = {}) {
  return {
    data: {
      total_chunks: 1,
      total_size_bytes: 256,
      total_display_count: 1,
      total_display_size_bytes: 256,
      daily_logs_count: 0,
      daily_entries_count: 0,
      daily_total_size_bytes: 0,
      backend: 'markdown',
      ...overrides,
    },
  } as never
}

function dreamStatusPayload(overrides: Record<string, unknown> = {}) {
  return {
    data: {
      enabled: true,
      archive_dir: '/tmp/dream-archive',
      pending_capsules: 2,
      archived_daily_logs: 1,
      promoted_count: 5,
      last_run_at: '2026-04-17T10:00:00Z',
      ...overrides,
    },
  } as never
}

async function settleSettingsView() {
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
      small_model_knowledge_fix_enabled: true,
      small_model_context_compress_enabled: true,
      small_model_doc_extract_enabled: true,
      context_compression_mode: 'auto',
      small_model_route_short_qa_enabled: true,
      no_llm_degrade_mode: 'deepresearch',
      small_model_unavailable_policy: 'ir_first',
      small_model_context_prune_enabled: true,
      small_model_media_intent_enabled: true,
      offline_ir_fallback_enabled: true,
      feature_intent_ir_enabled: true,
    },
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
      short_qa_route_attempts: 0,
      short_qa_route_success: 0,
      image_qa_route_attempts: 0,
      image_qa_route_success: 0,
      summary_attempts: 0,
      summary_success: 0,
      context_compress_attempts: 0,
      context_compress_success: 0,
      context_compress_latency_ms: 0,
      doc_extract_attempts: 0,
      doc_extract_success: 0,
      no_provider_deepresearch_total: 0,
      ir_takeover_total: 0,
      fallback_reasons: {},
    },
  } as never)
  vi.mocked(backupApi.list).mockResolvedValue({ data: [] } as never)
  vi.mocked(serviceApi.getInfo).mockResolvedValue({
    data: { installed: false, enabled: false },
  } as never)
  vi.mocked(memoryApi.stats).mockResolvedValue(statsPayload())
  vi.mocked(memoryApi.search).mockResolvedValue({ data: { results: [], total: 0 } } as never)
  vi.mocked(memoryApi.dreamStatus)
    .mockResolvedValueOnce(dreamStatusPayload())
    .mockResolvedValueOnce(
      dreamStatusPayload({
        pending_capsules: 0,
        archived_daily_logs: 2,
        promoted_count: 7,
        last_run_at: '2026-04-17T10:05:00Z',
      })
    )
  vi.mocked(memoryApi.runDream).mockResolvedValue({
    data: {
      run_id: 'dream-run-99',
      promoted_count: 2,
      archived_daily_count: 1,
      processed_capsules: 2,
    },
  } as never)
}

function mountSettingsView() {
  const pinia = createPinia()
  setActivePinia(pinia)
  return mount(SettingsView, {
    global: {
      plugins: [pinia, i18n],
    },
  })
}

describe('SettingsView userdata dream integration', () => {
  beforeEach(async () => {
    localStorageMock.clear()
    routeState.tab = 'userdata'
    routeState.fullPath = '/settings?tab=userdata'
    routerReplace.mockReset()
    vi.clearAllMocks()
    tauriState.isTauri = false
    tauriState.platform = 'unknown'
    tauriState.setCloseBehavior.mockReset()
    tauriState.restartServerRuntime.mockReset()
    await setLocale('en-US')
    primeApiMocks()
    vi.stubGlobal(
      'confirm',
      vi.fn(() => true)
    )
  })

  it('renders and runs dream controls from the userdata settings tab', async () => {
    const wrapper = mountSettingsView()
    await settleSettingsView()

    expect(memoryApi.stats).toHaveBeenCalledTimes(1)
    expect(memoryApi.dreamStatus).toHaveBeenCalledTimes(1)
    expect(wrapper.get('[data-testid="memory-dream-panel"]').text()).toContain('Pending Capsules')
    expect(wrapper.get('[data-testid="memory-dream-panel"]').text()).toContain('2')

    await wrapper.get('[data-testid="memory-dream-run"]').trigger('click')
    await settleSettingsView()

    expect(memoryApi.runDream).toHaveBeenCalledTimes(1)
    expect(memoryApi.dreamStatus).toHaveBeenCalledTimes(2)
    expect(wrapper.get('[data-testid="memory-dream-panel"]').text()).toContain('7')
    expect(wrapper.text()).toContain('Dream run complete: 2 capsules processed')

    wrapper.unmount()
  })
})
