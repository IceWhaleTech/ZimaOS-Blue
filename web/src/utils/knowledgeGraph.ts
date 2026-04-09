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

const GRAPH_WIDTH = 860
const GRAPH_HEIGHT = 560
const GRAPH_PADDING = 28

type NodePhysicsState = KnowledgeGraphNode & {
  vx: number
  vy: number
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

function orbitalRadius(distance: number, disconnected = false): number {
  if (disconnected) return 420
  if (distance <= 0) return 0
  if (distance === 1) return 150
  if (distance === 2) return 260
  if (distance === 3) return 350
  return 430 + (distance - 4) * 46
}

function layoutOrbitNodes(
  pages: readonly KnowledgePageSummary[],
  edges: readonly KnowledgeGraphEdge[],
  selectedSlug: string
): KnowledgeGraphNode[] {
  const degreeMap = buildDegreeMap(pages, edges)
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

  const nodes: KnowledgeGraphNode[] = []
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
    })
  }

  const sortedDistances = [...groups.keys()].sort((left, right) => left - right)
  for (const distance of sortedDistances) {
    const group = (groups.get(distance) || []).sort((left, right) => left.title.localeCompare(right.title))
    const ringRadius = orbitalRadius(distance)
    const angleStep = (Math.PI * 2) / Math.max(group.length, 1)
    const orbitStart = (hashString(`${selectedSlug}:${distance}`) % 360) * (Math.PI / 180)
    for (let index = 0; index < group.length; index += 1) {
      const page = group[index]
      const degree = degreeMap.get(page.slug) || 0
      const radius = clamp(18 + degree * 2 + (distance === 1 ? 2 : 0), 14, distance === 1 ? 30 : 26)
      const anchor = typeAnchors.get(page.page_type)
      const anchorAngle = anchor ? Math.atan2(anchor.y - centerY, anchor.x - centerX) : 0
      const angle = orbitStart + index * angleStep + anchorAngle * 0.12
      const ellipseY = distance === 1 ? 0.82 : distance === 2 ? 0.9 : 0.96
      nodes.push({
        slug: page.slug,
        title: page.title,
        pageType: page.page_type,
        status: page.status,
        degree,
        radius,
        x: clamp(centerX + Math.cos(angle) * ringRadius, radius + GRAPH_PADDING, GRAPH_WIDTH - radius - GRAPH_PADDING),
        y: clamp(
          centerY + Math.sin(angle) * ringRadius * ellipseY,
          radius + GRAPH_PADDING,
          GRAPH_HEIGHT - radius - GRAPH_PADDING
        ),
        selected: false,
      })
    }
  }

  if (disconnected.length) {
    const group = [...disconnected].sort((left, right) => left.title.localeCompare(right.title))
    const ringRadius = orbitalRadius(0, true)
    const angleStep = (Math.PI * 2) / Math.max(group.length, 1)
    const orbitStart = (hashString(`${selectedSlug}:disconnected`) % 360) * (Math.PI / 180)
    for (let index = 0; index < group.length; index += 1) {
      const page = group[index]
      const degree = degreeMap.get(page.slug) || 0
      const radius = clamp(12 + degree, 12, 20)
      const angle = orbitStart + index * angleStep
      nodes.push({
        slug: page.slug,
        title: page.title,
        pageType: page.page_type,
        status: page.status,
        degree,
        radius,
        x: clamp(centerX + Math.cos(angle) * ringRadius, radius + GRAPH_PADDING, GRAPH_WIDTH - radius - GRAPH_PADDING),
        y: clamp(centerY + Math.sin(angle) * ringRadius * 0.98, radius + GRAPH_PADDING, GRAPH_HEIGHT - radius - GRAPH_PADDING),
        selected: false,
      })
    }
  }

  return nodes
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
    const radius = clamp(18 + degree * 2 + (page.slug === selectedSlug ? 4 : 0), 18, 34)

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
      x: centerX + Math.cos(angle + index * 0.18) * ring,
      y: centerY + Math.sin(angle + index * 0.18) * ring * sway,
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

      const centerPull = node.slug === centerPage?.slug ? 0.04 : 0.0008 + node.degree * 0.0002
      node.vx += (centerX - node.x) * centerPull
      node.vy += (centerY - node.y) * centerPull

      node.vx *= 0.82
      node.vy *= 0.82
      node.x = clamp(
        node.x + node.vx,
        node.radius + GRAPH_PADDING,
        GRAPH_WIDTH - node.radius - GRAPH_PADDING
      )
      node.y = clamp(
        node.y + node.vy,
        node.radius + GRAPH_PADDING,
        GRAPH_HEIGHT - node.radius - GRAPH_PADDING
      )
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
