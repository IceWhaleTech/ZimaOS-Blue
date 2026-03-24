import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('./client', () => ({
  default: {
    post: vi.fn(),
  },
}))

import api from './client'
import { startRemoteAccess } from './remote-access'

describe('startRemoteAccess', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    ;(api as any).post.mockResolvedValue({})
  })

  it('omits the port when none is provided', async () => {
    await startRemoteAccess('auto')

    expect((api as any).post).toHaveBeenCalledWith('/tunnel/start', {
      provider: 'auto',
    })
  })

  it('forwards an explicit target port when provided', async () => {
    await startRemoteAccess('ngrok', 19091, 'token', undefined, 'blue.ngrok-free.app')

    expect((api as any).post).toHaveBeenCalledWith('/tunnel/start', {
      provider: 'ngrok',
      port: 19091,
      ngrok_authtoken: 'token',
      ngrok_domain: 'blue.ngrok-free.app',
    })
  })
})
