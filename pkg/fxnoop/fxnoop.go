// Package fxnoop provides inert implementations of the optional collaborator
// interfaces that service modules may be mounted without.
//
// A service module declares its optional dependencies with dig's
// `optional:"true"`. When the host binary does not mount the collaborator, the
// provider receives a nil interface and must substitute something usable, or the
// module fails to construct. Before this package each of the 24 service modules
// hand-rolled its own identical `discardRegistrar` / `discardRegistry` stub.
//
// These types are infrastructure fallbacks, not domain concepts, which is why
// they live here rather than in pkg/domain. Every type is asserted against its
// interface at compile time, so a signature change in pkg/domain breaks here
// instead of silently diverging from 24 copies.
package fxnoop

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/combi/pkg/domain/core"
	queuem "github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/rbac"
)

// Compile-time conformance guarantees.
var (
	_ rbac.PermissionRegistrar = NoopRegistrar{}
	_ core.Registry            = NoopRegistry{}
	_ queuem.Dispatcher        = NoopDispatcher{}
)

// NoopRegistrar satisfies rbac.PermissionRegistrar without recording anything.
// Mount it when the host has no RBAC data store to sync permissions into.
type NoopRegistrar struct{}

func (NoopRegistrar) Register(rbac.PermissionDefinition) {}

func (NoopRegistrar) SyncWithDataStore(context.Context) error { return nil }

func (NoopRegistrar) GetDefinitions() []rbac.PermissionDefinition { return nil }

// NoopRegistry satisfies core.Registry without recording manifests.
// Mount it when the host has no capability registry, so each service's manifest
// is discarded rather than panicking the graph.
type NoopRegistry struct{}

func (NoopRegistry) RegisterService(core.ServiceManifest) {}

func (NoopRegistry) GetSystemCapabilities(context.Context) ([]core.ServiceManifest, error) {
	return nil, nil
}

// NoopDispatcher satisfies queue.Dispatcher by failing loudly.
//
// Unlike the other no-ops this one does not silently succeed: a job that is never
// dispatched is a correctness bug, not a missing optional feature. Returning an
// error keeps callers on their existing error path and makes the gap visible in
// logs. It is deliberately identical to the message the three service copies used.
type NoopDispatcher struct{}

func (NoopDispatcher) Dispatch(_ context.Context, jobType core.JobType, _ json.RawMessage,
	_ ...queuem.DispatchOption) (*queuem.Job, error) {
	return nil, fmt.Errorf("queue service not yet ported: background job %s not dispatched", jobType)
}
