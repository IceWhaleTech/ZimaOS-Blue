import api from './index'

// Types
export type TenantStatus = 'active' | 'suspended' | 'deleted'
export type MemberRole = 'owner' | 'admin' | 'member'

export interface Tenant {
  id: string
  name: string
  slug: string
  description?: string
  status: TenantStatus
  settings?: string
  limits?: string
  owner_id: string
  created_at: string
  updated_at: string
}

export interface TenantMember {
  id: string
  tenant_id: string
  user_id: string
  role: MemberRole
  joined_at: string
  invited_by?: string
  username: string
  email?: string
}

export interface TenantInvitation {
  id: string
  tenant_id: string
  email: string
  role: MemberRole
  invited_by: string
  expires_at: string
  accepted_at?: string
  created_at: string
}

export interface TenantLimits {
  max_users: number
  max_storage: number
  max_api_requests: number
  max_workflows: number
  max_channels: number
}

export interface TenantSettings {
  default_language: string
  timezone: string
  features: Record<string, boolean>
}

export interface CreateTenantRequest {
  name: string
  slug: string
  description?: string
}

export interface UpdateTenantRequest {
  name?: string
  description?: string
  status?: TenantStatus
}

export interface InviteMemberRequest {
  email: string
  role: MemberRole
}

export interface UpdateMemberRequest {
  role: MemberRole
}

export interface ListTenantsResponse {
  tenants: Tenant[]
  total: number
}

export interface ListMembersResponse {
  members: TenantMember[]
  total: number
  page: number
  page_size: number
  total_pages: number
}

export interface ListInvitationsResponse {
  invitations: TenantInvitation[]
  total: number
}

// API functions
export async function listTenants(): Promise<ListTenantsResponse> {
  const response = await api.get('/api/v1/tenants')
  return response.data
}

export async function getTenant(tenantId: string): Promise<Tenant> {
  const response = await api.get(`/api/v1/tenants/${tenantId}`)
  return response.data
}

export async function createTenant(request: CreateTenantRequest): Promise<Tenant> {
  const response = await api.post('/api/v1/tenants', request)
  return response.data
}

export async function updateTenant(tenantId: string, request: UpdateTenantRequest): Promise<Tenant> {
  const response = await api.put(`/api/v1/tenants/${tenantId}`, request)
  return response.data
}

export async function deleteTenant(tenantId: string): Promise<void> {
  await api.delete(`/api/v1/tenants/${tenantId}`)
}

// Members
export async function listMembers(
  tenantId: string,
  page = 1,
  pageSize = 20,
  role?: MemberRole
): Promise<ListMembersResponse> {
  const params = new URLSearchParams({
    page: page.toString(),
    page_size: pageSize.toString(),
  })
  if (role) {
    params.append('role', role)
  }
  const response = await api.get(`/api/v1/tenants/${tenantId}/members?${params}`)
  return response.data
}

export async function updateMember(
  tenantId: string,
  userId: string,
  request: UpdateMemberRequest
): Promise<void> {
  await api.put(`/api/v1/tenants/${tenantId}/members/${userId}`, request)
}

export async function removeMember(tenantId: string, userId: string): Promise<void> {
  await api.delete(`/api/v1/tenants/${tenantId}/members/${userId}`)
}

// Invitations
export async function inviteMember(
  tenantId: string,
  request: InviteMemberRequest
): Promise<TenantInvitation> {
  const response = await api.post(`/api/v1/tenants/${tenantId}/invitations`, request)
  return response.data
}

export async function listInvitations(tenantId: string): Promise<ListInvitationsResponse> {
  const response = await api.get(`/api/v1/tenants/${tenantId}/invitations`)
  return response.data
}

export async function cancelInvitation(tenantId: string, invitationId: string): Promise<void> {
  await api.delete(`/api/v1/tenants/${tenantId}/invitations/${invitationId}`)
}

export async function acceptInvitation(token: string): Promise<Tenant> {
  const response = await api.post('/api/v1/invitations/accept', { token })
  return response.data
}

// Settings and Limits
export async function getTenantSettings(tenantId: string): Promise<TenantSettings> {
  const response = await api.get(`/api/v1/tenants/${tenantId}/settings`)
  return response.data
}

export async function updateTenantSettings(
  tenantId: string,
  settings: TenantSettings
): Promise<void> {
  await api.put(`/api/v1/tenants/${tenantId}/settings`, settings)
}

export async function getTenantLimits(tenantId: string): Promise<TenantLimits> {
  const response = await api.get(`/api/v1/tenants/${tenantId}/limits`)
  return response.data
}

export async function updateTenantLimits(tenantId: string, limits: TenantLimits): Promise<void> {
  await api.put(`/api/v1/tenants/${tenantId}/limits`, limits)
}

// Ownership
export async function transferOwnership(tenantId: string, newOwnerId: string): Promise<void> {
  await api.post(`/api/v1/tenants/${tenantId}/transfer`, { new_owner_id: newOwnerId })
}

// Helper functions
export function getRoleLabel(role: MemberRole): string {
  const labels: Record<MemberRole, string> = {
    owner: 'Owner',
    admin: 'Admin',
    member: 'Member',
  }
  return labels[role] || role
}

export function getRoleColor(role: MemberRole): string {
  const colors: Record<MemberRole, string> = {
    owner: '#8b5cf6',
    admin: '#3b82f6',
    member: '#6b7280',
  }
  return colors[role] || '#6b7280'
}

export function getStatusLabel(status: TenantStatus): string {
  const labels: Record<TenantStatus, string> = {
    active: 'Active',
    suspended: 'Suspended',
    deleted: 'Deleted',
  }
  return labels[status] || status
}

export function getStatusColor(status: TenantStatus): string {
  const colors: Record<TenantStatus, string> = {
    active: '#22c55e',
    suspended: '#f59e0b',
    deleted: '#ef4444',
  }
  return colors[status] || '#6b7280'
}

export function formatStorageSize(bytes: number): string {
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let size = bytes
  let unitIndex = 0

  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024
    unitIndex++
  }

  return `${size.toFixed(1)} ${units[unitIndex]}`
}
