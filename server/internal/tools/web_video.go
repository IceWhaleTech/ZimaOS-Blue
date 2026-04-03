package tools

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/stt"
	"golang.org/x/text/language"
)

const (
	webQueryVideoPlatformYouTube     = "youtube"
	webQueryVideoPlatformBilibili    = "bilibili"
	webQueryVideoPlatformDirectMedia = "direct_media"

	webQueryMediaKindVideo = "video"
	webQueryMediaKindAudio = "audio"

	webQueryTranscriptStatusOK          = "ok"
	webQueryTranscriptStatusPartial     = "partial"
	webQueryTranscriptStatusUnavailable = "unavailable"

	webQueryMaxVideoSearchCandidates = 3
	webQueryMaxMediaDownloadBytes    = 64 << 20
)

type webQueryVideoTarget struct {
	URL         string
	Platform    string
	DirectMedia bool
	MediaKind   string
	VideoID     string
}

type webQuerySubtitleCandidate struct {
	URL       string
	Language  string
	Label     string
	Source    string
	Generated bool
}

type webQueryParsedVideo struct {
	Media              *webQueryMedia
	TranscriptLanguage string
	SubtitleCandidates []webQuerySubtitleCandidate
	AudioURL           string
}

type webQueryVideoResolveResult struct {
	Target              webQueryVideoTarget
	Media               *webQueryMedia
	Transcript          *webQueryTranscript
	Warnings            []webQueryWarning
	Attempts            []webQueryAttempt
	QualityScore        float64
	TranscriptSourceURL string
}

type webQueryVideoCandidateOutcome struct {
	Index  int
	Result webQueryVideoResolveResult
}

type webQueryLanguagePreference struct {
	Raw       string
	Base      string
	Variants  []string
	Fallbacks []string
}

type webQueryTranscriptLocaleProfile struct {
	Canonical string
	Aliases   []string
}

type webQueryLanguageLabelHint struct {
	Language string
	Patterns []string
}

var (
	webQueryTranscriptLocaleProfiles = []webQueryTranscriptLocaleProfile{
		{Canonical: "ca-es", Aliases: []string{"ca"}},
		{Canonical: "cs-cz", Aliases: []string{"cs"}},
		{Canonical: "da-dk", Aliases: []string{"da"}},
		{Canonical: "de-de", Aliases: []string{"de"}},
		{Canonical: "el-gr", Aliases: []string{"el"}},
		{Canonical: "en-us", Aliases: []string{"en"}},
		{Canonical: "en-gb", Aliases: []string{"en-uk"}},
		{Canonical: "es-es", Aliases: []string{"es"}},
		{Canonical: "fr-fr", Aliases: []string{"fr"}},
		{Canonical: "ga-ie", Aliases: []string{"ga"}},
		{Canonical: "hr-hr", Aliases: []string{"hr"}},
		{Canonical: "hu-hu", Aliases: []string{"hu"}},
		{Canonical: "it-it", Aliases: []string{"it"}},
		{Canonical: "ja-jp", Aliases: []string{"ja"}},
		{Canonical: "ko-kr", Aliases: []string{"ko"}},
		{Canonical: "ml-in", Aliases: []string{"ml"}},
		{Canonical: "nb-no", Aliases: []string{"nb", "no", "no-no"}},
		{Canonical: "nl-nl", Aliases: []string{"nl"}},
		{Canonical: "pl-pl", Aliases: []string{"pl"}},
		{Canonical: "pt-br", Aliases: []string{"pt", "pt-latn", "pt-latn-br"}},
		{Canonical: "pt-pt", Aliases: []string{"pt-latn-pt"}},
		{Canonical: "ro-ro", Aliases: []string{"ro"}},
		{Canonical: "ru-ru", Aliases: []string{"ru"}},
		{Canonical: "sk-sk", Aliases: []string{"sk"}},
		{Canonical: "sv-se", Aliases: []string{"sv"}},
		{Canonical: "zh-cn", Aliases: []string{
			"zh", "zh-hans", "zh-hans-cn", "zh-chs",
			"cmn", "cmn-hans", "cmn-hans-cn",
			"zh-sg", "zh-hans-sg",
		}},
		{Canonical: "zh-tw", Aliases: []string{
			"zh-hant", "zh-hant-tw",
			"zh-hk", "zh-hant-hk",
			"zh-mo", "zh-hant-mo",
			"zh-cht",
			"cmn-hant", "cmn-hant-tw",
		}},
	}
	webQueryTranscriptLocaleProfileByCanonical, webQueryTranscriptLocaleCanonicalByAlias = buildWebQueryTranscriptLocaleIndexes()
	webQueryLanguageLabelHints                                                           = []webQueryLanguageLabelHint{
		{Language: "ca-es", Patterns: []string{"catalan", "catala", "català"}},
		{Language: "cs-cz", Patterns: []string{"czech", "cestina", "čeština"}},
		{Language: "da-dk", Patterns: []string{"danish", "dansk"}},
		{Language: "de-de", Patterns: []string{"german", "deutsch"}},
		{Language: "el-gr", Patterns: []string{"greek", "ellinika", "ελληνικά"}},
		{Language: "en-gb", Patterns: []string{"english uk", "english gb", "british english"}},
		{Language: "en-us", Patterns: []string{"english us", "american english", "english"}},
		{Language: "es-es", Patterns: []string{"spanish", "espanol", "español", "castellano"}},
		{Language: "fr-fr", Patterns: []string{"french", "francais", "français"}},
		{Language: "ga-ie", Patterns: []string{"irish", "gaeilge"}},
		{Language: "hr-hr", Patterns: []string{"croatian", "hrvatski"}},
		{Language: "hu-hu", Patterns: []string{"hungarian", "magyar"}},
		{Language: "it-it", Patterns: []string{"italian", "italiano"}},
		{Language: "ja-jp", Patterns: []string{"japanese", "日本語"}},
		{Language: "ko-kr", Patterns: []string{"korean", "한국어"}},
		{Language: "ml-in", Patterns: []string{"malayalam", "മലയാളം"}},
		{Language: "nb-no", Patterns: []string{"norwegian", "norsk bokmal", "norsk bokmål", "bokmal", "bokmål"}},
		{Language: "nl-nl", Patterns: []string{"dutch", "nederlands"}},
		{Language: "pl-pl", Patterns: []string{"polish", "polski"}},
		{Language: "pt-br", Patterns: []string{"brazilian portuguese", "portuguese brazil", "portugues brasil", "português brasil"}},
		{Language: "pt-pt", Patterns: []string{"european portuguese", "portuguese portugal", "portugues portugal", "português portugal"}},
		{Language: "ro-ro", Patterns: []string{"romanian", "romana", "română"}},
		{Language: "ru-ru", Patterns: []string{"russian", "русский"}},
		{Language: "sk-sk", Patterns: []string{"slovak", "slovencina", "slovenčina"}},
		{Language: "sv-se", Patterns: []string{"swedish", "svenska"}},
		{Language: "zh-tw", Patterns: []string{"traditional chinese", "chinese traditional", "繁體中文", "繁体中文"}},
		{Language: "zh-cn", Patterns: []string{"simplified chinese", "chinese simplified", "简体中文", "簡體中文", "chinese", "中文"}},
	}
)

func buildWebQueryTranscriptLocaleIndexes() (map[string]webQueryTranscriptLocaleProfile, map[string]string) {
	profileByCanonical := make(map[string]webQueryTranscriptLocaleProfile, len(webQueryTranscriptLocaleProfiles))
	canonicalByAlias := make(map[string]string, len(webQueryTranscriptLocaleProfiles)*3)
	for _, profile := range webQueryTranscriptLocaleProfiles {
		canonical := normalizeWebQueryLanguageTag(profile.Canonical)
		if canonical == "" {
			continue
		}
		normalizedProfile := webQueryTranscriptLocaleProfile{
			Canonical: canonical,
			Aliases:   make([]string, 0, len(profile.Aliases)),
		}
		profileByCanonical[canonical] = normalizedProfile
		if _, exists := canonicalByAlias[canonical]; !exists {
			canonicalByAlias[canonical] = canonical
		}
		for _, alias := range profile.Aliases {
			normalizedAlias := normalizeWebQueryLanguageTag(alias)
			if normalizedAlias == "" {
				continue
			}
			normalizedProfile.Aliases = append(normalizedProfile.Aliases, normalizedAlias)
			if _, exists := canonicalByAlias[normalizedAlias]; !exists {
				canonicalByAlias[normalizedAlias] = canonical
			}
		}
		profileByCanonical[canonical] = normalizedProfile
	}
	return profileByCanonical, canonicalByAlias
}

func isWebQueryUsableLanguageTag(raw string) bool {
	normalized := normalizeWebQueryLanguageTag(raw)
	if normalized == "" {
		return false
	}
	parts := strings.Split(normalized, "-")
	if len(parts) == 0 || len(parts) > 3 {
		return false
	}
	tag, err := language.Parse(normalized)
	if err != nil {
		return false
	}
	base, confidence := tag.Base()
	if confidence == language.No || strings.TrimSpace(base.String()) == "" || strings.EqualFold(base.String(), "und") {
		return false
	}
	for idx, part := range parts[1:] {
		if part == "" {
			return false
		}
		switch idx {
		case 0:
			if len(part) == 4 {
				if !isWebQueryAlphaToken(part) {
					return false
				}
				continue
			}
			if len(part) == 2 || len(part) == 3 {
				if !isWebQueryAlphaNumToken(part) {
					return false
				}
				continue
			}
		case 1:
			if len(parts[1]) == 4 && (len(part) == 2 || len(part) == 3) && isWebQueryAlphaNumToken(part) {
				continue
			}
		}
		if _, ok := webQueryTranscriptLocaleCanonicalByAlias[normalized]; ok {
			return true
		}
		return false
	}
	return true
}

