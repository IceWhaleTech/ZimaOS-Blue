// Strip markdown heading from first line if present.
// Keeps previous behavior: if first line starts with '#' after leading spaces/tabs,
// remove that line and trim leading whitespace from the remaining content.
export function stripFirstLineHeading(content: string): string {
  const len = content.length
  if (len === 0) return content

  const newlineIndex = content.indexOf('\n')
  const lineEnd = newlineIndex >= 0 ? newlineIndex : len

  let i = 0
  while (i < lineEnd) {
    const code = content.charCodeAt(i)
    if (code === 32 || code === 9 || code === 13) {
      i++
      continue
    }

    if (code !== 35) return content // '#'

    if (newlineIndex < 0) return ''

    let start = newlineIndex + 1
    while (start < len) {
      const c = content.charCodeAt(start)
      if (c === 32 || c === 9 || c === 10 || c === 13) {
        start++
        continue
      }
      break
    }
    return content.slice(start)
  }

  return content
}

const ZH_TOOL_FALLBACK_PREFIX = '工具执行已完成，但最终总结生成失败。以下是从工具结果自动提炼的安全摘要：'
const ZH_TOOL_FALLBACK_SUFFIX = '原始 stdout/stderr/error 字段已隐藏以保护安全。如需我重试完整总结，请回复“重试总结”。'

const EN_TOOL_FALLBACK_PREFIX = 'Tool execution completed, but final summary generation failed. Here is a safe fallback summary extracted from tool results:'
const EN_TOOL_FALLBACK_SUFFIX = 'Raw stdout/stderr/error fields remain hidden for safety. Ask me to retry summarizing for a full report.'

function unwrapToolFallbackSummary(content: string, prefix: string, suffix: string): string | null {
  const trimmed = content.trim()
  if (!trimmed.startsWith(prefix)) return null

  let body = trimmed.slice(prefix.length).trim()
  if (body.endsWith(suffix)) {
    body = body.slice(0, -suffix.length).trim()
  }
  return body || null
}

export function normalizeToolFallbackSummaryText(content: string): string {
  if (!content) return content

  const zh = unwrapToolFallbackSummary(content, ZH_TOOL_FALLBACK_PREFIX, ZH_TOOL_FALLBACK_SUFFIX)
  if (zh) return zh

  const en = unwrapToolFallbackSummary(content, EN_TOOL_FALLBACK_PREFIX, EN_TOOL_FALLBACK_SUFFIX)
  if (en) return en

  return content
}
