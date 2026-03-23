import { describe, expect, it } from 'vitest'

import { deepMergeMessages, type LocaleMessages } from '../i18n/merge'
import extensionsBrowseOverrides from '../i18n/extensions-browse-overrides'
import prioritySettingsOverrides from '../i18n/priority-settings-overrides'
import researchToolOverrides from '../i18n/research-tool-overrides'
import skillToolOverrides from '../i18n/skill-tool-overrides'
import { getLocalizedToolDescription, getLocalizedToolName } from './toolLocalization'

function fileNameFromModulePath(modulePath: string): string {
  return modulePath.split('/').pop() ?? modulePath
}

function localeCodeFromFile(fileName: string): string {
  return fileName.replace(/\.ts$/, '')
}

function getByPath(source: LocaleMessages, path: string): unknown {
  return path.split('.').reduce<unknown>((value, part) => {
    if (!value || typeof value !== 'object' || Array.isArray(value)) {
      return undefined
    }
    return (value as Record<string, unknown>)[part]
  }, source)
}

const localeModules = import.meta.glob<{ default: LocaleMessages }>('../i18n/locales/*.ts', {
  eager: true,
})

const localeMessagesByCode = new Map(
  Object.entries(localeModules).map(([modulePath, mod]) => [
    localeCodeFromFile(fileNameFromModulePath(modulePath)),
    mod.default,
  ])
)

const localeCodes = [...localeMessagesByCode.keys()].sort()
const baseLocale = localeMessagesByCode.get('en-US') as LocaleMessages

function buildMergedLocaleMessages(locale: string): LocaleMessages {
  const localeMessages = localeMessagesByCode.get(locale)
  if (!localeMessages) {
    throw new Error(`Missing locale: ${locale}`)
  }

  const mergedBase = locale === 'en-US' ? baseLocale : deepMergeMessages(baseLocale, localeMessages)
  const withSettingsOverrides = deepMergeMessages(
    mergedBase,
    (prioritySettingsOverrides as Record<string, LocaleMessages>)[locale] || {}
  )
  const withSkillToolOverrides = deepMergeMessages(
    withSettingsOverrides,
    (skillToolOverrides as Record<string, LocaleMessages>)[locale] || {}
  )
  const withBrowseOverrides = deepMergeMessages(
    withSkillToolOverrides,
    (extensionsBrowseOverrides as Record<string, LocaleMessages>)[locale] || {}
  )

  return deepMergeMessages(
    withBrowseOverrides,
    (researchToolOverrides as Record<string, LocaleMessages>)[locale] || {}
  )
}

const nameCoverage = [
  'cron',
  'web',
  'sessions',
  'file_read',
  'file_write',
  'read',
  'write',
  'web_search',
  'ui_reviewer',
  'analyze',
  'ask',
  'mediagen',
  'research_run',
  'research_status',
] as const

const descriptionCoverage = [
  {
    name: 'web',
    description: 'Unified web tool for searching, reading, extracting, or crawling web content',
  },
  {
    name: 'sessions',
    description: 'Unified sessions tool for listing, inspecting, creating, and sending messages',
  },
  {
    name: 'cron',
    description: 'Create, list, delete, and trigger scheduled tasks (cron jobs)',
  },
  {
    name: 'file_read',
    description: 'Read a local file and extract supported document content',
  },
  {
    name: 'file_write',
    description: 'Write text content to a local file',
  },
  {
    name: 'read',
    description: 'Read a local file and extract supported document content',
  },
  {
    name: 'write',
    description: 'Write text content to a local file',
  },
  {
    name: 'web_search',
    description: 'Keyword web search. Returns result listings without opening pages.',
  },
  {
    name: 'exec',
    description: 'Execute shell commands and capture output.',
  },
  {
    name: 'analyze',
    description: 'Deep-dive analysis tool.',
  },
  {
    name: 'ask',
    description: 'Ask the user follow-up questions.',
  },
  {
    name: 'mediagen',
    description: 'Generate images and videos using AI models.',
  },
  {
    name: 'research_run',
    description:
      'Run deep research. You can wait for the final report or get a job ID to check later.',
  },
  {
    name: 'research_status',
    description: 'Get the current status or final report for a deep research job.',
  },
  {
    name: 'docker',
    description: 'Docker container management',
  },
  {
    name: 'github',
    description: 'GitHub repository operations',
  },
  {
    name: 'notion',
    description: 'Notion workspace integration',
  },
  {
    name: 'slack',
    description: 'Slack workspace operations',
  },
  {
    name: 'discord',
    description: 'Discord server operations',
  },
  {
    name: 'browser',
    description: 'Browse the web, read pages, and interact with elements',
  },
  {
    name: 'sandbox',
    description: 'Execute commands in a sandboxed environment with resource limits',
  },
  {
    name: 'workflows',
    description: 'Create and execute n8n-style workflow automations with triggers and actions',
  },
] as const

