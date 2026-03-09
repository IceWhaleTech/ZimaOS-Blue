import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import ChatInput from '@/components/ChatInput.vue'
import { i18n } from '@/i18n'

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

    const cancelButton = wrapper.findAll('button').find(button => button.classes().includes('border-red-500/30'))
    expect(cancelButton?.exists()).toBe(true)

    await wrapper.find('textarea').setValue('hello')
    const sendButton = wrapper.findAll('button').find(button => button.classes().includes('chat-send-btn'))
    expect(sendButton?.exists()).toBe(true)
    await sendButton!.trigger('click')

    expect(wrapper.emitted('send')).toHaveLength(1)
    expect(wrapper.emitted('inject')).toBeUndefined()
  })
})
