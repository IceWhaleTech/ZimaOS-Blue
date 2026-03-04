package deepresearch

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/i18n"
)

type researchLang string

const (
	researchLangEN researchLang = "en"
	researchLangZH researchLang = "zh"
	researchLangJA researchLang = "ja"
	researchLangKO researchLang = "ko"
)

func normalizeResearchLang(lang, fallbackText string) researchLang {
	switch parseResearchLocale(lang, fallbackText) {
	case i18n.LangZhCN, i18n.LangZhTW:
		return researchLangZH
	case i18n.LangJaJP:
		return researchLangJA
	case i18n.LangKoKR:
		return researchLangKO
	default:
		return researchLangEN
	}
}

func parseResearchLocale(lang, fallbackText string) i18n.Language {
	raw := strings.TrimSpace(lang)
	if raw != "" {
		return i18n.ParseLanguage(raw)
	}
	switch detectResearchLangFromText(fallbackText) {
	case researchLangZH:
		return i18n.LangZhCN
	case researchLangJA:
		return i18n.LangJaJP
	case researchLangKO:
		return i18n.LangKoKR
	default:
		return i18n.LangEnUS
	}
}

func detectResearchLangFromText(text string) researchLang {
	for _, r := range text {
		switch {
		case unicode.In(r, unicode.Hiragana, unicode.Katakana):
			return researchLangJA
		case unicode.In(r, unicode.Hangul):
			return researchLangKO
		case unicode.In(r, unicode.Han):
			return researchLangZH
		}
	}
	return researchLangEN
}

func planQueriesForLang(query, lang string) []string {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil
	}

	switch normalizeResearchLang(lang, q) {
	case researchLangZH:
		return []string{
			q,
			fmt.Sprintf("%s 最新进展", q),
			fmt.Sprintf("%s 官方文档", q),
			fmt.Sprintf("%s 基准对比", q),
			fmt.Sprintf("%s 最佳实践", q),
		}
	case researchLangJA:
		return []string{
			q,
			fmt.Sprintf("%s 最新動向", q),
			fmt.Sprintf("%s 公式ドキュメント", q),
			fmt.Sprintf("%s ベンチマーク比較", q),
			fmt.Sprintf("%s ベストプラクティス", q),
		}
	case researchLangKO:
		return []string{
			q,
			fmt.Sprintf("%s 최신 동향", q),
			fmt.Sprintf("%s 공식 문서", q),
			fmt.Sprintf("%s 벤치마크 비교", q),
			fmt.Sprintf("%s 모범 사례", q),
		}
	default:
		return []string{
			q,
			fmt.Sprintf("%s latest updates", q),
			fmt.Sprintf("%s official documentation", q),
			fmt.Sprintf("%s benchmark comparison", q),
			fmt.Sprintf("%s best practices", q),
		}
	}
}

func searchRegionForLang(lang, query string) string {
	raw := strings.ToLower(strings.TrimSpace(lang))
	switch {
	case strings.HasPrefix(raw, "en-us"):
		return "us-en"
	case strings.HasPrefix(raw, "en-gb"):
		return "uk-en"
	case strings.HasPrefix(raw, "ja"):
		return "jp-jp"
	case strings.HasPrefix(raw, "ko"):
		return "kr-kr"
	case strings.HasPrefix(raw, "zh-cn"), strings.HasPrefix(raw, "zh-hans"):
		return "cn-zh"
	case strings.HasPrefix(raw, "zh-tw"), strings.HasPrefix(raw, "zh-hant"), strings.HasPrefix(raw, "zh-hk"):
		return "tw-tzh"
	}

	switch normalizeResearchLang(lang, query) {
	case researchLangJA:
		return "jp-jp"
	case researchLangKO:
		return "kr-kr"
	case researchLangZH:
		return "cn-zh"
	default:
		return "wt-wt"
	}
}

