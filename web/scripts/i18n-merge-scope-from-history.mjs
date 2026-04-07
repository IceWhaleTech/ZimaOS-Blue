#!/usr/bin/env node

import fs from 'node:fs'
import path from 'node:path'
import { execFileSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const WEB_ROOT = path.resolve(__dirname, '..')
const REPO_ROOT = path.resolve(WEB_ROOT, '..')
const LOCALES_DIR = path.join(WEB_ROOT, 'src', 'i18n', 'locales')

function parseArgs(argv) {
  const args = {
    from: 'HEAD~1',
    base: 'WORKTREE',
    scopes: [],
  }

  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index]

    if (arg === '--from') {
      const value = argv[index + 1]
      if (!value) throw new Error('--from requires a git revision')
      args.from = value
      index += 1
      continue
    }

    if (arg === '--base') {
      const value = argv[index + 1]
      if (!value) throw new Error('--base requires a git revision')
      args.base = value
      index += 1
      continue
    }

    if (arg === '--scope') {
      const value = argv[index + 1]
      if (!value) throw new Error('--scope requires a top-level locale scope')
      args.scopes.push(value)
      index += 1
      continue
    }

    if (arg === '--help' || arg === '-h') {
      console.log(
        'Usage: node scripts/i18n-merge-scope-from-history.mjs --scope skillStore [--from HEAD~1] [--base HEAD]'
      )
      process.exit(0)
    }

    throw new Error(`unsupported argument: ${arg}`)
  }

  if (args.scopes.length === 0) {
    throw new Error('At least one --scope is required')
  }

  return args
}

function readGitFile(revision, repoPath) {
  return execFileSync('git', ['show', `${revision}:${repoPath}`], {
    cwd: REPO_ROOT,
    encoding: 'utf8',
  })
}

function readBaseFile(base, repoPath, filePath) {
  if (base === 'WORKTREE') {
    return fs.readFileSync(filePath, 'utf8')
  }

  return readGitFile(base, repoPath)
}

function findScopeBlock(source, scope) {
  const marker = `\n  ${scope}:`
  const markerIndex = source.indexOf(marker)
  if (markerIndex < 0) {
    throw new Error(`Could not find top-level scope "${scope}"`)
  }

  const start = markerIndex + 1
  const objectStart = source.indexOf('{', start)
  if (objectStart < 0) {
    throw new Error(`Could not find opening brace for scope "${scope}"`)
  }

  let depth = 0
  let quote = null
  let escaped = false
  let templateExpressionDepth = 0
  let end = -1

  for (let index = objectStart; index < source.length; index += 1) {
    const char = source[index]
    const next = source[index + 1]

    if (quote) {
      if (escaped) {
        escaped = false
        continue
      }
      if (char === '\\') {
        escaped = true
        continue
      }
      if (quote === '`' && char === '$' && next === '{') {
        templateExpressionDepth += 1
        depth += 1
        index += 1
        continue
      }
      if (char === quote && !(quote === '`' && templateExpressionDepth > 0)) {
        quote = null
        continue
      }
    } else {
      if (char === "'" || char === '"' || char === '`') {
        quote = char
        continue
      }
      if (char === '{') {
        depth += 1
        continue
      }
      if (char === '}') {
        depth -= 1
        if (templateExpressionDepth > 0) {
          templateExpressionDepth -= 1
        } else if (depth === 0) {
          end = index + 1
          break
        }
      }
    }
  }

  if (end < 0) {
    throw new Error(`Could not find closing brace for scope "${scope}"`)
  }

  if (source[end] === ',') {
    end += 1
  }

  return {
    start,
    end,
    text: source.slice(start, end),
  }
}

function main() {
  const { from, base, scopes } = parseArgs(process.argv.slice(2))
  const localeFiles = fs
    .readdirSync(LOCALES_DIR)
    .filter((fileName) => fileName.endsWith('.ts'))
    .sort()

  for (const fileName of localeFiles) {
    const filePath = path.join(LOCALES_DIR, fileName)
    const repoPath = path.relative(REPO_ROOT, filePath).split(path.sep).join('/')
    let merged = readBaseFile(base, repoPath, filePath)

    for (const scope of scopes) {
      const incoming = findScopeBlock(readGitFile(from, repoPath), scope)
      const current = findScopeBlock(merged, scope)
      merged = `${merged.slice(0, current.start)}${incoming.text}${merged.slice(current.end)}`
    }

    fs.writeFileSync(filePath, merged, 'utf8')
  }

  console.log(
    `merged ${scopes.join(', ')} from ${from} into ${base} for ${localeFiles.length} locale files`
  )
}

try {
  main()
} catch (error) {
  console.error(error instanceof Error ? error.message : String(error))
  process.exit(1)
}
