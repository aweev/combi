// combi/cmd/workflow-engine — the Temporal host binary (per docs/01-DECISIONS.md
// D-09 binary consolidation).
//
// Temporal workers have their own lifecycle, which is why they get a
// dedicated binary instead of living inside platform-server. This host
// owns the Temporal client and mounts workflow/activity worker modules.
//
// Run from the repo root:
//
//	go run ./cmd/workflow-engine
//
// The Temporal client intentionally blocks until the server is reachable
// (pkg/temporal.NewClient retries ~90s) — without Temporal this host has
// no purpose, so failing loudly beats starting half-alive.
package main

import (
	"fmt"
	"os"

	"go.uber.org/fx"
	"go.temporal.io/sdk/client"

	"github.com/combi/pkg/config"
	"github.com/combi/pkg/fxcore"
	"github.com/combi/pkg/log"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "workflow-engine: FATAL: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// app.Run blocks until SIGINT/SIGTERM (fx handles signals) and runs
	// graceful shutdown through the lifecycle hooks.
	fx.New(appOptions(cfg)...).Run()
	return nil
}

func loadConfig() (*config.Config, error) {
	path := os.Getenv("CONFIG_PATH")
	if path == "" {
		path = "."
	}
	return config.Load(path)
}

// appOptions assembles the workflow-engine fx graph. Kept separate from
// run() so tests can validate the exact production graph via
// fx.ValidateApp without Temporal running.
func appOptions(cfg *config.Config) []fx.Option {
	return []fx.Option{
		fx.Supply(cfg),

		// Worker host: infra WITHOUT the gRPC server (workers are not
		// dialed directly — they pull from Temporal) but WITH the
		// Temporal client, which is this host's reason to exist.
		fxcore.InfraModule,
		fx.Provide(fxcore.ProvideTemporal),

		// ── Worker modules mount here as workflows are ported ───────
		// Each worker module exposes Module, e.g.:
		//
		//   outreach "github.com/combi/outreach-worker"
		//   outreach.Module,
		//
		// Worker modules take client.Client from the graph and
		// register their worker in their fx hooks.

		fx.Invoke(mountWorkers),
	}
}

// mountWorkers logs the boot line once the Temporal client is wired.
// Worker registration happens inside the mounted worker modules.
func mountWorkers(cfg *config.Config, temporalClient client.Client, logger log.Logger) {
	logger.Info("workflow-engine ready",
		log.String("temporal", cfg.Temporal.HostPort),
		log.String("namespace", cfg.Temporal.Namespace))
}
