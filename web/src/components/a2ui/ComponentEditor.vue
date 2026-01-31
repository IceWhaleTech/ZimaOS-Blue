<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Component, ComponentType, Action, Validation } from '@/api/a2ui'
import { getComponentLabel } from '@/api/a2ui'

const { t } = useI18n()

const props = defineProps<{
  component: Component
}>()

const emit = defineEmits<{
  update: [component: Component]
}>()

// Local copy for editing
const localComponent = ref<Component>({ ...props.component })

// UI state for collapsible sections
const expandedSections = ref<Record<string, boolean>>({
  properties: true,
  styles: false,
  actions: false,
  validation: false,
  children: false,
})

// Watch for external changes
watch(
  () => props.component,
  (newComponent) => {
    localComponent.value = JSON.parse(JSON.stringify(newComponent))
  },
  { deep: true }
)

// Emit changes
function updateProps(key: string, value: unknown): void {
  localComponent.value.props = {
    ...localComponent.value.props,
    [key]: value,
  }
  emit('update', localComponent.value)
}

function updateId(newId: string): void {
  localComponent.value.id = newId
  emit('update', localComponent.value)
}

function updateStyle(key: string, value: string): void {
  localComponent.value.style = {
    ...localComponent.value.style,
    [key]: value,
  }
  emit('update', localComponent.value)
}

// Action management
function addAction(): void {
  const newAction: Action = {
    id: `action-${Date.now()}`,
    type: 'click',
    handler: '',
    params: {},
  }
  localComponent.value.actions = [...(localComponent.value.actions || []), newAction]
  emit('update', localComponent.value)
}

function updateAction(index: number, key: keyof Action, value: unknown): void {
  if (!localComponent.value.actions) return
  const actions = [...localComponent.value.actions]
  const existingAction = actions[index]
  if (existingAction) {
    actions[index] = { ...existingAction, [key]: value }
    localComponent.value.actions = actions
    emit('update', localComponent.value)
  }
}

function removeAction(index: number): void {
  if (!localComponent.value.actions) return
  localComponent.value.actions = localComponent.value.actions.filter((_, i) => i !== index)
  emit('update', localComponent.value)
}

// Validation management
function updateValidation(key: keyof Validation, value: unknown): void {
  localComponent.value.validation = {
    ...localComponent.value.validation,
    [key]: value,
  }
  emit('update', localComponent.value)
}

function clearValidation(): void {
  localComponent.value.validation = undefined
  emit('update', localComponent.value)
}

// Check if component supports validation
const supportsValidation = computed(() => {
  return ['input', 'select', 'checkbox', 'slider'].includes(localComponent.value.type)
})

// Check if component supports actions
const supportsActions = computed(() => {
  return ['button', 'input', 'select', 'checkbox', 'card', 'list', 'table'].includes(localComponent.value.type)
})

// Check if component supports children (nested components)
const supportsChildren = computed(() => {
  return ['container', 'card', 'grid', 'tabs', 'accordion', 'form'].includes(localComponent.value.type)
})

// Children management
const selectedChildIndex = ref<number | null>(null)

function addChild(type: ComponentType): void {
  const newChild: Component = {
    id: `${type}-${Date.now()}`,
    type,
    props: getDefaultChildProps(type),
  }
  localComponent.value.children = [...(localComponent.value.children || []), newChild]
  selectedChildIndex.value = (localComponent.value.children?.length || 1) - 1
  emit('update', localComponent.value)
}

function getDefaultChildProps(type: ComponentType): Record<string, unknown> {
  switch (type) {
    case 'text':
      return { content: 'Text content' }
    case 'button':
      return { label: 'Button' }
    case 'input':
      return { label: 'Input', placeholder: 'Enter value...' }
    case 'checkbox':
      return { label: 'Checkbox' }
    case 'alert':
      return { type: 'info', message: 'Alert message' }
    default:
      return {}
  }
}

