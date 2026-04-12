import { describe, expect, it } from 'vitest'

import { localeKeys } from './locale-catalog'
import {
  getLocalizedExtraProviderApiFormatLabel,
  getLocalizedProviderCatalogDescription,
  providerCatalogDescriptionIds,
} from './provider-catalog-localization'

describe('provider catalog localization', () => {
  it('returns non-empty localized descriptions for every provider across all 27 locales', () => {
    expect(localeKeys).toHaveLength(27)

    for (const locale of localeKeys) {
      for (const providerId of providerCatalogDescriptionIds) {
        const description = getLocalizedProviderCatalogDescription(locale, providerId)
        expect(typeof description, `${locale} should localize ${providerId}`).toBe('string')
        expect(
          description.trim().length,
          `${locale} should not leave ${providerId} description empty`
        ).toBeGreaterThan(0)
      }
    }
  })

  it('translates complex chooser descriptions outside English locales', () => {
    const englishOllama = getLocalizedProviderCatalogDescription('en-US', 'ollama')
    const englishOpenRouter = getLocalizedProviderCatalogDescription('en-US', 'openrouter')

    for (const locale of localeKeys) {
      if (locale === 'en-US' || locale === 'en-GB') continue

      expect(
        getLocalizedProviderCatalogDescription(locale, 'ollama'),
        `${locale} should localize the Ollama chooser description`
      ).not.toBe(englishOllama)
      expect(
        getLocalizedProviderCatalogDescription(locale, 'openrouter'),
        `${locale} should localize the OpenRouter chooser description`
      ).not.toBe(englishOpenRouter)
    }
  })

  it('normalizes extra API-format chooser tags away from raw lowercase ids', () => {
    for (const locale of localeKeys) {
      expect(
        getLocalizedExtraProviderApiFormatLabel(locale, 'ollama'),
        `${locale} should normalize ollama tag casing`
      ).not.toBe('ollama')
      expect(
        getLocalizedExtraProviderApiFormatLabel(locale, 'cloudcode'),
        `${locale} should replace the raw cloudcode tag`
      ).not.toBe('cloudcode')
      expect(
        getLocalizedExtraProviderApiFormatLabel(locale, 'copilot'),
        `${locale} should replace the raw copilot tag`
      ).not.toBe('copilot')
    }
  })
})
