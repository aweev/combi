package secret

import (
	"context"
	"time"

	"github.com/combi/pkg/domain/secret"
	"github.com/combi/pkg/circuit"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/monitoring"
)

// MonitoredRepository is a decorator that wraps another secret.Repository
// to add monitoring (metrics) and circuit breaking.
type MonitoredRepository struct {
	next       secret.Repository // The repository implementation we are wrapping
	monitoring monitoring.Service
	breaker    circuit.Breaker
	logger     log.Logger
}

// Ensure MonitoredRepository implements the secret.Repository interface at compile time.
var _ secret.Repository = (*MonitoredRepository)(nil)

// NewMonitoredRepository creates a new repository decorator.
func NewMonitoredRepository(
	repoToWrap secret.Repository,
	mon monitoring.Service,
	breaker circuit.Breaker,
	logger log.Logger,
) *MonitoredRepository {
	return &MonitoredRepository{
		next:       repoToWrap,
		monitoring: mon,
		breaker:    breaker,
		logger:     logger,
	}
}

// recordMetrics is a helper to record the duration and error count for a repository operation.
func (m *MonitoredRepository) recordMetrics(operation string, start time.Time, err error) {
	duration := time.Since(start).Seconds()
	labels := monitoring.MetricLabels{
		monitoring.LabelMethod: operation,
	}

	m.monitoring.RecordHistogram("ngo_secrets_repo_duration_seconds", duration, labels)

	if err != nil {
		m.monitoring.IncrementCounter("ngo_secrets_repo_errors_total", labels)
	}
}

// Get wraps the underlying repository's Get method with circuit breaking and monitoring.
func (m *MonitoredRepository) Get(ctx context.Context, name string) (s *secret.Secret, err error) {
	// Use defer to ensure metrics are recorded even if a panic occurs.
	defer func(start time.Time) {
		m.recordMetrics("get", start, err)
	}(time.Now())

	// Execute the operation within the circuit breaker.
	err = m.breaker.Execute(ctx, func() error {
		var innerErr error
		s, innerErr = m.next.Get(ctx, name)
		if innerErr != nil {
			m.logger.Warn("Secret repository 'get' operation failed",
				log.String("secret_name", name),
				log.ErrorAttr(innerErr),
			)
		}
		return innerErr
	})

	// If the circuit breaker returned an error (e.g., it's open), 's' will be nil.
	// We return the error from the breaker directly.
	if err != nil {
		return nil, err
	}

	return s, nil
}

// Set wraps the underlying repository's Set method with circuit breaking and monitoring.
func (m *MonitoredRepository) Set(ctx context.Context, s *secret.Secret) (err error) {
	defer func(start time.Time) {
		m.recordMetrics("set", start, err)
	}(time.Now())

	err = m.breaker.Execute(ctx, func() error {
		innerErr := m.next.Set(ctx, s)
		if innerErr != nil {
			m.logger.Warn("Secret repository 'set' operation failed",
				log.String("secret_name", s.Name),
				log.ErrorAttr(innerErr),
			)
		}
		return innerErr
	})

	return err
}

// List wraps the underlying repository's List method with circuit breaking and monitoring.
func (m *MonitoredRepository) List(ctx context.Context, prefix string) (keys []string, err error) {
	defer func(start time.Time) {
		m.recordMetrics("list", start, err)
	}(time.Now())

	err = m.breaker.Execute(ctx, func() error {
		var innerErr error
		keys, innerErr = m.next.List(ctx, prefix)
		if innerErr != nil {
			m.logger.Warn("Secret repository 'list' operation failed",
				log.String("prefix", prefix),
				log.ErrorAttr(innerErr),
			)
		}
		return innerErr
	})

	if err != nil {
		return nil, err
	}

	return keys, nil
}

// Delete wraps the underlying repository's Delete method with circuit breaking and monitoring.
func (m *MonitoredRepository) Delete(ctx context.Context, name string) (err error) {
	defer func(start time.Time) {
		m.recordMetrics("delete", start, err)
	}(time.Now())

	err = m.breaker.Execute(ctx, func() error {
		innerErr := m.next.Delete(ctx, name)
		if innerErr != nil {
			m.logger.Warn("Secret repository 'delete' operation failed",
				log.String("secret_name", name),
				log.ErrorAttr(innerErr),
			)
		}
		return innerErr
	})

	return err
}

func (m *MonitoredRepository) FindSecretsDueForRotation(ctx context.Context, now time.Time) (secrets []*secret.Secret, err error) {
	defer func(start time.Time) {
		m.recordMetrics("find_due_for_rotation", start, err)
	}(time.Now())

	err = m.breaker.Execute(ctx, func() error {
		var innerErr error
		secrets, innerErr = m.next.FindSecretsDueForRotation(ctx, now)
		if innerErr != nil {
			m.logger.Warn("Secret repository 'find_due_for_rotation' operation failed", log.ErrorAttr(innerErr))
		}
		return innerErr
	})

	if err != nil {
		return nil, err
	}

	return secrets, nil
}

func (m *MonitoredRepository) GetVersion(ctx context.Context, name string, version int) (s *secret.Secret, err error) {
	defer func(start time.Time) {
		m.recordMetrics("get_version", start, err)
	}(time.Now())

	err = m.breaker.Execute(ctx, func() error {
		var innerErr error
		s, innerErr = m.next.GetVersion(ctx, name, version)
		if innerErr != nil {
			// Log warning but don't swallow error
			m.logger.Warn("Secret repository 'get_version' operation failed",
				log.String("secret_name", name),
				log.Int("version", version),
				log.ErrorAttr(innerErr),
			)
		}
		return innerErr
	})

	if err != nil {
		return nil, err
	}

	return s, nil
}
