// auth-service — authentication service as an fx library module.
//
// Ported from api/internal/services/auth (Phase 3, Tier 3) onto the combi
// chassis: persistence lives in pkg/persistence/postgres (AuthRepository),
// domain types in pkg/domain/auth. This module has no main package; a host
// binary (cmd/platform-server) mounts it via Module below.
package auth

import (
	"context"
	"encoding/base64"
	"errors"

	"github.com/jmoiron/sqlx"
	"go.uber.org/fx"

	"github.com/combi/pkg/cache"
	"github.com/combi/pkg/config"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/notification"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/domain/secret"
	"github.com/combi/pkg/domain/user"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/fxnoop"
	"github.com/combi/pkg/health"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/oauth"
	"github.com/combi/pkg/security"
)

// Module is the fx module for the auth service.
//
// Requires the fxcore.InfraModule providers in the host's graph:
// *sqlx.DB, *config.Config, log.Logger, cache.Service + security.RandomService
// (platform utils), events.Bus (fxcore).
//
// Optional dependencies — audit recorder, dispatcher, notification service,
// user service, secret service (for the OAuth provider factory), RBAC
// authorization service — arrive nil (dig `optional:"true"`) until their
// modules are mounted. The user service is additionally late-bound via
// SetUserService by the host (mirrors api wiring). Registrar/registry fall
// back to discards so the module also validates standalone.
var Module = fx.Module(
	"auth",
	fx.Provide(
		provideServices,
	),
	fx.Invoke(
		RegisterHooks,
	),
)

type moduleDeps struct {
	fx.In

	DB     *sqlx.DB
	Config *config.Config
	Logger log.Logger
	Cache  cache.Service
	Random security.RandomService
	Bus    events.Bus

	Audit      audit.EventRecorder       `optional:"true"`
	Dispatcher queue.Dispatcher          `optional:"true"`
	NotifSvc   notification.Service      `optional:"true"`
	UserSvc    user.Service              `optional:"true"`
	SecretSvc  secret.Service            `optional:"true"`
	Authz      rbac.AuthorizationService `optional:"true"`

	Registrar rbac.PermissionRegistrar `optional:"true"`
	Registry  core.Registry            `optional:"true"`
}

type servicesOut struct {
	fx.Out

	Container *Container

	Service        auth.Service
	AccountService auth.AccountService
	MFAService     auth.MFAService
	SessionService auth.SessionService
	AdminService   auth.AdminService
	OAuthService   auth.OAuthService
	ClaimService   auth.ClaimService
}

func provideServices(d moduleDeps) (servicesOut, error) {
	registrar := d.Registrar
	if registrar == nil {
		registrar = fxnoop.NoopRegistrar{}
	}
	registry := d.Registry
	if registry == nil {
		registry = fxnoop.NoopRegistry{}
	}
	crypto, err := totpCrypto(d.Config, d.Random)
	if err != nil {
		return servicesOut{}, err
	}
	var oauthFac OAuthProviderFactory
	if d.SecretSvc != nil {
		oauthFac = OAuthProviderFactory(oauth.NewOAuthProviderFactory(&d.Config.OAuth, d.SecretSvc))
	}
	c, err := NewContainer(Dependencies{
		DB:                   d.DB,
		Config:               &d.Config.Auth,
		Logger:               d.Logger,
		Audit:                d.Audit,
		NotifService:         d.NotifSvc,
		UserService:          d.UserSvc,
		CryptoService:        crypto,
		AuthzService:         d.Authz,
		Registrar:            registrar,
		Registry:             registry,
		Cache:                d.Cache,
		OAuthProviderFactory: oauthFac,
		EventBus:             d.Bus,
	})
	if err != nil {
		return servicesOut{}, err
	}
	return servicesOut{
		Container:      c,
		Service:        c.Service,
		AccountService: c.AccountService,
		MFAService:     c.MFAService,
		SessionService: c.SessionService,
		AdminService:   c.AdminService,
		OAuthService:   c.OAuthService,
		ClaimService:   c.ClaimService,
	}, nil
}

// totpCrypto builds the TOTP AES-GCM crypto service from the base64-encoded
// TOTP key (mirrors api wiring: base64 decode, then AES-GCM).
func totpCrypto(cfg *config.Config, random security.RandomService) (security.CryptoService, error) {
	raw := cfg.Auth.TOTPEncryptionKey
	if raw == "" {
		return nil, errors.New("auth.totpEncryptionKey is not set: TOTP secrets cannot be encrypted at rest without it")
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, errors.New("auth.totpEncryptionKey must be base64-encoded")
	}
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return nil, errors.New("auth.totpEncryptionKey must decode to 16, 24, or 32 bytes (AES-128/192/256)")
	}
	return security.NewAesGcmCryptoService(key, random)
}

type hooksDeps struct {
	fx.In

	LC        fx.Lifecycle
	Checker   *health.Checker
	Container *Container
}

func RegisterHooks(d hooksDeps) {
	d.Checker.Register("auth", func(ctx context.Context) error {
		return nil
	})

	d.LC.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return nil
		},
		OnStop: func(_ context.Context) error {
			return nil
		},
	})
}
