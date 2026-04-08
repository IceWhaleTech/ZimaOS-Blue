#!/usr/bin/env node

import { spawnSync } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const DEFAULT_CWD = path.resolve(__dirname, '..')
const DEFAULT_ALLOWLIST_PATH = path.join(DEFAULT_CWD, 'audit-allowlist.json')
const DEFAULT_MAX_ATTEMPTS = 3

export const EXIT_CODES = Object.freeze({
  ok: 0,
  vulnerabilities: 1,
  unavailable: 2,
})

const TRANSIENT_FAILURE_PATTERNS = [
  'advisories/bulk failed',
  'before secure tls connection was established',
  'client network socket disconnected',
  'eai_again',
  'econnreset',
  'fetch failed',
  'network timeout',
  'socket hang up',
  'tls',
  '503',
  '504',
]

function parseCliArgs(argv) {
  const extraArgs = []
  let cwd = DEFAULT_CWD
  let allowlistPath = DEFAULT_ALLOWLIST_PATH

  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index]

    if (arg === '--no-allowlist') {
      allowlistPath = null
      continue
    }

    if (arg === '--cwd' || arg.startsWith('--cwd=')) {
      const value = arg === '--cwd' ? argv[++index] : arg.slice('--cwd='.length)
      if (!value) {
        throw new Error('Missing value for --cwd')
      }
      cwd = path.resolve(value)
      continue
    }

    if (arg === '--allowlist' || arg.startsWith('--allowlist=')) {
      const value = arg === '--allowlist' ? argv[++index] : arg.slice('--allowlist='.length)
      if (!value) {
        throw new Error('Missing value for --allowlist')
      }
      allowlistPath = path.resolve(value)
      continue
    }

    extraArgs.push(arg)
  }

  return { allowlistPath, cwd, extraArgs }
}

function getMaxAttempts(env) {
  const rawValue = env?.NPM_AUDIT_MAX_ATTEMPTS
  if (!rawValue) {
    return DEFAULT_MAX_ATTEMPTS
  }

  const parsedValue = Number.parseInt(rawValue, 10)
  if (Number.isFinite(parsedValue) && parsedValue >= 1) {
    return parsedValue
  }

  return DEFAULT_MAX_ATTEMPTS
}

function isAuditReport(report) {
  return (
    report &&
    typeof report === 'object' &&
    typeof report.vulnerabilities === 'object' &&
    report.vulnerabilities !== null
  )
}

function isTransientFailure(text) {
  if (!text) {
    return false
  }

  const normalized = text.toLowerCase()
  return TRANSIENT_FAILURE_PATTERNS.some((pattern) => normalized.includes(pattern))
}

function formatFailureDetails(parts) {
  return parts.map((part) => (typeof part === 'string' ? part.trim() : '')).filter(Boolean).join('\n')
}

function classifyAuditFailure({ report, stderr, stdout }) {
  const message = typeof report?.message === 'string' ? report.message : ''
  const summary = typeof report?.error?.summary === 'string' ? report.error.summary : ''
  const detail = typeof report?.error?.detail === 'string' ? report.error.detail : ''
  const retryable =
    isTransientFailure(message) ||
    isTransientFailure(summary) ||
    isTransientFailure(detail) ||
    isTransientFailure(stderr)
  const failureDetails = formatFailureDetails([message, summary, detail, stderr])

  if (message || summary || detail) {
    return {
      exitCode: EXIT_CODES.unavailable,
      message: `${retryable ? 'npm audit could not complete because the npm registry request failed.' : 'npm audit could not complete.'}${failureDetails ? `\n${failureDetails}` : ''}`,
      retryable,
    }
  }

  return {
    exitCode: EXIT_CODES.unavailable,
    message: `Unexpected npm audit format:\n${stdout}`,
    retryable: false,
  }
}

