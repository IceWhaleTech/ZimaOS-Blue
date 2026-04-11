import type { LocaleKey } from './locale-catalog'

type ProcessTraceTerms = {
  message: string
  provider: string
  model: string
  attachments: string
  file: string
  attempt: string
  delay: string
  continuePreviousReply: string
  resumePreviousRequest: string
  requestReady: string
  requestSent: string
  waitingForResponse: string
  recoveringResponse: string
  requestDispatched: string
  waitingForFirstVisibleOutput: string
  recoveryStage1: string
  recoveryStage2: string
}

type VoiceProcessTraceTerms = {
  format: string
  size: string
  duration: string
  upload: string
  transcript: string
  mode: string
  conversation: string
  interruptCurrentReply: string
  sendNewRequest: string
  voiceInterruptDispatched: string
  voiceRequestDispatched: string
  voiceWaitingForResponse: string
  bargeIn: string
  transcriptionFailed: string
}

type ProcessTraceEventTerms = {
  providerResolved: string
  processing: string
}

type ProcessTraceDetailTerms = {
  providerResolved: string
  providerResolvedModelSwitch: string
  providerResolvedFailoverSwitch: string
  providerFailoverToolFollowUp: string
  providerFailoverContinuationFollowUp: string
}

function makeTerms(terms: ProcessTraceTerms): ProcessTraceTerms {
  return terms
}

function makeVoiceTerms(terms: VoiceProcessTraceTerms): VoiceProcessTraceTerms {
  return terms
}

function makeEventTerms(terms: ProcessTraceEventTerms): ProcessTraceEventTerms {
  return terms
}

function makeDetailTerms(terms: ProcessTraceDetailTerms): ProcessTraceDetailTerms {
  return terms
}

