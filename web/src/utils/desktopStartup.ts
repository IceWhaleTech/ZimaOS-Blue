export const DEFAULT_DESKTOP_STARTUP_PREVIEW_CHECK_TIMEOUT_MS = 250
export const STORED_SESSION_DESKTOP_STARTUP_PREVIEW_CHECK_TIMEOUT_MS = 0

function isDesktopChatStartupPath(isDesktop: boolean, path: string): boolean {
  if (!isDesktop) return false
  return path === '/' || path === '/chat'
}

export function getOptimisticStartupPreviewCheckTimeout(hasStoredSessionHint: boolean): number {
  return hasStoredSessionHint
    ? STORED_SESSION_DESKTOP_STARTUP_PREVIEW_CHECK_TIMEOUT_MS
    : DEFAULT_DESKTOP_STARTUP_PREVIEW_CHECK_TIMEOUT_MS
}

export function shouldPrefetchPreviewModeOnRouterInit(
  isDesktop: boolean,
  hasStoredSessionHint: boolean
): boolean {
  if (!isDesktop) return true
  return !hasStoredSessionHint
}

export function shouldDeferLocaleEnhancementsOnDesktopStartup(
  isDesktop: boolean,
  _hasStoredSessionHint: boolean,
  path: string
): boolean {
  return isDesktopChatStartupPath(isDesktop, path)
}

export function shouldDeferBrowserMonitorOnDesktopStartup(
  isDesktop: boolean,
  path: string
): boolean {
  return isDesktopChatStartupPath(isDesktop, path)
}

export function shouldDeferAskUserQuestionDialogOnDesktopStartup(
  isDesktop: boolean,
  path: string
): boolean {
  return isDesktopChatStartupPath(isDesktop, path)
}
