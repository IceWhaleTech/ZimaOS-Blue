package tts

import "strings"

// fromEspeak converts eSpeak IPA output to misaki/Kokoro phoneme format.
// Applied after eSpeak's TextToPhonemes for English words.
func fromEspeak(ipa string) string {
	if ipa == "" {
		return ""
	}

	// Multi-char replacements (longest first)
	r := strings.NewReplacer(
		"aɪ", "I",
		"aʊ", "W",
		"dʒ", "ʤ",
		"eɪ", "A",
		"tʃ", "ʧ",
		"ɔɪ", "Y",
		"oʊ", "O",
		"əl", "ᵊl",
		"ɜːɹ", "ɜɹ",
		"ɜː", "ɜɹ",
		"ɪə", "iə",
		"ɚ", "əɹ",
	)
	ipa = r.Replace(ipa)

	// Single-char replacements
	r2 := strings.NewReplacer(
		"r", "ɹ",
		"x", "k",
		"ç", "k",
		"ɐ", "ə",
		"ɬ", "l",
		"ɾ", "T",
		"ʔ", "t",
		"ʲ", "",
		"\u0303", "", // combining tilde
		"ː", "",     // remove length marks for US English
	)
	ipa = r2.Replace(ipa)

	return ipa
}
