package preview

// i18nTexts holds the translated text strings for each question ID.
type i18nTexts struct {
	Q1, Q2, Q3, Q4, Q6, Q7, Q8, Q9, Q10       string
	Q11, Q12, Q13, Q14, Q15, Q16, Q17, Q18     string
	Q19, Q20, Q22, Q23, Q42, Q43, Q44, Q45     string
	Q47, Q49, Q50, Q51, Q52                     string
}

// buildPresetQuestions constructs a preset question list from translated texts.
func buildPresetQuestions(t i18nTexts) []PresetQuestion {
	return []PresetQuestion{
		{ID: "q1", Text: t.Q1, Category: "writing", Icon: "✉️"},
		{ID: "q2", Text: t.Q2, Category: "learning", Icon: "🎓"},
		{ID: "q3", Text: t.Q3, Category: "entertainment", Icon: "🎬"},
		{ID: "q4", Text: t.Q4, Category: "lifestyle", Icon: "💪"},
		{ID: "q6", Text: t.Q6, Category: "coding", Icon: "💻"},
		{ID: "q7", Text: t.Q7, Category: "coding", Icon: "🔧"},
		{ID: "q8", Text: t.Q8, Category: "tech", Icon: "🐳"},
		{ID: "q9", Text: t.Q9, Category: "creative", Icon: "🌸"},
		{ID: "q10", Text: t.Q10, Category: "creative", Icon: "💡"},
		{ID: "q11", Text: t.Q11, Category: "nas", Icon: "💾"},
		{ID: "q12", Text: t.Q12, Category: "nas", Icon: "🏠"},
		{ID: "q13", Text: t.Q13, Category: "lifestyle", Icon: "🍽️"},
		{ID: "q14", Text: t.Q14, Category: "travel", Icon: "✈️"},
		{ID: "q15", Text: t.Q15, Category: "lifestyle", Icon: "🌅"},
		{ID: "q16", Text: t.Q16, Category: "learning", Icon: "📚"},
		{ID: "q17", Text: t.Q17, Category: "learning", Icon: "🔗"},
		{ID: "q18", Text: t.Q18, Category: "learning", Icon: "⚛️"},
		{ID: "q19", Text: t.Q19, Category: "career", Icon: "👔"},
		{ID: "q20", Text: t.Q20, Category: "productivity", Icon: "⚡"},
		{ID: "q22", Text: t.Q22, Category: "vision", Icon: "🎨", Attachments: []PresetQuestionAttachment{
			{Type: "image", Name: "cityscape.jpg", MimeType: "image/jpeg", Placeholder: "sample-scene"},
		}},
		{ID: "q23", Text: t.Q23, Category: "vision", Icon: "📷", Attachments: []PresetQuestionAttachment{
			{Type: "image", Name: "invoice.jpg", MimeType: "image/jpeg", Placeholder: "sample-text-image"},
		}},
		{ID: "q42", Text: t.Q42, Category: "agent2-ui", Icon: "📈", Attachments: []PresetQuestionAttachment{
			{Type: "image", Name: "chart.png", MimeType: "image/png", Placeholder: "sample-chart"},
		}},
		{ID: "q43", Text: t.Q43, Category: "agent2-ui", Icon: "💻", Attachments: []PresetQuestionAttachment{
			{Type: "file", Name: "hello.py", MimeType: "text/x-python", Placeholder: "sample-code"},
		}},
		{ID: "q44", Text: t.Q44, Category: "agent2-ui", Icon: "🔧", Attachments: []PresetQuestionAttachment{
			{Type: "file", Name: "hello.py", MimeType: "text/x-python", Placeholder: "sample-code"},
		}},
		{ID: "q45", Text: t.Q45, Category: "agent2-ui", Icon: "📋", Attachments: []PresetQuestionAttachment{
			{Type: "file", Name: "report.txt", MimeType: "text/plain", Placeholder: "sample-document"},
		}},
		{ID: "q47", Text: t.Q47, Category: "agent2-ui", Icon: "🖼️", Attachments: []PresetQuestionAttachment{
			{Type: "image", Name: "landscape.jpg", MimeType: "image/jpeg", Placeholder: "sample-image"},
		}},
		{ID: "q49", Text: t.Q49, Category: "agent2-ui", Icon: "📊", Attachments: []PresetQuestionAttachment{
			{Type: "file", Name: "sales_data.csv", MimeType: "text/csv", Placeholder: "sample-csv"},
		}},
		{ID: "q50", Text: t.Q50, Category: "agent2-ui", Icon: "⚙️", Attachments: []PresetQuestionAttachment{
			{Type: "file", Name: "config.json", MimeType: "application/json", Placeholder: "sample-json"},
		}},
		{ID: "q51", Text: t.Q51, Category: "agent2-ui", Icon: "🐛", Attachments: []PresetQuestionAttachment{
			{Type: "file", Name: "buggy_calculator.js", MimeType: "text/javascript", Placeholder: "sample-js"},
		}},
		{ID: "q52", Text: t.Q52, Category: "creative", Icon: "📐"},
	}
}

func traditionalChinesePresetQuestions() []PresetQuestion {
	return buildPresetQuestions(i18nTexts{
		Q1: "幫我寫一封請假信", Q2: "解釋什麼是機器學習", Q3: "推薦幾部科幻電影", Q4: "幫我制定健身計畫",
		Q6: "用 Python 寫一個快速排序", Q7: "解釋 REST API 的設計原則", Q8: "Docker 和虛擬機有什麼差別？",
		Q9: "幫我寫一首關於春天的詩", Q10: "幫我的新產品取個名字",
		Q11: "如何設定 NAS 自動備份？", Q12: "推薦適合家庭使用的 NAS 應用",
		Q13: "今天晚餐吃什麼好？", Q14: "幫我規劃一趟週末旅行", Q15: "如何養成早起的習慣？",
		Q16: "學英文有什麼好方法？", Q17: "解釋區塊鏈的運作原理", Q18: "什麼是量子運算？",
		Q19: "如何準備技術面試？", Q20: "如何提升工作效率？",
		Q22: "分析這張照片的構圖和色彩", Q23: "幫我辨識圖片中的文字",
		Q42: "幫我解讀這個圖表的數據", Q43: "分析這段程式碼的結構和邏輯", Q44: "幫我優化這段程式碼",
		Q45: "分析這份銷售報告並給出建議", Q47: "描述這張風景圖片的內容",
		Q49: "分析這個 CSV 銷售數據並找出趨勢", Q50: "幫我檢查這個設定檔是否有問題",
		Q51: "找出這段 JavaScript 程式碼中的 bug", Q52: "畫一個使用者註冊登入的流程圖",
	})
}

