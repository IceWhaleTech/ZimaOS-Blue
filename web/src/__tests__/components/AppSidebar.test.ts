import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createRouter, createWebHistory } from 'vue-router'
import AppSidebar from '@/components/AppSidebar.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'Home', component: { template: '<div>Home</div>' } },
    { path: '/dashboard', name: 'Dashboard', component: { template: '<div>Dashboard</div>' } },
  ],
})

describe('AppSidebar', () => {
  beforeEach(async () => {
    router.push('/')
    await router.isReady()
  })

  it('should render navigation items', () => {
    const wrapper = mount(AppSidebar, {
      global: {
        plugins: [router],
      },
    })

    expect(wrapper.text()).toContain('Home')
    expect(wrapper.text()).toContain('Dashboard')
  })

  it('should highlight active route', async () => {
    const wrapper = mount(AppSidebar, {
      global: {
        plugins: [router],
      },
    })

    const homeLink = wrapper.find('a[href="/"]')
    expect(homeLink.classes()).toContain('bg-blue-50')
  })

  it('should navigate to dashboard', async () => {
    const wrapper = mount(AppSidebar, {
      global: {
        plugins: [router],
      },
    })

    await router.push('/dashboard')
    await wrapper.vm.$nextTick()

    const dashboardLink = wrapper.find('a[href="/dashboard"]')
    expect(dashboardLink.classes()).toContain('bg-blue-50')
  })
})
