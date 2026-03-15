import type { PasswordPolicy } from '@/api/auth'

export interface PasswordCheck {
  key: 'length' | 'uppercase' | 'lowercase' | 'letter' | 'number' | 'special'
  passed: boolean
  label: string
}

type Translate = (key: string, params?: Record<string, unknown>) => string

export function buildPasswordChecks(
  password: string,
  policy: PasswordPolicy,
  t: Translate
): PasswordCheck[] {
  return [
    {
      key: 'length',
      passed: password.length >= policy.min_length,
      label: t('preview.passwordCheck.length', { n: policy.min_length }),
    },
    ...(policy.require_uppercase
      ? [
          {
            key: 'uppercase' as const,
            passed: /\p{Lu}/u.test(password),
            label: t('preview.passwordCheck.uppercase'),
          },
        ]
      : []),
    ...(policy.require_lowercase
      ? [
          {
            key: 'lowercase' as const,
            passed: /\p{Ll}/u.test(password),
            label: t('preview.passwordCheck.lowercase'),
          },
        ]
      : []),
    ...(policy.require_letter
      ? [
          {
            key: 'letter' as const,
            passed: /\p{L}/u.test(password),
            label: t('preview.passwordCheck.letter'),
          },
        ]
      : []),
    ...(policy.require_number
      ? [
          {
            key: 'number' as const,
            passed: /[0-9]/.test(password),
            label: t('preview.passwordCheck.number'),
          },
        ]
      : []),
    ...(policy.require_special
      ? [
          {
            key: 'special' as const,
            passed: /[^\p{L}\p{N}\s]/u.test(password),
            label: t('preview.passwordCheck.special'),
          },
        ]
      : []),
  ]
}
