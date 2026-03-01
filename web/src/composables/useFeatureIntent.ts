export interface FeatureIntentHint {
  deepResearch: boolean
  agentMode: boolean
}

const DEEP_RESEARCH_EXPLICIT = [
  'deep research',
  '深度搜索',
  '深度研究',
  '深入研究',
  '深度调研',
  '深入调研',
]

const DEEP_RESEARCH_ACTIONS = [
  'deep dive',
  'in-depth',
  'in depth',
  'comprehensive research',
  'research thoroughly',
  '详细调研',
  '全面调研',
  '深入分析',
  '全面分析',
]

const DEEP_RESEARCH_TARGETS = [
  '资料',
  '来源',
  '引用',
  '证据',
  'sources',
  'citations',
  'evidence',
  'references',
]

const AGENT_MODE_EXPLICIT = [
  'agent mode',
  'agent loop',
  'agent loop mode',
  'agent mode loop',
  '智能体模式',
  '智能体循环',
  '循环智能体',
  '代理模式',
  '自动代理',
  '自主代理',
]

const AGENT_MODE_ACTIONS = [
  'autonomous',
  'plan and execute',
  'multi-step',
  '自动执行',
  '自主执行',
  '自己完成',
  '自动完成',
  '分步执行',
  '端到端执行',
]

const AGENT_MODE_TARGETS = [
  'task',
  'tasks',
  'workflow',
  '步骤',
  '任务',
  '流程',
  '命令',
]

const DEEP_RESEARCH_NEGATIONS = [
  'no deep research',
  'without deep research',
  'disable deep research',
  '不要深度搜索',
  '不用深度搜索',
  '关闭深度搜索',
]

const AGENT_MODE_NEGATIONS = [
  'no agent mode',
  'no agent loop',
  'without agent mode',
  'without agent loop',
  'disable agent mode',
  'disable agent loop',
  '不要 agent mode',
  '不要 agent loop',
  '不要智能体模式',
  '不要智能体循环',
  '关闭agent loop',
  '关闭agent mode',
  '关闭智能体模式',
  '不要自动执行',
]

const QUESTION_PREFIXES = [
  'what is',
  "what's",
  'how to',
  'how do i',
  '什么是',
  '啥是',
  '怎么用',
  '如何使用',
  '介绍一下',
  '解释一下',
]

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
  return tokens.some(token => containsToken(text, token))
}

function isDefinitionQuestion(text: string): boolean {
  const asksDefinition = QUESTION_PREFIXES.some(prefix => text.startsWith(prefix))
  if (!asksDefinition) return false
  return containsAny(text, DEEP_RESEARCH_EXPLICIT) || containsAny(text, AGENT_MODE_EXPLICIT)
}

export function classifyFeatureIntent(message: string): FeatureIntentHint {
  const text = message.toLowerCase().trim()
  if (!text) return { deepResearch: false, agentMode: false }
  if (isDefinitionQuestion(text)) return { deepResearch: false, agentMode: false }

  const deepExplicit = containsAny(text, DEEP_RESEARCH_EXPLICIT)
  const deepComposite = containsAny(text, DEEP_RESEARCH_ACTIONS) && containsAny(text, DEEP_RESEARCH_TARGETS)
  const deepNegated = containsAny(text, DEEP_RESEARCH_NEGATIONS)
  const deepResearch = (deepExplicit || deepComposite) && !deepNegated

  const agentExplicit = containsAny(text, AGENT_MODE_EXPLICIT)
  const agentComposite = containsAny(text, AGENT_MODE_ACTIONS) && containsAny(text, AGENT_MODE_TARGETS)
  const agentNegated = containsAny(text, AGENT_MODE_NEGATIONS)
  const agentMode = (agentExplicit || agentComposite) && !agentNegated

  return { deepResearch, agentMode }
}
