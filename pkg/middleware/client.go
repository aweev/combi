package middleware

import (
	"context"
	"time"

	pkgerrors "github.com/combi/pkg/errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// DefaultCallTimeout is the spec-mandated deadline for inter-service gRPC
// calls: "Always call with a context.WithTimeout(ctx, 5*time.Second)".
const DefaultCallTimeout = 5 * time.Second

// Breaker abstracts the circuit breaker used by client interceptors. It is
// structurally satisfied by *circuit.CircuitBreaker (and anything with the
// same Call signature) so the middleware package stays dependency-light and
// trivially testable.
type Breaker interface {
	Call(ctx context.Context, key string, fn func() error) error
}

// DefaultUnaryClientInterceptors returns the standard client-side chain for
// cross-process service-to-service calls:
//
//  1. Metadata   — forwards request_id / tenant_id / trace_id
//  2. Deadline    — guarantees the 5s spec deadline when unset
//  3. CircuitBreak — trips on codes.Unavailable, converts to canonical
//     UNAVAILABLE so failures never cascade
//
// In-process calls (direct fx-provided interfaces) do NOT go through this
// chain — only DialService-created connections do.
func DefaultUnaryClientInterceptors(breaker Breaker) []grpc.UnaryClientInterceptor {
	return []grpc.UnaryClientInterceptor{
		MetadataUnaryClientInterceptor(),
		DeadlineUnaryClientInterceptor(DefaultCallTimeout),
		CircuitBreakerUnaryClientInterceptor(breaker),
	}
}

// ─── Metadata forwarding ──────────────────────────────────────────────────────

// MetadataUnaryClientInterceptor forwards the propagation headers onto the
// outgoing call so downstream services can correlate logs and traces.
func MetadataUnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		return invoker(InjectIntoGRPCMetadata(ctx), method, req, reply, cc, opts...)
	}
}

// ─── Deadline ─────────────────────────────────────────────────────────────────

// DeadlineUnaryClientInterceptor ensures every outgoing call carries a
// deadline. When the caller already set one (shorter or longer), it is
// respected; otherwise the given default (spec: 5s) is applied.
func DeadlineUnaryClientInterceptor(timeout time.Duration) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		if _, ok := ctx.Deadline(); !ok {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// ─── Circuit breaking ─────────────────────────────────────────────────────────

// CircuitBreakerUnaryClientInterceptor wraps calls in the breaker keyed by the
// target address. Per the spec: on codes.Unavailable the circuit records a
// failure and the caller receives a canonical UNAVAILABLE error — the failure
// must not cascade.
//
// A nil breaker disables breaking (useful in tests and local development).
func CircuitBreakerUnaryClientInterceptor(breaker Breaker) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		if breaker == nil {
			return invoker(ctx, method, req, reply, cc, opts...)
		}

		key := cc.Target()
		err := breaker.Call(ctx, key, func() error {
			return invoker(ctx, method, req, reply, cc, opts...)
		})

		if status.Code(err) == codes.Unavailable {
			// Normalize so every caller sees the canonical UNAVAILABLE error,
			// not the raw transport failure.
			return pkgerrors.Unavailable(key).ToGRPC()
		}
		return err
	}
}
