import { createI18n } from 'vue-i18n'

import { deepMergeMessages, type LocaleMessages } from './merge'
import { reportStartupMark } from '@/utils/startupTrace'

// Minimal fallback messages for initial render (before locale loads)
const minimalMessages = {
  common: {
    loading: 'Loading...',
  },
}

export type LocaleKey =
  | 'ca-ES'
  | 'cs-CZ'
  | 'da-DK'
  | 'de-DE'
  | 'el-GR'
  | 'en-GB'
  | 'en-US'
  | 'es-ES'
  | 'fr-FR'
  | 'ga-IE'
  | 'hr-HR'
  | 'hu-HU'
  | 'it-IT'
  | 'ja-JP'
  | 'ko-KR'
  | 'ml-IN'
  | 'nb-NO'
  | 'nl-NL'
  | 'pl-PL'
  | 'pt-BR'
  | 'pt-PT'
  | 'ro-RO'
  | 'ru-RU'
  | 'sk-SK'
  | 'sv-SE'
  | 'zh-CN'
  | 'zh-TW'

const LOCALE_KEY = 'zimaos-blue-locale'
const LOCALE_ENHANCEMENTS_START_DELAY_MS = 2500

// Map browser language codes to our locale keys
const browserLocaleMap: Record<string, LocaleKey> = {
  ca: 'ca-ES',
  cs: 'cs-CZ',
  da: 'da-DK',
  de: 'de-DE',
  el: 'el-GR',
  en: 'en-US',
  'en-GB': 'en-GB',
  'en-US': 'en-US',
  es: 'es-ES',
  fr: 'fr-FR',
  ga: 'ga-IE',
  hr: 'hr-HR',
  hu: 'hu-HU',
  it: 'it-IT',
  ja: 'ja-JP',
  ko: 'ko-KR',
  ml: 'ml-IN',
  nb: 'nb-NO',
  nl: 'nl-NL',
  no: 'nb-NO',
  pl: 'pl-PL',
  pt: 'pt-BR',
  'pt-BR': 'pt-BR',
  'pt-PT': 'pt-PT',
  ro: 'ro-RO',
  ru: 'ru-RU',
  sk: 'sk-SK',
  sv: 'sv-SE',
  zh: 'zh-CN',
  'zh-CN': 'zh-CN',
  'zh-TW': 'zh-TW',
  'zh-HK': 'zh-TW',
}

export const localeOptions = [
  { value: 'ca-ES', label: 'Català' },
  { value: 'cs-CZ', label: 'Čeština' },
  { value: 'da-DK', label: 'Dansk' },
  { value: 'de-DE', label: 'Deutsch' },
  { value: 'el-GR', label: 'Ελληνικά' },
  { value: 'en-GB', label: 'English (UK)' },
  { value: 'en-US', label: 'English (US)' },
  { value: 'es-ES', label: 'Español' },
  { value: 'fr-FR', label: 'Français' },
  { value: 'ga-IE', label: 'Gaeilge' },
  { value: 'hr-HR', label: 'Hrvatski' },
  { value: 'hu-HU', label: 'Magyar' },
  { value: 'it-IT', label: 'Italiano' },
  { value: 'ja-JP', label: '日本語' },
  { value: 'ko-KR', label: '한국어' },
  { value: 'ml-IN', label: 'മലയാളം' },
  { value: 'nb-NO', label: 'Norsk bokmål' },
  { value: 'nl-NL', label: 'Nederlands' },
  { value: 'pl-PL', label: 'Polski' },
  { value: 'pt-BR', label: 'Português (Brasil)' },
  { value: 'pt-PT', label: 'Português (Portugal)' },
  { value: 'ro-RO', label: 'Română' },
  { value: 'ru-RU', label: 'Русский' },
  { value: 'sk-SK', label: 'Slovenčina' },
  { value: 'sv-SE', label: 'Svenska' },
  { value: 'zh-CN', label: '简体中文' },
  { value: 'zh-TW', label: '繁體中文' },
] as const

function getDefaultLocale(): LocaleKey {
  const saved = localStorage.getItem(LOCALE_KEY)
  if (saved && localeOptions.some((o) => o.value === saved)) {
    return saved as LocaleKey
  }

  const browserLang = navigator.language
  // Try exact match first
  const exactMatch = browserLocaleMap[browserLang]
  if (exactMatch) {
    return exactMatch
  }
  // Try language code only
  const langCode = browserLang.split('-')[0]
  if (langCode) {
    const langMatch = browserLocaleMap[langCode]
    if (langMatch) {
      return langMatch
    }
  }

  return 'en-US'
}

