package tools

import (
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/i18n"
)

type officeTemplateGuidanceLocale struct {
	Language        i18n.Language
	TitleLabels     []string
	SubtitleLabels  []string
	StyleLabels     []string
	PlaceholderNoun []string
}

var officeTemplateGuidanceLocales = []officeTemplateGuidanceLocale{
	{
		Language:        i18n.LangEnUS,
		TitleLabels:     []string{"title", "slide title", "page title"},
		SubtitleLabels:  []string{"subtitle", "sub title"},
		StyleLabels:     []string{"style"},
		PlaceholderNoun: []string{"slide", "page"},
	},
	{
		Language:        i18n.LangEnGB,
		TitleLabels:     []string{"title", "slide title", "page title"},
		SubtitleLabels:  []string{"subtitle", "sub title"},
		StyleLabels:     []string{"style"},
		PlaceholderNoun: []string{"slide", "page"},
	},
	{
		Language:        i18n.LangZhCN,
		TitleLabels:     []string{"标题", "幻灯片标题", "页面标题"},
		SubtitleLabels:  []string{"副标题"},
		StyleLabels:     []string{"风格"},
		PlaceholderNoun: []string{"幻灯片", "页面", "页"},
	},
	{
		Language:        i18n.LangZhTW,
		TitleLabels:     []string{"標題", "投影片標題", "頁面標題"},
		SubtitleLabels:  []string{"副標題"},
		StyleLabels:     []string{"風格"},
		PlaceholderNoun: []string{"幻燈片", "投影片", "頁面", "頁"},
	},
	{
		Language:        i18n.LangJaJP,
		TitleLabels:     []string{"タイトル", "スライドタイトル", "ページタイトル"},
		SubtitleLabels:  []string{"サブタイトル"},
		StyleLabels:     []string{"スタイル"},
		PlaceholderNoun: []string{"スライド", "ページ"},
	},
	{
		Language:        i18n.LangKoKR,
		TitleLabels:     []string{"제목", "슬라이드 제목", "페이지 제목"},
		SubtitleLabels:  []string{"부제목"},
		StyleLabels:     []string{"스타일"},
		PlaceholderNoun: []string{"슬라이드", "페이지"},
	},
	{
		Language:        i18n.LangDeDE,
		TitleLabels:     []string{"titel", "folientitel", "seitentitel"},
		SubtitleLabels:  []string{"untertitel"},
		StyleLabels:     []string{"stil"},
		PlaceholderNoun: []string{"folie", "seite"},
	},
	{
		Language:        i18n.LangFrFR,
		TitleLabels:     []string{"titre", "titre de diapositive", "titre de page"},
		SubtitleLabels:  []string{"sous-titre", "sous titre"},
		StyleLabels:     []string{"style"},
		PlaceholderNoun: []string{"diapositive", "page"},
	},
	{
		Language:        i18n.LangEsES,
		TitleLabels:     []string{"título", "titulo", "título de diapositiva", "titulo de diapositiva", "título de página", "titulo de pagina"},
		SubtitleLabels:  []string{"subtítulo", "subtitulo"},
		StyleLabels:     []string{"estilo"},
		PlaceholderNoun: []string{"diapositiva", "página", "pagina"},
	},
	{
		Language:        i18n.LangItIT,
		TitleLabels:     []string{"titolo", "titolo diapositiva", "titolo pagina"},
		SubtitleLabels:  []string{"sottotitolo"},
		StyleLabels:     []string{"stile"},
		PlaceholderNoun: []string{"diapositiva", "pagina"},
	},
	{
		Language:        i18n.LangPtBR,
		TitleLabels:     []string{"título", "titulo", "título do slide", "titulo do slide", "título da página", "titulo da pagina"},
		SubtitleLabels:  []string{"subtítulo", "subtitulo"},
		StyleLabels:     []string{"estilo"},
		PlaceholderNoun: []string{"slide", "página", "pagina"},
	},
	{
		Language:        i18n.LangPtPT,
		TitleLabels:     []string{"título", "titulo", "título do slide", "titulo do slide", "título da página", "titulo da pagina"},
		SubtitleLabels:  []string{"subtítulo", "subtitulo"},
		StyleLabels:     []string{"estilo"},
		PlaceholderNoun: []string{"slide", "página", "pagina"},
	},
	{
		Language:        i18n.LangRuRU,
		TitleLabels:     []string{"заголовок", "заголовок слайда", "заголовок страницы"},
		SubtitleLabels:  []string{"подзаголовок"},
		StyleLabels:     []string{"стиль"},
		PlaceholderNoun: []string{"слайд", "страница"},
	},
	{
		Language:        i18n.LangPlPL,
		TitleLabels:     []string{"tytuł", "tytul", "tytuł slajdu", "tytul slajdu", "tytuł strony", "tytul strony"},
		SubtitleLabels:  []string{"podtytuł", "podtytul"},
		StyleLabels:     []string{"styl"},
		PlaceholderNoun: []string{"slajd", "strona"},
	},
	{
		Language:        i18n.LangNlNL,
		TitleLabels:     []string{"titel", "slidetitel", "paginatitel"},
		SubtitleLabels:  []string{"ondertitel"},
		StyleLabels:     []string{"stijl"},
		PlaceholderNoun: []string{"dia", "slide", "pagina"},
	},
	{
		Language:        i18n.LangSvSE,
		TitleLabels:     []string{"titel", "bildtitel", "slidetitel", "sidtitel"},
		SubtitleLabels:  []string{"undertitel"},
		StyleLabels:     []string{"stil"},
		PlaceholderNoun: []string{"bild", "slide", "sida"},
	},
	{
		Language:        i18n.LangDaDK,
		TitleLabels:     []string{"titel", "slidetitel", "sidetitel"},
		SubtitleLabels:  []string{"undertitel"},
		StyleLabels:     []string{"stil"},
		PlaceholderNoun: []string{"slide", "side"},
	},
	{
		Language:        i18n.LangNbNO,
		TitleLabels:     []string{"tittel", "lysbildetittel", "sidetittel"},
		SubtitleLabels:  []string{"undertittel"},
		StyleLabels:     []string{"stil"},
		PlaceholderNoun: []string{"lysbilde", "slide", "side"},
	},
	{
		Language:        i18n.LangCsCZ,
		TitleLabels:     []string{"název", "nazev", "název snímku", "nazev snimku", "název stránky", "nazev stranky"},
		SubtitleLabels:  []string{"podnadpis"},
		StyleLabels:     []string{"styl"},
		PlaceholderNoun: []string{"snímek", "snimek", "stránka", "stranka"},
	},
	{
		Language:        i18n.LangSkSK,
		TitleLabels:     []string{"názov", "nazov", "názov snímky", "nazov snimky", "názov stránky", "nazov stranky"},
		SubtitleLabels:  []string{"podnadpis"},
		StyleLabels:     []string{"štýl", "styl"},
		PlaceholderNoun: []string{"snímka", "snimka", "stránka", "stranka"},
	},
	{
		Language:        i18n.LangHuHU,
		TitleLabels:     []string{"cím", "cim", "dia címe", "dia cime", "oldal címe", "oldal cime"},
		SubtitleLabels:  []string{"alcím", "alcim"},
		StyleLabels:     []string{"stílus", "stilus"},
		PlaceholderNoun: []string{"dia", "oldal"},
	},
	{
		Language:        i18n.LangRoRO,
		TitleLabels:     []string{"titlu", "titlu diapozitiv", "titlu pagină", "titlu pagina"},
		SubtitleLabels:  []string{"subtitlu"},
		StyleLabels:     []string{"stil"},
		PlaceholderNoun: []string{"diapozitiv", "pagină", "pagina"},
	},
	{
		Language:        i18n.LangHrHR,
		TitleLabels:     []string{"naslov", "naslov slajda", "naslov stranice"},
		SubtitleLabels:  []string{"podnaslov"},
		StyleLabels:     []string{"stil"},
		PlaceholderNoun: []string{"slajd", "stranica"},
	},
	{
		Language:        i18n.LangElGR,
		TitleLabels:     []string{"τίτλος", "τιτλος", "τίτλος διαφάνειας", "τιτλος διαφανειας", "τίτλος σελίδας", "τιτλος σελιδας"},
		SubtitleLabels:  []string{"υπότιτλος", "υποτιτλος"},
		StyleLabels:     []string{"στυλ"},
		PlaceholderNoun: []string{"διαφάνεια", "διαφανεια", "σελίδα", "σελιδα"},
	},
	{
		Language:        i18n.LangCaES,
		TitleLabels:     []string{"títol", "titol", "títol de la diapositiva", "titol de la diapositiva", "títol de la pàgina", "titol de la pagina"},
		SubtitleLabels:  []string{"subtítol", "subtitol"},
		StyleLabels:     []string{"estil"},
		PlaceholderNoun: []string{"diapositiva", "pàgina", "pagina"},
	},
	{
		Language:        i18n.LangGaIE,
		TitleLabels:     []string{"teideal", "teideal sleamhnáin", "teideal sleamhnain", "teideal leathanaigh"},
		SubtitleLabels:  []string{"fotheideal"},
		StyleLabels:     []string{"stíl", "stil"},
		PlaceholderNoun: []string{"sleamhnán", "sleamhnan", "leathanach"},
	},
	{
		Language:        i18n.LangMlIN,
		TitleLabels:     []string{"ശീർഷകം", "സ്ലൈഡ് ശീർഷകം", "പേജ് ശീർഷകം"},
		SubtitleLabels:  []string{"ഉപശീർഷകം"},
		StyleLabels:     []string{"ശൈലി"},
		PlaceholderNoun: []string{"സ്ലൈഡ്", "പേജ്"},
	},
}

