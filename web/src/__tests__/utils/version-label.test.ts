import { describe, expect, it } from 'vitest'
import { formatVersionLabel } from '@/utils/version-label'

describe('version-label', () => {
  it('returns a dash for empty values', () => {
    expect(formatVersionLabel()).toBe('-')
    expect(formatVersionLabel('')).toBe('-')
    expect(formatVersionLabel('   ')).toBe('-')
  })

  it('adds a v prefix to numeric versions', () => {
    expect(formatVersionLabel('1.2.3')).toBe('v1.2.3')
    expect(formatVersionLabel(' 2.0.0 ')).toBe('v2.0.0')
  })

  it('keeps already-prefixed versions unchanged', () => {
    expect(formatVersionLabel('v3.4.5')).toBe('v3.4.5')
    expect(formatVersionLabel('V4.0.0')).toBe('V4.0.0')
  })

  it('does not prefix branch-style labels like main', () => {
    expect(formatVersionLabel('main')).toBe('main')
    expect(formatVersionLabel('latest')).toBe('latest')
  })
})
