import { describe, expect, it } from 'vitest'

import {
  DEFAULT_DESKTOP_STARTUP_PREVIEW_CHECK_TIMEOUT_MS,
  getOptimisticStartupPreviewCheckTimeout,
  STORED_SESSION_DESKTOP_STARTUP_PREVIEW_CHECK_TIMEOUT_MS,
  shouldPrefetchPreviewModeOnRouterInit,
} from '@/utils/desktopStartup'

describe('desktopStartup', () => {
  it('keeps the longer optimistic preview check window when there is no stored session hint', () => {
    expect(getOptimisticStartupPreviewCheckTimeout(false)).toBe(
      DEFAULT_DESKTOP_STARTUP_PREVIEW_CHECK_TIMEOUT_MS
    )
  })

  it('skips the optimistic preview wait when desktop startup already has a stored session hint', () => {
    expect(getOptimisticStartupPreviewCheckTimeout(true)).toBe(
      STORED_SESSION_DESKTOP_STARTUP_PREVIEW_CHECK_TIMEOUT_MS
    )
    expect(STORED_SESSION_DESKTOP_STARTUP_PREVIEW_CHECK_TIMEOUT_MS).toBe(0)
  })

  it('skips router-init preview prefetch for desktop startup with a stored session hint', () => {
    expect(shouldPrefetchPreviewModeOnRouterInit(true, true)).toBe(false)
  })

  it('keeps router-init preview prefetch for first-run desktop startup and non-desktop runs', () => {
    expect(shouldPrefetchPreviewModeOnRouterInit(true, false)).toBe(true)
    expect(shouldPrefetchPreviewModeOnRouterInit(false, true)).toBe(true)
  })
})
