package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/stt"
)

type scriptedWebQuerySTTService struct {
	response *stt.TranscribeResponse
	err      error
}

func (s *scriptedWebQuerySTTService) Transcribe(_ context.Context, _ *stt.TranscribeRequest) (*stt.TranscribeResponse, error) {
	return s.response, s.err
}

func (s *scriptedWebQuerySTTService) TranscribeWithProvider(_ context.Context, _ stt.ProviderType, _ *stt.TranscribeRequest) (*stt.TranscribeResponse, error) {
	return s.response, s.err
}

func (s *scriptedWebQuerySTTService) TranscribeStream(ctx context.Context, req *stt.TranscribeRequest, callback stt.StreamCallback) error {
	resp, err := s.Transcribe(ctx, req)
	if err != nil {
		return err
	}
	return callback(resp)
}

func (s *scriptedWebQuerySTTService) ListProviders() []stt.ProviderType {
	return []stt.ProviderType{stt.ProviderWhisper}
}
func (s *scriptedWebQuerySTTService) GetDefaultProvider() stt.ProviderType {
	return stt.ProviderWhisper
}
func (s *scriptedWebQuerySTTService) GetWhisperProvider() *stt.WhisperProvider  { return nil }
func (s *scriptedWebQuerySTTService) PeekWhisperProvider() *stt.WhisperProvider { return nil }
func (s *scriptedWebQuerySTTService) Close() error                              { return nil }

type webVideoRewriteHostTransport struct {
	target *url.URL
	base   http.RoundTripper
}

func (t *webVideoRewriteHostTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	targetURL := *req.URL
	targetURL.Scheme = t.target.Scheme
	targetURL.Host = t.target.Host
	cloned.URL = &targetURL
	return t.base.RoundTrip(cloned)
}

func newRewrittenHTTPClient(t *testing.T, server *httptest.Server) *http.Client {
	t.Helper()
	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}
	return &http.Client{
		Transport: &webVideoRewriteHostTransport{
			target: target,
			base:   server.Client().Transport,
		},
	}
}

