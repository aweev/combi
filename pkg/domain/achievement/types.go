package achievement

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/apperror"
	"github.com/google/uuid"
)

var (
	// 400 Bad Request
	ErrInvalidRuleCondition = apperror.New(http.StatusBadRequest, "INVALID_RULE_CONDITION", "The rule logic is malformed.", nil)
	ErrAlreadyAwarded       = apperror.New(http.StatusBadRequest, "ALREADY_AWARDED", "User already has this badge.", nil)

	// 404 Not Found
	ErrBadgeNotFound = apperror.New(http.StatusNotFound, "BADGE_NOT_FOUND", "The specified badge could not be found.", nil)
)

const (
	// PermAchievementManage allows creating badges and rules.
	PermAchievementManage core.PermissionName = "achievement:manage"

	// PermAchievementAward allows staff to manually grant badges.
	PermAchievementAward core.PermissionName = "achievement:award"

	// Emitted when a user earns a badge.
	// Consumers: NotificationService (send "Congrats!" email), RealtimeService (show confetti).
	EventBadgeAwarded core.EventAction = "achievement.badge.awarded"
)

type BadgeType string

const (
	BadgeTypeMilestone BadgeType = "MILESTONE" // e.g. "100 Hours"
	BadgeTypeEvent     BadgeType = "EVENT"     // e.g. "Attended Gala"
	BadgeTypeSkill     BadgeType = "SKILL"     // e.g. "Certified Mentor"
	BadgeTypeManual    BadgeType = "MANUAL"    // Manual award
)

type RuleType string

const (
	// RuleTypeOccurrence awards a badge immediately when a specific event occurs.
	// Condition: Event Metadata matches specific criteria.
	RuleTypeOccurrence RuleType = "OCCURRENCE"

	// RuleTypeAccumulation awards a badge when a tracked metric crosses a threshold.
	// Condition: Metric >= Threshold.
	RuleTypeAccumulation RuleType = "ACCUMULATION"
)

// BadgeAwardedPayload is emitted when a user earns a badge.
// Used by: Notification Service (to send congrats email), Realtime Service (toast).
type BadgeAwardedPayload struct {
	UserID    uuid.UUID `json:"user_id"`
	BadgeID   uuid.UUID `json:"badge_id"`
	BadgeName string    `json:"badge_name"`
	BadgeSlug string    `json:"badge_slug"`
	ImageURL  string    `json:"image_url,omitempty"`
	Reason    string    `json:"reason"`
	AwardedAt time.Time `json:"awarded_at"`
}

// --- Persistence Models ---