function removeChild(index: number): void {
  if (!localComponent.value.children) return
  localComponent.value.children = localComponent.value.children.filter((_, i) => i !== index)
  if (selectedChildIndex.value === index) {
    selectedChildIndex.value = null
  } else if (selectedChildIndex.value !== null && selectedChildIndex.value > index) {
    selectedChildIndex.value--
  }
  emit('update', localComponent.value)
}

function moveChild(index: number, direction: 'up' | 'down'): void {
  if (!localComponent.value.children) return
  const newIndex = direction === 'up' ? index - 1 : index + 1
  if (newIndex < 0 || newIndex >= localComponent.value.children.length) return

  const children = [...localComponent.value.children]
  const currentItem = children[index]
  const swapItem = children[newIndex]
  if (currentItem && swapItem) {
    children[index] = swapItem
    children[newIndex] = currentItem
    localComponent.value.children = children
  }

  if (selectedChildIndex.value === index) {
    selectedChildIndex.value = newIndex
  } else if (selectedChildIndex.value === newIndex) {
    selectedChildIndex.value = index
  }
  emit('update', localComponent.value)
}

function updateChild(index: number, updated: Component): void {
  if (!localComponent.value.children) return
  const children = [...localComponent.value.children]
  children[index] = updated
  localComponent.value.children = children
  emit('update', localComponent.value)
}

// Available child component types
const childComponentTypes: ComponentType[] = ['text', 'button', 'input', 'checkbox', 'alert', 'progress', 'image']

// Toggle section
function toggleSection(section: string): void {
  expandedSections.value[section] = !expandedSections.value[section]
}

// Get editable fields based on component type
const editableFields = computed(() => {
  const type = localComponent.value.type
  const fields: Array<{ key: string; label: string; type: 'text' | 'number' | 'boolean' | 'textarea' | 'select'; options?: string[] }> = []

  switch (type) {
    case 'text':
      fields.push({ key: 'content', label: t('a2ui.editor.content'), type: 'textarea' })
      fields.push({ key: 'variant', label: t('a2ui.editor.variant'), type: 'select', options: ['body', 'heading', 'caption', 'code'] })
      break
    case 'button':
      fields.push({ key: 'label', label: t('a2ui.editor.label'), type: 'text' })
      fields.push({ key: 'variant', label: t('a2ui.editor.variant'), type: 'select', options: ['primary', 'secondary', 'danger', 'ghost'] })
      fields.push({ key: 'disabled', label: t('a2ui.editor.disabled'), type: 'boolean' })
      break
    case 'input':
      fields.push({ key: 'label', label: t('a2ui.editor.label'), type: 'text' })
      fields.push({ key: 'placeholder', label: t('a2ui.editor.placeholder'), type: 'text' })
      fields.push({ key: 'type', label: t('a2ui.editor.inputType'), type: 'select', options: ['text', 'password', 'email', 'number', 'tel', 'url'] })
      fields.push({ key: 'required', label: t('a2ui.editor.required'), type: 'boolean' })
      break
    case 'select':
      fields.push({ key: 'label', label: t('a2ui.editor.label'), type: 'text' })
      fields.push({ key: 'placeholder', label: t('a2ui.editor.placeholder'), type: 'text' })
      fields.push({ key: 'required', label: t('a2ui.editor.required'), type: 'boolean' })
      break
    case 'checkbox':
      fields.push({ key: 'label', label: t('a2ui.editor.label'), type: 'text' })
      fields.push({ key: 'checked', label: t('a2ui.editor.checked'), type: 'boolean' })
      break
    case 'slider':
      fields.push({ key: 'min', label: t('a2ui.editor.min'), type: 'number' })
      fields.push({ key: 'max', label: t('a2ui.editor.max'), type: 'number' })
      fields.push({ key: 'value', label: t('a2ui.editor.value'), type: 'number' })
      fields.push({ key: 'step', label: t('a2ui.editor.step'), type: 'number' })
      break
    case 'image':
      fields.push({ key: 'src', label: t('a2ui.editor.src'), type: 'text' })
      fields.push({ key: 'alt', label: t('a2ui.editor.alt'), type: 'text' })
      fields.push({ key: 'width', label: t('a2ui.editor.width'), type: 'text' })
      fields.push({ key: 'height', label: t('a2ui.editor.height'), type: 'text' })
      break
    case 'card':
      fields.push({ key: 'title', label: t('a2ui.editor.cardTitle'), type: 'text' })
      fields.push({ key: 'subtitle', label: t('a2ui.editor.cardSubtitle'), type: 'text' })
      break
    case 'progress':
      fields.push({ key: 'value', label: t('a2ui.editor.value'), type: 'number' })
      fields.push({ key: 'max', label: t('a2ui.editor.max'), type: 'number' })
      fields.push({ key: 'showLabel', label: t('a2ui.editor.showLabel'), type: 'boolean' })
      break
    case 'alert':
      fields.push({ key: 'type', label: t('a2ui.editor.alertType'), type: 'select', options: ['info', 'success', 'warning', 'error'] })
      fields.push({ key: 'message', label: t('a2ui.editor.message'), type: 'textarea' })
      fields.push({ key: 'dismissible', label: t('a2ui.editor.dismissible'), type: 'boolean' })
      break
    case 'code':
      fields.push({ key: 'language', label: t('a2ui.editor.language'), type: 'select', options: ['javascript', 'typescript', 'python', 'go', 'rust', 'json', 'yaml', 'bash', 'sql'] })
      fields.push({ key: 'code', label: t('a2ui.editor.code'), type: 'textarea' })
      break
    case 'markdown':
      fields.push({ key: 'content', label: t('a2ui.editor.content'), type: 'textarea' })
      break
  }

  return fields
})

