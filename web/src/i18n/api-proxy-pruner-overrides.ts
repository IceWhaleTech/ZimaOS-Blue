type PrunerOverridePack = {
  prunerTitle: string
  prunerDesc: string
}

function buildLocaleOverrides(pack: PrunerOverridePack) {
  return {
    apiProxy: {
      prunerTitle: pack.prunerTitle,
      prunerDesc: pack.prunerDesc,
    },
  }
}

export default {
  'ca-ES': buildLocaleOverrides({
    prunerTitle: 'Podador global de context',
    prunerDesc:
      'Controla el podador del proxy API per a les sol·licituds /v1 que passen pel proxy. El xat de Blue encara pot ometre la poda en cada sol·licitud quan la pressió de context és baixa.',
  }),
  'cs-CZ': buildLocaleOverrides({
    prunerTitle: 'Globální ořezávač kontextu',
    prunerDesc:
      'Řídí ořezávač API proxy pro proxované požadavky /v1. Chat Blue může stále přeskočit ořezání u jednotlivých požadavků, když je tlak na kontext nízký.',
  }),
  'da-DK': buildLocaleOverrides({
    prunerTitle: 'Global kontekstbeskærer',
    prunerDesc:
      'Styrer API-proxyens kontekstbeskærer for proxiede /v1-forespørgsler. Blue-chat kan stadig springe beskæring over pr. forespørgsel, når kontekstpresset er lavt.',
  }),
  'de-DE': buildLocaleOverrides({
    prunerTitle: 'Globaler Kontext-Pruner',
    prunerDesc:
      'Steuert den API-Proxy-Pruner für weitergeleitete /v1-Anfragen. Der Blue-Chat kann das Kürzen bei einzelnen Anfragen weiterhin überspringen, wenn der Kontextdruck gering ist.',
  }),
  'el-GR': buildLocaleOverrides({
    prunerTitle: 'Καθολικός περικοπτής συμφραζομένων',
    prunerDesc:
      'Ελέγχει τον περικοπτή του API proxy για τα proxied αιτήματα /v1. Η συνομιλία Blue μπορεί ακόμη να παραλείπει την περικοπή ανά αίτημα όταν η πίεση συμφραζομένων είναι χαμηλή.',
  }),
  'en-GB': buildLocaleOverrides({
    prunerTitle: 'Global Context Pruner',
    prunerDesc:
      'Controls the API proxy pruner for proxied /v1 requests. Blue chat may still skip pruning per request when context pressure is low.',
  }),
  'en-US': buildLocaleOverrides({
    prunerTitle: 'Global Context Pruner',
    prunerDesc:
      'Controls the API proxy pruner for proxied /v1 requests. Blue chat may still skip pruning per request when context pressure is low.',
  }),
  'es-ES': buildLocaleOverrides({
    prunerTitle: 'Podador global de contexto',
    prunerDesc:
      'Controla el podador del proxy API para las solicitudes /v1 enviadas a través del proxy. El chat de Blue aún puede omitir la poda en solicitudes individuales cuando la presión de contexto es baja.',
  }),
  'fr-FR': buildLocaleOverrides({
    prunerTitle: 'Élagueur global de contexte',
    prunerDesc:
      'Contrôle l’élagueur du proxy API pour les requêtes /v1 acheminées via le proxy. Le chat Blue peut encore ignorer l’élagage requête par requête lorsque la pression de contexte est faible.',
  }),
  'ga-IE': buildLocaleOverrides({
    prunerTitle: 'Gearrthóir comhthéacs domhanda',
    prunerDesc:
      'Rialaíonn sé gearrthóir seachfhreastalaí an API do na hiarratais /v1 a théann tríd an seachfhreastalaí. Féadfaidh comhrá Blue bearradh a scipeáil fós ar bhonn gach iarratais nuair atá brú an chomhthéacs íseal.',
  }),
  'hr-HR': buildLocaleOverrides({
    prunerTitle: 'Globalni rezač konteksta',
    prunerDesc:
      'Kontrolira API proxy rezač za proxirane /v1 zahtjeve. Blue chat i dalje može preskočiti rezanje po pojedinom zahtjevu kada je pritisak konteksta nizak.',
  }),
  'hu-HU': buildLocaleOverrides({
    prunerTitle: 'Globális kontextusmetsző',
    prunerDesc:
      'Az API proxy kontextusmetszőjét vezérli a proxizott /v1 kérelmekhez. A Blue chat továbbra is kihagyhatja a metszést kérésenként, ha a kontextusterhelés alacsony.',
  }),
  'it-IT': buildLocaleOverrides({
    prunerTitle: 'Potatore globale del contesto',
    prunerDesc:
      'Controlla il potatore del proxy API per le richieste /v1 inoltrate tramite proxy. La chat di Blue può comunque saltare la potatura per singola richiesta quando la pressione del contesto è bassa.',
  }),
  'ja-JP': buildLocaleOverrides({
    prunerTitle: 'グローバルコンテキストプルーナー',
    prunerDesc:
      'プロキシされた /v1 リクエストに対する API プロキシのプルーナーを制御します。コンテキスト負荷が低い場合、Blue チャットはリクエストごとにプルーニングをスキップすることがあります。',
  }),
  'ko-KR': buildLocaleOverrides({
    prunerTitle: '전역 컨텍스트 프루너',
    prunerDesc:
      '프록시된 /v1 요청에 적용되는 API 프록시 프루너를 제어합니다. 컨텍스트 압박이 낮으면 Blue 채팅은 요청별 프루닝을 건너뛸 수 있습니다.',
  }),
  'ml-IN': buildLocaleOverrides({
    prunerTitle: 'ഗ്ലോബൽ സന്ദർഭ പ്രൂണർ',
    prunerDesc:
      'പ്രോക്സി ചെയ്യുന്ന /v1 അഭ്യർത്ഥനകൾക്കായുള്ള API പ്രോക്സി പ്രൂണറെ നിയന്ത്രിക്കുന്നു. സന്ദർഭ സമ്മർദ്ദം കുറഞ്ഞിരിക്കുമ്പോൾ Blue ചാറ്റ് ഓരോ അഭ്യർത്ഥനയിലും പ്രൂണിംഗ് ഒഴിവാക്കാനിടയുണ്ട്.',
  }),
  'nb-NO': buildLocaleOverrides({
    prunerTitle: 'Global kontekstbeskjærer',
    prunerDesc:
      'Styrer API-proxyens kontekstbeskjærer for proxiede /v1-forespørsler. Blue-chat kan fortsatt hoppe over beskjæring per forespørsel når kontekstpresset er lavt.',
  }),
  'nl-NL': buildLocaleOverrides({
    prunerTitle: 'Globale contextinkorter',
    prunerDesc:
      'Beheert de API-proxy-inkorter voor geproxiede /v1-verzoeken. Blue-chat kan inkorten per verzoek nog steeds overslaan wanneer de contextdruk laag is.',
  }),
  'pl-PL': buildLocaleOverrides({
    prunerTitle: 'Globalny przycinacz kontekstu',
    prunerDesc:
      'Steruje przycinaczem proxy API dla żądań /v1 przechodzących przez proxy. Czat Blue może nadal pomijać przycinanie dla pojedynczych żądań, gdy presja kontekstu jest niska.',
  }),
  'pt-BR': buildLocaleOverrides({
    prunerTitle: 'Podador global de contexto',
    prunerDesc:
      'Controla o podador do proxy de API para requisições /v1 encaminhadas pelo proxy. O chat do Blue ainda pode ignorar a poda por requisição quando a pressão de contexto estiver baixa.',
  }),
  'pt-PT': buildLocaleOverrides({
    prunerTitle: 'Podador global de contexto',
    prunerDesc:
      'Controla o podador do proxy de API para pedidos /v1 encaminhados pelo proxy. O chat do Blue pode ainda ignorar a poda por pedido quando a pressão de contexto for baixa.',
  }),
  'ro-RO': buildLocaleOverrides({
    prunerTitle: 'Pruner global de context',
    prunerDesc:
      'Controlează prunerul proxy API pentru cererile /v1 proxiate. Chatul Blue poate totuși să sară peste reducerea contextului pentru fiecare cerere atunci când presiunea de context este scăzută.',
  }),
  'ru-RU': buildLocaleOverrides({
    prunerTitle: 'Глобальный сокращатель контекста',
    prunerDesc:
      'Управляет сокращателем API-прокси для проксируемых запросов /v1. Чат Blue все еще может пропускать сокращение для отдельных запросов, когда нагрузка на контекст низкая.',
  }),
  'sk-SK': buildLocaleOverrides({
    prunerTitle: 'Globálny orezávač kontextu',
    prunerDesc:
      'Ovláda orezávač API proxy pre proxované požiadavky /v1. Chat Blue môže stále preskočiť orezanie pri jednotlivých požiadavkách, keď je tlak na kontext nízky.',
  }),
  'sv-SE': buildLocaleOverrides({
    prunerTitle: 'Global kontextbeskärare',
    prunerDesc:
      'Styr API-proxyns kontextbeskärare för proxade /v1-förfrågningar. Blue-chatten kan fortfarande hoppa över beskärning per förfrågan när kontexttrycket är lågt.',
  }),
  'zh-CN': buildLocaleOverrides({
    prunerTitle: '全局上下文裁剪器',
    prunerDesc:
      '控制 API Proxy 对代理 /v1 请求使用的全局 pruner。Blue 对话在上下文压力较低时，仍可能按单次请求跳过裁剪。',
  }),
  'zh-TW': buildLocaleOverrides({
    prunerTitle: '全域情境裁剪器',
    prunerDesc:
      '控制 API Proxy 對代理 /v1 請求使用的全域 pruner。當情境壓力較低時，Blue 對話仍可能依單次請求略過裁剪。',
  }),
}
