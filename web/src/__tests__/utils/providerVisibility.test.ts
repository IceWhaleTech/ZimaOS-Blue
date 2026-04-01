import { describe, expect, it } from 'vitest'
import {
  filterProvidersVisibleInUI,
  isProviderVisibleInUI,
} from '@/utils/providerVisibility'

describe('providerVisibility', () => {
  it('keeps oauth-backed providers visible in UI', () => {
    expect(isProviderVisibleInUI({ oauth: { connected: false } })).toBe(true)
    expect(isProviderVisibleInUI({ oauth: undefined })).toBe(true)
    expect(isProviderVisibleInUI(null)).toBe(true)
  })

  it('hides unsupported official catalog providers until their setup flow is complete', () => {
    expect(
      isProviderVisibleInUI({
        id: 'bedrock',
        metadata_mode: 'catalog',
        base_url: '',
      })
    ).toBe(false)
    expect(
      isProviderVisibleInUI({
        id: 'bedrock',
        metadata_mode: 'catalog',
        base_url: 'https://bedrock-runtime.us-east-1.amazonaws.com',
      })
    ).toBe(true)
    expect(
      isProviderVisibleInUI({
        id: 'bedrock',
        metadata_mode: 'dynamic',
        base_url: '',
      })
    ).toBe(true)
  })

  it('filters only providers that still lack a supported official setup flow', () => {
    const providers = [
      { id: 'anthropic' },
      { id: 'google-antigravity', oauth: { connected: false } },
      { id: 'bedrock', metadata_mode: 'catalog', base_url: '' },
      { id: 'openai' },
    ]

    expect(filterProvidersVisibleInUI(providers).map((provider) => provider.id)).toEqual([
      'anthropic',
      'google-antigravity',
      'openai',
    ])
  })
})
