export interface FrontmatterEntry {
  key: string
  value: string
}

export interface FrontmatterParseResult {
  entries: FrontmatterEntry[]
  body: string
  raw: string
  hasFrontmatter: boolean
}

const FRONTMATTER_START_RE = /^---\s*\r?\n/
const FRONTMATTER_END_RE = /\r?\n---\s*(?:\r?\n|$)/
const FRONTMATTER_KEY_RE = /^([A-Za-z0-9_.-]+)\s*:\s*(.*)$/

function cleanValue(raw: string): string {
  let value = raw.trim()
  const inlineComment = value.indexOf(' #')
  if (inlineComment >= 0) {
    value = value.slice(0, inlineComment).trim()
  }

  if (
    (value.startsWith('"') && value.endsWith('"')) ||
    (value.startsWith("'") && value.endsWith("'"))
  ) {
    value = value.slice(1, -1)
  }

  if (value.startsWith('[') && value.endsWith(']')) {
    const inner = value.slice(1, -1).trim()
    if (!inner) return ''
    return inner
      .split(',')
      .map((part) => cleanValue(part))
      .filter(Boolean)
      .join(', ')
  }

  return value
}

function appendValue(target: Map<string, string[]>, key: string, value: string) {
  const current = target.get(key) || []
  if (value) current.push(value)
  target.set(key, current)
}

function parseEntries(rawFrontmatter: string): FrontmatterEntry[] {
  const values = new Map<string, string[]>()
  const order: string[] = []
  let activeKey: string | null = null

  for (const line of rawFrontmatter.replace(/\r\n/g, '\n').split('\n')) {
    const trimmed = line.trim()
    if (!trimmed || trimmed.startsWith('#')) continue

    const keyMatch = line.match(FRONTMATTER_KEY_RE)
    if (keyMatch) {
      const key = keyMatch[1]!.trim()
      const value = cleanValue(keyMatch[2] || '')
      if (!values.has(key)) {
        values.set(key, [])
        order.push(key)
      }
      if (value) {
        values.set(key, [value])
        activeKey = null
      } else {
        activeKey = key
      }
      continue
    }

    if (!activeKey) continue

    const listMatch = line.match(/^\s*-\s*(.+)\s*$/)
    if (listMatch) {
      appendValue(values, activeKey, cleanValue(listMatch[1]!))
      continue
    }

    if (/^\s+/.test(line)) {
      appendValue(values, activeKey, cleanValue(trimmed))
      continue
    }

    activeKey = null
  }

  return order.map((key) => {
    const list = (values.get(key) || []).filter(Boolean)
    return {
      key,
      value: list.length ? Array.from(new Set(list)).join(', ') : '-',
    }
  })
}

export function parseFrontmatter(content: string): FrontmatterParseResult {
  if (!content) {
    return { entries: [], body: '', raw: '', hasFrontmatter: false }
  }

  const start = content.match(FRONTMATTER_START_RE)
  if (!start) {
    return { entries: [], body: content, raw: '', hasFrontmatter: false }
  }

  const rest = content.slice(start[0].length)
  const end = rest.match(FRONTMATTER_END_RE)
  if (!end || end.index === undefined) {
    return { entries: [], body: content, raw: '', hasFrontmatter: false }
  }

  const raw = rest.slice(0, end.index)
  const body = rest.slice(end.index + end[0].length).trimStart()

  return {
    entries: parseEntries(raw),
    body,
    raw,
    hasFrontmatter: true,
  }
}
