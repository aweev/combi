// // internal/domain/organization/events.go
// package organization

// import "github.com/combi/pkg/domain/core"

// const (
// EventOrganizationCreated      core.EventAction = "organization.created"
// EventOrganizationUpdated      core.EventAction = "organization.updated"
// EventOrganizationDeleted      core.EventAction = "organization.deleted"
// EventOrganizationOwnerChanged core.EventAction = "organization.owner.changed"

// EventMemberInvited     core.EventAction = "organization.member.invited"
// EventMemberJoined      core.EventAction = "organization.member.joined"
// EventMemberRemoved     core.EventAction = "organization.member.removed"
// EventMemberRoleChanged core.EventAction = "organization.member.role_changed"

// EventInvitationAccepted core.EventAction = "organization.invitation.accepted"
// EventInvitationDeclined core.EventAction = "organization.invitation.declined"
// )

package organization

import "github.com/google/uuid"

type OrgCreatedPayload struct {
	OrgID   uuid.UUID `json:"org_id"`
	Name    string    `json:"name"`
	OwnerID uuid.UUID `json:"owner_id"`
	Type    string    `json:"type"` // "nonprofit", "corporate"
}

type MemberAddedPayload struct {
	OrgID  uuid.UUID `json:"org_id"`
	UserID uuid.UUID `json:"user_id"`
	Role   string    `json:"role"` // "admin", "member"
}
