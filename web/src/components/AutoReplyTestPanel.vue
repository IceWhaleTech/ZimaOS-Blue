<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { TestRuleResponse } from '@/api/autoreply'

const { t } = useI18n()

const props = defineProps<{
  loading?: boolean
}>()

const emit = defineEmits<{
  test: [message: string, channel?: string]
  close: []
}>()

const testMessage = ref('')
const testChannel = ref('')
const testResult = ref<TestRuleResponse | null>(null)
const tested = ref(false)

const channels = computed(() => [
  { value: '', label: t('autoReply.anyChannel') },
  { value: 'telegram', label: 'Telegram' },
  { value: 'discord', label: 'Discord' },
  { value: 'slack', label: 'Slack' },
  { value: 'wechat', label: 'WeChat' },
  { value: 'feishu', label: 'Feishu' },
  { value: 'matrix', label: 'Matrix' },
  { value: 'whatsapp', label: 'WhatsApp' },
  { value: 'signal', label: 'Signal' },
])

function handleTest(): void {
  if (!testMessage.value.trim()) return
  tested.value = true
  emit('test', testMessage.value, testChannel.value || undefined)
}

function setResult(result: TestRuleResponse | null): void {
  testResult.value = result
}

function reset(): void {
  testMessage.value = ''
  testChannel.value = ''
  testResult.value = null
  tested.value = false
}

defineExpose({
  setResult,
  reset,
})
</script>

<template>
  <div class="space-y-4">
    <div class="text-sm text-gray-500 dark:text-gray-400 mb-4">
      {{ t('autoReply.testDescription') }}
    </div>

    <!-- Test Input -->
    <div>
      <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{ t('autoReply.testMessage') }}</label>
      <textarea
        v-model="testMessage"
        rows="3"
        :placeholder="t('autoReply.enterTestMessage')"
        class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500 resize-none border border-gray-300 dark:border-gray-600"
        @keydown.ctrl.enter="handleTest"
      />
    </div>

    <!-- Channel Selection -->
    <div>
      <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{ t('autoReply.channelOptional') }}</label>
      <select
        v-model="testChannel"
        class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500 border border-gray-300 dark:border-gray-600"
      >
        <option v-for="channel in channels" :key="channel.value" :value="channel.value">
          {{ channel.label }}
        </option>
      </select>
    </div>

    <!-- Test Button -->
    <button
      class="w-full px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
      :disabled="loading || !testMessage.trim()"
      @click="handleTest"
    >
      {{ loading ? t('autoReply.testing') : t('autoReply.testMessageBtn') }}
    </button>

    <!-- Result -->
    <div v-if="tested && testResult !== null" class="mt-4">
      <div class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t('autoReply.result') }}</div>

      <div
        v-if="testResult.matched"
        class="bg-green-50 dark:bg-green-900/30 border border-green-200 dark:border-green-800 rounded-lg p-4"
      >
        <div class="flex items-center gap-2 mb-2">
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-5 w-5 text-green-600 dark:text-green-400"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M5 13l4 4L19 7"
            />
          </svg>
          <span class="text-green-700 dark:text-green-400 font-medium">{{ t('autoReply.matched') }}</span>
        </div>
        <div class="space-y-2 text-sm">
          <div>
            <span class="text-gray-500 dark:text-gray-400">{{ t('autoReply.rule') }}:</span>
            <span class="text-gray-900 dark:text-white ml-2">{{ testResult.rule_name }}</span>
          </div>
          <div>
            <span class="text-gray-500 dark:text-gray-400">{{ t('autoReply.response') }}:</span>
            <div class="mt-1 p-2 bg-gray-100 dark:bg-gray-800 rounded text-gray-700 dark:text-gray-300">
              {{ testResult.response }}
            </div>
          </div>
        </div>
      </div>

      <div
        v-else
        class="bg-gray-100 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg p-4"
      >
        <div class="flex items-center gap-2">
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-5 w-5 text-gray-400"
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
          <span class="text-gray-500 dark:text-gray-400">{{ t('autoReply.noMatchingRule') }}</span>
        </div>
      </div>
    </div>

    <!-- Actions -->
    <div class="flex justify-end gap-3 pt-4 border-t border-gray-200 dark:border-gray-700">
      <button
        type="button"
        class="px-4 py-2 bg-gray-200 dark:bg-gray-700 hover:bg-gray-300 dark:hover:bg-gray-600 text-gray-700 dark:text-white rounded-lg transition-colors"
        @click="reset"
      >
        {{ t('autoReply.reset') }}
      </button>
      <button
        type="button"
        class="px-4 py-2 bg-gray-300 dark:bg-gray-600 hover:bg-gray-400 dark:hover:bg-gray-500 text-gray-700 dark:text-white rounded-lg transition-colors"
        @click="emit('close')"
      >
        {{ t('common.close') }}
      </button>
    </div>
  </div>
</template>
