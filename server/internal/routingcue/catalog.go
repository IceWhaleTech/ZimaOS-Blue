package routingcue

import (
	"strings"
	"sync"
)

// LocalizedSkillExample captures one curated route example plus the cue terms
// that should be recognized for that locale.
type LocalizedSkillExample struct {
	Locale  string
	Query   string
	Actions []string
	Objects []string
	Context []string
}

// SkillCue aggregates the multilingual trigger surface for one skill.
type SkillCue struct {
	Actions  []string
	Objects  []string
	Context  []string
	Examples []string
}

var (
	supportedLocales           []string
	localizedSkillExamples     map[string][]LocalizedSkillExample
	localizedURLBypassExamples map[string][]LocalizedSkillExample
	supportedLocalesOnce       sync.Once
	localizedSkillExamplesOnce sync.Once
	localizedURLBypassExamplesOnce sync.Once
)

func ensureSupportedLocales() {
	supportedLocalesOnce.Do(func() {
		supportedLocales = buildSupportedLocales()
	})
}

func ensureLocalizedSkillExamples() {
	localizedSkillExamplesOnce.Do(func() {
		localizedSkillExamples = buildLocalizedSkillExamples()
	})
}

func ensureLocalizedURLBypassExamples() {
	localizedURLBypassExamplesOnce.Do(func() {
		localizedURLBypassExamples = buildLocalizedURLBypassExamples()
	})
}

func buildSupportedLocales() []string {
	return []string{
		"ca-ES",
		"cs-CZ",
		"da-DK",
		"de-DE",
		"el-GR",
		"en-GB",
		"en-US",
		"es-ES",
		"fr-FR",
		"ga-IE",
		"hr-HR",
		"hu-HU",
		"it-IT",
		"ja-JP",
		"ko-KR",
		"ml-IN",
		"nb-NO",
		"nl-NL",
		"pl-PL",
		"pt-BR",
		"pt-PT",
		"ro-RO",
		"ru-RU",
		"sk-SK",
		"sv-SE",
		"zh-CN",
		"zh-TW",
	}
}

