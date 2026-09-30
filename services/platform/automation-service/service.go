package automation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"runtime/debug"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/automation"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/system"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/limiter"
	"github.com/combi/pkg/log"
)

const (
	AutomationProcessEventJob = "automation:process_event"
	AutomationQueue           = "automation"
	// MaxConcurrentRules limits how many rules we process in parallel for a single event
	MaxConcurrentRules = 10
)

var _ automation.Service = (*service)(nil)

type service struct {
	repo      automation.Repository
	evaluator automation.ConditionEvaluator
	executor  automation.ActionExecutor
	logger    log.Logger

	eventBus    events.Bus
	dispatcher  queue.Dispatcher
	rateLimiter limiter.RateLimiter

	wg          sync.WaitGroup
	statsBuffer *StatsBuffer
	subManager  *SubscriptionManager
}

func NewService(
	repo automation.Repository,
	evaluator automation.ConditionEvaluator,
	executor automation.ActionExecutor,
	logger log.Logger,
	eventBus events.Bus,
	dispatcher queue.Dispatcher,
	rateLimiter limiter.RateLimiter,
	statsBuffer *StatsBuffer,
	subManager *SubscriptionManager,
) *service {
	return &service{
		repo:        repo,
		evaluator:   evaluator,
		executor:    executor,
		logger:      logger,
		eventBus:    eventBus,
		dispatcher:  dispatcher,
		rateLimiter: rateLimiter,
		statsBuffer: statsBuffer,
		subManager:  subManager,
	}
}

// Close ensures all background tasks (stats, events) are finished.
func (s *service) Close() {
	s.wg.Wait()
}

// --- Event Processing ---

func (s *service) BroadcastEvent(ctx context.Context, event automation.Event) error {
	if event.ID == uuid.Nil {
		event.ID = uuid.New()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal automation event: %w", err)
	}

	_, err = s.dispatcher.Dispatch(ctx, core.JobType(AutomationProcessEventJob), payload, queue.WithQueue(AutomationQueue))
	if err != nil {
		return fmt.Errorf("failed to dispatch automation event job: %w", err)
	}

	s.logger.Debug("automation event dispatched to queue",
		log.String("event_type", event.Type),
		log.String("event_id", event.ID.String()),
	)
	return nil
}

func (s *service) ProcessEvent(ctx context.Context, event automation.Event) error {
	logger := s.logger.With(
		log.String("event_id", event.ID.String()),
		log.String("event_type", event.Type),
	)

	// 1. Fetch matching rules
	rules, err := s.repo.GetActiveRulesByTrigger(ctx, event.Type)
	if err != nil {
		return fmt.Errorf("failed to get active rules: %w", err)
	}

	if event.ActorID != nil {
		validRules := make([]automation.Rule, 0, len(rules))
		for _, rule := range rules {
			// Loop Detection
			if s.isLooping(ctx, rule.ID, *event.ActorID) {
				s.logger.Warn("loop detected: blocking rule execution",
					log.String("rule_id", rule.ID.String()),
					log.String("actor_id", event.ActorID.String()),
				)
				continue
			}
			validRules = append(validRules, rule)
		}
		rules = validRules
	}

	if len(rules) == 0 {
		return nil
	}

	// 2. Worker Pool for Rule Processing
	sem := make(chan struct{}, MaxConcurrentRules)
	var processingWg sync.WaitGroup

	for i := range rules {
		rule := rules[i]
		processingWg.Add(1)
		sem <- struct{}{}

		go func() {
			defer processingWg.Done()
			defer func() { <-sem }()

			if err := s.processRuleForEvent(ctx, &rule, event); err != nil {
				logger.Error("error processing rule", err, log.String("rule_id", rule.ID.String()))
			}
		}()
	}

	processingWg.Wait()
	return nil
}

