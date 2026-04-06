import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'

import CardAnalyze from '@/components/typeless/CardAnalyze.vue'

function createTestI18n(locale = 'en-US') {
  return createI18n({
    legacy: false,
    locale,
    fallbackLocale: 'en-US',
    messages: {
      'en-US': {
        analyze: {
          title: 'Analysis Report',
          reportLink: 'Open report',
          outputMode: {
            inline: 'Inline answer',
            report: 'Report output',
          },
          reportStyle: 'Style',
        },
      },
      'zh-CN': {
        analyze: {
          title: '分析报告',
          reportLink: '打开报告',
          outputMode: {
            inline: '内联回答',
            report: '报告输出',
          },
          reportStyle: '样式',
        },
      },
    },
  })
}

describe('CardAnalyze', () => {
  it('renders a dedicated analyze summary card with report metadata', () => {
    const wrapper = mount(CardAnalyze, {
      props: {
        card: {
          type: 'analyze',
          title: 'Market analysis',
          status: 'success',
          answer: 'Key findings go here.',
          output_mode: 'report',
          report_style: 'dashboard',
          report_url: '/api/v1/media/analyze/r1.html',
        },
      },
      global: {
        plugins: [createTestI18n('en-US')],
      },
    })

    expect(wrapper.text()).toContain('Market analysis')
    expect(wrapper.text()).toContain('Key findings go here.')
    expect(wrapper.text()).toContain('Report output')
    expect(wrapper.text()).toContain('dashboard')

    const link = wrapper.get('a')
    expect(link.attributes('href')).toBe('/api/v1/media/analyze/r1.html')
    expect(link.text()).toContain('Open report')
  })
})
