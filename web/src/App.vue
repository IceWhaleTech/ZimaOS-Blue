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

// Show window after content loads (prevents startup flash on desktop)
onMounted(async () => {
  // Dismiss the inline splash screen now that Vue has rendered
  dismissSplash()

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
