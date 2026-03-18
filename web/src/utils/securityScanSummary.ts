export interface SecurityScanSummary {
  passed: number
  warnings: number
  failed: number
}

export interface SecurityScanSummaryLabels {
  passed: string
  warnings: string
  failed: string
}

export interface SecurityScanSummaryMetric {
  key: 'passed' | 'warning' | 'failed'
  value: number
  label: string
}

export function getVisibleSecurityScanSummaryMetrics(
  summary: SecurityScanSummary,
  labels: SecurityScanSummaryLabels
): SecurityScanSummaryMetric[] {
  const metrics: SecurityScanSummaryMetric[] = [
    {
      key: 'passed',
      value: summary.passed,
      label: labels.passed,
    },
  ]

  if (summary.warnings > 0) {
    metrics.push({
      key: 'warning',
      value: summary.warnings,
      label: labels.warnings,
    })
  }

  if (summary.failed > 0) {
    metrics.push({
      key: 'failed',
      value: summary.failed,
      label: labels.failed,
    })
  }

  return metrics
}

export function formatSecurityScanSummary(
  summary: SecurityScanSummary,
  labels: SecurityScanSummaryLabels
): string {
  const parts = [`${summary.passed} ${labels.passed}`]

  if (summary.warnings > 0) {
    parts.push(`${summary.warnings} ${labels.warnings}`)
  }

  if (summary.failed > 0) {
    parts.push(`${summary.failed} ${labels.failed}`)
  }

  return parts.join(', ')
}
