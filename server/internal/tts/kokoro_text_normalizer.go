//go:build kokoro

package tts

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	isoDateExprRe            = regexp.MustCompile(`\b(\d{4})[-/.](\d{1,2})[-/.](\d{1,2})\b`)
	usDateExprRe             = regexp.MustCompile(`\b(\d{1,2})/(\d{1,2})/(\d{2,4})\b`)
	timeExprRe               = regexp.MustCompile(`\b(\d{1,2}):([0-5]\d)\s*([AaPp][Mm])?\b`)
	currencySymbolExprRe     = regexp.MustCompile(`([$¥￥€£])\s*([0-9]{1,3}(?:,[0-9]{3})*(?:\.[0-9]+)?|[0-9]+(?:\.[0-9]+)?)([KMBkmb])?`)
	currencyCodeExprRe       = regexp.MustCompile(`(?i)\b(USD|CNY|JPY|EUR|GBP)\s*([0-9]{1,3}(?:,[0-9]{3})*(?:\.[0-9]+)?|[0-9]+(?:\.[0-9]+)?)([KMBkmb])?\b`)
	techUnitExprRe           = regexp.MustCompile(`(?i)\b([0-9]+(?:\.[0-9]+)?)\s*(km\/h|kmh|kph|mph|m\/s|ms|sec|secs|mins|min|hrs|hr|h|kg|mg|g|km|cm|mm|tb|gb|mb|ghz|mhz|khz|hz|kw|kwh|wh|mah|v|a|w|m)\b`)
	techCelsiusExprRe        = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)\s*(?:°[Cc]|℃)`)
	techPercentExprRe        = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)\s*%`)
	phoneExprRe              = regexp.MustCompile(`(?i)\b(?:\+?\d[\d .()\-]{6,}\d)(?:\s*(?:ext\.?|x|#)\s*\d+)?\b`)
	phoneExtensionExprRe     = regexp.MustCompile(`(?i)\b(?:ext\.?|x|#)\s*(\d+)\b`)
	zhStandaloneTwoExprRe    = regexp.MustCompile(`(^|[^0-9])2\s*(个|位|台|只|件|条|本|名|岁)`)
	zhStandaloneNumberExprRe = regexp.MustCompile(`\d+(?:[.,:/-]\d+)*`)
	weekdayTokenExprRe       = regexp.MustCompile(`(?i)\b(mon(?:day)?|tue(?:s(?:day)?)?|wed(?:nesday)?|thu(?:r(?:s(?:day)?)?)?|fri(?:day)?|sat(?:urday)?|sun(?:day)?)\b`)
	monthTokenExprRe         = regexp.MustCompile(`(?i)\b(jan(?:uary)?|feb(?:ruary)?|mar(?:ch)?|apr(?:il)?|may|jun(?:e)?|jul(?:y)?|aug(?:ust)?|sep(?:t(?:ember)?)?|oct(?:ober)?|nov(?:ember)?|dec(?:ember)?)\b`)
	zhAlphaNumVersionExprRe  = regexp.MustCompile(`\b([A-Za-z]+)([0-9]+(?:[.,:/-][0-9]+)*)\b`)
	englishMonthNames        = [...]string{"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"}
	englishTechnicalUnitEn   = map[string]string{
		"h":       "hours",
		"min":     "minutes",
		"s":       "seconds",
		"ms":      "milliseconds",
		"kph":     "kilometers per hour",
		"mph":     "miles per hour",
		"mps":     "meters per second",
		"kg":      "kilograms",
		"g":       "grams",
		"mg":      "milligrams",
		"km":      "kilometers",
		"m":       "meters",
		"cm":      "centimeters",
		"mm":      "millimeters",
		"tb":      "terabytes",
		"gb":      "gigabytes",
		"mb":      "megabytes",
		"hz":      "hertz",
		"khz":     "kilohertz",
		"mhz":     "megahertz",
		"ghz":     "gigahertz",
		"v":       "volts",
		"a":       "amps",
		"w":       "watts",
		"kw":      "kilowatts",
		"kwh":     "kilowatt hours",
		"wh":      "watt hours",
		"mah":     "milliamp hours",
		"celsius": "degrees celsius",
		"percent": "percent",
	}
)

