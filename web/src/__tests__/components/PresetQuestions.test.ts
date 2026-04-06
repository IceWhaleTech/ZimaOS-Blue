import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import PresetQuestions from '@/components/onboarding/PresetQuestions.vue'
import PresetQuestionCard from '@/components/onboarding/PresetQuestionCard.vue'
import { i18n, setLocale } from '@/i18n'
import { previewApi } from '@/api/preview'
import type { PresetQuestion, PresetQuestionAttachment } from '@/api/preview'

vi.mock('@/api/preview', async () => {
  const actual = await vi.importActual<typeof import('@/api/preview')>('@/api/preview')
  return {
    ...actual,
    previewApi: {
      ...actual.previewApi,
      getPresetQuestions: vi.fn(),
    },
  }
})

function buildQuestion(
  index: number,
  overrides: Partial<PresetQuestion> = {}
): PresetQuestion {
  return {
    id: `q-${index}`,
    title: `Title ${index}`,
    description: `Description ${index}`,
    prompt: `Prompt ${index}`,
    text: `Prompt ${index}`,
    category: index % 2 === 0 ? 'product-design' : 'market-investing',
    tags: [index % 2 === 0 ? 'product-design' : 'market-investing'],
    editorial_score: 200 - index,
    icon: '✨',
    ...overrides,
  }
}

function buildQuestionPage(offset: number, count: number) {
  const allQuestions = Array.from({ length: 12 }, (_, index) => buildQuestion(index + 1))
  const questions = allQuestions.slice(offset, offset + count)
  const nextOffset = offset + questions.length

  return {
    questions,
    total: allQuestions.length,
    next_offset: nextOffset,
    has_more: nextOffset < allQuestions.length,
  }
}

