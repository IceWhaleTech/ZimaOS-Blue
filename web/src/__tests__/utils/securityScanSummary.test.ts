import { describe, expect, it } from 'vitest'

import {
  formatSecurityScanSummary,
  getVisibleSecurityScanSummaryMetrics,
} from '@/utils/securityScanSummary'

const labels = {
  passed: 'Passed',
  warnings: 'Warnings',
  failed: 'Failed',
}

describe('securityScanSummary', () => {
  it('hides the failed metric when the scan has no failed checks', () => {
    expect(
      getVisibleSecurityScanSummaryMetrics(
        {
          passed: 8,
          warnings: 0,
          failed: 0,
        },
        labels
      )
    ).toEqual([
      {
        key: 'passed',
        value: 8,
        label: 'Passed',
      },
    ])
  })

  it('shows the warning metric when the scan contains warnings', () => {
    expect(
      getVisibleSecurityScanSummaryMetrics(
        {
          passed: 7,
          warnings: 1,
          failed: 0,
        },
        labels
      )
    ).toEqual([
      {
        key: 'passed',
        value: 7,
        label: 'Passed',
      },
      {
        key: 'warning',
        value: 1,
        label: 'Warnings',
      },
    ])
  })

  it('keeps the failed metric when the scan contains failures', () => {
    expect(
      getVisibleSecurityScanSummaryMetrics(
        {
          passed: 6,
          warnings: 1,
          failed: 2,
        },
        labels
      )
    ).toEqual([
      {
        key: 'passed',
        value: 6,
        label: 'Passed',
      },
      {
        key: 'warning',
        value: 1,
        label: 'Warnings',
      },
      {
        key: 'failed',
        value: 2,
        label: 'Failed',
      },
    ])
  })

  it('omits zero-value warning and failed counts from summary copy', () => {
    expect(
      formatSecurityScanSummary(
        {
          passed: 8,
          warnings: 0,
          failed: 0,
        },
        labels
      )
    ).toBe('8 Passed')
  })

  it('includes warnings in summary copy when warnings exist', () => {
    expect(
      formatSecurityScanSummary(
        {
          passed: 7,
          warnings: 1,
          failed: 0,
        },
        labels
      )
    ).toBe('7 Passed, 1 Warnings')
  })

  it('includes the failed count in summary copy when failures exist', () => {
    expect(
      formatSecurityScanSummary(
        {
          passed: 6,
          warnings: 1,
          failed: 2,
        },
        labels
      )
    ).toBe('6 Passed, 1 Warnings, 2 Failed')
  })
})
