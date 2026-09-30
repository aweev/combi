// internal/services/auth/oauth_service.go
package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/user"
	"github.com/combi/pkg/cache"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/security"
)

// Statically verify implementation
var _ auth.OAuthService = (*OAuthService)(nil)

type OAuthProviderFactory func(provider string) (auth.OAuthProvider, error)

type OAuthService struct {
	repo          auth.Repository
	audit         audit.EventRecorder
	mainService   auth.Service // To create sessions and token pairs
	userService   user.Service
	providerFct   OAuthProviderFactory
	cache         cache.Service
	randomService security.TokenGenerator // Re-using for generating the state
	logger        log.Logger
}

func NewOAuthService(
	repo auth.Repository,
	auditRecorder audit.EventRecorder,
	mainService auth.Service,
	userService user.Service,
	providerFactory OAuthProviderFactory,
	cache cache.Service,
	randomService security.TokenGenerator,
	logger log.Logger,
) *OAuthService {
	return &OAuthService{
		repo:          repo,
		audit:         auditRecorder,
		mainService:   mainService,
		userService:   userService,
		providerFct:   providerFactory,
		cache:         cache,
		randomService: randomService,
		logger:        logger.With(log.String("component", "oauth_service")),
	}
}

func (s *OAuthService) Initiate(ctx context.Context, provider string) (redirectURL, state string, err error) {
	oauthProvider, err := s.providerFct(provider)
	if err != nil {
		return "", "", auth.ErrOAuthProviderNotSupported
	}

	state, _, err = s.randomService.Generate()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate state token: %w", err)
	}

	cacheKey := fmt.Sprintf("oauth:state:%s", state)
	if err := s.cache.Set(ctx, cacheKey, []byte("1"), 10*time.Minute); err != nil {
		return "", "", fmt.Errorf("failed to cache state token: %w", err)
	}

	redirectURL = oauthProvider.GetAuthCodeURL(state)
	return redirectURL, state, nil
}

// HandleCallback handles the callback from the provider and logs the user in.
func (s *OAuthService) HandleCallback(ctx context.Context, params auth.HandleCallbackParams) (*auth.LoginResult, error) {
	// 1. Verify state to prevent CSRF.
	cacheKey := fmt.Sprintf("oauth:state:%s", params.State)
	_, err := s.cache.Get(ctx, cacheKey)
	if err != nil {
		return nil, auth.ErrOAuthStateMismatch
	}
	_ = s.cache.Delete(ctx, cacheKey)

	// 2. Exchange code for user info from the provider.
	oauthProvider, err := s.providerFct(params.Provider)
	if err != nil {
		return nil, auth.ErrOAuthProviderNotSupported
	}
	userInfo, err := oauthProvider.ExchangeCodeForUserInfo(ctx, params.Code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange oauth code: %w", err)
	}

	// 3. Find or create the user within a transaction.
	var targetAccount *auth.Account
	err = s.repo.WithTx(ctx, func(txRepo auth.Repository) error {
		// 3a. Look for an existing AuthMethod with this provider's user ID.
		existingMethod, err := txRepo.GetAuthMethodByProvider(ctx, params.Provider, userInfo.ProviderUserID)
		if err == nil && existingMethod != nil {
			acc, err := txRepo.GetAccountByID(ctx, existingMethod.AccountID)
			if err != nil {
				return err
			}
			targetAccount = acc
			return nil
		}

		// 3b. No social login found, check if a local account with this email exists.
		acc, err := txRepo.GetAccountByEmail(ctx, userInfo.Email)
		if err == nil && acc != nil {
			// Account with this email exists, link the new social login.
			newMethod := &auth.AuthMethod{
				ID:         uuid.New(),
				AccountID:  acc.ID,
				Type:       auth.AuthMethodOAuth,
				Provider:   &params.Provider,
				Credential: userInfo.ProviderUserID,
				IsVerified: userInfo.IsEmailVerified,
			}
			if err := txRepo.CreateAuthMethod(ctx, newMethod); err != nil {
				return err
			}
			targetAccount = acc
			return nil
		}

		// 3c. No account found, this is a new user registration.
		newAccount := &auth.Account{
			ID:     uuid.New(),
			UserID: uuid.New(), // This will be the canonical ID for the new user.
			Email:  userInfo.Email,
			Status: auth.StatusActive,
		}
		if !userInfo.IsEmailVerified {
			newAccount.Status = auth.StatusUnverified
		}
		if err := txRepo.CreateAccount(ctx, newAccount); err != nil {
			return err
		}

		newMethod := &auth.AuthMethod{
			ID:         uuid.New(),
			AccountID:  newAccount.ID,
			Type:       auth.AuthMethodOAuth,
			Provider:   &params.Provider,
			Credential: userInfo.ProviderUserID,
			IsVerified: userInfo.IsEmailVerified,
		}
		if err := txRepo.CreateAuthMethod(ctx, newMethod); err != nil {
			return err
		}

		// Create the associated User profile.
		userParams := user.CreateUserParams{
			UserID:    newAccount.UserID,
			Email:     newAccount.Email,
			FirstName: userInfo.Name,
		}
		if _, err := s.userService.CreateUser(ctx, userParams); err != nil {
			return fmt.Errorf("failed to create user profile, rolling back account creation: %w", err)
		}

		evt := audit.NewEvent(newAccount.ID, "user", core.EventAccountRegistered).
			WithSuccess().
			WithMetadata(map[string]interface{}{"provider": params.Provider}).
			WithContext(ctx).Build()

		if err := s.audit.Record(context.Background(), evt); err != nil {
			s.logger.Error("failed to record audit event", err)
		}

		targetAccount = newAccount
		return nil
	})

	if err != nil {
		return nil, err
	}

	// 4. Create a session and token pair for the user.
	tokenPair, err := s.mainService.CreateSessionForAccount(ctx, targetAccount, params.IPAddress, params.UserAgent, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create session after oauth callback: %w", err)
	}

	return &auth.LoginResult{
		TokenPair:     tokenPair,
		MFAIsRequired: false, // Social logins typically bypass MFA for the initial login
	}, nil
}

func (s *OAuthService) SetUserService(userService user.Service) {
	s.userService = userService
}
