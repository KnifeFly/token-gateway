package dispatch

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/internal/provider"
	"github.com/KnifeFly/token-gateway/internal/provider/relay"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
	"go.opentelemetry.io/otel/attribute"
)

// dispatcher.go owns provider fallback, attempt telemetry, and adapter boundary calls.

// Dispatcher calls registered provider adapters and records attempts.
type Dispatcher struct {
	registry    *provider.Registry
	observe     engine.ObserveRecorder
	attempts    AttemptRecorder
	credentials CredentialResolver
	disable     DisableChecker
	limits      AttemptLimiter
	reliability ReliabilityRecorder
	retry       RetryPolicy
	logger      *slog.Logger
}

// AttemptRecorder persists provider attempts.
type AttemptRecorder interface {
	RecordProviderAttempt(ctx context.Context, state *engine.RequestState, attempt engine.ProviderAttempt) error
}

// CredentialResolver resolves provider credentials outside runtime snapshots.
type CredentialResolver interface {
	ResolveProviderAPIKey(ctx context.Context, channel engine.ChannelView) (string, error)
}

// DisableChecker checks emergency provider/channel disables.
type DisableChecker interface {
	IsProviderDisabled(ctx context.Context, providerType string) (bool, error)
	IsChannelDisabled(ctx context.Context, channelID string) (bool, error)
}

// AttemptLimiter reserves provider/channel-scoped capacity for one dispatch attempt.
type AttemptLimiter interface {
	AcquireForCandidate(ctx context.Context, state *engine.RequestState, candidate engine.ProviderCandidate) (engine.LimitRelease, error)
}

// ReliabilityRecorder observes attempts for circuit breakers and hot signals.
type ReliabilityRecorder interface {
	RecordProviderAttempt(ctx context.Context, state *engine.RequestState, attempt engine.ProviderAttempt)
}

// RetryPolicy bounds provider fallback attempts for one request.
type RetryPolicy struct {
	MaxAttempts int
	MaxElapsed  time.Duration
}

// DefaultRetryPolicy returns conservative request-local retry limits.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{MaxAttempts: 3, MaxElapsed: 10 * time.Second}
}

// New returns a dispatcher without external credential resolution.
func New(registry *provider.Registry, observe engine.ObserveRecorder, attempts AttemptRecorder, logger *slog.Logger) *Dispatcher {
	return NewWithCredentials(registry, observe, attempts, nil, logger)
}

// NewWithCredentials returns a dispatcher with optional credential and disable checks.
func NewWithCredentials(registry *provider.Registry, observe engine.ObserveRecorder, attempts AttemptRecorder, credentials CredentialResolver, logger *slog.Logger, disable ...DisableChecker) *Dispatcher {
	if logger == nil {
		logger = slog.Default()
	}
	if observe == nil {
		observe = engine.NoopObserveRecorder{}
	}
	d := &Dispatcher{registry: registry, observe: observe, attempts: attempts, credentials: credentials, logger: logger, retry: DefaultRetryPolicy()}
	if len(disable) > 0 {
		d.disable = disable[0]
	}
	return d
}

// WithReliability configures the provider reliability recorder.
func (d *Dispatcher) WithReliability(recorder ReliabilityRecorder) *Dispatcher {
	if d == nil {
		return d
	}
	d.reliability = recorder
	return d
}

// WithAttemptLimiter configures provider/channel-scoped per-attempt limits.
func (d *Dispatcher) WithAttemptLimiter(limiter AttemptLimiter) *Dispatcher {
	if d == nil {
		return d
	}
	d.limits = limiter
	return d
}

// WithRetryPolicy configures request-local retry limits.
func (d *Dispatcher) WithRetryPolicy(policy RetryPolicy) *Dispatcher {
	if d == nil {
		return d
	}
	if policy.MaxAttempts <= 0 {
		policy.MaxAttempts = DefaultRetryPolicy().MaxAttempts
	}
	if policy.MaxElapsed <= 0 {
		policy.MaxElapsed = DefaultRetryPolicy().MaxElapsed
	}
	d.retry = policy
	return d
}

