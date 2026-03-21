package mediagen

import (
	"strings"
	"unicode"
)

// MediaCategory represents the type of media generation intent.
type MediaCategory string

const (
	CategoryT2I  MediaCategory = "t2i"  // Text-to-Image
	CategoryT2V  MediaCategory = "t2v"  // Text-to-Video
	CategoryI2V  MediaCategory = "i2v"  // Image-to-Video
	CategoryI2I  MediaCategory = "i2i"  // Image Editing
	CategoryKF2V MediaCategory = "kf2v" // Keyframe-to-Video
	CategoryNone MediaCategory = ""     // Not a media intent
)

// MediaIntent is the result of intent classification.
type MediaIntent struct {
	Category   MediaCategory `json:"category"`
	Confidence float64       `json:"confidence"`
	Prompt     string        `json:"prompt"`
	HasImage   bool          `json:"has_image"`
	ImageCount int           `json:"image_count"`
}

// langKeywords holds action verbs and media nouns for a language group.
type langKeywords struct {
	actions    []string
	imageNouns []string
	videoNouns []string
	editVerbs  []string // edit/modify/change — boost i2i
	animVerbs  []string // animate/move — boost i2v
	negations  []string // don't/not — suppress
	questions  []string // how to/what is — suppress
	metaCues   []string // intent/classifier discussion — suppress
	metaStrong []string // strong meta-discussion phrases — suppress
}

