import { featureIntentTerms } from './featureIntentTerms.generated'

export interface FeatureIntentHint {
  deepResearch: boolean
  agentMode: boolean
}

const DEFINITION_BOUNDARY_MARKERS = [
  '?',
  '？',
  '!',
  '！',
  '\n',
  '。',
  ';',
  '；',
  ' and then ',
  ' then ',
  ', then ',
  ', please ',
  ' but ',
  ' however ',
  '，然后',
  '然后',
  '，请',
  '请',
] as const

const LEADING_COURTESY_PREFIXES = ['please ', 'pls ', '请帮我', '请你', '请'] as const
const WRAPPED_LITERAL_CLOSERS: Record<string, string> = {
  '"': '"',
  "'": "'",
  '`': '`',
  '“': '”',
  '‘': '’',
  '「': '」',
  '『': '』',
  '《': '》',
  '〈': '〉',
  '‹': '›',
}
const EXPLICIT_REQUEST_CUE_SUFFIXES = [
  'use',
  'enable',
  'run in',
  'switch to',
  'turn on',
  'activate',
  'start',
  'apply',
  'keep',
  'enter',
  '开启',
  '打开',
  '启用',
  '切换到',
  '进入',
  '使用',
] as const
const REFERENTIAL_CONTEXT_CUES = [
  'docs',
  'documentation',
  'help text',
  'help copy',
  'ui copy',
  'settings copy',
  'button copy',
  'tooltip',
  'label',
  'menu label',
  'button',
  'title',
  'heading',
  'command',
  'keyword',
  'term',
  'string',
  'word',
  'mention',
  'document',
  'by name',
  'rename',
  'renaming',
  'relabel',
  'relabeling',
  'reword',
  '文档',
  '帮助文案',
  '标签',
  '按钮',
  '按钮文案',
  '命令',
  '关键词',
  '术语',
  '字符串',
  '这个词',
  '这两个词',
  '写上',
  '写进',
  '改成',
  '改为',
  '重命名',
  '提到',
  '提及',
  '文案',
  '说明',
  '设置页文案',
  '标题',
  '移动',
] as const

function isAsciiToken(token: string): boolean {
  for (let i = 0; i < token.length; i++) {
    if (token.charCodeAt(i) > 127) return false
  }
  return true
}

function isAsciiWordChar(ch: string): boolean {
  if (!ch) return false
  const code = ch.charCodeAt(0)
  return (
    (code >= 48 && code <= 57) ||
    (code >= 65 && code <= 90) ||
    (code >= 97 && code <= 122) ||
    code === 95
  )
}

function containsToken(text: string, token: string): boolean {
  if (!token) return false
  const asciiToken = isAsciiToken(token)
  let from = 0

  while (from <= text.length) {
    const idx = text.indexOf(token, from)
    if (idx < 0) return false

    if (!asciiToken) return true

    const before = idx > 0 ? text.slice(idx - 1, idx) : ''
    const afterPos = idx + token.length
    const after = afterPos < text.length ? text.slice(afterPos, afterPos + 1) : ''
    if (!isAsciiWordChar(before) && !isAsciiWordChar(after)) {
      return true
    }

    from = idx + 1
  }
  return false
}

function containsAny(text: string, tokens: string[]): boolean {
  return tokens.some((token) => containsToken(text, token))
}

function trimTrailingWrappedLiteralOpener(text: string): string {
  let current = text.trimEnd()

  while (current) {
    const last = current.slice(-1)
    if (!(last in WRAPPED_LITERAL_CLOSERS)) return current
    current = current.slice(0, -1).trimEnd()
  }

  return current
}

function isWrappedLiteralOccurrence(text: string, start: number, end: number): boolean {
  if (start <= 0 || end >= text.length) return false
  const before = text.slice(start - 1, start)
  const after = text.slice(end, end + 1)
  return WRAPPED_LITERAL_CLOSERS[before] === after
}

function hasExplicitRequestCueBefore(text: string, start: number): boolean {
  const prefix = trimTrailingWrappedLiteralOpener(text.slice(Math.max(0, start - 48), start))
  return EXPLICIT_REQUEST_CUE_SUFFIXES.some((cue) => prefix.endsWith(cue))
}

