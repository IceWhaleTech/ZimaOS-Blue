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
  <strong>Magyar</strong> |
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
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Blue/ci.yml?branch=main&style=for-the-badge" alt="CI állapot"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Blue?include_prereleases&style=for-the-badge" alt="GitHub kiadás"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="MIT licenc"></a>
</p>

<p align="center">
  <a href="https://discord.gg/SrCYvumF"><img src="../../docs/assets/discord.png" alt="Discord" height="128" /></a>&nbsp;&nbsp;
  <a href="https://www.facebook.com/zimaboard/"><img src="../../docs/assets/facebook.png" alt="Facebook" height="128" /></a>&nbsp;&nbsp;
  <a href="https://x.com/ZimaSpace"><img src="../../docs/assets/x.png" alt="X" height="128" /></a>&nbsp;&nbsp;
  <img src="../../docs/assets/wechat.png" height="128"/>
  <details style="display:inline-block;">
    <summary style="list-style:none;cursor:pointer;">
    </summary>

  </details>
</p>

## Bevezetés

A Clawdbot által inspirálva hisszük, hogy a személyi számítástechnika **jövőjét** a **sokszínű, helyi-első AI ágensek** fogják alakítani a hálózat peremén.

**A ZimaOS Blue a mi válaszunk** — egy teljesen **nyílt forráskódú, auditálható és éles üzemre kész ágens futtatókörnyezet és eszközkészlet**, amellyel privát, saját üzemeltetésű ágenseket telepíthet súrlódásmentesen.

Bátor fejlesztőknek készült, akik **saját ágenseiket kreatívan vagy kézzel szeretnék megalkotni**. A Blue **teljesítményre tervezett**: **Go** nyelven íródott, mindössze 10 MB memóriaigénnyel. Fut **bármilyen x86-on, Raspberry Pi-n, Windowson, macOS-en** — bárhol, ahol van áram.

![](../../docs/assets/features.png)

## Kiemelkedő jellemzők

### Helyi-első tervezés és automatikus modell-hozzáférés

Tovább gondolva: natív támogatás **20+ IM platformhoz**, **hangvezérelt** felületek természetes, kontextus-tudatos párbeszédekhez, **konfiguráció nélküli modellváltás** IDE-felismeréssel, és SOUL-rétegű személyiségek.

### Gyors és könnyű

Natívan Go-ban fordítva — nincs interpreter, nincs VM, nincs többletterhelés. Csendben fut mindenen, a szerverektől az asztali eszközökig.

| Metrika | ZimaOS Blue (Go) | OpenClaw (Node + dist) |
|---------|-------------------|------------------------|
| `--help` hideg / meleg | **0.18 s / < 0.01 s** | 3.31 s / ~1.11 s |
| `status` futásidő (legjobb 3-ból) | **< 0.01 s** | 5.98 s |
| `--help` csúcs RSS | **~10 MB** | ~394 MB |
| `status` csúcs RSS | **~15 MB** | ~1.52 GB |
| Futásidejű függőségek | **Nincs** | Node.js 18+ |

> Mérés macOS arm64-en, azonos gépen, 3 futás legjobbja. 2026. feb.

### Tiszta Go, bármilyen eszköz

100% Go, statikus bináris. **5 célplatformra keresztfordítás** azonnal elérhető (![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png) amd64/arm64, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png) amd64/arm64, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png) amd64). Nem kell Node futtatókörnyezet, Python vagy konténer. Tegye egy NAS-ra, Raspberry Pi-re, régi x86 routerre vagy Mac-re — egyszerűen fut. **Aztán adja hozzá saját felületét, logikáját és ágens képességeit** — egy kódbázis, minden platform.

### Biztonság és irányítás

Beépített sidecar API proxy mélységi védelemmel:
- **Sandbox végrehajtás** – Minden eszközhívás izolált környezetben fut.
- **Prompt injection védelem** – 7+ beépített elfogási stratégia.
- **Munkamenet-auditálás** – Teljes munkamenet-felügyelet, minden interakció nyomon követhető.
- **RBAC és WebAuthn** – Finomhangolt hozzáférés-vezérlés jelszó nélküli hitelesítéssel.

## Miért Blue

