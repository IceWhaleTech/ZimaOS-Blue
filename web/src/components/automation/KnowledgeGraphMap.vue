<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import type { KnowledgePageSummary } from '@/api/knowledge'
import {
  buildKnowledgeGraphLayout,
  getKnowledgeGraphOrbitGuide,
  pickKnowledgeGraphDefaultFocusSlug,
  type KnowledgeGraphEdge,
  type KnowledgeGraphNode,
  type KnowledgeGraphRelation,
} from '@/utils/knowledgeGraph'

const props = withDefaults(
  defineProps<{
    pages: KnowledgePageSummary[]
    selectedSlug?: string
    pageTypeLabel: (pageType: string) => string
    statusLabel: (status: string) => string
  }>(),
  {
    selectedSlug: '',
  }
)

const emit = defineEmits<{
  select: [slug: string]
}>()

const ZOOM_STEPS = [0.88, 1, 1.18, 1.38, 1.6, 1.84] as const

const { t, te } = useI18n()
const zoom = ref(1)
const offsetX = ref(0)
const offsetY = ref(0)
const revealedSlug = ref('')
const isPanning = ref(false)
const panOrigin = ref({ x: 0, y: 0, offsetX: 0, offsetY: 0 })

function tr(key: string, fallback: string) {
  return te(key) ? t(key) : fallback
}

function clamp(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, value))
}

function relationLabel(relation: KnowledgeGraphRelation): string {
  if (relation === 'conflict') return tr('knowledge.graphLegendConflict', 'Conflict')
  if (relation === 'superseded') return tr('knowledge.graphLegendSuperseded', 'Superseded')
  return tr('knowledge.graphLegendReference', 'Reference')
}

function nodeClass(node: KnowledgeGraphNode): string {
  const tier = nodeDistanceTier(node)
  if (tier === 'ambient') {
    return node.status === 'conflicted'
      ? 'border-transparent bg-transparent text-rose-900 shadow-none opacity-38'
      : 'border-transparent bg-transparent text-slate-800 shadow-none opacity-36'
  }
  if (tier === 'selected') {
    return 'border-amber-300 bg-amber-100/90 text-amber-950 shadow-[0_0_0_1px_rgba(251,191,36,0.35),0_18px_45px_rgba(245,158,11,0.28)] scale-[1.03] opacity-100'
  }
  if (tier === 'near') {
    if (node.status === 'conflicted') {
      return 'border-rose-300 bg-rose-100/88 text-rose-950 shadow-[0_18px_44px_rgba(244,63,94,0.24)] opacity-100'
    }
    if (node.status === 'superseded') {
      return 'border-slate-300 bg-slate-100/88 text-slate-900 shadow-[0_18px_44px_rgba(15,23,42,0.18)] opacity-100'
    }
    return 'border-sky-300 bg-white/95 text-slate-900 shadow-[0_20px_46px_rgba(14,165,233,0.2)] opacity-100'
  }
  if (tier === 'mid') {
    return 'border-transparent bg-transparent text-slate-900 shadow-none opacity-26 saturate-75'
  }
  if (tier === 'far') {
    return 'border-transparent bg-transparent text-slate-900 shadow-none opacity-14 saturate-50'
  }
  if (tier === 'stacked') {
    return 'border-transparent bg-transparent text-slate-900 shadow-none opacity-[0.09] saturate-50'
  }
  return 'border-transparent bg-transparent text-slate-900 shadow-none opacity-[0.06] saturate-50'
}

function edgeClass(edge: KnowledgeGraphEdge): string {
  if (edge.relation === 'conflict') return 'stroke-rose-300'
  if (edge.relation === 'superseded') return 'stroke-slate-400'
  return 'stroke-sky-300'
}

const defaultFocusSlug = computed(() => pickKnowledgeGraphDefaultFocusSlug(props.pages || []))
const activeFocusSlug = computed(() => {
  const candidate = revealedSlug.value || props.selectedSlug || defaultFocusSlug.value
  if (!candidate) return ''
  return props.pages.some((page) => page.slug === candidate) ? candidate : defaultFocusSlug.value
})
const hasRevealedFocus = computed(() => Boolean(activeFocusSlug.value))
const graph = computed(() => buildKnowledgeGraphLayout(props.pages || [], activeFocusSlug.value))

