import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'

const storageState = new Map<string, string>()
const localStorageMock = {
  getItem: (key: string) => storageState.get(key) ?? null,
  setItem: (key: string, value: string) => {
    storageState.set(key, value)
  },
  removeItem: (key: string) => {
    storageState.delete(key)
  },
  clear: () => {
    storageState.clear()
  },
}

vi.mock('@/stores/tts', () => ({
  useTTSStore: () => ({
    languagePacks: [
      { language: 'en', name: 'English', size_kb: 512, downloaded: false },
      { language: 'zh', name: '中文', size_kb: 768, downloaded: true },
    ],
    loadLanguagePacks: vi.fn().mockResolvedValue(undefined),
    downloadLanguagePack: vi.fn().mockResolvedValue(undefined),
    deleteLanguagePack: vi.fn().mockResolvedValue(undefined),
  }),
}))

import { i18n, setLocale } from '@/i18n'
import VoicePackManager from '@/components/tts/VoicePackManager.vue'
import ErrorBoundary from '@/components/ErrorBoundary.vue'

describe('fallback UI i18n', () => {
  beforeEach(async () => {
    storageState.clear()
    vi.stubGlobal('localStorage', localStorageMock)
    if (typeof window !== 'undefined') {
      Object.defineProperty(window, 'localStorage', {
        value: localStorageMock,
        configurable: true,
      })
    }
    await setLocale('zh-CN')
  })

  it('localizes voice pack manager labels', async () => {
    const wrapper = mount(VoicePackManager, {
      global: {
        plugins: [i18n],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('语言包')
    expect(wrapper.text()).toContain('下载离线语音合成所需的语言包')
    expect(wrapper.text()).toContain('下载')
    expect(wrapper.text()).toContain('删除')
    expect(wrapper.text()).not.toContain('Download language packs for offline speech synthesis')
  })

  it('localizes error boundary fallback copy', async () => {
    const Crasher = defineComponent({
      name: 'Crasher',
      setup() {
        throw new Error('boom')
      },
      template: '<div />',
    })

    const wrapper = mount(
      {
        components: { ErrorBoundary, Crasher },
        template: '<ErrorBoundary><Crasher /></ErrorBoundary>',
      },
      {
        global: {
          plugins: [i18n],
        },
      }
    )

    await flushPromises()

    expect(wrapper.text()).toContain('出了点问题')
    expect(wrapper.text()).toContain('发生了意外错误，请重试。')
    expect(wrapper.text()).toContain('详情')
    expect(wrapper.text()).toContain('重试')
    expect(wrapper.text()).not.toContain('Something went wrong')
  })
})