func buildLocalizedSkillExamples() map[string][]LocalizedSkillExample {
	return map[string][]LocalizedSkillExample{
		"web_query": {
			{Locale: "ca-ES", Query: "Busca la documentacio mes recent de l'API Responses d'OpenAI.", Actions: []string{"busca"}, Objects: []string{"documentacio"}, Context: []string{"mes recent"}},
			{Locale: "cs-CZ", Query: "Vyhledej nejnovejsi dokumentaci k OpenAI Responses API.", Actions: []string{"vyhledej"}, Objects: []string{"dokumentaci"}, Context: []string{"nejnovejsi"}},
			{Locale: "da-DK", Query: "Find den nyeste dokumentation til OpenAI Responses API.", Actions: []string{"find"}, Objects: []string{"dokumentation"}, Context: []string{"nyeste"}},
			{Locale: "de-DE", Query: "Suche die neueste Dokumentation zur OpenAI Responses API.", Actions: []string{"suche"}, Objects: []string{"dokumentation"}, Context: []string{"neueste"}},
			{Locale: "el-GR", Query: "Αναζήτησε την πιο πρόσφατη τεκμηρίωση για το OpenAI Responses API.", Actions: []string{"αναζήτησε"}, Objects: []string{"τεκμηρίωση"}, Context: []string{"πιο πρόσφατη"}},
			{Locale: "en-GB", Query: "Search the latest OpenAI Responses API documentation.", Actions: []string{"search"}, Objects: []string{"documentation"}, Context: []string{"latest"}},
			{Locale: "en-US", Query: "Search the latest OpenAI Responses API documentation.", Actions: []string{"search"}, Objects: []string{"documentation"}, Context: []string{"latest"}},
			{Locale: "es-ES", Query: "Busca la documentacion mas reciente de OpenAI Responses API.", Actions: []string{"busca"}, Objects: []string{"documentacion"}, Context: []string{"mas reciente"}},
			{Locale: "fr-FR", Query: "Recherche la documentation la plus recente de l'API Responses d'OpenAI.", Actions: []string{"recherche"}, Objects: []string{"documentation"}, Context: []string{"plus recente"}},
			{Locale: "ga-IE", Query: "Cuardaigh an chaipéisíocht is déanaí don OpenAI Responses API.", Actions: []string{"cuardaigh"}, Objects: []string{"chaipéisíocht"}, Context: []string{"is déanaí"}},
			{Locale: "hr-HR", Query: "Pretrazi najnoviju dokumentaciju za OpenAI Responses API.", Actions: []string{"pretrazi"}, Objects: []string{"dokumentaciju"}, Context: []string{"najnoviju"}},
			{Locale: "hu-HU", Query: "Keresd meg az OpenAI Responses API legfrissebb dokumentaciojat.", Actions: []string{"keresd"}, Objects: []string{"dokumentaciojat"}, Context: []string{"legfrissebb"}},
			{Locale: "it-IT", Query: "Cerca la documentazione piu recente di OpenAI Responses API.", Actions: []string{"cerca"}, Objects: []string{"documentazione"}, Context: []string{"piu recente"}},
			{Locale: "ja-JP", Query: "OpenAI Responses API の最新ドキュメントを検索して。", Actions: []string{"検索して"}, Objects: []string{"ドキュメント"}, Context: []string{"最新"}},
			{Locale: "ko-KR", Query: "OpenAI Responses API 최신 문서를 검색해 줘.", Actions: []string{"검색해"}, Objects: []string{"문서"}, Context: []string{"최신"}},
			{Locale: "ml-IN", Query: "OpenAI Responses APIയുടെ ഏറ്റവും പുതിയ ഡോക്യുമെന്റേഷൻ തിരയൂ.", Actions: []string{"തിരയൂ"}, Objects: []string{"ഡോക്യുമെന്റേഷൻ"}, Context: []string{"ഏറ്റവും പുതിയ"}},
			{Locale: "nb-NO", Query: "Sok etter den nyeste dokumentasjonen for OpenAI Responses API.", Actions: []string{"sok"}, Objects: []string{"dokumentasjonen"}, Context: []string{"nyeste"}},
			{Locale: "nl-NL", Query: "Zoek de nieuwste documentatie voor OpenAI Responses API.", Actions: []string{"zoek"}, Objects: []string{"documentatie"}, Context: []string{"nieuwste"}},
			{Locale: "pl-PL", Query: "Wyszukaj najnowsza dokumentacje OpenAI Responses API.", Actions: []string{"wyszukaj"}, Objects: []string{"dokumentacje"}, Context: []string{"najnowsza"}},
			{Locale: "pt-BR", Query: "Busque a documentacao mais recente da OpenAI Responses API.", Actions: []string{"busque"}, Objects: []string{"documentacao"}, Context: []string{"mais recente"}},
			{Locale: "pt-PT", Query: "Procura a documentacao mais recente da OpenAI Responses API.", Actions: []string{"procura"}, Objects: []string{"documentacao"}, Context: []string{"mais recente"}},
			{Locale: "ro-RO", Query: "Cauta cea mai recenta documentatie pentru OpenAI Responses API.", Actions: []string{"cauta"}, Objects: []string{"documentatie"}, Context: []string{"cea mai recenta"}},
			{Locale: "ru-RU", Query: "Найди самую свежую документацию по OpenAI Responses API.", Actions: []string{"найди"}, Objects: []string{"документацию"}, Context: []string{"самую свежую"}},
			{Locale: "sk-SK", Query: "Vyhladaj najnovsiu dokumentaciu k OpenAI Responses API.", Actions: []string{"vyhladaj"}, Objects: []string{"dokumentaciu"}, Context: []string{"najnovsiu"}},
			{Locale: "sv-SE", Query: "Sok den senaste dokumentationen for OpenAI Responses API.", Actions: []string{"sok"}, Objects: []string{"dokumentationen"}, Context: []string{"senaste"}},
			{Locale: "zh-CN", Query: "搜索 OpenAI Responses API 的最新文档。", Actions: []string{"搜索"}, Objects: []string{"文档"}, Context: []string{"最新"}},
			{Locale: "zh-TW", Query: "搜尋 OpenAI Responses API 的最新文件。", Actions: []string{"搜尋"}, Objects: []string{"文件"}, Context: []string{"最新"}},
		},
		"workspace_local": {
			{Locale: "ca-ES", Query: "Resumeix el README de l'espai de treball i digues que fa el projecte.", Actions: []string{"resumeix"}, Objects: []string{"readme"}, Context: []string{"espai de treball"}},
			{Locale: "cs-CZ", Query: "Shrn README ve workspace a rekni, co projekt dela.", Actions: []string{"shrn"}, Objects: []string{"readme"}, Context: []string{"workspace"}},
			{Locale: "da-DK", Query: "Opsummer README i workspace og fortael, hvad projektet laver.", Actions: []string{"opsummer"}, Objects: []string{"readme"}, Context: []string{"workspace"}},
			{Locale: "de-DE", Query: "Fasse das README im Workspace zusammen und sag mir, was das Projekt macht.", Actions: []string{"fasse"}, Objects: []string{"readme"}, Context: []string{"workspace"}},
			{Locale: "el-GR", Query: "Σύνοψε το README στο workspace και πες μου τι κάνει το έργο.", Actions: []string{"σύνοψε"}, Objects: []string{"readme"}, Context: []string{"workspace"}},
			{Locale: "en-GB", Query: "Summarise the README in the workspace and tell me what the project does.", Actions: []string{"summarise"}, Objects: []string{"readme"}, Context: []string{"workspace"}},
			{Locale: "en-US", Query: "Summarize the README in the workspace and tell me what the project does.", Actions: []string{"summarize"}, Objects: []string{"readme"}, Context: []string{"workspace"}},
			{Locale: "es-ES", Query: "Resume el README del espacio de trabajo y dime que hace el proyecto.", Actions: []string{"resume"}, Objects: []string{"readme"}, Context: []string{"espacio de trabajo"}},
			{Locale: "fr-FR", Query: "Resume le README de l'espace de travail et dis-moi ce que fait le projet.", Actions: []string{"resume"}, Objects: []string{"readme"}, Context: []string{"espace de travail"}},
			{Locale: "ga-IE", Query: "Dean achoimre ar an README sa spás oibre agus inis dom cad a dhéanann an tionscadal.", Actions: []string{"achoimre"}, Objects: []string{"readme"}, Context: []string{"spás oibre"}},
			{Locale: "hr-HR", Query: "Sazmi README u workspaceu i reci mi sto projekt radi.", Actions: []string{"sazmi"}, Objects: []string{"readme"}, Context: []string{"workspaceu"}},
			{Locale: "hu-HU", Query: "Foglald ossze a README-t a workspaceben, es mondd el, mit csinal a projekt.", Actions: []string{"foglald ossze"}, Objects: []string{"readme"}, Context: []string{"workspaceben"}},
			{Locale: "it-IT", Query: "Riassumi il README nel workspace e dimmi cosa fa il progetto.", Actions: []string{"riassumi"}, Objects: []string{"readme"}, Context: []string{"workspace"}},
			{Locale: "ja-JP", Query: "workspace の README を要約して、プロジェクトが何をしているか教えて。", Actions: []string{"要約して"}, Objects: []string{"readme"}, Context: []string{"workspace"}},
			{Locale: "ko-KR", Query: "workspace 의 README 를 요약하고 프로젝트가 무엇을 하는지 알려 줘.", Actions: []string{"요약하고"}, Objects: []string{"readme"}, Context: []string{"workspace"}},
			{Locale: "ml-IN", Query: "workspace ലെയുള്ള README സംഗ്രഹിച്ച് പ്രോജക്ട് എന്താണ് ചെയ്യുന്നതെന്ന് പറയൂ.", Actions: []string{"സംഗ്രഹിച്ച്"}, Objects: []string{"readme"}, Context: []string{"workspace"}},
			{Locale: "nb-NO", Query: "Oppsummer README i workspace og fortell hva prosjektet gjor.", Actions: []string{"oppsummer"}, Objects: []string{"readme"}, Context: []string{"workspace"}},
			{Locale: "nl-NL", Query: "Vat de README in de workspace samen en vertel wat het project doet.", Actions: []string{"vat samen"}, Objects: []string{"readme"}, Context: []string{"workspace"}},
			{Locale: "pl-PL", Query: "Podsumuj README w workspace i powiedz, co robi projekt.", Actions: []string{"podsumuj"}, Objects: []string{"readme"}, Context: []string{"workspace"}},
			{Locale: "pt-BR", Query: "Resuma o README no workspace e diga o que o projeto faz.", Actions: []string{"resuma"}, Objects: []string{"readme"}, Context: []string{"workspace"}},
			{Locale: "pt-PT", Query: "Resume o README no workspace e diz-me o que o projeto faz.", Actions: []string{"resume"}, Objects: []string{"readme"}, Context: []string{"workspace"}},
			{Locale: "ro-RO", Query: "Rezuma README-ul din workspace si spune-mi ce face proiectul.", Actions: []string{"rezuma"}, Objects: []string{"readme"}, Context: []string{"workspace"}},
			{Locale: "ru-RU", Query: "Суммируй README в workspace и скажи, что делает проект.", Actions: []string{"суммируй"}, Objects: []string{"readme"}, Context: []string{"workspace"}},
			{Locale: "sk-SK", Query: "Zhrn README vo workspace a povedz mi, co projekt robi.", Actions: []string{"zhrn"}, Objects: []string{"readme"}, Context: []string{"workspace"}},
			{Locale: "sv-SE", Query: "Sammanfatta README i workspace och beratta vad projektet gor.", Actions: []string{"sammanfatta"}, Objects: []string{"readme"}, Context: []string{"workspace"}},
			{Locale: "zh-CN", Query: "总结 workspace 里的 README，并告诉我项目在做什么。", Actions: []string{"总结"}, Objects: []string{"readme"}, Context: []string{"workspace"}},
			{Locale: "zh-TW", Query: "總結 workspace 裡的 README，並告訴我專案在做什麼。", Actions: []string{"總結"}, Objects: []string{"readme"}, Context: []string{"workspace"}},
		},
		"reminder": {
			{Locale: "ca-ES", Query: "Programa un recordatori per dema a les 9 per enviar l'informe setmanal.", Actions: []string{"programa"}, Objects: []string{"recordatori"}, Context: []string{"dema"}},
			{Locale: "cs-CZ", Query: "Naplanuj pripominku na zitra v 9 pro odeslani tydenniho reportu.", Actions: []string{"naplanuj"}, Objects: []string{"pripominku"}, Context: []string{"zitra"}},
			{Locale: "da-DK", Query: "Planlaeg en pamindelse til i morgen klokken 9 om at sende ugerapporten.", Actions: []string{"planlaeg"}, Objects: []string{"pamindelse"}, Context: []string{"i morgen"}},
			{Locale: "de-DE", Query: "Plane eine Erinnerung fur morgen um 9 Uhr, damit ich den Wochenbericht sende.", Actions: []string{"plane"}, Objects: []string{"erinnerung"}, Context: []string{"morgen"}},
			{Locale: "el-GR", Query: "Προγραμμάτισε μια υπενθύμιση για αύριο στις 9 να στείλω την εβδομαδιαία αναφορά.", Actions: []string{"προγραμμάτισε"}, Objects: []string{"υπενθύμιση"}, Context: []string{"αύριο"}},
			{Locale: "en-GB", Query: "Schedule a reminder for tomorrow at 9 AM to send the weekly report.", Actions: []string{"schedule"}, Objects: []string{"reminder"}, Context: []string{"tomorrow"}},
			{Locale: "en-US", Query: "Schedule a reminder for tomorrow at 9 AM to send the weekly report.", Actions: []string{"schedule"}, Objects: []string{"reminder"}, Context: []string{"tomorrow"}},
			{Locale: "es-ES", Query: "Programa un recordatorio para manana a las 9 para enviar el informe semanal.", Actions: []string{"programa"}, Objects: []string{"recordatorio"}, Context: []string{"manana"}},
			{Locale: "fr-FR", Query: "Programme un rappel pour demain a 9 h afin d'envoyer le rapport hebdomadaire.", Actions: []string{"programme"}, Objects: []string{"rappel"}, Context: []string{"demain"}},
			{Locale: "ga-IE", Query: "Socraigh meabhruchan do amárach ag 9 chun an tuairisc sheachtainiúil a sheoladh.", Actions: []string{"socraigh"}, Objects: []string{"meabhruchan"}, Context: []string{"amárach"}},
			{Locale: "hr-HR", Query: "Zakazi podsjetnik za sutra u 9 da posaljem tjedni izvjestaj.", Actions: []string{"zakazi"}, Objects: []string{"podsjetnik"}, Context: []string{"sutra"}},
			{Locale: "hu-HU", Query: "Utemezz emlekeztetot holnap 9 orara a heti jelentes elkuldesehez.", Actions: []string{"utemezz"}, Objects: []string{"emlekeztetot"}, Context: []string{"holnap"}},
			{Locale: "it-IT", Query: "Programma un promemoria per domani alle 9 per inviare il rapporto settimanale.", Actions: []string{"programma"}, Objects: []string{"promemoria"}, Context: []string{"domani"}},
			{Locale: "ja-JP", Query: "明日朝9時に週報を送るリマインダーを設定して。", Actions: []string{"設定して"}, Objects: []string{"リマインダー"}, Context: []string{"明日朝9時"}},
			{Locale: "ko-KR", Query: "내일 아침 9시에 주간 보고서를 보내도록 알림을 예약해 줘.", Actions: []string{"예약해"}, Objects: []string{"알림"}, Context: []string{"내일 아침 9시"}},
			{Locale: "ml-IN", Query: "നാളെ രാവിലെ 9 മണിക്ക് ആഴ്ച റിപ്പോർട്ട് അയക്കാൻ ഒരു റിമൈൻഡർ സജ്ജമാക്കൂ.", Actions: []string{"സജ്ജമാക്കൂ"}, Objects: []string{"റിമൈൻഡർ"}, Context: []string{"നാളെ രാവിലെ 9 മണിക്ക്"}},
			{Locale: "nb-NO", Query: "Planlegg en paminnelse for i morgen klokken 9 om a sende ukesrapporten.", Actions: []string{"planlegg"}, Objects: []string{"paminnelse"}, Context: []string{"i morgen"}},
			{Locale: "nl-NL", Query: "Plan een herinnering voor morgen om 9 uur om het weekrapport te sturen.", Actions: []string{"plan"}, Objects: []string{"herinnering"}, Context: []string{"morgen"}},
			{Locale: "pl-PL", Query: "Zaplanuj przypomnienie na jutro na 9:00, zebym wyslal tygodniowy raport.", Actions: []string{"zaplanuj"}, Objects: []string{"przypomnienie"}, Context: []string{"jutro"}},
			{Locale: "pt-BR", Query: "Agende um lembrete para amanha as 9 para enviar o relatorio semanal.", Actions: []string{"agende"}, Objects: []string{"lembrete"}, Context: []string{"amanha"}},
			{Locale: "pt-PT", Query: "Agenda um lembrete para amanha as 9 para enviar o relatorio semanal.", Actions: []string{"agenda"}, Objects: []string{"lembrete"}, Context: []string{"amanha"}},
			{Locale: "ro-RO", Query: "Programeaza un memento pentru maine la 9 ca sa trimit raportul saptamanal.", Actions: []string{"programeaza"}, Objects: []string{"memento"}, Context: []string{"maine"}},
			{Locale: "ru-RU", Query: "Запланируй напоминание на завтра на 9:00, чтобы отправить недельный отчет.", Actions: []string{"запланируй"}, Objects: []string{"напоминание"}, Context: []string{"завтра"}},
			{Locale: "sk-SK", Query: "Naplanuj pripomienku na zajtra o 9:00, aby som poslal tyzdenny report.", Actions: []string{"naplanuj"}, Objects: []string{"pripomienku"}, Context: []string{"zajtra"}},
			{Locale: "sv-SE", Query: "Planera en paminnelse till i morgon klockan 9 for att skicka veckorapporten.", Actions: []string{"planera"}, Objects: []string{"paminnelse"}, Context: []string{"i morgon"}},
			{Locale: "zh-CN", Query: "帮我设置一个明早 9 点发送周报的提醒。", Actions: []string{"设置"}, Objects: []string{"提醒"}, Context: []string{"明早 9 点"}},
			{Locale: "zh-TW", Query: "幫我設定一個明早 9 點發送週報的提醒。", Actions: []string{"設定"}, Objects: []string{"提醒"}, Context: []string{"明早 9 點"}},
		},
		"browser": {
			{Locale: "ca-ES", Query: "Obre el lloc web https://example.com/pricing al navegador.", Actions: []string{"obre"}, Objects: []string{"lloc web"}, Context: []string{"https://example.com/pricing"}},
			{Locale: "cs-CZ", Query: "Otevri web https://example.com/pricing v prohlizeci.", Actions: []string{"otevri"}, Objects: []string{"web"}, Context: []string{"https://example.com/pricing"}},
			{Locale: "da-DK", Query: "Abn websitet https://example.com/pricing i browseren.", Actions: []string{"abn"}, Objects: []string{"websitet"}, Context: []string{"https://example.com/pricing"}},
			{Locale: "de-DE", Query: "Offne die Website https://example.com/pricing im Browser.", Actions: []string{"offne"}, Objects: []string{"website"}, Context: []string{"https://example.com/pricing"}},
			{Locale: "el-GR", Query: "Άνοιξε τον ιστότοπο https://example.com/pricing στο πρόγραμμα περιήγησης.", Actions: []string{"άνοιξε"}, Objects: []string{"ιστότοπο"}, Context: []string{"https://example.com/pricing"}},
			{Locale: "en-GB", Query: "Open the website https://example.com/pricing in the browser.", Actions: []string{"open"}, Objects: []string{"website"}, Context: []string{"https://example.com/pricing"}},
			{Locale: "en-US", Query: "Open the website https://example.com/pricing in the browser.", Actions: []string{"open"}, Objects: []string{"website"}, Context: []string{"https://example.com/pricing"}},
			{Locale: "es-ES", Query: "Abre el sitio web https://example.com/pricing en el navegador.", Actions: []string{"abre"}, Objects: []string{"sitio web"}, Context: []string{"https://example.com/pricing"}},
			{Locale: "fr-FR", Query: "Ouvre le site web https://example.com/pricing dans le navigateur.", Actions: []string{"ouvre"}, Objects: []string{"site web"}, Context: []string{"https://example.com/pricing"}},
			{Locale: "ga-IE", Query: "Oscail an suíomh gréasáin https://example.com/pricing sa bhrabhsálaí.", Actions: []string{"oscail"}, Objects: []string{"suíomh gréasáin"}, Context: []string{"https://example.com/pricing"}},
			{Locale: "hr-HR", Query: "Otvori web stranicu https://example.com/pricing u pregledniku.", Actions: []string{"otvori"}, Objects: []string{"web stranicu"}, Context: []string{"https://example.com/pricing"}},
			{Locale: "hu-HU", Query: "Nyisd meg a https://example.com/pricing weboldalt a bongeszoben.", Actions: []string{"nyisd meg"}, Objects: []string{"weboldalt"}, Context: []string{"https://example.com/pricing"}},
			{Locale: "it-IT", Query: "Apri il sito web https://example.com/pricing nel browser.", Actions: []string{"apri"}, Objects: []string{"sito web"}, Context: []string{"https://example.com/pricing"}},
			{Locale: "ja-JP", Query: "ブラウザで https://example.com/pricing のウェブサイトを開いて。", Actions: []string{"開いて"}, Objects: []string{"ウェブサイト"}, Context: []string{"https://example.com/pricing"}},
			{Locale: "ko-KR", Query: "브라우저에서 https://example.com/pricing 웹사이트를 열어 줘.", Actions: []string{"열어"}, Objects: []string{"웹사이트"}, Context: []string{"https://example.com/pricing"}},
			{Locale: "ml-IN", Query: "https://example.com/pricing എന്ന വെബ്സൈറ്റ് ബ്രൗസറിൽ തുറക്കൂ.", Actions: []string{"തുറക്കൂ"}, Objects: []string{"വെബ്സൈറ്റ്"}, Context: []string{"https://example.com/pricing"}},
			{Locale: "nb-NO", Query: "Apne nettstedet https://example.com/pricing i nettleseren.", Actions: []string{"apne"}, Objects: []string{"nettstedet"}, Context: []string{"https://example.com/pricing"}},
			{Locale: "nl-NL", Query: "Open de website https://example.com/pricing in de browser.", Actions: []string{"open"}, Objects: []string{"website"}, Context: []string{"https://example.com/pricing"}},
			{Locale: "pl-PL", Query: "Otworz strone https://example.com/pricing w przegladarce.", Actions: []string{"otworz"}, Objects: []string{"strone"}, Context: []string{"https://example.com/pricing"}},
			{Locale: "pt-BR", Query: "Abra o site https://example.com/pricing no navegador.", Actions: []string{"abra"}, Objects: []string{"site"}, Context: []string{"https://example.com/pricing"}},
			{Locale: "pt-PT", Query: "Abre o site https://example.com/pricing no navegador.", Actions: []string{"abre"}, Objects: []string{"site"}, Context: []string{"https://example.com/pricing"}},
			{Locale: "ro-RO", Query: "Deschide site-ul https://example.com/pricing in browser.", Actions: []string{"deschide"}, Objects: []string{"site-ul"}, Context: []string{"https://example.com/pricing"}},
			{Locale: "ru-RU", Query: "Открой сайт https://example.com/pricing в браузере.", Actions: []string{"открой"}, Objects: []string{"сайт"}, Context: []string{"https://example.com/pricing"}},
			{Locale: "sk-SK", Query: "Otvor web https://example.com/pricing v prehliadaci.", Actions: []string{"otvor"}, Objects: []string{"web"}, Context: []string{"https://example.com/pricing"}},
			{Locale: "sv-SE", Query: "Oppna webbplatsen https://example.com/pricing i webblasaren.", Actions: []string{"oppna"}, Objects: []string{"webbplatsen"}, Context: []string{"https://example.com/pricing"}},
			{Locale: "zh-CN", Query: "在浏览器里打开 https://example.com/pricing 这个网站。", Actions: []string{"打开"}, Objects: []string{"网站"}, Context: []string{"https://example.com/pricing"}},
			{Locale: "zh-TW", Query: "在瀏覽器裡打開 https://example.com/pricing 這個網站。", Actions: []string{"打開"}, Objects: []string{"網站"}, Context: []string{"https://example.com/pricing"}},
		},
		"ui_reviewer": {
			{Locale: "ca-ES", Query: "Revisa aquesta captura de pantalla i audita la interficie i l'accessibilitat.", Actions: []string{"revisa"}, Objects: []string{"captura de pantalla"}, Context: []string{"accessibilitat"}},
			{Locale: "cs-CZ", Query: "Zkontroluj tento snimek obrazovky a proved audit UI a pristupnosti.", Actions: []string{"zkontroluj"}, Objects: []string{"snimek obrazovky"}, Context: []string{"pristupnosti"}},
			{Locale: "da-DK", Query: "Gennemga dette skaermbillede og auditer UI og tilgaengelighed.", Actions: []string{"gennemga"}, Objects: []string{"skaermbillede"}, Context: []string{"tilgaengelighed"}},
			{Locale: "de-DE", Query: "Prufe diesen Screenshot und auditiere UI und Barrierefreiheit.", Actions: []string{"prufe"}, Objects: []string{"screenshot"}, Context: []string{"barrierefreiheit"}},
			{Locale: "el-GR", Query: "Έλεγξε αυτό το στιγμιότυπο οθόνης και κάνε audit στο UI και την προσβασιμότητα.", Actions: []string{"έλεγξε"}, Objects: []string{"στιγμιότυπο οθόνης"}, Context: []string{"προσβασιμότητα"}},
			{Locale: "en-GB", Query: "Review this screenshot and audit the UI and accessibility.", Actions: []string{"review"}, Objects: []string{"screenshot"}, Context: []string{"accessibility"}},
			{Locale: "en-US", Query: "Review this screenshot and audit the UI and accessibility.", Actions: []string{"review"}, Objects: []string{"screenshot"}, Context: []string{"accessibility"}},
			{Locale: "es-ES", Query: "Revisa esta captura de pantalla y audita la interfaz y la accesibilidad.", Actions: []string{"revisa"}, Objects: []string{"captura de pantalla"}, Context: []string{"accesibilidad"}},
			{Locale: "fr-FR", Query: "Examine cette capture d'ecran et audite l'interface et l'accessibilite.", Actions: []string{"examine"}, Objects: []string{"capture d'ecran"}, Context: []string{"accessibilite"}},
			{Locale: "ga-IE", Query: "Dean athbhreithniu ar an seat scaileain seo agus dean audit ar an UI agus ar an inrochtaineacht.", Actions: []string{"athbhreithniu"}, Objects: []string{"seat scaileain"}, Context: []string{"inrochtaineacht"}},
			{Locale: "hr-HR", Query: "Pregledaj ovu snimku zaslona i napravi audit UI-ja i pristupacnosti.", Actions: []string{"pregledaj"}, Objects: []string{"snimku zaslona"}, Context: []string{"pristupacnosti"}},
			{Locale: "hu-HU", Query: "Nezd at ezt a kepernyokepet, es auditald a UI-t es az akadalymentesseget.", Actions: []string{"nezd at"}, Objects: []string{"kepernyokepet"}, Context: []string{"akadalymentesseget"}},
			{Locale: "it-IT", Query: "Rivedi questo screenshot e fai un audit della UI e dell'accessibilita.", Actions: []string{"rivedi"}, Objects: []string{"screenshot"}, Context: []string{"accessibilita"}},
			{Locale: "ja-JP", Query: "このスクリーンショットをレビューして、UI とアクセシビリティを監査して。", Actions: []string{"レビューして"}, Objects: []string{"スクリーンショット"}, Context: []string{"アクセシビリティ"}},
			{Locale: "ko-KR", Query: "이 스크린샷을 검토하고 UI 와 접근성을 감사해 줘.", Actions: []string{"검토하고"}, Objects: []string{"스크린샷"}, Context: []string{"접근성"}},
			{Locale: "ml-IN", Query: "ഈ സ്ക്രീൻഷോട്ട് പരിശോധിച്ച് UIയും ആക്സസിബിലിറ്റിയും ഓഡിറ്റ് ചെയ്യൂ.", Actions: []string{"പരിശോധിച്ച്"}, Objects: []string{"സ്ക്രീൻഷോട്ട്"}, Context: []string{"ആക്സസിബിലിറ്റി"}},
			{Locale: "nb-NO", Query: "Ga gjennom dette skjermbildet og gjor en audit av UI og tilgjengelighet.", Actions: []string{"ga gjennom"}, Objects: []string{"skjermbildet"}, Context: []string{"tilgjengelighet"}},
			{Locale: "nl-NL", Query: "Beoordeel deze screenshot en audit de UI en toegankelijkheid.", Actions: []string{"beoordeel"}, Objects: []string{"screenshot"}, Context: []string{"toegankelijkheid"}},
			{Locale: "pl-PL", Query: "Przejrzyj ten zrzut ekranu i zrob audyt UI oraz dostepnosci.", Actions: []string{"przejrzyj"}, Objects: []string{"zrzut ekranu"}, Context: []string{"dostepnosci"}},
			{Locale: "pt-BR", Query: "Revise esta captura de tela e audite a UI e a acessibilidade.", Actions: []string{"revise"}, Objects: []string{"captura de tela"}, Context: []string{"acessibilidade"}},
			{Locale: "pt-PT", Query: "Reve esta captura de ecra e audita a UI e a acessibilidade.", Actions: []string{"reve"}, Objects: []string{"captura de ecra"}, Context: []string{"acessibilidade"}},
			{Locale: "ro-RO", Query: "Revizuieste aceasta captura de ecran si auditeaza UI-ul si accesibilitatea.", Actions: []string{"revizuieste"}, Objects: []string{"captura de ecran"}, Context: []string{"accesibilitatea"}},
			{Locale: "ru-RU", Query: "Проверь этот скриншот и сделай аудит UI и доступности.", Actions: []string{"проверь"}, Objects: []string{"скриншот"}, Context: []string{"доступности"}},
			{Locale: "sk-SK", Query: "Skontroluj tento screenshot a urob audit UI a pristupnosti.", Actions: []string{"skontroluj"}, Objects: []string{"screenshot"}, Context: []string{"pristupnosti"}},
			{Locale: "sv-SE", Query: "Granska den har skarmbilden och gor en audit av UI och tillganglighet.", Actions: []string{"granska"}, Objects: []string{"skarmbilden"}, Context: []string{"tillganglighet"}},
			{Locale: "zh-CN", Query: "检查这张截图，并审查界面和无障碍。", Actions: []string{"检查"}, Objects: []string{"截图"}, Context: []string{"无障碍"}},
			{Locale: "zh-TW", Query: "檢查這張截圖，並審查介面和無障礙。", Actions: []string{"檢查"}, Objects: []string{"截圖"}, Context: []string{"無障礙"}},
		},
	}
}

