<script setup lang="ts">
import { computed, markRaw, type Component, useSlots } from 'vue'
import { useDashboardStore } from '@/stores/dashboard'
import DashboardCustomizer from './DashboardCustomizer.vue'

// Import card components
import SystemStatusCard from './cards/SystemStatusCard.vue'
import UptimeCard from './cards/UptimeCard.vue'
import MemoryUsageCard from './cards/MemoryUsageCard.vue'
import GoroutinesCard from './cards/GoroutinesCard.vue'
import CpuChartCard from './cards/CpuChartCard.vue'
import FailoverStatusCard from './cards/FailoverStatusCard.vue'
// Metrics cards
import MetricsOverviewCard from './cards/MetricsOverviewCard.vue'
import TokenUsageChartCard from './cards/TokenUsageChartCard.vue'
import LatencyChartCard from './cards/LatencyChartCard.vue'
import ModelStatsCard from './cards/ModelStatsCard.vue'
import MediaGenerationCard from './cards/MediaGenerationCard.vue'

const props = withDefaults(
  defineProps<{
    metricsHistory?: Array<{
      timestamp: string
      cpu_percent: number
      memory_used_bytes: number
      goroutines: number
      heap_alloc_bytes: number
    }>
    showHeader?: boolean
    excludeCardIds?: string[]
    compactCardIds?: string[]
    disableHeroLayout?: boolean
    visibleCardIds?: string[]
  }>(),
  {
    showHeader: true,
    excludeCardIds: () => [],
    compactCardIds: () => [],
    disableHeroLayout: false,
    visibleCardIds: () => [],
  }
)

const dashboardStore = useDashboardStore()
const slots = useSlots()
const hasHeaderLeft = computed(() => Boolean(slots['header-left']))
const excludedCardIds = computed(() => new Set(props.excludeCardIds))
const compactCardIds = computed(() => new Set(props.compactCardIds))
const explicitVisibleCardIds = computed(() => props.visibleCardIds.filter(Boolean))

const visibleCards = computed(() => {
  const filtered = dashboardStore.enabledCards.filter((card) => !excludedCardIds.value.has(card.id))
  if (explicitVisibleCardIds.value.length === 0) return filtered

  const byId = new Map(filtered.map((card) => [card.id, card]))
  return explicitVisibleCardIds.value
    .map((id) => byId.get(id))
    .filter((card): card is NonNullable<typeof card> => Boolean(card))
})

const heroCardOrder: Record<string, number> = {
  'system-status': 1,
  uptime: 2,
  'memory-usage': 3,
  goroutines: 4,
}

const heroWideCardIds = new Set<string>(['system-status'])

// Map string component names to actual components
const componentMap: Record<string, Component> = {
  SystemStatusCard: markRaw(SystemStatusCard),
  UptimeCard: markRaw(UptimeCard),
  MemoryUsageCard: markRaw(MemoryUsageCard),
  GoroutinesCard: markRaw(GoroutinesCard),
  CpuChartCard: markRaw(CpuChartCard),
  FailoverStatusCard: markRaw(FailoverStatusCard),
  // Metrics cards
  MetricsOverviewCard: markRaw(MetricsOverviewCard),
  TokenUsageChartCard: markRaw(TokenUsageChartCard),
  LatencyChartCard: markRaw(LatencyChartCard),
  ModelStatsCard: markRaw(ModelStatsCard),
  MediaGenerationCard: markRaw(MediaGenerationCard),
}

// Get component from config
function getComponent(config: { component: Component | string }): Component | null {
  if (typeof config.component === 'string') {
    return componentMap[config.component] ?? null
  }
  return config.component
}

// Get grid column class based on minWidth
function getGridClass(minWidth?: number): string {
  switch (minWidth) {
    case 1:
      return 'dashboard-span-1'
    case 2:
      return 'dashboard-span-2'
    case 3:
      return 'dashboard-span-3'
    case 4:
      return 'dashboard-span-4'
    default:
      return 'dashboard-span-1'
  }
}

const metricsHistoryCardIds = new Set(['memory-usage', 'goroutines', 'cpu-chart'])

const compactCapableCardIds = new Set(['cpu-chart'])

