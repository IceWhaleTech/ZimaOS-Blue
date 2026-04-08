![](../../docs/assets/bannerX.png)

<h2 align="center">ZimaOS Blue: Un runtime d'agents local-first per a creadors valents</h2>

<p align="center"><strong>A punt des del primer moment · Codi obert · Universal · Neutral amb els proveïdors</strong></p>

<p align="center">
  <a href="../../README.md">English</a> |
  <strong>Català</strong> |
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
  <a href="../sk_SK/README.md">Slovenčina</a> |
  <a href="../sv_SE/README.md">Svenska</a> |
  <a href="../zh_CN/README.md">简体中文</a> |
  <a href="../zh_TW/README.md">繁體中文</a>
</p>

<p align="center">
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Blue/ci.yml?branch=main&style=for-the-badge" alt="Estat CI"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Blue?include_prereleases&style=for-the-badge" alt="Versió GitHub"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="Llicència MIT"></a>
</p>

<p align="center">
  <a href="https://discord.gg/b3AgFDxe9v"><img src="../../docs/assets/discord.png" alt="Discord" height="128" /></a>&nbsp;&nbsp;
  <a href="https://www.facebook.com/zimaboard/"><img src="../../docs/assets/facebook.png" alt="Facebook" height="128" /></a>&nbsp;&nbsp;
  <a href="https://x.com/ZimaSpace"><img src="../../docs/assets/x.png" alt="X" height="128" /></a>
</p>

## Introducció

Inspirats per OpenClaw, creiem que el futur de la informàtica personal estarà modelat per diversos agents d'IA locals que funcionen a la vora.

ZimaOS Blue és la nostra resposta: un conjunt d'eines i temps d'execució d'agents totalment de codi obert, auditable, neutral per a proveïdors i preparat per a la producció que us permet enviar agents privats i allotjats sense fricció.

Creat per a desenvolupadors atrevits que volen vibrar o crear els seus propis agents, Blue està dissenyat per al rendiment: escrit a Go, amb una empremta de memòria de tan sols 19 MB. S'executa a qualsevol x86, <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> ZimaOS, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS, a qualsevol lloc on connecteu l'alimentació.

## Demostracions

### Conversa i execució de tasques

Una demostració ràpida del flux de conversa i de l'execució de tasques a Blue.

<p align="center">
  <img src="../../docs/assets/demo.gif" alt="Conversation & Task Execution demo" width="800" />
</p>

### Integració de proveïdors LLM

Una demostració ràpida de l'experiència d'integració de proveïdors LLM a Blue.

<p align="center">
  <img src="../../docs/assets/demo Provider.gif" alt="LLM Providers Integration demo" width="800" />
</p>

### Visió general ràpida - Vista general, canals i configuració addicional

Una demostració ràpida que cobreix la vista general del producte, els canals i la configuració addicional.

<p align="center">
  <img src="../../docs/assets/demo quickv4.gif" alt="Quick Overview demo" width="720" />
</p>

## Per què Blue

<p align="center">
  <img src="../../docs/assets/design_principle.png" alt="Design Principle" />
</p>

### Pure Go, qualsevol dispositiu

100% Go, binari estàtic. Compila encreuament a 5 objectius fora de la caixa (![linux](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png) `linux/amd64`, `linux/arm64`, ![macOS](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png) `darwin/amd64`, `darwin/arm64`, ![windows](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png) `windows/amd64`). Sense temps d'execució de Node, sense Python, no requereixen contenidors. Col·loqueu-lo en un NAS, <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> ZimaOS, un ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, un antic encaminador x86 o un ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)Mac: només s'executa. A continuació, poseu una capa de les vostres pròpies habilitats d'interfície d'usuari, lògica i agent: una base de codi, cada plataforma.

### Fora de la caixa, llest per treballar

Tothom vol eines que siguin senzilles, fiables i escalables quan les necessiteu. Eines que només funcionen, perquè pugueu centrar-vos en allò que realment esteu construint.

Aquesta no és una filosofia nova. És el mateix que va crear <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> ZimaOS: senzill, fiable i dissenyat per mantenir-se fora del teu camí. Blue és aquesta filosofia, estesa a la pila d'agents.

### Dissenyat per a la teva vida, construït per mantenir-se local

Des d'una investigació profunda que ofereix un informe complet HTML, fins a OCR, PDF, l'automatització del navegador i la conversió de documents, Blue gestiona fluxos de treball complexos i del món real sense enviar les vostres dades al núvol. La activació de veu, STT/TTS, Talk Mode i el suport per a la inferència local fan que les interaccions quotidianes siguin instantànies, privades i sempre disponibles.

