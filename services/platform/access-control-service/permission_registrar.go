// access-control-service (package acl) — ported from
// api/internal/services/rbac/permission_registrar.go.
//
// PermissionRegistrar is a thread-safe container for all permission
// definitions discovered at application startup; it reconciles the
// code-defined catalog with the database (SyncWithDataStore) and also
// implements core.Registry for the configuration service.
package acl

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/log"
)

// Statically verify that it implements the domain interfaces.
var _ rbac.PermissionRegistrar = (*PermissionRegistrar)(nil)
var _ core.Registry = (*PermissionRegistrar)(nil)

// PermissionRegistrar is a thread-safe container for all permission definitions
// discovered at application startup. It provides the logic to sync these definitions
// with a persistent data store.
type PermissionRegistrar struct {
	repo        rbac.PermissionRepository
	logger      log.Logger
	definitions map[core.PermissionName]rbac.PermissionDefinition
	services    map[string]core.ServiceManifest // Track registered services
	mu          sync.RWMutex
}

// NewPermissionRegistrar creates a new, empty registrar.
func NewPermissionRegistrar(repo rbac.PermissionRepository, logger log.Logger) *PermissionRegistrar {
	return &PermissionRegistrar{
		repo:        repo,
		logger:      logger.With(log.String("component", "permission_registrar")),
		definitions: make(map[core.PermissionName]rbac.PermissionDefinition),
		services:    make(map[string]core.ServiceManifest),
	}
}

// Register adds a new permission definition to the registrar's collection.
// This method is intended to be called during the application's bootstrap phase.
func (r *PermissionRegistrar) Register(def rbac.PermissionDefinition) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.definitions[def.Name]; exists {
		// In case of duplicate registrations, we log a warning but don't panic,
		// allowing the application to start. The last one registered wins.
		r.logger.Warn("Duplicate permission registration detected. Overwriting.", log.String("permission", string(def.Name)))
	}
	r.definitions[def.Name] = def
}

// GetDefinitions returns a slice of all registered permission definitions.
func (r *PermissionRegistrar) GetDefinitions() []rbac.PermissionDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	defs := make([]rbac.PermissionDefinition, 0, len(r.definitions))
	for _, def := range r.definitions {
		defs = append(defs, def)
	}
	return defs
}

// ============================================================================
// core.Registry Implementation
// ============================================================================

// RegisterPermission implements core.Registry. It converts the
// core.PermDefinition shape (used by service manifests) into the
// rbac.PermissionDefinition shape.
func (r *PermissionRegistrar) RegisterPermission(def core.PermDefinition) {
	rbacDef := rbac.PermissionDefinition{
		Name:        core.PermissionName(def.Key),
		Description: def.Description,
		Category:    def.Category,
	}
	r.Register(rbacDef)
}

// GetPermission implements core.Registry
func (r *PermissionRegistrar) GetPermission(name core.PermissionName) (core.PermDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	def, ok := r.definitions[name]
	if !ok {
		return core.PermDefinition{}, false
	}

	return core.PermDefinition{
		Key:         def.Name,
		Description: def.Description,
		Category:    def.Category,
	}, true
}

// ListPermissions implements core.Registry
func (r *PermissionRegistrar) ListPermissions() []core.PermDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	perms := make([]core.PermDefinition, 0, len(r.definitions))
	for _, def := range r.definitions {
		perms = append(perms, core.PermDefinition{
			Key:         def.Name,
			Description: def.Description,
			Category:    def.Category,
		})
	}
	return perms
}

// RegisterService implements core.Registry
func (r *PermissionRegistrar) RegisterService(manifest core.ServiceManifest) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.services[manifest.ServiceName] = manifest
	r.logger.Info("Registered service",
		log.String("service", manifest.ServiceName),
		log.Int("permissions", len(manifest.Permissions)),
		log.Int("events", len(manifest.Events)),
		log.Int("jobs", len(manifest.Jobs)))

	// Auto-register all permissions from this service
	for _, perm := range manifest.Permissions {
		if _, exists := r.definitions[perm.Key]; exists {
			r.logger.Warn("Duplicate permission from service manifest. Overwriting.",
				log.String("permission", string(perm.Key)),
				log.String("service", manifest.ServiceName))
		}
		r.definitions[perm.Key] = rbac.PermissionDefinition{
			Name:        perm.Key,
			Description: perm.Description,
			Category:    perm.Category,
		}
	}
}

