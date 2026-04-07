import type { LocaleKey } from './locale-catalog'

type LocaleTerms = {
  sandboxEnabled: string
  knowledgeName: string
  knowledgeDescription: string
  pii: string
  credentials: string
  financial: string
  request: string
  response: string
  both: string
  email: string
  phone: string
  ssn: string
  apiKeyFormat: string
  apiKey: string
  bearerToken: string
  creditCard: string
}

type ProviderPoolTabs = {
  all: string
  trial: string
  builtin: string
  platform: string
  other: string
  custom: string
  media: string
}

type SecurityScanItemTerms = {
  passwordName: string
  passwordDescription: string
  passwordDetailPassed: string
  passwordDetailWarning: string
  passwordDetailFailed: string
  tokenName: string
  tokenDescription: string
  tokenDetailPassed: string
  tokenDetailWarningHours: string
  tokenDetailWarningTooLong: string
  tokenDetailFailedNotSet: string
  mfaName: string
  mfaAvailableDescription: string
  mfaEnabledDescription: string
  mfaDetailPassed: string
  mfaDetailWarning: string
  mfaEnabledDetailPassed: string
}

function makeTerms(
  sandboxEnabled: string,
  knowledgeName: string,
  knowledgeDescription: string,
  pii: string,
  credentials: string,
  financial: string,
  request: string,
  response: string,
  both: string,
  email: string,
  phone: string,
  ssn: string,
  apiKeyFormat: string,
  apiKey: string,
  bearerToken: string,
  creditCard: string
): LocaleTerms {
  return {
    sandboxEnabled,
    knowledgeName,
    knowledgeDescription,
    pii,
    credentials,
    financial,
    request,
    response,
    both,
    email,
    phone,
    ssn,
    apiKeyFormat,
    apiKey,
    bearerToken,
    creditCard,
  }
}

function makeTabs(
  all: string,
  trial: string,
  builtin: string,
  platform: string,
  other: string,
  custom: string,
  media: string
): ProviderPoolTabs {
  return { all, trial, builtin, platform, other, custom, media }
}

