import type { PresetQuestion } from '@/api/preview'

export const PRESET_QUESTION_I18N_SEGMENTS = {
  'memory-bank': 'memoryBank',
  'reading-companion': 'readingCompanion',
  'growth-map': 'growthMap',
  'content-planner': 'contentPlanner',
  'chip-market-briefing': 'chipMarketBriefing',
  'morning-briefing': 'morningBriefing',
  'macro-tracker': 'macroTracker',
  'design-adaptation': 'designAdaptation',
  'inspiration-feed': 'inspirationFeed',
  'sentiment-radar': 'sentimentRadar',
  'dream-dialogue': 'dreamDialogue',
  'aristotle-dialogue': 'aristotleDialogue',
} as const

export type PresetQuestionI18nSegment =
  (typeof PRESET_QUESTION_I18N_SEGMENTS)[keyof typeof PRESET_QUESTION_I18N_SEGMENTS]

type PresetQuestionCopyField = 'title' | 'description' | 'prompt'

const PRESET_QUESTION_COPY_FIELDS = ['title', 'description', 'prompt'] as const

export const PRESET_QUESTION_EXAMPLE_KEYS = Object.values(PRESET_QUESTION_I18N_SEGMENTS).flatMap(
  (segment) =>
    PRESET_QUESTION_COPY_FIELDS.map(
      (field) => `chat.presetQuestions.examples.${segment}.${field}`
    )
)

export function getPresetQuestionCopyKey(
  questionId: string,
  field: PresetQuestionCopyField
): string | null {
  const segment = PRESET_QUESTION_I18N_SEGMENTS[questionId as keyof typeof PRESET_QUESTION_I18N_SEGMENTS]
  if (!segment) return null
  return `chat.presetQuestions.examples.${segment}.${field}`
}

export function localizePresetQuestion(
  question: PresetQuestion,
  hasTranslation: (key: string) => boolean,
  translate: (key: string) => string
): PresetQuestion {
  const localizedTitleKey = getPresetQuestionCopyKey(question.id, 'title')
  const localizedDescriptionKey = getPresetQuestionCopyKey(question.id, 'description')
  const localizedPromptKey = getPresetQuestionCopyKey(question.id, 'prompt')

  const title =
    localizedTitleKey && hasTranslation(localizedTitleKey)
      ? translate(localizedTitleKey)
      : question.title
  const description =
    localizedDescriptionKey && hasTranslation(localizedDescriptionKey)
      ? translate(localizedDescriptionKey)
      : question.description
  const prompt =
    localizedPromptKey && hasTranslation(localizedPromptKey)
      ? translate(localizedPromptKey)
      : question.prompt || question.text

  return {
    ...question,
    title,
    description,
    prompt,
    text: prompt || question.text,
  }
}