const legend = computed(() => {
  const counts = new Map<KnowledgeGraphRelation, number>([
    ['reference', 0],
    ['conflict', 0],
    ['superseded', 0],
  ])
  for (const edge of graph.value.edges) {
    counts.set(edge.relation, (counts.get(edge.relation) || 0) + 1)
  }
  return [...counts.entries()]
    .filter(([, count]) => count > 0)
    .map(([relation, count]) => ({
      relation,
      count,
      label: relationLabel(relation),
    }))
})

const graphNodeMap = computed(() => new Map(graph.value.nodes.map((node) => [node.slug, node])))
const focusDistances = computed(() => {
  const distances = new Map<string, number>()
  if (!activeFocusSlug.value) return distances

  const adjacency = new Map<string, Set<string>>()
  for (const node of graph.value.nodes) {
    adjacency.set(node.slug, new Set())
  }
  for (const edge of graph.value.edges) {
    adjacency.get(edge.source)?.add(edge.target)
    adjacency.get(edge.target)?.add(edge.source)
  }

  const queue: string[] = [activeFocusSlug.value]
  distances.set(activeFocusSlug.value, 0)

  while (queue.length) {
    const current = queue.shift()
    if (!current) continue
    const currentDistance = distances.get(current) ?? 0
    for (const neighbor of adjacency.get(current) || []) {
      if (distances.has(neighbor)) continue
      distances.set(neighbor, currentDistance + 1)
      queue.push(neighbor)
    }
  }

  return distances
})
const selectedNeighborhood = computed(() => {
  const slugs = new Set<string>()
  if (!activeFocusSlug.value) return slugs
  slugs.add(activeFocusSlug.value)

  const relationWeight: Record<KnowledgeGraphRelation, number> = {
    conflict: 0,
    superseded: 1,
    reference: 2,
  }
  const candidates = new Map<
    string,
    {
      slug: string
      relation: KnowledgeGraphRelation
      degree: number
      title: string
    }
  >()

  for (const edge of graph.value.edges) {
    let neighborSlug = ''
    if (edge.source === activeFocusSlug.value) neighborSlug = edge.target
    else if (edge.target === activeFocusSlug.value) neighborSlug = edge.source
    if (!neighborSlug) continue

    const neighbor = graphNodeMap.value.get(neighborSlug)
    if (!neighbor) continue
    const existing = candidates.get(neighborSlug)
    if (!existing || relationWeight[edge.relation] < relationWeight[existing.relation]) {
      candidates.set(neighborSlug, {
        slug: neighborSlug,
        relation: edge.relation,
        degree: neighbor.degree,
        title: neighbor.title,
      })
    }
  }

  const limit =
    props.pages.length >= 240
      ? 4
      : props.pages.length >= 120
        ? 5
        : props.pages.length >= 60
          ? 6
          : props.pages.length >= 24
            ? 8
            : 12

  const visibleNeighbors = [...candidates.values()]
    .sort((left, right) => {
      const byRelation = relationWeight[left.relation] - relationWeight[right.relation]
      if (byRelation !== 0) return byRelation
      const byDegree = right.degree - left.degree
      if (byDegree !== 0) return byDegree
      return left.title.localeCompare(right.title)
    })
    .slice(0, limit)

  for (const neighbor of visibleNeighbors) {
    slugs.add(neighbor.slug)
  }
  return slugs
})