// Dispatch tries provider candidates in order and returns the first successful response.
func (d *Dispatcher) Dispatch(ctx context.Context, state *engine.RequestState) (*engine.ProviderResult, error) {
	if state.RoutePlan == nil || len(state.RoutePlan.Candidates) == 0 {
		return nil, apperr.ServiceUnavailable("no route is available", apperr.WithTemporary())
	}
	var lastErr error
	requestStarted := time.Now()
	retryBudget := d.retryBudget(len(state.RoutePlan.Candidates))
	retryBudgetLimit := retryBudget
	var firstFailed *engine.ProviderAttempt
	for _, candidate := range state.RoutePlan.Candidates {
		if retryBudget <= 0 || d.retryElapsed(requestStarted) {
			break
		}
		// Step 1: resolve adapter, channel, disable state, and credential.
		adapter, ok := d.registry.Adapter(candidate.ProviderType)
		if !ok {
			lastErr = apperr.ServiceUnavailable("provider adapter is unavailable", apperr.WithTemporary())
			continue
		}
		channel, ok := state.Snapshot.LookupChannel(candidate.ChannelID)
		if !ok || !channel.Enabled {
			lastErr = apperr.ServiceUnavailable("provider channel is unavailable", apperr.WithTemporary())
			continue
		}
		disabled, err := d.isDisabled(ctx, channel.ProviderType, channel.ID)
		if err != nil {
			lastErr = err
			continue
		}
		if disabled {
			lastErr = apperr.ServiceUnavailable("provider channel is disabled", apperr.WithTemporary())
			continue
		}
		apiKey := channel.APIKey
		if d.credentials != nil {
			resolved, err := d.credentials.ResolveProviderAPIKey(ctx, channel)
			if err != nil {
				lastErr = err
				continue
			}
			apiKey = resolved
		}
		attemptRelease, err := d.acquireAttemptLimit(ctx, state, candidate)
		if err != nil {
			if !candidateLimitFallback(err) {
				return nil, err
			}
			lastErr = err
			continue
		}
		releaseAttempt := true

		// Step 2: relay the request and build an auditable provider attempt.
		started := time.Now()
		spanCtx, span := d.observe.StartSpan(ctx, "gateway.provider_attempt",
			attribute.String("gateway.provider", candidate.ProviderType),
			attribute.String("gateway.channel_id", candidate.ChannelID),
			attribute.String("gateway.model", candidate.PublicModel),
		)
		response, err := adapter.Relay(spanCtx, relay.ChannelConfig{
			ChannelID:     candidate.ChannelID,
			ProviderType:  candidate.ProviderType,
			BaseURL:       channel.BaseURL,
			APIKey:        apiKey,
			UpstreamModel: candidate.UpstreamModel,
			Timeout:       candidate.Timeout,
		}, relay.Request{
			CanonicalAPI:  string(state.CanonicalAPI),
			PublicModel:   candidate.PublicModel,
			UpstreamModel: candidate.UpstreamModel,
			RawBody:       state.Parsed.RawBody,
			ContentType:   state.Incoming.Header.Get("Content-Type"),
			Headers:       state.Incoming.Header.Clone(),
			RequestID:     state.RequestID,
			Stream:        state.Stream,
		})
		attempt := engine.ProviderAttempt{
			AttemptIndex:         len(state.Attempts) + 1,
			ChannelID:            candidate.ChannelID,
			ProviderType:         candidate.ProviderType,
			PublicModel:          candidate.PublicModel,
			StartedAt:            started,
			Duration:             time.Since(started),
			RetryBudgetConsumed:  retryBudgetLimit - retryBudget + 1,
			RetryBudgetRemaining: retryBudget - 1,
			CircuitState:         circuitStateForCandidate(state, candidate),
		}
		if firstFailed != nil {
			attempt.FallbackFromChannelID = firstFailed.ChannelID
			attempt.FallbackFromProvider = firstFailed.ProviderType
		}
		if err != nil {
			// Step 3: record failed attempts before moving to fallback candidates.
			span.RecordError(err)
			lastErr = mapProviderError(err)
			attempt.Retryable = providerRetryable(err)
			attempt.ErrorCode = providerErrorCode(err)
			attempt.StatusCode = providerStatusCode(err)
			nextBudget := retryBudget - 1
			eligible := d.eligibleForFallback(err, state, nextBudget, requestStarted)
			if !eligible {
				attempt.Final = true
			}
			state.Attempts = append(state.Attempts, attempt)
			if firstFailed == nil {
				failed := attempt
				firstFailed = &failed
			}
			if recordErr := d.recordAttempt(ctx, state, attempt); recordErr != nil {
				if releaseAttempt {
					_ = attemptRelease.Release(ctx)
				}
				span.RecordError(recordErr)
				span.End()
				return nil, recordErr
			}
			d.recordReliability(ctx, state, attempt)
			span.End()
			if releaseAttempt {
				_ = attemptRelease.Release(ctx)
			}
			retryBudget = nextBudget
			if !eligible {
				return nil, lastErr
			}
			continue
		}

		// Step 4: record success and stop fallback.
		attempt.Success = true
		attempt.StatusCode = response.StatusCode
		attempt.Final = true
		state.ActualUsage = response.Usage
		state.Attempts = append(state.Attempts, attempt)
		if recordErr := d.recordAttempt(ctx, state, attempt); recordErr != nil {
			if releaseAttempt {
				_ = attemptRelease.Release(ctx)
			}
			span.RecordError(recordErr)
			span.End()
			return nil, recordErr
		}
		d.recordReliability(ctx, state, attempt)
		span.End()
		state.AddLimitRelease(attemptRelease)
		releaseAttempt = false
		return &engine.ProviderResult{
			Candidate: candidate,
			Response: &engine.GatewayResponse{
				StatusCode: response.StatusCode,
				Header:     response.Header,
				Body:       response.Body,
				Stream:     response.Stream,
				Usage:      response.Usage,
			},
			Usage: response.Usage,
		}, nil
	}
	if lastErr == nil {
		lastErr = apperr.ServiceUnavailable("provider is unavailable", apperr.WithTemporary())
	}
	markFinalAttempt(state)
	return nil, lastErr
}

