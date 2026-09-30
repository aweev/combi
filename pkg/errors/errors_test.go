package errors_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	pkgerrors "github.com/combi/pkg/errors"
	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ─── Constructor tests ─────────────────────────────────────────────────────────

func TestNotFound_MessageContainsResource(t *testing.T) {
	err := pkgerrors.NotFound("identity", "idn_01HXYZ")
	if err.Code != pkgerrors.CodeNotFound {
		t.Errorf("code: got %q want NOT_FOUND", err.Code)
	}
	if err.Resource != "identity" {
		t.Errorf("resource: got %q want identity", err.Resource)
	}
	if err.ResourceID != "idn_01HXYZ" {
		t.Errorf("resourceID: got %q want idn_01HXYZ", err.ResourceID)
	}
}

func TestNotFound_WithoutID(t *testing.T) {
	err := pkgerrors.NotFound("payment")
	if err.ResourceID != "" {
		t.Error("ResourceID must be empty when not provided")
	}
}

func TestInvalidInput_PopulatesFields(t *testing.T) {
	err := pkgerrors.InvalidInput("phone", "must be E.164 format")
	if err.Code != pkgerrors.CodeInvalidInput {
		t.Errorf("code: got %q want INVALID_INPUT", err.Code)
	}
	if len(err.Fields) != 1 {
		t.Fatalf("expected 1 field error, got %d", len(err.Fields))
	}
	if err.Fields[0].Field != "phone" {
		t.Errorf("field: got %q want phone", err.Fields[0].Field)
	}
}

func TestConflict(t *testing.T) {
	err := pkgerrors.Conflict("identity", "phone already registered")
	if err.Code != pkgerrors.CodeConflict {
		t.Errorf("code: got %q", err.Code)
	}
}

func TestForbidden(t *testing.T) {
	err := pkgerrors.Forbidden("payments:write")
	if err.Code != pkgerrors.CodeForbidden {
		t.Errorf("code: got %q", err.Code)
	}
}

func TestInternal_SafeMessage(t *testing.T) {
	err := pkgerrors.Internal("postgres connection pool exhausted — host=db1 port=5432")
	// safeMessage must NOT expose the internal details
	_, body := err.ToHTTP()
	if body.Message == err.Error() {
		t.Error("ToHTTP must not expose internal error details to caller")
	}
	if body.Message == "" {
		t.Error("safe message must not be empty")
	}
}

func TestUnavailable(t *testing.T) {
	err := pkgerrors.Unavailable("identity-service")
	if err.Code != pkgerrors.CodeUnavailable {
		t.Errorf("code: got %q", err.Code)
	}
}

func TestRateLimited_WithRetryAfter(t *testing.T) {
	err := pkgerrors.RateLimited("otp", 60)
	if err.RetryAfterSec != 60 {
		t.Errorf("RetryAfterSec: got %d want 60", err.RetryAfterSec)
	}
}

func TestWrap_CauseChain(t *testing.T) {
	cause := fmt.Errorf("pgx: connection refused")
	err := pkgerrors.Internal("db write failed").Wrap(cause)
	if !errors.Is(err, pkgerrors.ErrInternal) {
		t.Error("wrapped error must satisfy errors.Is(ErrInternal)")
	}
	if err.Cause() != cause {
		t.Error("Cause() must return the wrapped error")
	}
}

func TestWithTrace(t *testing.T) {
	err := pkgerrors.NotFound("device").WithTrace("trace-abc123")
	if err.TraceID != "trace-abc123" {
		t.Errorf("TraceID: got %q", err.TraceID)
	}
}

// ─── Domain-specific constructors ─────────────────────────────────────────────

func TestOTPInvalid(t *testing.T) {
	err := pkgerrors.OTPInvalid(2)
	if err.Code != pkgerrors.CodeOTPInvalid {
		t.Errorf("code: got %q", err.Code)
	}
}

func TestPhoneTaken_HasFieldError(t *testing.T) {
	err := pkgerrors.PhoneTaken("+237690000000")
	if len(err.Fields) == 0 {
		t.Fatal("PhoneTaken must have a field error on 'phone'")
	}
	if err.Fields[0].Field != "phone" {
		t.Errorf("field: got %q want phone", err.Fields[0].Field)
	}
}

