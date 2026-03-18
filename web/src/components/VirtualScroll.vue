<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, onUpdated, watch } from 'vue'

interface Props {
  // Total number of items
  itemCount: number
  // Optional source items for slot access
  items?: any[]
  // Estimated height of each item (used for initial calculation)
  estimatedItemHeight?: number
  // Number of items to render above/below visible area
  overscan?: number
  // Container height (if not using parent height)
  height?: number | string
  // External scroll container to reuse instead of creating a nested scrollbar
  scrollContainer?: HTMLElement | null
}

const props = withDefaults(defineProps<Props>(), {
  estimatedItemHeight: 100,
  overscan: 3,
})

const emit = defineEmits<{
  visibleRangeChange: [start: number, end: number]
}>()

class FenwickTree {
  private tree: number[] = []
  private n = 0

  build(values: number[]) {
    this.n = values.length
    this.tree = new Array(this.n + 1).fill(0)
    for (let i = 0; i < this.n; i++) {
      this.add(i, values[i] ?? 0)
    }
  }

  add(index: number, delta: number) {
    if (delta === 0 || index < 0 || index >= this.n) return
    let i = index + 1
    while (i <= this.n) {
      this.tree[i] = (this.tree[i] ?? 0) + delta
      i += i & -i
    }
  }

  resize(newSize: number, getValue: (index: number) => number) {
    if (newSize === this.n) return

    if (newSize <= 0) {
      this.n = 0
      this.tree = [0]
      return
    }

    // Shrink/reflow path: rebuild is simpler and still uncommon.
    if (newSize < this.n) {
      const values = new Array(newSize)
      for (let i = 0; i < newSize; i++) {
        values[i] = getValue(i)
      }
      this.build(values)
      return
    }

    // Grow path: append only new indexes, avoid O(n) full rebuild.
    const oldSize = this.n
    if (oldSize === 0) {
      const values = new Array(newSize)
      for (let i = 0; i < newSize; i++) {
        values[i] = getValue(i)
      }
      this.build(values)
      return
    }

    this.tree.length = newSize + 1
    for (let i = oldSize + 1; i <= newSize; i++) {
      this.tree[i] = this.tree[i] ?? 0
    }
    this.n = newSize

    for (let i = oldSize; i < newSize; i++) {
      this.add(i, getValue(i))
    }
  }

  sum(endExclusive: number) {
    if (endExclusive <= 0) return 0
    const end = Math.min(endExclusive, this.n)
    let i = end
    let result = 0
    while (i > 0) {
      result += this.tree[i] ?? 0
      i -= i & -i
    }
    return result
  }

  total() {
    return this.sum(this.n)
  }

  // Returns smallest index i where prefixSum(i + 1) > target.
  // If target >= total sum, returns n.
  lowerBound(target: number) {
    if (this.n <= 0) return 0
    if (target < 0) return 0
    const total = this.total()
    if (target >= total) return this.n

    let idx = 0
    let bit = 1
    while (bit << 1 <= this.n) bit <<= 1

    let current = 0
    while (bit > 0) {
      const next = idx + bit
      if (next <= this.n && current + (this.tree[next] ?? 0) <= target) {
        idx = next
        current += this.tree[next] ?? 0
      }
      bit >>= 1
    }
    return idx
  }
}

// Refs
const containerRef = ref<HTMLElement | null>(null)
const activeScrollContainer = ref<HTMLElement | null>(null)
const scrollTop = ref(0)
const containerHeight = ref(0)
const layoutVersion = ref(0)
let layoutVersionRafId: number | null = null
const usesExternalScroll = computed(() => Boolean(props.scrollContainer))

// Item height cache (for variable height items)
const itemHeights = ref<number[]>([])
const heightTree = new FenwickTree()
const updateHeightHandlers = new Map<number, (height: number) => void>()

function getUpdateHeightHandler(index: number): (height: number) => void {
  const cached = updateHeightHandlers.get(index)
  if (cached) return cached

  const handler = (height: number) => {
    updateItemHeight(index, height)
  }
  updateHeightHandlers.set(index, handler)
  return handler
}

