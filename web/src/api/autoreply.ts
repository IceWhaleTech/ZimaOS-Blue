import api from './client'

// Types
export type TriggerType = 'keyword' | 'regex' | 'contains' | 'prefix' | 'suffix'

export interface AutoReplyRule {
  id: string
  name: string
  trigger_type: TriggerType
  trigger_value: string
  responses: string[]
  priority: number
  enabled: boolean
  channels: string[]
  created_at: string
  updated_at: string
  match_count: number
}

export interface CreateRuleRequest {
  name: string
  trigger_type: TriggerType
  trigger_value: string
  responses: string[]
  priority: number
}

export interface UpdateRuleRequest {
  name: string
  trigger_type: TriggerType
  trigger_value: string
  responses: string[]
  priority: number
}

export interface TestRuleRequest {
  message: string
  channel?: string
}

export interface TestRuleResponse {
  matched: boolean
  rule_id?: string
  rule_name?: string
  response?: string
}

// Auto-Reply API
export const autoReplyApi = {
  // List all rules
  list: () => api.get<AutoReplyRule[]>('/autoreply/rules'),

  // Get a single rule
  get: (id: string) => api.get<AutoReplyRule>(`/autoreply/rules/${id}`),

  // Create a new rule
  create: (data: CreateRuleRequest) => api.post<AutoReplyRule>('/autoreply/rules', data),

  // Update a rule
  update: (id: string, data: UpdateRuleRequest) =>
    api.put<AutoReplyRule>(`/autoreply/rules/${id}`, data),

  // Delete a rule
  delete: (id: string) => api.delete(`/autoreply/rules/${id}`),

  // Enable a rule
  enable: (id: string) => api.post<{ success: boolean }>(`/autoreply/rules/${id}/enable`),

  // Disable a rule
  disable: (id: string) => api.post<{ success: boolean }>(`/autoreply/rules/${id}/disable`),

  // Set channels for a rule
  setChannels: (id: string, channels: string[]) =>
    api.put<{ success: boolean }>(`/autoreply/rules/${id}/channels`, { channels }),

  // Test a message against rules
  test: (data: TestRuleRequest) => api.post<TestRuleResponse>('/autoreply/test', data),
}
