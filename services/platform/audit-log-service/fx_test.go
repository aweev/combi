package auditlog

import (
	"testing"

	"github.com/jmoiron/sqlx"
	"go.uber.org/fx"

	"github.com/combi/pkg/config"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/health"
	"github.com/combi/pkg/log"
)

// TestModuleGraphValidates smoke-tests the module's fx wiring with
// fx.ValidateApp: the full dependency graph must resolve in dry-run mode
// (no constructor runs, no infrastructure needed), so a missing provider
// fails here instead of at host boot. The optional dependencies (queue
// dispatcher, storage, notification, privacy, permission registrar) are
// intentionally absent — the graph must still validate, matching the
// standalone-host configuration.
func TestModuleGraphValidates(t *testing.T) {
	err := fx.ValidateApp(
		Module,
		fx.Provide(
			// Host-equivalent providers the module expects from
			// fxcore.InfraModule / fxcore.ProvidePlatformUtils:
			func() *config.Config {
				cfg := &config.Config{}
				cfg.Env = "test"
				// 32-byte AES-256 key, matching combi/config.yml's
				// auth.auditEncryptionKey default.
				cfg.Auth.AuditEncryptionKey = "12345678901234567890123456789012"
				return cfg
			},
			func() log.Logger { return &log.NoOpLogger{} },
			func() *health.Checker { return health.NewChecker() },
			func() *sqlx.DB { return nil }, // never invoked in validate mode
		),
	)
	if err != nil {
		t.Fatalf("auditlog module fx graph invalid: %v", err)
	}
}

// TestInterfaceBindings pins the module's public surface: the concrete
// services must satisfy the domain interfaces other modules resolve.
func TestInterfaceBindings(t *testing.T) {
	var _ audit.Service = (*Service)(nil)
	var _ audit.EventRecorder = (*AsyncEventRecorder)(nil)
	var _ audit.ComplianceService = (*ComplianceService)(nil)
}
