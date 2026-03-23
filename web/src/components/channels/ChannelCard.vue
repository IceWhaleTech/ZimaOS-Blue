<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useTauri } from '@/composables/useTauri'
import PasswordInput from '@/components/ui/PasswordInput.vue'
import { getChannelIconStyleVars } from '@/utils/channelIcons'

interface ChannelFieldDef {
  key: string
  labelKey: string
  type: 'text' | 'password' | 'tel' | 'url' | 'textarea' | 'toggle'
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

const { t, te } = useI18n()
const { openInBrowser } = useTauri()

function openLink(url: string) {
  openInBrowser(url)
}

function tr(key: string | undefined, fallback = ''): string {
  if (!key) return fallback
  return te(key) ? t(key) : fallback
}

const translatedChannel = computed(() => ({
  name: props.channel.nameKey
    ? tr(props.channel.nameKey, props.channel.name || '')
    : props.channel.name!,
  description: tr(props.channel.descriptionKey, ''),
  hint: props.channel.hintKey ? tr(props.channel.hintKey, '') : undefined,
  fields: props.channel.fields.map((field) => ({
    ...field,
    label: tr(field.labelKey, field.key),
    placeholder:
      field.key === 'encrypt_key'
        ? t('channels.encryptKeyPlaceholder')
        : field.placeholderKey
          ? tr(field.placeholderKey, field.placeholder || '')
          : field.placeholder || '',
  })),
}))

const statusColor = computed(() => {
  switch (props.channel.status) {
    case 'connected':
      return 'bg-green-500'
    case 'connecting':
      return 'bg-yellow-500 animate-pulse'
    case 'error':
      return 'bg-red-500'
    default:
      return 'bg-gray-400'
  }
})

const statusText = computed(() => {
  switch (props.channel.status) {
    case 'connected':
      return t('channels.statusConnected')
    case 'connecting':
      return t('channels.statusConnecting')
    case 'error':
      return t('channels.statusError')
    default:
      return t('channels.statusDisconnected')
  }
})

const statusTitle = computed(() => {
  return statusText.value + (props.channel.lastError ? `: ${props.channel.lastError}` : '')
})

const statusTone = computed(() => {
  switch (props.channel.status) {
    case 'connected':
      return 'connected'
    case 'connecting':
      return 'connecting'
    case 'error':
      return 'error'
    default:
      return 'disconnected'
  }
})

const cardClasses = computed(() => [
  'channel-card',
  'dashboard-card-surface',
  `channel-card--${statusTone.value}`,
  {
    'channel-card--expanded': props.expanded,
    'channel-card--enabled': props.channel.enabled,
  },
])

const statusBadgeClass = computed(() => `channel-card__status-badge--${statusTone.value}`)
const statusPanelClass = computed(() => `channel-card__status-panel--${statusTone.value}`)
const channelIconStyle = computed(() => getChannelIconStyleVars(props.channel.id))

const feishuAppId = computed(() =>
  props.channel.id === 'feishu'
    ? props.channel.fields.find((field) => field.key === 'app_id')?.value
    : null
)

const telegramBotUsername = computed(() =>
  props.channel.id === 'telegram'
    ? props.channel.fields.find((field) => field.key === 'bot_username')?.value
    : null
)

const dingtalkRobotCode = computed(() =>
  props.channel.id === 'dingtalk'
    ? props.channel.fields.find((field) => field.key === 'robot_code')?.value
    : null
)

const whatsappPhoneNumber = computed(() => {
  if (props.channel.id !== 'whatsapp') return null
  const phone = props.channel.fields.find((field) => field.key === 'phone_number')?.value
  return phone ? phone.replace(/[^0-9]/g, '') : null
})

const showTestResult = computed(
  () => props.testResult && props.testResult.channelId === props.channel.id
)

function handleFieldInput(fieldIndex: number, event: Event) {
  const target = event.target as HTMLInputElement | HTMLTextAreaElement
  emit('updateField', fieldIndex, target.value)
}

function toggleFieldChecked(value: string | undefined): boolean {
  switch ((value || '').trim().toLowerCase()) {
    case '1':
    case 'true':
    case 'yes':
    case 'y':
    case 'on':
      return true
    default:
      return false
  }
}

function formatTime(dateStr: string | undefined): string {
  if (!dateStr) return '-'
  const date = new Date(dateStr)
  const pad = (n: number) => n.toString().padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

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
  }
  if (diffMinutes < 60) {
    return `${diffMinutes}分钟前`
  }
  if (diffHours < 24) {
    return `${diffHours}小时前`
  }
  if (diffDays < 7) {
    return `${diffDays}天前`
  }

