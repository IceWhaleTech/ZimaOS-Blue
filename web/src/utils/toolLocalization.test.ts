import { describe, expect, it } from 'vitest'

import builtinToolBackfills from '@/i18n/builtin-tool-backfills'

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

function isPlainObject(value: unknown): value is LocaleMessages {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value)
}

function deepMergeMessages(base: LocaleMessages, override: LocaleMessages): LocaleMessages {
  const merged: LocaleMessages = { ...base }

  for (const [key, overrideValue] of Object.entries(override)) {
    const baseValue = merged[key]
    merged[key] =
      isPlainObject(baseValue) && isPlainObject(overrideValue)
        ? deepMergeMessages(baseValue, overrideValue)
        : overrideValue
  }

  return merged
}

function getMergedLocaleMessages(locale: string): LocaleMessages {
  const base = getLocaleMessages(locale)
  const builtinToolOverrides =
    (builtinToolBackfills as Record<string, LocaleMessages>)[locale] || {}
  return deepMergeMessages(base, builtinToolOverrides)
}

const nameCoverage = [
  'advisor',
  'computer_use',
  'cron',
  'docx',
  'web',
  'sessions',
  'file_read',
  'file_write',
  'pdf',
  'pptx',
  'read',
  'write',
  'web_search',
  'ui_reviewer',
  'analyze',
  'ask',
  'mediagen',
  'research',
  'deep_research',
  'research_run',
  'research_status',
  'xlsx',
] as const

const descriptionCoverage = [
  {
    name: 'computer_use',
    description:
      'Control supported host OS windows and accessibility-backed browser flows for snapshots, targeting, scrolling, input, screenshots, and chat-style actions.',
  },
  {
    name: 'advisor',
    description:
      'Decision advisor for selection, replacement, migration, and best-practice questions.',
  },
  {
    name: 'docx',
    description:
      'Use when the task centers on a workspace .docx file and needs a native Word-style document for writing, template filling, placeholder edits, or validation.',
  },
  {
    name: 'pdf',
    description:
      'Use when the task centers on a workspace .pdf file and needs PDF-native reading, form filling, printable output, or layout-preserving reformatting.',
  },
  {
    name: 'pptx',
    description:
      'Use when the task centers on a workspace .pptx file and needs a native slide deck for editable slides, layout changes, or chart updates.',
  },
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
    name: 'research',
    description:
      'Run a Deep Research workflow. You can wait for the final report or get a job ID to check later.',
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
  {
    name: 'xlsx',
    description:
      'Use when the task centers on a workspace .xlsx file and needs a native spreadsheet for tables, formulas, sheet edits, analysis, or validation.',
  },
] as const

const reportedBuiltinToolCoverage = [
  {
    name: 'file_delete',
    englishLabel: 'File Delete',
    englishDescription:
      'Deletes a local file or directory. For directories, set recursive=true to remove non-empty contents.',
  },
  {
    name: 'config',
    englishLabel: 'Config',
    englishDescription:
      'Runtime configuration and admin tool. Use {domain}.{action} format. Domains: providers, settings, channels, skills, tools, system, proxy, users, apikeys, upgrade. Call with action="providers.list" first to explore available operations. Common: providers.list, settings.get, system.health, tools.list, users.list, upgrade.status.',
  },
  {
    name: 'convert',
    englishLabel: 'Convert',
    englishDescription:
      'Convert local files, attachments, and prior outputs. Preferred form: input_path + output_path using relative paths. Normal single-file jobs return synchronously; only heavier jobs return async=true with a task_id for polling.',
  },
] as const

const runtimeVisibleToolCoverage = [
  'advisor',
  'agents_list',
  'analyze',
  'bash',
  'browser',
  'canvas',
  'cron',
  'deep_research',
  'docx',
  'edit',
  'exec',
  'find',
  'gateway',
  'grep',
  'image',
  'ls',
  'mcp',
  'memory',
  'memory_forget',
  'memory_get',
  'memory_search',
  'memory_write',
  'message',
  'nodes',
  'office',
  'pdf',
  'ppt',
  'pptx',
  'research',
  'read',
  'session_status',
  'sessions',
  'sessions_history',
  'sessions_list',
  'sessions_send',
  'sessions_spawn',
  'subagents',
  'tool_search',
  'tts',
  'web_query',
  'write',
  'xlsx',
] as const

