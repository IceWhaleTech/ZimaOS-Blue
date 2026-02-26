import api from './client'

// Types
export interface Conversation {
  id: string
  title: string
  created_at: string
  updated_at: string
  pinned?: boolean
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
  type: 'image' | 'file' | 'audio'
  name: string
  mime_type: string
  data: string // base64 encoded
  duration?: number // audio duration in seconds
}

export interface SendMessageRequest {
  message: string
  provider: string  // Provider ID from Provider Pool
  model: string
  temperature?: number
  max_tokens?: number
  attachments?: MessageAttachment[]
  regenerate?: boolean  // True if this is a regenerate request
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
  error?: string
  stream_id?: string
  provider?: string
  model?: string
  // Tool execution status (sent when backend starts executing tool calls)
  tool_executing?: boolean
  tool_calls?: number
  tool_names?: string[]
  tool_commands?: string[]
  sandbox_available?: boolean
  // Tool results (sent after tool execution completes)
  tool_results?: Array<{ name: string; id: string; args?: string; result?: string }>
  tool_round?: number
  // Context pruning info (sent on first content chunk)
  pruned?: boolean
  messages_pruned?: number
  tokens_before?: number
  tokens_after?: number
  // Context compaction info (sent on first content chunk)
  compacted?: boolean
  before?: number
  after?: number
  // Mid-stream injection (sent when user injects a message during streaming)
  injection?: boolean
  user_message?: string
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
  // Set when server sends a synthetic done after tool execution produced no LLM text
  empty_response?: boolean
  // New message event — server persisted previous round, frontend should start new bubble
  new_message?: boolean
  // TODO advancement event — server updated a TODO checklist message in DB
  todo_updated?: boolean
  message_id?: string
  content?: string
}

// Conversation API
export const conversationApi = {
  create: (title?: string) =>
    api.post<Conversation>('/conversations', { title: title || '' }),

  list: (limit = 50, offset = 0) =>
    api.get<Conversation[]>('/conversations', { params: { limit, offset } }),

  get: (id: string) => api.get<Conversation>(`/conversations/${id}`),

  delete: (id: string) => api.delete(`/conversations/${id}`),

  pin: (id: string) => api.post(`/conversations/${id}/pin`),

  unpin: (id: string) => api.post(`/conversations/${id}/unpin`),

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

// Warmup API - Pre-compute system prompt and context to reduce TTFT
export const warmupApi = {
  /** Fire-and-forget warmup for a conversation. Returns 204. */
  trigger: (conversationId: string) =>
    api.post(`/conversations/${conversationId}/warmup`),
}

// Injection API - Send a message during active streaming
export const injectionApi = {
  /** Inject a user message into an active stream. Cancels current stream and restarts with new context. */
  inject: (conversationId: string, message: string) =>
    api.post<{ success: boolean; injected: boolean; stream_id: string }>(
      `/conversations/${conversationId}/inject`,
      { message }
    ),
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
  card_type?: string
  card_title?: string
  form_data?: Record<string, unknown>
}

export interface CardActionResponse {
  success: boolean
  message_id?: string
  message?: string
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

// Agent Task API
export interface AgentTask {
  id: string
  user_id: string
  conversation_id?: string
  goal: string
  plan: AgentPlanStep[]
  status: 'pending' | 'planning' | 'executing' | 'waiting_input' | 'completed' | 'failed' | 'cancelled'
  current_step: number
  progress: number
  result?: string
  error?: string
  questions?: AgentQuestion[] // pending questions from ask_user tool
  created_at: string
  updated_at: string
}

export interface AgentPlanStep {
  index: number
  description: string
  status: 'pending' | 'running' | 'completed' | 'failed' | 'skipped'
  output?: string
  started_at?: string
  completed_at?: string
}

// Agent Q&A types
export interface AgentQuestion {
  id: string
  question: string
  header: string
  options?: AgentQuestionOption[]
  multi_select?: boolean
  required?: boolean
}

export interface AgentQuestionOption {
  label: string
  description?: string
  value: string
}

export interface AgentQuestionAnswer {
  question_id: string
  values: string[]
  other_text?: string
}

export const agentApi = {
  createTask: (goal: string, conversationId?: string, context?: string) =>
    api.post<AgentTask>('/agent/tasks', { goal, conversation_id: conversationId, context }),

  listTasks: () =>
    api.get<AgentTask[]>('/agent/tasks'),

  getTask: (id: string) =>
    api.get<AgentTask>(`/agent/tasks/${id}`),

  cancelTask: (id: string) =>
    api.post(`/agent/tasks/${id}/cancel`),

  deleteTask: (id: string) =>
    api.delete(`/agent/tasks/${id}`),

  /** Send a message to a running agent task. Queued for injection at next natural boundary. */
  sendMessage: (taskId: string, message: string) =>
    api.post<{ status: string }>(`/agent/tasks/${taskId}/message`, { message }),

  /** Submit answers to a pending ask_user question. */
  submitAnswers: (taskId: string, answers: AgentQuestionAnswer[]) =>
    api.post<{ status: string }>(`/agent/tasks/${taskId}/answer`, { answers }),
}