func isWebQueryAlphaToken(raw string) bool {
	for _, r := range raw {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return raw != ""
}

func isWebQueryAlphaNumToken(raw string) bool {
	for _, r := range raw {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return raw != ""
}

func appendWebQueryLanguageVariants(dst []string, raws ...string) []string {
	for _, raw := range raws {
		for _, variant := range expandWebQueryLanguageVariants(raw) {
			seen := false
			for _, existing := range dst {
				if existing == variant {
					seen = true
					break
				}
			}
			if !seen {
				dst = append(dst, variant)
			}
		}
	}
	return dst
}

func normalizeWebQueryLanguageLabel(raw string) string {
	replacer := strings.NewReplacer(
		"_", " ",
		"-", " ",
		"/", " ",
		"(", " ",
		")", " ",
		"[", " ",
		"]", " ",
		"{", " ",
		"}", " ",
		":", " ",
		";", " ",
		",", " ",
		".", " ",
		"（", " ",
		"）", " ",
		"【", " ",
		"】", " ",
		"，", " ",
		"：", " ",
	)
	raw = strings.ToLower(strings.TrimSpace(replacer.Replace(raw)))
	return strings.Join(strings.Fields(raw), " ")
}

func inferWebQueryLanguageFromLabel(label string) string {
	normalized := normalizeWebQueryLanguageLabel(label)
	if normalized == "" {
		return ""
	}
	for _, hint := range webQueryLanguageLabelHints {
		for _, pattern := range hint.Patterns {
			if strings.Contains(normalized, normalizeWebQueryLanguageLabel(pattern)) {
				return canonicalizeWebQueryLanguageTag(hint.Language)
			}
		}
	}
	return ""
}

func inferWebQueryLanguageFromURL(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err == nil {
		for _, key := range []string{"lang", "tlang", "language", "locale", "language_code", "languageCode", "lan"} {
			if value := strings.TrimSpace(parsed.Query().Get(key)); isWebQueryUsableLanguageTag(value) {
				return canonicalizeWebQueryLanguageTag(value)
			}
		}
		filename := strings.TrimSuffix(filepath.Base(parsed.Path), filepath.Ext(parsed.Path))
		if isWebQueryUsableLanguageTag(filename) {
			return canonicalizeWebQueryLanguageTag(filename)
		}
	}
	return ""
}

func resolveWebQueryLanguageValue(raw, label, rawURL string, fallbacks ...string) string {
	candidates := []string{raw, inferWebQueryLanguageFromLabel(label), inferWebQueryLanguageFromURL(rawURL)}
	candidates = append(candidates, fallbacks...)
	for _, candidate := range candidates {
		if !isWebQueryUsableLanguageTag(candidate) {
			continue
		}
		return canonicalizeWebQueryLanguageTag(candidate)
	}
	return ""
}

func looksLikeWebQueryVideoURL(raw string) bool {
	_, ok := classifyWebQueryVideoTarget(raw)
	return ok
}

func looksLikeWebQueryTranscriptIntentQuery(query string) bool {
	lower := strings.ToLower(strings.TrimSpace(query))
	if lower == "" {
		return false
	}
	keywords := []string{
		"transcript", "caption", "captions", "subtitle", "subtitles", "cc",
		"字幕", "转录", "听写", "字幕稿", "逐字稿",
	}
	for _, keyword := range keywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

func classifyWebQueryVideoTarget(raw string) (webQueryVideoTarget, bool) {
	normalized, err := normalizeWebFetchURL(raw)
	if err != nil {
		return webQueryVideoTarget{}, false
	}
	parsed, err := url.Parse(normalized)
	if err != nil {
		return webQueryVideoTarget{}, false
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	target := webQueryVideoTarget{URL: normalized}
	switch {
	case host == "youtu.be" || host == "www.youtube.com" || host == "youtube.com" || host == "m.youtube.com":
		target.Platform = webQueryVideoPlatformYouTube
		target.MediaKind = webQueryMediaKindVideo
		target.VideoID = firstNonEmpty(strings.TrimSpace(parsed.Query().Get("v")), strings.Trim(strings.TrimSpace(parsed.Path), "/"))
		return target, true
	case host == "www.bilibili.com" || host == "bilibili.com" || host == "m.bilibili.com" || host == "b23.tv":
		target.Platform = webQueryVideoPlatformBilibili
		target.MediaKind = webQueryMediaKindVideo
		target.VideoID = strings.Trim(strings.TrimSpace(parsed.Path), "/")
		return target, true
	}

	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(parsed.Path), "."))
	switch ext {
	case "mp4", "m4v", "mov", "webm":
		target.Platform = webQueryVideoPlatformDirectMedia
		target.DirectMedia = true
		target.MediaKind = webQueryMediaKindVideo
		target.VideoID = trimWebQueryVideoExt(filepath.Base(parsed.Path))
		return target, true
	case "mp3", "m4a", "aac", "wav", "ogg", "opus", "flac":
		target.Platform = webQueryVideoPlatformDirectMedia
		target.DirectMedia = true
		target.MediaKind = webQueryMediaKindAudio
		target.VideoID = trimWebQueryVideoExt(filepath.Base(parsed.Path))
		return target, true
	default:
		return webQueryVideoTarget{}, false
	}
}

func normalizeWebQueryLanguageTag(raw string) string {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, "_", "-"))
	if raw == "" {
		return ""
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == '-' || r == '_'
	})
	if len(parts) == 0 {
		return ""
	}
	return strings.ToLower(strings.Join(parts, "-"))
}

func canonicalizeWebQueryLanguageTag(raw string) string {
	normalized := normalizeWebQueryLanguageTag(raw)
	if normalized == "" {
		return ""
	}
	if canonical, ok := webQueryTranscriptLocaleCanonicalByAlias[normalized]; ok {
		return canonical
	}
	if tag, err := language.Parse(normalized); err == nil {
		if parsed := strings.ToLower(strings.TrimSpace(tag.String())); parsed != "" && parsed != "und" {
			if canonical, ok := webQueryTranscriptLocaleCanonicalByAlias[parsed]; ok {
				return canonical
			}
			return parsed
		}
	}
	return normalized
}

func webQueryLanguageBase(raw string) string {
	raw = normalizeWebQueryLanguageTag(raw)
	if raw == "" {
		return ""
	}
	if idx := strings.IndexByte(raw, '-'); idx > 0 {
		return raw[:idx]
	}
	return raw
}

func expandWebQueryLanguageVariants(raw string) []string {
	normalized := normalizeWebQueryLanguageTag(raw)
	if normalized == "" {
		return nil
	}
	canonical := canonicalizeWebQueryLanguageTag(normalized)
	variants := make([]string, 0, 8)
	addVariant := func(value string) {
		value = normalizeWebQueryLanguageTag(value)
		if value == "" {
			return
		}
		for _, existing := range variants {
			if existing == value {
				return
			}
		}
		variants = append(variants, value)
	}

	addVariant(normalized)
	addVariant(canonical)
	if profile, ok := webQueryTranscriptLocaleProfileByCanonical[canonical]; ok {
		for _, alias := range profile.Aliases {
			addVariant(alias)
		}
	}

	if tag, err := language.Parse(canonical); err == nil {
		base, _ := tag.Base()
		script, _ := tag.Script()
		region, _ := tag.Region()
		baseValue := strings.ToLower(strings.TrimSpace(base.String()))
		scriptValue := strings.ToLower(strings.TrimSpace(script.String()))
		regionValue := strings.ToLower(strings.TrimSpace(region.String()))
		if baseValue != "" && baseValue != "und" {
			addVariant(baseValue)
			if scriptValue != "" && scriptValue != "zzzz" {
				addVariant(baseValue + "-" + scriptValue)
			}
			if regionValue != "" && regionValue != "zz" {
				addVariant(baseValue + "-" + regionValue)
			}
			if scriptValue != "" && scriptValue != "zzzz" && regionValue != "" && regionValue != "zz" {
				addVariant(baseValue + "-" + scriptValue + "-" + regionValue)
			}
		}
	} else {
		addVariant(webQueryLanguageBase(canonical))
	}
	return variants
}

func resolveWebQueryLanguagePreference(ctx context.Context, args map[string]interface{}) webQueryLanguagePreference {
	explicit := strings.TrimSpace(firstCompatStringDeep(args, "language", "lang"))
	contextLang := strings.TrimSpace(GetLang(ctx))
	raw := resolveWebQueryLanguageValue(explicit, "", "", contextLang, "en-us")
	if raw == "" {
		raw = "en-us"
	}
	fallbacks := make([]string, 0, 8)
	if contextResolved := resolveWebQueryLanguageValue(contextLang, "", ""); contextResolved != "" && contextResolved != raw {
		fallbacks = appendWebQueryLanguageVariants(fallbacks, contextResolved)
	}
	if webQueryLanguageBase(raw) != "en" {
		fallbacks = appendWebQueryLanguageVariants(fallbacks, "en-us")
	}
	return webQueryLanguagePreference{
		Raw:       raw,
		Base:      webQueryLanguageBase(raw),
		Variants:  expandWebQueryLanguageVariants(raw),
		Fallbacks: fallbacks,
	}
}

func resolveWebQuerySTTLanguage(pref webQueryLanguagePreference) string {
	switch pref.Base {
	case "nb":
		return "no"
	}
	if pref.Base != "" {
		return pref.Base
	}
	return pref.Raw
}

