<script setup lang="ts">
import { computed, ref, watch, onMounted, onUnmounted } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useSystemStore } from '@/stores/system'
import { useAuthStore } from '@/stores/auth'
import { PagePermissions } from '@/api/users'
import { storeToRefs } from 'pinia'

const { t } = useI18n()
const route = useRoute()
const systemStore = useSystemStore()
const authStore = useAuthStore()
const { health } = storeToRefs(systemStore)
const { isAdmin } = storeToRefs(authStore)

// Mobile menu state
const isOpen = ref(false)

// Collapsed state (desktop only)
const SIDEBAR_COLLAPSED_KEY = 'sidebar-collapsed'
const isCollapsed = ref(false)
const isMac = typeof navigator !== 'undefined' && navigator.platform.toUpperCase().indexOf('MAC') >= 0

// Keyboard shortcut handler (Cmd+B on macOS, Alt+B on others)
function handleKeydown(e: KeyboardEvent) {
  const modifierKey = isMac ? e.metaKey : e.altKey
  if (modifierKey && e.key.toLowerCase() === 'b') {
    e.preventDefault()
    toggleCollapse()
  }
}

// Load collapsed state from localStorage and setup keyboard listener
onMounted(() => {
  const saved = localStorage.getItem(SIDEBAR_COLLAPSED_KEY)
  if (saved !== null) {
    isCollapsed.value = saved === 'true'
  }
  // Add keyboard shortcut listener
  window.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeydown)
})

// Toggle collapsed state
function toggleCollapse() {
  isCollapsed.value = !isCollapsed.value
  localStorage.setItem(SIDEBAR_COLLAPSED_KEY, String(isCollapsed.value))
}

// Close menu when route changes
watch(() => route.path, () => {
  isOpen.value = false
})

// Expose toggle function for parent components
defineExpose({
  toggle: () => { isOpen.value = !isOpen.value },
  open: () => { isOpen.value = true },
  close: () => { isOpen.value = false },
  isOpen,
  isCollapsed,
  toggleCollapse
})

// Check if user has permission for a page
const _hasPermission = (permission?: string) => {
  if (!permission) return true
  return authStore.hasPermission(permission)
}

// Nav item type (adminOnly is optional for future use)
interface NavItem {
  name: string
  path: string
  icon: string
  permission: string
  adminOnly?: boolean
}

// Define all nav items with their permissions
const allNavItems: NavItem[] = [
  {
    name: 'nav.dashboard',
    path: '/home',
    icon: 'M4 5a1 1 0 011-1h14a1 1 0 011 1v2a1 1 0 01-1 1H5a1 1 0 01-1-1V5zM4 13a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H5a1 1 0 01-1-1v-6zM16 13a1 1 0 011-1h2a1 1 0 011 1v6a1 1 0 01-1 1h-2a1 1 0 01-1-1v-6z',
    permission: PagePermissions.HOME,
  },
  {
    name: 'nav.chat',
    path: '/chat',
    icon: 'M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z',
    permission: PagePermissions.CHAT,
  },
  {
    name: 'nav.channels',
    path: '/channels',
    icon: 'M17 8h2a2 2 0 012 2v6a2 2 0 01-2 2h-2v4l-4-4H9a1.994 1.994 0 01-1.414-.586m0 0L11 14h4a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2v4l.586-.586z',
    permission: PagePermissions.CHANNELS,
  },
  {
    name: 'nav.automation',
    path: '/automation',
    icon: 'M13 10V3L4 14h7v7l9-11h-7z',
    permission: PagePermissions.AUTOMATION,
  },
  {
    name: 'nav.plugins',
    path: '/plugins',
    icon: 'M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4',
    permission: PagePermissions.PLUGINS,
  },
  {
    name: 'nav.security',
    path: '/security',
    icon: 'M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z',
    permission: PagePermissions.SECURITY,
  },
  {
    name: 'nav.settings',
    path: '/settings',
    icon: 'M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z',
    permission: PagePermissions.SETTINGS,
  },
]

// Filter nav items based on permissions
const navItems = computed(() => {
  // Show all nav items - individual pages handle their own permission checks
  return allNavItems
    .filter(item => {
      // Only filter admin-only items
      if (item.adminOnly) {
        return isAdmin.value
      }
      return true
    })
    .map(item => ({
      ...item,
      name: t(item.name),
    }))
})
</script>

<template>
  <!-- Mobile overlay -->
  <div
    v-if="isOpen"
    class="fixed inset-0 bg-black/50 z-40 lg:hidden"
    @click="isOpen = false"
  />

  <!-- Sidebar -->
  <aside
    class="glass-sidebar min-h-full flex flex-col fixed lg:relative inset-y-0 left-0 z-50 transform transition-all duration-300 ease-in-out lg:transform-none"
    :class="[
      isOpen ? 'translate-x-0' : '-translate-x-full lg:translate-x-0',
      isCollapsed ? 'w-16' : 'w-64'
    ]"
  >
    <!-- Close button for mobile -->
    <div class="lg:hidden p-3 border-b border-gray-200 dark:border-glass-border flex justify-end">
      <button
        class="p-2 rounded-lg text-gray-500 hover:bg-gray-100 dark:hover:bg-white/10"
        @click="isOpen = false"
      >
        <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
        </svg>
      </button>
    </div>

    <nav class="p-3 space-y-1 flex-1 overflow-y-auto">
      <RouterLink
        v-for="item in navItems"
        :key="item.path"
        :to="item.path"
        class="flex items-center px-4 py-2.5 rounded-lg transition-all duration-200 cursor-pointer group"
        :class="[
          route.path === item.path
            ? 'bg-accent/20 text-accent border border-accent/30'
            : 'text-gray-600 dark:text-slate-300 hover:bg-gray-100 dark:hover:bg-white/5 hover:text-gray-900 dark:hover:text-white border border-transparent',
          isCollapsed ? 'justify-center' : 'space-x-3'
        ]"
        :title="isCollapsed ? item.name : undefined"
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="h-5 w-5 transition-transform duration-200 group-hover:scale-110 flex-shrink-0"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="item.icon" />
        </svg>
        <span v-if="!isCollapsed" class="font-medium whitespace-nowrap">{{ item.name }}</span>
      </RouterLink>
    </nav>

    <!-- Bottom section: Version + Collapse toggle -->
    <div class="p-3 border-t border-gray-200 dark:border-glass-border">
      <div class="flex items-center" :class="isCollapsed ? 'justify-center' : 'justify-between'">
        <!-- Version info -->
        <div v-if="health" class="text-xs text-gray-500 dark:text-slate-400" :class="{ 'hidden': isCollapsed }">
          v{{ health.version }}
        </div>
        <!-- Collapse toggle button (desktop only) -->
        <button
          class="hidden lg:flex p-1.5 rounded-lg text-gray-400 hover:text-gray-600 dark:hover:text-slate-200 hover:bg-gray-100 dark:hover:bg-white/10 transition-colors"
          :title="(isCollapsed ? t('nav.expandSidebar') : t('nav.collapseSidebar')) + (isMac ? ' (⌘B)' : ' (Alt+B)')"
          @click="toggleCollapse"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 transition-transform duration-300" :class="isCollapsed ? 'rotate-180' : ''" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 19l-7-7 7-7m8 14l-7-7 7-7" />
          </svg>
        </button>
      </div>
    </div>
  </aside>
</template>
