import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { ttsApi, type TTSProvider, type LanguagePack } from '@/api/tts'

export const useTTSStore = defineStore('tts', () => {
  // State
  const providers = ref<TTSProvider[]>([])
  const preferredProvider = ref<string>('espeak-ng')
  const languagePacks = ref<LanguagePack[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)

  // Computed
  const currentProvider = computed(() => {
    return providers.value.find(p => p.type === preferredProvider.value)
  })

  const downloadedLanguages = computed(() => {
    return languagePacks.value.filter(p => p.downloaded).map(p => p.language)
  })

  // Actions
  const loadProviders = async () => {
    try {
      loading.value = true
      error.value = null
      const response = await ttsApi.listProviders()
      providers.value = response.data.providers
      preferredProvider.value = response.data.preferred
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to load providers'
    } finally {
      loading.value = false
    }
  }

  const setPreferredProvider = async (provider: string) => {
    try {
      loading.value = true
      error.value = null
      await ttsApi.setPreferredProvider(provider)
      preferredProvider.value = provider
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to set provider'
    } finally {
      loading.value = false
    }
  }

  const loadLanguagePacks = async () => {
    try {
      loading.value = true
      error.value = null
      const response = await ttsApi.listLanguagePacks()
      languagePacks.value = response.data.packs
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to load language packs'
    } finally {
      loading.value = false
    }
  }

  const downloadLanguagePack = async (language: string) => {
    try {
      loading.value = true
      error.value = null
      await ttsApi.downloadLanguagePack(language)
      const pack = languagePacks.value.find(p => p.language === language)
      if (pack) {
        pack.downloaded = true
      }
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to download language pack'
    } finally {
      loading.value = false
    }
  }

  const deleteLanguagePack = async (language: string) => {
    try {
      loading.value = true
      error.value = null
      await ttsApi.deleteLanguagePack(language)
      const pack = languagePacks.value.find(p => p.language === language)
      if (pack) {
        pack.downloaded = false
      }
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to delete language pack'
    } finally {
      loading.value = false
    }
  }

  const downloadAllLanguagePacks = async () => {
    try {
      loading.value = true
      error.value = null
      await ttsApi.downloadAllLanguagePacks()
      // Mark all packs as downloaded
      languagePacks.value.forEach(pack => {
        pack.downloaded = true
      })
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to download all language packs'
      throw err
    } finally {
      loading.value = false
    }
  }

  return {
    // State
    providers,
    preferredProvider,
    languagePacks,
    loading,
    error,

    // Computed
    currentProvider,
    downloadedLanguages,

    // Actions
    loadProviders,
    setPreferredProvider,
    loadLanguagePacks,
    downloadLanguagePack,
    downloadAllLanguagePacks,
    deleteLanguagePack,
  }
})
