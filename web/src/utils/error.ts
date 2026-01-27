import type { AxiosError } from 'axios'
import { i18n } from '@/i18n'

/**
 * Get translated error message from HTTP status code
 */
export function getHttpErrorMessage(status: number): string {
  const t = i18n.global.t
  const key = `errors.http${status}`

  // Check if we have a specific translation for this status code
  if (i18n.global.te(key)) {
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
