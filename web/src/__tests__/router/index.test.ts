import { describe, expect, it } from 'vitest'

import { routes } from '@/router'

describe('router knowledge compatibility redirect', () => {
  it('redirects the legacy knowledge route into the evolution knowledge lane and preserves query', () => {
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
})
