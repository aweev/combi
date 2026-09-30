package integration

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/financial"
	"github.com/combi/pkg/domain/integration"
	"github.com/combi/pkg/domain/notification"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/security"
	"github.com/combi/integration/adapters"
)

type ProviderFactory struct {
	logger    log.Logger
	repo      integration.Repository
	cryptoSvc security.CryptoService
}

// Update constructor to take dependencies needed for GetPaymentGateway
func NewProviderFactory(logger log.Logger, repo integration.Repository, cryptoSvc security.CryptoService) *ProviderFactory {
	return &ProviderFactory{
		logger:    logger,
		repo:      repo,
		cryptoSvc: cryptoSvc,
	}
}

// GetPaymentGateway implements the method required by FinancialService.
// It looks up the active payment integration (Stripe/PayPal), decrypts creds, and builds the adapter.
func (f *ProviderFactory) GetPaymentGateway(ctx context.Context) (financial.PaymentGateway, error) {
	// 1. Try to find active Stripe integration
	integ, err := f.repo.GetActiveIntegrationByProvider(ctx, core.ProviderStripe)
	if err != nil {
		// Fallback to PayPal
		integ, err = f.repo.GetActiveIntegrationByProvider(ctx, core.ProviderPayPal)
		if err != nil {
			return nil, fmt.Errorf("no active payment integration found (Stripe or PayPal)")
		}
	}

	// 2. Decrypt credentials
	creds, err := f.decryptCredentials(ctx, integ)
	if err != nil {
		return nil, err
	}

	// 3. Build Adapter
	return f.BuildPaymentGateway(integ.Provider, creds)
}

func (f *ProviderFactory) BuildPaymentGateway(provider core.ProviderType, creds map[string]string) (financial.PaymentGateway, error) {
	switch provider {
	case core.ProviderStripe:
		return adapters.NewStripeAdapter(creds, f.logger)
	case core.ProviderPayPal:
		return adapters.NewPayPalAdapter(creds, f.logger)
	default:
		return nil, fmt.Errorf("provider %s does not support payment gateway interface", provider)
	}
}

func (f *ProviderFactory) BuildEmailProvider(provider core.ProviderType, creds map[string]string) (notification.EmailProvider, error) {
	switch provider {
	case core.ProviderBrevo:
		return adapters.NewBrevoAdapter(creds, f.logger)
	case core.ProviderSES:
		return adapters.NewSESAdapter(creds, f.logger)
	case core.ProviderSMTP:
		return adapters.NewSMTPAdapter(creds, f.logger)
	case core.ProviderSendGrid:
		return adapters.NewSendGridAdapter(creds, f.logger)
	default:
		return nil, fmt.Errorf("provider %s does not support email interface", provider)
	}
}

func (f *ProviderFactory) BuildSMSProvider(provider core.ProviderType, creds map[string]string) (notification.SMSProvider, error) {
	switch provider {
	case core.ProviderTwilio:
		return adapters.NewTwilioAdapter(creds, f.logger)
	default:
		return nil, fmt.Errorf("provider %s does not support sms interface", provider)
	}
}

func (f *ProviderFactory) BuildPushProvider(provider core.ProviderType, creds map[string]string) (notification.PushProvider, error) {
	switch provider {
	case core.ProviderFCM:
		return adapters.NewFCMAdapter(creds, f.logger)
	default:
		return nil, fmt.Errorf("provider %s does not support push interface", provider)
	}
}

func (f *ProviderFactory) BuildWebPushProvider(provider core.ProviderType, creds map[string]string) (notification.WebPushProvider, error) {
	switch provider {
	case core.ProviderVAPID:
		return adapters.NewVapidAdapter(creds, f.logger)
	default:
		return nil, fmt.Errorf("provider %s does not support web push interface", provider)
	}
}

func (f *ProviderFactory) GetWebhookParser(integ *integration.Integration, creds map[string]string) (integration.WebhookParser, error) {
	switch integ.Provider {
	case core.ProviderStripe:
		return adapters.NewStripeAdapter(creds, f.logger)
	case core.ProviderBrevo:
		return adapters.NewBrevoAdapter(creds, f.logger)
	case core.ProviderTwilio:
		return adapters.NewTwilioAdapter(creds, f.logger)
	case core.ProviderSES:
		return adapters.NewSESAdapter(creds, f.logger)
	case core.ProviderSendGrid:
		return adapters.NewSendGridAdapter(creds, f.logger)
	default:
		return nil, fmt.Errorf("provider %s does not support webhooks", integ.Provider)
	}
}

// Helper to reuse decryption logic
func (f *ProviderFactory) decryptCredentials(ctx context.Context, integ *integration.Integration) (map[string]string, error) {
	decryptedBytes, err := f.cryptoSvc.Decrypt(ctx, integ.CredentialsEncrypted, integ.EncryptionContext)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt integration credentials: %w", err)
	}
	var creds map[string]string
	if err := json.Unmarshal(decryptedBytes, &creds); err != nil {
		return nil, fmt.Errorf("invalid credential format: %w", err)
	}
	return creds, nil
}
