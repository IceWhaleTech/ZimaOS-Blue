<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import type { TypelessCardCountdown } from '@/types/typeless'

const { t } = useI18n()

const props = defineProps<{
  card: TypelessCardCountdown
}>()

const now = ref(Date.now())
let timer: ReturnType<typeof setInterval> | null = null

onMounted(() => {
  timer = setInterval(() => {
    now.value = Date.now()
  }, 1000)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})

const targetTime = computed(() => new Date(props.card.targetDate).getTime())

const timeLeft = computed(() => {
  const diff = targetTime.value - now.value
  if (diff <= 0) {
    return { days: 0, hours: 0, minutes: 0, seconds: 0, expired: true }
  }

  const days = Math.floor(diff / (1000 * 60 * 60 * 24))
  const hours = Math.floor((diff % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60))
  const minutes = Math.floor((diff % (1000 * 60 * 60)) / (1000 * 60))
  const seconds = Math.floor((diff % (1000 * 60)) / 1000)

  return { days, hours, minutes, seconds, expired: false }
})

const showDays = computed(() => props.card.showDays !== false)
const showHours = computed(() => props.card.showHours !== false)
const showMinutes = computed(() => props.card.showMinutes !== false)
const showSeconds = computed(() => props.card.showSeconds !== false)

function padZero(num: number): string {
  return num.toString().padStart(2, '0')
}
</script>

<template>
  <div
    class="countdown-card rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden bg-white dark:bg-gray-700"
    :class="{ 'p-4': card.variant === 'compact', 'p-6': card.variant !== 'compact' }"
  >
    <!-- Title -->
    <h4 v-if="card.title" class="font-medium text-gray-900 dark:text-white text-center mb-4">
      {{ card.title }}
    </h4>

    <!-- Expired state -->
    <div v-if="timeLeft.expired" class="text-center py-4">
      <p class="text-2xl font-bold text-gray-900 dark:text-white">
        {{ t('countdownCard.expired', "Time's up!") }}
      </p>
    </div>

    <!-- Countdown display -->
    <div v-else class="flex justify-center gap-3" :class="{ 'gap-2': card.variant === 'compact' }">
      <!-- Days -->
      <div v-if="showDays" class="text-center">
        <div
          class="bg-gray-100 dark:bg-gray-700 rounded-lg flex items-center justify-center font-mono font-bold text-gray-900 dark:text-white"
          :class="{
            'w-12 h-12 text-lg': card.variant === 'compact',
            'w-20 h-20 text-3xl': card.variant === 'large',
            'w-16 h-16 text-2xl': card.variant !== 'compact' && card.variant !== 'large',
          }"
        >
          {{ padZero(timeLeft.days) }}
        </div>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t('countdownCard.days', 'Days') }}
        </p>
      </div>

      <!-- Separator -->
      <div v-if="showDays && showHours" class="flex items-center text-2xl text-gray-400 font-bold">
        :
      </div>

      <!-- Hours -->
      <div v-if="showHours" class="text-center">
        <div
          class="bg-gray-100 dark:bg-gray-700 rounded-lg flex items-center justify-center font-mono font-bold text-gray-900 dark:text-white"
          :class="{
            'w-12 h-12 text-lg': card.variant === 'compact',
            'w-20 h-20 text-3xl': card.variant === 'large',
            'w-16 h-16 text-2xl': card.variant !== 'compact' && card.variant !== 'large',
          }"
        >
          {{ padZero(timeLeft.hours) }}
        </div>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t('countdownCard.hours', 'Hours') }}
        </p>
      </div>

      <!-- Separator -->
      <div
        v-if="showHours && showMinutes"
        class="flex items-center text-2xl text-gray-400 font-bold"
      >
        :
      </div>

      <!-- Minutes -->
      <div v-if="showMinutes" class="text-center">
        <div
          class="bg-gray-100 dark:bg-gray-700 rounded-lg flex items-center justify-center font-mono font-bold text-gray-900 dark:text-white"
          :class="{
            'w-12 h-12 text-lg': card.variant === 'compact',
            'w-20 h-20 text-3xl': card.variant === 'large',
            'w-16 h-16 text-2xl': card.variant !== 'compact' && card.variant !== 'large',
          }"
        >
          {{ padZero(timeLeft.minutes) }}
        </div>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t('countdownCard.minutes', 'Minutes') }}
        </p>
      </div>

      <!-- Separator -->
      <div
        v-if="showMinutes && showSeconds"
        class="flex items-center text-2xl text-gray-400 font-bold"
      >
        :
      </div>

      <!-- Seconds -->
      <div v-if="showSeconds" class="text-center">
        <div
          class="bg-gray-100 dark:bg-gray-700 rounded-lg flex items-center justify-center font-mono font-bold text-gray-900 dark:text-white"
          :class="{
            'w-12 h-12 text-lg': card.variant === 'compact',
            'w-20 h-20 text-3xl': card.variant === 'large',
            'w-16 h-16 text-2xl': card.variant !== 'compact' && card.variant !== 'large',
          }"
        >
          {{ padZero(timeLeft.seconds) }}
        </div>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t('countdownCard.seconds', 'Seconds') }}
        </p>
      </div>
    </div>

    <!-- Description -->
    <p v-if="card.description" class="mt-4 text-sm text-gray-500 dark:text-gray-400 text-center">
      {{ card.description }}
    </p>
  </div>
</template>
