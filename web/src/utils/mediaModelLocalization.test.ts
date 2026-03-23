import { describe, expect, it } from 'vitest'

import mediaFallbackOverrides from '@/i18n/media-fallback-overrides'
import type { LocaleMessages } from '@/i18n/merge'
import {
  getLocalizedMediaFallbackDisclosure,
  getLocalizedMediaFallbackLabel,
  getLocalizedMediaFallbackTemplateLabel,
  getMediaFallbackStyleLabel,
  getLocalizedMediaModelName,
} from './mediaModelLocalization'

function getByPath(source: LocaleMessages, path: string): unknown {
  return path.split('.').reduce<unknown>((value, part) => {
    if (!value || typeof value !== 'object' || Array.isArray(value)) {
      return undefined
    }
    return (value as Record<string, unknown>)[part]
  }, source)
}

const zhCNMessages = (mediaFallbackOverrides as Record<string, LocaleMessages>)['zh-CN'] || {}

function t(key: string): string {
  return String(getByPath(zhCNMessages, key) ?? '')
}

function te(key: string): boolean {
  const value = getByPath(zhCNMessages, key)
  return typeof value === 'string' && value.trim().length > 0
}

describe('media model localization', () => {
  it('localizes fallback web canvas models from strategy fields', () => {
    expect(
      getLocalizedMediaModelName(
        {
          id: 'fallback-web-canvas-t2i',
          name: 'Fallback Web Canvas',
          is_fallback: true,
          fallback_strategy: 'web_canvas',
        },
        t,
        te
      )
    ).toBe('备用网页画布')
  })

  it('localizes fallback public space models from provider-model ids', () => {
    expect(
      getLocalizedMediaModelName(
        {
          id: 'fallback-space-t2v',
          display_name: 'Fallback Public Space (Video)',
        },
        t,
        te
      )
    ).toBe('备用公共创意空间')
  })

  it('localizes backend fallback display names from runtime strategy info', () => {
    expect(
      getLocalizedMediaFallbackLabel(
        {
          display_name: 'Web Search + Canvas',
          strategy: 'web_canvas',
        },
        t,
        te
      )
    ).toBe('备用网页画布')
  })

  it('localizes fallback disclosure copy from runtime strategy info', () => {
    expect(
      getLocalizedMediaFallbackDisclosure(
        {
          strategy: 'web_canvas',
          disclosure:
            'No configured media API key was available, so this result used a built-in reference or placeholder preview path instead of a newly generated AI image.',
        },
        t,
        te
      )
    ).toBe(
      '未检测到可用媒体生成 API Key，当前结果不是按提示词新生成的图片，而是使用了内建的参考/占位预览兜底路径。下方会尽量披露参考素材来源与许可信息；若来自搜索引擎兜底，则会标注许可未核验。'
    )
  })

  it('localizes native timeline fallback models', () => {
    expect(
      getLocalizedMediaModelName(
        {
          id: 'fallback-native-t2v',
          display_name: 'Native Timeline Renderer',
          strategy: 'native_timeline',
        },
        t,
        te
      )
    ).toBe('备用原生时间轴')
  })

  it('leaves non-fallback models unchanged', () => {
    expect(
      getLocalizedMediaModelName(
        {
          id: 'gpt-image-1',
          name: 'gpt-image-1',
        },
        t,
        te
      )
    ).toBe('gpt-image-1')
  })

  it('normalizes nanoslides style labels for runtime fallback info', () => {
    expect(
      getMediaFallbackStyleLabel({
        style_preset: 'nano slides',
      })
    ).toBe('nanoslides')
  })

  it('formats fallback template labels using locale-aware copy', () => {
    expect(
      getLocalizedMediaFallbackTemplateLabel(
        {
          template_id: 'text_only',
        },
        t,
        te
      )
    ).toBe('文字摘要')
    expect(
      getLocalizedMediaFallbackTemplateLabel(
        {
          template_id: 'cover',
        },
        t,
        te
      )
    ).toBe('封面')
  })
})
