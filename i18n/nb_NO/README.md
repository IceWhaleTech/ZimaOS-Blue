# ZimaOS Blue

<p align="center">
  <img src="../../docs/public/logo.png" alt="ZimaOS Blue" width="200">
</p>

<p align="center">
  <strong>Sikker, Observerbar, Lokal-Først AI Agent Runtime</strong>
</p>

<p align="center">
  <a href="../../README.md">English</a> |
  <a href="../zh_CN/README.md">简体中文</a> |
  <a href="../de_DE/README.md">Deutsch</a> |
  <a href="../sv_SE/README.md">Svenska</a> |
  <a href="../da_DK/README.md">Dansk</a> |
  <strong>Norsk Bokmål</strong>
</p>

<p align="center">
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Blue/ci.yml?branch=main&style=for-the-badge" alt="CI status"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Blue?include_prereleases&style=for-the-badge" alt="GitHub release"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="MIT License"></a>
</p>

**ZimaOS Blue** er en herdet AI-agent runtime for NAS og edge-enheter. Dataene dine forblir på din maskinvare, hver handling er reviderbar, og AI-operasjoner kjører i isolerte sandkasser.

[Hurtigstart](#hurtigstart) · [Sikkerhet](#sikkerhetsherding)

## Kjerneprinsipper

### Lokal-Først

- **Datasuverenitet**: All data lagret lokalt på din NAS - ingen skyavhengighet
- **Ollama-integrasjon**: Kjør LLM-er helt på enheten uten eksterne API-kall
- **Offline-kapabel**: Kjernefunksjonalitet fungerer uten internettilkobling
- **Enkelt binærfil**: ~15MB native Go-binær, ingen runtime-avhengigheter

### Observerbar og Reviderbar

- **Revisjonslogging**: Hver AI-handling logget med full kontekst og tidsstempler
- **Prometheus-metrikker**: Sanntidsovervåking av alle systemoperasjoner
- **pprof-profilering**: Dyp innsikt i CPU, minne og goroutine-oppførsel
- **Strukturert logging**: JSON-logger for enkel parsing og varsling

### Sikkerhetsherding

- **Sandkasse-kjøring**: Alle verktøykall kjører i isolerte miljøer
- **RBAC**: Finkornet rollebasert tilgangskontroll
- **WebAuthn/Passkeys**: Passordløs FIDO2-autentisering
- **MFA/TOTP**: Støtte for flerfaktorautentisering
- **OIDC/OAuth 2.0**: Enterprise SSO-integrasjon
- **Circuit Breaker**: Automatisk feilisolering forhindrer kaskadefeil

## Hurtigstart

```bash
# Fra kilde
git clone https://github.com/IceWhaleTech/ZimaOS-Blue.git
cd ZimaOS-Blue/server
go build -o zimaos-blue ./cmd/server
./zimaos-blue server
```

## Sikkerhetsherding

### Autentiseringsstabel

| Lag | Teknologi | Formål |
|-----|-----------|--------|
| Primær | WebAuthn/Passkeys | Phishing-resistent passordløs autentisering |
| Sekundær | TOTP/MFA | Tidsbaserte engangspassord |
| Enterprise | OIDC/OAuth 2.0 | SSO med Google, GitHub, Okta |
| Autorisasjon | RBAC | Per-ressurs tillatelseskontroll |

### Runtime-beskyttelse

- **Sandkasse-isolasjon**: Verktøykjøring i begrensede miljøer
- **Hastighetsbegrensning**: Per-leietaker API-throttling
- **Leietaker-isolasjon**: Fullstendig data- og ressursseparasjon
- **Revisjonsspor**: Uforanderlige logger over alle privilegerte operasjoner

## Arkitektur

```
┌─────────────────────────────────────────────────────┐
│                    ZimaOS Blue                       │
├─────────────────────────────────────────────────────┤
│  Revisjonslogg │ Metrikker │ RBAC │ Rate Limiter   │
├─────────────────────────────────────────────────────┤
│              Sandkasse-kjøringslag                   │
│         Verktøyisolasjon │ Ressursgrenser          │
├─────────────────────────────────────────────────────┤
│              Agent Runtime (Go)                      │
│  LLM Provider │ Verktøy │ Minne │ Circuit Breaker  │
├─────────────────────────────────────────────────────┤
│              Lokalt datalag                          │
│  SQLite │ ECache │ Kryptert lagring                │
└─────────────────────────────────────────────────────┘
```

## Lisens

MIT-lisens - se [LICENSE](../../LICENSE) for detaljer.

---

<p align="center">
  <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
