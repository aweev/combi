package feature_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	audit_mocks "github.com/combi/pkg/domain/audit/mocks"
	"github.com/combi/pkg/domain/feature"
	feature_mocks "github.com/combi/pkg/domain/feature/mocks"
	bus_mocks "github.com/combi/pkg/events/mocks"
	"github.com/combi/pkg/log"
	feature_service "github.com/combi/feature"
)

func TestService_GetClientBootstrapData(t *testing.T) {
	// Mocks
	mockCache := feature_mocks.NewMockFlagCacheReader(t)
	mockEngine := feature_mocks.NewMockEngine(t)
	mockRepo := feature_mocks.NewMockRepository(t)
	mockNotifier := feature_mocks.NewMockNotifier(t)
	mockAudit := audit_mocks.NewMockEventRecorder(t)
	mockBus := bus_mocks.NewMockBus(t)

	svc := feature_service.NewService(mockRepo, mockCache, mockNotifier, mockEngine, mockAudit, mockBus, &log.NoOpLogger{})

	ctx := context.Background()
	evalCtx := feature.EvaluationContext{EntityID: "anon"}

	t.Run("Returns Evaluated Flags", func(t *testing.T) {
		// Arrange
		f1 := &feature.Flag{Key: "f1"}
		f2 := &feature.Flag{Key: "f2"}

		mockCache.EXPECT().ListActiveFlags().Return([]*feature.Flag{f1, f2}).Once()

		// Engine returns computed results
		res1 := feature.EvaluationResult{Key: "f1", VariationID: "on", Value: json.RawMessage("true")}
		res2 := feature.EvaluationResult{Key: "f2", VariationID: "red", Value: json.RawMessage(`"red"`)}

		mockEngine.EXPECT().Evaluate(f1, evalCtx).Return(res1).Once()
		mockEngine.EXPECT().Evaluate(f2, evalCtx).Return(res2).Once()

		// Act
		bootstrap, err := svc.GetClientBootstrapData(ctx, evalCtx)

		// Assert
		require.NoError(t, err)
		assert.Len(t, bootstrap.Flags, 2)

		assert.Equal(t, "on", bootstrap.Flags["f1"].VariationID)
		assert.JSONEq(t, "true", string(bootstrap.Flags["f1"].Value))

		assert.Equal(t, "red", bootstrap.Flags["f2"].VariationID)
	})
}
