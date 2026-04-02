import type { HarnessRunEvent, HarnessRunStatus, HarnessRunSummary } from '@/api/harness'

export interface HarnessRunTreeNode {
  run: HarnessRunSummary
  children: HarnessRunTreeNode[]
  parentID?: string
  hasChildren: boolean
  isDetached: boolean
  isCoordinator: boolean
}

export interface HarnessRunTreeSummary {
  coordinatorCount: number
  workerCount: number
  detachedCount: number
  runningCount: number
  completedCount: number
  failedCount: number
  waitingInputCount: number
  firstFailedRunID?: string
  firstWaitingInputRunID?: string
  firstDetachedRunID?: string
}

export interface HarnessRunChildVisibility {
  shouldCollapse: boolean
  visibleChildren: HarnessRunTreeNode[]
  hiddenChildren: HarnessRunTreeNode[]
  representativeLabels: string[]
}

const TERMINAL_RUN_STATUSES = new Set<HarnessRunStatus>([
  'completed',
  'failed',
  'cancelled',
  'aborted',
])

const FAILURE_RUN_STATUSES = new Set<HarnessRunStatus>(['failed', 'cancelled', 'aborted'])

function parseTime(value?: string | null): number {
  const parsed = Date.parse(String(value || '').trim())
  return Number.isFinite(parsed) ? parsed : 0
}

function attemptIndex(value?: number | null): number {
  return Number.isFinite(Number(value)) ? Number(value) : 0
}

export function isHarnessRunTerminalStatus(status?: string | null): boolean {
  return TERMINAL_RUN_STATUSES.has(String(status || '').trim() as HarnessRunStatus)
}

export function isHarnessRunActiveStatus(status?: string | null): boolean {
  const normalized = String(status || '').trim()
  return normalized.length > 0 && !isHarnessRunTerminalStatus(normalized)
}

function humanizeEnum(value?: string | null): string {
  const normalized = String(value || '').trim()
  if (!normalized) return ''
  return normalized.replace(/_/g, ' ').replace(/\b\w/g, (char) => char.toUpperCase())
}

function compactPreview(value?: string | null): string {
  const normalized = String(value || '').trim()
  if (!normalized) return ''
  return normalized.length > 180 ? `${normalized.slice(0, 177)}...` : normalized
}

export function compareHarnessRuns(left: HarnessRunSummary, right: HarnessRunSummary): number {
  const createdDiff = parseTime(left.created_at) - parseTime(right.created_at)
  if (createdDiff !== 0) return createdDiff

  const attemptDiff = attemptIndex(left.attempt_index) - attemptIndex(right.attempt_index)
  if (attemptDiff !== 0) return attemptDiff

  const updatedDiff = parseTime(left.updated_at) - parseTime(right.updated_at)
  if (updatedDiff !== 0) return updatedDiff

  const leftActive = isHarnessRunActiveStatus(left.status)
  const rightActive = isHarnessRunActiveStatus(right.status)
  if (leftActive !== rightActive) return leftActive ? -1 : 1

  return left.id.localeCompare(right.id)
}

export function buildHarnessRunTree(runs: HarnessRunSummary[]): HarnessRunTreeNode[] {
  const sortedRuns = [...runs].sort(compareHarnessRuns)
  const nodeByID = new Map<string, HarnessRunTreeNode>()

  for (const run of sortedRuns) {
    nodeByID.set(run.id, {
      run,
      children: [],
      parentID: run.parent_run_id,
      hasChildren: false,
      isDetached: false,
      isCoordinator: false,
    })
  }

  const roots: HarnessRunTreeNode[] = []

  for (const run of sortedRuns) {
    const node = nodeByID.get(run.id)
    if (!node) continue

    const parentID = String(run.parent_run_id || '').trim()
    if (!parentID) {
      roots.push(node)
      continue
    }

    const parentNode = nodeByID.get(parentID)
    if (!parentNode) {
      node.isDetached = true
      roots.push(node)
      continue
    }

    parentNode.children.push(node)
  }

  const finalize = (node: HarnessRunTreeNode) => {
    node.children.sort((left, right) => compareHarnessRuns(left.run, right.run))
    for (const child of node.children) finalize(child)
    node.hasChildren = node.children.length > 0
    node.isCoordinator = node.hasChildren
  }

  for (const node of roots) finalize(node)

  roots.sort((left, right) => compareHarnessRuns(left.run, right.run))

  return roots
}

