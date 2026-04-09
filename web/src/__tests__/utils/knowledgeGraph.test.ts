import { describe, expect, it } from 'vitest'

import type { KnowledgePageSummary } from '@/api/knowledge'
import { buildKnowledgeGraphLayout } from '@/utils/knowledgeGraph'

const pages: KnowledgePageSummary[] = [
  {
    title: 'Center',
    slug: 'center',
    page_type: 'source_summary',
    summary: 'Center node.',
    source_refs: ['CENTER.md'],
    keywords: ['center'],
    backlinks: ['near-a', 'near-b'],
    generated_at: '2026-04-05T12:00:00Z',
    updated_at: '2026-04-05T12:00:00Z',
    source_hash: 'hash-1',
    status: 'active',
    confidence: 'high',
    conflicts_with: [],
    superseded_by: [],
    derived_from_query: '',
  },
  {
    title: 'Near A',
    slug: 'near-a',
    page_type: 'concept',
    summary: 'Near node.',
    source_refs: ['A.md'],
    keywords: ['a'],
    backlinks: ['center', 'far'],
    generated_at: '2026-04-05T12:00:00Z',
    updated_at: '2026-04-05T12:00:00Z',
    source_hash: 'hash-2',
    status: 'active',
    confidence: 'medium',
    conflicts_with: [],
    superseded_by: [],
    derived_from_query: '',
  },
  {
    title: 'Near B',
    slug: 'near-b',
    page_type: 'concept',
    summary: 'Near node.',
    source_refs: ['B.md'],
    keywords: ['b'],
    backlinks: ['center'],
    generated_at: '2026-04-05T12:00:00Z',
    updated_at: '2026-04-05T12:00:00Z',
    source_hash: 'hash-3',
    status: 'active',
    confidence: 'medium',
    conflicts_with: [],
    superseded_by: [],
    derived_from_query: '',
  },
  {
    title: 'Far',
    slug: 'far',
    page_type: 'concept',
    summary: 'Far node.',
    source_refs: ['FAR.md'],
    keywords: ['far'],
    backlinks: ['near-a'],
    generated_at: '2026-04-05T12:00:00Z',
    updated_at: '2026-04-05T12:00:00Z',
    source_hash: 'hash-4',
    status: 'active',
    confidence: 'medium',
    conflicts_with: [],
    superseded_by: [],
    derived_from_query: '',
  },
]

function distanceFromCenter(x: number, y: number, width: number, height: number) {
  const dx = x - width / 2
  const dy = y - height / 2
  return Math.hypot(dx, dy)
}

describe('buildKnowledgeGraphLayout', () => {
  it('uses taller orbital rings so first-hop nodes stay closer than second-hop nodes', () => {
    const graph = buildKnowledgeGraphLayout(pages, 'center')

    expect(graph.height).toBeGreaterThan(420)

    const center = graph.nodes.find((node) => node.slug === 'center')
    const nearA = graph.nodes.find((node) => node.slug === 'near-a')
    const nearB = graph.nodes.find((node) => node.slug === 'near-b')
    const far = graph.nodes.find((node) => node.slug === 'far')

    expect(center).toBeTruthy()
    expect(nearA).toBeTruthy()
    expect(nearB).toBeTruthy()
    expect(far).toBeTruthy()

    const centerDistance = distanceFromCenter(center!.x, center!.y, graph.width, graph.height)
    const nearADistance = distanceFromCenter(nearA!.x, nearA!.y, graph.width, graph.height)
    const nearBDistance = distanceFromCenter(nearB!.x, nearB!.y, graph.width, graph.height)
    const farDistance = distanceFromCenter(far!.x, far!.y, graph.width, graph.height)

    expect(centerDistance).toBeLessThan(2)
    expect(nearADistance).toBeLessThan(farDistance)
    expect(nearBDistance).toBeLessThan(farDistance)
  })
})
