import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'

import type { KnowledgePageSummary } from '@/api/knowledge'
import KnowledgeGraphMap from '@/components/automation/KnowledgeGraphMap.vue'
import { i18n } from '@/i18n'

const pages: KnowledgePageSummary[] = [
  {
    title: 'Blue Knowledge',
    slug: 'readme',
    page_type: 'source_summary',
    summary: 'Compiled entry page.',
    source_refs: ['README.md'],
    keywords: ['blue', 'knowledge'],
    backlinks: ['architecture'],
    generated_at: '2026-04-05T12:00:00Z',
    updated_at: '2026-04-05T12:00:00Z',
    source_hash: 'hash-1',
    status: 'active',
    confidence: 'low',
    conflicts_with: [],
    superseded_by: [],
    derived_from_query: '',
  },
  {
    title: 'Blue Architecture',
    slug: 'architecture',
    page_type: 'source_summary',
    summary: 'Architecture detail.',
    source_refs: ['ARCHITECTURE.md'],
    keywords: ['architecture'],
    backlinks: ['readme'],
    generated_at: '2026-04-05T12:00:00Z',
    updated_at: '2026-04-05T12:00:00Z',
    source_hash: 'hash-2',
    status: 'conflicted',
    confidence: 'low',
    conflicts_with: ['readme'],
    superseded_by: [],
    derived_from_query: '',
  },
  {
    title: 'Detached Notes',
    slug: 'detached',
    page_type: 'concept',
    summary: 'Independent page.',
    source_refs: ['NOTES.md'],
    keywords: ['notes'],
    backlinks: [],
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
    title: 'Policy Notes',
    slug: 'policy',
    page_type: 'concept',
    summary: 'Connected through architecture.',
    source_refs: ['POLICY.md'],
    keywords: ['policy'],
    backlinks: ['architecture'],
    generated_at: '2026-04-05T12:00:00Z',
    updated_at: '2026-04-05T12:00:00Z',
    source_hash: 'hash-4',
    status: 'active',
    confidence: 'medium',
    conflicts_with: [],
    superseded_by: [],
    derived_from_query: '',
  },
  {
    title: 'Archive Trail',
    slug: 'archive',
    page_type: 'concept',
    summary: 'Further away in the graph.',
    source_refs: ['ARCHIVE.md'],
    keywords: ['archive'],
    backlinks: ['policy'],
    generated_at: '2026-04-05T12:00:00Z',
    updated_at: '2026-04-05T12:00:00Z',
    source_hash: 'hash-5',
    status: 'active',
    confidence: 'medium',
    conflicts_with: [],
    superseded_by: [],
    derived_from_query: '',
  },
]

function mountGraph(selectedSlug = 'architecture') {
  return mount(KnowledgeGraphMap, {
    props: {
      pages,
      selectedSlug,
      pageTypeLabel: (pageType: string) => pageType,
      statusLabel: (status: string) => status,
    },
    global: {
      plugins: [i18n],
    },
  })
}