func TestImmutableField(t *testing.T) {
	err := pkgerrors.ImmutableField("phone")
	if err.Code != pkgerrors.CodeImmutableField {
		t.Errorf("code: got %q", err.Code)
	}
	if len(err.Fields) == 0 || err.Fields[0].Field != "phone" {
		t.Error("ImmutableField must include a field error")
	}
}

// ─── ValidationBuilder ─────────────────────────────────────────────────────────

func TestValidation_NoFieldsReturnsNil(t *testing.T) {
	err := pkgerrors.Validation().Build()
	if err != nil {
		t.Errorf("expected nil when no fields added, got %v", err)
	}
}

func TestValidation_MultipleFields(t *testing.T) {
	err := pkgerrors.Validation().
		Field("phone", "PHONE_INVALID", "must be E.164").
		Field("country", "REQUIRED", "country is required").
		Build()
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if err.Code != pkgerrors.CodeInvalidInput {
		t.Errorf("code: got %q want INVALID_INPUT", err.Code)
	}
	if len(err.Fields) != 2 {
		t.Errorf("fields: got %d want 2", len(err.Fields))
	}
}

func TestValidation_FieldIf_TrueAdds(t *testing.T) {
	err := pkgerrors.Validation().
		FieldIf(true, "email", "INVALID", "bad email").
		FieldIf(false, "name", "REQUIRED", "name required").
		Build()
	if err == nil {
		t.Fatal("expected error")
	}
	if len(err.Fields) != 1 {
		t.Errorf("expected 1 field (condition=false skipped), got %d", len(err.Fields))
	}
}

func TestValidation_HasErrors(t *testing.T) {
	v := pkgerrors.Validation().Field("x", "Y", "z")
	if !v.HasErrors() {
		t.Error("HasErrors() should be true after Field()")
	}
	empty := pkgerrors.Validation()
	if empty.HasErrors() {
		t.Error("HasErrors() should be false before any Field()")
	}
}

// ─── gRPC conversion ──────────────────────────────────────────────────────────

func TestToGRPC_CodeMapping(t *testing.T) {
	cases := []struct {
		afrosCode pkgerrors.Code
		grpcCode  grpccodes.Code
	}{
		{pkgerrors.CodeNotFound, grpccodes.NotFound},
		{pkgerrors.CodeInvalidInput, grpccodes.InvalidArgument},
		{pkgerrors.CodeConflict, grpccodes.AlreadyExists},
		{pkgerrors.CodeUnauthorized, grpccodes.Unauthenticated},
		{pkgerrors.CodeForbidden, grpccodes.PermissionDenied},
		{pkgerrors.CodeRateLimited, grpccodes.ResourceExhausted},
		{pkgerrors.CodeUnavailable, grpccodes.Unavailable},
		{pkgerrors.CodeInternal, grpccodes.Internal},
		{pkgerrors.CodeTimeout, grpccodes.DeadlineExceeded},
		{pkgerrors.CodeNotImplemented, grpccodes.Unimplemented},
		{pkgerrors.CodePreconditionFailed, grpccodes.FailedPrecondition},
		// Domain-specific
		{pkgerrors.CodeTokenExpired, grpccodes.Unauthenticated},
		{pkgerrors.CodeTokenRevoked, grpccodes.Unauthenticated},
		{pkgerrors.CodeIdentitySuspended, grpccodes.FailedPrecondition},
		{pkgerrors.CodeOfflineTokenExpired, grpccodes.FailedPrecondition},
		{pkgerrors.CodeDeviceNotTrusted, grpccodes.FailedPrecondition},
		{pkgerrors.CodeACLForbidden, grpccodes.PermissionDenied},
		{pkgerrors.CodeBusBrokerDown, grpccodes.Unavailable},
		{pkgerrors.CodeOTPRateLimited, grpccodes.ResourceExhausted},
		{pkgerrors.CodeKYCTooManyAttempts, grpccodes.ResourceExhausted},
	}
	for _, c := range cases {
		t.Run(string(c.afrosCode), func(t *testing.T) {
			ae := &pkgerrors.AfrosError{Code: c.afrosCode, Message: "test"}
			grpcErr := ae.ToGRPC()
			st, ok := status.FromError(grpcErr)
			if !ok {
				t.Fatal("ToGRPC must return a gRPC status error")
			}
			if st.Code() != c.grpcCode {
				t.Errorf("gRPC code: got %v want %v", st.Code(), c.grpcCode)
			}
		})
	}
}

