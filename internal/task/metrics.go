package task

import (
	metricnames "github.com/KnifeFly/token-gateway/internal/infra/telemetry"
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics records async task and callback health signals.
type Metrics struct {
	transitions     *prometheus.CounterVec
	callbackRetries *prometheus.CounterVec
}

// NewMetrics registers task metrics.
func NewMetrics(registry *prometheus.Registry) (*Metrics, error) {
	m := &Metrics{
		transitions: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: metricnames.MetricTaskLifecycleTransitions,
			Help: "Total async task lifecycle transitions.",
		}, []string{"kind", "from_state", "to_state"}),
		callbackRetries: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: metricnames.MetricCallbackRetriesTotal,
			Help: "Total callback delivery retries by reason class.",
		}, []string{"reason"}),
	}
	if registry == nil {
		return m, nil
	}
	for _, collector := range []prometheus.Collector{m.transitions, m.callbackRetries} {
		if err := registry.Register(collector); err != nil {
			if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
				return nil, err
			}
		}
	}
	return m, nil
}

// RecordTransition records a task state transition.
func (m *Metrics) RecordTransition(kind Kind, from Status, to Status) {
	if m == nil || m.transitions == nil || to == "" {
		return
	}
	if kind == "" {
		kind = "unknown"
	}
	if from == "" {
		from = "new"
	}
	m.transitions.WithLabelValues(string(kind), string(from), string(to)).Inc()
}

// RecordCallbackRetry records one callback retry.
func (m *Metrics) RecordCallbackRetry(reason string) {
	if m == nil || m.callbackRetries == nil {
		return
	}
	if reason == "" {
		reason = "callback_error"
	}
	m.callbackRetries.WithLabelValues(reason).Inc()
}
