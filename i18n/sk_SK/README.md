# ZimaOS Blue

<p align="center">
  <img src="../../docs/public/logo.png" alt="ZimaOS Blue" width="200">
</p>

<p align="center">
  <strong>Bezpečný, Pozorovateľný, Local-First AI Agent Runtime</strong>
</p>

<p align="center">
  <a href="../../README.md">English</a> |
  <a href="../zh_CN/README.md">简体中文</a> |
  <a href="../de_DE/README.md">Deutsch</a> |
  <a href="../cs_CZ/README.md">Čeština</a> |
  <a href="../pl_PL/README.md">Polski</a> |
  <strong>Slovenčina</strong>
</p>

<p align="center">
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Blue/ci.yml?branch=main&style=for-the-badge" alt="CI status"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Blue?include_prereleases&style=for-the-badge" alt="GitHub release"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="MIT License"></a>
</p>

**ZimaOS Blue** je posilnený AI agent runtime pre NAS a edge zariadenia. Vaše dáta zostávajú na vašom hardvéri, každá akcia je auditovateľná a AI operácie bežia v izolovaných sandboxoch.

[Rýchly štart](#rýchly-štart) · [Bezpečnosť](#bezpečnostné-posilnenie)

## Základné princípy

### Local-First

- **Suverenita dát**: Všetky dáta uložené lokálne na vašom NAS - žiadna závislosť na cloude
- **Ollama integrácia**: Spustite LLM úplne na zariadení bez externých API volaní
- **Offline schopnosť**: Základná funkcionalita funguje bez internetového pripojenia
- **Jeden binárny súbor**: ~15MB natívny Go binárny súbor, žiadne runtime závislosti

### Pozorovateľný a Auditovateľný

- **Audit logovanie**: Každá AI akcia zaznamenaná s plným kontextom a časovými značkami
- **Prometheus metriky**: Monitorovanie všetkých systémových operácií v reálnom čase
- **pprof profilovanie**: Hlboký pohľad do správania CPU, pamäte a goroutín
- **Štruktúrované logovanie**: JSON logy pre jednoduché parsovanie a upozorňovanie

### Bezpečnostné posilnenie

- **Sandbox vykonávanie**: Všetky volania nástrojov bežia v izolovaných prostrediach
- **RBAC**: Jemne granulovaná kontrola prístupu založená na rolách
- **WebAuthn/Passkeys**: Bezheslová FIDO2 autentifikácia
- **MFA/TOTP**: Podpora viacfaktorovej autentifikácie
- **OIDC/OAuth 2.0**: Enterprise SSO integrácia
- **Circuit Breaker**: Automatická izolácia zlyhaní zabraňuje kaskádovým zlyhaniam

## Rýchly štart

```bash
# Zo zdrojového kódu
git clone https://github.com/IceWhaleTech/ZimaOS-Blue.git
cd ZimaOS-Blue/server
go build -o zimaos-blue ./cmd/server
./zimaos-blue server
```

## Bezpečnostné posilnenie

### Autentifikačný zásobník

| Vrstva | Technológia | Účel |
|--------|-------------|------|
| Primárna | WebAuthn/Passkeys | Phishing-odolná bezheslová autentifikácia |
| Sekundárna | TOTP/MFA | Časovo založené jednorazové heslá |
| Enterprise | OIDC/OAuth 2.0 | SSO s Google, GitHub, Okta |
| Autorizácia | RBAC | Kontrola oprávnení per zdroj |

### Runtime ochrana

- **Sandbox izolácia**: Vykonávanie nástrojov v obmedzených prostrediach
- **Obmedzenie rýchlosti**: API throttling per nájomník
- **Izolácia nájomníkov**: Kompletná separácia dát a zdrojov
- **Audit stopa**: Nemenné záznamy všetkých privilegovaných operácií

## Architektúra

```
┌─────────────────────────────────────────────────────┐
│                    ZimaOS Blue                       │
├─────────────────────────────────────────────────────┤
│  Audit log │ Metriky │ RBAC │ Rate Limiter         │
├─────────────────────────────────────────────────────┤
│              Sandbox vykonávacia vrstva              │
│         Izolácia nástrojov │ Limity zdrojov        │
├─────────────────────────────────────────────────────┤
│              Agent Runtime (Go)                      │
│  LLM Provider │ Nástroje │ Pamäť │ Circuit Breaker │
├─────────────────────────────────────────────────────┤
│              Lokálna dátová vrstva                   │
│  SQLite │ ECache │ Šifrované úložisko              │
└─────────────────────────────────────────────────────┘
```

## Licencia

MIT Licencia - pozri [LICENSE](../../LICENSE) pre detaily.

---

<p align="center">
  <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
