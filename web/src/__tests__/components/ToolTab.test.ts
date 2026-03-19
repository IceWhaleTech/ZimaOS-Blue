import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'

const fetchTools = vi.fn().mockResolvedValue(undefined)
const enableTool = vi.fn().mockResolvedValue(true)
const disableTool = vi.fn().mockResolvedValue(true)
const clearError = vi.fn()

const mockStore = {
  tools: [
    {
      id: 'custom.tool',
      name: 'Custom Tool',
      version: '1.0.0',
      description: 'Custom description',
      category: 'utility',
      enabled: true,
      builtin: false,
      parameters: [],
    },
    {
      id: 'builtin.tool',
      name: 'Builtin Tool',
      version: '1.0.0',
      description: 'Builtin description',
      category: 'system',
      enabled: true,
      builtin: true,
      parameters: [],
    },
  ],
  enabledTools: [
    {
      id: 'custom.tool',
      name: 'Custom Tool',
      version: '1.0.0',
      description: 'Custom description',
      category: 'utility',
      enabled: true,
      builtin: false,
      parameters: [],
    },
    {
      id: 'builtin.tool',
      name: 'Builtin Tool',
      version: '1.0.0',
      description: 'Builtin description',
      category: 'system',
      enabled: true,
      builtin: true,
      parameters: [],
    },
  ],
  builtinTools: [
    {
      id: 'builtin.tool',
      name: 'Builtin Tool',
      version: '1.0.0',
      description: 'Builtin description',
      category: 'system',
      enabled: true,
      builtin: true,
      parameters: [],
    },
  ],
  loading: false,
  error: null,
  fetchTools,
  enableTool,
  disableTool,
  clearError,
}

vi.mock('@/stores/tool', () => ({
  useToolStore: () => mockStore,
}))

vi.mock('@/utils/toolLocalization', () => ({
  getLocalizedToolName: (name: string) => name,
  getLocalizedToolDescription: (_name: string, description: string) => description,
}))

import ToolTab from '@/components/extensions/ToolTab.vue'

function createTestI18n() {
  return createI18n({
    legacy: false,
    locale: 'en-US',
    fallbackLocale: 'en-US',
    messages: {
      'en-US': {
        common: {
          loading: 'Loading...',
          enabled: 'Enabled',
          disabled: 'Disabled',
        },
        extensions: {
          tools: 'Tools',
        },
        plugins: {
          subtitle: 'Tool gallery',
          searchPlaceholder: 'Search tools',
          allCategories: 'All categories',
          noDescription: 'No description',
          noTools: 'No tools',
          noMatchingTools: 'No matching tools',
          noToolsTitle: 'No tools found',
          stats: {
            total: 'Total',
            enabled: 'Enabled',
          },
        },
        workspace: {
          desc: {
            tools: 'Tools for your workspace',
          },
        },
        skillStore: {
          status: {
            builtin: 'Built-in',
            local: 'Local',
          },
        },
      },
    },
  })
}

describe('ToolTab', () => {
  beforeEach(() => {
    fetchTools.mockClear()
    enableTool.mockClear()
    disableTool.mockClear()
    clearError.mockClear()
  })

  it('only renders a badge for built-in tools and never shows a local badge', async () => {
    const wrapper = mount(ToolTab, {
      global: {
        plugins: [createTestI18n()],
      },
    })

    await flushPromises()

    expect(fetchTools).toHaveBeenCalledTimes(1)
    expect(wrapper.findAll('.tool-showcase-card__badge').map((node) => node.text())).toEqual([
      'Built-in',
    ])
    expect(wrapper.text()).not.toContain('Local')
  })
})
