import type { KnowledgePageSummary, KnowledgeStatus } from '@/api/knowledge'

export type KnowledgeGraphRelation = 'reference' | 'conflict' | 'superseded'

export interface KnowledgeGraphNode {
  slug: string
  title: string
  pageType: string
  status: KnowledgeStatus
  degree: number
  radius: number
  x: number
  y: number
  selected: boolean
}

export interface KnowledgeGraphEdge {
  id: string
  source: string
  target: string
  relation: KnowledgeGraphRelation
}

export interface KnowledgeGraphLayout {
  width: number
  height: number
  nodes: KnowledgeGraphNode[]
  edges: KnowledgeGraphEdge[]
}

const GRAPH_WIDTH = 900
const GRAPH_HEIGHT = 620
const GRAPH_PADDING = 28

type NodePhysicsState = KnowledgeGraphNode & {
  vx: number
  vy: number
}

type OrbitLayoutState = KnowledgeGraphNode & {
  targetX: number
  targetY: number
  vx: number
  vy: number
  distance: number
  disconnected: boolean
}

function hashString(input: string): number {
  let hash = 0
  for (let index = 0; index < input.length; index += 1) {
    hash = (hash * 31 + input.charCodeAt(index)) >>> 0
  }
  return hash
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value))
}

function getViewportEnvelope(nodeRadius = 0) {
  return {
    centerX: GRAPH_WIDTH / 2,
    centerY: GRAPH_HEIGHT / 2,
    radiusX: GRAPH_WIDTH / 2 - nodeRadius - GRAPH_PADDING,
    radiusY: GRAPH_HEIGHT / 2 - nodeRadius - GRAPH_PADDING,
  }
}

export function isKnowledgeGraphPointInsideEnvelope(x: number, y: number, nodeRadius = 0): boolean {
  const { centerX, centerY, radiusX, radiusY } = getViewportEnvelope(nodeRadius)
  const normalized =
    ((x - centerX) * (x - centerX)) / (radiusX * radiusX) +
    ((y - centerY) * (y - centerY)) / (radiusY * radiusY)
  return normalized <= 1.001
}

function containInViewportEllipse(x: number, y: number, nodeRadius: number) {
  const { centerX, centerY, radiusX, radiusY } = getViewportEnvelope(nodeRadius)
  const dx = x - centerX
  const dy = y - centerY
  const normalized =
    (dx * dx) / (radiusX * radiusX) +
    (dy * dy) / (radiusY * radiusY)

  if (normalized <= 1) {
    return { x, y }
  }

  const scale = 1 / Math.sqrt(normalized)
  return {
    x: centerX + dx * scale,
    y: centerY + dy * scale,
  }
}

function normalizePairKey(left: string, right: string): string {
  return [left, right].sort().join('::')
}

function addEdge(
  edges: Map<string, KnowledgeGraphEdge>,
  pageIndex: Map<string, KnowledgePageSummary>,
  source: string,
  target: string,
  relation: KnowledgeGraphRelation
) {
  if (!source || !target || source === target) return
  if (!pageIndex.has(source) || !pageIndex.has(target)) return
  const key = `${relation}::${normalizePairKey(source, target)}`
  if (!edges.has(key)) {
    edges.set(key, {
      id: key,
      source,
      target,
      relation,
    })
  }
}

function buildEdges(pages: readonly KnowledgePageSummary[]): KnowledgeGraphEdge[] {
  const pageIndex = new Map(pages.map((page) => [page.slug, page]))
  const edges = new Map<string, KnowledgeGraphEdge>()

  for (const page of pages) {
    for (const backlink of page.backlinks || []) {
      addEdge(edges, pageIndex, page.slug, backlink, 'reference')
    }
    for (const conflict of page.conflicts_with || []) {
      addEdge(edges, pageIndex, page.slug, conflict, 'conflict')
    }
    for (const successor of page.superseded_by || []) {
      addEdge(edges, pageIndex, page.slug, successor, 'superseded')
    }
  }

  return [...edges.values()]
}

