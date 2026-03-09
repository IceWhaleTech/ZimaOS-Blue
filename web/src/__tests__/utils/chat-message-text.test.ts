import { describe, it, expect } from 'vitest'
import { stripFirstLineHeading, normalizeToolFallbackSummaryText } from '@/utils/chat-message-text'

describe('chat-message-text', () => {
  describe('stripFirstLineHeading', () => {
    it('returns empty string for heading-only content', () => {
      expect(stripFirstLineHeading('# Title')).toBe('')
    })

    it('strips a heading with leading whitespace on first line', () => {
      const input = ' \t  ## 标题\n\n  Body content'
      expect(stripFirstLineHeading(input)).toBe('Body content')
    })

    it('keeps content unchanged when first line is not a heading', () => {
      const input = 'Plain first line\n# second line heading-like text'
      expect(stripFirstLineHeading(input)).toBe(input)
    })

    it('preserves non-heading lines after removing first heading line', () => {
      const input = '# Heading\nline1\nline2'
      expect(stripFirstLineHeading(input)).toBe('line1\nline2')
    })
  })

  describe('normalizeToolFallbackSummaryText', () => {
    it('unwraps chinese extracted fallback summary boilerplate', () => {
      const input = '工具执行已完成，但最终总结生成失败。以下是从工具结果自动提炼的安全摘要：\n\nWeb search fallback results for "OpenClaw 最近动向":\n\n- OpenClaw Release Notes\n\n原始 stdout/stderr/error 字段已隐藏以保护安全。如需我重试完整总结，请回复“重试总结”。'
      expect(normalizeToolFallbackSummaryText(input)).toBe('Web search fallback results for "OpenClaw 最近动向":\n\n- OpenClaw Release Notes')
    })

    it('unwraps english extracted fallback summary boilerplate', () => {
      const input = 'Tool execution completed, but final summary generation failed. Here is a safe fallback summary extracted from tool results:\n\nWeb search fallback results for "OpenClaw recent updates":\n\n- OpenClaw Release Notes\n\nRaw stdout/stderr/error fields remain hidden for safety. Ask me to retry summarizing for a full report.'
      expect(normalizeToolFallbackSummaryText(input)).toBe('Web search fallback results for "OpenClaw recent updates":\n\n- OpenClaw Release Notes')
    })

    it('keeps non-fallback text unchanged', () => {
      const input = '最近一周 OpenClaw 主要动态集中在 GitHub 发布说明和文档更新。'
      expect(normalizeToolFallbackSummaryText(input)).toBe(input)
    })
  })
})
