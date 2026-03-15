import { afterEach, describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { createPinia } from 'pinia'
import { nextTick } from 'vue'

import CardAccordion from '@/components/typeless/CardAccordion.vue'
import CardChoice from '@/components/typeless/CardChoice.vue'
import CardCountdown from '@/components/typeless/CardCountdown.vue'
import CardFile from '@/components/typeless/CardFile.vue'
import FullscreenModal from '@/components/typeless/FullscreenModal.vue'
import { fullscreenContent, isFullscreen } from '@/composables/useFullscreen'

function createTestI18n(locale = 'zh-CN') {
  return createI18n({
    legacy: false,
    locale,
    fallbackLocale: 'en-US',
    messages: {
      'en-US': {
        common: {
          preview: 'Preview',
          download: 'Download',
          selectMultipleOptions: 'Select multiple options',
          processing: 'Processing...',
          copied: 'Copied!',
          copy: 'Copy',
        },
        askQuestion: {
          other: 'Other',
          otherPlaceholder: 'Type your answer...',
        },
        countdownCard: {
          expired: "Time's up!",
          days: 'Days',
          hours: 'Hours',
          minutes: 'Minutes',
          seconds: 'Seconds',
        },
        accordionCard: {
          thinking: 'Thinking',
        },
        fullscreenModal: {
          exitHint: 'Press {key} or double-click to exit fullscreen',
        },
      },
      'zh-CN': {
        common: {
          preview: '预览',
          download: '下载',
          selectMultipleOptions: '选择多个选项',
          processing: '处理中...',
          copied: '已复制',
          copy: '复制',
        },
        askQuestion: {
          other: '其他',
          otherPlaceholder: '输入你的回答...',
        },
        countdownCard: {
          expired: '时间到了！',
          days: '天',
          hours: '小时',
          minutes: '分钟',
          seconds: '秒',
        },
        accordionCard: {
          thinking: '思考中',
        },
        fullscreenModal: {
          exitHint: '按 {key} 或双击退出全屏',
        },
      },
    },
  })
}

afterEach(() => {
  isFullscreen.value = false
  fullscreenContent.value = null
  document.body.style.overflow = ''
  document.body.innerHTML = ''
})

describe('Typeless card i18n', () => {
  it('localizes file action titles', () => {
    const wrapper = mount(CardFile, {
      props: {
        card: {
          type: 'file',
          filename: 'report.pdf',
          previewUrl: 'https://example.com/preview',
          downloadUrl: 'https://example.com/download',
        },
      },
      global: {
        plugins: [createTestI18n()],
      },
    })

    const buttons = wrapper.findAll('button')
    expect(buttons[0]?.attributes('title')).toBe('预览')
    expect(buttons[0]?.attributes('aria-label')).toBe('预览')
    expect(buttons[1]?.attributes('title')).toBe('下载')
    expect(buttons[1]?.attributes('aria-label')).toBe('下载')
  })

  it('localizes countdown expired and unit labels', () => {
    const expiredWrapper = mount(CardCountdown, {
      props: {
        card: {
          type: 'countdown',
          targetDate: new Date(Date.now() - 1000).toISOString(),
        },
      },
      global: {
        plugins: [createTestI18n()],
      },
    })

    expect(expiredWrapper.text()).toContain('时间到了！')

    const runningWrapper = mount(CardCountdown, {
      props: {
        card: {
          type: 'countdown',
          targetDate: new Date(Date.now() + 24 * 60 * 60 * 1000).toISOString(),
        },
      },
      global: {
        plugins: [createTestI18n()],
      },
    })

    expect(runningWrapper.text()).toContain('天')
    expect(runningWrapper.text()).toContain('小时')
    expect(runningWrapper.text()).toContain('分钟')
    expect(runningWrapper.text()).toContain('秒')
  })

  it('localizes choice helper copy and loading state', async () => {
    const wrapper = mount(CardChoice, {
      props: {
        card: {
          type: 'choice',
          title: 'Choose',
          multiple: true,
          allowOther: true,
          options: [{ id: 'a', label: 'Alpha' }],
        },
        actionLoading: true,
        activeActionId: 'select',
      },
      global: {
        plugins: [createTestI18n()],
      },
    })

    expect(wrapper.text()).toContain('选择多个选项')
    expect(wrapper.text()).toContain('其他')
    expect(wrapper.text()).toContain('处理中...')

    await wrapper.findAll('button')[1]?.trigger('click')
    await nextTick()

    const input = wrapper.find('input')
    expect(input.exists()).toBe(false)
  })

  it('localizes thinking accordion header', () => {
    const wrapper = mount(CardAccordion, {
      props: {
        card: {
          type: 'accordion',
          id: 'thinking-1',
          items: [{ content: 'step' }],
        },
      },
      global: {
        plugins: [createPinia(), createTestI18n()],
      },
    })

    expect(wrapper.text()).toContain('思考中')
    expect(wrapper.text()).not.toContain('Thinking')
  })

  it('localizes fullscreen exit hint with key slot', async () => {
    isFullscreen.value = true
    fullscreenContent.value = {
      type: 'code',
      content: 'const x = 1',
    }

    const wrapper = mount(FullscreenModal, {
      attachTo: document.body,
      global: {
        plugins: [createTestI18n()],
      },
    })

    await nextTick()

    expect(document.body.textContent || '').toContain('按')
    expect(document.body.textContent || '').toContain('Esc')
    expect(document.body.textContent || '').toContain('退出全屏')

    wrapper.unmount()
  })
})
