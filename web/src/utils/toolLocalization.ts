type Translate = (key: string) => string
type HasTranslation = (key: string) => boolean

const PREFERRED_TOOL_NAME_MAP: Record<string, string> = {
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

const TOOL_NAME_ALIASES: Record<string, string[]> = {
  cron: ['scheduler'],
  image: ['image_generation'],
  message: ['reminder'],
  ppt: ['mediagen'],
  web: ['web_search'],
}

const TOOL_DESCRIPTION_KEY_MAP: Record<string, string[]> = {
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

function humanizeToolName(toolName: string): string {
  if (toolName === 'tts') return 'TTS'
  return toLegacyToolLabel(toolName)
}

function resolveFirstTranslated(keys: string[], t: Translate, te: HasTranslation): string | null {
  for (const key of keys) {
    if (te(key)) return t(key)
  }
  return null
}

export function getLocalizedToolName(toolName: string, t: Translate, te: HasTranslation): string {
  const preferredToolName = PREFERRED_TOOL_NAME_MAP[toolName] || toolName
  const directKey = `tools.names.${preferredToolName}`
  if (te(directKey)) return t(directKey)

  const aliases = TOOL_NAME_ALIASES[preferredToolName] || []
  const aliasKeys = aliases.map((alias) => `tools.names.${alias}`)
  const aliasValue = resolveFirstTranslated(aliasKeys, t, te)
  if (aliasValue) return aliasValue

  const legacyCandidates = [preferredToolName, ...aliases].map(
    (name) => `tools.names.${toLegacyToolLabel(name)}`
  )
  const legacyValue = resolveFirstTranslated(legacyCandidates, t, te)
  if (legacyValue) return legacyValue

  return humanizeToolName(preferredToolName)
}

export function getLocalizedToolDescription(
  toolName: string,
  fallbackDescription: string | undefined,
  t: Translate,
  te: HasTranslation
): string {
  const preferredToolName = PREFERRED_TOOL_NAME_MAP[toolName] || toolName
  const mappedValue = resolveFirstTranslated(
    TOOL_DESCRIPTION_KEY_MAP[preferredToolName] || [],
    t,
    te
  )
  if (mappedValue) return mappedValue

  const directKey = `tools.descriptions.${preferredToolName}`
  if (te(directKey)) return t(directKey)

  return fallbackDescription || ''
}
