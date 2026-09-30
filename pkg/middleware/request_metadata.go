package middleware

import (
	"context"
)

// ─── Request metadata (caller network info) ────────────────────────────────────
//
// RequestMetadata carries the caller's network-level info (IP address and
// user agent) through the context so SERVICE-layer code can record it
// (e.g. auth session creation, audit trails) without importing the HTTP
// transport layer.
//
// Absorbed from api/internal/ports/http/middleware/metadata.go during the
// auth-service port: the service layer previously read this via the ports
// package, which would invert the dependency direction (services must never
// import ports). The context carrier lives here in the Core kit; the HTTP
// injector middleware itself (which needs net/http) belongs to the Phase 5
// ports layer and will populate these values on ingress.

// RequestMetadata holds common metadata extracted from an incoming request.
type RequestMetadata struct {
	IPAddress string
	UserAgent string
}

type requestMetadataKey struct{}

// WithRequestMetadata returns a context carrying the caller's request
// metadata. Transport adapters (HTTP middleware in Phase 5, gRPC interceptors,
// workers synthesising internal calls) are the only writers; services only read.
func WithRequestMetadata(ctx context.Context, meta RequestMetadata) context.Context {
	return context.WithValue(ctx, requestMetadataKey{}, &meta)
}

// GetRequestMetadata retrieves the RequestMetadata from the context, or nil
// when no transport populated it (background jobs, tests, internal calls).
// Callers must nil-check and fall back to a sane default.
func GetRequestMetadata(ctx context.Context) *RequestMetadata {
	meta, _ := ctx.Value(requestMetadataKey{}).(*RequestMetadata)
	return meta
}
