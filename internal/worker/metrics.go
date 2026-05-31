package worker

import (
	metricnames "github.com/KnifeFly/token-gateway/internal/infra/telemetry"
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics records worker job execution health.
type Metrics struct {
	runs       *prometheus.CounterVec
	duration   *prometheus.HistogramVec
	inFlight   *prometheus.GaugeVec
	heartbeats *prometheus.CounterVec
}

// NewMetrics registers worker metrics.
func NewMetrics(registry *prometheus.Registry) (*Metrics, error) {
	m := &Metrics{
		runs: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: metricnames.MetricWorkerJobRunsTotal,
			Help: "Total worker job executions by outcome.",
		}, []string{"job", "outcome"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    metricnames.MetricWorkerJobDurationSeconds,
			Help:    "Worker job execution duration in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"job", "outcome"}),
		inFlight: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: metricnames.MetricWorkerJobInFlight,
			Help: "Current worker job executions.",
		}, []string{"job"}),
		heartbeats: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: metricnames.MetricWorkerLeaseHeartbeatsTotal,
			Help: "Total worker lease heartbeat renewals by outcome.",
		}, []string{"job", "outcome"}),
	}
	if registry == nil {
		return m, nil
	}
	for _, collector := range []prometheus.Collector{m.runs, m.duration, m.inFlight, m.heartbeats} {
		if err := registry.Register(collector); err != nil {
			if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
				return nil, err
			}
		}
	}
	return m, nil
}

func (m *Metrics) begin(job string) {
	if m == nil || m.inFlight == nil {
		return
	}
	m.inFlight.WithLabelValues(job).Inc()
}

func (m *Metrics) finish(job string, outcome string, seconds float64) {
	if m == nil {
		return
	}
	if outcome == "" {
		outcome = "unknown"
	}
	if m.inFlight != nil {
		m.inFlight.WithLabelValues(job).Dec()
	}
	if m.runs != nil {
		m.runs.WithLabelValues(job, outcome).Inc()
	}
	if m.duration != nil {
		m.duration.WithLabelValues(job, outcome).Observe(seconds)
	}
}

func (m *Metrics) skip(job string) {
	if m == nil || m.runs == nil {
		return
	}
	m.runs.WithLabelValues(job, "skipped").Inc()
}

func (m *Metrics) heartbeat(job string, outcome string) {
	if m == nil || m.heartbeats == nil {
		return
	}
	if outcome == "" {
		outcome = "unknown"
	}
	m.heartbeats.WithLabelValues(job, outcome).Inc()
}
