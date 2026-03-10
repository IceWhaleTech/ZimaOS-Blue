<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const props = defineProps<{
  text: string
  size?: 'sm' | 'md' | 'lg'
}>()

const emit = defineEmits<{
  copied: [text: string]
}>()

const copied = ref(false)
let timeout: ReturnType<typeof setTimeout> | null = null

async function copy() {
  try {
    await navigator.clipboard.writeText(props.text)
    copied.value = true
    emit('copied', props.text)

    if (timeout) {
      clearTimeout(timeout)
    }

    timeout = setTimeout(() => {
      copied.value = false
    }, 2000)
  } catch (e) {
    console.error('Failed to copy:', e)
  }
}

const sizeClasses = {
  sm: 'p-1',
  md: 'p-1.5',
  lg: 'p-2',
}

const iconSizes = {
  sm: 'w-4 h-4',
  md: 'w-5 h-5',
  lg: 'w-6 h-6',
}
</script>

<template>
  <button
    type="button"
    :class="[
      sizeClasses[size || 'md'],
      'rounded-md transition-all duration-200',
      'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200',
      'hover:bg-gray-100 dark:hover:bg-white/10',
      'focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400 focus:ring-offset-2 dark:focus:ring-offset-gray-800',
      copied ? 'text-green-500 dark:text-green-400' : '',
    ]"
    :title="copied ? t('common.copied', 'Copied!') : t('common.copy', 'Copy')"
    @click="copy"
  >
    <!-- Copy icon -->
    <svg
      v-if="!copied"
      xmlns="http://www.w3.org/2000/svg"
      :class="iconSizes[size || 'md']"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      stroke-width="2"
    >
      <path
        stroke-linecap="round"
        stroke-linejoin="round"
        d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"
      />
    </svg>
    <!-- Check icon (copied state) -->
    <svg
      v-else
      xmlns="http://www.w3.org/2000/svg"
      :class="iconSizes[size || 'md']"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      stroke-width="2"
    >
      <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
    </svg>
  </button>
</template>
