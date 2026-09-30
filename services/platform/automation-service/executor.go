// internal/services/automation/executor.go
package automation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/automation"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/notification"
	"github.com/combi/pkg/log"
)

var _ automation.ActionExecutor = (*actionExecutor)(nil)

type actionExecutor struct {
	logger   log.Logger
	handlers map[string]automation.ActionHandler // Use the plugin interface here
}

func NewActionExecutor(logger log.Logger) *actionExecutor {
	return &actionExecutor{
		logger:   logger,
		handlers: make(map[string]automation.ActionHandler),
	}
}

func (e *actionExecutor) RegisterHandler(handler automation.ActionHandler) {
	if handler == nil || handler.Type() == "" {
		return
	}
	e.logger.Info("registering automation action handler", log.String("type", handler.Type()))
	e.handlers[handler.Type()] = handler
}

// Execute implements automation.ActionExecutor
func (e *actionExecutor) Execute(ctx context.Context, action automation.Action, eventData map[string]any, idempotencyKey string) (map[string]any, error) {
	handler, exists := e.handlers[action.Type]
	if !exists {
		return nil, fmt.Errorf("unsupported action type: %s", action.Type)
	}

	e.logger.Debug("executing action via handler", log.String("action_type", action.Type))

	// Pass idempotencyKey down to the plugin
	return handler.Execute(ctx, action.Config, eventData, idempotencyKey)
}

// Validate implements automation.ActionExecutor
func (e *actionExecutor) Validate(ctx context.Context, action automation.Action) error {
	if action.Type == "" {
		return fmt.Errorf("action type is required")
	}

	handler, exists := e.handlers[action.Type]
	if !exists {
		return fmt.Errorf("unsupported action type: %s", action.Type)
	}

	return handler.Validate(ctx, action.Config)
}

