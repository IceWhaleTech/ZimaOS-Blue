import fs from 'node:fs'
import path from 'node:path'

import { describe, expect, it } from 'vitest'

function readDeclaredMermaidVersion(): string {
  const packageJson = JSON.parse(
    fs.readFileSync(path.resolve(process.cwd(), 'package.json'), 'utf8')
  ) as {
    dependencies?: Record<string, string>
  }

  const declaredVersion = packageJson.dependencies?.mermaid
  expect(declaredVersion).toBeTypeOf('string')
  return declaredVersion!.replace(/^[^\d]*/, '')
}

describe('mermaidRuntimeLoader', () => {
  it('keeps CDN fallback URLs aligned with the package mermaid version', async () => {
    const loader = (await import('./mermaidRuntimeLoader')) as Record<string, unknown>

    expect(loader).toHaveProperty('MERMAID_CDN_URLS')
    expect(loader.MERMAID_CDN_URLS).toEqual([
      `https://cdn.jsdelivr.net/npm/mermaid@${readDeclaredMermaidVersion()}/dist/mermaid.esm.min.mjs`,
      `https://unpkg.com/mermaid@${readDeclaredMermaidVersion()}/dist/mermaid.esm.min.mjs`,
    ])
  })
})
