import assert from 'node:assert/strict'
import { spawnSync } from 'node:child_process'
import test from 'node:test'
import { fileURLToPath } from 'node:url'
import { EXIT_CODES, getAuditExitCode } from './i18n-audit.mjs'

const SCRIPT_PATH = new URL('./i18n-audit.mjs', import.meta.url)

function runAuditJson() {
  const result = spawnSync(process.execPath, [fileURLToPath(SCRIPT_PATH), '--json'], {
    cwd: new URL('..', import.meta.url),
    encoding: 'utf8',
  })

  assert.equal(result.status, 0, result.stderr || result.stdout)

  return JSON.parse(result.stdout)
}

test('json audit report excludes known settings translation keys from unknown static keys', () => {
  const report = runAuditJson()
  const unknown = new Set(report.unknownStaticKeys)

  assert.equal(unknown.has('settings.agentcoreRunner.refLoading'), false)
  assert.equal(unknown.has('settings.agentcoreRunner.selectedCandidate'), false)
  assert.equal(unknown.has('settings.agentcoreRunner.paretoFrontier'), false)
  assert.equal(unknown.has('settings.agentcoreRunner.selectionBasis'), false)
  assert.equal(unknown.has('settings.agentcoreRunner.frontierCandidates'), false)
  assert.equal(unknown.has('settings.agentcoreRunner.evaluatedCandidates'), false)
  assert.equal(unknown.has('settings.agentcoreRunner.objectiveVector'), false)
  assert.equal(unknown.has('settings.agentcoreRunner.followupOutcome'), false)
  assert.equal(unknown.has('settings.agentcoreRunner.proposalSet'), false)
  assert.equal(unknown.has('settings.agentcoreRunner.topImprovements'), false)
  assert.equal(unknown.has('settings.agentcoreRunner.topTradeoffs'), false)
  assert.equal(unknown.has('tools.names.advisor'), false)
  assert.equal(unknown.has('settings.knowledgeManagement'), false)
  assert.equal(unknown.has('settings.knowledgeSurface'), false)
  assert.equal(unknown.has('settings.assistiveRouting'), false)
})

test('json audit report has no unresolved dynamic key patterns', () => {
  const report = runAuditJson()
  assert.deepEqual(report.unresolvedDynamicPatterns, [])
})

test('strict unknown-static-key mode fails only when unknown keys remain', () => {
  assert.equal(
    getAuditExitCode(
      { unknownStaticKeys: ['settings.knowledgeManagement'], unresolvedDynamicPatterns: [] },
      { failOnUnknownStaticKeys: true }
    ),
    EXIT_CODES.violations
  )

  assert.equal(
    getAuditExitCode(
      { unknownStaticKeys: [], unresolvedDynamicPatterns: [] },
      { failOnUnknownStaticKeys: true }
    ),
    EXIT_CODES.ok
  )
})