const preferredToolNameMap: Record<string, string> = {
  read: 'file_read',
  write: 'file_write',
  image_generation: 'image',
  generate_image: 'image',
  generateImage: 'image',
  memory_search: 'memory',
  memory_get: 'memory',
  memory_read: 'memory',
  memory_write: 'memory',
  memory_remember: 'memory',
  memory_store: 'memory',
  memory_forget: 'memory',
  memory_delete: 'memory',
  sessions_list: 'sessions',
  sessions_history: 'sessions',
  session_status: 'sessions',
  sessions_spawn: 'sessions',
  sessions_send: 'sessions',
  research: 'research_run',
  deep_research: 'research_run',
  web_query: 'web',
  web_search: 'web',
  web_fetch: 'web',
  web_read: 'web',
  web_extract: 'web',
  web_crawl: 'web',
}

const toolNameAliases: Record<string, string[]> = {
  cron: ['scheduler'],
  image: ['image_generation'],
  message: ['reminder'],
  ppt: ['mediagen'],
  web: ['web_search'],
}

const toolDescriptionKeyMap: Record<string, string[]> = {
  analyze: ['tools.descriptions.analyze'],
  ask: ['tools.descriptions.ask'],
  auto_reply: ['skills.builtin.autoreply.description'],
  autoreply: ['skills.builtin.autoreply.description'],
  browser: ['skills.builtin.browser.description'],
  calculator: ['skills.builtin.calculator.description'],
  calendar: ['skills.builtin.calendar.description'],
  contacts: ['skills.builtin.contacts.description'],
  cron: ['skills.builtin.scheduler.description'],
  crypto: ['skills.builtin.crypto.description'],
  datetime: ['skills.builtin.datetime.description'],
  deep_research: ['tools.descriptions.deep_research', 'tools.descriptions.research_run'],
  discord: ['skills.builtin.discord-skill.description'],
  docker: ['skills.builtin.docker.description'],
  email: ['skills.builtin.email.description'],
  exec: ['tools.descriptions.exec'],
  files: ['skills.builtin.files.description'],
  github: ['skills.builtin.github.description'],
  mediagen: ['tools.descriptions.mediagen'],
  message: ['skills.builtin.reminder.description'],
  news: ['skills.builtin.news.description'],
  network: ['skills.builtin.network.description'],
  notion: ['skills.builtin.notion.description'],
  notes: ['skills.builtin.notes.description'],
  notifications: ['skills.builtin.notifications.description'],
  ppt: ['tools.descriptions.mediagen'],
  process: ['skills.builtin.processes.description'],
  file_read: ['tools.descriptions.file_read', 'tools.descriptions.read'],
  research: ['tools.descriptions.research_run'],
  reminder: ['skills.builtin.reminder.description'],
  reminders: ['skills.builtin.reminder.description'],
  sandbox: ['skills.builtin.sandbox.description'],
  search: ['skills.builtin.search.description'],
  slack: ['skills.builtin.slack-skill.description'],
  stocks: ['skills.builtin.stocks.description'],
  system_info: ['skills.builtin.system-info.description'],
  tasks: ['skills.builtin.tasks.description'],
  timer: ['skills.builtin.timer.description'],
  translate: ['skills.builtin.translate.description'],
  ui_reviewer: ['skills.builtin.ui-reviewer.description'],
  unit_converter: ['skills.builtin.unit-converter.description'],
  weather: ['skills.builtin.weather.description'],
  web: ['tools.descriptions.web', 'tools.descriptions.web_search'],
  web_crawl: ['tools.descriptions.web_crawl'],
  web_extract: ['tools.descriptions.web_extract'],
  web_fetch: ['tools.descriptions.web_fetch'],
  web_read: ['tools.descriptions.web_read'],
  web_search: ['tools.descriptions.web_search'],
  workflows: ['skills.builtin.workflows.description'],
  file_write: ['tools.descriptions.file_write', 'tools.descriptions.write'],
}

function toLegacyToolLabel(toolName: string): string {
  switch (toolName) {
    case 'system_info':
      return 'System Info'
    case 'ui_reviewer':
      return 'UI Reviewer'
    case 'auto_reply':
      return 'Auto Reply'
    case 'web_search':
      return 'Web Search'
    case 'web_query':
      return 'Web Query'
    case 'web_fetch':
      return 'Web Fetch'
    case 'web_read':
      return 'Web Read'
    case 'web_extract':
      return 'Web Extract'
    case 'web_crawl':
      return 'Web Crawl'
    case 'file_read':
      return 'File Read'
    case 'file_write':
      return 'File Write'
    case 'current_time':
      return 'Current Time'
    default:
      return toolName
        .split('_')
        .filter(Boolean)
        .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
        .join(' ')
  }
}

