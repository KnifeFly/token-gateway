package router

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
)

const (
	// CircuitClosed allows normal traffic.
	CircuitClosed = "closed"
	// CircuitOpen rejects new route candidates until the open timeout elapses.
	CircuitOpen = "open"
	// CircuitHalfOpen allows a limited probe after the open timeout elapses.
	CircuitHalfOpen = "half_open"
)

// CircuitConfig controls in-process provider circuit transitions.
type CircuitConfig struct {
	FailureThreshold         int
	MinSamples               int
	OpenTimeout              time.Duration
	HalfOpenSuccessThreshold int
}

// DefaultCircuitConfig returns conservative local circuit defaults.
func DefaultCircuitConfig() CircuitConfig {
	return CircuitConfig{
		FailureThreshold:         3,
		MinSamples:               3,
		OpenTimeout:              30 * time.Second,
		HalfOpenSuccessThreshold: 1,
	}
}

// CircuitBreaker is both a hot signal provider and a provider-attempt observer.
type CircuitBreaker struct {
	mu      sync.Mutex
	config  CircuitConfig
	entries map[string]*circuitEntry
}

type circuitEntry struct {
	State        string
	Samples      int
	Failures     int
	Successes    int
	OpenedAt     time.Time
	LastError    string
	LastObserved time.Time
}

// NewCircuitBreaker returns an in-process provider/channel/model circuit breaker.
func NewCircuitBreaker(config CircuitConfig) *CircuitBreaker {
	if config.FailureThreshold <= 0 {
		config.FailureThreshold = DefaultCircuitConfig().FailureThreshold
	}
	if config.MinSamples <= 0 {
		config.MinSamples = config.FailureThreshold
	}
	if config.OpenTimeout <= 0 {
		config.OpenTimeout = DefaultCircuitConfig().OpenTimeout
	}
	if config.HalfOpenSuccessThreshold <= 0 {
		config.HalfOpenSuccessThreshold = DefaultCircuitConfig().HalfOpenSuccessThreshold
	}
	return &CircuitBreaker{config: config, entries: map[string]*circuitEntry{}}
}

// Signals returns circuit state as route signals.
func (b *CircuitBreaker) Signals(_ context.Context, _ *engine.RequestState, candidates []engine.ProviderCandidate) (RouteSignals, error) {
	out := RouteSignals{Candidates: map[string]CandidateSignal{}}
	if b == nil || len(candidates) == 0 {
		return out, nil
	}
	now := time.Now().UTC()
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, candidate := range candidates {
		key := circuitKey(candidate)
		entry := b.entry(key)
		b.refreshLocked(entry, now)
		signal := CandidateSignal{
			Healthy:         entry.State != CircuitOpen,
			HealthWeight:    circuitHealthWeight(entry.State),
			ModelCompatible: true,
			CircuitState:    entry.State,
		}
		if entry.Samples > 0 {
			signal.ErrorRate = float64(entry.Failures) / float64(entry.Samples)
			signal.SuccessRate = float64(entry.Successes) / float64(entry.Samples)
		}
		out.Candidates[candidate.ChannelID] = signal
	}
	return out, nil
}

// RecordProviderAttempt updates circuit state from a provider attempt.
func (b *CircuitBreaker) RecordProviderAttempt(_ context.Context, _ *engine.RequestState, attempt engine.ProviderAttempt) {
	if b == nil || attempt.ChannelID == "" {
		return
	}
	now := time.Now().UTC()
	b.mu.Lock()
	defer b.mu.Unlock()
	entry := b.entry(attemptKey(attempt))
	b.refreshLocked(entry, now)
	entry.Samples++
	entry.LastObserved = now
	if attempt.Success {
		entry.Successes++
		if entry.State == CircuitHalfOpen && entry.Successes >= b.config.HalfOpenSuccessThreshold {
			entry.State = CircuitClosed
			entry.Samples = 0
			entry.Failures = 0
			entry.Successes = 0
			entry.LastError = ""
		}
		return
	}
	if !isCircuitFailure(attempt.ErrorCode) {
		return
	}
	entry.Failures++
	entry.LastError = attempt.ErrorCode
	if entry.State == CircuitHalfOpen || (entry.Samples >= b.config.MinSamples && entry.Failures >= b.config.FailureThreshold) {
		entry.State = CircuitOpen
		entry.OpenedAt = now
	}
}

func (b *CircuitBreaker) entry(key string) *circuitEntry {
	entry := b.entries[key]
	if entry == nil {
		entry = &circuitEntry{State: CircuitClosed}
		b.entries[key] = entry
	}
	return entry
}

func (b *CircuitBreaker) refreshLocked(entry *circuitEntry, now time.Time) {
	if entry.State != CircuitOpen || entry.OpenedAt.IsZero() || now.Sub(entry.OpenedAt) < b.config.OpenTimeout {
		return
	}
	entry.State = CircuitHalfOpen
	entry.Samples = 0
	entry.Failures = 0
	entry.Successes = 0
}

func circuitKey(candidate engine.ProviderCandidate) string {
	return candidate.ProviderType + "|" + candidate.ChannelID + "|" + candidate.PublicModel
}

func attemptKey(attempt engine.ProviderAttempt) string {
	return attempt.ProviderType + "|" + attempt.ChannelID + "|" + attempt.PublicModel
}

func circuitHealthWeight(state string) float64 {
	switch state {
	case CircuitHalfOpen:
		return 0.1
	case CircuitOpen:
		return 0
	default:
		return 1
	}
}

func isCircuitFailure(code string) bool {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "provider_rate_limited", "provider_unavailable", "provider_timeout":
		return true
	default:
		return false
	}
}
