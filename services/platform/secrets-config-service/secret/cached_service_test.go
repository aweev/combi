package secret_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/combi/pkg/domain/secret"
	secret_mocks "github.com/combi/pkg/domain/secret/mocks"
	cache_mocks "github.com/combi/pkg/cache/mocks"
	"github.com/combi/pkg/log"
	secret_service "github.com/combi/config/secret"
)

func TestCachedService_GetSecretValue(t *testing.T) {
	ctx := context.Background()
	key := "stripe_key"
	expectedValue := secret.SecretValue{"key": "sk_123"}

	t.Run("Cache Hit", func(t *testing.T) {
		mockNext := secret_mocks.NewMockService(t)
		mockCache := cache_mocks.NewMockService(t)
		svc := secret_service.NewCachedSecretService(mockNext, mockCache, time.Minute, &log.NoOpLogger{})

		// Simulate cache hit
		mockCache.EXPECT().
			GetObject(ctx, "secret:stripe_key", mock.Anything).
			Run(func(_ context.Context, _ string, dest interface{}) {
				// Manually fill the destination pointer
				bytes, _ := json.Marshal(expectedValue)
				json.Unmarshal(bytes, dest)
			}).
			Return(true, nil).Once()

		// Next service should NOT be called
		val, err := svc.GetSecretValue(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, expectedValue, val)
	})

	t.Run("Cache Miss", func(t *testing.T) {
		mockNext := secret_mocks.NewMockService(t)
		mockCache := cache_mocks.NewMockService(t)
		svc := secret_service.NewCachedSecretService(mockNext, mockCache, time.Minute, &log.NoOpLogger{})

		// 1. Cache Miss
		mockCache.EXPECT().GetObject(ctx, "secret:stripe_key", mock.Anything).Return(false, nil).Once()

		// 2. Call Real Service
		mockNext.EXPECT().GetSecretValue(ctx, key).Return(expectedValue, nil).Once()

		// 3. Async Set (Use Maybe or relax constraint due to goroutine)
		// To strictly test this, we'd need a mechanism to wait for the goroutine.
		// For now, we simply assert that if called, it has correct params.
		mockCache.EXPECT().Set(mock.Anything, "secret:stripe_key", expectedValue, time.Minute).Return(nil).Maybe()

		val, err := svc.GetSecretValue(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, expectedValue, val)

		// Tiny sleep to allow goroutine to trigger (for code coverage mainly)
		time.Sleep(10 * time.Millisecond)
	})

	t.Run("Cache Error (Fallthrough)", func(t *testing.T) {
		mockNext := secret_mocks.NewMockService(t)
		mockCache := cache_mocks.NewMockService(t)
		svc := secret_service.NewCachedSecretService(mockNext, mockCache, time.Minute, &log.NoOpLogger{})

		// Redis is down
		mockCache.EXPECT().GetObject(ctx, "secret:stripe_key", mock.Anything).Return(false, errors.New("redis down")).Once()

		// Should still call next
		mockNext.EXPECT().GetSecretValue(ctx, key).Return(expectedValue, nil).Once()

		// Should attempt to set (and fail, probably, but that's async)
		mockCache.EXPECT().Set(mock.Anything, "secret:stripe_key", expectedValue, time.Minute).Return(errors.New("redis down")).Maybe()

		val, err := svc.GetSecretValue(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, expectedValue, val)

		time.Sleep(10 * time.Millisecond)
	})
}
