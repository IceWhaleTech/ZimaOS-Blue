export const smallModelFallbackReasonCodes = [
  'model_unready',
  'timeout',
  'circuit_open',
  'low_confidence',
  'schema_invalid',
  'resource_guard',
  'deepresearch_unavailable',
  'ir_no_signal',
  'auto_rollback_fallback_rate',
  'auto_rollback_summary_fallback_rate',
  'auto_rollback_doc_extract_fallback_rate',
] as const

export type SmallModelFallbackReasonCode = (typeof smallModelFallbackReasonCodes)[number]

type TranslateFn = (key: string) => string
type TranslationExistsFn = (key: string) => boolean

export function getSmallModelFallbackReasonLabelKey(reason: string): string {
  return `settings.smallModel.fallbackReasonLabels.${reason}`
}

export function formatSmallModelFallbackReason(
  reason: string,
  t: TranslateFn,
  te?: TranslationExistsFn
): string {
  const key = getSmallModelFallbackReasonLabelKey(reason)
  if (te?.(key)) {
    return t(key)
  }
  return reason
}
