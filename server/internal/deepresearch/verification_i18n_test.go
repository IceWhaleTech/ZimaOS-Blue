package deepresearch

import "testing"

func TestLocalizedVerificationLabels_CoverAllSupportedLocales(t *testing.T) {
	cases := []struct {
		lang         string
		axis         string
		focus        string
		heading      string
		resolved     string
		conflicted   string
		insufficient string
	}{
		{"en-US", "Overview", "Source diversity", "Verification summary", "Resolved", "Conflicted", "Insufficient"},
		{"en-GB", "Overview", "Source diversity", "Verification summary", "Resolved", "Conflicted", "Insufficient"},
		{"zh-CN", "概览", "来源多样性", "核验摘要", "已核实", "有冲突", "待补证"},
		{"zh-TW", "概覽", "來源多樣性", "核驗摘要", "已核實", "有衝突", "待補證"},
		{"ja-JP", "概要", "情報源の多様性", "検証サマリー", "確認済み", "競合あり", "根拠不足"},
		{"ko-KR", "개요", "출처 다양성", "검증 요약", "확인됨", "충돌 있음", "근거 부족"},
		{"de-DE", "Überblick", "Quellenvielfalt", "Verifizierungsübersicht", "Verifiziert", "Widersprüchlich", "Unzureichend"},
		{"fr-FR", "Vue d'ensemble", "Diversité des sources", "Résumé de vérification", "Vérifié", "Contradictoire", "Insuffisant"},
		{"es-ES", "Resumen general", "Diversidad de fuentes", "Resumen de verificación", "Verificado", "En conflicto", "Insuficiente"},
		{"it-IT", "Panoramica", "Diversità delle fonti", "Riepilogo verifica", "Verificato", "In conflitto", "Insufficiente"},
		{"pt-BR", "Visão geral", "Diversidade de fontes", "Resumo de verificação", "Verificado", "Conflitante", "Insuficiente"},
		{"pt-PT", "Visão geral", "Diversidade de fontes", "Resumo de verificação", "Verificado", "Conflitante", "Insuficiente"},
		{"ru-RU", "Обзор", "Разнообразие источников", "Сводка проверки", "Проверено", "Есть конфликт", "Недостаточно"},
		{"pl-PL", "Przegląd", "Różnorodność źródeł", "Podsumowanie weryfikacji", "Zweryfikowano", "Sprzeczne", "Niewystarczające"},
		{"nl-NL", "Overzicht", "Brondiversiteit", "Verificatieoverzicht", "Geverifieerd", "Tegenstrijdig", "Onvoldoende"},
		{"sv-SE", "Översikt", "Källmångfald", "Verifieringsöversikt", "Verifierad", "Motstridigt", "Otillräckligt"},
		{"da-DK", "Oversigt", "Kildediversitet", "Verifikationsoversigt", "Verificeret", "Modstridende", "Utilstrækkelig"},
		{"nb-NO", "Oversikt", "Kildemangfold", "Verifiseringsoversikt", "Verifisert", "Motstridende", "Utilstrekkelig"},
		{"cs-CZ", "Přehled", "Rozmanitost zdrojů", "Souhrn ověření", "Ověřeno", "V konfliktu", "Nedostatečné"},
		{"sk-SK", "Prehľad", "Rozmanitosť zdrojov", "Súhrn overenia", "Overené", "V konflikte", "Nedostatočné"},
		{"hu-HU", "Áttekintés", "Források sokfélesége", "Ellenőrzési összefoglaló", "Ellenőrizve", "Ellentmondásos", "Elégtelen"},
		{"ro-RO", "Prezentare generală", "Diversitatea surselor", "Rezumat verificare", "Verificat", "În conflict", "Insuficient"},
		{"hr-HR", "Pregled", "Raznolikost izvora", "Sažetak provjere", "Provjereno", "U sukobu", "Nedovoljno"},
		{"el-GR", "Επισκόπηση", "Ποικιλία πηγών", "Σύνοψη επαλήθευσης", "Επαληθευμένο", "Σε σύγκρουση", "Ανεπαρκές"},
		{"ca-ES", "Visió general", "Diversitat de fonts", "Resum de verificació", "Verificat", "En conflicte", "Insuficient"},
		{"ga-IE", "Forléargas", "Éagsúlacht foinsí", "Achoimre fíoraithe", "Fíoraithe", "Contrártha", "Neamhdhóthanach"},
		{"ml-IN", "അവലോകനം", "ഉറവിടങ്ങളുടെ വൈവിധ്യം", "സ്ഥിരീകരണ സംഗ്രഹം", "സ്ഥിരീകരിച്ചു", "വൈരുദ്ധ്യമുണ്ട്", "അപര്യാപ്തം"},
	}
	for _, tc := range cases {
		if got := localizedAxisLabel(tc.lang, "overview", "Overview"); got != tc.axis {
			t.Fatalf("localizedAxisLabel(%q)=%q, want %q", tc.lang, got, tc.axis)
		}
		if got := localizedFocusLabel(tc.lang, "source_diversity", "Source diversity"); got != tc.focus {
			t.Fatalf("localizedFocusLabel(%q)=%q, want %q", tc.lang, got, tc.focus)
		}
		if got := localizedVerificationHeading(tc.lang, "test"); got != tc.heading {
			t.Fatalf("localizedVerificationHeading(%q)=%q, want %q", tc.lang, got, tc.heading)
		}
		if got := localizedVerificationStatusLabel(tc.lang, verificationStatusResolved); got != tc.resolved {
			t.Fatalf("resolved status for %q=%q, want %q", tc.lang, got, tc.resolved)
		}
		if got := localizedVerificationStatusLabel(tc.lang, verificationStatusConflicted); got != tc.conflicted {
			t.Fatalf("conflicted status for %q=%q, want %q", tc.lang, got, tc.conflicted)
		}
		if got := localizedVerificationStatusLabel(tc.lang, verificationStatusInsufficient); got != tc.insufficient {
			t.Fatalf("insufficient status for %q=%q, want %q", tc.lang, got, tc.insufficient)
		}
	}
}

