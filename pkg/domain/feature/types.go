package feature

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/apperror"
	"github.com/google/uuid"
)

// --- Constants ---
const (
	PermFeatureManage core.PermissionName = "feature:manage"
	PermFeatureRead   core.PermissionName = "feature:read"
)

var (
	// --- Feature Flag Errors ---

	// 404 Not Found
	ErrFlagNotFound = apperror.New(http.StatusNotFound, "FLAG_NOT_FOUND", "The requested feature flag could not be found.", nil)

	// 409 Conflict
	ErrFlagAlreadyExists = apperror.New(http.StatusConflict, "FLAG_ALREADY_EXISTS", "A feature flag with this key already exists.", nil)

	// 422 Unprocessable Entity (Semantic errors in the request)
	ErrInvalidRule      = apperror.New(http.StatusUnprocessableEntity, "INVALID_TARGETING_RULE", "An invalid targeting rule was provided.", nil)
	ErrInvalidValueType = apperror.New(http.StatusUnprocessableEntity, "INVALID_VALUE_TYPE", "The provided value does not match the flag's type.", nil)

	// 500 Internal Server Error
	ErrEvaluationFailed = apperror.New(http.StatusInternalServerError, "EVALUATION_FAILED", "The feature flag evaluation failed unexpectedly.", nil)

	// --- Branding Errors ---

	// 404 Not Found
	ErrBrandNotFound        = apperror.New(http.StatusNotFound, "BRAND_NOT_FOUND", "Brand configuration for the specified organization could not be found.", nil)
	ErrDefaultBrandNotFound = apperror.New(http.StatusInternalServerError, "DEFAULT_BRAND_NOT_CONFIGURED", "The default platform brand is not configured.", nil)

	// 422 Unprocessable Entity
	ErrInvalidColorFormat = apperror.New(http.StatusUnprocessableEntity, "INVALID_COLOR_FORMAT", "Invalid color format; must be a hex code (e.g., #RRGGBB).", nil)
	ErrInvalidLocale      = apperror.New(http.StatusUnprocessableEntity, "INVALID_LOCALE", "An invalid locale code was provided.", nil)
)

// --- Types ---
type FlagType string

const (
	TypeBoolean FlagType = "BOOLEAN"
	TypeString  FlagType = "STRING"
	TypeNumber  FlagType = "NUMBER"
	TypeJSON    FlagType = "JSON"
)

type Operator string

const (
	OpIs          Operator = "IS"
	OpIsNot       Operator = "IS_NOT"
	OpContains    Operator = "CONTAINS"
	OpStartsWith  Operator = "STARTS_WITH"
	OpEndsWith    Operator = "ENDS_WITH"
	OpNotContains Operator = "NOT_CONTAINS"
	OpIsIn        Operator = "IS_IN"
	OpIsNotIn     Operator = "IS_NOT_IN"
	OpGt          Operator = "GT"
	OpGte         Operator = "GTE"
	OpLt          Operator = "LT"
	OpLte         Operator = "LTE"
	OpSemverGt    Operator = "SEMVER_GT"
	OpSemverGte   Operator = "SEMVER_GTE"
	OpSemverLt    Operator = "SEMVER_LT"
	OpSemverLte   Operator = "SEMVER_LTE"
	OpSemverEq    Operator = "SEMVER_EQ"
)

// --- Core Entities ---

// EvaluationResult is the output of the engine.
type EvaluationResult struct {
	Key         string          `json:"key"`
	Value       json.RawMessage `json:"value"`
	VariationID string          `json:"variation_id"`
	RuleID      *string         `json:"rule_id,omitempty"`
}

// ClientBootstrap is the DTO for bulk flag evaluation.
type ClientBootstrap struct {
	Flags    map[string]EvaluationResult `json:"flags"`
	Settings map[string]interface{}      `json:"settings"`
}

