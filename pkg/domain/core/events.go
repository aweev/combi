package core

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

// EventAction is the canonical key for events.
type EventAction string

func (e EventAction) String() string { return string(e) }

const (
	// --- Auth Domain ---
	EventUserLoggedIn              EventAction = "auth.user.loggedin"
	EventUserLoggedOut             EventAction = "auth.user.loggedout"
	EventPasswordChanged           EventAction = "auth.password.changed"
	EventIdentityVerified          EventAction = "auth.identity.verified"
	EventAccountRegistered         EventAction = "auth.account.registered"
	EventAccountCreated            EventAction = "auth.account.created" // Often synonymous, depends on workflow
	EventMagicLinkRequested        EventAction = "auth.magic_link.requested"
	EventPasswordResetReq          EventAction = "auth.password.reset_requested"
	EventPasswordReset             EventAction = "auth.password.reset_completed" // The actual change
	EventAdminImpersonationStarted EventAction = "auth.admin.impersonation_started"
	EventAdminAccountStatusUpdated EventAction = "auth.admin.status_updated"
	EventAdminAccountEmailUpdated  EventAction = "auth.admin.email_updated"
	EventAdminAllSessionsRevoked   EventAction = "auth.admin.sessions_revoked"
	EventSessionRevoked            EventAction = "auth.session.revoked"
	EventLogout                    EventAction = "auth.logout"
	EventTokenRefreshed            EventAction = "auth.token.refreshed"
	EventEmailVerified             EventAction = "auth.email.verified"
	EventMfaEnabled                EventAction = "auth.mfa.enabled"
	EventMfaDisabled               EventAction = "auth.mfa.disabled"
	EventLoginSuccess              EventAction = "auth.login.success"
	EventLoginFailed               EventAction = "auth.login.failed"
	EventLoginMfaRequired          EventAction = "auth.login.mfa_required"
	EventAccountSuspended          EventAction = "auth.account.suspended"

	// --- Audit/Privacy Domain ---
	EventLegalHoldApplied       EventAction = "audit.legal_hold.applied"
	EventLegalHoldReleased      EventAction = "audit.legal_hold.released"
	EventRetentionPolicyCreated EventAction = "audit.retention_policy.created"
	EventRetentionPolicyUpdated EventAction = "audit.retention_policy.updated"
	EventRetentionPolicyDeleted EventAction = "audit.retention_policy.deleted"
	EventAnonymizationRequested EventAction = "audit.anonymization.requested"
	EventAnonymizationCompleted EventAction = "audit.anonymization.completed"
	EventAuditExportRequested   EventAction = "audit.export.requested"

	// --- Automation Domain ---
	EventRuleCreated      EventAction = "automation.rule.created"
	EventRuleUpdated      EventAction = "automation.rule.updated"
	EventRuleDeleted      EventAction = "automation.rule.deleted"
	EventRuleTriggered    EventAction = "automation.rule.triggered"
	EventExecutionFailure EventAction = "automation.execution.failure" // Internal event for workers
	EventAutomationRetry  EventAction = "automation.retry"

	// --- Campaign Domain ---
	EventCampaignCreated   EventAction = "campaign.created"
	EventCampaignUpdated   EventAction = "campaign.updated"
	EventCampaignCompleted EventAction = "campaign.completed" // Goal met!
	EventFundraiserCreated EventAction = "campaign.fundraiser.created"
	EventFundraiserGoalMet EventAction = "campaign.fundraiser.goal_met"

	// --- Catalog Domain ---
	EventItemCreated              EventAction = "catalog.item.created"
	EventItemUpdated              EventAction = "catalog.item.updated"
	EventItemDeleted              EventAction = "catalog.item.deleted"
	EventApplicationSubmitted     EventAction = "catalog.application.submitted"
	EventApplicationStatusUpdated EventAction = "catalog.application.status_updated"

	// --- Configuration Domain ---
	EventSettingUpdated EventAction = "config.setting.updated"
	EventBrandUpdated   EventAction = "config.brand.updated"
	EventThemeUpdated   EventAction = "config.theme.updated"

	// --- Consent Domain ---
	EventConsentGranted    EventAction = "consent.granted"
	EventConsentRevoked    EventAction = "consent.revoked"
	EventRevisionPublished EventAction = "consent.revision.published"

	// --- User Domain ---
	EventUserCreated           EventAction = "user.created"
	EventUserUpdated           EventAction = "user.updated"
	EventUserDeleted           EventAction = "user.deleted"
	EventUserProfileUpdated    EventAction = "user.profile.updated"
	EventUserAvatarUpdated     EventAction = "user.avatar.updated"
	EventUserConnectionCreated EventAction = "user.connection.created"
	EventUserStatusUpdated     EventAction = "user.status.updated"
	EventUserConnectionDeleted EventAction = "user.connection.deleted"

	// --- Financial Domain ---
	EventPaymentSucceeded      EventAction = "financial.payment.succeeded"
	EventPaymentFailed         EventAction = "financial.payment.failed"
	EventDonationReceived      EventAction = "financial.donation.received" // High-level semantic event
	EventSubscriptionCreated   EventAction = "financial.subscription.created"
	EventSubscriptionRenewed   EventAction = "financial.subscription.renewed"
	EventSubscriptionCancelled EventAction = "financial.subscription.cancelled"
	EventSubscriptionFailed    EventAction = "financial.subscription.failed" // Initial creation failure or renewal failure

	// Invoices
	EventInvoicePaid   EventAction = "financial.invoice.paid"
	EventInvoiceVoided EventAction = "financial.invoice.voided"

	// Transactions & Admin
	EventTransactionRefunded EventAction = "financial.transaction.refunded"
	EventGrantDisbursed      EventAction = "financial.grant.disbursed" // Payouts

	// Payment Methods (Cards/Wallets)
	EventPaymentMethodAdded   EventAction = "financial.payment_method.added"
	EventPaymentMethodRemoved EventAction = "financial.payment_method.removed"

	// --- Content Domain (CMS) ---
	EventContentPublished       EventAction = "content.published"
	EventContentUnpublished     EventAction = "content.unpublished"
	EventContentUpdated         EventAction = "content.updated"
	EventContentArchived        EventAction = "content.archived"
	EventContentDeleted         EventAction = "content.deleted"
	EventContentVersionRestored EventAction = "content.version.restored"
	EventSpaceCreated           EventAction = "content.space.created"
	EventModelCreated           EventAction = "content.model.created"
	EventContentCreated         EventAction = "content.entry.created"
	EventSpaceUpdated           EventAction = "content.space.updated"
	EventSpaceDeleted           EventAction = "content.space.deleted"
	EventModelUpdated           EventAction = "content.model.updated"
	EventModelDeleted           EventAction = "content.model.deleted"

	// --- Event/Calendar Domain ---
	EventSessionScheduled      EventAction = "event.session.scheduled" // Created/Published
	EventCreated               EventAction = "event.created"           // Was EventSessionScheduled
	EventUpdated               EventAction = "event.updated"
	EventCancelled             EventAction = "event.cancelled"
	EventDeleted               EventAction = "event.deleted"
	EventTicketCreated         EventAction = "event.ticket.created"
	EventTicketUpdated         EventAction = "event.ticket.updated"
	EventScheduleItemCreated   EventAction = "event.schedule.item_created"
	EventScheduleItemUpdated   EventAction = "event.schedule.item_updated"
	EventVenueCreated          EventAction = "event.venue.created"
	EventSpeakerCreated        EventAction = "event.speaker.created"
	EventRegistrationCreated   EventAction = "event.registration.created"   // User Registered
	EventRegistrationWithdrawn EventAction = "event.registration.withdrawn" // User Withdrew
	EventUserCheckedIn         EventAction = "event.attendance.checked_in"  // Physical check-in
	EventSessionEnded          EventAction = "event.session.ended"
	EventAttendeeJoined        EventAction = "event.attendee.joined"

	// --- Feature Flag Domain ---
	EventFlagCreated EventAction = "feature.flag.created"
	EventFlagUpdated EventAction = "feature.flag.updated"
	EventFlagDeleted EventAction = "feature.flag.deleted"

	// --- Form Domain ---
	EventFormSubmission          EventAction = "form.submission"
	EventFormCreated             EventAction = "form.created"
	EventFormUpdated             EventAction = "form.updated"
	EventSubmissionReceived      EventAction = "form.submission.received"
	EventSubmissionStatusChanged EventAction = "form.submission.status_changed"
	EventVoteCast                EventAction = "form.vote.cast"
	EventVoteConcluded           EventAction = "form.vote.concluded"

	// --- Impact Domain ---
	EventImpactMetricUpdated EventAction = "impact.metric.updated"
	EventGoalReached         EventAction = "impact.goal.reached"
	EventGoalCreated         EventAction = "impact.goal.created"

	// --- Integration Domain ---
	EventIntegrationCreated          EventAction = "integration.created"
	EventIntegrationUpdated          EventAction = "integration.updated"
	EventIntegrationDeleted          EventAction = "integration.deleted"
	EventIntegrationConnectionTested EventAction = "integration.connection.tested"
	EventWebhookReceived             EventAction = "integration.webhook.received"

	// --- Localization Domain ---
	EventLocalizationKeyCreated   EventAction = "localization.key.created"
	EventLocalizationValueUpdated EventAction = "localization.value.updated"

	// --- Notification / System ---
	EventEmailBounced EventAction = "notification.email.bounced"
	EventSMSFailed    EventAction = "notification.sms.failed"

	// --- Matchmaking Domain ---
	EventMatchRequestCreated EventAction = "match.request.created" // Was EventMatchRequest
	EventMatchSuggestions    EventAction = "match.suggestions.ready"
	EventMatchProposed       EventAction = "match.proposed"
	EventMatchEstablished    EventAction = "match.established"

	// --- Catalog Domain (Opportunities) ---
	EventOpportunityCreated EventAction = "catalog.opportunity.created"
	EventOpportunityUpdated EventAction = "catalog.opportunity.updated"
	EventOpportunityDeleted EventAction = "catalog.opportunity.deleted"

	// --- Messaging Domain ---
	EventMessageCreated EventAction = "messaging.message.created"
	EventMessageRead    EventAction = "messaging.message.read"
	EventMessageFlagged EventAction = "messaging.message.flagged"

	// --- Notification Domain ---
	EventNotificationSent    EventAction = "notification.sent"
	EventNotificationFailed  EventAction = "notification.failed" // Internal failure
	EventNotificationRetried EventAction = "notification.retried"

	// --- Notification Config (Templates/Providers) ---
	EventTemplateCreated EventAction = "notification.template.created"
	EventTemplateUpdated EventAction = "notification.template.updated"
	EventTemplateDeleted EventAction = "notification.template.deleted"

	EventProviderCreated EventAction = "notification.provider.created"
	EventProviderUpdated EventAction = "notification.provider.updated"
	EventProviderDeleted EventAction = "notification.provider.deleted"

	EventTemplateTranslationAdded EventAction = "notification.template_translation.added"

	// --- Organization Domain ---
	EventOrganizationCreated      EventAction = "organization.created"
	EventOrganizationUpdated      EventAction = "organization.updated"
	EventOrganizationDeleted      EventAction = "organization.deleted"
	EventOrganizationOwnerChanged EventAction = "organization.owner.changed"

	EventMemberInvited     EventAction = "organization.member.invited"
	EventMemberJoined      EventAction = "organization.member.joined"
	EventMemberRemoved     EventAction = "organization.member.removed"
	EventMemberRoleChanged EventAction = "organization.member.role_changed"

	EventInvitationAccepted EventAction = "organization.invitation.accepted"
	EventInvitationDeclined EventAction = "organization.invitation.declined"

	// --- Volunteering Domain ---
	EventHoursLogged      EventAction = "project.hours.logged"
	EventTaskCompleted    EventAction = "project.task.completed"
	EventProjectCompleted EventAction = "project.completed"

	// --- Project Domain ---
	EventProjectCreated       EventAction = "project.created"
	EventProjectUpdated       EventAction = "project.updated"  // Added
	EventProjectArchived      EventAction = "project.archived" // Added
	EventProjectTaskCreated   EventAction = "project.task.created"
	EventProjectTaskUpdated   EventAction = "project.task.updated"
	EventProjectTaskAssigned  EventAction = "project.task.assigned"
	EventProjectTaskCompleted EventAction = "project.task.completed"

	// Member events (distinct from org members)
	EventProjectMemberAdded   EventAction = "project.member.added"
	EventProjectMemberRemoved EventAction = "project.member.removed"

	// --- Realtime Domain ---
	EventUserConnected     EventAction = "realtime.user.connected"
	EventUserDisconnected  EventAction = "realtime.user.disconnected"
	EventChannelSubscribed EventAction = "realtime.channel.subscribed"

	// Video/WebRTC Signaling (Internal routing event, often not stored but passed through)
	EventWebRTCSignal EventAction = "realtime.webrtc.signal"

	// --- Storage Domain ---
	EventFileUploadInitiated EventAction = "storage.file.upload_initiated"
	EventFileUploadCompleted EventAction = "storage.file.upload_completed"
	EventFileDeleted         EventAction = "storage.file.deleted"
	EventFileMetadataUpdated EventAction = "storage.file.metadata_updated"
	EventFileDownloaded      EventAction = "storage.file.downloaded"
	EventFileRestored        EventAction = "storage.file.restored"

	// --- File Processing Events ---
	EventFileProcessingStarted   EventAction = "storage.file.processing_started"
	EventFileProcessingCompleted EventAction = "storage.file.processing_completed"
	EventFileProcessingFailed    EventAction = "storage.file.processing_failed"

	// --- Administrative Events ---
	EventBucketCreated EventAction = "storage.bucket.created"
	EventBucketDeleted EventAction = "storage.bucket.deleted"
	EventBucketUpdated EventAction = "storage.file.updated"

	EventQuotaUpdated EventAction = "storage.quota.updated"

	// --- Gamification Domain ---
	EventBadgeAwarded EventAction = "achievement.badge.awarded"

	// --- Search Domain ---
	// Events for indexing are usually reactions to other domains, but these are for admin actions
	EventSearchIndexRebuilt EventAction = "search.index.rebuilt"

	// --- Workflow Domain ---
	EventWorkflowStarted   EventAction = "workflow.started"
	EventWorkflowCancelled EventAction = "workflow.cancelled"
	EventWorkflowSignaled  EventAction = "workflow.signaled"

	// --- Approval ---
	EventApprovalRequested EventAction = "workflow.approval.requested"
	EventApprovalDecided   EventAction = "workflow.approval.decided"

	// --- Reporting Domain ---
	EventReportRequested EventAction = "report.requested"
	EventReportGenerated EventAction = "report.generated"
	EventReportFailed    EventAction = "report.failed"

	// Template
	EventReportTemplateCreated EventAction = "report.template.created"

	// --- Tagging Domain ---
	EventTagCreated   EventAction = "tag.created"
	EventTagUpdated   EventAction = "tag.updated"
	EventTagDeleted   EventAction = "tag.deleted"
	EventTagsMerged   EventAction = "tag.merged"
	EventEntityTagged EventAction = "tag.entity.tagged"
)