const localeTerms: Record<LocaleKey, LocaleTerms> = {
  'ca-ES': makeTerms("L'execució en sandbox està habilitada", 'Lint nocturn del coneixement', 'Executa un lint nocturn del coneixement per mantenir saludable el coneixement compilat.', 'Dades personals', 'Credencials', 'Dades financeres', 'Sol·licitud', 'Resposta', 'Ambdues', 'Adreça electrònica', 'Número de telèfon', 'Número de la Seguretat Social', "Clau d'API (prefix sk-/key-)", "Clau d'API (etiqueta=valor)", 'Token Bearer', 'Número de targeta de crèdit'),
  'cs-CZ': makeTerms('Spouštění v sandboxu je povoleno', 'Noční lint znalostí', 'Spouští noční lint znalostí, aby kompilované znalosti zůstaly zdravé.', 'Osobní údaje', 'Přihlašovací údaje', 'Finanční údaje', 'Požadavek', 'Odpověď', 'Obojí', 'E-mailová adresa', 'Telefonní číslo', 'Číslo sociálního zabezpečení', 'API klíč (prefix sk-/key-)', 'API klíč (štítek=hodnota)', 'Bearer token', 'Číslo platební karty'),
  'da-DK': makeTerms('Sandbox-kørsel er aktiveret', 'Natlig videnslint', 'Kør en natlig videnslint for at holde den kompilerede viden sund.', 'Persondata', 'Adgangsoplysninger', 'Finansielle data', 'Anmodning', 'Svar', 'Begge', 'E-mailadresse', 'Telefonnummer', 'Socialsikringsnummer', 'API-nøgle (sk-/key- præfiks)', 'API-nøgle (etiket=værdi)', 'Bearer-token', 'Kreditkortnummer'),
  'de-DE': makeTerms('Sandbox-Ausführung ist aktiviert', 'Nächtlicher Wissens-Lint', 'Führt jede Nacht einen Wissens-Lint aus, um das kompilierte Wissen gesund zu halten.', 'Personenbezogene Daten', 'Anmeldedaten', 'Finanzdaten', 'Anfrage', 'Antwort', 'Beides', 'E-Mail-Adresse', 'Telefonnummer', 'Sozialversicherungsnummer', 'API-Schlüssel (sk-/key--Präfix)', 'API-Schlüssel (Label=Wert)', 'Bearer-Token', 'Kreditkartennummer'),
  'el-GR': makeTerms('Η εκτέλεση σε sandbox είναι ενεργή', 'Νυχτερινός έλεγχος γνώσης', 'Εκτελεί έναν νυχτερινό έλεγχο γνώσης για να διατηρεί την μεταγλωττισμένη γνώση υγιή.', 'Προσωπικά δεδομένα', 'Διαπιστευτήρια', 'Οικονομικά δεδομένα', 'Αίτημα', 'Απόκριση', 'Και τα δύο', 'Διεύθυνση email', 'Αριθμός τηλεφώνου', 'Αριθμός κοινωνικής ασφάλισης', 'Κλειδί API (πρόθεμα sk-/key-)', 'Κλειδί API (ετικέτα=τιμή)', 'Bearer token', 'Αριθμός πιστωτικής κάρτας'),
  'en-GB': makeTerms('Sandbox execution is enabled', 'Knowledge Nightly Lint', 'Run a nightly knowledge lint to keep compiled knowledge healthy.', 'Personal data', 'Credentials', 'Financial data', 'Request', 'Response', 'Both', 'Email Address', 'Phone Number', 'Social Security Number', 'API Key (sk-/key- prefix)', 'API Key (label=value)', 'Bearer Token', 'Credit Card Number'),
  'en-US': makeTerms('Sandbox execution is enabled', 'Knowledge Nightly Lint', 'Run a nightly knowledge lint to keep compiled knowledge healthy.', 'Personal data', 'Credentials', 'Financial data', 'Request', 'Response', 'Both', 'Email Address', 'Phone Number', 'Social Security Number', 'API Key (sk-/key- prefix)', 'API Key (label=value)', 'Bearer Token', 'Credit Card Number'),
  'es-ES': makeTerms('La ejecución en sandbox está habilitada', 'Lint nocturno del conocimiento', 'Ejecuta un lint nocturno del conocimiento para mantener saludable el conocimiento compilado.', 'Datos personales', 'Credenciales', 'Datos financieros', 'Solicitud', 'Respuesta', 'Ambos', 'Dirección de correo electrónico', 'Número de teléfono', 'Número de la Seguridad Social', 'Clave API (prefijo sk-/key-)', 'Clave API (etiqueta=valor)', 'Token Bearer', 'Número de tarjeta de crédito'),
  'fr-FR': makeTerms("L'exécution en sandbox est activée", 'Lint nocturne des connaissances', 'Exécute un lint nocturne des connaissances pour garder les connaissances compilées en bonne santé.', 'Données personnelles', 'Identifiants', 'Données financières', 'Requête', 'Réponse', 'Les deux', 'Adresse e-mail', 'Numéro de téléphone', 'Numéro de sécurité sociale', 'Clé API (préfixe sk-/key-)', 'Clé API (étiquette=valeur)', 'Jeton Bearer', 'Numéro de carte bancaire'),
  'ga-IE': makeTerms('Tá forghníomhú sa bhosca gainimh cumasaithe', 'Lint oíche an eolais', 'Rith lint oíche an eolais chun an t-eolas tiomsaithe a choinneáil slán.', 'Sonraí pearsanta', 'Dintiúir', 'Sonraí airgeadais', 'Iarratas', 'Freagra', 'An dá cheann', 'Seoladh ríomhphoist', 'Uimhir theileafóin', 'Uimhir leasa shóisialaigh', 'Eochair API (réimír sk-/key-)', 'Eochair API (lipéad=luach)', 'Comhartha Bearer', 'Uimhir chárta creidmheasa'),
  'hr-HR': makeTerms('Izvršavanje u sandboxu je omogućeno', 'Noćni lint znanja', 'Pokreće noćni lint znanja kako bi kompilirano znanje ostalo zdravo.', 'Osobni podaci', 'Vjerodajnice', 'Financijski podaci', 'Zahtjev', 'Odgovor', 'Oboje', 'Adresa e-pošte', 'Telefonski broj', 'Broj socijalnog osiguranja', 'API ključ (prefiks sk-/key-)', 'API ključ (oznaka=vrijednost)', 'Bearer token', 'Broj kreditne kartice'),
  'hu-HU': makeTerms('A sandbox-végrehajtás engedélyezve van', 'Éjszakai tudáslint', 'Éjszakai tudáslint futtatása a lefordított tudás egészségének megőrzéséhez.', 'Személyes adatok', 'Hitelesítési adatok', 'Pénzügyi adatok', 'Kérés', 'Válasz', 'Mindkettő', 'E-mail-cím', 'Telefonszám', 'Társadalombiztosítási szám', 'API-kulcs (sk-/key- előtag)', 'API-kulcs (címke=érték)', 'Bearer token', 'Bankkártyaszám'),
  'it-IT': makeTerms("L'esecuzione in sandbox è abilitata", 'Lint notturno della conoscenza', 'Esegue un lint notturno della conoscenza per mantenere in salute la conoscenza compilata.', 'Dati personali', 'Credenziali', 'Dati finanziari', 'Richiesta', 'Risposta', 'Entrambi', 'Indirizzo email', 'Numero di telefono', 'Numero di previdenza sociale', 'Chiave API (prefisso sk-/key-)', 'Chiave API (etichetta=valore)', 'Token Bearer', 'Numero di carta di credito'),
  'ja-JP': makeTerms('サンドボックス実行は有効です', '夜間ナレッジチェック', 'コンパイル済みナレッジの健全性を保つため、毎晩ナレッジチェックを実行します。', '個人データ', '認証情報', '金融データ', 'リクエスト', 'レスポンス', '両方', 'メールアドレス', '電話番号', '社会保障番号', 'API キー (sk-/key- プレフィックス)', 'API キー (label=value)', 'Bearer トークン', 'クレジットカード番号'),
  'ko-KR': makeTerms('샌드박스 실행이 활성화되어 있습니다', '지식 야간 점검', '컴파일된 지식의 건전성을 유지하기 위해 매일 밤 지식 점검을 실행합니다.', '개인 데이터', '자격 증명', '금융 데이터', '요청', '응답', '둘 다', '이메일 주소', '전화번호', '사회보장번호', 'API 키 (sk-/key- 접두사)', 'API 키 (label=value)', 'Bearer 토큰', '신용카드 번호'),
  'ml-IN': makeTerms('സാൻഡ്ബോക്സ് പ്രവർത്തനം സജ്ജമാണ്', 'രാത്രികാല നോളജ് ലിന്റ്', 'കമ്പൈൽ ചെയ്ത അറിവ് ആരോഗ്യകരമായി നിലനിർത്താൻ രാത്രികാല നോളജ് ലിന്റ് പ്രവർത്തിപ്പിക്കുക.', 'വ്യക്തിഗത ഡാറ്റ', 'ക്രെഡൻഷ്യലുകൾ', 'സാമ്പത്തിക ഡാറ്റ', 'അഭ്യർത്ഥന', 'പ്രതികരണം', 'രണ്ടും', 'ഇമെയിൽ വിലാസം', 'ഫോൺ നമ്പർ', 'സോഷ്യൽ സെക്യൂരിറ്റി നമ്പർ', 'API കീ (sk-/key- പ്രിഫിക്സ്)', 'API കീ (label=value)', 'Bearer ടോക്കൺ', 'ക്രെഡിറ്റ് കാർഡ് നമ്പർ'),
  'nb-NO': makeTerms('Sandkassekjøring er aktivert', 'Nattlig kunnskapslint', 'Kjør en nattlig kunnskapslint for å holde kompilert kunnskap sunn.', 'Personopplysninger', 'Påloggingsinformasjon', 'Finansielle data', 'Forespørsel', 'Svar', 'Begge', 'E-postadresse', 'Telefonnummer', 'Personnummer', 'API-nøkkel (sk-/key- prefiks)', 'API-nøkkel (etikett=verdi)', 'Bearer-token', 'Kredittkortnummer'),
  'nl-NL': makeTerms('Sandbox-uitvoering is ingeschakeld', 'Nachtelijke kennislint', 'Voer elke nacht een kennislint uit om gecompileerde kennis gezond te houden.', 'Persoonsgegevens', 'Inloggegevens', 'Financiële gegevens', 'Verzoek', 'Respons', 'Beide', 'E-mailadres', 'Telefoonnummer', 'Burgerservicenummer', 'API-sleutel (sk-/key- voorvoegsel)', 'API-sleutel (label=waarde)', 'Bearer-token', 'Creditcardnummer'),
  'pl-PL': makeTerms('Wykonywanie w piaskownicy jest włączone', 'Nocny lint wiedzy', 'Uruchamia nocny lint wiedzy, aby utrzymać skompilowaną wiedzę w dobrej kondycji.', 'Dane osobowe', 'Poświadczenia', 'Dane finansowe', 'Żądanie', 'Odpowiedź', 'Oba', 'Adres e-mail', 'Numer telefonu', 'Numer ubezpieczenia społecznego', 'Klucz API (prefiks sk-/key-)', 'Klucz API (etykieta=wartość)', 'Token Bearer', 'Numer karty kredytowej'),
  'pt-BR': makeTerms('A execução em sandbox está ativada', 'Lint noturno de conhecimento', 'Executa um lint noturno de conhecimento para manter saudável o conhecimento compilado.', 'Dados pessoais', 'Credenciais', 'Dados financeiros', 'Requisição', 'Resposta', 'Ambos', 'Endereço de e-mail', 'Número de telefone', 'Número de seguridade social', 'Chave de API (prefixo sk-/key-)', 'Chave de API (rótulo=valor)', 'Token Bearer', 'Número do cartão de crédito'),
  'pt-PT': makeTerms('A execução em sandbox está ativada', 'Lint noturno do conhecimento', 'Executa um lint noturno do conhecimento para manter saudável o conhecimento compilado.', 'Dados pessoais', 'Credenciais', 'Dados financeiros', 'Pedido', 'Resposta', 'Ambos', 'Endereço de e-mail', 'Número de telefone', 'Número da segurança social', 'Chave de API (prefixo sk-/key-)', 'Chave de API (rótulo=valor)', 'Token Bearer', 'Número do cartão de crédito'),
  'ro-RO': makeTerms('Execuția în sandbox este activată', 'Lint nocturn pentru cunoștințe', 'Rulează un lint nocturn pentru cunoștințe pentru a menține sănătoase cunoștințele compilate.', 'Date personale', 'Credențiale', 'Date financiare', 'Cerere', 'Răspuns', 'Ambele', 'Adresă de e-mail', 'Număr de telefon', 'Număr de asigurare socială', 'Cheie API (prefix sk-/key-)', 'Cheie API (etichetă=valoare)', 'Token Bearer', 'Număr card de credit'),
  'ru-RU': makeTerms('Выполнение в песочнице включено', 'Ночная проверка знаний', 'Запускает ночную проверку знаний, чтобы поддерживать скомпилированные знания в хорошем состоянии.', 'Персональные данные', 'Учетные данные', 'Финансовые данные', 'Запрос', 'Ответ', 'Оба', 'Адрес электронной почты', 'Номер телефона', 'Номер социального страхования', 'API-ключ (префикс sk-/key-)', 'API-ключ (метка=значение)', 'Bearer токен', 'Номер кредитной карты'),
  'sk-SK': makeTerms('Vykonávanie v sandboxe je povolené', 'Nočný lint znalostí', 'Spúšťa nočný lint znalostí, aby udržal skompilované znalosti v dobrom stave.', 'Osobné údaje', 'Prihlasovacie údaje', 'Finančné údaje', 'Požiadavka', 'Odpoveď', 'Oboje', 'E-mailová adresa', 'Telefónne číslo', 'Číslo sociálneho poistenia', 'API kľúč (prefix sk-/key-)', 'API kľúč (štítok=hodnota)', 'Bearer token', 'Číslo kreditnej karty'),
  'sv-SE': makeTerms('Sandbox-körning är aktiverad', 'Nattlig kunskapslint', 'Kör en nattlig kunskapslint för att hålla den kompilerade kunskapen frisk.', 'Personuppgifter', 'Inloggningsuppgifter', 'Finansiella data', 'Begäran', 'Svar', 'Båda', 'E-postadress', 'Telefonnummer', 'Personnummer', 'API-nyckel (sk-/key- prefix)', 'API-nyckel (etikett=värde)', 'Bearer-token', 'Kreditkortsnummer'),
  'zh-CN': makeTerms('已启用沙箱执行', '知识夜间检查', '每晚运行知识检查，维持编译知识健康度。', '个人信息', '凭证', '金融数据', '请求', '响应', '双向', '邮箱地址', '电话号码', '社会保障号码', 'API 密钥（sk-/key- 前缀）', 'API 密钥（标签=值）', 'Bearer 令牌', '信用卡号码'),
  'zh-TW': makeTerms('已啟用沙箱執行', '知識夜間檢查', '每晚執行知識檢查，維持編譯知識的健康度。', '個人資料', '憑證', '金融資料', '請求', '回應', '雙向', '電子郵件地址', '電話號碼', '社會安全號碼', 'API 金鑰（sk-/key- 前綴）', 'API 金鑰（標籤=值）', 'Bearer 權杖', '信用卡號碼'),
}

