import api from './client'

export type ProtocolKind = 'acp' | 'a2a'

export interface AgentProfile {
  id: string
  protocol: ProtocolKind
  name: string
  title?: string
  description?: string
  builtin?: boolean
  command?: string[]
  env?: Record<string, string>
  cwd?: string
  card_url?: string
  endpoint_url?: string
  headers?: Record<string, string>
  credential_provider_id?: string
  auth_method_id?: string
  metadata?: Record<string, unknown>
  health_status?: string
  health_message?: string
  last_verified_at?: string
  last_health_at?: string
  created_at?: string
  updated_at?: string
}

export interface ProfileVerifyResult {
  ok: boolean
  message?: string
  capabilities?: string[]
  details?: Record<string, unknown>
}

export interface ProfileHealthResult {
  healthy: boolean
  message?: string
  details?: Record<string, unknown>
}

export const agentSessionsApi = {
  listProfiles: () => api.get<{ profiles: AgentProfile[] }>('/agent-sessions/profiles'),
  saveProfile: (profile: AgentProfile) =>
    api.post<AgentProfile>('/agent-sessions/profiles', profile),
  verifyProfile: (payload: { id?: string; profile?: Partial<AgentProfile> }) =>
    api.post<ProfileVerifyResult>('/agent-sessions/profiles/verify', payload),
  healthProfile: (id: string) =>
    api.post<ProfileHealthResult>(`/agent-sessions/profiles/${encodeURIComponent(id)}/health`),
}
