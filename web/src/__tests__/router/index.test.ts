import { beforeEach, describe, expect, it, vi } from 'vitest'

const state = vi.hoisted(() => {
  let token = 'test-access-token'
  let fetchUserResolve: (() => void) | null = null

  const fetchUser = vi.fn(
    () =>
      new Promise<void>((resolve) => {
        fetchUserResolve = resolve
      })
  )

  const authStore = {
    user: null as { role?: string } | null,
    isAdmin: false,
    clearAuth: vi.fn(),
    hasPermission: vi.fn(() => true),
    hasAnyPermission: vi.fn(() => true),
    fetchUser,
  }

  const previewStore = {
    initialize: vi.fn(async () => {}),
  }

  return {
    authStore,
    previewStore,
    fetchUser,
    getToken: () => token,
    reset() {
      token = 'test-access-token'
      fetchUserResolve = null
      fetchUser.mockClear()
      authStore.user = null
      authStore.isAdmin = false
      authStore.clearAuth.mockClear()
      authStore.hasPermission.mockReset()
      authStore.hasPermission.mockReturnValue(true)
      authStore.hasAnyPermission.mockReset()
      authStore.hasAnyPermission.mockReturnValue(true)
      previewStore.initialize.mockClear()
    },
    resolveFetchUser() {
      const resolve = fetchUserResolve
      fetchUserResolve = null
      resolve?.()
    },
    setToken(value: string | null) {
      token = value ?? ''
    },
  }
})

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => state.authStore,
}))

vi.mock('@/stores/preview', () => ({
  usePreviewStore: () => state.previewStore,
}))

vi.mock('@/utils/authStorage', () => ({
  clearStoredAccessToken: vi.fn(() => state.setToken(null)),
  clearStoredAuthSession: vi.fn(),
  clearStoredPreviewToken: vi.fn(),
  clearStoredRefreshToken: vi.fn(),
  getStoredAccessToken: vi.fn(() => state.getToken()),
  getStoredPreviewToken: vi.fn(() => null),
  hasStoredSessionHint: vi.fn(() => true),
  setStoredAccessToken: vi.fn((value: string) => state.setToken(value)),
  setStoredPreviewToken: vi.fn(),
  syncAuthSessionStorage: vi.fn(),
}))

vi.mock('@/utils/startupTrace', () => ({
  reportStartupMark: vi.fn(),
}))

vi.mock('@/views/SettingsView.vue', () => ({
  default: { template: '<div>Settings</div>' },
}))

vi.mock('@/views/HomeView.vue', () => ({
  default: { template: '<div>Home</div>' },
}))

describe('router guard hydration behavior', () => {
  beforeEach(() => {
    vi.resetModules()
    state.reset()
    window.history.replaceState({}, '', '/profile')
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => ({
        ok: false,
        status: 404,
        json: async () => ({}),
      }))
    )
  })

  it('does not block non-admin authenticated navigation on pending user hydration', async () => {
    const { default: router } = await import('@/router')

    const navigationPromise = router.push('/settings')

    await vi.waitFor(() => {
      expect(state.fetchUser).toHaveBeenCalledTimes(1)
    })
    await vi.waitFor(() => {
      expect(router.currentRoute.value.path).toBe('/settings')
    })

    state.authStore.user = { role: 'user' }
    state.resolveFetchUser()
    await navigationPromise
    await Promise.resolve()
  })

  it('still waits for user hydration on admin-only routes', async () => {
    const { default: router } = await import('@/router')

    const navigationPromise = router.push('/home')

    await vi.waitFor(() => {
      expect(state.fetchUser).toHaveBeenCalledTimes(1)
    })

    expect(router.currentRoute.value.path).not.toBe('/home')

    state.authStore.user = { role: 'admin' }
    state.authStore.isAdmin = true
    state.resolveFetchUser()
    await navigationPromise

    expect(router.currentRoute.value.path).toBe('/home')
  })
})
