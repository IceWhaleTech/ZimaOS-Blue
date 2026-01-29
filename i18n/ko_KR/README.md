# ZimaOS Echo

<p align="center">
  <img src="../../docs/public/logo.png" alt="ZimaOS Echo" width="200">
</p>

<p align="center">
  <strong>안전하고, 관측 가능하며, 로컬 우선 AI 에이전트 런타임</strong>
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

**ZimaOS Echo**는 NAS 및 엣지 디바이스용 견고한 AI 에이전트 런타임입니다. 데이터는 사용자 하드웨어에 남고, 모든 동작은 감사 가능하며, AI는 격리된 샌드박스에서 실행됩니다.

[문서](https://echo.zimaos.com) · [빠른 시작](#빠른-시작) · [기능](#핵심-원칙) · [비교](#clawdbot과-비교)

## 왜 ZimaOS Echo인가?

ZimaOS Echo는 [clawdbot](https://github.com/clawdbot/clawdbot)에서 영감을 받아 Go로 다시 구현했습니다:

- **낮은 리소스 사용**: 256MB RAM 장치에서 실행
- **높은 성능**: 네이티브 Go 바이너리, goroutine 동시성
- **쉬운 배포**: 단일 바이너리, Node.js 불필요
- **NAS 최적화**: 저전력 장치 24/7 운영용

## 핵심 원칙

### 로컬 우선

- **데이터 주권**: 모든 데이터 NAS에 로컬 저장, 클라우드 의존 없음
- **Ollama 연동**: LLM 완전 온디바이스 실행, 외부 API 호출 제로
- **오프라인 지원**: 핵심 기능은 인터넷 없이 동작
- **단일 바이너리**: 약 15MB 네이티브 Go 바이너리, 런타임 의존성 없음

### 관측 가능·감사 가능

- **감사 로깅**: 모든 AI 동작을 컨텍스트·타임스탬프와 함께 기록
- **Prometheus 메트릭**: 시스템 동작 실시간 모니터링
- **pprof 프로파일링**: CPU, 메모리, goroutine 심층 가시성
- **구조화 로그**: JSON 로그로 파싱·알림 용이

### 보안 강화

- **샌드박스 실행**: 모든 도구 호출을 격리 환경에서 실행
- **RBAC**: 세밀한 역할 기반 접근 제어
- **WebAuthn/Passkeys**: 비밀번호 없는 FIDO2 인증
- **MFA/TOTP**: 다중 요소 인증
- **OIDC/OAuth 2.0**: 엔터프라이즈 SSO 연동
- **서킷 브레이커**: 자동 장애 격리로 연쇄 실패 방지

## 빠른 시작

```bash
# Linux / macOS
curl -fsSL https://echo.zimaos.com/install.sh | sudo bash

# 소스에서
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo/server
go build -o zimaos-echo ./cmd/server
./zimaos-echo server
```

## 보안 강화

### 인증 스택

| 계층 | 기술 | 용도 |
|------|------|------|
| 1차 | WebAuthn/Passkeys | 피싱 방지 비밀번호 없는 인증 |
| 2차 | TOTP/MFA | 시간 기반 일회용 비밀번호 |
| 엔터프라이즈 | OIDC/OAuth 2.0 | Google, GitHub, Okta SSO |
| 인가 | RBAC | 리소스별 권한 제어 |

### 런타임 보호

- **샌드박스 격리**: 도구는 제한된 환경에서 실행
- **속도 제한**: 테넌트별 API 스로틀링
- **테넌트 격리**: 데이터·리소스 완전 분리
- **감사 추적**: 모든 권한 작업의 불변 로그

### 복원력

- **서킷 브레이커**: 장애 시 서비스 자동 격리
- **우아한 저하**: 프로바이더 장애 시 폴백 전략
- **LLM 폴백 체인**: 프로바이더 자동 전환
- **핫 리로드**: 설정 변경 시 재시작 없이 반영

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

- 요청 지연 (p50, p95, p99)
- 프로바이더별 LLM 토큰 사용량
- 도구 실행 성공/실패율
- 메모리 및 goroutine 수
- 서킷 브레이커 상태 전이

## 아키텍처

```
┌─────────────────────────────────────────────────────┐
│                    ZimaOS Echo                       │
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

## 로컬 LLM 설정 (Ollama)

외부 API 호출 없이 완전 오프라인 AI 실행:

```bash
# Ollama 설치
curl -fsSL https://ollama.com/install.sh | sh

# 모델 다운로드
ollama pull llama3.2

# Echo 로컬 LLM 사용 설정
cat >> config.yaml << EOF
llm:
  provider: "ollama"
  model: "llama3.2"
  base_url: "http://localhost:11434"
EOF
```

## 개발 환경

### 사전 요구사항

| 도구 | 버전 | 설치 |
|------|------|------|
| Go | 1.21+ | [golang.org](https://golang.org/dl/) |
| Node.js | 18+ | [nodejs.org](https://nodejs.org/) |
| Make | - | macOS/Linux 기본 포함 |

### 원커맨드 시작

```bash
# 클론 후 전체 빌드·실행
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo
make build && ./dist/zimaos-echo
```

대시보드: `http://localhost:3000`

### 개발 모드 (핫 리로드)

```bash
# Linux / macOS
./dev.sh

# Windows
dev.bat
```

- 프론트엔드: `http://localhost:5173` (API는 백엔드로 프록시)
- 백엔드: `http://localhost:8080`

### 빌드 명령

```bash
make build              # 단일 바이너리 (프론트엔드 임베드)
make build-embedded     # Claude Code CLI 임베드 빌드
make build-all          # 전체 플랫폼 크로스 컴파일
make clean              # 빌드 산출물 정리
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

## Clawdbot과 비교

ZimaOS Echo는 clawdbot에서 영감을 받았으며 NAS/엣지 배포에 최적화되어 있습니다:

| 항목 | ZimaOS Echo | Clawdbot |
|------|-------------|----------|
| **언어** | Go | TypeScript/Node.js |
| **바이너리 크기** | ~15MB | ~200MB+ (node_modules 포함) |
| **메모리** | ~80MB 유휴 | ~200MB+ 유휴 |
| **시작 시간** | < 1s | 3–5s |
| **런타임** | 네이티브 바이너리 | Node.js 필요 |
| **대상** | NAS/엣지 | 데스크톱/서버 |

## 감사의 말

- [clawdbot](https://github.com/clawdbot/clawdbot) - 프로젝트 영감
- [IceWhaleTech/zorm](https://github.com/IceWhaleTech/zorm) - 경량 ORM

---

<p align="center">
  Made with ❤️ by <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
