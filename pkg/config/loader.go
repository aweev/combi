package config

import (
	"fmt"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

func Load(path string) (*Config, error) {
	_ = godotenv.Load()

	v := viper.New()

	// 1. Load Defaults from YAML
	v.SetConfigName("config") // Expects config.yml
	v.SetConfigType("yml")

	if path != "" && path != "." {
		v.AddConfigPath(path)
	}
	v.AddConfigPath(".")         // Root of project
	v.AddConfigPath("./config")  // Config subdirectory
	v.AddConfigPath("../config") // Parent config (useful for tests/sub-cmds)
	
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// 2. Overwrite with Environment Variables
	// Example: NGO_DATABASE_PASSWORD overrides database.password
	v.SetEnvPrefix("NGO")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// 3. Unmarshal into Struct
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// 4. Fail fast on malformed values (chassis hardening, F1) — a host
	// must never boot half-alive on a typo'd config.
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	if cfg.Env == "production" {
		if err := validateProductionSecrets(&cfg); err != nil {
			return nil, fmt.Errorf("PRODUCTION SECURITY CONFIG ERROR: %w", err)
		}
	}

	return &cfg, nil
}

func validateProductionSecrets(cfg *Config) error {
	invalidValues := []string{"", "change-me", "whsec_change_me_in_prod_12345"}

	checks := map[string]string{
		"Queue.WebhookSecret":      cfg.Queue.WebhookSecret,
		"Automation.Anonymization": cfg.Automation.AnonymizationSecret,
		"Auth.JWT.AccessSecret":    cfg.Auth.JWT.AccessSecret,
		"Database.Password":        cfg.Database.Password,
	}

	for name, value := range checks {
		for _, invalid := range invalidValues {
			if strings.Contains(value, invalid) || len(value) < 16 {
				return fmt.Errorf("field '%s' has an insecure default value. Must be set via Environment Variable", name)
			}
		}
	}
	return nil
}