type UserCreatedPayload struct {
	UserID   string `json:"user_id"`
	Email    string `json:"email"`
	FullName string `json:"full_name"`
	Role     string `json:"role"`
}

type DonationReceivedPayload struct {
	TransactionID string  `json:"tx_id"`
	Amount        string  `json:"amount"` // Use string to preserve decimal precision
	Currency      string  `json:"currency"`
	DonorID       string  `json:"donor_id"`
	ProgramID     *string `json:"program_id,omitempty"`
}

type ArticlePublishedPayload struct {
	ArticleID string `json:"article_id"`
	Slug      string `json:"slug"`
	AuthorID  string `json:"author_id"`
}

// JobType is the canonical type for all background job identifiers.
// It is defined here to break potential circular dependencies between the 'queue'
// domain (which owns the Job model) and other core domains (like 'audit')
// that need to define their own background jobs.
type JobType string

func (j JobType) String() string {
	return string(j)
}

type PermissionName string

// BuildPermissionName constructs the canonical permission string from its parts.
func BuildPermissionName(resource, action string) PermissionName {
	return PermissionName(fmt.Sprintf("%s:%s", resource, action))
}

func (p PermissionName) String() string {
	return string(p)
}

// Email is a value object that enforces email format validation.
type Email struct {
	value string
}

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// NewEmail creates a validated email address.
func NewEmail(email string) (Email, error) {
	if email == "" {
		return Email{}, errors.New("email cannot be empty")
	}
	if !emailRegex.MatchString(email) {
		return Email{}, errors.New("invalid email format")
	}
	return Email{value: email}, nil
}