const processTraceTerms: Record<LocaleKey, ProcessTraceTerms> = {
  'ca-ES': makeTerms({
    message: 'Missatge',
    provider: 'Proveïdor',
    model: 'Model',
    attachments: 'Adjunts',
    file: 'fitxer',
    attempt: 'Intent',
    delay: 'Retard',
    continuePreviousReply: 'Continua la resposta anterior',
    resumePreviousRequest: 'Reprèn la sol·licitud anterior',
    requestReady: 'Sol·licitud preparada',
    requestSent: 'Sol·licitud enviada',
    waitingForResponse: 'En espera de resposta',
    recoveringResponse: 'Recuperant la resposta',
    requestDispatched:
      "S'està esperant que el servidor accepti la sol·licitud i iniciï la resposta.",
    waitingForFirstVisibleOutput:
      "La sol·licitud ha estat acceptada. S'està esperant la primera sortida visible.",
    recoveryStage1: 'Recuperació silenciosa',
    recoveryStage2: 'Càrrega de recuperació reduïda',
  }),
  'cs-CZ': makeTerms({
    message: 'Zpráva',
    provider: 'Poskytovatel',
    model: 'Model',
    attachments: 'Přílohy',
    file: 'soubor',
    attempt: 'Pokus',
    delay: 'Zpoždění',
    continuePreviousReply: 'Pokračovat v předchozí odpovědi',
    resumePreviousRequest: 'Obnovit předchozí požadavek',
    requestReady: 'Požadavek připraven',
    requestSent: 'Požadavek odeslán',
    waitingForResponse: 'Čekání na odpověď',
    recoveringResponse: 'Obnovování odpovědi',
    requestDispatched: 'Čeká se, až server požadavek přijme a zahájí odpověď.',
    waitingForFirstVisibleOutput: 'Požadavek byl přijat. Čeká se na první viditelný výstup.',
    recoveryStage1: 'Tiché obnovení',
    recoveryStage2: 'Zmenšená data obnovy',
  }),
  'da-DK': makeTerms({
    message: 'Besked',
    provider: 'Udbyder',
    model: 'Model',
    attachments: 'Vedhæftninger',
    file: 'fil',
    attempt: 'Forsøg',
    delay: 'Forsinkelse',
    continuePreviousReply: 'Fortsæt forrige svar',
    resumePreviousRequest: 'Genoptag forrige anmodning',
    requestReady: 'Anmodning klar',
    requestSent: 'Anmodning sendt',
    waitingForResponse: 'Venter på svar',
    recoveringResponse: 'Gendanner svar',
    requestDispatched: 'Venter på, at serveren accepterer anmodningen og starter svaret.',
    waitingForFirstVisibleOutput:
      'Anmodningen blev accepteret. Venter på det første synlige output.',
    recoveryStage1: 'Stille gendannelse',
    recoveryStage2: 'Reduceret gendannelsespayload',
  }),
  'de-DE': makeTerms({
    message: 'Nachricht',
    provider: 'Anbieter',
    model: 'Modell',
    attachments: 'Anhänge',
    file: 'Datei',
    attempt: 'Versuch',
    delay: 'Verzögerung',
    continuePreviousReply: 'Vorherige Antwort fortsetzen',
    resumePreviousRequest: 'Vorherige Anfrage fortsetzen',
    requestReady: 'Anfrage bereit',
    requestSent: 'Anfrage gesendet',
    waitingForResponse: 'Warten auf Antwort',
    recoveringResponse: 'Antwort wird wiederhergestellt',
    requestDispatched: 'Warten, bis der Server die Anfrage annimmt und die Antwort startet.',
    waitingForFirstVisibleOutput:
      'Die Anfrage wurde angenommen. Warten auf die erste sichtbare Ausgabe.',
    recoveryStage1: 'Stille Wiederherstellung',
    recoveryStage2: 'Reduzierte Wiederherstellungsdaten',
  }),
  'el-GR': makeTerms({
    message: 'Μήνυμα',
    provider: 'Πάροχος',
    model: 'Μοντέλο',
    attachments: 'Συνημμένα',
    file: 'αρχείο',
    attempt: 'Προσπάθεια',
    delay: 'Καθυστέρηση',
    continuePreviousReply: 'Συνέχισε την προηγούμενη απάντηση',
    resumePreviousRequest: 'Συνέχισε το προηγούμενο αίτημα',
    requestReady: 'Το αίτημα είναι έτοιμο',
    requestSent: 'Το αίτημα στάλθηκε',
    waitingForResponse: 'Αναμονή απάντησης',
    recoveringResponse: 'Ανάκτηση απάντησης',
    requestDispatched:
      'Αναμονή να αποδεχτεί ο διακομιστής το αίτημα και να ξεκινήσει την απόκριση.',
    waitingForFirstVisibleOutput:
      'Το αίτημα έγινε αποδεκτό. Αναμονή για το πρώτο ορατό αποτέλεσμα.',
    recoveryStage1: 'Αθόρυβη ανάκτηση',
    recoveryStage2: 'Μειωμένο φορτίο ανάκτησης',
  }),
  'en-GB': makeTerms({
    message: 'Message',
    provider: 'Provider',
    model: 'Model',
    attachments: 'Attachments',
    file: 'file',
    attempt: 'Attempt',
    delay: 'Delay',
    continuePreviousReply: 'Continue the previous reply',
    resumePreviousRequest: 'Resume the previous request',
    requestReady: 'Request ready',
    requestSent: 'Request sent',
    waitingForResponse: 'Waiting for response',
    recoveringResponse: 'Recovering response',
    requestDispatched: 'Waiting for the server to accept and start the response.',
    waitingForFirstVisibleOutput: 'The request was accepted. Waiting for the first visible output.',
    recoveryStage1: 'Silent recovery',
    recoveryStage2: 'Reduced recovery payload',
  }),
  'en-US': makeTerms({
    message: 'Message',
    provider: 'Provider',
    model: 'Model',
    attachments: 'Attachments',
    file: 'file',
    attempt: 'Attempt',
    delay: 'Delay',
    continuePreviousReply: 'Continue the previous reply',
    resumePreviousRequest: 'Resume the previous request',
    requestReady: 'Request ready',
    requestSent: 'Request sent',
    waitingForResponse: 'Waiting for response',
    recoveringResponse: 'Recovering response',
    requestDispatched: 'Waiting for the server to accept and start the response.',
    waitingForFirstVisibleOutput: 'The request was accepted. Waiting for the first visible output.',
    recoveryStage1: 'Silent recovery',
    recoveryStage2: 'Reduced recovery payload',
  }),
  'es-ES': makeTerms({
    message: 'Mensaje',
    provider: 'Proveedor',
    model: 'Modelo',
    attachments: 'Adjuntos',
    file: 'archivo',
    attempt: 'Intento',
    delay: 'Retraso',
    continuePreviousReply: 'Continuar la respuesta anterior',
    resumePreviousRequest: 'Reanudar la solicitud anterior',
    requestReady: 'Solicitud lista',
    requestSent: 'Solicitud enviada',
    waitingForResponse: 'Esperando respuesta',
    recoveringResponse: 'Recuperando respuesta',
    requestDispatched: 'Esperando a que el servidor acepte la solicitud e inicie la respuesta.',
    waitingForFirstVisibleOutput: 'La solicitud fue aceptada. Esperando la primera salida visible.',
    recoveryStage1: 'Recuperación silenciosa',
    recoveryStage2: 'Carga de recuperación reducida',
  }),
  'fr-FR': makeTerms({
    message: 'Message',
    provider: 'Fournisseur',
    model: 'Modèle',
    attachments: 'Pièces jointes',
    file: 'fichier',
    attempt: 'Tentative',
    delay: 'Délai',
    continuePreviousReply: 'Continuer la réponse précédente',
    resumePreviousRequest: 'Reprendre la requête précédente',
    requestReady: 'Requête prête',
    requestSent: 'Requête envoyée',
    waitingForResponse: 'En attente de réponse',
    recoveringResponse: 'Récupération de la réponse',
    requestDispatched: 'En attente que le serveur accepte la requête et commence la réponse.',
    waitingForFirstVisibleOutput:
      'La requête a été acceptée. En attente du premier résultat visible.',
    recoveryStage1: 'Récupération silencieuse',
    recoveryStage2: 'Charge de récupération réduite',
  }),
  'ga-IE': makeTerms({
    message: 'Teachtaireacht',
    provider: 'Soláthraí',
    model: 'Samhail',
    attachments: 'Ceangaltáin',
    file: 'comhad',
    attempt: 'Iarracht',
    delay: 'Moill',
    continuePreviousReply: 'Lean leis an bhfreagra roimhe seo',
    resumePreviousRequest: 'Lean ar aghaidh leis an iarratas roimhe seo',
    requestReady: 'Iarratas réidh',
    requestSent: 'Iarratas seolta',
    waitingForResponse: 'Ag fanacht le freagra',
    recoveringResponse: 'Freagra á aisghabháil',
    requestDispatched:
      'Ag fanacht go nglacfaidh an freastalaí leis an iarratas agus go dtosóidh sé an freagra.',
    waitingForFirstVisibleOutput:
      'Glacadh leis an iarratas. Ag fanacht leis an gcéad aschur infheicthe.',
    recoveryStage1: 'Aisghabháil chiúin',
    recoveryStage2: 'Ualach sonraí aisghabhála laghdaithe',
  }),
  'hr-HR': makeTerms({
    message: 'Poruka',
    provider: 'Davatelj',
    model: 'Model',
    attachments: 'Privitci',
    file: 'datoteka',
    attempt: 'Pokušaj',
    delay: 'Odgoda',
    continuePreviousReply: 'Nastavi prethodni odgovor',
    resumePreviousRequest: 'Nastavi prethodni zahtjev',
    requestReady: 'Zahtjev je spreman',
    requestSent: 'Zahtjev je poslan',
    waitingForResponse: 'Čeka se odgovor',
    recoveringResponse: 'Oporavlja se odgovor',
    requestDispatched: 'Čeka se da poslužitelj prihvati zahtjev i započne odgovor.',
    waitingForFirstVisibleOutput: 'Zahtjev je prihvaćen. Čeka se prvi vidljivi izlaz.',
    recoveryStage1: 'Tiha obnova',
    recoveryStage2: 'Smanjeni paket oporavka',
  }),
  'hu-HU': makeTerms({
    message: 'Üzenet',
    provider: 'Szolgáltató',
    model: 'Modell',
    attachments: 'Mellékletek',
    file: 'fájl',
    attempt: 'Próbálkozás',
    delay: 'Késleltetés',
    continuePreviousReply: 'Előző válasz folytatása',
    resumePreviousRequest: 'Előző kérés folytatása',
    requestReady: 'Kérés kész',
    requestSent: 'Kérés elküldve',
    waitingForResponse: 'Válaszra várakozás',
    recoveringResponse: 'Válasz helyreállítása',
    requestDispatched:
      'Várakozás arra, hogy a kiszolgáló elfogadja a kérést és elindítsa a választ.',
    waitingForFirstVisibleOutput: 'A kérés elfogadva. Várakozás az első látható kimenetre.',
    recoveryStage1: 'Csendes helyreállítás',
    recoveryStage2: 'Csökkentett helyreállítási adatcsomag',
  }),
  'it-IT': makeTerms({
    message: 'Messaggio',
    provider: 'Fornitore',
    model: 'Modello',
    attachments: 'Allegati',
    file: 'file',
    attempt: 'Tentativo',
    delay: 'Ritardo',
    continuePreviousReply: 'Continua la risposta precedente',
    resumePreviousRequest: 'Riprendi la richiesta precedente',
    requestReady: 'Richiesta pronta',
    requestSent: 'Richiesta inviata',
    waitingForResponse: 'In attesa di risposta',
    recoveringResponse: 'Recupero della risposta',
    requestDispatched: 'In attesa che il server accetti la richiesta e avvii la risposta.',
    waitingForFirstVisibleOutput:
      'La richiesta è stata accettata. In attesa del primo output visibile.',
    recoveryStage1: 'Recupero silenzioso',
    recoveryStage2: 'Payload di recupero ridotto',
  }),
  'ja-JP': makeTerms({
    message: 'メッセージ',
    provider: 'プロバイダー',
    model: 'モデル',
    attachments: '添付ファイル',
    file: 'ファイル',
    attempt: '試行',
    delay: '遅延',
    continuePreviousReply: '前回の返信を続行',
    resumePreviousRequest: '前回のリクエストを再開',
    requestReady: 'リクエストの準備完了',
    requestSent: 'リクエストを送信済み',
    waitingForResponse: '応答を待機中',
    recoveringResponse: '応答を復旧中',
    requestDispatched: 'サーバーがリクエストを受け付けて応答を開始するのを待っています。',
    waitingForFirstVisibleOutput: 'リクエストは受け付けられました。最初の可視出力を待っています。',
    recoveryStage1: 'サイレント復旧',
    recoveryStage2: '復旧ペイロードを縮小',
  }),
  'ko-KR': makeTerms({
    message: '메시지',
    provider: '제공자',
    model: '모델',
    attachments: '첨부 파일',
    file: '파일',
    attempt: '시도',
    delay: '지연',
    continuePreviousReply: '이전 답변 계속하기',
    resumePreviousRequest: '이전 요청 다시 이어가기',
    requestReady: '요청 준비 완료',
    requestSent: '요청 전송됨',
    waitingForResponse: '응답 대기 중',
    recoveringResponse: '응답 복구 중',
    requestDispatched: '서버가 요청을 수락하고 응답을 시작할 때까지 기다리는 중입니다.',
    waitingForFirstVisibleOutput:
      '요청이 수락되었습니다. 첫 번째 표시 가능한 출력을 기다리는 중입니다.',
    recoveryStage1: '조용한 복구',
    recoveryStage2: '축소된 복구 페이로드',
  }),
  'ml-IN': makeTerms({
    message: 'സന്ദേശം',
    provider: 'പ്രൊവൈഡർ',
    model: 'മോഡൽ',
    attachments: 'അറ്റാച്ച്മെന്റുകൾ',
    file: 'ഫയൽ',
    attempt: 'ശ്രമം',
    delay: 'വൈകിപ്പ്',
    continuePreviousReply: 'മുൻപത്തെ മറുപടി തുടരുക',
    resumePreviousRequest: 'മുൻപത്തെ അഭ്യർത്ഥനം വീണ്ടും തുടരുക',
    requestReady: 'അഭ്യർത്ഥന തയ്യാറാണ്',
    requestSent: 'അഭ്യർത്ഥന അയച്ചു',
    waitingForResponse: 'പ്രതികരണത്തിനായി കാത്തിരിക്കുന്നു',
    recoveringResponse: 'പ്രതികരണം വീണ്ടെടുക്കുന്നു',
    requestDispatched: 'സെർവർ അഭ്യർത്ഥന സ്വീകരിച്ച് പ്രതികരണം ആരംഭിക്കുന്നതുവരെ കാത്തിരിക്കുന്നു.',
    waitingForFirstVisibleOutput:
      'അഭ്യർത്ഥന സ്വീകരിച്ചു. ആദ്യമായി കാണുന്ന ഔട്ട്പുട്ടിനായി കാത്തിരിക്കുന്നു.',
    recoveryStage1: 'നിശ്ശബ്ദ വീണ്ടെടുപ്പ്',
    recoveryStage2: 'ചുരുക്കിയ വീണ്ടെടുപ്പ് പേലോഡ്',
  }),
  'nb-NO': makeTerms({
    message: 'Melding',
    provider: 'Leverandør',
    model: 'Modell',
    attachments: 'Vedlegg',
    file: 'fil',
    attempt: 'Forsøk',
    delay: 'Forsinkelse',
    continuePreviousReply: 'Fortsett forrige svar',
    resumePreviousRequest: 'Gjenoppta forrige forespørsel',
    requestReady: 'Forespørsel klar',
    requestSent: 'Forespørsel sendt',
    waitingForResponse: 'Venter på svar',
    recoveringResponse: 'Gjenoppretter svar',
    requestDispatched: 'Venter på at serveren skal godta forespørselen og starte svaret.',
    waitingForFirstVisibleOutput:
      'Forespørselen ble godtatt. Venter på det første synlige resultatet.',
    recoveryStage1: 'Stille gjenoppretting',
    recoveryStage2: 'Redusert gjenopprettingslast',
  }),
  'nl-NL': makeTerms({
    message: 'Bericht',
    provider: 'Aanbieder',
    model: 'Model',
    attachments: 'Bijlagen',
    file: 'bestand',
    attempt: 'Poging',
    delay: 'Vertraging',
    continuePreviousReply: 'Vorig antwoord voortzetten',
    resumePreviousRequest: 'Vorige aanvraag hervatten',
    requestReady: 'Verzoek gereed',
    requestSent: 'Verzoek verzonden',
    waitingForResponse: 'Wachten op antwoord',
    recoveringResponse: 'Antwoord herstellen',
    requestDispatched: 'Wachten tot de server het verzoek accepteert en het antwoord start.',
    waitingForFirstVisibleOutput:
      'Het verzoek is geaccepteerd. Wachten op de eerste zichtbare uitvoer.',
    recoveryStage1: 'Stil herstel',
    recoveryStage2: 'Verminderde herstelpayload',
  }),
  'pl-PL': makeTerms({
    message: 'Wiadomość',
    provider: 'Dostawca',
    model: 'Model',
    attachments: 'Załączniki',
    file: 'plik',
    attempt: 'Próba',
    delay: 'Opóźnienie',
    continuePreviousReply: 'Kontynuuj poprzednią odpowiedź',
    resumePreviousRequest: 'Wznów poprzednie żądanie',
    requestReady: 'Żądanie gotowe',
    requestSent: 'Żądanie wysłane',
    waitingForResponse: 'Oczekiwanie na odpowiedź',
    recoveringResponse: 'Odzyskiwanie odpowiedzi',
    requestDispatched: 'Oczekiwanie, aż serwer zaakceptuje żądanie i rozpocznie odpowiedź.',
    waitingForFirstVisibleOutput:
      'Żądanie zostało zaakceptowane. Oczekiwanie na pierwszy widoczny wynik.',
    recoveryStage1: 'Ciche odzyskiwanie',
    recoveryStage2: 'Zmniejszony ładunek odzyskiwania',
  }),
  'pt-BR': makeTerms({
    message: 'Mensagem',
    provider: 'Provedor',
    model: 'Modelo',
    attachments: 'Anexos',
    file: 'arquivo',
    attempt: 'Tentativa',
    delay: 'Atraso',
    continuePreviousReply: 'Continuar a resposta anterior',
    resumePreviousRequest: 'Retomar a solicitação anterior',
    requestReady: 'Solicitação pronta',
    requestSent: 'Solicitação enviada',
    waitingForResponse: 'Aguardando resposta',
    recoveringResponse: 'Recuperando resposta',
    requestDispatched: 'Aguardando o servidor aceitar a solicitação e iniciar a resposta.',
    waitingForFirstVisibleOutput: 'A solicitação foi aceita. Aguardando a primeira saída visível.',
    recoveryStage1: 'Recuperação silenciosa',
    recoveryStage2: 'Carga de recuperação reduzida',
  }),
  'pt-PT': makeTerms({
    message: 'Mensagem',
    provider: 'Fornecedor',
    model: 'Modelo',
    attachments: 'Anexos',
    file: 'ficheiro',
    attempt: 'Tentativa',
    delay: 'Atraso',
    continuePreviousReply: 'Continuar a resposta anterior',
    resumePreviousRequest: 'Retomar o pedido anterior',
    requestReady: 'Pedido pronto',
    requestSent: 'Pedido enviado',
    waitingForResponse: 'A aguardar resposta',
    recoveringResponse: 'A recuperar resposta',
    requestDispatched: 'A aguardar que o servidor aceite o pedido e inicie a resposta.',
    waitingForFirstVisibleOutput: 'O pedido foi aceite. A aguardar pela primeira saída visível.',
    recoveryStage1: 'Recuperação silenciosa',
    recoveryStage2: 'Carga de recuperação reduzida',
  }),
  'ro-RO': makeTerms({
    message: 'Mesaj',
    provider: 'Furnizor',
    model: 'Model',
    attachments: 'Atașamente',
    file: 'fișier',
    attempt: 'Încercare',
    delay: 'Întârziere',
    continuePreviousReply: 'Continuă răspunsul anterior',
    resumePreviousRequest: 'Reia cererea anterioară',
    requestReady: 'Cererea este pregătită',
    requestSent: 'Cererea a fost trimisă',
    waitingForResponse: 'Se așteaptă răspunsul',
    recoveringResponse: 'Se recuperează răspunsul',
    requestDispatched: 'Se așteaptă ca serverul să accepte cererea și să înceapă răspunsul.',
    waitingForFirstVisibleOutput: 'Cererea a fost acceptată. Se așteaptă primul rezultat vizibil.',
    recoveryStage1: 'Recuperare silențioasă',
    recoveryStage2: 'Încărcătură de recuperare redusă',
  }),
  'ru-RU': makeTerms({
    message: 'Сообщение',
    provider: 'Провайдер',
    model: 'Модель',
    attachments: 'Вложения',
    file: 'файл',
    attempt: 'Попытка',
    delay: 'Задержка',
    continuePreviousReply: 'Продолжить предыдущий ответ',
    resumePreviousRequest: 'Возобновить предыдущий запрос',
    requestReady: 'Запрос готов',
    requestSent: 'Запрос отправлен',
    waitingForResponse: 'Ожидание ответа',
    recoveringResponse: 'Восстановление ответа',
    requestDispatched: 'Ожидание, пока сервер примет запрос и начнет ответ.',
    waitingForFirstVisibleOutput: 'Запрос принят. Ожидание первого видимого вывода.',
    recoveryStage1: 'Тихое восстановление',
    recoveryStage2: 'Уменьшенный пакет восстановления',
  }),
  'sk-SK': makeTerms({
    message: 'Správa',
    provider: 'Poskytovateľ',
    model: 'Model',
    attachments: 'Prílohy',
    file: 'súbor',
    attempt: 'Pokus',
    delay: 'Oneskorenie',
    continuePreviousReply: 'Pokračovať v predchádzajúcej odpovedi',
    resumePreviousRequest: 'Obnoviť predchádzajúcu požiadavku',
    requestReady: 'Požiadavka je pripravená',
    requestSent: 'Požiadavka odoslaná',
    waitingForResponse: 'Čaká sa na odpoveď',
    recoveringResponse: 'Obnovuje sa odpoveď',
    requestDispatched: 'Čaká sa, kým server prijme požiadavku a začne odpoveď.',
    waitingForFirstVisibleOutput: 'Požiadavka bola prijatá. Čaká sa na prvý viditeľný výstup.',
    recoveryStage1: 'Tiché obnovenie',
    recoveryStage2: 'Zmenšené údaje obnovenia',
  }),
  'sv-SE': makeTerms({
    message: 'Meddelande',
    provider: 'Leverantör',
    model: 'Modell',
    attachments: 'Bilagor',
    file: 'fil',
    attempt: 'Försök',
    delay: 'Fördröjning',
    continuePreviousReply: 'Fortsätt föregående svar',
    resumePreviousRequest: 'Återuppta föregående begäran',
    requestReady: 'Begäran klar',
    requestSent: 'Begäran skickad',
    waitingForResponse: 'Väntar på svar',
    recoveringResponse: 'Återställer svar',
    requestDispatched: 'Väntar på att servern ska acceptera begäran och starta svaret.',
    waitingForFirstVisibleOutput:
      'Begäran har accepterats. Väntar på den första synliga utmatningen.',
    recoveryStage1: 'Tyst återställning',
    recoveryStage2: 'Reducerad återställningspayload',
  }),
  'zh-CN': makeTerms({
    message: '消息',
    provider: '提供商',
    model: '模型',
    attachments: '附件',
    file: '文件',
    attempt: '尝试',
    delay: '延迟',
    continuePreviousReply: '继续上一条回复',
    resumePreviousRequest: '恢复上一条请求',
    requestReady: '请求已就绪',
    requestSent: '请求已发送',
    waitingForResponse: '正在等待响应',
    recoveringResponse: '正在恢复响应',
    requestDispatched: '正在等待服务器接受请求并开始响应。',
    waitingForFirstVisibleOutput: '请求已被接受。正在等待第一段可见输出。',
    recoveryStage1: '静默恢复',
    recoveryStage2: '缩减恢复负载',
  }),
  'zh-TW': makeTerms({
    message: '訊息',
    provider: '提供商',
    model: '模型',
    attachments: '附件',
    file: '檔案',
    attempt: '嘗試',
    delay: '延遲',
    continuePreviousReply: '繼續上一則回覆',
    resumePreviousRequest: '恢復上一則請求',
    requestReady: '請求已就緒',
    requestSent: '請求已送出',
    waitingForResponse: '正在等待回應',
    recoveringResponse: '正在恢復回應',
    requestDispatched: '正在等待伺服器接受請求並開始回應。',
    waitingForFirstVisibleOutput: '請求已被接受。正在等待第一段可見輸出。',
    recoveryStage1: '靜默恢復',
    recoveryStage2: '縮減恢復負載',
  }),
}

