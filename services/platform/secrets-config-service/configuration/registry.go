package configuration

import (
	"context"
	"sync"

	"github.com/combi/pkg/domain/core"
)

// RegistryImpl holds the in-memory map of what all other services can do.
type RegistryImpl struct {
	manifests []core.ServiceManifest
	mu        sync.RWMutex
}

func NewRegistry() *RegistryImpl {
	return &RegistryImpl{
		manifests: make([]core.ServiceManifest, 0),
	}
}

// RegisterService is called by other containers (e.g., Analytics, Secret) during startup.
func (r *RegistryImpl) RegisterService(m core.ServiceManifest) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Dedup: If service already exists (e.g. restart), replace it
	for i, existing := range r.manifests {
		if existing.ServiceName == m.ServiceName {
			r.manifests[i] = m
			return
		}
	}

	r.manifests = append(r.manifests, m)
}

func (r *RegistryImpl) GetSystemCapabilities(ctx context.Context) ([]core.ServiceManifest, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Return a copy to prevent race conditions if caller modifies it
	result := make([]core.ServiceManifest, len(r.manifests))
	copy(result, r.manifests)

	return result, nil
}

// --- Bridge Methods to Expose via Service Interface ---
// These allow the main Service struct to satisfy core.Registry

func (s *Service) RegisterService(m core.ServiceManifest) {
	s.registry.RegisterService(m)
}

func (s *Service) GetSystemCapabilities(ctx context.Context) ([]core.ServiceManifest, error) {
	return s.registry.GetSystemCapabilities(ctx)
}
