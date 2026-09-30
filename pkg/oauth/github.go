// internal/platform/oauth/github.go
package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/core"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

var _ auth.OAuthProvider = (*GitHubProvider)(nil)

const (
	githubUserAPI  = "https://api.github.com/user"
	githubEmailAPI = "https://api.github.com/user/emails"
)

type GitHubProvider struct {
	config *oauth2.Config
}

func NewGitHubProvider(cfg ProviderConfig) *GitHubProvider {
	requiredScopes := []string{"user:email"}
	scopes := mergeScopes(cfg.Scopes, requiredScopes)

	return &GitHubProvider{
		config: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Scopes:       scopes,
			Endpoint:     github.Endpoint,
		},
	}
}

func (p *GitHubProvider) GetAuthCodeURL(state string) string {
	return p.config.AuthCodeURL(state)
}

func (p *GitHubProvider) ExchangeCodeForUserInfo(ctx context.Context, code string) (*auth.OAuthUserInfo, error) {
	token, err := p.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("github: token exchange failed: %w", err)
	}

	client := p.config.Client(ctx, token)

	// Fetch user profile
	userResp, err := client.Get(githubUserAPI)
	if err != nil {
		return nil, fmt.Errorf("github: failed to request user profile: %w", err)
	}
	defer userResp.Body.Close()

	if userResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github: user profile endpoint returned status %s", userResp.Status)
	}

	var userData struct {
		ID    int64  `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"` // Can be null
	}
	if err := json.NewDecoder(userResp.Body).Decode(&userData); err != nil {
		return nil, fmt.Errorf("github: failed to decode user profile: %w", err)
	}

	// Fetch user emails
	emailStr, isVerified, err := p.fetchPrimaryEmail(ctx, client)
	if err != nil {
		// If the user has a public email, use that as a fallback
		if userData.Email != "" {
			emailStr = userData.Email
			// We can't be sure it's verified from this endpoint
			isVerified = false
		} else {
			return nil, err // No email found
		}
	}

	email, err := core.NewEmail(emailStr)
	if err != nil {
		return nil, fmt.Errorf("github: invalid email from provider: %w", err)
	}

	return &auth.OAuthUserInfo{
		ProviderUserID:  fmt.Sprintf("%d", userData.ID),
		Email:           email,
		IsEmailVerified: isVerified,
		Name:            userData.Name,
	}, nil
}

func (p *GitHubProvider) fetchPrimaryEmail(ctx context.Context, client *http.Client) (string, bool, error) {
	resp, err := client.Get(githubEmailAPI)
	if err != nil {
		return "", false, fmt.Errorf("github: failed to request emails: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("github: emails endpoint returned status %s", resp.Status)
	}

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", false, fmt.Errorf("github: failed to decode emails: %w", err)
	}

	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, true, nil
		}
	}

	// Fallback if no primary verified email is found
	if len(emails) > 0 {
		return emails[0].Email, emails[0].Verified, nil
	}

	return "", false, fmt.Errorf("github: no email found for user")
}
