package i18n

import (
	"strings"
	"testing"
)

const (
	testMsgBrowserScreenshotCaptured                       = "browser.screenshot_captured"
	testMsgBrowserScreenshotCapturedInteractiveUnavailable = "browser.screenshot_captured_interactive_unavailable"
	testMsgBrowserScreenshotCapturedFor                    = "browser.screenshot_captured_for"
	testMsgBrowserScreenshotCapturedForTab                 = "browser.screenshot_captured_for_tab"
	testMsgBrowserScreenshotCapturedForActiveTab           = "browser.screenshot_captured_for_active_tab"
	testMsgBrowserActionPerformedOnRef                     = "browser.action_performed_on_ref"
	testMsgBrowserScrolledPage                             = "browser.scrolled_page"
	testMsgBrowserOpenTabs                                 = "browser.open_tabs"
	testMsgBrowserTabClosed                                = "browser.tab_closed"
	testMsgBrowserRecipesAvailable                         = "browser.recipes_available"
	testMsgBrowserLargeDOMNote                             = "browser.large_dom_note"
	testMsgBrowserPage                                     = "browser.page"
	testMsgBrowserPageWithInteractiveCount                 = "browser.page_with_interactive_count"
	testMsgBrowserPageWithScreenshotInteractiveCount       = "browser.page_with_screenshot_interactive_count"
	testMsgBrowserPageMainContent                          = "browser.page_main_content"
	testMsgBrowserPageMainContentWithSection               = "browser.page_main_content_with_section"
	testMsgBrowserPageStructureSection                     = "browser.page_structure_section"
	testMsgBrowserInteractiveElementsSection               = "browser.interactive_elements_section"
)

func testAllLanguages() []Language {
	return []Language{
		LangEnUS, LangEnGB, LangZhCN, LangZhTW, LangJaJP, LangKoKR,
		LangDeDE, LangFrFR, LangEsES, LangItIT, LangPtBR, LangPtPT,
		LangRuRU, LangPlPL, LangNlNL, LangSvSE, LangDaDK, LangNbNO,
		LangCsCZ, LangSkSK, LangHuHU, LangRoRO, LangHrHR, LangElGR,
		LangCaES, LangGaIE, LangMlIN,
	}
}