export const i18n = createI18n({
  legacy: false,
  locale: 'en-US', // Start with en-US, will switch in initLocale after loading
  fallbackLocale: 'en-US',
  missingWarn: false,
  fallbackWarn: false,
  messages: {
    'en-US': minimalMessages, // Minimal messages, full locale loaded async
  },
})

type LocaleComposerBridge = {
  setLocaleMessage: (locale: string, message: LocaleMessages) => void
  getLocaleMessage: (locale: string) => LocaleMessages
  locale: { value: string }
}

type LocaleMessageMap = Record<string, LocaleMessages>

function createLocaleMessageMapLoader(
  loader: () => Promise<{ default: LocaleMessageMap }>
): () => Promise<LocaleMessageMap> {
  let pending: Promise<LocaleMessageMap> | null = null
  return () => {
    if (!pending) {
      pending = loader().then((module) => module.default as LocaleMessageMap)
    }
    return pending
  }
}

const loadPriorityLocaleOverrides = createLocaleMessageMapLoader(
  () => import('./priority-overrides')
)
const loadPriorityBillingOverrides = createLocaleMessageMapLoader(
  () => import('./priority-billing-overrides')
)
const loadPrioritySettingsOverrides = createLocaleMessageMapLoader(
  () => import('./priority-settings-overrides')
)
const loadContextCompressionOverrides = createLocaleMessageMapLoader(
  () => import('./context-compression-overrides')
)
const loadPriorityTranslationOverrides = createLocaleMessageMapLoader(
  () => import('./priority-translation-overrides')
)
const loadPresetQuestionContentOverrides = createLocaleMessageMapLoader(
  () => import('./preset-question-content-overrides')
)
const loadPrioritySmallModelOverrides = createLocaleMessageMapLoader(
  () => import('./priority-small-model-overrides')
)
const loadMediaFallbackOverrides = createLocaleMessageMapLoader(
  () => import('./media-fallback-overrides')
)
const loadResultCardMessageOverrides = createLocaleMessageMapLoader(
  () => import('./result-card-message-overrides')
)
const loadResearchToolOverrides = createLocaleMessageMapLoader(
  () => import('./research-tool-overrides')
)
const loadSecurityScanDetailOverrides = createLocaleMessageMapLoader(
  () => import('./security-scan-detail-overrides')
)
const loadSkillToolOverrides = createLocaleMessageMapLoader(() => import('./skill-tool-overrides'))
const loadSkillStoreMarketplaceOverrides = createLocaleMessageMapLoader(
  () => import('./skill-store-marketplace-overrides')
)
const loadSelfReflectProposalOverrides = createLocaleMessageMapLoader(
  () => import('./self-reflect-proposal-overrides')
)
const loadApiProxyPrunerOverrides = createLocaleMessageMapLoader(
  () => import('./api-proxy-pruner-overrides')
)
const loadMemorySurfaceOverrides = createLocaleMessageMapLoader(
  () => import('./memory-surface-overrides')
)
const loadRalphLoopHoverOverrides = createLocaleMessageMapLoader(
  () => import('./ralph-loop-hover-overrides')
)
const loadExtensionsBrowseOverrides = createLocaleMessageMapLoader(
  () => import('./extensions-browse-overrides')
)

const loadedBaseLocales = new Set<LocaleKey>()
const loadingBaseLocales = new Map<LocaleKey, Promise<void>>()
const loadedLocaleEnhancements = new Set<LocaleKey>()
const loadingLocaleEnhancements = new Map<LocaleKey, Promise<void>>()

function applyLocaleState(locale: LocaleKey): void {
  ;(i18n.global as unknown as LocaleComposerBridge).locale.value = locale
  localStorage.setItem(LOCALE_KEY, locale)
  document.documentElement.lang = locale

  if (window.__TAURI_INTERNALS__?.invoke) {
    window.__TAURI_INTERNALS__.invoke('set_tray_locale', { locale }).catch(() => {})
  }
}