// ─── HTTP conversion ──────────────────────────────────────────────────────────

func TestToHTTP_StatusCodeMapping(t *testing.T) {
	cases := []struct {
		afrosCode  pkgerrors.Code
		httpStatus int
	}{
		{pkgerrors.CodeNotFound, 404},
		{pkgerrors.CodeGone, 410},
		{pkgerrors.CodeInvalidInput, 400},
		{pkgerrors.CodeUnprocessable, 400},
		{pkgerrors.CodeConflict, 409},
		{pkgerrors.CodePhoneTaken, 409},
		{pkgerrors.CodeUnauthorized, 401},
		{pkgerrors.CodeTokenExpired, 401},
		{pkgerrors.CodeForbidden, 403},
		{pkgerrors.CodeIdentitySuspended, 403},
		{pkgerrors.CodeRateLimited, 429},
		{pkgerrors.CodeOTPRateLimited, 429},
		{pkgerrors.CodeDeviceLimit, 429},
		{pkgerrors.CodePreconditionFailed, 422},
		{pkgerrors.CodeImmutableField, 422},
		{pkgerrors.CodeOfflineTokenExpired, 422},
		{pkgerrors.CodePayloadTooLarge, 413},
		{pkgerrors.CodeInternal, 500},
		{pkgerrors.CodeUnavailable, 503},
		{pkgerrors.CodeTimeout, 504},
		{pkgerrors.CodeNotImplemented, 501},
	}
	for _, c := range cases {
		t.Run(string(c.afrosCode), func(t *testing.T) {
			ae := &pkgerrors.AfrosError{Code: c.afrosCode, Message: "test"}
			httpStatus, _ := ae.ToHTTP()
			if httpStatus != c.httpStatus {
				t.Errorf("HTTP status: got %d want %d", httpStatus, c.httpStatus)
			}
		})
	}
}

func TestToHTTP_BodyContainsCode(t *testing.T) {
	err := pkgerrors.NotFound("payment", "pay_01HXYZ")
	_, body := err.ToHTTP()
	if body.Code != string(pkgerrors.CodeNotFound) {
		t.Errorf("body.Code: got %q want NOT_FOUND", body.Code)
	}
}

func TestToHTTP_FieldsPreserved(t *testing.T) {
	err := pkgerrors.Validation().
		Field("phone", "PHONE_INVALID", "E.164 required").
		Build()
	_, body := err.ToHTTP()
	if len(body.Fields) != 1 {
		t.Errorf("fields: got %d want 1", len(body.Fields))
	}
}

func TestToHTTP_DocsURLPresent(t *testing.T) {
	_, body := pkgerrors.NotFound("identity").ToHTTP()
	if body.DocsURL == "" {
		t.Error("DocsURL must be populated")
	}
}

func TestWriteHTTP_ProducesValidJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	pkgerrors.NotFound("identity", "idn_01HXYZ").
		WithRequest("req_abc").
		WriteHTTP(rec)

	if rec.Code != http.StatusNotFound {
		t.Errorf("HTTP status: got %d want 404", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct == "" {
		t.Error("Content-Type must be set")
	}
	var envelope map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&envelope); err != nil {
		t.Fatalf("response body is not valid JSON: %v", err)
	}
	if _, ok := envelope["error"]; !ok {
		t.Error("envelope must have 'error' key")
	}
	if _, ok := envelope["data"]; !ok {
		t.Error("envelope must have 'data' key")
	}
}

func TestWriteHTTP_RetryAfterHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	pkgerrors.RateLimited("otp", 60).WriteHTTP(rec)
	if ra := rec.Header().Get("Retry-After"); ra != "60" {
		t.Errorf("Retry-After header: got %q want '60'", ra)
	}
}

