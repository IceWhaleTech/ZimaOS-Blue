import { describe, it, expect } from 'vitest'
import { hasTypelessCards } from '@/utils/typeless'

describe('Typeless Detection', () => {
  it('detects explicit typeless marker blocks', () => {
    const content = '```typeless\n{"type":"steps","steps":[]}\n```'
    expect(hasTypelessCards(content)).toBe(true)
  })

  it('detects markdown tables with at least two data rows', () => {
    const content = [
      '| Name | Value |',
      '| --- | --- |',
      '| CPU | 58% |',
      '| RAM | 72% |',
    ].join('\n')
    expect(hasTypelessCards(content)).toBe(true)
  })

  it('resets table detection after non-separator content', () => {
    const content = [
      '| Name | Value |',
      'not a table row',
      '| CPU | 58% |',
    ].join('\n')
    expect(hasTypelessCards(content)).toBe(false)
  })

  it('detects unordered lists across blank lines', () => {
    const content = '- item one\n\n- item two'
    expect(hasTypelessCards(content)).toBe(true)
  })

  it('detects ordered lists', () => {
    const content = '1. first\n2. second'
    expect(hasTypelessCards(content)).toBe(true)
  })

  it('detects unordered lists with tab indentation', () => {
    const content = '\t- first\n\t- second'
    expect(hasTypelessCards(content)).toBe(true)
  })

  it('does not detect single-line dash text as list card', () => {
    const content = 'task - keep plain text'
    expect(hasTypelessCards(content)).toBe(false)
  })

  it('does not detect separator-only table lines as table rows', () => {
    const content = '---|---\n---|---'
    expect(hasTypelessCards(content)).toBe(false)
  })
})
