import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

type MockMaskingRule = {
  id: string
  name: string
  category: 'pii' | 'credentials' | 'financial' | 'custom'
  pattern: string
  replacement: string
  direction: 'request' | 'response' | 'both'
  enabled: boolean
}

let rules: MockMaskingRule[] = []
let defaultRules: MockMaskingRule[] = []
let stats = {
  enabled: true,
  rule_count: 0,
  total_masks: 0,
  mask_counts: {},
  status: 'enabled',
}

const storageState = new Map<string, string>()
const localStorageMock = {
  getItem: (key: string) => storageState.get(key) ?? null,
  setItem: (key: string, value: string) => {
    storageState.set(key, value)
  },
  removeItem: (key: string) => {
    storageState.delete(key)
  },
  clear: () => {
    storageState.clear()
  },
}

vi.mock('@/api/proxy', () => ({
  proxyApi: {
    getMaskingStats: vi.fn(async () => ({ data: stats })),
    getMaskingRules: vi.fn(async () => ({ data: { rules, default_rules: defaultRules } })),
    toggleMasking: vi.fn(async (enabled: boolean) => ({
      data: {
        ...stats,
        enabled,
      },
    })),
    updateMaskingRule: vi.fn(async (id: string, patch: { enabled: boolean }) => {
      rules = rules.map((rule) => (rule.id === id ? { ...rule, enabled: patch.enabled } : rule))
      const rule = rules.find((item) => item.id === id)
      return {
        data: {
          message: 'updated',
          rule,
          stats,
        },
      }
    }),
    addMaskingRule: vi.fn(async (rule: MockMaskingRule) => {
      rules = [...rules, rule]
      stats = { ...stats, rule_count: rules.length }
      return { data: { message: 'created', rule } }
    }),
    removeMaskingRule: vi.fn(async (id: string) => {
      rules = rules.filter((rule) => rule.id !== id)
      stats = { ...stats, rule_count: rules.length }
      return { data: { message: 'deleted' } }
    }),
  },
}))

import { i18n, setLocale } from '@/i18n'
import { proxyApi } from '@/api/proxy'
import DataMaskingSettings from '@/components/security/DataMaskingSettings.vue'

describe('DataMaskingSettings', () => {
  beforeEach(async () => {
    vi.clearAllMocks()
    storageState.clear()
    vi.stubGlobal('localStorage', localStorageMock)
    if (typeof window !== 'undefined') {
      Object.defineProperty(window, 'localStorage', {
        value: localStorageMock,
        configurable: true,
      })
    }

    defaultRules = [
      {
        id: 'email',
        name: 'Email Address',
        category: 'pii',
        pattern: '[a-z]+@example.com',
        replacement: '【{MASKED}】[EMAIL]',
        direction: 'response',
        enabled: true,
      },
    ]

    rules = [...defaultRules]
    stats = {
      enabled: true,
      rule_count: rules.length,
      total_masks: 0,
      mask_counts: {},
      status: 'enabled',
    }

    await setLocale('en-US')
  })

  it('adds a custom masking rule from the security settings form', async () => {
    const wrapper = mount(DataMaskingSettings, {
      global: {
        plugins: [i18n],
      },
    })

    await flushPromises()

    await wrapper.get('[data-testid="masking-custom-name"]').setValue('Internal Ticket')
    await wrapper.get('[data-testid="masking-custom-direction"]').setValue('both')
    await wrapper.get('[data-testid="masking-custom-pattern"]').setValue('TKT-\\d{6}')
    await wrapper.get('[data-testid="masking-custom-replacement"]').setValue('[TICKET]')
    await wrapper.get('[data-testid="masking-add-rule"]').trigger('click')

    await flushPromises()
    await flushPromises()

    expect(proxyApi.addMaskingRule).toHaveBeenCalledWith(
      expect.objectContaining({
        name: 'Internal Ticket',
        category: 'custom',
        pattern: 'TKT-\\d{6}',
        replacement: '[TICKET]',
        direction: 'both',
        enabled: true,
        id: expect.stringMatching(/^custom-internal-ticket-/),
      })
    )
    expect(wrapper.text()).toContain('Internal Ticket')
    expect(wrapper.text()).toContain('Custom rules')
  })

  it('localizes built-in masking rule labels for zh-CN', async () => {
    await setLocale('zh-CN')

    const wrapper = mount(DataMaskingSettings, {
      global: {
        plugins: [i18n],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('邮箱地址')
    expect(wrapper.text()).toContain('个人信息')
    expect(wrapper.text()).toContain('响应')
    expect(wrapper.text()).not.toContain('Email Address')
    expect(wrapper.text()).not.toContain('pii')
    expect(wrapper.text()).not.toContain('response')
  })
})
