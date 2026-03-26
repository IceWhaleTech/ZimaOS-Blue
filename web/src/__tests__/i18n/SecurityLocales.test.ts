import { describe, expect, it } from 'vitest'
import { createI18n } from 'vue-i18n'

type LocaleLeaf = string | number | boolean | null | undefined
type LocaleValue = LocaleLeaf | LocaleNode | LocaleLeaf[] | LocaleNode[]
interface LocaleNode {
  [key: string]: LocaleValue
}

type LocaleMessages = Record<string, unknown>

const localeModules = import.meta.glob('@/i18n/locales/*.ts', { eager: true }) as Record<
  string,
  { default: LocaleMessages }
>

function fileNameFromModulePath(modulePath: string): string {
  return modulePath.split('/').pop() ?? modulePath
}

function getPathValue(messages: LocaleMessages, path: string): unknown {
  return path.split('.').reduce<unknown>((current, segment) => {
    if (current && typeof current === 'object' && segment in current) {
      return (current as Record<string, unknown>)[segment]
    }
    return undefined
  }, messages)
}

function collectLeafKeys(value: unknown, prefix = ''): string[] {
  if (Array.isArray(value)) {
    return []
  }
  if (!value || typeof value !== 'object') {
    return prefix ? [prefix] : []
  }

  const keys: string[] = []
  for (const [key, child] of Object.entries(value as LocaleNode)) {
    const nextPrefix = prefix ? `${prefix}.${key}` : key
    if (Array.isArray(child)) {
      continue
    }
    if (child && typeof child === 'object') {
      keys.push(...collectLeafKeys(child, nextPrefix))
      continue
    }
    keys.push(nextPrefix)
  }
  return keys
}

