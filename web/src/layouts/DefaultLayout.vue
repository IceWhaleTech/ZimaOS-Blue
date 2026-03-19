<script setup lang="ts">
import { RouterView, useRoute } from 'vue-router'
import { computed, ref, provide, onMounted, onUnmounted, defineAsyncComponent } from 'vue'
import { useI18n } from 'vue-i18n'
import AppSidebar from '@/components/AppSidebar.vue'
const FormFillerWidget = defineAsyncComponent(
  () => import('@/components/formfiller/FormFillerWidget.vue')
)
import PreviewOnboardingModal from '@/components/onboarding/PreviewOnboardingModal.vue'
import FullscreenModal from '@/components/typeless/FullscreenModal.vue'
import GlobalVoiceWakeBanner from '@/components/voicewake/GlobalVoiceWakeBanner.vue'
import { useFormFillerWidget } from '@/composables/useFormFillerWidget'
import { usePreviewStore } from '@/stores/preview'
import { refreshTauriDetection, useTauri } from '@/composables/useTauri'
import { useSettingsStore } from '@/stores/settings'

const previewStore = usePreviewStore()
const isPreviewMode = computed(() => previewStore.isPreviewMode)
const { t, te } = useI18n()

const { isTauri, platform, setCloseBehavior } = useTauri()
const settingsStore = useSettingsStore()
const DESKTOP_SIDEBAR_BREAKPOINT = 1024

const route = useRoute()
const noPadding = computed(() => route.meta.noPadding === true)
const hideLayout = computed(() => route.meta.hideLayout === true)
const isChatRoute = computed(() => route.path.startsWith('/chat'))
const isHomeRoute = computed(() => route.name === 'Home' || route.path === '/home')
const showMacosWindowChrome = computed(() => isTauri.value && platform.value === 'macos')
const showWindowChromeLabel = computed(() => !isChatRoute.value)
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

function chromeLabel(key: string, fallback: string) {
  return te(key) ? t(key) : fallback
}

const windowChromeSectionLabel = computed(() => {
  const routeName = typeof route.name === 'string' ? route.name : ''
  switch (routeName) {
    case 'Chat':
      return chromeLabel('nav.chat', 'Chat')
    case 'Home':
      return chromeLabel('nav.dashboard', 'Dashboard')
    case 'Settings':
      return chromeLabel('nav.settings', 'Settings')
    case 'Plugins':
      return chromeLabel('nav.plugins', 'Extensions')
    case 'Profile':
      return chromeLabel('nav.profile', 'Profile')
    case 'VoiceChat':
      return chromeLabel('voiceView.title', 'Voice')
    case 'Channels':
      return chromeLabel('nav.channels', 'Channels')
    case 'Security':
      return chromeLabel('nav.security', 'Security')
    case 'CronJobs':
      return chromeLabel('nav.automation', 'Automation')
    case 'AuditLogs':
      return chromeLabel('audit.title', 'Audit')
    case 'Billing':
      return chromeLabel('nav.billing', 'Billing')
    case 'Tenants':
      return chromeLabel('tenants.title', 'Workspaces')
    case 'TenantDetail':
      return chromeLabel('tenants.title', 'Workspaces')
    case 'AuthProviders':
      return chromeLabel('authProviders.title', 'Authentication Providers')
    case 'Users':
      return chromeLabel('nav.users', 'Users')
    case 'Login':
      return chromeLabel('auth.signIn', 'Sign in')
    case 'ConnectionError':
      return chromeLabel('connection.quality.offline', 'Offline')
    default:
      return routeName ? routeName.replace(/([a-z])([A-Z])/g, '$1 $2') : 'ZimaOS Blue'
  }
})

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
  refreshTauriDetection()
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
    >
      <div class="layout-window-chrome-bar" aria-hidden="true">
        <div class="layout-window-chrome-traffic-slot" aria-hidden="true" />
        <div v-if="showWindowChromeLabel" class="layout-window-chrome-pill">
          <span class="layout-window-chrome-app">ZimaOS Blue</span>
          <span class="layout-window-chrome-divider" aria-hidden="true" />
          <span class="layout-window-chrome-section">{{ windowChromeSectionLabel }}</span>
        </div>
        <div class="layout-window-chrome-trailing" aria-hidden="true" />
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
    <FormFillerWidget v-if="!hideLayout && route.path !== '/chat'" />
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
  --layout-chat-top-spacing: var(--layout-shell-spacing);
  --layout-chat-bottom-spacing: var(--layout-shell-spacing);
  --layout-chat-vertical-offset: calc(
    var(--layout-chat-top-spacing) + var(--layout-chat-bottom-spacing)
  );
  --workspace-dock-width: 28rem;
  --layout-viewport-height: 100vh;
  --layout-window-chrome-height: 0px;
  --layout-content-height: calc(
    var(--layout-viewport-height) - var(--layout-window-chrome-height)
  );
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
  --layout-chat-top-spacing: 0.34rem;
  --layout-content-height: var(--layout-viewport-height);
}

