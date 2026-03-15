import assert from 'node:assert/strict'
import test from 'node:test'

import { EXIT_CODES, main } from './audit-ci.mjs'

function createCapture() {
  const logs = []
  const errors = []

  return {
    error(message) {
      errors.push(String(message))
    },
    errors,
    log(message) {
      logs.push(String(message))
    },
    logs,
  }
}

test('classifies npm registry failures as audit unavailable', () => {
  const capture = createCapture()

  const exitCode = main(['--no-allowlist'], {
    ...capture,
    env: { NPM_AUDIT_MAX_ATTEMPTS: '1' },
    spawn() {
      return {
        stderr: '',
        stdout: JSON.stringify({
          error: {
            detail: '',
            summary: '',
          },
          message:
            'request to https://registry.npmjs.org/-/npm/v1/security/advisories/bulk failed, reason: Client network socket disconnected before secure TLS connection was established',
        }),
      }
    },
  })

  assert.equal(exitCode, EXIT_CODES.unavailable)
  assert.match(capture.errors.join('\n'), /npm audit could not complete because the npm registry request failed/i)
  assert.doesNotMatch(capture.errors.join('\n'), /Unexpected npm audit format/)
})

test('retries transient npm registry failures before succeeding', () => {
  const capture = createCapture()
  let attempts = 0

  const exitCode = main(['--no-allowlist'], {
    ...capture,
    env: { NPM_AUDIT_MAX_ATTEMPTS: '2' },
    spawn() {
      attempts += 1
      if (attempts === 1) {
        return {
          stderr: '',
          stdout: JSON.stringify({
            error: {
              detail: '',
              summary: '',
            },
            message:
              'request to https://registry.npmjs.org/-/npm/v1/security/advisories/bulk failed, reason: Client network socket disconnected before secure TLS connection was established',
          }),
        }
      }

      return {
        stderr: '',
        stdout: JSON.stringify({
          vulnerabilities: {},
        }),
      }
    },
  })

  assert.equal(exitCode, EXIT_CODES.ok)
  assert.equal(attempts, 2)
  assert.match(capture.errors.join('\n'), /Retrying npm audit \(2\/2\)\.\.\./)
  assert.match(capture.logs.join('\n'), /npm audit passed with no vulnerabilities\./)
})

test('keeps blocking vulnerability failures unchanged', () => {
  const capture = createCapture()

  const exitCode = main(['--no-allowlist'], {
    ...capture,
    env: { NPM_AUDIT_MAX_ATTEMPTS: '1' },
    spawn() {
      return {
        stderr: '',
        stdout: JSON.stringify({
          vulnerabilities: {
            semver: {
              via: [
                {
                  source: 12345,
                  title: 'Regular Expression Denial of Service',
                  url: 'https://github.com/advisories/GHSA-xxxx-yyyy-zzzz',
                },
              ],
            },
          },
        }),
      }
    },
  })

  assert.equal(exitCode, EXIT_CODES.vulnerabilities)
  assert.match(capture.errors.join('\n'), /blocking vulnerable package/)
  assert.match(capture.errors.join('\n'), /- semver/)
  assert.match(capture.errors.join('\n'), /12345: Regular Expression Denial of Service/)
})
