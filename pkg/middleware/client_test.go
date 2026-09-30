package middleware

import (
	"context"
	"testing"
	"time"

	pkgerrors "github.com/combi/pkg/errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// ─── Client: deadline ────────────────────────────────────────────────────────

func TestDeadlineUnaryClientInterceptorAppliesDefault(t *testing.T) {
	icep := DeadlineUnaryClientInterceptor(50 * time.Millisecond)
	invoker := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		dl, ok := ctx.Deadline()
		if !ok {
			t.Error("interceptor must set a deadline when none exists")
			return nil
		}
		if time.Until(dl) > 100*time.Millisecond {
			t.Errorf("deadline too far out: %v", time.Until(dl))
		}
		return nil
	}
	_ = icep(context.Background(), "/svc/Method", nil, nil, nil, invoker)
}

func TestDeadlineUnaryClientInterceptorRespectsExisting(t *testing.T) {
	icep := DeadlineUnaryClientInterceptor(50 * time.Millisecond)
	invoker := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		dl, _ := ctx.Deadline()
		// The caller's 2s deadline must win over the 50ms default: the
		// remaining time should still be well over one second.
		if time.Until(dl) <= time.Second {
			t.Errorf("caller deadline must be respected, got %v", time.Until(dl))
		}
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = icep(ctx, "/svc/Method", nil, nil, nil, invoker)
}

// ─── Client: metadata forwarding ──────────────────────────────────────────────

func TestMetadataUnaryClientInterceptorForwards(t *testing.T) {
	ctx := WithRequestID(WithTenantID(context.Background(), "ten_9"), "req_fwd")
	icep := MetadataUnaryClientInterceptor()
	invoker := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			t.Fatal("outgoing metadata missing")
		}
		if got := md.Get(HeaderRequestID); len(got) != 1 || got[0] != "req_fwd" {
			t.Errorf("x-request-id = %v", got)
		}
		if got := md.Get(HeaderTenantID); len(got) != 1 || got[0] != "ten_9" {
			t.Errorf("x-tenant-id = %v", got)
		}
		return nil
	}
	_ = icep(ctx, "/svc/Method", nil, nil, nil, invoker)
}

// ─── Client: circuit breaker ─────────────────────────────────────────────────

type recordingBreaker struct {
	called bool
}

func (b *recordingBreaker) Call(ctx context.Context, key string, fn func() error) error {
	b.called = true
	return fn()
}

func TestCircuitBreakerUnaryClientInterceptor(t *testing.T) {
	br := &recordingBreaker{}
	icep := CircuitBreakerUnaryClientInterceptor(br)

	invoker := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		return status.Error(codes.Unavailable, "connection refused")
	}

	err := icep(context.Background(), "/svc/Method", nil, nil, newTestConn(t, "identity-service:50051"), invoker)
	if !br.called {
		t.Error("breaker must wrap the call")
	}
	if status.Code(err) != codes.Unavailable {
		t.Errorf("code = %v, want Unavailable", status.Code(err))
	}
	// The normalized error must decode back into the canonical AfrosError
	// via FromGRPC — that is the cross-process contract.
	afros := pkgerrors.FromGRPC(err, "identity-service:50051")
	if afros == nil || afros.Code != pkgerrors.CodeUnavailable {
		t.Errorf("FromGRPC should rebuild the canonical UNAVAILABLE error, got %v", afros)
	}
}

func TestCircuitBreakerUnaryClientInterceptorNilBreaker(t *testing.T) {
	icep := CircuitBreakerUnaryClientInterceptor(nil)
	invoker := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		return status.Error(codes.NotFound, "nope")
	}
	err := icep(context.Background(), "/svc/Method", nil, nil, newTestConn(t, "x"), invoker)
	if status.Code(err) != codes.NotFound {
		t.Errorf("nil breaker must pass errors through, got %v", status.Code(err))
	}
}

// newTestConn returns a lazily-initialized *grpc.ClientConn whose Target()
// is the given address. grpc.NewClient performs no I/O until the first RPC,
// so this is safe in unit tests.
func newTestConn(t *testing.T, target string) *grpc.ClientConn {
	t.Helper()
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc.NewClient(%q): %v", target, err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}