func localizedSummaryTitle(query, lang string) string {
	switch parseResearchLocale(lang, query) {
	case i18n.LangZhCN:
		return fmt.Sprintf("调研总结：%s", query)
	case i18n.LangZhTW:
		return fmt.Sprintf("調研總結：%s", query)
	case i18n.LangJaJP:
		return fmt.Sprintf("調査サマリー: %s", query)
	case i18n.LangKoKR:
		return fmt.Sprintf("리서치 요약: %s", query)
	case i18n.LangDeDE:
		return fmt.Sprintf("Recherche-Zusammenfassung: %s", query)
	case i18n.LangFrFR:
		return fmt.Sprintf("Résumé de recherche : %s", query)
	case i18n.LangEsES:
		return fmt.Sprintf("Resumen de investigación: %s", query)
	case i18n.LangItIT:
		return fmt.Sprintf("Riepilogo ricerca: %s", query)
	case i18n.LangPtBR, i18n.LangPtPT:
		return fmt.Sprintf("Resumo da pesquisa: %s", query)
	case i18n.LangRuRU:
		return fmt.Sprintf("Сводка исследования: %s", query)
	case i18n.LangPlPL:
		return fmt.Sprintf("Podsumowanie badania: %s", query)
	case i18n.LangNlNL:
		return fmt.Sprintf("Onderzoeks­samenvatting: %s", query)
	case i18n.LangSvSE:
		return fmt.Sprintf("Forskningssammanfattning: %s", query)
	case i18n.LangDaDK:
		return fmt.Sprintf("Forskningsresumé: %s", query)
	case i18n.LangNbNO:
		return fmt.Sprintf("Forskningssammendrag: %s", query)
	case i18n.LangCsCZ:
		return fmt.Sprintf("Shrnutí průzkumu: %s", query)
	case i18n.LangSkSK:
		return fmt.Sprintf("Súhrn prieskumu: %s", query)
	case i18n.LangHuHU:
		return fmt.Sprintf("Kutatási összefoglaló: %s", query)
	case i18n.LangRoRO:
		return fmt.Sprintf("Rezumat cercetare: %s", query)
	case i18n.LangHrHR:
		return fmt.Sprintf("Sažetak istraživanja: %s", query)
	case i18n.LangElGR:
		return fmt.Sprintf("Σύνοψη έρευνας: %s", query)
	case i18n.LangCaES:
		return fmt.Sprintf("Resum de recerca: %s", query)
	case i18n.LangGaIE:
		return fmt.Sprintf("Achoimre taighde: %s", query)
	case i18n.LangMlIN:
		return fmt.Sprintf("ഗവേഷണ സംഗ്രഹം: %s", query)
	default:
		return fmt.Sprintf("Research summary for: %s", query)
	}
}

func localizedNoEvidence(lang, query string) string {
	switch parseResearchLocale(lang, query) {
	case i18n.LangZhCN:
		return "未收集到足够证据。"
	case i18n.LangZhTW:
		return "未收集到足夠證據。"
	case i18n.LangJaJP:
		return "十分な根拠を収集できませんでした。"
	case i18n.LangKoKR:
		return "충분한 근거를 수집하지 못했습니다."
	case i18n.LangDeDE:
		return "Es konnten nicht genügend Belege gesammelt werden."
	case i18n.LangFrFR:
		return "Aucune preuve suffisante n'a été collectée."
	case i18n.LangEsES:
		return "No se recopilaron pruebas suficientes."
	case i18n.LangItIT:
		return "Non sono state raccolte prove sufficienti."
	case i18n.LangPtBR, i18n.LangPtPT:
		return "Não foram coletadas evidências suficientes."
	case i18n.LangRuRU:
		return "Не удалось собрать достаточные доказательства."
	case i18n.LangPlPL:
		return "Nie zebrano wystarczających dowodów."
	case i18n.LangNlNL:
		return "Er is onvoldoende bewijs verzameld."
	case i18n.LangSvSE:
		return "Tillräckliga bevis kunde inte samlas in."
	case i18n.LangDaDK:
		return "Der blev ikke indsamlet tilstrækkelig evidens."
	case i18n.LangNbNO:
		return "Det ble ikke samlet inn tilstrekkelig dokumentasjon."
	case i18n.LangCsCZ:
		return "Nebylo shromážděno dostatek důkazů."
	case i18n.LangSkSK:
		return "Nepodarilo sa zhromaždiť dostatok dôkazov."
	case i18n.LangHuHU:
		return "Nem sikerült elegendő bizonyítékot gyűjteni."
	case i18n.LangRoRO:
		return "Nu au fost colectate dovezi suficiente."
	case i18n.LangHrHR:
		return "Nije prikupljeno dovoljno dokaza."
	case i18n.LangElGR:
		return "Δεν συγκεντρώθηκαν επαρκή αποδεικτικά στοιχεία."
	case i18n.LangCaES:
		return "No s'han recopilat proves suficients."
	case i18n.LangGaIE:
		return "Níor bailíodh go leor fianaise."
	case i18n.LangMlIN:
		return "പര്യാപ്തമായ തെളിവുകൾ ശേഖരിക്കാനായില്ല."
	default:
		return "No sufficient evidence was collected."
	}
}

