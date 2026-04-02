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
  <a href="../ga_IE/README.md">Gaeilge</a> |
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
  <strong>Română</strong> |
  <a href="../ru_RU/README.md">Русский</a> |
  <a href="../sk_SK/README.md">Slovenčina</a> |
  <a href="../sv_SE/README.md">Svenska</a> |
  <a href="../zh_CN/README.md">简体中文</a> |
  <a href="../zh_TW/README.md">繁體中文</a>
</p>

<p align="center">
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Blue/ci.yml?branch=main&style=for-the-badge" alt="Stare CI"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Blue?include_prereleases&style=for-the-badge" alt="Versiune GitHub"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="Licență MIT"></a>
</p>

<p align="center">
  <a href="https://discord.gg/b3AgFDxe9v"><img src="../../docs/assets/discord.png" alt="Discord" height="128" /></a>&nbsp;&nbsp;
  <a href="https://www.facebook.com/zimaboard/"><img src="../../docs/assets/facebook.png" alt="Facebook" height="128" /></a>&nbsp;&nbsp;
  <a href="https://x.com/ZimaSpace"><img src="../../docs/assets/x.png" alt="X" height="128" /></a>
</p>

## Introducere

Inspirați de Clawdbot, credem că **viitorul** informaticii personale va fi **modelat de agenți AI diversificați, cu prioritate locală**, care rulează la periferia rețelei.

**ZimaOS Blue este răspunsul nostru** — un **runtime și set de instrumente pentru agenți, complet open-source, auditabil și pregătit pentru producție**, care vă permite să implementați agenți privați, auto-găzduiți, fără nicio fricțiune.

Construit pentru dezvoltatori curajoși care doresc să-și **creeze propriii agenți liber sau manual**, Blue este **proiectat pentru performanță**: scris în **Go**, cu un consum de memorie de doar 10 MB. Rulează pe **orice x86, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS** — oriunde conectați la priză.

![](../../docs/assets/features.png)

## Caracteristici principale

### Design cu prioritate locală și acces automat la modele

Mergem mai departe: suport nativ pentru **peste 20 de platforme IM**, interfețe **controlate vocal** pentru dialog natural și conștient de context, **comutare automată a modelelor** cu scanare IDE și personalități bazate pe SOUL.

<p align="center">
  <img src="../../docs/assets/channels.png" alt="Supported Channels" />
</p>

### Rapid și ușor

Compilat nativ în Go — fără interpretor, fără VM, fără overhead. Rulează silențios pe orice, de la servere la dispozitive desktop.

| Metrică | ZimaOS Blue (Go) | Reference Agent (Node + dist) |
|---------|-------------------|------------------------|
| `help` la rece / la cald | **0.18 s / < 0.01 s** | 3.31 s / ~1.11 s |
| `status` timp de execuție (cel mai bun din 3) | **< 0.01 s** | 5.98 s |
| `help` RSS maxim | **~10 MB** | ~394 MB |
| `status` RSS maxim | **~15 MB** | ~1.52 GB |
| Dependențe de rulare | **Niciuna** | Node.js 18+ |

> Testat pe macOS arm64, aceeași gazdă, cel mai bun din 3 rulări. Feb 2026.

### Go pur, orice dispozitiv

100% Go, binar static. **Compilare încrucișată pentru 5 ținte** din start (![linux](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png) amd64/arm64, ![macOS](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png) amd64/arm64, ![windows](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png) amd64). Fără runtime Node, fără Python, fără containere necesare. Puneți-l pe un NAS, un ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, un router x86 vechi sau un ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)Mac — pur și simplu funcționează. **Apoi adăugați propria interfață, logică și abilități de agent** — o singură bază de cod, orice platformă.

### Securitate și guvernanță

Proxy API sidecar integrat cu apărare în profunzime:
- **Execuție în sandbox** – Toate apelurile de instrumente rulează în medii izolate.
- **Apărare contra injecției de prompt** – Peste 7 strategii de interceptare integrate.
- **Auditarea sesiunilor** – Monitorizare completă a sesiunilor, fiecare interacțiune trasabilă.
- **RBAC și WebAuthn** – Control granular al accesului cu autentificare fără parolă.

## De ce Blue

Credem că **informatica personală de nouă generație** îmbrățișează LLM-urile — dar agenții **controlabili și auditabili** rămân fundația atât pentru indivizi, cât și pentru echipe. **Blue oferă**:
- **Nucleu complet** – Gestionare avansată a modelelor, integrare IM, personalitate îmbunătățită și interfețe în limbaj natural optimizate pentru interacțiuni zilnice (căști, voce, ochelari inteligenți).
- **Prioritate locală, ultra-ușor, multi-dispozitiv** – Nu necesită hardware de ultimă generație. Rulează pe orice poate calcula.
- **Securizat și auditabil** – Auditarea sesiunilor, sandboxing, controlul permisiunilor și un proxy API integrat care acționează ca un firewall la nivel de aplicație — fiecare octet de intrare/ieșire este vizibil.

