import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const storageState = new Map<string, string>()
const localStorageMock = {
  getItem: (key: string) => storageState.get(key) ?? null,
  setItem: (key: string, value: string) => {
    storageState.set(key, String(value))
  },
  removeItem: (key: string) => {
    storageState.delete(key)
  },
  clear: () => {
    storageState.clear()
  },
}

function installBrowserState(browserLanguage = 'en-US') {
  vi.stubGlobal('localStorage', localStorageMock)
  if (typeof window !== 'undefined') {
    Object.defineProperty(window, 'localStorage', {
      value: localStorageMock,
      configurable: true,
    })
    Object.defineProperty(window.navigator, 'language', {
      value: browserLanguage,
      configurable: true,
    })
    Object.defineProperty(window, '__TAURI_INTERNALS__', {
      value: undefined,
      configurable: true,
      writable: true,
    })
  }

  document.documentElement.lang = ''
  document.documentElement.dir = ''
  document.documentElement.dataset.localeDirection = ''
}

async function loadI18nModule() {
  return import('./index')
}

describe('locale loading', () => {
  beforeEach(() => {
    vi.resetModules()
    vi.clearAllMocks()
    storageState.clear()
    installBrowserState()
  })

  afterEach(() => {
    vi.doUnmock('./locales/en-US')
    vi.doUnmock('./locales/zh-CN')
    vi.unstubAllGlobals()
  })

  it('loads and applies all 27 supported locales', async () => {
    const { getLocale, getLocaleDirection, localeKeys, setLocale } = await loadI18nModule()

    expect(localeKeys).toHaveLength(27)

    for (const locale of localeKeys) {
      await setLocale(locale)

      expect(getLocale()).toBe(locale)
      expect(document.documentElement.lang).toBe(locale)
      expect(document.documentElement.dir).toBe(getLocaleDirection(locale))
      expect(document.documentElement.dataset.localeDirection).toBe(getLocaleDirection(locale))
      expect(localStorageMock.getItem('zimaos-blue-locale')).toBe(locale)
    }
  }, 30000)

  it('setLocale only imports the requested locale when loading succeeds', async () => {
    const enUSImportSpy = vi.fn()
    const zhCNImportSpy = vi.fn()

    vi.doMock('./locales/en-US', () => {
      enUSImportSpy()
      return {
        default: {
          common: {
            loading: 'English Full',
          },
        },
      }
    })
    vi.doMock('./locales/zh-CN', () => {
      zhCNImportSpy()
      return {
        default: {
          common: {
            loading: '中文完整包',
          },
        },
      }
    })

    const { getLocale, i18n, setLocale } = await loadI18nModule()

    await setLocale('zh-CN')

    expect(zhCNImportSpy).toHaveBeenCalledTimes(1)
    expect(enUSImportSpy).not.toHaveBeenCalled()
    expect(getLocale()).toBe('zh-CN')
    expect(i18n.global.t('common.loading')).toBe('中文完整包')
  })

  it('initLocale loads the saved locale without scheduling enhancement work', async () => {
    const requestIdleCallbackSpy = vi.fn()
    storageState.set('zimaos-blue-locale', 'zh-CN')

    if (typeof window !== 'undefined') {
      Object.defineProperty(window, 'requestIdleCallback', {
        value: requestIdleCallbackSpy,
        configurable: true,
      })
    }

    const zhCNImportSpy = vi.fn()
    vi.doMock('./locales/zh-CN', () => {
      zhCNImportSpy()
      return {
        default: {
          common: {
            loading: '中文完整包',
          },
        },
      }
    })

    const { getLocale, initLocale } = await loadI18nModule()

    await initLocale()

    expect(zhCNImportSpy).toHaveBeenCalledTimes(1)
    expect(requestIdleCallbackSpy).not.toHaveBeenCalled()
    expect(getLocale()).toBe('zh-CN')
    expect(document.documentElement.lang).toBe('zh-CN')
  })

  it('falls back to en-US when the requested locale chunk fails to load', async () => {
    const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
    const enUSImportSpy = vi.fn()

    vi.doMock('./locales/en-US', () => {
      enUSImportSpy()
      return {
        default: {
          common: {
            loading: 'English Full',
          },
        },
      }
    })
    vi.doMock('./locales/zh-CN', () => {
      throw new Error('boom')
    })

    const { getLocale, i18n, setLocale } = await loadI18nModule()

    await setLocale('zh-CN')

    expect(enUSImportSpy).toHaveBeenCalledTimes(1)
    expect(getLocale()).toBe('en-US')
    expect(document.documentElement.lang).toBe('en-US')
    expect(localStorageMock.getItem('zimaos-blue-locale')).toBe('en-US')
    expect(i18n.global.t('common.loading')).toBe('English Full')
    expect(warnSpy).toHaveBeenCalled()
  })
})
