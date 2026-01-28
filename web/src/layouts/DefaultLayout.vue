<script setup lang="ts">
import { RouterView, useRoute } from 'vue-router'
import { computed, ref } from 'vue'
import AppHeader from '@/components/AppHeader.vue'
import AppSidebar from '@/components/AppSidebar.vue'

const route = useRoute()
const noPadding = computed(() => route.meta.noPadding === true)
const hideLayout = computed(() => route.meta.hideLayout === true)

// Sidebar ref for mobile toggle
const sidebarRef = ref<InstanceType<typeof AppSidebar> | null>(null)

function toggleSidebar() {
  sidebarRef.value?.toggle()
}
</script>

<template>
  <!-- Full-screen layout without navigation for setup/login pages -->
  <div v-if="hideLayout" class="h-screen bg-surface-base overflow-auto">
    <RouterView />
  </div>
  <!-- Default layout with header and sidebar -->
  <div v-else class="h-screen flex flex-col bg-surface-base overflow-hidden">
    <AppHeader @toggle-sidebar="toggleSidebar" />
    <div class="flex flex-1 min-h-0">
      <AppSidebar ref="sidebarRef" />
      <main class="flex-1 overflow-auto w-full" :class="{ 'p-4 sm:p-6': !noPadding }">
        <RouterView />
      </main>
    </div>
  </div>
</template>
