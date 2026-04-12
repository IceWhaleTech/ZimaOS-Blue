import { beforeEach, describe, expect, it, vi } from 'vitest'

const mediaApiMocks = vi.hoisted(() => ({
  listModels: vi.fn(),
  classifyIntent: vi.fn(),
  directGenerate: vi.fn(),
  getTask: vi.fn(),
  cancelTask: vi.fn(),
}))

const chatStoreMock = vi.hoisted(() => ({
  currentConversationId: 'conv-1',
  currentConversation: { id: 'conv-1', title: '' },
  createConversation: vi.fn(),
  fetchMessages: vi.fn(),
  fetchConversations: vi.fn(),
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, fallback?: string) => fallback ?? key,
  }),
}))

vi.mock('@/api/media', () => ({
  listModels: mediaApiMocks.listModels,
  classifyIntent: mediaApiMocks.classifyIntent,
  directGenerate: mediaApiMocks.directGenerate,
  getTask: mediaApiMocks.getTask,
  cancelTask: mediaApiMocks.cancelTask,
}))

vi.mock('@/stores/chat', () => ({
  useChatStore: () => chatStoreMock,
}))

describe('useMediaGenerate', () => {
  beforeEach(() => {
    vi.resetModules()
    vi.clearAllMocks()
    window.localStorage?.clear?.()

    mediaApiMocks.listModels.mockResolvedValue([])
    mediaApiMocks.classifyIntent.mockResolvedValue({ intent: null, models: [] })
    mediaApiMocks.directGenerate.mockResolvedValue({
      task_id: 'task-1',
      message_id: 'msg-1',
      status: 'pending',
      category: 't2i',
      model: 'model-1',
    })
  })

  it('opens the panel for an explicit Chinese image prompt even when no media models are available', async () => {
    const { useMediaGenerate } = await import('./useMediaGenerate')
    const mediaGen = useMediaGenerate()

    const detected = await mediaGen.classify('帮我生成一张灰泰迪的照片', false, 0, 'zh-CN')

    expect(detected).toBe(true)
    expect(mediaGen.intent.value?.category).toBe('t2i')
    expect(mediaGen.showPanel.value).toBe(true)
    expect(mediaGen.models.value).toEqual([])
    expect(mediaApiMocks.directGenerate).not.toHaveBeenCalled()
  })

  it('keeps the panel open instead of auto-submitting when only one model matches', async () => {
    const singleModel = {
      id: 'model-1',
      name: 'Single T2I',
      type: 'image',
      category: 't2i',
      provider: 'mock-provider',
    }
    mediaApiMocks.listModels.mockResolvedValue([singleModel])
    mediaApiMocks.classifyIntent.mockResolvedValue({
      intent: {
        category: 't2i',
        confidence: 0.85,
        prompt: '帮我生成一张灰泰迪的照片',
        has_image: false,
        image_count: 0,
      },
      models: [singleModel],
    })

    const { useMediaGenerate } = await import('./useMediaGenerate')
    const mediaGen = useMediaGenerate()

    const detected = await mediaGen.classify('帮我生成一张灰泰迪的照片', false, 0, 'zh-CN')

    expect(detected).toBe(true)
    expect(mediaGen.intent.value?.category).toBe('t2i')
    expect(mediaGen.showPanel.value).toBe(true)
    expect(mediaGen.selectedModel.value).toBe('model-1')
    expect(mediaApiMocks.directGenerate).not.toHaveBeenCalled()
  })

  it('preserves the original user message separately from the classified prompt', async () => {
    mediaApiMocks.classifyIntent.mockResolvedValue({
      intent: {
        category: 't2i',
        confidence: 0.85,
        prompt: '生成一张灰泰迪的照片',
        has_image: false,
        image_count: 0,
      },
      models: [],
    })

    const { useMediaGenerate } = await import('./useMediaGenerate')
    const mediaGen = useMediaGenerate()
    const originalMessage = '先别生成图片，继续帮我分析这只灰泰迪的构图和风格。生成一张灰泰迪的照片'

    const detected = await mediaGen.classify(originalMessage, false, 0, 'zh-CN')

    expect(detected).toBe(true)
    expect(mediaGen.originalPrompt.value).toBe(originalMessage)
    mediaGen.reset()
    expect(mediaGen.originalPrompt.value).toBe('')
  })
})
