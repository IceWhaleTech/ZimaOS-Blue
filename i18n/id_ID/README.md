# ZimaOS Echo

<p align="center">
  <img src="../../docs/public/logo.png" alt="ZimaOS Echo" width="200">
</p>

<p align="center">
  <strong>Runtime Agen AI yang Aman dan Teramati</strong>
</p>

<p align="center">
  <a href="../../README.md">English</a> |
  <a href="../zh_CN/README.md">简体中文</a> |
  <a href="../zh_TW/README.md">繁體中文</a> |
  <a href="../ja_JP/README.md">日本語</a> |
  <a href="../ko_KR/README.md">한국어</a> |
  <a href="../de_DE/README.md">Deutsch</a> |
  <a href="../fr_FR/README.md">Français</a> |
  <a href="../es_ES/README.md">Español</a> |
  <a href="../it_IT/README.md">Italiano</a> |
  <a href="../pt_BR/README.md">Português</a> |
  <a href="../ru_RU/README.md">Русский</a> |
  <a href="../ar_SA/README.md">العربية</a> |
  <a href="../hi_IN/README.md">हिन्दी</a> |
  <a href="../th_TH/README.md">ไทย</a> |
  <a href="../vi_VN/README.md">Tiếng Việt</a> |
  <strong>Bahasa Indonesia</strong> |
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

**ZimaOS Echo** adalah runtime agen AI ringan dan berperforma tinggi yang dirancang untuk NAS dan perangkat edge. Dibangun dengan Go, menyediakan platform siap produksi dengan penyebaran tanpa konfigurasi, pemantauan sesi, dan analitik penggunaan.

[Mulai Cepat](#mulai-cepat) · [Fitur](#fitur-inti)

## Sorotan

| Spesifikasi | Nilai |
|------|------|
| **Ukuran Biner** | ~40 MB (satu file eksekusi) |
| **Memori (Menganggur)** | ~4 MB |
| **Waktu Mulai** | &lt; 1 s |
| **Dependensi** | Tidak ada (penyebaran tanpa konfigurasi) |

## Fitur Inti

### Penyebaran Tanpa Konfigurasi

- **Satu Biner**: unduh dan jalankan, tanpa dependensi runtime
- **Konfigurasi Sesuai Permintaan**: siap pakai, sesuaikan bila perlu
- **Lintas Platform**: Windows, macOS, Linux — biner sama, pengalaman sama
- **Dukungan Daemon**: jalankan sebagai layanan latar belakang

### Pemantauan Sesi

- **Pelacakan Sesi Waktu Nyata**: pantau semua sesi AI aktif dan status langsung
- **Riwayat Percakapan**: jejak audit lengkap semua interaksi
- **Putar Ulang Sesi**: tinjau dan analisis percakapan sebelumnya
- **Isolasi Multi-tenant**: pemisahan sesi penuh antar pengguna

### Optimisasi Rantai Panggilan

- **Pelacakan Permintaan**: visibilitas ujung ke ujung setiap panggilan API
- **Analisis Latensi**: identifikasi bottleneck di pipeline permintaan
- **Perutean Penyedia**: perutean cerdas ke penyedia LLM optimal
- **Circuit Breaker**: failover otomatis saat penyedia gagal

### Analitik Penggunaan

- **Konsumsi Token**: lacak penggunaan per pengguna, sesi, dan penyedia
- **Atribusi Biaya**: rincian biaya per operasi
- **Pembatasan Laju**: manajemen kuota per tenant
- **Ekspor Laporan**: hasilkan laporan penggunaan dalam berbagai format

### Pengerasan Keamanan

- **Eksekusi Sandbox**: semua panggilan alat berjalan di lingkungan terisolasi
- **RBAC**: kontrol akses berbasis peran yang halus
- **WebAuthn/Passkeys**: autentikasi FIDO2 tanpa kata sandi
- **MFA/TOTP**: autentikasi multi-faktor
- **Jejak Audit**: log abadi untuk semua operasi istimewa

## Mulai Cepat

```bash
# Dari Sumber
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo
make build && ./dist/zimaos-echo server
```

Akses dasbor di `http://localhost:3000`.

## Konfigurasi Penyedia LLM

ZimaOS Echo mendukung beberapa penyedia LLM termasuk layanan LLM lokal:

```yaml
llm:
  # Penyedia cloud
  provider: "openai"  # atau "anthropic", "azure", dll.
  api_key: "your-api-key"

  # LLM lokal (opsional)
  # provider: "ollama"
  # base_url: "http://localhost:11434"
```

## Arsitektur

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

## Observabilitas

```yaml
# Aktifkan stack observabilitas penuh
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

### Metrik yang Diekspos

- Latensi permintaan (p50, p95, p99)
- Penggunaan token LLM per penyedia
- Tingkat keberhasilan/kegagalan eksekusi alat
- Jumlah memori dan goroutine
- Transisi keadaan circuit breaker

## Pengaturan Pengembangan

### Prasyarat

| Alat | Versi | Instalasi |
|------|---------|---------|
| Go | 1.21+ | [golang.org](https://golang.org/dl/) |
| Node.js | 18+ | [nodejs.org](https://nodejs.org/) |
| Make | - | Terpasang di macOS/Linux |

### Mode Pengembangan (Hot Reload)

```bash
# Linux / macOS
./dev.sh

# Windows
dev.bat
```

- Frontend: `http://localhost:3000`
- Backend: `http://localhost:23456`

### Perintah Build

```bash
make build              # Biner tunggal (frontend tertanam)
make build-embedded     # Build dengan Claude Code CLI tertanam
make build-all          # Kompilasi silang untuk semua platform
make clean              # Bersihkan artefak build
```

### Struktur Proyek

```
ZimaOS-Echo/
├── server/             # Backend Go
│   ├── cmd/echo/       # Titik masuk
│   └── internal/       # Modul inti
├── web/                # Frontend Vue 3
│   └── src/
└── dist/               # Output build
```

## Ucapan Terima Kasih

- [clawdbot](https://github.com/clawdbot/clawdbot) – Inspirasi proyek
- [IceWhaleTech/zorm](https://github.com/IceWhaleTech/zorm) – ORM ringan

---

<p align="center">
  Made with ❤️ by <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
