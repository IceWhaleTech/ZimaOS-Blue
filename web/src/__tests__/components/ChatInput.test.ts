import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import ChatInput from '@/components/ChatInput.vue'
import { i18n } from '@/i18n'

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

describe('ChatInput cancel affordance', () => {
  beforeEach(() => {
    Object.defineProperty(window, 'innerWidth', { value: 1280, writable: true, configurable: true })
    Object.defineProperty(window.navigator, 'userAgent', { value: 'desktop', configurable: true })
    Object.defineProperty(window.navigator, 'maxTouchPoints', { value: 0, configurable: true })
    localStorageMock.clear()
  })

  it('shows cancel when canCancel is true without streaming', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)

    const wrapper = mount(ChatInput, {
      shallow: true,
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

    await wrapper.vm.$nextTick()

    const cancelButton = wrapper
      .findAll('button')
      .find((button) => button.classes().includes('desktop-cancel-btn'))
    expect(cancelButton?.exists()).toBe(true)

    await wrapper.find('textarea').setValue('hello')
    const sendButton = wrapper
      .findAll('button')
      .find((button) => button.classes().includes('chat-send-btn'))
    expect(sendButton?.exists()).toBe(true)
    await sendButton!.trigger('click')

    expect(wrapper.emitted('send')).toHaveLength(1)
    expect(wrapper.emitted('inject')).toBeUndefined()
  })

  it('restores draft message from localStorage on mount', async () => {
    localStorage.setItem('zima.chat.input_draft.v1', 'cached draft message')

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
    expect((textarea.element as HTMLTextAreaElement).value).toBe('cached draft message')
  })

  it('persists draft while typing and clears draft after send', async () => {
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
    await textarea.setValue('message to cache')
    expect(localStorage.getItem('zima.chat.input_draft.v1')).toBe('message to cache')

    const sendButton = wrapper
      .findAll('button')
      .find((button) => button.classes().includes('chat-send-btn'))
    expect(sendButton?.exists()).toBe(true)
    await sendButton!.trigger('click')

    expect(wrapper.emitted('send')).toHaveLength(1)
    expect(localStorage.getItem('zima.chat.input_draft.v1')).toBeNull()
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

    expect(wrapper.find('.desktop-textarea-actions .desktop-inline-icon-btn.is-passive').exists()).toBe(
      false
    )
    expect(wrapper.find('.desktop-textarea-actions .chat-send-btn').exists()).toBe(true)
  })
})