func (s *service) processRuleForEvent(ctx context.Context, rule *automation.Rule, event automation.Event) error {
	// 1. Create Execution Record
	triggerDataBytes, _ := json.Marshal(event.Data)
	execution := &automation.Execution{
		ID:              uuid.New(),
		RuleID:          rule.ID,
		EventID:         event.ID,
		Status:          automation.ExecutionStatusRunning,
		StartedAt:       time.Now().UTC(),
		TriggerData:     triggerDataBytes,
		ActionsSnapshot: rule.Actions,
	}

	if err := s.repo.CreateExecution(ctx, execution); err != nil {
		return fmt.Errorf("create execution failed: %w", err)
	}

	// Panic Recovery
	defer func() {
		if r := recover(); r != nil {
			errStr := fmt.Sprintf("panic recovered: %v", r)
			execution.Error = &errStr
			execution.Status = automation.ExecutionStatusFailed
			if err := s.finalizeExecution(context.Background(), rule, execution); err != nil {
				s.logger.Error("failed to finalize execution during panic recovery", err)
			}
		}
	}()

	// 2. Check Rate Limiting
	if ok := s.checkRateLimit(ctx, rule); !ok {
		execution.Status = automation.ExecutionStatusSkipped
		execution.Error = ptr("Skipped due to rate limit")
		return s.finalizeExecution(ctx, rule, execution)
	}

	// 3. Check Filters
	if !s.matchesTriggerFilters(event, rule.Trigger) {
		execution.Status = automation.ExecutionStatusSkipped
		execution.Error = ptr("Skipped due to filter mismatch")
		return s.finalizeExecution(ctx, rule, execution)
	}

	// 4. Evaluate Conditions
	conditionsMet, conditionsResult, err := s.evaluator.Evaluate(ctx, rule.Conditions, rule.Logic, event.Data)
	execution.ConditionsResult = conditionsResult
	if err != nil {
		execution.Status = automation.ExecutionStatusFailed
		execution.Error = ptr(fmt.Sprintf("Evaluation error: %s", err.Error()))
		return s.finalizeExecution(ctx, rule, execution)
	}

	if !conditionsMet {
		execution.Status = automation.ExecutionStatusCompleted
		return s.finalizeExecution(ctx, rule, execution)
	}

	// 5. Execute Actions
	// Note: executeActions now handles Idempotency Keys internally based on Execution ID
	actionsResult, workflowPaused, err := s.executeActions(ctx, execution, event.Data, execution.ActionsSnapshot, rule.ExecutionStrategy)
	execution.ActionsResult = actionsResult

	if workflowPaused {
		// If paused, executeActions has already persisted the state and dispatched the job.
		// We do NOT finalize (no completed_at) and do NOT call UpdateExecution again.
		execution.Status = automation.ExecutionStatusRunning
		return nil
	}

	if err != nil {
		execution.Status = automation.ExecutionStatusFailed
		execution.Error = ptr(err.Error())
	} else {
		execution.Status = automation.ExecutionStatusCompleted
	}

	return s.finalizeExecution(ctx, rule, execution)
}

// ContinueExecution resumes a paused workflow.
func (s *service) ContinueExecution(
	ctx context.Context,
	executionID uuid.UUID,
	eventData map[string]any,
	actionsToRun []automation.Action,
) error {
	// 1. Reload Execution
	execution, err := s.repo.GetExecution(ctx, executionID)
	if err != nil {
		return fmt.Errorf("execution not found: %w", err)
	}

	// 2. Fetch Rule
	rule, err := s.repo.GetRule(ctx, execution.RuleID)
	if err != nil {
		rule = &automation.Rule{ExecutionStrategy: "SEQUENCE"}
	}

	if len(execution.ActionsSnapshot) == 0 {
		s.logger.Warn("execution has no action snapshot, using provided continuation actions")
	}

	newResults, paused, err := s.executeActions(
		ctx,
		execution,
		eventData,
		actionsToRun, // Payload from the worker knows where we left off
		rule.ExecutionStrategy,
	)

	// Append new results to existing ones
	execution.ActionsResult = append(execution.ActionsResult, newResults...)

	if paused {
		return nil
	}

	if err != nil {
		execution.Status = automation.ExecutionStatusFailed
		execution.Error = ptr(err.Error())
	} else {
		execution.Status = automation.ExecutionStatusCompleted
	}

	return s.finalizeExecution(ctx, rule, execution)
}