const providerPoolTabs: Record<LocaleKey, ProviderPoolTabs> = {
  'ca-ES': makeTabs('Tots', 'Prova', 'Integrat', 'Plataforma', 'Altres', 'Personalitzat', 'Mitjans'),
  'cs-CZ': makeTabs('Vše', 'Zkušební', 'Vestavěné', 'Platforma', 'Ostatní', 'Vlastní', 'Média'),
  'da-DK': makeTabs('Alle', 'Prøve', 'Indbygget', 'Platform', 'Andre', 'Brugerdefineret', 'Medier'),
  'de-DE': makeTabs('Alle', 'Testversion', 'Integriert', 'Plattform', 'Andere', 'Benutzerdefiniert', 'Medien'),
  'el-GR': makeTabs('Όλα', 'Δοκιμή', 'Ενσωματωμένα', 'Πλατφόρμα', 'Άλλα', 'Προσαρμοσμένα', 'Πολυμέσα'),
  'en-GB': makeTabs('All', 'Trial', 'Built-in', 'Platform', 'Other', 'Custom', 'Media'),
  'en-US': makeTabs('All', 'Trial', 'Built-in', 'Platform', 'Other', 'Custom', 'Media'),
  'es-ES': makeTabs('Todos', 'Prueba', 'Integrado', 'Plataforma', 'Otros', 'Personalizado', 'Medios'),
  'fr-FR': makeTabs('Tous', 'Essai', 'Intégré', 'Plateforme', 'Autres', 'Personnalisé', 'Médias'),
  'ga-IE': makeTabs('Uile', 'Triail', 'Ionsuite', 'Ardán', 'Eile', 'Saincheaptha', 'Meáin'),
  'hr-HR': makeTabs('Sve', 'Probno', 'Ugrađeno', 'Platforma', 'Ostalo', 'Prilagođeno', 'Mediji'),
  'hu-HU': makeTabs('Összes', 'Próba', 'Beépített', 'Platform', 'Egyéb', 'Egyéni', 'Média'),
  'it-IT': makeTabs('Tutti', 'Prova', 'Integrato', 'Piattaforma', 'Altro', 'Personalizzato', 'Media'),
  'ja-JP': makeTabs('すべて', 'トライアル', '組み込み', 'プラットフォーム', 'その他', 'カスタム', 'メディア'),
  'ko-KR': makeTabs('전체', '체험', '내장', '플랫폼', '기타', '사용자 정의', '미디어'),
  'ml-IN': makeTabs('എല്ലാം', 'ട്രയൽ', 'ബിൽറ്റ്-ഇൻ', 'പ്ലാറ്റ്ഫോം', 'മറ്റ്', 'കസ്റ്റം', 'മീഡിയ'),
  'nb-NO': makeTabs('Alle', 'Prøve', 'Innebygd', 'Plattform', 'Andre', 'Tilpasset', 'Media'),
  'nl-NL': makeTabs('Alle', 'Proef', 'Ingebouwd', 'Platform', 'Overig', 'Aangepast', 'Media'),
  'pl-PL': makeTabs('Wszystkie', 'Testowe', 'Wbudowane', 'Platforma', 'Inne', 'Niestandardowe', 'Media'),
  'pt-BR': makeTabs('Todos', 'Teste', 'Integrado', 'Plataforma', 'Outros', 'Personalizado', 'Mídia'),
  'pt-PT': makeTabs('Todos', 'Teste', 'Integrado', 'Plataforma', 'Outros', 'Personalizado', 'Multimédia'),
  'ro-RO': makeTabs('Toate', 'Trial', 'Integrat', 'Platformă', 'Altele', 'Personalizat', 'Media'),
  'ru-RU': makeTabs('Все', 'Пробный', 'Встроенные', 'Платформа', 'Другое', 'Пользовательские', 'Медиа'),
  'sk-SK': makeTabs('Všetky', 'Skúšobné', 'Vstavané', 'Platforma', 'Ostatné', 'Vlastné', 'Médiá'),
  'sv-SE': makeTabs('Alla', 'Testversion', 'Inbyggd', 'Plattform', 'Övrigt', 'Anpassad', 'Media'),
  'zh-CN': makeTabs('全部', '试用', '内置', '平台', '其他', '自定义', '媒体'),
  'zh-TW': makeTabs('全部', '試用', '內建', '平台', '其他', '自訂', '媒體'),
}

