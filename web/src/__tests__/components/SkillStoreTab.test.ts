import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import SkillStoreTab from '@/components/extensions/SkillStoreTab.vue'
import { i18n } from '@/i18n'
import { skillApi } from '@/api/skill'

const { sseState } = vi.hoisted(() => ({
  sseState: {
    handler: null as null | ((data: Record<string, unknown>) => void),
    embeddingHandler: null as null | ((data: Record<string, unknown>) => void),
  },
}))

vi.mock('@/api/skill', () => ({
  skillApi: {
    discoverStatus: vi.fn(),
    discoverRefresh: vi.fn(),
    embeddingStatus: vi.fn(),
    filtersMarket: vi.fn(),
    searchMarket: vi.fn(),
    adviseMarket: vi.fn(),
    getMarketplaceSkill: vi.fn(),
    installMarket: vi.fn(),
  },
}))

vi.mock('@/composables/useEventStream', () => ({
  onSSEEvent: vi.fn((event: string, handler: (data: Record<string, unknown>) => void) => {
    if (event === 'skill.market.discover.progress') {
      sseState.handler = handler
    }
    if (event === 'skill.market.embedding.progress') {
      sseState.embeddingHandler = handler
    }
  }),
  offSSEEvent: vi.fn((event: string, handler: (data: Record<string, unknown>) => void) => {
    if (event === 'skill.market.discover.progress' && sseState.handler === handler) {
      sseState.handler = null
    }
    if (event === 'skill.market.embedding.progress' && sseState.embeddingHandler === handler) {
      sseState.embeddingHandler = null
    }
  }),
}))

vi.mock('@/components/ui/SemanticSearchField.vue', () => ({
  default: {
    name: 'SemanticSearchField',
    props: ['modelValue'],
    emits: ['update:modelValue', 'clear', 'submit-shortcut'],
    template:
      '<input data-testid="search-input" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
  },
}))

function makeSkill(overrides: Record<string, unknown> = {}) {
  return {
    id: 'skill-alpha',
    name: 'Skill Alpha',
    version: '1.0.0',
    latest_version: '1.0.0',
    description: 'Fixture skill',
    category: 'development_tools',
    tags: ['alpha'],
    source_id: 'tencent-skillhub',
    source_name: 'Tencent SkillHub',
    source_group: 'skillhub',
    source_url: 'https://example.com/skill-alpha',
    homepage: 'https://example.com/skill-alpha',
    download_url: 'https://example.com/skill-alpha/SKILL.md',
    security_badge: 'green',
    risk_level: 'low',
    installable: true,
    install_type: 'raw_skill',
    installed: false,
    ...overrides,
  }
}

function makeSearchResponse(skills: Array<Record<string, unknown>>) {
  return {
    data: {
      skills: skills.map((skill) => ({ skill, score: 1 })),
      total: skills.length,
      page: 1,
      page_size: skills.length || 20,
      total_pages: 1,
    },
  }
}

function makeFiltersResponse() {
  return {
    data: {
      categories: [{ value: 'development_tools', label: 'Development Tools', count: 1 }],
      sources: [{ value: 'skillhub', label: 'Tencent SkillHub', count: 1 }],
      risk_badges: [{ value: 'green', label: 'Green shield', count: 1 }],
      install_types: [],
      artifact_kinds: [],
      installable: { true: 1 },
      security_signals: {},
    },
  }
}

function makeDetailResponse(skill: Record<string, unknown>) {
  return {
    data: {
      skill,
      version: {
        version: String(skill.version || skill.latest_version || '1.0.0'),
        source_url: String(skill.source_url || ''),
        skill_path: 'SKILL.md',
      },
      security: {
        skill_id: String(skill.id),
        version: String(skill.version || skill.latest_version || '1.0.0'),
        score: 96,
        risk_level: 'low',
        security_badge: 'green',
        vulnerability_status: 'not_applicable',
        permissions: [],
        install_surface: {
          install_type: 'raw_skill',
          artifact_kind: 'open_source',
          installable: true,
          has_binary: false,
          has_scripts: false,
        },
        has_vulnerabilities: false,
        has_prompt_injection: false,
        has_shell_injection: false,
        has_data_exfiltration: false,
      },
      installed: false,
      enabled: false,
    },
  }
}

