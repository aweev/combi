package main

import (
	"testing"

	"go.uber.org/fx"

	"github.com/combi/pkg/config"
)

// TestAppGraphValidates smoke-tests the exact production fx graph of
// platform-server (fxcore.InfraModule + gRPC + mounted service modules)
// using fx.ValidateApp: it resolves the full dependency graph in dry-run
// mode without starting any infrastructure or invoking constructors, so a
// missing wiring (e.g. a module needing a provider the host forgot) fails
// here instead of at boot.
func TestAppGraphValidates(t *testing.T) {
	cfg := &config.Config{}
	cfg.Env = "test"
	cfg.Server.GRPCPort = 0 // default 50051 path

	if err := fx.ValidateApp(appOptions(cfg)...); err != nil {
		t.Fatalf("platform-server fx graph invalid: %v", err)
	}
}
