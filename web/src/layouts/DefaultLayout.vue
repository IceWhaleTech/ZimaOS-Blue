<script setup lang="ts">
import { RouterView, useRoute } from 'vue-router'
import { computed, ref } from 'vue'
import AppHeader from '@/components/AppHeader.vue'
import AppSidebar from '@/components/AppSidebar.vue'

const route = useRoute()
const noPadding = computed(() => route.meta.noPadding === true)

// Sidebar ref for mobile toggle
const sidebarRef = ref<InstanceType<typeof AppSidebar> | null>(null)

function toggleSidebar() {
  sidebarRef.value?.toggle()
}
</script>

<template>
  <div class="h-screen flex flex-col bg-surface-base overflow-hidden">
    <AppHeader @toggle-sidebar="toggleSidebar" />
    <div class="flex flex-1 min-h-0">
      <AppSidebar ref="sidebarRef" />
      <main class="flex-1 overflow-auto w-full" :class="{ 'p-4 sm:p-6': !noPadding }">
        <RouterView />
      </main>
    </div>
  </div>
</template>
