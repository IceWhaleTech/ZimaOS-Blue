import { describe, expect, it } from 'vitest'

import type { HarnessRunSummary } from '@/api/harness'
import {
  buildHarnessRunTree,
  compareHarnessRuns,
  deriveHarnessRunChildVisibility,
  deriveHarnessRunPreview,
  isHarnessRunActiveStatus,
  isHarnessRunTerminalStatus,
  summarizeHarnessRunTree,
} from '@/utils/harnessRunTree'

function createRun(
  overrides: Partial<HarnessRunSummary> & Pick<HarnessRunSummary, 'id'>
): HarnessRunSummary {
  return {
    id: overrides.id,
    root_run_id: overrides.root_run_id || overrides.id,
    parent_run_id: overrides.parent_run_id,
    kind: overrides.kind || 'subagent',
    status: overrides.status || 'pending',
    goal: overrides.goal || overrides.id,
    attempt_index: overrides.attempt_index ?? 0,
    created_at: overrides.created_at || '2026-03-20T10:00:00Z',
    updated_at: overrides.updated_at || '2026-03-20T10:00:00Z',
    started_at: overrides.started_at,
    finished_at: overrides.finished_at,
    runtime_state: overrides.runtime_state,
    result: overrides.result,
    error: overrides.error,
    agent_id: overrides.agent_id,
    model: overrides.model,
    depth: overrides.depth,
    progress: overrides.progress,
    group_id: overrides.group_id,
    group_item_id: overrides.group_item_id,
  }
}

