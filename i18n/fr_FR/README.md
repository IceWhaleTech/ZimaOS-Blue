![](../../docs/assets/bannerX.png)

<h2 align="center">ZimaOS Blue : Un runtime d'agents local-first pour les créateurs audacieux</h2>

<p align="center"><strong>Prêt à l'emploi · Open source · Universel · Neutre vis-à-vis des fournisseurs</strong></p>

<p align="center">
  <a href="../../README.md">English</a> |
  <a href="../ca_ES/README.md">Català</a> |
  <a href="../cs_CZ/README.md">Čeština</a> |
  <a href="../da_DK/README.md">Dansk</a> |
  <a href="../de_DE/README.md">Deutsch</a> |
  <a href="../el_GR/README.md">Ελληνικά</a> |
  <a href="../en_GB/README.md">English (UK)</a> |
  <a href="../es_ES/README.md">Español</a> |
  <strong>Français</strong> |
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
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Blue/ci.yml?branch=main&style=for-the-badge" alt="Statut CI"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Blue?include_prereleases&style=for-the-badge" alt="Version GitHub"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="Licence MIT"></a>
</p>

<p align="center">
  <a href="https://discord.gg/b3AgFDxe9v"><img src="../../docs/assets/discord.png" alt="Discord" height="128" /></a>&nbsp;&nbsp;
  <a href="https://www.facebook.com/zimaboard/"><img src="../../docs/assets/facebook.png" alt="Facebook" height="128" /></a>&nbsp;&nbsp;
  <a href="https://x.com/ZimaSpace"><img src="../../docs/assets/x.png" alt="X" height="128" /></a>
</p>

## Présentation

Inspirés par Clawdbot, nous pensons que l'avenir de l'informatique personnelle sera façonné par des agents d'IA diversifiés et locaux fonctionnant à la périphérie.

ZimaOS Blue est notre réponse : un environnement d'exécution et une boîte à outils d'agent entièrement open source, vérifiables, indépendants des fournisseurs et prêts pour la production, qui vous permettent d'expédier des agents privés et auto-hébergés sans aucune friction.

Conçu pour les développeurs audacieux qui souhaitent créer ou créer leurs propres agents, Blue est conçu pour la performance : écrit en Go, avec une empreinte mémoire aussi faible que 19 Mo. Il fonctionne sur n'importe quel x86, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS — partout où vous branchez l'alimentation.

## Démos

### Conversation et exécution des tâches

Une démo rapide du flux de conversation et de l'exécution des tâches dans Blue.

<p align="center">
  <img src="../../docs/assets/demo.gif" alt="Conversation & Task Execution demo" width="800" />
</p>

### Intégration des fournisseurs LLM

Une démo rapide de l'expérience d'intégration des fournisseurs LLM dans Blue.

<p align="center">
  <img src="../../docs/assets/demo Provider.gif" alt="LLM Providers Integration demo" width="800" />
</p>

### Aperçu rapide - Vue d'ensemble, canaux et configuration supplémentaire

Une démo rapide couvrant la vue d'ensemble du produit, les canaux et la configuration supplémentaire.

<p align="center">
  <img src="../../docs/assets/demo quickv4.gif" alt="Quick Overview demo" width="720" />
</p>

## Pourquoi Blue

<p align="center">
  <img src="../../docs/assets/design_principle.png" alt="Design Principle" />
</p>

### Pure Go, sur n'importe quel appareil

100% Go, binaire statique. Compilations croisées vers 5 cibles prêtes à l'emploi (![linux](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png) `linux/amd64`, `linux/arm64`, ![macOS](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png) `darwin/amd64`, `darwin/arm64`, ![windows](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png) `windows/amd64`). Pas de runtime Node, pas de Python, pas de conteneurs requis. Déposez-le sur un NAS, un ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, un ancien routeur x86 ou un ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)Mac – il fonctionne tout simplement. Ajoutez ensuite vos propres compétences en matière d'interface utilisateur, de logique et d'agent : une base de code, chaque plate-forme.

### Prêt à l'emploi, prêt à fonctionner

Tout le monde veut des outils simples, fiables et évolutifs lorsque vous en avez besoin. Des outils qui fonctionnent, pour que vous puissiez vous concentrer sur ce que vous construisez réellement.

Ce n'est pas une nouvelle philosophie. C'est le même qui a construit <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> ZimaOS : simple, fiable et conçu pour ne pas vous gêner. Blue est cette philosophie, étendue à la pile d'agents.

### Conçu pour votre vie, construit pour rester local

