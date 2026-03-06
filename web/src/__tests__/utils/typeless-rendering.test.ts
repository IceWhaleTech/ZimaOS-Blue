import { describe, it, expect } from 'vitest'
import { renderCardToHtml } from '@/utils/typelessRenderers'
import type { TypelessCardCode, TypelessCardInfo } from '@/types/typeless'

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

  it('should render expand toggle for long code blocks', () => {
    const lines = Array.from({ length: 30 }, (_, index) => `line ${index + 1}`).join('\n')
    const card: TypelessCardCode = {
      type: 'code',
      id: 'test-5',
      code: lines,
      language: 'javascript',
      showLineNumbers: true,
    }

    const html = renderCardToHtml(card)

    expect(html).toContain('Show more 6 lines')
    expect(html).toContain('data-code-container-id=')
    expect(html).toContain('data-collapsed="true"')
  })
})

describe('Typeless Inline Parsing', () => {
  it('should keep identifier underscores literal in info cards', () => {
    const card: TypelessCardInfo = {
      type: 'info',
      id: 'info-1',
      content: 'task_id and message_id',
    }

    const html = renderCardToHtml(card)

    expect(html).toContain('task_id and message_id')
    expect(html).not.toContain('<em>id and message</em>')
  })

  it('should keep underscore markers literal but still support asterisk emphasis', () => {
    const card: TypelessCardInfo = {
      type: 'info',
      id: 'info-2',
      content: '*done* with _raw_token_',
    }

    const html = renderCardToHtml(card)

    expect(html).toContain('<em>done</em>')
    expect(html).toContain('_raw_token_')
    expect(html).not.toContain('<em>raw_token</em>')
  })
})
