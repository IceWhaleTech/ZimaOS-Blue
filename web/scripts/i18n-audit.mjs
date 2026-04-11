#!/usr/bin/env node

import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'
import ts from 'typescript'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const WEB_ROOT = path.resolve(__dirname, '..')
const SRC_DIR = path.join(WEB_ROOT, 'src')
const LOCALES_DIR = path.join(SRC_DIR, 'i18n', 'locales')
const HARNESS_LOCALE_ADDITIONS_MODULE_PATH = path.join(
  SRC_DIR,
  'i18n',
  'harness-locale-additions.ts',
)
const BUILTIN_SKILL_BACKFILL_MODULE_PATH = path.join(
  SRC_DIR,
  'i18n',
  'builtin-skill-backfills.ts',
)
const BUILTIN_TOOL_BACKFILL_MODULE_PATH = path.join(
  SRC_DIR,
  'i18n',
  'builtin-tool-backfills.ts',
)
const LOCALE_POST_MERGE_BACKFILL_MODULE_PATH = path.join(
  SRC_DIR,
  'i18n',
  'locale-post-merge-backfills.ts',
)
const TS_MODULE_CACHE = new Map()
const TS_MODULE_LOADING = new Set()
let builtinSkillBackfillBuilder = null
let builtinToolBackfills = null
let localePostMergeBackfillBuilder = null
let harnessLocaleMerger = null

export const EXIT_CODES = Object.freeze({
  ok: 0,
  violations: 1,
})

function collectFiles(dir, extensions) {
  const out = []
  const stack = [dir]
  while (stack.length > 0) {
    const current = stack.pop()
    if (!current) continue
    const entries = fs.readdirSync(current, { withFileTypes: true })
    for (const entry of entries) {
      const full = path.join(current, entry.name)
      if (entry.isDirectory()) {
        stack.push(full)
        continue
      }
      if (extensions.some((ext) => entry.name.endsWith(ext))) {
        out.push(full)
      }
    }
  }
  return out.sort()
}

function getLineNumber(text, index) {
  return text.slice(0, index).split('\n').length
}

function escapeRegExp(str) {
  return str.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

function templateToPattern(template) {
  const parts = template.split(/\$\{[^}]+\}/g)
  const prefix = parts[0] || ''
  const suffix = parts.length > 1 ? parts[parts.length - 1] || '' : ''
  const normalized = `${prefix}*${suffix}`
  const regex = new RegExp(`^${escapeRegExp(prefix)}.*${escapeRegExp(suffix)}$`)
  return { prefix, suffix, normalized, regex }
}

function parseFirstArgument(source, startIndex) {
  let i = startIndex
  let depth = 0
  let quote = null
  let escaped = false
  let expressionStart = startIndex

  while (i < source.length) {
    const ch = source[i]

    if (quote) {
      if (escaped) {
        escaped = false
        i += 1
        continue
      }
      if (ch === '\\') {
        escaped = true
        i += 1
        continue
      }
      if (ch === quote) {
        quote = null
      }
      i += 1
      continue
    }

    if (ch === "'" || ch === '"' || ch === '`') {
      quote = ch
      i += 1
      continue
    }
    if (ch === '(' || ch === '[' || ch === '{') {
      depth += 1
      i += 1
      continue
    }
    if (ch === ')' || ch === ']' || ch === '}') {
      if (depth > 0) {
        depth -= 1
        i += 1
        continue
      }
      if (ch === ')') {
        const arg = source.slice(expressionStart, i).trim()
        return { arg, delimiter: ')', endIndex: i }
      }
    }
    if (depth === 0 && ch === ',') {
      const arg = source.slice(expressionStart, i).trim()
      return { arg, delimiter: ',', endIndex: i }
    }
    if (depth === 0 && ch === ')') {
      const arg = source.slice(expressionStart, i).trim()
      return { arg, delimiter: ')', endIndex: i }
    }
    i += 1
  }

  return { arg: source.slice(expressionStart).trim(), delimiter: null, endIndex: source.length }
}

function unquote(value) {
  const trimmed = value.trim()
  if (trimmed.length < 2) return null
  const head = trimmed[0]
  const tail = trimmed[trimmed.length - 1]
  if ((head === "'" && tail === "'") || (head === '"' && tail === '"') || (head === '`' && tail === '`')) {
    return { quote: head, inner: trimmed.slice(1, -1) }
  }
  return null
}

