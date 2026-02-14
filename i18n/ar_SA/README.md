# ZimaOS Blue

<p align="center">
  <img src="../../docs/public/logo.png" alt="ZimaOS Blue" width="200">
</p>

<p align="center">
  <strong>بيئة تشغيل وكيل الذكاء الاصطناعي الآمنة والقابلة للمراقبة</strong>
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
  <strong>العربية</strong> |
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
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Blue/ci.yml?branch=main&style=for-the-badge" alt="CI status"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Blue?include_prereleases&style=for-the-badge" alt="GitHub release"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="MIT License"></a>
</p>

**ZimaOS Blue** هو بيئة تشغيل وكيل ذكاء اصطناعي خفيفة وعالية الأداء مصممة لأجهزة NAS والحافة. مبنية بلغة Go، توفر منصة جاهزة للإنتاج مع نشر بدون إعداد ومراقبة الجلسات وتحليلات الاستخدام.

[البدء السريع](#البدء-السريع) · [الميزات](#الميزات-الأساسية)

## أبرز المواصفات

| المواصفة | القيمة |
|------|------|
| **حجم الملف التنفيذي** | ~40 ميجابايت (ملف تنفيذي واحد) |
| **الذاكرة (خمول)** | ~4 ميجابايت |
| **وقت التشغيل** | &lt; 1 ثانية |
| **التبعيات** | لا يوجد (نشر بدون إعداد) |

## الميزات الأساسية

### نشر بدون إعداد

- **ملف تنفيذي واحد**: تنزيل وتشغيل، بدون تبعيات وقت التشغيل
- **إعداد عند الطلب**: يعمل مباشرة، قابل للتخصيص عند الحاجة
- **متعدد المنصات**: Windows و macOS و Linux — نفس الملف، نفس التجربة
- **دعم الخدمة الخلفية**: التشغيل كخدمة خلفية مستمرة

### مراقبة الجلسات

- **تتبع الجلسات في الوقت الفعلي**: مراقبة جميع جلسات الذكاء الاصطناعي النشطة وحالتها
- **سجل المحادثات**: أثر تدقيق كامل لجميع التفاعلات
- **إعادة تشغيل الجلسات**: مراجعة وتحليل المحادثات السابقة
- **عزل متعدد المستأجرين**: فصل كامل للجلسات بين المستخدمين

### تحسين سلسلة الاستدعاءات

- **تتبع الطلبات**: رؤية من طرف لطرف لكل استدعاء API
- **تحليل زمن الاستجابة**: تحديد الاختناقات في خط الطلبات
- **توجيه المزودين**: توجيه ذكي إلى مزودي LLM الأمثلين
- **قاطع الدائرة**: تبديل تلقائي عند فشل المزود

### تحليلات الاستخدام

- **استهلاك الرموز**: تتبع الاستخدام لكل مستخدم وجلسة ومزود
- **إسناد التكلفة**: تفصيل التكلفة حسب العملية
- **حد المعدل**: إدارة الحصص لكل مستأجر
- **تصدير التقارير**: إنشاء تقارير الاستخدام بصيغ متعددة

### تعزيز الأمان

- **التنفيذ في بيئة معزولة**: جميع استدعاءات الأدوات تعمل في بيئات معزولة
- **RBAC**: تحكم دقيق في الوصول بناءً على الأدوار
- **WebAuthn/Passkeys**: مصادقة FIDO2 بدون كلمة مرور
- **MFA/TOTP**: مصادقة متعددة العوامل
- **سجل التدقيق**: سجلات ثابتة لجميع العمليات المميزة

## البدء السريع

```bash
# من المصدر
git clone https://github.com/IceWhaleTech/ZimaOS-Blue.git
cd ZimaOS-Blue
make build && ./dist/zimaos-blue server
```

الوصول إلى لوحة التحكم على `http://localhost:3000`.

## إعداد مزودي LLM

يدعم ZimaOS Blue عدة مزودي LLM بما في ذلك خدمات LLM المحلية:

```yaml
llm:
  # مزودو السحابة
  provider: "openai"  # أو "anthropic", "azure" إلخ
  api_key: "your-api-key"

  # LLM محلي (اختياري)
  # provider: "ollama"
  # base_url: "http://localhost:11434"
```

## البنية

```
┌─────────────────────────────────────────────────────┐
│                    ZimaOS Blue                       │
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

## إمكانية المراقبة

```yaml
# تفعيل مجموعة المراقبة الكاملة
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

### المقاييس المعروضة

- زمن استجابة الطلبات (p50, p95, p99)
- استخدام رموز LLM لكل مزود
- معدلات نجاح/فشل تنفيذ الأدوات
- عدد الذاكرة والـ goroutines
- انتقالات حالة قاطع الدائرة

## بيئة التطوير

### المتطلبات

| الأداة | الإصدار | التثبيت |
|------|---------|---------|
| Go | 1.21+ | [golang.org](https://golang.org/dl/) |
| Node.js | 18+ | [nodejs.org](https://nodejs.org/) |
| Make | - | مثبت مسبقاً على macOS/Linux |

### وضع التطوير (إعادة تحميل ساخنة)

```bash
# Linux / macOS
./dev.sh

# Windows
dev.bat
```

- الواجهة الأمامية: `http://localhost:3000`
- الخلفية: `http://localhost:23456`

### أوامر البناء

```bash
make build              # ملف تنفيذي واحد (واجهة مدمجة)
make build-embedded     # بناء مع Claude Code CLI مدمج
make build-all          # تجميع عبر المنصات
make clean              # تنظيف مخرجات البناء
```

### هيكل المشروع

```
ZimaOS-Blue/
├── server/             # خلفية Go
│   ├── cmd/blue/       # نقطة الدخول
│   └── internal/       # الوحدات الأساسية
├── web/                # واجهة Vue 3
│   └── src/
└── dist/               # مخرجات البناء
```

## الشكر

- [clawdbot](https://github.com/clawdbot/clawdbot) — مصدر إلهام المشروع
- [IceWhaleTech/zorm](https://github.com/IceWhaleTech/zorm) — ORM خفيف

---

<p align="center">
  Made with ❤️ by <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