func (t *WebTool) tryExecuteVideoSearchQuery(ctx context.Context, args map[string]interface{}, query, format string, maxChars int, candidates []webQueryCandidate) (webQueryEnvelope, bool) {
	videoIndexes := make([]int, 0, webQueryMaxVideoSearchCandidates)
	for idx, candidate := range candidates {
		if looksLikeWebQueryVideoURL(candidate.Search.URL) {
			videoIndexes = append(videoIndexes, idx)
		}
		if len(videoIndexes) >= webQueryMaxVideoSearchCandidates {
			break
		}
	}
	if len(videoIndexes) == 0 {
		return webQueryEnvelope{}, false
	}

	pref := resolveWebQueryLanguagePreference(ctx, args)
	results := make(chan webQueryVideoCandidateOutcome, len(videoIndexes))
	var wg sync.WaitGroup
	for _, idx := range videoIndexes {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results <- webQueryVideoCandidateOutcome{
				Index:  idx,
				Result: t.resolveWebQueryVideoTranscript(ctx, args, candidates[idx].Search.URL, pref),
			}
		}(idx)
	}
	go func() {
		wg.Wait()
		close(results)
	}()

	resolved := make(map[int]webQueryVideoResolveResult, len(videoIndexes))
	bestIndex := -1
	bestScore := math.Inf(-1)
	for outcome := range results {
		resolved[outcome.Index] = outcome.Result
		score := candidates[outcome.Index].Score + outcome.Result.QualityScore
		if outcome.Result.Transcript == nil && outcome.Result.Media == nil {
			score -= 80
		}
		if score > bestScore {
			bestScore = score
			bestIndex = outcome.Index
		}
	}
	if bestIndex < 0 {
		return webQueryEnvelope{}, false
	}

	selected := candidates[bestIndex]
	selectedResult := resolved[bestIndex]
	pageResult := webQueryReadResult{}
	if !selectedResult.Target.DirectMedia {
		pageResult, _ = t.runReadPipeline(ctx, args, selected.Search.URL, format, maxChars, nil)
	}

	envelope := newWebQueryEnvelope(query, format)
	envelope.Query = query
	envelope.Mode = "video_search_read"
	envelope.Diagnostics.Route = "search_video"
	envelope.Diagnostics.SelectedSource = selected.Rank
	envelope.Diagnostics.Attempts = append(envelope.Diagnostics.Attempts, selectedResult.Attempts...)
	envelope.Diagnostics.Attempts = append(envelope.Diagnostics.Attempts, pageResult.Attempts...)
	envelope.Diagnostics.Degraded = len(videoIndexes) > 1 || len(pageResult.Attempts) > 1

	t.applyVideoResultToEnvelope(&envelope, selected.Search.URL, selectedResult, pageResult)
	envelope.Sources = buildWebQueryVideoSources(selected.Search.URL, selectedResult, pageResult, candidates, videoIndexes)
	if envelope.Status == "" {
		envelope.Status = webQueryStatusPartial
	}
	if envelope.Title == "" {
		envelope.Title = strings.TrimSpace(selected.Search.Title)
	}
	if envelope.TargetURL == "" {
		envelope.TargetURL = strings.TrimSpace(selected.Search.URL)
	}
	if envelope.FinalURL == "" {
		envelope.FinalURL = envelope.TargetURL
	}
	return envelope, true
}

func (t *WebTool) executeVideoURLQuery(ctx context.Context, args map[string]interface{}, input, format string, maxChars int) webQueryEnvelope {
	envelope := newWebQueryEnvelope(input, format)
	envelope.Mode = "video_read"
	envelope.TargetURL = input
	envelope.Diagnostics.Route = "video_url"

	pref := resolveWebQueryLanguagePreference(ctx, args)
	target, _ := classifyWebQueryVideoTarget(input)

	var (
		transcriptResult webQueryVideoResolveResult
		pageResult       webQueryReadResult
		wg               sync.WaitGroup
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		transcriptResult = t.resolveWebQueryVideoTranscript(ctx, args, input, pref)
	}()
	if !target.DirectMedia {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pageResult, _ = t.runReadPipeline(ctx, args, input, format, maxChars, nil)
		}()
	}
	wg.Wait()

	envelope.Diagnostics.Attempts = append(envelope.Diagnostics.Attempts, transcriptResult.Attempts...)
	envelope.Diagnostics.Attempts = append(envelope.Diagnostics.Attempts, pageResult.Attempts...)
	envelope.Diagnostics.CandidateCount = 1
	envelope.Diagnostics.SelectedSource = 1
	envelope.Diagnostics.Degraded = len(envelope.Diagnostics.Attempts) > 1

	t.applyVideoResultToEnvelope(&envelope, input, transcriptResult, pageResult)
	envelope.Sources = buildWebQueryVideoSources(input, transcriptResult, pageResult, nil, nil)
	if envelope.Status == "" {
		if transcriptResult.Transcript != nil && transcriptResult.Transcript.Status == webQueryTranscriptStatusOK {
			envelope.Status = webQueryStatusOK
		} else {
			envelope.Status = webQueryStatusPartial
		}
	}
	if envelope.TargetURL == "" {
		envelope.TargetURL = input
	}
	if envelope.FinalURL == "" {
		envelope.FinalURL = envelope.TargetURL
	}
	return envelope
}

func (t *WebTool) applyVideoResultToEnvelope(envelope *webQueryEnvelope, requestedURL string, resolved webQueryVideoResolveResult, pageResult webQueryReadResult) {
	if envelope == nil {
		return
	}
	if resolved.Media != nil {
		clone := *resolved.Media
		if clone.Language == "" && resolved.Transcript != nil {
			clone.Language = resolved.Transcript.Language
		}
		envelope.Media = &clone
	}
	if resolved.Transcript != nil {
		clone := *resolved.Transcript
		envelope.Transcript = &clone
		envelope.Content = strings.TrimSpace(clone.Text)
		envelope.ContentFormat = webReadFormatText
	}
	if page := buildWebQueryPage(pageResult); page != nil {
		envelope.Page = page
	}
	envelope.TargetURL = firstNonEmpty(strings.TrimSpace(requestedURL), envelope.TargetURL)
	if page := envelope.Page; page != nil {
		envelope.TargetURL = firstNonEmpty(strings.TrimSpace(page.TargetURL), envelope.TargetURL)
		envelope.FinalURL = firstNonEmpty(strings.TrimSpace(page.FinalURL), envelope.FinalURL, envelope.TargetURL)
		envelope.Title = firstNonEmpty(strings.TrimSpace(page.Title), envelope.Title)
	}
	if resolved.Media != nil {
		envelope.Title = firstNonEmpty(strings.TrimSpace(envelope.Title), inferWebQueryVideoTitleFromURL(envelope.TargetURL))
	}
	if resolved.Transcript != nil {
		envelope.Title = firstNonEmpty(strings.TrimSpace(envelope.Title), inferWebQueryVideoTitleFromURL(envelope.TargetURL))
		switch resolved.Transcript.Status {
		case webQueryTranscriptStatusOK:
			envelope.Status = webQueryStatusOK
			envelope.NextAction = webQueryNextActionNone
		case webQueryTranscriptStatusPartial:
			envelope.Status = webQueryStatusPartial
			envelope.NextAction = webQueryNextActionNone
		default:
			if pageResult.NeedsBrowser {
				envelope.Status = webQueryStatusNeedsBrowser
				envelope.NextAction = webQueryNextActionRetryBrowser
			} else {
				envelope.Status = webQueryStatusPartial
				envelope.NextAction = webQueryNextActionNone
			}
		}
	}
	if envelope.Transcript == nil && envelope.Page != nil {
		if pageResult.NeedsBrowser {
			envelope.Status = webQueryStatusNeedsBrowser
			envelope.NextAction = webQueryNextActionRetryBrowser
		} else if pageResult.HasSuccess {
			envelope.Status = webQueryStatusPartial
			envelope.NextAction = webQueryNextActionNone
		}
	}
	envelope.Warnings = append(envelope.Warnings, resolved.Warnings...)
	envelope.Warnings = append(envelope.Warnings, pageResult.Warnings...)
	if envelope.Content == "" && envelope.Page != nil {
		envelope.ContentFormat = firstNonEmpty(strings.TrimSpace(envelope.Page.ContentFormat), envelope.ContentFormat)
	}
	if envelope.Title == "" {
		envelope.Title = inferWebQueryVideoTitleFromURL(firstNonEmpty(envelope.TargetURL, requestedURL))
	}
}

func buildWebQueryPage(readResult webQueryReadResult) *webQueryPage {
	if !readResult.HasSuccess && strings.TrimSpace(readResult.Response.Content) == "" && strings.TrimSpace(readResult.Response.Title) == "" {
		return nil
	}
	resp := readResult.Response
	return &webQueryPage{
		Title:         strings.TrimSpace(resp.Title),
		Content:       strings.TrimSpace(resp.Content),
		ContentFormat: firstNonEmpty(strings.TrimSpace(resp.Format), webReadFormatText),
		TargetURL:     strings.TrimSpace(resp.URL),
		FinalURL:      firstNonEmpty(strings.TrimSpace(resp.FinalURL), strings.TrimSpace(resp.URL)),
		Source:        strings.TrimSpace(resp.Source),
	}
}

