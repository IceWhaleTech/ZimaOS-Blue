# ZimaOS Echo

<p align="center">
  <img src="../../docs/public/logo.png" alt="ZimaOS Echo" width="200">
</p>

<p align="center">
  <a href="../README.md">English</a> |
  <a href="../zh_CN/README.md">简体中文</a> |
  <a href="../en_GB/README.md">English (UK)</a> |
  <a href="../es_ES/README.md">Español (ES)</a> |
  <strong>Français</strong> |
  <a href="../de_DE/README.md">Deutsch</a> |
  <a href="../it_IT/README.md">Italiano</a> |
  <a href="../pt_PT/README.md">Português (PT)</a> |
  <a href="../pt_BR/README.md">Português (BR)</a> |
  <a href="../ru_RU/README.md">Русский</a> |
  <a href="../ja_JP/README.md">日本語</a> |
  <a href="../ko_KR/README.md">한국어</a> |
  <a href="../nl_NL/README.md">Nederlands</a> |
  <a href="../sv_SE/README.md">Svenska</a> |
  <a href="../da_DK/README.md">Dansk</a> |
  <a href="../fi_FI/README.md">Suomi</a> |
  <a href="../no_NO/README.md">Norsk</a> |
  <a href="../pl_PL/README.md">Polski</a> |
  <a href="../tr_TR/README.md">Türkçe</a> |
  <a href="../cs_CZ/README.md">Čeština</a> |
  <a href="../el_GR/README.md">Ελληνικά</a> |
  <a href="../ca_ES/README.md">Català</a> |
  <a href="../ga_IE/README.md">Gaeilge</a> |
  <a href="../ar_SA/README.md">العربية</a> |
  <a href="../hi_IN/README.md">हिन्दी</a> |
  <a href="../th_TH/README.md">ไทย</a> |
  <a href="../id_ID/README.md">Bahasa Indonesia</a> |
  <a href="../vi_VN/README.md">Tiếng Việt</a>
</p>

> Ceci est la version française du README de ZimaOS Echo.

Pour la documentation complète, rendez-vous sur : https://echo.zimaos.com  
Le reste de ce document suit la structure du README principal en anglais. Veuillez consulter `../README.md` pour les informations les plus récentes et détaillées.

# ZimaOS Echo

<p align="center">
  <img src="../../docs/public/logo.png" alt="ZimaOS Echo" width="200">
</p>

<p align="center">
  <strong>Environnement d'Exécution d'Agents Natif pour NAS</strong>
</p>

<p align="center">
  <a href="../../README.md">English</a> | <a href="../zh_CN/README.md">中文</a> | <strong>Français</strong>
</p>

<p align="center">
  <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Echo/ci.yml?branch=main&style=for-the-badge" alt="Statut CI"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Echo?include_prereleases&style=for-the-badge" alt="Version GitHub"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="Licence MIT"></a>
</p>

**ZimaOS Echo** est un environnement d'exécution d'agents IA léger et haute performance conçu spécifiquement pour les appareils NAS et edge. Construit avec Go, il fournit une plateforme prête pour la production pour exécuter des assistants IA sur du matériel à faible consommation.