func germanPresetQuestions() []PresetQuestion {
	return buildPresetQuestions(i18nTexts{
		Q1: "Hilf mir, eine Abwesenheitsmail zu schreiben", Q2: "Erkläre, was maschinelles Lernen ist", Q3: "Empfiehl mir ein paar Sci-Fi-Filme", Q4: "Erstelle mir einen Fitnessplan",
		Q6: "Schreibe einen Quicksort in Python", Q7: "Erkläre die Designprinzipien von REST-APIs", Q8: "Was ist der Unterschied zwischen Docker und VMs?",
		Q9: "Schreibe ein Gedicht über den Frühling", Q10: "Hilf mir, einen Namen für mein neues Produkt zu finden",
		Q11: "Wie richte ich ein automatisches NAS-Backup ein?", Q12: "Empfiehl NAS-Apps für den Heimgebrauch",
		Q13: "Was soll ich heute Abend essen?", Q14: "Hilf mir, einen Wochenendausflug zu planen", Q15: "Wie gewöhne ich mir das Frühaufstehen an?",
		Q16: "Wie lerne ich am besten Englisch?", Q17: "Erkläre, wie Blockchain funktioniert", Q18: "Was ist Quantencomputing?",
		Q19: "Wie bereite ich mich auf ein technisches Vorstellungsgespräch vor?", Q20: "Wie kann ich meine Arbeitseffizienz steigern?",
		Q22: "Analysiere Komposition und Farben dieses Fotos", Q23: "Erkenne den Text in diesem Bild",
		Q42: "Interpretiere die Daten in diesem Diagramm", Q43: "Analysiere Struktur und Logik dieses Codes", Q44: "Hilf mir, diesen Code zu optimieren",
		Q45: "Analysiere diesen Verkaufsbericht und gib Empfehlungen", Q47: "Beschreibe den Inhalt dieses Landschaftsbildes",
		Q49: "Analysiere diese CSV-Verkaufsdaten und finde Trends", Q50: "Prüfe diese Konfigurationsdatei auf Fehler",
		Q51: "Finde die Bugs in diesem JavaScript-Code", Q52: "Zeichne ein Flussdiagramm für Benutzerregistrierung und Login",
	})
}

func frenchPresetQuestions() []PresetQuestion {
	return buildPresetQuestions(i18nTexts{
		Q1: "Aide-moi à rédiger un e-mail de demande de congé", Q2: "Explique ce qu'est le machine learning", Q3: "Recommande-moi des films de science-fiction", Q4: "Aide-moi à créer un programme de fitness",
		Q6: "Écris un tri rapide en Python", Q7: "Explique les principes de conception des API REST", Q8: "Quelle est la différence entre Docker et les machines virtuelles ?",
		Q9: "Écris un poème sur le printemps", Q10: "Aide-moi à trouver un nom pour mon nouveau produit",
		Q11: "Comment configurer une sauvegarde automatique sur un NAS ?", Q12: "Recommande des applications NAS pour la maison",
		Q13: "Qu'est-ce que je mange ce soir ?", Q14: "Aide-moi à planifier un week-end", Q15: "Comment prendre l'habitude de se lever tôt ?",
		Q16: "Quelles sont les bonnes méthodes pour apprendre l'anglais ?", Q17: "Explique le fonctionnement de la blockchain", Q18: "Qu'est-ce que l'informatique quantique ?",
		Q19: "Comment se préparer à un entretien technique ?", Q20: "Comment améliorer ma productivité au travail ?",
		Q22: "Analyse la composition et les couleurs de cette photo", Q23: "Reconnais le texte dans cette image",
		Q42: "Interprète les données de ce graphique", Q43: "Analyse la structure et la logique de ce code", Q44: "Aide-moi à optimiser ce code",
		Q45: "Analyse ce rapport de ventes et donne des recommandations", Q47: "Décris le contenu de cette image de paysage",
		Q49: "Analyse ces données CSV de ventes et trouve les tendances", Q50: "Vérifie s'il y a des erreurs dans ce fichier de configuration",
		Q51: "Trouve les bugs dans ce code JavaScript", Q52: "Dessine un diagramme de flux pour l'inscription et la connexion",
	})
}

func spanishPresetQuestions() []PresetQuestion {
	return buildPresetQuestions(i18nTexts{
		Q1: "Ayúdame a escribir un correo de solicitud de permiso", Q2: "Explica qué es el aprendizaje automático", Q3: "Recomiéndame películas de ciencia ficción", Q4: "Ayúdame a crear un plan de ejercicios",
		Q6: "Escribe un quicksort en Python", Q7: "Explica los principios de diseño de las API REST", Q8: "¿Cuál es la diferencia entre Docker y las máquinas virtuales?",
		Q9: "Escribe un poema sobre la primavera", Q10: "Ayúdame a ponerle nombre a mi nuevo producto",
		Q11: "¿Cómo configurar copias de seguridad automáticas en un NAS?", Q12: "Recomienda aplicaciones NAS para el hogar",
		Q13: "¿Qué debería cenar hoy?", Q14: "Ayúdame a planificar una escapada de fin de semana", Q15: "¿Cómo adquirir el hábito de madrugar?",
		Q16: "¿Cuáles son buenos métodos para aprender inglés?", Q17: "Explica cómo funciona la blockchain", Q18: "¿Qué es la computación cuántica?",
		Q19: "¿Cómo prepararse para una entrevista técnica?", Q20: "¿Cómo puedo mejorar mi productividad laboral?",
		Q22: "Analiza la composición y los colores de esta foto", Q23: "Reconoce el texto en esta imagen",
		Q42: "Interpreta los datos de este gráfico", Q43: "Analiza la estructura y lógica de este código", Q44: "Ayúdame a optimizar este código",
		Q45: "Analiza este informe de ventas y da recomendaciones", Q47: "Describe el contenido de esta imagen de paisaje",
		Q49: "Analiza estos datos CSV de ventas y encuentra tendencias", Q50: "Revisa si hay errores en este archivo de configuración",
		Q51: "Encuentra los bugs en este código JavaScript", Q52: "Dibuja un diagrama de flujo de registro e inicio de sesión",
	})
}

