<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'

interface Props {
  // Total number of items
  itemCount: number
  // Estimated height of each item (used for initial calculation)
  estimatedItemHeight?: number
  // Number of items to render above/below visible area
  overscan?: number
  // Container height (if not using parent height)
  height?: number | string
}

const props = withDefaults(defineProps<Props>(), {
  estimatedItemHeight: 100,
  overscan: 3,
})

const emit = defineEmits<{
  visibleRangeChange: [start: number, end: number]
}>()

// Refs
const containerRef = ref<HTMLElement | null>(null)
const scrollTop = ref(0)
const containerHeight = ref(0)

// Item height cache (for variable height items)
const itemHeights = ref<Map<number, number>>(new Map())

// Get actual or estimated height for an item
function getItemHeight(index: number): number {
  return itemHeights.value.get(index) ?? props.estimatedItemHeight
}

// Calculate total height of all items
const totalHeight = computed(() => {
  let height = 0
  for (let i = 0; i < props.itemCount; i++) {
    height += getItemHeight(i)
  }
  return height
})

// Calculate visible range
const visibleRange = computed(() => {
  if (!containerHeight.value) {
    return { start: 0, end: Math.min(10, props.itemCount) }
  }

  let start = 0
  let accumulatedHeight = 0

  // Find start index
  for (let i = 0; i < props.itemCount; i++) {
    const itemHeight = getItemHeight(i)
    if (accumulatedHeight + itemHeight > scrollTop.value) {
      start = i
      break
    }
    accumulatedHeight += itemHeight
  }

  // Apply overscan
  start = Math.max(0, start - props.overscan)

  // Find end index
  let end = start
  accumulatedHeight = 0
  for (let i = start; i < props.itemCount; i++) {
    accumulatedHeight += getItemHeight(i)
    end = i + 1
    if (accumulatedHeight > containerHeight.value + props.estimatedItemHeight * props.overscan) {
      break
    }
  }

  // Apply overscan
  end = Math.min(props.itemCount, end + props.overscan)

  return { start, end }
})

// Calculate offset for the first visible item
const offsetTop = computed(() => {
  let offset = 0
  for (let i = 0; i < visibleRange.value.start; i++) {
    offset += getItemHeight(i)
  }
  return offset
})

// Visible items
const visibleItems = computed(() => {
  const items: number[] = []
  for (let i = visibleRange.value.start; i < visibleRange.value.end; i++) {
    items.push(i)
  }
  return items
})

// Handle scroll
function handleScroll(event: Event) {
  const target = event.target as HTMLElement
  scrollTop.value = target.scrollTop
}

// Update item height (called from slot)
function updateItemHeight(index: number, height: number) {
  if (itemHeights.value.get(index) !== height) {
    itemHeights.value.set(index, height)
  }
}

// Scroll to item
function scrollToItem(index: number, behavior: 'auto' | 'smooth' = 'auto') {
  if (!containerRef.value) return

  let offset = 0
  for (let i = 0; i < index; i++) {
    offset += getItemHeight(i)
  }

  containerRef.value.scrollTo({ top: offset, behavior })
}

// Scroll to bottom
function scrollToBottom(behavior: 'auto' | 'smooth' = 'auto') {
  if (!containerRef.value) return
  containerRef.value.scrollTo({ top: totalHeight.value, behavior })
}

// Expose methods
defineExpose({
  scrollToItem,
  scrollToBottom,
  updateItemHeight,
})

// Watch for visible range changes
watch(visibleRange, (range) => {
  emit('visibleRangeChange', range.start, range.end)
})

// Setup resize observer
let resizeObserver: ResizeObserver | null = null

onMounted(() => {
  if (containerRef.value) {
    containerHeight.value = containerRef.value.clientHeight

    resizeObserver = new ResizeObserver((entries) => {
      for (const entry of entries) {
        containerHeight.value = entry.contentRect.height
      }
    })
    resizeObserver.observe(containerRef.value)
  }
})

onUnmounted(() => {
  if (resizeObserver) {
    resizeObserver.disconnect()
  }
})
</script>

<template>
  <div
    ref="containerRef"
    class="virtual-scroll-container"
    :style="{ height: typeof height === 'number' ? `${height}px` : height }"
    @scroll="handleScroll"
  >
    <!-- Spacer for total height -->
    <div
      class="virtual-scroll-spacer"
      :style="{ height: `${totalHeight}px` }"
    >
      <!-- Visible items container -->
      <div
        class="virtual-scroll-content"
        :style="{ transform: `translateY(${offsetTop}px)` }"
      >
        <slot
          v-for="index in visibleItems"
          :key="index"
          :index="index"
          :update-height="(height: number) => updateItemHeight(index, height)"
        />
      </div>
    </div>
  </div>
</template>

<style scoped>
.virtual-scroll-container {
  overflow-y: auto;
  position: relative;
}

.virtual-scroll-spacer {
  position: relative;
}

.virtual-scroll-content {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
}
</style>
