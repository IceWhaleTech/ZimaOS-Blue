<script setup lang="ts">
import { computed, ref, onMounted, onUnmounted } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useThemeStore } from '@/stores/theme'
import { useSystemStore } from '@/stores/system'
import { usePreviewStore } from '@/stores/preview'
import { storeToRefs } from 'pinia'
import NetworkAddressBar from '@/components/network/NetworkAddressBar.vue'
import PreviewBanner from '@/components/preview/PreviewBanner.vue'

import { useTauri } from '@/composables/useTauri'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const themeStore = useThemeStore()
const systemStore = useSystemStore()
const previewStore = usePreviewStore()
const { health } = storeToRefs(systemStore)
const { openInBrowser } = useTauri()

// User menu dropdown state
const showUserMenu = ref(false)
const userMenuRef = ref<HTMLElement | null>(null)

// Close menu when clicking outside
function handleClickOutside(event: MouseEvent) {
  if (userMenuRef.value && !userMenuRef.value.contains(event.target as Node)) {
    showUserMenu.value = false
  }
}

// Initialize preview store
onMounted(async () => {
  document.addEventListener('click', handleClickOutside)
  // Fetch user data if authenticated but user info not loaded
  if (authStore.isAuthenticated && !authStore.user) {
    await authStore.fetchUser()
  }
  // Initialize preview mode detection
  await previewStore.initialize()
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
})

// Emit event to toggle sidebar
const emit = defineEmits<{
  toggleSidebar: []
}>()

const themeLabel = computed(() => {
  if (themeStore.theme === 'dark') return t('common.dark')
  if (themeStore.theme === 'light') return t('common.light')
  return t('common.system')
})

function getThemeIcon(): string {
  if (themeStore.theme === 'dark') {
    return 'M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z'
  } else if (themeStore.theme === 'light') {
    return 'M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z'
  }
  return 'M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z'
}

async function handleLogout() {
  showUserMenu.value = false
  await authStore.logout()
  // Redirect to login page after logout
  router.push('/login')
}
</script>

<template>
  <header class="glass-header px-4 sm:px-6 py-3 sm:py-4 sticky top-0 z-40">
    <div class="flex items-center justify-between">
      <div class="flex items-center space-x-3 sm:space-x-4">
        <!-- Mobile menu button -->
        <button
          class="lg:hidden p-2 -ml-2 rounded-lg text-gray-500 hover:bg-gray-100 dark:hover:bg-white/10 transition-colors"
          @click="emit('toggleSidebar')"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16" />
          </svg>
        </button>
        
        <!-- Logo/Brand -->
        <div class="flex items-center space-x-2 sm:space-x-3">
          <img
            src="/logo.svg"
            alt="Logo"
            class="h-8 w-8 sm:h-10 sm:w-10 rounded-full object-contain drop-shadow-lg dark:brightness-150"
          />
          <div>
            <h1 class="text-base sm:text-lg font-semibold text-gray-900 dark:text-white leading-tight">{{ t('brand.name') }}</h1>
            <p class="text-xs text-gray-500 dark:text-slate-400 leading-tight">{{ t('brand.tagline') }}</p>
          </div>
        </div>
        <span
          v-if="health"
          class="hidden sm:inline-flex px-3 py-1 text-xs font-medium rounded-full transition-all duration-200"
          :class="health.status === 'ok'
            ? 'bg-cta/20 text-cta border border-cta/30'
            : 'bg-red-500/20 text-red-400 border border-red-500/30'"
        >
          <span class="flex items-center space-x-1.5">
            <span
              class="w-2 h-2 rounded-full animate-pulse"
              :class="health.status === 'ok' ? 'bg-cta' : 'bg-red-400'"
            ></span>
            <span>{{ health.status === 'ok' ? t('common.online') : health.status }}</span>
          </span>
        </span>
        <!-- Mobile status indicator (smaller) -->
        <span
          v-if="health"
          class="sm:hidden w-2.5 h-2.5 rounded-full animate-pulse"
          :class="health.status === 'ok' ? 'bg-cta' : 'bg-red-400'"
        ></span>
      </div>
      <div class="flex items-center space-x-1 sm:space-x-3">
        <!-- Network address bar (only in Tauri desktop app) -->
        <NetworkAddressBar class="hidden sm:flex" />
        <!-- Preview mode banner -->
        <PreviewBanner />
        <!-- GitHub repo link -->
        <a
          href="https://github.com/IceWhaleTech/ZimaOS-Blue"
          target="_blank"
          rel="noopener noreferrer"
          class="p-2 rounded-lg text-gray-500 dark:text-slate-400 hover:bg-gray-100 dark:hover:bg-white/10 hover:text-gray-700 dark:hover:text-white transition-colors"
          :title="`GitHub · ${t('brand.githubTooltip')}`"
          @click.prevent="openInBrowser('https://github.com/IceWhaleTech/ZimaOS-Blue')"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" viewBox="0 0 24 24" fill="currentColor">
            <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z"/>
          </svg>
        </a>
        <!-- Theme toggle -->
        <button
          class="p-2 rounded-lg text-gray-500 dark:text-slate-400 hover:bg-gray-100 dark:hover:bg-white/10 hover:text-gray-700 dark:hover:text-white transition-colors"
          :title="themeLabel"
          @click="themeStore.toggleTheme"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getThemeIcon()" />
          </svg>
        </button>
        <!-- User menu dropdown (hidden in preview mode) -->
        <div
          v-if="authStore.isAuthenticated && !previewStore.isPreviewMode"
          ref="userMenuRef"
          class="relative"
        >
          <button
            class="flex items-center space-x-2 px-2 py-1.5 rounded-lg transition-colors"
            :class="
              showUserMenu || route.path === '/profile'
                ? 'bg-gray-200 dark:bg-gray-600/20 text-gray-900 dark:text-gray-300'
                : 'text-gray-500 dark:text-slate-400 hover:bg-gray-100 dark:hover:bg-white/10 hover:text-gray-700 dark:hover:text-white'
            "
            @click="showUserMenu = !showUserMenu"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
            </svg>
            <span class="hidden sm:inline text-sm font-medium">{{ authStore.username }}</span>
            <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 hidden sm:block" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
            </svg>
          </button>
          <!-- Dropdown menu -->
          <div
            v-if="showUserMenu"
            class="absolute right-0 mt-1 w-40 rounded-lg bg-white dark:bg-gray-700 shadow-lg border border-gray-200 dark:border-gray-700 py-1 z-50"
          >
            <RouterLink
              to="/profile"
              class="flex items-center space-x-2 px-4 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700"
              @click="showUserMenu = false"
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
              </svg>
              <span>{{ t('nav.profile') }}</span>
            </RouterLink>
            <button
              class="w-full flex items-center space-x-2 px-4 py-2 text-sm text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20"
              @click="handleLogout"
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
              </svg>
              <span>{{ t('auth.signOut') }}</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  </header>
</template>
