export const DEFAULT_DESKTOP_STARTUP_PREVIEW_CHECK_TIMEOUT_MS = 250
export const STORED_SESSION_DESKTOP_STARTUP_PREVIEW_CHECK_TIMEOUT_MS = 0

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
