import type { LocaleKey } from "./locale-catalog"

export type NativeDocumentToolLocaleTerms = {
  docxDescription: string
  xlsxDescription: string
  pptxDescription: string
  pdfDescription: string
}

const nativeDocumentToolBackfills: Record<LocaleKey, NativeDocumentToolLocaleTerms> = {
  "ca-ES": {
    docxDescription: "Llegeix, crea, edita, aplica plantilles o valida fitxers .docx nadius de l'espai de treball amb gestio OOXML native-first i telemetria de degradacio explicita.",
    xlsxDescription: "Llegeix, crea, edita, corregeix o valida fitxers .xlsx nadius de l'espai de treball amb gestio OOXML native-first i telemetria de degradacio explicita.",
    pptxDescription: "Llegeix, crea o edita fitxers .pptx nadius de l'espai de treball amb empaquetatge OOXML native-first i telemetria de degradacio explicita.",
    pdfDescription: "Llegeix metadades PDF, extreu text de PDF locals o remots, inspecciona camps de formulari interactius o crea i reformata fitxers PDF nadius de l'espai de treball amb telemetria explicita de pagina i disseny.",
  },
  "cs-CZ": {
    docxDescription: "Cte, vytvari, upravuje, sablonizuje nebo overuje nativni soubory .docx v pracovnim prostoru s native-first zpracovanim OOXML a explicitni telemetrii degradace.",
    xlsxDescription: "Cte, vytvari, upravuje, opravuje nebo overuje nativni soubory .xlsx v pracovnim prostoru s native-first zpracovanim OOXML a explicitni telemetrii degradace.",
    pptxDescription: "Cte, vytvari nebo upravuje nativni soubory .pptx v pracovnim prostoru s native-first balenim OOXML a explicitni telemetrii degradace.",
    pdfDescription: "Cte metadata PDF, extrahuje text z mistnich nebo vzdalenych PDF, kontroluje interaktivni polozky formularu nebo vytvari a preformatovava nativni PDF soubory v pracovnim prostoru s explicitni telemetrii stranek a rozlozeni.",
  },
  "da-DK": {
    docxDescription: "Laes, opret, rediger, skabeloniser eller valider lokale .docx-arbejdsomradefiler med native-first OOXML-handtering og tydelig degraderingstelemetri.",
    xlsxDescription: "Laes, opret, rediger, ret eller valider lokale .xlsx-arbejdsomradefiler med native-first OOXML-handtering og tydelig degraderingstelemetri.",
    pptxDescription: "Laes, opret eller rediger lokale .pptx-arbejdsomradefiler med native-first OOXML-pakning og tydelig degraderingstelemetri.",
    pdfDescription: "Laes PDF-metadata, udtraek tekst fra lokale eller eksterne PDF-filer, inspicer interaktive formularfelter, eller opret og omformater lokale PDF-arbejdsomradefiler med tydelig side- og layouttelemetri.",
  },
  "de-DE": {
    docxDescription: "Liest, erstellt, bearbeitet, templatet oder validiert native .docx-Arbeitsbereichsdateien mit native-first-OOXML-Verarbeitung und expliziter Degradations-Telemetrie.",
    xlsxDescription: "Liest, erstellt, bearbeitet, repariert oder validiert native .xlsx-Arbeitsbereichsdateien mit native-first-OOXML-Verarbeitung und expliziter Degradations-Telemetrie.",
    pptxDescription: "Liest, erstellt oder bearbeitet native .pptx-Arbeitsbereichsdateien mit native-first-OOXML-Paketierung und expliziter Degradations-Telemetrie.",
    pdfDescription: "Liest PDF-Metadaten, extrahiert Text aus lokalen oder entfernten PDFs, prüft interaktive Formularfelder oder erstellt und formatiert native PDF-Arbeitsbereichsdateien mit expliziter Seiten- und Layout-Telemetrie um.",
  },
  "el-GR": {
    docxDescription: "Διαβαζει, δημιουργει, επεξεργαζεται, εφαρμοζει προτυπα η επικυρωνει εγγενη αρχεια .docx του χωρου εργασιας με native-first χειρισμο OOXML και ρητη τηλεμετρια υποβαθμισης.",
    xlsxDescription: "Διαβαζει, δημιουργει, επεξεργαζεται, διορθωνει η επικυρωνει εγγενη αρχεια .xlsx του χωρου εργασιας με native-first χειρισμο OOXML και ρητη τηλεμετρια υποβαθμισης.",
    pptxDescription: "Διαβαζει, δημιουργει η επεξεργαζεται εγγενη αρχεια .pptx του χωρου εργασιας με native-first συσκευασια OOXML και ρητη τηλεμετρια υποβαθμισης.",
    pdfDescription: "Διαβαζει μεταδεδομενα PDF, εξαγει κειμενο απο τοπικα η απομακρυσμενα PDF, επιθεωρει διαδραστικα πεδια φορμας η δημιουργει και αναμορφωνει εγγενη αρχεια PDF του χωρου εργασιας με ρητη τηλεμετρια σελιδας και διαταξης.",
  },
  "en-GB": {
    docxDescription: "Read, create, edit, template, or validate native .docx workspace files with native-first OOXML handling and explicit degradation telemetry.",
    xlsxDescription: "Read, create, edit, fix, or validate native .xlsx workspace files with native-first OOXML handling and explicit degradation telemetry.",
    pptxDescription: "Read, create, or edit native .pptx workspace files with native-first OOXML packaging and explicit degradation telemetry.",
    pdfDescription: "Read PDF metadata, extract text from local/remote PDFs, inspect interactive form fields, or create/reformat native PDF workspace files with explicit page/layout telemetry.",
  },
  "en-US": {
    docxDescription: "Read, create, edit, template, or validate native .docx workspace files with native-first OOXML handling and explicit degradation telemetry.",
    xlsxDescription: "Read, create, edit, fix, or validate native .xlsx workspace files with native-first OOXML handling and explicit degradation telemetry.",
    pptxDescription: "Read, create, or edit native .pptx workspace files with native-first OOXML packaging and explicit degradation telemetry.",
    pdfDescription: "Read PDF metadata, extract text from local/remote PDFs, inspect interactive form fields, or create/reformat native PDF workspace files with explicit page/layout telemetry.",
  },
  "es-ES": {
    docxDescription: "Lee, crea, edita, aplica plantillas o valida archivos .docx nativos del espacio de trabajo con manejo OOXML native-first y telemetria de degradacion explicita.",
    xlsxDescription: "Lee, crea, edita, corrige o valida archivos .xlsx nativos del espacio de trabajo con manejo OOXML native-first y telemetria de degradacion explicita.",
    pptxDescription: "Lee, crea o edita archivos .pptx nativos del espacio de trabajo con empaquetado OOXML native-first y telemetria de degradacion explicita.",
    pdfDescription: "Lee metadatos PDF, extrae texto de PDF locales o remotos, inspecciona campos de formulario interactivos o crea y reformatea archivos PDF nativos del espacio de trabajo con telemetria explicita de pagina y diseno.",
  },
  "fr-FR": {
    docxDescription: "Lit, cree, modifie, applique des modeles ou valide des fichiers .docx natifs de l'espace de travail avec traitement OOXML native-first et telemetrie de degradation explicite.",
    xlsxDescription: "Lit, cree, modifie, repare ou valide des fichiers .xlsx natifs de l'espace de travail avec traitement OOXML native-first et telemetrie de degradation explicite.",
    pptxDescription: "Lit, cree ou modifie des fichiers .pptx natifs de l'espace de travail avec empaquetage OOXML native-first et telemetrie de degradation explicite.",
    pdfDescription: "Lit les metadonnees PDF, extrait le texte de PDF locaux ou distants, inspecte les champs de formulaire interactifs ou cree et reformate des fichiers PDF natifs de l'espace de travail avec une telemetrie explicite de page et de mise en page.",
  },
  "ga-IE": {
    docxDescription: "Leann, cruthaionn, cuir in eagar, cuireann teimplaidi i bhfeidhm no bailionn comhaid .docx duthchasacha san ionad oibre le laimhseail OOXML native-first agus teiliméadracht dho-ghradaithe shoiléir.",
    xlsxDescription: "Leann, cruthaionn, cuir in eagar, deisigh no bailionn comhaid .xlsx duthchasacha san ionad oibre le laimhseail OOXML native-first agus teiliméadracht dho-ghradaithe shoiléir.",
    pptxDescription: "Leann, cruthaionn no cuir in eagar comhaid .pptx duthchasacha san ionad oibre le pacaisteadh OOXML native-first agus teiliméadracht dho-ghradaithe shoiléir.",
    pdfDescription: "Leann meiteashonrai PDF, baintear teacs as PDFanna aitiula no iargulta, scrudaionn reimsi foirme idirghniomhaí no cruthaionn agus athfhoirmataionn comhaid PDF duthchasacha san ionad oibre le teiliméadracht shoiléir leathanaigh agus leagain amach.",
  },
  "hr-HR": {
    docxDescription: "Cita, stvara, uredjuje, primjenjuje predloske ili provjerava izvorne .docx datoteke radnog prostora uz native-first OOXML obradu i izricitu telemetriju degradacije.",
    xlsxDescription: "Cita, stvara, uredjuje, popravlja ili provjerava izvorne .xlsx datoteke radnog prostora uz native-first OOXML obradu i izricitu telemetriju degradacije.",
    pptxDescription: "Cita, stvara ili uredjuje izvorne .pptx datoteke radnog prostora uz native-first OOXML pakiranje i izricitu telemetriju degradacije.",
    pdfDescription: "Cita PDF metapodatke, izvlaci tekst iz lokalnih ili udaljenih PDF-ova, pregledava interaktivna polja obrasca ili stvara i preoblikuje izvorne PDF datoteke radnog prostora uz izricitu telemetriju stranice i rasporeda.",
  },
  "hu-HU": {
    docxDescription: "Natív .docx munkaterületfájlokat olvas, hoz létre, szerkeszt, sablonoz vagy ellenoriz native-first OOXML-kezeléssel és kifejezett degradacios telemetriaval.",
    xlsxDescription: "Natív .xlsx munkaterületfájlokat olvas, hoz létre, szerkeszt, javit vagy ellenoriz native-first OOXML-kezeléssel és kifejezett degradacios telemetriaval.",
    pptxDescription: "Natív .pptx munkaterületfájlokat olvas, hoz létre vagy szerkeszt native-first OOXML-csomagolással és kifejezett degradacios telemetriaval.",
    pdfDescription: "PDF-metaadatokat olvas, szöveget nyer ki helyi vagy tavoli PDF-ekbol, interaktiv urlapmezoket vizsgal, illetve natív PDF munkaterületfájlokat hoz létre es formal ujra kifejezett oldal- es elrendezestelemetriaval.",
  },
  "it-IT": {
    docxDescription: "Legge, crea, modifica, applica template o valida file .docx nativi dell'area di lavoro con gestione OOXML native-first e telemetria di degrado esplicita.",
    xlsxDescription: "Legge, crea, modifica, corregge o valida file .xlsx nativi dell'area di lavoro con gestione OOXML native-first e telemetria di degrado esplicita.",
    pptxDescription: "Legge, crea o modifica file .pptx nativi dell'area di lavoro con packaging OOXML native-first e telemetria di degrado esplicita.",
    pdfDescription: "Legge metadati PDF, estrae testo da PDF locali o remoti, ispeziona campi modulo interattivi oppure crea e riformatta file PDF nativi dell'area di lavoro con telemetria esplicita di pagina e layout.",
  },
  "ja-JP": {
    docxDescription: "native-first の OOXML 処理と明示的な劣化テレメトリで、ネイティブ .docx ワークスペースファイルの読み取り、作成、編集、テンプレート適用、検証を行います。",
    xlsxDescription: "native-first の OOXML 処理と明示的な劣化テレメトリで、ネイティブ .xlsx ワークスペースファイルの読み取り、作成、編集、修復、検証を行います。",
    pptxDescription: "native-first の OOXML パッケージングと明示的な劣化テレメトリで、ネイティブ .pptx ワークスペースファイルの読み取り、作成、編集を行います。",
    pdfDescription: "PDF メタデータの読み取り、ローカルまたはリモート PDF からのテキスト抽出、対話型フォーム欄の確認、または明示的なページ/レイアウト テレメトリ付きでネイティブ PDF ワークスペースファイルの作成と再整形を行います。",
  },
  "ko-KR": {
    docxDescription: "native-first OOXML 처리와 명시적인 성능 저하 텔레메트리로 네이티브 .docx 작업공간 파일을 읽고, 만들고, 편집하고, 템플릿을 적용하거나 검증합니다.",
    xlsxDescription: "native-first OOXML 처리와 명시적인 성능 저하 텔레메트리로 네이티브 .xlsx 작업공간 파일을 읽고, 만들고, 편집하고, 수정하거나 검증합니다.",
    pptxDescription: "native-first OOXML 패키징과 명시적인 성능 저하 텔레메트리로 네이티브 .pptx 작업공간 파일을 읽고, 만들고, 편집합니다.",
    pdfDescription: "PDF 메타데이터를 읽고, 로컬 또는 원격 PDF에서 텍스트를 추출하고, 대화형 양식 필드를 검사하거나, 명시적인 페이지 및 레이아웃 텔레메트리와 함께 네이티브 PDF 작업공간 파일을 생성하고 다시 포맷합니다.",
  },
  "ml-IN": {
    docxDescription: "native-first OOXML കൈകാര്യം ചെയ്യലും വ്യക്തമാക്കിയ ഡിഗ്രഡേഷൻ ടെലിമെട്രിയും ഉപയോഗിച്ച് നെറ്റീവ് .docx വർക്ക്‌സ്‌പേസ് ഫയലുകൾ വായിക്കാനും സൃഷ്ടിക്കാനും തിരുത്താനും ടെംപ്ലേറ്റ് പ്രയോഗിക്കാനും സാധുത പരിശോധിക്കാനും സഹായിക്കുന്നു.",
    xlsxDescription: "native-first OOXML കൈകാര്യം ചെയ്യലും വ്യക്തമാക്കിയ ഡിഗ്രഡേഷൻ ടെലിമെട്രിയും ഉപയോഗിച്ച് നെറ്റീവ് .xlsx വർക്ക്‌സ്‌പേസ് ഫയലുകൾ വായിക്കാനും സൃഷ്ടിക്കാനും തിരുത്താനും ശരിയാക്കാനും സാധുത പരിശോധിക്കാനും സഹായിക്കുന്നു.",
    pptxDescription: "native-first OOXML പാക്കേജിംഗും വ്യക്തമാക്കിയ ഡിഗ്രഡേഷൻ ടെലിമെട്രിയും ഉപയോഗിച്ച് നെറ്റീവ് .pptx വർക്ക്‌സ്‌പേസ് ഫയലുകൾ വായിക്കാനും സൃഷ്ടിക്കാനും തിരുത്താനും സഹായിക്കുന്നു.",
    pdfDescription: "PDF മെറ്റാഡേറ്റ വായിക്കാനും, ലോക്കൽ അല്ലെങ്കിൽ റിമോട്ട് PDF-കളിൽ നിന്ന് ടെക്സ്റ്റ് എടുക്കാനും, ഇന്ററാക്ടീവ് ഫോം ഫീൽഡുകൾ പരിശോധിക്കാനും, അല്ലെങ്കിൽ വ്യക്തമായ പേജ്, ലേഔട്ട് ടെലിമെട്രിയോടെ നെറ്റീവ് PDF വർക്ക്‌സ്‌പേസ് ഫയലുകൾ സൃഷ്ടിക്കാനും റീഫോർമാറ്റ് ചെയ്യാനും സഹായിക്കുന്നു.",
  },
  "nb-NO": {
    docxDescription: "Les, opprett, rediger, malsett eller valider lokale .docx-arbeidsomradefiler med native-first OOXML-handtering og eksplisitt degraderingstelemetri.",
    xlsxDescription: "Les, opprett, rediger, reparer eller valider lokale .xlsx-arbeidsomradefiler med native-first OOXML-handtering og eksplisitt degraderingstelemetri.",
    pptxDescription: "Les, opprett eller rediger lokale .pptx-arbeidsomradefiler med native-first OOXML-pakking og eksplisitt degraderingstelemetri.",
    pdfDescription: "Les PDF-metadata, hent ut tekst fra lokale eller eksterne PDF-er, inspiser interaktive skjemafelt eller opprett og reformater lokale PDF-arbeidsomradefiler med eksplisitt side- og layouttelemetri.",
  },
  "nl-NL": {
    docxDescription: "Leest, maakt, bewerkt, sjabloniseert of valideert native .docx-werkruimtedocumenten met native-first OOXML-verwerking en expliciete degradatietelemetrie.",
    xlsxDescription: "Leest, maakt, bewerkt, herstelt of valideert native .xlsx-werkruimtebestanden met native-first OOXML-verwerking en expliciete degradatietelemetrie.",
    pptxDescription: "Leest, maakt of bewerkt native .pptx-werkruimtebestanden met native-first OOXML-verpakking en expliciete degradatietelemetrie.",
    pdfDescription: "Leest PDF-metadata, haalt tekst uit lokale of externe PDF-bestanden, inspecteert interactieve formuliervelden of maakt en herformatteert native PDF-werkruimtebestanden met expliciete pagina- en layouttelemetrie.",
  },
  "pl-PL": {
    docxDescription: "Czyta, tworzy, edytuje, stosuje szablony lub waliduje natywne pliki .docx w obszarze roboczym z obsluga OOXML native-first i jawna telemetria degradacji.",
    xlsxDescription: "Czyta, tworzy, edytuje, naprawia lub waliduje natywne pliki .xlsx w obszarze roboczym z obsluga OOXML native-first i jawna telemetria degradacji.",
    pptxDescription: "Czyta, tworzy lub edytuje natywne pliki .pptx w obszarze roboczym z pakowaniem OOXML native-first i jawna telemetria degradacji.",
    pdfDescription: "Czyta metadane PDF, wyciaga tekst z lokalnych lub zdalnych plikow PDF, sprawdza interaktywne pola formularzy albo tworzy i przeformatowuje natywne pliki PDF w obszarze roboczym z jawna telemetria stron i ukladu.",
  },
  "pt-BR": {
    docxDescription: "Le, cria, edita, aplica modelos ou valida arquivos .docx nativos do workspace com tratamento OOXML native-first e telemetria explicita de degradacao.",
    xlsxDescription: "Le, cria, edita, corrige ou valida arquivos .xlsx nativos do workspace com tratamento OOXML native-first e telemetria explicita de degradacao.",
    pptxDescription: "Le, cria ou edita arquivos .pptx nativos do workspace com empacotamento OOXML native-first e telemetria explicita de degradacao.",
    pdfDescription: "Le metadados de PDF, extrai texto de PDFs locais ou remotos, inspeciona campos interativos de formulario ou cria e reformata arquivos PDF nativos do workspace com telemetria explicita de pagina e layout.",
  },
  "pt-PT": {
    docxDescription: "Le, cria, edita, aplica modelos ou valida ficheiros .docx nativos do workspace com tratamento OOXML native-first e telemetria explicita de degradacao.",
    xlsxDescription: "Le, cria, edita, corrige ou valida ficheiros .xlsx nativos do workspace com tratamento OOXML native-first e telemetria explicita de degradacao.",
    pptxDescription: "Le, cria ou edita ficheiros .pptx nativos do workspace com empacotamento OOXML native-first e telemetria explicita de degradacao.",
    pdfDescription: "Le metadados PDF, extrai texto de PDFs locais ou remotos, inspeciona campos interativos de formulario ou cria e reformata ficheiros PDF nativos do workspace com telemetria explicita de pagina e esquema.",
  },
  "ro-RO": {
    docxDescription: "Citeste, creeaza, editeaza, aplica sabloane sau valideaza fisiere .docx native din workspace cu procesare OOXML native-first si telemetrie explicita de degradare.",
    xlsxDescription: "Citeste, creeaza, editeaza, repara sau valideaza fisiere .xlsx native din workspace cu procesare OOXML native-first si telemetrie explicita de degradare.",
    pptxDescription: "Citeste, creeaza sau editeaza fisiere .pptx native din workspace cu impachetare OOXML native-first si telemetrie explicita de degradare.",
    pdfDescription: "Citeste metadate PDF, extrage text din PDF-uri locale sau la distanta, inspecteaza campuri de formular interactive ori creeaza si reformateaza fisiere PDF native din workspace cu telemetrie explicita de pagina si aspect.",
  },
  "ru-RU": {
    docxDescription: "Читает, создает, редактирует, применяет шаблоны или проверяет нативные файлы .docx рабочего пространства с native-first обработкой OOXML и явной телеметрией деградации.",
    xlsxDescription: "Читает, создает, редактирует, исправляет или проверяет нативные файлы .xlsx рабочего пространства с native-first обработкой OOXML и явной телеметрией деградации.",
    pptxDescription: "Читает, создает или редактирует нативные файлы .pptx рабочего пространства с native-first упаковкой OOXML и явной телеметрией деградации.",
    pdfDescription: "Читает метаданные PDF, извлекает текст из локальных или удаленных PDF, проверяет интерактивные поля формы либо создает и переформатирует нативные PDF-файлы рабочего пространства с явной телеметрией страниц и макета.",
  },
  "sk-SK": {
    docxDescription: "Cita, vytvara, upravuje, sablonizuje alebo overuje nativne subory .docx v pracovnom priestore s native-first OOXML spracovanim a explicitnou telemetriou degradacie.",
    xlsxDescription: "Cita, vytvara, upravuje, opravuje alebo overuje nativne subory .xlsx v pracovnom priestore s native-first OOXML spracovanim a explicitnou telemetriou degradacie.",
    pptxDescription: "Cita, vytvara alebo upravuje nativne subory .pptx v pracovnom priestore s native-first OOXML balenim a explicitnou telemetriou degradacie.",
    pdfDescription: "Cita metadata PDF, extrahuje text z lokalnych alebo vzdialenych PDF, kontroluje interaktivne formularove polia alebo vytvara a preformatuje nativne PDF subory v pracovnom priestore s explicitnou telemetriou stran a rozlozenia.",
  },
  "sv-SE": {
    docxDescription: "Laser, skapar, redigerar, mallstyr eller validerar inbyggda .docx-arbetsytefiler med native-first OOXML-hantering och tydlig degraderingstelemetri.",
    xlsxDescription: "Laser, skapar, redigerar, reparerar eller validerar inbyggda .xlsx-arbetsytefiler med native-first OOXML-hantering och tydlig degraderingstelemetri.",
    pptxDescription: "Laser, skapar eller redigerar inbyggda .pptx-arbetsytefiler med native-first OOXML-paketering och tydlig degraderingstelemetri.",
    pdfDescription: "Laser PDF-metadata, extraherar text ur lokala eller fjarranslutna PDF-filer, inspekterar interaktiva formularfalt eller skapar och formaterar om inbyggda PDF-arbetsytefiler med tydlig sido- och layouttelemetri.",
  },
  "zh-CN": {
    docxDescription: "以 native-first OOXML 处理和明确的降级遥测，读取、创建、编辑、套用模板或校验原生 .docx 工作区文件。",
    xlsxDescription: "以 native-first OOXML 处理和明确的降级遥测，读取、创建、编辑、修复或校验原生 .xlsx 工作区文件。",
    pptxDescription: "以 native-first OOXML 打包和明确的降级遥测，读取、创建或编辑原生 .pptx 工作区文件。",
    pdfDescription: "读取 PDF 元数据、从本地或远程 PDF 提取文本、检查交互式表单字段，或在带有明确页面与布局遥测的情况下创建和重排原生 PDF 工作区文件。",
  },
  "zh-TW": {
    docxDescription: "以 native-first OOXML 處理與明確的降級遙測，讀取、建立、編輯、套用範本或驗證原生 .docx 工作區檔案。",
    xlsxDescription: "以 native-first OOXML 處理與明確的降級遙測，讀取、建立、編輯、修復或驗證原生 .xlsx 工作區檔案。",
    pptxDescription: "以 native-first OOXML 封裝與明確的降級遙測，讀取、建立或編輯原生 .pptx 工作區檔案。",
    pdfDescription: "讀取 PDF 中繼資料、從本機或遠端 PDF 擷取文字、檢查互動式表單欄位，或在具有明確頁面與版面配置遙測的情況下建立並重新格式化原生 PDF 工作區檔案。",
  },
}

export default nativeDocumentToolBackfills
