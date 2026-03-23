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
          taskStageFailed: 'Failed',
          taskStageCancelled: 'Cancelled',
          taskKindResearch: 'Research',
          taskKindAgent: 'Agent',
          taskOpenConversation: 'Open conversation',
          taskCancel: 'Cancel',
          taskSendUpdatePlaceholder: 'Send an update to this task',
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
            can_cancel: true,
            can_open_chat: false,
            can_send_update: true,
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

    await wrapper.get('input').setValue('Keep the final answer concise')
    await wrapper.get('button[class*="bg-blue-600"]').trigger('click')
    await wrapper.get('button[class*="border-rose-200"]').trigger('click')

    expect(wrapper.emitted('message')?.[0]).toEqual(['task-1', 'Keep the final answer concise'])
    expect(wrapper.emitted('cancel')?.[0]).toEqual(['task-1'])
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
          actions: {
            can_cancel: false,
            can_open_chat: true,
            can_send_update: false,
          },
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
})
