package storage

import (
	"testing"

	"github.com/jmoiron/sqlx"
	redis "github.com/redis/go-redis/v9"
	"go.uber.org/fx"

	"github.com/combi/pkg/cache"
	cachemocks "github.com/combi/pkg/cache/mocks"
	"github.com/combi/pkg/config"
	"github.com/combi/pkg/events"
	eventsmocks "github.com/combi/pkg/events/mocks"
	"github.com/combi/pkg/health"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/monitoring"
	"github.com/combi/pkg/security"
	"context"
	"encoding/json"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"

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
			func() *sqlx.DB { return nil }, // never invoked in validate mode
			func() *redis.Client { return nil }, // never invoked in validate mode
			func() *config.Config { return &config.Config{} },
			func() cache.Service { return cachemocks.NewMockService(t) },
			func() events.Bus { return eventsmocks.NewMockBus(t) },
			func() monitoring.Service { return monitoring.NewRecorder("test") },
			func() security.RandomService { return security.NewRandomService() },
			func() queue.Dispatcher { return fakeDispatcher{} },

		),
	)
	if err != nil {
		t.Fatalf("storage module fx graph invalid: %v", err)
	}
}

// fakeDispatcher satisfies queue.Dispatcher for standalone graph validation.
type fakeDispatcher struct{}

func (fakeDispatcher) Dispatch(_ context.Context, _ core.JobType, _ json.RawMessage, _ ...queue.DispatchOption) (*queue.Job, error) {
	return &queue.Job{}, nil
}
