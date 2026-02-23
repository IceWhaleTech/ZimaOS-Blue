/**
 * Component Pool - Reuse Vue component instances to reduce creation/destruction overhead
 *
 * This pool manages async-loaded card components, keeping them alive when scrolled
 * out of view and reusing them when similar cards come into view.
 */

import { shallowRef, type Component, type ShallowRef, markRaw } from 'vue'

// ============================================================================
// Types
// ============================================================================

interface PooledComponent {
  component: Component
  lastUsed: number
  inUse: boolean
  cardType: string
}

interface PoolConfig {
  maxPoolSize: number      // Max components per type
  maxIdleTime: number      // Max time (ms) to keep unused components
  cleanupInterval: number  // How often to run cleanup
}

// ============================================================================
// Component Pool Class
// ============================================================================

class ComponentPool {
  private pools = new Map<string, PooledComponent[]>()
  private loadedComponents = new Map<string, Component>()
  private loadingPromises = new Map<string, Promise<Component>>()
  private config: PoolConfig
  private cleanupTimer: ReturnType<typeof setInterval> | null = null

  constructor(config: Partial<PoolConfig> = {}) {
    this.config = {
      maxPoolSize: 10,
      maxIdleTime: 60 * 1000, // 1 minute
      cleanupInterval: 30 * 1000, // 30 seconds
      ...config,
    }

    // Start cleanup timer
    this.startCleanup()
  }

  /**
   * Get or load a component for a card type
   */
  async getComponent(cardType: string): Promise<Component | null> {
    // Check if already loaded
    const loaded = this.loadedComponents.get(cardType)
    if (loaded) return loaded

    // Check if currently loading
    const loading = this.loadingPromises.get(cardType)
    if (loading) return loading

    // Start loading
    const loadPromise = this.loadComponent(cardType)
    this.loadingPromises.set(cardType, loadPromise)

    try {
      const component = await loadPromise
      this.loadedComponents.set(cardType, component)
      return component
    } finally {
      this.loadingPromises.delete(cardType)
    }
  }

  /**
   * Load a component dynamically
   */
  private async loadComponent(cardType: string): Promise<Component> {
    const componentMap: Record<string, () => Promise<{ default: Component }>> = {
      progress: () => import('@/components/typeless/CardProgress.vue'),
      action: () => import('@/components/typeless/CardAction.vue'),
      result: () => import('@/components/typeless/CardResult.vue'),
      detection: () => import('@/components/typeless/CardDetection.vue'),
      chart: () => import('@/components/typeless/CardChart.vue'),
      gallery: () => import('@/components/typeless/CardGallery.vue'),
      file: () => import('@/components/typeless/CardFile.vue'),
      link: () => import('@/components/typeless/CardLink.vue'),
      metric: () => import('@/components/typeless/CardMetric.vue'),
      comparison: () => import('@/components/typeless/CardComparison.vue'),
      steps: () => import('@/components/typeless/CardSteps.vue'),
      map: () => import('@/components/typeless/CardMap.vue'),
      weather: () => import('@/components/typeless/CardWeather.vue'),
      profile: () => import('@/components/typeless/CardProfile.vue'),
      countdown: () => import('@/components/typeless/CardCountdown.vue'),
      rating: () => import('@/components/typeless/CardRating.vue'),
      accordion: () => import('@/components/typeless/CardAccordion.vue'),
      audio: () => import('@/components/typeless/CardAudio.vue'),
      choice: () => import('@/components/typeless/CardChoice.vue'),
      'collapsible-code': () => import('@/components/typeless/CardCollapsibleCode.vue'),
      diff: () => import('@/components/typeless/CardDiff.vue'),
      video: () => import('@/components/typeless/CardVideo.vue'),
      mermaid: () => import('@/components/typeless/CardMermaid.vue'),
      search: () => import('@/components/typeless/CardSearch.vue'),
      'ui-review': () => import('@/components/typeless/CardUIReview.vue'),
      'media-generate': () => import('@/components/typeless/CardMediaGenerate.vue'),
    }

    const loader = componentMap[cardType]
    if (!loader) {
      throw new Error(`Unknown card type: ${cardType}`)
    }

    const module = await loader()
    return markRaw(module.default)
  }

