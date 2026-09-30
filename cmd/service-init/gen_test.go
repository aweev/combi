package main

import (
	"bytes"
	"go/format"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
)

func TestPackageName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"payment-links", "paymentlinks"},
		{"fx", "fx"},
		{"audit-log", "auditlog"},
		{"rbac", "rbac"},
	}
	for _, tc := range cases {
		if got := packageName(tc.in); got != tc.want {
			t.Errorf("packageName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestHumanize(t *testing.T) {
	if got := humanize("payment-links"); got != "Payment Links" {
		t.Errorf("humanize = %q", got)
	}
}

func TestRenderGoIsGoFormatted(t *testing.T) {
	c := &genContext{Package: "paymentlinks", ModulePath: "github.com/combi/paymentlinks", Short: "payment-links", Human: "Payment Links"}
	for _, tmpl := range []struct {
		name string
		t    *template.Template
	}{
		{"fx.go", tmplFX},
		{"service.go", tmplService},
		{"service_test.go", tmplTest},
	} {
		src, err := renderGo(tmpl.t, c)
		if err != nil {
			t.Fatalf("renderGo %s: %v", tmpl.name, err)
		}
		// renderGo gofmts via format.Source — formatting the output again
		// must be a no-op (idempotent gofmt).
		if again, err := format.Source(src); err != nil {
			t.Errorf("generated %s not gofmt-valid: %v", tmpl.name, err)
		} else if !bytes.Equal(again, src) {
			t.Errorf("generated %s not gofmt-stable", tmpl.name)
		}
	}
}

func TestScaffoldRefusesExistingDir(t *testing.T) {
	dir := t.TempDir()
	c := &genContext{
		Root:       dir,
		TierDir:    "platform",
		DirName:    "x-service",
		OutDir:     filepath.Join(dir, "services", "platform", "x-service"),
		Package:    "x",
		ModulePath: "github.com/combi/x",
		Short:      "x",
		Human:      "X",
	}
	if err := scaffold(c); err != nil {
		t.Fatalf("first scaffold: %v", err)
	}
	if err := scaffold(c); err == nil {
		t.Fatal("second scaffold must refuse to clobber")
	}
}

func TestPkgReplaceDepth(t *testing.T) {
	// platform tier: services/platform/x-service is 3 levels below root.
	c := &genContext{TierDir: "platform", Root: "/r"}
	depth := 3 + strings.Count(c.TierDir, "/")
	if got, want := strings.Repeat("../", depth)+"pkg", "../../../pkg"; got != want {
		t.Errorf("platform replace = %q, want %q", got, want)
	}
	// ngo tier: services/apps/ngo/x-service is 4 levels below root.
	c2 := &genContext{TierDir: "apps/ngo"}
	depth2 := 3 + strings.Count(c2.TierDir, "/")
	if got, want := strings.Repeat("../", depth2)+"pkg", "../../../../pkg"; got != want {
		t.Errorf("ngo replace = %q, want %q", got, want)
	}
}
