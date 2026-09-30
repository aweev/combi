package main

// gen.go — name normalisation, root resolution, scaffolding, verification.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// tierDirs maps tiers to their directory under services/.
var tierDirs = map[string]string{
	"platform": "platform",
	"ngo":      "apps/ngo",
	"cast":     "apps/cast",
}

var nameRE = regexp.MustCompile(`^[a-z][a-z0-9-]*[a-z0-9]$`)

type genContext struct {
	Tier       string // platform | ngo | cast
	TierDir    string // services/<tier> path fragment
	Short      string // payment-links (fx module name, health probe key)
	DirName    string // payment-links-service
	Package    string // paymentlinks (Go package name)
	ModulePath string // github.com/combi/paymentlinks
	Human      string // Payment Links (doc strings)
	PkgReplace string // relative path from new module dir to pkg/
	Root       string // absolute combi/ root
	OutDir     string // absolute services/<tier>/<dir>
}

func run(nameRaw, tier, rootFlag string, noTidy, noWire bool) error {
	name := strings.ToLower(strings.TrimSpace(nameRaw))
	name = strings.TrimSuffix(name, "-service") // regenerate-friendly

	tierDir, ok := tierDirs[strings.ToLower(tier)]
	if !ok {
		return fmt.Errorf("unknown tier %q (want platform|ngo|cast)", tier)
	}
	if !nameRE.MatchString(name) {
		return fmt.Errorf("invalid service name %q: lowercase letters, digits and hyphens only", name)
	}

	root, err := resolveRoot(rootFlag)
	if err != nil {
		return err
	}

	ctx := &genContext{
		Tier:    strings.ToLower(tier),
		TierDir: tierDir,
		Short:   name,
		DirName: name + "-service",
		Root:    root,
	}
	ctx.Package = packageName(name)
	ctx.ModulePath = "github.com/combi/" + ctx.Package
	ctx.Human = humanize(name)
	ctx.OutDir = filepath.Join(root, "services", tierDir, ctx.DirName)
	// Depth of the module dir below root: services/platform/x-service is 3
	// levels (services, platform, module) → ../../../pkg; the apps/* tiers
	// add one more: services/apps/ngo/x-service → ../../../../pkg.
	depth := 3 + strings.Count(tierDir, "/")
	ctx.PkgReplace = strings.Repeat("../", depth) + "pkg"

	if err := scaffold(ctx); err != nil {
		return err
	}

	fmt.Printf("created %s\n", ctx.OutDir)
	fmt.Printf("  module %s (package %s), tier %s\n", ctx.ModulePath, ctx.Package, ctx.Tier)
	fmt.Printf("  health probe key %q\n", ctx.Short)

	if noTidy {
		fmt.Println("skipping go work use / tidy / build (per -no-tidy)")
		return nil
	}
	if !noWire {
		if err := goWorkUse(ctx.Root, ctx.OutDir); err != nil {
			return err
		}
	}
	return tidyAndBuild(ctx)
}

// resolveRoot finds the combi/ root (directory containing go.work):
// explicit flag → CWD walk-up.
func resolveRoot(rootFlag string) (string, error) {
	candidates := []string{}
	if rootFlag != "" {
		candidates = append(candidates, rootFlag)
	}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, wd, filepath.Dir(wd), filepath.Dir(filepath.Dir(wd)))
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		abs, err := filepath.Abs(c)
		if err != nil {
			continue
		}
		if _, err := os.Stat(filepath.Join(abs, "go.work")); err == nil {
			return abs, nil
		}
	}
	return "", fmt.Errorf("cannot locate repo root (no go.work found); pass -root")
}

// scaffold writes the generated files, refusing to clobber an existing dir.
func scaffold(c *genContext) error {
	if _, err := os.Stat(c.OutDir); err == nil {
		return fmt.Errorf("output directory already exists: %s", c.OutDir)
	}

	fxSrc, err := renderGo(tmplFX, c)
	if err != nil {
		return err
	}
	svcSrc, err := renderGo(tmplService, c)
	if err != nil {
		return err
	}
	testSrc, err := renderGo(tmplTest, c)
	if err != nil {
		return err
	}
	goModSrc, err := renderText(tmplGoMod, c)
	if err != nil {
		return err
	}
	readmeSrc, err := renderTextBackticked(tmplReadme, c)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(c.OutDir, 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	files := map[string][]byte{
		"fx.go":           fxSrc,
		"service.go":      svcSrc,
		"service_test.go": testSrc,
		"go.mod":          goModSrc,
		"README.md":       readmeSrc,
	}
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(c.OutDir, name), src, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}
	return nil
}

// goWorkUse runs 'go work use <dir>' from the repo root.
func goWorkUse(root, dir string) error {
	cmd := exec.Command("go", "work", "use", dir)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go work use: %w\n%s", err, out)
	}
	fmt.Printf("wired into go.work: %s\n", dir)
	return nil
}

// tidyAndBuild runs tidy + build + test inside the new module as a standalone
// verification (the replace directive keeps this working outside the
// workspace too).
func tidyAndBuild(c *genContext) error {
	for _, step := range []struct {
		args []string
		what string
	}{
		{[]string{"mod", "tidy"}, "go mod tidy"},
		{[]string{"build", "./..."}, "go build ./..."},
		{[]string{"test", "./..."}, "go test ./..."},
	} {
		cmd := exec.Command("go", step.args...)
		cmd.Dir = c.OutDir
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%s: %w\n%s", step.what, err, out)
		}
	}
	fmt.Println("verified: go mod tidy + go build + go test passed")
	return nil
}

// packageName converts kebab-case to a Go package name:
// payment-links → paymentlinks; fx-service → fx (matches existing convention).
func packageName(name string) string {
	var b strings.Builder
	for _, p := range strings.Split(name, "-") {
		if p == "service" {
			continue
		}
		b.WriteString(p)
	}
	return b.String()
}

// humanize converts kebab-case to Title Case for doc strings.
func humanize(name string) string {
	parts := strings.Split(name, "-")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}
