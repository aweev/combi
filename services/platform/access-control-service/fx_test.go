package acl

import (
	"testing"

	"github.com/jmoiron/sqlx"
	"go.uber.org/fx"

	"github.com/combi/pkg/cache"
	"github.com/combi/pkg/cache/mocks"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/health"
	"github.com/combi/pkg/log"
)

// TestModuleGraphValidates smoke-tests the module's fx wiring with
// fx.ValidateApp: the full dependency graph must resolve in dry-run mode
// (no constructor runs, no infrastructure needed), so a missing provider
// fails here instead of at host boot.
func TestModuleGraphValidates(t *testing.T) {
	err := fx.ValidateApp(
		Module,
		fx.Provide(
			// Host-equivalent providers the module expects from
			// fxcore.InfraModule / fxcore.ProvidePlatformUtils:
			func() *health.Checker { return health.NewChecker() },
			func() log.Logger { return &log.NoOpLogger{} },
			func() cache.Service { return mocks.NewMockService(t) },
			func() *sqlx.DB { return nil }, // never invoked in validate mode
		),
	)
	if err != nil {
		t.Fatalf("acl module fx graph invalid: %v", err)
	}
}

// TestInterfaceBindings pins the module's public surface: the concrete
// services must satisfy the domain interfaces other modules resolve.
func TestInterfaceBindings(t *testing.T) {
	var _ rbac.AuthorizationService = (*AuthorizationService)(nil)
	var _ rbac.AdminService = (*AdminService)(nil)
	var _ rbac.PermissionRegistrar = (*PermissionRegistrar)(nil)
}
