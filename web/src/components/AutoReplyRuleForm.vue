<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AutoReplyRule, TriggerType, CreateRuleRequest } from '@/api/autoreply'

const { t } = useI18n()

const props = defineProps<{
  rule?: AutoReplyRule | null
  loading?: boolean
}>()

const emit = defineEmits<{
  save: [data: CreateRuleRequest]
  cancel: []
}>()

// Form state
const name = ref('')
const triggerType = ref<TriggerType>('keyword')
const triggerValue = ref('')
const responses = ref<string[]>([''])
const priority = ref(10)

// Template variables - descriptions will be translated in template
const templateVariables = [
  { name: '{{user}}', descKey: 'autoReply.varUser' },
  { name: '{{message}}', descKey: 'autoReply.varMessage' },
  { name: '{{time}}', descKey: 'autoReply.varTime' },
  { name: '{{date}}', descKey: 'autoReply.varDate' },
  { name: '{{channel}}', descKey: 'autoReply.varChannel' },
  { name: '{{$1}}', descKey: 'autoReply.varCapture1' },
  { name: '{{$2}}', descKey: 'autoReply.varCapture2' },
]

// Validation
const errors = ref<Record<string, string>>({})

const isValid = computed(() => {
  return (
    name.value.trim() !== '' &&
    triggerValue.value.trim() !== '' &&
    responses.value.some((r) => r.trim() !== '') &&
    Object.keys(errors.value).length === 0
  )
})

// Watch for rule changes (edit mode)
watch(
  () => props.rule,
  (newRule) => {
    if (newRule) {
      name.value = newRule.name
      triggerType.value = newRule.trigger_type
      triggerValue.value = newRule.trigger_value
      responses.value = [...newRule.responses]
      priority.value = newRule.priority
    } else {
      resetForm()
    }
  },
  { immediate: true }
)

// Validate regex
watch(
  [triggerType, triggerValue],
  ([type, value]) => {
    if (type === 'regex' && value) {
      try {
        new RegExp(value)
        delete errors.value.triggerValue
      } catch {
        errors.value.triggerValue = t('autoReply.invalidRegex')
      }
    } else {
      delete errors.value.triggerValue
    }
  }
)

function resetForm(): void {
  name.value = ''
  triggerType.value = 'keyword'
  triggerValue.value = ''
  responses.value = ['']
  priority.value = 10
  errors.value = {}
}

function addResponse(): void {
  responses.value.push('')
}

function removeResponse(index: number): void {
  if (responses.value.length > 1) {
    responses.value.splice(index, 1)
  }
}

function insertVariable(variable: string, responseIndex: number): void {
  const textarea = document.querySelector(
    `textarea[data-response-index="${responseIndex}"]`
  ) as HTMLTextAreaElement
  if (textarea) {
    const start = textarea.selectionStart
    const end = textarea.selectionEnd
    const text = responses.value[responseIndex] || ''
    responses.value[responseIndex] =
      text.substring(0, start) + variable + text.substring(end)
    // Restore cursor position
    setTimeout(() => {
      textarea.focus()
      textarea.setSelectionRange(start + variable.length, start + variable.length)
    }, 0)
  } else {
    responses.value[responseIndex] = (responses.value[responseIndex] || '') + variable
  }
}

function handleSubmit(): void {
  if (!isValid.value) return

  const data: CreateRuleRequest = {
    name: name.value.trim(),
    trigger_type: triggerType.value,
    trigger_value: triggerValue.value.trim(),
    responses: responses.value.filter((r) => r.trim() !== ''),
    priority: priority.value,
  }

  emit('save', data)
}
</script>

