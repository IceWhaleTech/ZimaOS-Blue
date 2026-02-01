/**
 * Performance monitoring utilities for tracking component performance
 */

interface PerformanceMetric {
  name: string
  duration: number
  timestamp: number
}

class PerformanceMonitor {
  private metrics: PerformanceMetric[] = []
  private maxMetrics = 100 // Keep last 100 metrics

  /**
   * Mark the start of a performance measurement
   */
  mark(name: string): void {
    if (typeof performance !== 'undefined' && performance.mark) {
      performance.mark(`${name}-start`)
    }
  }

  /**
   * Measure and record the duration since the start mark
   */
  measure(name: string): number | null {
    if (typeof performance === 'undefined' || !performance.mark || !performance.measure) {
      return null
    }

    try {
      const startMark = `${name}-start`
      const measureName = `${name}-measure`

      performance.mark(`${name}-end`)
      performance.measure(measureName, startMark, `${name}-end`)

      const measure = performance.getEntriesByName(measureName)[0]
      const duration = measure ? measure.duration : 0

      // Store metric
      this.metrics.push({
        name,
        duration,
        timestamp: Date.now(),
      })

      // Keep only last N metrics
      if (this.metrics.length > this.maxMetrics) {
        this.metrics.shift()
      }

      // Cleanup performance entries
      performance.clearMarks(startMark)
      performance.clearMarks(`${name}-end`)
      performance.clearMeasures(measureName)

      return duration
    } catch (error) {
      console.warn('Performance measurement failed:', error)
      return null
    }
  }

  /**
   * Get average duration for a specific metric name
   */
  getAverage(name: string): number {
    const filtered = this.metrics.filter((m) => m.name === name)
    if (filtered.length === 0) return 0

    const sum = filtered.reduce((acc, m) => acc + m.duration, 0)
    return sum / filtered.length
  }

  /**
   * Get all metrics for a specific name
   */
  getMetrics(name: string): PerformanceMetric[] {
    return this.metrics.filter((m) => m.name === name)
  }

  /**
   * Get all recorded metrics
   */
  getAllMetrics(): PerformanceMetric[] {
    return [...this.metrics]
  }

  /**
   * Clear all metrics
   */
  clear(): void {
    this.metrics = []
  }

  /**
   * Log performance summary to console
   */
  logSummary(): void {
    const metrics = this.metrics ?? []
    const grouped = metrics.reduce(
      (acc, metric) => {
        if (!acc[metric.name]) {
          acc[metric.name] = []
        }
        const arr = acc[metric.name]!
        arr.push(metric.duration)
        return acc
      },
      {} as Record<string, number[]>
    )

    console.group('Performance Summary')
    const entries = Object.entries(grouped) as [string, number[]][]
    for (const [name, list] of entries) {
      if (list.length === 0) continue
      const avg = list.reduce((a, b) => a + b, 0) / list.length
      const min = Math.min(...list)
      const max = Math.max(...list)
      console.log(`${name}: avg=${avg.toFixed(2)}ms, min=${min.toFixed(2)}ms, max=${max.toFixed(2)}ms, count=${list.length}`)
    }
    console.groupEnd()
  }
}

// Export singleton instance
export const performanceMonitor = new PerformanceMonitor()

/**
 * Composable for component performance tracking
 */
export function usePerformanceTracking(componentName: string) {
  const trackOperation = (operationName: string, fn: () => Promise<void> | void) => {
    return async () => {
      const metricName = `${componentName}:${operationName}`
      performanceMonitor.mark(metricName)

      try {
        await fn()
      } finally {
        const duration = performanceMonitor.measure(metricName)
        if (duration !== null && duration > 100) {
          console.warn(`Slow operation detected: ${metricName} took ${duration.toFixed(2)}ms`)
        }
      }
    }
  }

  return {
    trackOperation,
    getAverage: (operationName: string) => performanceMonitor.getAverage(`${componentName}:${operationName}`),
    logSummary: () => performanceMonitor.logSummary(),
  }
}