describe('PresetQuestions', () => {
  beforeEach(async () => {
    const storageState: Record<string, string> = {}
    const storage = {
      getItem: (key: string) => storageState[key] ?? null,
      setItem: (key: string, value: string) => {
        storageState[key] = String(value)
      },
      removeItem: (key: string) => {
        delete storageState[key]
      },
      clear: () => {
        for (const key of Object.keys(storageState)) {
          delete storageState[key]
        }
      },
    }

    Object.defineProperty(window, 'localStorage', {
      configurable: true,
      value: storage,
    })
    Object.defineProperty(globalThis, 'localStorage', {
      configurable: true,
      value: storage,
    })

    vi.mocked(previewApi.getPresetQuestions).mockReset()
    vi.mocked(previewApi.getPresetQuestions).mockImplementation(
      async (count = 4, _lang?: string, offset = 0) =>
        ({
          data: buildQuestionPage(offset, count),
        }) as Awaited<ReturnType<typeof previewApi.getPresetQuestions>>
    )
    await setLocale('zh-CN')
  })

  it('renders 4 real cards first while preserving 12 total slots with placeholders', async () => {
    const wrapper = mount(PresetQuestions, {
      props: { contextText: '' },
      global: {
        plugins: [i18n],
      },
    })

    await flushPromises()

    expect(wrapper.find('[data-testid="preset-questions-scroll"]').exists()).toBe(true)
    expect(vi.mocked(previewApi.getPresetQuestions)).toHaveBeenCalledWith(4, 'zh', 0)
    expect(wrapper.findAll('[data-testid^="preset-question-card-"]')).toHaveLength(4)
    expect(wrapper.findAll('[data-testid="preset-question-placeholder"]')).toHaveLength(8)
    expect(wrapper.text()).not.toContain('12/12')
  })

  it('emits the prompt, not the card title, when a card is clicked', async () => {
    const wrapper = mount(PresetQuestions, {
      props: { contextText: '' },
      global: {
        plugins: [i18n],
      },
    })

    await flushPromises()

    await wrapper.get('[data-testid="preset-question-card-q-1"]').trigger('click')

    expect(wrapper.emitted('select')).toBeTruthy()
    expect(wrapper.emitted('select')?.[0]?.[0]).toBe('Prompt 1')
  })

  it('strips bundled sample attachments from rendered questions and emitted payloads', async () => {
    const sampleAttachment: PresetQuestionAttachment = {
      type: 'image',
      name: 'chart.png',
      mime_type: 'image/png',
      placeholder: 'sample-chart',
    }

    vi.mocked(previewApi.getPresetQuestions).mockResolvedValueOnce({
      data: {
        questions: [buildQuestion(1, { attachments: [sampleAttachment] })],
        total: 1,
        next_offset: 1,
        has_more: false,
      },
    } as Awaited<ReturnType<typeof previewApi.getPresetQuestions>>)

    const wrapper = mount(PresetQuestions, {
      props: { contextText: '' },
      global: {
        plugins: [i18n],
      },
    })

    await flushPromises()

    const renderedQuestion = wrapper.getComponent(PresetQuestionCard).props('question') as PresetQuestion
    expect(renderedQuestion.attachments).toBeUndefined()

    await wrapper.get('[data-testid="preset-question-card-q-1"]').trigger('click')

    expect(wrapper.emitted('select')).toBeTruthy()
    expect(wrapper.emitted('select')?.[0]).toEqual(['Prompt 1', undefined])
  })

  it('loads the remaining questions when the list scrolls near the bottom', async () => {
    const wrapper = mount(PresetQuestions, {
      props: { contextText: '' },
      global: {
        plugins: [i18n],
      },
    })

    await flushPromises()

    const scrollContainer = wrapper.get('[data-testid="preset-questions-scroll"]').element
    Object.defineProperty(scrollContainer, 'clientHeight', {
      configurable: true,
      value: 240,
    })
    Object.defineProperty(scrollContainer, 'scrollHeight', {
      configurable: true,
      value: 720,
    })
    Object.defineProperty(scrollContainer, 'scrollTop', {
      configurable: true,
      writable: true,
      value: 432,
    })

    await wrapper.get('[data-testid="preset-questions-scroll"]').trigger('scroll')
    await flushPromises()

    expect(vi.mocked(previewApi.getPresetQuestions)).toHaveBeenNthCalledWith(2, 8, 'zh', 4)
    expect(wrapper.findAll('[data-testid^="preset-question-card-"]')).toHaveLength(12)
    expect(wrapper.findAll('[data-testid="preset-question-placeholder"]')).toHaveLength(0)
  })

  it('loads the remaining questions before filtering cards by the selected interest chip', async () => {
    const wrapper = mount(PresetQuestions, {
      props: { contextText: '' },
      global: {
        plugins: [i18n],
      },
    })

    await flushPromises()

    await wrapper.get('[data-testid="preset-interest-chip-product-design"]').trigger('click')
    await flushPromises()

    const cards = wrapper.findAll('[data-testid^="preset-question-card-"]')
    expect(vi.mocked(previewApi.getPresetQuestions)).toHaveBeenNthCalledWith(2, 8, 'zh', 4)
    expect(cards).toHaveLength(6)
    expect(wrapper.findAll('[data-testid="preset-question-placeholder"]')).toHaveLength(0)
    expect(cards.map((card) => card.attributes('data-testid'))).toEqual([
      'preset-question-card-q-2',
      'preset-question-card-q-4',
      'preset-question-card-q-6',
      'preset-question-card-q-8',
      'preset-question-card-q-10',
      'preset-question-card-q-12',
    ])
  })

  it('renders list cards with inline expandable details instead of absolute overlays', async () => {
    const wrapper = mount(PresetQuestions, {
      props: { contextText: '' },
      global: {
        plugins: [i18n],
      },
    })

    await flushPromises()

    const firstCard = wrapper.get('[data-testid="preset-question-card-q-1"]')
    const baseCard = firstCard.get('.preset-question-card-base')
    const expandedPreview = firstCard.get('.preset-question-card-expanded')

    expect(firstCard.classes()).toContain('w-full')
    expect(firstCard.classes().some((className) => className.startsWith('aspect-'))).toBe(false)
    expect(baseCard.classes()).toContain('rounded-[1.25rem]')
    expect(baseCard.classes()).toContain('min-h-[4.5rem]')
    expect(expandedPreview.classes()).not.toContain('pointer-events-none')
    expect(expandedPreview.classes()).not.toContain('absolute')
  })

  it('renders list cards without legacy grid placement styles', () => {
    const wrapper = mount(PresetQuestionCard, {
      props: {
        question: buildQuestion(7),
      },
    })

    expect(wrapper.attributes('style')).toBeUndefined()
    expect(wrapper.classes()).toContain('w-full')
  })
})
