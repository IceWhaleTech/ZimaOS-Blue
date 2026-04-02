import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'

import ChatActivityDock from '@/components/ChatActivityDock.vue'
import type { StreamUIState } from '@/stores/chat'
import type { UserTaskProjection } from '@/api/tasks'

function createTestI18n() {
  return createI18n({
    legacy: false,
    locale: 'en-US',
    fallbackLocale: 'en-US',
    messages: {
      'en-US': {
        common: {
          retry: 'Retry',
          dismiss: 'Dismiss',
        },
        chat: {
          activityDock: 'Activity',
          activityDockIdle: 'Ready',
          waitingThinking: 'Thinking',
          stopGenerating: 'Stop generating',
          currentTasks: 'Current tasks',
          backgroundTasks: 'Background tasks',
          currentTaskSingular: 'current task',
          currentTaskPlural: 'current tasks',
          streamConnecting: 'Connecting',
          streamStreaming: 'Streaming',
          streamExecuting: 'Executing',
          streamRecovering: 'Recovering',
          streamAwaitingConfirmation: 'Waiting',
          streamInterrupted: 'Interrupted',
          awaitingConfirmation: 'Waiting for your confirmation to continue',
          activeTodoTitle: 'Active checklist',
          activeTodoProgressFallback: '{completed} out of {total} tasks completed',
          activeTodo: {
            jumpToMessage: 'Jump to checklist message',
            expand: 'Expand todo list',
            collapse: 'Collapse todo list',
          },
          recentOutcome: 'Latest result',
          sourcesLabel: 'sources',
          taskBackToConversation: 'Back to task',
          taskKindAgent: 'Agent',
          taskKindResearch: 'Deep Research',
          taskKindWorkflow: 'Workflow',
          taskStagePlanning: 'Planning',
          taskStageWorking: 'Working',
          taskStageVerifying: 'Verifying',
          taskStageWaiting: 'Waiting',
          taskStageCompleted: 'Completed',
          taskStagePartial: 'Partially passed',
          taskStageFailed: 'Failed',
          taskStageCancelled: 'Cancelled',
          taskWaitingForApproval: 'Waiting for your approval',
          taskWaitingForAnswer: 'Waiting for your answer',
        },
      },
    },
  })
}

function makeStreamState(overrides: Partial<StreamUIState> = {}): StreamUIState {
  return {
    phase: 'idle',
    label: null,
    detail: null,
    updatedAt: Date.now(),
    recoveryAttempt: 0,
    canRetry: false,
    ...overrides,
  }
}

function makeTask(overrides: Partial<UserTaskProjection> = {}): UserTaskProjection {
  return {
    id: 'task-1',
    kind: 'agent_task',
    scope: 'current',
    title: 'Ship the migration',
    status: 'running',
    stage: 'working',
    progress: 42,
    actions: { items: [] },
    updated_at: '2026-04-02T00:00:00.000Z',
    ...overrides,
  }
}

