![](../../docs/assets/bannerX.png)

<h2 align="center">ZimaOS Blue: Local-first runtime agentov pre odvážnych tvorcov</h2>

<p align="center"><strong>Pripravené hneď po spustení · Open source · Univerzálne · Nezávislé od dodávateľa</strong></p>

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
  <a href="../ro_RO/README.md">Română</a> |
  <a href="../ru_RU/README.md">Русский</a> |
  <strong>Slovenčina</strong> |
  <a href="../sv_SE/README.md">Svenska</a> |
  <a href="../zh_CN/README.md">简体中文</a> |
  <a href="../zh_TW/README.md">繁體中文</a>
</p>

<p align="center">
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Blue/ci.yml?branch=main&style=for-the-badge" alt="Stav CI"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Blue?include_prereleases&style=for-the-badge" alt="Vydanie GitHub"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="Licencia MIT"></a>
</p>

<p align="center">
  <a href="https://discord.gg/b3AgFDxe9v"><img src="../../docs/assets/discord.png" alt="Discord" height="128" /></a>&nbsp;&nbsp;
  <a href="https://www.facebook.com/zimaboard/"><img src="../../docs/assets/facebook.png" alt="Facebook" height="128" /></a>&nbsp;&nbsp;
  <a href="https://x.com/ZimaSpace"><img src="../../docs/assets/x.png" alt="X" height="128" /></a>
</p>

## Úvod

Inšpirovaní OpenClawom a veríme, že budúcnosť osobných počítačov bude formovať rôznorodá, lokálna umelá inteligencia fungujúca na okraji.

ZimaOS Blue je naša odpoveď – plne open source, auditovateľný, dodávateľsky neutrálny a produkčne pripravený agent runtime a súprava nástrojov, ktorá vám umožňuje odosielať súkromných agentov s vlastným hosťovaním s nulovým trením.

Blue, stvorený pre odvážnych vývojárov, ktorí chcú vibrovať alebo ručne vyrábať svojich vlastných agentov, je navrhnutý pre výkon: napísaný v Go, s pamäťou len 19 MB. Beží na akomkoľvek x86, <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> ZimaOS, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS - kdekoľvek, kde pripojíte napájanie.

## Ukážky

### Konverzácia a vykonávanie úloh

Krátka ukážka toku konverzácie a vykonávania úloh v Blue.

<p align="center">
  <img src="../../docs/assets/demo.gif" alt="Conversation & Task Execution demo" width="800" />
</p>

### Integrácia poskytovateľov LLM

Krátka ukážka integrácie poskytovateľov LLM v Blue.

<p align="center">
  <img src="../../docs/assets/demo Provider.gif" alt="LLM Providers Integration demo" width="800" />
</p>

### Rýchly prehľad - Prehľad, kanály a dodatočná konfigurácia

Krátka ukážka, ktorá pokrýva celkový prehľad produktu, kanály a dodatočnú konfiguráciu.

<p align="center">
  <img src="../../docs/assets/demo quickv4.gif" alt="Quick Overview demo" width="720" />
</p>

## Prečo Blue

<p align="center">
  <img src="../../docs/assets/design_principle.png" alt="Design Principle" />
</p>

### Pure Go, akékoľvek zariadenie

100% Go, statický binárny súbor. Krížová kompilácia na 5 cieľov po vybalení z krabice (![linux](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png) `linux/amd64`, `linux/arm64`, ![macOS](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png) `darwin/amd64`, `darwin/arm64`, ![windows](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png) `windows/amd64`). Nevyžaduje sa žiadny runtime uzla, Python, žiadne kontajnery. Položte ho na NAS, <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> ZimaOS, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, starý x86 router alebo ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)Mac – jednoducho to beží. Potom navrstvite svoje vlastné používateľské rozhranie, logiku a schopnosti agentov – jedna kódová základňa, každá platforma.

### Po vybalení, pripravené na prácu

Každý chce nástroje, ktoré sú jednoduché, spoľahlivé a škálovateľné, keď ich potrebujete. Nástroje, ktoré jednoducho fungujú, takže sa môžete sústrediť na to, čo skutočne staviate.

