import api from './client'

// Connection types
export type ConnectionType = 'http' | 'websocket' | 'sse'

export interface GeoLocation {
  country?: string
  country_code?: string
  region?: string
  city?: string
  latitude?: number
  longitude?: number
  timezone?: string
  isp?: string
  is_private: boolean
}

export interface Connection {
  id: string
  type: ConnectionType
  client_ip: string
  user_agent?: string
  path: string
  method?: string
  connected_at: string
  last_activity: string
  request_count: number
  bytes_sent: number
  bytes_recv: number
  status: 'active' | 'idle' | 'completed' | 'closed'
  metadata?: Record<string, unknown>
  geo_location?: GeoLocation
}

export interface ConnectionStats {
  total_connections: number
  active_http: number
  active_websocket: number
  active_sse: number
  total_requests: number
  total_bytes_sent: number
  total_bytes_recv: number
  connections_by_ip: Record<string, number>
  avg_request_duration: number
  last_updated: string
}

export interface ConnectionListResponse {
  connections: Connection[]
  total: number
}

export interface ConnectionGeoResponse {
  connection_id: string
  client_ip: string
  ip_type: string
  platform: string
  geo_location: GeoLocation
}

export interface IPLookupResponse {
  ip: string
  ip_type: string
  geo_location: GeoLocation
}

// API functions
export async function getActiveConnections(type?: ConnectionType): Promise<ConnectionListResponse> {
  const params = type ? { type } : {}
  const response = await api.get<ConnectionListResponse>('/connections/active', { params })
  return response.data
}

export async function getConnectionStats(): Promise<ConnectionStats> {
  const response = await api.get<ConnectionStats>('/connections/stats')
  return response.data
}

export async function getConnection(id: string): Promise<Connection> {
  const response = await api.get<Connection>(`/connections/${id}`)
  return response.data
}

// Get geographic location for a specific connection (on-demand)
export async function getConnectionGeo(id: string): Promise<ConnectionGeoResponse> {
  const response = await api.get<ConnectionGeoResponse>(`/connections/${id}/geo`)
  return response.data
}

// Lookup geographic location for any IP address (on-demand)
export async function lookupIPGeo(ip: string): Promise<IPLookupResponse> {
  const response = await api.post<IPLookupResponse>('/connections/geo/lookup', { ip })
  return response.data
}