describe('ChatActivityDock', () => {
  it('prioritizes interrupted stream status above task and checklist summaries', () => {
    const wrapper = mount(ChatActivityDock, {
      props: {
        streamState: makeStreamState({
          phase: 'interrupted',
          label: 'Connection lost',
          detail: 'Trying to recover the session',
          canRetry: true,
        }),
        canStop: true,
        currentTasks: [
          makeTask({
            status: 'waiting_user',
            stage: 'waiting_user',
            title: 'Approve the workflow',
          }),
        ],
        backgroundTasks: [makeTask({ id: 'bg-1', scope: 'background', title: 'Background sync' })],
        recentOutcome: makeTask({ id: 'done-1', status: 'completed', stage: 'completed' }),
        todoSummary: {
          messageId: 'msg-1',
          todoCardId: 'todo-1',
          completedCount: 1,
          totalCount: 3,
          allCompleted: false,
          items: [
            { text: 'Check the interrupted stream', checked: true },
            { text: 'Retry the request', checked: false },
          ],
        },
      },
      global: {
        plugins: [createTestI18n()],
        stubs: {
          UserTaskProjectionCard: {
            props: ['task'],
            template: '<div class="task-card-stub">{{ task.title }}</div>',
          },
        },
      },
    })

    const header = wrapper.get('.chat-activity-dock__header')
    expect(header.text()).toContain('Interrupted')
    expect(header.text()).toContain('Connection lost')
    expect(header.text()).toContain('Trying to recover the session')
    expect(header.text()).not.toContain('Approve the workflow')
  })

  it('falls back to the waiting current task before background, outcome, and checklist summaries', () => {
    const wrapper = mount(ChatActivityDock, {
      props: {
        streamState: makeStreamState({
          phase: 'streaming',
          label: 'Thinking',
        }),
        canStop: true,
        currentTasks: [
          makeTask({
            id: 'wait-1',
            status: 'waiting_user',
            stage: 'waiting_user',
            title: 'Approve the workflow',
            blocker: {
              kind: 'approval',
              label: 'Waiting for your approval',
              pending_count: 1,
              modal_only: true,
            },
          }),
        ],
        backgroundTasks: [makeTask({ id: 'bg-1', scope: 'background', title: 'Background sync' })],
        recentOutcome: makeTask({ id: 'done-1', status: 'completed', stage: 'completed' }),
        todoSummary: {
          messageId: 'msg-1',
          todoCardId: 'todo-1',
          completedCount: 1,
          totalCount: 3,
          allCompleted: false,
          items: [{ text: 'Review the approval request', checked: false }],
        },
      },
      global: {
        plugins: [createTestI18n()],
        stubs: {
          UserTaskProjectionCard: {
            props: ['task'],
            template: '<div class="task-card-stub">{{ task.title }}</div>',
          },
        },
      },
    })

    const header = wrapper.get('.chat-activity-dock__header')
    expect(header.text()).toContain('Approve the workflow')
    expect(header.text()).toContain('Waiting for your approval')
    expect(header.text()).not.toContain('Background sync')
  })

  it('shows a single stop button plus retry when interrupted and emits dock actions', async () => {
    const wrapper = mount(ChatActivityDock, {
      props: {
        streamState: makeStreamState({
          phase: 'interrupted',
          label: 'Connection lost',
          canRetry: true,
        }),
        canStop: true,
        currentTasks: [],
        backgroundTasks: [],
      },
      global: {
        plugins: [createTestI18n()],
        stubs: {
          UserTaskProjectionCard: true,
        },
      },
    })

    expect(wrapper.findAll('[data-testid="chat-activity-dock-stop"]')).toHaveLength(1)
    expect(wrapper.get('[data-testid="chat-activity-dock-retry"]').exists()).toBe(true)

    await wrapper.get('[data-testid="chat-activity-dock-stop"]').trigger('click')
    await wrapper.get('[data-testid="chat-activity-dock-retry"]').trigger('click')

    expect(wrapper.emitted('cancel')).toHaveLength(1)
    expect(wrapper.emitted('retry')).toHaveLength(1)
  })

  it('renders grouped tasks, a dismissable recent outcome, and collapsible todo actions', async () => {
    const wrapper = mount(ChatActivityDock, {
      props: {
        expanded: true,
        streamState: makeStreamState(),
        canStop: false,
        currentTasks: [makeTask({ id: 'cur-1', title: 'Current audit' })],
        backgroundTasks: [makeTask({ id: 'bg-1', scope: 'background', title: 'Background audit' })],
        recentOutcome: makeTask({
          id: 'research-1',
          kind: 'research',
          title: 'Research wrap-up',
          status: 'completed',
          stage: 'completed',
          result_preview: 'Collected the latest citations.',
          research_sources: [
            { title: 'Source 1', url: 'https://example.com/1' },
            { title: 'Source 2', url: 'https://example.com/2' },
          ],
        }),
        todoSummary: {
          messageId: 'msg-1',
          todoCardId: 'todo-1',
          completedCount: 1,
          totalCount: 2,
          allCompleted: false,
          items: [
            { text: 'Collect sources', checked: true },
            { text: 'Summarize findings', checked: false },
          ],
        },
        todoCollapsed: false,
      },
      global: {
        plugins: [createTestI18n()],
        stubs: {
          UserTaskProjectionCard: {
            props: ['task'],
            template: '<div class="task-card-stub">{{ task.title }}</div>',
          },
        },
      },
    })

    expect(wrapper.get('[data-testid="chat-activity-dock-current"]').text()).toContain(
      'Current audit'
    )
    expect(wrapper.get('[data-testid="chat-activity-dock-background"]').text()).toContain(
      'Background audit'
    )
    expect(wrapper.get('[data-testid="chat-activity-dock-outcome"]').text()).toContain(
      'Research wrap-up'
    )
    expect(wrapper.get('[data-testid="chat-activity-dock-outcome"]').text()).toContain('2 sources')
    expect(wrapper.find('.chat-activity-dock__todo-list').exists()).toBe(true)

    await wrapper.get('[data-testid="chat-activity-dock-dismiss-outcome"]').trigger('click')
    await wrapper.get('[data-testid="chat-activity-dock-todo-jump"]').trigger('click')
    await wrapper.get('[data-testid="chat-activity-dock-todo-toggle"]').trigger('click')

    expect(wrapper.emitted('dismiss-outcome')).toHaveLength(1)
    expect(wrapper.emitted('todo-jump')).toHaveLength(1)
    expect(wrapper.emitted('todo-toggle')).toHaveLength(1)
  })
})
