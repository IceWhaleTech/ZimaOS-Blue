<script setup lang="ts">
import { onMounted, onUnmounted, shallowRef, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useSystemStore } from '@/stores/system'
import { useMetricsStore } from '@/stores/metrics'
import { systemApi } from '@/api/index'
import type { SystemMetrics } from '@/api/system'
import { ConfigurableDashboard, SystemStatusCard } from '@/components/dashboard'
import DashboardCustomizer from '@/components/dashboard/DashboardCustomizer.vue'
import UptimeCard from '@/components/dashboard/cards/UptimeCard.vue'
import CpuChartCard from '@/components/dashboard/cards/CpuChartCard.vue'
import MemoryUsageCard from '@/components/dashboard/cards/MemoryUsageCard.vue'
import GoroutinesCard from '@/components/dashboard/cards/GoroutinesCard.vue'

const { t } = useI18n()
const systemStore = useSystemStore()
const metricsStore = useMetricsStore()

const dashboardPrimaryCardIds = ['uptime', 'cpu-chart', 'memory-usage', 'goroutines']
const dashboardSecondaryExcludedCardIds = ['system-status', ...dashboardPrimaryCardIds]

let refreshInterval: ReturnType<typeof setInterval> | null = null
let abortController: AbortController | null = null

const metricsHistory = shallowRef<SystemMetrics[]>([])
const manualRefreshing = ref(false)
const isPageVisible = ref(true)

async function fetchMetricsHistory() {
  try {
    if (abortController) {
      abortController.abort()
    }

    abortController = new AbortController()
    const response = await systemApi.getMetricsHistory('5m')
    metricsHistory.value = response.data.metrics || []
  } catch (error: unknown) {
    const err = error as { name?: string }
    if (err?.name !== 'AbortError' && err?.name !== 'CanceledError') {
      metricsHistory.value = []
    }
  }
}

async function refreshAll(forceRefresh = false) {
  await Promise.all([
    systemStore.fetchAll(forceRefresh),
    metricsStore.fetchAll(),
    fetchMetricsHistory(),
  ])
}

async function refreshNow() {
  if (manualRefreshing.value) return
  manualRefreshing.value = true
  try {
    await refreshAll(true)
  } finally {
    manualRefreshing.value = false
  }
}

function handleVisibilityChange() {
  isPageVisible.value = !document.hidden
}

onMounted(async () => {
  await refreshAll()

  document.addEventListener('visibilitychange', handleVisibilityChange)

  refreshInterval = setInterval(() => {
    if (!isPageVisible.value) return
    void refreshAll()
  }, 15000)
})

onUnmounted(() => {
  if (refreshInterval) {
    clearInterval(refreshInterval)
    refreshInterval = null
  }

  if (abortController) {
    abortController.abort()
    abortController = null
  }

  document.removeEventListener('visibilitychange', handleVisibilityChange)
})
</script>

<template>
  <div class="home-page">
    <section class="dashboard-stage">
      <section class="dashboard-hero">
        <div class="dashboard-control-rail">
          <button
            class="dashboard-refresh-button"
            :disabled="manualRefreshing"
            :title="t('common.refresh')"
            :aria-label="t('common.refresh')"
            @click="refreshNow"
          >
            <svg
              class="h-4 w-4"
              :class="{ 'animate-spin': manualRefreshing }"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
              />
            </svg>
          </button>

          <div class="dashboard-customizer-compact">
            <DashboardCustomizer />
          </div>
        </div>

        <div class="dashboard-hero-copy">
          <h1 class="dashboard-welcome-title">{{ t('home.welcome') }}</h1>
          <p class="dashboard-description">{{ t('home.description') }}</p>
        </div>
      </section>

      <section class="dashboard-shell">
        <section class="dashboard-status-row">
          <div class="dashboard-card-surface dashboard-status-shell p-4">
            <SystemStatusCard compact />
          </div>
        </section>

        <section class="dashboard-primary-grid">
          <div class="dashboard-card-surface dashboard-small-card-shell p-4">
            <UptimeCard />
          </div>
          <div class="dashboard-card-surface dashboard-small-card-shell p-4">
            <CpuChartCard :metrics-history="metricsHistory" compact />
          </div>
          <div class="dashboard-card-surface dashboard-small-card-shell p-4">
            <MemoryUsageCard :metrics-history="metricsHistory" />
          </div>
          <div class="dashboard-card-surface dashboard-small-card-shell p-4">
            <GoroutinesCard :metrics-history="metricsHistory" />
          </div>
        </section>

        <ConfigurableDashboard
          :metrics-history="metricsHistory"
          :show-header="false"
          :disable-hero-layout="true"
          :exclude-card-ids="dashboardSecondaryExcludedCardIds"
        />
      </section>
    </section>
  </div>