func italianPresetQuestions() []PresetQuestion {
	return buildPresetQuestions(i18nTexts{
		Q1: "Aiutami a scrivere un'email di richiesta ferie", Q2: "Spiega cos'è il machine learning", Q3: "Consigliami dei film di fantascienza", Q4: "Aiutami a creare un piano di allenamento",
		Q6: "Scrivi un quicksort in Python", Q7: "Spiega i principi di progettazione delle API REST", Q8: "Qual è la differenza tra Docker e le macchine virtuali?",
		Q9: "Scrivi una poesia sulla primavera", Q10: "Aiutami a trovare un nome per il mio nuovo prodotto",
		Q11: "Come configurare il backup automatico del NAS?", Q12: "Consiglia app NAS per uso domestico",
		Q13: "Cosa mangio stasera?", Q14: "Aiutami a pianificare una gita nel weekend", Q15: "Come prendere l'abitudine di alzarsi presto?",
		Q16: "Quali sono i metodi migliori per imparare l'inglese?", Q17: "Spiega come funziona la blockchain", Q18: "Cos'è il calcolo quantistico?",
		Q19: "Come prepararsi per un colloquio tecnico?", Q20: "Come posso migliorare la mia produttività lavorativa?",
		Q22: "Analizza la composizione e i colori di questa foto", Q23: "Riconosci il testo in questa immagine",
		Q42: "Interpreta i dati di questo grafico", Q43: "Analizza la struttura e la logica di questo codice", Q44: "Aiutami a ottimizzare questo codice",
		Q45: "Analizza questo report di vendite e dai suggerimenti", Q47: "Descrivi il contenuto di questa immagine paesaggistica",
		Q49: "Analizza questi dati CSV di vendita e trova i trend", Q50: "Controlla se ci sono errori in questo file di configurazione",
		Q51: "Trova i bug in questo codice JavaScript", Q52: "Disegna un diagramma di flusso per registrazione e login utente",
	})
}

func brazilianPortuguesePresetQuestions() []PresetQuestion {
	return buildPresetQuestions(i18nTexts{
		Q1: "Me ajude a escrever um e-mail pedindo folga", Q2: "Explique o que é aprendizado de máquina", Q3: "Recomende filmes de ficção científica", Q4: "Me ajude a montar um plano de exercícios",
		Q6: "Escreva um quicksort em Python", Q7: "Explique os princípios de design de APIs REST", Q8: "Qual a diferença entre Docker e máquinas virtuais?",
		Q9: "Escreva um poema sobre a primavera", Q10: "Me ajude a dar um nome pro meu novo produto",
		Q11: "Como configurar backup automático no NAS?", Q12: "Recomende apps de NAS para uso doméstico",
		Q13: "O que eu janto hoje?", Q14: "Me ajude a planejar uma viagem de fim de semana", Q15: "Como criar o hábito de acordar cedo?",
		Q16: "Quais são bons métodos para aprender inglês?", Q17: "Explique como funciona a blockchain", Q18: "O que é computação quântica?",
		Q19: "Como se preparar para uma entrevista técnica?", Q20: "Como posso melhorar minha produtividade no trabalho?",
		Q22: "Analise a composição e as cores desta foto", Q23: "Reconheça o texto nesta imagem",
		Q42: "Interprete os dados deste gráfico", Q43: "Analise a estrutura e a lógica deste código", Q44: "Me ajude a otimizar este código",
		Q45: "Analise este relatório de vendas e dê recomendações", Q47: "Descreva o conteúdo desta imagem de paisagem",
		Q49: "Analise estes dados CSV de vendas e encontre tendências", Q50: "Verifique se há erros neste arquivo de configuração",
		Q51: "Encontre os bugs neste código JavaScript", Q52: "Desenhe um fluxograma de cadastro e login de usuário",
	})
}

func russianPresetQuestions() []PresetQuestion {
	return buildPresetQuestions(i18nTexts{
		Q1: "Помоги написать письмо с просьбой об отпуске", Q2: "Объясни, что такое машинное обучение", Q3: "Посоветуй фантастические фильмы", Q4: "Помоги составить план тренировок",
		Q6: "Напиши быструю сортировку на Python", Q7: "Объясни принципы проектирования REST API", Q8: "В чём разница между Docker и виртуальными машинами?",
		Q9: "Напиши стихотворение о весне", Q10: "Помоги придумать название для нового продукта",
		Q11: "Как настроить автоматическое резервное копирование на NAS?", Q12: "Посоветуй приложения для домашнего NAS",
		Q13: "Что приготовить на ужин?", Q14: "Помоги спланировать поездку на выходные", Q15: "Как выработать привычку рано вставать?",
		Q16: "Как лучше всего учить английский?", Q17: "Объясни, как работает блокчейн", Q18: "Что такое квантовые вычисления?",
		Q19: "Как подготовиться к техническому собеседованию?", Q20: "Как повысить продуктивность на работе?",
		Q22: "Проанализируй композицию и цвета этого фото", Q23: "Распознай текст на этом изображении",
		Q42: "Интерпретируй данные на этом графике", Q43: "Проанализируй структуру и логику этого кода", Q44: "Помоги оптимизировать этот код",
		Q45: "Проанализируй этот отчёт о продажах и дай рекомендации", Q47: "Опиши содержание этого пейзажного изображения",
		Q49: "Проанализируй эти CSV-данные о продажах и найди тренды", Q50: "Проверь этот конфигурационный файл на ошибки",
		Q51: "Найди баги в этом JavaScript-коде", Q52: "Нарисуй блок-схему регистрации и входа пользователя",
	})
}

func dutchPresetQuestions() []PresetQuestion {
	return buildPresetQuestions(i18nTexts{
		Q1: "Help me een verlofaanvraag-e-mail te schrijven", Q2: "Leg uit wat machine learning is", Q3: "Beveel sciencefictionfilms aan", Q4: "Help me een fitnessplan te maken",
		Q6: "Schrijf een quicksort in Python", Q7: "Leg de ontwerpprincipes van REST API's uit", Q8: "Wat is het verschil tussen Docker en virtuele machines?",
		Q9: "Schrijf een gedicht over de lente", Q10: "Help me een naam te bedenken voor mijn nieuwe product",
		Q11: "Hoe stel ik automatische NAS-back-ups in?", Q12: "Beveel NAS-apps aan voor thuisgebruik",
		Q13: "Wat zal ik vanavond eten?", Q14: "Help me een weekenduitje te plannen", Q15: "Hoe maak ik er een gewoonte van om vroeg op te staan?",
		Q16: "Wat zijn goede methoden om Engels te leren?", Q17: "Leg uit hoe blockchain werkt", Q18: "Wat is kwantumcomputing?",
		Q19: "Hoe bereid ik me voor op een technisch sollicitatiegesprek?", Q20: "Hoe kan ik mijn werkefficiëntie verbeteren?",
		Q22: "Analyseer de compositie en kleuren van deze foto", Q23: "Herken de tekst in deze afbeelding",
		Q42: "Interpreteer de gegevens in deze grafiek", Q43: "Analyseer de structuur en logica van deze code", Q44: "Help me deze code te optimaliseren",
		Q45: "Analyseer dit verkooprapport en geef aanbevelingen", Q47: "Beschrijf de inhoud van deze landschapsfoto",
		Q49: "Analyseer deze CSV-verkoopgegevens en vind trends", Q50: "Controleer dit configuratiebestand op fouten",
		Q51: "Vind de bugs in deze JavaScript-code", Q52: "Teken een stroomdiagram voor gebruikersregistratie en login",
	})
}

