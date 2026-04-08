![](../../docs/assets/bannerX.png)

<h2 align="center">ZimaOS Blue: Local-first ügynök-futtatókörnyezet merész alkotóknak</h2>

<p align="center"><strong>Azonnal használható · Nyílt forráskódú · Univerzális · Szállítósemleges</strong></p>

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
  <a href="https://discord.gg/b3AgFDxe9v"><img src="../../docs/assets/discord.png" alt="Discord" height="128" /></a>&nbsp;&nbsp;
  <a href="https://www.facebook.com/zimaboard/"><img src="../../docs/assets/facebook.png" alt="Facebook" height="128" /></a>&nbsp;&nbsp;
  <a href="https://x.com/ZimaSpace"><img src="../../docs/assets/x.png" alt="X" height="128" /></a>
</p>

## Bevezetés

A OpenClaw által ihletett, úgy gondoljuk, hogy a személyi számítástechnika jövőjét a legkülönfélébb, helyileg elsőként működő mesterséges intelligenciaügynökök alakítják majd.

A ZimaOS Blue a válaszunk – egy teljesen nyílt forráskódú, auditálható, szállító-semleges és termelésre kész ügynöki futtatókörnyezet és eszközkészlet, amely lehetővé teszi privát, saját üzemeltetésű ügynökök szállítását nulla súrlódás nélkül.

Azok a merész fejlesztők számára készült, akik saját ügynökeiket szeretnék megmozgatni vagy megalkotni, a Blue teljesítményre tervezték: Go nyelven íródott, 19 MB-os memóriaterülettel. Bármilyen x86-on, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi-n, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows-on, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS-en fut – bárhol, ahol csatlakoztatja a tápfeszültséget.

## Demók

### Beszélgetés és feladatvégrehajtás

Egy gyors demó a Blue beszélgetési folyamatáról és feladatvégrehajtásáról.

<p align="center">
  <img src="../../docs/assets/demo.gif" alt="Conversation & Task Execution demo" width="800" />
</p>

### LLM-szolgáltatók integrációja

Egy gyors demó a Blue LLM-szolgáltatói integrációs élményéről.

<p align="center">
  <img src="../../docs/assets/demo Provider.gif" alt="LLM Providers Integration demo" width="800" />
</p>

### Gyors áttekintés - Áttekintés, csatornák és további konfiguráció

Egy gyors demó, amely bemutatja a termék áttekintését, a csatornákat és a további konfigurációt.

<p align="center">
  <img src="../../docs/assets/demo quickv4.gif" alt="Quick Overview demo" width="720" />
</p>

## Miért Blue

<p align="center">
  <img src="../../docs/assets/design_principle.png" alt="Design Principle" />
</p>

### Pure Go, bármilyen eszköz

100% Go, statikus bináris. Keresztfordítást végez 5 célpontra (![linux](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png) `linux/amd64`, `linux/arm64`, ![macOS](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png) `darwin/amd64`, `darwin/arm64`, ![windows](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png) `windows/amd64`). Nincs Node futási idő, nincs Python, nincs szükség konténerekre. Tegye rá egy NAS-ra, egy <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> ZimaOS-ra, egy ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi-re, egy régi x86-os útválasztóra vagy egy ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)Mac-re – csak fut. Ezután a saját felhasználói felületére, logikájára és ügynöki készségeire építsen – egy kódbázis, minden platformon.

### A dobozból, munkára készen

Mindenki olyan eszközöket szeretne, amelyek egyszerűek, megbízhatóak és méretezhetők, amikor szüksége van rájuk. Olyan eszközök, amelyek egyszerűen működnek, így arra összpontosíthat, amit valójában épít.

Ez nem egy új filozófia. Ez ugyanaz, mint ami <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> ZimaOS: egyszerű, megbízható és úgy készült, hogy távol maradjon az útjából. Blue ez a filozófia, kiterjesztve az ügynökveremre.

### Az Ön életére tervezve, helyben maradáshoz készült

