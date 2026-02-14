import { ref, readonly, computed } from 'vue'

// Declare the Tauri internals type for TypeScript
declare global {
  interface Window {
    __TAURI_INTERNALS__?: {
      invoke: <T>(cmd: string, args?: Record<string, unknown>) => Promise<T>
    }
    __TAURI__?: Record<string, unknown>
  }
}

// Platform type
export type Platform = 'macos' | 'windows' | 'linux' | 'unknown'

// Singleton state - shared across all component instances
const isTauriApp = ref(false)
const isInitialized = ref(false)
const detectedPlatform = ref<Platform>('unknown')

/**
 * Composable to detect if the app is running inside a Tauri desktop application.
 *
 * In Tauri v2, the `__TAURI_INTERNALS__` object is injected into the window
 * when running inside the Tauri webview. This allows us to conditionally
 * show/hide features that only make sense in the desktop context.
 *
 * @example
 * ```vue
 * <script setup>
 * import { useTauri } from '@/composables/useTauri'
 * const { isTauri } = useTauri()
 * </script>
 *
 * <template>
 *   <div v-if="isTauri">Desktop-only content</div>
 * </template>
 * ```
 */
export function useTauri() {
  // Initialize detection only once
  if (!isInitialized.value) {
    detectTauri()
    detectPlatform()
    isInitialized.value = true
  }

  /**
   * Get the browser name based on platform.
   * Windows -> Edge, macOS -> Safari, Linux -> Browser
   */
  const browserName = computed(() => {
    switch (detectedPlatform.value) {
      case 'windows':
        return 'Edge'
      case 'macos':
        return 'Safari'
      case 'linux':
        return 'Firefox'
      default:
        return 'Browser'
    }
  })

  /**
   * Open a URL in the system's default browser.
   * In Tauri, uses custom command. In web browser, uses window.open.
   */
  async function openInBrowser(url: string): Promise<boolean> {
    // If not in Tauri, use regular window.open
    if (!isTauriApp.value) {
      try {
        window.open(url, '_blank')
        return true
      } catch (e) {
        console.error('Failed to open URL in browser:', e)
        return false
      }
    }

    // In Tauri, use custom command
    try {
      const internals = window.__TAURI_INTERNALS__
      if (internals?.invoke) {
        await internals.invoke('open_url', { url })
        return true
      }
      // Fallback to window.open if invoke not available
      window.open(url, '_blank')
      return true
    } catch (e) {
      console.error('Failed to open URL in browser:', e)
      // Try fallback
      try {
        window.open(url, '_blank')
        return true
      } catch {
        return false
      }
    }
  }

  async function setCloseBehavior(behavior: string): Promise<void> {
    if (!isTauriApp.value) return
    try {
      const internals = window.__TAURI_INTERNALS__
      if (internals?.invoke) {
        await internals.invoke('set_close_behavior', { behavior })
      }
    } catch (e) {
      console.error('Failed to set close behavior:', e)
    }
  }

  return {
    /** Whether the app is running inside Tauri */
    isTauri: readonly(isTauriApp),
    /** The detected platform */
    platform: readonly(detectedPlatform),
    /** Browser name based on platform */
    browserName,
    /** Open a URL in the system's default browser */
    openInBrowser,
    /** Set close behavior (quit or minimize to tray) */
    setCloseBehavior,
  }
}

/**
 * Detect if running inside Tauri by checking for the injected internals object.
 * This works for Tauri v2.x.
 */
function detectTauri(): void {
  // Check for Tauri v2 internals
  if (typeof window !== 'undefined' && '__TAURI_INTERNALS__' in window) {
    isTauriApp.value = true
    return
  }

  // Fallback: check for Tauri v1 style (for backwards compatibility)
  if (typeof window !== 'undefined' && '__TAURI__' in window) {
    isTauriApp.value = true
    return
  }

  isTauriApp.value = false
}

/**
 * Detect the current platform based on navigator.userAgent.
 */
function detectPlatform(): void {
  if (typeof navigator === 'undefined') {
    detectedPlatform.value = 'unknown'
    return
  }

  const ua = navigator.userAgent.toLowerCase()

  if (ua.includes('mac')) {
    detectedPlatform.value = 'macos'
  } else if (ua.includes('win')) {
    detectedPlatform.value = 'windows'
  } else if (ua.includes('linux')) {
    detectedPlatform.value = 'linux'
  } else {
    detectedPlatform.value = 'unknown'
  }
}

/**
 * Force re-detection of Tauri environment.
 * Useful for testing or when the detection needs to be refreshed.
 */
export function refreshTauriDetection(): void {
  detectTauri()
}