async function mountSkillStore(props: Record<string, unknown> = {}) {
  const wrapper = mount(SkillStoreTab, {
    props,
    global: {
      plugins: [i18n],
    },
  })
  await flushPromises()
  return wrapper
}

describe('SkillStoreTab', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.useRealTimers()
    sseState.handler = null
    sseState.embeddingHandler = null
    i18n.global.locale.value = 'en-US'

    const skill = makeSkill()
    vi.mocked(skillApi.discoverStatus).mockResolvedValue({
      data: {
        running: false,
        finished_at: '2026-03-22T00:00:00Z',
        result: { sources_processed: 1, discovered: 1, updated: 0, failed: 0 },
      },
    } as never)
    vi.mocked(skillApi.discoverRefresh).mockResolvedValue({
      data: { running: true },
    } as never)
    vi.mocked(skillApi.embeddingStatus).mockResolvedValue({
      data: { running: false },
    } as never)
    vi.mocked(skillApi.filtersMarket).mockResolvedValue(makeFiltersResponse() as never)
    vi.mocked(skillApi.searchMarket).mockResolvedValue(makeSearchResponse([skill]) as never)
    vi.mocked(skillApi.adviseMarket).mockResolvedValue({
      data: {
        query: '',
        need_store_search: false,
      },
    } as never)
    vi.mocked(skillApi.getMarketplaceSkill).mockImplementation(async (id: string) => {
      return makeDetailResponse(makeSkill({ id })) as never
    })
    vi.mocked(skillApi.installMarket).mockResolvedValue({ data: {} } as never)
  })

  afterEach(() => {
    vi.useRealTimers()
    sseState.handler = null
    sseState.embeddingHandler = null
  })

  it('keeps the toolbar free of curated/installable quick tags', async () => {
    const wrapper = await mountSkillStore()

    expect(wrapper.findAll('.toolbar-controls .sort-pill')).toHaveLength(4)
    expect(wrapper.findAll('.toolbar-controls button')).toHaveLength(4)
    expect(wrapper.text()).not.toContain('Curated only')
    expect(wrapper.text()).not.toContain('Installable only')
    expect(wrapper.text()).not.toContain('仅精选')
    expect(wrapper.text()).not.toContain('仅可安装')
  })

  it('shows the first-load hint and hides zero-count copy during progress', async () => {
    vi.mocked(skillApi.searchMarket).mockResolvedValue(makeSearchResponse([]) as never)
    const wrapper = await mountSkillStore()

    expect(sseState.handler).toBeTypeOf('function')
    sseState.handler?.({
      running: true,
      current_source_name: 'Tencent SkillHub',
    })
    await flushPromises()

    const progress = wrapper.get('.discover-progress')
    expect(progress.text()).toContain('First-time loading')
    expect(progress.text()).toContain('Initializing')
    expect(wrapper.findAll('.hero-summary-row .summary-pill')).toHaveLength(0)
    expect(wrapper.find('.panel-header p').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('0 skills')
  })

  it('shows cached catalog cards while a discover run is still in progress', async () => {
    vi.useFakeTimers()
    vi.mocked(skillApi.discoverStatus)
      .mockResolvedValueOnce({
        data: {
          running: true,
          total_sources: 3,
          processed_sources: 1,
          current_source_name: 'Tencent SkillHub',
        },
      } as never)
      .mockResolvedValueOnce({
        data: {
          running: true,
          total_sources: 3,
          processed_sources: 1,
          current_source_name: 'Tencent SkillHub',
        },
      } as never)
      .mockResolvedValueOnce({
        data: {
          running: false,
          finished_at: '2026-03-22T00:00:05Z',
          total_sources: 3,
          processed_sources: 3,
          result: { sources_processed: 3, discovered: 2, updated: 1, failed: 0 },
        },
      } as never)

    const wrapper = await mountSkillStore()

    expect(wrapper.findAll('.skill-card')).toHaveLength(1)
    expect(vi.mocked(skillApi.searchMarket).mock.calls[0]?.[0]).toMatchObject({
      sort: 'featured',
      page: 1,
      page_size: 20,
    })
    expect(vi.mocked(skillApi.searchMarket).mock.calls[0]?.[0]).not.toHaveProperty('curated')
    expect(wrapper.get('.discover-progress').text()).toContain('Tencent SkillHub')

    await vi.advanceTimersByTimeAsync(2100)
    await flushPromises()
  })

  it('refreshes visible results from discover polling when SSE progress is unavailable', async () => {
    vi.useFakeTimers()
    const refreshedSkills = [
      makeSkill(),
      makeSkill({
        id: 'skill-beta',
        name: 'Skill Beta',
        source_url: 'https://example.com/skill-beta',
        homepage: 'https://example.com/skill-beta',
        download_url: 'https://example.com/skill-beta/SKILL.md',
      }),
    ]

    vi.mocked(skillApi.discoverStatus)
      .mockResolvedValueOnce({
        data: {
          running: true,
          started_at: '2026-03-22T00:00:00Z',
          total_sources: 3,
          processed_sources: 0,
          result: { sources_processed: 0, discovered: 0, updated: 0, failed: 0 },
        },
      } as never)
      .mockResolvedValueOnce({
        data: {
          running: true,
          started_at: '2026-03-22T00:00:00Z',
          total_sources: 3,
          processed_sources: 1,
          current_source_name: 'Tencent SkillHub',
          result: { sources_processed: 1, discovered: 2, updated: 0, failed: 0 },
        },
      } as never)
      .mockResolvedValueOnce({
        data: {
          running: false,
          started_at: '2026-03-22T00:00:00Z',
          finished_at: '2026-03-22T00:00:05Z',
          total_sources: 3,
          processed_sources: 3,
          result: { sources_processed: 3, discovered: 2, updated: 0, failed: 0 },
        },
      } as never)

    vi.mocked(skillApi.searchMarket)
      .mockResolvedValueOnce(makeSearchResponse([]) as never)
      .mockResolvedValueOnce(makeSearchResponse(refreshedSkills) as never)
      .mockResolvedValueOnce(makeSearchResponse(refreshedSkills) as never)

    const wrapper = await mountSkillStore()

    expect(wrapper.findAll('.skill-card')).toHaveLength(0)

    await vi.advanceTimersByTimeAsync(360)
    await flushPromises()

    expect(skillApi.searchMarket).toHaveBeenCalledTimes(2)
    expect(vi.mocked(skillApi.searchMarket).mock.calls[0]?.[0]).toMatchObject({
      sort: 'featured',
      page: 1,
      page_size: 20,
    })
    expect(vi.mocked(skillApi.searchMarket).mock.calls[0]?.[0]).not.toHaveProperty('curated')
    expect(vi.mocked(skillApi.searchMarket).mock.calls[1]?.[0]).toMatchObject({
      sort: 'featured',
      page: 1,
      page_size: 20,
    })
    expect(vi.mocked(skillApi.searchMarket).mock.calls[1]?.[0]).not.toHaveProperty('curated')
    expect(wrapper.findAll('.skill-card')).toHaveLength(2)
    expect(wrapper.get('.panel-header').text()).toContain('2 skills')

    await vi.advanceTimersByTimeAsync(2100)
    await flushPromises()
  })

  it('refreshes visible results after progress events without resetting filters', async () => {
    vi.useFakeTimers()
    const wrapper = await mountSkillStore()

    const selects = wrapper.findAll('select')
    await selects[0]!.setValue('development_tools')
    await selects[1]!.setValue('skillhub')
    await selects[2]!.setValue('green')
    await flushPromises()

    vi.mocked(skillApi.searchMarket).mockClear()
    vi.mocked(skillApi.searchMarket).mockResolvedValue(makeSearchResponse([makeSkill()]) as never)

    sseState.handler?.({
      running: true,
      phase: 'batch',
      current_source_name: 'Tencent SkillHub',
    })
    await vi.advanceTimersByTimeAsync(360)
    await flushPromises()

    expect(skillApi.searchMarket).toHaveBeenCalledTimes(1)
    expect(vi.mocked(skillApi.searchMarket).mock.calls[0]?.[0]).toMatchObject({
      category: 'development_tools',
      categories: 'development_tools',
      sources: 'skillhub',
      risk_badges: 'green',
      sort: 'featured',
      semantic: true,
      page: 1,
      page_size: 1,
    })
    expect(vi.mocked(skillApi.searchMarket).mock.calls[0]?.[0]).not.toHaveProperty('curated')
    expect((selects[0]!.element as HTMLSelectElement).value).toBe('development_tools')
    expect((selects[1]!.element as HTMLSelectElement).value).toBe('skillhub')
    expect((selects[2]!.element as HTMLSelectElement).value).toBe('green')
  })

  it('renders a live activity feed and scrolls it on progress updates', async () => {
    vi.useFakeTimers()
    const scrollTo = vi.fn()
    Object.defineProperty(HTMLElement.prototype, 'scrollTo', {
      value: scrollTo,
      configurable: true,
      writable: true,
    })

    const wrapper = await mountSkillStore()

    sseState.handler?.({
      running: true,
      phase: 'started',
      total_sources: 3,
      processed_sources: 0,
    })
    sseState.handler?.({
      running: true,
      phase: 'batch',
      total_sources: 3,
      processed_sources: 1,
      current_source_name: 'Tencent SkillHub',
      batch_inserted: 3,
      batch_updated: 1,
    })
    await flushPromises()

    const feed = wrapper.get('.discover-activity')
    expect(feed.text()).toContain('Started refreshing sources')
    expect(feed.text()).toContain('Processing Tencent SkillHub')
    expect(feed.text()).toContain('+3 new')
    expect(scrollTo).toHaveBeenCalled()
  })

  it('shows embedding progress updates from SSE events', async () => {
    const wrapper = await mountSkillStore()

    expect(sseState.embeddingHandler).toBeTypeOf('function')
    sseState.embeddingHandler?.({
      running: true,
      phase: 'progress',
      total_skills: 12,
      processed_skills: 3,
      embedded_skills: 2,
      failed_skills: 1,
      current_skill_name: 'Git Expert',
    })
    await flushPromises()

    const progressCard = wrapper.get('.discover-progress')
    expect(progressCard.text()).toContain('Embedding skill search index')
    expect(progressCard.text()).toContain('Git Expert')
    expect(progressCard.text()).toContain('Processed 3/12 skills')
    expect(progressCard.text()).toContain('Embedded 2')
  })

  it('merges embedding progress into the active source-sync card', async () => {
    const wrapper = await mountSkillStore()

    sseState.handler?.({
      running: true,
      phase: 'batch',
      total_sources: 3,
      processed_sources: 1,
      current_source_name: 'Tencent SkillHub',
    })
    sseState.embeddingHandler?.({
      running: true,
      phase: 'progress',
      total_skills: 12,
      processed_skills: 3,
      embedded_skills: 2,
      failed_skills: 1,
      current_skill_name: 'Git Expert',
    })
    await flushPromises()

    const progressCards = wrapper.findAll('.discover-progress')
    expect(progressCards).toHaveLength(1)
    expect(progressCards[0]!.text()).toContain('Tencent SkillHub')
    expect(progressCards[0]!.text()).toContain('Embedding skill search index')
    expect(progressCards[0]!.findAll('.discover-progress__segment')).toHaveLength(2)
  })

  it('hides completed embedding progress when there was no queued work', async () => {
    vi.mocked(skillApi.embeddingStatus).mockResolvedValue({
      data: {
        running: false,
        phase: 'completed',
        finished_at: '2026-03-22T00:00:00Z',
        total_skills: 0,
        processed_skills: 0,
        embedded_skills: 0,
        failed_skills: 0,
      },
    } as never)

    const wrapper = await mountSkillStore()

    expect(wrapper.find('.discover-progress').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('Embedding skill search index')
  })

  it('treats the top pills as collection filters and only curates the featured tab', async () => {
    const wrapper = await mountSkillStore()

    vi.mocked(skillApi.searchMarket).mockClear()

    await wrapper.get('.sort-pill:nth-child(2)').trigger('click')
    await flushPromises()

    expect(skillApi.searchMarket).toHaveBeenCalledTimes(1)
    expect(vi.mocked(skillApi.searchMarket).mock.calls[0]?.[0]).toMatchObject({
      sort: 'trending',
      page: 1,
      page_size: 20,
    })
    expect(vi.mocked(skillApi.searchMarket).mock.calls[0]?.[0]).not.toHaveProperty('curated')
  })

  it('shows advisor suggestions for natural language search and lets users reuse them', async () => {
    vi.mocked(skillApi.adviseMarket).mockResolvedValue({
      data: {
        query: 'github actions release automation',
        need_store_search: true,
        search_queries: [
          'github actions release automation',
          'release automation changelog',
          'github actions workflow automation',
        ],
        capability_tags: ['github-actions', 'release', 'changelog'],
        recommended_ids: ['gh-release-bot'],
        results: [
          {
            score: 21,
            skill: makeSkill({
              id: 'gh-release-bot',
              name: 'GitHub Release Bot',
              description: 'Automates release notes and changelog publishing.',
              tags: ['github-actions', 'release', 'changelog'],
            }),
          },
        ],
      },
    } as never)

    const wrapper = await mountSkillStore()

    await wrapper.get('[data-testid="search-input"]').setValue('github actions release automation')
    wrapper.findComponent({ name: 'SemanticSearchField' }).vm.$emit('submit-shortcut')
    await flushPromises()

    expect(skillApi.adviseMarket).toHaveBeenCalledWith({
      query: 'github actions release automation',
    })

    const advisor = wrapper.get('.advisor-panel')
    expect(advisor.text()).toContain('Recommended skills for this task')
    expect(advisor.text()).toContain('release automation changelog')
    expect(advisor.text()).toContain('github-actions')
    expect(advisor.text()).toContain('GitHub Release Bot')

    vi.mocked(skillApi.searchMarket).mockClear()

    await wrapper.get('.advisor-chip-button').trigger('click')
    await flushPromises()

    expect(vi.mocked(skillApi.searchMarket).mock.calls[0]?.[0]).toMatchObject({
      q: 'release automation changelog',
      curated: true,
      sort: 'featured',
      page: 1,
      page_size: 20,
      semantic: true,
    })
    expect((wrapper.get('[data-testid="search-input"]').element as HTMLInputElement).value).toBe(
      'release automation changelog'
    )
  })

  it('prefills routed marketplace searches and fetches advisor guidance on mount', async () => {
    vi.mocked(skillApi.adviseMarket).mockResolvedValue({
      data: {
        query: 'release automation changelog',
        need_store_search: true,
        search_queries: ['release automation changelog'],
      },
    } as never)

    const wrapper = await mountSkillStore({
      initialSearchQuery: 'release automation changelog',
    })

    expect((wrapper.get('[data-testid="search-input"]').element as HTMLInputElement).value).toBe(
      'release automation changelog'
    )
    expect(skillApi.searchMarket).toHaveBeenCalledWith(
      expect.objectContaining({
        q: 'release automation changelog',
      })
    )
    expect(skillApi.adviseMarket).toHaveBeenCalledWith({
      query: 'release automation changelog',
    })
  })
})
