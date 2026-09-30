package grpc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/combi/pkg/config"
	"github.com/combi/pkg/health"
	"github.com/combi/pkg/log"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// TestNewServerPort verifies the configured gRPC port is honored — critical
// now that multiple hosts share the consolidated topology: workflow-engine
// must not collide with platform-server's listener, and operators must be
// able to remap ports via config.
func TestNewServerPort(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.GRPCPort = 50999

	srv := NewServer(cfg, &log.NoOpLogger{})
	if srv.Port() != 50999 {
		t.Errorf("Port() = %d, want 50999", srv.Port())
	}
}

// TestNewServerDefaultPort verifies the 50051 fallback when grpcPort is
// unset (zero).
func TestNewServerDefaultPort(t *testing.T) {
	cfg := &config.Config{}
	srv := NewServer(cfg, &log.NoOpLogger{})
	if srv.Port() != 50051 {
		t.Errorf("Port() = %d, want default 50051", srv.Port())
	}
}

// TestServerRegistersHealthAndReflection — the chassis must mount the
// grpc_health_v1 protocol (spec: required by all infrastructure) and
// server reflection by default.
func TestServerRegistersHealthAndReflection(t *testing.T) {
	srv := NewServer(&config.Config{}, &log.NoOpLogger{})
	info := srv.server.GetServiceInfo()

	if _, ok := info["grpc.health.v1.Health"]; !ok {
		t.Error("default chassis must register grpc.health.v1.Health")
	}
	if _, ok := info["grpc.reflection.v1.ServerReflection"]; !ok {
		t.Error("default chassis must register reflection v1")
	}
}

// TestServerWithoutHealthAndReflection verifies the opt-outs.
func TestServerWithoutHealthAndReflection(t *testing.T) {
	srv := NewServer(&config.Config{}, &log.NoOpLogger{},
		WithoutHealth(), WithoutReflection())
	info := srv.server.GetServiceInfo()

	if _, ok := info["grpc.health.v1.Health"]; ok {
		t.Error("WithoutHealth must skip grpc.health.v1 registration")
	}
	if _, ok := info["grpc.reflection.v1.ServerReflection"]; ok {
		t.Error("WithoutReflection must skip reflection registration")
	}
}

// freePort asks the kernel for an unused TCP port.
func freePort(t *testing.T) int {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("freePort: %v", err)
	}
	port := lis.Addr().(*net.TCPAddr).Port
	lis.Close()
	return port
}

func dialHealth(t *testing.T, port int) healthpb.HealthClient {
	t.Helper()
	conn, err := grpc.NewClient(fmt.Sprintf("127.0.0.1:%d", port),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return healthpb.NewHealthClient(conn)
}

// TestServerServesHealthProtocol — end to end: start, the health RPC
// answers SERVING, stop.
func TestServerServesHealthProtocol(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.GRPCPort = freePort(t)

	srv := NewServer(cfg, &log.NoOpLogger{})
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(srv.Stop)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client := dialHealth(t, cfg.Server.GRPCPort)
	resp, err := client.Check(ctx, &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("health check rpc: %v", err)
	}
	if resp.Status != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("status = %v, want SERVING", resp.Status)
	}
}

// TestServerHealthCheckerBridge — with a pkg/health.Checker wired, the
// overall gRPC status flips to NOT_SERVING while a component is down and
// back to SERVING once it recovers.
func TestServerHealthCheckerBridge(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.GRPCPort = freePort(t)

	hc := health.NewChecker()
	hc.Register("flaky", func(context.Context) error { return errors.New("component down") })

	srv := NewServer(cfg, &log.NoOpLogger{},
		WithHealthChecker(hc, 10*time.Millisecond))
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(srv.Stop)

	client := dialHealth(t, cfg.Server.GRPCPort)

	waitForStatus := func(want healthpb.HealthCheckResponse_ServingStatus) error {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		for {
			resp, err := client.Check(ctx, &healthpb.HealthCheckRequest{})
			if err == nil && resp.Status == want {
				return nil
			}
			select {
			case <-ctx.Done():
				return fmt.Errorf("status did not reach %v: %v", want, err)
			case <-time.After(5 * time.Millisecond):
			}
		}
	}

	if err := waitForStatus(healthpb.HealthCheckResponse_NOT_SERVING); err != nil {
		t.Fatal(err)
	}

	hc.Register("flaky", func(context.Context) error { return nil })
	if err := waitForStatus(healthpb.HealthCheckResponse_SERVING); err != nil {
		t.Fatal(err)
	}
}
