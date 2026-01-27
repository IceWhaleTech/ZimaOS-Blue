import api from './client'

export type WebhookType = 'incoming' | 'outgoing'

export interface Webhook {
  id: string
  name: string
  type: WebhookType
  description?: string
  url: string
  secret?: string
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface WebhookEvent {
  id: string
  webhook_id: string
  method: string
  path: string
  headers: Record<string, string>
  body?: string
  status_code: number
  response?: string
  processed_at: string
  duration_ms: number
}

export interface CreateWebhookRequest {
  name: string
  type: WebhookType
  description?: string
}

export interface UpdateWebhookRequest {
  name: string
  description?: string
  enabled: boolean
}

export const webhookApi = {
  // List webhooks
  list: () => api.get<Webhook[]>('/webhooks'),

  // Get webhook by ID
  get: (id: string) => api.get<Webhook>(`/webhooks/${id}`),

  // Create webhook
  create: (data: CreateWebhookRequest) => api.post<Webhook>('/webhooks', data),

  // Update webhook
  update: (id: string, data: UpdateWebhookRequest) =>
    api.put<{ status: string }>(`/webhooks/${id}`, data),

  // Delete webhook
  delete: (id: string) => api.delete<{ status: string }>(`/webhooks/${id}`),

  // Regenerate secret
  regenerateSecret: (id: string) =>
    api.post<{ secret: string }>(`/webhooks/${id}/regenerate-secret`),

  // Get webhook events
  getEvents: (id: string, limit = 20) =>
    api.get<WebhookEvent[]>(`/webhooks/${id}/events`, { params: { limit } }),
}