const voiceProcessTraceTerms: Record<LocaleKey, VoiceProcessTraceTerms> = {
  'ca-ES': makeVoiceTerms({
    format: 'Format',
    size: 'Mida',
    duration: 'Durada',
    upload: 'Càrrega',
    transcript: 'Transcripció',
    mode: 'Mode',
    conversation: 'Conversa',
    interruptCurrentReply: 'Interromp la resposta actual',
    sendNewRequest: 'Envia una sol·licitud nova',
    voiceInterruptDispatched: "Blue s'està reiniciant amb la teva darrera interrupció de veu.",
    voiceRequestDispatched: 'Blue està enviant la teva sol·licitud de veu.',
    voiceWaitingForResponse: "S'està esperant la primera resposta de Blue.",
    bargeIn: "La reproducció s'ha aturat perquè puguis continuar parlant.",
    transcriptionFailed: 'La transcripció ha fallat.',
  }),
  'cs-CZ': makeVoiceTerms({
    format: 'Formát',
    size: 'Velikost',
    duration: 'Délka',
    upload: 'Nahrávání',
    transcript: 'Přepis',
    mode: 'Režim',
    conversation: 'Konverzace',
    interruptCurrentReply: 'Přerušit aktuální odpověď',
    sendNewRequest: 'Poslat nový požadavek',
    voiceInterruptDispatched: 'Blue se restartuje s vaším nejnovějším hlasovým přerušením.',
    voiceRequestDispatched: 'Blue odesílá váš hlasový požadavek.',
    voiceWaitingForResponse: 'Čeká se na první odpověď od Blue.',
    bargeIn: 'Přehrávání bylo zastaveno, abyste mohli pokračovat v mluvení.',
    transcriptionFailed: 'Přepis selhal.',
  }),
  'da-DK': makeVoiceTerms({
    format: 'Format',
    size: 'Størrelse',
    duration: 'Varighed',
    upload: 'Upload',
    transcript: 'Transskription',
    mode: 'Tilstand',
    conversation: 'Samtale',
    interruptCurrentReply: 'Afbryd det aktuelle svar',
    sendNewRequest: 'Send ny anmodning',
    voiceInterruptDispatched: 'Blue genstarter med din seneste stemmeafbrydelse.',
    voiceRequestDispatched: 'Blue sender din stemmeanmodning.',
    voiceWaitingForResponse: 'Venter på det første svar fra Blue.',
    bargeIn: 'Afspilningen blev stoppet, så du kan fortsætte med at tale.',
    transcriptionFailed: 'Transskriptionen mislykkedes.',
  }),
  'de-DE': makeVoiceTerms({
    format: 'Format',
    size: 'Größe',
    duration: 'Dauer',
    upload: 'Upload',
    transcript: 'Transkript',
    mode: 'Modus',
    conversation: 'Konversation',
    interruptCurrentReply: 'Aktuelle Antwort unterbrechen',
    sendNewRequest: 'Neue Anfrage senden',
    voiceInterruptDispatched: 'Blue startet mit Ihrer neuesten Sprachunterbrechung neu.',
    voiceRequestDispatched: 'Blue sendet Ihre Sprachanfrage.',
    voiceWaitingForResponse: 'Warten auf die erste Antwort von Blue.',
    bargeIn: 'Die Wiedergabe wurde gestoppt, damit Sie weitersprechen können.',
    transcriptionFailed: 'Die Transkription ist fehlgeschlagen.',
  }),
  'el-GR': makeVoiceTerms({
    format: 'Μορφή',
    size: 'Μέγεθος',
    duration: 'Διάρκεια',
    upload: 'Μεταφόρτωση',
    transcript: 'Μεταγραφή',
    mode: 'Λειτουργία',
    conversation: 'Συνομιλία',
    interruptCurrentReply: 'Διακοπή της τρέχουσας απάντησης',
    sendNewRequest: 'Αποστολή νέου αιτήματος',
    voiceInterruptDispatched:
      'Το Blue επανεκκινεί με την πιο πρόσφατη φωνητική διακοπή σας.',
    voiceRequestDispatched: 'Το Blue στέλνει το φωνητικό σας αίτημα.',
    voiceWaitingForResponse: 'Αναμονή για την πρώτη απάντηση από το Blue.',
    bargeIn: 'Η αναπαραγωγή σταμάτησε ώστε να μπορείτε να συνεχίσετε να μιλάτε.',
    transcriptionFailed: 'Η απομαγνητοφώνηση απέτυχε.',
  }),
  'en-GB': makeVoiceTerms({
    format: 'Format',
    size: 'Size',
    duration: 'Duration',
    upload: 'Upload',
    transcript: 'Transcript',
    mode: 'Mode',
    conversation: 'Conversation',
    interruptCurrentReply: 'Interrupt current reply',
    sendNewRequest: 'Send new request',
    voiceInterruptDispatched: 'Blue is restarting with your latest voice interruption.',
    voiceRequestDispatched: 'Blue is sending your voice request.',
    voiceWaitingForResponse: 'Waiting for the first response from Blue.',
    bargeIn: 'Playback stopped so you can continue speaking.',
    transcriptionFailed: 'Transcription failed.',
  }),
  'en-US': makeVoiceTerms({
    format: 'Format',
    size: 'Size',
    duration: 'Duration',
    upload: 'Upload',
    transcript: 'Transcript',
    mode: 'Mode',
    conversation: 'Conversation',
    interruptCurrentReply: 'Interrupt current reply',
    sendNewRequest: 'Send new request',
    voiceInterruptDispatched: 'Blue is restarting with your latest voice interruption.',
    voiceRequestDispatched: 'Blue is sending your voice request.',
    voiceWaitingForResponse: 'Waiting for the first response from Blue.',
    bargeIn: 'Playback stopped so you can continue speaking.',
    transcriptionFailed: 'Transcription failed.',
  }),
  'es-ES': makeVoiceTerms({
    format: 'Formato',
    size: 'Tamaño',
    duration: 'Duración',
    upload: 'Subida',
    transcript: 'Transcripción',
    mode: 'Modo',
    conversation: 'Conversación',
    interruptCurrentReply: 'Interrumpir la respuesta actual',
    sendNewRequest: 'Enviar nueva solicitud',
    voiceInterruptDispatched: 'Blue se está reiniciando con tu última interrupción de voz.',
    voiceRequestDispatched: 'Blue está enviando tu solicitud de voz.',
    voiceWaitingForResponse: 'Esperando la primera respuesta de Blue.',
    bargeIn: 'La reproducción se detuvo para que puedas seguir hablando.',
    transcriptionFailed: 'La transcripción ha fallado.',
  }),
  'fr-FR': makeVoiceTerms({
    format: 'Format',
    size: 'Taille',
    duration: 'Durée',
    upload: 'Téléversement',
    transcript: 'Transcription',
    mode: 'Mode',
    conversation: 'Conversation',
    interruptCurrentReply: 'Interrompre la réponse en cours',
    sendNewRequest: 'Envoyer une nouvelle requête',
    voiceInterruptDispatched:
      'Blue redémarre avec votre dernière interruption vocale.',
    voiceRequestDispatched: 'Blue envoie votre requête vocale.',
    voiceWaitingForResponse: 'En attente de la première réponse de Blue.',
    bargeIn: 'La lecture a été arrêtée pour que vous puissiez continuer à parler.',
    transcriptionFailed: 'La transcription a échoué.',
  }),
  'ga-IE': makeVoiceTerms({
    format: 'Formáid',
    size: 'Méid',
    duration: 'Fad',
    upload: 'Uaslódáil',
    transcript: 'Tras-scríbhinn',
    mode: 'Mód',
    conversation: 'Comhrá',
    interruptCurrentReply: 'Cuir isteach ar an bhfreagra reatha',
    sendNewRequest: 'Seol iarratas nua',
    voiceInterruptDispatched:
      "Tá Blue ag atosú le d'idirbhriseadh gutha is déanaí.",
    voiceRequestDispatched: "Tá Blue ag seoladh d'iarratais gutha.",
    voiceWaitingForResponse: 'Ag fanacht leis an gcéad fhreagra ó Blue.',
    bargeIn: 'Cuireadh stop leis an athsheinm ionas gur féidir leat leanúint ar aghaidh ag caint.',
    transcriptionFailed: 'Theip ar an tras-scríobh.',
  }),
  'hr-HR': makeVoiceTerms({
    format: 'Format',
    size: 'Veličina',
    duration: 'Trajanje',
    upload: 'Prijenos',
    transcript: 'Transkript',
    mode: 'Način',
    conversation: 'Razgovor',
    interruptCurrentReply: 'Prekini trenutačni odgovor',
    sendNewRequest: 'Pošalji novi zahtjev',
    voiceInterruptDispatched:
      'Blue se ponovno pokreće s vašim najnovijim glasovnim prekidom.',
    voiceRequestDispatched: 'Blue šalje vaš glasovni zahtjev.',
    voiceWaitingForResponse: 'Čeka se prvi odgovor od Blue.',
    bargeIn: 'Reprodukcija je zaustavljena kako biste mogli nastaviti govoriti.',
    transcriptionFailed: 'Transkripcija nije uspjela.',
  }),
  'hu-HU': makeVoiceTerms({
    format: 'Formátum',
    size: 'Méret',
    duration: 'Időtartam',
    upload: 'Feltöltés',
    transcript: 'Átirat',
    mode: 'Mód',
    conversation: 'Beszélgetés',
    interruptCurrentReply: 'Aktuális válasz megszakítása',
    sendNewRequest: 'Új kérés küldése',
    voiceInterruptDispatched:
      'A Blue újraindul az Ön legutóbbi hangos megszakításával.',
    voiceRequestDispatched: 'A Blue elküldi az Ön hangkérését.',
    voiceWaitingForResponse: 'Várakozás a Blue első válaszára.',
    bargeIn: 'A lejátszás leállt, így folytathatja a beszédet.',
    transcriptionFailed: 'Az átírás sikertelen.',
  }),
  'it-IT': makeVoiceTerms({
    format: 'Formato',
    size: 'Dimensione',
    duration: 'Durata',
    upload: 'Caricamento',
    transcript: 'Trascrizione',
    mode: 'Modalità',
    conversation: 'Conversazione',
    interruptCurrentReply: 'Interrompi la risposta corrente',
    sendNewRequest: 'Invia una nuova richiesta',
    voiceInterruptDispatched:
      'Blue si sta riavviando con la tua ultima interruzione vocale.',
    voiceRequestDispatched: 'Blue sta inviando la tua richiesta vocale.',
    voiceWaitingForResponse: 'In attesa della prima risposta da Blue.',
    bargeIn: 'La riproduzione è stata interrotta per permetterti di continuare a parlare.',
    transcriptionFailed: 'Trascrizione non riuscita.',
  }),
  'ja-JP': makeVoiceTerms({
    format: '形式',
    size: 'サイズ',
    duration: '長さ',
    upload: 'アップロード',
    transcript: '文字起こし',
    mode: 'モード',
    conversation: '会話',
    interruptCurrentReply: '現在の応答を中断',
    sendNewRequest: '新しいリクエストを送信',
    voiceInterruptDispatched: 'Blue は最新の音声割り込みで再開しています。',
    voiceRequestDispatched: 'Blue が音声リクエストを送信しています。',
    voiceWaitingForResponse: 'Blue からの最初の応答を待っています。',
    bargeIn: '再生を停止したので、そのまま話し続けられます。',
    transcriptionFailed: '文字起こしに失敗しました。',
  }),
  'ko-KR': makeVoiceTerms({
    format: '형식',
    size: '크기',
    duration: '지속 시간',
    upload: '업로드',
    transcript: '전사',
    mode: '모드',
    conversation: '대화',
    interruptCurrentReply: '현재 응답 중단',
    sendNewRequest: '새 요청 보내기',
    voiceInterruptDispatched:
      'Blue가 최신 음성 인터럽트를 반영해 다시 시작하고 있습니다.',
    voiceRequestDispatched: 'Blue가 음성 요청을 보내고 있습니다.',
    voiceWaitingForResponse: 'Blue의 첫 응답을 기다리는 중입니다.',
    bargeIn: '계속 말씀하실 수 있도록 재생을 중지했습니다.',
    transcriptionFailed: '전사에 실패했습니다.',
  }),
  'ml-IN': makeVoiceTerms({
    format: 'ഫോർമാറ്റ്',
    size: 'വലുപ്പം',
    duration: 'ദൈർഘ്യം',
    upload: 'അപ്‌ലോഡ്',
    transcript: 'ട്രാൻസ്ക്രിപ്റ്റ്',
    mode: 'മോഡ്',
    conversation: 'സംഭാഷണം',
    interruptCurrentReply: 'നിലവിലെ മറുപടി തടസ്സപ്പെടുത്തുക',
    sendNewRequest: 'പുതിയ അഭ്യർത്ഥന അയയ്ക്കുക',
    voiceInterruptDispatched:
      'നിങ്ങളുടെ ഏറ്റവും പുതിയ ശബ്ദ ഇടപെടലോടെ Blue വീണ്ടും ആരംഭിക്കുന്നു.',
    voiceRequestDispatched: 'Blue നിങ്ങളുടെ ശബ്ദ അഭ്യർത്ഥന അയയ്ക്കുകയാണ്.',
    voiceWaitingForResponse: 'Blueയിൽ നിന്ന് ആദ്യ പ്രതികരണം കാത്തിരിക്കുകയാണ്.',
    bargeIn: 'നിങ്ങൾക്ക് സംസാരിക്കുന്നത് തുടരാൻ പ്ലേബാക്ക് നിർത്തി.',
    transcriptionFailed: 'ലിപ്യന്തരണം പരാജയപ്പെട്ടു.',
  }),
  'nb-NO': makeVoiceTerms({
    format: 'Format',
    size: 'Størrelse',
    duration: 'Varighet',
    upload: 'Opplasting',
    transcript: 'Transkripsjon',
    mode: 'Modus',
    conversation: 'Samtale',
    interruptCurrentReply: 'Avbryt gjeldende svar',
    sendNewRequest: 'Send ny forespørsel',
    voiceInterruptDispatched: 'Blue starter på nytt med den siste taleavbrytelsen din.',
    voiceRequestDispatched: 'Blue sender taleforespørselen din.',
    voiceWaitingForResponse: 'Venter på det første svaret fra Blue.',
    bargeIn: 'Avspillingen ble stoppet slik at du kan fortsette å snakke.',
    transcriptionFailed: 'Transkripsjonen mislyktes.',
  }),
  'nl-NL': makeVoiceTerms({
    format: 'Formaat',
    size: 'Grootte',
    duration: 'Duur',
    upload: 'Upload',
    transcript: 'Afschrift',
    mode: 'Modus',
    conversation: 'Gesprek',
    interruptCurrentReply: 'Huidig antwoord onderbreken',
    sendNewRequest: 'Nieuw verzoek verzenden',
    voiceInterruptDispatched: 'Blue start opnieuw met je meest recente spraakonderbreking.',
    voiceRequestDispatched: 'Blue verstuurt je spraakverzoek.',
    voiceWaitingForResponse: 'Wachten op de eerste reactie van Blue.',
    bargeIn: 'De weergave is gestopt zodat je verder kunt praten.',
    transcriptionFailed: 'Transcriptie mislukt.',
  }),
  'pl-PL': makeVoiceTerms({
    format: 'Format',
    size: 'Rozmiar',
    duration: 'Czas trwania',
    upload: 'Przesyłanie',
    transcript: 'Transkrypcja',
    mode: 'Tryb',
    conversation: 'Rozmowa',
    interruptCurrentReply: 'Przerwij bieżącą odpowiedź',
    sendNewRequest: 'Wyślij nowe żądanie',
    voiceInterruptDispatched:
      'Blue uruchamia się ponownie z Twoim ostatnim przerwaniem głosowym.',
    voiceRequestDispatched: 'Blue wysyła Twoje żądanie głosowe.',
    voiceWaitingForResponse: 'Oczekiwanie na pierwszą odpowiedź od Blue.',
    bargeIn: 'Odtwarzanie zatrzymano, aby można było dalej mówić.',
    transcriptionFailed: 'Transkrypcja nie powiodła się.',
  }),
  'pt-BR': makeVoiceTerms({
    format: 'Formato',
    size: 'Tamanho',
    duration: 'Duração',
    upload: 'Upload',
    transcript: 'Transcrição',
    mode: 'Modo',
    conversation: 'Conversa',
    interruptCurrentReply: 'Interromper resposta atual',
    sendNewRequest: 'Enviar nova solicitação',
    voiceInterruptDispatched:
      'Blue está reiniciando com sua interrupção de voz mais recente.',
    voiceRequestDispatched: 'Blue está enviando sua solicitação de voz.',
    voiceWaitingForResponse: 'Aguardando a primeira resposta do Blue.',
    bargeIn: 'A reprodução foi interrompida para que você possa continuar falando.',
    transcriptionFailed: 'A transcrição falhou.',
  }),
  'pt-PT': makeVoiceTerms({
    format: 'Formato',
    size: 'Tamanho',
    duration: 'Duração',
    upload: 'Carregamento',
    transcript: 'Transcrição',
    mode: 'Modo',
    conversation: 'Conversa',
    interruptCurrentReply: 'Interromper a resposta atual',
    sendNewRequest: 'Enviar novo pedido',
    voiceInterruptDispatched:
      'O Blue está a reiniciar com a sua interrupção de voz mais recente.',
    voiceRequestDispatched: 'O Blue está a enviar o seu pedido de voz.',
    voiceWaitingForResponse: 'A aguardar a primeira resposta do Blue.',
    bargeIn: 'A reprodução foi interrompida para que possa continuar a falar.',
    transcriptionFailed: 'A transcrição falhou.',
  }),
  'ro-RO': makeVoiceTerms({
    format: 'Format',
    size: 'Dimensiune',
    duration: 'Durată',
    upload: 'Încărcare',
    transcript: 'Transcriere',
    mode: 'Mod',
    conversation: 'Conversație',
    interruptCurrentReply: 'Întrerupe răspunsul curent',
    sendNewRequest: 'Trimite o solicitare nouă',
    voiceInterruptDispatched:
      'Blue repornește cu cea mai recentă întrerupere vocală a ta.',
    voiceRequestDispatched: 'Blue trimite solicitarea ta vocală.',
    voiceWaitingForResponse: 'Se așteaptă primul răspuns de la Blue.',
    bargeIn: 'Redarea a fost oprită ca să poți continua să vorbești.',
    transcriptionFailed: 'Transcrierea a eșuat.',
  }),
  'ru-RU': makeVoiceTerms({
    format: 'Формат',
    size: 'Размер',
    duration: 'Длительность',
    upload: 'Загрузка',
    transcript: 'Транскрипция',
    mode: 'Режим',
    conversation: 'Разговор',
    interruptCurrentReply: 'Прервать текущий ответ',
    sendNewRequest: 'Отправить новый запрос',
    voiceInterruptDispatched:
      'Blue перезапускается с вашим последним голосовым прерыванием.',
    voiceRequestDispatched: 'Blue отправляет ваш голосовой запрос.',
    voiceWaitingForResponse: 'Ожидание первого ответа от Blue.',
    bargeIn: 'Воспроизведение остановлено, чтобы вы могли продолжить говорить.',
    transcriptionFailed: 'Транскрипция не удалась.',
  }),
  'sk-SK': makeVoiceTerms({
    format: 'Formát',
    size: 'Veľkosť',
    duration: 'Trvanie',
    upload: 'Nahrávanie',
    transcript: 'Prepis',
    mode: 'Režim',
    conversation: 'Konverzácia',
    interruptCurrentReply: 'Prerušiť aktuálnu odpoveď',
    sendNewRequest: 'Odoslať novú požiadavku',
    voiceInterruptDispatched:
      'Blue sa reštartuje s vaším najnovším hlasovým prerušením.',
    voiceRequestDispatched: 'Blue odosiela vašu hlasovú požiadavku.',
    voiceWaitingForResponse: 'Čaká sa na prvú odpoveď od Blue.',
    bargeIn: 'Prehrávanie sa zastavilo, aby ste mohli pokračovať v hovorení.',
    transcriptionFailed: 'Prepis zlyhal.',
  }),
  'sv-SE': makeVoiceTerms({
    format: 'Format',
    size: 'Storlek',
    duration: 'Varaktighet',
    upload: 'Uppladdning',
    transcript: 'Transkription',
    mode: 'Läge',
    conversation: 'Konversation',
    interruptCurrentReply: 'Avbryt aktuellt svar',
    sendNewRequest: 'Skicka ny begäran',
    voiceInterruptDispatched: 'Blue startar om med ditt senaste röstavbrott.',
    voiceRequestDispatched: 'Blue skickar din röstförfrågan.',
    voiceWaitingForResponse: 'Väntar på det första svaret från Blue.',
    bargeIn: 'Uppspelningen stoppades så att du kan fortsätta prata.',
    transcriptionFailed: 'Transkriptionen misslyckades.',
  }),
  'zh-CN': makeVoiceTerms({
    format: '格式',
    size: '大小',
    duration: '时长',
    upload: '上传',
    transcript: '转写',
    mode: '模式',
    conversation: '对话',
    interruptCurrentReply: '中断当前回复',
    sendNewRequest: '发送新请求',
    voiceInterruptDispatched: 'Blue 正根据你最新的语音打断重新开始。',
    voiceRequestDispatched: 'Blue 正在发送你的语音请求。',
    voiceWaitingForResponse: '正在等待 Blue 的第一条响应。',
    bargeIn: '已停止播放，方便你继续说话。',
    transcriptionFailed: '转写失败。',
  }),
  'zh-TW': makeVoiceTerms({
    format: '格式',
    size: '大小',
    duration: '時長',
    upload: '上傳',
    transcript: '逐字稿',
    mode: '模式',
    conversation: '對話',
    interruptCurrentReply: '中斷目前回覆',
    sendNewRequest: '發送新請求',
    voiceInterruptDispatched: 'Blue 正根據你最新的語音打斷重新開始。',
    voiceRequestDispatched: 'Blue 正在傳送你的語音請求。',
    voiceWaitingForResponse: '正在等待 Blue 的第一則回應。',
    bargeIn: '已停止播放，方便你繼續說話。',
    transcriptionFailed: '轉寫失敗。',
  }),
}