// allLangKeywords maps language prefixes to their keyword sets.
// Covers all 27 supported locales grouped by language family.
var allLangKeywords = map[string]*langKeywords{
	"en": {
		actions:    []string{"generate", "create", "draw", "paint", "make", "produce", "render", "design", "sketch"},
		imageNouns: []string{"image", "picture", "photo", "photograph", "illustration", "artwork", "poster", "wallpaper", "portrait", "icon", "logo", "banner"},
		videoNouns: []string{"video", "animation", "clip", "movie", "film", "motion"},
		editVerbs:  []string{"edit", "change", "modify", "transform", "alter", "adjust", "fix", "retouch", "enhance", "upscale", "remove", "replace", "restyle", "photoshop", "touch up", "touchup", "crop", "inpaint", "outpaint", "colorize", "sharpen", "denoise", "swap face", "restore", "cleanup", "clean up"},
		animVerbs:  []string{"animate", "move", "come alive", "bring to life", "make it move"},
		negations:  []string{"don't", "do not", "stop", "cancel", "no more", "not"},
		questions:  []string{"how to", "how do", "what is", "can you explain", "tell me about", "what does"},
		metaCues:   []string{"intent", "keyword", "classifier", "classification", "matching", "route", "routing", "rule", "density", "trigger", "false positive"},
		metaStrong: []string{"keyword matching", "intent classification", "intent classifier", "not this intent"},
	},
	"zh": {
		actions:    []string{"生成", "画", "绘", "制作", "创建", "做", "弄", "设计", "绘制"},
		imageNouns: []string{"图", "图片", "照片", "图像", "插画", "海报", "壁纸", "头像", "图标", "logo", "画"},
		videoNouns: []string{"视频", "动画", "影片", "短片", "动图", "视频片段"},
		editVerbs:  []string{"编辑", "修改", "改", "变", "调整", "修复", "美化", "去除", "替换", "换", "修图", "改图", "p图", "抠图", "磨皮", "滤镜", "换脸", "换背景", "去水印", "美颜", "瘦脸", "祛痘", "补光"},
		animVerbs:  []string{"动起来", "动画化", "让它动", "变成视频"},
		negations:  []string{"不要", "别", "停止", "取消", "不用"},
		questions:  []string{"怎么", "如何", "什么是", "能不能解释", "介绍一下"},
		metaCues:   []string{"意图", "关键词", "匹配", "命中", "分类", "分类器", "规则", "触发", "密度", "误判"},
		metaStrong: []string{"不是这个意图", "关键词匹配", "意图分类", "意图识别"},
	},
	"ja": {
		actions:    []string{"生成", "描く", "作る", "作成", "描いて", "書いて", "作って", "デザイン"},
		imageNouns: []string{"画像", "写真", "イラスト", "絵", "ポスター", "壁紙", "アイコン"},
		videoNouns: []string{"動画", "ビデオ", "映像", "アニメ", "ムービー", "クリップ"},
		editVerbs:  []string{"編集", "変更", "修正", "加工", "調整", "変えて", "直して", "レタッチ", "修整", "補正", "切り抜き", "合成"},
		animVerbs:  []string{"アニメーション", "動かして", "動かす", "動画にして"},
		negations:  []string{"しないで", "やめて", "描かないで", "作らないで"},
		questions:  []string{"どうやって", "方法", "とは", "について", "作り方"},
	},
	"ko": {
		actions:    []string{"생성", "그려", "만들", "그리다", "제작", "디자인", "만들어"},
		imageNouns: []string{"이미지", "사진", "그림", "일러스트", "포스터", "배경화면", "아이콘"},
		videoNouns: []string{"영상", "동영상", "비디오", "애니메이션", "클립", "무비"},
		editVerbs:  []string{"편집", "수정", "변경", "바꿔", "고쳐", "조정", "보정", "리터치", "합성", "배경제거"},
		animVerbs:  []string{"애니메이션", "움직여", "동영상으로"},
		negations:  []string{"하지마", "안해", "그만", "취소"},
		questions:  []string{"어떻게", "방법", "뭐야", "설명해"},
	},
	"de": {
		actions:    []string{"erstellen", "erzeugen", "zeichnen", "malen", "generieren", "entwerfen", "gestalten"},
		imageNouns: []string{"bild", "foto", "illustration", "grafik", "poster", "hintergrundbild", "porträt"},
		videoNouns: []string{"video", "animation", "clip", "film"},
		editVerbs:  []string{"bearbeiten", "ändern", "modifizieren", "anpassen", "korrigieren", "entfernen", "retuschieren", "zuschneiden", "verbessern", "verschönern"},
		animVerbs:  []string{"animieren", "bewegen", "zum leben erwecken"},
		negations:  []string{"nicht", "kein", "stopp", "abbrechen"},
		questions:  []string{"wie kann", "was ist", "wie geht", "erkläre"},
	},
	"fr": {
		actions:    []string{"générer", "créer", "dessiner", "peindre", "produire", "concevoir", "fabriquer"},
		imageNouns: []string{"image", "photo", "illustration", "dessin", "affiche", "portrait"},
		videoNouns: []string{"vidéo", "animation", "clip", "film"},
		editVerbs:  []string{"éditer", "modifier", "changer", "ajuster", "retoucher", "supprimer", "recadrer", "améliorer", "embellir", "détourer"},
		animVerbs:  []string{"animer", "mettre en mouvement", "donner vie"},
		negations:  []string{"ne pas", "pas de", "arrête", "annuler"},
		questions:  []string{"comment", "qu'est-ce", "c'est quoi", "explique"},
	},
	"es": {
		actions:    []string{"generar", "crear", "dibujar", "pintar", "producir", "diseñar", "hacer"},
		imageNouns: []string{"imagen", "foto", "ilustración", "dibujo", "póster", "retrato"},
		videoNouns: []string{"vídeo", "video", "animación", "clip", "película"},
		editVerbs:  []string{"editar", "modificar", "cambiar", "ajustar", "retocar", "eliminar", "recortar", "mejorar", "embellecer", "restaurar"},
		animVerbs:  []string{"animar", "mover", "dar vida"},
		negations:  []string{"no", "deja de", "para de", "cancelar"},
		questions:  []string{"cómo", "qué es", "explica", "cuál es"},
	},
	"pt": {
		actions:    []string{"gerar", "criar", "desenhar", "pintar", "produzir", "fazer"},
		imageNouns: []string{"imagem", "foto", "ilustração", "desenho", "pôster", "retrato"},
		videoNouns: []string{"vídeo", "animação", "clipe", "filme"},
		editVerbs:  []string{"editar", "modificar", "alterar", "ajustar", "retocar", "remover", "recortar", "melhorar", "embelezar", "restaurar"},
		animVerbs:  []string{"animar", "mover", "dar vida"},
		negations:  []string{"não", "pare de", "cancele"},
		questions:  []string{"como", "o que é", "explique"},
	},
	"it": {
		actions:    []string{"generare", "creare", "disegnare", "dipingere", "produrre", "fare"},
		imageNouns: []string{"immagine", "foto", "illustrazione", "disegno", "poster", "ritratto"},
		videoNouns: []string{"video", "animazione", "clip", "film"},
		editVerbs:  []string{"modificare", "cambiare", "editare", "aggiustare", "ritoccare", "rimuovere", "ritagliare", "migliorare", "abbellire", "restaurare"},
		animVerbs:  []string{"animare", "muovere", "dare vita"},
		negations:  []string{"non", "smetti di", "annulla"},
		questions:  []string{"come", "cos'è", "spiega"},
	},
	"nl": {
		actions:    []string{"genereren", "maken", "tekenen", "schilderen", "ontwerpen"},
		imageNouns: []string{"afbeelding", "foto", "illustratie", "tekening", "poster"},
		videoNouns: []string{"video", "animatie", "clip", "film"},
		editVerbs:  []string{"bewerken", "wijzigen", "aanpassen", "verwijderen"},
		animVerbs:  []string{"animeren", "bewegen", "tot leven brengen"},
		negations:  []string{"niet", "geen", "stop", "annuleer"},
		questions:  []string{"hoe", "wat is", "leg uit"},
	},
	"ru": {
		actions:    []string{"сгенерировать", "создать", "нарисовать", "сделать", "нарисуй", "создай", "сгенерируй", "сделай"},
		imageNouns: []string{"изображение", "картинку", "фото", "картину", "иллюстрацию", "постер", "обои"},
		videoNouns: []string{"видео", "анимацию", "клип", "ролик", "фильм"},
		editVerbs:  []string{"редактировать", "изменить", "поменять", "исправить", "убрать", "заменить", "ретушировать", "обрезать", "улучшить", "отретушировать", "фотошоп"},
		animVerbs:  []string{"анимировать", "оживить", "сделать видео из"},
		negations:  []string{"не надо", "не нужно", "стоп", "отмена", "не"},
		questions:  []string{"как", "что такое", "объясни", "расскажи"},
	},
	"pl": {
		actions:    []string{"wygeneruj", "stwórz", "narysuj", "zrób", "zaprojektuj"},
		imageNouns: []string{"obraz", "zdjęcie", "ilustrację", "rysunek", "plakat"},
		videoNouns: []string{"wideo", "animację", "klip", "film"},
		editVerbs:  []string{"edytuj", "zmień", "popraw", "usuń", "zamień"},
		animVerbs:  []string{"animuj", "ożyw", "porusz"},
		negations:  []string{"nie", "przestań", "anuluj"},
		questions:  []string{"jak", "co to", "wyjaśnij"},
	},
	"sv": {
		actions:    []string{"generera", "skapa", "rita", "måla", "designa"},
		imageNouns: []string{"bild", "foto", "illustration", "teckning", "affisch"},
		videoNouns: []string{"video", "animation", "klipp", "film"},
		editVerbs:  []string{"redigera", "ändra", "justera", "ta bort"},
		animVerbs:  []string{"animera", "ge liv åt"},
		negations:  []string{"inte", "sluta", "avbryt"},
		questions:  []string{"hur", "vad är", "förklara"},
	},
	"da": {
		actions:    []string{"generer", "opret", "tegn", "mal", "design"},
		imageNouns: []string{"billede", "foto", "illustration", "tegning", "plakat"},
		videoNouns: []string{"video", "animation", "klip", "film"},
		editVerbs:  []string{"rediger", "ændr", "juster", "fjern"},
		animVerbs:  []string{"animer", "giv liv"},
		negations:  []string{"ikke", "stop", "annuller"},
		questions:  []string{"hvordan", "hvad er", "forklar"},
	},
	"nb": {
		actions:    []string{"generer", "lag", "tegn", "mal", "design"},
		imageNouns: []string{"bilde", "foto", "illustrasjon", "tegning", "plakat"},
		videoNouns: []string{"video", "animasjon", "klipp", "film"},
		editVerbs:  []string{"rediger", "endre", "juster", "fjern"},
		animVerbs:  []string{"animer", "gi liv"},
		negations:  []string{"ikke", "stopp", "avbryt"},
		questions:  []string{"hvordan", "hva er", "forklar"},
	},
	"cs": {
		actions:    []string{"vygeneruj", "vytvoř", "nakresli", "udělej"},
		imageNouns: []string{"obrázek", "fotku", "ilustraci", "kresbu", "plakát"},
		videoNouns: []string{"video", "animaci", "klip", "film"},
		editVerbs:  []string{"uprav", "změň", "oprav", "odstraň"},
		animVerbs:  []string{"animuj", "oživit", "rozpohybuj"},
		negations:  []string{"ne", "přestaň", "zruš"},
		questions:  []string{"jak", "co je", "vysvětli"},
	},
	"sk": {
		actions:    []string{"vygeneruj", "vytvor", "nakresli", "urob"},
		imageNouns: []string{"obrázok", "fotku", "ilustráciu", "kresbu", "plagát"},
		videoNouns: []string{"video", "animáciu", "klip", "film"},
		editVerbs:  []string{"uprav", "zmeň", "oprav", "odstráň"},
		animVerbs:  []string{"animuj", "oživiť"},
		negations:  []string{"nie", "prestaň", "zruš"},
		questions:  []string{"ako", "čo je", "vysvetli"},
	},
	"hu": {
		actions:    []string{"generálj", "készíts", "rajzolj", "alkoss", "tervezz"},
		imageNouns: []string{"képet", "fotót", "illusztrációt", "rajzot", "plakátot"},
		videoNouns: []string{"videót", "animációt", "klipet", "filmet"},
		editVerbs:  []string{"szerkeszd", "módosítsd", "változtasd", "távolítsd el"},
		animVerbs:  []string{"animáld", "keltsd életre", "mozgasd"},
		negations:  []string{"ne", "hagyd abba", "mégsem"},
		questions:  []string{"hogyan", "mi az", "magyarázd"},
	},
	"ro": {
		actions:    []string{"generează", "creează", "desenează", "fă", "proiectează"},
		imageNouns: []string{"imagine", "fotografie", "ilustrație", "desen", "poster"},
		videoNouns: []string{"video", "animație", "clip", "film"},
		editVerbs:  []string{"editează", "modifică", "schimbă", "elimină"},
		animVerbs:  []string{"animează", "dă viață"},
		negations:  []string{"nu", "oprește", "anulează"},
		questions:  []string{"cum", "ce este", "explică"},
	},
	"el": {
		actions:    []string{"δημιούργησε", "ζωγράφισε", "φτιάξε", "σχεδίασε", "κάνε"},
		imageNouns: []string{"εικόνα", "φωτογραφία", "σχέδιο", "αφίσα"},
		videoNouns: []string{"βίντεο", "κινούμενο", "κλιπ", "ταινία"},
		editVerbs:  []string{"επεξεργάσου", "άλλαξε", "τροποποίησε", "αφαίρεσε"},
		animVerbs:  []string{"κίνησε", "ζωντάνεψε"},
		negations:  []string{"μην", "σταμάτα", "ακύρωσε"},
		questions:  []string{"πώς", "τι είναι", "εξήγησε"},
	},
	"hr": {
		actions:    []string{"generiraj", "stvori", "nacrtaj", "napravi", "dizajniraj"},
		imageNouns: []string{"sliku", "fotografiju", "ilustraciju", "crtež", "plakat"},
		videoNouns: []string{"video", "animaciju", "klip", "film"},
		editVerbs:  []string{"uredi", "promijeni", "prilagodi", "ukloni"},
		animVerbs:  []string{"animiraj", "oživi"},
		negations:  []string{"ne", "prestani", "otkaži"},
		questions:  []string{"kako", "što je", "objasni"},
	},
	"ml": {
		actions:    []string{"സൃഷ്ടിക്കുക", "വരയ്ക്കുക", "ഉണ്ടാക്കുക", "നിർമ്മിക്കുക"},
		imageNouns: []string{"ചിത്രം", "ഫോട്ടോ", "ചിത്രീകരണം"},
		videoNouns: []string{"വീഡിയോ", "ആനിമേഷൻ", "ക്ലിപ്പ്"},
		editVerbs:  []string{"എഡിറ്റ്", "മാറ്റുക", "നീക്കം"},
		animVerbs:  []string{"ആനിമേറ്റ്", "ചലിപ്പിക്കുക"},
		negations:  []string{"വേണ്ട", "നിർത്തുക"},
		questions:  []string{"എങ്ങനെ", "എന്താണ്"},
	},
}