func buildWebQueryVideoSources(selectedURL string, resolved webQueryVideoResolveResult, pageResult webQueryReadResult, candidates []webQueryCandidate, candidateIndexes []int) []webQuerySource {
	sources := make([]webQuerySource, 0, 2+len(candidateIndexes))
	rank := 1
	transcriptSelected := resolved.Transcript != nil && strings.TrimSpace(resolved.Transcript.Text) != ""
	if resolved.Transcript != nil {
		sources = append(sources, webQuerySource{
			Rank:         rank,
			Kind:         transcriptSourceKind(resolved.Transcript.Source),
			URL:          firstNonEmpty(strings.TrimSpace(resolved.TranscriptSourceURL), strings.TrimSpace(selectedURL)),
			FinalURL:     firstNonEmpty(strings.TrimSpace(resolved.TranscriptSourceURL), strings.TrimSpace(selectedURL)),
			Title:        transcriptSourceTitle(resolved),
			Snippet:      truncateRunes(strings.TrimSpace(resolved.Transcript.Text), 280),
			Source:       strings.TrimSpace(resolved.Transcript.Source),
			ContentChars: len([]rune(strings.TrimSpace(resolved.Transcript.Text))),
			Selected:     transcriptSelected,
		})
		rank++
	}
	if page := buildWebQueryPage(pageResult); page != nil {
		sources = append(sources, webQuerySource{
			Rank:         rank,
			Kind:         "page",
			URL:          strings.TrimSpace(page.TargetURL),
			FinalURL:     firstNonEmpty(strings.TrimSpace(page.FinalURL), strings.TrimSpace(page.TargetURL)),
			Title:        strings.TrimSpace(page.Title),
			Snippet:      truncateRunes(strings.TrimSpace(page.Content), 280),
			Source:       strings.TrimSpace(page.Source),
			ContentChars: len([]rune(strings.TrimSpace(page.Content))),
			Selected:     !transcriptSelected,
		})
		rank++
	}
	for _, idx := range candidateIndexes {
		if idx < 0 || idx >= len(candidates) {
			continue
		}
		candidate := candidates[idx]
		if strings.TrimSpace(candidate.Search.URL) == strings.TrimSpace(selectedURL) {
			continue
		}
		sources = append(sources, webQuerySource{
			Rank:         rank,
			Kind:         "search",
			URL:          strings.TrimSpace(candidate.Search.URL),
			FinalURL:     strings.TrimSpace(candidate.Search.URL),
			Title:        strings.TrimSpace(candidate.Search.Title),
			Snippet:      strings.TrimSpace(candidate.Search.Description),
			Source:       strings.TrimSpace(candidate.Search.Source),
			ContentChars: 0,
			Selected:     false,
		})
		rank++
	}
	return sources
}

func transcriptSourceKind(source string) string {
	switch strings.TrimSpace(source) {
	case "subtitle_manual", "subtitle_auto":
		return "subtitle"
	case "asr_local":
		return "audio"
	case "provider":
		return "provider"
	default:
		return "subtitle"
	}
}

func transcriptSourceTitle(resolved webQueryVideoResolveResult) string {
	if resolved.Media != nil && strings.TrimSpace(resolved.Media.Platform) != "" {
		switch strings.TrimSpace(resolved.Transcript.Source) {
		case "subtitle_manual":
			return strings.Title(resolved.Media.Platform) + " subtitles"
		case "subtitle_auto":
			return strings.Title(resolved.Media.Platform) + " auto subtitles"
		case "asr_local":
			return "Local ASR transcript"
		case "provider":
			return "Provider transcript"
		}
	}
	switch strings.TrimSpace(resolved.Transcript.Source) {
	case "subtitle_manual":
		return "Subtitles"
	case "subtitle_auto":
		return "Auto subtitles"
	case "asr_local":
		return "Local ASR transcript"
	case "provider":
		return "Provider transcript"
	default:
		return "Transcript"
	}
}

func inferWebQueryVideoTitleFromURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "video transcript"
	}
	base := trimWebQueryVideoExt(filepath.Base(parsed.Path))
	base = strings.TrimSpace(strings.ReplaceAll(base, "-", " "))
	if base == "" || base == "." || base == "/" {
		return "video transcript"
	}
	return base
}

func (t *WebTool) resolveWebQueryVideoTranscript(ctx context.Context, args map[string]interface{}, targetURL string, pref webQueryLanguagePreference) webQueryVideoResolveResult {
	target, ok := classifyWebQueryVideoTarget(targetURL)
	if !ok {
		return webQueryVideoResolveResult{
			Warnings:     []webQueryWarning{{Code: "unsupported_video_url", Message: "web_query video mode only supports YouTube, Bilibili, and direct media URLs"}},
			QualityScore: math.Inf(-1),
		}
	}
	if target.DirectMedia {
		return t.resolveWebQueryDirectMediaTranscript(ctx, args, target, pref, targetURL)
	}

	httpResult, err := t.fetchWebQueryVideoPageHTTP(ctx, args, target.URL)
	resolveAttempt := webQueryAttempt{
		Stage:        "video_resolve",
		Mode:         target.Platform,
		URL:          target.URL,
		Status:       webQueryAttemptStatus(err),
		ContentChars: len(httpResult.Body),
		Error:        errorString(err),
	}
	result := webQueryVideoResolveResult{
		Target:   target,
		Media:    &webQueryMedia{Kind: target.MediaKind, Platform: target.Platform, VideoID: target.VideoID},
		Attempts: []webQueryAttempt{resolveAttempt},
	}
	if err != nil {
		result.Warnings = append(result.Warnings, webQueryWarning{Code: "video_resolve_failed", Message: err.Error()})
		result.QualityScore = math.Inf(-1)
		return result
	}
	if finalTarget, ok := classifyWebQueryVideoTarget(firstNonEmpty(httpResult.FinalURL, target.URL)); ok {
		target = finalTarget
		result.Target = finalTarget
		if result.Media != nil {
			result.Media.Platform = finalTarget.Platform
			result.Media.Kind = finalTarget.MediaKind
			result.Media.VideoID = finalTarget.VideoID
		}
		if finalTarget.DirectMedia {
			return t.resolveWebQueryDirectMediaTranscript(ctx, args, finalTarget, pref, firstNonEmpty(httpResult.FinalURL, target.URL))
		}
	}

	var parsed webQueryParsedVideo
	switch target.Platform {
	case webQueryVideoPlatformYouTube:
		parsed, err = parseWebQueryYouTubePage(string(httpResult.Body), firstNonEmpty(httpResult.FinalURL, target.URL), pref)
	case webQueryVideoPlatformBilibili:
		parsed, err = parseWebQueryBilibiliPage(string(httpResult.Body), firstNonEmpty(httpResult.FinalURL, target.URL), pref)
	default:
		err = fmt.Errorf("unsupported video platform: %s", target.Platform)
	}
	parseAttempt := webQueryAttempt{
		Stage:  "video_parse",
		Mode:   target.Platform,
		URL:    firstNonEmpty(httpResult.FinalURL, target.URL),
		Status: webQueryAttemptStatus(err),
		Error:  errorString(err),
	}
	result.Attempts = append(result.Attempts, parseAttempt)
	if err != nil {
		result.Warnings = append(result.Warnings, webQueryWarning{Code: "video_parse_failed", Message: err.Error()})
		result.QualityScore = math.Inf(-1)
		return result
	}
	if parsed.Media != nil {
		result.Media = parsed.Media
		if result.Media.Platform == "" {
			result.Media.Platform = target.Platform
		}
		if result.Media.Kind == "" {
			result.Media.Kind = target.MediaKind
		}
	}

	subtitleTranscript, subtitleURL, subtitleAttempts, subtitleWarnings := t.fetchBestWebQuerySubtitle(ctx, args, firstNonEmpty(httpResult.FinalURL, target.URL), parsed.SubtitleCandidates, pref)
	result.Attempts = append(result.Attempts, subtitleAttempts...)
	result.Warnings = append(result.Warnings, subtitleWarnings...)
	if subtitleTranscript != nil {
		result.Transcript = subtitleTranscript
		result.TranscriptSourceURL = subtitleURL
	}

	if shouldFallbackToWebQueryASR(result.Transcript) {
		if strings.TrimSpace(parsed.AudioURL) == "" {
			result.Warnings = append(result.Warnings, webQueryWarning{Code: "audio_stream_unavailable", Message: "no downloadable audio stream was found for local transcription fallback"})
		} else if t.sttService == nil {
			result.Warnings = append(result.Warnings, webQueryWarning{Code: "stt_unavailable", Message: "local STT service is unavailable for transcript fallback"})
		} else {
			asrTranscript, asrAttempts, asrWarnings, asrSourceURL := t.fetchWebQueryASRTranscript(ctx, args, parsed.AudioURL, pref)
			result.Attempts = append(result.Attempts, asrAttempts...)
			result.Warnings = append(result.Warnings, asrWarnings...)
			if webQueryTranscriptScore(asrTranscript) > webQueryTranscriptScore(result.Transcript) {
				result.Transcript = asrTranscript
				result.TranscriptSourceURL = asrSourceURL
			}
		}
	}

	result.QualityScore = webQueryTranscriptScore(result.Transcript)
	effectiveLanguage := resolveWebQueryLanguageValue(
		firstNonEmpty(
			func() string {
				if result.Transcript != nil {
					return result.Transcript.Language
				}
				return ""
			}(),
			func() string {
				if result.Media != nil {
					return result.Media.Language
				}
				return ""
			}(),
		),
		"",
		"",
		parsed.TranscriptLanguage,
		pref.Raw,
		pref.Base,
	)
	if result.Transcript != nil {
		result.Transcript.Language = resolveWebQueryLanguageValue(result.Transcript.Language, "", "", effectiveLanguage)
	}
	if result.Media != nil {
		if result.Transcript != nil && result.Media.DurationMS == 0 {
			result.Media.DurationMS = result.Transcript.DurationMS
		}
		result.Media.Language = resolveWebQueryLanguageValue(result.Media.Language, "", "", effectiveLanguage)
	}
	return result
}

func shouldFallbackToWebQueryASR(transcript *webQueryTranscript) bool {
	if transcript == nil {
		return true
	}
	if transcript.Status == webQueryTranscriptStatusOK {
		return false
	}
	if transcript.CoverageRatio >= 0.75 && len([]rune(strings.TrimSpace(transcript.Text))) >= webFetchMinReadableChars {
		return false
	}
	return true
}

