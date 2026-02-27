import { describe, it, expect } from 'vitest'
import { parseTypelessContent } from '@/utils/typeless'

describe('Typeless Card Parsing', () => {
  it('parses exec typeless blocks into exec cards', () => {
    const content = [
      '```typeless',
      '{"type":"exec","command":"blue web_search query=\\\"openclaw 最新消息\\\"","status":"success","exit_code":0,"stdout":"query: openclaw 最新消息"}',
      '```',
    ].join('\n')

    const result = parseTypelessContent(content)

    expect(result.cards).toHaveLength(1)
    expect(result.cards[0]?.type).toBe('exec')
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const execCard = result.cards[0] as any
    expect(execCard.command).toContain('blue web_search')
    expect(execCard.stdout).toContain('query: openclaw 最新消息')
  })
})

