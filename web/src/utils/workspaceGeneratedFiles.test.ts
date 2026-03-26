import { describe, expect, it } from 'vitest'
import type { TypelessCardConvertTask, TypelessCardResult } from '@/types/typeless'
import {
  extractLocalPathCandidatesFromCard,
  resolveWorkspaceGeneratedPathCandidate,
} from './workspaceGeneratedFiles'

describe('workspaceGeneratedFiles', () => {
  it('resolves relative workspace paths and ignores API urls', () => {
    expect(resolveWorkspaceGeneratedPathCandidate('exports/report.pdf', '/tmp/workspace')).toBe(
      '/tmp/workspace/exports/report.pdf'
    )
    expect(
      resolveWorkspaceGeneratedPathCandidate(
        '/api/v1/convert/tasks/task-1/download/out-1',
        '/tmp/workspace'
      )
    ).toBeNull()
  })

  it('extracts local output paths from convert-task cards', () => {
    const card: TypelessCardConvertTask = {
      type: 'convert-task',
      task_id: 'task-1',
      status: 'succeeded',
      outputs: [
        {
          output_id: 'out-1',
          name: 'report.pdf',
          path: '/tmp/workspace/exports/report.pdf',
          download_url: '/api/v1/convert/tasks/task-1/download/out-1',
        },
      ],
    }

    expect(extractLocalPathCandidatesFromCard(card, '/tmp/workspace')).toEqual([
      '/tmp/workspace/exports/report.pdf',
    ])
  })

  it('extracts relative result-card paths into workspace absolute paths', () => {
    const card: TypelessCardResult = {
      type: 'result',
      title: 'write_commit',
      status: 'success',
      details: [{ label: 'path', value: 'reports/summary.md' }],
    }

    expect(extractLocalPathCandidatesFromCard(card, '/tmp/workspace')).toEqual([
      '/tmp/workspace/reports/summary.md',
    ])
  })

  it('extracts reveal-path targets when the displayed path is not directly resolvable', () => {
    const card = {
      type: 'result',
      title: 'file_write',
      status: 'success',
      details: [
        {
          label: 'Path',
          value: '@docs/reports/summary.md',
          reveal_path: '/tmp/workspace/reports/summary.md',
        },
      ],
    } as TypelessCardResult

    expect(extractLocalPathCandidatesFromCard(card, '/tmp/workspace')).toEqual([
      '/tmp/workspace/reports/summary.md',
    ])
  })
})
