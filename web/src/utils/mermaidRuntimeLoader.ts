export type MermaidRuntime = {
  default: {
    initialize: (config: Record<string, unknown>) => void
    render: (id: string, code: string) => Promise<{ svg: string }>
  }
}

const MERMAID_CDN_URLS = [
  'https://cdn.jsdelivr.net/npm/mermaid@11.14.0/dist/mermaid.esm.min.mjs',
  'https://unpkg.com/mermaid@11.14.0/dist/mermaid.esm.min.mjs',
] as const

export const embeddedMermaidBundleDisabled =
  __EMBED_DISABLE_MERMAID__ || import.meta.env.VITE_EMBED_DISABLE_MERMAID === '1'

let remoteMermaidPromise: Promise<MermaidRuntime> | null = null

async function importRemoteMermaid(url: string): Promise<MermaidRuntime> {
  return import(/* @vite-ignore */ url) as Promise<MermaidRuntime>
}

async function loadRemoteMermaidRuntime(): Promise<MermaidRuntime> {
  let lastError: unknown = null

  for (const url of MERMAID_CDN_URLS) {
    try {
      return await importRemoteMermaid(url)
    } catch (error) {
      lastError = error
    }
  }

  throw lastError instanceof Error
    ? lastError
    : new Error('Failed to load Mermaid runtime from npm CDN')
}

export async function loadMermaidRuntime(): Promise<MermaidRuntime> {
  if (!embeddedMermaidBundleDisabled) {
    return (await import('mermaid')) as MermaidRuntime
  }

  if (!remoteMermaidPromise) {
    remoteMermaidPromise = loadRemoteMermaidRuntime().catch((error) => {
      remoteMermaidPromise = null
      throw error
    })
  }

  return remoteMermaidPromise
}
