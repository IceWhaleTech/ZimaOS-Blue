import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'

import CardBrowserProgress from '@/components/typeless/CardBrowserProgress.vue'

function createTestI18n(locale = 'en-US') {
  return createI18n({
    legacy: false,
    locale,
    fallbackLocale: 'en-US',
    messages: {
      'en-US': {
        browserProgress: {
          title: 'Browser Progress',
          steps: {
            start: 'Starting browser',
            navigate: 'Navigating',
            snapshot: 'Reading page',
            screenshot: 'Capturing screenshot',
            recipe: 'Running {recipe}',
          },
        },
      },
      'zh-CN': {
        browserProgress: {
          title: '浏览器进度',
          steps: {
            start: '正在启动浏览器',
            navigate: '正在导航',
            snapshot: '正在读取页面',
            screenshot: '正在截取屏幕截图',
            recipe: '正在运行 {recipe}',
          },
        },
      },
    },
  })
}

describe('CardBrowserProgress', () => {
  it('translates known browser steps including screenshots and recipes', () => {
    const wrapper = mount(CardBrowserProgress, {
      props: {
        card: {
          type: 'browser-progress',
          steps: [
            { step: 'start', name: 'Starting browser', status: 'success' },
            { step: 'navigate', name: 'Navigating', status: 'running' },
            { step: 'snapshot', name: 'Reading page', status: 'pending' },
            { step: 'screenshot', name: 'Capturing screenshot', status: 'success' },
            { step: 'recipe', name: 'Running login recipe', status: 'running', recipe_name: 'login recipe' },
          ],
        },
      },
      global: {
        plugins: [createTestI18n('zh-CN')],
      },
    })

    expect(wrapper.text()).toContain('浏览器进度')
    expect(wrapper.text()).toContain('正在启动浏览器')
    expect(wrapper.text()).toContain('正在导航')
    expect(wrapper.text()).toContain('正在读取页面')
    expect(wrapper.text()).toContain('正在截取屏幕截图')
    expect(wrapper.text()).toContain('正在运行 login recipe')
    expect(wrapper.text()).not.toContain('Starting browser')
    expect(wrapper.text()).not.toContain('Capturing screenshot')
  })

  it('translates legacy recipe names by parsing the raw english label', () => {
    const wrapper = mount(CardBrowserProgress, {
      props: {
        card: {
          type: 'browser-progress',
          steps: [
            { step: 'recipe', name: 'Running login recipe', status: 'running' },
          ],
        },
      },
      global: {
        plugins: [createTestI18n('zh-CN')],
      },
    })

    expect(wrapper.text()).toContain('正在运行 login recipe')
  })

  it('falls back to raw step names for unknown ids', () => {
    const wrapper = mount(CardBrowserProgress, {
      props: {
        card: {
          type: 'browser-progress',
          steps: [
            { step: 'download', name: 'Downloading assets', status: 'running' },
          ],
        },
      },
      global: {
        plugins: [createTestI18n()],
      },
    })

    expect(wrapper.text()).toContain('Downloading assets')
  })
})
