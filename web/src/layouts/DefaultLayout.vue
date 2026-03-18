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
    :class="{ 'app-shell-home': isHomeRoute }"
  >
    <header v-if="showMacosWindowChrome" class="layout-window-chrome">
      <div class="layout-window-chrome-bar" data-tauri-drag-region>
        <div class="layout-window-chrome-balance" aria-hidden="true" />
        <div class="layout-window-chrome-pill" data-tauri-drag-region>
          <span class="layout-window-chrome-app" data-tauri-drag-region>ZimaOS Blue</span>
          <span class="layout-window-chrome-divider" aria-hidden="true" data-tauri-drag-region />
          <span class="layout-window-chrome-section" data-tauri-drag-region>
            {{ windowChromeSectionLabel }}
          </span>
        </div>
        <div class="layout-window-chrome-balance" aria-hidden="true" />
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
  --workspace-dock-width: 28rem;
  background:
    radial-gradient(circle at 0% 0%, rgba(14, 165, 233, 0.1), transparent 42%),
    radial-gradient(circle at 100% 0%, rgba(45, 212, 191, 0.08), transparent 36%),
    var(--color-bg-base);
}

.app-shell-home {
  background: var(--color-bg-base);
}

.layout-window-chrome {
  position: relative;
  z-index: 20;
  padding: 0.32rem 0.9rem 0.02rem;
}

.layout-window-chrome-bar {
  min-height: 2.2rem;
  display: grid;
  grid-template-columns: 5.25rem minmax(0, 1fr) 5.25rem;
  align-items: center;
  user-select: none;
  -webkit-user-select: none;
  app-region: drag;
  -webkit-app-region: drag;
}

.layout-window-chrome-balance {
  min-height: 1px;
}

.layout-window-chrome-pill {
  justify-self: center;
  display: inline-flex;
  align-items: center;
  gap: 0.65rem;
  min-width: 0;
  padding: 0.46rem 0.82rem;
  border-radius: 999px;
  border: 1px solid rgba(148, 163, 184, 0.18);
  background: rgba(15, 23, 42, 0.22);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.12),
    0 18px 34px -26px rgba(2, 6, 23, 0.4);
  backdrop-filter: blur(18px) saturate(1.12);
  -webkit-backdrop-filter: blur(18px) saturate(1.12);
  app-region: drag;
  -webkit-app-region: drag;
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
  font-size: 0.75rem;
  font-weight: 650;
  color: rgba(255, 255, 255, 0.92);
}

.layout-window-chrome-section {
  font-size: 0.71rem;
  font-weight: 560;
  color: rgba(191, 219, 254, 0.86);
}

.layout-window-chrome-divider {
  width: 1px;
  height: 0.85rem;
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
  border-color: rgba(148, 163, 184, 0.22);
  background: rgba(15, 23, 42, 0.26);
}

html.light[data-blue-macos-glass='true'] .layout-window-chrome-pill,
html[data-theme='light'][data-blue-macos-glass='true'] .layout-window-chrome-pill {
  border-color: rgba(186, 203, 223, 0.44);
  background: rgba(255, 255, 255, 0.42);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.72),
    0 18px 34px -26px rgba(148, 163, 184, 0.34);
}

html.light[data-blue-macos-glass='true'] .layout-window-chrome-app,
html[data-theme='light'][data-blue-macos-glass='true'] .layout-window-chrome-app {
  color: rgba(15, 23, 42, 0.88);
}

html.light[data-blue-macos-glass='true'] .layout-window-chrome-section,
html[data-theme='light'][data-blue-macos-glass='true'] .layout-window-chrome-section {
  color: rgba(37, 99, 235, 0.8);
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

html[data-blue-macos-glass='true'] .layout-body.layout-body-chat {
  padding-top: 0.18rem;
}

@media (max-width: 767px) {
  .layout-window-chrome {
    padding-inline: 0.72rem;
  }

  .layout-window-chrome-bar {
    grid-template-columns: 4.15rem minmax(0, 1fr) 4.15rem;
  }

  .layout-window-chrome-pill {
    padding: 0.44rem 0.74rem;
  }

  .layout-window-chrome-section {
    max-width: 9rem;
    overflow: hidden;
    text-overflow: ellipsis;
  }
}
</style>
