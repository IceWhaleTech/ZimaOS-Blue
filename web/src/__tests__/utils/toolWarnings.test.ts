import { describe, expect, it } from 'vitest'

import { formatToolWarningCodeLabel } from '@/utils/toolWarnings'

const translate = (key: string, fallback?: string) => {
  const messages: Record<string, string> = {
    'toolWarnings.warning': '警告',
    'toolWarnings.loginWall': '需要登录',
    'toolWarnings.challenge': '验证',
    'toolWarnings.browserRequired': '需要浏览器',
  }
  return messages[key] || fallback || key
}

describe('formatToolWarningCodeLabel', () => {
  it('translates known warning codes', () => {
    expect(formatToolWarningCodeLabel('login_wall', translate)).toBe('需要登录')
    expect(formatToolWarningCodeLabel('challenge', translate)).toBe('验证')
  })

  it('keeps raw badge formatting for unknown codes by default', () => {
    expect(formatToolWarningCodeLabel('custom_warning', translate)).toBe(
      'warning_code=custom_warning'
    )
  })

  it('formats unknown codes as human labels in label mode', () => {
    expect(formatToolWarningCodeLabel('custom_warning', translate, 'label')).toBe('custom warning')
  })
})
