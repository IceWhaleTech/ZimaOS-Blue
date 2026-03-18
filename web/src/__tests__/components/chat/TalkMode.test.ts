import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'
import TalkMode from '@/components/chat/TalkMode.vue'
import { i18n } from '@/i18n'

const mocks = vi.hoisted(() => ({
  latestVADOptions: null as any,
  latestVAD: null as any,
  speechGetStatus: vi.fn(),
  speechTranscribe: vi.fn(),
  speechSynthesize: vi.fn(),
  convertToWav: vi.fn(),
  ttsPlay: vi.fn(),
  ttsStop: vi.fn(),
  ttsIsPlaying: vi.fn(),
  stopSpeaking: vi.fn(),
}))

const chatStore = reactive({
  messages: [] as Array<Record<string, any>>,
  streaming: false,
  sending: false,
  toolExecuting: false,
  awaitingConfirmation: false,
  processTrace: [] as Array<Record<string, unknown>>,
})

vi.mock('@/stores/chat', () => ({
  useChatStore: () => chatStore,
}))

vi.mock('@/stores/locale', () => ({
  useLocaleStore: () => ({
    currentLocale: 'en-US',
  }),
}))

vi.mock('@/api/speech', () => ({
  speechApi: {
    getStatus: (...args: unknown[]) => mocks.speechGetStatus(...args),
    transcribe: (...args: unknown[]) => mocks.speechTranscribe(...args),
    synthesize: (...args: unknown[]) => mocks.speechSynthesize(...args),
  },
}))

vi.mock('@/api/voice', () => ({
  ttsAudioManager: {
    play: (...args: unknown[]) => mocks.ttsPlay(...args),
    stop: (...args: unknown[]) => mocks.ttsStop(...args),
    isPlaying: (...args: unknown[]) => mocks.ttsIsPlaying(...args),
  },
  voiceApi: {
    stopSpeaking: (...args: unknown[]) => mocks.stopSpeaking(...args),
  },
}))

vi.mock('@/utils/audioConverter', () => ({
  convertToWav: (...args: unknown[]) => mocks.convertToWav(...args),
}))

vi.mock('@/utils/markdown', () => ({
  markdownToText: (text: string) => text,
}))

vi.mock('@/utils/vad', () => ({
  EnergyVAD: class {
    isListening = false
    isSpeaking = false

    constructor(options: any) {
      mocks.latestVADOptions = options
      mocks.latestVAD = this
    }

    async start() {
      this.isListening = true
    }

    pause() {
      this.isListening = true
    }

    resume() {
      this.isListening = true
    }

    destroy() {
      this.isListening = false
    }
  },
}))

async function mountTalkMode() {
  const wrapper = mount(TalkMode, {
    props: {
      modelValue: false,
      conversationId: 'conv-1',
    },
    global: {
      plugins: [i18n],
      stubs: {
        Teleport: true,
        Transition: true,
        ModelDownloadPrompt: true,
      },
    },
  })

  await wrapper.setProps({ modelValue: true })
  await flushPromises()
  return wrapper
}

