type TranslateFn = (key: string, fallback?: string) => string

type UnknownLabelMode = 'badge' | 'label'

const WARNING_LABEL_KEYS: Record<string, string> = {
  login_wall: 'toolWarnings.loginWall',
  challenge: 'toolWarnings.challenge',
  browser_required: 'toolWarnings.browserRequired',
}

const WARNING_LABEL_FALLBACKS: Record<string, string> = {
  login_wall: 'Login wall',
  challenge: 'Challenge',
  browser_required: 'Browser required',
}

export function formatToolWarningCodeLabel(
  code: string | undefined,
  t: TranslateFn,
  unknownLabelMode: UnknownLabelMode = 'badge'
): string {
  const normalized = (code || '').trim()
  if (!normalized) return ''
  const key = WARNING_LABEL_KEYS[normalized]
  if (!key) {
    return unknownLabelMode === 'label'
      ? normalized.replace(/_/g, ' ')
      : `warning_code=${normalized}`
  }
  return t(key, WARNING_LABEL_FALLBACKS[normalized] || normalized)
}
