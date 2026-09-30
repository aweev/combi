package configuration_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/combi/pkg/domain/configuration"
	config_mocks "github.com/combi/pkg/domain/configuration/mocks"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/events"
	events_mocks "github.com/combi/pkg/events/mocks"
	"github.com/combi/pkg/log"
	config_service "github.com/combi/config/configuration"
)

func setupService(t *testing.T) (*config_service.Service, *config_mocks.MockRepository, *events_mocks.MockBus) {
	repo := config_mocks.NewMockRepository(t)
	bus := events_mocks.NewMockBus(t)
	svc := config_service.NewService(repo, bus, &log.NoOpLogger{})
	return svc, repo, bus
}

func TestService_GetBootstrap(t *testing.T) {
	ctx := context.Background()

	t.Run("Full Waterfall Resolution", func(t *testing.T) {
		svc, repo, _ := setupService(t)

		// 1. Setup Data
		sysSettings := []configuration.SystemSetting{
			{Key: "maintenance_mode", Value: false, IsPublic: true},
			{Key: "internal_api_key", Value: "secret", IsPublic: false},
		}

		themeID := uuid.New()
		baseTheme := &configuration.Theme{
			ID:        themeID,
			Name:      "Light Theme",
			Variables: map[string]string{"primary": "blue", "bg": "white"},
		}

		orgID := uuid.New()
		orgBrand := &configuration.Brand{
			ID:              uuid.New(),
			OrganizationID:  &orgID,
			ThemeID:         themeID,
			LogoURL:         "https://acme.com/logo.png",
			CustomOverrides: map[string]string{"primary": "red"},
		}

		// 2. Mocks
		repo.EXPECT().ListSettings(ctx).Return(sysSettings, nil)

		// FIX 1: Use typed nil (*uuid.UUID)(nil)
		repo.EXPECT().GetBrand(ctx, &orgID, (*uuid.UUID)(nil)).Return(orgBrand, nil)

		// FIX 2: Do NOT expect GetSeasonalTheme.
		// Logic: If specific brand found, seasonal check is skipped.

		repo.EXPECT().GetTheme(ctx, themeID).Return(baseTheme, nil)

		// 3. Execution
		bootstrap, err := svc.GetBootstrap(ctx, configuration.BootstrapParams{OrgID: &orgID})
		require.NoError(t, err)

		// 4. Assertions
		assert.Equal(t, false, bootstrap.Settings["maintenance_mode"])
		assert.NotContains(t, bootstrap.Settings, "internal_api_key")
		assert.Equal(t, "red", bootstrap.Theme["primary"]) // Override applied
		assert.Equal(t, "https://acme.com/logo.png", bootstrap.BrandAssets.LogoURL)
	})

	t.Run("Fallback to System Defaults", func(t *testing.T) {
		svc, repo, _ := setupService(t)

		sysTheme := &configuration.Theme{
			ID:        uuid.New(),
			Variables: map[string]string{"primary": "default-orange"},
		}
		sysBrand := &configuration.Brand{
			Name:    "NGO System",
			ThemeID: sysTheme.ID,
		}

		repo.EXPECT().ListSettings(ctx).Return([]configuration.SystemSetting{}, nil)

		// FIX 3: Do NOT expect GetBrand.
		// Logic: If params.OrgID is nil, code skips specific lookup and goes straight to SystemBrand.

		repo.EXPECT().GetSystemBrand(ctx).Return(sysBrand, nil)
		repo.EXPECT().GetSeasonalTheme(ctx, mock.Anything).Return(nil, nil)
		repo.EXPECT().GetTheme(ctx, sysTheme.ID).Return(sysTheme, nil)

		bootstrap, err := svc.GetBootstrap(ctx, configuration.BootstrapParams{OrgID: nil})
		require.NoError(t, err)

		assert.Equal(t, "default-orange", bootstrap.Theme["primary"])
	})
}

func TestService_UpdateTheme(t *testing.T) {
	ctx := context.Background()
	svc, repo, bus := setupService(t)

	t.Run("Blocks System Theme Update", func(t *testing.T) {
		sysTheme := &configuration.Theme{ID: uuid.New(), IsSystem: true}
		repo.EXPECT().GetTheme(ctx, sysTheme.ID).Return(sysTheme, nil)

		_, err := svc.UpdateTheme(ctx, *sysTheme)
		assert.ErrorIs(t, err, configuration.ErrSystemThemeProtected)
	})

	t.Run("Updates Custom Theme", func(t *testing.T) {
		customTheme := &configuration.Theme{ID: uuid.New(), IsSystem: false, Name: "New Name"}

		repo.EXPECT().GetTheme(ctx, customTheme.ID).Return(customTheme, nil)
		repo.EXPECT().UpsertTheme(ctx, mock.Anything).Return(nil)

		bus.EXPECT().Publish(mock.Anything, mock.MatchedBy(func(e events.DomainEvent) bool {
			return e.Action == core.EventThemeUpdated
		})).Return(nil)

		_, err := svc.UpdateTheme(ctx, *customTheme)
		require.NoError(t, err)

		// Wait for goroutine
		time.Sleep(50 * time.Millisecond)
	})
}

func TestRegistry_Concurrency(t *testing.T) {
	svc, _, _ := setupService(t)
	ctx := context.Background()

	m1 := core.ServiceManifest{ServiceName: "svc-1"}
	m2 := core.ServiceManifest{ServiceName: "svc-2"}

	go svc.RegisterService(m1)
	go svc.RegisterService(m2)

	time.Sleep(20 * time.Millisecond)

	caps, err := svc.GetSystemCapabilities(ctx)
	require.NoError(t, err)
	assert.Len(t, caps, 2)
}