function buildDegreeMap(
  pages: readonly KnowledgePageSummary[],
  edges: readonly KnowledgeGraphEdge[]
): Map<string, number> {
  const degree = new Map(pages.map((page) => [page.slug, 0]))
  for (const edge of edges) {
    degree.set(edge.source, (degree.get(edge.source) || 0) + 1)
    degree.set(edge.target, (degree.get(edge.target) || 0) + 1)
  }
  return degree
}

function pageRelationCount(page: KnowledgePageSummary): number {
  return (page.backlinks?.length || 0) + (page.conflicts_with?.length || 0) + (page.superseded_by?.length || 0)
}

function degreeWeight(degree: number, maxDegree: number): number {
  if (maxDegree <= 0) return 0
  return degree / maxDegree
}

function orbitNodeRadius(distance: number, degree: number, maxDegree: number, disconnected = false) {
  const weight = degreeWeight(degree, maxDegree)
  if (disconnected) {
    return clamp(7 + weight * 2.4 + degree * 0.45, 6, 11)
  }
  if (distance <= 1) {
    return clamp(15 + weight * 7 + degree * 0.55, 15, 34)
  }
  if (distance === 2) {
    return clamp(11 + weight * 4.5 + degree * 0.35, 10, 20)
  }
  if (distance === 3) {
    return clamp(9 + weight * 3 + degree * 0.25, 8, 16)
  }
  return clamp(8 + weight * 2 + degree * 0.2, 7, 14)
}

function orbitSpacing(distance: number, disconnected = false) {
  if (disconnected) return 34
  if (distance <= 1) return 18
  if (distance === 2) return 22
  if (distance === 3) return 26
  return 30
}

function orbitAnchorPull(distance: number, disconnected = false) {
  if (disconnected) return 0.018
  if (distance <= 1) return 0.09
  if (distance === 2) return 0.06
  if (distance === 3) return 0.038
  return 0.026
}

