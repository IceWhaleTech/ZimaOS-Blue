# ZimaOS Echo

<p align="center">
  <img src="../../docs/public/logo.png" alt="ZimaOS Echo" width="200">
</p>

<p align="center">
  <strong>รันไทม์เอเจนต์ AI ที่ปลอดภัยและสังเกตได้</strong>
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
  <strong>ไทย</strong> |
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

**ZimaOS Echo** เป็นรันไทม์เอเจนต์ AI ที่เบาและประสิทธิภาพสูง ออกแบบมาสำหรับ NAS และอุปกรณ์เอจ สร้างด้วย Go ให้แพลตฟอร์มพร้อมใช้งานจริงพร้อมการติดตั้งแบบไม่ต้องตั้งค่า การตรวจสอบเซสชัน และการวิเคราะห์การใช้งาน

[เริ่มต้นอย่างรวดเร็ว](#เริ่มต้นอย่างรวดเร็ว) · [ฟีเจอร์](#ฟีเจอร์หลัก)

## ไฮไลท์

| รายการ | ค่า |
|------|------|
| **ขนาดไบนารี** | ~40 MB (ไฟล์ปฏิบัติการเดียว) |
| **หน่วยความจำ (ว่าง)** | ~4 MB |
| **เวลาเริ่มต้น** | &lt; 1 วินาที |
| **การพึ่งพา** | ไม่มี (การติดตั้งแบบไม่ต้องตั้งค่า) |

## ฟีเจอร์หลัก

### การติดตั้งแบบไม่ต้องตั้งค่า

- **ไบนารีเดียว**: ดาวน์โหลดและรัน ไม่ต้องพึ่งพารันไทม์
- **ตั้งค่าเมื่อต้องการ**: ใช้ได้ทันที ปรับแต่งเมื่อจำเป็น
- **ข้ามแพลตฟอร์ม**: Windows, macOS, Linux — ไบนารีเดียวกัน ประสบการณ์เดียวกัน
- **รองรับเดมอน**: รันเป็นบริการพื้นหลังได้

### การตรวจสอบเซสชัน

- **ติดตามเซสชันแบบเรียลไทม์**: ตรวจสอบเซสชัน AI ที่ใช้งานทั้งหมดและสถานะสด
- **ประวัติการสนทนา**: บันทึกการตรวจสอบครบทุกการโต้ตอบ
- **เล่นซ้ำเซสชัน**: ดูและวิเคราะห์การสนทนาในอดีต
- **แยกหลายผู้เช่า**: แยกเซสชันระหว่างผู้ใช้อย่างสมบูรณ์

### การปรับสายโซ่การเรียก

- **ติดตามคำขอ**: มองเห็นจากต้นทางถึงปลายทางทุกการเรียก API
- **วิเคราะห์ความหน่วง**: หาจุดคอขวดในไปป์ไลน์คำขอ
- **การกำหนดเส้นทางผู้ให้บริการ**: กำหนดเส้นทางอัจฉริยะไปยังผู้ให้บริการ LLM ที่เหมาะสม
- **เซอร์กิตเบรกเกอร์**: สลับอัตโนมัติเมื่อผู้ให้บริการล้มเหลว

### การวิเคราะห์การใช้งาน

- **การใช้โทเค็น**: ติดตามการใช้งานต่อผู้ใช้ เซสชัน และผู้ให้บริการ
- **การจัดสรรต้นทุน**: แยกรายละเอียดต้นทุนต่อการดำเนินการ
- **จำกัดอัตรา**: จัดการโควต้าต่อผู้เช่า
- **ส่งออกรายงาน**: สร้างรายงานการใช้งานหลายรูปแบบ

### การเสริมความปลอดภัย

- **รันในแซนด์บ็อกซ์**: การเรียกเครื่องมือทั้งหมดรันในสภาพแวดล้อมแยก
- **RBAC**: การควบคุมการเข้าถึงตามบทบาทแบบละเอียด
- **WebAuthn/Passkeys**: การยืนยันตัวตน FIDO2 แบบไม่ใช้รหัสผ่าน
- **MFA/TOTP**: การยืนยันตัวตนหลายปัจจัย
- **บันทึกการตรวจสอบ**: บันทึกที่ไม่เปลี่ยนได้ของการดำเนินการที่มีสิทธิ์ทั้งหมด

## เริ่มต้นอย่างรวดเร็ว

```bash
# จากซอร์ส
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo
make build && ./dist/zimaos-echo server
```

เข้าแดชบอร์ดที่ `http://localhost:3000`

## การตั้งค่าผู้ให้บริการ LLM

ZimaOS Echo รองรับผู้ให้บริการ LLM หลายราย รวมถึงบริการ LLM ในเครื่อง:

```yaml
llm:
  # ผู้ให้บริการคลาวด์
  provider: "openai"  # หรือ "anthropic", "azure" เป็นต้น
  api_key: "your-api-key"

  # LLM ในเครื่อง (ทางเลือก)
  # provider: "ollama"
  # base_url: "http://localhost:11434"
```

## สถาปัตยกรรม

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

## การสังเกตได้

```yaml
# เปิดสแต็กการสังเกตแบบเต็ม
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

### เมตริกที่เปิดเผย

- ความหน่วงคำขอ (p50, p95, p99)
- การใช้โทเค็น LLM ต่อผู้ให้บริการ
- อัตราความสำเร็จ/ล้มเหลวการรันเครื่องมือ
- นับหน่วยความจำและ goroutine
- การเปลี่ยนสถานะเซอร์กิตเบรกเกอร์

## การตั้งค่าเพื่อพัฒนา

### ความต้องการ

| เครื่องมือ | เวอร์ชัน | การติดตั้ง |
|------|---------|---------|
| Go | 1.21+ | [golang.org](https://golang.org/dl/) |
| Node.js | 18+ | [nodejs.org](https://nodejs.org/) |
| Make | - | ติดตั้งมากับ macOS/Linux |

### โหมดพัฒนา (โหลดซ้ำแบบร้อน)

```bash
# Linux / macOS
./dev.sh

# Windows
dev.bat
```

- ฟรอนต์เอนด์: `http://localhost:3000`
- แบ็กเอนด์: `http://localhost:23456`

### คำสั่งบิลด์

```bash
make build              # ไบนารีเดียว (ฟรอนต์เอนด์ฝัง)
make build-embedded     # บิลด์พร้อม Claude Code CLI ฝัง
make build-all          # คอมไพล์ข้ามแพลตฟอร์มทั้งหมด
make clean              # ล้างอาร์ติแฟกต์บิลด์
```

### โครงสร้างโปรเจกต์

```
ZimaOS-Echo/
├── server/             # แบ็กเอนด์ Go
│   ├── cmd/echo/       # จุดเข้า
│   └── internal/       # โมดูลหลัก
├── web/                # ฟรอนต์เอนด์ Vue 3
│   └── src/
└── dist/               # ผลลัพธ์บิลด์
```

## ขอบคุณ

- [clawdbot](https://github.com/clawdbot/clawdbot) — แรงบันดาลใจของโปรเจกต์
- [IceWhaleTech/zorm](https://github.com/IceWhaleTech/zorm) — ORM เบา

---

<p align="center">
  Made with ❤️ by <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
