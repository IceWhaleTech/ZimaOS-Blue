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

function buildLocalePatch(localeKey: LocaleKey, terms: LocaleTerms): object {
  return {
    security: {
      scan: {
        detailMessages: {
          sandbox_enabled: terms.sandboxEnabled,
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
    buildLocalePatch(localeKey as LocaleKey, terms),
  ])
) as Partial<Record<LocaleKey, object>>

export default securityCronMaskingBackfills
