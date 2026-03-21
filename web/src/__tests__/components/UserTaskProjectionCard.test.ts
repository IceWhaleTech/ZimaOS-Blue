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
    expect(wrapper.text()).toContain('Waiting for your approval')
    expect(wrapper.text()).toContain('Drafted the plan')
    expect(wrapper.text()).toContain('Summary')

    await wrapper.get('input').setValue('Keep the final answer concise')
    await wrapper.get('button[class*="bg-blue-600"]').trigger('click')
    await wrapper.get('button[class*="border-rose-200"]').trigger('click')

    expect(wrapper.emitted('message')?.[0]).toEqual(['task-1', 'Keep the final answer concise'])
    expect(wrapper.emitted('cancel')?.[0]).toEqual(['task-1'])
  })
})
