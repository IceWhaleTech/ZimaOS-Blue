import { describe, it, expect } from 'vitest'
import { stripFirstLineHeading } from '@/utils/chat-message-text'

describe('chat-message-text', () => {
  describe('stripFirstLineHeading', () => {
    it('returns empty string for heading-only content', () => {
      expect(stripFirstLineHeading('# Title')).toBe('')
    })

    it('strips a heading with leading whitespace on first line', () => {
      const input = ' \t  ## 标题\n\n  Body content'
      expect(stripFirstLineHeading(input)).toBe('Body content')
    })

    it('keeps content unchanged when first line is not a heading', () => {
      const input = 'Plain first line\n# second line heading-like text'
      expect(stripFirstLineHeading(input)).toBe(input)
    })

    it('preserves non-heading lines after removing first heading line', () => {
      const input = '# Heading\nline1\nline2'
      expect(stripFirstLineHeading(input)).toBe('line1\nline2')
    })
  })
})