func (d *Dispatcher) acquireAttemptLimit(ctx context.Context, state *engine.RequestState, candidate engine.ProviderCandidate) (engine.LimitRelease, error) {
	if d == nil || d.limits == nil {
		return noopAttemptRelease{}, nil
	}
	release, err := d.limits.AcquireForCandidate(ctx, state, candidate)
	if release == nil {
		release = noopAttemptRelease{}
	}
	return release, err
}

func candidateLimitFallback(err error) bool {
	appErr, ok := apperr.As(err)
	return ok && appErr.Code == apperr.CodeRateLimited
}

type noopAttemptRelease struct{}

func (noopAttemptRelease) Release(context.Context) error {
	return nil
}

func (d *Dispatcher) retryBudget(candidateCount int) int {
	maxAttempts := d.retry.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = DefaultRetryPolicy().MaxAttempts
	}
	if candidateCount > 0 && maxAttempts > candidateCount {
		maxAttempts = candidateCount
	}
	return maxAttempts
}

func (d *Dispatcher) retryElapsed(started time.Time) bool {
	return d.retry.MaxElapsed > 0 && time.Since(started) >= d.retry.MaxElapsed
}

func (d *Dispatcher) eligibleForFallback(err error, state *engine.RequestState, remaining int, started time.Time) bool {
	if remaining <= 0 || d.retryElapsed(started) {
		return false
	}
	if state == nil || len(state.Parsed.RawBody) == 0 {
		return false
	}
	return providerRetryable(err)
}

func (d *Dispatcher) isDisabled(ctx context.Context, providerType, channelID string) (bool, error) {
	if d.disable == nil {
		return false, nil
	}
	providerDisabled, err := d.disable.IsProviderDisabled(ctx, providerType)
	if err != nil || providerDisabled {
		return providerDisabled, err
	}
	return d.disable.IsChannelDisabled(ctx, channelID)
}

func (d *Dispatcher) recordAttempt(ctx context.Context, state *engine.RequestState, attempt engine.ProviderAttempt) error {
	if d.attempts == nil {
		return nil
	}
	return d.attempts.RecordProviderAttempt(ctx, state, attempt)
}

func (d *Dispatcher) recordReliability(ctx context.Context, state *engine.RequestState, attempt engine.ProviderAttempt) {
	d.observe.RecordProviderAttempt(ctx, state, attempt)
	if d.reliability != nil {
		d.reliability.RecordProviderAttempt(ctx, state, attempt)
	}
}

func markFinalAttempt(state *engine.RequestState) {
	if state == nil || len(state.Attempts) == 0 {
		return
	}
	state.Attempts[len(state.Attempts)-1].Final = true
}

func circuitStateForCandidate(state *engine.RequestState, candidate engine.ProviderCandidate) string {
	if state == nil || state.Internal == nil {
		return ""
	}
	states, ok := state.Internal["route.circuit_states"].(map[string]string)
	if !ok {
		return ""
	}
	return states[candidate.ChannelID]
}

func mapProviderError(err error) error {
	var providerErr *relay.ProviderError
	if !errors.As(err, &providerErr) {
		return apperr.ProviderError("provider request failed", apperr.WithCause(err), apperr.WithTemporary())
	}
	switch providerErr.StatusCode {
	case http.StatusTooManyRequests:
		return apperr.RateLimited("provider rate limited the request", apperr.WithCause(err), apperr.WithTemporary())
	case http.StatusUnauthorized, http.StatusForbidden:
		return apperr.ProviderError("provider authentication failed", apperr.WithCause(err))
	default:
		opts := []apperr.Option{apperr.WithCause(err)}
		if providerErr.Retryable {
			opts = append(opts, apperr.WithTemporary())
		}
		return apperr.ProviderError("provider request failed", opts...)
	}
}

func providerRetryable(err error) bool {
	var providerErr *relay.ProviderError
	return errors.As(err, &providerErr) && providerErr.Retryable
}

func providerErrorCode(err error) string {
	var providerErr *relay.ProviderError
	if errors.As(err, &providerErr) {
		return providerErr.Code
	}
	return "provider_error"
}

func providerStatusCode(err error) int {
	var providerErr *relay.ProviderError
	if errors.As(err, &providerErr) {
		return providerErr.StatusCode
	}
	return http.StatusBadGateway
}
