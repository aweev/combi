package privacy

import (
	_ "embed"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/privacy"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/log"
)


type Container struct {
	Manager       privacy.Manager
	ErasureWorker queue.JobProcessor
}

type Dependencies struct {
	DB             *sqlx.DB
	Logger         log.Logger
	AnonymizerSvcs []privacy.Anonymizer
	Registry       core.Registry
}

func NewContainer(deps Dependencies) (*Container, error) {
	repo := postgres.NewPrivacyRepository(deps.DB, deps.Logger)
	manager := NewPrivacyManager(repo, deps.Logger)
	deps.Registry.RegisterService(BuildManifest())

	// Register all known anonymizers
	for _, a := range deps.AnonymizerSvcs {
		// FIX: errcheck - Check for registration errors
		if err := manager.RegisterAnonymizer(a); err != nil {
			return nil, fmt.Errorf("failed to register anonymizer: %w", err)
		}
	}
	worker := NewErasureWorker(manager, deps.Logger)

	return &Container{
		Manager:       manager,
		ErasureWorker: worker,
	}, nil
}

func BuildManifest() core.ServiceManifest {
	return core.ServiceManifest{
		ServiceName: "privacy-service",
		DisplayName: "Privacy & Compliance",
		Description: "Manages GDPR erasure requests and data exports.",
		Icon:        "shield-check",
		Events: []core.EventDefinition{
			{
				Key:         core.EventAnonymizationRequested, // CHANGED
				Label:       "Erasure Requested",
				Description: "A user has requested their account be deleted.",
				Schema: map[string]string{
					"user_id": "uuid",
					"reason":  "string",
				},
			},
		},
		Jobs: []core.JobDefinition{
			{
				Key:         "privacy:process_erasure",
				Label:       "Process Erasure",
				Description: "Orchestrates the deletion of user data across all services.",
				InputSchema: map[string]string{
					"request_id": "uuid",
				},
			},
		},
		Permissions: []core.PermDefinition{
			// Privacy requests are usually self-service (Authenticated User),
			// but admins might need oversight.
			{
				Key:         "privacy:read_all",
				Label:       "View Privacy Requests",
				Category:    "Compliance",
				Description: "Allows viewing the status of all privacy requests.",
			},
		},
	}
}
