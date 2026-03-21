import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { nextTick, reactive } from 'vue'

let mockChatStore: any
const taskProjectionStoreMock = {
  refreshNow: vi.fn(),
}

vi.mock('@/stores/chat', () => ({
  useChatStore: () => mockChatStore,
}))

vi.mock('@/stores/taskProjections', () => ({
  useTaskProjectionsStore: () => taskProjectionStoreMock,
}))

import AskUserQuestionDialog from '@/components/AskUserQuestionDialog.vue'

function createTestI18n() {
  return createI18n({
    legacy: false,
    locale: 'en-US',
    fallbackLocale: 'en-US',
    messages: {
      'en-US': {
        askQuestion: {
          title: 'Confirmation',
          subtitle: 'Review before continuing',
          timeout: '{seconds}s',
          skip: 'Skip',
          submit: 'Submit',
          next: 'Next',
          step: '{current}/{total}',
          browserCheckpoint: {
            title: 'Browser Checkpoint',
            riskLevel: 'Risk',
            step: 'Step',
            action: 'Action',
            url: 'URL',
            screenshotAlt: 'Screenshot',
            continue: 'Continue',
            continueOnce: 'Allow This Time',
            continueDescription: 'Continue only for this step.',
            allowSite: 'Always Allow This Site',
            allowSiteDescription: 'Skip future prompts for {site}.',
            cancel: 'Cancel',
            cancelDescription: 'Stop this browser action.',
          },
        },
        execCard: {
          risk: {
            high: 'High',
          },
        },
      },
    },
  })
}

function createCheckpointQuestion() {
  return {
    id: 'ask-browser-checkpoint',
    expires_at: Date.now() + 60_000,
    context: {
      kind: 'browser_checkpoint',
      risk_level: 'high',
      step: 'click',
      action: 'Press the continue button',
      url: 'https://example.com/dashboard',
      site_origin: 'https://example.com',
    },
    questions: [
      {
        id: 'q1',
        question: 'Allow this browser action?',
        multi_select: false,
        options: [
          { label: 'Continue', value: 'continue' },
          { label: 'Allow site', value: 'allow_site' },
          { label: 'Cancel', value: 'cancel' },
        ],
      },
    ],
  }
}

function mountCheckpointDialog() {
  return mount(AskUserQuestionDialog, {
    global: {
      plugins: [createTestI18n()],
      stubs: {
        Teleport: true,
        Transition: false,
      },
    },
  })
}

describe('AskUserQuestionDialog browser checkpoint', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.spyOn(console, 'log').mockImplementation(() => {})
    taskProjectionStoreMock.refreshNow.mockReset().mockResolvedValue(undefined)
    mockChatStore = reactive({
      pendingQuestion: null,
      submitQuestionAnswers: vi.fn(),
      dismissQuestion: vi.fn(),
    })
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.clearAllTimers()
    vi.useRealTimers()
  })

  it('shows checkpoint shortcuts and submits allow_site with one click', async () => {
    const wrapper = mountCheckpointDialog()
    mockChatStore.pendingQuestion = createCheckpointQuestion()
    await nextTick()

    expect(wrapper.text()).toContain('Allow This Time')
    expect(wrapper.text()).toContain('Always Allow This Site')
    expect(wrapper.text()).toContain('Skip future prompts for https://example.com.')

    const allowSite = wrapper
      .findAll('label')
      .find((node) => node.text().includes('Always Allow This Site'))
    expect(allowSite).toBeTruthy()

    await allowSite!.trigger('click')

    expect(mockChatStore.submitQuestionAnswers).toHaveBeenCalledTimes(1)
    expect(mockChatStore.submitQuestionAnswers).toHaveBeenCalledWith([
      {
        question_id: 'q1',
        selected: ['allow_site'],
        other_text: '',
      },
    ])
    expect(mockChatStore.dismissQuestion).not.toHaveBeenCalled()

    mockChatStore.pendingQuestion = null
    await nextTick()
    wrapper.unmount()
  })

  it('maps the cancel action to a checkpoint answer instead of dismissing the dialog', async () => {
    const wrapper = mountCheckpointDialog()
    mockChatStore.pendingQuestion = createCheckpointQuestion()
    await nextTick()

    const cancelButton = wrapper.findAll('button').find((node) => node.text().trim() === 'Cancel')
    expect(cancelButton).toBeTruthy()

    await cancelButton!.trigger('click')

    expect(mockChatStore.submitQuestionAnswers).toHaveBeenCalledTimes(1)
    expect(mockChatStore.submitQuestionAnswers).toHaveBeenCalledWith([
      {
        question_id: 'q1',
        selected: ['cancel'],
        other_text: '',
      },
    ])
    expect(mockChatStore.dismissQuestion).not.toHaveBeenCalled()

    mockChatStore.pendingQuestion = null
    await nextTick()
    wrapper.unmount()
  })
})
