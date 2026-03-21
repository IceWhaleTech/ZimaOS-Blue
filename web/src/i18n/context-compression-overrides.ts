import type { LocaleMessages } from './merge'

const contextCompressionOverrides: Record<string, LocaleMessages> = {
  'ca-ES': {
    settings: {
      smallModel: {
        contextCompression: 'Compressió del context',
        contextCompressionHint:
          "Permet que el model lleuger comprimeixi l'historial llarg, mentre que l'últim missatge de l'usuari continua decidint què passa ara.",
        contextCompressionMode: 'Mode de compressió',
        contextCompressionModeHint:
          "Això només tria el camí de compressió. La intenció del torn actual continua venint de l'últim missatge de l'usuari.",
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
          'Umožní lehkému modelu komprimovat dlouhou historii, zatímco o tom, co se má dělat teď, stále rozhoduje poslední uživatelská zpráva.',
        contextCompressionMode: 'Režim komprese',
        contextCompressionModeHint:
          'Tímto se vybírá pouze cesta komprese. Záměr aktuálního kola stále vychází z poslední uživatelské zprávy.',
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
          'Lader letvægtsmodellen komprimere lang historik, mens den seneste brugerbesked stadig afgør, hvad der skal ske nu.',
        contextCompressionMode: 'Komprimeringstilstand',
        contextCompressionModeHint:
          'Dette vælger kun komprimeringsstien. Hensigten i den aktuelle tur kommer stadig fra den seneste brugerbesked.',
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
          'Lässt das Leichtmodell lange Verläufe komprimieren, während die neueste Nutzernachricht weiterhin bestimmt, was jetzt passiert.',
        contextCompressionMode: 'Komprimierungsmodus',
        contextCompressionModeHint:
          'Dies wählt nur den Komprimierungspfad. Die aktuelle Absicht kommt weiterhin aus der neuesten Nutzernachricht.',
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
          'Επιτρέπει στο ελαφρύ μοντέλο να συμπιέζει μεγάλο ιστορικό, ενώ το πιο πρόσφατο μήνυμα του χρήστη συνεχίζει να καθορίζει τι γίνεται τώρα.',
        contextCompressionMode: 'Λειτουργία συμπίεσης',
        contextCompressionModeHint:
          'Αυτό επιλέγει μόνο τη διαδρομή συμπίεσης. Η πρόθεση του τρέχοντος γύρου εξακολουθεί να προέρχεται από το πιο πρόσφατο μήνυμα του χρήστη.',
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
          'Lets the lightweight model compress long history, while the latest user message still decides what happens now.',
        contextCompressionMode: 'Compression Mode',
        contextCompressionModeHint:
          'This chooses the compression path only. Current-turn intent still comes from the latest user message.',
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
          'Permite que el modelo ligero comprima historiales largos, mientras que el último mensaje del usuario sigue decidiendo qué ocurre ahora.',
        contextCompressionMode: 'Modo de compresión',
        contextCompressionModeHint:
          'Esto solo elige la ruta de compresión. La intención del turno actual sigue viniendo del último mensaje del usuario.',
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
          "Permet au modèle léger de compresser un long historique, tandis que le dernier message de l'utilisateur continue de décider de ce qui se passe maintenant.",
        contextCompressionMode: 'Mode de compression',
        contextCompressionModeHint:
          "Ceci choisit uniquement le chemin de compression. L'intention du tour courant vient toujours du dernier message de l'utilisateur.",
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
          'Ligeann sé don tsamhail éadrom stair fhada a chomhbhrú, agus leanann an teachtaireacht úsáideora is déanaí ag cinneadh cad a tharlaíonn anois.',
        contextCompressionMode: 'Mód comhbhrúite',
        contextCompressionModeHint:
          'Ní roghnaíonn sé seo ach an bealach comhbhrúite. Tagann intinn an turn reatha fós ón teachtaireacht úsáideora is déanaí.',
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
          'Dopušta laganom modelu da sažme dugu povijest, dok najnovija korisnička poruka i dalje odlučuje što se sada radi.',
        contextCompressionMode: 'Način sažimanja',
        contextCompressionModeHint:
          'Ovo bira samo put sažimanja. Namjera trenutnog kruga i dalje dolazi iz najnovije korisničke poruke.',
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
          'Lehetővé teszi, hogy a könnyű modell összetömörítse a hosszú előzményeket, miközben továbbra is a legutóbbi felhasználói üzenet dönti el, mi történjen most.',
        contextCompressionMode: 'Tömörítési mód',
        contextCompressionModeHint:
          'Ez csak a tömörítési útvonalat választja ki. Az aktuális kör szándéka továbbra is a legutóbbi felhasználói üzenetből jön.',
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
          "Consente al modello leggero di comprimere una cronologia lunga, mentre l'ultimo messaggio dell'utente continua a decidere cosa succede adesso.",
        contextCompressionMode: 'Modalità di compressione',
        contextCompressionModeHint:
          "Questo sceglie solo il percorso di compressione. L'intento del turno corrente continua a provenire dall'ultimo messaggio dell'utente.",
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
          '軽量モデルで長い履歴を圧縮できますが、今何をするかは引き続き最新のユーザーメッセージだけが決めます。',
        contextCompressionMode: '圧縮モード',
        contextCompressionModeHint:
          'ここで選ぶのは圧縮経路だけです。現在ターンの意図は引き続き最新のユーザーメッセージから決まります。',
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
          '경량 모델이 긴 기록을 압축하도록 하되, 지금 무엇을 할지는 계속 최신 사용자 메시지가 결정합니다.',
        contextCompressionMode: '압축 모드',
        contextCompressionModeHint:
          '여기서는 압축 경로만 선택합니다. 현재 턴의 의도는 여전히 최신 사용자 메시지에서 결정됩니다.',
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
          'ദൈർഘ്യമേറിയ ചരിത്രം ലഘു മോഡൽ ചുരുക്കാൻ അനുവദിക്കും; എന്നാൽ ഇപ്പോൾ എന്ത് ചെയ്യണമെന്നത് അവസാന ഉപയോക്തൃ സന്ദേശം തന്നെയാണ് തീരുമാനിക്കുന്നത്.',
        contextCompressionMode: 'സംക്ഷേപണ മോഡ്',
        contextCompressionModeHint:
          'ഇത് സംക്ഷേപണ പാത മാത്രം തിരഞ്ഞെടുക്കുന്നു. നിലവിലെ ടേൺ ഉദ്ദേശം ഇപ്പോഴും ഏറ്റവും പുതിയ ഉപയോക്തൃ സന്ദേശത്തിൽ നിന്നാണ് വരുന്നത്.',
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
          'Lar lettvektsmodellen komprimere lang historikk, mens den siste brukermeldingen fortsatt bestemmer hva som skal skje nå.',
        contextCompressionMode: 'Komprimeringsmodus',
        contextCompressionModeHint:
          'Dette velger bare komprimeringsbanen. Intensjonen i gjeldende tur kommer fortsatt fra den siste brukermeldingen.',
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
          'Laat het lichte model een lange geschiedenis comprimeren, terwijl het nieuwste gebruikersbericht nog steeds bepaalt wat er nu gebeurt.',
        contextCompressionMode: 'Compressiemodus',
        contextCompressionModeHint:
          'Dit kiest alleen het compressiepad. De intentie van de huidige beurt komt nog steeds uit het nieuwste gebruikersbericht.',
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
          'Pozwala lekkim modelom kompresować długą historię, a o tym, co zrobić teraz, nadal decyduje najnowsza wiadomość użytkownika.',
        contextCompressionMode: 'Tryb kompresji',
        contextCompressionModeHint:
          'To wybiera tylko ścieżkę kompresji. Intencja bieżącej tury nadal pochodzi z najnowszej wiadomości użytkownika.',
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
          'Permite que o modelo leve comprima um histórico longo, enquanto a mensagem mais recente do usuário continua decidindo o que acontece agora.',
        contextCompressionMode: 'Modo de compressão',
        contextCompressionModeHint:
          'Isso escolhe apenas o caminho de compressão. A intenção do turno atual ainda vem da mensagem mais recente do usuário.',
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
          'Permite que o modelo leve comprima um histórico longo, enquanto a mensagem mais recente do utilizador continua a decidir o que acontece agora.',
        contextCompressionMode: 'Modo de compressão',
        contextCompressionModeHint:
          'Isto escolhe apenas o caminho de compressão. A intenção do turno atual continua a vir da mensagem mais recente do utilizador.',
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
          'Permite modelului ușor să comprime un istoric lung, în timp ce cel mai recent mesaj al utilizatorului decide în continuare ce se întâmplă acum.',
        contextCompressionMode: 'Mod de compresie',
        contextCompressionModeHint:
          'Aici se alege doar calea de compresie. Intenția rundei curente vine în continuare din cel mai recent mesaj al utilizatorului.',
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
          'Позволяет лёгкой модели сжимать длинную историю, при этом последнее сообщение пользователя по-прежнему определяет, что делать сейчас.',
        contextCompressionMode: 'Режим сжатия',
        contextCompressionModeHint:
          'Здесь выбирается только путь сжатия. Намерение текущего хода по-прежнему берётся из последнего сообщения пользователя.',
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
          'Umožní ľahkému modelu komprimovať dlhú históriu, pričom o tom, čo sa má robiť teraz, stále rozhoduje posledná používateľská správa.',
        contextCompressionMode: 'Režim kompresie',
        contextCompressionModeHint:
          'Toto vyberá iba cestu kompresie. Zámer aktuálneho kola stále pochádza z poslednej používateľskej správy.',
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
          'Låter den lätta modellen komprimera lång historik, medan det senaste användarmeddelandet fortfarande avgör vad som ska hända nu.',
        contextCompressionMode: 'Komprimeringsläge',
        contextCompressionModeHint:
          'Det här väljer bara komprimeringsvägen. Avsikten i den aktuella turen kommer fortfarande från det senaste användarmeddelandet.',
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
          '讓小模型參與長歷史壓縮，但當前輪要做什麼仍然只由最新的使用者訊息決定。',
        contextCompressionMode: '壓縮模式',
        contextCompressionModeHint:
          '這裡只決定走哪一條壓縮路徑，不會改變「最新訊息優先」的安全規則。',
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