describe('TalkMode', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.clearAllMocks()
    i18n.global.locale.value = 'en-US'
    chatStore.messages = []
    chatStore.streaming = false
    chatStore.sending = false
    chatStore.toolExecuting = false
    chatStore.awaitingConfirmation = false
    chatStore.processTrace = []
    mocks.latestVADOptions = null
    mocks.latestVAD = null
    mocks.speechGetStatus.mockResolvedValue({ data: { asr: { ready: true } } })
    mocks.convertToWav.mockResolvedValue(new Blob(['wav'], { type: 'audio/wav' }))
    mocks.speechTranscribe.mockResolvedValue({
      text: 'Tell me more',
      duration: 1.2,
      editable: true,
    })
    mocks.speechSynthesize.mockResolvedValue({
      audio: 'ZmFrZQ==',
      content_type: 'audio/mp3',
    })
    mocks.ttsPlay.mockResolvedValue(undefined)
    mocks.ttsStop.mockReturnValue(true)
    mocks.ttsIsPlaying.mockReturnValue(false)
    mocks.stopSpeaking.mockResolvedValue(undefined)

    vi.stubGlobal('localStorage', {
      getItem: vi.fn((key: string) => {
        if (key === 'tts-auto-play') return 'true'
        if (key === 'tts-speech-volume') return '100'
        return null
      }),
      setItem: vi.fn(),
      removeItem: vi.fn(),
    })
    Object.defineProperty(window, 'isSecureContext', {
      value: true,
      configurable: true,
    })
    Object.defineProperty(window.navigator, 'mediaDevices', {
      value: {
        getUserMedia: vi.fn().mockResolvedValue({
          getTracks: () => [{ stop: vi.fn() }],
        }),
      },
      configurable: true,
    })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('shows audio upload progress and emits the transcript once transcription completes', async () => {
    let resolveTranscribe: ((value: any) => void) | null = null
    mocks.speechTranscribe.mockImplementation((_blob, _format, _lang, options) => {
      options?.onUploadProgress?.(0.45, { loaded: 45, total: 100 } as any)
      return new Promise((resolve) => {
        resolveTranscribe = resolve
      })
    })

    const wrapper = await mountTalkMode()

    const speechEndPromise = mocks.latestVADOptions.onSpeechEnd(
      new Blob(['webm'], { type: 'audio/webm' })
    )
    await flushPromises()

    expect(wrapper.text()).toContain('chat.talkMode.uploadingAudio')
    expect(wrapper.text()).toContain('45%')

    resolveTranscribe?.({
      text: 'Tell me more',
      duration: 1.2,
      editable: true,
    })
    await speechEndPromise
    await flushPromises()

    expect(wrapper.emitted('transcript')).toEqual([['Tell me more']])
    await wrapper.get('.talk-process-panel__toggle').trigger('click')
    expect(wrapper.text()).toContain('chat.talkMode.transcriptReady')
  })

  it('stops TTS playback and returns to listening when barge-in is detected', async () => {
    let releasePlayback: (() => void) | null = null
    mocks.ttsPlay.mockImplementation(
      () =>
        new Promise<void>((resolve) => {
          releasePlayback = resolve
        })
    )

    const wrapper = await mountTalkMode()

    chatStore.messages = [
      {
        id: 'assistant-1',
        role: 'assistant',
        content: 'Here is the answer',
      },
    ]
    chatStore.streaming = true
    await flushPromises()

    chatStore.streaming = false
    await flushPromises()

    expect(mocks.ttsPlay).toHaveBeenCalled()

    mocks.latestVADOptions.onBargeIn()
    await flushPromises()

    expect(mocks.ttsStop).toHaveBeenCalled()
    expect(mocks.stopSpeaking).toHaveBeenCalled()
    expect(wrapper.text()).toContain('chat.stillListening')

    releasePlayback?.()
  })

  it('localizes talk-mode process details', async () => {
    i18n.global.setLocaleMessage('zh-CN', {
      chat: {
        processTrace: {
          fields: {
            format: '格式',
            size: '大小',
            duration: '时长',
            upload: '上传',
            transcript: '转写文本',
            conversation: '会话',
            mode: '模式',
          },
          details: {
            voiceRequestDispatched: 'Blue 正在发送你的语音请求。',
            voiceWaitingForResponse: '正在等待 Blue 的第一段响应。',
          },
          summaryValues: {
            sendNewRequest: '发送新请求',
          },
        },
        talkMode: {
          transcriptReady: '转写已就绪',
        },
      },
    } as never)
    i18n.global.locale.value = 'zh-CN'

    const wrapper = await mountTalkMode()

    await mocks.latestVADOptions.onSpeechEnd(new Blob(['webm'], { type: 'audio/webm' }))
    await flushPromises()

    await wrapper.get('.talk-process-panel__toggle').trigger('click')

    expect(wrapper.text()).toContain('格式:')
    expect(wrapper.text()).toContain('大小:')
    expect(wrapper.text()).toContain('转写文本: Tell me more')
    expect(wrapper.text()).toContain('模式: 发送新请求')
    expect(wrapper.text()).toContain('会话: conv-1')
    expect(wrapper.text()).toContain('Blue 正在发送你的语音请求。')
    expect(wrapper.text()).toContain('正在等待 Blue 的第一段响应。')

    i18n.global.locale.value = 'en-US'
  })
})
