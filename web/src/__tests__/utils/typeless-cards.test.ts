import { describe, it, expect, beforeEach } from 'vitest'
import {
  parseTypelessContent,
  parseTypelessContentIncremental,
  splitIntoSegments,
  clearSplitSegmentsIncrementalState,
} from '@/utils/typeless'

describe('Typeless Card Parsing', () => {
  beforeEach(() => {
    clearSplitSegmentsIncrementalState()
  })

  it('parses exec typeless blocks into exec cards', () => {
    const content = [
      '```typeless',
      '{"type":"exec","command":"blue web_search query=\\\"zimaos-blue 最新消息\\\"","status":"success","exit_code":0,"stdout":"query: zimaos-blue 最新消息"}',
      '```',
    ].join('\n')

    const result = parseTypelessContent(content)

    expect(result.cards).toHaveLength(1)
    expect(result.cards[0]?.type).toBe('exec')
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const execCard = result.cards[0] as any
    expect(execCard.command).toContain('blue web_search')
    expect(execCard.stdout).toContain('query: zimaos-blue 最新消息')
  })

  it('parses web-fetch typeless cards with warning metadata', () => {
    const content = [
      '```typeless',
      '{"type":"web-fetch","id":"web-fetch-https%3A%2F%2Fwww.reddit.com%2Fr%2Ftest","title":"Sign in","status":"warning","url":"https://www.reddit.com/r/test","content":"Log in to continue","content_type":"text/html","extract_mode":"text","extractor":"html","warning":"page appears to be a login wall; use browser or pass browser_target_id","warning_code":"login_wall","actions":[{"id":"use_browser","label":"Use browser","variant":"primary","form_data":{"url":"https://www.reddit.com/r/test"}}]}',
      '```',
    ].join('\n')

    const result = parseTypelessContent(content)

    expect(result.cards).toHaveLength(1)
    const card = result.cards[0] as any
    expect(card.type).toBe('web-fetch')
    expect(card.warning_code).toBe('login_wall')
    expect(card.url).toContain('reddit.com')
    expect(card.id).toBe('web-fetch-https%3A%2F%2Fwww.reddit.com%2Fr%2Ftest')
    expect(card.actions?.[0]?.id).toBe('use_browser')
    expect(card.actions?.[0]?.form_data?.url).toBe('https://www.reddit.com/r/test')
  })

  it('preserves warning_code on typeless result cards', () => {
    const content = [
      '```typeless',
      '{"type":"result","title":"Sign in","status":"warning","warning":"page appears to be a login wall; use browser or pass browser_target_id","warning_code":"login_wall","details":[{"label":"url","value":"https://www.reddit.com/r/test"}]}',
      '```',
    ].join('\n')

    const result = parseTypelessContent(content)

    expect(result.cards).toHaveLength(1)
    const card = result.cards[0] as any
    expect(card.type).toBe('result')
    expect(card.warning_code).toBe('login_wall')
    expect(card.warning).toContain('login wall')
  })

  it('parses typeless exec block correctly after function_calls block', () => {
    const content = [
      '<function_calls>',
      '<invoke name="web_search">',
      '<parameter name="query">ZimaOS Blue 最新消息 2026</parameter>',
      '</invoke>',
      '</function_calls>',
      '',
      '```typeless',
      '{"type":"exec","id":"card-0","command":"blue web_search query=\\"ZimaOS Blue 最新消息 2026\\"","status":"success","stdout":"query: ZimaOS Blue 最新消息 2026\\nprovider: duckduckgo"}',
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
      '<parameter name="query">ZimaOS Blue 最新消息 2026</parameter>',
      '</invoke>',
      '</function_calls>',
      '',
      '```typeless',
      '{"type":"exec","id":"card-0","command":"blue web_search query=\\"ZimaOS Blue 最新消息 2026\\"","status":"running","stdout":"query: ZimaOS Blue',
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

  it('returns cached incremental parse result when content is unchanged', () => {
    const content = [
      '<function_calls>',
      '<invoke name="web_search">',
      '<parameter name="query">ZimaOS Blue 最新消息 2026</parameter>',
      '</invoke>',
      '</function_calls>',
      '',
      '```typeless',
      '{"type":"exec","id":"card-0","command":"blue web_search query=\\"ZimaOS Blue 最新消息 2026\\"","status":"running","stdout":"query: ZimaOS Blue 最新消息 2026"}',
      '```',
    ].join('\n')

    const first = parseTypelessContentIncremental(content, 'msg-streaming-stable', 'conv-stable')
    const second = parseTypelessContentIncremental(content, 'msg-streaming-stable', 'conv-stable')

    expect(second).toBe(first)
  })

  it('parses multiple consecutive typeless blocks and keeps ui-review progress cards', () => {
    const content = [
      '```typeless',
      '{"type":"ui-review-progress","step":"navigate","name":"加载页面","status":"success","url":"https://www.zimaspace.com/zimaos"}',
      '```',
      '',
      '```typeless',
      '{"type":"ui-review-progress","step":"visual","name":"视觉评审","status":"running","url":"https://www.zimaspace.com/zimaos"}',
      '```',
      '',
      '```typeless',
      '{"type":"ui-review","url":"https://www.zimaspace.com/zimaos","overall":{"score":82}}',
      '```',
    ].join('\n')

    const result = parseTypelessContent(content)
    const progressCards = result.cards.filter(c => c.type === 'ui-review-progress')
    const reviewCards = result.cards.filter(c => c.type === 'ui-review')

    expect(progressCards).toHaveLength(2)
    expect(reviewCards).toHaveLength(1)
    expect(result.text).toContain('[[TYPELESS_CARD:')
    expect(result.text).not.toContain('```typeless')

    const segments = splitIntoSegments(result.text, result.cards)
    const cardSegments = segments.filter(s => s.type === 'card')
    expect(cardSegments.length).toBe(2) // merged progress + ui-review

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const mergedProgress = cardSegments[0]?.content as any
    expect(mergedProgress.type).toBe('ui-review-progress')
    expect(Array.isArray(mergedProgress.steps)).toBe(true)
    expect(mergedProgress.steps).toHaveLength(2)
  })

  it('keeps malformed empty placeholders as plain text while parsing later valid placeholders', () => {
    const text = 'prefix [[TYPELESS_CARD:]] mid [[TYPELESS_CARD:card-1]] suffix'
    const cards = [{ type: 'result', id: 'card-1' }] as any

    const segments = splitIntoSegments(text, cards)

    expect(segments).toHaveLength(3)
    expect(segments[0]).toEqual({ type: 'text', content: 'prefix [[TYPELESS_CARD:]] mid' })
    expect(segments[1]?.type).toBe('card')
    expect((segments[1]?.content as any).id).toBe('card-1')
    expect(segments[2]).toEqual({ type: 'text', content: 'suffix' })
  })

  it('drops unknown placeholders and keeps surrounding text segments', () => {
    const text = 'left [[TYPELESS_CARD:missing]] right'

    const segments = splitIntoSegments(text, [])

    expect(segments).toEqual([
      { type: 'text', content: 'left' },
      { type: 'text', content: 'right' },
    ])
  })

  it('incrementally appends text after trailing card segment', () => {
    const cards = [{ type: 'result', id: 'card-1' }] as any
    const key = 'conv-1:msg-1'

    const first = splitIntoSegments('[[TYPELESS_CARD:card-1]]', cards, key)
    const second = splitIntoSegments('[[TYPELESS_CARD:card-1]] done', cards, key)

    expect(first).toEqual([{ type: 'card', content: cards[0] }])
    expect(second).toEqual([
      { type: 'card', content: cards[0] },
      { type: 'text', content: 'done' },
    ])
  })

  it('incremental split falls back to full scan when delta adds card placeholder', () => {
    const cards = [{ type: 'result', id: 'card-1' }] as any
    const key = 'conv-1:msg-2'

    const first = splitIntoSegments('prefix', cards, key)
    const second = splitIntoSegments('prefix [[TYPELESS_CARD:card-1]] suffix', cards, key)

    expect(first).toEqual([{ type: 'text', content: 'prefix' }])
    expect(second).toEqual([
      { type: 'text', content: 'prefix' },
      { type: 'card', content: cards[0] },
      { type: 'text', content: 'suffix' },
    ])
  })

  it('resolves placeholder that was split across incremental chunks', () => {
    const cards = [{ type: 'result', id: 'card-1' }] as any
    const key = 'conv-1:msg-3'

    const first = splitIntoSegments('prefix [[TYPELESS_CARD:card-1', cards, key)
    const second = splitIntoSegments('prefix [[TYPELESS_CARD:card-1]] suffix', cards, key)

    expect(first).toEqual([{ type: 'text', content: 'prefix [[TYPELESS_CARD:card-1' }])
    expect(second).toEqual([
      { type: 'text', content: 'prefix' },
      { type: 'card', content: cards[0] },
      { type: 'text', content: 'suffix' },
    ])
  })

  it('merges progress cards across incremental chunk boundaries', () => {
    const cards = [
      { type: 'ui-review-progress', id: 'p1', step: 'first', status: 'running' },
      { type: 'ui-review-progress', id: 'p2', step: 'second', status: 'success' },
    ] as any
    const key = 'conv-1:msg-4'

    const first = splitIntoSegments('[[TYPELESS_CARD:p1]]', cards, key)
    const second = splitIntoSegments('[[TYPELESS_CARD:p1]][[TYPELESS_CARD:p2]]', cards, key)

    expect(first).toHaveLength(1)
    expect(first[0]?.type).toBe('card')
    expect(second).toHaveLength(1)
    expect(second[0]?.type).toBe('card')
    const merged = second[0]?.content as any
    expect(merged.type).toBe('ui-review-progress')
    expect(merged.steps).toHaveLength(2)
  })
})
