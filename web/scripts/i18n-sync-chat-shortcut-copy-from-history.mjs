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

const SHORTCUT_BLOCKS = [
  {
    key: 'analyzeReport',
    marker: '    analyzeReportShortcutTitle:',
    blockStart: '    analyzeReportHoverState:',
    blockEnd: '    ralphLoopHoverDescription:',
  },
  {
    key: 'uiReview',
    marker: '    uiReviewShortcutTitle:',
    blockStart: '    uiReviewHoverState:',
    blockEnd: '    showShortcutDetails:',
  },
]

function readGitFile(revision, repoPath) {
  return execFileSync('git', ['show', `${revision}:${repoPath}`], {
    cwd: REPO_ROOT,
    encoding: 'utf8',
  })
}

function extractIndentedBlock(source, startMarker, endMarker) {
  const start = source.indexOf(startMarker)
  if (start < 0) {
    throw new Error(`Could not find ${startMarker.trim()}`)
  }
  const end = source.indexOf(endMarker, start)
  if (end < 0) {
    throw new Error(`Could not find ${endMarker.trim()} after ${startMarker.trim()}`)
  }
  return source.slice(start, end)
}

const localeFiles = fs
  .readdirSync(LOCALES_DIR)
  .filter((fileName) => fileName.endsWith('.ts'))
  .sort()

for (const fileName of localeFiles) {
  const filePath = path.join(LOCALES_DIR, fileName)
  const repoPath = path.relative(REPO_ROOT, filePath).split(path.sep).join('/')
  const historySource = readGitFile('HEAD~1', repoPath)
  let currentSource = fs.readFileSync(filePath, 'utf8')

  for (const block of SHORTCUT_BLOCKS) {
    if (currentSource.includes(block.blockStart)) continue

    const markerIndex = currentSource.indexOf(block.marker)
    if (markerIndex < 0) {
      throw new Error(`Could not find ${block.marker.trim()} in ${fileName}`)
    }

    const insertAt = currentSource.indexOf('\n', markerIndex)
    if (insertAt < 0) {
      throw new Error(`Could not find insertion point after ${block.marker.trim()} in ${fileName}`)
    }

    const historyBlock = extractIndentedBlock(historySource, block.blockStart, block.blockEnd)
    currentSource = `${currentSource.slice(0, insertAt + 1)}${historyBlock}${currentSource.slice(
      insertAt + 1
    )}`
  }

  fs.writeFileSync(filePath, currentSource, 'utf8')
}

console.log(`synced legacy chat shortcut copy in ${localeFiles.length} locale files`)
