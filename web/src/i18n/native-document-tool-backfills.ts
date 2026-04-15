import type { LocaleKey } from "./locale-catalog"

export type NativeDocumentToolLocaleTerms = {
  docxName: string
  docxDescription: string
  xlsxName: string
  xlsxDescription: string
  pptxName: string
  pptxDescription: string
  pdfName: string
  pdfDescription: string
}

const nativeDocumentToolNames = {
  docxName: 'DOCX',
  xlsxName: 'XLSX',
  pptxName: 'PPTX',
  pdfName: 'PDF',
} as const

const nativeDocumentToolBackfills: Record<LocaleKey, NativeDocumentToolLocaleTerms> = {
  "ca-ES": {
    ...nativeDocumentToolNames,
    docxDescription: "Fes-lo servir quan la tasca se centri en un fitxer .docx de l'espai de treball i necessiti un document natiu d'estil Word per redactar, omplir plantilles, editar marcadors o validar-lo.",
    xlsxDescription: "Fes-lo servir quan la tasca se centri en un fitxer .xlsx de l'espai de treball i necessiti un full de càlcul natiu per a taules, fórmules, edicions de fulls, anàlisi o validació.",
    pptxDescription: "Fes-lo servir quan la tasca se centri en un fitxer .pptx de l'espai de treball i necessiti una presentació nativa per a diapositives editables, canvis de disseny o actualitzacions de gràfics.",
    pdfDescription: "Fes-lo servir quan la tasca se centri en un fitxer .pdf de l'espai de treball i necessiti lectura nativa de PDF, emplenament de formularis, sortida imprimible o reformatació que preservi el disseny.",
  },
  "cs-CZ": {
    ...nativeDocumentToolNames,
    docxDescription: "Použijte jej, když se úkol týká souboru .docx v pracovním prostoru a potřebuje nativní dokument ve stylu Wordu pro psaní, vyplnění šablony, úpravy zástupných symbolů nebo validaci.",
    xlsxDescription: "Použijte jej, když se úkol týká souboru .xlsx v pracovním prostoru a potřebuje nativní tabulku pro tabulky, vzorce, úpravy listů, analýzu nebo validaci.",
    pptxDescription: "Použijte jej, když se úkol týká souboru .pptx v pracovním prostoru a potřebuje nativní prezentaci pro upravovatelné snímky, změny rozvržení nebo aktualizace grafů.",
    pdfDescription: "Použijte jej, když se úkol týká souboru .pdf v pracovním prostoru a potřebuje nativní čtení PDF, vyplňování formulářů, tiskový výstup nebo přeformátování se zachováním rozvržení.",
  },
  "da-DK": {
    ...nativeDocumentToolNames,
    docxDescription: "Brug det, når opgaven handler om en .docx-fil i arbejdsområdet og kræver et indbygget Word-lignende dokument til skrivning, skabelonudfyldning, pladsholderredigering eller validering.",
    xlsxDescription: "Brug det, når opgaven handler om en .xlsx-fil i arbejdsområdet og kræver et indbygget regneark til tabeller, formler, arkredigering, analyse eller validering.",
    pptxDescription: "Brug det, når opgaven handler om en .pptx-fil i arbejdsområdet og kræver en indbygget præsentation til redigerbare slides, layoutændringer eller diagramopdateringer.",
    pdfDescription: "Brug det, når opgaven handler om en .pdf-fil i arbejdsområdet og kræver PDF-native læsning, formularudfyldning, printklar output eller omformatering, der bevarer layoutet.",
  },
  "de-DE": {
    ...nativeDocumentToolNames,
    docxDescription: "Verwende es, wenn sich die Aufgabe auf eine .docx-Datei im Arbeitsbereich konzentriert und ein natives Word-ähnliches Dokument zum Schreiben, Ausfüllen von Vorlagen, Bearbeiten von Platzhaltern oder Validieren benötigt.",
    xlsxDescription: "Verwende es, wenn sich die Aufgabe auf eine .xlsx-Datei im Arbeitsbereich konzentriert und eine native Tabellenkalkulation für Tabellen, Formeln, Blattbearbeitungen, Analysen oder Validierung benötigt.",
    pptxDescription: "Verwende es, wenn sich die Aufgabe auf eine .pptx-Datei im Arbeitsbereich konzentriert und eine native Präsentation für bearbeitbare Folien, Layoutänderungen oder Diagramm-Updates benötigt.",
    pdfDescription: "Verwende es, wenn sich die Aufgabe auf eine .pdf-Datei im Arbeitsbereich konzentriert und natives PDF-Lesen, Formularausfüllung, druckfertige Ausgabe oder layouttreue Neuformatierung benötigt.",
  },
  "el-GR": {
    ...nativeDocumentToolNames,
    docxDescription: "Χρησιμοποίησέ το όταν η εργασία επικεντρώνεται σε αρχείο .docx του χώρου εργασίας και χρειάζεται εγγενές έγγραφο τύπου Word για σύνταξη, συμπλήρωση προτύπων, επεξεργασία placeholders ή επικύρωση.",
    xlsxDescription: "Χρησιμοποίησέ το όταν η εργασία επικεντρώνεται σε αρχείο .xlsx του χώρου εργασίας και χρειάζεται εγγενές υπολογιστικό φύλλο για πίνακες, τύπους, επεξεργασίες φύλλων, ανάλυση ή επικύρωση.",
    pptxDescription: "Χρησιμοποίησέ το όταν η εργασία επικεντρώνεται σε αρχείο .pptx του χώρου εργασίας και χρειάζεται εγγενή παρουσίαση για επεξεργάσιμες διαφάνειες, αλλαγές διάταξης ή ενημερώσεις γραφημάτων.",
    pdfDescription: "Χρησιμοποίησέ το όταν η εργασία επικεντρώνεται σε αρχείο .pdf του χώρου εργασίας και χρειάζεται εγγενή ανάγνωση PDF, συμπλήρωση φορμών, εκτυπώσιμο αποτέλεσμα ή αναμορφοποίηση που διατηρεί τη διάταξη.",
  },
  "en-GB": {
    ...nativeDocumentToolNames,
    docxDescription: "Use when the task centres on a workspace .docx file and needs a native Word-style document for writing, template filling, placeholder edits, or validation.",
    xlsxDescription: "Use when the task centres on a workspace .xlsx file and needs a native spreadsheet for tables, formulas, sheet edits, analysis, or validation.",
    pptxDescription: "Use when the task centres on a workspace .pptx file and needs a native slide deck for editable slides, layout changes, or chart updates.",
    pdfDescription: "Use when the task centres on a workspace .pdf file and needs PDF-native reading, form filling, printable output, or layout-preserving reformatting.",
  },
  "en-US": {
    ...nativeDocumentToolNames,
    docxDescription: "Use when the task centers on a workspace .docx file and needs a native Word-style document for writing, template filling, placeholder edits, or validation.",
    xlsxDescription: "Use when the task centers on a workspace .xlsx file and needs a native spreadsheet for tables, formulas, sheet edits, analysis, or validation.",
    pptxDescription: "Use when the task centers on a workspace .pptx file and needs a native slide deck for editable slides, layout changes, or chart updates.",
    pdfDescription: "Use when the task centers on a workspace .pdf file and needs PDF-native reading, form filling, printable output, or layout-preserving reformatting.",
  },
  "es-ES": {
    ...nativeDocumentToolNames,
    docxDescription: "Úsalo cuando la tarea se centre en un archivo .docx del espacio de trabajo y necesite un documento nativo de estilo Word para redactar, rellenar plantillas, editar marcadores o validarlo.",
    xlsxDescription: "Úsalo cuando la tarea se centre en un archivo .xlsx del espacio de trabajo y necesite una hoja de cálculo nativa para tablas, fórmulas, edición de hojas, análisis o validación.",
    pptxDescription: "Úsalo cuando la tarea se centre en un archivo .pptx del espacio de trabajo y necesite una presentación nativa para diapositivas editables, cambios de diseño o actualizaciones de gráficos.",
    pdfDescription: "Úsalo cuando la tarea se centre en un archivo .pdf del espacio de trabajo y necesite lectura nativa de PDF, rellenado de formularios, salida lista para imprimir o reformateo que preserve el diseño.",
  },
  "fr-FR": {
    ...nativeDocumentToolNames,
    docxDescription: "Utilisez-le lorsque la tâche porte sur un fichier .docx de l'espace de travail et qu'elle nécessite un document natif de type Word pour rédiger, remplir un modèle, modifier des espaces réservés ou valider le fichier.",
    xlsxDescription: "Utilisez-le lorsque la tâche porte sur un fichier .xlsx de l'espace de travail et qu'elle nécessite un tableur natif pour des tableaux, des formules, des modifications de feuilles, de l'analyse ou de la validation.",
    pptxDescription: "Utilisez-le lorsque la tâche porte sur un fichier .pptx de l'espace de travail et qu'elle nécessite une présentation native pour des diapositives modifiables, des changements de mise en page ou des mises à jour de graphiques.",
    pdfDescription: "Utilisez-le lorsque la tâche porte sur un fichier .pdf de l'espace de travail et qu'elle nécessite une lecture PDF native, le remplissage de formulaires, une sortie imprimable ou un reformatage qui préserve la mise en page.",
  },
  "ga-IE": {
    ...nativeDocumentToolNames,
    docxDescription: "Úsáid é nuair a dhíríonn an tasc ar chomhad .docx sa spás oibre agus nuair is gá doiciméad dúchasach cosúil le Word chun scríobh, teimpléid a líonadh, ionadchoinneálaithe a chur in eagar nó bailíochtú a dhéanamh.",
    xlsxDescription: "Úsáid é nuair a dhíríonn an tasc ar chomhad .xlsx sa spás oibre agus nuair is gá scarbhileog dhúchasach do tháblaí, foirmlí, eagarthóireacht bileog, anailís nó bailíochtú.",
    pptxDescription: "Úsáid é nuair a dhíríonn an tasc ar chomhad .pptx sa spás oibre agus nuair is gá cur i láthair dúchasach do shleamhnáin in-eagarthóireachta, athruithe leagain amach nó nuashonruithe cairte.",
    pdfDescription: "Úsáid é nuair a dhíríonn an tasc ar chomhad .pdf sa spás oibre agus nuair is gá léamh PDF dúchasach, líonadh foirmeacha, aschur inphriontáilte nó athfhormáidiú a chaomhnaíonn an leagan amach.",
  },
  "hr-HR": {
    ...nativeDocumentToolNames,
    docxDescription: "Koristi ga kada je zadatak usmjeren na .docx datoteku u radnom prostoru i treba izvorni dokument nalik Wordu za pisanje, popunjavanje predložaka, uređivanje rezerviranih mjesta ili provjeru valjanosti.",
    xlsxDescription: "Koristi ga kada je zadatak usmjeren na .xlsx datoteku u radnom prostoru i treba izvornu proračunsku tablicu za tablice, formule, uređivanje listova, analizu ili provjeru valjanosti.",
    pptxDescription: "Koristi ga kada je zadatak usmjeren na .pptx datoteku u radnom prostoru i treba izvornu prezentaciju za uređive slajdove, promjene rasporeda ili ažuriranja grafikona.",
    pdfDescription: "Koristi ga kada je zadatak usmjeren na .pdf datoteku u radnom prostoru i treba izvorno čitanje PDF-a, ispunjavanje obrazaca, ispisni izlaz ili preoblikovanje koje čuva raspored.",
  },
  "hu-HU": {
    ...nativeDocumentToolNames,
    docxDescription: "Akkor használd, ha a feladat egy munkaterületi .docx fájl köré épül, és natív, Word-szerű dokumentumra van szükség íráshoz, sablonkitöltéshez, helykitöltők szerkesztéséhez vagy ellenőrzéshez.",
    xlsxDescription: "Akkor használd, ha a feladat egy munkaterületi .xlsx fájl köré épül, és natív táblázatra van szükség táblákhoz, képletekhez, munkalapszerkesztéshez, elemzéshez vagy ellenőrzéshez.",
    pptxDescription: "Akkor használd, ha a feladat egy munkaterületi .pptx fájl köré épül, és natív prezentációra van szükség szerkeszthető diákhoz, elrendezésmódosításokhoz vagy diagramfrissítésekhez.",
    pdfDescription: "Akkor használd, ha a feladat egy munkaterületi .pdf fájl köré épül, és natív PDF-olvasásra, űrlapkitöltésre, nyomtatható kimenetre vagy az elrendezést megőrző újraformázásra van szükség.",
  },
  "it-IT": {
    ...nativeDocumentToolNames,
    docxDescription: "Usalo quando l'attività è incentrata su un file .docx dell'area di lavoro e richiede un documento nativo in stile Word per scrivere, compilare modelli, modificare segnaposto o validare il file.",
    xlsxDescription: "Usalo quando l'attività è incentrata su un file .xlsx dell'area di lavoro e richiede un foglio di calcolo nativo per tabelle, formule, modifiche ai fogli, analisi o validazione.",
    pptxDescription: "Usalo quando l'attività è incentrata su un file .pptx dell'area di lavoro e richiede una presentazione nativa per slide modificabili, cambi di layout o aggiornamenti dei grafici.",
    pdfDescription: "Usalo quando l'attività è incentrata su un file .pdf dell'area di lavoro e richiede lettura PDF nativa, compilazione di moduli, output stampabile o riformattazione che preservi il layout.",
  },
  "ja-JP": {
    ...nativeDocumentToolNames,
    docxDescription: "ワークスペースの .docx ファイルが作業の中心で、文章作成、テンプレート入力、プレースホルダー編集、検証のためにネイティブな Word 風文書が必要なときに使います。",
    xlsxDescription: "ワークスペースの .xlsx ファイルが作業の中心で、表、数式、シート編集、分析、検証のためにネイティブなスプレッドシートが必要なときに使います。",
    pptxDescription: "ワークスペースの .pptx ファイルが作業の中心で、編集可能なスライド、レイアウト変更、グラフ更新のためにネイティブなスライドデッキが必要なときに使います。",
    pdfDescription: "ワークスペースの .pdf ファイルが作業の中心で、PDF ネイティブな読み取り、フォーム入力、印刷向け出力、またはレイアウトを保つ再整形が必要なときに使います。",
  },
  "ko-KR": {
    ...nativeDocumentToolNames,
    docxDescription: "작업이 작업공간의 .docx 파일을 중심으로 진행되고, 작성, 템플릿 채우기, 플레이스홀더 편집 또는 검증을 위한 네이티브 Word 스타일 문서가 필요할 때 사용합니다.",
    xlsxDescription: "작업이 작업공간의 .xlsx 파일을 중심으로 진행되고, 표, 수식, 시트 편집, 분석 또는 검증을 위한 네이티브 스프레드시트가 필요할 때 사용합니다.",
    pptxDescription: "작업이 작업공간의 .pptx 파일을 중심으로 진행되고, 편집 가능한 슬라이드, 레이아웃 변경 또는 차트 업데이트를 위한 네이티브 슬라이드 덱이 필요할 때 사용합니다.",
    pdfDescription: "작업이 작업공간의 .pdf 파일을 중심으로 진행되고, PDF 네이티브 읽기, 양식 작성, 인쇄용 출력 또는 레이아웃을 유지하는 재포맷이 필요할 때 사용합니다.",
  },
  "ml-IN": {
    ...nativeDocumentToolNames,
    docxDescription: "ടാസ്‌ക് വർക്ക്‌സ്‌പേസിലെ .docx ഫയലിനെ കേന്ദ്രീകരിക്കുമ്പോഴും എഴുതൽ, ടെംപ്ലേറ്റ് പൂരിപ്പിക്കൽ, പ്ലേസ്‌ഹോൾഡർ തിരുത്തൽ, അല്ലെങ്കിൽ സാധുത പരിശോധനക്കായി നേറ്റീവ് Word ശൈലിയിലുള്ള ഡോക്യുമെന്റ് വേണമെങ്കിലും ഇത് ഉപയോഗിക്കുക.",
    xlsxDescription: "ടാസ്‌ക് വർക്ക്‌സ്‌പേസിലെ .xlsx ഫയലിനെ കേന്ദ്രീകരിക്കുമ്പോഴും പട്ടികകൾ, ഫോർമുലകൾ, ഷീറ്റ് തിരുത്തൽ, വിശകലനം, അല്ലെങ്കിൽ സാധുത പരിശോധനക്കായി നേറ്റീവ് സ്പ്രെഡ്ഷീറ്റ് വേണമെങ്കിലും ഇത് ഉപയോഗിക്കുക.",
    pptxDescription: "ടാസ്‌ക് വർക്ക്‌സ്‌പേസിലെ .pptx ഫയലിനെ കേന്ദ്രീകരിക്കുമ്പോഴും എഡിറ്റുചെയ്യാവുന്ന സ്ലൈഡുകൾ, ലേയൗട്ട് മാറ്റങ്ങൾ, അല്ലെങ്കിൽ ചാർട്ട് അപ്ഡേറ്റുകൾക്കായി നേറ്റീവ് സ്ലൈഡ് ഡെക്ക് വേണമെങ്കിലും ഇത് ഉപയോഗിക്കുക.",
    pdfDescription: "ടാസ്‌ക് വർക്ക്‌സ്‌പേസിലെ .pdf ഫയലിനെ കേന്ദ്രീകരിക്കുമ്പോഴും PDF നേറ്റീവ് വായന, ഫോം പൂരിപ്പിക്കൽ, പ്രിന്റ് ചെയ്യാവുന്ന ഔട്ട്പുട്ട്, അല്ലെങ്കിൽ ലേയൗട്ട് സംരക്ഷിക്കുന്ന റീഫോർമാറ്റിംഗ് വേണമെങ്കിലും ഇത് ഉപയോഗിക്കുക.",
  },
  "nb-NO": {
    ...nativeDocumentToolNames,
    docxDescription: "Bruk det når oppgaven dreier seg om en .docx-fil i arbeidsområdet og trenger et innebygd Word-lignende dokument for skriving, malutfylling, redigering av plassholdere eller validering.",
    xlsxDescription: "Bruk det når oppgaven dreier seg om en .xlsx-fil i arbeidsområdet og trenger et innebygd regneark for tabeller, formler, arkredigering, analyse eller validering.",
    pptxDescription: "Bruk det når oppgaven dreier seg om en .pptx-fil i arbeidsområdet og trenger en innebygd presentasjon for redigerbare lysbilder, layoutendringer eller diagramoppdateringer.",
    pdfDescription: "Bruk det når oppgaven dreier seg om en .pdf-fil i arbeidsområdet og trenger PDF-native lesing, skjemautfylling, utskriftsklar output eller reformatering som bevarer layouten.",
  },
  "nl-NL": {
    ...nativeDocumentToolNames,
    docxDescription: "Gebruik dit wanneer de taak draait om een .docx-bestand in de werkruimte en een native document in Word-stijl nodig heeft voor schrijven, sjablonen invullen, placeholders bewerken of valideren.",
    xlsxDescription: "Gebruik dit wanneer de taak draait om een .xlsx-bestand in de werkruimte en een native spreadsheet nodig heeft voor tabellen, formules, bladbewerkingen, analyse of validatie.",
    pptxDescription: "Gebruik dit wanneer de taak draait om een .pptx-bestand in de werkruimte en een native presentatie nodig heeft voor bewerkbare dia's, lay-outwijzigingen of grafiekupdates.",
    pdfDescription: "Gebruik dit wanneer de taak draait om een .pdf-bestand in de werkruimte en PDF-native lezen, formulierinvulling, afdrukbare output of herformattering met behoud van lay-out nodig heeft.",
  },
  "pl-PL": {
    ...nativeDocumentToolNames,
    docxDescription: "Użyj tego, gdy zadanie koncentruje się na pliku .docx w obszarze roboczym i wymaga natywnego dokumentu w stylu Worda do pisania, wypełniania szablonów, edycji placeholderów lub walidacji.",
    xlsxDescription: "Użyj tego, gdy zadanie koncentruje się na pliku .xlsx w obszarze roboczym i wymaga natywnego arkusza kalkulacyjnego do tabel, formuł, edycji arkuszy, analizy lub walidacji.",
    pptxDescription: "Użyj tego, gdy zadanie koncentruje się na pliku .pptx w obszarze roboczym i wymaga natywnej prezentacji do edytowalnych slajdów, zmian układu lub aktualizacji wykresów.",
    pdfDescription: "Użyj tego, gdy zadanie koncentruje się na pliku .pdf w obszarze roboczym i wymaga natywnego odczytu PDF, wypełniania formularzy, wydruku lub przeformatowania z zachowaniem układu.",
  },
  "pt-BR": {
    ...nativeDocumentToolNames,
    docxDescription: "Use quando a tarefa estiver centrada em um arquivo .docx do workspace e precisar de um documento nativo no estilo Word para escrever, preencher modelos, editar marcadores ou validar.",
    xlsxDescription: "Use quando a tarefa estiver centrada em um arquivo .xlsx do workspace e precisar de uma planilha nativa para tabelas, fórmulas, edição de abas, análise ou validação.",
    pptxDescription: "Use quando a tarefa estiver centrada em um arquivo .pptx do workspace e precisar de uma apresentação nativa para slides editáveis, mudanças de layout ou atualizações de gráficos.",
    pdfDescription: "Use quando a tarefa estiver centrada em um arquivo .pdf do workspace e precisar de leitura nativa de PDF, preenchimento de formulários, saída para impressão ou reformatação que preserve o layout.",
  },
  "pt-PT": {
    ...nativeDocumentToolNames,
    docxDescription: "Use quando a tarefa estiver centrada num ficheiro .docx do workspace e precisar de um documento nativo ao estilo Word para escrever, preencher modelos, editar marcadores ou validar.",
    xlsxDescription: "Use quando a tarefa estiver centrada num ficheiro .xlsx do workspace e precisar de uma folha de cálculo nativa para tabelas, fórmulas, edição de folhas, análise ou validação.",
    pptxDescription: "Use quando a tarefa estiver centrada num ficheiro .pptx do workspace e precisar de uma apresentação nativa para diapositivos editáveis, alterações de esquema ou atualizações de gráficos.",
    pdfDescription: "Use quando a tarefa estiver centrada num ficheiro .pdf do workspace e precisar de leitura nativa de PDF, preenchimento de formulários, saída para impressão ou reformatação que preserve o esquema.",
  },
  "ro-RO": {
    ...nativeDocumentToolNames,
    docxDescription: "Folosește-l când sarcina este centrată pe un fișier .docx din workspace și are nevoie de un document nativ de tip Word pentru redactare, completarea șabloanelor, editarea placeholderelor sau validare.",
    xlsxDescription: "Folosește-l când sarcina este centrată pe un fișier .xlsx din workspace și are nevoie de un tabelar nativ pentru tabele, formule, editări de foi, analiză sau validare.",
    pptxDescription: "Folosește-l când sarcina este centrată pe un fișier .pptx din workspace și are nevoie de o prezentare nativă pentru diapozitive editabile, schimbări de aspect sau actualizări de grafice.",
    pdfDescription: "Folosește-l când sarcina este centrată pe un fișier .pdf din workspace și are nevoie de citire PDF nativă, completare de formulare, ieșire pentru tipar sau reformatare care păstrează aspectul.",
  },
  "ru-RU": {
    ...nativeDocumentToolNames,
    docxDescription: "Используйте это, когда задача сосредоточена на файле .docx в рабочем пространстве и нужен нативный документ в стиле Word для написания текста, заполнения шаблонов, редактирования плейсхолдеров или проверки.",
    xlsxDescription: "Используйте это, когда задача сосредоточена на файле .xlsx в рабочем пространстве и нужна нативная таблица для таблиц, формул, правок листов, анализа или проверки.",
    pptxDescription: "Используйте это, когда задача сосредоточена на файле .pptx в рабочем пространстве и нужна нативная презентация для редактируемых слайдов, изменений макета или обновления диаграмм.",
    pdfDescription: "Используйте это, когда задача сосредоточена на файле .pdf в рабочем пространстве и нужно нативное чтение PDF, заполнение форм, печатный вывод или переформатирование с сохранением макета.",
  },
  "sk-SK": {
    ...nativeDocumentToolNames,
    docxDescription: "Použite ho, keď sa úloha sústreďuje na súbor .docx v pracovnom priestore a potrebuje natívny dokument v štýle Wordu na písanie, vyplnenie šablóny, úpravy zástupných symbolov alebo validáciu.",
    xlsxDescription: "Použite ho, keď sa úloha sústreďuje na súbor .xlsx v pracovnom priestore a potrebuje natívny tabuľkový hárok na tabuľky, vzorce, úpravy listov, analýzu alebo validáciu.",
    pptxDescription: "Použite ho, keď sa úloha sústreďuje na súbor .pptx v pracovnom priestore a potrebuje natívnu prezentáciu na upraviteľné snímky, zmeny rozloženia alebo aktualizácie grafov.",
    pdfDescription: "Použite ho, keď sa úloha sústreďuje na súbor .pdf v pracovnom priestore a potrebuje natívne čítanie PDF, vypĺňanie formulárov, tlačový výstup alebo preformátovanie so zachovaním rozloženia.",
  },
  "sv-SE": {
    ...nativeDocumentToolNames,
    docxDescription: "Använd det när uppgiften kretsar kring en .docx-fil i arbetsytan och behöver ett inbyggt Word-liknande dokument för skrivande, mallifyllning, redigering av platshållare eller validering.",
    xlsxDescription: "Använd det när uppgiften kretsar kring en .xlsx-fil i arbetsytan och behöver ett inbyggt kalkylblad för tabeller, formler, bladredigering, analys eller validering.",
    pptxDescription: "Använd det när uppgiften kretsar kring en .pptx-fil i arbetsytan och behöver en inbyggd presentation för redigerbara bilder, layoutändringar eller diagramuppdateringar.",
    pdfDescription: "Använd det när uppgiften kretsar kring en .pdf-fil i arbetsytan och behöver PDF-inbyggd läsning, formulärifyllning, utskriftsklart resultat eller omformatering som bevarar layouten.",
  },
  "zh-CN": {
    ...nativeDocumentToolNames,
    docxDescription: "当任务围绕工作区中的 .docx 文件展开，并且需要原生 Word 风格文档来撰写内容、填写模板、编辑占位符或做校验时，使用它。",
    xlsxDescription: "当任务围绕工作区中的 .xlsx 文件展开，并且需要原生电子表格来处理表格、公式、工作表编辑、分析或校验时，使用它。",
    pptxDescription: "当任务围绕工作区中的 .pptx 文件展开，并且需要原生演示文稿来编辑幻灯片、调整版式或更新图表时，使用它。",
    pdfDescription: "当任务围绕工作区中的 .pdf 文件展开，并且需要原生 PDF 阅读、表单填写、可打印输出或保留版式的重排时，使用它。",
  },
  "zh-TW": {
    ...nativeDocumentToolNames,
    docxDescription: "當任務圍繞工作區中的 .docx 檔案展開，並且需要原生 Word 風格文件來撰寫內容、填寫範本、編輯佔位符或進行驗證時，使用它。",
    xlsxDescription: "當任務圍繞工作區中的 .xlsx 檔案展開，並且需要原生試算表來處理表格、公式、工作表編輯、分析或驗證時，使用它。",
    pptxDescription: "當任務圍繞工作區中的 .pptx 檔案展開，並且需要原生簡報來編輯投影片、調整版面或更新圖表時，使用它。",
    pdfDescription: "當任務圍繞工作區中的 .pdf 檔案展開，並且需要原生 PDF 閱讀、表單填寫、可列印輸出或保留版面的重新格式化時，使用它。",
  },
}

export default nativeDocumentToolBackfills
