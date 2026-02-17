![](../../docs/assets/bannerX.png)

<p align="center">
  <a href="../../README.md">English</a> |
  <a href="../ca_ES/README.md">Català</a> |
  <a href="../cs_CZ/README.md">Čeština</a> |
  <a href="../da_DK/README.md">Dansk</a> |
  <a href="../de_DE/README.md">Deutsch</a> |
  <a href="../el_GR/README.md">Ελληνικά</a> |
  <a href="../en_GB/README.md">English (UK)</a> |
  <a href="../es_ES/README.md">Español</a> |
  <a href="../fr_FR/README.md">Français</a> |
  <strong>Gaeilge</strong> |
  <a href="../hr_HR/README.md">Hrvatski</a> |
  <a href="../hu_HU/README.md">Magyar</a> |
  <a href="../it_IT/README.md">Italiano</a> |
  <a href="../ja_JP/README.md">日本語</a> |
  <a href="../ko_KR/README.md">한국어</a> |
  <a href="../ml_IN/README.md">മലയാളം</a> |
  <a href="../nb_NO/README.md">Norsk Bokmål</a> |
  <a href="../nl_NL/README.md">Nederlands</a> |
  <a href="../pl_PL/README.md">Polski</a> |
  <a href="../pt_BR/README.md">Português (BR)</a> |
  <a href="../pt_PT/README.md">Português (PT)</a> |
  <a href="../ro_RO/README.md">Română</a> |
  <a href="../ru_RU/README.md">Русский</a> |
  <a href="../sk_SK/README.md">Slovenčina</a> |
  <a href="../sv_SE/README.md">Svenska</a> |
  <a href="../zh_CN/README.md">简体中文</a> |
  <a href="../zh_TW/README.md">繁體中文</a>
</p>

<p align="center">
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Blue/ci.yml?branch=main&style=for-the-badge" alt="Stádas CI"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Blue?include_prereleases&style=for-the-badge" alt="Eisiúint GitHub"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="Ceadúnas MIT"></a>
</p>

<p align="center">
  <a href="https://discord.gg/SrCYvumF"><img src="../../docs/assets/discord.png" alt="Discord" height="128" /></a>&nbsp;&nbsp;
  <a href="https://www.facebook.com/zimaboard/"><img src="../../docs/assets/facebook.png" alt="Facebook" height="128" /></a>&nbsp;&nbsp;
  <a href="https://x.com/ZimaSpace"><img src="../../docs/assets/x.png" alt="X" height="128" /></a>&nbsp;&nbsp;
  <img src="../../docs/assets/wechat.png" height="128"/>
</p>

## Réamhrá

Spreagtha ag Clawdbot, creidimid go mbeidh **todhchaí** na ríomhaireachta pearsanta **múnlaithe ag gníomhairí AI éagsúla, áitiúil-ar-dtús** atá ag rith ar an imeall.

Is é **ZimaOS Blue ár bhfreagra** — **am rite gníomhaire agus uirlisí foinse oscailte, in-iniúchta agus réidh le haghaidh táirgthe** a ligeann duit gníomhairí príobháideacha, féin-óstáilte a sheoladh gan aon fhrithchuimilt.

Tógtha d'fhorbróirí dána ar mhaith leo **a gcuid gníomhairí féin a chruthú nó a cheardú**, tá Blue **innealtóirithe le haghaidh feidhmíochta**: scríofa i **Go**, le lorg cuimhne chomh híseal le 10 MB. Ritheann sé ar **aon x86, Raspberry Pi, Windows, macOS** — áit ar bith a nascann tú cumhacht.

![](../../docs/assets/features.png)

## Buaicphointí

### Dearadh Áitiúil-ar-Dtús & Rochtain Uathoibríoch ar Shamhlacha

Téigh níos faide: soláthraíonn sé tacaíocht dhúchasach do **20+ ardán IM**, comhéadain **tiomáinte ag guth** le haghaidh comhrá nádúrtha, comhthéacs-fheasach, **athrú samhla gan aon chumraíocht** le scanadh IDE, agus pearsantachtaí SOUL-sraitheacha.

