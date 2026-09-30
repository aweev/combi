package consent

import (
	"context"
	"net/http"
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/apperror"
	"github.com/google/uuid"
)

var (
	// 400 Bad Request
	ErrRevisionInactive = apperror.New(http.StatusBadRequest, "REVISION_INACTIVE", "Cannot consent to an inactive or outdated revision.", nil)
	ErrAlreadyConsented = apperror.New(http.StatusBadRequest, "ALREADY_CONSENTED", "User has already consented to this revision.", nil)
	ErrMandatoryRevoke  = apperror.New(http.StatusBadRequest, "MANDATORY_REVOKE", "Cannot revoke a mandatory consent while account is active. Please delete account instead.", nil)

	// 404 Not Found
	ErrAgreementNotFound = apperror.New(http.StatusNotFound, "AGREEMENT_NOT_FOUND", "The specified agreement could not be found.", nil)
	ErrRevisionNotFound  = apperror.New(http.StatusNotFound, "REVISION_NOT_FOUND", "The specified revision could not be found.", nil)
)

const (

	// JobConsentAuditExport generates a CSV of all consents for a legal audit.
	JobConsentAuditExport core.JobType = "consent:audit_export"

	// PermConsentManage allows creating agreements and publishing revisions.
	PermConsentManage core.PermissionName = "consent:manage"

	// PermConsentViewLogs allows viewing the history of user consents (for audits).
	PermConsentViewLogs core.PermissionName = "consent:view_logs"
)

type ConsentType string

const (
	TypeTermsOfService ConsentType = "TERMS_OF_SERVICE"
	TypePrivacyPolicy  ConsentType = "PRIVACY_POLICY"
	TypeMarketing      ConsentType = "MARKETING"     // e.g., Newsletter
	TypeDataSharing    ConsentType = "DATA_SHARING"  // e.g., Share profile with partners
	TypePhotoRelease   ConsentType = "PHOTO_RELEASE" // Specific to NGO events
	TypeCookieTracking ConsentType = "COOKIE_TRACKING"
)

type ConsentStatus string

const (
	StatusActive  ConsentStatus = "ACTIVE"  // Current agreement
	StatusRevoked ConsentStatus = "REVOKED" // User withdrew consent
	StatusExpired ConsentStatus = "EXPIRED" // New version available, re-consent needed
)

// --- Core Models ---

// Agreement defines a type of legal document or permission request.
type Agreement struct {
	ID          uuid.UUID   `json:"id" db:"id"`
	Slug        string      `json:"slug" db:"slug"` // e.g., "global-tos", "event-photo-release"
	Type        ConsentType `json:"type" db:"type"`
	Title       string      `json:"title" db:"title"`
	Description string      `json:"description" db:"description"`
	IsMandatory bool        `json:"is_mandatory" db:"is_mandatory"` // Block app usage if false?
	CreatedAt   time.Time   `json:"created_at" db:"created_at"`
}

// Revision represents a specific version of an Agreement.
type Revision struct {
	ID          uuid.UUID `json:"id" db:"id"`
	AgreementID uuid.UUID `json:"agreement_id" db:"agreement_id"`
	Version     string    `json:"version" db:"version"`           // e.g., "1.0", "2023-10-01"
	ContentURL  string    `json:"content_url" db:"content_url"`   // Link to the PDF/HTML text
	ContentHash string    `json:"content_hash" db:"content_hash"` // SHA256 of the text (for audit)
	PublishedAt time.Time `json:"published_at" db:"published_at"`
	IsActive    bool      `json:"is_active" db:"is_active"`
	Summary     string    `json:"summary" db:"summary"`
}

// UserConsent tracks a user's response to a specific Revision.
type UserConsent struct {
	ID         uuid.UUID     `json:"id" db:"id"`
	UserID     uuid.UUID     `json:"user_id" db:"user_id"`
	RevisionID uuid.UUID     `json:"revision_id" db:"revision_id"`
	Status     ConsentStatus `json:"status" db:"status"`
	IPAddress  string        `json:"ip_address" db:"ip_address"` // Audit requirement
	UserAgent  string        `json:"user_agent" db:"user_agent"` // Audit requirement
	GrantedAt  time.Time     `json:"granted_at" db:"granted_at"`
	RevokedAt  *time.Time    `json:"revoked_at,omitempty" db:"revoked_at"`
}