// Check if card needs metricsHistory prop
function needsMetricsHistory(cardId: string): boolean {
  return metricsHistoryCardIds.has(cardId)
}

function getCardProps(cardId: string): Record<string, unknown> {
  const bindProps: Record<string, unknown> = {}
  if (needsMetricsHistory(cardId)) {
    bindProps.metricsHistory = props.metricsHistory
  }
  if (compactCapableCardIds.has(cardId) && compactCardIds.value.has(cardId)) {
    bindProps.compact = true
  }
  return bindProps
}

function getHeroGridClass(cardId: string): string {
  return heroWideCardIds.has(cardId) ? 'dashboard-hero-span-2' : 'dashboard-hero-span-1'
}

function getHeroVariantClass(cardId: string): string {
  switch (cardId) {
    case 'system-status':
      return 'dashboard-hero-card-status'
    case 'uptime':
      return 'dashboard-hero-card-uptime'
    case 'memory-usage':
      return 'dashboard-hero-card-memory'
    case 'goroutines':
      return 'dashboard-hero-card-goroutines'
    default:
      return 'dashboard-hero-card-default'
  }
}

function getHeroSlotClass(cardId: string): string {
  switch (cardId) {
    case 'system-status':
      return 'dashboard-hero-slot-status'
    case 'uptime':
      return 'dashboard-hero-slot-uptime'
    case 'memory-usage':
      return 'dashboard-hero-slot-memory'
    case 'goroutines':
      return 'dashboard-hero-slot-goroutines'
    default:
      return ''
  }
}

const heroCards = computed(() => {
  if (props.disableHeroLayout) return []
  return visibleCards.value
    .filter((card) => card.config && heroCardOrder[card.id] != null)
    .sort((a, b) => (heroCardOrder[a.id] ?? 99) - (heroCardOrder[b.id] ?? 99))
})

// Group cards by size for additional layout sections
const smallCards = computed(() => {
  return visibleCards.value.filter(
    (card) =>
      card.config &&
      (((card.config.minWidth ?? 1) === 1 &&
        (props.disableHeroLayout || heroCardOrder[card.id] == null)) ||
        compactCardIds.value.has(card.id))
  )
})

const largeCards = computed(() => {
  return visibleCards.value.filter(
    (card) =>
      card.config &&
      (card.config.minWidth ?? 1) > 1 &&
      (props.disableHeroLayout || heroCardOrder[card.id] == null) &&
      !compactCardIds.value.has(card.id)
  )
})

const priorityLargeCardOrder: Record<string, number> = {
  'cpu-chart': 1,
}

const priorityLargeCards = computed(() =>
  largeCards.value
    .filter((card) => priorityLargeCardOrder[card.id] != null)
    .sort((a, b) => (priorityLargeCardOrder[a.id] ?? 99) - (priorityLargeCardOrder[b.id] ?? 99))
)

const standardLargeCards = computed(() =>
  largeCards.value.filter((card) => priorityLargeCardOrder[card.id] == null)
)

function getPriorityGridClass(cardId: string): string {
  const priorityCount = priorityLargeCards.value.length

  switch (cardId) {
    case 'cpu-chart':
      return priorityCount <= 1 ? 'dashboard-priority-span-full' : 'dashboard-priority-span-half'
    default:
      return ''
  }
}
</script>

