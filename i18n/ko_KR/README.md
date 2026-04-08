![](../../docs/assets/banner.png)

<p align="center">
  ZimaOS Blue: 대담한 빌더를 위한 <strong>로컬 우선</strong> 에이전트 런타임<br>
  즉시 사용 가능 · 오픈소스 · 범용 · 벤더 중립
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
  <a href="https://discord.gg/b3AgFDxe9v"><img src="../../docs/assets/discord.png" alt="Discord" height="128" /></a>&nbsp;&nbsp;
  <a href="https://www.facebook.com/zimaboard/"><img src="../../docs/assets/facebook.png" alt="Facebook" height="128" /></a>&nbsp;&nbsp;
  <a href="https://x.com/ZimaSpace"><img src="../../docs/assets/x.png" alt="X" height="128" /></a>
</p>

## 소개

Clawdbot에서 영감을 받아 개인 컴퓨팅의 미래는 엣지에서 실행되는 다양한 로컬 우선 AI 에이전트에 의해 형성될 것이라고 믿습니다.

ZimaOS Blue이 우리의 대답입니다. 완전한 오픈 소스, 감사 가능, 공급업체 중립적, 프로덕션 지원 에이전트 런타임 및 툴킷으로, 마찰 없이 비공개 자체 호스팅 에이전트를 제공할 수 있습니다.

자신만의 에이전트를 개발하거나 직접 만들고 싶어하는 대담한 개발자를 위해 제작된 Blue은 성능을 위해 설계되었습니다. Go로 작성되었으며 메모리 공간은 19MB에 불과합니다. x86, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS 등 전원을 연결하는 모든 곳에서 실행됩니다.

## 데모

### 대화 및 작업 실행

Blue의 대화 흐름과 작업 실행을 빠르게 보여주는 데모입니다.

[데모 보기](<../../docs/assets/demo.mp4>)

### LLM 제공업체 통합

Blue의 LLM 제공업체 통합 경험을 빠르게 보여주는 데모입니다.

[데모 보기](<../../docs/assets/demo Provider.mp4>)

### 빠른 개요 - 개요, 채널 및 추가 구성

제품 개요, 채널 및 추가 구성을 빠르게 보여주는 데모입니다.

[데모 보기](<../../docs/assets/demo quickv4.mp4>)

## 왜 Blue

<p align="center">
  <img src="../../docs/assets/design_principle.png" alt="Design Principle" />
</p>

### Pure Go, 모든 장치

100% Go, 정적 바이너리. 즉시 사용 가능한 5개 대상(![linux](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png) `linux/amd64`, `linux/arm64`, ![macOS](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png) `darwin/amd64`, `darwin/arm64`, ![windows](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png) `windows/amd64`)으로 크로스 컴파일합니다. 노드 런타임, Python, 컨테이너가 필요하지 않습니다. NAS, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, 오래된 x86 라우터 또는 ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)Mac에 올려놓으면 바로 실행됩니다. 그런 다음 모든 플랫폼에 하나의 코드베이스로 자신만의 UI, 로직, 에이전트 기술을 추가하세요.

### 즉시 사용 가능, 작업 준비 완료

누구나 간단하고 안정적이며 필요할 때 확장 가능한 도구를 원합니다. 제대로 작동하는 도구이므로 실제로 구축 중인 작업에 집중할 수 있습니다.

이것은 새로운 철학이 아닙니다. <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> ZimaOS을 만든 것과 동일합니다. 간단하고 안정적이며 방해가 되지 않도록 제작되었습니다. Blue은 에이전트 스택으로 확장된 철학입니다.

### 귀하의 삶을 위해 설계되고 지역에 머물도록 제작되었습니다.

전체 HTML 보고서를 제공하는 심층 연구부터 OCR, PDF, 브라우저 자동화 및 문서 변환에 이르기까지 Blue은 데이터를 클라우드로 전송하지 않고도 복잡한 실제 워크플로를 처리합니다. 음성 깨우기, STT/TTS, Talk Mode 및 로컬 추론 지원을 통해 일상적인 상호 작용을 즉각적이고 비공개이며 항상 사용할 수 있습니다.

## 빠른 시작

### 옵션 1: 데스크톱 앱 다운로드

종속성이나 컴파일이 필요 없는 기본 애플리케이션을 받으세요. 몇 초 만에 온보딩이 가능한 평가판 구성이 내장되어 있습니다. 봇 설정이 필요 없이 원격 연결을 통해 즉시 채팅을 시작할 수 있습니다. 진정한 즉시 사용 가능한 경험.

- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)**macOS**: [DMG 다운로드](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)**Windows**: [설치 프로그램 다운로드](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)

### 옵션 2: 스크립트 설치