function relaxOrbitNodes(states: OrbitLayoutState[]): KnowledgeGraphNode[] {
  const center = states.find((node) => node.selected)
  const useSparseRepulsion = states.length > 900
  const ticks = useSparseRepulsion ? 52 : 90
  const cellSize = useSparseRepulsion ? 64 : 0

  for (let tick = 0; tick < ticks; tick += 1) {
    if (useSparseRepulsion) {
      const buckets = new Map<string, number[]>()
      for (let index = 0; index < states.length; index += 1) {
        const node = states[index]
        if (!node || node.selected) continue
        const cellX = Math.floor(node.x / cellSize)
        const cellY = Math.floor(node.y / cellSize)
        const key = `${cellX}:${cellY}`
        const bucket = buckets.get(key)
        if (bucket) bucket.push(index)
        else buckets.set(key, [index])
      }

      for (let leftIndex = 0; leftIndex < states.length; leftIndex += 1) {
        const left = states[leftIndex]
        if (!left || left.selected) continue
        const cellX = Math.floor(left.x / cellSize)
        const cellY = Math.floor(left.y / cellSize)

        for (let offsetX = -1; offsetX <= 1; offsetX += 1) {
          for (let offsetY = -1; offsetY <= 1; offsetY += 1) {
            const neighbor = buckets.get(`${cellX + offsetX}:${cellY + offsetY}`)
            if (!neighbor) continue
            for (const rightIndex of neighbor) {
              if (rightIndex <= leftIndex) continue
              const right = states[rightIndex]
              if (!right || right.selected) continue

              const dx = right.x - left.x
              const dy = right.y - left.y
              const distance = Math.max(Math.hypot(dx, dy), 1)
              const desired =
                left.radius +
                right.radius +
                Math.max(
                  orbitSpacing(left.distance, left.disconnected),
                  orbitSpacing(right.distance, right.disconnected)
                )

              const push =
                distance < desired
                  ? (desired - distance) * 0.18
                  : ((desired * desired) / (distance * distance)) * 0.26
              const pushX = (dx / distance) * push
              const pushY = (dy / distance) * push

              left.vx -= pushX
              left.vy -= pushY
              right.vx += pushX
              right.vy += pushY
            }
          }
        }
      }
    } else {
      for (let leftIndex = 0; leftIndex < states.length; leftIndex += 1) {
        const left = states[leftIndex]
        if (!left || left.selected) continue
        for (let rightIndex = leftIndex + 1; rightIndex < states.length; rightIndex += 1) {
          const right = states[rightIndex]
          if (!right || right.selected) continue
          const dx = right.x - left.x
          const dy = right.y - left.y
          const distance = Math.max(Math.hypot(dx, dy), 1)
          const desired =
            left.radius +
            right.radius +
            Math.max(
              orbitSpacing(left.distance, left.disconnected),
              orbitSpacing(right.distance, right.disconnected)
            )

          const push =
            distance < desired
              ? (desired - distance) * 0.16
              : ((desired * desired) / (distance * distance)) * 0.38
          const pushX = (dx / distance) * push
          const pushY = (dy / distance) * push

          left.vx -= pushX
          left.vy -= pushY
          right.vx += pushX
          right.vy += pushY
        }
      }
    }

    for (const node of states) {
      if (node.selected) continue
      const anchorPull = orbitAnchorPull(node.distance, node.disconnected)
      node.vx += (node.targetX - node.x) * anchorPull
      node.vy += (node.targetY - node.y) * anchorPull

      if (center) {
        const spreadBias = node.disconnected ? -0.0008 : node.distance <= 1 ? 0.0025 : 0
        node.vx += (center.x - node.x) * spreadBias
        node.vy += (center.y - node.y) * spreadBias
      }

      const envelope = getViewportEnvelope(node.radius)
      const normalizedX = (node.x - envelope.centerX) / envelope.radiusX
      const normalizedY = (node.y - envelope.centerY) / envelope.radiusY
      const boundaryDistance = Math.hypot(normalizedX, normalizedY)
      if (boundaryDistance > 0.82) {
        const boundaryPull = (boundaryDistance - 0.82) * 0.07
        node.vx -= normalizedX * boundaryPull * envelope.radiusX
        node.vy -= normalizedY * boundaryPull * envelope.radiusY
      }

      node.vx *= 0.72
      node.vy *= 0.72

      const next = containInViewportEllipse(node.x + node.vx, node.y + node.vy, node.radius)
      node.x = next.x
      node.y = next.y
    }
  }

  return states.map(({ targetX: _targetX, targetY: _targetY, vx: _vx, vy: _vy, distance: _distance, disconnected: _disconnected, ...node }) => node)
}

export function pickKnowledgeGraphDefaultFocusSlug(
  pages: readonly KnowledgePageSummary[]
): string {
  if (!pages.length) return ''

  const edges = buildEdges(pages)
  const degreeMap = buildDegreeMap(pages, edges)

  const preferred = [...pages].sort((left, right) => {
    const byDegree = (degreeMap.get(right.slug) || 0) - (degreeMap.get(left.slug) || 0)
    if (byDegree !== 0) return byDegree

    const byRelations = pageRelationCount(right) - pageRelationCount(left)
    if (byRelations !== 0) return byRelations

    const byUpdated = Date.parse(right.updated_at || '') - Date.parse(left.updated_at || '')
    if (!Number.isNaN(byUpdated) && byUpdated !== 0) return byUpdated

    return left.title.localeCompare(right.title)
  })

  return preferred[0]?.slug || ''
}

function buildAdjacency(edges: readonly KnowledgeGraphEdge[]) {
  const adjacency = new Map<string, Set<string>>()
  for (const edge of edges) {
    if (!adjacency.has(edge.source)) adjacency.set(edge.source, new Set())
    if (!adjacency.has(edge.target)) adjacency.set(edge.target, new Set())
    adjacency.get(edge.source)?.add(edge.target)
    adjacency.get(edge.target)?.add(edge.source)
  }
  return adjacency
}

