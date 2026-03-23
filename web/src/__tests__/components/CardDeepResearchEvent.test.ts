import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'

import CardDeepResearchEvent from '@/components/typeless/CardDeepResearchEvent.vue'

function createTestI18n() {
  return createI18n({
    legacy: false,
    locale: 'en-US',
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
          deepResearchActionSynthesizing: 'Synthesizing report',
          deepResearchActionLoopStopped: 'Research loop stopped',
          deepResearchLatestGap: 'Research gap',
        },
      },
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

    expect(wrapper.text()).toContain('Verification pass completed')
    expect(wrapper.text()).toContain('Research brief')
    expect(wrapper.text()).toContain('Retry guidance')
    expect(wrapper.text()).toContain('Recovery queries')
    expect(wrapper.text()).toContain('recover the missing official confirmation')
    expect(wrapper.text()).toContain('vendor revenue growth primary source verification')
    expect(wrapper.text()).toContain('Compare earnings release against filings')
    expect(wrapper.text()).toContain('Investor relations')
    expect(wrapper.text()).toContain('Need one more primary source.')
    expect(wrapper.text()).toContain('Resolved 3')
    expect(wrapper.text()).toContain('Conflicted 1')
    expect(wrapper.text()).toContain('Insufficient 2')
  })
})