Hisszük, hogy a **következő generációs személyi számítástechnika** magába foglalja az LLM-eket — de az **irányítható, auditálható** ágensek maradnak az alapkő egyének és csapatok számára egyaránt. **A Blue nyújtja**:
- **Átfogó mag** – Fejlett modellkezelés, IM integráció, bővített személyiség és természetes nyelvi felületek, amelyek a mindennapi interakciókra vannak hangolva (fejhallgatók, hang, okosszemüvegek).
- **Helyi-első, ultrakönnyű, eszközök közötti** – Nem szükséges csúcskategóriás hardver. Fut mindenen, ami képes számítani.
- **Biztonságos és auditálható** – Munkamenet-auditálás, sandboxing, jogosultság-kezelés és beépített API proxy, amely alkalmazásszintű tűzfalként működik — minden bejövő/kimenő bájt látható.

Minimalizáljuk a sablonkódot, hogy **arra összpontosíthasson, ami számít**. A **ZimaOS tervezési filozófiájához** hűen a Blue nyújtja:
- **Nulláról egyre egy kattintással** – Azonnali telepítés, nincs bonyolult konfiguráció.
- **Gyors prototípuskészítés** – Kreatívan vagy kézzel készítsen forgatókönyv-specifikus eszközöket, interakciókat és alkalmazáscsomagokat.
- **Globálisan kész** – **A világ hatalmas**, és nem alapértelmezetten angol. **20+ nyelv, natívan**, akadályok nélkül.
- **Nyílt modell-ökoszisztéma** – Nincs szállítói kötöttség. Hozza saját modelljeit.

![](../../docs/assets/design_principle.png)

## Gyors kezdés

### 1. lehetőség: Asztali alkalmazás letöltése (macOS és Windows)

Töltse le a natív alkalmazást — nincs függőség, nincs fordítás.

- **macOS**: [DMG letöltése](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)
- **Windows**: [Telepítő letöltése](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)

### 2. lehetőség: Telepítő szkript

**macOS / Linux**
```bash
curl -fsSL https://ota.zimaos.com/blue | sh
```

**Windows (PowerShell)**
```powershell
irm https://ota.zimaos.com/blue/windows | iex
```

### 3. lehetőség: Fordítás forráskódból

```bash
git clone https://github.com/IceWhaleTech/ZimaOS-Blue.git
cd ZimaOS-Blue
./dev.sh
```

## Architektúra áttekintés

![](../../docs/assets/architecture.png)

### Adatfolyam

**Chat kérés (Proxy gyors útvonal)**
```
Client [Proxy API Key] → Auth Gate → Prompt Guard → Context Pruner (optional)
  → Provider Pool (route:auto/cloud/local) → CC Cache (L1→L2) check
  → Upstream LLM → Response → Cache Store → Metrics Writer → Client (SSE stream)
```

**Csatorna üzenetfolyam**
```
Telegram/Discord/... → Channel Manager → AutoReply check
  → (no match) → Chat Handler → LLM → Humanizer (MD→text) → Channel → User
```

**Hang pipeline**
```
WebSocket audio → STT (Whisper) → LLM Processing → TTS (eSpeak/Edge) → WebSocket audio
```

### Csomagtérkép (`server/internal/`)

| Réteg | Csomagok |
|-------|----------|
| Gateway | bootstrap, server, gateway |
| Proxy | proxy, connection, streaming, resilience |
| Szolgáltató | providerpool, providers, llm |
| Pruner | pruner (detector, segmenter, bm25, pipeline, cache) |
| Ágens | context, tools, personality, humanizer |
| Memória | memory, embedding, kvstore |
| Csatorna | channel, autoreply, i18n |
| Biztonság | security, auth, permission, rbac, mfa, password, oidc, extauth, sandbox, promptguard, audit |
| Hang | voice, tts, stt, speech |
| Megfigyelés | metrics, heartbeat, companion, profiling, leakdetect |
| Bővítmény | plugin, skill, skillstore |
| Integráció | browser, homeassistant, cron, workflow, formfiller, tunnel, crawler |
| Ütemező | scheduler, worker, workerpool, pool |
| Mag | lifecycle, config, logger, database, cache, ratelimit, retry, timeutil, sync |
| Rendszer | sysinfo, cgroup, iotask, watcher, resources, backup, update |
| Többbérlős | tenant, user, session, preview |