func localizedOpenQuestion(lang, query string) string {
	switch parseResearchLocale(lang, query) {
	case i18n.LangZhCN:
		return "建议回查一手来源并进行最终核验。"
	case i18n.LangZhTW:
		return "建議回查一手來源並進行最終核驗。"
	case i18n.LangJaJP:
		return "最終確認のため一次情報を確認してください。"
	case i18n.LangKoKR:
		return "최종 검증을 위해 1차 출처를 확인하세요."
	case i18n.LangDeDE:
		return "Bitte zur finalen Verifikation Primärquellen prüfen."
	case i18n.LangFrFR:
		return "Vérifiez les sources primaires pour la validation finale."
	case i18n.LangEsES:
		return "Revise fuentes primarias para la verificación final."
	case i18n.LangItIT:
		return "Verifica le fonti primarie per la validazione finale."
	case i18n.LangPtBR, i18n.LangPtPT:
		return "Verifique fontes primárias para validação final."
	case i18n.LangRuRU:
		return "Проверьте первоисточники для финальной верификации."
	case i18n.LangPlPL:
		return "Sprawdź źródła pierwotne dla końcowej weryfikacji."
	case i18n.LangNlNL:
		return "Controleer primaire bronnen voor definitieve verificatie."
	case i18n.LangSvSE:
		return "Kontrollera primärkällor för slutlig verifiering."
	case i18n.LangDaDK:
		return "Kontrollér primære kilder for endelig verifikation."
	case i18n.LangNbNO:
		return "Sjekk primærkilder for endelig verifisering."
	case i18n.LangCsCZ:
		return "Pro závěrečné ověření zkontrolujte primární zdroje."
	case i18n.LangSkSK:
		return "Na záverečné overenie skontrolujte primárne zdroje."
	case i18n.LangHuHU:
		return "A végső ellenőrzéshez vizsgálja meg az elsődleges forrásokat."
	case i18n.LangRoRO:
		return "Verificați sursele primare pentru validarea finală."
	case i18n.LangHrHR:
		return "Provjerite primarne izvore za završnu verifikaciju."
	case i18n.LangElGR:
		return "Ελέγξτε πρωτογενείς πηγές για τελική επαλήθευση."
	case i18n.LangCaES:
		return "Reviseu fonts primàries per a la verificació final."
	case i18n.LangGaIE:
		return "Seiceáil príomhfhoinsí le haghaidh fíoraithe deiridh."
	case i18n.LangMlIN:
		return "അവസാന സ്ഥിരീകരണത്തിനായി പ്രാഥമിക ഉറവിടങ്ങൾ പരിശോധിക്കുക."
	default:
		return "Check primary sources for final verification."
	}
}

