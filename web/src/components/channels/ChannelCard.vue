<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import PasswordInput from '@/components/ui/PasswordInput.vue'

interface ChannelFieldDef {
  key: string
  labelKey: string
  type: 'text' | 'password' | 'tel' | 'url' | 'textarea'
  placeholder?: string
  placeholderKey?: string
  value: string
  required?: boolean
}

interface ChannelDef {
  id: string
  nameKey?: string
  name?: string
  icon: string
  enabled: boolean
  status: 'connected' | 'disconnected' | 'error' | 'connecting'
  lastError?: string
  descriptionKey: string
  hintKey?: string
  docUrl?: string
  fields: ChannelFieldDef[]
  // Message statistics
  messagesReceived?: number
  messagesSent?: number
  lastMessageAt?: string
  lastReplyAt?: string
}

const props = defineProps<{
  channel: ChannelDef
  expanded: boolean
  toggling: boolean
  saving: boolean
  testingConnection: boolean
  testResult: { channelId: string; success: boolean; message: string } | null
}>()

const emit = defineEmits<{
  toggle: []
  toggleEnabled: [enabled: boolean]
  save: []
  testConnection: []
  updateField: [fieldIndex: number, value: string]
}>()

const { t } = useI18n()

// Pre-computed translated values
const translatedChannel = computed(() => ({
  name: props.channel.nameKey ? t(props.channel.nameKey) : props.channel.name!,
  description: t(props.channel.descriptionKey),
  hint: props.channel.hintKey ? t(props.channel.hintKey) : undefined,
  fields: props.channel.fields.map(f => ({
    ...f,
    label: t(f.labelKey),
    placeholder: f.key === 'encrypt_key' 
      ? t('channels.encryptKeyPlaceholder') 
      : (f.placeholderKey ? t(f.placeholderKey) : f.placeholder || ''),
  })),
}))

const statusColor = computed(() => {
  switch (props.channel.status) {
    case 'connected': return 'bg-green-500'
    case 'connecting': return 'bg-yellow-500 animate-pulse'
    case 'error': return 'bg-red-500'
    default: return 'bg-gray-400'
  }
})

const statusText = computed(() => {
  switch (props.channel.status) {
    case 'connected': return t('channels.statusConnected')
    case 'connecting': return t('channels.statusConnecting')
    case 'error': return t('channels.statusError')
    default: return t('channels.statusDisconnected')
  }
})

const statusTitle = computed(() => {
  return statusText.value + (props.channel.lastError ? ': ' + props.channel.lastError : '')
})

// Pre-computed field values for specific channels
const feishuAppId = computed(() =>
  props.channel.id === 'feishu'
    ? props.channel.fields.find(f => f.key === 'app_id')?.value
    : null
)

const telegramBotUsername = computed(() =>
  props.channel.id === 'telegram'
    ? props.channel.fields.find(f => f.key === 'bot_username')?.value
    : null
)

const dingtalkRobotCode = computed(() =>
  props.channel.id === 'dingtalk'
    ? props.channel.fields.find(f => f.key === 'robot_code')?.value
    : null
)

const whatsappPhoneNumber = computed(() => {
  if (props.channel.id !== 'whatsapp') return null
  const phone = props.channel.fields.find(f => f.key === 'phone_number')?.value
  return phone ? phone.replace(/[^0-9]/g, '') : null
})

const showTestResult = computed(() =>
  props.testResult && props.testResult.channelId === props.channel.id
)

function handleFieldInput(fieldIndex: number, event: Event) {
  const target = event.target as HTMLInputElement | HTMLTextAreaElement
  emit('updateField', fieldIndex, target.value)
}

