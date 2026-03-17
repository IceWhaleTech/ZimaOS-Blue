import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'

const { revealPathMock, resolveLocalFileMock, isCurrentHostLoopbackMock, authFetchMock } =
  vi.hoisted(() => ({
  revealPathMock: vi.fn(),
  resolveLocalFileMock: vi.fn(),
  isCurrentHostLoopbackMock: vi.fn(),
   authFetchMock: vi.fn(),
 }))

vi.mock('@/api/system', () => ({
  systemApi: {
    revealPath: revealPathMock,
    resolveLocalFile: resolveLocalFileMock,
  },
}))

vi.mock('@/api/client', () => ({
  authFetch: authFetchMock,
}))

vi.mock('@/utils/localPath', async () => {
  const actual = await vi.importActual<typeof import('@/utils/localPath')>('@/utils/localPath')
  return {
    ...actual,
    isCurrentHostLoopback: isCurrentHostLoopbackMock,
  }
})

import { useTauri, refreshTauriDetection } from './useTauri'

declare global {
  interface Window {
    __TAURI__?: Record<string, unknown>
  }
}

describe('useTauri', () => {
  function mockBlobResponse(body: string, type: string, ok = true): Response {
    return {
      ok,
      blob: vi.fn().mockResolvedValue(new Blob([body], { type })),
    } as unknown as Response
  }

  beforeEach(() => {
    delete window.__TAURI_INTERNALS__
    delete window.__TAURI__
    delete window.__BLUE_DESKTOP__

    revealPathMock.mockReset()
    revealPathMock.mockResolvedValue({ success: true })
    resolveLocalFileMock.mockReset()
    resolveLocalFileMock.mockResolvedValue({
      data: {
        download_url: '/api/v1/system/local-file/content?path=%2Ftmp%2Freport.txt',
      },
    })
    authFetchMock.mockReset()
    authFetchMock.mockResolvedValue(mockBlobResponse('report-data', 'text/plain'))
    isCurrentHostLoopbackMock.mockReset()
    isCurrentHostLoopbackMock.mockReturnValue(true)
    ;(URL as unknown as { createObjectURL: (blob: Blob) => string }).createObjectURL = vi
      .fn()
      .mockReturnValue('blob:protected-resource')
    ;(URL as unknown as { revokeObjectURL: (url: string) => void }).revokeObjectURL = vi.fn()
  })

  afterEach(() => {
    delete window.__TAURI_INTERNALS__
    delete window.__TAURI__
    delete window.__BLUE_DESKTOP__
    vi.restoreAllMocks()
  })

  describe('isTauri detection', () => {
    it('should return false when not in Tauri environment', () => {
      refreshTauriDetection()
      const { isTauri } = useTauri()
      expect(isTauri.value).toBe(false)
    })

    it('should return true when __TAURI_INTERNALS__ is present (Tauri v2)', () => {
      window.__TAURI_INTERNALS__ = {
        invoke: vi.fn(),
      }

      refreshTauriDetection()
      const { isTauri } = useTauri()
      expect(isTauri.value).toBe(true)
    })

    it('should return true when __TAURI__ is present (Tauri v1)', () => {
      window.__TAURI__ = {
        invoke: vi.fn(),
      }

      refreshTauriDetection()
      const { isTauri } = useTauri()
      expect(isTauri.value).toBe(true)
    })
  })

  describe('singleton behavior', () => {
    it('should return the same ref across multiple calls', () => {
      refreshTauriDetection()
      const { isTauri: isTauri1 } = useTauri()
      const { isTauri: isTauri2 } = useTauri()
      expect(isTauri1).toBe(isTauri2)
    })
  })

  describe('openInBrowser', () => {
    it('should prefer reveal_path for local absolute paths in tauri desktop runtime', async () => {
      const invoke = vi.fn().mockResolvedValue(undefined)
      window.__TAURI_INTERNALS__ = { invoke }
      window.__BLUE_DESKTOP__ = true

      refreshTauriDetection()
      const { openInBrowser } = useTauri()
      const ok = await openInBrowser('/tmp/report.txt')

      expect(ok).toBe(true)
      expect(invoke).toHaveBeenCalledWith('reveal_path', { path: '/tmp/report.txt' })
      expect(invoke).not.toHaveBeenCalledWith('open_url', expect.anything())
      expect(resolveLocalFileMock).not.toHaveBeenCalled()
    })

    it('should fallback to local-file download URL when reveal fails', async () => {
      const invoke = vi.fn().mockImplementation((cmd: string) => {
        if (cmd === 'reveal_path') return Promise.reject(new Error('reveal failed'))
        if (cmd === 'open_url') return Promise.resolve(undefined)
        return Promise.reject(new Error(`unexpected command ${cmd}`))
      })
      const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null)
      window.__TAURI_INTERNALS__ = { invoke }
      window.__BLUE_DESKTOP__ = true
      isCurrentHostLoopbackMock.mockReturnValue(false)

      refreshTauriDetection()
      const { openInBrowser } = useTauri()
      const ok = await openInBrowser('/tmp/report.txt')

      expect(ok).toBe(true)
      expect(invoke).toHaveBeenCalledWith('reveal_path', { path: '/tmp/report.txt' })
      expect(resolveLocalFileMock).toHaveBeenCalledWith('/tmp/report.txt')
      expect(authFetchMock).toHaveBeenCalledWith(
        '/api/v1/system/local-file/content?path=%2Ftmp%2Freport.txt',
      )
      expect(openSpy).toHaveBeenCalledWith('blob:protected-resource', '_blank')
      expect(invoke).not.toHaveBeenCalledWith('open_url', { url: '/tmp/report.txt' })
    })

    it('should fallback to open_url when local resolve fails in tauri runtime', async () => {
      const invoke = vi.fn().mockImplementation((cmd: string) => {
        if (cmd === 'reveal_path') return Promise.reject(new Error('reveal failed'))
        if (cmd === 'open_url') return Promise.resolve(undefined)
        return Promise.reject(new Error(`unexpected command ${cmd}`))
      })
      resolveLocalFileMock.mockRejectedValueOnce(new Error('resolve failed'))
      window.__TAURI_INTERNALS__ = { invoke }
      window.__BLUE_DESKTOP__ = true
      isCurrentHostLoopbackMock.mockReturnValue(false)

      refreshTauriDetection()
      const { openInBrowser } = useTauri()
      const ok = await openInBrowser('/tmp/report.txt')

      expect(ok).toBe(true)
      expect(invoke).toHaveBeenCalledWith('open_url', { url: '/tmp/report.txt' })
    })

    it('should use reveal-path API fallback in desktop runtime when invoke fails on loopback host', async () => {
      const invoke = vi.fn().mockRejectedValue(new Error('reveal failed'))
      window.__TAURI_INTERNALS__ = { invoke }
      window.__BLUE_DESKTOP__ = true
      isCurrentHostLoopbackMock.mockReturnValue(true)

      refreshTauriDetection()
      const { openInBrowser } = useTauri()
      const ok = await openInBrowser('/tmp/report.txt')

      expect(ok).toBe(true)
      expect(revealPathMock).toHaveBeenCalledWith('/tmp/report.txt')
      expect(resolveLocalFileMock).not.toHaveBeenCalled()
    })

    it('should return false for local paths in non-tauri runtime when reveal and resolve both fail', async () => {
      isCurrentHostLoopbackMock.mockReturnValue(false)
      resolveLocalFileMock.mockRejectedValueOnce(new Error('resolve failed'))
      const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null)

      refreshTauriDetection()
      const { openInBrowser } = useTauri()
      const ok = await openInBrowser('/tmp/report.txt')

      expect(ok).toBe(false)
      expect(openSpy).not.toHaveBeenCalled()
    })

    it('should use reveal-path API for local paths on non-tauri loopback host', async () => {
      const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null)
      isCurrentHostLoopbackMock.mockReturnValue(true)

      refreshTauriDetection()
      const { openInBrowser } = useTauri()
      const ok = await openInBrowser('/tmp/report.txt')

      expect(ok).toBe(true)
      expect(revealPathMock).toHaveBeenCalledWith('/tmp/report.txt')
      expect(openSpy).not.toHaveBeenCalled()
      expect(resolveLocalFileMock).not.toHaveBeenCalled()
    })

    it('should use local-file download fallback on non-loopback web hosts', async () => {
      const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null)
      isCurrentHostLoopbackMock.mockReturnValue(false)

      refreshTauriDetection()
      const { openInBrowser } = useTauri()
      const ok = await openInBrowser('/tmp/report.txt')

      expect(ok).toBe(true)
      expect(revealPathMock).not.toHaveBeenCalled()
      expect(resolveLocalFileMock).toHaveBeenCalledWith('/tmp/report.txt')
      expect(authFetchMock).toHaveBeenCalledWith(
        '/api/v1/system/local-file/content?path=%2Ftmp%2Freport.txt',
      )
      expect(openSpy).toHaveBeenCalledWith('blob:protected-resource', '_blank')
    })
  })

  describe('revealInFileManager', () => {
    it('should invoke reveal_path in tauri desktop runtime', async () => {
      const invoke = vi.fn().mockResolvedValue(undefined)
      window.__TAURI_INTERNALS__ = { invoke }
      window.__BLUE_DESKTOP__ = true

      refreshTauriDetection()
      const { revealInFileManager } = useTauri()
      const ok = await revealInFileManager('/tmp/report.txt')

      expect(ok).toBe(true)
      expect(invoke).toHaveBeenCalledWith('reveal_path', { path: '/tmp/report.txt' })
    })

    it('should fallback to reveal-path API in desktop runtime when Tauri IPC is unavailable', async () => {
      window.__BLUE_DESKTOP__ = true
      isCurrentHostLoopbackMock.mockReturnValue(true)

      refreshTauriDetection()
      const { revealInFileManager } = useTauri()
      const ok = await revealInFileManager('/tmp/report.txt')

      expect(ok).toBe(true)
      expect(revealPathMock).toHaveBeenCalledWith('/tmp/report.txt')
    })

    it('should use reveal-path API on non-tauri loopback hosts', async () => {
      isCurrentHostLoopbackMock.mockReturnValue(true)
      refreshTauriDetection()
      const { revealInFileManager } = useTauri()
      const ok = await revealInFileManager('/tmp/report.txt')

      expect(ok).toBe(true)
      expect(revealPathMock).toHaveBeenCalledWith('/tmp/report.txt')
    })

    it('should return false on non-loopback hosts outside tauri', async () => {
      isCurrentHostLoopbackMock.mockReturnValue(false)
      refreshTauriDetection()
      const { revealInFileManager } = useTauri()
      const ok = await revealInFileManager('/tmp/report.txt')

      expect(ok).toBe(false)
      expect(revealPathMock).not.toHaveBeenCalled()
    })

    it('should fallback to reveal-path API when invoke fails in desktop runtime', async () => {
      const invoke = vi.fn().mockRejectedValue(new Error('invoke failed'))
      window.__TAURI_INTERNALS__ = { invoke }
      window.__BLUE_DESKTOP__ = true
      isCurrentHostLoopbackMock.mockReturnValue(true)

      refreshTauriDetection()
      const { revealInFileManager } = useTauri()
      const ok = await revealInFileManager('/tmp/report.txt')

      expect(ok).toBe(true)
      expect(revealPathMock).toHaveBeenCalledWith('/tmp/report.txt')
    })

    it('should return false when invoke fails and loopback fallback is unavailable', async () => {
      const invoke = vi.fn().mockRejectedValue(new Error('invoke failed'))
      window.__TAURI_INTERNALS__ = { invoke }
      window.__BLUE_DESKTOP__ = true
      isCurrentHostLoopbackMock.mockReturnValue(false)

      refreshTauriDetection()
      const { revealInFileManager } = useTauri()
      const ok = await revealInFileManager('/tmp/report.txt')

      expect(ok).toBe(false)
      expect(revealPathMock).not.toHaveBeenCalled()
    })
  })
})
