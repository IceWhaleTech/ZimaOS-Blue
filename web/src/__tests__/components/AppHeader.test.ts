import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import AppHeader from '@/components/AppHeader.vue'
import { createPinia, setActivePinia } from 'pinia'
import { i18n } from '@/i18n'
import { useSystemStore } from '@/stores/system'

vi.mock('@/api/preview', () => ({
  previewApi: {
    getSystemMode: vi.fn().mockResolvedValue({ data: { mode: 'normal', features: {} } }),
    getStatus: vi.fn().mockResolvedValue({ data: { mode: 'normal' } }),
    getPreviewToken: vi.fn().mockResolvedValue({ data: { token: '' } }),
    upgrade: vi.fn(),
  },
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
vi.stubGlobal(
  'matchMedia',
  vi.fn().mockImplementation(() => ({
    matches: false,
    media: '',
    onchange: null,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }))
)

function createTestRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/home', component: { template: '<div>Home</div>' } },
      { path: '/profile', component: { template: '<div>Profile</div>' } },
      { path: '/login', component: { template: '<div>Login</div>' } },
    ],
  })
}

describe('AppHeader', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorageMock.clear()
  })

  it('should render title', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const router = createTestRouter()
    router.push('/home')
    await router.isReady()
    const wrapper = mount(AppHeader, {
      shallow: true,
      global: {
        plugins: [pinia, i18n, router],
      },
    })
    expect(wrapper.text()).toContain('brand.name')
  })

  it('should show status badge when health is available', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const systemStore = useSystemStore()
    systemStore.$patch({
      health: { status: 'ok', version: '1.0.0' } as never,
    })
    const router = createTestRouter()
    router.push('/home')
    await router.isReady()

    const wrapper = mount(AppHeader, {
      shallow: true,
      global: {
        plugins: [pinia, i18n, router],
      },
    })

    expect(wrapper.text()).toContain('common.online')
  })
})