const securityScanItemTerms: Record<LocaleKey, SecurityScanItemTerms> = {
  'ca-ES': {
    passwordName: 'Longitud mínima de la contrasenya',
    passwordDescription:
      'Comprova si la longitud mínima de la contrasenya compleix els requisits de seguretat (es recomana 12+)',
    passwordDetailPassed:
      'La longitud mínima de la contrasenya és de {length} caràcters (compleix el requisit)',
    passwordDetailWarning:
      'La longitud mínima de la contrasenya és de {length} caràcters. Es recomana augmentar-la a 12+ per a més seguretat.',
    passwordDetailFailed:
      'La longitud mínima de la contrasenya ({length}) és massa curta. Es requereixen almenys 8 caràcters i se\'n recomanen 12+.',
    tokenName: "Temps d'expiració del token",
    tokenDescription: "Comprova si l'expiració del token està configurada amb una durada raonable",
    tokenDetailPassed: 'El token expira en {minutes} minuts',
    tokenDetailWarningHours:
      "L'expiració del token ({hours} hores) és llarga. Considera una durada més curta per a operacions sensibles.",
    tokenDetailWarningTooLong:
      "L'expiració del token supera les 8 hores. Això augmenta el risc de robatori de tokens.",
    tokenDetailFailedNotSet:
      "L'expiració del token no està configurada. Comprova `security.jwt.expiration` a la configuració de seguretat carregada.",
    mfaName: 'Autenticació multifactor',
    mfaAvailableDescription: "Comprova si l'MFA està disponible i s'aplica",
    mfaEnabledDescription: "Comprova si l'MFA està disponible i configurada",
    mfaDetailPassed: "L'MFA és obligatòria per a tots els usuaris",
    mfaDetailWarning:
      "L'MFA està disponible però no és obligatòria. Considera aplicar-la per millorar la seguretat.",
    mfaEnabledDetailPassed: "L'MFA està disponible per als usuaris",
  },
  'cs-CZ': {
    passwordName: 'Minimální délka hesla',
    passwordDescription:
      'Zkontroluje, zda minimální délka hesla splňuje bezpečnostní požadavky (doporučeno 12+)',
    passwordDetailPassed: 'Minimální délka hesla je {length} znaků (splňuje požadavek)',
    passwordDetailWarning:
      'Minimální délka hesla je {length} znaků. Pro lepší zabezpečení doporučujeme zvýšit na 12+.',
    passwordDetailFailed:
      'Minimální délka hesla ({length}) je příliš krátká. Vyžaduje se alespoň 8 znaků, doporučeno 12+.',
    tokenName: 'Doba platnosti tokenu',
    tokenDescription: 'Zkontroluje, zda je platnost tokenu nastavena na rozumnou dobu',
    tokenDetailPassed: 'Platnost tokenu končí za {minutes} minut',
    tokenDetailWarningHours:
      'Platnost tokenu ({hours} hodin) je dlouhá. U citlivých operací zvažte kratší dobu.',
    tokenDetailWarningTooLong:
      'Platnost tokenu přesahuje 8 hodin. To zvyšuje riziko krádeže tokenu.',
    tokenDetailFailedNotSet:
      'Platnost tokenu není nastavena. Zkontrolujte `security.jwt.expiration` v načtené konfiguraci zabezpečení.',
    mfaName: 'Vícefaktorové ověřování',
    mfaAvailableDescription: 'Zkontroluje, zda je MFA dostupné a vynucované',
    mfaEnabledDescription: 'Zkontroluje, zda je MFA dostupné a nakonfigurované',
    mfaDetailPassed: 'MFA je vyžadováno pro všechny uživatele',
    mfaDetailWarning:
      'MFA je dostupné, ale není vyžadováno. Zvažte jeho vynucení pro vyšší bezpečnost.',
    mfaEnabledDetailPassed: 'MFA je uživatelům dostupné',
  },
  'da-DK': {
    passwordName: 'Minimumslængde for adgangskode',
    passwordDescription:
      'Kontroller, om minimumslængden for adgangskoder opfylder sikkerhedskravene (12+ anbefales)',
    passwordDetailPassed: 'Minimumslængden for adgangskoden er {length} tegn (opfylder kravet)',
    passwordDetailWarning:
      'Minimumslængden for adgangskoden er {length} tegn. Det anbefales at øge den til 12+ for bedre sikkerhed.',
    passwordDetailFailed:
      'Minimumslængden for adgangskoden ({length}) er for kort. Mindst 8 tegn kræves, og 12+ anbefales.',
    tokenName: 'Token-udløbstid',
    tokenDescription: 'Kontroller, om tokenets udløbstid er sat til en rimelig varighed',
    tokenDetailPassed: 'Token udløber om {minutes} minutter',
    tokenDetailWarningHours:
      'Tokenets udløbstid ({hours} timer) er lang. Overvej en kortere varighed for følsomme handlinger.',
    tokenDetailWarningTooLong:
      'Tokenets udløbstid overstiger 8 timer. Det øger risikoen for token-tyveri.',
    tokenDetailFailedNotSet:
      'Tokenets udløbstid er ikke sat. Kontroller `security.jwt.expiration` i den indlæste sikkerhedskonfiguration.',
    mfaName: 'Multifaktorgodkendelse',
    mfaAvailableDescription: 'Kontroller, om MFA er tilgængelig og håndhæves',
    mfaEnabledDescription: 'Kontroller, om MFA er tilgængelig og konfigureret',
    mfaDetailPassed: 'MFA er påkrævet for alle brugere',
    mfaDetailWarning:
      'MFA er tilgængelig, men ikke påkrævet. Overvej at håndhæve MFA for bedre sikkerhed.',
    mfaEnabledDetailPassed: 'MFA er tilgængelig for brugere',
  },
  'de-DE': {
    passwordName: 'Minimale Passwortlänge',
    passwordDescription:
      'Prüft, ob die minimale Passwortlänge den Sicherheitsanforderungen entspricht (12+ empfohlen)',
    passwordDetailPassed:
      'Die minimale Passwortlänge beträgt {length} Zeichen (Anforderung erfüllt)',
    passwordDetailWarning:
      'Die minimale Passwortlänge beträgt {length} Zeichen. Für mehr Sicherheit wird 12+ empfohlen.',
    passwordDetailFailed:
      'Die minimale Passwortlänge ({length}) ist zu kurz. Mindestens 8 Zeichen sind erforderlich, 12+ werden empfohlen.',
    tokenName: 'Token-Ablaufzeit',
    tokenDescription: 'Prüft, ob die Ablaufzeit des Tokens sinnvoll gesetzt ist',
    tokenDetailPassed: 'Das Token läuft in {minutes} Minuten ab',
    tokenDetailWarningHours:
      'Die Token-Ablaufzeit ({hours} Stunden) ist lang. Für sensible Vorgänge ist eine kürzere Dauer ratsam.',
    tokenDetailWarningTooLong:
      'Die Token-Ablaufzeit überschreitet 8 Stunden. Das erhöht das Risiko eines Token-Diebstahls.',
    tokenDetailFailedNotSet:
      'Für das Token ist keine Ablaufzeit gesetzt. Prüfen Sie `security.jwt.expiration` in der geladenen Sicherheitskonfiguration.',
    mfaName: 'Multi-Faktor-Authentifizierung',
    mfaAvailableDescription: 'Prüft, ob MFA verfügbar und erzwungen ist',
    mfaEnabledDescription: 'Prüft, ob MFA verfügbar und konfiguriert ist',
    mfaDetailPassed: 'MFA ist für alle Benutzer erforderlich',
    mfaDetailWarning:
      'MFA ist verfügbar, aber nicht verpflichtend. Erwägen Sie eine verpflichtende MFA für mehr Sicherheit.',
    mfaEnabledDetailPassed: 'MFA ist für Benutzer verfügbar',
  },
  'el-GR': {
    passwordName: 'Ελάχιστο μήκος κωδικού πρόσβασης',
    passwordDescription:
      'Ελέγχει αν το ελάχιστο μήκος κωδικού πρόσβασης πληροί τις απαιτήσεις ασφαλείας (συνιστάται 12+)',
    passwordDetailPassed:
      'Το ελάχιστο μήκος κωδικού πρόσβασης είναι {length} χαρακτήρες (πληροί την απαίτηση)',
    passwordDetailWarning:
      'Το ελάχιστο μήκος κωδικού πρόσβασης είναι {length} χαρακτήρες. Συνιστάται αύξηση σε 12+ για καλύτερη ασφάλεια.',
    passwordDetailFailed:
      'Το ελάχιστο μήκος κωδικού πρόσβασης ({length}) είναι πολύ μικρό. Απαιτούνται τουλάχιστον 8 χαρακτήρες και συνιστώνται 12+.',
    tokenName: 'Χρόνος λήξης διακριτικού',
    tokenDescription: 'Ελέγχει αν η λήξη του διακριτικού έχει οριστεί σε λογική διάρκεια',
    tokenDetailPassed: 'Το διακριτικό λήγει σε {minutes} λεπτά',
    tokenDetailWarningHours:
      'Η λήξη του διακριτικού ({hours} ώρες) είναι μεγάλη. Εξετάστε μικρότερη διάρκεια για ευαίσθητες λειτουργίες.',
    tokenDetailWarningTooLong:
      'Η λήξη του διακριτικού υπερβαίνει τις 8 ώρες. Αυτό αυξάνει τον κίνδυνο κλοπής διακριτικού.',
    tokenDetailFailedNotSet:
      'Δεν έχει οριστεί λήξη διακριτικού. Ελέγξτε το `security.jwt.expiration` στη φορτωμένη διαμόρφωση ασφαλείας.',
    mfaName: 'Πολυπαραγοντικός έλεγχος ταυτότητας',
    mfaAvailableDescription: 'Ελέγχει αν το MFA είναι διαθέσιμο και επιβάλλεται',
    mfaEnabledDescription: 'Ελέγχει αν το MFA είναι διαθέσιμο και ρυθμισμένο',
    mfaDetailPassed: 'Το MFA απαιτείται για όλους τους χρήστες',
    mfaDetailWarning:
      'Το MFA είναι διαθέσιμο αλλά δεν απαιτείται. Εξετάστε την επιβολή του για ισχυρότερη ασφάλεια.',
    mfaEnabledDetailPassed: 'Το MFA είναι διαθέσιμο για τους χρήστες',
  },
  'en-GB': {
    passwordName: 'Password Minimum Length',
    passwordDescription:
      'Check if password minimum length meets security requirements (12+ recommended)',
    passwordDetailPassed: 'Password minimum length is {length} characters (meets requirement)',
    passwordDetailWarning:
      'Password minimum length is {length} characters. Recommend increasing to 12+ for better security.',
    passwordDetailFailed:
      'Password minimum length ({length}) is too short. Minimum 8 characters required, 12+ recommended.',
    tokenName: 'Token Expiration Time',
    tokenDescription: 'Check if token expiration is set to a reasonable duration',
    tokenDetailPassed: 'Token expires in {minutes} minutes',
    tokenDetailWarningHours:
      'Token expiration ({hours} hours) is long. Consider shorter duration for sensitive operations.',
    tokenDetailWarningTooLong:
      'Token expiration exceeds 8 hours. This increases risk of token theft.',
    tokenDetailFailedNotSet:
      'Token expiration is not set. Check security.jwt.expiration in the loaded security configuration.',
    mfaName: 'Multi-Factor Authentication',
    mfaAvailableDescription: 'Check if MFA is available and enforced',
    mfaEnabledDescription: 'Check if MFA is available and configured',
    mfaDetailPassed: 'MFA is required for all users',
    mfaDetailWarning:
      'MFA is available but not required. Consider enforcing MFA for enhanced security.',
    mfaEnabledDetailPassed: 'MFA is available for users',
  },
  'en-US': {
    passwordName: 'Password Minimum Length',
    passwordDescription:
      'Check if password minimum length meets security requirements (12+ recommended)',
    passwordDetailPassed: 'Password minimum length is {length} characters (meets requirement)',
    passwordDetailWarning:
      'Password minimum length is {length} characters. Recommend increasing to 12+ for better security.',
    passwordDetailFailed:
      'Password minimum length ({length}) is too short. Minimum 8 characters required, 12+ recommended.',
    tokenName: 'Token Expiration Time',
    tokenDescription: 'Check if token expiration is set to a reasonable duration',
    tokenDetailPassed: 'Token expires in {minutes} minutes',
    tokenDetailWarningHours:
      'Token expiration ({hours} hours) is long. Consider shorter duration for sensitive operations.',
    tokenDetailWarningTooLong:
      'Token expiration exceeds 8 hours. This increases risk of token theft.',
    tokenDetailFailedNotSet:
      'Token expiration is not set. Check security.jwt.expiration in the loaded security configuration.',
    mfaName: 'Multi-Factor Authentication',
    mfaAvailableDescription: 'Check if MFA is available and enforced',
    mfaEnabledDescription: 'Check if MFA is available and configured',
    mfaDetailPassed: 'MFA is required for all users',
    mfaDetailWarning:
      'MFA is available but not required. Consider enforcing MFA for enhanced security.',
    mfaEnabledDetailPassed: 'MFA is available for users',
  },
  'es-ES': {
    passwordName: 'Longitud mínima de la contraseña',
    passwordDescription:
      'Comprueba si la longitud mínima de la contraseña cumple los requisitos de seguridad (se recomienda 12+)',
    passwordDetailPassed:
      'La longitud mínima de la contraseña es de {length} caracteres (cumple el requisito)',
    passwordDetailWarning:
      'La longitud mínima de la contraseña es de {length} caracteres. Se recomienda aumentarla a 12+ para mayor seguridad.',
    passwordDetailFailed:
      'La longitud mínima de la contraseña ({length}) es demasiado corta. Se requieren al menos 8 caracteres y se recomiendan 12+.',
    tokenName: 'Tiempo de expiración del token',
    tokenDescription:
      'Comprueba si la expiración del token está configurada con una duración razonable',
    tokenDetailPassed: 'El token expira en {minutes} minutos',
    tokenDetailWarningHours:
      'La expiración del token ({hours} horas) es larga. Considera una duración más corta para operaciones sensibles.',
    tokenDetailWarningTooLong:
      'La expiración del token supera las 8 horas. Esto aumenta el riesgo de robo de tokens.',
    tokenDetailFailedNotSet:
      'La expiración del token no está configurada. Comprueba `security.jwt.expiration` en la configuración de seguridad cargada.',
    mfaName: 'Autenticación multifactor',
    mfaAvailableDescription: 'Comprueba si MFA está disponible y se aplica',
    mfaEnabledDescription: 'Comprueba si MFA está disponible y configurada',
    mfaDetailPassed: 'MFA es obligatoria para todos los usuarios',
    mfaDetailWarning:
      'MFA está disponible pero no es obligatoria. Considera exigir MFA para mejorar la seguridad.',
    mfaEnabledDetailPassed: 'MFA está disponible para los usuarios',
  },
  'fr-FR': {
    passwordName: 'Longueur minimale du mot de passe',
    passwordDescription:
      'Vérifie si la longueur minimale du mot de passe respecte les exigences de sécurité (12+ recommandé)',
    passwordDetailPassed:
      'La longueur minimale du mot de passe est de {length} caractères (exigence respectée)',
    passwordDetailWarning:
      'La longueur minimale du mot de passe est de {length} caractères. Il est recommandé de passer à 12+ pour une meilleure sécurité.',
    passwordDetailFailed:
      'La longueur minimale du mot de passe ({length}) est trop courte. Un minimum de 8 caractères est requis et 12+ est recommandé.',
    tokenName: "Durée d'expiration du jeton",
    tokenDescription:
      "Vérifie si l'expiration du jeton est définie sur une durée raisonnable",
    tokenDetailPassed: 'Le jeton expire dans {minutes} minutes',
    tokenDetailWarningHours:
      "La durée d'expiration du jeton ({hours} heures) est longue. Envisagez une durée plus courte pour les opérations sensibles.",
    tokenDetailWarningTooLong:
      "La durée d'expiration du jeton dépasse 8 heures. Cela augmente le risque de vol de jeton.",
    tokenDetailFailedNotSet:
      "L'expiration du jeton n'est pas définie. Vérifiez `security.jwt.expiration` dans la configuration de sécurité chargée.",
    mfaName: 'Authentification multifacteur',
    mfaAvailableDescription: 'Vérifie si la MFA est disponible et imposée',
    mfaEnabledDescription: 'Vérifie si la MFA est disponible et configurée',
    mfaDetailPassed: 'La MFA est obligatoire pour tous les utilisateurs',
    mfaDetailWarning:
      "La MFA est disponible mais pas obligatoire. Envisagez de l'imposer pour renforcer la sécurité.",
    mfaEnabledDetailPassed: 'La MFA est disponible pour les utilisateurs',
  },
  'ga-IE': {
    passwordName: 'Iosfhad an fhocail faire',
    passwordDescription:
      'Seiceálann sé an gcomhlíonann íosfhad an fhocail faire na riachtanais slándála (moltar 12+)',
    passwordDetailPassed:
      'Tá íosfhad an fhocail faire {length} carachtar (comhlíonann sé an riachtanas)',
    passwordDetailWarning:
      'Tá íosfhad an fhocail faire {length} carachtar. Moltar é a ardú go 12+ ar mhaithe le slándáil níos fearr.',
    passwordDetailFailed:
      'Tá íosfhad an fhocail faire ({length}) ró-ghearr. Tá 8 gcarachtar ar a laghad riachtanach, agus moltar 12+.',
    tokenName: 'Am éaga an chomhartha',
    tokenDescription:
      'Seiceálann sé an bhfuil éaga an chomhartha socraithe go fad réasúnta',
    tokenDetailPassed: 'Rachaidh an comhartha in éag i gceann {minutes} nóiméad',
    tokenDetailWarningHours:
      "Tá éaga an chomhartha ({hours} uair an chloig) fada. Smaoinigh ar thréimhse níos giorra d'oibríochtaí íogaire.",
    tokenDetailWarningTooLong:
      'Sáraíonn éaga an chomhartha 8 n-uaire an chloig. Méadaíonn sé sin an baol go ngoidfear comhartha.',
    tokenDetailFailedNotSet:
      'Níl éaga an chomhartha socraithe. Seiceáil `security.jwt.expiration` sa chumraíocht slándála luchtaithe.',
    mfaName: 'Fíordheimhniú ilfhachtóra',
    mfaAvailableDescription:
      'Seiceálann sé an bhfuil MFA ar fáil agus curtha i bhfeidhm',
    mfaEnabledDescription: 'Seiceálann sé an bhfuil MFA ar fáil agus cumraithe',
    mfaDetailPassed: 'Tá MFA riachtanach do gach úsáideoir',
    mfaDetailWarning:
      'Tá MFA ar fáil ach níl sé riachtanach. Smaoinigh ar MFA a fhorfheidhmiú le haghaidh slándála níos láidre.',
    mfaEnabledDetailPassed: "Tá MFA ar fáil d'úsáideoirí",
  },
  'hr-HR': {
    passwordName: 'Minimalna duljina lozinke',
    passwordDescription:
      'Provjerava zadovoljava li minimalna duljina lozinke sigurnosne zahtjeve (preporučeno 12+)',
    passwordDetailPassed:
      'Minimalna duljina lozinke je {length} znakova (zadovoljava zahtjev)',
    passwordDetailWarning:
      'Minimalna duljina lozinke je {length} znakova. Preporučuje se povećanje na 12+ radi bolje sigurnosti.',
    passwordDetailFailed:
      'Minimalna duljina lozinke ({length}) je prekratka. Potrebno je najmanje 8 znakova, a preporučuje se 12+.',
    tokenName: 'Vrijeme isteka tokena',
    tokenDescription: 'Provjerava je li istek tokena postavljen na razumno trajanje',
    tokenDetailPassed: 'Token istječe za {minutes} minuta',
    tokenDetailWarningHours:
      'Istek tokena ({hours} sati) je dug. Razmotrite kraće trajanje za osjetljive radnje.',
    tokenDetailWarningTooLong:
      'Istek tokena prelazi 8 sati. To povećava rizik od krađe tokena.',
    tokenDetailFailedNotSet:
      'Istek tokena nije postavljen. Provjerite `security.jwt.expiration` u učitanoj sigurnosnoj konfiguraciji.',
    mfaName: 'Višefaktorska autentikacija',
    mfaAvailableDescription: 'Provjerava je li MFA dostupna i obavezna',
    mfaEnabledDescription: 'Provjerava je li MFA dostupna i konfigurirana',
    mfaDetailPassed: 'MFA je obavezna za sve korisnike',
    mfaDetailWarning:
      'MFA je dostupna, ali nije obavezna. Razmotrite obveznu MFA radi bolje sigurnosti.',
    mfaEnabledDetailPassed: 'MFA je dostupna korisnicima',
  },
  'hu-HU': {
    passwordName: 'Jelszó minimális hossza',
    passwordDescription:
      'Ellenőrzi, hogy a jelszó minimális hossza megfelel-e a biztonsági követelményeknek (12+ ajánlott)',
    passwordDetailPassed:
      'A jelszó minimális hossza {length} karakter (megfelel a követelménynek)',
    passwordDetailWarning:
      'A jelszó minimális hossza {length} karakter. A jobb biztonság érdekében 12+ javasolt.',
    passwordDetailFailed:
      'A jelszó minimális hossza ({length}) túl rövid. Legalább 8 karakter szükséges, 12+ ajánlott.',
    tokenName: 'Token lejárati ideje',
    tokenDescription:
      'Ellenőrzi, hogy a token lejárata ésszerű időtartamra van-e beállítva',
    tokenDetailPassed: 'A token {minutes} perc múlva lejár',
    tokenDetailWarningHours:
      'A token lejárati ideje ({hours} óra) hosszú. Érzékeny műveleteknél rövidebb időtartam javasolt.',
    tokenDetailWarningTooLong:
      'A token lejárati ideje meghaladja a 8 órát. Ez növeli a tokenlopás kockázatát.',
    tokenDetailFailedNotSet:
      'A token lejárata nincs beállítva. Ellenőrizze a `security.jwt.expiration` értékét a betöltött biztonsági konfigurációban.',
    mfaName: 'Többtényezős hitelesítés',
    mfaAvailableDescription: 'Ellenőrzi, hogy az MFA elérhető és kikényszerített-e',
    mfaEnabledDescription: 'Ellenőrzi, hogy az MFA elérhető és konfigurált-e',
    mfaDetailPassed: 'Az MFA minden felhasználó számára kötelező',
    mfaDetailWarning:
      'Az MFA elérhető, de nem kötelező. A jobb biztonság érdekében érdemes kötelezővé tenni.',
    mfaEnabledDetailPassed: 'Az MFA elérhető a felhasználók számára',
  },
  'it-IT': {
    passwordName: 'Lunghezza minima della password',
    passwordDescription:
      'Verifica se la lunghezza minima della password soddisfa i requisiti di sicurezza (12+ consigliati)',
    passwordDetailPassed:
      'La lunghezza minima della password è di {length} caratteri (requisito soddisfatto)',
    passwordDetailWarning:
      'La lunghezza minima della password è di {length} caratteri. Si consiglia di aumentarla a 12+ per una sicurezza migliore.',
    passwordDetailFailed:
      'La lunghezza minima della password ({length}) è troppo corta. Sono richiesti almeno 8 caratteri e se ne consigliano 12+.',
    tokenName: 'Tempo di scadenza del token',
    tokenDescription:
      'Verifica se la scadenza del token è impostata su una durata ragionevole',
    tokenDetailPassed: 'Il token scade tra {minutes} minuti',
    tokenDetailWarningHours:
      'La scadenza del token ({hours} ore) è lunga. Valuta una durata più breve per le operazioni sensibili.',
    tokenDetailWarningTooLong:
      'La scadenza del token supera le 8 ore. Questo aumenta il rischio di furto del token.',
    tokenDetailFailedNotSet:
      'La scadenza del token non è impostata. Controlla `security.jwt.expiration` nella configurazione di sicurezza caricata.',
    mfaName: 'Autenticazione a più fattori',
    mfaAvailableDescription: 'Verifica se MFA è disponibile ed è applicata',
    mfaEnabledDescription: 'Verifica se MFA è disponibile e configurata',
    mfaDetailPassed: 'MFA è obbligatoria per tutti gli utenti',
    mfaDetailWarning:
      'MFA è disponibile ma non obbligatoria. Valuta di imporla per una sicurezza maggiore.',
    mfaEnabledDetailPassed: 'MFA è disponibile per gli utenti',
  },
  'ja-JP': {
    passwordName: 'パスワード最小文字数',
    passwordDescription:
      'パスワードの最小文字数がセキュリティ要件を満たしているかを確認します（12 文字以上推奨）',
    passwordDetailPassed:
      'パスワードの最小文字数は {length} 文字です（要件を満たしています）',
    passwordDetailWarning:
      'パスワードの最小文字数は {length} 文字です。より安全にするため 12 文字以上を推奨します。',
    passwordDetailFailed:
      'パスワードの最小文字数 ({length}) は短すぎます。最低 8 文字が必要で、12 文字以上を推奨します。',
    tokenName: 'トークンの有効期限',
    tokenDescription: 'トークンの有効期限が適切な長さに設定されているかを確認します',
    tokenDetailPassed: 'トークンは {minutes} 分で期限切れになります',
    tokenDetailWarningHours:
      'トークンの有効期限（{hours} 時間）は長めです。機密性の高い操作では、より短い時間を検討してください。',
    tokenDetailWarningTooLong:
      'トークンの有効期限が 8 時間を超えています。トークン盗難のリスクが高まります。',
    tokenDetailFailedNotSet:
      'トークンの有効期限が設定されていません。読み込まれたセキュリティ設定の `security.jwt.expiration` を確認してください。',
    mfaName: '多要素認証',
    mfaAvailableDescription: 'MFA が利用可能で強制されているかを確認します',
    mfaEnabledDescription: 'MFA が利用可能で設定されているかを確認します',
    mfaDetailPassed: 'すべてのユーザーに MFA が必須です',
    mfaDetailWarning:
      'MFA は利用可能ですが必須ではありません。セキュリティ向上のため、MFA の強制を検討してください。',
    mfaEnabledDetailPassed: 'MFA はユーザーが利用できます',
  },
  'ko-KR': {
    passwordName: '비밀번호 최소 길이',
    passwordDescription:
      '비밀번호 최소 길이가 보안 요구 사항을 충족하는지 확인합니다(12자 이상 권장)',
    passwordDetailPassed: '비밀번호 최소 길이는 {length}자입니다(요구 사항 충족)',
    passwordDetailWarning:
      '비밀번호 최소 길이는 {length}자입니다. 더 나은 보안을 위해 12자 이상으로 늘리는 것을 권장합니다.',
    passwordDetailFailed:
      '비밀번호 최소 길이({length}자)가 너무 짧습니다. 최소 8자가 필요하며 12자 이상을 권장합니다.',
    tokenName: '토큰 만료 시간',
    tokenDescription: '토큰 만료 기간이 적절하게 설정되어 있는지 확인합니다',
    tokenDetailPassed: '토큰은 {minutes}분 후 만료됩니다',
    tokenDetailWarningHours:
      '토큰 만료 기간({hours}시간)이 깁니다. 민감한 작업에는 더 짧은 기간을 고려하세요.',
    tokenDetailWarningTooLong:
      '토큰 만료 기간이 8시간을 초과합니다. 이는 토큰 도난 위험을 높입니다.',
    tokenDetailFailedNotSet:
      '토큰 만료 시간이 설정되지 않았습니다. 로드된 보안 설정의 `security.jwt.expiration`을 확인하세요.',
    mfaName: '다단계 인증',
    mfaAvailableDescription: 'MFA를 사용할 수 있고 강제되는지 확인합니다',
    mfaEnabledDescription: 'MFA를 사용할 수 있고 구성되어 있는지 확인합니다',
    mfaDetailPassed: '모든 사용자에게 MFA가 필요합니다',
    mfaDetailWarning:
      'MFA를 사용할 수 있지만 필수는 아닙니다. 보안 강화를 위해 MFA 강제를 고려하세요.',
    mfaEnabledDetailPassed: '사용자가 MFA를 사용할 수 있습니다',
  },
  'ml-IN': {
    passwordName: 'പാസ്‌വേഡിന്റെ കുറഞ്ഞ നീളം',
    passwordDescription:
      'പാസ്‌വേഡിന്റെ കുറഞ്ഞ നീളം സുരക്ഷാ ആവശ്യകതകൾ പാലിക്കുന്നുണ്ടോ എന്ന് പരിശോധിക്കുക (12+ ശുപാർശ ചെയ്യുന്നു)',
    passwordDetailPassed:
      'പാസ്‌വേഡിന്റെ കുറഞ്ഞ നീളം {length} അക്ഷരങ്ങളാണ് (ആവശ്യകത നിറവേറ്റുന്നു)',
    passwordDetailWarning:
      'പാസ്‌വേഡിന്റെ കുറഞ്ഞ നീളം {length} അക്ഷരങ്ങളാണ്. കൂടുതൽ സുരക്ഷയ്ക്കായി 12+ ആയി വർധിപ്പിക്കാൻ ശുപാർശ ചെയ്യുന്നു.',
    passwordDetailFailed:
      'പാസ്‌വേഡിന്റെ കുറഞ്ഞ നീളം ({length}) വളരെ കുറവാണ്. കുറഞ്ഞത് 8 അക്ഷരങ്ങൾ ആവശ്യമാണ്, 12+ ശുപാർശ ചെയ്യുന്നു.',
    tokenName: 'ടോക്കൺ കാലഹരണ സമയം',
    tokenDescription:
      'ടോക്കൺ കാലാവധി യുക്തിസഹമായ ദൈർഘ്യത്തിൽ സജ്ജീകരിച്ചിട്ടുണ്ടോ എന്ന് പരിശോധിക്കുക',
    tokenDetailPassed: 'ടോക്കൺ {minutes} മിനിറ്റിനകം കാലഹരണപ്പെടും',
    tokenDetailWarningHours:
      'ടോക്കൺ കാലാവധി ({hours} മണിക്കൂർ) ദൈർഘ്യമേറിയതാണ്. സെൻസിറ്റീവ് പ്രവർത്തനങ്ങൾക്ക് കുറഞ്ഞ ദൈർഘ്യം പരിഗണിക്കുക.',
    tokenDetailWarningTooLong:
      'ടോക്കൺ കാലാവധി 8 മണിക്കൂറിന് മുകളിലാണ്. ഇത് ടോക്കൺ മോഷണത്തിന്റെ അപകടസാധ്യത ഉയർത്തുന്നു.',
    tokenDetailFailedNotSet:
      'ടോക്കൺ കാലാവധി സജ്ജീകരിച്ചിട്ടില്ല. ലോഡ് ചെയ്ത സുരക്ഷാ കോൺഫിഗറേഷനിലെ `security.jwt.expiration` പരിശോധിക്കുക.',
    mfaName: 'മൾട്ടി-ഫാക്ടർ ഓതന്റിക്കേഷൻ',
    mfaAvailableDescription:
      'MFA ലഭ്യമാണോയും നിർബന്ധിതമാക്കിയിട്ടുണ്ടോയും പരിശോധിക്കുക',
    mfaEnabledDescription: 'MFA ലഭ്യമാണോയും കോൺഫിഗർ ചെയ്തിട്ടുണ്ടോയും പരിശോധിക്കുക',
    mfaDetailPassed: 'എല്ലാ ഉപയോക്താക്കൾക്കും MFA നിർബന്ധമാണ്',
    mfaDetailWarning:
      'MFA ലഭ്യമാണ്, പക്ഷേ നിർബന്ധമല്ല. കൂടുതൽ സുരക്ഷയ്ക്കായി MFA നിർബന്ധമാക്കുന്നത് പരിഗണിക്കുക.',
    mfaEnabledDetailPassed: 'ഉപയോക്താക്കൾക്ക് MFA ലഭ്യമാണ്',
  },
  'nb-NO': {
    passwordName: 'Minimum passordlengde',
    passwordDescription:
      'Kontroller om minimum passordlengde oppfyller sikkerhetskravene (12+ anbefales)',
    passwordDetailPassed: 'Minimum passordlengde er {length} tegn (oppfyller kravet)',
    passwordDetailWarning:
      'Minimum passordlengde er {length} tegn. Det anbefales å øke til 12+ for bedre sikkerhet.',
    passwordDetailFailed:
      'Minimum passordlengde ({length}) er for kort. Minst 8 tegn kreves, og 12+ anbefales.',
    tokenName: 'Utløpstid for token',
    tokenDescription: 'Kontroller om tokenets utløpstid er satt til en rimelig varighet',
    tokenDetailPassed: 'Tokenet utløper om {minutes} minutter',
    tokenDetailWarningHours:
      'Tokenets utløpstid ({hours} timer) er lang. Vurder kortere varighet for sensitive operasjoner.',
    tokenDetailWarningTooLong:
      'Tokenets utløpstid overstiger 8 timer. Dette øker risikoen for token-tyveri.',
    tokenDetailFailedNotSet:
      'Tokenets utløpstid er ikke satt. Kontroller `security.jwt.expiration` i den lastede sikkerhetskonfigurasjonen.',
    mfaName: 'Flerfaktorautentisering',
    mfaAvailableDescription: 'Kontroller om MFA er tilgjengelig og håndheves',
    mfaEnabledDescription: 'Kontroller om MFA er tilgjengelig og konfigurert',
    mfaDetailPassed: 'MFA er påkrevd for alle brukere',
    mfaDetailWarning:
      'MFA er tilgjengelig, men ikke påkrevd. Vurder å håndheve MFA for bedre sikkerhet.',
    mfaEnabledDetailPassed: 'MFA er tilgjengelig for brukere',
  },
  'nl-NL': {
    passwordName: 'Minimale wachtwoordlengte',
    passwordDescription:
      'Controleer of de minimale wachtwoordlengte voldoet aan de beveiligingseisen (12+ aanbevolen)',
    passwordDetailPassed:
      'De minimale wachtwoordlengte is {length} tekens (voldoet aan de eis)',
    passwordDetailWarning:
      'De minimale wachtwoordlengte is {length} tekens. Verhoog dit bij voorkeur naar 12+ voor betere beveiliging.',
    passwordDetailFailed:
      'De minimale wachtwoordlengte ({length}) is te kort. Minimaal 8 tekens zijn vereist, 12+ wordt aanbevolen.',
    tokenName: 'Verlooptijd van token',
    tokenDescription:
      'Controleer of de verlooptijd van het token op een redelijke duur is ingesteld',
    tokenDetailPassed: 'Het token verloopt over {minutes} minuten',
    tokenDetailWarningHours:
      'De verlooptijd van het token ({hours} uur) is lang. Overweeg een kortere duur voor gevoelige handelingen.',
    tokenDetailWarningTooLong:
      'De verlooptijd van het token is langer dan 8 uur. Dit vergroot het risico op tokendiefstal.',
    tokenDetailFailedNotSet:
      'De verlooptijd van het token is niet ingesteld. Controleer `security.jwt.expiration` in de geladen beveiligingsconfiguratie.',
    mfaName: 'Multi-factor-authenticatie',
    mfaAvailableDescription: 'Controleer of MFA beschikbaar is en wordt afgedwongen',
    mfaEnabledDescription: 'Controleer of MFA beschikbaar is en geconfigureerd is',
    mfaDetailPassed: 'MFA is vereist voor alle gebruikers',
    mfaDetailWarning:
      'MFA is beschikbaar maar niet verplicht. Overweeg MFA af te dwingen voor betere beveiliging.',
    mfaEnabledDetailPassed: 'MFA is beschikbaar voor gebruikers',
  },
  'pl-PL': {
    passwordName: 'Minimalna długość hasła',
    passwordDescription:
      'Sprawdza, czy minimalna długość hasła spełnia wymagania bezpieczeństwa (zalecane 12+)',
    passwordDetailPassed:
      'Minimalna długość hasła to {length} znaków (spełnia wymaganie)',
    passwordDetailWarning:
      'Minimalna długość hasła to {length} znaków. Dla lepszego bezpieczeństwa zaleca się zwiększenie do 12+.',
    passwordDetailFailed:
      'Minimalna długość hasła ({length}) jest zbyt mała. Wymagane jest co najmniej 8 znaków, zalecane 12+.',
    tokenName: 'Czas wygaśnięcia tokenu',
    tokenDescription:
      'Sprawdza, czy czas wygaśnięcia tokenu jest ustawiony na rozsądny okres',
    tokenDetailPassed: 'Token wygaśnie za {minutes} minut',
    tokenDetailWarningHours:
      'Czas wygaśnięcia tokenu ({hours} godzin) jest długi. Rozważ krótszy okres dla wrażliwych operacji.',
    tokenDetailWarningTooLong:
      'Czas wygaśnięcia tokenu przekracza 8 godzin. Zwiększa to ryzyko kradzieży tokenu.',
    tokenDetailFailedNotSet:
      'Czas wygaśnięcia tokenu nie jest ustawiony. Sprawdź `security.jwt.expiration` w załadowanej konfiguracji bezpieczeństwa.',
    mfaName: 'Uwierzytelnianie wieloskładnikowe',
    mfaAvailableDescription: 'Sprawdza, czy MFA jest dostępne i wymuszane',
    mfaEnabledDescription: 'Sprawdza, czy MFA jest dostępne i skonfigurowane',
    mfaDetailPassed: 'MFA jest wymagane dla wszystkich użytkowników',
    mfaDetailWarning:
      'MFA jest dostępne, ale nie jest wymagane. Rozważ wymuszenie MFA dla lepszego bezpieczeństwa.',
    mfaEnabledDetailPassed: 'MFA jest dostępne dla użytkowników',
  },
  'pt-BR': {
    passwordName: 'Comprimento mínimo da senha',
    passwordDescription:
      'Verifica se o comprimento mínimo da senha atende aos requisitos de segurança (12+ recomendado)',
    passwordDetailPassed:
      'O comprimento mínimo da senha é de {length} caracteres (atende ao requisito)',
    passwordDetailWarning:
      'O comprimento mínimo da senha é de {length} caracteres. Recomenda-se aumentar para 12+ para melhorar a segurança.',
    passwordDetailFailed:
      'O comprimento mínimo da senha ({length}) é muito curto. São necessários pelo menos 8 caracteres, e 12+ é recomendado.',
    tokenName: 'Tempo de expiração do token',
    tokenDescription:
      'Verifica se a expiração do token está configurada com uma duração razoável',
    tokenDetailPassed: 'O token expira em {minutes} minutos',
    tokenDetailWarningHours:
      'A expiração do token ({hours} horas) é longa. Considere uma duração menor para operações sensíveis.',
    tokenDetailWarningTooLong:
      'A expiração do token excede 8 horas. Isso aumenta o risco de roubo de token.',
    tokenDetailFailedNotSet:
      'A expiração do token não está configurada. Verifique `security.jwt.expiration` na configuração de segurança carregada.',
    mfaName: 'Autenticação multifator',
    mfaAvailableDescription: 'Verifica se o MFA está disponível e é exigido',
    mfaEnabledDescription: 'Verifica se o MFA está disponível e configurado',
    mfaDetailPassed: 'MFA é obrigatório para todos os usuários',
    mfaDetailWarning:
      'MFA está disponível, mas não é obrigatório. Considere exigir MFA para aumentar a segurança.',
    mfaEnabledDetailPassed: 'MFA está disponível para os usuários',
  },
  'pt-PT': {
    passwordName: 'Comprimento mínimo da palavra-passe',
    passwordDescription:
      'Verifica se o comprimento mínimo da palavra-passe cumpre os requisitos de segurança (12+ recomendado)',
    passwordDetailPassed:
      'O comprimento mínimo da palavra-passe é de {length} caracteres (cumpre o requisito)',
    passwordDetailWarning:
      'O comprimento mínimo da palavra-passe é de {length} caracteres. Recomenda-se aumentar para 12+ para melhorar a segurança.',
    passwordDetailFailed:
      'O comprimento mínimo da palavra-passe ({length}) é demasiado curto. São necessários pelo menos 8 caracteres e 12+ é recomendado.',
    tokenName: 'Tempo de expiração do token',
    tokenDescription:
      'Verifica se a expiração do token está definida para uma duração razoável',
    tokenDetailPassed: 'O token expira em {minutes} minutos',
    tokenDetailWarningHours:
      'A expiração do token ({hours} horas) é longa. Considere uma duração mais curta para operações sensíveis.',
    tokenDetailWarningTooLong:
      'A expiração do token excede 8 horas. Isto aumenta o risco de roubo do token.',
    tokenDetailFailedNotSet:
      'A expiração do token não está definida. Verifique `security.jwt.expiration` na configuração de segurança carregada.',
    mfaName: 'Autenticação multifator',
    mfaAvailableDescription: 'Verifica se o MFA está disponível e é imposto',
    mfaEnabledDescription: 'Verifica se o MFA está disponível e configurado',
    mfaDetailPassed: 'O MFA é obrigatório para todos os utilizadores',
    mfaDetailWarning:
      'O MFA está disponível, mas não é obrigatório. Considere impor MFA para reforçar a segurança.',
    mfaEnabledDetailPassed: 'O MFA está disponível para os utilizadores',
  },
  'ro-RO': {
    passwordName: 'Lungimea minimă a parolei',
    passwordDescription:
      'Verifică dacă lungimea minimă a parolei respectă cerințele de securitate (12+ recomandat)',
    passwordDetailPassed:
      'Lungimea minimă a parolei este de {length} caractere (respectă cerința)',
    passwordDetailWarning:
      'Lungimea minimă a parolei este de {length} caractere. Se recomandă creșterea la 12+ pentru o securitate mai bună.',
    passwordDetailFailed:
      'Lungimea minimă a parolei ({length}) este prea mică. Sunt necesare cel puțin 8 caractere, iar 12+ este recomandat.',
    tokenName: 'Timpul de expirare al tokenului',
    tokenDescription:
      'Verifică dacă expirarea tokenului este setată la o durată rezonabilă',
    tokenDetailPassed: 'Tokenul expiră în {minutes} minute',
    tokenDetailWarningHours:
      'Expirarea tokenului ({hours} ore) este lungă. Ia în considerare o durată mai scurtă pentru operațiunile sensibile.',
    tokenDetailWarningTooLong:
      'Expirarea tokenului depășește 8 ore. Acest lucru crește riscul de furt al tokenului.',
    tokenDetailFailedNotSet:
      'Expirarea tokenului nu este setată. Verifică `security.jwt.expiration` în configurația de securitate încărcată.',
    mfaName: 'Autentificare multifactor',
    mfaAvailableDescription: 'Verifică dacă MFA este disponibilă și impusă',
    mfaEnabledDescription: 'Verifică dacă MFA este disponibilă și configurată',
    mfaDetailPassed: 'MFA este obligatorie pentru toți utilizatorii',
    mfaDetailWarning:
      'MFA este disponibilă, dar nu este obligatorie. Ia în considerare impunerea MFA pentru o securitate sporită.',
    mfaEnabledDetailPassed: 'MFA este disponibilă pentru utilizatori',
  },
  'ru-RU': {
    passwordName: 'Минимальная длина пароля',
    passwordDescription:
      'Проверяет, соответствует ли минимальная длина пароля требованиям безопасности (рекомендуется 12+)',
    passwordDetailPassed:
      'Минимальная длина пароля составляет {length} символов (требование выполнено)',
    passwordDetailWarning:
      'Минимальная длина пароля составляет {length} символов. Для лучшей безопасности рекомендуется увеличить до 12+.',
    passwordDetailFailed:
      'Минимальная длина пароля ({length}) слишком короткая. Требуется минимум 8 символов, рекомендуется 12+.',
    tokenName: 'Время истечения токена',
    tokenDescription:
      'Проверяет, установлено ли время истечения токена на разумный срок',
    tokenDetailPassed: 'Срок действия токена истекает через {minutes} минут',
    tokenDetailWarningHours:
      'Время истечения токена ({hours} часов) слишком велико. Для чувствительных операций стоит выбрать более короткий срок.',
    tokenDetailWarningTooLong:
      'Время истечения токена превышает 8 часов. Это повышает риск кражи токена.',
    tokenDetailFailedNotSet:
      'Время истечения токена не задано. Проверьте `security.jwt.expiration` в загруженной конфигурации безопасности.',
    mfaName: 'Многофакторная аутентификация',
    mfaAvailableDescription: 'Проверяет, доступна ли MFA и принудительно ли она включена',
    mfaEnabledDescription: 'Проверяет, доступна ли MFA и настроена ли она',
    mfaDetailPassed: 'MFA обязательна для всех пользователей',
    mfaDetailWarning:
      'MFA доступна, но не обязательна. Рассмотрите принудительное включение MFA для усиления безопасности.',
    mfaEnabledDetailPassed: 'MFA доступна пользователям',
  },
  'sk-SK': {
    passwordName: 'Minimálna dĺžka hesla',
    passwordDescription:
      'Kontroluje, či minimálna dĺžka hesla spĺňa bezpečnostné požiadavky (odporúča sa 12+)',
    passwordDetailPassed:
      'Minimálna dĺžka hesla je {length} znakov (spĺňa požiadavku)',
    passwordDetailWarning:
      'Minimálna dĺžka hesla je {length} znakov. Pre lepšiu bezpečnosť sa odporúča zvýšiť ju na 12+.',
    passwordDetailFailed:
      'Minimálna dĺžka hesla ({length}) je príliš krátka. Vyžaduje sa aspoň 8 znakov, odporúča sa 12+.',
    tokenName: 'Čas vypršania tokenu',
    tokenDescription: 'Kontroluje, či je vypršanie tokenu nastavené na primeranú dobu',
    tokenDetailPassed: 'Token vyprší o {minutes} minút',
    tokenDetailWarningHours:
      'Vypršanie tokenu ({hours} hodín) je dlhé. Pri citlivých operáciách zvážte kratšiu dobu.',
    tokenDetailWarningTooLong:
      'Vypršanie tokenu presahuje 8 hodín. Zvyšuje to riziko krádeže tokenu.',
    tokenDetailFailedNotSet:
      'Vypršanie tokenu nie je nastavené. Skontrolujte `security.jwt.expiration` v načítanej bezpečnostnej konfigurácii.',
    mfaName: 'Viacfaktorové overenie',
    mfaAvailableDescription: 'Kontroluje, či je MFA dostupné a vynucované',
    mfaEnabledDescription: 'Kontroluje, či je MFA dostupné a nakonfigurované',
    mfaDetailPassed: 'MFA je povinné pre všetkých používateľov',
    mfaDetailWarning:
      'MFA je dostupné, ale nie je povinné. Zvážte jeho vynútenie pre vyššiu bezpečnosť.',
    mfaEnabledDetailPassed: 'MFA je dostupné pre používateľov',
  },
  'sv-SE': {
    passwordName: 'Minsta lösenordslängd',
    passwordDescription:
      'Kontrollerar om minsta lösenordslängd uppfyller säkerhetskraven (12+ rekommenderas)',
    passwordDetailPassed: 'Minsta lösenordslängd är {length} tecken (uppfyller kravet)',
    passwordDetailWarning:
      'Minsta lösenordslängd är {length} tecken. Det rekommenderas att öka till 12+ för bättre säkerhet.',
    passwordDetailFailed:
      'Minsta lösenordslängd ({length}) är för kort. Minst 8 tecken krävs och 12+ rekommenderas.',
    tokenName: 'Tokenets utgångstid',
    tokenDescription:
      'Kontrollerar om tokenets utgångstid är satt till en rimlig varaktighet',
    tokenDetailPassed: 'Tokenet går ut om {minutes} minuter',
    tokenDetailWarningHours:
      'Tokenets utgångstid ({hours} timmar) är lång. Överväg en kortare varaktighet för känsliga operationer.',
    tokenDetailWarningTooLong:
      'Tokenets utgångstid överstiger 8 timmar. Det ökar risken för tokenstöld.',
    tokenDetailFailedNotSet:
      'Tokenets utgångstid är inte satt. Kontrollera `security.jwt.expiration` i den inlästa säkerhetskonfigurationen.',
    mfaName: 'Flerfaktorsautentisering',
    mfaAvailableDescription: 'Kontrollerar om MFA är tillgängligt och krävs',
    mfaEnabledDescription: 'Kontrollerar om MFA är tillgängligt och konfigurerat',
    mfaDetailPassed: 'MFA krävs för alla användare',
    mfaDetailWarning:
      'MFA är tillgängligt men inte obligatoriskt. Överväg att kräva MFA för bättre säkerhet.',
    mfaEnabledDetailPassed: 'MFA är tillgängligt för användare',
  },
  'zh-CN': {
    passwordName: '密码最小长度',
    passwordDescription: '检查密码最小长度是否符合安全要求（建议 12 位以上）',
    passwordDetailPassed: '当前密码最小长度为 {length} 位（符合要求）',
    passwordDetailWarning:
      '当前密码最小长度为 {length} 位。建议提升到 12 位以上以增强安全性。',
    passwordDetailFailed:
      '当前密码最小长度（{length} 位）过短。至少需要 8 位，建议 12 位以上。',
    tokenName: 'Token 过期时间',
    tokenDescription: '检查 Token 过期时间是否设置在合理范围内',
    tokenDetailPassed: 'Token 将在 {minutes} 分钟后过期',
    tokenDetailWarningHours:
      'Token 过期时间（{hours} 小时）偏长。对于敏感操作，建议缩短时长。',
    tokenDetailWarningTooLong:
      'Token 过期时间超过 8 小时，这会增加 Token 被盗用的风险。',
    tokenDetailFailedNotSet:
      '未设置 Token 过期时间。请检查已加载安全配置中的 `security.jwt.expiration`。',
    mfaName: '多因素认证',
    mfaAvailableDescription: '检查是否提供并强制执行 MFA',
    mfaEnabledDescription: '检查是否提供并配置了 MFA',
    mfaDetailPassed: '已为所有用户强制启用 MFA',
    mfaDetailWarning:
      '已提供 MFA，但尚未强制执行。建议启用强制 MFA 以提升安全性。',
    mfaEnabledDetailPassed: '用户可使用 MFA',
  },
  'zh-TW': {
    passwordName: '密碼最小長度',
    passwordDescription: '檢查密碼最小長度是否符合安全要求（建議 12 位以上）',
    passwordDetailPassed: '目前密碼最小長度為 {length} 位（符合要求）',
    passwordDetailWarning:
      '目前密碼最小長度為 {length} 位。建議提升到 12 位以上以增強安全性。',
    passwordDetailFailed:
      '目前密碼最小長度（{length} 位）過短。至少需要 8 位，建議 12 位以上。',
    tokenName: 'Token 到期時間',
    tokenDescription: '檢查 Token 到期時間是否設定在合理範圍內',
    tokenDetailPassed: 'Token 將在 {minutes} 分鐘後到期',
    tokenDetailWarningHours:
      'Token 到期時間（{hours} 小時）偏長。對於敏感操作，建議縮短時長。',
    tokenDetailWarningTooLong:
      'Token 到期時間超過 8 小時，這會增加 Token 被盜用的風險。',
    tokenDetailFailedNotSet:
      '尚未設定 Token 到期時間。請檢查已載入安全設定中的 `security.jwt.expiration`。',
    mfaName: '多因素驗證',
    mfaAvailableDescription: '檢查 MFA 是否可用並已強制執行',
    mfaEnabledDescription: '檢查 MFA 是否可用並已設定',
    mfaDetailPassed: '已為所有使用者強制啟用 MFA',
    mfaDetailWarning:
      'MFA 已可用，但尚未強制執行。建議啟用強制 MFA 以提升安全性。',
    mfaEnabledDetailPassed: '使用者可使用 MFA',
  },
}