const processTraceEventTerms: Record<LocaleKey, ProcessTraceEventTerms> = {
  'ca-ES': makeEventTerms({
    providerResolved: "S'utilitza una ruta disponible",
    processing: 'Processant',
  }),
  'cs-CZ': makeEventTerms({
    providerResolved: 'Používá se dostupná trasa',
    processing: 'Zpracovává se',
  }),
  'da-DK': makeEventTerms({
    providerResolved: 'Bruger en tilgængelig rute',
    processing: 'Behandler',
  }),
  'de-DE': makeEventTerms({
    providerResolved: 'Verfügbare Route wird verwendet',
    processing: 'Wird verarbeitet',
  }),
  'el-GR': makeEventTerms({
    providerResolved: 'Χρήση διαθέσιμης διαδρομής',
    processing: 'Σε επεξεργασία',
  }),
  'en-GB': makeEventTerms({
    providerResolved: 'Using available route',
    processing: 'Processing',
  }),
  'en-US': makeEventTerms({
    providerResolved: 'Using available route',
    processing: 'Processing',
  }),
  'es-ES': makeEventTerms({
    providerResolved: 'Usando una ruta disponible',
    processing: 'Procesando',
  }),
  'fr-FR': makeEventTerms({
    providerResolved: "Utilisation d'une route disponible",
    processing: 'Traitement',
  }),
  'ga-IE': makeEventTerms({
    providerResolved: 'Ag úsáid bealaigh atá ar fáil',
    processing: 'Á phróiseáil',
  }),
  'hr-HR': makeEventTerms({
    providerResolved: 'Korištenje dostupne rute',
    processing: 'Obrada',
  }),
  'hu-HU': makeEventTerms({
    providerResolved: 'Elérhető útvonal használata',
    processing: 'Feldolgozás',
  }),
  'it-IT': makeEventTerms({
    providerResolved: 'Uso di un percorso disponibile',
    processing: 'Elaborazione',
  }),
  'ja-JP': makeEventTerms({
    providerResolved: '利用可能なルートを使用中',
    processing: '処理中',
  }),
  'ko-KR': makeEventTerms({
    providerResolved: '사용 가능한 경로 사용 중',
    processing: '처리 중',
  }),
  'ml-IN': makeEventTerms({
    providerResolved: 'ലഭ്യമായ റൂട്ട് ഉപയോഗിക്കുന്നു',
    processing: 'പ്രോസസ്സിംഗ്',
  }),
  'nb-NO': makeEventTerms({
    providerResolved: 'Bruker tilgjengelig rute',
    processing: 'Behandler',
  }),
  'nl-NL': makeEventTerms({
    providerResolved: 'Beschikbare route in gebruik',
    processing: 'Bezig met verwerken',
  }),
  'pl-PL': makeEventTerms({
    providerResolved: 'Korzystanie z dostępnej trasy',
    processing: 'Przetwarzanie',
  }),
  'pt-BR': makeEventTerms({
    providerResolved: 'Usando rota disponível',
    processing: 'Processando',
  }),
  'pt-PT': makeEventTerms({
    providerResolved: 'A usar rota disponível',
    processing: 'A processar',
  }),
  'ro-RO': makeEventTerms({
    providerResolved: 'Se folosește o rută disponibilă',
    processing: 'În procesare',
  }),
  'ru-RU': makeEventTerms({
    providerResolved: 'Используется доступный маршрут',
    processing: 'Обработка',
  }),
  'sk-SK': makeEventTerms({
    providerResolved: 'Používa sa dostupná trasa',
    processing: 'Spracováva sa',
  }),
  'sv-SE': makeEventTerms({
    providerResolved: 'Använder tillgänglig rutt',
    processing: 'Bearbetar',
  }),
  'zh-CN': makeEventTerms({
    providerResolved: '已选定可用路由',
    processing: '处理中',
  }),
  'zh-TW': makeEventTerms({
    providerResolved: '已選定可用路由',
    processing: '處理中',
  }),
}

