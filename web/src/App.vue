<script setup lang="ts">
import { computed, onMounted, reactive, watch } from 'vue'
import DefaultLayout from '@/layouts/DefaultLayout.vue'
import NotificationContainer from '@/components/NotificationContainer.vue'
import AskUserQuestionDialog from '@/components/AskUserQuestionDialog.vue'
import { PagePermissions } from '@/api/users'
import { useEventStream } from '@/composables/useEventStream'
import { useWebPush } from '@/composables/useWebPush'
import { useAuthStore } from '@/stores/auth'
import { useProviderPoolStore } from '@/stores/providerPool'
import { usePreviewStore } from '@/stores/preview'
import { useSettingsStore } from '@/stores/settings'

const { connect: connectEventStream } = useEventStream()
const { subscribe: subscribeWebPush } = useWebPush()
const authStore = useAuthStore()
const previewStore = usePreviewStore()
const providerPoolStore = useProviderPoolStore()
const settingsStore = useSettingsStore()
const bootstrapState = reactive({
  eventStream: false,
  webPush: false,
  providers: false,
  backendSettings: false,
  claudeCode: false,
})

const hasProtectedSession = computed(() => {
  return previewStore.isPreviewMode || authStore.isAuthenticated
})

const canPrefetchProviders = computed(() => {
  return previewStore.isPreviewMode || authStore.hasPermission(PagePermissions.PROVIDERS)
})

const canPrefetchBackendSettings = computed(() => {
  return previewStore.isPreviewMode || authStore.hasPermission(PagePermissions.SETTINGS)
})

const canPrefetchClaudeCode = computed(() => {
  return previewStore.isPreviewMode || authStore.hasPermission(PagePermissions.CHAT)
})

function syncProviderSettingsFromPool() {
  const llmProviders = providerPoolStore.providers.filter((p: any) => p.type !== 'media')
  settingsStore.updateFromPoolProviders(llmProviders)
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
  bootstrapState.claudeCode = false
}

function initializeProtectedFeatures() {
  if (!hasProtectedSession.value) return

  if (!bootstrapState.webPush) {
    bootstrapState.webPush = true
    subscribeWebPush().catch(() => {})
  }

  if (!bootstrapState.eventStream) {
    bootstrapState.eventStream = true
    connectEventStream()
  }

  if (!bootstrapState.providers && canPrefetchProviders.value) {
    bootstrapState.providers = true
    providerPoolStore
      .fetchProviders()
      .then(() => {
        syncProviderSettingsFromPool()
      })
      .catch(() => {
        bootstrapState.providers = false
      })
  }

  if (!bootstrapState.backendSettings && canPrefetchBackendSettings.value) {
    bootstrapState.backendSettings = true
    settingsStore.fetchBackendSettings().catch(() => {
      bootstrapState.backendSettings = false
    })
  }

  if (!bootstrapState.claudeCode && canPrefetchClaudeCode.value) {
    bootstrapState.claudeCode = true
    settingsStore.fetchClaudeCodeEnabled().catch(() => {
      bootstrapState.claudeCode = false
    })
  }
}

watch(
  [
    () => hasProtectedSession.value,
    () => canPrefetchProviders.value,
    () => canPrefetchBackendSettings.value,
    () => canPrefetchClaudeCode.value,
  ],
  ([nextHasProtectedSession]) => {
    if (!nextHasProtectedSession) {
      resetProtectedBootstrap()
      return
    }
    initializeProtectedFeatures()
  }
)

// Show window after content loads (prevents startup flash on desktop)
onMounted(() => {
  dismissSplash()
  initializeProtectedFeatures()
})
</script>

<template>
  <DefaultLayout />
  <NotificationContainer />
  <AskUserQuestionDialog />
</template>