![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
curl -fsSL https://ota.zimaos.com/blue | sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows(파워셸)**
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

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows(파워셸)**
```powershell
.\build.bat
```

> **참고:** Windows 빌드에는 다음이 필요합니다.
> - [MinGW-w64](https://www.mingw-w64.org/) (gcc) 및 [CMake](https://cmake.org/) - 네이티브 C 종속성(espeak-ng, Whisper.cpp, opus, kokoro, onnx)
> - 시스템 라이브러리(winmm 등)용 [Windows SDK](https://developer.microsoft.com/en-us/windows/downloads/windows-sdk/)
>
> `gcc`, `cmake`이 `PATH`에 있는지 확인하세요.

## 아키텍처 개요

<p align="center">
  <img src="../../docs/assets/architecture.png" alt="architecture" />
</p>

더 나아가서 **20개 이상의 IM 플랫폼**, 자연스러운 상황 인식 대화를 위한 **음성 기반** 인터페이스, IDE 스캐닝을 통한 **제로 구성 모델 전환**에 대한 기본 지원을 제공합니다.

<p align="center">
  <img src="../../docs/assets/providers.png" alt="Supported Providers" />
</p>

## 빌드 방법

<p align="center">
  <img src="../../docs/assets/handcraft.png" alt="handcraft" />
</p>

> ⚠️ [!IMPORTANT]
>
> Blue 위에 튜닝이나 바이브 코딩을 계속할 계획이라면 몇 가지 멋진 채팅을 릴리스 증거로 간주하지 마십시오. 라우팅, 실행 동작, 도구 표면, 예산 제어, 모델 선택 또는 실행 프레임워크에 영향을 미치는 모든 변경 사항은 즉석 확인이 아닌 Blue Harness을 사용하여 검증해야 합니다.
>
> Blue 여기서는 하나의 간단한 규칙을 따라야 합니다: 데이터 우선, 게이트 우선, 컷오버 마지막. 실제로 이는 변경 사항을 판단하기 전에 관련 Harness 데이터 세트/평가 사양을 업데이트한 다음 전체 시도에서 하나의 안정적인 `candidate_id`을 유지하여 선택기, 실행, 예산 및 준비 보고서가 모두 관련 없는 4개의 실행 대신 동일한 후보를 설명한다는 것을 의미합니다.

### 권장 Harness 작업 흐름

1. `blue harness selector verify` 실행
2. `blue harness execution verify` 실행
3. `blue harness budget gate`에 대한 선택기 평가 실행을 재사용합니다.
4. `blue harness cutover-readiness`으로 마무리

로컬 반복, 야간 검증 또는 CI 증거 수집의 경우 `python3 scripts/cutover_candidate_pipeline.py`을 선호합니다. 하나의 공유 후보 아래에서 전체 선택기 -> 실행 -> 예산 -> 준비 순서를 실행하므로 결과를 더 쉽게 비교, 검토 및 전환할 수 있습니다.

### 추가 가드레일

| 면적 | 볼만한 동영상 |
|------|---|
| 기준 안정성 | 기준선, 데이터 세트 버전 및 `candidate_id`을 안정적으로 유지하지 않으면 비교가 표류되어 결과를 신뢰할 수 없게 됩니다. |
| 실제 빌드 출력 | Harness을 실행하기 전에 영향을 받은 바이너리 또는 프런트엔드 번들을 다시 빌드하세요. 그렇지 않으면 현재 변경 사항 대신 오래된 동작을 검증하게 될 수도 있습니다. |
| 노선등록 | 프런트엔드와 백엔드가 함께 변경되는 경우 UI 동작을 통해 기능을 판단하기 전에 모든 새 백엔드 경로가 실제로 등록되었는지 확인하세요. 등록 누락은 종종 논리 버그처럼 보이지만 실제로는 `404`이기 때문입니다. |
| 판정 해제 | Harness에서 의미 있는 회귀가 표시되지 않고 컷오버 준비 상태가 후보가 실제로 컷오버할 준비가 되었음을 확인하는 경우에만 튜닝 패스가 준비된 것입니다. |

간단히 말해서 Blue에 튜닝을 한다는 것은 "채팅 몇 번 하면 기분이 좋아진다"는 것이 아닙니다. 이는 후보자를 Harness에 배치하고, 비교 가능한 증거를 수집하고, 게이트 및 준비 결과에 따라 변경 사항이 실제로 유지하기에 안전한지 결정하게 하는 것입니다.

## 기능

| 기능 | 그것이 제공하는 것 |
|---------|------|
| 고가용성 웹 검색 및 브라우저 런타임 | Blue의 **가장 뚜렷한 차별화 요소** 중 하나입니다. Blue은 검색, 읽기, 추출, 크롤링을 위한 **4개의 웹 액세스 경로**를 통합합니다. HTTP, 프록시 추출 및 브라우저 세션 전반에 걸쳐 **3개의 대체 레이어**를 유지합니다. 챌린지 감지, 쿠키/세션 재사용, 스텔스 및 브라우저 핸드오프를 통해 **안티봇 페이지**를 처리합니다. **세 가지 브라우저 엔진**(`lightpanda`, 관리형 Chromium 및 릴레이/로컬 Chromium)을 통해 라우팅됩니다. |
| 3-in-1 연구 런타임 | **하나의 공개 연구 항목**은 `deep_research`, `analyze` 및 `ui_review`으로 라우팅될 수 있습니다. 그런 다음 동일한 발견 및 증거 스택을 통해 **인용 우선 연구**, **제한 보고서**, **구조화된 UI/UX/접근성 검토**가 생성됩니다. |
| Harness 런타임, 평가 및 발전 프레임워크 | 개발, 훈련, 생산 전반에 걸쳐 평가를 **런타임 기본**으로 만듭니다. Harness은 **회귀 및 연기 검사**, 채점, 기준선, 보고서 및 런타임 검증을 다룬 다음 동일한 증거를 **기술 발전**, 후속 평가, 승격 또는 롤백, `AGENTS.md` 또는 지침 제안 검토에 전달합니다. |
| 멀티모달 네이티브 기능 우선 런타임 | **실제로 필요할 때만 모델 라우팅**을 사용하여 **네이티브 및 로컬 경로 우선**에서 **음성, OCR, PDF, 브라우저 작업, 문서 변환, 구조화된 양식 채우기, 미디어 처리 및 미디어 생성**을 유지합니다. |
| 보안 및 거버넌스 | **샌드박스 실행**, **즉각적 주입 방어**, **세션 감사**, 권한, **RBAC**, **WebAuthn**, 운영 가드레일, **기술 보안 스캐닝**이 포함됩니다. |
| LLM 위키 및 지식 공간 | 메모리, 연구 및 런타임 출력을 **요약 페이지**, 색인, **백링크**, **신선성** 및 **보관 워크플로**를 통해 **위키와 같은 지식 표면**으로 전환합니다. |
| 스킬 스토어 및 마켓플레이스 | **내장된 기술 검색**, 큐레이션, 동기화 및 **로컬 스캐닝**을 제공하므로 **첫 날부터** 확장성을 사용할 수 있습니다. |
| 프로덕션 등급 공급자 풀 | 장기 실행 워크로드를 위한 **상태 확인**, **자동 장애 조치**, **회로 차단기** 및 **공급자 경주** 기능을 갖춘 실제 공급자 풀을 제공합니다. |
| 내장형 로컬 소형 모델 런타임 | **로컬의 짧은 Q&A**, 이미지 인식, 도구 라우팅, 요약, **컨텍스트 압축** 및 **문서 전처리**를 위한 기본 제공 **`Qwen3.5-0.8B` + `llama.cpp`** 런타임을 제공합니다. |
| 장기적 신뢰성 | **OTA 업데이트**, **백업 및 복원**, **핫 리로드 구성** 및 **실패 후 복구**를 **내재된 운영 문제**로 처리합니다. |

## 마일스톤 타임라인

<p align="center">
  <img src="../../docs/assets/timeline.png" alt="Milestone Timeline" />
</p>

| 날짜 | 버전 | 키워드/특징 |
|------|---------|---------|
| 2026년 1월 26일 | `v0.1–v0.9` | Go 런타임, 플러그인 시스템, 브라우저 자동화 |
| 2026년 1월 27~28일 | `v0.9.0–v0.9.2` | 브라우저 작업 보기, Blue Companion, Smart Form Filler |
| 2026년 1월 29~31일 | `v0.10.0–v0.10.9` | Claude Code CLI, API Proxy, UI 개편 |
| 2026년 2월 1~3일 | `v0.10.1–v0.10.22` | 메트릭, 원격 액세스, 컨텍스트 캐시 |
| 2026년 2월 5~18일 | `v0.10.25–v0.10.29` | i18n, CC 캐시, 릴리스 파이프라인 |
| 2026년 2월 20~25일 | `v0.10.28–v0.10.29` | 데스크탑 로더, 모바일 UX, 메모리 재설계 |
| 2026년 2월 28일~3월 2일 | `v0.10.30` | Deep Research, 스킬 재순위, 보안 스캔 |
| 2026년 3월 9~18일 | `v0.10.31` | 대시보드 점검, VoiceChat 리팩터링, 승인된 사이트 |
| 2026년 3월 19~22일 | `v0.10.32` | Harness 출시, 기록 감사, 웹 검색 |
| 2026년 3월 23~25일 | `v0.10.33` | Harness 그룹, 브라우저 승인, 스킬 시장 |
| 2026년 3월 29~30일 | `v0.10.35` | Harness v3, 브라우저 릴레이, 컨텍스트 압축 |
| 2026년 3월 31일~4월 1일 | `v0.10.36` | 성적표 감사, Harness 오버레이, 도구 구문 분석 |
| 2026년 4월 1일 | `v0.10.37` | 런타임 강화, Skill+Exec 컷오버, 복구 개선 |
| 2026년 4월 2~5일 | `v0.10.38` | GitHub 지원, 시장 개선, 신뢰성 개선 |
| 2026년 4월 6~7일 | `v0.10.39` | 연구 통합, 진화 표면, 메모리 사용량 감소 |

## 커뮤니티 및 지원

- **문제**: [버그 및 기능 요청은 여기에 제출하세요.](https://github.com/IceWhaleTech/ZimaOS-Blue/issues)
- **토론**: [Discord](https://discord.gg/zwWbKA4S2)
- [GitHub](https://github.com/IceWhaleTech)에서 **팔로우**하세요.

[![Star History Chart](https://api.star-history.com/svg?repos=IceWhaleTech/ZimaOS-Blue&type=Date)](https://star-history.com/#IceWhaleTech/ZimaOS-Blue&Date)

## 라이센스

이 프로젝트는 MIT 라이선스에 따라 라이선스가 부여됩니다. 자세한 내용은 [LICENSE](../../LICENSE) 파일을 참조하세요. 우리는 오픈 소스와 커뮤니티에 대한 환원을 믿습니다.

## 기여자

모든 Blue 기여자에게 감사드립니다:

<a href="https://community.vaunt.dev/board/IceWhaleTech/repository/ZimaOS-Blue">
  <img src="https://api.vaunt.dev/v1/github/entities/IceWhaleTech/repositories/ZimaOS-Blue/contributors?format=svg&limit=30" width="450" />
</a>

## 참고자료

1. **OpenClaw** — 로컬 우선 오픈 소스 에이전트. 채널 어댑터 및 도구 호출을 통해 LLM을 로컬 장치에 연결하는 데 앞장섰으며 Blue의 에이전트 런타임 아키텍처에 직접적인 영감을 주었습니다. https://github.com/openclaw/openclaw
2. **MiroMind** — 증거 기반 합성을 갖춘 심층 연구 모드입니다. Blue의 기본 심층 연구 파이프라인 형성: 계획, 병렬 검색, 증거 중복 제거 및 HTML 보고서 생성. https://www.miromind.ai
3. **Karpathy's LLM Wiki** — 지식 컴파일러로서의 LLM. RAG의 축적 함정을 넘어 지속적이고 진화하는 지식 공간을 구축하기 위해 LLM을 재구성합니다.
4. **OpenSpace (HKUDS)** — 스스로 진화하는 스킬 엔진. 에이전트가 실패로부터 학습하고 전문 기술을 습득하는 DAG 기반 프레임워크입니다. https://github.com/HKUDS/OpenSpace
5. **Andrew Ng's Context Hub** — 코딩 에이전트를 위한 버전이 지정된 API 문서 레지스트리입니다. 상담원의 환각과 잊혀진 세션 지식을 해결합니다. 주석 및 피드백 루프가 포함된 선별되고 버전이 지정된 문서를 제공하여 문서를 자체 개선되는 지식 계층으로 전환합니다. https://github.com/andrewyng/context-hub
6. **Notion** — 단순하고 인간적이며 의도적으로 조용합니다. Notion의 미니멀리스트 정신에서 영감을 받은 Blue은 그리드에 따뜻함을 되살려줍니다. 세련된 세리프가 사려 깊은 디자인과 만나 집처럼 느껴지는 공간을 만들어냅니다. https://www.notion.com/about
7. **Matrix** — 상징적인 디지털 비의 미학에서 시각적 영감을 얻었습니다. Blue 기술 다이어그램의 미학적 방향.
8. **IceWhale** — 사랑, 죽음, 로봇 S2E2 "Ice". 인터넷 거대 기업의 벽을 무너뜨리고 데이터 집중에 저항하기 위해 전 세계적으로 모이는 집단입니다. 얼음고래는 가장자리에서 함께 주권 도구를 구축하는 공동체를 상징합니다.
9. **ZimaOS Blue** — 사랑, 죽음, 로봇 S1E14 "지마 Blue". 비유: 서비스에서 시작하여 세계를 탐험하기 위해 진화하는 지능. Blue은 단순함에 뿌리를 두고 깊이를 추구하는 지혜의 대리인입니다.
10. **ZimaOS** — 단순화되고 집중적이며 개방적인 디자인 원칙. ZimaOS과 Blue 모두 기술이 사용자에게 서비스를 제공해야 한다는 믿음을 공유합니다. 즉, 30초 안에 배포하고, 어디에서나 실행하고, 공급업체 중립성을 유지해야 합니다. https://www.zimaspace.com/zimaos