func (t *WebTool) fetchBestWebQuerySubtitle(ctx context.Context, args map[string]interface{}, pageURL string, candidates []webQuerySubtitleCandidate, pref webQueryLanguagePreference) (*webQueryTranscript, string, []webQueryAttempt, []webQueryWarning) {
	if len(candidates) == 0 {
		return nil, "", nil, nil
	}
	sorted := append([]webQuerySubtitleCandidate(nil), candidates...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return webQuerySubtitleCandidateScore(sorted[i], pref) > webQuerySubtitleCandidateScore(sorted[j], pref)
	})

	attempts := make([]webQueryAttempt, 0, len(sorted))
	warnings := make([]webQueryWarning, 0)
	for _, candidate := range sorted {
		transcript, err := t.fetchWebQuerySubtitleCandidate(ctx, args, pageURL, candidate)
		attempts = append(attempts, webQueryAttempt{
			Stage:        "subtitle",
			Mode:         candidate.Source,
			URL:          candidate.URL,
			Status:       webQueryAttemptStatus(err),
			ContentChars: len([]rune(textFromWebQueryTranscript(transcript))),
			Error:        errorString(err),
		})
		if err != nil {
			warnings = append(warnings, webQueryWarning{Code: "subtitle_fetch_failed", Message: err.Error()})
			continue
		}
		return transcript, candidate.URL, attempts, warnings
	}
	return nil, "", attempts, warnings
}

func webQuerySubtitleCandidateScore(candidate webQuerySubtitleCandidate, pref webQueryLanguagePreference) float64 {
	score := 0.0
	candidateVariants := expandWebQueryLanguageVariants(candidate.Language)
	bestLangScore := 0.0
	for idx, variant := range pref.Variants {
		for _, candidateVariant := range candidateVariants {
			if candidateVariant == variant {
				matchScore := 100 - float64(idx*8)
				if matchScore > bestLangScore {
					bestLangScore = matchScore
				}
				continue
			}
			if pref.Base != "" && (candidateVariant == pref.Base || strings.HasPrefix(candidateVariant, pref.Base+"-")) {
				if 75 > bestLangScore {
					bestLangScore = 75
				}
			}
		}
	}
	if bestLangScore == 0 {
		for idx, variant := range pref.Fallbacks {
			for _, candidateVariant := range candidateVariants {
				if candidateVariant == variant {
					matchScore := 48 - float64(idx*3)
					if matchScore > bestLangScore {
						bestLangScore = matchScore
					}
					continue
				}
				if candidateVariant == "en" || strings.HasPrefix(candidateVariant, "en-") {
					if 34 > bestLangScore {
						bestLangScore = 34
					}
				}
			}
		}
	}
	score += bestLangScore
	if pref.Base == "" && !candidate.Generated {
		score += 30
	}
	if !candidate.Generated {
		score += 20
	}
	if candidate.Source == "subtitle_manual" {
		score += 15
	}
	return score
}

func (t *WebTool) fetchWebQuerySubtitleCandidate(ctx context.Context, args map[string]interface{}, pageURL string, candidate webQuerySubtitleCandidate) (*webQueryTranscript, error) {
	targetURL := resolveWebQueryAssetURL(pageURL, candidate.URL)
	result, err := t.fetchWebQueryVideoHTTPResult(ctx, args, targetURL, "text/plain, application/json, text/xml, application/xml;q=0.9", false)
	if err != nil {
		return nil, err
	}
	if result.StatusCode < http.StatusOK || result.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("subtitle fetch failed: HTTP %d", result.StatusCode)
	}
	trimmed := strings.TrimSpace(string(result.Body))
	if trimmed == "" {
		return nil, errors.New("subtitle track returned empty content")
	}

	if transcript, err := parseWebQueryJSON3Transcript(trimmed, candidate); err == nil && transcript != nil {
		return transcript, nil
	}
	if transcript, err := parseWebQueryBilibiliSubtitleJSON(trimmed, candidate); err == nil && transcript != nil {
		return transcript, nil
	}
	if transcript, err := parseWebQueryXMLTranscript(trimmed, candidate); err == nil && transcript != nil {
		return transcript, nil
	}
	if transcript, err := parseWebQueryVTTTranscript(trimmed, candidate); err == nil && transcript != nil {
		return transcript, nil
	}
	return nil, errors.New("unsupported subtitle payload")
}

func (t *WebTool) fetchWebQueryASRTranscript(ctx context.Context, args map[string]interface{}, mediaURL string, pref webQueryLanguagePreference) (*webQueryTranscript, []webQueryAttempt, []webQueryWarning, string) {
	path, contentType, finalURL, cleanup, err := t.downloadWebQueryMediaToTempFile(ctx, args, mediaURL)
	attempts := []webQueryAttempt{{
		Stage:  "audio_download",
		Mode:   "asr_local",
		URL:    mediaURL,
		Status: webQueryAttemptStatus(err),
		Error:  errorString(err),
	}}
	if err != nil {
		return nil, attempts, []webQueryWarning{{Code: "audio_download_failed", Message: err.Error()}}, mediaURL
	}
	defer cleanup()

	file, err := os.Open(path)
	attempt := webQueryAttempt{Stage: "transcript", Mode: "asr_local", URL: firstNonEmpty(finalURL, mediaURL)}
	if err != nil {
		attempt.Status = webQueryAttemptStatus(err)
		attempt.Error = err.Error()
		attempts = append(attempts, attempt)
		return nil, attempts, []webQueryWarning{{Code: "audio_open_failed", Message: err.Error()}}, firstNonEmpty(finalURL, mediaURL)
	}
	defer file.Close()

	resp, err := t.sttService.Transcribe(ctx, &stt.TranscribeRequest{
		Audio:    file,
		Format:   inferWebQueryMediaAudioFormat(contentType, firstNonEmpty(finalURL, mediaURL)),
		Language: resolveWebQuerySTTLanguage(pref),
	})
	attempt.Status = webQueryAttemptStatus(err)
	attempt.Error = errorString(err)
	if err != nil {
		attempts = append(attempts, attempt)
		return nil, attempts, []webQueryWarning{{Code: "transcription_failed", Message: err.Error()}}, firstNonEmpty(finalURL, mediaURL)
	}
	transcript := transcriptFromSTTResponse(resp, "asr_local", false, pref.Raw)
	attempt.ContentChars = len([]rune(strings.TrimSpace(transcript.Text)))
	attempts = append(attempts, attempt)
	return transcript, attempts, nil, firstNonEmpty(finalURL, mediaURL)
}

func (t *WebTool) resolveWebQueryDirectMediaTranscript(ctx context.Context, args map[string]interface{}, target webQueryVideoTarget, pref webQueryLanguagePreference, mediaURL string) webQueryVideoResolveResult {
	result := webQueryVideoResolveResult{
		Target: target,
		Media: &webQueryMedia{
			Kind:     target.MediaKind,
			Platform: target.Platform,
			VideoID:  target.VideoID,
		},
	}
	if t.sttService == nil {
		result.Warnings = append(result.Warnings, webQueryWarning{Code: "stt_unavailable", Message: "local STT service is unavailable for direct media transcription"})
		result.QualityScore = math.Inf(-1)
		return result
	}
	transcript, attempts, warnings, sourceURL := t.fetchWebQueryASRTranscript(ctx, args, mediaURL, pref)
	result.Attempts = append(result.Attempts, attempts...)
	result.Warnings = append(result.Warnings, warnings...)
	result.Transcript = transcript
	result.TranscriptSourceURL = sourceURL
	result.QualityScore = webQueryTranscriptScore(transcript)
	if transcript != nil {
		result.Media.DurationMS = transcript.DurationMS
		result.Media.Language = resolveWebQueryLanguageValue(transcript.Language, "", "", pref.Raw, pref.Base)
		result.Transcript.Language = resolveWebQueryLanguageValue(transcript.Language, "", "", pref.Raw, pref.Base)
	}
	return result
}

func transcriptFromSTTResponse(resp *stt.TranscribeResponse, source string, generated bool, fallbackLanguage string) *webQueryTranscript {
	if resp == nil {
		return nil
	}
	segments := make([]webQueryTranscriptSegment, 0, len(resp.Segments))
	for _, segment := range resp.Segments {
		text := strings.TrimSpace(segment.Text)
		if text == "" {
			continue
		}
		segments = append(segments, webQueryTranscriptSegment{
			StartMS: int64(segment.Start * 1000),
			EndMS:   int64(segment.End * 1000),
			Text:    text,
		})
	}
	text := strings.TrimSpace(resp.Text)
	if text == "" && len(segments) > 0 {
		text = joinWebQueryTranscriptSegments(segments)
	}
	durationMS := int64(resp.Duration * 1000)
	transcript := &webQueryTranscript{
		Status:        classifyWebQueryTranscriptStatus(text, segments, durationMS),
		Source:        source,
		Language:      resolveWebQueryLanguageValue(strings.TrimSpace(resp.Language), "", "", fallbackLanguage),
		Text:          text,
		Segments:      segments,
		DurationMS:    durationMS,
		CoverageRatio: webQueryTranscriptCoverage(segments, durationMS),
		Confidence:    resp.Confidence,
		Generated:     generated,
	}
	return transcript
}

func inferWebQueryMediaAudioFormat(contentType, rawURL string) stt.AudioFormat {
	lowerContentType := strings.ToLower(strings.TrimSpace(contentType))
	lowerURL := strings.ToLower(strings.TrimSpace(rawURL))
	switch {
	case strings.Contains(lowerContentType, "wav") || strings.HasSuffix(lowerURL, ".wav"):
		return stt.FormatWAV
	case strings.Contains(lowerContentType, "mpeg") || strings.HasSuffix(lowerURL, ".mp3"):
		return stt.FormatMP3
	case strings.Contains(lowerContentType, "webm") || strings.HasSuffix(lowerURL, ".webm"):
		return stt.FormatWebM
	case strings.Contains(lowerContentType, "flac") || strings.HasSuffix(lowerURL, ".flac"):
		return stt.FormatFLAC
	case strings.Contains(lowerContentType, "ogg") || strings.Contains(lowerContentType, "opus") || strings.HasSuffix(lowerURL, ".ogg") || strings.HasSuffix(lowerURL, ".opus"):
		return stt.FormatOGG
	case strings.Contains(lowerContentType, "audio/mp4"), strings.HasSuffix(lowerURL, ".m4a"), strings.HasSuffix(lowerURL, ".aac"):
		return stt.AudioFormat("m4a")
	case strings.Contains(lowerContentType, "video/mp4"), strings.HasSuffix(lowerURL, ".mp4"), strings.HasSuffix(lowerURL, ".m4v"), strings.HasSuffix(lowerURL, ".mov"):
		return stt.AudioFormat("mp4")
	default:
		return stt.AudioFormat("mp4")
	}
}

