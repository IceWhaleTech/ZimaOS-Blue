import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'

import { skillApi } from '@/api/skill'
import { harnessApi } from '@/api/harness'
import { selfReflectApi } from '@/api/selfReflect'
import { mergeHarnessLocale } from '@/i18n/harness-locale-additions'
import zhCN from '@/i18n/locales/zh-CN'

let mockSettingsStore: Record<string, unknown>

const routerPushMock = vi.fn()
const routerReplaceMock = vi.fn()
const routeMock = {
  query: {} as Record<string, string>,
}

vi.mock('@/api/skill', () => ({
  skillApi: {
    list: vi.fn(),
    getContent: vi.fn(),
  },
}))

vi.mock('@/api/harness', () => ({
  harnessApi: {
    listSkillRevisions: vi.fn(),
    listSkillDecisionHistory: vi.fn(),
    listSkillEvolutionCases: vi.fn(),
    getSkillEvolutionCase: vi.fn(),
    getEvalRunReport: vi.fn(),
    promoteSkillRevision: vi.fn(),
    rollbackSkillRevision: vi.fn(),
    optimizeSkill: vi.fn(),
  },
}))

vi.mock('@/api/selfReflect', () => ({
  selfReflectApi: {
    listProposals: vi.fn(),
    getProposal: vi.fn(),
    getPatchPreview: vi.fn(),
    approveProposal: vi.fn(),
    rejectProposal: vi.fn(),
  },
}))

vi.mock('@/components/harness/AgentcoreRunnerPanel.vue', () => ({
  default: {
    name: 'AgentcoreRunnerPanel',
    template: '<div data-testid="agentcore-runner-panel-stub">Runner panel</div>',
  },
}))

vi.mock('@/components/automation/AutomationTabs.vue', () => ({
  default: {
    name: 'AutomationTabs',
    template: '<div data-testid="automation-tabs-stub">Automation tabs</div>',
  },
}))

vi.mock('@/stores/notification', () => ({
  useNotificationStore: () => ({
    success: vi.fn(),
    error: vi.fn(),
    info: vi.fn(),
  }),
}))

vi.mock('@/stores/settings', () => ({
  useSettingsStore: () => mockSettingsStore,
}))

vi.mock('vue-router', () => ({
  useRoute: () => routeMock,
  useRouter: () => ({
    push: routerPushMock,
    replace: routerReplaceMock,
  }),
}))

function createTestI18n(locale: 'en-US' | 'zh-CN' = 'en-US') {
  if (locale === 'zh-CN') {
    return createI18n({
      legacy: false,
      locale,
      fallbackLocale: locale,
      missingWarn: false,
      fallbackWarn: false,
      messages: {
        'zh-CN': mergeHarnessLocale('zh-CN', zhCN as Record<string, unknown>),
      },
    })
  }

  return createI18n({
    legacy: false,
    locale,
    fallbackLocale: locale,
    missingWarn: false,
    fallbackWarn: false,
    messages: {
      'en-US': {
        common: {
          loading: 'Loading',
          error: 'Error',
          refresh: 'Refresh',
          notAvailable: 'Not available',
        },
      },
    },
  })
}

function setConfirmResult(result: boolean) {
  const confirmMock = vi.fn(() => result)
  Object.defineProperty(window, 'confirm', {
    value: confirmMock,
    configurable: true,
    writable: true,
  })
  return confirmMock
}

