import { describe, expect, it } from 'vitest'
import { classifyMediaIntent } from './useMediaIntent'

describe('classifyMediaIntent', () => {
  it('suppresses meta discussion about keyword matching density', () => {
    const result = classifyMediaIntent(
      'IR匹配关键词的时候，也需要考虑关键词命中的密度吧，比如在一大段文本内部出现了生成图片可能就不是这个意图',
      false,
      0,
      'zh-CN'
    )

    expect(result).toBeNull()
  })

  it('suppresses long low-density mentions inside descriptive text', () => {
    const result = classifyMediaIntent(
      '这是一段很长的产品说明文本，主要在讨论文档整理、消息同步、权限控制和缓存策略。中间顺带提到系统未来也许会支持生成图片能力，但这里并不是在向你发出生成请求。',
      false,
      0,
      'zh-CN'
    )

    expect(result).toBeNull()
  })

  it('keeps strong long-form generation prompts', () => {
    const result = classifyMediaIntent(
      'Please generate a highly detailed image of a moonlit harbor with watercolor textures, warm reflections, and soft cinematic lighting.',
      false,
      0,
      'en-US'
    )

    expect(result?.category).toBe('t2i')
    expect(result?.confidence ?? 0).toBeGreaterThanOrEqual(0.7)
  })

  it('suppresses weak long-form mentions when the signal is only incidental', () => {
    const result = classifyMediaIntent(
      'We should write onboarding docs, describe the workspace rules, and somewhere in the middle mention generate image support for future versions.',
      false,
      0,
      'en-US'
    )

    expect(result).toBeNull()
  })

  it('keeps image edits confident when image context is present', () => {
    const result = classifyMediaIntent(
      'Please edit this image by replacing the background with a warm sunset beach and lightly retouching the colors.',
      true,
      1,
      'en-US'
    )

    expect(result?.category).toBe('i2i')
    expect(result?.confidence ?? 0).toBeGreaterThanOrEqual(0.8)
  })
})