func (t *WebTool) fetchWebQueryVideoPageHTTP(ctx context.Context, args map[string]interface{}, targetURL string) (webFetchHTTPResult, error) {
	return t.fetchWebQueryVideoHTTPResult(ctx, args, targetURL, "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8", true)
}

func (t *WebTool) fetchWebQueryVideoHTTPResult(ctx context.Context, args map[string]interface{}, targetURL, acceptHeader string, preferConfiguredFetch bool) (webFetchHTTPResult, error) {
	normalized, err := normalizeWebFetchURL(targetURL)
	if err != nil {
		return webFetchHTTPResult{}, err
	}
	opts, err := t.webQueryVideoRequestOptions(ctx, args, normalized)
	if err != nil {
		return webFetchHTTPResult{}, err
	}
	if fetchTool := t.webQueryFetchTool(); preferConfiguredFetch && fetchTool != nil {
		return fetchTool.fetchHTTPResult(ctx, normalized, opts, acceptHeader)
	}

	client := newGuardedMediaHTTPClient(webFetchDefaultTimeout)
	userAgent := webFetchDefaultUserAgent
	if fetchTool := t.webQueryFetchTool(); fetchTool != nil {
		client = fetchTool.httpClient
		userAgent = fetchTool.config.UserAgent
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, normalized, nil)
	if err != nil {
		return webFetchHTTPResult{}, err
	}
	req.Header.Set("Accept", acceptHeader)
	req.Header.Set("User-Agent", userAgent)
	for key, value := range opts.extraHeaders {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return webFetchHTTPResult{}, err
	}
	defer resp.Body.Close()

	bodyLimit := int64(webFetchDefaultMaxResponseBytes)
	body, bodyTruncated, err := readLimitedBody(resp.Body, bodyLimit)
	if err != nil {
		return webFetchHTTPResult{}, err
	}
	finalURL := normalized
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	return webFetchHTTPResult{
		FinalURL:      finalURL,
		StatusCode:    resp.StatusCode,
		ContentType:   strings.TrimSpace(resp.Header.Get("Content-Type")),
		Body:          body,
		BodyTruncated: bodyTruncated,
		Headers:       cloneWebFetchHTTPHeader(resp.Header),
		Source:        webAccessSourceHTTP,
	}, nil
}

func (t *WebTool) webQueryVideoRequestOptions(ctx context.Context, args map[string]interface{}, normalizedURL string) (webFetchRequestOptions, error) {
	opts, err := parseWebFetchRequestOptions(args)
	if err != nil {
		return webFetchRequestOptions{}, err
	}
	if fetchTool := t.webQueryFetchTool(); fetchTool != nil {
		if err := fetchTool.applyBrowserSessionCookies(ctx, normalizedURL, &opts); err != nil {
			return webFetchRequestOptions{}, err
		}
		return opts, nil
	}
	if strings.TrimSpace(opts.browserTargetID) == "" {
		return opts, nil
	}
	if t.browser == nil {
		return webFetchRequestOptions{}, errors.New("browser_target_id requires browser backend support")
	}
	cookieHeader, err := t.browser.CookieHeader(ctx, opts.browserTargetID, normalizedURL)
	if err != nil {
		return webFetchRequestOptions{}, fmt.Errorf("failed to read browser session cookies: %w", err)
	}
	if strings.TrimSpace(cookieHeader) != "" {
		if existing := strings.TrimSpace(opts.extraHeaders[http.CanonicalHeaderKey("Cookie")]); existing != "" {
			cookieHeader = mergeWebFetchCookieHeaders(cookieHeader, existing)
		}
		if err := setWebFetchHeader(opts.extraHeaders, "Cookie", cookieHeader); err != nil {
			return webFetchRequestOptions{}, err
		}
	}
	return opts, nil
}

func (t *WebTool) webQueryFetchTool() *WebFetchTool {
	if fetchTool, ok := t.fetch.(*WebFetchTool); ok && fetchTool != nil {
		return fetchTool
	}
	if readTool, ok := t.read.(*WebReadTool); ok && readTool != nil && readTool.runtime != nil {
		return readTool.runtime.base
	}
	return nil
}

func (t *WebTool) downloadWebQueryMediaToTempFile(ctx context.Context, args map[string]interface{}, mediaURL string) (string, string, string, func(), error) {
	normalized, err := normalizeWebFetchURL(mediaURL)
	if err != nil {
		return "", "", "", func() {}, err
	}
	opts, err := t.webQueryVideoRequestOptions(ctx, args, normalized)
	if err != nil {
		return "", "", "", func() {}, err
	}
	client := newGuardedMediaHTTPClient(webFetchDefaultTimeout)
	userAgent := webFetchDefaultUserAgent
	if fetchTool := t.webQueryFetchTool(); fetchTool != nil {
		client = fetchTool.httpClient
		userAgent = fetchTool.config.UserAgent
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, normalized, nil)
	if err != nil {
		return "", "", "", func() {}, err
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", userAgent)
	for key, value := range opts.extraHeaders {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", "", "", func() {}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", "", "", func() {}, fmt.Errorf("media download failed: HTTP %d", resp.StatusCode)
	}

	finalURL := normalized
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if resp.ContentLength > webQueryMaxMediaDownloadBytes && resp.ContentLength > 0 {
		return "", "", "", func() {}, fmt.Errorf("media is too large for local transcription (%d bytes > %d)", resp.ContentLength, webQueryMaxMediaDownloadBytes)
	}
	requestPath := ""
	if resp.Request != nil && resp.Request.URL != nil {
		requestPath = resp.Request.URL.Path
	}
	ext := filepath.Ext(strings.TrimSpace(requestPath))
	if ext == "" {
		ext = filepath.Ext(strings.TrimSpace(req.URL.Path))
	}
	tmp, err := os.CreateTemp("", "zimaos-blue-web-query-media-*"+ext)
	if err != nil {
		return "", "", "", func() {}, err
	}
	cleanup := func() { _ = os.Remove(tmp.Name()) }
	written, err := io.Copy(tmp, io.LimitReader(resp.Body, webQueryMaxMediaDownloadBytes+1))
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		cleanup()
		return "", "", "", func() {}, err
	}
	if written > webQueryMaxMediaDownloadBytes {
		cleanup()
		return "", "", "", func() {}, fmt.Errorf("media exceeds %d byte transcription limit", webQueryMaxMediaDownloadBytes)
	}
	return tmp.Name(), contentType, finalURL, cleanup, nil
}

func resolveWebQueryAssetURL(pageURL, assetURL string) string {
	assetURL = strings.TrimSpace(assetURL)
	if assetURL == "" {
		return ""
	}
	if strings.HasPrefix(assetURL, "//") {
		parsed, err := url.Parse(strings.TrimSpace(pageURL))
		if err == nil && parsed.Scheme != "" {
			return parsed.Scheme + ":" + assetURL
		}
		return "https:" + assetURL
	}
	if resolved, err := url.Parse(assetURL); err == nil && resolved.IsAbs() {
		return assetURL
	}
	base, err := url.Parse(strings.TrimSpace(pageURL))
	if err != nil {
		return assetURL
	}
	ref, err := url.Parse(assetURL)
	if err != nil {
		return assetURL
	}
	return base.ResolveReference(ref).String()
}

func parseWebQueryYouTubePage(htmlDoc, pageURL string, pref webQueryLanguagePreference) (webQueryParsedVideo, error) {
	playerJSON := firstNonEmpty(
		extractEmbeddedJSONObject(htmlDoc, "ytInitialPlayerResponse ="),
		extractEmbeddedJSONObject(htmlDoc, "var ytInitialPlayerResponse ="),
		extractEmbeddedJSONObject(htmlDoc, `"ytInitialPlayerResponse":`),
	)
	if playerJSON == "" {
		return webQueryParsedVideo{}, errors.New("youtube player response not found in page")
	}
	var player map[string]interface{}
	if err := json.Unmarshal([]byte(playerJSON), &player); err != nil {
		return webQueryParsedVideo{}, fmt.Errorf("decode youtube player response: %w", err)
	}

	out := webQueryParsedVideo{
		Media: &webQueryMedia{
			Kind:         webQueryMediaKindVideo,
			Platform:     webQueryVideoPlatformYouTube,
			VideoID:      firstNonEmpty(getStringAtPath(player, "videoDetails", "videoId"), urlParamValue(pageURL, "v")),
			DurationMS:   int64(getFloatAtPath(player, "videoDetails", "lengthSeconds") * 1000),
			ThumbnailURL: youtubeThumbnailURL(player),
			Author:       firstNonEmpty(getStringAtPath(player, "videoDetails", "author"), getStringAtPath(player, "microformat", "playerMicroformatRenderer", "ownerChannelName")),
			PublishedAt:  getStringAtPath(player, "microformat", "playerMicroformatRenderer", "publishDate"),
		},
	}

	if tracks := getArrayAtPath(player, "captions", "playerCaptionsTracklistRenderer", "captionTracks"); len(tracks) > 0 {
		for _, item := range tracks {
			track, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			baseURL := strings.TrimSpace(getString(track, "baseUrl"))
			if baseURL == "" {
				continue
			}
			if !strings.Contains(baseURL, "fmt=") {
				separator := "&"
				if !strings.Contains(baseURL, "?") {
					separator = "?"
				}
				baseURL += separator + "fmt=json3"
			}
			source := "subtitle_manual"
			generated := false
			if strings.EqualFold(strings.TrimSpace(getString(track, "kind")), "asr") {
				source = "subtitle_auto"
				generated = true
			}
			label := firstNonEmpty(getStringAtPath(track, "name", "simpleText"), getString(track, "vssId"))
			out.SubtitleCandidates = append(out.SubtitleCandidates, webQuerySubtitleCandidate{
				URL:       baseURL,
				Language:  resolveWebQueryLanguageValue(getString(track, "languageCode"), label, baseURL, pref.Raw, pref.Base),
				Label:     label,
				Source:    source,
				Generated: generated,
			})
		}
	}
	if formats := getArrayAtPath(player, "streamingData", "adaptiveFormats"); len(formats) > 0 {
		out.AudioURL = pickWebQueryAudioURL(formats)
	}
	out.TranscriptLanguage = preferredTranscriptLanguage(out.SubtitleCandidates, pref)
	if out.Media != nil {
		out.Media.Language = resolveWebQueryLanguageValue(out.Media.Language, "", "", out.TranscriptLanguage, pref.Raw, pref.Base)
	}
	return out, nil
}

