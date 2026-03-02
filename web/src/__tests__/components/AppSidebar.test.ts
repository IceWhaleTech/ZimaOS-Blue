import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import AppSidebar from '@/components/AppSidebar.vue'
import { createPinia, setActivePinia } from 'pinia'
import { i18n } from '@/i18n'
import { useAuthStore } from '@/stores/auth'

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

function createTestRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/home', name: 'Home', component: { template: '<div>Home</div>' } },
      { path: '/chat', name: 'Chat', component: { template: '<div>Chat</div>' } },
      { path: '/settings', name: 'Settings', component: { template: '<div>Settings</div>' } },
    ],
  })
}

describe('AppSidebar', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorageMock.clear()
  })

  it('should render navigation items for admin user', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const authStore = useAuthStore()
    authStore.$patch({
      token: 'test-token',
      user: { username: 'admin', role: 'admin' } as never,
    })
    const router = createTestRouter()
    router.push('/home')
    await router.isReady()
    const wrapper = mount(AppSidebar, {
      global: {
        plugins: [pinia, router, i18n],
      },
    })

    expect(wrapper.text()).toContain('Dashboard')
    expect(wrapper.text()).toContain('Chat')
  })

  it('should highlight active route', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const authStore = useAuthStore()
    authStore.$patch({
      token: 'test-token',
      user: { username: 'admin', role: 'admin' } as never,
    })
    const router = createTestRouter()
    router.push('/home')
    await router.isReady()
    const wrapper = mount(AppSidebar, {
      global: {
        plugins: [pinia, router, i18n],
      },
    })

    const homeLink = wrapper.find('a[href="/home"]')
    expect(homeLink.exists()).toBe(true)
    expect(homeLink.classes()).toContain('bg-gray-100')
  })

  it('should highlight chat route after navigation', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const authStore = useAuthStore()
    authStore.$patch({
      token: 'test-token',
      user: { username: 'admin', role: 'admin' } as never,
    })
    const router = createTestRouter()
    router.push('/home')
    await router.isReady()
    const wrapper = mount(AppSidebar, {
      global: {
        plugins: [pinia, router, i18n],
      },
    })

    await router.push('/chat')
    await wrapper.vm.$nextTick()

    const chatLink = wrapper.find('a[href="/chat"]')
    expect(chatLink.exists()).toBe(true)
    expect(chatLink.classes()).toContain('bg-gray-100')
  })
})
