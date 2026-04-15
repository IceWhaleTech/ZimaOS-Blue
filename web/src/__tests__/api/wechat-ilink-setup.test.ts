import { beforeEach, describe, expect, it, vi } from 'vitest'

const postMock = vi.fn()
const getMock = vi.fn()

vi.mock('@/api/client', () => ({
  default: {
    post: postMock,
    get: getMock,
  },
}))

describe('api/wechat-ilink-setup', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('uses the channel API base path for setup session requests', async () => {
    const api = await import('@/api/wechat-ilink-setup')

    await api.createWeChatILinkSetupSession()
    await api.getWeChatILinkSetupSession('session-1')
    await api.completeWeChatILinkSetupSession('session-1', { bot_token: 'token' })

    expect(postMock).toHaveBeenNthCalledWith(
      1,
      '/channels/wechat_ilink/setup/session',
      undefined,
      expect.objectContaining({ baseURL: '/api' })
    )
    expect(getMock).toHaveBeenCalledWith(
      '/channels/wechat_ilink/setup/session/session-1',
      expect.objectContaining({ baseURL: '/api' })
    )
    expect(postMock).toHaveBeenNthCalledWith(
      2,
      '/channels/wechat_ilink/setup/session/session-1/complete',
      { pairing_payload: { bot_token: 'token' } },
      expect.objectContaining({ baseURL: '/api' })
    )
  })
})
