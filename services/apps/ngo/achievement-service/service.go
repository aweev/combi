package achievement

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/google/uuid"
	"github.com/combi/pkg/config"
	"github.com/combi/pkg/domain/achievement"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/user"
	"github.com/combi/pkg/cache"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)

// Ensure interface compliance
var _ achievement.Service = (*Service)(nil)

type Service struct {
	repo     achievement.Repository
	userSvc  user.Service
	eventBus events.Bus
	cache    cache.Service
	config   config.AchievementConfig
	logger   log.Logger
}

func NewService(
	repo achievement.Repository,
	userSvc user.Service,
	eventBus events.Bus,
	cache cache.Service,
	cfg config.AchievementConfig,
	logger log.Logger,
) *Service {
	return &Service{
		repo:     repo,
		userSvc:  userSvc,
		eventBus: eventBus,
		cache:    cache,
		config:   cfg,
		logger:   logger.With(log.String("service", "achievement")),
	}
}

// --- Rule Engine ---

func (s *Service) ProcessEvent(ctx context.Context, userID uuid.UUID, action core.EventAction, payload map[string]any) error {
	rules, err := s.repo.GetRulesByTrigger(ctx, action)
	if err != nil {
		return fmt.Errorf("failed to fetch rules: %w", err)
	}

	if len(rules) == 0 {
		return nil
	}

	for _, rule := range rules {
		if !rule.IsActive {
			continue
		}

		var processErr error
		switch rule.RuleType {
		case achievement.RuleTypeOccurrence:
			processErr = s.processOccurrenceRule(ctx, userID, rule, payload)
		case achievement.RuleTypeAccumulation:
			processErr = s.processAccumulationRule(ctx, userID, rule, payload)
		}

		if processErr != nil {
			s.logger.Error("failed to process achievement rule", processErr,
				log.String("rule_id", rule.ID.String()),
				log.String("user_id", userID.String()),
			)
		}
	}
	return nil
}

func (s *Service) processOccurrenceRule(ctx context.Context, userID uuid.UUID, rule achievement.AchievementRule, payload map[string]any) error {
	// If filter is empty/nil, it matches everything
	if len(rule.MetadataFilter) == 0 || string(rule.MetadataFilter) == "{}" {
		return s.grantBadge(ctx, userID, rule.BadgeID, "Event: "+string(rule.EventAction))
	}

	// 1. Unmarshal the rule requirements
	var requirements map[string]any
	if err := json.Unmarshal(rule.MetadataFilter, &requirements); err != nil {
		return fmt.Errorf("invalid metadata_filter json in rule %s: %w", rule.ID, err)
	}

	// 2. Check strict subset match
	if matchesFilter(payload, requirements) {
		return s.grantBadge(ctx, userID, rule.BadgeID, "Event: "+string(rule.EventAction))
	}

	return nil
}

func (s *Service) processAccumulationRule(ctx context.Context, userID uuid.UUID, rule achievement.AchievementRule, payload map[string]any) error {
	if rule.MetricName == nil || rule.Threshold == nil {
		return errors.New("accumulation rule missing required metric/threshold fields")
	}

	delta := 1.0
	// Heuristic to find the value in the payload
	if val, ok := payload[*rule.MetricName]; ok {
		if f, ok := toFloat64(val); ok {
			delta = f
		}
	} else if val, ok := payload["amount"]; ok {
		if f, ok := toFloat64(val); ok {
			delta = f
		}
	} else if val, ok := payload["count"]; ok {
		if f, ok := toFloat64(val); ok {
			delta = f
		}
	}

	newTotal, err := s.repo.IncrementMetric(ctx, userID, *rule.MetricName, delta)
	if err != nil {
		return fmt.Errorf("failed to increment metric: %w", err)
	}

	if newTotal >= *rule.Threshold {
		reason := fmt.Sprintf("Threshold met: %s (%.2f >= %.2f)", *rule.MetricName, newTotal, *rule.Threshold)
		return s.grantBadge(ctx, userID, rule.BadgeID, reason)
	}

	return nil
}

func (s *Service) grantBadge(ctx context.Context, userID, badgeID uuid.UUID, reason string) error {
	// 1. Persist
	if err := s.repo.AwardBadge(ctx, userID, badgeID); err != nil {
		if errors.Is(err, achievement.ErrAlreadyAwarded) {
			return nil
		}
		return err
	}

	s.logger.Info("Badge awarded", log.String("user_id", userID.String()), log.String("badge_id", badgeID.String()))

	// 2. Hydrate Details for Event
	badge, err := s.repo.GetBadge(ctx, badgeID)
	if err != nil {
		s.logger.Warn("badge awarded but failed to fetch details for event", log.ErrorAttr(err))
		return nil
	}

	// 3. Publish Event (Using Strong Type)
	payload := achievement.BadgeAwardedPayload{
		UserID:    userID,
		BadgeID:   badgeID,
		BadgeName: badge.Name,
		BadgeSlug: badge.Slug,
		ImageURL:  badge.ImageURL,
		Reason:    reason,
		AwardedAt: time.Now().UTC(),
	}

	evt, err := events.NewEvent(core.EventBadgeAwarded, userID, "user", payload)
	if err != nil {
		return err
	}

	if err := s.eventBus.Publish(ctx, evt); err != nil {
		s.logger.Error("failed to publish badge awarded event", err)
	}

	return nil
}

// --- Admin Methods ---

