import type { MediaCategory, MediaIntent } from '@/api/media'

interface LangKeywords {
  actions: string[]
  imageNouns: string[]
  videoNouns: string[]
  editVerbs: string[]
  animVerbs: string[]
  negations: string[]
  questions: string[]
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

  const lower = message.toLowerCase().trim()
  const langKey = localeToLangKey(locale)
  const kw = (allLangKeywords[langKey] ?? allLangKeywords.en)!
  const kwEN = allLangKeywords.en!

  // Layer 0: Negation and question filters
  if (isNegated(lower, kw) || isNegated(lower, kwEN)) return null
  if (isQuestion(lower, kw) || isQuestion(lower, kwEN)) return null

  // Layer 1: Keyword matching
  let hasAction = containsAny(lower, kw.actions)
  let hasImageNoun = containsAny(lower, kw.imageNouns)
  let hasVideoNoun = containsAny(lower, kw.videoNouns)
  let hasEditVerb = containsAny(lower, kw.editVerbs)
  let hasAnimVerb = containsAny(lower, kw.animVerbs)

  if (langKey !== 'en') {
    if (!hasAction) hasAction = containsAny(lower, kwEN.actions)
    if (!hasImageNoun) hasImageNoun = containsAny(lower, kwEN.imageNouns)
    if (!hasVideoNoun) hasVideoNoun = containsAny(lower, kwEN.videoNouns)
    if (!hasEditVerb) hasEditVerb = containsAny(lower, kwEN.editVerbs)
    if (!hasAnimVerb) hasAnimVerb = containsAny(lower, kwEN.animVerbs)
  }

  // Layer 2: Category determination
  let category: MediaCategory | '' = ''
  let confidence = 0

  let alternativeCategory: MediaCategory | undefined

  if (imageCount >= 2 && (hasVideoNoun || hasAnimVerb)) {
    category = 'i2v'
    confidence = 0.85
    alternativeCategory = 'kf2v'
  } else if (hasImages && hasAnimVerb) {
    category = 'i2v'
    confidence = 0.9
  } else if (hasImages && hasVideoNoun) {
    category = 'i2v'
    confidence = 0.85
  } else if (hasImages && hasEditVerb) {
    category = 'i2i'
    confidence = 0.9
  } else if (hasImages && hasAction && hasImageNoun) {
    category = 'i2i'
    confidence = 0.7
  } else if (hasAction && hasVideoNoun) {
    category = 't2v'
    confidence = 0.85
  } else if (hasVideoNoun && !hasAction) {
    category = 't2v'
    confidence = 0.6
  } else if (hasAction && hasImageNoun) {
    category = 't2i'
    confidence = 0.85
  } else if (hasImageNoun && !hasAction) {
    category = 't2i'
    confidence = 0.5
  }

  if (!category) return null

  return {
    category,
    confidence,
    prompt: message,
    has_image: hasImages,
    image_count: imageCount,
    alternative_category: alternativeCategory,
  }
}
