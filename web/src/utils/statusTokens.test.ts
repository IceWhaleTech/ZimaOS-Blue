import { describe, expect, it } from 'vitest'

import { localizeStatusToken } from '@/utils/statusTokens'

const zhMessages: Record<string, string> = {
  'chat.taskStageCompleted': '已完成',
  'chat.taskStageWorking': '处理中',
  'chat.taskStageFailed': '失败',
  'system.statusOk': '正常',
}

function translate(key: string, fallback: string): string {
  return zhMessages[key] || fallback
}

describe('statusTokens', () => {
  it('localizes wrapped completed tokens', () => {
    expect(localizeStatusToken('[completed]', translate)).toBe('已完成')
    expect(localizeStatusToken('【completed】', translate)).toBe('已完成')
  })

  it('localizes ok tokens', () => {
    expect(localizeStatusToken('ok', translate)).toBe('正常')
    expect(localizeStatusToken('【ok】', translate)).toBe('正常')
  })

  it('keeps unknown tokens human-readable', () => {
    expect(localizeStatusToken('rate_limited', translate)).toBe('Rate limited')
  })
})
