package server

import (
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/i18n"
)

func TestBrowserCheckpointAllowSiteLabel_LocalizedAcrossSupportedLanguages(t *testing.T) {
	tests := []struct {
		lang i18n.Language
		want string
	}{
		{lang: i18n.LangEnUS, want: "Always allow this site"},
		{lang: i18n.LangEnGB, want: "Always allow this site"},
		{lang: i18n.LangZhCN, want: "始终允许此网站"},
		{lang: i18n.LangZhTW, want: "一律允許此網站"},
		{lang: i18n.LangJaJP, want: "このサイトを常に許可する"},
		{lang: i18n.LangKoKR, want: "이 사이트를 항상 허용"},
		{lang: i18n.LangDeDE, want: "Erlauben Sie diese Seite immer"},
		{lang: i18n.LangFrFR, want: "Toujours autoriser ce site"},
		{lang: i18n.LangEsES, want: "Permitir siempre este sitio"},
		{lang: i18n.LangItIT, want: "Consenti sempre questo sito"},
		{lang: i18n.LangPtBR, want: "Sempre permitir este site"},
		{lang: i18n.LangPtPT, want: "Permitir sempre este site"},
		{lang: i18n.LangRuRU, want: "Всегда разрешать этот сайт"},
		{lang: i18n.LangPlPL, want: "Zawsze zezwalaj na tę witrynę"},
		{lang: i18n.LangNlNL, want: "Sta deze site altijd toe"},
		{lang: i18n.LangSvSE, want: "Tillåt alltid den här webbplatsen"},
		{lang: i18n.LangDaDK, want: "Tillad altid dette websted"},
		{lang: i18n.LangNbNO, want: "Tillat alltid dette nettstedet"},
		{lang: i18n.LangCsCZ, want: "Vždy povolit tento web"},
		{lang: i18n.LangSkSK, want: "Vždy povoliť túto stránku"},
		{lang: i18n.LangHuHU, want: "Mindig engedélyezze ezt a webhelyet"},
		{lang: i18n.LangRoRO, want: "Permiteți întotdeauna acest site"},
		{lang: i18n.LangHrHR, want: "Uvijek dopusti ovu stranicu"},
		{lang: i18n.LangElGR, want: "Να επιτρέπεται πάντα αυτός ο ιστότοπος"},
		{lang: i18n.LangCaES, want: "Permet sempre aquest lloc"},
		{lang: i18n.LangGaIE, want: "Ceadaigh an suíomh seo i gcónaí"},
		{lang: i18n.LangMlIN, want: "ഈ സൈറ്റ് എപ്പോഴും അനുവദിക്കുക"},
	}

	for _, tt := range tests {
		t.Run(string(tt.lang), func(t *testing.T) {
			got := browserCheckpointAllowSiteLabel(tt.lang)
			if got != tt.want {
				t.Fatalf("browserCheckpointAllowSiteLabel(%q) = %q, want %q", tt.lang, got, tt.want)
			}
		})
	}
}

func TestBrowserCheckpointAllowSiteLabel_FallsBackToEnglish(t *testing.T) {
	if got := browserCheckpointAllowSiteLabel(i18n.Language("zz-ZZ")); got != "Always allow this site" {
		t.Fatalf("browserCheckpointAllowSiteLabel(fallback) = %q, want %q", got, "Always allow this site")
	}
}
