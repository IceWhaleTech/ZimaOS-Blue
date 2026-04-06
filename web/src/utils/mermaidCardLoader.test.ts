import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

describe('mermaidCardLoader', () => {
  beforeEach(() => {
    vi.resetModules()
    vi.unstubAllEnvs()
  })

  afterEach(() => {
    vi.doUnmock('@/components/typeless/CardMermaid.vue')
    vi.unstubAllEnvs()
  })

  it('loads the standard mermaid renderer by default', async () => {
    vi.doMock('@/components/typeless/CardMermaid.vue', () => ({
      default: { name: 'CardMermaid' },
    }))

    const { embeddedMermaidBundleDisabled, loadMermaidCardComponent } = await import(
      './mermaidCardLoader'
    )

    expect(embeddedMermaidBundleDisabled).toBe(false)
    await expect(loadMermaidCardComponent()).resolves.toMatchObject({
      default: { name: 'CardMermaid' },
    })
  })

  it('keeps the mermaid card component while disabling the embedded local bundle', async () => {
    vi.stubEnv('VITE_EMBED_DISABLE_MERMAID', '1')
    vi.doMock('@/components/typeless/CardMermaid.vue', () => ({
      default: { name: 'CardMermaid' },
    }))

    const { embeddedMermaidBundleDisabled, loadMermaidCardComponent } = await import(
      './mermaidCardLoader'
    )

    expect(embeddedMermaidBundleDisabled).toBe(true)
    await expect(loadMermaidCardComponent()).resolves.toMatchObject({
      default: { name: 'CardMermaid' },
    })
  })
})
