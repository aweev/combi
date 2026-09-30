package analytics_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/combi/pkg/domain/analytics"
	analytics_mocks "github.com/combi/pkg/domain/analytics/mocks"
	"github.com/combi/pkg/log"
	analytics_service "github.com/combi/analytics"
)

func TestService_TrackEvent_Enrichment(t *testing.T) {
	t.Run("should enrich desktop user agent", func(t *testing.T) {
		mockRepo := analytics_mocks.NewMockRepository(t)
		svc := analytics_service.NewService(mockRepo, &log.NoOpLogger{})

		// Expect the event to land in Repo with DeviceType = "desktop"
		mockRepo.EXPECT().
			InsertBatch(mock.Anything, mock.MatchedBy(func(events []analytics.AnalyticsEvent) bool {
				return len(events) == 1 && events[0].DeviceType == "desktop"
			})).
			Return(nil).
			Once()

		evt := analytics.AnalyticsEvent{
			ID:        uuid.New(),
			EventType: "view",
			// Chrome on Mac
			UserAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.114 Safari/537.36",
			CreatedAt: time.Now(),
		}

		err := svc.TrackEvent(context.Background(), evt)
		require.NoError(t, err)

		// Force flush
		svc.Shutdown(context.Background())
	})

	t.Run("should enrich mobile user agent", func(t *testing.T) {
		mockRepo := analytics_mocks.NewMockRepository(t)
		svc := analytics_service.NewService(mockRepo, &log.NoOpLogger{})

		mockRepo.EXPECT().
			InsertBatch(mock.Anything, mock.MatchedBy(func(events []analytics.AnalyticsEvent) bool {
				return len(events) == 1 && events[0].DeviceType == "mobile"
			})).
			Return(nil).
			Once()

		evt := analytics.AnalyticsEvent{
			ID:        uuid.New(),
			EventType: "tap",
			// iPhone
			UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 14_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.1.1 Mobile/15E148 Safari/604.1",
			CreatedAt: time.Now(),
		}

		err := svc.TrackEvent(context.Background(), evt)
		require.NoError(t, err)

		svc.Shutdown(context.Background())
	})

	t.Run("should enrich bot user agent", func(t *testing.T) {
		mockRepo := analytics_mocks.NewMockRepository(t)
		svc := analytics_service.NewService(mockRepo, &log.NoOpLogger{})

		mockRepo.EXPECT().
			InsertBatch(mock.Anything, mock.MatchedBy(func(events []analytics.AnalyticsEvent) bool {
				return len(events) == 1 && events[0].DeviceType == "bot"
			})).
			Return(nil).
			Once()

		evt := analytics.AnalyticsEvent{
			ID:        uuid.New(),
			EventType: "crawl",
			// Googlebot
			UserAgent: "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
			CreatedAt: time.Now(),
		}

		err := svc.TrackEvent(context.Background(), evt)
		require.NoError(t, err)

		svc.Shutdown(context.Background())
	})
}

func TestService_TrackEvent_Batching(t *testing.T) {
	// We assume batch size is 100 in implementation (hardcoded in NewService for this example)
	mockRepo := analytics_mocks.NewMockRepository(t)
	svc := analytics_service.NewService(mockRepo, &log.NoOpLogger{})

	totalEvents := 105
	batchSize := 100

	// 1. First batch triggers automatically when size hits 100
	mockRepo.EXPECT().
		InsertBatch(mock.Anything, mock.MatchedBy(func(events []analytics.AnalyticsEvent) bool {
			return len(events) == batchSize
		})).
		Return(nil).
		Once()

	// 2. Remaining 5 trigger on Shutdown
	mockRepo.EXPECT().
		InsertBatch(mock.Anything, mock.MatchedBy(func(events []analytics.AnalyticsEvent) bool {
			return len(events) == (totalEvents - batchSize)
		})).
		Return(nil).
		Once()

	ctx := context.Background()
	for i := 0; i < totalEvents; i++ {
		svc.TrackEvent(ctx, analytics.AnalyticsEvent{ID: uuid.New(), EventType: "test"})
	}

	// Wait briefly for the channel to process the first 100
	time.Sleep(50 * time.Millisecond)

	// Trigger shutdown to flush remainder
	err := svc.Shutdown(ctx)
	require.NoError(t, err)
}

func TestService_Alias(t *testing.T) {
	mockRepo := analytics_mocks.NewMockRepository(t)
	svc := analytics_service.NewService(mockRepo, &log.NoOpLogger{})
	defer svc.Shutdown(context.Background())

	sessionID := "anon-123"
	userID := uuid.New()

	mockRepo.EXPECT().
		LinkAnonymousSession(mock.Anything, sessionID, userID).
		Return(nil).
		Once()

	err := svc.Alias(context.Background(), sessionID, userID)
	require.NoError(t, err)
}
