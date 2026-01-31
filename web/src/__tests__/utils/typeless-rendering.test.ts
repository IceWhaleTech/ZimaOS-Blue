import { describe, it, expect } from 'vitest'
import { renderCardToHtml } from '@/utils/typelessRenderers'
import type { TypelessCardCode } from '@/types/typeless'

describe('Typeless Code Card Rendering', () => {
  it('should render code card without language as "Text"', () => {
    const card: TypelessCardCode = {
      type: 'code',
      id: 'test-1',
      code: '人工智能（AI）\n├── 传统方法',
      language: undefined,
      showLineNumbers: true,
    }

    const html = renderCardToHtml(card)

    // Should show "Text" badge
    expect(html).toContain('Text')
    // Should not show line numbers for plain text
    expect(html).not.toContain('inline-block w-8 text-right')
  })

  it('should render code card with language and show line numbers', () => {
    const card: TypelessCardCode = {
      type: 'code',
      id: 'test-2',
      code: 'console.log("hello")',
      language: 'javascript',
      showLineNumbers: true,
    }

    const html = renderCardToHtml(card)

    // Should show "JavaScript" badge
    expect(html).toContain('JavaScript')
    // Should show line numbers
    expect(html).toContain('inline-block w-8 text-right')
  })

  it('should render code card with markdown language', () => {
    const card: TypelessCardCode = {
      type: 'code',
      id: 'test-3',
      code: '# Title',
      language: 'markdown',
      showLineNumbers: true,
    }

    const html = renderCardToHtml(card)

    // Should show "Markdown" badge
    expect(html).toContain('Markdown')
    // Should show line numbers
    expect(html).toContain('inline-block w-8 text-right')
  })

  it('should not show line numbers for plain text', () => {
    const card: TypelessCardCode = {
      type: 'code',
      id: 'test-4',
      code: 'line 1\nline 2\nline 3',
      language: undefined,
      showLineNumbers: true, // Even if requested, should be disabled for plain text
    }

    const html = renderCardToHtml(card)

    // Should not contain line number spans
    expect(html).not.toContain('inline-block w-8 text-right')
    // Should contain the actual content
    expect(html).toContain('line 1')
    expect(html).toContain('line 2')
  })
})
