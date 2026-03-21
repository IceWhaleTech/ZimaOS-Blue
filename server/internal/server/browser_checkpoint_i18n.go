package server

import (
	"fmt"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/i18n"
)

type browserCheckpointLocaleStrings struct {
	Header          string
	Question        string
	Continue        string
	Cancel          string
	StepLabel       string
	ActionLabel     string
	URLLabel        string
	ScreenshotLabel string
	IMIntro         string
	ReplyHint       string
	StepAct         string
	StepRecipe      string
	StepRelay       string
	ActionClick     string
	ActionType      string
	ActionSelect    string
	ActionSubmit    string
	ActionRelayUse  string
	ActionRelayTabs string
	ActionRelayView string
	RecipeLogin     string
	RecipeFillForm  string
}

var browserCheckpointStrings = map[i18n.Language]browserCheckpointLocaleStrings{
	i18n.LangEnUS: {Header: "Confirm", Question: "Browser action needs confirmation. Continue?", Continue: "Continue", Cancel: "Cancel", StepLabel: "Step", ActionLabel: "Action", URLLabel: "URL", ScreenshotLabel: "Screenshot", IMIntro: "High-risk browser action needs confirmation.", ReplyHint: "Reply `1` to continue or `2` to cancel.", StepAct: "Interaction", StepRecipe: "Recipe", StepRelay: "Relay mode", ActionClick: "Click", ActionType: "Type", ActionSelect: "Select", ActionSubmit: "Submit", ActionRelayUse: "Use connected browser session", ActionRelayTabs: "List connected tabs", ActionRelayView: "Inspect connected page", RecipeLogin: "Log in", RecipeFillForm: "Fill form"},
	i18n.LangEnGB: {Header: "Confirm", Question: "Browser action needs confirmation. Continue?", Continue: "Continue", Cancel: "Cancel", StepLabel: "Step", ActionLabel: "Action", URLLabel: "URL", ScreenshotLabel: "Screenshot", IMIntro: "High-risk browser action needs confirmation.", ReplyHint: "Reply `1` to continue or `2` to cancel.", StepAct: "Interaction", StepRecipe: "Recipe", StepRelay: "Relay mode", ActionClick: "Click", ActionType: "Type", ActionSelect: "Select", ActionSubmit: "Submit", ActionRelayUse: "Use connected browser session", ActionRelayTabs: "List connected tabs", ActionRelayView: "Inspect connected page", RecipeLogin: "Log in", RecipeFillForm: "Fill form"},
	i18n.LangZhCN: {Header: "确认", Question: "浏览器操作需要确认。是否继续？", Continue: "继续", Cancel: "取消", StepLabel: "步骤", ActionLabel: "动作", URLLabel: "URL", ScreenshotLabel: "截图", IMIntro: "高风险浏览器操作需要确认。", ReplyHint: "回复 `1` 继续，或回复 `2` 取消。", StepAct: "交互", StepRecipe: "配方", StepRelay: "Relay 模式", ActionClick: "点击", ActionType: "输入", ActionSelect: "选择", ActionSubmit: "提交", ActionRelayUse: "使用已连接的浏览器会话", ActionRelayTabs: "查看已连接标签页", ActionRelayView: "查看已连接页面", RecipeLogin: "登录", RecipeFillForm: "填写表单"},
	i18n.LangZhTW: {Header: "確認", Question: "瀏覽器操作需要確認。是否繼續？", Continue: "繼續", Cancel: "取消", StepLabel: "步驟", ActionLabel: "動作", URLLabel: "URL", ScreenshotLabel: "截圖", IMIntro: "高風險瀏覽器操作需要確認。", ReplyHint: "回覆 `1` 繼續，或回覆 `2` 取消。", StepAct: "互動", StepRecipe: "配方", StepRelay: "Relay 模式", ActionClick: "點擊", ActionType: "輸入", ActionSelect: "選取", ActionSubmit: "提交", ActionRelayUse: "使用已連接的瀏覽器工作階段", ActionRelayTabs: "檢視已連接分頁", ActionRelayView: "檢視已連接頁面", RecipeLogin: "登入", RecipeFillForm: "填寫表單"},
	i18n.LangJaJP: {Header: "確認", Question: "ブラウザー操作に確認が必要です。続行しますか？", Continue: "続行", Cancel: "キャンセル", StepLabel: "手順", ActionLabel: "操作", URLLabel: "URL", ScreenshotLabel: "スクリーンショット", IMIntro: "高リスクのブラウザー操作には確認が必要です。", ReplyHint: "`1` で続行、`2` でキャンセルしてください。", StepAct: "操作", StepRecipe: "レシピ", ActionClick: "クリック", ActionType: "入力", ActionSelect: "選択", ActionSubmit: "送信", RecipeLogin: "ログイン", RecipeFillForm: "フォーム入力"},
	i18n.LangKoKR: {Header: "확인", Question: "브라우저 작업은 확인이 필요합니다. 계속할까요?", Continue: "계속", Cancel: "취소", StepLabel: "단계", ActionLabel: "작업", URLLabel: "URL", ScreenshotLabel: "스크린샷", IMIntro: "고위험 브라우저 작업은 확인이 필요합니다.", ReplyHint: "계속하려면 `1`, 취소하려면 `2`로 답하세요.", StepAct: "상호작용", StepRecipe: "레시피", ActionClick: "클릭", ActionType: "입력", ActionSelect: "선택", ActionSubmit: "제출", RecipeLogin: "로그인", RecipeFillForm: "양식 작성"},
	i18n.LangDeDE: {Header: "Bestätigen", Question: "Browser-Aktion benötigt Bestätigung. Fortfahren?", Continue: "Fortfahren", Cancel: "Abbrechen", StepLabel: "Schritt", ActionLabel: "Aktion", URLLabel: "URL", ScreenshotLabel: "Screenshot", IMIntro: "Browser-Aktion mit hohem Risiko benötigt Bestätigung.", ReplyHint: "Mit `1` fortfahren oder mit `2` abbrechen.", StepAct: "Interaktion", StepRecipe: "Rezept", ActionClick: "Klicken", ActionType: "Eingeben", ActionSelect: "Auswählen", ActionSubmit: "Absenden", RecipeLogin: "Anmelden", RecipeFillForm: "Formular ausfüllen"},
	i18n.LangFrFR: {Header: "Confirmer", Question: "L'action du navigateur nécessite une confirmation. Continuer ?", Continue: "Continuer", Cancel: "Annuler", StepLabel: "Étape", ActionLabel: "Action", URLLabel: "URL", ScreenshotLabel: "Capture", IMIntro: "L'action du navigateur à haut risque nécessite une confirmation.", ReplyHint: "Répondez `1` pour continuer ou `2` pour annuler.", StepAct: "Interaction", StepRecipe: "Recette", ActionClick: "Cliquer", ActionType: "Saisir", ActionSelect: "Sélectionner", ActionSubmit: "Soumettre", RecipeLogin: "Connexion", RecipeFillForm: "Remplir le formulaire"},
	i18n.LangEsES: {Header: "Confirmar", Question: "La acción del navegador requiere confirmación. ¿Continuar?", Continue: "Continuar", Cancel: "Cancelar", StepLabel: "Paso", ActionLabel: "Acción", URLLabel: "URL", ScreenshotLabel: "Captura", IMIntro: "La acción del navegador de alto riesgo requiere confirmación.", ReplyHint: "Responde `1` para continuar o `2` para cancelar.", StepAct: "Interacción", StepRecipe: "Receta", ActionClick: "Hacer clic", ActionType: "Escribir", ActionSelect: "Seleccionar", ActionSubmit: "Enviar", RecipeLogin: "Iniciar sesión", RecipeFillForm: "Rellenar formulario"},
	i18n.LangItIT: {Header: "Conferma", Question: "L'azione del browser richiede conferma. Continuare?", Continue: "Continua", Cancel: "Annulla", StepLabel: "Passo", ActionLabel: "Azione", URLLabel: "URL", ScreenshotLabel: "Screenshot", IMIntro: "L'azione del browser ad alto rischio richiede conferma.", ReplyHint: "Rispondi `1` per continuare o `2` per annullare.", StepAct: "Interazione", StepRecipe: "Ricetta", ActionClick: "Clic", ActionType: "Inserimento", ActionSelect: "Selezione", ActionSubmit: "Invio", RecipeLogin: "Accesso", RecipeFillForm: "Compilazione modulo"},
	i18n.LangPtBR: {Header: "Confirmar", Question: "A ação do navegador precisa de confirmação. Continuar?", Continue: "Continuar", Cancel: "Cancelar", StepLabel: "Etapa", ActionLabel: "Ação", URLLabel: "URL", ScreenshotLabel: "Captura", IMIntro: "A ação de alto risco do navegador precisa de confirmação.", ReplyHint: "Responda `1` para continuar ou `2` para cancelar.", StepAct: "Interação", StepRecipe: "Receita", ActionClick: "Clicar", ActionType: "Digitar", ActionSelect: "Selecionar", ActionSubmit: "Enviar", RecipeLogin: "Fazer login", RecipeFillForm: "Preencher formulário"},
	i18n.LangPtPT: {Header: "Confirmar", Question: "A ação do navegador precisa de confirmação. Continuar?", Continue: "Continuar", Cancel: "Cancelar", StepLabel: "Passo", ActionLabel: "Ação", URLLabel: "URL", ScreenshotLabel: "Captura", IMIntro: "A ação de alto risco do navegador precisa de confirmação.", ReplyHint: "Responda `1` para continuar ou `2` para cancelar.", StepAct: "Interação", StepRecipe: "Receita", ActionClick: "Clicar", ActionType: "Escrever", ActionSelect: "Selecionar", ActionSubmit: "Submeter", RecipeLogin: "Iniciar sessão", RecipeFillForm: "Preencher formulário"},
	i18n.LangRuRU: {Header: "Подтвердить", Question: "Действие браузера требует подтверждения. Продолжить?", Continue: "Продолжить", Cancel: "Отмена", StepLabel: "Шаг", ActionLabel: "Действие", URLLabel: "URL", ScreenshotLabel: "Снимок", IMIntro: "Высокорисковое действие браузера требует подтверждения.", ReplyHint: "Ответьте `1`, чтобы продолжить, или `2`, чтобы отменить.", StepAct: "Взаимодействие", StepRecipe: "Рецепт", ActionClick: "Нажатие", ActionType: "Ввод", ActionSelect: "Выбор", ActionSubmit: "Отправка", RecipeLogin: "Вход", RecipeFillForm: "Заполнение формы"},
	i18n.LangPlPL: {Header: "Potwierdź", Question: "Działanie przeglądarki wymaga potwierdzenia. Kontynuować?", Continue: "Kontynuuj", Cancel: "Anuluj", StepLabel: "Krok", ActionLabel: "Akcja", URLLabel: "URL", ScreenshotLabel: "Zrzut", IMIntro: "Działanie przeglądarki wysokiego ryzyka wymaga potwierdzenia.", ReplyHint: "Odpowiedz `1`, aby kontynuować, lub `2`, aby anulować.", StepAct: "Interakcja", StepRecipe: "Przepis", ActionClick: "Kliknięcie", ActionType: "Wpisywanie", ActionSelect: "Wybór", ActionSubmit: "Wysłanie", RecipeLogin: "Logowanie", RecipeFillForm: "Wypełnianie formularza"},
	i18n.LangNlNL: {Header: "Bevestigen", Question: "Browseractie vereist bevestiging. Doorgaan?", Continue: "Doorgaan", Cancel: "Annuleren", StepLabel: "Stap", ActionLabel: "Actie", URLLabel: "URL", ScreenshotLabel: "Screenshot", IMIntro: "Browseractie met hoog risico vereist bevestiging.", ReplyHint: "Antwoord `1` om door te gaan of `2` om te annuleren.", StepAct: "Interactie", StepRecipe: "Recept", ActionClick: "Klikken", ActionType: "Typen", ActionSelect: "Selecteren", ActionSubmit: "Verzenden", RecipeLogin: "Inloggen", RecipeFillForm: "Formulier invullen"},
	i18n.LangSvSE: {Header: "Bekräfta", Question: "Webbläsaråtgärden kräver bekräftelse. Fortsätta?", Continue: "Fortsätt", Cancel: "Avbryt", StepLabel: "Steg", ActionLabel: "Åtgärd", URLLabel: "URL", ScreenshotLabel: "Skärmdump", IMIntro: "Webbläsaråtgärd med hög risk kräver bekräftelse.", ReplyHint: "Svara `1` för att fortsätta eller `2` för att avbryta.", StepAct: "Interaktion", StepRecipe: "Recept", ActionClick: "Klicka", ActionType: "Skriva", ActionSelect: "Välja", ActionSubmit: "Skicka", RecipeLogin: "Logga in", RecipeFillForm: "Fylla i formulär"},
	i18n.LangDaDK: {Header: "Bekræft", Question: "Browserhandling kræver bekræftelse. Fortsæt?", Continue: "Fortsæt", Cancel: "Annuller", StepLabel: "Trin", ActionLabel: "Handling", URLLabel: "URL", ScreenshotLabel: "Skærmbillede", IMIntro: "Browserhandling med høj risiko kræver bekræftelse.", ReplyHint: "Svar `1` for at fortsætte eller `2` for at annullere.", StepAct: "Interaktion", StepRecipe: "Opskrift", ActionClick: "Klik", ActionType: "Skriv", ActionSelect: "Vælg", ActionSubmit: "Send", RecipeLogin: "Log ind", RecipeFillForm: "Udfyld formular"},
	i18n.LangNbNO: {Header: "Bekreft", Question: "Nettleserhandling krever bekreftelse. Fortsette?", Continue: "Fortsett", Cancel: "Avbryt", StepLabel: "Trinn", ActionLabel: "Handling", URLLabel: "URL", ScreenshotLabel: "Skjermbilde", IMIntro: "Nettleserhandling med høy risiko krever bekreftelse.", ReplyHint: "Svar `1` for å fortsette eller `2` for å avbryte.", StepAct: "Interaksjon", StepRecipe: "Oppskrift", ActionClick: "Klikk", ActionType: "Skriv", ActionSelect: "Velg", ActionSubmit: "Send inn", RecipeLogin: "Logg inn", RecipeFillForm: "Fyll ut skjema"},
	i18n.LangCsCZ: {Header: "Potvrdit", Question: "Akce prohlížeče vyžaduje potvrzení. Pokračovat?", Continue: "Pokračovat", Cancel: "Zrušit", StepLabel: "Krok", ActionLabel: "Akce", URLLabel: "URL", ScreenshotLabel: "Snímek", IMIntro: "Vysoce riziková akce prohlížeče vyžaduje potvrzení.", ReplyHint: "Odpovězte `1` pro pokračování nebo `2` pro zrušení.", StepAct: "Interakce", StepRecipe: "Recept", ActionClick: "Kliknutí", ActionType: "Psaní", ActionSelect: "Výběr", ActionSubmit: "Odeslání", RecipeLogin: "Přihlášení", RecipeFillForm: "Vyplnění formuláře"},
	i18n.LangSkSK: {Header: "Potvrdiť", Question: "Akcia prehliadača vyžaduje potvrdenie. Pokračovať?", Continue: "Pokračovať", Cancel: "Zrušiť", StepLabel: "Krok", ActionLabel: "Akcia", URLLabel: "URL", ScreenshotLabel: "Snímka", IMIntro: "Vysokoriziková akcia prehliadača vyžaduje potvrdenie.", ReplyHint: "Odpovedzte `1` pre pokračovanie alebo `2` pre zrušenie.", StepAct: "Interakcia", StepRecipe: "Recept", ActionClick: "Kliknutie", ActionType: "Písanie", ActionSelect: "Výber", ActionSubmit: "Odoslanie", RecipeLogin: "Prihlásenie", RecipeFillForm: "Vyplnenie formulára"},
	i18n.LangHuHU: {Header: "Megerősítés", Question: "A böngészőművelet megerősítést igényel. Folytatja?", Continue: "Folytatás", Cancel: "Mégse", StepLabel: "Lépés", ActionLabel: "Művelet", URLLabel: "URL", ScreenshotLabel: "Képernyőkép", IMIntro: "A magas kockázatú böngészőművelet megerősítést igényel.", ReplyHint: "A folytatáshoz válaszoljon `1`-gyel, a megszakításhoz `2`-vel.", StepAct: "Interakció", StepRecipe: "Recept", ActionClick: "Kattintás", ActionType: "Beírás", ActionSelect: "Kiválasztás", ActionSubmit: "Beküldés", RecipeLogin: "Bejelentkezés", RecipeFillForm: "Űrlap kitöltése"},
	i18n.LangRoRO: {Header: "Confirmă", Question: "Acțiunea browserului necesită confirmare. Continui?", Continue: "Continuă", Cancel: "Anulează", StepLabel: "Pas", ActionLabel: "Acțiune", URLLabel: "URL", ScreenshotLabel: "Captură", IMIntro: "Acțiunea browserului cu risc ridicat necesită confirmare.", ReplyHint: "Răspunde cu `1` pentru a continua sau cu `2` pentru a anula.", StepAct: "Interacțiune", StepRecipe: "Rețetă", ActionClick: "Clic", ActionType: "Introducere", ActionSelect: "Selectare", ActionSubmit: "Trimitere", RecipeLogin: "Autentificare", RecipeFillForm: "Completare formular"},
	i18n.LangHrHR: {Header: "Potvrdi", Question: "Radnja preglednika zahtijeva potvrdu. Nastaviti?", Continue: "Nastavi", Cancel: "Odustani", StepLabel: "Korak", ActionLabel: "Radnja", URLLabel: "URL", ScreenshotLabel: "Snimka", IMIntro: "Radnja preglednika visokog rizika zahtijeva potvrdu.", ReplyHint: "Odgovori `1` za nastavak ili `2` za otkazivanje.", StepAct: "Interakcija", StepRecipe: "Recept", ActionClick: "Klik", ActionType: "Upis", ActionSelect: "Odabir", ActionSubmit: "Slanje", RecipeLogin: "Prijava", RecipeFillForm: "Ispunjavanje obrasca"},
	i18n.LangElGR: {Header: "Επιβεβαίωση", Question: "Η ενέργεια του προγράμματος περιήγησης απαιτεί επιβεβαίωση. Συνέχεια;", Continue: "Συνέχεια", Cancel: "Ακύρωση", StepLabel: "Βήμα", ActionLabel: "Ενέργεια", URLLabel: "URL", ScreenshotLabel: "Στιγμιότυπο", IMIntro: "Η ενέργεια υψηλού κινδύνου του προγράμματος περιήγησης απαιτεί επιβεβαίωση.", ReplyHint: "Απαντήστε `1` για συνέχεια ή `2` για ακύρωση.", StepAct: "Αλληλεπίδραση", StepRecipe: "Συνταγή", ActionClick: "Κλικ", ActionType: "Πληκτρολόγηση", ActionSelect: "Επιλογή", ActionSubmit: "Υποβολή", RecipeLogin: "Σύνδεση", RecipeFillForm: "Συμπλήρωση φόρμας"},
	i18n.LangCaES: {Header: "Confirma", Question: "L'acció del navegador necessita confirmació. Vols continuar?", Continue: "Continua", Cancel: "Cancel·la", StepLabel: "Pas", ActionLabel: "Acció", URLLabel: "URL", ScreenshotLabel: "Captura", IMIntro: "L'acció del navegador d'alt risc necessita confirmació.", ReplyHint: "Respon `1` per continuar o `2` per cancel·lar.", StepAct: "Interacció", StepRecipe: "Recepta", ActionClick: "Fer clic", ActionType: "Escriure", ActionSelect: "Seleccionar", ActionSubmit: "Enviar", RecipeLogin: "Inicia la sessió", RecipeFillForm: "Omple el formulari"},
	i18n.LangGaIE: {Header: "Deimhnigh", Question: "Tá deimhniú de dhíth ar ghníomh an bhrabhsálaí. Lean ar aghaidh?", Continue: "Lean ar aghaidh", Cancel: "Cealaigh", StepLabel: "Céim", ActionLabel: "Gníomh", URLLabel: "URL", ScreenshotLabel: "Gabháil", IMIntro: "Tá deimhniú de dhíth ar ghníomh ardriosca an bhrabhsálaí.", ReplyHint: "Freagair `1` chun leanúint ar aghaidh nó `2` chun cealú.", StepAct: "Idirghníomhú", StepRecipe: "Oideas", ActionClick: "Cliceáil", ActionType: "Clóscríobh", ActionSelect: "Roghnaigh", ActionSubmit: "Cuir isteach", RecipeLogin: "Logáil isteach", RecipeFillForm: "Líon foirm"},
	i18n.LangMlIN: {Header: "സ്ഥിരീകരിക്കുക", Question: "ബ്രൗസർ പ്രവർത്തനത്തിന് സ്ഥിരീകരണം ആവശ്യമാണ്. തുടരാമോ?", Continue: "തുടരുക", Cancel: "റദ്ദാക്കുക", StepLabel: "ഘട്ടം", ActionLabel: "പ്രവർത്തനം", URLLabel: "URL", ScreenshotLabel: "സ്ക്രീൻഷോട്ട്", IMIntro: "ഉയർന്ന അപകടസാധ്യതയുള്ള ബ്രൗസർ പ്രവർത്തനത്തിന് സ്ഥിരീകരണം ആവശ്യമാണ്.", ReplyHint: "തുടരാൻ `1` നും റദ്ദാക്കാൻ `2` നും മറുപടി നൽകുക.", StepAct: "ഇടപെടൽ", StepRecipe: "റെസിപ്പി", ActionClick: "ക്ലിക്ക്", ActionType: "ടൈപ്പ്", ActionSelect: "തിരഞ്ഞെടുക്കുക", ActionSubmit: "സമർപ്പിക്കുക", RecipeLogin: "ലോഗിൻ", RecipeFillForm: "ഫോം പൂരിപ്പിക്കൽ"},
}

