import { describe, expect, it } from 'vitest'

import mediaFallbackOverrides from '@/i18n/media-fallback-overrides'
import type { LocaleMessages } from '@/i18n/merge'
import {
  getLocalizedMediaFallbackLabel,
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
})
