# StarLens

[English](README.md) | **한국어**

[StarRocks](https://www.starrocks.io/) 레이크하우스 클러스터를 위한 관리·관측성
대시보드 — 클러스터 토폴로지, 지능형 SQL 워크시트, 데이터 스큐/리니지 시각화를
한곳에서 제공합니다.

StarRocks는 MySQL 와이어 프로토콜을 사용하므로 클러스터에 아무것도 설치할 필요가
없습니다. 프론트엔드 쿼리 포트만 지정하면 카탈로그를 직접 읽습니다.

## 기술 스택

| 계층       | 선택                                                              |
| ---------- | ----------------------------------------------------------------- |
| 백엔드     | Go 1.26, Gin, `go-sql-driver/mysql`                               |
| 프론트엔드 | React 19 + Vite (TypeScript), Tailwind CSS v4, shadcn/ui          |
| 상태 관리  | Zustand (UI 상태), TanStack Query (서버 상태)                     |
| 시각화     | Monaco Editor, ECharts, React Flow *(개발 예정)*                  |

## 빠른 시작

```bash
# 선택: Docker로 로컬 단일 노드 StarRocks 실행 (FE 쿼리 포트 9030)
make starrocks-up

# API(:8080)와 웹 개발 서버(:5173)를 함께 시작. Ctrl-C 한 번에 둘 다 종료
make run
```

`make run`은 `backend/.env`가 있으면 자동으로 로드합니다
([`backend/.env.example`](backend/.env.example)을 복사해 사용). 없으면 API는
`root` 계정으로 `127.0.0.1:9030`에 접속합니다. `make help`로 전체 타겟을 확인할
수 있습니다 — `build`, `test`, `lint`, `check`(CI가 실행하는 전부), `deps`,
`clean`, `starrocks-up/down`. 포트는 오버라이드 가능합니다:
`make run BACKEND_PORT=9090 FRONTEND_PORT=3000`.

### 수동 설정 (make 없이)

프로세스는 두 개입니다: `:8080`의 Go API와 `:5173`의 Vite 개발 서버. 개발 서버가
`/api`를 API로 프록시하므로 브라우저는 same-origin을 유지합니다.

### 1. 백엔드

```bash
cd backend
export STARROCKS_DSN='root:@tcp(127.0.0.1:9030)/information_schema?charset=utf8mb4&timeout=5s'
go run ./cmd/server
```

전체 설정(풀 크기, 쿼리 타임아웃, CORS 오리진, 포트)은
[`backend/.env.example`](backend/.env.example)을 참고하세요. StarRocks에 연결할
수 없어도 서버는 정상 기동합니다 — 클러스터 관련 엔드포인트가 드라이버 에러를
담은 `503`을 반환하며, 보통 그 에러 메시지가 실제 답입니다.

```bash
curl localhost:8080/healthz             # API가 살아있는가?
curl localhost:8080/api/v1/health       # StarRocks에 접근 가능한가?
curl localhost:8080/api/v1/topology     # 클러스터 멤버십
```

### 2. 프론트엔드

```bash
cd frontend
npm install
npm run dev            # http://localhost:5173
```

API가 `localhost:8080`에서 실행되지 않는다면 `VITE_PROXY_TARGET`을 설정하세요
([`frontend/.env.example`](frontend/.env.example) 참고).

### 사용할 클러스터가 없다면?

단일 노드 StarRocks는 FE 쿼리 포트를 9030으로 노출합니다:

```bash
docker run -p 9030:9030 -p 8030:8030 -p 8040:8040 \
  --name starrocks -itd starrocks/allin1-ubuntu
```

## API

| 메서드 | 경로                        | 설명                                               |
| ------ | --------------------------- | -------------------------------------------------- |
| `GET`  | `/healthz`                  | API 자체 생존 여부. 클러스터 상태와 무관.          |
| `GET`  | `/api/v1/health`            | StarRocks 접근 가능 여부 (불가 시 `503`).          |
| `GET`  | `/api/v1/cluster/topology`  | FE/BE/CN 멤버십, 역할, 생존 상태, 용량, 클러스터 배포 모드. |
| `GET`  | `/api/v1/topology`          | 위 엔드포인트의 별칭.                              |
| `GET`  | `/api/v1/loads/routine`     | 전체 데이터베이스의 Routine Load 잡: 상태, 통계, 근사 오프셋 지연. |
| `GET`  | `/api/v1/alerts`            | 발생한 알림 히스토리 (인메모리, 최신순).           |
| `POST` | `/api/v1/alerts/test`       | 모든 노티파이어로 테스트 알림을 발사하고 채널별 결과를 보고. |

실패 응답은 단일 envelope을 공유하므로 UI가 일관되게 렌더링할 수 있습니다:

```json
{
  "error": {
    "code": "starrocks_unavailable",
    "message": "Could not read cluster topology from StarRocks.",
    "detail": "dial tcp 127.0.0.1:9030: connect: connection refused"
  }
}
```

토폴로지는 `SHOW FRONTENDS`, `SHOW BACKENDS`, `SHOW COMPUTE NODES`에서
파생되므로 **두 배포 모드를 모두 지원합니다**: shared-nothing 클러스터는 BE를,
shared-data 클러스터는 CN을(웨어하우스 할당 정보 포함) 표시하며, 상태 판정은
존재하는 컴퓨트 계층 기준으로 이루어집니다. `runMode` 필드는 FE의 `run_mode`
설정에서 읽은 배포 방식을 알려줍니다: `shared_data`, `shared_nothing`(설정
항목이 없는 3.0 미만 릴리스도 이 값으로 추론), 또는 접속 계정에 FE 설정을 읽을
ADMIN 권한이 없을 때의 `unknown`.

SHOW 결과의 컬럼 구성은 StarRocks 릴리스마다 달라지므로, 모든 필드는 별칭
폴백(`IP`/`Host`, `Role`/`IsMaster`, `BackendId`/`ComputeNodeId`)과 함께 이름으로
읽으며, 없는 컬럼은 0이 아니라 "없음"으로 직렬화됩니다.

Routine Load 잡은 `information_schema.routine_load_jobs`(StarRocks 3.1+)에서
읽습니다. 구버전에서는 자동으로 각 사용자 데이터베이스를 `SHOW ALL ROUTINE
LOAD`로 순회하는 방식으로 폴백하며, 스냅샷의 `source` 필드에 어떤 경로로
조회했는지 기록됩니다.

## 알림 (Alerting)

StarLens는 백그라운드 주기로 알림 규칙을 평가하고 결과를 플러그형 노티파이어로
전달합니다. 같은 조건의 반복은 `ALERT_COOLDOWN`(기본 10분) 동안 억제됩니다. 모든
설정은 환경 변수로 이루어집니다 —
[`backend/.env.example`](backend/.env.example)을 참고하세요.

내장 규칙 (Routine Load):

| 규칙                       | 심각도   | 발생 조건                                                     |
| -------------------------- | -------- | ------------------------------------------------------------- |
| `routine_load_paused`      | warning  | 잡이 `PAUSED` 상태 (메시지에 StarRocks의 사유 포함).           |
| `routine_load_cancelled`   | critical | 잡이 `CANCELLED` 상태 — 적재가 스스로 재개되지 않음.           |
| `routine_load_error_ratio` | warning  | `errorRows/totalRows`가 `ALERT_ERROR_ROWS_RATIO` 초과.         |
| `routine_load_offset_lag`  | warning  | 근사 지연이 `ALERT_MAX_OFFSET_LAG` 초과 (opt-in).              |

내장 노티파이어:

- **log** — 항상 켜져 있으며, 심각도에 맞는 레벨로 서버 로그에 기록됩니다.
- **webhook** — `ALERT_WEBHOOK_URL` 설정. `ALERT_WEBHOOK_FORMAT=generic`은
  `{"source":"starlens","alert":{...}}`를, `slack`은 Slack 호환 수신
  웹훅용 `{"text": ...}`를 전송합니다.

`curl -X POST localhost:8080/api/v1/alerts/test` 또는 Routine Load 페이지의
**테스트** 버튼으로 채널을 end-to-end로 검증할 수 있습니다.

채널 추가(이메일, PagerDuty, Opsgenie 등)는 `backend/internal/alert`의 2-메서드
`alert.Notifier` 인터페이스를 구현하고 `cmd/server/main.go`에 등록하면 됩니다.

## UI 언어

대시보드는 **영어**와 **한국어**를 지원하며, 언어 전환기는 헤더에 있고 선택은
브라우저별로 기억됩니다(미선택 시 브라우저 로케일, 그다음 영어 순으로 폴백).

번역은 Dify와 Grafana가 쓰는 패턴 — react-i18next + 언어당 JSON 파일 하나 —을
따르며, 언어는 [`frontend/src/locales/`](frontend/src/locales/)에서
**자동 발견**됩니다. 새 언어 추가는 등록 절차 없는 단일 파일 기여입니다:
`en.json`을 복사해 값을 번역하고 `meta.name`에 해당 언어의 자국어 표기를 넣으면
끝입니다. 자세한 내용은
[`frontend/src/locales/README.md`](frontend/src/locales/README.md)를 참고하세요.

## 디렉토리 구조

```
backend/
├── cmd/server/          # 진입점: 설정, 배선, graceful shutdown
├── config/              # 환경 변수 로더와 DSN 정규화
└── internal/
    ├── api/             # Gin 라우팅, 컨트롤러, CORS, 에러 envelope
    ├── service/         # 비즈니스 로직: 원시 행 -> 도메인 모델, 알림 규칙
    ├── repository/      # StarRocks 커넥션 풀 + 동적 컬럼 스캔
    ├── alert/           # 알림: Notifier 인터페이스, 매니저, 폴러
    └── model/           # API 노출 타입 (JSON 계약)

frontend/src/
├── components/layout/   # DashboardLayout, Sidebar, Header
├── components/ui/       # shadcn/ui 프리미티브
├── config/              # 내비게이션·라우팅 상수
├── features/topology/   # 클러스터 토폴로지 대시보드
├── features/loads/      # Routine Load 모니터링 + 알림 히스토리
├── hooks/               # TanStack Query 훅
├── lib/                 # axios 클라이언트, 포매터, 쿼리 클라이언트
├── locales/             # UI 번역, 언어당 JSON 하나
├── store/               # Zustand UI 상태
├── types/               # API 계약의 TypeScript 미러
└── i18n.ts              # 로케일 자동 발견 기반 react-i18next 설정
```

## 개발

```bash
make test    # 백엔드 유닛 테스트
make lint    # gofmt 체크 + go vet + oxlint
make build   # API 바이너리(backend/bin) + 프로덕션 웹 번들(frontend/dist)
make check   # 위 전부 — CI가 실행하는 것
```

## 로드맵

- [x] 클러스터 토폴로지 뷰어 (FE/BE 생존 상태, 역할, 태블릿, 디스크·메모리)
- [x] Routine Load 모니터링 — 잡 상태, 에러 행, 근사 지연
- [x] 알림 — 규칙 평가 루프, log + webhook 노티파이어, 테스트 발사 엔드포인트
- [x] UI 다국어 지원 (영어·한국어, 단일 파일로 언어 추가)
- [ ] SQL 워크시트 — Monaco 에디터, 결과 그리드, 쿼리 프로파일
- [ ] 데이터 리니지 — React Flow 기반 베이스 테이블 ↔ Materialized View DAG
- [ ] 메트릭 — ECharts 기반 백엔드별 CPU/메모리 시계열 (Prometheus 클라이언트)
- [ ] 추가 노티파이어 채널 (이메일, PagerDuty) & 알림 규칙 설정 UI