const revealedNode = computed(
  () => graph.value.nodes.find((node) => node.slug === activeFocusSlug.value) || null
)
const revealLayer = computed(() => {
  if (!activeFocusSlug.value) return 0
  if (zoom.value >= 1.6) return 4
  if (zoom.value >= 1.38) return 3
  if (zoom.value >= 1.18) return 2
  return 1
})
const orbitLayers = computed(() => {
  if (!activeFocusSlug.value) return []

  const connectedDistances = new Set<number>()
  let hasDisconnected = false

  for (const node of graph.value.nodes) {
    if (node.slug === activeFocusSlug.value) continue
    const distance = focusDistances.value.get(node.slug)
    if (distance == null) {
      hasDisconnected = true
      continue
    }
    if (distance > 0) connectedDistances.add(Math.min(distance, 4))
  }

  const layers = [...connectedDistances]
    .sort((left, right) => left - right)
    .slice(0, 4)
    .map((distance) => ({
      key: `distance-${distance}`,
      distance,
      disconnected: false,
      ...getKnowledgeGraphOrbitGuide(distance),
    }))

  if (hasDisconnected) {
    layers.push({
      key: 'distance-disconnected',
      distance: 4,
      disconnected: true,
      ...getKnowledgeGraphOrbitGuide(0, true),
    })
  }

  return layers
})
const layerIndicator = computed(() => {
  if (!activeFocusSlug.value) {
    return {
      level: '',
      description: tr(
        'knowledge.graphLayerIdle',
        'Click a node to focus. Zoom to peel back outer layers.'
      ),
    }
  }

  const description =
    revealLayer.value === 1
      ? tr('knowledge.graphLayerNear', 'Focus with direct neighbors')
      : revealLayer.value === 2
        ? tr('knowledge.graphLayerMid', 'Second-hop context')
        : revealLayer.value === 3
          ? tr('knowledge.graphLayerFar', 'Extended orbit')
          : tr('knowledge.graphLayerFull', 'Outer background and detached knowledge')

  return {
    level: `${tr('knowledge.graphLayer', 'Layer')} ${revealLayer.value} / 4`,
    description,
  }
})
const viewportStyle = computed(() => ({
  transform: `translate(${offsetX.value}px, ${offsetY.value}px) scale(${zoom.value})`,
  transformOrigin: 'center center',
}))
const defaultEdgeOpacity = computed(() =>
  props.pages.length >= 180 ? 0.025 : props.pages.length >= 80 ? 0.05 : 0.1
)

function nodeStyle(node: KnowledgeGraphNode) {
  const tier = nodeDistanceTier(node)
  const zIndex =
    tier === 'selected'
      ? 6
      : tier === 'near'
        ? 5
        : tier === 'mid'
          ? 4
          : tier === 'far'
            ? 3
            : 2

  return {
    left: `${(node.x / graph.value.width) * 100}%`,
    top: `${(node.y / graph.value.height) * 100}%`,
    zIndex,
  }
}

function isDistanceRevealed(distance: number | null | undefined): boolean {
  if (distance == null) return revealLayer.value >= 4
  if (distance <= 1) return true
  if (distance === 2) return revealLayer.value >= 2
  if (distance === 3) return revealLayer.value >= 3
  return revealLayer.value >= 4
}

function nodeHighlightState(
  node: KnowledgeGraphNode
): 'default' | 'selected' | 'neighbor' | 'muted' {
  if (!activeFocusSlug.value) return 'default'
  if (node.slug === activeFocusSlug.value) return 'selected'
  if (selectedNeighborhood.value.has(node.slug)) return 'neighbor'
  return 'muted'
}

function nodeDistanceTier(
  node: KnowledgeGraphNode
): 'ambient' | 'selected' | 'near' | 'mid' | 'far' | 'muted' | 'stacked' {
  if (!activeFocusSlug.value) return 'ambient'
  const distance = focusDistances.value.get(node.slug)
  if (distance === 0) return 'selected'
  if (distance === 1 && selectedNeighborhood.value.has(node.slug)) return 'near'
  if (distance === 1) return revealLayer.value >= 2 ? 'mid' : 'stacked'
  if (distance === 2) return revealLayer.value >= 2 ? 'mid' : 'stacked'
  if (distance === 3) return revealLayer.value >= 3 ? 'far' : 'stacked'
  if (distance == null || distance >= 4) return revealLayer.value >= 4 ? 'muted' : 'stacked'
  return 'far'
}

function edgeHighlightState(edge: KnowledgeGraphEdge): 'default' | 'selected' | 'muted' {
  if (!activeFocusSlug.value) return 'default'
  if (edge.source === activeFocusSlug.value || edge.target === activeFocusSlug.value) {
    const neighborSlug = edge.source === activeFocusSlug.value ? edge.target : edge.source
    if (selectedNeighborhood.value.has(neighborSlug)) return 'selected'
  }
  return 'muted'
}

