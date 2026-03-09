import { describe, expect, it } from 'vitest'

import { deepMergeMessages } from './merge'

describe('deepMergeMessages', () => {
  it('preserves nested base keys when locale overrides only part of an object', () => {
    const base = {
      auth: {
        signIn: 'Sign in',
        confirmPassword: 'Confirm Password',
      },
      skills: {
        detail: {
          sections: {
            documentation: 'Documentation',
            frontmatter: 'Frontmatter',
          },
        },
      },
    }

    const overrides = {
      auth: {
        signIn: '登录',
      },
      skills: {
        detail: {
          sections: {
            documentation: '文档',
          },
        },
      },
    }

    expect(deepMergeMessages(base, overrides)).toEqual({
      auth: {
        signIn: '登录',
        confirmPassword: 'Confirm Password',
      },
      skills: {
        detail: {
          sections: {
            documentation: '文档',
            frontmatter: 'Frontmatter',
          },
        },
      },
    })
  })

  it('does not mutate the base messages object', () => {
    const base = {
      common: {
        save: 'Save',
        cancel: 'Cancel',
      },
    }

    const overrides = {
      common: {
        save: '保存',
      },
    }

    const merged = deepMergeMessages(base, overrides)

    expect(merged).toEqual({
      common: {
        save: '保存',
        cancel: 'Cancel',
      },
    })
    expect(base).toEqual({
      common: {
        save: 'Save',
        cancel: 'Cancel',
      },
    })
  })
})
