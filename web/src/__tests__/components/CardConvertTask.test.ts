import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import CardConvertTask from '@/components/typeless/CardConvertTask.vue'
import type { TypelessCardConvertTask } from '@/types/typeless'
import { i18n } from '@/i18n'
import enUS from '@/i18n/locales/en-US'
import zhCN from '@/i18n/locales/zh-CN'

const { revealInFileManagerMock } = vi.hoisted(() => ({
  revealInFileManagerMock: vi.fn(),
}))

vi.mock('@/composables/useTauri', () => ({
  useTauri: () => ({
    revealInFileManager: revealInFileManagerMock,
  }),
}))

vi.mock('@/api/client', () => ({
  authFetch: (input: RequestInfo | URL, init?: RequestInit) => fetch(input, init),
}))

function makeCard(overrides: Partial<TypelessCardConvertTask> = {}): TypelessCardConvertTask {
  return {
    type: 'convert-task',
    id: 'convert-task-task-1',
    task_id: 'task-1',
    status: 'processing',
    action: 'tts',
    source_summary: 'demo source',
    target_format: 'wav',
    progress: 35,
    message: 'Processing',
    outputs: [],
    ...overrides,
  }
}

function mockResponse(data: unknown, ok = true): Response {
  return {
    ok,
    json: vi.fn().mockResolvedValue(data),
  } as unknown as Response
}

function mockBlobResponse(body: string, type: string, ok = true): Response {
  return {
    ok,
    blob: vi.fn().mockResolvedValue(new Blob([body], { type })),
  } as unknown as Response
}

function applyLocale(locale: 'en-US' | 'zh-CN') {
  if (locale === 'en-US') {
    i18n.global.setLocaleMessage('en-US', enUS as never)
  } else {
    i18n.global.setLocaleMessage('en-US', enUS as never)
    i18n.global.setLocaleMessage('zh-CN', zhCN as never)
  }
  i18n.global.locale.value = locale
}

