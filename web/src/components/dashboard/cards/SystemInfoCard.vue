<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useSystemStore } from '@/stores/system'

const { t } = useI18n()
const systemStore = useSystemStore()

function formatDate(dateStr: string | undefined): string {
  if (!dateStr) return '-'
  return new Date(dateStr).toLocaleString([], {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function formatCount(value: number | undefined): string {
  return typeof value === 'number' ? value.toLocaleString() : '-'
}

const workerPoolSize = computed(() => systemStore.workerStats?.pool_size)
const activeWorkers = computed(() => systemStore.workerStats?.running)

const workerUsagePercent = computed(() => {
  if (
    typeof workerPoolSize.value !== 'number' ||
    typeof activeWorkers.value !== 'number' ||
    workerPoolSize.value <= 0
  ) {
    return null
  }

  return Math.max(0, Math.min(100, Math.round((activeWorkers.value / workerPoolSize.value) * 100)))
})

const workerUsageWidth = computed(() =>
  workerUsagePercent.value == null ? '0%' : `${workerUsagePercent.value}%`
)

const systemInfoItems = computed(() => [
  {
    key: 'go',
    label: t('system.goVersion'),
    value: systemStore.health?.go_version || '-',
  },
  {
    key: 'updated',
    label: t('system.timestamp'),
    value: formatDate(systemStore.health?.timestamp),
  },
  {
    key: 'active',
    label: t('common.active'),
    value: formatCount(activeWorkers.value),
  },
  {
    key: 'total',
    label: t('common.total'),
    value: formatCount(workerPoolSize.value),
  },
])
</script>

<template>
  <div class="dashboard-card-surface system-info-card p-5">
    <div class="dashboard-card-stack">
      <div class="dashboard-card-footer">
        <div class="dashboard-card-copy">
          <p class="dashboard-card-label">Snapshot</p>
          <p class="dashboard-card-subtitle mt-2">Runtime release and worker pool context</p>
        </div>
        <span class="dashboard-card-chip system-info-chip">Runtime</span>
      </div>

      <div class="system-info-grid">
        <div class="dashboard-card-subsurface system-info-release-panel p-4">
          <p class="system-info-release-label">Deployment</p>
          <p class="system-info-version">v{{ systemStore.health?.version || '-' }}</p>
          <p class="system-info-release-copy">Current deployed service version</p>
          <p class="dashboard-card-footnote mt-3">
            {{ t('system.timestamp') }} {{ formatDate(systemStore.health?.timestamp) }}
          </p>
        </div>

        <div class="system-info-metrics">
          <div
            v-for="item in systemInfoItems"
            :key="item.key"
            class="dashboard-card-subsurface system-info-stat p-3"
          >
            <p class="system-info-stat-label">{{ item.label }}</p>
            <p class="system-info-stat-value">{{ item.value }}</p>
          </div>
        </div>
      </div>

      <div class="dashboard-card-subsurface system-info-worker-panel p-4">
        <div class="system-info-worker-head">
          <div class="dashboard-card-copy">
            <p class="system-info-worker-label">Worker Pool</p>
            <p class="system-info-worker-title">
              {{ formatCount(activeWorkers) }} / {{ formatCount(workerPoolSize) }} running
            </p>
          </div>
          <span class="system-info-worker-pill">
            {{ workerUsagePercent == null ? '--' : `${workerUsagePercent}%` }}
          </span>
        </div>

        <div class="dashboard-card-progress system-info-worker-progress">
          <span :style="{ width: workerUsageWidth }"></span>
        </div>

        <p class="dashboard-card-footnote">Running workers against configured runtime capacity</p>
      </div>
    </div>
  </div>
</template>

<style scoped>
.system-info-card {
  border-color: rgba(191, 219, 254, 0.72);
  background:
    radial-gradient(circle at 100% 0%, rgba(219, 234, 254, 0.3), transparent 44%),
    radial-gradient(circle at 0% 100%, rgba(226, 232, 240, 0.28), transparent 42%),
    linear-gradient(180deg, #fbfdff 0%, #f2f6fb 100%);
}

.system-info-chip {
  min-width: 4.6rem;
}

.system-info-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.08fr) repeat(2, minmax(0, 1fr));
  gap: 0.78rem;
}

.system-info-release-panel {
  display: flex;
  flex-direction: column;
  justify-content: flex-end;
  min-height: 11rem;
  background:
    radial-gradient(circle at 100% 0%, rgba(219, 234, 254, 0.42), transparent 44%),
    linear-gradient(180deg, rgba(255, 255, 255, 0.96) 0%, rgba(247, 250, 252, 0.98) 100%);
}

.system-info-metrics {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.78rem;
}

.system-info-release-label {
  margin: 0;
  font-size: 0.66rem;
  line-height: 1;
  letter-spacing: 0.1em;
  text-transform: uppercase;
  font-weight: 700;
  color: #64748b;
}

.system-info-release-copy {
  margin: 0.58rem 0 0;
  font-size: 0.9rem;
  line-height: 1.45;
  color: #334155;
  font-weight: 600;
}

.system-info-version {
  margin: 0.46rem 0 0;
  font-size: clamp(2.1rem, 1.6vw + 1rem, 3rem);
  line-height: 0.96;
  letter-spacing: -0.04em;
  font-weight: 650;
  color: #0f172a;
}

.system-info-stat-label,
.system-info-stat-value {
  margin: 0;
}

.system-info-stat-label {
  font-size: 0.64rem;
  line-height: 1.35;
  color: #64748b;
  text-transform: uppercase;
  letter-spacing: 0.1em;
  font-weight: 700;
}

.system-info-stat-value {
  margin: 0.54rem 0 0;
  font-size: 1rem;
  line-height: 1.28;
  font-weight: 650;
  color: #0f172a;
  word-break: break-word;
}

.system-info-stat {
  min-height: 5.2rem;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
}

.system-info-worker-panel {
  display: flex;
  flex-direction: column;
  gap: 0.78rem;
  background:
    radial-gradient(circle at 100% 0%, rgba(255, 255, 255, 0.42), transparent 44%),
    linear-gradient(180deg, rgba(255, 255, 255, 0.94) 0%, rgba(245, 248, 252, 0.98) 100%);
}

.system-info-worker-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.82rem;
}

.system-info-worker-label {
  margin: 0;
  font-size: 0.64rem;
  line-height: 1.2;
  letter-spacing: 0.1em;
  text-transform: uppercase;
  font-weight: 700;
  color: #64748b;
}

.system-info-worker-title {
  margin: 0.48rem 0 0;
  font-size: 1.04rem;
  line-height: 1.2;
  font-weight: 650;
  color: #0f172a;
}

.system-info-worker-pill {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 3rem;
  min-height: 1.8rem;
  padding: 0 0.62rem;
  border-radius: 999px;
  border: 1px solid rgba(147, 197, 253, 0.68);
  background: rgba(219, 234, 254, 0.72);
  color: #1d4ed8;
  font-size: 0.76rem;
  font-weight: 700;
}

.system-info-worker-progress > span {
  background: linear-gradient(90deg, #2563eb 0%, #60a5fa 100%);
}

:root.dark .system-info-card,
[data-theme='dark'] .system-info-card,
html.dark .system-info-card {
  border-color: rgba(100, 116, 139, 0.56);
  background:
    radial-gradient(circle at 100% 0%, rgba(37, 99, 235, 0.16), transparent 44%),
    radial-gradient(circle at 0% 100%, rgba(51, 65, 85, 0.22), transparent 42%),
    linear-gradient(180deg, #28384c 0%, #233245 100%);
}

:root.dark .system-info-release-panel,
[data-theme='dark'] .system-info-release-panel,
html.dark .system-info-release-panel {
  background:
    radial-gradient(circle at 100% 0%, rgba(37, 99, 235, 0.18), transparent 44%),
    linear-gradient(180deg, rgba(39, 54, 74, 0.96) 0%, rgba(31, 43, 61, 0.98) 100%);
  border-color: rgba(100, 116, 139, 0.5);
}

:root.dark .system-info-worker-panel,
[data-theme='dark'] .system-info-worker-panel,
html.dark .system-info-worker-panel {
  background:
    radial-gradient(circle at 100% 0%, rgba(37, 99, 235, 0.12), transparent 44%),
    linear-gradient(180deg, rgba(37, 51, 71, 0.96) 0%, rgba(29, 40, 57, 0.98) 100%);
  border-color: rgba(100, 116, 139, 0.5);
}

:root.dark .system-info-version,
[data-theme='dark'] .system-info-version,
html.dark .system-info-version,
:root.dark .system-info-stat-value,
[data-theme='dark'] .system-info-stat-value,
html.dark .system-info-stat-value,
:root.dark .system-info-worker-title,
[data-theme='dark'] .system-info-worker-title,
html.dark .system-info-worker-title {
  color: rgb(241 245 249);
}

:root.dark .system-info-release-copy,
[data-theme='dark'] .system-info-release-copy,
html.dark .system-info-release-copy {
  color: rgb(203 213 225);
}

:root.dark .system-info-release-label,
[data-theme='dark'] .system-info-release-label,
html.dark .system-info-release-label,
:root.dark .system-info-stat-label,
[data-theme='dark'] .system-info-stat-label,
html.dark .system-info-stat-label,
:root.dark .system-info-worker-label,
[data-theme='dark'] .system-info-worker-label,
html.dark .system-info-worker-label {
  color: rgb(148 163 184);
}

:root.dark .system-info-worker-pill,
[data-theme='dark'] .system-info-worker-pill,
html.dark .system-info-worker-pill {
  color: rgb(191 219 254);
  border-color: rgba(96, 165, 250, 0.32);
  background: rgba(30, 64, 175, 0.2);
}

@media (max-width: 960px) {
  .system-info-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .system-info-release-panel {
    min-height: 9.6rem;
    grid-column: span 2 / span 2;
  }
}

@media (max-width: 640px) {
  .system-info-grid {
    grid-template-columns: 1fr;
  }

  .system-info-release-panel {
    grid-column: span 1 / span 1;
  }

  .system-info-metrics {
    grid-template-columns: 1fr 1fr;
  }

  .system-info-worker-head {
    flex-direction: column;
    align-items: stretch;
  }
}
</style>