type Flag struct {
	ID          uuid.UUID `json:"id"`
	Key         string    `json:"key"` // Unique lookup key
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Type        FlagType  `json:"type"`
	IsActive    bool      `json:"is_active"`
	Archived    bool      `json:"archived"`

	// Variations config
	OffVariation       json.RawMessage `json:"off_variation"`
	Variations         []Variation     `json:"variations"`
	DefaultVariationID string          `json:"default_variation_id"` // Fallback if active but no rules match
	OffVariationID     string          `json:"off_variation_id"`     // Fallback if inactive

	// Rules
	TargetingRules []TargetingRule `json:"targeting_rules"`
	DefaultRule    *Rollout        `json:"default_rule,omitempty"` // Optional complex default

	Version   int       `json:"version"` // Optimistic locking
	UpdatedAt time.Time `json:"updated_at"`
	UpdatedBy uuid.UUID `json:"updated_by"`
}

type Variation struct {
	ID    string          `json:"id"` // "on", "off", "red", "blue"
	Value json.RawMessage `json:"value"`
	Name  string          `json:"name"`
}

type TargetingRule struct {
	ID           string      `json:"id"`
	Priority     int         `json:"priority"` // Explicit ordering
	Conditions   []Condition `json:"conditions"`
	Distribution Rollout     `json:"distribution"` // Renamed from Rollout to Distribution for clarity
}

type Condition struct {
	ContextKey string          `json:"context_key"` // "email", "user_id"
	Operator   Operator        `json:"operator"`
	Value      json.RawMessage `json:"value"` // Single value or array depending on operator
	Negate     bool            `json:"negate"`
}

type Rollout struct {
	// Simple case: 100% to one variation
	VariationID *string `json:"variation_id,omitempty"`
	// Split case: Weighted
	Variations []WeightedVariation `json:"variations,omitempty"`
}

type WeightedVariation struct {
	VariationID string `json:"variation_id"`
	Weight      int    `json:"weight"` // 0-10000 (basis points)
}

// --- Evaluation Context ---

type EvaluationContext struct {
	// Primary lookup keys
	EntityID   string // UserID or SessionID
	EntityType string // "user", "anon"

	// Context attributes for rule matching
	Attributes map[string]any
}

// --- Service Interfaces ---

type Service interface {
	EvaluationService
	// Convenience methods
	IsEnabled(ctx context.Context, key string, context EvaluationContext, defaultVal bool) (bool, error)

	// Management (Admin)
	UpsertFlag(ctx context.Context, flag Flag) (*Flag, error)
	GetFlag(ctx context.Context, key string) (*Flag, error)
	ListFlags(ctx context.Context) ([]Flag, error)
	DeleteFlag(ctx context.Context, key string) error
}

// --- Infrastructure Interfaces ---

type Repository interface {
	Get(ctx context.Context, key string) (*Flag, error)
	ListAll(ctx context.Context) ([]Flag, error)
	Upsert(ctx context.Context, flag Flag) error
	Delete(ctx context.Context, key string) error
}

type FlagCacheReader interface {
	GetFlag(key string) *Flag
	ListActiveFlags() []*Flag
}

type CacheLifecycle interface {
	Start(ctx context.Context)
	Stop()
	Refresh(ctx context.Context, key string) // Manual refresh trigger
}

type Engine interface {
	Evaluate(flag *Flag, ctx EvaluationContext) EvaluationResult
}

type Notifier interface {
	NotifyFlagUpdated(ctx context.Context, key string) error
	Subscribe(ctx context.Context, onUpdate func(key string))
}

type EvaluationService interface {
	IsEnabled(ctx context.Context, key string, evalCtx EvaluationContext, defaultValue bool) (bool, error)
	GetStringValue(ctx context.Context, key string, evalCtx EvaluationContext, defaultValue string) (string, error)
	Evaluate(ctx context.Context, key string, evalCtx EvaluationContext) (EvaluationResult, error)
	GetClientBootstrapData(ctx context.Context, evalCtx EvaluationContext) (*ClientBootstrap, error)
}
