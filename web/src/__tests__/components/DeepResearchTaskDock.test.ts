import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'

import DeepResearchTaskDock from '@/components/DeepResearchTaskDock.vue'

const DEEP_RESEARCH_DOCK_COLLAPSED_KEY = 'zima.chat.deep_research_dock_collapsed.v1'

const mocks = vi.hoisted(() => ({
  deepResearchJobsStore: {
    activeJobs: [] as Array<Record<string, unknown>>,
    cancelJob: vi.fn(),
  },
}))

vi.mock('@/stores/deepResearchJobs', () => ({
  useDeepResearchJobsStore: () => mocks.deepResearchJobsStore,
}))

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
        common: {
          loading: 'Loading...',
        },
        chat: {
          deepResearchProgress: 'Deep Research Running',
          deepResearchStageIntake: 'Intake',
          deepResearchStagePlanning: 'Planning',
          deepResearchStageRetrieve: 'Retrieving',
          deepResearchStageVerify: 'Verifying',
          deepResearchStageSynthesize: 'Synthesizing',
          deepResearchRunningTasks: 'Running research tasks',
          deepResearchRunningElsewhere: 'Track active deep research jobs across conversations.',
          deepResearchIteration: 'Iteration',
          deepResearchBackToTask: 'View result',
          deepResearchCancelTask: 'Cancel',
        },
      },
    },
  })
}

function makeJobs() {
  return [
    {
      id: 'job-1',
      job_id: 'job-1',
      conversation_id: 'conv-1',
      query: 'Track session badge regressions',
      status: 'running',
      stage: 'planning',
      progress: 48,
      iteration: 1,
      latest_action: 'Reviewing recent routing changes',
      latest_gap: '',
      updated_at: '2026-03-18T10:00:00.000Z',
    },
    {
      id: 'job-2',
      job_id: 'job-2',
      conversation_id: 'conv-2',
      query: 'Compare browser grounding changes',
      status: 'running',
      stage: 'verify',
      progress: 72,
      iteration: 2,
      latest_action: 'Checking source credibility',
      latest_gap: 'Need one primary source',
      updated_at: '2026-03-18T09:55:00.000Z',
    },
  ]
}

describe('DeepResearchTaskDock', () => {
  beforeEach(() => {
    localStorage.clear()
    mocks.deepResearchJobsStore.cancelJob.mockReset()
    mocks.deepResearchJobsStore.activeJobs = makeJobs()
  })

  it('starts collapsed and expands to show active job details', async () => {
    const wrapper = mount(DeepResearchTaskDock, {
      global: {
        plugins: [createTestI18n()],
      },
    })

    expect(wrapper.get('[data-testid="deep-research-task-dock-toggle"]').attributes('aria-expanded')).toBe(
      'false'
    )
    expect(wrapper.find('[data-testid="deep-research-task-dock-details"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('Running research tasks')
    expect(wrapper.text()).toContain('Planning')
    expect(wrapper.text()).toContain('48%')
    expect(wrapper.text()).toContain('Track session badge regressions')
    expect(wrapper.text()).not.toContain('Compare browser grounding changes')

    await wrapper.get('[data-testid="deep-research-task-dock-toggle"]').trigger('click')

    expect(wrapper.get('[data-testid="deep-research-task-dock-toggle"]').attributes('aria-expanded')).toBe(
      'true'
    )
    expect(wrapper.text()).toContain('Track active deep research jobs across conversations.')
    expect(wrapper.text()).toContain('Compare browser grounding changes')
    expect(wrapper.text()).toContain('Need one primary source')
  })

  it('restores expanded state and keeps view and cancel actions working', async () => {
    localStorage.setItem(DEEP_RESEARCH_DOCK_COLLAPSED_KEY, '0')

    const wrapper = mount(DeepResearchTaskDock, {
      global: {
        plugins: [createTestI18n()],
      },
    })

    expect(wrapper.get('[data-testid="deep-research-task-dock-toggle"]').attributes('aria-expanded')).toBe(
      'true'
    )

    await wrapper.get('[data-testid="deep-research-task-dock-view-job-1"]').trigger('click')
    expect(wrapper.emitted('view')).toBeTruthy()
    expect(wrapper.emitted('view')?.[0]?.[0]).toMatchObject({ job_id: 'job-1' })

    await wrapper.get('[data-testid="deep-research-task-dock-cancel-job-2"]').trigger('click')
    expect(mocks.deepResearchJobsStore.cancelJob).toHaveBeenCalledWith('job-2')
  })
})
