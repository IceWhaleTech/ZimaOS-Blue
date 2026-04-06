import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'

import UserTaskProjectionCard from '@/components/UserTaskProjectionCard.vue'

function createTestI18n() {
  return createI18n({
    legacy: false,
    locale: 'en-US',
    fallbackLocale: 'en-US',
    messages: {
      'en-US': {
        chat: {
          taskStagePlanning: 'Planning',
          taskStageWorking: 'Working',
          taskStageVerifying: 'Verifying',
          taskStageWaiting: 'Waiting',
          taskStageCompleted: 'Completed',
          taskStagePartial: 'Partially passed',
          taskStageFailed: 'Failed',
          taskStageCancelled: 'Cancelled',
          taskKindResearch: 'Deep Research',
          taskKindAgent: 'Agent',
          taskKindWorkflow: 'Workflow',
          taskOpenConversation: 'Open conversation',
          taskResume: 'Resume',
          taskCancel: 'Cancel',
          taskSendUpdate: 'Send update',
          taskWaitingForApproval: 'Waiting for your approval',
          taskWaitingForAnswer: 'Waiting for your answer',
          taskRuntimeExecute: 'Executing',
          deepResearchSourceInventory: 'Source Inventory',
          deepResearchPublishedAt: 'Published',
          deepResearchFetchedAt: 'Fetched',
          deepResearchRelevance: 'Rel',
          deepResearchCredibility: 'Cred',
          deepResearchStageCompleted: 'Completed',
          deepResearchActionCompleted: 'Completed',
          taskHarnessRunStatus: 'Run',
          taskHarnessVerificationStatus: 'Verification',
          taskHarnessEvidenceCount: 'Evidence',
          taskHarnessRecorded: 'Recorded',
          taskHarnessViewDetails: 'View run details',
          taskHarnessAddToRegression: 'Add to regression',
          taskHarnessRerun: 'Rerun',
        },
        harness: {
          groups: {
            score: 'Score',
          },
        },
      },
      'zh-CN': {
        chat: {
          taskStageCompleted: '已完成',
          taskOpenConversation: '打开会话',
          deepResearchSourceInventory: '来源清单',
          deepResearchPublishedAt: '发布时间',
          deepResearchFetchedAt: '抓取时间',
          deepResearchRelevance: '相关性',
          deepResearchCredibility: '可信度',
          deepResearchStageCompleted: '已完成',
          deepResearchActionCompleted: '已完成',
        },
      },
    },
  })
}

