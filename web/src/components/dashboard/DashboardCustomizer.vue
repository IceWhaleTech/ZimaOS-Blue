<script setup lang="ts">
import { ref, computed, watch, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useDashboardStore } from '@/stores/dashboard'
import { cardRegistry } from './cardRegistry'

const { t, te } = useI18n()
const dashboardStore = useDashboardStore()

const isOpen = ref(false)
const activeCategory = ref<'all' | 'overview' | 'system' | 'metrics'>('all')
let bodyOverflowBeforeLock = ''

const categories = [
  { id: 'all', labelKey: 'dashboard.categories.all' },
  { id: 'overview', labelKey: 'dashboard.categories.overview' },
  { id: 'system', labelKey: 'dashboard.categories.system' },
  { id: 'metrics', labelKey: 'dashboard.categories.metrics' },
] as const

function tr(key: string, fallback = ''): string {
  return te(key) ? t(key) : fallback
}

const filteredCards = computed(() => {
  const resolvedStateMap = new Map(
    dashboardStore.resolvedCardStates.map((state) => [state.id, state])
  )
  const cards = cardRegistry.map((config) => {
    const state = resolvedStateMap.get(config.id)
    return {
      config,
      enabled: state?.enabled ?? config.defaultEnabled,
    }
  })

  if (activeCategory.value === 'all') {
    return cards
  }
  return cards.filter((card) => card.config.category === activeCategory.value)
})

const totals = computed(() => {
  const resolvedStateMap = new Map(
    dashboardStore.resolvedCardStates.map((state) => [state.id, state])
  )
  const summary = cardRegistry.reduce(
    (acc, config) => {
      const state = resolvedStateMap.get(config.id)
      const enabled = state?.enabled ?? config.defaultEnabled
      if (enabled) acc.enabled += 1
      acc.total += 1
      return acc
    },
    { enabled: 0, total: 0 }
  )
  return summary
})

function toggleCard(id: string) {
  dashboardStore.toggleCard(id)
}

function resetToDefaults() {
  if (confirm(t('dashboard.confirmReset'))) {
    dashboardStore.resetToDefaults()
  }
}

function close() {
  isOpen.value = false
  dashboardStore.setCustomizing(false)
}

function open() {
  isOpen.value = true
  dashboardStore.setCustomizing(true)
}

function handleEscClose(event: KeyboardEvent) {
  if (event.key === 'Escape' && isOpen.value) {
    close()
  }
}

watch(isOpen, (open) => {
  if (typeof document === 'undefined' || typeof window === 'undefined') return

  if (open) {
    bodyOverflowBeforeLock = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    window.addEventListener('keydown', handleEscClose)
    return
  }

  document.body.style.overflow = bodyOverflowBeforeLock
  window.removeEventListener('keydown', handleEscClose)
})

onUnmounted(() => {
  if (typeof document !== 'undefined') {
    document.body.style.overflow = bodyOverflowBeforeLock
  }
  if (typeof window !== 'undefined') {
    window.removeEventListener('keydown', handleEscClose)
  }
})

defineExpose({ open, close })
</script>