function pruneUpdateHeightHandlers(maxExclusive: number) {
  if (updateHeightHandlers.size === 0) return
  for (const index of updateHeightHandlers.keys()) {
    if (index >= maxExclusive) {
      updateHeightHandlers.delete(index)
    }
  }
}

function bumpLayoutVersion() {
  layoutVersion.value++
}

function scheduleLayoutVersionBump() {
  if (layoutVersionRafId !== null) return
  layoutVersionRafId = window.requestAnimationFrame(() => {
    layoutVersionRafId = null
    bumpLayoutVersion()
  })
}

function rebuildHeightTree() {
  const heights = new Array(props.itemCount)
  for (let i = 0; i < props.itemCount; i++) {
    heights[i] = itemHeights.value[i] ?? props.estimatedItemHeight
  }
  itemHeights.value = heights
  heightTree.build(heights)
  bumpLayoutVersion()
}

watch(
  () => [props.itemCount, props.estimatedItemHeight] as const,
  ([itemCount, estimatedHeight], oldValue) => {
    const prevCount = oldValue?.[0]
    const prevEstimated = oldValue?.[1]

    if (prevCount === undefined || prevEstimated === undefined) {
      rebuildHeightTree()
      return
    }

    if (itemCount === prevCount && estimatedHeight === prevEstimated) return

    // Item height estimate changed or count shrank: rebuild for correctness.
    if (estimatedHeight !== prevEstimated || itemCount < prevCount) {
      if (itemCount < prevCount) {
        pruneUpdateHeightHandlers(itemCount)
      }
      rebuildHeightTree()
      return
    }

    // Append-only growth path.
    if (itemHeights.value.length > itemCount) {
      itemHeights.value.length = itemCount
    }
    for (let i = prevCount; i < itemCount; i++) {
      itemHeights.value[i] = estimatedHeight
    }
    heightTree.resize(itemCount, (index) => itemHeights.value[index] ?? estimatedHeight)
    bumpLayoutVersion()
  },
  { immediate: true }
)

// Get actual or estimated height for an item
function getItemHeight(index: number): number {
  return itemHeights.value[index] ?? props.estimatedItemHeight
}

// Calculate total height of all items
const totalHeight = computed(() => {
  layoutVersion.value
  return heightTree.total()
})

// Calculate visible range
const visibleRange = computed(() => {
  layoutVersion.value
  if (!containerHeight.value) {
    return { start: 0, end: Math.min(10, props.itemCount) }
  }

  const rawStart = heightTree.lowerBound(scrollTop.value)
  const start = Math.max(0, rawStart - props.overscan)

  const viewportBottom = scrollTop.value + containerHeight.value
  const rawEnd = heightTree.lowerBound(viewportBottom)
  const end = Math.min(props.itemCount, rawEnd + 1 + props.overscan)

  return { start, end }
})

// Calculate offset for the first visible item
const offsetTop = computed(() => {
  layoutVersion.value
  return heightTree.sum(visibleRange.value.start)
})

const visibleCount = computed(() => Math.max(0, visibleRange.value.end - visibleRange.value.start))

let emitRangeRafId: number | null = null
let pendingRange: { start: number; end: number } | null = null
let scrollRafId: number | null = null
let pendingScrollContainer: HTMLElement | null = null
const SCROLL_ANCHOR_EPSILON = 1

function scheduleEmitVisibleRange(start: number, end: number) {
  pendingRange = { start, end }
  if (emitRangeRafId !== null) return
  emitRangeRafId = window.requestAnimationFrame(() => {
    emitRangeRafId = null
    if (!pendingRange) return
    emit('visibleRangeChange', pendingRange.start, pendingRange.end)
    pendingRange = null
  })
}

// Handle scroll
function handleScroll(event: Event) {
  pendingScrollContainer = event.target as HTMLElement
  if (scrollRafId !== null) return
  scrollRafId = window.requestAnimationFrame(() => {
    scrollRafId = null
    syncScrollMetrics(pendingScrollContainer)
    pendingScrollContainer = null
  })
}

