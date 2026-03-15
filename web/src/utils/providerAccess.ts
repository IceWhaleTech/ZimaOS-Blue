import type { Provider } from '@/api/providerPool'

type ProviderLike = Pick<Provider, 'type' | 'api_keys'>

export function providerHasEnabledApiKey(provider: ProviderLike): boolean {
  return (provider.api_keys || []).some((apiKey) => apiKey.enabled !== false)
}

export function hasConfiguredLlmApiKey(providers: ProviderLike[]): boolean {
  return providers.some(
    (provider) => provider.type !== 'media' && providerHasEnabledApiKey(provider)
  )
}
