import { mount, flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import PreviewUpgradeForm from '@/components/preview/PreviewUpgradeForm.vue'
import { i18n } from '@/i18n'

const mocks = vi.hoisted(() => ({
  upgrade: vi.fn(),
  login: vi.fn(),
  getPasswordPolicy: vi.fn(),
}))

vi.mock('@/stores/preview', () => ({
  usePreviewStore: () => ({
    upgrade: mocks.upgrade,
  }),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    login: mocks.login,
  }),
}))

vi.mock('@/api/auth', () => ({
  authApi: {
    getPasswordPolicy: mocks.getPasswordPolicy,
  },
}))

function mountForm() {
  return mount(PreviewUpgradeForm, {
    global: {
      plugins: [i18n],
      stubs: {
        Teleport: true,
      },
    },
  })
}

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((res) => {
    resolve = res
  })
  return { promise, resolve }
}

describe('PreviewUpgradeForm password confirmation', () => {
  beforeEach(() => {
    mocks.upgrade.mockReset().mockResolvedValue({ success: true })
    mocks.login.mockReset().mockResolvedValue(true)
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
  })

  it('hides the confirm password field after showing the password', async () => {
    const wrapper = mountForm()

    await flushPromises()

    const confirmLabel = String(i18n.global.t('auth.confirmPassword'))
    expect(wrapper.text()).toContain(confirmLabel)

    await wrapper.find('div.relative button[type="button"]').trigger('click')

    expect(wrapper.text()).not.toContain(confirmLabel)
  })

  it('submits with a visible valid password without requiring confirmation', async () => {
    const wrapper = mountForm()

    await flushPromises()

    const inputs = wrapper.findAll('input')
    await inputs[0]!.setValue('preview-admin')

    await wrapper.find('div.relative button[type="button"]').trigger('click')

    const visibleInputs = wrapper.findAll('input')
    await visibleInputs[1]!.setValue('ValidPass1!')

    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    expect(wrapper.text()).not.toContain(String(i18n.global.t('auth.passwordMismatch')))
    expect(mocks.upgrade).toHaveBeenCalledWith('preview-admin', 'ValidPass1!')
    expect(mocks.login).toHaveBeenCalledWith('preview-admin', 'ValidPass1!')
  })

  it('does not submit before the password policy finishes loading', async () => {
    const policyRequest = deferred<{
      data: {
        min_length: number
        require_uppercase: boolean
        require_lowercase: boolean
        require_letter: boolean
        require_number: boolean
        require_special: boolean
      }
    }>()
    mocks.getPasswordPolicy.mockReset().mockReturnValueOnce(policyRequest.promise)

    const wrapper = mountForm()

    const inputs = wrapper.findAll('input')
    await inputs[0]!.setValue('preview-admin')

    await wrapper.find('div.relative button[type="button"]').trigger('click')

    const visibleInputs = wrapper.findAll('input')
    await visibleInputs[1]!.setValue('ValidPass1!')

    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    expect(mocks.upgrade).not.toHaveBeenCalled()

    policyRequest.resolve({
      data: {
        min_length: 6,
        require_uppercase: false,
        require_lowercase: false,
        require_letter: true,
        require_number: true,
        require_special: true,
      },
    })
    await flushPromises()

    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    expect(mocks.upgrade).toHaveBeenCalledWith('preview-admin', 'ValidPass1!')
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

    const wrapper = mountForm()

    await flushPromises()

    const inputs = wrapper.findAll('input')
    await inputs[0]!.setValue('preview-admin')

    await wrapper.find('div.relative button[type="button"]').trigger('click')

    let visibleInputs = wrapper.findAll('input')
    await visibleInputs[1]!.setValue('alllower1!')

    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    expect(wrapper.text()).toContain(String(i18n.global.t('preview.passwordCheck.uppercase')))
    expect(mocks.upgrade).not.toHaveBeenCalled()

    visibleInputs = wrapper.findAll('input')
    await visibleInputs[1]!.setValue('ValidPass1!')

    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    expect(mocks.upgrade).toHaveBeenCalledTimes(1)
  })
})
