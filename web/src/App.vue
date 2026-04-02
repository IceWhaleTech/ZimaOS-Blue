<script setup lang="ts">
import { computed, defineAsyncComponent, onMounted, reactive, ref, watch } from 'vue'
import { RouterView, useRoute, useRouter } from 'vue-router'
import { PagePermissions } from '@/constants/pagePermissions'
import { useAuthStore } from '@/stores/auth'
import { usePreviewStore } from '@/stores/preview'
import { reportStartupMark } from '@/utils/startupTrace'
import { prefetchCriticalRoutes } from '@/utils/prefetch'
import { hasStoredSessionHint } from '@/utils/authStorage'
import DefaultLayout from '@/layouts/DefaultLayout.vue'

const NotificationContainer = defineAsyncComponent(
  () => import('@/components/NotificationContainer.vue')
)
const AskUserQuestionDialog = defineAsyncComponent(
  () => import('@/components/AskUserQuestionDialog.vue')
)

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const previewStore = usePreviewStore()
const bootstrapState = reactive({
  eventStream: false,
  webPush: false,
  providers: false,
  backendSettings: false,
})

async function createProtectedBootstrapDeps() {
  const [eventStreamModule, webPushModule, providerPoolModule, settingsModule] = await Promise.all([
    import('@/composables/useEventStream'),
    import('@/composables/useWebPush'),
    import('@/stores/providerPool'),
    import('@/stores/settings'),
  ])

  const { connect } = eventStreamModule.useEventStream()
  const { subscribe } = webPushModule.useWebPush()

  return {
    connectEventStream: connect,
    subscribeWebPush: subscribe,
    providerPoolStore: providerPoolModule.useProviderPoolStore(),
    settingsStore: settingsModule.useSettingsStore(),
  }
}

type ProtectedBootstrapDeps = Awaited<ReturnType<typeof createProtectedBootstrapDeps>>

let protectedBootstrapPromise: Promise<ProtectedBootstrapDeps> | undefined

function isDesktopPortFallbackStartup(): boolean {
  if (typeof window === 'undefined' || !window.__BLUE_DESKTOP__) return false

  const expectedPort = window.location.protocol === 'https:' ? '443' : '80'
  const currentPort = String(window.location.port || '').trim()
  return !!currentPort && currentPort !== expectedPort
}

function canEagerRenderProtectedShell(): boolean {
  if (typeof window === 'undefined') return false

  const path = window.location.pathname
  const isChatStartupPath = path === '/' || path === '/chat'
  if (!isChatStartupPath) return false

  // When desktop falls back from the default loopback port, the webview origin changes
  // and origin-scoped localStorage tokens may not be visible yet. Keep the shell eager
  // so the user doesn't sit on the startup splash while the router resolves auth.
  if (isDesktopPortFallbackStartup()) {
    return true
  }

  return hasStoredSessionHint()
}

const initialRouteReady = ref(canEagerRenderProtectedShell())

const hasProtectedSession = computed(() => {
  return previewStore.isPreviewMode || authStore.isAuthenticated
})

const hideLayout = computed(() => route.meta.hideLayout === true)

const canPrefetchProviders = computed(() => {
  return previewStore.isPreviewMode || authStore.hasPermission(PagePermissions.PROVIDERS)
})

const canPrefetchBackendSettings = computed(() => {
  return previewStore.isPreviewMode || authStore.hasPermission(PagePermissions.SETTINGS)
})
let criticalRoutesPrefetched = false

async function loadProtectedBootstrapDeps(): Promise<ProtectedBootstrapDeps> {
  protectedBootstrapPromise ??= createProtectedBootstrapDeps()
  return protectedBootstrapPromise
}

function syncProviderSettingsFromPool(deps: ProtectedBootstrapDeps) {
  const llmProviders = deps.providerPoolStore.providers.filter((p) => p.type !== 'media')
  deps.settingsStore.updateFromPoolProviders(llmProviders)
}

function dismissSplash() {
  const splash = document.getElementById('app-splash')
  if (splash) {
    splash.classList.add('fade-out')
    setTimeout(() => splash.remove(), 300)
  }
}

function resetProtectedBootstrap() {
  bootstrapState.eventStream = false
  bootstrapState.webPush = false
  bootstrapState.providers = false
  bootstrapState.backendSettings = false
}

function scheduleCriticalRoutePrefetch() {
  if (criticalRoutesPrefetched || hideLayout.value) return
  criticalRoutesPrefetched = true
  prefetchCriticalRoutes()
}

async function initializeProtectedFeatures() {
  if (!hasProtectedSession.value) return
  const deps = await loadProtectedBootstrapDeps()
  if (!hasProtectedSession.value) return

  if (!bootstrapState.webPush) {
    bootstrapState.webPush = true
    deps.subscribeWebPush().catch(() => {})
  }

  if (!bootstrapState.eventStream) {
    bootstrapState.eventStream = true
    deps.connectEventStream()
  }

  if (!bootstrapState.providers && canPrefetchProviders.value) {
    bootstrapState.providers = true
    deps.providerPoolStore
      .fetchProviders()
      .then(() => {
        syncProviderSettingsFromPool(deps)
      })
      .catch(() => {
        bootstrapState.providers = false
      })
  }

  if (!bootstrapState.backendSettings && canPrefetchBackendSettings.value) {
    bootstrapState.backendSettings = true
    deps.settingsStore.fetchBackendSettings().catch(() => {
      bootstrapState.backendSettings = false
    })
  }

}

watch(
  [
    () => hasProtectedSession.value,
    () => canPrefetchProviders.value,
    () => canPrefetchBackendSettings.value,
  ],
  ([nextHasProtectedSession]) => {
    if (!nextHasProtectedSession) {
      resetProtectedBootstrap()
      return
    }
    void initializeProtectedFeatures()
  }
)

watch(
  [() => initialRouteReady.value, () => hideLayout.value],
  ([ready, nextHideLayout]) => {
    if (!ready || nextHideLayout) return
    scheduleCriticalRoutePrefetch()
  },
  { immediate: true }
)

// Show window after content loads (prevents startup flash on desktop)
onMounted(() => {
  if (initialRouteReady.value) {
    reportStartupMark('app_shell_eager_render')
  }
  dismissSplash()
  void initializeProtectedFeatures()
  void router.isReady().then(() => {
    initialRouteReady.value = true
    scheduleCriticalRoutePrefetch()
    reportStartupMark('app_initial_route_ready')
  })
})
</script>

<template>
  <div v-if="!initialRouteReady" class="app-startup-shell">
    <img src="/logo.svg" alt="ZimaOS Blue" class="app-startup-shell__logo" />
  </div>
  <RouterView v-else-if="hideLayout" />
  <DefaultLayout v-else />
  <NotificationContainer v-if="initialRouteReady" />
  <AskUserQuestionDialog v-if="initialRouteReady" />
</template>

<style scoped>
.app-startup-shell {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background:
    radial-gradient(circle at 0% 0%, rgba(14, 165, 233, 0.1), transparent 42%),
    radial-gradient(circle at 100% 0%, rgba(45, 212, 191, 0.08), transparent 36%), #f3f4f6;
}

.app-startup-shell__logo {
  width: 4rem;
  height: 4rem;
  object-fit: contain;
  opacity: 0.92;
  filter: drop-shadow(0 10px 24px rgba(15, 23, 42, 0.12));
}
</style>
