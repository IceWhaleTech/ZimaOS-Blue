<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, shallowRef, type Component } from 'vue'
import { useI18n } from 'vue-i18n'
import { useSystemStore } from '@/stores/system'
import { useMetricsStore } from '@/stores/metrics'
import { useDashboardStore } from '@/stores/dashboard'
import { systemApi } from '@/api/index'
import type { DetailedSystemInfo, SystemMetrics } from '@/api/system'
import { ConfigurableDashboard, SystemStatusCard } from '@/components/dashboard'
import DashboardCustomizer from '@/components/dashboard/DashboardCustomizer.vue'
import UptimeCard from '@/components/dashboard/cards/UptimeCard.vue'
import CpuChartCard from '@/components/dashboard/cards/CpuChartCard.vue'
import MemoryUsageCard from '@/components/dashboard/cards/MemoryUsageCard.vue'
import GoroutinesCard from '@/components/dashboard/cards/GoroutinesCard.vue'
import ProgressBar from '@/components/ProgressBar.vue'
import DonutChart from '@/components/DonutChart.vue'

const { t } = useI18n()
const systemStore = useSystemStore()
const metricsStore = useMetricsStore()
const dashboardStore = useDashboardStore()

const dashboardPrimaryCardIds = ['uptime', 'cpu-chart', 'memory-usage', 'goroutines'] as const
const dashboardSecondaryExcludedCardIds = ['system-status', ...dashboardPrimaryCardIds]
type DashboardPrimaryCardId = (typeof dashboardPrimaryCardIds)[number]

interface HomePrimaryCard {
  id: DashboardPrimaryCardId
  component: Component
  props: Record<string, unknown>
}

const primaryCardComponentMap: Record<DashboardPrimaryCardId, Component> = {
  uptime: UptimeCard,
  'cpu-chart': CpuChartCard,
  'memory-usage': MemoryUsageCard,
  goroutines: GoroutinesCard,
}

let refreshInterval: ReturnType<typeof setInterval> | null = null
let abortController: AbortController | null = null

const metricsHistory = shallowRef<SystemMetrics[]>([])
const manualRefreshing = ref(false)
const isPageVisible = ref(true)
const detailedInfo = ref<DetailedSystemInfo | null>(null)
const detailedInfoLoading = ref(false)
const showDetailedInfo = ref(false)

const enabledCardIdSet = computed(() => new Set(dashboardStore.enabledCards.map((card) => card.id)))
const showStatusCard = computed(() => enabledCardIdSet.value.has('system-status'))

const visiblePrimaryCards = computed<HomePrimaryCard[]>(() =>
  dashboardPrimaryCardIds
    .filter((id) => enabledCardIdSet.value.has(id))
    .map((id) => ({
      id,
      component: primaryCardComponentMap[id],
      props:
        id === 'cpu-chart'
          ? { metricsHistory: metricsHistory.value, compact: true }
          : id === 'uptime'
            ? {}
            : { metricsHistory: metricsHistory.value },
    }))
)

const osInfoItems = computed(() => {
  const info = detailedInfo.value
  if (!info) return []

  return [
    { label: t('system.osVersion'), value: info.os.version || '-' },
    { label: t('system.kernel'), value: info.os.kernel || '-' },
    { label: t('system.architecture'), value: info.os.architecture || '-' },
    { label: t('system.hostname'), value: info.os.hostname || '-' },
    { label: t('system.uptime'), value: info.os.uptime_human || '-' },
    { label: t('system.bootTime'), value: formatDateTime(info.os.boot_time) },
  ]
})

const cpuInfoItems = computed(() => {
  const info = detailedInfo.value
  if (!info) return []

  return [
    { label: t('system.cpuModel'), value: info.hardware.cpu.model || '-' },
    { label: t('system.cpuVendor'), value: info.hardware.cpu.vendor_id || '-' },
    {
      label: t('system.cpuCores'),
      value: `${info.hardware.cpu.cores || '-'} ${t('system.cores')} / ${info.hardware.cpu.threads || '-'} ${t('system.threads')}`,
    },
    { label: t('system.cpuFrequency'), value: formatFrequency(info.hardware.cpu.frequency) },
  ]
})