describe('CardConvertTask', () => {
  let fetchMock: ReturnType<typeof vi.fn>

  beforeEach(() => {
    vi.useFakeTimers()
    fetchMock = vi.fn()
    global.fetch = fetchMock as typeof fetch
    revealInFileManagerMock.mockReset()
    revealInFileManagerMock.mockResolvedValue(true)
    ;(URL as unknown as { createObjectURL: (blob: Blob) => string }).createObjectURL = vi
      .fn()
      .mockReturnValue('blob:convert-task-1')
    ;(URL as unknown as { revokeObjectURL: (url: string) => void }).revokeObjectURL = vi.fn()
    applyLocale('en-US')
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('polls running tasks and renders localized action and audio output when finished', async () => {
    fetchMock.mockResolvedValueOnce(
      mockResponse({
        task_id: 'task-1',
        status: 'succeeded',
        action: 'tts',
        target_format: 'wav',
        message: 'Speech audio created',
        outputs: [
          {
            output_id: 'out-1',
            name: 'speech.wav',
            preview_kind: 'audio',
            path: '/tmp/exports/speech.wav',
            ref: 'out:task-1:out-1',
            download_url: '/api/v1/convert/tasks/task-1/download/out-1',
          },
        ],
      })
    )
    fetchMock.mockResolvedValueOnce(mockBlobResponse('wav-data', 'audio/wav'))

    const wrapper = mount(CardConvertTask, {
      props: {
        card: makeCard(),
      },
      global: {
        plugins: [i18n],
      },
    })

    await vi.advanceTimersByTimeAsync(2000)
    await flushPromises()
    await flushPromises()

    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      '/api/v1/convert/tasks/task-1',
      undefined
    )
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      '/api/v1/convert/tasks/task-1/download/out-1',
      undefined
    )
    expect(wrapper.text()).toContain('Text-to-Speech')
    expect(wrapper.text()).toContain('Speech audio created')
    expect(wrapper.find('audio').exists()).toBe(true)
    expect(wrapper.find('audio').attributes('src')).toBe('blob:convert-task-1')
    expect(wrapper.text()).toContain('Download audio')
    expect(wrapper.text()).toContain('/tmp/exports/speech.wav')
    expect(wrapper.text()).not.toContain('out:task-1:out-1')
    const hasCancelButton = wrapper
      .findAll('button')
      .some((button) => button.text().includes('Cancel'))
    expect(hasCancelButton).toBe(false)
  })

  it('cancels a running task through the REST endpoint', async () => {
    fetchMock.mockResolvedValueOnce(
      mockResponse({
        task_id: 'task-1',
        status: 'cancelled',
        action: 'convert',
        target_format: 'pdf',
        message: 'Task cancelled',
        outputs: [],
      })
    )

    const wrapper = mount(CardConvertTask, {
      props: {
        card: makeCard({ action: 'convert', target_format: 'pdf' }),
      },
      global: {
        plugins: [i18n],
      },
    })

    await wrapper.get('button').trigger('click')
    await flushPromises()

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/convert/tasks/task-1/cancel',
      { method: 'POST' }
    )
    expect(wrapper.text()).toContain('Task cancelled')
    expect(wrapper.find('button').exists()).toBe(false)
  })

  it('renders transcript previews and text download actions for asr tasks', () => {
    const wrapper = mount(CardConvertTask, {
      props: {
        card: makeCard({
          status: 'succeeded',
          action: 'asr',
          target_format: 'txt',
          message: 'Transcription completed',
          transcript_preview: 'hello world from transcript',
          outputs: [
            {
              output_id: 'out-1',
              name: 'transcript.txt',
              preview_kind: 'text',
              preview_text: 'hello world from transcript',
              download_url: '/api/v1/convert/tasks/task-1/download/out-1',
            },
          ],
        }),
      },
      global: {
        plugins: [i18n],
      },
    })

    expect(wrapper.text()).toContain('Speech Recognition')
    expect(wrapper.text()).toContain('hello world from transcript')
    expect(wrapper.text()).toContain('Download text')
    expect(wrapper.find('audio').exists()).toBe(false)
  })

  it('renders compact source labels for multi-source tasks', () => {
    const wrapper = mount(CardConvertTask, {
      props: {
        card: makeCard({
          status: 'succeeded',
          action: 'convert',
          target_format: 'pdf',
          message: 'Completed',
          sources: [
            '/Users/orca/.zimaos-blue/data/workspace/phone_specs_2026/完整汇总表格.md',
            '/tmp/reports/summary-notes.txt',
          ],
        }),
      },
      global: {
        plugins: [i18n],
      },
    })

    expect(wrapper.text()).toContain('Sources')
    expect(wrapper.text()).toContain('完整汇总表格.md')
    expect(wrapper.text()).toContain('summary-notes.txt')
    expect(wrapper.text()).not.toContain('/Users/orca/.zimaos-blue/data/workspace/phone_specs_2026/完整汇总表格.md')
  })

  it('hides the sources section for single-source tasks when source summary already covers it', () => {
    const wrapper = mount(CardConvertTask, {
      props: {
        card: makeCard({
          status: 'succeeded',
          action: 'convert',
          target_format: 'pdf',
          message: 'Completed',
          sources: ['/Users/orca/.zimaos-blue/data/workspace/phone_specs_2026/完整汇总表格.md'],
        }),
      },
      global: {
        plugins: [i18n],
      },
    })

    expect(wrapper.text()).not.toContain('Sources')
    expect(wrapper.text()).toContain('demo source')
  })

  it('localizes stable action and message labels in zh-CN', () => {
    applyLocale('zh-CN')

    const wrapper = mount(CardConvertTask, {
      props: {
        card: makeCard({
          status: 'cancelled',
          action: 'tts',
          message: 'Task cancelled',
          outputs: [
            {
              output_id: 'out-1',
              name: 'speech.wav',
              preview_kind: 'audio',
              download_url: '/api/v1/convert/tasks/task-1/download/out-1',
            },
          ],
        }),
      },
      global: {
        plugins: [i18n],
      },
    })

    expect(wrapper.text()).toContain('语音合成')
    expect(wrapper.text()).toContain('任务已取消')
    expect(wrapper.text()).toContain('下载音频')
    expect(wrapper.text()).not.toContain('任务 task-1')
  })

  it('shows task id only while the convert task is still running', () => {
    const runningWrapper = mount(CardConvertTask, {
      props: {
        card: makeCard({
          status: 'processing',
          action: 'convert',
          target_format: 'pdf',
        }),
      },
      global: {
        plugins: [i18n],
      },
    })

    expect(runningWrapper.text()).toContain('Task task-1')
    expect(runningWrapper.text()).toContain('pdf')

    const completedWrapper = mount(CardConvertTask, {
      props: {
        card: makeCard({
          status: 'succeeded',
          action: 'convert',
          target_format: 'pdf',
          message: 'Completed',
        }),
      },
      global: {
        plugins: [i18n],
      },
    })

    expect(completedWrapper.text()).not.toContain('Task task-1')
    expect(completedWrapper.text()).toContain('pdf')
  })

  it('opens output location via location API before revealing in file manager', async () => {
    fetchMock.mockResolvedValueOnce(
      mockResponse({
        path: '/tmp/result.pdf',
        parent_path: '/tmp',
      })
    )

    const wrapper = mount(CardConvertTask, {
      props: {
        card: makeCard({
          status: 'succeeded',
          action: 'convert',
          target_format: 'pdf',
          message: 'Completed',
          outputs: [
            {
              output_id: 'out-1',
              name: 'result.pdf',
              preview_kind: 'pdf',
              download_url: '/api/v1/convert/tasks/task-1/download/out-1',
            },
          ],
        }),
      },
      global: {
        plugins: [i18n],
      },
    })

    const locationButton = wrapper
      .findAll('button')
      .find((button) => button.text().includes('Open location'))
    expect(locationButton).toBeTruthy()
    await locationButton!.trigger('click')
    await flushPromises()

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/convert/tasks/task-1/outputs/out-1/location',
      undefined
    )
    expect(revealInFileManagerMock).toHaveBeenCalledWith('/tmp/result.pdf')
    expect(wrapper.text()).toContain('Download PDF')
  })

  it('reveals output location directly from card output path when available', async () => {
    const wrapper = mount(CardConvertTask, {
      props: {
        card: makeCard({
          status: 'succeeded',
          action: 'convert',
          target_format: 'pdf',
          message: 'Completed',
          outputs: [
            {
              output_id: 'out-1',
              name: 'result.pdf',
              path: '/tmp/direct-result.pdf',
              preview_kind: 'pdf',
              download_url: '/api/v1/convert/tasks/task-1/download/out-1',
            },
          ],
        }),
      },
      global: {
        plugins: [i18n],
      },
    })

    expect(wrapper.text()).toContain('/tmp/direct-result.pdf')
    const locationButton = wrapper
      .findAll('button')
      .find((button) => button.text().includes('Open location'))
    expect(locationButton).toBeTruthy()
    await locationButton!.trigger('click')
    await flushPromises()

    expect(revealInFileManagerMock).toHaveBeenCalledWith('/tmp/direct-result.pdf')
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('falls back to output download when location reveal is unavailable', async () => {
    revealInFileManagerMock.mockResolvedValue(false)
    fetchMock.mockResolvedValueOnce(
      mockResponse({
        path: '/tmp/result.pdf',
        parent_path: '/tmp',
      })
    )
    fetchMock.mockResolvedValueOnce(mockBlobResponse('pdf-data', 'application/pdf'))
    const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})

    const wrapper = mount(CardConvertTask, {
      props: {
        card: makeCard({
          status: 'succeeded',
          action: 'convert',
          target_format: 'pdf',
          message: 'Completed',
          outputs: [
            {
              output_id: 'out-1',
              name: 'result.pdf',
              preview_kind: 'pdf',
              download_url: '/api/v1/convert/tasks/task-1/download/out-1',
            },
          ],
        }),
      },
      global: {
        plugins: [i18n],
      },
    })

    const locationButton = wrapper
      .findAll('button')
      .find((button) => button.text().includes('Open location'))
    expect(locationButton).toBeTruthy()
    await locationButton!.trigger('click')
    await flushPromises()

    expect(revealInFileManagerMock).toHaveBeenCalledWith('/tmp/result.pdf')
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      '/api/v1/convert/tasks/task-1/download/out-1',
      undefined
    )
    expect(clickSpy).toHaveBeenCalled()
  })
})
