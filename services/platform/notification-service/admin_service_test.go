package notification

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	auditmocks "github.com/combi/pkg/domain/audit/mocks"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/notification"
	notifmocks "github.com/combi/pkg/domain/notification/mocks"
	"github.com/combi/pkg/domain/queue"
	queuemocks "github.com/combi/pkg/domain/queue/mocks"
	"github.com/combi/pkg/log"
	cryptoMocks "github.com/combi/pkg/security/mocks"
)

// ------------------------------------------------------------------
// Suite
// ------------------------------------------------------------------

type adminServiceSuite struct {
	suite.Suite
	ctx        context.Context
	actor      *auth.Actor
	repo       *notifmocks.MockRepository
	dispatcher *queuemocks.MockDispatcher
	auditRec   *auditmocks.MockEventRecorder
	crypto     *cryptoMocks.MockCryptoService
	service    *AdminService
}

func (s *adminServiceSuite) SetupTest() {
	s.actor = &auth.Actor{UserID: uuid.New(), AccountID: uuid.New()}
	s.ctx = auth.NewContextWithActor(context.Background(), s.actor)

	s.repo = notifmocks.NewMockRepository(s.T())
	s.dispatcher = queuemocks.NewMockDispatcher(s.T())
	s.auditRec = auditmocks.NewMockEventRecorder(s.T())
	s.crypto = cryptoMocks.NewMockCryptoService(s.T())
	s.service = NewAdminService(s.repo, s.dispatcher, s.auditRec, s.crypto, &log.NoOpLogger{})
}

func TestAdminService(t *testing.T) {
	suite.Run(t, new(adminServiceSuite))
}

func (s *adminServiceSuite) expectAudit() {
	s.auditRec.EXPECT().
		Record(mock.Anything, mock.Anything). // ← matches any context + any event
		Return(nil)
}

// ------------------------------------------------------------------
// Notification life-cycle
// ------------------------------------------------------------------

func (s *adminServiceSuite) TestGetNotification() {
	id := uuid.New()
	want := &notification.Notification{ID: id}

	s.repo.EXPECT().GetNotification(s.ctx, id).Return(want, nil)

	got, err := s.service.GetNotification(s.ctx, id)
	s.NoError(err)
	s.Equal(want, got)
}

func (s *adminServiceSuite) TestGetNotificationEvents() {
	nID := uuid.New()
	events := []notification.NotificationEvent{{ID: uuid.New(), NotificationID: nID}}

	s.repo.EXPECT().GetNotificationEvents(s.ctx, nID).Return(events, nil)

	res, err := s.service.GetNotificationEvents(s.ctx, nID)
	s.NoError(err)
	s.Equal(events, res)
}

func (s *adminServiceSuite) TestRetryFailedNotification() {
	nID := uuid.New()
	original := &notification.Notification{
		ID:       nID,
		Status:   notification.StatusFailed,
		Attempts: 3,
	}

	// inside tx
	s.repo.EXPECT().WithTx(s.ctx, mock.Anything).Run(func(ctx context.Context, fn func(notification.Repository) error) {
		tx := notifmocks.NewMockRepository(s.T())
		tx.EXPECT().GetNotification(ctx, nID).Return(original, nil)
		tx.EXPECT().UpdateNotification(ctx, mock.AnythingOfType("*notification.Notification")).
			Run(func(_ context.Context, n *notification.Notification) {
				s.Equal(notification.StatusPending, n.Status)
				s.Equal(0, n.Attempts)
				s.Nil(n.LastError)
			}).Return(nil)
		s.NoError(fn(tx))
	}).Return(nil)

	// dispatch & audit
	s.dispatcher.EXPECT().
		Dispatch(s.ctx, notification.JobSendNotification, mock.AnythingOfType("json.RawMessage"), mock.AnythingOfType("queue.DispatchOption")).
		Return(&queue.Job{}, nil)

	s.expectAudit()

	retried, err := s.service.RetryFailedNotification(s.ctx, nID)
	s.NoError(err)
	s.Equal(notification.StatusPending, retried.Status)
}

func (s *adminServiceSuite) TestRetryFailedNotification_Unauthorized() {
	s.ctx = auth.NewContextWithActor(context.Background(), nil)
	_, err := s.service.RetryFailedNotification(s.ctx, uuid.New())
	s.ErrorIs(err, notification.ErrUnauthorized)
}

// ------------------------------------------------------------------
// Template CRUD
// ------------------------------------------------------------------

