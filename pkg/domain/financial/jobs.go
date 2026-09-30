// internal/domain/financial/jobs.go
package financial

import "github.com/combi/pkg/domain/core"

const (
	// JobFinancialProcessWebhook processes incoming webhooks from payment gateways (e.g., Stripe).
	JobFinancialProcessWebhook core.JobType = "financial:process_webhook"
	// JobFinancialProcessPayouts handles payouts to vendors or partners.
	JobFinancialProcessPayouts core.JobType = "financial:process_payouts"
	// JobFinancialRenewSubscriptions processes due subscription renewals.
	JobFinancialRenewSubscriptions core.JobType = "financial:renew_subscriptions"
	JobRefundTransaction           core.JobType = "financial:refund_transaction"
)
