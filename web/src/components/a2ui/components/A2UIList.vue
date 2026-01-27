<template>
  <div class="a2ui-list" :class="[`style-${listStyle}`]">
    <component :is="ordered ? 'ol' : 'ul'" class="list">
      <li v-for="(item, index) in items" :key="index" class="list-item">
        {{ item }}
      </li>
    </component>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { Component } from '@/api/a2ui'

const props = defineProps<{
  component: Component
}>()

const items = computed(() => {
  const itemsData = props.component.props?.items
  if (Array.isArray(itemsData)) {
    return itemsData as string[]
  }
  return []
})

const ordered = computed(() => props.component.props?.ordered as boolean || false)
const listStyle = computed(() => props.component.props?.style as string || 'default')
</script>

<style scoped>
.a2ui-list {
  margin: 0;
}

.list {
  margin: 0;
  padding-left: 1.5rem;
}

.list-item {
  padding: 0.25rem 0;
  line-height: 1.5;
}

.style-none .list {
  list-style: none;
  padding-left: 0;
}

.style-disc .list {
  list-style-type: disc;
}

.style-circle .list {
  list-style-type: circle;
}

.style-square .list {
  list-style-type: square;
}

.style-decimal .list {
  list-style-type: decimal;
}
</style>