func (s *adminServiceSuite) TestCreateTemplate() {
	in := notification.NotificationTemplate{Name: "Welcome", Slug: "welcome"}

	s.repo.EXPECT().CreateTemplate(s.ctx, mock.AnythingOfType("*notification.NotificationTemplate")).
		Run(func(_ context.Context, t *notification.NotificationTemplate) {
			s.NotEqual(uuid.Nil, t.ID)
			s.Equal(s.actor.UserID, t.CreatedBy)
		}).Return(nil)
	s.expectAudit()

	created, err := s.service.CreateTemplate(s.ctx, in)
	s.NoError(err)
	s.Equal("Welcome", created.Name)
}

func (s *adminServiceSuite) TestGetTemplate() {
	id := uuid.New()
	want := &notification.NotificationTemplate{ID: id}

	s.repo.EXPECT().GetTemplate(s.ctx, id).Return(want, nil)

	got, err := s.service.GetTemplate(s.ctx, id)
	s.NoError(err)
	s.Equal(want, got)
}

func (s *adminServiceSuite) TestListTemplates() {
	list := []notification.NotificationTemplate{{ID: uuid.New()}}
	s.repo.EXPECT().ListTemplates(s.ctx, 10, 0).Return(list, 1, nil)

	res, total, err := s.service.ListTemplates(s.ctx, 10, 0)
	s.NoError(err)
	s.Equal(list, res)
	s.Equal(1, total)
}

func (s *adminServiceSuite) TestUpdateTemplate() {
	existing := &notification.NotificationTemplate{ID: uuid.New(), Name: "Old"}
	updated := notification.NotificationTemplate{ID: existing.ID, Name: "New", Slug: "new"}

	s.repo.EXPECT().GetTemplate(s.ctx, existing.ID).Return(existing, nil)
	s.repo.EXPECT().UpdateTemplate(s.ctx, mock.AnythingOfType("*notification.NotificationTemplate")).
		Run(func(_ context.Context, t *notification.NotificationTemplate) {
			s.Equal("New", t.Name)
			s.Equal("new", t.Slug)
		}).Return(nil)
	s.expectAudit()
	out, err := s.service.UpdateTemplate(s.ctx, updated)
	s.NoError(err)
	s.Equal("New", out.Name)
}

func (s *adminServiceSuite) TestDeleteTemplate() {
	id := uuid.New()
	tpl := &notification.NotificationTemplate{ID: id, Name: "TPL"}

	s.repo.EXPECT().GetTemplate(s.ctx, id).Return(tpl, nil)
	s.repo.EXPECT().IsTemplateInUse(s.ctx, id).Return(nil, nil) // not in use
	s.repo.EXPECT().DeleteTemplate(s.ctx, id).Return(nil)
	s.expectAudit()
	err := s.service.DeleteTemplate(s.ctx, id)
	s.NoError(err)
}

func (s *adminServiceSuite) TestDeleteTemplate_InUse() {
	id := uuid.New()
	tpl := &notification.NotificationTemplate{ID: id, Name: "TPL"}

	s.repo.EXPECT().GetTemplate(s.ctx, id).Return(tpl, nil)
	s.repo.EXPECT().IsTemplateInUse(s.ctx, id).
		Return(&notification.NotificationTemplate{ID: uuid.New()}, nil)

	err := s.service.DeleteTemplate(s.ctx, id)
	s.Error(err)
	s.ErrorContains(err, "in use")
}

// ------------------------------------------------------------------
// Translations
// ------------------------------------------------------------------

func (s *adminServiceSuite) TestAddTemplateTranslation() {
	tplID := uuid.New()
	tpl := &notification.NotificationTemplate{ID: tplID}
	translation := notification.TemplateTranslation{TemplateID: tplID, Locale: "es"}

	s.repo.EXPECT().GetTemplate(s.ctx, tplID).Return(tpl, nil)
	s.repo.EXPECT().CreateTemplateTranslation(s.ctx, mock.AnythingOfType("*notification.TemplateTranslation")).
		Run(func(_ context.Context, tt *notification.TemplateTranslation) {
			s.NotEqual(uuid.Nil, tt.ID)
		}).Return(nil)
	s.expectAudit()
	created, err := s.service.AddTemplateTranslation(s.ctx, translation)
	s.NoError(err)
	s.Equal("es", created.Locale)
}

// ------------------------------------------------------------------
// Stats
// ------------------------------------------------------------------

func (s *adminServiceSuite) TestGetSystemStats() {
	start, end := time.Now().Add(-24*time.Hour), time.Now()
	stats := &notification.NotificationStats{TotalSent: 42}

	s.repo.EXPECT().GetSystemStats(s.ctx, start, end).Return(stats, nil)

	out, err := s.service.GetSystemStats(s.ctx, start, end)
	s.NoError(err)
	s.Equal(stats, out)
}
