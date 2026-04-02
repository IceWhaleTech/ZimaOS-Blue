import { describe, it, expect, beforeEach, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import ChatInput from '@/components/ChatInput.vue'
import { skillApi } from '@/api/skill'
import { i18n } from '@/i18n'
import { useSettingsStore } from '@/stores/settings'

const { routerPushMock } = vi.hoisted(() => ({
  routerPushMock: vi.fn(),
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
vi.stubGlobal('indexedDB', undefined)

vi.mock('@/api/voice', () => ({
  AudioRecorder: vi.fn(),
  voiceApi: {
    transcribe: vi.fn(),
  },
}))

vi.mock('@/api/speech', () => ({
  speechApi: {
    getStatus: vi.fn(),
    transcribe: vi.fn(),
  },
}))

vi.mock('@/utils/audioConverter', () => ({
  convertToWav: vi.fn(),
}))

vi.mock('@/utils/vad', () => ({
  EnergyVAD: vi.fn(),
}))

vi.mock('@/composables/useFeatureIntent', () => ({
  classifyFeatureIntent: vi.fn(() => ({ deepResearch: false, agentMode: false })),
}))

vi.mock('@/api/skill', () => ({
  skillApi: {
    adviseMarket: vi.fn(),
  },
}))

vi.mock('vue-router', async () => {
  const actual = await vi.importActual<typeof import('vue-router')>('vue-router')
  return {
    ...actual,
    useRouter: () => ({
      push: routerPushMock,
    }),
  }
})

async function settleComposer(wrapper: ReturnType<typeof mount>) {
  await wrapper.vm.$nextTick()
  await Promise.resolve()
  await new Promise((resolve) => setTimeout(resolve, 0))
  await wrapper.vm.$nextTick()
}

async function attachDraftFile(wrapper: ReturnType<typeof mount>, file: File) {
  const fileInput = wrapper.findAll('input[type="file"]')[0]
  if (!fileInput) {
    throw new Error('File input not found')
  }

  Object.defineProperty(fileInput.element, 'files', {
    configurable: true,
    value: [file],
  })

  await fileInput.trigger('change')
  await settleComposer(wrapper)
}

async function waitFor(check: () => boolean, attempts = 10) {
  for (let index = 0; index < attempts; index += 1) {
    if (check()) return
    await Promise.resolve()
    await new Promise((resolve) => setTimeout(resolve, 10))
  }
  throw new Error('Timed out waiting for condition')
}

describe('ChatInput cancel affordance', () => {
  beforeEach(() => {
    Object.defineProperty(window, 'innerWidth', { value: 1280, writable: true, configurable: true })
    Object.defineProperty(window.navigator, 'userAgent', { value: 'desktop', configurable: true })
    Object.defineProperty(window.navigator, 'maxTouchPoints', { value: 0, configurable: true })
    localStorageMock.clear()
    vi.clearAllMocks()
    vi.useRealTimers()
    routerPushMock.mockReset()
    vi.mocked(skillApi.adviseMarket).mockResolvedValue({
      data: {
        query: '',
        need_store_search: false,
      },
    } as never)
    i18n.global.setLocaleMessage('en-US', {
      common: {
        enabled: 'Enabled',
        disabled: 'Disabled',
        cancel: 'Cancel',
      },
      agent: {
        autoConfirm: 'Auto-confirm',
      },
      ui: {
        deepResearchTitle: 'Deep Research',
      },
      harness: {
        quickEval: {
          researchLabel: 'Research',
        },
      },
      uiReview: {
        visual: 'Visual',
        accessibility: 'Accessibility',
      },
      chat: {
        taskLoop: 'Ralph Loop',
        attachFile: 'Attach file',
        takePhoto: 'Take photo',
        moreActions: 'More actions',
        inputPlaceholder: 'Type a message...',
        inputPlaceholderShort: 'Type a message...',
        send: 'Send',
        startRecording: 'Start voice recording',
        stopRecording: 'Stop voice recording',
        switchToKeyboard: 'Switch to keyboard',
        switchToVoice: 'Switch to voice',
        startDictation: 'Voice input',
        stopDictation: 'Stop voice input',
        recording: 'Recording...',
        talkMode: {
          title: 'Talk Mode',
        },
        routingMode: {
          title: 'Routing',
        },
        analyzeReportShortcutTitle: 'Analysis Report',
        analyzeReportPrompt:
          'Create a structured analysis report.\n- Topic:\n- URLs, files, or input text:\n- Key questions, comparisons, or decisions to cover:',
        uiReviewShortcutTitle: 'UI Review',
        uiReviewPrompt:
          'Please run a UI review and return clear findings plus improvement suggestions.\n- Page URL or screenshot:\n- Target device: desktop / mobile\n- Focus areas: visual hierarchy, interaction flow, accessibility',
      },
    } as never)
    i18n.global.locale.value = 'en-US'
  })

  it('keeps send available when canCancel is true without streaming', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)

    const wrapper = mount(ChatInput, {
      props: {
        streaming: false,
        canCancel: true,
      },
      global: {
        plugins: [pinia, i18n],
        stubs: {
          ImagePreview: true,
          ModelDownloadPrompt: true,
        },
      },
    })

    await settleComposer(wrapper)

    await wrapper.find('textarea').setValue('hello')
    const sendButton = wrapper
      .findAll('button')
      .find((button) => button.classes().includes('chat-send-btn'))
    expect(sendButton?.exists()).toBe(true)
    await sendButton!.trigger('click')

    expect(wrapper.emitted('send')).toHaveLength(1)
    expect(wrapper.emitted('inject')).toBeUndefined()
  })

  it('hides the inline cancel affordance when showInlineCancel is false', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)

    const wrapper = mount(ChatInput, {
      props: {
        streaming: false,
        canCancel: true,
        showInlineCancel: false,
      },
      global: {
        plugins: [pinia, i18n],
        stubs: {
          ImagePreview: true,
          ModelDownloadPrompt: true,
        },
      },
    })

    await settleComposer(wrapper)

    const cancelButton = wrapper
      .findAll('button')
      .find((button) => button.attributes('title') === 'Cancel')
    expect(cancelButton).toBeUndefined()
  })

  it('restores draft message for the current conversation on mount', async () => {
    localStorage.setItem('zima.chat.input_draft.v2:conv-1', 'cached draft message')

    const pinia = createPinia()
    setActivePinia(pinia)

    const wrapper = mount(ChatInput, {
      shallow: true,
      props: {
        conversationId: 'conv-1',
      },
      global: {
        plugins: [pinia, i18n],
        stubs: {
          ImagePreview: true,
          ModelDownloadPrompt: true,
        },
      },
    })

    await wrapper.vm.$nextTick()
    await wrapper.vm.$nextTick()

    const textarea = wrapper.find('textarea')
    expect((textarea.element as HTMLTextAreaElement).value).toBe('cached draft message')
  })

  it('restores the legacy draft slot for a new chat', async () => {
    localStorage.setItem('zima.chat.input_draft.v1', 'legacy cached draft')

    const pinia = createPinia()
    setActivePinia(pinia)

    const wrapper = mount(ChatInput, {
      shallow: true,
      global: {
        plugins: [pinia, i18n],
        stubs: {
          ImagePreview: true,
          ModelDownloadPrompt: true,
        },
      },
    })

    await wrapper.vm.$nextTick()
    await wrapper.vm.$nextTick()

    const textarea = wrapper.find('textarea')
    expect((textarea.element as HTMLTextAreaElement).value).toBe('legacy cached draft')
    expect(localStorage.getItem('zima.chat.input_draft.v2:__new__')).toBe('legacy cached draft')
    expect(localStorage.getItem('zima.chat.input_draft.v1')).toBeNull()
  })

  it('persists draft while typing and clears draft after send', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)

    const wrapper = mount(ChatInput, {
      shallow: true,
      props: {
        conversationId: 'conv-a',
      },
      global: {
        plugins: [pinia, i18n],
        stubs: {
          ImagePreview: true,
          ModelDownloadPrompt: true,
        },
      },
    })

    const textarea = wrapper.find('textarea')
    await textarea.setValue('message to cache')
    expect(localStorage.getItem('zima.chat.input_draft.v2:conv-a')).toBe('message to cache')

    const sendButton = wrapper
      .findAll('button')
      .find((button) => button.classes().includes('chat-send-btn'))
    expect(sendButton?.exists()).toBe(true)
    await sendButton!.trigger('click')

    expect(wrapper.emitted('send')).toHaveLength(1)
    expect(localStorage.getItem('zima.chat.input_draft.v2:conv-a')).toBeNull()
  })

  it('keeps drafts isolated when switching conversations', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)

    const wrapper = mount(ChatInput, {
      shallow: true,
      props: {
        conversationId: 'conv-a',
      },
      global: {
        plugins: [pinia, i18n],
        stubs: {
          ImagePreview: true,
          ModelDownloadPrompt: true,
        },
      },
    })

    const textarea = wrapper.find('textarea')
    await textarea.setValue('draft for a')
    expect(localStorage.getItem('zima.chat.input_draft.v2:conv-a')).toBe('draft for a')

    await wrapper.setProps({ conversationId: 'conv-b' })
    await wrapper.vm.$nextTick()
    await wrapper.vm.$nextTick()

    expect((wrapper.find('textarea').element as HTMLTextAreaElement).value).toBe('')

    await wrapper.find('textarea').setValue('draft for b')
    expect(localStorage.getItem('zima.chat.input_draft.v2:conv-b')).toBe('draft for b')

    await wrapper.setProps({ conversationId: 'conv-a' })
    await wrapper.vm.$nextTick()
    await wrapper.vm.$nextTick()

    expect((wrapper.find('textarea').element as HTMLTextAreaElement).value).toBe('draft for a')
  })

  it('restores attachment drafts for the current conversation on remount', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)

    const wrapper = mount(ChatInput, {
      shallow: true,
      props: {
        conversationId: 'conv-files',
      },
      global: {
        plugins: [pinia, i18n],
        stubs: {
          ImagePreview: true,
          ModelDownloadPrompt: true,
        },
      },
    })

    await attachDraftFile(
      wrapper,
      new File(['draft attachment'], 'draft-notes.txt', { type: 'text/plain' })
    )
    expect(wrapper.text()).toContain('draft-notes.txt')

    wrapper.unmount()

    const restoredWrapper = mount(ChatInput, {
      shallow: true,
      props: {
        conversationId: 'conv-files',
      },
      global: {
        plugins: [pinia, i18n],
        stubs: {
          ImagePreview: true,
          ModelDownloadPrompt: true,
        },
      },
    })

    await settleComposer(restoredWrapper)
    await waitFor(() => restoredWrapper.text().includes('draft-notes.txt'))

    expect(restoredWrapper.text()).toContain('draft-notes.txt')
  })

  it('keeps attachment drafts isolated when switching conversations', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)

    const wrapper = mount(ChatInput, {
      shallow: true,
      props: {
        conversationId: 'conv-a',
      },
      global: {
        plugins: [pinia, i18n],
        stubs: {
          ImagePreview: true,
          ModelDownloadPrompt: true,
        },
      },
    })

    await attachDraftFile(wrapper, new File(['a'], 'alpha.txt', { type: 'text/plain' }))
    expect(wrapper.text()).toContain('alpha.txt')

    await wrapper.setProps({ conversationId: 'conv-b' })
    await settleComposer(wrapper)

    expect(wrapper.text()).not.toContain('alpha.txt')

    await attachDraftFile(wrapper, new File(['b'], 'bravo.txt', { type: 'text/plain' }))
    expect(wrapper.text()).toContain('bravo.txt')

    await wrapper.setProps({ conversationId: 'conv-a' })
    await settleComposer(wrapper)

    expect(wrapper.text()).toContain('alpha.txt')
    expect(wrapper.text()).not.toContain('bravo.txt')
  })

  it('clears attachment drafts after send', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)

    const wrapper = mount(ChatInput, {
      shallow: true,
      props: {
        conversationId: 'conv-send',
      },
      global: {
        plugins: [pinia, i18n],
        stubs: {
          ImagePreview: true,
          ModelDownloadPrompt: true,
        },
      },
    })

    await attachDraftFile(wrapper, new File(['payload'], 'to-send.txt', { type: 'text/plain' }))

    const sendButton = wrapper
      .findAll('button')
      .find((button) => button.classes().includes('chat-send-btn'))
    expect(sendButton?.exists()).toBe(true)

    await sendButton!.trigger('click')
    await settleComposer(wrapper)

    const sentAttachments = wrapper.emitted('send')?.[0]?.[1] as Array<{ name: string }> | undefined
    expect(sentAttachments?.map((attachment) => attachment.name)).toEqual(['to-send.txt'])

    wrapper.unmount()

    const restoredWrapper = mount(ChatInput, {
      shallow: true,
      props: {
        conversationId: 'conv-send',
      },
      global: {
        plugins: [pinia, i18n],
        stubs: {
          ImagePreview: true,
          ModelDownloadPrompt: true,
        },
      },
    })

    await settleComposer(restoredWrapper)

    expect(restoredWrapper.text()).not.toContain('to-send.txt')
  })

  it('does not show a loading spinner next to send while typed text is ready to send', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)

    const wrapper = mount(ChatInput, {
      shallow: true,
      global: {
        plugins: [pinia, i18n],
        stubs: {
          ImagePreview: true,
          ModelDownloadPrompt: true,
        },
      },
    })

    await wrapper.find('textarea').setValue('this should only show the send button')

    expect(
      wrapper.find('.desktop-textarea-actions .desktop-inline-icon-btn.is-passive').exists()
    ).toBe(false)
    expect(wrapper.find('.desktop-textarea-actions .chat-send-btn').exists()).toBe(true)
  })

  it('grows the desktop composer height for multiline input', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)

    const wrapper = mount(ChatInput, {
      shallow: true,
      global: {
        plugins: [pinia, i18n],
        stubs: {
          ImagePreview: true,
          ModelDownloadPrompt: true,
        },
      },
    })

    const textarea = wrapper.find('textarea')
    Object.defineProperty(textarea.element, 'scrollHeight', {
      configurable: true,
      value: 96,
    })

    await textarea.setValue('first line\nsecond line')

    expect((textarea.element as HTMLTextAreaElement).style.height).toBe('96px')
    expect((textarea.element as HTMLTextAreaElement).style.overflowY).toBe('hidden')
  })

  it('shows skill guidance for natural-language tasks and opens the skill store with the suggested query', async () => {
    vi.useFakeTimers()
    vi.mocked(skillApi.adviseMarket).mockResolvedValue({
      data: {
        query: 'need a github release workflow with changelog generation',
        need_store_search: true,
        search_queries: ['github actions release automation', 'release changelog generator'],
        capability_tags: ['github-actions', 'release-management'],
        recommended_ids: ['release-bot'],
        results: [
          {
            score: 0.97,
            skill: {
              id: 'release-bot',
              name: 'Release Bot',
            },
          },
        ],
      },
    } as never)

    const pinia = createPinia()
    setActivePinia(pinia)

    const wrapper = mount(ChatInput, {
      shallow: true,
      global: {
        plugins: [pinia, i18n],
        stubs: {
          ImagePreview: true,
          ModelDownloadPrompt: true,
        },
      },
    })

    await wrapper
      .find('textarea')
      .setValue('need a github release workflow with changelog generation')
    await vi.advanceTimersByTimeAsync(700)
    await flushPromises()

    expect(skillApi.adviseMarket).toHaveBeenCalledWith({
      query: 'need a github release workflow with changelog generation',
    })
    expect(wrapper.find('.chat-skill-advice').exists()).toBe(true)
    expect(wrapper.find('.chat-skill-advice').text()).toContain('github actions release automation')
    expect(wrapper.find('.chat-skill-advice').text()).toContain('github-actions')
    expect(wrapper.find('.chat-skill-advice').text()).toContain('Release Bot')

    const queryChip = wrapper.findAll('.chat-skill-advice__query-chip')[0]
    expect(queryChip?.exists()).toBe(true)
    await queryChip!.trigger('click')

    expect(routerPushMock).toHaveBeenCalledWith({
      name: 'Plugins',
      query: {
        tab: 'store',
        q: 'github actions release automation',
      },
    })
  })

  it('toggles Ralph Loop auto-confirm on context menu', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const settingsStore = useSettingsStore()
    const setAgentAutoConfirm = vi
      .spyOn(settingsStore, 'setAgentAutoConfirm')
      .mockResolvedValue(undefined)

    const wrapper = mount(ChatInput, {
      shallow: true,
      global: {
        plugins: [pinia, i18n],
        stubs: {
          ImagePreview: true,
          ModelDownloadPrompt: true,
        },
      },
    })

    await wrapper.vm.$nextTick()

    const taskLoopButton = wrapper.find('button.mode-chip-loop')
    expect(taskLoopButton.exists()).toBe(true)

    await taskLoopButton.trigger('contextmenu')

    expect(setAgentAutoConfirm).toHaveBeenCalledWith(true)
  })

  it('fills the composer with the analysis report template when the shortcut is clicked', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)

    const wrapper = mount(ChatInput, {
      shallow: true,
      global: {
        plugins: [pinia, i18n],
        stubs: {
          ImagePreview: true,
          ModelDownloadPrompt: true,
        },
      },
    })

    await wrapper.vm.$nextTick()

    const reportButton = wrapper.find('button.mode-chip-report')
    expect(reportButton.exists()).toBe(true)

    await reportButton.trigger('click')

    const textarea = wrapper.find('textarea')
    expect((textarea.element as HTMLTextAreaElement).value).toContain(
      'Create a structured analysis report.'
    )
    expect((textarea.element as HTMLTextAreaElement).value).toContain('- Topic:')
  })

  it('fills the composer with the UI review template when the shortcut is clicked', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)

    const wrapper = mount(ChatInput, {
      shallow: true,
      global: {
        plugins: [pinia, i18n],
        stubs: {
          ImagePreview: true,
          ModelDownloadPrompt: true,
        },
      },
    })

    await wrapper.vm.$nextTick()

    const uiReviewButton = wrapper.find('button.mode-chip-ui')
    expect(uiReviewButton.exists()).toBe(true)

    await uiReviewButton.trigger('click')

    const textarea = wrapper.find('textarea')
    expect((textarea.element as HTMLTextAreaElement).value).toContain(
      'Please run a UI review and return clear findings plus improvement suggestions.'
    )
    expect((textarea.element as HTMLTextAreaElement).value).toContain('- Page URL or screenshot:')
  })

  it('shows a compact info card for the analysis report shortcut', async () => {
    Object.defineProperty(window, 'innerWidth', { value: 390, writable: true, configurable: true })
    Object.defineProperty(window.navigator, 'userAgent', { value: 'iphone', configurable: true })

    const pinia = createPinia()
    setActivePinia(pinia)

    const wrapper = mount(ChatInput, {
      shallow: true,
      global: {
        plugins: [pinia, i18n],
        stubs: {
          ImagePreview: true,
          ModelDownloadPrompt: true,
        },
      },
    })

    await wrapper.vm.$nextTick()

    const infoButton = wrapper.find('.compact-mode-info-toggle--report')
    expect(infoButton.exists()).toBe(true)

    await infoButton.trigger('click')

    expect(wrapper.find('.compact-mode-info-card--report').exists()).toBe(true)
    expect(wrapper.text()).toContain('Prompt template')
    expect(wrapper.text()).toContain('Analysis Report')
  })

  it('shows a compact info card for deep research', async () => {
    Object.defineProperty(window, 'innerWidth', { value: 390, writable: true, configurable: true })
    Object.defineProperty(window.navigator, 'userAgent', { value: 'iphone', configurable: true })

    const pinia = createPinia()
    setActivePinia(pinia)

    const wrapper = mount(ChatInput, {
      shallow: true,
      global: {
        plugins: [pinia, i18n],
        stubs: {
          ImagePreview: true,
          ModelDownloadPrompt: true,
        },
      },
    })

    await wrapper.vm.$nextTick()

    const infoButton = wrapper.find('.compact-mode-info-toggle--research')
    expect(infoButton.exists()).toBe(true)

    await infoButton.trigger('click')

    expect(wrapper.find('.compact-mode-info-card--research').exists()).toBe(true)
    expect(wrapper.text()).toContain('Research')
    expect(wrapper.text()).toContain('Disabled')
  })

  it('shows a compact info card for Ralph Loop', async () => {
    Object.defineProperty(window, 'innerWidth', { value: 390, writable: true, configurable: true })
    Object.defineProperty(window.navigator, 'userAgent', { value: 'iphone', configurable: true })

    const pinia = createPinia()
    setActivePinia(pinia)

    const wrapper = mount(ChatInput, {
      shallow: true,
      global: {
        plugins: [pinia, i18n],
        stubs: {
          ImagePreview: true,
          ModelDownloadPrompt: true,
        },
      },
    })

    await wrapper.vm.$nextTick()

    const infoButton = wrapper.find('.compact-mode-info-toggle--loop')
    expect(infoButton.exists()).toBe(true)

    await infoButton.trigger('click')

    expect(wrapper.find('.compact-mode-info-card--loop').exists()).toBe(true)
    expect(wrapper.text()).toContain('Ralph Loop')
    expect(wrapper.text()).toContain('Plan')
  })

  it('shows a compact info card for the UI review shortcut', async () => {
    Object.defineProperty(window, 'innerWidth', { value: 390, writable: true, configurable: true })
    Object.defineProperty(window.navigator, 'userAgent', { value: 'iphone', configurable: true })

    const pinia = createPinia()
    setActivePinia(pinia)

    const wrapper = mount(ChatInput, {
      shallow: true,
      global: {
        plugins: [pinia, i18n],
        stubs: {
          ImagePreview: true,
          ModelDownloadPrompt: true,
        },
      },
    })

    await wrapper.vm.$nextTick()

    const infoButton = wrapper.find('.compact-mode-info-toggle--ui')
    expect(infoButton.exists()).toBe(true)

    await infoButton.trigger('click')

    expect(wrapper.find('.compact-mode-info-card--ui').exists()).toBe(true)
    expect(wrapper.text()).toContain('UI Review')
    expect(wrapper.text()).toContain('Visual')
  })
})
