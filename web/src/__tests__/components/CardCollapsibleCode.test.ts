import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'

import CardCollapsibleCode from '@/components/typeless/CardCollapsibleCode.vue'

const { openFullscreenMock } = vi.hoisted(() => ({
  openFullscreenMock: vi.fn(),
}))

vi.mock('@/composables/useFullscreen', () => ({
  useFullscreen: () => ({
    openFullscreen: openFullscreenMock,
  }),
}))

function createTestI18n(locale = 'en-US') {
  return createI18n({
    legacy: false,
    locale,
    fallbackLocale: 'en-US',
    messages: {
      'en-US': {
        common: {
          copy: 'Copy',
          copied: 'Copied',
        },
        execCard: {
          lines: 'lines',
          expand: 'Show more',
          collapse: 'Collapse',
        },
        media: {
          fullscreen: 'Full Screen',
        },
        codeBlock: {
          code: 'code',
        },
        tools: {
          names: {
            file_read: 'File Read',
          },
        },
      },
      'zh-CN': {
        common: {
          copy: '复制',
          copied: '已复制',
        },
        execCard: {
          lines: '行',
          expand: '展开',
          collapse: '收起',
        },
        media: {
          fullscreen: '全屏',
        },
        codeBlock: {
          code: '代码',
        },
        tools: {
          names: {
            file_read: '读取文件',
          },
        },
      },
    },
  })
}

describe('CardCollapsibleCode', () => {
  it('renders File Read cards as file preview headers', () => {
    const wrapper = mount(CardCollapsibleCode, {
      props: {
        card: {
          type: 'collapsible-code',
          title: 'File Read',
          filename: '/Users/orca/Documents/GitHub/ZimaOS-Blue/web/src/components/ChatMessage.vue',
          language: 'vue',
          code: '<template>\n  <div />\n</template>',
        },
      },
      global: {
        plugins: [createTestI18n('zh-CN')],
      },
    })

    expect(wrapper.text()).toContain('读取文件')
    expect(wrapper.text()).toContain('ChatMessage.vue')
    expect(wrapper.text()).toContain('/Users/orca/Documents/GitHub/ZimaOS-Blue/web/src/components')
    expect(wrapper.text()).toContain('VUE')
  })

  it('keeps generic collapsible code cards on the normal header path', () => {
    const wrapper = mount(CardCollapsibleCode, {
      props: {
        card: {
          type: 'collapsible-code',
          title: 'Build output',
          language: 'bash',
          code: 'npm run build',
        },
      },
      global: {
        plugins: [createTestI18n('en-US')],
      },
    })

    expect(wrapper.text()).toContain('Build output')
    expect(wrapper.text()).not.toContain('File Read')
    expect(wrapper.text()).toContain('1 lines')
  })
})
