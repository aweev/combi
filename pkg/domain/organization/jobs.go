// internal/domain/organization/jobs.go
package organization

import "github.com/combi/pkg/domain/core"

const (
	// JobOrgSendInviteEmail sends an email to a user who has been invited to an organization.
	JobOrgSendInviteEmail core.JobType = "organization:send_invite_email"
	// JobOrgCleanupExpiredInvites periodically removes expired, pending invitations.
	JobOrgCleanupExpiredInvites core.JobType = "organization:cleanup_expired_invites"
)
