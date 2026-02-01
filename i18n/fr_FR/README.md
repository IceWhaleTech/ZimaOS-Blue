# ZimaOS Echo

<p align="center">
  <img src="../../docs/public/logo.png" alt="ZimaOS Echo" width="200">
</p>

<p align="center">
  <strong>Runtime d’agent IA sécurisé et observable</strong>
</p>

<p align="center">
  <a href="../../README.md">English</a> |
  <a href="../zh_CN/README.md">简体中文</a> |
  <a href="../zh_TW/README.md">繁體中文</a> |
  <a href="../ja_JP/README.md">日本語</a> |
  <a href="../ko_KR/README.md">한국어</a> |
  <a href="../de_DE/README.md">Deutsch</a> |
  <strong>Français</strong> |
  <a href="../es_ES/README.md">Español</a> |
  <a href="../it_IT/README.md">Italiano</a> |
  <a href="../pt_BR/README.md">Português</a> |
  <a href="../ru_RU/README.md">Русский</a> |
  <a href="../ar_SA/README.md">العربية</a> |
  <a href="../hi_IN/README.md">हिन्दी</a> |
  <a href="../th_TH/README.md">ไทย</a> |
  <a href="../vi_VN/README.md">Tiếng Việt</a> |
  <a href="../id_ID/README.md">Bahasa Indonesia</a> |
  <a href="../tr_TR/README.md">Türkçe</a> |
  <a href="../pl_PL/README.md">Polski</a> |
  <a href="../nl_NL/README.md">Nederlands</a> |
  <a href="../sv_SE/README.md">Svenska</a>
</p>

<p align="center">
  <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Echo/ci.yml?branch=main&style=for-the-badge" alt="CI status"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Echo?include_prereleases&style=for-the-badge" alt="GitHub release"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="MIT License"></a>
</p>

**ZimaOS Echo** est un runtime d’agent IA léger et performant conçu pour les NAS et appareils edge. Développé en Go, il fournit une plateforme prête pour la production avec déploiement sans configuration, surveillance des sessions et analyses d’utilisation.

[Démarrage rapide](#démarrage-rapide) · [Fonctionnalités](#fonctionnalités-principales)

## Points forts

| Spécification | Valeur |
|------|------|
| **Taille du binaire** | ~40 Mo (exécutable unique) |
| **Mémoire (inactif)** | ~4 Mo |
| **Temps de démarrage** | < 1 s |
| **Dépendances** | Aucune (déploiement sans configuration) |

## Fonctionnalités principales

### Déploiement sans configuration

- **Binaire unique** : télécharger et exécuter, sans dépendance d’exécution
- **Configuration à la demande** : prêt à l’emploi, personnalisable si besoin
- **Multiplateforme** : Windows, macOS, Linux – même binaire, même expérience
- **Support démon** : exécution en service en arrière-plan

### Surveillance des sessions

- **Suivi en temps réel** : surveiller toutes les sessions IA actives et leur état
- **Historique des conversations** : traçabilité complète des interactions
- **Relecture de session** : revoir et analyser les conversations passées
- **Isolation multi-tenant** : séparation complète des sessions entre utilisateurs

### Optimisation de la chaîne d’appels

- **Traçage des requêtes** : visibilité de bout en bout de chaque appel API
- **Analyse de latence** : identifier les goulots d’étranglement
- **Routage des fournisseurs** : routage intelligent vers les fournisseurs LLM optimaux
- **Disjoncteur** : bascule automatique en cas de défaillance d’un fournisseur

### Analyses d’utilisation

- **Consommation de tokens** : suivi par utilisateur, session et fournisseur
- **Attribution des coûts** : détail des coûts par opération
- **Limitation de débit** : gestion des quotas par tenant
- **Export de rapports** : génération de rapports d’utilisation dans plusieurs formats

### Renforcement de la sécurité

- **Exécution en sandbox** : tous les appels d’outils s’exécutent dans des environnements isolés
- **RBAC** : contrôle d’accès fin basé sur les rôles
- **WebAuthn/Passkeys** : authentification FIDO2 sans mot de passe
- **MFA/TOTP** : authentification multi-facteurs
- **Journal d’audit** : journaux immuables de toutes les opérations privilégiées

## Démarrage rapide

```bash
# À partir des sources
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo
make build && ./dist/zimaos-echo server
```

Accéder au tableau de bord sur `http://localhost:3000`.

## Configuration des fournisseurs LLM

ZimaOS Echo prend en charge plusieurs fournisseurs LLM, y compris des services LLM locaux :

```yaml
llm:
  # Fournisseurs cloud
  provider: "openai"  # ou "anthropic", "azure", etc.
  api_key: "your-api-key"

  # LLM local (optionnel)
  # provider: "ollama"
  # base_url: "http://localhost:11434"
```

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                    ZimaOS Echo                       │
├─────────────────────────────────────────────────────┤
│  Session Monitor │ Usage Analytics │ Call Tracing  │
├─────────────────────────────────────────────────────┤
│  Audit Log  │  Metrics  │  RBAC  │  Rate Limiter   │
├─────────────────────────────────────────────────────┤
│              Sandbox Execution Layer                 │
│         Tool Isolation │ Resource Limits            │
├─────────────────────────────────────────────────────┤
│              Agent Runtime (Go)                      │
│  LLM Provider │ Tools │ Memory │ Circuit Breaker   │
├─────────────────────────────────────────────────────┤
│              Local Data Layer                        │
│  SQLite │ ECache │ Encrypted Storage                │
└─────────────────────────────────────────────────────┘
```

## Observabilité

```yaml
# Activer la pile d’observabilité complète
metrics:
  enabled: true
  endpoint: "/metrics"

profiling:
  enabled: true
  endpoint_prefix: "/debug/pprof"

audit:
  enabled: true
  retention_days: 90
```

### Métriques exposées

- Latence des requêtes (p50, p95, p99)
- Utilisation des tokens LLM par fournisseur
- Taux de succès/échec d’exécution des outils
- Compteurs mémoire et goroutines
- Transitions d’état du disjoncteur

## Environnement de développement

### Prérequis

| Outil | Version | Installation |
|------|---------|---------|
| Go | 1.21+ | [golang.org](https://golang.org/dl/) |
| Node.js | 18+ | [nodejs.org](https://nodejs.org/) |
| Make | - | Préinstallé sur macOS/Linux |

### Mode développement (rechargement à chaud)

```bash
# Linux / macOS
./dev.sh

# Windows
dev.bat
```

- Frontend : `http://localhost:3000`
- Backend : `http://localhost:23456`

### Commandes de build

```bash
make build              # Binaire unique (frontend intégré)
make build-embedded     # Build avec Claude Code CLI intégré
make build-all          # Cross-compilation pour toutes les plateformes
make clean              # Nettoyer les artefacts de build
```

### Structure du projet

```
ZimaOS-Echo/
├── server/             # Backend Go
│   ├── cmd/echo/       # Point d’entrée
│   └── internal/       # Modules principaux
├── web/                # Frontend Vue 3
│   └── src/
└── dist/               # Sortie de build
```

## Remerciements

- [clawdbot](https://github.com/clawdbot/clawdbot) – Inspiration du projet
- [IceWhaleTech/zorm](https://github.com/IceWhaleTech/zorm) – ORM léger

---

<p align="center">
  Made with ❤️ by <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
