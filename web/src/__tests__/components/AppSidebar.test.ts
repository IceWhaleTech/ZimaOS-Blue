import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createRouter, createWebHistory } from 'vue-router'
import AppSidebar from '@/components/AppSidebar.vue'
import { createPinia, setActivePinia } from 'pinia'
import { i18n } from '@/i18n'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'Home', component: { template: '<div>Home</div>' } },
    { path: '/dashboard', name: 'Dashboard', component: { template: '<div>Dashboard</div>' } },
  ],
})

describe('AppSidebar', () => {
  beforeEach(async () => {
    setActivePinia(createPinia())
    router.push('/')
    await router.isReady()
  })

  it('should render navigation items', () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const wrapper = mount(AppSidebar, {
      global: {
        plugins: [pinia, router, i18n],
      },
    })

    expect(wrapper.text()).toContain('Home')
    expect(wrapper.text()).toContain('Dashboard')
  })

  it('should highlight active route', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const wrapper = mount(AppSidebar, {
      global: {
        plugins: [pinia, router, i18n],
      },
    })

    const homeLink = wrapper.find('a[href="/"]')
    expect(homeLink.classes()).toContain('bg-accent/20')
  })

  it('should navigate to dashboard', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const wrapper = mount(AppSidebar, {
      global: {
        plugins: [pinia, router, i18n],
      },
    })

    await router.push('/dashboard')
    await wrapper.vm.$nextTick()

    const dashboardLink = wrapper.find('a[href="/dashboard"]')
    expect(dashboardLink.classes()).toContain('bg-accent/20')
  })
})
