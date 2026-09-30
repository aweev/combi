// Package middleware provides gRPC and HTTP middleware for locale propagation.
//
// Every inbound request carries a locale. This package extracts it from:
//   - gRPC metadata header: "x-locale"
//   - HTTP header: "Accept-Language" (with quality weighting)
//   - HTTP header: "X-Locale" (explicit override, takes priority)
//   - JWT claim: "locale" (highest priority — user's saved preference)
//
// The resolved locale is stored in the request context and accessible via
// middleware.LocaleFromContext(ctx).
package middleware

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/combi/pkg/i18n/ctxlocale"
	"github.com/combi/pkg/i18n/direction"
	"github.com/combi/pkg/i18n/locale"
)

// VerifiedClaimsKey is the context key where auth middleware stores verified JWT claims.
// Locale middleware reads from this key rather than re-parsing the raw token.
type VerifiedClaimsKey struct{}

// DefaultLocale is the fallback if no locale can be determined.
// Nigerian English is the primary market default.
const DefaultLocale = locale.EnNG

// WithLocale returns a new context with the locale stored.
func WithLocale(ctx context.Context, l locale.Locale) context.Context {
	return ctxlocale.WithLocale(ctx, l)
}

// LocaleFromContext extracts the locale from a context.
// Returns DefaultLocale if none is set.
func LocaleFromContext(ctx context.Context) locale.Locale {
	if l := ctxlocale.LocaleFromContext(ctx); l != "" {
		return l
	}
	return DefaultLocale
}

// ── gRPC interceptors ─────────────────────────────────────────────────────────

// UnaryServerInterceptor is a gRPC server interceptor that extracts the locale
// from incoming metadata and injects it into the request context.
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		_ *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		ctx = withLocaleFromGRPC(ctx)
		return handler(ctx, req)
	}
}

// StreamServerInterceptor is the streaming equivalent.
func StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		_ *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ctx := withLocaleFromGRPC(ss.Context())
		wrapped := &wrappedStream{ServerStream: ss, ctx: ctx}
		return handler(srv, wrapped)
	}
}

// UnaryClientInterceptor propagates the locale from the client context
// into outgoing gRPC metadata.
func UnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		ctx = injectLocaleToGRPC(ctx)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// StreamClientInterceptor propagates the locale from the client context
// into outgoing streaming gRPC metadata.
func StreamClientInterceptor() grpc.StreamClientInterceptor {
	return func(
		ctx context.Context,
		desc *grpc.StreamDesc,
		cc *grpc.ClientConn,
		method string,
		streamer grpc.Streamer,
		opts ...grpc.CallOption,
	) (grpc.ClientStream, error) {
		ctx = injectLocaleToGRPC(ctx)
		return streamer(ctx, desc, cc, method, opts...)
	}
}

// withLocaleFromGRPC reads "x-locale" from incoming gRPC metadata.
func withLocaleFromGRPC(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}
	values := md.Get("x-locale")
	if len(values) == 0 {
		return ctx
	}
	if len(values) > 1 {
		log.Printf("middleware: multiple x-locale values received; using first: %v", values)
	}
	l := locale.Locale(values[0])
	if _, ok := locale.Lookup(l); !ok {
		// Unknown locale — try Accept-Language style negotiation
		l = direction.LocaleForHTTP(values[0], DefaultLocale)
	}
	return WithLocale(ctx, l)
}

// injectLocaleToGRPC writes the locale from context into outgoing metadata.
func injectLocaleToGRPC(ctx context.Context) context.Context {
	md, ok := metadata.FromOutgoingContext(ctx)
	if ok && len(md.Get("x-locale")) > 0 {
		return ctx // caller already set the locale; respect it
	}

	l := LocaleFromContext(ctx)
	if !ok {
		md = metadata.New(nil)
	} else {
		md = md.Copy()
	}
	md.Set("x-locale", string(l))
	return metadata.NewOutgoingContext(ctx, md)
}

// ── HTTP middleware ────────────────────────────────────────────────────────────

// HTTPMiddleware is a standard net/http middleware that resolves the locale
// from HTTP headers and injects it into the request context.
func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		l := resolveHTTPLocale(r)
		ctx := WithLocale(r.Context(), l)

		// Set Vary header so CDNs cache per-locale responses correctly
		w.Header().Add("Vary", "Accept-Language, X-Locale")

		// Emit Content-Language so clients know what locale was selected
		w.Header().Set("Content-Language", string(l))

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ── Helpers for handler code ──────────────────────────────────────────────────

// IsRTL reports whether the request's locale uses RTL text direction.
func IsRTL(ctx context.Context) bool {
	return locale.IsRTL(LocaleFromContext(ctx))
}

// HTMLDir returns the HTML dir attribute value for the request's locale.
func HTMLDir(ctx context.Context) string {
	return direction.HTMLAttr(LocaleFromContext(ctx))
}

// LocaleUpdateHandler returns an http.HandlerFunc that validates a locale
// change request and stores it via the provided setter function.
func LocaleUpdateHandler(setter func(ctx context.Context, l locale.Locale) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body := struct {
			Locale string `json:"locale"`
		}{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		l := locale.Locale(body.Locale)
		if _, ok := locale.Lookup(l); !ok {
			http.Error(w, "unsupported locale", http.StatusUnprocessableEntity)
			return
		}
		if err := setter(r.Context(), l); err != nil {
			http.Error(w, "failed to save locale", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"locale": string(l)})
	}
}

// ── wrappedStream for streaming interceptor ───────────────────────────────────

type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context { return w.ctx }

// resolveHTTPLocale extracts and negotiates the best locale for an HTTP request.
func resolveHTTPLocale(r *http.Request) locale.Locale {
	// 1. Verified JWT claims set by upstream auth middleware.
	if claims, ok := r.Context().Value(VerifiedClaimsKey{}).(map[string]any); ok {
		if lv, ok := claims["locale"].(string); ok {
			l := locale.Locale(lv)
			if _, ok := locale.Lookup(l); ok {
				return l
			}
		}
	}

	// 2. JWT claim fallback for raw Authorization header.
	if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
		if l, ok := localeFromJWT(authHeader[7:]); ok {
			log.Printf("middleware: locale extracted from unverified JWT token")
			return l
		}
	}

	// 3. Explicit override header
	if explicit := r.Header.Get("X-Locale"); explicit != "" {
		l := locale.Locale(explicit)
		if _, ok := locale.Lookup(l); ok {
			return l
		}
	}

	// 4. Accept-Language negotiation
	if al := r.Header.Get("Accept-Language"); al != "" {
		return direction.LocaleForHTTP(al, DefaultLocale)
	}

	return DefaultLocale
}

type jwtClaims struct {
	Locale string `json:"locale"`
}

func localeFromJWT(token string) (locale.Locale, bool) {
	parts := strings.SplitN(token, ".", 3)
	if len(parts) != 3 {
		return "", false
	}

	// Pad the base64url segment to a multiple of 4.
	payload := parts[1]
	if pad := len(payload) % 4; pad != 0 {
		payload += strings.Repeat("=", 4-pad)
	}

	decoded, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		return "", false
	}

	var claims jwtClaims
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return "", false
	}

	if claims.Locale == "" {
		return "", false
	}

	l := locale.Locale(claims.Locale)
	if _, ok := locale.Lookup(l); !ok {
		return "", false
	}

	return l, true
}
