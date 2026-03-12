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
        common: {
          loading: 'Loading...',
        },
        chat: {
          waitingThinking: 'Thinking...',
          deepResearchTitle: 'Deep Research',
          deepResearchEvidence: 'Evidence',
          deepResearchSupport: 'Support',
          deepResearchConflict: 'Conflict',
          deepResearchHasConflict: 'Conflicting signals',
          deepResearchCitationCoverage: 'Citation Coverage',
          deepResearchStatus: 'Status',
          deepResearchIterations: 'Iterations',
          deepResearchVerificationSummary: 'Verification',
          deepResearchVerificationResolved: 'Resolved',
          deepResearchVerificationConflicted: 'Conflicted',
          deepResearchVerificationInsufficient: 'Insufficient',
          deepResearchOpenQuestions: 'Open questions',
          deepResearchCitations: 'Citations',
          deepResearchTimeline: 'Timeline',
          deepResearchWorkflowPhases: 'Workflow phases',
          deepResearchWorkflowCompleted: 'Completed',
          deepResearchWorkflowCurrent: 'Current',
          deepResearchWorkflowPending: 'Pending',
          deepResearchSourceInventory: 'Source Inventory',
          deepResearchPublishedAt: 'Published',
          deepResearchFetchedAt: 'Fetched',
          deepResearchRelevance: 'Rel',
          deepResearchCredibility: 'Cred',
          deepResearchCoverageSummary: 'Coverage Summary',
          deepResearchTasks: 'Tasks',
          deepResearchDomains: 'Domains',
          deepResearchObjectMap: 'Object Map',
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

describe('TypelessCard deep research integration', () => {
  it('loads knowledge-base deep research cards through the generic typeless wrapper', async () => {
    const wrapper = mount(TypelessCard, {
      props: {
        card: {
          type: 'deep-research',
          id: 'deep-research-kb-wrapper',
          query: 'European AI Act compliance map',
          mode: 'deep',
          report_style: 'knowledge_base',
          answer: 'Compliance obligations are grouped by object, phase, and evidence source.',
          evidence_count: 5,
          support_count: 4,
          conflict_count: 1,
          citation_coverage: 0.92,
          confidence: 0.81,
          status: 'completed',
          iterations: 4,
          citations: [
            {
              title: 'EU AI Act consolidated text',
              url: 'https://eur-lex.europa.eu/eli/reg/2024/1689/oj',
            },
          ],
          open_questions: ['Need delegated acts publication date'],
          verification_summary: {
            resolved_count: 4,
            conflicted_count: 1,
            insufficient_count: 0,
          },
          workflow_phases: [
            { id: 'scope', label: 'Scope', status: 'completed' },
            { id: 'sources', label: 'Sources', status: 'current' },
          ],
          source_inventory: [
            {
              source_id: 'src-1',
              title: 'EU AI Act consolidated text',
              url: 'https://eur-lex.europa.eu/eli/reg/2024/1689/oj',
              domain: 'eur-lex.europa.eu',
              source_type: 'law',
              fetched_at: '2026-03-08T00:00:00.000Z',
              relevance_score: 0.97,
              credibility_score: 0.99,
            },
          ],
          coverage_summary: {
            task_count: 6,
            evidence_count: 5,
            distinct_domain_count: 2,
            open_question_count: 1,
          },
          object_map: [
            {
              id: 'provider-obligations',
              label: 'Provider obligations',
              task_count: 3,
              time_windows: ['2025-2026'],
              status_counts: { resolved: 2, conflicted: 1 },
              questions: ['Which GPAI duties apply to open-weight models?'],
            },
          ],
        },
      },
      global: {
        plugins: [createTestI18n()],
      },
    })

    await settleCard()

    expect(wrapper.find('#deep-research-kb-wrapper').exists()).toBe(true)
    expect(wrapper.text()).toContain('European AI Act compliance map')
    expect(wrapper.text()).toContain('Workflow phases')
    expect(wrapper.text()).toContain('Scope')
    expect(wrapper.text()).toContain('Source Inventory')
    expect(wrapper.text()).toContain('EU AI Act consolidated text')
    expect(wrapper.text()).toContain('Coverage Summary')
    expect(wrapper.text()).toContain('Object Map')
    expect(wrapper.text()).toContain('Provider obligations')
  })
})
