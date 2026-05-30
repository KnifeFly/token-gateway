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
	d := &Dispatcher{registry: registry, observe: observe, attempts: attempts, credentials: credentials, logger: logger}
	if len(disable) > 0 {
		d.disable = disable[0]
	}
	return d
}

// Dispatch tries provider candidates in order and returns the first successful response.
func (d *Dispatcher) Dispatch(ctx context.Context, state *engine.RequestState) (*engine.ProviderResult, error) {
	if state.RoutePlan == nil || len(state.RoutePlan.Candidates) == 0 {
		return nil, apperr.ServiceUnavailable("no route is available", apperr.WithTemporary())
	}
	var lastErr error
	for _, candidate := range state.RoutePlan.Candidates {
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
			RequestID:     state.RequestID,
			Stream:        state.Stream,
		})
		attempt := engine.ProviderAttempt{
			AttemptIndex: len(state.Attempts) + 1,
			ChannelID:    candidate.ChannelID,
			ProviderType: candidate.ProviderType,
			PublicModel:  candidate.PublicModel,
			StartedAt:    started,
			Duration:     time.Since(started),
		}
		if err != nil {
			// Step 3: record failed attempts before moving to fallback candidates.
			span.RecordError(err)
			lastErr = mapProviderError(err)
			attempt.ErrorCode = providerErrorCode(err)
			attempt.StatusCode = providerStatusCode(err)
			state.Attempts = append(state.Attempts, attempt)
			if recordErr := d.recordAttempt(ctx, state, attempt); recordErr != nil {
				span.RecordError(recordErr)
				span.End()
				return nil, recordErr
			}
			d.observe.RecordProviderAttempt(ctx, state, attempt)
			span.End()
			if !providerRetryable(err) {
				continue
			}
			continue
		}

		// Step 4: record success and stop fallback.
		attempt.Success = true
		attempt.StatusCode = response.StatusCode
		state.ActualUsage = response.Usage
		state.Attempts = append(state.Attempts, attempt)
		if recordErr := d.recordAttempt(ctx, state, attempt); recordErr != nil {
			span.RecordError(recordErr)
			span.End()
			return nil, recordErr
		}
		d.observe.RecordProviderAttempt(ctx, state, attempt)
		span.End()
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
	return nil, lastErr
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