const runtimeInfoItems = computed(() => {
  const info = detailedInfo.value
  if (!info) return []

  return [
    { label: t('system.goVersion'), value: info.runtime.go_version || '-' },
    { label: t('system.numGoroutines'), value: `${info.runtime.num_goroutine ?? '-'}` },
    { label: t('system.goMaxProcs'), value: `${info.runtime.gomaxprocs ?? '-'}` },
    {
      label: t('system.heapAlloc'),
      value: formatBytes((info.runtime.alloc_mb ?? 0) * 1024 * 1024),
    },
    {
      label: t('system.totalAlloc'),
      value: formatBytes((info.runtime.total_alloc_mb ?? 0) * 1024 * 1024),
    },
    { label: t('system.sysMemory'), value: formatBytes((info.runtime.sys_mb ?? 0) * 1024 * 1024) },
    { label: t('system.gcCount'), value: `${info.runtime.num_gc ?? '-'}` },
  ]
})

const visibleDisks = computed(() => detailedInfo.value?.hardware.disk?.slice(0, 6) ?? [])
const visibleGpus = computed(() => detailedInfo.value?.hardware.gpu ?? [])
const visibleNetworkInterfaces = computed(
  () =>
    detailedInfo.value?.network.interfaces?.filter((iface) => !iface.is_loopback && iface.is_up) ??
    []
)