</template>

<style scoped>
.home-page {
  max-width: 1480px;
  margin: 0 auto;
  padding: 0 0.75rem 1.8rem;
}

.dashboard-stage {
  position: relative;
  padding: 1.15rem 0 0.25rem;
}

.dashboard-stage::before,
.dashboard-stage::after {
  content: '';
  position: absolute;
  width: 19rem;
  height: 19rem;
  pointer-events: none;
  opacity: 0.8;
  background-image: radial-gradient(circle, rgba(37, 99, 235, 0.18) 1px, transparent 1px);
  background-size: 14px 14px;
}

.dashboard-stage::before {
  right: 18%;
  top: 10.5rem;
}

.dashboard-stage::after {
  left: 24%;
  bottom: -1.6rem;
}

.dashboard-hero {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 0.9rem;
  padding: 0.15rem 0 1.2rem;
}

.dashboard-control-rail {
  position: absolute;
  top: 0;
  right: 0;
  display: flex;
  align-items: center;
  gap: 0.5rem;
  z-index: 2;
}

.dashboard-hero-copy {
  max-width: 42rem;
  padding-top: 0.1rem;
}

.dashboard-welcome-title {
  margin: 0;
  font-size: clamp(1.34rem, 0.7vw + 0.95rem, 1.9rem);
  line-height: 1.06;
  letter-spacing: -0.04em;
  font-weight: 700;
  color: #111827;
}

.dashboard-description {
  margin: 0.42rem 0 0;
  max-width: 34rem;
  font-size: 0.92rem;
  line-height: 1.55;
  color: #9ca3af;
}

.dashboard-refresh-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 2.4rem;
  height: 2.4rem;
  border: 0;
  border-radius: 999px;
  color: #6b7280;
  background: rgba(255, 255, 255, 0.9);
  box-shadow: 0 18px 36px -32px rgba(15, 23, 42, 0.32);
  opacity: 0;
  pointer-events: none;
  transform: translateY(-4px);
  transition:
    opacity 0.18s ease,
    transform 0.18s ease,
    background-color 0.18s ease,
    color 0.18s ease;
}

.dashboard-control-rail:hover .dashboard-refresh-button,
.dashboard-refresh-button:focus-visible {
  opacity: 1;
  pointer-events: auto;
  transform: translateY(0);
}

.dashboard-refresh-button:hover {
  color: #1d4ed8;
  background: #fff;
}

.dashboard-refresh-button:disabled {
  cursor: not-allowed;
}

.dashboard-customizer-compact {
  display: inline-flex;
}

:deep(.dashboard-customizer-compact .dashboard-customize-trigger) {
  width: 2.4rem;
  min-width: 2.4rem;
  height: 2.4rem;
  padding: 0;
  border-radius: 999px;
  justify-content: center;
  gap: 0;
  border: 0;
  background: rgba(255, 255, 255, 0.92);
  color: #6b7280;
  box-shadow: 0 18px 36px -32px rgba(15, 23, 42, 0.32);
}

:deep(.dashboard-customizer-compact .dashboard-customize-trigger:hover) {
  background: #fff;
  color: #111827;
}

:deep(.dashboard-customizer-compact .dashboard-customize-trigger > span) {
  display: none;
}

.dashboard-shell {
  position: relative;
  z-index: 1;
}

.dashboard-status-row {
  margin-bottom: 0.78rem;
}

.dashboard-primary-grid {
  display: grid;
  gap: 0.88rem;
  margin-bottom: 0.88rem;
}

