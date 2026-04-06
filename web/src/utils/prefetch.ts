/**
 * Route prefetch utilities for optimizing navigation performance
 */

// Cache for prefetched routes
const prefetchedRoutes = new Set<string>()

// Route component map for prefetching
const routeComponents: Record<string, () => Promise<unknown>> = {
  Home: () => import('@/views/HomeView.vue'),
  Chat: () => import('@/views/ChatView.vue'),
  Channels: () => import('@/views/ChannelsView.vue'),
  Operations: () => import('@/views/CronView.vue'),
  Settings: () => import('@/views/SettingsView.vue'),
  Plugins: () => import('@/views/PluginsView.vue'),
  Profile: () => import('@/views/ProfileView.vue'),
  Security: () => import('@/views/SecurityView.vue'),
}

/**
 * Prefetch a route component
 * Call this on mouseenter/focus of navigation links
 */
export function prefetchRoute(routeName: string): void {
  if (prefetchedRoutes.has(routeName)) return

  const loader = routeComponents[routeName]
  if (loader) {
    // Use requestIdleCallback for non-blocking prefetch
    if ('requestIdleCallback' in window) {
      requestIdleCallback(() => {
        loader()
        prefetchedRoutes.add(routeName)
      })
    } else {
      // Fallback for Safari
      setTimeout(() => {
        loader()
        prefetchedRoutes.add(routeName)
      }, 100)
    }
  }
}

/**
 * Prefetch critical routes after initial load
 * Call this after the app is mounted
 */
export function prefetchCriticalRoutes(): void {
  // Wait for idle time before prefetching
  if ('requestIdleCallback' in window) {
    requestIdleCallback(
      () => {
        // Prefetch most commonly visited routes
        prefetchRoute('Chat')
        prefetchRoute('Home')
        prefetchRoute('Settings')
        prefetchRoute('Plugins')
      },
      { timeout: 3000 }
    )
  }
}