func polishPresetQuestions() []PresetQuestion {
	return buildPresetQuestions(i18nTexts{
		Q1: "Pomóż mi napisać e-mail z prośbą o urlop", Q2: "Wyjaśnij, czym jest uczenie maszynowe", Q3: "Poleć filmy science fiction", Q4: "Pomóż mi ułożyć plan treningowy",
		Q6: "Napisz quicksort w Pythonie", Q7: "Wyjaśnij zasady projektowania REST API", Q8: "Jaka jest różnica między Dockerem a maszynami wirtualnymi?",
		Q9: "Napisz wiersz o wiośnie", Q10: "Pomóż mi wymyślić nazwę dla nowego produktu",
		Q11: "Jak skonfigurować automatyczny backup na NAS?", Q12: "Poleć aplikacje NAS do użytku domowego",
		Q13: "Co zjeść na kolację?", Q14: "Pomóż mi zaplanować weekendowy wyjazd", Q15: "Jak wyrobić nawyk wczesnego wstawania?",
		Q16: "Jakie są dobre metody nauki angielskiego?", Q17: "Wyjaśnij, jak działa blockchain", Q18: "Czym jest obliczenia kwantowe?",
		Q19: "Jak przygotować się do rozmowy technicznej?", Q20: "Jak poprawić efektywność w pracy?",
		Q22: "Przeanalizuj kompozycję i kolory tego zdjęcia", Q23: "Rozpoznaj tekst na tym obrazie",
		Q42: "Zinterpretuj dane na tym wykresie", Q43: "Przeanalizuj strukturę i logikę tego kodu", Q44: "Pomóż mi zoptymalizować ten kod",
		Q45: "Przeanalizuj ten raport sprzedaży i podaj rekomendacje", Q47: "Opisz zawartość tego zdjęcia krajobrazowego",
		Q49: "Przeanalizuj te dane CSV sprzedaży i znajdź trendy", Q50: "Sprawdź ten plik konfiguracyjny pod kątem błędów",
		Q51: "Znajdź błędy w tym kodzie JavaScript", Q52: "Narysuj schemat blokowy rejestracji i logowania użytkownika",
	})
}

func swedishPresetQuestions() []PresetQuestion {
	return buildPresetQuestions(i18nTexts{
		Q1: "Hjälp mig skriva ett ledighetsmail", Q2: "Förklara vad maskininlärning är", Q3: "Rekommendera science fiction-filmer", Q4: "Hjälp mig skapa ett träningsprogram",
		Q6: "Skriv en quicksort i Python", Q7: "Förklara designprinciperna för REST API:er", Q8: "Vad är skillnaden mellan Docker och virtuella maskiner?",
		Q9: "Skriv en dikt om våren", Q10: "Hjälp mig hitta ett namn till min nya produkt",
		Q11: "Hur ställer jag in automatisk NAS-backup?", Q12: "Rekommendera NAS-appar för hemmabruk",
		Q13: "Vad ska jag äta till middag?", Q14: "Hjälp mig planera en helgresa", Q15: "Hur skapar man en vana att gå upp tidigt?",
		Q16: "Vilka är bra metoder för att lära sig engelska?", Q17: "Förklara hur blockchain fungerar", Q18: "Vad är kvantdatorer?",
		Q19: "Hur förbereder man sig för en teknisk intervju?", Q20: "Hur kan jag förbättra min arbetseffektivitet?",
		Q22: "Analysera komposition och färger i detta foto", Q23: "Identifiera texten i denna bild",
		Q42: "Tolka data i detta diagram", Q43: "Analysera strukturen och logiken i denna kod", Q44: "Hjälp mig optimera denna kod",
		Q45: "Analysera denna försäljningsrapport och ge rekommendationer", Q47: "Beskriv innehållet i denna landskapsbild",
		Q49: "Analysera dessa CSV-försäljningsdata och hitta trender", Q50: "Kontrollera denna konfigurationsfil efter fel",
		Q51: "Hitta buggarna i denna JavaScript-kod", Q52: "Rita ett flödesschema för användarregistrering och inloggning",
	})
}

func danishPresetQuestions() []PresetQuestion {
	return buildPresetQuestions(i18nTexts{
		Q1: "Hjælp mig med at skrive en fraværsmail", Q2: "Forklar hvad maskinlæring er", Q3: "Anbefal science fiction-film", Q4: "Hjælp mig med at lave en træningsplan",
		Q6: "Skriv en quicksort i Python", Q7: "Forklar designprincipperne for REST API'er", Q8: "Hvad er forskellen mellem Docker og virtuelle maskiner?",
		Q9: "Skriv et digt om foråret", Q10: "Hjælp mig med at finde et navn til mit nye produkt",
		Q11: "Hvordan opsætter jeg automatisk NAS-backup?", Q12: "Anbefal NAS-apps til hjemmebrug",
		Q13: "Hvad skal jeg spise til aftensmad?", Q14: "Hjælp mig med at planlægge en weekendtur", Q15: "Hvordan vænner man sig til at stå tidligt op?",
		Q16: "Hvad er gode metoder til at lære engelsk?", Q17: "Forklar hvordan blockchain fungerer", Q18: "Hvad er kvantecomputing?",
		Q19: "Hvordan forbereder man sig til en teknisk samtale?", Q20: "Hvordan kan jeg forbedre min arbejdseffektivitet?",
		Q22: "Analysér komposition og farver i dette foto", Q23: "Genkend teksten i dette billede",
		Q42: "Fortolk dataene i dette diagram", Q43: "Analysér strukturen og logikken i denne kode", Q44: "Hjælp mig med at optimere denne kode",
		Q45: "Analysér denne salgsrapport og giv anbefalinger", Q47: "Beskriv indholdet af dette landskabsbillede",
		Q49: "Analysér disse CSV-salgsdata og find tendenser", Q50: "Tjek denne konfigurationsfil for fejl",
		Q51: "Find fejlene i denne JavaScript-kode", Q52: "Tegn et flowchart for brugerregistrering og login",
	})
}