describe('harnessRunTree', () => {
  it('builds a single root node', () => {
    const tree = buildHarnessRunTree([
      createRun({ id: 'run-root', kind: 'research', status: 'completed' }),
    ])

    expect(tree).toHaveLength(1)
    expect(tree[0].run.id).toBe('run-root')
    expect(tree[0].children).toHaveLength(0)
    expect(tree[0].isCoordinator).toBe(false)
    expect(tree[0].isDetached).toBe(false)
  })

  it('builds coordinator and nested worker branches', () => {
    const tree = buildHarnessRunTree([
      createRun({ id: 'worker-b', root_run_id: 'coordinator', parent_run_id: 'coordinator' }),
      createRun({ id: 'grandchild', root_run_id: 'coordinator', parent_run_id: 'worker-a' }),
      createRun({ id: 'coordinator', kind: 'agent_task', status: 'executing' }),
      createRun({ id: 'worker-a', root_run_id: 'coordinator', parent_run_id: 'coordinator' }),
    ])

    expect(tree).toHaveLength(1)
    expect(tree[0].run.id).toBe('coordinator')
    expect(tree[0].isCoordinator).toBe(true)
    expect(tree[0].children.map((node) => node.run.id)).toEqual(['worker-a', 'worker-b'])
    expect(tree[0].children[0].children.map((node) => node.run.id)).toEqual(['grandchild'])
  })

  it('marks orphaned children as detached roots', () => {
    const tree = buildHarnessRunTree([
      createRun({ id: 'worker-detached', parent_run_id: 'missing-parent', root_run_id: 'root-x' }),
      createRun({ id: 'worker-root', kind: 'research' }),
    ])

    expect(tree.map((node) => node.run.id)).toEqual(['worker-detached', 'worker-root'])
    expect(tree[0].isDetached).toBe(true)
    expect(tree[0].parentID).toBe('missing-parent')
    expect(tree[1].isDetached).toBe(false)
  })

  it('sorts deterministically regardless of input order', () => {
    const input = [
      createRun({
        id: 'terminal-late',
        status: 'completed',
        created_at: '2026-03-20T10:00:00Z',
        updated_at: '2026-03-20T10:05:00Z',
      }),
      createRun({
        id: 'attempt-2',
        attempt_index: 2,
        created_at: '2026-03-20T09:59:00Z',
        updated_at: '2026-03-20T10:01:00Z',
      }),
      createRun({
        id: 'attempt-1',
        attempt_index: 1,
        created_at: '2026-03-20T09:59:00Z',
        updated_at: '2026-03-20T10:01:00Z',
      }),
      createRun({
        id: 'active-tie',
        status: 'executing',
        created_at: '2026-03-20T10:00:00Z',
        updated_at: '2026-03-20T10:05:00Z',
      }),
    ]

    expect(buildHarnessRunTree(input).map((node) => node.run.id)).toEqual([
      'attempt-1',
      'attempt-2',
      'active-tie',
      'terminal-late',
    ])
  })

  it('summarizes coordinator, worker, and status counts', () => {
    const tree = buildHarnessRunTree([
      createRun({ id: 'run-root', kind: 'agent_task', status: 'executing' }),
      createRun({
        id: 'worker-completed',
        root_run_id: 'run-root',
        parent_run_id: 'run-root',
        status: 'completed',
      }),
      createRun({
        id: 'worker-failed',
        root_run_id: 'run-root',
        parent_run_id: 'run-root',
        status: 'failed',
      }),
      createRun({
        id: 'worker-waiting',
        root_run_id: 'run-root',
        parent_run_id: 'run-root',
        status: 'waiting_input',
      }),
      createRun({
        id: 'worker-detached',
        root_run_id: 'missing',
        parent_run_id: 'missing-parent',
        status: 'completed',
      }),
    ])

    const summary = summarizeHarnessRunTree(tree)

    expect(summary.coordinatorCount).toBe(1)
    expect(summary.workerCount).toBe(4)
    expect(summary.detachedCount).toBe(1)
    expect(summary.runningCount).toBe(1)
    expect(summary.completedCount).toBe(2)
    expect(summary.failedCount).toBe(1)
    expect(summary.waitingInputCount).toBe(1)
    expect(summary.firstFailedRunID).toBe('worker-failed')
    expect(summary.firstWaitingInputRunID).toBe('worker-waiting')
    expect(summary.firstDetachedRunID).toBe('worker-detached')
  })

  it('collapses non-priority worker children into a batch summary', () => {
    const tree = buildHarnessRunTree([
      createRun({ id: 'run-root', kind: 'agent_task', status: 'executing' }),
      createRun({
        id: 'worker-completed',
        root_run_id: 'run-root',
        parent_run_id: 'run-root',
        status: 'completed',
        agent_id: 'worker.fetch',
      }),
      createRun({
        id: 'worker-failed',
        root_run_id: 'run-root',
        parent_run_id: 'run-root',
        status: 'failed',
        agent_id: 'worker.review',
      }),
      createRun({
        id: 'worker-waiting',
        root_run_id: 'run-root',
        parent_run_id: 'run-root',
        status: 'waiting_input',
        agent_id: 'worker.blocker',
      }),
      createRun({
        id: 'worker-running',
        root_run_id: 'run-root',
        parent_run_id: 'run-root',
        status: 'executing',
        agent_id: 'worker.verify',
      }),
    ])

    const visibility = deriveHarnessRunChildVisibility(tree[0], '', 3)

    expect(visibility.shouldCollapse).toBe(true)
    expect(visibility.visibleChildren.map((node) => node.run.id)).toEqual([
      'worker-waiting',
      'worker-failed',
    ])
    expect(visibility.hiddenChildren.map((node) => node.run.id)).toEqual([
      'worker-running',
      'worker-completed',
    ])
    expect(visibility.representativeLabels).toEqual(['worker.verify', 'worker.fetch'])
  })

  it('keeps the selected child visible even when the batch is collapsed', () => {
    const tree = buildHarnessRunTree([
      createRun({ id: 'run-root', kind: 'agent_task', status: 'executing' }),
      createRun({
        id: 'worker-completed',
        root_run_id: 'run-root',
        parent_run_id: 'run-root',
        status: 'completed',
      }),
      createRun({
        id: 'worker-failed',
        root_run_id: 'run-root',
        parent_run_id: 'run-root',
        status: 'failed',
      }),
      createRun({
        id: 'worker-waiting',
        root_run_id: 'run-root',
        parent_run_id: 'run-root',
        status: 'waiting_input',
      }),
      createRun({
        id: 'worker-running',
        root_run_id: 'run-root',
        parent_run_id: 'run-root',
        status: 'executing',
      }),
    ])

    const visibility = deriveHarnessRunChildVisibility(tree[0], 'worker-running', 3)

    expect(visibility.visibleChildren.map((node) => node.run.id)).toEqual([
      'worker-running',
      'worker-waiting',
      'worker-failed',
    ])
    expect(visibility.hiddenChildren.map((node) => node.run.id)).toEqual(['worker-completed'])
  })

  it('prefers result and error fields when deriving previews', () => {
    expect(
      deriveHarnessRunPreview(
        createRun({
          id: 'run-result',
          result: 'Final summary is ready',
          error: 'This should not win',
        })
      )
    ).toBe('Final summary is ready')

    expect(
      deriveHarnessRunPreview(
        createRun({
          id: 'run-error',
          error: 'Validation failed',
        })
      )
    ).toBe('Validation failed')
  })

  it('falls back to the newest event message when preview fields are empty', () => {
    const preview = deriveHarnessRunPreview(createRun({ id: 'run-events' }), [
      {
        type: 'tool_call',
        message: 'Older message',
        created_at: '2026-03-20T10:00:00Z',
      },
      {
        type: 'tool_call',
        message: 'Newest message',
        created_at: '2026-03-20T10:05:00Z',
      },
    ])

    expect(preview).toBe('Newest message')
  })

  it('falls back to tool, capability, and event type when needed', () => {
    expect(
      deriveHarnessRunPreview(createRun({ id: 'run-tool' }), [
        {
          type: 'tool_call',
          tool_name: 'shell.exec',
          created_at: '2026-03-20T10:05:00Z',
        },
      ])
    ).toBe('Tool: shell.exec')

    expect(
      deriveHarnessRunPreview(createRun({ id: 'run-capability' }), [
        {
          type: 'tool_call',
          capability_kind: 'file_search',
          created_at: '2026-03-20T10:05:00Z',
        },
      ])
    ).toBe('Capability: File Search')

    expect(
      deriveHarnessRunPreview(createRun({ id: 'run-type' }), [
        {
          type: 'agent_note',
          created_at: '2026-03-20T10:05:00Z',
        },
      ])
    ).toBe('Agent Note')
  })

  it('exposes stable run ordering helpers', () => {
    const active = createRun({ id: 'active', status: 'executing' })
    const terminal = createRun({ id: 'terminal', status: 'failed' })

    expect(isHarnessRunActiveStatus('executing')).toBe(true)
    expect(isHarnessRunTerminalStatus('failed')).toBe(true)
    expect(compareHarnessRuns(active, terminal)).toBeLessThan(0)
  })
})