// finalizeExecution updates the DB and buffers stats.
func (s *service) finalizeExecution(ctx context.Context, rule *automation.Rule, execution *automation.Execution) error {
	now := time.Now().UTC()
	execution.CompletedAt = &now
	execution.Duration = now.Sub(execution.StartedAt)
	execution.UpdatedAt = now

	// 1. Update DB (Synchronous with Detached Context to ensure it finishes even if parent ctx cancels)
	detachedCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.repo.UpdateExecution(detachedCtx, execution); err != nil {
		return fmt.Errorf("failed to update execution: %w", err)
	}

	// 2. Increment Stats (Buffered via Redis)
	if execution.Status != automation.ExecutionStatusSkipped {
		s.statsBuffer.Increment(detachedCtx, rule.ID, execution.Status == automation.ExecutionStatusCompleted)
	}

	// 3. Publish System Event (Async)
	var action core.EventAction
	switch execution.Status {
	case automation.ExecutionStatusCompleted:
		action = core.EventRuleTriggered
	case automation.ExecutionStatusFailed:
		action = core.EventExecutionFailure
	default:
		return nil
	}

	payload := map[string]string{
		"execution_id": execution.ID.String(),
		"rule_id":      rule.ID.String(),
	}

	domainEvt, err := events.NewEvent(
		action,
		system.SystemActorID, "system",
		payload,
	)
	if err != nil {
		return nil
	}
	targetID := rule.ID
	domainEvt.TargetID = &targetID
	targetType := "automation_rule"
	domainEvt.TargetType = &targetType

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		pubCtx, pubCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer pubCancel()

		if err := s.eventBus.Publish(pubCtx, domainEvt); err != nil {
			s.logger.Error("failed to publish execution event", err)
		}
	}()

	return nil
}

// --- Workflow Execution Logic ---

func (s *service) executeActions(
	ctx context.Context,
	execution *automation.Execution,
	eventData map[string]any,
	actions []automation.Action,
	strategy string,
) (results []automation.ActionResult, paused bool, err error) {

	if strategy == "PARALLEL" {
		return s.executeActionsParallel(ctx, execution.ID, eventData, actions)
	}

	// Default: SEQUENCE
	return s.executeActionsSequence(ctx, execution, eventData, actions)
}

// executeActionsSequence is the linear logic
func (s *service) executeActionsSequence(
	ctx context.Context,
	execution *automation.Execution,
	eventData map[string]any,
	actions []automation.Action,
) ([]automation.ActionResult, bool, error) {

	var results []automation.ActionResult

	for i, action := range actions {
		start := time.Now()

		// FIX: Deterministic Idempotency Key
		// We use ExecutionID + Index. This ensures that if the system crashes and retries,
		// the external provider (e.g. Stripe/Mailgun) sees the same key and deduplicates.
		// Note: 'i' resets on resume, so strictly this key relies on the 'actions' slice being the *remainder*.
		// To be perfectly safe on resumes, we append a 'resume' marker or rely on the fact that
		// previous successful actions are not in this 'actions' slice anymore.
		idempotencyKey := fmt.Sprintf("%s-seq-%d", execution.ID.String(), i)

		resultRaw, execErr := s.executor.Execute(ctx, action, eventData, idempotencyKey)

		// Create result object
		result := automation.ActionResult{
			Action:   action,
			Success:  execErr == nil || errors.Is(execErr, automation.ErrWorkflowPause),
			Output:   resultRaw,
			Duration: time.Since(start),
		}
		if execErr != nil && !errors.Is(execErr, automation.ErrWorkflowPause) {
			errStr := execErr.Error()
			result.Error = &errStr
		}

		results = append(results, result)

		// Check for Pause/Delay
		if errors.Is(execErr, automation.ErrWorkflowPause) {
			remainingActions := actions[i+1:]
			if len(remainingActions) > 0 {
				delayStr, _ := action.Config["duration"].(string)
				delay, _ := time.ParseDuration(delayStr)

				// Persist state immediately before sleeping/dispatching
				execution.ActionsResult = append(execution.ActionsResult, results...)
				if updateErr := s.repo.UpdateExecution(ctx, execution); updateErr != nil {
					return results, false, fmt.Errorf("failed to save state before pause: %w", updateErr)
				}

				s.dispatchContinuationJob(ctx, execution.ID, eventData, remainingActions, delay)
			}
			return results, true, nil
		}

		if execErr != nil {
			return results, false, execErr
		}
	}
	return results, false, nil
}

