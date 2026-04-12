export interface AppLocationState {
  path: string
  search: string
  fullPath: string
}

type LocationLike = Pick<Location, 'pathname' | 'search' | 'hash'>

function ensureLeadingSlash(value: string): string {
  const trimmed = String(value || '').trim()
  if (!trimmed) return '/'
  return trimmed.startsWith('/') ? trimmed : `/${trimmed}`
}

function parseModuleLocation(location: LocationLike): AppLocationState {
  const fallbackSearch = location.search || ''
  const rawHash = String(location.hash || '').replace(/^#/, '').trim()

  if (!rawHash) {
    return {
      path: '/',
      search: fallbackSearch,
      fullPath: `/${fallbackSearch}`,
    }
  }

  const normalizedHash = ensureLeadingSlash(rawHash)
  const queryIndex = normalizedHash.indexOf('?')
  const path = queryIndex >= 0 ? normalizedHash.slice(0, queryIndex) : normalizedHash
  const search = queryIndex >= 0 ? normalizedHash.slice(queryIndex) : fallbackSearch

  return {
    path: path || '/',
    search,
    fullPath: `${path || '/'}${search}`,
  }
}

export function getCurrentAppLocation(): AppLocationState {
  if (typeof window === 'undefined') {
    return { path: '/', search: '', fullPath: '/' }
  }

  if (import.meta.env.VITE_MODULE_UI === '1') {
    return parseModuleLocation(window.location)
  }

  const path = ensureLeadingSlash(window.location.pathname || '/')
  const search = window.location.search || ''
  const hash = window.location.hash || ''

  return {
    path,
    search,
    fullPath: `${path}${search}${hash}`,
  }
}
