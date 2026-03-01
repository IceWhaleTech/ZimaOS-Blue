import { describe, it, expect } from 'vitest'
import { renderMarkdown, renderMarkdownCached, copyCodeToClipboard, markdownToText } from '@/utils/markdown'

describe('Markdown Renderer', () => {
  describe('renderMarkdown', () => {
    it('should render plain text', () => {
      const result = renderMarkdown('Hello world')
      expect(result).toContain('Hello world')
      expect(result).toContain('<p')
    })

    it('should use plain rendering for multiline text without markdown syntax', () => {
      const result = renderMarkdown('line one\nline two\n\nline three')
      expect(result).toContain('<p class="my-1">line one</p>')
      expect(result).toContain('<p class="my-1">line two</p>')
      expect(result).toContain('<p class="my-1">line three</p>')
      expect(result).toContain('<br>')
      expect(result).not.toContain('<ul')
      expect(result).not.toContain('<blockquote')
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

    it('should keep ordered lists on multiline input', () => {
      const result = renderMarkdown('1. first\n2. second')
      expect(result).toContain('<ul')
      expect(result).toContain('<li>first</li>')
      expect(result).toContain('<li>second</li>')
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

    describe('tables', () => {
      it('should render a basic table with header', () => {
        const markdown = `| Header 1 | Header 2 |
|----------|----------|
| Cell 1   | Cell 2   |
| Cell 3   | Cell 4   |`
        const result = renderMarkdown(markdown)
        expect(result).toContain('<table')
        expect(result).toContain('<thead')
        expect(result).toContain('<tbody')
        expect(result).toContain('<th')
        expect(result).toContain('<td')
        expect(result).toContain('Header 1')
        expect(result).toContain('Header 2')
        expect(result).toContain('Cell 1')
        expect(result).toContain('Cell 2')
      })

      it('should render a table without leading/trailing pipes', () => {
        const markdown = `Header 1 | Header 2
---------|----------
Cell 1   | Cell 2`
        const result = renderMarkdown(markdown)
        expect(result).toContain('<table')
        expect(result).toContain('Header 1')
        expect(result).toContain('Cell 1')
      })

      it('should render Chinese content in tables', () => {
        const markdown = `| 特性 | 说明 |
|------|------|
| 叠加 | 量子比特可同时处于多个状态 |
| 纠缠 | 多个量子比特之间存在关联 |`
        const result = renderMarkdown(markdown)
        expect(result).toContain('<table')
        expect(result).toContain('特性')
        expect(result).toContain('说明')
        expect(result).toContain('叠加')
        expect(result).toContain('量子比特可同时处于多个状态')
      })

      it('should apply proper styling classes to tables', () => {
        const markdown = `| A | B |
|---|---|
| 1 | 2 |`
        const result = renderMarkdown(markdown)
        expect(result).toContain('overflow-x-auto')
        expect(result).toContain('border-collapse')
        expect(result).toContain('border-gray-300')
        expect(result).toContain('dark:border-gray-600')
      })

      it('should render inline markdown within table cells', () => {
        const markdown = `| Feature | Description |
|---------|-------------|
| **Bold** | Use \`code\` here |`
        const result = renderMarkdown(markdown)
        expect(result).toContain('<strong>Bold</strong>')
        expect(result).toContain('<code class="inline-code">code</code>')
      })

      it('should handle table with alignment separators', () => {
        const markdown = `| Left | Center | Right |
|:-----|:------:|------:|
| L    | C      | R     |`
        const result = renderMarkdown(markdown)
        expect(result).toContain('<table')
        expect(result).toContain('Left')
        expect(result).toContain('Center')
        expect(result).toContain('Right')
      })

      it('should flush table before other block elements', () => {
        const markdown = `| A | B |
|---|---|
| 1 | 2 |

# Header after table`
        const result = renderMarkdown(markdown)
        expect(result).toContain('</table>')
        expect(result).toContain('<h1')
        // Table should be closed before header
        const tableEnd = result.indexOf('</table>')
        const headerStart = result.indexOf('<h1')
        expect(tableEnd).toBeLessThan(headerStart)
      })

      it('should handle table followed by code block', () => {
        const markdown = `| A | B |
|---|---|
| 1 | 2 |

\`\`\`js
const x = 1;
\`\`\``
        const result = renderMarkdown(markdown)
        expect(result).toContain('</table>')
        expect(result).toContain('code-block')
      })

      it('should NOT render tree structures as tables', () => {
        const markdown = `人工智能（AI）
├── 传统方法（规则、搜索等）
├── 机器学习（ML）
│   ├── 浅层学习
│   └── 深度学习（DL）← 最先进的方向
└── 其他方法`
        const result = renderMarkdown(markdown)
        // Should NOT be rendered as a table
        expect(result).not.toContain('<table')
        expect(result).not.toContain('<th')
        expect(result).not.toContain('<td')
        // Should preserve the tree structure as paragraphs
        expect(result).toContain('├──')
        expect(result).toContain('└──')
        expect(result).toContain('│')
      })

      it('should NOT render lines with box-drawing characters as tables', () => {
        const markdown = `目录结构：
├── src/
│   ├── components/
│   └── utils/
└── package.json`
        const result = renderMarkdown(markdown)
        expect(result).not.toContain('<table')
        expect(result).toContain('├──')
        expect(result).toContain('└──')
      })

      it('should preserve tree structure formatting with preformatted block', () => {
        const markdown = `人工智能（AI）
├── 传统方法（规则、搜索等）
├── 机器学习（ML）
│   ├── 浅层学习
│   └── 深度学习（DL）← 最先进的方向
└── 其他方法`
        const result = renderMarkdown(markdown)
        // Tree lines should be in a pre block
        expect(result).toContain('<pre class="tree-structure')
        expect(result).toContain('whitespace-pre')
        // Should preserve the structure
        expect(result).toContain('├──')
        expect(result).toContain('└──')
        expect(result).toContain('│')
        // First line without tree chars should be a paragraph
        expect(result).toContain('<p class="my-1">人工智能（AI）</p>')
      })
    })
  })

  describe('renderMarkdownCached', () => {
    it('should keep append-only plain text output identical to renderMarkdown', () => {
      const scope = 'markdown-cached-append-plain'
      const steps = [
        'Hello',
        'Hello world',
        'Hello world and friends',
      ]

      for (const step of steps) {
        expect(renderMarkdownCached(step, scope)).toBe(renderMarkdown(step))
      }
    })

    it('should keep append-only multiline plain text output identical to renderMarkdown', () => {
      const scope = 'markdown-cached-append-multiline'
      const steps = [
        'line one\nline two',
        'line one\nline two more',
        'line one\nline two more\nline three',
        'line one\nline two more\nline three\nline four',
      ]

      for (const step of steps) {
        expect(renderMarkdownCached(step, scope)).toBe(renderMarkdown(step))
      }
    })

    it('should correctly fall back when appended text introduces markdown syntax', () => {
      const scope = 'markdown-cached-fastpath-transition'
      const plain = 'hello world'
      const markdown = 'hello world\n- item'

      expect(renderMarkdownCached(plain, scope)).toBe(renderMarkdown(plain))
      expect(renderMarkdownCached(markdown, scope)).toBe(renderMarkdown(markdown))
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

  describe('markdownToText', () => {
    it('should strip common markdown syntax', () => {
      const input = '# Title\n\n- **bold** item with `code`\n> quote'
      const result = markdownToText(input)

      expect(result).toBe('Title\n\nbold item with code\nquote')
    })

    it('should keep fenced code body and drop fences', () => {
      const input = '```ts\nconst x = 1\n```\n\nAfter'
      const result = markdownToText(input)

      expect(result).toBe('const x = 1\n\nAfter')
    })

    it('should keep link and image labels', () => {
      const input = '[OpenAI](https://openai.com) ![logo](https://example.com/a.png)'
      const result = markdownToText(input)

      expect(result).toBe('OpenAI logo')
    })
  })
})
