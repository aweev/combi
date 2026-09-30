package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateLocalesFailsOnMissingTranslation(t *testing.T) {
	tmp := t.TempDir()
	writeJSON := func(name, content string) {
		path := filepath.Join(tmp, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	writeJSON("en-NG.json", `{"hello":"Hello","welcome":"Welcome"}`)
	writeJSON("pcm-NG.json", `{"hello":"Hello"}`)
	writeJSON("fr-CM.json", `{"hello":"Bonjour","welcome":""}`)
	writeJSON("sw-KE.json", `{"hello":"Hello","welcome":"Karibu"}`)

	err := validateLocales(tmp)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}

	msg := err.Error()
	if !strings.Contains(msg, "pcm-NG") || !strings.Contains(msg, "fr-CM") {
		t.Fatalf("unexpected validation message: %s", msg)
	}
}