type Badge struct {
	ID          uuid.UUID `db:"id"`
	Slug        string    `db:"slug"` // Unique string ID for code ref (e.g. "volunteer-100")
	Name        string    `db:"name"`
	Description string    `db:"description"`
	ImageURL    string    `db:"image_url"`
	Type        BadgeType `db:"type"`
	Points      int       `db:"points"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

type AchievementRule struct {
	ID          uuid.UUID        `db:"id"`
	BadgeID     uuid.UUID        `db:"badge_id"`
	RuleType    RuleType         `db:"rule_type"`
	EventAction core.EventAction `db:"event_action"` // The trigger

	// For Accumulation Rules:
	MetricName *string  `db:"metric_name"` // e.g., "volunteer_hours"
	Threshold  *float64 `db:"threshold"`   // e.g., 100.0

	// For Occurrence Rules (Optional filter):
	// e.g. {"event_id": "uuid-of-gala-2024"}
	MetadataFilter json.RawMessage `db:"metadata_filter"`

	IsActive  bool      `db:"is_active"`
	CreatedAt time.Time `db:"created_at"`
}

type UserBadge struct {
	ID        uuid.UUID  `db:"id"`
	UserID    uuid.UUID  `db:"user_id"`
	BadgeID   uuid.UUID  `db:"badge_id"`
	AwardedAt time.Time  `db:"awarded_at"`
	Reason    string     `db:"reason"`
	RefID     *uuid.UUID `db:"ref_id"` // Link to the specific event/rule execution
}

// UserProgress is a simple key-value store for user metrics.
// Unique Constraint: (user_id, metric_name)
type UserProgress struct {
	UserID     uuid.UUID `db:"user_id"`
	MetricName string    `db:"metric_name"`
	Value      float64   `db:"value"`
	UpdatedAt  time.Time `db:"updated_at"`
}

// --- DTOs (Data Transfer Objects) ---

// UserAchievementStatus combines Badge definition, User's earned status,
// and current progress into one view for the frontend.
type UserAchievementStatus struct {
	Badge     Badge      `json:"badge"`
	IsEarned  bool       `json:"is_earned"`
	AwardedAt *time.Time `json:"awarded_at,omitempty"`

	// Progress is calculated dynamically
	ProgressCurrent float64 `json:"progress_current"`
	ProgressTarget  float64 `json:"progress_target"` // 0 if not accumulation
	ProgressPercent float64 `json:"progress_percent"`
}

type LeaderboardEntry struct {
	Rank        int       `json:"rank" db:"rank"`
	UserID      uuid.UUID `json:"user_id" db:"user_id"`
	DisplayName string    `json:"display_name" db:"display_name"` // From User Service
	AvatarURL   string    `json:"avatar_url" db:"avatar_url"`     // From User Service
	TotalPoints int       `json:"total_points" db:"total_points"`
}

// Service defines the business logic.
type Service interface {
	// Processing
	// ProcessEvent analyzes an incoming event, updates metrics, and checks rules.
	ProcessEvent(ctx context.Context, userID uuid.UUID, action core.EventAction, payload map[string]any) error

	// Administration
	CreateBadge(ctx context.Context, badge Badge) (*Badge, error)
	CreateRule(ctx context.Context, rule AchievementRule) (*AchievementRule, error)
	AwardBadgeManual(ctx context.Context, userID, badgeID uuid.UUID, reason string, adminID uuid.UUID) error

	// User Views
	// GetUserAchievements returns all badges with progress status for a user.
	GetUserAchievements(ctx context.Context, userID uuid.UUID) ([]UserAchievementStatus, error)
	ListBadges(ctx context.Context) ([]Badge, error)

	// GetLeaderboard returns the top N users.
	GetLeaderboard(ctx context.Context, limit int, period string) ([]LeaderboardEntry, error)
}

// Repository handles persistence.
type Repository interface {
	// Badge & Rules
	CreateBadge(ctx context.Context, b *Badge) error
	GetBadge(ctx context.Context, id uuid.UUID) (*Badge, error)
	ListBadges(ctx context.Context) ([]Badge, error)

	CreateRule(ctx context.Context, r *AchievementRule) error
	GetRulesByTrigger(ctx context.Context, action core.EventAction) ([]AchievementRule, error)
	// GetRulesByBadge fetches rules to determine targets for progress display
	GetRulesByBadge(ctx context.Context, badgeID uuid.UUID) ([]AchievementRule, error)

	AwardBadge(ctx context.Context, userID, badgeID uuid.UUID) error
	// User Data
	// AwardBadge inserts the record. Returns error if already exists (idempotency).
	GetUserBadges(ctx context.Context, userID uuid.UUID) ([]UserBadge, error)

	// Metrics
	// IncrementMetric atomically adds delta to value. Returns the NEW total.
	// UPSERT logic: If row missing, insert delta. If exists, update value = value + delta.
	IncrementMetric(ctx context.Context, userID uuid.UUID, metric string, delta float64) (float64, error)

	// GetUserMetrics returns map[metric_name]value
	GetUserMetrics(ctx context.Context, userID uuid.UUID) (map[string]float64, error)

	// Leaderboard
	// Calculates ranking based on Sum(Badge.Points).
	GetGlobalLeaderboard(ctx context.Context, limit int) ([]LeaderboardEntry, error)
}
