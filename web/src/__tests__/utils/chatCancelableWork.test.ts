import { describe, expect, it } from 'vitest'
import {
  getCurrentConversationDeepResearchJobs,
  hasCancelableChatWork,
} from '@/utils/chatCancelableWork'

describe('chatCancelableWork', () => {
  it('keeps only deep research jobs from the active conversation', () => {
    const jobs = [
      {
        id: 'job-1',
        job_id: 'job-1',
        conversation_id: 'conv-1',
        query: 'Current conversation research',
        status: 'running',
        stage: 'planning',
        progress: 48,
        updated_at: '2026-03-20T12:00:00.000Z',
      },
      {
        id: 'job-2',
        job_id: 'job-2',
        conversation_id: 'conv-other',
        query: 'Other conversation research',
        status: 'running',
        stage: 'planning',
        progress: 32,
        updated_at: '2026-03-20T12:01:00.000Z',
      },
    ]

    expect(getCurrentConversationDeepResearchJobs(jobs as never, 'conv-1')).toEqual([jobs[0]])
    expect(getCurrentConversationDeepResearchJobs(jobs as never, 'conv-missing')).toEqual([])
  })

  it('treats current-conversation deep research as cancelable work', () => {
    expect(
      hasCancelableChatWork({
        streaming: false,
        sending: false,
        toolExecuting: false,
        isRecovering: false,
        mediaGenerating: false,
        streamUIPhase: 'idle',
        currentConversationTaskCount: 0,
        currentConversationResearchJobCount: 1,
      })
    ).toBe(true)
  })

  it('does not expose cancelable work for deep research in another conversation only', () => {
    expect(
      hasCancelableChatWork({
        streaming: false,
        sending: false,
        toolExecuting: false,
        isRecovering: false,
        mediaGenerating: false,
        streamUIPhase: 'idle',
        currentConversationTaskCount: 0,
        currentConversationResearchJobCount: 0,
      })
    ).toBe(false)
  })

  it('treats executing tool work as cancelable even without streaming text yet', () => {
    expect(
      hasCancelableChatWork({
        streaming: false,
        sending: false,
        toolExecuting: true,
        isRecovering: false,
        mediaGenerating: false,
        streamUIPhase: 'executing',
        currentConversationTaskCount: 0,
        currentConversationResearchJobCount: 0,
      })
    ).toBe(true)
  })
})
