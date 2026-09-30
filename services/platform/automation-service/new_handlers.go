package automation

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/content"
	"github.com/combi/pkg/domain/impact"
	"github.com/combi/pkg/domain/notification"
	"github.com/combi/pkg/domain/project"
	"github.com/combi/pkg/domain/reporting"
	"github.com/combi/pkg/log"
)

// --- UpdateImpactMetricHandler ---
type UpdateImpactMetricHandler struct {
	impactSvc impact.Service
	logger    log.Logger
}

func NewUpdateImpactMetricHandler(impactSvc impact.Service, logger log.Logger) *UpdateImpactMetricHandler {
	return &UpdateImpactMetricHandler{impactSvc: impactSvc, logger: logger}
}

func (h *UpdateImpactMetricHandler) Type() string { return "update_impact_metric" }

func (h *UpdateImpactMetricHandler) Validate(ctx context.Context, config map[string]any) error {
	if _, ok := config["metric_key"]; !ok {
		return fmt.Errorf("'metric_key' is required")
	}
	if _, ok := config["value"]; !ok {
		return fmt.Errorf("'value' is required")
	}
	return nil
}

func (h *UpdateImpactMetricHandler) Execute(ctx context.Context, config map[string]any, data map[string]any, idempotencyKey string) (map[string]any, error) {
	metricKey, err := RenderTemplate(config["metric_key"], data)
	if err != nil {
		return nil, fmt.Errorf("failed to render metric_key: %w", err)
	}

	valStr, err := RenderTemplate(config["value"], data)
	if err != nil {
		return nil, fmt.Errorf("failed to render value: %w", err)
	}

	val, err := strconv.ParseFloat(valStr, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid impact value '%s': %w", valStr, err)
	}

	var prjID *uuid.UUID
	if prjRaw, ok := config["project_id"]; ok {
		prjStr, err := RenderTemplate(prjRaw, data)
		if err == nil && prjStr != "" {
			if id, err := uuid.Parse(prjStr); err == nil {
				prjID = &id
			}
		}
	}

	entry := impact.ImpactEntry{
		ID:            uuid.New(),
		Value:         val,
		OccurredAt:    time.Now().UTC(),
		SourceService: "automation-service",
		SourceID:      idempotencyKey,
		ProjectID:     prjID,
		Metadata:      data,
	}

	metric, err := h.impactSvc.GetMetricByKey(ctx, metricKey)
	if err != nil {
		return nil, fmt.Errorf("impact metric '%s' not found: %w", metricKey, err)
	}
	entry.MetricID = metric.ID

	if err := h.impactSvc.RecordImpact(ctx, entry); err != nil {
		return nil, fmt.Errorf("failed to record impact entry: %w", err)
	}

	return map[string]any{
		"entry_id":   entry.ID.String(),
		"metric_key": metricKey,
		"value":      val,
	}, nil
}

// --- CreateTaskHandler ---
type CreateTaskHandler struct {
	projectSvc project.Service
	logger     log.Logger
}

func NewCreateTaskHandler(projectSvc project.Service, logger log.Logger) *CreateTaskHandler {
	return &CreateTaskHandler{projectSvc: projectSvc, logger: logger}
}

func (h *CreateTaskHandler) Type() string { return "create_task" }

func (h *CreateTaskHandler) Validate(ctx context.Context, config map[string]any) error {
	if _, ok := config["project_id"]; !ok {
		return fmt.Errorf("'project_id' is required")
	}
	if _, ok := config["title"]; !ok {
		return fmt.Errorf("'title' is required")
	}
	return nil
}

func (h *CreateTaskHandler) Execute(ctx context.Context, config map[string]any, data map[string]any, idempotencyKey string) (map[string]any, error) {
	prjStr, err := RenderTemplate(config["project_id"], data)
	if err != nil {
		return nil, fmt.Errorf("failed to render project_id: %w", err)
	}
	prjID, err := uuid.Parse(prjStr)
	if err != nil {
		return nil, fmt.Errorf("invalid project_id UUID '%s': %w", prjStr, err)
	}

	title, err := RenderTemplate(config["title"], data)
	if err != nil {
		return nil, fmt.Errorf("failed to render title: %w", err)
	}

	descStr := ""
	if descRaw, ok := config["description"]; ok {
		descStr, _ = RenderTemplate(descRaw, data)
	}

	var assigneeID *uuid.UUID
	if assRaw, ok := config["assignee_id"]; ok {
		assStr, err := RenderTemplate(assRaw, data)
		if err == nil && assStr != "" {
			if id, err := uuid.Parse(assStr); err == nil {
				assigneeID = &id
			}
		}
	}

	params := project.CreateTaskParams{
		CreatorID:   uuid.Nil, // System creator
		ProjectID:   prjID,
		Title:       title,
		Description: &descStr,
		AssigneeID:  assigneeID,
		Priority:    1,
	}

	task, err := h.projectSvc.CreateTask(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create project task: %w", err)
	}

	return map[string]any{
		"task_id":    task.ID.String(),
		"task_title": task.Title,
	}, nil
}

// --- PostNewsFeedHandler ---
type PostNewsFeedHandler struct {
	contentSvc content.Service
	logger     log.Logger
}

func NewPostNewsFeedHandler(contentSvc content.Service, logger log.Logger) *PostNewsFeedHandler {
	return &PostNewsFeedHandler{contentSvc: contentSvc, logger: logger}
}

