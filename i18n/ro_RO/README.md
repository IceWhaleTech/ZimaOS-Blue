![](../../docs/assets/bannerX.png)

<h2 align="center">ZimaOS Blue: Un runtime de agenți local-first pentru creatori îndrăzneți</h2>

<p align="center"><strong>Gata de folosit din prima · Open-source · Universal · Neutru față de furnizori</strong></p>

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

Inspirați de Clawdbot, credem că viitorul computerelor personale va fi modelat de agenți diverși de AI, primiți locali, care rulează la margine.

ZimaOS Blue este răspunsul nostru — un agent de rulare și un set de instrumente complet open-source, auditabil, neutru pentru furnizor și pregătit pentru producție, care vă permite să expediați agenți privați, auto-găzduiți, fără fricțiuni.

Creat pentru dezvoltatorii îndrăzneți care doresc să vibreze sau să își creeze manual propriii agenți, Blue este proiectat pentru performanță: scris în Go, cu o amprentă de memorie de până la 19 MB. Funcționează pe orice x86, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS — oriunde vă conectați la curent.

## Demonstrații

### Conversație și execuția sarcinilor

O demonstrație rapidă a fluxului de conversație și a execuției sarcinilor în Blue.

<p align="center">
  <img src="../../docs/assets/demo.gif" alt="Conversation & Task Execution demo" width="800" />
</p>

### Integrarea furnizorilor LLM

O demonstrație rapidă a experienței de integrare a furnizorilor LLM în Blue.

<p align="center">
  <img src="../../docs/assets/demo Provider.gif" alt="LLM Providers Integration demo" width="800" />
</p>

### Prezentare rapidă - Prezentare generală, canale și configurare suplimentară

O demonstrație rapidă care acoperă prezentarea generală a produsului, canalele și configurarea suplimentară.

<p align="center">
  <img src="../../docs/assets/demo quickv4.gif" alt="Quick Overview demo" width="720" />
</p>

## De ce Blue

<p align="center">
  <img src="../../docs/assets/design_principle.png" alt="Design Principle" />
</p>

### Pure Go, orice dispozitiv

100% Go, binar static. Compilează încrucișat la 5 ținte din cutie (![linux](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png) `linux/amd64`, `linux/arm64`, ![macOS](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png) `darwin/amd64`, `darwin/arm64`, ![windows](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png) `windows/amd64`). Fără runtime Node, fără Python, fără containere necesare. Puneți-l pe un NAS, un ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, un router x86 vechi sau un ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)Mac - pur și simplu rulează. Apoi stratificați-vă propriile abilități de interfață de utilizare, logică și agent — o bază de cod, fiecare platformă.

### Din cutie, gata de lucru

Toată lumea își dorește instrumente simple, fiabile și scalabile atunci când aveți nevoie de ele. Instrumente care funcționează, astfel încât să vă puteți concentra pe ceea ce construiți de fapt.

Aceasta nu este o filozofie nouă. Este același care a construit <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> ZimaOS: simplu, de încredere și construit pentru a nu vă împiedica. Blue este acea filozofie, extinsă la stiva de agenți.

### Conceput pentru viața ta, construit pentru a rămâne local

De la cercetare profundă care oferă un raport complet HTML, până la OCR, PDF, automatizarea browserului și conversia documentelor, Blue gestionează fluxuri de lucru complexe, din lumea reală, fără a trimite datele dumneavoastră în cloud. Activarea vocală, STT/TTS, Talk Mode și suportul pentru inferența locală fac interacțiunile de zi cu zi instantanee, private și întotdeauna disponibile.

## Pornire rapidă

### Opțiunea 1: Descărcați aplicația desktop

Obțineți aplicația nativă - fără dependențe, fără compilare. Configurație de probă încorporată cu integrare în câteva secunde — începeți să conversați instantaneu prin conexiune la distanță, nu este necesară configurarea botului. Adevărata experiență ieșită din cutie.

- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)**macOS**: [Descărcați DMG](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)**Windows**: [Descărcați programul de instalare](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)

### Opțiunea 2: Instalați Scriptul

