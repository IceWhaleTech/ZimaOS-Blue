import { beforeEach, describe, expect, it, vi } from 'vitest'

const localStorageMock = (() => {
  let store: Record<string, string> = {}
  return {
    getItem: (key: string) => (key in store ? store[key] : null),
    setItem: (key: string, value: string) => {
      store[key] = String(value)
    },
    removeItem: (key: string) => {
      delete store[key]
    },
    clear: () => {
      store = {}
    },
  }
})()

vi.stubGlobal('localStorage', localStorageMock)

describe('api/client refresh coordination', () => {
  beforeEach(() => {
    vi.resetModules()
    vi.clearAllMocks()
    localStorageMock.clear()
    window.history.replaceState({}, '', '/login')
  })

  it('resolves all waiting refresh callers when the refresh request fails', async () => {
    localStorageMock.setItem('refresh_token', 'refresh-token')

    let rejectRefresh: ((reason?: unknown) => void) | null = null
    const fetchMock = vi.fn((input: RequestInfo | URL) => {
      const url = String(input)
      if (url === '/api/v1/auth/refresh') {
        return new Promise<Response>((_resolve, reject) => {
          rejectRefresh = reject
        })
      }
      throw new Error(`Unexpected fetch call: ${url}`)
    })
    vi.stubGlobal('fetch', fetchMock)

    const { ensureFreshToken } = await import('@/api/client')

    const firstRefresh = ensureFreshToken()
    await Promise.resolve()
    const secondRefresh = ensureFreshToken()

    expect(fetchMock).toHaveBeenCalledTimes(1)
    rejectRefresh?.(new Error('refresh failed'))

    await expect(Promise.all([firstRefresh, secondRefresh])).resolves.toEqual([null, null])
  })
})
