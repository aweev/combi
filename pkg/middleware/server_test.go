package middleware

import (
	"context"
	"errors"
	"testing"

	"github.com/combi/pkg/apperror"
	pkgerrors "github.com/combi/pkg/errors"
	"github.com/combi/pkg/log"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// noopLogger discards everything — interceptor logs are not under test.
type noopLogger struct{}

func (noopLogger) Debug(string, ...log.Attr)        {}
func (noopLogger) Info(string, ...log.Attr)         {}
func (noopLogger) Warn(string, ...log.Attr)         {}
func (noopLogger) Error(string, error, ...log.Attr) {}
func (l noopLogger) With(...log.Attr) log.Logger    { return l }

// ─── Recovery ────────────────────────────────────────────────────────────────

func TestRecoveryUnaryServerInterceptor(t *testing.T) {
	icep := RecoveryUnaryServerInterceptor(noopLogger{})
	handler := func(ctx context.Context, req any) (any, error) {
		panic("boom")
	}

	resp, err := icep(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/svc/Boom"}, handler)
	if err == nil {
		t.Fatal("expected error from panic recovery")
	}
	if resp != nil {
		t.Errorf("expected nil response, got %v", resp)
	}
	if status.Code(err) != codes.Internal {
		t.Errorf("status code = %v, want Internal", status.Code(err))
	}
}

// ─── Error normalization ────────────────────────────────────────────────────

func TestNormalizeErrorAfros(t *testing.T) {
	ctx := WithRequestID(WithTraceID(context.Background(), "tr"), "req_n")
	afros := pkgerrors.NotFound("identity", "idn_x")

	got := normalizeError(ctx, afros)
	st, ok := status.FromError(got)
	if !ok {
		t.Fatalf("expected gRPC status, got %T", got)
	}
	if st.Code() != codes.NotFound {
		t.Errorf("code = %v, want NotFound", st.Code())
	}
	// The canonical AfrosError is preserved for callers that unwrap it.
	var back *pkgerrors.AfrosError
	if !errors.As(afros, &back) || back.Code != pkgerrors.CodeNotFound {
		t.Errorf("original AfrosError should remain intact, got %v", afros)
	}
}

func TestNormalizeErrorAppErrorBridge(t *testing.T) {
	ctx := WithRequestID(context.Background(), "req_b")
	appErr := apperror.New(409, apperror.Conflict, "handle already taken", nil)

	got := normalizeError(ctx, appErr)
	st, _ := status.FromError(got)
	if st.Code() != codes.AlreadyExists {
		t.Errorf("bridged AppError code = %v, want AlreadyExists", st.Code())
	}
}

func TestNormalizeErrorPlainErrorBecomesInternal(t *testing.T) {
	ctx := WithRequestID(context.Background(), "req_p")
	secret := errors.New("database password leaked: hunter2")

	got := normalizeError(ctx, secret)
	st, _ := status.FromError(got)
	if st.Code() != codes.Internal {
		t.Errorf("code = %v, want Internal", st.Code())
	}
	if got, want := st.Message(), "INTERNAL"; got != want {
		t.Errorf("message = %q, want %q — internal details must not leak", got, want)
	}
}

func TestNormalizeErrorPassthroughStatus(t *testing.T) {
	ctx := context.Background()
	original := status.Error(codes.ResourceExhausted, "quota exhausted")

	got := normalizeError(ctx, original)
	if !errors.Is(got, original) {
		t.Error("pre-shaped status errors must pass through unchanged")
	}
}
