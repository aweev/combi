package catalog_test

import (
	"context"
	"strings"
	"testing"

	"github.com/combi/pkg/i18n/catalog"
	"github.com/combi/pkg/i18n/ctxlocale"
	"github.com/combi/pkg/i18n/locale"
)

func newTestRegistry(t *testing.T) *catalog.Registry {
	t.Helper()
	reg := catalog.New()

	enNG := []byte(`{
		"app.tagline": "Everything Casts. Nothing is safe.",
		"burnbook.locked.headline": {
			"one": "{{.N}} verdict has been Cast against you.",
			"other": "{{.N}} verdicts have been Cast against you."
		},
		"onboarding.otp.resend": "Nothing arrived? Resend in {{.Seconds}}s."
	}`)
	if err := reg.LoadJSON(locale.EnNG, enNG); err != nil {
		t.Fatalf("LoadJSON(en-NG): %v", err)
	}

	pcmNG := []byte(`{
		"app.tagline": "Everything dey Cast. Nobody dey safe.",
		"burnbook.locked.headline": "Your verdict don Cast against you."
	}`)
	if err := reg.LoadJSON(locale.PcmNG, pcmNG); err != nil {
		t.Fatalf("LoadJSON(pcm-NG): %v", err)
	}

	return reg
}

func TestCatalog_SimpleTranslation(t *testing.T) {
	reg := newTestRegistry(t)

	got := reg.T("app.tagline", locale.EnNG, nil, 0)
	want := "Everything Casts. Nothing is safe."
	if got != want {
		t.Errorf("T(app.tagline, en-NG) = %q, want %q", got, want)
	}

	got = reg.T("app.tagline", locale.PcmNG, nil, 0)
	want = "Everything dey Cast. Nobody dey safe."
	if got != want {
		t.Errorf("T(app.tagline, pcm-NG) = %q, want %q", got, want)
	}
}

func TestCatalog_PluralTranslation(t *testing.T) {
	reg := newTestRegistry(t)

	// Singular
	got := reg.T("burnbook.locked.headline", locale.EnNG, map[string]any{"N": 1}, 1)
	want := "1 verdict has been Cast against you."
	if got != want {
		t.Errorf("plural(1) = %q, want %q", got, want)
	}

	// Plural
	got = reg.T("burnbook.locked.headline", locale.EnNG, map[string]any{"N": 21}, 21)
	want = "21 verdicts have been Cast against you."
	if got != want {
		t.Errorf("plural(21) = %q, want %q", got, want)
	}
}

func TestCatalog_Interpolation(t *testing.T) {
	reg := newTestRegistry(t)

	got := reg.T("onboarding.otp.resend", locale.EnNG, map[string]any{"Seconds": 30}, 0)
	want := "Nothing arrived? Resend in 30s."
	if got != want {
		t.Errorf("interpolation = %q, want %q", got, want)
	}
}

func TestCatalog_FallbackChain(t *testing.T) {
	reg := newTestRegistry(t)

	// pcm-NG-x-slang doesn't have "app.tagline" but pcm-NG does (via fallback)
	got := reg.T("app.tagline", locale.PcmNGSlang, nil, 0)
	if got == "app.tagline" {
		t.Error("fallback chain should resolve app.tagline for pcm-NG-x-slang via pcm-NG")
	}
}

func TestCatalog_MissingKeyReturnsKey(t *testing.T) {
	reg := newTestRegistry(t)

	got := reg.T("nonexistent.key", locale.EnNG, nil, 0)
	if got != "nonexistent.key" {
		t.Errorf("missing key should return key itself, got %q", got)
	}
}

func TestCatalog_MissingKeys(t *testing.T) {
	reg := newTestRegistry(t)

	missing := reg.MissingKeys(locale.EnNG, locale.PcmNG)
	found := false
	for _, k := range missing {
		if k == "onboarding.otp.resend" {
			found = true
			break
		}
	}
	if !found {
		t.Error("onboarding.otp.resend should be missing from pcm-NG catalog")
	}
}

func TestCatalog_TCtxUsesContextLocale(t *testing.T) {
	reg := newTestRegistry(t)
	ctx := ctxlocale.WithLocale(context.Background(), locale.PcmNG)

	got := reg.TCtx(ctx, "app.tagline", nil, 0)
	if got != "Everything dey Cast. Nobody dey safe." {
		t.Errorf("TCtx = %q, want %q", got, "Everything dey Cast. Nobody dey safe.")
	}
}

func TestCatalog_ExportJSONIncludesEntries(t *testing.T) {
	reg := newTestRegistry(t)
	data, err := reg.ExportJSON(locale.EnNG)
	if err != nil {
		t.Fatalf("ExportJSON(en-NG) returned error: %v", err)
	}
	if !strings.Contains(string(data), "app.tagline") {
		t.Error("ExportJSON(en-NG) should include app.tagline")
	}
}

func TestCatalog_LoadJSONPluralRequiresOther(t *testing.T) {
	reg := catalog.New()
	bad := []byte(`{"count": {"one": "One item"}}`)
	if err := reg.LoadJSON(locale.EnNG, bad); err == nil {
		t.Fatal("expected error for plural entry missing other")
	}
}