func buildLocalizedURLBypassExamples() map[string][]LocalizedSkillExample {
	return map[string][]LocalizedSkillExample{
		"analyze": {
			{Locale: "ca-ES", Query: "Resumeix https://example.com/blog i extreu-ne els punts clau.", Actions: []string{"resumeix"}, Objects: []string{"blog"}, Context: []string{"punts clau"}},
			{Locale: "cs-CZ", Query: "Shrn https://example.com/blog a vypichni hlavni body.", Actions: []string{"shrn"}, Objects: []string{"blog"}, Context: []string{"hlavni body"}},
			{Locale: "da-DK", Query: "Opsummer https://example.com/blog og fremhaev hovedpunkterne.", Actions: []string{"opsummer"}, Objects: []string{"blog"}, Context: []string{"hovedpunkterne"}},
			{Locale: "de-DE", Query: "Fasse https://example.com/blog zusammen und nenne die wichtigsten Punkte.", Actions: []string{"fasse"}, Objects: []string{"blog"}, Context: []string{"wichtigsten Punkte"}},
			{Locale: "el-GR", Query: "Σύνοψε το https://example.com/blog και βγάλε τα βασικά σημεία.", Actions: []string{"σύνοψε"}, Objects: []string{"blog"}, Context: []string{"βασικά σημεία"}},
			{Locale: "en-GB", Query: "Summarise https://example.com/blog and extract the key points.", Actions: []string{"summarise"}, Objects: []string{"blog"}, Context: []string{"key points"}},
			{Locale: "en-US", Query: "Summarize https://example.com/blog and extract the key points.", Actions: []string{"summarize"}, Objects: []string{"blog"}, Context: []string{"key points"}},
			{Locale: "es-ES", Query: "Resume https://example.com/blog y extrae los puntos clave.", Actions: []string{"resume"}, Objects: []string{"blog"}, Context: []string{"puntos clave"}},
			{Locale: "fr-FR", Query: "Resume https://example.com/blog et extrais les points cles.", Actions: []string{"resume"}, Objects: []string{"blog"}, Context: []string{"points cles"}},
			{Locale: "ga-IE", Query: "Dean achoimre ar https://example.com/blog agus bain amach na priomhphointi.", Actions: []string{"achoimre"}, Objects: []string{"blog"}, Context: []string{"priomhphointi"}},
			{Locale: "hr-HR", Query: "Sazmi https://example.com/blog i izdvoji glavne tocke.", Actions: []string{"sazmi"}, Objects: []string{"blog"}, Context: []string{"glavne tocke"}},
			{Locale: "hu-HU", Query: "Foglald ossze a https://example.com/blog oldalt, es emeld ki a fo pontokat.", Actions: []string{"foglald ossze"}, Objects: []string{"blog"}, Context: []string{"fo pontokat"}},
			{Locale: "it-IT", Query: "Riassumi https://example.com/blog ed estrai i punti chiave.", Actions: []string{"riassumi"}, Objects: []string{"blog"}, Context: []string{"punti chiave"}},
			{Locale: "ja-JP", Query: "https://example.com/blog を要約して主要なポイントを抽出して。", Actions: []string{"要約して"}, Objects: []string{"blog"}, Context: []string{"主要なポイント"}},
			{Locale: "ko-KR", Query: "https://example.com/blog 을 요약하고 핵심 포인트를 추려 줘.", Actions: []string{"요약하고"}, Objects: []string{"blog"}, Context: []string{"핵심 포인트"}},
			{Locale: "ml-IN", Query: "https://example.com/blog സംഗ്രഹിച്ച് പ്രധാന പോയിന്റുകൾ എടുത്തുകാട്ടൂ.", Actions: []string{"സംഗ്രഹിച്ച്"}, Objects: []string{"blog"}, Context: []string{"പ്രധാന പോയിന്റുകൾ"}},
			{Locale: "nb-NO", Query: "Oppsummer https://example.com/blog og trekk ut hovedpunktene.", Actions: []string{"oppsummer"}, Objects: []string{"blog"}, Context: []string{"hovedpunktene"}},
			{Locale: "nl-NL", Query: "Vat https://example.com/blog samen en haal de belangrijkste punten eruit.", Actions: []string{"vat samen"}, Objects: []string{"blog"}, Context: []string{"belangrijkste punten"}},
			{Locale: "pl-PL", Query: "Podsumuj https://example.com/blog i wyciagnij najwazniejsze punkty.", Actions: []string{"podsumuj"}, Objects: []string{"blog"}, Context: []string{"najwazniejsze punkty"}},
			{Locale: "pt-BR", Query: "Resuma https://example.com/blog e extraia os pontos principais.", Actions: []string{"resuma"}, Objects: []string{"blog"}, Context: []string{"pontos principais"}},
			{Locale: "pt-PT", Query: "Resume https://example.com/blog e extrai os pontos principais.", Actions: []string{"resume"}, Objects: []string{"blog"}, Context: []string{"pontos principais"}},
			{Locale: "ro-RO", Query: "Rezuma https://example.com/blog si extrage punctele cheie.", Actions: []string{"rezuma"}, Objects: []string{"blog"}, Context: []string{"punctele cheie"}},
			{Locale: "ru-RU", Query: "Суммируй https://example.com/blog и выдели ключевые моменты.", Actions: []string{"суммируй"}, Objects: []string{"blog"}, Context: []string{"ключевые моменты"}},
			{Locale: "sk-SK", Query: "Zhrn https://example.com/blog a vytiahni hlavne body.", Actions: []string{"zhrn"}, Objects: []string{"blog"}, Context: []string{"hlavne body"}},
			{Locale: "sv-SE", Query: "Sammanfatta https://example.com/blog och lyft fram huvudpunkterna.", Actions: []string{"sammanfatta"}, Objects: []string{"blog"}, Context: []string{"huvudpunkterna"}},
			{Locale: "zh-CN", Query: "总结 https://example.com/blog 并提炼关键要点。", Actions: []string{"总结"}, Objects: []string{"blog"}, Context: []string{"关键要点"}},
			{Locale: "zh-TW", Query: "總結 https://example.com/blog 並提煉關鍵要點。", Actions: []string{"總結"}, Objects: []string{"blog"}, Context: []string{"關鍵要點"}},
		},
		"ui_reviewer": {
			{Locale: "ca-ES", Query: "Revisa la UI i l'accessibilitat de https://example.com/pricing.", Actions: []string{"revisa"}, Objects: []string{"ui"}, Context: []string{"accessibilitat"}},
			{Locale: "cs-CZ", Query: "Zkontroluj UI a pristupnost na https://example.com/pricing.", Actions: []string{"zkontroluj"}, Objects: []string{"ui"}, Context: []string{"pristupnost"}},
			{Locale: "da-DK", Query: "Gennemga UI og tilgaengelighed pa https://example.com/pricing.", Actions: []string{"gennemga"}, Objects: []string{"ui"}, Context: []string{"tilgaengelighed"}},
			{Locale: "de-DE", Query: "Prufe UI und Barrierefreiheit von https://example.com/pricing.", Actions: []string{"prufe"}, Objects: []string{"ui"}, Context: []string{"barrierefreiheit"}},
			{Locale: "el-GR", Query: "Έλεγξε το UI και την προσβασιμότητα του https://example.com/pricing.", Actions: []string{"έλεγξε"}, Objects: []string{"ui"}, Context: []string{"προσβασιμότητα"}},
			{Locale: "en-GB", Query: "Review the UI and accessibility of https://example.com/pricing.", Actions: []string{"review"}, Objects: []string{"ui"}, Context: []string{"accessibility"}},
			{Locale: "en-US", Query: "Review the UI and accessibility of https://example.com/pricing.", Actions: []string{"review"}, Objects: []string{"ui"}, Context: []string{"accessibility"}},
			{Locale: "es-ES", Query: "Revisa la UI y la accesibilidad de https://example.com/pricing.", Actions: []string{"revisa"}, Objects: []string{"ui"}, Context: []string{"accesibilidad"}},
			{Locale: "fr-FR", Query: "Examine l'interface et l'accessibilite de https://example.com/pricing.", Actions: []string{"examine"}, Objects: []string{"interface"}, Context: []string{"accessibilite"}},
			{Locale: "ga-IE", Query: "Dean athbhreithniu ar UI agus inrochtaineacht https://example.com/pricing.", Actions: []string{"athbhreithniu"}, Objects: []string{"ui"}, Context: []string{"inrochtaineacht"}},
			{Locale: "hr-HR", Query: "Pregledaj UI i pristupacnost stranice https://example.com/pricing.", Actions: []string{"pregledaj"}, Objects: []string{"ui"}, Context: []string{"pristupacnost"}},
			{Locale: "hu-HU", Query: "Nezd at a https://example.com/pricing UI-jat es akadalymentesseget.", Actions: []string{"nezd at"}, Objects: []string{"ui"}, Context: []string{"akadalymentesseget"}},
			{Locale: "it-IT", Query: "Rivedi la UI e l'accessibilita di https://example.com/pricing.", Actions: []string{"rivedi"}, Objects: []string{"ui"}, Context: []string{"accessibilita"}},
			{Locale: "ja-JP", Query: "https://example.com/pricing の UI とアクセシビリティをレビューして。", Actions: []string{"レビューして"}, Objects: []string{"ui"}, Context: []string{"アクセシビリティ"}},
			{Locale: "ko-KR", Query: "https://example.com/pricing 의 UI 와 접근성을 검토해 줘.", Actions: []string{"검토해"}, Objects: []string{"ui"}, Context: []string{"접근성"}},
			{Locale: "ml-IN", Query: "https://example.com/pricing എന്ന പേജിന്റെ UIയും ആക്സസിബിലിറ്റിയും പരിശോധിക്കൂ.", Actions: []string{"പരിശോധിക്കൂ"}, Objects: []string{"ui"}, Context: []string{"ആക്സസിബിലിറ്റി"}},
			{Locale: "nb-NO", Query: "Ga gjennom UI og tilgjengelighet for https://example.com/pricing.", Actions: []string{"ga gjennom"}, Objects: []string{"ui"}, Context: []string{"tilgjengelighet"}},
			{Locale: "nl-NL", Query: "Beoordeel de UI en toegankelijkheid van https://example.com/pricing.", Actions: []string{"beoordeel"}, Objects: []string{"ui"}, Context: []string{"toegankelijkheid"}},
			{Locale: "pl-PL", Query: "Przejrzyj UI i dostepnosc strony https://example.com/pricing.", Actions: []string{"przejrzyj"}, Objects: []string{"ui"}, Context: []string{"dostepnosc"}},
			{Locale: "pt-BR", Query: "Revise a UI e a acessibilidade de https://example.com/pricing.", Actions: []string{"revise"}, Objects: []string{"ui"}, Context: []string{"acessibilidade"}},
			{Locale: "pt-PT", Query: "Reve a UI e a acessibilidade de https://example.com/pricing.", Actions: []string{"reve"}, Objects: []string{"ui"}, Context: []string{"acessibilidade"}},
			{Locale: "ro-RO", Query: "Revizuieste UI-ul si accesibilitatea paginii https://example.com/pricing.", Actions: []string{"revizuieste"}, Objects: []string{"ui-ul"}, Context: []string{"accesibilitatea"}},
			{Locale: "ru-RU", Query: "Проверь UI и доступность страницы https://example.com/pricing.", Actions: []string{"проверь"}, Objects: []string{"ui"}, Context: []string{"доступность"}},
			{Locale: "sk-SK", Query: "Skontroluj UI a pristupnost stranky https://example.com/pricing.", Actions: []string{"skontroluj"}, Objects: []string{"ui"}, Context: []string{"pristupnost"}},
			{Locale: "sv-SE", Query: "Granska UI och tillganglighet for https://example.com/pricing.", Actions: []string{"granska"}, Objects: []string{"ui"}, Context: []string{"tillganglighet"}},
			{Locale: "zh-CN", Query: "审查 https://example.com/pricing 的 UI 和无障碍。", Actions: []string{"审查"}, Objects: []string{"ui"}, Context: []string{"无障碍"}},
			{Locale: "zh-TW", Query: "審查 https://example.com/pricing 的 UI 和無障礙。", Actions: []string{"審查"}, Objects: []string{"ui"}, Context: []string{"無障礙"}},
		},
	}
}

