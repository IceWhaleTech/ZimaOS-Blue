import { describe, expect, it } from 'vitest'
import { classifyMediaIntent, MEDIA_INTENT_SUPPORTED_LOCALES } from './useMediaIntent'

type BenchmarkRow = {
  name: string
  iterations: number
  elapsedMs: number
  avgMs: number
  opsPerSecond: number
}

function round(value: number): number {
  return Math.round(value * 1000) / 1000
}

function buildRow(name: string, iterations: number, elapsedMs: number): BenchmarkRow {
  const avgMs = iterations > 0 ? elapsedMs / iterations : 0
  const opsPerSecond = elapsedMs > 0 ? iterations / (elapsedMs / 1000) : 0
  return {
    name,
    iterations,
    elapsedMs: round(elapsedMs),
    avgMs: round(avgMs),
    opsPerSecond: round(opsPerSecond),
  }
}

describe('useMediaIntent performance', () => {
  it('reports classifier throughput for 27 locales and long-form prompts', () => {
    const deckPrompt = 'nanoslides quarterly strategy summary'
    const englishNoisePrompt =
      'We should write onboarding docs, describe the workspace rules, and somewhere in the middle mention generate image support for future versions, but this paragraph is not a direct generation request and should stay classified as chat text.'
    const chineseNoisePrompt =
      '这是一段很长的产品说明文本，主要在讨论文档整理、消息同步、权限控制和缓存策略。中间顺带提到系统未来也许会支持生成图片能力，但这里并不是在向你发出生成请求。'

    const coldStart = performance.now()
    let coldHits = 0
    for (const locale of MEDIA_INTENT_SUPPORTED_LOCALES) {
      const result = classifyMediaIntent(deckPrompt, false, 0, locale)
      if (result?.category === 't2i' && result.params?.style_preset === 'nano_slides') {
        coldHits++
      }
    }
    const coldElapsed = performance.now() - coldStart

    const warmIterations = 400
    const warmStart = performance.now()
    let warmHits = 0
    for (let iteration = 0; iteration < warmIterations; iteration++) {
      for (const locale of MEDIA_INTENT_SUPPORTED_LOCALES) {
        const result = classifyMediaIntent(deckPrompt, false, 0, locale)
        if (result?.category === 't2i' && result.params?.quality_profile === 'ppt') {
          warmHits++
        }
      }
    }
    const warmElapsed = performance.now() - warmStart

    const longPromptIterations = 800
    const longPromptStart = performance.now()
    let longPromptNulls = 0
    for (let iteration = 0; iteration < longPromptIterations; iteration++) {
      if (classifyMediaIntent(englishNoisePrompt, false, 0, 'en-US') === null) {
        longPromptNulls++
      }
      if (classifyMediaIntent(chineseNoisePrompt, false, 0, 'zh-CN') === null) {
        longPromptNulls++
      }
    }
    const longPromptElapsed = performance.now() - longPromptStart

    const report = [
      buildRow('cold_locale_init', MEDIA_INTENT_SUPPORTED_LOCALES.length, coldElapsed),
      buildRow(
        'warm_ppt_brand_hits',
        MEDIA_INTENT_SUPPORTED_LOCALES.length * warmIterations,
        warmElapsed
      ),
      buildRow('long_prompt_suppression', longPromptIterations * 2, longPromptElapsed),
    ]

    console.info('[useMediaIntent-benchmark]', JSON.stringify(report))

    expect(coldHits).toBe(MEDIA_INTENT_SUPPORTED_LOCALES.length)
    expect(warmHits).toBe(MEDIA_INTENT_SUPPORTED_LOCALES.length * warmIterations)
    expect(longPromptNulls).toBe(longPromptIterations * 2)
  })
})
