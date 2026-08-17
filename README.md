# StarLens

**English** | [한국어](README.ko.md)

A management and observability dashboard for [StarRocks](https://www.starrocks.io/)
lakehouse clusters — cluster topology, an intelligent SQL worksheet, and data
skew / lineage visualization in one place.

StarRocks speaks the MySQL wire protocol, so StarLens needs nothing installed on
the cluster: point it at a frontend query port and it reads the catalog directly.

## Stack

| Layer     | Choice                                                            |
| --------- | ----------------------------------------------------------------- |
| Backend   | Go 1.26, Gin, `go-sql-driver/mysql`                               |
| Frontend  | React 19 + Vite (TypeScript), Tailwind CSS v4, shadcn/ui          |
| State     | Zustand (UI state), TanStack Query (server state)                 |
| Visuals   | Monaco Editor, ECharts, React Flow *(upcoming phases)*            |

## Quickstart

```bash
# optional: local single-node StarRocks in Docker (FE query port 9030)
make starrocks-up

# start the API (:8080) and the web dev server (:5173) together; Ctrl-C stops both
make run
```

`make run` auto-loads `backend/.env` when present (copy
[`backend/.env.example`](backend/.env.example)); without it the API dials
`127.0.0.1:9030` as `root`. Run `make help` for every target — `build`, `test`,
`lint`, `check` (everything CI would run), `deps`, `clean`, `starrocks-up/down`.
Ports are overridable: `make run BACKEND_PORT=9090 FRONTEND_PORT=3000`.

### Manual setup (without make)

Two processes: the Go API on `:8080` and the Vite dev server on `:5173`. The dev
server proxies `/api` to the API, so the browser stays same-origin.

### 1. Backend

```bash
cd backend
export STARROCKS_DSN='root:@tcp(127.0.0.1:9030)/information_schema?charset=utf8mb4&timeout=5s'
go run ./cmd/server
```

See [`backend/.env.example`](backend/.env.example) for every setting (pool size,
query timeout, CORS origins, port). The server starts even when StarRocks is
unreachable — cluster endpoints then return `503` with the driver error, which is
usually the actual answer.

```bash
curl localhost:8080/healthz             # is the API up?
curl localhost:8080/api/v1/health       # can it reach StarRocks?
curl localhost:8080/api/v1/topology     # cluster membership
```

### 2. Frontend

```bash
cd frontend
npm install
npm run dev            # http://localhost:5173
```

If the API does not run on `localhost:8080`, set `VITE_PROXY_TARGET`
(see [`frontend/.env.example`](frontend/.env.example)).

### No cluster handy?

A single-node StarRocks exposes the FE query port on 9030:

```bash
docker run -p 9030:9030 -p 8030:8030 -p 8040:8040 \
  --name starrocks -itd starrocks/allin1-ubuntu
```

## API

| Method | Path                        | Description                                        |
| ------ | --------------------------- | -------------------------------------------------- |
| `GET`  | `/healthz`                  | API liveness; ignores cluster state.               |
| `GET`  | `/api/v1/health`            | StarRocks reachability (`503` when unreachable).   |
| `GET`  | `/api/v1/cluster/topology`  | FE/BE/CN membership, roles, liveness, capacity, and the cluster's deployment mode. |
| `GET`  | `/api/v1/topology`          | Alias of the above.                                |
| `GET`  | `/api/v1/loads/routine`     | Routine load jobs across all databases: state, statistics, approximate offset lag. |
| `GET`  | `/api/v1/alerts`            | Fired-alert history (in-memory, newest first).     |
| `POST` | `/api/v1/alerts/test`       | Fires a synthetic alert through every notifier and reports per-channel results. |

Failures share one envelope so the UI can render them uniformly:

```json
{
  "error": {
    "code": "starrocks_unavailable",
    "message": "Could not read cluster topology from StarRocks.",
    "detail": "dial tcp 127.0.0.1:9030: connect: connection refused"
  }
}
```

Topology is derived from `SHOW FRONTENDS`, `SHOW BACKENDS` and
`SHOW COMPUTE NODES`, so **both deployment modes are supported**: shared-nothing
clusters surface their BEs, shared-data clusters surface their CNs (with
warehouse assignment where reported), and health accounts for whichever compute
layer exists. The `runMode` field reports how the cluster is deployed, read from
the FE `run_mode` config: `shared_data`, `shared_nothing` (also inferred for
pre-3.0 releases where the config item does not exist), or `unknown` when the
connecting user lacks the ADMIN privilege to read FE configs.

SHOW column sets drift between StarRocks releases, so every field is read by
name with fallbacks (`IP`/`Host`, `Role`/`IsMaster`, `BackendId`/`ComputeNodeId`)
and missing columns serialize as absent rather than as zero.

Routine load jobs are read from `information_schema.routine_load_jobs`
(StarRocks 3.1+); on older versions StarLens automatically falls back to
sweeping every user database with `SHOW ALL ROUTINE LOAD`. The snapshot's
`source` field records which path served it.

## Alerting

StarLens evaluates alert rules on a background interval and fans results out to
pluggable notifiers. Repeats of the same condition are suppressed for
`ALERT_COOLDOWN` (default 10m). Everything is configured via environment
variables — see [`backend/.env.example`](backend/.env.example).

Built-in rules (routine load):

| Rule                       | Severity | Fires when                                                    |
| -------------------------- | -------- | ------------------------------------------------------------- |
| `routine_load_paused`      | warning  | A job is `PAUSED` (message carries StarRocks' reason).         |
| `routine_load_cancelled`   | critical | A job is `CANCELLED` — ingestion will not resume on its own.   |
| `routine_load_error_ratio` | warning  | `errorRows/totalRows` exceeds `ALERT_ERROR_ROWS_RATIO`.        |
| `routine_load_offset_lag`  | warning  | Approximate lag exceeds `ALERT_MAX_OFFSET_LAG` (opt-in).       |

Built-in notifiers:

- **log** — always on; alerts land in the server log at a severity-matched level.
- **webhook** — set `ALERT_WEBHOOK_URL`. `ALERT_WEBHOOK_FORMAT=generic` posts
  `{"source":"starlens","alert":{...}}`; `slack` posts `{"text": ...}` for
  Slack-compatible incoming webhooks.

Verify a channel end to end with `curl -X POST localhost:8080/api/v1/alerts/test`
or the **Test** button on the Routine Load page.

Adding a channel (email, PagerDuty, Opsgenie, ...) means implementing the
two-method `alert.Notifier` interface in `backend/internal/alert` and
registering it in `cmd/server/main.go`.

## UI languages

The dashboard ships in **English** and **Korean**; the switcher lives in the
header and the choice is remembered per browser (falling back to the browser
locale, then English).

Translations follow the pattern used by Dify and Grafana — react-i18next with
one JSON file per language — and languages are **auto-discovered** from
[`frontend/src/locales/`](frontend/src/locales/): adding one is a single-file
contribution with no registration step. Copy `en.json`, translate the values,
set `meta.name` to the language's native name, done. See
[`frontend/src/locales/README.md`](frontend/src/locales/README.md) for details.

## Layout

```
backend/
├── cmd/server/          # entrypoint: config, wiring, graceful shutdown
├── config/              # environment loader and DSN normalization
└── internal/
    ├── api/             # Gin routing, controllers, CORS, error envelope
    ├── service/         # business logic: raw rows -> domain model, alert rules
    ├── repository/      # StarRocks pool + dynamic-column scanning
    ├── alert/           # alerting: Notifier interface, manager, poller
    └── model/           # API-facing types (the JSON contract)

frontend/src/
├── components/layout/   # DashboardLayout, Sidebar, Header
├── components/ui/       # shadcn/ui primitives
├── config/              # navigation and routing constants
├── features/topology/   # cluster topology dashboard
├── features/loads/      # routine load monitoring + alert history
├── hooks/               # TanStack Query hooks
├── lib/                 # axios client, formatters, query client
├── locales/             # UI translations, one JSON per language
├── store/               # Zustand UI state
├── types/               # TypeScript mirrors of the API contract
└── i18n.ts              # react-i18next setup with locale auto-discovery
```

## Development

```bash
make test    # backend unit tests
make lint    # gofmt check + go vet + oxlint
make build   # API binary (backend/bin) + production web bundle (frontend/dist)
make check   # all of the above — what CI runs
```

## Roadmap

- [x] Cluster topology viewer (FE/BE liveness, roles, tablets, disk & memory)
- [x] Routine load monitoring — job states, error rows, approximate lag
- [x] Alerting — rule evaluation loop, log + webhook notifiers, test-fire endpoint
- [x] UI internationalization (English & Korean, single-file language contributions)
- [ ] SQL worksheet — Monaco editor, result grid, query profile
- [ ] Data lineage — base table ↔ materialized view DAG via React Flow
- [ ] Metrics — per-backend CPU/memory time series via ECharts (Prometheus client)
- [ ] More notifier channels (email, PagerDuty) & alert rule configuration UI

## License

StarLens is licensed under the [Apache License 2.0](LICENSE).
