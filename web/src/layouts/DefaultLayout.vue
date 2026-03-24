<script setup lang="ts">
import { RouterView, useRoute } from 'vue-router'
import {
  computed,
  ref,
  provide,
  onMounted,
  onUnmounted,
  defineAsyncComponent,
  type ComponentPublicInstance,
} from 'vue'
const FormFillerWidget = defineAsyncComponent(
  () => import('@/components/formfiller/FormFillerWidget.vue')
)
import { useFormFillerWidget } from '@/composables/useFormFillerWidget'
import { usePreviewStore } from '@/stores/preview'
import { refreshTauriDetection, useTauri } from '@/composables/useTauri'
import { useSettingsStore } from '@/stores/settings'
import { rafThrottle } from '@/utils/rafThrottle'

const AppSidebar = defineAsyncComponent(() => import('@/components/AppSidebar.vue'))
const PreviewOnboardingModal = defineAsyncComponent(
  () => import('@/components/onboarding/PreviewOnboardingModal.vue')
)
const BrowserMonitorWidget = defineAsyncComponent(
  () => import('@/components/BrowserMonitorWidget.vue')
)
const FullscreenModal = defineAsyncComponent(
  () => import('@/components/typeless/FullscreenModal.vue')
)
const GlobalVoiceWakeBanner = defineAsyncComponent(
  () => import('@/components/voicewake/GlobalVoiceWakeBanner.vue')
)

const previewStore = usePreviewStore()
const isPreviewMode = computed(() => previewStore.isPreviewMode)

const { isTauri, platform, setCloseBehavior, startWindowDragging } = useTauri()
const settingsStore = useSettingsStore()
const DESKTOP_SIDEBAR_BREAKPOINT = 1024
const WINDOW_RESIZE_PERF_ATTRIBUTE = 'data-blue-window-resizing'
const WINDOW_RESIZE_PERF_SETTLE_MS = 180

const route = useRoute()
const noPadding = computed(() => route.meta.noPadding === true)
const hideLayout = computed(() => route.meta.hideLayout === true)
const isChatRoute = computed(() => route.path.startsWith('/chat'))
const isHomeRoute = computed(() => route.name === 'Home' || route.path === '/home')
const showMacosWindowChrome = computed(() => isTauri.value && platform.value === 'macos')
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
type AppSidebarExposed = ComponentPublicInstance & {
  toggle?: () => void
  workspacePanelOpen?: boolean
  isCollapsed?: boolean
}

const sidebarRef = ref<AppSidebarExposed | null>(null)
const workspacePanelOpen = computed(() => Boolean(sidebarRef.value?.workspacePanelOpen))
const sidebarCollapsed = computed(() => Boolean(sidebarRef.value?.isCollapsed))
let windowResizePerfTimer: ReturnType<typeof window.setTimeout> | null = null

function toggleSidebar() {
  sidebarRef.value?.toggle?.()
}

function handleWindowChromeMouseDown(event: MouseEvent) {
  if (!showMacosWindowChrome.value || event.button !== 0) return
  void startWindowDragging()
}

// Provide toggle for child components (e.g. ChatView on mobile)
provide('toggleAppSidebar', toggleSidebar)
provide('hasGlobalMobileSidebarToggle', showSidebarToggle)

function syncViewportState() {
  hasHiddenSidebarViewport.value = detectHiddenSidebarViewport()
}

const syncViewportStateOnResize = rafThrottle(syncViewportState)

function markWindowResizing() {
  const root = document.documentElement
  root.setAttribute(WINDOW_RESIZE_PERF_ATTRIBUTE, 'true')
  if (windowResizePerfTimer !== null) {
    window.clearTimeout(windowResizePerfTimer)
  }
  windowResizePerfTimer = window.setTimeout(() => {
    windowResizePerfTimer = null
    root.removeAttribute(WINDOW_RESIZE_PERF_ATTRIBUTE)
  }, WINDOW_RESIZE_PERF_SETTLE_MS)
}

function clearWindowResizing() {
  if (windowResizePerfTimer !== null) {
    window.clearTimeout(windowResizePerfTimer)
    windowResizePerfTimer = null
  }
  document.documentElement.removeAttribute(WINDOW_RESIZE_PERF_ATTRIBUTE)
}

