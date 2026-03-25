import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'

import TypelessCard from '@/components/typeless/TypelessCard.vue'
import { fullscreenContent, isFullscreen } from '@/composables/useFullscreen'

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
        terminalCard: {
          title: 'Terminal',
        },
        execCard: {
          lines: 'lines',
        },
        media: {
          fullscreen: 'Full Screen',
        },
      },
      'zh-CN': {
        common: {
          copy: '复制',
          copied: '已复制',
        },
        terminalCard: {
          title: '终端',
        },
        execCard: {
          lines: '行',
        },
        media: {
          fullscreen: '全屏',
        },
      },
    },
  })
}

async function settleCard() {
  await flushPromises()
  await vi.dynamicImportSettled()
  await flushPromises()
}

describe('TypelessCard terminal integration', () => {
  beforeEach(() => {
    Object.defineProperty(window.navigator, 'clipboard', {
      configurable: true,
      value: {
        writeText: vi.fn().mockResolvedValue(undefined),
      },
    })

    isFullscreen.value = false
    fullscreenContent.value = null
    document.body.style.overflow = ''
  })

  afterEach(() => {
    isFullscreen.value = false
    fullscreenContent.value = null
    document.body.style.overflow = ''
    vi.restoreAllMocks()
  })

  it('loads terminal cards through the generic typeless wrapper with specialized metadata', async () => {
    const wrapper = mount(TypelessCard, {
      props: {
        card: {
          type: 'terminal',
          id: 'terminal-wrapper',
          title: 'Build output',
          content: '\u001b[31mError:\u001b[0m missing file\nCompleted',
          prompt: '$ ',
          showPrompt: true,
          maxHeight: 240,
          theme: 'dark',
        },
      },
      global: {
        plugins: [createTestI18n('en-US')],
      },
    })

    await settleCard()

    expect(wrapper.find('#terminal-wrapper').exists()).toBe(true)
    expect(wrapper.text()).toContain('Terminal')
    expect(wrapper.text()).toContain('Build output')
    expect(wrapper.text()).toContain('$')
    expect(wrapper.text()).toContain('2 lines')
    expect(wrapper.text()).toContain('240px')
    expect(wrapper.html()).toContain('text-red-600')
    expect(wrapper.html()).not.toContain('\u001b[31m')
  })

  it('copies plain terminal text without ANSI escape codes', async () => {
    const wrapper = mount(TypelessCard, {
      props: {
        card: {
          type: 'terminal',
          id: 'terminal-copy',
          title: 'Terminal',
          content: '\u001b[32mok\u001b[0m\nnext',
          theme: 'dark',
        },
      },
      global: {
        plugins: [createTestI18n('en-US')],
      },
    })

    await settleCard()

    await wrapper.get('[data-testid="terminal-copy"]').trigger('click')
    await flushPromises()

    expect(window.navigator.clipboard.writeText).toHaveBeenCalledWith('ok\nnext')
    expect(wrapper.text()).toContain('Copied')
  })

  it('localizes the terminal card title and opens fullscreen from the specialized component', async () => {
    const wrapper = mount(TypelessCard, {
      props: {
        card: {
          type: 'terminal',
          id: 'terminal-zh',
          content: '第一行',
          theme: 'light',
        },
      },
      global: {
        plugins: [createTestI18n('zh-CN')],
      },
    })

    await settleCard()

    expect(wrapper.text()).toContain('终端')
    expect(wrapper.text()).toContain('1 行')

    await wrapper.get('[data-testid="terminal-fullscreen"]').trigger('click')
    await flushPromises()

    expect(isFullscreen.value).toBe(true)
    expect(fullscreenContent.value).toEqual({
      type: 'terminal',
      title: '终端',
      content: '第一行',
    })
  })
})