func normalizeKokoroStructuredText(text string, language string) string {
	text = normalizeKokoroPunctuation(strings.TrimSpace(text))
	if text == "" {
		return ""
	}

	lang := normalizeKokoroLanguageHint(language)
	if lang == "" {
		lang = guessKokoroStructuredLanguageHint(text)
	}

	out := text
	out = expandCurrencyExpressions(out, lang)
	out = expandWeekdayExpressions(out, lang)
	out = expandMonthTokenExpressions(out, lang)
	out = expandDateExpressions(out, lang)
	out = expandTimeExpressions(out, lang)
	out = expandTechnicalUnitExpressions(out, lang)
	out = expandPhoneExpressions(out, lang)

	switch lang {
	case "zh":
		out = normalizeChineseTwoBeforeClassifier(out)
		out = expandZHAlphaNumericNumbers(out)
		out = expandZHStandaloneNumbers(out)
	case "en":
		out = expandAlphaNumericNumbersByLanguage(out, "en")
	}

	return collapseStructuredWhitespace(out)
}

func normalizeKokoroLanguageHint(language string) string {
	l := strings.ToLower(strings.TrimSpace(language))
	switch l {
	case "", "auto":
		return ""
	case "zh", "zh-cn", "zh-hans", "zh-tw", "zh-hant",
		"cmn", "mandarin",
		"chinese", "中文", "汉语", "漢語", "普通话", "普通話":
		return "zh"
	case "ja", "ja-jp", "japanese", "日本語", "にほんご":
		return "ja"
	case "ko", "ko-kr", "korean", "한국어", "조선말":
		return "ko"
	case "es", "es-es", "spanish", "español", "espanol", "西班牙语", "西班牙語":
		return "es"
	case "fr", "fr-fr", "french", "français", "francais", "法语", "法語":
		return "fr"
	case "hi", "hi-in", "hindi", "हिन्दी", "印地语", "印地語":
		return "hi"
	case "it", "it-it", "italian", "italiano", "意大利语", "義大利語":
		return "it"
	case "pt", "pt-br", "pt-pt", "portuguese", "português", "portugues", "葡萄牙语", "葡萄牙語":
		return "pt"
	case "en", "en-us", "en-gb", "english", "英语", "英語":
		return "en"
	default:
		return l
	}
}

func guessKokoroStructuredLanguageHint(text string) string {
	lang, _ := DetectLanguage(text)
	switch normalizeKokoroLanguageHint(lang) {
	case "zh", "ja", "ko", "es", "fr", "hi", "it", "pt", "en":
		return normalizeKokoroLanguageHint(lang)
	default:
		return "en"
	}
}

func normalizeKokoroPunctuation(text string) string {
	r := strings.NewReplacer(
		"\u3002", ".", // 。
		"\uff0c", ",", // ，
		"\uff01", "!", // ！
		"\uff1f", "?", // ？
		"\uff1b", ";", // ；
		"\uff1a", ":", // ：
		"\u201c", "\"",
		"\u201d", "\"",
		"\u2018", "'",
		"\u2019", "'",
		"\u3001", ",", // 、
		"\u2026", "...",
		"\uff08", "(",
		"\uff09", ")",
		"\u3010", "[",
		"\u3011", "]",
		"\u300a", "\"",
		"\u300b", "\"",
	)
	return strings.TrimSpace(r.Replace(text))
}

