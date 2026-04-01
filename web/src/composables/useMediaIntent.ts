import type { MediaCategory, MediaIntent } from '@/api/media'
import { localeKeys, type LocaleKey } from '@/i18n/locale-catalog'

type SupportedLocaleKey = LocaleKey

type KeywordGroup =
  | 'action'
  | 'imageNoun'
  | 'slideNoun'
  | 'videoNoun'
  | 'editVerb'
  | 'animVerb'
  | 'negation'
  | 'question'
  | 'metaCue'
  | 'metaStrong'
  | 'nanoPreset'
  | 'bananaPreset'

type KeywordSet = Record<KeywordGroup, string[]>

interface CueMatch {
  start: number
  end: number
  cue: string
}

interface GroupedCueMatch extends CueMatch {
  group: KeywordGroup
}

interface SegmentIntentCandidate {
  category: MediaCategory
  confidence: number
  prompt: string
  focusSpan: number
  alternativeCategory?: MediaCategory
  params?: Record<string, any>
}

interface KeywordPattern {
  cue: string
  group: KeywordGroup
  requireWordBoundary: boolean
  length: number
}

interface AutomatonNode {
  children: Map<string, number>
  fail: number
  outputs: number[]
}

interface LocaleMatcherProfile {
  locale: SupportedLocaleKey
  keywords: KeywordSet
  matcher: UnicodeAhoMatcher
}

const BRAND_KEYWORDS = {
  nanoPreset: ['nanoslides', 'nano slides', 'nano-slides', 'nano_slides'],
  bananaPreset: ['bananaslides', 'banana slides', 'banana-slides', 'banana_slides'],
} as const

const KEYWORD_GROUPS: readonly KeywordGroup[] = [
  'action',
  'imageNoun',
  'slideNoun',
  'videoNoun',
  'editVerb',
  'animVerb',
  'negation',
  'question',
  'metaCue',
  'metaStrong',
  'nanoPreset',
  'bananaPreset',
] as const

export const MEDIA_INTENT_SUPPORTED_LOCALES = localeKeys satisfies readonly SupportedLocaleKey[]

const SUPPORTED_LOCALE_LOOKUP = Object.freeze(
  Object.fromEntries(MEDIA_INTENT_SUPPORTED_LOCALES.map((locale) => [locale.toLowerCase(), locale]))
) as Readonly<Record<string, SupportedLocaleKey>>

const BASE_LOCALE_FALLBACK: Readonly<Record<string, SupportedLocaleKey>> = {
  ca: 'ca-ES',
  cs: 'cs-CZ',
  da: 'da-DK',
  de: 'de-DE',
  el: 'el-GR',
  en: 'en-US',
  es: 'es-ES',
  fr: 'fr-FR',
  ga: 'ga-IE',
  hr: 'hr-HR',
  hu: 'hu-HU',
  it: 'it-IT',
  ja: 'ja-JP',
  ko: 'ko-KR',
  ml: 'ml-IN',
  nb: 'nb-NO',
  nl: 'nl-NL',
  no: 'nb-NO',
  pl: 'pl-PL',
  pt: 'pt-BR',
  ro: 'ro-RO',
  ru: 'ru-RU',
  sk: 'sk-SK',
  sv: 'sv-SE',
  zh: 'zh-CN',
} as const

const EXTRA_LOCALE_ALIASES: Readonly<Record<string, SupportedLocaleKey>> = {
  'en-gb': 'en-GB',
  'en-us': 'en-US',
  'pt-br': 'pt-BR',
  'pt-pt': 'pt-PT',
  'zh-cn': 'zh-CN',
  'zh-tw': 'zh-TW',
  'zh-hk': 'zh-TW',
} as const

function normalizeMatcherText(text: string): string {
  return text.normalize('NFKC').toLocaleLowerCase().trim()
}

function dedupeKeywords(values: string[]): string[] {
  const seen = new Set<string>()
  const deduped: string[] = []
  for (const value of values) {
    const normalized = normalizeMatcherText(value)
    if (!normalized || seen.has(normalized)) continue
    seen.add(normalized)
    deduped.push(normalized)
  }
  return deduped
}

function createKeywordSet(spec: Partial<KeywordSet>): KeywordSet {
  return {
    action: dedupeKeywords(spec.action ?? []),
    imageNoun: dedupeKeywords(spec.imageNoun ?? []),
    slideNoun: dedupeKeywords(spec.slideNoun ?? []),
    videoNoun: dedupeKeywords(spec.videoNoun ?? []),
    editVerb: dedupeKeywords(spec.editVerb ?? []),
    animVerb: dedupeKeywords(spec.animVerb ?? []),
    negation: dedupeKeywords(spec.negation ?? []),
    question: dedupeKeywords(spec.question ?? []),
    metaCue: dedupeKeywords(spec.metaCue ?? []),
    metaStrong: dedupeKeywords(spec.metaStrong ?? []),
    nanoPreset: dedupeKeywords(spec.nanoPreset ?? [...BRAND_KEYWORDS.nanoPreset]),
    bananaPreset: dedupeKeywords(spec.bananaPreset ?? [...BRAND_KEYWORDS.bananaPreset]),
  }
}

function mergeKeywordSets(primary: KeywordSet, fallback: KeywordSet): KeywordSet {
  const merged = {} as KeywordSet
  for (const group of KEYWORD_GROUPS) {
    merged[group] = dedupeKeywords([...(primary[group] ?? []), ...(fallback[group] ?? [])])
  }
  return merged
}

