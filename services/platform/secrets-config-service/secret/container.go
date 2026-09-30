package secret

import (
	_ "embed"
	"encoding/base64"
	"fmt"
	"os"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/combi/pkg/config"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/domain/secret"
	"github.com/combi/pkg/persistence/postgres"
	pCache "github.com/combi/pkg/cache"
	pCircuit "github.com/combi/pkg/circuit"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/monitoring"
	pSecret "github.com/combi/pkg/secret"
	pRotator "github.com/combi/pkg/secret/rotator"
	"github.com/combi/pkg/security"
)


type Container struct {
	Service      secret.Service
	AdminService secret.AdminService
	Workers      []queue.JobProcessor
}

type Dependencies struct {
	Config         *config.Config
	DB             *sqlx.DB
	Logger         log.Logger
	AuditRecorder  audit.EventRecorder
	Monitoring     monitoring.Service
	BreakerFactory func(name string) pCircuit.Breaker // Injected Factory
	EventBus       events.Bus
	Dispatcher     queue.Dispatcher
	Registrar      rbac.PermissionRegistrar
	Registry       core.Registry
	Cache          pCache.Service
	RandomSvc      security.RandomService
}

func NewContainer(deps Dependencies) (*Container, error) {
	// 1. Register Permissions
	registerPermissions(deps.Registrar)
	deps.Registry.RegisterService(buildManifest())

	// 2. Build Repository (Postgres + Monitoring + Circuit Breaker)
	// Note: In production, you might switch on deps.Config.Secrets.Provider here
	// to choose between Vault or Postgres. For now, we standardize on Postgres.
	baseRepo := postgres.NewSecretRepository(deps.DB)

	breaker := deps.BreakerFactory("secret-repo")
	repo := pSecret.NewMonitoredRepository(baseRepo, deps.Monitoring, breaker, deps.Logger)

	// 3. Build Crypto Service (Root Key)
	rootCrypto, err := initRootCrypto(deps.Config, deps.RandomSvc, deps.Logger)
	if err != nil {
		return nil, err
	}

	// 4. Build Services
	baseService := NewService(repo, rootCrypto, deps.AuditRecorder, deps.EventBus, deps.Logger)
	cachedService := NewCachedSecretService(baseService, deps.Cache, 5*time.Minute, deps.Logger)

	adminService := NewAdminService(
		repo, rootCrypto, deps.AuditRecorder, deps.EventBus, deps.Dispatcher, deps.Cache, deps.Logger,
	)

	// 5. Build Workers
	rotatorFactory := pRotator.NewRotatorFactory(deps.RandomSvc)
	rotationWorker := NewRotationWorker(
		repo, rootCrypto, deps.AuditRecorder, deps.EventBus, deps.Cache, rotatorFactory, deps.Logger,
	)

	return &Container{
		Service:      cachedService,
		AdminService: adminService,
		Workers:      []queue.JobProcessor{rotationWorker},
	}, nil
}

func registerPermissions(registrar rbac.PermissionRegistrar) {
	const category = "Secrets Management"
	registrar.Register(rbac.PermissionDefinition{
		Name:        secret.PermSecretRead,
		Description: "Allows reading secret metadata and decrypted values.",
		Category:    category,
	})
	registrar.Register(rbac.PermissionDefinition{
		Name:        secret.PermSecretManage,
		Description: "Allows creating, updating, and deleting secrets.",
		Category:    category,
	})
	registrar.Register(rbac.PermissionDefinition{
		Name:        secret.PermSecretRotate,
		Description: "Allows triggering a manual secret rotation.",
		Category:    category,
	})
}

func initRootCrypto(cfg *config.Config, rnd security.RandomService, logger log.Logger) (security.CryptoService, error) {
	rootKeyStr := os.Getenv("ngo_SECRETS_ROOT_KEY")
	var key []byte
	var err error

	if rootKeyStr == "" {
		if cfg.Env == "development" {
			logger.Warn("ngo_SECRETS_ROOT_KEY missing, using dev default")
			// The dev default is a raw 32-byte string, so we can use it directly.
			key = []byte("12345678901234567890123456789012")
		} else {
			return nil, fmt.Errorf("ngo_SECRETS_ROOT_KEY env var is required in production")
		}
	} else {
		// The env var is set, so we assume it's a Base64 encoded key.
		// Decode it to get the raw bytes.
		key, err = base64.StdEncoding.DecodeString(rootKeyStr)
		if err != nil {
			return nil, fmt.Errorf("failed to decode ngo_SECRETS_ROOT_KEY from Base64: %w", err)
		}
	}

	// Now, 'key' holds the raw byte slice, which will be 32 bytes long.
	return security.NewAesGcmCryptoService(key, rnd)
}

func buildManifest() core.ServiceManifest {
	return core.ServiceManifest{
		ServiceName: "secret-service",
		DisplayName: "Secrets Manager",
		Description: "Secure storage and rotation of API keys and credentials.",
		Events: []core.EventDefinition{
			{Key: secret.EventSecretRotated, Label: "Secret Rotated", Description: "Fires when a credential is automatically rotated."},
			{Key: secret.EventSecretRotationFailed, Label: "Rotation Failed", Description: "Fires if an external provider rejects a rotation attempt."},
		},
		Jobs: []core.JobDefinition{
			{
				Key:         secret.JobSecretRotate,
				Label:       "Rotate Secret",
				Description: "Force immediate rotation of a specific credential.",
				InputSchema: map[string]string{"secret_name": "string"},
			},
		},
		Permissions: []core.PermDefinition{
			{Key: secret.PermSecretRead, Label: "Read Secrets", Category: "Security"},
			{Key: secret.PermSecretManage, Label: "Manage Secrets", Category: "Security"},
			{Key: secret.PermSecretRotate, Label: "Force Rotation", Category: "Security"},
		},
	}
}

func (c *Container) InjectAuditRecorder(rec audit.EventRecorder) error {
	// Inject into AdminService
	if admin, ok := c.AdminService.(*AdminService); ok {
		admin.audit = rec
	}

	// Inject into Read Service if it audits access
	if read, ok := c.Service.(*Service); ok {
		read.audit = rec
	}

	return nil
}
