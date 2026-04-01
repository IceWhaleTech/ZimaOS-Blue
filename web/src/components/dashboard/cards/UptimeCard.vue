<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useSystemStore } from '@/stores/system'
import Skeleton from '@/components/Skeleton.vue'

withDefaults(
  defineProps<{
    metricsHistory?: Array<{
      timestamp: string
      cpu_percent: number
      memory_used_bytes: number
      goroutines: number
      heap_alloc_bytes: number
    }>
  }>(),
  {
    metricsHistory: () => [],
  }
)

const { t } = useI18n()
const systemStore = useSystemStore()

const uptimeLabel = computed(() => {
  return systemStore.health?.uptime || '-'
})

const ringDashOffset = computed(() => {
  const status = (systemStore.health?.status || '').toLowerCase()
  const progress = status === 'ok' ? 0.88 : status === 'degraded' ? 0.56 : status ? 0.36 : 0.18
  const circumference = 2 * Math.PI * 28
  return circumference * (1 - progress)
})
</script>

<template>
  <div class="dashboard-card-stack">
    <div class="dashboard-card-footer">
      <div class="dashboard-card-copy">
        <p class="dashboard-card-label">{{ t('system.uptime') }}</p>
        <p class="dashboard-card-subtitle mt-2">{{ t('system.cards.uptime.subtitle') }}</p>
      </div>
    </div>

    <div class="dashboard-card-footer uptime-card-main">
      <div class="dashboard-card-copy">
        <template v-if="systemStore.loading">
          <Skeleton height="1.75rem" width="80%" rounded="md" />
        </template>
        <template v-else>
          <p class="dashboard-card-value truncate text-gray-900 dark:text-white">
            {{ uptimeLabel }}
          </p>
          <p class="dashboard-card-footnote mt-3">{{ t('system.cards.uptime.footnote') }}</p>
        </template>
      </div>

      <div class="cpu-ring-wrap flex-shrink-0" aria-hidden="true">
        <svg viewBox="0 0 72 72" class="cpu-ring-svg">
          <circle class="cpu-ring-track" cx="36" cy="36" r="28" />
          <circle
            class="cpu-ring-value"
            cx="36"
            cy="36"
            r="28"
            :stroke-dashoffset="ringDashOffset"
          />
        </svg>
      </div>
    </div>
  </div>
</template>

<style scoped>
.uptime-card-main {
  align-items: center;
}

.cpu-ring-wrap {
  width: 4.4rem;
  height: 4.4rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.cpu-ring-svg {
  width: 100%;
  height: 100%;
  transform: rotate(-90deg);
}

.cpu-ring-track {
  fill: none;
  stroke: var(--dashboard-hero-track, rgba(148, 163, 184, 0.42));
  stroke-width: 5;
  stroke-dasharray: 2 6;
  stroke-linecap: round;
}

.cpu-ring-value {
  fill: none;
  stroke: var(--dashboard-hero-accent, #2563eb);
  stroke-width: 5;
  stroke-linecap: butt;
  stroke-dasharray: 175.93;
  transition: stroke-dashoffset 220ms ease;
}

:root.dark .cpu-ring-track,
[data-theme='dark'] .cpu-ring-track {
  stroke: var(--dashboard-hero-track, rgba(100, 116, 139, 0.6));
}

:root.dark .cpu-ring-value,
[data-theme='dark'] .cpu-ring-value {
  stroke: var(--dashboard-hero-accent, #60a5fa);
}
</style>
