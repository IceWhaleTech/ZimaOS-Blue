import api from './client'

// Types
export interface Conversation {
  id: string
  title: string
  created_at: string
  updated_at: string
}

export interface ToolCall {
  id: string
  name: string
  arguments: string
}

export interface MessageStats {
  input_tokens: number
  output_tokens: number
  total_tokens: number
  latency_ms: number
  ttft_ms: number
  tokens_per_second: number
}

export interface Message {
  id: string
  conversation_id: string
  role: 'user' | 'assistant' | 'system' | 'tool'
  content: string
  tool_calls?: ToolCall[]
  tool_call_id?: string
  created_at: string
  // Runtime metadata (not persisted to database)
  provider?: string
  model?: string
  stats?: MessageStats
  // Attachments for user messages (for display purposes)
  attachments?: MessageAttachment[]
}

export interface MessageAttachment {
  type: 'image' | 'file'
  name: string
  mime_type: string
  data: string // base64 encoded
}

export interface SendMessageRequest {
  message: string
  provider: string  // Provider ID from Provider Pool
  model: string
  temperature?: number
  max_tokens?: number
  attachments?: MessageAttachment[]
}

export interface SendMessageResponse {
  id: string
  role: string
  content: string
}

export interface ToolDefinition {
  name: string
  description: string
  parameters: Record<string, unknown>
}

export interface StreamChunk {
  delta: string
  done: boolean
  stream_id?: string
  provider?: string
  model?: string
  stats?: {
    input_tokens: number
    output_tokens: number
    total_tokens: number
    latency_ms: number
    ttft_ms: number
    tokens_per_second: number
  }
  usage?: {
    prompt_tokens: number
    completion_tokens: number
    total_tokens: number
  }
}

// Conversation API
export const conversationApi = {
  create: (title?: string) =>
    api.post<Conversation>('/conversations', { title: title || '' }),

  list: (limit = 50, offset = 0) =>
    api.get<Conversation[]>('/conversations', { params: { limit, offset } }),

  get: (id: string) => api.get<Conversation>(`/conversations/${id}`),

  delete: (id: string) => api.delete(`/conversations/${id}`),

  search: (query: string, limit = 20) =>
    api.get<Conversation[]>('/conversations', { params: { q: query, limit } }),
}

// Message API
export const messageApi = {
  list: (conversationId: string, limit = 100, offset = 0) =>
    api.get<Message[]>(`/conversations/${conversationId}/messages`, {
      params: { limit, offset },
    }),

  send: (conversationId: string, request: SendMessageRequest) =>
    api.post<SendMessageResponse>(`/conversations/${conversationId}/messages`, request),

  delete: (conversationId: string, messageIds: string[]) =>
    api.delete<{ success: boolean; deleted: number }>(`/conversations/${conversationId}/messages`, {
      data: { message_ids: messageIds },
    }),

  // Note: For streaming, use the SSE utility instead
  getStreamUrl: (conversationId: string) =>
    `/api/v1/conversations/${conversationId}/messages/stream`,
}

// Tool API
export const toolApi = {
  list: () => api.get<ToolDefinition[]>('/tools'),
}

// Card Action API - For Typeless card interactions
export interface CardActionRequest {
  card_id: string
  action_id: string
  action_label?: string
  form_data?: Record<string, unknown>
}

export interface CardActionResponse {
  success: boolean
  message_id?: string
  error?: string
}

export const cardActionApi = {
  /**
   * Submit a card action (button click, choice selection, form submit)
   * This will trigger the agent to continue the conversation
   */
  submit: (conversationId: string, messageId: string, request: CardActionRequest) =>
    api.post<CardActionResponse>(
      `/conversations/${conversationId}/messages/${messageId}/card-action`,
      request
    ),
}
