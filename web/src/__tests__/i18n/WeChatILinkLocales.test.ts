import { describe, expect, it } from 'vitest'

type LocaleMessages = Record<string, unknown>

const localeModules = import.meta.glob('@/i18n/locales/*.ts', { eager: true }) as Record<
  string,
  { default: LocaleMessages }
>

function fileNameFromModulePath(modulePath: string): string {
  return modulePath.split('/').pop() ?? modulePath
}

function getLocaleCode(modulePath: string): string {
  return fileNameFromModulePath(modulePath).replace(/\.ts$/, '')
}

function findLocaleMessages(localeCode: string): LocaleMessages | undefined {
  const match = Object.entries(localeModules).find(([modulePath]) =>
    modulePath.endsWith(`/${localeCode}.ts`) || fileNameFromModulePath(modulePath) === `${localeCode}.ts`
  )
  return match?.[1].default
}

function getPathValue(messages: LocaleMessages, path: string): unknown {
  return path.split('.').reduce<unknown>((current, segment) => {
    if (current && typeof current === 'object' && segment in current) {
      return (current as Record<string, unknown>)[segment]
    }
    return undefined
  }, messages)
}

const requiredWeChatILinkKeys = [
  'channels.wechatILink',
  'channels.wechatProvider',
  'channels.wechatProviderWork',
  'channels.wechatProviderILink',
  'channels.placeholderILinkAPIBaseURL',
  'channels.apiBaseURL',
  'channels.wechatILinkPrimaryAction',
  'channels.wechatILinkScanAction',
  'channels.wechatILinkManualAction',
  'channels.wechatILinkScanHint',
  'channels.wechatILinkSetupCreating',
  'channels.wechatILinkSetupCreateFailed',
  'channels.wechatILinkSetupDescription',
  'channels.wechatILinkSetupStatus',
  'channels.wechatILinkOpenOnPhone',
  'channels.wechatILinkSetupEyebrow',
  'channels.wechatILinkSetupTitle',
  'channels.wechatILinkSetupMissingSession',
  'channels.wechatILinkSetupLoadFailed',
  'channels.wechatILinkSetupSubmit',
  'channels.wechatILinkSetupSubmitting',
  'channels.wechatILinkSetupSubmitFailed',
  'channels.wechatILinkSetupSuccess',
  'channels.wechatILinkSetupStatePending',
  'channels.wechatILinkSetupStateAuthorizing',
  'channels.wechatILinkSetupStateConfiguring',
  'channels.wechatILinkSetupStateConnected',
  'channels.wechatILinkSetupStateError',
  'channels.wechatILinkSetupStateExpired',
  'channels.wechatILinkPairingPayload',
  'channels.wechatILinkPairingPayloadPlaceholder',
  'channels.wechatILinkDesc',
  'channels.wechatILinkHint',
] as const

const nonEnglishGuardedKeys = {
  'channels.wechatProvider': 'WeChat Provider',
  'channels.apiBaseURL': 'API Base URL',
  'channels.wechatILinkPrimaryAction': 'Primary Action',
  'channels.wechatILinkScanAction': 'Scan To Connect',
  'channels.wechatILinkManualAction': 'Manual Config',
  'channels.wechatILinkSetupCreateFailed': 'Failed to create setup session',
  'channels.wechatILinkSetupTitle': 'Finish WeChat iLink Setup',
  'channels.wechatILinkDesc': 'Connect personal WeChat via iLink bot credentials',
  'channels.wechatILinkHint':
    'Use WeChat QR authorization as the primary path, or provide the Bot Token manually if needed',
} as const

