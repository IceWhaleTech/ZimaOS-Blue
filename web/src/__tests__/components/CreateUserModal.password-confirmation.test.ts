import { mount, flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import CreateUserModal from '@/components/users/CreateUserModal.vue'
import { i18n } from '@/i18n'

const mocks = vi.hoisted(() => ({
  createUser: vi.fn(),
  getPasswordPolicy: vi.fn(),
  getAvailablePermissions: vi.fn(),
}))

vi.mock('@/api/auth', () => ({
  authApi: {
    getPasswordPolicy: mocks.getPasswordPolicy,
  },
}))

vi.mock('@/api/users', () => ({
  usersApi: {
    create: mocks.createUser,
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

function mountModal() {
  return mount(CreateUserModal, {
    global: {
      plugins: [i18n],
      stubs: {
        Teleport: true,
      },
    },
  })
}

describe('CreateUserModal password confirmation', () => {
  beforeEach(() => {
    mocks.createUser.mockReset().mockResolvedValue({ data: {} })
    mocks.getPasswordPolicy.mockReset().mockResolvedValue({
      data: {
        min_length: 6,
        require_uppercase: false,
        require_lowercase: false,
        require_letter: true,
        require_number: true,
        require_special: true,
      },
    })
    mocks.getAvailablePermissions.mockReset().mockResolvedValue({
      data: {
        permissions: [],
      },
    })
  })

  it('hides the confirm password field after showing the password', async () => {
    const wrapper = mountModal()

    await flushPromises()

    const confirmLabel = String(i18n.global.t('auth.confirmPassword'))
    expect(wrapper.text()).toContain(confirmLabel)

    await wrapper.find('div.relative button[type="button"]').trigger('click')

    expect(wrapper.text()).not.toContain(confirmLabel)
  })

  it('submits with a visible valid password without requiring confirmation', async () => {
    const wrapper = mountModal()

    await flushPromises()

    const inputs = wrapper.findAll('input')
    await inputs[0]!.setValue('new-user')

    await wrapper.find('div.relative button[type="button"]').trigger('click')

    const visibleInputs = wrapper.findAll('input')
    await visibleInputs[2]!.setValue('ValidPass1!')

    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    expect(wrapper.text()).not.toContain(String(i18n.global.t('auth.passwordMismatch')))
    expect(mocks.createUser).toHaveBeenCalledWith({
      username: 'new-user',
      email: undefined,
      password: 'ValidPass1!',
      role: 'user',
      permissions: ['chat', 'profile', 'home'],
    })
  })

  it('respects uppercase and lowercase requirements before submitting', async () => {
    mocks.getPasswordPolicy.mockResolvedValueOnce({
      data: {
        min_length: 6,
        require_uppercase: true,
        require_lowercase: true,
        require_letter: true,
        require_number: true,
        require_special: true,
      },
    })

    const wrapper = mountModal()

    await flushPromises()

    const inputs = wrapper.findAll('input')
    await inputs[0]!.setValue('new-user')

    await wrapper.find('div.relative button[type="button"]').trigger('click')

    let visibleInputs = wrapper.findAll('input')
    await visibleInputs[2]!.setValue('alllower1!')

    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    expect(wrapper.text()).toContain(String(i18n.global.t('preview.passwordCheck.uppercase')))
    expect(mocks.createUser).not.toHaveBeenCalled()

    visibleInputs = wrapper.findAll('input')
    await visibleInputs[2]!.setValue('ValidPass1!')

    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    expect(mocks.createUser).toHaveBeenCalledTimes(1)
  })
})
