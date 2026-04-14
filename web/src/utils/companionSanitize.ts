const ANSI_ESCAPE_RE = new RegExp(String.raw`\u001B(?:[@-Z\\-_]|\[[0-?]*[ -/]*[@-~])`, 'g')
const CONTROL_CHAR_RE = new RegExp(String.raw`[\x00-\x08\x0B-\x1A\x1C-\x1F\x7F]`, 'g')

export function sanitizeCompanionPreview(input: unknown, maxLen = 160, singleLine = true): string {
  if (typeof input !== 'string') {
    return ''
  }

  let text = input
    .replace(ANSI_ESCAPE_RE, '')
    .replace(CONTROL_CHAR_RE, '')

  text = text
    .replace(/\b(Bearer)\s+[A-Za-z0-9._~+/-]+=*/gi, '$1 [REDACTED]')
    .replace(/\b(sk|rk)-[A-Za-z0-9_-]{12,}\b/g, '$1-[REDACTED]')
    .replace(
      /(["']?(?:api[_-]?key|token|secret|password|access[_-]?token|refresh[_-]?token)["']?\s*[:=]\s*["']?)([^\s,"'}]{6,})/gi,
      '$1[REDACTED]'
    )

  text = singleLine ? text.replace(/\s+/g, ' ').trim() : text.replace(/\r\n/g, '\n').trim()

  if (text.length <= maxLen) {
    return text
  }
  return `${text.slice(0, Math.max(0, maxLen - 3)).trimEnd()}...`
}
