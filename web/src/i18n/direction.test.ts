import { describe, expect, it } from 'vitest'

import { getLocaleDirection } from './index'

describe('getLocaleDirection', () => {
  it('treats common RTL language tags as rtl', () => {
    expect(getLocaleDirection('ar-SA')).toBe('rtl')
    expect(getLocaleDirection('fa-IR')).toBe('rtl')
    expect(getLocaleDirection('he-IL')).toBe('rtl')
    expect(getLocaleDirection('ur-PK')).toBe('rtl')
  })

  it('keeps the existing app locales as ltr', () => {
    expect(getLocaleDirection('en-US')).toBe('ltr')
    expect(getLocaleDirection('zh-CN')).toBe('ltr')
    expect(getLocaleDirection('ja-JP')).toBe('ltr')
  })
})
