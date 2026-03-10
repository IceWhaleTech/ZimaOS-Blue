package server

import (
	"fmt"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/i18n"
)

type imBrowserProgressLocaleStrings struct {
	Browser    string
	Start      string
	Navigate   string
	Snapshot   string
	Screenshot string
	Recipe     string
}

var imBrowserProgressStrings = map[i18n.Language]imBrowserProgressLocaleStrings{
	i18n.LangEnUS: {Browser: "Browser", Start: "Starting browser", Navigate: "Navigating", Snapshot: "Reading page", Screenshot: "Capturing screenshot", Recipe: "Running %s"},
	i18n.LangEnGB: {Browser: "Browser", Start: "Starting browser", Navigate: "Navigating", Snapshot: "Reading page", Screenshot: "Capturing screenshot", Recipe: "Running %s"},
	i18n.LangZhCN: {Browser: "浏览器", Start: "正在启动浏览器", Navigate: "正在导航", Snapshot: "正在读取页面", Screenshot: "正在截取屏幕截图", Recipe: "正在运行 %s"},
	i18n.LangZhTW: {Browser: "瀏覽器", Start: "正在啟動瀏覽器", Navigate: "正在導覽", Snapshot: "正在讀取頁面", Screenshot: "正在擷取螢幕截圖", Recipe: "正在執行 %s"},
	i18n.LangJaJP: {Browser: "ブラウザ", Start: "ブラウザーを起動中", Navigate: "ナビゲート中", Snapshot: "ページを読み取り中", Screenshot: "スクリーンショットを撮影中", Recipe: "%s を実行中"},
	i18n.LangKoKR: {Browser: "브라우저", Start: "브라우저 시작 중", Navigate: "이동 중", Snapshot: "페이지 읽는 중", Screenshot: "스크린샷 캡처 중", Recipe: "%s 실행 중"},
	i18n.LangDeDE: {Browser: "Browser", Start: "Browser wird gestartet", Navigate: "Navigiert", Snapshot: "Seite wird gelesen", Screenshot: "Screenshot wird erstellt", Recipe: "%s wird ausgeführt"},
	i18n.LangFrFR: {Browser: "Navigateur", Start: "Démarrage du navigateur", Navigate: "Navigation", Snapshot: "Lecture de la page", Screenshot: "Capture d’écran", Recipe: "Exécution de %s"},
	i18n.LangEsES: {Browser: "Navegador", Start: "Iniciando navegador", Navigate: "Navegando", Snapshot: "Leyendo página", Screenshot: "Capturando pantalla", Recipe: "Ejecutando %s"},
	i18n.LangItIT: {Browser: "Browser", Start: "Avvio del browser", Navigate: "Navigazione", Snapshot: "Lettura della pagina", Screenshot: "Acquisizione schermata", Recipe: "Esecuzione di %s"},
	i18n.LangPtBR: {Browser: "Navegador", Start: "Iniciando navegador", Navigate: "Navegando", Snapshot: "Lendo página", Screenshot: "Capturando imagem da tela", Recipe: "Executando %s"},
	i18n.LangPtPT: {Browser: "Navegador", Start: "A iniciar o navegador", Navigate: "A navegar", Snapshot: "A ler a página", Screenshot: "A capturar imagem do ecrã", Recipe: "A executar %s"},
	i18n.LangRuRU: {Browser: "Браузер", Start: "Запуск браузера", Navigate: "Навигация", Snapshot: "Чтение страницы", Screenshot: "Создание снимка экрана", Recipe: "Запуск %s"},
	i18n.LangPlPL: {Browser: "Przeglądarka", Start: "Uruchamianie przeglądarki", Navigate: "Nawigacja", Snapshot: "Odczytywanie strony", Screenshot: "Przechwytywanie zrzutu ekranu", Recipe: "Uruchamianie %s"},
	i18n.LangNlNL: {Browser: "Browser", Start: "Browser starten", Navigate: "Navigeren", Snapshot: "Pagina lezen", Screenshot: "Screenshot maken", Recipe: "%s uitvoeren"},
	i18n.LangSvSE: {Browser: "Webbläsare", Start: "Startar webbläsare", Navigate: "Navigerar", Snapshot: "Läser sida", Screenshot: "Tar skärmdump", Recipe: "Kör %s"},
	i18n.LangDaDK: {Browser: "Browser", Start: "Starter browser", Navigate: "Navigerer", Snapshot: "Læser side", Screenshot: "Tager skærmbillede", Recipe: "Kører %s"},
	i18n.LangNbNO: {Browser: "Nettleser", Start: "Starter nettleser", Navigate: "Navigerer", Snapshot: "Leser side", Screenshot: "Tar skjermbilde", Recipe: "Kjører %s"},
	i18n.LangCsCZ: {Browser: "Prohlížeč", Start: "Spouštění prohlížeče", Navigate: "Navigace", Snapshot: "Čtení stránky", Screenshot: "Pořizování snímku obrazovky", Recipe: "Spouštění %s"},
	i18n.LangSkSK: {Browser: "Prehliadač", Start: "Spúšťanie prehliadača", Navigate: "Navigácia", Snapshot: "Čítanie stránky", Screenshot: "Zhotovovanie snímky obrazovky", Recipe: "Spúšťanie %s"},
	i18n.LangHuHU: {Browser: "Böngésző", Start: "Böngésző indítása", Navigate: "Navigálás", Snapshot: "Oldal olvasása", Screenshot: "Képernyőkép készítése", Recipe: "%s futtatása"},
	i18n.LangRoRO: {Browser: "Browser", Start: "Pornire browser", Navigate: "Navigare", Snapshot: "Citire pagină", Screenshot: "Realizare captură de ecran", Recipe: "Rulare %s"},
	i18n.LangHrHR: {Browser: "Preglednik", Start: "Pokretanje preglednika", Navigate: "Navigacija", Snapshot: "Čitanje stranice", Screenshot: "Snimanje zaslona", Recipe: "Pokretanje %s"},
	i18n.LangElGR: {Browser: "Πρόγραμμα περιήγησης", Start: "Εκκίνηση προγράμματος περιήγησης", Navigate: "Πλοήγηση", Snapshot: "Ανάγνωση σελίδας", Screenshot: "Λήψη στιγμιότυπου οθόνης", Recipe: "Εκτέλεση %s"},
	i18n.LangCaES: {Browser: "Navegador", Start: "S'està iniciant el navegador", Navigate: "S'està navegant", Snapshot: "S'està llegint la pàgina", Screenshot: "S'està capturant la pantalla", Recipe: "S'està executant %s"},
	i18n.LangGaIE: {Browser: "Brabhsálaí", Start: "Brabhsálaí á thosú", Navigate: "Ag nascleanúint", Snapshot: "Leathanach á léamh", Screenshot: "Gabháil scáileáin á dhéanamh", Recipe: "%s á rith"},
	i18n.LangMlIN: {Browser: "ബ്രൗസർ", Start: "ബ്രൗസർ ആരംഭിക്കുന്നു", Navigate: "നാവിഗേറ്റ് ചെയ്യുന്നു", Snapshot: "പേജ് വായിക്കുന്നു", Screenshot: "സ്ക്രീൻഷോട്ട് പകർത്തുന്നു", Recipe: "%s പ്രവർത്തിപ്പിക്കുന്നു"},
}

