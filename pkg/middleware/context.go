package middleware

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"
)

// ─── Wire headers ─────────────────────────────────────────────────────────────
//
// These are the only headers/metadata keys the platform propagates. They match
// the spec's logging and tracing requirements exactly.

const (
	// HeaderRequestID identifies a single request across services.
	HeaderRequestID = "x-request-id"
	// HeaderTenantID lets infrastructure (gateway, mesh) forward the tenant.
	// NOTE: this is transport-level only. Handlers must read the tenant from
	// the context (set from verified JWT claims), never from this header.
	HeaderTenantID = "x-tenant-id"
	// HeaderTraceID propagates the W3C trace id for log correlation.
	HeaderTraceID = "x-trace-id"
)

// ─── Context keys ─────────────────────────────────────────────────────────────

type requestIDKey struct{}
type tenantIDKey struct{}
type identityIDKey struct{}
type traceIDKey struct{}

// ─── Request ID ────────────────────────────────────────────────────────────────

// NewRequestID returns a fresh request id ("req_" + random UUID).
func NewRequestID() string {
	return "req_" + uuid.NewString()
}

// WithRequestID returns a context carrying the request id.
func WithRequestID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, requestIDKey{}, id)
}

// RequestIDFromContext returns the request id, or "" when absent.
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

// ─── Tenant ID ─────────────────────────────────────────────────────────────────

// WithTenantID returns a context carrying the tenant id.
// Only auth middleware (after JWT verification) should call this — see
// WithIdentityFromClaims for the single entry point that handlers trust.
func WithTenantID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, tenantIDKey{}, id)
}

// TenantIDFromContext returns the tenant id, or "" when absent.
func TenantIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(tenantIDKey{}).(string)
	return id
}

// ─── Identity ID ───────────────────────────────────────────────────────────────

// WithIdentityID returns a context carrying the authenticated identity id.
func WithIdentityID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, identityIDKey{}, id)
}

// IdentityIDFromContext returns the authenticated identity id, or "" when the
// caller is unauthenticated (background/system work).
func IdentityIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(identityIDKey{}).(string)
	return id
}

// ─── Trace ID ──────────────────────────────────────────────────────────────────

// WithTraceID returns a context carrying the trace id.
func WithTraceID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, traceIDKey{}, id)
}

// TraceIDFromContext returns the trace id, or "" when absent.
func TraceIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(traceIDKey{}).(string)
	return id
}

// ─── Extraction: gRPC metadata ──────────────────────────────────────────────────

// ContextFromGRPCMetadata builds the propagation context values from incoming
// gRPC metadata. Request and trace ids are transport-level correlation ids and
// are safe to take from the wire; the tenant is NOT read from metadata (it is
// set later from verified JWT claims).
func ContextFromGRPCMetadata(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}
	if vals := md.Get(HeaderRequestID); len(vals) > 0 && vals[0] != "" {
		ctx = WithRequestID(ctx, vals[0])
	}
	if vals := md.Get(HeaderTraceID); len(vals) > 0 && vals[0] != "" {
		ctx = WithTraceID(ctx, vals[0])
	}
	return ctx
}

// InjectIntoGRPCMetadata writes the propagation values onto outgoing gRPC
// metadata so the downstream service can correlate logs.
func InjectIntoGRPCMetadata(ctx context.Context) context.Context {
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = metadata.New(nil)
	} else {
		md = md.Copy()
	}
	if v := RequestIDFromContext(ctx); v != "" {
		md.Set(HeaderRequestID, v)
	}
	if v := TenantIDFromContext(ctx); v != "" {
		md.Set(HeaderTenantID, v)
	}
	if v := TraceIDFromContext(ctx); v != "" {
		md.Set(HeaderTraceID, v)
	}
	return metadata.NewOutgoingContext(ctx, md)
}

// ─── Extraction: HTTP headers ──────────────────────────────────────────────────

// ContextFromHTTPHeaders builds the propagation context values from inbound
// HTTP headers. Same trust rules as the gRPC variant: correlation ids come
// from the wire, the tenant comes only from verified JWT claims.
func ContextFromHTTPHeaders(ctx context.Context, headers map[string][]string) context.Context {
	if vals := headers[HeaderRequestID]; len(vals) > 0 && vals[0] != "" {
		ctx = WithRequestID(ctx, vals[0])
	}
	if vals := headers[HeaderTraceID]; len(vals) > 0 && vals[0] != "" {
		ctx = WithTraceID(ctx, vals[0])
	}
	return ctx
}

// InjectIntoHTTPHeaders writes the propagation values onto outbound HTTP
// headers (used by the client transport when platform services call each
// other over REST).
func InjectIntoHTTPHeaders(ctx context.Context, headers map[string][]string) {
	if v := RequestIDFromContext(ctx); v != "" {
		headers[HeaderRequestID] = []string{v}
	}
	if v := TenantIDFromContext(ctx); v != "" {
		headers[HeaderTenantID] = []string{v}
	}
	if v := TraceIDFromContext(ctx); v != "" {
		headers[HeaderTraceID] = []string{v}
	}
}

// ─── Extraction: JWT claims ────────────────────────────────────────────────────

// claimString reads a string claim from a verified JWT claims map. The "sub"
// claim carries the identity id; "tenant_id" carries the tenant. Non-string
// claim values are formatted into their decimal representation rather than
// silently dropped.
func claimString(claims map[string]interface{}, key string) string {
	v, ok := claims[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case float64: // JSON numbers decode as float64
		return fmt.Sprintf("%.0f", t)
	case int:
		return fmt.Sprintf("%d", t)
	case int64:
		return fmt.Sprintf("%d", t)
	default:
		return ""
	}
}

// WithIdentityFromClaims is the single trusted entry point for binding an
// authenticated identity to the context. It must only be called after the JWT
// has been verified (signature, expiry, issuer) — the auth middleware does
// this; handlers read the values via IdentityIDFromContext/TenantIDFromContext.
//
// Claim names follow the platform convention:
//
//	sub        → identity id
//	tenant_id  → tenant id
func WithIdentityFromClaims(ctx context.Context, claims map[string]interface{}) context.Context {
	if sub := claimString(claims, "sub"); sub != "" {
		ctx = WithIdentityID(ctx, sub)
	}
	if tid := claimString(claims, "tenant_id"); tid != "" {
		ctx = WithTenantID(ctx, tid)
	}
	return ctx
}
