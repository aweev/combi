package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/system"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/monitoring"
	"github.com/combi/pkg/monitoring/prometheus"
)

// AlertWorker processes alert evaluation jobs.
type AlertWorker struct {
	ruleRepo    monitoring.AlertRuleRepository
	silenceRepo monitoring.AlertSilenceRepository
	prom        prometheus.Adapter
	eventBus    events.Bus
	logger      log.Logger
}

func NewAlertWorker(
	rules monitoring.AlertRuleRepository,
	silences monitoring.AlertSilenceRepository,
	prom prometheus.Adapter,
	bus events.Bus,
	logger log.Logger,
) *AlertWorker {
	return &AlertWorker{
		ruleRepo:    rules,
		silenceRepo: silences,
		prom:        prom,
		eventBus:    bus,
		logger:      logger.With(log.String("worker", "alert_evaluator")),
	}
}

func (w *AlertWorker) JobType() core.JobType {
	return monitoring.JobEvaluateAlerts
}

func (w *AlertWorker) ProcessJob(ctx context.Context, job *queue.Job) (json.RawMessage, error) {
	// 1. Fetch active rules
	rules, err := w.ruleRepo.ListRules(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list alert rules: %w", err)
	}

	// 2. Fetch active silences
	now := time.Now().UTC()
	silences, err := w.silenceRepo.ListActiveSilences(ctx, now)
	if err != nil {
		w.logger.Warn("Failed to fetch active silences, proceeding anyway", log.ErrorAttr(err))
	}

	firedCount := 0

	// 3. Evaluate each rule
	for _, rule := range rules {
		if !rule.IsEnabled {
			continue
		}

		firing, value, err := w.prom.QueryCondition(ctx, rule.PromQLQuery)
		if err != nil {
			w.logger.Error("Failed to evaluate rule", err, log.String("rule", rule.Name))
			continue
		}

		if firing {
			if w.isSilenced(rule, silences) {
				w.logger.Info("Alert fired but suppressed by silence", log.String("rule", rule.Name))
				continue
			}

			w.fireAlert(ctx, rule, value)
			firedCount++
		}
	}

	return json.RawMessage(fmt.Sprintf(`{"evaluated": %d, "fired": %d}`, len(rules), firedCount)), nil
}

func (w *AlertWorker) isSilenced(rule *monitoring.AlertRule, silences []*monitoring.AlertSilence) bool {
	for _, s := range silences {
		match := true
		for k, v := range s.Matchers {
			// If rule doesn't have label, or label value mismatch
			if rv, ok := rule.Labels[k]; !ok || rv != v {
				// Special check for 'alertname'
				if k == "alertname" && v == rule.Name {
					continue
				}
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func (w *AlertWorker) fireAlert(ctx context.Context, rule *monitoring.AlertRule, value float64) {
	alert := monitoring.Alert{
		RuleID:      rule.ID,
		State:       monitoring.StateFiring,
		Labels:      rule.Labels,
		Annotations: rule.Annotations,
		StartsAt:    time.Now().UTC(),
		Value:       fmt.Sprintf("%f", value),
	}
	alert.GenerateFingerprint()

	w.logger.Info("Firing Alert",
		log.String("rule", rule.Name),
		log.String("fingerprint", alert.Fingerprint),
		log.Float64("value", value),
	)

	// Publish Event
	evt, _ := events.NewEvent(
		core.EventAction("monitoring.alert.fired"),
		system.SystemActorID,
		system.SystemActorType,
		alert,
	)

	if err := w.eventBus.Publish(ctx, evt); err != nil {
		w.logger.Error("Failed to publish alert event", err)
	}
}
