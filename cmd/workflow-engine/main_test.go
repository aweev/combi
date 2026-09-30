package main

import (
	"testing"

	"go.uber.org/fx"

	"github.com/combi/pkg/config"
)

// TestAppGraphValidates smoke-tests the exact production fx graph of
// workflow-engine (fxcore.InfraModule + Temporal client) using
// fx.ValidateApp: the full dependency graph is resolved in dry-run mode
// without dialing Temporal or starting infrastructure.
func TestAppGraphValidates(t *testing.T) {
	cfg := &config.Config{}
	cfg.Env = "test"

	if err := fx.ValidateApp(appOptions(cfg)...); err != nil {
		t.Fatalf("workflow-engine fx graph invalid: %v", err)
	}
}
