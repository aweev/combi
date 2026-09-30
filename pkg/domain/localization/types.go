package localization

import (
	"context"
	"net/http"
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/apperror"
	"github.com/google/uuid"
)

// --- Errors ---
var (
	ErrInvalidLocale       = apperror.New(http.StatusBadRequest, "INVALID_LOCALE", "The requested locale code is invalid.", nil)
	ErrDuplicateKey        = apperror.New(http.StatusBadRequest, "DUPLICATE_KEY", "Translation key already exists.", nil)
	ErrKeyNotFound         = apperror.New(http.StatusNotFound, "TRANSLATION_KEY_NOT_FOUND", "Translation key not found.", nil)
	ErrLocaleNotFound      = apperror.New(http.StatusNotFound, "LOCALE_NOT_FOUND", "Locale not configured.", nil)
	ErrLocaleAlreadyExists = apperror.New(http.StatusConflict, "LOCALE_ALREADY_EXISTS", "Locale already exists.", nil)
)

// --- Constants ---
const (
	PermManage core.PermissionName = "localization:manage"
	PermRead   core.PermissionName = "localization:read"
)

type LocaleCode string

const (
	LocaleEnUS LocaleCode = "en-US"
	LocaleDeDE LocaleCode = "de-DE"
)

// --- Models ---
type Locale struct {
	Code       LocaleCode `json:"code" db:"code"`
	Name       string     `json:"name" db:"name"`
	IsDefault  bool       `json:"is_default" db:"is_default"`
	IsActive   bool       `json:"is_active" db:"is_active"`
	Direction  string     `json:"direction" db:"direction"` // "ltr", "rtl"
	DateFormat string     `json:"date_format" db:"date_format"`
	CreatedAt  time.Time  `json:"created_at" db:"created_at"`
}

type TranslationKey struct {
	ID          uuid.UUID `json:"id" db:"id"`
	Namespace   string    `json:"namespace" db:"namespace"`
	Key         string    `json:"key" db:"key"`
	Description string    `json:"description" db:"description"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

type TranslationValue struct {
	KeyID      uuid.UUID  `json:"key_id" db:"key_id"`
	Locale     LocaleCode `json:"locale" db:"locale"`
	Value      string     `json:"value" db:"value"`
	IsVerified bool       `json:"is_verified" db:"is_verified"`
	UpdatedAt  time.Time  `json:"updated_at" db:"updated_at"`
}

// --- Interfaces ---
type Service interface {
	GetTranslation(ctx context.Context, locale LocaleCode, namespace, key string) (string, error)

	GetBundle(ctx context.Context, locale LocaleCode, namespaces []string) (map[string]string, error)
	GetSupportedLocales(ctx context.Context) ([]Locale, error)
	// Admin
	CreateKey(ctx context.Context, key TranslationKey) (*TranslationKey, error)
	UpsertValue(ctx context.Context, val TranslationValue) error
	ListKeys(ctx context.Context, namespace string, page, limit int) ([]TranslationKey, int, error)
}

type Repository interface {
	GetLocales(ctx context.Context) ([]Locale, error)
	GetLocale(ctx context.Context, code LocaleCode) (*Locale, error)
	CreateKey(ctx context.Context, key *TranslationKey) error
	GetKey(ctx context.Context, namespace, key string) (*TranslationKey, error)
	ListKeys(ctx context.Context, namespace string, limit, offset int) ([]TranslationKey, int, error)
	DeleteKey(ctx context.Context, id uuid.UUID) error
	UpsertValue(ctx context.Context, val *TranslationValue) error
	GetValue(ctx context.Context, keyID uuid.UUID, locale LocaleCode) (*TranslationValue, error)
	GetValuesForNamespace(ctx context.Context, locale LocaleCode, namespace string) (map[string]string, error)
	GetKeysMissingValue(ctx context.Context, locale LocaleCode) ([]TranslationKey, error)
}