var compiledIRLangKeywords = compileLangKeywords(allLangKeywords)

// localeToLangKey maps a locale string (e.g. "zh-CN") to a language key.
func localeToLangKey(locale string) string {
	if locale == "" {
		return "en"
	}
	// Extract language prefix before hyphen
	lang := strings.ToLower(locale)
	if idx := strings.IndexByte(lang, '-'); idx > 0 {
		lang = lang[:idx]
	}
	// Map Catalan and Irish to their closest supported group
	switch lang {
	case "ca":
		return "es"
	case "ga":
		return "en"
	}
	if _, ok := allLangKeywords[lang]; ok {
		return lang
	}
	return "en"
}

// ClassifyMediaIntent determines if a user message is a media generation request.
// Returns nil if no media intent is detected.
func ClassifyMediaIntent(message string, hasImages bool, imageCount int, locale string) *MediaIntent {
	if message == "" {
		return nil
	}

	message = strings.TrimSpace(message)
	if message == "" {
		return nil
	}

	// Determine which language keywords to use
	langKey := localeToLangKey(locale)
	kw := compiledIRLangKeywords[langKey]
	if kw == nil {
		kw = compiledIRLangKeywords["en"]
	}

	// Also check English as fallback (many users mix English keywords)
	kwEN := compiledIRLangKeywords["en"]

	segments := splitIntentSegments(message)
	totalRunes := len([]rune(strings.ToLower(message)))

	var best *segmentIntentCandidate
	for _, segment := range segments {
		candidate := classifyMediaIntentSegment(segment, hasImages, imageCount, kw, kwEN, langKey != "en")
		if candidate == nil {
			continue
		}
		candidate.Confidence = applyFocusPenalty(candidate.Confidence, candidate.FocusSpan, totalRunes, hasImages)
		if candidate.Confidence <= 0 {
			continue
		}
		if best == nil || candidate.Confidence > best.Confidence ||
			(candidate.Confidence == best.Confidence && len([]rune(candidate.Prompt)) < len([]rune(best.Prompt))) {
			best = candidate
		}
	}

	if best == nil {
		return nil
	}

	return &MediaIntent{
		Category:   best.Category,
		Confidence: best.Confidence,
		Prompt:     best.Prompt,
		HasImage:   hasImages,
		ImageCount: imageCount,
	}
}