// SupportedLocales returns the canonical 27-locale routing surface.
func SupportedLocales() []string {
	ensureSupportedLocales()
	return append([]string(nil), supportedLocales...)
}

// LocalizedExamples returns curated multilingual examples for one skill.
func LocalizedExamples(skill string) []LocalizedSkillExample {
	ensureLocalizedSkillExamples()
	key := normalizeRoutingCueSkill(skill)
	examples := localizedSkillExamples[key]
	if len(examples) == 0 {
		ensureLocalizedURLBypassExamples()
		examples = localizedURLBypassExamples[key]
	}
	out := make([]LocalizedSkillExample, 0, len(examples))
	for _, example := range examples {
		out = append(out, LocalizedSkillExample{
			Locale:  example.Locale,
			Query:   example.Query,
			Actions: append([]string(nil), example.Actions...),
			Objects: append([]string(nil), example.Objects...),
			Context: append([]string(nil), example.Context...),
		})
	}
	return out
}

// LocalizedURLBypassExamples returns curated multilingual examples that
// should bypass the raw-URL -> browser rule while preserving skill routing.
func LocalizedURLBypassExamples(skill string) []LocalizedSkillExample {
	ensureLocalizedURLBypassExamples()
	examples := localizedURLBypassExamples[normalizeRoutingCueSkill(skill)]
	out := make([]LocalizedSkillExample, 0, len(examples))
	for _, example := range examples {
		out = append(out, LocalizedSkillExample{
			Locale:  example.Locale,
			Query:   example.Query,
			Actions: append([]string(nil), example.Actions...),
			Objects: append([]string(nil), example.Objects...),
			Context: append([]string(nil), example.Context...),
		})
	}
	return out
}

