![](../../docs/assets/bannerX.png)

<h2 align="center">ZimaOS Blue: Local-first runtime agentů pro odvážné tvůrce</h2>

<p align="center"><strong>Připravené hned po spuštění · Open source · Univerzální · Nezávislé na dodavateli</strong></p>

<p align="center">
  <a href="../../README.md">English</a> |
  <a href="../ca_ES/README.md">Català</a> |
  <strong>Čeština</strong> |
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
  <a href="../ro_RO/README.md">Română</a> |
  <a href="../ru_RU/README.md">Русский</a> |
  <a href="../sk_SK/README.md">Slovenčina</a> |
  <a href="../sv_SE/README.md">Svenska</a> |
  <a href="../zh_CN/README.md">简体中文</a> |
  <a href="../zh_TW/README.md">繁體中文</a>
</p>

<p align="center">
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Blue/ci.yml?branch=main&style=for-the-badge" alt="Stav CI"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Blue?include_prereleases&style=for-the-badge" alt="Vydání na GitHubu"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="Licence MIT"></a>
</p>

<p align="center">
  <a href="https://discord.gg/b3AgFDxe9v"><img src="../../docs/assets/discord.png" alt="Discord" height="128" /></a>&nbsp;&nbsp;
  <a href="https://www.facebook.com/zimaboard/"><img src="../../docs/assets/facebook.png" alt="Facebook" height="128" /></a>&nbsp;&nbsp;
  <a href="https://x.com/ZimaSpace"><img src="../../docs/assets/x.png" alt="X" height="128" /></a>
</p>

## Úvod

Jsme inspirováni Clawdbotem a věříme, že budoucnost osobních počítačů bude utvářena různorodými agenty umělé inteligence, kteří jsou první na místě.

ZimaOS Blue je naše odpověď – plně open source, auditovatelný, dodavatelsky neutrální a produkčně připravený agent runtime a sada nástrojů, která vám umožní odesílat soukromé agenty s vlastním hostitelem s nulovým třením.

Blue, vytvořený pro odvážné vývojáře, kteří chtějí vibrovat nebo vlastnoručně vyrobit své vlastní agenty, je navržen pro výkon: napsaný v Go, s velikostí paměti pouhých 19 MB. Funguje na jakémkoli x86, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS — kdekoli, kde připojíte napájení.

## Ukázky

### Konverzace a vykonávání úkolů

Krátká ukázka toku konverzace a vykonávání úkolů v Blue.

<p align="center">
  <img src="../../docs/assets/demo.gif" alt="Conversation & Task Execution demo" width="800" />
</p>

### Integrace poskytovatelů LLM

Krátká ukázka prostředí pro integraci poskytovatelů LLM v Blue.

<p align="center">
  <img src="../../docs/assets/demo Provider.gif" alt="LLM Providers Integration demo" width="800" />
</p>

### Rychlý přehled - Přehled, kanály a další konfigurace

Krátká ukázka, která pokrývá celkový přehled produktu, kanály a další konfiguraci.

<p align="center">
  <img src="../../docs/assets/demo quickv4.gif" alt="Quick Overview demo" width="720" />
</p>

## Proč Blue

<p align="center">
  <img src="../../docs/assets/design_principle.png" alt="Design Principle" />
</p>

### Pure Go, jakékoli zařízení

100% Go, statický binární soubor. Křížová kompilace na 5 cílů po vybalení z krabice (![linux](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png) `linux/amd64`, `linux/arm64`, ![macOS](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png) `darwin/amd64`, `darwin/arm64`, ![windows](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png) `windows/amd64`). Žádné runtime uzlu, žádný Python, žádné kontejnery. Pusťte to na NAS, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, starý x86 router nebo ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)Mac – prostě to běží. Poté navrstvěte své vlastní uživatelské rozhraní, logiku a dovednosti agentů – jedna kódová základna, každá platforma.

### Rozbaleno, připraveno k práci

Každý chce nástroje, které jsou jednoduché, spolehlivé a škálovatelné, když je potřebujete. Nástroje, které prostě fungují, takže se můžete soustředit na to, co vlastně stavíte.

To není nová filozofie. Je to ten samý, který postavil <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> ZimaOS: jednoduchý, spolehlivý a postavený tak, aby vám nepřekážel. Blue je tato filozofie rozšířená na zásobník agentů.

### Navrženo pro váš život, navrženo tak, aby zůstalo místní

Od hloubkového výzkumu, který poskytuje úplnou zprávu HTML, až po OCR, PDF, automatizaci prohlížeče a konverzi dokumentů, Blue zvládá složité pracovní postupy v reálném světě bez odesílání vašich dat do cloudu. Hlasové probuzení, STT/TTS, Talk Mode a podpora místního vyvozování umožňují okamžité, soukromé a vždy dostupné každodenní interakce.