const processTraceDetailTerms: Record<LocaleKey, ProcessTraceDetailTerms> = {
  'ca-ES': makeDetailTerms({
    providerResolved: "Aquesta resposta utilitzarà el proveïdor i el model upstream seleccionats.",
    providerResolvedModelSwitch: "Aquesta resposta ha canviat a un model upstream disponible.",
    providerResolvedFailoverSwitch:
      "Aquesta resposta ha canviat a un altre proveïdor o model upstream disponible.",
    providerFailoverToolFollowUp:
      "S'està tornant a provar el seguiment de l'eina sense el proveïdor fixat anteriorment.",
    providerFailoverContinuationFollowUp:
      "S'està tornant a provar el seguiment de continuació sense el proveïdor fixat anteriorment.",
  }),
  'cs-CZ': makeDetailTerms({
    providerResolved: 'Tato odpověď použije vybraného upstream poskytovatele a model.',
    providerResolvedModelSwitch: 'Tato odpověď se přepnula na dostupný upstream model.',
    providerResolvedFailoverSwitch:
      'Tato odpověď se přepnula na jiného dostupného upstream poskytovatele nebo model.',
    providerFailoverToolFollowUp:
      'Opakuje se navazující požadavek nástroje bez dříve připnutého poskytovatele.',
    providerFailoverContinuationFollowUp:
      'Opakuje se navazující pokračování bez dříve připnutého poskytovatele.',
  }),
  'da-DK': makeDetailTerms({
    providerResolved: 'Dette svar vil bruge den valgte upstream-udbyder og model.',
    providerResolvedModelSwitch: 'Dette svar skiftede til en tilgængelig upstream-model.',
    providerResolvedFailoverSwitch:
      'Dette svar skiftede til en anden tilgængelig upstream-udbyder eller model.',
    providerFailoverToolFollowUp:
      'Prøver værktøjsopfølgningen igen uden den tidligere fastgjorte udbyder.',
    providerFailoverContinuationFollowUp:
      'Prøver fortsættelsesopfølgningen igen uden den tidligere fastgjorte udbyder.',
  }),
  'de-DE': makeDetailTerms({
    providerResolved:
      'Diese Antwort verwendet den ausgewählten Upstream-Anbieter und das ausgewählte Modell.',
    providerResolvedModelSwitch:
      'Diese Antwort wurde auf ein verfügbares Upstream-Modell umgestellt.',
    providerResolvedFailoverSwitch:
      'Diese Antwort wurde auf einen anderen verfügbaren Upstream-Anbieter oder ein anderes Modell umgestellt.',
    providerFailoverToolFollowUp:
      'Die Tool-Nachverfolgung wird ohne den zuvor fixierten Anbieter erneut versucht.',
    providerFailoverContinuationFollowUp:
      'Die Fortsetzungs-Nachverfolgung wird ohne den zuvor fixierten Anbieter erneut versucht.',
  }),
  'el-GR': makeDetailTerms({
    providerResolved:
      'Αυτή η απάντηση θα χρησιμοποιήσει τον επιλεγμένο upstream πάροχο και μοντέλο.',
    providerResolvedModelSwitch:
      'Αυτή η απάντηση μεταφέρθηκε σε ένα διαθέσιμο upstream μοντέλο.',
    providerResolvedFailoverSwitch:
      'Αυτή η απάντηση μεταφέρθηκε σε άλλον διαθέσιμο upstream πάροχο ή μοντέλο.',
    providerFailoverToolFollowUp:
      'Γίνεται νέα προσπάθεια για τη συνέχεια του εργαλείου χωρίς τον προηγουμένως καρφιτσωμένο πάροχο.',
    providerFailoverContinuationFollowUp:
      'Γίνεται νέα προσπάθεια για τη συνέχεια της απάντησης χωρίς τον προηγουμένως καρφιτσωμένο πάροχο.',
  }),
  'en-GB': makeDetailTerms({
    providerResolved: 'This response will use the selected upstream provider and model.',
    providerResolvedModelSwitch: 'This response switched to an available upstream model.',
    providerResolvedFailoverSwitch:
      'This response switched to another available upstream provider or model.',
    providerFailoverToolFollowUp:
      'Retrying the tool follow-up without the previously pinned provider.',
    providerFailoverContinuationFollowUp:
      'Retrying the continuation follow-up without the previously pinned provider.',
  }),
  'en-US': makeDetailTerms({
    providerResolved: 'This response will use the selected upstream provider and model.',
    providerResolvedModelSwitch: 'This response switched to an available upstream model.',
    providerResolvedFailoverSwitch:
      'This response switched to another available upstream provider or model.',
    providerFailoverToolFollowUp:
      'Retrying the tool follow-up without the previously pinned provider.',
    providerFailoverContinuationFollowUp:
      'Retrying the continuation follow-up without the previously pinned provider.',
  }),
  'es-ES': makeDetailTerms({
    providerResolved: 'Esta respuesta usará el proveedor y el modelo upstream seleccionados.',
    providerResolvedModelSwitch: 'Esta respuesta cambió a un modelo upstream disponible.',
    providerResolvedFailoverSwitch:
      'Esta respuesta cambió a otro proveedor o modelo upstream disponible.',
    providerFailoverToolFollowUp:
      'Reintentando el seguimiento de la herramienta sin el proveedor fijado anteriormente.',
    providerFailoverContinuationFollowUp:
      'Reintentando el seguimiento de continuación sin el proveedor fijado anteriormente.',
  }),
  'fr-FR': makeDetailTerms({
    providerResolved: 'Cette réponse utilisera le fournisseur et le modèle upstream sélectionnés.',
    providerResolvedModelSwitch: 'Cette réponse est passée à un modèle upstream disponible.',
    providerResolvedFailoverSwitch:
      'Cette réponse est passée à un autre fournisseur ou modèle upstream disponible.',
    providerFailoverToolFollowUp:
      "Nouvelle tentative du suivi d'outil sans le fournisseur précédemment épinglé.",
    providerFailoverContinuationFollowUp:
      'Nouvelle tentative du suivi de continuation sans le fournisseur précédemment épinglé.',
  }),
  'ga-IE': makeDetailTerms({
    providerResolved:
      'Úsáidfidh an freagra seo an soláthraí agus an tsamhail upstream roghnaithe.',
    providerResolvedModelSwitch:
      "D'athraigh an freagra seo go samhail upstream atá ar fáil.",
    providerResolvedFailoverSwitch:
      "D'athraigh an freagra seo go soláthraí nó samhail upstream eile atá ar fáil.",
    providerFailoverToolFollowUp:
      'Ag atriail obair leantach na huirlise gan an soláthraí a bhí pionnaithe roimhe seo.',
    providerFailoverContinuationFollowUp:
      'Ag atriail obair leantach an leanúnais gan an soláthraí a bhí pionnaithe roimhe seo.',
  }),
  'hr-HR': makeDetailTerms({
    providerResolved: 'Ovaj odgovor upotrijebit će odabranog upstream pružatelja i model.',
    providerResolvedModelSwitch: 'Ovaj odgovor prebačen je na dostupan upstream model.',
    providerResolvedFailoverSwitch:
      'Ovaj odgovor prebačen je na drugog dostupnog upstream pružatelja ili model.',
    providerFailoverToolFollowUp:
      'Ponovno se pokušava naknadni zahtjev alata bez prethodno prikvačenog pružatelja.',
    providerFailoverContinuationFollowUp:
      'Ponovno se pokušava naknadni zahtjev nastavka bez prethodno prikvačenog pružatelja.',
  }),
  'hu-HU': makeDetailTerms({
    providerResolved:
      'Ez a válasz a kiválasztott upstream szolgáltatót és modellt fogja használni.',
    providerResolvedModelSwitch: 'Ez a válasz egy elérhető upstream modellre váltott.',
    providerResolvedFailoverSwitch:
      'Ez a válasz egy másik elérhető upstream szolgáltatóra vagy modellre váltott.',
    providerFailoverToolFollowUp:
      'Az eszköz utókövetése újrapróbálkozik a korábban rögzített szolgáltató nélkül.',
    providerFailoverContinuationFollowUp:
      'A folytatási utókövetés újrapróbálkozik a korábban rögzített szolgáltató nélkül.',
  }),
  'it-IT': makeDetailTerms({
    providerResolved: 'Questa risposta userà il provider upstream e il modello selezionati.',
    providerResolvedModelSwitch: 'Questa risposta è passata a un modello upstream disponibile.',
    providerResolvedFailoverSwitch:
      'Questa risposta è passata a un altro provider o modello upstream disponibile.',
    providerFailoverToolFollowUp:
      'Nuovo tentativo del follow-up dello strumento senza il provider precedentemente fissato.',
    providerFailoverContinuationFollowUp:
      'Nuovo tentativo del follow-up di continuazione senza il provider precedentemente fissato.',
  }),
  'ja-JP': makeDetailTerms({
    providerResolved: 'この応答では、選択された上流プロバイダーとモデルを使用します。',
    providerResolvedModelSwitch: 'この応答は、利用可能な上流モデルに切り替わりました。',
    providerResolvedFailoverSwitch:
      'この応答は、別の利用可能な上流プロバイダーまたはモデルに切り替わりました。',
    providerFailoverToolFollowUp:
      '以前に固定されたプロバイダーを使わずに、ツールの後続処理を再試行しています。',
    providerFailoverContinuationFollowUp:
      '以前に固定されたプロバイダーを使わずに、応答継続の後続処理を再試行しています。',
  }),
  'ko-KR': makeDetailTerms({
    providerResolved: '이 응답은 선택한 업스트림 공급자와 모델을 사용합니다.',
    providerResolvedModelSwitch: '이 응답은 사용 가능한 업스트림 모델로 전환되었습니다.',
    providerResolvedFailoverSwitch:
      '이 응답은 다른 사용 가능한 업스트림 공급자 또는 모델로 전환되었습니다.',
    providerFailoverToolFollowUp:
      '이전에 고정된 공급자 없이 도구 후속 요청을 다시 시도하는 중입니다.',
    providerFailoverContinuationFollowUp:
      '이전에 고정된 공급자 없이 이어쓰기 후속 요청을 다시 시도하는 중입니다.',
  }),
  'ml-IN': makeDetailTerms({
    providerResolved: 'ഈ പ്രതികരണം തെരഞ്ഞെടുത്ത അപ്‌സ്ട്രീം പ്രൊവൈഡറും മോഡലും ഉപയോഗിക്കും.',
    providerResolvedModelSwitch: 'ഈ പ്രതികരണം ലഭ്യമായ ഒരു അപ്‌സ്ട്രീം മോഡലിലേക്ക് മാറി.',
    providerResolvedFailoverSwitch:
      'ഈ പ്രതികരണം ലഭ്യമായ മറ്റൊരു അപ്‌സ്ട്രീം പ്രൊവൈഡറിലേക്കോ മോഡലിലേക്കോ മാറി.',
    providerFailoverToolFollowUp:
      'മുമ്പ് പിന്‍ ചെയ്ത പ്രൊവൈഡര്‍ ഇല്ലാതെ ടൂൾ ഫോളോ-അപ്പ് വീണ്ടും ശ്രമിക്കുന്നു.',
    providerFailoverContinuationFollowUp:
      'മുമ്പ് പിന്‍ ചെയ്ത പ്രൊവൈഡര്‍ ഇല്ലാതെ തുടർ ഫോളോ-അപ്പ് വീണ്ടും ശ്രമിക്കുന്നു.',
  }),
  'nb-NO': makeDetailTerms({
    providerResolved: 'Dette svaret vil bruke den valgte oppstrømsleverandøren og modellen.',
    providerResolvedModelSwitch: 'Dette svaret byttet til en tilgjengelig oppstrømsmodell.',
    providerResolvedFailoverSwitch:
      'Dette svaret byttet til en annen tilgjengelig oppstrømsleverandør eller modell.',
    providerFailoverToolFollowUp:
      'Prøver verktøyoppfølgingen på nytt uten den tidligere festede leverandøren.',
    providerFailoverContinuationFollowUp:
      'Prøver fortsettelsesoppfølgingen på nytt uten den tidligere festede leverandøren.',
  }),
  'nl-NL': makeDetailTerms({
    providerResolved:
      'Dit antwoord gebruikt de geselecteerde upstreamprovider en het geselecteerde model.',
    providerResolvedModelSwitch:
      'Dit antwoord is overgeschakeld naar een beschikbaar upstreammodel.',
    providerResolvedFailoverSwitch:
      'Dit antwoord is overgeschakeld naar een andere beschikbare upstreamprovider of een ander model.',
    providerFailoverToolFollowUp:
      'De toolopvolging wordt opnieuw geprobeerd zonder de eerder vastgezette provider.',
    providerFailoverContinuationFollowUp:
      'De vervolgopvolging wordt opnieuw geprobeerd zonder de eerder vastgezette provider.',
  }),
  'pl-PL': makeDetailTerms({
    providerResolved: 'Ta odpowiedź użyje wybranego dostawcy upstream i modelu.',
    providerResolvedModelSwitch: 'Ta odpowiedź przełączyła się na dostępny model upstream.',
    providerResolvedFailoverSwitch:
      'Ta odpowiedź przełączyła się na innego dostępnego dostawcę upstream lub model.',
    providerFailoverToolFollowUp:
      'Ponawianie dalszego wywołania narzędzia bez wcześniej przypiętego dostawcy.',
    providerFailoverContinuationFollowUp:
      'Ponawianie dalszej kontynuacji bez wcześniej przypiętego dostawcy.',
  }),
  'pt-BR': makeDetailTerms({
    providerResolved: 'Esta resposta usará o provedor upstream e o modelo selecionados.',
    providerResolvedModelSwitch: 'Esta resposta mudou para um modelo upstream disponível.',
    providerResolvedFailoverSwitch:
      'Esta resposta mudou para outro provedor ou modelo upstream disponível.',
    providerFailoverToolFollowUp:
      'Tentando novamente o acompanhamento da ferramenta sem o provedor fixado anteriormente.',
    providerFailoverContinuationFollowUp:
      'Tentando novamente o acompanhamento de continuação sem o provedor fixado anteriormente.',
  }),
  'pt-PT': makeDetailTerms({
    providerResolved: 'Esta resposta vai usar o fornecedor upstream e o modelo selecionados.',
    providerResolvedModelSwitch: 'Esta resposta mudou para um modelo upstream disponível.',
    providerResolvedFailoverSwitch:
      'Esta resposta mudou para outro fornecedor ou modelo upstream disponível.',
    providerFailoverToolFollowUp:
      'A repetir o seguimento da ferramenta sem o fornecedor anteriormente afixado.',
    providerFailoverContinuationFollowUp:
      'A repetir o seguimento da continuação sem o fornecedor anteriormente afixado.',
  }),
  'ro-RO': makeDetailTerms({
    providerResolved: 'Acest răspuns va folosi furnizorul upstream și modelul selectate.',
    providerResolvedModelSwitch: 'Acest răspuns a trecut la un model upstream disponibil.',
    providerResolvedFailoverSwitch:
      'Acest răspuns a trecut la un alt furnizor upstream disponibil sau la un alt model.',
    providerFailoverToolFollowUp:
      'Se reîncearcă urmărirea instrumentului fără furnizorul fixat anterior.',
    providerFailoverContinuationFollowUp:
      'Se reîncearcă urmărirea continuării fără furnizorul fixat anterior.',
  }),
  'ru-RU': makeDetailTerms({
    providerResolved:
      'Этот ответ будет использовать выбранного вышестоящего провайдера и модель.',
    providerResolvedModelSwitch:
      'Этот ответ переключился на доступную вышестоящую модель.',
    providerResolvedFailoverSwitch:
      'Этот ответ переключился на другого доступного вышестоящего провайдера или модель.',
    providerFailoverToolFollowUp:
      'Повторная попытка последующего вызова инструмента без ранее закреплённого провайдера.',
    providerFailoverContinuationFollowUp:
      'Повторная попытка продолжения без ранее закреплённого провайдера.',
  }),
  'sk-SK': makeDetailTerms({
    providerResolved: 'Táto odpoveď použije vybraného upstream poskytovateľa a model.',
    providerResolvedModelSwitch: 'Táto odpoveď sa prepla na dostupný upstream model.',
    providerResolvedFailoverSwitch:
      'Táto odpoveď sa prepla na iného dostupného upstream poskytovateľa alebo model.',
    providerFailoverToolFollowUp:
      'Opakuje sa následné volanie nástroja bez predtým pripnutého poskytovateľa.',
    providerFailoverContinuationFollowUp:
      'Opakuje sa následné pokračovanie bez predtým pripnutého poskytovateľa.',
  }),
  'sv-SE': makeDetailTerms({
    providerResolved:
      'Det här svaret kommer att använda den valda upstream-leverantören och modellen.',
    providerResolvedModelSwitch: 'Det här svaret bytte till en tillgänglig upstream-modell.',
    providerResolvedFailoverSwitch:
      'Det här svaret bytte till en annan tillgänglig upstream-leverantör eller modell.',
    providerFailoverToolFollowUp:
      'Försöker verktygsuppföljningen igen utan den tidigare fästa leverantören.',
    providerFailoverContinuationFollowUp:
      'Försöker fortsättningsuppföljningen igen utan den tidigare fästa leverantören.',
  }),
  'zh-CN': makeDetailTerms({
    providerResolved: '本次响应将使用所选的上游提供商和模型。',
    providerResolvedModelSwitch: '本次响应已切换到一个可用的上游模型。',
    providerResolvedFailoverSwitch: '本次响应已切换到另一个可用的上游提供商或模型。',
    providerFailoverToolFollowUp: '正在不使用上一个固定提供商重试这轮工具后续请求。',
    providerFailoverContinuationFollowUp: '正在不使用上一个固定提供商重试这轮续写后续请求。',
  }),
  'zh-TW': makeDetailTerms({
    providerResolved: '本次回應將使用所選的上游提供商和模型。',
    providerResolvedModelSwitch: '本次回應已切換到一個可用的上游模型。',
    providerResolvedFailoverSwitch: '本次回應已切換到另一個可用的上游提供商或模型。',
    providerFailoverToolFollowUp: '正在不使用上一個固定提供商重試這輪工具後續請求。',
    providerFailoverContinuationFollowUp: '正在不使用上一個固定提供商重試這輪續寫後續請求。',
  }),
}

