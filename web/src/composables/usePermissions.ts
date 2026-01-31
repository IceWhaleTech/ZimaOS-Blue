import { computed } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { PagePermissions, type PagePermission } from '@/api/users'

/**
 * Composable for checking user permissions
 */
export function usePermissions() {
  const authStore = useAuthStore()

  /**
   * Check if user has a specific permission
   */
  const hasPermission = (permission: PagePermission | string): boolean => {
    return authStore.hasPermission(permission)
  }

  /**
   * Check if user has any of the given permissions
   */
  const hasAnyPermission = (permissions: (PagePermission | string)[]): boolean => {
    return authStore.hasAnyPermission(permissions)
  }

  /**
   * Check if user has all of the given permissions
   */
  const hasAllPermissions = (permissions: (PagePermission | string)[]): boolean => {
    return authStore.hasAllPermissions(permissions)
  }

  /**
   * Check if user is admin
   */
  const isAdmin = computed(() => authStore.isAdmin)

  /**
   * Get all user permissions
   */
  const permissions = computed(() => authStore.permissions)

  /**
   * Page permission constants for convenience
   */
  const Pages = PagePermissions

  return {
    hasPermission,
    hasAnyPermission,
    hasAllPermissions,
    isAdmin,
    permissions,
    Pages,
  }
}
