package localization

import (
	"context"
	"time"

	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/localization"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/cache"
	"github.com/combi/pkg/log"
)

var _ localization.Service = (*Service)(nil)

type Service struct {
	repo   localization.Repository
	cache  cache.Service
	authz  rbac.AuthorizationService
	audit  audit.EventRecorder
	logger log.Logger
}

func NewService(
	repo localization.Repository,
	cache cache.Service,
	authz rbac.AuthorizationService,
	audit audit.EventRecorder,
	logger log.Logger,
) *Service {
	return &Service{
		repo:   repo,
		cache:  cache,
		authz:  authz,
		audit:  audit,
		logger: logger.With(log.String("service", "localization")),
	}
}

// --- Public Methods ---

func (s *Service) GetSupportedLocales(ctx context.Context) ([]localization.Locale, error) {
	// Cached for 1 hour as this rarely changes
	var locales []localization.Locale
	cacheKey := "locales:all"

	found, _ := s.cache.GetObject(ctx, cacheKey, &locales)
	if found {
		return locales, nil
	}

	locales, err := s.repo.GetLocales(ctx)
	if err != nil {
		return nil, err
	}

	_ = s.cache.Set(ctx, cacheKey, locales, 1*time.Hour)
	return locales, nil
}

func (s *Service) GetTranslation(ctx context.Context, locale localization.LocaleCode, namespace, key string) (string, error) {
	// 1. Try specific locale
	keyDef, err := s.repo.GetKey(ctx, namespace, key)
	if err != nil {
		return key, nil // Fallback to key name if not found
	}

	val, err := s.repo.GetValue(ctx, keyDef.ID, locale)
	if err == nil && val != nil {
		return val.Value, nil
	}

	// 2. Fallback to Default Locale (usually en-US)
	// Optimization: We could cache the default locale code
	defaultLocale := localization.LocaleEnUS
	if locale != defaultLocale {
		val, err = s.repo.GetValue(ctx, keyDef.ID, defaultLocale)
		if err == nil && val != nil {
			return val.Value, nil
		}
	}

	// 3. Fallback to Key
	return key, nil
}

// GetBundle retrieves all translations for a set of namespaces.
// Optimized for Frontend bootstrapping.
func (s *Service) GetBundle(ctx context.Context, locale localization.LocaleCode, namespaces []string) (map[string]string, error) {
	bundle := make(map[string]string)

	for _, ns := range namespaces {
		// TODO: Implement Batch Get in Repo for performance optimization
		values, err := s.repo.GetValuesForNamespace(ctx, locale, ns)
		if err != nil {
			s.logger.Error("failed to fetch namespace values", err, log.String("ns", ns))
			continue
		}
		// Merge into main bundle
		for k, v := range values {
			// Key format: "namespace:key" or just "key" depending on frontend preference
			// We'll use "namespace.key" to avoid collisions
			fullKey := ns + "." + k
			bundle[fullKey] = v
		}
	}
	return bundle, nil
}

// --- Admin Methods ---

func (s *Service) CreateKey(ctx context.Context, key localization.TranslationKey) (*localization.TranslationKey, error) {
	if err := s.checkPermission(ctx); err != nil {
		return nil, err
	}

	if err := s.repo.CreateKey(ctx, &key); err != nil {
		return nil, err
	}

	actor := auth.ActorFromContext(ctx)
	evt := audit.NewEvent(actor.AccountID, "user", core.EventLocalizationKeyCreated).
		WithTarget(key.ID, "translation_key", key.Namespace+"."+key.Key).
		WithSuccess().Build()

	// Fix: internal/services/localization/service.go:123:16: Error return value of s.audit.Record is not checked
	if err := s.audit.Record(ctx, evt); err != nil {
		s.logger.Error("failed to record audit event", err)
	}

	return &key, nil
}

func (s *Service) UpsertValue(ctx context.Context, val localization.TranslationValue) error {
	if err := s.checkPermission(ctx); err != nil {
		return err
	}

	if err := s.repo.UpsertValue(ctx, &val); err != nil {
		return err
	}

	// Invalidate bundle cache (if implemented)
	// s.cache.Delete(ctx, fmt.Sprintf("bundle:%s:%s", val.Locale, namespace))

	actor := auth.ActorFromContext(ctx)
	evt := audit.NewEvent(actor.AccountID, "user", core.EventLocalizationValueUpdated).
		WithTarget(val.KeyID, "translation_value", string(val.Locale)).
		WithSuccess().Build()

	// Fix: internal/services/localization/service.go:143:16: Error return value of s.audit.Record is not checked
	if err := s.audit.Record(ctx, evt); err != nil {
		s.logger.Error("failed to record audit event", err)
	}

	return nil
}

func (s *Service) ListKeys(ctx context.Context, namespace string, page, limit int) ([]localization.TranslationKey, int, error) {
	if err := s.checkPermission(ctx); err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * limit
	return s.repo.ListKeys(ctx, namespace, limit, offset)
}

func (s *Service) checkPermission(ctx context.Context) error {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return auth.ErrUnauthorized
	}
	// TODO: Implement real RBAC check using s.authz
	return nil
}