function loadAllowlist(allowlistPath, { existsSync = fs.existsSync, readFileSync = fs.readFileSync } = {}) {
  if (!allowlistPath) {
    return { allowedAdvisoryIds: new Set(), notes: {} }
  }

  if (!existsSync(allowlistPath)) {
    throw new Error(`Missing audit allowlist file: ${allowlistPath}`)
  }

  let parsed
  try {
    parsed = JSON.parse(readFileSync(allowlistPath, 'utf8'))
  } catch (error) {
    throw new Error(`Failed to parse ${allowlistPath}: ${error instanceof Error ? error.message : String(error)}`)
  }

  const ids = Array.isArray(parsed.allowedAdvisoryIds) ? parsed.allowedAdvisoryIds : []
  const allowedAdvisoryIds = new Set(ids.map(Number).filter((value) => Number.isFinite(value)))
  const notes = parsed.notes && typeof parsed.notes === 'object' ? parsed.notes : {}

  return { allowedAdvisoryIds, notes }
}

function formatRetryMessage(failure, attempt, maxAttempts) {
  return `${failure.message}\nRetrying npm audit (${attempt + 1}/${maxAttempts})...`
}

function resolveNpmCommand(platform = process.platform) {
  return platform === 'win32' ? 'npm.cmd' : 'npm'
}

function runNpmAudit(extraArgs, options = {}) {
  const {
    cwd = DEFAULT_CWD,
    env = process.env,
    error = console.error,
    maxAttempts = getMaxAttempts(env),
    platform = process.platform,
    spawn = spawnSync,
  } = options
  const npmCommand = resolveNpmCommand(platform)

  for (let attempt = 1; attempt <= maxAttempts; attempt += 1) {
    const spawnOptions = {
      cwd,
      encoding: 'utf8',
      env,
    }
    if (platform === 'win32') {
      spawnOptions.shell = true
    }

    const result = spawn(npmCommand, ['audit', '--json', ...extraArgs], {
      ...spawnOptions,
    })

    if (result.error) {
      return {
        exitCode: EXIT_CODES.unavailable,
        message: `Failed to run npm audit: ${result.error.message}`,
        ok: false,
      }
    }

    const stdout = result.stdout?.trim() || ''
    const stderr = result.stderr?.trim() || ''
    if (!stdout) {
      const retryable = isTransientFailure(stderr)
      const failure = {
        exitCode: EXIT_CODES.unavailable,
        message: `npm audit did not return JSON output.${stderr ? `\n${stderr}` : ''}`,
        retryable,
      }

      if (retryable && attempt < maxAttempts) {
        error(formatRetryMessage(failure, attempt, maxAttempts))
        continue
      }

      return { ...failure, ok: false }
    }

    let report
    try {
      report = JSON.parse(stdout)
    } catch (error) {
      return {
        exitCode: EXIT_CODES.unavailable,
        message: `npm audit returned non-JSON output: ${error instanceof Error ? error.message : String(error)}`,
        ok: false,
      }
    }

    if (isAuditReport(report)) {
      return { ok: true, report }
    }

    const failure = classifyAuditFailure({ report, stderr, stdout })
    if (failure.retryable && attempt < maxAttempts) {
      error(formatRetryMessage(failure, attempt, maxAttempts))
      continue
    }

    return { ...failure, ok: false }
  }

  return {
    exitCode: EXIT_CODES.unavailable,
    message: 'npm audit could not complete after exhausting retries.',
    ok: false,
  }
}

