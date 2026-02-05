import fs from 'fs'
import path from 'path'
import { fileURLToPath } from 'url'
const __dirname = path.dirname(fileURLToPath(import.meta.url))

/**
 * Extract all leaf key paths from a locale .ts file by parsing indentation + `key: value` syntax.
 * This is intentionally simple (fast + no TS parser). It works for our locale files which are
 * plain object literals with consistent indentation.
 */
function extractLeafKeys(content) {
  const keys = []
  const stack = [] // { indent, key }
  const lines = content.split('\n')

  for (const line of lines) {
    const trimmed = line.trimStart()
    const indent = line.length - trimmed.length

    // ignore spreads and comments
    if (trimmed.startsWith('...') || trimmed.startsWith('//')) continue

    // match: foo: {   OR   foo: 'bar'   OR   foo: `bar`   OR   foo: "bar"
    // we intentionally ignore quoted keys like 'foo-bar': because those are rare in our locales
    const keyMatch = trimmed.match(/^([a-zA-Z][a-zA-Z0-9]*)\s*:\s*(\{|['"`])/)
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

function main() {
  const locale = process.argv[2]
  if (!locale) {
    console.error('Usage: node scripts/compare-locale-keys.js <locale>')
    console.error('Example: node scripts/compare-locale-keys.js zh-CN')
    process.exit(1)
  }

  const localesDir = path.join(__dirname, '..', 'src', 'i18n', 'locales')
  const enPath = path.join(localesDir, 'en-US.ts')
  const targetPath = path.join(localesDir, `${locale}.ts`)

  if (!fs.existsSync(targetPath)) {
    console.error('Locale file not found:', targetPath)
    process.exit(1)
  }

  const enKeys = new Set(extractLeafKeys(fs.readFileSync(enPath, 'utf8')))
  const targetKeys = new Set(extractLeafKeys(fs.readFileSync(targetPath, 'utf8')))

  const missing = Array.from(enKeys).filter(k => !targetKeys.has(k)).sort()
  const extra = Array.from(targetKeys).filter(k => !enKeys.has(k)).sort()

  console.log(`=== en-US leaf keys: ${enKeys.size}`)
  console.log(`=== ${locale} leaf keys: ${targetKeys.size}`)
  console.log(`=== Missing in ${locale}: ${missing.length}`)
  console.log(`=== Extra in ${locale}: ${extra.length}`)

  if (missing.length) {
    console.log('\n--- Missing (first 200):')
    missing.slice(0, 200).forEach(k => console.log(k))
    if (missing.length > 200) console.log('... and', missing.length - 200, 'more')
  }

  if (extra.length) {
    console.log('\n--- Extra (first 100):')
    extra.slice(0, 100).forEach(k => console.log(k))
    if (extra.length > 100) console.log('... and', extra.length - 100, 'more')
  }
}

main()

