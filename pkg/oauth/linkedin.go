// internal/platform/oauth/linkedin.go
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
	"golang.org/x/oauth2/linkedin"
)

var _ auth.OAuthProvider = (*LinkedInProvider)(nil)

const (
	linkedinMeAPI    = "https://api.linkedin.com/v2/me?projection=(id,firstName,lastName,profilePicture(displayImage~:playableStreams))"
	linkedinEmailAPI = "https://api.linkedin.com/v2/emailAddress?q=members&projection=(elements*(handle~))"
)

type LinkedInProvider struct {
	config *oauth2.Config
}

func NewLinkedInProvider(cfg ProviderConfig) *LinkedInProvider {
	requiredScopes := []string{"r_liteprofile", "r_emailaddress"}
	scopes := mergeScopes(cfg.Scopes, requiredScopes)

	return &LinkedInProvider{
		config: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Scopes:       scopes,
			Endpoint:     linkedin.Endpoint,
		},
	}
}

func (p *LinkedInProvider) GetAuthCodeURL(state string) string {
	return p.config.AuthCodeURL(state)
}

func (p *LinkedInProvider) ExchangeCodeForUserInfo(ctx context.Context, code string) (*auth.OAuthUserInfo, error) {
	token, err := p.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("linkedin: token exchange failed: %w", err)
	}

	client := p.config.Client(ctx, token)

	// Fetch user profile
	userResp, err := client.Get(linkedinMeAPI)
	if err != nil {
		return nil, fmt.Errorf("linkedin: failed to request user profile: %w", err)
	}
	defer userResp.Body.Close()

	if userResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(userResp.Body)
		return nil, fmt.Errorf("linkedin: user profile endpoint returned status %s: %s", userResp.Status, string(body))
	}

	var userData struct {
		ID        string `json:"id"`
		FirstName struct {
			Localized map[string]string `json:"localized"`
		} `json:"firstName"`
		LastName struct {
			Localized map[string]string `json:"localized"`
		} `json:"lastName"`
	}
	if err := json.NewDecoder(userResp.Body).Decode(&userData); err != nil {
		return nil, fmt.Errorf("linkedin: failed to decode user profile: %w", err)
	}

	// Fetch user email
	emailStr, err := p.fetchPrimaryEmail(ctx, client)
	if err != nil {
		return nil, err
	}
	email, err := core.NewEmail(emailStr)
	if err != nil {
		return nil, fmt.Errorf("linkedin: invalid email from provider: %w", err)
	}

	return &auth.OAuthUserInfo{
		ProviderUserID:  userData.ID,
		Email:           email,
		IsEmailVerified: true, // LinkedIn emails are considered verified
		Name:            fmt.Sprintf("%s %s", userData.FirstName.Localized["en_US"], userData.LastName.Localized["en_US"]),
		RawData: map[string]interface{}{
			"firstName": userData.FirstName.Localized["en_US"],
			"lastName":  userData.LastName.Localized["en_US"],
		},
	}, nil
}

func (p *LinkedInProvider) fetchPrimaryEmail(ctx context.Context, client *http.Client) (string, error) {
	resp, err := client.Get(linkedinEmailAPI)
	if err != nil {
		return "", fmt.Errorf("linkedin: failed to request email: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("linkedin: emails endpoint returned status %s: %s", resp.Status, string(body))
	}

	var emailData struct {
		Elements []struct {
			Handle struct {
				EmailAddress string `json:"emailAddress"`
			} `json:"handle~"`
		} `json:"elements"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&emailData); err != nil {
		return "", fmt.Errorf("linkedin: failed to decode email response: %w", err)
	}

	if len(emailData.Elements) == 0 {
		return "", fmt.Errorf("linkedin: no email found for user")
	}

	return emailData.Elements[0].Handle.EmailAddress, nil
}