## Rychlý start

### Možnost 1: Stáhněte si aplikaci pro stolní počítače

Získejte nativní aplikaci – žádné závislosti, žádná kompilace. Vestavěná zkušební konfigurace s integrací během několika sekund — začněte chatovat okamžitě prostřednictvím vzdáleného připojení, není nutné žádné nastavení robota. Skutečný zážitek z krabice.

- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)**macOS**: [Stáhnout DMG](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)**Windows**: [Stáhnout instalační program](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)

### Možnost 2: Instalace skriptu

![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
curl -fsSL https://ota.zimaos.com/blue | sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
irm https://ota.zimaos.com/blue/windows | iex
```

### Možnost 3: Sestavení ze zdroje

```bash
git clone https://github.com/IceWhaleTech/ZimaOS-Blue.git
cd ZimaOS-Blue
git submodule update --init --recursive
```

![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
sh build.sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
.\build.bat
```

> **Poznámka:** Sestavení systému Windows vyžadují:
> - [MinGW-w64](https://www.mingw-w64.org/) (gcc) a [CMake](https://cmake.org/) pro nativní závislosti C (espeak-ng, whisper.cpp, opus, kokoro, onnx)
> - [Windows SDK](https://developer.microsoft.com/en-us/windows/downloads/windows-sdk/) pro systémové knihovny (winmm atd.)
>
> Ujistěte se, že `gcc`, `cmake` jsou ve vašem `PATH`.

## Přehled architektury

<p align="center">
  <img src="../../docs/assets/architecture.png" alt="architecture" />
</p>

Pokračujte dále: poskytuje nativní podporu pro **20+ platforem IM**, **hlasem řízená** rozhraní pro přirozený dialog s vědomím kontextu, **přepínání modelů s nulovou konfigurací** s skenováním IDE.

<p align="center">
  <img src="../../docs/assets/providers.png" alt="Supported Providers" />
</p>

## Jak stavět

<p align="center">
  <img src="../../docs/assets/handcraft.png" alt="handcraft" />
</p>

> ⚠️ [!IMPORTANT]
>
> Pokud plánujete pokračovat v ladění nebo kódování vibrací nad Blue, nepovažujte pár dobře vypadajících chatů za důkaz vydání. Jakákoli změna, která ovlivňuje směrování, chování při provádění, povrch nástroje, kontrolu rozpočtu, výběr modelu nebo rámec provádění, by měla být ověřena pomocí Blue Harness, nikoli pomocí namátkových kontrol ad hoc.
>
> Blue by se zde měl řídit jedním jednoduchým pravidlem: napřed data, napřed hradla, jako poslední ořez. V praxi to znamená aktualizovat příslušnou datovou sadu Harness / eval spec před posouzením změny a poté ponechat jednu stabilní `candidate_id` během celého pokusu, takže zprávy o výběru, provedení, rozpočtu a připravenosti všechny popisují stejného kandidáta namísto čtyř nesouvisejících běhů.

### Doporučeno Harness Pracovní postup

1. Spusťte `blue harness selector verify`
2. Spusťte `blue harness execution verify`
3. Znovu použijte výběrové hodnocení pro `blue harness budget gate`
4. Dokončete pomocí `blue harness cutover-readiness`

Pro místní opakování, noční ověřování nebo shromažďování důkazů CI preferujte `python3 scripts/cutover_candidate_pipeline.py`. Spouští celý selektor -> realizace -> rozpočet -> sekvence připravenosti pod jedním sdíleným kandidátem, což usnadňuje porovnání, kontrolu a odstranění výsledku.

### Extra zábradlí

| Oblast | Na co se dívat |
|------|----------------|
| Základní stabilita | Udržujte základní stav, verzi datové sady a `candidate_id` stabilní, jinak se srovnání posune a výsledek nebude důvěryhodný. |
| Skutečný stavební výkon | Před spuštěním Harness znovu sestavte dotčený binární nebo frontendový balíček, jinak můžete místo aktuální změny skončit ověřováním zastaralého chování. |
| Registrace trasy | Pokud se frontend a backend změní společně, ověřte, že všechny nové backendové trasy jsou skutečně zaregistrovány, než posoudíte funkci prostřednictvím chování uživatelského rozhraní, protože chybějící registrace často vypadá jako logická chyba, ale ve skutečnosti jde o `404`. |
| Vydání rozsudku | Průchod laděním je připraven pouze tehdy, když Harness nevykazuje žádnou smysluplnou regresi a připravenost na přeříznutí potvrdí, že kandidát je skutečně připraven přerušit. |

Stručně řečeno, ladění na vrcholu Blue není o tom, že "po pár chatech se cítí lépe." Jde o to zařadit kandidáta do Harness, shromáždit srovnatelné důkazy a nechat výsledky brány a připravenosti rozhodnout, zda je změna skutečně bezpečná.

## Funkce

| Funkce | Co přináší |
|---------|-------------------|
| Vysoká dostupnost webového vyhledávání a běhu prohlížeče | Jeden z **nejostřejších diferenciátorů** Blue. Blue sjednocuje **čtyři webové přístupové cesty** pro vyhledávání, čtení, extrahování a procházení; zachovává **tři záložní vrstvy** napříč HTTP, extrakcí proxy a relací prohlížeče; zpracovává **stránky proti botům** s detekcí výzev, opětovného použití souborů cookie/relací, utajení a předání prohlížeče; a směruje přes **tři vyhledávače**: `lightpanda`, spravovaný Chromium a reléový/místní Chromium. |
| Výzkumný běh tři v jednom | **Jeden veřejný výzkumný záznam** může směrovat do `deep_research`, `analyze` a `ui_review`. Stejný zásobník objevů a důkazů pak vytváří **výzkum založený na citacích**, **ohraničené zprávy** a **strukturované UI/UX/recenze přístupnosti**. |
| Harness runtime, evaluace a evoluční framework | Dělá z hodnocení **runtime primitivní** napříč vývojem, školením a výrobou. Harness pokrývá **regresní a kouřové kontroly**, bodování, základní linie, zprávy a validaci za běhu, poté přenáší stejné důkazy do **vývoje dovedností**, následného hodnocení, povýšení nebo vrácení zpět a `AGENTS.md` nebo přezkoumání návrhu instrukcí. |
| Multimodální Native-Capability-First Runtime | Udržuje **hlas, OCR, PDF, úlohy prohlížeče, převod dokumentů, vyplňování strukturovaných formulářů, zpracování médií a generování médií** na **nativních a místních cestách nejprve**, s **směrováním modelu pouze tehdy, když je to skutečně potřeba**. |
| Bezpečnost a správa | Zahrnuje **spuštění izolovaného prostoru**, **obranu rychlého vložení**, **audit relace**, oprávnění, **RBAC**, **WebAuthn**, provozní zábradlí a **bezpečnostní skenování**. |
| LLM Wiki a znalostní prostor | Přeměňuje výstupy paměti, výzkumu a běhu na **povrch znalostí podobný wiki** s **souhrnnými stránkami**, indexy, **zpětnými odkazy**, **aktuálností** a **pracovními postupy archivace**. |
| Obchod a tržiště dovedností | Odesílá **objevování vestavěných dovedností**, kurátorství, synchronizaci a **místní skenování**, takže rozšiřitelnost je dostupná **od prvního dne**. |
| Fond poskytovatelů produkčního stupně | Poskytuje skutečný fond poskytovatelů s **kontrolami stavu**, **automatickým převzetím služeb při selhání**, **jističi** a **závody poskytovatelů** pro dlouhodobou zátěž. |
| Vestavěný místní běh malého modelu | Dodává vestavěný **`Qwen3.5-0.8B` + `llama.cpp`** runtime pro **místní krátké otázky a odpovědi**, rozpoznávání obrázků, směrování nástrojů, sumarizaci, **kompresi kontextu** a **předzpracování dokumentů**. |
| Dlouhodobá spolehlivost | Zachází s **OTA aktualizacemi**, **zálohováním a obnovou**, **obnovením konfigurace za provozu** a **obnovením po selhání** jako s **vestavěnými provozními problémy**. |

## Časová osa milníku

<p align="center">
  <img src="../../docs/assets/timeline.png" alt="Milestone Timeline" />
</p>

| Datum | Verze | Klíčová slova / Funkce |
|------|---------|---------------------|
| 26. ledna 2026 | `v0.1–v0.9` | Go runtime, plugin systém, automatizace prohlížeče |
| 27.–28. ledna 2026 | `v0.9.0–v0.9.2` | Zobrazení úloh prohlížeče, Blue Companion, Smart Form Filler |
| 29.–31. ledna 2026 | `v0.10.0–v0.10.9` | Claude Code CLI, API Proxy, restrukturalizace uživatelského rozhraní |
| 1.–3. února 2026 | `v0.10.1–v0.10.22` | Metriky, vzdálený přístup, kontextová mezipaměť |
| 5.–18. února 2026 | `v0.10.25–v0.10.29` | i18n, CC Cache, uvolňovací kanál |
| 20.–25. února 2026 | `v0.10.28–v0.10.29` | Desktop loader, mobilní UX, redesign paměti |
| 28. února–2. března 2026 | `v0.10.30` | Deep Research, přehodnocení dovedností, bezpečnostní kontrola |
| 9.–18. března 2026 | `v0.10.31` | Generální oprava palubní desky, VoiceChat refaktor, schválená místa |
| 19.–22. března 2026 | `v0.10.32` | Zavedení Harness, audit přepisu, vyhledávání na webu |
| 23.–25. března 2026 | `v0.10.33` | Harness skupiny, schválení prohlížeče, trh dovedností |
| 29.–30. března 2026 | `v0.10.35` | Harness v3, relé prohlížeče, komprese kontextu |
| 31. března–1. dubna 2026 | `v0.10.36` | Audit přepisu, překryvy Harness, analýza nástroje |
| 1. dubna 2026 | `v0.10.37` | Kalení za běhu, řez Skill+Exec, obnovovací lesk |
| 2.–5. dubna 2026 | `v0.10.38` | GitHub podpora, vylepšení trhu, vylepšení spolehlivosti |
| 6.–7. dubna 2026 | `v0.10.39` | Sjednocení výzkumu, evoluční plochy, snížení paměťové náročnosti |

## Komunita a podpora

- **Problémy**: [Prosím zde nahlaste chyby a požadavky na funkce](https://github.com/IceWhaleTech/ZimaOS-Blue/issues)
- **Diskuse**: [Discord](https://discord.gg/zwWbKA4S2)
- **Sledujte nás** na [GitHub](https://github.com/IceWhaleTech)

[![Star History Chart](https://api.star-history.com/svg?repos=IceWhaleTech/ZimaOS-Blue&type=Date)](https://star-history.com/#IceWhaleTech/ZimaOS-Blue&Date)

## Licence

Tento projekt je licencován pod licencí MIT – podrobnosti naleznete v souboru [LICENSE](../../LICENSE). Věříme v open source a dávání zpět komunitě.

## Přispěvatelé

Děkujeme všem přispěvatelům Blue:

<a href="https://community.vaunt.dev/board/IceWhaleTech/repository/ZimaOS-Blue">
  <img src="https://api.vaunt.dev/v1/github/entities/IceWhaleTech/repositories/ZimaOS-Blue/contributors?format=svg&limit=30" width="450" />
</a>

## Reference

1. **OpenClaw** — Lokální agent s otevřeným zdrojovým kódem. Průkopnické připojení LLM k místním zařízením prostřednictvím kanálových adaptérů a volání nástrojů, přímo inspirující běhovou architekturu agenta Blue. https://github.com/openclaw/openclaw
2. **MiroMind** — Režim hlubokého výzkumu se syntézou podloženou důkazy. Vestavěný kanál hlubokého výzkumu ve tvaru Blue: plánování, paralelní vyhledávání, deduplikace důkazů a generování zpráv HTML. https://www.miromind.ai
3. **Karpathy's LLM Wiki** — LLM jako kompilátor znalostí. Přerámovává LLM tak, aby vybudovala trvalé, vyvíjející se znalostní prostory, které překračují akumulační pasti RAG.
4. **OpenSpace (HKUDS)** — Samovyvíjející se motor dovedností. Rámec založený na DAG, kde se agenti učí z neúspěchů a odvozují specializované dovednosti. https://github.com/HKUDS/OpenSpace
5. **Andrew Ng's Context Hub** — Registr dokumentace verze API pro kódovací agenty. Řeší halucinace agentů a zapomenuté znalosti relace. Poskytuje kurátorované dokumenty s verzemi s anotacemi a smyčkami zpětné vazby, čímž přeměňuje dokumentaci na sebezdokonalující se znalostní vrstvu. https://github.com/andrewyng/context-hub
6. **Notion** — Jednoduché, lidské a záměrně tiché. Inspirováno minimalistickým étosem Notion, Blue přináší teplo zpět do sítě. Kde se rafinovaný serif snoubí s promyšleným designem a vytváří prostor, který se cítí jako doma. https://www.notion.com/about
7. **Matrix** — Vizuální inspirace ikonickou estetikou digitálního deště. Estetický směr pro technická schémata Blue.
8. **IceWhale** — Láska, smrt a roboti S2E2 "Ice". Kolektiv, který se shromažďuje po celém světě, aby prolomil zdi internetových gigantů a odolal koncentraci dat. Ledová velryba symbolizuje komunitu budování suverénních nástrojů společně na okraji.
9. **ZimaOS Blue** — Láska, smrt a roboti S1E14 "Zima Blue". Metafora: inteligence, která začíná ve službě a vyvíjí se, aby prozkoumala svět. Blue je agentem moudrosti, má kořeny v jednoduchosti a sahající do hloubky.
10. **ZimaOS** — Principy zjednodušeného, ​​soustředěného a otevřeného návrhu. Jak ZimaOS, tak Blue sdílejí přesvědčení, že technologie by měla sloužit uživateli – nasazení do 30 sekund, spuštění kdekoli a neutrální vůči dodavateli. https://www.zimaspace.com/zimaos
