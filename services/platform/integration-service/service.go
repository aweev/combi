package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/integration"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/security"
)

var _ integration.AdminService = (*Service)(nil)
var _ integration.WebhookService = (*Service)(nil)

type ServiceDependencies struct {
	Repo            integration.Repository
	CryptoService   security.CryptoService
	ProviderFactory integration.ProviderFactory
	AuditRecorder   audit.EventRecorder
	Dispatcher      queue.Dispatcher
	Logger          log.Logger
}

type Service struct {
	repo          integration.Repository
	cryptoSvc     security.CryptoService
	factory       integration.ProviderFactory
	auditRecorder audit.EventRecorder
	dispatcher    queue.Dispatcher
	logger        log.Logger
}

func NewService(deps ServiceDependencies) *Service {
	return &Service{
		repo:          deps.Repo,
		cryptoSvc:     deps.CryptoService,
		factory:       deps.ProviderFactory,
		auditRecorder: deps.AuditRecorder,
		dispatcher:    deps.Dispatcher,
		logger:        deps.Logger.With(log.String("service", "integration")),
	}
}

// --- AdminService Implementation ---

func (s *Service) Create(ctx context.Context, params integration.CreateIntegrationParams) (*integration.Integration, error) {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return nil, integration.ErrPermissionDenied
	}

	configBytes, err := json.Marshal(params.Configuration)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", integration.ErrInvalidConfiguration, err)
	}

	credBytes, err := json.Marshal(params.Credentials)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", integration.ErrInvalidConfiguration, err)
	}

	encryptedCreds, encContext, err := s.cryptoSvc.Encrypt(ctx, credBytes)
	if err != nil {
		return nil, integration.ErrCryptoOperationFailed
	}

	integ := &integration.Integration{
		ID:                   uuid.New(),
		Provider:             params.Provider,
		Name:                 params.Name,
		Status:               integration.StatusActive,
		CredentialsEncrypted: encryptedCreds,
		EncryptionContext:    encContext,
		Configuration:        configBytes,
		CreatedAt:            time.Now().UTC(),
		UpdatedAt:            time.Now().UTC(),
	}

	if err := s.repo.CreateIntegration(ctx, integ); err != nil {
		return nil, err
	}

	s.audit(ctx, actor.UserID, core.EventIntegrationCreated, integ.ID, integ.Name)

	return integ, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*integration.Integration, error) {
	return s.repo.GetIntegration(ctx, id)
}

func (s *Service) GetWithCredentials(ctx context.Context, id uuid.UUID) (*integration.Integration, map[string]string, error) {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return nil, nil, integration.ErrPermissionDenied
	}

	integ, err := s.repo.GetIntegration(ctx, id)
	if err != nil {
		return nil, nil, err
	}

	// Decrypt
	decryptedBytes, err := s.cryptoSvc.Decrypt(ctx, integ.CredentialsEncrypted, integ.EncryptionContext)
	if err != nil {
		s.logger.Error("failed decryption of integration credentials", err, log.String("integration_id", id.String()))
		return nil, nil, integration.ErrCryptoOperationFailed
	}

	var creds map[string]string
	if err := json.Unmarshal(decryptedBytes, &creds); err != nil {
		return nil, nil, integration.ErrInvalidConfiguration
	}

	// Mask credentials for display
	maskedCreds := maskCredentials(creds)

	// LINT FIX: Check error on Record
	if err := s.auditRecorder.Record(ctx, audit.NewEvent(actor.UserID, "user", core.EventIntegrationConnectionTested).
		WithTarget(integ.ID, "integration", "credentials_viewed").
		WithSuccess().Build()); err != nil {
		s.logger.Warn("failed to record audit event", log.ErrorAttr(err))
	}

	return integ, maskedCreds, nil
}

func (s *Service) List(ctx context.Context, filter integration.IntegrationFilter) ([]*integration.Integration, int, error) {
	return s.repo.ListIntegrations(ctx, filter)
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, params integration.UpdateIntegrationParams) (*integration.Integration, error) {
	integ, err := s.repo.GetIntegration(ctx, id)
	if err != nil {
		return nil, err
	}

	if params.Name != nil {
		integ.Name = *params.Name
	}
	if params.Status != nil {
		integ.Status = *params.Status
	}
	if params.Configuration != nil {
		// LINT FIX: Handle json marshal error
		bytes, err := json.Marshal(params.Configuration)
		if err != nil {
			return nil, integration.ErrInvalidConfiguration
		}
		integ.Configuration = bytes
	}
	if params.Credentials != nil {
		// LINT FIX: Handle json marshal error
		bytes, err := json.Marshal(params.Credentials)
		if err != nil {
			return nil, integration.ErrInvalidConfiguration
		}
		enc, ctx, err := s.cryptoSvc.Encrypt(ctx, bytes)
		if err != nil {
			return nil, integration.ErrCryptoOperationFailed
		}
		integ.CredentialsEncrypted = enc
		integ.EncryptionContext = ctx
	}

	integ.UpdatedAt = time.Now().UTC()
	if err := s.repo.UpdateIntegration(ctx, integ); err != nil {
		return nil, err
	}

	if actor := auth.ActorFromContext(ctx); actor != nil {
		s.audit(ctx, actor.UserID, core.EventIntegrationUpdated, integ.ID, integ.Name)
	}

	return integ, nil
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.DeleteIntegration(ctx, id); err != nil {
		return err
	}
	if actor := auth.ActorFromContext(ctx); actor != nil {
		// LINT FIX: Check error on Record
		if err := s.auditRecorder.Record(ctx, audit.NewEvent(actor.UserID, "user", core.EventIntegrationDeleted).
			WithTarget(id, "integration", "").WithSuccess().Build()); err != nil {
			s.logger.Warn("failed to record audit event", log.ErrorAttr(err))
		}
	}
	return nil
}