<template>
  <!-- Trigger Button -->
  <button class="dashboard-customize-trigger" :title="t('dashboard.customize')" @click="open">
    <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path
        stroke-linecap="round"
        stroke-linejoin="round"
        stroke-width="2"
        d="M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4"
      />
    </svg>
    <span>{{ t('dashboard.customize') }}</span>
    <span class="dashboard-customize-badge">{{ totals.enabled }}/{{ totals.total }}</span>
  </button>

  <!-- Modal -->
  <Teleport to="body">
    <Transition name="fade">
      <div v-if="isOpen" class="dashboard-customize-layer">
        <!-- Backdrop -->
        <div class="dashboard-customize-backdrop" @click="close"></div>

        <!-- Modal Content -->
        <div class="dashboard-customize-modal">
          <!-- Header -->
          <div class="dashboard-customize-head">
            <div class="dashboard-customize-head-copy">
              <h2 class="dashboard-customize-title">{{ t('dashboard.customizeTitle') }}</h2>
              <p class="dashboard-customize-meta">
                {{ totals.enabled }}/{{ totals.total }} enabled
              </p>
            </div>
            <button class="dashboard-customize-close" @click="close">
              <svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M6 18L18 6M6 6l12 12"
                />
              </svg>
            </button>
          </div>

          <div class="dashboard-customize-frame">
            <!-- Category Tabs -->
            <div class="dashboard-customize-tabs">
              <button
                v-for="cat in categories"
                :key="cat.id"
                class="dashboard-customize-tab"
                :class="{ active: activeCategory === cat.id }"
                @click="activeCategory = cat.id as typeof activeCategory"
              >
                {{ tr(cat.labelKey, cat.id) }}
              </button>
            </div>

            <!-- Card List -->
            <div class="dashboard-customize-list">
              <div class="dashboard-customize-list-grid">
                <div
                  v-for="card in filteredCards"
                  :key="card.config.id"
                  class="dashboard-customize-item"
                  :class="{ enabled: card.enabled }"
                >
                  <div class="dashboard-customize-item-copy">
                    <div
                      class="dashboard-customize-item-dot"
                      :class="{ enabled: card.enabled }"
                    ></div>
                    <div>
                      <div class="dashboard-customize-item-title">
                        {{ tr(card.config.titleKey, card.config.id) }}
                      </div>
                      <div class="dashboard-customize-item-subtitle">
                        {{
                          tr(`dashboard.categories.${card.config.category}`, card.config.category)
                        }}
                      </div>
                    </div>
                  </div>
                  <label class="dashboard-customize-switch">
                    <input
                      type="checkbox"
                      :checked="card.enabled"
                      class="sr-only"
                      :aria-label="tr(card.config.titleKey, card.config.id)"
                      @change="toggleCard(card.config.id)"
                    />
                    <span class="dashboard-customize-switch-track">
                      <span class="dashboard-customize-switch-thumb"></span>
                    </span>
                  </label>
                </div>
              </div>
            </div>
          </div>

          <!-- Footer -->
          <div class="dashboard-customize-foot">
            <button class="dashboard-customize-reset" @click="resetToDefaults">
              {{ t('dashboard.resetToDefaults') }}
            </button>
            <button class="dashboard-customize-done" @click="close">
              {{ t('common.done') }}
            </button>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

.dashboard-customize-trigger {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  height: 2rem;
  border-radius: 0.68rem;
  border: 1px solid rgba(148, 163, 184, 0.45);
  background: rgba(255, 255, 255, 0.72);
  color: #334155;
  font-size: 0.8rem;
  font-weight: 600;
  padding: 0 0.68rem;
  transition:
    border-color 0.16s ease,
    background-color 0.16s ease;
}

.dashboard-customize-trigger:hover {
  border-color: rgba(100, 116, 139, 0.58);
  background: rgba(255, 255, 255, 0.96);
}

.dashboard-customize-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 2.4rem;
  padding: 0 0.34rem;
  height: 1.2rem;
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.2);
  color: #475569;
  font-size: 0.68rem;
  font-weight: 700;
}

.dashboard-customize-layer {
  position: fixed;
  inset: 0;
  z-index: 60;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 1rem;
}

.dashboard-customize-backdrop {
  position: absolute;
  inset: 0;
  background: rgba(2, 6, 23, 0.56);
}

.dashboard-customize-modal {
  position: relative;
  width: min(52rem, 100%);
  height: min(88vh, 48rem);
  max-height: min(88vh, 48rem);
  display: grid;
  grid-template-rows: auto minmax(0, 1fr) auto;
  overflow: hidden;
  border-radius: 1rem;
  border: 1px solid rgba(203, 213, 225, 0.82);
  background: #f8fafc;
  box-shadow:
    0 28px 60px -30px rgba(15, 23, 42, 0.55),
    inset 0 1px 0 rgba(255, 255, 255, 0.86);
}

.dashboard-customize-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.8rem;
  padding: 0.95rem 1rem 0.85rem;
  border-bottom: 1px solid rgba(203, 213, 225, 0.74);
}

.dashboard-customize-head-copy {
  display: flex;
  flex-direction: column;
  gap: 0.24rem;
}

.dashboard-customize-title {
  margin: 0;
  font-size: 1rem;
  line-height: 1.25;
  color: #0f172a;
  font-weight: 700;
}

.dashboard-customize-meta {
  margin: 0;
  font-size: 0.76rem;
  color: #64748b;
}

.dashboard-customize-close {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 2rem;
  height: 2rem;
  border-radius: 0.6rem;
  border: 1px solid rgba(148, 163, 184, 0.42);
  color: #64748b;
  background: rgba(255, 255, 255, 0.85);
}

