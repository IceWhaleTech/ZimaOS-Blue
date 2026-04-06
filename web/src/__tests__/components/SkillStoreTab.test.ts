import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import SkillStoreTab from '@/components/extensions/SkillStoreTab.vue'
import { i18n, setLocale } from '@/i18n'
import { skillApi } from '@/api/skill'
import { useNotificationStore } from '@/stores/notification'

enableAutoUnmount(afterEach)

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
    listSources: vi.fn(),
    previewSourceImport: vi.fn(),
    addSource: vi.fn(),
    installFromURL: vi.fn(),
    removeSource: vi.fn(),
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

function makeSource(overrides: Record<string, unknown> = {}) {
  return {
    id: 'tencent-skillhub',
    name: 'Tencent SkillHub',
    url: 'https://skillhub.example.com/catalog',
    type: 'lightmake_api',
    enabled: true,
    display_name: 'Tencent SkillHub',
    base_url: 'https://skillhub.example.com/catalog',
    source_group: 'skillhub',
    auth_mode: 'none',
    rate_limit_per_minute: 60,
    priority: 10,
    ...overrides,
  }
}

function makeFiltersResponse() {
  return {
    data: {
      categories: [{ value: 'development_tools', label: 'Development Tools', count: 1 }],
      sources: [{ value: 'skillhub', label: 'Tencent SkillHub', count: 1 }],
      risk_badges: [{ value: 'green', label: 'Security', count: 1 }],
      install_types: [],
      artifact_kinds: [],
      installable: { true: 1 },
      security_signals: {},
    },
  }
}

function makeDetailResponse(
  skill: Record<string, unknown>,
  overrides: Record<string, unknown> = {}
) {
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
      ...overrides,
    },
  }
}

async function mountSkillStore(props: Record<string, unknown> = {}) {
  const wrapper = mount(SkillStoreTab, {
    props,
    global: {
      plugins: [createPinia(), i18n],
    },
  })
  await flushPromises()
  return wrapper
}

