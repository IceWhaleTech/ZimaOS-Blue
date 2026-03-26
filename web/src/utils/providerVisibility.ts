import type { Provider } from '@/api/providerPool'

type ProviderLike = Pick<Provider, 'oauth'>

// Keep OAuth-backed providers registered in data/API responses, but hide them from
// end-user provider pickers until the dedicated UI is ready.
export function isProviderVisibleInUI(provider?: ProviderLike | null): boolean {
  return !provider?.oauth
}

export function filterProvidersVisibleInUI<T extends ProviderLike>(providers: readonly T[]): T[] {
  return providers.filter((provider) => isProviderVisibleInUI(provider))
}
