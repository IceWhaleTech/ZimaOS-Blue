import type { LocaleMessages } from './merge'

const contextCompressionOverrides: Record<string, LocaleMessages> = {
  'ca-ES': {
    settings: {
      smallModel: {
        contextCompression: 'Compressió del context',
        contextCompressionHint:
          'La compressió continua activant-se automàticament quan la pressió del context és alta; aquest interruptor només decideix si el model lleuger té la primera passada de compressió.',
        contextCompressionMode: 'Mode de compressió',
        contextCompressionModeHint:
          "La compressió s'activa automàticament quan hi ha pressió de context. Aquí només tries quina ruta de compressió es prefereix, mentre que la intenció del torn actual continua venint de l'últim missatge de l'usuari.",
        contextCompressionModeAuto:
          "Automàtic: prioritza la compressió amb model lleuger i recorre a l'offline",
        contextCompressionModeSmallModel: 'Prioritza el model lleuger',
        contextCompressionModeOffline: 'Offline determinista',
        contextCompressionModeOff: 'Desactivat',
        contextCompressionSuccessRate: "Taxa d'encert de la compressió del context",
        contextCompressionLatencyMs: 'Latència de la compressió del context',
      },
    },
  },
  'cs-CZ': {
    settings: {
      smallModel: {
        contextCompression: 'Komprese kontextu',
        contextCompressionHint:
          'Komprese se při vysokém tlaku na kontext stále spouští automaticky; tento přepínač jen určuje, zda má lehký model dostat první pokus o kompresi.',
        contextCompressionMode: 'Režim komprese',
        contextCompressionModeHint:
          'Komprese se při tlaku na kontext spouští automaticky. Tady se jen volí preferovaná cesta komprese, zatímco záměr aktuálního kola stále vychází z poslední uživatelské zprávy.',
        contextCompressionModeAuto:
          'Automaticky: preferovat kompresi lehkým modelem, při selhání přejít offline',
        contextCompressionModeSmallModel: 'Preferovat lehký model',
        contextCompressionModeOffline: 'Deterministicky offline',
        contextCompressionModeOff: 'Vypnuto',
        contextCompressionSuccessRate: 'Úspěšnost komprese kontextu',
        contextCompressionLatencyMs: 'Latence komprese kontextu',
      },
    },
  },
  'da-DK': {
    settings: {
      smallModel: {
        contextCompression: 'Kontekstkomprimering',
        contextCompressionHint:
          'Komprimering udløses stadig automatisk ved højt kontekstpres; denne kontakt styrer kun, om letvægtsmodellen får første forsøg på komprimeringen.',
        contextCompressionMode: 'Komprimeringstilstand',
        contextCompressionModeHint:
          'Komprimering udløses automatisk ved kontekstpres. Her vælger du kun, hvilken komprimeringssti der foretrækkes, mens hensigten i den aktuelle tur stadig kommer fra den seneste brugerbesked.',
        contextCompressionModeAuto:
          'Auto: foretræk komprimering med letvægtsmodel, fald tilbage til offline',
        contextCompressionModeSmallModel: 'Foretræk letvægtsmodel',
        contextCompressionModeOffline: 'Deterministisk offline',
        contextCompressionModeOff: 'Fra',
        contextCompressionSuccessRate: 'Træfrate for kontekstkomprimering',
        contextCompressionLatencyMs: 'Latens for kontekstkomprimering',
      },
    },
  },
  'de-DE': {
    settings: {
      smallModel: {
        contextCompression: 'Kontextkomprimierung',
        contextCompressionHint:
          'Die Komprimierung wird bei hohem Kontextdruck weiterhin automatisch ausgelöst; dieser Schalter steuert nur, ob das Leichtmodell den ersten Komprimierungsversuch bekommt.',
        contextCompressionMode: 'Komprimierungsmodus',
        contextCompressionModeHint:
          'Die Komprimierung wird bei Kontextdruck automatisch ausgelöst. Hier wird nur festgelegt, welcher Komprimierungspfad bevorzugt wird, während die aktuelle Absicht weiterhin aus der neuesten Nutzernachricht kommt.',
        contextCompressionModeAuto:
          'Automatisch: Leichtmodell-Komprimierung bevorzugen, sonst offline',
        contextCompressionModeSmallModel: 'Leichtmodell bevorzugen',
        contextCompressionModeOffline: 'Deterministisch offline',
        contextCompressionModeOff: 'Aus',
        contextCompressionSuccessRate: 'Trefferquote der Kontextkomprimierung',
        contextCompressionLatencyMs: 'Latenz der Kontextkomprimierung',
      },
    },
  },
  'el-GR': {
    settings: {
      smallModel: {
        contextCompression: 'Συμπίεση συμφραζομένων',
        contextCompressionHint:
          'Η συμπίεση εξακολουθεί να ενεργοποιείται αυτόματα όταν η πίεση συμφραζομένων είναι υψηλή· αυτός ο διακόπτης απλώς ορίζει αν το ελαφρύ μοντέλο θα έχει την πρώτη προσπάθεια συμπίεσης.',
        contextCompressionMode: 'Λειτουργία συμπίεσης',
        contextCompressionModeHint:
          'Η συμπίεση ενεργοποιείται αυτόματα όταν υπάρχει πίεση συμφραζομένων. Εδώ επιλέγεις μόνο ποια διαδρομή συμπίεσης θα προτιμάται, ενώ η πρόθεση του τρέχοντος γύρου εξακολουθεί να προέρχεται από το πιο πρόσφατο μήνυμα του χρήστη.',
        contextCompressionModeAuto:
          'Αυτόματο: προτίμηση στη συμπίεση με ελαφρύ μοντέλο, αλλιώς offline',
        contextCompressionModeSmallModel: 'Προτίμηση ελαφρού μοντέλου',
        contextCompressionModeOffline: 'Ντετερμινιστικό offline',
        contextCompressionModeOff: 'Απενεργοποιημένο',
        contextCompressionSuccessRate: 'Ποσοστό επιτυχίας συμπίεσης συμφραζομένων',
        contextCompressionLatencyMs: 'Καθυστέρηση συμπίεσης συμφραζομένων',
      },
    },
  },
  'en-GB': {
    settings: {
      smallModel: {
        contextCompression: 'Context Compression',
        contextCompressionHint:
          'Compression still triggers automatically under context pressure; this toggle only controls whether the lightweight model gets first pass on that compression.',
        contextCompressionMode: 'Compression Mode',
        contextCompressionModeHint:
          'Compression triggers automatically under context pressure. This only chooses which compression path to prefer, while current-turn intent still comes from the latest user message.',
        contextCompressionModeAuto:
          'Auto: prefer lightweight-model compression, fall back to offline',
        contextCompressionModeSmallModel: 'Prefer Lightweight Model',
        contextCompressionModeOffline: 'Deterministic Offline',
        contextCompressionModeOff: 'Off',
        contextCompressionSuccessRate: 'Context Compression Hit Rate',
        contextCompressionLatencyMs: 'Context Compression Latency',
      },
    },
  },
  'es-ES': {
    settings: {
      smallModel: {
        contextCompression: 'Compresión de contexto',
        contextCompressionHint:
          'La compresión sigue activándose automáticamente cuando la presión de contexto es alta; este interruptor solo controla si el modelo ligero recibe el primer intento de compresión.',
        contextCompressionMode: 'Modo de compresión',
        contextCompressionModeHint:
          'La compresión se activa automáticamente cuando hay presión de contexto. Aquí solo eliges qué ruta de compresión se prefiere, mientras que la intención del turno actual sigue viniendo del último mensaje del usuario.',
        contextCompressionModeAuto:
          'Automático: prioriza la compresión con modelo ligero y vuelve a offline',
        contextCompressionModeSmallModel: 'Priorizar modelo ligero',
        contextCompressionModeOffline: 'Offline determinista',
        contextCompressionModeOff: 'Desactivado',
        contextCompressionSuccessRate: 'Tasa de acierto de compresión de contexto',
        contextCompressionLatencyMs: 'Latencia de compresión de contexto',
      },
    },
  },
  'fr-FR': {
    settings: {
      smallModel: {
        contextCompression: 'Compression du contexte',
        contextCompressionHint:
          'La compression continue de se déclencher automatiquement lorsque la pression de contexte est élevée ; cet interrupteur décide seulement si le modèle léger obtient la première passe de compression.',
        contextCompressionMode: 'Mode de compression',
        contextCompressionModeHint:
          "La compression se déclenche automatiquement sous pression de contexte. Ici, on choisit seulement le chemin de compression à privilégier, tandis que l'intention du tour courant vient toujours du dernier message de l'utilisateur.",
        contextCompressionModeAuto:
          'Auto : privilégier la compression par modèle léger, sinon passer hors ligne',
        contextCompressionModeSmallModel: 'Privilégier le modèle léger',
        contextCompressionModeOffline: 'Hors ligne déterministe',
        contextCompressionModeOff: 'Désactivé',
        contextCompressionSuccessRate: 'Taux de réussite de la compression du contexte',
        contextCompressionLatencyMs: 'Latence de la compression du contexte',
      },
    },
  },
  'ga-IE': {
    settings: {
      smallModel: {
        contextCompression: 'Comhbhrú comhthéacs',
        contextCompressionHint:
          'Spreagtar an comhbhrú go huathoibríoch fós nuair a bhíonn brú ard comhthéacs ann; ní rialaíonn an lasc seo ach an bhfaigheann an tsamhail éadrom an chéad iarracht ar an gcomhbhrú.',
        contextCompressionMode: 'Mód comhbhrúite',
        contextCompressionModeHint:
          'Spreagtar an comhbhrú go huathoibríoch faoi bhrú comhthéacs. Ní roghnaíonn tú anseo ach an cosán comhbhrúite is fearr, agus tagann intinn an turn reatha fós ón teachtaireacht úsáideora is déanaí.',
        contextCompressionModeAuto:
          'Uathoibríoch: tabhair tosaíocht do chomhbhrú leis an tsamhail éadrom, le fallback as líne',
        contextCompressionModeSmallModel: 'Tabhair tosaíocht don tsamhail éadrom',
        contextCompressionModeOffline: 'As líne cinntitheach',
        contextCompressionModeOff: 'Múchta',
        contextCompressionSuccessRate: 'Ráta buailte comhbhrúite comhthéacs',
        contextCompressionLatencyMs: 'Moill comhbhrúite comhthéacs',
      },
    },
  },
  'hr-HR': {
    settings: {
      smallModel: {
        contextCompression: 'Sažimanje konteksta',
        contextCompressionHint:
          'Sažimanje se i dalje automatski pokreće kada je pritisak konteksta visok; ovaj prekidač samo određuje dobiva li lagani model prvi pokušaj sažimanja.',
        contextCompressionMode: 'Način sažimanja',
        contextCompressionModeHint:
          'Sažimanje se automatski pokreće pod pritiskom konteksta. Ovdje samo birate koju putanju sažimanja preferirati, dok namjera trenutnog kruga i dalje dolazi iz najnovije korisničke poruke.',
        contextCompressionModeAuto:
          'Automatski: preferiraj sažimanje laganim modelom, uz povratak na offline',
        contextCompressionModeSmallModel: 'Preferiraj lagani model',
        contextCompressionModeOffline: 'Deterministički offline',
        contextCompressionModeOff: 'Isključeno',
        contextCompressionSuccessRate: 'Stopa uspješnosti sažimanja konteksta',
        contextCompressionLatencyMs: 'Latencija sažimanja konteksta',
      },
    },
  },
  'hu-HU': {
    settings: {
      smallModel: {
        contextCompression: 'Kontextustömörítés',
        contextCompressionHint:
          'A tömörítés nagy kontextusterhelésnél továbbra is automatikusan indul; ez a kapcsoló csak azt szabályozza, hogy a könnyű modell kapja-e az első tömörítési próbálkozást.',
        contextCompressionMode: 'Tömörítési mód',
        contextCompressionModeHint:
          'A tömörítés kontextusnyomás alatt automatikusan indul. Itt csak azt választod ki, melyik tömörítési útvonal legyen előnyben, miközben az aktuális kör szándéka továbbra is a legutóbbi felhasználói üzenetből jön.',
        contextCompressionModeAuto:
          'Automatikus: a könnyű modell tömörítését részesíti előnyben, különben offline',
        contextCompressionModeSmallModel: 'Könnyű modell előnyben',
        contextCompressionModeOffline: 'Determinisztikus offline',
        contextCompressionModeOff: 'Kikapcsolva',
        contextCompressionSuccessRate: 'Kontextustömörítési találati arány',
        contextCompressionLatencyMs: 'Kontextustömörítési késleltetés',
      },
    },
  },
  'it-IT': {
    settings: {
      smallModel: {
        contextCompression: 'Compressione del contesto',
        contextCompressionHint:
          'La compressione continua ad attivarsi automaticamente quando la pressione del contesto e alta; questo interruttore controlla solo se il modello leggero ottiene il primo tentativo di compressione.',
        contextCompressionMode: 'Modalità di compressione',
        contextCompressionModeHint:
          "La compressione si attiva automaticamente quando c'e pressione sul contesto. Qui scegli solo quale percorso di compressione preferire, mentre l'intento del turno corrente continua a provenire dall'ultimo messaggio dell'utente.",
        contextCompressionModeAuto:
          'Automatico: preferisci la compressione con modello leggero, altrimenti offline',
        contextCompressionModeSmallModel: 'Preferisci il modello leggero',
        contextCompressionModeOffline: 'Offline deterministico',
        contextCompressionModeOff: 'Disattivato',
        contextCompressionSuccessRate: 'Tasso di successo della compressione del contesto',
        contextCompressionLatencyMs: 'Latenza della compressione del contesto',
      },
    },
  },
  'ja-JP': {
    settings: {
      smallModel: {
        contextCompression: 'コンテキスト圧縮',
        contextCompressionHint:
          'コンテキスト圧が高い場合でも圧縮は自動で発動し、この切り替えは軽量モデルに最初の圧縮処理を任せるかどうかだけを決めます。',
        contextCompressionMode: '圧縮モード',
        contextCompressionModeHint:
          '圧縮はコンテキスト圧に応じて自動で発動します。ここではどの圧縮経路を優先するかだけを選び、現在ターンの意図は引き続き最新のユーザーメッセージから決まります。',
        contextCompressionModeAuto:
          '自動: 軽量モデル圧縮を優先し、失敗時はオフラインにフォールバック',
        contextCompressionModeSmallModel: '軽量モデルを優先',
        contextCompressionModeOffline: '決定的オフライン',
        contextCompressionModeOff: 'オフ',
        contextCompressionSuccessRate: 'コンテキスト圧縮ヒット率',
        contextCompressionLatencyMs: 'コンテキスト圧縮レイテンシ',
      },
    },
  },
  'ko-KR': {
    settings: {
      smallModel: {
        contextCompression: '컨텍스트 압축',
        contextCompressionHint:
          '컨텍스트 압력이 높아지면 압축은 계속 자동으로 트리거되며, 이 토글은 경량 모델이 그 압축을 먼저 시도할지 여부만 정합니다.',
        contextCompressionMode: '압축 모드',
        contextCompressionModeHint:
          '압축은 컨텍스트 압력에 따라 자동으로 트리거됩니다. 여기서는 어떤 압축 경로를 우선할지만 고르며, 현재 턴의 의도는 여전히 최신 사용자 메시지에서 결정됩니다.',
        contextCompressionModeAuto: '자동: 경량 모델 압축을 우선하고, 실패 시 오프라인으로 폴백',
        contextCompressionModeSmallModel: '경량 모델 우선',
        contextCompressionModeOffline: '결정적 오프라인',
        contextCompressionModeOff: '끔',
        contextCompressionSuccessRate: '컨텍스트 압축 적중률',
        contextCompressionLatencyMs: '컨텍스트 압축 지연 시간',
      },
    },
  },
  'ml-IN': {
    settings: {
      smallModel: {
        contextCompression: 'സന്ദർഭ സംക്ഷേപണം',
        contextCompressionHint:
          'സന്ദർഭ സമ്മർദ്ദം കൂടുതലാകുമ്പോൾ സംക്ഷേപണം ഇപ്പോഴും സ്വയം ട്രിഗർ ചെയ്യും; ലഘു മോഡലിന് ആദ്യ സംക്ഷേപണ ശ്രമം നൽകണോ എന്നത് മാത്രം ഈ സ്വിച്ച് തീരുമാനിക്കുന്നു.',
        contextCompressionMode: 'സംക്ഷേപണ മോഡ്',
        contextCompressionModeHint:
          'സന്ദർഭ സമ്മർദ്ദത്തിൽ സംക്ഷേപണം സ്വയം പ്രവർത്തിക്കും. ഇവിടെ ഏത് സംക്ഷേപണ പാത മുൻഗണിക്കണമെന്ന് മാത്രം തിരഞ്ഞെടുക്കാം; നിലവിലെ ടേൺ ഉദ്ദേശം ഇപ്പോഴും ഏറ്റവും പുതിയ ഉപയോക്തൃ സന്ദേശത്തിൽ നിന്നുതന്നെയാണ് വരുന്നത്.',
        contextCompressionModeAuto:
          'ഓട്ടോ: ലഘു മോഡൽ സംക്ഷേപണം മുൻഗണിക്കുക, ഇല്ലെങ്കിൽ ഓഫ്‌ലൈൻ ഉപയോഗിക്കുക',
        contextCompressionModeSmallModel: 'ലഘു മോഡൽ മുൻഗണിക്കുക',
        contextCompressionModeOffline: 'നിശ്ചിത ഓഫ്‌ലൈൻ',
        contextCompressionModeOff: 'ഓഫ്',
        contextCompressionSuccessRate: 'സന്ദർഭ സംക്ഷേപണ വിജയനിരക്ക്',
        contextCompressionLatencyMs: 'സന്ദർഭ സംക്ഷേപണ വൈകൽ',
      },
    },
  },
  'nb-NO': {
    settings: {
      smallModel: {
        contextCompression: 'Kontekstkomprimering',
        contextCompressionHint:
          'Komprimering utløses fortsatt automatisk når kontekstpresset er høyt; denne bryteren bestemmer bare om lettvektsmodellen får første forsøk på komprimeringen.',
        contextCompressionMode: 'Komprimeringsmodus',
        contextCompressionModeHint:
          'Komprimering utløses automatisk under kontekstpress. Her velger du bare hvilken komprimeringsbane som skal foretrekkes, mens intensjonen i gjeldende tur fortsatt kommer fra den siste brukermeldingen.',
        contextCompressionModeAuto:
          'Auto: foretrekk komprimering med lettvektsmodell, fall tilbake til offline',
        contextCompressionModeSmallModel: 'Foretrekk lettvektsmodell',
        contextCompressionModeOffline: 'Deterministisk offline',
        contextCompressionModeOff: 'Av',
        contextCompressionSuccessRate: 'Treffrate for kontekstkomprimering',
        contextCompressionLatencyMs: 'Latens for kontekstkomprimering',
      },
    },
  },
  'nl-NL': {
    settings: {
      smallModel: {
        contextCompression: 'Contextcompressie',
        contextCompressionHint:
          'Compressie wordt nog steeds automatisch geactiveerd bij hoge contextdruk; deze schakelaar bepaalt alleen of het lichte model de eerste compressiepoging krijgt.',
        contextCompressionMode: 'Compressiemodus',
        contextCompressionModeHint:
          'Compressie wordt automatisch geactiveerd bij contextdruk. Hier kies je alleen welk compressiepad de voorkeur krijgt, terwijl de intentie van de huidige beurt nog steeds uit het nieuwste gebruikersbericht komt.',
        contextCompressionModeAuto:
          'Automatisch: geef voorkeur aan compressie met licht model, val anders terug op offline',
        contextCompressionModeSmallModel: 'Licht model prefereren',
        contextCompressionModeOffline: 'Deterministisch offline',
        contextCompressionModeOff: 'Uit',
        contextCompressionSuccessRate: 'Trefferpercentage contextcompressie',
        contextCompressionLatencyMs: 'Latentie van contextcompressie',
      },
    },
  },
  'pl-PL': {
    settings: {
      smallModel: {
        contextCompression: 'Kompresja kontekstu',
        contextCompressionHint:
          'Kompresja nadal uruchamia się automatycznie przy wysokiej presji kontekstu; ten przełącznik decyduje tylko, czy lekki model dostaje pierwszą próbę kompresji.',
        contextCompressionMode: 'Tryb kompresji',
        contextCompressionModeHint:
          'Kompresja uruchamia się automatycznie pod presją kontekstu. Tutaj wybierasz tylko preferowaną ścieżkę kompresji, a intencja bieżącej tury nadal pochodzi z najnowszej wiadomości użytkownika.',
        contextCompressionModeAuto:
          'Auto: preferuj kompresję lekkim modelem, w razie potrzeby wróć do offline',
        contextCompressionModeSmallModel: 'Preferuj lekki model',
        contextCompressionModeOffline: 'Deterministyczny offline',
        contextCompressionModeOff: 'Wyłączone',
        contextCompressionSuccessRate: 'Skuteczność kompresji kontekstu',
        contextCompressionLatencyMs: 'Opóźnienie kompresji kontekstu',
      },
    },
  },
  'pt-BR': {
    settings: {
      smallModel: {
        contextCompression: 'Compressão de contexto',
        contextCompressionHint:
          'A compressão continua sendo acionada automaticamente quando a pressão de contexto fica alta; este controle só define se o modelo leve recebe a primeira tentativa de compressão.',
        contextCompressionMode: 'Modo de compressão',
        contextCompressionModeHint:
          'A compressão é acionada automaticamente sob pressão de contexto. Aqui você só escolhe qual caminho de compressão deve ser priorizado, enquanto a intenção do turno atual continua vindo da mensagem mais recente do usuário.',
        contextCompressionModeAuto:
          'Automático: priorize a compressão com modelo leve e volte para offline se necessário',
        contextCompressionModeSmallModel: 'Priorizar modelo leve',
        contextCompressionModeOffline: 'Offline determinístico',
        contextCompressionModeOff: 'Desligado',
        contextCompressionSuccessRate: 'Taxa de acerto da compressão de contexto',
        contextCompressionLatencyMs: 'Latência da compressão de contexto',
      },
    },
  },
  'pt-PT': {
    settings: {
      smallModel: {
        contextCompression: 'Compressão de contexto',
        contextCompressionHint:
          'A compressão continua a ser ativada automaticamente quando a pressão de contexto é elevada; este controlo apenas define se o modelo leve recebe a primeira tentativa de compressão.',
        contextCompressionMode: 'Modo de compressão',
        contextCompressionModeHint:
          'A compressão é ativada automaticamente sob pressão de contexto. Aqui escolhes apenas qual caminho de compressão deve ser privilegiado, enquanto a intenção do turno atual continua a vir da mensagem mais recente do utilizador.',
        contextCompressionModeAuto:
          'Automático: dar prioridade à compressão com modelo leve e recuar para offline',
        contextCompressionModeSmallModel: 'Dar prioridade ao modelo leve',
        contextCompressionModeOffline: 'Offline determinístico',
        contextCompressionModeOff: 'Desligado',
        contextCompressionSuccessRate: 'Taxa de acerto da compressão de contexto',
        contextCompressionLatencyMs: 'Latência da compressão de contexto',
      },
    },
  },
  'ro-RO': {
    settings: {
      smallModel: {
        contextCompression: 'Compresia contextului',
        contextCompressionHint:
          'Compresia se declanșează în continuare automat când presiunea contextului este mare; acest comutator decide doar dacă modelul ușor primește prima încercare de compresie.',
        contextCompressionMode: 'Mod de compresie',
        contextCompressionModeHint:
          'Compresia se declanșează automat sub presiunea contextului. Aici alegi doar ce cale de compresie este preferată, iar intenția rundei curente vine în continuare din cel mai recent mesaj al utilizatorului.',
        contextCompressionModeAuto: 'Auto: preferă compresia cu model ușor, apoi revine offline',
        contextCompressionModeSmallModel: 'Preferă modelul ușor',
        contextCompressionModeOffline: 'Offline determinist',
        contextCompressionModeOff: 'Dezactivat',
        contextCompressionSuccessRate: 'Rata de succes a compresiei contextului',
        contextCompressionLatencyMs: 'Latența compresiei contextului',
      },
    },
  },
  'ru-RU': {
    settings: {
      smallModel: {
        contextCompression: 'Сжатие контекста',
        contextCompressionHint:
          'Сжатие по-прежнему запускается автоматически при высокой нагрузке на контекст; этот переключатель лишь определяет, получает ли лёгкая модель первую попытку сжатия.',
        contextCompressionMode: 'Режим сжатия',
        contextCompressionModeHint:
          'Сжатие автоматически запускается при давлении на контекст. Здесь выбирается только предпочтительный путь сжатия, а намерение текущего хода по-прежнему берётся из последнего сообщения пользователя.',
        contextCompressionModeAuto:
          'Авто: предпочитать сжатие лёгкой моделью, иначе переходить в офлайн',
        contextCompressionModeSmallModel: 'Предпочитать лёгкую модель',
        contextCompressionModeOffline: 'Детерминированный офлайн',
        contextCompressionModeOff: 'Выключено',
        contextCompressionSuccessRate: 'Доля успешного сжатия контекста',
        contextCompressionLatencyMs: 'Задержка сжатия контекста',
      },
    },
  },
  'sk-SK': {
    settings: {
      smallModel: {
        contextCompression: 'Komprimácia kontextu',
        contextCompressionHint:
          'Komprimácia sa pri vysokom tlaku na kontext stále spúšťa automaticky; tento prepínač len určuje, či ľahký model dostane prvý pokus o komprimáciu.',
        contextCompressionMode: 'Režim kompresie',
        contextCompressionModeHint:
          'Komprimácia sa pri tlaku na kontext spúšťa automaticky. Tu sa len vyberá preferovaná cesta kompresie, zatiaľ čo zámer aktuálneho kola stále pochádza z poslednej používateľskej správy.',
        contextCompressionModeAuto:
          'Automaticky: uprednostniť kompresiu ľahkým modelom, inak offline',
        contextCompressionModeSmallModel: 'Uprednostniť ľahký model',
        contextCompressionModeOffline: 'Deterministicky offline',
        contextCompressionModeOff: 'Vypnuté',
        contextCompressionSuccessRate: 'Úspešnosť kompresie kontextu',
        contextCompressionLatencyMs: 'Latencia kompresie kontextu',
      },
    },
  },
  'sv-SE': {
    settings: {
      smallModel: {
        contextCompression: 'Kontextkomprimering',
        contextCompressionHint:
          'Komprimering utlöses fortfarande automatiskt när kontexttrycket är högt; den här omkopplaren styr bara om den lätta modellen får första försöket att komprimera.',
        contextCompressionMode: 'Komprimeringsläge',
        contextCompressionModeHint:
          'Komprimering utlöses automatiskt under kontexttryck. Här väljer du bara vilken komprimeringsväg som ska prioriteras, medan avsikten i den aktuella turen fortfarande kommer från det senaste användarmeddelandet.',
        contextCompressionModeAuto:
          'Auto: prioritera komprimering med lätt modell, fall tillbaka till offline',
        contextCompressionModeSmallModel: 'Prioritera lätt modell',
        contextCompressionModeOffline: 'Deterministisk offline',
        contextCompressionModeOff: 'Av',
        contextCompressionSuccessRate: 'Träffsäkerhet för kontextkomprimering',
        contextCompressionLatencyMs: 'Latens för kontextkomprimering',
      },
    },
  },
  'zh-TW': {
    settings: {
      smallModel: {
        contextCompression: '上下文壓縮',
        contextCompressionHint:
          '上下文壓力過高時仍會自動觸發壓縮；這個開關只決定是否優先讓小模型先做第一輪壓縮。',
        contextCompressionMode: '壓縮模式',
        contextCompressionModeHint:
          '達到上下文壓力時會自動壓縮；這裡只決定優先走哪條壓縮路徑，不會改變「最新訊息優先」的安全規則。',
        contextCompressionModeAuto: '自動：優先小模型壓縮，失敗時退回離線壓縮',
        contextCompressionModeSmallModel: '優先小模型',
        contextCompressionModeOffline: '純離線確定性壓縮',
        contextCompressionModeOff: '關閉',
        contextCompressionSuccessRate: '上下文壓縮命中率',
        contextCompressionLatencyMs: '上下文壓縮耗時',
      },
    },
  },
}

export default contextCompressionOverrides