func TestLocalizedVerificationLabels_AdditionalKeys(t *testing.T) {
	cases := []struct {
		got  string
		want string
	}{
		{localizedAxisLabel("de-DE", "official", "Official sources"), "Offizielle Quellen"},
		{localizedAxisLabel("ja-JP", "best_practices", "Best practices"), "ベストプラクティス"},
		{localizedAxisLabel("ml-IN", "retry", "Retry recovery"), "വീണ്ടും ശ്രമിച്ച് തിരുത്തൽ"},
		{localizedFocusLabel("fr-FR", "freshness", "Freshness"), "Actualité"},
		{localizedFocusLabel("ko-KR", "claim_validation", "Claim validation"), "주장 검증"},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Fatalf("got %q, want %q", tc.got, tc.want)
		}
	}
}

func TestLocalizedVerificationSummaries_LocalizeRepresentativeLocales(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{
			name: "gap summary de",
			got:  localizedGapSummary("de-DE", "Überblick", 0, 0),
			want: "Überblick deckt nur 0 Beleg(e) über 0 Domain(s) ab; weitere Recherche ist nötig.",
		},
		{
			name: "official gap fr",
			got:  localizedOfficialGapSummary("fr-FR", "Sources officielles"),
			want: "Sources officielles ne dispose pas encore de sources primaires ou officielles stables.",
		},
		{
			name: "resolved ja",
			got:  localizedResolvedSummary("ja-JP", "概要", 2, 1),
			want: "概要 は 2 件の根拠と 1 件のドメインで裏付けられています。",
		},
		{
			name: "diversity gap ko",
			got:  localizedDiversityGapSummary("ko-KR", 1),
			want: "현재 확보한 고유 도메인은 1곳뿐이어서 출처 다양성 확대가 필요합니다.",
		},
		{
			name: "freshness gap it",
			got:  localizedFreshnessGapSummary("it-IT", 2022),
			want: "La prova confermata più recente risale al 2022 e non è abbastanza attuale.",
		},
		{
			name: "freshness gap no year it",
			got:  localizedFreshnessGapSummary("it-IT", 0),
			want: "Non è stato possibile confermare prove sufficientemente recenti.",
		},
		{
			name: "freshness resolved pt",
			got:  localizedFreshnessResolvedSummary("pt-BR", 2025),
			want: "A evidência recente chega até 2025.",
		},
		{
			name: "conflict gap es",
			got:  localizedConflictGapSummary("es-ES", 2, 1),
			want: "Se detectaron 2 grupo(s) de respaldo y 1 grupo(s) en conflicto; verifique con fuentes primarias u oficiales.",
		},
		{
			name: "conflict resolved nl",
			got:  localizedConflictResolvedSummary("nl-NL", 3),
			want: "Kernconclusies worden ondersteund door 3 claimgroep(en).",
		},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Fatalf("%s: got %q, want %q", tc.name, tc.got, tc.want)
		}
	}
}