// String returns the email as a string.
func (e Email) String() string {
	return e.value
}

// MarshalJSON implements json.Marshaler.
func (e Email) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"%s"`, e.value)), nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (e *Email) UnmarshalJSON(data []byte) error {
	// Remove quotes
	str := string(data)
	if len(str) >= 2 && str[0] == '"' && str[len(str)-1] == '"' {
		str = str[1 : len(str)-1]
	}

	email, err := NewEmail(str)
	if err != nil {
		return err
	}
	*e = email
	return nil
}

// --- SQL Scanner and Valuer Implementation ---

// Scan implements sql.Scanner for database scanning.
func (e *Email) Scan(value interface{}) error {
	if value == nil {
		return errors.New("cannot scan NULL into Email")
	}

	var str string
	switch v := value.(type) {
	case string:
		str = v
	case []byte:
		str = string(v)
	default:
		return fmt.Errorf("cannot scan type %T into Email", value)
	}

	email, err := NewEmail(str)
	if err != nil {
		return err
	}
	*e = email
	return nil
}

// Value implements driver.Valuer for database storage.
func (e Email) Value() (driver.Value, error) {
	return e.value, nil
}

// MustNewEmail is a test helper to create a Email and fail the test on error.
func MustNewEmail(t *testing.T, emailStr string) Email {
	email, err := NewEmail(emailStr)
	require.NoError(t, err)
	return email
}

// ProviderType is the canonical enum for all external service providers.
type ProviderType string

const (
	ProviderStripe   ProviderType = "STRIPE"
	ProviderBrevo    ProviderType = "BREVO"
	ProviderSendGrid ProviderType = "SENDGRID"
	ProviderTwilio   ProviderType = "TWILIO"
	ProviderFCM      ProviderType = "FCM"
	ProviderVAPID    ProviderType = "VAPID"
	ProviderPayPal   ProviderType = "PAYPAL"
	ProviderSES      ProviderType = "SES"
	ProviderSMTP     ProviderType = "SMTP"
	ProviderLog      ProviderType = "LOG" // For testing
)

// IsPaymentGateway returns true if the provider handles payments.
func (pt ProviderType) IsPaymentGateway() bool {
	return pt == ProviderStripe || pt == ProviderPayPal
}

// IsNotificationProvider returns true if the provider handles messaging.
func (pt ProviderType) IsNotificationProvider() bool {
	return pt == ProviderBrevo || pt == ProviderTwilio || pt == ProviderSES || pt == ProviderSMTP
}

// Entity is the base interface for all domain models.
type Entity interface {
	ID() string
}

// Localizable represents entities that have content in multiple languages.
// This supports your "LocalizationService" requirement later.
type Localizable map[string]string

func (l Localizable) Get(locale string) string {
	if v, ok := l[locale]; ok {
		return v
	}
	if v, ok := l["en"]; ok {
		return v // Fallback to English
	}
	return ""
}
