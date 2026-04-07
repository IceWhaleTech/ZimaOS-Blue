import { describe, expect, it } from 'vitest'

import {
  DEFAULT_DESKTOP_STARTUP_PREVIEW_CHECK_TIMEOUT_MS,
  getOptimisticStartupPreviewCheckTimeout,
  STORED_SESSION_DESKTOP_STARTUP_PREVIEW_CHECK_TIMEOUT_MS,
  shouldDeferAskUserQuestionDialogOnDesktopStartup,
  shouldDeferBrowserMonitorOnDesktopStartup,
  shouldDeferLocaleEnhancementsOnDesktopStartup,
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

  it('defers locale enhancements for desktop chat startup routes', () => {
    expect(shouldDeferLocaleEnhancementsOnDesktopStartup(true, true, '/')).toBe(true)
    expect(shouldDeferLocaleEnhancementsOnDesktopStartup(true, false, '/')).toBe(true)
    expect(shouldDeferLocaleEnhancementsOnDesktopStartup(true, true, '/chat')).toBe(true)
  })

  it('keeps locale enhancements eager outside the desktop hot-start path', () => {
    expect(shouldDeferLocaleEnhancementsOnDesktopStartup(true, true, '/login')).toBe(false)
    expect(shouldDeferLocaleEnhancementsOnDesktopStartup(false, true, '/chat')).toBe(false)
  })

  it('defers the browser monitor only for desktop chat startup routes', () => {
    expect(shouldDeferBrowserMonitorOnDesktopStartup(true, '/')).toBe(true)
    expect(shouldDeferBrowserMonitorOnDesktopStartup(true, '/chat')).toBe(true)
    expect(shouldDeferBrowserMonitorOnDesktopStartup(true, '/profile')).toBe(false)
    expect(shouldDeferBrowserMonitorOnDesktopStartup(false, '/chat')).toBe(false)
  })

  it('defers the ask-user-question dialog only for desktop chat startup routes', () => {
    expect(shouldDeferAskUserQuestionDialogOnDesktopStartup(true, '/')).toBe(true)
    expect(shouldDeferAskUserQuestionDialogOnDesktopStartup(true, '/chat')).toBe(true)
    expect(shouldDeferAskUserQuestionDialogOnDesktopStartup(true, '/profile')).toBe(false)
    expect(shouldDeferAskUserQuestionDialogOnDesktopStartup(false, '/chat')).toBe(false)
  })
})
