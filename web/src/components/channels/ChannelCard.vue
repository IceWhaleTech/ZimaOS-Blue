<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
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
const channelIconStyle = computed(() => getChannelIconStyleVars(props.channel.id))
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

      <div class="channel-card__aside">
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
      </div>
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

.channel-card--expanded {
  border-color: rgba(15, 23, 42, 0.14);
  box-shadow: 0 18px 38px -30px rgba(15, 23, 42, 0.4);
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

.channel-card__aside {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  gap: 0.72rem;
}

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

@media (max-width: 639px) {
  .channel-card__header {
    align-items: flex-start;
    padding: 0.82rem 0.88rem;
  }

  .channel-card__identity {
    align-items: flex-start;
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

:root.dark .channel-card--expanded,
[data-theme='dark'] .channel-card--expanded,
html.dark .channel-card--expanded {
  border-color: rgba(148, 163, 184, 0.42);
  box-shadow: 0 18px 38px -30px rgba(15, 23, 42, 0.78);
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
html.dark .channel-card__title {
  color: #f8fafc;
}

:root.dark .channel-card__description,
[data-theme='dark'] .channel-card__description,
html.dark .channel-card__description {
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