.dashboard-customize-frame {
  min-height: 0;
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
}

.dashboard-customize-tabs {
  display: flex;
  gap: 0.4rem;
  padding: 0.75rem 1rem;
  border-bottom: 1px solid rgba(203, 213, 225, 0.74);
  overflow-x: auto;
}

.dashboard-customize-tab {
  height: 1.95rem;
  border-radius: 0.6rem;
  border: 1px solid transparent;
  padding: 0 0.72rem;
  white-space: nowrap;
  font-size: 0.76rem;
  font-weight: 600;
  color: #64748b;
  background: transparent;
}

.dashboard-customize-tab:hover {
  background: rgba(148, 163, 184, 0.14);
}

.dashboard-customize-tab.active {
  color: #0f172a;
  border-color: rgba(148, 163, 184, 0.46);
  background: rgba(148, 163, 184, 0.18);
}

.dashboard-customize-list {
  min-height: 0;
  overflow-y: auto;
  overscroll-behavior: contain;
  scrollbar-gutter: stable;
  padding: 0.95rem 1rem;
}

.dashboard-customize-list-grid {
  display: grid;
  gap: 0.54rem;
}

.dashboard-customize-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.8rem;
  padding: 0.72rem 0.82rem;
  border-radius: 0.8rem;
  border: 1px solid rgba(203, 213, 225, 0.72);
  background: #eef3f7;
}

.dashboard-customize-item.enabled {
  border-color: rgba(74, 222, 128, 0.4);
  background: #edf8f0;
}

.dashboard-customize-item-copy {
  display: flex;
  align-items: center;
  gap: 0.6rem;
}

.dashboard-customize-item-dot {
  width: 0.52rem;
  height: 0.52rem;
  border-radius: 999px;
  background: #94a3b8;
  flex-shrink: 0;
}

.dashboard-customize-item-dot.enabled {
  background: #16a34a;
}

.dashboard-customize-item-title {
  font-size: 0.84rem;
  font-weight: 600;
  color: #0f172a;
}

.dashboard-customize-item-subtitle {
  margin-top: 0.1rem;
  font-size: 0.68rem;
  color: #64748b;
}

.dashboard-customize-switch {
  display: inline-flex;
  align-items: center;
  cursor: pointer;
}

.dashboard-customize-switch-track {
  width: 2.65rem;
  height: 1.5rem;
  border-radius: 999px;
  border: 1px solid rgba(148, 163, 184, 0.55);
  background: rgba(148, 163, 184, 0.24);
  display: inline-flex;
  align-items: center;
  padding: 0 0.14rem;
  transition:
    background-color 0.16s ease,
    border-color 0.16s ease;
}

.dashboard-customize-switch-thumb {
  width: 1.1rem;
  height: 1.1rem;
  border-radius: 999px;
  background: #ffffff;
  box-shadow: 0 2px 7px -5px rgba(15, 23, 42, 0.9);
  transition: transform 0.16s ease;
}

.dashboard-customize-switch input:checked + .dashboard-customize-switch-track {
  background: rgba(34, 197, 94, 0.28);
  border-color: rgba(22, 163, 74, 0.52);
}

.dashboard-customize-switch
  input:checked
  + .dashboard-customize-switch-track
  .dashboard-customize-switch-thumb {
  transform: translateX(1.08rem);
}

.dashboard-customize-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.6rem;
  padding: 0.8rem 1rem;
  border-top: 1px solid rgba(203, 213, 225, 0.74);
  background: inherit;
  position: relative;
  z-index: 1;
}

.dashboard-customize-reset,
.dashboard-customize-done {
  height: 2rem;
  border-radius: 0.62rem;
  padding: 0 0.7rem;
  font-size: 0.78rem;
  font-weight: 600;
}

.dashboard-customize-reset {
  border: 1px solid rgba(248, 113, 113, 0.42);
  color: #b91c1c;
  background: rgba(252, 165, 165, 0.16);
}

.dashboard-customize-done {
  border: 1px solid rgba(148, 163, 184, 0.45);
  color: #0f172a;
  background: rgba(226, 232, 240, 0.75);
}

:root.dark .dashboard-customize-trigger,
[data-theme='dark'] .dashboard-customize-trigger {
  color: rgb(203 213 225);
  border-color: rgba(148, 163, 184, 0.5);
  background: rgba(30, 41, 59, 0.8);
}

