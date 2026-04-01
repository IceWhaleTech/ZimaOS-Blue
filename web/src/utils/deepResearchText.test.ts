import { describe, expect, it } from 'vitest'

import {
  localizeDeepResearchSegment,
  localizeDeepResearchStatus,
  localizeResearchProgressLabel,
  localizeResearchRunningElsewhereLabel,
  localizeResearchSurfaceTitle,
} from '@/utils/deepResearchText'

const zhMessages: Record<string, string> = {
  'chat.deepResearchStageCompleted': '已完成',
  'chat.deepResearchActionCompleted': '已完成',
}

function translate(key: string, fallback: string): string {
  return zhMessages[key] || fallback
}

describe('deepResearchText', () => {
  it('localizes bracket-wrapped status tokens', () => {
    expect(localizeDeepResearchStatus('[completed]', translate)).toBe('已完成')
    expect(localizeDeepResearchStatus('【completed】', translate)).toBe('已完成')
  })

  it('localizes bracket-wrapped segment tokens', () => {
    expect(localizeDeepResearchSegment('[completed]', translate)).toBe('已完成')
    expect(localizeDeepResearchSegment('【completed】', translate)).toBe('已完成')
  })

  it('prefers deep research title and progress labels when available', () => {
    const translateDeepResearch = (key: string, fallback: string): string =>
      (
        ({
          'chat.deepResearchTitle': '深度研究',
          'chat.deepResearchProgress': '深度研究进行中',
          'chat.deepResearchRunningElsewhere': '跨会话跟踪当前进行中的深度研究任务。',
        }) as Record<string, string>
      )[key] || fallback

    expect(localizeResearchSurfaceTitle(translateDeepResearch)).toBe('深度研究')
    expect(localizeResearchProgressLabel(translateDeepResearch)).toBe('深度研究进行中')
    expect(localizeResearchRunningElsewhereLabel(translateDeepResearch)).toBe(
      '跨会话跟踪当前进行中的深度研究任务。'
    )
  })

  it('falls back to generic research aliases when deep research labels are absent', () => {
    const translateResearch = (key: string, fallback: string): string =>
      (
        ({
          'chat.researchTitle': '研究',
          'chat.researchProgress': '研究进行中',
          'chat.researchRunningElsewhere': '跨会话跟踪当前进行中的研究任务。',
        }) as Record<string, string>
      )[key] || fallback

    expect(localizeResearchSurfaceTitle(translateResearch)).toBe('研究')
    expect(localizeResearchProgressLabel(translateResearch)).toBe('研究进行中')
    expect(localizeResearchRunningElsewhereLabel(translateResearch)).toBe('跨会话跟踪当前进行中的研究任务。')
  })
})
