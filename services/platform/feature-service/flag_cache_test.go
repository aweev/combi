package feature_test

import (
	"context"
	"testing"
	"time"

	"github.com/combi/pkg/domain/feature"
	// Make sure this path matches where you generated your mocks
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	feature_mocks "github.com/combi/pkg/domain/feature/mocks"
	"github.com/combi/pkg/log"
	feature_service "github.com/combi/feature"
)

func TestFlagCache(t *testing.T) {
	// Base flag for testing
	flagV1 := &feature.Flag{Key: "flag1", IsActive: true, Name: "V1", Version: 1}

	t.Run("Refresh_IgnoresStaleUpdate", func(t *testing.T) {
		ctx := context.Background()
		repo := feature_mocks.NewMockRepository(t)

		// Initial load gets V2
		flagV2 := *flagV1
		flagV2.Version = 2
		flagV2.Name = "V2"

		// Expect ListAll to be called during NewFlagCache
		repo.EXPECT().ListAll(mock.Anything).Return([]feature.Flag{flagV2}, nil).Once()

		cache, err := feature_service.NewFlagCache(repo, &log.NoOpLogger{}, 1*time.Hour)
		require.NoError(t, err)

		// Check initial state is V2
		assert.Equal(t, 2, cache.GetFlag("flag1").Version)

		// Refresh triggered, but DB returns older V1 (simulating race condition or replication lag)
		repo.EXPECT().Get(ctx, "flag1").Return(flagV1, nil).Once()

		cache.Refresh(ctx, "flag1")

		// Assert cache STAYS at V2
		assert.Equal(t, 2, cache.GetFlag("flag1").Version, "Cache should not be overwritten by older version")
	})

	t.Run("Refresh_UpdatesOnNewerVersion", func(t *testing.T) {
		ctx := context.Background()
		repo := feature_mocks.NewMockRepository(t)
		repo.EXPECT().ListAll(mock.Anything).Return([]feature.Flag{*flagV1}, nil).Once()

		cache, _ := feature_service.NewFlagCache(repo, &log.NoOpLogger{}, 1*time.Hour)
		assert.Equal(t, 1, cache.GetFlag("flag1").Version)

		// New V3 available
		flagV3 := *flagV1
		flagV3.Version = 3
		flagV3.Name = "V3"
		repo.EXPECT().Get(ctx, "flag1").Return(&flagV3, nil).Once()

		cache.Refresh(ctx, "flag1")

		// Assert cache updates to V3
		assert.Equal(t, 3, cache.GetFlag("flag1").Version)
	})

	t.Run("GetFlag_ReturnsCopy", func(t *testing.T) {
		repo := feature_mocks.NewMockRepository(t)
		repo.EXPECT().ListAll(mock.Anything).Return([]feature.Flag{*flagV1}, nil).Once()
		cache, _ := feature_service.NewFlagCache(repo, &log.NoOpLogger{}, 1*time.Hour)

		f1 := cache.GetFlag("flag1")
		f1.Name = "Mutated"

		f2 := cache.GetFlag("flag1")
		assert.Equal(t, "V1", f2.Name, "Cache internal state should not be mutated")
	})
}