function edgeDistanceTier(
  edge: KnowledgeGraphEdge
): 'ambient' | 'near' | 'mid' | 'far' | 'muted' | 'stacked' {
  if (!activeFocusSlug.value) return 'ambient'
  const sourceDistance = focusDistances.value.get(edge.source)
  const targetDistance = focusDistances.value.get(edge.target)
  if (edge.source === activeFocusSlug.value || edge.target === activeFocusSlug.value) {
    const neighborSlug = edge.source === activeFocusSlug.value ? edge.target : edge.source
    return selectedNeighborhood.value.has(neighborSlug)
      ? 'near'
      : revealLayer.value >= 2
        ? 'mid'
        : 'stacked'
  }
  if (!isDistanceRevealed(sourceDistance) || !isDistanceRevealed(targetDistance)) return 'stacked'
  if (sourceDistance == null || targetDistance == null) return 'muted'
  const distance = Math.max(sourceDistance, targetDistance)
  if (distance <= 2) return 'mid'
  if (distance === 3) return 'far'
  return 'muted'
}

function edgePath(edge: KnowledgeGraphEdge): string {
  const source = graphNodeMap.value.get(edge.source)
  const target = graphNodeMap.value.get(edge.target)
  if (!source || !target) return ''

  const dx = target.x - source.x
  const dy = target.y - source.y
  const distance = Math.max(Math.hypot(dx, dy), 1)
  const midX = (source.x + target.x) / 2
  const midY = (source.y + target.y) / 2
  const normalX = -dy / distance
  const normalY = dx / distance
  const direction = (edge.id.charCodeAt(0) + edge.id.length) % 2 === 0 ? 1 : -1
  const curveStrength =
    edge.relation === 'conflict'
      ? 34
      : edgeDistanceTier(edge) === 'near'
        ? 28
        : edgeDistanceTier(edge) === 'mid'
          ? 20
          : 14
  const controlX = midX + normalX * curveStrength * direction
  const controlY = midY + normalY * curveStrength * direction
  return `M ${source.x} ${source.y} Q ${controlX} ${controlY} ${target.x} ${target.y}`
}

function shouldShowNodeLabel(node: KnowledgeGraphNode): boolean {
  if (!hasRevealedFocus.value) return false
  if (node.slug === activeFocusSlug.value) return true
  return selectedNeighborhood.value.has(node.slug)
}

function nodeCircleStyle(node: KnowledgeGraphNode) {
  const tier = nodeDistanceTier(node)
  const scale =
    tier === 'selected'
      ? 1.06
      : tier === 'near'
        ? 0.82
        : tier === 'mid'
          ? 0.56
          : tier === 'far'
            ? 0.38
            : tier === 'stacked'
              ? 0.18
            : tier === 'muted'
              ? 0.24
              : 0.46

  const background =
    node.status === 'conflicted'
      ? tier === 'selected' || tier === 'near'
        ? 'rgba(253, 164, 175, 0.82)'
        : tier === 'stacked'
          ? 'rgba(253, 164, 175, 0.12)'
        : 'rgba(253, 164, 175, 0.4)'
      : node.status === 'superseded'
        ? tier === 'selected' || tier === 'near'
          ? 'rgba(203, 213, 225, 0.88)'
          : tier === 'stacked'
            ? 'rgba(203, 213, 225, 0.14)'
          : 'rgba(203, 213, 225, 0.42)'
        : tier === 'selected' || tier === 'near'
          ? 'rgba(186, 230, 253, 0.88)'
          : tier === 'stacked'
            ? 'rgba(186, 230, 253, 0.12)'
          : 'rgba(186, 230, 253, 0.46)'

  return {
    width: `${Math.round(node.radius * 2 * scale)}px`,
    height: `${Math.round(node.radius * 2 * scale)}px`,
    background,
    filter: tier === 'stacked' ? 'blur(0.2px)' : 'none',
  }
}

