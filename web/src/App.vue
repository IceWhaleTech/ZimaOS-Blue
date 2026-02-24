<script setup lang="ts">
import { onMounted } from 'vue'
import DefaultLayout from '@/layouts/DefaultLayout.vue'
import NotificationContainer from '@/components/NotificationContainer.vue'
import { useEventStream } from '@/composables/useEventStream'
import { useWebPush } from '@/composables/useWebPush'

const { connect: connectEventStream } = useEventStream()
const { subscribe: subscribeWebPush } = useWebPush()

function dismissSplash() {
  const splash = document.getElementById('app-splash')
  if (splash) {
    splash.classList.add('fade-out')
    setTimeout(() => splash.remove(), 300)
  }
}

// Show window after content loads (prevents startup flash on Tauri)
onMounted(async () => {
  // Dismiss the inline splash screen now that Vue has rendered
  dismissSplash()

  // In Tauri, on_page_load already shows the window when the HTML loads.
  // This is a backup for edge cases (e.g. window recreated from tray click).
  if (window.__TAURI_INTERNALS__?.invoke) {
    try {
      await window.__TAURI_INTERNALS__.invoke('plugin:window|show')
    } catch {
      // Ignore — window may already be visible
    }
  }

  // Subscribe to Web Push notifications (also requests permission)
  subscribeWebPush().catch(() => {})

  // Connect to SSE event stream for real-time updates
  connectEventStream()
})
</script>

<template>
  <DefaultLayout />
  <NotificationContainer />
</template>