  /**
   * Acquire a pooled component instance
   */
  acquire(cardType: string): PooledComponent | null {
    const pool = this.pools.get(cardType)
    if (!pool || pool.length === 0) return null

    // Find an unused component
    const available = pool.find(p => !p.inUse)
    if (available) {
      available.inUse = true
      available.lastUsed = Date.now()
      return available
    }

    return null
  }

  /**
   * Release a component back to the pool
   */
  release(cardType: string, component: Component): void {
    let pool = this.pools.get(cardType)
    if (!pool) {
      pool = []
      this.pools.set(cardType, pool)
    }

    // Find the component in the pool
    const existing = pool.find(p => p.component === component)
    if (existing) {
      existing.inUse = false
      existing.lastUsed = Date.now()
      return
    }

    // Add to pool if under limit
    if (pool.length < this.config.maxPoolSize) {
      pool.push({
        component: markRaw(component),
        lastUsed: Date.now(),
        inUse: false,
        cardType,
      })
    }
  }

  /**
   * Preload components for common card types
   */
  async preload(cardTypes: string[]): Promise<void> {
    await Promise.all(cardTypes.map(type => this.getComponent(type)))
  }

  /**
   * Start periodic cleanup of idle components
   */
  private startCleanup(): void {
    if (this.cleanupTimer) return

    this.cleanupTimer = setInterval(() => {
      this.cleanup()
    }, this.config.cleanupInterval)
  }

  /**
   * Clean up idle components
   */
  private cleanup(): void {
    const now = Date.now()

    for (const [cardType, pool] of this.pools) {
      // Remove idle components that have exceeded max idle time
      const activeComponents = pool.filter(p => {
        if (p.inUse) return true
        return now - p.lastUsed < this.config.maxIdleTime
      })

      if (activeComponents.length !== pool.length) {
        this.pools.set(cardType, activeComponents)
      }

      // Remove empty pools
      if (activeComponents.length === 0) {
        this.pools.delete(cardType)
      }
    }
  }

  /**
   * Get pool statistics
   */
  getStats(): {
    totalPooled: number
    poolsByType: Record<string, { total: number; inUse: number }>
    loadedComponents: number
  } {
    const poolsByType: Record<string, { total: number; inUse: number }> = {}
    let totalPooled = 0

    for (const [cardType, pool] of this.pools) {
      const inUse = pool.filter(p => p.inUse).length
      poolsByType[cardType] = { total: pool.length, inUse }
      totalPooled += pool.length
    }

    return {
      totalPooled,
      poolsByType,
      loadedComponents: this.loadedComponents.size,
    }
  }

  /**
   * Clear all pools
   */
  clear(): void {
    this.pools.clear()
  }

  /**
   * Destroy the pool (cleanup timer)
   */
  destroy(): void {
    if (this.cleanupTimer) {
      clearInterval(this.cleanupTimer)
      this.cleanupTimer = null
    }
    this.clear()
    this.loadedComponents.clear()
  }
}

// ============================================================================
// Singleton Instance
// ============================================================================

export const componentPool = new ComponentPool()

// ============================================================================
// Composable for Vue components
// ============================================================================

/**
 * Composable to use pooled components in Vue
 */
export function usePooledComponent(cardType: string): {
  component: ShallowRef<Component | null>
  loading: ShallowRef<boolean>
  error: ShallowRef<Error | null>
} {
  const component = shallowRef<Component | null>(null)
  const loading = shallowRef(true)
  const error = shallowRef<Error | null>(null)

  componentPool
    .getComponent(cardType)
    .then(comp => {
      component.value = comp
      loading.value = false
    })
    .catch(err => {
      error.value = err
      loading.value = false
    })

  return { component, loading, error }
}

// ============================================================================
// Cleanup on page unload
// ============================================================================

if (typeof window !== 'undefined') {
  window.addEventListener('beforeunload', () => {
    componentPool.destroy()
  })
}