function extractConcatLiterals(expression) {
  const literals = []
  const matcher = /(['"`])((?:(?!\1)[\s\S])*)\1/g
  let match
  while ((match = matcher.exec(expression)) !== null) {
    literals.push(match[2])
  }
  return literals
}

function isLikelyI18nKey(value) {
  return /^[A-Za-z0-9][A-Za-z0-9_.-]*$/.test(value) && value.includes('.')
}

function stripQuotedStrings(expression) {
  return expression.replace(/(['"`])(?:\\.|(?!\1)[\s\S])*\1/g, '""')
}

function extractConditionalLiteralKeys(expression) {
  const trimmed = expression.trim()
  if (!/[?:|]/.test(trimmed)) return []

  const stripped = stripQuotedStrings(trimmed)
  if (/\b(?:\$?t|\$?te)\s*\(/.test(stripped)) return []

  const literals = extractConcatLiterals(trimmed)
  const keyLiterals = literals.filter(isLikelyI18nKey)
  if (keyLiterals.length < 2) return []

  return [...new Set(keyLiterals)]
}

function isFunctionDeclarationCall(content, callIndex) {
  const prefix = content.slice(Math.max(0, callIndex - 40), callIndex)
  return /\bfunction\s*$/.test(prefix)
}

function isGuardedByTe(content, callIndex, arg) {
  const scope = content.slice(Math.max(0, callIndex - 220), callIndex)
  const pattern = new RegExp(`\\b(?:\\$?te)\\s*\\(\\s*${escapeRegExp(arg.trim())}\\s*\\)`)
  return pattern.test(scope)
}

function extractKeyUsage(filePath, content) {
  const staticKeys = []
  const dynamicPatterns = []
  const dynamicExpressions = []
  const dynamicExistenceExpressions = []
  const dynamicGuardedExpressions = []
  const callMatcher = /\b(\$?t|\$?te)\s*\(/g

  let match
  while ((match = callMatcher.exec(content)) !== null) {
    if (content[match.index - 1] === '\\') continue
    if (isFunctionDeclarationCall(content, match.index)) continue

    const callee = match[1]
    const existenceOnly = callee.endsWith('te')
    const line = getLineNumber(content, match.index)
    const argStart = match.index + match[0].length
    const { arg } = parseFirstArgument(content, argStart)
    if (!arg) continue

    const literal = unquote(arg)
    if (literal) {
      if (literal.quote === '`' && literal.inner.includes('${')) {
        const pattern = templateToPattern(literal.inner)
        dynamicPatterns.push({
          file: filePath,
          line,
          template: literal.inner,
          ...pattern,
          source: 'direct-template',
        })
        continue
      }
      const key = literal.inner.trim()
      if (key) {
        staticKeys.push({ key, file: filePath, line })
      }
      continue
    }

    const conditionalKeys = extractConditionalLiteralKeys(arg)
    if (conditionalKeys.length > 0) {
      for (const key of conditionalKeys) {
        staticKeys.push({ key, file: filePath, line })
      }
      continue
    }

    if (arg.includes('+')) {
      const literals = extractConcatLiterals(arg)
      if (literals.length > 0) {
        const prefix = literals[0] || ''
        const suffix = literals.length > 1 ? literals[literals.length - 1] || '' : ''
        if (!prefix && !suffix) {
          if (existenceOnly) {
            dynamicExistenceExpressions.push({ file: filePath, line, expr: arg })
          } else if (isGuardedByTe(content, match.index, arg)) {
            dynamicGuardedExpressions.push({ file: filePath, line, expr: arg })
          } else {
            dynamicExpressions.push({ file: filePath, line, expr: arg })
          }
          continue
        }
        const pattern = templateToPattern(`${prefix}${prefix || suffix ? '${expr}' : ''}${suffix}`)
        dynamicPatterns.push({
          file: filePath,
          line,
          template: `${prefix}...${suffix}`,
          ...pattern,
          source: 'concat-expression',
        })
        continue
      }
    }

    if (existenceOnly) {
      dynamicExistenceExpressions.push({ file: filePath, line, expr: arg })
    } else if (isGuardedByTe(content, match.index, arg)) {
      dynamicGuardedExpressions.push({ file: filePath, line, expr: arg })
    } else {
      dynamicExpressions.push({ file: filePath, line, expr: arg })
    }
  }

  const variableTemplates = new Map()
  const varTemplateMatcher = /\b(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*`([^`]*\$\{[^`]+\}[^`]*)`/g
  while ((match = varTemplateMatcher.exec(content)) !== null) {
    const varName = match[1]
    const template = match[2]
    const line = getLineNumber(content, match.index)
    const list = variableTemplates.get(varName) || []
    list.push({ varName, template, line })
    variableTemplates.set(varName, list)
  }

  for (const [varName, templates] of variableTemplates.entries()) {
    const useMatcher = new RegExp(`\\b(?:\\$?t|\\$?te)\\s*\\(\\s*${escapeRegExp(varName)}\\s*(?:[,\\)])`, 'g')
    if (!useMatcher.test(content)) {
      continue
    }
    for (const item of templates) {
      const pattern = templateToPattern(item.template)
      dynamicPatterns.push({
        file: filePath,
        line: item.line,
        template: item.template,
        ...pattern,
        source: `var-template:${varName}`,
      })
    }
  }

  return { staticKeys, dynamicPatterns, dynamicExpressions, dynamicExistenceExpressions, dynamicGuardedExpressions }
}

function sanitizeLocaleCode(fileName) {
  return fileName.replace(/\.ts$/, '')
}

function resolveTsModulePath(parentFilePath, specifier) {
  if (!specifier.startsWith('.')) {
    throw new Error(`Unsupported module specifier in i18n audit: ${specifier}`)
  }

  const resolved = path.resolve(path.dirname(parentFilePath), specifier)
  if (fs.existsSync(resolved) && fs.statSync(resolved).isFile()) {
    return resolved
  }

  const withTs = `${resolved}.ts`
  if (fs.existsSync(withTs)) {
    return withTs
  }

  const indexTs = path.join(resolved, 'index.ts')
  if (fs.existsSync(indexTs)) {
    return indexTs
  }

  throw new Error(`Cannot resolve TS module from ${parentFilePath}: ${specifier}`)
}

function executeTsModule(
  filePath,
  cache = TS_MODULE_CACHE,
  loading = TS_MODULE_LOADING,
) {
  if (cache.has(filePath)) {
    return cache.get(filePath)
  }
  if (loading.has(filePath)) {
    throw new Error(`Circular TS module import detected: ${filePath}`)
  }
  if (!fs.existsSync(filePath)) {
    throw new Error(`TS module file not found: ${filePath}`)
  }

  loading.add(filePath)

  try {
    const raw = fs.readFileSync(filePath, 'utf8')
    const transpiled = ts.transpileModule(raw, {
      compilerOptions: {
        module: ts.ModuleKind.CommonJS,
        target: ts.ScriptTarget.ES2020,
      },
      fileName: filePath,
    }).outputText

    const module = { exports: {} }
    const localRequire = (specifier) =>
      executeTsModule(resolveTsModulePath(filePath, specifier), cache, loading)

    new Function('require', 'module', 'exports', transpiled)(
      localRequire,
      module,
      module.exports,
    )

    cache.set(filePath, module.exports)
    return module.exports
  } finally {
    loading.delete(filePath)
  }
}

function loadLocaleObjectByCode(localeCode, cache = new Map()) {
  if (cache.has(localeCode)) {
    return cache.get(localeCode)
  }
  const filePath = path.join(LOCALES_DIR, `${localeCode}.ts`)
  if (!fs.existsSync(filePath)) {
    throw new Error(`Locale file not found: ${filePath}`)
  }

  const localeModule = executeTsModule(filePath)
  const localeValue = localeModule?.default ?? localeModule
  cache.set(localeCode, localeValue)
  return localeValue
}

function getLocalePostMergeBackfillBuilder() {
  if (localePostMergeBackfillBuilder) {
    return localePostMergeBackfillBuilder
  }

  const localeBackfillModule = executeTsModule(LOCALE_POST_MERGE_BACKFILL_MODULE_PATH)
  const builder =
    localeBackfillModule?.buildLocalePostMergeBackfill ??
    localeBackfillModule?.default?.buildLocalePostMergeBackfill

  if (typeof builder !== 'function') {
    throw new Error('Failed to load buildLocalePostMergeBackfill for i18n audit')
  }

  localePostMergeBackfillBuilder = builder
  return localePostMergeBackfillBuilder
}

function getBuiltinSkillBackfillBuilder() {
  if (builtinSkillBackfillBuilder) {
    return builtinSkillBackfillBuilder
  }

  const builtinSkillModule = executeTsModule(BUILTIN_SKILL_BACKFILL_MODULE_PATH)
  const builder = builtinSkillModule?.default ?? builtinSkillModule?.buildBuiltinSkillBackfill

  if (typeof builder !== 'function') {
    throw new Error('Failed to load buildBuiltinSkillBackfill for i18n audit')
  }

  builtinSkillBackfillBuilder = builder
  return builtinSkillBackfillBuilder
}

function getBuiltinToolBackfills() {
  if (builtinToolBackfills) {
    return builtinToolBackfills
  }

  const builtinToolModule = executeTsModule(BUILTIN_TOOL_BACKFILL_MODULE_PATH)
  const catalog = builtinToolModule?.default ?? builtinToolModule?.builtinToolBackfills

  if (!isPlainObject(catalog)) {
    throw new Error('Failed to load builtinToolBackfills for i18n audit')
  }

  builtinToolBackfills = catalog
  return builtinToolBackfills
}

function getHarnessLocaleMerger() {
  if (harnessLocaleMerger) {
    return harnessLocaleMerger
  }

  const localeEnhancerModule = executeTsModule(HARNESS_LOCALE_ADDITIONS_MODULE_PATH)
  const merger =
    localeEnhancerModule?.mergeHarnessLocale ?? localeEnhancerModule?.default?.mergeHarnessLocale

  if (typeof merger !== 'function') {
    throw new Error('Failed to load mergeHarnessLocale for i18n audit')
  }

  harnessLocaleMerger = merger
  return harnessLocaleMerger
}

function isPlainObject(value) {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
}

function cloneMessageValue(value) {
  if (Array.isArray(value)) {
    return value.map((item) => cloneMessageValue(item))
  }
  if (isPlainObject(value)) {
    return Object.fromEntries(
      Object.entries(value).map(([key, nestedValue]) => [key, cloneMessageValue(nestedValue)]),
    )
  }
  return value
}

function deepMergeMessages(base, overrides) {
  const merged = cloneMessageValue(base)

  for (const [key, overrideValue] of Object.entries(overrides)) {
    const baseValue = merged[key]
    if (isPlainObject(baseValue) && isPlainObject(overrideValue)) {
      merged[key] = deepMergeMessages(baseValue, overrideValue)
      continue
    }

    merged[key] = cloneMessageValue(overrideValue)
  }

  return merged
}

function buildAuditedLocaleObject(localeCode, enUSObject, localeCache) {
  const localeBaseObject =
    localeCode === 'en-US'
      ? enUSObject
      : deepMergeMessages(enUSObject, loadLocaleObjectByCode(localeCode, localeCache))

  const builtinToolPatch = getBuiltinToolBackfills()[localeCode] ?? {}
  const localeWithBuiltinTools = deepMergeMessages(localeBaseObject, builtinToolPatch)
  const buildBuiltinSkillBackfill = getBuiltinSkillBackfillBuilder()
  const builtinSkillPatch = buildBuiltinSkillBackfill(localeCode, localeWithBuiltinTools)
  const localeWithRuntimeBase = deepMergeMessages(localeWithBuiltinTools, builtinSkillPatch)

  const buildLocalePostMergeBackfill = getLocalePostMergeBackfillBuilder()
  const runtimeBackfillPatch = buildLocalePostMergeBackfill(localeCode, localeWithRuntimeBase)

  if (!isPlainObject(runtimeBackfillPatch) || Object.keys(runtimeBackfillPatch).length === 0) {
    return getHarnessLocaleMerger()(localeCode, localeWithRuntimeBase)
  }

  return getHarnessLocaleMerger()(
    localeCode,
    deepMergeMessages(localeWithRuntimeBase, runtimeBackfillPatch),
  )
}

function flattenStringLeaves(value, prefix = '', out = new Map()) {
  if (typeof value === 'string') {
    out.set(prefix, value)
    return out
  }
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return out
  }
  for (const [k, v] of Object.entries(value)) {
    const next = prefix ? `${prefix}.${k}` : k
    if (typeof v === 'string') {
      out.set(next, v)
      continue
    }
    if (v && typeof v === 'object' && !Array.isArray(v)) {
      flattenStringLeaves(v, next, out)
    }
  }
  return out
}

function formatPercent(value) {
  if (!Number.isFinite(value)) return '0.0%'
  return `${value.toFixed(1)}%`
}

function pad(str, width, align = 'left') {
  const text = String(str)
  if (text.length >= width) return text
  return align === 'right' ? `${' '.repeat(width - text.length)}${text}` : `${text}${' '.repeat(width - text.length)}`
}

function relative(filePath) {
  return path.relative(WEB_ROOT, filePath)
}



const EXPECTED_SAME_AS_ENGLISH_KEYS = new Set([
  'agent.progress',
  'askQuestion.browserCheckpoint.url',
  'authProviders.placeholderRedirect',
  'bluebubbles',
  'brand.name',
  'browserAutomation.createTask.params.urlPlaceholder',
  'browserAutomation.security.blockedDomainsPlaceholder',
  'browserAutomation.security.testUrlPlaceholder',
  'cache.ttl',
  'channels.groupAccessAllowedChatsPlaceholder',
  'channels.instagramDM',
  'channels.qqBot',
  'channels.twitterDM',
  'chat.stats.ttft',
  'companion.anomalyGuide.types.commandInjection.example',
  'companion.anomalyGuide.types.pathTraversal.example',
  'companion.anomalyGuide.types.sqlInjection.example',
  'companion.anomalyGuide.types.xss.example',
  'connections.http',
  'connections.sse',
  'connections.websocket',
  'csv',
  'dingtalk',
  'discord',
  'feishu',
  'footer.version',
  'googlechat',
  'ideDiscovery.ide',
  'ideDiscovery.oauth',
  'ideDiscovery.sourceCCSwitch',
  'imessage',
  'instagram',
  'json',
  'matrix',
  'mattermost',
  'messenger',
  'metrics.claudeCodeCli',
  'metrics.cpu',
  'metrics.echoServer',
  'metrics.p50',
  'metrics.p90',
  'metrics.p95',
  'metrics.p99',
  'metrics.pid',
  'nextcloudtalk',
  'plugins.skillUrlPlaceholder',
  'qq',
  'resultCard.labels.url',
  'securityAlerts.labels.ip',
  'sandbox.form.workDirPlaceholder',
  'settings.baseUrlPlaceholder',
  'settings.providers.claude',
  'settings.providers.grok',
  'settings.providers.ollama',
  'settings.providers.openai',
  'settings.providers.qwen',
  'settings.providers.siliconflow',
  'signal',
  'skillStore.detail.sections.readme',
  'skillStore.modal.skillURLPlaceholder',
  'autoReply.regex',
  'apiProxy.maskingRuleNames.bearer_token',
  'channels.placeholderServerUrl',
  'channels.placeholderWebhookUrl',
  'channels.oaId',
  'chat.stats.speed',
  'claudecode.title',
  'common.id',
  'companion.anomalyGuide.types.xss.name',
  'connections.recv',
  'extensions.modal.url',
  'extensions.modal.urlPlaceholder',
  'harness.group.model',
  'harness.group.toolName',
  'harness.groups.kind',
  'harness.groups.status',
  'ideDiscovery.envVar',
  'memory.proposalSourceKindUrl',
  'providerPool.apiFormatOptions.google',
  'providerPool.apiFormatOptions.anthropic',
  'providerPool.apiFormatOptions.responses',
  'providerPool.beta',
  'resultCard.labels.ms',
  'resultCard.titles.grep',
  'resultCard.titles.rg',
  'settings.externalAgents.eyebrow',
  'settings.externalAgents.metadata',
  'settings.tts.eta',
  'skillStore.modal.typeClawdhub',
  'skillStore.modal.url',
  'skillStore.modal.urlPlaceholder',
  'skillStore.modal.type',
  'skillStore.downloads',
  'skillStore.detail.meta.downloads',
  'skillStore.marketplace.dynamic.permissions.docker',
  'skillStore.marketplace.dynamic.permissions.system',
  'skillStore.marketplace.dynamic.permissions.shell',
  'skillStore.marketplace.quick.focus',
  'skillStore.search.sortDownloads',
  'skillStore.search.sourceLocal',
  'skillStore.sort.downloads',
  'skillStore.stats.downloads',
  'skillStore.status.local',
  'skillStore.tabs.local',
  'slack',
  'speech.asrModelInfo.whisperBase.name',
  'speech.asrModelInfo.macosNative.name',
  'speech.asrModelInfo.whisperLargeTurbo.name',
  'speech.asrModelInfo.whisperSmall.name',
  'speech.asrModelInfo.whisperTiny.name',
  'speech.convertTask.previewKind.pdf',
  'speech.edgeTTSName',
  'speech.kokoroGithub',
  'system.goMaxProcs',
  'system.cards.goroutines.chip',
  'system.ipv4Address',
  'system.macAddress',
  'system.mtu',
  'system.ram',
  'system.statusOk',
  'system.vram',
  'teams',
  'telegram',
  'tenants.invite.emailPlaceholder',
  'tenants.settings.timezones.utc',
  'tenants.slugPlaceholder',
  'tools.names.mcp',
  'tools.params.id',
  'twitch',
  'twitter',
  'userdata.formatJson',
  'skillStore.modal.name',
  'skillStore.search.sortName',
  'skillStore.sort.name',
])

const EXPECTED_SAME_AS_ENGLISH_PATTERNS = [
  /^security\.scan\.items\.[^.]+\.details\.[^.]+$/,
  /^security\.scan\.items\.[^.]+\.(description|risk|impact|remediation)$/,
  /^security\.firewall\.builtin\.[^.]+\.description$/,
  /^authProviders\.types\.(auth0|authentik|github|google|keycloak|microsoft)$/,
  /^skillStore\.marketplace\.sources\.[^.]+\.label$/,
  /^channels\.(blueBubbles|discord|googleChat|imessage|line|matrix|mattermost|messenger|nextcloudTalk|signal|slack|teams|telegram|twitchBot|viber|whatsapp|zaloOA)$/,
  /^channels\.placeholder(?:AgentId|AppId|BlueBubblesServerUrl|BotToken|DingtalkAppKey|FeishuAppId|MatrixHomeserver|MatrixUserId|PhoneNumber|QQAppId|RobotCode|SlackAppToken|SlackBotToken|WechatCorpId)$/,
  /^companion\.platforms\.(api|discord|feishu|matrix|slack|telegram|whatsapp)$/,
  /^skills\.builtin\.(discord-skill|docker|github|notion|slack-skill)\.name$/,
  /^automation\.tabs\.harness$/,
  /^tenants\.settings\.timezones\.(london|shanghai)$/,
  /^tools\.names\.(discord|docker|github|notion|slack)$/,
]

const EXPECTED_SAME_AS_ENGLISH_BY_LOCALE = new Map(
  Object.entries({
    'terminalCard.title': [
      'ca-ES',
      'da-DK',
      'de-DE',
      'hr-HR',
      'nb-NO',
      'nl-NL',
      'pl-PL',
      'ro-RO',
      'sv-SE',
    ],
    'webhook.title': [
      'ca-ES',
      'da-DK',
      'de-DE',
      'el-GR',
      'fr-FR',
      'nb-NO',
      'pt-BR',
      'pt-PT',
      'sv-SE',
    ],
    'resultCard.labels.bytes': [
      'ca-ES',
      'da-DK',
      'de-DE',
      'el-GR',
      'nb-NO',
      'nl-NL',
      'pt-BR',
      'pt-PT',
    ],
    'billing.filters.groupModel': ['ca-ES', 'cs-CZ', 'da-DK', 'hr-HR', 'nl-NL', 'ro-RO', 'sk-SK'],
    'billing.filters.model': ['ca-ES', 'cs-CZ', 'da-DK', 'hr-HR', 'nl-NL', 'ro-RO', 'sk-SK'],
    'billing.table.model': ['ca-ES', 'cs-CZ', 'da-DK', 'hr-HR', 'nl-NL', 'ro-RO', 'sk-SK'],
    'chat.processTrace.fields.model': [
      'ca-ES',
      'cs-CZ',
      'da-DK',
      'hr-HR',
      'nl-NL',
      'ro-RO',
      'sk-SK',
    ],
    'media.model': ['ca-ES', 'cs-CZ', 'da-DK', 'hr-HR', 'nl-NL', 'ro-RO', 'sk-SK'],
    'metrics.model': ['ca-ES', 'cs-CZ', 'da-DK', 'hr-HR', 'nl-NL', 'ro-RO', 'sk-SK'],
    'settings.tts.model': ['ca-ES', 'cs-CZ', 'da-DK', 'hr-HR', 'nl-NL', 'ro-RO', 'sk-SK'],
    'speech.model': ['ca-ES', 'cs-CZ', 'da-DK', 'hr-HR', 'nl-NL', 'ro-RO', 'sk-SK'],
    'providerPool.capStreaming': ['da-DK', 'de-DE', 'hr-HR', 'hu-HU', 'it-IT', 'ro-RO', 'sv-SE'],
    'speech.streaming': ['da-DK', 'de-DE', 'hr-HR', 'hu-HU', 'it-IT', 'nb-NO', 'ro-RO', 'sv-SE'],
    'connections.quality.offline': [
      'cs-CZ',
      'da-DK',
      'de-DE',
      'hu-HU',
      'nl-NL',
      'ro-RO',
      'sk-SK',
      'sv-SE',
    ],
    'speech.offline': ['cs-CZ', 'da-DK', 'de-DE', 'hu-HU', 'nl-NL', 'ro-RO', 'sk-SK', 'sv-SE'],
    'media.video': ['cs-CZ', 'da-DK', 'hr-HR', 'it-IT', 'nb-NO', 'ro-RO', 'sk-SK', 'sv-SE'],
    'security.tabs.firewall': [
      'cs-CZ',
      'de-DE',
      'it-IT',
      'nl-NL',
      'pt-BR',
      'pt-PT',
      'ro-RO',
      'sk-SK',
    ],
    'localeNames.ml-IN': ['da-DK', 'de-DE', 'fr-FR', 'it-IT', 'nb-NO', 'nl-NL', 'ro-RO', 'sv-SE'],
    'speech.langName.hi-IN': ['de-DE', 'it-IT', 'nb-NO', 'nl-NL', 'pt-BR', 'pt-PT', 'ro-RO'],
    'heartbeat.interval': ['ca-ES', 'cs-CZ', 'da-DK', 'hr-HR', 'nl-NL', 'ro-RO', 'sk-SK'],
    'metrics.max': ['cs-CZ', 'de-DE', 'hu-HU', 'nl-NL', 'ro-RO', 'sk-SK', 'sv-SE'],
    'system.cpuModel': ['ca-ES', 'cs-CZ', 'da-DK', 'hr-HR', 'nl-NL', 'ro-RO', 'sk-SK'],
    'speech.espeakNGQuality': [
      'cs-CZ',
      'da-DK',
      'de-DE',
      'hu-HU',
      'nl-NL',
      'ro-RO',
      'sk-SK',
      'sv-SE',
    ],
    'companion.anomalyGuide.types.longResponseTime.severity': [
      'cs-CZ',
      'da-DK',
      'hr-HR',
      'hu-HU',
      'nb-NO',
      'ro-RO',
      'sk-SK',
      'sv-SE',
    ],
    'chat.deepResearchSourceTypeWeb': [
      'ca-ES',
      'cs-CZ',
      'da-DK',
      'de-DE',
      'es-ES',
      'fr-FR',
      'hr-HR',
      'hu-HU',
      'it-IT',
      'nb-NO',
      'nl-NL',
      'pt-BR',
      'pt-PT',
      'ro-RO',
      'sk-SK',
    ],
    'mermaid.diagram': ['cs-CZ', 'da-DK', 'hu-HU', 'nb-NO', 'nl-NL', 'sk-SK', 'sv-SE'],
    'mermaid.gitgraph': ['cs-CZ', 'da-DK', 'hu-HU', 'nb-NO', 'ro-RO', 'sk-SK', 'sv-SE'],
    'apiProxy.storageTypes.disk': ['cs-CZ', 'da-DK', 'hr-HR', 'nb-NO', 'sk-SK', 'sv-SE'],
    'automation.stats.status': ['da-DK', 'de-DE', 'hr-HR', 'nb-NO', 'nl-NL', 'sv-SE'],
    'billing.table.status': ['da-DK', 'de-DE', 'hr-HR', 'nb-NO', 'nl-NL', 'sv-SE'],
    'browserAutomation.taskDetail.status': [
      'da-DK',
      'de-DE',
      'hr-HR',
      'nb-NO',
      'nl-NL',
      'sv-SE',
    ],
    'cache.status': ['da-DK', 'de-DE', 'hr-HR', 'nb-NO', 'nl-NL', 'sv-SE'],
    'chat.deepResearchStatus': ['da-DK', 'de-DE', 'hr-HR', 'nb-NO', 'nl-NL', 'sv-SE'],
    'chat.processTrace.fields.format': ['ca-ES', 'da-DK', 'hr-HR', 'nb-NO', 'ro-RO', 'sv-SE'],
    'common.filter': ['da-DK', 'de-DE', 'nb-NO', 'nl-NL', 'sk-SK', 'sv-SE'],
    'common.status': ['da-DK', 'de-DE', 'hr-HR', 'nb-NO', 'nl-NL', 'sv-SE'],
    'companion.export.format': ['ca-ES', 'da-DK', 'hr-HR', 'nb-NO', 'ro-RO', 'sv-SE'],
    'companion.flow.status': ['da-DK', 'de-DE', 'hr-HR', 'nb-NO', 'nl-NL', 'sv-SE'],
    'companion.nodes.tokensIn': ['cs-CZ', 'de-DE', 'hu-HU', 'nl-NL', 'sk-SK', 'sv-SE'],
    'companion.statusLabel': ['da-DK', 'de-DE', 'hr-HR', 'nb-NO', 'nl-NL', 'sv-SE'],
    'diffCard.original': ['ca-ES', 'da-DK', 'de-DE', 'nb-NO', 'ro-RO', 'sv-SE'],
    'ideDiscovery.status': ['da-DK', 'de-DE', 'hr-HR', 'nb-NO', 'nl-NL', 'sv-SE'],
    'nav.chat': ['cs-CZ', 'da-DK', 'hu-HU', 'nb-NO', 'ro-RO', 'sk-SK'],
    'profile.scopeChat': ['cs-CZ', 'da-DK', 'hu-HU', 'nb-NO', 'ro-RO', 'sk-SK'],
    'resultCard.labels.format': ['ca-ES', 'da-DK', 'hr-HR', 'nb-NO', 'ro-RO', 'sv-SE'],
    'sandbox.config.status': ['da-DK', 'de-DE', 'hr-HR', 'nb-NO', 'nl-NL', 'sv-SE'],
    'speech.convertTask.previewKind.audio': ['de-DE', 'es-ES', 'fr-FR', 'it-IT', 'nl-NL', 'ro-RO'],
    'speech.convertTask.previewKind.text': ['ca-ES', 'cs-CZ', 'de-DE', 'ro-RO', 'sk-SK', 'sv-SE'],
    'speech.windowsNativeName': ['da-DK', 'el-GR', 'hu-HU', 'nb-NO', 'ro-RO', 'sv-SE'],
    'system.status': ['da-DK', 'de-DE', 'hr-HR', 'nb-NO', 'nl-NL', 'sv-SE'],
    'users.pagePermissions.page.chat': ['cs-CZ', 'da-DK', 'hu-HU', 'nb-NO', 'ro-RO', 'sk-SK'],
    'users.status': ['da-DK', 'de-DE', 'hr-HR', 'nb-NO', 'nl-NL', 'sv-SE'],
    'workspace.tokens': ['da-DK', 'nb-NO', 'nl-NL', 'pt-BR', 'pt-PT', 'sv-SE'],
    'a2ui.title': ['da-DK', 'el-GR', 'nb-NO', 'sv-SE'],
    'channels.placeholderBotTokenGeneric': ['cs-CZ', 'da-DK', 'hr-HR', 'hu-HU', 'nb-NO'],
    'channels.placeholderBotUsername': ['el-GR', 'hu-HU', 'ml-IN', 'ro-RO', 'sk-SK'],
    'channels.placeholderZaloAppId': ['da-DK', 'ga-IE', 'hu-HU', 'nb-NO', 'sv-SE'],
    'channels.placeholderLineChannelSecret': ['da-DK', 'hu-HU', 'nb-NO', 'sv-SE'],
    'channels.placeholderOaRefreshToken': ['da-DK', 'hu-HU', 'nb-NO', 'sv-SE'],
    'channels.placeholderTwitterAccessToken': ['da-DK', 'nb-NO', 'sk-SK', 'sv-SE'],
    'chat.deepResearchModeStandard': ['da-DK', 'de-DE', 'nb-NO', 'ro-RO', 'sv-SE'],
    'providerPool.capChat': ['cs-CZ', 'da-DK', 'nb-NO', 'ro-RO', 'sk-SK'],
    'speech.asrModelInfo.zipformerEn.name': ['da-DK', 'hr-HR', 'hu-HU', 'nb-NO', 'sv-SE'],
    'tenants.invite.role': ['cs-CZ'],
    'tenants.settings.timezones.tokyo': ['da-DK', 'fr-FR', 'nb-NO', 'ro-RO', 'sv-SE'],
    'theme.styles.minimal': ['da-DK', 'de-DE', 'nb-NO', 'ro-RO', 'sv-SE'],
    'theme.styles.ocean': ['da-DK', 'hr-HR', 'pl-PL', 'ro-RO', 'sv-SE'],
    'browserMonitor.buttonLabel': ['ca-ES', 'cs-CZ', 'hr-HR', 'hu-HU', 'nl-NL', 'sk-SK'],
    'audit.totalEntries': ['ca-ES', 'fr-FR', 'pt-BR', 'pt-PT', 'ro-RO'],
    'browserAutomation.createTask.params.timeoutPlaceholder': [
      'da-DK',
      'de-DE',
      'it-IT',
      'ro-RO',
      'sv-SE',
    ],
    'common.total': ['ca-ES', 'fr-FR', 'pt-BR', 'pt-PT', 'ro-RO'],
    'cron.timeout': ['da-DK', 'el-GR', 'it-IT', 'ro-RO', 'sv-SE'],
    'media.prompt': ['de-DE', 'hu-HU', 'nl-NL', 'ro-RO', 'sk-SK'],
    'metrics.total': ['ca-ES', 'fr-FR', 'pt-BR', 'pt-PT', 'ro-RO'],
    'sandbox.form.timeout': ['da-DK', 'el-GR', 'it-IT', 'ro-RO', 'sv-SE'],
    'speech.volume': ['fr-FR', 'it-IT', 'nl-NL', 'pt-BR', 'pt-PT'],
    'system.totalSpace': ['ca-ES', 'fr-FR', 'pt-BR', 'pt-PT', 'ro-RO'],
    'theme.styles.gradient': ['da-DK', 'nb-NO', 'ro-RO', 'sv-SE'],
    'tokenEconomy.pruner': ['cs-CZ', 'ga-IE', 'ro-RO', 'sk-SK'],
    'users.pagePermissions.page.home': ['cs-CZ', 'da-DK', 'nl-NL', 'sk-SK'],
    'apiProxy.maskingCategories.pii': ['cs-CZ', 'de-DE', 'ga-IE'],
    'audit.logins': ['ga-IE', 'pt-BR', 'pt-PT'],
    'approval.arguments': ['ca-ES', 'fr-FR'],
    'autoReply.prefix': ['ca-ES', 'ro-RO', 'sv-SE'],
    'autoReply.suffix': ['de-DE', 'sv-SE'],
    'backup.types.config': ['ca-ES', 'ro-RO'],
    'backup.types.full': ['nb-NO', 'sv-SE'],
    'billing.anomalyTable.cost': ['ca-ES', 'ro-RO'],
    'billing.table.estimatedCost': ['ca-ES', 'ro-RO'],
    'billing.table.session': ['da-DK', 'sv-SE'],
    'cache.prunerCompression': ['fr-FR'],
    'billing.drilldownActionFilter': ['da-DK', 'nb-NO', 'sk-SK'],
    'browserAutomation.security.test': ['da-DK', 'nb-NO'],
    'browserAutomation.security.testUrl': ['da-DK', 'nb-NO'],
    'browserAutomation.sessions.session': ['da-DK', 'sv-SE'],
    'browserAutomation.taskDetail.error': ['ca-ES', 'es-ES'],
    'browserAutomation.templates.webScraping.name': ['el-GR', 'hr-HR', 'ro-RO'],
    'browserMonitor.taskViewSmart': ['da-DK', 'nb-NO', 'sv-SE'],
    'cache.details': ['de-DE', 'nl-NL'],
    'channels.placeholderLineChannelToken': ['da-DK', 'nb-NO', 'sv-SE'],
    'channels.placeholderOaAccessToken': ['da-DK', 'nb-NO', 'sv-SE'],
    'channels.placeholderSecret': ['ca-ES', 'ro-RO'],
    'channels.placeholderTwitterApiSecret': ['cs-CZ', 'el-GR', 'hr-HR'],
    'channels.secret': ['ca-ES', 'fr-FR', 'ro-RO'],
    'channels.webhookUrl': ['hu-HU', 'ja-JP'],
    'chat.deepResearchIteration': ['de-DE', 'sv-SE'],
    'chat.deepResearchParallelism': ['da-DK', 'de-DE', 'nl-NL'],
    'chat.processTrace.fields.mode': ['ca-ES', 'fr-FR'],
    'chat.routingMode.cloud': ['cs-CZ', 'sk-SK'],
    'chat.routingMode.local': ['ca-ES', 'ro-RO'],
    'claudecode.platform': ['da-DK', 'hu-HU', 'nl-NL'],
    'claudecode.version': ['de-DE', 'fr-FR', 'sv-SE'],
    'cli.platform': ['da-DK', 'hu-HU', 'nl-NL'],
    'cli.version': ['de-DE', 'fr-FR', 'sv-SE'],
    'common.no': ['ca-ES', 'es-ES', 'it-IT'],
    'companion.anomalyGuide.types.sqlInjection.name': ['cs-CZ', 'el-GR', 'sk-SK'],
    'companion.demo.stopDemo': ['da-DK', 'ga-IE', 'hu-HU'],
    'companion.flow.nodeTypes.sandbox': ['ca-ES', 'el-GR', 'hu-HU'],
    'companion.platform': ['da-DK', 'hu-HU', 'nl-NL'],
    'companion.security.score': ['da-DK', 'fr-FR', 'nb-NO'],
    'companion.viewMode.flow': ['da-DK', 'hu-HU', 'nb-NO'],
    'errors.http504': ['da-DK', 'hr-HR', 'sv-SE'],
    'execCard.sandbox': ['ca-ES', 'el-GR', 'hu-HU'],
    'harness.terms.version': ['da-DK', 'fr-FR', 'sv-SE'],
    'ideDiscovery.parse': ['da-DK', 'nb-NO', 'sk-SK'],
    'memory.score': ['da-DK', 'fr-FR', 'nb-NO'],
    'skillStore.marketplace.artifactKinds.open_source': ['cs-CZ', 'da-DK', 'sk-SK'],
    'skillStore.marketplace.security.scripts': ['da-DK', 'fr-FR', 'nl-NL'],
    'speech.langName.th-TH': ['da-DK', 'nb-NO', 'sv-SE'],
    'system.version': ['de-DE', 'fr-FR', 'sv-SE'],
    'thinking.context': ['ca-ES', 'nl-NL', 'ro-RO'],
    'userdata.version': ['de-DE', 'fr-FR', 'sv-SE'],
    'profile.email': ['cs-CZ', 'el-GR', 'sk-SK'],
    'profile.role': ['cs-CZ', 'sk-SK'],
    'providerPool.accountStatus.items.limit': ['cs-CZ', 'pl-PL', 'sk-SK'],
    'resultCard.labels.urls': ['de-DE', 'pt-BR', 'pt-PT'],
    'audit.filter': ['da-DK', 'hr-HR', 'nb-NO', 'sk-SK'],
    'action': ['fr-FR'],
    'apiProxy.customMaskingNameLabel': ['de-DE'],
    'askQuestion.browserCheckpoint.action': ['fr-FR'],
    'backup.types.data': ['cs-CZ', 'da-DK', 'nb-NO', 'sv-SE'],
    'automation.stats.sessions': ['ca-ES'],
    'browserAutomation.categories.data': ['cs-CZ', 'da-DK', 'nb-NO', 'sv-SE'],
    'browserAutomation.tabs.sessions': ['ca-ES'],
    'channels.placeholderTwitterAccessTokenSecret': ['da-DK', 'hr-HR', 'nb-NO', 'sv-SE'],
    'channels.statusError': ['ca-ES'],
    'claudecode.sourceSystem': ['da-DK', 'de-DE', 'nb-NO', 'sv-SE'],
    'common.system': ['da-DK', 'de-DE', 'nb-NO', 'sv-SE'],
    'companion.metadata': ['cs-CZ', 'da-DK', 'nb-NO', 'sv-SE'],
    'companion.nodes.tokens': ['da-DK', 'nb-NO', 'nl-NL', 'sv-SE'],
    'connections.filter': ['da-DK', 'hr-HR', 'nb-NO', 'sk-SK'],
    'browserAutomation.stepLabels.hover': ['ga-IE', 'ro-RO'],
    'common.liveStreaming': ['da-DK', 'nb-NO'],
    'common.error': ['ca-ES'],
    'common.name': ['de-DE'],
    'common.optional': ['de-DE'],
    'common.send': ['da-DK', 'nb-NO'],
    'common.test': ['da-DK', 'nb-NO'],
    'common.type': ['da-DK', 'nb-NO'],
    'common.upload': ['da-DK'],
    'dashboard.categories.system': ['da-DK', 'de-DE', 'nb-NO', 'sv-SE'],
    'dashboard.cpus': ['ca-ES', 'de-DE', 'pt-BR', 'pt-PT'],
    'dashboard.title': ['cs-CZ', 'da-DK', 'nl-NL', 'sk-SK'],
    'harness.groups.score': ['da-DK', 'de-DE', 'fr-FR', 'nl-NL'],
    'ideDiscovery.sourceExtension': ['da-DK', 'hr-HR', 'nb-NO', 'sv-SE'],
    'memory.matchTypes.vector': ['ca-ES', 'es-ES', 'nl-NL', 'ro-RO'],
    'mermaid.mindmap': ['da-DK', 'de-DE', 'hr-HR', 'nl-NL'],
    'myProviders.test': ['cs-CZ', 'da-DK', 'nb-NO', 'sk-SK'],
    'nav.dashboard': ['cs-CZ', 'da-DK', 'nl-NL', 'sk-SK'],
    'pause': ['da-DK', 'de-DE', 'fr-FR', 'nb-NO'],
    'providerPool.capVision': ['da-DK', 'de-DE', 'fr-FR', 'sv-SE'],
    'providerPool.test': ['cs-CZ', 'da-DK', 'nb-NO', 'sk-SK'],
    'sandbox.form.arguments': ['ca-ES', 'fr-FR'],
    'send': ['da-DK', 'nb-NO'],
    'settings.tab.backup': ['da-DK', 'it-IT'],
    'settings.tts.local': ['ca-ES', 'ro-RO'],
    'settings.tts.title': ['de-DE', 'ro-RO'],
    'skillStore.marketplace.embedding.phaseStandby': ['da-DK', 'sv-SE'],
    'skillStore.marketplace.evidenceTypes.secret': ['ca-ES', 'fr-FR'],
    'skillStore.marketplace.progress.phaseStandby': ['da-DK', 'sv-SE'],
    'skillStore.marketplace.security.score': ['da-DK', 'fr-FR'],
    'speech.convertTask.action.trim': ['da-DK', 'nb-NO'],
    'speech.edgeTTSQuality': ['ca-ES', 'ro-RO'],
    'speech.espeakDepVocoder': ['da-DK', 'sv-SE'],
    'speech.kokoroQuality': ['ca-ES', 'ro-RO'],
    'start': ['da-DK', 'nb-NO'],
    'autoReply.card.trigger': ['nl-NL'],
    'billing.anomalyTable.action': ['fr-FR'],
    'chat.deepResearchWorkflowExtraction': ['fr-FR'],
    'chat.deepResearchWorkflowSources': ['fr-FR'],
    'chat.noProvider.statusError': ['ca-ES'],
    'chat.processTrace.fields.conversation': ['fr-FR'],
    'chat.processTrace.fields.file': ['it-IT'],
    'chat.processTrace.fields.message': ['fr-FR'],
    'chat.send': ['da-DK', 'nb-NO'],
    'chat.talkMode.conversation': ['fr-FR'],
    'chat.transcription.send': ['da-DK', 'nb-NO'],
    'chat.transcription.title': ['fr-FR'],
    'chat.deepResearchConflict': ['ro-RO'],
    'chat.stats.inputTokens': ['ro-RO', 'sv-SE'],
    'companion.anomalyGuide.types.rateLimit.severity': ['da-DK', 'sv-SE'],
    'companion.anomalyGuide.types.commandInjection.name': ['hu-HU'],
    'companion.anomalyGuide.types.promptInjection.name': ['cs-CZ'],
    'companion.anomalyGuide.types.rateLimit.name': ['hu-HU'],
    'companion.direction.inbound': ['ro-RO'],
    'companion.eventDetails.tokens': ['nb-NO', 'nl-NL'],
    'companion.eventDetails.input': ['da-DK'],
    'companion.eventType.error': ['ca-ES'],
    'companion.export.download': ['da-DK'],
    'companion.export.sessions': ['ca-ES'],
    'companion.export.to': ['hu-HU'],
    'companion.flow.type': ['da-DK', 'nb-NO'],
    'companion.flow.nodeTypes.message': ['fr-FR'],
    'companion.llmDetails.score': ['da-DK', 'fr-FR'],
    'companion.llmDetails.input': ['da-DK'],
    'companion.messageCount': ['fr-FR'],
    'companion.messages': ['fr-FR'],
    'companion.nodes.message': ['fr-FR'],
    'companion.nodes.sandbox': ['el-GR'],
    'companion.nodes.status.error': ['ca-ES'],
    'companion.realtime': ['nl-NL'],
    'companion.security.action': ['fr-FR'],
    'companion.security.details': ['de-DE', 'nl-NL'],
    'companion.sessions': ['ca-ES'],
    'companion.status.error': ['ca-ES'],
    'companion.threat.medium': ['da-DK', 'sv-SE'],
    'companion.tokens': ['nb-NO', 'nl-NL'],
    'connections.bytesSent': ['el-GR'],
    'connections.quality.excellent': ['fr-FR'],
    'cron.handlers.command': ['hu-HU'],
    'cron.name': ['de-DE'],
    'dashboard.uptime': ['nl-NL', 'sk-SK'],
    'errors.http409': ['ro-RO'],
    'execCard.local': ['ca-ES', 'ro-RO'],
    'execCard.session': ['da-DK', 'sv-SE'],
    'extensions.browse.sourceLabel': ['fr-FR'],
    'extensions.modal.debug': ['it-IT'],
    'extensions.modal.error': ['ca-ES'],
    'extensions.modal.name': ['de-DE'],
    'extensions.modal.type': ['da-DK', 'nb-NO'],
    'extensions.status.error': ['ca-ES'],
    'chat.deepResearchTimeWindowRecent': ['it-IT', 'pt-BR', 'pt-PT'],
    'claudecode.directoryAliasPlaceholder': ['de-DE'],
    'harness.group.errorVerdict': ['ca-ES', 'es-ES'],
    'harness.group.items': ['nl-NL'],
    'harness.groups.activeOnly': ['ro-RO'],
    'harness.groups.filters': ['nl-NL'],
    'ideDiscovery.oauthType': ['da-DK', 'nb-NO'],
    'media.fallbackLinkSource': ['fr-FR'],
    'media.fallbackTemplatePoster': ['ro-RO'],
    'memory.proposalScore': ['da-DK', 'fr-FR'],
    'memory.proposalVerdict': ['fr-FR', 'ro-RO'],
    'memory.prune': ['hu-HU', 'ro-RO'],
    'metrics.cost': ['ca-ES', 'ro-RO'],
    'metrics.tokensPerSecond': ['nl-NL'],
    'metrics.tokens': ['nb-NO', 'nl-NL'],
    'metrics.uptime': ['nl-NL', 'sk-SK'],
    'nav.configuration': ['fr-FR'],
    'nav.plugins': ['ca-ES'],
    'nav.workspaceUnknownConversation': ['fr-FR'],
    'network.hostname': ['de-DE'],
    'plugins.error': ['ca-ES'],
    'profile.expiration': ['fr-FR'],
    'profile.name': ['de-DE'],
    'providerPool.keyModels': ['ca-ES'],
    'providerPool.oauth.project': ['nl-NL'],
    'push.title': ['fr-FR'],
    'resultCard.labels.expression': ['fr-FR'],
    'resultCard.labels.pages': ['fr-FR'],
    'resultCard.labels.source': ['fr-FR'],
    'resultCard.labels.sources': ['fr-FR'],
    'resultCard.titles.calculator': ['ro-RO'],
    'resultCard.values.strategy.screenshot': ['de-DE'],
    'sandbox.configuration': ['fr-FR'],
    'sandbox.error': ['ca-ES'],
    'securityAlerts.labels.auth': ['de-DE'],
    'service.port': ['da-DK'],
    'settings.antigravity.models': ['ca-ES'],
    'settings.failover.configuration': ['fr-FR'],
    'settings.smallModel.source': ['fr-FR'],
    'settings.tab.general': ['ca-ES'],
    'settings.tab.service': ['fr-FR'],
    'skills.catalog.configuration.name': ['fr-FR'],
    'skills.names.Configuration': ['fr-FR'],
    'skillStore.empty.suggestions': ['fr-FR'],
    'skillStore.marketplace.filters.source': ['fr-FR'],
    'skillStore.marketplace.progress.catalog': ['ro-RO'],
    'skillStore.marketplace.progress.sourcesLabel': ['fr-FR'],
    'skillStore.marketplace.results.catalog': ['ro-RO'],
    'skillStore.marketplace.results.installable': ['fr-FR'],
    'skillStore.marketplace.results.pageState': ['fr-FR'],
    'skillStore.marketplace.results.sources': ['fr-FR'],
    'skillStore.marketplace.results.sourcesLabel': ['fr-FR'],
    'skillStore.marketplace.security.installable': ['fr-FR'],
    'skillStore.marketplace.sourceImport.sourceType': ['fr-FR'],
    'skillStore.modal.skillNameOptional': ['de-DE'],
    'mediaStats.images': ['fr-FR'],
    'mediaStats.videos': ['de-DE'],
    'analyze.reportStyle': ['fr-FR'],
    'resultCard.labels.links': ['da-DK', 'de-DE'],
    'resultCard.labels.session_id': ['da-DK', 'sv-SE'],
    'resultCard.labels.data': ['cs-CZ', 'da-DK', 'nb-NO', 'sv-SE'],
    'sandbox.config.maxTimeout': ['hr-HR', 'hu-HU'],
    'security.scan.categories.system': ['da-DK', 'de-DE', 'nb-NO', 'sv-SE'],
    'system.cpus': ['ca-ES', 'de-DE', 'pt-BR', 'pt-PT'],
    'system.debug': ['da-DK', 'hr-HR', 'it-IT', 'sk-SK'],
    'system.formatJson': ['ca-ES', 'hr-HR'],
    'system.heapAllocation': ['da-DK', 'nb-NO'],
    'system.title': ['da-DK', 'de-DE', 'nb-NO', 'sv-SE'],
    'system.totalAlloc': ['ca-ES', 'sv-SE'],
    'test': ['cs-CZ', 'da-DK', 'nb-NO', 'sk-SK'],
    'users.role': ['cs-CZ', 'sk-SK'],
    'webhook.type': ['da-DK', 'nb-NO'],
  })
)

function isExpectedSameAsEnglishKey(key, locale) {
  if (locale === 'en-GB') return true
  const localeScopedExpected = EXPECTED_SAME_AS_ENGLISH_BY_LOCALE.get(key)
  if (localeScopedExpected?.includes(locale)) return true
  if (EXPECTED_SAME_AS_ENGLISH_KEYS.has(key)) return true
  return EXPECTED_SAME_AS_ENGLISH_PATTERNS.some((pattern) => pattern.test(key))
}

const APPROVED_DYNAMIC_EXPRESSION_WHITELIST = [
  {
    file: /src\/components\/typeless\/CardAnalyzeProgress\.vue$/,
    expr: /^key$/,
    reason: 'typeless card step labels are normalized to runtime key map',
  },
  {
    file: /src\/components\/typeless\/CardResult\.vue$/,
    expr: /^key$/,
    reason: 'typeless result card derives labels/actions from runtime payload',
  },
  {
    file: /src\/utils\/typeless\.ts$/,
    expr: /^key$/,
    reason: 'shared typeless translator helper intentionally accepts dynamic keys',
  },
  {
    file: /src\/utils\/typelessRenderers\.ts$/,
    expr: /^key$/,
    reason: 'typeless renderer helper intentionally accepts dynamic keys',
  },
]

function matchDynamicExpressionWhitelist(entry) {
  const file = relative(entry.file)
  for (const rule of APPROVED_DYNAMIC_EXPRESSION_WHITELIST) {
    if (rule.file.test(file) && rule.expr.test(entry.expr.trim())) {
      return { file, reason: rule.reason }
    }
  }
  return null
}

export function getAuditExitCode(
  report,
  options = { failOnUnknownStaticKeys: false, failOnUnresolvedDynamicPatterns: false }
) {
  if (options.failOnUnknownStaticKeys && report.unknownStaticKeys.length > 0) {
    return EXIT_CODES.violations
  }

  if (options.failOnUnresolvedDynamicPatterns && report.unresolvedDynamicPatterns.length > 0) {
    return EXIT_CODES.violations
  }

  return EXIT_CODES.ok
}

export function main(argv = process.argv.slice(2)) {
  const args = new Set(argv)
  const details = args.has('--details')
  const json = args.has('--json')
  const failOnUnknownStaticKeys = args.has('--fail-on-unknown-static-keys')
  const failOnUnresolvedDynamicPatterns = args.has('--fail-on-unresolved-dynamic-patterns')

  const localeFiles = fs
    .readdirSync(LOCALES_DIR)
    .filter((name) => name.endsWith('.ts'))
    .sort()

  if (!localeFiles.includes('en-US.ts')) {
    throw new Error('en-US.ts not found in locale directory')
  }

  const localeCache = new Map()
  const enUSObject = loadLocaleObjectByCode('en-US', localeCache)
  const enUSRuntimeObject = buildAuditedLocaleObject('en-US', enUSObject, localeCache)
  const enUSMap = flattenStringLeaves(enUSRuntimeObject)
  const enUSKeys = [...enUSMap.keys()].sort()
  const enUSKeySet = new Set(enUSKeys)

  const sourceFiles = collectFiles(SRC_DIR, ['.vue', '.ts', '.js']).filter(
    (filePath) => !filePath.includes(`${path.sep}i18n${path.sep}locales${path.sep}`),
  )

  const allStaticUsage = []
  const allDynamicPatterns = []
  const allDynamicExpressions = []
  const allDynamicExistenceExpressions = []
  const allDynamicGuardedExpressions = []
  for (const filePath of sourceFiles) {
    const content = fs.readFileSync(filePath, 'utf8')
    const usage = extractKeyUsage(filePath, content)
    allStaticUsage.push(...usage.staticKeys)
    allDynamicPatterns.push(...usage.dynamicPatterns)
    allDynamicExpressions.push(...usage.dynamicExpressions)
    allDynamicExistenceExpressions.push(...usage.dynamicExistenceExpressions)
    allDynamicGuardedExpressions.push(...usage.dynamicGuardedExpressions)
  }

  const staticByKey = new Map()
  for (const item of allStaticUsage) {
    const list = staticByKey.get(item.key) || []
    list.push(item)
    staticByKey.set(item.key, list)
  }

  const knownStaticKeys = [...staticByKey.keys()].filter((key) => enUSKeySet.has(key)).sort()
  const unknownStaticKeys = [...staticByKey.keys()].filter((key) => !enUSKeySet.has(key)).sort()

  const dynamicPatternMap = new Map()
  for (const pattern of allDynamicPatterns) {
    const dedupeKey = `${pattern.template}|${pattern.source}|${relative(pattern.file)}:${pattern.line}`
    if (!dynamicPatternMap.has(dedupeKey)) {
      dynamicPatternMap.set(dedupeKey, pattern)
    }
  }
  const dynamicPatterns = [...dynamicPatternMap.values()]

  const dynamicMatches = []
  const dynamicMatchedKeys = new Set()
  const unresolvedDynamicPatterns = []
  for (const pattern of dynamicPatterns) {
    const matches = enUSKeys.filter((key) => pattern.regex.test(key))
    if (matches.length === 0) {
      unresolvedDynamicPatterns.push(pattern)
      continue
    }
    for (const key of matches) dynamicMatchedKeys.add(key)
    dynamicMatches.push({
      ...pattern,
      matchCount: matches.length,
    })
  }

  const usedKeys = new Set([...knownStaticKeys, ...dynamicMatchedKeys])
  const usedKeysArray = [...usedKeys].sort()

  const locales = []
  for (const fileName of localeFiles) {
    const locale = sanitizeLocaleCode(fileName)
    const filePath = path.join(LOCALES_DIR, fileName)
    const localeObject = buildAuditedLocaleObject(locale, enUSObject, localeCache)
    const localeMap = flattenStringLeaves(localeObject)
    const missing = []
    const fallbackToEnglish = []
    const expectedSameAsEnglish = []

    for (const key of usedKeysArray) {
      const enValue = enUSMap.get(key)
      const localeValue = localeMap.get(key)
      if (localeValue === undefined) {
        missing.push(key)
        continue
      }
      if (locale !== 'en-US' && enValue !== undefined && localeValue === enValue) {
        if (isExpectedSameAsEnglishKey(key, locale)) {
          expectedSameAsEnglish.push(key)
        } else {
          fallbackToEnglish.push(key)
        }
      }
    }

    const translatedCount = usedKeysArray.length - missing.length - fallbackToEnglish.length
    const translatedPct = usedKeysArray.length === 0 ? 0 : (translatedCount / usedKeysArray.length) * 100

    locales.push({
      locale,
      file: relative(filePath),
      totalUsedKeys: usedKeysArray.length,
      missingCount: missing.length,
      fallbackCount: fallbackToEnglish.length,
      translatedCount,
      translatedPct,
      missingKeys: missing,
      fallbackKeys: fallbackToEnglish,
      expectedSameAsEnglishKeys: expectedSameAsEnglish,
    })
  }

  locales.sort((a, b) => a.locale.localeCompare(b.locale))

  const dedupedDynamicExpressions = []
  const dynamicExpressionSeen = new Set()
  for (const expr of allDynamicExpressions) {
    const k = `${relative(expr.file)}:${expr.line}:${expr.expr}`
    if (dynamicExpressionSeen.has(k)) continue
    dynamicExpressionSeen.add(k)
    dedupedDynamicExpressions.push(expr)
  }

  const whitelistedDynamicExpressions = []
  const manualDynamicExpressions = []
  for (const expr of dedupedDynamicExpressions) {
    const match = matchDynamicExpressionWhitelist(expr)
    if (match) {
      whitelistedDynamicExpressions.push({ ...expr, ...match })
    } else {
      manualDynamicExpressions.push(expr)
    }
  }

  const dedupedDynamicExistenceExpressions = []
  const dynamicExistenceSeen = new Set()
  for (const expr of allDynamicExistenceExpressions) {
    const k = `${relative(expr.file)}:${expr.line}:${expr.expr}`
    if (dynamicExistenceSeen.has(k)) continue
    dynamicExistenceSeen.add(k)
    dedupedDynamicExistenceExpressions.push(expr)
  }

  const dedupedDynamicGuardedExpressions = []
  const dynamicGuardedSeen = new Set()
  for (const expr of allDynamicGuardedExpressions) {
    const k = `${relative(expr.file)}:${expr.line}:${expr.expr}`
    if (dynamicGuardedSeen.has(k)) continue
    dynamicGuardedSeen.add(k)
    dedupedDynamicGuardedExpressions.push(expr)
  }

  const report = {
    generatedAt: new Date().toISOString(),
    localesCount: locales.length,
    sourceFilesScanned: sourceFiles.length,
    enUSKeyCount: enUSKeys.length,
    staticKeysTotal: staticByKey.size,
    staticKeysKnown: knownStaticKeys.length,
    staticKeysUnknown: unknownStaticKeys.length,
    dynamicPatternsTotal: dynamicPatterns.length,
    dynamicPatternsResolved: dynamicMatches.length,
    dynamicPatternsUnresolved: unresolvedDynamicPatterns.length,
    dynamicExpressionCalls: manualDynamicExpressions.length,
    dynamicExpressionWhitelisted: whitelistedDynamicExpressions.length,
    dynamicExistenceChecks: dedupedDynamicExistenceExpressions.length,
    dynamicGuardedCalls: dedupedDynamicGuardedExpressions.length,
    usedKeyCount: usedKeysArray.length,
    unknownStaticKeys,
    unresolvedDynamicPatterns: unresolvedDynamicPatterns.map((p) => ({
      template: p.template,
      file: relative(p.file),
      line: p.line,
      source: p.source,
    })),
    dynamicExpressionCallsList: manualDynamicExpressions.map((expr) => ({
      expr: expr.expr,
      file: relative(expr.file),
      line: expr.line,
    })),
    dynamicExpressionWhitelistedList: whitelistedDynamicExpressions.map((expr) => ({
      expr: expr.expr,
      file: expr.file,
      line: expr.line,
      reason: expr.reason,
    })),
    dynamicExistenceChecksList: dedupedDynamicExistenceExpressions.map((expr) => ({
      expr: expr.expr,
      file: relative(expr.file),
      line: expr.line,
    })),
    dynamicGuardedCallsList: dedupedDynamicGuardedExpressions.map((expr) => ({
      expr: expr.expr,
      file: relative(expr.file),
      line: expr.line,
    })),
    locales: locales.map((l) => ({
      locale: l.locale,
      file: l.file,
      totalUsedKeys: l.totalUsedKeys,
      missingCount: l.missingCount,
      fallbackCount: l.fallbackCount,
      translatedCount: l.translatedCount,
      translatedPct: Number(l.translatedPct.toFixed(2)),
      missingKeys: l.missingKeys,
      fallbackKeys: l.fallbackKeys,
      expectedSameAsEnglishKeys: l.expectedSameAsEnglishKeys,
    })),
  }

  if (json) {
    process.stdout.write(`${JSON.stringify(report, null, 2)}\n`)
    return getAuditExitCode(report, {
      failOnUnknownStaticKeys,
      failOnUnresolvedDynamicPatterns,
    })
  }

  console.log('\nweb i18n audit')
  console.log('-------------')
  console.log(`locales: ${report.localesCount}`)
  console.log(`source files scanned: ${report.sourceFilesScanned}`)
  console.log(`en-US translation keys: ${report.enUSKeyCount}`)
  console.log(`used keys in UI (static + resolved dynamic): ${report.usedKeyCount}`)
  console.log(`static keys: ${report.staticKeysKnown} known / ${report.staticKeysUnknown} unknown`)
  console.log(
    `dynamic key patterns: ${report.dynamicPatternsResolved} resolved / ${report.dynamicPatternsUnresolved} unresolved`,
  )
  console.log(`dynamic expression calls (manual review): ${report.dynamicExpressionCalls}`)
  console.log(`dynamic expression calls (whitelisted): ${report.dynamicExpressionWhitelisted}`)
  console.log(`dynamic existence checks (te): ${report.dynamicExistenceChecks}`)
  console.log(`dynamic guarded calls (te + t): ${report.dynamicGuardedCalls}`)
  const expectedSameAsEnglishCount = locales.reduce((sum, locale) => sum + locale.expectedSameAsEnglishKeys.length, 0)
  console.log(`expected same-as-English keys skipped: ${expectedSameAsEnglishCount}`)

  const header = [
    pad('Locale', 10),
    pad('Missing', 8, 'right'),
    pad('FallbackEN', 11, 'right'),
    pad('Translated', 11, 'right'),
    pad('Coverage', 9, 'right'),
  ].join('  ')
  console.log(`\n${header}`)
  console.log('-'.repeat(header.length))
  for (const l of locales) {
    console.log(
      [
        pad(l.locale, 10),
        pad(l.missingCount, 8, 'right'),
        pad(l.fallbackCount, 11, 'right'),
        pad(l.translatedCount, 11, 'right'),
        pad(formatPercent(l.translatedPct), 9, 'right'),
      ].join('  '),
    )
  }

  if (unknownStaticKeys.length > 0) {
    console.log('\nunknown static keys (not found in en-US.ts):')
    for (const key of unknownStaticKeys.slice(0, 50)) {
      const firstUse = staticByKey.get(key)?.[0]
      if (!firstUse) continue
      console.log(`- ${key} (${relative(firstUse.file)}:${firstUse.line})`)
    }
    if (unknownStaticKeys.length > 50) {
      console.log(`- ... and ${unknownStaticKeys.length - 50} more`)
    }
  }

  if (unresolvedDynamicPatterns.length > 0) {
    console.log('\nunresolved dynamic key patterns:')
    for (const p of unresolvedDynamicPatterns.slice(0, 50)) {
      console.log(`- ${p.template} (${relative(p.file)}:${p.line}, ${p.source})`)
    }
    if (unresolvedDynamicPatterns.length > 50) {
      console.log(`- ... and ${unresolvedDynamicPatterns.length - 50} more`)
    }
  }

  if (manualDynamicExpressions.length > 0) {
    console.log('\ndynamic translation expressions (manual review):')
    for (const expr of manualDynamicExpressions.slice(0, 50)) {
      console.log(`- ${expr.expr} (${relative(expr.file)}:${expr.line})`)
    }
    if (manualDynamicExpressions.length > 50) {
      console.log(`- ... and ${manualDynamicExpressions.length - 50} more`)
    }
  }

  if (whitelistedDynamicExpressions.length > 0) {
    console.log('\ndynamic translation expressions (whitelisted):')
    for (const expr of whitelistedDynamicExpressions.slice(0, 20)) {
      console.log(`- ${expr.expr} (${expr.file}:${expr.line}) [${expr.reason}]`)
    }
    if (whitelistedDynamicExpressions.length > 20) {
      console.log(`- ... and ${whitelistedDynamicExpressions.length - 20} more`)
    }
  }

  if (dedupedDynamicExistenceExpressions.length > 0) {
    console.log('\ndynamic translation existence checks (te):')
    for (const expr of dedupedDynamicExistenceExpressions.slice(0, 30)) {
      console.log(`- ${expr.expr} (${relative(expr.file)}:${expr.line})`)
    }
    if (dedupedDynamicExistenceExpressions.length > 30) {
      console.log(`- ... and ${dedupedDynamicExistenceExpressions.length - 30} more`)
    }
  }

  if (dedupedDynamicGuardedExpressions.length > 0) {
    console.log('\ndynamic translation guarded calls (te + t):')
    for (const expr of dedupedDynamicGuardedExpressions.slice(0, 30)) {
      console.log(`- ${expr.expr} (${relative(expr.file)}:${expr.line})`)
    }
    if (dedupedDynamicGuardedExpressions.length > 30) {
      console.log(`- ... and ${dedupedDynamicGuardedExpressions.length - 30} more`)
    }
  }

  if (details) {
    console.log('\nlocale fallback details:')
    for (const locale of locales) {
      if (locale.fallbackCount === 0) continue
      console.log(`\n[${locale.locale}] fallback to en-US (${locale.fallbackCount})`)
      for (const key of locale.fallbackKeys.slice(0, 40)) {
        console.log(`- ${key}`)
      }
      if (locale.fallbackKeys.length > 40) {
        console.log(`- ... and ${locale.fallbackKeys.length - 40} more`)
      }
    }
  }

  const exitCode = getAuditExitCode(report, {
    failOnUnknownStaticKeys,
    failOnUnresolvedDynamicPatterns,
  })

  if (exitCode !== EXIT_CODES.ok) {
    const reasons = []
    if (failOnUnknownStaticKeys && report.unknownStaticKeys.length > 0) {
      reasons.push(`${report.unknownStaticKeys.length} unknown static key(s)`)
    }
    if (failOnUnresolvedDynamicPatterns && report.unresolvedDynamicPatterns.length > 0) {
      reasons.push(`${report.unresolvedDynamicPatterns.length} unresolved dynamic pattern(s)`)
    }
    console.error(`\ni18n audit failed: ${reasons.join(', ')}`)
  }

  return exitCode
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  process.exitCode = main(process.argv.slice(2))
}
