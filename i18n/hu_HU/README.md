# ZimaOS Echo

<p align="center">
  <img src="../../docs/public/logo.png" alt="ZimaOS Echo" width="200">
</p>

<p align="center">
  <strong>Biztonságos, Megfigyelhető, Helyi-Első AI Ügynök Futtatókörnyezet</strong>
</p>

<p align="center">
  <a href="../../README.md">English</a> |
  <a href="../zh_CN/README.md">简体中文</a> |
  <a href="../de_DE/README.md">Deutsch</a> |
  <a href="../fr_FR/README.md">Français</a> |
  <a href="../es_ES/README.md">Español</a> |
  <strong>Magyar</strong>
</p>

<p align="center">
  <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Echo/ci.yml?branch=main&style=for-the-badge" alt="CI status"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Echo?include_prereleases&style=for-the-badge" alt="GitHub release"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="MIT License"></a>
</p>

**ZimaOS Echo** egy megerősített AI ügynök futtatókörnyezet NAS és peremeszközök számára. Az adatai a saját hardverén maradnak, minden művelet auditálható, és az AI műveletek izolált sandbox környezetekben futnak.

[Gyors kezdés](#gyors-kezdés) · [Biztonság](#biztonsági-megerősítés)

## Alapelvek

### Helyi-Első

- **Adatszuverenitás**: Minden adat helyben tárolva a NAS-on - nincs felhőfüggőség
- **Ollama integráció**: LLM-ek futtatása teljesen az eszközön, nulla külső API hívással
- **Offline képesség**: Az alapfunkciók internetkapcsolat nélkül is működnek
- **Egyetlen bináris**: ~15MB natív Go bináris, nincs futtatókörnyezet-függőség

### Megfigyelhető és Auditálható

- **Audit naplózás**: Minden AI művelet naplózva teljes kontextussal és időbélyegekkel
- **Prometheus metrikák**: Valós idejű megfigyelés minden rendszerműveletről
- **pprof profilozás**: Mély betekintés a CPU, memória és goroutine viselkedésbe
- **Strukturált naplózás**: JSON naplók az egyszerű elemzéshez és riasztáshoz

### Biztonsági megerősítés

- **Sandbox végrehajtás**: Minden eszközhívás izolált környezetben fut
- **RBAC**: Finomhangolt szerepalapú hozzáférés-vezérlés
- **WebAuthn/Passkeys**: Jelszó nélküli FIDO2 hitelesítés
- **MFA/TOTP**: Többfaktoros hitelesítés támogatás
- **OIDC/OAuth 2.0**: Vállalati SSO integráció
- **Circuit Breaker**: Automatikus hibaizoláció megakadályozza a kaszkád hibákat

## Gyors kezdés

```bash
# Forrásból
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo/server
go build -o zimaos-echo ./cmd/server
./zimaos-echo server
```

## Biztonsági megerősítés

### Hitelesítési verem

| Réteg | Technológia | Cél |
|-------|-------------|-----|
| Elsődleges | WebAuthn/Passkeys | Adathalászat-ellenálló jelszó nélküli hitelesítés |
| Másodlagos | TOTP/MFA | Időalapú egyszer használatos jelszavak |
| Vállalati | OIDC/OAuth 2.0 | SSO Google, GitHub, Okta szolgáltatásokkal |
| Jogosultság | RBAC | Erőforrásonkénti jogosultság-vezérlés |

### Futásidejű védelem

- **Sandbox izoláció**: Eszközvégrehajtás korlátozott környezetekben
- **Sebességkorlátozás**: Bérlőnkénti API szabályozás
- **Bérlő izoláció**: Teljes adat- és erőforrás-szeparáció
- **Audit nyomvonal**: Minden privilegizált művelet megváltoztathatatlan naplói

## Architektúra

```
┌─────────────────────────────────────────────────────┐
│                    ZimaOS Echo                       │
├─────────────────────────────────────────────────────┤
│  Audit napló │ Metrikák │ RBAC │ Rate Limiter      │
├─────────────────────────────────────────────────────┤
│              Sandbox végrehajtási réteg              │
│         Eszköz izoláció │ Erőforrás korlátok       │
├─────────────────────────────────────────────────────┤
│              Ügynök futtatókörnyezet (Go)           │
│  LLM Provider │ Eszközök │ Memória │ Circuit Breaker│
├─────────────────────────────────────────────────────┤
│              Helyi adatréteg                         │
│  SQLite │ ECache │ Titkosított tárolás              │
└─────────────────────────────────────────────────────┘
```

## Licenc

MIT Licenc - részletekért lásd [LICENSE](../../LICENSE).

---

<p align="center">
  <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
