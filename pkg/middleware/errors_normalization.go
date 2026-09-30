package middleware

import (
	"context"

	"github.com/combi/pkg/apperror"
	pkgerrors "github.com/combi/pkg/errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ─── Error normalization ───────────────────────────────────────────────────────

// ErrorNormalizationUnaryServerInterceptor converts any handler error into a
// canonical AfrosError-backed gRPC status:
//
//   - *errors.AfrosError     → ToGRPC() (request/trace ids attached)
//   - *apperror.AppError     → bridged via errors.FromAppError (legacy ported code)
//   - already a gRPC status  → passed through unchanged
//   - anything else          → INTERNAL with the original error logged, never leaked
func ErrorNormalizationUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		_ *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		resp, err := handler(ctx, req)
		if err == nil {
			return resp, nil
		}
		return resp, normalizeError(ctx, err)
	}
}

// ErrorNormalizationStreamServerInterceptor is the streaming equivalent.
func ErrorNormalizationStreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		_ *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		if err := handler(srv, ss); err != nil {
			return normalizeError(ss.Context(), err)
		}
		return nil
	}
}

// normalizeError maps an arbitrary handler error onto the canonical error
// system, attaching the request/trace correlation ids.
func normalizeError(ctx context.Context, err error) error {
	requestID := RequestIDFromContext(ctx)
	traceID := TraceIDFromContext(ctx)

	// Canonical AfrosError — possibly wrapped anywhere in the chain.
	if afros := pkgerrors.FromError(err); afros != nil {
		return afros.
			WithRequest(requestID).
			WithTrace(traceID).
			ToGRPC()
	}

	// Legacy AppError from ported service code — bridge to the canonical
	// format so both produce the same wire representation.
	if app := apperror.AsAfros(err); app != nil {
		return app.
			WithRequest(requestID).
			WithTrace(traceID).
			ToGRPC()
	}

	if _, ok := status.FromError(err); ok && status.Code(err) != codes.Unknown {
		// Already transport-shaped (including handler-returned statuses) —
		// pass through untouched.
		return err
	}

	// Unknown error shape: log-worthy, never leak the text to callers.
	return pkgerrors.Internal("an unexpected error occurred").
		WithRequest(requestID).
		WithTrace(traceID).
		Wrap(err).
		ToGRPC()
}

// ─── wrappedServerStream ───────────────────────────────────────────────────────

// wrappedServerStream lets interceptors replace a stream's context.
type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedServerStream) Context() context.Context { return w.ctx }
