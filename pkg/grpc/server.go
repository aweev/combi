// Package grpc is the consolidated gRPC chassis for every Africa OS host:
// a production-ready Server (transport, health protocol, reflection, the
// standard interceptor chain) plus DialService for cross-process calls.
//
// The Server mounts the platform-standard stack out of the box:
//
//   - grpc_health_v1 — the health protocol every infrastructure component
//     is required to expose (foundation services spec). When wired to a
//     pkg/health.Checker via WithHealthChecker, the overall status flips
//     to NOT_SERVING while any registered component (database, redis,
//     event bus, ...) is down.
//   - server reflection — grpcurl & friends work against every host.
//   - the default interceptor chain — request-id → recovery → logging →
//     error normalization → locale (see pkg/middleware and
//     pkg/i18n/middleware).
//
// This package supersedes the legacy pkg/server.RunGRPCServer (deleted in
// the F1 chassis consolidation): hosts either run under fx —
// fxcore.ProvideGrpcServer mounts this chassis with the platform health
// checker wired — or standalone via NewServer + Run.
package grpc

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/combi/pkg/config"
	"github.com/combi/pkg/health"
	i18nmw "github.com/combi/pkg/i18n/middleware"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/middleware"

	"google.golang.org/grpc"
	grpchealth "google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

// DefaultGRPCPort is the standard gRPC listen port used when config leaves
// the port unset.
const DefaultGRPCPort = 50051

// DefaultHealthPollInterval is how often the chassis re-evaluates the
// pkg/health.Checker (when wired) to update the grpc_health_v1 status.
const DefaultHealthPollInterval = 10 * time.Second

// Server wraps the standard gRPC server: lifecycle hooks, the platform
// interceptor chain, the grpc_health_v1 protocol, and reflection.
type Server struct {
	server *grpc.Server
	logger log.Logger
	port   int

	// Health protocol bridge (nil when health registration is skipped).
	healthServer  *grpchealth.Server
	healthChecker *health.Checker
	healthEvery   time.Duration
	healthStop    chan struct{}

	// Construction options.
	rawOpts        []grpc.ServerOption
	skipDefaults   bool
	skipHealth     bool
	skipReflection bool
}

// Option customises the chassis at construction.
type Option func(*Server)

// WithServerOptions appends raw grpc.ServerOptions (e.g. TLS credentials).
// They are applied after the default interceptor chain.
func WithServerOptions(opts ...grpc.ServerOption) Option {
	return func(s *Server) { s.rawOpts = append(s.rawOpts, opts...) }
}

// WithoutDefaultInterceptors disables the standard interceptor chain for
// hosts that must own the full chain themselves.
func WithoutDefaultInterceptors() Option {
	return func(s *Server) { s.skipDefaults = true }
}

// WithoutHealth skips grpc_health_v1 registration (rare; e.g. tests).
func WithoutHealth() Option {
	return func(s *Server) { s.skipHealth = true }
}

// WithoutReflection skips server reflection registration (e.g. hardened
// production hosts that must not advertise their API surface).
func WithoutReflection() Option {
	return func(s *Server) { s.skipReflection = true }
}

// WithHealthChecker bridges the platform health Checker into the gRPC
// health protocol: at every poll interval the overall serving status is
// set from Check(ctx). A non-positive interval uses
// DefaultHealthPollInterval. A nil checker is ignored.
func WithHealthChecker(checker *health.Checker, interval time.Duration) Option {
	return func(s *Server) {
		if checker == nil {
			return
		}
		s.healthChecker = checker
		s.healthEvery = interval
	}
}

// NewServer builds the chassis from config. The server is fully assembled
// (interceptors, health, reflection) before it returns; hosts register
// their services on GetServer().
func NewServer(cfg *config.Config, logger log.Logger, opts ...Option) *Server {
	if logger == nil {
		logger = &log.NoOpLogger{}
	}

	port := DefaultGRPCPort
	if cfg != nil && cfg.Server.GRPCPort > 0 {
		port = cfg.Server.GRPCPort
	}

	s := &Server{
		logger: logger.With(log.String("component", "grpc-server")),
		port:   port,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}

	var serverOpts []grpc.ServerOption
	if !s.skipDefaults {
		// Order matters: request-id outermost so every later stage
		// (recovery, logging, error normalization, locale) sees it; the
		// locale interceptor sits innermost, closest to the handler.
		unary := append(middleware.DefaultUnaryServerInterceptors(s.logger),
			i18nmw.UnaryServerInterceptor())
		stream := append(middleware.DefaultStreamServerInterceptors(s.logger),
			i18nmw.StreamServerInterceptor())
		serverOpts = append(serverOpts,
			grpc.ChainUnaryInterceptor(unary...),
			grpc.ChainStreamInterceptor(stream...),
		)
	}
	serverOpts = append(serverOpts, s.rawOpts...)

	s.server = grpc.NewServer(serverOpts...)

	if !s.skipHealth {
		s.healthServer = grpchealth.NewServer()
		healthpb.RegisterHealthServer(s.server, s.healthServer)
	}
	if !s.skipReflection {
		reflection.Register(s.server)
	}

	return s
}

// Port returns the configured listen port (useful for hosts that log their
// address on startup).
func (s *Server) Port() int {
	return s.port
}

// GetServer returns the underlying gRPC server instance to register services.
func (s *Server) GetServer() *grpc.Server {
	return s.server
}

// Start binds the listener and serves in the background. The overall health
// status is set to SERVING here — not at construction — so that pre-start
// service registration completes first.
func (s *Server) Start() error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", s.port, err)
	}

	if s.healthServer != nil {
		s.healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	}
	s.logger.Info("gRPC server listening", log.Int("port", s.port))

	if s.healthChecker != nil {
		s.startHealthPolling()
	}

	go func() {
		if err := s.server.Serve(lis); err != nil {
			// Serve always returns a non-nil error; grpc.ErrServerStopped
			// is the expected value after Stop.
			s.logger.Debug("gRPC server Serve stopped", log.ErrorAttr(err))
		}
	}()

	return nil
}

// Run is the standalone-host entry point: Start, block until SIGINT or
// SIGTERM, then gracefully Stop. Hosts running under fx use Start/Stop via
// lifecycle hooks instead.
func (s *Server) Run() error {
	if err := s.Start(); err != nil {
		return err
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	s.logger.Info("gRPC server shutting down", log.String("signal", sig.String()))
	s.Stop()
	return nil
}

// Stop announces NOT_SERVING on the health protocol (so load balancers
// drain the host) and gracefully stops, letting in-flight requests finish.
func (s *Server) Stop() {
	s.logger.Info("Stopping gRPC server...")

	if s.healthStop != nil {
		close(s.healthStop)
		s.healthStop = nil
	}
	if s.healthServer != nil {
		s.healthServer.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
	}

	s.server.GracefulStop()
}

// startHealthPolling updates the grpc_health_v1 overall status from the
// platform health checker until Stop is called.
func (s *Server) startHealthPolling() {
	interval := s.healthEvery
	if interval <= 0 {
		interval = DefaultHealthPollInterval
	}

	s.healthStop = make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-s.healthStop:
				return
			case <-ticker.C:
				status := healthpb.HealthCheckResponse_SERVING
				if ok, _ := s.healthChecker.Check(context.Background()); !ok {
					status = healthpb.HealthCheckResponse_NOT_SERVING
				}
				s.healthServer.SetServingStatus("", status)
			}
		}
	}()
}