// Style fields
const styleFields = [
  { key: 'width', label: 'Width' },
  { key: 'height', label: 'Height' },
  { key: 'padding', label: 'Padding' },
  { key: 'margin', label: 'Margin' },
  { key: 'backgroundColor', label: 'Background' },
  { key: 'color', label: 'Text Color' },
  { key: 'borderRadius', label: 'Border Radius' },
]
</script>

<template>
  <div class="h-full flex flex-col">
    <div class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">
      {{ t('a2ui.editor.title') }}
    </div>

    <div class="flex-1 overflow-y-auto space-y-2">
      <!-- Component ID -->
      <div>
        <label class="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">
          {{ t('a2ui.editor.componentId') }}
        </label>
        <input
          :value="localComponent.id"
          type="text"
          class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 border border-gray-300 dark:border-gray-600"
          @input="updateId(($event.target as HTMLInputElement).value)"
        />
      </div>

      <!-- Component Type (read-only) -->
      <div>
        <label class="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">
          {{ t('a2ui.editor.componentType') }}
        </label>
        <div class="px-3 py-1.5 bg-gray-200 dark:bg-gray-600 text-gray-700 dark:text-gray-300 rounded text-sm">
          {{ localComponent.type }}
        </div>
      </div>

      <!-- Properties Section (Collapsible) -->
      <div class="border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden">
        <button
          type="button"
          class="w-full flex items-center justify-between px-3 py-2 bg-gray-50 dark:bg-gray-700/50 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
          @click="toggleSection('properties')"
        >
          <span class="text-xs font-medium text-gray-600 dark:text-gray-300">
            {{ t('a2ui.editor.properties') }}
          </span>
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-4 w-4 text-gray-400 transition-transform"
            :class="{ 'rotate-180': expandedSections.properties }"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
          </svg>
        </button>
        <div v-show="expandedSections.properties" class="p-3 space-y-3">
          <div v-for="field in editableFields" :key="field.key" class="space-y-1">
            <label class="block text-xs font-medium text-gray-500 dark:text-gray-400">
              {{ field.label }}
            </label>

            <!-- Text Input -->
            <input
              v-if="field.type === 'text'"
              :value="localComponent.props?.[field.key] ?? ''"
              type="text"
              class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 border border-gray-300 dark:border-gray-600"
              @input="updateProps(field.key, ($event.target as HTMLInputElement).value)"
            />

            <!-- Number Input -->
            <input
              v-else-if="field.type === 'number'"
              :value="localComponent.props?.[field.key] ?? 0"
              type="number"
              class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 border border-gray-300 dark:border-gray-600"
              @input="updateProps(field.key, Number(($event.target as HTMLInputElement).value))"
            />

            <!-- Textarea -->
            <textarea
              v-else-if="field.type === 'textarea'"
              :value="String(localComponent.props?.[field.key] ?? '')"
              rows="3"
              class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 border border-gray-300 dark:border-gray-600 resize-none"
              @input="updateProps(field.key, ($event.target as HTMLTextAreaElement).value)"
            />

            <!-- Select -->
            <select
              v-else-if="field.type === 'select'"
              :value="localComponent.props?.[field.key] ?? ''"
              class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 border border-gray-300 dark:border-gray-600"
              @change="updateProps(field.key, ($event.target as HTMLSelectElement).value)"
            >
              <option v-for="opt in field.options" :key="opt" :value="opt">{{ opt }}</option>
            </select>

            <!-- Boolean (Checkbox) -->
            <label v-else-if="field.type === 'boolean'" class="flex items-center gap-2 cursor-pointer">
              <input
                type="checkbox"
                :checked="!!localComponent.props?.[field.key]"
                class="w-4 h-4 rounded border-gray-300 dark:border-gray-600 text-blue-600 focus:ring-blue-500"
                @change="updateProps(field.key, ($event.target as HTMLInputElement).checked)"
              />
              <span class="text-sm text-gray-700 dark:text-gray-300">{{ t('common.enabled') }}</span>
            </label>
          </div>
        </div>
      </div>

      <!-- Styles Section (Collapsible) -->
      <div class="border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden">
        <button
          type="button"
          class="w-full flex items-center justify-between px-3 py-2 bg-gray-50 dark:bg-gray-700/50 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
          @click="toggleSection('styles')"
        >
          <span class="text-xs font-medium text-gray-600 dark:text-gray-300">
            {{ t('a2ui.editor.styles') }}
          </span>
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-4 w-4 text-gray-400 transition-transform"
            :class="{ 'rotate-180': expandedSections.styles }"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
          </svg>
        </button>
        <div v-show="expandedSections.styles" class="p-3 space-y-3">
          <div v-for="field in styleFields" :key="field.key" class="space-y-1">
            <label class="block text-xs font-medium text-gray-500 dark:text-gray-400">
              {{ field.label }}
            </label>
            <input
              :value="localComponent.style?.[field.key] ?? ''"
              type="text"
              :placeholder="field.key === 'width' ? '100%' : field.key === 'padding' ? '1rem' : ''"
              class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 border border-gray-300 dark:border-gray-600"
              @input="updateStyle(field.key, ($event.target as HTMLInputElement).value)"
            />
          </div>
        </div>
      </div>

      <!-- Actions Section (Collapsible) -->
      <div v-if="supportsActions" class="border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden">
        <button
          type="button"
          class="w-full flex items-center justify-between px-3 py-2 bg-gray-50 dark:bg-gray-700/50 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
          @click="toggleSection('actions')"
        >
          <span class="text-xs font-medium text-gray-600 dark:text-gray-300">
            {{ t('a2ui.editor.actions') }}
            <span v-if="localComponent.actions?.length" class="ml-1 text-blue-500">({{ localComponent.actions.length }})</span>
          </span>
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-4 w-4 text-gray-400 transition-transform"
            :class="{ 'rotate-180': expandedSections.actions }"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
          </svg>
        </button>
        <div v-show="expandedSections.actions" class="p-3 space-y-3">
          <!-- Action List -->
          <div v-for="(action, index) in localComponent.actions" :key="action.id" class="p-2 bg-gray-50 dark:bg-gray-700/30 rounded-lg space-y-2">
            <div class="flex items-center justify-between">
              <span class="text-xs font-medium text-gray-600 dark:text-gray-400">{{ t('a2ui.editor.action') }} #{{ index + 1 }}</span>
              <button
                type="button"
                class="p-1 rounded hover:bg-red-100 dark:hover:bg-red-900/30 text-red-500"
                @click="removeAction(index)"
              >
                <svg xmlns="http://www.w3.org/2000/svg" class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>
            <!-- Action ID -->
            <div class="space-y-1">
              <label class="block text-xs text-gray-500 dark:text-gray-400">{{ t('a2ui.editor.actionId') }}</label>
              <input
                :value="action.id"
                type="text"
                class="w-full bg-white dark:bg-gray-700 text-gray-900 dark:text-white rounded px-2 py-1 text-xs focus:outline-none focus:ring-1 focus:ring-blue-500 border border-gray-300 dark:border-gray-600"
                @input="updateAction(index, 'id', ($event.target as HTMLInputElement).value)"
              />
            </div>
            <!-- Action Type -->
            <div class="space-y-1">
              <label class="block text-xs text-gray-500 dark:text-gray-400">{{ t('a2ui.editor.actionType') }}</label>
              <select
                :value="action.type"
                class="w-full bg-white dark:bg-gray-700 text-gray-900 dark:text-white rounded px-2 py-1 text-xs focus:outline-none focus:ring-1 focus:ring-blue-500 border border-gray-300 dark:border-gray-600"
                @change="updateAction(index, 'type', ($event.target as HTMLSelectElement).value)"
              >
                <option value="click">click</option>
                <option value="submit">submit</option>
                <option value="change">change</option>
                <option value="custom">custom</option>
              </select>
            </div>
            <!-- Handler -->
            <div class="space-y-1">
              <label class="block text-xs text-gray-500 dark:text-gray-400">{{ t('a2ui.editor.handler') }}</label>
              <input
                :value="action.handler"
                type="text"
                :placeholder="t('a2ui.editor.handlerPlaceholder')"
                class="w-full bg-white dark:bg-gray-700 text-gray-900 dark:text-white rounded px-2 py-1 text-xs focus:outline-none focus:ring-1 focus:ring-blue-500 border border-gray-300 dark:border-gray-600"
                @input="updateAction(index, 'handler', ($event.target as HTMLInputElement).value)"
              />
            </div>
            <!-- Label (optional) -->
            <div class="space-y-1">
              <label class="block text-xs text-gray-500 dark:text-gray-400">{{ t('a2ui.editor.actionLabel') }}</label>
              <input
                :value="action.label || ''"
                type="text"
                class="w-full bg-white dark:bg-gray-700 text-gray-900 dark:text-white rounded px-2 py-1 text-xs focus:outline-none focus:ring-1 focus:ring-blue-500 border border-gray-300 dark:border-gray-600"
                @input="updateAction(index, 'label', ($event.target as HTMLInputElement).value)"
              />
            </div>
          </div>
          <!-- Add Action Button -->
          <button
            type="button"
            class="w-full py-1.5 text-xs text-blue-600 dark:text-blue-400 hover:bg-blue-50 dark:hover:bg-blue-900/20 rounded border border-dashed border-blue-300 dark:border-blue-600 transition-colors"
            @click="addAction"
          >
            + {{ t('a2ui.editor.addAction') }}
          </button>
        </div>
      </div>

      <!-- Validation Section (Collapsible) -->
      <div v-if="supportsValidation" class="border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden">
        <button
          type="button"
          class="w-full flex items-center justify-between px-3 py-2 bg-gray-50 dark:bg-gray-700/50 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
          @click="toggleSection('validation')"
        >
          <span class="text-xs font-medium text-gray-600 dark:text-gray-300">
            {{ t('a2ui.editor.validation') }}
            <span v-if="localComponent.validation" class="ml-1 text-green-500">*</span>
          </span>
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-4 w-4 text-gray-400 transition-transform"
            :class="{ 'rotate-180': expandedSections.validation }"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
          </svg>
        </button>
        <div v-show="expandedSections.validation" class="p-3 space-y-3">
          <!-- Required -->
          <label class="flex items-center gap-2 cursor-pointer">
            <input
              type="checkbox"
              :checked="!!localComponent.validation?.required"
              class="w-4 h-4 rounded border-gray-300 dark:border-gray-600 text-blue-600 focus:ring-blue-500"
              @change="updateValidation('required', ($event.target as HTMLInputElement).checked)"
            />
            <span class="text-xs text-gray-700 dark:text-gray-300">{{ t('a2ui.editor.required') }}</span>
          </label>
          <!-- Min Length -->
          <div class="space-y-1">
            <label class="block text-xs text-gray-500 dark:text-gray-400">{{ t('a2ui.editor.minLength') }}</label>
            <input
              :value="localComponent.validation?.min_length ?? ''"
              type="number"
              min="0"
              class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded px-2 py-1 text-xs focus:outline-none focus:ring-1 focus:ring-blue-500 border border-gray-300 dark:border-gray-600"
              @input="updateValidation('min_length', Number(($event.target as HTMLInputElement).value) || undefined)"
            />
          </div>
          <!-- Max Length -->
          <div class="space-y-1">
            <label class="block text-xs text-gray-500 dark:text-gray-400">{{ t('a2ui.editor.maxLength') }}</label>
            <input
              :value="localComponent.validation?.max_length ?? ''"
              type="number"
              min="0"
              class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded px-2 py-1 text-xs focus:outline-none focus:ring-1 focus:ring-blue-500 border border-gray-300 dark:border-gray-600"
              @input="updateValidation('max_length', Number(($event.target as HTMLInputElement).value) || undefined)"
            />
          </div>
          <!-- Min Value (for number inputs) -->
          <div class="space-y-1">
            <label class="block text-xs text-gray-500 dark:text-gray-400">{{ t('a2ui.editor.minValue') }}</label>
            <input
              :value="localComponent.validation?.min ?? ''"
              type="number"
              class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded px-2 py-1 text-xs focus:outline-none focus:ring-1 focus:ring-blue-500 border border-gray-300 dark:border-gray-600"
              @input="updateValidation('min', Number(($event.target as HTMLInputElement).value) || undefined)"
            />
          </div>
          <!-- Max Value (for number inputs) -->
          <div class="space-y-1">
            <label class="block text-xs text-gray-500 dark:text-gray-400">{{ t('a2ui.editor.maxValue') }}</label>
            <input
              :value="localComponent.validation?.max ?? ''"
              type="number"
              class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded px-2 py-1 text-xs focus:outline-none focus:ring-1 focus:ring-blue-500 border border-gray-300 dark:border-gray-600"
              @input="updateValidation('max', Number(($event.target as HTMLInputElement).value) || undefined)"
            />
          </div>
          <!-- Pattern (regex) -->
          <div class="space-y-1">
            <label class="block text-xs text-gray-500 dark:text-gray-400">{{ t('a2ui.editor.pattern') }}</label>
            <input
              :value="localComponent.validation?.pattern ?? ''"
              type="text"
              :placeholder="t('a2ui.editor.patternPlaceholder')"
              class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded px-2 py-1 text-xs focus:outline-none focus:ring-1 focus:ring-blue-500 border border-gray-300 dark:border-gray-600 font-mono"
              @input="updateValidation('pattern', ($event.target as HTMLInputElement).value || undefined)"
            />
          </div>
          <!-- Error Message -->
          <div class="space-y-1">
            <label class="block text-xs text-gray-500 dark:text-gray-400">{{ t('a2ui.editor.errorMessage') }}</label>
            <input
              :value="localComponent.validation?.message ?? ''"
              type="text"
              :placeholder="t('a2ui.editor.errorMessagePlaceholder')"
              class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded px-2 py-1 text-xs focus:outline-none focus:ring-1 focus:ring-blue-500 border border-gray-300 dark:border-gray-600"
              @input="updateValidation('message', ($event.target as HTMLInputElement).value || undefined)"
            />
          </div>
          <!-- Clear Validation -->
          <button
            v-if="localComponent.validation"
            type="button"
            class="w-full py-1.5 text-xs text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 rounded border border-dashed border-red-300 dark:border-red-600 transition-colors"
            @click="clearValidation"
          >
            {{ t('a2ui.editor.clearValidation') }}
          </button>
        </div>
      </div>

      <!-- Children Section (Collapsible) - for container components -->
      <div v-if="supportsChildren" class="border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden">
        <button
          type="button"
          class="w-full flex items-center justify-between px-3 py-2 bg-gray-50 dark:bg-gray-700/50 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
          @click="toggleSection('children')"
        >
          <span class="text-xs font-medium text-gray-600 dark:text-gray-300">
            {{ t('a2ui.editor.children') }}
            <span v-if="localComponent.children?.length" class="ml-1 text-purple-500">({{ localComponent.children.length }})</span>
          </span>
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-4 w-4 text-gray-400 transition-transform"
            :class="{ 'rotate-180': expandedSections.children }"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
          </svg>
        </button>
        <div v-show="expandedSections.children" class="p-3 space-y-3">
          <!-- Add Child Component Buttons -->
          <div class="flex flex-wrap gap-1">
            <button
              v-for="childType in childComponentTypes"
              :key="childType"
              type="button"
              class="px-2 py-1 text-xs bg-gray-100 dark:bg-gray-700 hover:bg-purple-100 dark:hover:bg-purple-900/30 text-gray-600 dark:text-gray-400 hover:text-purple-600 dark:hover:text-purple-400 rounded transition-colors"
              @click="addChild(childType)"
            >
              + {{ getComponentLabel(childType) }}
            </button>
          </div>

          <!-- Children List -->
          <div v-if="localComponent.children?.length" class="space-y-2">
            <div
              v-for="(child, index) in localComponent.children"
              :key="child.id"
              class="p-2 bg-gray-50 dark:bg-gray-700/30 rounded-lg"
            >
              <div class="flex items-center justify-between mb-2">
                <div class="flex items-center gap-2">
                  <span class="text-xs font-medium text-gray-600 dark:text-gray-400">
                    {{ getComponentLabel(child.type) }}
                  </span>
                  <span class="text-xs text-gray-400 dark:text-gray-500">
                    {{ child.id }}
                  </span>
                </div>
                <div class="flex items-center gap-1">
                  <button
                    type="button"
                    class="p-1 rounded hover:bg-gray-200 dark:hover:bg-gray-600 disabled:opacity-30"
                    :disabled="index === 0"
                    @click="moveChild(index, 'up')"
                  >
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 15l7-7 7 7" />
                    </svg>
                  </button>
                  <button
                    type="button"
                    class="p-1 rounded hover:bg-gray-200 dark:hover:bg-gray-600 disabled:opacity-30"
                    :disabled="index === (localComponent.children?.length || 0) - 1"
                    @click="moveChild(index, 'down')"
                  >
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                    </svg>
                  </button>
                  <button
                    type="button"
                    class="p-1 rounded hover:bg-red-100 dark:hover:bg-red-900/30 text-red-500"
                    @click="removeChild(index)"
                  >
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                    </svg>
                  </button>
                </div>
              </div>
              <!-- Inline child editing for simple props -->
              <div v-if="child.type === 'text'" class="space-y-1">
                <input
                  :value="(child.props?.content as string) || ''"
                  type="text"
                  :placeholder="t('a2ui.editor.content')"
                  class="w-full bg-white dark:bg-gray-700 text-gray-900 dark:text-white rounded px-2 py-1 text-xs focus:outline-none focus:ring-1 focus:ring-blue-500 border border-gray-300 dark:border-gray-600"
                  @input="updateChild(index, { ...child, props: { ...child.props, content: ($event.target as HTMLInputElement).value } })"
                />
              </div>
              <div v-else-if="child.type === 'button'" class="space-y-1">
                <input
                  :value="(child.props?.label as string) || ''"
                  type="text"
                  :placeholder="t('a2ui.editor.label')"
                  class="w-full bg-white dark:bg-gray-700 text-gray-900 dark:text-white rounded px-2 py-1 text-xs focus:outline-none focus:ring-1 focus:ring-blue-500 border border-gray-300 dark:border-gray-600"
                  @input="updateChild(index, { ...child, props: { ...child.props, label: ($event.target as HTMLInputElement).value } })"
                />
              </div>
              <div v-else-if="child.type === 'input'" class="space-y-1">
                <input
                  :value="(child.props?.label as string) || ''"
                  type="text"
                  :placeholder="t('a2ui.editor.label')"
                  class="w-full bg-white dark:bg-gray-700 text-gray-900 dark:text-white rounded px-2 py-1 text-xs focus:outline-none focus:ring-1 focus:ring-blue-500 border border-gray-300 dark:border-gray-600"
                  @input="updateChild(index, { ...child, props: { ...child.props, label: ($event.target as HTMLInputElement).value } })"
                />
                <input
                  :value="(child.props?.placeholder as string) || ''"
                  type="text"
                  :placeholder="t('a2ui.editor.placeholder')"
                  class="w-full bg-white dark:bg-gray-700 text-gray-900 dark:text-white rounded px-2 py-1 text-xs focus:outline-none focus:ring-1 focus:ring-blue-500 border border-gray-300 dark:border-gray-600"
                  @input="updateChild(index, { ...child, props: { ...child.props, placeholder: ($event.target as HTMLInputElement).value } })"
                />
              </div>
              <div v-else-if="child.type === 'checkbox'" class="space-y-1">
                <input
                  :value="(child.props?.label as string) || ''"
                  type="text"
                  :placeholder="t('a2ui.editor.label')"
                  class="w-full bg-white dark:bg-gray-700 text-gray-900 dark:text-white rounded px-2 py-1 text-xs focus:outline-none focus:ring-1 focus:ring-blue-500 border border-gray-300 dark:border-gray-600"
                  @input="updateChild(index, { ...child, props: { ...child.props, label: ($event.target as HTMLInputElement).value } })"
                />
              </div>
              <div v-else-if="child.type === 'alert'" class="space-y-1">
                <select
                  :value="(child.props?.type as string) || 'info'"
                  class="w-full bg-white dark:bg-gray-700 text-gray-900 dark:text-white rounded px-2 py-1 text-xs focus:outline-none focus:ring-1 focus:ring-blue-500 border border-gray-300 dark:border-gray-600"
                  @change="updateChild(index, { ...child, props: { ...child.props, type: ($event.target as HTMLSelectElement).value } })"
                >
                  <option value="info">info</option>
                  <option value="success">success</option>
                  <option value="warning">warning</option>
                  <option value="error">error</option>
                </select>
                <input
                  :value="(child.props?.message as string) || ''"
                  type="text"
                  :placeholder="t('a2ui.editor.message')"
                  class="w-full bg-white dark:bg-gray-700 text-gray-900 dark:text-white rounded px-2 py-1 text-xs focus:outline-none focus:ring-1 focus:ring-blue-500 border border-gray-300 dark:border-gray-600"
                  @input="updateChild(index, { ...child, props: { ...child.props, message: ($event.target as HTMLInputElement).value } })"
                />
              </div>
              <div v-else class="text-xs text-gray-400 dark:text-gray-500 italic">
                {{ t('a2ui.editor.noInlineEdit') }}
              </div>
            </div>
          </div>
          <div v-else class="text-xs text-gray-400 dark:text-gray-500 text-center py-2">
            {{ t('a2ui.editor.noChildren') }}
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