func (e *actionExecutor) GetSupportedActions() []string {
	keys := make([]string, 0, len(e.handlers))
	for k := range e.handlers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// --- NotificationHandler (Universal: Email, SMS, Push, System) ---
// This replaces the redundancy. It delegates EVERYTHING to the Notification Service.
type NotificationHandler struct {
	service notification.Service
	channel notification.NotificationChannel // Optional: locks this handler to a specific channel (e.g. EMAIL)
}

func NewNotificationHandler(service notification.Service, channel notification.NotificationChannel) *NotificationHandler {
	return &NotificationHandler{
		service: service,
		channel: channel,
	}
}

// Type returns the action type string.
// If channel is set, it returns "send_email" / "send_sms".
// If channel is empty, it returns "send_notification" (generic).
func (h *NotificationHandler) Type() string {
	if h.channel != "" {
		return fmt.Sprintf("send_%s", strings.ToLower(string(h.channel)))
	}
	return "send_notification"
}

func (h *NotificationHandler) Validate(ctx context.Context, config map[string]any) error {
	// 1. Validate Recipient
	if _, ok := config["user_id"]; !ok {
		// If not sending to a system user ID, we might allow direct address (email/phone)
		if _, okEmail := config["to_address"]; !okEmail {
			return fmt.Errorf("either 'user_id' or 'to_address' is required")
		}
	}

	// 2. Validate Content (Template OR Raw)
	hasTemplate := false
	if _, ok := config["template_slug"]; ok {
		hasTemplate = true
	}

	hasRaw := false
	if _, ok := config["body"]; ok {
		hasRaw = true
	}

	if !hasTemplate && !hasRaw {
		return fmt.Errorf("must provide either 'template_slug' (recommended) or 'body' (raw content)")
	}

	// 3. Validate Subject if Email and Raw
	if h.channel == notification.ChannelEmail && hasRaw {
		if _, ok := config["subject"]; !ok {
			return fmt.Errorf("'subject' is required for raw email actions")
		}
	}

	return nil
}

func (h *NotificationHandler) Execute(ctx context.Context, config map[string]any, data map[string]any, idempotencyKey string) (map[string]any, error) {
	// 1. Render Input Data
	// We allow the Automation Rule to define dynamic values (e.g. {{.Data.email}})

	// Recipient (User ID takes precedence)
	var recipientID uuid.UUID
	var toAddress string

	if uidRaw, ok := config["user_id"]; ok {
		uidStr, err := RenderTemplate(uidRaw, data)
		if err != nil {
			return nil, fmt.Errorf("failed to render user_id: %w", err)
		}
		if uidStr != "" {
			recipientID, err = uuid.Parse(uidStr)
			if err != nil {
				return nil, fmt.Errorf("invalid user_id UUID: %s", uidStr)
			}
		}
	}

	if recipientID == uuid.Nil {
		// Try direct address
		if addrRaw, ok := config["to_address"]; ok {
			var err error
			toAddress, err = RenderTemplate(addrRaw, data)
			if err != nil {
				return nil, fmt.Errorf("failed to render to_address: %w", err)
			}
		}
	}

	// 2. Construct Dispatch Request
	req := notification.DispatchRequest{
		RecipientID: recipientID, // Can be Nil if ToAddress is set (depending on Notification Service logic)
		Reference: &notification.NotificationRef{
			EntityType: "automation_execution",
			// We use the Idempotency Key (ExecutionID + Index) as the Entity ID.
			// However, EntityID must be a UUID. The Idempotency Key is a string (UUID-Index).
			// To be safe, we assume the Automation Service passes the Execution UUID separately
			// or we generate a deterministic UUID based on the idempotency key string.
			// For now, let's assume the Notification Service accepts the raw string reference or we hash it.
			// Since NotificationRef.EntityID is UUID, we can't put the full key there directly.
			// Simplified: We skip linking specific Action Index, just link Execution ID.
			EntityID: uuid.MustParse(strings.Split(idempotencyKey, "-")[0]),
		},
	}

	if toAddress != "" {
		email, _ := core.NewEmail(toAddress)
		req.ToAddress = core.Email(email)
	}

	// 3. Payload (Template vs Raw)
	if slugRaw, ok := config["template_slug"]; ok {
		slug, err := RenderTemplate(slugRaw, data)
		if err != nil {
			return nil, err
		}

		req.Template = &notification.TemplatePayload{
			Slug: slug,
			Data: data, // Pass the event data to the notification template
		}
	} else {
		// Raw Payload
		body, err := RenderTemplate(config["body"], data)
		if err != nil {
			return nil, err
		}

		var subject *string
		if subjRaw, ok := config["subject"]; ok {
			s, err := RenderTemplate(subjRaw, data)
			if err != nil {
				return nil, err
			}
			subject = &s
		}

		// Determine channel
		channel := h.channel
		if channel == "" {
			// If generic handler, look for channel in config
			if c, ok := config["channel"].(string); ok {
				channel = notification.NotificationChannel(strings.ToUpper(c))
			} else {
				return nil, fmt.Errorf("channel is required for raw notifications")
			}
		}

		req.Raw = &notification.RawPayload{
			Channel:  channel,
			Category: "automation", // Default category
			Subject:  subject,
			Body:     body,
		}
	}

	// 4. Dispatch via Notification Service
	notif, err := h.service.Dispatch(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("notification dispatch failed: %w", err)
	}

	return map[string]any{
		"notification_id": notif.ID.String(),
		"status":          notif.Status,
		"channel":         notif.Channel,
	}, nil
}

// --- WebhookHandler (Remains mostly the same, ensuring Idempotency Headers) ---
type WebhookHandler struct {
	httpClient *http.Client
}

func NewWebhookHandler(client *http.Client) *WebhookHandler {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &WebhookHandler{httpClient: client}
}

func (h *WebhookHandler) Type() string { return "webhook" }

func (h *WebhookHandler) Validate(ctx context.Context, config map[string]any) error {
	if _, ok := config["url"]; !ok {
		return fmt.Errorf("'url' is required")
	}
	return nil
}

func (h *WebhookHandler) Execute(ctx context.Context, config map[string]any, data map[string]any, idempotencyKey string) (map[string]any, error) {
	url, err := RenderTemplate(config["url"], data)
	if err != nil {
		return nil, err
	}

	// Security Check
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return nil, errors.New("webhook url must start with http:// or https://")
	}

	method := "POST"
	if m, ok := config["method"].(string); ok && m != "" {
		method = strings.ToUpper(m)
	}

	// Payload construction
	payload := data
	if p, ok := config["payload"].(map[string]any); ok {
		payload = p
	}
	jsonPayload, err := json.Marshal(CleanData(payload))
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(jsonPayload))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "NGO-Automation/1.0")

	// FIX: Pass Idempotency Key to external system
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
		req.Header.Set("X-Idempotency-Key", idempotencyKey)
	}

	// Custom Headers
	if headers, ok := config["headers"].(map[string]any); ok {
		for key, value := range headers {
			if strValue, ok := value.(string); ok {
				if rendered, err := RenderTemplate(strValue, data); err == nil {
					req.Header.Set(key, rendered)
				}
			}
		}
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("webhook request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read limited response
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024*10)) // 10KB limit for logs

	output := map[string]any{
		"status_code": resp.StatusCode,
		"response":    string(body),
	}

	if resp.StatusCode >= 400 {
		return output, fmt.Errorf("webhook returned error: %d", resp.StatusCode)
	}

	return output, nil
}

