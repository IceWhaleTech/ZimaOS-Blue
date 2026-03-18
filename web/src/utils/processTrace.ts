export type ProcessTraceStatus = 'info' | 'pending' | 'active' | 'success' | 'error'
export type ProcessTraceSource = 'client' | 'server'
export type ProcessTraceCategory =
  | 'summary'
  | 'lifecycle'
  | 'retry'
  | 'recovery'
  | 'confirmation'
  | 'audio'
  | 'tts'

export interface ProcessTraceItem {
  id: string
  source: ProcessTraceSource
  event: string
  category: ProcessTraceCategory
  label: string
  status: ProcessTraceStatus
  timestamp: number
  detail?: string
  command?: string
  progress?: number
  metadata?: Record<string, string | number | boolean | undefined>
}

interface CreateProcessTraceItemInput
  extends Omit<ProcessTraceItem, 'id' | 'timestamp'> {
  id?: string
  timestamp?: number
}

export function createProcessTraceItem(
  input: CreateProcessTraceItemInput
): ProcessTraceItem {
  const timestamp = input.timestamp ?? Date.now()
  return {
    ...input,
    id: input.id || `${input.source}:${input.event}:${timestamp}:${Math.random().toString(36).slice(2, 8)}`,
    timestamp,
  }
}

export function cloneProcessTrace(items?: ProcessTraceItem[]): ProcessTraceItem[] {
  if (!items || items.length === 0) return []
  return items.map((item) => ({
    ...item,
    metadata: item.metadata ? { ...item.metadata } : undefined,
  }))
}