func norwegianPresetQuestions() []PresetQuestion {
	return buildPresetQuestions(i18nTexts{
		Q1: "Hjelp meg å skrive en fraværsmelding", Q2: "Forklar hva maskinlæring er", Q3: "Anbefal science fiction-filmer", Q4: "Hjelp meg å lage en treningsplan",
		Q6: "Skriv en quicksort i Python", Q7: "Forklar designprinsippene for REST API-er", Q8: "Hva er forskjellen mellom Docker og virtuelle maskiner?",
		Q9: "Skriv et dikt om våren", Q10: "Hjelp meg å finne et navn til det nye produktet mitt",
		Q11: "Hvordan setter jeg opp automatisk NAS-backup?", Q12: "Anbefal NAS-apper for hjemmebruk",
		Q13: "Hva skal jeg spise til middag?", Q14: "Hjelp meg å planlegge en helgetur", Q15: "Hvordan venner man seg til å stå opp tidlig?",
		Q16: "Hva er gode metoder for å lære engelsk?", Q17: "Forklar hvordan blokkjede fungerer", Q18: "Hva er kvantedatabehandling?",
		Q19: "Hvordan forbereder man seg til et teknisk intervju?", Q20: "Hvordan kan jeg forbedre arbeidseffektiviteten min?",
		Q22: "Analyser komposisjon og farger i dette bildet", Q23: "Gjenkjenn teksten i dette bildet",
		Q42: "Tolk dataene i dette diagrammet", Q43: "Analyser strukturen og logikken i denne koden", Q44: "Hjelp meg å optimalisere denne koden",
		Q45: "Analyser denne salgsrapporten og gi anbefalinger", Q47: "Beskriv innholdet i dette landskapsbildet",
		Q49: "Analyser disse CSV-salgsdataene og finn trender", Q50: "Sjekk denne konfigurasjonsfilen for feil",
		Q51: "Finn feilene i denne JavaScript-koden", Q52: "Tegn et flytskjema for brukerregistrering og innlogging",
	})
}

func czechPresetQuestions() []PresetQuestion {
	return buildPresetQuestions(i18nTexts{
		Q1: "Pomoz mi napsat e-mail s žádostí o dovolenou", Q2: "Vysvětli, co je strojové učení", Q3: "Doporuč sci-fi filmy", Q4: "Pomoz mi sestavit tréninkový plán",
		Q6: "Napiš quicksort v Pythonu", Q7: "Vysvětli principy návrhu REST API", Q8: "Jaký je rozdíl mezi Dockerem a virtuálními stroji?",
		Q9: "Napiš báseň o jaru", Q10: "Pomoz mi vymyslet název pro nový produkt",
		Q11: "Jak nastavit automatické zálohování na NAS?", Q12: "Doporuč NAS aplikace pro domácí použití",
		Q13: "Co si dát k večeři?", Q14: "Pomoz mi naplánovat víkendový výlet", Q15: "Jak si zvyknout na brzké vstávání?",
		Q16: "Jaké jsou dobré metody pro učení angličtiny?", Q17: "Vysvětli, jak funguje blockchain", Q18: "Co je kvantové počítání?",
		Q19: "Jak se připravit na technický pohovor?", Q20: "Jak zlepšit pracovní efektivitu?",
		Q22: "Analyzuj kompozici a barvy této fotografie", Q23: "Rozpoznej text v tomto obrázku",
		Q42: "Interpretuj data v tomto grafu", Q43: "Analyzuj strukturu a logiku tohoto kódu", Q44: "Pomoz mi optimalizovat tento kód",
		Q45: "Analyzuj tuto zprávu o prodeji a dej doporučení", Q47: "Popiš obsah tohoto krajinného snímku",
		Q49: "Analyzuj tato CSV data o prodeji a najdi trendy", Q50: "Zkontroluj tento konfigurační soubor na chyby",
		Q51: "Najdi chyby v tomto JavaScript kódu", Q52: "Nakresli vývojový diagram registrace a přihlášení uživatele",
	})
}

func slovakPresetQuestions() []PresetQuestion {
	return buildPresetQuestions(i18nTexts{
		Q1: "Pomôž mi napísať e-mail so žiadosťou o dovolenku", Q2: "Vysvetli, čo je strojové učenie", Q3: "Odporuč sci-fi filmy", Q4: "Pomôž mi zostaviť tréningový plán",
		Q6: "Napíš quicksort v Pythone", Q7: "Vysvetli princípy návrhu REST API", Q8: "Aký je rozdiel medzi Dockerom a virtuálnymi strojmi?",
		Q9: "Napíš báseň o jari", Q10: "Pomôž mi vymyslieť názov pre nový produkt",
		Q11: "Ako nastaviť automatické zálohovanie na NAS?", Q12: "Odporuč NAS aplikácie pre domáce použitie",
		Q13: "Čo si dať na večeru?", Q14: "Pomôž mi naplánovať víkendový výlet", Q15: "Ako si zvyknúť na skoré vstávanie?",
		Q16: "Aké sú dobré metódy na učenie angličtiny?", Q17: "Vysvetli, ako funguje blockchain", Q18: "Čo je kvantové počítanie?",
		Q19: "Ako sa pripraviť na technický pohovor?", Q20: "Ako zlepšiť pracovnú efektivitu?",
		Q22: "Analyzuj kompozíciu a farby tejto fotografie", Q23: "Rozpoznaj text v tomto obrázku",
		Q42: "Interpretuj údaje v tomto grafe", Q43: "Analyzuj štruktúru a logiku tohto kódu", Q44: "Pomôž mi optimalizovať tento kód",
		Q45: "Analyzuj túto správu o predaji a daj odporúčania", Q47: "Opíš obsah tohto krajinného snímku",
		Q49: "Analyzuj tieto CSV údaje o predaji a nájdi trendy", Q50: "Skontroluj tento konfiguračný súbor na chyby",
		Q51: "Nájdi chyby v tomto JavaScript kóde", Q52: "Nakresli vývojový diagram registrácie a prihlásenia používateľa",
	})
}

