// internal/domain/queue/options.go
package queue

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const (
	// MetaKeyUniqueKey is the key for the job's deduplication identifier.
	MetaKeyUniqueKey = "_unique_key"
	// MetaKeyUniqueWithin is the key for the deduplication time window in seconds.
	MetaKeyUniqueWithin = "_unique_within_seconds"
	// MetaKeyWebhookURL is the key for the URL to call on job completion/failure.
	MetaKeyWebhookURL = "_webhook_url"
)

// DispatchConfig holds all configurable parameters for a job dispatch.
type DispatchConfig struct {
	Queue         string
	Priority      int
	MaxAttempts   int
	Timeout       time.Duration
	Delay         time.Duration
	Tags          []string
	DependencyIDs []uuid.UUID
	Metadata      json.RawMessage
	UniqueKey     *string // If set, prevents duplicate jobs with this key within a time window
}

// DispatchOption is a function that configures a DispatchConfig.
type DispatchOption func(*DispatchConfig)

func WithQueue(queue string) DispatchOption {
	return func(c *DispatchConfig) { c.Queue = queue }
}

func WithPriority(priority int) DispatchOption {
	return func(c *DispatchConfig) { c.Priority = priority }
}

func WithMaxAttempts(attempts int) DispatchOption {
	return func(c *DispatchConfig) { c.MaxAttempts = attempts }
}

func WithTimeout(timeout time.Duration) DispatchOption {
	return func(c *DispatchConfig) { c.Timeout = timeout }
}

func WithDelay(delay time.Duration) DispatchOption {
	return func(c *DispatchConfig) { c.Delay = delay }
}

func WithTags(tags ...string) DispatchOption {
	return func(c *DispatchConfig) { c.Tags = append(c.Tags, tags...) }
}

func WithMetadata(metadata map[string]interface{}) DispatchOption {
	return func(c *DispatchConfig) {
		if data, err := json.Marshal(metadata); err == nil {
			c.Metadata = data
		}
	}
}

// WithUniqueKey prevents duplicate jobs with the same key from being enqueued
// within a specific time window. The underlying cache will handle the check.
// This key should be deterministic based on the job's context (e.g., "send-welcome-email:user-123").
func WithUniqueKey(key string, within time.Duration) DispatchOption {
	return func(c *DispatchConfig) {
		meta := make(map[string]interface{})
		if len(c.Metadata) > 0 {
			_ = json.Unmarshal(c.Metadata, &meta)
		}
		meta[MetaKeyUniqueKey] = key
		meta[MetaKeyUniqueWithin] = within.Seconds()

		if data, err := json.Marshal(meta); err == nil {
			c.Metadata = data
		}
	}
}

// WithWebhook specifies a URL to be called upon job completion or failure.
// The worker pool is responsible for triggering this webhook.
func WithWebhook(url string) DispatchOption {
	return func(c *DispatchConfig) {
		if url == "" {
			return
		}
		meta := make(map[string]interface{})
		if len(c.Metadata) > 0 {
			_ = json.Unmarshal(c.Metadata, &meta)
		}
		meta[MetaKeyWebhookURL] = url
		if data, err := json.Marshal(meta); err == nil {
			c.Metadata = data
		}
	}
}

// WithDependencies specifies one or more job IDs that must complete successfully
// before this job can be processed. The repository handles creating the dependency links.
func WithDependencies(jobIDs ...uuid.UUID) DispatchOption {
	return func(c *DispatchConfig) {
		if len(jobIDs) > 0 {
			c.DependencyIDs = append(c.DependencyIDs, jobIDs...)
		}
	}
}

// WithPayload sets the job's payload from a map, handling JSON marshaling.
// This is a convenience wrapper around manually marshaling the payload.
func WithPayload(payload map[string]interface{}) DispatchOption {
	return func(c *DispatchConfig) {
		// This option is intended to be used by the dispatcher,
		// which will marshal this into the final job's payload field.
		// For now, we'll store it in metadata to be picked up.
		meta := make(map[string]interface{})
		if len(c.Metadata) > 0 {
			_ = json.Unmarshal(c.Metadata, &meta)
		}
		meta["_payload"] = payload // Store it temporarily
		if data, err := json.Marshal(meta); err == nil {
			c.Metadata = data
		}
	}
}
