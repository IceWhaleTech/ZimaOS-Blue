# ZimaOS Blue

<p align="center">
  <img src="../../docs/public/logo.png" alt="ZimaOS Blue" width="200">
</p>

<p align="center">
  <strong>സുരക്ഷിതം, നിരീക്ഷിക്കാവുന്നത്, ലോക്കൽ-ഫസ്റ്റ് AI ഏജന്റ് റൺടൈം</strong>
</p>

<p align="center">
  <a href="../../README.md">English</a> |
  <a href="../zh_CN/README.md">简体中文</a> |
  <a href="../hi_IN/README.md">हिन्दी</a> |
  <strong>മലയാളം</strong>
</p>

<p align="center">
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Blue/ci.yml?branch=main&style=for-the-badge" alt="CI status"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Blue?include_prereleases&style=for-the-badge" alt="GitHub release"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="MIT License"></a>
</p>

**ZimaOS Blue** NAS, എഡ്ജ് ഉപകരണങ്ങൾക്കായുള്ള ഒരു ശക്തമായ AI ഏജന്റ് റൺടൈം ആണ്. നിങ്ങളുടെ ഡാറ്റ നിങ്ങളുടെ ഹാർഡ്‌വെയറിൽ തന്നെ നിലനിൽക്കുന്നു, എല്ലാ പ്രവർത്തനങ്ങളും ഓഡിറ്റ് ചെയ്യാവുന്നതാണ്, AI പ്രവർത്തനങ്ങൾ ഐസൊലേറ്റഡ് സാൻഡ്‌ബോക്സുകളിൽ പ്രവർത്തിക്കുന്നു.

[ക്വിക്ക് സ്റ്റാർട്ട്](#ക്വിക്ക്-സ്റ്റാർട്ട്) · [സുരക്ഷ](#സുരക്ഷാ-ശക്തിപ്പെടുത്തൽ)

## അടിസ്ഥാന തത്വങ്ങൾ

### ലോക്കൽ-ഫസ്റ്റ്

- **ഡാറ്റ പരമാധികാരം**: എല്ലാ ഡാറ്റയും നിങ്ങളുടെ NAS-ൽ ലോക്കലായി സംഭരിച്ചിരിക്കുന്നു - ക്ലൗഡ് ആശ്രയമില്ല
- **Ollama ഇന്റഗ്രേഷൻ**: ബാഹ്യ API കോളുകളില്ലാതെ LLM-കൾ പൂർണ്ണമായും ഉപകരണത്തിൽ പ്രവർത്തിപ്പിക്കുക
- **ഓഫ്‌ലൈൻ കഴിവ്**: ഇന്റർനെറ്റ് കണക്റ്റിവിറ്റിയില്ലാതെ കോർ ഫങ്ഷണാലിറ്റി പ്രവർത്തിക്കുന്നു
- **സിംഗിൾ ബൈനറി**: ~15MB നേറ്റീവ് Go ബൈനറി, റൺടൈം ഡിപെൻഡൻസികളില്ല

### നിരീക്ഷിക്കാവുന്നതും ഓഡിറ്റ് ചെയ്യാവുന്നതും

- **ഓഡിറ്റ് ലോഗിംഗ്**: എല്ലാ AI പ്രവർത്തനങ്ങളും പൂർണ്ണ സന്ദർഭവും ടൈംസ്റ്റാമ്പുകളും ഉപയോഗിച്ച് ലോഗ് ചെയ്തിരിക്കുന്നു
- **Prometheus മെട്രിക്സ്**: എല്ലാ സിസ്റ്റം പ്രവർത്തനങ്ങളുടെയും റിയൽ-ടൈം മോണിറ്ററിംഗ്
- **pprof പ്രൊഫൈലിംഗ്**: CPU, മെമ്മറി, goroutine പെരുമാറ്റത്തിലേക്കുള്ള ആഴത്തിലുള്ള ദൃശ്യപരത
- **സ്ട്രക്ചേർഡ് ലോഗിംഗ്**: എളുപ്പത്തിലുള്ള പാഴ്‌സിംഗിനും അലേർട്ടിംഗിനുമുള്ള JSON ലോഗുകൾ

### സുരക്ഷാ ശക്തിപ്പെടുത്തൽ

- **സാൻഡ്‌ബോക്സ് എക്സിക്യൂഷൻ**: എല്ലാ ടൂൾ കോളുകളും ഐസൊലേറ്റഡ് എൻവയോൺമെന്റുകളിൽ പ്രവർത്തിക്കുന്നു
- **RBAC**: ഫൈൻ-ഗ്രെയ്ൻഡ് റോൾ-ബേസ്ഡ് ആക്സസ് കൺട്രോൾ
- **WebAuthn/Passkeys**: പാസ്‌വേഡ്‌ലെസ് FIDO2 ഓതന്റിക്കേഷൻ
- **MFA/TOTP**: മൾട്ടി-ഫാക്ടർ ഓതന്റിക്കേഷൻ സപ്പോർട്ട്
- **OIDC/OAuth 2.0**: എന്റർപ്രൈസ് SSO ഇന്റഗ്രേഷൻ
- **Circuit Breaker**: കാസ്കേഡ് പരാജയങ്ങൾ തടയുന്ന ഓട്ടോമാറ്റിക് ഫെയിലർ ഐസൊലേഷൻ

## ക്വിക്ക് സ്റ്റാർട്ട്

```bash
# സോഴ്സിൽ നിന്ന്
git clone https://github.com/IceWhaleTech/ZimaOS-Blue.git
cd ZimaOS-Blue/server
go build -o zimaos-blue ./cmd/server
./zimaos-blue server
```

## ആർക്കിടെക്ചർ

```
┌─────────────────────────────────────────────────────┐
│                    ZimaOS Blue                       │
├─────────────────────────────────────────────────────┤
│  ഓഡിറ്റ് ലോഗ് │ മെട്രിക്സ് │ RBAC │ Rate Limiter │
├─────────────────────────────────────────────────────┤
│              സാൻഡ്‌ബോക്സ് എക്സിക്യൂഷൻ ലെയർ         │
│         ടൂൾ ഐസൊലേഷൻ │ റിസോഴ്സ് ലിമിറ്റ്സ്        │
├─────────────────────────────────────────────────────┤
│              ഏജന്റ് റൺടൈം (Go)                      │
│  LLM Provider │ ടൂൾസ് │ മെമ്മറി │ Circuit Breaker │
├─────────────────────────────────────────────────────┤
│              ലോക്കൽ ഡാറ്റ ലെയർ                      │
│  SQLite │ ECache │ എൻക്രിപ്റ്റഡ് സ്റ്റോറേജ്        │
└─────────────────────────────────────────────────────┘
```

## ലൈസൻസ്

MIT ലൈസൻസ് - വിശദാംശങ്ങൾക്ക് [LICENSE](../../LICENSE) കാണുക.

---

<p align="center">
  <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