// CleanData recursively traverses interface{} and ensures only JSON-safe primitives exist.
// This strips out structs, functions, and channels.
func CleanData(input any) any {
	if input == nil {
		return nil
	}

	switch v := input.(type) {
	// Primitives are safe
	case string, int, int64, float64, bool:
		return v
	case uuid.UUID:
		return v.String()
	case time.Time:
		return v.Format(time.RFC3339)

	// Collections need traversal
	case map[string]any:
		cleaned := make(map[string]any, len(v))
		for k, val := range v {
			cleaned[k] = CleanData(val)
		}
		return cleaned

	case []any:
		cleaned := make([]any, len(v))
		for i, val := range v {
			cleaned[i] = CleanData(val)
		}
		return cleaned

	// If it's a specific struct type that we missed, fail safe by converting to string or ignoring
	// Ideally, unmarshaling from JSON prevents this, but internal calls might pass structs.
	// We use JSON round-tripping as the nuclear option for complex unknown types.
	default:
		// Attempt to JSON encode/decode to strip methods/unsafe types
		b, err := json.Marshal(input)
		if err != nil {
			return fmt.Sprintf("<unsafe_type:%T>", input)
		}
		var res any
		_ = json.Unmarshal(b, &res)
		return res
	}
}

// --- LogHandler ---
type LogHandler struct {
	logger log.Logger
}

func NewLogHandler(logger log.Logger) *LogHandler {
	return &LogHandler{logger: logger}
}
func (h *LogHandler) Type() string { return "log" }
func (h *LogHandler) Validate(ctx context.Context, config map[string]any) error {
	if _, ok := config["message"]; !ok {
		return fmt.Errorf("'message' field is required for log action")
	}
	if level, ok := config["level"].(string); ok {
		switch level {
		case "debug", "info", "warn", "error":
			// valid
		default:
			return fmt.Errorf("invalid log level: '%s'. Must be one of: debug, info, warn, error", level)
		}
	}
	return nil
}

func (h *LogHandler) Execute(ctx context.Context, config map[string]any, data map[string]any, idempotencyKey string) (map[string]any, error) {
	message, err := RenderTemplate(config["message"], data)
	if err != nil {
		return nil, fmt.Errorf("failed to render 'message' field: %w", err)
	}

	level := "info" // Default level
	if l, ok := config["level"].(string); ok {
		level = l
	}

	// Optionally log the idempotency key for debugging tracing
	logFields := []log.Attr{
		log.String("automation_action_type", "log"),
		log.String("idempotency_key", idempotencyKey),
	}

	switch level {
	case "debug":
		h.logger.Debug(message, logFields...)
	case "info":
		h.logger.Info(message, logFields...)
	case "warn":
		h.logger.Warn(message, logFields...)
	case "error":
		h.logger.Error(message, nil, logFields...)
	}

	output := map[string]any{
		"logged_at": time.Now().UTC().Format(time.RFC3339),
		"level":     level,
		"message":   message,
	}
	return output, nil
}

// --- DelayHandler ---
type DelayHandler struct{}

func NewDelayHandler() *DelayHandler {
	return &DelayHandler{}
}

func (h *DelayHandler) Type() string { return "delay" }
func (h *DelayHandler) Validate(ctx context.Context, config map[string]any) error {
	durationStr, ok := config["duration"].(string)
	if !ok {
		return fmt.Errorf("'duration' field is required and must be a string for delay action (e.g., '5s', '1m')")
	}
	if _, err := time.ParseDuration(durationStr); err != nil {
		return fmt.Errorf("invalid duration format: %w", err)
	}
	return nil
}

// Execute for DelayHandler returns the specific Pause error
func (h *DelayHandler) Execute(ctx context.Context, config map[string]any, data map[string]any, idempotencyKey string) (map[string]any, error) {
	durationStr, _ := config["duration"].(string)
	// Validate again just in case
	if _, err := time.ParseDuration(durationStr); err != nil {
		return nil, fmt.Errorf("invalid duration: %w", err)
	}

	output := map[string]any{
		"status":          "PAUSING",
		"resume_after":    durationStr,
		"idempotency_key": idempotencyKey, // useful for debugging resume logic
	}
	// This specific error signals the Service layer to stop the loop and schedule a continuation
	return output, automation.ErrWorkflowPause
}

func RenderTemplate(templateStr any, data map[string]any) (string, error) {
	str, ok := templateStr.(string)
	if !ok {
		return fmt.Sprintf("%v", templateStr), nil
	}
	if str == "" {
		return "", nil
	}
	if !strings.Contains(str, "{{") {
		return str, nil
	}

	safeData := CleanData(data)

	templateData := map[string]any{
		"Data": safeData,
	}

	tmpl, err := templateCache.GetOrParse(str)
	if err != nil {
		return "", err // Parse error
	}

	if tmpl == nil {
		return str, nil
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, templateData); err != nil {
		if strings.Contains(err.Error(), "map has no entry for key") {
			return "", fmt.Errorf("template reference error: %w", err)
		}
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}