func localizedConflictOpenQuestion(lang, query string) string {
	switch parseResearchLocale(lang, query) {
	case i18n.LangZhCN:
		return "不同来源存在冲突，建议优先核对官方或原始发布时间。"
	case i18n.LangZhTW:
		return "不同來源存在衝突，建議優先核對官方或原始發布時間。"
	case i18n.LangJaJP:
		return "情報源の内容に不一致があります。公式一次情報と公開日時を優先して確認してください。"
	case i18n.LangKoKR:
		return "출처 간 내용이 상충합니다. 공식 1차 출처와 게시 시점을 우선 확인하세요."
	case i18n.LangDeDE:
		return "Quellen widersprechen sich. Bitte zuerst offizielle Primärquellen und Veröffentlichungsdaten prüfen."
	case i18n.LangFrFR:
		return "Les sources sont contradictoires. Vérifiez d'abord les sources primaires officielles et les dates."
	case i18n.LangEsES:
		return "Las fuentes se contradicen. Verifique primero fuentes oficiales y fechas de publicación."
	case i18n.LangItIT:
		return "Le fonti sono in conflitto. Verifica prima fonti ufficiali e date di pubblicazione."
	case i18n.LangPtBR, i18n.LangPtPT:
		return "As fontes entram em conflito. Verifique primeiro fontes oficiais e datas de publicação."
	case i18n.LangRuRU:
		return "Источники противоречат друг другу. Сначала проверьте официальные первоисточники и даты публикации."
	case i18n.LangPlPL:
		return "Źródła są sprzeczne. Najpierw sprawdź oficjalne źródła pierwotne i daty publikacji."
	case i18n.LangNlNL:
		return "Bronnen spreken elkaar tegen. Controleer eerst officiële primaire bronnen en publicatiedata."
	case i18n.LangSvSE:
		return "Källorna är motstridiga. Kontrollera först officiella primärkällor och publiceringsdatum."
	case i18n.LangDaDK:
		return "Kilderne er modstridende. Kontrollér først officielle primære kilder og publiceringsdatoer."
	case i18n.LangNbNO:
		return "Kildene er motstridende. Sjekk først offisielle primærkilder og publiseringsdatoer."
	case i18n.LangCsCZ:
		return "Zdroje si odporují. Nejprve ověřte oficiální primární zdroje a data publikace."
	case i18n.LangSkSK:
		return "Zdroje si odporujú. Najprv overte oficiálne primárne zdroje a dátumy publikácie."
	case i18n.LangHuHU:
		return "A források ellentmondanak egymásnak. Először hivatalos elsődleges forrásokat és dátumokat ellenőrizzen."
	case i18n.LangRoRO:
		return "Sursele se contrazic. Verificați mai întâi sursele primare oficiale și datele publicării."
	case i18n.LangHrHR:
		return "Izvori su proturječni. Prvo provjerite službene primarne izvore i datume objave."
	case i18n.LangElGR:
		return "Οι πηγές είναι αντικρουόμενες. Ελέγξτε πρώτα επίσημες πρωτογενείς πηγές και ημερομηνίες δημοσίευσης."
	case i18n.LangCaES:
		return "Les fonts són contradictòries. Verifiqueu primer fonts oficials i dates de publicació."
	case i18n.LangGaIE:
		return "Tá foinsí contrártha. Seiceáil ar dtús príomhfhoinsí oifigiúla agus dátaí foilsithe."
	case i18n.LangMlIN:
		return "ഉറവിടങ്ങളിൽ വൈരുദ്ധ്യമുണ്ട്. ഔദ്യോഗിക പ്രാഥമിക ഉറവിടങ്ങളും പ്രസിദ്ധീകരണ തീയതികളും ആദ്യം പരിശോധിക്കുക."
	default:
		return "Sources contain conflicting claims. Verify against official primary sources and publication dates."
	}
}

func localizedSourceLabel(lang, query string) string {
	switch parseResearchLocale(lang, query) {
	case i18n.LangZhCN:
		return "来源"
	case i18n.LangZhTW:
		return "來源"
	case i18n.LangJaJP:
		return "情報源"
	case i18n.LangKoKR:
		return "출처"
	case i18n.LangDeDE:
		return "Quellen"
	case i18n.LangFrFR:
		return "Sources"
	case i18n.LangEsES:
		return "Fuentes"
	case i18n.LangItIT:
		return "Fonti"
	case i18n.LangPtBR, i18n.LangPtPT:
		return "Fontes"
	case i18n.LangRuRU:
		return "Источники"
	case i18n.LangPlPL:
		return "Źródła"
	case i18n.LangNlNL:
		return "Bronnen"
	case i18n.LangSvSE:
		return "Källor"
	case i18n.LangDaDK, i18n.LangNbNO:
		return "Kilder"
	case i18n.LangCsCZ, i18n.LangSkSK:
		return "Zdroje"
	case i18n.LangHuHU:
		return "Források"
	case i18n.LangRoRO:
		return "Surse"
	case i18n.LangHrHR:
		return "Izvori"
	case i18n.LangElGR:
		return "Πηγές"
	case i18n.LangCaES:
		return "Fonts"
	case i18n.LangGaIE:
		return "Foinsí"
	case i18n.LangMlIN:
		return "ഉറവിടങ്ങൾ"
	default:
		return "Sources"
	}
}

func localizedQuoteLabel(lang, query string) string {
	switch parseResearchLocale(lang, query) {
	case i18n.LangZhCN, i18n.LangZhTW, i18n.LangJaJP:
		return "引用"
	case i18n.LangKoKR:
		return "인용"
	case i18n.LangDeDE:
		return "Zitat"
	case i18n.LangFrFR:
		return "Citation"
	case i18n.LangEsES:
		return "Cita"
	case i18n.LangItIT:
		return "Citazione"
	case i18n.LangPtBR, i18n.LangPtPT:
		return "Citação"
	case i18n.LangRuRU:
		return "Цитата"
	case i18n.LangPlPL:
		return "Cytat"
	case i18n.LangNlNL:
		return "Citaat"
	case i18n.LangSvSE, i18n.LangDaDK:
		return "Citat"
	case i18n.LangNbNO:
		return "Sitat"
	case i18n.LangCsCZ:
		return "Citace"
	case i18n.LangSkSK:
		return "Citácia"
	case i18n.LangHuHU:
		return "Idézet"
	case i18n.LangRoRO, i18n.LangHrHR:
		return "Citat"
	case i18n.LangElGR:
		return "Παράθεση"
	case i18n.LangCaES:
		return "Cita"
	case i18n.LangGaIE:
		return "Lua"
	case i18n.LangMlIN:
		return "ഉദ്ധരണം"
	default:
		return "Quote"
	}
}

