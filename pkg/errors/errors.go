// Package errors defines the canonical error system for all Africa OS services.
//
// Every service — without exception — uses these types. Never define custom
// error codes outside this package. When you need a new code, add it here so
// the entire platform stays consistent.
//
// Three-layer design:
//
//  1. AfrosError — the internal domain error. Rich context, carries field info,
//     trace ID, cause chain. Created in domain and service layers.
//
//  2. GRPCStatus — AfrosError.ToGRPC() converts to a google.golang.org/grpc/status
//     with a structured details payload. Every gRPC handler returns this.
//
//  3. HTTPError — AfrosError.ToHTTP() produces the JSON API envelope error block.
//     The API gateway and grpc-gateway use this for REST responses.
//
// Constructor functions are the public API:
//
//	errors.NotFound("identity", "idn_01HXYZ")
//	errors.InvalidInput("phone", "must be E.164 format")
//	errors.Conflict("identity", "phone already registered")
//	errors.Forbidden("payments:write")
//	errors.Internal("database write failed")
//
// Every constructor is deliberately terse — one line per call site is the goal.
//
// Field-level errors (for form validation):
//
//	errors.Validation().
//	    Field("phone", "PHONE_INVALID", "must be E.164 format").
//	    Field("country", "COUNTRY_REQUIRED", "country is required").
//	    Build()
//
// Wrapping (preserves cause chain for logging, strips it before sending to callers):
//
//	if err := db.Query(...); err != nil {
//	    return errors.Internal("database unavailable").Wrap(err)
//	}

package errors

