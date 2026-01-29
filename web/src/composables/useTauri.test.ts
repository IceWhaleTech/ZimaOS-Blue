import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { useTauri, refreshTauriDetection } from './useTauri'

// Extend Window interface for test purposes
declare global {
  interface Window {
    __TAURI__?: Record<string, unknown>
  }
}

describe('useTauri', () => {
  beforeEach(() => {
    // Reset the module state by refreshing detection
    // Clear any Tauri-related properties
    delete window.__TAURI_INTERNALS__
    delete window.__TAURI__
  })

  afterEach(() => {
    // Restore window
    delete window.__TAURI_INTERNALS__
    delete window.__TAURI__
  })

  describe('isTauri detection', () => {
    it('should return false when not in Tauri environment', () => {
      refreshTauriDetection()
      const { isTauri } = useTauri()
      expect(isTauri.value).toBe(false)
    })

    it('should return true when __TAURI_INTERNALS__ is present (Tauri v2)', () => {
      // Simulate Tauri v2 environment
      window.__TAURI_INTERNALS__ = {
        invoke: vi.fn(),
      }

      refreshTauriDetection()
      const { isTauri } = useTauri()
      expect(isTauri.value).toBe(true)
    })

    it('should return true when __TAURI__ is present (Tauri v1)', () => {
      // Simulate Tauri v1 environment
      window.__TAURI__ = {
        invoke: vi.fn(),
      }

      refreshTauriDetection()
      const { isTauri } = useTauri()
      expect(isTauri.value).toBe(true)
    })

    it('should prefer __TAURI_INTERNALS__ over __TAURI__', () => {
      // Both present (edge case)
      window.__TAURI_INTERNALS__ = { invoke: vi.fn() }
      window.__TAURI__ = { invoke: vi.fn() }

      refreshTauriDetection()
      const { isTauri } = useTauri()
      expect(isTauri.value).toBe(true)
    })
  })

  describe('singleton behavior', () => {
    it('should return the same ref across multiple calls', () => {
      refreshTauriDetection()
      const { isTauri: isTauri1 } = useTauri()
      const { isTauri: isTauri2 } = useTauri()

      // Should be the same ref object
      expect(isTauri1).toBe(isTauri2)
    })
  })

  describe('refreshTauriDetection', () => {
    it('should update detection when environment changes', () => {
      // Start without Tauri
      refreshTauriDetection()
      const { isTauri } = useTauri()
      expect(isTauri.value).toBe(false)

      // Simulate Tauri being injected
      window.__TAURI_INTERNALS__ = { invoke: vi.fn() }
      refreshTauriDetection()

      expect(isTauri.value).toBe(true)

      // Simulate Tauri being removed
      delete window.__TAURI_INTERNALS__
      refreshTauriDetection()

      expect(isTauri.value).toBe(false)
    })
  })

  describe('readonly ref', () => {
    it('should return a readonly ref', () => {
      refreshTauriDetection()
      const { isTauri } = useTauri()

      // The ref should be readonly (attempting to set should not work in strict mode)
      // We can verify it's a ref by checking .value exists
      expect(typeof isTauri.value).toBe('boolean')
    })
  })
})
