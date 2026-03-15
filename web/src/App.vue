<script setup lang="ts">
import { onMounted } from 'vue'
import DefaultLayout from '@/layouts/DefaultLayout.vue'
import NotificationContainer from '@/components/NotificationContainer.vue'
import AskUserQuestionDialog from '@/components/AskUserQuestionDialog.vue'
import { useEventStream } from '@/composables/useEventStream'
import { useWebPush } from '@/composables/useWebPush'
import { useProviderPoolStore } from '@/stores/providerPool'
import { useSettingsStore } from '@/stores/settings'

const { connect: connectEventStream } = useEventStream()
const { subscribe: subscribeWebPush } = useWebPush()
const providerPoolStore = useProviderPoolStore()
const settingsStore = useSettingsStore()

function dismissSplash() {
  const splash = document.getElementById('app-splash')
  if (splash) {
    splash.classList.add('fade-out')
    setTimeout(() => splash.remove(), 300)
  }
}

// Show window after content loads (prevents startup flash on desktop)
onMounted(async () => {
  // Dismiss the inline splash screen now that Vue has rendered
  dismissSplash()

  // Subscribe to Web Push notifications (also requests permission)
  subscribeWebPush().catch(() => {})

  // Connect to SSE event stream for real-time updates
  connectEventStream()

  // Pre-fetch provider and enhanced mode state so ChatView has data on first render
  providerPoolStore
    .fetchProviders()
    .then(() => {
      const llmProviders = providerPoolStore.providers.filter((p: any) => p.type !== 'media')
      settingsStore.updateFromPoolProviders(llmProviders)
    })
    .catch(() => {})
  settingsStore.fetchBackendSettings().catch(() => {})
  settingsStore.fetchClaudeCodeEnabled()
})
</script>

<template>
  <DefaultLayout />
  <NotificationContainer />
  <AskUserQuestionDialog />
</template>
