/**
 * Clipboard data parsing utilities for form filler
 */

/**
 * Calculate probability that a string is a URL (0-1)
 */
export function getUrlProbability(str: string): number {
  const s = str.trim()
  if (!s) return 0

  let score = 0

  // Strong indicators
  if (/^https?:\/\//i.test(s)) score += 0.6
  if (/^(www\.)/i.test(s)) score += 0.4

  // Domain-like patterns
  if (/\.(com|org|net|io|cn|top|xyz|app|dev|ai|co)($|\/)/i.test(s)) score += 0.2
  if (/:\d{2,5}(\/|$)/.test(s)) score += 0.1 // Port number
  if (/\/api\//i.test(s)) score += 0.1
  if (/\/v\d+\//i.test(s)) score += 0.1 // API version path

  // Negative indicators
  if (/^(sk-|pk-|api[_-]?key|bearer|token)/i.test(s)) score -= 0.5
  if (/^[a-zA-Z0-9_-]{30,}$/.test(s) && !s.includes('.')) score -= 0.3 // Long string without dots

  return Math.max(0, Math.min(1, score))
}

/**
 * Calculate probability that a string is a token/API key (0-1)
 */
export function getTokenProbability(str: string): number {
  const s = str.trim()
  if (!s) return 0

  let score = 0

  // Strong prefix indicators
  if (/^sk-/i.test(s)) score += 0.7 // OpenAI secret key
  if (/^pk-/i.test(s)) score += 0.6 // Public key
  if (/^(xoxb-|xoxp-|xoxa-)/i.test(s)) score += 0.7 // Slack tokens
  if (/^ghp_/i.test(s)) score += 0.7 // GitHub personal token
  if (/^gho_/i.test(s)) score += 0.7 // GitHub OAuth token
  if (/^Bearer\s+/i.test(s)) score += 0.5

  // Pattern indicators
  if (/^[a-zA-Z0-9_-]{20,}$/.test(s)) score += 0.4 // Long alphanumeric
  if (/^[a-f0-9]{32,}$/i.test(s)) score += 0.5 // Hex string (MD5/SHA-like)
  if (/^[A-Za-z0-9+/=]{20,}$/.test(s) && s.length % 4 === 0) score += 0.3 // Base64-like

  // Length-based scoring
  if (s.length >= 20 && s.length <= 200) score += 0.2
  if (s.length > 200) score -= 0.2 // Too long, probably not a token

  // Negative indicators
  if (/^https?:\/\//i.test(s)) score -= 0.6 // URLs are not tokens
  if (/\s/.test(s)) score -= 0.3 // Tokens usually don't have spaces
  if (/\.(com|org|net|io|cn)/i.test(s)) score -= 0.4 // Domain-like

  return Math.max(0, Math.min(1, score))
}

/**
 * Parse clipboard data - try to extract key-value pairs
 */
export function parseClipboardData(data: string): Record<string, string> {
  const result: Record<string, string> = {}
  if (!data.trim()) return result

  // First, try to parse as single-line multiple key-value pairs
  // Format: key1 = "value1" key2 = "value2" OR key1 = value1 key2 = value2
  const singleLineMatches = data.matchAll(/(\w+)\s*=\s*"([^"]+)"/g)
  for (const match of singleLineMatches) {
    const key = match[1]?.trim().toLowerCase() || ''
    const value = match[2]?.trim() || ''
    if (key) {
      result[key] = value
    }
  }

  // If we found matches with quoted values, return
  if (Object.keys(result).length > 0) {
    return result
  }

  // Try different parsing strategies line by line
  const lines = data
    .split('\n')
    .map((line) => line.trim())
    .filter((line) => line)

  // Special case: single line with space-separated URL + token
  // Format: "https://example.com/ sk-xxxxx" or "sk-xxxxx https://example.com/"
  if (lines.length === 1) {
    const firstLine = lines[0] || ''
    const parts = firstLine.split(/\s+/).filter((p) => p)
    if (parts.length === 2) {
      const part0 = parts[0] || ''
      const part1 = parts[1] || ''
      const part1UrlProb = getUrlProbability(part0)
      const part1TokenProb = getTokenProbability(part0)
      const part2UrlProb = getUrlProbability(part1)
      const part2TokenProb = getTokenProbability(part1)

      const threshold = 0.4
      let urlPart: string | null = null
      let tokenPart: string | null = null

      if (
        part1UrlProb >= threshold &&
        part2TokenProb >= threshold &&
        part1UrlProb > part1TokenProb
      ) {
        urlPart = part0 ?? null
        tokenPart = part1 ?? null
      } else if (
        part2UrlProb >= threshold &&
        part1TokenProb >= threshold &&
        part2UrlProb > part2TokenProb
      ) {
        urlPart = part1 ?? null
        tokenPart = part0 ?? null
      }

      if (urlPart && tokenPart) {
        result['base_url'] = urlPart
        result['api_key'] = tokenPart
        return result
      }
    }
  }

  // Special case: detect URL + token/key pattern (two lines)
  if (lines.length === 2) {
    const line0 = lines[0] || ''
    const line1 = lines[1] || ''
    const line1UrlProb = getUrlProbability(line0)
    const line1TokenProb = getTokenProbability(line0)
    const line2UrlProb = getUrlProbability(line1)
    const line2TokenProb = getTokenProbability(line1)

    // Check if one line is likely URL and other is likely token
    const threshold = 0.4
    let urlLine: string | null = null
    let tokenLine: string | null = null

    if (line1UrlProb >= threshold && line2TokenProb >= threshold && line1UrlProb > line1TokenProb) {
      urlLine = line0 ?? null
      tokenLine = line1 ?? null
    } else if (
      line2UrlProb >= threshold &&
      line1TokenProb >= threshold &&
      line2UrlProb > line2TokenProb
    ) {
      urlLine = line1 ?? null
      tokenLine = line0 ?? null
    }

    if (urlLine && tokenLine) {
      result['base_url'] = urlLine
      result['api_key'] = tokenLine
      return result
    }
  }

  for (const line of lines) {
    // Try "key = value" or "key=value" format first
    // Key must look like a variable name (no colons, no spaces before =)
    let match = line.match(/^([\w-]+)\s*=\s*(.+)$/)
    if (match && match[1] && match[2]) {
      const key = match[1].trim().toLowerCase()
      const value = match[2].trim().replace(/^["']|["']$/g, '') // Remove quotes
      result[key] = value
      continue
    }

    // Try "key: value" format (supports Chinese colon too)
    // But exclude URLs (don't match if value starts with //)
    match = line.match(/^([^:：]+)[：:]\s*(?!\/\/)(.+)$/)
    if (match && match[1] && match[2]) {
      const key = match[1].trim().toLowerCase()
      const value = match[2].trim().replace(/^["']|["']$/g, '') // Remove quotes
      result[key] = value
      continue
    }

    // Try "key\tvalue" (tab-separated) format
    match = line.match(/^([^\t]+)\t(.+)$/)
    if (match && match[1] && match[2]) {
      const key = match[1].trim().toLowerCase()
      const value = match[2].trim().replace(/^["']|["']$/g, '') // Remove quotes
      result[key] = value
      continue
    }

    // Try "key value" (space-separated, key is first word, value is rest)
    // Only if key looks like a field name (alphanumeric with underscores/hyphens)
    match = line.match(/^([\w-]+)\s+(.+)$/)
    if (match && match[1] && match[2]) {
      const key = match[1].trim().toLowerCase()
      const value = match[2].trim().replace(/^["']|["']$/g, '') // Remove quotes
      result[key] = value
      continue
    }
  }

  return result
}

/**
 * Parse clipboard data as positional fields (plain values without keys).
 * Supports newline-separated, whitespace-separated, and mixed separators.
 * Returns an array of non-empty trimmed tokens when no key-value pairs are detected.
 * Used for sequential filling: field 1 → input 1, field 2 → input 2, etc.
 */
export function parseClipboardFields(data: string): string[] {
  if (!data.trim()) return []

  // Check if data has explicit key-value indicators (=, :, tab-separated key-value)
  // If so, defer to parseClipboardData
  const lines = data
    .split('\n')
    .map((l) => l.trim())
    .filter((l) => l.length > 0)
  const hasKvIndicators = lines.some(
    (line) =>
      /^[\w-]+\s*=\s*.+$/.test(line) || // key=value
      /^[^:：]+[：:]\s*(?!\/\/).+$/.test(line) // key: value (not URL)
  )
  // Tab kv: every line must have exactly one tab with a word-like key
  const hasTabKv = lines.length > 0 && lines.every((line) => /^[\w-]+\t[^\t]+$/.test(line))
  // Also check single-line quoted key-value: key = "value"
  const hasQuotedKv = /\w+\s*=\s*"[^"]+"/.test(data)

  if (hasKvIndicators || hasQuotedKv || hasTabKv) return []

  // Check URL+token two-value pattern (handled by parseClipboardData)
  const allTokens: string[] = []
  for (const line of lines) {
    const parts = line.split(/\s+/)
    for (const part of parts) {
      if (part) allTokens.push(part)
    }
  }

  if (allTokens.length === 2) {
    const p0Url = getUrlProbability(allTokens[0]!)
    const p0Tok = getTokenProbability(allTokens[0]!)
    const p1Url = getUrlProbability(allTokens[1]!)
    const p1Tok = getTokenProbability(allTokens[1]!)
    const threshold = 0.4
    if ((p0Url >= threshold && p1Tok >= threshold) || (p1Url >= threshold && p0Tok >= threshold)) {
      return [] // URL+token pair — let parseClipboardData handle it
    }
  }

  // Need at least 2 tokens for positional mode
  if (allTokens.length < 2) return []

  return allTokens
}