.dashboard-status-shell {
  min-height: 0;
  padding: 0.92rem 1rem;
}

:deep(.dashboard-shell .dashboard-grid-stack) {
  gap: 0.9rem;
}

:deep(.dashboard-shell .dashboard-grid-wrap),
:deep(.dashboard-shell .dashboard-grid-cluster),
:deep(.dashboard-shell .dashboard-grid-wrap-hero) {
  padding: 0;
  border: 0;
  background: transparent;
  box-shadow: none;
}

:deep(.dashboard-shell .dashboard-grid-wrap.with-divider) {
  padding-top: 0;
  border-top: 0;
}

:deep(.dashboard-shell .dashboard-grid),
:deep(.dashboard-shell .dashboard-grid-hero),
:deep(.dashboard-shell .dashboard-grid-small),
:deep(.dashboard-shell .dashboard-grid-featured),
:deep(.dashboard-shell .dashboard-grid-large) {
  gap: 0.88rem;
}

:deep(.dashboard-primary-grid .dashboard-card-stack) {
  justify-content: space-between;
  gap: 0.62rem;
}

:deep(.dashboard-primary-grid .dashboard-card-footer:last-child) {
  align-items: center;
  min-height: 3.7rem;
}

:deep(.dashboard-primary-grid .dashboard-card-label) {
  padding: 0;
  border-radius: 0;
  background: transparent;
  color: #a3a3a3;
  font-size: 0.65rem;
  font-weight: 500;
  letter-spacing: 0;
  text-transform: none;
}

:deep(.dashboard-primary-grid .dashboard-card-subtitle) {
  margin-top: 0.2rem;
  color: #111827;
  font-size: 0.74rem;
  font-weight: 650;
}

:deep(.dashboard-primary-grid .dashboard-card-footnote) {
  margin-top: 0.22rem;
  color: #9ca3af;
  font-size: 0.67rem;
  line-height: 1.3;
}

:deep(.dashboard-primary-grid .dashboard-card-value) {
  color: #111827;
  font-size: clamp(1.38rem, 0.72vw + 0.85rem, 1.72rem);
  line-height: 1.04;
}

:deep(.dashboard-primary-grid .dashboard-card-chip) {
  border: 0;
  background: rgba(15, 23, 42, 0.06);
  color: #6b7280;
  min-height: 1.1rem;
  padding: 0.12rem 0.4rem;
  font-size: 0.58rem;
}

:deep(.dashboard-primary-grid .cpu-ring-wrap) {
  width: 3.8rem;
  height: 3.8rem;
}

:deep(.dashboard-primary-grid .dashboard-mini-sparkline) {
  width: 4.4rem;
  height: 3.8rem;
}

:deep(.dashboard-primary-grid .dashboard-mini-bars) {
  width: 4.8rem;
  height: 3.3rem;
}

:deep(.dashboard-shell .dashboard-grid-small) {
  grid-template-columns: repeat(1, minmax(0, 1fr));
}

:deep(.dashboard-shell .dashboard-grid-item-small) {
  min-height: 8.55rem;
}

:deep(.dashboard-shell .dashboard-grid-item-featured) {
  min-height: 10.6rem;
}

:deep(.dashboard-shell .dashboard-small-card-shell) {
  min-height: 100%;
  padding: 0.88rem 0.95rem;
}

:deep(.dashboard-shell .dashboard-card-surface) {
  border: 1px solid rgba(255, 255, 255, 0.92);
  border-radius: 1.5rem;
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.98) 0%, rgba(248, 250, 252, 0.98) 100%);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.9),
    0 22px 36px -34px rgba(15, 23, 42, 0.2);
  transition:
    transform 0.2s ease,
    box-shadow 0.2s ease,
    border-color 0.2s ease,
    background-color 0.2s ease;
}

:deep(.dashboard-shell .dashboard-card-surface:hover) {
  border-color: rgba(255, 255, 255, 0.98);
  transform: translateY(-1px);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.94),
    0 24px 38px -34px rgba(15, 23, 42, 0.24);
}

