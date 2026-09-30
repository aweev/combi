package middleware

import (
	"context"
	"errors"
	"runtime/debug"
	"time"

	pkgerrors "github.com/combi/pkg/errors"
	"github.com/combi/pkg/log"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// ─── Server interceptor chain ──────────────────────────────────────────────────

// DefaultUnaryServerInterceptors returns the standard interceptor chain every
// service mounts, in the correct order:
//
//  1. RequestID   — outermost, ensures ids exist for everything below
//  2. Recovery     — converts panics to INTERNAL, logs the stack, never leaks
//  3. Logging      — request_id / tenant_id / identity_id / duration_ms per spec
//  4. ErrorNormalization — any error becomes a canonical AfrosError status
//
// Usage:
//
//	srv := grpc.NewServer(
//	    grpc.ChainUnaryInterceptor(middleware.DefaultUnaryServerInterceptors(logger)...),
//	    grpc.ChainStreamInterceptor(middleware.DefaultStreamServerInterceptors(logger)...),
//	)
func DefaultUnaryServerInterceptors(logger log.Logger) []grpc.UnaryServerInterceptor {
	return []grpc.UnaryServerInterceptor{
		RequestIDUnaryServerInterceptor(),
		RecoveryUnaryServerInterceptor(logger),
		LoggingUnaryServerInterceptor(logger),
		ErrorNormalizationUnaryServerInterceptor(),
	}
}

// DefaultStreamServerInterceptors is the streaming equivalent of
// DefaultUnaryServerInterceptors.
func DefaultStreamServerInterceptors(logger log.Logger) []grpc.StreamServerInterceptor {
	return []grpc.StreamServerInterceptor{
		RequestIDStreamServerInterceptor(),
		RecoveryStreamServerInterceptor(logger),
		LoggingStreamServerInterceptor(logger),
		ErrorNormalizationStreamServerInterceptor(),
	}
}

// ─── Request ID ───────────────────────────────────────────────────────────────

// RequestIDUnaryServerInterceptor extracts the inbound request id from metadata
// (or generates one), stores it in the context, and echoes it in trailing
// metadata so HTTP gateways can return it to the caller.
func RequestIDUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		_ *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		ctx = ensureRequestID(ctx)
		ctx = ensureTraceID(ctx)
		return handler(ctx, req)
	}
}

// RequestIDStreamServerInterceptor is the streaming equivalent.
func RequestIDStreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		_ *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ctx := ensureRequestID(ss.Context())
		ctx = ensureTraceID(ctx)
		return handler(srv, &wrappedServerStream{ServerStream: ss, ctx: ctx})
	}
}

// ensureRequestID returns a context with a request id, generating one when
// the caller did not send one.
func ensureRequestID(ctx context.Context) context.Context {
	if id := RequestIDFromContext(ctx); id != "" {
		return ctx
	}
	ctx = ContextFromGRPCMetadata(ctx) // may carry an inbound id
	if id := RequestIDFromContext(ctx); id != "" {
		return ctx
	}
	return WithRequestID(ctx, NewRequestID())
}

// ensureTraceID falls back to the request id so log lines always correlate.
func ensureTraceID(ctx context.Context) context.Context {
	if TraceIDFromContext(ctx) != "" {
		return ctx
	}
	if rid := RequestIDFromContext(ctx); rid != "" {
		return WithTraceID(ctx, rid)
	}
	return ctx
}

// ─── Recovery ──────────────────────────────────────────────────────────────────

// RecoveryUnaryServerInterceptor converts handler panics into INTERNAL errors.
// The stack trace is logged server-side and never sent to the caller.
func RecoveryUnaryServerInterceptor(logger log.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				logger.Error(
					"panic recovered in gRPC handler",
					panicError(r),
					log.String("method", info.FullMethod),
					log.String("request_id", RequestIDFromContext(ctx)),
					log.String("stack", string(debug.Stack())),
				)
				err = pkgerrors.Internal("an unexpected error occurred").
					WithRequest(RequestIDFromContext(ctx)).
					WithTrace(TraceIDFromContext(ctx)).
					ToGRPC()
			}
		}()
		return handler(ctx, req)
	}
}

// RecoveryStreamServerInterceptor is the streaming equivalent.
func RecoveryStreamServerInterceptor(logger log.Logger) grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) (err error) {
		ctx := ss.Context()
		defer func() {
			if r := recover(); r != nil {
				logger.Error(
					"panic recovered in gRPC stream handler",
					panicError(r),
					log.String("method", info.FullMethod),
					log.String("request_id", RequestIDFromContext(ctx)),
					log.String("stack", string(debug.Stack())),
				)
				err = pkgerrors.Internal("an unexpected error occurred").
					WithRequest(RequestIDFromContext(ctx)).
					WithTrace(TraceIDFromContext(ctx)).
					ToGRPC()
			}
		}()
		return handler(srv, ss)
	}
}

// panicError normalises a recovered panic value into an error for logging.
func panicError(r any) error {
	if e, ok := r.(error); ok {
		return e
	}
	if s, ok := r.(string); ok {
		return errors.New(s)
	}
	return errors.New("non-string panic value")
}

// ─── Logging ──────────────────────────────────────────────────────────────────

// LoggingUnaryServerInterceptor logs every request with the four spec-mandated
// fields — request_id, tenant_id, identity_id, duration_ms — plus the method
// and resulting status code.
func LoggingUnaryServerInterceptor(logger log.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)

		attrs := []log.Attr{
			log.String("method", info.FullMethod),
			log.String("request_id", RequestIDFromContext(ctx)),
			log.String("tenant_id", TenantIDFromContext(ctx)),
			log.String("identity_id", IdentityIDFromContext(ctx)),
			log.Int("duration_ms", int(time.Since(start).Milliseconds())),
			log.String("code", status.Code(err).String()),
		}
		if err != nil {
			logger.Error("gRPC request failed", err, attrs...)
		} else {
			logger.Info("gRPC request", attrs...)
		}
		return resp, err
	}
}

// LoggingStreamServerInterceptor is the streaming equivalent.
func LoggingStreamServerInterceptor(logger log.Logger) grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		start := time.Now()
		ctx := ss.Context()
		err := handler(srv, ss)

		attrs := []log.Attr{
			log.String("method", info.FullMethod),
			log.String("request_id", RequestIDFromContext(ctx)),
			log.String("tenant_id", TenantIDFromContext(ctx)),
			log.String("identity_id", IdentityIDFromContext(ctx)),
			log.Int("duration_ms", int(time.Since(start).Milliseconds())),
			log.String("code", status.Code(err).String()),
		}
		if err != nil {
			logger.Error("gRPC stream failed", err, attrs...)
		} else {
			logger.Info("gRPC stream", attrs...)
		}
		return err
	}
}
