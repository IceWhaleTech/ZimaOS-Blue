import { describe, it, expect } from 'vitest'
import { parseTypelessContent } from '@/utils/typeless'

describe('Typeless Plain Text Code Blocks', () => {
  it('should parse code block without language as plain text', () => {
    const content = `
\`\`\`
人工智能（AI）
├── 传统方法（规则、搜索等）
├── 机器学习（ML）
│   ├── 浅层学习
│   └── 深度学习（DL）← 最先进的方向
└── 其他方法
\`\`\`
`
    const result = parseTypelessContent(content)

    expect(result.cards).toHaveLength(1)
    expect(result.cards[0]?.type).toBe('code')

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const codeCard = result.cards[0] as any
    expect(codeCard.language).toBeUndefined()
    expect(codeCard.code).toContain('人工智能（AI）')
    expect(codeCard.code).toContain('├──')
    expect(codeCard.code).toContain('│')
    expect(codeCard.code).toContain('└──')
  })

  it('should parse code block with language as code', () => {
    const content = `
\`\`\`javascript
console.log('hello')
\`\`\`
`
    const result = parseTypelessContent(content)

    expect(result.cards).toHaveLength(1)
    expect(result.cards[0]?.type).toBe('code')

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const codeCard = result.cards[0] as any
    expect(codeCard.language).toBe('javascript')
    expect(codeCard.code).toContain('console.log')
  })

  it('should parse code block with md language as markdown', () => {
    const content = `
\`\`\`md
# Title
\`\`\`
`
    const result = parseTypelessContent(content)

    expect(result.cards).toHaveLength(1)
    expect(result.cards[0]?.type).toBe('code')

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const codeCard = result.cards[0] as any
    expect(codeCard.language).toBe('markdown')
    expect(codeCard.code).toContain('# Title')
  })

  it('should handle empty language string as undefined', () => {
    const content = '```\ntest\n```'
    const result = parseTypelessContent(content)

    expect(result.cards).toHaveLength(1)
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const codeCard = result.cards[0] as any
    expect(codeCard.language).toBeUndefined()
  })
})
