import { describe, expect, it } from 'vitest'

import {
  localizeTaskProjectionSubtitle,
  localizeTaskProjectionPreviewText,
  localizeTaskProjectionTitle,
} from '@/utils/taskProjectionText'

const messages: Record<string, string> = {
  'chat.deepResearchStagePlanning': '规划中',
  'chat.deepResearchActionVerificationCompleted': '验证已完成',
  'chat.deepResearchGapNeedPrimaryOrOfficialSources': '需要一手或官方来源',
  'chat.taskRuntimeExecute': '执行中',
  'chat.taskDefaultResearchTitle': '研究任务',
  'chat.taskDefaultAgentTitle': '智能体任务',
  'chat.taskDefaultWorkflowTitle': '工作流任务',
  'chat.taskFailed': '任务失败',
  'chat.taskFailureGroundedVerificationFailed': '事实依据核验失败',
}

function translate(key: string, fallback: string): string {
  return messages[key] || fallback
}

describe('taskProjectionText', () => {
  it('localizes agent runtime subtitles', () => {
    expect(localizeTaskProjectionSubtitle('execute', 'agent_task', translate)).toBe('执行中')
  })

  it('localizes research subtitles while preserving free-form gaps', () => {
    expect(
      localizeTaskProjectionSubtitle(
        'planning • verification_completed • Need official source',
        'research',
        translate
      )
    ).toBe('规划中 · 验证已完成 · 需要一手或官方来源')
  })

  it('localizes default task titles', () => {
    expect(localizeTaskProjectionTitle('Research task', 'research', translate)).toBe('研究任务')
    expect(localizeTaskProjectionTitle('Agent task', 'agent_task', translate)).toBe('智能体任务')
    expect(localizeTaskProjectionTitle('', 'workflow', translate)).toBe('工作流任务')
  })

  it('localizes common task failure preview text', () => {
    expect(localizeTaskProjectionPreviewText('Task failed: grounded verification failed', translate)).toBe(
      '任务失败: 事实依据核验失败'
    )
  })
})
