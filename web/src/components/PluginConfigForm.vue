<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Plugin, PluginConfigProperty } from '@/api/plugin'

const { t } = useI18n()

const props = defineProps<{
  plugin: Plugin
  loading?: boolean
}>()

const emit = defineEmits<{
  (e: 'save', config: Record<string, unknown>): void
  (e: 'cancel'): void
}>()

const formData = ref<Record<string, unknown>>({})

// Initialize form data from plugin config
watch(
  () => props.plugin,
  (plugin) => {
    if (plugin.config) {
      formData.value = { ...plugin.config }
    } else if (plugin.config_schema) {
      // Initialize with defaults
      const defaults: Record<string, unknown> = {}
      Object.entries(plugin.config_schema.properties).forEach(([key, prop]) => {
        if (prop.default !== undefined) {
          defaults[key] = prop.default
        }
      })
      formData.value = defaults
    }
  },
  { immediate: true }
)

const schema = computed(() => props.plugin.config_schema)

function getFieldType(prop: PluginConfigProperty): string {
  if (prop.enum) return 'select'
  switch (prop.type) {
    case 'boolean':
      return 'checkbox'
    case 'number':
      return 'number'
    case 'array':
      return 'array'
    case 'object':
      return 'object'
    default:
      return 'text'
  }
}

function isRequired(key: string): boolean {
  return schema.value?.required?.includes(key) ?? false
}

function updateField(key: string, value: unknown) {
  formData.value = { ...formData.value, [key]: value }
}

function updateArrayField(key: string, index: number, value: string) {
  const arr = [...((formData.value[key] as string[]) || [])]
  arr[index] = value
  formData.value = { ...formData.value, [key]: arr }
}

function addArrayItem(key: string) {
  const arr = [...((formData.value[key] as string[]) || []), '']
  formData.value = { ...formData.value, [key]: arr }
}

function removeArrayItem(key: string, index: number) {
  const arr = [...((formData.value[key] as string[]) || [])]
  arr.splice(index, 1)
  formData.value = { ...formData.value, [key]: arr }
}

function updateJsonField(key: string, value: string) {
  try {
    updateField(key, JSON.parse(value))
  } catch {
    // Invalid JSON, ignore
  }
}

function handleSubmit() {
  emit('save', formData.value)
}
</script>

<template>
  <div class="plugin-config-form">
    <form class="space-y-4" @submit.prevent="handleSubmit">
      <template v-if="schema">
        <div v-for="(prop, key) in schema.properties" :key="key" class="space-y-2">
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ prop.title || key }}
            <span v-if="isRequired(key)" class="text-red-500 dark:text-red-400">*</span>
          </label>

          <p v-if="prop.description" class="text-xs text-gray-500">
            {{ prop.description }}
          </p>

          <!-- Text Input -->
          <input
            v-if="getFieldType(prop) === 'text'"
            :value="formData[key] as string"
            type="text"
            :required="isRequired(key)"
            class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400 border border-gray-300 dark:border-gray-600"
            :placeholder="(prop.default as string) || ''"
            @input="updateField(key, ($event.target as HTMLInputElement).value)"
          />

          <!-- Number Input -->
          <input
            v-else-if="getFieldType(prop) === 'number'"
            :value="formData[key] as number"
            type="number"
            :required="isRequired(key)"
            :min="prop.minimum"
            :max="prop.maximum"
            class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400 border border-gray-300 dark:border-gray-600"
            :placeholder="String(prop.default || '')"
            @input="updateField(key, parseFloat(($event.target as HTMLInputElement).value))"
          />

          <!-- Checkbox -->
          <label
            v-else-if="getFieldType(prop) === 'checkbox'"
            class="flex items-center gap-2 cursor-pointer"
          >
            <input
              :checked="formData[key] as boolean"
              type="checkbox"
              class="w-4 h-4 rounded border-gray-300 dark:border-gray-600 bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-gray-900 dark:focus:ring-gray-400"
              @change="updateField(key, ($event.target as HTMLInputElement).checked)"
            />
            <span class="text-gray-700 dark:text-gray-300">{{ prop.title || key }}</span>
          </label>

          <!-- Select -->
          <select
            v-else-if="getFieldType(prop) === 'select'"
            :value="formData[key] as string"
            :required="isRequired(key)"
            class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400 border border-gray-300 dark:border-gray-600"
            @change="updateField(key, ($event.target as HTMLSelectElement).value)"
          >
            <option value="" disabled>{{ t('common.selectOption', 'Select an option') }}</option>
            <option v-for="opt in prop.enum" :key="String(opt)" :value="opt">
              {{ opt }}
            </option>
          </select>

          <!-- Array Input -->
          <div v-else-if="getFieldType(prop) === 'array'" class="space-y-2">
            <div
              v-for="(item, index) in (formData[key] as string[]) || []"
              :key="index"
              class="flex items-center gap-2"
            >
              <input
                :value="item"
                type="text"
                class="flex-1 bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400 border border-gray-300 dark:border-gray-600"
                @input="updateArrayField(key, index, ($event.target as HTMLInputElement).value)"
              />
              <button
                type="button"
                class="p-2 text-red-500 dark:text-red-400 hover:text-red-600 dark:hover:text-red-300 hover:bg-red-100 dark:hover:bg-red-900/20 rounded-lg"
                @click="removeArrayItem(key, index)"
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
                    d="M6 18L18 6M6 6l12 12"
                  />
                </svg>
              </button>
            </div>
            <button
              type="button"
              class="text-sm text-gray-900 dark:text-white dark:text-white hover:text-gray-900 dark:text-white dark:hover:text-gray-900 dark:text-white flex items-center gap-1"
              @click="addArrayItem(key)"
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
                  d="M12 4v16m8-8H4"
                />
              </svg>
              {{ t('common.addItem', 'Add item') }}
            </button>
          </div>

          <!-- Object (JSON) Input -->
          <textarea
            v-else-if="getFieldType(prop) === 'object'"
            :value="JSON.stringify(formData[key] || {}, null, 2)"
            rows="4"
            class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400 font-mono text-sm border border-gray-300 dark:border-gray-600"
            @input="updateJsonField(key, ($event.target as HTMLTextAreaElement).value)"
          ></textarea>
        </div>
      </template>

      <div v-else class="text-gray-500 dark:text-gray-400 text-center py-4">
        {{ t('plugins.configForm.empty', 'This plugin has no configurable options.') }}
      </div>

      <!-- Actions -->
      <div class="flex gap-3 pt-4 border-t border-gray-200 dark:border-gray-700">
        <button
          type="submit"
          :disabled="loading"
          class="flex-1 px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg transition-colors disabled:opacity-50"
        >
          {{
            loading
              ? t('common.saving', 'Saving...')
              : t('plugins.configForm.save', 'Save Configuration')
          }}
        </button>
        <button
          type="button"
          class="px-4 py-2 bg-gray-200 dark:bg-gray-600 hover:bg-gray-300 dark:hover:bg-gray-500 text-gray-700 dark:text-white rounded-lg transition-colors"
          @click="emit('cancel')"
        >
          {{ t('common.cancel', 'Cancel') }}
        </button>
      </div>
    </form>
  </div>
</template>
