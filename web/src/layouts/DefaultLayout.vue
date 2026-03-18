<script setup lang="ts">
import { RouterView, useRoute } from 'vue-router'
import { computed, ref, provide, onMounted, onUnmounted, defineAsyncComponent } from 'vue'
import AppSidebar from '@/components/AppSidebar.vue'
const FormFillerWidget = defineAsyncComponent(
  () => import('@/components/formfiller/FormFillerWidget.vue')
)
import PreviewOnboardingModal from '@/components/onboarding/PreviewOnboardingModal.vue'
import FullscreenModal from '@/components/typeless/FullscreenModal.vue'
import GlobalVoiceWakeBanner from '@/components/voicewake/GlobalVoiceWakeBanner.vue'
import { useFormFillerWidget } from '@/composables/useFormFillerWidget'
import { usePreviewStore } from '@/stores/preview'
import { useTauri } from '@/composables/useTauri'
import { useSettingsStore } from '@/stores/settings'

const previewStore = usePreviewStore()
const isPreviewMode = computed(() => previewStore.isPreviewMode)

const { isTauri, setCloseBehavior } = useTauri()
const settingsStore = useSettingsStore()
const DESKTOP_SIDEBAR_BREAKPOINT = 1024

const route = useRoute()
const noPadding = computed(() => route.meta.noPadding === true)
const hideLayout = computed(() => route.meta.hideLayout === true)
const isChatRoute = computed(() => route.path.startsWith('/chat'))
const isHomeRoute = computed(() => route.name === 'Home' || route.path === '/home')
const showPreviewOnboarding = computed(
  () =>
    isPreviewMode.value &&
    isChatRoute.value &&
    settingsStore.claudeCodeEnabledLoaded &&
    !settingsStore.claudeCodeEnabled
)
function detectHiddenSidebarViewport() {
  if (typeof window === 'undefined') return false
  return window.innerWidth < DESKTOP_SIDEBAR_BREAKPOINT
}

const hasHiddenSidebarViewport = ref(detectHiddenSidebarViewport())
const showSidebarToggle = computed(() => hasHiddenSidebarViewport.value && !isChatRoute.value)

// Sidebar ref for mobile toggle
type AppSidebarExposed = InstanceType<typeof AppSidebar> & {
  workspacePanelOpen?: boolean
}

const sidebarRef = ref<AppSidebarExposed | null>(null)
const workspacePanelOpen = computed(() => Boolean(sidebarRef.value?.workspacePanelOpen))
const sidebarCollapsed = computed(() => Boolean(sidebarRef.value?.isCollapsed))

function toggleSidebar() {
  sidebarRef.value?.toggle()
}

// Provide toggle for child components (e.g. ChatView on mobile)
provide('toggleAppSidebar', toggleSidebar)
provide('hasGlobalMobileSidebarToggle', showSidebarToggle)

function syncViewportState() {
  hasHiddenSidebarViewport.value = detectHiddenSidebarViewport()
}

// Form filler widget - now shows on input focus, no need for route watching
const { setup, cleanup } = useFormFillerWidget()

onMounted(() => {
  syncViewportState()
  window.addEventListener('resize', syncViewportState)
  setup()
  // Sync close behavior setting to Tauri backend on startup
  if (isTauri.value) {
    setCloseBehavior(settingsStore.closeBehavior)
  }
})

onUnmounted(() => {
  window.removeEventListener('resize', syncViewportState)
  cleanup()
})
</script>

<template>
  <!-- Full-screen layout without navigation for setup/login pages -->
  <div v-if="hideLayout" class="h-screen bg-surface-base overflow-auto">
    <RouterView />
  </div>
  <!-- Default layout with header and sidebar -->
  <div v-else class="app-shell h-screen overflow-hidden" :class="{ 'app-shell-home': isHomeRoute }">
    <div
      class="layout-body flex h-full min-h-0"
      :class="{
        'layout-body-chat': isChatRoute,
        'layout-body-chat-nav-hidden': isChatRoute && sidebarCollapsed,
      }"
    >
      <AppSidebar ref="sidebarRef" />
      <div
        class="layout-right flex-1 min-w-0 min-h-0 flex flex-col"
        :class="{ 'layout-right-with-workspace': workspacePanelOpen }"
      >
        <main
          class="layout-main flex-1 w-full relative z-0"
          :class="[
            isChatRoute ? 'overflow-hidden' : 'overflow-auto',
            { 'p-[0.9rem] sm:p-[1.125rem] lg:p-[1.35rem]': !noPadding },
          ]"
        >
          <div v-if="showSidebarToggle" class="layout-mobile-nav-bar lg:hidden">
            <button
              class="layout-mobile-nav-button"
              aria-label="Open navigation"
              @click="toggleSidebar"
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="h-5 w-5"
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
          </div>
          <RouterView />
        </main>
      </div>
    </div>
    <!-- Form filler widget - lazy loaded, hidden on chat page -->
    <FormFillerWidget v-if="route.path !== '/chat'" />
    <GlobalVoiceWakeBanner />
    <!-- Preview mode onboarding tooltip -->
    <PreviewOnboardingModal v-if="showPreviewOnboarding" />
    <!-- Fullscreen modal for code/diff/terminal cards -->
    <FullscreenModal />
  </div>
