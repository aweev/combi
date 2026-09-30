// internal/platform/oauth/google.go
package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/core"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// Statically verify that it implements the interface.
var _ auth.OAuthProvider = (*GoogleProvider)(nil)

const googleUserInfoURL = "https://www.googleapis.com/oauth2/v2/userinfo"

// GoogleProvider implements the OAuthProvider interface for Google (OIDC).
type GoogleProvider struct {
	config *oauth2.Config
}

// NewGoogleProvider creates a new Google OAuth provider.
func NewGoogleProvider(cfg ProviderConfig) *GoogleProvider {
	// Ensure essential OIDC scopes for email & profile are present.
	requiredScopes := []string{"openid", "email", "profile"}
	scopes := mergeScopes(cfg.Scopes, requiredScopes)

	return &GoogleProvider{
		config: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Scopes:       scopes,
			Endpoint:     google.Endpoint,
		},
	}
}

// GetAuthCodeURL returns the URL for the Google login page.
func (p *GoogleProvider) GetAuthCodeURL(state string) string {
	// Request offline access to try to obtain a refresh token on first consent.
	return p.config.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

// ExchangeCodeForUserInfo exchanges an authorization code for user information from Google.
func (p *GoogleProvider) ExchangeCodeForUserInfo(ctx context.Context, code string) (*auth.OAuthUserInfo, error) {
	token, err := p.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("google: token exchange failed: %w", err)
	}

	client := p.config.Client(ctx, token)
	resp, err := client.Get(googleUserInfoURL)
	if err != nil {
		return nil, fmt.Errorf("google: failed to request userinfo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google: userinfo endpoint returned status %s", resp.Status)
	}

	var userInfo struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("google: failed to decode userinfo: %w", err)
	}

	email, err := core.NewEmail(userInfo.Email)
	if err != nil {
		return nil, fmt.Errorf("google: invalid email from provider: %w", err)
	}

	return &auth.OAuthUserInfo{
		ProviderUserID:  userInfo.Sub,
		Email:           email,
		IsEmailVerified: userInfo.EmailVerified,
		Name:            userInfo.Name,
	}, nil
}
