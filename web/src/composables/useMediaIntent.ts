import type { MediaCategory, MediaIntent } from '@/api/media'

interface LangKeywords {
  actions: string[]
  imageNouns: string[]
  videoNouns: string[]
  editVerbs: string[]
  animVerbs: string[]
  negations: string[]
  questions: string[]
  metaCues?: string[]
  metaStrong?: string[]
}

const allLangKeywords: Record<string, LangKeywords> = {
  en: {
    actions: [
      'generate',
      'create',
      'draw',
      'paint',
      'make',
      'produce',
      'render',
      'design',
      'sketch',
    ],
    imageNouns: [
      'image',
      'picture',
      'photo',
      'photograph',
      'illustration',
      'artwork',
      'poster',
      'wallpaper',
      'portrait',
      'icon',
      'logo',
      'banner',
    ],
    videoNouns: ['video', 'animation', 'clip', 'movie', 'film', 'motion'],
    editVerbs: [
      'edit',
      'change',
      'modify',
      'transform',
      'alter',
      'adjust',
      'fix',
      'retouch',
      'enhance',
      'upscale',
      'remove',
      'replace',
      'restyle',
      'photoshop',
      'touch up',
      'touchup',
      'crop',
      'inpaint',
      'outpaint',
      'colorize',
      'sharpen',
      'denoise',
      'swap face',
      'restore',
      'cleanup',
      'clean up',
    ],
    animVerbs: ['animate', 'move', 'come alive', 'bring to life', 'make it move'],
    negations: ["don't", 'do not', 'stop', 'cancel', 'no more', 'not'],
    questions: ['how to', 'how do', 'what is', 'can you explain', 'tell me about', 'what does'],
    metaCues: [
      'intent',
      'keyword',
      'classifier',
      'classification',
      'matching',
      'route',
      'routing',
      'rule',
      'density',
      'trigger',
      'false positive',
    ],
    metaStrong: ['keyword matching', 'intent classification', 'intent classifier', 'not this intent'],
  },
  zh: {
    actions: ['生成', '画', '绘', '制作', '创建', '做', '弄', '设计', '绘制'],
    imageNouns: [
      '图',
      '图片',
      '照片',
      '图像',
      '插画',
      '海报',
      '壁纸',
      '头像',
      '图标',
      'logo',
      '画',
    ],
    videoNouns: ['视频', '动画', '影片', '短片', '动图', '视频片段'],
    editVerbs: [
      '编辑',
      '修改',
      '改',
      '变',
      '调整',
      '修复',
      '美化',
      '去除',
      '替换',
      '换',
      '修图',
      '改图',
      'p图',
      '抠图',
      '磨皮',
      '滤镜',
      '换脸',
      '换背景',
      '去水印',
      '美颜',
      '瘦脸',
      '祛痘',
      '补光',
    ],
    animVerbs: ['动起来', '动画化', '让它动', '变成视频'],
    negations: ['不要', '别', '停止', '取消', '不用'],
    questions: ['怎么', '如何', '什么是', '能不能解释', '介绍一下'],
    metaCues: ['意图', '关键词', '匹配', '命中', '分类', '分类器', '规则', '触发', '密度', '误判'],
    metaStrong: ['不是这个意图', '关键词匹配', '意图分类', '意图识别'],
  },
  ja: {
    actions: ['生成', '描く', '作る', '作成', '描いて', '書いて', '作って', 'デザイン'],
    imageNouns: ['画像', '写真', 'イラスト', '絵', 'ポスター', '壁紙', 'アイコン'],
    videoNouns: ['動画', 'ビデオ', '映像', 'アニメ', 'ムービー', 'クリップ'],
    editVerbs: [
      '編集',
      '変更',
      '修正',
      '加工',
      '調整',
      '変えて',
      '直して',
      'レタッチ',
      '修整',
      '補正',
      '切り抜き',
      '合成',
    ],
    animVerbs: ['アニメーション', '動かして', '動かす', '動画にして'],
    negations: ['しないで', 'やめて', '描かないで', '作らないで'],
    questions: ['どうやって', '方法', 'とは', 'について', '作り方'],
  },
  ko: {
    actions: ['생성', '그려', '만들', '그리다', '제작', '디자인', '만들어'],
    imageNouns: ['이미지', '사진', '그림', '일러스트', '포스터', '배경화면', '아이콘'],
    videoNouns: ['영상', '동영상', '비디오', '애니메이션', '클립', '무비'],
    editVerbs: [
      '편집',
      '수정',
      '변경',
      '바꿔',
      '고쳐',
      '조정',
      '보정',
      '리터치',
      '합성',
      '배경제거',
    ],
    animVerbs: ['애니메이션', '움직여', '동영상으로'],
    negations: ['하지마', '안해', '그만', '취소'],
    questions: ['어떻게', '방법', '뭐야', '설명해'],
  },
}

