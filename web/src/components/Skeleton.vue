<script setup lang="ts">
defineProps<{
  width?: string
  height?: string
  rounded?: 'none' | 'sm' | 'md' | 'lg' | 'full'
  class?: string
}>()
</script>

<template>
  <div
    class="animate-pulse bg-gray-200 dark:bg-gray-700"
    :class="[
      rounded === 'none' ? '' : rounded === 'sm' ? 'rounded-sm' : rounded === 'md' ? 'rounded-md' : rounded === 'lg' ? 'rounded-lg' : rounded === 'full' ? 'rounded-full' : 'rounded',
      $props.class
    ]"
    :style="{
      width: width || '100%',
      height: height || '1rem'
    }"
  />
</template>
