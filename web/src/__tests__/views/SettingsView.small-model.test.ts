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
import { serviceApi } from '@/api/service'

let routeTab: 'proxy' | 'memory' = 'proxy'
const routerReplace = vi.fn()

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
    getToolStats: vi.fn(),
    getSkillRerankerModelStatus: vi.fn(),
    downloadSkillRerankerModel: vi.fn(),
    cancelSkillRerankerModelDownload: vi.fn(),
    getSmallModelStatus: vi.fn(),
    downloadSmallModel: vi.fn(),
    cancelSmallModelDownload: vi.fn(),
    listSoulProposals: vi.fn(),
    approveSoulProposal: vi.fn(),
    rejectSoulProposal: vi.fn(),
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

vi.mock('@/composables/useTauri', () => ({
  useTauri: () => ({
    isTauri: false,
    setCloseBehavior: vi.fn(),
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
      small_model_runtime: 'llama_cpp_native',
      small_model_id: 'lfm2.5-1.2b-instruct-q4km',
      small_model_shadow_ratio: 0.1,
      no_llm_degrade_mode: 'deepresearch',
      small_model_unavailable_policy: 'ir_first',
    },
  } as never)
  vi.mocked(settingsApi.patch).mockImplementation(async (payload: unknown) => ({ data: payload }) as never)
  vi.mocked(settingsApi.getSkillRerankerModelStatus).mockResolvedValue({
    data: { ready: false, downloading: false, state: 'idle' },
  } as never)
  vi.mocked(settingsApi.getSmallModelStatus).mockResolvedValue({
    data: {
      ready: false,
      downloading: false,
      model_id: 'lfm2.5-1.2b-instruct-q4km',
      runtime: 'llama_cpp_native',
      model_path: '/tmp/model.gguf',
      state: 'idle',
    },
  } as never)
  vi.mocked(settingsApi.getSmallModelStats).mockResolvedValue({
    data: {
      short_qa_route_attempts: 12,
      short_qa_route_success: 10,
      tool_dispatch_route_attempts: 6,
      tool_dispatch_route_success: 5,
      short_qa_shadow_total: 8,
      tool_dispatch_shadow_total: 4,
      shadow_failures: 1,
      no_provider_deepresearch_total: 2,
      ir_takeover_total: 3,
      fallback_reasons: { timeout: 1 },
    },
  } as never)
  vi.mocked(settingsApi.resetSmallModelStats).mockResolvedValue({ data: { success: true } } as never)
  vi.mocked(settingsApi.listSoulProposals).mockResolvedValue({
    data: {
      proposals: [
        {
          id: 'p-1',
          title: 'Keep tests first',
          content: 'Always reproduce and add a regression test.',
          source: 'user-feedback',
          status: 'pending',
          created_at: '2026-03-01T00:00:00Z',
        },
      ],
    },
  } as never)
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
    primeApiMocks()
  })

  it('triggers shadow ratio update, download, and stats reset on proxy tab', async () => {
    routeTab = 'proxy'
    const pinia = createPinia()
    setActivePinia(pinia)
    const store = useSettingsStore()

    const ratioSpy = vi.spyOn(store, 'setSmallModelShadowRatio').mockResolvedValue()
    const downloadSpy = vi.spyOn(store, 'startSmallModelDownload').mockResolvedValue({} as never)
    const resetSpy = vi.spyOn(store, 'resetSmallModelStats').mockResolvedValue({} as never)

    const wrapper = mount(SettingsView, {
      shallow: true,
      global: {
        plugins: [pinia, i18n],
      },
    })
    await flushPromises()

    await wrapper.get('[data-testid="small-model-shadow-ratio-30"]').trigger('click')
    await flushPromises()
    expect(ratioSpy).toHaveBeenCalledWith(0.3)

    await wrapper.get('[data-testid="small-model-download"]').trigger('click')
    await flushPromises()
    expect(downloadSpy).toHaveBeenCalledTimes(1)

    await wrapper.get('[data-testid="small-model-stats-reset"]').trigger('click')
    await flushPromises()
    expect(resetSpy).toHaveBeenCalledTimes(1)

    wrapper.unmount()
  })

  it('triggers SOUL proposal approve/reject actions on memory tab', async () => {
    routeTab = 'memory'
    const pinia = createPinia()
    setActivePinia(pinia)
    const store = useSettingsStore()

    const approveSpy = vi.spyOn(store, 'approveSoulProposal').mockResolvedValue({} as never)
    const rejectSpy = vi.spyOn(store, 'rejectSoulProposal').mockResolvedValue({} as never)

    const wrapper = mount(SettingsView, {
      shallow: true,
      global: {
        plugins: [pinia, i18n],
      },
    })
    await flushPromises()

    await wrapper.get('[data-testid="soul-approve-p-1"]').trigger('click')
    await flushPromises()
    expect(approveSpy).toHaveBeenCalledWith('p-1')

    await wrapper.get('[data-testid="soul-reject-p-1"]').trigger('click')
    await flushPromises()
    expect(rejectSpy).toHaveBeenCalledWith('p-1')

    wrapper.unmount()
  })
})
