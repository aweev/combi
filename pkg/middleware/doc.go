// Package middleware provides the shared request-context propagation and
// server/client interceptors that every Africa OS service uses.
//
// The package implements the SHARED RULES from the foundation services spec:
//
//   - tenant_id comes from verified JWT claims only — never from headers or
//     request bodies (SeeContext, WithContext propagation below)
//   - every log line carries request_id, tenant_id, identity_id, duration_ms
//   - inter-service gRPC calls run with a 5s deadline and circuit-break on
//     codes.Unavailable so failures do not cascade
//   - panics are recovered and converted to INTERNAL errors without leaking
//     stack traces to callers
//
// The kit is composed of three layers:
//
//   - context.go — typed keys + accessors for RequestID, TenantID, IdentityID
//     and extraction from gRPC metadata, HTTP headers, and JWT claims
//   - server.go  — gRPC server interceptors (recovery, logging, request-id,
//     error normalization) and HTTP middleware equivalents
//   - client.go  — gRPC client interceptors (deadline, retry with circuit
//     breaking, metadata forwarding) used for cross-process calls
package middleware
