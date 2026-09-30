// internal/services/automation/integration.go
package automation

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/automation"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/log"
)

// AutomationIntegration provides helper methods for other services to integrate with automation
type AutomationIntegration struct {
	service    automation.Service
	dispatcher queue.Dispatcher
	logger     log.Logger
}

// NewAutomationIntegration creates a new automation integration helper
func NewAutomationIntegration(
	service automation.Service,
	dispatcher queue.Dispatcher,
	logger log.Logger,
) *AutomationIntegration {
	return &AutomationIntegration{
		service:    service,
		dispatcher: dispatcher,
		logger:     logger,
	}
}

// TriggerEvent is a convenience method for other services to trigger automation events
func (a *AutomationIntegration) TriggerEvent(ctx context.Context, eventType string, data map[string]any, actorID *uuid.UUID) error {
	event := automation.Event{
		ID:        uuid.New(),
		Type:      eventType,
		Source:    "system",
		Timestamp: time.Now(),
		Data:      data,
		ActorID:   actorID,
	}

	// Queue the event for async processing
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	if _, err := a.dispatcher.Dispatch(ctx, (automation.JobAutomationProcessEvent), payload); err != nil {
		return fmt.Errorf("failed to dispatch automation event: %w", err)
	}

	a.logger.Debug("automation event triggered",
		log.String("event_type", eventType),
		log.String("event_id", event.ID.String()),
	)

	return nil
}

// Common event trigger helpers for standard platform events

// TriggerUserRegistered triggers automation when a user registers
func (a *AutomationIntegration) TriggerUserRegistered(ctx context.Context, userID uuid.UUID, email, name string) error {
	return a.TriggerEvent(ctx, "user.registered", map[string]any{
		"user_id": userID.String(),
		"email":   email,
		"name":    name,
	}, &userID)
}

// TriggerDonationReceived triggers automation when a donation is received
func (a *AutomationIntegration) TriggerDonationReceived(ctx context.Context, donationID uuid.UUID, donorID uuid.UUID, amount float64, currency string) error {
	return a.TriggerEvent(ctx, "donation.received", map[string]any{
		"donation_id": donationID.String(),
		"donor_id":    donorID.String(),
		"amount":      amount,
		"currency":    currency,
	}, &donorID)
}

// TriggerProgramApplication triggers automation when someone applies to a program
func (a *AutomationIntegration) TriggerProgramApplication(ctx context.Context, applicationID uuid.UUID, programID uuid.UUID, applicantID uuid.UUID) error {
	return a.TriggerEvent(ctx, "program.application", map[string]any{
		"application_id": applicationID.String(),
		"program_id":     programID.String(),
		"applicant_id":   applicantID.String(),
	}, &applicantID)
}

// TriggerVolunteerRegistration triggers automation when a volunteer registers
func (a *AutomationIntegration) TriggerVolunteerRegistration(ctx context.Context, volunteerID uuid.UUID, userID uuid.UUID, skills []string) error {
	return a.TriggerEvent(ctx, "volunteer.registered", map[string]any{
		"volunteer_id": volunteerID.String(),
		"user_id":      userID.String(),
		"skills":       skills,
	}, &userID)
}

// TriggerEventRegistration triggers automation when someone registers for an event
func (a *AutomationIntegration) TriggerEventRegistration(ctx context.Context, registrationID uuid.UUID, eventID uuid.UUID, attendeeID uuid.UUID) error {
	return a.TriggerEvent(ctx, "event.registration", map[string]any{
		"registration_id": registrationID.String(),
		"event_id":        eventID.String(),
		"attendee_id":     attendeeID.String(),
	}, &attendeeID)
}

// TriggerTaskAssigned triggers automation when a task is assigned
func (a *AutomationIntegration) TriggerTaskAssigned(ctx context.Context, taskID uuid.UUID, assigneeID uuid.UUID, assignerID uuid.UUID) error {
	return a.TriggerEvent(ctx, "task.assigned", map[string]any{
		"task_id":     taskID.String(),
		"assignee_id": assigneeID.String(),
		"assigner_id": assignerID.String(),
	}, &assignerID)
}

// TriggerMilestoneAchieved triggers automation when a milestone is achieved
func (a *AutomationIntegration) TriggerMilestoneAchieved(ctx context.Context, milestoneID uuid.UUID, entityType string, entityID uuid.UUID, metadata map[string]any) error {
	data := map[string]any{
		"milestone_id": milestoneID.String(),
		"entity_type":  entityType,
		"entity_id":    entityID.String(),
	}

	// Merge metadata
	for k, v := range metadata {
		data[k] = v
	}

	return a.TriggerEvent(ctx, "milestone.achieved", data, nil)
}

