package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSyncLocalesAddsMissingKeysPreservesExisting(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "en-NG.json")
	if err := os.WriteFile(path, []byte(`{"hello":"Hello","existing":"value"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	keys := []string{"hello", "new.key"}
	if _, err := syncLocales(keys, tmp); err != nil {
		t.Fatal(err)
	}

	catalog, err := loadJSONFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if catalog["hello"] != "Hello" {
		t.Fatalf("expected hello to be preserved, got %v", catalog["hello"])
	}
	if catalog["new.key"] != "" {
		t.Fatalf("expected new.key to be added as empty string, got %v", catalog["new.key"])
	}
	if catalog["existing"] != "value" {
		t.Fatalf("expected existing key to remain, got %v", catalog["existing"])
	}
}
