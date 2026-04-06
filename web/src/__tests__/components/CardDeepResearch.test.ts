import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { createPinia } from 'pinia'

import CardDeepResearch from '@/components/typeless/CardDeepResearch.vue'
import CardDeepResearchProgress from '@/components/typeless/CardDeepResearchProgress.vue'
import zhCN from '@/i18n/locales/zh-CN'
import { mergeHarnessLocale } from '@/i18n/harness-locale-additions'

function createTestI18n(locale = 'en-US') {
  return createI18n({
    legacy: false,
    locale,
    fallbackLocale: 'en-US',
    messages: {
      'en-US': {
        common: {
          loading: 'Loading...',
        },
        chat: {
          researchTitle: 'Research',
          researchProgress: 'Research Running',
          researchRunningElsewhere: 'Track active research tasks across conversations.',
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
          deepResearchDetails: 'Research details',
          deepResearchTrace: 'Research trace',
          deepResearchTraceEntries: 'Iterations',
          deepResearchIteration: 'Iteration',
          deepResearchEvidenceAdded: 'Evidence added',
          deepResearchVerificationSummary: 'Verification',
          deepResearchVerificationResolved: 'Resolved',
          deepResearchVerificationConflicted: 'Conflicted',
          deepResearchVerificationInsufficient: 'Insufficient',
          deepResearchGapNeedEvidenceCoverage: 'Need evidence coverage',
          deepResearchGapNeedPrimaryOrOfficialSources: 'Need primary or official sources',
          deepResearchGapNeedBroaderEvidenceCoverage: 'Need broader evidence coverage',
          deepResearchGapNeedBroaderSourceDiversity: 'Need broader source diversity',
          deepResearchGapNeedFresherSources: 'Need fresher sources',
          deepResearchGapResolveConflictingClaims: 'Resolve conflicting claims',
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
          deepResearchPlannedTasks: 'Planned tasks',
          deepResearchLiveSources: 'Live sources',
          deepResearchExpandDetails: 'Expand research details',
          deepResearchCollapseDetails: 'Collapse research details',
          deepResearchProgress: 'Deep Research in progress',
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
          deepResearchRetainedSearches: 'Retained web searches',
          deepResearchReportStyleKnowledgeBase: 'Knowledge base',
          deepResearchOpenQuestions: 'Open questions',
          deepResearchCitations: 'Citations',
          deepResearchViewTask: 'View task',
          deepResearchCancelTask: 'Cancel',
          deepResearchRunningElsewhere: 'Track active Deep Research tasks across conversations.',
        },
      },
      'zh-CN': {
        chat: {
          deepResearchWorkflowCompleted: '已完成',
          deepResearchWorkflowCurrent: '当前',
          deepResearchWorkflowPending: '待处理',
          deepResearchSourceTypeLaw: '法规',
          deepResearchSourceTypeFiling: '备案文件',
          deepResearchSourceTypePaper: '论文',
          deepResearchSourceTypeWeb: '网页',
          deepResearchMetaOfficial: '官方',
          deepResearchMetaFinancial: '财务',
          deepResearchWorkflowScope: '范围',
          deepResearchWorkflowSources: '来源',
          deepResearchWorkflowExtraction: '提取',
          deepResearchGapNeedEvidenceCoverage: '需要补充证据',
          deepResearchGapNeedPrimaryOrOfficialSources: '需要一手或官方来源',
          deepResearchGapNeedBroaderEvidenceCoverage: '需要更广泛的证据覆盖',
          deepResearchGapNeedBroaderSourceDiversity: '需要更广泛的来源多样性',
          deepResearchGapNeedFresherSources: '需要更新的来源',
          deepResearchGapResolveConflictingClaims: '需要解决冲突说法',
          deepResearchRetainedSearches: '保留的网页搜索',
          deepResearchReportStyleKnowledgeBase: '知识库',
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

describe('Deep research cards', () => {
  it('keeps heavy research details collapsed until expanded', async () => {
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
          citations: [{ title: 'Official filing', url: 'https://example.com/filing' }],
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
          source_inventory: [
            {
              source_id: 'src-1',
              title: 'Official filing',
              url: 'https://example.com/filing',
              domain: 'example.com',
              source_type: 'filing',
              fetched_at: '2026-03-10T10:00:00Z',
              relevance_score: 0.97,
              credibility_score: 0.95,
            },
          ],
        },
      },
      global: {
        plugins: [createTestI18n()],
      },
    })

    expect(wrapper.text()).toContain('Research')
    expect(
      wrapper.get('[data-testid="deep-research-summary-toggle"]').attributes('aria-expanded')
    ).toBe('false')
    expect(wrapper.text()).toContain('Answer')
    expect(wrapper.text()).not.toContain('Official filing')
    expect(wrapper.text()).not.toContain('Research details')

    await wrapper.get('[data-testid="deep-research-summary-toggle"]').trigger('click')

    expect(wrapper.text()).toContain('Official filing')
    expect(wrapper.text()).toContain('Need official confirmation on one metric')
    expect(wrapper.text()).toContain('Resolve conflicting claims')
    expect(wrapper.text()).toContain('topic official statement primary source')
    expect(wrapper.text()).toContain('Citation Coverage 88%')
    expect(wrapper.text()).toContain('Source Inventory')
    expect(wrapper.text()).toContain('Research details')
  })

  it('renders knowledge-base artifacts when present after expansion', async () => {
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
          calibration: {
            coverage: 0.82,
            groundedness: 0.91,
            freshness: 0.68,
            conflict_risk: 'medium',
            confidence: 0.79,
            recommended_action: 'caution',
            takeaway_candidates: [
              {
                lesson:
                  'Keep the final conclusion explicitly cautious when coverage stays below 80%.',
                evidence: 'Citation coverage stayed near the caution threshold.',
              },
            ],
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

    expect(
      wrapper.get('[data-testid="deep-research-summary-toggle"]').attributes('aria-expanded')
    ).toBe('false')
    expect(wrapper.text()).not.toContain('Workflow phases')

    await wrapper.get('[data-testid="deep-research-summary-toggle"]').trigger('click')

    expect(wrapper.text()).toContain('Knowledge base')
    expect(wrapper.text()).not.toContain('knowledge_base')
    expect(wrapper.text()).toContain('Workflow phases')
    expect(wrapper.text()).toContain('Coverage Summary')
    expect(wrapper.text()).toContain('Calibration')
    expect(wrapper.text()).toContain('Use caution')
    expect(wrapper.text()).toContain('Takeaway candidates 1')
    expect(wrapper.text()).toContain('Object Map')
    expect(wrapper.text()).toContain('Overview')
    expect(wrapper.text()).toContain('Official docs')
    expect(wrapper.text()).toContain('Scope · Completed')
  })

  it('renders retained search cards when present after expansion', async () => {
    const wrapper = mount(CardDeepResearch, {
      props: {
        uiStateKey: 'deep-research:test-retained-searches',
        card: {
          type: 'deep-research',
          query: 'topic',
          mode: 'standard',
          answer: 'Answer',
          evidence_count: 2,
          support_count: 2,
          conflict_count: 0,
          citation_coverage: 0.75,
          confidence: 0.7,
          status: 'completed',
          search_cards: [
            {
              type: 'search',
              id: 'retained-search-1',
              query: 'topic overview official',
              totalCount: 2,
              results: [
                {
                  title: 'Official docs',
                  url: 'https://example.com/docs',
                  description: 'official documentation',
                },
                {
                  title: 'Release notes',
                  url: 'https://example.com/release',
                  description: 'recent updates',
                },
              ],
            },
          ],
        },
      },
      global: {
        plugins: [createTestI18n()],
      },
    })

    expect(wrapper.text()).not.toContain('Retained web searches')
    expect(wrapper.text()).not.toContain('Official docs')

    await wrapper.get('[data-testid="deep-research-summary-toggle"]').trigger('click')

    expect(wrapper.text()).toContain('Retained web searches')
    expect(wrapper.text()).not.toContain('knowledge_base')
    expect(wrapper.text()).toContain('topic overview official')
    expect(wrapper.text()).toContain('Official docs')
    expect(wrapper.text()).toContain('Release notes')
  })

  it('localizes known gap text for zh-CN cards', async () => {
    const wrapper = mount(CardDeepResearch, {
      props: {
        uiStateKey: 'deep-research:test-zh-gap',
        card: {
          type: 'deep-research',
          query: 'topic zh gap',
          mode: 'standard',
          answer: 'Answer',
          status: 'completed',
          verification_summary: {
            resolved_count: 0,
            conflicted_count: 0,
            insufficient_count: 1,
            items: [
              {
                focus: '概览',
                gap: 'Need evidence coverage',
                status: 'insufficient',
              },
            ],
          },
          research_trace: [
            {
              iteration: 1,
              focus: '概览',
              gap: 'Need evidence coverage',
              verification_outcome: 'insufficient',
            },
          ],
        },
      },
      global: {
        plugins: [createTestI18n('zh-CN')],
      },
    })

    await wrapper.get('[data-testid="deep-research-summary-toggle"]').trigger('click')

    expect(wrapper.text()).toContain('需要补充证据')
    expect(wrapper.text()).not.toContain('Need evidence coverage')
  })

  it('localizes structured workflow and source tokens for zh-CN cards', async () => {
    const wrapper = mount(CardDeepResearch, {
      props: {
        uiStateKey: 'deep-research:test-zh-structured-values',
        card: {
          type: 'deep-research',
          query: 'topic zh structured values',
          mode: 'standard',
          report_style: 'knowledge_base',
          answer: 'Answer',
          status: 'completed',
          workflow_phases: [
            { id: 'scope', label: 'Scope', status: 'completed' },
            { id: 'sources', label: 'sources', status: 'current' },
            { id: 'extraction', label: 'Extraction', status: 'pending' },
          ],
          source_inventory: [
            {
              source_id: 'src-law',
              title: '法规来源',
              url: 'https://example.com/law',
              domain: 'example.com',
              source_type: 'law',
            },
          ],
        },
      },
      global: {
        plugins: [createTestI18n('zh-CN')],
      },
    })

    await wrapper.get('[data-testid="deep-research-summary-toggle"]').trigger('click')

    expect(wrapper.text()).toContain('范围 · 已完成')
    expect(wrapper.text()).toContain('来源 · 当前')
    expect(wrapper.text()).toContain('提取 · 待处理')
    expect(wrapper.text()).toContain('example.com · 法规')
    expect(wrapper.text()).not.toContain('Scope ·')
    expect(wrapper.text()).not.toContain('sources ·')
    expect(wrapper.text()).not.toContain('Extraction ·')
    expect(wrapper.text()).not.toContain('example.com · law')
  })

  it('localizes object map labels for zh-CN runtime locales instead of exposing english tokens', async () => {
    const wrapper = mount(CardDeepResearch, {
      props: {
        uiStateKey: 'deep-research:test-zh-runtime-object-map',
        card: {
          type: 'deep-research',
          query: 'topic zh object map',
          mode: 'standard',
          report_style: 'knowledge_base',
          answer: 'Answer',
          status: 'completed',
          object_map: [
            {
              id: 'overview',
              label: 'Overview',
              task_count: 2,
              status_counts: { completed: 1, pending: 1 },
              questions: ['What is the product?'],
            },
          ],
        },
      },
      global: {
        plugins: [createRuntimeLocaleI18n('zh-CN')],
      },
    })

    await wrapper.get('[data-testid="deep-research-summary-toggle"]').trigger('click')

    expect(wrapper.text()).toContain('对象地图')
    expect(wrapper.text()).toContain('概览')
    expect(wrapper.text()).not.toContain('Overview')
  })

  it('localizes runtime deep research axis, focus, and time window tokens for zh-CN cards', async () => {
    const wrapper = mount(CardDeepResearch, {
      props: {
        uiStateKey: 'deep-research:test-zh-runtime-token-labels',
        card: {
          type: 'deep-research',
          query: 'topic zh runtime tokens',
          mode: 'standard',
          report_style: 'knowledge_base',
          answer: 'Answer',
          status: 'completed',
          time_windows: ['recent'],
          object_map: [
            {
              id: 'latest',
              label: 'Latest',
              task_count: 1,
              time_windows: ['earlier', '2025-2026'],
              status_counts: { completed: 1 },
              questions: ['What changed in this period?'],
            },
          ],
          research_trace: [
            {
              iteration: 1,
              focus: 'Claim validation',
              verification_outcome: 'current',
            },
          ],
        },
      },
      global: {
        plugins: [createRuntimeLocaleI18n('zh-CN')],
      },
    })

    await wrapper.get('[data-testid="deep-research-summary-toggle"]').trigger('click')

    expect(wrapper.text()).toContain('最新动态')
    expect(wrapper.text()).toContain('结论核验')
    expect(wrapper.text()).toContain('早期')
    expect(wrapper.text()).toContain('近期')
    expect(wrapper.text()).toContain('2025-2026')
    expect(wrapper.text()).not.toContain('Latest')
    expect(wrapper.text()).not.toContain('Claim validation')
    expect(wrapper.text()).not.toContain('earlier')
    expect(wrapper.text()).not.toContain('recent')
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

    expect(wrapper.text()).toContain('Deep Research in progress')
    expect(wrapper.text()).toContain('Verifying')
    expect(wrapper.text()).toContain('Iteration 2')
    expect(wrapper.text()).toContain('Verification completed')
    expect(wrapper.text()).toContain('Need primary or official sources')
    expect(wrapper.text()).toContain('View task')
    expect(wrapper.text()).toContain('Cancel')
  })
})
