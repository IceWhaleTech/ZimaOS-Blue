<script setup lang="ts">
import { ref } from 'vue'
import type { TypelessCardDetection, Detection } from '@/types/typeless'

defineProps<{
  card: TypelessCardDetection
}>()

const imageLoaded = ref(false)
const selectedDetection = ref<Detection | null>(null)

// Generate colors for detections
const detectionColors = [
  '#ef4444', // red
  '#f97316', // orange
  '#eab308', // yellow
  '#22c55e', // green
  '#06b6d4', // cyan
  '#3b82f6', // blue
  '#8b5cf6', // violet
  '#ec4899', // pink
]

function getDetectionColor(detection: Detection, index: number): string {
  if (detection.color) return detection.color
  return detectionColors[index % detectionColors.length] || '#3b82f6'
}

function getBoxStyle(detection: Detection, index: number) {
  const bbox = detection.bbox
  const unit = bbox.unit || 'percent'
  const color = getDetectionColor(detection, index)

  if (unit === 'percent') {
    return {
      left: `${bbox.x}%`,
      top: `${bbox.y}%`,
      width: `${bbox.width}%`,
      height: `${bbox.height}%`,
      borderColor: color,
      '--detection-color': color,
    }
  } else {
    return {
      left: `${bbox.x}px`,
      top: `${bbox.y}px`,
      width: `${bbox.width}px`,
      height: `${bbox.height}px`,
      borderColor: color,
      '--detection-color': color,
    }
  }
}

function formatConfidence(confidence?: number): string {
  if (confidence === undefined) return ''
  return `${Math.round(confidence * 100)}%`
}

function handleImageLoad() {
  imageLoaded.value = true
}

function handleDetectionClick(detection: Detection) {
  if (selectedDetection.value === detection) {
    selectedDetection.value = null
  } else {
    selectedDetection.value = detection
  }
}
</script>

<template>
  <div class="detection-card rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden bg-white dark:bg-gray-800">
    <!-- Title -->
    <div v-if="card.title" class="px-4 py-2 border-b border-gray-200 dark:border-gray-700">
      <h4 class="font-medium text-gray-900 dark:text-white">{{ card.title }}</h4>
    </div>

    <!-- Image with detection boxes -->
    <div class="relative">
      <img
        ref="imageRef"
        :src="card.image"
        class="w-full h-auto"
        @load="handleImageLoad"
      />

      <!-- Detection boxes overlay -->
      <div v-if="imageLoaded" class="absolute inset-0">
        <div
          v-for="(detection, index) in card.detections"
          :key="index"
          class="detection-box absolute border-2 cursor-pointer transition-all duration-200"
          :class="{ 'selected': selectedDetection === detection }"
          :style="getBoxStyle(detection, index)"
          @click="handleDetectionClick(detection)"
        >
          <!-- Label badge -->
          <div
            class="detection-label absolute -top-6 left-0 px-2 py-0.5 text-xs font-medium text-white rounded whitespace-nowrap"
            :style="{ backgroundColor: getDetectionColor(detection, index) }"
          >
            {{ detection.label }}
            <span v-if="detection.confidence" class="ml-1 opacity-80">
              {{ formatConfidence(detection.confidence) }}
            </span>
          </div>

          <!-- Corner markers with breathing effect -->
          <div class="corner-marker top-left" :style="{ backgroundColor: getDetectionColor(detection, index) }" />
          <div class="corner-marker top-right" :style="{ backgroundColor: getDetectionColor(detection, index) }" />
          <div class="corner-marker bottom-left" :style="{ backgroundColor: getDetectionColor(detection, index) }" />
          <div class="corner-marker bottom-right" :style="{ backgroundColor: getDetectionColor(detection, index) }" />
        </div>
      </div>
    </div>

    <!-- Detection list -->
    <div v-if="card.detections.length > 0" class="px-4 py-3 border-t border-gray-200 dark:border-gray-700">
      <div class="flex flex-wrap gap-2">
        <button
          v-for="(detection, index) in card.detections"
          :key="index"
          class="detection-tag px-2 py-1 text-xs rounded-full text-white transition-all duration-200"
          :class="{ 'ring-2 ring-offset-2 ring-offset-white dark:ring-offset-gray-800': selectedDetection === detection }"
          :style="{ backgroundColor: getDetectionColor(detection, index) }"
          @click="handleDetectionClick(detection)"
        >
          {{ detection.label }}
          <span v-if="detection.confidence" class="ml-1 opacity-80">
            {{ formatConfidence(detection.confidence) }}
          </span>
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.detection-box {
  animation: breathe 2s ease-in-out infinite;
}

.detection-box.selected {
  animation: breathe-selected 1.5s ease-in-out infinite;
  box-shadow: 0 0 0 4px var(--detection-color, #3b82f6);
}

@keyframes breathe {
  0%, 100% {
    opacity: 0.7;
    transform: scale(1);
  }
  50% {
    opacity: 1;
    transform: scale(1.01);
  }
}

@keyframes breathe-selected {
  0%, 100% {
    opacity: 1;
    box-shadow: 0 0 0 2px var(--detection-color, #3b82f6);
  }
  50% {
    opacity: 1;
    box-shadow: 0 0 0 6px var(--detection-color, #3b82f6);
  }
}

.corner-marker {
  position: absolute;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  animation: pulse 2s ease-in-out infinite;
}

.corner-marker.top-left {
  top: -4px;
  left: -4px;
}

.corner-marker.top-right {
  top: -4px;
  right: -4px;
}

.corner-marker.bottom-left {
  bottom: -4px;
  left: -4px;
}

.corner-marker.bottom-right {
  bottom: -4px;
  right: -4px;
}

@keyframes pulse {
  0%, 100% {
    transform: scale(1);
    opacity: 1;
  }
  50% {
    transform: scale(1.3);
    opacity: 0.7;
  }
}

.detection-label {
  pointer-events: none;
}

.detection-tag:hover {
  transform: scale(1.05);
}
</style>