func hungarianPresetQuestions() []PresetQuestion {
	return buildPresetQuestions(i18nTexts{
		Q1: "Segíts szabadságkérő e-mailt írni", Q2: "Magyarázd el, mi a gépi tanulás", Q3: "Ajánlj sci-fi filmeket", Q4: "Segíts edzéstervet készíteni",
		Q6: "Írj egy quicksortot Pythonban", Q7: "Magyarázd el a REST API tervezési elveit", Q8: "Mi a különbség a Docker és a virtuális gépek között?",
		Q9: "Írj egy verset a tavaszról", Q10: "Segíts nevet találni az új termékemnek",
		Q11: "Hogyan állítsam be az automatikus NAS-mentést?", Q12: "Ajánlj NAS-alkalmazásokat otthoni használatra",
		Q13: "Mit egyek ma vacsorára?", Q14: "Segíts hétvégi kirándulást tervezni", Q15: "Hogyan szoktassam magam a korai keléshez?",
		Q16: "Milyen jó módszerek vannak az angoltanulásra?", Q17: "Magyarázd el, hogyan működik a blokklánc", Q18: "Mi a kvantumszámítás?",
		Q19: "Hogyan készüljek fel egy technikai interjúra?", Q20: "Hogyan javíthatom a munkahatékonyságomat?",
		Q22: "Elemezd a fotó kompozícióját és színeit", Q23: "Ismerd fel a szöveget ezen a képen",
		Q42: "Értelmezd az adatokat ezen a diagramon", Q43: "Elemezd ennek a kódnak a szerkezetét és logikáját", Q44: "Segíts optimalizálni ezt a kódot",
		Q45: "Elemezd ezt az értékesítési jelentést és adj javaslatokat", Q47: "Írd le ennek a tájképnek a tartalmát",
		Q49: "Elemezd ezeket a CSV értékesítési adatokat és találj trendeket", Q50: "Ellenőrizd ezt a konfigurációs fájlt hibákra",
		Q51: "Találd meg a hibákat ebben a JavaScript kódban", Q52: "Rajzolj folyamatábrát a felhasználói regisztrációhoz és bejelentkezéshez",
	})
}

func romanianPresetQuestions() []PresetQuestion {
	return buildPresetQuestions(i18nTexts{
		Q1: "Ajută-mă să scriu un e-mail de cerere de concediu", Q2: "Explică ce este învățarea automată", Q3: "Recomandă filme science fiction", Q4: "Ajută-mă să creez un plan de fitness",
		Q6: "Scrie un quicksort în Python", Q7: "Explică principiile de design ale API-urilor REST", Q8: "Care este diferența dintre Docker și mașinile virtuale?",
		Q9: "Scrie o poezie despre primăvară", Q10: "Ajută-mă să găsesc un nume pentru noul meu produs",
		Q11: "Cum configurez backup-ul automat pe NAS?", Q12: "Recomandă aplicații NAS pentru uz casnic",
		Q13: "Ce mănânc la cină?", Q14: "Ajută-mă să planific o excursie de weekend", Q15: "Cum îmi formez obiceiul de a mă trezi devreme?",
		Q16: "Care sunt metode bune pentru a învăța engleză?", Q17: "Explică cum funcționează blockchain-ul", Q18: "Ce este calculul cuantic?",
		Q19: "Cum mă pregătesc pentru un interviu tehnic?", Q20: "Cum îmi pot îmbunătăți eficiența la muncă?",
		Q22: "Analizează compoziția și culorile acestei fotografii", Q23: "Recunoaște textul din această imagine",
		Q42: "Interpretează datele din acest grafic", Q43: "Analizează structura și logica acestui cod", Q44: "Ajută-mă să optimizez acest cod",
		Q45: "Analizează acest raport de vânzări și oferă recomandări", Q47: "Descrie conținutul acestei imagini cu peisaj",
		Q49: "Analizează aceste date CSV de vânzări și găsește tendințe", Q50: "Verifică acest fișier de configurare pentru erori",
		Q51: "Găsește bug-urile din acest cod JavaScript", Q52: "Desenează o diagramă de flux pentru înregistrare și autentificare",
	})
}

func croatianPresetQuestions() []PresetQuestion {
	return buildPresetQuestions(i18nTexts{
		Q1: "Pomozi mi napisati e-mail za zahtjev za dopust", Q2: "Objasni što je strojno učenje", Q3: "Preporuči znanstvenofantastične filmove", Q4: "Pomozi mi napraviti plan vježbanja",
		Q6: "Napiši quicksort u Pythonu", Q7: "Objasni principe dizajna REST API-ja", Q8: "Koja je razlika između Dockera i virtualnih strojeva?",
		Q9: "Napiši pjesmu o proljeću", Q10: "Pomozi mi smisliti ime za novi proizvod",
		Q11: "Kako postaviti automatsko sigurnosno kopiranje na NAS?", Q12: "Preporuči NAS aplikacije za kućnu upotrebu",
		Q13: "Što da jedem za večeru?", Q14: "Pomozi mi isplanirati vikend izlet", Q15: "Kako steći naviku ranog ustajanja?",
		Q16: "Koje su dobre metode za učenje engleskog?", Q17: "Objasni kako funkcionira blockchain", Q18: "Što je kvantno računalstvo?",
		Q19: "Kako se pripremiti za tehnički intervju?", Q20: "Kako poboljšati radnu učinkovitost?",
		Q22: "Analiziraj kompoziciju i boje ove fotografije", Q23: "Prepoznaj tekst na ovoj slici",
		Q42: "Protumači podatke na ovom grafikonu", Q43: "Analiziraj strukturu i logiku ovog koda", Q44: "Pomozi mi optimizirati ovaj kod",
		Q45: "Analiziraj ovo izvješće o prodaji i daj preporuke", Q47: "Opiši sadržaj ove pejzažne slike",
		Q49: "Analiziraj ove CSV podatke o prodaji i pronađi trendove", Q50: "Provjeri ovu konfiguracijsku datoteku za greške",
		Q51: "Pronađi greške u ovom JavaScript kodu", Q52: "Nacrtaj dijagram toka za registraciju i prijavu korisnika",
	})
}

