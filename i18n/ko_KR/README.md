![](../../docs/assets/banner.png)

<p align="center">
  대담한 빌더를 위한 <strong>로컬 우선</strong> 에이전트 런타임<br>
  즉시 사용 · 오픈소스 · 범용 · 이중 감독
</p>

<p align="center">
  <a href="../../README.md">English</a> |
  <a href="../ca_ES/README.md">Català</a> |
  <a href="../cs_CZ/README.md">Čeština</a> |
  <a href="../da_DK/README.md">Dansk</a> |
  <a href="../de_DE/README.md">Deutsch</a> |
  <a href="../el_GR/README.md">Ελληνικά</a> |
  <a href="../en_GB/README.md">English (UK)</a> |
  <a href="../es_ES/README.md">Español</a> |
  <a href="../fr_FR/README.md">Français</a> |
  <a href="../ga_IE/README.md">Gaeilge</a> |
  <a href="../hr_HR/README.md">Hrvatski</a> |
  <a href="../hu_HU/README.md">Magyar</a> |
  <a href="../it_IT/README.md">Italiano</a> |
  <a href="../ja_JP/README.md">日本語</a> |
  <strong>한국어</strong> |
  <a href="../ml_IN/README.md">മലയാളം</a> |
  <a href="../nb_NO/README.md">Norsk Bokmål</a> |
  <a href="../nl_NL/README.md">Nederlands</a> |
  <a href="../pl_PL/README.md">Polski</a> |
  <a href="../pt_BR/README.md">Português (BR)</a> |
  <a href="../pt_PT/README.md">Português (PT)</a> |
  <a href="../ro_RO/README.md">Română</a> |
  <a href="../ru_RU/README.md">Русский</a> |
  <a href="../sk_SK/README.md">Slovenčina</a> |
  <a href="../sv_SE/README.md">Svenska</a> |
  <a href="../zh_CN/README.md">简体中文</a> |
  <a href="../zh_TW/README.md">繁體中文</a>
</p>

<p align="center">
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Blue/ci.yml?branch=main&style=for-the-badge" alt="CI status"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Blue?include_prereleases&style=for-the-badge" alt="GitHub release"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="MIT License"></a>
</p>

<p align="center">
  <a href="https://discord.gg/SrCYvumF"><img src="../../docs/assets/discord.png" alt="Discord" height="128" /></a>&nbsp;&nbsp;
  <a href="https://www.facebook.com/zimaboard/"><img src="../../docs/assets/facebook.png" alt="Facebook" height="128" /></a>&nbsp;&nbsp;
  <a href="https://x.com/ZimaSpace"><img src="../../docs/assets/x.png" alt="X" height="128" /></a>
</p>

## 소개

Clawdbot에서 영감을 받아, 우리는 개인 컴퓨팅의 **미래**가 엣지에서 실행되는 **다양한 로컬 우선 AI 에이전트**에 의해 **형성될 것**이라고 믿습니다.

**ZimaOS Blue는 우리의 해답입니다** — 완전히 **오픈소스이며, 감사 가능하고, 프로덕션에 바로 사용할 수 있는 에이전트 런타임 및 툴킷**으로, 프라이빗하고 셀프 호스팅되는 에이전트를 마찰 없이 배포할 수 있게 해줍니다.

자신만의 에이전트를 **바이브 코딩하거나 직접 만들고 싶은** 대담한 개발자를 위해 만들어진 Blue는 **성능에 최적화**되어 있습니다: **Go**로 작성되었으며, 메모리 사용량은 최소 10 MB입니다. **모든 x86, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS** — 전원만 연결하면 어디서든 실행됩니다.

![](../../docs/assets/features.png)

## 주요 특징

### 로컬 우선 설계 & 자동 모델 접근

한 걸음 더 나아갑니다: **20개 이상의 IM 플랫폼** 네이티브 지원, 자연스럽고 맥락을 인식하는 대화를 위한 **음성 기반** 인터페이스, IDE 스캔을 통한 **설정 없는 모델 전환**, 그리고 SOUL 레이어 퍼스널리티를 제공합니다.

