import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { setLocale, getLocale, localeOptions, type LocaleKey } from '@/i18n'

export const useLocaleStore = defineStore('locale', () => {
  const currentLocale = ref<LocaleKey>(getLocale())
  const loading = ref(false)

  const options = computed(() => localeOptions)

  const currentLocaleName = computed(() => {
    const option = localeOptions.find((o) => o.value === currentLocale.value)
    return option?.label || currentLocale.value
  })

  async function changeLocale(locale: LocaleKey) {
    loading.value = true
    try {
      await setLocale(locale)
      currentLocale.value = locale
    } finally {
      loading.value = false
    }
  }

  // Sync store state with the actual i18n locale (e.g. after initLocale runs)
  function syncFromI18n() {
    currentLocale.value = getLocale()
  }

  async function toggleLocale() {
    const currentIndex = localeOptions.findIndex((o) => o.value === currentLocale.value)
    const nextIndex = (currentIndex + 1) % localeOptions.length
    const nextOption = localeOptions[nextIndex]
    if (nextOption) {
      await changeLocale(nextOption.value as LocaleKey)
    }
  }

  return {
    currentLocale,
    currentLocaleName,
    options,
    loading,
    changeLocale,
    syncFromI18n,
    toggleLocale,
  }
})