## Inici ràpid

### Opció 1: Baixeu l'aplicació d'escriptori

Obteniu l'aplicació nativa: sense dependències, sense compilació. Configuració de prova integrada amb incorporació en qüestió de segons: comenceu a xatejar a l'instant mitjançant connexió remota, sense necessitat de configuració de bot. Autèntica experiència fora de la caixa.

- <img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="16" /> **ZimaOS**: [Executa a ZimaOS](https://www.zimaspace.com/zimaos?utm_source=blue)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)**macOS**: [Baixa DMG](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)**Windows**: [Baixa l'instal·lador](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)

### Opció 2: instal·lar script

<img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="16" /> ZimaOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
curl -fsSL https://ota.zimaos.com/blue | sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
irm https://ota.zimaos.com/blue/windows | iex
```

### Opció 3: Crear des de la font

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

> **Nota:** Les compilacions de Windows requereixen:
> - [MinGW-w64](https://www.mingw-w64.org/) (gcc) i [CMake](https://cmake.org/) per a dependències C natives (espeak-ng, whisper.cpp, opus, kokoro, onnx)
> - [Windows SDK](https://developer.microsoft.com/en-us/windows/downloads/windows-sdk/) per a les biblioteques del sistema (winmm, etc.)
>
> Assegureu-vos que `gcc`, `cmake` estiguin al vostre `PATH`.

## Visió general de l'arquitectura

<p align="center">
  <img src="../../docs/assets/architecture.png" alt="architecture" />
</p>

Porta-ho més lluny: ofereix suport natiu per a més de 20 plataformes de missatgeria instantània, interfícies **controlades per veu** per a un diàleg natural i conscient del context, **canvi de model de configuració zero** amb escaneig IDE.

<p align="center">
  <img src="../../docs/assets/providers.png" alt="Supported Providers" />
</p>

## Com construir

<p align="center">
  <img src="../../docs/assets/handcraft.png" alt="handcraft" />
</p>

> ⚠️ [!IMPORTANT]
>
> Si teniu previst seguir ajustant o codificant vibracions a sobre de Blue, no considereu uns quants xats atractius com a proves d'alliberament. Qualsevol canvi que afecti l'encaminament, el comportament d'execució, la superfície de l'eina, el control del pressupost, la selecció de models o el marc d'execució s'ha de validar amb Blue Harness, no amb comprovacions puntuals ad hoc.
>
> Blue hauria de seguir una regla senzilla aquí: primer les dades, primer les portes, retallades per darrera. A la pràctica, això significa actualitzar el conjunt de dades/evaluació rellevant de Harness abans de jutjar un canvi i, a continuació, mantenir un `candidate_id` estable durant tot l'intent, de manera que els informes de selecció, execució, pressupost i preparació descriguin el mateix candidat en lloc de quatre execucions no relacionades.

### Flux de treball Harness recomanat

1. Executeu `blue harness selector verify`
2. Executeu `blue harness execution verify`
3. Reutilitza l'execució d'avaluació del selector per a `blue harness budget gate`
4. Acaba amb `blue harness cutover-readiness`

Per a la iteració local, la validació nocturna o la recollida d'evidències de CI, preferiu `python3 scripts/cutover_candidate_pipeline.py`. Executa el selector complet -> execució -> pressupost -> seqüència de preparació sota un candidat compartit, cosa que fa que el resultat sigui més fàcil de comparar, revisar i retallar.

### Baranes addicionals

| Àrea | Què veure |
|------|-----------------|
| Estabilitat de base | Manteniu la línia de base, la versió del conjunt de dades i `candidate_id` estables, o la comparació es desviarà i el resultat no serà fiable. |
| Sortida de construcció real | Reconstruïu el paquet binari o interfície afectat abans d'executar Harness, en cas contrari, podeu acabar validant el comportament obsolet en lloc del canvi actual. |
| Registre de ruta | Si l'interfície i el backend canvien junts, confirmeu que totes les rutes de backend noves estan realment registrades abans de jutjar la funció a través del comportament de la interfície d'usuari, perquè el registre perdut sovint sembla un error de lògica, però realment és un `404`. |
| Judici d'alliberament | Una passada de sintonització només està preparada quan Harness no mostra cap regressió significativa i la preparació per al tall confirma que el candidat està realment preparat per tallar-se. |

En resum, sintonitzar a la part superior de Blue no es tracta de "se sent millor en uns quants xats". Es tracta de posar el candidat a Harness, recollir proves comparables i deixar que els resultats de la porta i la preparació decideixin si el canvi és realment segur de mantenir.

## Característiques

| Característica | Què ofereix |
|---------|--------------------|
| Recuperació web d'alta disponibilitat i temps d'execució del navegador | Un dels **diferenciadors més clars** de Blue. Blue unifica **quatre camins d'accés web** per cercar, llegir, extreure i rastrejar; manté **tres capes alternatives** a les sessions HTTP, extracció de proxy i navegador; gestiona **pàgines anti-bot** amb detecció de desafiaments, reutilització de galetes/sessió, sigil·lació i transferència del navegador; i rutes a través de **tres motors de navegador**: `lightpanda`, Chromium gestionat i Chrome de retransmissió/local. |
| Temps d'execució d'investigació tres en un | **Una entrada de recerca pública** es pot dirigir a `deep_research`, `analyze` i `ui_review`. Aleshores, la mateixa pila de descobriments i proves produeix **recerca de primera cita**, **informes limitats** i **revisions estructurades d'IU/UX/accessibilitat**. |
| Harness Temps d'execució, avaluació i marc d'evolució | Fa que l'avaluació sigui una **primitiva en temps d'execució** en desenvolupament, formació i producció. Harness cobreix **regressions i controls de fum**, puntuació, línies de base, informes i validació del temps d'execució, i després inclou la mateixa evidència en **evolució d'habilitats**, avaluació de seguiment, promoció o retrocés i `AGENTS.md` o revisió de propostes d'instruccions. |
| Multimodal Native-Capability-First Runtime | Manté la **veu, OCR, PDF, les tasques del navegador, la conversió de documents, l'emplenament de formularis estructurats, el processament de mitjans i la generació local de mitjans** als **cams natius i locals en primer lloc**, amb **enrutament del model només quan realment es necessita**. |
| Seguretat i Governança | Inclou **execució sandbox**, **defensa d'injecció ràpida**, **auditoria de sessions**, permisos, **RBAC**, **WebAuthn**, baranes operatives i **exploració de seguretat d'habilitats**. |
| LLM Wiki i Espai de Coneixement | Converteix les sortides de memòria, investigació i temps d'execució en una **superfície de coneixement semblant a la wiki** amb **pàgines de resum**, índexs, **enllaços d'entrada**, **frescos** i **fluxos de treball d'arxiu**. |
| Botiga d'habilitats i mercat | S'envia **descobriment d'habilitats incorporats**, curació, sincronització i **escaneig local**, de manera que l'extensibilitat està disponible **des del primer dia**. |
| Grup de proveïdors de grau de producció | Proporciona un grup de proveïdors real amb **controls de salut**, **conversió automàtica per error**, **interruptors** i **curses de proveïdors** per a càrregues de treball de llarga durada. |
| Temps d'execució del model petit local integrat | Envia un temps d'execució **`Qwen3.5-0.8B` + `llama.cpp`** integrat per a **preguntes i respostes breus locals**, reconeixement d'imatges, encaminament d'eines, resum, **compressió de context** i **preprocessament de documents**. |
| Fiabilitat a llarg termini | Tracta **OTA actualitzacions**, **còpia de seguretat i restauració**, **recàrrega en calent de configuració** i **recuperació després d'un error** com a **preocupacions operatives integrades**. |

## Cronologia de la fita

<p align="center">
  <img src="../../docs/assets/timeline.png" alt="Milestone Timeline" />
</p>

| Data | Versió | Paraules clau / Característiques |
|------|----------|----------------------|
| 26 gener 2026 | `v0.1–v0.9` | Go d'execució, sistema de connectors, automatització del navegador |
| Del 27 al 28 de gener de 2026 | `v0.9.0–v0.9.2` | Visualització de tasques del navegador, Blue Companion, Smart Form Filler |
| Del 29 al 31 de gener de 2026 | `v0.10.0–v0.10.9` | Claude Code CLI, API Proxy, reestructuració de la IU |
| 1-3 de febrer de 2026 | `v0.10.1–v0.10.22` | Mètriques, accés remot, memòria cau de context |
| Del 5 al 18 de febrer de 2026 | `v0.10.25–v0.10.29` | i18n, CC Cache, canal de llançament |
| 20-25 de febrer de 2026 | `v0.10.28–v0.10.29` | Carregador d'escriptori, UX mòbil, redisseny de memòria |
| 28 de febrer al 2 de març de 2026 | `v0.10.30` | Deep Research, reclassificador d'habilitats, exploració de seguretat |
| Del 9 al 18 de març de 2026 | `v0.10.31` | Revisió del tauler, VoiceChat refactor, llocs aprovats |
| 19-22 de març de 2026 | `v0.10.32` | Harness llançament, auditoria de transcripcions, cerca web |
| 23-25 ​​de març de 2026 | `v0.10.33` | Harness grups, aprovacions de navegadors, mercat d'habilitats |
| 29-30 de març de 2026 | `v0.10.35` | Harness v3, relé del navegador, compressió de context |
| 31 de març a l'1 d'abril de 2026 | `v0.10.36` | Auditoria de transcripcions, superposicions Harness, anàlisi d'eines |
| 1 d'abril de 2026 | `v0.10.37` | Enduriment del temps d'execució, reducció de Skill+Exec, poliment de recuperació |
| Del 2 al 5 d'abril de 2026 | `v0.10.38` | Suport GitHub, perfeccionament del mercat, millores de fiabilitat |
| Del 6 al 7 d'abril de 2026 | `v0.10.39` | Unificació de la recerca, superfícies d'evolució, reducció de la petjada de memòria |

## Comunitat i suport

- **Problemes**: [Arxiu d'errors i sol·licituds de funcions aquí](https://github.com/IceWhaleTech/ZimaOS-Blue/issues)
- **Discussions**: [Discord](https://discord.gg/zwWbKA4S2)
- **Segueix-nos** a [GitHub](https://github.com/IceWhaleTech)

[![Star History Chart](https://api.star-history.com/svg?repos=IceWhaleTech/ZimaOS-Blue&type=Date)](https://star-history.com/#IceWhaleTech/ZimaOS-Blue&Date)

## Llicència

Aquest projecte té una llicència sota la llicència MIT; consulteu el fitxer [LICÈNCIA](../../LICÈNCIA) per obtenir més informació. Creiem en el codi obert i en el retorn a la comunitat.

## Col·laboradors

Gràcies a tots els col·laboradors de Blue:

<a href="https://community.vaunt.dev/board/IceWhaleTech/repository/ZimaOS-Blue">
  <img src="https://api.vaunt.dev/v1/github/entities/IceWhaleTech/repositories/ZimaOS-Blue/contributors?format=svg&limit=30" width="450" />
</a>

## Referències

1. **OpenClaw** — Primer agent de codi obert local. Va ser pioner en connectar LLM a dispositius locals mitjançant adaptadors de canal i trucades d'eines, inspirant directament l'arquitectura d'execució de l'agent Blue. https://github.com/openclaw/openclaw
2. **MiroMind** — Mode d'investigació profunda amb síntesi recolzada per evidències. El canal d'investigació profund integrat de Blue: planificació, recuperació paral·lela, deduplicació d'evidències i generació d'informes HTML. https://www.miromind.ai
3. **Karpathy's LLM Wiki** — LLM com a compilador de coneixements. Reformula els LLM per construir espais de coneixement persistents i en evolució, avançant més enllà de la trampa d'acumulació de RAG.
4. **OpenSpace (HKUDS)** — Motor d'habilitats autoevolució. Un marc basat en DAG on els agents aprenen dels errors i obtenen habilitats especialitzades. https://github.com/HKUDS/OpenSpace
5. **Andrew Ng's Context Hub** — Registre de documentació de l'API amb versions per a agents de codificació. Aborda les al·lucinacions dels agents i el coneixement de la sessió oblidada. Proporciona documents curats i versionats amb bucles d'anotacions i comentaris, convertint la documentació en una capa de coneixement que es millora. https://github.com/andrewyng/context-hub
6. **Notion** — Simple, humà i intencionadament tranquil. Inspirat en l'ethos minimalista de Notion, Blue retorna la calor a la graella. On el serif refinat es combina amb un disseny pensat, creant un espai que se senti com a casa. https://www.notion.com/about
7. **Matrix** — Inspiració visual de l'emblemàtica estètica digital de la pluja. La direcció estètica dels esquemes tècnics de Blue.
8. **IceWhale** — Love, Death & Robots S2E2 "Ice". Un col·lectiu que es reuneix arreu del món per trencar els murs dels gegants d'Internet i resistir la concentració de dades. La balena de gel simbolitza una comunitat que construeix eines sobiranes juntes a la vora.
9. **ZimaOS Blue** — Amor, mort i robots S1E14 "Zima Blue". Una metàfora: la intel·ligència que comença en servei i evoluciona per explorar el món. Blue és un agent de saviesa, arrelat en la simplicitat i que arriba a la profunditat.
10. **ZimaOS** — Principis de disseny simplificats, enfocats i oberts. Tant ZimaOS com Blue comparteixen la creença que la tecnologia hauria de servir per a l'usuari: desplegar-se en 30 segons, executar-se a qualsevol lloc, mantenir-se neutral envers el proveïdor. https://www.zimaspace.com/zimaos