func greekPresetQuestions() []PresetQuestion {
	return buildPresetQuestions(i18nTexts{
		Q1: "Βοήθησέ με να γράψω ένα email αίτησης άδειας", Q2: "Εξήγησε τι είναι η μηχανική μάθηση", Q3: "Πρότεινε ταινίες επιστημονικής φαντασίας", Q4: "Βοήθησέ με να φτιάξω πρόγραμμα γυμναστικής",
		Q6: "Γράψε μια quicksort σε Python", Q7: "Εξήγησε τις αρχές σχεδιασμού REST API", Q8: "Ποια είναι η διαφορά μεταξύ Docker και εικονικών μηχανών;",
		Q9: "Γράψε ένα ποίημα για την άνοιξη", Q10: "Βοήθησέ με να βρω όνομα για το νέο μου προϊόν",
		Q11: "Πώς ρυθμίζω αυτόματο backup στο NAS;", Q12: "Πρότεινε εφαρμογές NAS για οικιακή χρήση",
		Q13: "Τι να φάω απόψε;", Q14: "Βοήθησέ με να σχεδιάσω μια εκδρομή Σαββατοκύριακου", Q15: "Πώς να αποκτήσω τη συνήθεια του πρωινού ξυπνήματος;",
		Q16: "Ποιες είναι καλές μέθοδοι για εκμάθηση αγγλικών;", Q17: "Εξήγησε πώς λειτουργεί το blockchain", Q18: "Τι είναι η κβαντική υπολογιστική;",
		Q19: "Πώς να προετοιμαστώ για τεχνική συνέντευξη;", Q20: "Πώς μπορώ να βελτιώσω την αποδοτικότητά μου στη δουλειά;",
		Q22: "Ανάλυσε τη σύνθεση και τα χρώματα αυτής της φωτογραφίας", Q23: "Αναγνώρισε το κείμενο σε αυτή την εικόνα",
		Q42: "Ερμήνευσε τα δεδομένα σε αυτό το γράφημα", Q43: "Ανάλυσε τη δομή και τη λογική αυτού του κώδικα", Q44: "Βοήθησέ με να βελτιστοποιήσω αυτόν τον κώδικα",
		Q45: "Ανάλυσε αυτή την αναφορά πωλήσεων και δώσε συστάσεις", Q47: "Περίγραψε το περιεχόμενο αυτής της εικόνας τοπίου",
		Q49: "Ανάλυσε αυτά τα CSV δεδομένα πωλήσεων και βρες τάσεις", Q50: "Έλεγξε αυτό το αρχείο ρυθμίσεων για σφάλματα",
		Q51: "Βρες τα bugs σε αυτόν τον κώδικα JavaScript", Q52: "Σχεδίασε ένα διάγραμμα ροής για εγγραφή και σύνδεση χρήστη",
	})
}

func catalanPresetQuestions() []PresetQuestion {
	return buildPresetQuestions(i18nTexts{
		Q1: "Ajuda'm a escriure un correu de sol·licitud de vacances", Q2: "Explica què és l'aprenentatge automàtic", Q3: "Recomana pel·lícules de ciència-ficció", Q4: "Ajuda'm a crear un pla d'entrenament",
		Q6: "Escriu un quicksort en Python", Q7: "Explica els principis de disseny de les API REST", Q8: "Quina diferència hi ha entre Docker i les màquines virtuals?",
		Q9: "Escriu un poema sobre la primavera", Q10: "Ajuda'm a trobar un nom per al meu nou producte",
		Q11: "Com configuro la còpia de seguretat automàtica al NAS?", Q12: "Recomana aplicacions NAS per a ús domèstic",
		Q13: "Què sopo avui?", Q14: "Ajuda'm a planificar una escapada de cap de setmana", Q15: "Com agafar l'hàbit de llevar-se d'hora?",
		Q16: "Quins són bons mètodes per aprendre anglès?", Q17: "Explica com funciona la blockchain", Q18: "Què és la computació quàntica?",
		Q19: "Com preparar-se per a una entrevista tècnica?", Q20: "Com puc millorar la meva productivitat laboral?",
		Q22: "Analitza la composició i els colors d'aquesta foto", Q23: "Reconeix el text d'aquesta imatge",
		Q42: "Interpreta les dades d'aquest gràfic", Q43: "Analitza l'estructura i la lògica d'aquest codi", Q44: "Ajuda'm a optimitzar aquest codi",
		Q45: "Analitza aquest informe de vendes i dona recomanacions", Q47: "Descriu el contingut d'aquesta imatge de paisatge",
		Q49: "Analitza aquestes dades CSV de vendes i troba tendències", Q50: "Comprova si hi ha errors en aquest fitxer de configuració",
		Q51: "Troba els errors en aquest codi JavaScript", Q52: "Dibuixa un diagrama de flux per al registre i l'inici de sessió",
	})
}

func irishPresetQuestions() []PresetQuestion {
	return buildPresetQuestions(i18nTexts{
		Q1: "Cabhraigh liom ríomhphost saoire a scríobh", Q2: "Mínigh cad is foghlaim meaisín ann", Q3: "Mol scannáin ficsean eolaíochta", Q4: "Cabhraigh liom plean aclaíochta a dhéanamh",
		Q6: "Scríobh quicksort i Python", Q7: "Mínigh prionsabail deartha REST API", Q8: "Cad é an difríocht idir Docker agus meaisíní fíorúla?",
		Q9: "Scríobh dán faoin earrach", Q10: "Cabhraigh liom ainm a fháil do mo tháirge nua",
		Q11: "Conas cúltaca uathoibríoch NAS a shocrú?", Q12: "Mol aipeanna NAS le haghaidh úsáid bhaile",
		Q13: "Cad a íosfaidh mé don dinnéar?", Q14: "Cabhraigh liom turas deireadh seachtaine a phleanáil", Q15: "Conas an nós éirí go luath a chleachtadh?",
		Q16: "Cad iad na modhanna maithe chun Béarla a fhoghlaim?", Q17: "Mínigh conas a oibríonn blocshlabhra", Q18: "Cad é ríomhaireacht chandamach?",
		Q19: "Conas ullmhú d'agallamh teicniúil?", Q20: "Conas is féidir liom m'éifeachtúlacht oibre a fheabhsú?",
		Q22: "Déan anailís ar chomhdhéanamh agus dathanna na grianghraife seo", Q23: "Aithin an téacs san íomhá seo",
		Q42: "Léirmhínigh na sonraí sa chairt seo", Q43: "Déan anailís ar struchtúr agus loighic an chóid seo", Q44: "Cabhraigh liom an cód seo a bharrfheabhsú",
		Q45: "Déan anailís ar an tuarascáil díolacháin seo agus tabhair moltaí", Q47: "Déan cur síos ar ábhar na híomhá tírdhreacha seo",
		Q49: "Déan anailís ar na sonraí CSV díolacháin seo agus aimsigh treochtaí", Q50: "Seiceáil an comhad cumraíochta seo le haghaidh earráidí",
		Q51: "Aimsigh na fabhtanna sa chód JavaScript seo", Q52: "Tarraing sreabhchairt do chlárú agus logáil isteach úsáideora",
	})
}

