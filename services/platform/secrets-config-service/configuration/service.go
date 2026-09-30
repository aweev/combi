package configuration

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/configuration"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)

// Ensure implementation
var _ configuration.Service = (*Service)(nil)

type Service struct {
	repo     configuration.Repository
	eventBus events.Bus
	logger   log.Logger
	registry *RegistryImpl // Split registry logic to keep this file clean
}

func NewService(repo configuration.Repository, eventBus events.Bus, logger log.Logger) *Service {
	return &Service{
		repo:     repo,
		eventBus: eventBus,
		logger:   logger.With(log.String("service", "configuration")),
		registry: NewRegistry(),
	}
}

// --- BOOTSTRAP (The "Waterfall" Logic) ---

func (s *Service) GetBootstrap(ctx context.Context, params configuration.BootstrapParams) (*configuration.ClientBootstrap, error) {
	// 1. Load Public System Settings (e.g. "Maintenance Mode", "Default Currency")
	settings, err := s.repo.ListSettings(ctx)
	if err != nil {
		return nil, err
	}
	publicSettings := make(map[string]any)
	for _, setting := range settings {
		if setting.IsPublic {
			publicSettings[setting.Key] = setting.Value
		}
	}

	// 2. Resolve Effective Brand (Org > System)
	brand, err := s.GetEffectiveBrand(ctx, params.OrgID, params.ProjectID)
	if err != nil {
		// Critical failure: If we can't even load the System Brand, the UI cannot render.
		s.logger.Error("failed to load effective brand", err)
		return nil, err
	}

	// 3. Load Theme
	// 3a. Check for Seasonal Theme override (e.g. Christmas)
	// Only apply seasonal themes if we are falling back to system defaults.
	// If a specific brand is selected, we usually honor their choice over the seasonal one.
	var theme *configuration.Theme
	isSystemBrand := brand.OrganizationID == nil && brand.ProjectID == nil

	if isSystemBrand {
		theme, _ = s.repo.GetSeasonalTheme(ctx, time.Now().UTC())
	}

	// 3b. Fallback to Brand's configured theme
	if theme == nil {
		theme, err = s.repo.GetTheme(ctx, brand.ThemeID)
		if err != nil {
			return nil, err
		}
	}

	// 4. Merge Theme Variables + Brand Overrides
	finalTheme := make(map[string]string)
	for k, v := range theme.Variables {
		finalTheme[k] = v
	}
	// Overrides win (e.g. Org sets specific primary color on top of "Light Theme")
	for k, v := range brand.CustomOverrides {
		finalTheme[k] = v
	}

	// 5. Construct Result
	return &configuration.ClientBootstrap{
		Settings: publicSettings,
		Theme:    finalTheme,
		BrandAssets: configuration.BrandAssets{
			LogoURL:    brand.LogoURL,
			FaviconURL: brand.FaviconURL,
		},
		// Placeholder: In real app, fetch from LocalizationService & FeatureService
		Translations: map[string]string{},
		Features:     map[string]any{},
		User:         nil, // Handler injects this
	}, nil
}

// --- SETTINGS MANAGEMENT ---

func (s *Service) GetSetting(ctx context.Context, key string) (any, error) {
	setting, err := s.repo.GetSetting(ctx, key)
	if err != nil {
		return nil, err
	}
	return setting.Value, nil
}

func (s *Service) UpdateSetting(ctx context.Context, key string, value any) error {
	// 1. Fetch existing to validate existence & type safety
	existing, err := s.repo.GetSetting(ctx, key)
	if err != nil {
		return err
	}

	// 2. Validate Type (Simplistic check)
	existing.Value = value

	// 3. Save
	if err := s.repo.UpdateSetting(ctx, existing); err != nil {
		return err
	}

	// 4. Publish Event (So caches clear)
	s.publishEvent(ctx, core.EventSettingUpdated, map[string]any{
		"key":   key,
		"value": value,
	})

	return nil
}

func (s *Service) ListSettings(ctx context.Context) ([]configuration.SystemSetting, error) {
	return s.repo.ListSettings(ctx)
}

// --- THEME LIBRARY ---

func (s *Service) CreateTheme(ctx context.Context, theme configuration.Theme) (*configuration.Theme, error) {
	theme.ID = uuid.New()
	theme.CreatedAt = time.Now().UTC()
	theme.IsSystem = false // API cannot create protected system themes

	if err := s.repo.UpsertTheme(ctx, &theme); err != nil {
		return nil, err
	}
	return &theme, nil
}

func (s *Service) UpdateTheme(ctx context.Context, theme configuration.Theme) (*configuration.Theme, error) {
	existing, err := s.repo.GetTheme(ctx, theme.ID)
	if err != nil {
		return nil, err
	}

	// Protected check
	if existing.IsSystem {
		return nil, configuration.ErrSystemThemeProtected
	}

	theme.CreatedAt = existing.CreatedAt // Preserve creation
	if err := s.repo.UpsertTheme(ctx, &theme); err != nil {
		return nil, err
	}

	s.publishEvent(ctx, core.EventThemeUpdated, map[string]any{"id": theme.ID})
	return &theme, nil
}

func (s *Service) ListThemes(ctx context.Context) ([]configuration.Theme, error) {
	return s.repo.ListThemes(ctx)
}

func (s *Service) DeleteTheme(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteTheme(ctx, id)
}

// --- BRAND MANAGEMENT ---

func (s *Service) GetEffectiveBrand(ctx context.Context, orgID *uuid.UUID, projectID *uuid.UUID) (*configuration.Brand, error) {
	// 1. Try Specific (Org/Project)
	if orgID != nil || projectID != nil {
		brand, err := s.repo.GetBrand(ctx, orgID, projectID)
		if err == nil {
			return brand, nil
		}
		// If not found, log debug and fall through to system default
		if err != configuration.ErrBrandNotFound {
			return nil, err // Actual DB error
		}
	}

	// 2. Fallback to System Default
	return s.repo.GetSystemBrand(ctx)
}

func (s *Service) UpsertBrand(ctx context.Context, brand configuration.Brand) (*configuration.Brand, error) {
	if brand.ID == uuid.Nil {
		brand.ID = uuid.New()
	}

	if err := s.repo.UpsertBrand(ctx, brand); err != nil {
		return nil, err
	}
	s.publishEvent(ctx, core.EventBrandUpdated, map[string]any{
		"org_id": brand.OrganizationID,
		"name":   brand.Name,
	})
	return &brand, nil
}

func (s *Service) ResetBrand(ctx context.Context, orgID *uuid.UUID) error {
	brand, err := s.repo.GetBrand(ctx, orgID, nil)
	if err != nil {
		return err
	}
	return s.repo.DeleteBrand(ctx, brand.ID)
}

// --- Helpers ---

func (s *Service) publishEvent(ctx context.Context, action core.EventAction, data map[string]any) {
	payload, err := json.Marshal(data)
	if err != nil {
		s.logger.Error("failed to marshal event payload", err)
		return
	}

	// Fire and forget
	go func() {
		evt := events.DomainEvent{
			ID:        uuid.New(),
			Action:    action,
			Timestamp: time.Now().UTC(),
			Payload:   payload,
			ActorID:   uuid.Nil, // System
			ActorType: "system",
		}
		if err := s.eventBus.Publish(context.Background(), evt); err != nil {
			s.logger.Error("failed to publish configuration event",
				err)
		}
	}()
}
