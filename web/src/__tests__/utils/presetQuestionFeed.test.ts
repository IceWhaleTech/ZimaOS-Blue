import { describe, expect, it } from 'vitest'
import type { PresetQuestion } from '@/api/preview'
import {
  PRESET_FEED_INITIAL_LOAD_COUNT,
  createDefaultTryFeedState,
  rankPresetQuestions,
  recordPresetQuestionSend,
} from '@/utils/presetQuestionFeed'

function question(
  id: string,
  options: Partial<PresetQuestion> & {
    category: PresetQuestion['category']
    editorial_score: number
  }
): PresetQuestion {
  return {
    id,
    title: options.title || id,
    description: options.description || `${id} description`,
    prompt: options.prompt || `${id} prompt`,
    text: options.prompt || `${id} prompt`,
    category: options.category,
    tags: options.tags || [options.category],
    editorial_score: options.editorial_score,
    icon: options.icon,
    attachments: options.attachments,
  }
}

describe('presetQuestionFeed', () => {
  it('keeps the first paint load smaller than the full 12-slot feed', () => {
    expect(PRESET_FEED_INITIAL_LOAD_COUNT).toBe(4)
  })

  it('uses editorial score as the default cold-start order', () => {
    const questions = [
      question('b', { category: 'learning-growth', editorial_score: 100 }),
      question('a', { category: 'personal-knowledge', editorial_score: 120 }),
      question('c', { category: 'product-design', editorial_score: 90 }),
    ]

    const ranked = rankPresetQuestions(questions, { state: createDefaultTryFeedState() })

    expect(ranked.map((item) => item.id)).toEqual(['a', 'b', 'c'])
  })

  it('boosts explicitly selected interests ahead of higher editorial defaults', () => {
    const questions = [
      question('market', { category: 'market-investing', editorial_score: 92 }),
      question('design', { category: 'product-design', editorial_score: 108 }),
    ]

    const ranked = rankPresetQuestions(questions, {
      state: {
        selectedTags: ['market-investing'],
        clickCounts: {},
        lastSentIds: [],
      },
    })

    expect(ranked.map((item) => item.id)).toEqual(['market', 'design'])
  })

  it('caps history bonus at 24 points per card', () => {
    const questions = [
      question('market', { category: 'market-investing', editorial_score: 100 }),
      question('design', { category: 'product-design', editorial_score: 125 }),
    ]

    const ranked = rankPresetQuestions(questions, {
      state: {
        selectedTags: [],
        clickCounts: { 'market-investing': 10 },
        lastSentIds: [],
      },
    })

    expect(ranked.map((item) => item.id)).toEqual(['design', 'market'])
  })

  it('applies the adjacency penalty to break up consecutive primary tags', () => {
    const questions = [
      question('market-1', { category: 'market-investing', editorial_score: 100 }),
      question('market-2', { category: 'market-investing', editorial_score: 99 }),
      question('memory', { category: 'personal-knowledge', editorial_score: 81 }),
    ]

    const ranked = rankPresetQuestions(questions, { state: createDefaultTryFeedState() })

    expect(ranked.map((item) => item.id)).toEqual(['market-1', 'memory', 'market-2'])
  })

  it('records sent-question history by tag and recency', () => {
    const nextState = recordPresetQuestionSend(
      createDefaultTryFeedState(),
      question('memory-bank', {
        category: 'personal-knowledge',
        editorial_score: 120,
        tags: ['personal-knowledge', 'learning-growth'],
      })
    )

    expect(nextState.clickCounts['personal-knowledge']).toBe(1)
    expect(nextState.clickCounts['learning-growth']).toBe(1)
    expect(nextState.lastSentIds).toEqual(['memory-bank'])
  })
})
