# combi/pkg — shared utility kit

Shared Go packages for all combi services. Consumed via the workspace
(`go.work`) or a local `replace github.com/combi/pkg => <rel>/pkg`.

## Layout

| Package | Purpose |
|---|---|
| `apperror` | Platform error model (code, HTTP status, retryability) |
| `cache`, `redis` | Cache service, distributed locks, idempotency store |
| `circuit` | Circuit breaker |
| `compliance` | KYC tiers, limits, rules loader |
| `config` | Viper-based config (`NGO_` env overrides, `Validate()` fail-fast, prod secret validation) |
| `persistence` | Postgres (sqlx) connection factory (`persistence/postgres`) |
| `domain` | Shared domain types (`domain/query` cursor pagination, video, audit, …) |
| `envelope` | Canonical API response envelope (`data`/`meta`/`error`/`events`) |
| `errors` | Error → AppError / HTTP / gRPC-status mapping (Afros codes) |
| `events` | Event bus interface, NATS JetStream impl, `events.Emit` metadata helper |
| `fxcore` | fx modules: `InfraModule` (no gRPC/Temporal), `Module` (full) |
| `geo` | Address, country, pluscode, timezone utilities |
| `grpc` | Server wrapper (config-driven port) + `DialService` client factory |
| `health` | Health checker (component registry, parallel checks) |
| `i18n` | Locale, catalog, currency, plural rules, date/number formatting |
| `id` | Prefixed ULID identifiers (`idn_01ARZ…`, `New`/`MustNew`/`Parse`) |
| `identity` | Identity document utilities |
| `limiter` | Redis rate limiter |
| `log` | Logger interface + zap adapter (+ `NoOpLogger` for tests) |
| `middleware` | HTTP middleware: request/tenant/trace context, wire headers |
| `money` | Currency-safe money type |
| `monitoring` | Metrics recorder |
| `oauth`, `security` | OAuth providers, passwords, random, crypto |
| `queue` | Redis stream job broker |
| `realtime` | Realtime (WS) helpers |
| `secret` | Secret management |
| `telemetry`, `trace` | Tracing providers, exporters |
| `temporal` | Temporal client with retry |
| `testutil` | Test DB/Redis helpers |

## Key patterns

- **Envelope** (`envelope`): every HTTP response is `{data, meta, error, events}`;
  `error` is null on success, `data` null on error; internal details never leak.
- **Cursor pagination** (`domain/query`): `start_after`+`limit` in,
  `next_cursor` out; keyset pagination (sort value + ID tie-breaker); invalid
  cursors are rejected as INVALID_INPUT, never silently restarted.
- **Event metadata** (`events.Emit`): fills `request_id`/`tenant_id`/`trace_id`
  from middleware context — background jobs simply omit them.
- **gRPC clients** (`grpc.DialService`): cross-process only, env-var
  address lookup, detached status errors. In-process callers resolve the
  fx-provided Service interface directly.
- **fx modules** (`fxcore`): hosts mount `InfraModule` (+ `ProvideGrpcServer`
  when serving); Temporal is opt-in because its client retries ~90s.

## Topology (see ../docs/01-DECISIONS.md, D-09)

3 kinds of host binaries, services are library modules (no main package):

- `cmd/platform-server` — all platform services, gRPC :50051
- `cmd/workflow-engine` — Temporal workers
- `cast-<app>` — one per CAST app (services/apps/cast/*)

Generate a new service module with:

    go run ./cmd/service-init -tier platform <name>

Hosts read `config.yml` at the repo root (see ../config.yml).
