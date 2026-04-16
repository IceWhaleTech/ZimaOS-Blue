package tools

import (
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/i18n"
)

func TestOfficeTemplateGuidanceLocalesCoverSupportedLanguages(t *testing.T) {
	expected := []i18n.Language{
		i18n.LangEnUS,
		i18n.LangEnGB,
		i18n.LangZhCN,
		i18n.LangZhTW,
		i18n.LangJaJP,
		i18n.LangKoKR,
		i18n.LangDeDE,
		i18n.LangFrFR,
		i18n.LangEsES,
		i18n.LangItIT,
		i18n.LangPtBR,
		i18n.LangPtPT,
		i18n.LangRuRU,
		i18n.LangPlPL,
		i18n.LangNlNL,
		i18n.LangSvSE,
		i18n.LangDaDK,
		i18n.LangNbNO,
		i18n.LangCsCZ,
		i18n.LangSkSK,
		i18n.LangHuHU,
		i18n.LangRoRO,
		i18n.LangHrHR,
		i18n.LangElGR,
		i18n.LangCaES,
		i18n.LangGaIE,
		i18n.LangMlIN,
	}

	if len(officeTemplateGuidanceLocales) != len(expected) {
		t.Fatalf("len(officeTemplateGuidanceLocales) = %d, want %d", len(officeTemplateGuidanceLocales), len(expected))
	}

	seen := make(map[i18n.Language]struct{}, len(officeTemplateGuidanceLocales))
	for _, locale := range officeTemplateGuidanceLocales {
		if _, ok := seen[locale.Language]; ok {
			t.Fatalf("duplicate locale entry for %s", locale.Language)
		}
		seen[locale.Language] = struct{}{}
	}
	for _, lang := range expected {
		if _, ok := seen[lang]; !ok {
			t.Fatalf("missing locale guidance entry for %s", lang)
		}
	}
}

func TestOfficePPTXGuidanceLabelKindRecognizesSupportedLocalizedAliases(t *testing.T) {
	cases := []struct {
		label string
		want  string
	}{
		{"Title", "title"},
		{"Subtitle", "subtitle"},
		{"Style", "style"},
		{"Page Title", "title"},

		{"标题", "title"},
		{"副标题", "subtitle"},
		{"风格", "style"},

		{"標題", "title"},
		{"副標題", "subtitle"},
		{"風格", "style"},

		{"タイトル", "title"},
		{"サブタイトル", "subtitle"},
		{"スタイル", "style"},

		{"제목", "title"},
		{"부제목", "subtitle"},
		{"스타일", "style"},

		{"Titel", "title"},
		{"Untertitel", "subtitle"},
		{"Stil", "style"},

		{"Titre", "title"},
		{"Sous-titre", "subtitle"},
		{"Style", "style"},

		{"Título", "title"},
		{"Subtítulo", "subtitle"},
		{"Estilo", "style"},

		{"Titolo", "title"},
		{"Sottotitolo", "subtitle"},
		{"Stile", "style"},

		{"Título do slide", "title"},
		{"Subtítulo", "subtitle"},
		{"Estilo", "style"},

		{"Título da página", "title"},
		{"Subtítulo", "subtitle"},
		{"Estilo", "style"},

		{"Заголовок", "title"},
		{"Подзаголовок", "subtitle"},
		{"Стиль", "style"},

		{"Tytuł", "title"},
		{"Podtytuł", "subtitle"},
		{"Styl", "style"},

		{"Paginatitel", "title"},
		{"Ondertitel", "subtitle"},
		{"Stijl", "style"},

		{"Bildtitel", "title"},
		{"Undertitel", "subtitle"},
		{"Stil", "style"},

		{"Sidetitel", "title"},
		{"Undertitel", "subtitle"},
		{"Stil", "style"},

		{"Lysbildetittel", "title"},
		{"Undertittel", "subtitle"},
		{"Stil", "style"},

		{"Název snímku", "title"},
		{"Podnadpis", "subtitle"},
		{"Styl", "style"},

		{"Názov snímky", "title"},
		{"Podnadpis", "subtitle"},
		{"Štýl", "style"},

		{"Dia címe", "title"},
		{"Alcím", "subtitle"},
		{"Stílus", "style"},

		{"Titlu diapozitiv", "title"},
		{"Subtitlu", "subtitle"},
		{"Stil", "style"},

		{"Naslov slajda", "title"},
		{"Podnaslov", "subtitle"},
		{"Stil", "style"},

		{"Τίτλος", "title"},
		{"Υπότιτλος", "subtitle"},
		{"Στυλ", "style"},

		{"Títol de la diapositiva", "title"},
		{"Subtítol", "subtitle"},
		{"Estil", "style"},

		{"Teideal sleamhnáin", "title"},
		{"Fotheideal", "subtitle"},
		{"Stíl", "style"},

		{"സ്ലൈഡ് ശീർഷകം", "title"},
		{"ഉപശീർഷകം", "subtitle"},
		{"ശൈലി", "style"},
	}

	for _, tc := range cases {
		if got := officePPTXGuidanceLabelKind(tc.label); got != tc.want {
			t.Fatalf("officePPTXGuidanceLabelKind(%q) = %q, want %q", tc.label, got, tc.want)
		}
	}
}

