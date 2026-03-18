import { createI18n } from 'vue-i18n'

import { deepMergeMessages, type LocaleMessages } from './merge'
import priorityLocaleOverrides from './priority-overrides'
import priorityBillingOverrides from './priority-billing-overrides'
import prioritySettingsOverrides from './priority-settings-overrides'
import priorityTranslationOverrides from './priority-translation-overrides'
import prioritySmallModelOverrides from './priority-small-model-overrides'
import securityScanDetailOverrides from './security-scan-detail-overrides'
import skillToolOverrides from './skill-tool-overrides'
import skillStoreMarketplaceOverrides from './skill-store-marketplace-overrides'

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

// Track loaded locales
const loadedLocales = new Set<LocaleKey>()

// Lazy load locale messages
async function loadLocaleMessages(locale: LocaleKey): Promise<void> {
  if (loadedLocales.has(locale)) {
    return
  }

  try {
    if (locale !== 'en-US') {
      await loadLocaleMessages('en-US')
    }

    const messages = await import(`./locales/${locale}.ts`)
    const localeMessages = messages.default as LocaleMessages
    const localeOverrides =
      (priorityLocaleOverrides as Record<string, LocaleMessages>)[locale] || {}
    const localizedSecurityScanDetailOverrides =
      (securityScanDetailOverrides as Record<string, LocaleMessages>)[locale] || {}
    const billingOverrides =
      (priorityBillingOverrides as Record<string, LocaleMessages>)[locale] || {}
    const settingsOverrides =
      (prioritySettingsOverrides as Record<string, LocaleMessages>)[locale] || {}
    const translationOverrides =
      (priorityTranslationOverrides as Record<string, LocaleMessages>)[locale] || {}
    const localizedSkillToolOverrides =
      (skillToolOverrides as Record<string, LocaleMessages>)[locale] || {}
    const localizedSkillStoreMarketplaceOverrides =
      (skillStoreMarketplaceOverrides as Record<string, LocaleMessages>)[locale] || {}
    const smallModelOverrides =
      (prioritySmallModelOverrides as Record<string, LocaleMessages>)[locale] || {}
    const i18nGlobal = i18n.global as unknown as LocaleComposerBridge
    const mergedBaseMessages =
      locale === 'en-US'
        ? localeMessages
        : deepMergeMessages<LocaleMessages>(i18nGlobal.getLocaleMessage('en-US'), localeMessages)
    const withPriorityOverrides = deepMergeMessages<LocaleMessages>(
      mergedBaseMessages,
      localeOverrides
    )
    const withSecurityScanDetailOverrides = deepMergeMessages<LocaleMessages>(
      withPriorityOverrides,
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
    const withTranslationOverrides = deepMergeMessages<LocaleMessages>(
      withSettingsOverrides,
      translationOverrides
    )
    const withSkillToolOverrides = deepMergeMessages<LocaleMessages>(
      withTranslationOverrides,
      localizedSkillToolOverrides
    )
    const withSkillStoreMarketplaceOverrides = deepMergeMessages<LocaleMessages>(
      withSkillToolOverrides,
      localizedSkillStoreMarketplaceOverrides
    )
    const mergedMessages = deepMergeMessages<LocaleMessages>(
      withSkillStoreMarketplaceOverrides,
      smallModelOverrides
    )

    i18nGlobal.setLocaleMessage(locale, mergedMessages)
    loadedLocales.add(locale)
  } catch (error) {
    console.warn(`Failed to load locale ${locale}, falling back to en-US`, error)
  }
}

export async function setLocale(locale: LocaleKey): Promise<void> {
  // Always ensure fallback locale is fully loaded so missing keys in other locales
  // cleanly fall back to English instead of showing raw translation keys.
  await loadLocaleMessages('en-US')
  await loadLocaleMessages(locale)
  ;(i18n.global as unknown as LocaleComposerBridge).locale.value = locale
  localStorage.setItem(LOCALE_KEY, locale)
  document.documentElement.lang = locale

  // Sync tray menu language in Tauri desktop app
  if (window.__TAURI_INTERNALS__?.invoke) {
    window.__TAURI_INTERNALS__.invoke('set_tray_locale', { locale }).catch(() => {})
  }
}

export function getLocale(): LocaleKey {
  return (i18n.global as unknown as LocaleComposerBridge).locale.value as LocaleKey
}

// Initialize: always load the default locale (including en-US)
export async function initLocale(): Promise<void> {
  const defaultLocale = getDefaultLocale()
  await setLocale(defaultLocale)
}