  const pad = (n: number) => n.toString().padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
}
</script>

<template>
  <div :class="cardClasses">
    <div class="channel-card__header" @click="emit('toggle')">
      <div class="channel-card__identity">
        <div class="channel-card__icon-shell">
          <img
            :src="channel.icon"
            :alt="translatedChannel.name"
            class="channel-card__icon"
            :style="channelIconStyle"
          />
        </div>
        <div class="channel-card__copy">
          <div class="channel-card__title-row">
            <h3 class="channel-card__title">{{ translatedChannel.name }}</h3>
            <span class="channel-card__status-badge" :class="statusBadgeClass" :title="statusTitle">
              <span class="channel-card__status-dot"></span>
              {{ statusText }}
            </span>
          </div>
          <p
            class="channel-card__description"
            :class="{
              'channel-card__description--error': channel.status === 'error' && channel.lastError,
            }"
          >
            <template v-if="channel.status === 'error' && channel.lastError">
              {{ channel.lastError }}
            </template>
            <template v-else>
              {{ translatedChannel.description }}
            </template>
          </p>
        </div>
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
          <div
            class="channel-card__toggle bg-gray-200 peer-focus:outline-none peer-focus:ring-2 peer-focus:ring-gray-900 dark:peer-focus:ring-gray-400 rounded-full peer dark:bg-slate-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:bg-white after:border-gray-300 after:border after:rounded-full after:transition-all dark:border-slate-500 peer-checked:bg-green-600 dark:peer-checked:bg-green-500 peer-disabled:opacity-50"
          ></div>
        </label>
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="channel-card__chevron text-gray-400 transition-transform"
          :class="{ 'rotate-180': expanded }"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M19 9l-7 7-7-7"
          />
        </svg>
      </div>
    </div>

    <div v-if="expanded" class="channel-card__body">
      <template v-if="channel.enabled">
        <div class="channel-card__status-panel dashboard-card-subsurface" :class="statusPanelClass">
          <div class="flex items-center gap-2">
            <span class="w-2 h-2 rounded-full" :class="statusColor"></span>
            <span
              class="channel-card__status-text text-sm font-medium"
              :class="{
                'text-green-700 dark:text-green-400': channel.status === 'connected',
                'text-yellow-700 dark:text-yellow-400': channel.status === 'connecting',
                'text-red-700 dark:text-red-400': channel.status === 'error',
                'text-gray-600 dark:text-gray-400': channel.status === 'disconnected',
              }"
            >
              {{ statusText }}
            </span>
          </div>
          <p
            v-if="channel.lastError"
            class="channel-card__status-error text-xs text-red-600 dark:text-red-400 mt-1"
          >
            {{ channel.lastError }}
          </p>
        </div>

        <div class="channel-card__metrics">
          <div class="channel-card__metric dashboard-card-subsurface">
            <span class="channel-card__metric-label">{{ t('channels.messagesReceived') }}</span>
            <span class="channel-card__metric-value">{{ channel.messagesReceived ?? 0 }}</span>
          </div>
          <div class="channel-card__metric dashboard-card-subsurface">
            <span class="channel-card__metric-label">{{ t('channels.messagesSent') }}</span>
            <span class="channel-card__metric-value">{{ channel.messagesSent ?? 0 }}</span>
          </div>
          <div class="channel-card__metric dashboard-card-subsurface">
            <span class="channel-card__metric-label">{{ t('channels.lastMessageReceived') }}</span>
            <span
              class="channel-card__metric-meta font-mono cursor-help"
              :title="formatTime(channel.lastMessageAt)"
            >
              {{ formatRelativeTime(channel.lastMessageAt) }}
            </span>
          </div>
          <div class="channel-card__metric dashboard-card-subsurface">
            <span class="channel-card__metric-label">{{ t('channels.lastReplySent') }}</span>
            <span
              class="channel-card__metric-meta font-mono cursor-help"
              :title="formatTime(channel.lastReplyAt)"
            >
              {{ formatRelativeTime(channel.lastReplyAt) }}
            </span>
          </div>
        </div>

        <div class="channel-card__actions channel-card__actions--links">
          <a
            v-if="channel.docUrl"
            href="#"
            class="channel-card__link dashboard-card-chip"
            @click.prevent="openLink(channel.docUrl!)"
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
                d="M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 18.477 18.247 18 16.5 18c-1.746 0-3.332.477-4.5 1.253"
              />
            </svg>
            {{ t('channels.viewDocs') }}
          </a>
          <a
            v-if="feishuAppId"
            href="#"
            class="channel-card__link dashboard-card-chip"
            @click.prevent="
              openLink(`https://applink.feishu.cn/client/bot/open?appId=${feishuAppId}`)
            "
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
                d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z"
              />
            </svg>
            {{ t('channels.feishuOpenChat') }}
          </a>
          <a
            v-if="telegramBotUsername"
            href="#"
            class="channel-card__link dashboard-card-chip"
            @click.prevent="openLink(`https://t.me/${telegramBotUsername}`)"
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
                d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z"
              />
            </svg>
            {{ t('channels.telegramOpenChat') }}
          </a>
          <a
            v-if="dingtalkRobotCode"
            href="#"
            class="channel-card__link dashboard-card-chip"
            @click.prevent="
              openLink(`dingtalk://dingtalkclient/action/sendRobot?robotCode=${dingtalkRobotCode}`)
            "
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
                d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z"
              />
            </svg>
            {{ t('channels.dingtalkOpenChat') }}
          </a>
          <a
            v-if="whatsappPhoneNumber"
            href="#"
            class="channel-card__link dashboard-card-chip"
            @click.prevent="openLink(`https://wa.me/${whatsappPhoneNumber}`)"
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
                d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z"
              />
            </svg>
            {{ t('channels.whatsappOpenChat') }}
          </a>
        </div>
      </template>

      <template v-else>
        <p v-if="translatedChannel.hint" class="channel-card__hint dashboard-card-subsurface">
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-4 w-4 flex-shrink-0 mt-0.5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
            />
          </svg>
          {{ translatedChannel.hint }}
        </p>

        <div
          v-if="translatedChannel.fields.length > 0"
          class="channel-card__fields grid gap-4 md:grid-cols-2"
        >
          <div
            v-for="(field, fieldIndex) in translatedChannel.fields"
            :key="field.key"
            :class="field.type === 'textarea' || field.type === 'toggle' ? 'md:col-span-2' : ''"
          >
            <label class="channel-card__field-label">
              {{ field.label }}
              <span v-if="field.required" class="text-red-500">*</span>
            </label>
            <label
              v-if="field.type === 'toggle'"
              class="channel-card__toggle-field dashboard-card-subsurface"
            >
              <span class="channel-card__toggle-field-copy">
                {{
                  toggleFieldChecked(channel.fields[fieldIndex]?.value)
                    ? t('common.enabled')
                    : t('common.disabled')
                }}
              </span>
              <input
                :checked="toggleFieldChecked(channel.fields[fieldIndex]?.value)"
                type="checkbox"
                class="sr-only peer"
                @change="
                  emit(
                    'updateField',
                    fieldIndex,
                    ($event.target as HTMLInputElement).checked ? 'true' : 'false'
                  )
                "
              />
              <span
                class="channel-card__toggle relative inline-block bg-gray-200 peer-focus:outline-none peer-focus:ring-2 peer-focus:ring-gray-900 dark:peer-focus:ring-gray-400 rounded-full peer dark:bg-slate-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:bg-white after:border-gray-300 after:border after:rounded-full after:transition-all dark:border-slate-500 peer-checked:bg-green-600 dark:peer-checked:bg-green-500"
              ></span>
            </label>
            <textarea
              v-else-if="field.type === 'textarea'"
              :value="channel.fields[fieldIndex]?.value"
              :name="field.key"
              :placeholder="field.placeholder"
              rows="4"
              class="channel-card__input"
              @input="handleFieldInput(fieldIndex, $event)"
            />
            <PasswordInput
              v-else-if="field.type === 'password'"
              :model-value="channel.fields[fieldIndex]?.value ?? ''"
              :name="field.key"
              :placeholder="field.placeholder"
              class="channel-card__password-field"
              @update:model-value="emit('updateField', fieldIndex, $event)"
            />
            <input
              v-else
              :value="channel.fields[fieldIndex]?.value"
              :name="field.key"
              :type="field.type"
              :placeholder="field.placeholder"
              class="channel-card__input"
              @input="handleFieldInput(fieldIndex, $event)"
            />
          </div>
        </div>

        <p v-else class="channel-card__empty">
          {{ t('channels.noConfigRequired') }}
        </p>

        <div
          v-if="showTestResult"
          class="channel-card__feedback"
          :class="
            testResult!.success
              ? 'channel-card__feedback--success'
              : 'channel-card__feedback--error'
          "
        >
          {{ testResult!.message }}
        </div>

        <div class="channel-card__actions">
          <button
            v-if="channel.fields.length > 0"
            :disabled="testingConnection"
            class="channel-card__action channel-card__action--secondary"
            @click="emit('testConnection')"
          >
            {{ testingConnection ? t('channels.testing') : t('channels.testConnection') }}
          </button>
          <button
            :disabled="saving"
            class="channel-card__action channel-card__action--primary"
            @click="emit('save')"
          >
            {{ saving ? t('channels.saving') : t('common.save') }}
          </button>
          <a
            v-if="channel.docUrl"
            href="#"
            class="channel-card__link dashboard-card-chip"
            @click.prevent="openLink(channel.docUrl!)"
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
                d="M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 18.477 18.247 18 16.5 18c-1.746 0-3.332.477-4.5 1.253"
              />
            </svg>
            {{ t('channels.viewDocs') }}
          </a>
          <a
            v-if="feishuAppId"
            href="#"
            class="channel-card__link dashboard-card-chip"
            @click.prevent="
              openLink(`https://applink.feishu.cn/client/bot/open?appId=${feishuAppId}`)
            "
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
                d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z"
              />
            </svg>
            {{ t('channels.feishuOpenChat') }}
          </a>
          <a
            v-if="telegramBotUsername"
            href="#"
            class="channel-card__link dashboard-card-chip"
            @click.prevent="openLink(`https://t.me/${telegramBotUsername}`)"
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
                d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z"
              />
            </svg>
            {{ t('channels.telegramOpenChat') }}
          </a>
          <a
            v-if="dingtalkRobotCode"
            href="#"
            class="channel-card__link dashboard-card-chip"
            @click.prevent="
              openLink(`dingtalk://dingtalkclient/action/sendRobot?robotCode=${dingtalkRobotCode}`)
            "
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
                d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z"
              />
            </svg>
            {{ t('channels.dingtalkOpenChat') }}
          </a>
          <a
            v-if="whatsappPhoneNumber"
            href="#"
            class="channel-card__link dashboard-card-chip"
            @click.prevent="openLink(`https://wa.me/${whatsappPhoneNumber}`)"
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
                d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z"
              />
            </svg>
            {{ t('channels.whatsappOpenChat') }}
          </a>
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
.channel-card {
  position: relative;
  overflow: hidden;
  border: 1px solid rgba(226, 232, 240, 0.96);
  border-radius: 1.25rem;
  background: #ffffff;
  box-shadow: none;
  color: #0f172a;
  transition:
    transform 180ms ease,
    border-color 180ms ease,
    box-shadow 180ms ease;
}