func (h *PostNewsFeedHandler) Type() string { return "post_news_feed" }

func (h *PostNewsFeedHandler) Validate(ctx context.Context, config map[string]any) error {
	if _, ok := config["space_id"]; !ok {
		return fmt.Errorf("'space_id' is required")
	}
	if _, ok := config["model_id"]; !ok {
		return fmt.Errorf("'model_id' is required")
	}
	if _, ok := config["title"]; !ok {
		return fmt.Errorf("'title' is required")
	}
	return nil
}

func (h *PostNewsFeedHandler) Execute(ctx context.Context, config map[string]any, data map[string]any, idempotencyKey string) (map[string]any, error) {
	spaceStr, err := RenderTemplate(config["space_id"], data)
	if err != nil {
		return nil, err
	}
	spaceID, err := uuid.Parse(spaceStr)
	if err != nil {
		return nil, err
	}

	modelStr, err := RenderTemplate(config["model_id"], data)
	if err != nil {
		return nil, err
	}
	modelID, err := uuid.Parse(modelStr)
	if err != nil {
		return nil, err
	}

	title, err := RenderTemplate(config["title"], data)
	if err != nil {
		return nil, err
	}

	bodyStr := ""
	if bodyRaw, ok := config["body"]; ok {
		bodyStr, _ = RenderTemplate(bodyRaw, data)
	}

	params := content.CreateEntryParams{
		ModelID: modelID,
		SpaceID: spaceID,
		Fields: map[string]map[string]any{
			"en": {
				"title": title,
				"body":  bodyStr,
			},
		},
	}

	entry, err := h.contentSvc.CreateEntry(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create content entry: %w", err)
	}

	return map[string]any{
		"entry_id": entry.ID.String(),
		"status":   entry.Status,
	}, nil
}

// --- GenerateAIReportHandler ---
type GenerateAIReportHandler struct {
	reportingSvc reporting.Service
	logger       log.Logger
}

func NewGenerateAIReportHandler(reportingSvc reporting.Service, logger log.Logger) *GenerateAIReportHandler {
	return &GenerateAIReportHandler{reportingSvc: reportingSvc, logger: logger}
}

func (h *GenerateAIReportHandler) Type() string { return "generate_ai_report" }

func (h *GenerateAIReportHandler) Validate(ctx context.Context, config map[string]any) error {
	if _, ok := config["template_slug"]; !ok {
		return fmt.Errorf("'template_slug' is required")
	}
	if _, ok := config["requester_id"]; !ok {
		return fmt.Errorf("'requester_id' is required")
	}
	return nil
}

func (h *GenerateAIReportHandler) Execute(ctx context.Context, config map[string]any, data map[string]any, idempotencyKey string) (map[string]any, error) {
	slug, err := RenderTemplate(config["template_slug"], data)
	if err != nil {
		return nil, err
	}

	reqStr, err := RenderTemplate(config["requester_id"], data)
	if err != nil {
		return nil, err
	}
	reqID, err := uuid.Parse(reqStr)
	if err != nil {
		return nil, err
	}

	req := reporting.ReportRequest{
		TemplateSlug: slug,
		RequesterID:  reqID,
		Parameters:   data,
		Format:       reporting.FormatPDF,
	}

	report, err := h.reportingSvc.RequestReport(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("report generation request failed: %w", err)
	}

	return map[string]any{
		"report_id": report.ID.String(),
		"status":    report.Status,
	}, nil
}

// --- SendPersonalNoteHandler ---
type SendPersonalNoteHandler struct {
	notificationSvc notification.Service
	logger          log.Logger
}

func NewSendPersonalNoteHandler(notificationSvc notification.Service, logger log.Logger) *SendPersonalNoteHandler {
	return &SendPersonalNoteHandler{notificationSvc: notificationSvc, logger: logger}
}

func (h *SendPersonalNoteHandler) Type() string { return "send_personal_note" }

func (h *SendPersonalNoteHandler) Validate(ctx context.Context, config map[string]any) error {
	if _, ok := config["user_id"]; !ok {
		return fmt.Errorf("'user_id' is required")
	}
	if _, ok := config["body"]; !ok {
		return fmt.Errorf("'body' is required")
	}
	return nil
}

func (h *SendPersonalNoteHandler) Execute(ctx context.Context, config map[string]any, data map[string]any, idempotencyKey string) (map[string]any, error) {
	userStr, err := RenderTemplate(config["user_id"], data)
	if err != nil {
		return nil, err
	}
	userID, err := uuid.Parse(userStr)
	if err != nil {
		return nil, err
	}

	body, err := RenderTemplate(config["body"], data)
	if err != nil {
		return nil, err
	}

	// Dispatch a personalized in-app / push alert
	req := notification.DispatchRequest{
		RecipientID: userID,
		Reference: &notification.NotificationRef{
			EntityType: "personal_note",
			EntityID:   uuid.New(),
		},
		Raw: &notification.RawPayload{
			Channel:  notification.ChannelPush,
			Category: "personal_note",
			Body:     body,
		},
	}

	notif, err := h.notificationSvc.Dispatch(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("personal note notification failed: %w", err)
	}

	return map[string]any{
		"notification_id": notif.ID.String(),
		"status":          notif.Status,
	}, nil
}
