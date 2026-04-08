![](../../docs/assets/bannerX.png)

<h2 align="center">ZimaOS Blue: Lokalno-prvi runtime agenata za odvažne graditelje</h2>

<p align="center"><strong>Spremno za rad odmah · Otvorenog koda · Univerzalno · Neutralno prema dobavljačima</strong></p>

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
  <strong>Hrvatski</strong> |
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
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Blue/ci.yml?branch=main&style=for-the-badge" alt="Status CI"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Blue?include_prereleases&style=for-the-badge" alt="GitHub izdanje"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="MIT licenca"></a>
</p>

<p align="center">
  <a href="https://discord.gg/b3AgFDxe9v"><img src="../../docs/assets/discord.png" alt="Discord" height="128" /></a>&nbsp;&nbsp;
  <a href="https://www.facebook.com/zimaboard/"><img src="../../docs/assets/facebook.png" alt="Facebook" height="128" /></a>&nbsp;&nbsp;
  <a href="https://x.com/ZimaSpace"><img src="../../docs/assets/x.png" alt="X" height="128" /></a>
</p>

## Uvod

Inspirirani OpenClawom, vjerujemo da će budućnost osobnog računalstva oblikovati različiti, lokalni AI agenti koji rade na rubu.

ZimaOS Blue je naš odgovor — potpuno otvoreno okruženje, koje se može revidirati, neovisno o prodavaču i spremno za proizvodnju, izvršavanje agenta i skup alata koji vam omogućuje isporuku privatnih agenata koji se sami hostiraju bez problema.

Napravljen za odvažne programere koji žele vibrirati ili ručno izraditi vlastite agente, Blue je projektiran za performanse: napisan u Go, s memorijskim otiskom od samo 19 MB. Radi na bilo kojem x86, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Piju, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windowsu, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS-u — gdje god priključite napajanje.

## Demo prikazi

### Razgovor i izvršavanje zadataka

Kratki demo tijeka razgovora i izvršavanja zadataka u Blueu.

<p align="center">
  <img src="../../docs/assets/demo.gif" alt="Conversation & Task Execution demo" width="800" />
</p>

### Integracija LLM pružatelja

Kratki demo iskustva integracije LLM pružatelja u Blueu.

<p align="center">
  <img src="../../docs/assets/demo Provider.gif" alt="LLM Providers Integration demo" width="800" />
</p>

### Brzi pregled - Pregled, kanali i dodatna konfiguracija

Kratki demo koji pokriva pregled proizvoda, kanale i dodatnu konfiguraciju.

<p align="center">
  <img src="../../docs/assets/demo quickv4.gif" alt="Quick Overview demo" width="720" />
</p>

## Zašto Blue

<p align="center">
  <img src="../../docs/assets/design_principle.png" alt="Design Principle" />
</p>

### Pure Go, bilo koji uređaj

100% Go, statički binarni. Unakrsno kompajliranje na 5 ciljeva izvan okvira (![linux](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png) `linux/amd64`, `linux/arm64`, ![macOS](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png) `darwin/amd64`, `darwin/arm64`, ![windows](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png) `windows/amd64`). Nema Node runtimea, nema Pythona, nisu potrebni spremnici. Stavite ga na NAS, <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> ZimaOS, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, stari x86 usmjerivač ili ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)Mac — jednostavno radi. Zatim slojite vlastito korisničko sučelje, logiku i agentske vještine — jedna baza koda, svaka platforma.

### Izvan kutije, spreman za rad

Svatko želi alate koji su jednostavni, pouzdani i skaliraju kada su vam potrebni. Alati koji jednostavno rade, tako da se možete usredotočiti na ono što zapravo gradite.

Ovo nije nova filozofija. Isti je onaj koji je napravio <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> ZimaOS: jednostavan, pouzdan i napravljen da vam ne smeta. Blue je ta filozofija, proširena na hrpu agenata.

### Dizajniran za vaš život, stvoren da ostanete lokalni

Od dubinskog istraživanja koje donosi potpuno HTML izvješće, do OCR, PDF, automatizacije preglednika i konverzije dokumenata, Blue upravlja složenim tijekovima rada u stvarnom svijetu bez slanja vaših podataka u oblak. Glasovno buđenje, STT/TTS, Talk Mode i podrška za lokalno zaključivanje čine svakodnevne interakcije trenutnim, privatnim i uvijek dostupnima.

## Brzi početak

### Opcija 1: Preuzmite aplikaciju za stolna računala

Nabavite izvornu aplikaciju — bez ovisnosti, bez kompilacije. Ugrađena probna konfiguracija s integracijom u nekoliko sekundi — odmah počnite razgovarati putem daljinske veze, nije potrebno postavljanje bota. Pravo iskustvo izvan okvira.