type segmentIntentCandidate struct {
	Category   MediaCategory
	Confidence float64
	Prompt     string
	FocusSpan  int
}

func classifyMediaIntentSegment(segment string, hasImages bool, imageCount int, kw, kwEN *compiledLangKeywords, includeEnglishFallback bool) *segmentIntentCandidate {
	lower := strings.ToLower(strings.TrimSpace(segment))
	if lower == "" {
		return nil
	}

	// Suppress explicit negations, informational questions, and classifier/meta discussions.
	if isNegated(lower, kw.raw) {
		return nil
	}
	if includeEnglishFallback && isNegated(lower, kwEN.raw) {
		return nil
	}
	if isQuestion(lower, kw.raw) {
		return nil
	}
	if includeEnglishFallback && isQuestion(lower, kwEN.raw) {
		return nil
	}
	if isMetaDiscussion(lower, kw) {
		return nil
	}
	if includeEnglishFallback && isMetaDiscussion(lower, kwEN) {
		return nil
	}

	actionMatches := kw.actions.FindMatches(lower)
	imageMatches := kw.imageNouns.FindMatches(lower)
	videoMatches := kw.videoNouns.FindMatches(lower)
	editMatches := kw.editVerbs.FindMatches(lower)
	animMatches := kw.animVerbs.FindMatches(lower)

	if includeEnglishFallback {
		actionMatches = append(actionMatches, kwEN.actions.FindMatches(lower)...)
		imageMatches = append(imageMatches, kwEN.imageNouns.FindMatches(lower)...)
		videoMatches = append(videoMatches, kwEN.videoNouns.FindMatches(lower)...)
		editMatches = append(editMatches, kwEN.editVerbs.FindMatches(lower)...)
		animMatches = append(animMatches, kwEN.animVerbs.FindMatches(lower)...)
	}

	hasAction := len(actionMatches) > 0
	hasImageNoun := len(imageMatches) > 0
	hasVideoNoun := len(videoMatches) > 0
	hasEditVerb := len(editMatches) > 0
	hasAnimVerb := len(animMatches) > 0

	cat := CategoryNone
	confidence := 0.0
	focusSpan := 0

	switch {
	case imageCount >= 2 && (hasVideoNoun || hasAnimVerb):
		cat = CategoryKF2V
		confidence = 0.85
		focusSpan = pickMinPositive(minSingleSpan(videoMatches), minSingleSpan(animMatches))

	case hasImages && hasAnimVerb:
		cat = CategoryI2V
		confidence = 0.9
		focusSpan = pickMaxPositive(minSingleSpan(animMatches), minSpanBetween(animMatches, imageMatches))

	case hasImages && hasVideoNoun:
		cat = CategoryI2V
		confidence = 0.85
		focusSpan = pickMaxPositive(minSingleSpan(videoMatches), minSpanBetween(videoMatches, imageMatches))

	case hasImages && hasEditVerb:
		cat = CategoryI2I
		confidence = 0.9
		focusSpan = pickMaxPositive(minSingleSpan(editMatches), minSpanBetween(editMatches, imageMatches))

	case hasImages && hasAction && hasImageNoun:
		cat = CategoryI2I
		confidence = 0.7
		focusSpan = minSpanBetween(actionMatches, imageMatches)

	case hasAction && hasVideoNoun:
		cat = CategoryT2V
		confidence = 0.85
		focusSpan = minSpanBetween(actionMatches, videoMatches)

	case hasVideoNoun && !hasAction:
		cat = CategoryT2V
		confidence = 0.6
		focusSpan = minSingleSpan(videoMatches)

	case hasAction && hasImageNoun:
		cat = CategoryT2I
		confidence = 0.85
		focusSpan = minSpanBetween(actionMatches, imageMatches)

	case hasImageNoun && !hasAction:
		cat = CategoryT2I
		confidence = 0.5
		focusSpan = minSingleSpan(imageMatches)
	}

	if cat == CategoryNone {
		return nil
	}
	if focusSpan == 0 {
		focusSpan = len([]rune(lower))
	}

	return &segmentIntentCandidate{
		Category:   cat,
		Confidence: confidence,
		Prompt:     strings.TrimSpace(segment),
		FocusSpan:  focusSpan,
	}
}