// SkillTerms aggregates multilingual cue terms for one skill.
func SkillTerms(skill string) SkillCue {
	ensureLocalizedSkillExamples()
	key := normalizeRoutingCueSkill(skill)
	examples := localizedSkillExamples[key]
	if len(examples) == 0 {
		ensureLocalizedURLBypassExamples()
		examples = localizedURLBypassExamples[key]
	}
	cue := SkillCue{
		Actions:  make([]string, 0, len(examples)),
		Objects:  make([]string, 0, len(examples)),
		Context:  make([]string, 0, len(examples)),
		Examples: make([]string, 0, len(examples)),
	}
	for _, example := range examples {
		cue.Actions = append(cue.Actions, example.Actions...)
		cue.Objects = append(cue.Objects, example.Objects...)
		cue.Context = append(cue.Context, example.Context...)
		cue.Examples = append(cue.Examples, example.Query)
	}
	cue.Actions = uniqueFolded(cue.Actions)
	cue.Objects = uniqueFolded(cue.Objects)
	cue.Context = uniqueFolded(cue.Context)
	cue.Examples = uniqueFolded(cue.Examples)
	return cue
}

// LiveWebTerms returns multilingual live-web cues shared by query heuristics.
func LiveWebTerms() []string {
	return SkillTerms("web_query").All()
}

