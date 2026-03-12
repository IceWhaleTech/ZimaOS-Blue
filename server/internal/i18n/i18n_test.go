package i18n

import "testing"

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
	langs := []Language{
		LangEnUS, LangEnGB, LangZhCN, LangZhTW, LangJaJP, LangKoKR,
		LangDeDE, LangFrFR, LangEsES, LangItIT, LangPtBR, LangPtPT,
		LangRuRU, LangPlPL, LangNlNL, LangSvSE, LangDaDK, LangNbNO,
		LangCsCZ, LangSkSK, LangHuHU, LangRoRO, LangHrHR, LangElGR,
		LangCaES, LangGaIE, LangMlIN,
	}
	for _, lang := range langs {
		got := T(lang, MsgPathEscapesWorkspaceRoot)
		if got == MsgPathEscapesWorkspaceRoot {
			t.Errorf("T(%q, %q) returned key itself — missing translation", lang, MsgPathEscapesWorkspaceRoot)
		}
	}
}