### Tapa, Éadrom

Tiomsaithe go dúchasach i Go — gan ateangaire, gan VM, gan forchostais. Ritheann sé go ciúin ar gach rud ó fhreastalaithe go do ghléasanna deisce.

| Méadrach | ZimaOS Blue (Go) | OpenClaw (Node + dist) |
|--------|-------------------|------------------------|
| `--help` fuar / te | **0.18 s / < 0.01 s** | 3.31 s / ~1.11 s |
| `status` am rite (is fearr as 3) | **< 0.01 s** | 5.98 s |
| `--help` buaic RSS | **~10 MB** | ~394 MB |
| `status` buaic RSS | **~15 MB** | ~1.52 GB |
| Spleáchais am rite | **Dada** | Node.js 18+ |

> Tagarmharcáilte ar macOS arm64, an t-óstach céanna, is fearr as 3 rith. Feabhra 2026.

### Go Glan, Aon Ghléas

100% Go, dénártha statach. **Tras-tiomsaíonn sé go 5 sprioc** as an mbosca (![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png) amd64/arm64, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png) amd64/arm64, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png) amd64). Gan am rite Node, gan Python, gan coimeádáin ag teastáil. Cuir ar NAS é, ar Raspberry Pi, ar sheanlíontóir x86, nó ar Mac — ritheann sé díreach. **Ansin cuir do UI, loighic, agus scileanna gníomhaire féin air** — bonn cód amháin, gach ardán.

### Slándáil & Rialachas

Seachfhreastalaí API ionsuite le cosaint i ndoimhneacht:
- **Forghníomhú Gaineamhbhosca** – Ritheann gach glao uirlise i dtimpeallachtaí leithlisithe.
- **Cosaint in aghaidh Instealladh Leid** – 7+ straitéis idirghabhála ionsuite.
- **Iniúchadh Seisiúin** – Monatóireacht iomlán seisiúin, gach idirghníomhaíocht inrianaithe.
- **RBAC & WebAuthn** – Rialú rochtana mionsonraithe le fíordheimhniú gan pasfhocal.

## Cén Fáth Blue

Creidimid go nglacann **ríomhaireacht phearsanta na chéad ghlúine eile** le LLManna — ach fanann gníomhairí **inrialaithe, in-iniúchta** mar bhunchloch do dhaoine aonair agus d'fhoirne araon. **Soláthraíonn Blue**:
- **Croí Cuimsitheach** – Bainistíocht ardleibhéil samhlacha, comhtháthú IM, pearsana feabhsaithe, agus comhéadain teanga nádúrtha atá tiúnáilte le haghaidh idirghníomhaíochtaí laethúla (cluasáin, guth, spéaclaí cliste).
- **Áitiúil-ar-Dtús, Ultra-Éadrom, Tras-Ghléas** – Níl crua-earraí ardleibhéil ag teastáil. Ritheann sé ar aon rud is féidir ríomhaireacht a dhéanamh.
- **Slán & In-iniúchta** – Iniúchadh seisiúin, gaineamhbhoscú, rialtáin ceadanna, agus seachfhreastalaí API ionsuite a fheidhmíonn mar bhalla dóiteáin ciseal feidhmchláir — tá gach beart isteach/amach le feiceáil.

Laghdaímid an cód réamhdhéanta ionas go **ndíríonn tú ar an méid is tábhachtaí**. Ag fanacht dílis d'**fhealsúnacht deartha ZimaOS**, soláthraíonn Blue:
- **Ó Nialas go hAon le Clic Amháin** – Imscaradh láithreach, gan cumraíocht chasta.
- **Fréamhshamhlú Tapa** – Cruthaigh nó ceardaigh uirlisí, idirghníomhaíochtaí, agus pacáistí feidhmchlár atá sainiúil do chásanna.
- **Réidh don Domhan** – **Tá an domhan mór**, agus ní Béarla an réamhshocrú. **20+ teanga, dúchasach**, gan bacainní.
- **Éiceachóras Samhlacha Oscailte** – Gan glasáil díoltóra. Tabhair do shamhlacha féin leat.

