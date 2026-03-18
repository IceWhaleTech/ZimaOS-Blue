import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useThemeStore } from '@/stores/theme'

function createMatchMediaMock(matches = false) {
  return vi.fn().mockImplementation(() => ({
    matches,
    media: '(prefers-color-scheme: dark)',
    onchange: null,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }))
}

describe('theme store', () => {
  beforeEach(() => {
    window.localStorage?.removeItem?.('zimaos-blue-theme')
    document.documentElement.className = ''
    delete document.documentElement.dataset.theme
    vi.stubGlobal('matchMedia', createMatchMediaMock(false))
    setActivePinia(createPinia())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    window.localStorage?.removeItem?.('zimaos-blue-theme')
  })

  it('syncs dark theme to both html classes and data-theme', () => {
    const store = useThemeStore()

    store.setTheme('dark')

    expect(document.documentElement.classList.contains('dark')).toBe(true)
    expect(document.documentElement.classList.contains('light')).toBe(false)
    expect(document.documentElement.dataset.theme).toBe('dark')
  })

  it('syncs light theme to both html classes and data-theme', () => {
    const store = useThemeStore()

    store.setTheme('light')

    expect(document.documentElement.classList.contains('light')).toBe(true)
    expect(document.documentElement.classList.contains('dark')).toBe(false)
    expect(document.documentElement.dataset.theme).toBe('light')
  })
})