func imBrowserProgressLocale(lang i18n.Language) imBrowserProgressLocaleStrings {
	if localized, ok := imBrowserProgressStrings[i18n.ParseLanguage(string(lang))]; ok {
		return localized
	}
	return imBrowserProgressStrings[i18n.DefaultLanguage]
}

func imBrowserProgressRecipeName(card map[string]interface{}, stepName string) string {
	if recipeName, _ := card["recipe_name"].(string); strings.TrimSpace(recipeName) != "" {
		return strings.TrimSpace(recipeName)
	}
	trimmed := strings.TrimSpace(stepName)
	if strings.HasPrefix(trimmed, "Running ") {
		return strings.TrimSpace(strings.TrimPrefix(trimmed, "Running "))
	}
	return ""
}

func localizedIMBrowserProgressStep(lang i18n.Language, card map[string]interface{}) string {
	stepID, _ := card["step"].(string)
	stepName, _ := card["name"].(string)
	if stepName == "" {
		stepName = stepID
	}
	localized := imBrowserProgressLocale(lang)
	switch strings.ToLower(strings.TrimSpace(stepID)) {
	case "start":
		return localized.Start
	case "navigate":
		return localized.Navigate
	case "snapshot":
		return localized.Snapshot
	case "screenshot":
		return localized.Screenshot
	case "recipe":
		recipeName := imBrowserProgressRecipeName(card, stepName)
		if recipeName == "" {
			return stepName
		}
		return fmt.Sprintf(localized.Recipe, recipeName)
	default:
		return stepName
	}
}
