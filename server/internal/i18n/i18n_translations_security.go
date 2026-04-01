package i18n

func init() {
	// --- en-US (English) ---
	for k, v := range map[string]string{
		MsgSecPatternJailbreak:          "Known jailbreak attempts",
		MsgSecPatternIgnoreInstructions: "Attempts to override previous instructions",
		MsgSecPatternNewInstructions:    "Claims to provide new instructions",
		MsgSecPatternRoleOverride:       "Attempts to change the AI's role",
		MsgSecPatternSystemPrompt:       "Attempts to inject system-level prompts",
		MsgSecPatternCommandExecution:   "Requests to execute commands",
		MsgSecPatternDataExfiltration:   "Potential data exfiltration attempts",
		MsgSecPatternDelimiterInjection: "Markdown delimiters that may confuse parsing",
		MsgSecPatternElevatedAccess:     "Attempts to claim elevated access",
		MsgSecPatternRmRf:               "Dangerous file deletion command",
		MsgSecPatternDeleteAll:          "Mass deletion request",
		MsgSecPatternRoleSeparator:      "Attempts to inject role separators",
	} {
		AddTranslation(LangEnUS, k, v)
	}

	// --- zh-CN (Simplified Chinese) ---
	for k, v := range map[string]string{
		MsgSecPatternJailbreak:          "已知越狱尝试",
		MsgSecPatternIgnoreInstructions: "试图覆盖之前的指令",
		MsgSecPatternNewInstructions:    "声称提供新指令",
		MsgSecPatternRoleOverride:       "试图更改AI角色",
		MsgSecPatternSystemPrompt:       "试图注入系统级提示词",
		MsgSecPatternCommandExecution:   "请求执行命令",
		MsgSecPatternDataExfiltration:   "潜在的数据泄露尝试",
		MsgSecPatternDelimiterInjection: "可能混淆解析的Markdown分隔符",
		MsgSecPatternElevatedAccess:     "试图获取提升权限",
		MsgSecPatternRmRf:               "危险文件删除命令",
		MsgSecPatternDeleteAll:          "批量删除请求",
		MsgSecPatternRoleSeparator:      "试图注入角色分隔符",
	} {
		AddTranslation(LangZhCN, k, v)
	}

	// --- zh-TW (Traditional Chinese) ---
	for k, v := range map[string]string{
		MsgSecPatternJailbreak:          "已知越獄嘗試",
		MsgSecPatternIgnoreInstructions: "試圖覆蓋之前的指令",
		MsgSecPatternNewInstructions:    "聲稱提供新指令",
		MsgSecPatternRoleOverride:       "試圖更改AI角色",
		MsgSecPatternSystemPrompt:       "試圖注入系統級提示詞",
		MsgSecPatternCommandExecution:   "請求執行命令",
		MsgSecPatternDataExfiltration:   "潛在的數據洩露嘗試",
		MsgSecPatternDelimiterInjection: "可能混淆解析的Markdown分隔符",
		MsgSecPatternElevatedAccess:     "試圖獲取提升權限",
		MsgSecPatternRmRf:               "危險檔案刪除命令",
		MsgSecPatternDeleteAll:          "批量刪除請求",
		MsgSecPatternRoleSeparator:      "試圖注入角色分隔符",
	} {
		AddTranslation(LangZhTW, k, v)
	}

	// --- ja-JP (Japanese) ---
	for k, v := range map[string]string{
		MsgSecPatternJailbreak:          "既知のjailbreak試行",
		MsgSecPatternIgnoreInstructions: "以前の指示を上書きしようとする試み",
		MsgSecPatternNewInstructions:    "新しい指示を提供すると主張する",
		MsgSecPatternRoleOverride:       "AIの役割を変更しようとする試み",
		MsgSecPatternSystemPrompt:       "システムレベルのプロンプトを注入しようとする試み",
		MsgSecPatternCommandExecution:   "コマンドの実行を要求する",
		MsgSecPatternDataExfiltration:   "潜在的なデータ流出試行",
		MsgSecPatternDelimiterInjection: "解析を混乱させる可能性のあるMarkdown区切り文字",
		MsgSecPatternElevatedAccess:     "昇格アクセスを主張しようとする試み",
		MsgSecPatternRmRf:               "危険なファイル削除コマンド",
		MsgSecPatternDeleteAll:          "一括削除リクエスト",
		MsgSecPatternRoleSeparator:      "役割区切り文字を注入しようとする試み",
	} {
		AddTranslation(LangJaJP, k, v)
	}

	// --- ko-KR (Korean) ---
	for k, v := range map[string]string{
		MsgSecPatternJailbreak:          "알려진 jailbreak 시도",
		MsgSecPatternIgnoreInstructions: "이전 지시사항을 무시하려는 시도",
		MsgSecPatternNewInstructions:    "새로운 지시사항을 제공한다고 주장",
		MsgSecPatternRoleOverride:       "AI의 역할을 변경하려는 시도",
		MsgSecPatternSystemPrompt:       "시스템 수준 프롬프트를 주입하려는 시도",
		MsgSecPatternCommandExecution:   "명령 실행 요청",
		MsgSecPatternDataExfiltration:   "잠재적인 데이터 유출 시도",
		MsgSecPatternDelimiterInjection: "구문 분석을 혼란스럽게 할 수 있는 Markdown 구분자",
		MsgSecPatternElevatedAccess:     "승격된 액세스를 주장하려는 시도",
		MsgSecPatternRmRf:               "위험한 파일 삭제 명령",
		MsgSecPatternDeleteAll:          "대량 삭제 요청",
		MsgSecPatternRoleSeparator:      "역할 구분자 주입 시도",
	} {
		AddTranslation(LangKoKR, k, v)
	}

	// --- de-DE (German) ---
	for k, v := range map[string]string{
		MsgSecPatternJailbreak:          "Bekannte Jailbreak-Versuche",
		MsgSecPatternIgnoreInstructions: "Versuche, vorherige Anweisungen zu überschreiben",
		MsgSecPatternNewInstructions:    "Behauptet, neue Anweisungen zu liefern",
		MsgSecPatternRoleOverride:       "Versuche, die Rolle der KI zu ändern",
		MsgSecPatternSystemPrompt:       "Versuche, System-Prompts zu injizieren",
		MsgSecPatternCommandExecution:   "Aufforderungen zur Befehlsausführung",
		MsgSecPatternDataExfiltration:   "Potenzielle Datenexfiltrationsversuche",
		MsgSecPatternDelimiterInjection: "Markdown-Trennzeichen, die das Parsen verwirren können",
		MsgSecPatternElevatedAccess:     "Versuche, erhöhten Zugriff zu beanspruchen",
		MsgSecPatternRmRf:               "Gefährlicher Befehl zum Löschen von Dateien",
		MsgSecPatternDeleteAll:          "Massenlöschanfrage",
		MsgSecPatternRoleSeparator:      "Versuche, Rollentrennzeichen zu injizieren",
	} {
		AddTranslation(LangDeDE, k, v)
	}

	// --- fr-FR (French) ---
	for k, v := range map[string]string{
		MsgSecPatternJailbreak:          "Tentatives de jailbreak connues",
		MsgSecPatternIgnoreInstructions: "Tentatives de remplacer les instructions précédentes",
		MsgSecPatternNewInstructions:    "Prétend fournir de nouvelles instructions",
		MsgSecPatternRoleOverride:       "Tentatives de changer le rôle de l'IA",
		MsgSecPatternSystemPrompt:       "Tentatives d'injection de prompts système",
		MsgSecPatternCommandExecution:   "Demandes d'exécution de commandes",
		MsgSecPatternDataExfiltration:   "Tentatives potentielles d'exfiltration de données",
		MsgSecPatternDelimiterInjection: "Délimiteurs Markdown pouvant confondre l'analyse",
		MsgSecPatternElevatedAccess:     "Tentatives de revendiquer un accès élevé",
		MsgSecPatternRmRf:               "Commande dangereuse de suppression de fichiers",
		MsgSecPatternDeleteAll:          "Demande de suppression massive",
		MsgSecPatternRoleSeparator:      "Tentatives d'injection de séparateurs de rôle",
	} {
		AddTranslation(LangFrFR, k, v)
	}

	// --- es-ES (Spanish) ---
	for k, v := range map[string]string{
		MsgSecPatternJailbreak:          "Intentos de jailbreak conocidos",
		MsgSecPatternIgnoreInstructions: "Intentos de anular instrucciones anteriores",
		MsgSecPatternNewInstructions:    "Afirma proporcionar nuevas instrucciones",
		MsgSecPatternRoleOverride:       "Intentos de cambiar el rol de la IA",
		MsgSecPatternSystemPrompt:       "Intentos de inyectar prompts de nivel de sistema",
		MsgSecPatternCommandExecution:   "Solicitudes para ejecutar comandos",
		MsgSecPatternDataExfiltration:   "Intentos potenciales de exfiltración de datos",
		MsgSecPatternDelimiterInjection: "Delimitadores Markdown que pueden confundir el análisis",
		MsgSecPatternElevatedAccess:     "Intentos de reclamar acceso elevado",
		MsgSecPatternRmRf:               "Comando peligroso de eliminación de archivos",
		MsgSecPatternDeleteAll:          "Solicitud de eliminación masiva",
		MsgSecPatternRoleSeparator:      "Intentos de inyectar separadores de rol",
	} {
		AddTranslation(LangEsES, k, v)
	}

	// --- it-IT (Italian) ---
	for k, v := range map[string]string{
		MsgSecPatternJailbreak:          "Tentativi di jailbreak noti",
		MsgSecPatternIgnoreInstructions: "Tentativi di sovrascrivere le istruzioni precedenti",
		MsgSecPatternNewInstructions:    "Afferma di fornire nuove istruzioni",
		MsgSecPatternRoleOverride:       "Tentativi di cambiare il ruolo dell'IA",
		MsgSecPatternSystemPrompt:       "Tentativi di iniettare prompt a livello di sistema",
		MsgSecPatternCommandExecution:   "Richieste di eseguire comandi",
		MsgSecPatternDataExfiltration:   "Tentativi potenziali di esfiltrazione dati",
		MsgSecPatternDelimiterInjection: "Delimitatori Markdown che potrebbero confondere l'analisi",
		MsgSecPatternElevatedAccess:     "Tentativi di rivendicare accesso elevato",
		MsgSecPatternRmRf:               "Comando pericoloso di eliminazione file",
		MsgSecPatternDeleteAll:          "Richiesta di eliminazione massiva",
		MsgSecPatternRoleSeparator:      "Tentativi di iniettare separatori di ruolo",
	} {
		AddTranslation(LangItIT, k, v)
	}

	// --- pt-BR (Portuguese Brazil) ---
	for k, v := range map[string]string{
		MsgSecPatternJailbreak:          "Tentativas de jailbreak conhecidas",
		MsgSecPatternIgnoreInstructions: "Tentativas de substituir instruções anteriores",
		MsgSecPatternNewInstructions:    "Alega fornecer novas instruções",
		MsgSecPatternRoleOverride:       "Tentativas de alterar o papel da IA",
		MsgSecPatternSystemPrompt:       "Tentativas de injetar prompts de nível de sistema",
		MsgSecPatternCommandExecution:   "Solicitações para executar comandos",
		MsgSecPatternDataExfiltration:   "Tentativas potenciais de exfiltração de dados",
		MsgSecPatternDelimiterInjection: "Delimitadores Markdown que podem confundir a análise",
		MsgSecPatternElevatedAccess:     "Tentativas de reivindicar acesso elevado",
		MsgSecPatternRmRf:               "Comando perigoso de exclusão de arquivos",
		MsgSecPatternDeleteAll:          "Solicitação de exclusão em massa",
		MsgSecPatternRoleSeparator:      "Tentativas de injetar separadores de função",
	} {
		AddTranslation(LangPtBR, k, v)
	}

	// --- pt-PT (Portuguese Portugal) ---
	for k, v := range map[string]string{
		MsgSecPatternJailbreak:          "Tentativas de jailbreak conhecidas",
		MsgSecPatternIgnoreInstructions: "Tentativas de substituir instruções anteriores",
		MsgSecPatternNewInstructions:    "Alega fornecer novas instruções",
		MsgSecPatternRoleOverride:       "Tentativas de alterar a função da IA",
		MsgSecPatternSystemPrompt:       "Tentativas de injetar prompts de nível de sistema",
		MsgSecPatternCommandExecution:   "Pedidos para executar comandos",
		MsgSecPatternDataExfiltration:   "Tentativas potenciais de exfiltração de dados",
		MsgSecPatternDelimiterInjection: "Delimitadores Markdown que podem confundir a análise",
		MsgSecPatternElevatedAccess:     "Tentativas de reclamar acesso elevado",
		MsgSecPatternRmRf:               "Comando perigoso de eliminação de ficheiros",
		MsgSecPatternDeleteAll:          "Pedido de eliminação em massa",
		MsgSecPatternRoleSeparator:      "Tentativas de injetar separadores de função",
	} {
		AddTranslation(LangPtPT, k, v)
	}

	// --- ru-RU (Russian) ---
	for k, v := range map[string]string{
		MsgSecPatternJailbreak:          "Известные попытки jailbreak",
		MsgSecPatternIgnoreInstructions: "Попытки переопределить предыдущие инструкции",
		MsgSecPatternNewInstructions:    "Утверждает, что предоставляет новые инструкции",
		MsgSecPatternRoleOverride:       "Попытки изменить роль ИИ",
		MsgSecPatternSystemPrompt:       "Попытки внедрить системные промпты",
		MsgSecPatternCommandExecution:   "Запросы на выполнение команд",
		MsgSecPatternDataExfiltration:   "Потенциальные попытки эксфильтрации данных",
		MsgSecPatternDelimiterInjection: "Разделители Markdown, которые могут запутать парсинг",
		MsgSecPatternElevatedAccess:     "Попытки получить повышенный доступ",
		MsgSecPatternRmRf:               "Опасная команда удаления файлов",
		MsgSecPatternDeleteAll:          "Запрос на массовое удаление",
		MsgSecPatternRoleSeparator:      "Попытки внедрить разделители ролей",
	} {
		AddTranslation(LangRuRU, k, v)
	}

	// --- pl-PL (Polish) ---
	for k, v := range map[string]string{
		MsgSecPatternJailbreak:          "Znane próby jailbreak",
		MsgSecPatternIgnoreInstructions: "Próby zastąpienia poprzednich instrukcji",
		MsgSecPatternNewInstructions:    "Twierdzi, że dostarcza nowe instrukcje",
		MsgSecPatternRoleOverride:       "Próby zmiany roli AI",
		MsgSecPatternSystemPrompt:       "Próby wstrzyknięcia promptów systemowych",
		MsgSecPatternCommandExecution:   "Żądania wykonania poleceń",
		MsgSecPatternDataExfiltration:   "Potencjalne próby eksfiltracji danych",
		MsgSecPatternDelimiterInjection: "Ograniczniki Markdown, które mogą mylić parsowanie",
		MsgSecPatternElevatedAccess:     "Próby uzyskania podwyższonych uprawnień",
		MsgSecPatternRmRf:               "Niebezpieczne polecenie usuwania plików",
		MsgSecPatternDeleteAll:          "Żądanie masowego usuwania",
		MsgSecPatternRoleSeparator:      "Próby wstrzyknięcia separatorów ról",
	} {
		AddTranslation(LangPlPL, k, v)
	}

	// --- nl-NL (Dutch) ---
	for k, v := range map[string]string{
		MsgSecPatternJailbreak:          "Bekende jailbreak-pogingen",
		MsgSecPatternIgnoreInstructions: "Pogingen om eerdere instructies te negeren",
		MsgSecPatternNewInstructions:    "Beweer nieuwe instructies te verstrekken",
		MsgSecPatternRoleOverride:       "Pogingen om de rol van de AI te wijzigen",
		MsgSecPatternSystemPrompt:       "Pogingen om systeemprompts te injecteren",
		MsgSecPatternCommandExecution:   "Verzoeken om opdrachten uit te voeren",
		MsgSecPatternDataExfiltration:   "Potentiële pogingen tot data-exfiltratie",
		MsgSecPatternDelimiterInjection: "Markdown-scheidingstekens die het parsen kunnen verwarren",
		MsgSecPatternElevatedAccess:     "Pogingen om verhoogde toegang te claimen",
		MsgSecPatternRmRf:               "Gevaarlijk bestandsverwijderingscommando",
		MsgSecPatternDeleteAll:          "Massale verwijderingsaanvraag",
		MsgSecPatternRoleSeparator:      "Pogingen om rolscheidingstekens te injecteren",
	} {
		AddTranslation(LangNlNL, k, v)
	}

	// --- sv-SE (Swedish) ---
	for k, v := range map[string]string{
		MsgSecPatternJailbreak:          "Kända jailbreak-försök",
		MsgSecPatternIgnoreInstructions: "Försök att åsidosätta tidigare instruktioner",
		MsgSecPatternNewInstructions:    "Påstår sig tillhandahålla nya instruktioner",
		MsgSecPatternRoleOverride:       "Försök att ändra AI:ns roll",
		MsgSecPatternSystemPrompt:       "Försök att injicera systemnivå-prompter",
		MsgSecPatternCommandExecution:   "Förfrågningar om att utföra kommandon",
		MsgSecPatternDataExfiltration:   "Potentiella försök till dataexfiltrering",
		MsgSecPatternDelimiterInjection: "Markdown-avgränsare som kan förvirra parsning",
		MsgSecPatternElevatedAccess:     "Försök att göra anspråk på förhöjd åtkomst",
		MsgSecPatternRmRf:               "Farligt kommando för filradering",
		MsgSecPatternDeleteAll:          "Begäran om massradering",
		MsgSecPatternRoleSeparator:      "Försök att injicera rollavgränsare",
	} {
		AddTranslation(LangSvSE, k, v)
	}

	// --- da-DK (Danish) ---
	for k, v := range map[string]string{
		MsgSecPatternJailbreak:          "Kendte jailbreak-forsøg",
		MsgSecPatternIgnoreInstructions: "Forsøg på at tilsidesætte tidligere instruktioner",
		MsgSecPatternNewInstructions:    "Påstår at give nye instruktioner",
		MsgSecPatternRoleOverride:       "Forsøg på at ændre AI'ens rolle",
		MsgSecPatternSystemPrompt:       "Forsøg på at injicere systemniveau-prompts",
		MsgSecPatternCommandExecution:   "Anmodninger om at udføre kommandoer",
		MsgSecPatternDataExfiltration:   "Potentielle forsøg på dataeksfiltrering",
		MsgSecPatternDelimiterInjection: "Markdown-afgrænsere, der kan forvirre parsing",
		MsgSecPatternElevatedAccess:     "Forsøg på at gøre krav på forhøjet adgang",
		MsgSecPatternRmRf:               "Farlig kommando til sletning af filer",
		MsgSecPatternDeleteAll:          "Anmodning om massesletning",
		MsgSecPatternRoleSeparator:      "Forsøg på at injicere rolleafgrænsere",
	} {
		AddTranslation(LangDaDK, k, v)
	}

	// --- nb-NO (Norwegian Bokmål) ---
	for k, v := range map[string]string{
		MsgSecPatternJailbreak:          "Kjente jailbreak-forsøk",
		MsgSecPatternIgnoreInstructions: "Forsøk på å overstyre tidligere instruksjoner",
		MsgSecPatternNewInstructions:    "Hevder å gi nye instruksjoner",
		MsgSecPatternRoleOverride:       "Forsøk på å endre AI-en sin rolle",
		MsgSecPatternSystemPrompt:       "Forsøk på å injisere systemnivå-prompter",
		MsgSecPatternCommandExecution:   "Forespørsler om å utføre kommandoer",
		MsgSecPatternDataExfiltration:   "Potensielle forsøk på dataeksfiltrering",
		MsgSecPatternDelimiterInjection: "Markdown-skilletegn som kan forvirre parsing",
		MsgSecPatternElevatedAccess:     "Forsøk på å kreve forhøyet tilgang",
		MsgSecPatternRmRf:               "Farlig kommando for filsletting",
		MsgSecPatternDeleteAll:          "Forespørsel om massesletting",
		MsgSecPatternRoleSeparator:      "Forsøk på å injisere rolskilletegn",
	} {
		AddTranslation(LangNbNO, k, v)
	}

	// --- cs-CZ (Czech) ---
	for k, v := range map[string]string{
		MsgSecPatternJailbreak:          "Známé pokusy o jailbreak",
		MsgSecPatternIgnoreInstructions: "Pokusy o přepsání předchozích instrukcí",
		MsgSecPatternNewInstructions:    "Tvrdí, že poskytuje nové instrukce",
		MsgSecPatternRoleOverride:       "Pokusy o změnu role AI",
		MsgSecPatternSystemPrompt:       "Pokusy o injektáž systémových promptů",
		MsgSecPatternCommandExecution:   "Žádosti o provedení příkazů",
		MsgSecPatternDataExfiltration:   "Potenciální pokusy o exfiltraci dat",
		MsgSecPatternDelimiterInjection: "Oddělovače Markdown, které mohou zmást parsování",
		MsgSecPatternElevatedAccess:     "Pokusy o nárok na zvýšený přístup",
		MsgSecPatternRmRf:               "Nebezpečný příkaz pro mazání souborů",
		MsgSecPatternDeleteAll:          "Žádost o hromadné mazání",
		MsgSecPatternRoleSeparator:      "Pokusy o injektáž oddělovačů rolí",
	} {
		AddTranslation(LangCsCZ, k, v)
	}

	// --- sk-SK (Slovak) ---
	for k, v := range map[string]string{
		MsgSecPatternJailbreak:          "Známe pokusy o jailbreak",
		MsgSecPatternIgnoreInstructions: "Pokusy o prepísanie predchádzajúcich inštrukcií",
		MsgSecPatternNewInstructions:    "Tvrdí, že poskytuje nové inštrukcie",
		MsgSecPatternRoleOverride:       "Pokusy o zmenu roly AI",
		MsgSecPatternSystemPrompt:       "Pokusy o injektáž systémových promptov",
		MsgSecPatternCommandExecution:   "Žiadosti o vykonanie príkazov",
		MsgSecPatternDataExfiltration:   "Potenciálne pokusy o exfiltráciu dát",
		MsgSecPatternDelimiterInjection: "Oddeľovače Markdown, ktoré môžu miasť parsovanie",
		MsgSecPatternElevatedAccess:     "Pokusy o nárok na zvýšený prístup",
		MsgSecPatternRmRf:               "Nebezpečný príkaz na mazanie súborov",
		MsgSecPatternDeleteAll:          "Žiadosť o hromadné mazanie",
		MsgSecPatternRoleSeparator:      "Pokusy o injektáž oddeľovačov rolí",
	} {
		AddTranslation(LangSkSK, k, v)
	}

	// --- hu-HU (Hungarian) ---
	for k, v := range map[string]string{
		MsgSecPatternJailbreak:          "Ismert jailbreak-kísérletek",
		MsgSecPatternIgnoreInstructions: "Kísérletek a korábbi utasítások felülírására",
		MsgSecPatternNewInstructions:    "Azt állítja, hogy új utasításokat ad",
		MsgSecPatternRoleOverride:       "Kísérletek az AI szerepének megváltoztatására",
		MsgSecPatternSystemPrompt:       "Kísérletek rendszerszintű promptok befecskendezésére",
		MsgSecPatternCommandExecution:   "Parancsok végrehajtására vonatkozó kérések",
		MsgSecPatternDataExfiltration:   "Lehetséges adatszivárgási kísérletek",
		MsgSecPatternDelimiterInjection: "Markdown elválasztók, amelyek összezavarhatják az elemzést",
		MsgSecPatternElevatedAccess:     "Kísérletek megemelt hozzáférés követelésére",
		MsgSecPatternRmRf:               "Veszélyes fájltörlési parancs",
		MsgSecPatternDeleteAll:          "Tömeges törlési kérelem",
		MsgSecPatternRoleSeparator:      "Kísérletek szerepelválasztók befecskendezésére",
	} {
		AddTranslation(LangHuHU, k, v)
	}

	// --- ro-RO (Romanian) ---
	for k, v := range map[string]string{
		MsgSecPatternJailbreak:          "Încercări de jailbreak cunoscute",
		MsgSecPatternIgnoreInstructions: "Încercări de a înlocui instrucțiunile anterioare",
		MsgSecPatternNewInstructions:    "Revendică furnizarea de instrucțiuni noi",
		MsgSecPatternRoleOverride:       "Încercări de a schimba rolul AI-ului",
		MsgSecPatternSystemPrompt:       "Încercări de a injecta prompturi la nivel de sistem",
		MsgSecPatternCommandExecution:   "Cereri de executare a comenzilor",
		MsgSecPatternDataExfiltration:   "Încercări potențiale de exfiltrare a datelor",
		MsgSecPatternDelimiterInjection: "Delimitatori Markdown care pot confunda parsarea",
		MsgSecPatternElevatedAccess:     "Încercări de a revendica acces ridicat",
		MsgSecPatternRmRf:               "Comandă periculoasă de ștergere a fișierelor",
		MsgSecPatternDeleteAll:          "Cerere de ștergere în masă",
		MsgSecPatternRoleSeparator:      "Încercări de a injecta separatoare de rol",
	} {
		AddTranslation(LangRoRO, k, v)
	}

	// --- hr-HR (Croatian) ---
	for k, v := range map[string]string{
		MsgSecPatternJailbreak:          "Poznati pokušaji jailbreaka",
		MsgSecPatternIgnoreInstructions: "Pokušaji zanemarivanja prethodnih uputa",
		MsgSecPatternNewInstructions:    "Tvrdi da pruža nove upute",
		MsgSecPatternRoleOverride:       "Pokušaji promjene uloge AI-a",
		MsgSecPatternSystemPrompt:       "Pokušaji ubrizgavanja promptova na razini sustava",
		MsgSecPatternCommandExecution:   "Zahtjevi za izvršavanje naredbi",
		MsgSecPatternDataExfiltration:   "Potencijalni pokušaji ekshiltracije podataka",
		MsgSecPatternDelimiterInjection: "Markdown razdjelnici koji mogu zbuniti parsiranje",
		MsgSecPatternElevatedAccess:     "Pokušaji zatraživanja povišenog pristupa",
		MsgSecPatternRmRf:               "Opasna naredba za brisanje datoteka",
		MsgSecPatternDeleteAll:          "Zahtjev za masovno brisanje",
		MsgSecPatternRoleSeparator:      "Pokušaji ubrizgavanja razdjelnika uloga",
	} {
		AddTranslation(LangHrHR, k, v)
	}

	// --- el-GR (Greek) ---
	for k, v := range map[string]string{
		MsgSecPatternJailbreak:          "Γνωστές απόπειρες jailbreak",
		MsgSecPatternIgnoreInstructions: "Απόπειρες παράκαμψης προηγούμενων οδηγιών",
		MsgSecPatternNewInstructions:    "Ισχυρίζεται ότι παρέχει νέες οδηγίες",
		MsgSecPatternRoleOverride:       "Απόπειρες αλλαγής του ρόλου της AI",
		MsgSecPatternSystemPrompt:       "Απόπειρες έγχυσης prompt συστήματος",
		MsgSecPatternCommandExecution:   "Αιτήματα εκτέλεσης εντολών",
		MsgSecPatternDataExfiltration:   "Πιθανές απόπειρες εξαγωγής δεδομένων",
		MsgSecPatternDelimiterInjection: "Διαχωριστικά Markdown που μπορεί να συγχέουν την ανάλυση",
		MsgSecPatternElevatedAccess:     "Απόπειρες διεκδίκησης αυξημένης πρόσβασης",
		MsgSecPatternRmRf:               "Επικίνδυνη εντολή διαγραφής αρχείων",
		MsgSecPatternDeleteAll:          "Αίτημα μαζικής διαγραφής",
		MsgSecPatternRoleSeparator:      "Απόπειρες έγχυσης διαχωριστικών ρόλων",
	} {
		AddTranslation(LangElGR, k, v)
	}

	// --- ca-ES (Catalan) ---
	for k, v := range map[string]string{
		MsgSecPatternJailbreak:          "Intents de jailbreak coneguts",
		MsgSecPatternIgnoreInstructions: "Intents de substituir instruccions anteriors",
		MsgSecPatternNewInstructions:    "Afirma proporcionar noves instruccions",
		MsgSecPatternRoleOverride:       "Intents de canviar el rol de la IA",
		MsgSecPatternSystemPrompt:       "Intents d'injecció de prompts a nivell de sistema",
		MsgSecPatternCommandExecution:   "Peticions per executar comandes",
		MsgSecPatternDataExfiltration:   "Intents potencials d'exfiltració de dades",
		MsgSecPatternDelimiterInjection: "Delimitadors Markdown que poden confondre l'anàlisi",
		MsgSecPatternElevatedAccess:     "Intents de reclamar accés elevat",
		MsgSecPatternRmRf:               "Comanda perillosa d'eliminació de fitxers",
		MsgSecPatternDeleteAll:          "Petició d'eliminació massiva",
		MsgSecPatternRoleSeparator:      "Intents d'injecció de separadors de rol",
	} {
		AddTranslation(LangCaES, k, v)
	}

	// --- ga-IE (Irish) ---
	for k, v := range map[string]string{
		MsgSecPatternJailbreak:          "Iarrachtaí jailbreak aitheanta",
		MsgSecPatternIgnoreInstructions: "Iarrachtaí treoracha roimhe seo a shárú",
		MsgSecPatternNewInstructions:    "Aíonn sé treoracha nua a sholáthar",
		MsgSecPatternRoleOverride:       "Iarrachtaí ról an AI a athrú",
		MsgSecPatternSystemPrompt:       "Iarrachtaí leideanna leibhéil córais a instealladh",
		MsgSecPatternCommandExecution:   "Iarrataí orduithe a fhorghníomhú",
		MsgSecPatternDataExfiltration:   "Iarrachtaí féideartha exfiltration sonraí",
		MsgSecPatternDelimiterInjection: "Dealraithe Markdown a d'fhéadfadh an parsáil a chur amú",
		MsgSecPatternElevatedAccess:     "Iarrachtaí rochtain ardaithe a éileamh",
		MsgSecPatternRmRf:               "Ordú scriosta comhad contúirteach",
		MsgSecPatternDeleteAll:          "Iarratas scriosta mórscale",
		MsgSecPatternRoleSeparator:      "Iarrachtaí deilimitheoirí ról a instealladh",
	} {
		AddTranslation(LangGaIE, k, v)
	}

	// --- ml-IN (Malayalam) ---
	for k, v := range map[string]string{
		MsgSecPatternJailbreak:          "അറിയപ്പെടുന്ന jailbreak ശ്രമങ്ങൾ",
		MsgSecPatternIgnoreInstructions: "മുമ്പത്തെ നിർദ്ദേശങ്ങൾ അവഗണിക്കാൻ ശ്രമിക്കുന്നു",
		MsgSecPatternNewInstructions:    "പുതിയ നിർദ്ദേശങ്ങൾ നൽകുന്നതായി അവകാശപ്പെടുന്നു",
		MsgSecPatternRoleOverride:       "AI-ന്റെ റോൾ മാറ്റാൻ ശ്രമിക്കുന്നു",
		MsgSecPatternSystemPrompt:       "സിസ്റ്റം തല പ്രോംപ്റ്റുകൾ ഇൻജക്ട് ചെയ്യാൻ ശ്രമിക്കുന്നു",
		MsgSecPatternCommandExecution:   "കമാൻഡുകൾ എക്സിക്യൂട്ട് ചെയ്യാൻ അഭ്യർത്ഥനകൾ",
		MsgSecPatternDataExfiltration:   "സംഭാവ്യ ഡാറ്റ എക്സ്ഫിൽട്രേഷൻ ശ്രമങ്ങൾ",
		MsgSecPatternDelimiterInjection: "പാർസിംഗ് കണഫ്യൂസ് ചെയ്യാൻ കഴിയുന്ന മാർക്ഡൗൺ ഡിലിമിറ്ററുകൾ",
		MsgSecPatternElevatedAccess:     "ഉയർന്ന ആക്സസ് അവകാശപ്പെടാൻ ശ്രമിക്കുന്നു",
		MsgSecPatternRmRf:               "പ്രമാദകരമായ ഫയൽ ഡിലീറ്റ് കമാൻഡ്",
		MsgSecPatternDeleteAll:          "വലിയ തോതിലുള്ള ഡിലീറ്റ് അഭ്യർത്ഥന",
		MsgSecPatternRoleSeparator:      "റോൾ സെപ്പറേറ്ററുകൾ ഇൻജക്ട് ചെയ്യാൻ ശ്രമിക്കുന്നു",
	} {
		AddTranslation(LangMlIN, k, v)
	}
}
