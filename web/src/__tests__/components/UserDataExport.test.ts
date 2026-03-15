import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { i18n, setLocale } from '@/i18n'
import UserDataExport from '@/components/UserDataExport.vue'
import { companionSettingsApi } from '@/api/companion'

vi.mock('@/api/userdata', () => ({
  userDataApi: {
    export: vi.fn(),
    importPreview: vi.fn(),
    import: vi.fn(),
    previewCleanup: vi.fn(),
    cleanup: vi.fn(),
  },
}))

vi.mock('@/api/companion', () => ({
  companionSettingsApi: {
    getSettings: vi.fn(),
  },
}))

vi.mock('@/stores/settings', () => ({
  useSettingsStore: () => ({
    selectedProviderModel: 'gpt-5',
    temperature: 0.7,
    maxTokens: 2048,
  }),
}))

vi.mock('@/stores/locale', () => ({
  useLocaleStore: () => ({
    currentLocale: 'en-US',
  }),
}))

vi.mock('@/stores/theme', () => ({
  useThemeStore: () => ({
    theme: 'light',
  }),
}))

vi.mock('@/stores/chat', () => ({
  useChatStore: () => ({
    conversations: [],
  }),
}))

vi.mock('@/stores/preview', () => ({
  usePreviewStore: () => ({
    isPreviewMode: false,
  }),
}))

const storageState = new Map<string, string>()
const localStorageMock = {
  getItem: (key: string) => storageState.get(key) ?? null,
  setItem: (key: string, value: string) => {
    storageState.set(key, value)
  },
  removeItem: (key: string) => {
    storageState.delete(key)
  },
  clear: () => {
    storageState.clear()
  },
}

describe('UserDataExport', () => {
  beforeEach(async () => {
    vi.clearAllMocks()
    storageState.clear()
    vi.stubGlobal('localStorage', localStorageMock)
    if (typeof window !== 'undefined') {
      Object.defineProperty(window, 'localStorage', {
        value: localStorageMock,
        configurable: true,
      })
    }
    vi.mocked(companionSettingsApi.getSettings).mockResolvedValue({
      data: {
        storage_info: {
          session_count: 0,
          alert_count: 0,
          event_count: 0,
        },
      },
    } as never)
    await setLocale('en-US')
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('inherits layout classes from the parent settings section', async () => {
    const wrapper = mount(UserDataExport, {
      attrs: {
        class: 'mx-auto max-w-6xl',
      },
      global: {
        plugins: [i18n],
        stubs: {
          teleport: true,
        },
      },
    })

    await flushPromises()

    expect(companionSettingsApi.getSettings).toHaveBeenCalledTimes(1)
    expect(wrapper.classes()).toEqual(expect.arrayContaining(['w-full', 'mx-auto', 'max-w-6xl']))
    expect(wrapper.text()).toContain('Chat Data')

    wrapper.unmount()
  })
})