![](../../docs/assets/design_principle.png)

Minimizăm codul repetitiv pentru ca dvs. să vă **concentrați pe ceea ce contează**. Fideli <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> **filozofiei de design a ZimaOS**, Blue oferă:
- **De la zero la unu cu un singur clic** – Implementare instantanee, fără configurare complexă.
- **Prototipare rapidă** – Creați liber sau manual instrumente, interacțiuni și pachete de aplicații specifice scenariului.
- **Pregătit global** – **Lumea este mare** și nu vorbește implicit engleză. **Peste 20 de limbi, nativ**, fără bariere.
- **Ecosistem deschis de modele** – Fără dependență de furnizor. Aduceți propriile modele.

<details>
<summary>
<p align="center">
  <img src="../../docs/assets/providers.png" alt="Supported Providers" />
</p>
</summary>

| Furnizor | Modele | Tip |
|----------|--------|-----|
| OpenAI | GPT-4o, GPT-4, o1, o3 | Cloud |
| Anthropic | Claude 4.5, Claude 4 | Cloud |
| Google | Gemini 2.5, Gemini 2.0 | Cloud |
| Ollama | Llama, Qwen, Gemma, Phi etc. | Local |
| DeepSeek | DeepSeek-V3, DeepSeek-R1 | Cloud |
| Grok | Grok-3, Grok-3-mini | Cloud |
| Qwen | Qwen-Max, Qwen-Plus, Qwen-Turbo | Cloud |
| GLM | GLM-4, GLM-4-Flash | Cloud |
| Moonshot | Moonshot-v1 | Cloud |
| MiniMax | abab6.5, abab5.5 | Cloud |
| Venice | Llama, Mistral (confidențialitate prioritară) | Cloud |
| AWS Bedrock | Claude, Llama, Titan | Cloud |
| Azure | Modele OpenAI prin Azure | Cloud |
| OpenRouter | 100+ modele agregate | Cloud |
| AIHubMix | Agregator multi-furnizor | Cloud |
| Codex | OpenAI Codex | Cloud |
| SiliconFlow | DeepSeek, Qwen, Llama via SiliconFlow | Cloud |
| Personalizat | Orice API compatibil OpenAI / Anthropic / Gemini | Cloud / Local |

</details>

### IDE-uri suportate

<p align="center">
  <img src="../../docs/assets/ides.png" alt="Supported IDEs" />
</p>

## Pornire rapidă

### Opțiunea 1: Descărcați aplicația desktop

Obțineți aplicația nativă — fără dependențe, fără compilare.

- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)**macOS**: [Descărcați DMG](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)**Windows**: [Descărcați programul de instalare](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)

### Opțiunea 2: Script de instalare

