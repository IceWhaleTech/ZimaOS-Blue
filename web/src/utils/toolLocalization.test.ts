import { describe, expect, it } from 'vitest'

import { getLocalizedToolDescription, getLocalizedToolName } from './toolLocalization'

type LocaleMessages = Record<string, unknown>

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

function getLocaleMessages(locale: string): LocaleMessages {
  const localeMessages = localeMessagesByCode.get(locale)
  if (!localeMessages) {
    throw new Error(`Missing locale: ${locale}`)
  }
  return localeMessages
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
  'deep_research',
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
    name: 'deep_research',
    description:
      'Run a Deep Research workflow. You can wait for the final report or get a job ID to check later.',
  },
  {
    name: 'research_run',
    description:
      'Run a research workflow. You can wait for the final report or get a job ID to check later.',
  },
  {
    name: 'research_status',
    description: 'Get the current status or final report for a research job.',
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
      const messages = getLocaleMessages(locale)
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
      const messages = getLocaleMessages(locale)
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
    const messages = getLocaleMessages('en-US')
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
    expect(getLocalizedToolName('deep_research', t, te)).toBe('Deep Research')
    expect(getLocalizedToolDescription('web_query', 'fallback', t, te)).toBe(
      'Unified web tool for searching, reading, extracting, or crawling web content'
    )
    expect(getLocalizedToolDescription('deep_research', 'fallback', t, te)).toBe(
      'Run a Deep Research workflow. You can wait for the final report or get a job ID to check later.'
    )
    expect(getLocalizedToolDescription('web_fetch', 'fallback', t, te)).toBe(
      'Unified web tool for searching, reading, extracting, or crawling web content'
    )
    expect(getLocalizedToolDescription('file_read', 'fallback', t, te)).toBe(
      'Read a local file and extract supported document content'
    )
  })
})
