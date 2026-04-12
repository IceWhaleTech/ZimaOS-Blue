<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { TunnelProvider, TunnelStatus as TunnelStatusType } from '@/api/remote-access'
import { getTunnelProviderIcon } from '@/utils/channelIcons'
import { publicAsset } from '@/utils/publicAsset'
import TunnelStatus from '@/components/remote-access/TunnelStatus.vue'

type RemoteAccessState = 'loading' | 'ready' | 'connecting' | 'connected' | 'error'

const props = defineProps<{
  state: RemoteAccessState
  errorMessage: string | null
  tunnelStatus: TunnelStatusType | null
  tunnelProviders: TunnelProvider[]
  selectedProvider: string
  selectedProviderInfo: TunnelProvider | null
  currentProviderToken: string
  ngrokDomain: string
}>()

const emit = defineEmits<{
  selectProvider: [providerId: string]
  updateProviderToken: [value: string]
  updateNgrokDomain: [value: string]
  start: []
  stop: []
  retry: []
}>()

const { t } = useI18n()

const statusTone = computed(() => {
  switch (props.state) {
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

const statusText = computed(() => {
  switch (props.state) {
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

const descriptionText = computed(() => {
  if (props.state === 'error' && props.errorMessage) {
    return props.errorMessage
  }
  return t('remoteAccess.channelDescription')
})

const showToggle = computed(() => props.state === 'connected' || props.state === 'ready')

const canStart = computed(() => {
  if (!props.selectedProviderInfo?.requires_key) return true
  return Boolean(props.currentProviderToken)
})

const detailClasses = computed(() => [
  'remote-access-detail',
  'dashboard-card-surface',
  `remote-access-detail--${statusTone.value}`,
])

const statusBadgeClass = computed(() => `remote-access-detail__status-badge--${statusTone.value}`)
</script>

<template>
  <article :class="detailClasses">
    <div class="remote-access-detail__header">
      <div class="remote-access-detail__identity">
        <div class="remote-access-detail__icon-shell">
          <img
            :src="publicAsset('icons/tunnel/remote-access.svg')"
            :alt="t('remoteAccess.title')"
            class="remote-access-detail__icon"
          />
        </div>
        <div class="remote-access-detail__copy">
          <div class="remote-access-detail__title-row">
            <h3 class="remote-access-detail__title">{{ t('remoteAccess.title') }}</h3>
            <span class="remote-access-detail__status-badge" :class="statusBadgeClass">
              <span class="remote-access-detail__status-dot"></span>
              {{ statusText }}
            </span>
            <span class="remote-access-detail__badge">
              {{ t('remoteAccess.recommended') }}
            </span>
          </div>
          <p
            class="remote-access-detail__description"
            :class="{
              'remote-access-detail__description--error':
                props.state === 'error' && props.errorMessage,
            }"
          >
            {{ descriptionText }}
          </p>
        </div>
      </div>

      <label
        v-if="showToggle"
        class="relative inline-flex items-center cursor-pointer"
        @click.stop.prevent="props.state === 'connected' ? emit('stop') : emit('start')"
      >
        <input :checked="props.state === 'connected'" type="checkbox" class="sr-only peer" />
        <div
          class="remote-access-detail__toggle bg-gray-200 peer-focus:outline-none peer-focus:ring-2 peer-focus:ring-gray-900 dark:peer-focus:ring-gray-400 rounded-full peer dark:bg-slate-700 peer-checked:bg-green-600 dark:peer-checked:bg-green-500"
        ></div>
      </label>
    </div>

    <div class="remote-access-detail__body">
      <div v-if="props.state === 'loading'" class="flex items-center justify-center py-10">
        <svg
          class="animate-spin h-8 w-8 text-gray-900 dark:text-white"
          fill="none"
          viewBox="0 0 24 24"
        >
          <circle
            class="opacity-25"
            cx="12"
            cy="12"
            r="10"
            stroke="currentColor"
            stroke-width="4"
          />
          <path
            class="opacity-75"
            fill="currentColor"
            d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
          />
        </svg>
      </div>

      <div v-else-if="props.state === 'ready'" class="space-y-4">
        <div class="space-y-3">
          <label class="remote-access-detail__label">
            {{ t('remoteAccess.selectProvider') }}
          </label>
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
            <button
              v-for="provider in props.tunnelProviders"
              :key="provider.id"
              type="button"
              class="remote-access-detail__provider-option"
              :class="
                props.selectedProvider === provider.id
                  ? 'remote-access-detail__provider-option--selected'
                  : ''
              "
              @click="emit('selectProvider', provider.id)"
            >
              <img
                v-if="getTunnelProviderIcon(provider.id)"
                :src="getTunnelProviderIcon(provider.id)"
                :alt="provider.name"
                class="remote-access-detail__provider-icon"
              />
              <div
                v-else
                class="remote-access-detail__provider-icon remote-access-detail__provider-icon--fallback"
              >
                <svg fill="none" viewBox="0 0 24 24" stroke="currentColor" aria-hidden="true">
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9m-9 9a9 9 0 019-9"
                  />
                </svg>
              </div>
              <div class="min-w-0 flex-1">
                <div class="remote-access-detail__provider-name">{{ provider.name }}</div>
                <div class="remote-access-detail__provider-meta">
                  {{
                    provider.requires_key
                      ? t('remoteAccess.requiresKey')
                      : t('remoteAccess.noKeyRequired')
                  }}
                </div>
              </div>
            </button>
          </div>
        </div>

        <div v-if="props.selectedProviderInfo?.requires_key" class="space-y-2">
          <label class="remote-access-detail__label">
            {{ props.selectedProviderInfo.key_label || 'Auth Token' }}
          </label>
          <input
            :value="props.currentProviderToken"
            type="password"
            class="remote-access-detail__input"
            :placeholder="props.selectedProviderInfo.key_hint || ''"
            @input="emit('updateProviderToken', ($event.target as HTMLInputElement).value)"
          />
          <a
            v-if="props.selectedProviderInfo.doc_url"
            :href="props.selectedProviderInfo.doc_url"
            target="_blank"
            rel="noreferrer"
            class="remote-access-detail__helper-link"
          >
            <svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"
              />
            </svg>
            {{ t('remoteAccess.getToken') }}
          </a>
        </div>

        <div v-if="props.selectedProvider === 'ngrok'" class="space-y-2">
          <label class="remote-access-detail__label">
            {{ t('remoteAccess.ngrokDomain') }}
            <span class="remote-access-detail__optional-note">({{ t('common.optional') }})</span>
          </label>
          <input
            :value="props.ngrokDomain"
            type="text"
            class="remote-access-detail__input"
            :placeholder="t('remoteAccess.ngrokDomainPlaceholder')"
            @input="emit('updateNgrokDomain', ($event.target as HTMLInputElement).value)"
          />
          <p class="remote-access-detail__note">
            {{ t('remoteAccess.ngrokDomainHint') }}
            <a
              href="https://dashboard.ngrok.com/domains"
              target="_blank"
              rel="noreferrer"
              class="remote-access-detail__helper-link"
            >
              {{ t('remoteAccess.ngrokClaimDomain') }}
            </a>
          </p>
        </div>

        <button
          type="button"
          class="remote-access-detail__primary-action"
          :disabled="!canStart"
          :class="{ 'opacity-50 cursor-not-allowed': !canStart }"
          @click="emit('start')"
        >
          <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M13 10V3L4 14h7v7l9-11h-7z"
            />
          </svg>
          {{ t('remoteAccess.enable') }}
        </button>

        <p class="remote-access-detail__note remote-access-detail__note--centered">
          {{ t('remoteAccess.securityWarning') }}
        </p>
      </div>

      <div v-else-if="props.state === 'connecting'" class="space-y-4">
        <div class="flex items-center justify-center py-4">
          <div class="text-center">
            <svg
              class="animate-spin h-8 w-8 text-gray-900 dark:text-white mx-auto mb-4"
              fill="none"
              viewBox="0 0 24 24"
            >
              <circle
                class="opacity-25"
                cx="12"
                cy="12"
                r="10"
                stroke="currentColor"
                stroke-width="4"
              />
              <path
                class="opacity-75"
                fill="currentColor"
                d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
              />
            </svg>
            <p class="text-gray-600 dark:text-slate-300">
              {{ t('remoteAccess.connecting') }}
            </p>
          </div>
        </div>

        <TunnelStatus
          :status="props.tunnelStatus || { active: false, connecting: true }"
          @disconnect="emit('stop')"
        />
      </div>

      <div v-else-if="props.state === 'connected'" class="space-y-4">
        <TunnelStatus
          :status="props.tunnelStatus || { active: true, provider: props.selectedProvider }"
          @disconnect="emit('stop')"
        />
      </div>

      <div v-else-if="props.state === 'error'" class="space-y-4">
        <div class="remote-access-detail__error">
          <div class="flex items-start gap-3">
            <svg
              class="h-5 w-5 text-red-600 dark:text-red-400 mt-0.5"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
            <p class="text-sm text-red-800 dark:text-red-200">
              {{ props.errorMessage }}
            </p>
          </div>
        </div>

        <button type="button" class="remote-access-detail__secondary-action" @click="emit('retry')">
          {{ t('common.retry') }}
        </button>
        <TunnelStatus
          :status="props.tunnelStatus || { active: false }"
          @disconnect="emit('stop')"
        />
      </div>
    </div>
  </article>
</template>

<style scoped>
.remote-access-detail {
  position: relative;
  overflow: hidden;
  border: 1px solid rgba(226, 232, 240, 0.96);
  border-radius: 1.25rem;
  background: #ffffff;
  color: #0f172a;
}

.remote-access-detail::before {
  content: '';
  position: absolute;
  inset: 0 0 auto 0;
  height: 2px;
  opacity: 1;
}

.remote-access-detail--connected::before {
  background: rgba(22, 163, 74, 0.9);
}

.remote-access-detail--connecting::before {
  background: rgba(245, 158, 11, 0.92);
}

.remote-access-detail--error::before {
  background: rgba(239, 68, 68, 0.92);
}

.remote-access-detail--disconnected::before {
  background: rgba(148, 163, 184, 0.4);
}

.remote-access-detail__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 1rem 1.05rem;
}

.remote-access-detail__identity {
  min-width: 0;
  flex: 1;
  display: flex;
  align-items: center;
  gap: 0.82rem;
}

.remote-access-detail__icon-shell {
  width: 2.55rem;
  height: 2.55rem;
  flex-shrink: 0;
  display: grid;
  place-items: center;
  border-radius: 0.82rem;
  border: 1px solid rgba(226, 232, 240, 0.96);
  background: #f8fafc;
}

.remote-access-detail__icon {
  width: 1.56rem;
  height: 1.56rem;
  object-fit: contain;
}

.remote-access-detail__copy {
  min-width: 0;
  flex: 1;
}

.remote-access-detail__title-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.42rem;
}

.remote-access-detail__title {
  font-size: 0.9rem;
  font-weight: 700;
  color: #0f172a;
}

.remote-access-detail__description {
  margin-top: 0.28rem;
  font-size: 0.76rem;
  line-height: 1.45;
  color: #64748b;
}

.remote-access-detail__description--error {
  color: #b91c1c;
}

.remote-access-detail__status-badge,
.remote-access-detail__badge {
  display: inline-flex;
  align-items: center;
  gap: 0.26rem;
  min-height: 1.24rem;
  padding: 0.14rem 0.42rem;
  border-radius: 999px;
  font-size: 0.58rem;
  font-weight: 700;
  line-height: 1;
}

.remote-access-detail__status-badge {
  border: 1px solid transparent;
}

.remote-access-detail__status-dot {
  width: 0.24rem;
  height: 0.24rem;
  border-radius: 999px;
  background: currentColor;
}

.remote-access-detail__status-badge--connected {
  color: #166534;
  background: rgba(220, 252, 231, 0.95);
  border-color: rgba(134, 239, 172, 0.72);
}

.remote-access-detail__status-badge--connecting {
  color: #92400e;
  background: rgba(254, 243, 199, 0.95);
  border-color: rgba(252, 211, 77, 0.72);
}

.remote-access-detail__status-badge--error {
  color: #b91c1c;
  background: rgba(254, 226, 226, 0.95);
  border-color: rgba(252, 165, 165, 0.72);
}

.remote-access-detail__status-badge--disconnected {
  color: #475569;
  background: rgba(226, 232, 240, 0.92);
  border-color: rgba(203, 213, 225, 0.85);
}

.remote-access-detail__badge {
  background: rgba(241, 245, 249, 0.96);
  color: #0f172a;
}

.remote-access-detail__toggle {
  width: 2.16rem;
  height: 1.16rem;
  position: relative;
}

.remote-access-detail__toggle::after {
  top: 2px;
  inset-inline-start: 2px;
  width: 0.82rem;
  height: 0.82rem;
  content: '';
  position: absolute;
  border-radius: 9999px;
  background: #ffffff;
  border: 1px solid #d1d5db;
  transition: transform 150ms ease-in-out;
}

.peer:checked + .remote-access-detail__toggle::after {
  transform: translateX(100%);
  border-color: #ffffff;
}

:global(html[dir='rtl']) .peer:checked + .remote-access-detail__toggle::after {
  transform: translateX(-100%);
}

.remote-access-detail__body {
  border-top: 1px solid rgba(226, 232, 240, 0.96);
  padding: 1rem 1.05rem 1.08rem;
  background: #f8fafc;
}

.remote-access-detail__label {
  display: block;
  font-size: 0.7rem;
  font-weight: 700;
  color: #334155;
}

.remote-access-detail__provider-option {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.7rem 0.78rem;
  border: 1px solid rgba(203, 213, 225, 0.92);
  border-radius: 0.9rem;
  background: rgba(255, 255, 255, 0.96);
  text-align: start;
  transition:
    border-color 160ms ease,
    background-color 160ms ease,
    transform 160ms ease;
}

.remote-access-detail__provider-option:hover {
  transform: translateY(-1px);
  border-color: rgba(148, 163, 184, 0.96);
}

.remote-access-detail__provider-option--selected {
  border-color: rgba(15, 23, 42, 0.34);
  background: rgba(241, 245, 249, 0.96);
}

.remote-access-detail__provider-icon {
  width: 1.5rem;
  height: 1.5rem;
  flex-shrink: 0;
  border-radius: 0.4rem;
  object-fit: contain;
}

.remote-access-detail__provider-icon--fallback {
  display: grid;
  place-items: center;
  background: #0f172a;
  color: #94a3b8;
}

.remote-access-detail__provider-icon--fallback svg {
  width: 0.95rem;
  height: 0.95rem;
}

.remote-access-detail__provider-name {
  font-size: 0.76rem;
  font-weight: 700;
  color: #0f172a;
}

.remote-access-detail__provider-meta {
  margin-top: 0.16rem;
  font-size: 0.64rem;
  line-height: 1.4;
  color: #64748b;
}

.remote-access-detail__input {
  width: 100%;
  min-height: 2.18rem;
  padding: 0.46rem 0.62rem;
  border: 1px solid rgba(203, 213, 225, 0.96);
  border-radius: 0.72rem;
  background: #ffffff;
  color: #0f172a;
  font-size: 0.74rem;
}

.remote-access-detail__input:focus {
  outline: 2px solid rgba(15, 23, 42, 0.18);
  outline-offset: 2px;
  border-color: rgba(15, 23, 42, 0.28);
}

.remote-access-detail__helper-link {
  display: inline-flex;
  align-items: center;
  gap: 0.28rem;
  font-size: 0.64rem;
  font-weight: 600;
  color: #0f172a;
}

.remote-access-detail__helper-link:hover {
  text-decoration: underline;
}

.remote-access-detail__optional-note {
  margin-inline-start: 0.25rem;
  color: #94a3b8;
  font-size: 0.62rem;
  font-weight: 600;
}

.remote-access-detail__primary-action,
.remote-access-detail__secondary-action {
  width: 100%;
  min-height: 2.28rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.48rem;
  padding: 0.5rem 0.75rem;
  border-radius: 0.8rem;
  font-size: 0.74rem;
  font-weight: 700;
  transition: background-color 160ms ease;
}

.remote-access-detail__primary-action {
  border: 0;
  background: #111827;
  color: #ffffff;
}

.remote-access-detail__primary-action:hover {
  background: #030712;
}

.remote-access-detail__secondary-action {
  border: 0;
  background: #475569;
  color: #ffffff;
}

.remote-access-detail__secondary-action:hover {
  background: #334155;
}

.remote-access-detail__note {
  font-size: 0.64rem;
  line-height: 1.5;
  color: #64748b;
}

.remote-access-detail__note--centered {
  text-align: center;
}

.remote-access-detail__error {
  padding: 0.9rem;
  border-radius: 0.9rem;
  background: rgba(254, 242, 242, 0.96);
}

@media (max-width: 639px) {
  .remote-access-detail__header,
  .remote-access-detail__identity {
    align-items: flex-start;
  }

  .remote-access-detail__header {
    padding: 0.9rem;
  }

  .remote-access-detail__body {
    padding: 0.9rem;
  }
}

:root.dark .remote-access-detail,
[data-theme='dark'] .remote-access-detail,
html.dark .remote-access-detail {
  background: #111827;
  border-color: rgba(71, 85, 105, 0.46);
  color: #e2e8f0;
}

:root.dark .remote-access-detail__icon-shell,
[data-theme='dark'] .remote-access-detail__icon-shell,
html.dark .remote-access-detail__icon-shell {
  border-color: rgba(71, 85, 105, 0.46);
  background: rgba(15, 23, 42, 0.72);
}

:root.dark .remote-access-detail__title,
[data-theme='dark'] .remote-access-detail__title,
html.dark .remote-access-detail__title {
  color: #f8fafc;
}

:root.dark .remote-access-detail__description,
[data-theme='dark'] .remote-access-detail__description,
html.dark .remote-access-detail__description,
:root.dark .remote-access-detail__provider-meta,
[data-theme='dark'] .remote-access-detail__provider-meta,
html.dark .remote-access-detail__provider-meta,
:root.dark .remote-access-detail__note,
[data-theme='dark'] .remote-access-detail__note,
html.dark .remote-access-detail__note,
:root.dark .remote-access-detail__optional-note,
[data-theme='dark'] .remote-access-detail__optional-note,
html.dark .remote-access-detail__optional-note {
  color: #cbd5e1;
}

:root.dark .remote-access-detail__badge,
[data-theme='dark'] .remote-access-detail__badge,
html.dark .remote-access-detail__badge {
  background: rgba(30, 41, 59, 0.88);
  color: #f8fafc;
}

:root.dark .remote-access-detail__body,
[data-theme='dark'] .remote-access-detail__body,
html.dark .remote-access-detail__body {
  border-top-color: rgba(71, 85, 105, 0.46);
  background: #1e293b;
}

:root.dark .remote-access-detail__label,
[data-theme='dark'] .remote-access-detail__label,
html.dark .remote-access-detail__label,
:root.dark .remote-access-detail__provider-name,
[data-theme='dark'] .remote-access-detail__provider-name,
html.dark .remote-access-detail__provider-name,
:root.dark .remote-access-detail__helper-link,
[data-theme='dark'] .remote-access-detail__helper-link,
html.dark .remote-access-detail__helper-link {
  color: #f8fafc;
}

:root.dark .remote-access-detail__provider-option,
[data-theme='dark'] .remote-access-detail__provider-option,
html.dark .remote-access-detail__provider-option {
  border-color: rgba(71, 85, 105, 0.46);
  background: rgba(15, 23, 42, 0.74);
}

:root.dark .remote-access-detail__provider-option--selected,
[data-theme='dark'] .remote-access-detail__provider-option--selected,
html.dark .remote-access-detail__provider-option--selected {
  border-color: rgba(148, 163, 184, 0.76);
  background: rgba(30, 41, 59, 0.92);
}

:root.dark .remote-access-detail__input,
[data-theme='dark'] .remote-access-detail__input,
html.dark .remote-access-detail__input {
  border-color: rgba(71, 85, 105, 0.46);
  background: rgba(15, 23, 42, 0.74);
  color: #f8fafc;
}

:root.dark .remote-access-detail__input:focus,
[data-theme='dark'] .remote-access-detail__input:focus,
html.dark .remote-access-detail__input:focus {
  outline-color: rgba(148, 163, 184, 0.34);
  border-color: rgba(148, 163, 184, 0.56);
}

:root.dark .remote-access-detail__secondary-action,
[data-theme='dark'] .remote-access-detail__secondary-action,
html.dark .remote-access-detail__secondary-action {
  background: #475569;
}

:root.dark .remote-access-detail__secondary-action:hover,
[data-theme='dark'] .remote-access-detail__secondary-action:hover,
html.dark .remote-access-detail__secondary-action:hover {
  background: #64748b;
}

:root.dark .remote-access-detail__error,
[data-theme='dark'] .remote-access-detail__error,
html.dark .remote-access-detail__error {
  background: rgba(127, 29, 29, 0.24);
}
</style>
