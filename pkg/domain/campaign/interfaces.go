package campaign

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Service defines the public API for the Campaign domain.
type Service interface {
	// Public Read
	GetCampaign(ctx context.Context, slugOrID string) (*Campaign, error)
	ListActiveCampaigns(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*Campaign, error)
	GetFundraiser(ctx context.Context, slugOrID string) (*Fundraiser, error)
	ListCampaignFundraisers(ctx context.Context, campaignID uuid.UUID, limit, offset int) ([]*Fundraiser, error)

	// Volunteer/User Actions
	CreateFundraiser(ctx context.Context, params CreateFundraiserParams) (*Fundraiser, error)
	UpdateFundraiser(ctx context.Context, fundraiserID uuid.UUID, params UpdateFundraiserParams) (*Fundraiser, error)

	// Internal / Async Handlers
	// This is called when `financial.EventDonationProcessed` is received.
	RecordDonation(ctx context.Context, transactionID uuid.UUID, campaignID uuid.UUID, fundraiserID *uuid.UUID, amount int64) error
}

// AdminService defines back-office operations.
type AdminService interface {
	CreateCampaign(ctx context.Context, params CreateCampaignParams) (*Campaign, error)
	UpdateCampaign(ctx context.Context, id uuid.UUID, params UpdateCampaignParams) (*Campaign, error)
	DeleteCampaign(ctx context.Context, id uuid.UUID) error // Soft delete usually
}

// Repository defines the persistence layer.
type Repository interface {
	// Campaign
	CreateCampaign(ctx context.Context, c *Campaign) error
	GetCampaignByID(ctx context.Context, id uuid.UUID) (*Campaign, error)
	GetCampaignBySlug(ctx context.Context, slug string) (*Campaign, error)
	UpdateCampaign(ctx context.Context, c *Campaign) error
	ListCampaigns(ctx context.Context, filter CampaignFilter) ([]*Campaign, int, error)

	// Fundraiser
	CreateFundraiser(ctx context.Context, f *Fundraiser) error
	GetFundraiserByID(ctx context.Context, id uuid.UUID) (*Fundraiser, error)
	GetFundraiserBySlug(ctx context.Context, slug string) (*Fundraiser, error)
	UpdateFundraiser(ctx context.Context, f *Fundraiser) error
	ListFundraisers(ctx context.Context, campaignID uuid.UUID, limit, offset int) ([]*Fundraiser, int, error)

	// Stats & Attribution
	CreateAttribution(ctx context.Context, attr *DonationAttribution) error
	IncrementStats(ctx context.Context, campaignID uuid.UUID, fundraiserID *uuid.UUID, amount int64) error
}

// --- DTOs ---

type CreateCampaignParams struct {
	OrganizationID uuid.UUID
	Name           string
	Slug           string
	Description    string
	GoalAmount     int64
	Currency       string
	AllowP2P       bool
	ImageURL       string
	StartDate      *time.Time
	EndDate        *time.Time
}

type UpdateCampaignParams struct {
	Name        *string
	Description *string
	GoalAmount  *int64
	Status      *CampaignStatus
	IsFeatured  *bool
	ImageURL    *string
}

type CreateFundraiserParams struct {
	OrganizationID uuid.UUID
	CampaignID     uuid.UUID
	OwnerID        uuid.UUID
	Name           string
	Slug           string
	Description    string
	GoalAmount     int64
	ImageURL       string
}

type UpdateFundraiserParams struct {
	Name        *string
	Description *string
	GoalAmount  *int64
	ImageURL    *string
}

type CampaignFilter struct {
	OrganizationID uuid.UUID
	Status         *CampaignStatus
	Limit          int
	Offset         int
}
