import { ref, readonly, computed } from 'vue'
import { systemApi } from '@/api/system'
import { isCurrentHostLoopback, isLocalAbsolutePath } from '@/utils/localPath'
import { isProtectedResourceUrl, openProtectedResource } from '@/utils/protectedResource'

// Declare the desktop marker type for TypeScript
declare global {
  interface Window {
    __TAURI_INTERNALS__?: {
      invoke: <T>(cmd: string, args?: Record<string, unknown>) => Promise<T>
    }
    __TAURI__?: Record<string, unknown>
    __BLUE_DESKTOP__?: boolean
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
    const trimmedUrl = url.trim()
    if (!trimmedUrl) return false

    // If caller asks to open a local absolute path, first try native reveal.
    if (isLocalAbsolutePath(trimmedUrl)) {
      const revealed = await revealInFileManager(trimmedUrl)
      if (revealed) return true

      const downloadUrl = await resolveLocalFileDownloadURL(trimmedUrl)
      if (downloadUrl) {
        try {
          return await openProtectedResource(downloadUrl)
        } catch (e) {
          console.error('Failed to open local file download URL:', e)
        }
      }

      if (!isTauriApp.value) {
        // In web mode, local absolute filesystem paths are not valid browser URLs.
        return false
      }
    }

    if (isProtectedResourceUrl(trimmedUrl)) {
      try {
        return await openProtectedResource(trimmedUrl)
      } catch (e) {
        console.error('Failed to open protected URL in browser:', e)
        return false
      }
    }

    // If not in Tauri, use regular window.open
    if (!isTauriApp.value) {
      try {
        window.open(trimmedUrl, '_blank')
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
        await internals.invoke('open_url', { url: trimmedUrl })
        return true
      }
      // Fallback to window.open if invoke not available
      window.open(trimmedUrl, '_blank')
      return true
    } catch (e) {
      console.error('Failed to open URL in browser:', e)
      // Try fallback
      try {
        window.open(trimmedUrl, '_blank')
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

  async function setTrayLocale(locale: string): Promise<void> {
    if (!isTauriApp.value) return
    try {
      const internals = window.__TAURI_INTERNALS__
      if (internals?.invoke) {
        await internals.invoke('set_tray_locale', { locale })
      }
    } catch (e) {
      console.error('Failed to set tray locale:', e)
    }
  }

  /**
   * Reveal a local path in the system file manager.
   * Only available in desktop runtime.
   */
  async function revealInFileManager(path: string): Promise<boolean> {
    const trimmedPath = path.trim()
    if (!trimmedPath) return false
    if (!isLocalAbsolutePath(trimmedPath)) return false

    const revealViaLoopbackApi = async (): Promise<boolean> => {
      if (!isCurrentHostLoopback()) return false
      try {
        await systemApi.revealPath(trimmedPath)
        return true
      } catch (e) {
        console.error('Failed to reveal path via loopback API:', e)
        return false
      }
    }

    if (!isTauriApp.value) {
      return revealViaLoopbackApi()
    }

    try {
      const internals = window.__TAURI_INTERNALS__
      if (internals?.invoke) {
        await internals.invoke('reveal_path', { path: trimmedPath })
        return true
      }
      // External localhost pages in desktop runtime may not have Tauri IPC.
      return revealViaLoopbackApi()
    } catch (e) {
      console.error('Failed to reveal path in file manager:', e)
      return revealViaLoopbackApi()
    }
  }

  async function resolveLocalFileDownloadURL(path: string): Promise<string | null> {
    try {
      const response = await systemApi.resolveLocalFile(path)
      const data = response.data
      const downloadURL = String(data.download_url || '').trim()
      return downloadURL || null
    } catch (e) {
      console.error('Failed to resolve local file download URL:', e)
      return null
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
    /** Sync tray menu language with app locale */
    setTrayLocale,
    /** Reveal path in system file manager */
    revealInFileManager,
  }
}

/**
 * Detect if running inside the desktop app by checking for the injected marker.
 * The __BLUE_DESKTOP__ flag is set by the Tauri on_page_load handler.
 */
function detectTauri(): void {
  if (typeof window !== 'undefined' && (window as any).__BLUE_DESKTOP__) {
    isTauriApp.value = true
    return
  }

  // Fallback: check for Tauri v2/v1 internals (e.g. when loaded via asset protocol)
  if (typeof window !== 'undefined' && ('__TAURI_INTERNALS__' in window || '__TAURI__' in window)) {
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