.layout-window-chrome {
  position: absolute;
  inset: 0 0 auto 0;
  z-index: 20;
  height: var(--layout-window-chrome-height);
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
  grid-template-columns: 4.2rem minmax(0, 1fr) 4.2rem;
  align-items: center;
}

.layout-window-chrome-bar,
.layout-window-chrome-bar * {
  pointer-events: none;
}

.layout-window-chrome-pill {
  justify-self: center;
  display: inline-flex;
  align-items: center;
  gap: 0.42rem;
  min-width: 0;
  max-width: min(100%, 20rem);
  padding: 0;
}

.layout-window-chrome-traffic-slot {
  width: 4.2rem;
  min-width: 4.2rem;
}

.layout-window-chrome-trailing {
  width: 4.2rem;
  min-width: 4.2rem;
}

.layout-window-chrome-app,
.layout-window-chrome-section {
  display: inline-flex;
  align-items: center;
  min-width: 0;
  line-height: 1;
  letter-spacing: -0.02em;
  white-space: nowrap;
}

.layout-window-chrome-app {
  font-size: 0.68rem;
  font-weight: 520;
  color: rgba(15, 23, 42, 0.46);
}

.layout-window-chrome-section {
  font-size: 0.8rem;
  font-weight: 600;
  color: rgba(15, 23, 42, 0.84);
}

.layout-window-chrome-divider {
  width: 1px;
  height: 0.72rem;
  background: rgba(148, 163, 184, 0.24);
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

html[data-blue-macos-glass='true'] .layout-window-chrome-pill {
  background: transparent;
}

html.light[data-blue-macos-glass='true'] .layout-window-chrome-pill,
html[data-theme='light'][data-blue-macos-glass='true'] .layout-window-chrome-pill {
  background: transparent;
}

html.light[data-blue-macos-glass='true'] .layout-window-chrome-app,
html[data-theme='light'][data-blue-macos-glass='true'] .layout-window-chrome-app {
  color: rgba(15, 23, 42, 0.42);
}

html.light[data-blue-macos-glass='true'] .layout-window-chrome-section,
html[data-theme='light'][data-blue-macos-glass='true'] .layout-window-chrome-section {
  color: rgba(15, 23, 42, 0.82);
}

html.dark[data-blue-macos-glass='true'] .layout-window-chrome-app,
html[data-theme='dark'][data-blue-macos-glass='true'] .layout-window-chrome-app {
  color: rgba(255, 255, 255, 0.44);
}

html.dark[data-blue-macos-glass='true'] .layout-window-chrome-section,
html[data-theme='dark'][data-blue-macos-glass='true'] .layout-window-chrome-section {
  color: rgba(255, 255, 255, 0.86);
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

@media (min-width: 1024px) {
  .layout-body {
    --layout-sidebar-height: calc(var(--layout-content-height) - var(--layout-shell-offset));
    padding: var(--layout-shell-spacing);
    gap: var(--layout-shell-spacing);
    align-items: stretch;
  }

  .layout-body.layout-body-chat {
    --layout-sidebar-height: calc(
      var(--layout-content-height) - var(--layout-chat-vertical-offset)
    );
    padding-top: var(--layout-chat-top-spacing);
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
    min-height: calc(var(--layout-content-height) - var(--layout-chat-vertical-offset));
    height: calc(var(--layout-content-height) - var(--layout-chat-vertical-offset));
  }

  .layout-body.layout-body-chat :deep(.app-sidebar) {
    min-height: calc(var(--layout-content-height) - var(--layout-chat-vertical-offset));
    height: calc(var(--layout-content-height) - var(--layout-chat-vertical-offset));
    max-height: calc(var(--layout-content-height) - var(--layout-chat-vertical-offset));
  }

  .layout-right-with-workspace {
    padding-right: calc(var(--workspace-dock-width) + var(--layout-shell-spacing));
  }
}

@media (max-width: 767px) {
  .layout-window-chrome {
    padding-inline: 0.56rem;
  }

  .layout-window-chrome-pill {
    max-width: 100%;
  }

  .layout-window-chrome-traffic-slot {
    width: 3.2rem;
    min-width: 3.2rem;
  }

  .layout-window-chrome-trailing {
    width: 3.2rem;
    min-width: 3.2rem;
  }

  .layout-window-chrome-app {
    display: none;
  }

  .layout-window-chrome-divider {
    display: none;
  }

  .layout-window-chrome-section {
    max-width: 9rem;
    overflow: hidden;
    text-overflow: ellipsis;
  }
}
</style>
