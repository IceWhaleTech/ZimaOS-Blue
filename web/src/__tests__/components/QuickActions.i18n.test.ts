import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'

import QuickActions from '@/components/QuickActions.vue'

function createTestI18n() {
  return createI18n({
    legacy: false,
    locale: 'zh-CN',
    fallbackLocale: 'en-US',
    messages: {
      'en-US': {
        common: {
          quickActionsTitle: 'Quick Actions',
          loadingActions: 'Loading actions...',
          noActionsAvailable: 'No actions available',
        },
        cardActions: {
          use_browser: 'Use browser',
        },
        quickActions: {
          browserHint: 'Open the current page in the browser session',
        },
      },
      'zh-CN': {
        common: {
          quickActionsTitle: '快捷操作',
          loadingActions: '加载操作中...',
          noActionsAvailable: '暂无可用操作',
        },
        cardActions: {
          use_browser: '使用浏览器',
        },
        quickActions: {
          browserHint: '在当前浏览器会话中打开页面',
        },
      },
    },
  })
}

describe('QuickActions i18n', () => {
  it('renders labelKey and descriptionKey when provided', () => {
    const wrapper = mount(QuickActions, {
      props: {
        actions: [
          {
            id: 'use_browser',
            label: 'Use browser',
            labelKey: 'cardActions.use_browser',
            description: 'Open in browser',
            descriptionKey: 'quickActions.browserHint',
            icon: 'M0 0h24v24H0z',
          },
        ],
      },
      global: {
        plugins: [createTestI18n()],
      },
    })

    expect(wrapper.text()).toContain('使用浏览器')
    expect(wrapper.text()).toContain('在当前浏览器会话中打开页面')
    expect(wrapper.text()).not.toContain('Use browser')
  })
})