<template>
  <div class="dashboard-grid-stack">
    <!-- Header with Customizer -->
    <div
      v-if="props.showHeader"
      class="dashboard-grid-header"
      :class="{ 'has-left': hasHeaderLeft }"
    >
      <slot
        v-if="hasHeaderLeft"
        name="header-left"
      />
      <DashboardCustomizer />
    </div>

    <!-- Hero Cards Grid -->
    <section
      v-if="heroCards.length > 0"
      class="dashboard-grid-wrap dashboard-grid-wrap-hero"
    >
      <div class="dashboard-grid dashboard-grid-hero">
        <div
          v-for="card in heroCards"
          :key="card.id"
          class="dashboard-card-surface dashboard-grid-item dashboard-grid-item-small dashboard-hero-card p-4"
          :class="[
            getHeroGridClass(card.id),
            getHeroVariantClass(card.id),
            getHeroSlotClass(card.id),
            { 'dashboard-hero-card-featured': heroWideCardIds.has(card.id) },
          ]"
        >
          <component
            :is="getComponent(card.config!)"
            v-if="card.config && getComponent(card.config)"
            v-bind="getCardProps(card.id)"
          />
        </div>
      </div>
    </section>

    <!-- Small Cards Grid (1-column cards) -->
    <section
      v-if="smallCards.length > 0"
      class="dashboard-grid-wrap dashboard-grid-cluster dashboard-grid-cluster-small"
      :class="{ 'with-divider': heroCards.length > 0 }"
    >
      <div class="dashboard-grid dashboard-grid-small">
        <div
          v-for="card in smallCards"
          :key="card.id"
          class="dashboard-grid-item dashboard-grid-item-small"
        >
          <div class="dashboard-card-surface dashboard-small-card-shell p-4">
            <component
              :is="getComponent(card.config!)"
              v-if="card.config && getComponent(card.config)"
              v-bind="getCardProps(card.id)"
            />
          </div>
        </div>
      </div>
    </section>

    <section
      v-if="priorityLargeCards.length > 0"
      class="dashboard-grid-wrap dashboard-grid-cluster dashboard-grid-cluster-featured"
      :class="{ 'with-divider': heroCards.length > 0 || smallCards.length > 0 }"
    >
      <div class="dashboard-grid dashboard-grid-featured">
        <div
          v-for="card in priorityLargeCards"
          :key="card.id"
          class="dashboard-grid-item dashboard-grid-item-featured"
          :class="getPriorityGridClass(card.id)"
        >
          <component
            :is="getComponent(card.config!)"
            v-if="card.config && getComponent(card.config)"
            v-bind="getCardProps(card.id)"
          />
        </div>
      </div>
    </section>

    <!-- Large Cards Grid (2+ column cards) -->
    <section
      v-if="standardLargeCards.length > 0"
      class="dashboard-grid-wrap dashboard-grid-cluster dashboard-grid-cluster-large"
      :class="{
        'with-divider':
          heroCards.length > 0 || smallCards.length > 0 || priorityLargeCards.length > 0,
      }"
    >
      <div class="dashboard-grid dashboard-grid-large">
        <div
          v-for="card in standardLargeCards"
          :key="card.id"
          class="dashboard-grid-item"
          :class="getGridClass(card.config?.minWidth)"
        >
          <component
            :is="getComponent(card.config!)"
            v-if="card.config && getComponent(card.config)"
            v-bind="getCardProps(card.id)"
          />
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.dashboard-grid-stack {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.dashboard-grid-header {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  min-height: 2rem;
  padding: 0 0.12rem 0.72rem;
  border-bottom: 1px solid rgba(203, 213, 225, 0.62);
}

.dashboard-grid-header.has-left {
  justify-content: space-between;
}

.dashboard-grid {
  display: grid;
  gap: 0.88rem;
}

.dashboard-grid-wrap {
  display: flex;
  flex-direction: column;
}

.dashboard-grid-cluster {
  padding: 0.96rem;
  border-radius: 1.24rem;
  border: 1px solid rgba(203, 213, 225, 0.72);
  background:
    radial-gradient(circle at 100% 0%, rgba(255, 255, 255, 0.48), transparent 42%),
    linear-gradient(180deg, rgba(255, 255, 255, 0.62) 0%, rgba(248, 250, 252, 0.92) 100%);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.74);
}

.dashboard-grid-cluster-small {
  background:
    radial-gradient(circle at 100% 0%, rgba(219, 234, 254, 0.32), transparent 42%),
    linear-gradient(180deg, rgba(252, 253, 255, 0.72) 0%, rgba(248, 250, 252, 0.94) 100%);
}

.dashboard-grid-cluster-large {
  background:
    radial-gradient(circle at 100% 0%, rgba(226, 232, 240, 0.34), transparent 42%),
    linear-gradient(180deg, rgba(255, 255, 255, 0.66) 0%, rgba(246, 248, 251, 0.94) 100%);
}