function nameResourceKeysFor(toolName: string): string[] {
  const preferredToolName = preferredToolNameMap[toolName] || toolName
  const aliases = toolNameAliases[preferredToolName] || []
  return [
    `tools.names.${preferredToolName}`,
    ...aliases.map((alias) => `tools.names.${alias}`),
    ...[preferredToolName, ...aliases].map((name) => `tools.names.${toLegacyToolLabel(name)}`),
  ]
}

function descriptionResourceKeysFor(toolName: string): string[] {
  const preferredToolName = preferredToolNameMap[toolName] || toolName
  return [
    ...(toolDescriptionKeyMap[preferredToolName] || []),
    `tools.descriptions.${preferredToolName}`,
  ]
}

describe('tool page localization coverage', () => {
  it('loads the full 27-locale set', () => {
    expect(localeCodes.length).toBe(27)
  })

  it('resolves localized built-in tool names across all locales', () => {
    for (const locale of localeCodes) {
      const messages = getMergedLocaleMessages(locale)
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
      const messages = getMergedLocaleMessages(locale)
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
    const messages = getMergedLocaleMessages('en-US')
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
    expect(getLocalizedToolName('computer_use', t, te)).toBe('Computer Use')
    expect(getLocalizedToolName('research', t, te)).toBe('Deep Research')
    expect(getLocalizedToolName('deep_research', t, te)).toBe('Deep Research')
    expect(getLocalizedToolDescription('computer_use', 'fallback', t, te)).toBe(
      'Control supported host OS windows and accessibility-backed browser flows for snapshots, targeting, scrolling, input, screenshots, and chat-style actions.'
    )
    expect(getLocalizedToolDescription('web_query', 'fallback', t, te)).toBe(
      'Unified web tool for searching, reading, extracting, or crawling web content'
    )
    expect(getLocalizedToolDescription('research', 'fallback', t, te)).toBe(
      'Run a Deep Research workflow. You can wait for the final report or get a job ID to check later.'
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

  it('keeps the reported file_delete, config, and convert tools localized across all 27 locales', () => {
    for (const locale of localeCodes) {
      const messages = getMergedLocaleMessages(locale)
      const t = (key: string) => String(getByPath(messages, key) ?? '')
      const te = (key: string) => {
        const value = getByPath(messages, key)
        return typeof value === 'string' && value.trim().length > 0
      }

      for (const tool of reportedBuiltinToolCoverage) {
        const localizedName = getLocalizedToolName(tool.name, t, te)
        const localizedDescription = getLocalizedToolDescription(
          tool.name,
          tool.englishDescription,
          t,
          te
        )

        expect(
          localizedName.trim().length,
          `${locale} should localize name for ${tool.name}`
        ).toBeGreaterThan(0)
        expect(
          localizedDescription.trim().length,
          `${locale} should localize description for ${tool.name}`
        ).toBeGreaterThan(0)

        if (locale.startsWith('en')) {
          expect(localizedName, `${locale} English label for ${tool.name}`).toBe(tool.englishLabel)
          expect(localizedDescription, `${locale} English description for ${tool.name}`).toBe(
            tool.englishDescription
          )
          continue
        }

        expect(
          localizedName,
          `${locale} should not fall back to English label for ${tool.name}`
        ).not.toBe(tool.englishLabel)
        expect(
          localizedDescription,
          `${locale} should not fall back to English description for ${tool.name}`
        ).not.toBe(tool.englishDescription)
      }
    }
  })

  it('keeps runtime-visible advisor and tool resources available across all locales', () => {
    expect(runtimeVisibleToolCoverage).toHaveLength(42)
    const missingResources: string[] = []

    for (const locale of localeCodes) {
      const messages = getMergedLocaleMessages(locale)

      for (const toolName of runtimeVisibleToolCoverage) {
        const nameKeys = nameResourceKeysFor(toolName)
        const descriptionKeys = descriptionResourceKeysFor(toolName)
        const hasName = nameKeys.some((key) => {
          const value = getByPath(messages, key)
          return typeof value === 'string' && value.trim().length > 0
        })
        const hasDescription = descriptionKeys.some((key) => {
          const value = getByPath(messages, key)
          return typeof value === 'string' && value.trim().length > 0
        })

        if (!hasName) {
          missingResources.push(
            `${locale} missing name resource for ${toolName} via ${nameKeys.join(', ')}`
          )
        }
        if (!hasDescription) {
          missingResources.push(
            `${locale} missing description resource for ${toolName} via ${descriptionKeys.join(', ')}`
          )
        }
      }
    }

    expect(missingResources, missingResources.join('\n')).toEqual([])
  })
})