var (
	officePPTXGuidanceLabelKinds          = officeBuildGuidanceLabelKinds()
	officeGeneratedSlideHeadingPattern    = regexp.MustCompile(officeBuildGeneratedSlideHeadingRegexp(`.*`))
	officePlaceholderSlideTitlePattern    = regexp.MustCompile(officeBuildMultilineSlideHeadingRegexp(`.*`))
	officePlaceholderSlideTitleXMLPattern = regexp.MustCompile(
		`(?i)<vt:lpstr>` + officeBuildGeneratedSlideHeadingCore(`[^<]*`) + `</vt:lpstr>`,
	)
)

func officeBuildGuidanceLabelKinds() map[string]string {
	kinds := make(map[string]string, len(officeTemplateGuidanceLocales)*6)
	register := func(kind string, values []string) {
		for _, value := range values {
			value = officeNormalizeGuidanceLabel(value)
			if value == "" {
				continue
			}
			kinds[value] = kind
		}
	}
	for _, locale := range officeTemplateGuidanceLocales {
		register("title", locale.TitleLabels)
		register("subtitle", locale.SubtitleLabels)
		register("style", locale.StyleLabels)
	}
	return kinds
}

func officeNormalizeGuidanceLabel(label string) string {
	label = strings.ToLower(strings.TrimSpace(label))
	if label == "" {
		return ""
	}
	return strings.Join(strings.Fields(label), " ")
}

