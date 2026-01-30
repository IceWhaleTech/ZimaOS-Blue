<script setup lang="ts">
import { computed } from 'vue'

// Import all provider icons
import openaiIcon from '@/assets/providers/openai.svg'
import anthropicIcon from '@/assets/providers/anthropic.svg'
import googleIcon from '@/assets/providers/google.svg'
import deepseekIcon from '@/assets/providers/deepseek.svg'
import moonshotIcon from '@/assets/providers/moonshot.svg'
import azureIcon from '@/assets/providers/azure.svg'
import openrouterIcon from '@/assets/providers/openrouter.svg'
import aihubmixIcon from '@/assets/providers/aihubmix.svg'
import ollamaIcon from '@/assets/providers/ollama.svg'
import defaultIcon from '@/assets/providers/default.svg'

const props = defineProps<{
  providerId: string
  customIcon?: string
  size?: 'sm' | 'md' | 'lg' | 'xl'
}>()

const icons: Record<string, string> = {
  openai: openaiIcon,
  anthropic: anthropicIcon,
  google: googleIcon,
  deepseek: deepseekIcon,
  moonshot: moonshotIcon,
  azure: azureIcon,
  'azure-openai': azureIcon,
  openrouter: openrouterIcon,
  aihubmix: aihubmixIcon,
  ollama: ollamaIcon,
}

// Use custom icon if provided, otherwise fall back to built-in icons
const iconSrc = computed(() => {
  if (props.customIcon) {
    return props.customIcon
  }
  return icons[props.providerId] || defaultIcon
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
  <img :src="iconSrc" :alt="providerId" :class="sizeClass" class="inline-block" />
</template>