// TriggerContentPublished triggers automation when content is published
func (a *AutomationIntegration) TriggerContentPublished(ctx context.Context, contentID uuid.UUID, contentType string, authorID uuid.UUID, title string) error {
	return a.TriggerEvent(ctx, "content.published", map[string]any{
		"content_id":   contentID.String(),
		"content_type": contentType,
		"author_id":    authorID.String(),
		"title":        title,
	}, &authorID)
}

// TriggerPaymentFailed triggers automation when a payment fails
func (a *AutomationIntegration) TriggerPaymentFailed(ctx context.Context, paymentID uuid.UUID, userID uuid.UUID, amount float64, reason string) error {
	return a.TriggerEvent(ctx, "payment.failed", map[string]any{
		"payment_id": paymentID.String(),
		"user_id":    userID.String(),
		"amount":     amount,
		"reason":     reason,
	}, &userID)
}

// TriggerThresholdExceeded triggers automation when a threshold is exceeded
func (a *AutomationIntegration) TriggerThresholdExceeded(ctx context.Context, metricName string, currentValue float64, threshold float64, metadata map[string]any) error {
	data := map[string]any{
		"metric_name":   metricName,
		"current_value": currentValue,
		"threshold":     threshold,
	}

	// Merge metadata
	for k, v := range metadata {
		data[k] = v
	}

	return a.TriggerEvent(ctx, "threshold.exceeded", data, nil)
}

// TriggerScheduledEvent triggers automation for scheduled/cron events
func (a *AutomationIntegration) TriggerScheduledEvent(ctx context.Context, scheduleID string, scheduleName string) error {
	return a.TriggerEvent(ctx, "schedule.triggered", map[string]any{
		"schedule_id":   scheduleID,
		"schedule_name": scheduleName,
	}, nil)
}

// CreateStandardRules creates a set of standard automation rules for common scenarios
func (a *AutomationIntegration) CreateStandardRules(ctx context.Context, createdBy uuid.UUID) error {
	rules := []automation.CreateRuleParams{
		// Welcome new users
		{
			Name:        "Welcome New Users",
			Description: "Send welcome email to newly registered users",
			Priority:    90,
			Trigger: automation.Trigger{
				Type: "user.registered",
			},
			Logic: automation.ConditionLogicAND,
			Actions: []automation.Action{
				{
					Type: "send_email",
					Config: map[string]any{
						"to":      "{{.Data.email}}",
						"subject": "Welcome to the NGO Platform!",
						"body":    "Hello {{.Data.name}}, welcome to our platform!",
					},
				},
			},
		},
		// Thank donors
		{
			Name:        "Thank Donors",
			Description: "Send thank you email for donations over $50",
			Priority:    80,
			Trigger: automation.Trigger{
				Type: "donation.received",
			},
			Conditions: []automation.Condition{
				{
					Field:    "amount",
					Operator: "gte",
					Value:    50.0,
				},
			},
			Logic: automation.ConditionLogicAND,
			Actions: []automation.Action{
				{
					Type: "send_email",
					Config: map[string]any{
						"to":      "{{.Data.donor_email}}",
						"subject": "Thank You for Your Generous Donation",
						"body":    "Thank you for your donation of ${{.Data.amount}}!",
					},
				},
			},
		},
		// Notify on large donations
		{
			Name:        "Alert on Large Donations",
			Description: "Notify admin team when donation exceeds $1000",
			Priority:    100,
			Trigger: automation.Trigger{
				Type: "donation.received",
			},
			Conditions: []automation.Condition{
				{
					Field:    "amount",
					Operator: "gt",
					Value:    1000.0,
				},
			},
			Logic: automation.ConditionLogicAND,
			Actions: []automation.Action{
				{
					Type: "send_notification",
					Config: map[string]any{
						"user_id": "admin",
						"message": "Large donation received: ${{.Data.amount}}",
					},
				},
				{
					Type: "log",
					Config: map[string]any{
						"level":   "info",
						"message": "Large donation: ${{.Data.amount}} from {{.Data.donor_id}}",
					},
				},
			},
		},
	}

	for _, ruleReq := range rules {
		if _, err := a.service.CreateRule(ctx, ruleReq); err != nil {
			a.logger.Error("failed to create standard rule", err,
				log.String("rule_name", ruleReq.Name),
			)
			// Continue creating other rules
		}
	}

	return nil
}
