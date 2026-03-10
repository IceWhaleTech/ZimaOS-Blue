import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'

import CardAnalyzeProgress from '@/components/typeless/CardAnalyzeProgress.vue'

function createTestI18n(locale = 'zh-CN') {
  return createI18n({
    legacy: false,
    locale,
    fallbackLocale: 'en-US',
    messages: {
      'en-US': {
        analyze: {
          analyzing: 'Analyzing',
          meta: {
            chars: '{count} chars',
            results: '{count} results',
          },
          steps: {
            data_collection: 'Gathering data',
            doc_extract: 'Structuring key points',
            analysis: 'Analyzing content',
            report: 'Generating report',
            save_report: 'Saving report',
          },
        },
      },
      'zh-CN': {
        analyze: {
          analyzing: '正在分析',
          meta: {
            chars: '{count} 个字符',
            results: '{count} 条结果',
          },
          steps: {
            data_collection: '收集数据',
            doc_extract: '提炼结构化要点',
            analysis: '分析内容',
            report: '生成报告',
            save_report: '保存报告',
          },
        },
      },
    },
  })
}

describe('CardAnalyzeProgress', () => {
  it('renders detailed analyze progress metadata', () => {
    const wrapper = mount(CardAnalyzeProgress, {
      props: {
        card: {
          type: 'analyze-progress',
          steps: [
            {
              step: 'url_fetch_1',
              name: '抓取网页 1/2',
              status: 'success',
              current: 1,
              total: 2,
              source_label: 'https://example.com',
              detail: '网页内容已提取',
            },
            {
              step: 'doc_extract',
              name: '提炼结构化要点',
              status: 'skipped',
              detail: '已跳过小模型预处理',
            },
          ],
        },
      },
      global: {
        plugins: [createTestI18n('zh-CN')],
      },
    })

    expect(wrapper.text()).toContain('正在分析')
    expect(wrapper.text()).toContain('抓取网页 1/2')
    expect(wrapper.text()).toContain('https://example.com')
    expect(wrapper.text()).toContain('网页内容已提取')
    expect(wrapper.text()).toContain('1/2')
    expect(wrapper.text()).toContain('提炼结构化要点')
    expect(wrapper.text()).toContain('已跳过小模型预处理')
  })
})
