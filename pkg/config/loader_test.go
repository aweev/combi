package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/combi/pkg/config"
	"github.com/combi/pkg/i18n/locale"
)

func TestValidateAcceptsKnownGoodConfigs(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  *config.Config
	}{
		{"empty (test configs)", &config.Config{}},
		{"development", &config.Config{Env: config.EnvDevelopment}},
		{"production", &config.Config{Env: config.EnvProduction}},
		{"nigeria defaults", &config.Config{
			Env: config.EnvDevelopment,
			Localization: config.LocalizationConfig{
				DefaultLocale:   string(locale.EnNG),
				DefaultCurrency: "NGN",
			},
		}},
		{"explicit port", &config.Config{Server: config.ServerConfig{GRPCPort: 50051}}},
	} {
		if err := tc.cfg.Validate(); err != nil {
			t.Errorf("%s: Validate() = %v, want nil", tc.name, err)
		}
	}
}

func TestValidateRejectsBadConfigs(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  *config.Config
		want string
	}{
		{"bad env", &config.Config{Env: "prod"}, "env"},
		{"port too high", &config.Config{Server: config.ServerConfig{GRPCPort: 70000}}, "grpcPort"},
		{"port negative", &config.Config{Server: config.ServerConfig{GRPCPort: -1}}, "grpcPort"},
		{"unsupported locale", &config.Config{Localization: config.LocalizationConfig{DefaultLocale: "xx-XX"}}, "defaultLocale"},
		{"lowercase currency", &config.Config{Localization: config.LocalizationConfig{DefaultCurrency: "ngn"}}, "defaultCurrency"},
		{"2-letter currency", &config.Config{Localization: config.LocalizationConfig{DefaultCurrency: "NG"}}, "defaultCurrency"},
	} {
		err := tc.cfg.Validate()
		if err == nil {
			t.Errorf("%s: Validate() = nil, want error mentioning %q", tc.name, tc.want)
			continue
		}
		if !contains(err.Error(), tc.want) {
			t.Errorf("%s: Validate() = %v, want error mentioning %q", tc.name, err, tc.want)
		}
	}
}

func contains(haystack, needle string) bool {
	return len(needle) == 0 || (len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0)
}

func indexOf(h, n string) int {
	for i := 0; i+len(n) <= len(h); i++ {
		if h[i:i+len(n)] == n {
			return i
		}
	}
	return -1
}

// writeConfig writes a minimal config.yml into a temp dir for Load tests.
func writeConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.yml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return dir
}

func TestLoadValidatesFailFast(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		wantErr string
	}{
		{
			name: "valid config loads",
			body: "env: development\nserver:\n  grpcPort: 50051\nlocalization:\n  defaultLocale: en-NG\n  defaultCurrency: NGN\n",
		},
		{
			name:    "unknown locale rejected at load",
			body:    "env: development\nlocalization:\n  defaultLocale: xx-XX\n",
			wantErr: "defaultLocale",
		},
		{
			name:    "bad currency rejected at load",
			body:    "env: development\nlocalization:\n  defaultCurrency: ngn\n",
			wantErr: "defaultCurrency",
		},
		{
			name:    "bad env rejected at load",
			body:    "env: moon\n",
			wantErr: "env",
		},
	}

	for _, tc := range cases {
		dir := writeConfig(t, tc.body)
		cfg, err := config.Load(dir)
		if tc.wantErr == "" {
			if err != nil {
				t.Errorf("%s: Load() = %v, want nil", tc.name, err)
			}
			continue
		}
		if err == nil {
			t.Errorf("%s: Load() = nil, want error mentioning %q", tc.name, tc.wantErr)
			continue
		}
		if !contains(err.Error(), tc.wantErr) {
			t.Errorf("%s: Load() = %v, want error mentioning %q", tc.name, err, tc.wantErr)
		}
		_ = cfg
	}
}