func TestWebQueryToolYouTubeVideoURLReturnsTranscriptAndPageSummary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/watch":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<!doctype html><html><body><script>
var ytInitialPlayerResponse = {"videoDetails":{"videoId":"abc123","title":"Demo video","author":"Demo creator","lengthSeconds":"42","thumbnail":{"thumbnails":[{"url":"https://img.example/thumb.jpg"}]}},"microformat":{"playerMicroformatRenderer":{"publishDate":"2026-03-23"}},"captions":{"playerCaptionsTracklistRenderer":{"captionTracks":[{"baseUrl":"https://www.youtube.com/api/timedtext?v=abc123&lang=en","languageCode":"en","name":{"simpleText":"English"}}]}}};
</script></body></html>`))
		case r.URL.Path == "/api/timedtext":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"events":[{"tStartMs":0,"dDurationMs":1800,"segs":[{"utf8":"Hello from the first subtitle line."}]},{"tStartMs":1800,"dDurationMs":1800,"segs":[{"utf8":"This transcript is long enough to be considered readable by the video orchestrator."}]}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	fetchTool := NewWebFetchTool(WebFetchConfig{AllowPrivateHosts: true})
	fetchTool.httpClient = newRewrittenHTTPClient(t, server)
	readTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_read", Description: "read", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			resp := webReadResponse{
				URL:      args["url"].(string),
				FinalURL: args["url"].(string),
				Format:   "text",
				Source:   webAccessSourceHTTP,
				Title:    "Demo video",
				Content:  "This is the page summary for the selected YouTube video. It contains contextual notes and a concise description of the page.",
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}

	tool := NewWebQueryTool(nil, fetchTool, readTool, nil, nil)
	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"input": "https://www.youtube.com/watch?v=abc123",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	var envelope webQueryEnvelope
	if err := json.Unmarshal([]byte(raw.(string)), &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if envelope.Mode != "video_read" {
		t.Fatalf("mode = %q, want video_read", envelope.Mode)
	}
	if envelope.Transcript == nil || envelope.Transcript.Source != "subtitle_manual" {
		t.Fatalf("transcript = %+v, want subtitle_manual", envelope.Transcript)
	}
	if envelope.Page == nil || !strings.Contains(envelope.Page.Content, "page summary") {
		t.Fatalf("page = %+v, want page summary", envelope.Page)
	}
	if envelope.Media == nil || envelope.Media.Platform != webQueryVideoPlatformYouTube {
		t.Fatalf("media = %+v, want youtube", envelope.Media)
	}
	if !strings.Contains(envelope.Content, "Hello from the first subtitle line.") {
		t.Fatalf("content = %q, want transcript text", envelope.Content)
	}
	if len(envelope.Sources) < 2 || envelope.Sources[0].Kind != "subtitle" {
		t.Fatalf("sources = %+v, want subtitle + page sources", envelope.Sources)
	}
}

func TestWebQueryToolTranscriptSearchPrefersBilibiliCandidate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/video/BV1demo":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<!doctype html><html><body><script>
window.__INITIAL_STATE__={"videoData":{"bvid":"BV1demo","title":"Bili demo","pic":"//img.example/bili.jpg","owner":{"name":"Uploader"},"duration":65,"pubdate":1711142400}};
window.__playinfo__={"data":{"subtitle":{"subtitles":[{"lan":"zh-CN","lan_doc":"中文","subtitle_url":"//www.bilibili.com/subtitle.json"}]}}};
</script></body></html>`))
		case "/subtitle.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"body":[{"from":0.0,"to":1.2,"content":"第一句字幕。"},{"from":1.2,"to":3.8,"content":"这是一个足够长的字幕结果，用来验证搜索意图会优先返回视频转录。"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	fetchTool := NewWebFetchTool(WebFetchConfig{AllowPrivateHosts: true})
	fetchTool.httpClient = newRewrittenHTTPClient(t, server)
	searchTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_search", Description: "search", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			resp := WebSearchResponse{
				Query: "BV1demo 字幕",
				Results: []WebSearchResult{
					{Title: "Bili demo", URL: "https://www.bilibili.com/video/BV1demo", Description: "官方视频页面"},
					{Title: "普通网页", URL: "https://example.com/article", Description: "普通文章结果"},
				},
				TotalCount: 2,
				Provider:   "bing",
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	readTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_read", Description: "read", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			resp := webReadResponse{
				URL:      args["url"].(string),
				FinalURL: args["url"].(string),
				Format:   "text",
				Source:   webAccessSourceHTTP,
				Title:    "Bili demo",
				Content:  "这是视频页面的摘要内容。",
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}

	tool := NewWebQueryTool(searchTool, fetchTool, readTool, nil, nil)
	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"input": "BV1demo 字幕",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	var envelope webQueryEnvelope
	if err := json.Unmarshal([]byte(raw.(string)), &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if envelope.Mode != "video_search_read" {
		t.Fatalf("mode = %q, want video_search_read", envelope.Mode)
	}
	if envelope.Media == nil || envelope.Media.Platform != webQueryVideoPlatformBilibili {
		t.Fatalf("media = %+v, want bilibili", envelope.Media)
	}
	if envelope.Transcript == nil || !strings.Contains(envelope.Transcript.Text, "第一句字幕") {
		t.Fatalf("transcript = %+v, want bili subtitle text", envelope.Transcript)
	}
	if envelope.TargetURL != "https://www.bilibili.com/video/BV1demo" {
		t.Fatalf("target_url = %q, want bilibili candidate", envelope.TargetURL)
	}
}

