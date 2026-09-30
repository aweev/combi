package achievement_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/combi/pkg/config"
	"github.com/combi/pkg/domain/achievement"
	achievement_mocks "github.com/combi/pkg/domain/achievement/mocks"
	cache_mocks "github.com/combi/pkg/cache/mocks"
	events_mocks "github.com/combi/pkg/events/mocks"
	"github.com/combi/pkg/log"
	achievement_svc "github.com/combi/achievement"
)

func TestService_ProcessEvent(t *testing.T) {
	repo := achievement_mocks.NewMockRepository(t)
	bus := events_mocks.NewMockBus(t)
	cache := cache_mocks.NewMockService(t)
	svc := achievement_svc.NewService(repo, nil, bus, cache, config.AchievementConfig{}, &log.NoOpLogger{})
	ctx := context.Background()

	userID := uuid.New()
	badgeID := uuid.New()

	t.Run("Occurrence Rule - Match", func(t *testing.T) {
		// Rule: Event "video_watched" with "category": "education"
		filterJSON, _ := json.Marshal(map[string]any{"category": "education"})
		rule := achievement.AchievementRule{
			ID:             uuid.New(),
			BadgeID:        badgeID,
			RuleType:       achievement.RuleTypeOccurrence,
			EventAction:    "video_watched",
			MetadataFilter: filterJSON,
			IsActive:       true,
		}

		repo.EXPECT().GetRulesByTrigger(ctx, rule.EventAction).Return([]achievement.AchievementRule{rule}, nil)
		repo.EXPECT().AwardBadge(ctx, mock.Anything, mock.Anything).Return(nil)
		repo.EXPECT().GetBadge(ctx, badgeID).Return(&achievement.Badge{Name: "Learner"}, nil)
		bus.EXPECT().Publish(ctx, mock.Anything).Return(nil)

		err := svc.ProcessEvent(ctx, userID, "video_watched", map[string]any{"category": "education"})
		require.NoError(t, err)
	})

	t.Run("Accumulation Rule - Increment", func(t *testing.T) {
		metric := "hours"
		threshold := 10.0
		rule := achievement.AchievementRule{
			ID:          uuid.New(),
			BadgeID:     badgeID,
			RuleType:    achievement.RuleTypeAccumulation,
			EventAction: "log_hours",
			MetricName:  &metric,
			Threshold:   &threshold,
			IsActive:    true,
		}

		repo.EXPECT().GetRulesByTrigger(ctx, rule.EventAction).Return([]achievement.AchievementRule{rule}, nil)

		// 1. Increment returns 5 (Total) -> No Badge
		repo.EXPECT().IncrementMetric(ctx, userID, "hours", 2.0).Return(5.0, nil)
		// AwardBadge is NOT called

		err := svc.ProcessEvent(ctx, userID, "log_hours", map[string]any{"amount": 2})
		require.NoError(t, err)
	})

	t.Run("Accumulation Rule - Threshold Met", func(t *testing.T) {
		metric := "hours"
		threshold := 10.0
		rule := achievement.AchievementRule{
			ID:          uuid.New(),
			BadgeID:     badgeID,
			RuleType:    achievement.RuleTypeAccumulation,
			EventAction: "log_hours",
			MetricName:  &metric,
			Threshold:   &threshold,
			IsActive:    true,
		}

		repo.EXPECT().GetRulesByTrigger(ctx, rule.EventAction).Return([]achievement.AchievementRule{rule}, nil)

		// 2. Increment returns 10 (Total) -> Award Badge
		repo.EXPECT().IncrementMetric(ctx, userID, "hours", 5.0).Return(10.0, nil)
		repo.EXPECT().AwardBadge(ctx, mock.Anything, mock.Anything).Return(nil)
		repo.EXPECT().GetBadge(ctx, badgeID).Return(&achievement.Badge{Name: "Expert"}, nil)
		bus.EXPECT().Publish(ctx, mock.Anything).Return(nil)

		err := svc.ProcessEvent(ctx, userID, "log_hours", map[string]any{"amount": 5})
		require.NoError(t, err)
	})
}
