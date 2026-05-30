package billing

import (
	metricnames "github.com/KnifeFly/token-gateway/internal/infra/telemetry"
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics records billing health signals.
type Metrics struct {
	settlementFailures *prometheus.CounterVec
	failedBacklog      prometheus.Gauge
}

// NewMetrics registers billing metrics with the provided registry.
func NewMetrics(registry *prometheus.Registry) (*Metrics, error) {
	m := &Metrics{
		settlementFailures: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: metricnames.MetricSettlementFailuresTotal,
			Help: "Total settlement failures by safe reason class.",
		}, []string{"reason"}),
		failedBacklog: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: metricnames.MetricFailedSettlementBacklog,
			Help: "Current failed settlement backlog.",
		}),
	}
	if registry == nil {
		m.settlementFailures.WithLabelValues("settlement_error").Add(0)
		return m, nil
	}
	for _, collector := range []prometheus.Collector{m.settlementFailures, m.failedBacklog} {
		if err := registry.Register(collector); err != nil {
			if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
				return nil, err
			}
		}
	}
	m.settlementFailures.WithLabelValues("settlement_error").Add(0)
	return m, nil
}

// RecordSettlementFailure increments the safe settlement failure counter.
func (m *Metrics) RecordSettlementFailure() {
	if m == nil || m.settlementFailures == nil {
		return
	}
	m.settlementFailures.WithLabelValues("settlement_error").Inc()
}

// SetFailedBacklog sets the current failed settlement backlog gauge.
func (m *Metrics) SetFailedBacklog(value int) {
	if m == nil || m.failedBacklog == nil {
		return
	}
	m.failedBacklog.Set(float64(value))
}

// IncrementFailedBacklog increments the failed settlement backlog gauge.
func (m *Metrics) IncrementFailedBacklog() {
	if m == nil || m.failedBacklog == nil {
		return
	}
	m.failedBacklog.Inc()
}

// DecrementFailedBacklog decrements the failed settlement backlog gauge.
func (m *Metrics) DecrementFailedBacklog() {
	if m == nil || m.failedBacklog == nil {
		return
	}
	m.failedBacklog.Dec()
}