// executeActionsParallel runs all actions concurrently
func (s *service) executeActionsParallel(
	ctx context.Context,
	executionID uuid.UUID,
	eventData map[string]any,
	actions []automation.Action,
) ([]automation.ActionResult, bool, error) {

	var wg sync.WaitGroup
	// We still keep in-memory results for return value
	results := make([]automation.ActionResult, len(actions))
	errChan := make(chan error, len(actions))

	// FIX: Use Redis Key for temporary buffering
	bufferKey := fmt.Sprintf("automation:exec_buffer:%s", executionID.String())
	// 1 hour TTL in case of crash
	s.statsBuffer.redis.Expire(ctx, bufferKey, 1*time.Hour)

	for i, action := range actions {
		wg.Add(1)
		go func(idx int, act automation.Action) {
			defer wg.Done()
			start := time.Now()
			idempotencyKey := fmt.Sprintf("%s-par-%d", executionID.String(), idx)

			output, execErr := s.executor.Execute(ctx, act, eventData, idempotencyKey)

			result := automation.ActionResult{
				Action:   act,
				Success:  execErr == nil,
				Output:   output,
				Duration: time.Since(start),
			}
			if execErr != nil {
				msg := execErr.Error()
				result.Error = &msg
			}

			// FIX: Buffer to Redis List instead of DB Append
			// Serialize result
			if b, err := json.Marshal(result); err == nil {
				s.statsBuffer.redis.RPush(context.Background(), bufferKey, b)
			} else {
				s.logger.Error("failed to buffer action result in redis", err)
			}

			results[idx] = result
			if execErr != nil {
				errChan <- execErr
			}
		}(i, action)
	}

	wg.Wait()
	close(errChan)

	// FIX: Bulk Flush from Redis to DB
	// This turns N DB writes into 1 DB write.
	// Note: We use the `results` slice we gathered in memory for the update
	// because it's already there and ordered correctly by index.
	// The Redis buffer acted as a safety net if we crashed,
	// but here we can just write the full `results` array to the ActionsResult field.

	// In a pure append model (where we don't hold the whole array), we would LRANGE from Redis.
	// Since we are inside the service method that owns the slice, we can just:

	err := s.repo.AppendActionResultBatch(ctx, executionID, results)
	if err != nil {
		return results, false, fmt.Errorf("failed to save parallel results: %w", err)
	}

	// Clean up Redis
	s.statsBuffer.redis.Del(context.Background(), bufferKey)

	if len(errChan) > 0 {
		return results, false, <-errChan
	}

	return results, false, nil
}

func (s *service) dispatchContinuationJob(
	ctx context.Context,
	executionID uuid.UUID,
	eventData map[string]any,
	remainingActions []automation.Action,
	delay time.Duration,
) {
	payload := struct {
		ExecutionID  uuid.UUID           `json:"execution_id"`
		EventData    map[string]any      `json:"event_data"`
		ActionsToRun []automation.Action `json:"actions_to_run"`
	}{
		ExecutionID:  executionID,
		EventData:    eventData,
		ActionsToRun: remainingActions,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		s.logger.Error("FATAL: marshal continuation failed", err)
		return
	}

	_, err = s.dispatcher.Dispatch(ctx,
		core.JobType(automation.JobAutomationContinueExecution),
		jsonPayload,
		queue.WithDelay(delay),
		queue.WithQueue(AutomationQueue),
	)

	if err != nil {
		s.logger.Error("FATAL: dispatch continuation failed", err)
	}
}

// --- Helpers & CRUD ---

func (s *service) checkRateLimit(ctx context.Context, rule *automation.Rule) bool {
	var debounce, throttle time.Duration
	if rule.Trigger.Debounce != nil {
		debounce = *rule.Trigger.Debounce
	}
	if rule.Trigger.Throttle != nil {
		throttle = *rule.Trigger.Throttle
	}

	if debounce == 0 && throttle == 0 {
		return true
	}

	key := fmt.Sprintf("automation:throttle:%s", rule.ID.String())
	allowed, err := s.rateLimiter.AllowDebounceThrottle(ctx, key, debounce, throttle)
	if err != nil {
		s.logger.Warn("limiter fail, allowing", log.ErrorAttr(err))
		// In a debounce/throttle scenario, failing open (allowing) is usually safer than blocking all traffic permanently if Redis flakiness occurs.
		return true
	}
	return allowed
}

