// internal/platform/oauth/facebook.go
package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/core"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/facebook"
)

// Statically verify that it implements the interface.
var _ auth.OAuthProvider = (*FacebookProvider)(nil)

const fbGraphMe = "https://graph.facebook.com/me?fields=id,name,email,picture"

type FacebookProvider struct {
	config *oauth2.Config
}

func NewFacebookProvider(cfg ProviderConfig) *FacebookProvider {
	requiredScopes := []string{"email", "public_profile"}
	scopes := mergeScopes(cfg.Scopes, requiredScopes)

	return &FacebookProvider{
		config: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Scopes:       scopes,
			Endpoint:     facebook.Endpoint,
		},
	}
}

func (p *FacebookProvider) GetAuthCodeURL(state string) string {
	return p.config.AuthCodeURL(state)
}

func (p *FacebookProvider) ExchangeCodeForUserInfo(ctx context.Context, code string) (*auth.OAuthUserInfo, error) {
	token, err := p.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("facebook: token exchange failed: %w", err)
	}

	client := p.config.Client(ctx, token)
	resp, err := client.Get(fbGraphMe)
	if err != nil {
		return nil, fmt.Errorf("facebook: failed to request userinfo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("facebook: userinfo endpoint returned status %s: %s", resp.Status, string(body))
	}

	var fbData struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Email   string `json:"email"` // May be empty if user denies permission
		Picture struct {
			Data struct {
				URL string `json:"url"`
			} `json:"data"`
		} `json:"picture"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&fbData); err != nil {
		return nil, fmt.Errorf("facebook: failed to decode userinfo: %w", err)
	}

	if fbData.Email == "" {
		return nil, fmt.Errorf("facebook: user did not grant email permission")
	}

	email, err := core.NewEmail(fbData.Email)
	if err != nil {
		return nil, fmt.Errorf("facebook: invalid email from provider: %w", err)
	}

	return &auth.OAuthUserInfo{
		ProviderUserID:  fbData.ID,
		Email:           email,
		Name:            fbData.Name,
		PictureURL:      fbData.Picture.Data.URL,
		IsEmailVerified: true, // Facebook only returns verified emails
		RawData: map[string]interface{}{
			"name": fbData.Name,
		},
	}, nil
}