![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
curl -fsSL https://ota.zimaos.com/blue | sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
irm https://ota.zimaos.com/blue/windows | iex
```

### Opțiunea 3: Compilare din sursă

```bash
git clone https://github.com/IceWhaleTech/ZimaOS-Blue.git
cd ZimaOS-Blue
```

![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
sh build.sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
.\build.bat
```

## Prezentare generală a arhitecturii

![](../../docs/assets/architecture.png)

### Fluxul de date

**Cerere de chat (calea rapidă a proxy-ului)**
```
Client [Proxy API Key] → Auth Gate → Prompt Guard → Context Pruner (optional)
  → Provider Pool (route:auto/cloud/local) → CC Cache (L1→L2) check
  → Upstream LLM → Response → Cache Store → Metrics Writer → Client (SSE stream)
```

**Fluxul mesajelor pe canale**
```
Telegram/Discord/... → Channel Manager → AutoReply check
  → Chat Handler → LLM → Humanizer (MD→text) → Channel → User
```

**Pipeline vocal**
```
WebSocket audio → STT (Whisper) → LLM Processing → TTS (eSpeak/Edge) → WebSocket audio
```


### Harta pachetelor (`server/internal/`)

| Strat | Pachete |
|-------|---------|
| Gateway | bootstrap, server, gateway |
| Proxy | proxy, connection, streaming, resilience |
| Furnizor | providerpool, providers, llm |
| Pruner | pruner (detector, segmenter, bm25, pipeline, cache) |
| Agent | context, tools, personality, humanizer |
| Memorie | memory, embedding, kvstore |
| Canal | channel, autoreply, i18n |
| Securitate | security, auth, permission, rbac, mfa, password, oidc, extauth, sandbox, promptguard, audit |
| Voce | voice, tts, stt, speech |
| Observare | metrics, companion, profiling, leakdetect |
| Plugin | plugin, skill, skillstore |
| Integrare | browser, cron, workflow, formfiller, tunnel, crawler |
| Planificator | scheduler, worker, workerpool, pool |
| Nucleu | lifecycle, config, logger, database, cache, ratelimit, retry, timeutil, sync |
| Sistem | sysinfo, cgroup, iotask, watcher, resources, backup, update |
| Multi-tenant | tenant, user, session, preview |

## Mod de utilizare

![](../../docs/assets/handcraft.png)

## Cronologia etapelor

![](../../docs/assets/timeline.png)

| Versiune | Focus | Valoare cheie | Status |
|----------|-------|---------------|--------|
| v0.1 | Nucleu runtime Go | Kernel stabil, funcționare 24h | Done |
| v0.2 | Capabilități de bază | Minim utilizabil, integrare LLM | Done |
| v0.3 | Integrare NAS | NAS nativ, suport systemd | Done |
| v0.4 | Sistem de pluginuri | Extensibil, baze de securitate | Done |
| v0.5 | Bază de produs | Pregătit pentru producție, documentație | Done |
| v0.6 | Canale de mesagerie | Suport multi-canal | Done |
| v0.7 | Securitate | OIDC, MFA, audit | Done |
| v0.8 | Performanță | Optimizare, caching, benchmark-uri | Done |
| v0.9 | Ecosistem | Multi-tenant, automatizare browser, voce | Done |
| v0.10.0 | Împachetare CLI | Împachetare CC CLI, detectare, auto-actualizare | Done |
| v0.10.1 | Monitorizare metrici | Statistici API, urmărire tokenuri, TTFT | Done |
| v0.10.2 | Fiabilitate CLI | Ciclu de viață al proceselor, recuperare erori | Done |
| v0.10.3 | Integrare CLI | Asistent de configurare, auto-detectare furnizor | Done |
| v0.10.4 | Împachetare Tauri | Aplicație desktop, system tray | Done |
| v0.10.5 | API Proxy Sidecar | Selecție rute, prompt guard, statistici utilizare | Done |
| v0.10.6 | Pool de furnizori | Rutare multi-furnizor, verificare sănătate, failover | Done |
| v0.10.7 | Mod previzualizare | Acces neautentificat, controlul funcționalităților | Done |
| v0.10.8 | Skill Store | Infrastructură skill store, validare canale | Done |
| v0.10.9–10 | Gestionare utilizatori | Sub-utilizatori, permisiuni la nivel de pagină | Done |
| v0.10.13–14 | Securitate și abilități | Pagină de securitate, reproiectare skill store | Done |
| v0.10.15 | Îmbunătățiri chat | UX chat, pipeline mesaje | Done |
| v0.10.16 | Modul vocal | Sherpa TTS/ASR, eSpeak, comutare furnizor | Done |
| v0.10.17 | Acces la distanță | Ngrok, tuneluri Cloudflare, certificate ACME | Done |
| v0.10.18–20 | Sprint de performanță | Performanță pornire/chat, cache context | Done |
| v0.10.21–22 | Prompt și DingTalk | Prompt de sistem, canal DingTalk | Done |
| v0.10.23 | Actualizare OTA | Sistem de actualizare OTA | Done |
| v0.10.24 | Upgrade canale | 10 canale actualizate din stub-uri | Done |
| v0.10.25 | CC Cache | Cache pe două niveluri (L1 memorie + L2 disc) | Done |
| v0.10.26 | Humanizer | Pipeline de umanizare a răspunsurilor | Done |
| v0.10.27 | Context Pruner | 54% economie de tokeni pe cod (SWE-bench oficial), 46–47% pe documente generale (IR local), scorare BM25, segmentare | Done |
| v0.10.28 | Memory Service | Căutare progresivă, backend cu scriere duală | Done |

## Comunitate și suport

- **Probleme**: [Vă rugăm să raportați erori și solicitări de funcționalități aici](https://github.com/IceWhaleTech/ZimaOS-Blue/issues)
- **Discuții**: [Discord](https://discord.gg/b3AgFDxe9v)
- **Urmăriți-ne** pe [GitHub](https://github.com/IceWhaleTech)

## Licență

Acest proiect este licențiat sub Licența MIT — consultați fișierul [LICENSE](../../LICENSE) pentru detalii. Credem în open source și în a contribui înapoi comunității.

## Contribuitori

<p align="center">
  Made with ❤️ by <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