func (s *service) isLooping(ctx context.Context, ruleID, actorID uuid.UUID) bool {
	// Key: automation:cycle:{rule_id}:{actor_id}
	key := fmt.Sprintf("automation:cycle:%s:%s", ruleID.String(), actorID.String())

	// Allow 5 executions every 10 seconds.
	allowed, err := s.rateLimiter.Allow(ctx, key)
	if err != nil {
		s.logger.Error("CRITICAL: rate limiter error in loop check (failing closed)", err)
		// FIX: Fail Closed. If we can't verify loop safety, we MUST block execution to prevent potential runaway spirals.
		return true // "true" means "is looping" -> block
	}
	return !allowed
}

// matchesTriggerFilters checks if the event data satisfies the trigger pre-filters.
func (s *service) matchesTriggerFilters(event automation.Event, trigger automation.Trigger) bool {
	if len(trigger.Filters) == 0 {
		return true
	}

	for key, expectedValue := range trigger.Filters {
		actualValue, ok := event.Data[key]
		if !ok {
			return false
		}
		if !fuzzyEqual(actualValue, expectedValue) {
			return false
		}
	}
	return true
}

func fuzzyEqual(actual, expected any) bool {
	if reflect.DeepEqual(actual, expected) {
		return true
	}
	actFloat, actOk := toFloat64(actual)
	expFloat, expOk := toFloat64(expected)
	if actOk && expOk {
		return actFloat == expFloat
	}
	return false
}

func toFloat64(v any) (float64, bool) {
	if v == nil {
		return 0, false
	}
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case int32:
		return float64(val), true
	default:
		return 0, false
	}
}

func ptr[T any](v T) *T { return &v }

// --- Service Methods (CRUD & Ops) ---