func officeBuildGeneratedSlideHeadingRegexp(suffixChars string) string {
	return `(?i)^` + officeBuildGeneratedSlideHeadingCore(suffixChars) + `$`
}

func officeBuildMultilineSlideHeadingRegexp(suffixChars string) string {
	return `(?im)^` + officeBuildGeneratedSlideHeadingCore(suffixChars) + `$`
}

func officeBuildGeneratedSlideHeadingCore(suffixChars string) string {
	suffix := `(?:\s*[:：\-–—]` + suffixChars + `)?`
	return `(?:(?:第\s*\d+\s*(?:页|頁))|(?:(?:` + officeBuildLocalizedPlaceholderTermAlternation() + `)\s*\d+)|(?:\d+\s*(?:` + officeBuildLocalizedPlaceholderTermAlternation() + `)))` + suffix
}

func officeBuildLocalizedPlaceholderTermAlternation() string {
	terms := make([]string, 0, len(officeTemplateGuidanceLocales)*2)
	seen := make(map[string]struct{}, len(officeTemplateGuidanceLocales)*2)
	for _, locale := range officeTemplateGuidanceLocales {
		for _, term := range locale.PlaceholderNoun {
			term = strings.TrimSpace(term)
			if term == "" {
				continue
			}
			if _, ok := seen[term]; ok {
				continue
			}
			seen[term] = struct{}{}
			terms = append(terms, regexp.QuoteMeta(term))
		}
	}
	sort.Strings(terms)
	return strings.Join(terms, "|")
}