async function mountSkillStoreWithPinia(props: Record<string, unknown> = {}) {
  const pinia = createPinia()
  const wrapper = mount(SkillStoreTab, {
    props,
    global: {
      plugins: [pinia, i18n],
    },
  })
  await flushPromises()
  return { wrapper, pinia }
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
    vi.mocked(skillApi.listSources).mockResolvedValue({
      data: [makeSource()],
    } as never)
    vi.mocked(skillApi.previewSourceImport).mockResolvedValue({
      data: {
        url: 'https://catalog.example.com/skills',
        normalized_url: 'https://catalog.example.com/skills',
        kind: 'source',
        confidence: 'medium',
        suggested_source: {
          id: 'user-catalog-example-com',
          name: 'Catalog Example',
          url: 'https://catalog.example.com/skills',
          type: 'html_catalog',
          enabled: true,
          display_name: 'Catalog Example',
          base_url: 'https://catalog.example.com/skills',
          source_group: 'example',
          auth_mode: 'none',
          rate_limit_per_minute: 20,
          priority: 250,
        },
      },
    } as never)
    vi.mocked(skillApi.addSource).mockResolvedValue({
      data: {
        success: true,
        message: 'source added',
      },
    } as never)
    vi.mocked(skillApi.installFromURL).mockResolvedValue({
      data: {
        success: true,
        skill: {
          id: 'url-seed-skill',
          name: 'URL Seed Skill',
        },
      },
    } as never)
    vi.mocked(skillApi.removeSource).mockResolvedValue({
      data: {
        success: true,
        message: 'source removed',
      },
    } as never)
  })

  afterEach(() => {
    vi.useRealTimers()
    sseState.handler = null
    sseState.embeddingHandler = null
  })

  it('keeps the toolbar free of curated/installable quick tags', async () => {
    const wrapper = await mountSkillStore()

    expect(wrapper.findAll('.toolbar-controls .sort-pill')).toHaveLength(3)
    expect(wrapper.findAll('.toolbar-controls button')).toHaveLength(3)
    expect(wrapper.text()).not.toContain('Featured')
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
      sort: 'trending',
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
      sort: 'trending',
      page: 1,
      page_size: 20,
    })
    expect(vi.mocked(skillApi.searchMarket).mock.calls[0]?.[0]).not.toHaveProperty('curated')
    expect(vi.mocked(skillApi.searchMarket).mock.calls[1]?.[0]).toMatchObject({
      sort: 'trending',
      page: 1,
      page_size: 20,
    })
    expect(vi.mocked(skillApi.searchMarket).mock.calls[1]?.[0]).not.toHaveProperty('curated')
    expect(wrapper.findAll('.skill-card')).toHaveLength(2)
    expect(wrapper.get('.panel-header').text()).toContain('2 skills')

    await vi.advanceTimersByTimeAsync(2100)
    await flushPromises()
  })

  it('keeps cached results visible while long-running discover continues in the background', async () => {
    vi.useFakeTimers()
    vi.mocked(skillApi.discoverStatus).mockReset()
    vi.mocked(skillApi.discoverStatus).mockResolvedValue({
      data: {
        running: true,
        started_at: '2026-03-22T00:00:00Z',
        total_sources: 3,
        processed_sources: 1,
        current_source_name: 'Tencent SkillHub',
      },
    } as never)

    const wrapper = await mountSkillStore()

    expect(wrapper.findAll('.skill-card')).toHaveLength(1)
    expect(wrapper.find('.error-banner').exists()).toBe(false)

    await vi.advanceTimersByTimeAsync(60_000)
    await flushPromises()

    expect(wrapper.findAll('.skill-card')).toHaveLength(1)
    expect(wrapper.find('.error-banner').exists()).toBe(false)
    expect(wrapper.get('.discover-progress').text()).toContain('Tencent SkillHub')
  })

  it('submits refresh as a background discover task instead of waiting in the foreground', async () => {
    vi.useFakeTimers()
    vi.mocked(skillApi.discoverRefresh).mockResolvedValue({
      data: {
        accepted: true,
        running: true,
        started_at: '2026-03-22T00:00:00Z',
        total_sources: 3,
        processed_sources: 0,
      },
    } as never)
    vi.mocked(skillApi.discoverStatus).mockReset()
    vi.mocked(skillApi.discoverStatus)
      .mockResolvedValueOnce({
        data: {
          running: false,
          finished_at: '2026-03-22T00:00:00Z',
          result: { sources_processed: 1, discovered: 1, updated: 0, failed: 0 },
        },
      } as never)
      .mockResolvedValue({
        data: {
          running: true,
          started_at: '2026-03-22T00:00:00Z',
          total_sources: 3,
          processed_sources: 1,
          current_source_name: 'Tencent SkillHub',
        },
      } as never)

    const wrapper = await mountSkillStore()

    await wrapper.get('.hero-actions button').trigger('click')
    await flushPromises()

    expect(skillApi.discoverRefresh).toHaveBeenCalledTimes(1)
    expect(wrapper.get('.hero-actions button').text()).toContain('Refresh sources')
    expect(wrapper.find('.error-banner').exists()).toBe(false)

    await vi.advanceTimersByTimeAsync(2100)
    await flushPromises()

    expect(skillApi.discoverStatus).toHaveBeenCalled()
    expect(wrapper.get('.discover-progress').text()).toContain('Tencent SkillHub')
  })

  it('previews and adds a marketplace source from the toolbar', async () => {
    const { wrapper, pinia } = await mountSkillStoreWithPinia()
    const notification = useNotificationStore(pinia)

    await wrapper.get('[data-testid="source-import-toggle"]').trigger('click')
    await wrapper
      .get('[data-testid="source-import-input"]')
      .setValue('https://catalog.example.com/skills')
    await wrapper.get('[data-testid="source-import-preview"]').trigger('click')
    await flushPromises()

    expect(skillApi.previewSourceImport).toHaveBeenCalledWith({
      url: 'https://catalog.example.com/skills',
    })
    expect(wrapper.get('[data-testid="source-import-preview-result"]').text()).toContain(
      'Catalog Example'
    )

    await wrapper.get('[data-testid="source-import-confirm"]').trigger('click')
    await flushPromises()

    expect(skillApi.addSource).toHaveBeenCalledWith(
      expect.objectContaining({
        id: 'user-catalog-example-com',
        type: 'html_catalog',
        url: 'https://catalog.example.com/skills',
      })
    )
    expect(skillApi.discoverRefresh).toHaveBeenCalledTimes(1)
    expect(notification.notifications[0]?.type).toBe('success')
    expect(wrapper.find('[data-testid="source-import-panel"]').exists()).toBe(false)
  })

  it('localizes source import copy and type labels in zh-CN', async () => {
    const previousLocalStorage = globalThis.localStorage
    Object.defineProperty(globalThis, 'localStorage', {
      configurable: true,
      value: {
        getItem: vi.fn(() => null),
        setItem: vi.fn(),
        removeItem: vi.fn(),
      },
    })

    await setLocale('zh-CN')

    const wrapper = await mountSkillStore()

    try {
      await wrapper.get('[data-testid="source-import-toggle"]').trigger('click')
      await wrapper
        .get('[data-testid="source-import-input"]')
        .setValue('https://catalog.example.com/skills')

      expect(wrapper.get('[data-testid="source-import-panel"]').text()).toContain(
        '从 URL 导入技能市场来源'
      )
      expect(wrapper.get('[data-testid="source-import-preview"]').text()).toContain('分析来源')

      await wrapper.get('[data-testid="source-import-preview"]').trigger('click')
      await flushPromises()

      const preview = wrapper.get('[data-testid="source-import-preview-result"]').text()
      expect(wrapper.get('[data-testid="source-import-confirm"]').text()).toContain('添加来源')
      expect(preview).toContain('HTML 目录')
      expect(preview).toContain('这个 URL 可以保存为可复用的技能市场来源。')
    } finally {
      await setLocale('en-US')
      Object.defineProperty(globalThis, 'localStorage', {
        configurable: true,
        value: previousLocalStorage,
      })
    }
  })

  it('shows configured sources in the import panel and lets users remove custom ones', async () => {
    vi.mocked(skillApi.listSources)
      .mockResolvedValueOnce({
        data: [
          makeSource(),
          makeSource({
            id: 'user-catalog-example-com',
            name: 'Catalog Example',
            url: 'https://catalog.example.com/skills',
            type: 'html_catalog',
            display_name: 'Catalog Example',
            base_url: 'https://catalog.example.com/skills',
            priority: 250,
          }),
        ],
      } as never)
      .mockResolvedValueOnce({
        data: [makeSource()],
      } as never)

    const { wrapper, pinia } = await mountSkillStoreWithPinia()
    const notification = useNotificationStore(pinia)

    await wrapper.get('[data-testid="source-import-toggle"]').trigger('click')
    await flushPromises()

    const configured = wrapper.get('[data-testid="configured-sources"]')
    expect(configured.text()).toContain('Tencent SkillHub')
    expect(configured.text()).toContain('Catalog Example')
    expect(configured.text()).toContain('Built-in')
    expect(wrapper.find('[data-testid="remove-source-tencent-skillhub"]').exists()).toBe(false)

    await wrapper.get('[data-testid="remove-source-user-catalog-example-com"]').trigger('click')
    await flushPromises()

    expect(skillApi.removeSource).toHaveBeenCalledWith('user-catalog-example-com')
    expect(wrapper.get('[data-testid="configured-sources"]').text()).not.toContain(
      'Catalog Example'
    )
    expect(notification.notifications[0]?.type).toBe('success')
  })

  it('marks an existing custom source as already configured after preview', async () => {
    vi.mocked(skillApi.listSources).mockResolvedValue({
      data: [
        makeSource({
          id: 'user-catalog-example-com',
          name: 'Catalog Example',
          url: 'https://catalog.example.com/skills',
          type: 'html_catalog',
          display_name: 'Catalog Example',
          base_url: 'https://catalog.example.com/skills',
          priority: 250,
        }),
      ],
    } as never)

    const wrapper = await mountSkillStore()

    await wrapper.get('[data-testid="source-import-toggle"]').trigger('click')
    await wrapper
      .get('[data-testid="source-import-input"]')
      .setValue('https://catalog.example.com/skills')
    await wrapper.get('[data-testid="source-import-preview"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="source-import-preview-result"]').text()).toContain(
      'Already configured'
    )
    expect(wrapper.find('[data-testid="source-import-confirm"]').exists()).toBe(false)
  })

  it('marks built-in previewed sources as protected', async () => {
    vi.mocked(skillApi.previewSourceImport).mockResolvedValue({
      data: {
        url: 'https://skillhub.example.com/catalog',
        normalized_url: 'https://skillhub.example.com/catalog',
        kind: 'source',
        confidence: 'high',
        suggested_source: makeSource(),
        message: 'Detected marketplace source',
      },
    } as never)

    const wrapper = await mountSkillStore()

    await wrapper.get('[data-testid="source-import-toggle"]').trigger('click')
    await wrapper
      .get('[data-testid="source-import-input"]')
      .setValue('https://skillhub.example.com/catalog')
    await wrapper.get('[data-testid="source-import-preview"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="source-import-preview-result"]').text()).toContain(
      'Built-in source'
    )
    expect(wrapper.find('[data-testid="source-import-confirm"]').exists()).toBe(false)
  })

  it('installs a previewed seed directly from the import panel', async () => {
    vi.mocked(skillApi.previewSourceImport).mockResolvedValue({
      data: {
        url: 'https://github.com/demo/skills-repo',
        normalized_url: 'https://github.com/demo/skills-repo',
        kind: 'seed',
        confidence: 'high',
        seed_type: 'github_repo',
        seed_value: 'demo/skills-repo',
        message: 'Looks like a GitHub repository seed rather than a long-lived store source.',
      },
    } as never)
    vi.mocked(skillApi.installFromURL).mockResolvedValue({
      data: {
        success: true,
        entry_file: 'CLAUDE.md',
        warnings: ['legacy manifest compatibility fallback applied while parsing imported skill'],
        contract_status: 'legacy_fallback',
        contract_source: 'legacy_frontmatter_fallback',
        skill: {
          id: 'url-seed-skill',
          name: 'URL Seed Skill',
        },
      },
    } as never)

    const { wrapper, pinia } = await mountSkillStoreWithPinia()
    const notification = useNotificationStore(pinia)

    await wrapper.get('[data-testid="source-import-toggle"]').trigger('click')
    await wrapper
      .get('[data-testid="source-import-input"]')
      .setValue('https://github.com/demo/skills-repo')
    await wrapper.get('[data-testid="source-import-preview"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="source-import-preview-result"]').text()).toContain(
      'demo/skills-repo'
    )
    expect(wrapper.get('[data-testid="source-import-install-seed"]').text()).toContain(
      'From URL'
    )

    await wrapper.get('[data-testid="source-import-install-seed"]').trigger('click')
    await flushPromises()

    expect(skillApi.installFromURL).toHaveBeenCalledWith({
      url: 'https://github.com/demo/skills-repo',
    })
    expect(notification.notifications[0]?.type).toBe('success')
    expect(wrapper.get('[data-testid="source-import-panel"]').exists()).toBe(true)
    const installResult = wrapper.get('[data-testid="source-import-install-result"]')
    expect(installResult.text()).toContain('URL Seed Skill')
    expect(installResult.text()).toContain('Entry file: CLAUDE.md')
    expect(installResult.text()).toContain(
      'Legacy skill format detected; compatibility defaults were applied.'
    )
  })

  it('allows dismissing the inline seed install result without closing the import panel', async () => {
    vi.mocked(skillApi.previewSourceImport).mockResolvedValue({
      data: {
        url: 'https://github.com/demo/skills-repo',
        normalized_url: 'https://github.com/demo/skills-repo',
        kind: 'seed',
        confidence: 'high',
        seed_type: 'github_repo',
        seed_value: 'demo/skills-repo',
        message: 'Looks like a GitHub repository seed rather than a long-lived store source.',
      },
    } as never)
    vi.mocked(skillApi.installFromURL).mockResolvedValue({
      data: {
        success: true,
        entry_file: 'CLAUDE.md',
        skill: {
          id: 'url-seed-skill',
          name: 'URL Seed Skill',
        },
      },
    } as never)

    const wrapper = await mountSkillStore()

    await wrapper.get('[data-testid="source-import-toggle"]').trigger('click')
    await wrapper
      .get('[data-testid="source-import-input"]')
      .setValue('https://github.com/demo/skills-repo')
    await wrapper.get('[data-testid="source-import-preview"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="source-import-install-seed"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="source-import-install-result"]').exists()).toBe(true)

    await wrapper.get('[data-testid="source-import-install-result-dismiss"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="source-import-install-result"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="source-import-panel"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="source-import-input"]').exists()).toBe(true)
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
      sort: 'trending',
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

  it('treats the top pills as sort filters without curated requests', async () => {
    const wrapper = await mountSkillStore()

    vi.mocked(skillApi.searchMarket).mockClear()

    await wrapper.get('.sort-pill:nth-child(2)').trigger('click')
    await flushPromises()

    expect(skillApi.searchMarket).toHaveBeenCalledTimes(1)
    expect(vi.mocked(skillApi.searchMarket).mock.calls[0]?.[0]).toMatchObject({
      sort: 'newest',
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
      sort: 'trending',
      page: 1,
      page_size: 20,
      semantic: true,
    })
    expect(vi.mocked(skillApi.searchMarket).mock.calls[0]?.[0]).not.toHaveProperty('curated')
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

  it('shows contract metadata in the installed skill detail modal', async () => {
    const installedSkill = makeSkill({
      installed: true,
    })
    vi.mocked(skillApi.searchMarket).mockResolvedValue(
      makeSearchResponse([installedSkill]) as never
    )
    vi.mocked(skillApi.getMarketplaceSkill).mockResolvedValue(
      makeDetailResponse(installedSkill, {
        installed: true,
        enabled: true,
        contract_status: 'legacy_fallback',
        contract_source: 'legacy_frontmatter_fallback',
        contract_notes: [
          'legacy manifest compatibility fallback applied: missing metadata fields were backfilled.',
        ],
      }) as never
    )

    const wrapper = await mountSkillStore()

    await wrapper.get('.skill-card').trigger('click')
    await flushPromises()

    expect(skillApi.getMarketplaceSkill).toHaveBeenCalledWith('skill-alpha')
    expect(document.body.querySelector('.detail-contract-panel')).not.toBeNull()
    expect(document.body.textContent).toContain('Legacy fallback')
    expect(document.body.textContent).toContain('Legacy frontmatter fallback')
    expect(document.body.textContent).toContain(
      'legacy manifest compatibility fallback applied: missing metadata fields were backfilled.'
    )

    wrapper.unmount()
  })

  it('shows upstream source details for github awesome aggregated skills in the detail modal', async () => {
    const aggregatedSkill = makeSkill({
      source_id: 'github-awesome-skills',
      source_name: 'GitHub Awesome Skills',
      source_group: 'github-awesome-skills',
      origin_source_id: 'github-awesome-composio',
      origin_source_name: 'ComposioHQ Awesome Claude Skills',
      origin_source_url: 'https://github.com/ComposioHQ/awesome-claude-skills',
      homepage: 'https://github.com/demo/repo-seed',
      source_url: 'https://github.com/demo/repo-seed',
      download_url: 'https://raw.githubusercontent.com/demo/repo-seed/main/SKILL.md',
    })
    vi.mocked(skillApi.searchMarket).mockResolvedValue(
      makeSearchResponse([aggregatedSkill]) as never
    )
    vi.mocked(skillApi.getMarketplaceSkill).mockResolvedValue(
      makeDetailResponse(aggregatedSkill) as never
    )

    const wrapper = await mountSkillStore()

    await wrapper.get('.skill-card').trigger('click')
    await flushPromises()

    expect(document.body.textContent).toContain('GitHub Awesome Skills')
    expect(document.body.textContent).toContain('ComposioHQ Awesome Claude Skills')

    wrapper.unmount()
  })

  it('applies semantic tones to the security summary cards in the detail modal', async () => {
    const skill = makeSkill({
      security_badge: 'yellow',
      risk_level: 'medium',
    })
    const detail = makeDetailResponse(skill)
    detail.data.security = {
      ...detail.data.security,
      score: 100,
      risk_level: 'medium',
      security_badge: 'yellow',
      vulnerability_status: 'not_applicable',
      install_surface: {
        install_type: 'raw_skill',
        artifact_kind: 'open_source',
        installable: true,
        has_binary: false,
        has_scripts: false,
      },
    }

    vi.mocked(skillApi.getMarketplaceSkill).mockResolvedValue(detail as never)

    const wrapper = await mountSkillStore()

    await wrapper.get('.skill-card').trigger('click')
    await flushPromises()

    const scoreCard = document.body.querySelector(
      '.security-overview .score-card--score'
    ) as HTMLElement | null
    const badgeCard = document.body.querySelector(
      '.security-summary-grid .score-card--badge'
    ) as HTMLElement | null
    const vulnerabilityCard = document.body.querySelector(
      '.security-summary-grid .score-card--vulnerabilities'
    ) as HTMLElement | null
    const installableCard = document.body.querySelector(
      '.security-summary-grid .score-card--installable'
    ) as HTMLElement | null

    expect(scoreCard?.classList.contains('score-card--safe')).toBe(true)
    expect(badgeCard?.classList.contains('score-card--warn')).toBe(true)
    expect(vulnerabilityCard?.classList.contains('score-card--neutral')).toBe(true)
    expect(installableCard?.classList.contains('score-card--safe')).toBe(true)

    wrapper.unmount()
  })

  it('collapses risky security details by default in the detail modal', async () => {
    const skill = makeSkill({
      security_badge: 'yellow',
      risk_level: 'medium',
    })
    const detail = makeDetailResponse(skill)
    detail.data.security = {
      ...detail.data.security,
      permissions: ['network'],
      install_surface: {
        ...detail.data.security.install_surface,
        dependency_manifests: ['package.json'],
      },
      evidence: [
        {
          type: 'command_injection',
          severity: 'high',
          title: 'Command injection attempt detected',
          description: 'Shell or command injection pattern from shared threat detector',
          value: 'Matched: rm -rf /tmp/example',
        },
      ],
    }

    vi.mocked(skillApi.getMarketplaceSkill).mockResolvedValue(detail as never)

    const wrapper = await mountSkillStore()

    await wrapper.get('.skill-card').trigger('click')
    await flushPromises()

    const toggle = document.body.querySelector('.security-disclosure__toggle') as
      | HTMLButtonElement
      | null
    expect(toggle).not.toBeNull()
    expect(toggle?.getAttribute('aria-expanded')).toBe('false')
    expect(document.body.querySelector('.evidence-item')).toBeNull()
    expect(document.body.textContent || '').not.toContain('Command injection attempt detected')

    toggle?.click()
    await flushPromises()

    expect(toggle?.getAttribute('aria-expanded')).toBe('true')
    expect(document.body.querySelector('.evidence-item')).not.toBeNull()
    expect(document.body.textContent || '').toContain('Command injection attempt detected')

    wrapper.unmount()
  })

  it('shows a toast when a skill is blocked by policy', async () => {
    const blockedSkill = makeSkill({
      security_badge: 'red',
      risk_level: 'high',
    })
    vi.mocked(skillApi.searchMarket).mockResolvedValue(makeSearchResponse([blockedSkill]) as never)

    const { wrapper, pinia } = await mountSkillStoreWithPinia()
    const notificationStore = useNotificationStore(pinia)

    await wrapper.get('.skill-card .install-button').trigger('click')
    await flushPromises()

    expect(skillApi.installMarket).not.toHaveBeenCalled()
    expect(notificationStore.notifications.length).toBeGreaterThan(0)
    expect(notificationStore.notifications[0]?.type).toBe('error')
    expect(notificationStore.notifications[0]?.message).toContain(
      'This skill is blocked by the security policy.'
    )

    wrapper.unmount()
  })

  it('installs yellow skills directly without a risk acknowledgement modal', async () => {
    const reviewSkill = makeSkill({
      security_badge: 'yellow',
      risk_level: 'medium',
    })
    vi.mocked(skillApi.searchMarket).mockResolvedValue(makeSearchResponse([reviewSkill]) as never)

    const { wrapper } = await mountSkillStoreWithPinia()

    await wrapper.get('.skill-card .install-button').trigger('click')
    await flushPromises()

    expect(skillApi.installMarket).toHaveBeenCalledWith({
      id: 'skill-alpha',
      ack_risk: false,
      force_install: false,
    })
    expect(document.body.textContent || '').not.toContain('Risk acknowledgement required')

    wrapper.unmount()
  })

  it('localizes install warnings returned by the backend', async () => {
    const zhCN = await import('@/i18n/locales/zh-CN')
    ;(i18n.global as any).setLocaleMessage('zh-CN', zhCN.default)
    i18n.global.locale.value = 'zh-CN'

    vi.mocked(skillApi.installMarket).mockResolvedValue({
      data: {
        warnings: [
          'Installed payload scan escalated this skill from medium risk to high risk. Review the security report before using this skill.',
          'legacy manifest compatibility fallback applied: missing metadata fields were backfilled.',
        ],
      },
    } as never)

    const { wrapper, pinia } = await mountSkillStoreWithPinia()
    const notificationStore = useNotificationStore(pinia)

    await wrapper.get('.skill-card .install-button').trigger('click')
    await flushPromises()

    const warningNotifications = notificationStore.notifications.filter((n) => n.type === 'warning')
    expect(warningNotifications.length).toBeGreaterThan(0)

    const warningMessage = warningNotifications[0]?.message || ''
    expect(warningMessage).toContain('安装包扫描将该技能从中风险升级为高风险')
    expect(warningMessage).toContain('检测到旧版技能格式；已应用兼容性默认设置。')

    wrapper.unmount()
  })

  it('localizes raw security evidence fields in the detail modal', async () => {
    const zhCN = await import('@/i18n/locales/zh-CN')
    ;(i18n.global as any).setLocaleMessage('zh-CN', zhCN.default)
    i18n.global.locale.value = 'zh-CN'

    const skill = makeSkill()
    const detail = makeDetailResponse(skill)
    detail.data.security = {
      ...detail.data.security,
      risk_level: 'critical',
      permissions: ['network'],
      evidence: [
        {
          type: 'command_injection',
          severity: 'critical',
          title: 'Command injection attempt detected',
          description: 'Shell or command injection pattern from shared threat detector',
          value: 'Matched: `',
        },
        {
          type: 'permission',
          severity: 'medium',
          title: 'network',
          description: 'Privileged capability inferred from skill content but not declared',
          value: 'network',
        },
      ],
    }

    vi.mocked(skillApi.getMarketplaceSkill).mockResolvedValue(detail as never)

    const wrapper = await mountSkillStore()

    await wrapper.get('.skill-card').trigger('click')
    await flushPromises()

    const toggle = document.body.querySelector('.security-disclosure__toggle') as
      | HTMLButtonElement
      | null
    toggle?.click()
    await flushPromises()

    const bodyText = document.body.textContent || ''
    expect(bodyText).toContain('命令注入')
    expect(bodyText).toContain('严重')
    expect(bodyText).toContain('共享威胁检测器检测到 shell 或命令注入模式')
    expect(bodyText).toContain('网络')
    expect(bodyText).toContain('匹配项')
    expect(bodyText).not.toContain('Command injection attempt detected')
    expect(bodyText).not.toContain('Shell or command injection pattern from shared threat detector')
    expect(bodyText).not.toContain(
      'Privileged capability inferred from skill content but not declared'
    )

    wrapper.unmount()
    i18n.global.locale.value = 'en-US'
  })

  it('localizes marketplace source labels and descriptions', async () => {
    const zhCN = await import('@/i18n/locales/zh-CN')
    ;(i18n.global as any).setLocaleMessage('zh-CN', zhCN.default)
    i18n.global.locale.value = 'zh-CN'

    vi.mocked(skillApi.discoverStatus).mockResolvedValue({
      data: {
        running: true,
        total_sources: 2,
        processed_sources: 0,
        current_source_name: 'Tencent SkillHub',
      },
    } as never)
    vi.mocked(skillApi.filtersMarket).mockResolvedValue({
      data: {
        categories: [{ value: 'development_tools', label: 'Development Tools', count: 1 }],
        sources: [{ value: 'skillhub', label: 'Tencent SkillHub', count: 2 }],
        risk_badges: [{ value: 'green', label: 'Security', count: 1 }],
        install_types: [],
        artifact_kinds: [],
        installable: { true: 1 },
        security_signals: {},
      },
    } as never)

    const wrapper = await mountSkillStore()

    expect(wrapper.text()).toContain('腾讯 SkillHub')
    expect(wrapper.text()).toContain('腾讯 SkillHub 官方技能目录源。')

    const sourceChip = wrapper.get('.source-chip')
    expect(sourceChip.text()).toBe('腾讯 SkillHub')
    expect(sourceChip.attributes('title')).toBe('腾讯 SkillHub 官方技能目录源。')

    const sourceOptions = wrapper.findAll('select.filter-select')[1]?.findAll('option') || []
    expect(sourceOptions[1]?.text()).toContain('SkillHub')
    expect(sourceOptions[1]?.attributes('title')).toBe(
      '聚合自腾讯 SkillHub 与 SkillHub Club 的 SkillHub 目录。'
    )

    await wrapper.findAll('select.filter-select')[1]!.setValue('skillhub')
    await flushPromises()
    expect(wrapper.text()).toContain('聚合自腾讯 SkillHub 与 SkillHub Club 的 SkillHub 目录。')

    wrapper.unmount()
    i18n.global.locale.value = 'en-US'
  })

  it('localizes category-like skill tags on cards and uses the status chip for installed badges', async () => {
    vi.mocked(skillApi.searchMarket).mockResolvedValue(
      makeSearchResponse([
        makeSkill({
          installed: true,
          tags: ['AI 智能', 'productivity', 'automation'],
        }),
      ]) as never
    )

    const wrapper = await mountSkillStore()

    const renderedTags = wrapper
      .findAll('.skill-card .card-tag-row .meta-chip-soft')
      .map((tag) => tag.text())

    expect(renderedTags).toContain('AI Intelligence')
    expect(renderedTags).toContain('Productivity')
    expect(renderedTags).not.toContain('AI 智能')
    expect(renderedTags).not.toContain('productivity')

    const installedChip = wrapper.get('.skill-card .meta-chip-installed')
    expect(installedChip.classes()).toContain('meta-chip-status')

    wrapper.unmount()
  })

  it('localizes developer-tools tags on cards', async () => {
    vi.mocked(skillApi.searchMarket).mockResolvedValue(
      makeSearchResponse([
        makeSkill({
          tags: ['developer-tools'],
        }),
      ]) as never
    )

    const wrapper = await mountSkillStore()
    const renderedTags = wrapper
      .findAll('.skill-card .card-tag-row .meta-chip-soft')
      .map((tag) => tag.text())

    expect(renderedTags).toContain('Development Tools')
    expect(renderedTags).not.toContain('developer-tools')

    wrapper.unmount()
  })

  it('localizes productivity tags for zh-CN cards', async () => {
    const zhCN = await import('@/i18n/locales/zh-CN')
    ;(i18n.global as any).setLocaleMessage('zh-CN', zhCN.default)
    i18n.global.locale.value = 'zh-CN'

    vi.mocked(skillApi.searchMarket).mockResolvedValue(
      makeSearchResponse([
        makeSkill({
          tags: ['productivity'],
        }),
      ]) as never
    )

    const wrapper = await mountSkillStore()
    const renderedTags = wrapper
      .findAll('.skill-card .card-tag-row .meta-chip-soft')
      .map((tag) => tag.text())

    expect(renderedTags).toContain('效率提升')
    expect(renderedTags).not.toContain('productivity')

    wrapper.unmount()
    i18n.global.locale.value = 'en-US'
  })

  it('renders source brand icons in result cards, progress, and source filter hints', async () => {
    vi.mocked(skillApi.discoverStatus).mockResolvedValue({
      data: {
        running: true,
        total_sources: 2,
        processed_sources: 0,
        current_source_name: 'Tencent SkillHub',
      },
    } as never)

    const wrapper = await mountSkillStore()

    const sourceChipIcon = wrapper.get('.source-chip img')
    expect(sourceChipIcon.attributes('src')).toContain('skillhub.club/favicon-48x48.png')

    const progressSourceIcon = wrapper.get('.discover-progress__source img')
    expect(progressSourceIcon.attributes('src')).toContain('skillhub.club/favicon-48x48.png')

    await wrapper.findAll('select.filter-select')[1]!.setValue('skillhub')
    await flushPromises()

    const filterHintIcon = wrapper.get('.filter-field__hint img')
    expect(filterHintIcon.attributes('src')).toContain('skillhub.club/favicon-48x48.png')
  })

  it('shows MiniMax branding for GitHub MiniMax skills', async () => {
    const minimaxSkill = makeSkill({
      source_id: 'github-skill-md',
      source_name: 'GitHub SKILL.md',
      source_group: 'github',
      author: 'MiniMax',
      homepage: 'https://github.com/MiniMax-AI/skills/tree/main/skills/minimax-pdf',
      source_url: 'https://github.com/MiniMax-AI/skills/tree/main/skills/minimax-pdf',
    })

    vi.mocked(skillApi.searchMarket).mockResolvedValue(makeSearchResponse([minimaxSkill]) as never)

    const wrapper = await mountSkillStore()

    const sourceChip = wrapper.get('.source-chip')
    expect(sourceChip.text()).toContain('MiniMax')
    expect(sourceChip.text()).not.toContain('GitHub')

    const sourceChipIcon = wrapper.get('.source-chip img')
    expect(sourceChipIcon.attributes('src')).toContain('/icons/providers/minimax.svg')
  })
})
