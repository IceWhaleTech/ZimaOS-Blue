<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useSystemStore } from '@/stores/system'

const props = withDefaults(
  defineProps<{
    compact?: boolean
  }>(),
  {
    compact: false,
  }
)

const { t } = useI18n()
const systemStore = useSystemStore()

const statusKey = computed(() => {
  const status = (systemStore.health?.status || '').toLowerCase()
  if (status === 'ok') return 'ok'
  if (status === 'degraded') return 'degraded'
  if (status === 'error') return 'error'
  return 'unknown'
})

const statusText = computed(() => {
  if (systemStore.loading) return '-'
  switch (statusKey.value) {
    case 'ok':
      return t('system.statusOk')
    case 'degraded':
      return t('system.statusDegraded')
    case 'error':
      return t('system.statusError')
    default:
      return t('common.unknown')
  }
})

const statusColorClass = computed(() => {
  switch (statusKey.value) {
    case 'ok':
      return 'text-green-600 dark:text-green-400'
    case 'degraded':
      return 'text-amber-600 dark:text-amber-400'
    case 'error':
      return 'text-red-600 dark:text-red-400'
    default:
      return 'text-gray-400 dark:text-gray-500'
  }
})

const statusDotClass = computed(() => {
  switch (statusKey.value) {
    case 'ok':
      return 'bg-green-500'
    case 'degraded':
      return 'bg-amber-500'
    case 'error':
      return 'bg-red-500'
    default:
      return 'bg-gray-400'
  }
})

const statusToneClass = computed(() => {
  switch (statusKey.value) {
    case 'ok':
      return 'is-ok'
    case 'degraded':
      return 'is-degraded'
    case 'error':
      return 'is-error'
    default:
      return 'is-unknown'
  }
})

const statusSummary = computed(() => {
  switch (statusKey.value) {
    case 'ok':
      return 'Core service responding normally'
    case 'degraded':
      return 'Service is up, but checks need attention'
    case 'error':
      return 'Health checks need immediate attention'
    default:
      return 'Waiting for runtime health samples'
  }
})

function formatUpdatedTime(value?: string): string {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

const statusMetaItems = computed(() => [
  {
    key: 'version',
    label: t('system.version'),
    value: systemStore.health?.version ? `v${systemStore.health.version}` : '-',
  },
  {
    key: 'updated',
    label: t('system.timestamp'),
    value: formatUpdatedTime(systemStore.health?.timestamp),
  },
])
</script>

<template>
  <div class="dashboard-card-stack" :class="{ 'status-card-compact-layout': props.compact }">
    <div class="dashboard-card-footer status-card-header">
      <div class="dashboard-card-copy min-w-0">
        <p class="dashboard-card-label">{{ t('common.status') }}</p>
        <p class="dashboard-card-subtitle mt-2">
          {{ props.compact ? 'Core service health' : 'Service runtime' }}
        </p>
      </div>
      <span class="dashboard-card-chip status-card-state-pill" :class="statusToneClass">
        {{ statusText }}
      </span>
    </div>

    <div class="status-card-grid" :class="{ 'is-compact': props.compact }">
      <div class="dashboard-card-subsurface status-card-summary-panel" :class="statusToneClass">
        <div class="status-card-summary-main">
          <span class="status-card-signal-shell" :class="statusToneClass">
            <span class="status-card-signal-core" :class="statusDotClass"></span>
          </span>

          <div class="dashboard-card-copy min-w-0">
            <p class="dashboard-card-value truncate" :class="statusColorClass">
              {{ statusText }}
            </p>
            <p class="dashboard-card-footnote mt-2">{{ statusSummary }}</p>
          </div>
        </div>
      </div>

      <div class="status-card-meta">
        <div
          v-for="item in statusMetaItems"
          :key="item.key"
          class="dashboard-card-subsurface status-card-meta-item"
        >
          <p class="status-card-meta-label">{{ item.label }}</p>
          <p class="status-card-meta-value">{{ item.value }}</p>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.status-card-header {
  align-items: center;
}

.status-card-compact-layout {
  gap: 0.8rem;
}

.status-card-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.45fr) minmax(14rem, 0.95fr);
  gap: 0.72rem;
  align-items: stretch;
}

.status-card-grid.is-compact {
  grid-template-columns: minmax(0, 1.4fr) minmax(13rem, 0.92fr);
  gap: 0.68rem;
}

.status-card-summary-panel {
  display: flex;
  align-items: center;
  min-height: 100%;
  padding: 0.88rem 0.92rem;
  border: 1px solid transparent;
}

.status-card-summary-panel.is-ok {
  border-color: rgba(134, 239, 172, 0.44);
  background: linear-gradient(180deg, rgba(240, 253, 244, 0.96) 0%, rgba(255, 255, 255, 0.96) 100%);
}

.status-card-summary-panel.is-degraded {
  border-color: rgba(253, 230, 138, 0.52);
  background: linear-gradient(180deg, rgba(255, 251, 235, 0.98) 0%, rgba(255, 255, 255, 0.96) 100%);
}

.status-card-summary-panel.is-error {
  border-color: rgba(252, 165, 165, 0.44);
  background: linear-gradient(180deg, rgba(254, 242, 242, 0.98) 0%, rgba(255, 255, 255, 0.96) 100%);
}

.status-card-summary-panel.is-unknown {
  border-color: rgba(203, 213, 225, 0.68);
  background: linear-gradient(180deg, rgba(248, 250, 252, 0.98) 0%, rgba(255, 255, 255, 0.96) 100%);
}

.status-card-summary-main {
  display: flex;
  align-items: center;
  gap: 0.82rem;
  width: 100%;
}