func youtubeThumbnailURL(player map[string]interface{}) string {
	thumbs := getArrayAtPath(player, "videoDetails", "thumbnail", "thumbnails")
	if len(thumbs) == 0 {
		return ""
	}
	last := thumbs[len(thumbs)-1]
	if data, ok := last.(map[string]interface{}); ok {
		return strings.TrimSpace(getString(data, "url"))
	}
	return ""
}

func parseWebQueryBilibiliPage(htmlDoc, pageURL string, pref webQueryLanguagePreference) (webQueryParsedVideo, error) {
	stateJSON := firstNonEmpty(
		extractEmbeddedJSONObject(htmlDoc, "__INITIAL_STATE__="),
		extractEmbeddedJSONObject(htmlDoc, "window.__INITIAL_STATE__="),
	)
	playJSON := firstNonEmpty(
		extractEmbeddedJSONObject(htmlDoc, "__playinfo__="),
		extractEmbeddedJSONObject(htmlDoc, "window.__playinfo__="),
	)
	if stateJSON == "" && playJSON == "" {
		return webQueryParsedVideo{}, errors.New("bilibili player state not found in page")
	}
	var (
		state map[string]interface{}
		play  map[string]interface{}
	)
	if stateJSON != "" {
		if err := json.Unmarshal([]byte(stateJSON), &state); err != nil {
			return webQueryParsedVideo{}, fmt.Errorf("decode bilibili initial state: %w", err)
		}
	}
	if playJSON != "" {
		if err := json.Unmarshal([]byte(playJSON), &play); err != nil {
			return webQueryParsedVideo{}, fmt.Errorf("decode bilibili playinfo: %w", err)
		}
	}

	videoID := firstNonEmpty(getStringAtPath(state, "videoData", "bvid"), strings.Trim(strings.TrimSpace(pathFromURL(pageURL)), "/"))
	out := webQueryParsedVideo{
		Media: &webQueryMedia{
			Kind:         webQueryMediaKindVideo,
			Platform:     webQueryVideoPlatformBilibili,
			VideoID:      videoID,
			DurationMS:   int64(getFloatAtPath(state, "videoData", "duration") * 1000),
			ThumbnailURL: resolveWebQueryAssetURL(pageURL, getStringAtPath(state, "videoData", "pic")),
			Author:       firstNonEmpty(getStringAtPath(state, "videoData", "owner", "name"), getStringAtPath(state, "upData", "name")),
			PublishedAt:  unixStringToRFC3339(getStringAtPath(state, "videoData", "pubdate")),
		},
	}

	if subtitles := getArrayAtPath(play, "data", "subtitle", "subtitles"); len(subtitles) > 0 {
		for _, item := range subtitles {
			data, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			subtitleURL := firstNonEmpty(getString(data, "subtitle_url"), getString(data, "url"))
			if subtitleURL == "" {
				continue
			}
			out.SubtitleCandidates = append(out.SubtitleCandidates, webQuerySubtitleCandidate{
				URL:       resolveWebQueryAssetURL(pageURL, subtitleURL),
				Language:  resolveWebQueryLanguageValue(getString(data, "lan"), firstNonEmpty(getString(data, "lan_doc"), getString(data, "id")), subtitleURL, pref.Raw, pref.Base),
				Label:     firstNonEmpty(getString(data, "lan_doc"), getString(data, "id")),
				Source:    "subtitle_manual",
				Generated: false,
			})
		}
	}
	if audio := getArrayAtPath(play, "data", "dash", "audio"); len(audio) > 0 {
		out.AudioURL = pickWebQueryAudioURL(audio)
	}
	out.TranscriptLanguage = preferredTranscriptLanguage(out.SubtitleCandidates, pref)
	if out.Media != nil {
		out.Media.Language = resolveWebQueryLanguageValue(out.Media.Language, "", "", out.TranscriptLanguage, pref.Raw, pref.Base)
	}
	return out, nil
}

func preferredTranscriptLanguage(candidates []webQuerySubtitleCandidate, pref webQueryLanguagePreference) string {
	if len(candidates) == 0 {
		return firstNonEmpty(pref.Raw, pref.Base)
	}
	best := candidates[0]
	bestScore := webQuerySubtitleCandidateScore(best, pref)
	for _, candidate := range candidates[1:] {
		if score := webQuerySubtitleCandidateScore(candidate, pref); score > bestScore {
			best = candidate
			bestScore = score
		}
	}
	return resolveWebQueryLanguageValue(best.Language, best.Label, best.URL, pref.Raw, pref.Base)
}

func pickWebQueryAudioURL(entries []interface{}) string {
	bestURL := ""
	bestBitrate := float64(-1)
	for _, entry := range entries {
		item, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		mimeType := strings.ToLower(strings.TrimSpace(getString(item, "mimeType")))
		if mimeType != "" && !strings.Contains(mimeType, "audio/") {
			continue
		}
		audioURL := firstNonEmpty(getString(item, "url"), getString(item, "baseUrl"), getString(item, "base_url"))
		if audioURL == "" {
			if backup := getArray(item, "backupUrl"); len(backup) > 0 {
				audioURL = asString(backup[0])
			}
			if audioURL == "" {
				if backup := getArray(item, "backup_url"); len(backup) > 0 {
					audioURL = asString(backup[0])
				}
			}
		}
		if audioURL == "" {
			continue
		}
		bitrate := getFloat(item, "bitrate")
		if bitrate > bestBitrate {
			bestBitrate = bitrate
			bestURL = audioURL
		}
	}
	return strings.TrimSpace(bestURL)
}

func parseWebQueryJSON3Transcript(raw string, candidate webQuerySubtitleCandidate) (*webQueryTranscript, error) {
	var payload struct {
		Events []struct {
			StartMS    int64 `json:"tStartMs"`
			DurationMS int64 `json:"dDurationMs"`
			Segs       []struct {
				Text string `json:"utf8"`
			} `json:"segs"`
		} `json:"events"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, err
	}
	segments := make([]webQueryTranscriptSegment, 0, len(payload.Events))
	for _, event := range payload.Events {
		textParts := make([]string, 0, len(event.Segs))
		for _, seg := range event.Segs {
			clean := sanitizeWebQueryTranscriptText(seg.Text)
			if clean == "" {
				continue
			}
			textParts = append(textParts, clean)
		}
		text := strings.TrimSpace(strings.Join(textParts, " "))
		if text == "" {
			continue
		}
		segments = append(segments, webQueryTranscriptSegment{
			StartMS: event.StartMS,
			EndMS:   event.StartMS + maxWebQueryInt64(event.DurationMS, 0),
			Text:    text,
		})
	}
	if len(segments) == 0 {
		return nil, errors.New("json3 transcript contained no segments")
	}
	return buildWebQuerySubtitleTranscript(candidate, segments), nil
}

func parseWebQueryBilibiliSubtitleJSON(raw string, candidate webQuerySubtitleCandidate) (*webQueryTranscript, error) {
	var payload struct {
		Body []struct {
			From    float64 `json:"from"`
			To      float64 `json:"to"`
			Content string  `json:"content"`
		} `json:"body"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, err
	}
	if len(payload.Body) == 0 {
		return nil, errors.New("subtitle json contained no body")
	}
	segments := make([]webQueryTranscriptSegment, 0, len(payload.Body))
	for _, item := range payload.Body {
		text := sanitizeWebQueryTranscriptText(item.Content)
		if text == "" {
			continue
		}
		segments = append(segments, webQueryTranscriptSegment{
			StartMS: int64(item.From * 1000),
			EndMS:   int64(item.To * 1000),
			Text:    text,
		})
	}
	if len(segments) == 0 {
		return nil, errors.New("subtitle json contained no readable segments")
	}
	return buildWebQuerySubtitleTranscript(candidate, segments), nil
}

type webQueryXMLTranscriptPayload struct {
	Texts []struct {
		Start string `xml:"start,attr"`
		Dur   string `xml:"dur,attr"`
		Body  string `xml:",chardata"`
	} `xml:"text"`
}

