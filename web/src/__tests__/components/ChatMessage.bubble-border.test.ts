import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

function readSource(relativePath: string): string {
  return readFileSync(resolve(process.cwd(), relativePath), 'utf8')
}

function extractBlock(source: string, selector: string): string {
  const escapedSelector = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  const match = source.match(new RegExp(`${escapedSelector}\\s*\\{([\\s\\S]*?)\\n\\}`, 'm'))
  if (!match) {
    throw new Error(`Missing style block for selector: ${selector}`)
  }
  return match[1]
}

describe('ChatMessage assistant bubble border styles', () => {
  it('keeps assistant border variables wired to the global bubble class', () => {
    const styleSource = readSource('src/style.css')
    const assistantBubbleBlock = extractBlock(styleSource, '.chat-assistant-bubble')

    expect(assistantBubbleBlock).toContain('background: var(--chat-assistant-bg);')
    expect(assistantBubbleBlock).toContain('color: var(--chat-assistant-text);')
    expect(assistantBubbleBlock).toContain(
      'border: var(--chat-assistant-border, 1px solid transparent);'
    )
    expect(assistantBubbleBlock).toContain('border-radius: var(--chat-bubble-radius);')
  })

  it('does not let the base assistant wrapper clear the bubble border', () => {
    const componentSource = readSource('src/components/ChatMessage.vue')
    const baseAssistantBlock = extractBlock(componentSource, '.assistant-message')
    const assistantBubbleBlock = extractBlock(componentSource, '.chat-assistant-bubble')
    const indicatorOnlyBlock = extractBlock(
      componentSource,
      '.assistant-message.assistant-message-indicator-only'
    )

    expect(baseAssistantBlock).not.toMatch(/\bborder\s*:/)
    expect(assistantBubbleBlock).toContain(
      'border: var(--chat-assistant-border, 1px solid rgba(148, 163, 184, 0.24));'
    )
    expect(assistantBubbleBlock).not.toContain('border-radius: 0;')
    expect(indicatorOnlyBlock).toContain('border: 0;')
  })
})
