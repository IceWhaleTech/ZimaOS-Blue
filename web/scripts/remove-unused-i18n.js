/**
 * Remove unused i18n keys from en-US.ts (and optionally zh-CN.ts).
 * Reads keys from scripts/unused-i18n-keys.txt.
 * Usage: node scripts/remove-unused-i18n.js
 */
import fs from 'fs'
import path from 'path'
import { fileURLToPath } from 'url'
const __dirname = path.dirname(fileURLToPath(import.meta.url))

const scriptsDir = path.join(__dirname)
const localesDir = path.join(__dirname, '..', 'src', 'i18n', 'locales')
const unusedPath = path.join(scriptsDir, 'unused-i18n-keys.txt')

const unusedKeys = fs.readFileSync(unusedPath, 'utf8')
  .split('\n')
  .map(s => s.trim())
  .filter(Boolean)
const unusedSet = new Set(unusedKeys)

function removeUnusedFromFile(filePath) {
  const content = fs.readFileSync(filePath, 'utf8')
  const lines = content.split('\n')
  const stack = [] // { indent, key }
  const result = []

  for (const line of lines) {
    const trimmed = line.trimStart()
    const indent = line.length - trimmed.length
    while (stack.length > 0 && stack[stack.length - 1].indent >= indent) {
      stack.pop()
    }
    const unquoted = trimmed.match(/^([a-zA-Z][a-zA-Z0-9_]*)\s*:\s*(\{|['"`]|$)/)
    const quoted = trimmed.match(/^['"]([^'"]+)['"]\s*:\s*(\{|['"`]|$)/)
    const keyMatch = unquoted || quoted
    if (keyMatch) {
      const key = unquoted ? unquoted[1] : quoted[1]
      const isObject = keyMatch[2] === '{'
      const prefix = stack.length ? stack.map(s => s.key).join('.') + '.' : ''
      const fullKey = prefix + key
      if (unusedSet.has(fullKey)) {
        continue // skip this line
      }
      if (isObject) {
        stack.push({ indent, key })
      }
    }
    result.push(line)
  }

  fs.writeFileSync(filePath, result.join('\n'), 'utf8')
  console.log('Updated:', filePath)
}

removeUnusedFromFile(path.join(localesDir, 'en-US.ts'))

// zh-CN has its own full definitions; remove same keys to keep in sync
const zhCNPath = path.join(localesDir, 'zh-CN.ts')
if (fs.existsSync(zhCNPath)) {
  removeUnusedFromFile(zhCNPath)
}

console.log('Done. Removed', unusedSet.size, 'unused keys from en-US.ts (and zh-CN.ts).')