function hasReferentialContextAround(text: string, start: number, end: number): boolean {
  const prefix = text.slice(Math.max(0, start - 64), start)
  const suffix = text.slice(end, Math.min(text.length, end + 64))
  return REFERENTIAL_CONTEXT_CUES.some((cue) => prefix.includes(cue) || suffix.includes(cue))
}

function containsExplicitToken(text: string, token: string): boolean {
  if (!token) return false
  const asciiToken = isAsciiToken(token)
  let from = 0

  while (from <= text.length) {
    const idx = text.indexOf(token, from)
    if (idx < 0) return false

    const end = idx + token.length
    if (asciiToken) {
      const before = idx > 0 ? text.slice(idx - 1, idx) : ''
      const after = end < text.length ? text.slice(end, end + 1) : ''
      if (isAsciiWordChar(before) || isAsciiWordChar(after)) {
        from = idx + 1
        continue
      }
    }

    const hasRequestCue = hasExplicitRequestCueBefore(text, idx)

    if (isWrappedLiteralOccurrence(text, idx, end) && !hasRequestCue) {
      from = idx + 1
      continue
    }

    if (!hasRequestCue && hasReferentialContextAround(text, idx, end)) {
      from = idx + 1
      continue
    }

    if (!isWrappedLiteralOccurrence(text, idx, end) || hasRequestCue) {
      return true
    }

    from = idx + 1
  }

  return false
}

function containsAnyExplicit(text: string, tokens: string[]): boolean {
  return tokens.some((token) => containsExplicitToken(text, token))
}

function stripLeadingCourtesyPrefixes(text: string): string {
  let current = text.trimStart()

  while (true) {
    const matched = LEADING_COURTESY_PREFIXES.find((prefix) => current.startsWith(prefix))
    if (!matched) return current
    current = current.slice(matched.length).trimStart()
  }
}

function hasDefinitionPrefix(text: string): boolean {
  const normalized = stripLeadingCourtesyPrefixes(text)
  return featureIntentTerms.definitionPrefixes.some((prefix) => normalized.startsWith(prefix))
}

function findDefinitionBoundary(text: string): { index: number; markerLength: number } | null {
  let earliest: { index: number; markerLength: number } | null = null

  for (const marker of DEFINITION_BOUNDARY_MARKERS) {
    const index = text.indexOf(marker)
    if (index < 0) continue
    if (!earliest || index < earliest.index) {
      earliest = { index, markerLength: marker.length }
    }
  }

  return earliest
}

function extractDefinitionTail(text: string): string | null {
  const normalized = stripLeadingCourtesyPrefixes(text)
  if (!hasDefinitionPrefix(text)) return null

  const mentionsFeature =
    containsAny(normalized, featureIntentTerms.deepResearchExplicit) ||
    containsAny(normalized, featureIntentTerms.agentModeExplicit)
  if (!mentionsFeature) return null

  const boundary = findDefinitionBoundary(normalized)
  if (!boundary) return ''

  return normalized.slice(boundary.index + boundary.markerLength).trim()
}

function resolveIntentText(text: string): string {
  let current = text.trim()

  while (current) {
    const tail = extractDefinitionTail(current)
    if (tail === null) return current
    if (!tail) return ''
    current = tail
  }

  return ''
}

export function classifyFeatureIntent(message: string): FeatureIntentHint {
  const text = resolveIntentText(message.toLowerCase().trim())
  if (!text) return { deepResearch: false, agentMode: false }

  const deepExplicit = containsAnyExplicit(text, featureIntentTerms.deepResearchExplicit)
  const deepComposite =
    containsAny(text, featureIntentTerms.deepResearchActions) &&
    containsAny(text, featureIntentTerms.deepResearchTargets)
  const deepNegated = containsAny(text, featureIntentTerms.deepResearchNegations)
  const deepResearch = (deepExplicit || deepComposite) && !deepNegated

  const agentExplicit = containsAnyExplicit(text, featureIntentTerms.agentModeExplicit)
  const agentComposite =
    containsAny(text, featureIntentTerms.agentModeActions) &&
    containsAny(text, featureIntentTerms.agentModeTargets)
  const agentNegated = containsAny(text, featureIntentTerms.agentModeNegations)
  const agentMode = (agentExplicit || agentComposite) && !agentNegated

  return { deepResearch, agentMode }
}
