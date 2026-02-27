export function sanitizeCompanionPreview(input: unknown, maxLen = 160, singleLine = true): string {
  if (typeof input !== 'string') {
    return ''
  }

  let text = input
    .replace(/\x1B(?:[@-Z\\-_]|\[[0-?]*[ -/]*[@-~])/g, '')
    .replace(/[\u0000-\u0008\u000B-\u001A\u001C-\u001F\u007F]/g, '')

  text = text
    .replace(/\b(Bearer)\s+[A-Za-z0-9._~+\/-]+=*/gi, '$1 [REDACTED]')
    .replace(/\b(sk|rk)-[A-Za-z0-9_-]{12,}\b/g, '$1-[REDACTED]')
    .replace(/(["']?(?:api[_-]?key|token|secret|password|access[_-]?token|refresh[_-]?token)["']?\s*[:=]\s*["']?)([^\s,"'}]{6,})/gi, '$1[REDACTED]')

  text = singleLine ? text.replace(/\s+/g, ' ').trim() : text.replace(/\r\n/g, '\n').trim()

  if (text.length <= maxLen) {
    return text
  }
  return `${text.slice(0, Math.max(0, maxLen - 3)).trimEnd()}...`
}
