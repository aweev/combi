package middleware

import (
	"context"
	"testing"

	"google.golang.org/grpc/metadata"
)

func TestContextRoundTrip(t *testing.T) {
	ctx := context.Background()
	ctx = WithRequestID(ctx, "req_123")
	ctx = WithTenantID(ctx, "ten_1")
	ctx = WithIdentityID(ctx, "idn_1")
	ctx = WithTraceID(ctx, "trace_1")

	if got := RequestIDFromContext(ctx); got != "req_123" {
		t.Errorf("RequestIDFromContext = %q, want req_123", got)
	}
	if got := TenantIDFromContext(ctx); got != "ten_1" {
		t.Errorf("TenantIDFromContext = %q, want ten_1", got)
	}
	if got := IdentityIDFromContext(ctx); got != "idn_1" {
		t.Errorf("IdentityIDFromContext = %q, want idn_1", got)
	}
	if got := TraceIDFromContext(ctx); got != "trace_1" {
		t.Errorf("TraceIDFromContext = %q, want trace_1", got)
	}
}

func TestContextFromGRPCMetadata(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		HeaderRequestID, "req_md",
		HeaderTraceID, "trace_md",
		// x-tenant-id deliberately present: must NOT be trusted from the wire
		HeaderTenantID, "ten_spoofed",
	))
	ctx = ContextFromGRPCMetadata(ctx)

	if got := RequestIDFromContext(ctx); got != "req_md" {
		t.Errorf("request id from metadata = %q, want req_md", got)
	}
	if got := TraceIDFromContext(ctx); got != "trace_md" {
		t.Errorf("trace id from metadata = %q, want trace_md", got)
	}
	if got := TenantIDFromContext(ctx); got != "" {
		t.Errorf("tenant id from metadata = %q, want empty (must come from JWT only)", got)
	}
}

func TestWithIdentityFromClaims(t *testing.T) {
	claims := map[string]interface{}{
		"sub":       "idn_from_jwt",
		"tenant_id": "ten_from_jwt",
	}
	ctx := WithIdentityFromClaims(context.Background(), claims)

	if got := IdentityIDFromContext(ctx); got != "idn_from_jwt" {
		t.Errorf("identity id = %q, want idn_from_jwt", got)
	}
	if got := TenantIDFromContext(ctx); got != "ten_from_jwt" {
		t.Errorf("tenant id = %q, want ten_from_jwt", got)
	}
}

func TestEnsureRequestIDGeneratesWhenMissing(t *testing.T) {
	ctx := ensureRequestID(context.Background())
	if RequestIDFromContext(ctx) == "" {
		t.Error("ensureRequestID should generate a request id when missing")
	}
	ctx = ensureTraceID(ctx)
	if TraceIDFromContext(ctx) == "" {
		t.Error("ensureTraceID should fall back to the request id")
	}
}