function localeToLangKey(locale: string): string {
  if (!locale) return 'en'
  const lang = locale.toLowerCase().split('-')[0] || 'en'
  if (lang === 'ca') return 'es'
  if (lang === 'ga') return 'en'
  return allLangKeywords[lang] ? lang : 'en'
}

function isASCII(s: string): boolean {
  for (let i = 0; i < s.length; i++) {
    if (s.charCodeAt(i) > 127) return false
  }
  return true
}

function isWordMatch(text: string, sub: string, idx: number): boolean {
  if (idx > 0) {
    const before = text[idx - 1]
    if (before && /\p{L}/u.test(before)) return false
  }
  const end = idx + sub.length
  if (end < text.length) {
    const after = text[end]
    if (after && /\p{L}/u.test(after)) return false
  }
  return true
}

function containsAny(text: string, substrs: string[]): boolean {
  for (const s of substrs) {
    const idx = text.indexOf(s)
    if (idx < 0) continue
    if (s.length <= 4 && isASCII(s)) {
      if (isWordMatch(text, s, idx)) return true
      continue
    }
    return true
  }
  return false
}

function isNegated(text: string, kw: LangKeywords): boolean {
  for (const neg of kw.negations) {
    const negIdx = text.indexOf(neg)
    if (negIdx < 0) continue
    if (negIdx < 10) return true
    const after = text.slice(negIdx + neg.length).trimStart()
    for (const act of kw.actions) {
      if (after.startsWith(act)) return true
    }
  }
  return false
}

function isQuestion(text: string, kw: LangKeywords): boolean {
  for (const q of kw.questions) {
    if (text.startsWith(q)) return true
  }
  return false
}

function countCueHits(text: string, cues: string[] = []): number {
  let hits = 0
  for (const cue of cues) {
    if (cue && containsAny(text, [cue])) hits++
  }
  return hits
}

function isMetaDiscussion(text: string, ...kws: Array<LangKeywords | undefined>): boolean {
  let metaCueHits = 0
  for (const kw of kws) {
    if (!kw) continue
    if (containsAny(text, kw.metaStrong ?? [])) return true
    metaCueHits += countCueHits(text, kw.metaCues)
    if (metaCueHits >= 2) return true
  }
  return false
}

interface CueMatch {
  start: number
  end: number
}

interface SegmentIntentCandidate {
  category: MediaCategory
  confidence: number
  prompt: string
  focusSpan: number
  alternativeCategory?: MediaCategory
}

function isIntentBoundary(ch: string): boolean {
  return /[\n\r\t,，。!！?？;；:：、】【\[\]{}()<>|、]/u.test(ch)
}

function splitIntentSegments(message: string): string[] {
  const trimmed = message.trim()
  if (!trimmed) return []

  const segments = [trimmed]
  const seen = new Set<string>(segments)
  let start = 0
  for (let i = 0; i < trimmed.length; i++) {
    if (!isIntentBoundary(trimmed[i] ?? '')) continue
    const segment = trimmed.slice(start, i).trim()
    if (segment && !seen.has(segment)) {
      seen.add(segment)
      segments.push(segment)
    }
    start = i + 1
  }
  const tail = trimmed.slice(start).trim()
  if (tail && !seen.has(tail)) {
    segments.push(tail)
  }
  return segments
}

