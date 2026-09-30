package auth

import (
	_ "embed"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/combi/pkg/config"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/notification"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/domain/user"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/cache"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/security"
)


// Container holds all public-facing services for the Auth domain.
type Container struct {
	Service        auth.Service
	AccountService auth.AccountService
	MFAService     auth.MFAService
	SessionService auth.SessionService
	AdminService   auth.AdminService
	OAuthService   auth.OAuthService
	ClaimService   auth.ClaimService
	Workers        []queue.JobProcessor
}

// Dependencies holds all external dependencies needed to build the Auth services.
type Dependencies struct {
	DB                   *sqlx.DB
	Config               *config.AuthConfig
	Logger               log.Logger
	Audit                audit.EventRecorder
	NotifService         notification.Service
	UserService          user.Service
	CryptoService        security.CryptoService
	AuthzService         rbac.AuthorizationService
	Registrar            rbac.PermissionRegistrar
	Registry             core.Registry
	Cache                cache.Service
	OAuthProviderFactory OAuthProviderFactory
	EventBus             events.Bus
}

// NewContainer builds and wires the entire Auth service layer.
func NewContainer(deps Dependencies) (*Container, error) {
	// 1. Register Service Manifest (Events, Permissions, Jobs)
	deps.Registry.RegisterService(buildManifest())

	// 2. Register Permissions (Legacy method, technically covered by Manifest but good for double safety if RBAC uses explicit registry)
	deps.Registrar.Register(rbac.PermissionDefinition{Name: auth.PermAuthAccountManage, Description: "Can suspend and manage user accounts.", Category: "Auth"})
	deps.Registrar.Register(rbac.PermissionDefinition{Name: auth.PermAuthSessionRead, Description: "Can view active sessions for any user.", Category: "Auth"})
	deps.Registrar.Register(rbac.PermissionDefinition{Name: auth.PermAuthSessionRevoke, Description: "Can revoke sessions for any user.", Category: "Auth"})
	deps.Registrar.Register(rbac.PermissionDefinition{Name: auth.PermAuthAccountImpersonate, Description: "Can log in as another user.", Category: "Auth"})

	repo := postgres.NewAuthRepository(deps.DB, deps.Logger)

	passService := security.NewPasswordService(deps.Config.PasswordPolicy)
	tokenGen := security.NewTokenGenerator(security.NewRandomService())
	accessSigner, err := security.NewJWTSigner(deps.Config.JWT.AccessSecret, deps.Config.JWT.Issuer)
	if err != nil {
		return nil, fmt.Errorf("failed to create JWT signer: %w", err)
	}
	totpManager := security.NewTOTPManager(deps.CryptoService)

	accountService := NewAccountService(
		repo,
		deps.Audit,
		passService,
		tokenGen,
		deps.NotifService,
		deps.EventBus,
		&deps.Config.Links,
		deps.Logger,
	)
	claimService := NewClaimService(repo, accountService)
	mfaService := NewMFAService(repo, deps.Audit, totpManager, passService, tokenGen, deps.Logger)
	sessionService := NewSessionService(repo, deps.Audit, deps.AuthzService, deps.Logger)

	mainService := NewService(ServiceDependencies{
		Repo:         repo,
		Audit:        deps.Audit,
		PassService:  passService,
		TokenGen:     tokenGen,
		AccessSigner: accessSigner,
		MfaService:   mfaService,
		Logger:       deps.Logger,
		AccessTTL:    deps.Config.JWT.AccessTTL,
		RefreshTTL:   deps.Config.JWT.RefreshTTL,
		EventBus:     deps.EventBus,
		LinkConfig:   &deps.Config.Links,
	})

	adminService := NewAdminService(
		repo,
		deps.Audit,
		accountService,
		mainService,
		deps.AuthzService,
		deps.Logger,
	)

	oauthFactory := deps.OAuthProviderFactory
	if oauthFactory == nil {
		// FIX: SA9003 - Handle empty branch with log
		deps.Logger.Warn("OAuth provider factory not injected, OAuth login will fail if used")
	}

	oauthService := NewOAuthService(
		repo,
		deps.Audit,
		mainService,
		deps.UserService,
		oauthFactory,
		deps.Cache,
		tokenGen,
		deps.Logger,
	)

	tokenWorker := NewTokenCleanupWorker(repo, deps.Logger)
	sessionWorker := NewSessionCleanupWorker(repo, deps.Logger)

	return &Container{
		Service:        mainService,
		AccountService: accountService,
		MFAService:     mfaService,
		SessionService: sessionService,
		AdminService:   adminService,
		OAuthService:   oauthService,
		ClaimService:   claimService,
		Workers: []queue.JobProcessor{
			tokenWorker,
			sessionWorker,
		},
	}, nil
}

func buildManifest() core.ServiceManifest {
	return core.ServiceManifest{
		ServiceName: "auth-service",
		DisplayName: "Authentication & Identity",
		Description: "Manages users, sessions, MFA, and OAuth providers.",

		// 1. Events this service emits (for other services to consume)
		Events: []core.EventDefinition{
			{Key: core.EventAccountRegistered, Label: "User Registered", Description: "New account created"},
			{Key: core.EventLoginSuccess, Label: "Login Success", Description: "User logged in successfully"},
			{Key: core.EventLoginFailed, Label: "Login Failed", Description: "Invalid credentials provided"},
			{Key: core.EventPasswordChanged, Label: "Password Changed", Description: "User updated their password"},
			{Key: core.EventMfaEnabled, Label: "MFA Enabled", Description: "2FA turned on for account"},
			{Key: core.EventMfaDisabled, Label: "MFA Disabled", Description: "2FA turned off for account"},
			{Key: core.EventAccountSuspended, Label: "Account Suspended", Description: "Admin banned a user"},
		},

		// 2. Permissions this service requires/defines
		Permissions: []core.PermDefinition{
			{
				Key:         auth.PermAuthAccountManage,
				Label:       "Manage Accounts",
				Description: "Can suspend/unsuspend users and update emails.",
				Category:    "Identity",
			},
			{
				Key:         auth.PermAuthSessionRead,
				Label:       "View Sessions",
				Description: "Can inspect active devices for users.",
				Category:    "Identity",
			},
			{
				Key:         auth.PermAuthSessionRevoke,
				Label:       "Revoke Sessions",
				Description: "Can force logout users.",
				Category:    "Identity",
			},
			{
				Key:         auth.PermAuthAccountImpersonate,
				Label:       "Impersonate Users",
				Description: "Can log in as another user for support.",
				Category:    "Identity",
				Dangerous:   true, // UI hint to show in red/warning
			},
		},

		// 3. Background Jobs this service owns
		Jobs: []core.JobDefinition{
			{
				Key:         auth.JobAuthCleanupExpiredTokens,
				Label:       "Cleanup Expired Tokens",
				Description: "Deletes old verification and reset tokens from the database.",
			},
			{
				Key:         auth.JobAuthCleanupExpiredSessions,
				Label:       "Cleanup Expired Sessions",
				Description: "Deletes old user sessions to reclaim storage space.",
			},
		},
	}
}
