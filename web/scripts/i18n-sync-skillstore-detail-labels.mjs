#!/usr/bin/env node

import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const LOCALES_DIR = path.resolve(__dirname, '..', 'src', 'i18n', 'locales')

function findBlock(source, marker) {
  const markerIndex = source.indexOf(marker)
  if (markerIndex < 0) {
    throw new Error(`Could not find block marker: ${marker.trim()}`)
  }

  const objectStart = source.indexOf('{', markerIndex)
  if (objectStart < 0) {
    throw new Error(`Could not find opening brace for ${marker.trim()}`)
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
      }
      continue
    }

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

  if (end < 0) {
    throw new Error(`Could not find closing brace for ${marker.trim()}`)
  }

  return {
    start: markerIndex,
    end,
    text: source.slice(markerIndex, end),
  }
}

function readDetailsLiteral(commonBlock) {
  const match = commonBlock.match(/\n    details:\s*([^,\n]+),/)
  if (!match) {
    throw new Error('Could not find common.details')
  }
  return match[1].trim()
}

function upsertProperty(block, indent, key, literal) {
  const propertyPattern = new RegExp(`\\n${indent}${key}:\\s*([^,\\n]+),`)
  if (propertyPattern.test(block)) {
    return block.replace(propertyPattern, `\n${indent}${key}: ${literal},`)
  }

  const openingBraceIndex = block.indexOf('{')
  if (openingBraceIndex < 0) {
    throw new Error(`Could not insert ${key}`)
  }
  return `${block.slice(0, openingBraceIndex + 1)}\n${indent}${key}: ${literal},${block.slice(
    openingBraceIndex + 1
  )}`
}

const localeFiles = fs
  .readdirSync(LOCALES_DIR)
  .filter((fileName) => fileName.endsWith('.ts'))
  .sort()

for (const fileName of localeFiles) {
  const filePath = path.join(LOCALES_DIR, fileName)
  let source = fs.readFileSync(filePath, 'utf8')

  const commonBlock = findBlock(source, '\n  common:')
  const detailsLiteral = readDetailsLiteral(commonBlock.text)
  const skillStoreBlock = findBlock(source, '\n  skillStore:')
  const actionsBlock = findBlock(skillStoreBlock.text, '\n    actions:')
  const detailBlock = findBlock(skillStoreBlock.text, '\n    detail:')

  const nextActionsBlock = upsertProperty(actionsBlock.text, '      ', 'details', detailsLiteral)
  let nextDetailBlock = upsertProperty(detailBlock.text, '      ', 'title', detailsLiteral)
  const detailSectionsBlock = findBlock(nextDetailBlock, '\n      sections:')
  const nextDetailSectionsBlock = upsertProperty(
    detailSectionsBlock.text,
    '        ',
    'details',
    detailsLiteral
  )
  nextDetailBlock = `${nextDetailBlock.slice(
    0,
    detailSectionsBlock.start
  )}${nextDetailSectionsBlock}${nextDetailBlock.slice(detailSectionsBlock.end)}`

  let nextSkillStoreBlock = `${skillStoreBlock.text.slice(
    0,
    actionsBlock.start
  )}${nextActionsBlock}${skillStoreBlock.text.slice(actionsBlock.end)}`

  const refreshedDetailBlock = findBlock(nextSkillStoreBlock, '\n    detail:')
  nextSkillStoreBlock = `${nextSkillStoreBlock.slice(
    0,
    refreshedDetailBlock.start
  )}${nextDetailBlock}${nextSkillStoreBlock.slice(refreshedDetailBlock.end)}`

  source = `${source.slice(0, skillStoreBlock.start)}${nextSkillStoreBlock}${source.slice(
    skillStoreBlock.end
  )}`
  fs.writeFileSync(filePath, source, 'utf8')
}

console.log(`synced skillStore details labels in ${localeFiles.length} locale files`)