// ─── FromGRPC ─────────────────────────────────────────────────────────────────

func TestFromGRPC_Nil(t *testing.T) {
	if pkgerrors.FromGRPC(nil, "identity-service") != nil {
		t.Error("FromGRPC(nil) must return nil")
	}
}

func TestFromGRPC_AlreadyAfrosError(t *testing.T) {
	original := pkgerrors.NotFound("identity")
	got := pkgerrors.FromGRPC(original, "identity-service")
	if got != original {
		t.Error("FromGRPC of existing AfrosError must return it unchanged")
	}
}

func TestFromGRPC_NotFound(t *testing.T) {
	st := status.Error(grpccodes.NotFound, "not found")
	got := pkgerrors.FromGRPC(st, "identity-service")
	if got.Code != pkgerrors.CodeNotFound {
		t.Errorf("code: got %q want NOT_FOUND", got.Code)
	}
}

func TestFromGRPC_Unavailable_CircuitBreak(t *testing.T) {
	// Downstream Unavailable must become local Unavailable
	// — no downstream details leak
	st := status.Error(grpccodes.Unavailable, "downstream internal: redis OOM details")
	got := pkgerrors.FromGRPC(st, "payments-service")
	if got.Code != pkgerrors.CodeUnavailable {
		t.Errorf("code: got %q want UNAVAILABLE", got.Code)
	}
	// The internal details must NOT be in the output message
	if got.Message == "downstream internal: redis OOM details" {
		t.Error("internal downstream details must not leak through FromGRPC")
	}
}

func TestFromGRPC_Internal_TreatedAsUnavailable(t *testing.T) {
	// Downstream Internal becomes local Unavailable (circuit-breaker pattern)
	st := status.Error(grpccodes.Internal, "NullPointerException at line 47")
	got := pkgerrors.FromGRPC(st, "kyc-service")
	if got.Code != pkgerrors.CodeUnavailable {
		t.Errorf("downstream Internal must map to local UNAVAILABLE, got %q", got.Code)
	}
}

func TestFromGRPC_DeadlineExceeded(t *testing.T) {
	st := status.Error(grpccodes.DeadlineExceeded, "context deadline exceeded")
	got := pkgerrors.FromGRPC(st, "fx-service")
	if got.Code != pkgerrors.CodeUnavailable {
		t.Errorf("DeadlineExceeded from downstream must become UNAVAILABLE, got %q", got.Code)
	}
}

func TestFromGRPC_PermissionDenied(t *testing.T) {
	st := status.Error(grpccodes.PermissionDenied, "FORBIDDEN")
	got := pkgerrors.FromGRPC(st, "acl-service")
	if got.Code != pkgerrors.CodeForbidden {
		t.Errorf("code: got %q want FORBIDDEN", got.Code)
	}
}

// ─── errors.Is sentinel matching ─────────────────────────────────────────────

func TestErrorIs_SentinelMatching(t *testing.T) {
	err := pkgerrors.NotFound("device")
	if !errors.Is(err, pkgerrors.ErrNotFound) {
		t.Error("NotFound error must match ErrNotFound sentinel")
	}
	if errors.Is(err, pkgerrors.ErrInternal) {
		t.Error("NotFound error must NOT match ErrInternal sentinel")
	}
}

func TestErrorIs_WrappedError(t *testing.T) {
	cause := fmt.Errorf("raw cause")
	wrapped := pkgerrors.Internal("db failed").Wrap(cause)
	if !errors.Is(wrapped, pkgerrors.ErrInternal) {
		t.Error("wrapped internal error must match ErrInternal sentinel")
	}
}

// ─── Error() string format ────────────────────────────────────────────────────

