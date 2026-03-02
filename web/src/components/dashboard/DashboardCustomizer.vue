<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useDashboardStore } from '@/stores/dashboard'
import { cardRegistry } from './cardRegistry'

const { t, te } = useI18n()
const dashboardStore = useDashboardStore()

const isOpen = ref(false)
const activeCategory = ref<'all' | 'overview' | 'system' | 'metrics'>('all')

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
  const cards = cardRegistry.map((config) => {
    const state = dashboardStore.cardStates.find((s) => s.id === config.id)
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

defineExpose({ open, close })
</script>

<template>
  <!-- Trigger Button -->
  <button
    class="p-2 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
    :title="t('dashboard.customize')"
    @click="open"
  >
    <svg class="w-5 h-5 text-gray-500 dark:text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path
        stroke-linecap="round"
        stroke-linejoin="round"
        stroke-width="2"
        d="M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4"
      />
    </svg>
  </button>

  <!-- Modal -->
  <Teleport to="body">
    <Transition name="fade">
      <div v-if="isOpen" class="fixed inset-0 z-50 flex items-center justify-center p-4">
        <!-- Backdrop -->
        <div class="absolute inset-0 bg-black/50" @click="close"></div>

        <!-- Modal Content -->
        <div class="relative bg-white dark:bg-gray-700 rounded-xl shadow-xl w-full max-w-2xl max-h-[80vh] flex flex-col">
          <!-- Header -->
          <div class="flex items-center justify-between p-4 border-b border-gray-200 dark:border-gray-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('dashboard.customizeTitle') }}
            </h2>
            <button
              class="p-1 hover:bg-gray-100 dark:hover:bg-gray-700 rounded transition-colors"
              @click="close"
            >
              <svg class="w-5 h-5 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>

          <!-- Category Tabs -->
          <div class="sticky top-0 bg-white dark:bg-gray-700 flex gap-2 p-4 border-b border-gray-200 dark:border-gray-700 overflow-x-auto z-10">
            <button
              v-for="cat in categories"
              :key="cat.id"
              class="px-3 py-1.5 text-sm font-medium rounded-lg transition-colors whitespace-nowrap"
              :class="
                activeCategory === cat.id
                  ? 'bg-gray-100 dark:bg-gray-700/30 text-gray-900 dark:text-gray-300'
                  : 'text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700'
              "
              @click="activeCategory = cat.id as typeof activeCategory"
            >
              {{ tr(cat.labelKey, cat.id) }}
            </button>
          </div>

          <!-- Card List -->
          <div class="flex-1 overflow-y-auto p-4">
            <div class="grid gap-3">
              <div
                v-for="card in filteredCards"
                :key="card.config.id"
                class="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-700/50 rounded-lg"
              >
                <div class="flex items-center gap-3">
                  <div
                    class="w-2 h-2 rounded-full"
                    :class="card.enabled ? 'bg-green-500' : 'bg-gray-400'"
                  ></div>
                  <div>
                    <div class="font-medium text-gray-900 dark:text-white">
                      {{ tr(card.config.titleKey, card.config.id) }}
                    </div>
                    <div class="text-xs text-gray-500 dark:text-gray-400">
                      {{ tr(`dashboard.categories.${card.config.category}`, card.config.category) }}
                    </div>
                  </div>
                </div>
                <label class="relative inline-flex items-center cursor-pointer">
                  <input
                    type="checkbox"
                    :checked="card.enabled"
                    class="sr-only peer"
                    @change="toggleCard(card.config.id)"
                  />
                  <div
                    class="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-gray-900 dark:focus:ring-gray-400 dark:peer-focus:ring-gray-900 dark:focus:ring-gray-400 rounded-full peer dark:bg-gray-600 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-green-600 dark:peer-checked:bg-green-500"
                  ></div>
                </label>
              </div>
            </div>
          </div>

          <!-- Footer -->
          <div class="flex items-center justify-between p-4 border-t border-gray-200 dark:border-gray-700">
            <button
              class="px-4 py-2 text-sm text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg transition-colors"
              @click="resetToDefaults"
            >
              {{ t('dashboard.resetToDefaults') }}
            </button>
            <button
              class="px-4 py-2 text-sm bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg transition-colors"
              @click="close"
            >
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
</style>
