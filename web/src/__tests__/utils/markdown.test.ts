import { describe, it, expect } from 'vitest'
import { renderMarkdown, copyCodeToClipboard } from '@/utils/markdown'

describe('Markdown Renderer', () => {
  describe('renderMarkdown', () => {
    it('should render plain text', () => {
      const result = renderMarkdown('Hello world')
      expect(result).toContain('Hello world')
      expect(result).toContain('<p')
    })

    it('should render headers', () => {
      expect(renderMarkdown('# Header 1')).toContain('<h1')
      expect(renderMarkdown('## Header 2')).toContain('<h2')
      expect(renderMarkdown('### Header 3')).toContain('<h3')
    })

    it('should render bold text', () => {
      const result = renderMarkdown('**bold text**')
      expect(result).toContain('<strong>bold text</strong>')
    })

    it('should render italic text', () => {
      const result = renderMarkdown('*italic text*')
      expect(result).toContain('<em>italic text</em>')
    })

    it('should render inline code', () => {
      const result = renderMarkdown('Use `console.log()` for debugging')
      expect(result).toContain('<code class="inline-code">console.log()</code>')
    })

    it('should render code blocks', () => {
      const markdown = '```javascript\nconst x = 1;\n```'
      const result = renderMarkdown(markdown)
      expect(result).toContain('code-block')
      expect(result).toContain('const x = 1;')
    })

    it('should render links', () => {
      const result = renderMarkdown('[Google](https://google.com)')
      expect(result).toContain('href="https://google.com"')
      expect(result).toContain('Google')
    })

    it('should render unordered lists', () => {
      const markdown = '- Item 1\n- Item 2\n- Item 3'
      const result = renderMarkdown(markdown)
      expect(result).toContain('<ul')
      expect(result).toContain('<li>Item 1</li>')
      expect(result).toContain('<li>Item 2</li>')
      expect(result).toContain('<li>Item 3</li>')
    })

    it('should render blockquotes', () => {
      const result = renderMarkdown('> This is a quote')
      expect(result).toContain('<blockquote')
      expect(result).toContain('This is a quote')
    })

    it('should render horizontal rules', () => {
      const result = renderMarkdown('---')
      expect(result).toContain('<hr')
    })

    it('should escape HTML to prevent XSS', () => {
      const result = renderMarkdown('<script>alert("xss")</script>')
      expect(result).not.toContain('<script>')
      expect(result).toContain('&lt;script&gt;')
    })

    it('should handle strikethrough', () => {
      const result = renderMarkdown('~~deleted~~')
      expect(result).toContain('<del>deleted</del>')
    })

    it('should detect language aliases', () => {
      const markdown = '```js\nconst x = 1;\n```'
      const result = renderMarkdown(markdown)
      expect(result).toContain('javascript')
    })
  })

  describe('copyCodeToClipboard', () => {
    it('should call clipboard API', async () => {
      const mockWriteText = vi.fn().mockResolvedValue(undefined)
      vi.stubGlobal('navigator', {
        clipboard: {
          writeText: mockWriteText,
        },
      })

      await copyCodeToClipboard('test code')
      expect(mockWriteText).toHaveBeenCalledWith('test code')

      vi.unstubAllGlobals()
    })
  })
})