.status-card-signal-shell {
  width: 3rem;
  height: 3rem;
  border-radius: 999px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  background: rgba(255, 255, 255, 0.84);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.92);
}

.status-card-signal-shell.is-ok {
  color: #16a34a;
}

.status-card-signal-shell.is-degraded {
  color: #d97706;
}

.status-card-signal-shell.is-error {
  color: #dc2626;
}

.status-card-signal-shell.is-unknown {
  color: #64748b;
}

.status-card-signal-core {
  width: 0.92rem;
  height: 0.92rem;
  border-radius: 999px;
  box-shadow: 0 0 0 0.34rem rgba(255, 255, 255, 0.84);
}

.status-card-state-pill {
  min-width: 2.9rem;
}

.status-card-state-pill.is-ok {
  background: rgba(220, 252, 231, 0.86);
  color: #166534;
}

.status-card-state-pill.is-degraded {
  background: rgba(254, 243, 199, 0.92);
  color: #b45309;
}

.status-card-state-pill.is-error {
  background: rgba(254, 226, 226, 0.92);
  color: #b91c1c;
}

.status-card-state-pill.is-unknown {
  background: rgba(226, 232, 240, 0.86);
  color: #475569;
}

.status-card-meta {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.62rem;
}

.status-card-meta-item {
  min-height: 4.6rem;
  padding: 0.78rem 0.84rem;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
}

.status-card-meta-label,
.status-card-meta-value {
  margin: 0;
}

.status-card-compact-layout .status-card-summary-panel {
  padding: 0.82rem 0.88rem;
}

.status-card-compact-layout .status-card-signal-shell {
  width: 2.75rem;
  height: 2.75rem;
}

.status-card-compact-layout .dashboard-card-value {
  font-size: clamp(1.4rem, 0.95vw + 0.7rem, 1.85rem);
}

.status-card-compact-layout .status-card-meta {
  gap: 0.5rem;
}

.status-card-compact-layout .status-card-meta-item {
  min-height: 0;
  padding: 0.68rem 0.76rem;
}

@media (max-width: 860px) {
  .status-card-grid,
  .status-card-grid.is-compact {
    grid-template-columns: minmax(0, 1fr);
  }
}

.status-card-meta-label {
  font-size: 0.62rem;
  line-height: 1.2;
  letter-spacing: 0.1em;
  text-transform: uppercase;
  font-weight: 700;
  color: #64748b;
}

.status-card-meta-value {
  margin-top: 0.4rem;
  font-size: 1rem;
  line-height: 1.2;
  font-weight: 650;
  color: #0f172a;
  overflow-wrap: anywhere;
}

:root.dark .status-card-summary-panel.is-ok,
[data-theme='dark'] .status-card-summary-panel.is-ok,
html.dark .status-card-summary-panel.is-ok {
  border-color: rgba(34, 197, 94, 0.2);
  background: linear-gradient(180deg, rgba(20, 83, 45, 0.2) 0%, rgba(15, 23, 42, 0.56) 100%);
}

:root.dark .status-card-summary-panel.is-degraded,
[data-theme='dark'] .status-card-summary-panel.is-degraded,
html.dark .status-card-summary-panel.is-degraded {
  border-color: rgba(245, 158, 11, 0.22);
  background: linear-gradient(180deg, rgba(120, 53, 15, 0.2) 0%, rgba(15, 23, 42, 0.56) 100%);
}

:root.dark .status-card-summary-panel.is-error,
[data-theme='dark'] .status-card-summary-panel.is-error,
html.dark .status-card-summary-panel.is-error {
  border-color: rgba(248, 113, 113, 0.22);
  background: linear-gradient(180deg, rgba(127, 29, 29, 0.22) 0%, rgba(15, 23, 42, 0.56) 100%);
}

:root.dark .status-card-summary-panel.is-unknown,
[data-theme='dark'] .status-card-summary-panel.is-unknown,
html.dark .status-card-summary-panel.is-unknown {
  border-color: rgba(71, 85, 105, 0.28);
  background: linear-gradient(180deg, rgba(51, 65, 85, 0.26) 0%, rgba(15, 23, 42, 0.56) 100%);
}

:root.dark .status-card-signal-shell,
[data-theme='dark'] .status-card-signal-shell,
html.dark .status-card-signal-shell {
  background: rgba(15, 23, 42, 0.48);
  box-shadow: inset 0 1px 0 rgba(148, 163, 184, 0.06);
}

:root.dark .status-card-meta-label,
[data-theme='dark'] .status-card-meta-label,
html.dark .status-card-meta-label {
  color: rgb(148 163 184);
}

:root.dark .status-card-meta-value,
[data-theme='dark'] .status-card-meta-value,
html.dark .status-card-meta-value {
  color: rgb(241 245 249);
}

:root.dark .status-card-state-pill.is-ok,
[data-theme='dark'] .status-card-state-pill.is-ok,
html.dark .status-card-state-pill.is-ok {
  background: rgba(20, 83, 45, 0.52);
  color: #bbf7d0;
}

:root.dark .status-card-state-pill.is-degraded,
[data-theme='dark'] .status-card-state-pill.is-degraded,
html.dark .status-card-state-pill.is-degraded {
  background: rgba(120, 53, 15, 0.52);
  color: #fde68a;
}

:root.dark .status-card-state-pill.is-error,
[data-theme='dark'] .status-card-state-pill.is-error,
html.dark .status-card-state-pill.is-error {
  background: rgba(127, 29, 29, 0.54);
  color: #fecaca;
}

:root.dark .status-card-state-pill.is-unknown,
[data-theme='dark'] .status-card-state-pill.is-unknown,
html.dark .status-card-state-pill.is-unknown {
  background: rgba(51, 65, 85, 0.56);
  color: #cbd5e1;
}
</style>
