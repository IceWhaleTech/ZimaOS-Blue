import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'

import TypelessCard from '@/components/typeless/TypelessCard.vue'

function createTestI18n() {
  return createI18n({
    legacy: false,
    locale: 'en-US',
    fallbackLocale: 'en-US',
    messages: {
      'en-US': {
        tools: {
          names: {
            advisor: 'Advisor',
          },
        },
        advisorCard: {
          pending: 'Advisor in progress',
          recommendation: 'Recommendation',
          rationale: 'Why',
          winner: 'Winner',
          topCandidates: 'Top candidates',
          weightedCriteria: 'Weighted criteria',
          tradeoffs: 'Tradeoffs',
          risks: 'Risks',
          bestPractices: 'Best practices',
          alternatives: 'Alternatives',
          confidence: 'Confidence',
          evidence: 'Evidence',
          progress: 'Progress',
          jobId: 'Job ID',
          secondOpinion: 'Second opinion',
        },
      },
    },
  })
}

async function settleCard() {
  await flushPromises()
  await vi.dynamicImportSettled()
  await flushPromises()
}

describe('TypelessCard advisor integration', () => {
  it('loads advisor cards through the generic typeless wrapper', async () => {
    const wrapper = mount(TypelessCard, {
      props: {
        card: {
          type: 'advisor',
          id: 'advisor-wrapper-1',
          recommendation: 'Prefer OnlyOffice for collaborative editing in the browser.',
          confidence: 0.78,
          winner: 'OnlyOffice',
          candidates: [
            { name: 'OnlyOffice', rank: 1, total_score: 88 },
            { name: 'LibreOffice', rank: 2, total_score: 71 },
          ],
          weights: [{ criterion: 'fitness', label: 'Fitness', weight: 0.25 }],
          tradeoffs: ['Desktop compatibility is weaker than LibreOffice.'],
          risks: ['Self-hosting adds one more service to operate.'],
          evidence_count: 1,
        } as any,
      },
      global: {
        plugins: [createTestI18n()],
      },
    })

    await settleCard()

    expect(wrapper.find('#advisor-wrapper-1').exists()).toBe(true)
    expect(wrapper.text()).toContain('Advisor')
    expect(wrapper.text()).toContain('Recommendation')
    expect(wrapper.text()).toContain('OnlyOffice')
    expect(wrapper.text()).toContain('Weighted criteria')
    expect(wrapper.text()).toContain('Tradeoffs')
    expect(wrapper.text()).toContain('Risks')
  })
})
