<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  providerId: string
  customIcon?: string
  size?: 'sm' | 'md' | 'lg' | 'xl'
}>()

// Alias mapping for providers with multiple names
const aliases: Record<string, string> = {
  'azure-openai': 'azure',
  'bedrock': 'aws',
  'zimaos-blue-trial': '__trial__',
}

// Known provider icon files
const knownIcons = new Set([
  'aihubmix', 'anthropic', 'aws', 'azure', 'codex', 'deepseek', 'default',
  'glm', 'google', 'grok', 'minimax', 'moonshot', 'ngrok', 'ollama',
  'openai', 'openrouter', 'qwen', 'venice',
])

// Try to match a known icon from a custom provider ID (e.g. "custom-anthropic-cursor" → "anthropic")
function resolveIcon(id: string): string {
  if (knownIcons.has(id)) return id
  for (const known of knownIcons) {
    if (id.includes(known)) return known
  }
  return 'default'
}

const iconSrc = computed(() => {
  if (props.customIcon) return props.customIcon
  const id = aliases[props.providerId] || props.providerId
  if (id === '__trial__') return '/logo.svg'
  return `/icons/providers/${resolveIcon(id)}.svg`
})

const sizeClass = computed(() => {
  switch (props.size) {
    case 'sm': return 'w-4 h-4'
    case 'md': return 'w-5 h-5'
    case 'lg': return 'w-6 h-6'
    case 'xl': return 'w-8 h-8'
    default: return 'w-5 h-5'
  }
})
</script>

<template>
  <img
    :src="iconSrc"
    :alt="providerId"
    :class="[sizeClass, 'dark:brightness-150']"
    class="inline-block"
    @error="($event.target as HTMLImageElement).src = '/icons/providers/default.svg'"
  />
</template>
