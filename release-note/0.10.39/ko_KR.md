# ZimaOS Blue v0.10.39

출시일: 2026-04-09

통합된 Research 워크플로, 새로운 Knowledge 및 Evolution 워크스페이스, 더 강력한 대화 복구, 그리고 문서 추출 및 실행 안정성 향상이 포함된 릴리스입니다.

## 주요 내용

- research, knowledge, evolution 작업을 위한 더 통합된 워크플로
- 진행 중이거나 대기 중인 대화로 돌아갈 때 더 나은 복구
- 문서 추출과 제어된 실행에 대한 더 강한 안정성

## 새로운 기능

- 모드별 출력은 유지하면서 통합된 `Research` 진입점을 추가
- 컴파일된 페이지, lint 상태, ask-and-archive 작업을 위한 `Knowledge` 워크스페이스를 추가
- 검토와 진단을 위한 `Evolution` 콘솔과 로컬 `blue audit` 도구를 추가

## 개선 사항

- 다시 연 채팅에서 더 많은 활성 상태와 대기 중 승인을 복원할 수 있도록 대화 부트스트랩을 개선
- 더 반복 가능한 평가 실행을 위해 Harness 데이터셋 워크플로를 개선
- 컨텍스트 압축, failover 처리, 로컬라이제이션 일관성을 개선

## 수정 사항

- 제공자 카탈로그 검증 중 발생하던 새로고침 루프를 수정
- artifact recovery 처리로 반복적인 읽기 루프 시나리오를 수정
- PDF 텍스트 추출 중 잘못된 문자 복구 문제를 수정

## 보안

- `blue exec` 및 계층형 sandbox 라우팅으로 명령 실행을 강화

## 참고

문제가 발생하면 43,000명 이상의 멤버가 있는 Zima 커뮤니티 Discord에 참여해 지원을 받으세요:

https://zimaboard.com/discord
