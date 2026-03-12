import api from './client'

export type Policy = 'auto' | 'ask' | 'deny'
export type Decision = 'approve' | 'deny'
export type ExecDecision = 'allow-once' | 'allow-always' | 'deny'

export interface ApprovalConfig {
  enabled: boolean
  default_policy: Policy
  tool_policies: Record<string, Policy>
}

export interface PendingRequest {
  id: string
  tool_name: string
  tool_call_id: string
  arguments: Record<string, unknown>
  session_id?: string
  created_at: string
}

export const approvalApi = {
  getConfig: () => api.get<ApprovalConfig>('/approval/config'),

  updateConfig: (config: ApprovalConfig) =>
    api.put<ApprovalConfig>('/approval/config', config),

  listPending: (sessionId?: string) => api.get<PendingRequest[]>('/approval/pending', {
    params: sessionId ? { session_id: sessionId } : undefined,
  }),

  resolve: (requestId: string, decision: Decision | ExecDecision) =>
    api.post<{ status: string }>('/approval/resolve', {
      request_id: requestId,
      decision,
    }),
}
