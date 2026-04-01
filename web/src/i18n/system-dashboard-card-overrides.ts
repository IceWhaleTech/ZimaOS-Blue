import type { LocaleKey } from './locale-catalog'

const systemDashboardCardOverrides = {
  'ca-ES': {
    system: {
      cards: {
        status: {
          compactSubtitle: "Salut del servei principal",
          fullSubtitle: "Temps d'execució del servei",
          summaryOk: "El servei principal respon amb normalitat",
          summaryDegraded: "El servei està actiu, però les comprovacions requereixen atenció",
          summaryError: "Les comprovacions d'estat requereixen atenció immediata",
          summaryUnknown: "S'estan esperant mostres d'estat del temps d'execució",
        },
        uptime: {
          subtitle: 'Disponibilitat del procés',
          footnote: "Des de l'últim inici del procés",
        },
        cpu: {
          awaitingPeak: "S'està esperant la mostra de pic",
          peakWindow: 'Pic de {value}% en 5 minuts',
          caption: 'Pressió de càlcul contínua durant la finestra més recent de 5 minuts',
        },
        heap: {
          subtitle: 'Assignació actual del heap',
          chip: 'Ara',
          footnote: "Empremta actual de l'assignador del heap",
        },
        goroutines: {
          subtitle: 'Càrrega del planificador',
          chip: 'En viu',
          footnote: 'Rutines actives a la finestra actual del planificador',
        },
      },
    },
  },
  'cs-CZ': {
    system: {
      cards: {
        status: {
          compactSubtitle: 'Stav hlavní služby',
          fullSubtitle: 'Běh služby',
          summaryOk: 'Hlavní služba odpovídá normálně',
          summaryDegraded: 'Služba běží, ale kontroly vyžadují pozornost',
          summaryError: 'Kontroly stavu vyžadují okamžitou pozornost',
          summaryUnknown: 'Čeká se na vzorky stavu běhu',
        },
        uptime: {
          subtitle: 'Dostupnost procesu',
          footnote: 'Od posledního spuštění procesu',
        },
        cpu: {
          awaitingPeak: 'Čeká se na špičkový vzorek',
          peakWindow: 'Špička {value}% za 5 minut',
          caption: 'Klouzavé výpočetní zatížení za poslední 5minutové okno',
        },
        heap: {
          subtitle: 'Aktuální alokace haldy',
          chip: 'Nyní',
          footnote: 'Aktuální stopa alokátoru haldy',
        },
        goroutines: {
          subtitle: 'Zatížení plánovače',
          chip: 'Živě',
          footnote: 'Aktivní rutiny v aktuálním okně plánovače',
        },
      },
    },
  },
  'da-DK': {
    system: {
      cards: {
        status: {
          compactSubtitle: 'Kernetjenestens tilstand',
          fullSubtitle: 'Tjenestens køretid',
          summaryOk: 'Kernetjenesten svarer normalt',
          summaryDegraded: 'Tjenesten kører, men kontrollerne kræver opmærksomhed',
          summaryError: 'Sundhedstjek kræver øjeblikkelig opmærksomhed',
          summaryUnknown: 'Afventer prøver på runtime-sundhed',
        },
        uptime: {
          subtitle: 'Procestilgængelighed',
          footnote: 'Siden seneste processtart',
        },
        cpu: {
          awaitingPeak: 'Afventer toppunktmåling',
          peakWindow: 'Top på {value}% over 5 minutter',
          caption: 'Rullende beregningsbelastning over det seneste vindue på 5 minutter',
        },
        heap: {
          subtitle: 'Aktuel heap-allokering',
          chip: 'Nu',
          footnote: 'Aktuelt heap-allokatoraftryk',
        },
        goroutines: {
          subtitle: 'Scheduler-belastning',
          chip: 'Live',
          footnote: 'Aktive rutiner i det aktuelle scheduler-vindue',
        },
      },
    },
  },
  'de-DE': {
    system: {
      cards: {
        status: {
          compactSubtitle: 'Zustand des Kerndienstes',
          fullSubtitle: 'Dienst-Laufzeit',
          summaryOk: 'Kerndienst antwortet normal',
          summaryDegraded: 'Dienst läuft, aber Prüfungen erfordern Aufmerksamkeit',
          summaryError: 'Integritätsprüfungen erfordern sofortige Aufmerksamkeit',
          summaryUnknown: 'Warte auf Laufzeit-Statusdaten',
        },
        uptime: {
          subtitle: 'Prozessverfügbarkeit',
          footnote: 'Seit dem letzten Prozessstart',
        },
        cpu: {
          awaitingPeak: 'Warte auf Spitzenwert',
          peakWindow: 'Spitze {value}% in den letzten 5 Minuten',
          caption: 'Rollierende Rechenlast im jüngsten 5-Minuten-Fenster',
        },
        heap: {
          subtitle: 'Aktuelle Heap-Belegung',
          chip: 'Jetzt',
          footnote: 'Aktueller Footprint des Heap-Allokators',
        },
        goroutines: {
          subtitle: 'Scheduler-Auslastung',
          chip: 'Live',
          footnote: 'Aktive Routinen im aktuellen Scheduler-Fenster',
        },
      },
    },
  },
  'el-GR': {
    system: {
      cards: {
        status: {
          compactSubtitle: 'Υγεία βασικής υπηρεσίας',
          fullSubtitle: 'Χρόνος εκτέλεσης υπηρεσίας',
          summaryOk: 'Η βασική υπηρεσία αποκρίνεται κανονικά',
          summaryDegraded: 'Η υπηρεσία εκτελείται, αλλά οι έλεγχοι χρειάζονται προσοχή',
          summaryError: 'Οι έλεγχοι υγείας χρειάζονται άμεση προσοχή',
          summaryUnknown: 'Αναμονή για δείγματα υγείας χρόνου εκτέλεσης',
        },
        uptime: {
          subtitle: 'Διαθεσιμότητα διεργασίας',
          footnote: 'Από την τελευταία εκκίνηση της διεργασίας',
        },
        cpu: {
          awaitingPeak: 'Αναμονή για δείγμα κορυφής',
          peakWindow: 'Κορυφή {value}% σε 5 λεπτά',
          caption: 'Κυλιόμενη υπολογιστική πίεση στο πιο πρόσφατο παράθυρο 5 λεπτών',
        },
        heap: {
          subtitle: 'Τρέχουσα κατανομή σωρού',
          chip: 'Τώρα',
          footnote: 'Τρέχον αποτύπωμα του κατανεμητή σωρού',
        },
        goroutines: {
          subtitle: 'Φόρτος προγραμματιστή',
          chip: 'Ζωντανά',
          footnote: 'Ενεργές ρουτίνες στο τρέχον παράθυρο του προγραμματιστή',
        },
      },
    },
  },
  'en-GB': {
    system: {
      cards: {
        status: {
          compactSubtitle: 'Core service health',
          fullSubtitle: 'Service runtime',
          summaryOk: 'Core service responding normally',
          summaryDegraded: 'Service is up, but checks need attention',
          summaryError: 'Health checks need immediate attention',
          summaryUnknown: 'Waiting for runtime health samples',
        },
        uptime: {
          subtitle: 'Process availability',
          footnote: 'Since latest process start',
        },
        cpu: {
          awaitingPeak: 'Awaiting peak sample',
          peakWindow: 'Peak {value}% over 5 minutes',
          caption: 'Rolling compute pressure across the most recent 5 minute window',
        },
        heap: {
          subtitle: 'Current heap alloc',
          chip: 'Now',
          footnote: 'Live heap allocator footprint',
        },
        goroutines: {
          subtitle: 'Scheduler load',
          chip: 'Live',
          footnote: 'Active routines in the current scheduler window',
        },
      },
    },
  },
  'en-US': {
    system: {
      cards: {
        status: {
          compactSubtitle: 'Core service health',
          fullSubtitle: 'Service runtime',
          summaryOk: 'Core service responding normally',
          summaryDegraded: 'Service is up, but checks need attention',
          summaryError: 'Health checks need immediate attention',
          summaryUnknown: 'Waiting for runtime health samples',
        },
        uptime: {
          subtitle: 'Process availability',
          footnote: 'Since latest process start',
        },
        cpu: {
          awaitingPeak: 'Awaiting peak sample',
          peakWindow: 'Peak {value}% over 5 minutes',
          caption: 'Rolling compute pressure across the most recent 5 minute window',
        },
        heap: {
          subtitle: 'Current heap alloc',
          chip: 'Now',
          footnote: 'Live heap allocator footprint',
        },
        goroutines: {
          subtitle: 'Scheduler load',
          chip: 'Live',
          footnote: 'Active routines in the current scheduler window',
        },
      },
    },
  },
  'es-ES': {
    system: {
      cards: {
        status: {
          compactSubtitle: 'Estado del servicio principal',
          fullSubtitle: 'Tiempo de ejecución del servicio',
          summaryOk: 'El servicio principal responde con normalidad',
          summaryDegraded: 'El servicio está activo, pero las comprobaciones requieren atención',
          summaryError: 'Las comprobaciones de estado requieren atención inmediata',
          summaryUnknown: 'Esperando muestras de estado del tiempo de ejecución',
        },
        uptime: {
          subtitle: 'Disponibilidad del proceso',
          footnote: 'Desde el último inicio del proceso',
        },
        cpu: {
          awaitingPeak: 'Esperando la muestra de pico',
          peakWindow: 'Pico de {value}% en 5 minutos',
          caption: 'Presión de cálculo continua durante la ventana más reciente de 5 minutos',
        },
        heap: {
          subtitle: 'Asignación actual del montón',
          chip: 'Ahora',
          footnote: 'Huella activa del asignador del montón',
        },
        goroutines: {
          subtitle: 'Carga del planificador',
          chip: 'En vivo',
          footnote: 'Rutinas activas en la ventana actual del planificador',
        },
      },
    },
  },
  'fr-FR': {
    system: {
      cards: {
        status: {
          compactSubtitle: 'État du service principal',
          fullSubtitle: 'Exécution du service',
          summaryOk: 'Le service principal répond normalement',
          summaryDegraded: 'Le service fonctionne, mais certains contrôles demandent de l’attention',
          summaryError: 'Les contrôles de santé nécessitent une attention immédiate',
          summaryUnknown: "En attente d'échantillons de santé d'exécution",
        },
        uptime: {
          subtitle: 'Disponibilité du processus',
          footnote: 'Depuis le dernier démarrage du processus',
        },
        cpu: {
          awaitingPeak: "En attente d'un pic de mesure",
          peakWindow: 'Pic de {value}% sur 5 minutes',
          caption: 'Pression de calcul glissante sur la fenêtre des 5 dernières minutes',
        },
        heap: {
          subtitle: 'Allocation actuelle du tas',
          chip: 'Maintenant',
          footnote: "Empreinte actuelle de l'allocateur de tas",
        },
        goroutines: {
          subtitle: "Charge de l'ordonnanceur",
          chip: 'En direct',
          footnote: "Routines actives dans la fenêtre actuelle de l'ordonnanceur",
        },
      },
    },
  },
  'ga-IE': {
    system: {
      cards: {
        status: {
          compactSubtitle: 'Sláinte na príomhsheirbhíse',
          fullSubtitle: 'Am rite na seirbhíse',
          summaryOk: 'Tá an phríomhsheirbhís ag freagairt de ghnáth',
          summaryDegraded: 'Tá an tseirbhís ag rith, ach teastaíonn aird ó na seiceálacha',
          summaryError: 'Teastaíonn aird láithreach ó na seiceálacha sláinte',
          summaryUnknown: 'Ag fanacht le samplaí sláinte am rite',
        },
        uptime: {
          subtitle: 'Infhaighteacht an phróisis',
          footnote: 'Ó thús an phróisis is déanaí',
        },
        cpu: {
          awaitingPeak: 'Ag fanacht le sampla buaice',
          peakWindow: 'Buaic {value}% thar 5 nóiméad',
          caption: 'Brú ríomhaireachta rollach le linn na fuinneoige 5 nóiméad is déanaí',
        },
        heap: {
          subtitle: 'Leithdháileadh heap reatha',
          chip: 'Anois',
          footnote: 'Lorg beo an leithdháilteora heap',
        },
        goroutines: {
          subtitle: 'Ualach an sceidealaitheora',
          chip: 'Beo',
          footnote: 'Gnáthaimh ghníomhacha i bhfuinneog reatha an sceidealaitheora',
        },
      },
    },
  },
  'hr-HR': {
    system: {
      cards: {
        status: {
          compactSubtitle: 'Zdravlje jezgrene usluge',
          fullSubtitle: 'Vrijeme rada usluge',
          summaryOk: 'Jezgrena usluga odgovara normalno',
          summaryDegraded: 'Usluga radi, ali provjere zahtijevaju pozornost',
          summaryError: 'Provjere zdravlja zahtijevaju hitnu pozornost',
          summaryUnknown: 'Čekaju se uzorci zdravlja vremena rada',
        },
        uptime: {
          subtitle: 'Dostupnost procesa',
          footnote: 'Od posljednjeg pokretanja procesa',
        },
        cpu: {
          awaitingPeak: 'Čeka se vršni uzorak',
          peakWindow: 'Vrh od {value}% tijekom 5 minuta',
          caption: 'Kontinuirano računsko opterećenje u posljednjem prozoru od 5 minuta',
        },
        heap: {
          subtitle: 'Trenutačna alokacija hrpe',
          chip: 'Sada',
          footnote: 'Trenutačni otisak alokatora hrpe',
        },
        goroutines: {
          subtitle: 'Opterećenje raspoređivača',
          chip: 'Uživo',
          footnote: 'Aktivne rutine u trenutačnom prozoru raspoređivača',
        },
      },
    },
  },
  'hu-HU': {
    system: {
      cards: {
        status: {
          compactSubtitle: 'A központi szolgáltatás állapota',
          fullSubtitle: 'A szolgáltatás futási ideje',
          summaryOk: 'A központi szolgáltatás rendben válaszol',
          summaryDegraded: 'A szolgáltatás fut, de az ellenőrzések figyelmet igényelnek',
          summaryError: 'Az állapotellenőrzések azonnali figyelmet igényelnek',
          summaryUnknown: 'Várakozás futásidejű állapotmintákra',
        },
        uptime: {
          subtitle: 'Folyamat rendelkezésre állása',
          footnote: 'Az utolsó folyamatindítás óta',
        },
        cpu: {
          awaitingPeak: 'Csúcsminta várakozás alatt',
          peakWindow: '{value}% csúcs 5 perc alatt',
          caption: 'Gördülő számítási terhelés az elmúlt 5 perces ablakban',
        },
        heap: {
          subtitle: 'Aktuális heap-foglalás',
          chip: 'Most',
          footnote: 'A heap-allokátor aktuális lábnyoma',
        },
        goroutines: {
          subtitle: 'Ütemező terhelése',
          chip: 'Élő',
          footnote: 'Aktív rutinok az aktuális ütemezőablakban',
        },
      },
    },
  },
  'it-IT': {
    system: {
      cards: {
        status: {
          compactSubtitle: 'Stato del servizio core',
          fullSubtitle: 'Runtime del servizio',
          summaryOk: 'Il servizio core risponde normalmente',
          summaryDegraded: 'Il servizio è attivo, ma i controlli richiedono attenzione',
          summaryError: 'I controlli di salute richiedono attenzione immediata',
          summaryUnknown: 'In attesa di campioni di stato del runtime',
        },
        uptime: {
          subtitle: 'Disponibilità del processo',
          footnote: "Dall'ultimo avvio del processo",
        },
        cpu: {
          awaitingPeak: 'In attesa del campione di picco',
          peakWindow: 'Picco di {value}% negli ultimi 5 minuti',
          caption: 'Pressione di calcolo continua nella finestra più recente di 5 minuti',
        },
        heap: {
          subtitle: 'Allocazione heap corrente',
          chip: 'Ora',
          footnote: "Impronta attiva dell'allocatore heap",
        },
        goroutines: {
          subtitle: 'Carico dello scheduler',
          chip: 'In tempo reale',
          footnote: 'Routine attive nella finestra corrente dello scheduler',
        },
      },
    },
  },
  'ja-JP': {
    system: {
      cards: {
        status: {
          compactSubtitle: 'コアサービスの健全性',
          fullSubtitle: 'サービスランタイム',
          summaryOk: 'コアサービスは正常に応答しています',
          summaryDegraded: 'サービスは稼働中ですが、チェック項目に注意が必要です',
          summaryError: 'ヘルスチェックに直ちに対応する必要があります',
          summaryUnknown: 'ランタイムのヘルスサンプルを待機しています',
        },
        uptime: {
          subtitle: 'プロセス可用性',
          footnote: '直近のプロセス起動以降',
        },
        cpu: {
          awaitingPeak: 'ピークサンプルを待機中',
          peakWindow: '5 分間のピーク {value}%',
          caption: '直近 5 分間ウィンドウの継続的な計算負荷',
        },
        heap: {
          subtitle: '現在のヒープ割り当て',
          chip: '現在',
          footnote: 'ライブヒープアロケータのフットプリント',
        },
        goroutines: {
          subtitle: 'スケジューラ負荷',
          chip: 'ライブ',
          footnote: '現在のスケジューラウィンドウ内のアクティブルーチン',
        },
      },
    },
  },
  'ko-KR': {
    system: {
      cards: {
        status: {
          compactSubtitle: '코어 서비스 상태',
          fullSubtitle: '서비스 런타임',
          summaryOk: '코어 서비스가 정상적으로 응답하고 있습니다',
          summaryDegraded: '서비스는 실행 중이지만 점검 항목에 주의가 필요합니다',
          summaryError: '상태 점검에 즉시 대응이 필요합니다',
          summaryUnknown: '런타임 상태 샘플을 기다리는 중입니다',
        },
        uptime: {
          subtitle: '프로세스 가용성',
          footnote: '가장 최근 프로세스 시작 이후',
        },
        cpu: {
          awaitingPeak: '최대값 샘플을 기다리는 중',
          peakWindow: '5분 동안 최대 {value}%',
          caption: '최근 5분 창의 순환 계산 부하',
        },
        heap: {
          subtitle: '현재 힙 할당',
          chip: '현재',
          footnote: '실시간 힙 할당기 사용량',
        },
        goroutines: {
          subtitle: '스케줄러 부하',
          chip: '실시간',
          footnote: '현재 스케줄러 창의 활성 루틴',
        },
      },
    },
  },
  'ml-IN': {
    system: {
      cards: {
        status: {
          compactSubtitle: 'കോർ സേവനത്തിന്റെ ആരോഗ്യനില',
          fullSubtitle: 'സേവന റൺടൈം',
          summaryOk: 'കോർ സേവനം സാധാരണമായി പ്രതികരിക്കുന്നു',
          summaryDegraded: 'സേവനം പ്രവർത്തിക്കുന്നുണ്ടെങ്കിലും പരിശോധനകൾ ശ്രദ്ധിക്കണം',
          summaryError: 'ഹെൽത്ത് പരിശോധനകൾക്ക് അടിയന്തര ശ്രദ്ധ ആവശ്യമാണ്',
          summaryUnknown: 'റൺടൈം ആരോഗ്യ സാമ്പിളുകൾ കാത്തിരിക്കുന്നു',
        },
        uptime: {
          subtitle: 'പ്രോസസ് ലഭ്യത',
          footnote: 'അവസാന പ്രോസസ് ആരംഭം മുതൽ',
        },
        cpu: {
          awaitingPeak: 'പീക്ക് സാമ്പിൾ കാത്തിരിക്കുന്നു',
          peakWindow: '5 മിനിറ്റിൽ പരമാവധി {value}%',
          caption: 'ഏറ്റവും പുതിയ 5 മിനിറ്റ് വിൻഡോയിലെ റോളിംഗ് കമ്പ്യൂട്ട് ഭാരം',
        },
        heap: {
          subtitle: 'നിലവിലെ ഹീപ്പ് അലോക്കേഷൻ',
          chip: 'ഇപ്പോൾ',
          footnote: 'ലൈവ് ഹീപ്പ് അലോക്കേറ്റർ ഫുട്പ്രിന്റ്',
        },
        goroutines: {
          subtitle: 'ഷെഡ്യൂളർ ലോഡ്',
          chip: 'ലൈവ്',
          footnote: 'നിലവിലെ ഷെഡ്യൂളർ വിൻഡോയിലെ സജീവ റൂട്ടീനുകൾ',
        },
      },
    },
  },
  'nb-NO': {
    system: {
      cards: {
        status: {
          compactSubtitle: 'Helsen til kjernetjenesten',
          fullSubtitle: 'Tjenestens kjøretid',
          summaryOk: 'Kjernetjenesten svarer normalt',
          summaryDegraded: 'Tjenesten kjører, men kontrollene trenger oppmerksomhet',
          summaryError: 'Helsekontroller krever umiddelbar oppmerksomhet',
          summaryUnknown: 'Venter på prøver for kjøretidshelse',
        },
        uptime: {
          subtitle: 'Prosesstilgjengelighet',
          footnote: 'Siden siste prosessstart',
        },
        cpu: {
          awaitingPeak: 'Venter på toppmåling',
          peakWindow: 'Topp på {value}% over 5 minutter',
          caption: 'Rullerende beregningsbelastning i det siste 5-minuttersvinduet',
        },
        heap: {
          subtitle: 'Gjeldende heap-allokering',
          chip: 'Nå',
          footnote: 'Gjeldende heap-allokatoravtrykk',
        },
        goroutines: {
          subtitle: 'Planleggerbelastning',
          chip: 'Live',
          footnote: 'Aktive rutiner i gjeldende planleggervindu',
        },
      },
    },
  },
  'nl-NL': {
    system: {
      cards: {
        status: {
          compactSubtitle: 'Status van de kernservice',
          fullSubtitle: 'Service-runtime',
          summaryOk: 'Kernservice reageert normaal',
          summaryDegraded: 'De service draait, maar controles vragen aandacht',
          summaryError: 'Gezondheidscontroles vereisen onmiddellijke aandacht',
          summaryUnknown: 'Wachten op runtime-statusmetingen',
        },
        uptime: {
          subtitle: 'Beschikbaarheid van het proces',
          footnote: 'Sinds de laatste processtart',
        },
        cpu: {
          awaitingPeak: 'Wachten op piekmeting',
          peakWindow: 'Piek van {value}% over 5 minuten',
          caption: 'Doorlopende rekenbelasting in het meest recente venster van 5 minuten',
        },
        heap: {
          subtitle: 'Huidige heapallocatie',
          chip: 'Nu',
          footnote: 'Huidige footprint van de heap-allocator',
        },
        goroutines: {
          subtitle: 'Schedulerbelasting',
          chip: 'Live',
          footnote: 'Actieve routines in het huidige scheduler-venster',
        },
      },
    },
  },
  'pl-PL': {
    system: {
      cards: {
        status: {
          compactSubtitle: 'Stan głównej usługi',
          fullSubtitle: 'Czas działania usługi',
          summaryOk: 'Główna usługa odpowiada prawidłowo',
          summaryDegraded: 'Usługa działa, ale kontrole wymagają uwagi',
          summaryError: 'Kontrole stanu wymagają natychmiastowej uwagi',
          summaryUnknown: 'Oczekiwanie na próbki stanu środowiska uruchomieniowego',
        },
        uptime: {
          subtitle: 'Dostępność procesu',
          footnote: 'Od ostatniego uruchomienia procesu',
        },
        cpu: {
          awaitingPeak: 'Oczekiwanie na próbkę szczytową',
          peakWindow: 'Szczyt {value}% w ciągu 5 minut',
          caption: 'Kroczące obciążenie obliczeniowe w ostatnim 5-minutowym oknie',
        },
        heap: {
          subtitle: 'Bieżąca alokacja sterty',
          chip: 'Teraz',
          footnote: 'Bieżący ślad alokatora sterty',
        },
        goroutines: {
          subtitle: 'Obciążenie planisty',
          chip: 'Na żywo',
          footnote: 'Aktywne procedury w bieżącym oknie planisty',
        },
      },
    },
  },
  'pt-BR': {
    system: {
      cards: {
        status: {
          compactSubtitle: 'Saúde do serviço principal',
          fullSubtitle: 'Runtime do serviço',
          summaryOk: 'O serviço principal está respondendo normalmente',
          summaryDegraded: 'O serviço está ativo, mas as verificações precisam de atenção',
          summaryError: 'As verificações de integridade exigem atenção imediata',
          summaryUnknown: 'Aguardando amostras de integridade do runtime',
        },
        uptime: {
          subtitle: 'Disponibilidade do processo',
          footnote: 'Desde a última inicialização do processo',
        },
        cpu: {
          awaitingPeak: 'Aguardando amostra de pico',
          peakWindow: 'Pico de {value}% em 5 minutos',
          caption: 'Pressão de computação contínua na janela mais recente de 5 minutos',
        },
        heap: {
          subtitle: 'Alocação atual de heap',
          chip: 'Agora',
          footnote: 'Pegada atual do alocador de heap',
        },
        goroutines: {
          subtitle: 'Carga do escalonador',
          chip: 'Ao vivo',
          footnote: 'Rotinas ativas na janela atual do escalonador',
        },
      },
    },
  },
  'pt-PT': {
    system: {
      cards: {
        status: {
          compactSubtitle: 'Estado do serviço principal',
          fullSubtitle: 'Runtime do serviço',
          summaryOk: 'O serviço principal está a responder normalmente',
          summaryDegraded: 'O serviço está ativo, mas as verificações precisam de atenção',
          summaryError: 'As verificações de integridade exigem atenção imediata',
          summaryUnknown: 'A aguardar amostras de integridade do runtime',
        },
        uptime: {
          subtitle: 'Disponibilidade do processo',
          footnote: 'Desde o último arranque do processo',
        },
        cpu: {
          awaitingPeak: 'A aguardar amostra de pico',
          peakWindow: 'Pico de {value}% em 5 minutos',
          caption: 'Pressão de computação contínua na janela mais recente de 5 minutos',
        },
        heap: {
          subtitle: 'Alocação atual da heap',
          chip: 'Agora',
          footnote: 'Pegada atual do alocador da heap',
        },
        goroutines: {
          subtitle: 'Carga do escalonador',
          chip: 'Ao vivo',
          footnote: 'Rotinas ativas na janela atual do escalonador',
        },
      },
    },
  },
  'ro-RO': {
    system: {
      cards: {
        status: {
          compactSubtitle: 'Starea serviciului principal',
          fullSubtitle: 'Timpul de rulare al serviciului',
          summaryOk: 'Serviciul principal răspunde normal',
          summaryDegraded: 'Serviciul este activ, dar verificările necesită atenție',
          summaryError: 'Verificările de sănătate necesită atenție imediată',
          summaryUnknown: 'Se așteaptă mostre de sănătate pentru runtime',
        },
        uptime: {
          subtitle: 'Disponibilitatea procesului',
          footnote: 'De la ultima pornire a procesului',
        },
        cpu: {
          awaitingPeak: 'Se așteaptă eșantionul de vârf',
          peakWindow: 'Vârf de {value}% în 5 minute',
          caption: 'Presiune de calcul continuă în cea mai recentă fereastră de 5 minute',
        },
        heap: {
          subtitle: 'Alocarea curentă a heap-ului',
          chip: 'Acum',
          footnote: 'Amprenta curentă a alocatorului heap',
        },
        goroutines: {
          subtitle: 'Încărcarea planificatorului',
          chip: 'În timp real',
          footnote: 'Rutine active în fereastra curentă a planificatorului',
        },
      },
    },
  },
  'ru-RU': {
    system: {
      cards: {
        status: {
          compactSubtitle: 'Состояние основного сервиса',
          fullSubtitle: 'Время работы сервиса',
          summaryOk: 'Основной сервис отвечает нормально',
          summaryDegraded: 'Сервис работает, но проверки требуют внимания',
          summaryError: 'Проверки состояния требуют немедленного внимания',
          summaryUnknown: 'Ожидание выборок состояния времени выполнения',
        },
        uptime: {
          subtitle: 'Доступность процесса',
          footnote: 'С момента последнего запуска процесса',
        },
        cpu: {
          awaitingPeak: 'Ожидание пикового значения',
          peakWindow: 'Пик {value}% за 5 минут',
          caption: 'Скользящая вычислительная нагрузка за последние 5 минут',
        },
        heap: {
          subtitle: 'Текущее выделение кучи',
          chip: 'Сейчас',
          footnote: 'Текущий объём аллокатора кучи',
        },
        goroutines: {
          subtitle: 'Нагрузка планировщика',
          chip: 'В реальном времени',
          footnote: 'Активные процедуры в текущем окне планировщика',
        },
      },
    },
  },
  'sk-SK': {
    system: {
      cards: {
        status: {
          compactSubtitle: 'Stav hlavnej služby',
          fullSubtitle: 'Bežiaci čas služby',
          summaryOk: 'Hlavná služba odpovedá normálne',
          summaryDegraded: 'Služba beží, ale kontroly vyžadujú pozornosť',
          summaryError: 'Kontroly stavu vyžadujú okamžitú pozornosť',
          summaryUnknown: 'Čaká sa na vzorky stavu runtime',
        },
        uptime: {
          subtitle: 'Dostupnosť procesu',
          footnote: 'Od posledného spustenia procesu',
        },
        cpu: {
          awaitingPeak: 'Čaká sa na špičkovú vzorku',
          peakWindow: 'Špička {value}% za 5 minút',
          caption: 'Kĺzavé výpočtové zaťaženie za posledné 5-minútové okno',
        },
        heap: {
          subtitle: 'Aktuálna alokácia haldy',
          chip: 'Teraz',
          footnote: 'Aktuálna stopa alokátora haldy',
        },
        goroutines: {
          subtitle: 'Zaťaženie plánovača',
          chip: 'Naživo',
          footnote: 'Aktívne rutiny v aktuálnom okne plánovača',
        },
      },
    },
  },
  'sv-SE': {
    system: {
      cards: {
        status: {
          compactSubtitle: 'Hälsa för kärntjänsten',
          fullSubtitle: 'Tjänstens körtid',
          summaryOk: 'Kärntjänsten svarar normalt',
          summaryDegraded: 'Tjänsten körs, men kontrollerna behöver uppmärksamhet',
          summaryError: 'Hälsokontrollerna kräver omedelbar uppmärksamhet',
          summaryUnknown: 'Väntar på hälsoprover för körtiden',
        },
        uptime: {
          subtitle: 'Processtillgänglighet',
          footnote: 'Sedan senaste processstart',
        },
        cpu: {
          awaitingPeak: 'Väntar på toppvärde',
          peakWindow: 'Topp på {value}% under 5 minuter',
          caption: 'Rullande beräkningsbelastning under det senaste 5-minutersfönstret',
        },
        heap: {
          subtitle: 'Aktuell heap-allokering',
          chip: 'Nu',
          footnote: 'Aktuellt heap-allokeringsfotavtryck',
        },
        goroutines: {
          subtitle: 'Schemaläggarbelastning',
          chip: 'Live',
          footnote: 'Aktiva rutiner i det aktuella schemaläggarfönstret',
        },
      },
    },
  },
  'zh-CN': {
    system: {
      cards: {
        status: {
          compactSubtitle: '核心服务健康',
          fullSubtitle: '服务运行时',
          summaryOk: '核心服务响应正常',
          summaryDegraded: '服务运行中，但检查项需要关注',
          summaryError: '健康检查需要立即处理',
          summaryUnknown: '等待运行时健康采样',
        },
        uptime: {
          subtitle: '进程可用性',
          footnote: '自最近一次进程启动以来',
        },
        cpu: {
          awaitingPeak: '等待峰值采样',
          peakWindow: '5 分钟峰值 {value}%',
          caption: '最近 5 分钟窗口内的滚动计算压力',
        },
        heap: {
          subtitle: '当前堆内存分配',
          chip: '当前',
          footnote: '实时堆分配器占用',
        },
        goroutines: {
          subtitle: '调度器负载',
          chip: '实时',
          footnote: '当前调度窗口中的活跃协程',
        },
      },
    },
  },
  'zh-TW': {
    system: {
      cards: {
        status: {
          compactSubtitle: '核心服務健康狀態',
          fullSubtitle: '服務執行階段',
          summaryOk: '核心服務回應正常',
          summaryDegraded: '服務正在執行，但檢查項目需要留意',
          summaryError: '健康檢查需要立即處理',
          summaryUnknown: '等待執行階段健康樣本',
        },
        uptime: {
          subtitle: '程序可用性',
          footnote: '自最近一次程序啟動以來',
        },
        cpu: {
          awaitingPeak: '等待峰值樣本',
          peakWindow: '5 分鐘峰值 {value}%',
          caption: '最近 5 分鐘視窗內的滾動運算壓力',
        },
        heap: {
          subtitle: '目前堆記憶體配置',
          chip: '目前',
          footnote: '即時堆配置器占用',
        },
        goroutines: {
          subtitle: '排程器負載',
          chip: '即時',
          footnote: '目前排程視窗中的活躍協程',
        },
      },
    },
  },
} satisfies Partial<Record<LocaleKey, Record<string, unknown>>>

export default systemDashboardCardOverrides
