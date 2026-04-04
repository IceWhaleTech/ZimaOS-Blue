<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { getChannelIconStyleVars } from '@/utils/channelIcons'
import ChannelCardShell from '@/components/channels/ChannelCardShell.vue'

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
}>()

const emit = defineEmits<{
  toggle: []
  toggleEnabled: [enabled: boolean]
}>()

const { t, te } = useI18n()

function tr(key: string | undefined, fallback = ''): string {
  if (!key) return fallback
  return te(key) ? t(key) : fallback
}

const translatedChannel = computed(() => ({
  name: props.channel.nameKey
    ? tr(props.channel.nameKey, props.channel.name || '')
    : props.channel.name!,
  description: tr(props.channel.descriptionKey, ''),
}))

const descriptionError = computed(() => props.channel.status === 'error' && !!props.channel.lastError)
const descriptionText = computed(() =>
  descriptionError.value ? props.channel.lastError || '' : translatedChannel.value.description
)

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

const channelIconStyle = computed(() => getChannelIconStyleVars(props.channel.id))
</script>

<template>
  <ChannelCardShell
    class-prefix="channel-card"
    :title="translatedChannel.name"
    :description="descriptionText"
    :description-error="descriptionError"
    :status-tone="statusTone"
    :status-text="statusText"
    :status-title="statusTitle"
    :expanded="expanded"
    :icon-src="channel.icon"
    :icon-alt="translatedChannel.name"
    :icon-style="channelIconStyle"
    @header-click="emit('toggle')"
  >
    <template #actions>
      <label class="relative inline-flex items-center cursor-pointer" @click.stop>
        <input
          :checked="channel.enabled"
          type="checkbox"
          class="sr-only peer channel-card__toggle-input"
          :disabled="toggling"
          @change="emit('toggleEnabled', ($event.target as HTMLInputElement).checked)"
        />
        <div
          class="channel-card__toggle bg-gray-200 peer-focus:outline-none peer-focus:ring-2 peer-focus:ring-gray-900 dark:peer-focus:ring-gray-400 rounded-full peer dark:bg-slate-700 after:content-[''] after:absolute after:bg-white after:border-gray-300 after:border after:rounded-full after:transition-all dark:border-slate-500 peer-checked:bg-green-600 dark:peer-checked:bg-green-500 peer-disabled:opacity-50"
        ></div>
      </label>
      <svg
        xmlns="http://www.w3.org/2000/svg"
        class="channel-card__chevron"
        :class="{ 'channel-card__chevron--active': expanded }"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
        aria-hidden="true"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M9 5l7 7-7 7"
        />
      </svg>
    </template>
  </ChannelCardShell>
</template>

<style scoped>
.channel-card__toggle {
  width: 2.16rem;
  height: 1.16rem;
}

.channel-card__toggle::after {
  top: 2px;
  inset-inline-start: 2px;
  width: 0.82rem;
  height: 0.82rem;
}

.channel-card__toggle-input:checked + .channel-card__toggle::after {
  inset-inline-start: calc(100% - 0.82rem - 2px);
  border-color: #fff;
}

.channel-card__chevron {
  width: 0.92rem;
  height: 0.92rem;
  color: #94a3b8;
  transition:
    transform 160ms ease,
    color 160ms ease;
}

.channel-card__chevron--active {
  color: #0f172a;
  transform: translateX(2px);
}

:root.dark .channel-card__chevron,
[data-theme='dark'] .channel-card__chevron,
html.dark .channel-card__chevron {
  color: #64748b;
}

:root.dark .channel-card__chevron--active,
[data-theme='dark'] .channel-card__chevron--active,
html.dark .channel-card__chevron--active {
  color: #f8fafc;
}
</style>
