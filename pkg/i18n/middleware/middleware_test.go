package middleware_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/combi/pkg/i18n/locale"
	"github.com/combi/pkg/i18n/middleware"
)

func TestLocaleUpdateHandler(t *testing.T) {
	var saved locale.Locale
	handler := middleware.LocaleUpdateHandler(func(ctx context.Context, l locale.Locale) error {
		saved = l
		return nil
	})

	body := map[string]string{"locale": string(locale.YoNG)}
	payload, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPatch, "/me/locale", bytes.NewReader(payload))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if saved != locale.YoNG {
		t.Fatalf("saved locale = %q, want %q", saved, locale.YoNG)
	}
}