:deep(.dashboard-shell .dashboard-card-subsurface) {
  border: 0;
  background: rgba(243, 246, 249, 0.92);
  box-shadow: none;
}

:deep(.dashboard-shell .dashboard-card-stack) {
  gap: 0.72rem;
}

:deep(.dashboard-shell .dashboard-card-footer) {
  gap: 0.6rem;
}

:deep(.dashboard-status-shell .dashboard-card-stack) {
  gap: 0.68rem;
}

:deep(.dashboard-shell .dashboard-card-label) {
  padding: 0;
  border-radius: 0;
  background: transparent;
  color: #a3a3a3;
  font-size: 0.68rem;
  font-weight: 500;
  letter-spacing: 0;
  text-transform: none;
}

:deep(.dashboard-shell .dashboard-card-chip) {
  border: 0;
  background: rgba(15, 23, 42, 0.06);
  color: #6b7280;
  min-height: 1.1rem;
  padding: 0.12rem 0.4rem;
  font-size: 0.6rem;
}

:deep(.dashboard-shell .dashboard-card-subtitle) {
  color: #111827;
  font-size: 0.78rem;
  font-weight: 650;
}

:deep(.dashboard-shell .dashboard-card-footnote) {
  color: #9ca3af;
  font-size: 0.72rem;
}

:deep(.dashboard-shell .dashboard-card-value) {
  color: #111827;
}

:deep(.dashboard-shell .dashboard-small-card-shell .dashboard-card-stack) {
  justify-content: space-between;
  gap: 0.62rem;
}

:deep(.dashboard-shell .dashboard-small-card-shell .dashboard-card-footer:last-child) {
  align-items: center;
  min-height: 3.7rem;
}

:deep(.dashboard-shell .dashboard-small-card-shell .dashboard-card-label) {
  font-size: 0.65rem;
}

:deep(.dashboard-shell .dashboard-small-card-shell .dashboard-card-subtitle) {
  margin-top: 0.2rem;
  font-size: 0.74rem;
}

:deep(.dashboard-shell .dashboard-small-card-shell .dashboard-card-value) {
  font-size: clamp(1.38rem, 0.72vw + 0.85rem, 1.72rem);
  line-height: 1.04;
}

:deep(.dashboard-shell .dashboard-small-card-shell .dashboard-card-footnote) {
  margin-top: 0.22rem;
  font-size: 0.67rem;
  line-height: 1.3;
}

:deep(.dashboard-shell .dashboard-small-card-shell .dashboard-card-chip) {
  font-size: 0.58rem;
}

:deep(.dashboard-shell .dashboard-small-card-shell .cpu-ring-wrap) {
  width: 3.8rem;
  height: 3.8rem;
}

:deep(.dashboard-shell .dashboard-small-card-shell .dashboard-mini-sparkline) {
  width: 4.4rem;
  height: 3.8rem;
}

:deep(.dashboard-shell .dashboard-small-card-shell .dashboard-mini-bars) {
  width: 4.8rem;
  height: 3.3rem;
}

:deep(.dashboard-shell .dashboard-hero-card .dashboard-card-value) {
  font-size: clamp(1.7rem, 0.9vw + 0.9rem, 2.05rem);
}

:deep(.dashboard-shell .dashboard-hero-card-featured .dashboard-card-value) {
  font-size: clamp(1.9rem, 1vw + 1rem, 2.3rem);
}

:deep(.dashboard-shell .dashboard-hero-card .dashboard-mini-bars) {
  width: 5.6rem;
  height: 3.7rem;
}

:deep(.dashboard-shell .dashboard-hero-card) {
  min-height: 9.8rem;
  padding: 0.95rem 1rem;
  border-radius: 1.5rem;
}

:deep(.dashboard-shell .dashboard-hero-card-featured) {
  min-height: 10rem;
  padding: 1rem 1.05rem;
  box-shadow: 0 22px 36px -34px rgba(15, 23, 42, 0.2);
}

:root.dark .dashboard-welcome-title,
[data-theme='dark'] .dashboard-welcome-title,
html.dark .dashboard-welcome-title {
  color: rgb(241 245 249);
}

