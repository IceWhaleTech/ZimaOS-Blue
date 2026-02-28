import { describe, it, expect } from 'vitest'
import { parseTypelessContent, parseTypelessContentIncremental } from '@/utils/typeless'

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

  it('parses typeless exec block correctly after function_calls block', () => {
    const content = [
      '<function_calls>',
      '<invoke name="web_search">',
      '<parameter name="query">OpenClaw 最新消息 2026</parameter>',
      '</invoke>',
      '</function_calls>',
      '',
      '```typeless',
      '{"type":"exec","id":"card-0","command":"blue web_search query=\\"OpenClaw 最新消息 2026\\"","status":"success","stdout":"query: OpenClaw 最新消息 2026\\nprovider: duckduckgo"}',
      '```',
    ].join('\n')

    const result = parseTypelessContent(content)

    expect(result.cards.length).toBeGreaterThanOrEqual(2)
    const execCard = result.cards.find(c => c.type === 'exec')
    expect(execCard).toBeTruthy()
    expect(result.text).toContain('[[TYPELESS_CARD:fc-0]]')
    expect(result.text).toContain('[[TYPELESS_CARD:card-0]]')
    expect(result.text).not.toContain('```typeless')
    expect(result.text).not.toContain('"type":"exec"')
  })

  it('keeps streaming typeless exec block stable with function_calls prefix', () => {
    const chunk1 = [
      '<function_calls>',
      '<invoke name="web_search">',
      '<parameter name="query">OpenClaw 最新消息 2026</parameter>',
      '</invoke>',
      '</function_calls>',
      '',
      '```typeless',
      '{"type":"exec","id":"card-0","command":"blue web_search query=\\"OpenClaw 最新消息 2026\\"","status":"running","stdout":"query: OpenClaw',
    ].join('\n')

    const partial = parseTypelessContentIncremental(chunk1, 'msg-streaming-1', 'conv-1')
    const partialExec = partial.cards.find(c => c.type === 'exec')
    expect(partialExec).toBeTruthy()

    const chunk2 = chunk1 + ' 最新消息 2026\\nprovider: duckduckgo"}\n```'
    const full = parseTypelessContentIncremental(chunk2, 'msg-streaming-1', 'conv-1')
    const fullExec = full.cards.find(c => c.type === 'exec')
    expect(fullExec).toBeTruthy()

    expect(full.text).toContain('[[TYPELESS_CARD:fc-0]]')
    expect(full.text).toContain('[[TYPELESS_CARD:card-0]]')
    expect(full.text).not.toContain('```typeless')
    expect(full.text).not.toContain('"type":"exec"')
  })
})
