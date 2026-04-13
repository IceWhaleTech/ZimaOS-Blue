import { afterEach, describe, expect, it, vi } from 'vitest'

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

describe('router knowledge compatibility redirect', () => {
  it('redirects the legacy knowledge route into the evolution knowledge lane and preserves query', async () => {
    const { routes } = await import('@/router')
    const knowledgeRoute = routes.find((route) => route.path === '/operations/knowledge')

    expect(knowledgeRoute).toBeTruthy()
    expect(typeof knowledgeRoute?.redirect).toBe('function')

    const redirect = knowledgeRoute?.redirect as NonNullable<typeof knowledgeRoute>['redirect']
    const redirected = (redirect as (to: { query?: Record<string, string> }) => unknown)({
      query: {
        page: 'readme',
        source: 'legacy-link',
      },
    }) as {
      name?: string
      query?: Record<string, string>
    }

    expect(redirected).toEqual({
      name: 'Evolution',
      query: {
        pane: 'knowledge',
        page: 'readme',
        source: 'legacy-link',
      },
    })
  })

  it('does not prefetch preview mode during vitest router imports', async () => {
    vi.resetModules()
    const fetchSpy = vi.fn(async () => new Response(JSON.stringify({ mode: 'normal' })))
    vi.stubGlobal('fetch', fetchSpy)

    await import('@/router')

    expect(fetchSpy).not.toHaveBeenCalled()
  })
})
