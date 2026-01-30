# ZimaOS Echo

<p align="center">
  <img src="../../docs/public/logo.png" alt="ZimaOS Echo" width="200">
</p>

<p align="center">
  <strong>Runtime agent AI an toàn, có thể quan sát</strong>
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
  <strong>Tiếng Việt</strong> |
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

**ZimaOS Echo** là runtime agent AI nhẹ, hiệu năng cao dành cho NAS và thiết bị biên. Xây bằng Go, cung cấp nền tảng sẵn sàng production với triển khai không cấu hình, giám sát phiên và phân tích sử dụng.

[Bắt đầu nhanh](#bắt-đầu-nhanh) · [Tính năng](#tính-năng-chính)

## Điểm nổi bật

| Thông số | Giá trị |
|------|------|
| **Kích thước binary** | ~40 MB (một file thực thi) |
| **Bộ nhớ (nhàn rỗi)** | ~4 MB |
| **Thời gian khởi động** | &lt; 1 s |
| **Phụ thuộc** | Không (triển khai không cấu hình) |

## Tính năng chính

### Triển khai không cấu hình

- **Một binary**: tải và chạy, không cần runtime
- **Cấu hình khi cần**: dùng ngay, tùy chỉnh khi cần
- **Đa nền tảng**: Windows, macOS, Linux — cùng binary, cùng trải nghiệm
- **Hỗ trợ daemon**: chạy như dịch vụ nền

### Giám sát phiên

- **Theo dõi phiên thời gian thực**: giám sát mọi phiên AI đang hoạt động và trạng thái
- **Lịch sử hội thoại**: nhật ký kiểm toán đầy đủ mọi tương tác
- **Phát lại phiên**: xem và phân tích hội thoại trước
- **Cách ly đa tenant**: tách biệt phiên hoàn toàn giữa người dùng

### Tối ưu chuỗi gọi

- **Truy vết yêu cầu**: nhìn thấy đầu-cuối mọi lần gọi API
- **Phân tích độ trễ**: xác định điểm nghẽn trong pipeline yêu cầu
- **Định tuyến nhà cung cấp**: định tuyến thông minh tới nhà cung cấp LLM tối ưu
- **Circuit breaker**: chuyển dự phòng tự động khi nhà cung cấp lỗi

### Phân tích sử dụng

- **Tiêu thụ token**: theo dõi theo người dùng, phiên và nhà cung cấp
- **Phân bổ chi phí**: chi tiết chi phí theo thao tác
- **Giới hạn tốc độ**: quản lý hạn ngạch theo tenant
- **Xuất báo cáo**: tạo báo cáo sử dụng nhiều định dạng

### Tăng cường bảo mật

- **Chạy trong sandbox**: mọi lệnh gọi công cụ chạy trong môi trường cô lập
- **RBAC**: kiểm soát truy cập theo vai trò chi tiết
- **WebAuthn/Passkeys**: xác thực FIDO2 không mật khẩu
- **MFA/TOTP**: xác thực đa yếu tố
- **Nhật ký kiểm toán**: log bất biến mọi thao tác đặc quyền

## Bắt đầu nhanh

```bash
# Từ mã nguồn
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo
make build && ./dist/zimaos-echo server
```

Truy cập bảng điều khiển tại `http://localhost:3000`.

## Cấu hình nhà cung cấp LLM

ZimaOS Echo hỗ trợ nhiều nhà cung cấp LLM, bao gồm dịch vụ LLM local:

```yaml
llm:
  # Nhà cung cấp đám mây
  provider: "openai"  # hoặc "anthropic", "azure", v.v.
  api_key: "your-api-key"

  # LLM local (tùy chọn)
  # provider: "ollama"
  # base_url: "http://localhost:11434"
```

## Kiến trúc

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

## Khả năng quan sát

```yaml
# Bật stack quan sát đầy đủ
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

### Metric hiển thị

- Độ trễ yêu cầu (p50, p95, p99)
- Sử dụng token LLM theo nhà cung cấp
- Tỷ lệ thành công/thất bại thực thi công cụ
- Số lượng bộ nhớ và goroutine
- Chuyển trạng thái circuit breaker

## Môi trường phát triển

### Yêu cầu

| Công cụ | Phiên bản | Cài đặt |
|------|---------|---------|
| Go | 1.21+ | [golang.org](https://golang.org/dl/) |
| Node.js | 18+ | [nodejs.org](https://nodejs.org/) |
| Make | - | Có sẵn trên macOS/Linux |

### Chế độ phát triển (hot reload)

```bash
# Linux / macOS
./dev.sh

# Windows
dev.bat
```

- Frontend: `http://localhost:3000`
- Backend: `http://localhost:8080`

### Lệnh build

```bash
make build              # Một binary (frontend nhúng)
make build-embedded     # Build kèm Claude Code CLI nhúng
make build-all          # Cross-compile mọi nền tảng
make clean              # Xóa artifact build
```

### Cấu trúc dự án

```
ZimaOS-Echo/
├── server/             # Backend Go
│   ├── cmd/echo/       # Điểm vào
│   └── internal/       # Module lõi
├── web/                # Frontend Vue 3
│   └── src/
└── dist/               # Kết quả build
```

## Cảm ơn

- [clawdbot](https://github.com/clawdbot/clawdbot) — Nguồn cảm hứng cho dự án
- [IceWhaleTech/zorm](https://github.com/IceWhaleTech/zorm) — ORM nhẹ

---

<p align="center">
  Made with ❤️ by <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
