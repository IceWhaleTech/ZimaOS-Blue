import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'

import CardUIReview from '@/components/typeless/CardUIReview.vue'

function createTestI18n(locale = 'en-US') {
  return createI18n({
    legacy: false,
    locale,
    fallbackLocale: 'en-US',
    messages: {
      'en-US': {
        uiReview: {
          title: 'UI Review',
          visual: 'Visual',
          functional: 'Functional',
          accessibility: 'Accessibility',
          showScreenshot: 'Show screenshot',
          hideScreenshot: 'Hide screenshot',
          actions: {
            recheck: 'Re-check',
            check_a11y: 'Accessibility Only',
            full_report: 'Full Report',
          },
        },
        security: {
          scan: {
            passed: 'PASS',
            failed: 'FAIL',
          },
        },
        common: {
          processing: 'Processing...',
        },
        askQuestion: {
          browserCheckpoint: {
            screenshotAlt: 'Page screenshot',
          },
        },
      },
    },
  })
}

describe('CardUIReview', () => {
  it('renders device/channel badges and screenshot URLs', async () => {
    const wrapper = mount(CardUIReview, {
      props: {
        card: {
          type: 'ui-review',
          url: 'https://example.com',
          overall: 82,
          pass: true,
          visual: { score: 80 },
          functional: { score: 84 },
          accessibility: { score: 81 },
          device: 'desktop',
          channel: 'feishu',
          screenshots: ['/api/v1/media/ui-review/a.png'],
          actions: [],
        },
      },
      global: {
        plugins: [createTestI18n()],
      },
    })

    expect(wrapper.text()).toContain('desktop')
    expect(wrapper.text()).toContain('feishu')
    await wrapper.find('button').trigger('click')
    expect(wrapper.text()).toContain('Show screenshot')
    await wrapper.findAll('button')[1]?.trigger('click')
    const img = wrapper.find('img')
    expect(img.exists()).toBe(true)
    expect(img.attributes('src')).toContain('/api/v1/media/ui-review/a.png')
  })
})