## Használat

![](../../docs/assets/handcraft.png)

## Mérföldkő ütemterv

![](../../docs/assets/timeline.png)

| Verzió | Fókusz | Fő érték | Állapot |
|--------|--------|----------|---------|
| v0.1 | Go futtatókörnyezet mag | Stabil kernel, 24 órás üzem | Done |
| v0.2 | Alapképességek | Minimálisan használható, LLM integráció | Done |
| v0.3 | NAS integráció | NAS natív, systemd támogatás | Done |
| v0.4 | Bővítményrendszer | Bővíthető, biztonsági alapok | Done |
| v0.5 | Termék alapvonal | Éles üzemre kész, dokumentáció | Done |
| v0.6 | Üzenetcsatornák | Többcsatornás támogatás | Done |
| v0.7 | Biztonság | OIDC, MFA, audit | Done |
| v0.8 | Teljesítmény | Optimalizálás, gyorsítótárazás, benchmarkok | Done |
| v0.9 | Ökoszisztéma | Többbérlős, böngésző-automatizálás, hang | Done |
| v0.10.0 | CLI csomagolás | CC CLI csomagolás, felismerés, automatikus frissítés | Done |
| v0.10.1 | Metrika-felügyelet | API statisztikák, token követés, TTFT | Done |
| v0.10.2 | CLI megbízhatóság | Folyamat-életciklus, hibakezelés | Done |
| v0.10.3 | CLI integráció | Beállítási varázsló, szolgáltató automatikus felismerés | Done |
| v0.10.4 | Tauri csomagolás | Asztali alkalmazás, rendszertálca | Done |
| v0.10.5 | API Proxy Sidecar | Útvonalválasztás, prompt guard, használati statisztikák | Done |
| v0.10.6 | Szolgáltató pool | Több szolgáltatós útválasztás, állapotellenőrzés, failover | Done |
| v0.10.7 | Előnézeti mód | Hitelesítés nélküli hozzáférés, funkció-kapuzás | Done |
| v0.10.8 | Skill Store | Skill store infrastruktúra, csatorna-validáció | Done |
| v0.10.9–10 | Felhasználókezelés | Alfelhasználók, oldalszintű jogosultságok | Done |
| v0.10.13–14 | Biztonság és skillek | Biztonsági oldal, skill store újratervezés | Done |
| v0.10.15 | Chat fejlesztések | Chat UX, üzenet-pipeline | Done |
| v0.10.16 | Beszédmodul | Sherpa TTS/ASR, eSpeak, szolgáltatóváltás | Done |
| v0.10.17 | Távoli hozzáférés | Ngrok, Cloudflare alagutak, ACME tanúsítványok | Done |
| v0.10.18–20 | Teljesítmény sprint | Indítási/chat teljesítmény, kontextus-gyorsítótár | Done |
| v0.10.21–22 | Prompt és DingTalk | Rendszer prompt, DingTalk csatorna | Done |
| v0.10.23 | OTA frissítés | OTA frissítési rendszer | Done |
| v0.10.24 | Csatorna frissítés | 10 csatorna fejlesztése csonkokból | Done |
| v0.10.25 | CC Cache | Kétszintű gyorsítótár (L1 memória + L2 lemez) | Done |
| v0.10.26 | Humanizer | Válasz-humanizálási pipeline | Done |
| v0.10.27 | Context Pruner | BM25 pontozás, szegmentálás, benchmarkok | Done |
| v0.10.28 | Memory Service | Progresszív keresés, kettős írás backend | Done |

## Közösség és támogatás

- **Hibajegyek**: [Kérjük, itt jelezze a hibákat és funkciókéréseket](https://github.com/IceWhaleTech/ZimaOS-Blue/issues)
- **Beszélgetések**: [Discord](https://discord.gg/SrCYvumF)
- **Kövessen minket** a [GitHubon](https://github.com/IceWhaleTech)

## Licenc

Ez a projekt az MIT licenc alatt áll — a részletekért lásd a [LICENSE](../../LICENSE) fájlt. Hiszünk a nyílt forráskódban és a közösségnek való visszaadásban.

## Közreműködők

<p align="center">
  Made with ❤️ by <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
