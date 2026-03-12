import { mount, flushPromises } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import CreateUserModal from '@/components/users/CreateUserModal.vue'
import { i18n } from '@/i18n'

const mocks = vi.hoisted(() => ({
  getPasswordPolicy: vi.fn().mockResolvedValue({
    data: {
      min_length: 6,
      require_uppercase: false,
      require_lowercase: false,
      require_letter: true,
      require_number: true,
      require_special: true,
    },
  }),
  getAvailablePermissions: vi.fn().mockResolvedValue({
    data: {
      permissions: [],
    },
  }),
}))

vi.mock('@/api/auth', () => ({
  authApi: {
    getPasswordPolicy: mocks.getPasswordPolicy,
  },
}))

vi.mock('@/api/users', () => ({
  usersApi: {
    create: vi.fn(),
  },
  permissionsApi: {
    getAvailablePermissions: mocks.getAvailablePermissions,
  },
  PagePermissions: {
    CHAT: 'chat',
    PROFILE: 'profile',
    HOME: 'home',
  },
}))

describe('CreateUserModal password confirmation', () => {
  it('keeps the confirm password field visible after showing the password', async () => {
    const wrapper = mount(CreateUserModal, {
      global: {
        plugins: [i18n],
        stubs: {
          Teleport: true,
        },
      },
    })

    await flushPromises()

    const confirmLabel = String(i18n.global.t('auth.confirmPassword'))
    expect(wrapper.text()).toContain(confirmLabel)

    await wrapper.find('div.relative button[type="button"]').trigger('click')

    expect(wrapper.text()).toContain(confirmLabel)
  })
})
