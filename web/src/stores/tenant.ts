import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { Tenant, TenantMember, TenantSettings, TenantLimits } from '@/api/tenant'
import * as tenantApi from '@/api/tenant'

export const useTenantStore = defineStore('tenant', () => {
  // State
  const tenants = ref<Tenant[]>([])
  const currentTenant = ref<Tenant | null>(null)
  const currentTenantId = ref<string | null>(localStorage.getItem('current_tenant_id'))
  const members = ref<TenantMember[]>([])
  const settings = ref<TenantSettings | null>(null)
  const limits = ref<TenantLimits | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)

  // Computed
  const hasTenants = computed(() => tenants.value.length > 0)
  const isOwner = computed(() => {
    if (!currentTenant.value) return false
    const userId = localStorage.getItem('user_id')
    return currentTenant.value.owner_id === userId
  })

  // Actions
  async function loadTenants() {
    loading.value = true
    error.value = null

    try {
      const response = await tenantApi.listTenants()
      tenants.value = response.tenants

      // Auto-select first tenant if none selected
      if (!currentTenantId.value && tenants.value.length > 0) {
        const firstTenant = tenants.value[0]
        if (firstTenant) {
          await selectTenant(firstTenant.id)
        }
      } else if (currentTenantId.value) {
        // Load current tenant details
        const tenant = tenants.value.find((t) => t.id === currentTenantId.value)
        if (tenant) {
          currentTenant.value = tenant
        } else {
          // Tenant no longer accessible, select first available
          if (tenants.value.length > 0) {
            const firstTenant = tenants.value[0]
            if (firstTenant) {
              await selectTenant(firstTenant.id)
            }
          } else {
            currentTenantId.value = null
            currentTenant.value = null
            localStorage.removeItem('current_tenant_id')
          }
        }
      }
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to load tenants'
    } finally {
      loading.value = false
    }
  }

  async function selectTenant(tenantId: string) {
    loading.value = true
    error.value = null

    try {
      const tenant = await tenantApi.getTenant(tenantId)
      currentTenant.value = tenant
      currentTenantId.value = tenantId
      localStorage.setItem('current_tenant_id', tenantId)

      // Reset related data
      members.value = []
      settings.value = null
      limits.value = null
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to select tenant'
    } finally {
      loading.value = false
    }
  }

  async function createTenant(request: tenantApi.CreateTenantRequest) {
    loading.value = true
    error.value = null

    try {
      const tenant = await tenantApi.createTenant(request)
      tenants.value.push(tenant)
      await selectTenant(tenant.id)
      return tenant
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to create tenant'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function updateTenant(tenantId: string, request: tenantApi.UpdateTenantRequest) {
    loading.value = true
    error.value = null

    try {
      const tenant = await tenantApi.updateTenant(tenantId, request)
      const index = tenants.value.findIndex((t) => t.id === tenantId)
      if (index !== -1) {
        tenants.value[index] = tenant
      }
      if (currentTenant.value?.id === tenantId) {
        currentTenant.value = tenant
      }
      return tenant
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to update tenant'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function deleteTenant(tenantId: string) {
    loading.value = true
    error.value = null

    try {
      await tenantApi.deleteTenant(tenantId)
      tenants.value = tenants.value.filter((t) => t.id !== tenantId)

      if (currentTenant.value?.id === tenantId) {
        if (tenants.value.length > 0) {
          const firstTenant = tenants.value[0]
          if (firstTenant) {
            await selectTenant(firstTenant.id)
          }
        } else {
          currentTenant.value = null
          currentTenantId.value = null
          localStorage.removeItem('current_tenant_id')
        }
      }
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to delete tenant'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function loadMembers(page = 1, pageSize = 20) {
    if (!currentTenantId.value) return

    loading.value = true
    error.value = null

    try {
      const response = await tenantApi.listMembers(currentTenantId.value, page, pageSize)
      members.value = response.members
      return response
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to load members'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function inviteMember(email: string, role: tenantApi.MemberRole) {
    if (!currentTenantId.value) return

    loading.value = true
    error.value = null

    try {
      const invitation = await tenantApi.inviteMember(currentTenantId.value, { email, role })
      return invitation
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to invite member'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function updateMemberRole(userId: string, role: tenantApi.MemberRole) {
    if (!currentTenantId.value) return

    loading.value = true
    error.value = null

    try {
      await tenantApi.updateMember(currentTenantId.value, userId, { role })
      const member = members.value.find((m) => m.user_id === userId)
      if (member) {
        member.role = role
      }
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to update member'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function removeMember(userId: string) {
    if (!currentTenantId.value) return

    loading.value = true
    error.value = null

    try {
      await tenantApi.removeMember(currentTenantId.value, userId)
      members.value = members.value.filter((m) => m.user_id !== userId)
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to remove member'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function loadSettings() {
    if (!currentTenantId.value) return

    loading.value = true
    error.value = null

    try {
      settings.value = await tenantApi.getTenantSettings(currentTenantId.value)
      return settings.value
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to load settings'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function updateSettings(newSettings: TenantSettings) {
    if (!currentTenantId.value) return

    loading.value = true
    error.value = null

    try {
      await tenantApi.updateTenantSettings(currentTenantId.value, newSettings)
      settings.value = newSettings
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to update settings'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function loadLimits() {
    if (!currentTenantId.value) return

    loading.value = true
    error.value = null

    try {
      limits.value = await tenantApi.getTenantLimits(currentTenantId.value)
      return limits.value
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to load limits'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function transferOwnership(newOwnerId: string) {
    if (!currentTenantId.value) return

    loading.value = true
    error.value = null

    try {
      await tenantApi.transferOwnership(currentTenantId.value, newOwnerId)
      // Reload tenant to get updated owner
      await selectTenant(currentTenantId.value)
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to transfer ownership'
      throw err
    } finally {
      loading.value = false
    }
  }

  return {
    // State
    tenants,
    currentTenant,
    currentTenantId,
    members,
    settings,
    limits,
    loading,
    error,

    // Computed
    hasTenants,
    isOwner,

    // Actions
    loadTenants,
    selectTenant,
    createTenant,
    updateTenant,
    deleteTenant,
    loadMembers,
    inviteMember,
    updateMemberRole,
    removeMember,
    loadSettings,
    updateSettings,
    loadLimits,
    transferOwnership,
  }
})