A teljes HTML jelentést készítő mélyreható kutatástól a OCR, PDF, böngészőautomatizálásig és dokumentumkonverzióig a Blue összetett, valós munkafolyamatokat kezel anélkül, hogy az adatokat a felhőbe küldené. A Voice Wake, STT/TTS, Talk Mode és a helyi következtetés támogatása azonnalivá, priváttá és mindig elérhetővé teszi a mindennapi interakciókat.

## Gyorsindítás

### 1. lehetőség: Töltse le az asztali alkalmazást

Szerezze be a natív alkalmazást – nincs függőség, nincs fordítás. Beépített próbakonfiguráció másodpercek alatti beléptetéssel – azonnal elkezdhet csevegni távoli kapcsolaton keresztül, nincs szükség bot beállítására. Valódi készenléti élmény.

- <img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="16" /> **ZimaOS**: [Futtatás ZimaOS rendszeren](https://www.zimaspace.com/zimaos?utm_source=blue)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)**macOS**: [DMG letöltése](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)**Windows**: [Telepítő letöltése](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)

### 2. lehetőség: Szkript telepítése

<img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="16" /> ZimaOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
curl -fsSL https://ota.zimaos.com/blue | sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
irm https://ota.zimaos.com/blue/windows | iex
```

### 3. lehetőség: Build from Source

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

> **Megjegyzés:** A Windows buildekhez:
> - [MinGW-w64](https://www.mingw-w64.org/) (gcc) és [CMake](https://cmake.org/) a natív C-függőségekhez (espeak-ng, whisper.cpp, opus, kokoro, onnx)
> - [Windows SDK](https://developer.microsoft.com/en-us/windows/downloads/windows-sdk/) rendszerkönyvtárak számára (winmm stb.)
>
> Győződjön meg arról, hogy `gcc`, `cmake` a `PATH`-ban van.

## Építészet áttekintése

<p align="center">
  <img src="../../docs/assets/architecture.png" alt="architecture" />
</p>

Lépjen tovább: natív támogatást nyújt **20+ IM platformhoz**, **hangvezérelt** interfészek a természetes, környezettudatos párbeszédhez, **nulla konfigurációjú modellváltás** IDE-kereséssel.

<p align="center">
  <img src="../../docs/assets/providers.png" alt="Supported Providers" />
</p>

## Hogyan építsünk

<p align="center">
  <img src="../../docs/assets/handcraft.png" alt="handcraft" />
</p>

> ⚠️ [!IMPORTANT]
>
> Ha azt tervezi, hogy a hangolást vagy a hangulatkódolást a Blue tetején folytatja, ne tekintsen néhány jóképű csevegést a kiadás bizonyítékaként. Minden olyan változtatást, amely érinti az útválasztást, a végrehajtási viselkedést, a szerszámfelületet, a költségvetés-vezérlést, a modellválasztást vagy a végrehajtási keretrendszert, a Blue Harness segítségével kell érvényesíteni, nem pedig ad hoc helyszíni ellenőrzésekkel.
>
> Blue itt egy egyszerű szabályt kell követnie: az adatok először, a kapuk az elsők, a vágás az utolsó. A gyakorlatban ez azt jelenti, hogy frissíteni kell a megfelelő Harness adatkészletet / eval specifikációt a változás megítélése előtt, majd megtartani egy stabil `candidate_id` értéket a teljes kísérlet során, így a kiválasztási, végrehajtási, költségvetési és készenléti jelentések mind ugyanazt a jelöltet írják le a négy független futtatás helyett.

### Ajánlott Harness munkafolyamat

1. Futtassa a `blue harness selector verify` parancsot
2. Futtassa a `blue harness execution verify` parancsot
3. Használja újra a választó kiértékelési futtatását `blue harness budget gate`
4. Fejezd be a `blue harness cutover-readiness` karakterrel

Helyi iterációhoz, éjszakai érvényesítéshez vagy CI-bizonyítékok gyűjtéséhez előnyben részesítse a `python3 scripts/cutover_candidate_pipeline.py` címet. Egy megosztott jelölt alatt futtatja a teljes választó -> végrehajtás -> költségvetés -> készenléti szekvenciát, ami megkönnyíti az eredmény összehasonlítását, áttekintését és leválasztását.

### Extra védőkorlátok

| Terület | Mit kell nézni |
|------|-----------------|
| Kiindulási stabilitás | Tartsa stabilan az alapvonalat, az adatkészlet-verziót és a `candidate_id`, különben az összehasonlítás eltolódik, és az eredmény nem lesz megbízható. |
| Valódi összeállítási teljesítmény | A Harness futtatása előtt építse újra az érintett bináris vagy frontend csomagot, ellenkező esetben előfordulhat, hogy az elavult viselkedést fogja érvényesíteni a jelenlegi változás helyett. |
| Útvonal regisztráció | Ha a frontend és a háttérrendszer együtt változik, győződjön meg arról, hogy minden új háttérútvonal valóban regisztrálva van, mielőtt a szolgáltatást a felhasználói felület viselkedése alapján ítélné meg, mert a regisztráció hiánya gyakran logikai hibának tűnik, de valójában `404`. |
| Felmentési ítélet | A hangolás csak akkor készen áll, ha Harness nem mutat jelentős regressziót, és az átvágási készenlét megerősíti, hogy a jelölt valóban készen áll az átvágásra. |

Röviden, a Blue hangolása nem arról szól, hogy "néhány csevegés után jobban érzi magát." Arról van szó, hogy a jelöltet a Harness-ba helyezzük, összehasonlítható bizonyítékokat gyűjtünk, és hagyjuk, hogy a kapu és a készenléti eredmények eldöntsék, valóban biztonságos-e a változás megtartása.

## Jellemzők

| Funkció | Mit nyújt |
|---------|--------------------|
| Nagy rendelkezésre állású webes lekérdezés és böngészőfutási idő | Blue egyik **legélesebb megkülönböztetője**. A Blue **négy internetes elérési utat** egyesít a kereséshez, olvasáshoz, kibontáshoz és feltérképezéshez; **három tartalék réteget** tart fenn a HTTP, a proxy kibontása és a böngésző munkamenetei között; kezeli a **robotellenes oldalakat** kihívás észleléssel, cookie-k/munkamenetek újrafelhasználásával, lopakodással és böngésző átadás-átvétellel; és útvonalak **három böngészőmotoron**: `lightpanda`, felügyelt Chromium és közvetítő/helyi Chrome. |
| Három az egyben kutatási futásidő | **Egy nyilvános kutatási bejegyzés** a `deep_research`, `analyze` és `ui_review` címekre irányítható. Ugyanez a felfedezés és bizonyítékhalmaz **hivatkozás-első kutatást**, **korlátozott jelentéseket** és **strukturált felhasználói felület/UX/akadálymentesítési áttekintéseket** készít. |
| Harness Futásidejű, értékelési és evolúciós keretrendszer | Az értékelést **futásidejű primitívvé** teszi a fejlesztés, a képzés és a gyártás során. A Harness lefedi a **regresszió- és füstellenőrzést**, a pontozást, az alapvonalakat, a jelentéseket és a futásidejű érvényesítést, majd ugyanezeket a bizonyítékokat hordozza a **készségfejlesztés**, a nyomon követés értékelése, az előléptetés vagy a visszaállítás, valamint a `AGENTS.md` vagy az utasítási javaslatok áttekintése terén. |
| Multimodális natív képességek első futási ideje | Megtartja a **hangot, OCR, PDF, böngészőfeladatokat, dokumentumkonverziót, strukturált űrlapkitöltést, adathordozó-feldolgozást és helyi médiagenerálást** **elsősorban natív és helyi útvonalakon**, **modell-útválasztással csak akkor, ha valóban szükség van rá**. |
| Biztonság és kormányzás | Tartalmazza a **sandbox végrehajtást**, **azonnali befecskendezési védelmet**, **munkamenet-auditálást**, engedélyeket, **RBAC**, **WebAuthn**, működési védőkorlátokat és **készséges biztonsági szkennelést**. |
| LLM Wiki és Tudástér | A memóriát, a kutatást és a futásidejű kimeneteket **wikiszerű tudásfelületté** alakítja **összefoglaló oldalakkal**, indexekkel, **backlinkekkel**, **frissítéssel** és **archiválási munkafolyamatokkal**. |
| Skill Store és piactér | **Beépített készségfeltárást**, gondozást, szinkronizálást és **helyi szkennelést** szállít, így a bővíthetőség **az első naptól kezdve** elérhető. |
| Gyártási fokozatú szolgáltatók csoportja | Valódi szolgáltatói készletet biztosít **állapotellenőrzéssel**, **automatikus feladatátvétellel**, **megszakítókkal** és **szolgáltatói versenyfutással** a hosszan tartó terhelésekhez. |
| Beépített helyi kismodell futásidejű | Beépített **`Qwen3.5-0.8B` + `llama.cpp`** futásidejű **helyi rövid kérdések és válaszok**, képfelismerés, eszközútválasztás, összegzés, **környezettömörítés** és **dokumentum-előfeldolgozás**. |
| Hosszú távú megbízhatóság | A **OTA frissítéseket**, a **biztonsági mentést és visszaállítást**, a **konfiguráció gyors újratöltését** és a **hiba utáni helyreállítást** **beépített működési problémaként** kezeli. |

## Mérföldkő idővonal

<p align="center">
  <img src="../../docs/assets/timeline.png" alt="Milestone Timeline" />
</p>

| Dátum | Verzió | Kulcsszavak / Jellemzők |
|------|---------|----------------------|
| 2026. január 26. | `v0.1–v0.9` | Go runtime, plugin rendszer, böngésző automatizálás |
| 2026. január 27–28. | `v0.9.0–v0.9.2` | Böngésző feladat nézet, Blue Companion, Smart Form Filler |
| 2026. január 29–31. | `v0.10.0–v0.10.9` | Claude Code CLI, API Proxy, UI átalakítás |
| 2026. február 1–3. | `v0.10.1–v0.10.22` | Metrikák, távoli hozzáférés, kontextus gyorsítótár |
| 2026. február 5–18. | `v0.10.25–v0.10.29` | i18n, CC gyorsítótár, kiadási folyamat |
| 2026. február 20–25. | `v0.10.28–v0.10.29` | Asztali betöltő, mobil UX, memória újratervezés |
| 2026. február 28–március 2. | `v0.10.30` | Deep Research, készség átrendező, biztonsági vizsgálat |
| 2026. március 9–18. | `v0.10.31` | Irányítópult felújítása, VoiceChat refaktor, jóváhagyott helyek |
| 2026. március 19–22. | `v0.10.32` | Harness közzététel, átirat-ellenőrzés, webes keresés |
| 2026. március 23–25. | `v0.10.33` | Harness csoportok, böngésző jóváhagyások, képességpiac |
| 2026. március 29–30. | `v0.10.35` | Harness v3, böngésző relé, környezettömörítés |
| 2026. március 31–ápr. 1. | `v0.10.36` | Átirat-ellenőrzés, Harness átfedések, eszközelemzés |
| 2026. április 1. | `v0.10.37` | Üzemidejű edzés, Skill+Exec cutover, regeneráló polírozás |
| 2026. április 2–5. | `v0.10.38` | GitHub támogatás, piactér finomítása, megbízhatósági fejlesztések |
| 2026. április 6–7. | `v0.10.39` | Kutatási egységesítés, evolúciós felületek, memóriaigény csökkentése |

## Közösség és támogatás

- **Problémák**: [Kérjük, ide küldje el a hibákat és a funkciókra vonatkozó kéréseket](https://github.com/IceWhaleTech/ZimaOS-Blue/issues)
- **Megbeszélések**: [Discord](https://discord.gg/zwWbKA4S2)
- **Kövessen minket** a [GitHub](https://github.com/IceWhaleTech) címen

[![Star History Chart](https://api.star-history.com/svg?repos=IceWhaleTech/ZimaOS-Blue&type=Date)](https://star-history.com/#IceWhaleTech/ZimaOS-Blue&Date)

## Licenc

Ez a projekt az MIT Licenc alatt van licencelve – a részletekért lásd a [LICENSE](../../LICENSE) fájlt. Hiszünk a nyílt forráskódban és a közösségnek való visszaadásban.

## Közreműködők

Köszönet minden Blue közreműködőnek:

<a href="https://community.vaunt.dev/board/IceWhaleTech/repository/ZimaOS-Blue">
  <img src="https://api.vaunt.dev/v1/github/entities/IceWhaleTech/repositories/ZimaOS-Blue/contributors?format=svg&limit=30" width="450" />
</a>

## Referenciák

1. **OpenClaw** — Helyi első nyílt forráskódú ügynök. Az LLM-ek helyi eszközökhöz való csatlakoztatásának úttörője csatornaadapterek és eszközhívások révén, közvetlenül inspirálva a Blue ügynök futásidejű architektúráját. https://github.com/openclaw/openclaw
2. **MiroMind** — Mély kutatási mód bizonyítékokkal alátámasztott szintézissel. Megalakította Blue beépített mély kutatási folyamatát: tervezés, párhuzamos visszakeresés, bizonyítékok deduplikációja és HTML jelentések generálása. https://www.miromind.ai
3. **Karpathy's LLM Wiki** — LLM mint tudásfordító. Átalakítja az LLM-eket, hogy kitartó, fejlődő tudástereket építsen ki, túllépve a RAG felhalmozási csapdáján.
4. **OpenSpace (HKUDS)** — Önfejlesztő készségmotor. DAG-alapú keretrendszer, amelyben az ügynökök tanulnak a kudarcokból, és speciális készségekre tesznek szert. https://github.com/HKUDS/OpenSpace
5. **Andrew Ng's Context Hub** — Verziózott API dokumentációs nyilvántartás kódoló ügynökök számára. Az ügynök hallucinációival és az elfelejtett munkamenet-ismeretekkel foglalkozik. Feljegyzésekkel és visszacsatolási hurkokkal ellátott kurált, verziózott dokumentumokat biztosít, így a dokumentációt önfejlesztő tudásréteggé alakítja. https://github.com/andrewyng/context-hub
6. **Notion** – Egyszerű, emberi és szándékosan halk. A Notion minimalista szellemisége által ihletett Blue melegséget hoz vissza a hálózatba. Ahol a kifinomult serif találkozik az átgondolt dizájnnal, otthonos teret teremtve. https://www.notion.com/about
7. **Matrix** — Vizuális inspiráció az ikonikus digitális esőesztétikából. Blue műszaki diagramjainak esztétikai iránya.
8. **IceWhale** — Love, Death & Robots S2E2 "Jég". Egy kollektíva, amely világszerte összegyűlik, hogy áttörje az internetes óriások falait, és ellenálljon az adatkoncentrációnak. A jégbálna egy közösséget jelképez, amely szuverén eszközöket épít össze a szélén.
9. **ZimaOS Blue** — Love, Death & Robots S1E14 "Zima Blue". Metafora: az intelligencia, amely a szolgálatban kezdődik, és a világ felfedezésére fejlődik. A Blue a bölcsesség ügynöke, amely az egyszerűségben gyökerezik és mélységig nyúlik.
10. **ZimaOS** – Egyszerűsített, fókuszált, nyitott tervezési elvek. Mind a ZimaOS, mind a Blue hisz abban, hogy a technológiának ki kell szolgálnia a felhasználót – 30 másodperc alatt telepíthető, bárhol futhat, szállítósemleges marad. https://www.zimaspace.com/zimaos