var officePPTXDeckLabelsByLanguage = map[i18n.Language]officePPTXDeckLabels{
	i18n.LangEnUS: {Presentation: "Presentation", TableOfContents: "Table of Contents", Content: "Content", Summary: "Summary"},
	i18n.LangEnGB: {Presentation: "Presentation", TableOfContents: "Table of Contents", Content: "Content", Summary: "Summary"},
	i18n.LangZhCN: {Presentation: "演示文稿", TableOfContents: "目录", Content: "内容", Summary: "总结"},
	i18n.LangZhTW: {Presentation: "簡報", TableOfContents: "目錄", Content: "內容", Summary: "總結"},
	i18n.LangJaJP: {Presentation: "プレゼンテーション", TableOfContents: "目次", Content: "内容", Summary: "まとめ"},
	i18n.LangKoKR: {Presentation: "프레젠테이션", TableOfContents: "목차", Content: "내용", Summary: "요약"},
	i18n.LangDeDE: {Presentation: "Präsentation", TableOfContents: "Inhaltsverzeichnis", Content: "Inhalt", Summary: "Zusammenfassung"},
	i18n.LangFrFR: {Presentation: "Présentation", TableOfContents: "Table des matières", Content: "Contenu", Summary: "Résumé"},
	i18n.LangEsES: {Presentation: "Presentación", TableOfContents: "Índice", Content: "Contenido", Summary: "Resumen"},
	i18n.LangItIT: {Presentation: "Presentazione", TableOfContents: "Indice", Content: "Contenuto", Summary: "Riepilogo"},
	i18n.LangPtBR: {Presentation: "Apresentação", TableOfContents: "Sumário", Content: "Conteúdo", Summary: "Resumo"},
	i18n.LangPtPT: {Presentation: "Apresentação", TableOfContents: "Índice", Content: "Conteúdo", Summary: "Resumo"},
	i18n.LangRuRU: {Presentation: "Презентация", TableOfContents: "Содержание", Content: "Материал", Summary: "Итоги"},
	i18n.LangPlPL: {Presentation: "Prezentacja", TableOfContents: "Spis treści", Content: "Treść", Summary: "Podsumowanie"},
	i18n.LangNlNL: {Presentation: "Presentatie", TableOfContents: "Inhoudsopgave", Content: "Inhoud", Summary: "Samenvatting"},
	i18n.LangSvSE: {Presentation: "Presentation", TableOfContents: "Innehållsförteckning", Content: "Innehåll", Summary: "Sammanfattning"},
	i18n.LangDaDK: {Presentation: "Præsentation", TableOfContents: "Indholdsfortegnelse", Content: "Indhold", Summary: "Opsummering"},
	i18n.LangNbNO: {Presentation: "Presentasjon", TableOfContents: "Innholdsfortegnelse", Content: "Innhold", Summary: "Oppsummering"},
	i18n.LangCsCZ: {Presentation: "Prezentace", TableOfContents: "Obsah", Content: "Text", Summary: "Shrnutí"},
	i18n.LangSkSK: {Presentation: "Prezentácia", TableOfContents: "Obsah", Content: "Text", Summary: "Zhrnutie"},
	i18n.LangHuHU: {Presentation: "Bemutató", TableOfContents: "Tartalomjegyzék", Content: "Tartalom", Summary: "Összegzés"},
	i18n.LangRoRO: {Presentation: "Prezentare", TableOfContents: "Cuprins", Content: "Conținut", Summary: "Rezumat"},
	i18n.LangHrHR: {Presentation: "Prezentacija", TableOfContents: "Pregled sadržaja", Content: "Sadržaj", Summary: "Sažetak"},
	i18n.LangElGR: {Presentation: "Παρουσίαση", TableOfContents: "Περιεχόμενα", Content: "Περιεχόμενο", Summary: "Σύνοψη"},
	i18n.LangCaES: {Presentation: "Presentació", TableOfContents: "Índex", Content: "Contingut", Summary: "Resum"},
	i18n.LangGaIE: {Presentation: "Láithreoireacht", TableOfContents: "Clár Ábhair", Content: "Ábhar", Summary: "Achoimre"},
	i18n.LangMlIN: {Presentation: "അവതരണം", TableOfContents: "ഉള്ളടക്ക പട്ടിക", Content: "ഉള്ളടക്കം", Summary: "സംഗ്രഹം"},
}