function collectBlockedPackages(vulnerabilities, allowedAdvisoryIds) {
  const directBlockedPackages = new Set()
  const dependencyEdges = new Map()
  const blockedAdvisories = new Map()
  const allowlistedAdvisories = new Map()
  const packageNames = new Set(Object.keys(vulnerabilities))

  for (const [pkgName, details] of Object.entries(vulnerabilities)) {
    const viaEntries = Array.isArray(details?.via) ? details.via : []
    const deps = new Set()

    for (const via of viaEntries) {
      if (typeof via === 'string') {
        deps.add(via)
        continue
      }

      if (!via || typeof via !== 'object') {
        directBlockedPackages.add(pkgName)
        continue
      }

      const sourceId = Number(via.source)
      if (!Number.isFinite(sourceId)) {
        directBlockedPackages.add(pkgName)
        continue
      }

      if (allowedAdvisoryIds.has(sourceId)) {
        if (!allowlistedAdvisories.has(sourceId)) {
          allowlistedAdvisories.set(sourceId, via)
        }
        continue
      }

      directBlockedPackages.add(pkgName)
      if (!blockedAdvisories.has(sourceId)) {
        blockedAdvisories.set(sourceId, via)
      }
    }

    dependencyEdges.set(pkgName, deps)
  }

  const blockedPackages = new Set(directBlockedPackages)
  let changed = true
  while (changed) {
    changed = false
    for (const [pkgName, deps] of dependencyEdges.entries()) {
      if (blockedPackages.has(pkgName)) continue

      for (const dep of deps) {
        if (!packageNames.has(dep) || blockedPackages.has(dep)) {
          blockedPackages.add(pkgName)
          changed = true
          break
        }
      }
    }
  }

  return { blockedPackages, blockedAdvisories, allowlistedAdvisories }
}

export function main(argv = process.argv.slice(2), options = {}) {
  const { error = console.error, existsSync, log = console.log, readFileSync, spawn, env = process.env, platform = process.platform } = options

  let parsedArgs
  try {
    parsedArgs = parseCliArgs(argv)
  } catch (parseError) {
    error(parseError instanceof Error ? parseError.message : String(parseError))
    return EXIT_CODES.unavailable
  }

  let allowlist
  try {
    allowlist = loadAllowlist(parsedArgs.allowlistPath, { existsSync, readFileSync })
  } catch (loadError) {
    error(loadError instanceof Error ? loadError.message : String(loadError))
    return EXIT_CODES.unavailable
  }

  const auditResult = runNpmAudit(parsedArgs.extraArgs, {
    cwd: parsedArgs.cwd,
    env,
    error,
    platform,
    spawn,
  })

  if (!auditResult.ok) {
    error(auditResult.message)
    return auditResult.exitCode
  }

  const { allowedAdvisoryIds, notes } = allowlist
  const vulnerabilities = auditResult.report.vulnerabilities || {}
  const { blockedPackages, blockedAdvisories, allowlistedAdvisories } = collectBlockedPackages(
    vulnerabilities,
    allowedAdvisoryIds
  )

  if (blockedPackages.size === 0) {
    if (allowlistedAdvisories.size > 0) {
      log(`npm audit passed with allowlist (${allowlistedAdvisories.size} advisory/advisories ignored):`)
      for (const [id, advisory] of allowlistedAdvisories.entries()) {
        const note = notes[String(id)]
        const ghsa = note?.ghsa || 'unknown-ghsa'
        const title = advisory?.title || 'No title'
        log(`- ${id} (${ghsa}): ${title}`)
      }
    } else {
      log('npm audit passed with no vulnerabilities.')
    }
    return EXIT_CODES.ok
  }

  error(`npm audit failed: found ${blockedPackages.size} blocking vulnerable package(s).`)
  error('Blocking packages:')
  for (const pkgName of [...blockedPackages].sort()) {
    error(`- ${pkgName}`)
  }

  if (blockedAdvisories.size > 0) {
    error('Blocking advisories:')
    for (const [id, advisory] of blockedAdvisories.entries()) {
      const title = advisory?.title || 'No title'
      const url = advisory?.url || ''
      error(`- ${id}: ${title}${url ? ` (${url})` : ''}`)
    }
  }

  return EXIT_CODES.vulnerabilities
}

if (path.resolve(process.argv[1] || '') === __filename) {
  process.exit(main())
}
