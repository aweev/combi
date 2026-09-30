# audit-log-service (fx module — Audit & Compliance)

Ported from `api/internal/services/audit` (Phase 3, Tier 1) onto the combi
chassis. This is a **library module**, not a standalone binary: a host
binary (`cmd/platform-server`) mounts it via fx.

## What it provides

- **AsyncEventRecorder** — fire-and-forget `audit.EventRecorder` for every
  other service: encrypts the sensitive payload (AES-GCM), then enqueues an
  `audit:store_event` job on the `critical` queue (5 attempts). Never fails
  the caller: on any error the full encrypted payload is logged as a
  last-resort fallback.
- **Service** — audit trail read side: fetch/search with at-rest
  decryption, and `ExportEvents` which dispatches an `audit:export` job on
  the `reports` queue. Also implements `reporting.DataFetcher` so the
  reporting service can render audit data.
- **ComplianceService** — legal holds (apply/release, idempotent), data
  retention policy CRUD, and `AnonymizeUser` (GDPR right-to-be-forgotten)
  which initiates an erasure via the privacy manager and dispatches the
  `compliance:anonymize_user` job.
- **Workers** — `StoreWorker` (`audit:store_event`), `ExportWorker`
  (`audit:export`, CSV/JSONL → storage upload → notification),
  `ComplianceWorker` (`compliance:anonymize_user` → privacy manager).

## Layout

- `fx.go` — fx wiring: repository, module-owned crypto/enqueuer adapters,
  services + interface bindings, workers, health probe, permission
  registration OnStart.
- `service.go`, `recorder.go`, `compliance_service.go`,
  `compliance_worker.go`, `store_worker.go`, `export_worker.go`,
  `container.go`, `manifest.go` — the service layer (direct construction
  via `NewContainer` is also available for non-fx hosts).
- `errors.go` — honest 503s for not-yet-mounted dependencies.
- `*_test.go` — ported unit tests (mocked repository/crypto/queue/storage/
  notification/privacy).

## Mounting in a host binary

```go
import (
	auditlog "github.com/combi/auditlog"
	"github.com/combi/pkg/fxcore"
	"go.uber.org/fx"
)

app := fx.New(
	fxcore.InfraModule,
	fx.Provide(fxcore.ProvideGrpcServer),
	auditlog.Module,
	// ...other service modules...
)
```

The module requires the `fxcore.InfraModule` providers: `*config.Config`
(supplied), `*sqlx.DB`, `log.Logger`, `*health.Checker`, and a non-empty
`auth.auditEncryptionKey` (16/24/32 bytes; see `combi/config.yml` or
`NGO_AUTH_AUDITENCRYPTIONKEY`).

In `cmd/platform-server` the mounted module also closes the RBAC stopgap
from the acl port: the host's `bindAuditRecorder` invoke late-binds this
module's recorder into `acl.AdminService.SetAuditRecorder`, so RBAC
mutations are audit-logged from boot on.

## Stopgaps (deliberate)

- **Optional dependencies** (`queue.Dispatcher`, `storage.Service`,
  `notification.Service`, `privacy.Manager`, `rbac.PermissionRegistrar`)
  arrive nil from the graph until their modules are ported. The service
  layer is nil-safe: query/read paths work fully; export, anonymization,
  and export-file delivery return `AUDIT_DISPATCHER_NOT_CONFIGURED` /
  `AUDIT_PRIVACY_NOT_CONFIGURED` (503) or fail non-retryably in the
  affected workers. Late-bind via `SetDispatcher`/`Inject*` on the concrete
  types (or mount the missing modules) to activate them.
- **Workers not yet polled**: the three job processors are provided in the
  graph for the future queue/worker host; nothing consumes them until
  then.
- Transport (gRPC handlers for `proto/combi/audit/v1/audit.proto`, already
  generated in `gen/go`) is Phase 5 work.

## Domain & persistence

- Types: `pkg/domain/audit` (Event, SensitiveData, LegalHold,
  RetentionPolicy, filters, errors, jobs, permissions)
- Repository: `pkg/persistence/postgres.AuditRepository`
- Migration: `pkg/persistence/migrations` (audit schema, already in pkg)
- Crypto: `pkg/security.NewAesGcmCryptoService` keyed from
  `auth.auditEncryptionKey`