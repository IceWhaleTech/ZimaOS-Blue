import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import PresetQuestions from '@/components/onboarding/PresetQuestions.vue'
import PresetQuestionCard from '@/components/onboarding/PresetQuestionCard.vue'
import { i18n, setLocale } from '@/i18n'
import { previewApi } from '@/api/preview'
import type { PresetQuestion } from '@/api/preview'

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

function buildQuestion(index: number): PresetQuestion {
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

    vi.mocked(previewApi.getPresetQuestions).mockResolvedValue({
      data: {
        questions: Array.from({ length: 12 }, (_, index) => buildQuestion(index + 1)),
      },
    } as Awaited<ReturnType<typeof previewApi.getPresetQuestions>>)
    await setLocale('zh-CN')
  })

  it('renders the full 12-item scrollable list immediately', async () => {
    const wrapper = mount(PresetQuestions, {
      props: { contextText: '' },
      global: {
        plugins: [i18n],
      },
    })

    await flushPromises()

    expect(wrapper.find('[data-testid="preset-questions-scroll"]').exists()).toBe(true)
    expect(wrapper.findAll('[data-testid^="preset-question-card-"]')).toHaveLength(12)
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

  it('filters cards by the selected interest chip instead of reordering them', async () => {
    const wrapper = mount(PresetQuestions, {
      props: { contextText: '' },
      global: {
        plugins: [i18n],
      },
    })

    await flushPromises()

    await wrapper.get('[data-testid="preset-interest-chip-product-design"]').trigger('click')

    const cards = wrapper.findAll('[data-testid^="preset-question-card-"]')
    expect(cards).toHaveLength(6)
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
