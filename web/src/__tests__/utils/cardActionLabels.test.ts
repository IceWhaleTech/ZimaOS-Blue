import { describe, expect, it } from 'vitest'
import { shouldTranslateCardActionLabel, translateCardActionLabel } from '@/utils/cardActionLabels'

const messages: Record<string, string> = {
  'cardActions.use_browser': '使用浏览器',
  'cardActions.extract_with_web_fetch': '用 Web Fetch 提取',
  'resultCard.actions.extract_with_web_fetch': '从结果卡提取',
}

const te = (key: string) => key in messages
const t = (key: string, fallback?: string) => messages[key] ?? fallback ?? key

describe('card action labels', () => {
  it('prefers scoped translations before generic action keys', () => {
    expect(translateCardActionLabel({
      id: 'extract_with_web_fetch',
      fallback: 'Extract with Web Fetch',
      t,
      te,
      scopes: ['resultCard.actions'],
    })).toBe('从结果卡提取')
  })

  it('falls back to generic action translations', () => {
    expect(translateCardActionLabel({
      id: 'use_browser',
      fallback: 'Use browser',
      t,
      te,
    })).toBe('使用浏览器')
  })

  it('keeps custom labels untouched', () => {
    expect(shouldTranslateCardActionLabel('use_browser', 'Open current browser session')).toBe(false)
    expect(translateCardActionLabel({
      id: 'use_browser',
      fallback: 'Open current browser session',
      t,
      te,
    })).toBe('Open current browser session')
  })
})
