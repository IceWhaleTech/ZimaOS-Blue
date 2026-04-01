import type { Provider } from '@/api/providerPool'

type ProviderLike = Pick<Provider, 'id' | 'oauth' | 'metadata_mode' | 'base_url'>

function isTemporarilyUnsupportedOfficialProvider(provider?: ProviderLike | null): boolean {
  return (
    provider?.id === 'bedrock' &&
    provider.metadata_mode === 'catalog' &&
    !provider.base_url?.trim()
  )
}

// Keep official providers visible so the provider settings UI can surface both
// API-key and OAuth setup flows. Still hide official providers that do not yet
// have a complete first-party setup path.
export function isProviderVisibleInUI(provider?: ProviderLike | null): boolean {
  return !isTemporarilyUnsupportedOfficialProvider(provider)
}

export function filterProvidersVisibleInUI<T extends ProviderLike>(providers: readonly T[]): T[] {
  return providers.filter((provider) => isProviderVisibleInUI(provider))
}
