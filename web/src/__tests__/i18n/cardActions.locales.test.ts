import { describe, expect, it } from 'vitest'

const localeModules = import.meta.glob('@/i18n/locales/*.ts', { eager: true }) as Record<string, { default: any }>
const REQUIRED_CARD_ACTION_KEYS = [
  'use_browser',
  'extract_with_web_fetch',
  'recheck',
  'check_a11y',
  'full_report',
] as const

const REQUIRED_COMPANION_TOAST_KEYS = [
  'memorySavedTitle',
  'memorySavedMessage',
  'manageMemory',
] as const

describe('cardActions locale coverage', () => {
  it('exposes required generic action keys in every locale module', () => {
    const entries = Object.entries(localeModules)
    expect(entries.length).toBeGreaterThan(0)

    for (const [path, mod] of entries) {
      const cardActions = mod.default?.cardActions
      expect(cardActions, `${path} missing cardActions`).toBeTruthy()
      for (const key of REQUIRED_CARD_ACTION_KEYS) {
        expect(cardActions?.[key], `${path} missing cardActions.${key}`).toBeTruthy()
      }
    }
  })

  it('exposes required companion toast keys in every locale module', () => {
    const entries = Object.entries(localeModules)
    expect(entries.length).toBeGreaterThan(0)

    for (const [path, mod] of entries) {
      const toasts = mod.default?.companion?.toasts
      expect(toasts, `${path} missing companion.toasts`).toBeTruthy()
      for (const key of REQUIRED_COMPANION_TOAST_KEYS) {
        expect(toasts?.[key], `${path} missing companion.toasts.${key}`).toBeTruthy()
      }
    }
  })
})
