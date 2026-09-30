package secret

import (
	"fmt"

	"github.com/combi/pkg/config"
	"github.com/combi/pkg/domain/secret"
	"github.com/combi/pkg/secret/env"
	"github.com/combi/pkg/secret/vault"
	"github.com/hashicorp/vault/api"
)

// NewRepository creates the appropriate repository backend based on config.
func NewRepository(cfg config.SecretsConfig) (secret.Repository, error) {
	switch cfg.Provider {
	case "env":
		return env.NewEnvRepository()
	case "vault":
		clientConfig := api.DefaultConfig()
		clientConfig.Address = cfg.Vault.Address

		client, err := api.NewClient(clientConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create vault client: %w", err)
		}

		if cfg.Vault.Token != "" {
			client.SetToken(cfg.Vault.Token)
		}

		return vault.NewVaultRepository(client, cfg.Vault.Prefix), nil
	default:
		return nil, fmt.Errorf("unknown secret provider: %s", cfg.Provider)
	}
}