:root.dark .dashboard-description,
[data-theme='dark'] .dashboard-description,
html.dark .dashboard-description {
  color: rgb(148 163 184);
}

:root.dark .dashboard-refresh-button,
[data-theme='dark'] .dashboard-refresh-button,
html.dark .dashboard-refresh-button {
  color: rgb(148 163 184);
  background: rgba(30, 41, 59, 0.88);
  box-shadow: 0 18px 36px -28px rgba(2, 6, 23, 0.72);
}

:root.dark :deep(.dashboard-customizer-compact .dashboard-customize-trigger),
[data-theme='dark'] :deep(.dashboard-customizer-compact .dashboard-customize-trigger),
html.dark :deep(.dashboard-customizer-compact .dashboard-customize-trigger) {
  color: rgb(148 163 184);
  background: rgba(30, 41, 59, 0.88);
  box-shadow: 0 18px 36px -28px rgba(2, 6, 23, 0.72);
}

:root.dark :deep(.dashboard-customizer-compact .dashboard-customize-trigger:hover),
[data-theme='dark'] :deep(.dashboard-customizer-compact .dashboard-customize-trigger:hover),
html.dark :deep(.dashboard-customizer-compact .dashboard-customize-trigger:hover) {
  color: rgb(226 232 240);
  background: rgba(51, 65, 85, 0.92);
}

:root.dark .dashboard-refresh-button:hover,
[data-theme='dark'] .dashboard-refresh-button:hover,
html.dark .dashboard-refresh-button:hover {
  color: rgb(191 219 254);
  background: rgba(51, 65, 85, 0.92);
}

:root.dark .dashboard-stage::before,
[data-theme='dark'] .dashboard-stage::before,
html.dark .dashboard-stage::before,
:root.dark .dashboard-stage::after,
[data-theme='dark'] .dashboard-stage::after,
html.dark .dashboard-stage::after {
  background-image: radial-gradient(circle, rgba(96, 165, 250, 0.22) 1px, transparent 1px);
  opacity: 0.56;
}

:root.dark :deep(.dashboard-shell .dashboard-card-surface),
[data-theme='dark'] :deep(.dashboard-shell .dashboard-card-surface),
html.dark :deep(.dashboard-shell .dashboard-card-surface) {
  border: 1px solid rgba(71, 85, 105, 0.42);
  background: linear-gradient(180deg, rgba(30, 41, 59, 0.9) 0%, rgba(15, 23, 42, 0.96) 100%);
  box-shadow:
    inset 0 1px 0 rgba(148, 163, 184, 0.06),
    0 24px 38px -34px rgba(2, 6, 23, 0.72);
}

:root.dark :deep(.dashboard-shell .dashboard-card-subsurface),
[data-theme='dark'] :deep(.dashboard-shell .dashboard-card-subsurface),
html.dark :deep(.dashboard-shell .dashboard-card-subsurface) {
  border: 1px solid rgba(71, 85, 105, 0.26);
  background: linear-gradient(180deg, rgba(30, 41, 59, 0.64) 0%, rgba(15, 23, 42, 0.76) 100%);
}

:root.dark :deep(.dashboard-shell .dashboard-card-label),
[data-theme='dark'] :deep(.dashboard-shell .dashboard-card-label),
html.dark :deep(.dashboard-shell .dashboard-card-label),
:root.dark :deep(.dashboard-shell .dashboard-card-footnote),
[data-theme='dark'] :deep(.dashboard-shell .dashboard-card-footnote),
html.dark :deep(.dashboard-shell .dashboard-card-footnote) {
  color: rgb(148 163 184);
}

:root.dark :deep(.dashboard-shell .dashboard-card-subtitle),
[data-theme='dark'] :deep(.dashboard-shell .dashboard-card-subtitle),
html.dark :deep(.dashboard-shell .dashboard-card-subtitle),
:root.dark :deep(.dashboard-shell .dashboard-card-value),
[data-theme='dark'] :deep(.dashboard-shell .dashboard-card-value),
html.dark :deep(.dashboard-shell .dashboard-card-value) {
  color: rgb(241 245 249);
}