.channel-card:hover {
  transform: translateY(-1px);
}

.channel-card::before {
  content: '';
  position: absolute;
  inset: 0 0 auto 0;
  height: 2px;
  opacity: 0;
  transition: opacity 180ms ease;
}

.channel-card--expanded::before,
.channel-card:hover::before {
  opacity: 1;
}

.channel-card--connected::before {
  opacity: 1;
  background: rgba(22, 163, 74, 0.9);
}

.channel-card--connecting::before {
  opacity: 1;
  background: rgba(245, 158, 11, 0.92);
}

.channel-card--error::before {
  opacity: 1;
  background: rgba(239, 68, 68, 0.92);
}

.channel-card__header {
  display: flex;
  align-items: center;
  gap: 1.1rem;
  min-height: 4.9rem;
  padding: 0.92rem 1rem;
  cursor: pointer;
  transition: background-color 160ms ease;
}

.channel-card__header:hover {
  background: rgba(148, 163, 184, 0.08);
}

.channel-card__identity {
  min-width: 0;
  flex: 1;
  display: flex;
  align-items: center;
  gap: 0.82rem;
}

.channel-card__icon-shell {
  width: 2.55rem;
  height: 2.55rem;
  flex-shrink: 0;
  display: grid;
  place-items: center;
  border-radius: 0.82rem;
  background: #f8fafc;
  border: 1px solid rgba(226, 232, 240, 0.96);
  box-shadow: none;
  transition:
    border-color 180ms ease,
    background-color 180ms ease,
    transform 180ms ease;
}

