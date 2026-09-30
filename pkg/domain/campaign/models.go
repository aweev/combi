package campaign

import (
	"time"

	"github.com/google/uuid"
)

type CampaignStatus string

const (
	StatusDraft     CampaignStatus = "DRAFT"
	StatusActive    CampaignStatus = "ACTIVE"
	StatusPaused    CampaignStatus = "PAUSED"
	StatusCompleted CampaignStatus = "COMPLETED" // Goal reached or time expired
	StatusArchived  CampaignStatus = "ARCHIVED"
)

// Campaign represents a major fundraising initiative (e.g., "Winter Appeal 2024").
type Campaign struct {
	ID             uuid.UUID      `json:"id" db:"id"`
	OrganizationID uuid.UUID      `json:"organization_id" db:"organization_id"`
	Slug           string         `json:"slug" db:"slug"` // SEO friendly URL part
	Name           string         `json:"name" db:"name"`
	Description    string         `json:"description" db:"description"` // Rich text / HTML
	GoalAmount     int64          `json:"goal_amount" db:"goal_amount"` // In cents
	Currency       string         `json:"currency" db:"currency"`
	Status         CampaignStatus `json:"status" db:"status"`
	ImageURL       string         `json:"image_url" db:"image_url"`

	// Configuration
	AllowP2P   bool `json:"allow_p2p" db:"allow_p2p"` // Can users create sub-pages?
	IsFeatured bool `json:"is_featured" db:"is_featured"`

	// Stats (Denormalized for read performance)
	TotalRaised int64 `json:"total_raised" db:"total_raised"`
	DonorCount  int   `json:"donor_count" db:"donor_count"`

	StartDate *time.Time `json:"start_date" db:"start_date"`
	EndDate   *time.Time `json:"end_date" db:"end_date"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at"`
}

// Fundraiser represents a Peer-to-Peer page created by a user/volunteer linked to a Campaign.
// Example: "John's Birthday Fundraiser for Winter Appeal".
type Fundraiser struct {
	ID             uuid.UUID `json:"id" db:"id"`
	OrganizationID uuid.UUID `json:"organization_id" db:"organization_id"`
	CampaignID     uuid.UUID `json:"campaign_id" db:"campaign_id"`
	OwnerID        uuid.UUID `json:"owner_id" db:"owner_id"` // The User ID of the volunteer

	Slug        string `json:"slug" db:"slug"`
	Name        string `json:"name" db:"name"`
	Description string `json:"description" db:"description"` // Personal story
	GoalAmount  int64  `json:"goal_amount" db:"goal_amount"`
	ImageURL    string `json:"image_url" db:"image_url"`

	// Stats
	TotalRaised int64 `json:"total_raised" db:"total_raised"`
	DonorCount  int   `json:"donor_count" db:"donor_count"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// DonationAttribution links a financial transaction to a campaign/fundraiser.
// This is the bridge between Financial Domain and Campaign Domain.
type DonationAttribution struct {
	TransactionID  uuid.UUID  `db:"transaction_id"`
	OrganizationID uuid.UUID  `db:"organization_id"`
	CampaignID     uuid.UUID  `db:"campaign_id"`
	FundraiserID   *uuid.UUID `db:"fundraiser_id"` // Optional
	Amount         int64      `db:"amount"`
	CreatedAt      time.Time  `db:"created_at"`
}
