package feature

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/feature"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)

var _ feature.Service = (*Service)(nil)

type Service struct {
	repo     feature.Repository
	cache    feature.FlagCacheReader
	notifier feature.Notifier
	engine   feature.Engine
	audit    audit.EventRecorder
	eventBus events.Bus
	logger   log.Logger
}

func NewService(
	repo feature.Repository,
	cache feature.FlagCacheReader,
	notifier feature.Notifier,
	engine feature.Engine,
	audit audit.EventRecorder,
	eventBus events.Bus,
	logger log.Logger,
) *Service {
	return &Service{
		repo:     repo,
		cache:    cache,
		notifier: notifier,
		engine:   engine,
		audit:    audit,
		eventBus: eventBus,
		logger:   logger.With(log.String("service", "feature")),
	}
}

// --- Evaluation ---

func (s *Service) Evaluate(ctx context.Context, key string, evalCtx feature.EvaluationContext) (feature.EvaluationResult, error) {
	// 1. Try Cache (In-Memory)
	flag := s.cache.GetFlag(key)

	// 2. Fallback to DB if missing (Safety net, though cache should be populated)
	if flag == nil {
		var err error
		flag, err = s.repo.Get(ctx, key)
		if err != nil {
			// Return error or default? Usually error, let caller decide default.
			return feature.EvaluationResult{}, err
		}
	}

	// 3. Evaluate using Pure Engine
	return s.engine.Evaluate(flag, evalCtx), nil
}

func (s *Service) IsEnabled(ctx context.Context, key string, evalCtx feature.EvaluationContext, defaultVal bool) (bool, error) {
	res, err := s.Evaluate(ctx, key, evalCtx)
	if err != nil {
		return defaultVal, err
	}

	var val bool
	if err := json.Unmarshal(res.Value, &val); err != nil {
		s.logger.Warn("Flag value is not boolean", log.String("key", key))
		return defaultVal, nil // Wrong type acts as default
	}
	return val, nil
}

// --- Management ---

func (s *Service) UpsertFlag(ctx context.Context, flag feature.Flag) (*feature.Flag, error) {
	actor := auth.ActorFromContext(ctx)
	if actor != nil {
		flag.UpdatedBy = actor.UserID
	}
	flag.UpdatedAt = time.Now().UTC()

	// 1. Persist
	if err := s.repo.Upsert(ctx, flag); err != nil {
		return nil, err
	}

	// 2. Notify Cache (Pub/Sub)
	// This triggers other instances to refresh their in-memory cache
	if err := s.notifier.NotifyFlagUpdated(ctx, flag.Key); err != nil {
		s.logger.Error("Failed to broadcast flag update", err)
	}

	action := core.EventFlagUpdated
	s.publishEvent(ctx, action, flag.ID, flag)

	if err := s.audit.Record(ctx, audit.NewEvent(flag.UpdatedBy, "user", action).
		WithTarget(flag.ID, "feature_flag", flag.Key).
		WithSuccess().Build()); err != nil {
		s.logger.Error("failed to record flag upsert audit event", err)
	}

	return &flag, nil
}

func (s *Service) GetFlag(ctx context.Context, key string) (*feature.Flag, error) {
	// Management always hits DB for consistency
	return s.repo.Get(ctx, key)
}

func (s *Service) ListFlags(ctx context.Context) ([]feature.Flag, error) {
	return s.repo.ListAll(ctx)
}

func (s *Service) DeleteFlag(ctx context.Context, key string) error {
	// 1. Get ID for Audit
	f, err := s.repo.Get(ctx, key)
	if err != nil {
		return err
	}

	// 2. Delete
	if err := s.repo.Delete(ctx, key); err != nil {
		return err
	}

	// 3. Notify
	if err := s.notifier.NotifyFlagUpdated(ctx, key); err != nil {
		s.logger.Error("Failed to notify flag deletion", err)
	}

	s.publishEvent(ctx, core.EventFlagDeleted, f.ID, map[string]string{"key": key})

	// 4. Audit
	actor := auth.ActorFromContext(ctx)
	var actorID uuid.UUID
	if actor != nil {
		actorID = actor.UserID
	}
	if err := s.audit.Record(ctx, audit.NewEvent(actorID, "user", core.EventFlagDeleted).
		WithTarget(f.ID, "feature_flag", key).
		WithSuccess().Build()); err != nil {
		s.logger.Error("failed to record flag delete audit event", err)
	}

	return nil
}

func (s *Service) GetStringValue(ctx context.Context, key string, evalCtx feature.EvaluationContext, defaultValue string) (string, error) {
	res, err := s.Evaluate(ctx, key, evalCtx)
	if err != nil {
		return defaultValue, err
	}

	var val string
	if err := json.Unmarshal(res.Value, &val); err != nil {
		s.logger.Warn("Flag value is not string", log.String("key", key), log.ErrorAttr(err))
		return defaultValue, nil
	}
	return val, nil
}

func (s *Service) GetClientBootstrapData(ctx context.Context, evalCtx feature.EvaluationContext) (*feature.ClientBootstrap, error) {
	// 1. Get all active flags from cache (fastest)
	allFlags := s.cache.ListActiveFlags()

	results := make(map[string]feature.EvaluationResult)

	// 2. Evaluate every active flag against the user context
	for _, f := range allFlags {
		// We evaluate strictly server-side.
		// This encapsulates all rule logic (percentage rollouts, user targeting)
		// so the client just receives "red" or "blue".
		res := s.engine.Evaluate(f, evalCtx)

		// Optimization: We strip RuleID to save bandwidth as clients generally don't need to know *why* they got a value.
		// We keep VariationID as it's useful for tracking/analytics events on the client.
		results[f.Key] = feature.EvaluationResult{
			Key:         res.Key,
			Value:       res.Value,
			VariationID: res.VariationID,
			// RuleID is intentionally omitted from client payload
		}
	}

	return &feature.ClientBootstrap{
		Flags: results,
		// Settings can be populated from a ConfigService if you have one.
		// For now, we return an empty map.
		Settings: map[string]interface{}{},
	}, nil
}

func (s *Service) publishEvent(ctx context.Context, action core.EventAction, targetID uuid.UUID, payload interface{}) {
	evt, err := events.NewEvent(action, uuid.Nil, "system", payload)
	if err == nil {
		tType := "feature_flag"
		evt.TargetID = &targetID
		evt.TargetType = &tType
		_ = s.eventBus.Publish(ctx, evt)
	}
}
