package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractKeys(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	if err := os.Mkdir(src, 0o755); err != nil {
		t.Fatal(err)
	}

	file := filepath.Join(src, "sample.go")
	if err := os.WriteFile(file, []byte(`package sample

func example() {
	T("hello.world")
	reg.TPlural("item.count", 3, nil)
}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	keys, err := extractKeys(src)
	if err != nil {
		t.Fatal(err)
	}

	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}
	if keys[0] != "hello.world" || keys[1] != "item.count" {
		t.Fatalf("unexpected keys: %v", keys)
	}
}