function handleWindowResize() {
  markWindowResizing()
  syncViewportStateOnResize()
}

// Form filler widget - now shows on input focus, no need for route watching
const { setup, cleanup } = useFormFillerWidget()

onMounted(() => {
  refreshTauriDetection()
  syncViewportState()
  window.addEventListener('resize', handleWindowResize)
  setup()
  // Sync close behavior setting to Tauri backend on startup
  if (isTauri.value) {
    setCloseBehavior(settingsStore.closeBehavior)
  }
})

onUnmounted(() => {
  window.removeEventListener('resize', handleWindowResize)
  syncViewportStateOnResize.cancel()
  clearWindowResizing()
  cleanup()
})
</script>

<template>
  <div
    class="app-shell h-screen overflow-hidden flex flex-col"
    :class="{
      'app-shell-home': isHomeRoute,
      'app-shell-macos-chrome': showMacosWindowChrome,
    }"
  >
    <header
      v-if="showMacosWindowChrome"
      class="layout-window-chrome"
      data-tauri-drag-region
      @mousedown="handleWindowChromeMouseDown"
    >
      <div class="layout-window-chrome-bar" aria-hidden="true" data-tauri-drag-region>
        <div class="layout-window-chrome-traffic-slot" aria-hidden="true" data-tauri-drag-region />
        <div class="layout-window-chrome-spacer" aria-hidden="true" data-tauri-drag-region />
      </div>
    </header>

    <!-- Full-screen layout without navigation for setup/login pages -->
    <div v-if="hideLayout" class="layout-public-view flex-1 min-h-0 overflow-auto">
      <RouterView />
    </div>

    <!-- Default layout with header and sidebar -->
    <div
      v-else
      class="layout-body flex flex-1 min-h-0"
      :class="{
        'layout-body-chat': isChatRoute,
        'layout-body-chat-nav-hidden': isChatRoute && sidebarCollapsed,
      }"
    >
      <Suspense>
        <AppSidebar ref="sidebarRef" />
        <template #fallback>
          <aside class="layout-sidebar-loading" aria-hidden="true">
            <div class="layout-sidebar-loading__logo" />
            <div class="layout-sidebar-loading__nav">
              <div class="layout-sidebar-loading__item" />
              <div class="layout-sidebar-loading__item" />
              <div class="layout-sidebar-loading__item is-wide" />
            </div>
            <div class="layout-sidebar-loading__footer" />
          </aside>
        </template>
      </Suspense>
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
          <RouterView v-slot="{ Component }">
            <Suspense v-if="Component">
              <component :is="Component" />
              <template #fallback>
                <div
                  class="layout-route-loading"
                  :class="{ 'layout-route-loading-chat': isChatRoute }"
                  aria-hidden="true"
                >
                  <div class="layout-route-loading__hero" />
                  <div class="layout-route-loading__line is-long" />
                  <div class="layout-route-loading__line" />
                  <div class="layout-route-loading__line is-short" />
                </div>
              </template>
            </Suspense>
            <div
              v-else
              class="layout-route-loading"
              :class="{ 'layout-route-loading-chat': isChatRoute }"
              aria-hidden="true"
            >
              <div class="layout-route-loading__hero" />
              <div class="layout-route-loading__line is-long" />
              <div class="layout-route-loading__line" />
              <div class="layout-route-loading__line is-short" />
            </div>
          </RouterView>
        </main>
      </div>
    </div>
    <!-- Form filler widget - lazy loaded, hidden on chat page -->
    <FormFillerWidget v-if="!hideLayout && route.path !== '/chat'" />
    <BrowserMonitorWidget v-if="!hideLayout" />
    <GlobalVoiceWakeBanner v-if="!hideLayout" />
    <!-- Preview mode onboarding tooltip -->
    <PreviewOnboardingModal v-if="!hideLayout && showPreviewOnboarding" />
    <!-- Fullscreen modal for code/diff/terminal cards -->
    <FullscreenModal v-if="!hideLayout" />
  </div>
</template>