async function loadLocaleBaseMessages(locale: LocaleKey): Promise<void> {
  if (loadedBaseLocales.has(locale)) {
    return
  }

  const existing = loadingBaseLocales.get(locale)
  if (existing) {
    return existing
  }

  const pending = (async () => {
    try {
      if (locale !== 'en-US') {
        await loadLocaleBaseMessages('en-US')
      }

      const messages = await import(`./locales/${locale}.ts`)
      const localeMessages = messages.default as LocaleMessages
      const i18nGlobal = i18n.global as unknown as LocaleComposerBridge
      const apiProxyPrunerOverrides = (await loadApiProxyPrunerOverrides())[locale] || {}
      const memorySurfaceOverrides = (await loadMemorySurfaceOverrides())[locale] || {}
      const ralphLoopHoverOverrides = (await loadRalphLoopHoverOverrides())[locale] || {}
      const mergedSelfReflectProposalOverrides =
        locale === 'en-US' || locale === 'zh-CN'
          ? {}
          : (await loadSelfReflectProposalOverrides())[locale] || {}
      const mergedBaseMessages =
        locale === 'en-US'
          ? localeMessages
          : deepMergeMessages<LocaleMessages>(i18nGlobal.getLocaleMessage('en-US'), localeMessages)
      const localizedBaseMessages = deepMergeMessages<LocaleMessages>(
        mergedBaseMessages,
        mergedSelfReflectProposalOverrides
      )
      const fullyLocalizedBaseMessages = deepMergeMessages<LocaleMessages>(
        localizedBaseMessages,
        apiProxyPrunerOverrides
      )
      const fullyLocalizedBaseMessagesWithMemorySurface = deepMergeMessages<LocaleMessages>(
        fullyLocalizedBaseMessages,
        memorySurfaceOverrides
      )
      const fullyLocalizedBaseMessagesWithRalphLoop = deepMergeMessages<LocaleMessages>(
        fullyLocalizedBaseMessagesWithMemorySurface,
        ralphLoopHoverOverrides
      )

      i18nGlobal.setLocaleMessage(locale, fullyLocalizedBaseMessagesWithRalphLoop)
      loadedBaseLocales.add(locale)
    } catch (error) {
      console.warn(`Failed to load locale base ${locale}, falling back to en-US`, error)
    } finally {
      loadingBaseLocales.delete(locale)
    }
  })()

  loadingBaseLocales.set(locale, pending)
  return pending
}

