package realtime

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/domain/realtime"
	"github.com/combi/pkg/log"
)

type RBACAuthorizer struct {
	rbac   rbac.AuthorizationService
	logger log.Logger
}

func NewRBACAuthorizer(rbac rbac.AuthorizationService, logger log.Logger) *RBACAuthorizer {
	return &RBACAuthorizer{rbac: rbac, logger: logger}
}

func (a *RBACAuthorizer) CanSubscribe(ctx context.Context, userID *uuid.UUID, channel realtime.ChannelID) (bool, error) {
	sChan := string(channel)

	// 1. Public Channels
	if strings.HasPrefix(sChan, "public:") {
		return true, nil
	}

	// 2. User Channels (Must match ID)
	if userID == nil {
		return false, nil
	}
	if channel.IsUserChannel() {
		return *userID == channel.GetUserID(), nil
	}

	// 3. RBAC Channels (e.g. "admin:dashboard" -> Check "realtime:subscribe:admin:dashboard")
	perm := core.PermissionName(fmt.Sprintf("%s:%s", realtime.PermRealtimeSubscribe, sChan))
	return a.rbac.Authorize(ctx, rbac.PermissionCheck{
		ActorID:        *userID,
		PermissionName: perm,
	})
}
