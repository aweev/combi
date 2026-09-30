package automation

import (
	"context"
	"sync"
	"time"

	"github.com/combi/pkg/domain/automation"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)

// SubscriptionManager ensures the EventBus is subscribed to every Trigger Type
// currently used by an Active automation rule.
type SubscriptionManager struct {
	bus     events.Bus
	repo    automation.Repository
	handler events.EventHandler // The universal handler from the Subscriber (usually handleAny)
	logger  log.Logger

	// subscribed tracks which topics we are already listening to.
	// We use sync.Map for thread-safe concurrent access.
	subscribed    sync.Map // map[string]bool
	refreshTicker *time.Ticker
	stopCh        chan struct{}
	once          sync.Once
}

func NewSubscriptionManager(
	bus events.Bus,
	repo automation.Repository,
	handler events.EventHandler,
	logger log.Logger,
) *SubscriptionManager {
	return &SubscriptionManager{
		bus:     bus,
		repo:    repo,
		handler: handler,
		logger:  logger.With(log.String("component", "automation_sub_manager")),
		stopCh:  make(chan struct{}),
	}
}

// Start begins the background sync loop.
// It should be called when the application starts (in App.StartBackgroundServices).
func (sm *SubscriptionManager) Start(ctx context.Context) {
	// 1. Initial Sync
	sm.Sync(ctx)

	// 2. Poll every minute to catch any drift or manual DB changes
	sm.refreshTicker = time.NewTicker(1 * time.Minute)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-sm.stopCh:
				return
			case <-sm.refreshTicker.C:
				sm.Sync(ctx)
			}
		}
	}()
}

// Stop gracefully shuts down the background poller.
func (sm *SubscriptionManager) Stop() {
	sm.once.Do(func() {
		if sm.refreshTicker != nil {
			sm.refreshTicker.Stop()
		}
		close(sm.stopCh)
	})
}

// Sync queries the DB for active triggers and subscribes to any new ones found.
// This is idempotent; calling it multiple times is safe.
func (sm *SubscriptionManager) Sync(ctx context.Context) {
	// 1. Get all distinct trigger types from ACTIVE rules in the DB.
	// This ensures we stop listening to topics if no active rules use them
	// (though RedisStreamBus usually keeps subscriptions open, which is fine).
	triggerTypes, err := sm.repo.GetUniqueActiveTriggers(ctx)
	if err != nil {
		sm.logger.Error("failed to sync subscriptions: database error", err)
		return
	}

	newCount := 0
	for _, topic := range triggerTypes {
		// 2. Check if we are already subscribed
		if _, exists := sm.subscribed.Load(topic); !exists {

			// 3. Dynamic Subscribe
			// The EventBus implementation must support "Hot Joining" (subscribing after Start).
			sm.bus.Subscribe(topic, sm.handler)

			// Mark as subscribed
			sm.subscribed.Store(topic, true)
			newCount++

			sm.logger.Debug("dynamically subscribed to new automation topic", log.String("topic", topic))
		}
	}

	if newCount > 0 {
		sm.logger.Info("automation subscription sync complete", log.Int("new_topics_count", newCount))
	}
}