func browserCheckpointLocale(lang i18n.Language) browserCheckpointLocaleStrings {
	if localized, ok := browserCheckpointStrings[i18n.ParseLanguage(string(lang))]; ok {
		return localized
	}
	return browserCheckpointStrings[i18n.DefaultLanguage]
}

func browserCheckpointAllowSiteLabel(lang i18n.Language) string {
	switch i18n.ParseLanguage(string(lang)) {
	case i18n.LangZhCN:
		return "始终允许此网站"
	case i18n.LangZhTW:
		return "永遠允許此網站"
	default:
		return "Always allow this site"
	}
}

func localizedBrowserCheckpointStep(lang i18n.Language, step string) string {
	localized := browserCheckpointLocale(lang)
	switch strings.ToLower(strings.TrimSpace(step)) {
	case "act":
		return localized.StepAct
	case "recipe":
		return localized.StepRecipe
	case "relay":
		if strings.TrimSpace(localized.StepRelay) != "" {
			return localized.StepRelay
		}
		return "Relay mode"
	default:
		return step
	}
}

func localizedBrowserCheckpointAction(lang i18n.Language, step, action string) string {
	localized := browserCheckpointLocale(lang)
	normalizedStep := strings.ToLower(strings.TrimSpace(step))
	normalizedAction := strings.ToLower(strings.TrimSpace(action))
	switch normalizedStep {
	case "act":
		switch normalizedAction {
		case "click":
			return localized.ActionClick
		case "type":
			return localized.ActionType
		case "select":
			return localized.ActionSelect
		case "submit":
			return localized.ActionSubmit
		}
	case "recipe":
		switch normalizedAction {
		case "login":
			return localized.RecipeLogin
		case "fill_form":
			return localized.RecipeFillForm
		}
	case "relay":
		switch normalizedAction {
		case "use_connected_session":
			if strings.TrimSpace(localized.ActionRelayUse) != "" {
				return localized.ActionRelayUse
			}
		case "list_connected_tabs":
			if strings.TrimSpace(localized.ActionRelayTabs) != "" {
				return localized.ActionRelayTabs
			}
		case "inspect_connected_session":
			if strings.TrimSpace(localized.ActionRelayView) != "" {
				return localized.ActionRelayView
			}
		}
	}
	return action
}

