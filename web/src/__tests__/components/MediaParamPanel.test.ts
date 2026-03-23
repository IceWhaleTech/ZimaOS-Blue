import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'

import MediaParamPanel from '@/components/MediaParamPanel.vue'

function createTestI18n() {
  return createI18n({
    legacy: false,
    locale: 'zh-CN',
    fallbackLocale: 'en-US',
    messages: {
      'en-US': {
        common: {
          close: 'Close',
        },
        media: {
          t2i: 'Text to Image',
          model: 'Model',
          mediaDetected: 'Media generation detected',
          noChat: 'No, just chat',
          generate: 'Generate',
          generating: 'Generating...',
          fallbackWebCanvas: 'Fallback Web Canvas',
          fallbackPublicSpace: 'Fallback Public Space',
        },
      },
      'zh-CN': {
        common: {
          close: '关闭',
        },
        media: {
          t2i: '文生图',
          model: '模型',
          mediaDetected: '检测到媒体生成',
          noChat: '不生成，仅聊天',
          generate: '生成',
          generating: '生成中...',
          fallbackWebCanvas: '备用网页画布',
          fallbackPublicSpace: '备用公共创意空间',
        },
      },
    },
  })
}

describe('MediaParamPanel', () => {
  it('localizes fallback model labels in the selector', () => {
    const wrapper = mount(MediaParamPanel, {
      props: {
        intent: {
          category: 't2i',
          confidence: 0.96,
          prompt: 'draw a cat astronaut',
          has_image: false,
          image_count: 0,
        },
        models: [
          {
            id: 'fallback-web-canvas-t2i',
            name: 'Fallback Web Canvas',
            type: 'image',
            provider: 'fallback',
            is_fallback: true,
            fallback_strategy: 'web_canvas',
          },
          {
            id: 'fallback-space-t2i',
            name: 'Fallback Public Space (Image)',
            type: 'image',
            provider: 'fallback',
            is_fallback: true,
            fallback_strategy: 'public_space',
          },
        ],
        selectedModel: 'fallback-web-canvas-t2i',
        generating: false,
      },
      global: {
        plugins: [createTestI18n()],
      },
    })

    const options = wrapper.findAll('option').map((option) => option.text())

    expect(options).toContain('备用网页画布')
    expect(options).toContain('备用公共创意空间')
    expect(options).not.toContain('Fallback Web Canvas')
    expect(options).not.toContain('Fallback Public Space (Image)')
  })

  it('shows nanoslides PPT badges when the intent carries slide params', () => {
    const wrapper = mount(MediaParamPanel, {
      props: {
        intent: {
          category: 't2i',
          confidence: 0.95,
          prompt: 'Create a nanoslides strategy summary for Q4 growth',
          has_image: false,
          image_count: 0,
          params: {
            quality_profile: 'ppt',
            source: 'ppt',
            style_preset: 'nano_slides',
          },
        },
        models: [
          {
            id: 'fallback-web-canvas-t2i',
            name: 'Fallback Web Canvas',
            type: 'image',
            provider: 'fallback',
            is_fallback: true,
            fallback_strategy: 'web_canvas',
          },
        ],
        selectedModel: 'fallback-web-canvas-t2i',
        generating: false,
      },
      global: {
        plugins: [createTestI18n()],
      },
    })

    const badges = wrapper.findAll('.mpp-meta-pill').map((node) => node.text())

    expect(badges).toContain('nanoslides')
    expect(badges).toContain('ppt')
    expect(wrapper.find('.mpp-meta--nanoslides').exists()).toBe(true)
  })
})