function buildDistanceMap(
  pages: readonly KnowledgePageSummary[],
  edges: readonly KnowledgeGraphEdge[],
  selectedSlug?: string
) {
  const distances = new Map<string, number>()
  if (!selectedSlug) return distances
  const adjacency = buildAdjacency(edges)
  for (const page of pages) {
    if (!adjacency.has(page.slug)) adjacency.set(page.slug, new Set())
  }
  const queue: string[] = [selectedSlug]
  distances.set(selectedSlug, 0)

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
}

function orbitEllipseY(distance: number, disconnected = false): number {
  if (disconnected) return 0.94
  if (distance <= 1) return 0.76
  if (distance === 2) return 0.84
  if (distance === 3) return 0.9
  return 0.95
}

function orbitalRadius(distance: number, disconnected = false): number {
  if (disconnected) return 388
  if (distance <= 0) return 0
  if (distance === 1) return 136
  if (distance === 2) return 228
  if (distance === 3) return 304
  return 362 + (distance - 4) * 32
}

export function getKnowledgeGraphOrbitGuide(distance: number, disconnected = false) {
  const radiusX = orbitalRadius(distance, disconnected)
  return {
    radiusX,
    radiusY: Math.round(radiusX * orbitEllipseY(distance, disconnected)),
  }
}

