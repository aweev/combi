package configuration

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/apperror"
	"github.com/google/uuid"
)

// --- Errors ---
// Using platform/apperror for consistent error handling across the monorepo.
var (
	ErrSettingNotFound = apperror.New(
		http.StatusNotFound,
		"SETTING_NOT_FOUND",
		"The requested system setting could not be found.",
		nil,
	)
	ErrThemeNotFound = apperror.New(
		http.StatusNotFound,
		"THEME_NOT_FOUND",
		"The requested theme definition could not be found.",
		nil,
	)
	ErrBrandNotFound = apperror.New(
		http.StatusNotFound,
		"BRAND_NOT_FOUND",
		"Brand configuration not found for this context.",
		nil,
	)
	ErrInvalidConfig = apperror.New(
		http.StatusBadRequest,
		"INVALID_CONFIG",
		"The configuration value format is invalid or incompatible with the type.",
		nil,
	)
	ErrSystemThemeProtected = apperror.New(
		http.StatusForbidden,
		"SYSTEM_THEME_PROTECTED",
		"System themes cannot be deleted or modified directly.",
		nil,
	)
)

// --- Constants ---
const (
	PermRead   core.PermissionName = "config:read"
	PermManage core.PermissionName = "config:manage" // Super Admin only
)

// --- 1. System Settings (Logic Configuration) ---
// These control HOW the system works (e.g., limits, feature toggles, currency).

type SettingType string

const (
	TypeString  SettingType = "STRING"
	TypeBoolean SettingType = "BOOLEAN"
	TypeInteger SettingType = "INTEGER"
	TypeJSON    SettingType = "JSON"
)

type SystemSetting struct {
	Key         string      `json:"key" db:"key"`
	Value       any         `json:"value" db:"value"`
	Type        SettingType `json:"type" db:"type"`
	Description string      `json:"description" db:"description"`
	IsPublic    bool        `json:"is_public" db:"is_public"` // Included in ClientBootstrap?
	UpdatedAt   time.Time   `json:"updated_at" db:"updated_at"`
}

// --- 2. Themes (Visual Presets) ---
// A Theme is a NAMED COLLECTION of CSS variables.

type Theme struct {
	ID       uuid.UUID `json:"id" db:"id"`
	Name     string    `json:"name" db:"name"`
	Slug     string    `json:"slug" db:"slug"`
	IsSystem bool      `json:"is_system" db:"is_system"` // If true, cannot be deleted

	Priority int `json:"priority" db:"priority"`
	// Variables stores the raw CSS variable map.
	// DB: stored as jsonb. Code: map[string]string.
	Variables map[string]string `json:"variables" db:"variables"`

	// Scheduling (Optional): For Seasonal Themes (e.g., Pride Month, Winter Holidays)
	ActiveFrom  *time.Time `json:"active_from,omitempty" db:"active_from"`
	ActiveUntil *time.Time `json:"active_until,omitempty" db:"active_until"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// --- 3. Brand (Identity Application) ---
// A Brand connects an Entity (Org, Project, or System) to a Logo and a Theme.

type Brand struct {
	ID             uuid.UUID  `json:"id" db:"id"`
	OrganizationID *uuid.UUID `json:"organization_id,omitempty" db:"organization_id"` // Null = Global
	ProjectID      *uuid.UUID `json:"project_id,omitempty" db:"project_id"`           // Null = Global

	Name string `json:"name" db:"name"` // Internal Reference Name

	// Identity Assets
	LogoURL    string `json:"logo_url" db:"logo_url"`
	FaviconURL string `json:"favicon_url" db:"favicon_url"`

	// Visual Look
	ThemeID uuid.UUID `json:"theme_id" db:"theme_id"`

	// Overrides allow an Org to use a standard theme but change specific vars
	CustomOverrides map[string]string `json:"custom_overrides" db:"custom_overrides"`

	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// --- 4. The Output (DTO) ---

// ClientBootstrap is the merged configuration sent to the frontend on load.
type ClientBootstrap struct {
	// ResolvedSettings is a map of Key->Value for all IsPublic=true settings
	Settings map[string]any `json:"settings"`

	// ResolvedTheme is the flattened CSS variables map
	// Merged: Theme.Variables + Brand.CustomOverrides
	Theme map[string]string `json:"theme"`

	// Identity Assets
	BrandAssets BrandAssets `json:"assets"`

	// Contextual Data
	Translations map[string]string `json:"translations"`
	Features     map[string]any    `json:"features"`
	User         json.RawMessage   `json:"user,omitempty"` // Raw JSON of the user profile
}

type BrandAssets struct {
	LogoURL    string `json:"logo_url"`
	FaviconURL string `json:"favicon_url"`
}

type BootstrapParams struct {
	LocaleCode string
	OrgID      *uuid.UUID
	ProjectID  *uuid.UUID
	UserID     *uuid.UUID
}

// --- Interfaces ---

// Service defines the business logic for configuration.
// It also implements core.Registry to act as the central discovery point.
type Service interface {
	core.Registry // Embeds RegisterService and GetSystemCapabilities

	// --- Bootstrap ---
	GetBootstrap(ctx context.Context, params BootstrapParams) (*ClientBootstrap, error)

	// --- Settings ---
	GetSetting(ctx context.Context, key string) (any, error)
	UpdateSetting(ctx context.Context, key string, value any) error
	ListSettings(ctx context.Context) ([]SystemSetting, error)

	// --- Themes ---
	CreateTheme(ctx context.Context, theme Theme) (*Theme, error)
	UpdateTheme(ctx context.Context, theme Theme) (*Theme, error)
	ListThemes(ctx context.Context) ([]Theme, error)
	DeleteTheme(ctx context.Context, id uuid.UUID) error

	// --- Brands ---
	GetEffectiveBrand(ctx context.Context, orgID *uuid.UUID, projectID *uuid.UUID) (*Brand, error)
	UpsertBrand(ctx context.Context, brand Brand) (*Brand, error)
	ResetBrand(ctx context.Context, orgID *uuid.UUID) error
}

// Repository defines the persistence layer.
type Repository interface {
	// Settings
	GetSetting(ctx context.Context, key string) (*SystemSetting, error)
	ListSettings(ctx context.Context) ([]SystemSetting, error)
	UpdateSetting(ctx context.Context, s *SystemSetting) error

	// Themes
	UpsertTheme(ctx context.Context, t *Theme) error
	GetTheme(ctx context.Context, id uuid.UUID) (*Theme, error)
	ListThemes(ctx context.Context) ([]Theme, error)
	DeleteTheme(ctx context.Context, id uuid.UUID) error
	GetSeasonalTheme(ctx context.Context, now time.Time) (*Theme, error)

	// Brands
	GetBrand(ctx context.Context, orgID *uuid.UUID, projectID *uuid.UUID) (*Brand, error)
	GetSystemBrand(ctx context.Context) (*Brand, error)
	UpsertBrand(ctx context.Context, brand Brand) error
	DeleteBrand(ctx context.Context, id uuid.UUID) error
}
