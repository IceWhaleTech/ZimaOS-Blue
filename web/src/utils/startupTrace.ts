const startupTraceStart =
  typeof performance !== 'undefined' ? performance.now() : 0

const reportedMarks = new Set<string>()

function parseBoolFlag(raw: string | null): boolean {
  if (!raw) return false
  switch (raw.trim().toLowerCase()) {
    case '1':
    case 'true':
    case 'yes':
    case 'on':
      return true
    default:
      return false
  }
}

export function isStartupTraceEnabled(): boolean {
  if (typeof window === 'undefined') return false
  const urlFlag = parseBoolFlag(new URLSearchParams(window.location.search).get('startup_trace'))
  const desktopFlag = !!(window as any).__BLUE_STARTUP_TRACE__
  return urlFlag || desktopFlag
}

export function reportStartupMark(
  label: string,
  extra: Record<string, string | number | boolean | null | undefined> = {},
) {
  if (typeof window === 'undefined' || !isStartupTraceEnabled()) return
  if (reportedMarks.has(label)) return
  reportedMarks.add(label)

  const params = new URLSearchParams({
    startup_mark: label,
    startup_ms: (performance.now() - startupTraceStart).toFixed(1),
    path: window.location.pathname,
  })

  for (const [key, value] of Object.entries(extra)) {
    if (value === undefined || value === null) continue
    params.set(key, String(value))
  }

  void fetch(`/api/v1/health?${params.toString()}`, {
    cache: 'no-store',
    credentials: 'same-origin',
  }).catch(() => {})
}