.dashboard-grid-cluster-featured {
  background:
    radial-gradient(circle at 100% 0%, rgba(191, 219, 254, 0.24), transparent 42%),
    linear-gradient(180deg, rgba(252, 254, 255, 0.76) 0%, rgba(246, 249, 252, 0.96) 100%);
}

.dashboard-grid-wrap.with-divider {
  padding-top: 1rem;
  border-top: 1px solid rgba(203, 213, 225, 0.62);
}

.dashboard-grid-cluster.with-divider {
  padding-top: 0.96rem;
  border-top: 0;
}

.dashboard-grid-wrap-hero {
  padding: 0.96rem;
  border-radius: 1.3rem;
  border: 1px solid rgba(191, 219, 254, 0.58);
  background:
    radial-gradient(circle at 100% 0%, rgba(219, 234, 254, 0.26), transparent 42%),
    linear-gradient(180deg, rgba(255, 255, 255, 0.44) 0%, rgba(245, 248, 252, 0.82) 100%);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.8);
  margin-bottom: 0.22rem;
}

.dashboard-grid-small {
  grid-template-columns: repeat(1, minmax(0, 1fr));
}

.dashboard-grid-hero {
  grid-template-columns: repeat(1, minmax(0, 1fr));
}

.dashboard-grid-large {
  grid-template-columns: repeat(1, minmax(0, 1fr));
}

.dashboard-grid-featured {
  grid-template-columns: minmax(0, 1fr);
}

.dashboard-grid-item {
  min-width: 0;
}

.dashboard-grid-item > * {
  height: 100%;
}

.dashboard-small-card-shell {
  min-height: 100%;
}

.dashboard-grid-item-small {
  min-height: 10rem;
}

.dashboard-grid-item-featured {
  min-height: 14rem;
}

.dashboard-priority-span-full,
.dashboard-priority-span-half {
  grid-column: span 1 / span 1;
}