.channel-card__icon {
  width: 1.58rem;
  height: 1.58rem;
  object-fit: contain;
  transform: scale(var(--channel-icon-scale, 1));
  transform-origin: center;
}

.channel-card__copy {
  min-width: 0;
  flex: 1;
}

.channel-card__title-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.42rem;
  min-height: 1.3rem;
}

.channel-card__title {
  font-size: 0.9rem;
  font-weight: 700;
  color: #0f172a;
}

.channel-card__description {
  margin: 0.28rem 0 0;
  font-size: 0.76rem;
  line-height: 1.4;
  color: #64748b;
  display: -webkit-box;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.channel-card__description--error {
  color: #b91c1c;
}

.channel-card__status-badge {
  display: inline-flex;
  align-items: center;
  gap: 0.26rem;
  min-height: 1.24rem;
  padding: 0.14rem 0.42rem;
  border-radius: 999px;
  border: 1px solid transparent;
  font-size: 0.58rem;
  font-weight: 700;
  line-height: 1;
}

.channel-card__status-dot {
  width: 0.24rem;
  height: 0.24rem;
  border-radius: 999px;
  background: currentColor;
}

.channel-card__status-badge--connected {
  color: #166534;
  background: rgba(220, 252, 231, 0.95);
  border-color: rgba(134, 239, 172, 0.72);
}

.channel-card__status-badge--connecting {
  color: #92400e;
  background: rgba(254, 243, 199, 0.95);
  border-color: rgba(252, 211, 77, 0.72);
}

.channel-card__status-badge--error {
  color: #b91c1c;
  background: rgba(254, 226, 226, 0.95);
  border-color: rgba(252, 165, 165, 0.72);
}

.channel-card__status-badge--disconnected {
  color: #475569;
  background: rgba(226, 232, 240, 0.92);
  border-color: rgba(203, 213, 225, 0.85);
}

.channel-card__status-badge--connecting .channel-card__status-dot {
  animation: channel-card-status-pulse 1.35s ease-in-out infinite;
}

.channel-card__body {
  padding: 0 1rem 1rem;
  border-top: 1px solid rgba(226, 232, 240, 0.96);
  background: #f8fafc;
  animation: channel-card-body-in 180ms ease;
}

.channel-card__status-panel {
  margin-bottom: 0.82rem;
  padding: 0.72rem 0.84rem;
}

.channel-card__status-text {
  font-size: 0.72rem;
}

.channel-card__status-error {
  font-size: 0.62rem;
}

.channel-card__status-panel--connected {
  border-color: rgba(134, 239, 172, 0.85);
  background: rgba(240, 253, 244, 0.96);
}

.channel-card__status-panel--connecting {
  border-color: rgba(252, 211, 77, 0.82);
  background: rgba(255, 251, 235, 0.98);
}

.channel-card__status-panel--error {
  border-color: rgba(252, 165, 165, 0.8);
  background: rgba(254, 242, 242, 0.98);
}

.channel-card__metrics {
  display: grid;
  gap: 0.56rem;
  margin-bottom: 0.82rem;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.channel-card__metric {
  padding: 0.64rem 0.72rem;
  display: flex;
  flex-direction: column;
  gap: 0.26rem;
  transition:
    transform 160ms ease,
    border-color 160ms ease;
}

.channel-card__metric:hover {
  transform: translateY(-1px);
}

.channel-card__metric-label {
  font-size: 0.58rem;
  line-height: 1.2;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  font-weight: 700;
  color: #64748b;
}

.channel-card__metric-value {
  font-size: 0.98rem;
  line-height: 1;
  letter-spacing: -0.02em;
  font-weight: 800;
  color: #0f172a;
}

.channel-card__metric-meta {
  font-size: 0.66rem;
  color: #475569;
}

.channel-card__hint {
  display: flex;
  align-items: flex-start;
  gap: 0.46rem;
  margin-bottom: 0.82rem;
  padding: 0.72rem 0.84rem;
  font-size: 0.7rem;
  line-height: 1.45;
  color: #475569;
}

.channel-card__fields {
  gap: 0.72rem;
}

.channel-card__field-label {
  display: block;
  margin-bottom: 0.34rem;
  font-size: 0.68rem;
  font-weight: 700;
  color: #334155;
}

.channel-card__input {
  width: 100%;
  min-height: 2.2rem;
  padding: 0.42rem 0.58rem;
  border-radius: 0.7rem;
  border: 1px solid rgba(203, 213, 225, 0.95);
  background: rgba(255, 255, 255, 0.94);
  color: #0f172a;
  font-size: 0.72rem;
  line-height: 1.45;
  transition:
    border-color 150ms ease,
    box-shadow 150ms ease;
}

.channel-card__password-field :deep(input),
:deep(input.channel-card__password-field) {
  width: 100%;
  min-height: 2.2rem;
  padding: 0.42rem 0.58rem;
  border-radius: 0.7rem;
  border: 1px solid rgba(203, 213, 225, 0.95);
  background: rgba(255, 255, 255, 0.94);
  color: #0f172a;
  font-size: 0.72rem;
  line-height: 1.45;
  transition:
    border-color 150ms ease,
    box-shadow 150ms ease;
}

.channel-card__input:focus {
  outline: none;
  border-color: rgba(37, 99, 235, 0.6);
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.12);
}

.channel-card__password-field :deep(input:focus),
:deep(input.channel-card__password-field:focus) {
  outline: none;
  border-color: rgba(37, 99, 235, 0.6);
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.12);
}