describe('UserTaskProjectionCard', () => {
  it('renders a current task projection and emits task actions', async () => {
    const wrapper = mount(UserTaskProjectionCard, {
      props: {
        task: {
          id: 'task-1',
          kind: 'agent_task',
          scope: 'current',
          conversation_id: 'conv-1',
          title: 'Ship the migration',
          subtitle: 'execute',
          status: 'waiting_user',
          stage: 'waiting_user',
          progress: 41,
          blocker: {
            kind: 'approval',
            label: 'Waiting for your approval',
            pending_count: 1,
            modal_only: true,
          },
          result_preview: 'Drafted the plan',
          artifacts: [{ kind: 'report', label: 'Summary', url: 'https://example.com/report' }],
          actions: {
            items: [
              {
                id: 'cancel',
                label: 'Cancel',
                method: 'POST',
                path: '/tasks/task-1/actions/cancel',
                variant: 'danger',
              },
              {
                id: 'send_update',
                label: 'Send update',
                method: 'POST',
                path: '/agent/tasks/task-1/message',
                requires_input: true,
                input: {
                  fields: [
                    {
                      key: 'message',
                      label: 'Message',
                      kind: 'textarea',
                      target: 'root',
                      required: true,
                      placeholder: 'Send an update to this task',
                    },
                  ],
                },
              },
            ],
          },
          updated_at: '2026-03-20T12:00:00.000Z',
        },
      },
      global: {
        plugins: [createTestI18n()],
      },
    })

    expect(wrapper.text()).toContain('Ship the migration')
    expect(wrapper.text()).toContain('Waiting')
    expect(wrapper.text()).toContain('Executing')
    expect(wrapper.text()).toContain('Waiting for your approval')
    expect(wrapper.text()).toContain('Drafted the plan')
    expect(wrapper.text()).toContain('Summary')
    expect(wrapper.text()).toContain('Send update')

    await wrapper.get('button[class*="border-slate-200"]').trigger('click')
    await wrapper.get('button[class*="border-rose-200"]').trigger('click')

    expect(wrapper.emitted('action')?.[0]).toEqual([
      expect.objectContaining({ id: 'task-1' }),
      'send_update',
    ])
    expect(wrapper.emitted('action')?.[1]).toEqual([
      expect.objectContaining({ id: 'task-1' }),
      'cancel',
    ])
  })

  it('renders harness summary chips and emits navigation shortcuts', async () => {
    const wrapper = mount(UserTaskProjectionCard, {
      props: {
        task: {
          id: 'task-harness-1',
          kind: 'agent_task',
          scope: 'current',
          title: 'Regression validation',
          status: 'failed',
          stage: 'partial',
          progress: 100,
          actions: { items: [] },
          run_status: 'completed',
          verification_status: 'partial',
          score: 0.76,
          evidence_count: 5,
          detail_href: '/operations/harness/group-1',
          updated_at: '2026-03-20T12:00:00.000Z',
        },
      },
      global: {
        plugins: [createTestI18n()],
      },
    })

    expect(wrapper.text()).toContain('Partially passed')
    expect(wrapper.text()).toContain('Run: Completed')
    expect(wrapper.text()).toContain('Verification: Partially passed')
    expect(wrapper.text()).toContain('Score: 0.76')
    expect(wrapper.text()).toContain('Evidence: 5')
    expect(wrapper.text()).toContain('Recorded')
    expect(wrapper.text()).toContain('View run details')
    expect(wrapper.text()).toContain('Rerun')

    await wrapper
      .findAll('button')
      .find((button) => button.text() === 'View run details')!
      .trigger('click')

    expect(wrapper.emitted('navigate')?.[0]).toEqual(['/operations/harness/group-1'])
  })

  it('renders subagent summary chips and the latest child run label', () => {
    const wrapper = mount(UserTaskProjectionCard, {
      props: {
        task: {
          id: 'task-subagents-1',
          kind: 'agent_task',
          scope: 'current',
          title: 'Parallel fix-up',
          status: 'running',
          stage: 'working',
          progress: 58,
          subagent_summary: {
            total: 4,
            running: 2,
            waiting_user: 1,
            completed: 1,
            latest_title: 'Check failing specs',
            latest_status: 'waiting_user',
            latest_updated_at: '2026-03-20T12:05:00.000Z',
          },
          actions: { items: [] },
          updated_at: '2026-03-20T12:10:00.000Z',
        },
      },
      global: {
        plugins: [createTestI18n()],
      },
    })

    expect(wrapper.text()).toContain('4 subagents')
    expect(wrapper.text()).toContain('2 running')
    expect(wrapper.text()).toContain('1 waiting')
    expect(wrapper.text()).toContain('1 completed')
    expect(wrapper.text()).toContain('Check failing specs · waiting user')
  })

  it('localizes collapsed research details and shows the source inventory once expanded', async () => {
    const i18n = createTestI18n()
    i18n.global.locale.value = 'zh-CN'

    const wrapper = mount(UserTaskProjectionCard, {
      props: {
        collapseByDefault: true,
        task: {
          id: 'task-research-1',
          kind: 'research',
          scope: 'current',
          conversation_id: 'conv-1',
          title: 'Agent memory systems',
          subtitle: 'completed·completed·需要补足概览证据',
          status: 'completed',
          stage: 'completed',
          progress: 100,
          result_preview: '谨慎结论：请结合原始来源再次核验。',
          research_sources: [
            {
              title: 'MemGPT paper',
              url: 'https://example.com/memgpt',
              domain: 'example.com',
              source_type: 'paper',
              published_at: '2026-03-20T00:00:00.000Z',
              relevance_score: 0.98,
              credibility_score: 0.95,
            },
          ],
          actions: { items: [] },
          updated_at: '2026-03-20T12:00:00.000Z',
        },
      },
      global: {
        plugins: [i18n],
      },
    })

    expect(wrapper.text()).toContain('Agent memory systems')

    await wrapper.get('button').trigger('click')

    expect((wrapper.text().match(/Agent memory systems/g) || []).length).toBe(1)
    expect(wrapper.text()).toContain('已完成 · 已完成 · 需要补足概览证据')
    expect(wrapper.text()).toContain('来源清单')
    expect(wrapper.get('a[href="https://example.com/memgpt"]').isVisible()).toBe(true)
    expect(wrapper.text()).toContain('MemGPT paper')
    expect(wrapper.text()).toContain('谨慎结论：请结合原始来源再次核验。')
  })

  it('renders workflow-specific affordances and emits resume', async () => {
    const wrapper = mount(UserTaskProjectionCard, {
      props: {
        task: {
          id: 'task-workflow-1',
          kind: 'workflow',
          scope: 'current',
          conversation_id: 'conv-9',
          title: '',
          subtitle: 'Nightly Sync • pause_for_approval',
          status: 'waiting_user',
          stage: 'waiting_user',
          progress: 63,
          actions: {
            items: [
              {
                id: 'resume',
                label: 'Resume workflow',
                method: 'POST',
                path: '/tasks/task-workflow-1/actions/resume',
                requires_input: true,
              },
            ],
          },
          updated_at: '2026-03-20T12:00:00.000Z',
        },
      },
      global: {
        plugins: [createTestI18n()],
      },
    })

    expect(wrapper.text()).toContain('Workflow task')
    expect(wrapper.text()).toContain('Workflow')
    expect(wrapper.text()).toContain('Resume workflow')

    await wrapper.get('button[class*="border-blue-200"]').trigger('click')

    expect(wrapper.emitted('action')?.[0]).toEqual([
      expect.objectContaining({ id: 'task-workflow-1' }),
      'resume',
    ])
  })
})
