package notification

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/notification"
	"github.com/combi/pkg/cache"
	"github.com/combi/pkg/log"
)

// Statically verify that it implements the domain interface.
var _ notification.PreferenceChecker = (*PreferenceChecker)(nil)

// PreferenceChecker implements the business logic for respecting user notification preferences.
type PreferenceChecker struct {
	repo   notification.Repository
	cache  cache.Service // Optional: For caching user preferences to reduce DB load
	logger log.Logger
}

// NewPreferenceChecker creates a new preference checker service.
func NewPreferenceChecker(repo notification.Repository, cache cache.Service, logger log.Logger) *PreferenceChecker {
	return &PreferenceChecker{
		repo:   repo,
		cache:  cache,
		logger: logger.With(log.String("component", "preference_checker")),
	}
}

// CanSend determines if a notification is allowed based on the user's stored preferences.
// It checks preferences in order of specificity: Notification -> Category -> Channel -> Global.
func (p *PreferenceChecker) CanSend(
	ctx context.Context,
	userID uuid.UUID,
	channel notification.NotificationChannel,
	category string,
	templateID *uuid.UUID,
) (bool, error) {

	// 1. Fetch all preferences for the user. Caching is highly recommended here.
	// In a real system, you would add caching logic around this repository call.
	prefs, err := p.repo.GetUserPreferences(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("could not fetch user preferences: %w", err)
	}

	// If no preferences are set, the default is to allow.
	if len(prefs) == 0 {
		return true, nil
	}

	// 2. Create a map for efficient lookups.
	// The key represents the preference scope and target.
	prefMap := make(map[string]bool)
	for _, pref := range prefs {
		key := p.buildPreferenceKey(pref)
		prefMap[key] = pref.IsEnabled
	}

	// 3. Check from most specific to least specific.

	// Check for a specific template-level preference.
	if templateID != nil {
		key := fmt.Sprintf("NOTIFICATION:%s:%s", channel, templateID.String())
		if enabled, ok := prefMap[key]; ok {
			p.logger.Debug("preference check result", log.String("scope", "notification"), log.Bool("allowed", enabled))
			return enabled, nil
		}
	}

	// Check for a category-level preference.
	if category != "" {
		key := fmt.Sprintf("CATEGORY:%s:%s", channel, category)
		if enabled, ok := prefMap[key]; ok {
			p.logger.Debug("preference check result", log.String("scope", "category"), log.Bool("allowed", enabled))
			return enabled, nil
		}
	}

	// Check for a channel-level preference (global for that channel).
	key := fmt.Sprintf("GLOBAL:%s", channel)
	if enabled, ok := prefMap[key]; ok {
		p.logger.Debug("preference check result", log.String("scope", "channel_global"), log.Bool("allowed", enabled))
		return enabled, nil
	}

	// 4. If no specific preference is found, the default is to allow.
	p.logger.Debug("no specific preference found, defaulting to allowed", log.String("user_id", userID.String()))
	return true, nil
}

// buildPreferenceKey creates a consistent map key for a preference.
func (p *PreferenceChecker) buildPreferenceKey(pref notification.NotificationPreference) string {
	switch pref.Scope {
	case notification.ScopeNotification:
		if pref.TemplateID != nil {
			return fmt.Sprintf("NOTIFICATION:%s:%s", pref.Channel, pref.TemplateID.String())
		}
	case notification.ScopeCategory:
		if pref.Category != nil {
			return fmt.Sprintf("CATEGORY:%s:%s", pref.Channel, *pref.Category)
		}
	case notification.ScopeGlobal:
		return fmt.Sprintf("GLOBAL:%s", pref.Channel)
	}
	return ""
}
