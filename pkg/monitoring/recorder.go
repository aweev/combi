package monitoring

import (
	"database/sql"
	"fmt"
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var _ Service = (*recorder)(nil)

type recorder struct {
	registry    *prometheus.Registry
	counters    map[string]*prometheus.CounterVec
	gauges      map[string]*prometheus.GaugeVec
	histograms  map[string]*prometheus.HistogramVec
	serviceName string
	mu          sync.RWMutex
}

func NewRecorder(serviceName string) Service {
	registry := prometheus.NewRegistry()

	registry.MustRegister(collectors.NewGoCollector())
	registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	return &recorder{
		registry:    registry,
		counters:    make(map[string]*prometheus.CounterVec),
		gauges:      make(map[string]*prometheus.GaugeVec),
		histograms:  make(map[string]*prometheus.HistogramVec),
		serviceName: serviceName,
	}
}

func (rec *recorder) HTTPHandler() http.Handler {
	return promhttp.HandlerFor(rec.registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})
}

func (rec *recorder) RegisterCounterVec(name, help string, labelNames []string) error {
	rec.mu.Lock()
	defer rec.mu.Unlock()

	if _, exists := rec.counters[name]; exists {
		return fmt.Errorf("%w: counter '%s'", ErrMetricAlreadyRegistered, name)
	}
	counter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:        name,
		Help:        help,
		ConstLabels: prometheus.Labels{"service": rec.serviceName},
	}, labelNames)
	if err := rec.registry.Register(counter); err != nil {
		return fmt.Errorf("failed to register counter '%s': %w", name, err)
	}
	rec.counters[name] = counter
	return nil
}

func (rec *recorder) RegisterGaugeVec(name, help string, labelNames []string) error {
	rec.mu.Lock()
	defer rec.mu.Unlock()

	if _, exists := rec.gauges[name]; exists {
		return fmt.Errorf("%w: gauge '%s'", ErrMetricAlreadyRegistered, name)
	}
	gauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name:        name,
		Help:        help,
		ConstLabels: prometheus.Labels{"service": rec.serviceName},
	}, labelNames)
	if err := rec.registry.Register(gauge); err != nil {
		return fmt.Errorf("failed to register gauge '%s': %w", name, err)
	}
	rec.gauges[name] = gauge
	return nil
}

func (rec *recorder) RegisterHistogramVec(name, help string, labelNames []string) error {
	rec.mu.Lock()
	defer rec.mu.Unlock()

	if _, exists := rec.histograms[name]; exists {
		return fmt.Errorf("%w: histogram '%s'", ErrMetricAlreadyRegistered, name)
	}
	histo := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:        name,
		Help:        help,
		ConstLabels: prometheus.Labels{"service": rec.serviceName},
		Buckets:     prometheus.DefBuckets, // Default buckets: .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10
	}, labelNames)
	if err := rec.registry.Register(histo); err != nil {
		return fmt.Errorf("failed to register histogram '%s': %w", name, err)
	}
	rec.histograms[name] = histo
	return nil
}

func (rec *recorder) RegisterDBStatsCollector(db *sql.DB, dbName string) error {
	collector := collectors.NewDBStatsCollector(db, dbName)
	if err := rec.registry.Register(collector); err != nil {
		return fmt.Errorf("failed to register db stats collector for '%s': %w", dbName, err)
	}
	return nil
}

func (rec *recorder) RegisterDefaultMetrics() error {
	// HTTP Metrics
	if err := rec.RegisterCounterVec(MetricHTTPRequestsTotal, "Total number of HTTP requests.", []string{string(LabelMethod), string(LabelPath), string(LabelCode)}); err != nil {
		return err
	}
	if err := rec.RegisterHistogramVec(MetricHTTPRequestDurationSeconds, "Latency of HTTP requests in seconds.", []string{string(LabelMethod), string(LabelPath)}); err != nil {
		return err
	}

	// Queue Metrics
	if err := rec.RegisterCounterVec(MetricJobsDispatchedTotal, "Total number of jobs dispatched.", []string{string(LabelQueue), string(LabelJobType)}); err != nil {
		return err
	}
	if err := rec.RegisterCounterVec(MetricJobsProcessedTotal, "Total number of jobs processed.", []string{string(LabelQueue), string(LabelJobType), string(LabelStatus)}); err != nil {
		return err
	}
	if err := rec.RegisterHistogramVec(MetricJobDurationSeconds, "Duration of job processing in seconds.", []string{string(LabelQueue), string(LabelJobType)}); err != nil {
		return err
	}
	if err := rec.RegisterGaugeVec(MetricQueueJobs, "Current number of jobs in a queue by state.", []string{string(LabelQueue), string(LabelState)}); err != nil {
		return err
	}
	if err := rec.RegisterCounterVec(MetricReaperJobsClaimedTotal, "Total number of stalled jobs re-claimed by the reaper.", []string{string(LabelQueue)}); err != nil {
		return err
	}
	if err := rec.RegisterCounterVec(MetricReaperJobsDLQTotal, "Total number of jobs moved to the DLQ by the reaper.", []string{string(LabelQueue)}); err != nil {
		return err
	}
	if err := rec.RegisterCounterVec(MetricSchedulerRunsTotal, "Total number of times a scheduled job has been triggered.", []string{string(LabelJobType), string(LabelStatus)}); err != nil {
		return err
	}

	if err := rec.RegisterCounterVec(MetricEventsPublishedTotal, "Total number of events published.", []string{string(LabelTopic)}); err != nil {
		return err
	}
	if err := rec.RegisterCounterVec(MetricEventsProcessedTotal, "Total number of events processed by subscribers.", []string{string(LabelTopic), string(LabelStatus)}); err != nil {
		return err
	}

	if err := rec.RegisterCounterVec(MetricAlertsEvaluatedTotal, "Total number of alert rule evaluations.", []string{string(LabelAlertName), string(LabelStatus)}); err != nil {
		return err
	}
	if err := rec.RegisterCounterVec(MetricAlertsSentTotal, "Total number of alert notifications sent.", []string{string(LabelAlertName), string(LabelSeverity), string(LabelStatus)}); err != nil {
		return err
	}

	return nil
}

func (rec *recorder) IncrementCounter(name string, labels MetricLabels) {
	rec.mu.RLock()
	defer rec.mu.RUnlock()
	if counter, ok := rec.counters[name]; ok {
		counter.With(toPromLabels(labels)).Inc()
	}
}

func (rec *recorder) AddToCounter(name string, value float64, labels MetricLabels) {
	rec.mu.RLock()
	defer rec.mu.RUnlock()
	if counter, ok := rec.counters[name]; ok {
		counter.With(toPromLabels(labels)).Add(value)
	}
}

func (rec *recorder) SetGauge(name string, value float64, labels MetricLabels) {
	rec.mu.RLock()
	defer rec.mu.RUnlock()
	if gauge, ok := rec.gauges[name]; ok {
		gauge.With(toPromLabels(labels)).Set(value)
	}
}

func (rec *recorder) RecordHistogram(name string, value float64, labels MetricLabels) {
	rec.mu.RLock()
	defer rec.mu.RUnlock()
	if histo, ok := rec.histograms[name]; ok {
		histo.With(toPromLabels(labels)).Observe(value)
	}
}

func toPromLabels(labels MetricLabels) prometheus.Labels {
	promLabels := make(prometheus.Labels)
	for k, v := range labels {
		promLabels[string(k)] = v
	}
	return promLabels
}