func TestParseLanguage(t *testing.T) {
	tests := []struct {
		input string
		want  Language
	}{
		{"", LangEnUS},
		{"en", LangEnUS},
		{"en-US", LangEnUS},
		{"en-GB", LangEnGB},
		{"zh", LangZhCN},
		{"zh-CN", LangZhCN},
		{"zh-Hans", LangZhCN},
		{"zh-TW", LangZhTW},
		{"zh-Hant", LangZhTW},
		{"zh-HK", LangZhTW},
		{"ja", LangJaJP},
		{"ja-JP", LangJaJP},
		{"ko", LangKoKR},
		{"de", LangDeDE},
		{"fr", LangFrFR},
		{"es", LangEsES},
		{"it", LangItIT},
		{"pt-BR", LangPtBR},
		{"pt-PT", LangPtPT},
		{"pt", LangPtPT},
		{"ru", LangRuRU},
		{"pl", LangPlPL},
		{"nl", LangNlNL},
		{"sv", LangSvSE},
		{"da", LangDaDK},
		{"nb", LangNbNO},
		{"no", LangNbNO},
		{"cs", LangCsCZ},
		{"sk", LangSkSK},
		{"hu", LangHuHU},
		{"ro", LangRoRO},
		{"hr", LangHrHR},
		{"el", LangElGR},
		{"ca", LangCaES},
		{"ga", LangGaIE},
		{"ml", LangMlIN},
		// Unknown falls back to en-US
		{"xx", LangEnUS},
	}
	for _, tt := range tests {
		got := ParseLanguage(tt.input)
		if got != tt.want {
			t.Errorf("ParseLanguage(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestT_AllLanguagesHaveMediaKeys(t *testing.T) {
	langs := []Language{
		LangEnUS, LangZhCN, LangZhTW, LangJaJP, LangKoKR,
		LangDeDE, LangFrFR, LangEsES, LangItIT, LangPtBR, LangPtPT,
		LangRuRU, LangPlPL, LangNlNL, LangSvSE, LangDaDK, LangNbNO,
		LangCsCZ, LangSkSK, LangHuHU, LangRoRO, LangHrHR, LangElGR,
		LangCaES, LangGaIE, LangMlIN,
	}
	keys := []string{
		MsgMediaGenerating, MsgMediaGenFailed, MsgMediaGenCancelled,
		MsgMediaGenNoOutput, MsgMediaImageGenerated, MsgMediaVideoGenerated,
		MsgMediaGenerated, MsgMediaGenTimeout,
	}
	for _, lang := range langs {
		for _, key := range keys {
			got := T(lang, key, "test")
			if got == key {
				t.Errorf("T(%q, %q) returned key itself — missing translation", lang, key)
			}
		}
	}
}

func TestT_AllLanguagesHaveWorkspaceRootEscapeError(t *testing.T) {
	for _, lang := range testAllLanguages() {
		got := T(lang, MsgPathEscapesWorkspaceRoot)
		if got == MsgPathEscapesWorkspaceRoot {
			t.Errorf("T(%q, %q) returned key itself — missing translation", lang, MsgPathEscapesWorkspaceRoot)
		}
	}
}

func TestT_AllLanguagesHaveDeclarativeSkillMessage(t *testing.T) {
	for _, lang := range testAllLanguages() {
		got := T(lang, MsgSkillDeclarativeHandledByLLM, "a11y")
		if got == MsgSkillDeclarativeHandledByLLM {
			t.Errorf("T(%q, %q) returned key itself — missing translation", lang, MsgSkillDeclarativeHandledByLLM)
		}
	}
}

func TestT_DeclarativeSkillMessageIsLocalizedOutsideEnglish(t *testing.T) {
	en := T(LangEnUS, MsgSkillDeclarativeHandledByLLM, "a11y")
	for _, lang := range testAllLanguages() {
		if lang == LangEnUS || lang == LangEnGB {
			continue
		}
		got := T(lang, MsgSkillDeclarativeHandledByLLM, "a11y")
		if got == en {
			t.Errorf("T(%q, %q) = %q, expected a non-English translation", lang, MsgSkillDeclarativeHandledByLLM, got)
		}
	}
}

func TestT_AllLanguagesHaveBrowserToolErrorKeys(t *testing.T) {
	tests := []struct {
		key  string
		args []interface{}
	}{
		{key: MsgNativeDocumentOutputVerificationFailed},
		{key: MsgBrowserNotRunning},
		{key: MsgBrowserServiceNotAvailable},
		{key: MsgBrowserServiceNotAvailableReviewURL},
		{key: MsgProxyBridgeNotAvailable},
		{key: MsgProxyBridgeNotAvailableCallVLM},
		{key: MsgNavigationURLNotAllowed},
		{key: MsgBrowserStartFailed, args: []interface{}{"launch timeout"}},
		{key: MsgNavigationFailed, args: []interface{}{"net::ERR_NAME_NOT_RESOLVED"}},
	}

	for _, lang := range testAllLanguages() {
		for _, tt := range tests {
			got := T(lang, tt.key, tt.args...)
			if got == tt.key {
				t.Errorf("T(%q, %q) returned key itself — missing translation", lang, tt.key)
			}
		}
	}
}

func TestT_BrowserToolErrorKeysAreLocalizedOutsideEnglish(t *testing.T) {
	protected := []struct {
		lang Language
		key  string
		args []interface{}
	}{
		{lang: LangZhTW, key: MsgNativeDocumentOutputVerificationFailed},
		{lang: LangZhTW, key: MsgBrowserServiceNotAvailableReviewURL},
		{lang: LangJaJP, key: MsgBrowserStartFailed, args: []interface{}{"launch timeout"}},
		{lang: LangJaJP, key: MsgNativeDocumentOutputVerificationFailed},
		{lang: LangKoKR, key: MsgProxyBridgeNotAvailableCallVLM},
		{lang: LangDeDE, key: MsgNavigationURLNotAllowed},
		{lang: LangFrFR, key: MsgNativeDocumentOutputVerificationFailed},
		{lang: LangFrFR, key: MsgBrowserServiceNotAvailable},
		{lang: LangEsES, key: MsgNavigationFailed, args: []interface{}{"net::ERR_NAME_NOT_RESOLVED"}},
		{lang: LangEsES, key: MsgNativeDocumentOutputVerificationFailed},
		{lang: LangItIT, key: MsgBrowserNotRunning},
		{lang: LangPtBR, key: MsgBrowserStartFailed, args: []interface{}{"launch timeout"}},
		{lang: LangPtBR, key: MsgNativeDocumentOutputVerificationFailed},
		{lang: LangPtPT, key: MsgNavigationFailed, args: []interface{}{"net::ERR_NAME_NOT_RESOLVED"}},
		{lang: LangRuRU, key: MsgProxyBridgeNotAvailable},
		{lang: LangRuRU, key: MsgNativeDocumentOutputVerificationFailed},
		{lang: LangPlPL, key: MsgBrowserServiceNotAvailableReviewURL},
		{lang: LangNlNL, key: MsgBrowserStartFailed, args: []interface{}{"launch timeout"}},
		{lang: LangNlNL, key: MsgNativeDocumentOutputVerificationFailed},
		{lang: LangSvSE, key: MsgNavigationURLNotAllowed},
		{lang: LangDaDK, key: MsgBrowserNotRunning},
		{lang: LangDaDK, key: MsgNativeDocumentOutputVerificationFailed},
		{lang: LangNbNO, key: MsgProxyBridgeNotAvailableCallVLM},
		{lang: LangCsCZ, key: MsgNavigationFailed, args: []interface{}{"net::ERR_NAME_NOT_RESOLVED"}},
		{lang: LangCsCZ, key: MsgNativeDocumentOutputVerificationFailed},
		{lang: LangSkSK, key: MsgBrowserServiceNotAvailable},
		{lang: LangHuHU, key: MsgBrowserStartFailed, args: []interface{}{"launch timeout"}},
		{lang: LangHuHU, key: MsgNativeDocumentOutputVerificationFailed},
		{lang: LangRoRO, key: MsgNavigationURLNotAllowed},
		{lang: LangHrHR, key: MsgProxyBridgeNotAvailable},
		{lang: LangHrHR, key: MsgNativeDocumentOutputVerificationFailed},
		{lang: LangElGR, key: MsgBrowserServiceNotAvailableReviewURL},
		{lang: LangCaES, key: MsgBrowserStartFailed, args: []interface{}{"launch timeout"}},
		{lang: LangCaES, key: MsgNativeDocumentOutputVerificationFailed},
		{lang: LangGaIE, key: MsgNavigationFailed, args: []interface{}{"net::ERR_NAME_NOT_RESOLVED"}},
		{lang: LangMlIN, key: MsgBrowserServiceNotAvailable},
		{lang: LangMlIN, key: MsgNativeDocumentOutputVerificationFailed},
	}

	for _, tt := range protected {
		got := T(tt.lang, tt.key, tt.args...)
		en := T(LangEnUS, tt.key, tt.args...)
		if got == en {
			t.Errorf("T(%q, %q) = %q, expected a non-English translation", tt.lang, tt.key, got)
		}
	}
}

func TestT_AllLanguagesHaveBrowserScreenshotStatusKeys(t *testing.T) {
	tests := []struct {
		key  string
		args []interface{}
	}{
		{key: testMsgBrowserScreenshotCaptured},
		{key: testMsgBrowserScreenshotCapturedInteractiveUnavailable},
		{key: testMsgBrowserScreenshotCapturedFor, args: []interface{}{"https://example.com"}},
		{key: testMsgBrowserScreenshotCapturedForTab, args: []interface{}{"tab-42"}},
		{key: testMsgBrowserScreenshotCapturedForActiveTab},
	}

	for _, lang := range testAllLanguages() {
		for _, tt := range tests {
			got := T(lang, tt.key, tt.args...)
			if got == tt.key {
				t.Errorf("T(%q, %q) returned key itself — missing translation", lang, tt.key)
			}
		}
	}
}

func TestT_BrowserScreenshotStatusKeysAreLocalizedOutsideEnglish(t *testing.T) {
	protected := []struct {
		lang Language
		key  string
		args []interface{}
	}{
		{lang: LangZhCN, key: testMsgBrowserScreenshotCapturedForTab, args: []interface{}{"tab-42"}},
		{lang: LangZhTW, key: testMsgBrowserScreenshotCapturedForActiveTab},
		{lang: LangJaJP, key: testMsgBrowserScreenshotCapturedFor, args: []interface{}{"https://example.com"}},
		{lang: LangKoKR, key: testMsgBrowserScreenshotCapturedInteractiveUnavailable},
		{lang: LangDeDE, key: testMsgBrowserScreenshotCapturedForTab, args: []interface{}{"tab-42"}},
		{lang: LangFrFR, key: testMsgBrowserScreenshotCapturedForActiveTab},
		{lang: LangEsES, key: testMsgBrowserScreenshotCapturedFor, args: []interface{}{"https://example.com"}},
		{lang: LangItIT, key: testMsgBrowserScreenshotCapturedInteractiveUnavailable},
		{lang: LangPtBR, key: testMsgBrowserScreenshotCapturedForTab, args: []interface{}{"tab-42"}},
		{lang: LangPtPT, key: testMsgBrowserScreenshotCapturedForActiveTab},
		{lang: LangRuRU, key: testMsgBrowserScreenshotCapturedFor, args: []interface{}{"https://example.com"}},
		{lang: LangPlPL, key: testMsgBrowserScreenshotCapturedInteractiveUnavailable},
		{lang: LangNlNL, key: testMsgBrowserScreenshotCapturedForTab, args: []interface{}{"tab-42"}},
		{lang: LangSvSE, key: testMsgBrowserScreenshotCapturedForActiveTab},
		{lang: LangDaDK, key: testMsgBrowserScreenshotCapturedFor, args: []interface{}{"https://example.com"}},
		{lang: LangNbNO, key: testMsgBrowserScreenshotCapturedInteractiveUnavailable},
		{lang: LangCsCZ, key: testMsgBrowserScreenshotCapturedForTab, args: []interface{}{"tab-42"}},
		{lang: LangSkSK, key: testMsgBrowserScreenshotCapturedForActiveTab},
		{lang: LangHuHU, key: testMsgBrowserScreenshotCapturedFor, args: []interface{}{"https://example.com"}},
		{lang: LangRoRO, key: testMsgBrowserScreenshotCapturedInteractiveUnavailable},
		{lang: LangHrHR, key: testMsgBrowserScreenshotCapturedForTab, args: []interface{}{"tab-42"}},
		{lang: LangElGR, key: testMsgBrowserScreenshotCapturedForActiveTab},
		{lang: LangCaES, key: testMsgBrowserScreenshotCapturedFor, args: []interface{}{"https://example.com"}},
		{lang: LangGaIE, key: testMsgBrowserScreenshotCapturedInteractiveUnavailable},
		{lang: LangMlIN, key: testMsgBrowserScreenshotCapturedForActiveTab},
	}

	for _, tt := range protected {
		got := T(tt.lang, tt.key, tt.args...)
		en := T(LangEnUS, tt.key, tt.args...)
		if got == en {
			t.Errorf("T(%q, %q) = %q, expected a non-English translation", tt.lang, tt.key, got)
		}
	}
}

func TestT_AllLanguagesHaveBrowserResultStatusKeys(t *testing.T) {
	tests := []struct {
		key  string
		args []interface{}
	}{
		{key: testMsgBrowserActionPerformedOnRef, args: []interface{}{"click", 7}},
		{key: testMsgBrowserScrolledPage, args: []interface{}{"down"}},
		{key: testMsgBrowserOpenTabs, args: []interface{}{2}},
		{key: testMsgBrowserTabClosed, args: []interface{}{"tab-7"}},
		{key: testMsgBrowserRecipesAvailable, args: []interface{}{3}},
		{key: testMsgBrowserLargeDOMNote, args: []interface{}{66}},
		{key: testMsgBrowserPage, args: []interface{}{"Example", "https://example.com", "TREE"}},
		{key: testMsgBrowserPageWithInteractiveCount, args: []interface{}{"Example", "https://example.com", 2, "TREE"}},
		{key: testMsgBrowserPageWithScreenshotInteractiveCount, args: []interface{}{"Example", "https://example.com", 2, "TREE"}},
		{key: testMsgBrowserPageMainContent, args: []interface{}{"Example", "https://example.com", "CONTENT"}},
		{key: testMsgBrowserPageMainContentWithSection, args: []interface{}{"Example", "https://example.com", "CONTENT", "Interactive elements", "TREE"}},
		{key: testMsgBrowserPageStructureSection},
		{key: testMsgBrowserInteractiveElementsSection},
	}

	for _, lang := range testAllLanguages() {
		for _, tt := range tests {
			got := T(lang, tt.key, tt.args...)
			if got == tt.key {
				t.Errorf("T(%q, %q) returned key itself — missing translation", lang, tt.key)
			}
		}
	}
}

func TestT_BrowserResultStatusKeysAreLocalizedOutsideEnglish(t *testing.T) {
	protected := []struct {
		lang Language
		key  string
		args []interface{}
	}{
		{lang: LangZhCN, key: testMsgBrowserOpenTabs, args: []interface{}{2}},
		{lang: LangZhTW, key: testMsgBrowserTabClosed, args: []interface{}{"tab-7"}},
		{lang: LangJaJP, key: testMsgBrowserLargeDOMNote, args: []interface{}{66}},
		{lang: LangKoKR, key: testMsgBrowserPageWithInteractiveCount, args: []interface{}{"Example", "https://example.com", 2, "TREE"}},
		{lang: LangDeDE, key: testMsgBrowserPageMainContent, args: []interface{}{"Example", "https://example.com", "CONTENT"}},
		{lang: LangFrFR, key: testMsgBrowserPageStructureSection},
		{lang: LangEsES, key: testMsgBrowserInteractiveElementsSection},
		{lang: LangItIT, key: testMsgBrowserRecipesAvailable, args: []interface{}{3}},
		{lang: LangPtBR, key: testMsgBrowserScrolledPage, args: []interface{}{"down"}},
		{lang: LangPtPT, key: testMsgBrowserActionPerformedOnRef, args: []interface{}{"click", 7}},
		{lang: LangRuRU, key: testMsgBrowserPageWithScreenshotInteractiveCount, args: []interface{}{"Example", "https://example.com", 2, "TREE"}},
		{lang: LangPlPL, key: testMsgBrowserOpenTabs, args: []interface{}{2}},
		{lang: LangNlNL, key: testMsgBrowserTabClosed, args: []interface{}{"tab-7"}},
		{lang: LangSvSE, key: testMsgBrowserLargeDOMNote, args: []interface{}{66}},
		{lang: LangDaDK, key: testMsgBrowserPageMainContentWithSection, args: []interface{}{"Example", "https://example.com", "CONTENT", "Interactive elements", "TREE"}},
		{lang: LangNbNO, key: testMsgBrowserPageStructureSection},
		{lang: LangCsCZ, key: testMsgBrowserInteractiveElementsSection},
		{lang: LangSkSK, key: testMsgBrowserRecipesAvailable, args: []interface{}{3}},
		{lang: LangHuHU, key: testMsgBrowserScrolledPage, args: []interface{}{"down"}},
		{lang: LangRoRO, key: testMsgBrowserActionPerformedOnRef, args: []interface{}{"click", 7}},
		{lang: LangHrHR, key: testMsgBrowserPageWithScreenshotInteractiveCount, args: []interface{}{"Example", "https://example.com", 2, "TREE"}},
		{lang: LangElGR, key: testMsgBrowserPageMainContent, args: []interface{}{"Example", "https://example.com", "CONTENT"}},
		{lang: LangCaES, key: testMsgBrowserPageStructureSection},
		{lang: LangGaIE, key: testMsgBrowserInteractiveElementsSection},
		{lang: LangMlIN, key: testMsgBrowserLargeDOMNote, args: []interface{}{66}},
	}

	for _, tt := range protected {
		got := T(tt.lang, tt.key, tt.args...)
		en := T(LangEnUS, tt.key, tt.args...)
		if got == en {
			t.Errorf("T(%q, %q) = %q, expected a non-English translation", tt.lang, tt.key, got)
		}
	}
}

func TestTranslations_AllLanguagesHaveToolLoopKeys(t *testing.T) {
	ensureTranslations()

	keys := []string{
		MsgToolLoopAbortRepeatedOverwrite,
		MsgToolLoopAbortIdenticalRepeat,
		MsgToolLoopAbortErrorRepeat,
		MsgToolLoopAbortPingPong,
		MsgToolLoopAbortPollingNoProgress,
		MsgToolLoopAbortGeneric,
		MsgToolLoopRecoveryRepeatedOverwrite,
		MsgToolLoopRecoveryGeneric,
	}

	mu.RLock()
	defer mu.RUnlock()

	for _, lang := range testAllLanguages() {
		msgs, ok := translations[lang]
		if !ok {
			t.Fatalf("translations[%q] missing", lang)
		}
		for _, key := range keys {
			got, ok := msgs[key]
			if !ok {
				t.Errorf("translations[%q][%q] missing explicit translation", lang, key)
				continue
			}
			if strings.TrimSpace(got) == "" {
				t.Errorf("translations[%q][%q] is blank", lang, key)
			}
		}
	}
}
