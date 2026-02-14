# ZimaOS Blue

<p align="center">
  <img src="../../docs/public/logo.png" alt="ZimaOS Blue" width="200">
</p>

<p align="center">
  <strong>Sigurno, Promatrano, Lokalno-Prvo AI Agent Runtime</strong>
</p>

<p align="center">
  <a href="../../README.md">English</a> |
  <a href="../zh_CN/README.md">简体中文</a> |
  <a href="../de_DE/README.md">Deutsch</a> |
  <a href="../fr_FR/README.md">Français</a> |
  <a href="../es_ES/README.md">Español</a> |
  <strong>Hrvatski</strong>
</p>

<p align="center">
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Blue/ci.yml?branch=main&style=for-the-badge" alt="CI status"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Blue?include_prereleases&style=for-the-badge" alt="GitHub release"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="MIT License"></a>
</p>

**ZimaOS Blue** je ojačano AI agent runtime okruženje za NAS i rubne uređaje. Vaši podaci ostaju na vašem hardveru, svaka radnja je revizijska, a AI operacije se izvode u izoliranim sandbox okruženjima.

[Brzi početak](#brzi-početak) · [Sigurnost](#sigurnosno-ojačanje)

## Temeljna načela

### Lokalno-Prvo

- **Suverenitet podataka**: Svi podaci pohranjeni lokalno na vašem NAS-u - bez ovisnosti o oblaku
- **Ollama integracija**: Pokrenite LLM-ove potpuno na uređaju bez vanjskih API poziva
- **Offline sposobnost**: Osnovne funkcionalnosti rade bez internetske veze
- **Jedna binarna datoteka**: ~15MB nativna Go binarna datoteka, bez runtime ovisnosti

### Promatrano i Revizijsko

- **Revizijski zapisi**: Svaka AI radnja zabilježena s punim kontekstom i vremenskim oznakama
- **Prometheus metrike**: Praćenje svih sistemskih operacija u stvarnom vremenu
- **pprof profiliranje**: Duboki uvid u CPU, memoriju i ponašanje gorutina
- **Strukturirani zapisi**: JSON zapisi za jednostavno parsiranje i upozorenja

### Sigurnosno ojačanje

- **Sandbox izvršavanje**: Svi pozivi alata izvode se u izoliranim okruženjima
- **RBAC**: Fino granulirana kontrola pristupa temeljena na ulogama
- **WebAuthn/Passkeys**: Autentifikacija bez lozinke FIDO2
- **MFA/TOTP**: Podrška za višefaktorsku autentifikaciju
- **OIDC/OAuth 2.0**: Integracija s enterprise SSO
- **Circuit Breaker**: Automatska izolacija kvarova sprječava kaskadne kvarove

## Brzi početak

```bash
# Iz izvora
git clone https://github.com/IceWhaleTech/ZimaOS-Blue.git
cd ZimaOS-Blue/server
go build -o zimaos-blue ./cmd/server
./zimaos-blue server
```

## Sigurnosno ojačanje

### Autentifikacijski stog

| Sloj | Tehnologija | Svrha |
|------|-------------|-------|
| Primarni | WebAuthn/Passkeys | Autentifikacija otporna na phishing |
| Sekundarni | TOTP/MFA | Jednokratne lozinke temeljene na vremenu |
| Enterprise | OIDC/OAuth 2.0 | SSO s Google, GitHub, Okta |
| Autorizacija | RBAC | Kontrola dozvola po resursu |

### Runtime zaštita

- **Sandbox izolacija**: Izvršavanje alata u ograničenim okruženjima
- **Ograničenje brzine**: API throttling po stanaru
- **Izolacija stanara**: Potpuna separacija podataka i resursa
- **Revizijski trag**: Nepromjenjivi zapisi svih privilegiranih operacija

## Arhitektura

```
┌─────────────────────────────────────────────────────┐
│                    ZimaOS Blue                       │
├─────────────────────────────────────────────────────┤
│  Revizijski zapis │ Metrike │ RBAC │ Rate Limiter  │
├─────────────────────────────────────────────────────┤
│              Sandbox izvršni sloj                    │
│         Izolacija alata │ Ograničenja resursa       │
├─────────────────────────────────────────────────────┤
│              Agent Runtime (Go)                      │
│  LLM Provider │ Alati │ Memorija │ Circuit Breaker │
├─────────────────────────────────────────────────────┤
│              Lokalni podatkovni sloj                 │
│  SQLite │ ECache │ Enkriptirano spremište           │
└─────────────────────────────────────────────────────┘
```

## Licenca

MIT Licenca - pogledajte [LICENSE](../../LICENSE) za detalje.

---

<p align="center">
  <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
