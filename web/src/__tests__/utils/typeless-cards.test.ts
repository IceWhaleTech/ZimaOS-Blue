import { describe, it, expect, beforeEach } from 'vitest'
import {
  hasTypelessCards,
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

  it('parses absolute directory paths into file cards', () => {
    const result = parseTypelessContent('/Users/orca/Documents/GitHub/ZimaOS-Blue')

    expect(result.cards).toHaveLength(1)
    const card = result.cards[0] as any
    expect(card.type).toBe('file')
    expect(card.filename).toBe('ZimaOS-Blue')
    expect(card.downloadUrl).toBe('/Users/orca/Documents/GitHub/ZimaOS-Blue')
  })

  it('does not parse broken html tag fragments as local file cards', () => {
    const content = [
      '/html>',
      '',
      "printf '%s\\n' '<!DOCTYPE html>' > /Users/orca/.zimaos-blue/data/workspace/solar-system.html",
    ].join('\n')

    const result = parseTypelessContent(content)

    expect(result.cards).toHaveLength(0)
    expect(result.text).toContain('/html>')
    expect(result.text).toContain('/Users/orca/.zimaos-blue/data/workspace/solar-system.html')
  })

  it('does not parse /api paths as local file cards', () => {
    const result = parseTypelessContent('/api/v1/media/analyze/r1.html')

    expect(result.cards).toHaveLength(0)
    expect(result.text).toContain('/api/v1/media/analyze/r1.html')
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
    const execCard = result.cards.find((c) => c.type === 'exec')
    expect(execCard).toBeTruthy()
    expect(result.text).toContain('[[TYPELESS_CARD:fc-0]]')
    expect(result.text).toContain('[[TYPELESS_CARD:card-0]]')
    expect(result.text).not.toContain('```typeless')
    expect(result.text).not.toContain('"type":"exec"')
  })

  it('treats function_calls-only payloads as typeless content', () => {
    const content = [
      '<function_calls>',
      '<invoke name="web_query">',
      '<parameter name="query">OpenAI Responses API docs</parameter>',
      '</invoke>',
      '</function_calls>',
    ].join('\n')

    expect(hasTypelessCards(content)).toBe(true)
  })

  it('unwraps $blue wrapper invocations into the underlying tool calls', () => {
    const content = [
      '<function_calls>',
      '<invoke name="$blue">',
      '<parameter name="command">deep_research</parameter>',
      '<parameter name="args">',
      '<parameter name="query">Apple AAPL stock price today April 2026</parameter>',
      '</parameter>',
      '</invoke>',
      '</function_calls>',
      '<function_calls>',
      '<invoke name="file_write">',
      '<parameter name="path">stock_report.txt</parameter>',
      '<parameter name="content">苹果公司(AAPL)股票报告</parameter>',
      '</invoke>',
      '</function_calls>',
    ].join('\n')

    const result = parseTypelessContent(content)
    const stepsCards = result.cards.filter((card) => card.type === 'steps') as any[]
    const allSteps = stepsCards.flatMap((card) => card.steps ?? [])

    expect(stepsCards).toHaveLength(2)
    expect(result.text).not.toContain('<function_calls>')
    expect(allSteps).toHaveLength(2)
    expect(allSteps[0]?.title).toContain('Apple AAPL stock price today April 2026')
    expect(allSteps[0]?.title).not.toContain('$blue')
    expect(allSteps[1]?.title.toLowerCase()).toContain('file')
    expect(allSteps[1]?.description).toContain('stock_report.txt')
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
    const partialExec = partial.cards.find((c) => c.type === 'exec')
    expect(partialExec).toBeTruthy()

    const chunk2 = chunk1 + ' 最新消息 2026\\nprovider: duckduckgo"}\n```'
    const full = parseTypelessContentIncremental(chunk2, 'msg-streaming-1', 'conv-1')
    const fullExec = full.cards.find((c) => c.type === 'exec')
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

  it('assigns a stable explicit card id to checklist cards within the same message', () => {
    const initial = parseTypelessContent(
      '- [ ] gather facts\n- [ ] write summary',
      'render-msg-1',
      'conv-1'
    )
    const updated = parseTypelessContent(
      '- [x] gather facts\n- [ ] write summary',
      'render-msg-1',
      'conv-1'
    )

    const initialChecklist = initial.cards.find((card) => card.type === 'list') as any
    const updatedChecklist = updated.cards.find((card) => card.type === 'list') as any

    expect(initialChecklist).toBeTruthy()
    expect(updatedChecklist).toBeTruthy()
    expect(initialChecklist.variant).toBe('checklist')
    expect(updatedChecklist.variant).toBe('checklist')
    expect(initialChecklist.id).toBe(updatedChecklist.id)
    expect(initialChecklist.id).toContain('todo-checklist-')
  })

  it('prefers explicit todo card ids when provided by message metadata', () => {
    const result = parseTypelessContent(
      '- [ ] gather facts\n- [ ] write summary',
      'render-msg-1',
      'conv-1',
      'todo-checklist-msg-assistant-current'
    )

    const checklist = result.cards.find((card) => card.type === 'list') as any

    expect(checklist.id).toBe('todo-checklist-msg-assistant-current')
    expect(checklist.variant).toBe('checklist')
  })

  it('keeps non-checklist card ids stable when an explicit todo card id arrives later', () => {
    const content = [
      '```ts',
      'console.log("hello")',
      '```',
      '',
      '- [ ] gather facts',
      '- [ ] write summary',
    ].join('\n')

    const initial = parseTypelessContent(content, 'render-msg-1', 'conv-1')
    const updated = parseTypelessContent(
      content,
      'render-msg-1',
      'conv-1',
      'todo-checklist-msg-assistant-current'
    )

    const initialCode = initial.cards.find((card) => card.type === 'code') as any
    const updatedCode = updated.cards.find((card) => card.type === 'code') as any
    const updatedChecklist = updated.cards.find((card) => card.type === 'list') as any

    expect(initialCode).toBeTruthy()
    expect(updatedCode).toBeTruthy()
    expect(initialCode.id).toBe(updatedCode.id)
    expect(updatedChecklist.id).toBe('todo-checklist-msg-assistant-current')
  })

  it('keeps checklist card ids unique across different messages with identical content', () => {
    const first = parseTypelessContent(
      '- [ ] gather facts\n- [ ] write summary',
      'render-msg-1',
      'conv-1'
    )
    const second = parseTypelessContent(
      '- [ ] gather facts\n- [ ] write summary',
      'render-msg-2',
      'conv-1'
    )

    const firstChecklist = first.cards.find((card) => card.type === 'list') as any
    const secondChecklist = second.cards.find((card) => card.type === 'list') as any

    expect(firstChecklist.id).not.toBe(secondChecklist.id)
  })

  it('keeps streaming typeless cards alive when JSON content contains inner code fences', () => {
    const chunk1 = [
      '```typeless',
      '{"type":"web-fetch","id":"card-inner-fence","title":"Doc","status":"success","content":"介绍如下：\\n\\n```mermaid\\ngraph TD\\nA-->B\\n```"}',
    ].join('\n')

    const partial = parseTypelessContentIncremental(
      chunk1,
      'msg-streaming-inner-fence',
      'conv-inner-fence'
    )
    const partialCard = partial.cards.find((c) => c.id === 'card-inner-fence') as any

    expect(partialCard).toBeTruthy()
    expect(partialCard.type).toBe('web-fetch')
    expect(partialCard._streaming).toBe(true)
    expect(partial.text).toContain('[[TYPELESS_CARD:card-inner-fence]]')
    expect(partial.text).not.toContain('```typeless')

    const chunk2 = `${chunk1}\n\`\`\``
    const full = parseTypelessContentIncremental(
      chunk2,
      'msg-streaming-inner-fence',
      'conv-inner-fence'
    )
    const fullCard = full.cards.find((c) => c.id === 'card-inner-fence') as any

    expect(fullCard).toBeTruthy()
    expect(fullCard.type).toBe('web-fetch')
    expect(fullCard._streaming).toBeUndefined()
    expect(full.text).toContain('[[TYPELESS_CARD:card-inner-fence]]')
    expect(full.text).not.toContain('```typeless')
  })

  it('parses later typeless cards when earlier card JSON contains a literal typeless marker', () => {
    const content = [
      '```typeless',
      '{"type":"web-fetch","id":"card-literal-marker","title":"Doc","status":"success","content":"literal marker ```typeless inside text"}',
      '```',
      '',
      '```typeless',
      '{"type":"result","id":"card-after-literal-marker","title":"Done","status":"success"}',
      '```',
    ].join('\n')

    const result = parseTypelessContent(content)

    expect(result.cards).toHaveLength(2)
    expect(result.cards.find((card) => (card as any).id === 'card-literal-marker')).toBeTruthy()
    expect(
      result.cards.find((card) => (card as any).id === 'card-after-literal-marker')
    ).toBeTruthy()
    expect(result.text).not.toContain('```typeless')
  })

  it('does not invent a streaming card from a literal typeless marker inside completed card JSON', () => {
    const content = [
      '```typeless',
      '{"type":"web-fetch","id":"card-fake-inner-stream","title":"Doc","status":"success","content":"literal marker ```typeless {\\"type\\":\\"info\\",\\"content\\":\\"fake\\"} inside text"}',
      '```',
    ].join('\n')

    const result = parseTypelessContentIncremental(
      content,
      'msg-fake-inner-stream',
      'conv-fake-inner-stream'
    )

    expect(result.cards).toHaveLength(1)
    expect((result.cards[0] as any).id).toBe('card-fake-inner-stream')
    expect((result.cards[0] as any)._streaming).toBeUndefined()
    expect(result.text).toContain('[[TYPELESS_CARD:card-fake-inner-stream]]')
    expect(result.cards.some((card) => (card as any)._streaming)).toBe(false)
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
    const progressCards = result.cards.filter((c) => c.type === 'ui-review-progress')
    const reviewCards = result.cards.filter((c) => c.type === 'ui-review')

    expect(progressCards).toHaveLength(2)
    expect(reviewCards).toHaveLength(1)
    expect(result.text).toContain('[[TYPELESS_CARD:')
    expect(result.text).not.toContain('```typeless')

    const segments = splitIntoSegments(result.text, result.cards)
    const cardSegments = segments.filter((s) => s.type === 'card')
    expect(cardSegments.length).toBe(2) // merged progress + ui-review

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const mergedProgress = cardSegments[0]?.content as any
    expect(mergedProgress.type).toBe('ui-review-progress')
    expect(Array.isArray(mergedProgress.steps)).toBe(true)
    expect(mergedProgress.steps).toHaveLength(2)
  })

  it('merges consecutive deep research progress and event cards into a single timeline card', () => {
    const content = [
      '```typeless',
      '{"type":"deep-research-progress","id":"dr-progress-1","job_id":"job-1","query":"EU AI Act provider obligations","mode":"deep","stage":"retrieve","status":"running","progress":34,"iteration":1,"latest_action":"initial_retrieve"}',
      '```',
      '',
      '```typeless',
      '{"type":"deep-research-event","id":"dr-event-1","job_id":"job-1","query":"EU AI Act provider obligations","mode":"deep","event_kind":"planning","status":"info","summary":"Planned 5 research task(s)","iteration":1,"task_count":5,"tasks":[{"question":"Review provider duties","axis":"official"}]}',
      '```',
      '',
      '```typeless',
      '{"type":"deep-research-event","id":"dr-event-2","job_id":"job-1","query":"EU AI Act provider obligations","mode":"deep","event_kind":"source","status":"info","summary":"Collected 3 source(s)","iteration":1,"parallelism":4,"sources":[{"title":"EU AI Act text","url":"https://eur-lex.europa.eu/eli/reg/2024/1689/oj","domain":"eur-lex.europa.eu"}]}',
      '```',
      '',
      '```typeless',
      '{"type":"deep-research","id":"dr-result-1","query":"EU AI Act provider obligations","status":"completed","answer":"Done"}',
      '```',
    ].join('\n')

    const result = parseTypelessContent(content)
    const segments = splitIntoSegments(result.text, result.cards)
    const cardSegments = segments.filter((segment) => segment.type === 'card')

    expect(cardSegments).toHaveLength(2)

    const mergedTimeline = cardSegments[0]?.content as any
    expect(mergedTimeline.type).toBe('deep-research-timeline')
    expect(mergedTimeline.progress).toBe(34)
    expect(mergedTimeline.iteration).toBe(1)
    expect(mergedTimeline.steps).toHaveLength(2)
    expect(mergedTimeline.steps[0]?.event_kind).toBe('planning')
    expect(mergedTimeline.steps[1]?.event_kind).toBe('source')

    const finalResult = cardSegments[1]?.content as any
    expect(finalResult.type).toBe('deep-research')
    expect(finalResult.id).toBe('dr-result-1')
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

  it('updates trailing text immediately for small streaming deltas after a card', () => {
    const first = [
      '```typeless',
      '{"type":"result","id":"card-1","title":"Done","status":"success"}',
      '```',
      '',
      '已完成，接下来',
    ].join('\n')

    const second = `${first}继续说明`

    const initial = parseTypelessContentIncremental(
      first,
      'msg-streaming-small-delta',
      'conv-small-delta'
    )
    const updated = parseTypelessContentIncremental(
      second,
      'msg-streaming-small-delta',
      'conv-small-delta'
    )

    expect(initial.cards.find((card) => (card as any).id === 'card-1')).toBeTruthy()
    expect(initial.text).toContain('已完成，接下来')
    expect(updated.cards.find((card) => (card as any).id === 'card-1')).toBeTruthy()
    expect(updated.text).toContain('已完成，接下来继续说明')
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

  it('keeps appending streamed deep research events into the same merged timeline card', () => {
    const cards = [
      {
        type: 'deep-research-progress',
        id: 'dr-progress-1',
        job_id: 'job-1',
        query: 'Research topic',
        stage: 'retrieve',
        status: 'running',
        progress: 28,
        iteration: 1,
      },
      {
        type: 'deep-research-event',
        id: 'dr-event-1',
        job_id: 'job-1',
        event_kind: 'planning',
        status: 'info',
        summary: 'Planned 4 research task(s)',
        iteration: 1,
      },
      {
        type: 'deep-research-event',
        id: 'dr-event-2',
        job_id: 'job-1',
        event_kind: 'verification',
        status: 'info',
        summary: 'Verification completed',
        iteration: 1,
      },
    ] as any
    const key = 'conv-1:msg-dr-1'

    const first = splitIntoSegments(
      '[[TYPELESS_CARD:dr-progress-1]][[TYPELESS_CARD:dr-event-1]]',
      cards,
      key
    )
    const second = splitIntoSegments(
      '[[TYPELESS_CARD:dr-progress-1]][[TYPELESS_CARD:dr-event-1]][[TYPELESS_CARD:dr-event-2]]',
      cards,
      key
    )

    expect(first).toHaveLength(1)
    expect(first[0]?.type).toBe('card')
    expect((first[0]?.content as any).type).toBe('deep-research-timeline')
    expect((first[0]?.content as any).steps).toHaveLength(1)

    expect(second).toHaveLength(1)
    expect(second[0]?.type).toBe('card')
    expect((second[0]?.content as any).type).toBe('deep-research-timeline')
    expect((second[0]?.content as any).steps).toHaveLength(2)
    expect((second[0]?.content as any).steps[1]?.event_kind).toBe('verification')
  })

  it('preserves browser progress recipe metadata when merging steps', () => {
    const cards = [
      {
        type: 'browser-progress',
        id: 'bp1',
        step: 'screenshot',
        name: 'Capturing screenshot',
        status: 'success',
      },
      {
        type: 'browser-progress',
        id: 'bp2',
        step: 'recipe',
        name: 'Running login recipe',
        status: 'running',
        recipe_name: 'login recipe',
      },
    ] as any

    const segments = splitIntoSegments('[[TYPELESS_CARD:bp1]][[TYPELESS_CARD:bp2]]', cards)

    expect(segments).toHaveLength(1)
    expect(segments[0]?.type).toBe('card')
    const merged = segments[0]?.content as any
    expect(merged.type).toBe('browser-progress')
    expect(merged.steps).toHaveLength(2)
    expect(merged.steps[1]?.recipe_name).toBe('login recipe')
  })

  it('preserves pre-merged browser progress steps from persisted cards', () => {
    const cards = [
      {
        type: 'browser-progress',
        id: 'bp-merged',
        steps: [
          {
            step: 'navigate',
            name: 'Navigating',
            status: 'completed',
            url: 'https://openai.com/blog',
          },
          {
            step: 'snapshot',
            name: 'Reading page',
            status: 'running',
            url: 'https://openai.com/blog',
          },
        ],
      },
    ] as any

    const segments = splitIntoSegments('[[TYPELESS_CARD:bp-merged]]', cards)

    expect(segments).toHaveLength(1)
    expect(segments[0]?.type).toBe('card')
    const merged = segments[0]?.content as any
    expect(merged.type).toBe('browser-progress')
    expect(merged.steps).toHaveLength(2)
    expect(merged.steps[0]?.name).toBe('Navigating')
    expect(merged.steps[1]?.name).toBe('Reading page')
    expect(merged.steps[1]?.url).toBe('https://openai.com/blog')
  })

  it('keeps the latest analyze-progress step state and metadata when merging duplicates', () => {
    const cards = [
      {
        type: 'analyze-progress',
        id: 'ap1',
        step: 'url_fetch_1',
        name: 'Fetching URL 1/2',
        status: 'running',
        current: 1,
        total: 2,
        source_label: 'https://example.com',
      },
      {
        type: 'analyze-progress',
        id: 'ap2',
        step: 'url_fetch_1',
        name: 'Fetching URL 1/2',
        status: 'success',
        current: 1,
        total: 2,
        source_label: 'https://example.com',
        detail: 'Page content extracted',
        char_count: 1200,
      },
      {
        type: 'analyze-progress',
        id: 'ap3',
        step: 'analysis',
        name: 'Analyzing content',
        status: 'running',
      },
    ] as any

    const segments = splitIntoSegments(
      '[[TYPELESS_CARD:ap1]][[TYPELESS_CARD:ap2]][[TYPELESS_CARD:ap3]]',
      cards
    )

    expect(segments).toHaveLength(1)
    expect(segments[0]?.type).toBe('card')
    const merged = segments[0]?.content as any
    expect(merged.type).toBe('analyze-progress')
    expect(merged.steps).toHaveLength(2)
    expect(merged.steps[0]?.status).toBe('success')
    expect(merged.steps[0]?.detail).toBe('Page content extracted')
    expect(merged.steps[0]?.char_count).toBe(1200)
    expect(merged.steps[1]?.step).toBe('analysis')
  })
})
