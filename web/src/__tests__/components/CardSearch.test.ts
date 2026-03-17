import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { nextTick } from 'vue'

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
        },
      },
    },
  })
}

describe('CardSearch', () => {
  it('keeps search results collapsed by default until toggled open', async () => {
    const wrapper = mount(CardSearch, {
      props: {
        uiStateKey: 'search-card-collapsed',
        card: {
          type: 'search',
          id: 'search-openai',
          query: 'OpenAI latest news',
          results: [
            {
              title: 'OpenAI updates',
              url: 'https://openai.com/blog',
              description: 'Recent product updates and releases',
            },
          ],
        },
      },
      global: {
        plugins: [createTestI18n('en-US')],
      },
    })

    expect(wrapper.text()).toContain('Web search')
    expect(wrapper.text()).toContain('OpenAI latest news')
    expect(wrapper.text()).toContain('1 results')
    expect(wrapper.text()).not.toContain('Recent product updates and releases')

    await wrapper.get('button').trigger('click')
    await nextTick()

    expect(wrapper.text()).toContain('Recent product updates and releases')
    expect(wrapper.text()).toContain('OpenAI updates')
  })

  it('keeps the detailed results expanded across streaming updates', async () => {
    const wrapper = mount(CardSearch, {
      props: {
        uiStateKey: 'search-card-persist',
        card: {
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
        },
      },
      global: {
        plugins: [createTestI18n('en-US')],
      },
    })

    await wrapper.get('button').trigger('click')
    await nextTick()
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
    await nextTick()

    expect(wrapper.text()).toContain('First result')
    expect(wrapper.text()).toContain('Second result')
  })
})
