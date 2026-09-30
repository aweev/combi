package ctxlocale

import (
	"context"

	"github.com/combi/pkg/i18n/locale"
)

// contextKey is unexported to avoid collisions with other packages.
type contextKey struct{}

var localeCtxKey = contextKey{}

// WithLocale returns a new context with the locale stored.
func WithLocale(ctx context.Context, l locale.Locale) context.Context {
	return context.WithValue(ctx, localeCtxKey, l)
}

// LocaleFromContext extracts the locale from a context.
// Returns locale.EnNG if none is set.
func LocaleFromContext(ctx context.Context) locale.Locale {
	if l, ok := ctx.Value(localeCtxKey).(locale.Locale); ok {
		return l
	}
	return locale.EnNG
}