describe('security locale compilation', () => {
  it('compiles every security and apiProxy string in every locale', () => {
    const entries = Object.entries(localeModules).sort(([a], [b]) => a.localeCompare(b))
    expect(entries.length).toBe(27)

    const enUSPath = entries.find(([modulePath]) => modulePath.endsWith('/en-US.ts'))?.[0]
    expect(enUSPath).toBeTruthy()
    if (!enUSPath) {
      throw new Error('Missing en-US locale module')
    }

    const referenceMessages = localeModules[enUSPath]?.default
    expect(referenceMessages).toBeTruthy()
    if (!referenceMessages) {
      throw new Error('Missing en-US locale messages')
    }

    const namespaces = ['security', 'apiProxy'] as const
    const requiredKeys = namespaces.flatMap((namespace) => {
      const namespaceValue = getPathValue(referenceMessages, namespace)
      return collectLeafKeys(namespaceValue).map((key) => `${namespace}.${key}`)
    })

    expect(requiredKeys.length).toBeGreaterThan(0)

    for (const [modulePath, mod] of entries) {
      const locale = fileNameFromModulePath(modulePath).replace(/\.ts$/, '')
      const i18n = createI18n({
        legacy: false,
        locale,
        fallbackLocale: locale,
        missingWarn: false,
        fallbackWarn: false,
        messages: {
          [locale]: mod.default,
        },
      })

      for (const key of requiredKeys) {
        const value = getPathValue(mod.default, key)
        expect(typeof value, `${locale} should expose ${key}`).toBe('string')
        expect(String(value).trim().length, `${locale} should not leave ${key} empty`).toBeGreaterThan(
          0
        )
        expect(() => i18n.global.t(key), `${locale} should compile ${key}`).not.toThrow()
      }
    }
  })

  it('localizes key security page labels in non-English locales', () => {
    const entries = Object.entries(localeModules).sort(([a], [b]) => a.localeCompare(b))
    const enUSPath = entries.find(([modulePath]) => modulePath.endsWith('/en-US.ts'))?.[0]
    expect(enUSPath).toBeTruthy()
    if (!enUSPath) {
      throw new Error('Missing en-US locale module')
    }

    const referenceMessages = localeModules[enUSPath]?.default
    expect(referenceMessages).toBeTruthy()
    if (!referenceMessages) {
      throw new Error('Missing en-US locale messages')
    }

    const localizedKeys = [
      'security.title',
      'security.approvedBrowserSites',
      'security.approvedBrowserSitesDesc',
      'security.noApprovedBrowserSites',
      'security.tabs.overview',
      'security.tabs.network',
      'security.tabs.monitoring',
      'security.tabs.events',
      'security.tabs.logs',
      'security.statusSecure',
      'security.statusWarning',
      'security.statusFailed',
      'security.statusScanning',
      'security.scanSummary',
      'security.scanInProgress',
      'security.scan.details',
      'security.scan.fix',
      'security.scan.fixAll',
      'security.scan.manualFix',
      'security.scan.fixError',
      'security.scan.fixPreview',
      'security.scan.fixDescription',
      'security.scan.previewError',
      'security.scan.checkScanning',
      'security.scan.risk',
      'security.scan.impact',
      'security.scan.remediation',
      'security.scan.changes',
      'security.scan.reversible',
      'security.scan.irreversible',
      'security.scan.applyFix',
      'security.scan.categories.sandbox',
      'security.firewall.builtin.builtin_role_injection.description',
      'security.firewall.builtin.builtin_instruction_override.description',
      'security.firewall.builtin.builtin_delimiter_attacks.description',
      'security.firewall.builtin.builtin_encoding_attacks.description',
      'security.firewall.builtin.builtin_jailbreak_patterns.description',
      'security.firewall.builtin.builtin_data_exfiltration.description',
      'security.firewall.builtin.builtin_input_length_guard.description',
    ]

    const exemptLocales = new Set(['en-US', 'en-GB'])

    for (const [modulePath, mod] of entries) {
      const locale = fileNameFromModulePath(modulePath).replace(/\.ts$/, '')
      if (exemptLocales.has(locale)) continue

      for (const key of localizedKeys) {
        expect(getPathValue(mod.default, key), `${locale} should localize ${key}`).not.toBe(
          getPathValue(referenceMessages, key)
        )
      }
    }
  })

  it('only keeps intentional shared English security labels in non-English locales', () => {
    const entries = Object.entries(localeModules).sort(([a], [b]) => a.localeCompare(b))
    const enUSPath = entries.find(([modulePath]) => modulePath.endsWith('/en-US.ts'))?.[0]
    expect(enUSPath).toBeTruthy()
    if (!enUSPath) {
      throw new Error('Missing en-US locale module')
    }

    const referenceMessages = localeModules[enUSPath]?.default
    expect(referenceMessages).toBeTruthy()
    if (!referenceMessages) {
      throw new Error('Missing en-US locale messages')
    }

    const securityReferenceEntries = Object.fromEntries(
      collectLeafKeys(getPathValue(referenceMessages, 'security')).map((key) => [
        `security.${key}`,
        getPathValue(referenceMessages, `security.${key}`),
      ])
    )

    const alwaysAllowed = new Set(['security.tabs.harness'])
    const localeSpecificAllowed: Record<string, string[]> = {
      'cs-CZ': ['security.tabs.firewall'],
      'da-DK': ['security.directoryWhitelistAlias', 'security.scan.categories.system'],
      'de-DE': [
        'security.directoryWhitelistAlias',
        'security.tabs.firewall',
        'security.scan.categories.system',
      ],
      'el-GR': ['security.threats.types.sqlInjection'],
      'es-ES': ['security.directoryWhitelistAlias'],
      'fr-FR': ['security.directoryWhitelistAlias'],
      'hr-HR': ['security.directoryWhitelistAlias'],
      'hu-HU': ['security.threats.types.bruteForce'],
      'it-IT': ['security.directoryWhitelistAlias', 'security.tabs.firewall'],
      'nb-NO': ['security.directoryWhitelistAlias', 'security.scan.categories.system'],
      'nl-NL': ['security.directoryWhitelistAlias', 'security.tabs.firewall'],
      'pl-PL': ['security.directoryWhitelistAlias'],
      'pt-BR': ['security.directoryWhitelistAlias', 'security.tabs.firewall'],
      'pt-PT': ['security.directoryWhitelistAlias', 'security.tabs.firewall'],
      'ro-RO': ['security.directoryWhitelistAlias', 'security.tabs.firewall'],
      'sk-SK': ['security.directoryWhitelistAlias', 'security.tabs.firewall'],
      'sv-SE': ['security.directoryWhitelistAlias', 'security.scan.categories.system'],
    }

    const exemptLocales = new Set(['en-US', 'en-GB'])

    for (const [modulePath, mod] of entries) {
      const locale = fileNameFromModulePath(modulePath).replace(/\.ts$/, '')
      if (exemptLocales.has(locale)) continue

      const sameAsEnglish = Object.keys(securityReferenceEntries)
        .filter((key) => getPathValue(mod.default, key) === securityReferenceEntries[key])
        .sort()
      const allowed = new Set([...alwaysAllowed, ...(localeSpecificAllowed[locale] ?? [])])
      const unexpectedSameAsEnglish = sameAsEnglish.filter((key) => !allowed.has(key))

      expect(
        unexpectedSameAsEnglish,
        `${locale} should only keep intentional shared English security labels`
      ).toEqual([])
    }
  })
})
