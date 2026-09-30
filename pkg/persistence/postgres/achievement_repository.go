package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/combi/pkg/domain/achievement"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/log"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// Ensure implementation satisfies interface
var _ achievement.Repository = (*AchievementRepository)(nil)

type AchievementRepository struct {
	db     *sqlx.DB
	logger log.Logger
}

func NewAchievementRepository(db *sqlx.DB, logger log.Logger) *AchievementRepository {
	return &AchievementRepository{
		db:     db,
		logger: logger.With(log.String("component", "achievement_repo")),
	}
}

// --- Badge Management ---

func (r *AchievementRepository) CreateBadge(ctx context.Context, b *achievement.Badge) error {
	query := `
		INSERT INTO badges (id, slug, name, description, image_url, type, points, created_at, updated_at) 
		VALUES (:id, :slug, :name, :description, :image_url, :type, :points, :created_at, :updated_at)
	`
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	now := time.Now().UTC()
	if b.CreatedAt.IsZero() {
		b.CreatedAt = now
	}
	b.UpdatedAt = now

	_, err := sqlx.NamedExecContext(ctx, r.db, query, b)
	if err != nil {
		return fmt.Errorf("failed to create badge: %w", err)
	}
	return nil
}

func (r *AchievementRepository) GetBadge(ctx context.Context, id uuid.UUID) (*achievement.Badge, error) {
	var b achievement.Badge
	query := `SELECT * FROM badges WHERE id = $1`
	if err := r.db.GetContext(ctx, &b, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, achievement.ErrBadgeNotFound
		}
		return nil, fmt.Errorf("failed to get badge: %w", err)
	}
	return &b, nil
}

func (r *AchievementRepository) ListBadges(ctx context.Context) ([]achievement.Badge, error) {
	var badges []achievement.Badge
	query := `SELECT * FROM badges ORDER BY points ASC, name ASC`
	if err := r.db.SelectContext(ctx, &badges, query); err != nil {
		return nil, fmt.Errorf("failed to list badges: %w", err)
	}
	return badges, nil
}

// --- Rule Management ---

func (r *AchievementRepository) CreateRule(ctx context.Context, rule *achievement.AchievementRule) error {
	query := `
		INSERT INTO achievement_rules 
		(id, badge_id, rule_type, event_action, metric_name, threshold, metadata_filter, is_active, created_at)
		VALUES 
		(:id, :badge_id, :rule_type, :event_action, :metric_name, :threshold, :metadata_filter, :is_active, NOW())
	`
	if rule.ID == uuid.Nil {
		rule.ID = uuid.New()
	}

	if len(rule.MetadataFilter) == 0 {
		rule.MetadataFilter = json.RawMessage("{}")
	}

	_, err := sqlx.NamedExecContext(ctx, r.db, query, rule)
	if err != nil {
		return fmt.Errorf("failed to create rule: %w", err)
	}
	return nil
}

func (r *AchievementRepository) GetRulesByTrigger(ctx context.Context, action core.EventAction) ([]achievement.AchievementRule, error) {
	var rules []achievement.AchievementRule
	query := `SELECT * FROM achievement_rules WHERE event_action = $1 AND is_active = TRUE`
	if err := r.db.SelectContext(ctx, &rules, query, action); err != nil {
		return nil, fmt.Errorf("failed to get rules by trigger: %w", err)
	}
	return rules, nil
}

func (r *AchievementRepository) GetRulesByBadge(ctx context.Context, badgeID uuid.UUID) ([]achievement.AchievementRule, error) {
	var rules []achievement.AchievementRule
	query := `SELECT * FROM achievement_rules WHERE badge_id = $1 AND is_active = TRUE`
	if err := r.db.SelectContext(ctx, &rules, query, badgeID); err != nil {
		return nil, fmt.Errorf("failed to get rules by badge: %w", err)
	}
	return rules, nil
}

// --- User Data ---

func (r *AchievementRepository) GetUserBadges(ctx context.Context, userID uuid.UUID) ([]achievement.UserBadge, error) {
	var badges []achievement.UserBadge
	query := `SELECT * FROM user_badges WHERE user_id = $1 ORDER BY awarded_at DESC`
	if err := r.db.SelectContext(ctx, &badges, query, userID); err != nil {
		return nil, fmt.Errorf("failed to get user badges: %w", err)
	}
	return badges, nil
}

// --- Metrics (Atomic Operations) ---

func (r *AchievementRepository) IncrementMetric(ctx context.Context, userID uuid.UUID, metric string, delta float64) (float64, error) {
	query := `
		INSERT INTO user_progress (user_id, metric_name, value, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (user_id, metric_name) 
		DO UPDATE SET 
			value = user_progress.value + EXCLUDED.value,
			updated_at = NOW()
		RETURNING value
	`

	var newValue float64
	if err := r.db.QueryRowContext(ctx, query, userID, metric, delta).Scan(&newValue); err != nil {
		return 0, fmt.Errorf("failed to increment metric: %w", err)
	}

	return newValue, nil
}

func (r *AchievementRepository) GetUserMetrics(ctx context.Context, userID uuid.UUID) (map[string]float64, error) {
	var rows []achievement.UserProgress
	query := `SELECT * FROM user_progress WHERE user_id = $1`

	if err := r.db.SelectContext(ctx, &rows, query, userID); err != nil {
		return nil, fmt.Errorf("failed to get metrics: %w", err)
	}

	result := make(map[string]float64)
	for _, row := range rows {
		result[row.MetricName] = row.Value
	}
	return result, nil
}

// --- Leaderboard ---

func (r *AchievementRepository) GetGlobalLeaderboard(ctx context.Context, limit int) ([]achievement.LeaderboardEntry, error) {
	query := `
		WITH user_points AS (
			SELECT 
				ub.user_id, 
				COALESCE(SUM(b.points), 0) as total_points
			FROM user_badges ub
			JOIN badges b ON ub.badge_id = b.id
			GROUP BY ub.user_id
		)
		SELECT 
			user_id,
			total_points,
			DENSE_RANK() OVER (ORDER BY total_points DESC) as rank
		FROM user_points
		ORDER BY rank ASC, user_id ASC
		LIMIT $1
	`

	var entries []achievement.LeaderboardEntry
	if err := r.db.SelectContext(ctx, &entries, query, limit); err != nil {
		return nil, fmt.Errorf("failed to get leaderboard: %w", err)
	}
	return entries, nil
}

func (r *AchievementRepository) AwardBadge(ctx context.Context, userID, badgeID uuid.UUID) error {
	// FIX: Check first to ensure error is returned
	var count int
	r.db.GetContext(ctx, &count, "SELECT COUNT(*) FROM user_badges WHERE user_id=$1 AND badge_id=$2", userID, badgeID)
	if count > 0 {
		return achievement.ErrAlreadyAwarded
	}

	_, err := r.db.ExecContext(ctx, `INSERT INTO user_badges (id, user_id, badge_id, awarded_at) VALUES ($1, $2, $3, NOW())`, uuid.New(), userID, badgeID)
	if err != nil {
		if isUniqueViolation(err) {
			return achievement.ErrAlreadyAwarded
		}
		return err
	}
	return nil
}

func (r *AchievementRepository) GetProgressAndLock(ctx context.Context, userID uuid.UUID, metricName string) (float64, error) {
	query := `
        SELECT value FROM user_progress 
        WHERE user_id = $1 AND metric_name = $2 
        FOR UPDATE
    `
	var val float64
	err := r.db.GetContext(ctx, &val, query, userID, metricName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	return val, nil
}
