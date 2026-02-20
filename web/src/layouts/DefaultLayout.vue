<script setup lang="ts">
import { RouterView, useRoute } from 'vue-router'
import { computed, ref, provide, onMounted, onUnmounted, defineAsyncComponent } from 'vue'
import AppHeader from '@/components/AppHeader.vue'
import AppSidebar from '@/components/AppSidebar.vue'
const FormFillerWidget = defineAsyncComponent(() => import('@/components/formfiller/FormFillerWidget.vue'))
import PreviewOnboardingModal from '@/components/onboarding/PreviewOnboardingModal.vue'
import FullscreenModal from '@/components/typeless/FullscreenModal.vue'
import { useFormFillerWidget } from '@/composables/useFormFillerWidget'
import { usePreviewStore } from '@/stores/preview'
import { useTauri } from '@/composables/useTauri'
import { useSettingsStore } from '@/stores/settings'
import { storeToRefs } from 'pinia'

const previewStore = usePreviewStore()
const { isPreviewMode } = storeToRefs(previewStore)

const { isTauri, setCloseBehavior } = useTauri()
const settingsStore = useSettingsStore()

const route = useRoute()
const noPadding = computed(() => route.meta.noPadding === true)
const hideLayout = computed(() => route.meta.hideLayout === true)

// On mobile + chat page with ID, hide AppHeader to avoid double header
const windowWidth = ref(window.innerWidth)
const isChatMobile = computed(() => {
  if (route.path !== '/chat' || windowWidth.value >= 768) return false
  // Hide header only when viewing a specific conversation (has ID in query)
  const id = route.query.id as string | undefined
  return !!id
})

function onResize() { windowWidth.value = window.innerWidth }

// Sidebar ref for mobile toggle
const sidebarRef = ref<InstanceType<typeof AppSidebar> | null>(null)

function toggleSidebar() {
  sidebarRef.value?.toggle()
}

// Provide toggle for child components (e.g. ChatView on mobile)
provide('toggleAppSidebar', toggleSidebar)

// Form filler widget - now shows on input focus, no need for route watching
const { setup, cleanup } = useFormFillerWidget()

onMounted(() => {
  setup()
  window.addEventListener('resize', onResize)
  // Sync close behavior setting to Tauri backend on startup
  if (isTauri.value) {
    setCloseBehavior(settingsStore.closeBehavior)
  }
})

onUnmounted(() => {
  cleanup()
  window.removeEventListener('resize', onResize)
})
</script>

<template>
  <!-- Full-screen layout without navigation for setup/login pages -->
  <div v-if="hideLayout" class="h-screen bg-surface-base overflow-auto">
    <RouterView />
  </div>
  <!-- Default layout with header and sidebar -->
  <div v-else class="h-screen flex flex-col bg-surface-base overflow-hidden">
    <AppHeader v-if="!isChatMobile" @toggle-sidebar="toggleSidebar" />
    <div class="flex flex-1 min-h-0">
      <AppSidebar ref="sidebarRef" />
      <main class="flex-1 overflow-auto w-full" :class="{ 'p-4 sm:p-6': !noPadding }">
        <RouterView />
      </main>
    </div>
    <!-- Form filler widget - lazy loaded, hidden on chat page -->
    <FormFillerWidget v-if="route.path !== '/chat'" />
    <!-- Preview mode onboarding tooltip -->
    <PreviewOnboardingModal v-if="isPreviewMode" />
    <!-- Fullscreen modal for code/diff/terminal cards -->
    <FullscreenModal />
  </div>
</template>
