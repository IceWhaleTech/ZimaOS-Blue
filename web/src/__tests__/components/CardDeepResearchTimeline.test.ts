import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'

import CardDeepResearchTimeline from '@/components/typeless/CardDeepResearchTimeline.vue'
import zhCN from '@/i18n/locales/zh-CN'
import { mergeHarnessLocale } from '@/i18n/harness-locale-additions'

function createRuntimeLocaleI18n(locale = 'zh-CN') {
  const baseMessages =
    locale === 'zh-CN' ? (zhCN as Record<string, unknown>) : ({ chat: {} } as Record<string, unknown>)

  return createI18n({
    legacy: false,
    locale,
    fallbackLocale: 'en-US',
    messages: {
      [locale]: mergeHarnessLocale(locale as any, baseMessages),
    },
  })
}

describe('CardDeepResearchTimeline', () => {
  it('localizes runtime focus and time window tokens for zh-CN timeline cards', () => {
    const wrapper = mount(CardDeepResearchTimeline, {
      props: {
        card: {
          type: 'deep-research-timeline',
          query: '深度研究时间线',
          status: 'running',
          progress: 42,
          iteration: 1,
          stage: 'retrieve',
          mode: 'standard',
          steps: [
            {
              id: 'planning-step',
              type: 'deep-research-event',
              event_kind: 'planning',
              status: 'info',
              summary: 'Planned 1 research task(s)',
              iteration: 1,
              brief: {
                time_windows: ['recent'],
              },
              tasks: [
                {
                  question: '核对阶段性变化',
                  axis: 'latest',
                  time_window: 'earlier',
                },
                {
                  question: '核对完整时间范围',
                  time_window: '2025-2026',
                },
              ],
              focus: 'Claim validation',
            },
          ],
        },
      },
      global: {
        plugins: [createRuntimeLocaleI18n('zh-CN')],
      },
    })

    expect(wrapper.text()).toContain('规划任务: 1')
    expect(wrapper.text()).toContain('最新动态')
    expect(wrapper.text()).toContain('结论核验')
    expect(wrapper.text()).toContain('近期')
    expect(wrapper.text()).toContain('早期')
    expect(wrapper.text()).toContain('2025-2026')
    expect(wrapper.text()).not.toContain('latest')
    expect(wrapper.text()).not.toContain('Claim validation')
    expect(wrapper.text()).not.toContain('recent')
    expect(wrapper.text()).not.toContain('earlier')
    expect(wrapper.text()).not.toContain('2025 2026')
  })

  it('localizes per-step status badges for zh-CN timeline cards', () => {
    const wrapper = mount(CardDeepResearchTimeline, {
      props: {
        card: {
          type: 'deep-research-timeline',
          query: '深度研究时间线',
          status: 'running',
          progress: 42,
          iteration: 1,
          stage: 'retrieve',
          mode: 'standard',
          steps: [
            {
              id: 'info-step',
              type: 'deep-research-event',
              event_kind: 'planning',
              status: 'info',
              summary: 'Planned 1 research task(s)',
            },
            {
              id: 'warning-step',
              type: 'deep-research-event',
              event_kind: 'verification',
              status: 'warning',
              summary: 'Verification pass completed',
            },
          ],
        },
      },
      global: {
        plugins: [createRuntimeLocaleI18n('zh-CN')],
      },
    })

    expect(wrapper.text()).toContain('信息')
    expect(wrapper.text()).toContain('警告')
    expect(wrapper.text()).not.toContain('info')
    expect(wrapper.text()).not.toContain('warning')
  })
})