function applyScrollAnchorDelta(
  delta: number,
  scrollContainer: HTMLElement | null = getScrollContainer()
) {
  if (!scrollContainer || delta === 0) return

  const nextScrollTop = Math.max(0, scrollContainer.scrollTop + delta)
  if (scrollContainer.scrollTop !== nextScrollTop) {
    scrollContainer.scrollTop = nextScrollTop
  }

  const nextViewportTop = Math.max(0, scrollTop.value + delta)
  if (scrollTop.value !== nextViewportTop) {
    scrollTop.value = nextViewportTop
  }
}

// Update item height (called from slot)
function updateItemHeight(index: number, height: number) {
  if (index < 0 || index >= props.itemCount) return
  const prev = getItemHeight(index)
  if (prev === height) return
  const delta = height - prev
  const itemTop = heightTree.sum(index)
  const itemBottom = itemTop + prev
  const shouldPreserveAnchor = itemBottom <= scrollTop.value + SCROLL_ANCHOR_EPSILON

  itemHeights.value[index] = height
  heightTree.add(index, delta)
  if (shouldPreserveAnchor) {
    applyScrollAnchorDelta(delta)
  }
  scheduleLayoutVersionBump()
}

// Scroll to item
function scrollToItem(index: number, behavior: 'auto' | 'smooth' = 'auto') {
  const scrollContainer = getScrollContainer()
  if (!scrollContainer) return
  const offset = heightTree.sum(index)
  if (scrollContainer === containerRef.value) {
    scrollContainer.scrollTo({ top: offset, behavior })
    return
  }

  scrollContainer.scrollTo({ top: getContainerOffset(scrollContainer) + offset, behavior })
}

// Scroll to bottom
function scrollToBottom(behavior: 'auto' | 'smooth' = 'auto') {
  const scrollContainer = getScrollContainer()
  if (!scrollContainer) return
  if (scrollContainer === containerRef.value) {
    scrollContainer.scrollTo({ top: totalHeight.value, behavior })
    return
  }

  scrollContainer.scrollTo({ top: scrollContainer.scrollHeight, behavior })
}

function getContainer() {
  return getScrollContainer()
}

function getScrollContainer() {
  return props.scrollContainer ?? containerRef.value
}

function getContainerOffset(scrollContainer: HTMLElement) {
  if (!containerRef.value || scrollContainer === containerRef.value) return 0

  const containerRect = containerRef.value.getBoundingClientRect()
  const scrollRect = scrollContainer.getBoundingClientRect()
  return containerRect.top - scrollRect.top + scrollContainer.scrollTop
}

function syncScrollMetrics(scrollContainer: HTMLElement | null = getScrollContainer()) {
  if (!scrollContainer) {
    if (scrollTop.value !== 0) scrollTop.value = 0
    if (containerHeight.value !== 0) containerHeight.value = 0
    return
  }

  if (scrollContainer === containerRef.value) {
    if (scrollTop.value !== scrollContainer.scrollTop) {
      scrollTop.value = scrollContainer.scrollTop
    }
    if (containerHeight.value !== scrollContainer.clientHeight) {
      containerHeight.value = scrollContainer.clientHeight
    }
    return
  }

  const topOffset = getContainerOffset(scrollContainer)
  const viewportTop = Math.max(0, scrollContainer.scrollTop - topOffset)
  const viewportBottom = Math.max(
    0,
    scrollContainer.scrollTop + scrollContainer.clientHeight - topOffset
  )
  const nextContainerHeight = Math.max(0, viewportBottom - viewportTop)

  if (scrollTop.value !== viewportTop) {
    scrollTop.value = viewportTop
  }
  if (containerHeight.value !== nextContainerHeight) {
    containerHeight.value = nextContainerHeight
  }
}

// Expose methods
defineExpose({
  scrollToItem,
  scrollToBottom,
  updateItemHeight,
  getContainer,
})