<style scoped>
.app-shell {
  --layout-shell-spacing: 0.8rem;
  --layout-shell-offset: calc(var(--layout-shell-spacing) * 2);
  --workspace-dock-width: 28rem;
  --layout-viewport-height: 100vh;
  --layout-window-chrome-height: 0px;
  --layout-window-drag-height: 0px;
  --layout-content-height: calc(var(--layout-viewport-height) - var(--layout-window-chrome-height));
  position: relative;
  min-height: 0;
  height: var(--layout-viewport-height);
  background:
    radial-gradient(circle at 0% 0%, rgba(14, 165, 233, 0.1), transparent 42%),
    radial-gradient(circle at 100% 0%, rgba(45, 212, 191, 0.08), transparent 36%),
    var(--color-bg-base);
}

@supports (height: 100dvh) {
  .app-shell {
    --layout-viewport-height: 100dvh;
  }
}

.app-shell-home {
  background: var(--color-bg-base);
}

.app-shell-macos-chrome {
  --layout-window-chrome-height: 2.92rem;
  --layout-window-drag-height: 1.7rem;
  --layout-content-height: var(--layout-viewport-height);
}

.layout-window-chrome {
  position: absolute;
  inset: 0 0 auto 0;
  z-index: 70;
  height: var(--layout-window-drag-height);
  box-sizing: border-box;
  padding: 0.22rem 0.78rem 0;
  user-select: none;
  -webkit-user-select: none;
  cursor: default;
}

.layout-window-chrome-bar {
  box-sizing: border-box;
  min-height: 100%;
  display: grid;
  grid-template-columns: 4.2rem minmax(0, 1fr);
  align-items: center;
}

.layout-window-chrome-traffic-slot {
  width: 4.2rem;
  min-width: 4.2rem;
}

.layout-window-chrome-spacer {
  min-width: 0;
  min-height: 100%;
}

.layout-public-view {
  min-height: 0;
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

html[data-blue-macos-glass='true'] .layout-public-view {
  background: transparent;
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

.layout-sidebar-loading {
  width: 17rem;
  flex-shrink: 0;
  padding: 1rem 0.9rem;
  display: flex;
  flex-direction: column;
  gap: 1rem;
  border-right: 1px solid rgba(148, 163, 184, 0.16);
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.76), rgba(248, 250, 252, 0.92));
}

.layout-sidebar-loading__logo,
.layout-sidebar-loading__item,
.layout-sidebar-loading__footer,
.layout-route-loading__hero,
.layout-route-loading__line {
  position: relative;
  overflow: hidden;
  background: rgba(226, 232, 240, 0.88);
  border-radius: 999px;
}

.layout-sidebar-loading__logo::after,
.layout-sidebar-loading__item::after,
.layout-sidebar-loading__footer::after,
.layout-route-loading__hero::after,
.layout-route-loading__line::after {
  content: '';
  position: absolute;
  inset: 0;
  transform: translateX(-100%);
  background: linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.72), transparent);
  animation: layout-loading-shimmer 1.15s ease-in-out infinite;
}

.layout-sidebar-loading__logo {
  width: 7.5rem;
  height: 1.1rem;
}

