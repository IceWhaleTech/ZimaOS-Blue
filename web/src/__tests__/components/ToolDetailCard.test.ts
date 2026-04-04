import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import ToolDetailCard from '@/components/ToolDetailCard.vue'
import { i18n } from '@/i18n'
import type { ToolResultItem } from '@/stores/chat'

function makeItem(overrides: Partial<ToolResultItem> = {}): ToolResultItem {
  return {
    name: 'tool_search',
    id: 'tool-search-1',
    command: 'analyze deep research analysis synthesis report',
    args: JSON.stringify({
      query: 'analyze deep research analysis synthesis report',
    }),
    icon: '✓',
    status: '',
    output: '',
    timestamp: Date.now(),
    ...overrides,
  }
}

describe('ToolDetailCard', () => {
  it('renders tool_search inputs as a query instead of a shell command', () => {
    const wrapper = mount(ToolDetailCard, {
      props: {
        item: makeItem(),
      },
      global: {
        plugins: [i18n],
      },
    })

    expect(wrapper.find('.tool-detail-card__command-label').text()).toBe('Query')
    expect(wrapper.find('.tool-detail-card__command-text').text()).toBe(
      'analyze deep research analysis synthesis report'
    )
    expect(wrapper.text()).not.toContain('$ analyze deep research analysis synthesis report')
  })

  it('keeps shell-style command rendering for exec tool results', () => {
    const wrapper = mount(ToolDetailCard, {
      props: {
        item: makeItem({
          name: 'exec',
          id: 'exec-1',
          command: 'npm test',
          args: JSON.stringify({ cmd: 'npm test' }),
        }),
      },
      global: {
        plugins: [i18n],
      },
    })

    expect(wrapper.find('.tool-detail-card__command-label').text()).toBe('$')
    expect(wrapper.find('.tool-detail-card__command-text').text()).toBe('npm test')
  })
})
