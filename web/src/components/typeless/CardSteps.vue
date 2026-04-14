<script setup lang="ts">
import type { TypelessCardSteps, StepItem } from '@/types/typeless'

defineProps<{
  card: TypelessCardSteps
}>()

function getStepStatus(step: StepItem, index: number, currentStep?: number): string {
  if (step.status) return step.status
  if (currentStep === undefined) return 'pending'
  if (index < currentStep) return 'completed'
  if (index === currentStep) return 'current'
  return 'pending'
}

const statusColors: Record<string, string> = {
  pending: 'bg-gray-700 dark:bg-gray-500 text-gray-500 dark:text-gray-400',
  current: 'bg-gray-700 dark:bg-gray-500 text-white ring-4 ring-gray-200 dark:ring-gray-800',
  completed: 'bg-green-500 text-white',
  error: 'bg-red-500 text-white',
}

const lineColors: Record<string, string> = {
  pending: 'bg-gray-700 dark:bg-gray-500',
  current: 'bg-gray-700 dark:bg-gray-500',
  completed: 'bg-green-500',
  error: 'bg-red-500',
}
</script>

<template>
  <div
    class="steps-card rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden bg-white dark:bg-gray-700"
  >
    <!-- Title -->
    <div
      v-if="card.title"
      class="px-3 py-1.5 border-b border-gray-200 dark:border-gray-700"
    >
      <h4 class="text-sm font-medium text-gray-900 dark:text-white">
        {{ card.title }}
      </h4>
    </div>

    <!-- Horizontal Steps -->
    <div
      v-if="card.variant !== 'vertical'"
      class="p-6"
    >
      <div class="flex items-start">
        <template
          v-for="(step, index) in card.steps"
          :key="index"
        >
          <!-- Step -->
          <div class="flex flex-col items-center flex-1">
            <!-- Circle -->
            <div
              class="w-10 h-10 rounded-full flex items-center justify-center font-medium transition-all"
              :class="statusColors[getStepStatus(step, index, card.currentStep)]"
            >
              <template v-if="getStepStatus(step, index, card.currentStep) === 'completed'">
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  class="h-5 w-5"
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
              </template>
              <template v-else-if="getStepStatus(step, index, card.currentStep) === 'error'">
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  class="h-5 w-5"
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
              <template v-else-if="step.icon">
                {{ step.icon }}
              </template>
              <template v-else>
                {{ index + 1 }}
              </template>
            </div>
            <!-- Title & Description -->
            <div class="mt-3 text-center">
              <p
                class="text-sm font-medium"
                :class="
                  getStepStatus(step, index, card.currentStep) === 'current'
                    ? 'text-gray-900 dark:text-white dark:text-white'
                    : 'text-gray-900 dark:text-white'
                "
              >
                {{ step.title }}
              </p>
              <p
                v-if="step.description"
                class="mt-1 text-xs text-gray-500 dark:text-gray-400 max-w-[120px]"
              >
                {{ step.description }}
              </p>
            </div>
          </div>
          <!-- Connector line -->
          <div
            v-if="index < card.steps.length - 1"
            class="flex-1 h-0.5 mt-5 mx-2"
            :class="lineColors[getStepStatus(step, index, card.currentStep)]"
          />
        </template>
      </div>
    </div>

    <!-- Vertical Steps -->
    <div
      v-else
      class="px-3 py-2"
    >
      <div class="relative">
        <template
          v-for="(step, index) in card.steps"
          :key="index"
        >
          <div class="flex gap-3 pb-3 last:pb-0">
            <!-- Circle and line -->
            <div class="flex flex-col items-center">
              <div
                class="w-6 h-6 rounded-full flex items-center justify-center text-xs font-medium flex-shrink-0"
                :class="statusColors[getStepStatus(step, index, card.currentStep)]"
              >
                <template v-if="getStepStatus(step, index, card.currentStep) === 'completed'">
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    class="h-3 w-3"
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
                </template>
                <template v-else-if="step.icon">
                  {{ step.icon }}
                </template>
                <template v-else>
                  {{ index + 1 }}
                </template>
              </div>
              <!-- Vertical line -->
              <div
                v-if="index < card.steps.length - 1"
                class="w-0.5 flex-1 mt-2"
                :class="lineColors[getStepStatus(step, index, card.currentStep)]"
              />
            </div>
            <!-- Content -->
            <div class="flex-1 pt-1">
              <p
                class="text-sm font-medium"
                :class="
                  getStepStatus(step, index, card.currentStep) === 'current'
                    ? 'text-gray-900 dark:text-white'
                    : 'text-gray-900 dark:text-white'
                "
              >
                {{ step.title }}
              </p>
              <div
                v-if="step.description"
                class="mt-1 flex flex-wrap gap-1"
              >
                <span
                  v-for="(tag, ti) in step.description.split('|')"
                  :key="ti"
                  class="inline-block px-1.5 py-0.5 text-xs rounded bg-gray-100 dark:bg-gray-600 text-gray-500 dark:text-gray-300"
                >
                  {{ tag }}
                </span>
              </div>
            </div>
          </div>
        </template>
      </div>
    </div>
  </div>
</template>
