package grpc

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/combi/pkg/middleware"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ─── Cross-process service dialing ────────────────────────────────────────────
//
// Per the foundation services spec ("How services call each other"):
//
//  1. Get the target service address from an env var (e.g.
//     IDENTITY_SERVICE_ADDR=identity-service:50051)
//  2. Create the gRPC client at startup (not per-request)
//  3. Every call runs with the 5s default deadline (client interceptor)
//  4. On codes.Unavailable the circuit breaks and the caller sees a canonical
//     UNAVAILABLE error — no cascade
//
// In-process calls inside a host binary (e.g. platform-server calling its own
// auth module) must NOT use this factory — they resolve the fx-provided
// Service interface directly. DialService is exclusively for cross-process
// calls (gateway → separate queue-server, app-server → platform, etc.).

// ServiceNameToEnvVar converts a service name to its address env var:
// "identity-service" → "IDENTITY_SERVICE_ADDR".
func ServiceNameToEnvVar(serviceName string) string {
	envName := strings.ToUpper(serviceName)
	envName = strings.ReplaceAll(envName, "-", "_")
	envName = strings.ReplaceAll(envName, ":", "_")
	return envName + "_ADDR"
}

// LookupServiceAddr resolves the gRPC address for a service name via the
// IDENTITY_SERVICE_ADDR-style environment convention. An empty result means
// "not configured — treat as in-process or missing".
func LookupServiceAddr(serviceName string) string {
	return strings.TrimSpace(os.Getenv(ServiceNameToEnvVar(serviceName)))
}

// ClientConfig controls DialService behaviour.
type ClientConfig struct {
	// ServiceName is the target service, e.g. "identity-service".
	ServiceName string

	// Address overrides the env lookup (mainly for tests). When empty,
	// LookupServiceAddr(ServiceName) is used.
	Address string

	// Breaker is the circuit breaker applied to every call. Nil disables
	// breaking (local dev, tests).
	Breaker middleware.Breaker

	// CallTimeout is the per-call deadline default (spec: 5s). Zero means
	// middleware.DefaultCallTimeout.
	CallTimeout time.Duration

	// ExtraInterceptors are appended after the default chain (e.g. otel
	// tracing, i18n locale propagation).
	ExtraInterceptors []grpc.UnaryClientInterceptor
}

// DialService creates a cross-process gRPC client connection with the full
// default client chain (metadata forwarding, deadline, circuit breaking).
//
// Connections are long-lived: create once at startup (fx constructor), close
// via the returned conn (hosts wire it into fx lifecycle). Re-dialing per
// request defeats the circuit breaker and is forbidden by the spec.
func DialService(cfg ClientConfig) (*grpc.ClientConn, error) {
	addr := cfg.Address
	if addr == "" {
		addr = LookupServiceAddr(cfg.ServiceName)
	}
	if addr == "" {
		return nil, fmt.Errorf(
			"grpc client: no address for %q — set %s",
			cfg.ServiceName, ServiceNameToEnvVar(cfg.ServiceName),
		)
	}

	timeout := cfg.CallTimeout
	if timeout == 0 {
		timeout = middleware.DefaultCallTimeout
	}

	chain := append(
		middleware.DefaultUnaryClientInterceptors(cfg.Breaker),
		cfg.ExtraInterceptors...,
	)

	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(chain...),
	)
	if err != nil {
		return nil, fmt.Errorf("grpc client: dial %q failed: %w", addr, err)
	}
	return conn, nil
}