export function flattenHarnessRunTree(nodes: HarnessRunTreeNode[]): HarnessRunTreeNode[] {
  const flattened: HarnessRunTreeNode[] = []

  const visit = (node: HarnessRunTreeNode) => {
    flattened.push(node)
    for (const child of node.children) visit(child)
  }

  for (const node of nodes) visit(node)

  return flattened
}

export function summarizeHarnessRunTree(nodes: HarnessRunTreeNode[]): HarnessRunTreeSummary {
  const flattened = flattenHarnessRunTree(nodes)

  let runningCount = 0
  let completedCount = 0
  let failedCount = 0
  let waitingInputCount = 0

  for (const node of flattened) {
    const normalizedStatus = String(node.run.status || '').trim() as HarnessRunStatus
    if (normalizedStatus === 'waiting_input') {
      waitingInputCount += 1
      continue
    }
    if (normalizedStatus === 'completed') {
      completedCount += 1
      continue
    }
    if (FAILURE_RUN_STATUSES.has(normalizedStatus)) {
      failedCount += 1
      continue
    }
    if (isHarnessRunActiveStatus(normalizedStatus)) {
      runningCount += 1
    }
  }

  return {
    coordinatorCount: flattened.filter((node) => node.isCoordinator && !node.parentID).length,
    workerCount: flattened.filter((node) => Boolean(node.parentID)).length,
    detachedCount: flattened.filter((node) => node.isDetached).length,
    runningCount,
    completedCount,
    failedCount,
    waitingInputCount,
    firstFailedRunID: flattened.find((node) =>
      FAILURE_RUN_STATUSES.has(String(node.run.status || '').trim() as HarnessRunStatus)
    )?.run.id,
    firstWaitingInputRunID: flattened.find(
      (node) => String(node.run.status || '').trim() === 'waiting_input'
    )?.run.id,
    firstDetachedRunID: flattened.find((node) => node.isDetached)?.run.id,
  }
}

export function deriveHarnessRunPreview(
  run?: HarnessRunSummary | null,
  events: Pick<
    HarnessRunEvent,
    'capability_kind' | 'created_at' | 'message' | 'tool_name' | 'type'
  >[] = []
): string {
  const directPreview = compactPreview(String(run?.result || run?.error || '').trim())
  if (directPreview) return directPreview

  const latestEvent = [...events]
    .sort((left, right) => parseTime(right.created_at) - parseTime(left.created_at))
    .find(
      (event) =>
        String(event.message || '').trim() ||
        String(event.tool_name || '').trim() ||
        String(event.capability_kind || '').trim() ||
        String(event.type || '').trim()
    )

  if (!latestEvent) return ''

  const message = compactPreview(latestEvent.message)
  if (message) return message

  const toolName = String(latestEvent.tool_name || '').trim()
  if (toolName) return compactPreview(`Tool: ${toolName}`)

  const capabilityKind = String(latestEvent.capability_kind || '').trim()
  if (capabilityKind) return compactPreview(`Capability: ${humanizeEnum(capabilityKind)}`)

  return compactPreview(humanizeEnum(latestEvent.type))
}

function isPriorityBranch(node: HarnessRunTreeNode, selectedRunID = ''): boolean {
  if (node.run.id === selectedRunID) return true
  const normalizedStatus = String(node.run.status || '').trim()
  if (normalizedStatus === 'failed' || normalizedStatus === 'waiting_input') return true
  return node.children.some((child) => isPriorityBranch(child, selectedRunID))
}

export function deriveHarnessRunChildVisibility(
  node: HarnessRunTreeNode,
  selectedRunID = '',
  batchThreshold = 3
): HarnessRunChildVisibility {
  if (node.children.length < batchThreshold) {
    return {
      shouldCollapse: false,
      visibleChildren: node.children,
      hiddenChildren: [],
      representativeLabels: [],
    }
  }

  const visibleChildren = node.children.filter((child) => isPriorityBranch(child, selectedRunID))
  const hiddenChildren = node.children.filter((child) => !isPriorityBranch(child, selectedRunID))

  return {
    shouldCollapse: hiddenChildren.length > 0,
    visibleChildren,
    hiddenChildren,
    representativeLabels: hiddenChildren
      .map((child) => String(child.run.agent_id || child.run.kind || child.run.id).trim())
      .filter(Boolean)
      .slice(0, 3),
  }
}