</template>

<style scoped>
.app-shell {
  --layout-shell-spacing: 0.8rem;
  --layout-shell-offset: calc(var(--layout-shell-spacing) * 2);
  --workspace-dock-width: 28rem;
  background:
    radial-gradient(circle at 0% 0%, rgba(14, 165, 233, 0.1), transparent 42%),
    radial-gradient(circle at 100% 0%, rgba(45, 212, 191, 0.08), transparent 36%),
    var(--color-bg-base);
}

.app-shell-home {
  background: var(--color-bg-base);
}

html[data-blue-macos-glass='true'] .app-shell {
  background:
    radial-gradient(circle at 0% 0%, rgba(56, 189, 248, 0.18), transparent 42%),
    radial-gradient(circle at 100% 0%, rgba(45, 212, 191, 0.14), transparent 36%),
    linear-gradient(180deg, rgba(15, 23, 42, 0.68) 0%, rgba(15, 23, 42, 0.52) 100%);
}

html[data-blue-macos-glass='true'] .app-shell-home {
  background: linear-gradient(180deg, rgba(15, 23, 42, 0.62) 0%, rgba(15, 23, 42, 0.48) 100%);
}

html.light[data-blue-macos-glass='true'] .app-shell,
html[data-theme='light'][data-blue-macos-glass='true'] .app-shell {
  background:
    radial-gradient(circle at 0% 0%, rgba(96, 165, 250, 0.22), transparent 42%),
    radial-gradient(circle at 100% 0%, rgba(45, 212, 191, 0.16), transparent 34%),
    linear-gradient(180deg, rgba(255, 255, 255, 0.74) 0%, rgba(241, 245, 249, 0.64) 100%);
}

html.light[data-blue-macos-glass='true'] .app-shell-home,
html[data-theme='light'][data-blue-macos-glass='true'] .app-shell-home {
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.7) 0%, rgba(241, 245, 249, 0.6) 100%);
}

.layout-body {
  min-height: 0;
}

.layout-right {
  min-height: 0;
  overflow: hidden;
  transition: padding-right 0.22s ease;
}

.layout-main {
  min-height: 0;
}

.layout-mobile-nav-bar {
  display: flex;
  padding-bottom: 0.9rem;
}

.layout-mobile-nav-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 2.75rem;
  height: 2.75rem;
  border-radius: 1rem;
  border: 1px solid rgba(148, 163, 184, 0.26);
  color: #0f172a;
  background: rgba(255, 255, 255, 0.78);
  box-shadow: 0 14px 26px -24px rgba(15, 23, 42, 0.28);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
}

.layout-mobile-nav-button:hover {
  background: rgba(255, 255, 255, 0.9);
}

:root.dark .layout-mobile-nav-button,
[data-theme='dark'] .layout-mobile-nav-button,
html.dark .layout-mobile-nav-button {
  color: #e2e8f0;
  background: rgba(15, 23, 42, 0.7);
  border-color: rgba(100, 116, 139, 0.34);
  box-shadow: 0 14px 26px -22px rgba(2, 6, 23, 0.6);
}

:root.dark .layout-mobile-nav-button:hover,
[data-theme='dark'] .layout-mobile-nav-button:hover,
html.dark .layout-mobile-nav-button:hover {
  background: rgba(15, 23, 42, 0.82);
}

@media (min-width: 1024px) {
  .layout-body {
    padding: var(--layout-shell-spacing);
    gap: var(--layout-shell-spacing);
    align-items: stretch;
  }

  .layout-body.layout-body-chat {
    padding-bottom: var(--layout-shell-spacing);
    gap: 0;
  }

  .layout-body.layout-body-chat.layout-body-chat-nav-hidden {
    padding-left: 0;
  }

  .layout-right {
    min-height: calc(100vh - var(--layout-shell-offset));
    height: calc(100vh - var(--layout-shell-offset));
  }

  .layout-body.layout-body-chat .layout-right {
    min-height: calc(100vh - var(--layout-shell-offset));
    height: calc(100vh - var(--layout-shell-offset));
  }

  .layout-body.layout-body-chat :deep(.app-sidebar) {
    min-height: calc(100vh - var(--layout-shell-offset));
    height: calc(100vh - var(--layout-shell-offset));
    max-height: calc(100vh - var(--layout-shell-offset));
  }

  .layout-right-with-workspace {
    padding-right: calc(var(--workspace-dock-width) + var(--layout-shell-spacing));
  }
}
</style>