const ENGLISH_KEYWORDS = createKeywordSet({
  action: ['generate', 'create', 'draw', 'paint', 'make', 'produce', 'render', 'design'],
  imageNoun: [
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
  slideNoun: ['slide', 'slides', 'deck', 'presentation', 'ppt', 'pptx', 'keynote'],
  videoNoun: ['video', 'animation', 'clip', 'movie', 'film', 'motion'],
  editVerb: [
    'edit',
    'change',
    'modify',
    'transform',
    'alter',
    'adjust',
    'fix',
    'retouch',
    'enhance',
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
    'restore',
    'cleanup',
    'clean up',
  ],
  animVerb: ['animate', 'move', 'come alive', 'bring to life', 'make it move'],
  negation: ["don't", 'do not', 'stop', 'cancel', 'no more', 'not'],
  question: ['how to', 'how do', 'what is', 'can you explain', 'tell me about', 'what does'],
  metaCue: [
    'intent',
    'keyword',
    'classifier',
    'classification',
    'matching',
    'route',
    'routing',
    'rule',
    'trigger',
    'false positive',
  ],
  metaStrong: [
    'keyword matching',
    'intent classification',
    'intent classifier',
    'not this intent',
  ],
})

const CATALAN_KEYWORDS = createKeywordSet({
  action: ['genera', 'crea', 'dibuixa', 'fes', 'produeix', 'dissenya'],
  imageNoun: [
    'imatge',
    'foto',
    'fotografia',
    'il·lustració',
    'cartell',
    'fons',
    'retrat',
    'icona',
    'logotip',
  ],
  slideNoun: ['diapositiva', 'diapositives', 'presentació', 'ppt', 'pptx', 'keynote'],
  videoNoun: ['vídeo', 'animació', 'clip', 'pel·lícula', 'moviment'],
  editVerb: ['edita', 'canvia', 'modifica', 'transforma', 'ajusta', 'retoca', 'substitueix'],
  animVerb: ['anima', 'fes que es mogui', 'dona-li vida', 'converteix-ho en vídeo'],
  negation: ['no', 'atura', 'cancel·la'],
  question: ['com', 'què és', 'pots explicar'],
  metaCue: ['intenció', 'paraula clau', 'coincidència', 'classificació', 'regla', 'activador'],
  metaStrong: ['coincidència de paraules clau', 'classificació d’intencions', 'no és aquesta intenció'],
})

const CZECH_KEYWORDS = createKeywordSet({
  action: ['vygeneruj', 'vytvoř', 'nakresli', 'udělej', 'navrhni'],
  imageNoun: ['obrázek', 'obrazek', 'fotku', 'fotografie', 'ilustraci', 'plakát', 'ikonu', 'logo'],
  slideNoun: ['snímek', 'snímek', 'slidy', 'prezentaci', 'prezentace', 'ppt', 'pptx'],
  videoNoun: ['video', 'animaci', 'animace', 'klip', 'film'],
  editVerb: ['uprav', 'změň', 'modifikuj', 'retušuj', 'nahraď', 'odstraň'],
  animVerb: ['animuj', 'rozpohybuj', 'udělej z toho video'],
  negation: ['ne', 'zastav', 'zruš'],
  question: ['jak', 'co je', 'můžeš vysvětlit'],
  metaCue: ['záměr', 'klíčové slovo', 'shoda', 'klasifikace', 'pravidlo', 'spouštěč'],
  metaStrong: ['shoda klíčových slov', 'klasifikace záměru', 'není to tento záměr'],
})

const DANISH_KEYWORDS = createKeywordSet({
  action: ['generer', 'lav', 'skab', 'tegn', 'design'],
  imageNoun: ['billede', 'foto', 'fotografi', 'illustration', 'plakat', 'ikon', 'logo'],
  slideNoun: ['slide', 'slides', 'præsentation', 'dæk', 'ppt', 'pptx'],
  videoNoun: ['video', 'animation', 'klip', 'film'],
  editVerb: ['rediger', 'ændr', 'modificer', 'retoucher', 'erstat', 'fjern'],
  animVerb: ['animér', 'få det til at bevæge sig', 'lav det til en video'],
  negation: ['ikke', 'stop', 'annuller'],
  question: ['hvordan', 'hvad er', 'kan du forklare'],
  metaCue: ['hensigt', 'nøgleord', 'match', 'klassifikation', 'regel', 'udløser'],
  metaStrong: ['søgeordsmatch', 'intentklassifikation', 'ikke denne hensigt'],
})

const GERMAN_KEYWORDS = createKeywordSet({
  action: ['generiere', 'erstelle', 'zeichne', 'mache', 'entwirf'],
  imageNoun: ['bild', 'foto', 'fotografie', 'illustration', 'poster', 'symbol', 'logo', 'banner'],
  slideNoun: ['folie', 'folien', 'präsentation', 'praesentation', 'deck', 'ppt', 'pptx'],
  videoNoun: ['video', 'animation', 'clip', 'film'],
  editVerb: ['bearbeite', 'ändere', 'aendere', 'modifiziere', 'retuschiere', 'ersetze', 'entferne'],
  animVerb: ['animiere', 'beweg es', 'mach daraus ein video'],
  negation: ['nicht', 'stopp', 'abbrechen'],
  question: ['wie', 'was ist', 'kannst du erklären'],
  metaCue: ['absicht', 'schlüsselwort', 'schluesselwort', 'treffer', 'klassifizierung', 'regel'],
  metaStrong: ['schlüsselwortabgleich', 'intent-klassifizierung', 'nicht diese absicht'],
})

const GREEK_KEYWORDS = createKeywordSet({
  action: ['δημιούργησε', 'φτιάξε', 'σχεδίασε', 'παρήγαγε'],
  imageNoun: ['εικόνα', 'φωτογραφία', 'εικονογράφηση', 'πόστερ', 'εικονίδιο', 'λογότυπο'],
  slideNoun: ['διαφάνεια', 'διαφάνειες', 'παρουσίαση', 'ppt', 'pptx'],
  videoNoun: ['βίντεο', 'κινούμενο σχέδιο', 'κλιπ', 'ταινία'],
  editVerb: ['επεξεργάσου', 'άλλαξε', 'τροποποίησε', 'ρετούσαρε', 'αντικατέστησε', 'αφαίρεσε'],
  animVerb: ['κάνε το animation', 'κάνε το να κινηθεί', 'κάνε το βίντεο'],
  negation: ['μη', 'σταμάτα', 'ακύρωσε'],
  question: ['πώς', 'τι είναι', 'μπορείς να εξηγήσεις'],
  metaCue: ['πρόθεση', 'λέξη κλειδί', 'αντιστοίχιση', 'ταξινόμηση', 'κανόνας'],
  metaStrong: ['αντιστοίχιση λέξεων κλειδιών', 'ταξινόμηση πρόθεσης', 'όχι αυτή η πρόθεση'],
})

const SPANISH_KEYWORDS = createKeywordSet({
  action: ['genera', 'crea', 'dibuja', 'haz', 'produce', 'diseña'],
  imageNoun: ['imagen', 'foto', 'fotografía', 'ilustración', 'cartel', 'icono', 'logo', 'banner'],
  slideNoun: ['diapositiva', 'diapositivas', 'presentación', 'presentacion', 'ppt', 'pptx'],
  videoNoun: ['video', 'animación', 'animacion', 'clip', 'película'],
  editVerb: ['edita', 'cambia', 'modifica', 'transforma', 'retoca', 'reemplaza', 'elimina'],
  animVerb: ['anima', 'haz que se mueva', 'conviértelo en video', 'dale vida'],
  negation: ['no', 'detén', 'deten', 'cancela'],
  question: ['cómo', 'como', 'qué es', 'que es', 'puedes explicar'],
  metaCue: ['intención', 'intencion', 'palabra clave', 'coincidencia', 'clasificación', 'regla'],
  metaStrong: ['coincidencia de palabras clave', 'clasificación de intención', 'no es esta intención'],
})

const FRENCH_KEYWORDS = createKeywordSet({
  action: ['génère', 'genere', 'crée', 'cree', 'dessine', 'fais', 'produis', 'conçois'],
  imageNoun: ['image', 'photo', 'photographie', 'illustration', 'affiche', 'icône', 'icone', 'logo'],
  slideNoun: ['diapositive', 'diapositives', 'présentation', 'presentation', 'ppt', 'pptx'],
  videoNoun: ['vidéo', 'video', 'animation', 'clip', 'film'],
  editVerb: ['édite', 'edite', 'change', 'modifie', 'retouche', 'remplace', 'supprime'],
  animVerb: ['anime', 'fais-le bouger', 'transforme-le en vidéo'],
  negation: ['ne', 'pas', 'arrête', 'arrete', 'annule'],
  question: ['comment', 'qu’est-ce que', "qu'est-ce que", 'peux-tu expliquer'],
  metaCue: ['intention', 'mot-clé', 'mot cle', 'correspondance', 'classification', 'règle'],
  metaStrong: ['correspondance de mots-clés', 'classification d’intention', 'pas cette intention'],
})

const IRISH_KEYWORDS = createKeywordSet({
  action: ['gin', 'cruthaigh', 'tarraing', 'dean', 'déan'],
  imageNoun: ['íomhá', 'iomha', 'pictiúr', 'grianghraf', 'léaráid', 'logo'],
  slideNoun: ['sleamhnán', 'sleamhnain', 'cur i láthair', 'cur i lathair', 'ppt', 'pptx'],
  videoNoun: ['físeán', 'fisean', 'beochan', 'gearrthóg', 'gearrthog'],
  editVerb: ['eagar', 'athraigh', 'modhnaigh', 'coigeartaigh', 'cuir ina ionad'],
  animVerb: ['beoigh', 'bog é', 'dean físeán de', 'déan físeán de'],
  negation: ['ná', 'na', 'stop', 'cealaigh'],
  question: ['conas', 'cad é', 'cad e', 'an féidir leat a mhíniú'],
  metaCue: ['rún', 'run', 'eochairfhocal', 'meaitseáil', 'rangú', 'riail'],
  metaStrong: ['meaitseáil eochairfhocal', 'rangú rúin', 'ní hé seo an rún'],
})

const CROATIAN_KEYWORDS = createKeywordSet({
  action: ['generiraj', 'stvori', 'nacrtaj', 'napravi', 'dizajniraj'],
  imageNoun: ['sliku', 'slika', 'fotografiju', 'fotografija', 'ilustraciju', 'poster', 'ikonu', 'logo'],
  slideNoun: ['slajd', 'slajdove', 'prezentaciju', 'prezentacija', 'ppt', 'pptx'],
  videoNoun: ['video', 'animaciju', 'animacija', 'isječak', 'isjecak', 'film'],
  editVerb: ['uredi', 'promijeni', 'izmijeni', 'retuširaj', 'zamijeni', 'ukloni'],
  animVerb: ['animiraj', 'pokreni', 'pretvori u video'],
  negation: ['ne', 'stani', 'otkaži', 'otkazi'],
  question: ['kako', 'što je', 'sto je', 'možeš objasniti'],
  metaCue: ['namjera', 'ključna riječ', 'kljucna rijec', 'podudaranje', 'klasifikacija', 'pravilo'],
  metaStrong: ['podudaranje ključnih riječi', 'klasifikacija namjere', 'nije ova namjera'],
})

const HUNGARIAN_KEYWORDS = createKeywordSet({
  action: ['generálj', 'generalj', 'hozz létre', 'hozz letre', 'rajzolj', 'készíts', 'keszits'],
  imageNoun: ['képet', 'kepet', 'kép', 'kep', 'fotót', 'fotot', 'illusztrációt', 'posztert', 'ikont', 'logót'],
  slideNoun: ['diát', 'diat', 'dia', 'prezentációt', 'prezentaciot', 'prezentáció', 'ppt', 'pptx'],
  videoNoun: ['videót', 'videot', 'videó', 'video', 'animációt', 'animaciot', 'klipet'],
  editVerb: ['szerkeszd', 'módosítsd', 'modositsd', 'változtasd', 'valtoztasd', 'retusáld', 'cseréld'],
  animVerb: ['animáld', 'mozgasd meg', 'csinálj belőle videót'],
  negation: ['ne', 'állj', 'allj', 'mégse', 'megse'],
  question: ['hogyan', 'mi az', 'el tudod magyarázni'],
  metaCue: ['szándék', 'szandek', 'kulcsszó', 'kulcsszo', 'egyezés', 'osztályozás', 'szabály'],
  metaStrong: ['kulcsszó-egyezés', 'szándékosztályozás', 'nem ez a szándék'],
})

const ITALIAN_KEYWORDS = createKeywordSet({
  action: ['genera', 'crea', 'disegna', 'fai', 'produci', 'progetta'],
  imageNoun: ['immagine', 'foto', 'fotografia', 'illustrazione', 'poster', 'icona', 'logo'],
  slideNoun: ['slide', 'diapositiva', 'diapositive', 'presentazione', 'ppt', 'pptx'],
  videoNoun: ['video', 'animazione', 'clip', 'film'],
  editVerb: ['modifica', 'cambia', 'trasforma', 'ritocca', 'sostituisci', 'rimuovi'],
  animVerb: ['anima', 'fallo muovere', 'trasformalo in video'],
  negation: ['non', 'ferma', 'annulla'],
  question: ['come', 'cos’è', "cos'e", 'puoi spiegare'],
  metaCue: ['intento', 'parola chiave', 'corrispondenza', 'classificazione', 'regola'],
  metaStrong: ['corrispondenza di parole chiave', 'classificazione dell’intento', 'non è questo intento'],
})

const JAPANESE_KEYWORDS = createKeywordSet({
  action: ['生成', '作る', '作成', '描く', '描いて', '作って', 'デザイン'],
  imageNoun: ['画像', '写真', 'イラスト', '絵', 'ポスター', '壁紙', 'アイコン', 'ロゴ'],
  slideNoun: ['スライド', 'プレゼン', 'プレゼンテーション', '資料', 'ppt', 'pptx'],
  videoNoun: ['動画', 'ビデオ', '映像', 'アニメ', 'ムービー', 'クリップ'],
  editVerb: ['編集', '変更', '修正', '加工', '調整', 'レタッチ', '置き換え', '削除'],
  animVerb: ['アニメーション', '動かして', '動かす', '動画にして'],
  negation: ['しないで', 'やめて', '停止', 'キャンセル'],
  question: ['どうやって', 'とは', 'について', '説明して'],
  metaCue: ['意図', 'キーワード', '一致', '分類', '分類器', 'ルール', '誤判定'],
  metaStrong: ['キーワード一致', '意図分類', 'この意図ではない'],
})

const KOREAN_KEYWORDS = createKeywordSet({
  action: ['생성', '그려', '만들', '그리다', '제작', '디자인', '만들어'],
  imageNoun: ['이미지', '사진', '그림', '일러스트', '포스터', '배경화면', '아이콘', '로고'],
  slideNoun: ['슬라이드', '프레젠테이션', '발표자료', '덱', 'ppt', 'pptx'],
  videoNoun: ['영상', '동영상', '비디오', '애니메이션', '클립', '무비'],
  editVerb: ['편집', '수정', '변경', '바꿔', '고쳐', '조정', '보정', '교체', '제거'],
  animVerb: ['애니메이션', '움직여', '움직이게', '동영상으로'],
  negation: ['하지마', '안해', '그만', '취소'],
  question: ['어떻게', '뭐야', '설명해'],
  metaCue: ['의도', '키워드', '매칭', '분류', '분류기', '규칙', '오탐'],
  metaStrong: ['키워드 매칭', '의도 분류', '이 의도가 아님'],
})

const MALAYALAM_KEYWORDS = createKeywordSet({
  action: ['സൃഷ്ടിക്കുക', 'ഉണ്ടാക്കുക', 'വരയ്ക്കുക', 'തയ്യാറാക്കുക', 'ഡിസൈൻ ചെയ്യുക'],
  imageNoun: ['ചിത്രം', 'ഇമേജ്', 'ഫോട്ടോ', 'ഇലസ്ട്രേഷൻ', 'പോസ്റ്റർ', 'ഐക്കൺ', 'ലോഗോ'],
  slideNoun: ['സ്ലൈഡ്', 'പ്രസന്റേഷൻ', 'അവതരണം', 'ppt', 'pptx'],
  videoNoun: ['വീഡിയോ', 'ആനിമേഷൻ', 'ക്ലിപ്പ്', 'മോഷൻ'],
  editVerb: ['എഡിറ്റ് ചെയ്യുക', 'മാറ്റുക', 'തിരുത്തുക', 'റീടച്ച് ചെയ്യുക', 'പകരംവയ്ക്കുക', 'നീക്കുക'],
  animVerb: ['ആനിമേറ്റ് ചെയ്യുക', 'ചലിപ്പിക്കുക', 'വീഡിയോയാക്കുക'],
  negation: ['വേണ്ട', 'നിർത്തുക', 'റദ്ദാക്കുക'],
  question: ['എങ്ങനെ', 'എന്താണ്', 'വിശദീകരിക്കുക'],
  metaCue: ['ഉദ്ദേശ്യം', 'കീവേഡ്', 'മാച്ചിംഗ്', 'വർഗ്ഗീകരണം', 'നിയമം'],
  metaStrong: ['കീവേഡ് മാച്ചിംഗ്', 'ഇന്റന്റ് വർഗ്ഗീകരണം', 'ഇത് ആ ഉദ്ദേശ്യമല്ല'],
})

const NORWEGIAN_KEYWORDS = createKeywordSet({
  action: ['generer', 'lag', 'skap', 'tegn', 'design'],
  imageNoun: ['bilde', 'foto', 'fotografi', 'illustrasjon', 'plakat', 'ikon', 'logo'],
  slideNoun: ['slide', 'slides', 'presentasjon', 'dekk', 'ppt', 'pptx'],
  videoNoun: ['video', 'animasjon', 'klipp', 'film'],
  editVerb: ['rediger', 'endre', 'modifiser', 'retusjer', 'erstatt', 'fjern'],
  animVerb: ['animer', 'få det til å bevege seg', 'gjør det til en video'],
  negation: ['ikke', 'stopp', 'avbryt'],
  question: ['hvordan', 'hva er', 'kan du forklare'],
  metaCue: ['intensjon', 'nøkkelord', 'match', 'klassifisering', 'regel', 'utløser'],
  metaStrong: ['nøkkelordsmatch', 'intensjonsklassifisering', 'ikke denne intensjonen'],
})

const DUTCH_KEYWORDS = createKeywordSet({
  action: ['genereer', 'maak', 'creëer', 'creeer', 'teken', 'ontwerp'],
  imageNoun: ['afbeelding', 'foto', 'illustratie', 'poster', 'icoon', 'logo', 'banner'],
  slideNoun: ['slide', 'slides', 'presentatie', 'deck', 'ppt', 'pptx'],
  videoNoun: ['video', 'animatie', 'clip', 'film'],
  editVerb: ['bewerk', 'verander', 'wijzig', 'retoucheer', 'vervang', 'verwijder'],
  animVerb: ['animeer', 'laat het bewegen', 'maak er een video van'],
  negation: ['niet', 'stop', 'annuleer'],
  question: ['hoe', 'wat is', 'kun je uitleggen'],
  metaCue: ['intentie', 'trefwoord', 'overeenkomst', 'classificatie', 'regel', 'trigger'],
  metaStrong: ['trefwoordmatch', 'intentieclassificatie', 'niet deze intentie'],
})

const POLISH_KEYWORDS = createKeywordSet({
  action: ['wygeneruj', 'stwórz', 'stworz', 'narysuj', 'zrób', 'zrob', 'zaprojektuj'],
  imageNoun: ['obraz', 'obrazek', 'zdjęcie', 'zdjecie', 'ilustrację', 'ilustracje', 'plakat', 'ikonę', 'ikone', 'logo'],
  slideNoun: ['slajd', 'slajdy', 'prezentację', 'prezentacje', 'prezentacja', 'ppt', 'pptx'],
  videoNoun: ['wideo', 'video', 'animację', 'animacje', 'klip', 'film'],
  editVerb: ['edytuj', 'zmień', 'zmien', 'modyfikuj', 'retuszuj', 'zastąp', 'zastap', 'usuń', 'usun'],
  animVerb: ['animuj', 'porusz', 'zrób z tego wideo', 'zrob z tego wideo'],
  negation: ['nie', 'stop', 'anuluj'],
  question: ['jak', 'co to jest', 'czy możesz wyjaśnić'],
  metaCue: ['intencja', 'słowo kluczowe', 'slowo kluczowe', 'dopasowanie', 'klasyfikacja', 'reguła'],
  metaStrong: ['dopasowanie słów kluczowych', 'klasyfikacja intencji', 'to nie ta intencja'],
})

const PORTUGUESE_KEYWORDS = createKeywordSet({
  action: ['gere', 'gera', 'crie', 'cria', 'desenhe', 'faça', 'faca', 'produza', 'projete'],
  imageNoun: ['imagem', 'foto', 'fotografia', 'ilustração', 'ilustracao', 'cartaz', 'ícone', 'icone', 'logo'],
  slideNoun: ['slide', 'slides', 'apresentação', 'apresentacao', 'ppt', 'pptx'],
  videoNoun: ['vídeo', 'video', 'animação', 'animacao', 'clipe', 'filme'],
  editVerb: ['edite', 'mude', 'modifique', 'transforme', 'retoque', 'substitua', 'remova'],
  animVerb: ['anime', 'faça mover', 'faca mover', 'transforme em vídeo', 'transforme em video'],
  negation: ['não', 'nao', 'pare', 'cancele'],
  question: ['como', 'o que é', 'o que e', 'pode explicar'],
  metaCue: ['intenção', 'intencao', 'palavra-chave', 'palavra chave', 'correspondência', 'classificação', 'regra'],
  metaStrong: ['correspondência de palavras-chave', 'classificação de intenção', 'não é esta intenção'],
})

const ROMANIAN_KEYWORDS = createKeywordSet({
  action: ['generează', 'genereaza', 'creează', 'creeaza', 'desenează', 'deseneaza', 'fă', 'fa', 'proiectează'],
  imageNoun: ['imagine', 'poză', 'poza', 'fotografie', 'ilustrație', 'ilustratie', 'poster', 'iconiță', 'iconita', 'logo'],
  slideNoun: ['slide', 'slide-uri', 'prezentare', 'ppt', 'pptx'],
  videoNoun: ['video', 'animație', 'animatie', 'clip', 'film'],
  editVerb: ['editează', 'editeaza', 'schimbă', 'schimba', 'modifică', 'modifica', 'retușează', 'retuseaza', 'înlocuiește', 'inlocuieste'],
  animVerb: ['animează', 'animeaza', 'fă-l să se miște', 'fa-l sa se miste', 'transformă-l în video', 'transforma-l in video'],
  negation: ['nu', 'oprește', 'opreste', 'anulează', 'anuleaza'],
  question: ['cum', 'ce este', 'poți explica', 'poti explica'],
  metaCue: ['intenție', 'intentie', 'cuvânt cheie', 'cuvant cheie', 'potrivire', 'clasificare', 'regulă', 'regula'],
  metaStrong: ['potrivire de cuvinte cheie', 'clasificarea intenției', 'nu este această intenție'],
})

const RUSSIAN_KEYWORDS = createKeywordSet({
  action: ['сгенерируй', 'создай', 'нарисуй', 'сделай', 'спроектируй'],
  imageNoun: ['изображение', 'картинку', 'картинка', 'фото', 'иллюстрацию', 'постер', 'иконку', 'логотип'],
  slideNoun: ['слайд', 'слайды', 'презентацию', 'презентация', 'ppt', 'pptx'],
  videoNoun: ['видео', 'анимацию', 'анимация', 'клип', 'фильм'],
  editVerb: ['отредактируй', 'измени', 'модифицируй', 'ретушируй', 'замени', 'удали'],
  animVerb: ['анимируй', 'оживи', 'сделай видео'],
  negation: ['не', 'стоп', 'отмени'],
  question: ['как', 'что такое', 'можешь объяснить'],
  metaCue: ['намерение', 'ключевое слово', 'совпадение', 'классификация', 'правило', 'триггер'],
  metaStrong: ['совпадение ключевых слов', 'классификация намерения', 'это не то намерение'],
})

const SLOVAK_KEYWORDS = createKeywordSet({
  action: ['vygeneruj', 'vytvor', 'vytvoř', 'nakresli', 'urob', 'navrhni'],
  imageNoun: ['obrázok', 'obrazok', 'fotku', 'fotografia', 'ilustráciu', 'ilustraciu', 'plagát', 'ikonu', 'logo'],
  slideNoun: ['snímku', 'snimku', 'snímka', 'snimka', 'prezentáciu', 'prezentaciu', 'prezentácia', 'ppt', 'pptx'],
  videoNoun: ['video', 'animáciu', 'animaciu', 'klip', 'film'],
  editVerb: ['uprav', 'zmeň', 'zmen', 'modifikuj', 'retušuj', 'nahraď', 'nahrad', 'odstráň', 'odstran'],
  animVerb: ['animuj', 'rozpohybuj', 'urob z toho video'],
  negation: ['nie', 'stop', 'zruš', 'zrus'],
  question: ['ako', 'čo je', 'co je', 'môžeš vysvetliť', 'mozes vysvetlit'],
  metaCue: ['zámer', 'zamer', 'kľúčové slovo', 'klucove slovo', 'zhoda', 'klasifikácia', 'pravidlo'],
  metaStrong: ['zhoda kľúčových slov', 'klasifikácia zámeru', 'nie je to tento zámer'],
})

const SWEDISH_KEYWORDS = createKeywordSet({
  action: ['generera', 'skapa', 'rita', 'gör', 'gor', 'designa'],
  imageNoun: ['bild', 'foto', 'fotografi', 'illustration', 'affisch', 'ikon', 'logotyp', 'logo'],
  slideNoun: ['slide', 'slides', 'presentation', 'presentationsbild', 'ppt', 'pptx'],
  videoNoun: ['video', 'animation', 'klipp', 'film'],
  editVerb: ['redigera', 'ändra', 'andra', 'modifiera', 'retuschera', 'ersätt', 'ersatt', 'ta bort'],
  animVerb: ['animera', 'få den att röra sig', 'gor den till en video', 'gör den till en video'],
  negation: ['inte', 'stopp', 'avbryt'],
  question: ['hur', 'vad är', 'vad ar', 'kan du förklara', 'kan du forklara'],
  metaCue: ['avsikt', 'nyckelord', 'matchning', 'klassificering', 'regel', 'utlösare', 'utlosare'],
  metaStrong: ['nyckelordsmatchning', 'avsiktsklassificering', 'inte denna avsikt'],
})

const ZH_CN_KEYWORDS = createKeywordSet({
  action: ['生成', '画', '绘', '制作', '创建', '做', '弄', '设计', '绘制'],
  imageNoun: ['图', '图片', '照片', '图像', '插画', '海报', '壁纸', '头像', '图标', 'logo', '画'],
  slideNoun: ['幻灯片', '演示文稿', '演示稿', '汇报', '路演', 'ppt', 'pptx', '封面页'],
  videoNoun: ['视频', '动画', '影片', '短片', '动图', '视频片段'],
  editVerb: [
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
    '滤镜',
    '换脸',
    '换背景',
    '去水印',
  ],
  animVerb: ['动起来', '动画化', '让它动', '变成视频'],
  negation: ['不要', '别', '停止', '取消', '不用'],
  question: ['怎么', '如何', '什么是', '能不能解释', '介绍一下'],
  metaCue: ['意图', '关键词', '匹配', '命中', '分类', '分类器', '规则', '触发', '误判'],
  metaStrong: ['不是这个意图', '关键词匹配', '意图分类', '意图识别'],
})

const ZH_TW_KEYWORDS = createKeywordSet({
  action: ['生成', '畫', '繪', '製作', '建立', '做', '弄', '設計', '繪製'],
  imageNoun: ['圖', '圖片', '照片', '影像', '插畫', '海報', '桌布', '頭像', '圖示', 'logo', '畫'],
  slideNoun: ['投影片', '簡報', '演示文稿', '演示稿', 'ppt', 'pptx', '封面頁'],
  videoNoun: ['影片', '視頻', '動畫', '短片', '動圖', '影片片段'],
  editVerb: [
    '編輯',
    '修改',
    '改',
    '變',
    '調整',
    '修復',
    '美化',
    '去除',
    '替換',
    '換',
    '修圖',
    '改圖',
    'p圖',
    '去背',
    '濾鏡',
    '換臉',
    '換背景',
    '去浮水印',
  ],
  animVerb: ['動起來', '動畫化', '讓它動', '變成影片'],
  negation: ['不要', '別', '停止', '取消', '不用'],
  question: ['怎麼', '如何', '什麼是', '能不能解釋', '介紹一下'],
  metaCue: ['意圖', '關鍵詞', '匹配', '命中', '分類', '分類器', '規則', '觸發', '誤判'],
  metaStrong: ['不是這個意圖', '關鍵詞匹配', '意圖分類', '意圖識別'],
})

const LOCALE_KEYWORDS: Readonly<Record<SupportedLocaleKey, KeywordSet>> = {
  'ca-ES': CATALAN_KEYWORDS,
  'cs-CZ': CZECH_KEYWORDS,
  'da-DK': DANISH_KEYWORDS,
  'de-DE': GERMAN_KEYWORDS,
  'el-GR': GREEK_KEYWORDS,
  'en-GB': ENGLISH_KEYWORDS,
  'en-US': ENGLISH_KEYWORDS,
  'es-ES': SPANISH_KEYWORDS,
  'fr-FR': FRENCH_KEYWORDS,
  'ga-IE': IRISH_KEYWORDS,
  'hr-HR': CROATIAN_KEYWORDS,
  'hu-HU': HUNGARIAN_KEYWORDS,
  'it-IT': ITALIAN_KEYWORDS,
  'ja-JP': JAPANESE_KEYWORDS,
  'ko-KR': KOREAN_KEYWORDS,
  'ml-IN': MALAYALAM_KEYWORDS,
  'nb-NO': NORWEGIAN_KEYWORDS,
  'nl-NL': DUTCH_KEYWORDS,
  'pl-PL': POLISH_KEYWORDS,
  'pt-BR': PORTUGUESE_KEYWORDS,
  'pt-PT': PORTUGUESE_KEYWORDS,
  'ro-RO': ROMANIAN_KEYWORDS,
  'ru-RU': RUSSIAN_KEYWORDS,
  'sk-SK': SLOVAK_KEYWORDS,
  'sv-SE': SWEDISH_KEYWORDS,
  'zh-CN': ZH_CN_KEYWORDS,
  'zh-TW': ZH_TW_KEYWORDS,
}

function isASCII(text: string): boolean {
  for (let i = 0; i < text.length; i++) {
    if (text.charCodeAt(i) > 127) return false
  }
  return true
}

function isWordBoundary(rune?: string): boolean {
  if (!rune) return true
  return !/[\p{L}\p{N}]/u.test(rune)
}

class UnicodeAhoMatcher {
  private readonly patterns: KeywordPattern[]
  private readonly nodes: AutomatonNode[]

  constructor(patterns: KeywordPattern[]) {
    this.patterns = patterns
    this.nodes = [{ children: new Map<string, number>(), fail: 0, outputs: [] }]

    for (let patternIndex = 0; patternIndex < this.patterns.length; patternIndex++) {
      const pattern = this.patterns[patternIndex]!
      let node = 0
      for (const rune of Array.from(pattern.cue)) {
        const next = this.nodes[node]!.children.get(rune)
        if (typeof next === 'number') {
          node = next
          continue
        }
        const newIndex = this.nodes.length
        this.nodes.push({ children: new Map<string, number>(), fail: 0, outputs: [] })
        this.nodes[node]!.children.set(rune, newIndex)
        node = newIndex
      }
      this.nodes[node]!.outputs.push(patternIndex)
    }

    this.buildFailures()
  }

  findAll(text: string): GroupedCueMatch[] {
    if (!text) return []

    const normalized = normalizeMatcherText(text)
    const runes = Array.from(normalized)
    if (!runes.length) return []

    const matches: GroupedCueMatch[] = []
    let node = 0

    for (let index = 0; index < runes.length; index++) {
      const rune = runes[index]!
      while (node !== 0 && !this.nodes[node]!.children.has(rune)) {
        node = this.nodes[node]!.fail
      }

      const next = this.nodes[node]!.children.get(rune)
      node = typeof next === 'number' ? next : 0

      const outputs = this.nodes[node]!.outputs
      if (!outputs.length) continue

      for (const patternIndex of outputs) {
        const pattern = this.patterns[patternIndex]!
        const end = index + 1
        const start = end - pattern.length
        if (start < 0) continue

        if (pattern.requireWordBoundary) {
          if (!isWordBoundary(runes[start - 1]) || !isWordBoundary(runes[end])) continue
        }

        matches.push({ cue: pattern.cue, group: pattern.group, start, end })
      }
    }

    return matches
  }

  private buildFailures() {
    const queue: number[] = []
    for (const child of this.nodes[0]!.children.values()) {
      this.nodes[child]!.fail = 0
      queue.push(child)
    }

    while (queue.length > 0) {
      const nodeIndex = queue.shift()!
      for (const [rune, childIndex] of this.nodes[nodeIndex]!.children.entries()) {
        queue.push(childIndex)

        let fail = this.nodes[nodeIndex]!.fail
        while (fail !== 0 && !this.nodes[fail]!.children.has(rune)) {
          fail = this.nodes[fail]!.fail
        }

        const fallback = this.nodes[fail]!.children.get(rune)
        this.nodes[childIndex]!.fail =
          typeof fallback === 'number' && fallback !== childIndex ? fallback : 0

        const failOutputs = this.nodes[this.nodes[childIndex]!.fail]!.outputs
        if (failOutputs.length > 0) {
          this.nodes[childIndex]!.outputs.push(...failOutputs)
        }
      }
    }
  }
}

function buildPatterns(keywords: KeywordSet): KeywordPattern[] {
  const seen = new Set<string>()
  const patterns: KeywordPattern[] = []

  for (const group of KEYWORD_GROUPS) {
    for (const cue of keywords[group]) {
      const normalizedCue = normalizeMatcherText(cue)
      if (!normalizedCue) continue
      const dedupeKey = `${group}:${normalizedCue}`
      if (seen.has(dedupeKey)) continue
      seen.add(dedupeKey)
      patterns.push({
        cue: normalizedCue,
        group,
        requireWordBoundary: normalizedCue.length <= 4 && isASCII(normalizedCue),
        length: Array.from(normalizedCue).length,
      })
    }
  }

  return patterns
}

function emptyGroupedMatches(): Record<KeywordGroup, CueMatch[]> {
  return {
    action: [],
    imageNoun: [],
    slideNoun: [],
    videoNoun: [],
    editVerb: [],
    animVerb: [],
    negation: [],
    question: [],
    metaCue: [],
    metaStrong: [],
    nanoPreset: [],
    bananaPreset: [],
  }
}

function groupMatches(matches: GroupedCueMatch[]): Record<KeywordGroup, CueMatch[]> {
  const grouped = emptyGroupedMatches()
  for (const match of matches) {
    grouped[match.group].push({ cue: match.cue, start: match.start, end: match.end })
  }
  return grouped
}

function normalizeLocale(locale: string): SupportedLocaleKey {
  const normalized = locale.trim().toLowerCase()
  if (!normalized) return 'en-US'

  const exact = SUPPORTED_LOCALE_LOOKUP[normalized]
  if (exact) return exact

  const alias = EXTRA_LOCALE_ALIASES[normalized]
  if (alias) return alias

  const base = normalized.split('-')[0] ?? ''
  return BASE_LOCALE_FALLBACK[base] ?? 'en-US'
}

const localeMatcherCache = new Map<SupportedLocaleKey, LocaleMatcherProfile>()

function getLocaleMatcherProfile(locale: string): LocaleMatcherProfile {
  const normalizedLocale = normalizeLocale(locale)
  const cached = localeMatcherCache.get(normalizedLocale)
  if (cached) return cached

  const primary = LOCALE_KEYWORDS[normalizedLocale] ?? ENGLISH_KEYWORDS
  const effectiveKeywords =
    normalizedLocale === 'en-US' || normalizedLocale === 'en-GB'
      ? primary
      : mergeKeywordSets(primary, ENGLISH_KEYWORDS)

  const profile: LocaleMatcherProfile = {
    locale: normalizedLocale,
    keywords: effectiveKeywords,
    matcher: new UnicodeAhoMatcher(buildPatterns(effectiveKeywords)),
  }
  localeMatcherCache.set(normalizedLocale, profile)
  return profile
}

function isQuestion(matches: CueMatch[]): boolean {
  return matches.some((match) => match.start === 0)
}

function countDistinctCueHits(matches: CueMatch[]): number {
  return new Set(matches.map((match) => match.cue)).size
}

function isMetaDiscussion(grouped: Record<KeywordGroup, CueMatch[]>): boolean {
  return grouped.metaStrong.length > 0 || countDistinctCueHits(grouped.metaCue) >= 2
}

function isNegated(textRunes: string[], grouped: Record<KeywordGroup, CueMatch[]>): boolean {
  for (const negation of grouped.negation) {
    if (negation.start < 10) return true
    for (const action of grouped.action) {
      if (action.start < negation.end) continue
      const between = textRunes.slice(negation.end, action.start).join('').trim()
      if (!between) return true
    }
  }
  return false
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

function buildSlideIntentParams(stylePreset: string): Record<string, any> {
  const params: Record<string, any> = {
    quality_profile: 'ppt',
    source: 'ppt',
  }
  if (stylePreset) params.style_preset = stylePreset
  return params
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

function classifyMediaIntentSegment(
  segment: string,
  hasImages: boolean,
  imageCount: number,
  profile: LocaleMatcherProfile
): SegmentIntentCandidate | null {
  const normalizedSegment = normalizeMatcherText(segment)
  if (!normalizedSegment) return null

  const textRunes = Array.from(normalizedSegment)
  const grouped = groupMatches(profile.matcher.findAll(normalizedSegment))

  if (isNegated(textRunes, grouped)) return null
  if (isQuestion(grouped.question)) return null
  if (isMetaDiscussion(grouped)) return null

  const hasAction = grouped.action.length > 0
  const hasImageNoun = grouped.imageNoun.length > 0
  const hasSlideNoun = grouped.slideNoun.length > 0
  const hasVideoNoun = grouped.videoNoun.length > 0
  const hasEditVerb = grouped.editVerb.length > 0
  const hasAnimVerb = grouped.animVerb.length > 0
  const hasNanoPreset = grouped.nanoPreset.length > 0
  const hasBananaPreset = grouped.bananaPreset.length > 0
  const slideStylePreset = hasNanoPreset
    ? 'nano_slides'
    : hasBananaPreset
      ? 'banana_slides'
      : ''
  const hasSlidePreset = slideStylePreset.length > 0

  let category: MediaCategory | '' = ''
  let confidence = 0
  let focusSpan = 0
  let alternativeCategory: MediaCategory | undefined
  let params: Record<string, any> | undefined

  if (imageCount >= 2 && (hasVideoNoun || hasAnimVerb)) {
    category = 'i2v'
    confidence = 0.85
    focusSpan = pickMinPositive(minSingleSpan(grouped.videoNoun), minSingleSpan(grouped.animVerb))
    alternativeCategory = 'kf2v'
  } else if (hasSlidePreset) {
    category = 't2i'
    confidence = 0.95
    focusSpan = pickMaxPositive(
      minSingleSpan(grouped.nanoPreset),
      minSingleSpan(grouped.bananaPreset),
      minSingleSpan(grouped.slideNoun),
      Math.ceil(textRunes.length / 3)
    )
    params = buildSlideIntentParams(slideStylePreset)
  } else if (hasAction && hasSlideNoun && !hasVideoNoun) {
    category = 't2i'
    confidence = 0.9
    focusSpan = pickMaxPositive(
      minSpanBetween(grouped.action, grouped.slideNoun),
      minSingleSpan(grouped.slideNoun)
    )
    params = buildSlideIntentParams('')
  } else if (hasSlideNoun && !hasAction && !hasVideoNoun) {
    category = 't2i'
    confidence = 0.72
    focusSpan = minSingleSpan(grouped.slideNoun)
    params = buildSlideIntentParams('')
  } else if (hasImages && hasAnimVerb) {
    category = 'i2v'
    confidence = 0.9
    focusSpan = pickMaxPositive(
      minSingleSpan(grouped.animVerb),
      minSpanBetween(grouped.animVerb, grouped.imageNoun)
    )
  } else if (hasImages && hasVideoNoun) {
    category = 'i2v'
    confidence = 0.85
    focusSpan = pickMaxPositive(
      minSingleSpan(grouped.videoNoun),
      minSpanBetween(grouped.videoNoun, grouped.imageNoun)
    )
  } else if (hasImages && hasEditVerb) {
    category = 'i2i'
    confidence = 0.9
    focusSpan = pickMaxPositive(
      minSingleSpan(grouped.editVerb),
      minSpanBetween(grouped.editVerb, grouped.imageNoun)
    )
  } else if (hasImages && hasAction && hasImageNoun) {
    category = 'i2i'
    confidence = 0.7
    focusSpan = minSpanBetween(grouped.action, grouped.imageNoun)
  } else if (hasAction && hasVideoNoun) {
    category = 't2v'
    confidence = 0.85
    focusSpan = minSpanBetween(grouped.action, grouped.videoNoun)
  } else if (hasVideoNoun && !hasAction) {
    category = 't2v'
    confidence = 0.6
    focusSpan = minSingleSpan(grouped.videoNoun)
  } else if (hasAction && hasImageNoun) {
    category = 't2i'
    confidence = 0.85
    focusSpan = minSpanBetween(grouped.action, grouped.imageNoun)
  } else if (hasImageNoun && !hasAction) {
    category = 't2i'
    confidence = 0.5
    focusSpan = minSingleSpan(grouped.imageNoun)
  }

  if (!category) return null
  if (focusSpan === 0) {
    focusSpan = textRunes.length
  }

  return {
    category,
    confidence,
    prompt: segment.trim(),
    focusSpan,
    alternativeCategory,
    params,
  }
}

/**
 * Client-side media intent classifier backed by a Unicode Aho-Corasick matcher,
 * so one scan can cover all cue groups for the active locale.
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

  const profile = getLocaleMatcherProfile(locale)
  const totalRunes = Array.from(normalizeMatcherText(trimmed)).length
  const segments = splitIntentSegments(trimmed)

  let best: SegmentIntentCandidate | null = null
  for (const segment of segments) {
    const candidate = classifyMediaIntentSegment(segment, hasImages, imageCount, profile)
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
    params: best.params,
  }
}
