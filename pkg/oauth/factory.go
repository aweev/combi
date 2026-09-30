// internal/platform/oauth/factory.go
package oauth

import (
	"context"
	"fmt"

	"github.com/combi/pkg/config"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/secret"
)

// ProviderConfig holds the necessary credentials for an OAuth provider.
type ProviderConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
}

// NewOAuthProviderFactory creates a factory function for OAuth providers.
// It returns a raw function signature to avoid importing the service layer,
// preventing circular dependencies.
func NewOAuthProviderFactory(cfg *config.OAuthConfig, secretSvc secret.Service) func(string) (auth.OAuthProvider, error) {
	return func(providerName string) (auth.OAuthProvider, error) {
		providerCfg, exists := cfg.Providers[providerName]
		if !exists {
			return nil, fmt.Errorf("unsupported or unconfigured oauth provider: %s", providerName)
		}

		// We use context.Background() here because the factory signature typically
		// doesn't support context passing, and secret fetching is required.
		secretPath := fmt.Sprintf("oauth.%s", providerName)
		secrets, err := secretSvc.GetSecretValue(context.Background(), secretPath)
		if err != nil {
			return nil, fmt.Errorf("failed to get secrets for oauth provider '%s': %w", providerName, err)
		}

		clientSecret, ok := secrets["client_secret"].(string)
		if !ok || clientSecret == "" {
			return nil, fmt.Errorf("client_secret not found or empty for oauth provider '%s'", providerName)
		}

		pCfg := ProviderConfig{
			ClientID:     providerCfg.ClientID,
			ClientSecret: clientSecret,
			RedirectURL:  providerCfg.RedirectURL,
			Scopes:       providerCfg.Scopes,
		}

		switch providerName {
		case "google":
			return NewGoogleProvider(pCfg), nil
		case "github":
			return NewGitHubProvider(pCfg), nil
		case "facebook":
			return NewFacebookProvider(pCfg), nil
		case "linkedin":
			return NewLinkedInProvider(pCfg), nil
		default:
			return nil, fmt.Errorf("unsupported oauth provider: %s", providerName)
		}
	}
}

// mergeScopes is a helper to combine default/required scopes with user-configured scopes.
func mergeScopes(userScopes, requiredScopes []string) []string {
	scopeSet := make(map[string]struct{})
	for _, s := range userScopes {
		scopeSet[s] = struct{}{}
	}
	for _, s := range requiredScopes {
		scopeSet[s] = struct{}{}
	}

	finalScopes := make([]string, 0, len(scopeSet))
	for s := range scopeSet {
		finalScopes = append(finalScopes, s)
	}
	return finalScopes
}
