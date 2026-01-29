# ZimaOS Echo

<p align="center">
  <img src="../../docs/public/logo.png" alt="ZimaOS Echo" width="200">
</p>

<p align="center">
  <strong>Runtime d’agents IA : sécurisé, observable, local-first</strong>
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

**ZimaOS Echo** est un runtime d’agents IA durci pour NAS et appareils edge. Les données restent sur votre matériel, chaque action est auditable, et l’IA s’exécute dans des sandboxes isolées.

[Documentation](https://echo.zimaos.com) · [Démarrage rapide](#démarrage-rapide) · [Fonctionnalités](#principes-fondamentaux) · [Comparaison](#comparaison-avec-clawdbot)

## Pourquoi ZimaOS Echo ?

ZimaOS Echo s’inspire de [clawdbot](https://github.com/clawdbot/clawdbot) et est reconstruit en Go pour :

- **Moins de ressources** : tourne sur des appareils avec seulement 256 Mo de RAM
- **Meilleures performances** : binaire Go natif, concurrence goroutines
- **Déploiement plus simple** : binaire unique, pas de Node.js
- **Optimisé NAS** : conçu pour du 24/7 sur matériel faible consommation

## Principes fondamentaux

### Local-first

- **Souveraineté des données** : tout stocké localement sur le NAS, pas de cloud
- **Intégration Ollama** : LLM entièrement on-device, zéro appel API externe
- **Fonctionnement hors ligne** : les fonctions principales marchent sans Internet
- **Binaire unique** : ~15 Mo en Go natif, sans dépendances runtime

### Observable et auditable

- **Audit logging** : chaque action IA enregistrée avec contexte et horodatage
- **Métriques Prometheus** : surveillance en temps réel des opérations
- **Profilage pprof** : visibilité sur CPU, mémoire, goroutines
- **Logs structurés** : JSON pour parsing et alertes

### Durcissement sécuritaire

- **Exécution en sandbox** : tous les appels d’outils en environnement isolé
- **RBAC** : contrôle d’accès à granularité fine
- **WebAuthn / Passkeys** : auth sans mot de passe FIDO2
- **MFA / TOTP** : authentification multifacteur
- **OIDC / OAuth 2.0** : SSO entreprise
- **Circuit breaker** : isolement automatique en cas de panne, limitation des cascades

## Démarrage rapide

```bash
# Linux / macOS
curl -fsSL https://echo.zimaos.com/install.sh | sudo bash

# Depuis les sources
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo/server
go build -o zimaos-echo ./cmd/server
./zimaos-echo server
```

## Durcissement sécuritaire

### Stack d’authentification

| Couche | Technologie | Rôle |
|--------|-------------|------|
| Primaire | WebAuthn / Passkeys | Auth sans mot de passe résistante au phishing |
| Secondaire | TOTP / MFA | Mots de passe à usage unique temporels |
| Entreprise | OIDC / OAuth 2.0 | SSO Google, GitHub, Okta, etc. |
| Autorisation | RBAC | Contrôle des droits par ressource |

### Protection runtime

- **Isolation sandbox** : exécution des outils en environnement restreint
- **Rate limiting** : limitation d’API par tenant
- **Isolation des tenants** : données et ressources séparées
- **Piste d’audit** : logs immuables des opérations privilégiées

### Résilience

- **Circuit breaker** : isolement du service en cas de défaillance
- **Dégradation gracieuse** : stratégies de repli si un fournisseur tombe
- **Chaîne de fallback LLM** : bascule automatique de fournisseur
- **Hot reload** : changements de config sans redémarrage

## Observabilité

```yaml
# Activer la stack d’observabilité complète
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
- Utilisation de tokens LLM par fournisseur
- Taux de succès / échec d’exécution des outils
- Compteurs mémoire et goroutines
- Transitions d’état du circuit breaker

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                    ZimaOS Echo                       │
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

## Configuration LLM local (Ollama)

Exécuter l’IA entièrement hors ligne, sans appels API externes :

```bash
# Installer Ollama
curl -fsSL https://ollama.com/install.sh | sh

# Télécharger un modèle
ollama pull llama3.2

# Configurer Echo pour le LLM local
cat >> config.yaml << EOF
llm:
  provider: "ollama"
  model: "llama3.2"
  base_url: "http://localhost:11434"
EOF
```

## Environnement de développement

### Prérequis

| Outil | Version | Installation |
|-------|---------|--------------|
| Go | 1.21+ | [golang.org](https://golang.org/dl/) |
| Node.js | 18+ | [nodejs.org](https://nodejs.org/) |
| Make | - | Inclus sur macOS / Linux |

### Démarrage en une commande

```bash
# Cloner et lancer
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo
make build && ./dist/zimaos-echo
```

Dashboard sur `http://localhost:3000`

### Mode développement (hot reload)

```bash
# Linux / macOS
./dev.sh

# Windows
dev.bat
```

- Frontend : `http://localhost:5173` (API proxyfiée vers le backend)
- Backend : `http://localhost:8080`

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
│   └── internal/       # Modules cœur
├── web/                # Frontend Vue 3
│   └── src/
└── dist/               # Sortie de build
```

## Comparaison avec Clawdbot

ZimaOS Echo est inspiré de clawdbot mais optimisé pour NAS/edge :

| Élément | ZimaOS Echo | Clawdbot |
|--------|-------------|----------|
| **Langage** | Go | TypeScript/Node.js |
| **Taille binaire** | ~15 Mo | ~200 Mo+ (avec node_modules) |
| **Mémoire** | ~80 Mo inactif | ~200 Mo+ inactif |
| **Démarrage** | < 1 s | 3–5 s |
| **Runtime** | Binaire natif | Node.js requis |
| **Plateforme** | NAS/Edge | Bureau/Serveur |

## Remerciements

- [clawdbot](https://github.com/clawdbot/clawdbot) - Inspiration du projet
- [IceWhaleTech/zorm](https://github.com/IceWhaleTech/zorm) - ORM léger

---

<p align="center">
  Made with ❤️ by <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