func TestOfficeLooksLikeGeneratedSlideHeadingRecognizesSupportedLanguages(t *testing.T) {
	cases := []string{
		"Slide 1: Cover",
		"Page 2: Agenda",
		"第1页：封面",
		"第2頁：封面",
		"スライド 3: 概要",
		"슬라이드 4: 소개",
		"Folie 5: Überblick",
		"Diapositive 6 : Vue d'ensemble",
		"Diapositiva 7: Portada",
		"Diapositiva 8: Panoramica",
		"Slide 9: Visão geral",
		"Página 10: Visão geral",
		"Слайд 11: Обзор",
		"Slajd 12: Przegląd",
		"Dia 13: Overzicht",
		"Bild 14: Översikt",
		"Slide 15: Oversigt",
		"Lysbilde 16: Oversikt",
		"Snímek 17: Přehled",
		"Snímka 18: Prehľad",
		"Dia 19: Áttekintés",
		"Diapozitiv 20: Prezentare generală",
		"Slajd 21: Pregled",
		"Διαφάνεια 22: Επισκόπηση",
		"Diapositiva 23: Resum",
		"Sleamhnán 24: Forbhreathnú",
		"സ്ലൈഡ് 25: അവലോകനം",
	}

	for _, text := range cases {
		if !officeLooksLikeGeneratedSlideHeading(text) {
			t.Fatalf("officeLooksLikeGeneratedSlideHeading(%q) = false, want true", text)
		}
	}
}

func TestOfficeBuildPresentationSlidesCleansSpanishPagePlaceholdersAndGuidanceLabels(t *testing.T) {
	slides := officeBuildPresentationSlides(officeDocSpec{
		Title:    "Qwen3 Lanzamiento",
		Subtitle: "Think Deeper, Act Faster",
		Sections: []officeDocSection{
			{
				Heading: "Diapositiva 1: Portada",
				ParagraphBlocks: []officeDocBlock{
					{Kind: officeDocBlockParagraph, Text: "Título: Qwen3 Lanzamiento"},
					{Kind: officeDocBlockParagraph, Text: "Subtítulo: Think Deeper, Act Faster"},
					{Kind: officeDocBlockParagraph, Text: "Estilo: Editorial moderno"},
				},
			},
			{
				Heading: "Diapositiva 2: ¿Qué es Qwen3?",
				ParagraphBlocks: []officeDocBlock{
					{Kind: officeDocBlockParagraph, Text: "Título: ¿Qué es Qwen3?"},
					{Kind: officeDocBlockParagraph, Text: "Qwen3 es una nueva generación de modelos de lenguaje."},
				},
			},
		},
	})

	if len(slides) != 3 {
		t.Fatalf("len(slides) = %d, want 3 (%#v)", len(slides), slides)
	}
	if slides[0].Title != "Qwen3 Lanzamiento" {
		t.Fatalf("cover title = %q, want clean deck title", slides[0].Title)
	}
	if len(slides[0].Lines) != 1 || slides[0].Lines[0] != "Think Deeper, Act Faster" {
		t.Fatalf("cover lines = %#v, want only cleaned subtitle", slides[0].Lines)
	}
	if slides[1].Title != "Table of Contents" || len(slides[1].Lines) != 1 || slides[1].Lines[0] != "1. ¿Qué es Qwen3?" {
		t.Fatalf("toc = %#v, want cleaned section title without Diapositiva N prefix", slides[1])
	}
	if slides[2].Title != "¿Qué es Qwen3?" {
		t.Fatalf("section title = %q, want cleaned title guidance value", slides[2].Title)
	}
	if len(slides[2].Blocks) != 1 || slides[2].Blocks[0].Text != "Qwen3 es una nueva generación de modelos de lenguaje." {
		t.Fatalf("section blocks = %#v, want guidance labels stripped from body", slides[2].Blocks)
	}
}

func TestOfficePPTXDeckLabelsForExplicitSupportedLanguage(t *testing.T) {
	cases := []struct {
		lang string
		want officePPTXDeckLabels
	}{
		{
			lang: "es-ES",
			want: officePPTXDeckLabels{
				Presentation:    "Presentación",
				TableOfContents: "Índice",
				Content:         "Contenido",
				Summary:         "Resumen",
			},
		},
		{
			lang: "ru-RU",
			want: officePPTXDeckLabels{
				Presentation:    "Презентация",
				TableOfContents: "Содержание",
				Content:         "Материал",
				Summary:         "Итоги",
			},
		},
		{
			lang: "ja-JP",
			want: officePPTXDeckLabels{
				Presentation:    "プレゼンテーション",
				TableOfContents: "目次",
				Content:         "内容",
				Summary:         "まとめ",
			},
		},
		{
			lang: "ml-IN",
			want: officePPTXDeckLabels{
				Presentation:    "അവതരണം",
				TableOfContents: "ഉള്ളടക്ക പട്ടിക",
				Content:         "ഉള്ളടക്കം",
				Summary:         "സംഗ്രഹം",
			},
		},
	}

	for _, tc := range cases {
		got := officePPTXDeckLabelsForSpec(officeDocSpec{Language: tc.lang})
		if got != tc.want {
			t.Fatalf("officePPTXDeckLabelsForSpec(%q) = %#v, want %#v", tc.lang, got, tc.want)
		}
	}
}