textarea.channel-card__input {
  min-height: 5.5rem;
  font-family:
    ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New',
    monospace;
  padding-top: 0.48rem;
  padding-bottom: 0.48rem;
  font-size: 0.68rem;
}

.channel-card__empty {
  font-size: 0.72rem;
  color: #64748b;
}

.channel-card__feedback {
  margin-top: 0.82rem;
  padding: 0.64rem 0.78rem;
  border-radius: 0.76rem;
  font-size: 0.68rem;
  font-weight: 700;
}

.channel-card__feedback--success {
  background: rgba(220, 252, 231, 0.94);
  color: #166534;
  border: 1px solid rgba(134, 239, 172, 0.72);
}

.channel-card__feedback--error {
  background: rgba(254, 226, 226, 0.94);
  color: #b91c1c;
  border: 1px solid rgba(252, 165, 165, 0.72);
}

.channel-card__actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.46rem;
  margin-top: 0.82rem;
}

.channel-card__actions--links {
  margin-top: 0;
}

.channel-card__action {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 2.1rem;
  padding: 0.4rem 0.7rem;
  border-radius: 0.72rem;
  font-size: 0.68rem;
  font-weight: 700;
  flex: 1 1 10rem;
  transition:
    transform 160ms ease,
    filter 160ms ease,
    background-color 160ms ease;
}

