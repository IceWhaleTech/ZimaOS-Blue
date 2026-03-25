<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const _props = defineProps<{
  modelValue: boolean
  src: string
  alt?: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

const scale = ref(1)
const position = ref({ x: 0, y: 0 })
const isDragging = ref(false)
const dragStart = ref({ x: 0, y: 0 })

function close() {
  emit('update:modelValue', false)
  // Reset state
  scale.value = 1
  position.value = { x: 0, y: 0 }
}

function handleWheel(event: WheelEvent) {
  event.preventDefault()
  const delta = event.deltaY > 0 ? -0.1 : 0.1
  scale.value = Math.max(0.5, Math.min(3, scale.value + delta))
}

function handleMouseDown(event: MouseEvent) {
  isDragging.value = true
  dragStart.value = { x: event.clientX - position.value.x, y: event.clientY - position.value.y }
}

function handleMouseMove(event: MouseEvent) {
  if (!isDragging.value) return
  position.value = {
    x: event.clientX - dragStart.value.x,
    y: event.clientY - dragStart.value.y,
  }
}

function handleMouseUp() {
  isDragging.value = false
}

function zoomIn() {
  scale.value = Math.min(3, scale.value + 0.25)
}

function zoomOut() {
  scale.value = Math.max(0.5, scale.value - 0.25)
}

function resetZoom() {
  scale.value = 1
  position.value = { x: 0, y: 0 }
}
</script>

<template>
  <Teleport to="body">
    <Transition name="fade">
      <div
        v-if="modelValue"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/90"
        @click.self="close"
        @wheel="handleWheel"
        @mousemove="handleMouseMove"
        @mouseup="handleMouseUp"
        @mouseleave="handleMouseUp"
      >
        <!-- Close button -->
        <button
          class="absolute top-4 end-4 p-2 rounded-lg bg-white/10 hover:bg-white/20 text-white transition-colors cursor-pointer z-10"
          @click="close"
        >
          <svg class="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M6 18L18 6M6 6l12 12"
            />
          </svg>
        </button>

        <!-- Zoom controls -->
        <div
          class="absolute bottom-4 start-1/2 -translate-x-1/2 flex items-center gap-2 bg-white/10 rounded-lg p-2 z-10"
        >
          <button
            class="p-2 rounded-lg hover:bg-white/20 text-white transition-colors cursor-pointer"
            :title="t('chat.imagePreview.zoomOut')"
            @click="zoomOut"
          >
            <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 12H4" />
            </svg>
          </button>
          <button
            class="px-3 py-1 rounded-lg hover:bg-white/20 text-white text-sm transition-colors cursor-pointer"
            :title="t('chat.imagePreview.reset')"
            @click="resetZoom"
          >
            {{ Math.round(scale * 100) }}%
          </button>
          <button
            class="p-2 rounded-lg hover:bg-white/20 text-white transition-colors cursor-pointer"
            :title="t('chat.imagePreview.zoomIn')"
            @click="zoomIn"
          >
            <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M12 4v16m8-8H4"
              />
            </svg>
          </button>
        </div>

        <!-- Image -->
        <img
          :src="src"
          :alt="alt"
          class="max-w-[90vw] max-h-[90vh] object-contain select-none"
          :class="{ 'cursor-grab': !isDragging, 'cursor-grabbing': isDragging }"
          :style="{
            transform: `translate(${position.x}px, ${position.y}px) scale(${scale})`,
            transition: isDragging ? 'none' : 'transform 0.2s ease',
          }"
          draggable="false"
          @mousedown="handleMouseDown"
        />
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.3s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