func parseWebQueryXMLTranscript(raw string, candidate webQuerySubtitleCandidate) (*webQueryTranscript, error) {
	var payload webQueryXMLTranscriptPayload
	if err := xml.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, err
	}
	if len(payload.Texts) == 0 {
		return nil, errors.New("xml transcript contained no text nodes")
	}
	segments := make([]webQueryTranscriptSegment, 0, len(payload.Texts))
	for _, item := range payload.Texts {
		start, _ := strconv.ParseFloat(strings.TrimSpace(item.Start), 64)
		dur, _ := strconv.ParseFloat(strings.TrimSpace(item.Dur), 64)
		text := sanitizeWebQueryTranscriptText(item.Body)
		if text == "" {
			continue
		}
		segments = append(segments, webQueryTranscriptSegment{
			StartMS: int64(start * 1000),
			EndMS:   int64((start + dur) * 1000),
			Text:    text,
		})
	}
	if len(segments) == 0 {
		return nil, errors.New("xml transcript contained no readable segments")
	}
	return buildWebQuerySubtitleTranscript(candidate, segments), nil
}

func parseWebQueryVTTTranscript(raw string, candidate webQuerySubtitleCandidate) (*webQueryTranscript, error) {
	if !strings.Contains(strings.ToUpper(raw), "WEBVTT") && !strings.Contains(raw, "-->") {
		return nil, errors.New("not a vtt transcript")
	}
	lines := strings.Split(raw, "\n")
	segments := make([]webQueryTranscriptSegment, 0)
	for idx := 0; idx < len(lines); idx++ {
		line := strings.TrimSpace(lines[idx])
		if !strings.Contains(line, "-->") {
			continue
		}
		parts := strings.Split(line, "-->")
		if len(parts) != 2 {
			continue
		}
		startMS, okStart := parseWebQueryVTTTimestamp(parts[0])
		endMS, okEnd := parseWebQueryVTTTimestamp(parts[1])
		if !okStart || !okEnd {
			continue
		}
		textLines := make([]string, 0)
		for idx+1 < len(lines) {
			idx++
			next := strings.TrimSpace(lines[idx])
			if next == "" {
				break
			}
			textLines = append(textLines, sanitizeWebQueryTranscriptText(next))
		}
		text := strings.TrimSpace(strings.Join(textLines, " "))
		if text == "" {
			continue
		}
		segments = append(segments, webQueryTranscriptSegment{StartMS: startMS, EndMS: endMS, Text: text})
	}
	if len(segments) == 0 {
		return nil, errors.New("vtt transcript contained no segments")
	}
	return buildWebQuerySubtitleTranscript(candidate, segments), nil
}

func parseWebQueryVTTTimestamp(raw string) (int64, bool) {
	trimmed := strings.TrimSpace(strings.Split(strings.TrimSpace(raw), " ")[0])
	trimmed = strings.ReplaceAll(trimmed, ",", ".")
	parts := strings.Split(trimmed, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, false
	}
	var hours, minutes int64
	secondsPart := ""
	if len(parts) == 3 {
		hoursParsed, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
		if err != nil {
			return 0, false
		}
		hours = hoursParsed
		parts = parts[1:]
	}
	minutesParsed, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
	if err != nil {
		return 0, false
	}
	minutes = minutesParsed
	secondsPart = strings.TrimSpace(parts[1])
	seconds, err := strconv.ParseFloat(secondsPart, 64)
	if err != nil {
		return 0, false
	}
	return hours*3600*1000 + minutes*60*1000 + int64(seconds*1000), true
}

func buildWebQuerySubtitleTranscript(candidate webQuerySubtitleCandidate, segments []webQueryTranscriptSegment) *webQueryTranscript {
	sort.SliceStable(segments, func(i, j int) bool { return segments[i].StartMS < segments[j].StartMS })
	text := joinWebQueryTranscriptSegments(segments)
	durationMS := int64(0)
	if len(segments) > 0 {
		durationMS = segments[len(segments)-1].EndMS
	}
	transcript := &webQueryTranscript{
		Status:        classifyWebQueryTranscriptStatus(text, segments, durationMS),
		Source:        strings.TrimSpace(candidate.Source),
		Language:      resolveWebQueryLanguageValue(candidate.Language, candidate.Label, candidate.URL),
		Text:          text,
		Segments:      segments,
		DurationMS:    durationMS,
		CoverageRatio: webQueryTranscriptCoverage(segments, durationMS),
		Confidence:    defaultWebQueryTranscriptConfidence(candidate),
		Generated:     candidate.Generated,
	}
	return transcript
}

func defaultWebQueryTranscriptConfidence(candidate webQuerySubtitleCandidate) float64 {
	if candidate.Generated {
		return 0.75
	}
	return 0.98
}

func classifyWebQueryTranscriptStatus(text string, segments []webQueryTranscriptSegment, durationMS int64) string {
	contentChars := len([]rune(strings.TrimSpace(text)))
	if contentChars == 0 {
		return webQueryTranscriptStatusUnavailable
	}
	coverage := webQueryTranscriptCoverage(segments, durationMS)
	if contentChars >= webFetchMinReadableChars && (coverage == 0 || coverage >= 0.65 || len(segments) >= 3) {
		return webQueryTranscriptStatusOK
	}
	return webQueryTranscriptStatusPartial
}

func webQueryTranscriptCoverage(segments []webQueryTranscriptSegment, durationMS int64) float64 {
	if len(segments) == 0 || durationMS <= 0 {
		return 0
	}
	lastEnd := segments[len(segments)-1].EndMS
	if lastEnd <= 0 {
		return 0
	}
	coverage := float64(lastEnd) / float64(durationMS)
	if coverage < 0 {
		return 0
	}
	if coverage > 1 {
		return 1
	}
	return coverage
}

func webQueryTranscriptScore(transcript *webQueryTranscript) float64 {
	if transcript == nil {
		return math.Inf(-1)
	}
	score := float64(len([]rune(strings.TrimSpace(transcript.Text)))) / 8
	switch transcript.Status {
	case webQueryTranscriptStatusOK:
		score += 120
	case webQueryTranscriptStatusPartial:
		score += 45
	default:
		score -= 120
	}
	switch strings.TrimSpace(transcript.Source) {
	case "subtitle_manual":
		score += 30
	case "subtitle_auto":
		score += 20
	case "asr_local":
		score += 16
	case "provider":
		score += 12
	}
	score += transcript.CoverageRatio * 30
	score += transcript.Confidence * 10
	return score
}

func textFromWebQueryTranscript(transcript *webQueryTranscript) string {
	if transcript == nil {
		return ""
	}
	return strings.TrimSpace(transcript.Text)
}

func joinWebQueryTranscriptSegments(segments []webQueryTranscriptSegment) string {
	parts := make([]string, 0, len(segments))
	for _, segment := range segments {
		text := strings.TrimSpace(segment.Text)
		if text == "" {
			continue
		}
		parts = append(parts, text)
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func sanitizeWebQueryTranscriptText(raw string) string {
	raw = strings.ReplaceAll(raw, "\u00a0", " ")
	raw = strings.ReplaceAll(raw, "\n", " ")
	raw = strings.TrimSpace(raw)
	raw = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\t' {
			return -1
		}
		return r
	}, raw)
	return strings.Join(strings.Fields(raw), " ")
}

func extractEmbeddedJSONObject(doc, marker string) string {
	idx := strings.Index(doc, marker)
	if idx < 0 {
		return ""
	}
	start := idx + len(marker)
	for start < len(doc) && unicode.IsSpace(rune(doc[start])) {
		start++
	}
	if start >= len(doc) {
		return ""
	}
	if doc[start] != '{' && doc[start] != '[' {
		if brace := strings.IndexAny(doc[start:], "{["); brace >= 0 {
			start += brace
		} else {
			return ""
		}
	}
	return extractBalancedWebQueryJSON(doc[start:])
}

func extractBalancedWebQueryJSON(raw string) string {
	if raw == "" {
		return ""
	}
	open := raw[0]
	close := byte('}')
	if open == '[' {
		close = ']'
	}
	depth := 0
	inString := false
	escapeNext := false
	for idx := 0; idx < len(raw); idx++ {
		ch := raw[idx]
		if inString {
			if escapeNext {
				escapeNext = false
				continue
			}
			switch ch {
			case '\\':
				escapeNext = true
			case '"':
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return raw[:idx+1]
			}
		}
	}
	return ""
}

func getAtPath(root interface{}, path ...string) interface{} {
	current := root
	for _, segment := range path {
		if current == nil {
			return nil
		}
		if idx, err := strconv.Atoi(segment); err == nil {
			switch typed := current.(type) {
			case []interface{}:
				if idx < 0 || idx >= len(typed) {
					return nil
				}
				current = typed[idx]
				continue
			default:
				return nil
			}
		}
		object, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		current = object[segment]
	}
	return current
}

func getStringAtPath(root interface{}, path ...string) string {
	return strings.TrimSpace(asString(getAtPath(root, path...)))
}

func getFloatAtPath(root interface{}, path ...string) float64 {
	return getFloatValue(getAtPath(root, path...))
}

func getArrayAtPath(root interface{}, path ...string) []interface{} {
	if value, ok := getAtPath(root, path...).([]interface{}); ok {
		return value
	}
	return nil
}

func getString(root map[string]interface{}, key string) string {
	return strings.TrimSpace(asString(root[key]))
}

func getFloat(root map[string]interface{}, key string) float64 {
	return getFloatValue(root[key])
}

func getArray(root map[string]interface{}, key string) []interface{} {
	if value, ok := root[key].([]interface{}); ok {
		return value
	}
	return nil
}

func getFloatValue(value interface{}) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		v, _ := typed.Float64()
		return v
	case string:
		v, _ := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return v
	default:
		return 0
	}
}

func pathFromURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	return parsed.Path
}

func urlParamValue(raw, key string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(parsed.Query().Get(key))
}

func unixStringToRFC3339(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	seconds, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return ""
	}
	return time.Unix(seconds, 0).UTC().Format(time.RFC3339)
}

func trimWebQueryVideoExt(name string) string {
	ext := filepath.Ext(strings.TrimSpace(name))
	if ext == "" {
		return strings.TrimSpace(name)
	}
	return strings.TrimSuffix(strings.TrimSpace(name), ext)
}

func maxWebQueryInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