describe('KnowledgeGraphMap', () => {
  it('starts focused on the hottest node and keeps outer nodes smaller while zooming outward', async () => {
    const wrapper = mountGraph()

    expect(wrapper.get('[data-testid="knowledge-graph-focus-card"]').text()).toContain(
      'Blue Architecture'
    )
    expect(wrapper.get('[data-testid="knowledge-graph-layer-indicator"]').text()).toContain('1 / 4')
    expect(wrapper.get('[data-testid="knowledge-graph-zoom-in"]').text()).toContain('+')
    expect(wrapper.get('[data-testid="knowledge-graph-zoom-out"]').text()).toContain('-')
    expect(wrapper.get('[data-testid="knowledge-graph-reset"]').text()).toContain('Reset')
    expect(wrapper.get('[data-testid="knowledge-graph-viewport"]').classes()).toContain(
      'transition-transform'
    )
    expect(wrapper.get('[data-testid="knowledge-graph-stage"]').classes()).toContain('min-h-[40rem]')
    expect(wrapper.get('[data-testid="knowledge-graph-edge"]').element.tagName).toBe('path')
    expect(wrapper.get('[data-testid="knowledge-graph-edge"]').attributes('d')).toContain('Q')
    expect(wrapper.find('[data-testid="knowledge-graph-tooltip"]').exists()).toBe(false)
    expect(
      wrapper.get('[data-testid="knowledge-graph-node-architecture"]').attributes('data-label-visible')
    ).toBe('true')
    expect(wrapper.get('[data-testid="knowledge-graph-node-readme"]').attributes()).toMatchObject({
      'data-label-visible': 'true',
      'data-distance-tier': 'near',
    })
    expect(wrapper.get('[data-testid="knowledge-graph-node-archive"]').attributes()).toMatchObject({
      'data-label-visible': 'false',
      'data-distance-tier': 'stacked',
    })

    const selectedCircle = wrapper
      .get('[data-testid="knowledge-graph-node-architecture"] span')
      .attributes('style')
    const detachedCircle = wrapper
      .get('[data-testid="knowledge-graph-node-detached"] span')
      .attributes('style')
    const selectedWidth = Number(/width:\s*(\d+)px/.exec(selectedCircle)?.[1] || 0)
    const detachedWidth = Number(/width:\s*(\d+)px/.exec(detachedCircle)?.[1] || 0)
    expect(selectedWidth).toBeGreaterThan(40)
    expect(detachedWidth).toBeLessThan(selectedWidth / 4)

    await wrapper.get('[data-testid="knowledge-graph-zoom-in"]').trigger('click')
    expect(wrapper.get('[data-testid="knowledge-graph-viewport"]').attributes('style')).toContain(
      'scale(1.18)'
    )

    const stage = wrapper.get('[data-testid="knowledge-graph-stage"]')
    await stage.trigger('pointerdown', { clientX: 120, clientY: 140 })
    await stage.trigger('pointermove', { clientX: 170, clientY: 180 })
    await stage.trigger('pointerup', { clientX: 170, clientY: 180 })

    expect(wrapper.get('[data-testid="knowledge-graph-viewport"]').attributes('style')).toContain(
      'translate(50px, 40px)'
    )

    await wrapper.get('[data-testid="knowledge-graph-node-readme"]').trigger('click')
    expect(wrapper.emitted('select')).toEqual([['readme']])
    expect(wrapper.get('[data-testid="knowledge-graph-viewport"]').attributes('style')).toContain(
      'translate(0px, 0px)'
    )
    expect(wrapper.get('[data-testid="knowledge-graph-viewport"]').attributes('style')).toContain(
      'scale(1)'
    )

    await wrapper.setProps({ selectedSlug: 'readme' })
    expect(wrapper.get('[data-testid="knowledge-graph-focus-card"]').text()).toContain(
      'Blue Knowledge'
    )
    expect(wrapper.get('[data-testid="knowledge-graph-layer-indicator"]').text()).toContain('1 / 4')
    expect(wrapper.findAll('[data-testid="knowledge-graph-orbit"]').length).toBeGreaterThan(0)
    expect(
      wrapper.get('[data-testid="knowledge-graph-node-readme"]').attributes()
    ).toMatchObject({
      'data-highlight-state': 'selected',
      'data-label-visible': 'true',
    })
    expect(wrapper.get('[data-testid="knowledge-graph-node-architecture"]').attributes()).toMatchObject({
      'data-highlight-state': 'neighbor',
      'data-label-visible': 'true',
      'data-distance-tier': 'near',
    })
    expect(wrapper.get('[data-testid="knowledge-graph-node-archive"]').attributes()).toMatchObject({
      'data-label-visible': 'false',
      'data-distance-tier': 'stacked',
    })
    expect(wrapper.get('[data-testid="knowledge-graph-node-detached"]').attributes()).toMatchObject(
      {
        'data-highlight-state': 'muted',
        'data-label-visible': 'false',
        'data-distance-tier': 'stacked',
      }
    )

    await wrapper.get('[data-testid="knowledge-graph-zoom-in"]').trigger('click')
    expect(wrapper.get('[data-testid="knowledge-graph-layer-indicator"]').text()).toContain('2 / 4')
    expect(wrapper.get('[data-testid="knowledge-graph-viewport"]').attributes('style')).toContain(
      'scale(1.18)'
    )
    expect(wrapper.get('[data-testid="knowledge-graph-node-archive"]').attributes()).toMatchObject({
      'data-distance-tier': 'stacked',
    })
    expect(wrapper.get('[data-testid="knowledge-graph-node-detached"]').attributes()).toMatchObject(
      {
        'data-distance-tier': 'stacked',
      }
    )

    await wrapper.get('[data-testid="knowledge-graph-zoom-in"]').trigger('click')
    expect(wrapper.get('[data-testid="knowledge-graph-layer-indicator"]').text()).toContain('3 / 4')
    expect(wrapper.get('[data-testid="knowledge-graph-node-archive"]').attributes()).toMatchObject({
      'data-distance-tier': 'far',
    })

    await wrapper.get('[data-testid="knowledge-graph-zoom-in"]').trigger('click')
    expect(wrapper.get('[data-testid="knowledge-graph-layer-indicator"]').text()).toContain('4 / 4')
    expect(wrapper.get('[data-testid="knowledge-graph-node-detached"]').attributes()).toMatchObject(
      {
        'data-distance-tier': 'muted',
      }
    )

    await wrapper.get('[data-testid="knowledge-graph-reset"]').trigger('click')
    expect(wrapper.get('[data-testid="knowledge-graph-viewport"]').attributes('style')).toContain(
      'scale(1)'
    )
    expect(wrapper.get('[data-testid="knowledge-graph-layer-indicator"]').text()).toContain('1 / 4')
    expect(wrapper.get('[data-testid="knowledge-graph-node-archive"]').attributes()).toMatchObject({
      'data-distance-tier': 'stacked',
    })
  })
})