func TestOfficeBuildPresentationSlidesLocalizesDeckChromeFromExplicitLanguage(t *testing.T) {
	slides := officeBuildPresentationSlides(officeDocSpec{
		Title:    "Qwen3 Lanzamiento",
		Subtitle: "Think Deeper, Act Faster",
		Summary:  "Resumen final",
		Language: "es-ES",
		Sections: []officeDocSection{
			{
				Heading:    "¿Qué es Qwen3?",
				Paragraphs: []string{"Qwen3 es una nueva generación de modelos de lenguaje."},
			},
		},
	})

	if len(slides) != 4 {
		t.Fatalf("len(slides) = %d, want 4 (%#v)", len(slides), slides)
	}
	if slides[1].Title != "Índice" {
		t.Fatalf("toc title = %q, want Índice", slides[1].Title)
	}
	if slides[3].Title != "Resumen" {
		t.Fatalf("summary title = %q, want Resumen", slides[3].Title)
	}
}

func TestOfficeBuildPresentationSlidesInfersJapaneseDeckChromeFromContent(t *testing.T) {
	slides := officeBuildPresentationSlides(officeDocSpec{
		Title:    "Qwen3 最新紹介",
		Subtitle: "より深く考え、より速く動く",
		Summary:  "最後のまとめ",
		Sections: []officeDocSection{
			{
				Heading:    "Qwen3とは？",
				Paragraphs: []string{"Qwen3は新世代の大規模言語モデルです。"},
			},
		},
	})

	if len(slides) != 4 {
		t.Fatalf("len(slides) = %d, want 4 (%#v)", len(slides), slides)
	}
	if slides[1].Title != "目次" {
		t.Fatalf("toc title = %q, want 目次", slides[1].Title)
	}
	if slides[3].Title != "まとめ" {
		t.Fatalf("summary title = %q, want まとめ", slides[3].Title)
	}
}

func TestBuildOfficePPTXUsesExplicitLocaleForSlideTextRuns(t *testing.T) {
	pptxData, _, err := buildOfficePPTX(officeDocSpec{
		Title:    "Qwen3 Lanzamiento",
		Subtitle: "Piensa más, actúa más rápido",
		Language: "es-ES",
		Sections: []officeDocSection{
			{
				Heading:    "¿Qué es Qwen3?",
				Paragraphs: []string{"Qwen3 es una nueva generación de modelos de lenguaje."},
			},
		},
	})
	if err != nil {
		t.Fatalf("buildOfficePPTX failed: %v", err)
	}

	slideXML := officeZipEntryText(t, pptxData, "ppt/slides/slide1.xml")
	if !containsSubstring(slideXML, `lang="es-ES"`) {
		t.Fatalf("expected slide XML to contain es-ES language metadata, got %s", slideXML)
	}
	if containsSubstring(slideXML, `lang="en-US"`) {
		t.Fatalf("expected slide XML to stop hardcoding en-US when explicit locale is provided, got %s", slideXML)
	}

	coreXML := officeZipEntryText(t, pptxData, "docProps/core.xml")
	if !containsSubstring(coreXML, `<dc:language>es-ES</dc:language>`) {
		t.Fatalf("expected core.xml to contain es-ES language metadata, got %s", coreXML)
	}
}

func TestBuildOfficePPTXUsesExplicitLocaleForChartTextMetadata(t *testing.T) {
	pptxData, _, err := buildOfficePPTX(officeDocSpec{
		Title:    "Qwen3 最新紹介",
		Language: "ja-JP",
		Sections: []officeDocSection{
			{
				Heading: "性能比較",
				Chart: &officeChartSpec{
					Type:       "bar",
					Categories: []string{"Q1", "Q2"},
					Series: []officeChartSeries{
						{Name: "精度", Values: []float64{82, 91}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("buildOfficePPTX failed: %v", err)
	}

	chartXML := officeZipEntryText(t, pptxData, "ppt/charts/chart1.xml")
	for _, needle := range []string{
		`<c:lang val="ja-JP"/>`,
		`<a:defRPr lang="ja-JP"`,
		`<a:rPr lang="ja-JP"`,
		`<a:endParaRPr lang="ja-JP"`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart XML to contain %q, got %s", needle, chartXML)
		}
	}
	if containsSubstring(chartXML, `lang="en-US"`) {
		t.Fatalf("expected chart XML to stop hardcoding en-US when explicit locale is provided, got %s", chartXML)
	}
}
