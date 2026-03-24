import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { SystemMode, PreviewStatus } from '@/api/preview'
import { getCachedPreviewMode } from '@/router'
import {
  clearStoredPreviewToken,
  getStoredPreviewToken,
  setStoredAccessToken,
  setStoredPreviewToken,
} from '@/utils/authStorage'

type PreviewModule = typeof import('@/api/preview')

let previewModulePromise: Promise<PreviewModule> | null = null

function loadPreviewModule(): Promise<PreviewModule> {
  if (!previewModulePromise) {
    previewModulePromise = import('@/api/preview')
  }
  return previewModulePromise
}

export const usePreviewStore = defineStore('preview', () => {
  // State
  const systemMode = ref<SystemMode | null>(null)
  const previewStatus = ref<PreviewStatus | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)
  const initialized = ref(false)
  const previewTokenFetched = ref(false)

  // Computed
  const isPreviewMode = computed(() => systemMode.value?.mode === 'preview')
  const features = computed(() => systemMode.value?.features || {})
  const statusMessage = computed(() => previewStatus.value?.message || '')

  // Actions
  async function fetchSystemMode() {
    try {
      loading.value = true
      error.value = null
      const { previewApi } = await loadPreviewModule()
      const response = await previewApi.getSystemMode()
      systemMode.value = response.data
      initialized.value = true

      // If in preview mode, ensure we have a token
      if (response.data.mode === 'preview') {
        await ensurePreviewToken()
      }
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch system mode'
      // Default to normal mode on error
      systemMode.value = {
        mode: 'normal',
        features: {},
      }
    } finally {
      loading.value = false
    }
  }

  async function ensurePreviewToken() {
    // Prevent multiple simultaneous token fetches
    if (previewTokenFetched.value) return

    // Check if we already have a valid token
    const existingToken = getStoredPreviewToken()
    if (existingToken) {
      // Set it as the auth token
      setStoredAccessToken(existingToken)
      previewTokenFetched.value = true
      return
    }

    // Fetch a new preview token
    try {
      const { previewApi } = await loadPreviewModule()
      const response = await previewApi.getPreviewToken()
      if (response.data.token) {
        setStoredPreviewToken(response.data.token)
        setStoredAccessToken(response.data.token)
        previewTokenFetched.value = true
      }
    } catch (e) {
      console.error('Failed to get preview token:', e)
    }
  }

  async function fetchPreviewStatus() {
    try {
      loading.value = true
      error.value = null
      const { previewApi } = await loadPreviewModule()
      const response = await previewApi.getStatus()
      previewStatus.value = response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch preview status'
    } finally {
      loading.value = false
    }
  }

  async function upgrade(username: string, password: string) {
    try {
      loading.value = true
      error.value = null
      const { previewApi } = await loadPreviewModule()
      const response = await previewApi.upgrade({ username, password })

      if (response.data.success) {
        // Clear preview token
        clearStoredPreviewToken()

        // Update system mode to normal
        systemMode.value = {
          mode: 'normal',
          features: {
            chat: true,
            attachment: true,
            image_upload: true,
            voice_play: true,
            voice_input: true,
            provider_config: true,
            settings: true,
            admin: true,
            user_management: true,
          },
        }
        return response.data
      }
      return null
    } catch (e) {
      const axiosError = e as { response?: { data?: { message?: string } } }
      error.value = axiosError.response?.data?.message || 'Failed to create admin account'
      throw e
    } finally {
      loading.value = false
    }
  }

  function clearError() {
    error.value = null
  }

  // Initialize on first use — reuses the router's cached preview mode check
  // to avoid a duplicate /api/v1/system/mode fetch.
  async function initialize() {
    if (initialized.value) return

    // The router guard already checked preview mode — reuse that result
    const cached = getCachedPreviewMode()
    if (cached.checked) {
      systemMode.value = {
        mode: cached.preview ? 'preview' : 'normal',
        features: {},
      }
      initialized.value = true
      if (cached.preview) {
        await ensurePreviewToken()
      }
      return
    }

    // Fallback: fetch if router hasn't checked yet (shouldn't happen normally)
    await fetchSystemMode()
  }

  return {
    // State
    systemMode,
    previewStatus,
    loading,
    error,
    initialized,

    // Computed
    isPreviewMode,
    features,
    statusMessage,

    // Actions
    fetchSystemMode,
    fetchPreviewStatus,
    upgrade,
    clearError,
    initialize,
    ensurePreviewToken,
  }
})