function orbitOpacity(distance: number, disconnected = false) {
  if (!activeFocusSlug.value) return 0
  if (disconnected) return revealLayer.value >= 4 ? 0.08 : 0.015
  if (distance === 1) return 0.16
  if (distance === 2) return revealLayer.value >= 2 ? 0.1 : 0.03
  if (distance === 3) return revealLayer.value >= 3 ? 0.07 : 0.02
  return revealLayer.value >= 4 ? 0.05 : 0.015
}

function orbitFillOpacity(distance: number, disconnected = false) {
  if (!activeFocusSlug.value) return 0
  if (disconnected) return revealLayer.value >= 4 ? 0.01 : 0.002
  if (distance === 1) return 0.02
  if (distance === 2) return revealLayer.value >= 2 ? 0.014 : 0.004
  if (distance === 3) return revealLayer.value >= 3 ? 0.01 : 0.003
  return revealLayer.value >= 4 ? 0.008 : 0.002
}

function orbitDasharray(distance: number, disconnected = false) {
  if (disconnected) return '5 11'
  if (distance === 1) return '0'
  if (distance === 2) return '6 10'
  return '3 12'
}

function nextZoomStep(direction: 'in' | 'out') {
  if (direction === 'in') {
    return ZOOM_STEPS.find((step) => step > zoom.value + 0.001) ?? ZOOM_STEPS[ZOOM_STEPS.length - 1]!
  }
  return [...ZOOM_STEPS].reverse().find((step) => step < zoom.value - 0.001) ?? ZOOM_STEPS[0]!
}

function zoomIn() {
  zoom.value = nextZoomStep('in')
}

function zoomOut() {
  zoom.value = nextZoomStep('out')
}

function resetView() {
  zoom.value = 1
  offsetX.value = 0
  offsetY.value = 0
}

function handleWheel(event: WheelEvent) {
  if (event.deltaY < 0) {
    zoomIn()
    return
  }
  zoomOut()
}

function startPan(event: PointerEvent) {
  const target = event.target
  if (target instanceof Element && target.closest('[data-graph-node="true"]')) return
  isPanning.value = true
  panOrigin.value = {
    x: event.clientX,
    y: event.clientY,
    offsetX: offsetX.value,
    offsetY: offsetY.value,
  }
}

function updatePan(event: PointerEvent) {
  if (!isPanning.value) return
  offsetX.value = clamp(panOrigin.value.offsetX + event.clientX - panOrigin.value.x, -220, 220)
  offsetY.value = clamp(panOrigin.value.offsetY + event.clientY - panOrigin.value.y, -180, 180)
}

function endPan() {
  isPanning.value = false
}

function handleNodeSelect(slug: string) {
  revealedSlug.value = slug
  resetView()
  emit('select', slug)
}

watch(
  () => props.selectedSlug,
  (next, previous) => {
    if (!next) return
    const shouldReset = Boolean(previous) && previous !== next
    revealedSlug.value = next
    if (shouldReset) resetView()
  },
  { immediate: true }
)

watch(
  defaultFocusSlug,
  (next) => {
    if (props.selectedSlug || !next || revealedSlug.value) return
    revealedSlug.value = next
  },
  { immediate: true }
)
</script>

