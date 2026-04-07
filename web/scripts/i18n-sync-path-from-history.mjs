#!/usr/bin/env node

import fs from 'node:fs'
import path from 'node:path'
import { execFileSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import ts from 'typescript'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const WEB_ROOT = path.resolve(__dirname, '..')
const REPO_ROOT = path.resolve(WEB_ROOT, '..')
const LOCALES_DIR = path.join(WEB_ROOT, 'src', 'i18n', 'locales')

function parseArgs(argv) {
  const args = {
    from: 'HEAD~1',
    base: 'WORKTREE',
    paths: [],
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

    if (arg === '--path') {
      const value = argv[index + 1]
      if (!value) throw new Error('--path requires a dot-separated locale path')
      args.paths.push(value)
      index += 1
      continue
    }

    if (arg === '--help' || arg === '-h') {
      console.log(
        'Usage: node scripts/i18n-sync-path-from-history.mjs --path settings.agentReflection [--from HEAD~1] [--base HEAD]'
      )
      process.exit(0)
    }

    throw new Error(`unsupported argument: ${arg}`)
  }

  if (args.paths.length === 0) {
    throw new Error('At least one --path is required')
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

function parseSource(sourceText, filePath) {
  return ts.createSourceFile(filePath, sourceText, ts.ScriptTarget.Latest, true, ts.ScriptKind.TS)
}

function getPropertyName(node) {
  if (ts.isIdentifier(node) || ts.isStringLiteral(node) || ts.isNumericLiteral(node)) {
    return node.text
  }
  return null
}

function findRootObjectLiteral(sourceFile) {
  for (const statement of sourceFile.statements) {
    if (!ts.isExportAssignment(statement)) continue

    const expression = statement.expression
    if (ts.isCallExpression(expression)) {
      for (let index = expression.arguments.length - 1; index >= 0; index -= 1) {
        const arg = expression.arguments[index]
        if (ts.isObjectLiteralExpression(arg)) {
          return arg
        }
      }
    }

    if (ts.isObjectLiteralExpression(expression)) {
      return expression
    }
  }

  throw new Error(`Could not find exported locale object in ${sourceFile.fileName}`)
}

function findPropertyInfo(sourceText, filePath, pathSegments) {
  const sourceFile = parseSource(sourceText, filePath)
  let currentObject = findRootObjectLiteral(sourceFile)
  let parentObject = null
  let propertyNode = null

  for (const segment of pathSegments) {
    parentObject = currentObject
    propertyNode = currentObject.properties.find((property) => {
      if (
        !ts.isPropertyAssignment(property) &&
        !ts.isShorthandPropertyAssignment(property) &&
        !ts.isMethodDeclaration(property)
      ) {
        return false
      }
      return getPropertyName(property.name) === segment
    })

    if (!propertyNode) {
      return {
        sourceFile,
        parentObject,
        propertyNode: null,
      }
    }

    if (!ts.isPropertyAssignment(propertyNode)) {
      throw new Error(`Unsupported property syntax at ${pathSegments.join('.')} in ${filePath}`)
    }

    if (segment !== pathSegments[pathSegments.length - 1]) {
      if (!ts.isObjectLiteralExpression(propertyNode.initializer)) {
        throw new Error(`Path ${pathSegments.join('.')} is not backed by nested object literals`)
      }
      currentObject = propertyNode.initializer
    }
  }

  return {
    sourceFile,
    parentObject,
    propertyNode,
  }
}

function getClosingIndentation(sourceText, objectLiteral) {
  const closeBraceIndex = objectLiteral.end - 1
  const lineStart = sourceText.lastIndexOf('\n', closeBraceIndex - 1) + 1
  return sourceText.slice(lineStart, closeBraceIndex)
}

function insertProperty(currentText, parentObject, propertyText) {
  const closeBraceIndex = parentObject.end - 1
  const closingLineStart = currentText.lastIndexOf('\n', closeBraceIndex - 1) + 1
  const closingIndent = currentText.slice(closingLineStart, closeBraceIndex)
  const childIndent = `${closingIndent}  `
  const trimmedPropertyText = propertyText.trim()
  const hasProperties = parentObject.properties.length > 0
  const insertText = hasProperties
    ? `${childIndent}${trimmedPropertyText},\n`
    : `\n${childIndent}${trimmedPropertyText},\n${closingIndent}`

  return `${currentText.slice(0, closingLineStart)}${insertText}${currentText.slice(closingLineStart)}`
}

function syncPath(currentText, historyText, filePath, localePath) {
  const pathSegments = localePath.split('.').filter(Boolean)
  if (pathSegments.length === 0) {
    throw new Error(`Invalid empty path: ${localePath}`)
  }

  const historyInfo = findPropertyInfo(historyText, `${filePath}@history`, pathSegments)
  if (!historyInfo.propertyNode || !historyInfo.parentObject) {
    throw new Error(`Could not find history path "${localePath}" in ${filePath}`)
  }

  const historyPropertyText = historyText.slice(
    historyInfo.propertyNode.getStart(historyInfo.sourceFile),
    historyInfo.propertyNode.end
  )

  const currentInfo = findPropertyInfo(currentText, filePath, pathSegments)
  if (!currentInfo.parentObject) {
    throw new Error(`Could not find parent path for "${localePath}" in ${filePath}`)
  }

  if (currentInfo.propertyNode) {
    const start = currentInfo.propertyNode.getStart(currentInfo.sourceFile)
    const end = currentInfo.propertyNode.end
    return `${currentText.slice(0, start)}${historyPropertyText}${currentText.slice(end)}`
  }

  return insertProperty(currentText, currentInfo.parentObject, historyPropertyText)
}

function main() {
  const { from, base, paths } = parseArgs(process.argv.slice(2))
  const localeFiles = fs
    .readdirSync(LOCALES_DIR)
    .filter((fileName) => fileName.endsWith('.ts'))
    .sort()

  for (const fileName of localeFiles) {
    const filePath = path.join(LOCALES_DIR, fileName)
    const repoPath = path.relative(REPO_ROOT, filePath).split(path.sep).join('/')
    const historyText = readGitFile(from, repoPath)
    let nextText = readBaseFile(base, repoPath, filePath)

    for (const localePath of paths) {
      nextText = syncPath(nextText, historyText, filePath, localePath)
    }

    fs.writeFileSync(filePath, nextText, 'utf8')
  }

  console.log(
    `synced ${paths.join(', ')} from ${from} into ${base} for ${localeFiles.length} locale files`
  )
}

try {
  main()
} catch (error) {
  console.error(error instanceof Error ? error.message : String(error))
  process.exit(1)
}