<template>
  <form class="space-y-4" @submit.prevent="handleSubmit">
    <!-- Name -->
    <div>
      <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{ t('autoReply.ruleName') }}</label>
      <input
        v-model="name"
        type="text"
        :placeholder="t('autoReply.enterRuleName')"
        class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400 border border-gray-300 dark:border-gray-600"
        required
      />
    </div>

    <!-- Trigger Type -->
    <div>
      <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{ t('autoReply.triggerType') }}</label>
      <select
        v-model="triggerType"
        class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400 border border-gray-300 dark:border-gray-600"
      >
        <option value="keyword">{{ t('autoReply.keywordExact') }}</option>
        <option value="contains">{{ t('autoReply.contains') }}</option>
        <option value="prefix">{{ t('autoReply.prefixStartsWith') }}</option>
        <option value="suffix">{{ t('autoReply.suffixEndsWith') }}</option>
        <option value="regex">{{ t('autoReply.regularExpression') }}</option>
      </select>
    </div>

    <!-- Trigger Value -->
    <div>
      <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
        {{ t('autoReply.triggerValue') }}
        <span v-if="triggerType === 'regex'" class="text-gray-400 dark:text-gray-500 font-normal">
          {{ t('autoReply.regexPattern') }}
        </span>
      </label>
      <input
        v-model="triggerValue"
        type="text"
        :placeholder="t('autoReply.enterTriggerText')"
        class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 border border-gray-300 dark:border-gray-600"
        :class="errors.triggerValue ? 'ring-2 ring-red-500' : 'focus:ring-gray-900 dark:focus:ring-gray-400'"
        required
      />
      <p v-if="errors.triggerValue" class="mt-1 text-sm text-red-500 dark:text-red-400">
        {{ errors.triggerValue }}
      </p>
    </div>

    <!-- Priority -->
    <div>
      <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
        {{ t('autoReply.priority') }}
        <span class="text-gray-400 dark:text-gray-500 font-normal">{{ t('autoReply.priorityHint') }}</span>
      </label>
      <input
        v-model.number="priority"
        type="number"
        min="0"
        max="100"
        class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400 border border-gray-300 dark:border-gray-600"
      />
    </div>

    <!-- Responses -->
    <div>
      <div class="flex items-center justify-between mb-2">
        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
          {{ t('autoReply.responses') }}
          <span class="text-gray-400 dark:text-gray-500 font-normal">{{ t('autoReply.responsesHint') }}</span>
        </label>
        <button
          type="button"
          class="text-sm text-gray-900 dark:text-gray-300 hover:text-gray-700 dark:hover:text-gray-200"
          @click="addResponse"
        >
          {{ t('autoReply.addResponse') }}
        </button>
      </div>

      <!-- Template Variables -->
      <div class="mb-3 p-3 bg-gray-100 dark:bg-gray-700 rounded-lg border border-gray-200 dark:border-gray-700">
        <div class="text-xs text-gray-500 dark:text-gray-500 mb-2">{{ t('autoReply.availableVariables') }}</div>
        <div class="flex flex-wrap gap-2">
          <button
            v-for="variable in templateVariables"
            :key="variable.name"
            type="button"
            class="px-2 py-1 bg-gray-700 dark:bg-gray-500 hover:bg-gray-300 dark:hover:bg-gray-600 text-gray-700 dark:text-gray-300 rounded text-xs transition-colors"
            :title="t(variable.descKey)"
            @click="insertVariable(variable.name, 0)"
          >
            {{ variable.name }}
          </button>
        </div>
      </div>

      <!-- Response Inputs -->
      <div class="space-y-2">
        <div
          v-for="(_response, index) in responses"
          :key="index"
          class="flex gap-2"
        >
          <textarea
            v-model="responses[index]"
            :data-response-index="index"
            rows="2"
            :placeholder="t('autoReply.enterResponseMessage')"
            class="flex-1 bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400 resize-none border border-gray-300 dark:border-gray-600"
          />
          <button
            v-if="responses.length > 1"
            type="button"
            class="px-2 text-gray-400 hover:text-red-500 dark:hover:text-red-400 transition-colors"
            @click="removeResponse(index)"
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="h-5 w-5"
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
      </div>
    </div>

    <!-- Actions -->
    <div class="flex justify-end gap-3 pt-4 border-t border-gray-200 dark:border-gray-700">
      <button
        type="button"
        class="px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-300 dark:hover:bg-gray-600 text-gray-700 dark:text-white rounded-lg transition-colors"
        :disabled="loading"
        @click="emit('cancel')"
      >
        {{ t('common.cancel') }}
      </button>
      <button
        type="submit"
        class="px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-700 dark:hover:bg-gray-600 text-white rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
        :disabled="loading || !isValid"
      >
        {{ loading ? t('autoReply.saving') : rule ? t('autoReply.updateRule') : t('autoReply.createRule') }}
      </button>
    </div>
  </form>
</template>
