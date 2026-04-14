<script setup lang="ts">
import type { TypelessCardComparison } from '@/types/typeless'

defineProps<{
  card: TypelessCardComparison
}>()
</script>

<template>
  <div
    class="comparison-card rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden bg-white dark:bg-gray-700"
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

    <div class="overflow-x-auto">
      <table class="w-full">
        <!-- Item headers -->
        <thead>
          <tr>
            <th
              class="p-4 text-start text-sm font-medium text-gray-500 dark:text-gray-400 bg-gray-50 dark:bg-gray-700/50 w-40"
            >
              Feature
            </th>
            <th
              v-for="(item, index) in card.items"
              :key="index"
              class="p-4 text-center min-w-[150px]"
              :class="{ 'bg-gray-700 dark:bg-gray-500/20': item.highlighted }"
            >
              <div class="flex flex-col items-center gap-2">
                <!-- Badge -->
                <span
                  v-if="item.badge"
                  class="px-2 py-0.5 text-xs font-medium rounded-full"
                  :class="
                    item.highlighted
                      ? 'bg-gray-700 dark:bg-gray-500 text-white'
                      : 'bg-gray-700 dark:bg-gray-500 text-gray-600 dark:text-gray-300'
                  "
                >
                  {{ item.badge }}
                </span>
                <!-- Image -->
                <img
                  v-if="item.image"
                  :src="item.image"
                  :alt="item.name"
                  class="w-16 h-16 object-contain"
                >
                <!-- Name -->
                <span class="font-medium text-gray-900 dark:text-white">{{ item.name }}</span>
                <!-- Price -->
                <span
                  v-if="item.price"
                  class="text-lg font-bold text-gray-900 dark:text-white"
                >{{
                  item.price
                }}</span>
              </div>
            </th>
          </tr>
        </thead>

        <!-- Features -->
        <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
          <tr
            v-for="(feature, fIndex) in card.features"
            :key="fIndex"
            class="hover:bg-gray-50 dark:hover:bg-gray-700/50"
          >
            <td class="p-4 text-sm text-gray-600 dark:text-gray-400 font-medium">
              {{ feature.name }}
            </td>
            <td
              v-for="(value, vIndex) in feature.values"
              :key="vIndex"
              class="p-4 text-center"
              :class="{ 'bg-gray-100 dark:bg-gray-700/10': card.items[vIndex]?.highlighted }"
            >
              <!-- Boolean value -->
              <template v-if="typeof value === 'boolean'">
                <svg
                  v-if="value"
                  xmlns="http://www.w3.org/2000/svg"
                  class="h-5 w-5 mx-auto text-green-500"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M5 13l4 4L19 7"
                  />
                </svg>
                <svg
                  v-else
                  xmlns="http://www.w3.org/2000/svg"
                  class="h-5 w-5 mx-auto text-gray-300 dark:text-gray-600"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M6 18L18 6M6 6l12 12"
                  />
                </svg>
              </template>
              <!-- String/Number value -->
              <span
                v-else
                class="text-sm text-gray-700 dark:text-gray-300"
              >{{ value }}</span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