function buildProcessTracePatch(
  terms: ProcessTraceTerms,
  voiceTerms: VoiceProcessTraceTerms,
  eventTerms: ProcessTraceEventTerms,
  detailTerms: ProcessTraceDetailTerms
): object {
  return {
    chat: {
      processTrace: {
        fields: {
          message: terms.message,
          provider: terms.provider,
          model: terms.model,
          attachments: terms.attachments,
          file: terms.file,
          attempt: terms.attempt,
          delay: terms.delay,
          format: voiceTerms.format,
          size: voiceTerms.size,
          duration: voiceTerms.duration,
          upload: voiceTerms.upload,
          transcript: voiceTerms.transcript,
          mode: voiceTerms.mode,
          conversation: voiceTerms.conversation,
        },
        summaryValues: {
          continuePreviousReply: terms.continuePreviousReply,
          resumePreviousRequest: terms.resumePreviousRequest,
          interruptCurrentReply: voiceTerms.interruptCurrentReply,
          sendNewRequest: voiceTerms.sendNewRequest,
        },
        events: {
          requestReady: terms.requestReady,
          requestSent: terms.requestSent,
          waitingForResponse: terms.waitingForResponse,
          recoveringResponse: terms.recoveringResponse,
          providerResolved: eventTerms.providerResolved,
          processing: eventTerms.processing,
        },
        details: {
          requestDispatched: terms.requestDispatched,
          waitingForResponse: terms.waitingForFirstVisibleOutput,
          providerResolved: detailTerms.providerResolved,
          providerResolvedModelSwitch: detailTerms.providerResolvedModelSwitch,
          providerResolvedFailoverSwitch: detailTerms.providerResolvedFailoverSwitch,
          providerFailoverToolFollowUp: detailTerms.providerFailoverToolFollowUp,
          providerFailoverContinuationFollowUp:
            detailTerms.providerFailoverContinuationFollowUp,
          recoveryStage1: terms.recoveryStage1,
          recoveryStage2: terms.recoveryStage2,
          voiceInterruptDispatched: voiceTerms.voiceInterruptDispatched,
          voiceRequestDispatched: voiceTerms.voiceRequestDispatched,
          voiceWaitingForResponse: voiceTerms.voiceWaitingForResponse,
          bargeIn: voiceTerms.bargeIn,
          transcriptionFailed: voiceTerms.transcriptionFailed,
        },
      },
    },
  }
}

const processTraceLocaleBackfills = Object.fromEntries(
  Object.entries(processTraceTerms).map(([localeKey, terms]) => {
    const typedLocaleKey = localeKey as LocaleKey
    return [
      typedLocaleKey,
      buildProcessTracePatch(
        terms,
        voiceProcessTraceTerms[typedLocaleKey],
        processTraceEventTerms[typedLocaleKey],
        processTraceDetailTerms[typedLocaleKey]
      ),
    ]
  })
) as Partial<Record<LocaleKey, object>>

export default processTraceLocaleBackfills