<p align="center">
  <img src="../../docs/assets/providers.png" alt="Supported Providers" />
</p>

### 빠르고 가벼움

Go로 네이티브 컴파일 — 인터프리터 없음, VM 없음, 오버헤드 없음. 서버부터 데스크톱 기기까지 조용히 실행됩니다.

| 지표 | ZimaOS Blue (Go) | OpenClaw (Node + dist) |
|--------|-------------------|------------------------|
| `--help` 콜드 / 웜 | **0.18 s / < 0.01 s** | 3.31 s / ~1.11 s |
| `status` 런타임 (3회 최고) | **< 0.01 s** | 5.98 s |
| `--help` 최대 RSS | **~10 MB** | ~394 MB |
| `status` 최대 RSS | **~15 MB** | ~1.52 GB |
| 런타임 의존성 | **없음** | Node.js 18+ |

> 벤치마크 환경: macOS arm64 (서버 모드, 데스크톱 UI 없음), 동일 호스트, 3회 최고 기록. 2026년 2월.

### 순수 Go, 모든 기기

100% Go, 정적 바이너리. **5개 타겟으로 크로스 컴파일** 기본 지원 (![linux](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png) amd64/arm64, ![macOS](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png) amd64/arm64, ![windows](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png) amd64). Node 런타임 불필요, Python 불필요, 컨테이너 불필요. NAS, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, 오래된 x86 라우터, 또는 Mac에 놓기만 하면 바로 실행됩니다. **그 위에 자신만의 UI, 로직, 에이전트 스킬을 쌓으세요** — 하나의 코드베이스, 모든 플랫폼.

### 보안 & 거버넌스

심층 방어가 내장된 사이드카 API 프록시:
- **샌드박스 실행** – 모든 도구 호출은 격리된 환경에서 실행됩니다.
- **프롬프트 인젝션 방어** – 7개 이상의 내장 차단 전략.
- **세션 감사** – 전체 세션 모니터링, 모든 상호작용 추적 가능.
- **RBAC & WebAuthn** – 비밀번호 없는 인증과 세밀한 접근 제어.

## Blue를 선택하는 이유

우리는 **차세대 개인 컴퓨팅**이 LLM을 수용하지만 — **제어 가능하고 감사 가능한** 에이전트가 개인과 팀 모두에게 기반이 된다고 믿습니다. **Blue가 제공하는 것**:
- **포괄적인 코어** – 고급 모델 관리, IM 통합, 향상된 페르소나, 일상적인 상호작용(헤드셋, 음성, 스마트 글래스)에 최적화된 자연어 인터페이스.
- **로컬 우선, 초경량, 크로스 디바이스** – 고사양 하드웨어 불필요. 연산이 가능한 모든 기기에서 실행.
- **안전하고 감사 가능** – 세션 감사, 샌드박싱, 권한 제어, 애플리케이션 레이어 방화벽 역할을 하는 내장 API 프록시 — 모든 입출력 바이트가 가시적.

보일러플레이트를 최소화하여 **중요한 것에 집중**할 수 있습니다. <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> **ZimaOS의 설계 철학**에 충실하게, Blue는 다음을 제공합니다:
- **원클릭으로 제로에서 원까지** – 복잡한 설정 없이 즉시 배포.
- **빠른 프로토타이핑** – 시나리오별 도구, 상호작용, 앱 패키지를 바이브 코딩하거나 직접 제작.
- **글로벌 대응** – **세상은 넓고**, 영어가 기본이 아닙니다. **20개 이상의 언어, 네이티브 지원**, 장벽 없음.
- **개방형 모델 생태계** – 벤더 종속 없음. 자신의 모델을 가져오세요.

![](../../docs/assets/design_principle.png)

## 빠른 시작

### 옵션 1: 데스크톱 앱 다운로드