function findCueMatches(text: string, cues: string[]): CueMatch[] {
  const textRunes = Array.from(text)
  const matches: CueMatch[] = []

  for (const cue of cues) {
    if (!cue) continue
    const cueRunes = Array.from(cue)
    if (!cueRunes.length || cueRunes.length > textRunes.length) continue
    const requireWordBoundary = cue.length <= 4 && isASCII(cue)

    for (let i = 0; i <= textRunes.length - cueRunes.length; i++) {
      let matched = true
      for (let j = 0; j < cueRunes.length; j++) {
        if (textRunes[i + j] !== cueRunes[j]) {
          matched = false
          break
        }
      }
      if (!matched) continue
      if (requireWordBoundary && !isRuneWordMatch(textRunes, i, cueRunes.length)) {
        continue
      }
      matches.push({ start: i, end: i + cueRunes.length })
    }
  }

  return matches
}

function isRuneWordMatch(text: string[], start: number, length: number): boolean {
  const before = text[start - 1]
  if (before && /\p{L}/u.test(before)) return false
  const after = text[start + length]
  if (after && /\p{L}/u.test(after)) return false
  return true
}

function minSingleSpan(matches: CueMatch[]): number {
  let best = 0
  for (const match of matches) {
    const span = match.end - match.start
    if (span <= 0) continue
    if (best === 0 || span < best) best = span
  }
  return best
}

function minSpanBetween(left: CueMatch[], right: CueMatch[]): number {
  let best = 0
  for (const l of left) {
    for (const r of right) {
      const start = Math.min(l.start, r.start)
      const end = Math.max(l.end, r.end)
      const span = end - start
      if (span <= 0) continue
      if (best === 0 || span < best) best = span
    }
  }
  return best
}

function pickMinPositive(...values: number[]): number {
  let best = 0
  for (const value of values) {
    if (value <= 0) continue
    if (best === 0 || value < best) best = value
  }
  return best
}

function pickMaxPositive(...values: number[]): number {
  let best = 0
  for (const value of values) {
    if (value <= 0) continue
    if (value > best) best = value
  }
  return best
}

function applyFocusPenalty(
  confidence: number,
  focusSpan: number,
  totalRunes: number,
  hasImages: boolean
): number {
  if (confidence <= 0 || focusSpan <= 0 || totalRunes <= 0) return confidence
  if (totalRunes <= 16) return confidence

  const ratio = focusSpan / totalRunes
  if (!hasImages && totalRunes >= 72 && ratio < 0.1) return 0

  if (hasImages) {
    if (totalRunes >= 72 && ratio < 0.08) confidence -= 0.25
    else if (totalRunes >= 40 && ratio < 0.15) confidence -= 0.1
  } else {
    if (totalRunes >= 120 && ratio < 0.15) confidence -= 0.25
    else if (totalRunes >= 60 && ratio < 0.2) confidence -= 0.15
    else if (totalRunes >= 24 && ratio < 0.12) confidence -= 0.2
  }

  return Math.max(0, confidence)
}

