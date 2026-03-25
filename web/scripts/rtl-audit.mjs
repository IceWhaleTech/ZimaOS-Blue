#!/usr/bin/env node

import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const rawArgs = process.argv.slice(2)
const args = new Set(rawArgs)
const details = args.has('--details')
const fileFilters = rawArgs.filter((arg) => !arg.startsWith('--'))

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const WEB_ROOT = path.resolve(__dirname, '..')
const SRC_DIR = path.join(WEB_ROOT, 'src')
const FILE_EXTENSIONS = new Set(['.vue', '.ts', '.css'])
const IGNORED_SEGMENTS = ['/__tests__/', '/i18n/locales/']
const IGNORE_LINE_MARKER = 'rtl-audit-ignore-line'
const IGNORE_NEXT_LINE_MARKER = 'rtl-audit-ignore-next-line'

const RULES = [
  {
    name: 'css-physical-properties',
    description: 'CSS physical directional properties',
    regex:
      /\b(?:left|right|margin-left|margin-right|padding-left|padding-right|border-left|border-right)\s*:/g,
    ignoreVueScriptBlocks: true,
    skipExtensions: new Set(['.ts']),
  },
  {
    name: 'css-physical-alignment',
    description: 'Physical text alignment',
    regex: /\btext-align\s*:\s*(?:left|right)\b/g,
    ignoreVueScriptBlocks: true,
    skipExtensions: new Set(['.ts']),
  },
  {
    name: 'tailwind-physical-spacing',
    description: 'Tailwind physical spacing utilities',
    regex:
      /(^|[\s"'`])(?:[A-Za-z0-9_-]+:)*(?:ml|mr|pl|pr|left|right)-[\w[\]()./%-]+(?=$|[\s"'`])/gm,
    ignoreLocaleCodes: true,
  },
  {
    name: 'tailwind-physical-text',
    description: 'Tailwind physical text utilities',
    regex: /(^|[\s"'`])(?:[A-Za-z0-9_-]+:)*text-(?:left|right)(?=$|[\s"'`])/gm,
  },
  {
    name: 'tailwind-physical-shape',
    description: 'Tailwind physical border radius and border utilities',
    regex:
      /(^|[\s"'`])(?:[A-Za-z0-9_-]+:)*(?:rounded-[lr](?:-[\w[\]()./%-]+)?|border-[lr](?:-[\w[\]()./%-]+)?)(?=$|[\s"'`])/gm,
  },
]

function collectFiles(dir) {
  const out = []
  const stack = [dir]

  while (stack.length > 0) {
    const current = stack.pop()
    if (!current) continue
    const entries = fs.readdirSync(current, { withFileTypes: true })

    for (const entry of entries) {
      const fullPath = path.join(current, entry.name)
      if (entry.isDirectory()) {
        stack.push(fullPath)
        continue
      }
      if (FILE_EXTENSIONS.has(path.extname(entry.name))) {
        out.push(fullPath)
      }
    }
  }

  return out.sort()
}

function shouldIgnore(filePath) {
  const normalized = filePath.split(path.sep).join('/')
  return IGNORED_SEGMENTS.some((segment) => normalized.includes(segment))
}

function matchesFileFilter(relativePath) {
  if (fileFilters.length === 0) return true

  const normalizedPath = relativePath.split(path.sep).join('/')
  return fileFilters.some((filter) => normalizedPath.includes(filter.replace(/^\.\//, '')))
}

function lineNumberForIndex(source, index) {
  let line = 1
  for (let i = 0; i < index; i += 1) {
    if (source[i] === '\n') line += 1
  }
  return line
}

function collectVueScriptRanges(source) {
  const scriptRanges = []
  const scriptRegex = /<script\b[^>]*>[\s\S]*?<\/script>/gi

  for (const match of source.matchAll(scriptRegex)) {
    const start = match.index ?? 0
    scriptRanges.push({ start, end: start + match[0].length })
  }

  return scriptRanges
}

function isIndexWithinRanges(index, ranges) {
  return ranges.some((range) => index >= range.start && index < range.end)
}

function normalizeMatchedToken(match) {
  return match
    .trim()
    .replace(/^[\s"'`]+/, '')
    .replace(/[\s"'`]+$/, '')
}

function isLocaleCodeToken(token) {
  return /^[a-z]{2,3}-(?:[A-Z]{2}|[A-Z][a-z]{3}|\d{3})$/.test(token)
}

function shouldIgnoreLine(lineNumber, lines) {
  const currentLine = lines[lineNumber - 1] ?? ''
  const previousLine = lines[lineNumber - 2] ?? ''

  return currentLine.includes(IGNORE_LINE_MARKER) || previousLine.includes(IGNORE_NEXT_LINE_MARKER)
}

const findings = []

for (const filePath of collectFiles(SRC_DIR)) {
  if (shouldIgnore(filePath)) continue

  const source = fs.readFileSync(filePath, 'utf8')
  const relativePath = path.relative(WEB_ROOT, filePath)
  if (!matchesFileFilter(relativePath)) continue
  const extension = path.extname(filePath)
  const vueScriptRanges = extension === '.vue' ? collectVueScriptRanges(source) : []
  const sourceLines = source.split('\n')

  for (const rule of RULES) {
    if (rule.skipExtensions?.has(extension)) continue

    for (const match of source.matchAll(rule.regex)) {
      const matchIndex = match.index ?? 0
      if (rule.ignoreVueScriptBlocks && isIndexWithinRanges(matchIndex, vueScriptRanges)) continue

      const line = lineNumberForIndex(source, matchIndex)
      if (shouldIgnoreLine(line, sourceLines)) continue

      const normalizedToken = normalizeMatchedToken(match[0])
      if (rule.ignoreLocaleCodes && isLocaleCodeToken(normalizedToken)) continue

      findings.push({
        file: relativePath,
        line,
        rule: rule.name,
        description: rule.description,
        match: normalizedToken,
      })
    }
  }
}

const fileCounts = new Map()
for (const finding of findings) {
  fileCounts.set(finding.file, (fileCounts.get(finding.file) || 0) + 1)
}

const topFiles = [...fileCounts.entries()]
  .sort((left, right) => right[1] - left[1] || left[0].localeCompare(right[0]))
  .slice(0, 20)

console.log(
  `RTL audit found ${findings.length} physical-direction matches across ${fileCounts.size} files.`
)

if (topFiles.length > 0) {
  console.log('')
  console.log('Top files:')
  for (const [file, count] of topFiles) {
    console.log(`- ${file}: ${count}`)
  }
}

if (details && findings.length > 0) {
  console.log('')
  console.log('Detailed findings:')
  for (const finding of findings) {
    console.log(
      `- ${finding.file}:${finding.line} [${finding.rule}] ${finding.match} (${finding.description})`
    )
  }
}

process.exitCode = findings.length > 0 ? 1 : 0
