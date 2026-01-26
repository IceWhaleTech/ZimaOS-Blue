<script setup lang="ts">
import { RouterLink } from 'vue-router'
</script>

<template>
  <div class="flex flex-col items-center justify-center min-h-[60vh]">
    <h1 class="text-6xl font-bold text-gray-300 mb-4">404</h1>
    <p class="text-xl text-gray-600 mb-8">Page not found</p>
    <RouterLink
      to="/"
      class="px-6 py-3 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors"
    >
      Go Home
    </RouterLink>
  </div>
</template>