async function loadLocaleEnhancements(locale: LocaleKey): Promise<void> {
  if (loadedLocaleEnhancements.has(locale)) {
    return
  }

  const existing = loadingLocaleEnhancements.get(locale)
  if (existing) {
    return existing
  }

  const pending = (async () => {
    try {
      await loadLocaleBaseMessages(locale)

      const [
        priorityLocaleOverrides,
        resultCardMessageOverrides,
        securityScanDetailOverrides,
        priorityBillingOverrides,
        prioritySettingsOverrides,
        contextCompressionOverrides,
        priorityTranslationOverrides,
        presetQuestionContentOverrides,
        mediaFallbackOverrides,
        skillToolOverrides,
        researchToolOverrides,
        skillStoreMarketplaceOverrides,
        prioritySmallModelOverrides,
        extensionsBrowseOverrides,
      ] = await Promise.all([
        loadPriorityLocaleOverrides(),
        loadResultCardMessageOverrides(),
        loadSecurityScanDetailOverrides(),
        loadPriorityBillingOverrides(),
        loadPrioritySettingsOverrides(),
        loadContextCompressionOverrides(),
        loadPriorityTranslationOverrides(),
        loadPresetQuestionContentOverrides(),
        loadMediaFallbackOverrides(),
        loadSkillToolOverrides(),
        loadResearchToolOverrides(),
        loadSkillStoreMarketplaceOverrides(),
        loadPrioritySmallModelOverrides(),
        loadExtensionsBrowseOverrides(),
      ])

      const localeOverrides = priorityLocaleOverrides[locale] || {}
      const localizedResultCardMessageOverrides = resultCardMessageOverrides[locale] || {}
      const localizedSecurityScanDetailOverrides = securityScanDetailOverrides[locale] || {}
      const billingOverrides = priorityBillingOverrides[locale] || {}
      const settingsOverrides = prioritySettingsOverrides[locale] || {}
      const localizedContextCompressionOverrides = contextCompressionOverrides[locale] || {}
      const translationOverrides = priorityTranslationOverrides[locale] || {}
      const localizedPresetQuestionContentOverrides = presetQuestionContentOverrides[locale] || {}
      const localizedMediaFallbackOverrides = mediaFallbackOverrides[locale] || {}
      const localizedSkillToolOverrides = skillToolOverrides[locale] || {}
      const localizedResearchToolOverrides = researchToolOverrides[locale] || {}
      const localizedSkillStoreMarketplaceOverrides = skillStoreMarketplaceOverrides[locale] || {}
      const smallModelOverrides = prioritySmallModelOverrides[locale] || {}
      const localizedExtensionsBrowseOverrides = extensionsBrowseOverrides[locale] || {}
      const i18nGlobal = i18n.global as unknown as LocaleComposerBridge
      const currentMessages = i18nGlobal.getLocaleMessage(locale)
      const withPriorityOverrides = deepMergeMessages<LocaleMessages>(
        currentMessages,
        localeOverrides
      )
      const withResultCardMessageOverrides = deepMergeMessages<LocaleMessages>(
        withPriorityOverrides,
        localizedResultCardMessageOverrides
      )
      const withSecurityScanDetailOverrides = deepMergeMessages<LocaleMessages>(
        withResultCardMessageOverrides,
        localizedSecurityScanDetailOverrides
      )
      const withBillingOverrides = deepMergeMessages<LocaleMessages>(
        withSecurityScanDetailOverrides,
        billingOverrides
      )
      const withSettingsOverrides = deepMergeMessages<LocaleMessages>(
        withBillingOverrides,
        settingsOverrides
      )
      const withContextCompressionOverrides = deepMergeMessages<LocaleMessages>(
        withSettingsOverrides,
        localizedContextCompressionOverrides
      )
      const withTranslationOverrides = deepMergeMessages<LocaleMessages>(
        withContextCompressionOverrides,
        translationOverrides
      )
      const withPresetQuestionContentOverrides = deepMergeMessages<LocaleMessages>(
        withTranslationOverrides,
        localizedPresetQuestionContentOverrides
      )
      const withMediaFallbackOverrides = deepMergeMessages<LocaleMessages>(
        withPresetQuestionContentOverrides,
        localizedMediaFallbackOverrides
      )
      const withSkillToolOverrides = deepMergeMessages<LocaleMessages>(
        withMediaFallbackOverrides,
        localizedSkillToolOverrides
      )
      const withResearchToolOverrides = deepMergeMessages<LocaleMessages>(
        withSkillToolOverrides,
        localizedResearchToolOverrides
      )
      const withSkillStoreMarketplaceOverrides = deepMergeMessages<LocaleMessages>(
        withResearchToolOverrides,
        localizedSkillStoreMarketplaceOverrides
      )
      const mergedMessages = deepMergeMessages<LocaleMessages>(
        withSkillStoreMarketplaceOverrides,
        smallModelOverrides
      )
      const fullyMergedMessages = deepMergeMessages<LocaleMessages>(
        mergedMessages,
        localizedExtensionsBrowseOverrides
      )

      i18nGlobal.setLocaleMessage(locale, fullyMergedMessages)
      loadedLocaleEnhancements.add(locale)
    } catch (error) {
      console.warn(`Failed to load locale enhancements for ${locale}`, error)
    } finally {
      loadingLocaleEnhancements.delete(locale)
    }
  })()

  loadingLocaleEnhancements.set(locale, pending)
  return pending
}

export async function setLocale(locale: LocaleKey): Promise<void> {
  // Always ensure fallback locale is fully loaded so missing keys in other locales
  // cleanly fall back to English instead of showing raw translation keys.
  await loadLocaleBaseMessages('en-US')
  await loadLocaleBaseMessages(locale)
  await loadLocaleEnhancements('en-US')
  if (locale !== 'en-US') {
    await loadLocaleEnhancements(locale)
  }
  applyLocaleState(locale)
}

export function getLocale(): LocaleKey {
  return (i18n.global as unknown as LocaleComposerBridge).locale.value as LocaleKey
}

// Initialize: always load the default locale (including en-US)
export async function initLocale(): Promise<void> {
  const defaultLocale = getDefaultLocale()
  await loadLocaleBaseMessages('en-US')
  await loadLocaleBaseMessages(defaultLocale)
  applyLocaleState(defaultLocale)

  const scheduleEnhancements = (fn: () => void) => {
    const runWhenIdle = () => {
      if (typeof window !== 'undefined' && 'requestIdleCallback' in window) {
        ;(
          window as Window & {
            requestIdleCallback: (cb: () => void, options?: { timeout?: number }) => number
          }
        ).requestIdleCallback(fn, {
          timeout: LOCALE_ENHANCEMENTS_START_DELAY_MS,
        })
        return
      }
      setTimeout(fn, 0)
    }

    setTimeout(runWhenIdle, LOCALE_ENHANCEMENTS_START_DELAY_MS)
  }

  scheduleEnhancements(() => {
    reportStartupMark('locale_enhancements_start')
    void loadLocaleEnhancements('en-US')
    if (defaultLocale !== 'en-US') {
      void loadLocaleEnhancements(defaultLocale)
    }
  })
}
