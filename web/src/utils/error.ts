import type { AxiosError } from 'axios'
import { i18n } from '@/i18n'

const HTTP_STATUS_KEY_MAP: Record<number, string> = {
  400: 'errors.http400',
  401: 'errors.http401',
  403: 'errors.http403',
  404: 'errors.http404',
  408: 'errors.http408',
  409: 'errors.http409',
  413: 'errors.http413',
  422: 'errors.http422',
  429: 'errors.http429',
  500: 'errors.http500',
  502: 'errors.http502',
  503: 'errors.http503',
  504: 'errors.http504',
}

/**
 * Get translated error message from HTTP status code
 */
export function getHttpErrorMessage(status: number): string {
  const t = i18n.global.t
  const key = HTTP_STATUS_KEY_MAP[status]

  // Check if we have a specific translation for this status code
  if (key && i18n.global.te(key)) {
    return t(key)
  }

  // Fall back to generic message
  return t('errors.requestFailed', { code: status })
}

/**
 * Get translated error message from Axios error
 */
export function getErrorMessage(error: unknown): string {
  const t = i18n.global.t

  if (!error) {
    return t('errors.unknownError')
  }

  // Handle Axios errors
  if (isAxiosError(error)) {
    const axiosError = error as AxiosError

    // Network error (no response)
    if (!axiosError.response) {
      if (axiosError.code === 'ECONNABORTED') {
        return t('errors.timeout')
      }
      return t('errors.connectionFailed')
    }

    // Server responded with error status
    const status = axiosError.response.status

    // Try to get error message from response body
    const responseData = axiosError.response.data as { error?: string; message?: string } | undefined
    if (responseData?.error) {
      return responseData.error
    }
    if (responseData?.message) {
      return responseData.message
    }

    // Fall back to translated HTTP status message
    return getHttpErrorMessage(status)
  }

  // Handle standard Error objects
  if (error instanceof Error) {
    return error.message
  }

  // Handle string errors
  if (typeof error === 'string') {
    return error
  }

  return t('errors.unknownError')
}

/**
 * Type guard for Axios errors
 */
function isAxiosError(error: unknown): error is AxiosError {
  return (
    typeof error === 'object' &&
    error !== null &&
    'isAxiosError' in error &&
    (error as AxiosError).isAxiosError === true
  )
}

/**
 * Format error for display (with optional retry suggestion)
 */
export function formatErrorForDisplay(error: unknown, showRetry = true): string {
  const t = i18n.global.t
  const message = getErrorMessage(error)

  if (showRetry) {
    return `${message}. ${t('errors.pleaseRetry')}`
  }

  return message
}
