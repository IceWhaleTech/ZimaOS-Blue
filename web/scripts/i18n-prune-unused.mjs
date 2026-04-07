#!/usr/bin/env node

import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import ts from 'typescript'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const WEB_ROOT = path.resolve(__dirname, '..')
const SRC_DIR = path.join(WEB_ROOT, 'src')
const LOCALES_DIR = path.join(SRC_DIR, 'i18n', 'locales')
const REPO_ROOT = path.resolve(WEB_ROOT, '..')
const SERVER_DIR = path.join(REPO_ROOT, 'server')

function parseArgs(argv) {
  const args = { apply: false, details: false }
  for (const arg of argv) {
    if (arg === '--apply') {
      args.apply = true
      continue
    }
    if (arg === '--details') {
      args.details = true
      continue
    }
    if (arg === '--help' || arg === '-h') {
      // eslint-disable-next-line no-console
      console.log('Usage: node scripts/i18n-prune-unused.mjs [--apply] [--details]')
      process.exit(0)
    }
    throw new Error(`unsupported argument: ${arg}`)
  }
  return args
}

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

function isTestFile(filePath) {
  const normalized = filePath.split(path.sep).join('/')
  if (normalized.includes('/__tests__/')) return true
  if (normalized.endsWith('.test.ts')) return true
  if (normalized.endsWith('.test.js')) return true
  if (normalized.endsWith('.spec.ts')) return true
  if (normalized.endsWith('.spec.js')) return true
  return false
}

function isExcludedSourceFile(filePath) {
  const normalized = filePath.split(path.sep).join('/')
  if (normalized.includes('/i18n/locales/')) return true
  if (isTestFile(filePath)) return true
  return false
}

function isPlainObjectLiteral(value) {
  return value && typeof value === 'object' && !Array.isArray(value)
}

function getPropertyName(node) {
  if (!node) return null
  if (ts.isIdentifier(node)) return node.text
  if (ts.isStringLiteral(node)) return node.text
  if (ts.isNumericLiteral(node)) return node.text
  return null
}

function findLocaleMessagesObject(sourceFile) {
  let found = null

  function visit(node) {
    if (found) return
    if (ts.isCallExpression(node)) {
      const callee = node.expression
      const calleeName = ts.isIdentifier(callee)
        ? callee.text
        : ts.isPropertyAccessExpression(callee)
          ? callee.name.text
          : null
      if (calleeName === 'mergeHarnessLocale' && node.arguments.length >= 2) {
        const arg = node.arguments[1]
        if (ts.isObjectLiteralExpression(arg)) {
          found = arg
          return
        }
      }
    }
    ts.forEachChild(node, visit)
  }

  visit(sourceFile)
  return found
}

function collectStringLeafPaths(node, prefix = '', out = new Set()) {
  if (!ts.isObjectLiteralExpression(node)) return out

  for (const prop of node.properties) {
    if (!ts.isPropertyAssignment(prop)) continue
    const name = getPropertyName(prop.name)
    if (!name) continue
    const fullPath = prefix ? `${prefix}.${name}` : name
    const init = prop.initializer

    if (ts.isStringLiteral(init) || ts.isNoSubstitutionTemplateLiteral(init)) {
      out.add(fullPath)
      continue
    }

    if (ts.isObjectLiteralExpression(init)) {
      collectStringLeafPaths(init, fullPath, out)
    }
  }

  return out
}

