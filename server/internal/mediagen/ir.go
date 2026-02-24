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
	},
	"zh": {
		actions:    []string{"生成", "画", "绘", "制作", "创建", "做", "弄", "设计", "绘制"},
		imageNouns: []string{"图", "图片", "照片", "图像", "插画", "海报", "壁纸", "头像", "图标", "logo", "画"},
		videoNouns: []string{"视频", "动画", "影片", "短片", "动图", "视频片段"},
		editVerbs:  []string{"编辑", "修改", "改", "变", "调整", "修复", "美化", "去除", "替换", "换", "修图", "改图", "p图", "抠图", "磨皮", "滤镜", "换脸", "换背景", "去水印", "美颜", "瘦脸", "祛痘", "补光"},
		animVerbs:  []string{"动起来", "动画化", "让它动", "变成视频"},
		negations:  []string{"不要", "别", "停止", "取消", "不用"},
		questions:  []string{"怎么", "如何", "什么是", "能不能解释", "介绍一下"},
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

	lower := strings.ToLower(message)
	lower = strings.TrimSpace(lower)

	// Determine which language keywords to use
	langKey := localeToLangKey(locale)
	kw := allLangKeywords[langKey]
	if kw == nil {
		kw = allLangKeywords["en"]
	}

	// Also check English as fallback (many users mix English keywords)
	kwEN := allLangKeywords["en"]

	// Layer 0: Negation and question filters
	if isNegated(lower, kw) || isNegated(lower, kwEN) {
		return nil
	}
	if isQuestion(lower, kw) || isQuestion(lower, kwEN) {
		return nil
	}

	// Layer 1: Keyword matching — find action + noun combinations
	var (
		hasAction     bool
		hasImageNoun  bool
		hasVideoNoun  bool
		hasEditVerb   bool
		hasAnimVerb   bool
		promptText    = message // default: use full message as prompt
	)

	// Check primary language
	hasAction = containsAny(lower, kw.actions)
	hasImageNoun = containsAny(lower, kw.imageNouns)
	hasVideoNoun = containsAny(lower, kw.videoNouns)
	hasEditVerb = containsAny(lower, kw.editVerbs)
	hasAnimVerb = containsAny(lower, kw.animVerbs)

	// Also check English (fallback for mixed-language input)
	if langKey != "en" {
		if !hasAction {
			hasAction = containsAny(lower, kwEN.actions)
		}
		if !hasImageNoun {
			hasImageNoun = containsAny(lower, kwEN.imageNouns)
		}
		if !hasVideoNoun {
			hasVideoNoun = containsAny(lower, kwEN.videoNouns)
		}
		if !hasEditVerb {
			hasEditVerb = containsAny(lower, kwEN.editVerbs)
		}
		if !hasAnimVerb {
			hasAnimVerb = containsAny(lower, kwEN.animVerbs)
		}
	}

	// Layer 2: Context signal boosting + category determination
	cat := CategoryNone
	confidence := 0.0

	switch {
	// KF2V: 2+ images + video/anim intent
	case imageCount >= 2 && (hasVideoNoun || hasAnimVerb):
		cat = CategoryKF2V
		confidence = 0.85

	// I2V: image attached + animate/video keywords
	case hasImages && hasAnimVerb:
		cat = CategoryI2V
		confidence = 0.9

	case hasImages && hasVideoNoun:
		cat = CategoryI2V
		confidence = 0.85

	// I2I: image attached + edit keywords
	case hasImages && hasEditVerb:
		cat = CategoryI2I
		confidence = 0.9

	case hasImages && hasAction && hasImageNoun:
		// Ambiguous: could be i2i or t2i with reference
		cat = CategoryI2I
		confidence = 0.7

	// T2V: no image + video keywords
	case hasAction && hasVideoNoun:
		cat = CategoryT2V
		confidence = 0.85

	case hasVideoNoun && !hasAction:
		// "a video of cats" — implicit generation
		cat = CategoryT2V
		confidence = 0.6

	// T2I: no image + image keywords
	case hasAction && hasImageNoun:
		cat = CategoryT2I
		confidence = 0.85

	case hasImageNoun && !hasAction:
		// "a picture of a sunset" — implicit generation
		cat = CategoryT2I
		confidence = 0.5
	}

	if cat == CategoryNone {
		return nil
	}

	return &MediaIntent{
		Category:   cat,
		Confidence: confidence,
		Prompt:     promptText,
		HasImage:   hasImages,
		ImageCount: imageCount,
	}
}

// containsAny checks if text contains any of the given substrings.
// For short Latin substrings (<=4 bytes, ASCII-only), requires word boundary to avoid
// false matches like "move" inside "remove". Non-ASCII substrings always use simple Contains.
func containsAny(text string, substrs []string) bool {
	for _, s := range substrs {
		idx := strings.Index(text, s)
		if idx < 0 {
			continue
		}
		// Only apply word boundary check for short ASCII-only strings
		if len(s) <= 4 && isASCII(s) {
			if isWordMatch(text, s, idx) {
				return true
			}
			continue
		}
		return true
	}
	return false
}

// isASCII returns true if all bytes in s are ASCII.
func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			return false
		}
	}
	return true
}

// isWordMatch checks if the substring at idx is a whole word (bounded by non-letters).
func isWordMatch(text, sub string, idx int) bool {
	// Check character before
	if idx > 0 {
		runes := []rune(text[:idx])
		if len(runes) > 0 && unicode.IsLetter(runes[len(runes)-1]) {
			return false
		}
	}
	// Check character after
	end := idx + len(sub)
	if end < len(text) {
		runes := []rune(text[end:])
		if len(runes) > 0 && unicode.IsLetter(runes[0]) {
			return false
		}
	}
	return true
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
