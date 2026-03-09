import { spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import path from 'node:path'

const mode = process.argv[2]
const command = mode === 'build' ? 'validate' : mode

if (!mode || !['dev', 'build'].includes(mode)) {
  console.error('Usage: node ./scripts/run-mint.mjs <dev|build>')
  process.exit(1)
}

const cwd = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const localMint = path.join(cwd, 'node_modules', '.bin', process.platform === 'win32' ? 'mint.cmd' : 'mint')
const attempts = [
  { cmd: localMint, args: [command], label: 'local mint' },
  { cmd: 'mint', args: [command], label: 'global mint' },
  { cmd: 'npx', args: ['-y', 'mintlify', command], label: 'npx mintlify' },
  { cmd: 'npx', args: ['-y', 'mint', command], label: 'npx mint' },
]

let lastStatus = 1

for (const attempt of attempts) {
  const result = spawnSync(attempt.cmd, attempt.args, {
    cwd,
    stdio: 'inherit',
    shell: false,
  })

  if (!result.error && result.status === 0) {
    process.exit(0)
  }

  if (result.error?.code !== 'ENOENT') {
    lastStatus = result.status ?? 1
  }

  console.error(`Failed using ${attempt.label}. Trying next option...`)
}

process.exit(lastStatus)