.dashboard-hero-card {
  --dashboard-hero-accent: #2563eb;
  --dashboard-hero-accent-soft: #60a5fa;
  --dashboard-hero-accent-strong: #1d4ed8;
  --dashboard-hero-soft: rgba(191, 219, 254, 0.48);
  --dashboard-hero-soft-edge: rgba(219, 234, 254, 0.3);
  --dashboard-hero-border: rgba(147, 197, 253, 0.62);
  --dashboard-hero-chip-bg: rgba(255, 255, 255, 0.78);
  --dashboard-hero-label-bg: rgba(219, 234, 254, 0.72);
  --dashboard-hero-muted: #64748b;
  --dashboard-hero-text: #0f172a;
  --dashboard-hero-track: rgba(148, 163, 184, 0.34);
  min-height: 10.2rem;
  display: flex;
  padding: 1.08rem;
  border-color: rgba(191, 219, 254, 0.78);
  background:
    radial-gradient(circle at 100% 0%, rgba(191, 219, 254, 0.34), transparent 46%),
    radial-gradient(circle at 0% 100%, rgba(226, 232, 240, 0.26), transparent 42%),
    linear-gradient(180deg, #fcfdff 0%, #f1f5f9 100%);
}

.dashboard-hero-card > * {
  width: 100%;
}

.dashboard-hero-card-featured {
  min-height: 13rem;
  padding: 1.22rem;
  box-shadow:
    0 22px 34px -34px rgba(15, 23, 42, 0.34),
    inset 0 1px 0 rgba(255, 255, 255, 0.92);
}

.dashboard-hero-card-status {
  --dashboard-hero-accent: #0f766e;
  --dashboard-hero-accent-soft: #2dd4bf;
  --dashboard-hero-accent-strong: #115e59;
  --dashboard-hero-soft: rgba(153, 246, 228, 0.48);
  --dashboard-hero-soft-edge: rgba(204, 251, 241, 0.34);
  --dashboard-hero-border: rgba(94, 234, 212, 0.62);
  --dashboard-hero-label-bg: rgba(204, 251, 241, 0.76);
  background:
    radial-gradient(circle at 100% 0%, rgba(153, 246, 228, 0.38), transparent 48%),
    radial-gradient(circle at 0% 100%, rgba(204, 251, 241, 0.28), transparent 44%),
    linear-gradient(180deg, #f6fffd 0%, #edf9f7 100%);
}

.dashboard-hero-card-uptime {
  --dashboard-hero-accent: #c2410c;
  --dashboard-hero-accent-soft: #fb923c;
  --dashboard-hero-accent-strong: #9a3412;
  --dashboard-hero-soft: rgba(253, 230, 138, 0.52);
  --dashboard-hero-soft-edge: rgba(254, 243, 199, 0.3);
  --dashboard-hero-border: rgba(251, 191, 36, 0.56);
  --dashboard-hero-label-bg: rgba(254, 243, 199, 0.8);
  background:
    radial-gradient(circle at 100% 0%, rgba(253, 230, 138, 0.42), transparent 48%),
    radial-gradient(circle at 0% 100%, rgba(254, 243, 199, 0.26), transparent 42%),
    linear-gradient(180deg, #fffdf7 0%, #fbf5eb 100%);
}

.dashboard-hero-card-memory {
  --dashboard-hero-accent: #7c3aed;
  --dashboard-hero-accent-soft: #a78bfa;
  --dashboard-hero-accent-strong: #6d28d9;
  --dashboard-hero-soft: rgba(221, 214, 254, 0.52);
  --dashboard-hero-soft-edge: rgba(243, 232, 255, 0.32);
  --dashboard-hero-border: rgba(196, 181, 253, 0.64);
  --dashboard-hero-label-bg: rgba(237, 233, 254, 0.78);
  background:
    radial-gradient(circle at 100% 0%, rgba(221, 214, 254, 0.42), transparent 48%),
    radial-gradient(circle at 0% 100%, rgba(243, 232, 255, 0.28), transparent 42%),
    linear-gradient(180deg, #fcfaff 0%, #f4effb 100%);
}

.dashboard-hero-card-goroutines {
  --dashboard-hero-accent: #0f766e;
  --dashboard-hero-accent-soft: #38bdf8;
  --dashboard-hero-accent-strong: #155e75;
  --dashboard-hero-soft: rgba(191, 219, 254, 0.42);
  --dashboard-hero-soft-edge: rgba(207, 250, 254, 0.34);
  --dashboard-hero-border: rgba(125, 211, 252, 0.64);
  --dashboard-hero-label-bg: rgba(224, 242, 254, 0.8);
  background:
    radial-gradient(circle at 100% 0%, rgba(186, 230, 253, 0.42), transparent 48%),
    radial-gradient(circle at 0% 100%, rgba(207, 250, 254, 0.3), transparent 42%),
    linear-gradient(180deg, #f7fdff 0%, #edf7fb 100%);
}

.dashboard-hero-card :deep(.dashboard-card-label) {
  background: var(--dashboard-hero-label-bg);
  color: var(--dashboard-hero-accent-strong);
}

.dashboard-hero-card :deep(.dashboard-card-chip) {
  border-color: var(--dashboard-hero-border);
  background: var(--dashboard-hero-chip-bg);
  color: var(--dashboard-hero-accent-strong);
}

.dashboard-hero-card :deep(.dashboard-card-subtitle) {
  color: rgba(15, 23, 42, 0.72);
}

.dashboard-hero-card :deep(.dashboard-card-value) {
  font-size: clamp(2rem, 1.4vw + 1rem, 2.45rem);
  color: var(--dashboard-hero-text);
}

.dashboard-hero-card-featured :deep(.dashboard-card-value) {
  font-size: clamp(2.3rem, 1.9vw + 1rem, 2.9rem);
}

.dashboard-hero-card :deep(.dashboard-card-footnote) {
  color: var(--dashboard-hero-muted);
}

.dashboard-hero-card :deep(.dashboard-card-stack) {
  gap: 1rem;
}

.dashboard-hero-card :deep(.dashboard-mini-bars) {
  width: clamp(4.8rem, 18vw, 7rem);
  height: 4.35rem;
}

.dashboard-hero-span-1 {
  grid-column: span 1 / span 1;
}

.dashboard-hero-span-2 {
  grid-column: span 1 / span 1;
}

.dashboard-hero-slot-uptime,
.dashboard-hero-slot-memory {
  min-height: 9.5rem;
}

.dashboard-hero-slot-goroutines {
  min-height: 10.4rem;
}

.dashboard-span-1 {
  grid-column: span 1 / span 1;
}

.dashboard-span-2 {
  grid-column: span 1 / span 1;
}

.dashboard-span-3 {
  grid-column: span 1 / span 1;
}

.dashboard-span-4 {
  grid-column: span 1 / span 1;
}

@media (min-width: 640px) {
  .dashboard-grid-small {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .dashboard-grid-hero {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (min-width: 960px) {
  .dashboard-grid-hero {
    grid-template-columns: repeat(4, minmax(0, 1fr));
    grid-template-areas:
      'status status uptime memory'
      'status status goroutines goroutines';
    align-items: stretch;
  }

  .dashboard-grid-large {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .dashboard-hero-span-2 {
    grid-column: span 2 / span 2;
  }

  .dashboard-hero-slot-status {
    grid-area: status;
  }

  .dashboard-hero-slot-uptime {
    grid-area: uptime;
  }

  .dashboard-hero-slot-memory {
    grid-area: memory;
  }

  .dashboard-hero-slot-goroutines {
    grid-area: goroutines;
  }
}

@media (min-width: 1180px) {
  .dashboard-grid-small {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .dashboard-grid-featured {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }

  .dashboard-priority-span-full {
    grid-column: span 4 / span 4;
  }

  .dashboard-priority-span-half {
    grid-column: span 2 / span 2;
  }
}

@media (min-width: 1280px) {
  .dashboard-grid-large {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }

  .dashboard-span-2 {
    grid-column: span 2 / span 2;
  }

  .dashboard-span-3 {
    grid-column: span 3 / span 3;
  }

  .dashboard-span-4 {
    grid-column: span 4 / span 4;
  }
}

:root.dark .dashboard-hero-card-featured,
[data-theme='dark'] .dashboard-hero-card-featured {
  box-shadow:
    0 22px 34px -30px rgba(2, 6, 23, 0.82),
    inset 0 1px 0 rgba(148, 163, 184, 0.08);
}

:root.dark .dashboard-grid-wrap-hero,
[data-theme='dark'] .dashboard-grid-wrap-hero {
  border-color: rgba(100, 116, 139, 0.42);
  background:
    radial-gradient(circle at 100% 0%, rgba(37, 99, 235, 0.1), transparent 42%),
    linear-gradient(180deg, rgba(24, 33, 47, 0.54) 0%, rgba(19, 27, 40, 0.84) 100%);
  box-shadow: inset 0 1px 0 rgba(148, 163, 184, 0.06);
}

:root.dark .dashboard-grid-header,
[data-theme='dark'] .dashboard-grid-header {
  border-bottom-color: rgba(100, 116, 139, 0.46);
}

:root.dark .dashboard-grid-cluster,
[data-theme='dark'] .dashboard-grid-cluster {
  border-color: rgba(100, 116, 139, 0.4);
  background:
    radial-gradient(circle at 100% 0%, rgba(51, 65, 85, 0.2), transparent 42%),
    linear-gradient(180deg, rgba(24, 33, 47, 0.68) 0%, rgba(17, 24, 39, 0.9) 100%);
  box-shadow: inset 0 1px 0 rgba(148, 163, 184, 0.06);
}

:root.dark .dashboard-grid-cluster-small,
[data-theme='dark'] .dashboard-grid-cluster-small {
  background:
    radial-gradient(circle at 100% 0%, rgba(37, 99, 235, 0.16), transparent 42%),
    linear-gradient(180deg, rgba(27, 39, 56, 0.72) 0%, rgba(19, 27, 40, 0.9) 100%);
}

:root.dark .dashboard-grid-cluster-large,
[data-theme='dark'] .dashboard-grid-cluster-large {
  background:
    radial-gradient(circle at 100% 0%, rgba(71, 85, 105, 0.18), transparent 42%),
    linear-gradient(180deg, rgba(24, 33, 47, 0.7) 0%, rgba(17, 24, 39, 0.9) 100%);
}

:root.dark .dashboard-grid-cluster-featured,
[data-theme='dark'] .dashboard-grid-cluster-featured {
  background:
    radial-gradient(circle at 100% 0%, rgba(37, 99, 235, 0.12), transparent 42%),
    linear-gradient(180deg, rgba(28, 40, 58, 0.76) 0%, rgba(20, 29, 42, 0.92) 100%);
}

:root.dark .dashboard-hero-card,
[data-theme='dark'] .dashboard-hero-card {
  --dashboard-hero-chip-bg: rgba(15, 23, 42, 0.24);
  --dashboard-hero-text: rgb(241 245 249);
  --dashboard-hero-muted: rgb(148 163 184);
  --dashboard-hero-track: rgba(100, 116, 139, 0.56);
  border-color: rgba(96, 165, 250, 0.28);
  background:
    radial-gradient(circle at 100% 0%, rgba(37, 99, 235, 0.16), transparent 46%),
    radial-gradient(circle at 0% 100%, rgba(51, 65, 85, 0.24), transparent 42%),
    linear-gradient(180deg, #2a3950 0%, #223144 100%);
}

:root.dark .dashboard-hero-card-status,
[data-theme='dark'] .dashboard-hero-card-status {
  --dashboard-hero-accent: #5eead4;
  --dashboard-hero-accent-soft: #99f6e4;
  --dashboard-hero-accent-strong: #99f6e4;
  --dashboard-hero-label-bg: rgba(15, 118, 110, 0.26);
  --dashboard-hero-border: rgba(45, 212, 191, 0.32);
  background:
    radial-gradient(circle at 100% 0%, rgba(45, 212, 191, 0.16), transparent 46%),
    radial-gradient(circle at 0% 100%, rgba(15, 118, 110, 0.16), transparent 42%),
    linear-gradient(180deg, #203842 0%, #1b2f37 100%);
}

:root.dark .dashboard-hero-card-uptime,
[data-theme='dark'] .dashboard-hero-card-uptime {
  --dashboard-hero-accent: #fdba74;
  --dashboard-hero-accent-soft: #fed7aa;
  --dashboard-hero-accent-strong: #fed7aa;
  --dashboard-hero-label-bg: rgba(154, 52, 18, 0.28);
  --dashboard-hero-border: rgba(251, 146, 60, 0.32);
  background:
    radial-gradient(circle at 100% 0%, rgba(251, 191, 36, 0.18), transparent 46%),
    radial-gradient(circle at 0% 100%, rgba(194, 65, 12, 0.18), transparent 42%),
    linear-gradient(180deg, #3b3026 0%, #2f251f 100%);
}

:root.dark .dashboard-hero-card-memory,
[data-theme='dark'] .dashboard-hero-card-memory {
  --dashboard-hero-accent: #c4b5fd;
  --dashboard-hero-accent-soft: #ddd6fe;
  --dashboard-hero-accent-strong: #ddd6fe;
  --dashboard-hero-label-bg: rgba(109, 40, 217, 0.24);
  --dashboard-hero-border: rgba(167, 139, 250, 0.34);
  background:
    radial-gradient(circle at 100% 0%, rgba(139, 92, 246, 0.18), transparent 46%),
    radial-gradient(circle at 0% 100%, rgba(124, 58, 237, 0.16), transparent 42%),
    linear-gradient(180deg, #31284a 0%, #271f3f 100%);
}

:root.dark .dashboard-hero-card-goroutines,
[data-theme='dark'] .dashboard-hero-card-goroutines {
  --dashboard-hero-accent: #7dd3fc;
  --dashboard-hero-accent-soft: #bae6fd;
  --dashboard-hero-accent-strong: #bae6fd;
  --dashboard-hero-label-bg: rgba(8, 145, 178, 0.24);
  --dashboard-hero-border: rgba(56, 189, 248, 0.34);
  background:
    radial-gradient(circle at 100% 0%, rgba(56, 189, 248, 0.18), transparent 46%),
    radial-gradient(circle at 0% 100%, rgba(14, 116, 144, 0.16), transparent 42%),
    linear-gradient(180deg, #1f3646 0%, #1a2d39 100%);
}

:root.dark .dashboard-grid-wrap.with-divider,
[data-theme='dark'] .dashboard-grid-wrap.with-divider {
  border-top-color: rgba(100, 116, 139, 0.46);
}
</style>