function formatBytes(bytes: number | undefined | null): string {
  if (bytes == null) return '-'
  if (bytes === 0) return '0 B'

  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(k)), sizes.length - 1)
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(2))} ${sizes[i]}`
}

function formatFrequency(frequency: number | undefined | null): string {
  if (frequency == null || frequency <= 0) return '-'
  return `${frequency.toFixed(0)} MHz`
}

function formatDateTime(timestamp: number | undefined | null): string {
  if (timestamp == null || timestamp <= 0) return '-'
  return new Date(timestamp * 1000).toLocaleString()
}

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

async function fetchDetailedInfo(forceRefresh = false) {
  if (detailedInfoLoading.value) return
  if (!forceRefresh && detailedInfo.value) return

  detailedInfoLoading.value = true
  try {
    const response = await systemApi.getInfo(true)
    detailedInfo.value = response.data.system || null
  } catch {
    detailedInfo.value = null
  } finally {
    detailedInfoLoading.value = false
  }
}

async function refreshAll(forceRefresh = false) {
  const tasks: Promise<unknown>[] = [
    systemStore.fetchAll(forceRefresh),
    metricsStore.fetchAll(),
    fetchMetricsHistory(),
  ]

  if (forceRefresh && showDetailedInfo.value) {
    tasks.push(fetchDetailedInfo(true))
  }

  await Promise.all(tasks)
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

async function toggleDetailedInfo() {
  showDetailedInfo.value = !showDetailedInfo.value
  if (showDetailedInfo.value) {
    await fetchDetailedInfo()
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
        <section v-if="showStatusCard" class="dashboard-status-row">
          <div class="dashboard-card-surface dashboard-status-shell p-4">
            <SystemStatusCard compact />
          </div>
        </section>

        <section v-if="visiblePrimaryCards.length > 0" class="dashboard-primary-grid">
          <div
            v-for="card in visiblePrimaryCards"
            :key="card.id"
            class="dashboard-card-surface dashboard-small-card-shell p-4"
          >
            <component :is="card.component" v-bind="card.props" />
          </div>
        </section>

        <ConfigurableDashboard
          :metrics-history="metricsHistory"
          :show-header="false"
          :disable-hero-layout="true"
          :exclude-card-ids="dashboardSecondaryExcludedCardIds"
        />
        <section class="dashboard-details-stage">
          <div class="dashboard-card-surface dashboard-details-toggle">
            <div class="dashboard-card-copy min-w-0">
              <p class="dashboard-card-label">{{ t('system.detailedInfo') }}</p>
              <h2 class="dashboard-card-subtitle mt-2">{{ t('system.detailedSystemInfo') }}</h2>
            </div>
            <button class="dashboard-details-button" type="button" @click="toggleDetailedInfo">
              <span class="dashboard-card-chip">
                {{ showDetailedInfo ? t('common.close') : t('system.detailedInfo') }}
              </span>
            </button>
          </div>

          <div v-if="showDetailedInfo" class="dashboard-details-grid">
            <div
              v-if="detailedInfoLoading"
              class="dashboard-card-surface dashboard-details-panel dashboard-details-panel-span-full dashboard-details-empty"
            >
              <p class="dashboard-card-footnote text-sm">
                {{ t('system.loadingDetailedInfo') }}
              </p>
            </div>

            <template v-else-if="detailedInfo">
              <div class="dashboard-card-surface dashboard-details-panel">
                <div class="dashboard-details-panel-head">
                  <p class="dashboard-card-label">{{ t('system.osInfo') }}</p>
                </div>
                <div class="dashboard-details-kv-grid">
                  <div
                    v-for="item in osInfoItems"
                    :key="item.label"
                    class="dashboard-card-subsurface dashboard-details-kv"
                  >
                    <p class="dashboard-card-label dashboard-details-micro-label">
                      {{ item.label }}
                    </p>
                    <p class="dashboard-card-subtitle mt-2 text-sm">
                      {{ item.value }}
                    </p>
                  </div>
                </div>
              </div>

              <div class="dashboard-card-surface dashboard-details-panel">
                <div class="dashboard-details-panel-head">
                  <p class="dashboard-card-label">{{ t('system.cpuInfo') }}</p>
                </div>
                <div class="dashboard-details-split">
                  <div class="dashboard-details-visual">
                    <DonutChart
                      :value="detailedInfo.hardware.cpu.usage || 0"
                      :max="100"
                      :label="t('system.cpuUsage')"
                      color="auto"
                      :size="138"
                    />
                  </div>
                  <div class="dashboard-details-kv-grid dashboard-details-kv-grid-compact">
                    <div
                      v-for="item in cpuInfoItems"
                      :key="item.label"
                      class="dashboard-card-subsurface dashboard-details-kv"
                    >
                      <p class="dashboard-card-label dashboard-details-micro-label">
                        {{ item.label }}
                      </p>
                      <p class="dashboard-card-subtitle mt-2 text-sm">
                        {{ item.value }}
                      </p>
                    </div>
                  </div>
                </div>
              </div>

              <div class="dashboard-card-surface dashboard-details-panel">
                <div class="dashboard-details-panel-head">
                  <p class="dashboard-card-label">{{ t('system.memoryInfo') }}</p>
                </div>
                <div class="dashboard-details-split">
                  <div class="dashboard-details-visual">
                    <DonutChart
                      :value="detailedInfo.hardware.memory.used || 0"
                      :max="Math.max(detailedInfo.hardware.memory.total || 0, 1)"
                      :label="t('system.ram')"
                      :value-label="formatBytes(detailedInfo.hardware.memory.used)"
                      color="auto"
                      :size="120"
                    />
                    <DonutChart
                      v-if="detailedInfo.hardware.memory.swap_total > 0"
                      :value="detailedInfo.hardware.memory.swap_used || 0"
                      :max="Math.max(detailedInfo.hardware.memory.swap_total || 0, 1)"
                      :label="t('system.swap')"
                      :value-label="formatBytes(detailedInfo.hardware.memory.swap_used)"
                      color="purple"
                      :size="120"
                    />
                  </div>
                  <div class="dashboard-details-progress-stack">
                    <div class="dashboard-card-subsurface dashboard-details-progress-block">
                      <div class="mb-3 flex items-center justify-between gap-3 text-sm">
                        <span class="dashboard-card-title">{{ t('system.ram') }}</span>
                        <span class="dashboard-card-subtitle text-sm">
                          {{ formatBytes(detailedInfo.hardware.memory.used) }} /
                          {{ formatBytes(detailedInfo.hardware.memory.total) }}
                        </span>
                      </div>
                      <ProgressBar
                        :value="detailedInfo.hardware.memory.used || 0"
                        :max="Math.max(detailedInfo.hardware.memory.total || 0, 1)"
                        :show-percent="false"
                        color="auto"
                        size="md"
                      />
                      <p class="dashboard-card-footnote mt-3">
                        {{ t('system.availableMemory') }}:
                        {{ formatBytes(detailedInfo.hardware.memory.available) }}
                      </p>
                    </div>

                    <div
                      v-if="detailedInfo.hardware.memory.swap_total > 0"
                      class="dashboard-card-subsurface dashboard-details-progress-block"
                    >
                      <div class="mb-3 flex items-center justify-between gap-3 text-sm">
                        <span class="dashboard-card-title">{{ t('system.swap') }}</span>
                        <span class="dashboard-card-subtitle text-sm">
                          {{ formatBytes(detailedInfo.hardware.memory.swap_used) }} /
                          {{ formatBytes(detailedInfo.hardware.memory.swap_total) }}
                        </span>
                      </div>
                      <ProgressBar
                        :value="detailedInfo.hardware.memory.swap_used || 0"
                        :max="Math.max(detailedInfo.hardware.memory.swap_total || 0, 1)"
                        :show-percent="false"
                        color="purple"
                        size="md"
                      />
                    </div>
                  </div>
                </div>
              </div>

              <div class="dashboard-card-surface dashboard-details-panel">
                <div class="dashboard-details-panel-head">
                  <p class="dashboard-card-label">{{ t('system.runtimeInfo') }}</p>
                </div>
                <div class="dashboard-details-kv-grid">
                  <div
                    v-for="item in runtimeInfoItems"
                    :key="item.label"
                    class="dashboard-card-subsurface dashboard-details-kv"
                  >
                    <p class="dashboard-card-label dashboard-details-micro-label">
                      {{ item.label }}
                    </p>
                    <p class="dashboard-card-subtitle mt-2 text-sm">
                      {{ item.value }}
                    </p>
                  </div>
                </div>
              </div>

              <div
                v-if="visibleGpus.length > 0"
                class="dashboard-card-surface dashboard-details-panel dashboard-details-panel-span-full"
              >
                <div class="dashboard-details-panel-head">
                  <p class="dashboard-card-label">{{ t('system.gpuInfo') }}</p>
                </div>
                <div class="dashboard-details-resource-grid">
                  <div
                    v-for="gpu in visibleGpus"
                    :key="`${gpu.name}-${gpu.vendor}-${gpu.driver}`"
                    class="dashboard-card-subsurface dashboard-details-resource-card"
                  >
                    <div class="flex items-start justify-between gap-3">
                      <div class="min-w-0">
                        <p class="dashboard-card-subtitle truncate text-sm">
                          {{ gpu.name || '-' }}
                        </p>
                        <p class="dashboard-card-footnote mt-1">
                          {{ gpu.vendor || '-' }}
                        </p>
                      </div>
                      <span class="dashboard-card-chip">{{ gpu.driver || '-' }}</span>
                    </div>

                    <div v-if="gpu.memory_total > 0" class="mt-4">
                      <div class="mb-3 flex items-center justify-between gap-3 text-sm">
                        <span class="dashboard-card-title">{{ t('system.vram') }}</span>
                        <span class="dashboard-card-subtitle text-sm">
                          {{ formatBytes(gpu.memory_used) }} / {{ formatBytes(gpu.memory_total) }}
                        </span>
                      </div>
                      <ProgressBar
                        :value="gpu.memory_used || 0"
                        :max="Math.max(gpu.memory_total || 0, 1)"
                        :show-percent="false"
                        color="purple"
                        size="md"
                      />
                    </div>
                  </div>
                </div>
              </div>

              <div
                v-if="visibleDisks.length > 0"
                class="dashboard-card-surface dashboard-details-panel dashboard-details-panel-span-full"
              >
                <div class="dashboard-details-panel-head">
                  <p class="dashboard-card-label">{{ t('system.diskInfo') }}</p>
                </div>
                <div class="dashboard-details-resource-grid">
                  <div
                    v-for="disk in visibleDisks"
                    :key="disk.device"
                    class="dashboard-card-subsurface dashboard-details-resource-card"
                  >
                    <div class="mb-3 flex items-center justify-between gap-3">
                      <div class="min-w-0">
                        <p
                          class="dashboard-card-subtitle truncate text-sm"
                          :title="disk.mount_point"
                        >
                          {{ disk.mount_point || disk.device }}
                        </p>
                        <p class="dashboard-card-footnote mt-1">
                          {{ disk.device || '-' }}
                        </p>
                      </div>
                      <span class="dashboard-card-chip">{{ disk.fs_type || '-' }}</span>
                    </div>

                    <ProgressBar
                      :value="disk.used || 0"
                      :max="Math.max(disk.total || 0, 1)"
                      :show-percent="false"
                      color="auto"
                      size="md"
                    />

                    <div class="mt-3 flex items-center justify-between gap-3 text-xs">
                      <span class="dashboard-card-footnote">
                        {{ formatBytes(disk.used) }} {{ t('system.used') }}
                      </span>
                      <span class="dashboard-card-footnote">
                        {{ formatBytes(disk.available) }} {{ t('system.free') }}
                      </span>
                    </div>
                  </div>
                </div>
              </div>

              <div
                v-if="visibleNetworkInterfaces.length > 0"
                class="dashboard-card-surface dashboard-details-panel dashboard-details-panel-span-full"
              >
                <div class="dashboard-details-panel-head">
                  <p class="dashboard-card-label">{{ t('system.networkInfo') }}</p>
                </div>
                <div class="dashboard-details-network-list">
                  <div
                    v-for="iface in visibleNetworkInterfaces"
                    :key="iface.name"
                    class="dashboard-card-subsurface dashboard-details-network-item"
                  >
                    <div class="flex flex-wrap items-center gap-2">
                      <span class="dashboard-card-subtitle text-sm">
                        {{ iface.name }}
                      </span>
                      <span class="dashboard-details-pill">
                        {{ t('system.interfaceUp') }}
                      </span>
                    </div>

                    <div class="dashboard-details-kv-grid dashboard-details-kv-grid-compact mt-4">
                      <div v-if="iface.mac" class="dashboard-details-network-field">
                        <p class="dashboard-card-label dashboard-details-micro-label">
                          {{ t('system.macAddress') }}
                        </p>
                        <p class="mt-2 break-all font-mono text-sm text-gray-900 dark:text-white">
                          {{ iface.mac }}
                        </p>
                      </div>

                      <div v-if="iface.ipv4?.length" class="dashboard-details-network-field">
                        <p class="dashboard-card-label dashboard-details-micro-label">
                          {{ t('system.ipv4Address') }}
                        </p>
                        <p class="mt-2 break-all font-mono text-sm text-gray-900 dark:text-white">
                          {{ iface.ipv4.join(', ') }}
                        </p>
                      </div>

                      <div class="dashboard-details-network-field">
                        <p class="dashboard-card-label dashboard-details-micro-label">
                          {{ t('system.mtu') }}
                        </p>
                        <p class="dashboard-card-subtitle mt-2 text-sm">
                          {{ iface.mtu }}
                        </p>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </template>

            <div
              v-else
              class="dashboard-card-surface dashboard-details-panel dashboard-details-panel-span-full dashboard-details-empty"
            >
              <p class="dashboard-card-footnote text-sm">{{ t('system.noDetailedInfo') }}</p>
            </div>
          </div>
        </section>
      </section>
    </section>
  </div>
</template>

<style scoped>
.home-page {
  max-width: 1480px;
  margin: 0 auto;
  padding: 0 0.75rem 1.8rem;
  --dashboard-card-border: var(--color-border);
  --dashboard-card-border-hover: var(--glass-border);
  --dashboard-card-surface-top: var(--color-bg-elevated);
  --dashboard-card-surface-bottom: var(--color-bg-base);
  --dashboard-card-subsurface-top: var(--color-bg-surface);
  --dashboard-card-subsurface-bottom: var(--color-bg-elevated);
  --dashboard-card-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.06), var(--shadow-lg);
  --dashboard-card-shadow-hover:
    inset 0 1px 0 rgba(255, 255, 255, 0.08), 0 18px 30px -28px rgba(15, 23, 42, 0.28);
  --dashboard-card-label-color: var(--color-text-secondary);
  --dashboard-card-subtitle-color: var(--color-text);
  --dashboard-card-footnote-color: var(--color-text-secondary);
  --dashboard-card-value-color: var(--color-text);
  --dashboard-card-chip-bg: var(--color-bg-surface);
  --dashboard-card-chip-color: var(--color-text-primary);
  --dashboard-card-divider: var(--color-border);
  --dashboard-details-pill-bg: rgba(16, 185, 129, 0.12);
  --dashboard-details-pill-color: #047857;
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

.dashboard-details-stage {
  display: flex;
  flex-direction: column;
  gap: 0.88rem;
  margin-top: 0.18rem;
  padding-top: 1rem;
  border-top: 1px solid var(--dashboard-card-divider);
}

.dashboard-details-toggle {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.94rem 1rem;
}

.dashboard-details-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  border: 0;
  background: transparent;
  color: inherit;
  transition: transform 0.18s ease;
}

.dashboard-details-button:hover {
  transform: translateY(-1px);
}

.dashboard-details-button :deep(.dashboard-card-chip) {
  min-height: 1.65rem;
  padding-inline: 0.72rem;
}

.dashboard-details-grid {
  display: grid;
  gap: 0.88rem;
  grid-template-columns: repeat(1, minmax(0, 1fr));
}

.dashboard-details-panel {
  min-width: 0;
  padding: 1rem 1.05rem;
}

.dashboard-details-panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  margin-bottom: 0.9rem;
}

.dashboard-details-kv-grid {
  display: grid;
  gap: 0.72rem;
  grid-template-columns: repeat(1, minmax(0, 1fr));
}

.dashboard-details-kv-grid-compact {
  grid-template-columns: repeat(1, minmax(0, 1fr));
}

.dashboard-details-kv {
  padding: 0.85rem 0.9rem;
}

.dashboard-details-micro-label {
  font-size: 0.54rem;
  letter-spacing: 0.1em;
}

.dashboard-details-split {
  display: grid;
  gap: 0.88rem;
}

.dashboard-details-visual {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.88rem;
  flex-wrap: wrap;
}

.dashboard-details-progress-stack,
.dashboard-details-network-list {
  display: flex;
  flex-direction: column;
  gap: 0.72rem;
}

.dashboard-details-progress-block,
.dashboard-details-network-item,
.dashboard-details-resource-card {
  padding: 0.9rem;
}

.dashboard-details-resource-grid {
  display: grid;
  gap: 0.78rem;
  grid-template-columns: repeat(1, minmax(0, 1fr));
}

.dashboard-details-network-field {
  min-width: 0;
}

.dashboard-details-pill {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 1.4rem;
  padding: 0 0.5rem;
  border-radius: 999px;
  background: var(--dashboard-details-pill-bg);
  color: var(--dashboard-details-pill-color);
  font-size: 0.72rem;
  font-weight: 600;
}

.dashboard-details-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 8rem;
  text-align: center;
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
  color: var(--dashboard-card-label-color);
  font-size: 0.65rem;
  font-weight: 500;
  letter-spacing: 0;
  text-transform: none;
}

:deep(.dashboard-primary-grid .dashboard-card-subtitle) {
  margin-top: 0.2rem;
  color: var(--dashboard-card-subtitle-color);
  font-size: 0.74rem;
  font-weight: 650;
}

:deep(.dashboard-primary-grid .dashboard-card-footnote) {
  margin-top: 0.22rem;
  color: var(--dashboard-card-footnote-color);
  font-size: 0.67rem;
  line-height: 1.3;
}

:deep(.dashboard-primary-grid .dashboard-card-value) {
  color: var(--dashboard-card-value-color);
  font-size: clamp(1.38rem, 0.72vw + 0.85rem, 1.72rem);
  line-height: 1.04;
}

:deep(.dashboard-primary-grid .dashboard-card-chip) {
  border: 0;
  background: var(--dashboard-card-chip-bg);
  color: var(--dashboard-card-chip-color);
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
  border: 1px solid var(--dashboard-card-border);
  border-radius: 1.5rem;
  background: linear-gradient(
    180deg,
    var(--dashboard-card-surface-top) 0%,
    var(--dashboard-card-surface-bottom) 100%
  );
  box-shadow: var(--dashboard-card-shadow);
  transition:
    transform 0.2s ease,
    box-shadow 0.2s ease,
    border-color 0.2s ease,
    background-color 0.2s ease;
}

:deep(.dashboard-shell .dashboard-card-surface:hover) {
  border-color: var(--dashboard-card-border-hover);
  transform: translateY(-1px);
  box-shadow: var(--dashboard-card-shadow-hover);
}

:deep(.dashboard-shell .dashboard-card-subsurface) {
  border: 0;
  background: linear-gradient(
    180deg,
    var(--dashboard-card-subsurface-top) 0%,
    var(--dashboard-card-subsurface-bottom) 100%
  );
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
  color: var(--dashboard-card-label-color);
  font-size: 0.68rem;
  font-weight: 500;
  letter-spacing: 0;
  text-transform: none;
}

:deep(.dashboard-shell .dashboard-card-chip) {
  border: 0;
  background: var(--dashboard-card-chip-bg);
  color: var(--dashboard-card-chip-color);
  min-height: 1.1rem;
  padding: 0.12rem 0.4rem;
  font-size: 0.6rem;
}

:deep(.dashboard-shell .dashboard-card-subtitle) {
  color: var(--dashboard-card-subtitle-color);
  font-size: 0.78rem;
  font-weight: 650;
}

:deep(.dashboard-shell .dashboard-card-footnote) {
  color: var(--dashboard-card-footnote-color);
  font-size: 0.72rem;
}

:deep(.dashboard-shell .dashboard-card-value) {
  color: var(--dashboard-card-value-color);
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

:global(.dark .dashboard-customizer-compact .dashboard-customize-trigger),
:global([data-theme='dark'] .dashboard-customizer-compact .dashboard-customize-trigger) {
  color: rgb(148 163 184);
  background: rgba(30, 41, 59, 0.88);
  box-shadow: 0 18px 36px -28px rgba(2, 6, 23, 0.72);
}

:global(.dark .dashboard-customizer-compact .dashboard-customize-trigger:hover),
:global([data-theme='dark'] .dashboard-customizer-compact .dashboard-customize-trigger:hover) {
  color: rgb(226 232 240);
  background: rgba(51, 65, 85, 0.92);
}

:root.dark .dashboard-refresh-button:hover,
[data-theme='dark'] .dashboard-refresh-button:hover,
html.dark .dashboard-refresh-button:hover {
  color: rgb(191 219 254);
  background: rgba(51, 65, 85, 0.92);
}

:root.dark .dashboard-details-button,
[data-theme='dark'] .dashboard-details-button,
html.dark .dashboard-details-button {
  color: rgb(226 232 240);
}

:root.dark .dashboard-details-button:hover,
[data-theme='dark'] .dashboard-details-button:hover,
html.dark .dashboard-details-button:hover {
  background: transparent;
}

:root.dark .dashboard-details-pill,
[data-theme='dark'] .dashboard-details-pill,
html.dark .dashboard-details-pill {
  background: rgba(16, 185, 129, 0.16);
  color: rgb(110 231 183);
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

@media (max-width: 1100px) {
  .home-page {
    padding-top: 4rem;
  }
}

@media (min-width: 700px) {
  .dashboard-primary-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .dashboard-details-kv-grid,
  .dashboard-details-kv-grid-compact,
  .dashboard-details-resource-grid {
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

  .dashboard-details-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .dashboard-details-panel-span-full {
    grid-column: 1 / -1;
  }

  .dashboard-details-kv-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .dashboard-details-kv-grid-compact {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .dashboard-details-resource-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
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

  .dashboard-details-toggle {
    flex-direction: column;
    align-items: flex-start;
  }

  .dashboard-details-button {
    width: 100%;
    justify-content: flex-start;
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

@media (min-width: 900px) {
  .dashboard-details-split {
    grid-template-columns: minmax(0, 0.78fr) minmax(0, 1.22fr);
    align-items: center;
  }
}
</style>
