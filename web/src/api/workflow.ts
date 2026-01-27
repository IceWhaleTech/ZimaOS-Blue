import api from './client'

export type WorkflowStatus = 'draft' | 'active' | 'disabled' | 'error'
export type ExecutionStatus = 'pending' | 'running' | 'completed' | 'failed' | 'cancelled'
export type NodeType = 'trigger' | 'action' | 'condition' | 'loop' | 'delay'

export interface WorkflowNode {
  id: string
  type: NodeType
  name: string
  config: Record<string, unknown>
  position?: { x: number; y: number }
}

export interface WorkflowConnection {
  id: string
  source_node: string
  target_node: string
  source_port?: string
  target_port?: string
  condition?: string
}

export interface WorkflowSettings {
  max_executions?: number
  timeout_seconds?: number
  retry_on_failure?: boolean
  max_retries?: number
}

export interface Workflow {
  id: string
  tenant_id: string
  name: string
  description?: string
  status: WorkflowStatus
  nodes: WorkflowNode[]
  connections: WorkflowConnection[]
  variables?: Record<string, string>
  settings?: WorkflowSettings
  tags?: string[]
  created_by: string
  updated_by: string
  created_at: string
  updated_at: string
  version: number
}

export interface WorkflowExecution {
  id: string
  workflow_id: string
  status: ExecutionStatus
  trigger_data?: Record<string, unknown>
  result?: Record<string, unknown>
  error?: string
  started_at: string
  completed_at?: string
  duration_ms?: number
}

export interface ExecutionLog {
  id: string
  execution_id: string
  node_id: string
  level: 'info' | 'warn' | 'error'
  message: string
  data?: Record<string, unknown>
  timestamp: string
}

export interface WorkflowStats {
  total_workflows: number
  active_workflows: number
  total_executions: number
  successful_executions: number
  failed_executions: number
}

export interface WorkflowTemplate {
  id: string
  name: string
  description: string
  category: string
  tags: string[]
  workflow: Partial<Workflow>
}

export interface ListWorkflowsResponse {
  workflows: Workflow[]
  total: number
  offset: number
  limit: number
}

export interface ListExecutionsResponse {
  executions: WorkflowExecution[]
  total: number
  offset: number
  limit: number
}

export interface ListLogsResponse {
  logs: ExecutionLog[]
  total: number
  offset: number
  limit: number
}

export interface CreateWorkflowRequest {
  name: string
  description?: string
  nodes: WorkflowNode[]
  connections?: WorkflowConnection[]
  variables?: Record<string, string>
  settings?: WorkflowSettings
  tags?: string[]
}

export interface UpdateWorkflowRequest {
  name?: string
  description?: string
  status?: WorkflowStatus
  nodes?: WorkflowNode[]
  connections?: WorkflowConnection[]
  variables?: Record<string, string>
  settings?: WorkflowSettings
  tags?: string[]
}

export const workflowApi = {
  // List workflows
  list: (params?: { offset?: number; limit?: number; status?: string; name?: string }) =>
    api.get<ListWorkflowsResponse>('/workflows', { params }),

  // Get workflow by ID
  get: (id: string) => api.get<Workflow>(`/workflows/${id}`),

  // Create workflow
  create: (data: CreateWorkflowRequest) => api.post<Workflow>('/workflows', data),

  // Update workflow
  update: (id: string, data: UpdateWorkflowRequest) =>
    api.put<Workflow>(`/workflows/${id}`, data),

  // Delete workflow
  delete: (id: string) => api.delete(`/workflows/${id}`),

  // Enable workflow
  enable: (id: string) => api.post<{ status: string }>(`/workflows/${id}/enable`),

  // Disable workflow
  disable: (id: string) => api.post<{ status: string }>(`/workflows/${id}/disable`),

  // Validate workflow
  validate: (workflow: Partial<Workflow>) =>
    api.post<{ valid: boolean; error?: string }>(`/workflows/${workflow.id}/validate`, workflow),

  // Execute workflow
  execute: (id: string, triggerData?: Record<string, unknown>) =>
    api.post<WorkflowExecution>(`/workflows/${id}/execute`, { trigger_data: triggerData }),

  // List executions
  listExecutions: (workflowId: string, params?: { offset?: number; limit?: number }) =>
    api.get<ListExecutionsResponse>(`/workflows/${workflowId}/executions`, { params }),

  // Get execution
  getExecution: (workflowId: string, executionId: string) =>
    api.get<WorkflowExecution>(`/workflows/${workflowId}/executions/${executionId}`),

  // Cancel execution
  cancelExecution: (workflowId: string, executionId: string) =>
    api.post<{ status: string }>(`/workflows/${workflowId}/executions/${executionId}/cancel`),

  // Retry execution
  retryExecution: (workflowId: string, executionId: string) =>
    api.post<WorkflowExecution>(`/workflows/${workflowId}/executions/${executionId}/retry`),

  // Get execution logs
  getExecutionLogs: (workflowId: string, executionId: string, params?: { offset?: number; limit?: number }) =>
    api.get<ListLogsResponse>(`/workflows/${workflowId}/executions/${executionId}/logs`, { params }),

  // Get stats
  getStats: () => api.get<WorkflowStats>('/workflows/stats'),

  // Get templates
  getTemplates: () => api.get<WorkflowTemplate[]>('/workflows/templates'),
}