function layoutOrbitNodes(
  pages: readonly KnowledgePageSummary[],
  edges: readonly KnowledgeGraphEdge[],
  selectedSlug: string
): KnowledgeGraphNode[] {
  const GOLDEN_ANGLE = Math.PI * (3 - Math.sqrt(5))
  const degreeMap = buildDegreeMap(pages, edges)
  const maxDegree = Math.max(...degreeMap.values(), 1)
  const centerX = GRAPH_WIDTH / 2
  const centerY = GRAPH_HEIGHT / 2
  const distanceMap = buildDistanceMap(pages, edges, selectedSlug)
  const typeAnchors = createTypeAnchors(pages)

  const groups = new Map<number, KnowledgePageSummary[]>()
  const disconnected: KnowledgePageSummary[] = []

  for (const page of pages) {
    if (page.slug === selectedSlug) continue
    const distance = distanceMap.get(page.slug)
    if (distance == null) {
      disconnected.push(page)
      continue
    }
    if (!groups.has(distance)) groups.set(distance, [])
    groups.get(distance)?.push(page)
  }

  const nodes: OrbitLayoutState[] = []
  const centerPage = pages.find((page) => page.slug === selectedSlug)
  if (centerPage) {
    nodes.push({
      slug: centerPage.slug,
      title: centerPage.title,
      pageType: centerPage.page_type,
      status: centerPage.status,
      degree: degreeMap.get(centerPage.slug) || 0,
      radius: clamp(24 + (degreeMap.get(centerPage.slug) || 0) * 2, 24, 36),
      x: centerX,
      y: centerY,
      selected: true,
      targetX: centerX,
      targetY: centerY,
      vx: 0,
      vy: 0,
      distance: 0,
      disconnected: false,
    })
  }

  const sortedDistances = [...groups.keys()].sort((left, right) => left - right)
  const clampInt = (value: number, min: number, max: number) =>
    Math.max(min, Math.min(max, Math.round(value)))
  for (const distance of sortedDistances) {
    const group = groups.get(distance) || []
    const byType = new Map<string, KnowledgePageSummary[]>()
    for (const page of group) {
      const key = page.page_type || 'unknown'
      const bucket = byType.get(key)
      if (bucket) bucket.push(page)
      else byType.set(key, [page])
    }

    const typeKeys = [...byType.keys()].sort((left, right) => {
      const leftAnchor = typeAnchors.get(left)
      const rightAnchor = typeAnchors.get(right)
      const leftAngle = leftAnchor ? Math.atan2(leftAnchor.y - centerY, leftAnchor.x - centerX) : 0
      const rightAngle = rightAnchor
        ? Math.atan2(rightAnchor.y - centerY, rightAnchor.x - centerX)
        : 0
      const byAngle = leftAngle - rightAngle
      if (Math.abs(byAngle) > 0.0001) return byAngle
      return left.localeCompare(right)
    })

    const { radiusX, radiusY } = getKnowledgeGraphOrbitGuide(distance)
    const orbitStart = (hashString(`${selectedSlug}:${distance}`) % 360) * (Math.PI / 180)

    const clusterArc =
      distance <= 1 ? 0.9 : distance === 2 ? 0.68 : distance === 3 ? 0.58 : 0.5
    const maxColumns = distance <= 1 ? 10 : distance === 2 ? 12 : 14
    const shellBase =
      distance <= 1 ? 0.32 : distance === 2 ? 0.78 : distance === 3 ? 0.97 : 1.04
    const shellStep =
      distance <= 1 ? 0.055 : distance === 2 ? 0.045 : distance === 3 ? 0.036 : 0.03

    for (const typeKey of typeKeys) {
      const cluster = (byType.get(typeKey) || []).sort((left, right) => {
        const byDegree = (degreeMap.get(right.slug) || 0) - (degreeMap.get(left.slug) || 0)
        if (byDegree !== 0) return byDegree
        return left.title.localeCompare(right.title)
      })
      const anchor = typeAnchors.get(typeKey)
      const anchorAngle = anchor ? Math.atan2(anchor.y - centerY, anchor.x - centerX) : 0
      const anchorWeight = distance <= 1 ? 0.92 : distance === 2 ? 0.74 : 0.62
      const clusterCenter = orbitStart + anchorAngle * anchorWeight

      const shellCount = clampInt(Math.sqrt(cluster.length) / 2.3, 2, 6)
      const columns = Math.max(1, Math.min(maxColumns, Math.ceil(cluster.length / shellCount)))
      const angleStep = columns <= 1 ? 0 : clusterArc / (columns - 1)

      for (let index = 0; index < cluster.length; index += 1) {
        const page = cluster[index]
        if (!page) continue
        const seed = hashString(`${page.slug}:${distance}:${typeKey}:cluster`)
        const degree = degreeMap.get(page.slug) || 0
        const weight = degreeWeight(degree, maxDegree)
        const radius = orbitNodeRadius(distance, degree, maxDegree)

        const columnIndex = Math.floor(index / shellCount) % columns
        const shellIndex = index % shellCount
        const columnOffset = columns <= 1 ? 0 : (columnIndex - (columns - 1) / 2) * angleStep
        const angleJitter =
          (((seed >>> 3) % 19) - 9) * (distance <= 1 ? 0.02 : distance === 2 ? 0.015 : 0.012)
        const angle = clusterCenter + columnOffset + angleJitter

        const shellNoise = ((seed % 17) / 100) * 0.8
        const radialScale =
          shellBase +
          shellIndex * shellStep +
          (1 - weight) * (distance <= 1 ? 0.18 : distance === 2 ? 0.2 : 0.16) +
          shellNoise

        const position = containInViewportEllipse(
          centerX + Math.cos(angle) * radiusX * radialScale,
          centerY + Math.sin(angle) * radiusY * radialScale,
          radius
        )
        nodes.push({
          slug: page.slug,
          title: page.title,
          pageType: page.page_type,
          status: page.status,
          degree,
          radius,
          x: position.x,
          y: position.y,
          selected: false,
          targetX: position.x,
          targetY: position.y,
          vx: 0,
          vy: 0,
          distance,
          disconnected: false,
        })
      }
    }
  }

  if (disconnected.length) {
    const group = [...disconnected].sort((left, right) => left.title.localeCompare(right.title))
    const { radiusX, radiusY } = getKnowledgeGraphOrbitGuide(0, true)
    const orbitStart = (hashString(`${selectedSlug}:disconnected`) % 360) * (Math.PI / 180)
    const shellCount = Math.max(1, Math.min(3, Math.ceil(group.length / 8)))
    for (let index = 0; index < group.length; index += 1) {
      const page = group[index]
      if (!page) continue
      const seed = hashString(`${page.slug}:detached`)
      const degree = degreeMap.get(page.slug) || 0
      const radius = orbitNodeRadius(4, degree, maxDegree, true)
      const angle =
        orbitStart +
        index * GOLDEN_ANGLE * 0.86 +
        (((seed >>> 5) % 23) - 11) * 0.024
      const shellIndex = index % shellCount
      const shell = 0.74 + shellIndex * 0.09 + ((seed % 17) / 100)
      const position = containInViewportEllipse(
        centerX + Math.cos(angle) * radiusX * shell,
        centerY + Math.sin(angle) * radiusY * shell,
        radius
      )
      nodes.push({
        slug: page.slug,
        title: page.title,
        pageType: page.page_type,
        status: page.status,
        degree,
        radius,
        x: position.x,
        y: position.y,
        selected: false,
        targetX: position.x,
        targetY: position.y,
        vx: 0,
        vy: 0,
        distance: 4,
        disconnected: true,
      })
    }
  }

  return relaxOrbitNodes(nodes)
}