[Documentation](https://echo.zimaos.com) · [Démarrage Rapide](#démarrage-rapide) · [Fonctionnalités](#fonctionnalités) · [Comparaison](#comparaison-avec-clawdbot)

## Pourquoi ZimaOS Echo ?

ZimaOS Echo est inspiré de [clawdbot](https://github.com/clawdbot/clawdbot) mais reconstruit de zéro en Go pour :

- **Utilisation Réduite des Ressources** : Fonctionne sur des appareils avec seulement 256 Mo de RAM
- **Meilleures Performances** : Binaire Go natif avec concurrence efficace basée sur les goroutines
- **Déploiement Plus Facile** : Un seul binaire, pas besoin de Node.js
- **Optimisation NAS** : Conçu pour un fonctionnement 24/7 sur des appareils à faible consommation

## Démarrage Rapide

### Linux / macOS

```bash
curl -fsSL https://echo.zimaos.com/install.sh | sudo bash
```

### Windows (PowerShell en tant qu'Administrateur)

```powershell
irm https://echo.zimaos.com/install.ps1 | iex
```

### Depuis le Code Source

```bash
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo/server
go build -o zimaos-echo ./cmd/server
./zimaos-echo server
```

## Fonctionnalités

### Fonctionnalités Principales

- 🚀 **Léger** : Un seul binaire < 15 Mo, mémoire < 80 Mo
- ⚡ **Haute Performance** : Basé sur Go avec concurrence de goroutines
- 🔌 **Multi-Fournisseur** : OpenAI, Anthropic, Ollama et plus
- 🛡️ **Prêt pour la Production** : Circuit breaker, dégradation élégante, récupération automatique
- 📊 **Observable** : Métriques Prometheus, profilage pprof, journalisation structurée
- 🔄 **Rechargement à Chaud** : Changements de configuration sans redémarrage
- 💾 **Sauvegarde/Restauration** : Sauvegarde automatisée avec restauration à un point dans le temps

### Intégration Maison Intelligente

- 🏠 **Home Assistant** : Intégration native avec l'API Home Assistant
- 💡 **Contrôle des Appareils** : Lumières, interrupteurs, capteurs, climat et plus
- 🤖 **Automatisation IA** : Commandes en langage naturel pour le contrôle de la maison intelligente
- 📡 **Événements en Temps Réel** : Abonnement aux changements d'état des appareils via WebSocket

### Capacités Vocales

- 🎤 **Reconnaissance Vocale** : Conversion parole-texte basée sur Whisper
- 🔊 **Synthèse Vocale** : Support de plusieurs moteurs TTS
- 👂 **Réveil Vocal** : Détection de mot d'activation personnalisable
- 🗣️ **Commandes Vocales** : Interaction mains libres avec l'assistant IA

### Architecture Multi-Locataire

- 👥 **Isolation des Locataires** : Isolation complète des données et des ressources
- 🔐 **Auth par Locataire** : Authentification indépendante par locataire
- 📊 **Quotas de Ressources** : Limites CPU, mémoire et taux API par locataire
- 🎛️ **Tableau de Bord Locataire** : Portail de gestion en libre-service

### Canaux de Communication

- 💬 **Protocole Matrix** : Messagerie décentralisée et chiffrée de bout en bout
- 📱 **Telegram/Discord/Slack** : Support des plateformes de messagerie populaires
- 📞 **Signal/WhatsApp** : Intégration de messagerie sécurisée
- 🍎 **iMessage** : Support natif iMessage pour macOS

### Sécurité et Authentification

- 🔑 **WebAuthn/Passkeys** : Authentification sans mot de passe avec FIDO2
- 🔐 **OIDC/OAuth 2.0** : SSO entreprise (Google, GitHub, Okta, etc.)
- 📲 **MFA/TOTP** : Support de l'authentification multifacteur
- 🛡️ **RBAC** : Contrôle d'accès basé sur les rôles détaillé
- 📝 **Journalisation d'Audit** : Piste d'audit de sécurité complète
- 🔒 **Sandbox** : Environnement d'exécution isolé pour les outils

### Frontend

- 🎨 **Tableau de Bord Vue 3** : Interface web moderne et responsive
- 💬 **Interface de Chat** : Réponses en streaming avec support Markdown
- 📈 **Moniteur Système** : Graphiques d'utilisation des ressources en temps réel
- ⚙️ **UI de Configuration** : Gestion de configuration facile

## Comparaison avec Clawdbot

ZimaOS Echo est inspiré de clawdbot mais optimisé pour le déploiement NAS/edge :

| Fonctionnalité | ZimaOS Echo | Clawdbot |
|----------------|-------------|----------|
| **Langage** | Go | TypeScript/Node.js |
| **Taille du Binaire** | ~15 Mo | ~200 Mo+ (avec node_modules) |
| **Utilisation Mémoire** | ~80 Mo au repos | ~200 Mo+ au repos |
| **Temps de Démarrage** | < 1s | 3-5s |
| **Runtime** | Binaire natif | Node.js requis |
| **Plateforme Cible** | Appareils NAS/Edge | Bureau/Serveur |

## Architecture

```
┌─────────────────────────────────────────────────┐
│                  ZimaOS-Echo                     │
├─────────────────────────────────────────────────┤
│  Frontend Vue 3  │  REST API  │  WebSocket      │
├─────────────────────────────────────────────────┤
│              Runtime Principal (Go)              │
│  Boucle d'Événements │ Pool de Workers │ Config │ Logger │
├─────────────────────────────────────────────────┤
│              Runtime Agent                       │
│  Fournisseur LLM │ Outils │ Mémoire │ Contexte  │
├─────────────────────────────────────────────────┤
│              Couche de Données                   │
│  SQLite (Zorm) │ ECache │ Fichiers              │
└─────────────────────────────────────────────────┘
```

## Feuille de Route

- [x] **v0.1.0** - Runtime Principal (Boucle d'événements, Pool de workers, Config, Logger)
- [x] **v0.2.0** - Runtime Agent (Fournisseurs LLM incl. AWS Bedrock, Outils, Mémoire)
- [x] **v0.3.0** - Couche API (REST, WebSocket, Streaming)
- [x] **v0.4.0** - Système de Plugins (Modules Go, support WASM)
- [x] **v0.5.0** - Prêt pour la Production (Métriques, Profilage, Sauvegarde)
- [x] **v0.6.0** - Canaux de Messagerie (Telegram, Discord, Slack, WhatsApp, Signal, iMessage)
- [x] **v0.7.0** - Sécurité (OIDC, MFA, WebAuthn, Journalisation d'audit, Sandbox)
- [x] **v0.8.0** - Performance (ECache, Zorm, HTTP/2)
- [x] **v0.9.0** - Améliorations Futures (A2UI, Automatisation du Navigateur)
- [ ] **v1.0.0** - RAG et Base de Connaissances

## Contribuer

Les contributions sont les bienvenues ! Veuillez lire notre [Guide de Contribution](CONTRIBUTING.md) pour plus de détails.

```bash
# Cloner le dépôt
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo

# Installer les dépendances
cd server && go mod download

# Exécuter les tests
go test ./...

# Compiler
go build -o zimaos-echo ./cmd/server
```

## Licence

Licence MIT - voir [LICENSE](LICENSE) pour plus de détails.

## Remerciements

- [clawdbot](https://github.com/clawdbot/clawdbot) - Inspiration pour le projet
- [IceWhaleTech/zorm](https://github.com/IceWhaleTech/zorm) - ORM léger

---

<p align="center">
  Fait avec ❤️ par <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