func collapseStructuredWhitespace(text string) string {
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.Join(strings.Fields(line), " ")
		if line != "" {
			out = append(out, line)
		}
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func expandWeekdayExpressions(text string, lang string) string {
	return replaceAllWithSubmatches(weekdayTokenExprRe, text, func(groups []string) string {
		name := canonicalEnglishWeekday(groups[1])
		if name == "" {
			return groups[0]
		}
		switch lang {
		case "zh":
			return englishWeekdayToChinese(name)
		case "ja":
			return englishWeekdayToJapanese(name)
		default:
			return name
		}
	})
}

func expandMonthTokenExpressions(text string, lang string) string {
	return replaceAllWithSubmatches(monthTokenExprRe, text, func(groups []string) string {
		month := englishMonthIndex(groups[1])
		if month < 1 || month > 12 {
			return groups[0]
		}
		switch lang {
		case "zh", "ja":
			return fmt.Sprintf("%d月", month)
		default:
			return englishMonthNames[month-1]
		}
	})
}

func canonicalEnglishWeekday(token string) string {
	t := strings.ToLower(strings.TrimSpace(token))
	switch t {
	case "mon", "monday":
		return "Monday"
	case "tue", "tues", "tuesday":
		return "Tuesday"
	case "wed", "wednesday":
		return "Wednesday"
	case "thu", "thur", "thurs", "thursday":
		return "Thursday"
	case "fri", "friday":
		return "Friday"
	case "sat", "saturday":
		return "Saturday"
	case "sun", "sunday":
		return "Sunday"
	default:
		return ""
	}
}

func englishWeekdayToChinese(name string) string {
	switch name {
	case "Monday":
		return "星期一"
	case "Tuesday":
		return "星期二"
	case "Wednesday":
		return "星期三"
	case "Thursday":
		return "星期四"
	case "Friday":
		return "星期五"
	case "Saturday":
		return "星期六"
	case "Sunday":
		return "星期日"
	default:
		return name
	}
}

func englishWeekdayToJapanese(name string) string {
	switch name {
	case "Monday":
		return "月曜日"
	case "Tuesday":
		return "火曜日"
	case "Wednesday":
		return "水曜日"
	case "Thursday":
		return "木曜日"
	case "Friday":
		return "金曜日"
	case "Saturday":
		return "土曜日"
	case "Sunday":
		return "日曜日"
	default:
		return name
	}
}

func englishMonthIndex(token string) int {
	t := strings.ToLower(strings.TrimSpace(token))
	switch t {
	case "jan", "january":
		return 1
	case "feb", "february":
		return 2
	case "mar", "march":
		return 3
	case "apr", "april":
		return 4
	case "may":
		return 5
	case "jun", "june":
		return 6
	case "jul", "july":
		return 7
	case "aug", "august":
		return 8
	case "sep", "sept", "september":
		return 9
	case "oct", "october":
		return 10
	case "nov", "november":
		return 11
	case "dec", "december":
		return 12
	default:
		return 0
	}
}

func expandZHAlphaNumericNumbers(text string) string {
	return replaceAllWithSubmatches(zhAlphaNumVersionExprRe, text, func(groups []string) string {
		prefix := groups[1]
		num := groups[2]
		if prefix == "" || num == "" {
			return groups[0]
		}
		return formatAlphaNumericSuffix(prefix, num, "zh")
	})
}

func expandZHStandaloneNumbers(text string) string {
	return replaceAllWithSubmatches(zhStandaloneNumberExprRe, text, func(groups []string) string {
		token := strings.TrimSpace(groups[0])
		if token == "" {
			return groups[0]
		}
		if han := mapDigitsToChineseHan(token); strings.TrimSpace(han) != "" {
			return han
		}
		return groups[0]
	})
}

func expandAlphaNumericNumbersByLanguage(text string, lang string) string {
	return replaceAllWithSubmatches(zhAlphaNumVersionExprRe, text, func(groups []string) string {
		prefix := groups[1]
		num := groups[2]
		if prefix == "" || num == "" {
			return groups[0]
		}
		return formatAlphaNumericSuffix(prefix, num, lang)
	})
}

func formatAlphaNumericSuffix(prefix string, num string, lang string) string {
	switch lang {
	case "zh":
		if isAllASCIIDigits(num) {
			if d := chineseDigitsContinuous(num); d != "" {
				return prefix + d
			}
			return prefix + num
		}
		han := mapDigitsToChineseHan(num)
		if strings.TrimSpace(han) == "" {
			return prefix + num
		}
		return prefix + han
	case "en":
		if isAllASCIIDigits(num) {
			return prefix + " " + englishModelNumberWords(num)
		}
		return prefix + " " + englishMixedNumberWords(num)
	default:
		if isAllASCIIDigits(num) {
			return prefix + " " + num
		}
		return prefix + num
	}
}

func englishModelNumberWords(digits string) string {
	if digits == "" {
		return ""
	}
	if len(digits) == 4 {
		a := digits[:2]
		b := digits[2:]
		if isAllASCIIDigits(a) && isAllASCIIDigits(b) {
			aw := englishTwoDigitWords(a)
			bw := englishTwoDigitWords(b)
			if aw != "" && bw != "" {
				return aw + " " + bw
			}
		}
	}
	return englishDigitsWords(digits)
}

func englishMixedNumberWords(text string) string {
	var out []string
	for i := 0; i < len(text); i++ {
		c := text[i]
		switch {
		case c >= '0' && c <= '9':
			out = append(out, englishDigitWord(c))
		case c == '.':
			out = append(out, "point")
		case c == '/':
			out = append(out, "slash")
		case c == '-':
			out = append(out, "dash")
		case c == ':':
			out = append(out, "colon")
		}
	}
	return strings.Join(out, " ")
}

func englishDigitsWords(digits string) string {
	if !isAllASCIIDigits(digits) {
		return ""
	}
	words := make([]string, 0, len(digits))
	for i := 0; i < len(digits); i++ {
		words = append(words, englishDigitWord(digits[i]))
	}
	return strings.Join(words, " ")
}

func englishDigitWord(d byte) string {
	switch d {
	case '0':
		return "zero"
	case '1':
		return "one"
	case '2':
		return "two"
	case '3':
		return "three"
	case '4':
		return "four"
	case '5':
		return "five"
	case '6':
		return "six"
	case '7':
		return "seven"
	case '8':
		return "eight"
	case '9':
		return "nine"
	default:
		return ""
	}
}

func englishTwoDigitWords(two string) string {
	if len(two) != 2 || !isAllASCIIDigits(two) {
		return ""
	}
	n, err := strconv.Atoi(two)
	if err != nil {
		return ""
	}
	if n < 10 {
		return englishDigitWord(two[1])
	}
	ones := []string{"", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine"}
	teens := []string{"ten", "eleven", "twelve", "thirteen", "fourteen", "fifteen", "sixteen", "seventeen", "eighteen", "nineteen"}
	tens := []string{"", "", "twenty", "thirty", "forty", "fifty", "sixty", "seventy", "eighty", "ninety"}
	if n < 20 {
		return teens[n-10]
	}
	t := n / 10
	o := n % 10
	if o == 0 {
		return tens[t]
	}
	return tens[t] + " " + ones[o]
}

func normalizeChineseTwoBeforeClassifier(text string) string {
	return replaceAllWithSubmatches(zhStandaloneTwoExprRe, text, func(groups []string) string {
		return groups[1] + "两" + groups[2]
	})
}

func expandDateExpressions(text string, lang string) string {
	out := replaceAllWithSubmatches(isoDateExprRe, text, func(groups []string) string {
		year, okY := parseIntRange(groups[1], 0, 9999)
		month, okM := parseIntRange(groups[2], 1, 12)
		day, okD := parseIntRange(groups[3], 1, 31)
		if !okY || !okM || !okD {
			return groups[0]
		}
		return renderDateByLanguage(lang, year, month, day)
	})
	out = replaceAllWithSubmatches(usDateExprRe, out, func(groups []string) string {
		month, okM := parseIntRange(groups[1], 1, 12)
		day, okD := parseIntRange(groups[2], 1, 31)
		year, okY := parseIntRange(groups[3], 0, 9999)
		if !okM || !okD || !okY {
			return groups[0]
		}
		if len(groups[3]) == 2 {
			year += 2000
		}
		return renderDateByLanguage(lang, year, month, day)
	})
	return out
}

func expandTimeExpressions(text string, lang string) string {
	return replaceAllWithSubmatches(timeExprRe, text, func(groups []string) string {
		hour, okH := parseIntRange(groups[1], 0, 23)
		minute, okM := parseIntRange(groups[2], 0, 59)
		if !okH || !okM {
			return groups[0]
		}
		marker := strings.ToUpper(strings.TrimSpace(groups[3]))
		switch lang {
		case "zh":
			return renderChineseTime(hour, minute, marker)
		case "ja":
			return renderJapaneseTime(hour, minute, marker)
		default:
			return renderDefaultTime(hour, minute, marker)
		}
	})
}

func expandCurrencyExpressions(text string, lang string) string {
	out := replaceAllWithSubmatches(currencyCodeExprRe, text, func(groups []string) string {
		repl := formatCurrencyExpansion(lang, groups[2], groups[3], groups[1])
		if repl == "" {
			return groups[0]
		}
		return repl
	})
	out = replaceAllWithSubmatches(currencySymbolExprRe, out, func(groups []string) string {
		repl := formatCurrencyExpansion(lang, groups[2], groups[3], groups[1])
		if repl == "" {
			return groups[0]
		}
		return repl
	})
	return out
}

func expandTechnicalUnitExpressions(text string, lang string) string {
	out := replaceAllWithSubmatches(techUnitExprRe, text, func(groups []string) string {
		amount := sanitizeNumericLiteral(groups[1])
		if amount == "" {
			return groups[0]
		}
		unit := canonicalTechnicalUnit(groups[2])
		return formatTechnicalUnitExpansion(lang, amount, unit, groups[0])
	})
	out = replaceAllWithSubmatches(techCelsiusExprRe, out, func(groups []string) string {
		amount := sanitizeNumericLiteral(groups[1])
		if amount == "" {
			return groups[0]
		}
		return formatTechnicalUnitExpansion(lang, amount, "celsius", groups[0])
	})
	out = replaceAllWithSubmatches(techPercentExprRe, out, func(groups []string) string {
		amount := sanitizeNumericLiteral(groups[1])
		if amount == "" {
			return groups[0]
		}
		return formatTechnicalUnitExpansion(lang, amount, "percent", groups[0])
	})
	return out
}

func expandPhoneExpressions(text string, lang string) string {
	return replaceAllWithSubmatches(phoneExprRe, text, func(groups []string) string {
		match := groups[0]
		if strings.Contains(match, ":") || isoDateExprRe.MatchString(match) || usDateExprRe.MatchString(match) {
			return match
		}
		base := match
		extDigits := ""
		if extLoc := phoneExtensionExprRe.FindStringSubmatchIndex(match); len(extLoc) >= 4 {
			base = strings.TrimSpace(match[:extLoc[0]])
			extDigits = extractASCIIDigits(match[extLoc[2]:extLoc[3]])
		}
		mainDigits := extractASCIIDigits(base)
		if len(mainDigits) < 7 {
			return match
		}
		main := spellDigitsByLanguage(mainDigits, lang)
		if main == "" {
			return match
		}
		if extDigits == "" {
			return main
		}
		ext := spellDigitsByLanguage(extDigits, lang)
		switch lang {
		case "zh":
			return main + " 分机 " + ext
		case "ja":
			return main + " 内線 " + ext
		default:
			return main + " extension " + ext
		}
	})
}

func replaceAllWithSubmatches(re *regexp.Regexp, text string, repl func(groups []string) string) string {
	matches := re.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return text
	}

	var out strings.Builder
	last := 0
	for _, m := range matches {
		out.WriteString(text[last:m[0]])
		groups := make([]string, len(m)/2)
		for i := 0; i < len(m)/2; i++ {
			start := m[2*i]
			end := m[2*i+1]
			if start >= 0 && end >= 0 {
				groups[i] = text[start:end]
			}
		}
		out.WriteString(repl(groups))
		last = m[1]
	}
	out.WriteString(text[last:])
	return out.String()
}

func parseIntRange(raw string, min int, max int) (int, bool) {
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || v < min || v > max {
		return 0, false
	}
	return v, true
}

func renderDateByLanguage(lang string, year int, month int, day int) string {
	switch lang {
	case "zh", "ja":
		return fmt.Sprintf("%d年%d月%d日", year, month, day)
	default:
		if month < 1 || month > 12 {
			return fmt.Sprintf("%d-%d-%d", year, month, day)
		}
		return fmt.Sprintf("%s %d %d", englishMonthNames[month-1], day, year)
	}
}

func renderChineseTime(hour int, minute int, marker string) string {
	prefix := ""
	displayHour := hour
	switch marker {
	case "AM":
		prefix = "上午"
		if displayHour == 0 {
			displayHour = 12
		}
		if displayHour > 12 {
			displayHour -= 12
		}
	case "PM":
		prefix = "下午"
		if displayHour == 0 {
			displayHour = 12
		}
		if displayHour > 12 {
			displayHour -= 12
		}
	}
	if minute == 0 {
		return fmt.Sprintf("%s%d点", prefix, displayHour)
	}
	return fmt.Sprintf("%s%d点%d分", prefix, displayHour, minute)
}

func renderJapaneseTime(hour int, minute int, marker string) string {
	prefix := ""
	displayHour := hour
	switch marker {
	case "AM":
		prefix = "午前"
		if displayHour == 0 {
			displayHour = 12
		}
		if displayHour > 12 {
			displayHour -= 12
		}
	case "PM":
		prefix = "午後"
		if displayHour == 0 {
			displayHour = 12
		}
		if displayHour > 12 {
			displayHour -= 12
		}
	}
	if minute == 0 {
		return fmt.Sprintf("%s%d時", prefix, displayHour)
	}
	return fmt.Sprintf("%s%d時%d分", prefix, displayHour, minute)
}

func renderDefaultTime(hour int, minute int, marker string) string {
	displayHour := hour
	if marker != "" && displayHour > 12 {
		displayHour -= 12
	}
	if marker != "" {
		return fmt.Sprintf("%d %02d %s", displayHour, minute, marker)
	}
	return fmt.Sprintf("%d %02d", displayHour, minute)
}

func sanitizeNumericLiteral(raw string) string {
	return strings.ReplaceAll(strings.TrimSpace(raw), ",", "")
}

func currencyMultiplierWord(lang string, suffix string) string {
	switch strings.ToUpper(strings.TrimSpace(suffix)) {
	case "K":
		switch lang {
		case "zh", "ja":
			return "千"
		default:
			return "thousand"
		}
	case "M":
		switch lang {
		case "zh", "ja":
			return "百万"
		default:
			return "million"
		}
	case "B":
		switch lang {
		case "zh":
			return "十亿"
		case "ja":
			return "十億"
		default:
			return "billion"
		}
	default:
		return ""
	}
}

func currencyNameByLanguage(lang string, currency string) string {
	key := strings.ToLower(strings.TrimSpace(currency))
	switch lang {
	case "zh":
		switch key {
		case "$", "usd":
			return "美元"
		case "¥", "￥", "cny":
			return "元"
		case "jpy":
			return "日元"
		case "€", "eur":
			return "欧元"
		case "£", "gbp":
			return "英镑"
		}
	case "ja":
		switch key {
		case "$", "usd":
			return "ドル"
		case "¥", "￥", "jpy":
			return "円"
		case "cny":
			return "人民元"
		case "€", "eur":
			return "ユーロ"
		case "£", "gbp":
			return "ポンド"
		}
	default:
		switch key {
		case "$":
			return "dollars"
		case "¥", "￥":
			return "yen"
		case "€":
			return "euros"
		case "£":
			return "pounds"
		case "usd":
			return "US dollars"
		case "cny":
			return "yuan"
		case "jpy":
			return "Japanese yen"
		case "eur":
			return "euros"
		case "gbp":
			return "British pounds"
		}
	}
	return ""
}

func formatCurrencyExpansion(lang string, amountRaw string, multiplierRaw string, currencyRaw string) string {
	amount := sanitizeNumericLiteral(amountRaw)
	if amount == "" {
		return ""
	}
	multiplier := currencyMultiplierWord(lang, multiplierRaw)
	currency := currencyNameByLanguage(lang, currencyRaw)
	switch lang {
	case "zh", "ja":
		return strings.TrimSpace(amount + multiplier + currency)
	default:
		parts := make([]string, 0, 3)
		parts = append(parts, amount)
		if multiplier != "" {
			parts = append(parts, multiplier)
		}
		if currency != "" {
			parts = append(parts, currency)
		}
		return strings.Join(parts, " ")
	}
}

func canonicalTechnicalUnit(unit string) string {
	u := strings.ToLower(strings.TrimSpace(unit))
	switch u {
	case "km/h", "kmh", "kph":
		return "kph"
	case "m/s":
		return "mps"
	case "hr", "hrs":
		return "h"
	case "min", "mins":
		return "min"
	case "sec", "secs":
		return "s"
	default:
		return u
	}
}

func technicalUnitByLanguage(lang string, unit string) string {
	unit = canonicalTechnicalUnit(unit)
	switch lang {
	case "zh":
		switch unit {
		case "h":
			return "小时"
		case "min":
			return "分钟"
		case "s":
			return "秒"
		case "ms":
			return "毫秒"
		case "kph":
			return "公里每小时"
		case "mph":
			return "英里每小时"
		case "mps":
			return "米每秒"
		case "kg":
			return "千克"
		case "g":
			return "克"
		case "mg":
			return "毫克"
		case "km":
			return "千米"
		case "m":
			return "米"
		case "cm":
			return "厘米"
		case "mm":
			return "毫米"
		case "tb":
			return "太字节"
		case "gb":
			return "吉字节"
		case "mb":
			return "兆字节"
		case "hz":
			return "赫兹"
		case "khz":
			return "千赫兹"
		case "mhz":
			return "兆赫兹"
		case "ghz":
			return "吉赫兹"
		case "v":
			return "伏特"
		case "a":
			return "安培"
		case "w":
			return "瓦"
		case "kw":
			return "千瓦"
		case "kwh":
			return "千瓦时"
		case "wh":
			return "瓦时"
		case "mah":
			return "毫安时"
		case "celsius":
			return "摄氏度"
		case "percent":
			return "百分之"
		}
	case "ja":
		switch unit {
		case "h":
			return "時間"
		case "min":
			return "分"
		case "s":
			return "秒"
		case "ms":
			return "ミリ秒"
		case "kph":
			return "キロメートル毎時"
		case "mph":
			return "マイル毎時"
		case "mps":
			return "メートル毎秒"
		case "kg":
			return "キログラム"
		case "g":
			return "グラム"
		case "mg":
			return "ミリグラム"
		case "km":
			return "キロメートル"
		case "m":
			return "メートル"
		case "cm":
			return "センチメートル"
		case "mm":
			return "ミリメートル"
		case "tb":
			return "テラバイト"
		case "gb":
			return "ギガバイト"
		case "mb":
			return "メガバイト"
		case "hz":
			return "ヘルツ"
		case "khz":
			return "キロヘルツ"
		case "mhz":
			return "メガヘルツ"
		case "ghz":
			return "ギガヘルツ"
		case "v":
			return "ボルト"
		case "a":
			return "アンペア"
		case "w":
			return "ワット"
		case "kw":
			return "キロワット"
		case "kwh":
			return "キロワット時"
		case "wh":
			return "ワット時"
		case "mah":
			return "ミリアンペア時"
		case "celsius":
			return "摂氏"
		case "percent":
			return "パーセント"
		}
	default:
		return englishTechnicalUnitEn[unit]
	}
	return ""
}

func formatTechnicalUnitExpansion(lang string, amount string, unit string, fallback string) string {
	label := technicalUnitByLanguage(lang, unit)
	if label == "" {
		return fallback
	}
	if unit == "percent" {
		switch lang {
		case "zh":
			return label + amount
		default:
			return amount + " " + label
		}
	}
	switch lang {
	case "zh", "ja":
		return amount + label
	default:
		return amount + " " + label
	}
}

func extractASCIIDigits(text string) string {
	var b strings.Builder
	for _, r := range text {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func spellDigitsByLanguage(digits string, lang string) string {
	if digits == "" {
		return ""
	}
	words := make([]string, 0, len(digits))
	for i := 0; i < len(digits); i++ {
		d := digits[i]
		switch lang {
		case "zh":
			switch d {
			case '0':
				words = append(words, "零")
			case '1':
				words = append(words, "一")
			case '2':
				words = append(words, "二")
			case '3':
				words = append(words, "三")
			case '4':
				words = append(words, "四")
			case '5':
				words = append(words, "五")
			case '6':
				words = append(words, "六")
			case '7':
				words = append(words, "七")
			case '8':
				words = append(words, "八")
			case '9':
				words = append(words, "九")
			}
		case "ja":
			switch d {
			case '0':
				words = append(words, "ゼロ")
			case '1':
				words = append(words, "イチ")
			case '2':
				words = append(words, "ニ")
			case '3':
				words = append(words, "サン")
			case '4':
				words = append(words, "ヨン")
			case '5':
				words = append(words, "ゴ")
			case '6':
				words = append(words, "ロク")
			case '7':
				words = append(words, "ナナ")
			case '8':
				words = append(words, "ハチ")
			case '9':
				words = append(words, "キュウ")
			}
		default:
			words = append(words, string(d))
		}
	}
	return strings.Join(words, " ")
}

func mapDigitsToChineseHan(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	normalized := strings.ReplaceAll(token, ",", "")
	if normalized == "" {
		return ""
	}

	if strings.Count(normalized, ":") == 1 {
		parts := strings.SplitN(normalized, ":", 2)
		if isAllASCIIDigits(parts[0]) && isAllASCIIDigits(parts[1]) {
			left := chineseIntegerOrDigitsHan(parts[0])
			right := chineseIntegerOrDigitsHan(parts[1])
			if left != "" && right != "" {
				return left + "点" + right
			}
		}
	}

	if strings.Count(normalized, ".") == 1 {
		parts := strings.SplitN(normalized, ".", 2)
		if isAllASCIIDigits(parts[0]) && isAllASCIIDigits(parts[1]) {
			left := chineseIntegerOrDigitsHan(parts[0])
			if left == "" {
				left = "零"
			}
			right := chineseDigitsContinuous(parts[1])
			if right != "" {
				return left + "点" + right
			}
		}
	}

	if isAllASCIIDigits(normalized) {
		if v := chineseIntegerOrDigitsHan(normalized); v != "" {
			return v
		}
	}

	return mapDigitsByCharToChineseHan(normalized)
}

func mapDigitsByCharToChineseHan(token string) string {
	var b strings.Builder
	for i := 0; i < len(token); i++ {
		switch token[i] {
		case '0':
			b.WriteRune('零')
		case '1':
			b.WriteRune('一')
		case '2':
			b.WriteRune('二')
		case '3':
			b.WriteRune('三')
		case '4':
			b.WriteRune('四')
		case '5':
			b.WriteRune('五')
		case '6':
			b.WriteRune('六')
		case '7':
			b.WriteRune('七')
		case '8':
			b.WriteRune('八')
		case '9':
			b.WriteRune('九')
		case '.', ':':
			b.WriteRune('点')
		case '/', '-':
			b.WriteRune('杠')
		case '+':
			b.WriteRune('加')
		}
	}
	return b.String()
}

func isAllASCIIDigits(text string) bool {
	if text == "" {
		return false
	}
	for i := 0; i < len(text); i++ {
		if text[i] < '0' || text[i] > '9' {
			return false
		}
	}
	return true
}

func chineseDigitsContinuous(digits string) string {
	if digits == "" {
		return ""
	}
	var b strings.Builder
	for i := 0; i < len(digits); i++ {
		switch digits[i] {
		case '0':
			b.WriteRune('零')
		case '1':
			b.WriteRune('一')
		case '2':
			b.WriteRune('二')
		case '3':
			b.WriteRune('三')
		case '4':
			b.WriteRune('四')
		case '5':
			b.WriteRune('五')
		case '6':
			b.WriteRune('六')
		case '7':
			b.WriteRune('七')
		case '8':
			b.WriteRune('八')
		case '9':
			b.WriteRune('九')
		}
	}
	return b.String()
}

func chineseIntegerOrDigitsHan(digits string) string {
	if !isAllASCIIDigits(digits) {
		return ""
	}
	if len(digits) > 1 && digits[0] == '0' {
		return chineseDigitsContinuous(digits)
	}
	if len(digits) == 4 {
		if y, err := strconv.Atoi(digits); err == nil && y >= 1000 && y <= 2999 {
			return chineseDigitsContinuous(digits)
		}
	}
	n, err := strconv.ParseInt(digits, 10, 64)
	if err != nil {
		return chineseDigitsContinuous(digits)
	}
	if n == 0 {
		return "零"
	}
	if n < 0 {
		return "负" + chineseIntegerToHan(-n)
	}
	return chineseIntegerToHan(n)
}

func chineseIntegerToHan(n int64) string {
	if n == 0 {
		return "零"
	}
	orig := n
	bigUnits := []string{"", "万", "亿", "兆"}
	groups := make([]int, 0, 4)
	for n > 0 {
		groups = append(groups, int(n%10000))
		n /= 10000
	}
	if len(groups) > len(bigUnits) {
		return chineseDigitsContinuous(strconv.FormatInt(orig, 10))
	}

	var out strings.Builder
	zeroPending := false
	for i := len(groups) - 1; i >= 0; i-- {
		group := groups[i]
		if group == 0 {
			if out.Len() > 0 {
				zeroPending = true
			}
			continue
		}
		if out.Len() > 0 && (zeroPending || group < 1000) {
			out.WriteRune('零')
		}
		out.WriteString(chineseGroupToHan(group))
		out.WriteString(bigUnits[i])
		zeroPending = false
	}
	return out.String()
}

func chineseGroupToHan(group int) string {
	if group <= 0 || group > 9999 {
		return ""
	}
	digits := []string{"零", "一", "二", "三", "四", "五", "六", "七", "八", "九"}
	units := []string{"千", "百", "十", ""}
	values := []int{1000, 100, 10, 1}

	var out strings.Builder
	zeroPending := false
	for i, v := range values {
		d := group / v
		group %= v
		if d == 0 {
			if out.Len() > 0 {
				zeroPending = true
			}
			continue
		}
		if zeroPending {
			out.WriteRune('零')
			zeroPending = false
		}
		if !(v == 10 && d == 1 && out.Len() == 0) {
			out.WriteString(digits[d])
		}
		out.WriteString(units[i])
	}
	return out.String()
}