func officePPTXDeckLanguageForSpec(spec officeDocSpec) i18n.Language {
	if explicit := strings.TrimSpace(spec.Language); explicit != "" {
		return i18n.ParseLanguage(explicit)
	}
	if inferred, ok := officePPTXInferDeckLanguageFromSamples(officePPTXLanguageSamples(spec)); ok {
		return inferred
	}
	return i18n.LangEnUS
}

func officePPTXLanguageSamples(spec officeDocSpec) []string {
	samples := make([]string, 0, len(spec.Sections)*3+4)
	appendSample := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		samples = append(samples, value)
	}

	appendSample(spec.Title)
	appendSample(spec.Subtitle)
	appendSample(spec.Summary)
	for _, note := range spec.Notes {
		appendSample(note)
	}
	for _, section := range spec.Sections {
		appendSample(section.Heading)
		for _, block := range officeDocBlocksOrParagraphs(section.ParagraphBlocks, section.Paragraphs) {
			appendSample(block.Text)
			if len(samples) >= 16 {
				break
			}
		}
		for _, bullet := range section.Bullets {
			appendSample(bullet)
			if len(samples) >= 16 {
				break
			}
		}
		if len(samples) >= 16 {
			break
		}
	}
	return samples
}

func officePPTXInferDeckLanguageFromSamples(samples []string) (i18n.Language, bool) {
	hasHan := false
	hasKana := false
	hasHangul := false
	hasCyrillic := false
	hasGreek := false
	hasMalayalam := false

	for _, sample := range samples {
		for _, r := range sample {
			switch {
			case unicode.In(r, unicode.Hiragana, unicode.Katakana):
				hasKana = true
			case unicode.In(r, unicode.Hangul):
				hasHangul = true
			case unicode.In(r, unicode.Han):
				hasHan = true
			case unicode.In(r, unicode.Cyrillic):
				hasCyrillic = true
			case unicode.In(r, unicode.Greek):
				hasGreek = true
			case unicode.In(r, unicode.Malayalam):
				hasMalayalam = true
			}
		}
	}

	switch {
	case hasMalayalam:
		return i18n.LangMlIN, true
	case hasGreek:
		return i18n.LangElGR, true
	case hasCyrillic:
		return i18n.LangRuRU, true
	case hasKana:
		return i18n.LangJaJP, true
	case hasHangul:
		return i18n.LangKoKR, true
	case hasHan:
		return i18n.LangZhCN, true
	default:
		return "", false
	}
}
