# ZimaOS Echo

<p align="center">
  <img src="../../docs/public/logo.png" alt="ZimaOS Echo" width="200">
</p>

<p align="center">
  <strong>Güvenli, Gözlemlenebilir AI Ajan Çalışma Zamanı</strong>
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
  <a href="../id_ID/README.md">Bahasa Indonesia</a> |
  <strong>Türkçe</strong> |
  <a href="../pl_PL/README.md">Polski</a> |
  <a href="../nl_NL/README.md">Nederlands</a> |
  <a href="../sv_SE/README.md">Svenska</a>
</p>

<p align="center">
  <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Echo/ci.yml?branch=main&style=for-the-badge" alt="CI status"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Echo?include_prereleases&style=for-the-badge" alt="GitHub release"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="MIT License"></a>
</p>

**ZimaOS Echo**, NAS ve kenar cihazlar için tasarlanmış hafif, yüksek performanslı bir AI ajan çalışma zamanıdır. Go ile yazılmış olup, sıfır yapılandırma dağıtımı, oturum izleme ve kullanım analitiği sunan üretime hazır bir platform sağlar.

[Hızlı Başlangıç](#hızlı-başlangıç) · [Özellikler](#temel-özellikler) · [Dağıtım Modları](#dağıtım-modları)

## Öne Çıkanlar

| Özellik | Değer |
|------|------|
| **İkili Boyut** | ~40 MB (tek çalıştırılabilir) |
| **Bellek (Boşta)** | ~4 MB |
| **Başlangıç Süresi** | &lt; 1 s |
| **Bağımlılıklar** | Yok (sıfır yapılandırma dağıtımı) |

## Temel Özellikler

### Sıfır Yapılandırma Dağıtımı

- **Tek İkili**: İndir ve çalıştır, çalışma zamanı bağımlılığı yok
- **İhtiyaç Halinde Yapılandırma**: Kutudan çalışır, gerektiğinde özelleştir
- **Çapraz Platform**: Windows, macOS, Linux — aynı ikili, aynı deneyim
- **Arka Plan Hizmeti Desteği**: Sürekli arka plan hizmeti olarak çalıştırılabilir

### Oturum İzleme

- **Gerçek Zamanlı Oturum Takibi**: Tüm aktif AI oturumlarını ve canlı durumunu izleyin
- **Konuşma Geçmişi**: Tüm etkileşimlerin tam denetim izi
- **Oturum Tekrarı**: Geçmiş konuşmaları inceleyin ve analiz edin
- **Çok Kiracılı İzolasyon**: Kullanıcılar arasında tam oturum ayrımı

### Çağrı Zinciri Optimizasyonu

- **İstek İzleme**: Her API çağrısının uçtan uca görünürlüğü
- **Gecikme Analizi**: İstek hattındaki darboğazları tespit edin
- **Sağlayıcı Yönlendirme**: Optimal LLM sağlayıcılarına akıllı yönlendirme
- **Devre Kesici**: Sağlayıcı arızalarında otomatik yedekleme

### Kullanım Analitiği

- **Token Tüketimi**: Kullanıcı, oturum ve sağlayıcı bazında kullanım takibi
- **Maliyet Ataması**: İşlem bazında ayrıntılı maliyet dağılımı
- **Hız Sınırlama**: Kiracı bazında kota yönetimi
- **Rapor Dışa Aktarma**: Birden fazla formatta kullanım raporları oluşturma

### Güvenlik Sertleştirme

- **Kum Havuzu Çalıştırma**: Tüm araç çağrıları izole ortamlarda çalışır
- **RBAC**: İnce taneli rol tabanlı erişim kontrolü
- **WebAuthn/Passkeys**: Şifresiz FIDO2 kimlik doğrulama
- **MFA/TOTP**: Çok faktörlü kimlik doğrulama
- **Denetim İzi**: Tüm ayrıcalıklı işlemlerin değiştirilemez günlükleri

## Hızlı Başlangıç

```bash
# Kaynak Koddan
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo
make build && ./dist/zimaos-echo server
```

Kontrol paneline `http://localhost:3000` adresinden erişin.

## LLM Sağlayıcı Yapılandırması

ZimaOS Echo, yerel LLM hizmetleri dahil birden fazla LLM sağlayıcısını destekler:

```yaml
llm:
  # Bulut sağlayıcıları
  provider: "openai"  # veya "anthropic", "azure" vb.
  api_key: "your-api-key"

  # Yerel LLM (isteğe bağlı)
  # provider: "ollama"
  # base_url: "http://localhost:11434"
```

## Mimari

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

## Gözlemlenebilirlik

```yaml
# Tam gözlemlenebilirlik yığınını etkinleştir
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

### Açığa Çıkan Metrikler

- İstek gecikmesi (p50, p95, p99)
- Sağlayıcı başına LLM token kullanımı
- Araç çalıştırma başarı/başarısızlık oranları
- Bellek ve goroutine sayıları
- Devre kesici durum geçişleri

## Geliştirme Ortamı

### Ön Koşullar

| Araç | Sürüm | Kurulum |
|------|---------|---------|
| Go | 1.21+ | [golang.org](https://golang.org/dl/) |
| Node.js | 18+ | [nodejs.org](https://nodejs.org/) |
| Make | - | macOS/Linux'ta önceden yüklü |

### Geliştirme Modu (Sıcak Yeniden Yükleme)

```bash
# Linux / macOS
./dev.sh

# Windows
dev.bat
```

- Ön Uç: `http://localhost:3000`
- Arka Uç: `http://localhost:23456`

### Derleme Komutları

```bash
make build              # Tek ikili (ön uç gömülü)
make build-embedded     # Claude Code CLI gömülü derleme
make build-all          # Tüm platformlar için çapraz derleme
make clean              # Derleme çıktılarını temizle
```

### Proje Yapısı

```
ZimaOS-Echo/
├── server/             # Go arka ucu
│   ├── cmd/echo/       # Giriş noktası
│   └── internal/       # Çekirdek modüller
├── web/                # Vue 3 ön ucu
│   └── src/
└── dist/               # Derleme çıktısı
```

## Teşekkürler

- [clawdbot](https://github.com/clawdbot/clawdbot) – Proje ilham kaynağı
- [IceWhaleTech/zorm](https://github.com/IceWhaleTech/zorm) – Hafif ORM

---

<p align="center">
  Made with ❤️ by <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
