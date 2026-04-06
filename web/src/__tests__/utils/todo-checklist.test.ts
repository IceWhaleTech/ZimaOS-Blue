import { describe, it, expect } from 'vitest'

import type { Message } from '@/api/chat'
import {
  extractFirstTodoChecklistBlock,
  findLatestTodoChecklistSummary,
  getTodoChecklistSignature,
  summarizeTodoChecklist,
  stripDuplicateTodoChecklistForMessage,
  stripFirstTodoChecklistBlock,
} from '@/utils/todoChecklist'

function assistantMessage(id: string, content: string, overrides: Partial<Message> = {}): Message {
  return {
    id,
    conversation_id: 'conv-1',
    role: 'assistant',
    content,
    created_at: '2026-03-11T00:00:00.000Z',
    ...overrides,
  }
}

function userMessage(id: string, content: string): Message {
  return {
    id,
    conversation_id: 'conv-1',
    role: 'user',
    content,
    created_at: '2026-03-11T00:00:00.000Z',
  }
}

describe('todo checklist rendering helpers', () => {
  it('extracts and strips the first markdown checklist block', () => {
    const content = [
      '先给你当前进度：',
      '',
      '- [x] 收集信息',
      '- [ ] 写总结',
      '',
      '下面继续执行第二步。',
    ].join('\n')

    expect(extractFirstTodoChecklistBlock(content)).toBe('- [x] 收集信息\n- [ ] 写总结')
    expect(stripFirstTodoChecklistBlock(content)).toBe('先给你当前进度：\n\n下面继续执行第二步。')
  })

  it('builds a stable signature that ignores checkbox state', () => {
    const pending = '- [ ] 收集信息\n- [ ] 写总结'
    const updated = '- [x] 收集信息\n- [ ] 写总结'

    expect(getTodoChecklistSignature(pending)).toBe(getTodoChecklistSignature(updated))
  })

  it('summarizes checklist progress and item state', () => {
    const summary = summarizeTodoChecklist('- [x] 收集信息\n- [ ] 写总结')

    expect(summary).toEqual({
      items: [
        { checked: true, text: '收集信息' },
        { checked: false, text: '写总结' },
      ],
      totalCount: 2,
      completedCount: 1,
      pendingCount: 1,
      allCompleted: false,
    })
  })

  it('returns the latest assistant checklist summary', () => {
    const messages = [
      userMessage('msg-u1', '先做第一版'),
      assistantMessage('msg-a1', '- [ ] 旧任务\n- [ ] 旧验证'),
      userMessage('msg-u2', '继续'),
      assistantMessage('msg-a2', '- [x] 收集信息\n- [ ] 写总结\n\n我继续执行第二步。'),
      assistantMessage('msg-a3', '这里是不带 checklist 的总结。'),
    ]

    expect(findLatestTodoChecklistSummary(messages)).toEqual({
      messageId: 'msg-a2',
      focusMessageId: 'msg-a2',
      todoCardId: undefined,
      items: [
        { checked: true, text: '收集信息' },
        { checked: false, text: '写总结' },
      ],
      totalCount: 2,
      completedCount: 1,
      pendingCount: 1,
      allCompleted: false,
    })
  })

  it('returns null after a new non-continuation user scope starts', () => {
    const messages = [
      userMessage('msg-u1', '先查一下'),
      assistantMessage('msg-a1', '- [x] 收集信息\n- [ ] 写总结'),
      userMessage('msg-u2', '换个问题，直接帮我解释这个报错'),
      assistantMessage('msg-a2', '这是新的回答，不再沿用旧 checklist。'),
    ]

    expect(findLatestTodoChecklistSummary(messages)).toBeNull()
  })

  it('returns null when a completion-style summary follows the checklist', () => {
    const messages = [
      userMessage('msg-u1', '继续把任务做完'),
      assistantMessage('msg-a1', '- [x] 收集信息\n- [ ] 最终总结\n\n我先整理最后的交付内容。'),
      assistantMessage(
        'msg-a2',
        '任务已完成。\n完成内容：已整理最终结果并补充说明。\n使用方法：直接查看上面的输出。'
      ),
    ]

    expect(findLatestTodoChecklistSummary(messages)).toBeNull()
  })

  it('returns null when an artifact-delivery style message follows the checklist', () => {
    const messages = [
      userMessage('msg-u1', '继续把任务做完'),
      assistantMessage('msg-a1', '- [x] 收集信息\n- [ ] 将完整报告写入 reports/final.md'),
      assistantMessage('msg-a2', '已将完整报告写入 `reports/final.md`，可以直接查看。'),
    ]

    expect(findLatestTodoChecklistSummary(messages)).toBeNull()
  })

  it('returns null when the latest checklist is already fully completed', () => {
    const messages = [
      userMessage('msg-u1', '继续'),
      assistantMessage('msg-a1', '- [x] 收集信息\n- [x] 写总结'),
    ]

    expect(findLatestTodoChecklistSummary(messages)).toBeNull()
  })

  it('can keep a fully completed checklist visible while the turn is still in progress', () => {
    const messages = [
      userMessage('msg-u1', '继续'),
      assistantMessage('msg-a1', '- [x] 收集信息\n- [x] 写总结'),
    ]

    expect(
      findLatestTodoChecklistSummary(messages, null, {
        allowCompletedChecklist: true,
      })
    ).toEqual({
      messageId: 'msg-a1',
      focusMessageId: 'msg-a1',
      todoCardId: undefined,
      items: [
        { checked: true, text: '收集信息' },
        { checked: true, text: '写总结' },
      ],
      totalCount: 2,
      completedCount: 2,
      pendingCount: 0,
      allCompleted: true,
    })
  })

  it('keeps active summary pinned to the canonical checklist for duplicate echoes', () => {
    const canonical = assistantMessage('msg-a1', '- [ ] 收集信息\n- [ ] 写总结', {
      todo_card_id: 'todo-checklist-msg-a1',
    })
    const duplicate = assistantMessage(
      'msg-a3',
      '- [x] 收集信息\n- [ ] 写总结\n\n我继续执行第二步。'
    )
    const messages = [
      userMessage('msg-u1', '帮我整理一下这个问题'),
      canonical,
      userMessage('msg-u2', '继续'),
      duplicate,
    ]

    expect(findLatestTodoChecklistSummary(messages)).toEqual({
      messageId: 'msg-a1',
      focusMessageId: 'msg-a1',
      todoCardId: 'todo-checklist-msg-a1',
      items: [
        { checked: false, text: '收集信息' },
        { checked: false, text: '写总结' },
      ],
      totalCount: 2,
      completedCount: 0,
      pendingCount: 2,
      allCompleted: false,
    })
  })

  it('returns null when the latest checklist has an explicit completion signal', () => {
    const canonical = assistantMessage('msg-a1', '- [ ] 收集信息\n- [ ] 写总结', {
      todo_card_id: 'todo-checklist-msg-a1',
    })

    expect(
      findLatestTodoChecklistSummary([canonical], {
        messageId: 'msg-a1',
        todoCardId: 'todo-checklist-msg-a1',
      })
    ).toBeNull()
  })

  it('ignores the explicit completion signal when completed checklists should stay visible', () => {
    const canonical = assistantMessage('msg-a1', '- [x] 收集信息\n- [x] 写总结', {
      todo_card_id: 'todo-checklist-msg-a1',
    })

    expect(
      findLatestTodoChecklistSummary(
        [canonical],
        {
          messageId: 'msg-a1',
          todoCardId: 'todo-checklist-msg-a1',
        },
        {
          allowCompletedChecklist: true,
        }
      )
    ).toEqual(
      expect.objectContaining({
        messageId: 'msg-a1',
        todoCardId: 'todo-checklist-msg-a1',
        allCompleted: true,
      })
    )
  })

  it('suppresses duplicate checklist echoes after affirmative continuation turns', () => {
    const canonical = assistantMessage('msg-a1', '- [ ] 收集信息\n- [ ] 写总结', {
      todo_card_id: 'todo-checklist-msg-a1',
    })
    const duplicate = assistantMessage(
      'msg-a3',
      '- [x] 收集信息\n- [ ] 写总结\n\n我继续执行第二步。'
    )
    const messages = [
      userMessage('msg-u1', '帮我整理一下这个问题'),
      canonical,
      userMessage('msg-u2', '继续'),
      duplicate,
    ]

    expect(stripDuplicateTodoChecklistForMessage(messages, duplicate)).toBe('我继续执行第二步。')
  })

  it('keeps the canonical checklist bubble intact', () => {
    const canonical = assistantMessage('msg-a1', '- [x] 收集信息\n- [ ] 写总结', {
      todo_card_id: 'todo-checklist-msg-a1',
    })

    expect(stripDuplicateTodoChecklistForMessage([canonical], canonical)).toBe(canonical.content)
  })

  it('does not suppress a new checklist after a non-continuation user turn', () => {
    const oldChecklist = assistantMessage('msg-a1', '- [ ] 收集信息\n- [ ] 写总结')
    const newChecklist = assistantMessage(
      'msg-a3',
      '- [x] 收集信息\n- [ ] 写总结\n\n我会重新整理一版。'
    )
    const messages = [
      userMessage('msg-u1', '先查一下'),
      oldChecklist,
      userMessage('msg-u2', '换个主题，帮我重写输出结构'),
      newChecklist,
    ]

    expect(stripDuplicateTodoChecklistForMessage(messages, newChecklist)).toBe(newChecklist.content)
  })
})
