import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'

import CardWebFetch from '@/components/typeless/CardWebFetch.vue'

function createTestI18n(locale = 'en-US') {
  return createI18n({
    legacy: false,
    locale,
    fallbackLocale: 'en-US',
    messages: {
      'en-US': {
        common: {
          copied: 'Copied',
          processing: 'Processing...',
        },
        toolWarnings: {
          warning: 'Warning',
          loginWall: 'Login wall',
          challenge: 'Challenge',
          browserRequired: 'Browser required',
        },
        webFetchCard: {
          title: 'Web fetch',
          copyUrl: 'Copy URL',
          copyText: 'Copy text',
          noContent: 'No extracted content',
          actions: {
            use_browser: 'Use browser',
          },
        },
        execCard: {
          outputTruncated: 'truncated',
          collapse: 'Show less',
          expand: 'Show more',
        },
      },
      'zh-CN': {
        common: {
          copied: '已复制',
          processing: '处理中...',
        },
        toolWarnings: {
          warning: '警告',
          loginWall: '需要登录',
          challenge: '验证',
          browserRequired: '需要浏览器',
        },
        webFetchCard: {
          title: '网页抓取',
          copyUrl: '复制 URL',
          copyText: '复制文本',
          noContent: '没有提取到内容',
          actions: {
            use_browser: '使用浏览器',
          },
        },
        execCard: {
          outputTruncated: '已截断',
          collapse: '收起',
          expand: '展开',
        },
      },
    },
  })
}

describe('CardWebFetch', () => {
  it('localizes warning labels and actions for zh-CN', () => {
    const wrapper = mount(CardWebFetch, {
      props: {
        card: {
          type: 'web-fetch',
          title: 'web_fetch',
          url: 'notaurl',
          status: 'warning',
          warning_code: 'login_wall',
          content: 'Log in to continue',
          actions: [
            { id: 'use_browser', label: 'Use browser', variant: 'primary' },
          ],
        },
      },
      global: {
        plugins: [createTestI18n('zh-CN')],
      },
    })

    expect(wrapper.text()).toContain('网页抓取')
    expect(wrapper.text()).toContain('需要登录')
    expect(wrapper.text()).toContain('使用浏览器')
    expect(wrapper.text()).toContain('复制 URL')
    expect(wrapper.text()).toContain('复制文本')
    expect(wrapper.text()).not.toContain('Use browser')
  })

  it('shows localized processing text for the active action', () => {
    const wrapper = mount(CardWebFetch, {
      props: {
        card: {
          type: 'web-fetch',
          title: 'web_fetch',
          url: 'notaurl',
          content: 'Log in to continue',
          actions: [
            { id: 'use_browser', label: 'Use browser', variant: 'primary' },
          ],
        },
        actionLoading: true,
        activeActionId: 'use_browser',
      },
      global: {
        plugins: [createTestI18n('zh-CN')],
      },
    })

    expect(wrapper.text()).toContain('处理中...')
  })
})
