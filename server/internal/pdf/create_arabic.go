package pdf

import "strings"

// Arabic shaping helpers adapted from garabic's presentation-form approach so
// the PDF path can render joined Arabic glyphs without a full shaping engine.
type createArabicLetterGroup struct {
	backLetter  rune
	letter      rune
	frontLetter rune
}

type createArabicLetterShape struct {
	Independent rune
	Initial     rune
	Medial      rune
	Final       rune
}

var createArabicAlphabetShapes = map[rune]createArabicLetterShape{
	'\u0621': {Independent: '\uFE80', Initial: '\u0621', Medial: '\u0621', Final: '\u0621'},
	'\u0622': {Independent: '\uFE81', Initial: '\u0622', Medial: '\uFE82', Final: '\uFE82'},
	'\u0623': {Independent: '\uFE83', Initial: '\u0623', Medial: '\uFE84', Final: '\uFE84'},
	'\u0624': {Independent: '\uFE85', Initial: '\u0624', Medial: '\uFE86', Final: '\uFE86'},
	'\u0625': {Independent: '\uFE87', Initial: '\u0625', Medial: '\uFE88', Final: '\uFE88'},
	'\u0626': {Independent: '\uFE89', Initial: '\uFE8B', Medial: '\uFE8C', Final: '\uFE8A'},
	'\u0627': {Independent: '\uFE8D', Initial: '\u0627', Medial: '\uFE8E', Final: '\uFE8E'},
	'\u0628': {Independent: '\uFE8F', Initial: '\uFE91', Medial: '\uFE92', Final: '\uFE90'},
	'\u0629': {Independent: '\uFE93', Initial: '\u0629', Medial: '\u0629', Final: '\uFE94'},
	'\u062A': {Independent: '\uFE95', Initial: '\uFE97', Medial: '\uFE98', Final: '\uFE96'},
	'\u062B': {Independent: '\uFE99', Initial: '\uFE9B', Medial: '\uFE9C', Final: '\uFE9A'},
	'\u062C': {Independent: '\uFE9D', Initial: '\uFE9F', Medial: '\uFEA0', Final: '\uFE9E'},
	'\u062D': {Independent: '\uFEA1', Initial: '\uFEA3', Medial: '\uFEA4', Final: '\uFEA2'},
	'\u062E': {Independent: '\uFEA5', Initial: '\uFEA7', Medial: '\uFEA8', Final: '\uFEA6'},
	'\u062F': {Independent: '\uFEA9', Initial: '\u062F', Medial: '\uFEAA', Final: '\uFEAA'},
	'\u0630': {Independent: '\uFEAB', Initial: '\u0630', Medial: '\uFEAC', Final: '\uFEAC'},
	'\u0631': {Independent: '\uFEAD', Initial: '\u0631', Medial: '\uFEAE', Final: '\uFEAE'},
	'\u0632': {Independent: '\uFEAF', Initial: '\u0632', Medial: '\uFEB0', Final: '\uFEB0'},
	'\u0633': {Independent: '\uFEB1', Initial: '\uFEB3', Medial: '\uFEB4', Final: '\uFEB2'},
	'\u0634': {Independent: '\uFEB5', Initial: '\uFEB7', Medial: '\uFEB8', Final: '\uFEB6'},
	'\u0635': {Independent: '\uFEB9', Initial: '\uFEBB', Medial: '\uFEBC', Final: '\uFEBA'},
	'\u0636': {Independent: '\uFEBD', Initial: '\uFEBF', Medial: '\uFEC0', Final: '\uFEBE'},
	'\u0637': {Independent: '\uFEC1', Initial: '\uFEC3', Medial: '\uFEC4', Final: '\uFEC2'},
	'\u0638': {Independent: '\uFEC5', Initial: '\uFEC7', Medial: '\uFEC8', Final: '\uFEC6'},
	'\u0639': {Independent: '\uFEC9', Initial: '\uFECB', Medial: '\uFECC', Final: '\uFECA'},
	'\u063A': {Independent: '\uFECD', Initial: '\uFECF', Medial: '\uFED0', Final: '\uFECE'},
	'\u0640': {Independent: '\u0640', Initial: '\u0640', Medial: '\u0640', Final: '\u0640'},
	'\u0641': {Independent: '\uFED1', Initial: '\uFED3', Medial: '\uFED4', Final: '\uFED2'},
	'\u0642': {Independent: '\uFED5', Initial: '\uFED7', Medial: '\uFED8', Final: '\uFED6'},
	'\u0643': {Independent: '\uFED9', Initial: '\uFEDB', Medial: '\uFEDC', Final: '\uFEDA'},
	'\u0644': {Independent: '\uFEDD', Initial: '\uFEDF', Medial: '\uFEE0', Final: '\uFEDE'},
	'\u0645': {Independent: '\uFEE1', Initial: '\uFEE3', Medial: '\uFEE4', Final: '\uFEE2'},
	'\u0646': {Independent: '\uFEE5', Initial: '\uFEE7', Medial: '\uFEE8', Final: '\uFEE6'},
	'\u0647': {Independent: '\uFEE9', Initial: '\uFEEB', Medial: '\uFEEC', Final: '\uFEEA'},
	'\u0648': {Independent: '\uFEED', Initial: '\u0648', Medial: '\uFEEE', Final: '\uFEEE'},
	'\u0649': {Independent: '\uFEEF', Initial: '\u0649', Medial: '\uFEF0', Final: '\uFEF0'},
	'\u064A': {Independent: '\uFEF1', Initial: '\uFEF3', Medial: '\uFEF4', Final: '\uFEF2'},
}