// ComplianceStatus summarizes a user's standing.
type ComplianceStatus struct {
	UserID          uuid.UUID            `json:"user_id"`
	IsCompliant     bool                 `json:"is_compliant"` // True if all MANDATORY active revisions are agreed to
	MissingConsents []Agreement          `json:"missing_consents"`
	ActiveConsents  []UserConsentSummary `json:"active_consents"`
}

type UserConsentSummary struct {
	Slug      string    `json:"slug"`
	Version   string    `json:"version"`
	GrantedAt time.Time `json:"granted_at"`
}

// Service is the primary interface for checking and granting consent.
type Service interface {
	// User Actions
	GrantConsent(ctx context.Context, userID uuid.UUID, revisionID uuid.UUID, ip, ua string) error
	RevokeConsent(ctx context.Context, userID uuid.UUID, agreementID uuid.UUID) error
	CheckCompliance(ctx context.Context, userID uuid.UUID) (*ComplianceStatus, error)

	// Internal Checks (used by other services)
	HasConsented(ctx context.Context, userID uuid.UUID, slug string) (bool, error)

	// Admin Management
	CreateAgreement(ctx context.Context, agreement Agreement) (*Agreement, error)
	PublishRevision(ctx context.Context, revision Revision) (*Revision, error)
	GetLatestRevision(ctx context.Context, slug string) (*Revision, error)

	GetAgreement(ctx context.Context, slug string) (*Agreement, error)
}

// Repository defines the persistence contract.
type Repository interface {
	AnonymizeUserConsents(ctx context.Context, userID uuid.UUID) (int64, error)
	// Agreements & Revisions
	CreateAgreement(ctx context.Context, a *Agreement) error
	GetAgreementBySlug(ctx context.Context, slug string) (*Agreement, error)
	CreateRevision(ctx context.Context, r *Revision) error
	GetRevision(ctx context.Context, id uuid.UUID) (*Revision, error)
	GetLatestActiveRevision(ctx context.Context, agreementID uuid.UUID) (*Revision, error)

	// User Consents
	CreateUserConsent(ctx context.Context, uc *UserConsent) error
	RevokeUserConsent(ctx context.Context, userID, agreementID uuid.UUID, revokedAt time.Time) error
	GetUserConsent(ctx context.Context, userID, revisionID uuid.UUID) (*UserConsent, error)

	// Compliance Queries
	// Returns the slugs of mandatory agreements that the user has NOT signed the latest active version of.
	GetMissingMandatoryConsents(ctx context.Context, userID uuid.UUID) ([]Agreement, error)
	// Checks if a valid, active consent exists for a specific agreement slug
	HasActiveConsent(ctx context.Context, userID uuid.UUID, slug string) (bool, error)
}

// ConsentGrantedPayload (for "consent.granted")
type ConsentGrantedPayload struct {
	UserID      uuid.UUID `json:"user_id"`
	AgreementID uuid.UUID `json:"agreement_id"`
	RevisionID  uuid.UUID `json:"revision_id"`
	IPAddress   string    `json:"ip_address"` // Logged for audit, scrubbed later if needed
}

// ConsentRevokedPayload (for "consent.revoked")
type ConsentRevokedPayload struct {
	UserID      uuid.UUID `json:"user_id"`
	AgreementID uuid.UUID `json:"agreement_id"`
}

// RevisionPublishedPayload (for "consent.revision.published")
// Used by: Notification (Email: "Our Terms have changed")
type RevisionPublishedPayload struct {
	AgreementID uuid.UUID `json:"agreement_id"`
	RevisionID  uuid.UUID `json:"revision_id"`
	Version     string    `json:"version"`
	Summary     string    `json:"summary"`
}
