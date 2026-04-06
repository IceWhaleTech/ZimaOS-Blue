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
    vi.unstubAllEnvs()
    storageState.clear()
    installBrowserState()
  })

  afterEach(() => {
    vi.doUnmock('./locales/en-US')
    vi.doUnmock('./locales/zh-CN')
    vi.doUnmock('./harness-locale-additions')
    vi.unstubAllEnvs()
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

  it('keeps all 27 locales available for embedded builds too', async () => {
    vi.stubEnv('VITE_EMBED_DISABLE_MERMAID', '1')

    const { getLocale, localeKeys, localeOptions } = await loadI18nModule()

    expect(localeKeys).toHaveLength(27)
    expect(localeOptions).toHaveLength(27)
    expect(localeKeys).toContain('fr-FR')
    expect(localeKeys).toContain('zh-TW')
    expect(getLocale()).toBe('en-US')
  })

  it('preserves supported saved and browser locales for embedded builds', async () => {
    vi.stubEnv('VITE_EMBED_DISABLE_MERMAID', '1')

    storageState.set('zimaos-blue-locale', 'fr-FR')
    installBrowserState('fr-FR')

    const withSavedLocale = await loadI18nModule()

    expect(withSavedLocale.getLocale()).toBe('fr-FR')

    vi.resetModules()
    storageState.clear()
    vi.stubEnv('VITE_EMBED_DISABLE_MERMAID', '1')
    installBrowserState('zh-HK')

    const withChineseBrowserLocale = await loadI18nModule()

    expect(withChineseBrowserLocale.getLocale()).toBe('zh-TW')
  })

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

  it('initLocale hydrates a cached locale immediately and refreshes it in idle time', async () => {
    storageState.set('zimaos-blue-locale', 'zh-CN')
    storageState.set(
      'zimaos-blue-locale-cache:v1:zh-CN',
      JSON.stringify({
        common: {
          loading: '缓存中文',
        },
      })
    )

    const idleRefresh = {
      callback: null as
        | null
        | ((deadline: { didTimeout: boolean; timeRemaining: () => number }) => void),
    }
    const requestIdleCallbackSpy = vi.fn(
      (callback: (deadline: { didTimeout: boolean; timeRemaining: () => number }) => void) => {
        idleRefresh.callback = callback
        return 1
      }
    )

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

    const { getLocale, i18n, initLocale } = await loadI18nModule()

    expect(i18n.global.t('common.loading')).toBe('缓存中文')

    await initLocale()

    expect(getLocale()).toBe('zh-CN')
    expect(document.documentElement.lang).toBe('zh-CN')
    expect(requestIdleCallbackSpy).toHaveBeenCalledTimes(1)
    expect(zhCNImportSpy).not.toHaveBeenCalled()
    expect(i18n.global.t('common.loading')).toBe('缓存中文')

    if (!idleRefresh.callback) {
      throw new Error('expected requestIdleCallback to schedule a refresh')
    }

    idleRefresh.callback({
      didTimeout: false,
      timeRemaining: () => 50,
    })
    await vi.waitFor(() => {
      expect(zhCNImportSpy).toHaveBeenCalledTimes(1)
      expect(i18n.global.t('common.loading')).toBe('中文完整包')
    })
  })

  it('delays cached desktop chat locale refresh before loading heavy enhancements', async () => {
    storageState.set('zimaos-blue-locale', 'zh-CN')
    storageState.set(
      'zimaos-blue-locale-cache:v1:zh-CN',
      JSON.stringify({
        common: {
          loading: '缓存中文',
        },
      })
    )

    const scheduledCallbacks: Array<() => void> = []
    const idleCallbacks: Array<
      (deadline: { didTimeout: boolean; timeRemaining: () => number }) => void
    > = []
    const requestIdleCallbackSpy = vi.fn(
      (callback: (deadline: { didTimeout: boolean; timeRemaining: () => number }) => void) => {
        idleCallbacks.push(callback)
        return idleCallbacks.length
      }
    )
    const setTimeoutSpy = vi.fn((callback: () => void) => {
      scheduledCallbacks.push(callback)
      return scheduledCallbacks.length
    })

    if (typeof window !== 'undefined') {
      Object.defineProperty(window, '__BLUE_DESKTOP__', {
        value: true,
        configurable: true,
        writable: true,
      })
      Object.defineProperty(window, 'requestIdleCallback', {
        value: requestIdleCallbackSpy,
        configurable: true,
      })
      Object.defineProperty(window, 'setTimeout', {
        value: setTimeoutSpy,
        configurable: true,
      })
      window.history.replaceState({}, '', '/chat')
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

    const enhancementSpy = vi.fn((locale: string, messages: Record<string, unknown>) => ({
      ...messages,
      harness: {
        groups: {
          status: `${locale}-enhanced`,
        },
      },
    }))
    vi.doMock('./harness-locale-additions', () => ({
      mergeHarnessLocale: enhancementSpy,
    }))

    const { getLocale, i18n, initLocale } = await loadI18nModule()

    await initLocale()

    expect(getLocale()).toBe('zh-CN')
    expect(i18n.global.t('common.loading')).toBe('缓存中文')
    expect(zhCNImportSpy).not.toHaveBeenCalled()
    expect(enhancementSpy).not.toHaveBeenCalled()
    expect(setTimeoutSpy).toHaveBeenCalledTimes(1)
    expect(requestIdleCallbackSpy).not.toHaveBeenCalled()

    const delayedRefresh = scheduledCallbacks.shift()
    if (!delayedRefresh) {
      throw new Error('expected delayed refresh scheduling for cached desktop locale')
    }
    delayedRefresh()

    expect(requestIdleCallbackSpy).toHaveBeenCalledTimes(1)

    const refreshIdle = idleCallbacks.shift()
    if (!refreshIdle) {
      throw new Error('expected idle refresh callback after delayed desktop locale refresh')
    }
    refreshIdle({
      didTimeout: false,
      timeRemaining: () => 50,
    })

    await vi.waitFor(() => {
      expect(zhCNImportSpy).toHaveBeenCalledTimes(1)
      expect(i18n.global.t('common.loading')).toBe('中文完整包')
    })

    expect(enhancementSpy).not.toHaveBeenCalled()
    expect(setTimeoutSpy).toHaveBeenCalledTimes(2)

    const delayedEnhancement = scheduledCallbacks.shift()
    if (!delayedEnhancement) {
      throw new Error('expected delayed enhancement scheduling after cached locale refresh')
    }
    delayedEnhancement()

    expect(requestIdleCallbackSpy).toHaveBeenCalledTimes(2)

    const enhancementIdle = idleCallbacks.shift()
    if (!enhancementIdle) {
      throw new Error('expected idle enhancement callback after delayed enhancement scheduling')
    }
    enhancementIdle({
      didTimeout: false,
      timeRemaining: () => 50,
    })

    await vi.waitFor(() => {
      expect(enhancementSpy).toHaveBeenCalledTimes(1)
      expect(i18n.global.t('harness.groups.status')).toBe('zh-CN-enhanced')
    })
  })

  it('defers heavy locale enhancements for desktop chat startup even without a stored session hint', async () => {
    storageState.set('zimaos-blue-locale', 'zh-CN')

    const delayedEnhancement = {
      callback: null as null | (() => void),
    }
    const idleRefresh = {
      callback: null as
        | null
        | ((deadline: { didTimeout: boolean; timeRemaining: () => number }) => void),
    }
    const requestIdleCallbackSpy = vi.fn(
      (callback: (deadline: { didTimeout: boolean; timeRemaining: () => number }) => void) => {
        idleRefresh.callback = callback
        return 1
      }
    )
    const setTimeoutSpy = vi.fn((callback: () => void) => {
      delayedEnhancement.callback = callback
      return 1
    })

    if (typeof window !== 'undefined') {
      Object.defineProperty(window, '__BLUE_DESKTOP__', {
        value: true,
        configurable: true,
        writable: true,
      })
      Object.defineProperty(window, 'requestIdleCallback', {
        value: requestIdleCallbackSpy,
        configurable: true,
      })
      Object.defineProperty(window, 'setTimeout', {
        value: setTimeoutSpy,
        configurable: true,
      })
      window.history.replaceState({}, '', '/chat')
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

    const enhancementSpy = vi.fn((locale: string, messages: Record<string, unknown>) => ({
      ...messages,
      harness: {
        groups: {
          status: `${locale}-enhanced`,
        },
      },
    }))
    vi.doMock('./harness-locale-additions', () => ({
      mergeHarnessLocale: enhancementSpy,
    }))

    const { getLocale, i18n, initLocale } = await loadI18nModule()

    await initLocale()

    expect(getLocale()).toBe('zh-CN')
    expect(zhCNImportSpy).toHaveBeenCalledTimes(1)
    expect(enhancementSpy).not.toHaveBeenCalled()
    expect(setTimeoutSpy).toHaveBeenCalledTimes(1)
    expect(requestIdleCallbackSpy).not.toHaveBeenCalled()
    expect(i18n.global.t('common.loading')).toBe('中文完整包')

    if (!delayedEnhancement.callback) {
      throw new Error('expected delayed enhancement scheduling before idle callback')
    }

    delayedEnhancement.callback()

    expect(requestIdleCallbackSpy).toHaveBeenCalledTimes(1)

    if (!idleRefresh.callback) {
      throw new Error('expected requestIdleCallback to schedule locale enhancements')
    }

    idleRefresh.callback({
      didTimeout: false,
      timeRemaining: () => 50,
    })

    await vi.waitFor(() => {
      expect(enhancementSpy).toHaveBeenCalledTimes(1)
      expect(i18n.global.t('harness.groups.status')).toBe('zh-CN-enhanced')
    })
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