.channel-card__action:hover {
  transform: translateY(-1px);
}

.channel-card__action--secondary {
  background: rgba(226, 232, 240, 0.96);
  border: 1px solid rgba(203, 213, 225, 0.92);
  color: #0f172a;
}

.channel-card__action--primary {
  border: 1px solid #0f172a;
  background: #0f172a;
  color: #fff;
}

.channel-card__link {
  display: inline-flex;
  align-items: center;
  gap: 0.46rem;
  min-height: 1.54rem;
  padding: 0.22rem 0.42rem;
  font-size: 0.62rem;
  text-decoration: none;
  transition:
    transform 160ms ease,
    border-color 160ms ease;
}

.channel-card__toggle {
  width: 2.16rem;
  height: 1.16rem;
}

.channel-card__toggle::after {
  top: 2px;
  left: 2px;
  width: 0.82rem;
  height: 0.82rem;
}

.channel-card__toggle-field {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.9rem;
  border-radius: 0.95rem;
  padding: 0.9rem 1rem;
  cursor: pointer;
}

.channel-card__toggle-field-copy {
  font-size: 0.94rem;
  font-weight: 600;
  color: #0f172a;
}

.channel-card__chevron {
  width: 0.84rem;
  height: 0.84rem;
}

.channel-card__link:hover {
  text-decoration: none;
  transform: translateY(-1px);
}

