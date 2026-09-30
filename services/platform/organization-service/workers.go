package organization

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/notification"
	"github.com/combi/pkg/domain/organization"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/log"
)

// InviteCleanupWorker periodically marks old pending invites as expired in the DB.
type InviteCleanupWorker struct {
	repo   organization.Repository
	logger log.Logger
}

func NewInviteCleanupWorker(repo organization.Repository, logger log.Logger) *InviteCleanupWorker {
	return &InviteCleanupWorker{
		repo:   repo,
		logger: logger.With(log.String("worker", "org_invite_cleanup")),
	}
}

func (w *InviteCleanupWorker) JobType() core.JobType {
	return organization.JobOrgCleanupExpiredInvites
}

func (w *InviteCleanupWorker) ProcessJob(ctx context.Context, job *queue.Job) (json.RawMessage, error) {
	// The payload might define a custom time, but we usually default to "NOW"
	now := time.Now().UTC()

	count, err := w.repo.ExpirePendingInvitations(ctx, now)
	if err != nil {
		return nil, err
	}

	if count > 0 {
		w.logger.Info("Expired old invitations", log.Int("count", int(count)))
	}

	return json.Marshal(map[string]int64{"expired_count": count})
}

type InviteSenderWorker struct {
	notifSvc notification.Service
	logger   log.Logger
}

func NewInviteSenderWorker(notifSvc notification.Service, logger log.Logger) *InviteSenderWorker {
	return &InviteSenderWorker{
		notifSvc: notifSvc,
		logger:   logger.With(log.String("worker", "org_invite_sender")),
	}
}

func (w *InviteSenderWorker) JobType() core.JobType {
	return organization.JobOrgSendInviteEmail
}

func (w *InviteSenderWorker) ProcessJob(ctx context.Context, job *queue.Job) (json.RawMessage, error) {
	var payload struct {
		Email   string `json:"email"`
		OrgID   string `json:"org_id"`
		Token   string `json:"token"`
		Inviter string `json:"inviter"`
	}
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return nil, queue.NewNonRetryableError(err)
	}

	// Dispatch notification
	req := notification.DispatchRequest{
		ToAddress: core.MustNewEmail(nil, payload.Email),

		Template: &notification.TemplatePayload{
			Slug: "org-invite",
			Data: map[string]interface{}{
				"invite_link": fmt.Sprintf("https://app.ngo-platform.org/join?token=%s", payload.Token),
				"org_id":      payload.OrgID,
			},
		},
	}

	if _, err := w.notifSvc.Dispatch(ctx, req); err != nil {
		return nil, err
	}

	return nil, nil
}