func TestWebQueryToolDirectMediaFallsBackToLocalASR(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/clip.mp3" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = w.Write([]byte("fake mp3 bytes"))
	}))
	defer server.Close()

	searchTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_search", Description: "search", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			resp := WebSearchResponse{
				Query: "clip transcript",
				Results: []WebSearchResult{
					{Title: "Audio clip", URL: server.URL + "/clip.mp3", Description: "downloadable audio clip"},
				},
				TotalCount: 1,
				Provider:   "bing",
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	fetchTool := NewWebFetchTool(WebFetchConfig{AllowPrivateHosts: true})
	sttService := &scriptedWebQuerySTTService{
		response: &stt.TranscribeResponse{
			Text:     "Local ASR recovered the transcript from the downloadable audio clip with enough detail to count as a strong result. It includes multiple complete sentences about the recording, mentions the speaker context, preserves the most important details, and is intentionally long enough to cross the strong transcript threshold used by the video query orchestrator.",
			Language: "en",
			Duration: 6.2,
			Segments: []stt.Segment{
				{Start: 0, End: 3.1, Text: "Local ASR recovered the transcript from the downloadable audio clip with enough detail to count as a strong result."},
				{Start: 3.1, End: 6.2, Text: "It includes multiple complete sentences about the recording, mentions the speaker context, preserves the most important details, and is intentionally long enough to cross the strong transcript threshold used by the video query orchestrator."},
			},
			Confidence: 0.89,
		},
	}

	tool := NewWebQueryTool(searchTool, fetchTool, nil, nil, nil)
	tool.SetSTTService(sttService)
	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"input": "clip transcript",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	var envelope webQueryEnvelope
	if err := json.Unmarshal([]byte(raw.(string)), &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if envelope.Transcript == nil || envelope.Transcript.Source != "asr_local" {
		t.Fatalf("transcript = %+v, want asr_local", envelope.Transcript)
	}
	if envelope.Media == nil || envelope.Media.Platform != webQueryVideoPlatformDirectMedia {
		t.Fatalf("media = %+v, want direct_media", envelope.Media)
	}
	if envelope.Status != webQueryStatusOK {
		t.Fatalf("status = %q, want %q", envelope.Status, webQueryStatusOK)
	}
	if len(envelope.Transcript.Segments) != 2 {
		t.Fatalf("segments = %+v, want 2 segments", envelope.Transcript.Segments)
	}
}

func TestResolveWebQueryLanguagePreferenceSupportsAllUILocales(t *testing.T) {
	tests := []struct {
		locale       string
		wantRaw      string
		wantBase     string
		wantContains []string
	}{
		{locale: "ca-ES", wantRaw: "ca-es", wantBase: "ca", wantContains: []string{"ca-es", "ca"}},
		{locale: "cs-CZ", wantRaw: "cs-cz", wantBase: "cs", wantContains: []string{"cs-cz", "cs"}},
		{locale: "da-DK", wantRaw: "da-dk", wantBase: "da", wantContains: []string{"da-dk", "da"}},
		{locale: "de-DE", wantRaw: "de-de", wantBase: "de", wantContains: []string{"de-de", "de"}},
		{locale: "el-GR", wantRaw: "el-gr", wantBase: "el", wantContains: []string{"el-gr", "el"}},
		{locale: "en-GB", wantRaw: "en-gb", wantBase: "en", wantContains: []string{"en-gb", "en-uk", "en"}},
		{locale: "en-US", wantRaw: "en-us", wantBase: "en", wantContains: []string{"en-us", "en"}},
		{locale: "es-ES", wantRaw: "es-es", wantBase: "es", wantContains: []string{"es-es", "es"}},
		{locale: "fr-FR", wantRaw: "fr-fr", wantBase: "fr", wantContains: []string{"fr-fr", "fr"}},
		{locale: "ga-IE", wantRaw: "ga-ie", wantBase: "ga", wantContains: []string{"ga-ie", "ga"}},
		{locale: "hr-HR", wantRaw: "hr-hr", wantBase: "hr", wantContains: []string{"hr-hr", "hr"}},
		{locale: "hu-HU", wantRaw: "hu-hu", wantBase: "hu", wantContains: []string{"hu-hu", "hu"}},
		{locale: "it-IT", wantRaw: "it-it", wantBase: "it", wantContains: []string{"it-it", "it"}},
		{locale: "ja-JP", wantRaw: "ja-jp", wantBase: "ja", wantContains: []string{"ja-jp", "ja"}},
		{locale: "ko-KR", wantRaw: "ko-kr", wantBase: "ko", wantContains: []string{"ko-kr", "ko"}},
		{locale: "ml-IN", wantRaw: "ml-in", wantBase: "ml", wantContains: []string{"ml-in", "ml"}},
		{locale: "nb-NO", wantRaw: "nb-no", wantBase: "nb", wantContains: []string{"nb-no", "nb", "no"}},
		{locale: "nl-NL", wantRaw: "nl-nl", wantBase: "nl", wantContains: []string{"nl-nl", "nl"}},
		{locale: "pl-PL", wantRaw: "pl-pl", wantBase: "pl", wantContains: []string{"pl-pl", "pl"}},
		{locale: "pt-BR", wantRaw: "pt-br", wantBase: "pt", wantContains: []string{"pt-br", "pt-latn-br", "pt"}},
		{locale: "pt-PT", wantRaw: "pt-pt", wantBase: "pt", wantContains: []string{"pt-pt", "pt-latn-pt", "pt"}},
		{locale: "ro-RO", wantRaw: "ro-ro", wantBase: "ro", wantContains: []string{"ro-ro", "ro"}},
		{locale: "ru-RU", wantRaw: "ru-ru", wantBase: "ru", wantContains: []string{"ru-ru", "ru"}},
		{locale: "sk-SK", wantRaw: "sk-sk", wantBase: "sk", wantContains: []string{"sk-sk", "sk"}},
		{locale: "sv-SE", wantRaw: "sv-se", wantBase: "sv", wantContains: []string{"sv-se", "sv"}},
		{locale: "zh-CN", wantRaw: "zh-cn", wantBase: "zh", wantContains: []string{"zh-cn", "zh-hans", "zh"}},
		{locale: "zh-TW", wantRaw: "zh-tw", wantBase: "zh", wantContains: []string{"zh-tw", "zh-hant", "zh-hk", "zh"}},
	}

	for _, tt := range tests {
		t.Run(tt.locale, func(t *testing.T) {
			pref := resolveWebQueryLanguagePreference(WithLang(context.Background(), tt.locale), nil)
			if pref.Raw != tt.wantRaw {
				t.Fatalf("raw = %q, want %q", pref.Raw, tt.wantRaw)
			}
			if pref.Base != tt.wantBase {
				t.Fatalf("base = %q, want %q", pref.Base, tt.wantBase)
			}
			for _, want := range tt.wantContains {
				if !containsWebQueryLanguageVariant(pref.Variants, want) {
					t.Fatalf("variants = %v, want to contain %q", pref.Variants, want)
				}
			}
		})
	}
}

