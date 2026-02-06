<script setup lang="ts">
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
</script>

<template>
  <div class="flex flex-col items-center justify-center min-h-[60vh]">
    <h1 class="text-6xl font-bold text-gray-400 dark:text-gray-300 mb-4">404</h1>
    <p class="text-xl text-gray-500 dark:text-gray-400 mb-8">{{ t('errors.notFound') }}</p>
    <RouterLink
      to="/"
      class="px-6 py-3 bg-gray-700 dark:bg-gray-700 text-white rounded-lg hover:bg-gray-700 dark:bg-gray-700 transition-colors"
    >
      {{ t('errors.goHome') }}
    </RouterLink>
  </div>
</template>
