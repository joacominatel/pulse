package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// Metrics holds all prometheus metrics for pulse.
// uses a custom registry to avoid polluting the global namespace.
type Metrics struct {
	Registry *prometheus.Registry

	// http_request_duration_seconds - histogram for api latency
	HTTPRequestDuration *prometheus.HistogramVec

	// pulse_events_ingested_total - counter for ingested events
	EventsIngestedTotal *prometheus.CounterVec

	// pulse_buffer_size - gauge for current event buffer size
	BufferSize prometheus.Gauge

	// pulse_momentum_calculation_duration_seconds - histogram for momentum worker
	MomentumCalculationDuration prometheus.Histogram
}

// New creates and registers all prometheus metrics.
func New() *Metrics {
	reg := prometheus.NewRegistry()

	// add standard go runtime and process collectors
	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	m := &Metrics{
		Registry: reg,

		HTTPRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path", "status"},
		),

		EventsIngestedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "pulse_events_ingested_total",
				Help: "Total number of activity events ingested",
			},
			[]string{"community_id", "event_type"},
		),

		BufferSize: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pulse_buffer_size",
			Help: "Current number of events waiting in the ingestion buffer",
		}),

		MomentumCalculationDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "pulse_momentum_calculation_duration_seconds",
			Help:    "Duration of momentum calculation cycles in seconds",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 10), // 100ms to ~100s
		}),
	}

	// register all custom metrics
	reg.MustRegister(
		m.HTTPRequestDuration,
		m.EventsIngestedTotal,
		m.BufferSize,
		m.MomentumCalculationDuration,
	)

	return m
}

// RecordHTTPRequest records the duration of an HTTP request.
func (m *Metrics) RecordHTTPRequest(method, path, status string, durationSeconds float64) {
	m.HTTPRequestDuration.WithLabelValues(method, path, status).Observe(durationSeconds)
}

// RecordEventIngested increments the events ingested counter.
func (m *Metrics) RecordEventIngested(communityID, eventType string) {
	m.EventsIngestedTotal.WithLabelValues(communityID, eventType).Inc()
}

// SetBufferSize sets the current buffer size gauge.
func (m *Metrics) SetBufferSize(size int) {
	m.BufferSize.Set(float64(size))
}

// RecordMomentumCalculation records the duration of a momentum calculation cycle.
func (m *Metrics) RecordMomentumCalculation(durationSeconds float64) {
	m.MomentumCalculationDuration.Observe(durationSeconds)
}
