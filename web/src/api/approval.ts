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
  binding_hash?: string
  created_at: string
}

export interface ApprovedDirectoryEntry {
  id: string
  path: string
  added_at: string
  last_used: string
  approved_by?: string
}

export interface ApprovedBrowserSiteEntry {
  id: string
  origin: string
  added_at: string
  last_used: string
  approved_by?: string
}

export const approvalApi = {
  getConfig: () => api.get<ApprovalConfig>('/approval/config'),

  updateConfig: (config: ApprovalConfig) => api.put<ApprovalConfig>('/approval/config', config),

  resolve: (requestId: string, decision: Decision | ExecDecision, bindingHash?: string) =>
    api.post<{ status: string }>('/approval/resolve', {
      request_id: requestId,
      decision,
      ...(bindingHash ? { binding_hash: bindingHash } : {}),
    }),

  listApprovedDirectories: () =>
    api.get<{ entries: ApprovedDirectoryEntry[] }>('/exec/approvals/directories'),

  revokeApprovedDirectory: (id: string) =>
    api.delete<{ deleted: boolean }>(`/exec/approvals/directories/${encodeURIComponent(id)}`),

  listApprovedBrowserSites: () =>
    api.get<{ entries: ApprovedBrowserSiteEntry[] }>('/browser/approvals/sites'),

  revokeApprovedBrowserSite: (id: string) =>
    api.delete<{ deleted: boolean }>(`/browser/approvals/sites/${encodeURIComponent(id)}`),
}