<template>
  <div class="space-y-3">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div class="flex flex-wrap gap-2">
        <span class="rounded-full bg-slate-100 px-3 py-1 text-xs text-slate-700">
          {{
            tr('knowledge.graphNodeCount', '{count} nodes').replace(
              '{count}',
              String(graph.nodes.length)
            )
          }}
        </span>
        <span class="rounded-full bg-slate-100 px-3 py-1 text-xs text-slate-700">
          {{
            tr('knowledge.graphEdgeCount', '{count} links').replace(
              '{count}',
              String(graph.edges.length)
            )
          }}
        </span>
      </div>

      <div
        v-if="legend.length"
        class="flex flex-wrap items-center justify-end gap-2 text-[11px] uppercase tracking-[0.16em] text-slate-500"
      >
        <span
          v-for="item in legend"
          :key="item.relation"
          class="inline-flex items-center gap-2 rounded-full border border-slate-200 bg-white/80 px-3 py-1"
        >
          <span
            class="h-2.5 w-2.5 rounded-full"
            :class="
              item.relation === 'conflict'
                ? 'bg-rose-300'
                : item.relation === 'superseded'
                  ? 'bg-slate-400'
                  : 'bg-sky-300'
            "
          />
          <span>{{ item.label }}</span>
          <span class="text-slate-400">{{ item.count }}</span>
        </span>
      </div>
    </div>

    <div
      class="relative overflow-hidden rounded-[28px] border border-slate-200 bg-[radial-gradient(circle_at_top,_rgba(251,191,36,0.2),_transparent_35%),radial-gradient(circle_at_bottom_left,_rgba(56,189,248,0.18),_transparent_30%),linear-gradient(180deg,_rgba(248,250,252,0.94),_rgba(226,232,240,0.72))]"
    >
      <div
        class="pointer-events-none absolute inset-x-0 top-0 h-20 bg-gradient-to-b from-white/55 to-transparent"
      />

      <div
        v-if="!graph.nodes.length"
        class="flex min-h-[24rem] items-center justify-center px-6 text-center text-sm text-slate-500"
      >
        {{
          tr(
            'knowledge.graphEmpty',
            'No visible pages yet. Run ingest or relax your filters to grow the graph.'
          )
        }}
      </div>

      <div v-else class="relative min-h-[40rem]">
        <div
          data-testid="knowledge-graph-stage"
          :data-layer-depth="String(revealLayer)"
          class="absolute inset-0 min-h-[40rem] cursor-grab touch-none"
          :class="{ 'cursor-grabbing': isPanning }"
          @pointerdown="startPan"
          @pointermove="updatePan"
          @pointerup="endPan"
          @pointerleave="endPan"
          @wheel.prevent="handleWheel"
        >
          <div
            data-testid="knowledge-graph-viewport"
            class="absolute inset-0 will-change-transform transition-transform duration-500 ease-out"
            :style="viewportStyle"
          >
            <svg
              class="absolute inset-0 h-full w-full"
              :viewBox="`0 0 ${graph.width} ${graph.height}`"
              fill="none"
            >
              <ellipse
                v-for="orbit in orbitLayers"
                :key="orbit.key"
                data-testid="knowledge-graph-orbit"
                :cx="graph.width / 2"
                :cy="graph.height / 2"
                :rx="orbit.radiusX"
                :ry="orbit.radiusY"
                class="transition-all duration-500 ease-out"
                stroke="rgba(148, 163, 184, 0.9)"
                :fill="`rgba(255, 255, 255, ${orbitFillOpacity(orbit.distance, orbit.disconnected)})`"
                :opacity="orbitOpacity(orbit.distance, orbit.disconnected)"
                :stroke-dasharray="orbitDasharray(orbit.distance, orbit.disconnected)"
                :stroke-width="orbit.distance === 1 && !orbit.disconnected ? 1.3 : 1"
              />
              <path
                v-for="edge in graph.edges"
                :key="edge.id"
                data-testid="knowledge-graph-edge"
                :data-highlight-state="edgeHighlightState(edge)"
                :data-distance-tier="edgeDistanceTier(edge)"
                :d="edgePath(edge)"
                class="transition-[opacity,stroke-width] duration-500 ease-out"
                :class="edgeClass(edge)"
                :opacity="
                  edgeDistanceTier(edge) === 'ambient'
                    ? defaultEdgeOpacity
                    : edgeDistanceTier(edge) === 'near'
                      ? 0.95
                      : edgeDistanceTier(edge) === 'stacked'
                        ? 0.05
                      : edgeDistanceTier(edge) === 'mid'
                        ? 0.28
                        : edgeDistanceTier(edge) === 'far'
                          ? 0.12
                          : 0.06
                "
                :stroke-width="
                  edgeDistanceTier(edge) === 'near'
                    ? 2.2
                    : edgeDistanceTier(edge) === 'stacked'
                      ? 0.9
                    : edgeDistanceTier(edge) === 'mid'
                      ? 1.5
                      : edgeDistanceTier(edge) === 'far'
                        ? 1.1
                        : 1
                "
                stroke-linecap="round"
              />
            </svg>

            <div class="absolute inset-0">
              <button
                v-for="node in graph.nodes"
                :key="node.slug"
                :data-testid="`knowledge-graph-node-${node.slug}`"
                data-graph-node="true"
                :data-highlight-state="nodeHighlightState(node)"
                :data-label-visible="shouldShowNodeLabel(node) ? 'true' : 'false'"
                :data-distance-tier="nodeDistanceTier(node)"
                type="button"
                class="absolute flex -translate-x-1/2 -translate-y-1/2 flex-col items-center gap-2 rounded-3xl px-2 pb-2 pt-1.5 text-center transition-[left,top,transform,opacity,background-color,border-color,box-shadow] duration-500 ease-out hover:scale-[1.02] focus:outline-none focus:ring-2 focus:ring-amber-300"
                :class="nodeClass(node)"
                :style="nodeStyle(node)"
                :title="`${pageTypeLabel(node.pageType)} · ${statusLabel(node.status)}`"
                @click="handleNodeSelect(node.slug)"
              >
                <span
                  class="inline-flex items-center justify-center rounded-full border border-black/5 text-[11px] font-semibold"
                  :style="nodeCircleStyle(node)"
                >
                  {{ Math.max(node.degree, 1) }}
                </span>
                <span
                  v-if="shouldShowNodeLabel(node)"
                  class="max-w-[7.5rem] text-[11px] font-medium leading-4 sm:text-[12px]"
                >
                  {{ node.title }}
                </span>
              </button>
            </div>
          </div>

          <div class="absolute right-3 top-3 z-10 flex flex-col gap-2" data-graph-controls="true">
            <button
              data-testid="knowledge-graph-zoom-in"
              type="button"
              class="inline-flex h-11 w-11 items-center justify-center rounded-2xl border border-white/70 bg-white/85 text-base font-semibold text-slate-900 shadow-sm backdrop-blur transition hover:bg-white"
              :aria-label="tr('knowledge.graphZoomIn', 'Zoom in')"
              @click.stop="zoomIn"
            >
              +
            </button>
            <button
              data-testid="knowledge-graph-zoom-out"
              type="button"
              class="inline-flex h-11 w-11 items-center justify-center rounded-2xl border border-white/70 bg-white/85 text-base font-semibold text-slate-900 shadow-sm backdrop-blur transition hover:bg-white"
              :aria-label="tr('knowledge.graphZoomOut', 'Zoom out')"
              @click.stop="zoomOut"
            >
              -
            </button>
            <button
              data-testid="knowledge-graph-reset"
              type="button"
              class="inline-flex min-h-11 items-center justify-center rounded-2xl border border-white/70 bg-white/85 px-3 text-xs font-medium text-slate-700 shadow-sm backdrop-blur transition hover:bg-white"
              @click.stop="resetView"
            >
              {{ tr('knowledge.graphReset', 'Reset') }}
            </button>
          </div>

          <div
            data-testid="knowledge-graph-layer-indicator"
            class="pointer-events-none absolute bottom-3 right-3 max-w-[16rem] rounded-2xl border border-white/70 bg-white/88 px-3 py-2 text-xs text-slate-500 shadow-sm backdrop-blur"
          >
            <div v-if="layerIndicator.level" class="font-semibold tracking-[0.08em] text-slate-700">
              {{ layerIndicator.level }}
            </div>
            <div :class="layerIndicator.level ? 'mt-1' : ''">
              {{ layerIndicator.description }}
            </div>
            <div class="mt-1.5 text-[11px] text-slate-400">
              {{ tr('knowledge.graphPanHint', 'Drag to pan. Use the wheel or buttons to zoom.') }}
            </div>
          </div>
        </div>

        <div
          v-if="revealedNode"
          data-testid="knowledge-graph-focus-card"
          class="pointer-events-none absolute bottom-3 left-3 rounded-2xl border border-white/70 bg-white/85 px-3 py-2 text-xs text-slate-600 shadow-sm backdrop-blur"
        >
          <div class="uppercase tracking-[0.16em] text-slate-400">
            {{ tr('knowledge.graphFocus', 'Focus') }}
          </div>
          <div class="mt-1 font-semibold text-slate-900">{{ revealedNode.title }}</div>
          <div class="mt-0.5">
            {{ pageTypeLabel(revealedNode.pageType) }} · {{ statusLabel(revealedNode.status) }}
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
