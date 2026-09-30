package consent

import (
	_ "embed"

	"github.com/jmoiron/sqlx"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/consent"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)


type Container struct {
	Service consent.Service
}

type Dependencies struct {
	DB        *sqlx.DB
	Logger    log.Logger
	Registrar rbac.PermissionRegistrar
	Audit     audit.EventRecorder
	Registry  core.Registry
	EventBus  events.Bus
}

func NewContainer(deps Dependencies) (*Container, error) {
	registerPermissions(deps.Registrar)
	deps.Registry.RegisterService(buildManifest())

	repo := postgres.NewConsentRepository(deps.DB, deps.Logger)
	svc := NewService(repo, deps.Audit, deps.EventBus, deps.Logger)

	return &Container{Service: svc}, nil
}

// Consent is a producer of legal events, but rarely a consumer (unless automating revocation on account deletion)
// If we wanted to automate revocation on "user.deleted", we'd add a subscriber here.
func (c *Container) RegisterSubscribers(bus events.Bus) {}

func buildManifest() core.ServiceManifest {
	return core.ServiceManifest{
		ServiceName: "consent-service",
		DisplayName: "Consent Manager",
		Description: "GDPR Consent and Legal Agreements.",
		Events: []core.EventDefinition{
			{Key: core.EventConsentGranted, Label: "Consent Granted"},
			{Key: core.EventConsentRevoked, Label: "Consent Revoked"},
			{Key: core.EventRevisionPublished, Label: "New TOS Version"},
		},
		Permissions: []core.PermDefinition{
			{Key: consent.PermConsentManage, Label: "Manage Agreements", Category: "Legal"},
			{Key: consent.PermConsentViewLogs, Label: "View Consent Logs", Category: "Legal"},
		},
	}
}

func registerPermissions(registrar rbac.PermissionRegistrar) {
	registrar.Register(rbac.PermissionDefinition{
		Name:        consent.PermConsentManage,
		Description: "Can create agreements and publish revisions.",
		Category:    "Consent Management",
	})
	registrar.Register(rbac.PermissionDefinition{
		Name:        consent.PermConsentViewLogs,
		Description: "Can view consent history for audits.",
		Category:    "Consent Management",
	})
}