func createTextAlign(text string) string {
	if createHasArabicLetters(text) {
		return "R"
	}
	return "L"
}

func createHasArabicLetters(text string) bool {
	for _, r := range text {
		if createIsArabicLetter(r) {
			return true
		}
	}
	return false
}

func createShapeArabicVisual(input string) string {
	if !createHasArabicLetters(input) {
		return strings.TrimSpace(input)
	}
	sections := createArabicSections(input)
	visual := make([]string, 0, len(sections))
	for _, section := range sections {
		if createArabicOnly(section) {
			for _, word := range strings.Fields(section) {
				visual = append(visual, createShapeArabicWordVisual(word))
			}
			continue
		}
		section = strings.TrimSpace(section)
		if section != "" {
			visual = append(visual, section)
		}
	}
	for i, j := 0, len(visual)-1; i < j; i, j = i+1, j-1 {
		visual[i], visual[j] = visual[j], visual[i]
	}
	return strings.Join(visual, " ")
}

func createArabicSections(input string) []string {
	sections := make([]string, 0, 4)
	var arabicPart strings.Builder
	var otherPart strings.Builder
	flush := func(part *strings.Builder) {
		text := strings.TrimSpace(part.String())
		if text != "" {
			sections = append(sections, text)
		}
		part.Reset()
	}
	for _, r := range input {
		if createIsArabicLetter(r) {
			if otherPart.Len() > 0 {
				flush(&otherPart)
			}
			arabicPart.WriteRune(r)
			continue
		}
		if arabicPart.Len() > 0 {
			flush(&arabicPart)
		}
		otherPart.WriteRune(r)
	}
	if otherPart.Len() > 0 {
		flush(&otherPart)
	}
	if arabicPart.Len() > 0 {
		flush(&arabicPart)
	}
	return sections
}

func createShapeArabicWordVisual(input string) string {
	shaped := createShapeArabicWordLogical(input)
	if shaped == input {
		return input
	}
	return createReverseRunes(shaped)
}

func createShapeArabicWordLogical(input string) string {
	if !createArabicOnly(input) {
		return input
	}
	letters := []rune(createRemoveArabicHarakat(input))
	shaped := make([]rune, 0, len(letters))
	for i, letter := range letters {
		back, front := rune(0), rune(0)
		if i > 0 {
			back = letters[i-1]
		}
		if i+1 < len(letters) {
			front = letters[i+1]
		}
		if _, ok := createArabicAlphabetShapes[letter]; ok {
			shaped = append(shaped, createAdjustArabicLetter(createArabicLetterGroup{backLetter: back, letter: letter, frontLetter: front}))
			continue
		}
		shaped = append(shaped, letter)
	}
	if len(shaped) == len([]rune(input)) {
		return string(shaped)
	}
	out := make([]rune, 0, len([]rune(input)))
	letterIndex := 0
	for _, r := range input {
		if _, ok := createArabicAlphabetShapes[r]; ok {
			out = append(out, shaped[letterIndex])
			letterIndex++
			continue
		}
		out = append(out, r)
	}
	return string(out)
}

func createRemoveArabicHarakat(input string) string {
	out := make([]rune, 0, len([]rune(input)))
	for _, r := range input {
		switch r {
		case '\u0640', '\u064B', '\u064C', '\u064D', '\u064E', '\u064F', '\u0650', '\u0651', '\u0652', '\u0670':
			continue
		case '\u0671':
			out = append(out, '\u0627')
		default:
			out = append(out, r)
		}
	}
	return string(out)
}

func createAdjustArabicLetter(group createArabicLetterGroup) rune {
	shapes := createArabicAlphabetShapes[group.letter]
	switch {
	case group.backLetter > 0 && group.frontLetter > 0:
		if createArabicIsAlwaysInitial(group.backLetter) {
			return shapes.Initial
		}
		return shapes.Medial
	case group.backLetter == 0 && group.frontLetter > 0:
		return shapes.Initial
	case group.backLetter > 0 && group.frontLetter == 0:
		if createArabicIsAlwaysInitial(group.backLetter) {
			return shapes.Independent
		}
		return shapes.Final
	default:
		return shapes.Independent
	}
}

func createArabicIsAlwaysInitial(letter rune) bool {
	switch letter {
	case '\u0627', '\u0623', '\u0622', '\u0625', '\u0649', '\u0621', '\u0624', '\u0629', '\u062F', '\u0630', '\u0631', '\u0632', '\u0648':
		return true
	default:
		return false
	}
}

func createArabicOnly(input string) bool {
	if strings.TrimSpace(input) == "" {
		return false
	}
	for _, r := range input {
		if !createIsArabicLetter(r) && !strings.ContainsRune(" \t\r\n", r) {
			return false
		}
	}
	return true
}

func createIsArabicLetter(r rune) bool {
	return r >= 0x0600 && r <= 0x06FF
}

func createReverseRunes(input string) string {
	runes := []rune(input)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}
