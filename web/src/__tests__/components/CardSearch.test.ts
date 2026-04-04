import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { nextTick } from 'vue'

const { mockApiGet } = vi.hoisted(() => ({
  mockApiGet: vi.fn(),
}))

vi.mock('@/api/index', () => ({
  default: {
    get: mockApiGet,
  },
}))

import CardSearch from '@/components/typeless/CardSearch.vue'

function createTestI18n(locale = 'en-US') {
  return createI18n({
    legacy: false,
    locale,
    fallbackLocale: 'en-US',
    messages: {
      'en-US': {
        common: {
          searching: 'Searching...',
          noResponses: 'No responses',
        },
        search: {
          summaryTitle: 'Web search',
          resultCount: '{count} results',
          moreResults: '+{count} more',
          emptyState: 'No matching results',
          partialState: 'Showing source summaries because readable page content was unavailable.',
        },
      },
      'zh-CN': {
        common: {
          searching: '搜索中...',
          noResponses: '暂无回复',
        },
        search: {
          summaryTitle: '网页搜索',
          resultCount: '{count} 条结果',
          moreResults: '另有 {count} 条',
          emptyState: '未找到匹配结果',
          partialState: '搜索源不可读，仅显示摘要',
        },
      },
    },
  })
}

function createMatchMediaMock(matches = true) {
  return vi.fn().mockImplementation((query: string) => ({
    matches: query === '(hover: hover) and (pointer: fine)' ? matches : false,
    media: query,
    onchange: null,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }))
}

function mountSearchCard(card: Record<string, unknown>, locale = 'en-US') {
  return mount(CardSearch, {
    props: {
      uiStateKey: String(card.id || 'search-card'),
      card,
    },
    global: {
      plugins: [createTestI18n(locale)],
      stubs: {
        Teleport: true,
        Transition: false,
      },
    },
  })
}

async function settleCard() {
  await nextTick()
  await flushPromises()
  await nextTick()
}

