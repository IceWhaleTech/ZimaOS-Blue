#!/usr/bin/env node

import fs from 'node:fs'
import path from 'node:path'
import { brotliCompressSync, constants as zlibConstants, gzipSync } from 'node:zlib'

const localeKeys = [
  'ca-ES',
  'cs-CZ',
  'da-DK',
  'de-DE',
  'el-GR',
  'en-GB',
  'en-US',
  'es-ES',
  'fr-FR',
  'ga-IE',
  'hr-HR',
  'hu-HU',
  'it-IT',
  'ja-JP',
  'ko-KR',
  'ml-IN',
  'nb-NO',
  'nl-NL',
  'pl-PL',
  'pt-BR',
  'pt-PT',
  'ro-RO',
  'ru-RU',
  'sk-SK',
  'sv-SE',
  'zh-CN',
  'zh-TW',
]

const baselineTotals = {
  raw: 6_979_289,
  gzip: 2_268_675,
  brotli: 1_849_833,
}

const sizeBudgets = {
  rawDelta: 50 * 1024,
  gzipDelta: 25 * 1024,
}

function parseArgs(argv) {
  const args = { distDir: 'dist', json: false }
  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index]
    if (arg === '--json') {
      args.json = true
      continue
    }
    if (arg === '--dist') {
      const next = argv[index + 1]
      if (!next) {
        throw new Error('--dist requires a path')
      }
      args.distDir = next
      index += 1
      continue
    }
    throw new Error(`unsupported argument: ${arg}`)
  }
  return args
}

function formatBytes(value) {
  const sign = value < 0 ? '-' : ''
  return `${sign}${Math.abs(value).toLocaleString('en-US')} B`
}

function readDirFiles(distDir) {
  return fs
    .readdirSync(distDir, { withFileTypes: true })
    .filter((entry) => entry.isFile())
    .map((entry) => entry.name)
}

function resolveLocaleChunk(distDir, files, localeKey) {
  const matcher = new RegExp(`^${localeKey.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}-.*\\.js$`)
  const matches = files.filter((file) => matcher.test(file))
  if (matches.length !== 1) {
    throw new Error(
      `expected exactly 1 built locale chunk for ${localeKey}, found ${matches.length}: ${matches.join(', ')}`
    )
  }
  return path.join(distDir, matches[0])
}

function compressedSize(rawBuffer, precompressedPath, algorithm) {
  if (fs.existsSync(precompressedPath)) {
    return fs.statSync(precompressedPath).size
  }
  if (algorithm === 'gzip') {
    return gzipSync(rawBuffer).length
  }
  return brotliCompressSync(rawBuffer, {
    params: {
      [zlibConstants.BROTLI_PARAM_QUALITY]: 11,
    },
  }).length
}

function collectRows(distDir) {
  const files = readDirFiles(distDir)
  return localeKeys.map((locale) => {
    const filePath = resolveLocaleChunk(distDir, files, locale)
    const rawBuffer = fs.readFileSync(filePath)
    const raw = rawBuffer.length
    const gzip = compressedSize(rawBuffer, `${filePath}.gz`, 'gzip')
    const brotli = compressedSize(rawBuffer, `${filePath}.br`, 'brotli')
    return {
      locale,
      file: path.basename(filePath),
      raw,
      gzip,
      brotli,
    }
  })
}

function sumBy(rows, key) {
  return rows.reduce((total, row) => total + row[key], 0)
}

function printTable(rows, totals, delta, budget) {
  console.log('Locale chunk size report')
  console.log('------------------------')
  console.log(`Baseline totals (2026-04-03): raw ${formatBytes(baselineTotals.raw)}, gzip ${formatBytes(baselineTotals.gzip)}, brotli ${formatBytes(baselineTotals.brotli)}`)
  console.log('')
  console.log(
    [
      'Locale'.padEnd(8),
      'Raw'.padStart(12),
      'Gzip'.padStart(12),
      'Brotli'.padStart(12),
      'Chunk'.padStart(28),
    ].join('  ')
  )
  console.log('-'.repeat(82))
  for (const row of rows) {
    console.log(
      [
        row.locale.padEnd(8),
        formatBytes(row.raw).padStart(12),
        formatBytes(row.gzip).padStart(12),
        formatBytes(row.brotli).padStart(12),
        row.file.padStart(28),
      ].join('  ')
    )
  }
  console.log('')
  console.log('Totals')
  console.log(`- raw: ${formatBytes(totals.raw)} (delta ${formatBytes(delta.raw)})`)
  console.log(`- gzip: ${formatBytes(totals.gzip)} (delta ${formatBytes(delta.gzip)})`)
  console.log(`- brotli: ${formatBytes(totals.brotli)} (delta ${formatBytes(delta.brotli)})`)
  console.log('')
  console.log('Budget check')
  console.log(
    `- raw delta <= ${formatBytes(sizeBudgets.rawDelta)}: ${budget.rawPass ? 'PASS' : 'FAIL'}`
  )
  console.log(
    `- gzip delta <= ${formatBytes(sizeBudgets.gzipDelta)}: ${budget.gzipPass ? 'PASS' : 'FAIL'}`
  )
}

function main() {
  const { distDir, json } = parseArgs(process.argv.slice(2))
  const resolvedDistDir = path.resolve(process.cwd(), distDir)
  if (!fs.existsSync(resolvedDistDir)) {
    throw new Error(`dist directory not found: ${resolvedDistDir}`)
  }

  const rows = collectRows(resolvedDistDir)
  const totals = {
    raw: sumBy(rows, 'raw'),
    gzip: sumBy(rows, 'gzip'),
    brotli: sumBy(rows, 'brotli'),
  }
  const delta = {
    raw: totals.raw - baselineTotals.raw,
    gzip: totals.gzip - baselineTotals.gzip,
    brotli: totals.brotli - baselineTotals.brotli,
  }
  const budget = {
    rawPass: delta.raw <= sizeBudgets.rawDelta,
    gzipPass: delta.gzip <= sizeBudgets.gzipDelta,
  }

  if (json) {
    console.log(
      JSON.stringify(
        {
          generated_at: new Date().toISOString(),
          dist_dir: resolvedDistDir,
          baseline_totals: baselineTotals,
          totals,
          delta,
          budgets: {
            raw_delta_budget: sizeBudgets.rawDelta,
            gzip_delta_budget: sizeBudgets.gzipDelta,
            ...budget,
          },
          rows,
        },
        null,
        2
      )
    )
    return
  }

  printTable(rows, totals, delta, budget)
}

try {
  main()
} catch (error) {
  console.error(error instanceof Error ? error.message : String(error))
  process.exit(1)
}