![](../../docs/assets/design_principle.png)

## Tús Tapa

### Rogha 1: Íoslódáil an Aip Deisce (macOS & Windows)

Faigh an feidhmchlár dúchasach — gan spleáchais, gan tiomsú.

- **macOS**: [Íoslódáil DMG](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)
- **Windows**: [Íoslódáil Suiteálaí](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)

### Rogha 2: Script Suiteála

**macOS / Linux**
```bash
curl -fsSL https://ota.zimaos.com/blue | sh
```

**Windows (PowerShell)**
```powershell
irm https://ota.zimaos.com/blue/windows | iex
```

### Rogha 3: Tóg ón bhFoinse

```bash
git clone https://github.com/IceWhaleTech/ZimaOS-Blue.git
cd ZimaOS-Blue
```

**macOS / Linux**
```bash
sh dev.sh
```

**Windows (PowerShell)**
```powershell
.\dev.bat
```

## Forbhreathnú Ailtireachta

![](../../docs/assets/architecture.png)

### Sreabhadh Sonraí

**Iarratas Comhrá (Conair The Seachfhreastalaí)**
```
Client [Proxy API Key] → Auth Gate → Prompt Guard → Context Pruner (optional)
  → Provider Pool (route:auto/cloud/local) → CC Cache (L1→L2) check
  → Upstream LLM → Response → Cache Store → Metrics Writer → Client (SSE stream)
```

**Sreabhadh Teachtaireachtaí Cainéil**
```
Telegram/Discord/... → Channel Manager → AutoReply check
  → Chat Handler → LLM → Humanizer (MD→text) → Channel → User
```

**Píblíne Gutha**
```
WebSocket audio → STT (Whisper) → LLM Processing → TTS (eSpeak/Edge) → WebSocket audio
```

### Mapa Pacáistí (`server/internal/`)

| Ciseal | Pacáistí |
|-------|----------|
| Gateway | bootstrap, server, gateway |
| Proxy | proxy, connection, streaming, resilience |
| Provider | providerpool, providers, llm |
| Pruner | pruner (detector, segmenter, bm25, pipeline, cache) |
| Agent | context, tools, personality, humanizer |
| Memory | memory, embedding, kvstore |
| Channel | channel, autoreply, i18n |
| Security | security, auth, permission, rbac, mfa, password, oidc, extauth, sandbox, promptguard, audit |
| Voice | voice, tts, stt, speech |
| Observe | metrics, heartbeat, companion, profiling, leakdetect |
| Plugin | plugin, skill, skillstore |
| Integrate | browser, homeassistant, cron, workflow, formfiller, tunnel, crawler |
| Scheduler | scheduler, worker, workerpool, pool |
| Core | lifecycle, config, logger, database, cache, ratelimit, retry, timeutil, sync |
| System | sysinfo, cgroup, iotask, watcher, resources, backup, update |
| Multi-tenant | tenant, user, session, preview |

## Conas É a Úsáid

![](../../docs/assets/handcraft.png)

## Amlíne Garspriocanna

![](../../docs/assets/timeline.png)