describe('CardSearch', () => {
  beforeEach(() => {
    mockApiGet.mockReset()
    vi.stubGlobal('matchMedia', createMatchMediaMock(true))
    window.localStorage?.clear?.()
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
    window.localStorage?.clear?.()
  })

  it('shows two collapsed previews and a remaining-count hint before expanding the full result list', async () => {
    const wrapper = mountSearchCard({
      type: 'search',
      id: 'search-openai',
      query: 'OpenAI latest news',
      results: [
        {
          title: 'OpenAI updates',
          url: 'https://openai.com/blog',
          description: 'Recent product updates and releases',
        },
        {
          title: 'API pricing changes',
          url: 'https://platform.openai.com/docs/pricing',
          description: 'Pricing notes for the latest API models',
        },
        {
          title: 'Developer changelog',
          url: 'https://platform.openai.com/docs/changelog',
          description: 'SDK and platform release notes',
        },
      ],
    })

    expect(wrapper.text()).toContain('Web search')
    expect(wrapper.text()).toContain('OpenAI latest news')
    expect(wrapper.text()).toContain('3 results')
    expect(wrapper.text()).toContain('OpenAI updates')
    expect(wrapper.text()).toContain('Recent product updates and releases')
    expect(wrapper.text()).toContain('API pricing changes')
    expect(wrapper.text()).toContain('Pricing notes for the latest API models')
    expect(wrapper.text()).toContain('+1 more')
    expect(wrapper.text()).not.toContain('Developer changelog')

    await wrapper.get('button').trigger('click')
    await settleCard()

    expect(wrapper.text()).toContain('Developer changelog')
    expect(wrapper.text()).toContain('SDK and platform release notes')
  })

  it('falls back to the result domain in the collapsed preview when no description exists', () => {
    const wrapper = mountSearchCard({
      type: 'search',
      id: 'search-domain-preview',
      query: 'OpenAI docs',
      results: [
        {
          title: 'API reference',
          url: 'https://platform.openai.com/docs/api-reference',
        },
      ],
    })

    expect(wrapper.text()).toContain('API reference')
    expect(wrapper.text()).toContain('platform.openai.com')
  })

  it('shows a visible empty-state message instead of disappearing when there are no results', async () => {
    const wrapper = mountSearchCard(
      {
        type: 'search',
        id: 'search-empty',
        query: 'No hits',
        status: 'empty',
        results: [],
      } as any,
      'zh-CN'
    )

    await wrapper.get('button').trigger('click')
    await settleCard()

    expect(wrapper.text()).toContain('未找到匹配结果')
  })

  it('shows a partial-state hint when only source summaries are available', async () => {
    const wrapper = mountSearchCard(
      {
        type: 'search',
        id: 'search-partial',
        query: 'snippet only',
        status: 'partial',
        results: [
          {
            title: 'Snippet source',
            url: 'https://example.com/snippet',
            description: 'Only snippet content is available',
          },
        ],
      } as any,
      'zh-CN'
    )

    await wrapper.get('button').trigger('click')
    await settleCard()

    expect(wrapper.text()).toContain('搜索源不可读，仅显示摘要')
  })

  it('keeps a fallback hover preview visible on desktop when /link-preview fails', async () => {
    vi.useFakeTimers()
    mockApiGet.mockRejectedValueOnce(new Error('preview failed'))

    const wrapper = mountSearchCard({
      type: 'search',
      id: 'search-preview-fallback',
      query: 'ZimaOS release notes',
      results: [
        {
          title: 'ZimaOS Release Notes',
          url: 'https://example.com/release',
          description: 'Official release summary',
        },
      ],
    })

    await wrapper.get('button').trigger('click')
    await settleCard()

    await wrapper.get('a[href="https://example.com/release"]').trigger('mouseenter')
    await nextTick()
    vi.advanceTimersByTime(400)
    await settleCard()

    expect(mockApiGet).toHaveBeenCalledTimes(1)
    expect(mockApiGet).toHaveBeenCalledWith('/link-preview', {
      params: { url: 'https://example.com/release' },
      timeout: 8000,
    })

    const preview = wrapper.get('.search-card__preview-popover')
    expect(preview.text()).toContain('ZimaOS Release Notes')
    expect(preview.text()).toContain('Official release summary')
    expect(preview.text()).toContain('example.com')
  })

  it('disables hover preview fetching on coarse-pointer devices', async () => {
    vi.useFakeTimers()
    vi.stubGlobal('matchMedia', createMatchMediaMock(false))

    const wrapper = mountSearchCard({
      type: 'search',
      id: 'search-no-mobile-preview',
      query: 'ZimaOS release notes',
      results: [
        {
          title: 'ZimaOS Release Notes',
          url: 'https://example.com/release',
          description: 'Official release summary',
        },
      ],
    })

    await wrapper.get('button').trigger('click')
    await settleCard()

    await wrapper.get('a[href="https://example.com/release"]').trigger('mouseenter')
    await nextTick()
    vi.advanceTimersByTime(400)
    await settleCard()

    expect(mockApiGet).not.toHaveBeenCalled()
    expect(wrapper.find('.search-card__preview-popover').exists()).toBe(false)
  })

  it('keeps the detailed results expanded across streaming updates', async () => {
    const wrapper = mountSearchCard({
      type: 'search',
      id: 'search-persist',
      query: 'OpenAI roadmap',
      results: [
        {
          title: 'Roadmap 1',
          url: 'https://example.com/1',
          description: 'First result',
        },
      ],
    })

    await wrapper.get('button').trigger('click')
    await settleCard()
    expect(wrapper.text()).toContain('First result')

    await wrapper.setProps({
      card: {
        type: 'search',
        id: 'search-persist',
        query: 'OpenAI roadmap',
        _streaming: true,
        results: [
          {
            title: 'Roadmap 1',
            url: 'https://example.com/1',
            description: 'First result',
          },
          {
            title: 'Roadmap 2',
            url: 'https://example.com/2',
            description: 'Second result',
          },
        ],
      },
    })
    await settleCard()

    expect(wrapper.text()).toContain('First result')
    expect(wrapper.text()).toContain('Second result')
  })
})