func localizedTimelineHeading(lang, query string) string {
	switch normalizeResearchLang(lang, query) {
	case researchLangZH:
		return "## 观点演变时间线"
	case researchLangJA:
		return "## 見解のタイムライン"
	case researchLangKO:
		return "## 관점 변화 타임라인"
	default:
		return "## Timeline Insights"
	}
}

func localizedStageErrorHint(lang, query string) string {
	switch normalizeResearchLang(lang, query) {
	case researchLangZH:
		return "部分检索任务失败，建议补充一手来源复核。"
	case researchLangJA:
		return "一部の検索ステップに失敗しました。一次情報で補完確認してください。"
	case researchLangKO:
		return "일부 검색 단계가 실패했습니다. 1차 출처로 추가 검증하세요."
	default:
		return "Some retrieval steps failed; verify with additional primary sources."
	}
}

func localizedCautiousConclusionPrefix(lang, query string) string {
	switch normalizeResearchLang(lang, query) {
	case researchLangZH:
		return "谨慎结论：当前引用覆盖率较低，请结合原始来源再次核验。"
	case researchLangJA:
		return "慎重な結論: 引用カバレッジが低いため、一次情報で再検証してください。"
	case researchLangKO:
		return "신중한 결론: 인용 커버리지가 낮아 1차 출처 재검증이 필요합니다."
	default:
		return "Cautious conclusion: citation coverage is below threshold; verify against primary sources."
	}
}

func localizedTimelineDefaultLabels(lang string) []string {
	switch i18n.ParseLanguage(lang) {
	case i18n.LangZhCN:
		return []string{"早期", "中期", "近期"}
	case i18n.LangZhTW:
		return []string{"早期", "中期", "近期"}
	case i18n.LangJaJP:
		return []string{"初期", "中期", "最近"}
	case i18n.LangKoKR:
		return []string{"초기", "중기", "최근"}
	case i18n.LangDeDE:
		return []string{"Frühphase", "Mittelphase", "Aktuell"}
	case i18n.LangFrFR:
		return []string{"Début", "Milieu", "Récent"}
	case i18n.LangEsES:
		return []string{"Temprano", "Intermedio", "Reciente"}
	case i18n.LangItIT:
		return []string{"Iniziale", "Intermedio", "Recente"}
	case i18n.LangPtBR, i18n.LangPtPT:
		return []string{"Inicial", "Intermediário", "Recente"}
	case i18n.LangRuRU:
		return []string{"Ранний", "Средний", "Недавний"}
	case i18n.LangPlPL:
		return []string{"Wczesny", "Średni", "Najnowszy"}
	case i18n.LangNlNL:
		return []string{"Vroeg", "Midden", "Recent"}
	case i18n.LangSvSE:
		return []string{"Tidigt", "Mellan", "Nyligen"}
	case i18n.LangDaDK:
		return []string{"Tidlig", "Midt", "Seneste"}
	case i18n.LangNbNO:
		return []string{"Tidlig", "Midt", "Nylig"}
	case i18n.LangCsCZ:
		return []string{"Raný", "Střední", "Nedávný"}
	case i18n.LangSkSK:
		return []string{"Skoré", "Stredné", "Nedávne"}
	case i18n.LangHuHU:
		return []string{"Korai", "Középső", "Friss"}
	case i18n.LangRoRO:
		return []string{"Timpuriu", "Mediu", "Recent"}
	case i18n.LangHrHR:
		return []string{"Rano", "Srednje", "Nedavno"}
	case i18n.LangElGR:
		return []string{"Πρώιμο", "Μεσαίο", "Πρόσφατο"}
	case i18n.LangCaES:
		return []string{"Inicial", "Mitjà", "Recent"}
	case i18n.LangGaIE:
		return []string{"Luath", "Lár", "Le Déanaí"}
	case i18n.LangMlIN:
		return []string{"ആദ്യഘട്ടം", "ഇടക്കാലം", "സമീപകാലം"}
	default:
		return []string{"Early", "Middle", "Recent"}
	}
}