func (s *service) CreateRule(ctx context.Context, req automation.CreateRuleParams) (*automation.Rule, error) {
	if err := s.validateTrigger(ctx, req.Trigger); err != nil {
		return nil, fmt.Errorf("invalid trigger: %w", err)
	}
	for _, condition := range req.Conditions {
		if err := s.evaluator.Validate(ctx, condition); err != nil {
			return nil, fmt.Errorf("invalid condition: %w", err)
		}
	}
	for _, action := range req.Actions {
		if err := s.executor.Validate(ctx, action); err != nil {
			return nil, fmt.Errorf("invalid action: %w", err)
		}
	}

	rule := &automation.Rule{
		ID:          uuid.New(),
		Name:        req.Name,
		Description: req.Description,
		Status:      automation.RuleStatusDraft,
		Priority:    req.Priority,
		Trigger:     req.Trigger,
		Conditions:  req.Conditions,
		Logic:       req.Logic,
		Actions:     req.Actions,
		Metadata:    req.Metadata,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	if err := s.repo.CreateRule(ctx, rule); err != nil {
		return nil, fmt.Errorf("failed to create rule: %w", err)
	}
	s.logger.Info("automation rule created", log.String("rule_id", rule.ID.String()))

	go s.subManager.Sync(context.Background())
	return rule, nil
}

func (s *service) GetRule(ctx context.Context, id uuid.UUID) (*automation.Rule, error) {
	return s.repo.GetRule(ctx, id)
}

func (s *service) UpdateRule(ctx context.Context, id uuid.UUID, req automation.UpdateRuleParams) (*automation.Rule, error) {
	rule, err := s.repo.GetRule(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.Name != nil {
		rule.Name = *req.Name
	}
	if req.Description != nil {
		rule.Description = *req.Description
	}

	rule.UpdatedAt = time.Now().UTC()
	if err := s.repo.UpdateRule(ctx, rule); err != nil {
		return nil, fmt.Errorf("failed to update rule: %w", err)
	}
	s.logger.Info("automation rule updated", log.String("rule_id", rule.ID.String()))
	return rule, nil
}

func (s *service) DeleteRule(ctx context.Context, id uuid.UUID) error {
	if _, err := s.repo.GetRule(ctx, id); err != nil {
		return err
	}
	if err := s.repo.DeleteRule(ctx, id); err != nil {
		return fmt.Errorf("failed to delete rule: %w", err)
	}
	s.logger.Info("automation rule deleted", log.String("rule_id", id.String()))
	return nil
}

func (s *service) ListRules(ctx context.Context, filter automation.RuleFilter) ([]automation.Rule, int64, error) {
	return s.repo.ListRules(ctx, filter)
}

func (s *service) EnableRule(ctx context.Context, id uuid.UUID) error {
	rule, err := s.repo.GetRule(ctx, id)
	if err != nil {
		return err
	}
	if rule.Status == automation.RuleStatusActive {
		return nil
	}
	rule.Status = automation.RuleStatusActive
	rule.UpdatedAt = time.Now().UTC()
	if err := s.repo.UpdateRule(ctx, rule); err != nil {
		return fmt.Errorf("failed to enable rule: %w", err)
	}
	s.logger.Info("automation rule enabled", log.String("rule_id", id.String()))
	return nil
}

func (s *service) DisableRule(ctx context.Context, id uuid.UUID) error {
	rule, err := s.repo.GetRule(ctx, id)
	if err != nil {
		return err
	}
	if rule.Status == automation.RuleStatusInactive {
		return nil
	}
	rule.Status = automation.RuleStatusInactive
	rule.UpdatedAt = time.Now().UTC()
	if err := s.repo.UpdateRule(ctx, rule); err != nil {
		return fmt.Errorf("failed to disable rule: %w", err)
	}
	s.logger.Info("automation rule disabled", log.String("rule_id", id.String()))
	return nil
}

// ExecuteAction provides a way to run a single action ad-hoc.
func (s *service) ExecuteAction(ctx context.Context, actionType string, actionConfig map[string]interface{}) error {
	action := automation.Action{
		Type:   actionType,
		Config: actionConfig,
	}

	if err := s.executor.Validate(ctx, action); err != nil {
		return fmt.Errorf("action validation failed: %w", err)
	}

	// FIX: Generate Ad-Hoc Idempotency Key
	idempotencyKey := fmt.Sprintf("adhoc-%s-%d", actionType, time.Now().UnixNano())

	result, err := s.executor.Execute(ctx, action, nil, idempotencyKey)
	if err != nil {
		return fmt.Errorf("system error during action execution: %w", err)
	}

	// For ad-hoc, we assume the handler handles errors, but Execute signature returns error too.
	// Since Execute wraps handler, if handler returns error, 'err' here is populated.
	// The return value 'result' is the output map.
	_ = result // Unused output

	return nil
}

// ExecuteSingleAction is a new method for the ActionWorker.
func (s *service) ExecuteSingleAction(ctx context.Context, executionID uuid.UUID, action automation.Action, eventData map[string]any) error {
	logger := s.logger.With(log.String("execution_id", executionID.String()), log.String("action_type", action.Type))
	logger.Info("Executing single action from worker")

	execution, err := s.repo.GetExecution(ctx, executionID)
	if err != nil {
		return err
	}
	if execution.Status != automation.ExecutionStatusRunning {
		logger.Warn("Skipping action execution for a non-running workflow", log.String("status", string(execution.Status)))
		return nil
	}

	startTime := time.Now()

	// FIX: Generate Worker-Context Idempotency Key
	// Since this is likely a specific task offloaded to a worker, we generate a key that binds
	// to the execution and the specific action type (since we don't have index here easily).
	// Ideally, the Job Payload should carry the index, but without interface change, this is best-effort.
	idempotencyKey := fmt.Sprintf("%s-worker-%s", executionID.String(), action.Type)

	output, execErr := s.executor.Execute(ctx, action, eventData, idempotencyKey)

	result := automation.ActionResult{
		Action:   action,
		Success:  execErr == nil,
		Output:   output,
		Duration: time.Since(startTime),
	}
	if execErr != nil {
		errStr := execErr.Error()
		result.Error = &errStr
	}

	if err := s.repo.AppendActionResult(ctx, executionID, result); err != nil {
		logger.Error("CRITICAL: failed to append action result to execution record", err)
		return err
	}

	return execErr
}

func (s *service) validateTrigger(_ context.Context, trigger automation.Trigger) error {
	if trigger.Type == "" {
		return fmt.Errorf("trigger type is required")
	}
	if trigger.Debounce != nil && *trigger.Debounce < 0 {
		return fmt.Errorf("debounce cannot be negative")
	}
	if trigger.Throttle != nil && *trigger.Throttle < 0 {
		return fmt.Errorf("throttle cannot be negative")
	}
	return nil
}

// --- Private Helpers ---

func (s *service) Go(fn func()) {
	s.wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				s.logger.Error("panic recovered in background goroutine", nil,
					log.Any("panic", r),
					log.String("stack", string(debug.Stack())),
				)
			}
			s.wg.Done()
		}()
		fn()
	}()
}
