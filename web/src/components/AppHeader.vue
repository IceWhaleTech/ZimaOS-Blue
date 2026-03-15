<script setup lang="ts">
import { onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useSystemStore } from '@/stores/system'
import { usePreviewStore } from '@/stores/preview'
import { storeToRefs } from 'pinia'
import NetworkAddressBar from '@/components/network/NetworkAddressBar.vue'
import PreviewBanner from '@/components/preview/PreviewBanner.vue'

const { t } = useI18n()
const systemStore = useSystemStore()
const previewStore = usePreviewStore()
const { health } = storeToRefs(systemStore)

onMounted(() => {
  void previewStore.initialize()
})

// Emit event to toggle sidebar
const emit = defineEmits<{
  toggleSidebar: []
}>()
</script>

<template>
  <header class="app-header sticky top-0 z-40 px-3 sm:px-5 py-3">
    <div class="header-panel flex items-center justify-between">
      <div class="flex items-center space-x-3 sm:space-x-4">
        <!-- Mobile menu button -->
        <button
          class="header-icon-btn lg:hidden p-2 -ml-2 rounded-lg text-gray-500 dark:text-slate-300 hover:text-gray-900 dark:hover:text-white transition-colors"
          @click="emit('toggleSidebar')"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-6 w-6"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M4 6h16M4 12h16M4 18h16"
            />
          </svg>
        </button>

        <!-- Logo/Brand -->
        <div class="flex items-center space-x-2 sm:space-x-3">
          <img
            src="/logo.svg"
            alt="Logo"
            class="h-8 w-8 sm:h-10 sm:w-10 rounded-2xl object-contain drop-shadow-lg dark:brightness-150"
          />
          <div>
            <h1
              class="text-base sm:text-lg font-semibold text-gray-900 dark:text-white leading-tight"
            >
              {{ t('brand.name') }}
            </h1>
            <p class="text-xs text-gray-500 dark:text-slate-400 leading-tight">
              {{ t('brand.tagline') }}
            </p>
          </div>
        </div>
        <span
          v-if="health"
          class="hidden sm:inline-flex px-3 py-1 text-xs font-medium rounded-full transition-all duration-200"
          :class="
            health.status === 'ok'
              ? 'bg-cta/20 text-cta border border-cta/30'
              : 'bg-red-500/20 text-red-400 border border-red-500/30'
          "
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
      <div class="header-actions flex items-center justify-end gap-2 sm:gap-3">
        <!-- Network address bar (only in Tauri desktop app) -->
        <NetworkAddressBar class="hidden sm:flex" />
        <!-- Preview mode banner -->
        <PreviewBanner />
      </div>
    </div>
  </header>
</template>

<style scoped>
.app-header {
  background: transparent;
}

.header-panel {
  border: 1px solid var(--glass-border);
  border-radius: 16px;
  padding: 0.7rem 0.9rem;
  background: var(--glass-bg);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  box-shadow: 0 10px 26px -20px rgba(2, 6, 23, 0.6);
}

.header-icon-btn {
  border: 1px solid transparent;
}

.header-icon-btn:hover {
  border-color: rgba(148, 163, 184, 0.35);
  background: rgba(148, 163, 184, 0.12);
}

:root.dark .header-panel,
[data-theme='dark'] .header-panel,
html.dark .header-panel {
  background: rgba(15, 23, 42, 0.72);
  border-color: rgba(100, 116, 139, 0.3);
  box-shadow: 0 14px 30px -24px rgba(2, 6, 23, 0.86);
}

:root.light .header-panel,
[data-theme='light'] .header-panel {
  background: rgba(255, 255, 255, 0.88);
  border-color: rgba(148, 163, 184, 0.32);
  box-shadow: 0 12px 26px -22px rgba(15, 23, 42, 0.35);
}

@media (max-width: 640px) {
  .header-panel {
    padding: 0.68rem 0.76rem;
  }
}
</style>
