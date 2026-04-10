import type { LocaleKey } from './locale-catalog'

export type HarnessProviderRemediationStrings = readonly [
  remediationInfraProviderAuth: string,
  remediationInfraProviderQuota: string,
  remediationInfraProviderBlocked: string,
]

export const harnessProviderRemediationCopyByLocale: Record<
  LocaleKey,
  HarnessProviderRemediationStrings
> = {
  'ca-ES': [
    "Torna a connectar les credencials del proveïdor, confirma que la clau API o els encapçalaments d'autenticació són vàlids i torna a provar l'execució.",
    'Restaura la quota o el saldo del proveïdor, o deriva el cas a un altre proveïdor amb saldo abans de tornar-ho a provar.',
    "Inspecciona els senyals de disponibilitat, sobrecàrrega o limitació de taxa del proveïdor a les execucions enllaçades i torna-ho a provar quan la ruta del proveïdor estigui sana.",
  ],
  'cs-CZ': [
    'Znovu připojte přihlašovací údaje poskytovatele, ověřte platnost API klíče nebo autorizačních hlaviček a poté běh opakujte.',
    'Obnovte kvótu nebo kredit poskytovatele, případně přesměrujte případ na jiného financovaného poskytovatele, než běh zopakujete.',
    'Zkontrolujte v propojených bězích signály dostupnosti poskytovatele, přetížení nebo omezení rychlosti a běh zopakujte, až bude cesta poskytovatele v pořádku.',
  ],
  'da-DK': [
    'Tilslut udbyderens legitimationsoplysninger igen, bekræft at API-nøgler eller godkendelsesheaders er gyldige, og prøv derefter kørslen igen.',
    'Gendan udbyderens kvote eller kredit, eller rout sagen til en anden finansieret udbyder, før du prøver igen.',
    'Undersøg signaler om udbydertilgængelighed, overbelastning eller hastighedsbegrænsning i tilknyttede kørsler, og prøv igen, når udbyderstien er sund.',
  ],
  'de-DE': [
    'Verbinden Sie die Anbieter-Anmeldedaten erneut, prüfen Sie, ob API-Schlüssel oder Auth-Header gültig sind, und wiederholen Sie dann den Lauf.',
    'Stellen Sie Anbieter-Kontingent oder Guthaben wieder her oder leiten Sie den Fall vor dem erneuten Versuch an einen anderen finanzierten Anbieter weiter.',
    'Prüfen Sie in den verknüpften Läufen Signale zur Anbieterverfügbarkeit, Überlastung oder Ratenbegrenzung und wiederholen Sie den Versuch, wenn der Anbieterpfad wieder stabil ist.',
  ],
  'el-GR': [
    'Συνδέστε ξανά τα διαπιστευτήρια του παρόχου, επιβεβαιώστε ότι το API key ή οι κεφαλίδες αυθεντικοποίησης είναι έγκυρες και έπειτα επαναλάβετε την εκτέλεση.',
    'Αποκαταστήστε το quota ή το υπόλοιπο του παρόχου ή δρομολογήστε την περίπτωση σε άλλον χρηματοδοτημένο πάροχο πριν δοκιμάσετε ξανά.',
    'Ελέγξτε στα συνδεδεμένα runs τα σήματα διαθεσιμότητας παρόχου, υπερφόρτωσης ή ορίου ρυθμού και επαναλάβετε όταν η διαδρομή του παρόχου είναι ξανά υγιής.',
  ],
  'en-GB': [
    'Reconnect provider credentials, confirm API keys or auth headers are valid, then retry the run.',
    'Restore provider quota or credits, or route the case to another funded provider before retrying.',
    'Inspect provider availability, overload, or rate-limit signals in linked runs, then retry when the provider path is healthy.',
  ],
  'en-US': [
    'Reconnect provider credentials, confirm API keys or auth headers are valid, then retry the run.',
    'Restore provider quota or credits, or route the case to another funded provider before retrying.',
    'Inspect provider availability, overload, or rate-limit signals in linked runs, then retry when the provider path is healthy.',
  ],
  'es-ES': [
    'Vuelve a conectar las credenciales del proveedor, confirma que la clave API o las cabeceras de autenticación son válidas y vuelve a intentar la ejecución.',
    'Restablece la cuota o el saldo del proveedor, o redirige el caso a otro proveedor con fondos antes de reintentar.',
    'Inspecciona en las ejecuciones vinculadas las señales de disponibilidad del proveedor, sobrecarga o limitación de tasa, y reintenta cuando la ruta del proveedor vuelva a estar sana.',
  ],
  'fr-FR': [
    "Reconnectez les identifiants du fournisseur, confirmez que la clé API ou les en-têtes d'authentification sont valides, puis relancez l'exécution.",
    'Restaurez le quota ou le crédit du fournisseur, ou redirigez le cas vers un autre fournisseur approvisionné avant de réessayer.',
    'Inspectez dans les exécutions liées les signaux de disponibilité du fournisseur, de surcharge ou de limitation de débit, puis réessayez lorsque le chemin fournisseur est de nouveau sain.',
  ],
  'ga-IE': [
    "Athcheangail dintiúir an tsoláthraí, dearbhaigh go bhfuil an eochair API nó na ceanntásca fíordheimhnithe bailí, agus ansin bain triail eile as an rith.",
    'Athbhunaigh cuóta nó creidmheas an tsoláthraí, nó seol an cás chuig soláthraí eile a bhfuil maoiniú aige sula ndéanann tú iarracht eile.',
    'Scrúdaigh sna ritheanna nasctha comharthaí infhaighteachta an tsoláthraí, ró-ualaigh nó ráta-teorannaithe, agus bain triail eile as nuair atá cosán an tsoláthraí slán arís.',
  ],
  'hr-HR': [
    'Ponovno povežite vjerodajnice pružatelja, potvrdite da su API ključ ili zaglavlja za autentifikaciju valjani, pa zatim ponovno pokrenite izvršavanje.',
    'Obnovite kvotu ili sredstva pružatelja ili preusmjerite slučaj na drugog financiranog pružatelja prije ponovnog pokušaja.',
    'U povezanim pokretanjima provjerite signale dostupnosti pružatelja, preopterećenja ili ograničenja brzine te pokušajte ponovno kada je putanja pružatelja ponovno stabilna.',
  ],
  'hu-HU': [
    'Csatlakoztassa újra a szolgáltatói hitelesítő adatokat, ellenőrizze, hogy az API-kulcs vagy az auth fejlécek érvényesek-e, majd futtassa újra a kört.',
    'Állítsa helyre a szolgáltató kvótáját vagy egyenlegét, vagy irányítsa át az esetet egy másik finanszírozott szolgáltatóhoz, mielőtt újrapróbálja.',
    'Ellenőrizze a kapcsolt futásokban a szolgáltató elérhetőségére, túlterhelésére vagy sebességkorlátozására utaló jeleket, és akkor próbálja újra, amikor a szolgáltatói útvonal ismét stabil.',
  ],
  'it-IT': [
    'Ricollega le credenziali del provider, verifica che la chiave API o gli header di autenticazione siano validi, quindi riprova l\'esecuzione.',
    'Ripristina quota o credito del provider, oppure instrada il caso verso un altro provider con fondi prima di riprovare.',
    'Controlla nelle esecuzioni collegate i segnali di disponibilità del provider, sovraccarico o rate limit, poi riprova quando il percorso del provider è di nuovo stabile.',
  ],
  'ja-JP': [
    'プロバイダーの認証情報を再接続し、API キーまたは認証ヘッダーが有効か確認してから、実行を再試行してください。',
    'プロバイダーのクォータまたは残高を回復するか、再試行前に資金のある別のプロバイダーへこのケースをルーティングしてください。',
    '関連する実行でプロバイダーの可用性、過負荷、またはレート制限のシグナルを確認し、プロバイダーパスが健全に戻ってから再試行してください。',
  ],
  'ko-KR': [
    '제공자 자격 증명을 다시 연결하고 API 키 또는 인증 헤더가 유효한지 확인한 다음 실행을 다시 시도하세요.',
    '재시도하기 전에 제공자 할당량 또는 크레딧을 복구하거나, 자금이 있는 다른 제공자로 이 케이스를 라우팅하세요.',
    '연결된 실행에서 제공자 가용성, 과부하 또는 속도 제한 신호를 확인한 뒤 제공자 경로가 정상으로 돌아오면 다시 시도하세요.',
  ],
  'ml-IN': [
    'ദാതാവിന്റെ ക്രെഡൻഷ്യലുകൾ വീണ്ടും ബന്ധിപ്പിക്കുക, API കീ അല്ലെങ്കിൽ ഓത്ത് ഹെഡറുകൾ സാധുവാണെന്ന് ഉറപ്പാക്കുക, പിന്നെ റൺ വീണ്ടും ശ്രമിക്കുക.',
    'വീണ്ടും ശ്രമിക്കുന്നതിന് മുമ്പ് ദാതാവിന്റെ ക്വോട്ടയോ ക്രെഡിറ്റോ പുനഃസ്ഥാപിക്കുക, അല്ലെങ്കിൽ ഈ കേസ് ഫണ്ടുള്ള മറ്റൊരു ദാതാവിലേക്ക് റൂട്ടുചെയ്യുക.',
    'ബന്ധപ്പെട്ട റൺകളിലെ ദാതാവിന്റെ ലഭ്യത, അമിതഭാരം, അല്ലെങ്കിൽ നിരക്ക്-പരിധി സൂചനകൾ പരിശോധിച്ച് ദാതൃപാത വീണ്ടും ആരോഗ്യകരമാകുമ്പോൾ വീണ്ടും ശ്രമിക്കുക.',
  ],
  'nb-NO': [
    'Koble til leverandørens legitimasjon på nytt, bekreft at API-nøkkelen eller autentiseringsheaderne er gyldige, og prøv deretter kjøringen på nytt.',
    'Gjenopprett leverandørens kvote eller kreditt, eller ruter saken til en annen finansiert leverandør før du prøver igjen.',
    'Undersøk signaler om leverandørtilgjengelighet, overbelastning eller hastighetsbegrensning i tilknyttede kjøringer, og prøv igjen når leverandørbanen er frisk.',
  ],
  'nl-NL': [
    'Verbind de providerreferenties opnieuw, controleer of de API-sleutel of authenticatieheaders geldig zijn en probeer de run daarna opnieuw.',
    'Herstel de providerquota of het tegoed, of stuur deze case naar een andere gefinancierde provider voordat je opnieuw probeert.',
    'Controleer in gekoppelde runs signalen voor providerbeschikbaarheid, overbelasting of rate-limiting en probeer opnieuw zodra het providerpad weer gezond is.',
  ],
  'pl-PL': [
    'Połącz ponownie poświadczenia dostawcy, potwierdź, że klucz API lub nagłówki uwierzytelniania są prawidłowe, a następnie ponów uruchomienie.',
    'Przywróć limit lub środki dostawcy albo skieruj ten przypadek do innego finansowanego dostawcy przed ponowną próbą.',
    'Sprawdź w powiązanych uruchomieniach sygnały dostępności dostawcy, przeciążenia lub ograniczeń szybkości i spróbuj ponownie, gdy ścieżka dostawcy znów będzie stabilna.',
  ],
  'pt-BR': [
    'Reconecte as credenciais do provedor, confirme que a chave de API ou os cabeçalhos de autenticação são válidos e tente executar novamente.',
    'Restaure a cota ou o crédito do provedor, ou encaminhe o caso para outro provedor com saldo antes de tentar novamente.',
    'Inspecione nos runs vinculados os sinais de disponibilidade do provedor, sobrecarga ou limitação de taxa e tente novamente quando o caminho do provedor estiver saudável.',
  ],
  'pt-PT': [
    'Volte a ligar as credenciais do fornecedor, confirme que a chave API ou os cabeçalhos de autenticação são válidos e tente novamente a execução.',
    'Reponha a quota ou o saldo do fornecedor, ou encaminhe o caso para outro fornecedor com saldo antes de tentar novamente.',
    'Inspecione nas execuções associadas os sinais de disponibilidade do fornecedor, sobrecarga ou limitação de taxa e tente novamente quando o caminho do fornecedor estiver novamente estável.',
  ],
  'ro-RO': [
    'Reconectați acreditările furnizorului, confirmați că cheia API sau antetele de autentificare sunt valide, apoi reîncercați rularea.',
    'Restabiliți cota sau creditul furnizorului ori redirecționați cazul către un alt furnizor finanțat înainte de a reîncerca.',
    'Inspectați în rulările asociate semnalele de disponibilitate a furnizorului, supraîncărcare sau limitare de rată și reîncercați când ruta furnizorului revine la o stare sănătoasă.',
  ],
  'ru-RU': [
    'Повторно подключите учётные данные провайдера, убедитесь, что API-ключ или заголовки аутентификации действительны, а затем повторите запуск.',
    'Восстановите квоту или баланс провайдера либо направьте этот случай к другому провайдеру с доступным бюджетом перед повторной попыткой.',
    'Проверьте в связанных запусках сигналы доступности провайдера, перегрузки или rate limit и повторите попытку, когда путь к провайдеру снова станет стабильным.',
  ],
  'sk-SK': [
    'Znova pripojte poverenia poskytovateľa, overte, že API kľúč alebo autorizačné hlavičky sú platné, a potom spustenie zopakujte.',
    'Obnovte kvótu alebo kredit poskytovateľa, prípadne presmerujte prípad na iného financovaného poskytovateľa pred ďalším pokusom.',
    'V prepojených behoch skontrolujte signály dostupnosti poskytovateľa, preťaženia alebo obmedzenia rýchlosti a skúste to znova, keď bude cesta poskytovateľa opäť v poriadku.',
  ],
  'sv-SE': [
    'Anslut leverantörens autentiseringsuppgifter igen, bekräfta att API-nyckeln eller autentiseringshuvudena är giltiga och försök sedan köra igen.',
    'Återställ leverantörens kvot eller saldo, eller dirigera fallet till en annan finansierad leverantör innan du försöker igen.',
    'Granska signaler om leverantörstillgänglighet, överbelastning eller hastighetsbegränsning i länkade körningar och försök igen när leverantörsvägen är stabil igen.',
  ],
  'zh-CN': [
    '请重新连接提供商凭证，确认 API Key 或鉴权头有效后，再重试这次运行。',
    '请先恢复提供商额度或余额，或切换到另一个有额度的提供商后再重试。',
    '请检查关联运行中的提供商可用性、过载或限流信号，待提供商链路恢复健康后再重试。',
  ],
  'zh-TW': [
    '請重新連接提供商憑證，確認 API Key 或鑑權標頭有效後，再重試這次執行。',
    '請先恢復提供商額度或餘額，或切換到另一個有額度的提供商後再重試。',
    '請檢查關聯執行中的提供商可用性、過載或限流訊號，待提供商鏈路恢復健康後再重試。',
  ],
}

export function harnessProviderRemediationGroup(localeKey: LocaleKey) {
  const [
    remediationInfraProviderAuth,
    remediationInfraProviderQuota,
    remediationInfraProviderBlocked,
  ] = harnessProviderRemediationCopyByLocale[localeKey]

  return {
    remediationInfraProviderAuth,
    remediationInfraProviderQuota,
    remediationInfraProviderBlocked,
  }
}