func TestError_StringFormat(t *testing.T) {
	cases := []struct {
		err     *pkgerrors.AfrosError
		wantSub string // substring that must appear
	}{
		{pkgerrors.NotFound("identity", "idn_01HXYZ"), "NOT_FOUND"},
		{pkgerrors.NotFound("identity", "idn_01HXYZ"), "identity"},
		{pkgerrors.InvalidInput("phone", "bad"), "INVALID_INPUT"},
		{pkgerrors.Internal("db failed"), "INTERNAL"},
	}
	for _, c := range cases {
		s := c.err.Error()
		if s == "" {
			t.Errorf("Error() must not be empty")
		}
		found := false
		for i := 0; i <= len(s)-len(c.wantSub); i++ {
			if s[i:i+len(c.wantSub)] == c.wantSub {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Error() = %q, expected to contain %q", s, c.wantSub)
		}
	}
}

// ─── All codes have both gRPC and HTTP mappings ────────────────────────────────
// This test ensures no code is accidentally left unmapped.

func TestAllCodesHaveGRPCMapping(t *testing.T) {
	allCodes := []pkgerrors.Code{
		pkgerrors.CodeNotFound, pkgerrors.CodeInvalidInput, pkgerrors.CodeConflict,
		pkgerrors.CodeUnauthorized, pkgerrors.CodeForbidden, pkgerrors.CodeRateLimited,
		pkgerrors.CodePreconditionFailed, pkgerrors.CodeUnprocessable, pkgerrors.CodePayloadTooLarge,
		pkgerrors.CodeGone, pkgerrors.CodeInternal, pkgerrors.CodeUnavailable,
		pkgerrors.CodeTimeout, pkgerrors.CodeNotImplemented,
		pkgerrors.CodeOTPInvalid, pkgerrors.CodeOTPExpired, pkgerrors.CodeOTPRateLimited,
		pkgerrors.CodeTokenExpired, pkgerrors.CodeTokenRevoked, pkgerrors.CodeTokenInvalid,
		pkgerrors.CodeSIMSwapLockout, pkgerrors.CodePhoneInvalid, pkgerrors.CodePhoneTaken,
		pkgerrors.CodeHandleTaken, pkgerrors.CodeImmutableField, pkgerrors.CodeDeviceLimit,
		pkgerrors.CodeIdentitySuspended, pkgerrors.CodeIdentityDeleted,
		pkgerrors.CodeKYCDocumentInvalid, pkgerrors.CodeKYCDocumentQuality,
		pkgerrors.CodeKYCWatchlistHit, pkgerrors.CodeKYCMinimumAge, pkgerrors.CodeKYCTooManyAttempts,
		pkgerrors.CodeKYCDuplicate, pkgerrors.CodeInsufficientFunds,
		pkgerrors.CodePaymentLimitExceeded, pkgerrors.CodeCurrencyUnsupported,
		pkgerrors.CodeFXRateUnavailable, pkgerrors.CodeOfflineTokenExpired,
		pkgerrors.CodeOfflineTokenRevoked, pkgerrors.CodeOfflineSyncConflict,
		pkgerrors.CodeOfflineClockSkew, pkgerrors.CodeDeviceNotTrusted,
		pkgerrors.CodeBusTopicNotFound, pkgerrors.CodeBusSchemaInvalid,
		pkgerrors.CodeBusBrokerDown, pkgerrors.CodeBusPayloadTooLarge,
		pkgerrors.CodeACLForbidden, pkgerrors.CodeACLRoleNotFound, pkgerrors.CodeACLRoleInUse,
		pkgerrors.CodeACLSystemRole, pkgerrors.CodeACLRoleExists,
	}

	for _, code := range allCodes {
		t.Run(string(code), func(t *testing.T) {
			ae := &pkgerrors.AfrosError{Code: code, Message: "test"}

			// gRPC mapping must produce a valid gRPC error
			grpcErr := ae.ToGRPC()
			st, ok := status.FromError(grpcErr)
			if !ok {
				t.Fatalf("ToGRPC: not a gRPC status error for code %q", code)
			}
			if st.Code() == grpccodes.OK {
				t.Errorf("ToGRPC: code %q mapped to gRPC OK — must map to an error code", code)
			}

			// HTTP mapping must produce a non-2xx status
			httpStatus, body := ae.ToHTTP()
			if httpStatus < 400 {
				t.Errorf("ToHTTP: code %q mapped to HTTP %d — must be 4xx or 5xx", code, httpStatus)
			}
			if body.Code != string(code) {
				t.Errorf("ToHTTP: body.Code %q != original code %q", body.Code, code)
			}
		})
	}
}
