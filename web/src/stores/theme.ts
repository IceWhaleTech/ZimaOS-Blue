import { defineStore } from 'pinia'
import { ref, watch } from 'vue'

const THEME_KEY = 'zimaos-echo-theme'

export type Theme = 'light' | 'dark' | 'system'

export const useThemeStore = defineStore('theme', () => {
  const theme = ref<Theme>((localStorage.getItem(THEME_KEY) as Theme) || 'dark')
  const systemPrefersDark = ref(window.matchMedia('(prefers-color-scheme: dark)').matches)

  // Listen for system theme changes
  const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
  mediaQuery.addEventListener('change', (e) => {
    systemPrefersDark.value = e.matches
    applyTheme()
  })

  function applyTheme() {
    const isDark =
      theme.value === 'dark' || (theme.value === 'system' && systemPrefersDark.value)

    if (isDark) {
      document.documentElement.classList.add('dark')
      document.documentElement.classList.remove('light')
    } else {
      document.documentElement.classList.remove('dark')
      document.documentElement.classList.add('light')
    }
  }

  function setTheme(newTheme: Theme) {
    theme.value = newTheme
    localStorage.setItem(THEME_KEY, newTheme)
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
