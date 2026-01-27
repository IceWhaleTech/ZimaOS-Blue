<template>
  <div class="a2ui-text" :style="componentStyle">
    {{ content }}
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { Component } from '@/api/a2ui'

const props = defineProps<{
  component: Component
}>()

const content = computed(() => props.component.props?.content as string || '')

const componentStyle = computed(() => {
  const style = props.component.style || {}
  return {
    fontSize: style.fontSize,
    fontWeight: style.fontWeight,
    color: style.color,
    textAlign: style.textAlign,
  }
})
</script>

<style scoped>
.a2ui-text {
  line-height: 1.6;
}
</style>