// Format time as fixed format (YYYY-MM-DD HH:mm:ss) for tooltip
function formatTime(dateStr: string | undefined): string {
  if (!dateStr) return '-'
  const date = new Date(dateStr)
  const pad = (n: number) => n.toString().padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

// Format time as relative (刚刚, X分钟前, X小时前, or specific date)
function formatRelativeTime(dateStr: string | undefined): string {
  if (!dateStr) return '-'
  const date = new Date(dateStr)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffSeconds = Math.floor(diffMs / 1000)
  const diffMinutes = Math.floor(diffSeconds / 60)
  const diffHours = Math.floor(diffMinutes / 60)
  const diffDays = Math.floor(diffHours / 24)

  if (diffSeconds < 60) {
    return '刚刚'
  } else if (diffMinutes < 60) {
    return `${diffMinutes}分钟前`
  } else if (diffHours < 24) {
    return `${diffHours}小时前`
  } else if (diffDays < 7) {
    return `${diffDays}天前`
  } else {
    // Show specific date for older times
    const pad = (n: number) => n.toString().padStart(2, '0')
    return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
  }
}
</script>

<template>
  <div class="bg-white dark:bg-gray-700 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
    <!-- Channel Header -->
    <div
      class="flex items-center gap-4 p-4 cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors"
      @click="emit('toggle')"
    >
      <img :src="channel.icon" :alt="translatedChannel.name" class="w-10 h-10 object-contain" />
      <div class="flex-1 min-w-0">
        <div class="flex items-center gap-2">
          <h3 class="font-medium text-gray-900 dark:text-white">{{ translatedChannel.name }}</h3>
          <span
            class="w-2 h-2 rounded-full"
            :class="statusColor"
            :title="statusTitle"
          ></span>
        </div>
        <p class="text-sm text-gray-500 dark:text-gray-400 truncate">
          <template v-if="channel.status === 'error' && channel.lastError">
            <span class="text-red-500">{{ channel.lastError }}</span>
          </template>
          <template v-else>
            {{ translatedChannel.description }}
          </template>
        </p>
      </div>
      <div class="flex items-center gap-3">
        <label class="relative inline-flex items-center cursor-pointer" @click.stop>
          <input
            :checked="channel.enabled"
            type="checkbox"
            class="sr-only peer"
            :disabled="toggling"
            @change="emit('toggleEnabled', ($event.target as HTMLInputElement).checked)"
          />
          <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-2 peer-focus:ring-gray-900 dark:focus:ring-gray-400 dark:peer-focus:ring-gray-900 dark:focus:ring-gray-400 rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-gray-700 dark:peer-checked:bg-gray-700600 peer-disabled:opacity-50"></div>
        </label>
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="h-5 w-5 text-gray-400 transition-transform"
          :class="{ 'rotate-180': expanded }"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
        </svg>
      </div>
    </div>

    <!-- Channel Config (Expanded) -->
    <div
      v-if="expanded"
      class="border-t border-gray-200 dark:border-gray-700 p-4 bg-gray-50 dark:bg-gray-700/50"
    >
      <!-- Runtime Statistics (when enabled) -->
      <template v-if="channel.enabled">
        <!-- Connection Status -->
        <div
class="mb-4 p-3 rounded-lg" :class="{
          'bg-green-50 dark:bg-green-900/20': channel.status === 'connected',
          'bg-yellow-50 dark:bg-yellow-900/20': channel.status === 'connecting',
          'bg-red-50 dark:bg-red-900/20': channel.status === 'error',
          'bg-gray-100 dark:bg-gray-700/50': channel.status === 'disconnected'
        }">
          <div class="flex items-center gap-2">
            <span class="w-2 h-2 rounded-full" :class="statusColor"></span>
            <span
