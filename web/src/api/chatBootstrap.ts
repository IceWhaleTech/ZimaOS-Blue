import api from './client'
import type {
  ConversationActiveStreamState,
  ConversationCommandState,
} from './chat'
import type { UserTaskProjection } from './tasks'

export interface ConversationBootstrapResponse {
  command_state: ConversationCommandState
  active_stream: ConversationActiveStreamState
  current_tasks: UserTaskProjection[]
  background_tasks: UserTaskProjection[]
  pending_approval?: Record<string, unknown> | null
  pending_question?: Record<string, unknown> | null
  pending_exec_approval?: Record<string, unknown> | null
}

const inflightConversationBootstrap = new Map<
  string,
  Promise<{ data: ConversationBootstrapResponse }>
>()

export const chatBootstrapApi = {
  getConversationBootstrap(conversationId: string) {
    const normalizedConversationId = String(conversationId || '').trim()
    if (!normalizedConversationId) {
      return Promise.resolve({
        data: {
          command_state: {
            conversation_id: '',
            offline: false,
          },
          active_stream: {
            conversation_id: '',
            active: false,
          },
          current_tasks: [],
          background_tasks: [],
          pending_approval: null,
          pending_question: null,
          pending_exec_approval: null,
        },
      })
    }

    const existing = inflightConversationBootstrap.get(normalizedConversationId)
    if (existing) {
      return existing
    }

    const request = api
      .get<ConversationBootstrapResponse>(`/conversations/${normalizedConversationId}/bootstrap`)
      .finally(() => {
        inflightConversationBootstrap.delete(normalizedConversationId)
      })

    inflightConversationBootstrap.set(normalizedConversationId, request)
    return request
  },
}