const staleLegacyDirectConnectCopyByLocale = {
  'ca-ES': {
    wechatILinkScanAction: 'Escaneja per configurar',
    wechatILinkScanHint:
      "Crea una sessio de configuracio d'un sol us, reutilitza o inicia automaticament l'acces remot i acaba l'autoritzacio al teu telefon.",
    wechatILinkSetupDescription:
      "Escaneja el codi QR amb el teu telefon, completa l'autoritzacio d'iLink i Blue escriura la configuracio i activara el canal automaticament.",
    wechatILinkOpenOnPhone: 'Obre la configuracio al telefon',
  },
  'cs-CZ': {
    wechatILinkScanAction: 'Naskenovat pro konfiguraci',
    wechatILinkScanHint:
      'Vytvorte jednorazovou konfiguracni relaci, znovu pouzijte nebo automaticky spustte vzdaleny pristup a pote dokoncete autorizaci v telefonu.',
    wechatILinkSetupDescription:
      'Naskenujte QR kod v telefonu, dokoncete autorizaci iLink a Blue automaticky zapise konfiguraci a povoli kanal.',
    wechatILinkOpenOnPhone: 'Otevrit nastaveni v telefonu',
  },
  'da-DK': {
    wechatILinkScanAction: 'Scan for at konfigurere',
    wechatILinkScanHint:
      'Opret en engangsopsaetningssession, genbrug eller start fjernadgang automatisk, og fuldfor derefter godkendelsen pa din telefon.',
    wechatILinkSetupDescription:
      'Scan QR-koden pa din telefon, fuldfor iLink-godkendelsen, sa skriver Blue konfigurationen og aktiverer kanalen automatisk.',
    wechatILinkOpenOnPhone: 'Aben opsaetning pa telefonen',
  },
  'de-DE': {
    wechatILinkScanAction: 'Zum Konfigurieren scannen',
    wechatILinkScanHint:
      'Erstellen Sie eine einmalige Einrichtungssitzung, verwenden Sie den Fernzugriff wieder oder starten Sie ihn automatisch und schliessen Sie dann die Autorisierung auf Ihrem Telefon ab.',
    wechatILinkSetupDescription:
      'Scannen Sie den QR-Code auf Ihrem Telefon, schliessen Sie die iLink-Autorisierung ab und Blue schreibt die Konfiguration und aktiviert den Kanal automatisch.',
    wechatILinkOpenOnPhone: 'Einrichtung auf dem Telefon offnen',
  },
  'el-GR': {
    wechatILinkScanAction: 'Sarosi gia rythmisi',
    wechatILinkScanHint:
      'Dimiourgiste mia efapax synedria rythmisis, epanaxrisimopoiiste i ekkiniste automata tin apomakrysmeni prosvasi kai olokliroste tin exousiodotisi sto tilefono sas.',
    wechatILinkSetupDescription:
      'Saroste ton kodiko QR apo to tilefono sas, olokliroste tin exousiodotisi iLink kai to Blue tha grapsei ti rythmisi kai tha energopoiisei to kanali automata.',
    wechatILinkOpenOnPhone: 'Anoigma rythmisis sto tilefono',
  },
  'en-GB': {
    wechatILinkScanAction: 'Scan to Configure',
    wechatILinkScanHint:
      'Create a one-off setup session, reuse or auto-start remote access, then finish authorisation on your phone.',
    wechatILinkSetupDescription:
      'Scan the QR code on your phone, finish iLink authorisation, then Blue will write the configuration and enable the channel automatically.',
    wechatILinkOpenOnPhone: 'Open Setup on Phone',
  },
  'en-US': {
    wechatILinkScanAction: 'Scan To Configure',
    wechatILinkScanHint:
      'Create a one-time setup session, reuse or auto-start remote access, then finish authorization on your phone.',
    wechatILinkSetupDescription:
      'Scan the QR code on your phone, finish iLink authorization, then Blue will write the configuration and enable the channel automatically.',
    wechatILinkOpenOnPhone: 'Open Setup On Phone',
  },
  'es-ES': {
    wechatILinkScanAction: 'Escanear para configurar',
    wechatILinkScanHint:
      'Crea una sesion de configuracion de un solo uso, reutiliza o inicia automaticamente el acceso remoto y despues completa la autorizacion en tu telefono.',
    wechatILinkSetupDescription:
      'Escanea el codigo QR con tu telefono, completa la autorizacion de iLink y Blue escribira la configuracion y habilitara el canal automaticamente.',
    wechatILinkOpenOnPhone: 'Abrir configuracion en el telefono',
  },
  'fr-FR': {
    wechatILinkScanAction: 'Scanner pour configurer',
    wechatILinkScanHint:
      'Creez une session de configuration a usage unique, reutilisez ou demarrez automatiquement l acces distant, puis terminez l autorisation sur votre telephone.',
    wechatILinkSetupDescription:
      'Scannez le code QR avec votre telephone, terminez l autorisation iLink, puis Blue ecrira la configuration et activera le canal automatiquement.',
    wechatILinkOpenOnPhone: 'Ouvrir la configuration sur le telephone',
  },
  'ga-IE': {
    wechatILinkScanAction: 'Scanail chun a chumru',
    wechatILinkScanHint:
      'Cruthaigh seisiun socraithe aonuaire, athusaid no tosaigh rochtain chianda go huathoibrioch, ansin criochnaigh an t-udaru ar do ghuthan.',
    wechatILinkSetupDescription:
      'Scanail an cod QR ar do ghuthan, criochnaigh udaru iLink, ansin scribhfaidh Blue an chumraiocht agus cumasoidh se an cainéal go huathoibrioch.',
    wechatILinkOpenOnPhone: 'Oscail an socru ar an bhfón',
  },
  'hr-HR': {
    wechatILinkScanAction: 'Skeniraj za konfiguraciju',
    wechatILinkScanHint:
      'Izradite jednokratnu sesiju postavljanja, ponovno upotrijebite ili automatski pokrenite udaljeni pristup, a zatim dovrsite autorizaciju na telefonu.',
    wechatILinkSetupDescription:
      'Skenirajte QR kod na telefonu, dovrsite iLink autorizaciju, a Blue ce automatski upisati konfiguraciju i omoguciti kanal.',
    wechatILinkOpenOnPhone: 'Otvori postavljanje na telefonu',
  },
  'hu-HU': {
    wechatILinkScanAction: 'Beolvasas a beallitashoz',
    wechatILinkScanHint:
      'Hozzon letre egyszer hasznalatos beallitasi munkamenetet, hasznalja ujra vagy inditsa el automatikusan a tavoli hozzaferest, majd fejezze be az engedelyezest a telefonjan.',
    wechatILinkSetupDescription:
      'Olvassa be a QR-kodot a telefonjan, fejezze be az iLink engedelyezest, ezutan a Blue automatikusan elmenti a beallitast es engedelyezi a csatornat.',
    wechatILinkOpenOnPhone: 'Beallitas megnyitasa a telefonon',
  },
  'it-IT': {
    wechatILinkScanAction: 'Scansiona per configurare',
    wechatILinkScanHint:
      'Crea una sessione di configurazione monouso, riutilizza o avvia automaticamente l accesso remoto, quindi completa l autorizzazione sul telefono.',
    wechatILinkSetupDescription:
      'Scansiona il codice QR dal telefono, completa l autorizzazione iLink e Blue scrivera la configurazione e abilitera automaticamente il canale.',
    wechatILinkOpenOnPhone: 'Apri configurazione sul telefono',
  },
  'ja-JP': {
    wechatILinkScanAction: 'スキャンして設定',
    wechatILinkScanHint:
      'ワンタイムのセットアップセッションを作成し、リモートアクセスを再利用または自動起動してから、スマートフォンで認可を完了してください。',
    wechatILinkSetupDescription:
      'スマートフォンで QR コードをスキャンして iLink の認可を完了すると、Blue が設定を書き込み、チャンネルを自動で有効化します。',
    wechatILinkOpenOnPhone: 'スマートフォンで設定を開く',
  },
  'ko-KR': {
    wechatILinkScanAction: '스캔으로 구성',
    wechatILinkScanHint:
      '일회성 설정 세션을 만들고 원격 액세스를 재사용하거나 자동 시작한 다음 휴대폰에서 인증을 완료하세요.',
    wechatILinkSetupDescription:
      '휴대폰으로 QR 코드를 스캔하고 iLink 인증을 완료하면 Blue가 설정을 기록하고 채널을 자동으로 활성화합니다.',
    wechatILinkOpenOnPhone: '휴대폰에서 설정 열기',
  },
  'ml-IN': {
    wechatILinkScanAction: 'സ്കാൻ ചെയ്ത് ക്രമീകരിക്കുക',
    wechatILinkScanHint:
      'ഒരുതവണ മാത്രം ഉപയോഗിക്കാവുന്ന സജ്ജീകരണ സെഷൻ സൃഷ്ടിച്ച്, റിമോട്ട് ആക്സസ് പുനരുപയോഗിക്കുകയോ സ്വയം ആരംഭിക്കുകയോ ചെയ്ത്, ശേഷം നിങ്ങളുടെ ഫോണിൽ അംഗീകാരം പൂർത്തിയാക്കുക.',
    wechatILinkSetupDescription:
      'ഫോണിൽ QR കോഡ് സ്കാൻ ചെയ്ത് iLink അംഗീകാരം പൂർത്തിയാക്കുക. തുടർന്ന് Blue ക്രമീകരണം എഴുതുകയും ചാനൽ സ്വയം സജീവമാക്കുകയും ചെയ്യും.',
    wechatILinkOpenOnPhone: 'ഫോണിൽ സജ്ജീകരണം തുറക്കുക',
  },
  'nb-NO': {
    wechatILinkScanAction: 'Skann for a konfigurere',
    wechatILinkScanHint:
      'Opprett en engangsoppsettsokt, gjenbruk eller start fjernaksess automatisk, og fullfor deretter autorisasjonen pa telefonen.',
    wechatILinkSetupDescription:
      'Skann QR-koden pa telefonen, fullfor iLink-autoriseringen, sa skriver Blue konfigurasjonen og aktiverer kanalen automatisk.',
    wechatILinkOpenOnPhone: 'Apne oppsett pa telefonen',
  },
  'nl-NL': {
    wechatILinkScanAction: 'Scannen om te configureren',
    wechatILinkScanHint:
      'Maak een eenmalige configuratiesessie aan, hergebruik of start externe toegang automatisch en voltooi daarna de autorisatie op je telefoon.',
    wechatILinkSetupDescription:
      'Scan de QR-code op je telefoon, voltooi de iLink-autorisatie en Blue schrijft de configuratie weg en schakelt het kanaal automatisch in.',
    wechatILinkOpenOnPhone: 'Configuratie op telefoon openen',
  },
  'pl-PL': {
    wechatILinkScanAction: 'Skanuj, aby skonfigurowac',
    wechatILinkScanHint:
      'Utworz jednorazowa sesje konfiguracji, uzyj ponownie zdalnego dostepu lub uruchom go automatycznie, a nastepnie dokoncz autoryzacje na telefonie.',
    wechatILinkSetupDescription:
      'Zeskanuj kod QR telefonem, dokoncz autoryzacje iLink, a Blue automatycznie zapisze konfiguracje i wlaczy kanal.',
    wechatILinkOpenOnPhone: 'Otworz konfiguracje na telefonie',
  },
  'pt-BR': {
    wechatILinkScanAction: 'Escanear para configurar',
    wechatILinkScanHint:
      'Crie uma sessao unica de configuracao, reutilize ou inicie automaticamente o acesso remoto e depois conclua a autorizacao no seu telefone.',
    wechatILinkSetupDescription:
      'Escaneie o QR code no seu telefone, conclua a autorizacao do iLink e o Blue gravara a configuracao e ativara o canal automaticamente.',
    wechatILinkOpenOnPhone: 'Abrir configuracao no telefone',
  },
  'pt-PT': {
    wechatILinkScanAction: 'Digitalizar para configurar',
    wechatILinkScanHint:
      'Crie uma sessao unica de configuracao, reutilize ou inicie automaticamente o acesso remoto e depois conclua a autorizacao no seu telemovel.',
    wechatILinkSetupDescription:
      'Digitalize o codigo QR no telemovel, conclua a autorizacao iLink e o Blue ira gravar a configuracao e ativar o canal automaticamente.',
    wechatILinkOpenOnPhone: 'Abrir configuracao no telemovel',
  },
  'ro-RO': {
    wechatILinkScanAction: 'Scaneaza pentru configurare',
    wechatILinkScanHint:
      'Creati o sesiune unica de configurare, reutilizati sau porniti automat accesul la distanta, apoi finalizati autorizarea pe telefon.',
    wechatILinkSetupDescription:
      'Scanati codul QR de pe telefon, finalizati autorizarea iLink, iar Blue va scrie configuratia si va activa automat canalul.',
    wechatILinkOpenOnPhone: 'Deschide configurarea pe telefon',
  },
  'ru-RU': {
    wechatILinkScanAction: 'Сканировать для настройки',
    wechatILinkScanHint:
      'Создайте одноразовую сессию настройки, повторно используйте или автоматически запустите удаленный доступ, затем завершите авторизацию на телефоне.',
    wechatILinkSetupDescription:
      'Отсканируйте QR-код телефоном, завершите авторизацию iLink, и Blue автоматически запишет настройки и включит канал.',
    wechatILinkOpenOnPhone: 'Открыть настройку на телефоне',
  },
  'sk-SK': {
    wechatILinkScanAction: 'Naskenovat na konfiguraciu',
    wechatILinkScanHint:
      'Vytvorte jednorazovu relaciu nastavenia, znova pouzite alebo automaticky spustite vzdialeny pristup a potom dokoncite autorizaciu v telefone.',
    wechatILinkSetupDescription:
      'Naskenujte QR kod v telefone, dokoncite autorizaciu iLink a Blue automaticky zapise konfiguraciu a povoli kanal.',
    wechatILinkOpenOnPhone: 'Otvorit nastavenie v telefone',
  },
  'sv-SE': {
    wechatILinkScanAction: 'Skanna for att konfigurera',
    wechatILinkScanHint:
      'Skapa en engangsinstallningssession, ateranvand eller starta fjarratkomst automatiskt och slutför sedan auktoriseringen pa telefonen.',
    wechatILinkSetupDescription:
      'Skanna QR-koden pa telefonen, slutför iLink-auktoriseringen och Blue skriver konfigurationen och aktiverar kanalen automatiskt.',
    wechatILinkOpenOnPhone: 'Oppna konfiguration pa telefonen',
  },
  'zh-CN': {
    wechatILinkScanAction: '扫码一键配置',
    wechatILinkScanHint: '创建一次性配置会话，复用或自动拉起远程访问，然后在手机上完成授权。',
    wechatILinkSetupDescription: '请在手机上扫码并完成 iLink 授权，Blue 会自动写入配置并启用该 channel。',
    wechatILinkOpenOnPhone: '在手机上打开配置页',
  },
  'zh-TW': {
    wechatILinkScanAction: '掃碼一鍵配置',
    wechatILinkScanHint: '建立一次性配置會話，重用或自動啟動遠端存取，然後在手機上完成授權。',
    wechatILinkSetupDescription: '請在手機上掃碼並完成 iLink 授權，Blue 會自動寫入設定並啟用該渠道。',
    wechatILinkOpenOnPhone: '在手機上開啟配置頁',
  },
} as const