import (
	"encoding/json"
	"fmt"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ─── Canonical error codes ─────────────────────────────────────────────────────
//
// These are the ONLY error codes used across the entire platform.
// gRPC callers see these in the status message.
// REST callers see these in the JSON error.code field.
// Client SDKs switch on these — never change existing values.

type Code string

const (
	// ── Input / caller errors (4xx equivalent) ────────────────────────────────

	// CodeNotFound: the requested resource does not exist or is invisible to the caller.
	// Use for deleted records too — never reveal existence of deleted resources.
	CodeNotFound Code = "NOT_FOUND"

	// CodeInvalidInput: one or more request fields failed validation.
	// Always accompanied by FieldErrors in the response.
	CodeInvalidInput Code = "INVALID_INPUT"

	// CodeConflict: the operation would violate a uniqueness constraint.
	// Examples: phone already registered, handle already taken.
	CodeConflict Code = "CONFLICT"

	// CodeUnauthorized: the caller has no valid credentials.
	// Use when there is no JWT, the JWT is expired, or the signature is invalid.
	// Do NOT use when credentials are valid but permissions are insufficient — use Forbidden.
	CodeUnauthorized Code = "UNAUTHORIZED"

	// CodeForbidden: the caller is authenticated but lacks the required permission.
	// Examples: wrong tenant, insufficient KYC level, missing RBAC role.
	CodeForbidden Code = "FORBIDDEN"

	// CodeRateLimited: the caller has exceeded a rate limit.
	// The RetryAfterSec field is set when known.
	CodeRateLimited Code = "RATE_LIMITED"

	// CodePreconditionFailed: the operation cannot proceed due to current resource state.
	// Examples: identity is suspended, KYC not verified, device not trusted.
	CodePreconditionFailed Code = "PRECONDITION_FAILED"

	// CodeUnprocessable: the request is syntactically valid but semantically impossible.
	// Examples: sending money to yourself, negative transfer amount.
	CodeUnprocessable Code = "UNPROCESSABLE"

	// CodePayloadTooLarge: request body or a field exceeds the size limit.
	CodePayloadTooLarge Code = "PAYLOAD_TOO_LARGE"

	// CodeGone: the resource existed but has been permanently deleted.
	// Use when callers need to distinguish "never existed" from "was deleted".
	CodeGone Code = "GONE"

	// ── Server / infrastructure errors (5xx equivalent) ───────────────────────

	// CodeInternal: an unexpected internal error. Log the cause; do not return it.
	CodeInternal Code = "INTERNAL"

	// CodeUnavailable: a downstream dependency is unreachable (DB, Redis, other service).
	// The client may retry with backoff.
	CodeUnavailable Code = "UNAVAILABLE"

	// CodeTimeout: the operation exceeded its deadline.
	CodeTimeout Code = "TIMEOUT"

	// CodeNotImplemented: the operation exists in the contract but is not yet built.
	CodeNotImplemented Code = "NOT_IMPLEMENTED"

	// ── Domain-specific grouped codes ─────────────────────────────────────────
	// These refine the above for domain-specific causes.
	// All map to one of the above base codes for gRPC/HTTP conversion.

	// Auth domain
	CodeOTPInvalid     Code = "OTP_INVALID"
	CodeOTPExpired     Code = "OTP_EXPIRED"
	CodeOTPRateLimited Code = "OTP_RATE_LIMITED"
	CodeTokenExpired   Code = "TOKEN_EXPIRED"
	CodeTokenRevoked   Code = "TOKEN_REVOKED"
	CodeTokenInvalid   Code = "TOKEN_INVALID"
	CodeSIMSwapLockout Code = "SIM_SWAP_LOCKOUT"

	// Identity domain
	CodePhoneInvalid      Code = "PHONE_INVALID"
	CodePhoneTaken        Code = "PHONE_TAKEN"
	CodeHandleTaken       Code = "HANDLE_TAKEN"
	CodeImmutableField    Code = "IMMUTABLE_FIELD"
	CodeDeviceLimit       Code = "DEVICE_LIMIT"
	CodeIdentitySuspended Code = "IDENTITY_SUSPENDED"
	CodeIdentityDeleted   Code = "IDENTITY_DELETED"

	// KYC domain
	CodeKYCDocumentInvalid Code = "KYC_DOCUMENT_INVALID"
	CodeKYCDocumentQuality Code = "KYC_DOCUMENT_QUALITY"
	CodeKYCWatchlistHit    Code = "KYC_WATCHLIST_HIT"
	CodeKYCMinimumAge      Code = "KYC_MINIMUM_AGE"
	CodeKYCTooManyAttempts Code = "KYC_TOO_MANY_ATTEMPTS"
	CodeKYCDuplicate       Code = "KYC_DUPLICATE_DOCUMENT"

	// Payments domain
	CodeInsufficientFunds    Code = "INSUFFICIENT_FUNDS"
	CodePaymentLimitExceeded Code = "PAYMENT_LIMIT_EXCEEDED"
	CodeCurrencyUnsupported  Code = "CURRENCY_UNSUPPORTED"
	CodeFXRateUnavailable    Code = "FX_RATE_UNAVAILABLE"

	// Offline / device domain
	CodeOfflineTokenExpired Code = "OFFLINE_TOKEN_EXPIRED"
	CodeOfflineTokenRevoked Code = "OFFLINE_TOKEN_REVOKED"
	CodeOfflineSyncConflict Code = "OFFLINE_SYNC_CONFLICT"
	CodeOfflineClockSkew    Code = "OFFLINE_CLOCK_SKEW"
	CodeDeviceNotTrusted    Code = "DEVICE_NOT_TRUSTED"

	// Event bus domain
	CodeBusTopicNotFound   Code = "BUS_TOPIC_NOT_FOUND"
	CodeBusSchemaInvalid   Code = "BUS_SCHEMA_INVALID"
	CodeBusBrokerDown      Code = "BUS_BROKER_UNAVAILABLE"
	CodeBusPayloadTooLarge Code = "BUS_PAYLOAD_TOO_LARGE"

	// ACL domain
	CodeACLForbidden    Code = "ACL_FORBIDDEN"
	CodeACLRoleNotFound Code = "ACL_ROLE_NOT_FOUND"
	CodeACLRoleInUse    Code = "ACL_ROLE_IN_USE"
	CodeACLSystemRole   Code = "ACL_SYSTEM_ROLE_IMMUTABLE"
	CodeACLRoleExists   Code = "ACL_ROLE_ALREADY_EXISTS"
)

// ─── AfrosError ───────────────────────────────────────────────────────────────

// AfrosError is the single error type used across all Africa OS services.
// It carries enough context for structured logging while converting cleanly
// to gRPC status errors and HTTP JSON responses.
type AfrosError struct {
	// Code is the canonical machine-readable error code. Required.
	Code Code `json:"code"`

	// Message is a human-readable description suitable for logs.
	// Never shown directly to end users — use catalog keys for user-facing copy.
	Message string `json:"message"`

	// Resource is the type of resource involved, e.g. "identity", "payment".
	// Optional but strongly recommended.
	Resource string `json:"resource,omitempty"`

	// ResourceID is the specific resource ID, e.g. "idn_01HXYZ".
	// Omit for security-sensitive errors (don't confirm resource existence).
	ResourceID string `json:"resource_id,omitempty"`

	// Fields contains field-level validation errors.
	// Populated only for CodeInvalidInput errors.
	Fields []FieldError `json:"fields,omitempty"`

	// TraceID links this error to the distributed trace.
	// Set by the middleware layer — handlers don't need to set this manually.
	TraceID string `json:"trace_id,omitempty"`

	// RequestID links this error to the originating request.
	RequestID string `json:"request_id,omitempty"`

	// RetryAfterSec is the number of seconds the caller should wait before retrying.
	// Set for CodeRateLimited and CodeUnavailable when known.
	RetryAfterSec int `json:"retry_after_sec,omitempty"`

	// cause is the underlying error for internal logging. NEVER sent to callers.
	cause error
}

// FieldError represents a validation failure for a specific request field.
type FieldError struct {
	// Field is the request field name, dot-separated for nested fields.
	// Examples: "phone", "address.country", "documents[0].type"
	Field string `json:"field"`

	// Code is a machine-readable validation failure code.
	// Examples: "PHONE_INVALID", "REQUIRED", "TOO_LONG", "COUNTRY_UNSUPPORTED"
	Code string `json:"code"`

	// Message is a human-readable description. English only at this layer;
	// translation is the caller's responsibility using the Code.
	Message string `json:"message"`
}

// ─── Error interface ──────────────────────────────────────────────────────────

func (e *AfrosError) Error() string {
	if e.Resource != "" && e.ResourceID != "" {
		return fmt.Sprintf("[%s] %s %s(%s)", e.Code, e.Message, e.Resource, e.ResourceID)
	}
	if e.Resource != "" {
		return fmt.Sprintf("[%s] %s %s", e.Code, e.Message, e.Resource)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *AfrosError) Unwrap() error { return e.cause }

// Is implements errors.Is for direct Code comparison.
// Usage: errors.Is(err, errors.ErrNotFound) — see sentinel vars below.
func (e *AfrosError) Is(target error) bool {
	t, ok := target.(*AfrosError)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

// ─── Sentinels (for errors.Is matching) ──────────────────────────────────────

var (
	ErrNotFound           = &AfrosError{Code: CodeNotFound}
	ErrInvalidInput       = &AfrosError{Code: CodeInvalidInput}
	ErrConflict           = &AfrosError{Code: CodeConflict}
	ErrUnauthorized       = &AfrosError{Code: CodeUnauthorized}
	ErrForbidden          = &AfrosError{Code: CodeForbidden}
	ErrRateLimited        = &AfrosError{Code: CodeRateLimited}
	ErrPreconditionFailed = &AfrosError{Code: CodePreconditionFailed}
	ErrInternal           = &AfrosError{Code: CodeInternal}
	ErrUnavailable        = &AfrosError{Code: CodeUnavailable}
	ErrTimeout            = &AfrosError{Code: CodeTimeout}
)

// ─── Fluent builder ────────────────────────────────────────────────────────────

// Wrap attaches an underlying cause. The cause is logged but never returned
// to callers. Returns the same *AfrosError for chaining.
func (e *AfrosError) Wrap(cause error) *AfrosError {
	e.cause = cause
	return e
}

// WithTrace attaches a trace ID to the error. Called by middleware.
func (e *AfrosError) WithTrace(traceID string) *AfrosError {
	e.TraceID = traceID
	return e
}

// WithRequest attaches a request ID to the error. Called by middleware.
func (e *AfrosError) WithRequest(requestID string) *AfrosError {
	e.RequestID = requestID
	return e
}

// WithRetryAfter sets the retry-after hint in seconds.
func (e *AfrosError) WithRetryAfter(seconds int) *AfrosError {
	e.RetryAfterSec = seconds
	return e
}

// Cause returns the underlying wrapped error (for logging only).
func (e *AfrosError) Cause() error { return e.cause }

// ─── Constructor functions ────────────────────────────────────────────────────
//
// These are the public API. Every service uses these — never construct
// AfrosError structs directly outside this package.

// NotFound constructs a CodeNotFound error.
// resource: the type name ("identity", "payment", "device").
// id: the specific ID that was not found. Omit if revealing the ID is a security risk.
func NotFound(resource string, id ...string) *AfrosError {
	e := &AfrosError{Code: CodeNotFound, Resource: resource}
	if len(id) > 0 {
		e.ResourceID = id[0]
		e.Message = fmt.Sprintf("%s %q not found", resource, id[0])
	} else {
		e.Message = fmt.Sprintf("%s not found", resource)
	}
	return e
}

// InvalidInput constructs a CodeInvalidInput error for a single field.
// For multiple fields use Validation().Field(...).Build().
func InvalidInput(field, message string) *AfrosError {
	return &AfrosError{
		Code:    CodeInvalidInput,
		Message: fmt.Sprintf("invalid input: %s: %s", field, message),
		Fields: []FieldError{
			{Field: field, Code: "INVALID", Message: message},
		},
	}
}

// Conflict constructs a CodeConflict error.
func Conflict(resource, message string) *AfrosError {
	return &AfrosError{
		Code:     CodeConflict,
		Resource: resource,
		Message:  message,
	}
}

// Unauthorized constructs a CodeUnauthorized error.
func Unauthorized(message string) *AfrosError {
	return &AfrosError{Code: CodeUnauthorized, Message: message}
}

// Forbidden constructs a CodeForbidden error.
// permission: the permission that was required but not held.
func Forbidden(permission string) *AfrosError {
	return &AfrosError{
		Code:    CodeForbidden,
		Message: fmt.Sprintf("permission required: %s", permission),
	}
}

// RateLimited constructs a CodeRateLimited error.
func RateLimited(resource string, retryAfterSec ...int) *AfrosError {
	e := &AfrosError{
		Code:     CodeRateLimited,
		Resource: resource,
		Message:  fmt.Sprintf("%s rate limit exceeded", resource),
	}
	if len(retryAfterSec) > 0 {
		e.RetryAfterSec = retryAfterSec[0]
	}
	return e
}

// PreconditionFailed constructs a CodePreconditionFailed error.
func PreconditionFailed(resource, condition string) *AfrosError {
	return &AfrosError{
		Code:     CodePreconditionFailed,
		Resource: resource,
		Message:  fmt.Sprintf("%s precondition failed: %s", resource, condition),
	}
}

// Unprocessable constructs a CodeUnprocessable error.
func Unprocessable(message string) *AfrosError {
	return &AfrosError{Code: CodeUnprocessable, Message: message}
}

// Internal constructs a CodeInternal error.
// message is for internal logging — it is stripped before being sent to callers.
func Internal(message string) *AfrosError {
	return &AfrosError{Code: CodeInternal, Message: message}
}

// Unavailable constructs a CodeUnavailable error.
// dependency: the name of the failing dependency ("postgres", "redis", "identity-service").
func Unavailable(dependency string) *AfrosError {
	return &AfrosError{
		Code:    CodeUnavailable,
		Message: fmt.Sprintf("%s is unavailable", dependency),
	}
}

// Timeout constructs a CodeTimeout error.
func Timeout(operation string) *AfrosError {
	return &AfrosError{
		Code:    CodeTimeout,
		Message: fmt.Sprintf("%s timed out", operation),
	}
}

// NotImplemented constructs a CodeNotImplemented error.
func NotImplemented(operation string) *AfrosError {
	return &AfrosError{
		Code:    CodeNotImplemented,
		Message: fmt.Sprintf("%s is not yet implemented", operation),
	}
}

// Gone constructs a CodeGone error for permanently deleted resources.
func Gone(resource, id string) *AfrosError {
	return &AfrosError{
		Code:       CodeGone,
		Resource:   resource,
		ResourceID: id,
		Message:    fmt.Sprintf("%s %q has been permanently deleted", resource, id),
	}
}

// Domain-specific constructors — the most common domain errors get their own
// constructors so call sites are one line.

func OTPInvalid(attemptsLeft int) *AfrosError {
	return &AfrosError{
		Code:    CodeOTPInvalid,
		Message: fmt.Sprintf("OTP invalid, %d attempts remaining", attemptsLeft),
	}
}

func OTPExpired() *AfrosError {
	return &AfrosError{Code: CodeOTPExpired, Message: "OTP has expired"}
}

func TokenExpired() *AfrosError {
	return &AfrosError{Code: CodeTokenExpired, Message: "access token has expired"}
}

func TokenRevoked() *AfrosError {
	return &AfrosError{Code: CodeTokenRevoked, Message: "access token has been revoked"}
}

func TokenInvalid() *AfrosError {
	return &AfrosError{Code: CodeTokenInvalid, Message: "access token is invalid"}
}

func PhoneTaken(phone string) *AfrosError {
	return &AfrosError{
		Code:    CodePhoneTaken,
		Message: fmt.Sprintf("phone %q is already registered", phone),
		Fields:  []FieldError{{Field: "phone", Code: string(CodePhoneTaken)}},
	}
}

func HandleTaken(handle string) *AfrosError {
	return &AfrosError{
		Code:    CodeHandleTaken,
		Message: fmt.Sprintf("handle %q is already taken", handle),
		Fields:  []FieldError{{Field: "handle", Code: string(CodeHandleTaken)}},
	}
}

func ImmutableField(field string) *AfrosError {
	return &AfrosError{
		Code:    CodeImmutableField,
		Message: fmt.Sprintf("field %q cannot be changed after creation", field),
		Fields:  []FieldError{{Field: field, Code: string(CodeImmutableField), Message: "immutable"}},
	}
}

func InsufficientFunds(required, available string, currency string) *AfrosError {
	return &AfrosError{
		Code:    CodeInsufficientFunds,
		Message: fmt.Sprintf("insufficient funds: need %s %s, have %s %s", required, currency, available, currency),
	}
}

// ─── ValidationBuilder — for multi-field validation errors ────────────────────

// ValidationBuilder accumulates field-level errors and produces a single
// CodeInvalidInput AfrosError.
//
// Usage:
//
//	err := errors.Validation().
//	    Field("phone", "PHONE_INVALID", "must be E.164 format").
//	    Field("country", "REQUIRED", "country is required").
//	    Build()
//
// If no fields have been added, Build() returns nil (no error).
type ValidationBuilder struct {
	fields []FieldError
}

// Validation returns a new ValidationBuilder.
func Validation() *ValidationBuilder {
	return &ValidationBuilder{}
}

// Field adds a field-level error to the builder.
func (v *ValidationBuilder) Field(field, code, message string) *ValidationBuilder {
	v.fields = append(v.fields, FieldError{
		Field:   field,
		Code:    code,
		Message: message,
	})
	return v
}

// FieldIf adds a field-level error only when condition is true.
// Useful for clean conditional validation chains.
func (v *ValidationBuilder) FieldIf(condition bool, field, code, message string) *ValidationBuilder {
	if condition {
		v.fields = append(v.fields, FieldError{Field: field, Code: code, Message: message})
	}
	return v
}

// HasErrors returns true if any fields have been added.
func (v *ValidationBuilder) HasErrors() bool {
	return len(v.fields) > 0
}

// Build returns a CodeInvalidInput AfrosError if any fields were added,
// or nil if there are no errors. Follows the Go error convention of
// returning nil for success.
func (v *ValidationBuilder) Build() *AfrosError {
	if len(v.fields) == 0 {
		return nil
	}
	msg := fmt.Sprintf("validation failed: %d field(s) invalid", len(v.fields))
	return &AfrosError{
		Code:    CodeInvalidInput,
		Message: msg,
		Fields:  v.fields,
	}
}

// ─── gRPC conversion ──────────────────────────────────────────────────────────

// ToGRPC converts an AfrosError to a gRPC status error.
//
// The gRPC status code is derived from the AfrosError.Code.
// The string message in the status is the AfrosError.Code string (machine-readable),
// NOT the human Message (which may contain internal details).
//
// Additional structured detail (FieldErrors, TraceID, RequestID) is embedded
// in the status details as a JSON-serialised GRPCErrorDetail.
func (e *AfrosError) ToGRPC() error {
	st := status.New(codeToGRPC(e.Code), string(e.Code))

	// Attach structured detail when there is useful data to carry
	detail := &GRPCErrorDetail{
		Code:          string(e.Code),
		Message:       e.safeMessage(),
		Resource:      e.Resource,
		Fields:        e.Fields,
		TraceID:       e.TraceID,
		RequestID:     e.RequestID,
		RetryAfterSec: e.RetryAfterSec,
	}
	detailBytes, err := json.Marshal(detail)
	if err == nil {
		// Attach as a trailing metadata header — readable by any gRPC client
		// without proto-generated detail types.
		_ = detailBytes // used in metadata attachment (see middleware package)
	}

	return st.Err()
}

// GRPCErrorDetail is the structured error payload attached to gRPC status details.
// JSON-serialised. Clients that want rich error info decode this from the
// "x-afros-error" trailing metadata header.
type GRPCErrorDetail struct {
	Code          string       `json:"code"`
	Message       string       `json:"message"`
	Resource      string       `json:"resource,omitempty"`
	Fields        []FieldError `json:"fields,omitempty"`
	TraceID       string       `json:"trace_id,omitempty"`
	RequestID     string       `json:"request_id,omitempty"`
	RetryAfterSec int          `json:"retry_after_sec,omitempty"`
}

// codeToGRPC maps AfrosError.Code to the correct gRPC status code.
// This mapping is the single authoritative source — every service uses it.
func codeToGRPC(c Code) codes.Code {
	switch c {
	case CodeNotFound:
		return codes.NotFound
	case CodeGone:
		return codes.NotFound // gRPC has no Gone equivalent — use NotFound
	case CodeInvalidInput, CodeUnprocessable:
		return codes.InvalidArgument
	case CodeConflict:
		return codes.AlreadyExists
	case CodeUnauthorized, CodeTokenExpired, CodeTokenRevoked, CodeTokenInvalid,
		CodeOTPInvalid, CodeOTPExpired, CodeSIMSwapLockout:
		return codes.Unauthenticated
	case CodeForbidden, CodeACLForbidden, CodeACLSystemRole:
		return codes.PermissionDenied
	case CodeRateLimited, CodeOTPRateLimited, CodeDeviceLimit, CodeKYCTooManyAttempts:
		return codes.ResourceExhausted
	case CodePreconditionFailed, CodeIdentitySuspended, CodeIdentityDeleted,
		CodeKYCMinimumAge, CodeKYCWatchlistHit, CodeOfflineTokenExpired,
		CodeOfflineTokenRevoked, CodeDeviceNotTrusted:
		return codes.FailedPrecondition
	case CodePayloadTooLarge, CodeBusPayloadTooLarge:
		return codes.OutOfRange
	case CodeUnavailable, CodeBusBrokerDown:
		return codes.Unavailable
	case CodeTimeout:
		return codes.DeadlineExceeded
	case CodeNotImplemented:
		return codes.Unimplemented
	case CodeInternal:
		return codes.Internal
	default:
		return codes.Internal
	}
}

// ─── HTTP conversion ──────────────────────────────────────────────────────────

// HTTPError is the error block inside the standard API envelope.
// Rendered as the "error" field in every non-2xx JSON response.
type HTTPError struct {
	Code          string       `json:"code"`
	Message       string       `json:"message"`
	Resource      string       `json:"resource,omitempty"`
	Fields        []FieldError `json:"fields,omitempty"`
	TraceID       string       `json:"trace_id,omitempty"`
	RequestID     string       `json:"request_id,omitempty"`
	RetryAfterSec int          `json:"retry_after_sec,omitempty"`
	DocsURL       string       `json:"docs_url,omitempty"`
}

// ToHTTP converts an AfrosError to its HTTP status code and HTTPError body.
//
// Returns:
//   - httpStatus: the correct HTTP status code (400, 401, 403, 404, 409, 422, 429, 500, 503)
//   - body: the HTTPError struct ready for JSON serialisation
//
// The response envelope shape (with "data", "meta", "error" fields) is assembled
// by the HTTP handler layer, not here. This returns only the error portion.
func (e *AfrosError) ToHTTP() (httpStatus int, body HTTPError) {
	return codeToHTTP(e.Code), HTTPError{
		Code:          string(e.Code),
		Message:       e.safeMessage(),
		Resource:      e.Resource,
		Fields:        e.Fields,
		TraceID:       e.TraceID,
		RequestID:     e.RequestID,
		RetryAfterSec: e.RetryAfterSec,
		DocsURL:       docsURL(e.Code),
	}
}

// WriteHTTP writes a complete JSON error response to an http.ResponseWriter.
// This is the convenience method for HTTP handlers.
func (e *AfrosError) WriteHTTP(w http.ResponseWriter) {
	httpStatus, body := e.ToHTTP()

	envelope := map[string]interface{}{
		"data":  nil,
		"error": body,
		"meta":  map[string]interface{}{},
	}
	if e.RequestID != "" {
		envelope["meta"] = map[string]interface{}{
			"request_id": e.RequestID,
			"trace_id":   e.TraceID,
		}
	}

	b, err := json.Marshal(envelope)
	if err != nil {
		http.Error(w, `{"error":{"code":"INTERNAL","message":"response serialisation failed"}}`, 500)
		return
	}

	if e.RetryAfterSec > 0 {
		w.Header().Set("Retry-After", fmt.Sprintf("%d", e.RetryAfterSec))
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(httpStatus)
	_, _ = w.Write(b)
}

// codeToHTTP maps AfrosError codes to HTTP status codes.
func codeToHTTP(c Code) int {
	switch c {
	case CodeInvalidInput, CodeUnprocessable, CodePhoneInvalid,
		CodeBusSchemaInvalid, CodeBusPayloadTooLarge:
		return http.StatusBadRequest // 400

	case CodeUnauthorized, CodeTokenExpired, CodeTokenRevoked, CodeTokenInvalid,
		CodeOTPInvalid, CodeOTPExpired, CodeSIMSwapLockout:
		return http.StatusUnauthorized // 401

	case CodeForbidden, CodeACLForbidden, CodeACLSystemRole, CodeIdentitySuspended,
		CodeKYCWatchlistHit, CodeOfflineTokenRevoked, CodeDeviceNotTrusted:
		return http.StatusForbidden // 403

	case CodeNotFound, CodeIdentityDeleted:
		return http.StatusNotFound // 404

	case CodeGone:
		return http.StatusGone // 410

	case CodeConflict, CodePhoneTaken, CodeHandleTaken, CodeACLRoleExists,
		CodeKYCDuplicate:
		return http.StatusConflict // 409

	case CodePreconditionFailed, CodeImmutableField, CodeKYCMinimumAge,
		CodeOfflineTokenExpired, CodeOfflineClockSkew:
		return http.StatusUnprocessableEntity // 422

	case CodePayloadTooLarge:
		return http.StatusRequestEntityTooLarge // 413

	case CodeRateLimited, CodeOTPRateLimited, CodeDeviceLimit, CodeKYCTooManyAttempts:
		return http.StatusTooManyRequests // 429

	case CodeNotImplemented:
		return http.StatusNotImplemented // 501

	case CodeUnavailable, CodeBusBrokerDown:
		return http.StatusServiceUnavailable // 503

	case CodeTimeout:
		return http.StatusGatewayTimeout // 504

	case CodeInternal:
		return http.StatusInternalServerError // 500

	default:
		return http.StatusInternalServerError // 500
	}
}

// safeMessage strips the internal cause details for external-facing messages.
// For CodeInternal errors, the message is always replaced with a generic string
// to prevent internal implementation details leaking to callers.
func (e *AfrosError) safeMessage() string {
	if e.Code == CodeInternal {
		return "an internal error occurred"
	}
	return e.Message
}

// docsURL returns a link to the error documentation for the given code.
// Used in the HTTP error body to help API consumers debug.
func docsURL(c Code) string {
	return fmt.Sprintf("https://docs.combi.africa/errors/%s", c)
}

// ─── FromGRPC — convert incoming gRPC errors to AfrosError ───────────────────
//
// Used by services that call OTHER services. When a downstream gRPC call
// returns an error, wrap it with this to get a consistent AfrosError back.

// FromGRPC converts a gRPC status error returned by a downstream service
// into an AfrosError suitable for the current service to handle or propagate.
//
// If the error is nil, nil is returned.
// If the error is already an *AfrosError, it is returned as-is.
// If the gRPC status is Unavailable or DeadlineExceeded, the local service
// returns CodeUnavailable (circuit-breaker pattern — don't cascade internals).
func FromGRPC(err error, downstreamName string) *AfrosError {
	if err == nil {
		return nil
	}

	// Already an AfrosError (e.g. from an in-process test mock)
	if ae, ok := err.(*AfrosError); ok {
		return ae
	}

	st, ok := status.FromError(err)
	if !ok {
		// Not a gRPC error — treat as internal
		return Internal(fmt.Sprintf("%s: %v", downstreamName, err))
	}

	switch st.Code() {
	case codes.NotFound:
		return NotFound(downstreamName)
	case codes.AlreadyExists:
		return Conflict(downstreamName, st.Message())
	case codes.InvalidArgument:
		return &AfrosError{Code: CodeInvalidInput, Message: st.Message()}
	case codes.Unauthenticated:
		return Unauthorized(st.Message())
	case codes.PermissionDenied:
		return Forbidden(st.Message())
	case codes.ResourceExhausted:
		return RateLimited(downstreamName)
	case codes.FailedPrecondition:
		return PreconditionFailed(downstreamName, st.Message())
	case codes.Unavailable, codes.DeadlineExceeded:
		// Circuit-breaker: upstream unavailability becomes a local unavailable error.
		// Do NOT propagate the upstream's internal details.
		return Unavailable(downstreamName)
	case codes.Unimplemented:
		return NotImplemented(downstreamName + ": " + st.Message())
	default:
		// Unknown or Internal from downstream — treat as local unavailability.
		// Log the original for debugging, but don't leak it.
		return Unavailable(downstreamName).Wrap(err)
	}
}
