import type { LocaleKey } from './locale-catalog'

const dashboardCardCopyOverrides = {
  'ca-ES': {
    system: {
      cards: {
        memoryChart: {
          compactSubtitle: 'Marca màxima dels darrers 5 minuts',
          awaitingSample: 'Esperant la mostra recent',
          currentValue: 'Actual {value}',
          chartSubtitle: "Empremta de l'assignador durant els darrers 5 minuts",
          chartCaption:
            "Assignació amb suport de heap mostrejada a la finestra d'execució més recent",
        },
        heapChart: {
          chartSubtitle: 'Assignació gestionada del heap durant els darrers 5 minuts',
          chartCaption:
            "Creixement i retenció del heap seguits dins la finestra d'execució més recent",
        },
        goroutinesChart: {
          chartSubtitle: 'Concurrència del planificador durant els darrers 5 minuts',
          chartCaption:
            "Treball concurrent del temps d'execució i pressió del planificador durant la finestra de mostres més recent",
        },
        info: {
          subtitle: 'Context de la versió del runtime i del pool de treballadors',
          versionFootnote: 'Versió del servei desplegat actualment',
          workerPool: 'Pool de treballadors',
          runningWorkers: '{active} / {total} en execució',
          capacityFootnote:
            'Treballadors en execució en comparació amb la capacitat configurada del runtime',
        },
      },
    },
    metrics: {
      cards: {
        overview: { subtitle: "Resum d'ús" },
        modelStats: { subtitle: '{count} models seguits' },
      },
    },
    mediaStats: {
      cards: { subtitle: 'Activitat de generació' },
    },
    settings: {
      failover: {
        errorTypes: {
          timeout: "Temps d'espera esgotat",
          unknown: 'Desconegut',
        },
        chips: {
          healthy: 'Correcte',
          halfOpen: 'Semiobert',
          open: 'Obert',
        },
      },
    },
  },
  'cs-CZ': {
    system: {
      cards: {
        memoryChart: {
          compactSubtitle: 'Nejvyšší hodnota za 5 minut',
          awaitingSample: 'Čeká se na poslední vzorek',
          currentValue: 'Aktuálně {value}',
          chartSubtitle: 'Stopa alokátoru za posledních 5 minut',
          chartCaption: 'Alokace podporovaná haldou vzorkovaná v posledním běhovém okně',
        },
        heapChart: {
          chartSubtitle: 'Spravovaná alokace haldy za posledních 5 minut',
          chartCaption: 'Sledovaný růst a retence haldy v posledním běhovém okně',
        },
        goroutinesChart: {
          chartSubtitle: 'Souběžnost plánovače za posledních 5 minut',
          chartCaption: 'Souběžná práce běhu a tlak plánovače v posledním vzorkovacím okně',
        },
        info: {
          subtitle: 'Kontext verze runtime a fondu workerů',
          versionFootnote: 'Aktuálně nasazená verze služby',
          workerPool: 'Fond workerů',
          runningWorkers: '{active} / {total} běží',
          capacityFootnote: 'Běžící workery vůči nastavené kapacitě runtime',
        },
      },
    },
    metrics: {
      cards: {
        overview: { subtitle: 'Přehled využití' },
        modelStats: { subtitle: '{count} sledovaných modelů' },
      },
    },
    mediaStats: {
      cards: { subtitle: 'Aktivita generování' },
    },
    settings: {
      failover: {
        errorTypes: {
          timeout: 'Vypršel časový limit',
          unknown: 'Neznámé',
        },
        chips: {
          healthy: 'OK',
          halfOpen: 'Polootevřený',
          open: 'Otevřený',
        },
      },
    },
  },
  'da-DK': {
    system: {
      cards: {
        memoryChart: {
          compactSubtitle: 'Højeste niveau over 5 minutter',
          awaitingSample: 'Afventer seneste prøve',
          currentValue: 'Aktuel {value}',
          chartSubtitle: 'Allokatorens aftryk over de seneste 5 minutter',
          chartCaption: 'Heap-understøttet allokering målt i det seneste runtime-vindue',
        },
        heapChart: {
          chartSubtitle: 'Administreret heap-allokering over de seneste 5 minutter',
          chartCaption: 'Sporer heap-vækst og fastholdelse i det seneste runtime-vindue',
        },
        goroutinesChart: {
          chartSubtitle: 'Scheduler-samtidighed over de seneste 5 minutter',
          chartCaption:
            'Samtidigt runtime-arbejde og scheduler-pres i det seneste prøvevindue',
        },
        info: {
          subtitle: 'Kontekst for runtime-version og worker-pool',
          versionFootnote: 'Nuværende deployede serviceversion',
          workerPool: 'Worker-pool',
          runningWorkers: '{active} / {total} kører',
          capacityFootnote: 'Kørende workers i forhold til den konfigurerede runtime-kapacitet',
        },
      },
    },
    metrics: {
      cards: {
        overview: { subtitle: 'Brugsoverblik' },
        modelStats: { subtitle: '{count} sporede modeller' },
      },
    },
    mediaStats: {
      cards: { subtitle: 'Genereringsaktivitet' },
    },
    settings: {
      failover: {
        errorTypes: {
          timeout: 'Tidsudløb',
          unknown: 'Ukendt',
        },
        chips: {
          healthy: 'OK',
          halfOpen: 'Halvåben',
          open: 'Åben',
        },
      },
    },
  },
  'de-DE': {
    system: {
      cards: {
        memoryChart: {
          compactSubtitle: 'Höchstwert der letzten 5 Minuten',
          awaitingSample: 'Warte auf aktuellen Messwert',
          currentValue: 'Aktuell {value}',
          chartSubtitle: 'Footprint des Allokators über die letzten 5 Minuten',
          chartCaption: 'Heap-gestützte Allokation im jüngsten Laufzeitfenster gemessen',
        },
        heapChart: {
          chartSubtitle: 'Verwaltete Heap-Belegung über die letzten 5 Minuten',
          chartCaption:
            'Verfolgtes Heap-Wachstum und Heap-Bindung im jüngsten Laufzeitfenster',
        },
        goroutinesChart: {
          chartSubtitle: 'Scheduler-Konkurrenz über die letzten 5 Minuten',
          chartCaption:
            'Gleichzeitige Laufzeitarbeit und Scheduler-Druck im jüngsten Stichprobenfenster',
        },
        info: {
          subtitle: 'Kontext zu Runtime-Version und Worker-Pool',
          versionFootnote: 'Aktuell bereitgestellte Service-Version',
          workerPool: 'Worker-Pool',
          runningWorkers: '{active} / {total} laufen',
          capacityFootnote:
            'Laufende Worker im Verhältnis zur konfigurierten Runtime-Kapazität',
        },
      },
    },
    metrics: {
      cards: {
        overview: { subtitle: 'Nutzungsübersicht' },
        modelStats: { subtitle: '{count} verfolgte Modelle' },
      },
    },
    mediaStats: {
      cards: { subtitle: 'Generierungsaktivität' },
    },
    settings: {
      failover: {
        errorTypes: {
          timeout: 'Zeitüberschreitung',
          unknown: 'Unbekannt',
        },
        chips: {
          healthy: 'OK',
          halfOpen: 'Halboffen',
          open: 'Offen',
        },
      },
    },
  },
  'el-GR': {
    system: {
      cards: {
        memoryChart: {
          compactSubtitle: 'Υψηλότερη τιμή 5 λεπτών',
          awaitingSample: 'Αναμονή για πρόσφατο δείγμα',
          currentValue: 'Τρέχον {value}',
          chartSubtitle: 'Αποτύπωμα allocator στα τελευταία 5 λεπτά',
          chartCaption:
            'Κατανομή με υποστήριξη heap όπως δειγματοληπτήθηκε στο πιο πρόσφατο παράθυρο εκτέλεσης',
        },
        heapChart: {
          chartSubtitle: 'Διαχειριζόμενη κατανομή heap στα τελευταία 5 λεπτά',
          chartCaption:
            'Παρακολουθούμενη αύξηση και διατήρηση heap στο πιο πρόσφατο παράθυρο εκτέλεσης',
        },
        goroutinesChart: {
          chartSubtitle: 'Ταυτόχρονη εκτέλεση scheduler στα τελευταία 5 λεπτά',
          chartCaption:
            'Ταυτόχρονη εργασία χρόνου εκτέλεσης και πίεση scheduler στο πιο πρόσφατο παράθυρο δειγμάτων',
        },
        info: {
          subtitle: 'Πλαίσιο έκδοσης runtime και ομάδας workers',
          versionFootnote: 'Τρέχουσα αναπτυγμένη έκδοση υπηρεσίας',
          workerPool: 'Ομάδα workers',
          runningWorkers: '{active} / {total} σε εκτέλεση',
          capacityFootnote:
            'Workers σε εκτέλεση σε σχέση με τη ρυθμισμένη χωρητικότητα runtime',
        },
      },
    },
    metrics: {
      cards: {
        overview: { subtitle: 'Επισκόπηση χρήσης' },
        modelStats: { subtitle: '{count} παρακολουθούμενα μοντέλα' },
      },
    },
    mediaStats: {
      cards: { subtitle: 'Δραστηριότητα δημιουργίας' },
    },
    settings: {
      failover: {
        errorTypes: {
          timeout: 'Λήξη χρονικού ορίου',
          unknown: 'Άγνωστο',
        },
        chips: {
          healthy: 'OK',
          halfOpen: 'Μισάνοιχτο',
          open: 'Ανοιχτό',
        },
      },
    },
  },
  'en-GB': {
    system: {
      cards: {
        memoryChart: {
          compactSubtitle: '5 minute high-water mark',
          awaitingSample: 'Awaiting recent sample',
          currentValue: 'Current {value}',
          chartSubtitle: 'Allocator footprint over the latest 5 minutes',
          chartCaption: 'Heap-backed allocation sampled across the most recent runtime window',
        },
        heapChart: {
          chartSubtitle: 'Managed heap allocation across the latest 5 minutes',
          chartCaption: 'Tracked heap growth and retention inside the most recent runtime window',
        },
        goroutinesChart: {
          chartSubtitle: 'Scheduler concurrency over the latest 5 minutes',
          chartCaption:
            'Concurrent runtime work and scheduler pressure across the latest sample window',
        },
        info: {
          subtitle: 'Runtime release and worker pool context',
          versionFootnote: 'Current deployed service version',
          workerPool: 'Worker Pool',
          runningWorkers: '{active} / {total} running',
          capacityFootnote: 'Running workers against configured runtime capacity',
        },
      },
    },
    metrics: {
      cards: {
        overview: { subtitle: 'Usage overview' },
        modelStats: { subtitle: '{count} tracked models' },
      },
    },
    mediaStats: {
      cards: { subtitle: 'Generation activity' },
    },
    settings: {
      failover: {
        errorTypes: {
          timeout: 'Timeout',
          unknown: 'Unknown',
        },
        chips: {
          healthy: 'OK',
          halfOpen: 'Half-open',
          open: 'Open',
        },
      },
    },
  },
  'en-US': {
    system: {
      cards: {
        memoryChart: {
          compactSubtitle: '5 minute high-water mark',
          awaitingSample: 'Awaiting recent sample',
          currentValue: 'Current {value}',
          chartSubtitle: 'Allocator footprint over the latest 5 minutes',
          chartCaption: 'Heap-backed allocation sampled across the most recent runtime window',
        },
        heapChart: {
          chartSubtitle: 'Managed heap allocation across the latest 5 minutes',
          chartCaption: 'Tracked heap growth and retention inside the most recent runtime window',
        },
        goroutinesChart: {
          chartSubtitle: 'Scheduler concurrency over the latest 5 minutes',
          chartCaption:
            'Concurrent runtime work and scheduler pressure across the latest sample window',
        },
        info: {
          subtitle: 'Runtime release and worker pool context',
          versionFootnote: 'Current deployed service version',
          workerPool: 'Worker Pool',
          runningWorkers: '{active} / {total} running',
          capacityFootnote: 'Running workers against configured runtime capacity',
        },
      },
    },
    metrics: {
      cards: {
        overview: { subtitle: 'Usage overview' },
        modelStats: { subtitle: '{count} tracked models' },
      },
    },
    mediaStats: {
      cards: { subtitle: 'Generation activity' },
    },
    settings: {
      failover: {
        errorTypes: {
          timeout: 'Timeout',
          unknown: 'Unknown',
        },
        chips: {
          healthy: 'OK',
          halfOpen: 'Half-open',
          open: 'Open',
        },
      },
    },
  },
  'es-ES': {
    system: {
      cards: {
        memoryChart: {
          compactSubtitle: 'Máximo de los últimos 5 minutos',
          awaitingSample: 'Esperando una muestra reciente',
          currentValue: 'Actual {value}',
          chartSubtitle: 'Huella del asignador durante los últimos 5 minutos',
          chartCaption:
            'Asignación respaldada por heap muestreada en la ventana de ejecución más reciente',
        },
        heapChart: {
          chartSubtitle: 'Asignación administrada del heap durante los últimos 5 minutos',
          chartCaption:
            'Crecimiento y retención del heap seguidos dentro de la ventana de ejecución más reciente',
        },
        goroutinesChart: {
          chartSubtitle: 'Concurrencia del planificador durante los últimos 5 minutos',
          chartCaption:
            'Trabajo concurrente del runtime y presión del planificador en la ventana de muestras más reciente',
        },
        info: {
          subtitle: 'Contexto de la versión del runtime y del grupo de workers',
          versionFootnote: 'Versión desplegada actual del servicio',
          workerPool: 'Grupo de workers',
          runningWorkers: '{active} / {total} en ejecución',
          capacityFootnote:
            'Workers en ejecución frente a la capacidad configurada del runtime',
        },
      },
    },
    metrics: {
      cards: {
        overview: { subtitle: 'Resumen de uso' },
        modelStats: { subtitle: '{count} modelos seguidos' },
      },
    },
    mediaStats: {
      cards: { subtitle: 'Actividad de generación' },
    },
    settings: {
      failover: {
        errorTypes: {
          timeout: 'Tiempo de espera agotado',
          unknown: 'Desconocido',
        },
        chips: {
          healthy: 'OK',
          halfOpen: 'Semiabierto',
          open: 'Abierto',
        },
      },
    },
  },
  'fr-FR': {
    system: {
      cards: {
        memoryChart: {
          compactSubtitle: 'Pic des 5 dernières minutes',
          awaitingSample: "En attente d'un échantillon récent",
          currentValue: 'Actuel {value}',
          chartSubtitle: "Empreinte de l'allocateur sur les 5 dernières minutes",
          chartCaption:
            "Allocation appuyée sur le tas échantillonnée sur la fenêtre d'exécution la plus récente",
        },
        heapChart: {
          chartSubtitle: 'Allocation du tas gérée sur les 5 dernières minutes',
          chartCaption:
            'Croissance et rétention du tas suivies dans la fenêtre d’exécution la plus récente',
        },
        goroutinesChart: {
          chartSubtitle: "Concurrence de l'ordonnanceur sur les 5 dernières minutes",
          chartCaption:
            "Travail d'exécution concurrent et pression de l'ordonnanceur dans la fenêtre d'échantillons la plus récente",
        },
        info: {
          subtitle: "Contexte de la version d'exécution et du pool de workers",
          versionFootnote: 'Version du service actuellement déployée',
          workerPool: 'Pool de workers',
          runningWorkers: '{active} / {total} en cours',
          capacityFootnote:
            "Workers en cours d'exécution par rapport à la capacité configurée du runtime",
        },
      },
    },
    metrics: {
      cards: {
        overview: { subtitle: "Vue d'ensemble de l'utilisation" },
        modelStats: { subtitle: '{count} modèles suivis' },
      },
    },
    mediaStats: {
      cards: { subtitle: 'Activité de génération' },
    },
    settings: {
      failover: {
        errorTypes: {
          timeout: 'Délai dépassé',
          unknown: 'Inconnu',
        },
        chips: {
          healthy: 'OK',
          halfOpen: 'Semi-ouvert',
          open: 'Ouvert',
        },
      },
    },
  },
  'ga-IE': {
    system: {
      cards: {
        memoryChart: {
          compactSubtitle: 'Buaicmharc 5 nóiméad',
          awaitingSample: 'Ag fanacht le sampla le déanaí',
          currentValue: 'Reatha {value}',
          chartSubtitle: 'Lorg an leithdháilteora le 5 nóiméad anuas',
          chartCaption:
            'Leithdháileadh tacaithe ag heap mar a sampláladh é sa fhuinneog ama rite is déanaí',
        },
        heapChart: {
          chartSubtitle: 'Leithdháileadh heap bainistithe le 5 nóiméad anuas',
          chartCaption:
            'Fás agus coinneáil heap rianaithe laistigh den fhuinneog ama rite is déanaí',
        },
        goroutinesChart: {
          chartSubtitle: 'Comhuaineachas an sceidealaitheora le 5 nóiméad anuas',
          chartCaption:
            'Obair ama rite chomhuaineach agus brú ar an sceidealaitheoir sa fhuinneog shamplach is déanaí',
        },
        info: {
          subtitle: 'Comhthéacs leagan ama rite agus linn oibrithe',
          versionFootnote: 'Leagan reatha na seirbhíse atá imscaraithe',
          workerPool: 'Linn oibrithe',
          runningWorkers: '{active} / {total} ag rith',
          capacityFootnote:
            'Oibrithe ag rith i gcoinne acmhainn chumraithe an ama rite',
        },
      },
    },
    metrics: {
      cards: {
        overview: { subtitle: 'Forléargas úsáide' },
        modelStats: { subtitle: '{count} múnla rianaithe' },
      },
    },
    mediaStats: {
      cards: { subtitle: 'Gníomhaíocht ghiniúna' },
    },
    settings: {
      failover: {
        errorTypes: {
          timeout: 'Teorainn ama caite',
          unknown: 'Anaithnid',
        },
        chips: {
          healthy: 'Ceart go leor',
          halfOpen: 'Leathoscailte',
          open: 'Oscailte',
        },
      },
    },
  },
  'hr-HR': {
    system: {
      cards: {
        memoryChart: {
          compactSubtitle: 'Najviša vrijednost u 5 minuta',
          awaitingSample: 'Čeka se nedavni uzorak',
          currentValue: 'Trenutno {value}',
          chartSubtitle: 'Otisak alokatora tijekom zadnjih 5 minuta',
          chartCaption:
            'Alokacija potpomognuta hrpom uzorkovana u najnovijem prozoru izvođenja',
        },
        heapChart: {
          chartSubtitle: 'Upravljana alokacija hrpe tijekom zadnjih 5 minuta',
          chartCaption:
            'Praćen rast i zadržavanje hrpe unutar najnovijeg prozora izvođenja',
        },
        goroutinesChart: {
          chartSubtitle: 'Istovremenost raspoređivača tijekom zadnjih 5 minuta',
          chartCaption:
            'Istovremeni rad vremena izvođenja i pritisak raspoređivača u najnovijem prozoru uzoraka',
        },
        info: {
          subtitle: 'Kontekst verzije runtimea i skupa radnika',
          versionFootnote: 'Trenutačno raspoređena verzija usluge',
          workerPool: 'Skup radnika',
          runningWorkers: '{active} / {total} radi',
          capacityFootnote:
            'Pokrenuti radnici u odnosu na konfigurirani kapacitet runtimea',
        },
      },
    },
    metrics: {
      cards: {
        overview: { subtitle: 'Pregled upotrebe' },
        modelStats: { subtitle: '{count} praćenih modela' },
      },
    },
    mediaStats: {
      cards: { subtitle: 'Aktivnost generiranja' },
    },
    settings: {
      failover: {
        errorTypes: {
          timeout: 'Istek vremena',
          unknown: 'Nepoznato',
        },
        chips: {
          healthy: 'U redu',
          halfOpen: 'Poluotvoren',
          open: 'Otvoren',
        },
      },
    },
  },
  'hu-HU': {
    system: {
      cards: {
        memoryChart: {
          compactSubtitle: '5 perces csúcsterhelés',
          awaitingSample: 'Várakozás friss mintára',
          currentValue: 'Jelenlegi {value}',
          chartSubtitle: 'Az allokátor lábnyoma az elmúlt 5 percben',
          chartCaption: 'Heap-alapú allokáció mintavételezve a legutóbbi futási ablakban',
        },
        heapChart: {
          chartSubtitle: 'Kezelt heap-allokáció az elmúlt 5 percben',
          chartCaption:
            'Követett heapnövekedés és megtartás a legutóbbi futási ablakban',
        },
        goroutinesChart: {
          chartSubtitle: 'Ütemező-párhuzamosság az elmúlt 5 percben',
          chartCaption:
            'Egyidejű futásidejű munka és ütemezőnyomás a legutóbbi mintavételi ablakban',
        },
        info: {
          subtitle: 'Futásidejű kiadás és munkamenet-készlet kontextusa',
          versionFootnote: 'A szolgáltatás jelenleg telepített verziója',
          workerPool: 'Munkavégző készlet',
          runningWorkers: '{active} / {total} fut',
          capacityFootnote:
            'Futó munkavégzők a beállított futásidejű kapacitáshoz viszonyítva',
        },
      },
    },
    metrics: {
      cards: {
        overview: { subtitle: 'Használati áttekintés' },
        modelStats: { subtitle: '{count} követett modell' },
      },
    },
    mediaStats: {
      cards: { subtitle: 'Generálási aktivitás' },
    },
    settings: {
      failover: {
        errorTypes: {
          timeout: 'Időtúllépés',
          unknown: 'Ismeretlen',
        },
        chips: {
          healthy: 'OK',
          halfOpen: 'Félig nyitott',
          open: 'Nyitott',
        },
      },
    },
  },
  'it-IT': {
    system: {
      cards: {
        memoryChart: {
          compactSubtitle: 'Picco degli ultimi 5 minuti',
          awaitingSample: 'In attesa di un campione recente',
          currentValue: 'Attuale {value}',
          chartSubtitle: "Impronta dell'allocatore negli ultimi 5 minuti",
          chartCaption:
            "Allocazione supportata da heap campionata nella finestra di runtime più recente",
        },
        heapChart: {
          chartSubtitle: 'Allocazione heap gestita negli ultimi 5 minuti',
          chartCaption:
            "Crescita e mantenimento dell'heap tracciati nella finestra di runtime più recente",
        },
        goroutinesChart: {
          chartSubtitle: 'Concorrenza dello scheduler negli ultimi 5 minuti',
          chartCaption:
            'Lavoro concorrente del runtime e pressione sullo scheduler nella finestra di campionamento più recente',
        },
        info: {
          subtitle: 'Contesto della release runtime e del pool di worker',
          versionFootnote: 'Versione del servizio attualmente distribuita',
          workerPool: 'Pool di worker',
          runningWorkers: '{active} / {total} in esecuzione',
          capacityFootnote:
            'Worker in esecuzione rispetto alla capacità runtime configurata',
        },
      },
    },
    metrics: {
      cards: {
        overview: { subtitle: "Panoramica dell'utilizzo" },
        modelStats: { subtitle: '{count} modelli monitorati' },
      },
    },
    mediaStats: {
      cards: { subtitle: 'Attività di generazione' },
    },
    settings: {
      failover: {
        errorTypes: {
          timeout: 'Timeout',
          unknown: 'Sconosciuto',
        },
        chips: {
          healthy: 'OK',
          halfOpen: 'Semiaperto',
          open: 'Aperto',
        },
      },
    },
  },
  'ja-JP': {
    system: {
      cards: {
        memoryChart: {
          compactSubtitle: '直近 5 分間の最大値',
          awaitingSample: '最近のサンプルを待機中',
          currentValue: '現在 {value}',
          chartSubtitle: '直近 5 分間のアロケータ使用量',
          chartCaption: '直近のランタイムウィンドウで取得したヒープ由来の割り当て量',
        },
        heapChart: {
          chartSubtitle: '直近 5 分間の管理ヒープ割り当て',
          chartCaption: '直近のランタイムウィンドウ内で追跡したヒープの増加と保持',
        },
        goroutinesChart: {
          chartSubtitle: '直近 5 分間のスケジューラ並行度',
          chartCaption: '最新のサンプルウィンドウにおける並行ランタイム作業とスケジューラ負荷',
        },
        info: {
          subtitle: 'ランタイムリリースとワーカープールの状況',
          versionFootnote: '現在デプロイされているサービスのバージョン',
          workerPool: 'ワーカープール',
          runningWorkers: '{active} / {total} 稼働中',
          capacityFootnote: '設定済みランタイム容量に対する稼働ワーカー数',
        },
      },
    },
    metrics: {
      cards: {
        overview: { subtitle: '使用状況の概要' },
        modelStats: { subtitle: '追跡中のモデル {count} 件' },
      },
    },
    mediaStats: {
      cards: { subtitle: '生成アクティビティ' },
    },
    settings: {
      failover: {
        errorTypes: {
          timeout: 'タイムアウト',
          unknown: '不明',
        },
        chips: {
          healthy: '正常',
          halfOpen: '半開',
          open: 'オープン',
        },
      },
    },
  },
  'ko-KR': {
    system: {
      cards: {
        memoryChart: {
          compactSubtitle: '최근 5분 최고치',
          awaitingSample: '최근 샘플을 기다리는 중',
          currentValue: '현재 {value}',
          chartSubtitle: '최근 5분간의 할당기 사용량',
          chartCaption: '가장 최근 런타임 구간에서 샘플링한 힙 기반 할당량',
        },
        heapChart: {
          chartSubtitle: '최근 5분간의 관리형 힙 할당',
          chartCaption: '가장 최근 런타임 구간에서 추적한 힙 증가와 유지 상태',
        },
        goroutinesChart: {
          chartSubtitle: '최근 5분간의 스케줄러 동시성',
          chartCaption: '가장 최근 샘플 구간의 동시 런타임 작업과 스케줄러 부하',
        },
        info: {
          subtitle: '런타임 릴리스와 워커 풀 컨텍스트',
          versionFootnote: '현재 배포된 서비스 버전',
          workerPool: '워커 풀',
          runningWorkers: '{active} / {total} 실행 중',
          capacityFootnote: '구성된 런타임 용량 대비 실행 중인 워커 수',
        },
      },
    },
    metrics: {
      cards: {
        overview: { subtitle: '사용 개요' },
        modelStats: { subtitle: '추적 중인 모델 {count}개' },
      },
    },
    mediaStats: {
      cards: { subtitle: '생성 활동' },
    },
    settings: {
      failover: {
        errorTypes: {
          timeout: '시간 초과',
          unknown: '알 수 없음',
        },
        chips: {
          healthy: '정상',
          halfOpen: '반열림',
          open: '열림',
        },
      },
    },
  },
  'ml-IN': {
    system: {
      cards: {
        memoryChart: {
          compactSubtitle: 'കഴിഞ്ഞ 5 മിനിറ്റിലെ ഉയർന്ന നില',
          awaitingSample: 'സമീപകാല സാമ്പിൾ കാത്തിരിക്കുന്നു',
          currentValue: 'നിലവിൽ {value}',
          chartSubtitle: 'കഴിഞ്ഞ 5 മിനിറ്റിലത്തെ അലോക്കേറ്റർ ഉപയോഗചിഹ്നം',
          chartCaption:
            'ഏറ്റവും പുതിയ റൺടൈം വിൻഡോയിൽ സാമ്പിൾ എടുത്ത heap അടിസ്ഥാനത്തിലുള്ള അലോക്കേഷൻ',
        },
        heapChart: {
          chartSubtitle: 'കഴിഞ്ഞ 5 മിനിറ്റിലത്തെ മാനേജുചെയ്ത heap അലോക്കേഷൻ',
          chartCaption:
            'ഏറ്റവും പുതിയ റൺടൈം വിൻഡോയിലെ heap വളർച്ചയും നിലനിൽപ്പും പിന്തുടരുന്നു',
        },
        goroutinesChart: {
          chartSubtitle: 'കഴിഞ്ഞ 5 മിനിറ്റിലത്തെ ഷെഡ്യൂളർ സമകാലികത',
          chartCaption:
            'ഏറ്റവും പുതിയ സാമ്പിൾ വിൻഡോയിലെ സമകാലിക റൺടൈം പ്രവൃത്തിയും ഷെഡ്യൂളർ സമ്മർദവും',
        },
        info: {
          subtitle: 'റൺടൈം റിലീസിന്റെയും worker pool ന്റെയും പശ്ചാത്തലം',
          versionFootnote: 'നിലവിൽ വിന്യസിച്ചിരിക്കുന്ന സേവന പതിപ്പ്',
          workerPool: 'വർക്കർ പൂൾ',
          runningWorkers: '{active} / {total} പ്രവർത്തിക്കുന്നു',
          capacityFootnote: 'കോൺഫിഗർ ചെയ്ത റൺടൈം ശേഷിയോട് താരതമ്യത്തിലുള്ള പ്രവർത്തിക്കുന്ന വർക്കർമാർ',
        },
      },
    },
    metrics: {
      cards: {
        overview: { subtitle: 'ഉപയോഗ അവലോകനം' },
        modelStats: { subtitle: '{count} പിന്തുടരുന്ന മോഡലുകൾ' },
      },
    },
    mediaStats: {
      cards: { subtitle: 'ജനറേഷൻ പ്രവർത്തനം' },
    },
    settings: {
      failover: {
        errorTypes: {
          timeout: 'സമയം കഴിഞ്ഞു',
          unknown: 'അജ്ഞാതം',
        },
        chips: {
          healthy: 'ശരി',
          halfOpen: 'പാതി തുറന്നത്',
          open: 'തുറന്നത്',
        },
      },
    },
  },
  'nb-NO': {
    system: {
      cards: {
        memoryChart: {
          compactSubtitle: 'Toppnivå siste 5 minutter',
          awaitingSample: 'Venter på nylig prøve',
          currentValue: 'Nåværende {value}',
          chartSubtitle: 'Allokatoravtrykk de siste 5 minuttene',
          chartCaption: 'Heap-basert allokering prøvet i det nyeste kjøretidsvinduet',
        },
        heapChart: {
          chartSubtitle: 'Administrert heap-allokering de siste 5 minuttene',
          chartCaption: 'Sporing av heap-vekst og -retensjon i det nyeste kjøretidsvinduet',
        },
        goroutinesChart: {
          chartSubtitle: 'Planlegger-samtidighet de siste 5 minuttene',
          chartCaption:
            'Samtidig kjøretidsarbeid og planleggerpress i det nyeste prøvevinduet',
        },
        info: {
          subtitle: 'Kontekst for runtime-versjon og worker-pool',
          versionFootnote: 'Nåværende utrullede tjenesteversjon',
          workerPool: 'Worker-pool',
          runningWorkers: '{active} / {total} kjører',
          capacityFootnote:
            'Kjørende workers opp mot konfigurert runtime-kapasitet',
        },
      },
    },
    metrics: {
      cards: {
        overview: { subtitle: 'Bruksoversikt' },
        modelStats: { subtitle: '{count} sporede modeller' },
      },
    },
    mediaStats: {
      cards: { subtitle: 'Genereringsaktivitet' },
    },
    settings: {
      failover: {
        errorTypes: {
          timeout: 'Tidsavbrudd',
          unknown: 'Ukjent',
        },
        chips: {
          healthy: 'OK',
          halfOpen: 'Halvåpen',
          open: 'Åpen',
        },
      },
    },
  },
  'nl-NL': {
    system: {
      cards: {
        memoryChart: {
          compactSubtitle: 'Hoogste stand in 5 minuten',
          awaitingSample: 'Wachten op recente meting',
          currentValue: 'Huidig {value}',
          chartSubtitle: 'Footprint van de allocator over de laatste 5 minuten',
          chartCaption:
            'Heap-gedragen allocatie gemeten binnen het meest recente runtimevenster',
        },
        heapChart: {
          chartSubtitle: 'Beheerde heapallocatie over de laatste 5 minuten',
          chartCaption:
            'Bijgehouden groei en retentie van de heap binnen het meest recente runtimevenster',
        },
        goroutinesChart: {
          chartSubtitle: 'Scheduler-concurrentie over de laatste 5 minuten',
          chartCaption:
            'Gelijktijdig runtimewerk en schedulerdruk in het meest recente meetvenster',
        },
        info: {
          subtitle: 'Context voor runtimeversie en workerpool',
          versionFootnote: 'Huidige uitgerolde serviceversie',
          workerPool: 'Workerpool',
          runningWorkers: '{active} / {total} actief',
          capacityFootnote:
            'Actieve workers ten opzichte van de geconfigureerde runtimecapaciteit',
        },
      },
    },
    metrics: {
      cards: {
        overview: { subtitle: 'Gebruiksoverzicht' },
        modelStats: { subtitle: '{count} bijgehouden modellen' },
      },
    },
    mediaStats: {
      cards: { subtitle: 'Generatieactiviteit' },
    },
    settings: {
      failover: {
        errorTypes: {
          timeout: 'Time-out',
          unknown: 'Onbekend',
        },
        chips: {
          healthy: 'OK',
          halfOpen: 'Halfopen',
          open: 'Open',
        },
      },
    },
  },
  'pl-PL': {
    system: {
      cards: {
        memoryChart: {
          compactSubtitle: 'Szczyt z ostatnich 5 minut',
          awaitingSample: 'Oczekiwanie na ostatnią próbkę',
          currentValue: 'Bieżące {value}',
          chartSubtitle: 'Ślad alokatora z ostatnich 5 minut',
          chartCaption:
            'Alokacja wsparta stertą próbkowana w najnowszym oknie działania',
        },
        heapChart: {
          chartSubtitle: 'Zarządzana alokacja sterty z ostatnich 5 minut',
          chartCaption:
            'Śledzony wzrost i retencja sterty w najnowszym oknie działania',
        },
        goroutinesChart: {
          chartSubtitle: 'Współbieżność planisty z ostatnich 5 minut',
          chartCaption:
            'Współbieżna praca środowiska uruchomieniowego i obciążenie planisty w najnowszym oknie próbek',
        },
        info: {
          subtitle: 'Kontekst wersji runtime i puli workerów',
          versionFootnote: 'Aktualnie wdrożona wersja usługi',
          workerPool: 'Pula workerów',
          runningWorkers: '{active} / {total} działa',
          capacityFootnote:
            'Działające workery względem skonfigurowanej pojemności runtime',
        },
      },
    },
    metrics: {
      cards: {
        overview: { subtitle: 'Przegląd użycia' },
        modelStats: { subtitle: '{count} śledzonych modeli' },
      },
    },
    mediaStats: {
      cards: { subtitle: 'Aktywność generowania' },
    },
    settings: {
      failover: {
        errorTypes: {
          timeout: 'Przekroczono limit czasu',
          unknown: 'Nieznane',
        },
        chips: {
          healthy: 'OK',
          halfOpen: 'Półotwarty',
          open: 'Otwarty',
        },
      },
    },
  },
  'pt-BR': {
    system: {
      cards: {
        memoryChart: {
          compactSubtitle: 'Pico dos últimos 5 minutos',
          awaitingSample: 'Aguardando amostra recente',
          currentValue: 'Atual {value}',
          chartSubtitle: 'Pegada do alocador nos últimos 5 minutos',
          chartCaption:
            'Alocação baseada em heap amostrada na janela de runtime mais recente',
        },
        heapChart: {
          chartSubtitle: 'Alocação gerenciada de heap nos últimos 5 minutos',
          chartCaption:
            'Crescimento e retenção de heap acompanhados na janela de runtime mais recente',
        },
        goroutinesChart: {
          chartSubtitle: 'Concorrência do escalonador nos últimos 5 minutos',
          chartCaption:
            'Trabalho concorrente de runtime e pressão do escalonador na janela de amostragem mais recente',
        },
        info: {
          subtitle: 'Contexto da release do runtime e do pool de workers',
          versionFootnote: 'Versão atualmente implantada do serviço',
          workerPool: 'Pool de workers',
          runningWorkers: '{active} / {total} em execução',
          capacityFootnote:
            'Workers em execução em relação à capacidade configurada do runtime',
        },
      },
    },
    metrics: {
      cards: {
        overview: { subtitle: 'Visão geral de uso' },
        modelStats: { subtitle: '{count} modelos monitorados' },
      },
    },
    mediaStats: {
      cards: { subtitle: 'Atividade de geração' },
    },
    settings: {
      failover: {
        errorTypes: {
          timeout: 'Tempo esgotado',
          unknown: 'Desconhecido',
        },
        chips: {
          healthy: 'OK',
          halfOpen: 'Semiaberto',
          open: 'Aberto',
        },
      },
    },
  },
  'pt-PT': {
    system: {
      cards: {
        memoryChart: {
          compactSubtitle: 'Pico dos últimos 5 minutos',
          awaitingSample: 'A aguardar amostra recente',
          currentValue: 'Atual {value}',
          chartSubtitle: 'Pegada do alocador nos últimos 5 minutos',
          chartCaption:
            'Alocação baseada em heap amostrada na janela de runtime mais recente',
        },
        heapChart: {
          chartSubtitle: 'Alocação gerida da heap nos últimos 5 minutos',
          chartCaption:
            'Crescimento e retenção da heap acompanhados na janela de runtime mais recente',
        },
        goroutinesChart: {
          chartSubtitle: 'Concorrência do escalonador nos últimos 5 minutos',
          chartCaption:
            'Trabalho concorrente de runtime e pressão do escalonador na janela de amostragem mais recente',
        },
        info: {
          subtitle: 'Contexto da release do runtime e do pool de workers',
          versionFootnote: 'Versão atualmente implantada do serviço',
          workerPool: 'Pool de workers',
          runningWorkers: '{active} / {total} em execução',
          capacityFootnote:
            'Workers em execução face à capacidade configurada do runtime',
        },
      },
    },
    metrics: {
      cards: {
        overview: { subtitle: 'Visão geral de utilização' },
        modelStats: { subtitle: '{count} modelos monitorizados' },
      },
    },
    mediaStats: {
      cards: { subtitle: 'Atividade de geração' },
    },
    settings: {
      failover: {
        errorTypes: {
          timeout: 'Tempo esgotado',
          unknown: 'Desconhecido',
        },
        chips: {
          healthy: 'OK',
          halfOpen: 'Semiaberto',
          open: 'Aberto',
        },
      },
    },
  },
  'ro-RO': {
    system: {
      cards: {
        memoryChart: {
          compactSubtitle: 'Vârf pe ultimele 5 minute',
          awaitingSample: 'Se așteaptă un eșantion recent',
          currentValue: 'Curent {value}',
          chartSubtitle: 'Amprenta allocatorului în ultimele 5 minute',
          chartCaption:
            'Alocare bazată pe heap eșantionată în cea mai recentă fereastră de runtime',
        },
        heapChart: {
          chartSubtitle: 'Alocare heap gestionată în ultimele 5 minute',
          chartCaption:
            'Creștere și retenție heap urmărite în cea mai recentă fereastră de runtime',
        },
        goroutinesChart: {
          chartSubtitle: 'Concurența planificatorului în ultimele 5 minute',
          chartCaption:
            'Lucru concurent de runtime și presiune asupra planificatorului în cea mai recentă fereastră de eșantionare',
        },
        info: {
          subtitle: 'Contextul versiunii runtime și al poolului de workeri',
          versionFootnote: 'Versiunea serviciului implementată în prezent',
          workerPool: 'Pool de workeri',
          runningWorkers: '{active} / {total} rulează',
          capacityFootnote:
            'Workeri activi raportat la capacitatea runtime configurată',
        },
      },
    },
    metrics: {
      cards: {
        overview: { subtitle: 'Prezentare generală a utilizării' },
        modelStats: { subtitle: '{count} modele urmărite' },
      },
    },
    mediaStats: {
      cards: { subtitle: 'Activitate de generare' },
    },
    settings: {
      failover: {
        errorTypes: {
          timeout: 'Expirare timp',
          unknown: 'Necunoscut',
        },
        chips: {
          healthy: 'OK',
          halfOpen: 'Semi-deschis',
          open: 'Deschis',
        },
      },
    },
  },
  'ru-RU': {
    system: {
      cards: {
        memoryChart: {
          compactSubtitle: 'Пик за последние 5 минут',
          awaitingSample: 'Ожидание недавнего образца',
          currentValue: 'Текущее {value}',
          chartSubtitle: 'След аллокатора за последние 5 минут',
          chartCaption:
            'Распределение на основе кучи, измеренное в последнем окне времени выполнения',
        },
        heapChart: {
          chartSubtitle: 'Управляемое выделение кучи за последние 5 минут',
          chartCaption:
            'Отслеживаемый рост и удержание кучи в последнем окне времени выполнения',
        },
        goroutinesChart: {
          chartSubtitle: 'Параллелизм планировщика за последние 5 минут',
          chartCaption:
            'Параллельная работа времени выполнения и нагрузка на планировщик в последнем окне выборки',
        },
        info: {
          subtitle: 'Контекст версии runtime и пула воркеров',
          versionFootnote: 'Текущая развёрнутая версия сервиса',
          workerPool: 'Пул воркеров',
          runningWorkers: '{active} / {total} выполняется',
          capacityFootnote:
            'Работающие воркеры относительно настроенной ёмкости runtime',
        },
      },
    },
    metrics: {
      cards: {
        overview: { subtitle: 'Обзор использования' },
        modelStats: { subtitle: 'Отслеживаемых моделей: {count}' },
      },
    },
    mediaStats: {
      cards: { subtitle: 'Активность генерации' },
    },
    settings: {
      failover: {
        errorTypes: {
          timeout: 'Тайм-аут',
          unknown: 'Неизвестно',
        },
        chips: {
          healthy: 'OK',
          halfOpen: 'Полуоткрыт',
          open: 'Открыт',
        },
      },
    },
  },
  'sk-SK': {
    system: {
      cards: {
        memoryChart: {
          compactSubtitle: 'Vrchol za posledných 5 minút',
          awaitingSample: 'Čaká sa na nedávnu vzorku',
          currentValue: 'Aktuálne {value}',
          chartSubtitle: 'Stopa alokátora za posledných 5 minút',
          chartCaption:
            'Alokácia podporená haldou vzorkovaná v najnovšom okne behu',
        },
        heapChart: {
          chartSubtitle: 'Spravovaná alokácia haldy za posledných 5 minút',
          chartCaption:
            'Sledovaný rast a retencia haldy v najnovšom okne behu',
        },
        goroutinesChart: {
          chartSubtitle: 'Súbežnosť plánovača za posledných 5 minút',
          chartCaption:
            'Súbežná runtime práca a tlak plánovača v najnovšom okne vzoriek',
        },
        info: {
          subtitle: 'Kontext verzie runtime a fondu workerov',
          versionFootnote: 'Aktuálne nasadená verzia služby',
          workerPool: 'Fond workerov',
          runningWorkers: '{active} / {total} beží',
          capacityFootnote:
            'Bežiaci workeri voči konfigurovanej kapacite runtime',
        },
      },
    },
    metrics: {
      cards: {
        overview: { subtitle: 'Prehľad používania' },
        modelStats: { subtitle: '{count} sledovaných modelov' },
      },
    },
    mediaStats: {
      cards: { subtitle: 'Aktivita generovania' },
    },
    settings: {
      failover: {
        errorTypes: {
          timeout: 'Časový limit vypršal',
          unknown: 'Neznáme',
        },
        chips: {
          healthy: 'OK',
          halfOpen: 'Polootvorený',
          open: 'Otvorený',
        },
      },
    },
  },
  'sv-SE': {
    system: {
      cards: {
        memoryChart: {
          compactSubtitle: 'Toppnivå under de senaste 5 minuterna',
          awaitingSample: 'Väntar på nytt prov',
          currentValue: 'Nuvarande {value}',
          chartSubtitle: 'Allokerarens fotavtryck under de senaste 5 minuterna',
          chartCaption:
            'Heap-baserad allokering samplad över det senaste körtidsfönstret',
        },
        heapChart: {
          chartSubtitle: 'Hanterad heap-allokering under de senaste 5 minuterna',
          chartCaption:
            'Spårad heap-tillväxt och kvarhållning i det senaste körtidsfönstret',
        },
        goroutinesChart: {
          chartSubtitle: 'Schemaläggarkonkurrens under de senaste 5 minuterna',
          chartCaption:
            'Samtidigt körtidsarbete och schemaläggartryck i det senaste provfönstret',
        },
        info: {
          subtitle: 'Kontext för runtime-version och workerpool',
          versionFootnote: 'Nuvarande distribuerad tjänsteversion',
          workerPool: 'Workerpool',
          runningWorkers: '{active} / {total} körs',
          capacityFootnote:
            'Körande workers i förhållande till konfigurerad runtime-kapacitet',
        },
      },
    },
    metrics: {
      cards: {
        overview: { subtitle: 'Användningsöversikt' },
        modelStats: { subtitle: '{count} spårade modeller' },
      },
    },
    mediaStats: {
      cards: { subtitle: 'Genereringsaktivitet' },
    },
    settings: {
      failover: {
        errorTypes: {
          timeout: 'Tidsgränsen överskreds',
          unknown: 'Okänd',
        },
        chips: {
          healthy: 'OK',
          halfOpen: 'Halvöppen',
          open: 'Öppen',
        },
      },
    },
  },
  'zh-CN': {
    system: {
      cards: {
        memoryChart: {
          compactSubtitle: '最近 5 分钟峰值',
          awaitingSample: '等待最近采样',
          currentValue: '当前 {value}',
          chartSubtitle: '最近 5 分钟内的分配器占用',
          chartCaption: '在最近运行窗口中采样的堆支撑分配情况',
        },
        heapChart: {
          chartSubtitle: '最近 5 分钟内的托管堆分配',
          chartCaption: '最近运行窗口内追踪的堆增长与保留情况',
        },
        goroutinesChart: {
          chartSubtitle: '最近 5 分钟内的调度并发度',
          chartCaption: '最近采样窗口内的并发运行时工作与调度器压力',
        },
        info: {
          subtitle: '运行时版本与工作池上下文',
          versionFootnote: '当前已部署的服务版本',
          workerPool: '工作池',
          runningWorkers: '{active} / {total} 运行中',
          capacityFootnote: '运行中的工作进程与已配置运行时容量的对比',
        },
      },
    },
    metrics: {
      cards: {
        overview: { subtitle: '使用概览' },
        modelStats: { subtitle: '已跟踪模型 {count} 个' },
      },
    },
    mediaStats: {
      cards: { subtitle: '生成活动' },
    },
    settings: {
      failover: {
        errorTypes: {
          timeout: '超时',
          unknown: '未知',
        },
        chips: {
          healthy: '正常',
          halfOpen: '半开',
          open: '打开',
        },
      },
    },
  },
  'zh-TW': {
    system: {
      cards: {
        memoryChart: {
          compactSubtitle: '最近 5 分鐘峰值',
          awaitingSample: '等待最近樣本',
          currentValue: '目前 {value}',
          chartSubtitle: '最近 5 分鐘內的配置器占用',
          chartCaption: '在最近執行視窗中取樣的堆支援配置情況',
        },
        heapChart: {
          chartSubtitle: '最近 5 分鐘內的受管堆配置',
          chartCaption: '最近執行視窗內追蹤的堆成長與保留情況',
        },
        goroutinesChart: {
          chartSubtitle: '最近 5 分鐘內的排程並行度',
          chartCaption: '最近樣本視窗內的並行執行階段工作與排程器壓力',
        },
        info: {
          subtitle: '執行階段版本與工作池脈絡',
          versionFootnote: '目前已部署的服務版本',
          workerPool: '工作池',
          runningWorkers: '{active} / {total} 執行中',
          capacityFootnote: '執行中的工作程序相對於已設定執行階段容量',
        },
      },
    },
    metrics: {
      cards: {
        overview: { subtitle: '使用概覽' },
        modelStats: { subtitle: '已追蹤模型 {count} 個' },
      },
    },
    mediaStats: {
      cards: { subtitle: '生成活動' },
    },
    settings: {
      failover: {
        errorTypes: {
          timeout: '逾時',
          unknown: '未知',
        },
        chips: {
          healthy: '正常',
          halfOpen: '半開',
          open: '開啟',
        },
      },
    },
  },
} satisfies Partial<Record<LocaleKey, Record<string, unknown>>>

export default dashboardCardCopyOverrides
