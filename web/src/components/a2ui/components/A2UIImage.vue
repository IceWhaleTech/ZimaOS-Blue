<template>
  <figure class="a2ui-image" :style="componentStyle">
    <img :src="src" :alt="alt" class="image" @error="handleError" />
    <figcaption v-if="caption" class="caption">{{ caption }}</figcaption>
  </figure>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import type { Component } from '@/api/a2ui'

const props = defineProps<{
  component: Component
}>()

const hasError = ref(false)

const src = computed(() => props.component.props?.src as string || '')
const alt = computed(() => props.component.props?.alt as string || '')
const caption = computed(() => props.component.props?.caption as string)

const componentStyle = computed(() => {
  const style = props.component.style || {}
  return {
    maxWidth: (style.maxWidth as string) || '100%',
    borderRadius: style.borderRadius as string | undefined,
  }
})

function handleError() {
  hasError.value = true
}
</script>

<style scoped>
.a2ui-image {
  margin: 0;
  display: inline-block;
}

.image {
  max-width: 100%;
  height: auto;
  display: block;
  border-radius: inherit;
}

.caption {
  margin-top: 0.5rem;
  font-size: 0.875rem;
  color: var(--color-text-muted, #6b7280);
  text-align: center;
}
</style>
