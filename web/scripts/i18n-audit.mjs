#!/usr/bin/env node

import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const args = new Set(process.argv.slice(2))
const details = args.has('--details')
const json = args.has('--json')

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const WEB_ROOT = path.resolve(__dirname, '..')
const SRC_DIR = path.join(WEB_ROOT, 'src')
const LOCALES_DIR = path.join(SRC_DIR, 'i18n', 'locales')

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

function parseImports(rawSource) {
  const importMatcher = /^\s*import\s+([A-Za-z_$][\w$]*)\s+from\s+['"]\.\/([A-Za-z0-9-]+)['"]\s*;?\s*$/gm
  const imports = []
  let match
  while ((match = importMatcher.exec(rawSource)) !== null) {
    imports.push({ localName: match[1], importedLocale: match[2] })
  }
  return imports
}

function loadLocaleObjectByCode(localeCode, cache = new Map(), loading = new Set()) {
  if (cache.has(localeCode)) {
    return cache.get(localeCode)
  }
  if (loading.has(localeCode)) {
    throw new Error(`Circular locale import detected: ${localeCode}`)
  }

  loading.add(localeCode)
  const filePath = path.join(LOCALES_DIR, `${localeCode}.ts`)
  if (!fs.existsSync(filePath)) {
    throw new Error(`Locale file not found: ${filePath}`)
  }

  const raw = fs.readFileSync(filePath, 'utf8')
  const imports = parseImports(raw)
  const importValues = []
  const importNames = []

  for (const item of imports) {
    importNames.push(item.localName)
    importValues.push(loadLocaleObjectByCode(item.importedLocale, cache, loading))
  }

  const withoutImports = raw.replace(/^\s*import\s+[A-Za-z_$][\w$]*\s+from\s+['"]\.\/[A-Za-z0-9-]+['"]\s*;?\s*$/gm, '')
  const executable = withoutImports.replace(/^\s*export\s+default\s*/m, 'return ')
  if (!executable.includes('return ')) {
    throw new Error(`Cannot parse locale file: ${filePath}`)
  }

  const localeValue = new Function(...importNames, executable)(...importValues)
  cache.set(localeCode, localeValue)
  loading.delete(localeCode)
  return localeValue
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

function main() {
  const localeFiles = fs
    .readdirSync(LOCALES_DIR)
    .filter((name) => name.endsWith('.ts'))
    .sort()

  if (!localeFiles.includes('en-US.ts')) {
    throw new Error('en-US.ts not found in locale directory')
  }

  const localeCache = new Map()
  const enUSObject = loadLocaleObjectByCode('en-US', localeCache)
  const enUSMap = flattenStringLeaves(enUSObject)
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
    const localeObject = loadLocaleObjectByCode(locale, localeCache)
    const localeMap = flattenStringLeaves(localeObject)
    const missing = []
    const fallbackToEnglish = []

    for (const key of usedKeysArray) {
      const enValue = enUSMap.get(key)
      const localeValue = localeMap.get(key)
      if (localeValue === undefined) {
        missing.push(key)
        continue
      }
      if (locale !== 'en-US' && enValue !== undefined && localeValue === enValue) {
        fallbackToEnglish.push(key)
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
    })),
  }

  if (json) {
    process.stdout.write(`${JSON.stringify(report, null, 2)}\n`)
    return
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
}

main()