.layout-sidebar-loading__nav {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.layout-sidebar-loading__item {
  height: 2.5rem;
  border-radius: 1rem;
}

.layout-sidebar-loading__item.is-wide {
  width: 88%;
}

.layout-sidebar-loading__footer {
  margin-top: auto;
  height: 3.25rem;
  border-radius: 1.2rem;
}

.layout-route-loading {
  min-height: 100%;
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 1rem;
  padding: 1.25rem;
}

.layout-route-loading-chat {
  max-width: 62rem;
  margin: 0 auto;
  width: 100%;
}

.layout-route-loading__hero {
  height: 12rem;
  border-radius: 2rem;
}

.layout-route-loading__line {
  height: 1rem;
}

.layout-route-loading__line.is-long {
  width: 92%;
}

.layout-route-loading__line.is-short {
  width: 62%;
}

@keyframes layout-loading-shimmer {
  100% {
    transform: translateX(100%);
  }
}

:root.dark .layout-sidebar-loading,
[data-theme='dark'] .layout-sidebar-loading,
html.dark .layout-sidebar-loading {
  background: linear-gradient(180deg, rgba(15, 23, 42, 0.78), rgba(15, 23, 42, 0.96));
  border-right-color: rgba(71, 85, 105, 0.32);
}

:root.dark .layout-sidebar-loading__logo,
:root.dark .layout-sidebar-loading__item,
:root.dark .layout-sidebar-loading__footer,
:root.dark .layout-route-loading__hero,
:root.dark .layout-route-loading__line,
[data-theme='dark'] .layout-sidebar-loading__logo,
[data-theme='dark'] .layout-sidebar-loading__item,
[data-theme='dark'] .layout-sidebar-loading__footer,
[data-theme='dark'] .layout-route-loading__hero,
[data-theme='dark'] .layout-route-loading__line,
html.dark .layout-sidebar-loading__logo,
html.dark .layout-sidebar-loading__item,
html.dark .layout-sidebar-loading__footer,
html.dark .layout-route-loading__hero,
html.dark .layout-route-loading__line {
  background: rgba(51, 65, 85, 0.88);
}

:root.dark .layout-sidebar-loading__logo::after,
:root.dark .layout-sidebar-loading__item::after,
:root.dark .layout-sidebar-loading__footer::after,
:root.dark .layout-route-loading__hero::after,
:root.dark .layout-route-loading__line::after,
[data-theme='dark'] .layout-sidebar-loading__logo::after,
[data-theme='dark'] .layout-sidebar-loading__item::after,
[data-theme='dark'] .layout-sidebar-loading__footer::after,
[data-theme='dark'] .layout-route-loading__hero::after,
[data-theme='dark'] .layout-route-loading__line::after,
html.dark .layout-sidebar-loading__logo::after,
html.dark .layout-sidebar-loading__item::after,
html.dark .layout-sidebar-loading__footer::after,
html.dark .layout-route-loading__hero::after,
html.dark .layout-route-loading__line::after {
  background: linear-gradient(90deg, transparent, rgba(148, 163, 184, 0.2), transparent);
}

@media (min-width: 1024px) {
  .layout-body {
    --layout-sidebar-height: calc(var(--layout-content-height) - var(--layout-shell-offset));
    padding: var(--layout-shell-spacing);
    gap: var(--layout-shell-spacing);
    align-items: stretch;
  }

  .layout-body.layout-body-chat {
    --layout-sidebar-height: calc(var(--layout-content-height) - var(--layout-shell-offset));
    padding-top: var(--layout-shell-spacing);
    padding-bottom: var(--layout-shell-spacing);
    gap: 0;
  }

  .layout-body.layout-body-chat.layout-body-chat-nav-hidden {
    padding-left: 0;
  }

  .layout-right {
    min-height: calc(var(--layout-content-height) - var(--layout-shell-offset));
    height: calc(var(--layout-content-height) - var(--layout-shell-offset));
  }

  .layout-body.layout-body-chat .layout-right {
    min-height: calc(var(--layout-content-height) - var(--layout-shell-offset));
    height: calc(var(--layout-content-height) - var(--layout-shell-offset));
  }

  .layout-body.layout-body-chat :deep(.app-sidebar) {
    min-height: calc(var(--layout-content-height) - var(--layout-shell-offset));
    height: calc(var(--layout-content-height) - var(--layout-shell-offset));
    max-height: calc(var(--layout-content-height) - var(--layout-shell-offset));
  }

  .layout-right-with-workspace {
    padding-right: calc(var(--workspace-dock-width) + var(--layout-shell-spacing));
  }
}

@media (max-width: 767px) {
  .layout-sidebar-loading {
    display: none;
  }

  .layout-route-loading {
    padding: 1rem;
  }

  .layout-route-loading__hero {
    height: 9rem;
  }

  .layout-window-chrome {
    padding-inline: 0.56rem;
  }

  .layout-window-chrome-traffic-slot {
    width: 3.2rem;
    min-width: 3.2rem;
  }

  .layout-window-chrome-bar {
    grid-template-columns: 3.2rem minmax(0, 1fr);
  }
}
</style>
