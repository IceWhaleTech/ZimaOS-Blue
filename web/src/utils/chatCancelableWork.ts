import type { DeepResearchJobSummary } from '@/api/deepResearch'

export interface ChatCancelableWorkState {
  streaming: boolean
  sending: boolean
  isRecovering: boolean
  mediaGenerating: boolean
  streamUIPhase?: string | null
  currentConversationTaskCount: number
  currentConversationResearchJobCount: number
}

function normalizeConversationId(value: unknown): string {
  return String(value ?? '').trim()
}

export function getCurrentConversationDeepResearchJobs(
  activeJobs: DeepResearchJobSummary[] | null | undefined,
  conversationId: unknown
): DeepResearchJobSummary[] {
  const normalizedConversationId = normalizeConversationId(conversationId)
  if (!normalizedConversationId || !Array.isArray(activeJobs)) return []
  return activeJobs.filter(
    (job) => normalizeConversationId(job?.conversation_id) === normalizedConversationId
  )
}

export function hasCancelableChatWork(state: ChatCancelableWorkState): boolean {
  if (state.streaming || state.sending || state.isRecovering || state.mediaGenerating) {
    return true
  }

  if (
    state.streamUIPhase === 'recovering' ||
    state.streamUIPhase === 'awaiting_confirmation' ||
    state.streamUIPhase === 'interrupted'
  ) {
    return true
  }

  return (
    state.currentConversationTaskCount > 0 || state.currentConversationResearchJobCount > 0
  )
}