.channel-card--connected .channel-card__icon-shell {
  border-color: rgba(134, 239, 172, 0.8);
  background: rgba(240, 253, 244, 0.96);
  box-shadow: none;
}

.channel-card--connecting .channel-card__icon-shell {
  border-color: rgba(252, 211, 77, 0.82);
  background: rgba(255, 251, 235, 0.98);
  box-shadow: none;
}

.channel-card--error .channel-card__icon-shell {
  border-color: rgba(252, 165, 165, 0.8);
  background: rgba(254, 242, 242, 0.98);
  box-shadow: none;
}

@keyframes channel-card-status-pulse {
  0%,
  100% {
    transform: scale(1);
    opacity: 1;
  }

  50% {
    transform: scale(1.45);
    opacity: 0.55;
  }
}

@keyframes channel-card-body-in {
  from {
    opacity: 0;
    transform: translateY(-6px);
  }

  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@media (min-width: 1024px) {
  .channel-card__metrics {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }
}

@media (max-width: 639px) {
  .channel-card__header {
    align-items: flex-start;
    padding: 0.82rem 0.88rem;
  }

  .channel-card__identity {
    align-items: flex-start;
  }

  .channel-card__body {
    padding: 0 0.88rem 0.88rem;
  }
}

:root.dark .channel-card,
[data-theme='dark'] .channel-card,
html.dark .channel-card {
  border-color: rgba(71, 85, 105, 0.46);
  background: #111827;
  box-shadow: none;
  color: #e2e8f0;
}

:root.dark .channel-card__icon-shell,
[data-theme='dark'] .channel-card__icon-shell,
html.dark .channel-card__icon-shell {
  background: rgba(15, 23, 42, 0.72);
  border-color: rgba(71, 85, 105, 0.5);
}

:root.dark .channel-card__header:hover,
[data-theme='dark'] .channel-card__header:hover,
html.dark .channel-card__header:hover {
  background: rgba(51, 65, 85, 0.28);
}

:root.dark .channel-card__title,
[data-theme='dark'] .channel-card__title,
html.dark .channel-card__title,
:root.dark .channel-card__metric-value,
[data-theme='dark'] .channel-card__metric-value,
html.dark .channel-card__metric-value {
  color: #f8fafc;
}

:root.dark .channel-card__description,
[data-theme='dark'] .channel-card__description,
html.dark .channel-card__description,
:root.dark .channel-card__metric-meta,
[data-theme='dark'] .channel-card__metric-meta,
html.dark .channel-card__metric-meta,
:root.dark .channel-card__hint,
[data-theme='dark'] .channel-card__hint,
html.dark .channel-card__hint,
:root.dark .channel-card__empty,
[data-theme='dark'] .channel-card__empty,
html.dark .channel-card__empty {
  color: #cbd5e1;
}

:root.dark .channel-card__description--error,
[data-theme='dark'] .channel-card__description--error,
html.dark .channel-card__description--error {
  color: #fecaca;
}

:root.dark .channel-card__status-badge--disconnected,
[data-theme='dark'] .channel-card__status-badge--disconnected,
html.dark .channel-card__status-badge--disconnected {
  color: #cbd5e1;
  background: rgba(51, 65, 85, 0.86);
  border-color: rgba(100, 116, 139, 0.72);
}

:root.dark .channel-card__status-badge--connected,
[data-theme='dark'] .channel-card__status-badge--connected,
html.dark .channel-card__status-badge--connected {
  color: #bbf7d0;
  background: rgba(20, 83, 45, 0.52);
  border-color: rgba(74, 222, 128, 0.34);
}

:root.dark .channel-card__status-badge--connecting,
[data-theme='dark'] .channel-card__status-badge--connecting,
html.dark .channel-card__status-badge--connecting {
  color: #fde68a;
  background: rgba(120, 53, 15, 0.48);
  border-color: rgba(251, 191, 36, 0.34);
}

:root.dark .channel-card__status-badge--error,
[data-theme='dark'] .channel-card__status-badge--error,
html.dark .channel-card__status-badge--error {
  color: #fecaca;
  background: rgba(127, 29, 29, 0.5);
  border-color: rgba(248, 113, 113, 0.34);
}

:root.dark .channel-card__body,
[data-theme='dark'] .channel-card__body,
html.dark .channel-card__body {
  background: #1e293b;
  border-top-color: rgba(71, 85, 105, 0.46);
}

:root.dark .channel-card__status-panel--connected,
[data-theme='dark'] .channel-card__status-panel--connected,
html.dark .channel-card__status-panel--connected {
  border-color: rgba(74, 222, 128, 0.34);
  background: rgba(20, 83, 45, 0.32);
}

:root.dark .channel-card__status-panel--connecting,
[data-theme='dark'] .channel-card__status-panel--connecting,
html.dark .channel-card__status-panel--connecting {
  border-color: rgba(251, 191, 36, 0.32);
  background: rgba(120, 53, 15, 0.32);
}

:root.dark .channel-card__status-panel--error,
[data-theme='dark'] .channel-card__status-panel--error,
html.dark .channel-card__status-panel--error {
  border-color: rgba(248, 113, 113, 0.32);
  background: rgba(127, 29, 29, 0.32);
}

:root.dark .channel-card__metric-label,
[data-theme='dark'] .channel-card__metric-label,
html.dark .channel-card__metric-label,
:root.dark .channel-card__field-label,
[data-theme='dark'] .channel-card__field-label,
html.dark .channel-card__field-label {
  color: #94a3b8;
}

:root.dark .channel-card__input,
[data-theme='dark'] .channel-card__input,
html.dark .channel-card__input {
  background: rgba(15, 23, 42, 0.78);
  color: #f8fafc;
  border-color: rgba(100, 116, 139, 0.58);
}

:root.dark .channel-card__password-field :deep(input),
[data-theme='dark'] .channel-card__password-field :deep(input),
html.dark .channel-card__password-field :deep(input),
:root.dark input.channel-card__password-field,
[data-theme='dark'] input.channel-card__password-field,
html.dark input.channel-card__password-field {
  background: rgba(15, 23, 42, 0.78);
  color: #f8fafc;
  border-color: rgba(100, 116, 139, 0.58);
}

:root.dark .channel-card__toggle-field-copy,
[data-theme='dark'] .channel-card__toggle-field-copy,
html.dark .channel-card__toggle-field-copy {
  color: #e2e8f0;
}

:root.dark .channel-card--connected .channel-card__icon-shell,
[data-theme='dark'] .channel-card--connected .channel-card__icon-shell,
html.dark .channel-card--connected .channel-card__icon-shell {
  border-color: rgba(74, 222, 128, 0.32);
  background: rgba(20, 83, 45, 0.2);
  box-shadow: none;
}

:root.dark .channel-card--connecting .channel-card__icon-shell,
[data-theme='dark'] .channel-card--connecting .channel-card__icon-shell,
html.dark .channel-card--connecting .channel-card__icon-shell {
  border-color: rgba(251, 191, 36, 0.32);
  background: rgba(120, 53, 15, 0.2);
  box-shadow: none;
}

:root.dark .channel-card--error .channel-card__icon-shell,
[data-theme='dark'] .channel-card--error .channel-card__icon-shell,
html.dark .channel-card--error .channel-card__icon-shell {
  border-color: rgba(248, 113, 113, 0.32);
  background: rgba(127, 29, 29, 0.2);
  box-shadow: none;
}

:root.dark .channel-card__feedback--success,
[data-theme='dark'] .channel-card__feedback--success,
html.dark .channel-card__feedback--success {
  background: rgba(20, 83, 45, 0.42);
  color: #bbf7d0;
  border-color: rgba(74, 222, 128, 0.32);
}

:root.dark .channel-card__feedback--error,
[data-theme='dark'] .channel-card__feedback--error,
html.dark .channel-card__feedback--error {
  background: rgba(127, 29, 29, 0.42);
  color: #fecaca;
  border-color: rgba(248, 113, 113, 0.32);
}

:root.dark .channel-card__action--secondary,
[data-theme='dark'] .channel-card__action--secondary,
html.dark .channel-card__action--secondary {
  background: rgba(30, 41, 59, 0.9);
  border-color: rgba(100, 116, 139, 0.58);
  color: #f8fafc;
}

:root.dark .channel-card__action--primary,
[data-theme='dark'] .channel-card__action--primary,
html.dark .channel-card__action--primary {
  border-color: #2563eb;
  background: #2563eb;
}
</style>
