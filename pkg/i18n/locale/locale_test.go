package locale_test

import (
	"strings"
	"testing"

	"github.com/combi/pkg/i18n/locale"
)

func TestLookupEthiopianLocales(t *testing.T) {
	for _, l := range []locale.Locale{locale.AmET, locale.TiET, locale.OmET} {
		if _, ok := locale.Lookup(l); !ok {
			t.Fatalf("expected locale %s to be registered", l)
		}
	}
}

func TestSoftBreakEthiopicText(t *testing.T) {
	input := "አማርኛቋንቋ"
	output := locale.SoftBreakEthiopicText(input)
	if output == input {
		t.Fatal("expected Ethiopic text to receive soft break hints")
	}
	if !strings.ContainsRune(output, '\u200B') {
		t.Fatal("expected output to contain a zero-width space")
	}
}

func TestLocale_AllRegisteredLocalesHaveValidMeta(t *testing.T) {
	for l, meta := range locale.All {
		if meta.Tag != l {
			t.Errorf("locale %q: Meta.Tag mismatch (%q)", l, meta.Tag)
		}
		if meta.Script == "" {
			t.Errorf("locale %q: missing Script", l)
		}
		if meta.Direction == "" {
			t.Errorf("locale %q: missing Direction", l)
		}
		if meta.PluralFamily == "" {
			t.Errorf("locale %q: missing PluralFamily", l)
		}
		if len(meta.FallbackChain) == 0 {
			t.Errorf("locale %q: empty FallbackChain", l)
		}
		if meta.NativeName == "" {
			t.Errorf("locale %q: missing NativeName", l)
		}
	}
}

func TestLocale_RTLMirrorSymmetry(t *testing.T) {
	// If locale A has RTLMirror B, then B should have RTLMirror A.
	for l, meta := range locale.All {
		if meta.RTLMirror == "" {
			continue
		}
		mirror, ok := locale.Lookup(meta.RTLMirror)
		if !ok {
			t.Errorf("locale %q: RTLMirror %q is not registered", l, meta.RTLMirror)
			continue
		}
		if mirror.RTLMirror != l {
			t.Errorf("locale %q: RTLMirror %q does not point back", l, meta.RTLMirror)
		}
	}
}