// Watch for visible range changes
watch(
  [() => visibleRange.value.start, () => visibleRange.value.end],
  ([start, end]) => {
    scheduleEmitVisibleRange(start, end)
  },
  { immediate: true }
)

// Setup resize observer
let resizeObserver: ResizeObserver | null = null
const observedResizeTargets = new Set<HTMLElement>()

function syncResizeObserverTargets() {
  if (!resizeObserver) return

  const nextTargets = new Set<HTMLElement>()
  if (containerRef.value) nextTargets.add(containerRef.value)
  if (activeScrollContainer.value) nextTargets.add(activeScrollContainer.value)

  for (const target of observedResizeTargets) {
    if (!nextTargets.has(target)) {
      resizeObserver.unobserve(target)
    }
  }
  for (const target of nextTargets) {
    if (!observedResizeTargets.has(target)) {
      resizeObserver.observe(target)
    }
  }

  observedResizeTargets.clear()
  nextTargets.forEach((target) => observedResizeTargets.add(target))
}

function syncScrollContainerBinding() {
  const nextContainer = getScrollContainer()
  if (activeScrollContainer.value !== nextContainer) {
    activeScrollContainer.value?.removeEventListener('scroll', handleScroll)
    activeScrollContainer.value = nextContainer
    activeScrollContainer.value?.addEventListener('scroll', handleScroll, { passive: true })
  }

  syncResizeObserverTargets()
  syncScrollMetrics(nextContainer)
}

onMounted(() => {
  resizeObserver = new ResizeObserver(() => {
    syncScrollMetrics()
  })
  syncScrollContainerBinding()
})

onUpdated(() => {
  syncScrollMetrics()
})

watch([containerRef, () => props.scrollContainer], () => {
  syncScrollContainerBinding()
})

onUnmounted(() => {
  if (layoutVersionRafId !== null) {
    window.cancelAnimationFrame(layoutVersionRafId)
    layoutVersionRafId = null
  }
  if (scrollRafId !== null) {
    window.cancelAnimationFrame(scrollRafId)
    scrollRafId = null
  }
  if (emitRangeRafId !== null) {
    window.cancelAnimationFrame(emitRangeRafId)
    emitRangeRafId = null
  }
  activeScrollContainer.value?.removeEventListener('scroll', handleScroll)
  pendingRange = null
  pendingScrollContainer = null
  updateHeightHandlers.clear()
  observedResizeTargets.clear()
  if (resizeObserver) {
    resizeObserver.disconnect()
  }
})

const containerStyle = computed(() => {
  if (usesExternalScroll.value) return undefined
  if (typeof props.height === 'number') {
    return { height: `${props.height}px` }
  }
  if (props.height !== undefined) {
    return { height: props.height }
  }
  return undefined
})
</script>

<template>
  <div
    ref="containerRef"
    class="virtual-scroll-container"
    :class="{ 'virtual-scroll-container-external': usesExternalScroll }"
    :style="containerStyle"
  >
    <!-- Spacer for total height -->
    <div class="virtual-scroll-spacer" :style="{ height: `${totalHeight}px` }">
      <!-- Visible items container -->
      <div class="virtual-scroll-content" :style="{ transform: `translateY(${offsetTop}px)` }">
        <slot
          v-for="offset in visibleCount"
          :key="visibleRange.start + offset - 1"
          :index="visibleRange.start + offset - 1"
          :item="items ? items[visibleRange.start + offset - 1] : undefined"
          :update-height="getUpdateHeightHandler(visibleRange.start + offset - 1)"
        />
      </div>
    </div>
  </div>
</template>

<style scoped>
.virtual-scroll-container {
  overflow-y: auto;
  position: relative;
  overflow-anchor: none;
}

.virtual-scroll-container-external {
  overflow: visible;
  height: auto !important;
}

.virtual-scroll-spacer {
  position: relative;
  overflow-anchor: none;
}

.virtual-scroll-content {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  overflow-anchor: none;
}
</style>
