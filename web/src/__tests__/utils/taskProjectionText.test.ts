import { describe, expect, it } from 'vitest'

import {
  localizeTaskProjectionSubtitle,
  localizeTaskProjectionTitle,
} from '@/utils/taskProjectionText'

const messages: Record<string, string> = {
  'chat.deepResearchStagePlanning': '规划中',
  'chat.deepResearchActionVerificationCompleted': '验证已完成',
  'chat.taskRuntimeExecute': '执行中',
  'chat.taskDefaultResearchTitle': '研究任务',
  'chat.taskDefaultAgentTitle': '智能体任务',
  'chat.taskDefaultWorkflowTitle': '工作流任务',
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
    ).toBe('规划中 · 验证已完成 · Need official source')
  })

  it('localizes default task titles', () => {
    expect(localizeTaskProjectionTitle('Research task', 'research', translate)).toBe('研究任务')
    expect(localizeTaskProjectionTitle('Agent task', 'agent_task', translate)).toBe('智能体任务')
    expect(localizeTaskProjectionTitle('', 'workflow', translate)).toBe('工作流任务')
  })
})
