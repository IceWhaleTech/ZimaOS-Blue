import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { createPinia } from 'pinia'

import CardDeepResearch from '@/components/typeless/CardDeepResearch.vue'
import CardDeepResearchProgress from '@/components/typeless/CardDeepResearchProgress.vue'

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
          deepResearchStopReason: 'Stop reason',
          deepResearchLatestAction: 'Latest action',
          deepResearchLatestGap: 'Latest gap',
          deepResearchTrace: 'Research trace',
          deepResearchTraceEntries: 'Iterations',
          deepResearchIteration: 'Iteration',
          deepResearchEvidenceAdded: 'Evidence added',
          deepResearchVerificationSummary: 'Verification',
          deepResearchVerificationResolved: 'Resolved',
          deepResearchVerificationConflicted: 'Conflicted',
          deepResearchVerificationInsufficient: 'Insufficient',
          deepResearchStopReasonCoverage: 'Coverage target reached',
          deepResearchStopReasonNoNewEvidence: 'No new canonical evidence found',
          deepResearchStopReasonBudget: 'Research budget exhausted',
          deepResearchActionAugmentQuery: 'Augmenting query',
          deepResearchActionInitialRetrieve: 'Running initial retrieval',
          deepResearchActionFollowupRetrieve: 'Running follow-up retrieval',
          deepResearchActionVerification: 'Verifying evidence',
          deepResearchActionVerificationCompleted: 'Verification completed',
          deepResearchActionFollowupPlanned: 'Follow-up planned',
          deepResearchActionLoopStopped: 'Research loop stopped',
          deepResearchActionSynthesizing: 'Synthesizing report',
          deepResearchActionCompleted: 'Completed',
          deepResearchProgress: 'Deep Research Running',
          deepResearchStageIntake: 'Intake',
          deepResearchStagePlanning: 'Planning',
          deepResearchStageRetrieve: 'Retrieving',
          deepResearchStageVerify: 'Verifying',
          deepResearchStageSynthesize: 'Synthesizing',
          deepResearchStageCompleted: 'Completed',
          deepResearchStageFailed: 'Failed',
          deepResearchStageCancelled: 'Cancelled',
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
          deepResearchOpenQuestions: 'Open questions',
          deepResearchCitations: 'Citations',
          deepResearchViewTask: 'View task',
          deepResearchCancelTask: 'Cancel',
          deepResearchRunningElsewhere: 'Track active deep research jobs across conversations.',
        },
      },
    },
  })
}

describe('Deep research cards', () => {
  it('renders citations, verification summary, and research trace details', () => {
    const wrapper = mount(CardDeepResearch, {
      props: {
        card: {
          type: 'deep-research',
          query: 'topic',
          mode: 'standard',
          answer: 'Answer',
          evidence_count: 2,
          support_count: 1,
          conflict_count: 1,
          citation_coverage: 0.88,
          confidence: 0.72,
          status: 'completed',
          iterations: 2,
          stop_reason: 'coverage_sufficient',
          citations: [
            { title: 'Official filing', url: 'https://example.com/filing' },
          ],
          open_questions: ['Need official confirmation on one metric'],
          verification_summary: {
            resolved_count: 1,
            conflicted_count: 1,
            insufficient_count: 0,
            items: [
              {
                focus: 'Claim validation',
                gap: 'Resolve conflicting claims',
                status: 'conflicted',
                summary: 'Conflicting reports require primary-source verification.',
              },
            ],
          },
          research_trace: [
            {
              iteration: 1,
              focus: 'Claim validation',
              gap: 'Resolve conflicting claims',
              follow_up_query: 'topic official statement primary source',
              evidence_added: 2,
              verification_outcome: 'conflicted',
            },
          ],
        },
      },
      global: {
        plugins: [createTestI18n()],
      },
    })

    expect(wrapper.text()).toContain('Deep Research')
    expect(wrapper.text()).toContain('Official filing')
    expect(wrapper.text()).toContain('Need official confirmation on one metric')
    expect(wrapper.text()).toContain('Resolve conflicting claims')
    expect(wrapper.text()).toContain('topic official statement primary source')
    expect(wrapper.text()).toContain('Citation Coverage 88%')
  })



  it('renders knowledge-base artifacts when present', () => {
    const wrapper = mount(CardDeepResearch, {
      props: {
        card: {
          type: 'deep-research',
          query: 'build a knowledge base',
          mode: 'standard',
          report_style: 'knowledge_base',
          answer: `## Executive Summary\n- Key finding`,
          evidence_count: 3,
          support_count: 2,
          conflict_count: 0,
          citation_coverage: 0.9,
          confidence: 0.8,
          status: 'completed',
          workflow_phases: [
            { id: 'scope', label: 'scope', status: 'completed' },
            { id: 'audit', label: 'audit', status: 'current' },
          ],
          coverage_summary: {
            task_count: 4,
            evidence_count: 3,
            distinct_domain_count: 2,
            open_question_count: 1,
          },
          object_map: [
            {
              id: 'overview',
              label: 'Overview',
              task_count: 2,
              questions: ['What is the product?', 'Who is it for?'],
              status_counts: { completed: 1, pending: 1 },
            },
          ],
          source_inventory: [
            {
              source_id: 'src-1',
              title: 'Official docs',
              url: 'https://example.com/docs',
              domain: 'example.com',
              source_type: 'web',
              fetched_at: '2026-03-10T10:00:00Z',
              relevance_score: 0.9,
              credibility_score: 0.95,
            },
          ],
        },
      },
      global: {
        plugins: [createTestI18n()],
      },
    })

    expect(wrapper.text()).toContain('Workflow phases')
    expect(wrapper.text()).toContain('Coverage Summary')
    expect(wrapper.text()).toContain('Object Map')
    expect(wrapper.text()).toContain('Overview')
    expect(wrapper.text()).toContain('Official docs')
    expect(wrapper.text()).toContain('scope · Completed')
  })

  it('renders verify-stage progress with buttons', () => {
    const wrapper = mount(CardDeepResearchProgress, {
      props: {
        card: {
          type: 'deep-research-progress',
          job_id: 'job-1',
          conversation_id: 'conv-1',
          query: 'topic',
          mode: 'deep',
          stage: 'verify',
          status: 'running',
          progress: 68,
          iteration: 2,
          latest_action: 'verification_completed',
          latest_gap: 'Need official sources',
        },
      },
      global: {
        plugins: [createTestI18n(), createPinia()],
      },
    })

    expect(wrapper.text()).toContain('Deep Research Running')
    expect(wrapper.text()).toContain('Verifying')
    expect(wrapper.text()).toContain('Iteration 2')
    expect(wrapper.text()).toContain('Verification completed')
    expect(wrapper.text()).toContain('Need official sources')
    expect(wrapper.text()).toContain('View task')
    expect(wrapper.text()).toContain('Cancel')
  })
})
