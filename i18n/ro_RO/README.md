# ZimaOS Echo

<p align="center">
  <img src="../../docs/public/logo.png" alt="ZimaOS Echo" width="200">
</p>

<p align="center">
  <strong>Securizat, Observabil, Local-First AI Agent Runtime</strong>
</p>

<p align="center">
  <a href="../../README.md">English</a> |
  <a href="../zh_CN/README.md">简体中文</a> |
  <a href="../de_DE/README.md">Deutsch</a> |
  <a href="../fr_FR/README.md">Français</a> |
  <a href="../it_IT/README.md">Italiano</a> |
  <strong>Română</strong>
</p>

<p align="center">
  <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Echo/ci.yml?branch=main&style=for-the-badge" alt="CI status"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Echo?include_prereleases&style=for-the-badge" alt="GitHub release"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="MIT License"></a>
</p>

**ZimaOS Echo** este un runtime AI agent întărit pentru NAS și dispozitive edge. Datele tale rămân pe hardware-ul tău, fiecare acțiune este auditabilă, iar operațiunile AI rulează în sandbox-uri izolate.

[Documentație](https://echo.zimaos.com) · [Start rapid](#start-rapid) · [Securitate](#întărire-securitate)

## Principii fundamentale

### Local-First

- **Suveranitatea datelor**: Toate datele stocate local pe NAS-ul tău - fără dependență de cloud
- **Integrare Ollama**: Rulează LLM-uri complet pe dispozitiv fără apeluri API externe
- **Capabil offline**: Funcționalitatea de bază funcționează fără conexiune la internet
- **Binar unic**: ~15MB binar Go nativ, fără dependențe runtime

### Observabil și Auditabil

- **Jurnalizare audit**: Fiecare acțiune AI înregistrată cu context complet și timestamp-uri
- **Metrici Prometheus**: Monitorizare în timp real a tuturor operațiunilor sistemului
- **Profilare pprof**: Vizibilitate profundă în comportamentul CPU, memorie și goroutine
- **Jurnalizare structurată**: Jurnale JSON pentru parsare și alertare ușoară

### Întărire securitate

- **Execuție sandbox**: Toate apelurile de instrumente rulează în medii izolate
- **RBAC**: Control acces bazat pe roluri cu granularitate fină
- **WebAuthn/Passkeys**: Autentificare FIDO2 fără parolă
- **MFA/TOTP**: Suport autentificare multi-factor
- **OIDC/OAuth 2.0**: Integrare SSO enterprise
- **Circuit Breaker**: Izolarea automată a eșecurilor previne eșecurile în cascadă

## Start rapid

```bash
# Linux / macOS
curl -fsSL https://echo.zimaos.com/install.sh | sudo bash

# Din sursă
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo/server
go build -o zimaos-echo ./cmd/server
./zimaos-echo server
```

## Întărire securitate

### Stiva de autentificare

| Strat | Tehnologie | Scop |
|-------|------------|------|
| Primar | WebAuthn/Passkeys | Autentificare fără parolă rezistentă la phishing |
| Secundar | TOTP/MFA | Parole unice bazate pe timp |
| Enterprise | OIDC/OAuth 2.0 | SSO cu Google, GitHub, Okta |
| Autorizare | RBAC | Control permisiuni per resursă |

### Protecție runtime

- **Izolare sandbox**: Execuția instrumentelor în medii restricționate
- **Limitare rată**: Throttling API per chiriaș
- **Izolare chiriaș**: Separare completă a datelor și resurselor
- **Urmă audit**: Jurnale imuabile ale tuturor operațiunilor privilegiate

## Arhitectură

```
┌─────────────────────────────────────────────────────┐
│                    ZimaOS Echo                       │
├─────────────────────────────────────────────────────┤
│  Jurnal audit │ Metrici │ RBAC │ Rate Limiter      │
├─────────────────────────────────────────────────────┤
│              Strat execuție sandbox                  │
│         Izolare instrumente │ Limite resurse        │
├─────────────────────────────────────────────────────┤
│              Agent Runtime (Go)                      │
│  LLM Provider │ Instrumente │ Memorie │ Circuit Breaker│
├─────────────────────────────────────────────────────┤
│              Strat date local                        │
│  SQLite │ ECache │ Stocare criptată                │
└─────────────────────────────────────────────────────┘
```

## Licență

Licență MIT - vezi [LICENSE](../../LICENSE) pentru detalii.

---

<p align="center">
  <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
