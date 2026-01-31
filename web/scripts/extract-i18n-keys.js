import fs from 'fs'
import path from 'path'
import { fileURLToPath } from 'url'
const __dirname = path.dirname(fileURLToPath(import.meta.url))

function walk(dir, exts, exclude, files = []) {
  const list = fs.readdirSync(dir)
  for (const f of list) {
    const p = path.join(dir, f)
    const stat = fs.statSync(p)
    if (stat.isDirectory()) {
      if (f !== 'node_modules' && f !== 'locales' && f !== '__tests__') walk(p, exts, exclude, files)
    } else if (exts.some(e => f.endsWith(e)) && !exclude.some(x => p.includes(x))) {
      files.push(p)
    }
  }
  return files
}

const srcDir = path.join(__dirname, '..', 'src')
const files = walk(srcDir, ['.vue', '.ts', '.tsx'], ['auto-imports.d.ts', 'i18n/locales'])
const used = new Set()
const re = /t\s*\(\s*['"]([a-zA-Z][a-zA-Z0-9_.]*)['"]/g
for (const file of files) {
  const s = fs.readFileSync(file, 'utf8')
  let m
  while ((m = re.exec(s)) !== null) used.add(m[1])
}

/** Build all key paths from en-US.ts by parsing lines (indent + key: value or key: {) */
function extractEnKeys(content) {
  const keys = []
  const stack = []
  const lines = content.split('\n')
  for (const line of lines) {
    const trimmed = line.trimStart()
    const indent = line.length - trimmed.length
    const keyMatch = trimmed.match(/^([a-zA-Z][a-zA-Z0-9]*)\s*:\s*(\{|['"`]|$)/)
    if (!keyMatch) continue
    const key = keyMatch[1]
    const isObject = keyMatch[2] === '{'
    while (stack.length > 0 && stack[stack.length - 1].indent >= indent) stack.pop()
    const prefix = stack.length ? stack.map(s => s.key).join('.') + '.' : ''
    const fullKey = prefix + key
    if (isObject) {
      stack.push({ indent, key })
    } else {
      keys.push(fullKey)
    }
  }
  return keys
}

const enPath = path.join(__dirname, '..', 'src', 'i18n', 'locales', 'en-US.ts')
const enContent = fs.readFileSync(enPath, 'utf8')
const enKeys = extractEnKeys(enContent)

const usedArr = Array.from(used).sort()
const unused = enKeys.filter(k => {
  if (usedArr.includes(k)) return false
  const prefix = k + '.'
  const usedAsPrefix = usedArr.some(u => u === k || u.startsWith(prefix))
  return !usedAsPrefix
})
const missing = usedArr.filter(u => !enKeys.includes(u))

console.log('=== Used keys (count):', usedArr.length)
console.log('=== En-US keys (count):', enKeys.length)
console.log('=== Unused keys in en-US (total):', unused.length)
console.log('=== Missing from en-US (total):', missing.length)
if (unused.length) {
  console.log('\n--- Unused (first 120):')
  unused.slice(0, 120).forEach(k => console.log(k))
  if (unused.length > 120) console.log('... and', unused.length - 120, 'more')
}
if (missing.length) {
  console.log('\n--- Missing (first 30):')
  missing.slice(0, 30).forEach(k => console.log(k))
}

// Write unused list to file for review (do NOT blindly remove - many are used via t(`key.${var}`))
fs.writeFileSync(
  path.join(__dirname, 'unused-i18n-keys.txt'),
  unused.join('\n'),
  'utf8'
)
console.log('\nUnused keys list written to scripts/unused-i18n-keys.txt')
console.log('Note: Do not remove keys without auditing - many are used dynamically (e.g. errors.http*, settings.tab.*, system.*, service.*, companion.*).')