Toto nie je nová filozofia. Je to ten istý, ktorý postavil <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> ZimaOS: jednoduchý, spoľahlivý a vyrobený tak, aby vám neprekážal. Blue je táto filozofia rozšírená na zásobník agentov.

### Navrhnuté pre váš život, vytvorené tak, aby zostali miestne

Od hĺbkového výskumu, ktorý poskytuje úplnú správu HTML, po OCR, PDF, automatizáciu prehliadača a konverziu dokumentov, Blue zvláda zložité pracovné postupy v reálnom svete bez odosielania údajov do cloudu. Hlasové budenie, STT/TTS, Talk Mode a podpora miestneho vyvodzovania robia každodenné interakcie okamžitými, súkromnými a vždy dostupnými.

## Rýchly štart

### Možnosť 1: Stiahnite si aplikáciu pre stolné počítače

Získajte natívnu aplikáciu – žiadne závislosti, žiadna kompilácia. Zabudovaná skúšobná konfigurácia s integráciou v priebehu niekoľkých sekúnd — začnite chatovať okamžite cez vzdialené pripojenie, nie je potrebné žiadne nastavenie robota. Skutočný zážitok z krabice.

- <img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="16" /> **ZimaOS**: [Spustiť na ZimaOS](https://www.zimaspace.com/zimaos?utm_source=blue)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)**macOS**: [Stiahnuť DMG](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)**Windows**: [Stiahnuť inštalačný program](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)

### Možnosť 2: Inštalácia skriptu

<img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="16" /> ZimaOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
curl -fsSL https://ota.zimaos.com/blue | sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
irm https://ota.zimaos.com/blue/windows | iex
```

### Možnosť 3: Zostavte zo zdroja

```bash
git clone https://github.com/IceWhaleTech/ZimaOS-Blue.git
cd ZimaOS-Blue
git submodule update --init --recursive
```

<img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="16" /> ZimaOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
sh build.sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
.\build.bat
```

> **Poznámka:** Zostavy systému Windows vyžadujú:
> - [MinGW-w64](https://www.mingw-w64.org/) (gcc) a [CMake](https://cmake.org/) pre natívne závislosti C (espeak-ng, whisper.cpp, opus, kokoro, onnx)
> - [Windows SDK](https://developer.microsoft.com/en-us/windows/downloads/windows-sdk/) pre systémové knižnice (winmm atď.)
>
> Uistite sa, že `gcc`, `cmake` sú vo vašom `PATH`.

## Prehľad architektúry

<p align="center">
  <img src="../../docs/assets/architecture.png" alt="architecture" />
</p>

Pokračujte ďalej: poskytuje natívnu podporu pre **20+ platforiem okamžitých správ**, **hlasom ovládané** rozhrania pre prirodzený, kontextový dialóg, **prepínanie modelov s nulovou konfiguráciou** so skenovaním IDE.

<p align="center">
  <img src="../../docs/assets/providers.png" alt="Supported Providers" />
</p>

## Ako stavať

<p align="center">
  <img src="../../docs/assets/handcraft.png" alt="handcraft" />
</p>

> ⚠️ [!IMPORTANT]
>
> Ak plánujete pokračovať v ladení alebo kódovaní vibrácií nad Blue, nepovažujte pár dobre vyzerajúcich rozhovorov za dôkaz vydania. Akákoľvek zmena, ktorá ovplyvňuje smerovanie, správanie pri vykonávaní, povrch nástroja, kontrolu rozpočtu, výber modelu alebo rámec vykonávania, by sa mala overiť pomocou Blue Harness, nie náhodnými kontrolami.
>
> Blue by sa tu mal riadiť jedným jednoduchým pravidlom: dáta ako prvé, brány ako prvé, prerezané ako posledné. V praxi to znamená aktualizovať príslušnú množinu údajov Harness / eval spec pred posúdením zmeny a potom ponechať jednu stabilnú `candidate_id` počas celého pokusu, takže správy o výbere, realizácii, rozpočte a pripravenosti popisujú rovnakého kandidáta namiesto štyroch nesúvisiacich cyklov.

### Odporúčané Harness Pracovný postup

1. Spustite `blue harness selector verify`
2. Spustite `blue harness execution verify`
3. Opätovne použite výberové hodnotenie pre `blue harness budget gate`
4. Dokončite pomocou `blue harness cutover-readiness`

Pre lokálnu iteráciu, nočnú validáciu alebo zber dôkazov CI uprednostnite `python3 scripts/cutover_candidate_pipeline.py`. Spustí celý výber -> realizácia -> rozpočet -> postupnosť pripravenosti pod jedným zdieľaným kandidátom, čo uľahčuje porovnávanie, kontrolu a oddeľovanie výsledkov.

### Prídavné zábradlia

| Oblasť | Čo pozerať |
|------|----------------|
| Základná stabilita | Udržujte základnú líniu, verziu súboru údajov a `candidate_id` stabilné, inak sa porovnanie posunie a výsledok nebude dôveryhodný. |
| Skutočný stavebný výkon | Pred spustením Harness znova vytvorte ovplyvnený binárny alebo frontendový balík, inak sa môže stať, že namiesto aktuálnej zmeny overíte zastarané správanie. |
| Registrácia trasy | Ak sa frontend a backend zmenia spoločne, pred posúdením funkcie prostredníctvom správania používateľského rozhrania sa uistite, že všetky nové backendové trasy sú skutočne zaregistrované, pretože chýbajúca registrácia často vyzerá ako logická chyba, ale v skutočnosti ide o `404`. |
| Vydanie rozsudku | Tuning pass je pripravený len vtedy, keď Harness nevykazuje žiadnu zmysluplnú regresiu a pripravenosť na prerezanie potvrdí, že kandidát je skutočne pripravený prerezať. |

Stručne povedané, naladenie na Blue nie je o tom, že „po niekoľkých chatoch sa cíti lepšie“. Ide o zaradenie kandidáta do Harness, zhromaždenie porovnateľných dôkazov a o tom, či je skutočne bezpečné ponechať zmenu, nech rozhodnú výsledky brány a pripravenosti.

## Funkcie

| Funkcia | Čo prináša |
|---------|-------------------|
| Webové vyhľadávanie s vysokou dostupnosťou a spustenie prehliadača | Jeden z **najostrejších diferenciátorov** Blue. Blue zjednocuje **štyri webové prístupové cesty** na vyhľadávanie, čítanie, extrahovanie a prehľadávanie; zachováva **tri záložné vrstvy** cez HTTP, extrakciu proxy a relácie prehliadača; spracováva **stránky proti botom** s detekciou výziev, opätovným použitím súborov cookie/relácií, utajením a odovzdaním prehliadača; a trasy cez **tri motory prehliadača**: `lightpanda`, spravovaný prehliadač Chromium a prenosový/miestny prehliadač Chrome. |
| Výskumná doba tri v jednom | **Jeden verejný výskumný záznam** môže smerovať do `deep_research`, `analyze` a `ui_review`. Rovnaký balík objavov a dôkazov potom vytvára **výskum založený na citáciách**, **ohraničené správy** a **štruktúrované prehľady používateľského rozhrania/UX/dostupnosti**. |
| Harness Runtime, hodnotenie a vývojový rámec | Robí hodnotenie **primitívnym** počas vývoja, školenia a výroby. Harness pokrýva **regresné a dymové kontroly**, skórovanie, základné línie, správy a validáciu za behu, potom prináša rovnaké dôkazy do **vývoja zručností**, následného hodnotenia, povýšenia alebo vrátenia späť a `AGENTS.md` alebo preskúmania návrhu pokynov. |
| Multimodálny Native-Capability-First Runtime | Udržuje **hlas, OCR, PDF, úlohy prehliadača, konverziu dokumentov, vypĺňanie štruktúrovaných formulárov, spracovanie médií a lokálne generovanie médií** najprv na **natívnych a lokálnych cestách**, s **smerovaním modelu iba vtedy, keď je to skutočne potrebné**. |
| Bezpečnosť a správa | Zahŕňa **spustenie v karanténe**, **obranu rýchlej injekcie**, **audit relácie**, povolenia, **RBAC**, **WebAuthn**, operačné zábradlia a **bezpečnostné skenovanie**. |
| LLM Wiki and Knowledge Space | Premení výstupy z pamäte, výskumu a runtime na **povrch znalostí podobný wiki** s **súhrnnými stránkami**, indexmi, **spätnými odkazmi**, **aktuálnosťou** a **pracovnými postupmi archivácie**. |
| Skill Store and Marketplace | Dodáva **objavovanie vstavaných zručností**, kurátorstvo, synchronizáciu a **miestne skenovanie**, takže rozšíriteľnosť je dostupná **od prvého dňa**. |
| Skupina poskytovateľov produkčnej úrovne | Poskytuje skutočný fond poskytovateľov s **kontrolami stavu**, **automatickým zlyhaním**, **ističmi** a **pretekaním poskytovateľov** pre dlhotrvajúce pracovné zaťaženie. |
| Zabudovaný miestny beh malého modelu | Dodáva vstavaný **`Qwen3.5-0.8B` + `llama.cpp`** runtime pre **miestne krátke otázky a odpovede**, rozpoznávanie obrázkov, smerovanie nástrojov, sumarizáciu, **kompresiu kontextu** a **predspracovanie dokumentov**. |
| Dlhodobá spoľahlivosť | **OTA aktualizácie**, **zálohovanie a obnovenie**, **obnovenie konfigurácie** a **obnovenie po zlyhaní** ako **vstavané prevádzkové problémy**. |

## Časová os míľnika

<p align="center">
  <img src="../../docs/assets/timeline.png" alt="Milestone Timeline" />
</p>

| Dátum | Verzia | Kľúčové slová / funkcie |
|------|---------|---------------------|
| 26. januára 2026 | `v0.1–v0.9` | Go runtime, plugin systém, automatizácia prehliadača |
| 27. – 28. januára 2026 | `v0.9.0–v0.9.2` | Zobrazenie úloh prehliadača, Blue Companion, Smart Form Filler |
| 29. – 31. januára 2026 | `v0.10.0–v0.10.9` | Claude Code CLI, API Proxy, reštrukturalizácia používateľského rozhrania |
| 1. – 3. februára 2026 | `v0.10.1–v0.10.22` | Metriky, vzdialený prístup, kontextová vyrovnávacia pamäť |
| 5. – 18. február 2026 | `v0.10.25–v0.10.29` | i18n, vyrovnávacia pamäť CC, kanál uvoľnenia |
| 20. – 25. február 2026 | `v0.10.28–v0.10.29` | Desktop loader, mobilné UX, redizajn pamäte |
| 28. február – 2. marec 2026 | `v0.10.30` | Deep Research, prehodnotenie schopností, bezpečnostná kontrola |
| 9. – 18. marec 2026 | `v0.10.31` | Generálna oprava prístrojovej dosky, VoiceChat refaktor, schválené miesta |
| 19. – 22. marec 2026 | `v0.10.32` | Harness zavádzanie, audit prepisu, vyhľadávanie na webe |
| 23. – 25. marec 2026 | `v0.10.33` | Harness skupiny, schválenia prehliadača, trh zručností |
| 29. – 30. marec 2026 | `v0.10.35` | Harness v3, relé prehliadača, kompresia kontextu |
| 31. marec – 1. apríl 2026 | `v0.10.36` | Audit prepisu, Harness prekrytia, analýza nástroja |
| 1. apríla 2026 | `v0.10.37` | Vytvrdzovanie počas prevádzky, rez Skill+Exec, obnovovací lesk |
| 2. – 5. apríla 2026 | `v0.10.38` | GitHub podpora, vylepšenie trhu, vylepšenia spoľahlivosti |
| 6. – 7. apríla 2026 | `v0.10.39` | Zjednotenie výskumu, vývojové plochy, zníženie pamäťových nárokov |

## Komunita a podpora

- **Problémy**: [Prosím, tu nahláste chyby a požiadavky na funkcie](https://github.com/IceWhaleTech/ZimaOS-Blue/issues)
- **Diskusie**: [Discord](https://discord.gg/zwWbKA4S2)
- **Sledujte nás** na [GitHub](https://github.com/IceWhaleTech)

[![Star History Chart](https://api.star-history.com/svg?repos=IceWhaleTech/ZimaOS-Blue&type=Date)](https://star-history.com/#IceWhaleTech/ZimaOS-Blue&Date)

## Licencia

Tento projekt je licencovaný pod licenciou MIT – podrobnosti nájdete v súbore [LICENSE](../../LICENSE). Veríme v open source a dávame späť komunite.

## Prispievatelia

Ďakujeme všetkým prispievateľom Blue:

<a href="https://community.vaunt.dev/board/IceWhaleTech/repository/ZimaOS-Blue">
  <img src="https://api.vaunt.dev/v1/github/entities/IceWhaleTech/repositories/ZimaOS-Blue/contributors?format=svg&limit=30" width="450" />
</a>

## Referencie

1. **OpenClaw** — lokálny agent s otvoreným zdrojovým kódom. Bol priekopníkom pri pripájaní LLM k lokálnym zariadeniam prostredníctvom kanálových adaptérov a volania nástrojov, čo priamo inšpirovalo runtime architektúru agenta Blue. https://github.com/openclaw/openclaw
2. **MiroMind** — Režim hlbokého výskumu so syntézou podloženou dôkazmi. Zabudovaný kanál hlbokého výskumu v tvare Blue: plánovanie, paralelné vyhľadávanie, deduplikácia dôkazov a generovanie správ HTML. https://www.miromind.ai
3. **Karpathy's LLM Wiki** — LLM ako kompilátor znalostí. Preformuluje LLM na vybudovanie trvalých, vyvíjajúcich sa znalostných priestorov, ktoré presahujú akumulačnú pascu RAG.
4. **OpenSpace (HKUDS)** — Samovyvíjajúci sa motor zručností. Rámec založený na DAG, kde sa agenti učia zo zlyhaní a získavajú špecializované zručnosti. https://github.com/HKUDS/OpenSpace
5. **Andrew Ng's Context Hub** — Register dokumentácie verzie API pre kódovacích agentov. Rieši halucinácie agentov a zabudnuté znalosti relácie. Poskytuje kurátorské dokumenty s verziou s anotačnými a spätnoväzbovými slučkami, čím sa dokumentácia mení na vedomostnú vrstvu, ktorá sa zlepšuje. https://github.com/andrewyng/context-hub
6. **Notion** — Jednoduché, ľudské a zámerne tiché. Inšpirovaný minimalistickým étosom Notion, Blue prináša teplo späť do siete. Kde sa rafinovaný serif stretáva s premysleným dizajnom a vytvára priestor, ktorý sa cíti ako doma. https://www.notion.com/about
7. **Matrix** — Vizuálna inšpirácia ikonickou estetikou digitálneho dažďa. Estetický smer pre technické schémy Blue.
8. **IceWhale** — Láska, smrť a roboty S2E2 "Ľad". Kolektív, ktorý sa zhromažďuje po celom svete, aby prelomil múry internetových gigantov a odolal koncentrácii údajov. Ľadová veľryba symbolizuje komunitu, ktorá spolu na okraji vytvára suverénne nástroje.
9. **ZimaOS Blue** — Láska, smrť a roboti S1E14 "Zima Blue". Metafora: inteligencia, ktorá začína v službe a vyvíja sa, aby preskúmala svet. Blue je agent múdrosti, zakorenený v jednoduchosti a siahajúci do hĺbky.
10. **ZimaOS** — Princípy zjednodušeného, ​​sústredeného a otvoreného dizajnu. ZimaOS aj Blue zdieľajú presvedčenie, že technológia by mala slúžiť používateľovi – nasadiť do 30 sekúnd, spustiť kdekoľvek, zostať neutrálna voči predajcovi. https://www.zimaspace.com/zimaos
