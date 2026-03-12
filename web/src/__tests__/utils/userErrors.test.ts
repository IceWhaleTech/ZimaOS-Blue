import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest'
import { getLocale, setLocale } from '@/i18n'
import { getUserErrorMessage } from '@/utils/userErrors'

vi.stubGlobal('localStorage', {
  getItem: () => null,
  setItem: () => {},
  removeItem: () => {},
  clear: () => {},
})

describe('getUserErrorMessage', () => {
  const originalLocale = getLocale()

  beforeAll(async () => {
    await setLocale('en-US')
  })

  afterEach(async () => {
    await setLocale(originalLocale)
  })

  it('translates known backend password errors', async () => {
    await setLocale('zh-CN')

    expect(getUserErrorMessage('password does not meet requirements', 'preview.upgradeFailed')).toBe('密码不符合要求')
  })

  it('returns the localized fallback when the backend message is empty', async () => {
    await setLocale('en-US')

    expect(getUserErrorMessage(undefined, 'preview.upgradeFailed')).toBe('Failed to create admin account')
  })

  it('passes through unknown backend errors', () => {
    expect(getUserErrorMessage('custom failure', 'users.error.createFailed')).toBe('custom failure')
  })
})