class="text-sm font-medium" :class="{
              'text-green-700 dark:text-green-400': channel.status === 'connected',
              'text-yellow-700 dark:text-yellow-400': channel.status === 'connecting',
              'text-red-700 dark:text-red-400': channel.status === 'error',
              'text-gray-600 dark:text-gray-400': channel.status === 'disconnected'
            }">{{ statusText }}</span>
          </div>
          <p v-if="channel.lastError" class="text-xs text-red-600 dark:text-red-400 mt-1">{{ channel.lastError }}</p>
        </div>

        <!-- Statistics Row -->
        <div class="flex items-center justify-between gap-4 mb-4 p-3 bg-white dark:bg-gray-700 rounded-lg border border-gray-200 dark:border-gray-600">
          <div class="flex items-center gap-6">
            <div class="flex items-center gap-2">
              <span class="text-xs text-gray-500 dark:text-gray-400">{{ t('channels.messagesReceived') }}:</span>
              <span class="text-sm font-semibold text-gray-900 dark:text-white">{{ channel.messagesReceived ?? 0 }}</span>
            </div>
            <div class="flex items-center gap-2">
              <span class="text-xs text-gray-500 dark:text-gray-400">{{ t('channels.messagesSent') }}:</span>
              <span class="text-sm font-semibold text-gray-900 dark:text-white">{{ channel.messagesSent ?? 0 }}</span>
            </div>
          </div>
          <div class="flex items-center gap-6 text-xs">
            <div class="flex items-center gap-1">
              <span class="text-gray-500 dark:text-gray-400">{{ t('channels.lastMessageReceived') }}:</span>
              <span class="text-gray-700 dark:text-gray-300 font-mono cursor-help" :title="formatTime(channel.lastMessageAt)">{{ formatRelativeTime(channel.lastMessageAt) }}</span>
            </div>
            <div class="flex items-center gap-1">
              <span class="text-gray-500 dark:text-gray-400">{{ t('channels.lastReplySent') }}:</span>
              <span class="text-gray-700 dark:text-gray-300 font-mono cursor-help" :title="formatTime(channel.lastReplyAt)">{{ formatRelativeTime(channel.lastReplyAt) }}</span>
            </div>
          </div>
        </div>

        <!-- Quick Links (when enabled) -->
        <div class="flex flex-wrap items-center gap-3 pt-3 border-t border-gray-200 dark:border-gray-700">
          <a
            v-if="channel.docUrl"
            :href="channel.docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="inline-flex items-center gap-1.5 px-3 py-2 text-sm text-gray-900 dark:text-white dark:text-gray-900 dark:text-white hover:text-gray-900 dark:text-white dark:hover:text-gray-900 dark:text-white hover:underline"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 18.477 18.247 18 16.5 18c-1.746 0-3.332.477-4.5 1.253" />
            </svg>
            {{ t('channels.viewDocs') }}
          </a>
          <!-- Feishu Open Bot Chat Link -->
          <a
            v-if="feishuAppId"
            :href="`https://applink.feishu.cn/client/bot/open?appId=${feishuAppId}`"
            target="_blank"
            rel="noopener noreferrer"
            class="inline-flex items-center gap-1.5 px-3 py-2 text-sm text-gray-900 dark:text-white dark:text-gray-900 dark:text-white hover:text-gray-900 dark:text-white dark:hover:text-gray-900 dark:text-white hover:underline"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
            </svg>
            {{ t('channels.feishuOpenChat') }}
          </a>
          <!-- Telegram Open Bot Chat Link -->
          <a
            v-if="telegramBotUsername"
            :href="`https://t.me/${telegramBotUsername}`"
            target="_blank"
            rel="noopener noreferrer"
            class="inline-flex items-center gap-1.5 px-3 py-2 text-sm text-gray-900 dark:text-white dark:text-gray-900 dark:text-white hover:text-gray-900 dark:text-white dark:hover:text-gray-900 dark:text-white hover:underline"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
            </svg>
            {{ t('channels.telegramOpenChat') }}
          </a>
          <!-- DingTalk Open Bot Chat Link -->
          <a
            v-if="dingtalkRobotCode"
            :href="`dingtalk://dingtalkclient/action/sendRobot?robotCode=${dingtalkRobotCode}`"
            target="_blank"
            rel="noopener noreferrer"
            class="inline-flex items-center gap-1.5 px-3 py-2 text-sm text-gray-900 dark:text-white dark:text-gray-900 dark:text-white hover:text-gray-900 dark:text-white dark:hover:text-gray-900 dark:text-white hover:underline"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
            </svg>
            {{ t('channels.dingtalkOpenChat') }}
          </a>
          <!-- WhatsApp Open Chat Link -->
          <a
            v-if="whatsappPhoneNumber"
            :href="`https://wa.me/${whatsappPhoneNumber}`"
            target="_blank"
            rel="noopener noreferrer"
            class="inline-flex items-center gap-1.5 px-3 py-2 text-sm text-gray-900 dark:text-white dark:text-gray-900 dark:text-white hover:text-gray-900 dark:text-white dark:hover:text-gray-900 dark:text-white hover:underline"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
            </svg>
            {{ t('channels.whatsappOpenChat') }}
          </a>
        </div>
      </template>

      <!-- Configuration Form (when disabled) -->
      <template v-else>
      <!-- Hint -->
      <p v-if="translatedChannel.hint" class="text-xs text-gray-500 dark:text-gray-400 mb-4 flex items-start gap-2">
        <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 flex-shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
        {{ translatedChannel.hint }}
      </p>

      <!-- Fields -->
      <div v-if="translatedChannel.fields.length > 0" class="space-y-4">
        <div v-for="(field, fieldIndex) in translatedChannel.fields" :key="field.key">
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            {{ field.label }}
            <span v-if="field.required" class="text-red-500">*</span>
          </label>
          <textarea
            v-if="field.type === 'textarea'"
            :value="channel.fields[fieldIndex]?.value"
            :name="field.key"
            :placeholder="field.placeholder"
            rows="4"
            class="w-full bg-white dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 border border-gray-300 dark:border-gray-600 focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400 font-mono text-sm"
            @input="handleFieldInput(fieldIndex, $event)"
          />
          <PasswordInput
            v-else-if="field.type === 'password'"
            :model-value="channel.fields[fieldIndex]?.value ?? ''"
            :name="field.key"
            :placeholder="field.placeholder"
            class="w-full bg-white dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 border border-gray-300 dark:border-gray-600 focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400"
            @update:model-value="emit('updateField', fieldIndex, $event)"
          />
          <input
            v-else
            :value="channel.fields[fieldIndex]?.value"
            :name="field.key"
            :type="field.type"
            :placeholder="field.placeholder"
            class="w-full bg-white dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 border border-gray-300 dark:border-gray-600 focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400"
            @input="handleFieldInput(fieldIndex, $event)"
          />
        </div>
      </div>

      <p v-else class="text-sm text-gray-500 dark:text-gray-400">
        {{ t('channels.noConfigRequired') }}
      </p>

      <!-- Test Result -->
      <div
        v-if="showTestResult"
        class="mt-4 p-3 rounded-lg"
        :class="testResult!.success ? 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400' : 'bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-400'"
      >
        {{ testResult!.message }}
      </div>

      <!-- Actions -->
      <div class="flex flex-wrap items-center gap-3 mt-4">
        <button
          v-if="channel.fields.length > 0"
          :disabled="testingConnection"
          class="px-4 py-2 bg-gray-200 dark:bg-gray-600 hover:bg-gray-300 dark:hover:bg-gray-500 text-gray-900 dark:text-white rounded-lg transition-colors disabled:opacity-50 text-sm"
          @click="emit('testConnection')"
        >
          {{ testingConnection ? t('channels.testing') : t('channels.testConnection') }}
        </button>
        <button
          :disabled="saving"
          class="px-4 py-2 bg-gray-700 dark:bg-gray-700 hover:bg-gray-700 dark:bg-gray-700 text-white rounded-lg transition-colors disabled:opacity-50 text-sm"
          @click="emit('save')"
        >
          {{ saving ? t('channels.saving') : t('common.save') }}
        </button>
        <a
          v-if="channel.docUrl"
          :href="channel.docUrl"
          target="_blank"
          rel="noopener noreferrer"
          class="inline-flex items-center gap-1.5 px-3 py-2 text-sm text-gray-900 dark:text-white dark:text-gray-900 dark:text-white hover:text-gray-900 dark:text-white dark:hover:text-gray-900 dark:text-white hover:underline"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 18.477 18.247 18 16.5 18c-1.746 0-3.332.477-4.5 1.253" />
          </svg>
          {{ t('channels.viewDocs') }}
        </a>
        <!-- Feishu Open Bot Chat Link -->
        <a
          v-if="feishuAppId"
          :href="`https://applink.feishu.cn/client/bot/open?appId=${feishuAppId}`"
          target="_blank"
          rel="noopener noreferrer"
          class="inline-flex items-center gap-1.5 px-3 py-2 text-sm text-gray-900 dark:text-white dark:text-gray-900 dark:text-white hover:text-gray-900 dark:text-white dark:hover:text-gray-900 dark:text-white hover:underline"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
          </svg>
          {{ t('channels.feishuOpenChat') }}
        </a>
        <!-- Telegram Open Bot Chat Link -->
        <a
          v-if="telegramBotUsername"
          :href="`https://t.me/${telegramBotUsername}`"
          target="_blank"
          rel="noopener noreferrer"
          class="inline-flex items-center gap-1.5 px-3 py-2 text-sm text-gray-900 dark:text-white dark:text-gray-900 dark:text-white hover:text-gray-900 dark:text-white dark:hover:text-gray-900 dark:text-white hover:underline"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
          </svg>
          {{ t('channels.telegramOpenChat') }}
        </a>
        <!-- DingTalk Open Bot Chat Link -->
        <a
          v-if="dingtalkRobotCode"
          :href="`dingtalk://dingtalkclient/action/sendRobot?robotCode=${dingtalkRobotCode}`"
          target="_blank"
          rel="noopener noreferrer"
          class="inline-flex items-center gap-1.5 px-3 py-2 text-sm text-gray-900 dark:text-white dark:text-gray-900 dark:text-white hover:text-gray-900 dark:text-white dark:hover:text-gray-900 dark:text-white hover:underline"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
          </svg>
          {{ t('channels.dingtalkOpenChat') }}
        </a>
        <!-- WhatsApp Open Chat Link -->
        <a
          v-if="whatsappPhoneNumber"
          :href="`https://wa.me/${whatsappPhoneNumber}`"
          target="_blank"
          rel="noopener noreferrer"
          class="inline-flex items-center gap-1.5 px-3 py-2 text-sm text-gray-900 dark:text-white dark:text-gray-900 dark:text-white hover:text-gray-900 dark:text-white dark:hover:text-gray-900 dark:text-white hover:underline"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
          </svg>
          {{ t('channels.whatsappOpenChat') }}
        </a>
      </div>
      </template>
    </div>
  </div>
</template>
