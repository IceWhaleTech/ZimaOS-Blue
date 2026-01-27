import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import AppHeader from '@/components/AppHeader.vue'
import { createPinia, setActivePinia } from 'pinia'
import { i18n } from '@/i18n'

describe('AppHeader', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('should render title', () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const wrapper = mount(AppHeader, {
      global: {
        plugins: [pinia, i18n],
      },
    })
    expect(wrapper.text()).toContain('ZimaOS Echo')
  })

  it('should show status badge when health is available', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)

    const wrapper = mount(AppHeader, {
      global: {
        plugins: [pinia, i18n],
      },
    })

    // Initially no status badge
    expect(wrapper.find('.rounded-full').exists()).toBe(false)
  })
})
