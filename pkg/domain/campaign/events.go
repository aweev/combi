package campaign

import "github.com/google/uuid"

type CampaignCreatedPayload struct {
	CampaignID uuid.UUID `json:"campaign_id"`
	Name       string    `json:"name"`
	GoalAmount int64     `json:"goal_amount"`
	Currency   string    `json:"currency"`
}

type FundraiserCreatedPayload struct {
	FundraiserID uuid.UUID `json:"fundraiser_id"`
	CampaignID   uuid.UUID `json:"campaign_id"`
	OwnerID      uuid.UUID `json:"owner_id"`
	Name         string    `json:"name"`
	GoalAmount   int64     `json:"goal_amount"`
}