func formatBrowserCheckpointDetail(lang i18n.Language, step, action string) string {
	localized := browserCheckpointLocale(lang)
	parts := make([]string, 0, 2)
	if strings.TrimSpace(step) != "" {
		parts = append(parts, fmt.Sprintf("%s: %s", localized.StepLabel, step))
	}
	if strings.TrimSpace(action) != "" {
		parts = append(parts, fmt.Sprintf("%s: %s", localized.ActionLabel, action))
	}
	return strings.Join(parts, " | ")
}

func formatBrowserCheckpointConfirmMessage(lang i18n.Language, step, action, url, screenshotURL string) string {
	localized := browserCheckpointLocale(lang)
	lines := []string{localized.IMIntro}
	if strings.TrimSpace(step) != "" {
		lines = append(lines, fmt.Sprintf("%s: %s", localized.StepLabel, step))
	}
	if strings.TrimSpace(action) != "" {
		lines = append(lines, fmt.Sprintf("%s: %s", localized.ActionLabel, action))
	}
	if strings.TrimSpace(url) != "" {
		lines = append(lines, fmt.Sprintf("%s: %s", localized.URLLabel, url))
	}
	if strings.TrimSpace(screenshotURL) != "" {
		lines = append(lines, fmt.Sprintf("%s: %s", localized.ScreenshotLabel, screenshotURL))
	}
	lines = append(lines, localized.ReplyHint)
	return strings.Join(lines, "\n")
}
