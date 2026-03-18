import { defineStore } from 'pinia'
import { ref, watch } from 'vue'

const THEME_KEY = 'zimaos-blue-theme'

export type Theme = 'light' | 'dark' | 'system'

function getStorageTheme(): Theme {
  try {
    if (typeof localStorage === 'undefined' || typeof localStorage.getItem !== 'function') {
      return 'system'
    }
    return (localStorage.getItem(THEME_KEY) as Theme) || 'system'
  } catch {
    return 'system'
  }
}

function setStorageTheme(theme: Theme): void {
  try {
    if (typeof localStorage === 'undefined' || typeof localStorage.setItem !== 'function') {
      return
    }
    localStorage.setItem(THEME_KEY, theme)
  } catch {
    // Ignore storage failures in restricted environments.
  }
}

export const useThemeStore = defineStore('theme', () => {
  const theme = ref<Theme>(getStorageTheme())
  const systemPrefersDark = ref(window.matchMedia('(prefers-color-scheme: dark)').matches)

  // Listen for system theme changes
  const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
  mediaQuery.addEventListener('change', (e) => {
    systemPrefersDark.value = e.matches
    applyTheme()
  })

  function applyTheme() {
    const isDark = theme.value === 'dark' || (theme.value === 'system' && systemPrefersDark.value)
    const root = document.documentElement

    if (isDark) {
      root.classList.add('dark')
      root.classList.remove('light')
      root.dataset.theme = 'dark'
    } else {
      root.classList.remove('dark')
      root.classList.add('light')
      root.dataset.theme = 'light'
    }
  }

  function setTheme(newTheme: Theme) {
    theme.value = newTheme
    setStorageTheme(newTheme)
    applyTheme()
  }

  function toggleTheme() {
    if (theme.value === 'dark') {
      setTheme('light')
    } else if (theme.value === 'light') {
      setTheme('system')
    } else {
      setTheme('dark')
    }
  }

  // Apply theme on init
  watch(theme, applyTheme, { immediate: true })

  return {
    theme,
    systemPrefersDark,
    setTheme,
    toggleTheme,
    applyTheme,
  }
})
