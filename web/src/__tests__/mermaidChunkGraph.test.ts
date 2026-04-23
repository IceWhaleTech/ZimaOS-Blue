// @vitest-environment node

import { execFile as execFileCallback } from 'node:child_process'
import { mkdtemp, readdir, readFile, rm } from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'
import { promisify } from 'node:util'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const execFile = promisify(execFileCallback)

function collectJsImports(code: string, files: Set<string>): string[] {
  return [...code.matchAll(/from"\.\/([^"]+)"/g)].map((match) => match[1]).filter((file) => files.has(file))
}

function findCycles(graph: Map<string, string[]>): string[][] {
  const visited = new Set<string>()
  const onStack = new Set<string>()
  const stack: string[] = []
  const cycles = new Set<string>()

  function visit(node: string): void {
    visited.add(node)
    onStack.add(node)
    stack.push(node)

    for (const next of graph.get(node) || []) {
      if (!visited.has(next)) {
        visit(next)
        continue
      }

      if (!onStack.has(next)) {
        continue
      }

      const startIndex = stack.indexOf(next)
      cycles.add(stack.slice(startIndex).concat(next).join(' -> '))
    }

    stack.pop()
    onStack.delete(node)
  }

  for (const node of graph.keys()) {
    if (!visited.has(node)) {
      visit(node)
    }
  }

  return [...cycles].sort().map((cycle) => cycle.split(' -> '))
}

describe('mermaid production build chunk graph', () => {
  it(
    'does not emit circular imports between vendor and mermaid runtime chunks',
    async () => {
      const testFile = fileURLToPath(import.meta.url)
      const webRoot = path.resolve(path.dirname(testFile), '../..')
      const tmpOutDir = await mkdtemp(path.join(os.tmpdir(), 'zimaos-web-build-'))

      try {
        await execFile(process.platform === 'win32' ? 'npm.cmd' : 'npm', ['run', 'build', '--', '--outDir', tmpOutDir], {
          cwd: webRoot,
          env: {
            ...process.env,
            CI: '1',
          },
        })

        const jsFiles = (await readdir(tmpOutDir)).filter((file) => file.endsWith('.js'))
        const jsFileSet = new Set(jsFiles)
        const graph = new Map<string, string[]>()

        for (const file of jsFiles) {
          const code = await readFile(path.join(tmpOutDir, file), 'utf8')
          graph.set(file, collectJsImports(code, jsFileSet))
        }

        const badCycles = findCycles(graph)
          .map((cycle) => cycle.join(' -> '))
          .filter(
            (cycle) =>
              cycle.includes('vendor-') &&
              (
                cycle.includes('chunk-') ||
                cycle.includes('infoDiagram-') ||
                cycle.includes('mermaid-runtime-')
              )
          )

        expect(badCycles).toEqual([])
      } finally {
        await rm(tmpOutDir, { recursive: true, force: true })
      }
    },
    120_000
  )
})
