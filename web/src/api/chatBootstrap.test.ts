import { beforeEach, describe, expect, it, vi } from 'vitest'
import { chatBootstrapApi } from './chatBootstrap'

const mocks = vi.hoisted(() => ({
  apiGet: vi.fn(),
}))

vi.mock('./client', () => ({
  default: {
    get: (...args: unknown[]) => mocks.apiGet(...args),
  },
}))

describe('chatBootstrapApi', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('dedupes concurrent bootstrap requests for the same conversation', async () => {
    let resolveRequest!: (value: { data: unknown }) => void
    const pending = new Promise<{ data: unknown }>((resolve) => {
      resolveRequest = resolve
    })
    mocks.apiGet.mockReturnValue(pending)

    const [firstPromise, secondPromise] = [
      chatBootstrapApi.getConversationBootstrap('conv-1'),
      chatBootstrapApi.getConversationBootstrap('conv-1'),
    ]

    expect(mocks.apiGet).toHaveBeenCalledTimes(1)
    expect(mocks.apiGet).toHaveBeenCalledWith('/conversations/conv-1/bootstrap')

    resolveRequest({
      data: {
        command_state: {
          conversation_id: 'conv-1',
          offline: false,
        },
        active_stream: {
          conversation_id: 'conv-1',
          active: false,
        },
        current_tasks: [],
        background_tasks: [],
      },
    })

    const [firstResponse, secondResponse] = await Promise.all([firstPromise, secondPromise])
    expect(firstResponse).toEqual(secondResponse)
  })

  it('returns an empty sparse payload for blank conversation ids without hitting the network', async () => {
    const response = await chatBootstrapApi.getConversationBootstrap('   ')

    expect(mocks.apiGet).not.toHaveBeenCalled()
    expect(response.data).toEqual({
      command_state: {
        conversation_id: '',
        offline: false,
      },
      active_stream: {
        conversation_id: '',
        active: false,
      },
      current_tasks: [],
      background_tasks: [],
      pending_approval: null,
      pending_question: null,
      pending_exec_approval: null,
    })
  })
})