- <img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="16" /> **ZimaOS**: [Pokreni na ZimaOS](https://www.zimaspace.com/zimaos?utm_source=blue)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)**macOS**: [Preuzmi DMG](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)**Windows**: [Preuzmi instalacijski program](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)

### Opcija 2: Instalirajte skriptu

<img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="16" /> ZimaOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
curl -fsSL https://ota.zimaos.com/blue | sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
irm https://ota.zimaos.com/blue/windows | iex
```

### Opcija 3: Izgradnja iz izvora

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

> **Napomena:** Međuverzije sustava Windows zahtijevaju:
> - [MinGW-w64](https://www.mingw-w64.org/) (gcc) i [CMake](https://cmake.org/) za izvorne C ovisnosti (espeak-ng, whisper.cpp, opus, kokoro, onnx)
> - [Windows SDK](https://developer.microsoft.com/en-us/windows/downloads/windows-sdk/) za sistemske biblioteke (winmm, itd.)
>
> Provjerite jesu li `gcc`, `cmake` u vašem `PATH`.

## Pregled arhitekture

<p align="center">
  <img src="../../docs/assets/architecture.png" alt="architecture" />
</p>

Idite dalje: pruža izvornu podršku za **20+ IM platformi**, **glasom vođena** sučelja za prirodan dijalog s obzirom na kontekst, **zamjenu modela bez konfiguracije** s IDE skeniranjem.

<p align="center">
  <img src="../../docs/assets/providers.png" alt="Supported Providers" />
</p>

## Kako graditi

<p align="center">
  <img src="../../docs/assets/handcraft.png" alt="handcraft" />
</p>

> ⚠️ [!IMPORTANT]
>
> Ako planirate nastaviti s ugađanjem ili kodiranjem vibe na Blue, nemojte smatrati nekoliko zgodnih razgovora dokazima za puštanje. Sve promjene koje utječu na usmjeravanje, ponašanje pri izvođenju, površinu alata, kontrolu proračuna, odabir modela ili okvir za izvođenje treba potvrditi pomoću Blue Harness, a ne ad hoc provjerama na licu mjesta.
>
> Blue bi ovdje trebao slijediti jedno jednostavno pravilo: prvo podaci, prvo vrata, zadnji rez. U praksi to znači ažuriranje relevantnog Harness skupa podataka/specifikacija procjene prije prosuđivanja promjene, zatim održavanje jednog stabilnog `candidate_id` tijekom cijelog pokušaja tako da izvješća o selektoru, izvršenju, proračunu i spremnosti opisuju istog kandidata umjesto četiri nepovezana pokretanja.

### Preporučeni tijek rada Harness

1. Pokrenite `blue harness selector verify`
2. Pokrenite `blue harness execution verify`
3. Ponovno upotrijebite procjenu selektora za `blue harness budget gate`
4. Završite s `blue harness cutover-readiness`

Za lokalnu iteraciju, noćnu provjeru valjanosti ili prikupljanje CI dokaza preferirajte `python3 scripts/cutover_candidate_pipeline.py`. Pokreće cijeli selektor -> izvođenje -> proračun -> redoslijed spremnosti pod jednim zajedničkim kandidatom, što čini rezultat lakšim za usporedbu, pregled i prekid.

### Dodatne zaštitne ograde

| Područje | Što gledati |
|------|----------------|
| Osnovna stabilnost | Držite osnovnu liniju, verziju skupa podataka i `candidate_id` stabilnima ili će se usporedba povući i rezultat neće biti pouzdan. |
| Stvarni izlaz | Ponovno izgradite pogođeni binarni ili sučelni paket prije pokretanja Harness, inače biste mogli završiti s provjerom valjanosti ustajalog ponašanja umjesto trenutne promjene. |
| Registracija rute | Ako se sučelje i pozadina mijenjaju zajedno, potvrdite da su sve nove pozadinske rute stvarno registrirane prije prosuđivanja značajke kroz ponašanje korisničkog sučelja, jer registracija koja nedostaje često izgleda kao logička pogreška, ali zapravo je `404`. |
| Presuda o oslobađanju | Prolaz za ugađanje spreman je tek kada Harness ne pokaže značajnu regresiju i spremnost za presjecanje potvrđuje da je kandidat zapravo spreman za prelazak. |

Ukratko, ugađanje na Blue ne odnosi se na "osjećaj je bolji nakon nekoliko razgovora." Radi se o stavljanju kandidata u Harness, prikupljanju usporedivih dokaza i prepuštanju rezultatima izlaza i spremnosti da odluče je li promjena doista sigurna za zadržavanje.

## Značajke

| Značajka | Što donosi |
|---------|------------------|
| Web-pretraživanje visoke dostupnosti i izvođenje preglednika | Jedan od Blue **najoštrijih razlika**. Blue objedinjuje **četiri staze pristupa webu** za pretraživanje, čitanje, izdvajanje i indeksiranje; čuva **tri rezervna sloja** preko HTTP-a, ekstrakcije proxyja i sesija preglednika; rukuje **anti-bot stranicama** s otkrivanjem izazova, ponovnim korištenjem kolačića/sesije, skrivenim pristupom i prijenosom preglednika; i rute preko **tri motora preglednika**: `lightpanda`, upravljani Chromium i relejni/lokalni Chrome. |
| Vrijeme izvođenja istraživanja tri u jednom | **Jedan javni istraživački unos** može usmjeriti u `deep_research`, `analyze` i `ui_review`. Isti hrp otkrića i dokaza zatim proizvodi **istraživanje prvo s citatima**, **ograničena izvješća** i **strukturirane UI/UX/preglede pristupačnosti**. |
| Harness okvir za izvođenje, evaluaciju i evoluciju | Čini evaluaciju **primitivom vremena izvođenja** u razvoju, obuci i proizvodnji. Harness pokriva **regresijske i dimne provjere**, bodovanje, osnovne linije, izvješća i provjeru vremena izvođenja, a zatim prenosi iste dokaze o **razvoju vještina**, naknadnoj evaluaciji, promociji ili povratu, te `AGENTS.md` ili pregledu prijedloga uputa. |
| Multimodalna Native-Capability-First Runtime | Zadržava **glas, OCR, PDF, zadatke preglednika, pretvorbu dokumenata, strukturirano ispunjavanje obrazaca, obradu medija i lokalno generiranje medija** na **nativnim i lokalnim stazama prvo**, uz **usmjeravanje modela samo kada je stvarno potrebno**. |
| Sigurnost i upravljanje | Uključuje **izvršenje u sandboxu**, **zaštitu od brzog ubacivanja**, **reviziju sesije**, dopuštenja, **RBAC**, **WebAuthn**, operativne zaštitne ograde i **sigurnosno skeniranje vještina**. |
| LLM Wiki i prostor znanja | Pretvara memoriju, istraživanje i rezultate izvođenja u **površinu znanja nalik na wiki** sa **stranicama sažetka**, indeksima, **povratnim vezama**, **svježinom** i **arhivskim tijekovima rada**. |
| Trgovina vještina i tržnica | Isporučuje **ugrađeno otkrivanje vještina**, upravljanje, sinkronizaciju i **lokalno skeniranje** tako da je proširivost dostupna **od prvog dana**. |
| Grupa pružatelja produkcijske razine | Pruža pravi skup pružatelja usluga s **provjerama stanja**, **automatskim prebacivanjem u slučaju greške**, **prekidačima strujnog kruga** i **utrkom pružatelja usluga** za dugotrajna radna opterećenja. |
| Ugrađeno lokalno vrijeme izvođenja malog modela | Isporučuje ugrađeno **`Qwen3.5-0.8B` + `llama.cpp`** runtime za **lokalna kratka pitanja i odgovore**, prepoznavanje slika, usmjeravanje alata, sažimanje, **kompresiju konteksta** i **pretprocesiranje dokumenata**. |
| Dugotrajna pouzdanost | Tretira **OTA ažuriranja**, **sigurnosno kopiranje i vraćanje**, **vruće ponovno učitavanje konfiguracije** i **oporavak nakon kvara** kao **ugrađene radne probleme**. |

## Vremenska traka prekretnica

<p align="center">
  <img src="../../docs/assets/timeline.png" alt="Milestone Timeline" />
</p>

| Datum | Verzija | Ključne riječi / značajke |
|------|---------|--------------------|
| 26. siječnja 2026. | `v0.1–v0.9` | Go runtime, sustav dodataka, automatizacija preglednika |
| 27. – 28. siječnja 2026. | `v0.9.0–v0.9.2` | Prikaz zadataka preglednika, Blue Companion, Smart Form Filler |
| 29. – 31. siječnja 2026. | `v0.10.0–v0.10.9` | Claude Code CLI, API Proxy, restrukturiranje korisničkog sučelja |
| 1. – 3. veljače 2026. | `v0.10.1–v0.10.22` | Mjerni podaci, daljinski pristup, predmemorija konteksta |
| 5. – 18. veljače 2026. | `v0.10.25–v0.10.29` | i18n, CC predmemorija, cjevovod izdanja |
| 20. – 25. veljače 2026. | `v0.10.28–v0.10.29` | Desktop loader, mobilni UX, redizajn memorije |
| 28. veljače – 2. ožujka 2026. | `v0.10.30` | Deep Research, reranker vještina, sigurnosno skeniranje |
| 9. – 18. ožujka 2026. | `v0.10.31` | Remont nadzorne ploče, VoiceChat refactor, odobrena mjesta |
| 19. – 22. ožujka 2026. | `v0.10.32` | Harness uvođenje, revizija prijepisa, pretraživanje weba |
| 23. – 25. ožujka 2026. | `v0.10.33` | Harness grupe, odobrenja preglednika, tržište vještina |
| 29. – 30. ožujka 2026. | `v0.10.35` | Harness v3, relej preglednika, kompresija konteksta |
| 31. ožujka – 1. travnja 2026. | `v0.10.36` | Revizija prijepisa, Harness preklapanja, raščlanjivanje alata |
| 1. travnja 2026. | `v0.10.37` | Runtime hardening, Skill+Exec cutover, recovery polish |
| 2. – 5. travnja 2026. | `v0.10.38` | GitHub podrška, usavršavanje tržišta, poboljšanja pouzdanosti |
| 6. – 7. travnja 2026. | `v0.10.39` | Objedinjavanje istraživanja, evolucijske površine, smanjenje memorijskog otiska |

## Zajednica i podrška

- **Problemi**: [Ovdje prijavite greške i zahtjeve za značajke](https://github.com/IceWhaleTech/ZimaOS-Blue/issues)
- **Rasprave**: [Discord](https://discord.gg/zwWbKA4S2)
- **Pratite nas** na [GitHub](https://github.com/IceWhaleTech)

[![Star History Chart](https://api.star-history.com/svg?repos=IceWhaleTech/ZimaOS-Blue&type=Date)](https://star-history.com/#IceWhaleTech/ZimaOS-Blue&Date)

## Licenca

Ovaj je projekt licenciran pod licencom MIT - pogledajte datoteku [LICENSE](../../LICENSE) za detalje. Vjerujemo u otvoreni kod i vraćanje zajednici.

## Suradnici

Hvala svim Blue suradnicima:

<a href="https://community.vaunt.dev/board/IceWhaleTech/repository/ZimaOS-Blue">
  <img src="https://api.vaunt.dev/v1/github/entities/IceWhaleTech/repositories/ZimaOS-Blue/contributors?format=svg&limit=30" width="450" />
</a>

## Reference

1. **OpenClaw** — prvi lokalni agent otvorenog koda. Pionir u povezivanju LLM-a s lokalnim uređajima putem adaptera kanala i pozivanja alata, izravno inspirirajući Blue arhitekturu vremena izvršavanja agenta. https://github.com/openclaw/openclaw
2. **MiroMind** — način dubokog istraživanja sa sintezom potkrijepljenom dokazima. Oblikovan Blue ugrađeni cjevovod dubinskog istraživanja: planiranje, paralelno dohvaćanje, deduplikacija dokaza i HTML generiranje izvješća. https://www.miromind.ai
3. **Karpathy's LLM Wiki** — LLM kao kompilator znanja. Preoblikuje LLM kako bi izgradio postojane prostore znanja koji se razvijaju, nadilazeći RAG-ovu zamku akumulacije.
4. **OpenSpace (HKUDS)** — Samorazvijajući motor vještina. Okvir temeljen na DAG-u u kojem agenti uče iz neuspjeha i stječu specijalizirane vještine. https://github.com/HKUDS/OpenSpace
5. **Andrew Ng's Context Hub** — verzionirani API registar dokumentacije za agente kodiranja. Rješava halucinacije agenta i zaboravljeno znanje o sesiji. Pruža odabrane dokumente s verzijama s petljama primjedbi i povratnih informacija, pretvarajući dokumentaciju u sloj znanja koji se sam poboljšava. https://github.com/andrewyng/context-hub
6. **Notion** — Jednostavno, ljudski i namjerno tiho. Nadahnut minimalističkim etosom Notion, Blue vraća toplinu u mrežu. Gdje se profinjeni serif susreće s promišljenim dizajnom, stvarajući prostor koji se osjeća kao kod kuće. https://www.notion.com/about
7. **Matrix** — Vizualna inspiracija iz legendarne digitalne kišne estetike. Estetski smjer tehničkih dijagrama Blue.
8. **IceWhale** — Ljubav, smrt i roboti S2E2 "Led". Kolektiv koji se okuplja diljem svijeta kako bi probio zidove internetskih divova i odupro se koncentraciji podataka. Ledeni kit simbolizira zajednicu koja zajedno na rubu gradi suverene alate.
9. **ZimaOS Blue** — Ljubav, smrt i roboti S1E14 "Zima Blue". Metafora: inteligencija koja počinje u službi i razvija se kako bi istražila svijet. Blue je zastupnik mudrosti, ukorijenjen u jednostavnosti i posezanju za dubinom.
10. **ZimaOS** — Pojednostavljena, fokusirana, otvorena načela dizajna. I ZimaOS i Blue dijele uvjerenje da tehnologija treba služiti korisniku — implementirati u 30 sekundi, pokrenuti bilo gdje, ostati neutralan prema dobavljaču. https://www.zimaspace.com/zimaos