function collectUsedKeysFromCalls(content) {
  const staticKeys = new Set()
  const patterns = []
  const unresolvedTemplates = []

  const callMatcher = /\b(\$?t|\$?te)\s*\(/g
  let match
  while ((match = callMatcher.exec(content)) !== null) {
    const argStart = match.index + match[0].length
    // super small parser: grab until first ',' or ')', respecting quotes/brackets.
    let i = argStart
    let depth = 0
    let quote = null
    let escaped = false
    const expressionStart = argStart
    while (i < content.length) {
      const ch = content[i]
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
        if (ch === quote) quote = null
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
        if (ch === ')') break
      }
      if (depth === 0 && (ch === ',' || ch === ')')) break
      i += 1
    }

    const rawArg = content.slice(expressionStart, i).trim()
    if (!rawArg) continue

    const head = rawArg[0]
    const tail = rawArg[rawArg.length - 1]
    const isQuoted =
      (head === "'" && tail === "'") ||
      (head === '"' && tail === '"') ||
      (head === '`' && tail === '`')

    if (isQuoted) {
      const inner = rawArg.slice(1, -1)
      if (head === '`' && inner.includes('${')) {
        const parts = inner.split(/\$\{[^}]+\}/g)
        const prefix = parts[0] || ''
        const suffix = parts.length > 1 ? parts[parts.length - 1] || '' : ''
        if (!prefix && !suffix) {
          // Can't safely resolve a `${key}`-style template for pruning.
          unresolvedTemplates.push(inner)
          continue
        }
        patterns.push({ prefix, suffix })
        continue
      }
      staticKeys.add(inner)
      continue
    }

    // concat patterns like 'foo.' + bar + '.baz'
    if (rawArg.includes('+')) {
      const literalMatcher = /(['"`])((?:(?!\1)[\s\S])*)\1/g
      const literals = []
      let m
      while ((m = literalMatcher.exec(rawArg)) !== null) literals.push(m[2])
      if (literals.length > 0) {
        const prefix = literals[0] || ''
        const suffix = literals.length > 1 ? literals[literals.length - 1] || '' : ''
        if (prefix || suffix) patterns.push({ prefix, suffix })
      }
    }
  }

  return { staticKeys, patterns, unresolvedTemplates }
}

function collectI18nKeyStringLiterals(content) {
  const out = new Set()
  const matcher = /(['"`])([A-Za-z0-9][A-Za-z0-9_.-]*\.[A-Za-z0-9_.-]+)\1/g
  let match
  while ((match = matcher.exec(content)) !== null) {
    out.add(match[2])
  }
  return out
}

function pruneLocaleFile(filePath, usedKeys) {
  const raw = fs.readFileSync(filePath, 'utf8')
  const sourceFile = ts.createSourceFile(
    filePath,
    raw,
    ts.ScriptTarget.ES2022,
    true,
    ts.ScriptKind.TS
  )
  const messagesObject = findLocaleMessagesObject(sourceFile)
  if (!messagesObject) {
    throw new Error(`Unable to locate mergeHarnessLocale(..., { ... }) in ${filePath}`)
  }

  function pruneObject(node, prefix = '') {
    const nextProps = []

    for (const prop of node.properties) {
      if (!ts.isPropertyAssignment(prop)) {
        nextProps.push(prop)
        continue
      }
      const name = getPropertyName(prop.name)
      if (!name) {
        nextProps.push(prop)
        continue
      }
      const fullPath = prefix ? `${prefix}.${name}` : name
      const init = prop.initializer

      if (ts.isObjectLiteralExpression(init)) {
        const { node: prunedChild, kept } = pruneObject(init, fullPath)
        if (!kept) {
          continue
        }
        nextProps.push(ts.factory.updatePropertyAssignment(prop, prop.name, prunedChild))
        continue
      }

      if (ts.isStringLiteral(init) || ts.isNoSubstitutionTemplateLiteral(init)) {
        if (usedKeys.has(fullPath)) {
          nextProps.push(prop)
        }
        continue
      }

      nextProps.push(prop)
    }

    const kept = nextProps.length > 0
    return { node: ts.factory.updateObjectLiteralExpression(node, nextProps), kept }
  }

  const pruned = pruneObject(messagesObject, '')
  const printer = ts.createPrinter({ newLine: ts.NewLineKind.LineFeed, removeComments: false })
  const printed = printer.printNode(ts.EmitHint.Unspecified, pruned.node, sourceFile)

  const start = messagesObject.getStart(sourceFile)
  const end = messagesObject.getEnd()
  const nextRaw = `${raw.slice(0, start)}${printed}${raw.slice(end)}`

  if (nextRaw !== raw) {
    fs.writeFileSync(filePath, nextRaw, 'utf8')
    return true
  }

  return false
}

function main() {
  const { apply, details } = parseArgs(process.argv.slice(2))

  if (apply) {
    throw new Error(
      'Automatic locale pruning is disabled. This codebase uses dynamic i18n scopes and runtime backfills, so candidate keys must be reviewed manually.'
    )
  }

  const enUSPath = path.join(LOCALES_DIR, 'en-US.ts')
  const enUSRaw = fs.readFileSync(enUSPath, 'utf8')
  const enUSSourceFile = ts.createSourceFile(
    enUSPath,
    enUSRaw,
    ts.ScriptTarget.ES2022,
    true,
    ts.ScriptKind.TS
  )
  const enUSMessages = findLocaleMessagesObject(enUSSourceFile)
  if (!enUSMessages) {
    throw new Error('en-US.ts does not contain mergeHarnessLocale(..., { ... })')
  }

  const allKeys = collectStringLeafPaths(enUSMessages)
  const allKeysArray = [...allKeys].sort()
  const allKeySet = new Set(allKeysArray)

  const webSourceFiles = collectFiles(SRC_DIR, ['.vue', '.ts', '.js']).filter(
    (filePath) => !isExcludedSourceFile(filePath)
  )
  const serverSourceFiles = fs.existsSync(SERVER_DIR) ? collectFiles(SERVER_DIR, ['.go']) : []
  const scannedSourceFiles = [...webSourceFiles, ...serverSourceFiles]

  const usedKeys = new Set()
  const unresolvedPatterns = []
  const unresolvedTemplates = []

  for (const filePath of scannedSourceFiles) {
    const content = fs.readFileSync(filePath, 'utf8')

    if (filePath.startsWith(SRC_DIR)) {
      const usage = collectUsedKeysFromCalls(content)
      for (const key of usage.staticKeys) {
        if (allKeySet.has(key)) usedKeys.add(key)
      }
      for (const template of usage.unresolvedTemplates) {
        unresolvedTemplates.push({ file: filePath, template })
      }

      for (const { prefix, suffix } of usage.patterns) {
        const regex = new RegExp(
          `^${prefix.replace(/[.*+?^${}()|[\\]\\\\]/g, '\\\\$&')}.*${suffix.replace(/[.*+?^${}()|[\\]\\\\]/g, '\\\\$&')}$`
        )
        const matches = allKeysArray.filter((key) => regex.test(key))
        if (matches.length === 0) {
          unresolvedPatterns.push({ file: filePath, template: `${prefix}...${suffix}` })
          continue
        }
        for (const key of matches) usedKeys.add(key)
      }
    }

    const literals = collectI18nKeyStringLiterals(content)
    for (const key of literals) {
      if (allKeySet.has(key)) usedKeys.add(key)
    }
  }

  const unusedKeys = allKeysArray.filter((key) => !usedKeys.has(key))

  // eslint-disable-next-line no-console
  console.log('web locale prune report')
  // eslint-disable-next-line no-console
  console.log(`- locale string keys in en-US.ts: ${allKeysArray.length}`)
  // eslint-disable-next-line no-console
  console.log(`- production web files scanned: ${webSourceFiles.length}`)
  // eslint-disable-next-line no-console
  console.log(`- server files scanned: ${serverSourceFiles.length}`)
  // eslint-disable-next-line no-console
  console.log(`- used keys found (conservative): ${usedKeys.size}`)
  // eslint-disable-next-line no-console
  console.log(`- unused keys (candidates to prune): ${unusedKeys.length}`)
  // eslint-disable-next-line no-console
  console.log(`- unresolved dynamic patterns: ${unresolvedPatterns.length}`)
  // eslint-disable-next-line no-console
  console.log(`- unresolved template keys: ${unresolvedTemplates.length}`)

  if (details && unresolvedPatterns.length > 0) {
    // eslint-disable-next-line no-console
    console.log('\nunresolved dynamic patterns:')
    for (const item of unresolvedPatterns.slice(0, 20)) {
      // eslint-disable-next-line no-console
      console.log(`- ${item.template} (${path.relative(WEB_ROOT, item.file)})`)
    }
    if (unresolvedPatterns.length > 20) {
      // eslint-disable-next-line no-console
      console.log(`- ... and ${unresolvedPatterns.length - 20} more`)
    }
  }

  if (details && unresolvedTemplates.length > 0) {
    // eslint-disable-next-line no-console
    console.log('\nunresolved template literals:')
    for (const item of unresolvedTemplates.slice(0, 20)) {
      // eslint-disable-next-line no-console
      console.log(`- \`${item.template}\` (${path.relative(WEB_ROOT, item.file)})`)
    }
    if (unresolvedTemplates.length > 20) {
      // eslint-disable-next-line no-console
      console.log(`- ... and ${unresolvedTemplates.length - 20} more`)
    }
  }

  if (details && unusedKeys.length > 0) {
    // eslint-disable-next-line no-console
    console.log('\nfirst 200 unused keys:')
    for (const key of unusedKeys.slice(0, 200)) {
      // eslint-disable-next-line no-console
      console.log(`- ${key}`)
    }
    if (unusedKeys.length > 200) {
      // eslint-disable-next-line no-console
      console.log(`- ... and ${unusedKeys.length - 200} more`)
    }
  }

  return
}

try {
  main()
} catch (error) {
  // eslint-disable-next-line no-console
  console.error(error instanceof Error ? error.message : String(error))
  process.exit(1)
}
