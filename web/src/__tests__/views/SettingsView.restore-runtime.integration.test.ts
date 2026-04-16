import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { ref } from 'vue'
import SettingsView from '@/views/SettingsView.vue'
import { i18n, setLocale } from '@/i18n'
import { backupApi } from '@/api/index'
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

vi.mock('@/components/MemoryManager.vue', () => ({
  default: { name: 'MemoryManager', template: '<div />' },
}))

vi.mock('@/components/KnowledgeManagerCard.vue', () => ({
  default: { name: 'KnowledgeManagerCard', template: '<div />' },
}))

vi.mock('@/components/BackupManager.vue', () => ({
  default: {
    name: 'BackupManager',
    template:
      '<button data-testid="mock-backup-restore" @click="$emit(\'restore\', \'backup-1\')" />',
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
}

function mountRestoreView() {
  const pinia = createPinia()
  setActivePinia(pinia)
  return mount(SettingsView, {
    global: {
      plugins: [pinia, i18n],
    },
  })
}

describe('SettingsView restore runtime integration', () => {
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
    tauriState.restartServerRuntime.mockResolvedValue(true)
    await setLocale('en-US')
    primeApiMocks()
    vi.stubGlobal(
      'confirm',
      vi.fn(() => true)
    )
  })

  it('routes restore through the desktop shell and keeps restart in-process', async () => {
    tauriState.isTauri = true
    vi.mocked(backupApi.restore).mockResolvedValue({
      data: {
        success: true,
        message: 'Restore staged successfully.',
        requires_restart: true,
        restarting: false,
      },
    } as never)

    const wrapper = mountRestoreView()
    await settleSettingsAsyncTabComponents()

    await wrapper.get('[data-testid="mock-backup-restore"]').trigger('click')
    await settleSettingsAsyncTabComponents()

    expect(backupApi.restore).toHaveBeenCalledWith(
      'backup-1',
      expect.objectContaining({
        require_restart: true,
        auto_restart: false,
        create_checkpoint: true,
      })
    )
    expect(tauriState.restartServerRuntime).toHaveBeenCalledWith('/settings?tab=userdata')

    wrapper.unmount()
  })

  it('does not render the knowledge management section in the data management tab', async () => {
    const wrapper = mountRestoreView()
    await settleSettingsAsyncTabComponents()

    expect(wrapper.text()).not.toContain('Knowledge Space')
    expect(wrapper.text()).not.toContain('Knowledge Maintenance & Compilation')

    wrapper.unmount()
  })

  it('keeps server-managed auto restart outside the desktop shell in browser mode', async () => {
    vi.mocked(backupApi.restore).mockResolvedValue({
      data: {
        success: true,
        message: 'Restore staged successfully.',
        requires_restart: true,
        restarting: true,
      },
    } as never)

    const wrapper = mountRestoreView()
    await settleSettingsAsyncTabComponents()

    await wrapper.get('[data-testid="mock-backup-restore"]').trigger('click')
    await settleSettingsAsyncTabComponents()

    expect(backupApi.restore).toHaveBeenCalledWith(
      'backup-1',
      expect.objectContaining({
        require_restart: true,
        auto_restart: true,
        create_checkpoint: true,
      })
    )
    expect(tauriState.restartServerRuntime).not.toHaveBeenCalled()

    wrapper.unmount()
  })
})
