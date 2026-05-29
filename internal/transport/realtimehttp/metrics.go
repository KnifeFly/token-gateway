package realtimehttp

import (
	metricnames "github.com/KnifeFly/token-gateway/internal/infra/telemetry"
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics owns the reserved realtime Prometheus counters.
type Metrics struct {
	sessions    *prometheus.CounterVec
	connections *prometheus.CounterVec
}

// NewMetrics registers realtime metrics when a registry is available.
func NewMetrics(registry *prometheus.Registry) (*Metrics, error) {
	metrics := &Metrics{
		sessions: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: metricnames.MetricRealtimeSessionsTotal,
			Help: "Total realtime session API requests by outcome.",
		}, []string{"operation", "outcome", "error_code"}),
		connections: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: metricnames.MetricRealtimeConnectionsTotal,
			Help: "Total realtime connection attempts by outcome.",
		}, []string{"operation", "outcome", "error_code"}),
	}
	if registry == nil {
		return metrics, nil
	}
	for _, collector := range []prometheus.Collector{metrics.sessions, metrics.connections} {
		if err := registry.Register(collector); err != nil {
			if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
				return nil, err
			}
		}
	}
	return metrics, nil
}

func (m *Metrics) recordSession(operation string, err error) {
	if m == nil || m.sessions == nil {
		return
	}
	outcome, code := metricOutcome(err)
	m.sessions.WithLabelValues(operation, outcome, code).Inc()
}

func (m *Metrics) recordConnection(operation string, err error) {
	if m == nil || m.connections == nil {
		return
	}
	outcome, code := metricOutcome(err)
	m.connections.WithLabelValues(operation, outcome, code).Inc()
}
