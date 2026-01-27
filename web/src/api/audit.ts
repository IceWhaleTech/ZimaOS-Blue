import api from './client'

export type AuditAction =
  | 'login'
  | 'login_failed'
  | 'logout'
  | 'user_create'
  | 'user_update'
  | 'user_delete'
  | 'mfa_setup'
  | 'mfa_disable'
  | 'api_key_create'
  | 'api_key_delete'
  | 'settings_update'

export type AuditStatus = 'success' | 'failure'

export interface AuditEntry {
  id: string
  timestamp: string
  user_id?: string
  username: string
  action: AuditAction
  resource_type: string
  resource_id: string
  ip_address: string
  user_agent: string
  request_id: string
  status: AuditStatus
  details?: Record<string, unknown>
}

export interface AuditQueryResult {
  entries: AuditEntry[]
  total: number
  page: number
  page_size: number
}

export interface AuditStats {
  by_action: Record<string, number>
  by_status: {
    success: number
    failure: number
  }
  time_range: {
    start: string
    end?: string
  }
}

export interface AuditQueryParams {
  user_id?: string
  action?: AuditAction
  resource_type?: string
  resource_id?: string
  status?: AuditStatus
  start_time?: string
  end_time?: string
  ip_address?: string
  page?: number
  page_size?: number
  sort_by?: string
  sort_dir?: 'asc' | 'desc'
}

export const auditApi = {
  // List audit logs
  list: (params?: AuditQueryParams) => api.get<AuditQueryResult>('/audit', { params }),

  // Get audit log by ID
  get: (id: string) => api.get<AuditEntry>(`/audit/${id}`),

  // Export audit logs
  export: (params?: AuditQueryParams & { format?: 'json' | 'csv' }) => {
    const queryParams = new URLSearchParams()
    if (params) {
      Object.entries(params).forEach(([key, value]) => {
        if (value !== undefined) {
          queryParams.append(key, String(value))
        }
      })
    }
    const url = `/api/v1/audit/export?${queryParams.toString()}`
    window.open(url, '_blank')
  },

  // Get audit stats
  getStats: (startTime?: string, endTime?: string) =>
    api.get<AuditStats>('/audit/stats', {
      params: { start_time: startTime, end_time: endTime },
    }),
}
