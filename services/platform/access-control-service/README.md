# access-control-service (fx module — RBAC/ACL)

Ported from `api/internal/services/rbac` (Phase 3, Tier 0) onto the combi
chassis. This is a **library module**, not a standalone binary: a host
binary (`cmd/platform-server`) mounts it via fx.

## What it provides

- **AuthorizationService** — read-side permission checks (RBAC) with a
  Redis cache-aside strategy (15 min TTL) plus an ABAC policy evaluator
  (`isResourceOwner` self-service policy).
- **AdminService** — role CRUD, role<->permission mappings, user<->role
  assignments, permission discovery. Guards system roles, fails deletes
  for roles still in use, audits every mutation, and invalidates caches.
- **PermissionRegistrar** — startup reconciliation of the code-defined
  permission catalog with the database (`SyncWithDataStore`); also
  implements `core.Registry` for the configuration service.
- **CacheInvalidationWorker** — `rbac:invalidate_user_cache` job processor
  that clears per-user permission caches when a role changes.

## Layout

- `fx.go` — fx wiring: repository, services, interface bindings, health
  probe, permission-sync OnStart hook.
- `authorization_service.go`, `admin_service.go`, `policy_evaluator.go`,
  `permission_registrar.go`, `cache_invalidation_worker.go`,
  `container.go` — the service layer (direct construction via
  `NewContainer` is also available for non-fx hosts).
- `*_test.go` — ported unit tests (mocked repository/cache/dispatcher/
  audit).

## Mounting in a host binary

```go
import (
	acl "github.com/combi/acl"
	"github.com/combi/pkg/fxcore"
	"go.uber.org/fx"
)

app := fx.New(
	fxcore.InfraModule,
	fx.Provide(fxcore.ProvideGrpcServer),
	acl.Module,
	// ...other service modules...
)
```

The module requires the `fxcore.InfraModule` providers: `*sqlx.DB`,
`cache.Service` (from `fxcore.ProvidePlatformUtils`), and `log.Logger`.

## Stopgaps (deliberate)

- **Audit** (`audit.EventRecorder`) and **queue dispatcher** are not yet
  provided by the host graph. `AdminService` is nil-safe without them:
  mutations succeed, audit trail and async cache invalidation are
  skipped. Late-bind via `SetAuditRecorder`/`SetDispatcher` on the
  concrete `*acl.AdminService` once those services are ported.
- **Worker not yet polled**: `NewCacheInvalidationWorker` is provided in
  the graph for the future queue/worker host. Until then, cache
  staleness after role changes is bounded by the 15-minute TTL.
- Transport (gRPC handlers for `proto/combi/acl/v1/acl.proto`) is Phase 5
  work.

## Domain & persistence

- Types: `pkg/domain/rbac` (Role, Permission, RoleAssignment, errors, events, jobs)
- Repository: `pkg/persistence/postgres.RBACRepository`
- Migration: `pkg/persistence/migrations/000006_create_rbac_schema`