func malayalamPresetQuestions() []PresetQuestion {
	return buildPresetQuestions(i18nTexts{
		Q1: "ഒരു അവധി അഭ്യർത്ഥന ഇമെയിൽ എഴുതാൻ സഹായിക്കൂ", Q2: "മെഷീൻ ലേണിംഗ് എന്താണെന്ന് വിശദീകരിക്കൂ", Q3: "സയൻസ് ഫിക്ഷൻ സിനിമകൾ ശുപാർശ ചെയ്യൂ", Q4: "ഒരു ഫിറ്റ്നസ് പ്ലാൻ ഉണ്ടാക്കാൻ സഹായിക്കൂ",
		Q6: "Python-ൽ ഒരു quicksort എഴുതൂ", Q7: "REST API ഡിസൈൻ തത്വങ്ങൾ വിശദീകരിക്കൂ", Q8: "Docker-ഉം വെർച്വൽ മെഷീനുകളും തമ്മിലുള്ള വ്യത്യാസം എന്താണ്?",
		Q9: "വസന്തത്തെക്കുറിച്ച് ഒരു കവിത എഴുതൂ", Q10: "എന്റെ പുതിയ ഉൽപ്പന്നത്തിന് ഒരു പേര് കണ്ടെത്താൻ സഹായിക്കൂ",
		Q11: "NAS-ൽ ഓട്ടോമാറ്റിക് ബാക്കപ്പ് എങ്ങനെ സെറ്റ് ചെയ്യാം?", Q12: "വീട്ടിലെ ഉപയോഗത്തിന് NAS ആപ്പുകൾ ശുപാർശ ചെയ്യൂ",
		Q13: "ഇന്ന് രാത്രി എന്ത് കഴിക്കണം?", Q14: "ഒരു വാരാന്ത്യ യാത്ര ആസൂത്രണം ചെയ്യാൻ സഹായിക്കൂ", Q15: "നേരത്തെ എഴുന്നേൽക്കുന്ന ശീലം എങ്ങനെ ഉണ്ടാക്കാം?",
		Q16: "ഇംഗ്ലീഷ് പഠിക്കാൻ നല്ല മാർഗങ്ങൾ എന്തൊക്കെ?", Q17: "ബ്ലോക്ക്ചെയിൻ എങ്ങനെ പ്രവർത്തിക്കുന്നുവെന്ന് വിശദീകരിക്കൂ", Q18: "ക്വാണ്ടം കമ്പ്യൂട്ടിംഗ് എന്താണ്?",
		Q19: "ഒരു ടെക്നിക്കൽ ഇന്റർവ്യൂവിന് എങ്ങനെ തയ്യാറെടുക്കാം?", Q20: "ജോലിയിലെ കാര്യക്ഷമത എങ്ങനെ മെച്ചപ്പെടുത്താം?",
		Q22: "ഈ ഫോട്ടോയുടെ കോമ്പോസിഷനും നിറങ്ങളും വിശകലനം ചെയ്യൂ", Q23: "ഈ ചിത്രത്തിലെ ടെക്സ്റ്റ് തിരിച്ചറിയൂ",
		Q42: "ഈ ചാർട്ടിലെ ഡാറ്റ വ്യാഖ്യാനിക്കൂ", Q43: "ഈ കോഡിന്റെ ഘടനയും ലോജിക്കും വിശകലനം ചെയ്യൂ", Q44: "ഈ കോഡ് ഒപ്റ്റിമൈസ് ചെയ്യാൻ സഹായിക്കൂ",
		Q45: "ഈ സെയിൽസ് റിപ്പോർട്ട് വിശകലനം ചെയ്ത് ശുപാർശകൾ നൽകൂ", Q47: "ഈ ലാൻഡ്സ്കേപ്പ് ചിത്രത്തിന്റെ ഉള്ളടക്കം വിവരിക്കൂ",
		Q49: "ഈ CSV സെയിൽസ് ഡാറ്റ വിശകലനം ചെയ്ത് ട്രെൻഡുകൾ കണ്ടെത്തൂ", Q50: "ഈ കോൺഫിഗറേഷൻ ഫയലിൽ പിശകുകൾ ഉണ്ടോ പരിശോധിക്കൂ",
		Q51: "ഈ JavaScript കോഡിലെ ബഗുകൾ കണ്ടെത്തൂ", Q52: "ഉപയോക്തൃ രജിസ്ട്രേഷനും ലോഗിനും ഫ്ലോചാർട്ട് വരയ്ക്കൂ",
	})
}

func europeanPortuguesePresetQuestions() []PresetQuestion {
	return buildPresetQuestions(i18nTexts{
		Q1: "Ajuda-me a escrever um e-mail a pedir férias", Q2: "Explica o que é aprendizagem automática", Q3: "Recomenda filmes de ficção científica", Q4: "Ajuda-me a criar um plano de treino",
		Q6: "Escreve um quicksort em Python", Q7: "Explica os princípios de design de APIs REST", Q8: "Qual é a diferença entre Docker e máquinas virtuais?",
		Q9: "Escreve um poema sobre a primavera", Q10: "Ajuda-me a encontrar um nome para o meu novo produto",
		Q11: "Como configurar cópias de segurança automáticas no NAS?", Q12: "Recomenda aplicações NAS para uso doméstico",
		Q13: "O que é que janto hoje?", Q14: "Ajuda-me a planear uma escapadela de fim de semana", Q15: "Como criar o hábito de acordar cedo?",
		Q16: "Quais são bons métodos para aprender inglês?", Q17: "Explica como funciona a blockchain", Q18: "O que é computação quântica?",
		Q19: "Como preparar-se para uma entrevista técnica?", Q20: "Como posso melhorar a minha produtividade no trabalho?",
		Q22: "Analisa a composição e as cores desta fotografia", Q23: "Reconhece o texto nesta imagem",
		Q42: "Interpreta os dados deste gráfico", Q43: "Analisa a estrutura e a lógica deste código", Q44: "Ajuda-me a otimizar este código",
		Q45: "Analisa este relatório de vendas e dá recomendações", Q47: "Descreve o conteúdo desta imagem de paisagem",
		Q49: "Analisa estes dados CSV de vendas e encontra tendências", Q50: "Verifica se há erros neste ficheiro de configuração",
		Q51: "Encontra os bugs neste código JavaScript", Q52: "Desenha um fluxograma de registo e início de sessão do utilizador",
	})
}