// isNegated checks if the message starts with or contains negation patterns.
func isNegated(text string, kw *langKeywords) bool {
	if kw == nil {
		return false
	}
	for _, neg := range kw.negations {
		// Check if negation appears before any action verb
		negIdx := strings.Index(text, neg)
		if negIdx < 0 {
			continue
		}
		// Negation at start or near start is a strong signal
		if negIdx < 10 {
			return true
		}
		// Check if negation is immediately before an action verb
		after := text[negIdx+len(neg):]
		after = strings.TrimLeftFunc(after, unicode.IsSpace)
		for _, act := range kw.actions {
			if strings.HasPrefix(after, act) {
				return true
			}
		}
	}
	return false
}

// isQuestion checks if the message is an informational question, not a generation request.
func isQuestion(text string, kw *langKeywords) bool {
	if kw == nil {
		return false
	}
	for _, q := range kw.questions {
		if strings.HasPrefix(text, q) {
			return true
		}
	}
	return false
}

func isMetaDiscussion(text string, kw *compiledLangKeywords) bool {
	if kw == nil {
		return false
	}
	if kw.metaStrong.Contains(text) {
		return true
	}
	return kw.metaCues.CountDistinct(text) >= 2
}

func splitIntentSegments(message string) []string {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return nil
	}

	segments := []string{trimmed}
	seen := map[string]struct{}{trimmed: {}}
	for _, raw := range strings.FieldsFunc(trimmed, isIntentBoundary) {
		segment := strings.TrimSpace(raw)
		if segment == "" {
			continue
		}
		if _, ok := seen[segment]; ok {
			continue
		}
		seen[segment] = struct{}{}
		segments = append(segments, segment)
	}
	return segments
}

