import { describe, expect, it } from 'vitest'
import {
  filterProvidersVisibleInUI,
  isProviderVisibleInUI,
} from '@/utils/providerVisibility'

describe('providerVisibility', () => {
  it('treats oauth-backed providers as hidden in UI', () => {
    expect(isProviderVisibleInUI({ oauth: { connected: false } })).toBe(false)
    expect(isProviderVisibleInUI({ oauth: undefined })).toBe(true)
    expect(isProviderVisibleInUI(null)).toBe(true)
  })

  it('filters oauth-backed providers out of UI lists', () => {
    const providers = [
      { id: 'anthropic' },
      { id: 'google-antigravity', oauth: { connected: false } },
      { id: 'openai' },
    ]

    expect(filterProvidersVisibleInUI(providers).map((provider) => provider.id)).toEqual([
      'anthropic',
      'openai',
    ])
  })
})
