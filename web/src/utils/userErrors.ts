import { i18n } from '@/i18n'

const USER_ERROR_KEY_MAP: Record<string, string> = {
  'password does not meet requirements': 'users.error.passwordNotMeetRequirements',
  'username already exists': 'users.error.usernameExists',
  'email already exists': 'users.error.emailExists',
}

export function getUserErrorMessage(message: string | null | undefined, fallbackKey: string): string {
  if (message) {
    const key = USER_ERROR_KEY_MAP[message]
    if (key) {
      return String(i18n.global.t(key))
    }
    return message
  }

  return String(i18n.global.t(fallbackKey))
}
