<script setup lang="ts">
import { computed } from 'vue'
import type { TypelessCardRating } from '@/types/typeless'

const props = defineProps<{
  card: TypelessCardRating
}>()

const maxRating = computed(() => props.card.maxRating || 5)
const fullStars = computed(() => Math.floor(props.card.rating))
const hasHalfStar = computed(() => props.card.rating % 1 >= 0.5)
const emptyStars = computed(() => maxRating.value - fullStars.value - (hasHalfStar.value ? 1 : 0))

function getBarWidth(percentage?: number, count?: number): string {
  if (percentage !== undefined) return `${percentage}%`
  if (count !== undefined && props.card.reviewCount) {
    return `${(count / props.card.reviewCount) * 100}%`
  }
  return '0%'
}
</script>

<template>
  <div
    class="rating-card rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden bg-white dark:bg-gray-700"
  >
    <!-- Title -->
    <div
      v-if="card.title"
      class="px-4 py-3 border-b border-gray-200 dark:border-gray-700"
    >
      <h4 class="font-medium text-gray-900 dark:text-white">
        {{ card.title }}
      </h4>
    </div>

    <div class="p-4">
      <!-- Main rating display -->
      <div class="flex items-center gap-6">
        <!-- Big number -->
        <div class="text-center">
          <p class="text-5xl font-bold text-gray-900 dark:text-white">
            {{ card.rating.toFixed(1) }}
          </p>
          <div class="flex items-center justify-center mt-2">
            <!-- Full stars -->
            <span
              v-for="i in fullStars"
              :key="'full-' + i"
              class="text-yellow-400 text-xl"
            >★</span>
            <!-- Half star -->
            <span
              v-if="hasHalfStar"
              class="text-yellow-400 text-xl"
            >☆</span>
            <!-- Empty stars -->
            <span
              v-for="i in emptyStars"
              :key="'empty-' + i"
              class="text-gray-300 dark:text-gray-600 text-xl"
            >★</span>
          </div>
          <p
            v-if="card.reviewCount"
            class="mt-1 text-sm text-gray-500 dark:text-gray-400"
          >
            {{ card.reviewCount.toLocaleString() }} reviews
          </p>
        </div>

        <!-- Breakdown bars -->
        <div
          v-if="card.breakdown?.length"
          class="flex-1 space-y-2"
        >
          <div
            v-for="item in card.breakdown"
            :key="item.stars"
            class="flex items-center gap-2"
          >
            <span class="text-sm text-gray-600 dark:text-gray-400 w-8">{{ item.stars }}★</span>
            <div class="flex-1 h-2 bg-gray-100 dark:bg-gray-700 rounded-full overflow-hidden">
              <div
                class="h-full bg-yellow-400 rounded-full transition-all duration-500"
                :style="{ width: getBarWidth(item.percentage, item.count) }"
              />
            </div>
            <span class="text-xs text-gray-500 dark:text-gray-400 w-10 text-end">
              {{ item.count?.toLocaleString() || `${item.percentage}%` }}
            </span>
          </div>
        </div>
      </div>

      <!-- Featured review -->
      <div
        v-if="card.review"
        class="mt-6 pt-4 border-t border-gray-200 dark:border-gray-700"
      >
        <div class="flex items-start gap-3">
          <div
            v-if="card.review.avatar"
            class="w-10 h-10 rounded-full overflow-hidden bg-gray-100 dark:bg-gray-700 flex-shrink-0"
          >
            <img
              :src="card.review.avatar"
              :alt="card.review.author"
              class="w-full h-full object-cover"
            >
          </div>
          <div
            v-else
            class="w-10 h-10 rounded-full bg-gradient-to-br from-gray-700 to-gray-900 flex items-center justify-center text-white font-medium flex-shrink-0"
          >
            {{ card.review.author?.[0]?.toUpperCase() ?? '?' }}
          </div>
          <div class="flex-1 min-w-0">
            <div class="flex items-center gap-2">
              <span class="font-medium text-gray-900 dark:text-white">{{
                card.review.author
              }}</span>
              <span
                v-if="card.review.date"
                class="text-xs text-gray-500 dark:text-gray-400"
              >{{
                card.review.date
              }}</span>
            </div>
            <p class="mt-1 text-sm text-gray-600 dark:text-gray-300">
              {{ card.review.content }}
            </p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
