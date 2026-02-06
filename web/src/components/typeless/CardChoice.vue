<script setup lang="ts">
import { ref } from 'vue'
import type { TypelessCardChoice, ChoiceOption } from '@/types/typeless'

const props = defineProps<{
  card: TypelessCardChoice
}>()

const emit = defineEmits<{
  select: [_selectedIds: string[], _otherText?: string]
}>()

const selectedIds = ref<Set<string>>(new Set(props.card.selectedIds || []))
const otherText = ref('')
const otherSelected = ref(false)

function isSelected(optionId: string): boolean {
  return selectedIds.value.has(optionId)
}

function toggleOption(option: ChoiceOption) {
  if (option.disabled) return

  if (props.card.multiple) {
    // Multi-select mode
    if (selectedIds.value.has(option.id)) {
      selectedIds.value.delete(option.id)
    } else {
      selectedIds.value.add(option.id)
    }
  } else {
    // Single-select mode
    selectedIds.value.clear()
    selectedIds.value.add(option.id)
    otherSelected.value = false
  }
  selectedIds.value = new Set(selectedIds.value)
  emitSelection()
}

function toggleOther() {
  if (props.card.multiple) {
    otherSelected.value = !otherSelected.value
  } else {
    selectedIds.value.clear()
    otherSelected.value = true
  }
  emitSelection()
}

function handleOtherInput() {
  emitSelection()
}

function emitSelection() {
  const ids = Array.from(selectedIds.value)
  emit('select', ids, otherSelected.value ? otherText.value : undefined)
}
</script>

<template>
  <div class="choice-card rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden bg-white dark:bg-gray-700">
    <!-- Header -->
    <div v-if="card.title || card.description" class="px-4 py-3 border-b border-gray-200 dark:border-gray-700">
      <h4 v-if="card.title" class="font-medium text-gray-900 dark:text-white">
        {{ card.title }}
        <span v-if="card.required" class="text-red-500 ml-1">*</span>
      </h4>
      <p v-if="card.description" class="mt-1 text-sm text-gray-500 dark:text-gray-400">
        {{ card.description }}
      </p>
      <p v-if="card.multiple" class="mt-1 text-xs text-gray-400 dark:text-gray-500">
        (Select multiple options)
      </p>
    </div>

    <!-- Options -->
    <div class="p-4 space-y-2">
      <button
        v-for="option in card.options"
        :key="option.id"
        class="w-full p-3 rounded-lg border-2 text-left transition-all flex items-start gap-3"
        :class="{
          'border-gray-900 dark:border-white bg-gray-700 dark:bg-gray-700 dark:bg-gray-700 dark:bg-gray-700/20': isSelected(option.id),
          'border-gray-200 dark:border-gray-700 hover:border-gray-300 dark:hover:border-gray-600': !isSelected(option.id) && !option.disabled,
          'border-gray-100 dark:border-gray-800 opacity-50 cursor-not-allowed': option.disabled,
        }"
        :disabled="option.disabled"
        @click="toggleOption(option)"
      >
        <!-- Checkbox/Radio indicator -->
        <div
          class="flex-shrink-0 w-5 h-5 mt-0.5 rounded flex items-center justify-center border-2 transition-colors"
          :class="{
            'rounded-full': !card.multiple,
            'border-gray-900 dark:border-white bg-gray-700 dark:bg-gray-700': isSelected(option.id),
            'border-gray-300 dark:border-gray-600': !isSelected(option.id),
          }"
        >
          <svg
            v-if="isSelected(option.id)"
            xmlns="http://www.w3.org/2000/svg"
            class="h-3 w-3 text-white"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="3" d="M5 13l4 4L19 7" />
          </svg>
        </div>

        <!-- Icon -->
        <span v-if="option.icon" class="flex-shrink-0 text-xl">{{ option.icon }}</span>

        <!-- Content -->
        <div class="flex-1 min-w-0">
          <p class="font-medium text-gray-900 dark:text-white">{{ option.label }}</p>
          <p v-if="option.description" class="mt-0.5 text-sm text-gray-500 dark:text-gray-400">
            {{ option.description }}
          </p>
        </div>
      </button>

      <!-- Other option -->
      <div v-if="card.allowOther" class="space-y-2">
        <button
          class="w-full p-3 rounded-lg border-2 text-left transition-all flex items-start gap-3"
          :class="{
            'border-gray-900 dark:border-white bg-gray-700 dark:bg-gray-700 dark:bg-gray-700 dark:bg-gray-700/20': otherSelected,
            'border-gray-200 dark:border-gray-700 hover:border-gray-300 dark:hover:border-gray-600': !otherSelected,
          }"
          @click="toggleOther"
        >
          <!-- Checkbox/Radio indicator -->
          <div
            class="flex-shrink-0 w-5 h-5 mt-0.5 rounded flex items-center justify-center border-2 transition-colors"
            :class="{
              'rounded-full': !card.multiple,
              'border-gray-900 dark:border-white bg-gray-700 dark:bg-gray-700': otherSelected,
              'border-gray-300 dark:border-gray-600': !otherSelected,
            }"
          >
            <svg
              v-if="otherSelected"
              xmlns="http://www.w3.org/2000/svg"
              class="h-3 w-3 text-white"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="3" d="M5 13l4 4L19 7" />
            </svg>
          </div>

          <span class="font-medium text-gray-900 dark:text-white">Other</span>
        </button>

        <!-- Other text input -->
        <div v-if="otherSelected" class="pl-8">
          <input
            v-model="otherText"
            type="text"
            :placeholder="card.otherPlaceholder || 'Please specify...'"
            class="w-full px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400 focus:border-transparent"
            @input="handleOtherInput"
          />
        </div>
      </div>
    </div>
  </div>
</template>