De la recherche approfondie qui fournit un rapport HTML complet aux OCR, PDF, à l'automatisation du navigateur et à la conversion de documents, Blue gère des flux de travail complexes et réels sans envoyer vos données vers le cloud. Le réveil vocal, STT/TTS, Talk Mode et la prise en charge de l'inférence locale rendent les interactions quotidiennes instantanées, privées et toujours disponibles.

## Démarrage rapide

### Option 1 : Télécharger l'application de bureau

Obtenez l'application native : pas de dépendances, pas de compilation. Configuration d'essai intégrée avec intégration en quelques secondes : commencez à discuter instantanément via une connexion à distance, aucune configuration de robot n'est requise. Une véritable expérience hors du commun.

- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)**macOS** : [Télécharger DMG](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)**Windows** : [Télécharger le programme d'installation](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)

### Option 2 : Installer le script

![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
curl -fsSL https://ota.zimaos.com/blue | sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
irm https://ota.zimaos.com/blue/windows | iex
```

### Option 3 : Construire à partir des sources

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

> **Remarque :** Les builds Windows nécessitent :
> - [MinGW-w64](https://www.mingw-w64.org/) (gcc) et [CMake](https://cmake.org/) pour les dépendances natives C (espeak-ng, whisper.cpp, opus, kokoro, onnx)
> - [Windows SDK](https://developer.microsoft.com/en-us/windows/downloads/windows-sdk/) pour les bibliothèques système (winmm, etc.)
>
> Assurez-vous que `gcc`, `cmake` sont dans votre `PATH`.

## Présentation de l'architecture

<p align="center">
  <img src="../../docs/assets/architecture.png" alt="architecture" />
</p>

Allez plus loin : il offre une prise en charge native de **20+ plates-formes de messagerie instantanée**, des interfaces **à commande vocale** pour un dialogue naturel et contextuel, une **commutation de modèle sans configuration** avec analyse IDE.

<p align="center">
  <img src="../../docs/assets/providers.png" alt="Supported Providers" />
</p>

## Comment construire

<p align="center">
  <img src="../../docs/assets/handcraft.png" alt="handcraft" />
</p>

> ⚠️ [!IMPORTANT]
>
> Si vous prévoyez de continuer à régler ou à coder l'ambiance en plus de Blue, ne considérez pas quelques belles discussions comme une preuve de publication. Tout changement affectant le routage, le comportement d'exécution, la surface de l'outil, le contrôle budgétaire, la sélection de modèle ou le cadre d'exécution doit être validé avec Blue Harness, et non avec des contrôles ponctuels ad hoc.
>
> Blue devrait suivre une règle simple ici : les données en premier, les portes en premier, la coupure en dernier. En pratique, cela signifie mettre à jour l'ensemble de données / spécification d'évaluation Harness avant de juger un changement, puis en conserver un `candidate_id` stable tout au long de la tentative afin que les rapports de sélection, d'exécution, de budget et de préparation décrivent tous le même candidat au lieu de quatre exécutions sans rapport.

### Recommandé Harness Flux de travail

1. Exécutez `blue harness selector verify`
2. Exécutez `blue harness execution verify`
3. Réutilisez l'exécution de l'évaluation du sélecteur pour `blue harness budget gate`
4. Terminez avec `blue harness cutover-readiness`

Pour une itération locale, une validation nocturne ou une collecte de preuves CI, préférez `python3 scripts/cutover_candidate_pipeline.py`. Il exécute la séquence de sélection complète -> exécution -> budget -> préparation sous un seul candidat partagé, ce qui facilite la comparaison, l'examen et la transition du résultat.

### Garde-corps supplémentaires

| Zone | Que regarder |
|------|----------------|
| Stabilité de base | Gardez la ligne de base, la version de l'ensemble de données et `candidate_id` stables, sinon la comparaison dérivera et le résultat ne sera pas fiable. |
| Sortie de construction réelle | Reconstruisez le bundle binaire ou frontal concerné avant d'exécuter Harness, sinon vous risquez de valider un comportement obsolète au lieu de la modification actuelle. |
| Inscription des itinéraires | Si le frontend et le backend changent ensemble, confirmez que toutes les nouvelles routes backend sont réellement enregistrées avant de juger la fonctionnalité via le comportement de l'interface utilisateur, car l'enregistrement manquant ressemble souvent à un bug logique mais est en réalité un `404`. |
| Relâcher le jugement | Une passe de réglage n'est prête que lorsque Harness ne montre aucune régression significative et que la préparation au basculement confirme que le candidat est réellement prêt à effectuer le basculement. |

En bref, le réglage au-dessus de Blue ne consiste pas à « se sentir mieux après quelques discussions ». Il s'agit de placer le candidat dans Harness, de collecter des preuves comparables et de laisser les résultats de la porte d'entrée et de l'état de préparation décider si le changement est vraiment sûr à conserver.

## Fonctionnalités

| Fonctionnalité | Ce qu'il offre |
|---------|---------|
| Récupération Web haute disponibilité et exécution du navigateur | L'un des **différentiateurs les plus marqués** de Blue. Blue unifie **quatre chemins d'accès Web** pour la recherche, la lecture, l'extraction et l'exploration ; conserve **trois couches de secours** dans les sessions HTTP, d'extraction de proxy et de navigateur ; gère les **pages anti-bot** avec détection de défi, réutilisation des cookies/sessions, furtivité et transfert du navigateur ; et des itinéraires à travers **trois moteurs de navigateur** : `lightpanda`, Chromium géré et Chromium relais/local. |
| Exécution de recherche trois-en-un | **Une entrée de recherche publique** peut être acheminée vers `deep_research`, `analyze` et `ui_review`. La même pile de découvertes et de preuves produit ensuite des **recherches axées sur les citations**, des **rapports limités** et des **examens structurés de l'interface utilisateur/UX/accessibilité**. |
| Harness Cadre d'exécution, d'évaluation et d'évolution | Fait de l'évaluation une **primitive d'exécution** pour le développement, la formation et la production. Harness couvre les **contrôles de régression et de fumée**, la notation, les références, les rapports et la validation d'exécution, puis intègre les mêmes preuves dans l'**évolution des compétences**, l'évaluation de suivi, la promotion ou la restauration, et `AGENTS.md` ou l'examen des propositions d'instructions. |
| Runtime multimodal à capacité native | Conserve **la voix, OCR, PDF, les tâches du navigateur, la conversion de documents, le remplissage de formulaires structurés, le traitement multimédia et la génération multimédia** sur les **chemins natifs et locaux en premier**, avec le **routage de modèle uniquement lorsque cela est réellement nécessaire**. |
| Sécurité et gouvernance | Inclut l'**exécution sandbox**, la **défense par injection d'invite**, l'**audit de session**, les autorisations, **RBAC**, **WebAuthn**, les garde-fous opérationnels et l'**analyse de sécurité des compétences**. |
| Wiki LLM et espace de connaissances | Transforme les sorties de mémoire, de recherche et d'exécution en une **surface de connaissances de type wiki** avec des **pages de résumé**, des index, des **backlinks**, de la **fraîcheur** et des **workflows d'archives**. |
| Magasin de compétences et marché | Fournit **la découverte de compétences intégrée**, la conservation, la synchronisation et l'**analyse locale** afin que l'extensibilité soit disponible **dès le premier jour**. |
| Pool de fournisseurs de qualité production | Fournit un véritable pool de fournisseurs avec des **vérifications d'état**, un **basculement automatique**, des **disjoncteurs** et une **course de fournisseurs** pour les charges de travail de longue durée. |
| Runtime de petit modèle local intégré | Fourni un environnement d'exécution **`Qwen3.5-0.8B` + `llama.cpp`** intégré pour les **courtes questions et réponses locales**, la reconnaissance d'images, le routage d'outils, la synthèse, la **compression de contexte** et le **prétraitement de documents**. |
| Fiabilité à long terme | Traite les **OTA mises à jour**, **sauvegarde et restauration**, **rechargement à chaud de la configuration** et **récupération après panne** comme des **problèmes de fonctionnement intégrés**. |

## Chronologie des jalons

<p align="center">
  <img src="../../docs/assets/timeline.png" alt="Milestone Timeline" />
</p>

| Dates | Version | Mots-clés/Caractéristiques |
|------|---------|-----------|
| 26 janvier 2026 | `v0.1–v0.9` | Passez au runtime, au système de plugins, à l'automatisation du navigateur |
| 27 et 28 janvier 2026 | `v0.9.0–v0.9.2` | Vue des tâches du navigateur, Blue Companion, Smart Form Filler |
| 29-31 janvier 2026 | `v0.10.0–v0.10.9` | Claude Code CLI, API Proxy, restructuration de l'interface utilisateur |
| 1er au 3 février 2026 | `v0.10.1–v0.10.22` | Métriques, accès à distance, cache contextuel |
| 5-18 février 2026 | `v0.10.25–v0.10.29` | i18n, CC Cache, pipeline de versions |
| 20-25 février 2026 | `v0.10.28–v0.10.29` | Chargeur de bureau, UX mobile, refonte de la mémoire |
| 28 février – 2 mars 2026 | `v0.10.30` | Deep Research, reclassement des compétences, analyse de sécurité |
| 9-18 mars 2026 | `v0.10.31` | Refonte du tableau de bord, VoiceChat refactor, sites approuvés |
| 19-22 mars 2026 | `v0.10.32` | Harness déploiement, audit des relevés de notes, recherche sur le Web |
| 23-25 ​​mars 2026 | `v0.10.33` | Harness groupes, approbations des navigateurs, marché des compétences |
| 29 et 30 mars 2026 | `v0.10.35` | Harness v3, relais navigateur, compression de contexte |
| 31 mars – 1er avril 2026 | `v0.10.36` | Audit de transcription, superpositions Harness, analyse d'outils |
| 1 avril 2026 | `v0.10.37` | Durcissement de l'exécution, basculement Skill+Exec, polissage de récupération |
| 2-5 avril 2026 | `v0.10.38` | GitHub support, perfectionnement du marché, améliorations de la fiabilité |
| 6 et 7 avril 2026 | `v0.10.39` | Unification de la recherche, surfaces d'évolution, réduction de l'empreinte mémoire |

## Communauté et assistance

- **Problèmes** : [Veuillez déposer les bogues et les demandes de fonctionnalités ici](https://github.com/IceWhaleTech/ZimaOS-Blue/issues)
- **Discussions** : [Discord](https://discord.gg/zwWbKA4S2)
- **Suivez-nous** sur [GitHub](https://github.com/IceWhaleTech)

[![Star History Chart](https://api.star-history.com/svg?repos=IceWhaleTech/ZimaOS-Blue&type=Date)](https://star-history.com/#IceWhaleTech/ZimaOS-Blue&Date)

## Licence

Ce projet est sous licence MIT - voir le fichier [LICENSE](../../LICENSE) pour plus de détails. Nous croyons en l'open source et en redonnant à la communauté.

## Contributeurs

Merci à tous les contributeurs Blue :

<a href="https://community.vaunt.dev/board/IceWhaleTech/repository/ZimaOS-Blue">
  <img src="https://api.vaunt.dev/v1/github/entities/IceWhaleTech/repositories/ZimaOS-Blue/contributors?format=svg&limit=30" width="450" />
</a>

## Références

1. **OpenClaw** — Agent open source local. Pionnier de la connexion des LLM aux appareils locaux via des adaptateurs de canal et des appels d'outils, inspirant directement l'architecture d'exécution d'agent de Blue. https://github.com/openclaw/openclaw
2. **MiroMind** — Mode de recherche approfondie avec synthèse fondée sur des preuves. Nous avons façonné le pipeline de recherche approfondi intégré du Blue : planification, récupération parallèle, déduplication des preuves et génération de rapports HTML. https://www.miromind.ai
3. **Karpathy's LLM Wiki** — LLM comme compilateur de connaissances. Recadre les LLM pour créer des espaces de connaissances persistants et évolutifs, allant au-delà du piège d'accumulation de RAG.
4. **OpenSpace (HKUDS)** — Moteur de compétences à évolution automatique. Un cadre basé sur DAG dans lequel les agents apprennent des échecs et acquièrent des compétences spécialisées. https://github.com/HKUDS/OpenSpace
5. **Andrew Ng's Context Hub** — Registre de documentation d'API versionné pour les agents de codage. Résout les hallucinations des agents et les connaissances de session oubliées. Fournit des documents organisés et versionnés avec des boucles d'annotation et de rétroaction, transformant la documentation en une couche de connaissances auto-améliorable. https://github.com/andrewyng/context-hub
6. **Notion** — Simple, humain et intentionnellement silencieux. Inspiré par la philosophie minimaliste de Notion, Blue redonne de la chaleur à la grille. Là où les empattements raffinés rencontrent un design réfléchi, créant un espace qui ressemble à celui de la maison. https://www.notion.com/about
7. **Matrix** — Inspiration visuelle de l'esthétique emblématique de la pluie numérique. La direction esthétique des schémas techniques de Blue.
8. **IceWhale** — Amour, Mort et Robots S2E2 "Glace". Un collectif qui se rassemble dans le monde entier pour briser les murs des géants de l’Internet et résister à la concentration des données. La baleine de glace symbolise une communauté construisant ensemble des outils souverains au bord.
9. **ZimaOS Blue** — Amour, mort et robots S1E14 "Zima Blue". Une métaphore : l'intelligence qui commence au service et évolue pour explorer le monde. Blue est un agent de sagesse, enraciné dans la simplicité et cherchant la profondeur.
10. **ZimaOS** — Principes de conception simplifiés, ciblés et ouverts. ZimaOS et Blue partagent la conviction que la technologie doit servir l'utilisateur : déployer en 30 secondes, s'exécuter n'importe où, rester neutre vis-à-vis des fournisseurs. https://www.zimaspace.com/zimaos
