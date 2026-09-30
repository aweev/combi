package notification

// DispatchConfig holds behavioral options for a dispatch call.
type DispatchConfig struct {
	FireAndForget bool
}

// DispatchOption is a function that configures a dispatch call.
type DispatchOption func(*DispatchConfig)

// FireAndForget is a dispatch option that makes the call non-blocking and suppresses errors.
func FireAndForget() DispatchOption {
	return func(c *DispatchConfig) {
		c.FireAndForget = true
	}
}

// RawContent defines the content for a non-templated notification.
type RawContent struct {
	Channel  NotificationChannel
	Category string
	Subject  *string // Optional, for email
	Body     string
}

// TemplateContent defines the content for a template-based notification.
type TemplateContent struct {
	Slug string
	Data map[string]any
}

// DispatchPayload is a unified struct for sending either a raw or templated notification.
// Exactly one of `Raw` or `Template` must be non-nil.
type DispatchPayload struct {
	Raw      *RawContent
	Template *TemplateContent
}