function classifyMediaIntentSegment(
  segment: string,
  hasImages: boolean,
  imageCount: number,
  kw: LangKeywords,
  kwEN: LangKeywords,
  includeEnglishFallback: boolean
): SegmentIntentCandidate | null {
  const lower = segment.toLowerCase().trim()
  if (!lower) return null

  if (isNegated(lower, kw) || isNegated(lower, kwEN)) return null
  if (isQuestion(lower, kw) || isQuestion(lower, kwEN)) return null
  if (isMetaDiscussion(lower, kw, kwEN)) return null

  let actionMatches = findCueMatches(lower, kw.actions)
  let imageMatches = findCueMatches(lower, kw.imageNouns)
  let videoMatches = findCueMatches(lower, kw.videoNouns)
  let editMatches = findCueMatches(lower, kw.editVerbs)
  let animMatches = findCueMatches(lower, kw.animVerbs)

  if (includeEnglishFallback) {
    actionMatches = actionMatches.concat(findCueMatches(lower, kwEN.actions))
    imageMatches = imageMatches.concat(findCueMatches(lower, kwEN.imageNouns))
    videoMatches = videoMatches.concat(findCueMatches(lower, kwEN.videoNouns))
    editMatches = editMatches.concat(findCueMatches(lower, kwEN.editVerbs))
    animMatches = animMatches.concat(findCueMatches(lower, kwEN.animVerbs))
  }

  const hasAction = actionMatches.length > 0
  const hasImageNoun = imageMatches.length > 0
  const hasVideoNoun = videoMatches.length > 0
  const hasEditVerb = editMatches.length > 0
  const hasAnimVerb = animMatches.length > 0

  let category: MediaCategory | '' = ''
  let confidence = 0
  let focusSpan = 0
  let alternativeCategory: MediaCategory | undefined

  if (imageCount >= 2 && (hasVideoNoun || hasAnimVerb)) {
    category = 'i2v'
    confidence = 0.85
    focusSpan = pickMinPositive(minSingleSpan(videoMatches), minSingleSpan(animMatches))
    alternativeCategory = 'kf2v'
  } else if (hasImages && hasAnimVerb) {
    category = 'i2v'
    confidence = 0.9
    focusSpan = pickMaxPositive(minSingleSpan(animMatches), minSpanBetween(animMatches, imageMatches))
  } else if (hasImages && hasVideoNoun) {
    category = 'i2v'
    confidence = 0.85
    focusSpan = pickMaxPositive(minSingleSpan(videoMatches), minSpanBetween(videoMatches, imageMatches))
  } else if (hasImages && hasEditVerb) {
    category = 'i2i'
    confidence = 0.9
    focusSpan = pickMaxPositive(minSingleSpan(editMatches), minSpanBetween(editMatches, imageMatches))
  } else if (hasImages && hasAction && hasImageNoun) {
    category = 'i2i'
    confidence = 0.7
    focusSpan = minSpanBetween(actionMatches, imageMatches)
  } else if (hasAction && hasVideoNoun) {
    category = 't2v'
    confidence = 0.85
    focusSpan = minSpanBetween(actionMatches, videoMatches)
  } else if (hasVideoNoun && !hasAction) {
    category = 't2v'
    confidence = 0.6
    focusSpan = minSingleSpan(videoMatches)
  } else if (hasAction && hasImageNoun) {
    category = 't2i'
    confidence = 0.85
    focusSpan = minSpanBetween(actionMatches, imageMatches)
  } else if (hasImageNoun && !hasAction) {
    category = 't2i'
    confidence = 0.5
    focusSpan = minSingleSpan(imageMatches)
  }

  if (!category) return null
  if (focusSpan === 0) {
    focusSpan = Array.from(lower).length
  }

  return {
    category,
    confidence,
    prompt: segment.trim(),
    focusSpan,
    alternativeCategory,
  }
}

/**
 * Client-side media intent classifier — mirrors the Go ClassifyMediaIntent logic
 * for zero-latency classification in the browser.
 */
export function classifyMediaIntent(
  message: string,
  hasImages = false,
  imageCount = 0,
  locale = ''
): MediaIntent | null {
  if (!message) return null

  const trimmed = message.trim()
  if (!trimmed) return null

  const langKey = localeToLangKey(locale)
  const kw = (allLangKeywords[langKey] ?? allLangKeywords.en)!
  const kwEN = allLangKeywords.en!
  const totalRunes = Array.from(trimmed.toLowerCase()).length
  const segments = splitIntentSegments(trimmed)

  let best: SegmentIntentCandidate | null = null
  for (const segment of segments) {
    const candidate = classifyMediaIntentSegment(
      segment,
      hasImages,
      imageCount,
      kw,
      kwEN,
      langKey !== 'en'
    )
    if (!candidate) continue
    candidate.confidence = applyFocusPenalty(
      candidate.confidence,
      candidate.focusSpan,
      totalRunes,
      hasImages
    )
    if (candidate.confidence <= 0) continue
    if (
      !best ||
      candidate.confidence > best.confidence ||
      (candidate.confidence === best.confidence &&
        Array.from(candidate.prompt).length < Array.from(best.prompt).length)
    ) {
      best = candidate
    }
  }

  if (!best) return null

  return {
    category: best.category,
    confidence: best.confidence,
    prompt: best.prompt,
    has_image: hasImages,
    image_count: imageCount,
    alternative_category: best.alternativeCategory,
  }
}
