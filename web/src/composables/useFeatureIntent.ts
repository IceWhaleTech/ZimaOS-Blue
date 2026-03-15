import { featureIntentTerms } from './featureIntentTerms.generated'

export interface FeatureIntentHint {
  deepResearch: boolean
  agentMode: boolean
}

function isAsciiToken(token: string): boolean {
  for (let i = 0; i < token.length; i++) {
    if (token.charCodeAt(i) > 127) return false
  }
  return true
}

function isAsciiWordChar(ch: string): boolean {
  if (!ch) return false
  const code = ch.charCodeAt(0)
  return (
    (code >= 48 && code <= 57) ||
    (code >= 65 && code <= 90) ||
    (code >= 97 && code <= 122) ||
    code === 95
  )
}

function containsToken(text: string, token: string): boolean {
  if (!token) return false
  const asciiToken = isAsciiToken(token)
  let from = 0

  while (from <= text.length) {
    const idx = text.indexOf(token, from)
    if (idx < 0) return false

    if (!asciiToken) return true

    const before = idx > 0 ? text.slice(idx - 1, idx) : ''
    const afterPos = idx + token.length
    const after = afterPos < text.length ? text.slice(afterPos, afterPos + 1) : ''
    if (!isAsciiWordChar(before) && !isAsciiWordChar(after)) {
      return true
    }

    from = idx + 1
  }
  return false
}

function containsAny(text: string, tokens: string[]): boolean {
  return tokens.some((token) => containsToken(text, token))
}

function isDefinitionQuestion(text: string): boolean {
  const asksDefinition = featureIntentTerms.definitionPrefixes.some((prefix) =>
    text.startsWith(prefix)
  )
  if (!asksDefinition) return false
  return (
    containsAny(text, featureIntentTerms.deepResearchExplicit) ||
    containsAny(text, featureIntentTerms.agentModeExplicit)
  )
}

export function classifyFeatureIntent(message: string): FeatureIntentHint {
  const text = message.toLowerCase().trim()
  if (!text) return { deepResearch: false, agentMode: false }
  if (isDefinitionQuestion(text)) return { deepResearch: false, agentMode: false }

  const deepExplicit = containsAny(text, featureIntentTerms.deepResearchExplicit)
  const deepComposite =
    containsAny(text, featureIntentTerms.deepResearchActions) &&
    containsAny(text, featureIntentTerms.deepResearchTargets)
  const deepNegated = containsAny(text, featureIntentTerms.deepResearchNegations)
  const deepResearch = (deepExplicit || deepComposite) && !deepNegated

  const agentExplicit = containsAny(text, featureIntentTerms.agentModeExplicit)
  const agentComposite =
    containsAny(text, featureIntentTerms.agentModeActions) &&
    containsAny(text, featureIntentTerms.agentModeTargets)
  const agentNegated = containsAny(text, featureIntentTerms.agentModeNegations)
  const agentMode = (agentExplicit || agentComposite) && !agentNegated

  return { deepResearch, agentMode }
}
