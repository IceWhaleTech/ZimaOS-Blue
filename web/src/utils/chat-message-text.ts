// Strip markdown heading from first line if present.
// Keeps previous behavior: if first line starts with '#' after leading spaces/tabs,
// remove that line and trim leading whitespace from the remaining content.
export function stripFirstLineHeading(content: string | null | undefined): string {
  if (typeof content !== 'string') return ''
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
