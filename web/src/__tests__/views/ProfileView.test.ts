import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const state = vi.hoisted(() => {
  let fetchApiKeysResolve: (() => void) | null = null
  let mfaModuleLoads = 0
  let webAuthnModuleLoads = 0

  const fetchUser = vi.fn(async () => {})
  const fetchApiKeys = vi.fn(
    () =>
      new Promise<void>((resolve) => {
        fetchApiKeysResolve = resolve
      })
  )

  const authStore = {
    user: {
      username: 'orca',
      email: 'orca@example.com',
      role: 'admin',
    },
    apiKeys: [],
    loading: false,
    error: null as string | null,
    fetchUser,
    fetchApiKeys,
    updateProfile: vi.fn(async () => true),
    createApiKey: vi.fn(async () => null),
    deleteApiKey: vi.fn(async () => true),
  }

  const extauthApi = {
    getLinkedAccounts: vi.fn(async () => ({ data: [] })),
    listProviders: vi.fn(async () => ({ data: [] })),
    unlinkAccount: vi.fn(async () => ({})),
  }

  return {
    authStore,
    extauthApi,
    fetchUser,
    fetchApiKeys,
    getMfaModuleLoads() {
      return mfaModuleLoads
    },
    getWebAuthnModuleLoads() {
      return webAuthnModuleLoads
    },
    trackMfaModuleLoad() {
      mfaModuleLoads += 1
    },
    trackWebAuthnModuleLoad() {
      webAuthnModuleLoads += 1
    },
    reset() {
      fetchApiKeysResolve = null
      mfaModuleLoads = 0
      webAuthnModuleLoads = 0
      authStore.user = {
        username: 'orca',
        email: 'orca@example.com',
        role: 'admin',
      }
      authStore.apiKeys = []
      authStore.loading = false
      authStore.error = null
      fetchUser.mockClear()
      fetchApiKeys.mockClear()
      authStore.updateProfile.mockClear()
      authStore.createApiKey.mockClear()
      authStore.deleteApiKey.mockClear()
      extauthApi.getLinkedAccounts.mockClear()
      extauthApi.listProviders.mockClear()
      extauthApi.unlinkAccount.mockClear()
      extauthApi.getLinkedAccounts.mockResolvedValue({ data: [] })
      extauthApi.listProviders.mockResolvedValue({ data: [] })
    },
    resolveFetchApiKeys() {
      const resolve = fetchApiKeysResolve
      fetchApiKeysResolve = null
      resolve?.()
    },
  }
})

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => state.authStore,
}))

vi.mock('@/api/extauth', () => ({
  extauthApi: state.extauthApi,
  getProviderDisplayName: (type: string) => type,
}))

vi.mock('@/components/MFASettings.vue', () => {
  state.trackMfaModuleLoad()
  return {
    default: { name: 'MFASettings', template: '<div class="mfa-settings-stub" />' },
  }
})

vi.mock('@/components/WebAuthnSettings.vue', () => {
  state.trackWebAuthnModuleLoad()
  return {
    default: { name: 'WebAuthnSettings', template: '<div class="webauthn-settings-stub" />' },
  }
})

async function loadProfileView() {
  return (await import('@/views/ProfileView.vue')).default
}

describe('ProfileView', () => {
  beforeEach(() => {
    vi.resetModules()
    vi.clearAllMocks()
    state.reset()
    vi.stubGlobal('confirm', vi.fn(() => true))
  })

  it('does not eagerly load the security subpanels when the profile route module is imported', async () => {
    await loadProfileView()

    expect(state.getMfaModuleLoads()).toBe(0)
    expect(state.getWebAuthnModuleLoads()).toBe(0)
  })

  it('does not refetch the user and starts profile detail requests in parallel when user data is already hydrated', async () => {
    const ProfileView = await loadProfileView()

    const wrapper = mount(ProfileView, {
      global: {
        mocks: {
          $t: (key: string) => key,
        },
        stubs: {
          RouterLink: {
            template: '<a><slot /></a>',
          },
        },
      },
    })

    await Promise.resolve()
    await Promise.resolve()

    expect(state.fetchUser).not.toHaveBeenCalled()
    expect(state.fetchApiKeys).toHaveBeenCalledTimes(1)
    expect(state.extauthApi.getLinkedAccounts).toHaveBeenCalledTimes(1)
    expect(state.extauthApi.listProviders).toHaveBeenCalledTimes(1)

    state.resolveFetchApiKeys()
    await flushPromises()

    wrapper.unmount()
  })
})
