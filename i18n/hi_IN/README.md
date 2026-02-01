# ZimaOS Echo

<p align="center">
  <img src="../../docs/public/logo.png" alt="ZimaOS Echo" width="200">
</p>

<p align="center">
  <strong>सुरक्षित, निरीक्षणयोग्य AI एजेंट रनटाइम</strong>
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
  <strong>हिन्दी</strong> |
  <a href="../th_TH/README.md">ไทย</a> |
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

**ZimaOS Echo** NAS और एज डिवाइस के लिए बनाया गया हल्का, उच्च-प्रदर्शन AI एजेंट रनटाइम है। Go में बना, यह शून्य-कॉन्फ़िग डिप्लॉयमेंट, सत्र निगरानी और उपयोग विश्लेषण के साथ प्रोडक्शन-तैयार प्लेटफ़ॉर्म देता है।

[त्वरित शुरुआत](#त्वरित-शुरुआत) · [सुविधाएँ](#मुख्य-सुविधाएँ)

## मुख्य बिंदु

| विशिष्टता | मान |
|------|------|
| **बाइनरी आकार** | ~40 MB (एकल निष्पादन योग्य) |
| **मेमोरी (निष्क्रिय)** | ~4 MB |
| **स्टार्टअप समय** | &lt; 1 s |
| **निर्भरताएँ** | कोई नहीं (शून्य-कॉन्फ़िग डिप्लॉयमेंट) |

## मुख्य सुविधाएँ

### शून्य-कॉन्फ़िग डिप्लॉयमेंट

- **एकल बाइनरी**: डाउनलोड करें और चलाएँ — रनटाइम निर्भरता की ज़रूरत नहीं
- **जरूरत पर कॉन्फ़िग**: बॉक्स से तैयार, जरूरत पड़ने पर अनुकूलित करें
- **क्रॉस-प्लेटफ़ॉर्म**: Windows, macOS, Linux — एक ही बाइनरी, एक ही अनुभव
- **डेमन सपोर्ट**: पृष्ठभूमि सेवा के रूप में चलाने योग्य

### सत्र निगरानी

- **रीयल-टाइम सत्र ट्रैकिंग**: सभी सक्रिय AI सत्रों और लाइव स्थिति की निगरानी
- **वार्तालाप इतिहास**: सभी इंटरैक्शन की पूर्ण ऑडिट ट्रेल
- **सत्र रिप्ले**: पिछली वार्तालापों की समीक्षा और विश्लेषण
- **मल्टी-टेनेंट आइसोलेशन**: उपयोगकर्ताओं के बीच पूर्ण सत्र पृथक्करण

### कॉल चेन ऑप्टिमाइज़ेशन

- **अनुरोध ट्रेसिंग**: हर API कॉल की एंड-टू-एंड दृश्यता
- **लेटेंसी विश्लेषण**: अनुरोध पाइपलाइन में बॉटलनेक पहचानें
- **प्रोवाइडर रूटिंग**: इष्टतम LLM प्रोवाइडरों तक स्मार्ट रूटिंग
- **सर्किट ब्रेकर**: प्रोवाइडर विफलता पर ऑटो फेलओवर

### उपयोग विश्लेषण

- **टोकन खपत**: प्रति उपयोगकर्ता, सत्र और प्रोवाइडर उपयोग ट्रैक करें
- **लागत आवंटन**: ऑपरेशन के अनुसार विस्तृत लागत विभाजन
- **रेट लिमिटिंग**: प्रति टेनेंट कोटा प्रबंधन
- **रिपोर्ट एक्सपोर्ट**: कई फॉर्मैट में उपयोग रिपोर्ट जनरेट करें

### सुरक्षा सख्ती

- **सैंडबॉक्स एक्ज़ीक्यूशन**: सभी टूल कॉल अलग वातावरण में चलते हैं
- **RBAC**: महीन भूमिका-आधारित एक्सेस कंट्रोल
- **WebAuthn/Passkeys**: पासवर्ड-रहित FIDO2 प्रमाणीकरण
- **MFA/TOTP**: मल्टी-फैक्टर प्रमाणीकरण
- **ऑडिट ट्रेल**: सभी विशेषाधिकार ऑपरेशनों की अपरिवर्तनीय लॉग

## त्वरित शुरुआत

```bash
# सोर्स से
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo
make build && ./dist/zimaos-echo server
```

डैशबोर्ड पर `http://localhost:3000` से पहुँचें।

## LLM प्रोवाइडर कॉन्फ़िगरेशन

ZimaOS Echo कई LLM प्रोवाइडरों को सपोर्ट करता है, जिसमें लोकल LLM सेवाएँ शामिल हैं:

```yaml
llm:
  # क्लाउड प्रोवाइडर
  provider: "openai"  # या "anthropic", "azure" आदि
  api_key: "your-api-key"

  # लोकल LLM (वैकल्पिक)
  # provider: "ollama"
  # base_url: "http://localhost:11434"
```

## आर्किटेक्चर

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

## निरीक्षणीयता

```yaml
# पूर्ण निरीक्षण स्टैक सक्षम करें
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

### एक्सपोज़ मेट्रिक्स

- अनुरोध लेटेंसी (p50, p95, p99)
- प्रोवाइडर के अनुसार LLM टोकन उपयोग
- टूल एक्ज़ीक्यूशन सफलता/विफलता दरें
- मेमोरी और गोरूटीन काउंट
- सर्किट ब्रेकर स्टेट ट्रांजिशन

## डेवलपमेंट सेटअप

### आवश्यकताएँ

| टूल | संस्करण | इंस्टॉल |
|------|---------|---------|
| Go | 1.21+ | [golang.org](https://golang.org/dl/) |
| Node.js | 18+ | [nodejs.org](https://nodejs.org/) |
| Make | - | macOS/Linux पर प्री-इंस्टॉल |

### डेवलपमेंट मोड (हॉट रीलोड)

```bash
# Linux / macOS
./dev.sh

# Windows
dev.bat
```

- फ्रंटएंड: `http://localhost:3000`
- बैकएंड: `http://localhost:23456`

### बिल्ड कमांड

```bash
make build              # एकल बाइनरी (फ्रंटएंड एम्बेडेड)
make build-embedded     # Claude Code CLI एम्बेडेड के साथ बिल्ड
make build-all          # सभी प्लेटफ़ॉर्म के लिए क्रॉस-कंपाइल
make clean              # बिल्ड आर्टिफैक्ट साफ़ करें
```

### प्रोजेक्ट संरचना

```
ZimaOS-Echo/
├── server/             # Go बैकएंड
│   ├── cmd/echo/       # एंट्री पॉइंट
│   └── internal/       # कोर मॉड्यूल
├── web/                # Vue 3 फ्रंटएंड
│   └── src/
└── dist/               # बिल्ड आउटपुट
```

## आभार

- [clawdbot](https://github.com/clawdbot/clawdbot) — प्रोजेक्ट प्रेरणा
- [IceWhaleTech/zorm](https://github.com/IceWhaleTech/zorm) — हल्का ORM

---

<p align="center">
  Made with ❤️ by <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
