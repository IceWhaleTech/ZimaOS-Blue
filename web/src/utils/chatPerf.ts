type ChatPerfAggregate = {
  count: number
  totalMs: number
  maxMs: number
}

type ChatPerfRegistry = {
  counters: Map<string, number>
  timings: Map<string, ChatPerfAggregate>
}

declare global {
  // eslint-disable-next-line no-var
  var __zimaChatPerfRegistry: ChatPerfRegistry | undefined
}

function isDevPerfEnabled(): boolean {
  return import.meta.env.DEV && typeof performance !== 'undefined'
}

function getRegistry(): ChatPerfRegistry | null {
  if (!isDevPerfEnabled()) return null
  if (!globalThis.__zimaChatPerfRegistry) {
    globalThis.__zimaChatPerfRegistry = {
      counters: new Map<string, number>(),
      timings: new Map<string, ChatPerfAggregate>(),
    }
  }
  return globalThis.__zimaChatPerfRegistry
}

export function recordChatPerfCount(name: string, delta = 1): void {
  const registry = getRegistry()
  if (!registry) return
  registry.counters.set(name, (registry.counters.get(name) || 0) + delta)
}

function updateTiming(name: string, durationMs: number): void {
  const registry = getRegistry()
  if (!registry) return

  const current = registry.timings.get(name) || { count: 0, totalMs: 0, maxMs: 0 }
  current.count += 1
  current.totalMs += durationMs
  current.maxMs = Math.max(current.maxMs, durationMs)
  registry.timings.set(name, current)
}

export function measureChatPerf<T>(name: string, fn: () => T): T {
  if (!isDevPerfEnabled()) return fn()

  const markStart = `${name}:start:${performance.now()}`
  const markEnd = `${name}:end:${performance.now()}`
  const measureName = `${name}:measure:${performance.now()}`
  performance.mark(markStart)
  const startedAt = performance.now()

  try {
    return fn()
  } finally {
    const durationMs = performance.now() - startedAt
    updateTiming(name, durationMs)
    performance.mark(markEnd)
    try {
      performance.measure(measureName, markStart, markEnd)
    } catch {
      // Ignore duplicate/unsupported measure failures in dev tooling.
    }
    performance.clearMarks(markStart)
    performance.clearMarks(markEnd)
    performance.clearMeasures(measureName)
  }
}
