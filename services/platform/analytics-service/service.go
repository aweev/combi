package analytics

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/mileusna/useragent"
	"github.com/combi/pkg/domain/analytics"
	"github.com/combi/pkg/log"
)

const MaxPayloadSize = 16 * 1024

type Service struct {
	repo   analytics.Repository
	logger log.Logger

	// Ingestion buffer
	eventChan  chan analytics.AnalyticsEvent
	batchSize  int
	flushTimer time.Duration

	// Lifecycle
	wg       sync.WaitGroup
	shutDown atomic.Bool
}

func NewService(repo analytics.Repository, logger log.Logger) *Service {
	s := &Service{
		repo:       repo,
		logger:     logger.With(log.String("service", "analytics")),
		eventChan:  make(chan analytics.AnalyticsEvent, 5000), // High capacity
		batchSize:  100,
		flushTimer: 5 * time.Second,
	}

	s.wg.Add(1)
	go s.runFlusher()

	return s
}

// TrackEvent validates, enriches, and buffers the event.
func (s *Service) TrackEvent(ctx context.Context, e analytics.AnalyticsEvent) error {
	if s.shutDown.Load() {
		return errors.New("analytics service is shutting down")
	}

	// 1. Validation
	if len(e.Properties) > 0 {
		data, err := json.Marshal(e.Properties)
		if err != nil {
			return analytics.ErrInvalidEvent
		}
		if len(data) > MaxPayloadSize {
			s.logger.Warn("analytics payload too large", log.Int("size", len(data)))
			return errors.New("payload exceeds limit")
		}
	}

	// 2. Enrichment (User Agent Parsing)
	// We do this here (CPU bound) rather than in the worker to distribute load
	if e.UserAgent != "" {
		ua := useragent.Parse(e.UserAgent)
		if ua.Bot {
			e.DeviceType = "bot"
		} else if ua.Mobile {
			e.DeviceType = "mobile"
		} else if ua.Tablet {
			e.DeviceType = "tablet"
		} else {
			e.DeviceType = "desktop"
		}
	}

	// 3. Buffer (Non-blocking)
	select {
	case s.eventChan <- e:
		return nil
	default:
		// Buffer full: Drop event rather than block caller
		s.logger.Warn("analytics buffer full, dropping event", log.String("type", e.EventType))
		return nil
	}
}

func (s *Service) Alias(ctx context.Context, sessionID string, userID uuid.UUID) error {
	return s.repo.LinkAnonymousSession(ctx, sessionID, userID)
}

func (s *Service) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down analytics service...")

	// 1. Mark as shutting down to stop new accepts
	s.shutDown.Store(true)

	// 2. Close channel to signal flusher to drain
	close(s.eventChan)

	// 3. Wait for flusher to finish
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info("analytics service stopped")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Service) runFlusher() {
	defer s.wg.Done()

	// Pre-allocate buffer
	batch := make([]analytics.AnalyticsEvent, 0, s.batchSize)
	ticker := time.NewTicker(s.flushTimer)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := s.repo.InsertBatch(ctx, batch); err != nil {
			s.logger.Error("failed to flush analytics batch", err, log.Int("count", len(batch)))
		}

		// Optimization: Reuse the underlying array capacity to reduce GC churn
		// reset slice length to 0, keep capacity
		batch = batch[:0]
	}

	// Drain loop handles both ticker and channel close
	for {
		select {
		case e, ok := <-s.eventChan:
			if !ok {
				// Channel closed, flush remaining and exit
				flush()
				return
			}
			batch = append(batch, e)
			if len(batch) >= s.batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

// --- Reporting Methods (Pass-through) ---

func (s *Service) GetTopFeatures(ctx context.Context, limit int) ([]string, error) {
	return s.repo.GetTopFeatures(ctx, limit)
}

func (s *Service) GetDailyActiveUsers(ctx context.Context, start, end time.Time) ([]analytics.TimeSeriesPoint, error) {
	return s.repo.CountUniqueUsers(ctx, start, end)
}

func (s *Service) GetPageViews(ctx context.Context, path string, start, end time.Time) (int, error) {
	return s.repo.CountEventsByType(ctx, "page_view", start, end)
}

func (s *Service) GetConversionFunnel(ctx context.Context, steps []string, start, end time.Time) ([]analytics.FunnelStep, error) {
	counts, err := s.repo.GetFunnelCounts(ctx, steps, start, end)
	if err != nil {
		return nil, err
	}

	result := make([]analytics.FunnelStep, len(steps))
	previousCount := 0

	for i, stepName := range steps {
		count := counts[stepName]
		dropoff := 0
		if i > 0 && previousCount > 0 {
			// Calculate % drop from previous step
			dropoff = int(float64(previousCount-count) / float64(previousCount) * 100)
		}
		result[i] = analytics.FunnelStep{
			Name:      stepName,
			EventType: stepName,
			Count:     count,
			Dropoff:   dropoff,
		}
		previousCount = count
	}
	return result, nil
}
