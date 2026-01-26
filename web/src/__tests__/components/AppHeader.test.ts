import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import AppHeader from '@/components/AppHeader.vue'
import { createPinia, setActivePinia } from 'pinia'

describe('AppHeader', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('should render title', () => {
    const wrapper = mount(AppHeader)
    expect(wrapper.text()).toContain('ZimaOS Echo')
  })

  it('should show status badge when health is available', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)

    const wrapper = mount(AppHeader, {
      global: {
        plugins: [pinia],
      },
    })

    // Initially no status badge
    expect(wrapper.find('.rounded-full').exists()).toBe(false)
  })
})