func (s *Service) CreateBadge(ctx context.Context, b achievement.Badge) (*achievement.Badge, error) {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	if err := s.repo.CreateBadge(ctx, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

func (s *Service) CreateRule(ctx context.Context, r achievement.AchievementRule) (*achievement.AchievementRule, error) {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	if err := s.repo.CreateRule(ctx, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *Service) AwardBadgeManual(ctx context.Context, userID, badgeID uuid.UUID, reason string, adminID uuid.UUID) error {
	if _, err := s.repo.GetBadge(ctx, badgeID); err != nil {
		return err
	}
	fullReason := fmt.Sprintf("%s (By Admin %s)", reason, adminID.String())
	return s.grantBadge(ctx, userID, badgeID, fullReason)
}

// --- Read Methods ---

func (s *Service) ListBadges(ctx context.Context) ([]achievement.Badge, error) {
	return s.repo.ListBadges(ctx)
}

func (s *Service) GetUserAchievements(ctx context.Context, userID uuid.UUID) ([]achievement.UserAchievementStatus, error) {
	// Optimization: This could use errgroup for parallel fetching
	badges, err := s.repo.ListBadges(ctx)
	if err != nil {
		return nil, err
	}
	earned, err := s.repo.GetUserBadges(ctx, userID)
	if err != nil {
		return nil, err
	}
	metrics, err := s.repo.GetUserMetrics(ctx, userID)
	if err != nil {
		return nil, err
	}

	earnedMap := make(map[uuid.UUID]achievement.UserBadge)
	for _, ub := range earned {
		earnedMap[ub.BadgeID] = ub
	}

	result := make([]achievement.UserAchievementStatus, 0, len(badges))

	for _, badge := range badges {
		status := achievement.UserAchievementStatus{
			Badge: badge,
		}

		if ub, ok := earnedMap[badge.ID]; ok {
			status.IsEarned = true
			status.AwardedAt = &ub.AwardedAt
			status.ProgressPercent = 100.0
		} else {
			// Calculate Progress
			rules, _ := s.repo.GetRulesByBadge(ctx, badge.ID)
			for _, r := range rules {
				if r.RuleType == achievement.RuleTypeAccumulation && r.MetricName != nil && r.Threshold != nil {
					val := metrics[*r.MetricName]
					target := *r.Threshold
					status.ProgressCurrent = val
					status.ProgressTarget = target
					if target > 0 {
						status.ProgressPercent = (val / target) * 100.0
						if status.ProgressPercent > 100 {
							status.ProgressPercent = 100
						}
					}
					break
				}
			}
		}
		result = append(result, status)
	}
	return result, nil
}

func (s *Service) GetLeaderboard(ctx context.Context, limit int, period string) ([]achievement.LeaderboardEntry, error) {
	// 1. Construct Cache Key
	cacheKey := fmt.Sprintf("gamification:leaderboard:%s:%d", period, limit)

	// 2. Try Cache
	var cachedEntries []achievement.LeaderboardEntry
	if _, err := s.cache.Get(ctx, cacheKey); err == nil {
		return cachedEntries, nil // Hit!
	}

	// 3. Cache Miss - Hit DB
	// Note: We might want a "SingleFlight" here if high concurrency,
	// but standard Redis cache pattern is sufficient for now.
	entries, err := s.repo.GetGlobalLeaderboard(ctx, limit)
	if err != nil {
		return nil, err
	}

	// 4. Enrich Data (This is expensive, so good thing we cache it)
	for i := range entries {
		profile, err := s.userSvc.GetProfile(ctx, entries[i].UserID)
		if err == nil {
			entries[i].DisplayName = profile.DisplayName
			if profile.AvatarFileID != nil {
				entries[i].AvatarURL = profile.AvatarFileID.String()
			}
		} else {
			entries[i].DisplayName = "User " + entries[i].UserID.String()[0:8]
		}
	}

	// 5. Save to Cache
	// Use Configured TTL (e.g., 5 minutes)
	if err := s.cache.Set(ctx, cacheKey, entries, s.config.LeaderboardCacheTTL); err != nil {
		s.logger.Warn("failed to cache leaderboard", log.ErrorAttr(err))
	}

	return entries, nil
}

// --- Helpers ---

func matchesFilter(payload, requirements map[string]any) bool {
	for reqKey, reqVal := range requirements {
		payloadVal, exists := payload[reqKey]
		if !exists {
			return false // Requirement missing from event
		}

		if !valuesEqual(reqVal, payloadVal) {
			return false
		}
	}
	return true
}

// valuesEqual performs a loose comparison suitable for JSON types
func valuesEqual(a, b any) bool {
	// 1. Direct equality (strings, bools, exact matches)
	if reflect.DeepEqual(a, b) {
		return true
	}

	// 2. Numeric normalization
	// Go unmarshals JSON numbers as float64.
	// Internal logic might use int/int64/float32.
	numA, isNumA := toFloat64(a)
	numB, isNumB := toFloat64(b)

	if isNumA && isNumB {
		// Epsilon check for floats could be added here if needed,
		// but typically for badges we look for exact ID matches or integer thresholds.
		return numA == numB
	}

	return false
}

func toFloat64(v any) (float64, bool) {
	switch i := v.(type) {
	case float64:
		return i, true
	case float32:
		return float64(i), true
	case int:
		return float64(i), true
	case int32:
		return float64(i), true
	case int64:
		return float64(i), true
	default:
		return 0, false
	}
}