const staleLegacyHintByLocale = {
  'ca-ES':
    "Fes servir la configuracio per escaneig com a via principal o omple manualment l'URL base de l'API i el token del bot com a alternativa",
  'cs-CZ':
    'Jako hlavni cestu pouzijte konfiguraci pres skenovani, pripadne rucne vyplnte zakladni URL API a token bota',
  'da-DK':
    'Brug scan-til-konfiguration som primaer vej, eller udfyld API-base-URL og bottoken manuelt som fallback',
  'de-DE':
    'Verwenden Sie die Scan-Konfiguration als primaren Weg oder tragen Sie API-Basis-URL und Bot-Token manuell als Fallback ein',
  'el-GR':
    'Xrisimopoiiste ti rythmisi meso sarosis os kyria diadromi i sympliroste cheirokinita to vasiko URL API kai to token tou bot os efedreia',
  'en-GB':
    'Use scan-to-config as the primary path, or fill API Base URL and Bot Token manually as fallback',
  'en-US':
    'Use scan-to-config as the primary path, or fill API Base URL and Bot Token manually as fallback',
  'es-ES':
    'Usa la configuracion por escaneo como via principal o rellena manualmente la URL base de la API y el token del bot como alternativa',
  'fr-FR':
    'Utilisez la configuration par scan comme voie principale, ou saisissez manuellement l URL de base de l API et le jeton du bot en secours',
  'ga-IE':
    'Usaid cumraiocht scanala mar phriomhbhealach, no lion isteach Bun-URL an API agus comhartha an bhota de laimh mar chultaca',
  'hr-HR':
    'Koristite konfiguraciju skeniranjem kao glavni put ili rucno ispunite osnovni URL API-ja i token bota kao rezervu',
  'hu-HU':
    'Elsodleges utkent hasznalja a szkenneleses beallitast, vagy tartalekkent adja meg kezzel az API alap URL-t es a bot tokent',
  'it-IT':
    'Usa la configurazione tramite scansione come percorso principale oppure inserisci manualmente URL base API e token del bot come alternativa',
  'ja-JP':
    '基本はスキャン設定を使い、失敗した場合は API ベース URL と Bot トークンを手動入力してください',
  'ko-KR':
    '기본 경로로 스캔 설정을 사용하고, 실패하면 API 기본 URL과 봇 토큰을 수동 입력하세요',
  'ml-IN':
    'പ്രധാന മാർഗമായി സ്കാൻ-ടു-കോൺഫിഗർ ഉപയോഗിക്കുക; പകരമായി API അടിസ്ഥാന URLയും ബോട്ട് ടോക്കണും കൈയോടെ നൽകാം',
  'nb-NO':
    'Bruk skann-for-konfigurasjon som hovedvei, eller fyll inn API-base-URL og bottoken manuelt som reserve',
  'nl-NL':
    'Gebruik scannen-om-te-configureren als hoofdpad, of vul de API-basis-URL en het bottoken handmatig in als fallback',
  'pl-PL':
    'Uzyj konfiguracji przez skanowanie jako glownej sciezki albo recznie wpisz bazowy URL API i token bota jako obejscie',
  'pt-BR':
    'Use a configuracao por escaneamento como caminho principal ou preencha manualmente a URL base da API e o token do bot como alternativa',
  'pt-PT':
    'Use a configuracao por digitalizacao como via principal ou preencha manualmente a URL base da API e o token do bot como alternativa',
  'ro-RO':
    'Folositi configurarea prin scanare ca drum principal sau completati manual URL-ul de baza API si tokenul botului ca rezerva',
  'ru-RU':
    'Используйте настройку через сканирование как основной путь или вручную заполните базовый URL API и токен бота как запасной вариант',
  'sk-SK':
    'Ako hlavny sposob pouzite konfiguraciu skenovanim, alebo rucne vyplnte zakladnu URL API a token bota ako zalohu',
  'sv-SE':
    'Anvand skanna-for-att-konfigurera som huvudvag, eller fyll i API-bas-URL och bottoken manuellt som reserv',
  'zh-CN': '默认使用扫码一键配置，扫码失败时可手动填写 API 基础地址和 Bot Token',
  'zh-TW': '預設使用掃碼一鍵配置，掃碼失敗時可手動填寫 API 基礎地址和 Bot Token',
} as const