function createTypeAnchors(pages: readonly KnowledgePageSummary[]) {
  const types = [...new Set(pages.map((page) => page.page_type).filter(Boolean))].sort()
  const anchors = new Map<string, { x: number; y: number }>()
  if (!types.length) return anchors

  const centerX = GRAPH_WIDTH / 2
  const centerY = GRAPH_HEIGHT / 2
  const radius = Math.min(GRAPH_WIDTH, GRAPH_HEIGHT) * 0.26

  types.forEach((pageType, index) => {
    const angle = (Math.PI * 2 * index) / types.length - Math.PI / 2
    anchors.set(pageType, {
      x: centerX + Math.cos(angle) * radius,
      y: centerY + Math.sin(angle) * radius * 0.72,
    })
  })

  return anchors
}

function layoutNodes(
  pages: readonly KnowledgePageSummary[],
  edges: readonly KnowledgeGraphEdge[],
  selectedSlug?: string
): KnowledgeGraphNode[] {
  if (selectedSlug && pages.some((page) => page.slug === selectedSlug)) {
    return layoutOrbitNodes(pages, edges, selectedSlug)
  }

  const degreeMap = buildDegreeMap(pages, edges)
  const maxDegree = Math.max(...degreeMap.values(), 1)
  const centerPage =
    pages.find((page) => page.slug === selectedSlug) ||
    [...pages].sort((left, right) => {
      const byDegree = (degreeMap.get(right.slug) || 0) - (degreeMap.get(left.slug) || 0)
      if (byDegree !== 0) return byDegree
      return left.title.localeCompare(right.title)
    })[0]

  const centerX = GRAPH_WIDTH / 2
  const centerY = GRAPH_HEIGHT / 2
  const typeAnchors = createTypeAnchors(pages)
  const physics: NodePhysicsState[] = pages.map((page, index) => {
    const degree = degreeMap.get(page.slug) || 0
    const radius = clamp(14 + degreeWeight(degree, maxDegree) * 7 + degree * 0.45 + (page.slug === selectedSlug ? 5 : 0), 14, 32)

    if (centerPage && page.slug === centerPage.slug) {
      return {
        slug: page.slug,
        title: page.title,
        pageType: page.page_type,
        status: page.status,
        degree,
        radius,
        x: centerX,
        y: centerY,
        vx: 0,
        vy: 0,
        selected: page.slug === selectedSlug,
      }
    }

    const seed = hashString(`${page.slug}:${page.page_type}:${page.status}`)
    const angle = ((seed % 360) * Math.PI) / 180
    const ring = 110 + (seed % 130) + degree * 6
    const sway = 0.68 + ((seed >>> 3) % 24) / 100

    return {
      slug: page.slug,
      title: page.title,
      pageType: page.page_type,
      status: page.status,
      degree,
      radius,
      ...containInViewportEllipse(
        centerX + Math.cos(angle + index * 0.18) * ring,
        centerY + Math.sin(angle + index * 0.18) * ring * sway,
        radius
      ),
      vx: 0,
      vy: 0,
      selected: page.slug === selectedSlug,
    }
  })

  const edgeList = [...edges]
  const nodeIndex = new Map(physics.map((node) => [node.slug, node]))

  for (let tick = 0; tick < 160; tick += 1) {
    for (let leftIndex = 0; leftIndex < physics.length; leftIndex += 1) {
      const left = physics[leftIndex]
      if (!left) continue
      for (let rightIndex = leftIndex + 1; rightIndex < physics.length; rightIndex += 1) {
        const right = physics[rightIndex]
        if (!right) continue
        const dx = right.x - left.x
        const dy = right.y - left.y
        const distance = Math.max(Math.hypot(dx, dy), 1)
        const repulsion = 5200 / (distance * distance)
        const pushX = (dx / distance) * repulsion
        const pushY = (dy / distance) * repulsion

        left.vx -= pushX
        left.vy -= pushY
        right.vx += pushX
        right.vy += pushY
      }
    }

    for (const edge of edgeList) {
      const source = nodeIndex.get(edge.source)
      const target = nodeIndex.get(edge.target)
      if (!source || !target) continue

      const dx = target.x - source.x
      const dy = target.y - source.y
      const distance = Math.max(Math.hypot(dx, dy), 1)
      const desired =
        edge.relation === 'conflict' ? 182 : edge.relation === 'superseded' ? 140 : 110
      const attraction = (distance - desired) * 0.0055
      const pullX = (dx / distance) * attraction
      const pullY = (dy / distance) * attraction

      source.vx += pullX
      source.vy += pullY
      target.vx -= pullX
      target.vy -= pullY
    }

    for (const node of physics) {
      const typeAnchor = typeAnchors.get(node.pageType)
      if (typeAnchor) {
        node.vx += (typeAnchor.x - node.x) * 0.0017
        node.vy += (typeAnchor.y - node.y) * 0.0014
      }

      const envelope = getViewportEnvelope(node.radius)
      const normalizedX = (node.x - envelope.centerX) / envelope.radiusX
      const normalizedY = (node.y - envelope.centerY) / envelope.radiusY
      const boundaryDistance = Math.hypot(normalizedX, normalizedY)
      if (boundaryDistance > 0.84) {
        const boundaryPull = (boundaryDistance - 0.84) * 0.055
        node.vx -= normalizedX * boundaryPull * envelope.radiusX
        node.vy -= normalizedY * boundaryPull * envelope.radiusY
      }

      const centerPull =
        node.slug === centerPage?.slug ? 0.05 : 0.001 + degreeWeight(node.degree, maxDegree) * 0.006
      node.vx += (centerX - node.x) * centerPull
      node.vy += (centerY - node.y) * centerPull

      node.vx *= 0.82
      node.vy *= 0.82
      const nextPosition = containInViewportEllipse(node.x + node.vx, node.y + node.vy, node.radius)
      node.x = nextPosition.x
      node.y = nextPosition.y
    }
  }

  return physics.map(({ vx: _vx, vy: _vy, ...node }) => node)
}

export function buildKnowledgeGraphLayout(
  pages: readonly KnowledgePageSummary[],
  selectedSlug?: string
): KnowledgeGraphLayout {
  const edges = buildEdges(pages)
  const nodes = layoutNodes(pages, edges, selectedSlug)

  return {
    width: GRAPH_WIDTH,
    height: GRAPH_HEIGHT,
    nodes,
    edges,
  }
}