describe('EvolutionView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    routerPushMock.mockResolvedValue(undefined)
    routerReplaceMock.mockImplementation(async ({ query }: { query?: Record<string, unknown> }) => {
      routeMock.query = Object.fromEntries(
        Object.entries(query || {}).map(([key, value]) => [key, String(value ?? '')])
      )
    })
    routeMock.query = {}
    mockSettingsStore = {
      experimentalAgentcoreRunnerEnabled: true,
      agentcoreRunnerStatus: {
        enabled: true,
        toolchain_ready: true,
        binary_ready: true,
        last_optimization_run_id: 'opt-runner-1',
        last_optimization_state: 'completed',
        last_optimization_summary: 'Follow-up selector gate accepted the skill candidate.',
        supported_parts: ['skill_definition', 'prompt_template', 'runner_code'],
        optimized_parts: ['skill_definition', 'prompt_template'],
        primary_part: 'skill_definition',
        source_eval_run_id: 'eval-runner-source',
      },
      agentcoreRunnerStatusLoading: false,
      agentcoreRunnerStatusError: null,
      agentcoreRunnerLastRun: {
        id: 'opt-runner-1',
        created_at: '2026-04-05T09:00:00Z',
        reason: 'runtime_failure',
        candidate_id: 'candidate-browser-runner',
        eval_run_id: 'eval-runner-source',
        source_eval_run_id: 'eval-runner-source',
        base_eval_run_id: 'eval-runner-baseline',
        optimization_surface: 'skill_definition',
        optimized_parts: ['skill_definition', 'prompt_template'],
        primary_part: 'skill_definition',
        runner_protocol: 'acp',
        runner_stop_reason: 'completed',
        runner_response_text: '{"status":"candidate_ready"}',
        runner_duration_ms: 2420,
        runner_transcript: [
          {
            direction: 'outbound',
            method: 'session/prompt',
            text: 'Optimization trigger received.',
          },
          {
            direction: 'inbound',
            method: 'session/update',
            text: '{"status":"candidate_ready","message":"browser recovery guidance improved"}',
          },
        ],
        followup_gate: 'selector',
        followup_state: 'accepted',
        followup_message: 'Candidate accepted by selector gate.',
        followup_eval_run_id: 'eval-runner-followup',
        skill_evolution_case_id: 'case-browser-1',
        skill_revision_id: 'rev-accepted',
        materialized_skill_candidate: {
          skill_id: 'browser',
          candidate_id: 'candidate-browser-runner',
          source_path: 'assets/skills/browser/SKILL.md',
          content:
            '# Browser\n\nUse the browser carefully and recover after navigation errors.\n',
          sha256: 'sha-runner-candidate',
        },
      },
      agentcoreRunnerLastRunLoading: false,
      agentcoreRunnerLastRunError: null,
      fetchAgentcoreRunnerStatus: vi.fn().mockResolvedValue({
        enabled: true,
      }),
      fetchAgentcoreRunnerLastRun: vi.fn().mockResolvedValue({
        id: 'opt-runner-1',
      }),
    }

    vi.mocked(skillApi.list).mockResolvedValue({
      data: [
        {
          id: 'browser',
          name: 'Browser',
          version: '1.0.0',
          description: 'Browser automation skill',
          builtin: true,
          enabled: true,
        },
        {
          id: 'self_reflect',
          name: 'Self Reflect',
          version: '1.0.0',
          description: 'Reflection skill',
          builtin: true,
          enabled: true,
        },
        {
          id: 'workspace-note',
          name: 'Workspace Note',
          version: '1.0.0',
          description: 'Local workspace skill',
          builtin: false,
          enabled: true,
        },
      ],
    } as never)

    vi.mocked(skillApi.getContent).mockResolvedValue({
      data: {
        id: 'browser',
        content: '# Browser\n\nUse the browser carefully.\n',
      },
    } as never)

    vi.mocked(harnessApi.listSkillRevisions).mockResolvedValue({
      data: [
        {
          id: 'rev-accepted',
          skill_id: 'browser',
          status: 'accepted',
          source_path: 'assets/skills/browser/SKILL.md',
          candidate_id: 'candidate-browser-1',
          base_content_sha256: 'sha-browser-base',
          origin_case_id: 'case-browser-1',
          eval_run_id: 'eval-browser-1',
          followup_gate: 'execution_gate',
          optimization_surface: 'skill_definition',
          content: '# Browser\n\nUse the browser carefully and recover after navigation errors.\n',
          content_sha256: 'sha-browser-next',
          created_at: '2026-04-05T08:00:00Z',
        },
        {
          id: 'rev-promoted',
          skill_id: 'browser',
          status: 'promoted',
          source_path: 'assets/skills/browser/SKILL.md',
          candidate_id: 'candidate-browser-current',
          decision_action: 'promote',
          review_note: 'Promoted after explicit operator sign-off.',
          reviewed_by: 'user-promote-handler',
          reviewed_at: '2026-04-03T08:10:00Z',
          decision_log_json:
            '{"action":"promote","review_note":"Promoted after explicit operator sign-off.","reviewed_by":"user-promote-handler","reviewed_at":"2026-04-03T08:10:00Z","selected_revision_id":"rev-promoted","target_revision_id":"rev-promoted","backup_revision_id":"rev-backup","written_source_path":"/Users/orca/Documents/GitHub/ZimaOS-Blue/assets/skills/browser/SKILL.md"}',
          content: '# Browser\n\nUse the browser carefully.\n',
          content_sha256: 'sha-browser-current',
          created_at: '2026-04-03T08:00:00Z',
        },
        {
          id: 'rev-backup',
          skill_id: 'browser',
          status: 'backup',
          source_path: 'assets/skills/browser/SKILL.md',
          backup_of_revision_id: 'rev-promoted',
          content: '# Browser\n\nUse the browser carefully.\n',
          content_sha256: 'sha-browser-backup',
          created_at: '2026-04-04T08:00:00Z',
        },
        {
          id: 'rev-capture',
          skill_id: 'browser',
          status: 'candidate',
          source_path: 'assets/skills/browser/SKILL.md',
          candidate_id: 'candidate-browser-capture',
          origin_case_id: 'case-browser-capture',
          content: '# Browser\n\nUse the browser carefully and include successful recovery examples.\n',
          content_sha256: 'sha-browser-capture',
          created_at: '2026-04-02T08:00:00Z',
        },
        {
          id: 'rev-runtime',
          skill_id: 'browser',
          status: 'candidate',
          source_path: 'assets/skills/browser/SKILL.md',
          candidate_id: 'candidate-browser-runtime',
          origin_case_id: 'case-browser-runtime',
          content:
            '# Browser\n\nUse the browser carefully and validate navigation output before summarizing.\n',
          content_sha256: 'sha-browser-runtime',
          created_at: '2026-04-01T08:00:00Z',
        },
      ],
    } as never)
    vi.mocked(harnessApi.listSkillDecisionHistory).mockResolvedValue({
      data: [
        {
          revision_id: 'rev-promoted',
          skill_id: 'browser',
          status: 'promoted',
          source_path: 'assets/skills/browser/SKILL.md',
          candidate_id: 'candidate-browser-current',
          decision_action: 'promote',
          review_note: 'Promoted after explicit operator sign-off.',
          reviewed_by: 'user-promote-handler',
          decision_log_json:
            '{"action":"promote","review_note":"Promoted after explicit operator sign-off.","reviewed_by":"user-promote-handler","reviewed_at":"2026-04-03T08:10:00Z","selected_revision_id":"rev-promoted","target_revision_id":"rev-promoted","backup_revision_id":"rev-backup","written_source_path":"/Users/orca/Documents/GitHub/ZimaOS-Blue/assets/skills/browser/SKILL.md"}',
          eval_run_id: 'eval-browser-1',
          decision_at: '2026-04-03T08:10:00Z',
          reviewed_at: '2026-04-03T08:10:00Z',
          created_at: '2026-04-03T08:00:00Z',
          promoted_at: '2026-04-03T08:10:00Z',
        },
      ],
    } as never)

    vi.mocked(harnessApi.listSkillEvolutionCases).mockResolvedValue({
      data: [
        {
          id: 'case-browser-capture',
          skill_id: 'browser',
          mode: 'capture',
          reason: 'runtime_capture',
          source_kind: 'eval_run',
          source_id: 'eval-browser-capture',
          candidate_id: 'candidate-browser-capture',
          base_content_sha256: 'sha-browser-base',
          summary: 'Recovered browser sessions revealed a reusable guidance pattern worth capturing.',
          evidence_json: JSON.stringify({
            lesson: 'Add a recovery example for browser sessions that resume successfully after navigation issues.',
            when_to_apply: 'When the browser task succeeds only after the agent retries and restores state.',
            confidence: 0.86,
            groundedness: 0.91,
            evidence_ids: ['ev-cap-1', 'ev-cap-2'],
          }),
          revision_id: 'rev-capture',
          status: 'candidate_created',
          created_at: '2026-04-02T08:00:00Z',
          updated_at: '2026-04-02T08:10:00Z',
        },
        {
          id: 'case-browser-1',
          skill_id: 'browser',
          mode: 'fix',
          reason: 'runtime_failure',
          source_kind: 'eval_run',
          source_id: 'eval-browser-1',
          candidate_id: 'candidate-browser-1',
          base_content_sha256: 'sha-browser-base',
          summary: 'Navigation recovery guidance was missing for repeated browser failures.',
          evidence_json: JSON.stringify({
            failure_signature: 'browser.navigation_timeout',
            validation: {
              recovered: true,
              failure_count: 3,
            },
          }),
          revision_id: 'rev-accepted',
          status: 'accepted',
          created_at: '2026-04-05T08:00:00Z',
          updated_at: '2026-04-05T08:10:00Z',
        },
        {
          id: 'case-browser-runtime',
          skill_id: 'browser',
          mode: 'fix',
          reason: 'runtime_failure',
          source_kind: 'runtime_run',
          source_id: 'run-browser-runtime',
          candidate_id: 'candidate-browser-runtime',
          base_content_sha256: 'sha-browser-base',
          summary: 'A direct runtime browser failure generated a repair candidate before any eval group existed.',
          evidence_json: JSON.stringify({
            runtime_run_id: 'run-browser-runtime',
            runtime_status: 'failed',
            runtime_state: 'execute',
            runtime_metrics: {
              duration_ms: 3400,
              event_count: 7,
            },
            runtime_usage: {
              input_tokens: 320,
              output_tokens: 220,
              total_tokens: 540,
            },
            runtime_quality: {
              verification_passed: false,
              failure_label: 'missing_title',
              outcome_score: 0.42,
            },
          }),
          revision_id: 'rev-runtime',
          status: 'candidate_created',
          created_at: '2026-04-01T08:00:00Z',
          updated_at: '2026-04-01T08:10:00Z',
        },
      ],
    } as never)
    vi.mocked(harnessApi.getSkillEvolutionCase).mockImplementation(async (caseID: string) => {
      if (caseID === 'case-browser-runtime') {
        return {
          data: {
            id: 'case-browser-runtime',
            skill_id: 'browser',
            mode: 'fix',
            reason: 'runtime_failure',
            source_kind: 'runtime_run',
            source_id: 'run-browser-runtime',
            source_run_id: 'run-browser-runtime',
            source_run: {
              id: 'run-browser-runtime',
              root_run_id: 'run-browser-runtime',
              kind: 'agent_task',
              status: 'failed',
              runtime_state: 'execute',
              goal: 'Investigate browser regression',
              result: 'Browser task produced partial output before failure.',
              error: 'navigation timed out',
              created_at: '2026-04-01T08:00:00Z',
              updated_at: '2026-04-01T08:10:00Z',
              started_at: '2026-04-01T08:00:01Z',
              finished_at: '2026-04-01T08:00:04Z',
            },
            candidate_id: 'candidate-browser-runtime',
            base_content_sha256: 'sha-browser-base',
            summary:
              'A direct runtime browser failure generated a repair candidate before any eval group existed.',
            evidence_json: JSON.stringify({
              runtime_run_id: 'run-browser-runtime',
              runtime_status: 'failed',
              runtime_state: 'execute',
              runtime_metrics: {
                duration_ms: 3400,
                event_count: 7,
              },
              runtime_usage: {
                input_tokens: 320,
                output_tokens: 220,
                total_tokens: 540,
              },
              runtime_quality: {
                verification_passed: false,
                failure_label: 'missing_title',
                outcome_score: 0.42,
              },
            }),
            revision_id: 'rev-runtime',
            linked_revision: {
              id: 'rev-runtime',
              skill_id: 'browser',
              status: 'candidate',
              source_path: 'assets/skills/browser/SKILL.md',
              candidate_id: 'candidate-browser-runtime',
              origin_case_id: 'case-browser-runtime',
              content:
                '# Browser\n\nUse the browser carefully and validate navigation output before summarizing.\n',
              content_sha256: 'sha-browser-runtime',
              created_at: '2026-04-01T08:00:00Z',
            },
            status: 'candidate_created',
            created_at: '2026-04-01T08:00:00Z',
            updated_at: '2026-04-01T08:10:00Z',
          },
        } as never
      }
      if (caseID === 'case-browser-capture') {
        return {
          data: {
            id: 'case-browser-capture',
            skill_id: 'browser',
            mode: 'capture',
            reason: 'runtime_capture',
            source_kind: 'eval_run',
            source_id: 'eval-browser-capture',
            source_eval_run_id: 'eval-browser-capture',
            candidate_id: 'candidate-browser-capture',
            base_content_sha256: 'sha-browser-base',
            summary:
              'Recovered browser sessions revealed a reusable guidance pattern worth capturing.',
            evidence_json: JSON.stringify({
              lesson:
                'Add a recovery example for browser sessions that resume successfully after navigation issues.',
            }),
            revision_id: 'rev-capture',
            linked_revision: {
              id: 'rev-capture',
              skill_id: 'browser',
              status: 'candidate',
              source_path: 'assets/skills/browser/SKILL.md',
              candidate_id: 'candidate-browser-capture',
              origin_case_id: 'case-browser-capture',
              eval_run_id: 'eval-browser-capture',
              content:
                '# Browser\n\nUse the browser carefully and include successful recovery examples.\n',
              content_sha256: 'sha-browser-capture',
              created_at: '2026-04-02T08:00:00Z',
            },
            linked_eval_run_id: 'eval-browser-capture',
            source_eval_run: {
              id: 'eval-browser-capture',
              eval_spec_id: 'spec-browser',
              group_id: 'group-browser-capture',
              status: 'completed',
              created_at: '2026-04-02T08:05:00Z',
              updated_at: '2026-04-02T08:08:00Z',
            },
            linked_eval_run: {
              id: 'eval-browser-capture',
              eval_spec_id: 'spec-browser',
              group_id: 'group-browser-capture',
              status: 'completed',
              created_at: '2026-04-02T08:05:00Z',
              updated_at: '2026-04-02T08:08:00Z',
            },
            status: 'candidate_created',
            created_at: '2026-04-02T08:00:00Z',
            updated_at: '2026-04-02T08:10:00Z',
          },
        } as never
      }
      return {
        data: {
          id: 'case-browser-1',
          skill_id: 'browser',
          mode: 'fix',
          reason: 'runtime_failure',
          source_kind: 'eval_run',
          source_id: 'eval-browser-1',
          source_eval_run_id: 'eval-browser-1',
          candidate_id: 'candidate-browser-1',
          base_content_sha256: 'sha-browser-base',
          summary: 'Navigation recovery guidance was missing for repeated browser failures.',
          evidence_json: JSON.stringify({
            failure_signature: 'browser.navigation_timeout',
          }),
          revision_id: 'rev-accepted',
          linked_revision: {
            id: 'rev-accepted',
            skill_id: 'browser',
            status: 'accepted',
            source_path: 'assets/skills/browser/SKILL.md',
            candidate_id: 'candidate-browser-1',
            base_content_sha256: 'sha-browser-base',
            origin_case_id: 'case-browser-1',
            eval_run_id: 'eval-browser-1',
            followup_gate: 'execution_gate',
            optimization_surface: 'skill_definition',
            content:
              '# Browser\n\nUse the browser carefully and recover after navigation errors.\n',
            content_sha256: 'sha-browser-next',
            created_at: '2026-04-05T08:00:00Z',
          },
          linked_eval_run_id: 'eval-browser-1',
          source_eval_run: {
            id: 'eval-browser-1',
            eval_spec_id: 'spec-browser',
            group_id: 'group-browser',
            status: 'completed',
            created_at: '2026-04-05T08:05:00Z',
            updated_at: '2026-04-05T08:08:00Z',
          },
          linked_eval_run: {
            id: 'eval-browser-1',
            eval_spec_id: 'spec-browser',
            group_id: 'group-browser',
            status: 'completed',
            created_at: '2026-04-05T08:05:00Z',
            updated_at: '2026-04-05T08:08:00Z',
          },
          status: 'accepted',
          created_at: '2026-04-05T08:00:00Z',
          updated_at: '2026-04-05T08:10:00Z',
        },
      } as never
    })

    vi.mocked(harnessApi.getEvalRunReport).mockImplementation(async (evalRunID: string) => {
      const reportByID: Record<string, unknown> = {
        'eval-browser-1': {
          eval_run: {
            id: 'eval-browser-1',
            eval_spec_id: 'spec-browser',
            group_id: 'group-browser',
            baseline_eval_run_id: 'eval-browser-baseline',
            status: 'completed',
            summary: {
              overall_score: 0.93,
              pass_rate: 1,
              overall_score_delta: 0.07,
              pass_rate_delta: 0.15,
              verification_pass_rate_delta: 0.12,
              evidence_backed_pass_rate_delta: 0.18,
              total_tokens: 4200,
              avg_duration_ms: 1850,
            },
            created_at: '2026-04-05T08:05:00Z',
            updated_at: '2026-04-05T08:08:00Z',
          },
          group_report: {
            group: {
              id: 'group-browser',
              kind: 'eval',
              status: 'completed',
              created_at: '2026-04-05T08:05:00Z',
              updated_at: '2026-04-05T08:08:00Z',
            },
            overall_score: 0.93,
            pass_rate: 1,
          },
        },
        'eval-runner-source': {
          eval_run: {
            id: 'eval-runner-source',
            eval_spec_id: 'spec-runner-source',
            group_id: 'group-runner-source',
            baseline_eval_run_id: 'eval-runner-baseline',
            status: 'completed',
            summary: {
              overall_score: 0.84,
              pass_rate: 0.8,
              verification_pass_rate: 0.78,
              total_tokens: 2100,
              avg_duration_ms: 1640,
            },
            created_at: '2026-04-05T09:00:00Z',
            updated_at: '2026-04-05T09:04:00Z',
          },
          group_report: {
            group: {
              id: 'group-runner-source',
              kind: 'eval',
              status: 'completed',
              created_at: '2026-04-05T09:00:00Z',
              updated_at: '2026-04-05T09:04:00Z',
            },
            overall_score: 0.84,
            pass_rate: 0.8,
          },
        },
        'eval-runner-followup': {
          eval_run: {
            id: 'eval-runner-followup',
            eval_spec_id: 'spec-runner-followup',
            group_id: 'group-runner-followup',
            baseline_eval_run_id: 'eval-runner-baseline',
            status: 'completed',
            summary: {
              overall_score: 0.96,
              pass_rate: 1,
              verification_pass_rate: 1,
              evidence_backed_pass_rate: 0.94,
              total_tokens: 2600,
              avg_duration_ms: 1930,
            },
            created_at: '2026-04-05T09:05:00Z',
            updated_at: '2026-04-05T09:09:00Z',
          },
          group_report: {
            group: {
              id: 'group-runner-followup',
              kind: 'eval',
              status: 'completed',
              created_at: '2026-04-05T09:05:00Z',
              updated_at: '2026-04-05T09:09:00Z',
            },
            overall_score: 0.96,
            pass_rate: 1,
          },
        },
      }

      return {
        data: (reportByID[evalRunID] || reportByID['eval-browser-1']) as never,
      } as never
    })

    vi.mocked(harnessApi.promoteSkillRevision).mockResolvedValue({
      data: {
        promoted_revision_id: 'rev-accepted',
        backup_revision_id: 'rev-backup-new',
        written_source_path: 'assets/skills/browser/SKILL.md',
      },
    } as never)
    vi.mocked(harnessApi.rollbackSkillRevision).mockResolvedValue({
      data: {
        promoted_revision_id: 'rev-rollback-new',
        backup_revision_id: 'rev-backup-after-rollback',
        written_source_path: 'assets/skills/browser/SKILL.md',
      },
    } as never)
    vi.mocked(harnessApi.optimizeSkill).mockResolvedValue({
      data: {
        reason: 'manual_skill_optimize',
        eval_run_id: 'eval-browser-1',
        optimization_surface: 'skill_definition',
        metadata: {
          followup_gate: 'execution_gate',
        },
      },
    } as never)

    const proposal = {
      id: 'proposal-1',
      source_kind: 'harness_group',
      source_id: 'group-1',
      target_file: 'AGENTS.md',
      status: 'pending',
      lesson: 'Keep skill evolution review separated from general harness dashboards.',
      when_to_apply: 'When adding productized review surfaces for runtime-generated changes.',
      evidence: 'Users asked for a dedicated evolution lane and explicit AGENTS approval.',
      evidence_ids: ['ev-1', 'ev-2'],
      patch_preview: '@@ AGENTS.md @@\n+ Add a dedicated evolution review checkpoint.\n',
      created_at: '2026-04-05T08:00:00Z',
      updated_at: '2026-04-05T08:05:00Z',
    }

    const proposalApproved = {
      id: 'proposal-2',
      source_kind: 'eval_run',
      source_id: 'eval-browser-1',
      target_file: 'docs/OPERATIONS.md',
      status: 'approved',
      lesson: 'Capture rollout notes once the new evolution console actions are verified.',
      when_to_apply: 'When promoting or rolling back a live version changes the operator workflow.',
      evidence: 'Operators need documented rollout steps after the console gains more direct controls.',
      evidence_ids: ['ev-ops-1'],
      patch_preview: '@@ docs/OPERATIONS.md @@\n+ Document the evolution rollout checklist.\n',
      created_at: '2026-04-04T08:00:00Z',
      updated_at: '2026-04-04T08:05:00Z',
    }

    const proposalsByID = {
      [proposal.id]: proposal,
      [proposalApproved.id]: proposalApproved,
    }

    vi.mocked(selfReflectApi.listProposals).mockResolvedValue({
      data: [proposal, proposalApproved],
    } as never)
    vi.mocked(selfReflectApi.getProposal).mockImplementation((id: string) =>
      Promise.resolve({
        data: proposalsByID[id as keyof typeof proposalsByID],
      } as never)
    )
    vi.mocked(selfReflectApi.getPatchPreview).mockImplementation((id: string) =>
      Promise.resolve({
        data: {
          id,
          target_file: proposalsByID[id as keyof typeof proposalsByID]?.target_file,
          patch_preview: proposalsByID[id as keyof typeof proposalsByID]?.patch_preview,
        },
      } as never)
    )
    vi.mocked(selfReflectApi.approveProposal).mockResolvedValue({
      data: {
        ...proposal,
        status: 'approved',
      },
    } as never)
    vi.mocked(selfReflectApi.rejectProposal).mockResolvedValue({
      data: {
        ...proposal,
        status: 'rejected',
      },
    } as never)
  })

  async function mountView(locale: 'en-US' | 'zh-CN' = 'en-US') {
    const EvolutionView = (await import('@/views/EvolutionView.vue')).default
    const wrapper = mount(EvolutionView, {
      global: {
        plugins: [createTestI18n(locale)],
      },
    })
    await flushPromises()
    return wrapper
  }

  it('localizes evolution enum-driven labels for zh-CN', async () => {
    const wrapper = await mountView('zh-CN')

    expect(wrapper.get('[data-testid="evolution-skill-case-case-browser-1"]').text()).toContain(
      '修复'
    )
    expect(wrapper.get('[data-testid="evolution-skill-case-case-browser-1"]').text()).toContain(
      '运行失败'
    )
    expect(wrapper.get('[data-testid="evolution-skill-case-case-browser-1"]').text()).toContain(
      '已接受'
    )
    expect(wrapper.get('[data-testid="evolution-skill-revision-rev-accepted"]').text()).toContain(
      '技能定义'
    )
    expect(wrapper.get('[data-testid="evolution-skill-decision-live"]').text()).toContain(
      '当前线上版本'
    )
    expect(wrapper.get('[data-testid="evolution-skill-diff-summary-additions"]').text()).toContain(
      '新增行'
    )

    await wrapper.get('[data-testid="evolution-tab-runner"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="evolution-runner-summary-parts"]').text()).toContain(
      '2 个部分已变更'
    )
    expect(wrapper.get('[data-testid="evolution-runner-summary-parts"]').text()).toContain(
      '技能定义'
    )

    await wrapper.get('[data-testid="evolution-tab-instructions"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="evolution-health-instructions-review_queue"]').text()).toContain(
      '1 条提案待审'
    )
    expect(wrapper.text()).toContain('待处理')
    expect(wrapper.text()).toContain('Harness 分组')
  })

  it('renders skills, runner, and instructions lanes and shows skill evidence with patch metrics', async () => {
    const wrapper = await mountView()

    expect(wrapper.text()).toContain('Skills')
    expect(wrapper.text()).toContain('Runner')
    expect(wrapper.text()).toContain('Review Queue')
    expect(wrapper.get('[data-testid="evolution-beta-badge"]').text()).toContain('Beta')
    expect(wrapper.text()).toContain('Browser')
    expect(wrapper.text()).not.toContain('Workspace Note')
    expect(wrapper.text()).toContain('Navigation recovery guidance was missing')
    expect(
      wrapper.get('[data-testid="evolution-skill-case-status-summary-case-browser-1"]').text()
    ).toContain('Accepted, waiting promote')
    expect(
      wrapper.get('[data-testid="evolution-skill-case-status-summary-case-browser-capture"]').text()
    ).toContain('Candidate created, waiting gate')
    expect(
      wrapper.get('[data-testid="evolution-skill-case-chip-case-browser-1-candidate_id"]').text()
    ).toContain('candidate-browser-1')
    expect(
      wrapper.get('[data-testid="evolution-skill-case-chip-case-browser-1-source_id"]').text()
    ).toContain('eval-browser-1')
    expect(
      wrapper.get('[data-testid="evolution-skill-case-chip-case-browser-1-revision_id"]').text()
    ).toContain('rev-accepted')
    expect(wrapper.get('[data-testid="evolution-skill-diff"]').text()).toContain(
      'recover after navigation errors'
    )
    expect(wrapper.get('[data-testid="evolution-skill-diff-summary-additions"]').text()).toContain(
      '1'
    )
    expect(wrapper.get('[data-testid="evolution-skill-diff-summary-deletions"]').text()).toContain(
      '1'
    )
    expect(
      wrapper.get('[data-testid="evolution-skill-evidence-summary-failure_signature"]').text()
    ).toContain('browser.navigation_timeout')
    expect(
      wrapper.get('[data-testid="evolution-skill-evidence-summary-recovered"]').text()
    ).toContain('Yes')
    expect(
      wrapper.get('[data-testid="evolution-skill-evidence-summary-failure_count"]').text()
    ).toContain('3')
    expect(wrapper.get('[data-testid="evolution-skill-evidence-review-why_changed"]').text()).toContain(
      'Repair missing guidance'
    )
    expect(wrapper.get('[data-testid="evolution-skill-evidence-review-strength"]').text()).toContain(
      'Runtime failure recovered'
    )
    expect(wrapper.get('[data-testid="evolution-skill-diff-review-areas"]').text()).toContain(
      'Recovery guidance'
    )
    expect(wrapper.get('[data-testid="evolution-skill-diff-review-risk"]').text()).toContain(
      'Low review risk'
    )
    expect(wrapper.get('[data-testid="evolution-skill-promote-readiness-gate"]').text()).toContain(
      'Accepted by gate'
    )
    expect(
      wrapper.get('[data-testid="evolution-skill-promote-readiness-safety"]').text()
    ).toContain('Base SHA recorded')
    expect(wrapper.get('[data-testid="evolution-skill-switch-preview-current"]').text()).toContain(
      'rev-promoted'
    )
    expect(wrapper.get('[data-testid="evolution-skill-switch-preview-candidate"]').text()).toContain(
      'rev-accepted'
    )
    expect(wrapper.get('[data-testid="evolution-skill-switch-preview-preserve"]').text()).toContain(
      'Current live preserved as backup'
    )
    expect(wrapper.get('[data-testid="evolution-skill-selected-badges"]').text()).toContain(
      'Ready to promote'
    )
    expect(
      wrapper.get('[data-testid="evolution-skill-revision-badge-rev-promoted-current"]').text()
    ).toContain('Current canonical')
    expect(wrapper.get('[data-testid="evolution-skill-case-timeline-open"]').text()).toContain(
      'Case opened'
    )
    expect(
      wrapper.get('[data-testid="evolution-skill-case-timeline-candidate_created"]').text()
    ).toContain('Candidate created')
    expect(
      wrapper.get('[data-testid="evolution-skill-case-timeline-accepted"]').text()
    ).toContain('Accepted by gate')
    expect(
      wrapper.get('[data-testid="evolution-skill-case-timeline-promoted"]').text()
    ).toContain('Promoted to canonical')
    expect(wrapper.text()).toContain('Version Comparison')
    expect(wrapper.get('[data-testid="evolution-skill-baseline-id"]').text()).toContain(
      'eval-browser-baseline'
    )
    expect(
      wrapper.get('[data-testid="evolution-skill-delta-overall_score_delta"]').text()
    ).toContain('+0.07')
    expect(wrapper.get('[data-testid="evolution-skill-delta-pass_rate_delta"]').text()).toContain(
      '+15%'
    )
    expect(wrapper.text()).toContain('Overall score')
    expect(wrapper.text()).toContain('Total tokens')
    expect(wrapper.text()).toContain('4,200')
    expect(wrapper.get('[data-testid="evolution-skill-decision-live"]').text()).toContain(
      'rev-promoted'
    )
    expect(wrapper.get('[data-testid="evolution-skill-decision-selected"]').text()).toContain(
      'Better candidate'
    )
    expect(wrapper.get('[data-testid="evolution-skill-decision-next_step"]').text()).toContain(
      'Promote to canonical'
    )
    expect(wrapper.get('[data-testid="evolution-skill-decision-next_step"]').text()).toContain(
      'Score delta +0.07'
    )
  })

  it('loads selected case detail and exposes direct revision and eval actions in the evidence panel', async () => {
    const wrapper = await mountView()

    expect(harnessApi.getSkillEvolutionCase).toHaveBeenCalledWith('case-browser-1')
    expect(
      wrapper.get('[data-testid="evolution-skill-selected-case-open-revision"]').text()
    ).toContain('Open linked revision')
    expect(
      wrapper.get('[data-testid="evolution-skill-selected-case-open-source"]').text()
    ).toContain('Open source eval run')
    expect(
      wrapper.get('[data-testid="evolution-skill-selected-case-open-linked-eval"]').text()
    ).toContain('Open linked eval run')
    expect(wrapper.text()).toContain('Source eval run')
    expect(wrapper.text()).toContain('Linked eval run')

    await wrapper.get('[data-testid="evolution-skill-selected-case-open-source"]').trigger('click')
    await flushPromises()

    expect(routerPushMock).toHaveBeenCalledWith({
      name: 'HarnessGroups',
      query: { evalRunId: 'eval-browser-1' },
    })
  })

  it('requires confirmation before promoting an accepted skill revision and still supports AGENTS instructions approval', async () => {
    const wrapper = await mountView()
    const confirmSpy = setConfirmResult(true)

    await wrapper.get('[data-testid="evolution-skill-promote"]').trigger('click')
    await flushPromises()

    expect(confirmSpy).toHaveBeenCalledTimes(1)
    expect(harnessApi.promoteSkillRevision).toHaveBeenCalledWith('rev-accepted')

    await wrapper.get('[data-testid="evolution-tab-instructions"]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('AGENTS.md')
    expect(wrapper.get('[data-testid="evolution-instructions-patch"]').text()).toContain(
      'evolution review checkpoint'
    )
    expect(
      wrapper.get('[data-testid="evolution-instructions-patch-summary-additions"]').text()
    ).toContain('1')

    await wrapper.get('[data-testid="evolution-instructions-approve"]').trigger('click')
    await flushPromises()

    expect(confirmSpy).toHaveBeenCalledTimes(2)
    expect(selfReflectApi.approveProposal).toHaveBeenCalledWith('proposal-1', '')
  })

  it('renders instruction review details in a more compact layout with smaller summary typography', async () => {
    const wrapper = await mountView()

    await wrapper.get('[data-testid="evolution-tab-instructions"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="evolution-instructions-review-title"]').text()).toContain(
      'AGENTS.md'
    )
    expect(
      wrapper.get('[data-testid="evolution-instructions-review-title"]').classes()
    ).toContain('text-lg')
    expect(
      wrapper.get('[data-testid="evolution-instructions-review-title"]').classes()
    ).not.toContain('text-xl')

    expect(wrapper.get('[data-testid="evolution-instructions-review-meta"]').text()).toContain(
      'Pending'
    )
    expect(wrapper.get('[data-testid="evolution-instructions-review-meta"]').text()).toContain(
      '2026'
    )
    expect(
      wrapper.get('[data-testid="evolution-instructions-evidence-compact"]').text()
    ).toContain('When To Apply')
    expect(
      wrapper.get('[data-testid="evolution-instructions-evidence-compact"]').text()
    ).toContain('Evidence')
    expect(wrapper.get('[data-testid="evolution-instructions-meta-compact"]').text()).toContain(
      'Source kind'
    )
    expect(wrapper.get('[data-testid="evolution-instructions-meta-compact"]').text()).toContain(
      'Evidence IDs'
    )

    expect(
      wrapper.get('[data-testid="evolution-instructions-patch-summary-value-additions"]').classes()
    ).toContain('text-base')
    expect(
      wrapper.get('[data-testid="evolution-instructions-patch-summary-value-additions"]').classes()
    ).not.toContain('text-xl')
  })

  it('compacts lower skill and runner secondary summaries after the primary review cards', async () => {
    const wrapper = await mountView()

    expect(wrapper.get('[data-testid="evolution-skill-case-meta-compact"]').text()).toContain(
      'Source kind'
    )
    expect(wrapper.get('[data-testid="evolution-skill-case-meta-compact"]').text()).toContain(
      'Source eval run'
    )
    expect(
      wrapper.get('[data-testid="evolution-skill-evidence-summary-value-failure_count"]').classes()
    ).toContain('text-base')
    expect(
      wrapper.get('[data-testid="evolution-skill-evidence-summary-value-failure_count"]').classes()
    ).not.toContain('text-xl')
    expect(
      wrapper.get('[data-testid="evolution-skill-diff-summary-value-additions"]').classes()
    ).toContain('text-base')
    expect(
      wrapper.get('[data-testid="evolution-skill-diff-summary-value-additions"]').classes()
    ).not.toContain('text-xl')

    await wrapper.get('[data-testid="evolution-tab-runner"]').trigger('click')
    await flushPromises()

    expect(
      wrapper.get('[data-testid="evolution-runner-diff-summary-value-additions"]').classes()
    ).toContain('text-base')
    expect(
      wrapper.get('[data-testid="evolution-runner-diff-summary-value-additions"]').classes()
    ).not.toContain('text-xl')
    expect(
      wrapper.get('[data-testid="evolution-runner-diff-summary-value-additions"]').classes()
    ).not.toContain('sm:text-xl')
  })

  it('keeps the page header and top overview cards compact after the lower review sections were tightened', async () => {
    const wrapper = await mountView()

    expect(wrapper.get('[data-testid="evolution-page-title"]').text()).toContain(
      'Self-Repair And Evolution Console'
    )
    expect(wrapper.get('[data-testid="evolution-page-title"]').classes()).toContain('text-lg')
    expect(wrapper.get('[data-testid="evolution-page-title"]').classes()).not.toContain(
      'sm:text-2xl'
    )

    expect(wrapper.get('[data-testid="evolution-summary-visible-skills"]').classes()).toContain(
      'text-lg'
    )
    expect(
      wrapper.get('[data-testid="evolution-summary-visible-skills"]').classes()
    ).not.toContain('sm:text-2xl')
    expect(wrapper.get('[data-testid="evolution-summary-hint-skills"]').classes()).toContain(
      'text-[11px]'
    )
    expect(wrapper.get('[data-testid="evolution-summary-hint-skills"]').classes()).toContain(
      'leading-4'
    )
  })

  it('compacts lane switch cards and pane health details after the overview cards were tightened', async () => {
    const wrapper = await mountView()

    expect(wrapper.get('[data-testid="evolution-lane-description-skills"]').classes()).toContain(
      'text-xs'
    )
    expect(wrapper.get('[data-testid="evolution-lane-description-skills"]').classes()).toContain(
      'line-clamp-2'
    )
    expect(wrapper.get('[data-testid="evolution-lane-supporting-skills"]').text()).toContain(
      'Visible skills'
    )
    expect(wrapper.get('[data-testid="evolution-lane-supporting-skills"]').classes()).toContain(
      'text-[11px]'
    )
    expect(
      wrapper.get('[data-testid="evolution-health-skills-better_version-details"]').classes()
    ).toContain('line-clamp-2')
    expect(
      wrapper.get('[data-testid="evolution-health-skills-better_version-details"]').classes()
    ).toContain('text-[11px]')

    await wrapper.get('[data-testid="evolution-tab-instructions"]').trigger('click')
    await flushPromises()

    expect(
      wrapper.get('[data-testid="evolution-health-instructions-review_queue-details"]').classes()
    ).toContain('line-clamp-2')
  })

  it('keeps skill list cards and runner transcript entries compact after the header and health cards were tightened', async () => {
    const wrapper = await mountView()

    expect(wrapper.get('[data-testid="evolution-skill-item-title-browser"]').classes()).toContain(
      'text-xs'
    )
    expect(
      wrapper.get('[data-testid="evolution-skill-item-description-browser"]').classes()
    ).toContain('line-clamp-1')
    expect(
      wrapper.get('[data-testid="evolution-skill-item-description-browser"]').classes()
    ).toContain('leading-4')

    await wrapper.get('[data-testid="evolution-tab-runner"]').trigger('click')
    await flushPromises()

    expect(
      wrapper.get('[data-testid="evolution-runner-transcript-entry-0"]').classes()
    ).toContain('px-3')
    expect(
      wrapper.get('[data-testid="evolution-runner-transcript-text-0"]').classes()
    ).toContain('text-xs')
    expect(
      wrapper.get('[data-testid="evolution-runner-transcript-text-0"]').classes()
    ).toContain('leading-5')
  })

  it('keeps skill alerts and the selected-skill header compact after the list cards were tightened', async () => {
    const wrapper = await mountView()

    expect(wrapper.get('[data-testid="evolution-skill-selected-title"]').classes()).toContain(
      'text-lg'
    )
    expect(
      wrapper.get('[data-testid="evolution-skill-selected-title"]').classes()
    ).not.toContain('text-xl')
    expect(
      wrapper.get('[data-testid="evolution-skill-selected-description"]').classes()
    ).toContain('leading-5')
    expect(wrapper.get('[data-testid="evolution-skill-safe-rollback-details"]').classes()).toContain(
      'text-xs'
    )

    await wrapper.get('[data-testid="evolution-skill-search"]').setValue('self reflect')
    await flushPromises()

    expect(
      wrapper.get('[data-testid="evolution-skill-hidden-selection-details"]').classes()
    ).toContain('text-xs')
    expect(
      wrapper.get('[data-testid="evolution-skill-hidden-selection-details"]').classes()
    ).toContain('leading-4')

    await wrapper.get('[data-testid="evolution-skill-case-mode-filter"]').setValue('capture')
    await flushPromises()

    expect(
      wrapper.get('[data-testid="evolution-skill-case-hidden-selection-details"]').classes()
    ).toContain('text-xs')

    await wrapper.get('[data-testid="evolution-skill-revision-status-filter"]').setValue('backup')
    await flushPromises()

    expect(
      wrapper.get('[data-testid="evolution-skill-revision-hidden-selection-details"]').classes()
    ).toContain('text-xs')
  })

  it('keeps case and revision list cards compact after the skill alerts were tightened', async () => {
    const wrapper = await mountView()

    expect(wrapper.get('[data-testid="evolution-skill-case-case-browser-1"]').classes()).toContain(
      'px-3'
    )
    expect(wrapper.get('[data-testid="evolution-skill-case-case-browser-1"]').classes()).toContain(
      'py-2.5'
    )
    expect(
      wrapper.get('[data-testid="evolution-skill-case-summary-case-browser-1"]').classes()
    ).toContain('text-[11px]')
    expect(
      wrapper.get('[data-testid="evolution-skill-case-summary-case-browser-1"]').classes()
    ).toContain('leading-4')

    expect(
      wrapper.get('[data-testid="evolution-skill-revision-rev-accepted"]').classes()
    ).toContain('px-3')
    expect(
      wrapper.get('[data-testid="evolution-skill-revision-rev-accepted"]').classes()
    ).toContain('py-2.5')
    expect(
      wrapper.get('[data-testid="evolution-skill-revision-title-rev-accepted"]').classes()
    ).toContain('text-[11px]')
    expect(
      wrapper.get('[data-testid="evolution-skill-revision-title-rev-accepted"]').classes()
    ).toContain('leading-4')
  })

  it('further tightens case and revision card body copy after the base compact layout', async () => {
    const wrapper = await mountView()

    expect(wrapper.get('[data-testid="evolution-skill-case-case-browser-1"]').classes()).toContain(
      'py-2.5'
    )
    expect(
      wrapper.get('[data-testid="evolution-skill-case-summary-case-browser-1"]').classes()
    ).toContain('text-[11px]')
    expect(
      wrapper.get('[data-testid="evolution-skill-case-summary-case-browser-1"]').classes()
    ).toContain('leading-4')
    expect(
      wrapper.get('[data-testid="evolution-skill-case-summary-case-browser-1"]').classes()
    ).toContain('line-clamp-2')
    expect(
      wrapper.get('[data-testid="evolution-skill-case-updated-case-browser-1"]').classes()
    ).toContain('text-[11px]')

    expect(
      wrapper.get('[data-testid="evolution-skill-revision-rev-accepted"]').classes()
    ).toContain('py-2.5')
    expect(
      wrapper.get('[data-testid="evolution-skill-revision-title-rev-accepted"]').classes()
    ).toContain('text-[11px]')
    expect(
      wrapper.get('[data-testid="evolution-skill-revision-title-rev-accepted"]').classes()
    ).toContain('leading-4')
    expect(
      wrapper.get('[data-testid="evolution-skill-revision-created-rev-accepted"]').classes()
    ).toContain('text-[11px]')
  })

  it('keeps lifecycle and decision-history rows compact after the list cards were tightened', async () => {
    const wrapper = await mountView()

    expect(wrapper.get('[data-testid="evolution-skill-case-timeline-open"]').classes()).toContain(
      'px-3'
    )
    expect(wrapper.get('[data-testid="evolution-skill-case-timeline-open"]').classes()).toContain(
      'py-3'
    )
    expect(
      wrapper.get('[data-testid="evolution-skill-case-timeline-label-open"]').classes()
    ).toContain('text-xs')

    await wrapper.get('[data-testid="evolution-skill-revision-rev-promoted"]').trigger('click')
    await flushPromises()

    expect(
      wrapper.get('[data-testid="evolution-skill-decision-timeline-card-rev-promoted"]').classes()
    ).toContain('px-3')
    expect(
      wrapper.get('[data-testid="evolution-skill-decision-timeline-summary-rev-promoted"]').classes()
    ).toContain('text-xs')
    expect(
      wrapper.get('[data-testid="evolution-skill-decision-history-entry-rev-promoted"]').classes()
    ).toContain('p-3')
    expect(
      wrapper.get('[data-testid="evolution-skill-decision-history-rev-promoted-title"]').classes()
    ).toContain('text-sm')
    expect(
      wrapper.get('[data-testid="evolution-skill-decision-history-rev-promoted-summary"]').classes()
    ).toContain('text-xs')
  })

  it('keeps lineage and linked-eval snapshot panels compact after the timeline rows were tightened', async () => {
    const wrapper = await mountView()

    expect(wrapper.get('[data-testid="evolution-skill-lineage-entry-0"]').classes()).toContain(
      'px-3'
    )
    expect(wrapper.get('[data-testid="evolution-skill-lineage-entry-0"]').classes()).toContain(
      'py-3'
    )
    expect(wrapper.get('[data-testid="evolution-skill-lineage-value-0"]').classes()).toContain(
      'text-xs'
    )

    await wrapper.get('[data-testid="evolution-skill-revision-rev-promoted"]').trigger('click')
    await flushPromises()

    expect(
      wrapper.get('[data-testid="evolution-skill-decision-history-rev-promoted-evidence-panel"]').classes()
    ).toContain('p-3')
    expect(
      wrapper.get('[data-testid="evolution-skill-decision-history-rev-promoted-evidence-hint"]').classes()
    ).toContain('text-xs')
    expect(
      wrapper.get('[data-testid="evolution-skill-decision-history-rev-promoted-evidence-overall_score"]').classes()
    ).toContain('px-3')
    expect(
      wrapper.get('[data-testid="evolution-skill-decision-history-rev-promoted-evidence-overall_score"]').classes()
    ).toContain('py-2.5')
  })

  it('keeps version metadata and runner evidence snapshot cards compact after the lineage pass', async () => {
    const wrapper = await mountView()

    expect(wrapper.get('[data-testid="evolution-skill-version-meta-entry-0"]').classes()).toContain(
      'px-3'
    )
    expect(wrapper.get('[data-testid="evolution-skill-version-meta-entry-0"]').classes()).toContain(
      'py-3'
    )
    expect(wrapper.get('[data-testid="evolution-skill-version-meta-value-0"]').classes()).toContain(
      'text-xs'
    )
    expect(wrapper.get('[data-testid="evolution-skill-version-meta-value-0"]').classes()).toContain(
      'leading-5'
    )

    await wrapper.get('[data-testid="evolution-tab-runner"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="evolution-runner-link-followup-eval"]').classes()).toContain(
      'px-3'
    )
    expect(wrapper.get('[data-testid="evolution-runner-link-followup-eval"]').classes()).toContain(
      'py-3'
    )
    expect(
      wrapper.get('[data-testid="evolution-runner-link-linked-revision"]').classes()
    ).toContain('px-3')
    expect(
      wrapper.get('[data-testid="evolution-runner-report-followup-overall_score"]').classes()
    ).toContain('px-3')
    expect(
      wrapper.get('[data-testid="evolution-runner-report-followup-overall_score"]').classes()
    ).toContain('py-3')
    expect(wrapper.get('[data-testid="evolution-runner-report-source-overall_score"]').classes()).toContain(
      'px-3'
    )
    expect(wrapper.get('[data-testid="evolution-runner-report-source-overall_score"]').classes()).toContain(
      'py-3'
    )
  })

  it('sends operator rationale to promote and rollback endpoints when provided', async () => {
    const wrapper = await mountView()
    const confirmSpy = setConfirmResult(true)

    await wrapper
      .get('[data-testid="evolution-skill-review-note-suggestion-0"]')
      .trigger('click')
    await flushPromises()

    await wrapper.get('[data-testid="evolution-skill-promote"]').trigger('click')
    await flushPromises()

    expect(confirmSpy).toHaveBeenCalledTimes(1)
    expect(harnessApi.promoteSkillRevision).toHaveBeenCalledWith('rev-accepted', {
      review_note: 'Passed gate with grounded evidence and clear operator value.',
    })

    await wrapper.get('[data-testid="evolution-skill-revision-rev-backup"]').trigger('click')
    await flushPromises()

    await wrapper
      .get('[data-testid="evolution-skill-review-note-suggestion-0"]')
      .trigger('click')
    await flushPromises()

    await wrapper.get('[data-testid="evolution-skill-rollback"]').trigger('click')
    await flushPromises()

    expect(confirmSpy).toHaveBeenCalledTimes(2)
    expect(harnessApi.rollbackSkillRevision).toHaveBeenCalledWith('rev-backup', {
      review_note: 'Restore the previous stable canonical behavior while preserving the current live version as backup.',
    })
  })

  it('switches evidence digest when a capture revision is selected', async () => {
    const wrapper = await mountView()

    await wrapper.get('[data-testid="evolution-skill-revision-rev-capture"]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Capture evidence emphasizes the reusable lesson')
    expect(
      wrapper.get('[data-testid="evolution-skill-case-timeline-candidate_created"]').text()
    ).toContain('Candidate created')
    expect(
      wrapper.get('[data-testid="evolution-skill-case-timeline-accepted"]').text()
    ).toContain('Accepted by gate')
    expect(wrapper.get('[data-testid="evolution-skill-evidence-summary-lesson"]').text()).toContain(
      'Add a recovery example'
    )
    expect(
      wrapper.get('[data-testid="evolution-skill-evidence-summary-when_to_apply"]').text()
    ).toContain('When the browser task succeeds only after the agent retries')
    expect(
      wrapper.get('[data-testid="evolution-skill-evidence-summary-confidence"]').text()
    ).toContain('86%')
    expect(
      wrapper.get('[data-testid="evolution-skill-evidence-summary-groundedness"]').text()
    ).toContain('91%')
    expect(
      wrapper.get('[data-testid="evolution-skill-evidence-summary-evidence_ids"]').text()
    ).toContain('2')
    expect(wrapper.get('[data-testid="evolution-skill-evidence-review-why_changed"]').text()).toContain(
      'Capture learned guidance'
    )
    expect(wrapper.get('[data-testid="evolution-skill-diff-review-areas"]').text()).toContain(
      'Examples'
    )
  })

  it('opens the linked revision directly from a case card', async () => {
    const wrapper = await mountView()

    await wrapper
      .get('[data-testid="evolution-skill-case-open-revision-case-browser-capture"]')
      .trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Capture evidence emphasizes the reusable lesson')
    expect(wrapper.find('[data-testid="evolution-skill-selected-badges"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="evolution-skill-evidence-summary-lesson"]').text()).toContain(
      'Add a recovery example'
    )
  })

  it('surfaces runtime-origin evidence metrics for runtime-triggered skill cases', async () => {
    const wrapper = await mountView()

    await wrapper.get('[data-testid="evolution-skill-revision-rev-runtime"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="evolution-skill-evidence-summary-duration_ms"]').text()).toContain(
      '3.4 s'
    )
    expect(wrapper.get('[data-testid="evolution-skill-evidence-summary-total_tokens"]').text()).toContain(
      '540'
    )
    expect(
      wrapper.get('[data-testid="evolution-skill-evidence-summary-verification_passed"]').text()
    ).toContain('No')
    expect(wrapper.get('[data-testid="evolution-skill-evidence-summary-outcome_score"]').text()).toContain(
      '42%'
    )
    expect(wrapper.get('[data-testid="evolution-skill-case-chip-case-browser-runtime-source_id"]').text()).toContain(
      'run-browser-runtime'
    )
  })

  it('opens the linked source eval run directly from a case card', async () => {
    const wrapper = await mountView()

    await wrapper
      .get('[data-testid="evolution-skill-case-open-source-case-browser-1"]')
      .trigger('click')
    await flushPromises()

    expect(routerPushMock).toHaveBeenCalledWith({
      name: 'HarnessGroups',
      query: { evalRunId: 'eval-browser-1' },
    })
  })

  it('opens revision eval evidence, baseline eval evidence, and instruction source groups from the console', async () => {
    const wrapper = await mountView()

    await wrapper.get('[data-testid="evolution-skill-open-eval-run"]').trigger('click')
    await flushPromises()

    expect(routerPushMock).toHaveBeenCalledWith({
      name: 'HarnessGroups',
      query: { evalRunId: 'eval-browser-1' },
    })

    await wrapper.get('[data-testid="evolution-skill-open-baseline-eval-run"]').trigger('click')
    await flushPromises()

    expect(routerPushMock).toHaveBeenCalledWith({
      name: 'HarnessGroups',
      query: { evalRunId: 'eval-browser-baseline' },
    })

    await wrapper.get('[data-testid="evolution-tab-instructions"]').trigger('click')
    await flushPromises()

    await wrapper.get('[data-testid="evolution-instructions-open-source-group"]').trigger('click')
    await flushPromises()

    expect(routerPushMock).toHaveBeenCalledWith({
      name: 'HarnessGroupDetail',
      params: { id: 'group-1' },
    })
  })

  it('supports searching skills and filtering cases and instruction proposals', async () => {
    const wrapper = await mountView()

    await wrapper.get('[data-testid="evolution-skill-search"]').setValue('self reflect')
    await flushPromises()

    expect(wrapper.find('[data-testid="evolution-skill-item-browser"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="evolution-skill-item-self_reflect"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="evolution-summary-visible-skills"]').text()).toBe('1')
    expect(wrapper.get('[data-testid="evolution-summary-total-skills"]').text()).toContain('2')

    await wrapper.get('[data-testid="evolution-skill-search"]').setValue('')
    await wrapper.get('[data-testid="evolution-skill-case-mode-filter"]').setValue('capture')
    await flushPromises()

    expect(wrapper.find('[data-testid="evolution-skill-case-case-browser-1"]').exists()).toBe(
      false
    )
    expect(
      wrapper.find('[data-testid="evolution-skill-case-case-browser-capture"]').exists()
    ).toBe(true)
    expect(wrapper.get('[data-testid="evolution-summary-visible-cases"]').text()).toBe('1')
    expect(wrapper.get('[data-testid="evolution-summary-total-cases"]').text()).toContain('3')

    await wrapper.get('[data-testid="evolution-tab-instructions"]').trigger('click')
    await flushPromises()

    await wrapper.get('[data-testid="evolution-instructions-search"]').setValue('operations')
    await wrapper.get('[data-testid="evolution-instructions-status-filter"]').setValue('approved')
    await flushPromises()

    expect(wrapper.find('[data-testid="evolution-instructions-proposal-proposal-1"]').exists()).toBe(
      false
    )
    expect(wrapper.find('[data-testid="evolution-instructions-proposal-proposal-2"]').exists()).toBe(
      true
    )
    expect(wrapper.get('[data-testid="evolution-summary-visible-instructions"]').text()).toBe('1')
    expect(wrapper.get('[data-testid="evolution-summary-total-instructions"]').text()).toContain(
      '2'
    )
  })

  it('shows pane health badges for version readiness, filters, AGENTS approvals, and runner boundaries', async () => {
    const wrapper = await mountView()

    expect(wrapper.get('[data-testid="evolution-health-skills-better_version"]').text()).toContain(
      'Selected revision ready'
    )
    expect(wrapper.get('[data-testid="evolution-health-skills-rollback"]').text()).toContain(
      'rollback backup ready'
    )
    expect(wrapper.get('[data-testid="evolution-health-skills-visibility"]').text()).toContain(
      'All visible'
    )

    await wrapper.get('[data-testid="evolution-skill-search"]').setValue('self reflect')
    await flushPromises()

    expect(wrapper.get('[data-testid="evolution-health-skills-filters"]').text()).toContain(
      '1 active filter'
    )
    expect(wrapper.get('[data-testid="evolution-health-skills-visibility"]').text()).toContain(
      '1 hidden selection'
    )

    await wrapper.get('[data-testid="evolution-tab-instructions"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="evolution-health-instructions-review_queue"]').text()).toContain(
      '1 pending proposal'
    )
    expect(wrapper.get('[data-testid="evolution-health-instructions-agents"]').text()).toContain(
      '1 approval waiting'
    )
    expect(wrapper.get('[data-testid="evolution-health-instructions-agents"]').text()).toContain(
      'AGENTS.md'
    )

    await wrapper.get('[data-testid="evolution-instructions-status-filter"]').setValue('approved')
    await flushPromises()

    expect(wrapper.get('[data-testid="evolution-health-instructions-filters"]').text()).toContain(
      '1 active filter'
    )
    expect(wrapper.get('[data-testid="evolution-health-instructions-visibility"]').text()).toContain(
      '1 hidden selection'
    )

    await wrapper.get('[data-testid="evolution-tab-runner"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="evolution-health-runner-switch_boundary"]').text()).toContain(
      'Skills handles promote'
    )
    expect(
      wrapper.get('[data-testid="evolution-health-runner-instruction_boundary"]').text()
    ).toContain('AGENTS.md waiting')
  })

  it('lets operators use pane health actions to clear filters, switch versions, and jump to AGENTS review', async () => {
    const wrapper = await mountView()

    await wrapper.get('[data-testid="evolution-skill-revision-rev-backup"]').trigger('click')
    await flushPromises()

    await wrapper
      .get('[data-testid="evolution-health-skills-better_version-action"]')
      .trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="evolution-skill-decision-selected"]').text()).toContain(
      'Better candidate'
    )

    await wrapper.get('[data-testid="evolution-health-skills-rollback-action"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="evolution-skill-decision-selected"]').text()).toContain(
      'Rollback backup'
    )

    await wrapper.get('[data-testid="evolution-skill-search"]').setValue('self reflect')
    await flushPromises()

    await wrapper.get('[data-testid="evolution-health-skills-filters-action"]').trigger('click')
    await flushPromises()

    expect((wrapper.get('[data-testid="evolution-skill-search"]').element as HTMLInputElement).value).toBe(
      ''
    )

    await wrapper.get('[data-testid="evolution-tab-instructions"]').trigger('click')
    await flushPromises()

    await wrapper.get('[data-testid="evolution-instructions-status-filter"]').setValue('approved')
    await flushPromises()

    await wrapper
      .get('[data-testid="evolution-health-instructions-filters-action"]')
      .trigger('click')
    await flushPromises()

    expect(
      (wrapper.get('[data-testid="evolution-instructions-status-filter"]').element as HTMLSelectElement)
        .value
    ).toBe('all')

    await wrapper.get('[data-testid="evolution-instructions-proposal-proposal-2"]').trigger('click')
    await flushPromises()

    await wrapper
      .get('[data-testid="evolution-health-instructions-agents-action"]')
      .trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('AGENTS.md')
    expect(wrapper.get('[data-testid="evolution-instructions-patch"]').text()).toContain(
      'evolution review checkpoint'
    )

    await wrapper.get('[data-testid="evolution-tab-runner"]').trigger('click')
    await flushPromises()

    await wrapper
      .get('[data-testid="evolution-health-runner-switch_boundary-action"]')
      .trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Selected Skill')

    await wrapper.get('[data-testid="evolution-tab-runner"]').trigger('click')
    await flushPromises()

    await wrapper
      .get('[data-testid="evolution-health-runner-instruction_boundary-action"]')
      .trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Review Proposal')
    expect(wrapper.text()).toContain('AGENTS.md')
  })

  it('supports drill-down from decision cards and review shortcuts into comparison, metrics, evidence, and diff sections', async () => {
    const wrapper = await mountView()

    await wrapper.get('[data-testid="evolution-skill-decision-live-action"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="evolution-skill-section-lineage"]').attributes('data-focused')).toBe(
      'true'
    )

    await wrapper.get('[data-testid="evolution-skill-decision-selected-action"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="evolution-skill-section-diff"]').attributes('data-focused')).toBe(
      'true'
    )

    await wrapper.get('[data-testid="evolution-skill-decision-next_step-action"]').trigger('click')
    await flushPromises()

    expect(
      wrapper.get('[data-testid="evolution-skill-section-comparison"]').attributes('data-focused')
    ).toBe('true')

    await wrapper.get('[data-testid="evolution-skill-drilldown-metrics-action"]').trigger('click')
    await flushPromises()

    expect(
      wrapper.get('[data-testid="evolution-skill-section-metrics"]').attributes('data-focused')
    ).toBe('true')

    await wrapper.get('[data-testid="evolution-skill-drilldown-evidence-action"]').trigger('click')
    await flushPromises()

    expect(
      wrapper.get('[data-testid="evolution-skill-section-evidence"]').attributes('data-focused')
    ).toBe('true')
  })

  it('renders a structured evaluation scorecard with quality, runtime, token, and evidence summaries', async () => {
    const wrapper = await mountView()

    expect(wrapper.get('[data-testid="evolution-skill-scorecard-quality"]').text()).toContain('93%')
    expect(wrapper.get('[data-testid="evolution-skill-scorecard-quality"]').text()).toContain(
      'Score delta +0.07'
    )
    expect(
      wrapper.get('[data-testid="evolution-skill-scorecard-verification"]').text()
    ).toContain('+12%')
    expect(wrapper.get('[data-testid="evolution-skill-scorecard-runtime"]').text()).toContain(
      '1.9 s'
    )
    expect(wrapper.get('[data-testid="evolution-skill-scorecard-tokens"]').text()).toContain(
      '4,200'
    )
    expect(wrapper.get('[data-testid="evolution-skill-scorecard-evidence"]').text()).toContain(
      'browser.navigation_timeout'
    )

    await wrapper.get('[data-testid="evolution-skill-scorecard-quality-action"]').trigger('click')
    await flushPromises()

    expect(
      wrapper.get('[data-testid="evolution-skill-section-comparison"]').attributes('data-focused')
    ).toBe('true')

    await wrapper.get('[data-testid="evolution-skill-scorecard-tokens-action"]').trigger('click')
    await flushPromises()

    expect(
      wrapper.get('[data-testid="evolution-skill-section-metrics"]').attributes('data-focused')
    ).toBe('true')
  })

  it('supports searching and filtering revisions for version switching and rollback flows', async () => {
    const wrapper = await mountView()

    await wrapper.get('[data-testid="evolution-skill-revision-status-filter"]').setValue('backup')
    await flushPromises()

    expect(wrapper.find('[data-testid="evolution-skill-revision-rev-accepted"]').exists()).toBe(
      false
    )
    expect(wrapper.find('[data-testid="evolution-skill-revision-rev-promoted"]').exists()).toBe(
      false
    )
    expect(wrapper.find('[data-testid="evolution-skill-revision-rev-backup"]').exists()).toBe(
      true
    )
    expect(wrapper.get('[data-testid="evolution-summary-visible-revisions"]').text()).toBe('1')
    expect(wrapper.get('[data-testid="evolution-summary-total-revisions"]').text()).toContain('5')

    await wrapper.get('[data-testid="evolution-skill-revision-status-filter"]').setValue('all')
    await wrapper.get('[data-testid="evolution-skill-revision-search"]').setValue('capture')
    await flushPromises()

    expect(wrapper.find('[data-testid="evolution-skill-revision-rev-capture"]').exists()).toBe(
      true
    )
    expect(wrapper.find('[data-testid="evolution-skill-revision-rev-backup"]').exists()).toBe(
      false
    )
  })

  it('shows hidden-selection prompts and can reveal the selected skill, revision, case, and proposal', async () => {
    const wrapper = await mountView()

    await wrapper.get('[data-testid="evolution-skill-search"]').setValue('self reflect')
    await flushPromises()

    expect(wrapper.get('[data-testid="evolution-skill-hidden-selection"]').text()).toContain(
      'Selected skill is hidden'
    )

    await wrapper.get('[data-testid="evolution-skill-hidden-selection-reveal"]').trigger('click')
    await flushPromises()

    expect((wrapper.get('[data-testid="evolution-skill-search"]').element as HTMLInputElement).value).toBe(
      ''
    )
    expect(wrapper.find('[data-testid="evolution-skill-hidden-selection"]').exists()).toBe(false)

    await wrapper.get('[data-testid="evolution-skill-case-mode-filter"]').setValue('capture')
    await flushPromises()

    expect(wrapper.get('[data-testid="evolution-skill-case-hidden-selection"]').text()).toContain(
      'Selected case is hidden'
    )

    await wrapper
      .get('[data-testid="evolution-skill-case-hidden-selection-reveal"]')
      .trigger('click')
    await flushPromises()

    expect(
      (wrapper.get('[data-testid="evolution-skill-case-mode-filter"]').element as HTMLSelectElement)
        .value
    ).toBe('all')

    await wrapper.get('[data-testid="evolution-skill-revision-rev-backup"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="evolution-skill-revision-status-filter"]').setValue('candidate')
    await flushPromises()

    expect(
      wrapper.get('[data-testid="evolution-skill-revision-hidden-selection"]').text()
    ).toContain('Selected revision is hidden')

    await wrapper
      .get('[data-testid="evolution-skill-revision-hidden-selection-reveal"]')
      .trigger('click')
    await flushPromises()

    expect(
      (wrapper.get('[data-testid="evolution-skill-revision-status-filter"]').element as HTMLSelectElement)
        .value
    ).toBe('all')

    await wrapper.get('[data-testid="evolution-tab-instructions"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="evolution-instructions-status-filter"]').setValue('approved')
    await flushPromises()

    expect(wrapper.get('[data-testid="evolution-instructions-hidden-selection"]').text()).toContain(
      'Selected proposal is hidden'
    )

    await wrapper
      .get('[data-testid="evolution-instructions-hidden-selection-reveal"]')
      .trigger('click')
    await flushPromises()

    expect(
      (wrapper.get('[data-testid="evolution-instructions-status-filter"]').element as HTMLSelectElement)
        .value
    ).toBe('all')
  })

  it('shows next actions for empty cases, missing accepted revisions, and empty instruction proposals', async () => {
    vi.mocked(harnessApi.listSkillRevisions).mockResolvedValue({
      data: [
        {
          id: 'rev-promoted',
          skill_id: 'browser',
          status: 'promoted',
          source_path: 'assets/skills/browser/SKILL.md',
          candidate_id: 'candidate-browser-current',
          content: '# Browser\n\nUse the browser carefully.\n',
          content_sha256: 'sha-browser-current',
          created_at: '2026-04-03T08:00:00Z',
        },
        {
          id: 'rev-backup',
          skill_id: 'browser',
          status: 'backup',
          source_path: 'assets/skills/browser/SKILL.md',
          backup_of_revision_id: 'rev-promoted',
          content: '# Browser\n\nUse the browser carefully.\n',
          content_sha256: 'sha-browser-backup',
          created_at: '2026-04-04T08:00:00Z',
        },
        {
          id: 'rev-capture',
          skill_id: 'browser',
          status: 'candidate',
          source_path: 'assets/skills/browser/SKILL.md',
          candidate_id: 'candidate-browser-capture',
          origin_case_id: 'case-browser-capture',
          content: '# Browser\n\nUse the browser carefully and include successful recovery examples.\n',
          content_sha256: 'sha-browser-capture',
          created_at: '2026-04-02T08:00:00Z',
        },
      ],
    } as never)
    vi.mocked(harnessApi.listSkillDecisionHistory).mockResolvedValue({
      data: [],
    } as never)
    vi.mocked(harnessApi.listSkillEvolutionCases).mockResolvedValue({
      data: [],
    } as never)
    vi.mocked(selfReflectApi.listProposals).mockResolvedValue({
      data: [],
    } as never)

    const wrapper = await mountView()

    expect(wrapper.get('[data-testid="evolution-skill-no-accepted-revision"]').text()).toContain(
      'No accepted revision yet'
    )

    await wrapper
      .get('[data-testid="evolution-skill-no-accepted-review-candidate"]')
      .trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="evolution-skill-decision-selected"]').text()).toContain(
      'Pending gate'
    )
    expect(wrapper.get('[data-testid="evolution-skill-case-empty-actions"]').text()).toContain(
      'No cases yet'
    )

    await wrapper
      .get('[data-testid="evolution-skill-case-empty-open-harness"]')
      .trigger('click')
    await flushPromises()

    expect(routerPushMock).toHaveBeenCalledWith({
      name: 'HarnessGroups',
    })

    await wrapper.get('[data-testid="evolution-tab-instructions"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="evolution-instructions-empty-actions"]').text()).toContain(
      'No proposals yet'
    )

    await wrapper
      .get('[data-testid="evolution-instructions-empty-open-runner"]')
      .trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="agentcore-runner-panel-stub"]').text()).toContain(
      'Runner panel'
    )
  })

  it('restores pane, selection, and filters from the route query and writes updated state back', async () => {
    routeMock.query = {
      pane: 'instructions',
      skill: 'browser',
      revision: 'rev-capture',
      skillSearch: 'browser',
      revisionSearch: 'capture',
      revisionStatus: 'candidate',
      caseMode: 'capture',
      proposal: 'proposal-2',
      instructionSearch: 'operations',
      instructionStatus: 'approved',
    }

    const wrapper = await mountView()

    expect(wrapper.text()).toContain('Instruction Review Queue')
    expect((wrapper.get('[data-testid="evolution-instructions-search"]').element as HTMLInputElement).value).toBe(
      'operations'
    )
    expect(
      (wrapper.get('[data-testid="evolution-instructions-status-filter"]').element as HTMLSelectElement)
        .value
    ).toBe('approved')
    expect(wrapper.find('[data-testid="evolution-instructions-proposal-proposal-1"]').exists()).toBe(
      false
    )
    expect(wrapper.find('[data-testid="evolution-instructions-proposal-proposal-2"]').exists()).toBe(
      true
    )

    routerReplaceMock.mockClear()

    await wrapper.get('[data-testid="evolution-tab-skills"]').trigger('click')
    await flushPromises()
    expect(
      (wrapper.get('[data-testid="evolution-skill-revision-search"]').element as HTMLInputElement)
        .value
    ).toBe('capture')
    expect(
      (wrapper.get('[data-testid="evolution-skill-revision-status-filter"]').element as HTMLSelectElement)
        .value
    ).toBe('candidate')
    await wrapper.get('[data-testid="evolution-skill-case-status-filter"]').setValue('accepted')
    await flushPromises()

    expect(routerReplaceMock).toHaveBeenCalled()
    const lastCall = routerReplaceMock.mock.calls.at(-1)?.[0] as { query?: Record<string, string> }
    expect(lastCall.query).toMatchObject({
      skill: 'browser',
      revision: 'rev-capture',
      skillSearch: 'browser',
      revisionSearch: 'capture',
      revisionStatus: 'candidate',
      caseMode: 'capture',
      caseStatus: 'accepted',
      proposal: 'proposal-2',
      instructionSearch: 'operations',
      instructionStatus: 'approved',
    })
    expect(lastCall.query?.pane).toBeUndefined()
  })

  it('triggers a fresh evolution run from the selected revision eval evidence', async () => {
    const wrapper = await mountView()

    expect(wrapper.get('[data-testid="evolution-skill-optimize"]').text()).toContain(
      'Run Evolution From This Eval'
    )

    await wrapper.get('[data-testid="evolution-skill-optimize"]').trigger('click')
    await flushPromises()

    expect(harnessApi.optimizeSkill).toHaveBeenCalledWith('browser', {
      eval_run_id: 'eval-browser-1',
      source_path: 'assets/skills/browser/SKILL.md',
    })
  })

  it('does not promote when the confirmation dialog is cancelled', async () => {
    const wrapper = await mountView()
    const confirmSpy = setConfirmResult(false)

    await wrapper.get('[data-testid="evolution-skill-promote"]').trigger('click')
    await flushPromises()

    expect(confirmSpy).toHaveBeenCalledTimes(1)
    expect(harnessApi.promoteSkillRevision).not.toHaveBeenCalled()
  })

  it('keeps operator rationale drafts per revision and includes them in promote and rollback confirmations', async () => {
    const wrapper = await mountView()
    const confirmSpy = setConfirmResult(false)

    await wrapper
      .get('[data-testid="evolution-skill-review-note-suggestion-0"]')
      .trigger('click')
    await flushPromises()

    expect((wrapper.get('[data-testid="evolution-skill-review-note-input"]').element as HTMLTextAreaElement).value).toContain(
      'Passed gate with grounded evidence'
    )

    await wrapper.get('[data-testid="evolution-skill-promote"]').trigger('click')
    await flushPromises()

    expect(confirmSpy).toHaveBeenCalledTimes(1)
    expect(String(confirmSpy.mock.calls[0]?.[0] || '')).toContain('Operator rationale')
    expect(String(confirmSpy.mock.calls[0]?.[0] || '')).toContain(
      'Passed gate with grounded evidence'
    )

    await wrapper.get('[data-testid="evolution-skill-revision-rev-backup"]').trigger('click')
    await flushPromises()

    expect((wrapper.get('[data-testid="evolution-skill-review-note-input"]').element as HTMLTextAreaElement).value).toBe(
      ''
    )

    await wrapper
      .get('[data-testid="evolution-skill-review-note-suggestion-0"]')
      .trigger('click')
    await flushPromises()

    expect((wrapper.get('[data-testid="evolution-skill-review-note-input"]').element as HTMLTextAreaElement).value).toContain(
      'Restore the previous stable canonical behavior'
    )

    await wrapper.get('[data-testid="evolution-skill-rollback"]').trigger('click')
    await flushPromises()

    expect(confirmSpy).toHaveBeenCalledTimes(2)
    expect(String(confirmSpy.mock.calls[1]?.[0] || '')).toContain('Operator rationale')
    expect(String(confirmSpy.mock.calls[1]?.[0] || '')).toContain(
      'Restore the previous stable canonical behavior'
    )

    await wrapper.get('[data-testid="evolution-skill-revision-rev-accepted"]').trigger('click')
    await flushPromises()

    expect((wrapper.get('[data-testid="evolution-skill-review-note-input"]').element as HTMLTextAreaElement).value).toContain(
      'Passed gate with grounded evidence'
    )
  })

  it('renders operator sign-off and decision log previews for promote and rollback actions', async () => {
    const wrapper = await mountView()

    expect(wrapper.get('[data-testid="evolution-skill-signoff"]').text()).toContain(
      'Operator sign-off'
    )
    expect(wrapper.get('[data-testid="evolution-skill-signoff-decision"]').text()).toContain(
      'Promote to canonical'
    )
    expect(wrapper.get('[data-testid="evolution-skill-signoff-log-current"]').text()).toContain(
      'rev-promoted'
    )
    expect(wrapper.get('[data-testid="evolution-skill-signoff-log-target"]').text()).toContain(
      'rev-accepted'
    )
    expect(wrapper.get('[data-testid="evolution-skill-signoff-log-rationale"]').text()).toContain(
      'Not captured yet'
    )

    await wrapper
      .get('[data-testid="evolution-skill-review-note-suggestion-0"]')
      .trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="evolution-skill-signoff-log-rationale"]').text()).toContain(
      'Passed gate with grounded evidence'
    )

    await wrapper.get('[data-testid="evolution-skill-revision-rev-backup"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="evolution-skill-signoff-decision"]').text()).toContain(
      'Rollback to this backup'
    )
    expect(wrapper.get('[data-testid="evolution-skill-signoff-log-source"]').text()).toContain(
      'rev-backup'
    )
    expect(wrapper.get('[data-testid="evolution-skill-signoff-log-lineage"]').text()).toContain(
      'rev-promoted'
    )
    expect(wrapper.get('[data-testid="evolution-skill-signoff-log-rationale"]').text()).toContain(
      'Not captured yet'
    )

    await wrapper
      .get('[data-testid="evolution-skill-review-note-suggestion-0"]')
      .trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="evolution-skill-signoff-log-rationale"]').text()).toContain(
      'Restore the previous stable canonical behavior'
    )
  })

  it('shows structured decision history for promoted revisions instead of raw decision JSON blobs', async () => {
    const wrapper = await mountView()

    await wrapper.get('[data-testid="evolution-skill-revision-rev-promoted"]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Decision History')
    expect(
      wrapper.get('[data-testid="evolution-skill-decision-history-entry-rev-promoted"]').text()
    ).toContain('Promoted to canonical')
    expect(
      wrapper.get('[data-testid="evolution-skill-decision-history-rev-promoted-badge-selected"]').text()
    ).toContain('Selected revision')
    expect(
      wrapper.get('[data-testid="evolution-skill-decision-history-rev-promoted-badge-live"]').text()
    ).toContain('Live now')
    expect(
      wrapper.get('[data-testid="evolution-skill-decision-history-rev-promoted-reviewed_by"]').text()
    ).toContain('user-promote-handler')
    expect(
      wrapper.get('[data-testid="evolution-skill-decision-history-rev-promoted-review_note"]').text()
    ).toContain('Promoted after explicit operator sign-off.')
    expect(
      wrapper.get('[data-testid="evolution-skill-decision-history-rev-promoted-backup_revision"]').text()
    ).toContain('rev-backup')
    expect(
      wrapper.get('[data-testid="evolution-skill-decision-history-rev-promoted-written_source_path"]').text()
    ).toContain('assets/skills/browser/SKILL.md')
    expect(harnessApi.listSkillDecisionHistory).toHaveBeenCalledWith('browser', { limit: 50 })
    expect(wrapper.text()).not.toContain('"target_revision_id":"rev-promoted"')
  })

  it('supports quick rollback directly from a decision history entry', async () => {
    const wrapper = await mountView()
    const confirmSpy = setConfirmResult(true)

    await wrapper.get('[data-testid="evolution-skill-revision-rev-promoted"]').trigger('click')
    await flushPromises()

    await wrapper
      .get('[data-testid="evolution-skill-decision-history-rev-promoted-action-quick-rollback"]')
      .trigger('click')
    await flushPromises()

    expect(confirmSpy).toHaveBeenCalledTimes(1)
    expect(harnessApi.rollbackSkillRevision).toHaveBeenCalledWith('rev-backup')
  })

  it('renders a decision timeline and exports the decision log as JSON', async () => {
    const originalCreateObjectURL = URL.createObjectURL
    const originalRevokeObjectURL = URL.revokeObjectURL
    const createObjectURLMock = vi.fn(() => 'blob:test')
    const revokeObjectURLMock = vi.fn()
    const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})

    Object.defineProperty(URL, 'createObjectURL', {
      configurable: true,
      value: createObjectURLMock,
    })
    Object.defineProperty(URL, 'revokeObjectURL', {
      configurable: true,
      value: revokeObjectURLMock,
    })

    try {
      const wrapper = await mountView()

      expect(wrapper.get('[data-testid="evolution-skill-decision-timeline"]').text()).toContain(
        'Decision Timeline'
      )
      expect(
        wrapper.get('[data-testid="evolution-skill-decision-timeline-rev-promoted"]').text()
      ).toContain('Promoted to canonical')

      await wrapper.get('[data-testid="evolution-skill-decision-history-export"]').trigger('click')
      await flushPromises()

      expect(createObjectURLMock).toHaveBeenCalledTimes(1)
      const exportedBlob = createObjectURLMock.mock.calls[0]?.[0] as Blob
      const exportedText = await exportedBlob.text()
      expect(exportedText).toContain('"skill_id": "browser"')
      expect(exportedText).toContain('"revision_id": "rev-promoted"')
      expect(exportedText).toContain('"links"')
      expect(exportedText).toContain('"details"')
      expect(clickSpy).toHaveBeenCalledTimes(1)
      expect(revokeObjectURLMock).toHaveBeenCalledTimes(1)
    } finally {
      clickSpy.mockRestore()
      Object.defineProperty(URL, 'createObjectURL', {
        configurable: true,
        value: originalCreateObjectURL,
      })
      Object.defineProperty(URL, 'revokeObjectURL', {
        configurable: true,
        value: originalRevokeObjectURL,
      })
    }
  })

  it('renders rollback decision history with restored/live/backup lineage fields', async () => {
    vi.mocked(harnessApi.listSkillRevisions).mockResolvedValue({
      data: [
        {
          id: 'rev-accepted',
          skill_id: 'browser',
          status: 'accepted',
          source_path: 'assets/skills/browser/SKILL.md',
          candidate_id: 'candidate-browser-1',
          base_content_sha256: 'sha-browser-base',
          origin_case_id: 'case-browser-1',
          eval_run_id: 'eval-browser-1',
          content: '# Browser\n\nUse the browser carefully and recover after navigation errors.\n',
          content_sha256: 'sha-browser-next',
          created_at: '2026-04-05T08:00:00Z',
        },
        {
          id: 'rev-promoted',
          skill_id: 'browser',
          status: 'promoted',
          source_path: 'assets/skills/browser/SKILL.md',
          candidate_id: 'candidate-browser-current',
          decision_action: 'promote',
          review_note: 'Promoted after explicit operator sign-off.',
          reviewed_by: 'user-promote-handler',
          reviewed_at: '2026-04-03T08:10:00Z',
          decision_log_json:
            '{"action":"promote","review_note":"Promoted after explicit operator sign-off.","reviewed_by":"user-promote-handler","reviewed_at":"2026-04-03T08:10:00Z","selected_revision_id":"rev-promoted","target_revision_id":"rev-promoted","backup_revision_id":"rev-backup","written_source_path":"/Users/orca/Documents/GitHub/ZimaOS-Blue/assets/skills/browser/SKILL.md"}',
          content: '# Browser\n\nUse the browser carefully.\n',
          content_sha256: 'sha-browser-current',
          created_at: '2026-04-03T08:00:00Z',
        },
        {
          id: 'rev-backup',
          skill_id: 'browser',
          status: 'backup',
          source_path: 'assets/skills/browser/SKILL.md',
          backup_of_revision_id: 'rev-promoted',
          content: '# Browser\n\nUse the browser carefully.\n',
          content_sha256: 'sha-browser-backup',
          created_at: '2026-04-04T08:00:00Z',
        },
        {
          id: 'rev-rollback-live',
          skill_id: 'browser',
          status: 'promoted',
          source_path: 'assets/skills/browser/SKILL.md',
          candidate_id: 'candidate-browser-rollback',
          decision_action: 'rollback',
          review_note: 'Rollback restored the last stable browser flow.',
          reviewed_by: 'user-rollback-handler',
          reviewed_at: '2026-04-01T09:30:00Z',
          decision_log_json:
            '{"action":"rollback","review_note":"Rollback restored the last stable browser flow.","reviewed_by":"user-rollback-handler","reviewed_at":"2026-04-01T09:30:00Z","selected_revision_id":"rev-backup-old","source_revision_id":"rev-backup-old","target_revision_id":"rev-rollback-live","current_live_revision_id":"rev-promoted-old","backup_revision_id":"rev-promoted-old-backup","written_source_path":"/Users/orca/Documents/GitHub/ZimaOS-Blue/assets/skills/browser/SKILL.md"}',
          content: '# Browser\n\nRollback-restored browser guidance.\n',
          content_sha256: 'sha-browser-rollback',
          created_at: '2026-04-01T09:20:00Z',
          promoted_at: '2026-04-01T09:30:00Z',
        },
      ],
    } as never)
    vi.mocked(harnessApi.listSkillDecisionHistory).mockResolvedValue({
      data: [
        {
          revision_id: 'rev-promoted',
          skill_id: 'browser',
          status: 'promoted',
          source_path: 'assets/skills/browser/SKILL.md',
          candidate_id: 'candidate-browser-current',
          decision_action: 'promote',
          review_note: 'Promoted after explicit operator sign-off.',
          reviewed_by: 'user-promote-handler',
          decision_log_json:
            '{"action":"promote","review_note":"Promoted after explicit operator sign-off.","reviewed_by":"user-promote-handler","reviewed_at":"2026-04-03T08:10:00Z","selected_revision_id":"rev-promoted","target_revision_id":"rev-promoted","backup_revision_id":"rev-backup","written_source_path":"/Users/orca/Documents/GitHub/ZimaOS-Blue/assets/skills/browser/SKILL.md"}',
          decision_at: '2026-04-03T08:10:00Z',
          reviewed_at: '2026-04-03T08:10:00Z',
          created_at: '2026-04-03T08:00:00Z',
          promoted_at: '2026-04-03T08:10:00Z',
        },
        {
          revision_id: 'rev-rollback-live',
          skill_id: 'browser',
          status: 'promoted',
          source_path: 'assets/skills/browser/SKILL.md',
          candidate_id: 'candidate-browser-rollback',
          decision_action: 'rollback',
          review_note: 'Rollback restored the last stable browser flow.',
          reviewed_by: 'user-rollback-handler',
          decision_log_json:
            '{"action":"rollback","review_note":"Rollback restored the last stable browser flow.","reviewed_by":"user-rollback-handler","reviewed_at":"2026-04-01T09:30:00Z","selected_revision_id":"rev-backup-old","source_revision_id":"rev-backup-old","target_revision_id":"rev-rollback-live","current_live_revision_id":"rev-promoted-old","backup_revision_id":"rev-promoted-old-backup","written_source_path":"/Users/orca/Documents/GitHub/ZimaOS-Blue/assets/skills/browser/SKILL.md"}',
          decision_at: '2026-04-01T09:30:00Z',
          reviewed_at: '2026-04-01T09:30:00Z',
          created_at: '2026-04-01T09:20:00Z',
          promoted_at: '2026-04-01T09:30:00Z',
        },
      ],
    } as never)

    const wrapper = await mountView()

    expect(
      wrapper.get('[data-testid="evolution-skill-decision-history-entry-rev-rollback-live"]').text()
    ).toContain('Rollback promoted live')
    expect(
      wrapper.get('[data-testid="evolution-skill-decision-history-rev-rollback-live-source_revision"]').text()
    ).toContain('rev-backup-old')
    expect(
      wrapper.get(
        '[data-testid="evolution-skill-decision-history-rev-rollback-live-current_live_revision"]'
      ).text()
    ).toContain('rev-promoted-old')
    expect(
      wrapper.get('[data-testid="evolution-skill-decision-history-rev-rollback-live-backup_revision"]').text()
    ).toContain('rev-promoted-old-backup')
    expect(
      wrapper.get('[data-testid="evolution-skill-decision-history-rev-rollback-live-review_note"]').text()
    ).toContain('Rollback restored the last stable browser flow.')
  })

  it('does not approve an instruction proposal when the confirmation dialog is cancelled', async () => {
    const wrapper = await mountView()
    const confirmSpy = setConfirmResult(false)

    await wrapper.get('[data-testid="evolution-tab-instructions"]').trigger('click')
    await flushPromises()

    await wrapper.get('[data-testid="evolution-instructions-approve"]').trigger('click')
    await flushPromises()

    expect(confirmSpy).toHaveBeenCalledTimes(1)
    expect(selfReflectApi.approveProposal).not.toHaveBeenCalled()
  })

  it('shows the dedicated runner lane separately from skills and instructions', async () => {
    const wrapper = await mountView()

    await wrapper.get('[data-testid="evolution-tab-runner"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="agentcore-runner-panel-stub"]').text()).toContain(
      'Runner panel'
    )
  })

  it('places the runner settings panel above the runner summary cards', async () => {
    const wrapper = await mountView()

    await wrapper.get('[data-testid="evolution-tab-runner"]').trigger('click')
    await flushPromises()

    const panel = wrapper.get('[data-testid="agentcore-runner-panel-stub"]').element
    const summaryCard = wrapper.get('[data-testid="evolution-runner-summary-candidate"]').element

    expect(panel.compareDocumentPosition(summaryCard) & Node.DOCUMENT_POSITION_FOLLOWING).toBe(
      Node.DOCUMENT_POSITION_FOLLOWING
    )
  })

  it('renders runner evolution metrics, candidate diff, and cross-lane links from the optimization record', async () => {
    const wrapper = await mountView()

    await wrapper.get('[data-testid="evolution-tab-runner"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="evolution-runner-summary-candidate"]').text()).toContain(
      'rev-accepted'
    )
    expect(wrapper.get('[data-testid="evolution-runner-summary-followup"]').text()).toContain(
      'Accepted via Selector gate'
    )
    expect(wrapper.get('[data-testid="evolution-runner-summary-runtime"]').text()).toContain('2.4s')
    expect(wrapper.get('[data-testid="evolution-runner-summary-parts"]').text()).toContain(
      '2 parts changed'
    )
    expect(wrapper.get('[data-testid="evolution-runner-link-followup-eval"]').text()).toContain(
      'eval-runner-followup'
    )
    expect(wrapper.get('[data-testid="evolution-runner-link-linked-revision"]').text()).toContain(
      'rev-accepted'
    )
    expect(wrapper.get('[data-testid="evolution-runner-report-followup-overall_score"]').text()).toContain(
      '96%'
    )
    expect(wrapper.get('[data-testid="evolution-runner-report-followup-total_tokens"]').text()).toContain(
      '2,600'
    )
    expect(wrapper.get('[data-testid="evolution-runner-candidate-diff"]').text()).toContain(
      'recover after navigation errors'
    )
    expect(wrapper.get('[data-testid="evolution-runner-transcript"]').text()).toContain(
      'Optimization trigger received.'
    )
  })

  it('opens runner-linked evals and sends linked revisions back into the Skills lane', async () => {
    const wrapper = await mountView()

    await wrapper.get('[data-testid="evolution-tab-runner"]').trigger('click')
    await flushPromises()

    await wrapper.get('[data-testid="evolution-runner-open-source-eval"]').trigger('click')
    await flushPromises()
    expect(routerPushMock).toHaveBeenCalledWith({
      name: 'HarnessGroups',
      query: { evalRunId: 'eval-runner-source' },
    })

    await wrapper.get('[data-testid="evolution-runner-open-followup-eval"]').trigger('click')
    await flushPromises()
    expect(routerPushMock).toHaveBeenCalledWith({
      name: 'HarnessGroups',
      query: { evalRunId: 'eval-runner-followup' },
    })

    await wrapper.get('[data-testid="evolution-runner-open-linked-revision"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="evolution-skill-revision-rev-accepted"]').attributes('data-active')).toBe(
      'true'
    )
    expect(wrapper.get('[data-testid="evolution-skill-diff"]').text()).toContain(
      'recover after navigation errors'
    )
  })

  it('allows rolling back from a selected backup revision', async () => {
    const wrapper = await mountView()
    const confirmSpy = setConfirmResult(true)

    await wrapper.get('[data-testid="evolution-skill-revision-rev-backup"]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Backup')
    expect(wrapper.get('[data-testid="evolution-skill-rollback"]').text()).toContain('Rollback')
    expect(wrapper.text()).toContain('Restorable backup')
    expect(wrapper.text()).toContain('Backup of revision')
    expect(wrapper.text()).toContain('rev-promoted')
    expect(wrapper.get('[data-testid="evolution-skill-decision-selected"]').text()).toContain(
      'Rollback backup'
    )
    expect(wrapper.get('[data-testid="evolution-skill-decision-next_step"]').text()).toContain(
      'Rollback eligible'
    )
    expect(wrapper.get('[data-testid="evolution-skill-rollback-impact-target"]').text()).toContain(
      'Restore previous canonical'
    )
    expect(wrapper.get('[data-testid="evolution-skill-rollback-impact-live"]').text()).toContain(
      'rev-promoted'
    )
    expect(wrapper.get('[data-testid="evolution-skill-rollback-impact-guard"]').text()).toContain(
      'Lineage must still match'
    )
    expect(wrapper.get('[data-testid="evolution-skill-switch-preview-source"]').text()).toContain(
      'rev-backup'
    )
    expect(wrapper.get('[data-testid="evolution-skill-switch-preview-next"]').text()).toContain(
      'rev-promoted'
    )
    expect(wrapper.get('[data-testid="evolution-skill-switch-preview-preserve"]').text()).toContain(
      'Current live becomes a new backup'
    )

    await wrapper.get('[data-testid="evolution-skill-rollback"]').trigger('click')
    await flushPromises()

    expect(confirmSpy).toHaveBeenCalledTimes(1)
    expect(harnessApi.rollbackSkillRevision).toHaveBeenCalledWith('rev-backup')
  })

  it('navigates from decision history entry to revision, eval run, and related revisions', async () => {
    // Use a fixture with rollback action for navigation testing
    vi.mocked(harnessApi.listSkillRevisions).mockResolvedValue({
      data: [
        {
          id: 'rev-promoted',
          skill_id: 'browser',
          status: 'promoted',
          source_path: 'assets/skills/browser/SKILL.md',
          candidate_id: 'candidate-browser',
          decision_action: 'rollback',
          review_note: 'Rolled back to stable version.',
          reviewed_by: 'user-rollback',
          reviewed_at: '2026-04-01T09:30:00Z',
          decision_log_json: JSON.stringify({
            action: 'rollback',
            review_note: 'Rolled back to stable version.',
            reviewed_by: 'user-rollback',
            reviewed_at: '2026-04-01T09:30:00Z',
            selected_revision_id: 'rev-backup-old',
            source_revision_id: 'rev-backup-old',
            target_revision_id: 'rev-promoted',
            current_live_revision_id: 'rev-failed',
            backup_revision_id: 'rev-failed-backup',
            eval_run_id: 'eval-rollback-1',
          }),
          eval_run_id: 'eval-rollback-1',
          content: '# Browser\n\nRestored content.',
          content_sha256: 'sha-restored',
          created_at: '2026-04-01T09:20:00Z',
          promoted_at: '2026-04-01T09:30:00Z',
        },
        {
          id: 'rev-backup-old',
          skill_id: 'browser',
          status: 'backup',
          source_path: 'assets/skills/browser/SKILL.md',
          backup_of_revision_id: 'rev-promoted',
          content: '# Browser\n\nOld backup content.',
          content_sha256: 'sha-backup-old',
          created_at: '2026-03-30T08:00:00Z',
        },
        {
          id: 'rev-failed',
          skill_id: 'browser',
          status: 'promoted',
          source_path: 'assets/skills/browser/SKILL.md',
          candidate_id: 'candidate-failed',
          decision_action: 'promote',
          review_note: 'Original promotion.',
          reviewed_by: 'user-promote',
          reviewed_at: '2026-03-28T10:00:00Z',
          decision_log_json: JSON.stringify({
            action: 'promote',
            selected_revision_id: 'rev-failed',
            target_revision_id: 'rev-failed',
            backup_revision_id: 'rev-failed-backup',
          }),
          content: '# Browser\n\nFailed content.',
          content_sha256: 'sha-failed',
          created_at: '2026-03-28T09:00:00Z',
          promoted_at: '2026-03-28T10:00:00Z',
        },
      ],
    } as never)
    vi.mocked(harnessApi.listSkillDecisionHistory).mockResolvedValue({
      data: [
        {
          revision_id: 'rev-promoted',
          skill_id: 'browser',
          status: 'promoted',
          source_path: 'assets/skills/browser/SKILL.md',
          candidate_id: 'candidate-browser',
          decision_action: 'rollback',
          review_note: 'Rolled back to stable version.',
          reviewed_by: 'user-rollback',
          decision_log_json: JSON.stringify({
            action: 'rollback',
            review_note: 'Rolled back to stable version.',
            reviewed_by: 'user-rollback',
            reviewed_at: '2026-04-01T09:30:00Z',
            selected_revision_id: 'rev-backup-old',
            source_revision_id: 'rev-backup-old',
            target_revision_id: 'rev-promoted',
            current_live_revision_id: 'rev-failed',
            backup_revision_id: 'rev-failed-backup',
            eval_run_id: 'eval-rollback-1',
          }),
          eval_run_id: 'eval-rollback-1',
          decision_at: '2026-04-01T09:30:00Z',
          reviewed_at: '2026-04-01T09:30:00Z',
          created_at: '2026-04-01T09:20:00Z',
          promoted_at: '2026-04-01T09:30:00Z',
        },
        {
          revision_id: 'rev-failed',
          skill_id: 'browser',
          status: 'promoted',
          source_path: 'assets/skills/browser/SKILL.md',
          candidate_id: 'candidate-failed',
          decision_action: 'promote',
          review_note: 'Original promotion.',
          reviewed_by: 'user-promote',
          decision_log_json: JSON.stringify({
            action: 'promote',
            selected_revision_id: 'rev-failed',
            target_revision_id: 'rev-failed',
            backup_revision_id: 'rev-failed-backup',
          }),
          decision_at: '2026-03-28T10:00:00Z',
          reviewed_at: '2026-03-28T10:00:00Z',
          created_at: '2026-03-28T09:00:00Z',
          promoted_at: '2026-03-28T10:00:00Z',
        },
      ],
    } as never)

    const wrapper = await mountView()

    // Click the rollback revision to see its decision history
    await wrapper.get('[data-testid="evolution-skill-revision-rev-promoted"]').trigger('click')
    await flushPromises()

    // Should have view revision action for the rollback revision
    expect(
      wrapper.find('[data-testid="evolution-skill-decision-history-rev-promoted-action-view-revision"]').exists()
    ).toBe(true)

    // Should have eval run link
    expect(
      wrapper.find('[data-testid="evolution-skill-decision-history-rev-promoted-action-open-eval"]').exists()
    ).toBe(true)

    expect(
      wrapper.get('[data-testid="evolution-skill-decision-history-rev-promoted-evidence-overall_score"]').text()
    ).toContain('93%')
    expect(
      wrapper.get('[data-testid="evolution-skill-decision-history-rev-promoted-evidence-pass_rate"]').text()
    ).toContain('100%')
    expect(
      wrapper.get('[data-testid="evolution-skill-decision-history-rev-promoted-evidence-avg_duration_ms"]').text()
    ).toContain('1.9 s')
    expect(
      wrapper.get('[data-testid="evolution-skill-decision-history-rev-promoted-evidence-total_tokens"]').text()
    ).toContain('4,200')

    expect(
      wrapper.find('[data-testid="evolution-skill-decision-history-rev-promoted-action-compare-source-target"]').exists()
    ).toBe(true)

    await wrapper
      .get('[data-testid="evolution-skill-decision-history-rev-promoted-action-compare-source-target"]')
      .trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="evolution-skill-history-comparison"]').text()).toContain(
      'History Pair Comparison'
    )
    expect(wrapper.get('[data-testid="evolution-skill-history-comparison"]').text()).toContain(
      'Source vs target'
    )
    expect(wrapper.get('[data-testid="evolution-skill-history-comparison-left"]').text()).toContain(
      'rev-backup-old'
    )
    expect(wrapper.get('[data-testid="evolution-skill-history-comparison-right"]').text()).toContain(
      'rev-promoted'
    )

    // Should have view backup action for the preserved backup
    expect(
      wrapper.find('[data-testid="evolution-skill-decision-history-rev-promoted-action-view-backup"]').exists()
    ).toBe(true)

    // Should show rollback-specific navigation options
    expect(
      wrapper.find('[data-testid="evolution-skill-decision-history-rev-promoted-action-view-source"]').exists()
    ).toBe(true)
    expect(
      wrapper.find('[data-testid="evolution-skill-decision-history-rev-promoted-action-view-live"]').exists()
    ).toBe(true)
  })
})
