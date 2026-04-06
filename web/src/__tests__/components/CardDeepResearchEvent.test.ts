import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'

import CardDeepResearchEvent from '@/components/typeless/CardDeepResearchEvent.vue'
import zhCN from '@/i18n/locales/zh-CN'
import { mergeHarnessLocale } from '@/i18n/harness-locale-additions'

function createTestI18n(locale = 'en-US') {
  return createI18n({
    legacy: false,
    locale,
    fallbackLocale: 'en-US',
    messages: {
      'en-US': {
        chat: {
          deepResearchProcess: 'Research process',
          deepResearchLiveSources: 'Live sources',
          deepResearchResearchBrief: 'Research brief',
          deepResearchMustVerify: 'Must verify',
          deepResearchRetryGuidance: 'Retry guidance',
          deepResearchRetryQueries: 'Recovery queries',
          deepResearchPlannedTasks: 'Planned tasks',
          deepResearchFollowUpQuery: 'Follow-up query',
          deepResearchVerificationSummary: 'Verification',
          deepResearchVerificationResolved: 'Resolved',
          deepResearchVerificationConflicted: 'Conflicted',
          deepResearchVerificationInsufficient: 'Insufficient',
          deepResearchStageErrors: 'Stage warnings',
          deepResearchIteration: 'Iteration',
          deepResearchGapNeedPrimaryOrOfficialSources: 'Need primary or official sources',
          deepResearchActionVerificationCompleted: 'Verification completed',
          deepResearchActionFollowupPlanned: 'Follow-up planned',
          deepResearchActionSynthesizing: 'Synthesizing report',
          deepResearchActionLoopStopped: 'Research loop stopped',
          deepResearchLatestGap: 'Research gap',
          deepResearchStopReasonCoverage: 'Coverage target reached',
        },
      },
      'zh-CN': {
        chat: {
          deepResearchPlannedTasks: '规划任务',
          deepResearchGapNeedPrimaryOrOfficialSources: '需要一手或官方来源',
          deepResearchStopReasonCoverage: '已达到覆盖目标',
          deepResearchMetaOfficial: '官方',
          deepResearchMetaFinancial: '财务',
        },
      },
    },
  })
}

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

describe('CardDeepResearchEvent', () => {
  it('renders brief, tasks, live sources, gap, and verification summary', () => {
    const wrapper = mount(CardDeepResearchEvent, {
      props: {
        card: {
          type: 'deep-research-event',
          event_kind: 'verification',
          status: 'warning',
          summary: 'Verification pass completed',
          query: 'research topic',
          iteration: 2,
          brief: {
            goal: 'Check the latest vendor disclosures.',
            must_verify_claims: ['Revenue growth', 'Customer count'],
            retry_context:
              'Harness retry guidance: recover the missing official confirmation before finishing.',
            retry_queries: ['vendor revenue growth primary source verification'],
          },
          tasks: [
            {
              question: 'Compare earnings release against filings',
              axis: 'official',
              category: 'financial',
              time_window: 'Q4 2025',
            },
          ],
          sources: [
            {
              title: 'Investor relations',
              domain: 'example.com',
              url: 'https://example.com/ir',
            },
          ],
          verification: {
            resolved_count: 3,
            conflicted_count: 1,
            insufficient_count: 2,
          },
          gap: 'Need one more primary source.',
          follow_up_query: 'company q4 2025 filing pdf',
        },
      },
      global: {
        plugins: [createTestI18n()],
      },
    })

    expect(wrapper.text()).toContain('Verification completed')
    expect(wrapper.text()).toContain('Research brief')
    expect(wrapper.text()).toContain('Retry guidance')
    expect(wrapper.text()).toContain('Recovery queries')
    expect(wrapper.text()).toContain('recover the missing official confirmation')
    expect(wrapper.text()).toContain('vendor revenue growth primary source verification')
    expect(wrapper.text()).toContain('Compare earnings release against filings')
    expect(wrapper.text()).toContain('Investor relations')
    expect(wrapper.text()).toContain('Need primary or official sources')
    expect(wrapper.text()).toContain('Resolved 3')
    expect(wrapper.text()).toContain('Conflicted 1')
    expect(wrapper.text()).toContain('Insufficient 2')
  })

  it('localizes structured task metadata and payload summaries for zh-CN events', () => {
    const wrapper = mount(CardDeepResearchEvent, {
      props: {
        card: {
          type: 'deep-research-event',
          event_kind: 'planning',
          status: 'info',
          summary: 'Planned 2 research task(s)',
          focus: 'official',
          gap: 'Need one primary source',
          stop_reason: 'coverage_sufficient',
          tasks: [
            {
              question: '核对官方披露',
              axis: 'official',
              category: 'financial',
              time_window: '2025',
            },
          ],
        },
      },
      global: {
        plugins: [createTestI18n('zh-CN')],
      },
    })

    expect(wrapper.text()).toContain('官方')
    expect(wrapper.text()).toContain('财务')
    expect(wrapper.text()).toContain('规划任务: 2')
    expect(wrapper.text()).toContain('需要一手或官方来源')
    expect(wrapper.text()).toContain('已达到覆盖目标')
    expect(wrapper.text()).not.toContain('official')
    expect(wrapper.text()).not.toContain('financial')
    expect(wrapper.text()).not.toContain('Planned 2 research task(s)')
  })

  it('localizes runtime focus and time window tokens for zh-CN events', () => {
    const wrapper = mount(CardDeepResearchEvent, {
      props: {
        card: {
          type: 'deep-research-event',
          event_kind: 'planning',
          status: 'info',
          summary: 'Planned 1 research task(s)',
          focus: 'Claim validation',
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

  it('localizes status badges for zh-CN runtime event cards', () => {
    const wrapper = mount(CardDeepResearchEvent, {
      props: {
        card: {
          type: 'deep-research-event',
          event_kind: 'verification',
          status: 'warning',
          summary: 'Verification pass completed',
        },
      },
      global: {
        plugins: [createRuntimeLocaleI18n('zh-CN')],
      },
    })

    expect(wrapper.text()).toContain('警告')
    expect(wrapper.text()).not.toContain('warning')
  })
})