func TestResolveWebQueryLanguagePreferenceNormalizesCommonAliases(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantRaw      string
		wantContains []string
	}{
		{name: "english_uk_alias", input: "en-UK", wantRaw: "en-gb", wantContains: []string{"en-gb", "en-uk", "en"}},
		{name: "norwegian_alias", input: "no", wantRaw: "nb-no", wantContains: []string{"nb-no", "nb", "no"}},
		{name: "traditional_chinese_hk", input: "zh-HK", wantRaw: "zh-tw", wantContains: []string{"zh-tw", "zh-hk", "zh-hant"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pref := resolveWebQueryLanguagePreference(context.Background(), map[string]interface{}{"language": tt.input})
			if pref.Raw != tt.wantRaw {
				t.Fatalf("raw = %q, want %q", pref.Raw, tt.wantRaw)
			}
			for _, want := range tt.wantContains {
				if !containsWebQueryLanguageVariant(pref.Variants, want) {
					t.Fatalf("variants = %v, want to contain %q", pref.Variants, want)
				}
			}
		})
	}
}

func TestWebQuerySubtitleCandidateScorePrefersExactLocaleMatches(t *testing.T) {
	tests := []struct {
		name        string
		language    string
		better      webQuerySubtitleCandidate
		worse       webQuerySubtitleCandidate
		wantSTTLang string
	}{
		{
			name:        "prefers_pt_pt_over_pt_br",
			language:    "pt-PT",
			better:      webQuerySubtitleCandidate{Language: "pt-PT", Source: "subtitle_manual"},
			worse:       webQuerySubtitleCandidate{Language: "pt-BR", Source: "subtitle_manual"},
			wantSTTLang: "pt",
		},
		{
			name:        "prefers_traditional_chinese_over_simplified",
			language:    "zh-TW",
			better:      webQuerySubtitleCandidate{Language: "zh-Hant", Source: "subtitle_manual"},
			worse:       webQuerySubtitleCandidate{Language: "zh-CN", Source: "subtitle_manual"},
			wantSTTLang: "zh",
		},
		{
			name:        "prefers_en_gb_alias_over_en_us",
			language:    "en-GB",
			better:      webQuerySubtitleCandidate{Language: "en-UK", Source: "subtitle_manual"},
			worse:       webQuerySubtitleCandidate{Language: "en-US", Source: "subtitle_manual"},
			wantSTTLang: "en",
		},
		{
			name:        "maps_nb_no_to_no_for_stt",
			language:    "nb-NO",
			better:      webQuerySubtitleCandidate{Language: "no", Source: "subtitle_manual"},
			worse:       webQuerySubtitleCandidate{Language: "sv-SE", Source: "subtitle_manual"},
			wantSTTLang: "no",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pref := resolveWebQueryLanguagePreference(context.Background(), map[string]interface{}{"language": tt.language})
			betterScore := webQuerySubtitleCandidateScore(tt.better, pref)
			worseScore := webQuerySubtitleCandidateScore(tt.worse, pref)
			if betterScore <= worseScore {
				t.Fatalf("better score = %v, worse score = %v", betterScore, worseScore)
			}
			if got := resolveWebQuerySTTLanguage(pref); got != tt.wantSTTLang {
				t.Fatalf("stt language = %q, want %q", got, tt.wantSTTLang)
			}
		})
	}
}