| Leagan | Fócas | Príomhluach | Stádas |
|---------|-------|-----------|--------|
| v0.1 | Croí Am Rite Go | Eithne cobhsaí, ag rith 24u | Déanta |
| v0.2 | Cumais Chroí | Íosta inúsáidte, comhtháthú LLM | Déanta |
| v0.3 | Comhtháthú NAS | NAS dúchasach, tacaíocht systemd | Déanta |
| v0.4 | Córas Breiseán | Insínte, bunúsacha slándála | Déanta |
| v0.5 | Bonnlíne Táirge | Réidh le haghaidh táirgthe, doiciméadú | Déanta |
| v0.6 | Cainéil Teachtaireachtaí | Tacaíocht ilchainéil | Déanta |
| v0.7 | Slándáil | OIDC, MFA, iniúchadh | Déanta |
| v0.8 | Feidhmíocht | Optamú, taiscéadú, tagarmharcanna | Déanta |
| v0.9 | Éiceachóras | Ilthionónta, uathoibriú brabhsálaí, guth | Déanta |
| v0.10.0 | Buntáil CLI | Buntáil CC CLI, brath, uath-nuashonrú | Déanta |
| v0.10.1 | Monatóireacht Méadracha | Staitisticí API, rianú comharthaí, TTFT | Déanta |
| v0.10.2 | Iontaofacht CLI | Saolré próisis, téarnamh earráidí | Déanta |
| v0.10.3 | Comhtháthú CLI | Draoi socraithe, uathbhrath soláthróra | Déanta |
| v0.10.4 | Pacáistiú Tauri | Aip deisce, tráidire córais | Déanta |
| v0.10.5 | Seachfhreastalaí API | Roghnú bealaigh, garda leid, staitisticí úsáide | Déanta |
| v0.10.6 | Linn Soláthróirí | Ródú ilsoláthróra, seiceáil sláinte, teip thairis | Déanta |
| v0.10.7 | Mód Réamhamhairc | Rochtain neamhfhíordheimhnithe, geataíocht gnéithe | Déanta |
| v0.10.8 | Siopa Scileanna | Bonneagar siopa scileanna, bailíochtú cainéil | Déanta |
| v0.10.9–10 | Bainistíocht Úsáideoirí | Fo-úsáideoirí, ceadanna ar leibhéal leathanaigh | Déanta |
| v0.10.13–14 | Slándáil & Scileanna | Leathanach slándála, athdearadh siopa scileanna | Déanta |
| v0.10.15 | Feabhsuithe Comhrá | UX comhrá, píblíne teachtaireachtaí | Déanta |
| v0.10.16 | Modúl Cainte | Sherpa TTS/ASR, eSpeak, athrú soláthróra | Déanta |
| v0.10.17 | Rochtain Chianda | Tollán Ngrok, Cloudflare, teastais ACME | Déanta |
| v0.10.18–20 | Sprint Feidhmíochta | Feidhmíocht tosaithe/comhrá, taisce comhthéacs | Déanta |
| v0.10.21–22 | Leid & DingTalk | Leid córais, cainéal DingTalk | Déanta |
| v0.10.23 | Nuashonrú OTA | Córas nuashonraithe OTA | Déanta |
| v0.10.24 | Uasghrádú Cainéil | 10 gcainéal uasghrádaithe ó stuib | Déanta |
| v0.10.25 | CC Cache | Taisce dhá leibhéal (L1 cuimhne + L2 diosca) | Déanta |
| v0.10.26 | Humanizer | Píblíne daonnaithe freagraí | Déanta |
| v0.10.27 | Context Pruner | 54% coigilt comharthaí ar chód (SWE-bench oifigiúil), 46–47% ar dhoiciméid ghinearálta (IR áitiúil), scóráil BM25, deighilt | Déanta |
| v0.10.28 | Seirbhís Cuimhne | Cuardach forásach, cúl-taobh dé-scríofa | Déanta |

## Pobal & Tacaíocht

- **Saincheisteanna**: [Cuir fabhtanna agus iarratais ar ghnéithe isteach anseo le do thoil](https://github.com/IceWhaleTech/ZimaOS-Blue/issues)
- **Plé**: [Discord](https://discord.gg/SrCYvumF)
- **Lean sinn** ar [GitHub](https://github.com/IceWhaleTech)

## Ceadúnas

Tá an tionscadal seo ceadúnaithe faoin gCeadúnas MIT - féach ar an gcomhad [LICENSE](../../LICENSE) le haghaidh sonraí. Creidimid i bhfoinse oscailte agus i dtabhairt ar ais don phobal.

## Rannchuiditheoirí

<p align="center">
  Déanta le ❤️ ag <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