function buildLocalePatch(
  localeKey: LocaleKey,
  terms: LocaleTerms,
  itemTerms: SecurityScanItemTerms
): object {
  return {
    security: {
      scan: {
        detailMessages: {
          sandbox_enabled: terms.sandboxEnabled,
        },
        items: {
          auth_password_length: {
            name: itemTerms.passwordName,
            description: itemTerms.passwordDescription,
            details: {
              passed: itemTerms.passwordDetailPassed,
              warning: itemTerms.passwordDetailWarning,
              failed: itemTerms.passwordDetailFailed,
            },
          },
          auth_token_expiration: {
            name: itemTerms.tokenName,
            description: itemTerms.tokenDescription,
            details: {
              passed: itemTerms.tokenDetailPassed,
              warningHours: itemTerms.tokenDetailWarningHours,
              warningTooLong: itemTerms.tokenDetailWarningTooLong,
              failedNotSet: itemTerms.tokenDetailFailedNotSet,
            },
          },
          auth_mfa_available: {
            name: itemTerms.mfaName,
            description: itemTerms.mfaAvailableDescription,
            details: {
              passed: itemTerms.mfaDetailPassed,
              warning: itemTerms.mfaDetailWarning,
            },
          },
          auth_mfa_enabled: {
            name: itemTerms.mfaName,
            description: itemTerms.mfaEnabledDescription,
            details: {
              passed: itemTerms.mfaEnabledDetailPassed,
            },
          },
        },
      },
    },
    apiProxy: {
      maskingRuleNames: {
        email: terms.email,
        phone: terms.phone,
        ssn: terms.ssn,
        api_key_format: terms.apiKeyFormat,
        api_key: terms.apiKey,
        bearer_token: terms.bearerToken,
        credit_card: terms.creditCard,
      },
      maskingCategories: {
        pii: terms.pii,
        credentials: terms.credentials,
        financial: terms.financial,
      },
      maskingDirections: {
        request: terms.request,
        response: terms.response,
        both: terms.both,
      },
    },
    cron: {
      systemJobs: {
        knowledgeNightlyLint: {
          name: terms.knowledgeName,
          description: terms.knowledgeDescription,
        },
      },
    },
    providerPool: {
      tabs: providerPoolTabs[localeKey],
    },
  }
}

const securityCronMaskingBackfills = Object.fromEntries(
  Object.entries(localeTerms).map(([localeKey, terms]) => [
    localeKey,
    buildLocalePatch(
      localeKey as LocaleKey,
      terms,
      securityScanItemTerms[localeKey as LocaleKey]
    ),
  ])
) as Partial<Record<LocaleKey, object>>

export default securityCronMaskingBackfills
