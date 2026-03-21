import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'

import UserTaskProjectionDock from '@/components/UserTaskProjectionDock.vue'

const TASK_DOCK_COLLAPSED_KEY = 'zima.chat.task_projection_dock_collapsed.v1'

const localStorageMock = (() => {
  let store: Record<string, string> = {}
  return {
    getItem: (key: string) => (key in store ? store[key] : null),
    setItem: (key: string, value: string) => {
      store[key] = String(value)
    },
    removeItem: (key: string) => {
      delete store[key]
    },
    clear: () => {
      store = {}
    },
  }
})()

vi.stubGlobal('localStorage', localStorageMock)

function createTestI18n() {
  return createI18n({
    legacy: false,
    locale: 'en-US',
    fallbackLocale: 'en-US',
    messages: {
      'en-US': {
        chat: {
          backgroundTasks: 'Background tasks',
          taskStagePlanning: 'Planning',
          taskStageWorking: 'Working',
          taskStageVerifying: 'Verifying',
          taskStageWaiting: 'Waiting',
          taskRunningElsewhere: 'Track active work running in other conversations.',
          taskKindResearch: 'Research',
          taskKindAgent: 'Agent',
          taskBackToConversation: 'Back to task',
          taskCancel: 'Cancel',
        },
      },
    },
  })
}

function makeTasks() {
  return [
    {
      id: 'task-1',
      kind: 'research',
      scope: 'background',
      conversation_id: 'conv-1',
      title: 'Track regressions',
      status: 'running',
      stage: 'planning',
      progress: 48,
      actions: { can_cancel: true, can_open_chat: true, can_send_update: false },
      updated_at: '2026-03-20T12:00:00.000Z',
    },
    {
      id: 'task-2',
      kind: 'agent_task',
      scope: 'background',
      conversation_id: 'conv-2',
      title: 'Compare routing changes',
      status: 'waiting_user',
      stage: 'waiting_user',
      progress: 72,
      subtitle: 'Need your approval',
      actions: { can_cancel: true, can_open_chat: true, can_send_update: false },
      updated_at: '2026-03-20T11:58:00.000Z',
    },
  ]
}

describe('UserTaskProjectionDock', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('starts collapsed and expands to show background task details', async () => {
    const wrapper = mount(UserTaskProjectionDock, {
      props: {
        tasks: makeTasks(),
      },
      global: {
        plugins: [createTestI18n()],
      },
    })

    expect(wrapper.get('[aria-expanded]').attributes('aria-expanded')).toBe('false')
    expect(wrapper.text()).toContain('Background tasks')
    expect(wrapper.text()).toContain('Planning')
    expect(wrapper.text()).toContain('48%')
    expect(wrapper.text()).toContain('Track regressions')
    expect(wrapper.text()).not.toContain('Compare routing changes')

    await wrapper.get('[aria-expanded]').trigger('click')

    expect(wrapper.get('[aria-expanded]').attributes('aria-expanded')).toBe('true')
    expect(wrapper.text()).toContain('Compare routing changes')
    expect(wrapper.text()).toContain('Back to task')
  })

  it('restores expanded state and emits open/cancel events', async () => {
    localStorage.setItem(TASK_DOCK_COLLAPSED_KEY, '0')

    const wrapper = mount(UserTaskProjectionDock, {
      props: {
        tasks: makeTasks(),
      },
      global: {
        plugins: [createTestI18n()],
      },
    })

    expect(wrapper.get('[aria-expanded]').attributes('aria-expanded')).toBe('true')

    await wrapper.get('button[class*="border-slate-200"]').trigger('click')
    await wrapper.get('button[class*="border-amber-200"]').trigger('click')

    expect(wrapper.emitted('open')?.[0]?.[0]).toMatchObject({ id: 'task-1' })
    expect(wrapper.emitted('cancel')?.[0]).toEqual(['task-1'])
  })
})