describe('WeChat iLink locale coverage', () => {
  it('exposes the full WeChat iLink copy block in all 27 locales', () => {
    const entries = Object.entries(localeModules).sort(([a], [b]) => a.localeCompare(b))
    expect(entries).toHaveLength(27)

    for (const [modulePath, mod] of entries) {
      const file = fileNameFromModulePath(modulePath)
      const messages = mod.default

      for (const key of requiredWeChatILinkKeys) {
        const value = getPathValue(messages, key)
        expect(typeof value, `${file} missing ${key}`).toBe('string')
        expect(String(value).trim().length, `${file} empty ${key}`).toBeGreaterThan(0)
      }
    }
  })

  it('keeps key WeChat iLink labels localized outside English locales', () => {
    const entries = Object.entries(localeModules).sort(([a], [b]) => a.localeCompare(b))
    expect(entries).toHaveLength(27)

    for (const [modulePath, mod] of entries) {
      const locale = getLocaleCode(modulePath)
      const file = fileNameFromModulePath(modulePath)
      const messages = mod.default

      if (locale === 'en-US' || locale === 'en-GB') {
        continue
      }

      for (const [key, englishValue] of Object.entries(nonEnglishGuardedKeys)) {
        const value = getPathValue(messages, key)
        expect(typeof value, `${file} missing ${key}`).toBe('string')
        expect(String(value).trim().length, `${file} empty ${key}`).toBeGreaterThan(0)
        expect(value, `${file} should not fall back to English for ${key}`).not.toBe(englishValue)
      }
    }
  })

  it('keeps setup-session wording localized in Chinese locales', () => {
    const zhCN = findLocaleMessages('zh-CN')
    const zhTW = findLocaleMessages('zh-TW')

    for (const [file, messages] of [
      ['zh-CN.ts', zhCN],
      ['zh-TW.ts', zhTW],
    ] as const) {
      expect(messages, `${file} should be loaded`).toBeTruthy()

      for (const key of [
        'channels.wechatILinkScanHint',
        'channels.wechatILinkSetupCreateFailed',
        'channels.wechatILinkSetupMissingSession',
        'channels.wechatILinkSetupLoadFailed',
      ]) {
        const value = String(getPathValue(messages as LocaleMessages, key) || '')
        expect(value, `${file} should not keep raw setup/session wording for ${key}`).not.toMatch(
          /setup session|setup 会话|setup 會話/i
        )
      }
    }
  })

  it('removes legacy setup-page copy from the direct-connect flow in all 27 locales', () => {
    for (const [locale, staleCopy] of Object.entries(staleLegacyDirectConnectCopyByLocale)) {
      const messages = findLocaleMessages(locale)
      expect(messages, `${locale} should be loaded`).toBeTruthy()

      for (const [key, staleValue] of Object.entries(staleCopy)) {
        const value = getPathValue(messages as LocaleMessages, `channels.${key}`)
        expect(typeof value, `${locale} missing channels.${key}`).toBe('string')
        expect(String(value).trim().length, `${locale} empty channels.${key}`).toBeGreaterThan(0)
        expect(
          value,
          `${locale} should no longer use legacy direct-connect copy for channels.${key}`
        ).not.toBe(staleValue)
      }
    }
  })

  it('replaces the old scan-to-config hint in all 27 locales', () => {
    for (const [locale, staleValue] of Object.entries(staleLegacyHintByLocale)) {
      const messages = findLocaleMessages(locale)
      expect(messages, `${locale} should be loaded`).toBeTruthy()

      const value = getPathValue(messages as LocaleMessages, 'channels.wechatILinkHint')
      expect(typeof value, `${locale} missing channels.wechatILinkHint`).toBe('string')
      expect(String(value).trim().length, `${locale} empty channels.wechatILinkHint`).toBeGreaterThan(0)
      expect(value, `${locale} should no longer use the old scan-to-config hint`).not.toBe(
        staleValue
      )
    }
  })

  it('removes API Base URL references from the bot-only fallback copy in all 27 locales', () => {
    const entries = Object.entries(localeModules).sort(([a], [b]) => a.localeCompare(b))
    expect(entries).toHaveLength(27)

    for (const [modulePath, mod] of entries) {
      const locale = getLocaleCode(modulePath)
      const messages = mod.default
      const apiBaseURLLabel = String(getPathValue(messages, 'channels.apiBaseURL') || '').trim()
      const hint = String(getPathValue(messages, 'channels.wechatILinkHint') || '').trim()
      const placeholder = String(
        getPathValue(messages, 'channels.wechatILinkPairingPayloadPlaceholder') || ''
      ).trim()

      expect(apiBaseURLLabel, `${locale} missing channels.apiBaseURL`).not.toBe('')
      expect(hint, `${locale} missing channels.wechatILinkHint`).not.toBe('')
      expect(
        hint,
        `${locale} should not mention API Base URL in channels.wechatILinkHint`
      ).not.toContain(apiBaseURLLabel)
      expect(
        placeholder,
        `${locale} should not mention API Base URL in channels.wechatILinkPairingPayloadPlaceholder`
      ).not.toContain(apiBaseURLLabel)
    }
  })
})
