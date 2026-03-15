<script setup lang="ts">
import type { TypelessCardWeather } from '@/types/typeless'

const props = defineProps<{
  card: TypelessCardWeather
}>()

const weatherIcons: Record<string, string> = {
  sunny: '☀️',
  cloudy: '☁️',
  rainy: '🌧️',
  snowy: '❄️',
  stormy: '⛈️',
  foggy: '🌫️',
  windy: '💨',
  'partly-cloudy': '⛅',
}

function getTemperatureDisplay(): string {
  const unit = props.card.unit === 'fahrenheit' ? '°F' : '°C'
  return `${props.card.temperature}${unit}`
}

function getForecastIcon(condition: string): string {
  return weatherIcons[condition] || '🌡️'
}
</script>

<template>
  <div
    class="weather-card rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden bg-gradient-to-br from-gray-600 to-gray-800 dark:from-gray-700 dark:to-gray-900 text-white"
  >
    <!-- Main weather -->
    <div class="p-6">
      <div class="flex items-start justify-between">
        <div>
          <p class="text-sm opacity-90">{{ card.location }}</p>
          <p class="text-5xl font-light mt-2">{{ getTemperatureDisplay() }}</p>
          <p class="text-lg mt-2 capitalize">{{ card.condition.replace('-', ' ') }}</p>
        </div>
        <div class="text-6xl">
          {{ weatherIcons[card.condition] || '🌡️' }}
        </div>
      </div>

      <!-- Additional info -->
      <div
        v-if="card.humidity !== undefined || card.windSpeed !== undefined"
        class="mt-6 flex gap-6"
      >
        <div v-if="card.humidity !== undefined" class="flex items-center gap-2">
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-5 w-5 opacity-80"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M19.428 15.428a2 2 0 00-1.022-.547l-2.387-.477a6 6 0 00-3.86.517l-.318.158a6 6 0 01-3.86.517L6.05 15.21a2 2 0 00-1.806.547M8 4h8l-1 1v5.172a2 2 0 00.586 1.414l5 5c1.26 1.26.367 3.414-1.415 3.414H4.828c-1.782 0-2.674-2.154-1.414-3.414l5-5A2 2 0 009 10.172V5L8 4z"
            />
          </svg>
          <span class="text-sm">{{ card.humidity }}%</span>
        </div>
        <div v-if="card.windSpeed !== undefined" class="flex items-center gap-2">
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-5 w-5 opacity-80"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M14 5l7 7m0 0l-7 7m7-7H3"
            />
          </svg>
          <span class="text-sm">{{ card.windSpeed }} {{ card.windUnit || 'km/h' }}</span>
        </div>
      </div>
    </div>

    <!-- Forecast -->
    <div v-if="card.forecast?.length" class="px-6 pb-4">
      <div class="pt-4 border-t border-white/20">
        <div class="grid grid-cols-5 gap-2">
          <div v-for="(day, index) in card.forecast.slice(0, 5)" :key="index" class="text-center">
            <p class="text-xs opacity-80">{{ day.day }}</p>
            <p class="text-xl my-1">{{ getForecastIcon(day.condition) }}</p>
            <p class="text-xs">
              <span class="font-medium">{{ day.high }}°</span>
              <span class="opacity-70"> / {{ day.low }}°</span>
            </p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
