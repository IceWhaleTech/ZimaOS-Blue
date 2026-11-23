package querylang

import (
	"path"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/abadojack/whatlanggo"
	"golang.org/x/text/unicode/norm"
)

var queryLanguageTypography = strings.NewReplacer(
	"’", "'", "‘", "'", "ʼ", "'",
	"\u200b", " ", "\u2060", "", "\ufeff", "",
)

// DetectQueryLanguage returns a supported language only when the local detector
// has enough evidence to override the UI preference. Empty means uncertain;
// ambiguous keywords, filenames and comparable mixed scripts retain that locale.
// Prefer coverage for supported languages while retaining evidence thresholds.
func DetectQueryLanguage(query string) string {
	if len(query) > 4096 || !utf8.ValidString(query) {
		return ""
	}
	query = strings.TrimSpace(languageIDUnquotedText(queryLanguageTypography.Replace(norm.NFKC.String(query))))
	if strings.ContainsAny(query, "/\\") {
		return ""
	}
	if extension := path.Ext(query); len(extension) > 1 && !strings.ContainsAny(extension, " \t\n\r") {
		return ""
	}
	var letters, latin, han, kana, kanaModifiers, hangul, greek, malayalam, cyrillic int
	for _, r := range query {
		if !unicode.IsLetter(r) {
			continue
		}
		letters++
		switch {
		case unicode.In(r, unicode.Latin):
			latin++
		case unicode.In(r, unicode.Han):
			han++
		case unicode.In(r, unicode.Hiragana, unicode.Katakana):
			kana++
		case r == 'ー' || r == 'ｰ' || r == 'ﾞ' || r == 'ﾟ':
			// Unicode assigns these Japanese sound marks to Common, even
			// though they are letters. They require actual kana as evidence.
			kanaModifiers++
		case unicode.In(r, unicode.Hangul):
			hangul++
		case unicode.In(r, unicode.Greek):
			greek++
		case unicode.In(r, unicode.Malayalam):
			malayalam++
		case unicode.In(r, unicode.Cyrillic):
			cyrillic++
		}
	}
	if letters == 0 {
		return ""
	}
	switch {
	case kana > 0 && kana+han+kanaModifiers == letters:
		return "ja"
	case hangul > 0 && hangul+han == letters:
		return "ko"
	case han == letters && hasSimplifiedChineseEvidence(query):
		return "zh"
	case han == letters && han < 3:
		return ""
	case greek == letters:
		return "el"
	case malayalam == letters:
		return "ml"
	}
	model, err := builtinLanguageID()
	if err != nil {
		if han == letters && han >= 4 {
			return "zh"
		}
		return ""
	}
	query = strings.Join(strings.Fields(query), " ")
	if query == strings.ToUpper(query) {
		query = strings.ToLower(query)
	}
	predictions := model.predict(query)
	best := predictions[0]
	margin := best.Probability - predictions[1].Probability
	if han == letters && han >= 4 && !(best.Language == "ja" && best.Probability >= 0.9 && margin >= 0.2) {
		// Shared Han sentences retain the Chinese route unless the model has
		// strong Japanese evidence. The planner still honors ja/ko preferences.
		return "zh"
	}
	words := len(strings.Fields(query))
	corroborated := false
	// Standard Turkish has neither ə nor q. Require the Azerbaijani model to
	// remain among the candidates before using that alphabet evidence.
	if latin == letters && best.Language == "tr" && strings.ContainsAny(query, "əƏq") {
		for _, candidate := range predictions {
			if candidate.Language == "az" && candidate.Probability >= 0.02 {
				best, corroborated = candidate, true
				break
			}
		}
	}
	// A weak winning profile need not discard a supported runner-up when an
	// independent, confident detector agrees on a sufficiently long phrase.
	if !corroborated && latin == letters && words >= 3 && letters >= 12 && (best.Probability < 0.6 || margin < 0.2) {
		other := whatlanggo.Detect(query)
		language := other.Lang.Iso6391()
		if language == "nb" {
			language = "no"
		}
		if other.Confidence >= 0.8 {
			for _, candidate := range predictions {
				// English passthrough must retain the normal model floor: a
				// weak English profile otherwise suppresses needed translation.
				if candidate.Language == language && candidate.Probability >= 0.15 && (language != "en" || candidate.Probability >= 0.4) {
					best, corroborated = candidate, true
					break
				}
			}
		}
	}
	if !corroborated && (best.Probability < 0.4 || margin < 0.1) {
		return ""
	}
	language := best.Language
	if language == "no" {
		language = "nb"
	}
	// Strong model predictions can stand alone. Weaker ones need independent
	// trigram agreement; its short-query confidence is not a second cutoff.
	if !corroborated && (best.Probability < 0.6 || margin < 0.2) {
		// These three supported languages have no auxiliary trigram profile.
		withoutProfile := language == "ga" || language == "ca" || language == "sk"
		if !(withoutProfile && words >= 3 && margin >= 0.2) && whatlanggo.Detect(query).Lang.Iso6391() != language {
			return ""
		}
	}
	scriptLetters := map[string]int{"zh": han, "ja": han + kana, "ko": han + hangul, "el": greek, "ml": malayalam, "ru": cyrillic}[language]
	if language == "ja" && kana > 0 {
		scriptLetters += kanaModifiers
	}
	if latin > 0 && latin != letters {
		// A strong non-Latin majority can contain a Latin camera/brand name.
		// Comparable scripts or a stray foreign character remain ambiguous.
		if best.Probability < 0.9 || scriptLetters*5 < letters*3 || scriptLetters+latin != letters {
			return ""
		}
	} else if latin == letters {
		if letters < 4 || (words == 1 && best.Probability < 0.9) {
			return ""
		}
	} else if scriptLetters != letters {
		return ""
	}
	// Keep all detector profiles in competition: excluding unsupported languages
	// would force their text into an unrelated, downloadable NMT language.
	switch language {
	case "en", "it", "nb", "zh", "ja", "ko", "fr", "de", "az", "sv", "el", "hr", "pt", "ru", "tr", "sk", "ro", "pl", "nl", "ml", "hu", "ga", "es", "da", "cs", "ca":
		return language
	default:
		return ""
	}
}

// Literal text supplies no evidence for the language of the containing query:
// a French request for a sign reading "STOP" is still a French request.
func languageIDUnquotedText(query string) string {
	var result strings.Builder
	var closing rune
	for _, r := range query {
		if closing != 0 {
			if r == closing {
				closing = 0
				result.WriteByte(' ')
			}
			continue
		}
		switch r {
		case '"':
			closing = '"'
		case '«':
			closing = '»'
		case '「':
			closing = '」'
		case '『':
			closing = '』'
		case '“':
			closing = '”'
		default:
			result.WriteRune(r)
		}
	}
	if closing != 0 {
		return ""
	}
	return result.String()
}