func (s *Service) TestConnection(ctx context.Context, id uuid.UUID) (bool, string, error) {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return false, "", integration.ErrPermissionDenied
	}

	integ, err := s.repo.GetIntegration(ctx, id)
	if err != nil {
		return false, "", err
	}

	// Decrypt credentials internally
	decryptedBytes, err := s.cryptoSvc.Decrypt(ctx, integ.CredentialsEncrypted, integ.EncryptionContext)
	if err != nil {
		return false, "crypto error", err
	}
	var creds map[string]string
	if err := json.Unmarshal(decryptedBytes, &creds); err != nil {
		return false, "config error", err
	}

	// Build Adapter using Factory
	var adapter interface{}
	var buildErr error

	if integ.Provider.IsPaymentGateway() {
		adapter, buildErr = s.factory.BuildPaymentGateway(integ.Provider, creds)
	} else if integ.Provider.IsNotificationProvider() {
		adapter, buildErr = s.factory.BuildEmailProvider(integ.Provider, creds)
		if buildErr != nil && integ.Provider == core.ProviderTwilio {
			adapter, buildErr = s.factory.BuildSMSProvider(integ.Provider, creds)
		}
	} else {
		return false, fmt.Sprintf("provider type %s not supported for testing", integ.Provider), nil
	}

	if buildErr != nil {
		return false, buildErr.Error(), nil
	}

	// Execute Test
	if tester, ok := adapter.(integration.ConnectionTester); ok {
		if err := tester.TestConnection(ctx); err != nil {
			return false, err.Error(), nil
		}

		s.audit(ctx, actor.UserID, core.EventIntegrationConnectionTested, integ.ID, "status_check")

		return true, "Success", nil
	}

	return true, "Initialized (No Test Capability Available)", nil
}

// --- WebhookService Implementation ---

func (s *Service) ProcessWebhook(ctx context.Context, providerType core.ProviderType, r *http.Request) (*integration.Webhook, error) {
	// 1. Find active integration for this provider
	integ, err := s.repo.GetActiveIntegrationByProvider(ctx, providerType)
	if err != nil {
		s.logger.Warn("received webhook for unknown or inactive provider", log.String("provider", string(providerType)))
		return nil, integration.ErrIntegrationNotFound
	}

	// Decrypt creds for signature verification
	decryptedBytes, err := s.cryptoSvc.Decrypt(ctx, integ.CredentialsEncrypted, integ.EncryptionContext)
	if err != nil {
		s.logger.Error("failed to decrypt credentials for webhook verification", err)
		return nil, integration.ErrCryptoOperationFailed
	}
	var creds map[string]string
	// LINT FIX: Check error for Unmarshal
	if err := json.Unmarshal(decryptedBytes, &creds); err != nil {
		s.logger.Error("failed to unmarshal credentials", err, log.String("integration_id", integ.ID.String()))
		return nil, integration.ErrInvalidConfiguration
	}

	// 2. Get Parser
	parser, err := s.factory.GetWebhookParser(integ, creds)
	if err != nil {
		return nil, fmt.Errorf("no parser found: %w", err)
	}

	// 3. Parse & Validate (Signature check happens here)
	eventType, eventID, payload, err := parser.Parse(r, integ)
	if err != nil {
		s.logger.Warn("webhook signature validation failed", log.ErrorAttr(err), log.String("provider", string(providerType)))
		return nil, integration.ErrInvalidSignature
	}

	// 4. Idempotency Check
	if _, err := s.repo.GetWebhookByProviderID(ctx, providerType, eventID); err == nil {
		s.logger.Info("duplicate webhook ignored", log.String("event_id", eventID))
		return nil, integration.ErrDuplicateWebhook
	}

	// 5. Persist Webhook
	webhook := &integration.Webhook{
		ID:              uuid.New(),
		IntegrationID:   integ.ID,
		Provider:        providerType,
		ProviderEventID: eventID,
		EventType:       eventType,
		Status:          integration.WebhookPending,
		Payload:         payload,
		ReceivedAt:      time.Now().UTC(),
	}
	if err := s.repo.CreateWebhook(ctx, webhook); err != nil {
		return nil, err
	}

	// 6. Queue for Async Processing
	// LINT FIX: Check json Marshal error
	jobPayload, err := json.Marshal(map[string]string{"webhook_id": webhook.ID.String()})
	if err != nil {
		// Should rarely happen with a simple map
		s.logger.Error("failed to marshal job payload", err)
		return webhook, nil // Return webhook anyway as it is persisted
	}

	_, err = s.dispatcher.Dispatch(ctx, integration.JobIntegrationProcessWebhook, jobPayload, queue.WithPriority(0)) // High priority
	if err != nil {
		s.logger.Error("failed to dispatch webhook processing job", err, log.String("webhook_id", webhook.ID.String()))
		// We return nil error so the provider gets a 200 OK, as we have persisted the webhook.
	}

	return webhook, nil
}

// --- Helpers ---

func maskCredentials(creds map[string]string) map[string]string {
	masked := make(map[string]string)
	for k, v := range creds {
		if len(v) > 8 {
			masked[k] = v[:4] + "..." + v[len(v)-4:]
		} else {
			masked[k] = "********"
		}
	}
	return masked
}

func (s *Service) audit(ctx context.Context, actorID uuid.UUID, action core.EventAction, targetID uuid.UUID, msg string) {
	if err := s.auditRecorder.Record(ctx, audit.NewEvent(actorID, "user", action).
		WithTarget(targetID, "integration", msg).
		WithSuccess().Build()); err != nil {
		s.logger.Warn("failed to record audit event", log.ErrorAttr(err))
	}
}
