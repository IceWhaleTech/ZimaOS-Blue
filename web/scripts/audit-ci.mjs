#!/usr/bin/env node

import { spawnSync } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const WEB_ROOT = path.resolve(__dirname, '..')
const ALLOWLIST_PATH = path.join(WEB_ROOT, 'audit-allowlist.json')

function fail(message) {
  console.error(message)
  process.exit(1)
}

function loadAllowlist() {
  if (!fs.existsSync(ALLOWLIST_PATH)) {
    fail(`Missing audit allowlist file: ${ALLOWLIST_PATH}`)
  }

  let parsed
  try {
    parsed = JSON.parse(fs.readFileSync(ALLOWLIST_PATH, 'utf8'))
  } catch (error) {
    fail(`Failed to parse ${ALLOWLIST_PATH}: ${error instanceof Error ? error.message : String(error)}`)
  }

  const ids = Array.isArray(parsed.allowedAdvisoryIds) ? parsed.allowedAdvisoryIds : []
  const allowedAdvisoryIds = new Set(ids.map(Number).filter((value) => Number.isFinite(value)))
  const notes = parsed.notes && typeof parsed.notes === 'object' ? parsed.notes : {}

  return { allowedAdvisoryIds, notes }
}

function runNpmAudit(extraArgs) {
  const result = spawnSync('npm', ['audit', '--json', ...extraArgs], {
    cwd: WEB_ROOT,
    encoding: 'utf8',
    env: process.env,
  })

  if (result.error) {
    fail(`Failed to run npm audit: ${result.error.message}`)
  }

  const stdout = result.stdout?.trim() || ''
  if (!stdout) {
    const stderr = result.stderr?.trim() || ''
    fail(`npm audit did not return JSON output.\n${stderr}`)
  }

  let report
  try {
    report = JSON.parse(stdout)
  } catch (error) {
    fail(`npm audit returned non-JSON output: ${error instanceof Error ? error.message : String(error)}`)
  }

  if (!report || typeof report !== 'object' || typeof report.vulnerabilities !== 'object' || report.vulnerabilities === null) {
    fail(`Unexpected npm audit format:\n${stdout}`)
  }

  return report
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

function main() {
  const extraArgs = process.argv.slice(2)
  const { allowedAdvisoryIds, notes } = loadAllowlist()
  const report = runNpmAudit(extraArgs)
  const vulnerabilities = report.vulnerabilities || {}
  const { blockedPackages, blockedAdvisories, allowlistedAdvisories } = collectBlockedPackages(
    vulnerabilities,
    allowedAdvisoryIds
  )

  if (blockedPackages.size === 0) {
    if (allowlistedAdvisories.size > 0) {
      console.log(`npm audit passed with allowlist (${allowlistedAdvisories.size} advisory/advisories ignored):`)
      for (const [id, advisory] of allowlistedAdvisories.entries()) {
        const note = notes[String(id)]
        const ghsa = note?.ghsa || 'unknown-ghsa'
        const title = advisory?.title || 'No title'
        console.log(`- ${id} (${ghsa}): ${title}`)
      }
    } else {
      console.log('npm audit passed with no vulnerabilities.')
    }
    return
  }

  console.error(`npm audit failed: found ${blockedPackages.size} blocking vulnerable package(s).`)
  console.error('Blocking packages:')
  for (const pkgName of [...blockedPackages].sort()) {
    console.error(`- ${pkgName}`)
  }

  if (blockedAdvisories.size > 0) {
    console.error('Blocking advisories:')
    for (const [id, advisory] of blockedAdvisories.entries()) {
      const title = advisory?.title || 'No title'
      const url = advisory?.url || ''
      console.error(`- ${id}: ${title}${url ? ` (${url})` : ''}`)
    }
  }

  process.exit(1)
}

main()
