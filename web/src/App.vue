<script setup lang="ts">
import { onMounted } from 'vue'
import DefaultLayout from '@/layouts/DefaultLayout.vue'
import NotificationContainer from '@/components/NotificationContainer.vue'

// Show window after content loads (prevents startup flash on Tauri)
onMounted(async () => {
  if (window.__TAURI__) {
    try {
      const { getCurrentWindow } = window.__TAURI__.window
      await getCurrentWindow().show()
    } catch (e) {
      console.warn('Failed to show window:', e)
    }
  }
})
</script>

<template>
  <DefaultLayout />
  <NotificationContainer />
</template>