// GetService implements core.Registry
func (r *PermissionRegistrar) GetService(name string) (core.ServiceManifest, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	svc, ok := r.services[name]
	return svc, ok
}

// ListServices implements core.Registry
func (r *PermissionRegistrar) ListServices() []core.ServiceManifest {
	r.mu.RLock()
	defer r.mu.RUnlock()

	svcs := make([]core.ServiceManifest, 0, len(r.services))
	for _, svc := range r.services {
		svcs = append(svcs, svc)
	}
	return svcs
}

// GetSystemCapabilities implements core.Registry
func (r *PermissionRegistrar) GetSystemCapabilities(ctx context.Context) ([]core.ServiceManifest, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Collect all events and jobs from registered services
	var allEvents []core.EventDefinition
	var allJobs []core.JobDefinition

	for _, svc := range r.services {
		allEvents = append(allEvents, svc.Events...)
		allJobs = append(allJobs, svc.Jobs...)
	}

	// NOTE: list permissions inline rather than calling r.ListPermissions():
	// we already hold the read lock, and re-entering RLock from the same
	// goroutine can deadlock when a writer is queued between the two calls.
	perms := make([]core.PermDefinition, 0, len(r.definitions))
	for _, def := range r.definitions {
		perms = append(perms, core.PermDefinition{
			Key:         def.Name,
			Description: def.Description,
			Category:    def.Category,
		})
	}

	// Return the comprehensive capabilities struct, and nil for the error
	return []core.ServiceManifest{
		{
			Permissions: perms,
			Events:      allEvents,
			Jobs:        allJobs,
		},
	}, nil
}

// ============================================================================
// Database Synchronization
// ============================================================================

// SyncWithDataStore is the core method that reconciles the code-defined permissions
// with the permissions stored in the database.
func (r *PermissionRegistrar) SyncWithDataStore(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	r.logger.Info("Starting synchronization of code-defined permissions with the database...")

	// 1. Fetch all existing permissions from the database.
	// We use a large limit to fetch all permissions in one go.
	existingPermissions, _, err := r.repo.ListPermissions(ctx, rbac.PermissionFilters{Limit: 10000})
	if err != nil {
		r.logger.Error("Failed to list existing permissions from database", err)
		return err
	}

	existingMap := make(map[core.PermissionName]*rbac.Permission)
	for _, p := range existingPermissions {
		existingMap[p.Name] = p
	}

	var toUpsert []*rbac.Permission
	foundInCode := make(map[core.PermissionName]bool)
	now := time.Now().UTC()

	// 2. Compare code definitions against existing DB records.
	for name, def := range r.definitions {
		foundInCode[name] = true
		existing, exists := existingMap[name]

		if !exists {
			// New permission: Add to the upsert list.
			r.logger.Info("New permission found, will be created.", log.String("permission", string(name)))
			toUpsert = append(toUpsert, &rbac.Permission{
				ID:          uuid.New(),
				Name:        name,
				Description: def.Description,
				Category:    def.Category,
				IsSystem:    true, // All code-defined permissions are system permissions.
				CreatedAt:   now,
				UpdatedAt:   now,
			})
		} else if existing.Description != def.Description || existing.Category != def.Category {
			// Existing permission with changes: Update and add to upsert list.
			r.logger.Info("Existing permission has changed, will be updated.", log.String("permission", string(name)))
			existing.Description = def.Description
			existing.Category = def.Category
			existing.UpdatedAt = now
			toUpsert = append(toUpsert, existing)
		}
	}

	// 3. Perform the bulk upsert operation if there are any changes.
	if len(toUpsert) > 0 {
		if err := r.repo.UpsertPermissions(ctx, toUpsert); err != nil {
			r.logger.Error("Failed to upsert permissions in the database", err)
			return err
		}
		r.logger.Info("Successfully upserted permissions.", log.Int("count", len(toUpsert)))
	} else {
		r.logger.Info("No new or changed permissions to sync.")
	}

	// 4. (Optional) Identify and flag orphaned permissions.
	for name, perm := range existingMap {
		if !foundInCode[name] && perm.IsSystem {
			r.logger.Warn("Orphaned system permission found in database. It is defined in the DB but no longer in the code. Please consider manual review or cleanup.",
				log.String("permission", string(name)),
			)

			perm.IsSystem = false
			if err := r.repo.UpdatePermission(ctx, perm); err != nil {
				r.logger.Error("Failed to mark orphaned permission as non-system",
					err)
			}
		}
	}

	r.logger.Info("Permission synchronization complete.")
	return nil
}