func isIntentBoundary(r rune) bool {
	switch r {
	case '\n', '\r', '\t', ',', '，', '。', '!', '！', '?', '？', ';', '；', ':', '：', '、', '(', ')', '[', ']', '{', '}', '<', '>', '|':
		return true
	default:
		return false
	}
}

func minSingleSpan(matches []cueMatch) int {
	best := 0
	for _, match := range matches {
		span := match.end - match.start
		if span <= 0 {
			continue
		}
		if best == 0 || span < best {
			best = span
		}
	}
	return best
}

func minSpanBetween(left, right []cueMatch) int {
	best := 0
	for _, l := range left {
		for _, r := range right {
			start := l.start
			if r.start < start {
				start = r.start
			}
			end := l.end
			if r.end > end {
				end = r.end
			}
			span := end - start
			if span <= 0 {
				continue
			}
			if best == 0 || span < best {
				best = span
			}
		}
	}
	return best
}

func pickMinPositive(values ...int) int {
	best := 0
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if best == 0 || value < best {
			best = value
		}
	}
	return best
}

func pickMaxPositive(values ...int) int {
	best := 0
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if value > best {
			best = value
		}
	}
	return best
}

func applyFocusPenalty(confidence float64, focusSpan, totalRunes int, hasImages bool) float64 {
	if confidence <= 0 || focusSpan <= 0 || totalRunes <= 0 {
		return confidence
	}
	if totalRunes <= 16 {
		return confidence
	}

	ratio := float64(focusSpan) / float64(totalRunes)
	if !hasImages && totalRunes >= 72 && ratio < 0.10 {
		return 0
	}

	if hasImages {
		switch {
		case totalRunes >= 72 && ratio < 0.08:
			confidence -= 0.25
		case totalRunes >= 40 && ratio < 0.15:
			confidence -= 0.10
		}
	} else {
		switch {
		case totalRunes >= 120 && ratio < 0.15:
			confidence -= 0.25
		case totalRunes >= 60 && ratio < 0.20:
			confidence -= 0.15
		case totalRunes >= 24 && ratio < 0.12:
			confidence -= 0.20
		}
	}

	if confidence < 0 {
		return 0
	}
	return confidence
}
