import { ref, readonly } from 'vue'

// Declare the Tauri internals type for TypeScript
declare global {
  interface Window {
    __TAURI_INTERNALS__?: Record<string, unknown>
  }
}

// Singleton state - shared across all component instances
const isTauriApp = ref(false)
const isInitialized = ref(false)

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
    isInitialized.value = true
  }

  return {
    /** Whether the app is running inside Tauri */
    isTauri: readonly(isTauriApp),
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
 * Force re-detection of Tauri environment.
 * Useful for testing or when the detection needs to be refreshed.
 */
export function refreshTauriDetection(): void {
  detectTauri()
}