// LocalWorkspaceTerms returns multilingual local-workspace cues shared by query heuristics.
func LocalWorkspaceTerms() []string {
	return SkillTerms("workspace_local").All()
}

// ProductivityTerms returns multilingual reminder/productivity cues shared by query heuristics.
func ProductivityTerms() []string {
	return SkillTerms("reminder").All()
}

// UIArtifactTerms returns multilingual UI review cues shared by query heuristics.
func UIArtifactTerms() []string {
	return uniqueFolded(append(SkillTerms("ui_reviewer").All(), URLBypassTermsForSkill("ui_reviewer")...))
}

// URLBypassTerms returns multilingual cues that should bypass the raw-URL -> browser rule.
func URLBypassTerms() []string {
	return uniqueFolded(append(
		append(
			append(SkillTerms("analyze").All(), SkillTerms("ui_reviewer").All()...),
			URLBypassTermsForSkill("analyze")...,
		),
		URLBypassTermsForSkill("ui_reviewer")...,
	))
}

// URLBypassTermsForSkill returns multilingual cue terms for raw-URL queries
// that should stay on a non-browser skill.
func URLBypassTermsForSkill(skill string) []string {
	examples := LocalizedURLBypassExamples(skill)
	cue := SkillCue{
		Actions:  make([]string, 0, len(examples)),
		Objects:  make([]string, 0, len(examples)),
		Context:  make([]string, 0, len(examples)),
		Examples: make([]string, 0, len(examples)),
	}
	for _, example := range examples {
		cue.Actions = append(cue.Actions, example.Actions...)
		cue.Objects = append(cue.Objects, example.Objects...)
		cue.Context = append(cue.Context, example.Context...)
		cue.Examples = append(cue.Examples, example.Query)
	}
	return cue.All()
}

// All returns the flattened unique cue set for one skill.
func (c SkillCue) All() []string {
	return uniqueFolded(append(append(append(append([]string(nil), c.Actions...), c.Objects...), c.Context...), c.Examples...))
}

func uniqueFolded(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		key := strings.ToLower(strings.TrimSpace(value))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}

func normalizeRoutingCueSkill(skill string) string {
	switch strings.ToLower(strings.TrimSpace(skill)) {
	case "web_search", "web-search", "websearch":
		return "web_query"
	default:
		return strings.ToLower(strings.TrimSpace(skill))
	}
}