:root.dark .dashboard-customize-badge,
[data-theme='dark'] .dashboard-customize-badge {
  background: rgba(100, 116, 139, 0.45);
  color: rgb(226 232 240);
}

:root.dark .dashboard-customize-modal,
[data-theme='dark'] .dashboard-customize-modal {
  border-color: rgba(100, 116, 139, 0.58);
  background: #243244;
  box-shadow:
    0 28px 70px -34px rgba(2, 6, 23, 0.92),
    inset 0 1px 0 rgba(148, 163, 184, 0.12);
}

:root.dark .dashboard-customize-head,
:root.dark .dashboard-customize-tabs,
:root.dark .dashboard-customize-foot,
[data-theme='dark'] .dashboard-customize-head,
[data-theme='dark'] .dashboard-customize-tabs,
[data-theme='dark'] .dashboard-customize-foot {
  border-color: rgba(100, 116, 139, 0.48);
}

:root.dark .dashboard-customize-title,
[data-theme='dark'] .dashboard-customize-title {
  color: rgb(241 245 249);
}

:root.dark .dashboard-customize-meta,
:root.dark .dashboard-customize-item-subtitle,
[data-theme='dark'] .dashboard-customize-meta,
[data-theme='dark'] .dashboard-customize-item-subtitle {
  color: rgb(148 163 184);
}

:root.dark .dashboard-customize-close,
[data-theme='dark'] .dashboard-customize-close {
  border-color: rgba(100, 116, 139, 0.58);
  color: rgb(203 213 225);
  background: rgba(30, 41, 59, 0.9);
}

:root.dark .dashboard-customize-tab,
[data-theme='dark'] .dashboard-customize-tab {
  color: rgb(148 163 184);
}

:root.dark .dashboard-customize-tab:hover,
[data-theme='dark'] .dashboard-customize-tab:hover {
  background: rgba(51, 65, 85, 0.6);
}

:root.dark .dashboard-customize-tab.active,
[data-theme='dark'] .dashboard-customize-tab.active {
  color: rgb(241 245 249);
  border-color: rgba(100, 116, 139, 0.64);
  background: rgba(51, 65, 85, 0.82);
}

:root.dark .dashboard-customize-item,
[data-theme='dark'] .dashboard-customize-item {
  border-color: rgba(100, 116, 139, 0.52);
  background: rgba(30, 41, 59, 0.9);
}

:root.dark .dashboard-customize-item.enabled,
[data-theme='dark'] .dashboard-customize-item.enabled {
  border-color: rgba(74, 222, 128, 0.35);
  background: rgba(22, 101, 52, 0.34);
}

:root.dark .dashboard-customize-item-title,
[data-theme='dark'] .dashboard-customize-item-title {
  color: rgb(226 232 240);
}

:root.dark .dashboard-customize-switch-track,
[data-theme='dark'] .dashboard-customize-switch-track {
  border-color: rgba(100, 116, 139, 0.65);
  background: rgba(51, 65, 85, 0.75);
}

:root.dark .dashboard-customize-switch input:checked + .dashboard-customize-switch-track,
[data-theme='dark'] .dashboard-customize-switch input:checked + .dashboard-customize-switch-track {
  background: rgba(22, 163, 74, 0.3);
  border-color: rgba(74, 222, 128, 0.42);
}

:root.dark .dashboard-customize-reset,
[data-theme='dark'] .dashboard-customize-reset {
  border-color: rgba(248, 113, 113, 0.42);
  color: rgb(252 165 165);
  background: rgba(127, 29, 29, 0.32);
}

:root.dark .dashboard-customize-done,
[data-theme='dark'] .dashboard-customize-done {
  border-color: rgba(100, 116, 139, 0.56);
  color: rgb(241 245 249);
  background: rgba(51, 65, 85, 0.85);
}

@media (max-width: 640px) {
  .dashboard-customize-trigger {
    height: 1.9rem;
    padding: 0 0.58rem;
    font-size: 0.74rem;
  }

  .dashboard-customize-layer {
    padding: 0.68rem;
  }

  .dashboard-customize-modal {
    width: 100%;
    height: 92vh;
    max-height: 92vh;
    border-radius: 0.86rem;
  }

  .dashboard-customize-head,
  .dashboard-customize-tabs,
  .dashboard-customize-list,
  .dashboard-customize-foot {
    padding-inline: 0.75rem;
  }

  .dashboard-customize-foot {
    flex-direction: column;
    align-items: stretch;
  }
}
</style>
