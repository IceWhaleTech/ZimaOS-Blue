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

function mountGraph() {
  return mount(KnowledgeGraphMap, {
    props: {
      pages,
      selectedSlug: 'readme',
      pageTypeLabel: (pageType: string) => pageType,
      statusLabel: (status: string) => status,
    },
    global: {
      plugins: [i18n],
    },
  })
}

describe('KnowledgeGraphMap', () => {
  it('keeps the graph quiet until click, then recenters and reveals only the local neighborhood', async () => {
    const wrapper = mountGraph()

    expect(wrapper.get('[data-testid="knowledge-graph-zoom-in"]').text()).toContain('+')
    expect(wrapper.get('[data-testid="knowledge-graph-zoom-out"]').text()).toContain('-')
    expect(wrapper.get('[data-testid="knowledge-graph-reset"]').text()).toContain('Reset')
    expect(wrapper.get('[data-testid="knowledge-graph-viewport"]').classes()).toContain(
      'transition-transform'
    )
    expect(wrapper.get('[data-testid="knowledge-graph-stage"]').classes()).toContain('min-h-[36rem]')
    expect(wrapper.get('[data-testid="knowledge-graph-edge"]').element.tagName).toBe('path')
    expect(wrapper.get('[data-testid="knowledge-graph-edge"]').attributes('d')).toContain('Q')
    expect(wrapper.find('[data-testid="knowledge-graph-tooltip"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="knowledge-graph-focus-card"]').exists()).toBe(false)
    expect(
      wrapper.get('[data-testid="knowledge-graph-node-readme"]').attributes('data-label-visible')
    ).toBe('false')

    await wrapper.get('[data-testid="knowledge-graph-zoom-in"]').trigger('click')
    expect(wrapper.get('[data-testid="knowledge-graph-viewport"]').attributes('style')).toContain(
      'scale(1.15)'
    )

    const stage = wrapper.get('[data-testid="knowledge-graph-stage"]')
    await stage.trigger('pointerdown', { clientX: 120, clientY: 140 })
    await stage.trigger('pointermove', { clientX: 170, clientY: 180 })
    await stage.trigger('pointerup', { clientX: 170, clientY: 180 })

    expect(wrapper.get('[data-testid="knowledge-graph-viewport"]').attributes('style')).toContain(
      'translate(50px, 40px)'
    )

    await wrapper.get('[data-testid="knowledge-graph-node-architecture"]').trigger('click')
    expect(wrapper.emitted('select')).toEqual([['architecture']])
    expect(wrapper.get('[data-testid="knowledge-graph-viewport"]').attributes('style')).toContain(
      'translate(0px, 0px)'
    )

    await wrapper.setProps({ selectedSlug: 'architecture' })
    expect(wrapper.get('[data-testid="knowledge-graph-focus-card"]').text()).toContain(
      'Blue Architecture'
    )
    expect(
      wrapper.get('[data-testid="knowledge-graph-node-architecture"]').attributes()
    ).toMatchObject({
      'data-highlight-state': 'selected',
      'data-label-visible': 'true',
    })
    expect(wrapper.get('[data-testid="knowledge-graph-node-readme"]').attributes()).toMatchObject({
      'data-highlight-state': 'neighbor',
      'data-label-visible': 'true',
      'data-distance-tier': 'near',
    })
    expect(wrapper.get('[data-testid="knowledge-graph-node-policy"]').attributes()).toMatchObject({
      'data-highlight-state': 'neighbor',
      'data-label-visible': 'true',
      'data-distance-tier': 'near',
    })
    expect(wrapper.get('[data-testid="knowledge-graph-node-archive"]').attributes()).toMatchObject({
      'data-label-visible': 'false',
      'data-distance-tier': 'mid',
    })
    expect(wrapper.get('[data-testid="knowledge-graph-node-detached"]').attributes()).toMatchObject(
      {
        'data-highlight-state': 'muted',
        'data-label-visible': 'false',
        'data-distance-tier': 'muted',
      }
    )

    await wrapper.get('[data-testid="knowledge-graph-reset"]').trigger('click')
    expect(wrapper.get('[data-testid="knowledge-graph-viewport"]').attributes('style')).toContain(
      'scale(1)'
    )
  })
})
