package snapshot

import (
	"context"
	"log/slog"
	"time"

	cpsnapshot "github.com/KnifeFly/token-gateway/internal/controlplane/snapshot"
	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	metricnames "github.com/KnifeFly/token-gateway/internal/infra/telemetry"
	"github.com/prometheus/client_golang/prometheus"
)

// ActiveRuntimeProvider loads the currently active control-plane runtime snapshot.
type ActiveRuntimeProvider interface {
	ActiveRuntimeSnapshot(ctx context.Context) (*cpsnapshot.RuntimeSnapshot, bool, error)
}

// EventSource notifies the watcher that a new active snapshot may be available.
type EventSource interface {
	SnapshotEvents(ctx context.Context) (<-chan struct{}, func() error, error)
}

// Watcher polls the active snapshot and atomically replaces the data-plane store.
type Watcher struct {
	provider ActiveRuntimeProvider
	store    *Store
	metrics  *Metrics
	interval time.Duration
	logger   *slog.Logger
	events   EventSource
}

// WatcherOption configures watcher behavior.
type WatcherOption func(*Watcher)

// WithEventSource adds push-triggered polling to the watcher.
func WithEventSource(events EventSource) WatcherOption {
	return func(w *Watcher) {
		w.events = events
	}
}

// NewWatcher returns a snapshot watcher.
func NewWatcher(provider ActiveRuntimeProvider, store *Store, metrics *Metrics, interval time.Duration, logger *slog.Logger, opts ...WatcherOption) *Watcher {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	if logger == nil {
		logger = slog.Default()
	}
	watcher := &Watcher{provider: provider, store: store, metrics: metrics, interval: interval, logger: logger}
	for _, opt := range opts {
		opt(watcher)
	}
	return watcher
}

// FallbackActiveProvider reads primary first and falls back to the secondary provider.
type FallbackActiveProvider struct {
	primary  ActiveRuntimeProvider
	fallback ActiveRuntimeProvider
}

// NewFallbackActiveProvider returns an active snapshot provider with fallback.
func NewFallbackActiveProvider(primary ActiveRuntimeProvider, fallback ActiveRuntimeProvider) *FallbackActiveProvider {
	return &FallbackActiveProvider{primary: primary, fallback: fallback}
}

// ActiveRuntimeSnapshot loads the active runtime snapshot from primary or fallback.
func (p *FallbackActiveProvider) ActiveRuntimeSnapshot(ctx context.Context) (*cpsnapshot.RuntimeSnapshot, bool, error) {
	if p == nil {
		return nil, false, nil
	}
	var primaryErr error
	if p.primary != nil {
		runtime, ok, err := p.primary.ActiveRuntimeSnapshot(ctx)
		if err == nil && ok {
			return runtime, true, nil
		}
		primaryErr = err
	}
	if p.fallback != nil {
		runtime, ok, err := p.fallback.ActiveRuntimeSnapshot(ctx)
		if err != nil || ok {
			return runtime, ok, err
		}
	}
	return nil, false, primaryErr
}

// Start runs the watcher until ctx is canceled.
func (w *Watcher) Start(ctx context.Context) {
	if w == nil || w.provider == nil || w.store == nil {
		return
	}
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	eventCh, closeEvents := w.subscribeEvents(ctx)
	defer func() {
		if closeEvents != nil {
			_ = closeEvents()
		}
	}()
	_ = w.Poll(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-eventCh:
			if !ok {
				eventCh = nil
				continue
			}
			if err := w.Poll(ctx); err != nil {
				w.logger.Warn("snapshot event poll failed", "error", err)
			}
		case <-ticker.C:
			if err := w.Poll(ctx); err != nil {
				w.logger.Warn("snapshot poll failed", "error", err)
			}
		}
	}
}

func (w *Watcher) subscribeEvents(ctx context.Context) (<-chan struct{}, func() error) {
	if w == nil || w.events == nil {
		return nil, nil
	}
	events, closeEvents, err := w.events.SnapshotEvents(ctx)
	if err != nil {
		w.logger.Warn("snapshot event subscription failed", "error", err)
		return nil, nil
	}
	return events, closeEvents
}

// Poll loads and swaps the active snapshot if it changed.
func (w *Watcher) Poll(ctx context.Context) error {
	runtime, ok, err := w.provider.ActiveRuntimeSnapshot(ctx)
	if err != nil || !ok {
		if w.metrics != nil && err != nil {
			w.metrics.RecordPublishError()
		}
		return err
	}
	current, _ := w.store.Current()
	if current != nil && current.Ref().Version == runtime.Version {
		if w.metrics != nil {
			w.metrics.Observe(current.Ref())
		}
		return nil
	}
	indexed, err := Build(*runtime)
	if err != nil {
		if w.metrics != nil {
			w.metrics.RecordPublishError()
		}
		return err
	}
	if err := w.store.Replace(indexed); err != nil {
		if w.metrics != nil {
			w.metrics.RecordPublishError()
		}
		return err
	}
	if w.metrics != nil {
		w.metrics.Observe(indexed.Ref())
	}
	return nil
}

// Metrics records snapshot version and staleness.
type Metrics struct {
	versionValue *prometheus.GaugeVec
	staleness    prometheus.Gauge
	errors       prometheus.Counter
}

// NewMetrics registers snapshot metrics.
func NewMetrics(registry *prometheus.Registry) (*Metrics, error) {
	if registry == nil {
		return nil, nil
	}
	metrics := &Metrics{
		versionValue: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: metricnames.MetricSnapshotActive,
			Help: "Active runtime snapshot version.",
		}, []string{"version"}),
		staleness: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: metricnames.MetricSnapshotStalenessSeconds,
			Help: "Age of the active runtime snapshot.",
		}),
		errors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: metricnames.MetricSnapshotPublishErrorsTotal,
			Help: "Total snapshot load/build/publish errors.",
		}),
	}
	if err := registry.Register(metrics.versionValue); err != nil {
		return nil, err
	}
	if err := registry.Register(metrics.staleness); err != nil {
		return nil, err
	}
	if err := registry.Register(metrics.errors); err != nil {
		return nil, err
	}
	return metrics, nil
}

// Observe records the current active snapshot.
func (m *Metrics) Observe(ref engine.SnapshotRef) {
	if m == nil {
		return
	}
	if ref.Version != "" && m.versionValue != nil {
		m.versionValue.WithLabelValues(ref.Version).Set(1)
	}
	if !ref.CreatedAt.IsZero() && m.staleness != nil {
		m.staleness.Set(time.Since(ref.CreatedAt).Seconds())
	}
}

// RecordPublishError increments snapshot publish/load error count.
func (m *Metrics) RecordPublishError() {
	if m != nil && m.errors != nil {
		m.errors.Inc()
	}
}