describe('tool page localization coverage', () => {
  it('loads the full 27-locale set', () => {
    expect(localeCodes.length).toBe(27)
  })

  it('resolves localized built-in tool names across all locales', () => {
    for (const locale of localeCodes) {
      const messages = buildMergedLocaleMessages(locale)
      const t = (key: string) => String(getByPath(messages, key) ?? '')
      const te = (key: string) => {
        const value = getByPath(messages, key)
        return typeof value === 'string' && value.trim().length > 0
      }

      for (const toolName of nameCoverage) {
        const localized = getLocalizedToolName(toolName, t, te)
        expect(localized.trim().length, `${locale} should localize ${toolName}`).toBeGreaterThan(0)
        expect(localized, `${locale} should not fall back to raw id ${toolName}`).not.toBe(toolName)
      }
    }
  })

  it('resolves localized built-in tool descriptions across all locales', () => {
    for (const locale of localeCodes) {
      const messages = buildMergedLocaleMessages(locale)
      const t = (key: string) => String(getByPath(messages, key) ?? '')
      const te = (key: string) => {
        const value = getByPath(messages, key)
        return typeof value === 'string' && value.trim().length > 0
      }

      for (const tool of descriptionCoverage) {
        const localized = getLocalizedToolDescription(tool.name, tool.description, t, te)
        expect(
          localized.trim().length,
          `${locale} should provide a description for ${tool.name}`
        ).toBeGreaterThan(0)
        if (!locale.startsWith('en')) {
          expect(
            localized,
            `${locale} should not fall back to the English description for ${tool.name}`
          ).not.toBe(tool.description)
        }
      }
    }
  })

  it('prefers unified labels and descriptions for legacy alias tools', () => {
    const messages = buildMergedLocaleMessages('en-US')
    const t = (key: string) => String(getByPath(messages, key) ?? '')
    const te = (key: string) => {
      const value = getByPath(messages, key)
      return typeof value === 'string' && value.trim().length > 0
    }

    expect(getLocalizedToolName('web_query', t, te)).toBe('Web')
    expect(getLocalizedToolName('web_search', t, te)).toBe('Web')
    expect(getLocalizedToolName('file_read', t, te)).toBe('File Read')
    expect(getLocalizedToolName('read', t, te)).toBe('File Read')
    expect(getLocalizedToolName('image_generation', t, te)).toBe('Image')
    expect(getLocalizedToolName('generate_image', t, te)).toBe('Image')
    expect(getLocalizedToolName('generateImage', t, te)).toBe('Image')
    expect(getLocalizedToolName('sessions_list', t, te)).toBe('Sessions')
    expect(getLocalizedToolDescription('web_query', 'fallback', t, te)).toBe(
      'Unified web tool for searching, reading, extracting, or crawling web content'
    )
    expect(getLocalizedToolDescription('web_fetch', 'fallback', t, te)).toBe(
      'Unified web tool for searching, reading, extracting, or crawling web content'
    )
    expect(getLocalizedToolDescription('file_read', 'fallback', t, te)).toBe(
      'Read a local file and extract supported document content'
    )
  })
})
