<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { usePreviewStore } from '@/stores/preview'
import { storeToRefs } from 'pinia'
import { resetPreviewModeStatus } from '@/router'
import PreviewUpgradeForm from './PreviewUpgradeForm.vue'

const { t } = useI18n()
const previewStore = usePreviewStore()
const { isPreviewMode } = storeToRefs(previewStore)

// Dropdown state
const showDropdown = ref(false)
const showUpgradeModal = ref(false)
const dropdownRef = ref<HTMLElement | null>(null)

// Close dropdown when clicking outside
function handleClickOutside(event: MouseEvent) {
  if (dropdownRef.value && !dropdownRef.value.contains(event.target as Node)) {
    showDropdown.value = false
  }
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
})

function openUpgradeModal() {
  showDropdown.value = false
  showUpgradeModal.value = true
}

function handleUpgradeSuccess() {
  showUpgradeModal.value = false
  // Reset preview mode cache in router
  resetPreviewModeStatus()
  // Reload the page to apply new auth state
  window.location.reload()
}
</script>

<template>
  <div v-if="isPreviewMode" ref="dropdownRef" class="relative" data-preview-banner>
    <!-- Preview mode button -->
    <button
      class="flex items-center space-x-2 px-3 py-1.5 rounded-lg text-sm font-medium transition-colors"
      :class="
        showDropdown
          ? 'bg-amber-500/20 text-amber-600 dark:text-amber-400'
          : 'bg-amber-500/10 text-amber-600 dark:text-amber-400 hover:bg-amber-500/20'
      "
      @click="showDropdown = !showDropdown"
    >
      <svg
        xmlns="http://www.w3.org/2000/svg"
        class="h-4 w-4"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
        />
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"
        />
      </svg>
      <span class="hidden sm:inline">{{ t('preview.createAccount') }}</span>
      <svg
        xmlns="http://www.w3.org/2000/svg"
        class="h-4 w-4"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
      >
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
      </svg>
    </button>

    <!-- Dropdown menu -->
    <div
      v-if="showDropdown"
      class="absolute right-0 mt-2 w-56 rounded-lg bg-white dark:bg-gray-700 shadow-lg border border-gray-200 dark:border-gray-700 py-1 z-50"
    >
      <button
        class="w-full flex items-center space-x-3 px-4 py-3 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700"
        @click="openUpgradeModal"
      >
        <span class="text-lg">🎉</span>
        <span class="font-medium">{{ t('preview.createAdminAccount') }}</span>
      </button>
      <div class="border-t border-gray-200 dark:border-gray-700 my-1"></div>
      <div class="px-4 py-2 text-xs text-gray-500 dark:text-gray-400">
        {{ t('preview.hint') }}
      </div>
    </div>

    <!-- Upgrade Modal -->
    <PreviewUpgradeForm
      v-if="showUpgradeModal"
      @close="showUpgradeModal = false"
      @success="handleUpgradeSuccess"
    />
  </div>
</template>