![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
curl -fsSL https://ota.zimaos.com/blue | sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
irm https://ota.zimaos.com/blue/windows | iex
```

### Opțiunea 3: Construiește din sursă

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

> **Notă:** versiunile Windows necesită:
> - [MinGW-w64](https://www.mingw-w64.org/) (gcc) și [CMake](https://cmake.org/) pentru dependențe native C (espeak-ng, whisper.cpp, opus, kokoro, onnx)
> - [Windows SDK](https://developer.microsoft.com/en-us/windows/downloads/windows-sdk/) pentru bibliotecile de sistem (winmm, etc.)
>
> Asigurați-vă că `gcc`, `cmake` sunt în `PATH` dvs.

## Prezentare generală asupra arhitecturii

<p align="center">
  <img src="../../docs/assets/architecture.png" alt="architecture" />
</p>

Mergeți mai departe: oferă suport nativ pentru **20+ platforme IM**, interfețe **voice** pentru dialog natural, în funcție de context, **schimbare de model cu configurație zero** cu scanare IDE.

<p align="center">
  <img src="../../docs/assets/providers.png" alt="Supported Providers" />
</p>

## Cum se construiește

<p align="center">
  <img src="../../docs/assets/handcraft.png" alt="handcraft" />
</p>

> ⚠️ [!IMPORTANT]
>
> Dacă intenționați să continuați reglarea sau codarea vibrațiilor pe lângă Blue, nu tratați câteva chat-uri arătoase drept dovezi de eliberare. Orice modificare care afectează rutarea, comportamentul execuției, suprafața sculei, controlul bugetului, selecția modelului sau cadrul de execuție trebuie validată cu Blue Harness, nu cu verificări ad-hoc la fața locului.
>
> Blue ar trebui să urmeze o regulă simplă aici: datele mai întâi, porțile întâi, tăiat peste ultimul. În practică, aceasta înseamnă actualizarea setului de date/eval relevant Harness înainte de a judeca o modificare, apoi păstrarea unui `candidate_id` stabil pe întreaga încercare, astfel încât rapoartele de selecție, execuție, buget și pregătire să descrie același candidat în loc de patru rulări fără legătură.

### Flux de lucru Harness recomandat

1. Rulați `blue harness selector verify`
2. Rulați `blue harness execution verify`
3. Reutilizați rularea de evaluare a selectorului pentru `blue harness budget gate`
4. Terminați cu `blue harness cutover-readiness`

Pentru iterație locală, validare nocturnă sau colectare de dovezi CI, preferați `python3 scripts/cutover_candidate_pipeline.py`. Rulează selectorul complet -> execuție -> buget -> secvență de pregătire sub un candidat partajat, ceea ce face rezultatul mai ușor de comparat, revizuit și tăiat.

### Balustrade suplimentare

| Zona | Ce să urmărești |
|------|----------------|
| Stabilitate de bază | Păstrați linia de bază, versiunea setului de date și `candidate_id` stabile, altfel comparația se va deplasa și rezultatul nu va fi de încredere. |
| Real build output | Reconstruiți pachetul binar sau frontend afectat înainte de a rula Harness, altfel puteți ajunge să validați comportamentul învechit în loc de modificarea curentă. |
| Înregistrarea traseului | Dacă front-end-ul și back-end-ul se schimbă împreună, confirmați că toate rutele back-end noi sunt de fapt înregistrate înainte de a judeca caracteristica prin comportamentul UI, deoarece înregistrarea lipsă arată adesea ca o eroare logică, dar este într-adevăr un `404`. |
| Eliberare judecată | O trecere de reglare este gata numai atunci când Harness nu arată nicio regresie semnificativă și pregătirea pentru trecere confirmă că candidatul este de fapt gata să treacă. |

Pe scurt, reglarea pe Blue nu se referă la „se simte mai bine în câteva conversații”. Este vorba despre introducerea candidatului în Harness, strângerea de dovezi comparabile și lăsarea rezultatelor poarta și pregătirea să decidă dacă schimbarea este cu adevărat sigură de păstrat.

## Caracteristici

| Caracteristica | Ce oferă |
|---------|--------------------|
| Preluare Web de înaltă disponibilitate și timp de rulare a browserului | Unul dintre **cele mai clare diferențieri** ai lui Blue. Blue unifică **patru căi de acces la web** pentru căutare, citire, extragere și accesare cu crawlere; păstrează **trei straturi de rezervă** în sesiunile HTTP, extracție proxy și browser; gestionează **pagini anti-bot** cu detectarea provocării, reutilizarea cookie-urilor/sesiunii, ascuns și transferarea browserului; și rute prin **trei motoare de browser**: `lightpanda`, Chromium gestionat și Chromium releu/local. |
| Timp de execuție de cercetare trei-în-unul | **O înregistrare publică de cercetare** poate fi direcționată către `deep_research`, `analyze` și `ui_review`. Aceeași stivă de descoperiri și dovezi generează apoi **cercetare care primește citare**, **rapoarte limitate** și **evaluări structurate UI/UX/accesibilitate**. |
| Harness Cadru de rulare, evaluare și evoluție | Face din evaluare un **primitiv de rulare** în dezvoltare, instruire și producție. Harness acoperă **verificări de regresie și de fum**, scor, linii de bază, rapoarte și validarea timpului de execuție, apoi prezintă aceleași dovezi în **evoluția competențelor**, evaluare ulterioară, promovare sau derulare și `AGENTS.md` sau revizuirea propunerilor de instrucțiuni. |
| Multimodal Native-Capability-First Runtime | Păstrează **voce, OCR, PDF, sarcini de browser, conversie de documente, completare structurată a formularelor, procesare media și generare media locală** pe **căile native și locale mai întâi**, cu **rutarea modelului numai atunci când este efectiv necesar**. |
| Securitate și guvernare | Include **execuție sandbox**, **apărare prin injectare promptă**, **audit de sesiune**, permisiuni, **RBAC**, **WebAuthn**, balustrade operaționale și **scanare de securitate a competențelor**. |
| LLM Wiki și spațiu de cunoaștere | Transformă memoria, cercetarea și rezultatele de rulare într-o **suprafață de cunoștințe asemănătoare wiki** cu **pagini de rezumat**, indexuri, **backlink-uri**, **prospețime** și **fluxuri de lucru de arhivă**. |
| Magazin de abilități și piață | Se livrează **descoperirea abilităților încorporate**, curatarea, sincronizarea și **scanarea locală**, astfel încât extensibilitatea este disponibilă **din prima zi**. |
| Grup de furnizori de nivel de producție | Oferă un grup real de furnizori cu **verificări de sănătate**, **recuperare automată la eroare**, **întrerupătoare de circuit** și **curse de furnizori** pentru sarcini de lucru de lungă durată. |
| Timp de rulare a modelului mic local încorporat | Livrează un timp de rulare **`Qwen3.5-0.8B` + `llama.cpp`** încorporat pentru **Întrebări și răspunsuri scurte locale**, recunoaștere a imaginii, rutare a instrumentelor, rezumare, **comprimare a contextului** și **preprocesare a documentelor**. |
| Fiabilitate de lungă durată | Tratează **actualizările OTA**, **backup și restaurare**, **reîncărcarea la cald a configurației** și **recuperarea după eșec** ca **preocupări de funcționare încorporate**. |

## Cronologie de reper

<p align="center">
  <img src="../../docs/assets/timeline.png" alt="Milestone Timeline" />
</p>

| Data | Versiune | Cuvinte cheie / Caracteristici |
|------|----------|----------------------|
| 26 ianuarie 2026 | `v0.1–v0.9` | Go runtime, sistem plugin, automatizare browser |
| 27–28 ianuarie 2026 | `v0.9.0–v0.9.2` | Vizualizarea activității browser, Blue Companion, Smart Form Filler |
| 29–31 ianuarie 2026 | `v0.10.0–v0.10.9` | Claude Code CLI, API Proxy, restructurare UI |
| 1–3 februarie 2026 | `v0.10.1–v0.10.22` | Valori, acces la distanță, cache context |
| 5–18 februarie 2026 | `v0.10.25–v0.10.29` | i18n, CC Cache, conductă de lansare |
| 20–25 februarie 2026 | `v0.10.28–v0.10.29` | Încărcător desktop, UX mobil, reproiectare memorie |
| 28 februarie–2 martie 2026 | `v0.10.30` | Deep Research, reranker de aptitudini, scanare de securitate |
| 9–18 martie 2026 | `v0.10.31` | Revizuirea tabloului de bord, VoiceChat refactor, site-uri aprobate |
| 19–22 martie 2026 | `v0.10.32` | Harness lansare, audit de transcriere, căutare pe web |
| 23–25 martie 2026 | `v0.10.33` | Harness grupuri, aprobări browser, piață de competențe |
| 29–30 martie 2026 | `v0.10.35` | Harness v3, retransmitere browser, compresie context |
| 31 martie–1 apr 2026 | `v0.10.36` | Audit transcriere, suprapuneri Harness, analiza instrumentului |
| 1 apr 2026 | `v0.10.37` | Întărirea timpului de execuție, cutover Skill+Exec, lustruire de recuperare |
| 2–5 aprilie 2026 | `v0.10.38` | Asistență GitHub, rafinament pe piață, îmbunătățiri ale fiabilității |
| 6–7 aprilie 2026 | `v0.10.39` | Unificarea cercetării, suprafețe de evoluție, reducerea amprentei de memorie |

## Comunitate și asistență

- **Probleme**: [Te rugăm să înregistrezi erori și solicitări de funcții aici](https://github.com/IceWhaleTech/ZimaOS-Blue/issues)
- **Discuții**: [Discord](https://discord.gg/zwWbKA4S2)
- **Urmăriți-ne** pe [GitHub](https://github.com/IceWhaleTech)

[![Star History Chart](https://api.star-history.com/svg?repos=IceWhaleTech/ZimaOS-Blue&type=Date)](https://star-history.com/#IceWhaleTech/ZimaOS-Blue&Date)

## Licență

Acest proiect este licențiat sub licența MIT - consultați fișierul [LICENȚĂ](../../LICENSE) pentru detalii. Credem în sursa deschisă și în redarea comunității.

## Colaboratori

Mulțumim tuturor colaboratorilor Blue:

<a href="https://community.vaunt.dev/board/IceWhaleTech/repository/ZimaOS-Blue">
  <img src="https://api.vaunt.dev/v1/github/entities/IceWhaleTech/repositories/ZimaOS-Blue/contributors?format=svg&limit=30" width="450" />
</a>

## Referințe

1. **OpenClaw** — Primul agent open source local. A fost pionier în conectarea LLM-urilor la dispozitive locale prin adaptoare de canal și apeluri de instrumente, inspirând direct arhitectura de rulare a agentului Blue. https://github.com/openclaw/openclaw
2. **MiroMind** — Modul de cercetare profundă cu sinteză susținută de dovezi. Conținutul de cercetare profundă încorporat în Blue: planificare, extragere paralelă, deduplicare a dovezilor și generare de rapoarte HTML. https://www.miromind.ai
3. **Karpathy's LLM Wiki** — LLM ca compilator de cunoștințe. Reîncadrează LLM-urile pentru a construi spații de cunoștințe persistente și în evoluție, trecând dincolo de capcana de acumulare a RAG.
4. **OpenSpace (HKUDS)** — Motor de abilități care evoluează singur. Un cadru bazat pe DAG în care agenții învață din eșecuri și obțin abilități specializate. https://github.com/HKUDS/OpenSpace
5. **Andrew Ng's Context Hub** — Registrul de documentație API cu versiuni pentru agenții de codare. Abordează halucinațiile agenților și cunoștințele uitate ale sesiunii. Oferă documente organizate, versiuni, cu bucle de adnotări și feedback, transformând documentația într-un strat de cunoștințe care se auto-îmbunătățește. https://github.com/andrewyng/context-hub
6. **Notion** — Simplu, uman și intenționat liniștit. Inspirat de etosul minimalist al lui Notion, Blue aduce căldura înapoi în grilă. Unde serif rafinat se întâlnește cu designul atent, creând un spațiu care se simte ca acasă. https://www.notion.com/about
7. **Matrix** — Inspirație vizuală din estetica emblematică a ploaiei digitale. Direcția estetică pentru diagramele tehnice Blue.
8. **IceWhale** — Dragoste, moarte și roboți S2E2 „Gheață”. Un colectiv care se adună în întreaga lume pentru a sparge zidurile giganților internetului și a rezista concentrării datelor. Balena de gheață simbolizează o comunitate care construiește unelte suverane împreună la margine.
9. **ZimaOS Blue** — Dragoste, moarte și roboți S1E14 „Zima Blue”. O metaforă: inteligența care începe în serviciu și evoluează pentru a explora lumea. Blue este un agent al înțelepciunii, înrădăcinat în simplitate și care ajunge la profunzime.
10. **ZimaOS** — Principii de design simplificate, concentrate, deschise. Atât ZimaOS, cât și Blue împărtășesc convingerea că tehnologia ar trebui să servească utilizatorului - implementați în 30 de secunde, rulați oriunde, rămâneți neutru față de furnizor. https://www.zimaspace.com/zimaos
