# ZimaOS Echo

<p align="center">
  <img src="../../docs/public/logo.png" alt="ZimaOS Echo" width="200">
</p>

<p align="center">
  <strong>안전하고 관측 가능한 AI 에이전트 런타임</strong>
</p>

<p align="center">
  <a href="../../README.md">English</a> |
  <a href="../zh_CN/README.md">简体中文</a> |
  <a href="../zh_TW/README.md">繁體中文</a> |
  <a href="../ja_JP/README.md">日本語</a> |
  <strong>한국어</strong> |
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

**ZimaOS Echo**는 NAS 및 엣지 디바이스를 위한 경량·고성능 AI 에이전트 런타임입니다. Go로 빌드되어 제로 설정 배포, 세션 모니터링, 사용 분석을 갖춘 프로덕션 준비 플랫폼을 제공합니다.

[빠른 시작](#빠른-시작) · [기능](#핵심-기능) · [배포 모드](#배포-모드)

## 하이라이트

| 항목 | 값 |
|------|------|
| **바이너리 크기** | ~40MB(단일 실행 파일) |
| **메모리(유휴)** | ~4MB |
| **시작 시간** | < 1s |
| **의존성** | 없음(제로 설정 배포) |

## 핵심 기능

### 제로 설정 배포

- **단일 바이너리**: 다운로드 후 실행, 런타임 의존성 불필요
- **필요 시 설정**: 기본 동작, 필요 시 맞춤 설정
- **크로스 플랫폼**: Windows, macOS, Linux 동일 바이너리·동일 경험
- **데몬 지원**: 백그라운드 서비스로 상시 실행 가능

### 세션 모니터링

- **실시간 세션 추적**: 모든 활성 AI 세션 및 라이브 상태 모니터링
- **대화 기록**: 전체 상호작용 감사 추적
- **세션 재생**: 과거 대화 검토 및 분석
- **멀티테넌트 격리**: 사용자 간 세션 완전 분리

### 호출 체인 최적화

- **요청 추적**: 모든 API 호출의 엔드투엔드 가시성
- **지연 분석**: 요청 파이프라인 병목 식별
- **프로바이더 라우팅**: 최적 LLM 프로바이더로의 지능형 라우팅
- **서킷 브레이커**: 프로바이더 장애 시 자동 페일오버

### 사용 분석

- **토큰 소비**: 사용자·세션·프로바이더별 사용량 추적
- **비용 귀속**: 작업별 상세 비용 분석
- **속도 제한**: 테넌트별 할당량 관리
- **보고서 내보내기**: 다중 형식 사용 보고서 생성

### 보안 강화

- **샌드박스 실행**: 모든 도구 호출을 격리 환경에서 실행
- **RBAC**: 세밀한 역할 기반 접근 제어
- **WebAuthn/Passkeys**: 비밀번호 없는 FIDO2 인증
- **MFA/TOTP**: 다중 인증
- **감사 추적**: 모든 권한 작업의 불변 로그

## 빠른 시작

```bash
# 소스에서
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo
make build && ./dist/zimaos-echo server
```

대시보드는 `http://localhost:3000`에서 접근할 수 있습니다.

## LLM 프로바이더 설정

ZimaOS Echo는 로컬 LLM 서비스를 포함한 여러 LLM 프로바이더를 지원합니다:

```yaml
llm:
  # 클라우드 프로바이더
  provider: "openai"  # 또는 "anthropic", "azure" 등
  api_key: "your-api-key"

  # 로컬 LLM(선택)
  # provider: "ollama"
  # base_url: "http://localhost:11434"
```

## 아키텍처

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

## 관측 가능성

```yaml
# 전체 관측성 스택 활성화
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

### 노출 메트릭

- 요청 지연(p50, p95, p99)
- 프로바이더별 LLM 토큰 사용량
- 도구 실행 성공/실패율
- 메모리·고루틴 수
- 서킷 브레이커 상태 전이

## 개발 환경

### 사전 요구 사항

| 도구 | 버전 | 설치 |
|------|------|------|
| Go | 1.21+ | [golang.org](https://golang.org/dl/) |
| Node.js | 18+ | [nodejs.org](https://nodejs.org/) |
| Make | - | macOS/Linux 기본 포함 |

### 개발 모드(핫 리로드)

```bash
# Linux / macOS
./dev.sh

# Windows
dev.bat
```

- 프론트엔드: `http://localhost:3000`
- 백엔드: `http://localhost:8080`

### 빌드 명령

```bash
make build              # 단일 바이너리 빌드(프론트엔드 내장)
make build-embedded     # Claude Code CLI 내장 빌드
make build-all         # 모든 플랫폼 크로스 컴파일
make clean             # 빌드 산출물 정리
```

### 프로젝트 구조

```
ZimaOS-Echo/
├── server/             # Go 백엔드
│   ├── cmd/echo/       # 진입점
│   └── internal/       # 코어 모듈
├── web/                # Vue 3 프론트엔드
│   └── src/
└── dist/               # 빌드 출력
```

## 감사의 말

- [clawdbot](https://github.com/clawdbot/clawdbot) - 프로젝트 영감
- [IceWhaleTech/zorm](https://github.com/IceWhaleTech/zorm) - 경량 ORM

---

<p align="center">
  Made with ❤️ by <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
