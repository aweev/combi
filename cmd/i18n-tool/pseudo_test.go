package main

import (
	"strings"
	"testing"
)

func TestPseudoTranslateValue(t *testing.T) {
	value := map[string]any{
		"simple": "Hello",
		"plural": map[string]any{"one": "One", "other": "Other"},
	}

	translated := pseudoTranslateValue(value).(map[string]any)
	if !strings.HasPrefix(translated["simple"].(string), "[!!") {
		t.Fatalf("expected pseudo translation wrapper, got %q", translated["simple"])
	}
	if !strings.ContainsAny(translated["simple"].(string), "àéîôûÀÉÎÔÛ") {
		t.Fatalf("expected accent replacement in %q", translated["simple"])
	}
	plural := translated["plural"].(map[string]any)
	if !strings.HasPrefix(plural["one"].(string), "[!!") {
		t.Fatalf("expected pseudo translation for plural form, got %q", plural["one"])
	}
}