:root.dark :deep(.dashboard-shell .dashboard-card-chip),
[data-theme='dark'] :deep(.dashboard-shell .dashboard-card-chip),
html.dark :deep(.dashboard-shell .dashboard-card-chip) {
  border: 0;
  background: rgba(148, 163, 184, 0.12);
  color: rgb(203 213 225);
}

:root.dark :deep(.dashboard-primary-grid .dashboard-card-label),
[data-theme='dark'] :deep(.dashboard-primary-grid .dashboard-card-label),
html.dark :deep(.dashboard-primary-grid .dashboard-card-label),
:root.dark :deep(.dashboard-primary-grid .dashboard-card-footnote),
[data-theme='dark'] :deep(.dashboard-primary-grid .dashboard-card-footnote),
html.dark :deep(.dashboard-primary-grid .dashboard-card-footnote) {
  color: rgb(148 163 184);
}

:root.dark :deep(.dashboard-primary-grid .dashboard-card-subtitle),
[data-theme='dark'] :deep(.dashboard-primary-grid .dashboard-card-subtitle),
html.dark :deep(.dashboard-primary-grid .dashboard-card-subtitle),
:root.dark :deep(.dashboard-primary-grid .dashboard-card-value),
[data-theme='dark'] :deep(.dashboard-primary-grid .dashboard-card-value),
html.dark :deep(.dashboard-primary-grid .dashboard-card-value) {
  color: rgb(241 245 249);
}

:root.dark :deep(.dashboard-primary-grid .dashboard-card-chip),
[data-theme='dark'] :deep(.dashboard-primary-grid .dashboard-card-chip),
html.dark :deep(.dashboard-primary-grid .dashboard-card-chip) {
  background: rgba(148, 163, 184, 0.12);
  color: rgb(203 213 225);
}

@media (max-width: 1100px) {
  .home-page {
    padding-top: 4rem;
  }
}

@media (min-width: 700px) {
  .dashboard-primary-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  :deep(.dashboard-shell .dashboard-grid-small) {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (min-width: 1100px) {
  .dashboard-primary-grid {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }

  :deep(.dashboard-shell .dashboard-grid-small) {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }
}

@media (max-width: 768px) {
  .home-page {
    padding-inline: 0;
    padding-bottom: 1rem;
  }

  .dashboard-stage {
    padding-top: 0.7rem;
  }

  .dashboard-stage::before,
  .dashboard-stage::after {
    width: 12rem;
    height: 12rem;
    background-size: 12px 12px;
  }

  .dashboard-stage::before {
    right: 0.2rem;
    top: 10rem;
  }

  .dashboard-stage::after {
    left: 0;
    bottom: 0.4rem;
  }

  .dashboard-hero {
    gap: 0.82rem;
    padding-bottom: 1rem;
  }

  .dashboard-control-rail {
    gap: 0.42rem;
  }

  .dashboard-welcome-title {
    font-size: 1.5rem;
  }

  .dashboard-description {
    font-size: 0.86rem;
  }

  .dashboard-refresh-button {
    opacity: 1;
    pointer-events: auto;
    transform: none;
  }

  :deep(.dashboard-shell .dashboard-grid),
  :deep(.dashboard-shell .dashboard-grid-hero),
  :deep(.dashboard-shell .dashboard-grid-small),
  :deep(.dashboard-shell .dashboard-grid-featured),
  :deep(.dashboard-shell .dashboard-grid-large) {
    gap: 0.78rem;
  }

  :deep(.dashboard-shell .dashboard-card-surface),
  :deep(.dashboard-shell .dashboard-hero-card) {
    border-radius: 1.3rem;
  }

  :deep(.dashboard-shell .dashboard-grid-item-small),
  :deep(.dashboard-shell .dashboard-hero-card) {
    min-height: 8.7rem;
  }

  :deep(.dashboard-shell .dashboard-grid-item-featured),
  :deep(.dashboard-shell .dashboard-hero-card-featured) {
    min-height: 9.6rem;
  }

  .dashboard-status-shell {
    padding: 0.88rem 0.92rem;
  }

  :deep(.dashboard-shell .dashboard-small-card-shell) {
    padding: 0.84rem 0.9rem;
  }
}
</style>
