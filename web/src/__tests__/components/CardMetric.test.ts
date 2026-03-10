import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'

import CardMetric from '@/components/typeless/CardMetric.vue'

function createTestI18n(locale = 'zh-CN') {
  return createI18n({
    legacy: false,
    locale,
    fallbackLocale: 'en-US',
    messages: {
      'en-US': {
        resultCard: {
          labels: {
            functional_score: 'Functional Score',
            functional_issues: 'Functional Issues',
          },
        },
      },
      'zh-CN': {
        resultCard: {
          labels: {
            functional_score: '功能评分',
            functional_issues: '功能问题',
          },
        },
      },
    },
  })
}

describe('CardMetric', () => {
  it('localizes metric labels through result-card label keys', () => {
    const wrapper = mount(CardMetric, {
      props: {
        card: {
          type: 'metric',
          metrics: [
            { label: 'functional_score', value: 100 },
            { label: 'functional_issues', value: 0 },
            { label: 'custom_metric', value: 1 },
          ],
        },
      },
      global: {
        plugins: [createTestI18n()],
      },
    })

    expect(wrapper.text()).toContain('功能评分')
    expect(wrapper.text()).toContain('功能问题')
    expect(wrapper.text()).toContain('custom_metric')
    expect(wrapper.text()).not.toContain('functional_score')
    expect(wrapper.text()).not.toContain('functional_issues')
  })
})
