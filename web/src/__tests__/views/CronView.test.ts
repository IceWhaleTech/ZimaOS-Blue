import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import CronView from '@/views/CronView.vue'
import { i18n, setLocale } from '@/i18n'
import { cronApi } from '@/api/cron'

vi.mock('@/api/cron', () => ({
  cronApi: {
    list: vi.fn(),
    getExecutions: vi.fn(),
    create: vi.fn(),
    update: vi.fn(),
    delete: vi.fn(),
    enable: vi.fn(),
    disable: vi.fn(),
    trigger: vi.fn(),
  },
}))

vi.mock('@/components/automation/AutomationTabs.vue', () => ({
  default: { name: 'AutomationTabs', template: '<div class="automation-tabs-stub"></div>' },
}))

const storageState = new Map<string, string>()
const localStorageMock = {
  getItem: (key: string) => storageState.get(key) ?? null,
  setItem: (key: string, value: string) => {
    storageState.set(key, value)
  },
  removeItem: (key: string) => {
    storageState.delete(key)
  },
  clear: () => {
    storageState.clear()
  },
}

describe('CronView', () => {
  beforeEach(async () => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    storageState.clear()
    vi.stubGlobal('localStorage', localStorageMock)
    if (typeof window !== 'undefined') {
      Object.defineProperty(window, 'localStorage', {
        value: localStorageMock,
        configurable: true,
      })
    }
    vi.mocked(cronApi.list).mockResolvedValue({
      data: [
        {
          id: 'job-1',
          name: 'Daily backup',
          description: 'Back up important files',
          schedule: '0 0 * * * *',
          handler: 'command',
          enabled: true,
          status: 'active',
          last_run_at: '2026-03-07T00:00:00Z',
          next_run_at: '2026-03-08T00:00:00Z',
          created_at: '2026-03-01T00:00:00Z',
          updated_at: '2026-03-01T00:00:00Z',
          run_count: 3,
          fail_count: 0,
          payload: { command: 'echo ok' },
        },
      ],
    } as never)
    vi.mocked(cronApi.create).mockResolvedValue({
      data: {
        id: 'job-created',
      },
    } as never)
    vi.mocked(cronApi.getExecutions).mockResolvedValue({
      data: [
        {
          id: 'exec-1',
          job_id: 'job-1',
          status: 'completed',
          started_at: '2026-03-08T00:00:00Z',
          ended_at: '2026-03-08T00:00:02Z',
          duration: 2_000_000,
        },
      ],
    } as never)
    await setLocale('en-US')
  })

  it('renders next run from next_run_at', async () => {
    const wrapper = mount(CronView, {
      global: {
        plugins: [createPinia(), i18n],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('Daily backup')
    expect(wrapper.text()).toContain('2026')
    expect(wrapper.text()).not.toContain('cron.calculating')
  })

  it('humanizes machine-style task names in the list', async () => {
    vi.mocked(cronApi.list).mockResolvedValueOnce({
      data: [
        {
          id: 'job-2',
          name: 'uptime_check',
          schedule: '0 0 * * * *',
          handler: 'command',
          enabled: true,
          status: 'active',
          created_at: '2026-03-01T00:00:00Z',
          updated_at: '2026-03-01T00:00:00Z',
          run_count: 1,
          fail_count: 0,
          payload: { command: 'uptime' },
        },
      ],
    } as never)

    const wrapper = mount(CronView, {
      global: {
        plugins: [createPinia(), i18n],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('Uptime Check')
  })

  it('renders execution duration from duration nanoseconds', async () => {
    const wrapper = mount(CronView, {
      global: {
        plugins: [createPinia(), i18n],
      },
    })

    await flushPromises()
    await wrapper.get('button[title="View executions"]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('2ms')
    expect(wrapper.text()).toContain('completed')
  })

  it('keeps optional settings collapsed until expanded in the create modal', async () => {
    const wrapper = mount(CronView, {
      global: {
        plugins: [createPinia(), i18n],
      },
    })

    await flushPromises()
    await wrapper.get('.automation-create-button').trigger('click')

    expect(wrapper.find('.automation-modal-panel--form').exists()).toBe(true)
    expect(wrapper.find('.automation-modal-scroll--form').exists()).toBe(true)
    expect(wrapper.find('#cron-optional-settings').exists()).toBe(false)

    await wrapper.get('.automation-disclosure-button').trigger('click')

    expect(wrapper.find('#cron-optional-settings').exists()).toBe(true)
  })

  it('does not submit the default timeout for http tasks', async () => {
    const wrapper = mount(CronView, {
      global: {
        plugins: [createPinia(), i18n],
      },
    })

    await flushPromises()
    await wrapper.get('.automation-create-button').trigger('click')

    const handlerCards = wrapper.findAll('.automation-handler-card')
    await handlerCards[1]!.trigger('click')

    const textInputs = wrapper.findAll('input[type="text"]')
    await textInputs[0]!.setValue('Status webhook')
    await textInputs[1]!.setValue('0 * * * *')
    await wrapper.get('input[type="url"]').setValue('https://example.com/hook')
    await wrapper.get('form.automation-form--modal').trigger('submit')
    await flushPromises()

    expect(cronApi.create).toHaveBeenCalledTimes(1)

    const request = vi.mocked(cronApi.create).mock.calls[0]?.[0]
    expect(request).toMatchObject({
      name: 'Status webhook',
      schedule: '0 * * * *',
      handler: 'http',
      payload: {
        url: 'https://example.com/hook',
        method: 'GET',
      },
    })
    expect(request?.payload).not.toHaveProperty('timeout')
  })

  it('auto-generates a readable title when creating a task without a name', async () => {
    const wrapper = mount(CronView, {
      global: {
        plugins: [createPinia(), i18n],
      },
    })

    await flushPromises()
    await wrapper.get('.automation-create-button').trigger('click')

    const handlerCards = wrapper.findAll('.automation-handler-card')
    await handlerCards[1]!.trigger('click')

    const textInputs = wrapper.findAll('input[type="text"]')
    await textInputs[1]!.setValue('0 * * * *')
    await wrapper.get('input[type="url"]').setValue('https://example.com/hook')
    await wrapper.get('form.automation-form--modal').trigger('submit')
    await flushPromises()

    const request = vi.mocked(cronApi.create).mock.calls.at(-1)?.[0]
    expect(request).toBeDefined()
    expect(request?.name).toMatch(/^(Request|请求) example\.com\/hook$/)
    expect(request).toMatchObject({
      schedule: '0 * * * *',
      handler: 'http',
      payload: {
        url: 'https://example.com/hook',
        method: 'GET',
      },
    })
  })

  it('localizes the nightly knowledge lint system job in zh-CN', async () => {
    await setLocale('zh-CN')
    vi.mocked(cronApi.list).mockResolvedValueOnce({
      data: [
        {
          id: 'job-knowledge-lint',
          name: 'Knowledge Nightly Lint',
          description: 'nightly knowledge lint',
          schedule: '0 0 3 * * *',
          handler: 'knowledge_lint',
          enabled: true,
          status: 'active',
          created_at: '2026-03-01T00:00:00Z',
          updated_at: '2026-03-01T00:00:00Z',
          run_count: 4,
          fail_count: 0,
          payload: {},
        },
      ],
    } as never)

    const wrapper = mount(CronView, {
      global: {
        plugins: [createPinia(), i18n],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('知识夜间检查')
    expect(wrapper.text()).toContain('每晚运行知识检查，维持编译知识健康度。')
    expect(wrapper.text()).not.toContain('Knowledge Nightly Lint')
    expect(wrapper.text()).not.toContain('nightly knowledge lint')
  })
})