func TestResolveWebQueryLanguagePreferenceFallsBackToContextAndEnglish(t *testing.T) {
	pref := resolveWebQueryLanguagePreference(
		WithLang(context.Background(), "ja-JP"),
		map[string]interface{}{"language": "not_a_locale"},
	)
	if pref.Raw != "ja-jp" {
		t.Fatalf("raw = %q, want ja-jp", pref.Raw)
	}
	if !containsWebQueryLanguageVariant(pref.Fallbacks, "en-us") {
		t.Fatalf("fallbacks = %v, want en-us", pref.Fallbacks)
	}
}

func TestResolveWebQueryLanguageValueUsesURLAndLabelFallbacks(t *testing.T) {
	if got := resolveWebQueryLanguageValue("", "", "https://www.youtube.com/api/timedtext?v=abc123&lang=pt-BR"); got != "pt-br" {
		t.Fatalf("url fallback = %q, want pt-br", got)
	}
	if got := resolveWebQueryLanguageValue("", "English (UK) auto-generated", ""); got != "en-gb" {
		t.Fatalf("label fallback = %q, want en-gb", got)
	}
	if got := resolveWebQueryLanguageValue("", "", "", "zh-HK"); got != "zh-tw" {
		t.Fatalf("fallback chain = %q, want zh-tw", got)
	}
}

func TestParseWebQueryYouTubePageFallsBackLanguageWithoutCountryLeak(t *testing.T) {
	html := `<!doctype html><html><body><script>
var ytInitialPlayerResponse = {"videoDetails":{"videoId":"abc123","title":"Demo video","author":"Demo creator","lengthSeconds":"42","thumbnail":{"thumbnails":[{"url":"https://img.example/thumb.jpg"}]}},"microformat":{"playerMicroformatRenderer":{"publishDate":"2026-03-23","availableCountries":["US"]}},"captions":{"playerCaptionsTracklistRenderer":{"captionTracks":[{"baseUrl":"https://www.youtube.com/api/timedtext?v=abc123&lang=en","name":{"simpleText":"English (UK) auto-generated"},"kind":"asr"}]}}};
</script></body></html>`

	pref := resolveWebQueryLanguagePreference(context.Background(), map[string]interface{}{"language": "fr-FR"})
	parsed, err := parseWebQueryYouTubePage(html, "https://www.youtube.com/watch?v=abc123", pref)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if parsed.TranscriptLanguage != "en-gb" {
		t.Fatalf("transcript language = %q, want en-gb", parsed.TranscriptLanguage)
	}
	if parsed.Media == nil || parsed.Media.Language != "en-gb" {
		t.Fatalf("media language = %+v, want en-gb", parsed.Media)
	}
	if len(parsed.SubtitleCandidates) != 1 || parsed.SubtitleCandidates[0].Language != "en-gb" {
		t.Fatalf("subtitle candidates = %+v, want en-gb fallback", parsed.SubtitleCandidates)
	}
}

func TestWebQuerySubtitleCandidateScorePrefersEnglishFallbackWhenPrimaryLocaleMissing(t *testing.T) {
	pref := resolveWebQueryLanguagePreference(context.Background(), map[string]interface{}{"language": "zh-TW"})
	englishScore := webQuerySubtitleCandidateScore(webQuerySubtitleCandidate{Language: "en-US", Source: "subtitle_manual"}, pref)
	frenchScore := webQuerySubtitleCandidateScore(webQuerySubtitleCandidate{Language: "fr-FR", Source: "subtitle_manual"}, pref)
	if englishScore <= frenchScore {
		t.Fatalf("english score = %v, french score = %v, want english fallback to rank higher", englishScore, frenchScore)
	}
}

func containsWebQueryLanguageVariant(variants []string, want string) bool {
	for _, variant := range variants {
		if variant == want {
			return true
		}
	}
	return false
}