네이티브 애플리케이션 — 의존성 없음, 컴파일 불필요. 체험 설정이 내장되어 있어 즉시 시작 가능 — 원격 연결로 바로 대화를 시작하세요, 봇 설정 불필요. 진정한 즉시 사용.

- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)**macOS**: [DMG 다운로드](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)**Windows**: [설치 프로그램 다운로드](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)

### 옵션 2: 설치 스크립트

![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
curl -fsSL https://ota.zimaos.com/blue | sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
irm https://ota.zimaos.com/blue/windows | iex
```

### 옵션 3: 소스에서 빌드

```bash
git clone https://github.com/IceWhaleTech/ZimaOS-Blue.git
cd ZimaOS-Blue
git submodule update --init --recursive
```

![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
sh build.sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
.\build.bat
```

> **참고:** Windows 빌드에는 다음이 필요합니다：
> - 네이티브 C 종속성(espeak-ng, whisper.cpp, opus, kokoro, onnx)을 위해 [MinGW-w64](https://www.mingw-w64.org/)(gcc)와 [CMake](https://cmake.org/)
> - 시스템 라이브러리(winmm 등)를 위해 [Windows SDK](https://developer.microsoft.com/en-us/windows/downloads/windows-sdk/)
>
> `gcc`와 `cmake`가 `PATH`에 포함되어 있는지 확인하세요.

## 아키텍처 개요

<details>
<summary>
<img src="../../docs/assets/architecture.png" alt="Architecture" />
</summary>

### 패키지 맵 (`server/internal/`)

| 레이어 | 패키지 |
|-------|----------|
| 게이트웨이 | bootstrap, server, gateway |
| 프록시 | proxy, connection, streaming, resilience |
| 프로바이더 | providerpool, providers, llm |
| 프루너 | pruner (detector, segmenter, bm25, pipeline, cache) |
| 에이전트 | context, tools, personality, humanizer |
| 메모리 | memory, embedding, kvstore |
| 채널 | channel, autoreply, i18n |
| 보안 | security, auth, permission, rbac, mfa, password, oidc, extauth, sandbox, promptguard, audit |
| 음성 | voice, tts, stt, speech |
| 관측 | metrics, heartbeat, companion, profiling, leakdetect |
| 플러그인 | plugin, skill, skillstore |
| 통합 | browser, homeassistant, cron, workflow, formfiller, tunnel, crawler |
| 스케줄러 | scheduler, worker, workerpool, pool |
| 코어 | lifecycle, config, logger, database, cache, ratelimit, retry, timeutil, sync |
| 시스템 | sysinfo, cgroup, iotask, watcher, resources, backup, update |
| 멀티테넌트 | tenant, user, session, preview |

</details>

### 데이터 흐름

**채팅 요청 (프록시 핫 패스)**
```
Client [Proxy API Key] → Auth Gate → Prompt Guard → Context Pruner (optional)
  → Provider Pool (route:auto/cloud/local) → CC Cache (L1→L2) check
  → Upstream LLM → Response → Cache Store → Metrics Writer → Client (SSE stream)
```

**채널 메시지 흐름**
```
Telegram/Discord/... → Channel Manager → AutoReply check
  → Chat Handler → LLM → Humanizer (MD→text) → Channel → User
```

**음성 파이프라인**
```
WebSocket audio → STT (Whisper) → LLM Processing → TTS (eSpeak/Edge) → WebSocket audio
```

## 사용 방법

![](../../docs/assets/handcraft.png)

## 마일스톤 타임라인

<details>
<summary>
<img src="../../docs/assets/timeline.png" alt="Milestone Timeline" />
</summary>

| 버전 | 초점 | 핵심 가치 | 상태 |
|---------|-------|-----------|--------|
| v0.1 | Go 런타임 코어 | 안정적인 커널, 24시간 운영 | Done |
| v0.2 | 핵심 기능 | 최소 사용 가능, LLM 통합 | Done |
| v0.3 | NAS 통합 | NAS 네이티브, systemd 지원 | Done |
| v0.4 | 플러그인 시스템 | 확장 가능, 보안 기초 | Done |
| v0.5 | 제품 기준선 | 프로덕션 준비 완료, 문서화 | Done |
| v0.6 | 메시지 채널 | 멀티 채널 지원 | Done |
| v0.7 | 보안 | OIDC, MFA, 감사 | Done |
| v0.8 | 성능 | 최적화, 캐싱, 벤치마크 | Done |
| v0.9 | 생태계 | 멀티테넌트, 브라우저 자동화, 음성 | Done |
| v0.10.0 | CLI 번들링 | CC CLI 번들링, 감지, 자동 업데이트 | Done |
| v0.10.1 | 메트릭 모니터링 | API 통계, 토큰 추적, TTFT | Done |
| v0.10.2 | CLI 안정성 | 프로세스 생명주기, 오류 복구 | Done |
| v0.10.3 | CLI 통합 | 설정 마법사, 프로바이더 자동 감지 | Done |
| v0.10.4 | Tauri 패키징 | 데스크톱 앱, 시스템 트레이 | Done |
| v0.10.5 | API 프록시 사이드카 | 라우트 선택, 프롬프트 가드, 사용 통계 | Done |
| v0.10.6 | 프로바이더 풀 | 멀티 프로바이더 라우팅, 헬스 체크, 페일오버 | Done |
| v0.10.7 | 미리보기 모드 | 비인증 접근, 기능 게이팅 | Done |
| v0.10.8 | 스킬 스토어 | 스킬 스토어 인프라, 채널 검증 | Done |
| v0.10.9–10 | 사용자 관리 | 하위 사용자, 페이지 수준 권한 | Done |
| v0.10.13–14 | 보안 & 스킬 | 보안 페이지, 스킬 스토어 재설계 | Done |
| v0.10.15 | 채팅 개선 | 채팅 UX, 메시지 파이프라인 | Done |
| v0.10.16 | 음성 모듈 | Sherpa TTS/ASR, eSpeak, 프로바이더 전환 | Done |
| v0.10.17 | 원격 접속 | Ngrok, Cloudflare 터널, ACME 인증서 | Done |
| v0.10.18–20 | 성능 스프린트 | 시작/채팅 성능, 컨텍스트 캐시 | Done |
| v0.10.21–22 | 프롬프트 & DingTalk | 시스템 프롬프트, DingTalk 채널 | Done |
| v0.10.23 | OTA 업데이트 | OTA 업데이트 시스템 | Done |
| v0.10.24 | 채널 업그레이드 | 스텁에서 10개 채널 업그레이드 | Done |
| v0.10.25 | CC 캐시 | 2단계 캐시 (L1 메모리 + L2 디스크) | Done |
| v0.10.26 | 휴머나이저 | 응답 인간화 파이프라인 | Done |
| v0.10.27 | 컨텍스트 프루너 | 코드 54% 토큰 절감 (SWE-bench 공식), 일반 문서 46–47% (로컬 IR), BM25 스코어링, 세그멘테이션 | Done |
| v0.10.28 | 메모리 서비스 | 점진적 검색, 이중 쓰기 백엔드 | Done |

</details>

## 커뮤니티 & 지원

- **이슈**: [버그 및 기능 요청은 여기에 제출해 주세요](https://github.com/IceWhaleTech/ZimaOS-Blue/issues)
- **토론**: [Discord](https://discord.gg/SrCYvumF)
- **팔로우** [GitHub](https://github.com/IceWhaleTech)

## 라이선스

이 프로젝트는 MIT 라이선스에 따라 라이선스가 부여됩니다 - 자세한 내용은 [LICENSE](../../LICENSE) 파일을 참조하세요. 우리는 오픈소스와 커뮤니티에 대한 기여를 믿습니다.

## 기여자

<p align="center">
  Made with ❤️ by <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
